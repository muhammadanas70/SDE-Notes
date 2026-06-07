# Bare-Metal Networking Databases
## Complete In-Depth Systems Internals Guide
### Routing, Firewalls, Kernel Stack, Cloud — Data Structures, Algorithms, C & Rust

---

## Table of Contents

1. [Why Not PostgreSQL? — The Nanosecond Budget](#1-why-not-postgresql)
2. [Memory as the Database — Hierarchy, Cache Lines, NUMA](#2-memory-hierarchy)
3. [The Three Planes: Data, Control, Management](#3-three-planes)
4. [Routing Tables — The LPM Problem](#4-routing-tables)
5. [ARP and Neighbor Cache](#5-arp-neighbor-cache)
6. [Connection Tracking — The Stateful Firewall Engine](#6-connection-tracking)
7. [Firewall Rule Engines: iptables, nftables, eBPF](#7-firewall-engines)
8. [Traffic Control and QoS](#8-traffic-control)
9. [Protocol Headers and Packet Metadata](#9-protocol-headers)
10. [eBPF Maps — The Universal In-Kernel KV Store](#10-ebpf-maps)
11. [Hardware: CAM, TCAM, ASICs, P4](#11-hardware)
12. [Cloud Networking Data Structures](#12-cloud-networking)
13. [Concurrency Without Databases: RCU, Per-CPU, Seqlocks](#13-concurrency)
14. [Rust Implementations](#14-rust-implementations)
15. [Complete System View: Full Packet Journey](#15-complete-system-view)

---

## 1. Why Not PostgreSQL?

The core question: when you type `ip route add 10.0.0.0/8 via 192.168.1.1`, where does
that route go, and how is it looked up at line-rate (millions of packets per second)?

### The Latency Budget

A 10 Gbps NIC processes a 64-byte packet every 51 nanoseconds. A 100 Gbps NIC: every 5 ns.
PostgreSQL's best-case query time on a warm cache is 50–200 microseconds — 1000–40,000x too slow.

```
LATENCY HIERARCHY — WHY DATABASES DON'T WORK FOR PACKET FORWARDING
=======================================================================

                    ┌─────────────────────────────────┐
                    │          CPU Registers           │  ~0.3 ns   | ~1 KB
                    └────────────────┬────────────────┘
                                     │ 1–3 cycles
                    ┌────────────────▼────────────────┐
                    │   L1 Data Cache (per-core)       │  ~1   ns   | 32–64 KB
                    └────────────────┬────────────────┘
                                     │ 4–10 cycles
                    ┌────────────────▼────────────────┐
                    │   L2 Cache (per-core)            │  ~4   ns   | 256 KB–1 MB
                    └────────────────┬────────────────┘
                                     │ 10–40 cycles
                    ┌────────────────▼────────────────┐
                    │   L3 Cache (shared, LLC)         │  ~10–40 ns | 8–64 MB
                    └────────────────┬────────────────┘
                                     │ 100–200 cycles (cross-NUMA ×2)
                    ┌────────────────▼────────────────┐
                    │   DRAM (Main Memory)             │  ~60–100 ns| 32–512 GB
                    └────────────────┬────────────────┘
                                     │ kernel trap + syscall + driver
                    ┌────────────────▼────────────────┐
                    │   NVMe SSD (PCIe 4.0)            │  ~50–100 μs| 1–8 TB
                    └────────────────┬────────────────┘
                                     │ B-tree + WAL + transaction
                    ┌────────────────▼────────────────┐
                    │   PostgreSQL / MySQL Query       │  ~200 μs – 10 ms
                    └─────────────────────────────────┘

VERDICT:
  10GbE packet inter-arrival:   ~51 ns      ← must decide and forward IN THIS TIME
  100GbE packet inter-arrival:  ~5  ns
  PostgreSQL best-case query:   ~200,000 ns ← 4000x too slow for 10GbE
  DRAM random access:           ~100 ns     ← worst acceptable for packet forwarding
  L1 cache hit:                 ~1 ns       ← ideal for hot-path data (ARP entry, flow)
```

### What "Database" Means in Networking

Networking components use in-memory, kernel-space data structures that are:

- **Lockless or minimally locked** — no transaction logs, no MVCC, no WAL
- **Cache-aligned** — data laid out to fit in 1–4 cache lines (64 bytes each)
- **Algorithm-specific** — the data structure IS the lookup algorithm
- **Per-CPU where possible** — eliminate inter-core cache invalidation
- **RCU-protected** — readers never block, writers pay a small deferred cost

There is no "database engine." The data structure, the index, and the query optimizer
are one unified piece of code, often a single function call.

### Why Each "Database" Primitive is Rejected

```
PRIMITIVE      WHY REJECTED FOR PACKET FORWARDING
──────────────────────────────────────────────────────────────────────────
B-tree index   Disk-optimized (wide nodes for I/O). For memory, too many
               pointer chases; poor cache performance vs. tries.
SQL query      Parsing + planning overhead = milliseconds. Unacceptable.
Transactions   Locking + WAL + MVCC = too slow. RCU replaces this.
Heap storage   Row-oriented. We need structure-of-arrays, not array-of-
               structures, for cache efficiency on hot fields.
Buffer pool    Unnecessary. Everything must be in DRAM at all times.
               No eviction; kernel OOM kills processes instead.
ACID           Too expensive. Eventual consistency via RCU is fine.
               Route updates propagate in microseconds, not milliseconds.
```

---

## 2. Memory Hierarchy — The Real Database Engine

Memory itself IS the database. The data structures are designed to exploit cache
hierarchy: hot data in registers and L1, warm data in L2/L3, cold config in DRAM.

### Cache Lines — The Atomic Unit

Every cache operation is 64 bytes (one cache line on x86/ARM). This is the fundamental
unit of memory access. All networking data structures are designed around this.

```c
/*
 * A neighbour (ARP entry) is 256 bytes — 4 cache lines.
 * Hot fields (ha = MAC address, output fn ptr) are at the END,
 * placed together so a single cache line fetch gives you both.
 *
 * From include/net/neighbour.h (simplified):
 */
struct neighbour {
    /* Cache line 1: linkage, table ref, timestamps */
    struct neighbour __rcu  *next;          /* 8B */
    struct neigh_table      *tbl;           /* 8B */
    struct neigh_parms      *parms;         /* 8B */
    unsigned long           confirmed;      /* 8B */
    unsigned long           updated;        /* 8B */
    rwlock_t                lock;           /* 8B */
    refcount_t              refcnt;         /* 4B */
    unsigned int            arp_queue_len_bytes; /* 4B */
    /* ---- 64 bytes ---- cache line 1 done ---- */

    /* Cache line 2: ARP queue, timer */
    struct sk_buff_head     arp_queue;      /* 24B */
    struct timer_list       timer;          /* 24B */
    /* ... */
    /* ---- 64 bytes ---- cache line 2 done ---- */

    /* Cache line 3: state, probes, type */
    unsigned long           used;           /* 8B */
    atomic_t                probes;         /* 4B */
    u8                      nud_state;      /* 1B: NUD_REACHABLE etc. */
    u8                      type;           /* 1B */
    u8                      dead;           /* 1B */
    u8                      protocol;       /* 1B */
    /* ... */

    /* Cache line 4: HOT DATA — hardware address + output fn */
    unsigned char           ha[MAX_ADDR_LEN]; /* 6B MAC address */
    struct hh_cache         hh;             /* prebuilt L2 header */
    int (*output)(struct neighbour *, struct sk_buff *); /* fn ptr: 8B */
    /* ---- 64 bytes ---- cache line 4 (HOT PATH) ---- */
};
```

### NUMA — Non-Uniform Memory Access

Modern servers have multiple CPU sockets, each with local DRAM. Accessing remote
NUMA node memory costs 2–3x more latency.

```
NUMA TOPOLOGY (2-socket server, simplified)

  ┌─────────────────────────────┐   QPI/UPI Link   ┌─────────────────────────────┐
  │  Socket 0 (Node 0)          │◄─────────────────►│  Socket 1 (Node 1)          │
  │                             │   ~80-120 ns      │                             │
  │  Core 0 Core 1 Core 2 ...   │                   │  Core N Core N+1 ...        │
  │    L1    L1    L1           │                   │    L1    L1                 │
  │       L2    L2              │                   │       L2    L2              │
  │          L3 (LLC)           │                   │          L3 (LLC)           │
  │                             │                   │                             │
  │  Local DRAM: ~60 ns access  │                   │  Local DRAM: ~60 ns access  │
  │  Remote DRAM: ~120-140 ns   │                   │  Remote DRAM: ~120-140 ns   │
  │                             │                   │                             │
  │  NIC (eth0) via PCIe        │                   │  NIC (eth1) via PCIe        │
  └─────────────────────────────┘                   └─────────────────────────────┘

NETWORKING IMPLICATION:
  - NIC DMA and its IRQ handler run on a core local to the NIC's NUMA node
  - sk_buff (packet buffer) is allocated from that NUMA node's memory
  - Routing table lookups must happen on the same node to avoid remote DRAM access
  - Per-CPU routing caches (rt_cache) keep hot entries local
```

### Per-CPU Data — Eliminating Cache Contention

```c
/*
 * Global counter = cache thrashing (every write invalidates the
 * cache line on other cores). Solution: per-CPU counters.
 *
 * From net/core/dev.c — network device statistics:
 */
DEFINE_PER_CPU(struct netdev_queue_stats, netdev_queue_stats);

/* Read: sum across all CPUs */
static u64 get_total_rx_packets(struct net_device *dev)
{
    u64 total = 0;
    int cpu;
    for_each_possible_cpu(cpu)
        total += per_cpu_ptr(dev->tstats, cpu)->rx_packets;
    return total;
}

/* Write: only touch current CPU's copy — no cache invalidation */
static void rx_packet_counted(struct net_device *dev)
{
    this_cpu_inc(netdev_queue_stats.rx_packets);  /* lockless! */
}
```

### Memory Allocators — Why Slab/SLUB

The kernel uses slab allocator (SLUB is the default implementation) instead of
generic malloc. Slab pre-allocates pools of fixed-size objects:

```
SLAB ALLOCATOR — sk_buff cache example

  kmem_cache "skbuff_head_cache"
  ┌─────────────────────────────────────────────────────┐
  │  Slab 1 (one or more pages)                         │
  │  ┌──────┬──────┬──────┬──────┬──────┬──────┬──────┐│
  │  │sk_buf│sk_buf│sk_buf│sk_buf│sk_buf│sk_buf│sk_buf││
  │  │  (A) │  (B) │ free │  (C) │ free │  (D) │ free ││
  │  └──────┴──────┴──────┴──────┴──────┴──────┴──────┘│
  └─────────────────────────────────────────────────────┘

  Benefits:
  - No fragmentation (all objects same size)
  - Constructor (e.g., spinlock init) run once at slab creation
  - Per-CPU freelist: allocation is a single pointer update, no locks
  - Cache-hot: just-freed object goes back to same CPU's cache
  - NUMA-aware: slabs allocated on the node that will use them
```

---

## 3. The Three Planes

Every router/firewall/kernel stack separates concerns into three planes:

```
THREE PLANES OF NETWORK DEVICE OPERATION
=========================================

┌─────────────────────────────────────────────────────────────────────┐
│  MANAGEMENT PLANE  (slowest: seconds to minutes)                    │
│                                                                     │
│  • CLI / YANG / NETCONF / RESTCONF / gNMI                          │
│  • Configuration parsing and validation                             │
│  • Databases: SQLite, YANG datastore, flat files, Git               │
│  • User-space daemons: vtysh, FRR, sysctl, iproute2                │
│  • "ip route add 10.0.0.0/8 via 192.168.1.1" runs HERE             │
│  • Results pushed DOWN to control plane                             │
└────────────────────────┬────────────────────────────────────────────┘
                         │  Netlink socket / ioctl / bpf() syscall
                         │  (microseconds for propagation)
┌────────────────────────▼────────────────────────────────────────────┐
│  CONTROL PLANE  (medium speed: milliseconds)                        │
│                                                                     │
│  • Routing protocols: BGP, OSPF, IS-IS, RIP                        │
│  • Route computation: Dijkstra (OSPF), Bellman-Ford (BGP)          │
│  • RIB (Routing Information Base): best-route selection             │
│  • Netlink: pushes RIB entries into FIB (kernel data structures)   │
│  • ARP/NDP resolution, LLDP                                        │
│  • iptables-restore, nft, tc operations from user-space             │
│  • Databases used: in-process hash tables, red-black trees          │
│    (FRR uses its own in-memory route table, not PostgreSQL)         │
└────────────────────────┬────────────────────────────────────────────┘
                         │  Kernel FIB write (RCU-protected)
                         │  (microseconds to update trie node)
┌────────────────────────▼────────────────────────────────────────────┐
│  DATA PLANE  (fastest: nanoseconds per packet)                      │
│                                                                     │
│  • Kernel: net/ipv4/ip_forward.c, net/netfilter/, net/sched/       │
│  • XDP (eXpress Data Path): before sk_buff allocation              │
│  • DPDK: user-space, bypasses kernel entirely                       │
│  • Hardware ASICs: forwarding in silicon                            │
│  • Data structures: FIB trie, conntrack hash, nftables sets         │
│  • These are the "databases" this guide is about                    │
│  • Read path: LOCKLESS (RCU readers), ns-scale                     │
│  • Write path: rare, writer pays full cost                          │
└─────────────────────────────────────────────────────────────────────┘

DATA FLOW:
  admin types "ip route add ..."
       ↓ iproute2 calls
  Netlink RTNL message → kernel → fib_table_insert() → trie write
       ↓ RCU synchronize
  Next packet lookup: fib_table_lookup() → trie read, LOCKLESS
```

### How User-Space Config Reaches Kernel Data Structures

```c
/* When "ip route add 10.0.0.0/8 via 192.168.1.1" runs:
 *
 * 1. iproute2 builds a Netlink RTM_NEWROUTE message
 * 2. sends it via send() on a NETLINK_ROUTE socket
 * 3. kernel receives it in net/ipv4/fib_frontend.c:inet_rtm_newroute()
 * 4. calls fib_table_insert() → trie is modified
 * 5. RCU grace period ensures old readers finish safely
 * 6. next packet lookup sees new route IMMEDIATELY
 *
 * NO PostgreSQL. NO disk. NO transaction log. Pure in-memory trie update.
 */

/* Netlink message structure (how iproute2 talks to kernel) */
struct nlmsghdr {
    __u32   nlmsg_len;    /* Length including header */
    __u16   nlmsg_type;   /* RTM_NEWROUTE, RTM_DELROUTE, etc. */
    __u16   nlmsg_flags;  /* NLM_F_REQUEST | NLM_F_ACK | NLM_F_CREATE */
    __u32   nlmsg_seq;    /* Sequence number */
    __u32   nlmsg_pid;    /* Process PID */
};

struct rtmsg {
    unsigned char   rtm_family;    /* AF_INET, AF_INET6 */
    unsigned char   rtm_dst_len;   /* Prefix length: 8 for /8 */
    unsigned char   rtm_src_len;   /* Source prefix (usually 0) */
    unsigned char   rtm_tos;       /* TOS */
    unsigned char   rtm_table;     /* RT_TABLE_MAIN = 254 */
    unsigned char   rtm_protocol;  /* RTPROT_STATIC, RTPROT_BGP, etc. */
    unsigned char   rtm_scope;     /* RT_SCOPE_UNIVERSE */
    unsigned char   rtm_type;      /* RTN_UNICAST */
    unsigned        rtm_flags;
};
/* Followed by rtattr TLVs: RTA_DST, RTA_GATEWAY, RTA_OIF, etc. */
```

---

## 4. Routing Tables — The LPM Problem

### What is LPM (Longest Prefix Match)?

When a packet arrives destined for 10.5.3.200, the router has these routes:
```
  0.0.0.0/0    via 192.168.1.254   (default route — matches everything)
  10.0.0.0/8   via 10.1.1.1        (matches 10.x.x.x)
  10.5.0.0/16  via 10.1.2.1        (matches 10.5.x.x)
  10.5.3.0/24  via 10.1.3.1        (matches 10.5.3.x — WINNER: longest prefix)
```

All four match the destination. The rule is: use the most specific (longest prefix).
This is LPM — the fundamental routing problem.

### RIB vs FIB — Two Different Databases

```
RIB (Routing Information Base)           FIB (Forwarding Information Base)
────────────────────────────────         ────────────────────────────────────────
Lives in USER SPACE (FRR, BIRD, etc.)    Lives in KERNEL SPACE (or ASIC)
Contains ALL routes from all protocols   Contains ONLY best route per prefix
BGP path attributes, communities         Just: prefix → nexthop + interface
Used for route selection / policy        Used for packet forwarding, nanoseconds
Can use more memory (secondary storage)  Must be fastest possible lookup
Written to: config changes, BGP updates  Written to: when RIB best-route changes
Read by: control plane algorithms        Read by: data plane, every packet

EXAMPLE:
  RIB has 3 routes to 10.0.0.0/8:
    via EBGP peer A (AS_PATH 65001 65002, MED 100)
    via EBGP peer B (AS_PATH 65003, MED 200)
    via OSPF (cost 10)

  After BGP best-path selection:
    FIB has ONE entry: 10.0.0.0/8 via peer A's nexthop

  Linux FIB: struct fib_table + LC-Trie (fib_trie.c)
  Linux RIB: struct fib_info + fib_alias lists
```

### 4.1 Naive Approaches — Why They Fail

```
APPROACH 1: Linear Scan through routing table
  for each route in table:
      if (dest & route.mask) == route.prefix:
          candidates.add(route)
  return max_prefixlen(candidates)

  Time: O(n)  — n = number of routes
  For DFZ (Default-Free Zone, full Internet table): n = 950,000+
  At 100GbE: 20M packets/sec × 950K comparisons = 19 TRILLION ops/sec
  → IMPOSSIBLE

APPROACH 2: Binary search on sorted prefix array
  Sort routes by prefix value, binary search for range
  Time: O(log n) ≈ O(20) for 1M routes
  Problem: LPM requires checking multiple overlapping prefixes
  → Still requires O(n) comparisons in worst case for overlap resolution

APPROACH 3: Hash table (exact match)
  Hash the entire 32-bit destination address
  Time: O(1) — perfect!
  Problem: only finds /32 hosts. "Route 10.0.0.0/8 applies to 10.x.x.x"
           cannot be expressed as a single hash key.
  → Cannot do LPM

SOLUTION: Tries (prefix trees)
  Store each route at the level corresponding to its prefix length
  Traverse the tree from root to leaf, bit by bit
  Last matching node = longest prefix match
  Time: O(W) where W = address width (32 for IPv4, 128 for IPv6)
  → O(32) = constant time!
```

### 4.2 Patricia Trie (Binary Radix Tree)

The original LPM data structure, described by Donald Morrison in 1968, used in early
UNIX routing. A Patricia trie (Practical Algorithm To Retrieve Information Coded In
Alphanumeric) is a compressed binary trie where chains of single-child nodes are
merged into one edge.

```
PATRICIA TRIE STRUCTURE — IPv4 /30 prefixes (simplified to 8-bit keys)

  Routes: 10.0.0.0/8, 10.5.0.0/16, 10.5.3.0/24, 10.8.0.0/16, 192.168.0.0/16

  Binary representation (first 8 bits):
    10  = 00001010
    192 = 11000000

                           root (check bit 7)
                          /                  \
                    bit=0                     bit=1 (MSB)
                    /                              \
             (10.x.x.x area)               (192.168.x.x area)
             check bit 6                   check bit 6
                /    \                           |
           bit=0      bit=1               192.168.0.0/16
           (0-63)     (64-127)               [LEAF]
                  ...

  Each INTERNAL node stores:
    - bit position to test
    - left child (bit=0)
    - right child (bit=1)
    - prefix/mask (if this node also represents a route)

  Each LEAF stores:
    - full prefix
    - prefix length
    - nexthop information

  Key property: back-edges (pointing UP) signal we've reached a leaf.
                When bit position stops increasing, we've gone to a leaf.
```

```c
/*
 * PATRICIA TRIE — Full C Implementation
 * Supports IPv4 LPM lookup.
 */

#include 
#include 
#include 
#include <arpa/inet.h>

#define IPV4_MAXBITS    32
#define BIT_TEST(k, b)  ((k) & (1u << (31 - (b))))

struct patricia_node {
    uint32_t            prefix;      /* Network address (host byte order) */
    uint32_t            mask;        /* Subnet mask */
    int                 prefix_len;  /* CIDR prefix length */
    int                 bit;         /* Bit position to test at this node */
    struct patricia_node *left;      /* Bit=0 branch */
    struct patricia_node *right;     /* Bit=1 branch */
    void                *nexthop;    /* Forwarding info (NULL if internal) */
    int                 is_route;    /* 1 if this node holds a route */
};

struct patricia_tree {
    struct patricia_node *root;
    int                   count;
};

/* Convert prefix length to mask */
static uint32_t prefixlen_to_mask(int len)
{
    if (len == 0) return 0;
    if (len == 32) return 0xFFFFFFFF;
    return ~((1u << (32 - len)) - 1);
}

/* Allocate a new trie node */
static struct patricia_node *new_node(uint32_t prefix, int prefix_len,
                                       int bit, void *nexthop)
{
    struct patricia_node *n = calloc(1, sizeof(*n));
    if (!n) return NULL;
    n->prefix     = prefix;
    n->mask       = prefixlen_to_mask(prefix_len);
    n->prefix_len = prefix_len;
    n->bit        = bit;
    n->nexthop    = nexthop;
    n->is_route   = (nexthop != NULL);
    n->left       = n->right = NULL;
    return n;
}

/*
 * Insert a route into the Patricia trie.
 * prefix:      network address (e.g., 10.0.0.0)
 * prefix_len:  CIDR length (e.g., 8)
 * nexthop:     forwarding information
 */
int patricia_insert(struct patricia_tree *tree, uint32_t prefix,
                    int prefix_len, void *nexthop)
{
    uint32_t masked = prefix & prefixlen_to_mask(prefix_len);
    struct patricia_node **pp = &tree->root;
    struct patricia_node *p   = tree->root;
    struct patricia_node *last_internal = NULL;
    int newbit = 0;

    /* Find the first bit where the new prefix differs from what's in the tree */
    while (p) {
        /* If we've gone past where we're looking, stop */
        if (p->bit >= prefix_len || p->is_route) {
            /* Check if prefix matches at current node */
            if (p->is_route && p->prefix == masked &&
                p->prefix_len == prefix_len) {
                /* Exact match — update nexthop */
                p->nexthop  = nexthop;
                p->is_route = 1;
                return 0;
            }
            /* Need to insert */
            break;
        }
        last_internal = p;
        if (BIT_TEST(masked, p->bit))
            pp = &p->right, p = p->right;
        else
            pp = &p->left,  p = p->left;
    }

    /* Allocate new leaf node */
    struct patricia_node *leaf = new_node(masked, prefix_len, prefix_len, nexthop);
    if (!leaf) return -1;

    if (!p) {
        /* Simple append — no conflict */
        *pp = leaf;
    } else {
        /* Find first differing bit between new prefix and existing node */
        uint32_t diff = masked ^ (p->prefix & prefixlen_to_mask(prefix_len));
        for (newbit = 0; newbit < 32; newbit++) {
            if (BIT_TEST(diff, newbit)) break;
        }
        /* Insert internal branching node */
        struct patricia_node *branch = new_node(masked, prefix_len, newbit, NULL);
        if (!branch) { free(leaf); return -1; }
        branch->is_route = 0;
        leaf->bit        = prefix_len;
        if (BIT_TEST(masked, newbit)) {
            branch->right = leaf;
            branch->left  = p;
        } else {
            branch->left  = leaf;
            branch->right = p;
        }
        *pp = branch;
    }
    tree->count++;
    return 0;
}

/*
 * LPM Lookup — finds the most specific (longest prefix) match.
 * Returns the nexthop pointer, or NULL if no match.
 */
void *patricia_lookup(struct patricia_tree *tree, uint32_t dest)
{
    struct patricia_node *node = tree->root;
    void                 *best = NULL;

    while (node) {
        /* Check if this node's prefix matches the destination */
        if (node->is_route) {
            if ((dest & node->mask) == node->prefix) {
                best = node->nexthop;  /* Candidate: longer prefixes may win */
            }
        }

        /* Traverse based on bit test */
        if (node->bit >= IPV4_MAXBITS)
            break;  /* Leaf */

        if (BIT_TEST(dest, node->bit))
            node = node->right;
        else
            node = node->left;
    }

    return best;
}

/* Example usage */
int main(void)
{
    struct patricia_tree tree = {0};
    char *nh_default  = "via 1.1.1.1";
    char *nh_rfc1918a = "via 10.1.1.1";
    char *nh_specific = "via 10.5.3.1";

    /* Insert routes */
    patricia_insert(&tree, 0x00000000, 0,  nh_default);   /* 0.0.0.0/0  */
    patricia_insert(&tree, 0x0A000000, 8,  nh_rfc1918a);  /* 10.0.0.0/8 */
    patricia_insert(&tree, 0x0A050300, 24, nh_specific);  /* 10.5.3.0/24 */

    /* Lookup */
    void *result;
    result = patricia_lookup(&tree, 0x0A050364); /* 10.5.3.100 */
    /* Returns: "via 10.5.3.1" (matches /24, more specific than /8) */

    result = patricia_lookup(&tree, 0x0A0A0A0A); /* 10.10.10.10 */
    /* Returns: "via 10.1.1.1" (only /8 matches) */

    result = patricia_lookup(&tree, 0x08080808); /* 8.8.8.8 */
    /* Returns: "via 1.1.1.1" (only default /0 matches) */

    return 0;
}
```

### 4.3 LC-Trie (Level-Compressed Trie) — Linux Kernel's Actual Algorithm

The Linux kernel (since 2.6.13) uses an LC-Trie (Level-Compressed Trie), also called
"fib_trie". This is a dramatic improvement over Patricia tries for large routing tables.

**Key idea:** Instead of branching on one bit at a time, an LC-Trie node branches on
multiple bits at once. A node with `bits=k` has `2^k` children and skips `pos` bits
before branching. This "level compression" reduces tree height from O(W) to O(log n).

```
LC-TRIE NODE STRUCTURE
========================

  struct key_vector {
      t_key  key;      /* The prefix value (key bits) */
      u8     pos;      /* First bit position this node checks */
      u8     bits;     /* How many bits checked at this level */
      u8     slen;     /* Suffix length (= 32 - prefixlen for leaves) */
      union {
          struct hlist_node  leaf;     /* leaf: pos=0 AND bits=0 */
          struct {
              struct key_vector __rcu *parent;
              struct key_vector __rcu *tnode[0]; /* 2^bits children */
          };
      };
  };

LC-TRIE vs PATRICIA TRIE (for 100K routes)
─────────────────────────────────────────────────────────────
                    Patricia        LC-Trie
  Tree height:      ~32             ~7-10
  Avg cache misses  ~15-20          ~3-5
  Lookups/sec:      ~10M            ~40-100M (on modern hardware)
  Memory:           O(n)            O(n) but less fragmented
─────────────────────────────────────────────────────────────

LOOKUP EXAMPLE: dest = 10.5.3.100 (binary: 00001010.00000101.00000011.01100100)

  Root node: pos=24, bits=8  → extract bits [24..31] = 00001010 = 10
             tnode[10] → Internal node for 10.x.x.x

  Node:      pos=16, bits=8  → extract bits [16..23] = 00000101 = 5
             tnode[5]  → Internal node for 10.5.x.x

  Node:      pos=8,  bits=8  → extract bits [8..15]  = 00000011 = 3
             tnode[3]  → Internal node for 10.5.3.x

  Node:      pos=0,  bits=0  → LEAF: 10.5.3.0/24 ← LPM winner!

  Total: 4 memory accesses (vs ~32 for bit-by-bit Patricia)
```

```c
/*
 * SIMPLIFIED LC-TRIE LOOKUP
 * Mirrors the actual logic in net/ipv4/fib_trie.c:fib_table_lookup()
 *
 * Key insight: extract 'bits' bits from 'dest' starting at position 'pos',
 * use as index into tnode[] array.
 */

/* From include/net/ip_fib.h / net/ipv4/fib_trie.c */
typedef unsigned int t_key;
#define KEYLENGTH  (8 * sizeof(t_key))    /* 32 for IPv4 */

/* Extract 'n' bits from 'key' at position 'pos' (MSB=bit 0) */
static inline unsigned long get_index(t_key key, struct key_vector *kv)
{
    unsigned long index = key ^ kv->key;   /* XOR to get differing bits */
    /* Shift right by (KEYLENGTH - kv->pos - kv->bits) */
    index >>= (KEYLENGTH - kv->pos - kv->bits);
    /* Mask to 'bits' bits */
    index &= (1ul << kv->bits) - 1;
    return index;
}

/* Check if a key matches the prefix stored in a leaf */
static inline bool prefix_matches(const struct key_vector *l,
                                   t_key key, t_key slen)
{
    /* slen = suffix length = (32 - prefixlen) */
    /* mask off the suffix, compare prefix bits */
    return !((l->key ^ key) >> slen);
}

/*
 * Main lookup function (simplified from fib_table_lookup).
 * Returns pointer to fib_alias (route info) or NULL.
 */
struct fib_alias *lctrie_lookup(struct key_vector *root, t_key dest_key)
{
    struct key_vector *n = root;
    struct fib_alias *fa;
    unsigned long index;

    while (1) {
        /* Is this an internal node? (pos|bits > 0) */
        if (n->pos + n->bits > 0) {
            /* Extract the relevant bits and descend */
            index = get_index(dest_key, n);
            /* Bounds check: tnode has 2^bits children */
            if (index >= (1ul << n->bits))
                return NULL;   /* can't happen if tree is consistent */
            /* Descend to child */
            n = rcu_dereference(n->tnode[index]);
            if (!n)
                return NULL;   /* miss: no route here */
            continue;
        }

        /* n->pos == 0 && n->bits == 0: this is a LEAF */
        /* Check the hlist of fib_alias entries */
        hlist_for_each_entry_rcu(fa, &n->leaf, fa_list) {
            /* slen = 32 - prefix_len; smaller slen = more specific */
            if (prefix_matches(n, dest_key, fa->fa_slen))
                return fa;  /* Found! First match = longest prefix */
        }
        return NULL;
    }
}

/*
 * REAL fib_table_lookup iterates over multiple leaves to find
 * the LONGEST match. The simplified version above returns the first.
 * The full version backtracks up the trie checking parent nodes.
 *
 * Key files in Linux kernel source:
 *   net/ipv4/fib_trie.c     — IPv4 FIB trie
 *   net/ipv6/ip6_fib.c      — IPv6 FIB (uses different radix tree)
 *   net/ipv4/fib_frontend.c — Netlink→FIB interface
 *   net/ipv4/route.c        — Route cache and IP output path
 */
```

### 4.4 DIR-24-8 — Hardware and DPDK's Approach

For hardware and DPDK (user-space packet processing), the DIR-24-8 (Directed 24-8)
table is commonly used. It trades memory for extreme speed: 2 guaranteed memory
accesses for any IPv4 LPM lookup.

```
DIR-24-8 LAYOUT
================

  Main table (tbl24): 2^24 = 16,777,216 entries × 4 bytes = 64 MB
  Extended table (tbl8): groups of 256 entries, allocated on demand

  Each entry in tbl24:
  ┌─────────┬─────────────────────────────────────────────────────────┐
  │ bit 31  │ VALID: 1 = valid entry, 0 = no route                   │
  │ bit 30  │ EXT:   0 = direct nexthop,  1 = pointer to tbl8 group │
  │ bits 0-23│ nexthop_index OR tbl8_group_index                      │
  └─────────┴─────────────────────────────────────────────────────────┘

LOOKUP ALGORITHM:
  1. dest = 10.5.3.100 → top 24 bits = 0x0A0503 = 657667
  2. entry = tbl24[657667]
  3. if (entry.ext == 0): DONE, return entry.nexthop  ← /24 or shorter
  4. if (entry.ext == 1):
       group = entry.tbl8_index
       idx8  = dest & 0xFF  (bottom 8 bits = 100)
       entry = tbl8[group * 256 + 100]
       return entry.nexthop  ← /25 through /32

  WORST CASE: 2 memory accesses (for /25-/32 routes)
  BEST CASE:  1 memory access  (for /0-/24 routes, majority of table)

MEMORY COST:
  tbl24: always 64 MB
  tbl8:  256 entries × 4 bytes × number_of_groups
  For a full DFZ table (950K routes), most are /24 → fits in tbl24
  Only a few thousand /25-/32 entries need tbl8 extension
  Total: ~64 MB + ~1 MB = acceptable

  Compare to Patricia trie: ~50 MB but ~15-20 cache misses per lookup
  DIR-24-8: ~65 MB, maximum 2 cache misses per lookup → WINS for speed
```

```c
/*
 * DIR-24-8 Implementation — used in DPDK's rte_lpm library
 */

#include 
#include 
#include 

#define DIR24_8_TBL24_SIZE  (1 << 24)   /* 16M entries */
#define DIR24_8_TBL8_SIZE   256          /* entries per group */
#define DIR24_8_EXT_FLAG    (1 << 30)   /* entry is pointer to tbl8 */
#define DIR24_8_VALID_FLAG  (1 << 31)   /* entry is valid */
#define DIR24_8_NO_ROUTE    0xFFFFFFFF  /* miss sentinel */

struct dir24_8 {
    uint32_t  *tbl24;          /* 64MB main table */
    uint32_t  *tbl8;           /* extended tables */
    uint32_t   tbl8_count;     /* allocated tbl8 groups */
    uint32_t   tbl8_max;       /* maximum tbl8 groups */
};

struct dir24_8 *dir24_8_create(uint32_t max_tbl8_groups)
{
    struct dir24_8 *t = calloc(1, sizeof(*t));
    t->tbl24      = calloc(DIR24_8_TBL24_SIZE, sizeof(uint32_t));
    t->tbl8       = calloc((size_t)max_tbl8_groups * DIR24_8_TBL8_SIZE,
                            sizeof(uint32_t));
    t->tbl8_max   = max_tbl8_groups;
    t->tbl8_count = 0;
    /* Initialize all entries to "no route" */
    memset(t->tbl24, 0xFF, DIR24_8_TBL24_SIZE * sizeof(uint32_t));
    return t;
}

/*
 * Add route: prefix/prefix_len → nexthop_id
 * nexthop_id is an index into a separate nexthop table.
 */
int dir24_8_add(struct dir24_8 *t, uint32_t prefix, uint8_t prefix_len,
                uint32_t nexthop_id)
{
    prefix &= (prefix_len == 0) ? 0 : ~((1u << (32 - prefix_len)) - 1);

    if (prefix_len <= 24) {
        /* Route covers entire /24 blocks: fill all tbl24 entries */
        uint32_t start = prefix >> 8;
        uint32_t end   = start + (1u << (24 - prefix_len));
        uint32_t entry = DIR24_8_VALID_FLAG | nexthop_id;

        for (uint32_t i = start; i < end; i++) {
            /* Only overwrite if not already set to a more specific route */
            /* (In production, need prefix-len ordering for correctness) */
            if (!(t->tbl24[i] & DIR24_8_VALID_FLAG) ||
                (t->tbl24[i] & DIR24_8_EXT_FLAG)) {
                t->tbl24[i] = entry;
            }
        }
    } else {
        /* /25-/32: need tbl8 extension */
        uint32_t idx24 = prefix >> 8;

        /* Allocate a new tbl8 group if needed */
        if (!(t->tbl24[idx24] & DIR24_8_EXT_FLAG)) {
            if (t->tbl8_count >= t->tbl8_max) return -1;  /* OOM */
            uint32_t group = t->tbl8_count++;

            /* Copy existing /24 nexthop to all tbl8 slots (inheritance) */
            uint32_t inherit = t->tbl24[idx24] & ~DIR24_8_VALID_FLAG;
            uint32_t base    = group * DIR24_8_TBL8_SIZE;
            uint32_t ientry  = DIR24_8_VALID_FLAG | inherit;
            for (int i = 0; i < DIR24_8_TBL8_SIZE; i++)
                t->tbl8[base + i] = ientry;

            /* Point tbl24 to this group */
            t->tbl24[idx24] = DIR24_8_VALID_FLAG | DIR24_8_EXT_FLAG | group;
        }

        uint32_t group = t->tbl24[idx24] & ~(DIR24_8_VALID_FLAG | DIR24_8_EXT_FLAG);
        uint32_t base  = group * DIR24_8_TBL8_SIZE;
        uint8_t  start = (uint8_t)(prefix & 0xFF);
        uint8_t  count = (uint8_t)(1u << (32 - prefix_len));
        uint32_t entry = DIR24_8_VALID_FLAG | nexthop_id;

        for (int i = start; i < start + count; i++)
            t->tbl8[base + i] = entry;
    }
    return 0;
}

/*
 * Lookup — the hot path. MUST be inlined by compiler.
 * Returns nexthop_id, or DIR24_8_NO_ROUTE on miss.
 */
static inline uint32_t dir24_8_lookup(const struct dir24_8 *t, uint32_t dest)
{
    /* Access 1: main table indexed by top 24 bits */
    uint32_t entry = t->tbl24[dest >> 8];

    if (__builtin_expect(entry & DIR24_8_EXT_FLAG, 0)) {
        /* Access 2: extended table for /25-/32 */
        uint32_t group = entry & ~(DIR24_8_VALID_FLAG | DIR24_8_EXT_FLAG);
        entry = t->tbl8[group * DIR24_8_TBL8_SIZE + (dest & 0xFF)];
    }

    if (!(entry & DIR24_8_VALID_FLAG))
        return DIR24_8_NO_ROUTE;

    return entry & ~(DIR24_8_VALID_FLAG | DIR24_8_EXT_FLAG);
}
```

### 4.5 IPv6 Routing — struct fib6_node

IPv6 uses a different structure in `net/ipv6/ip6_fib.c`. The 128-bit address space
is too large for dir-24-8. Linux uses a tree of `struct fib6_node`:

```c
/*
 * IPv6 FIB node — from include/net/ip6_fib.h
 * This is a binary radix tree (not LC-compressed like IPv4 FIB)
 */
struct fib6_node {
    struct fib6_node    __rcu *parent;
    struct fib6_node    __rcu *left;
    struct fib6_node    __rcu *right;
#ifdef CONFIG_IPV6_SUBTREES
    struct fib6_node    __rcu *subtree;
#endif
    struct fib6_info    __rcu *leaf;    /* Route entries at this node */
    __u16               fn_bit;        /* bit position for this node */
    __u16               fn_flags;      /* RTN_ROOT, RTN_RTINFO */
    int                 fn_sernum;     /* serial number for cache validation */
    struct fib6_info    __rcu *rr_ptr; /* round-robin pointer for ECMP */
};

/* IPv6 route information */
struct fib6_info {
    struct fib6_table   *fib6_table;
    struct fib6_info    __rcu *fib6_next; /* linked list of routes at same node */
    struct rt6key        fib6_dst;        /* destination prefix */
    u32                  fib6_metric;     /* metric */
    u8                   fib6_protocol;   /* RTPROT_STATIC etc. */
    u8                   fib6_type;       /* RTN_UNICAST etc. */
    u8                   fib6_nh_upper_bound;
    u8                   should_flush:1,
                         dst_nocount:1,
                         dst_nopolicy:1,
                         fib6_destroying:1;
    struct nexthop       *nh;            /* ECMP nexthop group */
    struct fib6_nh       fib6_nh[];      /* per-path nexthop info */
};

/* Route key — destination prefix */
struct rt6key {
    struct in6_addr addr;   /* 128-bit IPv6 address */
    int             plen;   /* prefix length */
};
```

### 4.6 ECMP (Equal-Cost Multi-Path Routing)

When multiple routes have equal cost (ECMP), packets are distributed across paths.
The lookup returns a "nexthop group," and a hash selects which path to use:

```c
/*
 * ECMP nexthop selection — from net/ipv4/nexthop.c
 * After FIB lookup returns a struct nexthop, select one path.
 */

struct nexthop {
    struct rb_node      rb_node;    /* node in per-ns rb_tree of nexthops */
    struct list_head    fi_list;    /* fib_info references */
    u32                 id;         /* nexthop ID */
    u8                  protocol;   /* RTPROT_STATIC etc. */
    u8                  nh_flags;
    bool                is_group;   /* true if ECMP group */
    refcount_t          refcnt;
    struct rcu_head     rcu;
    union {
        struct nh_info       *nh_info;   /* single nexthop */
        struct nh_group      *nh_grp;   /* ECMP group */
    };
};

struct nh_group {
    struct nh_group     *spare;         /* for atomic replace */
    u16                  num_nh;        /* number of paths */
    bool                 is_multipath;
    bool                 hash_threshold; /* hash threshold mode */
    bool                 resilient;      /* resilient hashing mode */
    struct nh_grp_entry  nh_entries[];  /* per-path entries */
};

/*
 * Flow hash for ECMP selection.
 * Uses source IP, dest IP, ports, protocol — ensures same flow
 * always takes same path (for TCP connection consistency).
 */
static u32 flow_hash_from_keys(struct flow_keys *keys)
{
    __be32 addr[9]; /* src+dst addr (v4 or v6) + ports + proto */
    /* ... pack fields into addr[] ... */
    return jhash2((__u32 *)addr, sizeof(addr)/4, jhash_initval);
}

/* Select which nexthop to use */
static struct nh_grp_entry *nexthop_select_path(struct nh_group *nhg,
                                                  int hash)
{
    /* hash_threshold: divide hash space proportionally to weights */
    u32 bucket = (u64)hash * nhg->num_nh >> 32;
    return &nhg->nh_entries[bucket % nhg->num_nh];
}
```

---

## 5. ARP and Neighbor Cache

ARP (Address Resolution Protocol, IPv4) and NDP (Neighbor Discovery Protocol, IPv6)
resolve IP addresses to MAC addresses. Linux stores these in the **neighbor cache**,
a hash table protected by RCU and per-entry spinlocks.

### 5.1 Data Structure

```
NEIGHBOR CACHE HASH TABLE LAYOUT

  struct neigh_hash_table {
      struct neighbour __rcu **hash_buckets;  /* array of bucket heads */
      unsigned int             hash_shift;    /* log2(num_buckets) */
      __u32                    hash_rnd[4];   /* random seeds for jhash */
  };

  hash_buckets array:
  ┌───┬──────────────────────────────────────────────────────────────┐
  │ 0 │→ [192.168.1.1 | MAC: aa:bb:cc:dd:ee:01 | REACHABLE] → NULL │
  │ 1 │→ NULL                                                        │
  │ 2 │→ [10.0.0.1 | MAC: 00:11:22:33:44:55 | STALE]               │
  │   │  → [10.0.0.2 | MAC: 00:11:22:33:44:56 | REACHABLE] → NULL  │
  │ 3 │→ [172.16.0.1 | MAC: fe:dc:ba:98:76:54 | DELAY] → NULL      │
  │...│                                                               │
  └───┴──────────────────────────────────────────────────────────────┘

  Hash function: jhash(ip_address, 4, hash_rnd) & hash_mask
  Collision: chaining (linked list within bucket)

  arp_tbl  (IPv4 ARP)    → neigh_table with .family = AF_INET
  nd_tbl   (IPv6 NDP)    → neigh_table with .family = AF_INET6
```

### 5.2 NUD State Machine

NUD (Neighbor Unreachability Detection) is the state machine for each ARP/NDP entry.
This prevents stale MAC addresses from forwarding to wrong hosts.

```
NUD STATE MACHINE (RFC 4861 extended for Linux)
=================================================

    ┌─────────────────────────────────────────────────────┐
    │                INCOMPLETE                           │
    │  ARP request sent, waiting for reply                │
    │  Packets queued in arp_queue                        │
    └───────────┬──────────────────────────┬─────────────┘
                │ reply received           │ no reply (timer × N)
                ▼                          ▼
    ┌───────────────────┐          ┌───────────────────┐
    │    REACHABLE      │          │      FAILED       │
    │  Valid, confirmed │          │  Give up, purge   │
    │  ~30 sec timeout  │          │  NUD_FAILED state │
    └─────────┬─────────┘          └───────────────────┘
              │ reachable timeout
              ▼
    ┌───────────────────┐   new packet ──────────────────►
    │      STALE        │                                 │
    │  Was valid,       │───────────────────────────────► │
    │  validity unknown │   timer expires (DELAY)         │
    └─────────┬─────────┘                                 ▼
              │ packet to send                   ┌────────────────┐
              ▼                                  │   PROBE        │
    ┌───────────────────┐                        │ Unicast ARP    │
    │      DELAY        │────────────────────────│ probes sent    │
    │  Timer running,   │ delay expires          └────────┬───────┘
    │  waiting for NUD  │                                 │
    └───────────────────┘           reply ◄───────────────┘
                                      │
                              ┌───────▼───────┐
                              │  REACHABLE    │
                              └───────────────┘

  Linux NUD flags (include/net/neighbour.h):
  NUD_INCOMPLETE  = 0x01   NUD_REACHABLE = 0x02
  NUD_STALE       = 0x04   NUD_DELAY     = 0x08
  NUD_PROBE       = 0x10   NUD_FAILED    = 0x20
  NUD_NOARP       = 0x40   NUD_PERMANENT = 0x80
```

```c
/*
 * Neighbor cache lookup — from net/core/neighbour.c
 * Returns a neighbour with refcount incremented, or NULL.
 */
struct neighbour *neigh_lookup(struct neigh_table *tbl,
                                const void *pkey,        /* IP address */
                                struct net_device *dev)
{
    struct neighbour *n;
    struct neigh_hash_table *nht;
    u32 hash_val;

    rcu_read_lock_bh();  /* Read lock: no writer can swap the table */
    nht      = rcu_dereference_bh(tbl->nht);
    hash_val = tbl->hash(pkey, dev, nht->hash_rnd);
    /* Mask to bucket index */
    hash_val = hash_val >> (32 - nht->hash_shift);

    /* Walk the bucket chain */
    for (n = rcu_dereference_bh(nht->hash_buckets[hash_val]);
         n != NULL;
         n = rcu_dereference_bh(n->next)) {
        if (n->dev == dev && tbl->key_eq(n, pkey)) {
            /* Found: increment refcount */
            if (!refcount_inc_not_zero(&n->refcnt))
                n = NULL;  /* being freed by GC */
            break;
        }
    }
    rcu_read_unlock_bh();
    return n;
}

/*
 * Hard-coded hash for IPv4 ARP table.
 * Mixes IP address with random seeds to prevent hash-flooding attacks.
 */
static u32 arp_hash(const void *pkey, const struct net_device *dev,
                     __u32 *hash_rnd)
{
    u32 key = *(__u32 *)pkey;  /* IPv4 address */
    return arp_hashfn(key, dev, hash_rnd);
}

static u32 arp_hashfn(u32 key, const struct net_device *dev, u32 *hash_rnd)
{
    u32 val = key ^ ((u32)(unsigned long)dev);
    return jhash_1word(val, hash_rnd[0]);
}

/*
 * GC (Garbage Collection) for old neighbor entries.
 * Runs via timer, removes FAILED and old STALE entries.
 * threshold1, threshold2, threshold3 from sysctl:
 *   net.ipv4.neigh.default.gc_thresh1 = 128  (don't GC below this)
 *   net.ipv4.neigh.default.gc_thresh2 = 512  (start GC if over)
 *   net.ipv4.neigh.default.gc_thresh3 = 1024 (hard limit)
 */
```

### 5.3 ARP/NDP Acceleration — Hardware Header Cache

When the same neighbor is used repeatedly, the kernel caches the pre-built Ethernet
header (dst MAC + src MAC + ethertype) to avoid rebuilding it per-packet:

```c
/*
 * hh_cache: Hardware Header Cache
 * Stores a pre-built L2 header (14 bytes for Ethernet)
 * so the kernel doesn't call dev->header_ops->create() every packet.
 */
struct hh_cache {
    unsigned int    hh_len;             /* length of cached header */
    seqlock_t       hh_lock;            /* protects hh_data */
    unsigned long   hh_data[HH_DATA_ALIGN(LL_MAX_HEADER)/sizeof(long)];
    /* hh_data contains: dst_mac(6) + src_mac(6) + ethertype(2) = 14 bytes */
};

/* In neigh_output() (fast path), if hh is valid: */
static inline int neigh_hh_output(const struct hh_cache *hh, struct sk_buff *skb)
{
    unsigned int hh_alen;
    unsigned int seq;
    unsigned int hh_len;

    do {
        seq    = read_seqbegin(&hh->hh_lock);
        hh_len = hh->hh_len;
        hh_alen = HH_DATA_ALIGN(hh_len);
        /* Copy header directly into sk_buff's headroom */
        memcpy(skb->data - hh_alen, hh->hh_data, hh_alen);
    } while (read_seqretry(&hh->hh_lock, seq));

    skb_push(skb, hh_len);
    return dev_queue_xmit(skb);
}
/*
 * seqlock: Writer increments sequence before and after update.
 * Reader checks if sequence changed → retry if so.
 * Zero locks for readers if no concurrent write → ideal for pre-built headers.
 */
```

---

## 6. Connection Tracking — The Stateful Firewall Engine

**Connection tracking (conntrack)** maintains state for every active network flow.
This is what makes `iptables -A FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT`
work: the kernel knows a packet belongs to an established connection without inspecting
every rule again.

### 6.1 Why a Hash Table?

Unlike routing (LPM), connection lookup is **exact match**: given the 5-tuple
(src IP, dst IP, src port, dst port, protocol), find the exact connection entry.
A hash table gives O(1) average lookup — perfect.

```
CONNTRACK HASH TABLE
=====================

  nf_conntrack_hash[] — global hash table
  Size: nf_conntrack_htable_size (computed at boot, ~16K-131072 buckets)

  Each bucket: hlist_nulls_head (linked list with null-pointer sentinel)

  ┌─────┬──────────────────────────────────────────────────────────┐
  │  0  │                                                           │
  │     │ nf_conn{src=10.1.1.1:5000, dst=8.8.8.8:53, UDP, ORIG}   │
  │     │ nf_conn{src=8.8.8.8:53, dst=10.1.1.1:5000, UDP, REPLY}  │
  │     │   ↑ same nf_conn, TWO tuples (original + reply direction)│
  ├─────┤                                                           │
  │  1  │ NULL                                                      │
  ├─────┤                                                           │
  │  2  │ nf_conn{src=192.168.1.5:43210, dst=93.184.216.34:443,    │
  │     │         TCP, ESTABLISHED, 300s timeout}                   │
  │     │ nf_conn{src=93.184.216.34:443, dst=192.168.1.5:43210,    │
  │     │         TCP, ESTABLISHED, 300s timeout}                   │
  ├─────┤                                                           │
  │ ... │ ...                                                       │
  └─────┴──────────────────────────────────────────────────────────┘

  Hash key: jhash of (src_ip, dst_ip, src_port, dst_port, proto, zone)
  BOTH directions (original + reply) are inserted separately.
  Both hash_buckets point to the SAME nf_conn struct.
```

### 6.2 Core Data Structures

```c
/*
 * Connection tracking structures — from include/net/netfilter/nf_conntrack.h
 * and include/net/netfilter/nf_conntrack_tuple.h
 */

/* The 5-tuple identifying one direction of a flow */
struct nf_conntrack_tuple {
    struct nf_conntrack_man src;    /* source: address + layer4 id */
    struct {
        union nf_inet_addr  u3;     /* destination IP (v4 or v6) */
        union nf_conntrack_man_proto u;  /* dst port / ICMP id+code */
        u_int8_t            protonum;   /* IPPROTO_TCP, IPPROTO_UDP, etc. */
        u_int8_t            dir;        /* IP_CT_DIR_ORIGINAL or REPLY */
    } dst;
};

/* Source address + layer4 identifier */
struct nf_conntrack_man {
    union nf_inet_addr          u3;  /* IPv4 or IPv6 address */
    union nf_conntrack_man_proto u;  /* TCP/UDP port, ICMP id, etc. */
    u_int16_t                   l3num; /* AF_INET or AF_INET6 */
};

/* One direction in the hash table */
struct nf_conntrack_tuple_hash {
    struct hlist_nulls_node hnnode;  /* hash chain linkage */
    struct nf_conntrack_tuple tuple; /* the 5-tuple */
};

/*
 * The main connection entry.
 * Lives in the hash table indexed by BOTH original AND reply tuples.
 */
struct nf_conn {
    /* Reference count — core struct */
    struct nf_conntrack ct_general;     /* refcnt inside */
    spinlock_t          lock;           /* per-connection spinlock */
    u32                 timeout;        /* expiry, in jiffies */

    /* Hash table entries (forward and reply direction) */
    struct nf_conntrack_tuple_hash tuplehash[IP_CT_DIR_MAX];
    /*  tuplehash[IP_CT_DIR_ORIGINAL]: client→server direction */
    /*  tuplehash[IP_CT_DIR_REPLY]:    server→client direction */

    /* Connection status bits */
    unsigned long status;
    /* IPS_EXPECTED        = bit 0: created by helper/expectation */
    /* IPS_SEEN_REPLY      = bit 1: reply packet seen */
    /* IPS_ASSURED         = bit 2: connection is established */
    /* IPS_CONFIRMED       = bit 3: in hash table */
    /* IPS_SRC_NAT         = bit 4: src NAT applied */
    /* IPS_DST_NAT         = bit 5: dst NAT applied */
    /* IPS_NAT_DONE_MASK   = bits 6,7 */

    /* Protocol-specific state (TCP state, etc.) — variable length */
    u8 __nfct_protoinfo[/* protocol-specific size */0];

    /* NAT info (if CONFIG_NF_NAT enabled) */
    struct nf_conn_nat *nat;

    /* Extension area (timestamps, accounting, labels, etc.) */
    struct nf_ct_ext   *ext;
};

/*
 * TCP state within a conntrack entry.
 * Tracks TCP sequence numbers and state for connection monitoring.
 */
struct ip_ct_tcp {
    struct ip_ct_tcp_state seen[2];  /* info for each direction */
    u_int8_t    state;      /* TCP state (enum tcp_conntrack) */
    u_int8_t    last_dir;   /* direction of last packet */
    u_int8_t    retrans;    /* number of retransmissions seen */
    u_int8_t    last_index; /* INDEX_* of last packet */
    u_int32_t   last_seq;   /* sequence of last in-order segment */
    u_int32_t   last_ack;   /* last value of ack */
    u_int32_t   last_end;   /* last end of last segment */
    u_int16_t   last_win;   /* window of last segment */
    u_int8_t    last_wscale;/* window scale of last packet */
    u_int8_t    last_flags; /* 0 or TCP_CT_FLAG_* */
};

/* TCP conntrack states (not the same as socket TCP states) */
enum tcp_conntrack {
    TCP_CONNTRACK_NONE,         /* no traffic yet */
    TCP_CONNTRACK_SYN_SENT,     /* SYN sent */
    TCP_CONNTRACK_SYN_RECV,     /* SYN/ACK seen */
    TCP_CONNTRACK_ESTABLISHED,  /* ACK seen */
    TCP_CONNTRACK_FIN_WAIT,     /* FIN sent */
    TCP_CONNTRACK_CLOSE_WAIT,   /* FIN received */
    TCP_CONNTRACK_LAST_ACK,     /* FIN/ACK sent */
    TCP_CONNTRACK_TIME_WAIT,    /* Both FINs exchanged */
    TCP_CONNTRACK_CLOSE,        /* RST or simultaneous close */
    TCP_CONNTRACK_LISTEN,       /* not used in conntrack */
    TCP_CONNTRACK_MAX,
    TCP_CONNTRACK_IGNORE,
    TCP_CONNTRACK_RETRANS,
    TCP_CONNTRACK_UNACK,
    TCP_CONNTRACK_TIMEOUT_MAX
};
```

### 6.3 Conntrack Lookup — The Hot Path

```c
/*
 * nf_conntrack_in() — called for every packet entering netfilter.
 * Finds or creates the conntrack entry.
 * From net/netfilter/nf_conntrack_core.c
 */

/* Compute hash from tuple */
static u32 hash_conntrack(const struct net *net,
                           const struct nf_conntrack_tuple *tuple,
                           u32 seed)
{
    unsigned int n;
    u32 a, b;

    /* Hash the IP addresses and ports together */
    get_random_once(&seed, sizeof(seed));
    n = (sizeof(tuple->src) + sizeof(tuple->dst.u3)) / sizeof(u32);
    a = jhash2((u32 *)tuple, n, seed ^ ((u32)tuple->dst.protonum << 16));
    b = jhash2((u32 *)&tuple->dst.u, sizeof(tuple->dst.u)/sizeof(u32),
               seed ^ ((u32)tuple->dst.l3num << 16));
    return reciprocal_scale(jhash_2words(a, b, seed), nf_conntrack_htable_size);
}

/* Fast lookup */
static struct nf_conntrack_tuple_hash *
__nf_conntrack_find_get(struct net *net,
                         const struct nf_conntrack_zone *zone,
                         const struct nf_conntrack_tuple *tuple,
                         u32 hash)
{
    struct nf_conntrack_tuple_hash *h;
    struct hlist_nulls_node *n;
    struct nf_conn *ct;

begin:
    /* RCU read lock — lockless! */
    hlist_nulls_for_each_entry_rcu(h, n,
                                    &nf_conntrack_hash[hash],
                                    hnnode) {
        ct = nf_ct_tuplehash_to_ctrack(h);
        if (nf_ct_key_equal(h, tuple, zone, net)) {
            /* Increment refcount — if it drops to 0 between lookup
             * and incr, another CPU might be freeing it */
            if (unlikely(!refcount_inc_not_zero(&ct->ct_general.use)))
                continue;  /* freed, skip */
            /* Verify it didn't change while we were incrementing */
            if (unlikely(!nf_ct_key_equal(h, tuple, zone, net))) {
                nf_ct_put(ct);
                goto begin;
            }
            return h;
        }
    }
    /* nulls sentinel reached (NULL-like): no match */
    if (get_nulls_value(n) != hash)
        goto begin;  /* concurrent resize, retry */
    return NULL;
}
```

### 6.4 Timeout Management — Timing Wheels

Millions of connections must be expired efficiently. The kernel uses **timing wheels**
(not a simple sorted list): a circular array of slots, each covering a time interval.

```
TIMING WHEEL FOR CONNTRACK TIMEOUTS
=====================================

  timeout_wheel[] — circular buffer of buckets
  Each bucket = list of nf_conn entries expiring in that slot

    jiffies % WHEEL_SIZE  → current bucket

    0    1    2    3    4    ...  255
  ┌────┬────┬────┬────┬────┬───┬────┐
  │    │    │ ←  │ ←  │ ←  │   │    │  NOW = slot 2 (current second)
  │ε1  │ε2  │ε3  │ε4  │    │   │ε5  │
  │ε6  │    │    │    │    │   │    │
  └────┴────┴────┴────┴────┴───┴────┘
             ↑ timer fires here, expire all connections in slot 2

  Each nf_conn has:
      u32 timeout;  /* absolute jiffies when this entry expires */

  Bucket assignment: timeout % WHEEL_SIZE
  GC: walk current bucket, re-insert connections not yet expired

  WHY NOT SORTED LIST?
  - Inserting into sorted list: O(log n) per connection
  - Expiring from timing wheel: O(1) amortized
  - Most connections have similar timeouts (TCP ESTABLISHED = 432000 jiffies)
    → they cluster into nearby buckets → efficient bulk expiry

  Linux conntrack uses call_rcu() for deferred free:
  - Mark entry as dying
  - call_rcu(&ct->rcu, nf_ct_destroy)
  - After RCU grace period, actually free nf_conn
```

---

## 7. Firewall Rule Engines: iptables, nftables, eBPF

### 7.1 iptables — Tables, Chains, Rules as Linked Lists

iptables (deprecated but still widely used) organizes rules into **tables** (filter,
nat, mangle, raw, security), each with **chains** (PREROUTING, INPUT, FORWARD, OUTPUT,
POSTROUTING), and each chain is a **doubly-linked list** of rules traversed linearly.

```
IPTABLES DATA STRUCTURE HIERARCHY
===================================

  Per-table, per-chain: ipt_entry linked list

  filter table:
    INPUT chain:    rule1 → rule2 → rule3 → POLICY(DROP)
    FORWARD chain:  rule1 → rule2 → POLICY(DROP)
    OUTPUT chain:   rule1 → POLICY(ACCEPT)

  ipt_entry (one rule):
  ┌─────────────────────────────────────────────────────────────────┐
  │ struct ipt_ip {                                                 │
  │   u32  src, dst, smsk, dmsk;  /* IP ranges */                  │
  │   char iniface[IFNAMSIZ];     /* input interface */            │
  │   char outiface[IFNAMSIZ];    /* output interface */           │
  │   u8   proto;                 /* IP protocol */                │
  │   u8   flags, invflags;       /* inversion flags */            │
  │ }                                                               │
  │                                                                 │
  │ u16 target_offset;  /* offset to target (jump/verdict) */      │
  │ u16 next_offset;    /* offset to next rule */                  │
  │ u32 comefrom;       /* debug: what chains call this */         │
  │ struct xt_counters { u64 pcnt, bcnt; }  /* per-rule stats */   │
  │                                                                 │
  │ struct xt_entry_match[0]:  /* variable-length match extensions */
  │   struct xt_entry_match { /* -m tcp, -m udp, -m conntrack, ... */
  │     u16 match_size;
  │     char name[30];         /* "tcp", "conntrack", etc. */
  │     struct xt_match *match; /* function pointer to match logic */
  │     u8 data[];              /* match-specific data (port ranges etc.) */
  │   }
  │
  │ struct xt_entry_target:  /* -j ACCEPT, DROP, RETURN, custom target */
  │   struct xt_entry_target {
  │     u16 target_size;
  │     char name[30];
  │     struct xt_target *target;
  │     u8 data[];
  │   }
  └─────────────────────────────────────────────────────────────────┘

TRAVERSAL (ip_tables.c: ipt_do_table()):
  for each rule in chain:
      if rule.ip_matches(skb):       /* check IP header */
          if rule.matches_all(skb):  /* run all xt_entry_match extensions */
              execute target(skb)    /* ACCEPT → return, DROP → drop,
                                       JUMP → enter sub-chain, etc. */

PERFORMANCE PROBLEM:
  O(n) per packet for n rules in a chain.
  With 10,000 rules: 10,000 checks per packet.
  At 1Mpps: 10 billion comparisons/second → catastrophic!

  This is WHY nftables + eBPF exist: they use O(1) lookups.
```

```c
/* Core iptables traversal (simplified from net/ipv4/netfilter/ip_tables.c) */

unsigned int ipt_do_table(void *priv, struct sk_buff *skb,
                           const struct nf_hook_state *state)
{
    struct ipt_table *table = priv;
    struct ipt_entry *e;
    const struct xt_table_info *private = table->private;
    struct xt_action_param acpar;
    unsigned int verdict = NF_DROP;

    /* Walk rules one by one */
    e = get_entry(private->entries, private->hook_entry[state->hook]);

    do {
        struct ipt_entry_target *t;

        /* Match IP header fields */
        if (!ip_packet_match(skb, state->in, state->out,
                              &e->ip, acpar.fragoff)) {
            e = ipt_next_entry(e);
            continue;
        }

        /* Run match extensions (-m tcp, -m conntrack, etc.) */
        if (!xt_ematch_foreach(e, acpar) /* returns false if no match */) {
            e = ipt_next_entry(e);
            continue;
        }

        /* Execute target */
        t = ipt_get_target_c(e);
        verdict = t->u.kernel.target->target(skb, &acpar);

        if (verdict == XT_CONTINUE) {
            e = ipt_next_entry(e);  /* RETURN or fallthrough */
        } else {
            break;  /* Final verdict: ACCEPT, DROP, QUEUE, etc. */
        }
    } while (!ipt_is_last_entry(e, private));

    return verdict;
}
```

### 7.2 nftables — Set Infrastructure (Hash, RBTree, Bitmap, Pipapo)

nftables replaces iptables with a richer, more efficient model. Instead of linear
rule chains, it uses **sets** — efficient data structures for O(1) membership testing.

```
NFTABLES ARCHITECTURE
======================

  nft_rule: like iptables rule, but uses EXPRESSIONS instead of matches
            Each expression evaluates a part of the packet, stores result
            in a register. Final expression is the verdict.

  nft_set:  a COLLECTION of elements with efficient lookup
            The set type is chosen at creation time based on workload:

  SET TYPES:
  ┌─────────────────┬─────────────────────────────────────────────────┐
  │ nft_hash        │ Hash table: O(1) for exact IP/port/MAC lookup    │
  │ (nft_set_hash.c)│ Best for: IP blocklists, port sets              │
  │                 │ Collision: chaining with RCU-safe linked list    │
  ├─────────────────┼─────────────────────────────────────────────────┤
  │ nft_rbtree      │ Red-Black tree: O(log n) for RANGES              │
  │(nft_set_rbtree.c)│ Best for: port ranges (1024-65535), IP ranges  │
  │                 │ Interval sets: each element is [start, end]      │
  ├─────────────────┼─────────────────────────────────────────────────┤
  │ nft_bitmap      │ Bitmap: O(1) for dense small integer sets        │
  │(nft_set_bitmap.c)│ Best for: VLAN IDs (0-4095), small port sets   │
  │                 │ Memory: exact bits for range [0, max_val]        │
  ├─────────────────┼─────────────────────────────────────────────────┤
  │ nft_pipapo      │ Bit-parallel: O(1) multi-dimensional lookup      │
  │(nft_set_pipapo.c)│ Best for: (src_ip, dst_ip, dst_port) combined  │
  │                 │ Uses SIMD (AVX2) for batch bit testing           │
  ├─────────────────┼─────────────────────────────────────────────────┤
  │ nft_lhash (v6.x)│ Linked hash for small sets                       │
  └─────────────────┴─────────────────────────────────────────────────┘

RULESET EXAMPLE (nft syntax):
  table inet firewall {
      set blocklist {
          type ipv4_addr
          flags interval
          elements = { 10.0.0.0/8, 192.168.0.0/16 }
      }                ↑ nft_rbtree: interval support

      chain input {
          ip saddr @blocklist drop    # O(log n) lookup in rbtree
          tcp dport { 22, 80, 443 } accept  # O(1) bitmap lookup
      }
  }
```

```c
/*
 * nftables hash set operations (nft_set_hash.c, simplified)
 * Uses jhash and RCU-protected chaining.
 */

struct nft_hash {
    u32 seed;           /* random hash seed (prevents hash flooding) */
};

struct nft_hash_elem {
    struct hlist_node   node;      /* chain in bucket */
    struct nft_set_ext  ext;       /* key + data (variable length) */
};

/* Lookup in hash set: O(1) average */
static bool nft_hash_lookup(const struct net *net, const struct nft_set *set,
                              const u32 *key, const struct nft_set_ext **ext)
{
    struct nft_hash_elem *he;
    const struct nft_hash *priv = nft_set_priv(set);
    u32 hash;

    hash = nft_jhash(set, priv, key);  /* jhash of the key */
    hash_for_each_possible_rcu(*(priv->buckets), he, node, hash) {
        if (nft_set_elem_active(&he->ext, NFT_GENMASK_ANY) &&
            nft_set_elem_expired(&he->ext) == 0 &&
            !memcmp(nft_set_ext_key(&he->ext), key, set->klen)) {
            *ext = &he->ext;
            return true;  /* FOUND */
        }
    }
    return false;  /* MISS */
}

/*
 * nftables rbtree: stores ranges [start, end] for interval matching
 * e.g., tcp dport 1024-65535, ip saddr 10.0.0.0/8
 */
struct nft_rbtree {
    struct rb_root_cached root;  /* red-black tree root */
    rwlock_t              lock;  /* write lock (read side uses RCU) */
};

struct nft_rbtree_elem {
    struct rb_node  node;    /* rb_tree linkage */
    struct nft_set_ext ext;  /* key + data */
};

/* Lookup: traverse rb_tree, find rightmost node ≤ key */
static bool nft_rbtree_lookup(const struct net *net, const struct nft_set *set,
                                const u32 *key, const struct nft_set_ext **ext)
{
    const struct nft_rbtree *priv = nft_set_priv(set);
    const struct nft_rbtree_elem *rbe, *interval = NULL;
    const struct rb_node *parent;
    int d;

    rcu_read_lock();
    parent = rcu_dereference_raw(priv->root.rb_root.rb_node);

    while (parent != NULL) {
        rbe = rb_entry(parent, struct nft_rbtree_elem, node);
        d = nft_rbtree_cmp(set, rbe, key);

        if (d < 0) {
            parent = rcu_dereference_raw(parent->rb_right);
        } else if (d > 0) {
            parent = rcu_dereference_raw(parent->rb_left);
        } else {
            /* Exact match of start of interval */
            if (!nft_rbtree_elem_is_end(rbe)) {
                *ext = &rbe->ext;
                rcu_read_unlock();
                return true;
            }
            parent = rcu_dereference_raw(parent->rb_left);
        }
    }
    /* Check if we're within an interval that started earlier */
    if (interval) {
        *ext = &interval->ext;
        rcu_read_unlock();
        return true;
    }
    rcu_read_unlock();
    return false;
}
```

### 7.3 Pipapo — The SIMD Bit-Parallel Algorithm

Pipapo (Pipelining Packet Processing) is nftables's most advanced set type,
designed for matching on multiple fields simultaneously using AVX2/NEON SIMD.

```
PIPAPO CONCEPT — Multi-dimensional exact+range matching
=========================================================

  Match (src_ip ∈ {10.0.0.0/8, 192.168.0.0/16}) AND
        (dst_port ∈ {80, 443})

  Pipapo decomposes each field into bit slices.
  For each possible input value, maintains a bitmask of matching rules.

  Bit-parallel evaluation:
  1. For src_ip field: compute bitmask of rules matching src_ip
     Bitmask: 0b...01001 means rules 0 and 3 match src_ip
  2. For dst_port field: compute bitmask of rules matching dst_port
     Bitmask: 0b...01010 means rules 1 and 3 match dst_port
  3. AND the bitmasks: 0b...01000 means only rule 3 matches BOTH fields
  4. If bitmask != 0: match found

  SIMD advantage: process 256 bits (AVX2) at once
  Instead of checking 1 rule at a time: check 256 rules simultaneously
  For a set with 10,000 rules: ~40 AVX2 operations vs 10,000 comparisons

  Files:
    net/netfilter/nft_set_pipapo.c      — generic implementation
    net/netfilter/nft_set_pipapo_avx2.c — AVX2 SIMD fast path
```

---

## 8. Traffic Control and QoS

Traffic control (TC) manages how packets are queued, scheduled, and transmitted.
It uses a hierarchy of **qdiscs** (queueing disciplines), each backed by specific
data structures.

### 8.1 The qdisc Framework

```
QDISC HIERARCHY
================

  Each network device has ONE root qdisc.
  Root qdisc can be hierarchical (HTB, HFSC) or flat (pfifo_fast).

  struct net_device  →  struct Qdisc *qdisc   (root qdisc)
                              │
                        struct Qdisc_ops:
                          .enqueue(skb, sch)  — called on TX
                          .dequeue(sch)       — called by driver
                          .peek(sch)
                          .init(sch, opt)
                          .reset(sch)
                          .destroy(sch)
                          .change(sch, opt)
                          .dump(sch, skb)

  COMMON QDISCS:
  ┌──────────────┬───────────────────────────────────────────────────┐
  │ pfifo_fast   │ Default. 3-band priority FIFO. O(1).             │
  │              │ TOS bits select band 0 (high) or 1 or 2 (low).   │
  ├──────────────┼───────────────────────────────────────────────────┤
  │ tbf          │ Token Bucket Filter. Rate limiting.               │
  │ (Token Bucket)│ Tokens accumulated at rate r, burst up to b.    │
  ├──────────────┼───────────────────────────────────────────────────┤
  │ htb          │ Hierarchical Token Bucket. Classes with rates.    │
  │              │ Red-black tree for active class scheduling.       │
  ├──────────────┼───────────────────────────────────────────────────┤
  │ fq           │ Fair Queuing. Per-flow queues in red-black tree.  │
  │              │ Pacing: sends at exact rate to prevent bursts.    │
  ├──────────────┼───────────────────────────────────────────────────┤
  │ fq_codel     │ FQ + CoDel (Controlled Delay). Fights bufferbloat │
  │              │ Drops/ECN marks packets when queuing delay > 5ms  │
  ├──────────────┼───────────────────────────────────────────────────┤
  │ cake         │ Common Applications Kept Enhanced. FQ+AQM+shaping │
  │              │ Most sophisticated, best for home routers         │
  ├──────────────┼───────────────────────────────────────────────────┤
  │ hfsc         │ Hierarchical Fair Service Curve. Latency+bw goals │
  └──────────────┴───────────────────────────────────────────────────┘
```

### 8.2 HTB (Hierarchical Token Bucket) — Red-Black Tree

HTB is used for complex QoS with multiple traffic classes. It uses a red-black tree
to efficiently find the next class to dequeue from.

```
HTB CLASS HIERARCHY EXAMPLE
=============================

  Root HTB (1Gbps total)
  ├── Class 1:1 (1Gbps ceil)
  │   ├── Class 1:10 (500Mbps rate, 1Gbps ceil) — video
  │   ├── Class 1:20 (300Mbps rate, 1Gbps ceil) — web
  │   └── Class 1:30 (100Mbps rate, 1Gbps ceil) — bulk

  Each class maintains:
    - Token bucket: current tokens available
    - cmode: can_send, may_borrow, cant_send
    - rb_node in priority queue for dequeue ordering

  Red-black tree (htb.row[prio]):
    Ordered by: next_event time (when this class can next send)
    Dequeue: find class with earliest next_event → rb_first()
```

```c
/*
 * HTB class selection via red-black tree (simplified from net/sched/sch_htb.c)
 */
struct htb_class {
    struct Qdisc_class_common  common;  /* class ID + hashtable node */
    struct rb_node             pq_node; /* node in priority queue */
    struct rb_root             un.leaf.rb;  /* children rb_tree */
    unsigned long              pq_key;  /* jiffies: when to next send */

    /* Token buckets */
    struct psched_ratecfg      rate;    /* committed rate */
    struct psched_ratecfg      ceil;    /* ceiling rate */
    s64                        tokens;  /* current tokens (committed) */
    s64                        ctokens; /* current tokens (ceil) */
    s64                        t_c;     /* time of last token update */

    /* Class mode */
    enum htb_cmode             cmode;
    /* HTB_CAN_SEND:  tokens available, can send now */
    /* HTB_MAY_BORROW: need to borrow from parent */
    /* HTB_CANT_SEND: no tokens even with borrowing */
};

/* Dequeue: find earliest ready class */
static struct htb_class *htb_lookup_leaf(struct htb_sched *q, int prio)
{
    struct rb_node *p;
    struct htb_class *cl;

    /* rb_first() in O(log n): find class with smallest pq_key */
    p = rb_first(&q->row[prio]);
    if (!p) return NULL;

    cl = rb_entry(p, struct htb_class, pq_node);
    /* cl->pq_key = jiffies when this class can next dequeue */
    return cl;
}

/* After dequeue, update timing and re-insert into rb_tree */
static void htb_charge_class(struct htb_sched *q, struct htb_class *cl,
                              int level, struct sk_buff *skb)
{
    int bytes = qdisc_pkt_len(skb);
    /* Subtract tokens for bytes sent */
    cl->tokens  -= (s64)bytes;
    cl->ctokens -= (s64)bytes;

    if (cl->tokens <= 0) {
        /* Class used up its committed tokens */
        cl->cmode = HTB_MAY_BORROW;
        /* Calculate when tokens will be replenished */
        u64 next = ktime_get_ns() + psched_l2t_ns(&cl->rate, -cl->tokens);
        /* Re-insert into rb_tree at new priority time */
        rb_erase(&cl->pq_node, &q->row[level]);
        htb_add_to_wait_tree(q, cl, 0);
    }
}
```

### 8.3 FQ (Fair Queuing) — Per-Flow Red-Black Tree

FQ maintains one queue per flow (5-tuple hash). It ensures no single flow can
monopolize bandwidth and enables TCP pacing.

```c
/*
 * Fair Queuing (net/sched/sch_fq.c) — simplified
 * Red-black tree of flows, ordered by "virtual time" (credit)
 */
struct fq_flow {
    struct rb_node  node;          /* in the rb_tree of flows */
    struct sk_buff *head;          /* first packet in this flow's queue */
    struct sk_buff *tail;          /* last packet */
    u32             socket_hash;   /* hash of 5-tuple */
    int             qlen;          /* queue length */
    u64             time_next_packet; /* virtual time: when to next send */
    /* pacing: TCP sends bursts; FQ smooths them to per-packet rate */
};

struct fq_sched_data {
    struct rb_root  new_flows;     /* flows with remaining credit */
    struct rb_root  old_flows;     /* flows that used their quantum */
    struct rb_root  delayed;       /* flows waiting for their send time */
    u64             time_next_delayed_flow; /* earliest delayed flow time */
    u32             quantum;       /* bytes per scheduling round (~MTU) */
    u32             initial_quantum;
    u32             flow_refill_delay; /* refill interval */
    struct fq_flow  internal;      /* high-priority flow for TC use */
};

/* Enqueue: find or create flow, add packet */
static int fq_enqueue(struct sk_buff *skb, struct Qdisc *sch, ...)
{
    struct fq_sched_data *q = qdisc_priv(sch);
    struct fq_flow *f;
    u32 idx;

    /* Hash packet to flow */
    idx = skb_get_hash(skb) & (q->flow_hash_size - 1);
    f   = &q->flow_tab[idx];

    /* Add to flow's tail */
    if (!f->head)
        f->head = f->tail = skb;
    else {
        f->tail->next = skb;
        f->tail = skb;
    }
    f->qlen++;

    /* Insert into rb_tree if not already scheduled */
    if (!RB_EMPTY_NODE(&f->node))
        return NET_XMIT_SUCCESS;

    /* Set virtual send time */
    f->time_next_packet = max(f->time_next_packet, q->time_next_packet);
    fq_flow_set_next_packet(q, f);  /* insert into new_flows or delayed rb_tree */
    return NET_XMIT_SUCCESS;
}
```

---

## 9. Protocol Headers and Packet Metadata

Every protocol layer adds/reads metadata headers. These are defined as C structs
with exact byte layouts. The kernel accesses them via pointer arithmetic on sk_buff.

### 9.1 The sk_buff — Universal Packet Container

```c
/*
 * struct sk_buff — The Linux kernel's packet descriptor.
 * Does NOT store packet data inline; points to separate data buffer.
 * ~224 bytes for the descriptor itself.
 *
 * Memory layout:
 *
 *   skb->head ─────────────────────────────────────────────────────┐
 *                                                                   │
 *   skb->data ─────────────────────────────────────────────────────┤
 *              │ Network header (IP)                               │
 *              │ Transport header (TCP/UDP)                        │
 *              │ Payload                                           │
 *   skb->tail ─────────────────────────────────────────────────────┤
 *              │ (tailroom, available for appending)               │
 *   skb->end  ─────────────────────────────────────────────────────┘
 *
 *   skb->data - skb->head = headroom (for prepending L2/L3 headers)
 *   skb->tail - skb->data = data length (current packet size)
 *   skb->end  - skb->head = total buffer size
 */
struct sk_buff {
    /* Linked list: RX queue, TX queue, socket receive queue */
    union {
        struct {
            struct sk_buff  *next;
            struct sk_buff  *prev;
            /* sk_buff_head or rb_node for TCP OFO queue */
        };
        struct rb_node      rbnode;
        struct list_head    list;
    };

    /* Socket this buffer belongs to */
    struct sock             *sk;

    /* Device and timestamp */
    struct net_device       *dev;
    ktime_t                  tstamp;

    /* Data pointers */
    unsigned char           *head;      /* start of allocated buffer */
    unsigned char           *data;      /* start of current packet data */
    sk_buff_data_t          tail;       /* end of packet data */
    sk_buff_data_t          end;        /* end of allocated buffer */

    /* Header pointers (set by each layer) */
    union {
        struct tcphdr   *th;
        struct udphdr   *uh;
        struct icmphdr  *icmph;
        struct igmphdr  *igmph;
        struct iphdr    *ipiph;
        struct ipv6hdr  *ipv6h;
        unsigned char   *raw;
    } h;                    /* transport header */

    union {
        struct iphdr    *iph;
        struct ipv6hdr  *ipv6h;
        struct arphdr   *arph;
        unsigned char   *raw;
    } nh;                   /* network header */

    union {
        struct ethhdr   *ethernet;
        unsigned char   *raw;
    } mac;                  /* link layer header */

    /* Lengths */
    unsigned int    len;        /* total length (data + fragments) */
    unsigned int    data_len;   /* length of paged (non-linear) data */
    unsigned int    mac_len;    /* length of L2 header */
    unsigned int    hdr_len;    /* header length for copy on write */

    /* Protocol */
    __be16          protocol;   /* L3 protocol: ETH_P_IP, ETH_P_IPV6, etc. */

    /* Flags and metadata */
    __u8            pkt_type:3;      /* PACKET_HOST, PACKET_BROADCAST, etc. */
    __u8            ignore_df:1;     /* ignore Don't Fragment bit */
    __u8            nf_trace:1;      /* netfilter: tracing enabled */
    __u8            ip_summed:2;     /* checksum mode */
    __u8            ooo_okay:1;      /* TCP: out-of-order allowed */
    __u32           mark;            /* fwmark: used by routing, iptables */
    __u32           priority;        /* qdisc priority */
    unsigned int    truesize;        /* memory consumed (data + sk_buff) */
    __u32           hash;            /* flow hash for RSS/FQ */
    union {
        __wsum      csum;            /* checksum (if CHECKSUM_PARTIAL) */
        __u32       csum_offset;
    };
    /* ... and many more fields (conntrack, netfilter, tc, etc.) */
};

/* Access headers from sk_buff */
static inline struct iphdr *ip_hdr(const struct sk_buff *skb) {
    return (struct iphdr *)skb_network_header(skb);
}
static inline struct tcphdr *tcp_hdr(const struct sk_buff *skb) {
    return (struct tcphdr *)skb_transport_header(skb);
}
```

### 9.2 Ethernet Header

```c
/*
 * IEEE 802.3 Ethernet Frame (Linux: include/uapi/linux/if_ether.h)
 *
 *  0                   1                   2                   3
 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 * ┌──────────────────────────────────────────────────────────────────┐
 * │                   Destination MAC Address (48 bits)              │
 * │──────────────────────────────────────────────────────────────────┤
 * │                   Source MAC Address (48 bits)                   │
 * ├──────────────────────────────────────────────────────────────────┤
 * │    EtherType (16 bits)  │  Payload (46–1500 bytes)              │
 * └──────────────────────────────────────────────────────────────────┘
 *
 *  EtherType values:
 *    0x0800 = IPv4
 *    0x0806 = ARP
 *    0x86DD = IPv6
 *    0x8100 = 802.1Q VLAN tagged frame
 *    0x8847 = MPLS unicast
 *    0x8848 = MPLS multicast
 *    0x88CC = LLDP
 *    0x88A8 = 802.1ad (QinQ / double VLAN)
 */
struct ethhdr {
    unsigned char h_dest[ETH_ALEN];    /* 6 bytes: destination MAC */
    unsigned char h_source[ETH_ALEN];  /* 6 bytes: source MAC */
    __be16        h_proto;             /* 2 bytes: EtherType */
} __packed;  /* total: 14 bytes */

#define ETH_ALEN   6    /* bytes per MAC address */
#define ETH_HLEN  14    /* bytes in Ethernet header */
#define ETH_ZLEN  60    /* min packet size (no FCS) */
#define ETH_DATA_LEN 1500  /* max payload (MTU) */
#define ETH_FRAME_LEN 1514 /* max frame (header + payload, no FCS) */
```

### 9.3 IEEE 802.1Q VLAN Tag

```c
/*
 * 802.1Q VLAN Tag — inserted between EtherType and payload
 *
 * Tagged frame:
 *  dst_mac (6) | src_mac (6) | 0x8100 (2) | TCI (2) | inner_proto (2) | payload
 *                              ←── VLAN tag ──────────→
 *
 * TCI (Tag Control Information, 16 bits):
 *   bits 15-13: PCP (Priority Code Point) — 3 bits, 0-7 priority
 *   bit  12:    DEI (Drop Eligible Indicator) — 1 bit
 *   bits 11-0:  VID (VLAN ID) — 12 bits, values 0-4095
 *                 VID 0:    priority tag (no VLAN membership)
 *                 VID 1:    default VLAN (untagged traffic)
 *                 VID 4095: reserved
 */
struct vlan_hdr {
    __be16 h_vlan_TCI;                  /* PCP(3) | DEI(1) | VID(12) */
    __be16 h_vlan_encapsulated_proto;   /* inner EtherType */
} __packed;  /* 4 bytes */

#define VLAN_HLEN           4
#define VLAN_ETH_HLEN      18   /* 14 (Ethernet) + 4 (VLAN) */
#define VLAN_VID_MASK    0x0FFF /* 12-bit VID mask */
#define VLAN_PRIO_MASK   0xE000 /* 3-bit PCP mask */
#define VLAN_PRIO_SHIFT     13

/* Extract VID from TCI */
static inline u16 vlan_tci_to_vid(u16 tci) {
    return tci & VLAN_VID_MASK;
}

/*
 * QinQ (802.1ad) — double VLAN tagging for service providers
 * Outer tag: 0x88A8 (S-VLAN, service provider)
 * Inner tag: 0x8100 (C-VLAN, customer)
 * Allows multiplexing 4096 customer VLANs per service VLAN.
 */
```

### 9.4 IPv4 Header

```c
/*
 * IPv4 Header (RFC 791) — 20 bytes minimum
 *
 *  0                   1                   2                   3
 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 * ┌──────────────────────────────────────────────────────────────────┐
 * │ Ver(4) │ IHL(4) │  DSCP(6) │ ECN(2) │   Total Length (16)      │
 * ├──────────────────────────────────────────────────────────────────┤
 * │         Identification (16)         │Flags(3)│ Frag Offset (13) │
 * ├──────────────────────────────────────────────────────────────────┤
 * │     TTL (8)       │   Protocol (8)  │    Header Checksum (16)   │
 * ├──────────────────────────────────────────────────────────────────┤
 * │                   Source Address (32)                            │
 * ├──────────────────────────────────────────────────────────────────┤
 * │                Destination Address (32)                          │
 * └──────────────────────────────────────────────────────────────────┘
 *   [Options: 0-40 bytes, IHL*4 - 20 bytes]
 */
struct iphdr {
#if defined(__LITTLE_ENDIAN_BITFIELD)
    __u8    ihl:4,      /* Internet Header Length (words, min=5 → 20 bytes) */
            version:4;  /* IP version = 4 */
#elif defined (__BIG_ENDIAN_BITFIELD)
    __u8    version:4,
            ihl:4;
#endif
    __u8    tos;        /* DSCP(6 bits) + ECN(2 bits); was "Type of Service" */
    __be16  tot_len;    /* Total packet length (header + data), max 65535 */
    __be16  id;         /* Identification (for fragmentation reassembly) */
    __be16  frag_off;   /* Flags(3 bits) + Fragment Offset(13 bits) */
                        /* Flags: bit 0=reserved, bit 1=DF, bit 2=MF */
                        /* DF=Don't Fragment, MF=More Fragments */
    __u8    ttl;        /* Time To Live: hop limit, decremented by each router */
    __u8    protocol;   /* L4 protocol: 6=TCP, 17=UDP, 1=ICMP, 89=OSPF, 47=GRE */
    __sum16 check;      /* 1's complement checksum of header */
    __be32  saddr;      /* Source IPv4 address */
    __be32  daddr;      /* Destination IPv4 address */
    /* Options follow if IHL > 5 */
} __packed;

/* Protocol numbers (IPPROTO_* in include/uapi/linux/in.h) */
#define IPPROTO_ICMP     1
#define IPPROTO_IGMP     2
#define IPPROTO_TCP      6
#define IPPROTO_UDP     17
#define IPPROTO_GRE     47
#define IPPROTO_ESP     50
#define IPPROTO_AH      51
#define IPPROTO_ICMPv6  58
#define IPPROTO_OSPF    89
#define IPPROTO_SCTP   132
#define IPPROTO_UDPLITE 136

/* DSCP values (Differentiated Services Code Point, top 6 bits of ToS) */
#define IPTOS_DSCP_AF11 0x28  /* Assured Forwarding 11 */
#define IPTOS_DSCP_AF12 0x30
#define IPTOS_DSCP_EF   0xB8  /* Expedited Forwarding (low latency) */
#define IPTOS_DSCP_CS0  0x00  /* Default / Best Effort */
#define IPTOS_DSCP_CS6  0xC0  /* Internetwork Control */
```

### 9.5 IPv6 Header

```c
/*
 * IPv6 Header (RFC 8200) — FIXED 40 bytes (no options field)
 * Options go in "Extension Headers" with their own Next Header types.
 *
 *  0                   1                   2                   3
 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 * ┌──────────────────────────────────────────────────────────────────┐
 * │ Ver(4) │ Traffic Class(8)  │        Flow Label (20)             │
 * ├──────────────────────────────────────────────────────────────────┤
 * │      Payload Length (16)    │ Next Header (8) │  Hop Limit (8)  │
 * ├──────────────────────────────────────────────────────────────────┤
 * │                                                                  │
 * │                   Source Address (128 bits = 16 bytes)           │
 * │                                                                  │
 * │                                                                  │
 * ├──────────────────────────────────────────────────────────────────┤
 * │                                                                  │
 * │                 Destination Address (128 bits = 16 bytes)        │
 * │                                                                  │
 * │                                                                  │
 * └──────────────────────────────────────────────────────────────────┘
 */
struct ipv6hdr {
#if defined(__LITTLE_ENDIAN_BITFIELD)
    __u8    priority:4,   /* Traffic Class high 4 bits */
            version:4;    /* IP version = 6 */
#elif defined(__BIG_ENDIAN_BITFIELD)
    __u8    version:4,
            priority:4;
#endif
    __u8    flow_lbl[3];  /* Traffic Class low 4 bits + Flow Label 20 bits */
    __be16  payload_len;  /* Payload length (extension headers + data) */
    __u8    nexthdr;      /* Next Header type: 59=none, 6=TCP, 17=UDP, 43=Routing, 0=Hop-by-Hop */
    __u8    hop_limit;    /* Hop limit (= IPv4 TTL) */
    struct in6_addr saddr;   /* Source address (128 bits) */
    struct in6_addr daddr;   /* Destination address (128 bits) */
} __packed;   /* exactly 40 bytes */

/* IPv6 Extension Header types (nexthdr values) */
#define IPPROTO_HOPOPTS    0   /* Hop-by-Hop Options (must be first if present) */
#define IPPROTO_ROUTING   43   /* Routing Header (for segment routing: SRH) */
#define IPPROTO_FRAGMENT  44   /* Fragment Header */
#define IPPROTO_AH        51   /* Authentication Header */
#define IPPROTO_NONE      59   /* No Next Header */
#define IPPROTO_DSTOPTS   60   /* Destination Options */
#define IPPROTO_MH       135   /* Mobility Header */

/* Segment Routing Header (SRv6, RFC 8754) */
struct ipv6_sr_hdr {
    __u8    nexthdr;
    __u8    hdrlen;         /* length in 8-octet units, not counting first 8 */
    __u8    type;           /* = 4 for SRH */
    __u8    segments_left; /* number of remaining segments to visit */
    __u8    first_segment; /* index of last segment (= total_segs - 1) */
    __u8    flags;
    __be16  tag;
    struct in6_addr segments[0]; /* segment list: list of IPv6 addresses */
};
```

### 9.6 TCP Header

```c
/*
 * TCP Header (RFC 793) — 20 bytes minimum
 *
 *  0                   1                   2                   3
 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 * ┌──────────────────────────────────────────────────────────────────┐
 * │              Source Port (16)           │ Destination Port (16) │
 * ├──────────────────────────────────────────────────────────────────┤
 * │                     Sequence Number (32)                         │
 * ├──────────────────────────────────────────────────────────────────┤
 * │                   Acknowledgment Number (32)                     │
 * ├──────────────────────────────────────────────────────────────────┤
 * │Data│  Res  │C│E│U│A│P│R│S│F│           Window Size (16)        │
 * │Off │(4bits)│W│C│R│C│S│S│Y│I│                                   │
 * │(4) │       │R│E│G│K│H│T│N│N│                                   │
 * ├──────────────────────────────────────────────────────────────────┤
 * │            Checksum (16)                │   Urgent Pointer (16) │
 * └──────────────────────────────────────────────────────────────────┘
 *   [Options: 0-40 bytes]  [Padding]
 */
struct tcphdr {
    __be16  source;       /* Source port */
    __be16  dest;         /* Destination port */
    __be32  seq;          /* Sequence number */
    __be32  ack_seq;      /* Acknowledgment number */
#if defined(__LITTLE_ENDIAN_BITFIELD)
    __u16   res1:4,       /* Reserved (must be 0) */
            doff:4,       /* Data offset: header length in 32-bit words */
            fin:1,        /* FIN: no more data from sender */
            syn:1,        /* SYN: synchronize sequence numbers */
            rst:1,        /* RST: reset connection */
            psh:1,        /* PSH: push buffered data to application */
            ack:1,        /* ACK: acknowledgment field valid */
            urg:1,        /* URG: urgent pointer field significant */
            ece:1,        /* ECE: ECN-Echo (RFC 3168) */
            cwr:1;        /* CWR: Congestion Window Reduced (RFC 3168) */
#elif defined(__BIG_ENDIAN_BITFIELD)
    __u16   doff:4,
            res1:4,
            cwr:1,
            ece:1,
            urg:1,
            ack:1,
            psh:1,
            rst:1,
            syn:1,
            fin:1;
#endif
    __be16  window;       /* Receive window size (flow control) */
    __sum16 check;        /* Checksum (pseudo-header + header + data) */
    __be16  urg_ptr;      /* Urgent pointer: offset to end of urgent data */
} __packed;   /* 20 bytes minimum */

/* Common TCP option kinds (after 20-byte fixed header) */
#define TCPOPT_NOP          1   /* No-operation padding */
#define TCPOPT_MAXSEG       2   /* MSS option (4 bytes: kind+len+MSS) */
#define TCPOPT_WINDOW       3   /* Window scale factor (3 bytes) */
#define TCPOPT_SACK_PERM    4   /* SACK permitted (2 bytes) */
#define TCPOPT_SACK         5   /* SACK block list */
#define TCPOPT_TIMESTAMP   8    /* Timestamps (10 bytes: TSval+TSecr) */
#define TCPOPT_FASTOPEN    34   /* TCP Fast Open cookie */
```

### 9.7 UDP, ICMP Headers

```c
/*
 * UDP Header (RFC 768) — 8 bytes fixed
 */
struct udphdr {
    __be16  source;   /* Source port (0 if not used) */
    __be16  dest;     /* Destination port */
    __be16  len;      /* Length of UDP header + data in bytes (min 8) */
    __sum16 check;    /* Checksum (optional in IPv4, mandatory in IPv6) */
} __packed;   /* 8 bytes */

/*
 * ICMPv4 Header (RFC 792) — 8 bytes fixed
 */
struct icmphdr {
    __u8    type;       /* Message type */
    __u8    code;       /* Message code (subtype) */
    __sum16 checksum;   /* Checksum of ICMP header + data */
    union {
        struct {
            __be16  id;       /* Echo identifier (for matching req/reply) */
            __be16  sequence; /* Echo sequence number */
        } echo;
        __be32  gateway;      /* Redirect: gateway address */
        struct {
            __be16  unused;
            __be16  mtu;      /* PTB (Packet Too Big): next hop MTU */
        } frag;
        __u8  reserved[4];
    } un;
} __packed;   /* 8 bytes */

/* ICMP type values */
#define ICMP_ECHOREPLY       0   /* Echo Reply */
#define ICMP_DEST_UNREACH    3   /* Destination Unreachable */
#define ICMP_REDIRECT        5   /* Redirect */
#define ICMP_ECHO            8   /* Echo Request */
#define ICMP_TIME_EXCEEDED  11   /* TTL exceeded */
#define ICMP_PARAMETERPROB  12   /* Parameter Problem */
#define ICMP_TIMESTAMP      13   /* Timestamp Request */
#define ICMP_TIMESTAMPREPLY 14   /* Timestamp Reply */
#define ICMP_INFO_REQUEST   15   /* Information Request */
#define ICMP_INFO_REPLY     16   /* Information Reply */
#define ICMP_ADDRESS        17   /* Address Mask Request */
#define ICMP_ADDRESSREPLY   18   /* Address Mask Reply */

/*
 * ICMPv6 Header (RFC 4443) — same structure, different type values
 */
struct icmp6hdr {
    __u8        icmp6_type;
    __u8        icmp6_code;
    __sum16     icmp6_cksum;
    union {
        __be32  un_data32[1];
        __be16  un_data16[2];
        __u8    un_data8[4];
        struct icmpv6_echo {
            __be16 identifier;
            __be16 sequence;
        } u_echo;
        struct icmpv6_nd_advt {
#if defined(__LITTLE_ENDIAN_BITFIELD)
            __u32   reserved:5,
                    override:1, solicited:1, router:1, reserved2:24;
#elif defined(__BIG_ENDIAN_BITFIELD)
            __u32   router:1, solicited:1, override:1, reserved:29;
#endif
        } u_nd_advt;
        struct icmpv6_nd_ra {
            __u8    hop_limit;
#if defined(__LITTLE_ENDIAN_BITFIELD)
            __u8    reserved:3, router_pref:2, home_agent:1,
                    other:1, managed:1;
#elif defined(__BIG_ENDIAN_BITFIELD)
            __u8    managed:1, other:1, home_agent:1, router_pref:2, reserved:3;
#endif
            __be16  rt_lifetime;
        } u_nd_ra;
    } icmp6_dataun;
} __packed;

/* ICMPv6 type values (used in NDP) */
#define ICMPV6_ECHO_REQUEST        128
#define ICMPV6_ECHO_REPLY          129
#define ICMPV6_MGM_QUERY           130   /* MLD: Multicast Listener Query */
#define ICMPV6_MGM_REPORT          131   /* MLD: Report */
#define ICMPV6_MGM_REDUCTION       132   /* MLD: Done */
#define NDISC_ROUTER_SOLICITATION  133   /* NDP: RS */
#define NDISC_ROUTER_ADVERTISEMENT 134   /* NDP: RA */
#define NDISC_NEIGHBOUR_SOLICITATION 135 /* NDP: NS (= IPv6 ARP request) */
#define NDISC_NEIGHBOUR_ADVERTISEMENT 136 /* NDP: NA (= IPv6 ARP reply) */
#define NDISC_REDIRECT             137   /* NDP: Redirect */
```

### 9.8 MPLS — Label Switched Paths

```c
/*
 * MPLS Label Stack Entry (RFC 3032) — 4 bytes per entry
 * Inserted between Ethernet header and IP header.
 * Multiple labels stacked = MPLS label stack.
 *
 *  0                   1                   2                   3
 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 * ┌──────────────────────────────────────────────────────────────────┐
 * │                Label (20 bits)            │ TC(3) │S(1)│ TTL(8) │
 * └──────────────────────────────────────────────────────────────────┘
 *
 *  Label: 20-bit label value (0-15 reserved):
 *    0 = IPv4 Explicit Null (swap to this = pop and IPv4 forward)
 *    1 = Router Alert
 *    2 = IPv6 Explicit Null
 *    3 = Implicit Null (pop label without adding another)
 *    4-15 = reserved
 *    16-1048575 = user-assigned labels
 *
 *  TC  (Traffic Class, formerly "EXP"): 3 bits, QoS markings
 *  S   (Bottom of Stack): 1 bit, set on innermost label
 *  TTL: 8 bits, decremented at each LSR (Label Switching Router)
 *
 *  MPLS forwarding table (LFIB — Label Forwarding Information Base):
 *  struct mpls_route {
 *    u32 incoming_label;
 *    enum mpls_payload_type rt_payload_type;  /* IPv4, IPv6, ethernet */
 *    u8  rt_nhn;                              /* number of nexthops (ECMP) */
 *    struct mpls_nh rt_nh[0];                 /* nexthop array */
 *  };
 *
 *  MPLS operations:
 *    PUSH:    add label (tunnel ingress)
 *    SWAP:    replace top label (transit LSR)
 *    POP/PHP: remove label, forward on inner label or IP (egress/penultimate hop)
 */
struct mpls_label {
    __be32 entry;   /* packed: label(20) | tc(3) | bos(1) | ttl(8) */
};

#define MPLS_LS_LABEL_MASK       0xFFFFF000
#define MPLS_LS_LABEL_SHIFT      12
#define MPLS_LS_TC_MASK          0x00000E00
#define MPLS_LS_TC_SHIFT          9
#define MPLS_LS_S_MASK           0x00000100
#define MPLS_LS_S_SHIFT           8
#define MPLS_LS_TTL_MASK         0x000000FF
#define MPLS_LS_TTL_SHIFT         0

#define MPLS_LABEL_IMPLICIT_NULL  3   /* penultimate hop pop */
#define MPLS_LABEL_FIRST_UNRESERVED 16

static inline u32 mpls_entry_decode_label(struct mpls_label *ml) {
    return (be32_to_cpu(ml->entry) & MPLS_LS_LABEL_MASK) >> MPLS_LS_LABEL_SHIFT;
}
static inline bool mpls_entry_decode_bos(struct mpls_label *ml) {
    return (be32_to_cpu(ml->entry) & MPLS_LS_S_MASK) != 0;
}
```

### 9.9 GRE, VXLAN, GENEVE — Overlay Tunnels

```c
/*
 * GRE (Generic Routing Encapsulation, RFC 2784/2890)
 * Encapsulates ANY L3 protocol inside IP.
 * Used by: PPTP, NVGRE (virtual networking), WAN tunnels.
 *
 * Outer IP header (tunnel IP):
 * ┌─────────────────────────────────────────────────────────────────┐
 * │ IP: src=tunnel_src, dst=tunnel_dst, protocol=47 (GRE)          │
 * ├─────────────────────────────────────────────────────────────────┤
 * │ GRE header (4-16 bytes):                                        │
 * │   Flags(16): C|Reserved|K|S|...                                 │
 * │   Protocol(16): 0x0800=IPv4, 0x86DD=IPv6, 0x6558=Ethernet(NSH)│
 * │   [Checksum(32) if C=1]                                         │
 * │   [Key(32)       if K=1] ← VNI or tenant ID                    │
 * │   [Sequence(32)  if S=1]                                        │
 * ├─────────────────────────────────────────────────────────────────┤
 * │ Inner packet (e.g., IPv4 + TCP)                                 │
 * └─────────────────────────────────────────────────────────────────┘
 */
struct gre_base_hdr {
    __be16 flags;      /* C(1)|0|K(1)|S(1)|...|Ver(3) */
    __be16 protocol;   /* Encapsulated protocol EtherType */
} __packed;

#define GRE_CSUM    cpu_to_be16(0x8000)  /* Checksum present */
#define GRE_KEY     cpu_to_be16(0x2000)  /* Key present */
#define GRE_SEQ     cpu_to_be16(0x1000)  /* Sequence number present */

/*
 * VXLAN (Virtual eXtensible LAN, RFC 7348)
 * Encapsulates L2 Ethernet frames inside UDP/IP.
 * Standard tunnel for cloud virtual networks.
 * Default port: UDP 4789.
 *
 * Outer Ethernet (14) + Outer IP (20) + UDP (8) + VXLAN (8) + Inner Ethernet + ...
 *
 * VXLAN header (8 bytes):
 *  0                   1                   2                   3
 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 * ┌──────────────────────────────────────────────────────────────────┐
 * │I(1)|Reserved(7)  |           Reserved (24)                      │
 * ├──────────────────────────────────────────────────────────────────┤
 * │              VNI — Virtual Network Identifier (24)| Reserved (8)│
 * └──────────────────────────────────────────────────────────────────┘
 *  I=1: VNI valid (should always be set for standard VXLAN)
 *  VNI: 24-bit tenant ID (16M unique networks per VXLAN domain)
 */
struct vxlanhdr {
    __be32 vx_flags;   /* bit 3 (I flag) = VNI valid, rest reserved */
    __be32 vx_vni;     /* bits 31-8: VNI (24 bits), bits 7-0: reserved */
} __packed;   /* 8 bytes */

#define VXLAN_HF_VNI  cpu_to_be32(0x08000000)  /* VNI present flag */
#define VXLAN_VNI_MASK cpu_to_be32(0xFFFFFF00) /* VNI is top 24 bits */

/* Extract VNI from VXLAN header */
static inline __be32 vxlan_vni(struct vxlanhdr *hdr) {
    return (hdr->vx_vni & VXLAN_VNI_MASK) >> 8;
}

/*
 * GENEVE (Generic Network Virtualization Encapsulation, RFC 8926)
 * Like VXLAN but with TLV options for extensibility.
 * Used by: OVN, Cilium, some cloud providers.
 * Default port: UDP 6081.
 *
 * GENEVE header (variable, minimum 8 bytes):
 *  0                   1                   2                   3
 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 * ┌──────────────────────────────────────────────────────────────────┐
 * │Ver(2)|Opt Len(6)|O(1)|C(1)| Reserved(6)  |   Protocol (16)     │
 * ├──────────────────────────────────────────────────────────────────┤
 * │                     VNI (24 bits)               | Reserved (8)  │
 * ├──────────────────────────────────────────────────────────────────┤
 * │                  TLV Options (variable)                          │
 * └──────────────────────────────────────────────────────────────────┘
 *  Ver: version = 0
 *  Opt Len: length of options in 4-byte units (Opt Len×4 bytes after fixed hdr)
 *  O: OAM packet flag
 *  C: Critical option present
 *  Protocol: inner frame type (0x6558 = Ethernet)
 *  VNI: Virtual Network Identifier
 */
struct genevehdr {
#if defined(__LITTLE_ENDIAN_BITFIELD)
    u8  opt_len:6,   /* option length in 4-byte units */
        ver:2;       /* version = 0 */
    u8  rsvd1:6,
        critical:1,  /* critical option present */
        oam:1;       /* OAM frame */
#elif defined(__BIG_ENDIAN_BITFIELD)
    u8  ver:2, opt_len:6;
    u8  oam:1, critical:1, rsvd1:6;
#endif
    __be16  proto_type;   /* inner EtherType (0x6558 for Ethernet) */
    u8      vni[3];       /* 24-bit VNI */
    u8      reserved;
    struct geneve_opt options[];  /* TLV options */
} __packed;

/* GENEVE TLV option */
struct geneve_opt {
    __be16  opt_class;  /* Option class (who defines this option) */
    u8      type;       /* Option type within class */
    u8      length:5,   /* Option data length in 4-byte units */
            r3:1, r2:1, r1:1;
    u8      opt_data[]; /* Option data */
} __packed;

/*
 * STT (Stateless Transport Tunneling) — used by VMware NSX
 * Encapsulates frames in TCP segments (not a real TCP connection)
 * Port: TCP 7471. Allows hardware TCP offload for large frames.
 *
 * NVGRE (Network Virtualization using GRE) — Microsoft Hyper-V
 * GRE with Key field used as VSID (Virtual Subnet ID, 24 bits).
 *
 * ERSPAN (Encapsulated Remote SPAN) — Cisco port mirroring
 * GRE + ERSPAN header. Copies packets to remote analyzer.
 */
```

### 9.10 BGP and OSPF Control Plane Messages

```c
/*
 * BGP UPDATE message structure (RFC 4271)
 * BGP uses TCP (port 179). Messages include: OPEN, UPDATE, NOTIFICATION, KEEPALIVE.
 *
 * UPDATE message format:
 *
 *  Marker (16 bytes, all 1s)
 *  Length (2 bytes): total message length including marker
 *  Type (1 byte):    2 = UPDATE
 *
 *  Withdrawn Routes Length (2 bytes)
 *  Withdrawn Routes (variable): list of NLRI (prefix/len pairs) to remove
 *    Each NLRI: 1 byte prefix_len + ceil(prefix_len/8) bytes address
 *
 *  Total Path Attribute Length (2 bytes)
 *  Path Attributes (variable): TLV list of route attributes
 *    Each attribute: Flags(1) + Type(1) + Length(1 or 2) + Value(variable)
 *
 *  Network Layer Reachability Information (NLRI, variable):
 *    List of prefixes being advertised
 */

/* BGP common header */
struct bgp_header {
    u8      marker[16];  /* all 0xFF — authentication/sync */
    __be16  length;      /* total length including header */
    u8      type;        /* OPEN=1 UPDATE=2 NOTIFICATION=3 KEEPALIVE=4 */
} __packed;

/* BGP path attribute (TLV) */
struct bgp_attr_header {
    u8      flags;       /* Optional|Transitive|Partial|Extended-length */
    u8      type_code;   /* attribute type (see below) */
    /* length: 1 byte if Extended-length flag=0, else 2 bytes */
} __packed;

/* BGP attribute type codes */
#define BGP_ATTR_ORIGIN          1   /* IGP=0, EGP=1, INCOMPLETE=2 */
#define BGP_ATTR_AS_PATH         2   /* sequence/set of AS numbers */
#define BGP_ATTR_NEXT_HOP        3   /* IPv4 next hop address */
#define BGP_ATTR_MED             4   /* Multi-Exit Discriminator */
#define BGP_ATTR_LOCAL_PREF      5   /* LOCAL_PREF (iBGP only) */
#define BGP_ATTR_ATOMIC_AGGREGATE 6
#define BGP_ATTR_AGGREGATOR      7
#define BGP_ATTR_COMMUNITIES     8   /* RFC 1997 communities (NOADVERTISE etc.) */
#define BGP_ATTR_ORIGINATOR_ID   9   /* route reflector: originator */
#define BGP_ATTR_CLUSTER_LIST   10   /* route reflector: cluster path */
#define BGP_ATTR_MP_REACH_NLRI  14   /* multiprotocol reach (IPv6, VPNv4, etc.) */
#define BGP_ATTR_MP_UNREACH_NLRI 15  /* multiprotocol unreach */
#define BGP_ATTR_EXT_COMMUNITIES 16  /* RFC 4360 extended communities */
#define BGP_ATTR_AS4_PATH       17   /* 4-byte AS path (RFC 4893) */
#define BGP_ATTR_LARGE_COMMUNITY 32  /* RFC 8092 large communities */

/*
 * OSPF LSA (Link State Advertisement, RFC 2328)
 * OSPF uses IP protocol 89. Distributes topology via flooding.
 * Router-LSA, Network-LSA, Summary-LSA, AS-External-LSA, etc.
 */

/* OSPF LSA header (20 bytes, common to all LSA types) */
struct ospf_lsa_header {
    __be16  ls_age;         /* time since origination (seconds, max 3600) */
    u8      options;        /* optional capabilities */
    u8      type;           /* LSA type (1=Router, 2=Network, 3=Summary, etc.) */
    __be32  id;             /* LSA identifier (depends on type) */
    __be32  adv_router;     /* router ID of advertising router */
    __be32  ls_seqnum;      /* sequence number for version control */
    __be16  checksum;       /* Fletcher checksum (excludes ls_age) */
    __be16  length;         /* total LSA length in bytes */
} __packed;

/* Router-LSA body: describes router's links */
struct ospf_router_lsa {
    struct ospf_lsa_header hdr;
    u8     veb_flags;       /* V=virtual-link endpoint, E=AS boundary, B=area border */
    u8     zero;
    __be16 num_links;       /* number of router link descriptions */
    /* followed by num_links * ospf_router_link structs */
} __packed;

struct ospf_router_link {
    __be32  link_id;        /* depends on link type */
    __be32  link_data;      /* depends on link type */
    u8      type;           /* 1=p2p, 2=transit net, 3=stub net, 4=virtual link */
    u8      num_tos;        /* number of TOS metrics (usually 0) */
    __be16  metric;         /* cost metric */
} __packed;
```

---

## 10. eBPF Maps — The Universal In-Kernel KV Store

eBPF (extended Berkeley Packet Filter) programs run in kernel space with a verifier
ensuring safety. They communicate with user-space and between themselves via **BPF maps**
— kernel data structures accessible from both sides.

### 10.1 Map Types Overview

```
BPF MAP TYPES
==============

  Created by: bpf(BPF_MAP_CREATE, &attr, sizeof(attr))
  Accessed from eBPF: bpf_map_lookup_elem(), bpf_map_update_elem(), etc.
  Accessed from user-space: bpf() syscall with fd

  ┌─────────────────────────────┬──────────────────────────────────────────┐
  │ BPF_MAP_TYPE_HASH           │ Hash table. General purpose K→V store.   │
  │                             │ Uses jhash. Chaining. max_entries limit. │
  │                             │ O(1) avg. RCU-protected.                 │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_ARRAY          │ Integer-indexed array.                   │
  │                             │ Key = u32 index. Fixed size at creation. │
  │                             │ Pre-allocated. O(1). No resizing.        │
  │                             │ Used for: per-CPU counters, jump tables  │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_LPM_TRIE       │ Longest Prefix Match trie.               │
  │                             │ Key = {prefixlen, data[]}                │
  │                             │ Used for IP-based ACLs, routing rules     │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_LRU_HASH       │ LRU hash table.                          │
  │                             │ Evicts least-recently-used when full.    │
  │                             │ Used for: conntrack cache, DNS cache     │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_PERCPU_HASH    │ Per-CPU hash table.                      │
  │                             │ Each CPU has its own hash table.         │
  │                             │ Zero cache contention. Good for counters │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_PERCPU_ARRAY   │ Per-CPU array. Ultra-fast per-cpu stats  │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_DEVMAP         │ Device map (if_index → net_device).      │
  │                             │ Used by XDP for redirect to other NIC    │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_CPUMAP         │ CPU map. XDP redirect to specific CPU.   │
  │                             │ Used for RSS-like load balancing in XDP  │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_XSKMAP         │ XSK (AF_XDP socket) map.                │
  │                             │ Used for zero-copy packet handoff to     │
  │                             │ user-space AF_XDP sockets                │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_SOCKMAP        │ Socket map (key → socket).               │
  │                             │ Used for socket-level load balancing     │
  │                             │ bpf_sk_redirect_map() in stream parser   │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_SOCKHASH       │ Socket hash (hash key → socket).         │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_SK_STORAGE     │ Per-socket local storage.                │
  │                             │ Attached to individual socket, auto-free │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_RINGBUF        │ Ring buffer. Producer (eBPF) writes,     │
  │                             │ consumer (user-space) reads. Lock-free.  │
  │                             │ Used for: packet logs, flow events       │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_INODE_STORAGE  │ Per-inode storage.                       │
  │ BPF_MAP_TYPE_TASK_STORAGE   │ Per-task (process) storage.              │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_PROG_ARRAY     │ Array of eBPF program FDs.               │
  │                             │ Used for tail calls: jump to program     │
  ├─────────────────────────────┼──────────────────────────────────────────┤
  │ BPF_MAP_TYPE_CGROUP_ARRAY   │ Array of cgroup FDs.                     │
  │ BPF_MAP_TYPE_CGROUP_STORAGE │ Per-cgroup storage.                      │
  │ BPF_MAP_TYPE_CGROUP_HASH    │ Per-cgroup hash storage.                 │
  └─────────────────────────────┴──────────────────────────────────────────┘
```

### 10.2 LPM Trie Map — Kernel Implementation

```c
/*
 * BPF LPM Trie — kernel/bpf/lpm_trie.c
 * Used for IP prefix matching in eBPF programs.
 *
 * Key structure: {prefixlen: u32, data: u8[]}
 * For IPv4: data = 4 bytes. For IPv6: data = 16 bytes.
 */

/* Key passed to bpf_map_lookup_elem() for LPM lookup */
struct bpf_lpm_trie_key {
    __u32    prefixlen;   /* prefix length in bits */
    __u8     data[0];     /* prefix data (network byte order) */
};

/* Internal trie node (RCU-protected) */
struct lpm_trie_node {
    struct rcu_head            rcu;
    struct lpm_trie_node __rcu *child[2]; /* child[0]=bit0, child[1]=bit1 */
    u32                        prefixlen; /* prefix length for this node */
    u32                        flags;     /* NODE_TYPE_LEAF | NODE_TYPE_INTERMEDIATE */
    u8                         data[];    /* key data */
};

#define LPM_TREE_NODE_FLAG_IM  BIT(0)  /* intermediate (non-leaf) node */

/* LPM trie map descriptor */
struct lpm_trie {
    struct bpf_map             map;
    struct lpm_trie_node __rcu *root;
    size_t                     n_entries;
    size_t                     max_prefixlen;  /* 32 for IPv4, 128 for IPv6 */
    size_t                     data_size;       /* key size in bytes */
    spinlock_t                 lock;
};

/*
 * Lookup: walk trie bit by bit.
 * At each node, check if this node's prefix matches.
 * Track "last match" as we descend (= longest prefix so far).
 * When we can't descend further: return last match.
 */
static void *trie_lookup_elem(struct bpf_map *map, void *_key)
{
    struct lpm_trie *trie = container_of(map, struct lpm_trie, map);
    struct lpm_trie_node *node, *found = NULL;
    struct bpf_lpm_trie_key *key = _key;

    /* Traverse the trie */
    for (node = rcu_dereference(trie->root); node;) {
        unsigned int next_bit;
        size_t matchlen;

        /* Compute how many bits of this node's prefix match the key */
        matchlen = longest_prefix_match(trie, node, key);

        if (matchlen < node->prefixlen) {
            /* Node's prefix doesn't fully match — no route here */
            break;
        }

        if (!(node->flags & LPM_TREE_NODE_FLAG_IM)) {
            /* This is a real route (leaf) that fully matches */
            found = node;
        }

        if (matchlen == trie->max_prefixlen) {
            /* Hit a /32 (or /128): exact match, can't be more specific */
            break;
        }

        /* Descend to the next bit */
        next_bit = extract_bit(key->data, node->prefixlen);
        node = rcu_dereference(node->child[next_bit]);
    }

    if (!found)
        return NULL;

    /* Return pointer to value (stored after node struct) */
    return found->data + trie->data_size;
}
```

### 10.3 XDP Program with BPF Maps — Complete Example

```c
/*
 * XDP Firewall with LPM blocklist and per-CPU packet counter.
 * Runs at the earliest possible point: right after DMA, before sk_buff allocation.
 * Compiled with: clang -O2 -target bpf -c xdp_firewall.bpf.c
 */

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

/* LPM key for IPv4 (prefix + 4-byte address) */
struct ipv4_lpm_key {
    __u32 prefixlen;
    __u32 ip;
};

/* BPF Maps */

/* IP blocklist: LPM trie for subnet-level blocking */
struct {
    __uint(type, BPF_MAP_TYPE_LPM_TRIE);
    __uint(key_size, sizeof(struct ipv4_lpm_key));
    __uint(value_size, sizeof(__u64));  /* 0 = block */
    __uint(max_entries, 65536);
    __uint(map_flags, BPF_F_NO_PREALLOC);
} blocklist SEC(".maps");

/* Per-CPU counters for dropped and passed packets */
struct pkt_stats {
    __u64 dropped;
    __u64 passed;
};

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(struct pkt_stats));
    __uint(max_entries, 1);
} stats SEC(".maps");

/* Port allowlist: set of allowed destination ports */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(key_size, sizeof(__u16));   /* port in network byte order */
    __uint(value_size, sizeof(__u8));  /* 1 = allowed */
    __uint(max_entries, 64);
} port_allowlist SEC(".maps");

/* Helper: bounds-check pointer arithmetic for verifier */
static inline void *bounds_check(void *ptr, void *end, size_t size)
{
    if (ptr + size > end) return NULL;
    return ptr;
}

/* XDP program entry point */
SEC("xdp")
int xdp_firewall(struct xdp_md *ctx)
{
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    /* --- Layer 2: Parse Ethernet --- */
    struct ethhdr *eth = data;
    if (!bounds_check(eth, data_end, sizeof(*eth)))
        return XDP_DROP;

    /* Only process IPv4 */
    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return XDP_PASS;

    /* --- Layer 3: Parse IPv4 --- */
    struct iphdr *ip = (struct iphdr *)(eth + 1);
    if (!bounds_check(ip, data_end, sizeof(*ip)))
        return XDP_DROP;

    /* Check LPM blocklist for source IP */
    struct ipv4_lpm_key lpm_key = {
        .prefixlen = 32,
        .ip        = ip->saddr,  /* network byte order */
    };
    if (bpf_map_lookup_elem(&blocklist, &lpm_key)) {
        /* Source IP (or its subnet) is blocked */
        goto drop;
    }

    /* --- Layer 4: Parse TCP (if applicable) --- */
    if (ip->protocol == IPPROTO_TCP) {
        /* Compute header offset (IHL field × 4 bytes) */
        __u32 ihl = (ip->ihl & 0x0F) * 4;
        if (ihl < 20) goto drop;  /* sanity check */

        struct tcphdr *tcp = (struct tcphdr *)((void *)ip + ihl);
        if (!bounds_check(tcp, data_end, sizeof(*tcp)))
            goto drop;

        /* Check port allowlist */
        __u16 dst_port = tcp->dest;  /* already in network byte order */
        if (!bpf_map_lookup_elem(&port_allowlist, &dst_port)) {
            /* Port not in allowlist — drop */
            goto drop;
        }
    }

    /* Packet passed all checks */
    {
        __u32 key = 0;
        struct pkt_stats *s = bpf_map_lookup_elem(&stats, &key);
        if (s) __sync_fetch_and_add(&s->passed, 1);
    }
    return XDP_PASS;

drop:
    {
        __u32 key = 0;
        struct pkt_stats *s = bpf_map_lookup_elem(&stats, &key);
        if (s) __sync_fetch_and_add(&s->dropped, 1);
    }
    return XDP_DROP;
}

char _license[] SEC("license") = "GPL";
```

```c
/*
 * User-space loader for the XDP firewall (C).
 * Loads the eBPF program, configures maps, attaches to interface.
 */
#include <bpf/libbpf.h>
#include <bpf/bpf.h>
#include <net/if.h>
#include 
#include <arpa/inet.h>

struct ipv4_lpm_key {
    __u32 prefixlen;
    __u32 ip;
};

int main(int argc, char **argv)
{
    struct bpf_object *obj;
    struct bpf_program *prog;
    int xdp_fd, blocklist_fd, port_fd;
    unsigned int ifindex;

    /* Load compiled eBPF object */
    obj = bpf_object__open("xdp_firewall.bpf.o");
    bpf_object__load(obj);

    /* Get program fd */
    prog     = bpf_object__find_program_by_name(obj, "xdp_firewall");
    xdp_fd   = bpf_program__fd(prog);

    /* Get map fds */
    blocklist_fd = bpf_object__find_map_fd_by_name(obj, "blocklist");
    port_fd      = bpf_object__find_map_fd_by_name(obj, "port_allowlist");

    /* Add blocked subnets to LPM trie */
    struct ipv4_lpm_key key = { .prefixlen = 8, .ip = htonl(0x0A000000) }; /* 10.0.0.0/8 */
    __u64 val = 0;
    bpf_map_update_elem(blocklist_fd, &key, &val, BPF_ANY);

    key.prefixlen = 32;
    key.ip = inet_addr("192.0.2.1");  /* block single host */
    bpf_map_update_elem(blocklist_fd, &key, &val, BPF_ANY);

    /* Add allowed ports */
    __u16 port = htons(443);  __u8 one = 1;
    bpf_map_update_elem(port_fd, &port, &one, BPF_ANY);
    port = htons(80);
    bpf_map_update_elem(port_fd, &port, &one, BPF_ANY);

    /* Attach XDP program to network interface */
    ifindex = if_nametoindex("eth0");
    bpf_xdp_attach(ifindex, xdp_fd, XDP_FLAGS_DRV_MODE, NULL);
    /* XDP_FLAGS_DRV_MODE: run in driver context (before sk_buff allocation) */
    /* XDP_FLAGS_SKB_MODE: run later, after sk_buff creation (generic mode) */

    printf("XDP firewall attached to eth0\n");
    /* Block forever (real program would have event loop) */
    pause();
    return 0;
}
```

---

## 11. Hardware: CAM, TCAM, ASICs, P4

### 11.1 CAM (Content-Addressable Memory)

Regular RAM: you give an ADDRESS, get DATA.
CAM: you give DATA, get ADDRESS (or "found/not-found" + index). O(1) lookup in hardware.

```
REGULAR RAM vs CAM
===================

  Regular RAM (register-addressed memory):
    Input:  address (0x0A05)
    Output: data stored at that address (0xDEAD)
    Time:   O(1) — one cycle

  CAM (Content-Addressable Memory):
    Input:  data to search (MAC: 00:11:22:33:44:55)
    Output: index/address where that data is stored, or "miss"
    Time:   O(1) — one cycle (ALL entries compared in PARALLEL)

  HOW CAM WORKS:
  ┌───────────────────────────────────────────────────────────────┐
  │  CAM Array (example: 8 entries)                               │
  │                                                               │
  │  Entry  │  Stored Data           │  Match  │  Result         │
  │  ──────────────────────────────────────────────────────────── │
  │    0    │  00:11:22:33:44:55     │   YES ◄─┐                │
  │    1    │  aa:bb:cc:dd:ee:ff     │   NO    │                 │
  │    2    │  00:00:00:00:00:01     │   NO    │ PRIORITY ENCODER│
  │    3    │  ff:ff:ff:ff:ff:ff     │   NO    │ → returns index │
  │    4    │  00:11:22:33:44:55 (dup│   YES   │   0 (highest    │
  │   ...   │  ...                   │   ...   │   priority)     │
  │                                               ─────────────── │
  │  Compare wire: one per bit, all entries connected to output   │
  │  ALL comparisons happen in ONE clock cycle                    │
  └───────────────────────────────────────────────────────────────┘

  USAGE: MAC address tables in Ethernet switches
  - Key: destination MAC address (6 bytes)
  - Value: output port number
  - Miss: flood to all ports (unknown unicast)
  - Size: 16K–512K entries in enterprise switches
```

### 11.2 TCAM (Ternary Content-Addressable Memory)

TCAM extends CAM with a third state per bit: "don't care" (X). This enables wildcard
matching — perfect for routing tables and ACLs.

```
TCAM INTERNALS
===============

  Each TCAM entry stores TWO arrays:
    Data bits (D):  0 or 1
    Care bits (C):  1 = must match, 0 = don't care

  Comparison: match iff (input XOR D) AND C == 0 for all bits

  Example (8-bit simplified):
  Entry   Data       Care       Meaning                Matches
  ─────────────────────────────────────────────────────────────────
    0:    10001010   11111111   Exact: 10.x.x.x start  10.0.0.0 only
    1:    10000000   11111111   10.0.0.0/8 exact byte   10.0.0.0 only
    2:    10000000   11000000   10.0.0.0/2             128-191.x.x.x
    3:    00000000   00000000   Default (all don't care) ANY

  Lookup for 10000101 (= 10.5.0.x):
    Entry 0: (10000101 XOR 10001010) AND 11111111 = 00001111 ≠ 0  MISS
    Entry 1: (10000101 XOR 10000000) AND 11111111 = 00000101 ≠ 0  MISS
    Entry 2: (10000101 XOR 10000000) AND 11000000 = 00000000 = 0  MATCH!
    Entry 3: always MATCH (all X)

  ALL comparisons happen SIMULTANEOUSLY.
  Priority encoder selects first (highest priority) match.

TCAM CHARACTERISTICS:
  Speed:    O(1) — 1 clock cycle lookup
  Entries:  128K–4M typical in data center switches
  Power:    ~10W per 1M entries (much more than DRAM)
  Memory:   2× the bits of CAM (data + care arrays)
  Cost:     very expensive (price/entry >> DRAM)

TCAM ORGANIZATION IN ROUTERS:
  ┌──────────────────────────────────────────────────────────┐
  │  Line Card                                               │
  │                                                          │
  │  RX NIC → Parser → TCAM lookup → Action RAM → TX NIC    │
  │              ↑                       ↑                   │
  │         extracts 5-tuple         nexthop/action          │
  │                                                          │
  │  TCAM banks:                                             │
  │    Bank 0: /32 routes (host routes)                      │
  │    Bank 1: /24-/31 routes                                │
  │    Bank 2: /16-/23 routes                                │
  │    Bank 3: /0-/15 routes (coarse)                        │
  │    Bank 4: ACL rules                                     │
  │    Bank 5: QoS classification                            │
  │                                                          │
  │  Priority: Bank 0 first (most specific), cascade down   │
  └──────────────────────────────────────────────────────────┘
```

### 11.3 P4 — Programming Protocol-Independent Packet Processors

P4 is a domain-specific language for programming packet parsers and match-action tables
in ASICs, FPGAs, and SmartNICs. It makes TCAM/hash table structure explicit.

```
P4 PROGRAM STRUCTURE
======================

  Headers → Parser → Match-Action Pipeline → Deparser → Egress

  P4 defines:
  1. Header types (protocol field layouts)
  2. Parser: state machine for packet parsing
  3. Tables: TCAM/hash with actions
  4. Control blocks: if/else, table.apply()

  Example P4 (simplified):

  header ipv4_t {
      bit<4>   version;
      bit<4>   ihl;
      bit<8>   diffserv;
      bit<16>  total_len;
      bit<16>  identification;
      bit<3>   flags;
      bit<13>  frag_offset;
      bit<8>   ttl;
      bit<8>   protocol;
      bit<16>  hdr_checksum;
      bit<32>  src_addr;
      bit<32>  dst_addr;
  }

  /* LPM routing table → implemented as TCAM or LC-Trie in hardware */
  table ipv4_lpm {
      key = {
          hdr.ipv4.dst_addr: lpm;   /* longest-prefix match */
      }
      actions = {
          ipv4_forward;   /* set egress port + dst MAC */
          drop;
          NoAction;
      }
      size = 1024;
      default_action = drop();
  }

  /* ACL table → implemented as TCAM */
  table acl {
      key = {
          hdr.ipv4.src_addr: ternary;  /* wildcards allowed */
          hdr.ipv4.dst_addr: ternary;
          hdr.tcp.dst_port:  ternary;
          hdr.ipv4.protocol: ternary;
      }
      actions = { allow; deny; }
      size = 10000;
  }
```

### 11.4 SmartNIC / DPU Data Structures

Modern SmartNICs (NVIDIA BlueField, AMD Pensando/DSC, Intel IPU/E2000) have ARM
cores that run an operating system and can process packets before they reach the host.

```
SMARTNIC/DPU ARCHITECTURE
===========================

  ┌────────────────────────────────────────────────────────────┐
  │  NVIDIA BlueField-3 DPU                                    │
  │                                                            │
  │  ┌────────────┐    ┌──────────────────────────────────┐   │
  │  │ Network I/F│    │  On-chip ARM cores (16× Cortex-A78)│  │
  │  │ 2× 400GbE  │    │  Runs: Ubuntu / custom RTOS       │  │
  │  │ or 4× 200GbE    │  eBPF programs, OvS, Cilium       │  │
  │  └──────┬─────┘    └──────────────────┬───────────────┘   │
  │         │                             │                    │
  │  ┌──────▼─────────────────────────────▼───────────────┐   │
  │  │             Data Path (P4-programmable ASIC)        │   │
  │  │  Parser → Match-Action tables → Rewrite → Egress   │   │
  │  │                                                     │   │
  │  │  Hardware tables:                                   │   │
  │  │    - Exact-match hash (VXLAN VNI → port)           │   │
  │  │    - LPM trie (IP routing)                         │   │
  │  │    - TCAM (ACL)                                    │   │
  │  │    - Connection tracking offload                   │   │
  │  │    - Crypto offload (IPsec, TLS)                   │   │
  │  └────────────────────────────────────────────────────┘   │
  │         │                             │                    │
  │  ┌──────▼────────┐           ┌────────▼────────────┐      │
  │  │ PCIe Gen5 → │           │  DDR5 Memory (32GB)  │      │
  │  │  Host CPU   │           │  BPF maps, flow tables│      │
  │  └─────────────┘           └─────────────────────-─┘      │
  └────────────────────────────────────────────────────────────┘

  Key point: flow tables (hash maps) and BPF maps live in the DPU's memory,
  not the host. Packet processing happens WITHOUT touching the host CPU.
```

---

## 12. Cloud Networking Data Structures

### 12.1 OVS (Open vSwitch) — Megaflow Cache

OVS provides virtual switching for hypervisors (KVM, Xen) and containers.
Its key innovation is the **megaflow cache**: collapse many exact-match rules
into one wildcard rule, reducing per-packet computation.

```
OVS FLOW TABLE ARCHITECTURE
=============================

  Packet arrives from VM/container via vport
            ↓
  ┌─────────────────────────────────────────────────────────────────┐
  │  EMC (Exact Match Cache) — hash table                          │
  │  Per-CPU, ~8192 entries                                         │
  │  Key: full 5-tuple (exact), Value: actions (output port, etc.) │
  │  Hit: ~10ns. Miss rate <5% for steady traffic.                 │
  └───────────────────────┬─────────────────────────────────────────┘
                          │ miss
  ┌───────────────────────▼─────────────────────────────────────────┐
  │  SMC (Statistical Match Cache) — hash table                     │
  │  Larger than EMC, coarser key matching                          │
  └───────────────────────┬─────────────────────────────────────────┘
                          │ miss
  ┌───────────────────────▼─────────────────────────────────────────┐
  │  DPCLS (Datapath Classifier) — Megaflow cache                  │
  │  Subtable per unique set of wildcards.                          │
  │  Each subtable: hash table of wildcard flows.                   │
  │                                                                 │
  │  Example subtables:                                             │
  │    Subtable A: wildcards on dst_port only                       │
  │      Key template: (*, *, *, dst_port=443, *)                   │
  │      Entry: {dst_port=443} → HTTPS processing action           │
  │    Subtable B: wildcards on src_ip only                         │
  │      Key template: (src_ip=10.0.0.0/8, *, *, *, *)             │
  │      Entry: {src_ip=10.x.x.x} → route to vnet gateway          │
  │                                                                 │
  │  Each megaflow collapses thousands of OpenFlow rules into ONE.  │
  └───────────────────────┬─────────────────────────────────────────┘
                          │ miss
  ┌───────────────────────▼─────────────────────────────────────────┐
  │  OpenFlow Tables (user-space ovs-vswitchd)                     │
  │  Full rule pipeline. Slow path. Installs megaflow in DPCLS.    │
  └─────────────────────────────────────────────────────────────────┘

  MEGAFLOW KEY STRUCTURE (struct miniflow):
  Compressed: only stores non-zero fields.
  Avoids hashing zero fields for performance.
  Uses a bitmap to indicate which fields are present.
```

### 12.2 DPDK — User-Space Packet Processing

DPDK (Data Plane Development Kit) bypasses the kernel entirely, accessing NICs
directly from user space using `vfio` (IOMMU-protected direct DMA).

```
DPDK ARCHITECTURE
==================

  ┌─────────────────────────────────────────────────────────────────┐
  │  User Space Application (DPDK)                                  │
  │                                                                 │
  │  ┌───────────────────────────────────────────────────────────┐  │
  │  │  rte_hash (cuckoo hash): O(1) flow table               │  │
  │  │  rte_lpm:  dir-24-8 route lookup                        │  │
  │  │  rte_acl:  bitmap-based multi-field ACL                  │  │
  │  │  rte_ring: lock-free multi-producer/consumer ring buffer │  │
  │  │  rte_mempool: cache-aligned, NUMA-aware object pool      │  │
  │  └───────────────────────────────────────────────────────────┘  │
  │                                                                 │
  │  PMD (Poll Mode Driver) — DPDK's NIC driver in user-space      │
  │  Polls RX queues in a tight loop (no interrupts!)              │
  └──────────────────┬──────────────────────────────────────────────┘
                     │ UIO/VFIO (direct MMIO access)
  ┌──────────────────▼──────────────────────────────────────────────┐
  │  NIC Hardware (Intel XL710, Mellanox ConnectX, etc.)            │
  │  DMA directly into huge pages (2MB or 1GB)                      │
  │  No kernel involvement in packet I/O                            │
  └─────────────────────────────────────────────────────────────────┘
```

```c
/*
 * DPDK rte_hash (cuckoo hashing) — rte_hash.c
 * Cuckoo hashing: O(1) worst-case lookup (unlike chaining).
 * Two hash functions, two tables. If collision: kick existing entry.
 */

/* Create hash table */
struct rte_hash_parameters params = {
    .name       = "flow_table",
    .entries    = 1 << 20,              /* 1M entries */
    .key_len    = sizeof(struct rte_ipv4_tuple), /* 5-tuple */
    .hash_func  = rte_jhash,
    .socket_id  = rte_socket_id(),     /* NUMA-local */
    .extra_flag = RTE_HASH_EXTRA_FLAGS_RW_CONCURRENCY_LF, /* lock-free */
};
struct rte_hash *flow_table = rte_hash_create(&params);

/* 5-tuple flow key */
struct rte_ipv4_tuple {
    uint32_t src_addr;
    uint32_t dst_addr;
    union {
        struct { uint16_t dport; uint16_t sport; };
        uint32_t ports;
    };
    uint8_t  proto;
    uint8_t  pad[3];
} __rte_packed;

/* Lookup a flow entry */
struct flow_data *data;
int ret = rte_hash_lookup_data(flow_table, &tuple, (void **)&data);
if (ret >= 0) {
    /* Cache hit: use flow_data->action */
} else {
    /* New flow: classify and insert */
    data = rte_zmalloc(NULL, sizeof(*data), 0);
    classify_new_flow(&tuple, data);
    rte_hash_add_key_data(flow_table, &tuple, data);
}

/*
 * rte_acl — multi-field ACL using SIMD (Hyperscan-like algorithm)
 * Decomposes each field into bit-parallel arrays.
 * Processes 4 or 8 packets at once using AVX2/SSE4.
 *
 * Equivalent to nftables pipapo but in user-space.
 */
struct rte_acl_rule my_rules[] = {
    /* Rule: block src=10.0.0.0/8 */
    { .data = {.priority=1, .category_mask=1},
      .field[0] = {.value.u8 = IPPROTO_TCP, .mask_range.u8 = 0xFF},
      .field[1] = {.value.u32 = 0x0A000000, .mask_range.u32 = 8},  /* /8 */
      ...
    },
};
```

### 12.3 VPC Routing and Overlay Networks

In cloud environments (AWS VPC, Azure VNet, GCP VPC), every virtual machine thinks
it has a dedicated network. The underlying substrate uses overlays (VXLAN/GENEVE)
to multiplex millions of tenant flows.

```
AWS VPC ROUTING — SIMPLIFIED DATA STRUCTURES
=============================================

  Physical hosts run a hypervisor (Xen/KVM/Nitro).
  Each host has a software switch (VPC router component).

  FLOW TABLE (per-host, in DRAM):
  ┌────────────────────────────────────────────────────────────────┐
  │ Hash table: {src_vm_ip, dst_vm_ip} → {outer_dst_ip, VNI}      │
  │                                                                │
  │  Key: tenant IP (10.0.1.5 → 10.0.2.7)                        │
  │  Value: encap action:                                          │
  │           outer.dst_ip = physical host IP of 10.0.2.7's host  │
  │           VNI = VPC's network ID (24-bit)                      │
  │           action = VXLAN encap                                 │
  │                                                                │
  │  Populated by: control plane (mapping service)                 │
  │                → pushes host-to-VM mappings to all hosts       │
  └────────────────────────────────────────────────────────────────┘

  Security Group Rules:
  ┌────────────────────────────────────────────────────────────────┐
  │ Per-ENI (Elastic Network Interface) BPF maps:                  │
  │   Ingress: {src_sg_id, dst_port, proto} → ALLOW/DENY          │
  │   Egress:  {dst_sg_id, dst_port, proto} → ALLOW/DENY          │
  │                                                                │
  │  Implemented as: BPF hash maps or nftables sets               │
  │  Updated by: security group daemon (pushes changes from        │
  │              API to BPF map via bpf() syscall)                │
  └────────────────────────────────────────────────────────────────┘

  INTERNET GATEWAY (stateful NAT):
  ┌────────────────────────────────────────────────────────────────┐
  │  NAT table: {private_ip:port} ↔ {public_ip:port}              │
  │  Exact-match hash table (same as conntrack)                    │
  │  Implemented in software on dedicated gateway instances        │
  └────────────────────────────────────────────────────────────────┘
```

### 12.4 Cilium — eBPF-Native Kubernetes Networking

Cilium replaces kube-proxy and iptables with eBPF maps, replacing O(n) iptables
traversal with O(1) BPF hash lookups.

```c
/*
 * Cilium uses BPF maps for:
 *  - Service load balancing (replaces kube-proxy/iptables NAT)
 *  - Pod-to-pod routing
 *  - Network policies
 *  - Egress NAT
 *
 * Example: Service load balancing map
 *   Classic kube-proxy: iptables NAT rules, O(n) per packet
 *   Cilium:             BPF hash map, O(1) per packet
 */

/* Service key (VIP + port + protocol) */
struct lb4_key {
    __u32  address;     /* VIP IPv4 address */
    __u16  dport;       /* destination port */
    __u16  backend_slot; /* 0 = master entry, 1-N = backend slots */
    __u8   proto;       /* IPPROTO_TCP, IPPROTO_UDP */
    __u8   scope;       /* LB_LOOKUP_SCOPE_EXT or INT */
    __u8   pad[2];
};

/* Service value (backend or count) */
struct lb4_service {
    union {
        __u32 backend_id;    /* slot > 0: points to backend */
        __u32 backend_count; /* slot == 0: number of backends */
    };
    __u16 rev_nat_index;    /* reverse NAT table index */
    __u8  flags;
    __u8  flags2;
    __u8  pad[3];
};

/* Backend entry (real pod IP + port) */
struct lb4_backend {
    __u32  address;   /* Pod IP */
    __u16  port;      /* Pod port */
    __u8   proto;
    __u8   state;     /* active, terminating, quarantined */
    __u8   flags;
};

/*
 * Lookup sequence for a packet to VIP 10.96.0.1:80:
 *
 * 1. lookup lb4_services[{vip=10.96.0.1, port=80, slot=0}]
 *    → backend_count = 3 (three pods)
 *
 * 2. slot = (jhash(flow_key) % backend_count) + 1 = e.g., 2
 *
 * 3. lookup lb4_services[{vip=10.96.0.1, port=80, slot=2}]
 *    → backend_id = 42
 *
 * 4. lookup lb4_backends[42]
 *    → {address=10.0.1.15, port=8080}
 *
 * 5. DNAT packet: dst_ip = 10.0.1.15, dst_port = 8080
 *    Record in NAT table for reverse translation.
 *
 * Total: 3 BPF hash map lookups = 3 × ~100ns = ~300ns
 * Compare: iptables with 10,000 rules = ~10ms worst case
 */
```

---

## 13. Concurrency Without Databases: RCU, Per-CPU, Seqlocks

The kernel data structures must be safely accessed by multiple CPU cores simultaneously
without traditional database locks. The solutions are elegant and specific to the
read-mostly, write-rarely nature of routing/firewall data.

### 13.1 RCU (Read-Copy-Update) — The Core Mechanism

RCU is the most important concurrency primitive in the Linux kernel.

**Insight**: routing tables are read millions of times per second but written (updated)
perhaps once per second. Traditional locks penalize readers. RCU makes readers FREE.

```
RCU MECHANISM
==============

  Invariant: readers never block. Writers copy, update, then wait.

  Timeline:

  Reader 1:  [rcu_read_lock]────────[use old data]────[rcu_read_unlock]
  Reader 2:  [rcu_read_lock]────────────────[use old data]────[rcu_read_unlock]
  Writer:              [copy old]─[modify copy]─[rcu_assign_pointer]──[synchronize_rcu]──[free old]
                                                                        ↑
                                              waits for ALL pre-existing readers to finish

  KEY OPERATIONS:

  For READERS (extremely cheap — often just disabling preemption):
    rcu_read_lock()          — mark start of RCU read-side critical section
    rcu_dereference(ptr)     — safely load a pointer (compiler + memory barrier)
    rcu_read_unlock()        — mark end

  For WRITERS (slightly more expensive):
    old_ptr = rcu_dereference(global_ptr)
    new_ptr = kmalloc(...)       — allocate new copy
    *new_ptr = *old_ptr          — copy old data
    modify(new_ptr)              — apply changes to copy
    rcu_assign_pointer(global_ptr, new_ptr)  — atomically publish new version
    synchronize_rcu()            — wait for all old readers to finish
    kfree(old_ptr)               — now safe to free old version

  WHY IS READER FREE?
    - No spinlock: no atomic operations, no cache line bouncing
    - rcu_read_lock() in PREEMPT_NONE kernels = barrier (compiler fence only)
    - RCU "grace period" = time until all CPUs have been through a context switch
    - After grace period: guaranteed no reader is in old critical section

  COST:
    Reader: ~1-3 ns (just a compiler barrier)
    Writer: ~1-10 μs (synchronize_rcu can be expensive, use call_rcu() for async)
```

```c
/*
 * RCU Usage in Linux routing table (simplified)
 */

/* Protected pointer to routing table */
struct fib_table *fib_main __rcu;

/* READER: packet forwarding (called millions of times/sec) */
int ip_route_input(struct sk_buff *skb, ...)
{
    struct fib_table *tb;

    rcu_read_lock();   /* ← mark RCU read side; cheap */

    tb = rcu_dereference(fib_main);  /* safe pointer load */
    if (tb)
        ret = fib_table_lookup(tb, &fl4, &res, flags);

    rcu_read_unlock();  /* ← end of read side */
    return ret;
}

/* WRITER: adding a new route (called rarely, via Netlink) */
int fib_table_insert(struct net *net, struct fib_table *tb,
                     struct fib_config *cfg, ...)
{
    struct key_vector *tp;
    struct fib_alias *new_fa;

    /* Allocate new route entry */
    new_fa = kmem_cache_alloc(fn_alias_kmem, GFP_KERNEL);
    if (!new_fa) return -ENOMEM;

    /* Modify the trie — protected by RTNL mutex (writer lock) */
    ASSERT_RTNL();

    /* ... find/create trie node ... */

    /* Publish new entry: atomic pointer write */
    hlist_add_head_rcu(&new_fa->fa_list, fa_head);

    /* Notify listeners */
    rtmsg_fib(RTM_NEWROUTE, ...);
    return 0;
}

/* DEFERRED FREE with call_rcu() */
static void fib_release_info(struct fib_info *fi)
{
    spin_lock_bh(&fib_info_lock);
    /* ... unlink from lists ... */
    spin_unlock_bh(&fib_info_lock);
    /* Schedule free after RCU grace period */
    call_rcu(&fi->rcu, free_fib_info_rcu);
}

static void free_fib_info_rcu(struct rcu_head *head)
{
    struct fib_info *fi = container_of(head, struct fib_info, rcu);
    kfree(fi);  /* called after all readers have finished */
}
```

### 13.2 seqlock — For Fast-Changing Small Data

seqlocks are used when: data changes frequently (unlike RCU), but is small and
readers can tolerate retrying. Used for jiffies, hardware clock.

```c
/*
 * seqlock: writer increments sequence (odd = in progress, even = stable)
 * Reader checks sequence before and after read; retries if changed.
 *
 * Used in: hh_cache (pre-built Ethernet headers), routing timestamps
 */
seqlock_t lock;
u64 data;

/* Writer */
write_seqlock(&lock);   /* seq becomes odd */
data = new_value;       /* write data */
write_sequnlock(&lock); /* seq becomes even */

/* Reader — no blocking, but may retry */
unsigned int seq;
u64 local;
do {
    seq = read_seqbegin(&lock);  /* read current sequence */
    local = data;                /* read data */
} while (read_seqretry(&lock, seq));  /* retry if seq changed (odd or different even) */
/* local is now consistent */
```

### 13.3 Hazard Pointers — Lock-Free Deferred Reclaim

Used in DPDK and some BPF paths for truly lock-free reclaim:

```c
/*
 * Hazard pointers: each thread "declares" which pointer it's using.
 * Reclaimer checks all hazard pointers before freeing.
 *
 * Example: DPDK rte_rcu_qsbr (Quiescent State-Based Reclamation)
 * Thread declares it's safe: rte_rcu_qsbr_quiescent()
 * Reclaimer waits for all threads to pass through quiescent state.
 */

/* DPDK per-core flow table update */
struct rte_rcu_qsbr *rcu_handle;

/* Reader thread: periodically reports quiescent state */
void worker_thread(void) {
    while (running) {
        process_packets();
        rte_rcu_qsbr_quiescent(rcu_handle, core_id);  
Claude couldn't finish this response. Try again in a moment.

    }
}
```