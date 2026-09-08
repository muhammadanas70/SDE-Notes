# Netfilter / nftables — Complete In-Depth Guide

> **Scope**: Linux kernel packet-processing architecture, netfilter subsystem internals,
> nftables design and bytecode VM, connection tracking, NAT, flowtables, nfnetlink protocol,
> C and Rust implementations, production configurations, security threat model, testing.
>
> **Audience**: Senior security/systems engineers who need a first-principles mental model.

---

## Table of Contents

1. [Summary](#1-summary)
2. [Linux Network Stack — Packet Path Before Netfilter](#2-linux-network-stack--packet-path-before-netfilter)
3. [Netfilter Architecture — Deep Dive](#3-netfilter-architecture--deep-dive)
   - 3.1 Hook Points and Families
   - 3.2 Hook Priority System
   - 3.3 `sk_buff` — The Kernel's Packet Container
   - 3.4 Hook Registration and Dispatch Mechanics
   - 3.5 Verdict System
4. [nftables Architecture](#4-nftables-architecture)
   - 4.1 Design Goals and Differences from iptables
   - 4.2 Object Hierarchy: Table → Chain → Rule → Expression
   - 4.3 nftables Bytecode VM
   - 4.4 Expressions Catalogue
   - 4.5 Sets, Maps, and Verdict Maps
   - 4.6 Flowtable (Software Fast-Path)
5. [Kernel Implementation Internals](#5-kernel-implementation-internals)
   - 5.1 Core Data Structures
   - 5.2 RCU, Transactions, and Atomic Commit
   - 5.3 nfnetlink Protocol
   - 5.4 `nf_tables` Module Layout
6. [Connection Tracking (conntrack)](#6-connection-tracking-conntrack)
   - 6.1 Architecture and Hash Table
   - 6.2 State Machine
   - 6.3 ALG / CT Helpers
   - 6.4 NAT Integration
   - 6.5 Conntrack Zones
7. [NAT Subsystem](#7-nat-subsystem)
   - 7.1 SNAT / DNAT / Masquerade / Redirect
   - 7.2 NAT and Conntrack Interaction
   - 7.3 Full-Cone vs Symmetric NAT
8. [iptables vs nftables — Architecture Comparison](#8-iptables-vs-nftables--architecture-comparison)
9. [nft CLI — Syntax and Rule Language](#9-nft-cli--syntax-and-rule-language)
   - 9.1 Tables and Chains
   - 9.2 Rules and Expressions
   - 9.3 Sets, Maps, Verdict Maps
   - 9.4 Flowtables
   - 9.5 Limits, Quotas, Counters, Meters
   - 9.6 Stateful Objects
10. [C Implementations](#10-c-implementations)
    - 10.1 Kernel Module with Direct nf_hook_ops
    - 10.2 Userspace with libmnl + libnftnl
    - 10.3 Userspace with libnftables
11. [Rust Implementations](#11-rust-implementations)
    - 11.1 Netlink from Scratch with neli
    - 11.2 High-Level nftables-rs
    - 11.3 nft Rule Builder Pattern
12. [Security Threat Model and Mitigations](#12-security-threat-model-and-mitigations)
13. [Production Configurations](#13-production-configurations)
    - 13.1 Host Hardening Ruleset
    - 13.2 Kubernetes / Container Node Firewall
    - 13.3 NAT Gateway / Edge Node
    - 13.4 Securing the nftables Control Plane
14. [Testing, Fuzzing, and Benchmarking](#14-testing-fuzzing-and-benchmarking)
15. [Roll-out and Rollback Plan](#15-roll-out-and-rollback-plan)
16. [References](#16-references)

---

## 1. Summary

Netfilter is the in-kernel packet-interception framework introduced in Linux 2.4 (1998).
It exposes hook points at every significant stage of the IPv4/IPv6/ARP/bridge packet path.
Subsystems register callbacks on those hooks; the kernel dispatches packets through all
registered callbacks in priority order.

**nftables** (merged in 3.13, production-ready ~4.0+) is the modern successor to iptables/
ip6tables/arptables/ebtables. Instead of hard-coded match/target modules, it defines a
minimal register-based **bytecode virtual machine** evaluated per-packet in the kernel.
Userspace compiles high-level rules into that bytecode via the **nfnetlink** Netlink
protocol. This model gives:

- A **single, unified tool** (`nft`) across all address families.
- **Atomic rule-set transactions** — no partial state mid-commit.
- **Native sets and maps** with hash, rbtree, and interval backends — O(1) or O(log n) lookups.
- **Zero rule-duplication**: one set lookup replaces 10 000 iptables `-m set --match-set` rules.
- Extensibility without recompiling the kernel (new expressions as modules).

Everything downstream — Kubernetes kube-proxy (nftables mode), Cilium eBPF, firewalld,
nftables.service — builds on these primitives.

---

## 2. Linux Network Stack — Packet Path Before Netfilter

Understanding where netfilter hooks sit requires understanding the full packet journey.

### 2.1 Ingress (RX) Path

```
NIC hardware
    │  DMA → ring buffer (sk_buff allocated by driver)
    ▼
Driver NAPI poll  (net/core/dev.c: __netif_receive_skb)
    │  XDP hook (if eBPF XDP program attached — runs BEFORE skb allocation)
    ▼
Generic Receive Offload (GRO) / LRO merging
    │
    ▼
netif_receive_skb()
    │  tc ingress (Traffic Control / eBPF qdisc)
    ▼
__netif_receive_skb_core()
    │  Deliver to packet taps (AF_PACKET sockets, tcpdump)
    ▼
L3 protocol handler dispatch  (ip_rcv / ipv6_rcv / etc.)
    │
    ▼
[NETFILTER HOOK: NF_INET_PRE_ROUTING]  ◄──── first hook
    │
    ▼
Routing decision: ip_route_input()
    │
    ├─── local delivery ──► [NF_INET_LOCAL_IN] ──► transport layer (TCP/UDP/ICMP)
    │
    └─── forward ──────────► [NF_INET_FORWARD] ──► [NF_INET_POST_ROUTING] ──► TX path
```

### 2.2 Egress (TX) Path

```
Socket sendmsg()
    │  L4 (TCP/UDP): build segment, checksum offload flags
    ▼
ip_output() / ip6_output()
    │
    ▼
[NETFILTER HOOK: NF_INET_LOCAL_OUT]
    │
    ▼
Routing: ip_route_output()
    │
    ▼
[NETFILTER HOOK: NF_INET_POST_ROUTING]
    │
    ▼
ip_finish_output()
    │  GSO (Generic Segmentation Offload) fragmentation
    ▼
dev_queue_xmit()
    │  tc egress qdisc (HTB, FQ, eBPF)
    ▼
Driver TX ring buffer → NIC DMA → wire
```

### 2.3 Bridging Path

```
NIC RX → bridge port
    │
    ▼
br_handle_frame()
    │
    ├── [NF_BR_PRE_ROUTING]
    ├── L2 forwarding decision (FDB lookup)
    ├── [NF_BR_FORWARD]
    ├── [NF_BR_LOCAL_IN / NF_BR_LOCAL_OUT]
    └── [NF_BR_POST_ROUTING]
```

Bridging hooks matter for container networking: veth pairs in Docker/k8s bridge traffic
through the bridge hooks before it ever reaches the IP stack.

---

## 3. Netfilter Architecture — Deep Dive

### 3.1 Hook Points and Families

Netfilter defines **protocol families** (NFPROTO_*) and within each family a set of
**hook numbers**:

```
NFPROTO_IPV4   / NFPROTO_IPV6  (unified as NFPROTO_INET in nftables)
  NF_INET_PRE_ROUTING     = 0   (after L2, before routing decision)
  NF_INET_LOCAL_IN        = 1   (routed to local process)
  NF_INET_FORWARD         = 2   (routed to another interface)
  NF_INET_LOCAL_OUT       = 3   (from local process, before routing)
  NF_INET_POST_ROUTING    = 4   (after routing, just before TX)

NFPROTO_ARP
  NF_ARP_IN   = 0
  NF_ARP_OUT  = 1
  NF_ARP_FORWARD = 2

NFPROTO_BRIDGE
  NF_BR_PRE_ROUTING  = 0
  NF_BR_LOCAL_IN     = 1
  NF_BR_FORWARD      = 2
  NF_BR_LOCAL_OUT    = 3
  NF_BR_POST_ROUTING = 4

NFPROTO_NETDEV   (Linux 4.2+)
  NF_NETDEV_INGRESS  = 0   (very early, before routing/bridging)
  NF_NETDEV_EGRESS   = 1   (Linux 5.16+, after routing)
```

**NFPROTO_INET** is a pseudo-family in nftables that fires on both IPv4 and IPv6 at the
same hook point, simplifying dual-stack rulesets.

**NF_NETDEV** ingress/egress are the closest hooks to the NIC, ideal for high-performance
DDoS filtering without the overhead of reaching the routing code.

### 3.2 Hook Priority System

Multiple subsystems register callbacks on the same hook. The **priority** integer determines
call order within a hook — **lower numerical value = earlier execution**.

```
Priority  Constant                   Who uses it
────────  ─────────────────────────  ──────────────────────────────────────────
-450      NF_IP_PRI_RAW_BEFORE_DEFRAG  iptables raw (NOTRACK) before defrag
-400      NF_IP_PRI_CONNTRACK_DEFRAG   IP defragmentation reassembly
-300      NF_IP_PRI_RAW               iptables raw table / nftables raw chain
-225      NF_IP_PRI_SELINUX_FIRST      SELinux inbound packet labelling
-200      NF_IP_PRI_CONNTRACK          Connection tracking (ct state init)
-150      NF_IP_PRI_MANGLE             iptables mangle / nftables mangle chain
-100      NF_IP_PRI_NAT_DST            DNAT (destination NAT, pre-routing only)
   0      NF_IP_PRI_FILTER             Main firewall filter (default)
  50      NF_IP_PRI_SECURITY           SELinux outbound / AppArmor
 100      NF_IP_PRI_NAT_SRC            SNAT (source NAT, post-routing only)
 225      NF_IP_PRI_SELINUX_LAST       SELinux final checks
 300      NF_IP_PRI_CONNTRACK_HELPER   CT helper assignment
 INT_MAX  NF_IP_PRI_LAST               Always runs last (confirm/accounting)
```

**Why priority matters for security**: If you place a firewall rule at priority 0 (filter)
but connection tracking is at -200, the conntrack state is already established before your
rule sees the packet. A rule `ct state established accept` is only meaningful because
conntrack already ran. Conversely, placing `notrack` rules at -300 (raw) skips the
conntrack machinery entirely — useful for high-throughput flows.

### 3.3 `sk_buff` — The Kernel's Packet Container

Every packet in the kernel is represented as a `struct sk_buff` (socket buffer). This is
the most important data structure in Linux networking.

```c
/* Simplified, key fields only — actual struct is ~220 fields in net/skbuff.h */
struct sk_buff {
    /* Linked list pointers */
    struct sk_buff      *next, *prev;
    struct sk_buff_head *list;

    /* Timestamps, device, mark */
    ktime_t             tstamp;
    struct net_device   *dev;        /* receiving / sending device */
    __u32               mark;        /* SO_MARK / nfmark */
    __u32               priority;    /* qdisc priority */

    /* Routing / conntrack metadata */
    struct dst_entry    *_skb_refdst; /* routing destination cache */
    struct nf_conntrack *nfct;        /* pointer to conntrack entry */

    /* Transport/Network headers (pointers into data buffer) */
    union {
        struct tcphdr   *th;
        struct udphdr   *uh;
        struct icmphdr  *icmph;
        unsigned char   *raw;
    } h;  /* transport header */

    union {
        struct iphdr    *iph;
        struct ipv6hdr  *ipv6h;
        struct arphdr   *arph;
        unsigned char   *raw;
    } nh;  /* network header */

    union {
        struct ethhdr   *ethernet;
        unsigned char   *raw;
    } mac;  /* mac header */

    /* Data buffer layout:
     *
     *   head                               end
     *    │                                  │
     *    ▼                                  ▼
     *   ┌──────────────────────────────────────┐
     *   │ headroom │ data ... ... ... │tailroom │
     *   └──────────────────────────────────────┘
     *              ▲                 ▲
     *              data              tail
     *
     * len = tail - data  (current payload length)
     * data_len = amount in frags (for scatter-gather / zerocopy)
     */
    unsigned char       *head, *data, *tail, *end;
    unsigned int        len;        /* total length */
    unsigned int        data_len;   /* paged data length */
    __u16               mac_len;    /* MAC header length */
    __u16               hdr_len;    /* writable header length */

    /* Checksum fields */
    __wsum              csum;
    __u8                ip_summed;  /* CHECKSUM_NONE/UNNECESSARY/COMPLETE/PARTIAL */

    /* GSO (Generic Segmentation Offload) */
    __u16               gso_size;
    __u16               gso_segs;
    __u16               gso_type;   /* SKB_GSO_TCPV4/TCPV6/UDP_L4 etc. */

    /* Cloning / fragmentation */
    atomic_t            users;
    /* ... frags[], frag_list for non-linear skbs ... */
};
```

**Key insight**: Netfilter hooks receive a `struct sk_buff **skb` pointer-to-pointer because
hooks may need to *replace* the skb (e.g., after defragmentation reassembles fragments into
a new, larger skb).

**nfmark** (`skb->mark`): A 32-bit metadata field that persists through the kernel stack.
nftables can `meta mark set` to stamp a packet, then policy routing (`ip rule fwmark`)
picks it up. This is the mechanism behind Cilium's datapath and many VPN implementations.

### 3.4 Hook Registration and Dispatch Mechanics

```c
/* include/linux/netfilter.h */
struct nf_hook_ops {
    nf_hookfn           *hook;      /* callback function */
    struct net_device   *dev;       /* for NETDEV family, NULL otherwise */
    void                *priv;      /* private data passed to hook */
    u8                  pf;         /* protocol family: NFPROTO_* */
    enum nf_hook_ops_type hook_thresh; /* NF_HOOK_OP_UNDEFINED/NF_HOOK_OP_NF_TABLES */
    unsigned int        hooknum;    /* hook point: NF_INET_PRE_ROUTING etc. */
    int                 priority;   /* execution order within hook */
    struct list_head    list;       /* internal linkage */
};

/* Callback signature */
typedef unsigned int nf_hookfn(void *priv,
                                struct sk_buff *skb,
                                const struct nf_hook_state *state);
```

```c
/* include/linux/netfilter.h */
struct nf_hook_state {
    unsigned int        hook;       /* which hook point */
    u_int8_t            pf;         /* protocol family */
    struct net_device   *in;        /* ingress interface */
    struct net_device   *out;       /* egress interface */
    struct sock         *sk;        /* associated socket (may be NULL) */
    struct net          *net;       /* network namespace */
    int                 (*okfn)(struct net *, struct sock *, struct sk_buff *);
};
```

**Dispatch path** (`net/netfilter/core.c`):

```c
/* Simplified kernel dispatch loop */
unsigned int nf_hook_slow(struct sk_buff *skb, struct nf_hook_state *state,
                           const struct nf_hook_entries *e, unsigned int s)
{
    unsigned int verdict, i;

    for (i = s; i < e->num_hook_entries; i++) {
        verdict = e->hooks[i].hook(e->hooks[i].priv, skb, state);
        switch (verdict & NF_VERDICT_MASK) {
        case NF_ACCEPT:
            break;                   /* continue to next hook */
        case NF_DROP:
            kfree_skb(skb);
            return NF_DROP;          /* packet destroyed */
        case NF_QUEUE:
            return nf_queue(skb, state, i, verdict); /* to userspace via NFQUEUE */
        case NF_STOLEN:
            return NF_STOLEN;        /* hook consumed the packet (no free) */
        case NF_REPEAT:
            i--;                     /* re-run this hook entry */
            break;
        default:
            WARN_ON(1);
        }
    }
    return 1;  /* NF_ACCEPT, continue in stack */
}
```

`nf_hook_entries` is a **sorted array** (not a list) — sorted by priority at registration
time via `nf_hook_entries_insert_raw()`. This is a cache-friendly design: iteration is a
simple array walk.

### 3.5 Verdict System

Every hook callback returns a **verdict** (u32):

```
NF_DROP    = 0   Discard packet, free sk_buff
NF_ACCEPT  = 1   Continue normal processing
NF_STOLEN  = 2   Hook took ownership, no free (used by IP defrag)
NF_QUEUE   = 3   Queue to userspace (NFQUEUE subsystem)
NF_REPEAT  = 4   Repeat the hook (rare, IP options processing)
NF_STOP    = 5   Deprecated, treated as ACCEPT
```

Verdicts can carry metadata in the upper bits:
- For NF_QUEUE: `verdict = NF_QUEUE | (queue_num << 16)`
- For NF_DROP: ICMP unreachable type in upper 16 bits

**NFQUEUE** (netfilter queue) is the mechanism for userspace packet inspection tools
(Suricata inline IPS mode, nflog, custom firewalls). The kernel pauses the packet in a
queue, userspace daemon makes an accept/drop/modify/requeue decision. Latency cost is
one context switch per packet minimum. Libnetfilter_queue provides the userspace API.

---

## 4. nftables Architecture

### 4.1 Design Goals and Differences from iptables

iptables was designed in 2001 for a world of static rulesets evaluated linearly. Its
problems at scale:

| Problem | iptables | nftables |
|---------|----------|----------|
| Large IP blocklists | 50 000 `-s` rules = O(n) per packet | One set lookup = O(1) hash or O(log n) rbtree |
| Multi-protocol | Separate tools: iptables/ip6tables/arptables/ebtables | Single `nft` tool, unified syntax |
| Rule updates | Dump + re-inject entire table (non-atomic) | Transaction commit: NEWBATCH→rules→COMMIT |
| Kernel ABI | Every new match/target = kernel module with fixed struct | Bytecode VM: new expressions without kernel ABI changes |
| Debugging | `iptables -L` shows names; counters on rules only | Rich counters, named sets, nftrace tracing framework |
| Semantics | Table name implies semantics (filter/mangle/nat) | Tables are namespaces; chains carry type+hook+priority |

### 4.2 Object Hierarchy: Table → Chain → Rule → Expression

```
Namespace (network namespace: /proc/net/nft_*)
  │
  └── Table  (namespace + address family)
        │  name: e.g., "main", "filter", "nat"
        │  family: ip | ip6 | inet | arp | bridge | netdev
        │
        ├── Chain (base chain or regular chain)
        │     │  type: filter | nat | route
        │     │  hook: prerouting|input|forward|output|postrouting|ingress|egress
        │     │  priority: integer (NF_IP_PRI_*)
        │     │  policy: accept | drop
        │     │
        │     └── Rule
        │           │  handle: u64 (unique stable identifier)
        │           │  position: ordering within chain
        │           │
        │           └── [expression list] evaluated left-to-right as bytecode
        │                 e.g.: meta iifname "eth0" ip saddr 10.0.0.0/8 counter accept
        │
        ├── Set  (named lookup table)
        │     │  type: ipv4_addr | ipv6_addr | ether_addr | inet_service | ...
        │     │  flags: constant | interval | timeout | dynamic
        │     │  policy: performance (hash) | memory (rbtree)
        │     │  elements: { 10.0.0.1, 10.0.0.2, ... }
        │     │  timeout: per-element TTL (for dynamic sets)
        │     │
        │     └── used by rules: ip saddr @my_blocklist drop
        │
        ├── Map  (set that maps key → value)
        │     │  type: ipv4_addr : mark
        │     │  elements: { 10.0.0.1 : 0x10, 10.0.0.2 : 0x20 }
        │     │
        │     └── used in rules: meta mark set ip saddr map @route_map
        │
        ├── Verdict Map (vmap)
        │     │  type: ipv4_addr : verdict
        │     │  elements: { 10.0.0.1 : accept, 10.0.0.2 : drop }
        │     │
        │     └── used in rules: ip saddr vmap @verdict_map
        │
        ├── Flowtable  (software fast-path / hardware offload)
        │     │  hook: ingress (early NF_NETDEV_INGRESS)
        │     │  devices: [ eth0, eth1 ]
        │     │  flags: offload (hardware)
        │     └── used by rules: flow add @ft
        │
        ├── Counter  (named, stateful counter object)
        ├── Quota    (named byte/packet quota)
        ├── Limit    (named rate limit: packets/sec or bytes/sec)
        ├── CT Helper (conntrack protocol helper object)
        └── CT Timeout (conntrack timeout policy object)
```

**Base chain vs regular chain**:

- **Base chain**: attached to a netfilter hook. Has `type`, `hook`, `priority`, `policy`.
  Packets enter this chain automatically from the kernel hook.
- **Regular chain**: no hook attachment. Only reachable via `jump` or `goto` from a base
  chain or other chain. Used for structuring rule logic (subroutines).

`jump` pushes return context; after the target chain finishes (or hits `return`), evaluation
resumes in the calling chain after the `jump` rule.  
`goto` does **not** push return context; it's a tail-call to the target chain.

### 4.3 nftables Bytecode VM

This is the core innovation. When you write `nft add rule ...`, the `nft` binary uses
**libnftables** (or directly **libnftnl**) to compile the textual rule into a sequence of
**nft expressions**, each represented as a kernel object (C struct) and serialized as
Netlink attributes over the nfnetlink socket.

The kernel side evaluates expressions left-to-right using a small **register file**:

```
Registers:
  NFT_REG_VERDICT   (register 0): holds the current verdict (CONTINUE/ACCEPT/DROP/etc.)
  NFT_REG_1 .. NFT_REG_4: 16-byte general-purpose data registers

  In 32-bit register mode (NFT_REG32_00 .. NFT_REG32_15):
  Each 16-byte register is split into four 4-byte sub-registers.
  This allows simultaneous storage of two IPv4 addresses, ports, marks, etc.
```

**Evaluation loop** (`net/netfilter/nf_tables_core.c`):

```c
void nft_do_chain(struct nft_pktinfo *pkt, void *priv)
{
    const struct nft_chain *chain = priv;
    const struct nft_rule_dp *rule;
    const struct nft_expr *expr;
    struct nft_regs regs = {};

    regs.verdict.code = NFT_CONTINUE;

    /* Walk rule list in the chain */
    nft_rule_for_each(rule, chain) {

        /* For each expression in the rule */
        nft_rule_dp_for_each_expr(expr, last, rule) {

            /* Call expression's eval function */
            expr->ops->eval(expr, &regs, pkt);

            /* If expression set a non-CONTINUE verdict, stop */
            if (regs.verdict.code != NFT_CONTINUE)
                goto out;
        }
        /* Rule fully evaluated: if verdict still CONTINUE,
         * move to next rule. A matched rule would have set
         * a terminal verdict (ACCEPT, DROP, etc.)          */

        if (regs.verdict.code == NFT_CONTINUE)
            continue;

out:    switch (regs.verdict.code) {
        case NFT_BREAK:   /* expression short-circuited this rule */
            regs.verdict.code = NFT_CONTINUE;
            continue;     /* try next rule */
        case NFT_CONTINUE:
            continue;
        case NFT_JUMP:
            /* Push return point, tail-call target chain */
            ...
        case NFT_GOTO:
            chain = regs.verdict.chain;
            goto next_chain;
        case NFT_RETURN:
            /* Return to calling chain */
            ...
        default:
            /* NF_ACCEPT, NF_DROP, NF_QUEUE, NF_STOLEN */
            return;
        }
    }
}
```

Each expression's `eval` function does one or more of:
1. **Load** packet data into a register (e.g., `nft_payload_eval` loads bytes from skb).
2. **Compare/match** register against a constant, set, or another register.
3. **Transform** data (bitwise ops, byteorder swap, hash).
4. **Set the verdict register** to terminate rule evaluation.
5. **Modify** the packet or metadata (mark, dscp, ttl, NAT parameters).

**NFT_BREAK** verdict: returned by a comparison expression when the condition is *false*.
This causes the rule's remaining expressions to be skipped, and the outer loop moves to the
next rule. This is not exposed in the rule language — it is the mechanism by which
`ip saddr 1.2.3.4 drop` becomes "load saddr into reg, compare with 1.2.3.4, if mismatch →
BREAK (skip rule), else → DROP".

### 4.4 Expressions Catalogue

Every nftables keyword maps to a kernel expression struct:

```
Expression       Kernel struct/file                    What it does
───────────────  ────────────────────────────────────  ─────────────────────────────────────
payload          nft_payload  (net/netfilter/nft_payload.c)
                                                        Load/store raw bytes from skb at
                                                        given base+offset. Base types:
                                                        NFT_PAYLOAD_LL_HEADER (L2),
                                                        NFT_PAYLOAD_NETWORK_HEADER (L3),
                                                        NFT_PAYLOAD_TRANSPORT_HEADER (L4),
                                                        NFT_PAYLOAD_INNER_HEADER (tunnel)

meta             nft_meta    (net/netfilter/nft_meta.c)
                                                        Packet metadata: iifname, oifname,
                                                        mark, priority, pkttype, cpu,
                                                        random, time, day, hour, skuid,
                                                        skgid, cgroup, protocol, nfproto,
                                                        l4proto, iifkind, oifkind

ct               nft_ct      (net/netfilter/nft_ct.c)
                                                        Conntrack fields: state, status,
                                                        direction, mark, id, zone, helper,
                                                        src/dst addr, src/dst port,
                                                        expiration, count, avgpkt, bytes

cmp              nft_cmp     (net/netfilter/nft_cmp.c)
                                                        Compare register vs constant.
                                                        Ops: ==, !=, <, <=, >, >=.
                                                        If mismatch → NFT_BREAK.

range            nft_range   (net/netfilter/nft_range.c)
                                                        Check register is in [low, high].

bitwise          nft_bitwise (net/netfilter/nft_bitwise.c)
                                                        AND/OR/XOR/LSHIFT/RSHIFT on register.
                                                        Used for masking (e.g., subnet match).
                                                        ip saddr & 255.255.255.0 == 10.0.0.0

byteorder        nft_byteorder (net/netfilter/nft_byteorder.c)
                                                        Network↔host byte-order conversion.

lookup           nft_lookup  (net/netfilter/nft_lookup.c)
                                                        Lookup register value in a set.
                                                        Writes set element data to dest reg.
                                                        Used for: ip saddr @blocklist

dynset           nft_dynset  (net/netfilter/nft_dynset.c)
                                                        Insert/update/delete element in a
                                                        dynamic set at match time.
                                                        Used for: add @recent { ip saddr }

immediate        nft_immediate (net/netfilter/nft_immediate.c)
                                                        Load a constant (verdict or value)
                                                        into a register immediately.

counter          nft_counter (net/netfilter/nft_counter.c)
                                                        Per-cpu packet/byte counter.
                                                        Aggregated on read.

log              nft_log     (net/netfilter/nft_log.c)
                                                        Log via netlink (NFLOG) or kernel
                                                        log (printk). Supports prefix, group,
                                                        snaplen, level, flags.

reject           nft_reject  (net/netfilter/nft_reject*.c)
                                                        Drop + send ICMP/ICMPv6/TCP RST.

nat              nft_nat     (net/netfilter/nft_nat.c)
                                                        SNAT/DNAT/Masquerade/Redirect.
                                                        Writes NAT params to conntrack.

tproxy           nft_tproxy  (net/netfilter/nft_tproxy.c)
                                                        Transparent proxy: redirect to
                                                        local socket without DNAT visible
                                                        to application.

socket           nft_socket  (net/netfilter/nft_socket.c)
                                                        Match/load socket attributes:
                                                        transparent, wildcard, mark, cgroupv2.

osf              nft_osf     (net/netfilter/nft_osf.c)
                                                        Passive OS fingerprinting via SYN.

fib              nft_fib*    (net/netfilter/nft_fib*.c)
                                                        Reverse path filter / FIB lookup.
                                                        fib saddr oif check == "eth0".
                                                        Replaces rp_filter sysctl.

rt               nft_rt      (net/netfilter/nft_rt.c)
                                                        Routing metadata: classid, nexthop,
                                                        mtu.

hash             nft_hash    (net/netfilter/nft_hash.c)
                                                        Symmetric/Jenkins/JHash on register.
                                                        Used for stateless load balancing.

numgen           nft_numgen  (net/netfilter/nft_numgen.c)
                                                        Monotonic/random counter per chain.
                                                        Round-robin or random among N.

dup              nft_dup*    (net/netfilter/nft_dup*.c)
                                                        Duplicate packet to interface/nexthop.

fwd              nft_fwd*    (net/netfilter/nft_fwd*.c)
                                                        Forward packet to device (netdev).

flow offload     nft_flow_offload (net/netfilter/nft_flow_offload.c)
                                                        Install flow into flowtable fast-path.

tunnel           nft_tunnel  (net/netfilter/nft_tunnel.c)
                                                        VXLAN/Geneve tunnel metadata.

xfrm             nft_xfrm    (net/netfilter/nft_xfrm.c)
                                                        IPsec/xfrm: reqid, spi, ip, proto.

limit            nft_limit   (net/netfilter/nft_limit.c)
                                                        Token bucket rate limiter.
                                                        per-rule or named stateful object.

quota            nft_quota   (net/netfilter/nft_quota.c)
                                                        Byte/packet quota, one-shot trigger.

connlimit        nft_connlimit (net/netfilter/nft_connlimit.c)
                                                        Limit concurrent connections per key.

last             nft_last    (net/netfilter/nft_last.c)
                                                        Record timestamp of last match.

synproxy         nft_synproxy (net/netfilter/nft_synproxy.c)
                                                        SYN cookie proxy for DDoS mitigation.

queue            nft_queue   (net/netfilter/nft_queue.c)
                                                        NFQUEUE to userspace with bypass/fanout.
```

### 4.5 Sets, Maps, and Verdict Maps

Sets are the most powerful performance feature. Internally they use one of three data
structures depending on declared flags and element types:

```
Backend     nftables flag            Data structure     Complexity
──────────  ───────────────────────  ─────────────────  ──────────
hash        policy performance       jhash hashtable    O(1) average
rbtree      policy memory            red-black tree     O(log n)
bitmap      (auto for small ports)   bitfield array     O(1)
pipapo      interval (5-tuple)       multi-bit pipeline O(1) amortized

```

**pipapo** (Packet Inspection, Packets, or Prefixes) is the newest backend (5.6+), designed
specifically for sets with interval elements (CIDR ranges, port ranges, 5-tuples). It uses
a SIMD-accelerated multi-field pipeline: each field (src IP, dst IP, src port, dst port,
protocol) is processed by a lookup into a compressed bit-array, and results are ANDed
together. This replaces the rbtree for interval sets and achieves near-O(1) for large sets
of IP ranges that would otherwise require many rbtree comparisons.

**Concatenations** — matching on multiple fields in a single set lookup:

```
# Set matching on (src IP, dst port) tuple in one lookup
set allow_by_src_port {
    type ipv4_addr . inet_service
    flags interval
    elements = {
        192.168.1.0/24 . 80,
        10.0.0.0/8 . 443
    }
}
rule: ip saddr . tcp dport @allow_by_src_port accept
```

This is **one** set lookup vs. two nested `lookup` expressions — significant at scale.

**Dynamic sets** with timeouts are how you implement recent-match / rate-based blocklists:

```
set recent_scanners {
    type ipv4_addr
    flags dynamic, timeout
    timeout 60s
}

# In prerouting chain:
# Add source to set on first SYN, drop if already in set
tcp flags syn add @recent_scanners { ip saddr timeout 60s } \
    ct state new meter scan_rate { ip saddr limit rate 5/second } \
    accept
ip saddr @recent_scanners drop
```

### 4.6 Flowtable (Software Fast-Path)

Flowtables (introduced 4.16, hardware offload in 5.13) implement **connection-level
fastpath bypassing the full netfilter hook chain for established flows**.

```
Normal path (first packet of a new connection):
  NF_INET_PRE_ROUTING → conntrack (NEW) → filter → NF_INET_FORWARD → NF_INET_POST_ROUTING

Flowtable fast-path (subsequent packets of an ESTABLISHED connection):
  NF_NETDEV_INGRESS → flowtable lookup → if hit: bypass all other hooks → NF_NETDEV_EGRESS
```

Architecture:

```
┌─────────────────────────────────────────────────────────────────┐
│  Flowtable "ft"                                                  │
│                                                                  │
│  Hash table keyed by 5-tuple (src_ip, dst_ip, sport, dport, l4)│
│  Each entry: {                                                   │
│    iifidx, oifidx                                               │
│    src_mac, dst_mac  (for L2 rewrite)                           │
│    NAT transformations (from conntrack)                          │
│    timeout (GC via timer)                                        │
│    flags: NF_FLOW_HW (hardware offloaded)                        │
│  }                                                               │
│                                                                  │
│  Hardware offload path (kernel 5.13+):                          │
│    flow_offload_hw_add() → TC flower + pedit actions            │
│    → driver's ndo_setup_tc(TC_SETUP_CLSFLOWER)                  │
│    → NIC performs L2 rewrite + forwarding in hardware           │
└─────────────────────────────────────────────────────────────────┘
```

To use:
```
table inet filter {
    flowtable ft {
        hook ingress priority filter;
        devices = { eth0, eth1 };
        # flags offload;  # hardware offload if NIC supports
    }

    chain forward {
        type filter hook forward priority filter; policy drop;
        # Offload established flows to the flowtable fast-path
        ip protocol { tcp, udp } flow add @ft
        ct state { established, related } accept
    }
}
```

---

## 5. Kernel Implementation Internals

### 5.1 Core Data Structures

```c
/* include/net/netfilter/nf_tables.h — key structs */

/* The table: namespace for chains/sets/objects within one address family */
struct nft_table {
    struct list_head        list;       /* table list in net->nft.tables */
    struct rhltable         chains_ht;  /* chains hash table */
    struct list_head        chains;
    struct list_head        sets;
    struct list_head        objects;
    struct list_head        flowtables;
    u64                     hgenerations; /* handle generation counter */
    u32                     use;          /* refcount */
    u16                     family;       /* NFPROTO_* */
    u16                     flags;        /* NFT_TABLE_F_DORMANT */
    char                    name[];       /* table name */
};

/* Chain: list of rules, optionally attached to a hook */
struct nft_chain {
    struct nft_rule_blob    __rcu *blob_gen_0;  /* RCU-protected rule arrays */
    struct nft_rule_blob    __rcu *blob_gen_1;
    struct list_head        rules;              /* live rule list (for updates) */
    struct list_head        list;               /* chain list in table */
    struct rhlist_head      rhlhead;
    struct nft_table        *table;
    u64                     handle;
    u32                     use;                /* jump/goto refcount */
    u8                      flags;
    char                    *name;
    /* For base chains only: */
    struct nft_stats __percpu *stats;
    struct nft_hook         hook_list[]; /* attached nf_hook_ops */
};

/* Base chain (extends nft_chain) */
struct nft_base_chain {
    struct nf_hook_ops      ops;           /* the registered netfilter hook */
    struct list_head        hook_list;
    const struct nft_chain_type *type;     /* filter/nat/route */
    u8                      policy;        /* NF_ACCEPT or NF_DROP */
    struct nft_stats __percpu *stats;
    struct nft_chain        chain;         /* embedded */
    struct flow_block       flow_block;    /* for TC/flowtable integration */
};

/* Rule: sequence of expressions */
struct nft_rule {
    struct list_head        list;
    u64                     handle;
    /* Expressions are encoded in data[] as a flat byte array.
     * Each expression: nft_expr_info header + private data.
     * Layout designed for cache efficiency in evaluation. */
    unsigned int            dlen;       /* data length */
    u8                      udata;      /* has user data? */
    unsigned char           data[]
        __attribute__((aligned(__alignof__(struct nft_expr))));
};

/* Expression: base header in rule data[] */
struct nft_expr {
    const struct nft_expr_ops   *ops;
    /* ops->priv_size bytes of expression-specific data follow */
    unsigned char               data[];
};

/* Expression ops vtable */
struct nft_expr_ops {
    void (*eval)(const struct nft_expr *expr,
                 struct nft_regs *regs,
                 const struct nft_pktinfo *pkt);
    int  (*init)(const struct nft_ctx *ctx,
                 const struct nft_expr *expr,
                 const struct nlattr * const tb[]);
    void (*destroy)(const struct nft_ctx *ctx, const struct nft_expr *expr);
    int  (*dump)(struct sk_buff *skb, const struct nft_expr *expr);
    ...
    unsigned short              size;       /* priv_size: sizeof private data */
    unsigned int                flags;
    const struct nft_expr_type  *expr_type;
};

/* Register file passed to each expression eval */
struct nft_regs {
    union {
        u32     data[20];       /* 5 × 16-byte registers as u32 array */
        struct nft_verdict verdict;  /* overlaps data[0..1] */
    };
};

/* Packet info: wraps sk_buff + hook state for the eval loop */
struct nft_pktinfo {
    struct sk_buff          *skb;
    const struct nf_hook_state *state;
    bool                    tprot_set;  /* transport protocol parsed? */
    u8                      tprot;      /* L4 protocol number */
    u16                     fragoff;    /* IPv4 fragment offset */
    u16                     thoff;      /* transport header offset in skb */
    u16                     inneroff;   /* inner header offset (tunnels) */
};
```

### 5.2 RCU, Transactions, and Atomic Commit

**The locking challenge**: Packets are evaluated at softirq (BH) context, concurrent with
userspace rule updates from process context. Using a global lock would serialize softirq
processing — unacceptable for performance.

Solution: **RCU (Read-Copy-Update)** for the rule arrays, with a **generation counter**
scheme for atomic rule-set activation.

```
Two generations: GEN_0 and GEN_1

Active generation bit: per-network-namespace u8 nft_net->gencursor (0 or 1)

Each chain has:
  blob_gen_0: rule blob for generation 0
  blob_gen_1: rule blob for generation 1

Packet eval: reads blob for current gencursor under rcu_read_lock()

Rule update transaction:
  1. nft_do_chain evaluates blob[gencursor]
  2. Userspace sends NEWBATCH + rule changes
  3. Kernel builds new rule blobs in the *inactive* generation slot
  4. COMMIT: smp_wmb() + atomic flip of gencursor
  5. RCU grace period: wait for all in-flight evaluations to finish
  6. Free old blobs
```

This means rule updates are **truly atomic** at packet-processing level — no packet ever
sees a half-committed ruleset. This was impossible with iptables (which did a full dump +
restore via libiptc with a per-table lock).

**Transaction protocol** (nfnetlink batch):

```
NFNL_MSG_BATCH_BEGIN    (type=0x10, nfnetlink message)
  NFTA_BATCH_GENID: 42  (optional: ensure we're updating expected generation)

  NFT_MSG_NEWTABLE ...  (add/modify objects)
  NFT_MSG_NEWCHAIN ...
  NFT_MSG_NEWRULE  ...
  NFT_MSG_NEWSET   ...
  NFT_MSG_SETELEM  ...

NFNL_MSG_BATCH_END

If any message fails → kernel replies NFTA_BATCH_ABORT, no changes applied
If all succeed → kernel replies NFTA_BATCH_SUCCESS, generation is committed
```

### 5.3 nfnetlink Protocol

nfnetlink is built on top of Netlink (AF_NETLINK, NETLINK_NETFILTER). It adds a
subsystem-multiplexing layer:

```
Netlink message header (struct nlmsghdr):
  nlmsg_type = (subsystem_id << 8) | message_type

Subsystem IDs:
  NFNL_SUBSYS_CTNETLINK    = 1   (conntrack CRUD)
  NFNL_SUBSYS_CTNETLINK_EXP= 2   (conntrack expectations)
  NFNL_SUBSYS_QUEUE        = 3   (NFQUEUE)
  NFNL_SUBSYS_ULOG         = 4   (NFLOG / ulogd2)
  NFNL_SUBSYS_OSF          = 5   (OS fingerprinting)
  NFNL_SUBSYS_IPSET        = 6   (ipset compat — not nftables sets)
  NFNL_SUBSYS_ACCT         = 7   (NFACCT)
  NFNL_SUBSYS_CTNETLINK_TIMEOUT=8
  NFNL_SUBSYS_CTHELPER     = 9
  NFNL_SUBSYS_NFTABLES     = 12  (nftables — all nft* messages)
  NFNL_SUBSYS_NFT_COMPAT   = 13  (iptables-nft compat layer)

nftables message types (within subsys 12):
  NFT_MSG_NEWTABLE, NFT_MSG_GETTABLE, NFT_MSG_DELTABLE
  NFT_MSG_NEWCHAIN, NFT_MSG_GETCHAIN, NFT_MSG_DELCHAIN
  NFT_MSG_NEWRULE,  NFT_MSG_GETRULE,  NFT_MSG_DELRULE
  NFT_MSG_NEWSET,   NFT_MSG_GETSET,   NFT_MSG_DELSET
  NFT_MSG_NEWSETELEM, NFT_MSG_GETSETELEM, NFT_MSG_DELSETELEM
  NFT_MSG_NEWGEN,   NFT_MSG_GETGEN    (generation notification)
  NFT_MSG_TRACE                       (nftrace events)
  NFT_MSG_NEWFLOWTABLE, ...
  NFT_MSG_NEWOBJ, ...                 (stateful objects)
```

### 5.4 `nf_tables` Module Layout

```
net/netfilter/
  nf_tables_core.c      Main evaluation engine: nft_do_chain(), rule dispatch
  nf_tables_api.c       nfnetlink handlers: CRUD for tables/chains/rules/sets
  nf_tables_trace.c     nftrace packet tracing infrastructure
  nft_chain_filter.c    Chain type "filter" registration
  nft_chain_nat.c       Chain type "nat" (calls nf_nat_*)
  nft_chain_route.c     Chain type "route" (sets IP TOS/mark for routing)
  nft_payload.c         payload expression
  nft_meta.c            meta expression
  nft_ct.c              ct expression (conntrack integration)
  nft_cmp.c             cmp expression
  nft_bitwise.c         bitwise expression
  nft_byteorder.c       byteorder expression
  nft_lookup.c          lookup (set membership) expression
  nft_dynset.c          dynamic set update expression
  nft_nat.c             nat expression (snat/dnat/masq/redir)
  nft_limit.c           limit expression + stateful object
  nft_log.c             log expression
  nft_reject*.c         reject expression (IPv4/IPv6/bridge)
  nft_hash.c            hash expression (jhash/symmetric)
  nft_numgen.c          numgen (counter/random)
  nft_connlimit.c       connlimit expression
  nft_synproxy.c        synproxy expression
  nft_socket.c          socket expression
  nft_fib*.c            fib expression (IPv4/IPv6/bridge)
  nft_rt.c              rt (routing metadata) expression
  nft_tproxy.c          tproxy expression
  nft_xfrm.c            xfrm (IPsec) expression
  nft_tunnel.c          tunnel metadata expression
  nft_queue.c           queue (NFQUEUE) expression
  nft_set_hash.c        hash set backend
  nft_set_rbtree.c      rbtree set backend
  nft_set_bitmap.c      bitmap set backend
  nft_set_pipapo.c      pipapo interval set backend
  nft_set_pipapo_avx2.c  AVX2-accelerated pipapo
  nft_flow_offload.c    flowtable offload expression
  nft_counter.c         named counter stateful object
  nft_quota.c           named quota stateful object
  nft_ct_helper.c       named ct helper object
  nft_ct_timeout.c      named ct timeout object
  nft_last.c            last-match timestamp object
  nf_flow_table_core.c  flowtable core (hash + GC)
  nf_flow_table_ip.c    flowtable IPv4/IPv6 fast-path eval
  nf_flow_table_offload.c hardware offload via TC
```

---

## 6. Connection Tracking (conntrack)

### 6.1 Architecture and Hash Table

Connection tracking (`nf_conntrack`) maintains a state table of all tracked connections.
It is not specific to nftables — it is a separate netfilter subsystem that both iptables
and nftables use.

```
struct nf_conn {
    struct nf_conntrack         ct_general;         /* refcount */
    spinlock_t                  lock;

    /* Tuple pair: original direction and reply direction */
    struct nf_conntrack_tuple_hash tuplehash[IP_CT_DIR_MAX];
    /*
     * tuplehash[IP_CT_DIR_ORIGINAL]:  client→server direction
     * tuplehash[IP_CT_DIR_REPLY]:     server→client direction
     *
     * Each tuplehash contains:
     *   struct nf_conntrack_tuple {
     *     struct nf_conntrack_man src;  { addr, l4.{port,id,all} }
     *     struct {
     *       union nf_inet_addr ip;
     *       union nf_conntrack_man_proto l4;
     *       u_int8_t protonum;
     *       u_int8_t dir;
     *     } dst;
     *   }
     */

    unsigned long               status;             /* IPS_* bitmask */
    u32                         timeout;            /* timer */
    possible_net_t              ct_net;

    /* Extension area (optional, allocated dynamically):
     *   - NAT info (nf_nat_conn_key + manip info)
     *   - Conntrack helper private data
     *   - Conntrack accounting (per-dir byte/packet counters)
     *   - Conntrack mark (skb->nfct_reasm → ct->mark)
     *   - Conntrack labels (128-bit arbitrary label)
     *   - Conntrack seqadj (TCP sequence adjustment for NAT)
     *   - Conntrack synproxy
     *   - Conntrack timeout override
     */
    struct nf_ct_ext            *ext;

    union nf_conntrack_proto    proto;              /* protocol-specific state */
};
```

The global conntrack table is a **hash table of `nf_conntrack_tuple_hash`** entries,
keyed by the 5-tuple hash. Each connection has **two** entries in the table (one per
direction), pointing to the same `nf_conn`.

```
nf_conntrack_hash[] (per-netns, sized at boot by nf_conntrack_max sysctl)
   │
   ├── bucket[hash(src_ip,dst_ip,sport,dport,proto,zone)] → tuplehash ORIGINAL
   │      └── .tuplehash[ORIGINAL].tuple  = { caddr, saddr, cport, sport, proto }
   │      └── hlist → next (chain collision)
   │
   └── bucket[hash(dst_ip,src_ip,sport,dport,proto,zone)] → tuplehash REPLY
          └── .tuplehash[REPLY].tuple     = { saddr, caddr, sport, cport, proto }
```

**Conntrack zones** (`CT_DEFAULT_ZONE = 0`): Allow multiple independent conntrack
namespaces within one network namespace. Used by OVS (Open vSwitch) to track connections
per-VLAN or per-VRF without cross-contamination.

### 6.2 State Machine

The `status` field is a bitmask of `IPS_*` flags. The `ct state` nftables expression
maps these flags to human-readable states:

```
nftables state  IPS_* flags                 Meaning
──────────────  ──────────────────────────  ──────────────────────────────────────
new             !IPS_SEEN_REPLY             First packet of a new connection
                                            (reply direction not yet seen)
established     IPS_SEEN_REPLY              Bidirectional traffic seen
related         IPS_RELATED                 Related connection (FTP data, ICMP err)
invalid         (failed to track)           Malformed or out-of-state packet
untracked       (NOTRACK / raw chain)       Deliberately bypassed conntrack
```

**TCP state machine** (inside `proto.tcp`):

```
NONE → SYN_SENT → SYN_RECV → ESTABLISHED → FIN_WAIT → CLOSE_WAIT → LAST_ACK → TIME_WAIT → CLOSE
```

The conntrack TCP state machine validates flag combinations and sequence numbers. A packet
with unexpected flags (e.g., RST without being in an established connection) results in
`ct state invalid` — which should normally be dropped.

**Conntrack decision** — happens at PRE_ROUTING (priority -200):
1. Compute 5-tuple hash of incoming packet.
2. Look up in `nf_conntrack_hash`.
3. **HIT**: Found existing connection. Update state (ACK → ESTABLISHED, FIN → TIME_WAIT).
   Stamp `skb->nfct` with pointer to `nf_conn`.
4. **MISS**: New connection. Create `nf_conn`, add ORIGINAL entry. Mark as `new`.
   Apply connection expectations (helpers).
5. At POST_ROUTING (priority INT_MAX), `nf_ct_confirm()` adds the REPLY tuple entry,
   confirming the connection is tracked bidirectionally.

### 6.3 ALG / CT Helpers

Some protocols embed IP:port information inside the payload (FTP PORT/PASV, SIP, H.323,
IRC DCC). Without ALG (Application Layer Gateway) support, NAT would corrupt these.

CT Helpers parse the application-layer content and create **conntrack expectations**:

```
struct nf_conntrack_expect {
    struct hlist_node       lnode;      /* in nf_ct_expect_hash */
    struct nf_conntrack_tuple tuple;    /* expected 5-tuple */
    struct nf_conntrack_tuple mask;     /* wildcard mask */
    void (*expectfn)(struct nf_conn *, struct nf_conntrack_expect *);
    struct nf_conntrack_helper *helper;
    struct nf_conn          *master;    /* owning connection */
    struct timer_list       timeout;
    unsigned short          flags;
};
```

When a new connection matches an expectation, it is marked `IPS_RELATED` and the
expectation is destroyed. The related connection inherits the helper and any NAT
transformations defined by the expectation's `expectfn`.

**Security concern**: CT helpers are powerful but historically have had serious
vulnerabilities (CVE-2020-14386, several others). As of kernel 4.7+, the `sysctl`
`nf_conntrack_helper` defaults to **0** (disabled), and the `nf_conntrack_helper`
must be explicitly assigned per-connection via nftables `ct helper set` or iptables
`-j CT --helper`. Never enable the global helper auto-assignment sysctl in production.

### 6.4 NAT Integration

NAT is implemented as a conntrack **extension**. When a packet is NATted, the transformation
parameters are stored in the `nf_conn`:

```
nf_conn->ext:
  NFT_CT_EXT_NAT:
    struct nf_nat_conn_key: {
      union nf_inet_addr min, max;   /* IP range */
      union nf_conntrack_man_proto min_proto, max_proto;  /* port range */
    } range[IP_CT_DIR_MAX];
    enum nf_nat_manip_type manip;    /* NF_NAT_MANIP_SRC or DST */
```

The NAT module hooks into POST_ROUTING (SNAT, priority 100) and PRE_ROUTING (DNAT,
priority -100) and applies the stored transformation. For reply-direction packets,
the reverse transformation is automatically applied. This is why `ct state established`
flows work correctly through NAT without any additional rules.

### 6.5 Conntrack Zones

```
# Assign packets on vlan100 to zone 1
table netdev edge {
    chain ingress {
        type filter hook ingress device eth0 priority -300;
        vlan id 100 ct zone set 1
    }
}

# Use zone in filter rules
table inet filter {
    chain input {
        ct zone 1 ip saddr 10.100.0.0/24 accept
    }
}
```

Zones are identified by a u16 integer stored in `nf_conntrack_zone.id`. Lookups include
the zone ID in the hash key, so connections in different zones are completely independent.

---

## 7. NAT Subsystem

### 7.1 SNAT / DNAT / Masquerade / Redirect

```
SNAT (Source NAT):
  Modifies source address/port in POST_ROUTING.
  Used on egress for outbound connections.
  Rule: ip saddr 192.168.0.0/24 oif eth0 snat ip to 1.2.3.4

DNAT (Destination NAT):
  Modifies destination address/port in PRE_ROUTING.
  Used for port forwarding / load balancing.
  Rule: iif eth0 tcp dport 80 dnat ip to 10.0.0.5:8080

Masquerade:
  Dynamic SNAT: source IP is taken from the outgoing interface at match time.
  Handles DHCP-dynamic IPs automatically. Slightly more expensive than SNAT
  (must look up interface IP on every packet vs. static IP in SNAT).
  Rule: iif eth0 oif ppp0 masquerade

Redirect:
  DNAT to the local machine (loopback).
  Useful for transparent proxying: redirect TCP 80 to local port 3128.
  Rule: tcp dport 80 redirect to :3128
```

### 7.2 NAT and Conntrack Interaction

```
Packet 1 (SYN):  src=192.168.1.5:54321, dst=8.8.8.8:53
  → PRE_ROUTING conntrack: new connection, IPS_NEW, no DNAT rule matches
  → POST_ROUTING NAT chain: masquerade
      store in nf_conn: SNAT src → 1.2.3.4:60000 (allocated from ephemeral range)
  → POST_ROUTING conntrack CONFIRM: install REPLY tuple (8.8.8.8:53 → 1.2.3.4:60000)
  → Sent: src=1.2.3.4:60000, dst=8.8.8.8:53

Packet 2 (SYN-ACK): src=8.8.8.8:53, dst=1.2.3.4:60000
  → PRE_ROUTING conntrack: REPLY tuple match → IPS_ESTABLISHED
      read NAT info: reverse SNAT → restore dst=192.168.1.5:54321
  → Delivered to 192.168.1.5:54321 as if directly connected
```

### 7.3 Full-Cone vs Symmetric NAT

Linux netfilter implements **Full-Cone NAT** by default for UDP (all external hosts can
reach the mapped port) unless you use `--random-fully` flag or nftables `random`/`fully-random`
flags in the NAT rule, which gives **Symmetric NAT** (separate port allocation per
destination). Symmetric NAT breaks many P2P protocols (WebRTC without TURN, etc.).

```nft
# Full-cone (default)
iif eth0 udp dport 53 redirect to :5353

# Symmetric (unique port per dest)
oif eth0 masquerade fully-random
```

---

## 8. iptables vs nftables — Architecture Comparison

```
                    iptables                          nftables
──────────────────  ────────────────────────────────  ──────────────────────────────────────
Rule language       Match + Target (binary structs)   Expressions (bytecode VM)
Kernel update path  libiptc: dump + re-inject         nfnetlink batch transaction
Atomicity           Table-level lock (non-atomic)     Generation-counter swap (atomic)
Address families    4 tools (iptables/ip6/arp/eb)     1 tool (nft), unified syntax
Set support         External (ipset module)           Native (hash/rbtree/pipapo)
Rule numbering      Implicit (line number)            Explicit handles (u64)
Chains              Hardcoded: INPUT/OUTPUT/FORWARD   User-defined names + types
Counters            Per-rule only                     Per-rule + named stateful objects
Tracing             None (debug kernel only)          nftrace: per-rule trace flag
NAT                 iptables-extensions C modules     nft_nat.c expression
Kernel ABI          XT_MATCH_REVISION / XT_TARGET_*   Arbitrary expressions via modules
compat layer        iptables-nft (translates to nft)  N/A
Policy              Per-chain ACCEPT/DROP             Per-chain policy
Locking             BH spinlock + xtables mutex       RCU + per-table mutex for updates
```

**iptables-nft** (nft_compat subsystem): iptables rules translated into nftables bytecode,
allowing gradual migration. The `xt_match` and `xt_target` modules are wrapped as nftables
expressions via `nft_compat_match_eval()` and `nft_compat_target_eval()`. Not recommended
for new deployments — performance is slightly worse and semantics differ.

---

## 9. nft CLI — Syntax and Rule Language

### 9.1 Tables and Chains

```sh
# Create a table
nft add table inet my_firewall

# Delete a table (also deletes all chains/rules inside)
nft delete table inet my_firewall

# Flush a table (delete all chains/rules, keep table)
nft flush table inet my_firewall

# Create a base chain attached to a hook
nft add chain inet my_firewall input \
    '{ type filter hook input priority 0; policy drop; }'

# Create a regular chain (no hook, called via jump/goto)
nft add chain inet my_firewall allowed_tcp

# List everything
nft list ruleset

# List one table
nft list table inet my_firewall

# List with handles (needed for rule insertion/deletion by handle)
nft -a list ruleset
```

### 9.2 Rules and Expressions

```sh
# Append a rule to a chain
nft add rule inet my_firewall input \
    ct state established,related accept

# Insert a rule at the beginning
nft insert rule inet my_firewall input \
    iif lo accept

# Insert at specific position (by handle)
nft add rule inet my_firewall input \
    position 42 tcp dport 443 accept

# Delete a rule by handle
nft delete rule inet my_firewall input handle 7

# Replace a rule by handle
nft replace rule inet my_firewall input handle 7 \
    tcp dport { 80, 443 } accept

# Complex rule examples:

# Match on multiple header fields
nft add rule inet my_firewall input \
    ip version 4 ip protocol tcp \
    ip saddr 10.0.0.0/8 tcp dport 22 \
    ct state new counter accept

# TCP flags
nft add rule inet my_firewall input \
    tcp flags syn / syn,fin,rst,ack counter comment "SYN packets"

# ICMP rate limiting
nft add rule inet my_firewall input \
    icmp type echo-request limit rate 10/second burst 20 packets accept

# Reject with ICMP
nft add rule inet my_firewall input \
    reject with icmp type port-unreachable

# Reject TCP with RST
nft add rule inet my_firewall input \
    tcp dport 113 reject with tcp reset

# Log + drop
nft add rule inet my_firewall input \
    log prefix "DROPPED: " level warn flags all drop

# Jump to sub-chain
nft add rule inet my_firewall input \
    tcp dport { 22, 80, 443 } jump allowed_tcp

# Goto (no return)
nft add rule inet my_firewall input \
    ip saddr @trusted_hosts goto trusted_chain

# Time-based rules (requires nftables 0.9.1+, kernel 5.5+)
nft add rule inet my_firewall input \
    meta time "2024-01-01 00:00:00" - "2024-12-31 23:59:59" \
    tcp dport 8080 accept

# Day-of-week
nft add rule inet my_firewall input \
    meta day { "Monday", "Tuesday" } meta hour "09:00" - "17:00" accept

# Mark and routing
nft add rule inet my_firewall output \
    ip daddr 10.0.0.0/8 meta mark set 0x100
```

### 9.3 Sets, Maps, Verdict Maps

```sh
# Named set with elements
nft add set inet my_firewall blocked_ips \
    '{ type ipv4_addr; flags interval; }'

nft add element inet my_firewall blocked_ips \
    '{ 1.2.3.4, 10.0.0.0/8, 192.168.1.0/24 }'

nft delete element inet my_firewall blocked_ips '{ 1.2.3.4 }'

# Use set in rule
nft add rule inet my_firewall input \
    ip saddr @blocked_ips drop

# Timeout (dynamic) set
nft add set inet my_firewall recent_ssh \
    '{ type ipv4_addr; flags dynamic, timeout; timeout 60s; }'

nft add rule inet my_firewall input \
    tcp dport 22 ct state new \
    add @recent_ssh { ip saddr } \
    meter ssh_rate { ip saddr limit rate 3/minute } \
    accept

nft add rule inet my_firewall input \
    ip saddr @recent_ssh tcp dport 22 ct state new counter drop

# Map (key → value)
nft add map inet my_firewall interface_marks \
    '{ type ifindex : mark; }'

nft add element inet my_firewall interface_marks \
    '{ eth0 : 0x10, eth1 : 0x20 }'

nft add rule inet my_firewall postrouting \
    meta mark set iif map @interface_marks

# Verdict map
nft add map inet my_firewall src_actions \
    '{ type ipv4_addr : verdict; }'

nft add element inet my_firewall src_actions \
    '{ 10.0.0.1 : accept, 10.0.0.2 : drop, 192.168.1.0/24 : goto trusted_chain }'

nft add rule inet my_firewall input \
    ip saddr vmap @src_actions

# Concatenation set (src IP + dst port tuple)
nft add set inet my_firewall allowed_by_src \
    '{ type ipv4_addr . inet_service; flags interval; }'

nft add element inet my_firewall allowed_by_src \
    '{ 10.0.0.0/8 . 80, 192.168.1.0/24 . 443 }'

nft add rule inet my_firewall input \
    ip saddr . tcp dport @allowed_by_src accept
```

### 9.4 Flowtables

```sh
nft add flowtable inet my_firewall ft \
    '{ hook ingress priority filter; devices = { eth0, eth1 }; }'

# Enable hardware offload (requires NIC support + kernel 5.13+)
# nft add flowtable inet my_firewall ft \
#   '{ hook ingress priority filter; devices = { eth0, eth1 }; flags offload; }'

nft add rule inet my_firewall forward \
    ip protocol { tcp, udp } flow add @ft

nft list flowtable inet my_firewall ft
```

### 9.5 Limits, Quotas, Counters, Meters

```sh
# Per-rule limit (anonymous)
nft add rule inet my_firewall input \
    tcp dport 25 limit rate 100/second accept

# Named rate limit (shared across rules / persists across updates)
nft add limit inet my_firewall smtp_rate \
    '{ rate 100/second; }'

nft add rule inet my_firewall input \
    tcp dport 25 limit name smtp_rate accept

# Quota (byte-based, one-shot)
nft add quota inet my_firewall daily_quota \
    '{ 10 gbytes; }'

nft add rule inet my_firewall postrouting \
    quota name daily_quota accept

# Named counter (persistent across rule updates)
nft add counter inet my_firewall ssh_attempts

nft add rule inet my_firewall input \
    tcp dport 22 ct state new counter name ssh_attempts

nft get element inet my_firewall ssh_attempts

# Meter (per-IP state, uses dynamic set internally)
nft add rule inet my_firewall input \
    tcp dport 80 meter http_meter { ip saddr limit rate 100/second } accept
```

### 9.6 Stateful Objects

```sh
# CT Helper assignment (replaces nf_conntrack_helper auto-assign)
nft add ct helper inet my_firewall ftp_helper \
    '{ type "ftp" protocol tcp; }'

nft add rule inet my_firewall input \
    tcp dport 21 ct state new ct helper set "ftp_helper"

# CT Timeout override
nft add ct timeout inet my_firewall tcp_timeouts \
    '{ protocol tcp;
       policy = { established: 120, close: 10 }; }'

nft add rule inet my_firewall postrouting \
    ct state new tcp dport 80 ct timeout set "tcp_timeouts"

# Synproxy (SYN cookie DDoS mitigation)
nft add synproxy inet my_firewall syn_proxy \
    '{ mss 1460; wscale 7; timestamp; sack-perm; }'

nft add rule inet my_firewall prerouting \
    tcp dport 80 ct state invalid,untracked \
    synproxy name syn_proxy

# Named last (track when a rule last matched)
nft add last inet my_firewall last_ssh
nft add rule inet my_firewall input \
    tcp dport 22 last name last_ssh accept

nft list last inet my_firewall last_ssh
```

---

## 10. C Implementations

### 10.1 Kernel Module with Direct nf_hook_ops

A minimal kernel module that registers a hook at NF_INET_PRE_ROUTING to drop packets
from a hardcoded source IP.

```c
// nfhook_example.c
// Build: make -C /lib/modules/$(uname -r)/build M=$(pwd) modules
// Load:  insmod nfhook_example.ko
// Remove: rmmod nfhook_example

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/netfilter.h>
#include <linux/netfilter_ipv4.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/skbuff.h>
#include <linux/inet.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("example");
MODULE_DESCRIPTION("Netfilter hook example");

/* Block traffic from this IP */
static __be32 blocked_ip;
static char *block_ip_str = "10.0.0.1";
module_param(block_ip_str, charp, 0444);

/*
 * Hook function: called for every IPv4 packet at PRE_ROUTING.
 *
 * priv:  pointer set in nf_hook_ops.priv (we pass NULL)
 * skb:   the packet sk_buff
 * state: hook state (in/out interface, net namespace, etc.)
 */
static unsigned int my_hook_fn(void *priv,
                                struct sk_buff *skb,
                                const struct nf_hook_state *state)
{
    struct iphdr *iph;

    /* Sanity: skb must be non-NULL and have an IP header */
    if (!skb)
        return NF_ACCEPT;

    iph = ip_hdr(skb);
    if (!iph)
        return NF_ACCEPT;

    /* Drop if source matches */
    if (iph->saddr == blocked_ip) {
        pr_info("nfhook_example: blocking packet from %pI4\n", &iph->saddr);
        return NF_DROP;
    }

    return NF_ACCEPT;
}

static struct nf_hook_ops my_hook_ops = {
    .hook       = my_hook_fn,
    .pf         = NFPROTO_IPV4,
    .hooknum    = NF_INET_PRE_ROUTING,
    .priority   = NF_IP_PRI_FILTER,
};

static int __init nfhook_init(void)
{
    int ret;

    /* Parse IP string to binary */
    if (!in4_pton(block_ip_str, -1, (u8 *)&blocked_ip, -1, NULL)) {
        pr_err("nfhook_example: invalid IP: %s\n", block_ip_str);
        return -EINVAL;
    }

    /*
     * nf_register_net_hook registers for the init_net namespace only.
     * For all namespaces: nf_register_net_hooks(net, ops, n) per namespace,
     * or use nf_register_hook (deprecated, global) pre-4.13.
     */
    ret = nf_register_net_hook(&init_net, &my_hook_ops);
    if (ret) {
        pr_err("nfhook_example: hook registration failed: %d\n", ret);
        return ret;
    }

    pr_info("nfhook_example: loaded, blocking %pI4\n", &blocked_ip);
    return 0;
}

static void __exit nfhook_exit(void)
{
    nf_unregister_net_hook(&init_net, &my_hook_ops);
    pr_info("nfhook_example: unloaded\n");
}

module_init(nfhook_init);
module_exit(nfhook_exit);
```

```makefile
# Makefile
obj-m := nfhook_example.o

KDIR ?= /lib/modules/$(shell uname -r)/build

all:
	$(MAKE) -C $(KDIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean

load:
	insmod nfhook_example.ko block_ip_str="192.168.1.100"

unload:
	rmmod nfhook_example
```

### 10.2 Userspace with libmnl + libnftnl

Raw Netlink + nfnetlink construction without the nft binary. This is how nft itself works.
Useful for programmatic rule management in C daemons (e.g., firewalld, cloud-init agents).

```c
// nft_rules_libmnl.c
// Build: gcc -o nft_rules_libmnl nft_rules_libmnl.c \
//            -lmnl -lnftnl
// Requires: libmnl-dev, libnftnl-dev

#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <stdio.h>
#include <arpa/inet.h>
#include <linux/netfilter.h>
#include <linux/netfilter/nf_tables.h>
#include <libmnl/libmnl.h>
#include <libnftnl/table.h>
#include <libnftnl/chain.h>
#include <libnftnl/rule.h>
#include <libnftnl/expr.h>

/*
 * Helper: send an nfnetlink message and wait for ACK.
 * Returns 0 on success, -1 on error.
 */
static int send_and_ack(struct mnl_socket *nl, struct nlmsghdr *nlh,
                         unsigned int portid, unsigned int seq)
{
    char buf[MNL_SOCKET_BUFFER_SIZE];
    ssize_t ret;

    if (mnl_socket_sendto(nl, nlh, nlh->nlmsg_len) < 0) {
        perror("mnl_socket_sendto");
        return -1;
    }

    ret = mnl_socket_recvfrom(nl, buf, sizeof(buf));
    while (ret > 0) {
        ret = mnl_cb_run(buf, ret, seq, portid, NULL, NULL);
        if (ret <= 0)
            break;
        ret = mnl_socket_recvfrom(nl, buf, sizeof(buf));
    }
    if (ret < 0 && errno != EINTR) {
        perror("mnl_socket_recvfrom");
        return -1;
    }
    return 0;
}

/*
 * Create a table "my_table" in the inet family.
 */
static int create_table(struct mnl_socket *nl, unsigned int portid, unsigned int *seq)
{
    char buf[MNL_SOCKET_BUFFER_SIZE];
    struct nlmsghdr *nlh;
    struct nfgenmsg *nfg;
    struct nftnl_table *t;

    t = nftnl_table_alloc();
    if (!t) return -1;

    nftnl_table_set_str(t, NFTNL_TABLE_NAME, "my_table");
    nftnl_table_set_u32(t, NFTNL_TABLE_FAMILY, NFPROTO_INET);

    nlh = nftnl_table_nlmsg_build_hdr(buf,
            NFT_MSG_NEWTABLE,       /* message type */
            NFPROTO_INET,
            NLM_F_ACK | NLM_F_CREATE,
            (*seq)++);

    nftnl_table_nlmsg_build_payload(nlh, t);
    nftnl_table_free(t);

    return send_and_ack(nl, nlh, portid, *seq - 1);
}

/*
 * Create base chain "input" attached to NF_INET_LOCAL_IN, policy DROP.
 */
static int create_chain(struct mnl_socket *nl, unsigned int portid, unsigned int *seq)
{
    char buf[MNL_SOCKET_BUFFER_SIZE];
    struct nlmsghdr *nlh;
    struct nftnl_chain *c;

    c = nftnl_chain_alloc();
    if (!c) return -1;

    nftnl_chain_set_str(c,  NFTNL_CHAIN_TABLE,   "my_table");
    nftnl_chain_set_str(c,  NFTNL_CHAIN_NAME,    "input");
    nftnl_chain_set_u32(c,  NFTNL_CHAIN_FAMILY,  NFPROTO_INET);
    nftnl_chain_set_u32(c,  NFTNL_CHAIN_HOOKNUM, NF_INET_LOCAL_IN);
    nftnl_chain_set_s32(c,  NFTNL_CHAIN_PRIO,    0);
    nftnl_chain_set_u32(c,  NFTNL_CHAIN_POLICY,  NF_DROP);
    /* Chain type: "filter" */
    nftnl_chain_set_str(c,  NFTNL_CHAIN_TYPE,    "filter");

    nlh = nftnl_chain_nlmsg_build_hdr(buf,
            NFT_MSG_NEWCHAIN,
            NFPROTO_INET,
            NLM_F_ACK | NLM_F_CREATE,
            (*seq)++);

    nftnl_chain_nlmsg_build_payload(nlh, c);
    nftnl_chain_free(c);

    return send_and_ack(nl, nlh, portid, *seq - 1);
}

/*
 * Add rule: "ct state established,related accept"
 *
 * Bytecode breakdown:
 *   expr[0]: ct load "state" → reg1
 *   expr[1]: bitwise reg1 & 0x6 (ESTABLISHED=0x2|RELATED=0x4) → reg1
 *   expr[2]: cmp reg1 != 0 → if zero, BREAK (no match)
 *   expr[3]: immediate verdict = ACCEPT
 *
 * libnftnl handles encoding these as Netlink attributes.
 */
static int add_accept_established_rule(struct mnl_socket *nl,
                                        unsigned int portid,
                                        unsigned int *seq)
{
    char buf[MNL_SOCKET_BUFFER_SIZE];
    struct nlmsghdr *nlh;
    struct nftnl_rule *r;
    struct nftnl_expr *e;

    r = nftnl_rule_alloc();
    if (!r) return -1;

    nftnl_rule_set_str(r, NFTNL_RULE_TABLE,  "my_table");
    nftnl_rule_set_str(r, NFTNL_RULE_CHAIN,  "input");
    nftnl_rule_set_u32(r, NFTNL_RULE_FAMILY, NFPROTO_INET);

    /* Expression 1: load ct state into register 1 */
    e = nftnl_expr_alloc("ct");
    nftnl_expr_set_u32(e, NFTNL_EXPR_CT_KEY,   NFT_CT_STATE);
    nftnl_expr_set_u32(e, NFTNL_EXPR_CT_DREG,  NFT_REG_1);
    nftnl_rule_add_expr(r, e);

    /* Expression 2: bitwise AND with mask = ESTABLISHED(2) | RELATED(4) = 6 */
    {
        uint32_t mask = htonl(NF_CT_STATE_ESTABLISHED_BIT | NF_CT_STATE_RELATED_BIT);
        uint32_t xor  = 0;
        e = nftnl_expr_alloc("bitwise");
        nftnl_expr_set_u32(e, NFTNL_EXPR_BITWISE_SREG,  NFT_REG_1);
        nftnl_expr_set_u32(e, NFTNL_EXPR_BITWISE_DREG,  NFT_REG_1);
        nftnl_expr_set_u32(e, NFTNL_EXPR_BITWISE_LEN,   sizeof(uint32_t));
        nftnl_expr_set(e, NFTNL_EXPR_BITWISE_MASK, &mask, sizeof(mask));
        nftnl_expr_set(e, NFTNL_EXPR_BITWISE_XOR,  &xor,  sizeof(xor));
        nftnl_rule_add_expr(r, e);
    }

    /* Expression 3: cmp reg1 != 0 (if zero → no match → BREAK) */
    {
        uint32_t val = 0;
        e = nftnl_expr_alloc("cmp");
        nftnl_expr_set_u32(e, NFTNL_EXPR_CMP_SREG, NFT_REG_1);
        nftnl_expr_set_u32(e, NFTNL_EXPR_CMP_OP,   NFT_CMP_NEQ);
        nftnl_expr_set(e, NFTNL_EXPR_CMP_DATA, &val, sizeof(val));
        nftnl_rule_add_expr(r, e);
    }

    /* Expression 4: ACCEPT */
    e = nftnl_expr_alloc("immediate");
    nftnl_expr_set_u32(e, NFTNL_EXPR_IMM_DREG, NFT_REG_VERDICT);
    nftnl_expr_set_u32(e, NFTNL_EXPR_IMM_VERDICT, NF_ACCEPT);
    nftnl_rule_add_expr(r, e);

    nlh = nftnl_rule_nlmsg_build_hdr(buf,
            NFT_MSG_NEWRULE,
            NFPROTO_INET,
            NLM_F_ACK | NLM_F_APPEND | NLM_F_CREATE,
            (*seq)++);

    nftnl_rule_nlmsg_build_payload(nlh, r);
    nftnl_rule_free(r);

    return send_and_ack(nl, nlh, portid, *seq - 1);
}

int main(void)
{
    struct mnl_socket *nl;
    unsigned int seq = 1;
    unsigned int portid;

    /* Open Netlink socket with NETLINK_NETFILTER protocol */
    nl = mnl_socket_open(NETLINK_NETFILTER);
    if (!nl) {
        perror("mnl_socket_open");
        return EXIT_FAILURE;
    }

    if (mnl_socket_bind(nl, 0, MNL_SOCKET_AUTOPID) < 0) {
        perror("mnl_socket_bind");
        mnl_socket_close(nl);
        return EXIT_FAILURE;
    }

    portid = mnl_socket_get_portid(nl);

    /* Note: In production, wrap all three calls in a BATCH transaction:
     * NFNL_MSG_BATCH_BEGIN → create_table → create_chain → add_rule → NFNL_MSG_BATCH_END
     * For clarity, this example sends individual messages. */

    if (create_table(nl, portid, &seq) < 0) goto err;
    if (create_chain(nl, portid, &seq) < 0) goto err;
    if (add_accept_established_rule(nl, portid, &seq) < 0) goto err;

    printf("Rules applied successfully\n");
    mnl_socket_close(nl);
    return EXIT_SUCCESS;

err:
    fprintf(stderr, "Failed to apply rules\n");
    mnl_socket_close(nl);
    return EXIT_FAILURE;
}
```

### 10.3 Userspace with libnftables

`libnftables` is the high-level library used by the `nft` binary itself. It accepts nft
text syntax and handles compilation + netlink submission internally.

```c
// nft_libnftables.c
// Build: gcc -o nft_libnftables nft_libnftables.c -lnftables
// Requires: libnftables-dev (not available in all distros; check pkg-config)

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <nftables/libnftables.h>

int main(void)
{
    struct nft_ctx *ctx;
    int rc;

    /* Initialize nftables context */
    ctx = nft_ctx_new(NFT_CTX_DEFAULT);
    if (!ctx) {
        fprintf(stderr, "nft_ctx_new failed\n");
        return EXIT_FAILURE;
    }

    /* Suppress output to stdout (we handle it ourselves) */
    nft_ctx_buffer_output(ctx);
    nft_ctx_buffer_error(ctx);

    /* Run an nft command — same syntax as nft CLI */
    rc = nft_run_cmd_from_buffer(ctx,
        "add table inet my_fw\n"
        "add chain inet my_fw input { type filter hook input priority 0; policy drop; }\n"
        "add rule inet my_fw input ct state established,related accept\n"
        "add rule inet my_fw input iif lo accept\n"
        "add rule inet my_fw input tcp dport { 22, 80, 443 } ct state new accept\n");

    if (rc != 0) {
        fprintf(stderr, "nft error: %s\n", nft_ctx_get_error_buffer(ctx));
    } else {
        printf("Rules applied\n");
        /* Print current ruleset */
        nft_run_cmd_from_buffer(ctx, "list ruleset");
        printf("%s", nft_ctx_get_output_buffer(ctx));
    }

    nft_ctx_free(ctx);
    return (rc == 0) ? EXIT_SUCCESS : EXIT_FAILURE;
}
```

---

## 11. Rust Implementations

### 11.1 Netlink from Scratch with neli

Low-level approach: directly constructing nfnetlink messages using the `neli` crate.
Useful when you need full control or are embedding in a Rust network daemon.

```toml
# Cargo.toml
[package]
name = "nftables-rs-example"
version = "0.1.0"
edition = "2021"

[dependencies]
neli          = { version = "0.6", features = ["async"] }
neli-proc-macros = "0.1"
libc          = "0.2"
byteorder     = "1"
thiserror     = "1"
tokio         = { version = "1", features = ["full"] }
```

```rust
// src/netlink_raw.rs
//! Low-level nfnetlink message construction for nftables.
//! Demonstrates the wire protocol without high-level abstractions.

use byteorder::{BigEndian, NativeEndian, WriteBytesExt};
use libc::{AF_UNSPEC, NETLINK_NETFILTER};
use neli::{
    consts::nl::{NlmF, NlmFFlags, Nlmsg},
    consts::socket::NlFamily,
    nl::{NlPayload, Nlmsghdr},
    socket::NlSocketHandle,
    types::NlBuffer,
};
use std::io::Write;
use thiserror::Error;

// nfnetlink subsystem and message type constants
// (not yet in neli; define manually from <linux/netfilter/nfnetlink.h>)
const NFNL_SUBSYS_NFTABLES: u8 = 12;
const NFT_MSG_NEWTABLE: u8 = 0;
const NFT_MSG_NEWCHAIN: u8 = 4;

// nftables Netlink attribute types from <linux/netfilter/nf_tables.h>
const NFTA_TABLE_NAME:   u16 = 1;
const NFTA_TABLE_FLAGS:  u16 = 2;
const NFTA_CHAIN_TABLE:  u16 = 1;
const NFTA_CHAIN_NAME:   u16 = 3;
const NFTA_CHAIN_HOOK:   u16 = 4;
const NFTA_CHAIN_POLICY: u16 = 5;
const NFTA_CHAIN_TYPE:   u16 = 7;
const NFTA_HOOK_HOOKNUM: u16 = 1;
const NFTA_HOOK_PRIORITY:u16 = 2;

// Netfilter hook numbers
const NF_INET_LOCAL_IN: u32 = 1;
const NF_ACCEPT: u32 = 1;
const NF_DROP:   u32 = 0;

#[derive(Debug, Error)]
pub enum NftError {
    #[error("Netlink socket error: {0}")]
    Socket(#[from] neli::err::NlError<u16, Vec<u8>>),
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
}

/// Encode a Netlink attribute (TLV: type, length, value).
/// Length includes the 4-byte header. Padded to 4-byte alignment.
fn nlattr(buf: &mut Vec<u8>, attr_type: u16, data: &[u8]) {
    let len = 4 + data.len();
    let padded = (len + 3) & !3;
    buf.write_u16::<NativeEndian>(len as u16).unwrap();
    buf.write_u16::<NativeEndian>(attr_type).unwrap();
    buf.write_all(data).unwrap();
    // Padding bytes
    for _ in 0..(padded - len) {
        buf.push(0u8);
    }
}

/// Encode a nested Netlink attribute (NLA_F_NESTED set in type).
fn nlattr_nested(buf: &mut Vec<u8>, attr_type: u16, inner: &[u8]) {
    // NLA_F_NESTED = 0x8000
    nlattr(buf, attr_type | 0x8000, inner);
}

/// nfgenmsg header: family + version + resource_id
fn nfgenmsg(family: u8) -> Vec<u8> {
    vec![family, 0u8 /* version=0 */, 0u8, 0u8 /* res_id=0 */]
}

/// Build and send NFT_MSG_NEWTABLE for the "inet" family.
pub fn create_table(nl: &mut NlSocketHandle, table_name: &str) -> Result<(), NftError> {
    let mut payload = nfgenmsg(libc::NFPROTO_INET as u8);

    // NFTA_TABLE_NAME attribute
    let mut name_bytes = table_name.as_bytes().to_vec();
    name_bytes.push(0); // NUL-terminate
    nlattr(&mut payload, NFTA_TABLE_NAME, &name_bytes);

    // NFTA_TABLE_FLAGS = 0 (no dormant)
    nlattr(&mut payload, NFTA_TABLE_FLAGS, &0u32.to_be_bytes());

    // nfnetlink message type = (subsys << 8) | msg_type
    let nl_type = ((NFNL_SUBSYS_NFTABLES as u16) << 8) | (NFT_MSG_NEWTABLE as u16);

    let msg = Nlmsghdr::new(
        None,
        nl_type,
        NlmFFlags::new(&[NlmF::Request, NlmF::Ack, NlmF::Create]),
        None,
        None,
        NlPayload::Payload(payload),
    );

    nl.send(msg)?;

    // Read ACK
    let _ack: Nlmsghdr<u16, Vec<u8>> = nl.recv()?.ok_or_else(|| {
        std::io::Error::new(std::io::ErrorKind::UnexpectedEof, "no ACK received")
    })?;

    Ok(())
}

/// Build and send NFT_MSG_NEWCHAIN to create a base chain.
pub fn create_base_chain(
    nl: &mut NlSocketHandle,
    table_name: &str,
    chain_name: &str,
    hooknum: u32,
    priority: i32,
    policy: u32, // NF_ACCEPT or NF_DROP
) -> Result<(), NftError> {
    let mut payload = nfgenmsg(libc::NFPROTO_INET as u8);

    let mut tname = table_name.as_bytes().to_vec();
    tname.push(0);
    nlattr(&mut payload, NFTA_CHAIN_TABLE, &tname);

    let mut cname = chain_name.as_bytes().to_vec();
    cname.push(0);
    nlattr(&mut payload, NFTA_CHAIN_NAME, &cname);

    // NFTA_CHAIN_HOOK: nested { NFTA_HOOK_HOOKNUM, NFTA_HOOK_PRIORITY }
    let mut hook_inner = Vec::new();
    nlattr(&mut hook_inner, NFTA_HOOK_HOOKNUM, &hooknum.to_be_bytes());
    nlattr(&mut hook_inner, NFTA_HOOK_PRIORITY, &priority.to_be_bytes());
    nlattr_nested(&mut payload, NFTA_CHAIN_HOOK, &hook_inner);

    // NFTA_CHAIN_POLICY
    nlattr(&mut payload, NFTA_CHAIN_POLICY, &policy.to_be_bytes());

    // NFTA_CHAIN_TYPE = "filter\0"
    nlattr(&mut payload, NFTA_CHAIN_TYPE, b"filter\0");

    let nl_type = ((NFNL_SUBSYS_NFTABLES as u16) << 8) | (NFT_MSG_NEWCHAIN as u16);

    let msg = Nlmsghdr::new(
        None,
        nl_type,
        NlmFFlags::new(&[NlmF::Request, NlmF::Ack, NlmF::Create]),
        None,
        None,
        NlPayload::Payload(payload),
    );

    nl.send(msg)?;
    let _ack: Nlmsghdr<u16, Vec<u8>> = nl.recv()?.ok_or_else(|| {
        std::io::Error::new(std::io::ErrorKind::UnexpectedEof, "no ACK received")
    })?;

    Ok(())
}

#[tokio::main]
async fn main() -> Result<(), NftError> {
    // Open NETLINK_NETFILTER socket
    let mut nl = NlSocketHandle::connect(NlFamily::Route, None, &[])?;

    create_table(&mut nl, "rust_fw")?;
    println!("Table 'rust_fw' created");

    create_base_chain(
        &mut nl,
        "rust_fw",
        "input",
        NF_INET_LOCAL_IN,
        0,       // priority NF_IP_PRI_FILTER
        NF_DROP, // default policy
    )?;
    println!("Chain 'input' created with policy DROP");

    Ok(())
}
```

### 11.2 High-Level nftables-rs

The `nftables` crate provides idiomatic Rust bindings that generate nft JSON format
(consumed by `nft -j`) or call `nft` directly.

```toml
[dependencies]
nftables = "0.4"
serde     = { version = "1", features = ["derive"] }
serde_json = "1"
```

```rust
// src/nftables_high.rs
//! Production-grade firewall ruleset using the nftables crate.
//! Generates a complete host firewall with:
//!   - Default drop policy
//!   - Loopback allow
//!   - Established/related allow
//!   - Conntrack invalid drop
//!   - SSH rate limiting
//!   - Permitted TCP/UDP service list
//!   - ICMP rate limiting

use nftables::{
    batch::Batch,
    expr::{self, Expression, NamedExpression, Payload, PayloadField},
    schema::{self, Chain, NfListObject, NfObject, Nftables, Rule, Table},
    stmt::{self, Counter, Log, Match, Operator, Statement},
    types::{self, NfFamily},
};

fn build_host_firewall() -> Nftables {
    let mut batch = Batch::new();

    // ── Table ──────────────────────────────────────────────────────────────
    batch.add(NfListObject::Table(Table {
        family: NfFamily::INet,
        name:   "host_fw".into(),
        ..Default::default()
    }));

    // ── Base chain: input, policy drop ────────────────────────────────────
    batch.add(NfListObject::Chain(Chain {
        family: NfFamily::INet,
        table:  "host_fw".into(),
        name:   "input".into(),
        _type:  Some(types::NfChainType::Filter),
        hook:   Some(types::NfHook::Input),
        prio:   Some(0),
        policy: Some(types::NfChainPolicy::Drop),
        ..Default::default()
    }));

    // ── Rule: allow loopback ───────────────────────────────────────────────
    batch.add(NfListObject::Rule(Rule {
        family: NfFamily::INet,
        table:  "host_fw".into(),
        chain:  "input".into(),
        expr:   vec![
            Statement::Match(Match {
                left:  Expression::Named(NamedExpression::Meta(expr::Meta {
                    key: expr::MetaKey::Iifname,
                })),
                right: Expression::String("lo".into()),
                op:    Operator::EQ,
            }),
            Statement::Accept(None),
        ],
        ..Default::default()
    }));

    // ── Rule: allow established + related, drop invalid ───────────────────
    batch.add(NfListObject::Rule(Rule {
        family: NfFamily::INet,
        table:  "host_fw".into(),
        chain:  "input".into(),
        expr:   vec![
            Statement::Match(Match {
                left:  Expression::Named(NamedExpression::CT(expr::CT {
                    key:  expr::CTKey::State,
                    dir:  None,
                    family: None,
                })),
                right: Expression::List(vec![
                    Expression::String("established".into()),
                    Expression::String("related".into()),
                ]),
                op: Operator::IN,
            }),
            Statement::Accept(None),
        ],
        ..Default::default()
    }));

    batch.add(NfListObject::Rule(Rule {
        family: NfFamily::INet,
        table:  "host_fw".into(),
        chain:  "input".into(),
        expr:   vec![
            Statement::Match(Match {
                left:  Expression::Named(NamedExpression::CT(expr::CT {
                    key: expr::CTKey::State,
                    dir: None,
                    family: None,
                })),
                right: Expression::String("invalid".into()),
                op:    Operator::EQ,
            }),
            Statement::Log(Some(Log {
                prefix:   Some("INVALID: ".into()),
                ..Default::default()
            })),
            Statement::Drop(None),
        ],
        ..Default::default()
    }));

    // ── Rule: ICMP echo-request rate limit ────────────────────────────────
    batch.add(NfListObject::Rule(Rule {
        family: NfFamily::INet,
        table:  "host_fw".into(),
        chain:  "input".into(),
        expr:   vec![
            Statement::Match(Match {
                left:  Expression::Named(NamedExpression::Payload(Payload::PayloadField(
                    PayloadField { protocol: "icmp".into(), field: "type".into() },
                ))),
                right: Expression::String("echo-request".into()),
                op:    Operator::EQ,
            }),
            Statement::Limit(stmt::Limit {
                rate: 10,
                rate_unit: Some(stmt::LimitUnit::Packets),
                per: Some(stmt::LimitPer::Second),
                burst: Some(20),
                burst_unit: Some(stmt::LimitUnit::Packets),
                inv: Some(false),
            }),
            Statement::Accept(None),
        ],
        ..Default::default()
    }));

    // ── Rule: SSH with counter ─────────────────────────────────────────────
    batch.add(NfListObject::Rule(Rule {
        family: NfFamily::INet,
        table:  "host_fw".into(),
        chain:  "input".into(),
        expr:   vec![
            Statement::Match(Match {
                left:  Expression::Named(NamedExpression::Payload(Payload::PayloadField(
                    PayloadField { protocol: "tcp".into(), field: "dport".into() },
                ))),
                right: Expression::Number(22),
                op:    Operator::EQ,
            }),
            Statement::Match(Match {
                left:  Expression::Named(NamedExpression::CT(expr::CT {
                    key: expr::CTKey::State,
                    dir: None,
                    family: None,
                })),
                right: Expression::String("new".into()),
                op:    Operator::EQ,
            }),
            Statement::Counter(Some(Counter { packets: None, bytes: None })),
            Statement::Accept(None),
        ],
        ..Default::default()
    }));

    batch.to_nftables()
}

fn main() {
    let ruleset = build_host_firewall();

    // Serialize to nft JSON format
    let json = serde_json::to_string_pretty(&ruleset).expect("serialize");
    println!("{}", json);

    // Apply the ruleset
    // Option 1: pipe to `nft -j -f -` (uses nft binary)
    use std::process::{Command, Stdio};
    use std::io::Write as _;

    let mut nft = Command::new("nft")
        .args(["-j", "-f", "-"])
        .stdin(Stdio::piped())
        .spawn()
        .expect("nft binary not found");

    nft.stdin.as_mut().unwrap().write_all(json.as_bytes()).unwrap();
    let status = nft.wait().expect("nft wait");

    if status.success() {
        println!("Ruleset applied successfully");
    } else {
        eprintln!("nft failed with status: {}", status);
        std::process::exit(1);
    }
}
```

### 11.3 nft Rule Builder Pattern

A type-safe builder for constructing nft text syntax in Rust — useful for code generators,
orchestration daemons, or Kubernetes CNI plugins that need to emit nft commands.

```rust
// src/rule_builder.rs
//! Type-safe nft rule DSL for Rust.
//! Generates nft text syntax with proper escaping.

#[derive(Debug, Clone)]
pub enum Family { IP, IP6, Inet, Netdev, Bridge }

#[derive(Debug, Clone)]
pub enum ChainType { Filter, Nat, Route }

#[derive(Debug, Clone)]
pub enum Hook { Prerouting, Input, Forward, Output, Postrouting, Ingress, Egress }

#[derive(Debug, Clone)]
pub enum Policy { Accept, Drop }

#[derive(Debug, Clone)]
pub enum Verdict { Accept, Drop, Return, Jump(String), Goto(String) }

#[derive(Debug, Clone)]
pub struct RuleBuilder {
    exprs: Vec<String>,
}

impl RuleBuilder {
    pub fn new() -> Self { Self { exprs: Vec::new() } }

    pub fn iif(mut self, iface: &str) -> Self {
        self.exprs.push(format!("iif \"{}\"", iface));
        self
    }
    pub fn oif(mut self, iface: &str) -> Self {
        self.exprs.push(format!("oif \"{}\"", iface));
        self
    }
    pub fn iifname(mut self, iface: &str) -> Self {
        self.exprs.push(format!("iifname \"{}\"", iface));
        self
    }

    pub fn ip_saddr(mut self, cidr: &str) -> Self {
        self.exprs.push(format!("ip saddr {}", cidr));
        self
    }
    pub fn ip_daddr(mut self, cidr: &str) -> Self {
        self.exprs.push(format!("ip daddr {}", cidr));
        self
    }
    pub fn ip6_saddr(mut self, cidr: &str) -> Self {
        self.exprs.push(format!("ip6 saddr {}", cidr));
        self
    }

    pub fn tcp_dport(mut self, port: u16) -> Self {
        self.exprs.push(format!("tcp dport {}", port));
        self
    }
    pub fn tcp_dport_set(mut self, ports: &[u16]) -> Self {
        let s: Vec<String> = ports.iter().map(|p| p.to_string()).collect();
        self.exprs.push(format!("tcp dport {{ {} }}", s.join(", ")));
        self
    }
    pub fn udp_dport(mut self, port: u16) -> Self {
        self.exprs.push(format!("udp dport {}", port));
        self
    }

    pub fn ct_state(mut self, states: &[&str]) -> Self {
        self.exprs.push(format!("ct state {{ {} }}", states.join(", ")));
        self
    }
    pub fn ct_state_eq(mut self, state: &str) -> Self {
        self.exprs.push(format!("ct state {}", state));
        self
    }

    pub fn meta_mark(mut self, mark: u32) -> Self {
        self.exprs.push(format!("meta mark 0x{:x}", mark));
        self
    }
    pub fn meta_mark_set(mut self, mark: u32) -> Self {
        self.exprs.push(format!("meta mark set 0x{:x}", mark));
        self
    }

    pub fn lookup_set(mut self, set_name: &str) -> Self {
        self.exprs.push(format!("@{}", set_name));
        self
    }

    pub fn counter(mut self) -> Self {
        self.exprs.push("counter".into());
        self
    }
    pub fn log(mut self, prefix: &str) -> Self {
        self.exprs.push(format!("log prefix \"{}\"", prefix));
        self
    }
    pub fn limit_rate(mut self, rate: u32, unit: &str, per: &str) -> Self {
        self.exprs.push(format!("limit rate {}/{} ", rate, per));
        self
    }

    pub fn verdict(mut self, v: Verdict) -> Self {
        let s = match v {
            Verdict::Accept     => "accept".into(),
            Verdict::Drop       => "drop".into(),
            Verdict::Return     => "return".into(),
            Verdict::Jump(c)    => format!("jump {}", c),
            Verdict::Goto(c)    => format!("goto {}", c),
        };
        self.exprs.push(s);
        self
    }

    pub fn build(self) -> String {
        self.exprs.join(" ")
    }
}

pub struct NftBatch {
    commands: Vec<String>,
}

impl NftBatch {
    pub fn new() -> Self { Self { commands: Vec::new() } }

    pub fn add_table(&mut self, family: &Family, name: &str) {
        self.commands.push(format!("add table {} {}", family_str(family), name));
    }

    pub fn add_chain(
        &mut self,
        family: &Family,
        table: &str,
        chain: &str,
        chain_type: &ChainType,
        hook: &Hook,
        priority: i32,
        policy: &Policy,
    ) {
        self.commands.push(format!(
            "add chain {} {} {} {{ type {} hook {} priority {}; policy {}; }}",
            family_str(family),
            table, chain,
            chaintype_str(chain_type),
            hook_str(hook),
            priority,
            policy_str(policy),
        ));
    }

    pub fn add_rule(&mut self, family: &Family, table: &str, chain: &str, rule: RuleBuilder) {
        self.commands.push(format!(
            "add rule {} {} {} {}",
            family_str(family), table, chain, rule.build()
        ));
    }

    pub fn flush_table(&mut self, family: &Family, table: &str) {
        self.commands.push(format!("flush table {} {}", family_str(family), table));
    }

    pub fn to_script(&self) -> String {
        let mut s = String::from("#!/usr/sbin/nft -f\n\n");
        for cmd in &self.commands {
            s.push_str(cmd);
            s.push('\n');
        }
        s
    }

    pub fn apply(&self) -> std::io::Result<()> {
        use std::process::{Command, Stdio};
        use std::io::Write as _;

        let script = self.to_script();
        let mut child = Command::new("nft")
            .arg("-f").arg("-")
            .stdin(Stdio::piped())
            .spawn()?;

        child.stdin.as_mut().unwrap().write_all(script.as_bytes())?;
        let status = child.wait()?;

        if !status.success() {
            return Err(std::io::Error::new(
                std::io::ErrorKind::Other,
                format!("nft exited with: {}", status),
            ));
        }
        Ok(())
    }
}

fn family_str(f: &Family) -> &str {
    match f {
        Family::IP     => "ip",
        Family::IP6    => "ip6",
        Family::Inet   => "inet",
        Family::Netdev => "netdev",
        Family::Bridge => "bridge",
    }
}
fn chaintype_str(t: &ChainType) -> &str {
    match t { ChainType::Filter => "filter", ChainType::Nat => "nat", ChainType::Route => "route" }
}
fn hook_str(h: &Hook) -> &str {
    match h {
        Hook::Prerouting  => "prerouting",
        Hook::Input       => "input",
        Hook::Forward     => "forward",
        Hook::Output      => "output",
        Hook::Postrouting => "postrouting",
        Hook::Ingress     => "ingress",
        Hook::Egress      => "egress",
    }
}
fn policy_str(p: &Policy) -> &str {
    match p { Policy::Accept => "accept", Policy::Drop => "drop" }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_rule_builder() {
        let rule = RuleBuilder::new()
            .ct_state(&["established", "related"])
            .counter()
            .verdict(Verdict::Accept)
            .build();
        assert_eq!(rule, "ct state { established, related } counter accept");
    }

    #[test]
    fn test_batch_script() {
        let mut batch = NftBatch::new();
        batch.add_table(&Family::Inet, "fw");
        batch.add_chain(
            &Family::Inet, "fw", "input",
            &ChainType::Filter, &Hook::Input, 0, &Policy::Drop,
        );
        batch.add_rule(
            &Family::Inet, "fw", "input",
            RuleBuilder::new().ct_state(&["established"]).verdict(Verdict::Accept),
        );
        let script = batch.to_script();
        assert!(script.contains("add table inet fw"));
        assert!(script.contains("policy drop"));
        assert!(script.contains("ct state { established } accept"));
    }
}
```

---

## 12. Security Threat Model and Mitigations

### Architecture: Trust Zones and Attack Surfaces

```
┌───────────────────────────────────────────────────────────────────────────┐
│  THREAT MODEL: Host with nftables firewall                                │
│                                                                           │
│  EXTERNAL       │    KERNEL nftables       │    LOCAL PROCESSES           │
│  (untrusted)    │    (enforcement point)   │    (partially trusted)       │
│                 │                          │                              │
│  Internet ──────┼──► NF_NETDEV_INGRESS     │                              │
│  (DDoS, scan,   │    (earliest possible    │                              │
│   exploit)      │     filtering)           │                              │
│                 │         │                │                              │
│                 │         ▼                │                              │
│                 │    NF_INET_PRE_ROUTING   │                              │
│  Spoofed src ───┼──► (conntrack/RPF)       │                              │
│                 │         │                │                              │
│                 │         ▼                │                              │
│                 │    FILTER chain          │    ◄── CAP_NET_ADMIN control │
│                 │    (DROP invalid,        │        plane (nft CLI,       │
│                 │     rate limit,          │        nfnetlink socket)     │
│                 │     allow established)   │                              │
│                 │         │                │    ◄── NFQUEUE consumers     │
│  Internal ──────┼──► NF_INET_LOCAL_IN      │        (IDS/IPS daemons)     │
│  (lateral       │    to socket             │                              │
│   movement)     │                          │    ◄── /proc/net/nf_conntrack│
│                 │    NF_INET_FORWARD        │        (conntrack read)      │
│                 │    (inter-VM / pod)       │                              │
└─────────────────┴──────────────────────────┴──────────────────────────────┘
```

### Threat Matrix

```
Threat                    Attack Vector          Mitigation
────────────────────────  ─────────────────────  ──────────────────────────────────────────────────
IP Spoofing               Forged src addr        fib saddr oif check == "eth0" (RPF in nftables)
                                                  or sysctl net.ipv4.conf.all.rp_filter=1

Fragment Reassembly       Overlapping frags      nftables uses conntrack defrag at priority -400
Attack (Teardrop, etc.)   for evasion            (nf_defrag_ipv4/ipv6): complete frags only

TCP SYN Flood             SYN with spoofed src   ct state new limit rate 1000/second
                                                  synproxy with SYN cookies

Port Scan                 Many SYN to closed     reject with tcp reset (fast) or drop (stealth)
                          ports                  nmap -sS → CT state new + log + drop

CT Table Exhaustion       Many NEW connections   nf_conntrack_max tuning
                          to exhaust ct table    nf_conntrack_buckets sizing
                          → DoS all tracking     limit rate on NEW states
                                                 meter { ip saddr limit rate 100/second }

NAT Port Exhaustion       Exhaust ephemeral      per-source connlimit, connlimit-above
                          port range (65535)     64000 ports ÷ max concurrent limit

Conntrack Helper ALG      Exploit in ftp/sip     Disable nf_conntrack_helper sysctl (default off)
Exploit                   helpers (multiple      Assign helpers explicitly: ct helper set
                          historical CVEs)       Isolate helper-using flows in dedicated ct zone

NFQUEUE bypass            Crash/kill NFQUEUE     queue flags bypass: on process crash → drop
                          daemon → packets        queue policy drop (default)
                          bypass IPS

nfnetlink privilege       Unprivileged process   CAP_NET_ADMIN required for NFT_MSG_NEW*
escalation via rule       writes malicious        In containers: never give NET_ADMIN
modification              rule to gain access     User namespaces: restricted nft scope (4.18+)

Connection hijacking      Inject RST or data     tcp-auth (RFC 5925) or TLS above TCP
via TCP state machine     into established CT    ct state invalid drop (drops OOW pkts)

IPv6 extension header     Crafted hop-by-hop     nft: ip6 nexthdr { tcp, udp, icmpv6 } accept
evasion                   headers bypassing       (explicit allowlist vs accept-by-default)
                          L4 match

Connmark persistence      Malicious nfmark       Never expose SO_MARK to unprivileged sockets
                          bypass firewall         Check socket capabilities before trusting mark
                          using skb->mark

Time-of-check-            New connection enters  Use nft transactions, not incremental edits
Time-of-use in            between rule deletion  Atomic swap via generation counter
rule updates              and re-add

cgroup2 firewall          Process escaping       Enforce cgroup membership validation
bypass (Kubernetes)       expected cgroup via    Use UID/GID as well as cgroup meta match
                          exec or namespace op
```

### Control Plane Security

```sh
# Verify no process can modify nftables without CAP_NET_ADMIN
# (check current process capabilities)
capsh --print | grep net_admin

# In containers: never grant net_admin unless required
# docker run --cap-drop=ALL --cap-add=NET_BIND_SERVICE ...

# Protect nftables from user namespace attacks (kernel 4.18+)
# Prevents unprivileged user namespaces from modifying host nftables
sysctl -w kernel.unprivileged_userns_clone=0  # Debian/Ubuntu
# or in nftables, use netns-per-container model (Kubernetes does this)

# Audit nftables rule changes via auditd
auditctl -a always,exit -F arch=b64 -S sendmsg -F uid!=0 -k nftables_mod

# Monitor conntrack table fill level
watch -n1 'cat /proc/sys/net/netfilter/nf_conntrack_count; \
           cat /proc/sys/net/netfilter/nf_conntrack_max'

# Generate an alert when conntrack > 80% full
nf_count=$(cat /proc/sys/net/netfilter/nf_conntrack_count)
nf_max=$(cat /proc/sys/net/netfilter/nf_conntrack_max)
[ $((nf_count * 100 / nf_max)) -gt 80 ] && echo "ALERT: conntrack table >80% full"
```

### nftrace — Packet Tracing (Debugging Without tcpdump)

```sh
# Enable tracing on specific traffic only
nft add rule inet my_fw prerouting \
    ip saddr 1.2.3.4 tcp dport 80 meta nftrace set 1

# Listen to trace events (reads from NFLOG netlink)
nft monitor trace

# Example output:
# trace id 12345678 inet my_fw prerouting rule \
#   ip saddr 1.2.3.4 tcp dport 80 meta nftrace set 1 (verdict continue)
# trace id 12345678 inet my_fw input rule \
#   ct state established,related accept (verdict accept)
```

---

## 13. Production Configurations

### 13.1 Host Hardening Ruleset

```
# /etc/nftables.conf — Complete host hardening ruleset
# Apply: nft -f /etc/nftables.conf
# Test:  nft -c -f /etc/nftables.conf  (dry-run check syntax)

#!/usr/sbin/nft -f

# Atomic flush and reload
flush ruleset

#
# ─── CONSTANTS ────────────────────────────────────────────────────────────────
#
# Permitted management source CIDRs
define MGMT_NETS  = { 10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12 }
define SSH_PORT   = 22
define HTTP_PORT  = 80
define HTTPS_PORT = 443

#
# ─── TABLE ────────────────────────────────────────────────────────────────────
#
table inet host_fw {

    #
    # ── SETS ──────────────────────────────────────────────────────────────────
    #

    # Persistent blocklist (updated at runtime via nft add element)
    set blocklist_v4 {
        type ipv4_addr
        flags interval
        comment "IP blocklist (managed by threat intel feed)"
    }

    set blocklist_v6 {
        type ipv6_addr
        flags interval
        comment "IPv6 blocklist"
    }

    # SSH brute-force rate limiting (dynamic, auto-expires)
    set ssh_bruteforce {
        type ipv4_addr
        flags dynamic, timeout
        timeout 3600s
        comment "SSH brute-force sources, 1h timeout"
    }

    # Allowed services: protocol + port
    set allowed_tcp_dports {
        type inet_service
        flags interval
        elements = { 22, 80, 443, 8080, 9100 }
    }

    set allowed_udp_dports {
        type inet_service
        elements = { 53, 123 }
    }

    #
    # ── RATE LIMITS (named, persistent across rule updates) ───────────────────
    #

    limit ssh_new_limit { rate 5/minute }
    limit icmp_limit    { rate 10/second }

    #
    # ── COUNTERS ──────────────────────────────────────────────────────────────
    #

    counter dropped_invalid   { comment "ct state invalid drops" }
    counter dropped_blocklist { comment "explicit blocklist drops" }
    counter ssh_new_conns     { comment "SSH new connections" }

    #
    # ── CHAINS ────────────────────────────────────────────────────────────────
    #

    # INPUT chain: packets destined for local processes
    chain input {
        type filter hook input priority filter; policy drop;

        # Allow loopback unconditionally
        iif "lo" accept

        # Drop packets claiming to come from loopback (spoofed)
        iif != "lo" ip  saddr 127.0.0.0/8  counter drop
        iif != "lo" ip6 saddr ::1/128       counter drop

        # Reverse path filter for IPv4 (reject spoofed src)
        fib saddr . iif oif missing drop

        # Blocklist: drop early
        ip  saddr @blocklist_v4 counter name dropped_blocklist drop
        ip6 saddr @blocklist_v6 counter name dropped_blocklist drop

        # Drop ct invalid (out-of-state, malformed)
        ct state invalid counter name dropped_invalid log prefix "INVALID " drop

        # Allow established / related
        ct state { established, related } accept

        # ICMP: rate limited
        ip   protocol icmp   icmp   type { echo-request, destination-unreachable,
                                           time-exceeded, parameter-problem } \
                              limit name icmp_limit accept
        ip6 nexthdr  icmpv6  icmpv6 type { echo-request, nd-neighbor-solicit,
                                           nd-neighbor-advert, nd-router-advert,
                                           destination-unreachable, packet-too-big,
                                           time-exceeded, parameter-problem } \
                              limit name icmp_limit accept

        # Drop all other ICMP/ICMPv6 silently (rate exceeded or unexpected type)
        ip  protocol icmp   drop
        ip6 nexthdr  icmpv6 drop

        # SSH: brute-force mitigation
        tcp dport $SSH_PORT ct state new \
            ip saddr @ssh_bruteforce counter drop
        tcp dport $SSH_PORT ct state new \
            limit name ssh_new_limit \
            add @ssh_bruteforce { ip saddr } \
            counter name ssh_new_conns accept

        # Allowed services
        tcp dport @allowed_tcp_dports ct state new accept
        udp dport @allowed_udp_dports accept

        # Reject (not drop) unknown TCP — sends TCP RST, better than silent drop
        tcp flags & (fin|syn|rst|ack) == syn \
            reject with tcp reset

        # Drop everything else (policy drop handles this, but log it)
        log prefix "HOST-INPUT-DROP " level info
    }

    # OUTPUT chain: packets from local processes
    chain output {
        type filter hook output priority filter; policy accept;

        # Allow all local output (could tighten for high-security hosts)
        # For high-security: set policy drop and allowlist services
    }

    # FORWARD chain: packets being routed (off by default for pure hosts)
    chain forward {
        type filter hook forward priority filter; policy drop;
        # Enable only if this host routes traffic (container host, VM host)
    }
}
```

### 13.2 Kubernetes / Container Node Firewall

```
#!/usr/sbin/nft -f
#
# Kubernetes worker node nftables ruleset.
# Coexists with kube-proxy (nftables mode) which manages its own tables.
# This ruleset handles host-level protection only.
#

flush table inet k8s_node_fw

table inet k8s_node_fw {

    set pod_cidrs {
        type ipv4_addr
        flags interval
        comment "Pod CIDR ranges for this node cluster"
        elements = { 10.244.0.0/16, 172.20.0.0/16 }
    }

    set node_ips {
        type ipv4_addr
        comment "All cluster node IPs"
    }

    set k8s_api_servers {
        type ipv4_addr
        comment "Kubernetes API server IPs"
    }

    # Protect kubelet API (port 10250) — only API servers should access
    chain input {
        type filter hook input priority filter - 10; policy accept;

        # Protect kubelet: deny non-cluster-node access to 10250
        tcp dport 10250 ip saddr != @k8s_api_servers \
            ip saddr != @node_ips counter drop

        # Protect kube-proxy metrics (10249) — cluster only
        tcp dport 10249 ip saddr != @node_ips counter drop

        # Protect node-exporter (9100)
        tcp dport 9100 ip saddr != @node_ips counter drop

        # Allow pods to communicate via node IP
        ip saddr @pod_cidrs accept

        # Accept NodePort range (30000-32767) from any (services exposed to internet)
        tcp dport 30000-32767 accept
        udp dport 30000-32767 accept
    }

    # Track cross-node pod traffic for security auditing
    chain forward {
        type filter hook forward priority filter - 10; policy accept;
        # Drop invalid
        ct state invalid drop
        # Log denied cross-pod flows for SIEM
        log prefix "K8S-FORWARD: " group 1
    }
}
```

### 13.3 NAT Gateway / Edge Node

```
#!/usr/sbin/nft -f
#
# NAT gateway: masquerade private network 10.0.0.0/8 through public IP on eth0.
# DDoS mitigation: rate limiting, SYN proxy on public HTTP/HTTPS.
#

flush ruleset

define PUB_IF  = eth0
define PRIV_IF = eth1
define PRIV_NET = 10.0.0.0/8

table inet nat_gw {

    set syn_flood_sources {
        type ipv4_addr
        flags dynamic, timeout
        timeout 300s
    }

    # ── PREROUTING: DNAT (port forwarding) ────────────────────────────────────
    chain prerouting {
        type nat hook prerouting priority dstnat; policy accept;

        # Transparent SYN proxy for DDoS mitigation on HTTP/HTTPS
        iif $PUB_IF tcp dport { 80, 443 } ct state invalid,untracked \
            synproxy mss 1460 wscale 7 timestamp sack-perm

        # Port forwarding: external port 8080 → internal web server
        iif $PUB_IF tcp dport 8080 dnat ip to 10.0.0.10:80
    }

    # ── POSTROUTING: SNAT / Masquerade ────────────────────────────────────────
    chain postrouting {
        type nat hook postrouting priority srcnat; policy accept;

        # Masquerade all private traffic leaving on public interface
        ip saddr $PRIV_NET oif $PUB_IF masquerade
    }

    # ── FORWARD: stateful firewall between zones ───────────────────────────────
    chain forward {
        type filter hook forward priority filter; policy drop;

        # SYN flood protection on public-facing ingress
        iif $PUB_IF tcp flags & (fin|syn|rst|ack) == syn \
            ct state new meter syn_rate { ip saddr limit rate 50/second } \
            add @syn_flood_sources { ip saddr } \
            accept
        iif $PUB_IF ip saddr @syn_flood_sources tcp flags syn drop

        # Allow established and related
        ct state { established, related } accept

        # Allow private → public (outbound)
        iif $PRIV_IF oif $PUB_IF accept

        # Deny public → private (no port-forward rule matched)
        log prefix "FORWARD-DENY "
    }
}

table inet edge_filter {
    chain input {
        type filter hook input priority filter; policy drop;

        iif lo accept
        ct state invalid drop
        ct state { established, related } accept

        # Management SSH from trusted range only
        iif $PUB_IF ip saddr { 203.0.113.0/24 } tcp dport 22 accept

        # BGP (if this is also a router)
        tcp dport 179 ip saddr @bgp_peers accept
    }
}
```

### 13.4 Securing the nftables Control Plane

```sh
# ── 1. Prevent ruleset tampering with immutable sets ──────────────────────
# Mark sets as constant (cannot be modified after creation):
nft add set inet fw blocklist '{ type ipv4_addr; flags constant, interval; }'

# ── 2. Lock down nfnetlink socket access ──────────────────────────────────
# Only processes with CAP_NET_ADMIN can open NETLINK_NETFILTER.
# Check what has this capability in production:
ss -tlnp | grep -v '\b22\b'   # verify only expected services
capsh --decode=$(grep CapEff /proc/$(pgrep -x nft)/status | awk '{print $2}')

# ── 3. Atomic ruleset reload (zero traffic disruption) ────────────────────
# Never: nft flush + nft -f (window of no rules)
# Always: use a single nft -f with flush ruleset at top (atomic in one transaction)
nft -c -f /etc/nftables.conf   # dry-run check
nft -f /etc/nftables.conf      # atomic commit

# ── 4. Save and restore ruleset ───────────────────────────────────────────
nft list ruleset > /etc/nftables.conf.bak.$(date +%Y%m%d_%H%M%S)

# ── 5. Systemd unit for automatic reload ──────────────────────────────────
# /etc/systemd/system/nftables-reload.path:
# [Path]
# PathChanged=/etc/nftables.conf
# [Install]
# WantedBy=multi-user.target

# ── 6. Audit who modifies rules ───────────────────────────────────────────
# auditd rule watching nft binary execution
auditctl -w /usr/sbin/nft -p x -k nft_exec
# Watch nfnetlink socket creation (requires kernel audit support)
auditctl -a always,exit -F arch=b64 -S socket -F a0=16 -F a1=3 -k nfnetlink

# ── 7. Conntrack hardening ─────────────────────────────────────────────────
sysctl -w net.netfilter.nf_conntrack_tcp_loose=0          # strict TCP tracking
sysctl -w net.netfilter.nf_conntrack_tcp_be_liberal=0     # strict window check
sysctl -w net.netfilter.nf_conntrack_helper=0             # explicit helpers only
sysctl -w net.netfilter.nf_conntrack_max=2000000          # size for busy server
sysctl -w net.netfilter.nf_conntrack_buckets=500000       # hash buckets
# Rule of thumb: buckets = max / 4

# ── 8. Validate configuration before deploy ───────────────────────────────
# Test script:
nft --check -f /etc/nftables.conf && \
    echo "Syntax OK" || \
    (echo "SYNTAX ERROR — not applying"; exit 1)
```

---

## 14. Testing, Fuzzing, and Benchmarking

### Unit and Integration Tests

```sh
# ── Test 1: Verify ruleset syntax ────────────────────────────────────────
nft --check -f /etc/nftables.conf

# ── Test 2: Verify expected accept/drop behavior ─────────────────────────
# Use netns for isolation:
ip netns add test_ns
ip link add veth0 type veth peer name veth1
ip link set veth1 netns test_ns
ip addr add 10.0.0.1/24 dev veth0
ip netns exec test_ns ip addr add 10.0.0.2/24 dev veth1
ip link set veth0 up
ip netns exec test_ns ip link set veth1 up

# Apply ruleset in namespace
ip netns exec test_ns nft -f test_ruleset.nft

# Test: should accept (SSH allowed)
ip netns exec test_ns nc -z -w 2 10.0.0.1 22 && echo "PASS: SSH accepted"

# Test: should drop (unexpected port)
ip netns exec test_ns nc -z -w 2 10.0.0.1 8888
[ $? -ne 0 ] && echo "PASS: port 8888 blocked"

# Cleanup
ip netns del test_ns

# ── Test 3: nft trace verification ───────────────────────────────────────
nft add rule inet fw prerouting ip saddr 10.0.0.2 meta nftrace set 1
nft monitor trace &
ip netns exec test_ns ping -c 1 10.0.0.1
kill %1
nft delete rule inet fw prerouting handle $(nft -a list chain inet fw prerouting | \
    grep nftrace | awk '{print $NF}')

# ── Test 4: Conntrack state verification ──────────────────────────────────
# Establish a connection and verify ct state
conntrack -L | grep 10.0.0.2
# Expected: tcp 6 ESTABLISHED src=10.0.0.2 ...

# ── Test 5: Rate limiting verification ────────────────────────────────────
# Flood and check counter
hping3 -S -p 22 --fast 10.0.0.1 &
sleep 2
kill %1
nft list counter inet fw ssh_new_conns
# Verify counter incremented but connection was rate-limited
```

### Performance Benchmarking

```sh
# ── Benchmark 1: Raw packet throughput with pktgen ────────────────────────
# Load pktgen kernel module
modprobe pktgen

# Configure pktgen (write to /proc/net/pktgen/)
pgset() { local result; echo "$1" > /proc/net/pktgen/$PGDEV; }

PGDEV=pgctrl
pgset "reset"

PGDEV=kpktgend_0
pgset "rem_device_all"
pgset "add_device eth0"

PGDEV=eth0
pgset "count 10000000"        # 10M packets
pgset "pkt_size 64"
pgset "delay 0"
pgset "src_min 10.0.0.1"
pgset "src_max 10.0.0.254"
pgset "dst 10.0.1.1"
pgset "dst_mac 00:11:22:33:44:55"
pgset "flag IPSRC_RND"         # Random source IPs
pgset "flag IPDST_RND"

PGDEV=pgctrl
pgset "start"

# Check results
cat /proc/net/pktgen/eth0

# ── Benchmark 2: Rule evaluation with many rules vs set lookup ─────────────
# Create 10000 individual DROP rules (iptables-style anti-pattern)
for i in $(seq 1 10000); do
    nft add rule inet bench input ip saddr 10.0.$((i/256)).$((i%256)) drop
done
# Benchmark
hping3 --fast -c 1000 10.0.0.1 -I eth0
# Time: ~1000μs per packet (O(n) rule scan)

# Reset and use a set instead
nft flush chain inet bench input
nft add set inet bench block_ips '{ type ipv4_addr; flags interval; }'
# Add 10000 entries to the set
for i in $(seq 1 10000); do
    echo "10.0.$((i/256)).$((i%256))"
done | xargs -I{} nft add element inet bench block_ips "{ {} }"
nft add rule inet bench input ip saddr @block_ips drop
# Benchmark same traffic
hping3 --fast -c 1000 10.0.0.1 -I eth0
# Time: ~5μs per packet (O(1) hash lookup)

# ── Benchmark 3: Conntrack performance ────────────────────────────────────
# wrk HTTP benchmark through NAT
wrk -t 12 -c 400 -d 30s http://10.0.0.1/
# Monitor conntrack rate
watch -n0.5 'cat /proc/sys/net/netfilter/nf_conntrack_count'

# ── Benchmark 4: nft rule update latency ──────────────────────────────────
time nft add rule inet fw input \
    ip saddr 1.2.3.4 counter drop
# Should be < 5ms even with 10000-rule table (atomic swap, not linear insert)
```

### Fuzzing

```sh
# ── Fuzz 1: nft parser with AFL++ ────────────────────────────────────────
# Note: nft rule parsing happens in userspace (libnftables).
# Kernel-side fuzzing uses syzkaller.

# Build nft with ASAN + AFL instrumentation
CC=afl-clang-fast ./configure --enable-debug
AFL_USE_ASAN=1 make

mkdir -p fuzz/input
echo 'add table inet t' > fuzz/input/seed1.nft
echo 'add rule inet t c ip saddr 10.0.0.0/8 drop' > fuzz/input/seed2.nft

afl-fuzz -i fuzz/input -o fuzz/output -- \
    /usr/local/sbin/nft --check -f @@

# ── Fuzz 2: Kernel nftables via syzkaller ─────────────────────────────────
# syzkaller has built-in nftables syscall descriptions.
# See: github.com/google/syzkaller/blob/master/sys/linux/netfilter_nftables.txt

# Quick setup for local kernel fuzzing:
git clone https://github.com/google/syzkaller
cd syzkaller && make

cat > cfg.json << 'EOF'
{
  "target": "linux/amd64",
  "http": "127.0.0.1:56741",
  "workdir": "/tmp/syzkaller",
  "kernel_obj": "/path/to/kernel/build",
  "image": "/path/to/vm/image.qcow2",
  "sshkey": "/path/to/vm/key",
  "syzkaller": ".",
  "procs": 4,
  "type": "qemu",
  "vm": { "count": 2, "kernel": "/path/to/bzImage", "cpu": 2, "mem": 2048 },
  "enable_syscalls": ["setsockopt$NETLINK_NETFILTER*", "sendmsg$NETLINK*"]
}
EOF

./bin/syz-manager -config=cfg.json

# ── Fuzz 3: Packet fuzzing through nftables rules ─────────────────────────
# Use scapy to send malformed packets and verify no kernel panic
python3 << 'EOF'
from scapy.all import *
import random

# Malformed IPv4 fragments
for _ in range(1000):
    pkt = IP(
        dst="10.0.0.1",
        src=f"192.168.{random.randint(0,255)}.{random.randint(0,255)}",
        flags="MF",
        frag=random.randint(0, 8189),
        proto=random.randint(0, 255)
    ) / Raw(load=bytes(random.getrandbits(8) for _ in range(random.randint(0,1400))))
    send(pkt, verbose=0)

# TCP with all flag combinations
for flags in range(256):
    pkt = IP(dst="10.0.0.1") / TCP(dport=80, flags=flags, seq=random.randint(0,2**32))
    send(pkt, verbose=0)

print("Fuzzing complete — check dmesg for anomalies")
EOF

# Check for kernel warnings
dmesg | grep -E "WARN|BUG|Oops|general protection"
```

---

## 15. Roll-out and Rollback Plan

### Staged Roll-out

```
Phase 0: Preparation
  □ Inventory current iptables rules: iptables-save > /backup/iptables_$(date +%Y%m%d).txt
  □ Map all iptables rules to nftables equivalents
  □ Peer-review the nftables ruleset
  □ Syntax check: nft --check -f /etc/nftables.conf
  □ Set up monitoring: alert on conntrack table fill > 80%
  □ Backup: nft list ruleset > /backup/nftables_before.txt

Phase 1: Shadow mode (log only)
  □ Deploy nftables rules with ACCEPT policy and LOG prefix
  □ Compare logged drops vs expected traffic in SIEM
  □ Run for 24-72h, validate no legitimate traffic would be dropped
  □ Monitor performance: /proc/net/nf_conntrack_stat

Phase 2: Parallel mode (iptables + nftables)
  □ Do NOT run iptables and nftables on the same hooks simultaneously
    (they both register nf_hook_ops — rules from both apply, order matters)
  □ Migrate one hook at a time:
    - First migrate FORWARD chain (inter-VM, lower blast radius)
    - Then INPUT chain
    - Finally OUTPUT chain
  □ Keep iptables for hooks not yet migrated

Phase 3: Cutover
  □ Disable iptables: systemctl stop iptables; systemctl mask iptables
  □ Enable nftables: systemctl enable --now nftables
  □ Verify: nft list ruleset | grep -c chain

Phase 4: Post-cutover validation
  □ Run full connectivity test suite
  □ Verify conntrack counters incrementing: conntrack -S
  □ Verify nft counters: nft list ruleset | grep packets
  □ Monitor for 24h
```

### Rollback Plan

```sh
# Emergency rollback (< 60 seconds):

# Option A: Restore previous ruleset
nft -f /backup/nftables_before.txt

# Option B: Open all (if you are locked out — requires console access)
nft flush ruleset
nft add table inet emergency
nft add chain inet emergency input \
    '{ type filter hook input priority 0; policy accept; }'
# Then re-apply correct ruleset once you're back in

# Option C: Via systemd (if nftables service manages the rules)
systemctl stop nftables
iptables-restore < /backup/iptables_$(date +%Y%m%d).txt
systemctl start iptables

# Automated rollback with scheduled job (apply rules, rollback if no confirmation):
# 1. Apply new rules
nft -f new_rules.conf
# 2. Schedule rollback in 5 minutes
at now + 5 minutes << 'EOF'
nft -f /backup/nftables_before.txt
logger "nftables auto-rollback triggered"
EOF
# 3. Confirm new rules are OK (cancel the rollback job)
# atrm $(atq | awk '{print $1}')

# Monitoring hooks for auto-rollback integration:
# Check SSH connectivity from external probe
# If fails: trigger immediate rollback via out-of-band management interface
```

---

## 16. References

### Linux Kernel Source (most authoritative)

```
net/netfilter/                  Core netfilter + nftables (all .c files listed in §5.4)
include/linux/netfilter.h       nf_hook_ops, verdict macros, hook numbers
include/linux/netfilter/nf_tables.h  nft_* structs and constants
include/net/netfilter/nf_tables.h   Internal kernel API (nft_expr_ops etc.)
include/net/netfilter/nf_conntrack.h  nf_conn struct
net/netfilter/nf_conntrack_core.c   Conntrack hash + state machine
net/netfilter/nf_nat_core.c         NAT engine core
net/netfilter/nf_flow_table_core.c  Flowtable fast-path
```

### Kernel Documentation

```
Documentation/networking/nf_conntrack-sysctl.rst
Documentation/networking/netfilter-sysctl.rst
Documentation/networking/ip-sysctl.rst  (rp_filter etc.)
```

### Project Sites and Docs

```
https://netfilter.org/             Official netfilter project
https://netfilter.org/projects/nftables/  nftables main page
https://wiki.nftables.org/         nftables wiki (comprehensive syntax reference)
https://www.netfilter.org/projects/libmnl/   libmnl
https://www.netfilter.org/projects/libnftnl/ libnftnl
https://www.netfilter.org/projects/libnftables/ libnftables
```

### Key Papers and Talks

```
Pablo Neira Ayuso: "nftables: the new packet filtering framework" (netdev 0.1, 2015)
  https://netdevconf.info/0.1/docs/nftables-Pablo-Neira.pdf

Florian Westphal: "Conntrack and the challenges of connection tracking at scale"
  (various netdevconf talks, 2018-2022)

Pablo Neira Ayuso: "Flowtable: software and hardware offload" (netdev 2.2, 2017)
  https://netdevconf.info/2.2/papers/neira-flowtable-talk.pdf

Stefano Brivio: "PIPAPO: a multi-value matcher for the nf_tables ruleset"
  https://www.nftables.org/projects/pipapo/
```

### CVEs Relevant to This Architecture

```
CVE-2021-22555  Heap OOB write in nf_conntrack_netlink (privilege escalation)
CVE-2022-25636  Heap OOB write in nf_dup_netdev (privilege escalation)
CVE-2022-32250  nft_object UAF during netlink set (LPE)
CVE-2023-32233  nf_tables anonymous sets UAF (LPE)  ← critical, patched 6.3.1
CVE-2024-1085   nf_tables set element infrastructure UAF (LPE)
CVE-2024-26809  nf_tables double-free in nft_verdict_dump
```

**Mitigation**: Keep kernel updated. Pin kernel ABI with `CONFIG_MODULE_SIG=y`.
Run `CONFIG_LOCKDOWN_LSM=y` (integrity mode) to prevent unsigned module loading.
Monitor `syzbot.appspot.com` for new netfilter findings.

---

## Appendix A: Complete ASCII Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────────────┐
│           COMPLETE NETFILTER / NFTABLES ARCHITECTURE                         │
│                                                                              │
│  ┌──────────┐   DMA   ┌──────────────────────────────────────────────────┐  │
│  │  NIC/HW  │────────►│              Linux Network Stack                  │  │
│  └──────────┘         │                                                    │  │
│                        │  ┌─────────────────────────────────────────────┐ │  │
│  ┌──────────────────┐  │  │ XDP (eBPF — before sk_buff allocation)      │ │  │
│  │  NFPROTO_NETDEV  │  │  └──────────────────┬──────────────────────────┘ │  │
│  │                  │  │                     │ sk_buff allocated           │  │
│  │  [INGRESS hook]  │◄─┼─────────────────────┘                            │  │
│  │  priority -300   │  │  netif_receive_skb()                              │  │
│  │                  │  │     │ tc ingress (eBPF/TC)                        │  │
│  │  [EGRESS hook]   │  │     ▼                                             │  │
│  │  (5.16+)         │  │  L3 protocol dispatch                             │  │
│  └──────────────────┘  │     │                                             │  │
│                        │     ▼                                             │  │
│  ┌──────────────────────────────────────────────────────────────────────┐ │  │
│  │                   NFPROTO_IPV4 / NFPROTO_IPV6 / NFPROTO_INET        │ │  │
│  │                                                                        │ │  │
│  │  ip_rcv()                                                              │ │  │
│  │       │                                                                │ │  │
│  │       ▼                                                                │ │  │
│  │  [PRE_ROUTING hook]  priority order:                                   │ │  │
│  │       │  -450: raw(NOTRACK)                                            │ │  │
│  │       │  -400: IP defrag reassembly                                    │ │  │
│  │       │  -300: nftables raw chain                                      │ │  │
│  │       │  -200: conntrack (classify/create ct entry)                    │ │  │
│  │       │  -150: mangle                                                  │ │  │
│  │       │  -100: DNAT (destination NAT)                                  │ │  │
│  │       │     0: filter (nftables prerouting filter chain)               │ │  │
│  │       │                                                                │ │  │
│  │       ▼                                                                │ │  │
│  │  ip_route_input() ──── Routing Decision ────────────────────────┐     │ │  │
│  │       │                                                          │     │ │  │
│  │       │ local dest                                     forward   │     │ │  │
│  │       ▼                                                          │     │ │  │
│  │  [LOCAL_IN hook]                                [FORWARD hook]   │     │ │  │
│  │       │  -225: SELinux in                           │  0:filter  │     │ │  │
│  │       │     0: filter                               │            │     │ │  │
│  │       │                                             ▼            │     │ │  │
│  │       ▼                                    [POST_ROUTING hook] ◄─┘     │ │  │
│  │  Transport layer                               │  100: SNAT            │ │  │
│  │  (TCP/UDP/ICMP)                                │  INT_MAX: ct confirm  │ │  │
│  │       │                                        ▼                      │ │  │
│  │       ▼                                  ip_finish_output()            │ │  │
│  │  Socket receive queue                         │ tc egress / GSO        │ │  │
│  │                                               ▼                       │ │  │
│  │  [LOCAL_OUT hook]  ◄──── ip_output()    NIC TX queue → wire           │ │  │
│  │       │  -300: raw                                                     │ │  │
│  │       │  -200: conntrack                                               │ │  │
│  │       │     0: filter                                                  │ │  │
│  │       └──────────────────► routing ──► [POST_ROUTING hook]             │ │  │
│  └──────────────────────────────────────────────────────────────────────┘ │  │
│                                                                              │  │
│  ┌──────────────────────────────────────────────────────────────────────┐  │  │
│  │                   CONNTRACK (nf_conntrack)                            │  │  │
│  │                                                                        │  │  │
│  │  Hash table: nf_conntrack_hash[H(5-tuple)]                            │  │  │
│  │                                                                        │  │  │
│  │  ┌────────────────────────────────────────────────────────────────┐  │  │  │
│  │  │ nf_conn: {                                                       │  │  │  │
│  │  │   tuplehash[ORIGINAL]: src=192.168.1.5:1234 dst=8.8.8.8:53     │  │  │  │
│  │  │   tuplehash[REPLY]:    src=8.8.8.8:53     dst=192.168.1.5:1234 │  │  │  │
│  │  │   status: IPS_ESTABLISHED | IPS_CONFIRMED                       │  │  │  │
│  │  │   proto.tcp.state: TCP_CONNTRACK_ESTABLISHED                    │  │  │  │
│  │  │   ext[NAT]: snat → 1.2.3.4:60000                                │  │  │  │
│  │  │   ext[ACCT]: packets=42 bytes=3820                              │  │  │  │
│  │  │   mark: 0x100                                                    │  │  │  │
│  │  │ }                                                                │  │  │  │
│  │  └────────────────────────────────────────────────────────────────┘  │  │  │
│  └──────────────────────────────────────────────────────────────────────┘  │  │
│                                                                              │  │
│  ┌──────────────────────────────────────────────────────────────────────┐  │  │
│  │                   NFTABLES OBJECT HIERARCHY                           │  │  │
│  │                                                                        │  │  │
│  │  nft_net (per netns) → nft_table[] → nft_chain[] → nft_rule[]        │  │  │
│  │                                                                        │  │  │
│  │  nft_rule: [nft_expr: payload][nft_expr: cmp][nft_expr: immediate]   │  │  │
│  │                                                                        │  │  │
│  │  EVAL LOOP (per packet, softirq context):                             │  │  │
│  │    for each rule:                                                      │  │  │
│  │      for each expr: expr->ops->eval(expr, &regs, pkt)                │  │  │
│  │        if regs.verdict == NFT_BREAK → next rule                       │  │  │
│  │        if regs.verdict == NF_ACCEPT/NF_DROP → done                   │  │  │
│  │                                                                        │  │  │
│  │  REGISTERS: [verdict|data0|data1|data2|data3] (5×16 bytes)           │  │  │
│  │                                                                        │  │  │
│  │  RCU: chain->blob_gen[gencursor] ←──── atomic gencursor swap         │  │  │
│  │       (packets always see consistent ruleset)                         │  │  │
│  └──────────────────────────────────────────────────────────────────────┘  │  │
│                                                                              │  │
│  ┌──────────────────────────────────────────────────────────────────────┐  │  │
│  │                 CONTROL PLANE (userspace)                             │  │  │
│  │                                                                        │  │  │
│  │  nft CLI / libnftables / libnftnl / libmnl                           │  │  │
│  │       │                                                                │  │  │
│  │       │ AF_NETLINK / NETLINK_NETFILTER socket                         │  │  │
│  │       │ nfnetlink: NFNL_SUBSYS_NFTABLES (12)                         │  │  │
│  │       │                                                                │  │  │
│  │       │ BATCH: BATCH_BEGIN → NFT_MSG_NEW* × N → BATCH_END            │  │  │
│  │       │ Atomic commit: all-or-nothing, generation swap on success     │  │  │
│  │       ▼                                                                │  │  │
│  │  nf_tables_api.c (kernel)                                             │  │  │
│  │  Requirements: CAP_NET_ADMIN + correct network namespace              │  │  │
│  └──────────────────────────────────────────────────────────────────────┘  │  │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## Appendix B: Key Kernel Sysctls Reference

```sh
# ── Conntrack ────────────────────────────────────────────────────────────────
net.netfilter.nf_conntrack_max           # max tracked connections (default: 65536)
net.netfilter.nf_conntrack_buckets       # hash table size (should be max/4)
net.netfilter.nf_conntrack_tcp_timeout_established  # TCP established timeout (432000s)
net.netfilter.nf_conntrack_tcp_timeout_time_wait    # TIME_WAIT (120s)
net.netfilter.nf_conntrack_tcp_loose     # 0: strict TCP tracking
net.netfilter.nf_conntrack_helper        # 0: disable global helper auto-assign
net.netfilter.nf_conntrack_log_invalid   # Log invalid packets (0=off, 6=TCP)

# ── Network (affect netfilter behavior) ───────────────────────────────────────
net.ipv4.conf.all.rp_filter              # 1: loose RPF, 2: strict RPF
net.ipv4.conf.all.log_martians           # Log martian packets (spoofed src)
net.ipv4.tcp_syncookies                  # 1: enable SYN cookies (fallback to synproxy)
net.ipv4.ip_forward                      # 1: enable packet forwarding (router/container)

# ── Sizing for high-throughput ────────────────────────────────────────────────
net.netfilter.nf_conntrack_max=2000000
net.netfilter.nf_conntrack_buckets=500000
net.netfilter.nf_conntrack_tcp_timeout_established=86400
net.netfilter.nf_conntrack_tcp_timeout_time_wait=30
net.core.rmem_max=134217728
net.core.wmem_max=134217728
```

---

## Appendix C: Next 3 Steps

**Step 1 — Build mental model with live tracing**

```sh
# Set up a test namespace, apply a ruleset, enable nftrace, send packets.
# Reading the trace output correlating to the architecture above cements the
# hook-priority-conntrack-expression mental model faster than any documentation.

ip netns add lab
ip link add veth0 type veth peer name veth1 netns lab
ip addr add 10.99.0.1/24 dev veth0 && ip link set veth0 up
ip netns exec lab ip addr add 10.99.0.2/24 dev veth1 && ip netns exec lab ip link set veth1 up

# Apply a simple ruleset with trace enabled
ip netns exec lab nft -f - << 'EOF'
table inet lab {
  chain input {
    type filter hook input priority 0; policy drop;
    ip saddr 10.99.0.1 meta nftrace set 1
    ct state established,related accept
    tcp dport 80 accept
  }
}
EOF

ip netns exec lab nft monitor trace &
curl -s http://10.99.0.2/ &  # will fail (nothing listening), but trace is the goal
```

**Step 2 — Write the C kernel module and instrument it**

```sh
# Clone the example kernel module (§10.1), extend it to:
# (a) Log packet headers via trace_printk (see with trace-cmd)
# (b) Add a /proc entry exposing per-src-IP drop counters (use a hash table)
# (c) Make the blocked IP configurable via sysfs without module reload
# This builds deep intuition for sk_buff layout and the hook dispatch path.
```

**Step 3 — Benchmark set vs. rule-list scaling**

```sh
# Reproduce the O(n) vs O(1) benchmark (§14):
# 1. Build a test that adds N rules one by one, measures per-packet processing time
# 2. Rebuild with a set of N elements, measure same traffic
# 3. Plot: rules=100,1000,5000,10000,50000 vs latency
# This makes the pipapo/hash backend value viscerally clear and permanently informs
# your architecture decisions when designing firewall rule systems.
```
