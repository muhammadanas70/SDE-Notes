# sk_buff in Linux Networking: A Complete Deep Dive

> **Target Audience**: Kernel developers, network engineers, and systems programmers who want to understand
> how Linux manages network packets at the lowest level, from hardware interrupt to userspace socket.

---

## Table of Contents

1. [What is sk_buff?](#1-what-is-sk_buff)
2. [Memory Layout and Pointer Architecture](#2-memory-layout-and-pointer-architecture)
3. [The sk_buff Data Structure](#3-the-sk_buff-data-structure)
4. [Where Does Data Come From? (Hardware → sk_buff Path)](#4-where-does-data-come-from)
5. [sk_buff Allocation: kmalloc, slab, and page allocator](#5-sk_buff-allocation)
6. [Core Buffer Operations](#6-core-buffer-operations)
7. [sk_buff Queues and Linked Lists](#7-sk_buff-queues-and-linked-lists)
8. [Cloning vs Copying](#8-cloning-vs-copying)
9. [skb_shared_info: Fragmentation and Paged Data](#9-skb_shared_info)
10. [Reference Counting and Lifetime Management](#10-reference-counting-and-lifetime-management)
11. [The Network Stack: How Each Layer Uses sk_buff](#11-the-network-stack)
12. [Buffer Cleanup and Freeing](#12-buffer-cleanup-and-freeing)
13. [GSO, GRO, TSO: Offload Machinery](#13-gso-gro-tso)
14. [Zero-Copy Techniques](#14-zero-copy-techniques)
15. [C Implementation Samples](#15-c-implementation-samples)
16. [Rust in the Linux Kernel: sk_buff Bindings](#16-rust-implementation-samples)
17. [Mental Model Summary](#17-mental-model-summary)

---

## 1. What is sk_buff?

`sk_buff` (socket buffer) is the **central data structure** of the Linux kernel networking stack. Every network packet — incoming or outgoing — is wrapped in an `sk_buff` as it travels through the kernel. It is **not** simply a raw byte buffer. It is a **metadata container** that *points to* packet data, tracks the packet's position in the protocol stack, carries control information, and manages memory lifetime.

Think of `sk_buff` as an **envelope + manifest**:
- The actual bytes (payload, headers) live in a separate memory region.
- `sk_buff` holds pointers into that region, describing where headers start, where data starts, and where the buffer ends.
- It carries protocol-specific metadata: checksums, timestamps, VLAN tags, routing info, socket owner, etc.

```
sk_buff (the manifest):           Linear Data Buffer (the actual bytes):
┌──────────────────────┐          ┌──────────────────────────────────────────────┐
│  next / prev         │          │ headroom │ L2 hdr │ L3 hdr │ L4 hdr │ data  │
│  sk (owning socket)  │          └──────────────────────────────────────────────┘
│  dev (net device)    │               ↑          ↑                       ↑      ↑
│  head ──────────────────────────────┘          │                       │      │
│  data ──────────────────────────────────────────┘                      │      │
│  tail ─────────────────────────────────────────────────────────────────┘      │
│  end  ─────────────────────────────────────────────────────────────────────────┘
│  len                 │
│  data_len            │
│  protocol            │
│  ip_summed           │
│  skb_shared_info ───────────────────── (at 'end', fragment list, GSO info)
└──────────────────────┘
```

### Why Does This Design Exist?

A naive design would copy the entire packet at each layer (add Ethernet header → copy, add IP header → copy). Linux uses **pointer manipulation** instead. The buffer is allocated with extra space (headroom and tailroom), and each layer manipulates only pointers. Adding an L2 header means moving `data` pointer backward by 14 bytes and writing into headroom — **zero copy, O(1) cost**.

---

## 2. Memory Layout and Pointer Architecture

This is the foundation. You must internalize this layout before anything else makes sense.

```
Physical Memory Layout of an sk_buff's Linear Buffer:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

head
 │
 ▼
 ┌──────────────────────────────────────────────────────────────────────┐
 │          HEADROOM            │   PROTOCOL HEADERS + DATA  │TAILROOM │
 │  (reserved for future hdrs)  │                             │         │
 └──────────────────────────────────────────────────────────────────────┘
 ▲                              ▲                             ▲         ▲
head                           data                          tail      end

 ├────── skb_headroom() ───────┤
                                ├──────────── len ────────────┤
 ├──────────────────────────── skb_end_offset() ─────────────────────────┤
                                                               ├─tail───┤
                                                              room()

After skb_reserve(skb, NET_SKB_PAD):   [used by alloc to push data ptr right]
After skb_put(skb, data_size):          tail moves right by data_size
After skb_push(skb, hdr_size):          data moves left by hdr_size
After skb_pull(skb, hdr_size):          data moves right by hdr_size (consume)
```

### The Four Critical Pointers

| Pointer | Type | Meaning |
|---------|------|---------|
| `head`  | `unsigned char *` | Start of the allocated buffer. Never moves. |
| `data`  | `unsigned char *` | Start of valid packet data. Moves as headers are added/removed. |
| `tail`  | `sk_buff_data_t` (offset or pointer) | End of valid packet data. |
| `end`   | `sk_buff_data_t` | End of the allocated buffer. Never moves. `skb_shared_info` lives here. |

### Derived quantities

```c
// From include/linux/skbuff.h

static inline unsigned int skb_headroom(const struct sk_buff *skb)
{
    return skb->data - skb->head;
}

static inline int skb_tailroom(const struct sk_buff *skb)
{
    return skb_is_nonlinear(skb) ? 0 : skb->end - skb->tail;
}

static inline unsigned int skb_headlen(const struct sk_buff *skb)
{
    return skb->len - skb->data_len;  // bytes in linear portion
}
```

### Why tail and end Are Offsets on 64-bit

On `CONFIG_HAVE_EFFICIENT_UNALIGNED_ACCESS` 64-bit systems, `tail` and `end` are stored as `sk_buff_data_t` which is `__u16` offsets from `head`. This keeps the `sk_buff` structure smaller (fits in fewer cache lines) and avoids pointer arithmetic. The macros `skb_tail_pointer()` and `skb_end_pointer()` convert these back to actual pointers.

```c
#ifdef NET_SKBUFF_DATA_USES_OFFSET
typedef unsigned int sk_buff_data_t;
static inline unsigned char *skb_tail_pointer(const struct sk_buff *skb)
{
    return skb->head + skb->tail;
}
#else
typedef unsigned char *sk_buff_data_t;
static inline unsigned char *skb_tail_pointer(const struct sk_buff *skb)
{
    return skb->tail;
}
#endif
```

---

## 3. The sk_buff Data Structure

Here is the actual `sk_buff` struct from `include/linux/skbuff.h`, annotated in full:

```c
struct sk_buff {
    union {
        struct {
            /* Doubly-linked list for sk_buff_head queues */
            struct sk_buff      *next;
            struct sk_buff      *prev;

            union {
                /* Socket that owns this skb (or NULL for forwarded pkts) */
                struct sock     *sk;
                int             ip_defrag_offset;
            };
        };
        struct rb_node          rbnode;  /* used in TCP send queue */
        struct list_head        list;    /* used in some queue implementations */
        struct llist_node       ll_node; /* lockless list node */
    };

    union {
        struct net_device   *dev;   /* RX: the device the pkt arrived on
                                       TX: the device to send the pkt out */
        unsigned long       dev_scratch;
    };

    /*
     * ── TRANSPORT LAYER HEADERS ──────────────────────────────────────────
     * Union of pointers to various transport headers.
     * These are set by the relevant protocol handler.
     */
    union {
        struct tcphdr       *th;
        struct udphdr       *uh;
        struct icmphdr      *icmph;
        struct igmphdr      *igmph;
        struct iphdr        *ipiph;
        struct ipv6hdr      *ipv6h;
        unsigned char       *raw;
    } h;  /* transport header */

    /* Network layer (L3) header pointer */
    union {
        struct iphdr        *iph;
        struct ipv6hdr      *ipv6h;
        struct arphdr       *arph;
        unsigned char       *raw;
    } nh;  /* network header */

    /* Link layer (L2) header pointer */
    union {
        struct ethhdr       *ethernet;
        unsigned char       *raw;
    } mac;  /* mac header */

    /*
     * In newer kernels (5.x+), the header pointers are stored as
     * signed 16-bit offsets from skb->head:
     */
    sk_buff_data_t          transport_header;
    sk_buff_data_t          network_header;
    sk_buff_data_t          mac_header;

    /* ── BUFFER POINTERS ─────────────────────────────────────────────── */
    unsigned char           *head;      /* start of allocated buffer    */
    unsigned char           *data;      /* start of valid packet data   */
    sk_buff_data_t          tail;       /* end of valid packet data     */
    sk_buff_data_t          end;        /* end of allocated buffer      */

    /* ── LENGTH FIELDS ──────────────────────────────────────────────── */
    unsigned int            len;        /* total length of all data:
                                           linear + paged fragments     */
    unsigned int            data_len;   /* bytes in paged fragments only */
    __u16                   mac_len;    /* length of MAC header          */
    __u16                   hdr_len;    /* writable header length (clone)*/

    /* ── CHECKSUMS ──────────────────────────────────────────────────── */
    union {
        __wsum              csum;
        struct {
            __u16           csum_start;  /* offset from head to start checksum */
            __u16           csum_offset; /* offset from csum_start to place result */
        };
    };
    __u8                    ip_summed;  /* checksum state:
                                           CHECKSUM_NONE / PARTIAL / COMPLETE /
                                           UNNECESSARY                  */
    __u8                    csum_valid; /* csum is known valid           */

    /* ── PACKET METADATA ────────────────────────────────────────────── */
    __u16                   queue_mapping;  /* for multiqueue NICs       */
    __u8                    cloned:1,       /* head may be shared        */
                            nohdr:1,        /* payload reference only    */
                            fclone:2,       /* skb was allocated from fclone cache */
                            peeked:1,       /* skb was peeked             */
                            head_frag:1,    /* skb->head is a page frag   */
                            pfmemalloc:1,   /* was alloc from PFMEMALLOC reserve */
                            pp_recycle:1;   /* page_pool recycling        */

    /* ── PROTOCOL AND ROUTING ──────────────────────────────────────── */
    __be16                  protocol;   /* L3 protocol (ETH_P_IP, ETH_P_IPV6...) */
    __u16                   transport_header_valid:1;
    __u8                    encapsulation:1; /* inner headers are valid   */

    /* ── TIMESTAMPS ─────────────────────────────────────────────────── */
    ktime_t                 tstamp;     /* hardware or software RX timestamp */

    /* ── MARK AND PRIORITY ──────────────────────────────────────────── */
    __u32                   mark;       /* generic packet mark (iptables, tc) */
    __u32                   priority;   /* queueing discipline priority   */

    /* ── REFERENCE COUNTING ─────────────────────────────────────────── */
    refcount_t              users;      /* user count; kfree_skb when 0  */

    /* ── NETWORK NAMESPACE ──────────────────────────────────────────── */
    /* Accessed via skb_net(skb) */

    /* ... many more fields for VLAN, SEC, conntrack, XDP, etc. ... */
};
```

### skb Control Block (cb[])

```c
    char    cb[48] __aligned(8);
```

This is a 48-byte scratch pad that **each protocol layer owns temporarily**. As the packet moves through layers, each layer writes its own metadata here. The layout is cast via layer-specific structs:

```c
/* TCP uses cb[] as tcp_skb_cb: */
struct tcp_skb_cb {
    __u32   seq;         /* starting sequence number     */
    __u32   end_seq;     /* SEQ + FIN + SYN + datalen    */
    union { ... };
    __u8    tcp_flags;   /* TCP header flags             */
    __u8    sacked;      /* SACK / RETRANS / LOST flags  */
    ...
};
#define TCP_SKB_CB(__skb)   ((struct tcp_skb_cb *)&((__skb)->cb[0]))

/* IPv4 routing uses cb[] as inet_skb_parm: */
struct inet_skb_parm {
    int                 iif;        /* incoming interface           */
    struct ip_options   opt;        /* parsed IP options            */
    u16                 flags;
    u16                 frag_max_size;
};
#define IPCB(skb)  ((struct inet_skb_parm *)&((skb)->cb[0]))
```

This is a **zero-copy metadata overlay**: no allocation, just a reinterpretation of the same 48 bytes by each layer.

---

## 4. Where Does Data Come From?

The path from wire to sk_buff has several stages. Understanding all of them is critical.

```
HARDWARE/DRIVER PATH (RX - Receive):

  Network Cable / WiFi
        │
        ▼
  ┌──────────────────────────────────────────────────┐
  │  NIC Hardware (e.g., Intel igb, Mellanox mlx5)   │
  │                                                  │
  │  DMA Ring Buffer (RX descriptor ring)            │
  │  ┌──────┬──────┬──────┬──────┬──────┐           │
  │  │ desc │ desc │ desc │ desc │ desc │  ← ring   │
  │  └──┬───┴──────┴──────┴──────┴──────┘           │
  │     │ each desc holds: DMA addr, len, status     │
  └─────┼────────────────────────────────────────────┘
        │
        │  NIC DMAs raw bytes into pre-allocated page(s)
        │  pointed to by the descriptor
        ▼
  ┌──────────────────────────────────────────────────┐
  │  Pre-allocated DMA-coherent pages                │
  │  (filled by hardware without CPU involvement)    │
  └──────────────────────────────────────────────────┘
        │
        │  Hardware raises IRQ (or NAPI poll clears pending)
        ▼
  ┌──────────────────────────────────────────────────┐
  │  Hard IRQ Handler: driver's irq_handler()        │
  │  - Acknowledges interrupt                        │
  │  - Schedules NAPI poll (napi_schedule())         │
  │  - Disables further IRQ for this NIC             │
  └──────────────────────────────────────────────────┘
        │
        ▼
  ┌──────────────────────────────────────────────────┐
  │  Softirq: NET_RX_SOFTIRQ                         │
  │  net_rx_action() → napi_poll() → driver's poll() │
  │                                                  │
  │  For each completed descriptor:                  │
  │    1. napi_alloc_skb() or build_skb()            │
  │    2. skb->data points into the DMA'd page       │
  │    3. skb_put(skb, pkt_len) sets tail            │
  │    4. skb->dev = net_device                      │
  │    5. eth_type_trans() sets protocol             │
  │    6. napi_gro_receive() or netif_receive_skb()  │
  └──────────────────────────────────────────────────┘
        │
        ▼
  ┌──────────────────────────────────────────────────┐
  │  netif_receive_skb() → __netif_receive_skb()     │
  │  - Runs RX packet handlers (tcpdump hooks here)  │
  │  - Finds the right protocol handler via          │
  │    ptype_base hash table (keyed on ETH_P_*)      │
  │  - Calls ip_rcv() for IPv4 packets               │
  └──────────────────────────────────────────────────┘
        │
        ▼
  ┌──────────────────────────────────────────────────┐
  │  Protocol Stack (ip_rcv → tcp_v4_rcv → ...)     │
  └──────────────────────────────────────────────────┘
        │
        ▼
  ┌──────────────────────────────────────────────────┐
  │  Socket receive queue: sk->sk_receive_queue      │
  │  (sk_buff_head, a doubly-linked list)            │
  └──────────────────────────────────────────────────┘
        │
        ▼
  ┌──────────────────────────────────────────────────┐
  │  Userspace: recv() / read() → copy_to_user()     │
  └──────────────────────────────────────────────────┘
```

### Stage 1: DMA Ring Descriptors

The driver pre-populates a **ring buffer** of RX descriptors before packets arrive. Each descriptor tells the NIC: "when a packet arrives, DMA it here." The NIC writes directly to RAM without CPU involvement.

```c
/* Simplified igb-style RX descriptor */
struct e1000_rx_desc {
    __le64  buffer_addr;   /* DMA address where NIC writes packet bytes */
    __le16  length;        /* filled by NIC: bytes written */
    __le16  csum;          /* filled by NIC: checksum (if HW offload) */
    __u8    status;        /* filled by NIC: DD bit = descriptor done */
    __u8    errors;
    __le16  special;
};

/* During driver init, for each ring slot: */
static int igb_alloc_rx_buffers(struct igb_ring *rx_ring, u16 cleaned_count)
{
    struct sk_buff *skb;
    dma_addr_t dma;

    /* Allocate a page (or skb) and map it for DMA */
    skb = netdev_alloc_skb_ip_align(rx_ring->netdev, rx_ring->rx_buf_len);
    dma = dma_map_single(dev, skb->data, rx_ring->rx_buf_len, DMA_FROM_DEVICE);

    /* Give the DMA address to the hardware descriptor */
    rx_desc->buffer_addr = cpu_to_le64(dma);
    /* NIC now knows where to write incoming packet bytes */
}
```

### Stage 2: NAPI — New API for Polling

The **NAPI** (New API) mechanism replaced the old IRQ-per-packet model. With pure IRQs, a 10Gbps link floods the CPU with millions of interrupts/sec. NAPI solves this with **interrupt coalescing + polling**:

```
1st packet arrives:  IRQ fires → disable IRQ → schedule NAPI poll
Poll runs:           process up to budget (e.g., 64) packets
No more packets:     re-enable IRQ, exit poll
More packets:        continue polling (IRQ stays disabled)
```

```c
/* Driver registers a NAPI poll function */
netif_napi_add(netdev, &adapter->napi, igb_poll, 64);

/* The poll function runs in softirq context */
static int igb_poll(struct napi_struct *napi, int budget)
{
    int work_done = 0;

    while (work_done < budget) {
        /* Check if NIC has completed a descriptor */
        if (!(rx_desc->status & E1000_RXD_STAT_DD))
            break;

        /* Build sk_buff from pre-allocated page */
        skb = build_skb(page_address(rx_buf->page) + rx_buf->page_offset,
                        truesize);
        skb_reserve(skb, NET_SKB_PAD + NET_IP_ALIGN);
        skb_put(skb, length);

        /* Unmap DMA */
        dma_unmap_single(...);

        /* Determine L3 protocol from Ethernet type field */
        skb->protocol = eth_type_trans(skb, netdev);

        /* Hand off to protocol stack */
        napi_gro_receive(napi, skb);

        work_done++;
    }

    if (work_done < budget)
        napi_complete_done(napi, work_done); /* re-enable IRQ */

    return work_done;
}
```

### Stage 3: build_skb() vs. netdev_alloc_skb()

There are two main paths to create an sk_buff for received data:

**`netdev_alloc_skb()`**: Allocates both `sk_buff` and linear data buffer together from the slab cache. The NIC DMAs into `skb->data`.

**`build_skb()`**: The NIC DMAs into a raw page (via page pool). After the DMA completes, `build_skb()` creates an `sk_buff` that wraps that page. This is the modern **zero-copy** path — the packet bytes move from NIC to RAM once, never copied again.

```c
/**
 * build_skb - build a network buffer
 * @data: data buffer provided by caller (DMA page)
 * @frag_size: size of data (or 0 if head_frag is set)
 *
 * Allocates only the sk_buff struct (from slab).
 * Points sk_buff->data directly at the DMA'd memory.
 * No copy of packet bytes occurs.
 */
struct sk_buff *build_skb(void *data, unsigned int frag_size)
{
    struct skb_shared_info *shinfo;
    struct sk_buff *skb;
    unsigned int size = frag_size ? : ksize(data);

    skb = kmem_cache_alloc(skbuff_head_cache, GFP_ATOMIC);
    if (unlikely(!skb))
        return NULL;

    size -= SKB_DATA_ALIGN(sizeof(struct skb_shared_info));

    memset(skb, 0, offsetof(struct sk_buff, tail));
    skb->truesize = SKB_TRUESIZE(size);
    refcount_set(&skb->users, 1);
    skb->head = data;
    skb->data = data;
    skb_reset_tail_pointer(skb);
    skb->end = skb->tail + size;
    skb->mac_header = (typeof(skb->mac_header))~0U;
    skb->transport_header = (typeof(skb->transport_header))~0U;

    /* Initialize skb_shared_info at skb->end */
    shinfo = skb_shinfo(skb);
    memset(shinfo, 0, offsetof(struct skb_shared_info, frags));
    atomic_set(&shinfo->dataref, 1);

    return skb;
}
```

### Page Pool: The Modern Receive Path

The Linux kernel's **page pool** is a per-NIC, per-CPU pool of pre-allocated DMA pages. It replaces the old `alloc_page() + dma_map()` per-packet pattern:

```
Page Pool Architecture:

  ┌─────────────────────────────────────────────────────┐
  │                 Page Pool (per NIC queue)           │
  │                                                     │
  │  ┌──────────┐  ┌──────────┐  ┌──────────┐         │
  │  │  Page 0  │  │  Page 1  │  │  Page 2  │  ...    │
  │  │ DMA-mapped│  │ DMA-mapped│  │ DMA-mapped│        │
  │  └──────────┘  └──────────┘  └──────────┘         │
  │                                                     │
  │  When packet arrives:  pool gives page to NIC desc │
  │  When skb freed:       page returns to pool        │
  │  (via page_pool_put_page, not put_page)            │
  └─────────────────────────────────────────────────────┘

  Key property: pages are RECYCLED, avoiding allocator overhead
```

---

## 5. sk_buff Allocation

### The slab/slub Caches

The kernel maintains two dedicated slab caches for sk_buff:

```c
/* From net/core/skbuff.c */

/* Cache for sk_buff structs themselves */
static struct kmem_cache *skbuff_head_cache __ro_after_init;

/* Cache for sk_buff + skb_shared_info + small data area
 * Used by alloc_skb() for small packets */
static struct kmem_cache *skbuff_fclone_cache __ro_after_init;

void __init skb_init(void)
{
    skbuff_head_cache = kmem_cache_create_usercopy(
        "skbuff_head_cache",
        sizeof(struct sk_buff), 0,
        SLAB_HWCACHE_ALIGN | SLAB_PANIC,
        offsetof(struct sk_buff, cb),
        sizeof_field(struct sk_buff, cb),
        NULL);

    skbuff_fclone_cache = kmem_cache_create(
        "skbuff_fclone_cache",
        2 * sizeof(struct sk_buff) +      /* original + one clone slot */
        sizeof(atomic_t),
        0,
        SLAB_HWCACHE_ALIGN | SLAB_PANIC,
        NULL);
}
```

### alloc_skb() — The Core Allocator

```c
/**
 * __alloc_skb - allocate a network buffer
 * @size:   size of the data area (excluding skb_shared_info)
 * @gfp_mask: allocation mask (GFP_ATOMIC for IRQ context, GFP_KERNEL for process)
 * @flags:  0, SKB_ALLOC_FCLONE (for clone-ready), SKB_ALLOC_RX (for receive path)
 * @node:   NUMA node (-1 for any)
 */
struct sk_buff *__alloc_skb(unsigned int size, gfp_t gfp_mask,
                             int flags, int node)
{
    struct kmem_cache *cache;
    struct sk_buff *skb;
    unsigned char *data;
    bool pfmemalloc;
    u8 *ptr;

    /* Choose between normal and fclone cache */
    cache = (flags & SKB_ALLOC_FCLONE)
        ? skbuff_fclone_cache : skbuff_head_cache;

    /* 1. Allocate the sk_buff descriptor from slab */
    skb = kmem_cache_alloc_node(cache, gfp_mask & ~__GFP_DMA, node);
    if (unlikely(!skb))
        return NULL;
    prefetchw(skb);

    /* 2. Round size up for skb_shared_info alignment */
    size = SKB_DATA_ALIGN(size);
    size += SKB_DATA_ALIGN(sizeof(struct skb_shared_info));

    /* 3. Allocate the linear data buffer with kmalloc */
    data = kmalloc_reserve(&size, gfp_mask, node, &pfmemalloc);
    if (unlikely(!data))
        goto nodata;

    /* 4. Initialize the sk_buff */
    /* memset only the hot fields (up to 'tail') */
    memset(skb, 0, offsetof(struct sk_buff, tail));
    skb->truesize = SKB_TRUESIZE(size);
    skb->pfmemalloc = pfmemalloc;
    refcount_set(&skb->users, 1);
    skb->head = data;
    skb->data = data;
    skb_reset_tail_pointer(skb);   /* tail = data */
    skb->end  = skb->tail + size - SKB_DATA_ALIGN(sizeof(struct skb_shared_info));

    /* 5. Initialize skb_shared_info at the end of the buffer */
    skb->mac_header = (typeof(skb->mac_header))~0U;
    skb->transport_header = (typeof(skb->transport_header))~0U;
    atomic_set(&skb_shinfo(skb)->dataref, 1);
    skb_shinfo(skb)->nr_frags = 0;

    return skb;

nodata:
    kmem_cache_free(cache, skb);
    return NULL;
}
```

### Memory Layout After alloc_skb(size=1500, GFP_KERNEL)

```
kmalloc'd region (one contiguous allocation):
┌────────────────────────────────────────────────────────────┐
│ ← SKB_DATA_ALIGN(1500) = 1504 bytes →│← skb_shared_info →│
│                                       │ (nr_frags, gso_*)  │
│ [packet data goes here]               │                    │
└────────────────────────────────────────────────────────────┘
▲                                       ▲                    ▲
head=data=tail                          end                  (end + sizeof shinfo)

skb->head = data_region
skb->data = data_region      (initially same as head, no headroom yet)
skb->tail = data_region      (no data yet, tail == data)
skb->end  = data_region + 1504

After skb_reserve(skb, NET_SKB_PAD):   (e.g. NET_SKB_PAD = 64 bytes)
skb->data += 64
skb->tail += 64  (tail tracks data)
headroom = 64 bytes now available for headers
```

---

## 6. Core Buffer Operations

These are the fundamental operations every protocol layer uses. They are **pointer arithmetic only** — no data is copied.

### skb_reserve() — Reserve Headroom

Called **once** after allocation, before any data is added. Pushes `data` and `tail` forward to create headroom for headers that will be prepended later (by lower layers on TX, or already present on RX).

```c
static inline void skb_reserve(struct sk_buff *skb, int len)
{
    skb->data += len;
    skb->tail += len;
    /* len is ADDED to both, so no data region changes size */
    /* headroom grows by len, tailroom shrinks by len */
}

/* Usage on TX path (socket to wire): */
skb = alloc_skb(MAX_HEADER + payload_len, GFP_KERNEL);
/*                 ↑ MAX_HEADER = space for all headers (Eth+IP+TCP = ~54 bytes) */
skb_reserve(skb, MAX_HEADER);     /* push data ptr past reserved header space */
/* Now data points to where payload should start */
skb_put(skb, payload_len);        /* extend tail by payload_len */
memcpy(skb->data, payload, payload_len);
```

### skb_put() — Add Data at Tail

Used after reserve to claim the data area. Extends the valid data region at the tail.

```c
static inline void *skb_put(struct sk_buff *skb, unsigned int len)
{
    void *tmp = skb_tail_pointer(skb);  /* save current tail */
    SKB_LINEAR_ASSERT(skb);
    skb->tail += len;
    skb->len  += len;
    if (unlikely(skb->tail > skb->end))
        skb_over_panic(skb, len, __builtin_return_address(0));
    return tmp;  /* returns pointer to where data was written */
}

/* Common usage: */
unsigned char *dest = skb_put(skb, data_len);
memcpy(dest, src_data, data_len);

/* Or with zeroing: */
void *dest = skb_put_zero(skb, len);
```

### skb_push() — Prepend Header

Used on **TX path** when a lower layer adds its header. Moves `data` pointer backward.

```c
static inline void *skb_push(struct sk_buff *skb, unsigned int len)
{
    skb->data -= len;
    skb->len  += len;
    if (unlikely(skb->data < skb->head))
        skb_under_panic(skb, len, __builtin_return_address(0));
    return skb->data;
}

/* Example: TCP adds its header, then IP adds its header */

/* TCP layer: */
struct tcphdr *th = skb_push(skb, sizeof(struct tcphdr));
th->source = htons(sport);
th->dest   = htons(dport);
/* ... fill rest of TCP header ... */

/* IP layer (runs after TCP): */
struct iphdr *iph = skb_push(skb, sizeof(struct iphdr));
iph->version = 4;
iph->ihl     = 5;
iph->tot_len = htons(skb->len);
/* ... fill rest of IP header ... */

/* Ethernet layer (runs after IP): */
struct ethhdr *eth = skb_push(skb, ETH_HLEN);
memcpy(eth->h_dest, dst_mac, ETH_ALEN);
memcpy(eth->h_source, src_mac, ETH_ALEN);
eth->h_proto = htons(ETH_P_IP);
```

### skb_pull() — Consume Header (RX path)

Used on **RX path** when a layer has processed its header and passes the payload to the next layer up.

```c
static inline void *skb_pull(struct sk_buff *skb, unsigned int len)
{
    return skb_pull_inline(skb, len);
}

static inline void *skb_pull_inline(struct sk_buff *skb, unsigned int len)
{
    return unlikely(len > skb->len) ? NULL : __skb_pull(skb, len);
}

static inline unsigned char *__skb_pull(struct sk_buff *skb, unsigned int len)
{
    skb->len -= len;
    BUG_ON(skb->len < skb->data_len);
    return skb->data += len;
}

/* Example: IP layer processes its header, hands payload to TCP */
/* After ip_rcv() validates the IP header: */
skb_pull(skb, ip_hdrlen(skb));  /* move data ptr past IP header */
skb_reset_transport_header(skb); /* mark this as transport header start */
/* Now skb->data points to TCP header */
```

### Visual Summary of All Four Operations

```
INITIAL STATE after alloc_skb + skb_reserve(64):
 head                  data=tail                        end
  │                      │                               │
  ▼                      ▼                               ▼
  ├──────────────────────┤───────────────────────────────┤──────────┤
  │    headroom (64B)    │          tailroom             │  shinfo  │
  └──────────────────────┴───────────────────────────────┴──────────┘

AFTER skb_put(payload_len=100):
  ├──────────────────────┤██████████████████│────────────┤──────────┤
  │    headroom (64B)    │   payload(100B)  │  tailroom  │  shinfo  │
  data                                      tail

AFTER skb_push(sizeof(tcphdr)=20):    [data moves LEFT]
  ├──────────┤████████████████████████████████│──────────┤──────────┤
  │ headroom │  tcp_hdr(20) │  payload(100B)  │ tailroom │  shinfo  │
  data                                         tail

AFTER skb_push(sizeof(iphdr)=20):    [data moves LEFT again]
  ├──────┤████████████████████████████████████████│──────┤──────────┤
  │h.r.  │ ip_hdr(20) │ tcp_hdr(20) │ payload(100)│ t.r. │  shinfo  │
  data                                              tail

AFTER skb_push(ETH_HLEN=14):    [data moves LEFT again]
  ├──┤████████████████████████████████████████████████│──┤──────────┤
  │  │ eth(14)│ ip(20) │ tcp(20) │ payload(100)       │  │  shinfo  │
  data                                                   tail
  (headroom now only 16 bytes remaining)
```

### Header Pointer Reset Functions

```c
/* After skb_pull(ip_hlen) on RX: */
skb_reset_transport_header(skb);
/* Sets skb->transport_header = skb->data - skb->head */
/* Now skb_transport_header(skb) returns pointer to TCP hdr */

/* After building Ethernet header on TX: */
skb_reset_mac_header(skb);

/* Access any header by offset: */
static inline struct iphdr *ip_hdr(const struct sk_buff *skb)
{
    return (struct iphdr *)skb_network_header(skb);
}
static inline unsigned char *skb_network_header(const struct sk_buff *skb)
{
    return skb->head + skb->network_header;
}
```

---

## 7. sk_buff Queues and Linked Lists

### sk_buff_head — The Queue Structure

```c
struct sk_buff_head {
    struct sk_buff  *next;    /* same layout as start of sk_buff */
    struct sk_buff  *prev;
    __u32           qlen;     /* number of sk_buffs in queue */
    spinlock_t      lock;     /* protects the queue */
};
```

The head node is a sentinel (dummy) node. The ring looks like:

```
sk_buff_head (sentinel):
┌─────────────────────┐
│ next ───────────────┼────► skb1 ──► skb2 ──► skb3 ──┐
│ prev ───────────────┼──────────────────────────────┐  │
│ qlen = 3            │  ◄───────────────────────────┘  │
└─────────────────────┘                                  │
        ▲                                                 │
        └─────────────────────────────────────────────────┘
(circular doubly-linked list)
```

### Queue Operations

```c
/* Initialize */
void skb_queue_head_init(struct sk_buff_head *list);

/* Enqueue to tail (common: producer appends, consumer pops from head) */
void skb_queue_tail(struct sk_buff_head *list, struct sk_buff *newsk);

/* Dequeue from head */
struct sk_buff *skb_dequeue(struct sk_buff_head *list);

/* Peek without removing */
struct sk_buff *skb_peek(const struct sk_buff_head *list);

/* Purge entire queue (drops all skbs) */
void skb_queue_purge(struct sk_buff_head *list);

/* Iterate */
struct sk_buff *skb;
skb_queue_walk(list, skb) {
    /* process skb */
}
```

### Where Queues Are Used

```
Socket RX:    sk->sk_receive_queue      (TCP: received in-order data ready for recv())
              sk->sk_backlog            (skbs received while socket is locked)
              sk->sk_error_queue        (out-of-band error messages, MSG_ERRQUEUE)

TCP specifics:
              sk->tcp_rtx_queue         (rb_tree: data sent but not yet ACK'd)
              sk->sk_write_queue        (data queued for send, pre-segmentation)

Queueing disciplines (tc):
              sch->q                    (per-qdisc queue, e.g., pfifo, htb)

NIC TX:
              netdev_queue->qdisc->q    (final TX queue before driver)
              txq->_xmit_lock           (per-queue spinlock)
```

---

## 8. Cloning vs Copying

This distinction is essential for performance and correctness.

### skb_clone() — Shallow Copy

Creates a **new sk_buff descriptor** that shares the same data buffer. The data is not copied. Both the original and clone point to the same `head` memory.

```c
struct sk_buff *skb_clone(struct sk_buff *skb, gfp_t gfp_mask)
{
    struct sk_buff_fclones *fclones;
    struct sk_buff *n;

    /* Fast path: use fclone slot if available */
    if (skb->fclone == SKB_FCLONE_ORIG) {
        fclones = container_of(skb, struct sk_buff_fclones, skb1);
        /* atomic compare-and-swap to claim the fclone slot */
        if (atomic_read(&fclones->fclone_ref) == 1)
            /* slot is free, grab it */
            n = &fclones->skb2;
        /* ... */
    } else {
        n = kmem_cache_alloc(skbuff_head_cache, gfp_mask);
    }

    return __skb_clone(n, skb);
}

static struct sk_buff *__skb_clone(struct sk_buff *n, struct sk_buff *skb)
{
#define C(x) n->x = skb->x    /* macro to copy field */

    n->next = n->prev = NULL;
    n->sk = NULL;
    /* Copy all metadata fields: */
    C(tstamp); C(dev); C(transport_header); C(network_header);
    C(mac_header); C(head); C(data); C(truesize);
    /* ... more fields ... */

    /* Bump reference count on the shared data */
    atomic_inc(&(skb_shinfo(skb)->dataref));

    n->hdr_len  = skb->nohdr ? skb_headroom(skb) : skb->hdr_len;
    n->cloned   = 1;
    n->nohdr    = 0;
    n->destructor = NULL;
    /* skb->users is 1 for the clone, independent of original */
    refcount_set(&n->users, 1);

    return n;
}
```

**When is clone used?**
- `tee` and `mirror` in netfilter
- Multicast/broadcast: one sk_buff cloned to multiple sockets
- `AF_PACKET` capture: tcpdump clones each packet
- TCP retransmit: the original stays in retransmit queue; a clone is sent

```
Before clone:

  skb_orig                           data buffer
  ┌──────────────┐                 ┌─────────────────┐
  │ data ────────┼────────────────►│ [eth][ip][tcp]  │
  │ users=1      │                 │ payload bytes   │
  │ cloned=0     │                 └─────────────────┘
  └──────────────┘                   ▲
                                 shinfo.dataref=1

After skb_clone(skb_orig):

  skb_orig                           data buffer
  ┌──────────────┐                 ┌─────────────────┐
  │ data ────────┼────────────────►│ [eth][ip][tcp]  │
  │ users=1      │       ┌────────►│ payload bytes   │
  │ cloned=1     │       │         └─────────────────┘
  └──────────────┘       │           ▲
                         │       shinfo.dataref=2  ← incremented
  skb_clone              │
  ┌──────────────┐       │
  │ data ────────┼───────┘
  │ users=1      │
  │ cloned=1     │
  └──────────────┘
```

**Key rule**: Neither skb_orig nor skb_clone can modify the shared header bytes. If either needs to write to headers, it must call `skb_copy_header()` or `pskb_expand_head()` first ("copy on write for headers").

### skb_copy() — Deep Copy

Allocates a brand new sk_buff and copies **all data** (linear + fragments). Independent lifetimes.

```c
struct sk_buff *skb_copy(const struct sk_buff *skb, gfp_t gfp_mask)
{
    int headerlen = skb_headroom(skb);
    unsigned int size = skb_end_offset(skb) + skb->data_len;

    struct sk_buff *n = __alloc_skb(size, gfp_mask, 0, NUMA_NO_NODE);
    if (!n)
        return NULL;

    /* Reserve the same headroom */
    skb_reserve(n, headerlen);
    /* Copy linear data */
    skb_put(n, skb_headlen(skb));
    /* Copy header area + data */
    BUG_ON(skb_copy_bits(skb, -headerlen, n->head, headerlen + skb->len));

    skb_copy_header(n, skb);
    return n;
}
```

### pskb_copy() — Partial Copy

Copies the sk_buff header and linear data, but **shares** the page fragments (non-linear data). A compromise between clone and full copy, used when you need to modify headers but not payload.

### Copy-on-Write (skb_cow())

```c
/**
 * skb_cow - copy header of skb when it is required
 * If the skb is shared (cloned) and the caller needs to modify headers,
 * this makes a private copy of the header area.
 */
static inline int skb_cow(struct sk_buff *skb, unsigned int headroom)
{
    return __skb_cow(skb, headroom, skb_cloned(skb));
}

int __skb_cow(struct sk_buff *skb, unsigned int headroom, int cloned)
{
    int delta = 0;

    if (headroom > skb_headroom(skb))
        delta = headroom - skb_headroom(skb);

    if (delta || cloned)
        return pskb_expand_head(skb, ALIGN(delta, NET_SKB_PAD), 0, GFP_ATOMIC);
    return 0;
}
```

Used extensively in netfilter: before modifying packet headers (NAT, mangling), check if cloned and COW if needed.

---

## 9. skb_shared_info

At `skb->end` lives `struct skb_shared_info`. It's part of the same kmalloc'd buffer as the linear data.

```c
struct skb_shared_info {
    __u8            flags;          /* various flags */
    __u8            meta_len;
    __u8            nr_frags;       /* number of PAGE fragments */
    __u8            tx_flags;       /* TX metadata for hardware */
    unsigned short  gso_size;       /* GSO segment size */
    unsigned short  gso_segs;       /* GSO segment count */
    struct sk_buff  *frag_list;     /* linked list of skbs for non-linear data */
    struct skb_shared_hwtstamps hwtstamps;  /* hardware timestamps */
    unsigned int    gso_type;       /* GSO type flags */
    u32             tskey;
    atomic_t        dataref;        /* reference count for the data buffer */
    unsigned int    xdp_frags_size; /* XDP multi-buffer */

    /* Scatter-gather page fragments: */
    skb_frag_t      frags[MAX_SKB_FRAGS];  /* MAX_SKB_FRAGS = 17 */
};

/* Each fragment: */
struct skb_frag {
    union {
        struct page    *p;          /* the page */
    } bv_page;
    __u32              bv_offset;   /* offset within the page */
    __u32              bv_len;      /* length of data in this page */
};
```

### Linear + Non-Linear Data Model

```
sk_buff with paged data (e.g., sendfile, splice, large TCP):

  sk_buff
  ┌──────────────────┐
  │ head             │──►┌────────────────────────────────────────┐
  │ data             │──►│ [Eth hdr][IP hdr][TCP hdr][start data] │  ← linear
  │ tail             │   └────────────────────────────────────────┘
  │ end              │──►skb_shared_info
  │ len = 60000      │     ┌─────────────────────────────────────┐
  │ data_len = 59960 │     │ nr_frags = 15                       │
  │                  │     │ dataref = 1                         │
  └──────────────────┘     │ frags[0] → page0, off=0, len=4096  │──►┌────────┐
                           │ frags[1] → page1, off=0, len=4096  │   │ page0  │
                           │ ...                                 │   └────────┘
                           │ frags[14]→ page14, off=0, len=3768 │   ┌────────┐
                           │ frag_list → NULL                   │   │ page1  │
                           └─────────────────────────────────────┘   └────────┘
                                                                       ...

  skb->len = skb_headlen() + skb->data_len
           = 40 (headers) + 59960 (paged) = 60000

  skb_is_nonlinear(skb) == true  (data_len > 0)
```

### frag_list — Chained sk_buffs

When a packet is too large to fit in one sk_buff's fragment list (IP fragmentation, UDP cork), multiple sk_buffs can be chained via `skb_shinfo(skb)->frag_list`:

```
  sk_buff (master)
  ┌──────────────────┐
  │ len = 65535      │
  │ data_len = 64495 │
  │ [IP hdr only]    │
  └──────────────────┘
        │
        └──► skb_shinfo->frag_list:
              ┌──────────┐     ┌──────────┐     ┌──────────┐
              │  skb1    │────►│  skb2    │────►│  skb3    │────►NULL
              │ 21845B   │     │ 21845B   │     │ 20845B   │
              └──────────┘     └──────────┘     └──────────┘
```

This is used by IP fragmentation on TX and IP reassembly on RX. `skb_copy_bits()` handles transparent iteration over linear + fragment + frag_list data.

---

## 10. Reference Counting and Lifetime Management

### Two Reference Counts

sk_buff has **two independent reference counts**, serving different purposes:

#### 1. `skb->users` (refcount_t) — sk_buff struct lifetime

```
skb->users controls when the sk_buff struct itself (and its data) are freed.

Starts at 1 on alloc_skb().
skb_get(skb):     refcount_inc(&skb->users)  — take reference
kfree_skb(skb):   if refcount_dec_and_test(&skb->users) → __kfree_skb(skb)
consume_skb(skb): same, but signals "consumed normally" (vs dropped)
```

#### 2. `skb_shinfo(skb)->dataref` — data buffer lifetime

```
dataref controls when the underlying data buffer (head..end) is freed.

Starts at 1.
skb_clone():      atomic_inc(&skb_shinfo(skb)->dataref)
skb freed:        if atomic_dec_and_test(&skb_shinfo(skb)->dataref) → kfree(skb->head)
```

#### Why Two Counts?

```
Scenario: clone + free original

  Before:           skb_orig (users=1)    skb_clone (users=1)
                          │                     │
                          └──────────┬──────────┘
                                     ▼
                               data buffer (dataref=2)

  kfree_skb(skb_orig):
    users goes 1→0: free skb_orig struct
    dataref goes 2→1: do NOT free data buffer (clone still holds it)

  kfree_skb(skb_clone):
    users goes 1→0: free skb_clone struct
    dataref goes 1→0: NOW free data buffer (last reference)
```

### skb_cloned() and skb_shared()

```c
/* Is the data buffer shared with another sk_buff? */
static inline int skb_cloned(const struct sk_buff *skb)
{
    return skb->cloned &&
           (atomic_read(&skb_shinfo(skb)->dataref) & SKB_DATAREF_MASK) != 1;
}

/* Is skb->users > 1? (rare: manually incremented) */
static inline int skb_shared(const struct sk_buff *skb)
{
    return refcount_read(&skb->users) != 1;
}
```

### skb Destructor

When `kfree_skb()` frees an skb, if `skb->destructor` is set, it is called first:

```c
void kfree_skb_reason(struct sk_buff *skb, enum skb_drop_reason reason)
{
    if (!skb_unref(skb))   /* decrement users, return true if reached 0 */
        return;
    __kfree_skb(skb);
}

void __kfree_skb(struct sk_buff *skb)
{
    skb_release_all(skb);
    kfree_skbmem(skb);
}

static void skb_release_all(struct sk_buff *skb)
{
    /* Call destructor first (e.g., sock_wfree to update socket wmem) */
    if (skb->destructor) {
        WARN_ON(in_hardirq());
        skb->destructor(skb);
    }
    skb_release_data(skb);  /* free/unref data buffer */
}
```

Common destructors:
- `sock_wfree`: decrements socket's `sk_wmem_alloc` (TX buffer accounting)
- `sock_rfree`: decrements socket's `sk_rmem_alloc` (RX buffer accounting)
- `tcp_wfree`: TX completion, potentially wakes up writers

---

## 11. The Network Stack

This is the full journey of an sk_buff through every layer. We cover both RX (receive) and TX (transmit) paths.

### Full RX Stack

```
RECEIVE PATH (NIC → Userspace):

Wire
 │
 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  LAYER 1: NIC + Driver                                               │
│  ─────────────────────────────────────────────────────────────────── │
│  • DMA writes raw bytes into pre-alloc'd page                        │
│  • Driver: build_skb() wraps page → sk_buff                          │
│  • skb->dev = net_device (e.g., eth0)                                │
│  • skb->protocol = eth_type_trans()  (reads EtherType field)         │
│  • napi_gro_receive(napi, skb)       OR  netif_receive_skb(skb)      │
└──────────────────────────────────────────────────────────────────────┘
 │
 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  GRO: Generic Receive Offload (optional, in NAPI context)            │
│  ─────────────────────────────────────────────────────────────────── │
│  • Attempts to coalesce multiple TCP segments into one large skb      │
│  • gro_list: per-NAPI list of partially coalesced flows               │
│  • On flush: passes combined skb to netif_receive_skb()              │
└──────────────────────────────────────────────────────────────────────┘
 │
 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  LAYER 2: netif_receive_skb() — L2 / Link Layer                      │
│  ─────────────────────────────────────────────────────────────────── │
│  • Calls __netif_receive_skb_core()                                   │
│  • Iterates ptype_all list: packet_type handlers (AF_PACKET = tcpdump)│
│  • Bridge / VLAN processing if configured                            │
│  • Looks up ptype_base[hash(protocol)] for L3 handler               │
│  • Calls handler: ip_rcv() for ETH_P_IP                             │
│                   ipv6_rcv() for ETH_P_IPV6                          │
│                   arp_rcv() for ETH_P_ARP                            │
└──────────────────────────────────────────────────────────────────────┘
 │
 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  LAYER 3: ip_rcv() — Network Layer (IPv4)                            │
│  ─────────────────────────────────────────────────────────────────── │
│  1. Validate IP header (length, checksum)                            │
│  2. Run NETFILTER hook: NF_INET_PRE_ROUTING                          │
│     → iptables/nftables PREROUTING chain                            │
│     → conntrack: nf_conntrack_in() — track connection state         │
│  3. ip_rcv_finish():                                                  │
│     a. ip_route_input(): routing decision:                           │
│        - LOCAL: destined for this machine → ip_local_deliver()       │
│        - FORWARD: needs forwarding → ip_forward()                    │
│     b. skb_pull(ip_hdr_len): consume IP header                      │
│     c. NETFILTER hook: NF_INET_LOCAL_IN                              │
│  4. ip_local_deliver_finish():                                        │
│     - Lookup inet_protos[protocol] (e.g., protocol=6 → tcp_v4_rcv)  │
│     - Call tcp_v4_rcv() or udp_rcv()                                 │
└──────────────────────────────────────────────────────────────────────┘
 │
 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  LAYER 4: tcp_v4_rcv() — Transport Layer                             │
│  ─────────────────────────────────────────────────────────────────── │
│  1. Find matching socket: __inet_lookup_skb()                        │
│     - Hash lookup in ehash (established) or bhash (bind) tables     │
│  2. If socket found in TIME_WAIT: tcp_timewait_state_process()       │
│  3. skb_set_owner_r(skb, sk): link skb to socket, update sk_rmem    │
│  4. TCP state machine: tcp_rcv_state_process() or                   │
│                        tcp_rcv_established()                         │
│     a. Validate TCP header (checksum, seq numbers)                  │
│     b. Out-of-order: add to ooo_queue (red-black tree)              │
│     c. In-order: add to sk->sk_receive_queue                        │
│     d. ACK processing: tcp_ack() → tcp_clean_rtx_queue()            │
│     e. Window updates, congestion control (CUBIC, BBR, etc.)        │
│  5. sk->sk_data_ready(sk): wake up sleeping recv() call             │
└──────────────────────────────────────────────────────────────────────┘
 │
 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  SOCKET LAYER: tcp_recvmsg() — syscall recv()                        │
│  ─────────────────────────────────────────────────────────────────── │
│  1. lock_sock(sk)                                                    │
│  2. Dequeue from sk->sk_receive_queue                                │
│  3. copy_to_user(user_buf, skb->data, len)  ← only copy happens here│
│  4. skb_consume_udp() or tcp_eat_recv_skb(): kfree_skb when done    │
│  5. release_sock(sk)                                                 │
└──────────────────────────────────────────────────────────────────────┘
 │
 ▼
Userspace buffer (malloc'd application memory)
```

### Full TX Stack

```
TRANSMIT PATH (Userspace → Wire):

Userspace: write(fd, buf, len) or send(fd, buf, len, 0)
 │
 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  SOCKET LAYER: tcp_sendmsg()                                         │
│  ─────────────────────────────────────────────────────────────────── │
│  1. lock_sock(sk)                                                    │
│  2. copy_from_user() into pages (zero-copy via get_user_pages later) │
│  3. sk_stream_alloc_skb(): allocate sk_buff                          │
│  4. skb_add_data_nocopy() or skb_copy_to_page_nocopy():             │
│     - adds user data as page fragments into skb_shinfo->frags[]     │
│     - avoids copying: pages are pinned from userspace               │
│  5. tcp_push(): send the segment(s)                                  │
│  6. TCP segmentation: split into MSS-sized segments if needed        │
│  7. tcp_transmit_skb(): add TCP header via skb_push()               │
│     - Fills TCP header fields                                        │
│     - Sets skb->destructor = tcp_wfree                              │
│     - Adds to sk->tcp_rtx_queue (retransmit queue)                  │
└──────────────────────────────────────────────────────────────────────┘
 │
 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  LAYER 3: ip_queue_xmit() / ip_output()                              │
│  ─────────────────────────────────────────────────────────────────── │
│  1. Route lookup: ip_route_output() → fills skb->_skb_refdst        │
│  2. ip_local_out():                                                  │
│     a. NETFILTER: NF_INET_LOCAL_OUT (iptables OUTPUT chain)          │
│  3. ip_output():                                                      │
│     a. NETFILTER: NF_INET_POST_ROUTING (iptables POSTROUTING)        │
│     b. ip_finish_output():                                           │
│        - Check if fragmentation needed (skb->len > mtu)             │
│        - If GSO: pass as-is to driver for offload                   │
│        - Else: ip_fragment() → splits into multiple skbs            │
│     c. ip_finish_output2():                                          │
│        - Resolve ARP: neigh_output() → dev_queue_xmit()             │
│        - Prepend Ethernet header via skb_push(ETH_HLEN)             │
└──────────────────────────────────────────────────────────────────────┘
 │
 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  QUEUEING DISCIPLINE (tc / qdisc)                                    │
│  ─────────────────────────────────────────────────────────────────── │
│  dev_queue_xmit() → __dev_queue_xmit()                               │
│  1. Selects the TX queue (txq) based on skb->queue_mapping           │
│  2. Calls qdisc_run(): runs the queueing discipline:                 │
│     - pfifo_fast: simple 3-band FIFO based on TOS/priority          │
│     - htb: Hierarchical Token Bucket (rate limiting)                │
│     - fq_codel: Fair Queuing + CoDel AQM                            │
│  3. sch_direct_xmit(): dequeue from qdisc, call netdev->ndo_start_xmit│
└──────────────────────────────────────────────────────────────────────┘
 │
 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  LAYER 2: Driver ndo_start_xmit()                                    │
│  ─────────────────────────────────────────────────────────────────── │
│  1. Map sk_buff data to DMA addresses:                               │
│     dma_map_single(dev, skb->data, skb_headlen(skb), DMA_TO_DEVICE)  │
│  2. Map page fragments: dma_map_page() for each frag                 │
│  3. Write TX descriptor(s) to NIC ring                               │
│  4. Ring the doorbell: writel(tail_ptr, &hw->tx_tail_reg)           │
│  5. NIC reads descriptor, DMAs data from RAM to TX FIFO             │
│  6. NIC sends frames on wire                                         │
│  7. TX completion interrupt: driver calls dev_consume_skb_any(skb)  │
└──────────────────────────────────────────────────────────────────────┘
 │
 ▼
Wire (electrical/optical signal)
```

### Protocol-Specific cb[] Usage

```
Layer           cb[] struct                 Key Fields
──────────────────────────────────────────────────────────────────────
Ethernet        (none standard)             —
IPv4 RX         inet_skb_parm               iif, opt, flags
IPv4 TX         inet_cork / ipcm_cookie     (in socket, not cb)
IPv6            inet6_skb_parm              iif, nhoff, flags, frag_max
TCP RX          tcp_skb_cb                  seq, end_seq, ack_seq, tcp_flags
TCP TX          tcp_skb_cb                  seq, end_seq, when, sacked
UDP             udp_skb_cb                  cscov, partial_cov
ICMP            (raw, for fragment offset)
VXLAN           vxlan_metadata              vni
MPLS            mpls_nh_via_table           label info
```

---

## 12. Buffer Cleanup and Freeing

### The Complete kfree_skb Path

```c
void kfree_skb_reason(struct sk_buff *skb, enum skb_drop_reason reason)
{
    if (unlikely(!skb))
        return;
    if (likely(refcount_read(&skb->users) == 1))
        smp_rmb();
    else if (!refcount_dec_and_test(&skb->users))
        return;   /* still has references, don't free */

    /* Tracepoint for packet drop analysis */
    trace_kfree_skb(skb, __builtin_return_address(0), reason);

    __kfree_skb(skb);
}

void __kfree_skb(struct sk_buff *skb)
{
    skb_release_all(skb);
    kfree_skbmem(skb);
}

static void skb_release_all(struct sk_buff *skb)
{
    if (skb->destructor) {
        WARN_ON(in_hardirq());
        skb->destructor(skb);   /* sock_wfree, tcp_wfree, etc. */
    }
#if IS_ENABLED(CONFIG_NF_CONNTRACK)
    nf_conntrack_put(skb_nfct(skb));   /* release conntrack reference */
#endif
    skb_release_data(skb);
}

static void skb_release_data(struct sk_buff *skb)
{
    struct skb_shared_info *shinfo = skb_shinfo(skb);

    /* Decrement dataref; if not zero, data is shared → don't free */
    if (!skb->cloned ||
        !atomic_sub_return(skb->nohdr ? (1 << SKB_DATAREF_SHIFT) + 1 : 1,
                           &shinfo->dataref)) {

        /* Free page fragments */
        for (int i = 0; i < shinfo->nr_frags; i++) {
            skb_frag_t *frag = &shinfo->frags[i];
            if (skb_frag_page(frag))
                put_page(skb_frag_page(frag));  /* or page_pool_put_page() */
        }

        /* Free frag_list (recursively frees chained skbs) */
        skb_drop_fraglist(skb);

        /* Free the linear data buffer (kmalloc'd region) */
        if (skb->head_frag)
            put_page(virt_to_page(skb->head));  /* was a page */
        else
            kfree(skb->head);   /* was a kmalloc allocation */
    }
}

static void kfree_skbmem(struct sk_buff *skb)
{
    struct sk_buff_fclones *fclones;

    switch (skb->fclone) {
    case SKB_FCLONE_UNAVAILABLE:
        /* Normal skb: return to slab cache */
        kmem_cache_free(skbuff_head_cache, skb);
        return;

    case SKB_FCLONE_ORIG:
        /* Part of fclone pair: check if clone is also freed */
        fclones = container_of(skb, struct sk_buff_fclones, skb1);
        /* Use atomic to coordinate with clone freeing */
        if (!atomic_dec_return(&fclones->fclone_ref))
            kmem_cache_free(skbuff_fclone_cache, fclones);
        return;
    }
}
```

### How a Layer Signals "I'm Done With This skb"

```c
/* Normal consumption (not a drop): */
consume_skb(skb);        /* kfree_skb with reason=CONSUMED */

/* Drop (packet discarded, not delivered): */
kfree_skb(skb);          /* kfree_skb with reason=NOT_SPECIFIED */
kfree_skb_reason(skb, SKB_DROP_REASON_NO_SOCKET);

/* TCP: data acknowledged, clean retransmit queue: */
tcp_clean_rtx_queue(sk);   /* calls tcp_rtx_queue_unlink_and_free() */
```

### Socket Memory Accounting

The kernel enforces per-socket memory limits to prevent one socket from consuming all RAM:

```c
/* On RX: when adding to sk_receive_queue */
skb_set_owner_r(skb, sk);
    → atomic_add(skb->truesize, &sk->sk_rmem_alloc);
    → skb->destructor = sock_rfree;

/* When recv() consumes the skb (kfree_skb called): */
sock_rfree(skb):
    → atomic_sub(skb->truesize, &sk->sk_rmem_alloc);
    /* Wakes writers if sk_rcvbuf was exceeded */

/* Check: is there room in the receive buffer? */
sk_rmem_alloc_get(sk) < sk->sk_rcvbuf
/* sk_rcvbuf default: net.core.rmem_default = 212992 bytes */
/* Max: net.core.rmem_max */
```

### TCP Retransmit Queue Cleanup

```
TCP RTX Queue (red-black tree, not sk_buff_head):

  sk->tcp_rtx_queue
         │
         ▼ (rb_tree ordered by seq number)
      ┌──────┐
      │ skb1 │ seq=1000, end_seq=2000   ← waiting for ACK
      └──────┘
      ┌──────┐
      │ skb2 │ seq=2000, end_seq=3000   ← waiting for ACK
      └──────┘

When ACK arrives for seq=2500:
  tcp_ack() → tcp_clean_rtx_queue():
    - skb1 fully acked (end_seq=2000 ≤ 2500): remove from tree, kfree_skb
    - skb2 partially acked: trim skb2's data (seq becomes 2500)
    - Update cwnd, RTT estimates
```

---

## 13. GSO, GRO, TSO: Offload Machinery

### TSO — TCP Segmentation Offload (Hardware)

Without TSO, the kernel must segment a large TCP write into MSS-sized (1460B) segments. With TSO, it sends one giant sk_buff to the NIC, and the NIC hardware segments it:

```
Without TSO (CPU does segmentation):
  64KB write → kernel creates ~44 sk_buffs of 1460B each → 44 DMA operations

With TSO (NIC does segmentation):
  64KB write → kernel creates 1 sk_buff of 64KB → 1 DMA operation
  NIC segments into 44 frames on the wire

  skb (TSO):
  ┌──────────────────┐
  │ gso_size = 1460  │   (MSS)
  │ gso_segs = 44    │
  │ gso_type = SKB_GSO_TCPV4 │
  │ len = 65535      │
  └──────────────────┘
```

### GSO — Generic Segmentation Offload (Software Fallback)

GSO is the software implementation of TSO. If the NIC doesn't support TSO (or the packet goes through a virtual interface), GSO segments the sk_buff in software just before handing to the driver:

```c
/* In dev_hard_start_xmit(), before ndo_start_xmit() */
if (netif_needs_gso(skb, features)) {
    struct sk_buff *segs = skb_gso_segment(skb, features);
    /* segs is a linked list of MSS-sized skbs */
    while (segs) {
        struct sk_buff *next = segs->next;
        segs->next = NULL;
        ndo_start_xmit(segs, dev);
        segs = next;
    }
}
```

### GRO — Generic Receive Offload

GRO is the receive-side counterpart. Instead of delivering 44 small TCP segments to the protocol stack, GRO coalesces them into one large skb, reducing per-packet overhead:

```c
/* In NAPI poll, before netif_receive_skb: */
napi_gro_receive(napi, skb);

/* gro_list: per-NAPI list of in-progress GRO coalescing */
struct gro_list {
    struct list_head  list;
    int               count;
};

/* GRO flow:
   1. Check if skb matches an existing GRO flow (same 5-tuple)
   2. If match: extend the existing GRO skb's data_len, update frag list
   3. If no match: start new GRO flow
   4. On napi_complete or gro_flush_oldest: flush coalesced skb to stack
*/
```

---

## 14. Zero-Copy Techniques

Linux uses multiple techniques to avoid copying packet data:

### 1. Receive: build_skb() + page_pool

```
NIC DMA → page_pool page → build_skb() wraps it → tcp: kmap page → copy_to_user
                                                                          ▲
                                                          one copy (page → userspace)
```

### 2. Transmit: sendfile / splice

```c
/* sendfile(): zero-copy file to socket */
/* The file pages are added directly to skb->frags[] */

tcp_sendpage():
  1. get page reference (page_cache_get)
  2. skb_fill_page_desc(): add page to shinfo->frags[]
  3. NO copy of file data occurs
  4. DMA directly maps the file page
```

### 3. MSG_ZEROCOPY (sendmsg with SO_ZEROCOPY)

```c
/* Userspace: */
setsockopt(fd, SOL_SOCKET, SO_ZEROCOPY, &one, sizeof(one));
send(fd, buf, len, MSG_ZEROCOPY);
/* Kernel pins userspace pages, adds to skb frags */
/* Notification via error queue when DMA completes and pages can be reused */
```

### 4. XDP — eXpress Data Path

XDP runs a BPF program at the earliest possible point (driver NAPI, before sk_buff allocation) to allow kernel bypass, redirect, or early drop:

```
Wire → NIC DMA → [XDP BPF program] → decision:
                                        XDP_DROP:    drop in driver, 0 cycles
                                        XDP_PASS:    continue to normal stack
                                        XDP_TX:      retransmit out same interface
                                        XDP_REDIRECT: redirect to different interface/CPU
                                        XDP_ABORTED: drop + trace

When XDP_PASS: build_skb() called, normal stack proceeds
When XDP_DROP: NO sk_buff allocated at all (maximum efficiency)
```

---

## 15. C Implementation Samples

### Sample 1: Writing a Minimal Network Driver RX Path

```c
/*
 * minimal_rx.c — Skeleton of an NIC driver's RX path
 * Demonstrates how raw DMA bytes become an sk_buff.
 */

#include <linux/netdevice.h>
#include <linux/skbuff.h>
#include <linux/dma-mapping.h>
#include <linux/etherdevice.h>

/* Per-RX-ring-slot descriptor metadata (driver-private) */
struct rx_buffer {
    struct sk_buff  *skb;       /* pre-allocated skb */
    dma_addr_t      dma;        /* DMA address given to HW */
};

/* Pre-populate RX ring with empty skbs */
static int allocate_rx_buffer(struct net_device *dev,
                               struct rx_buffer *rxbuf,
                               unsigned int buf_size)
{
    struct sk_buff *skb;
    dma_addr_t dma;

    /*
     * NET_IP_ALIGN = 2: ensures IP header is 4-byte aligned
     * after the 14-byte Ethernet header (14 + 2 = 16, 16-byte aligned)
     */
    skb = netdev_alloc_skb_ip_align(dev, buf_size);
    if (!skb)
        return -ENOMEM;

    /* Map buffer for DMA FROM device (NIC will write here) */
    dma = dma_map_single(dev->dev.parent, skb->data,
                         buf_size, DMA_FROM_DEVICE);
    if (dma_mapping_error(dev->dev.parent, dma)) {
        dev_kfree_skb_any(skb);
        return -ENOMEM;
    }

    rxbuf->skb = skb;
    rxbuf->dma = dma;
    return 0;
}

/* Called from NAPI poll when HW descriptor is done */
static struct sk_buff *process_rx_desc(struct net_device *dev,
                                        struct rx_buffer *rxbuf,
                                        unsigned int pkt_len)
{
    struct sk_buff *skb = rxbuf->skb;

    /* Unmap: CPU now owns the memory (sync for CPU reads) */
    dma_unmap_single(dev->dev.parent, rxbuf->dma,
                     pkt_len, DMA_FROM_DEVICE);

    /* Set valid data length:
     * skb->data already points to start of packet (Ethernet header)
     * skb_put advances tail by pkt_len
     */
    skb_put(skb, pkt_len);

    /* Determine L3 protocol from Ethernet type field.
     * Also: strips Ethernet header from skb (advances skb->data past Eth hdr)
     * and sets skb->mac_header, skb->protocol, skb->pkt_type
     */
    skb->protocol = eth_type_trans(skb, dev);

    /* Update device statistics */
    dev->stats.rx_packets++;
    dev->stats.rx_bytes += pkt_len;

    return skb;
}

/* The NAPI poll function */
static int my_driver_poll(struct napi_struct *napi, int budget)
{
    struct my_adapter *adapter = container_of(napi, struct my_adapter, napi);
    struct net_device *dev = adapter->netdev;
    int work_done = 0;

    while (work_done < budget) {
        struct hw_rx_desc *hw_desc = &adapter->rx_ring[adapter->rx_head];

        /* Check if hardware has completed this descriptor */
        if (!(hw_desc->status & HW_DESC_STATUS_DONE))
            break;

        /* Memory barrier: ensure we read status before reading data */
        rmb();

        struct rx_buffer *rxbuf = &adapter->rx_buffers[adapter->rx_head];
        unsigned int pkt_len = hw_desc->length;

        /* Convert DMA'd data to sk_buff */
        struct sk_buff *skb = process_rx_desc(dev, rxbuf, pkt_len);

        /*
         * Hardware checksum offload:
         * If NIC verified the checksum, set ip_summed to tell the stack
         * it doesn't need to reverify.
         */
        if (hw_desc->status & HW_DESC_STATUS_CSUM_OK)
            skb->ip_summed = CHECKSUM_UNNECESSARY;

        /*
         * Pass to GRO layer (which may coalesce with other skbs)
         * or directly to netif_receive_skb() for immediate processing.
         */
        napi_gro_receive(napi, skb);

        /* Allocate replacement buffer for this ring slot */
        allocate_rx_buffer(dev, rxbuf, adapter->rx_buf_size);

        /* Write new DMA addr to HW descriptor for next packet */
        hw_desc->buffer_addr = cpu_to_le64(rxbuf->dma);
        hw_desc->status = 0;  /* give descriptor back to hardware */

        adapter->rx_head = (adapter->rx_head + 1) % RX_RING_SIZE;
        work_done++;
    }

    /* If we used less than budget: no more work, re-enable IRQ */
    if (work_done < budget) {
        napi_complete_done(napi, work_done);
        /* Re-enable hardware interrupt for this queue */
        writel(IRQ_ENABLE, adapter->hw_base + IRQ_CTRL_REG);
    }

    return work_done;
}
```

### Sample 2: Walking the Protocol Stack — Custom Protocol Handler

```c
/*
 * custom_proto.c — Register a custom L3 protocol handler
 * Demonstrates how sk_buff arrives at an L3 handler and how
 * to consume its data.
 */

#include <linux/module.h>
#include <linux/netdevice.h>
#include <linux/skbuff.h>
#include <linux/if_ether.h>

#define MY_PROTO_TYPE  htons(0x8888)  /* custom EtherType */

/*
 * Our custom protocol header:
 * (follows Ethernet header in the packet)
 */
struct my_proto_hdr {
    __be16  msg_type;
    __be16  msg_len;
    __be32  src_id;
    __be32  dst_id;
} __packed;

/*
 * L3 handler: called by __netif_receive_skb_core() when
 * skb->protocol == MY_PROTO_TYPE
 *
 * At entry:
 *   skb->data   → points past Ethernet header (at our protocol header)
 *   skb->mac_header → start of Ethernet header
 *   skb->len    → length from skb->data to end of packet
 */
static int my_proto_rcv(struct sk_buff *skb,
                         struct net_device *dev,
                         struct packet_type *pt,
                         struct net_device *orig_dev)
{
    struct my_proto_hdr *hdr;
    unsigned int hdr_size = sizeof(struct my_proto_hdr);

    /*
     * If skb is cloned (e.g., tcpdump is running), we need our own
     * copy to safely read headers. pskb_may_pull ensures the first
     * hdr_size bytes are in the linear portion.
     */
    if (!pskb_may_pull(skb, hdr_size))
        goto drop;

    /* Point at our header — skb->data is already past Ethernet hdr */
    hdr = (struct my_proto_hdr *)skb->data;

    pr_debug("my_proto: type=%u len=%u src=%u dst=%u\n",
             ntohs(hdr->msg_type),
             ntohs(hdr->msg_len),
             ntohl(hdr->src_id),
             ntohl(hdr->dst_id));

    /*
     * Set network header offset (for later use by tools like tc, BPF)
     * Currently: skb->data is at our protocol header
     */
    skb_reset_network_header(skb);

    /*
     * Consume our header by advancing data pointer past it.
     * skb->data now points to payload.
     */
    if (skb_pull(skb, hdr_size) == NULL)
        goto drop;

    /*
     * Mark transport header start
     * (payload = "transport layer" in our protocol)
     */
    skb_reset_transport_header(skb);

    /* Process payload... */
    unsigned int payload_len = skb->len;
    unsigned char *payload = skb->data;

    /*
     * Access non-linear data safely:
     * skb_copy_bits handles linear + fragments transparently
     */
    if (skb->data_len > 0) {
        /* Packet has paged data — can't directly dereference */
        unsigned char tmp[128];
        int to_read = min(payload_len, (unsigned int)sizeof(tmp));
        if (skb_copy_bits(skb, 0, tmp, to_read) < 0)
            goto drop;
        /* process tmp... */
    }

    /* Done processing: free the sk_buff */
    consume_skb(skb);  /* "consumed", not dropped — for stats */
    return NET_RX_SUCCESS;

drop:
    kfree_skb(skb);
    return NET_RX_DROP;
}

static struct packet_type my_packet_type = {
    .type = MY_PROTO_TYPE,    /* filter by EtherType */
    .func = my_proto_rcv,
    .dev  = NULL,             /* NULL = receive on all interfaces */
};

static int __init my_proto_init(void)
{
    dev_add_pack(&my_packet_type);
    pr_info("my_proto: registered EtherType 0x8888\n");
    return 0;
}

static void __exit my_proto_exit(void)
{
    dev_remove_pack(&my_packet_type);
}

module_init(my_proto_init);
module_exit(my_proto_exit);
MODULE_LICENSE("GPL");
```

### Sample 3: sk_buff Queue — Consumer/Producer Pattern

```c
/*
 * skb_queue_demo.c — How sk_buff queues work
 * (as used internally by TCP receive path, UDP, etc.)
 */

#include <linux/skbuff.h>
#include <linux/netdevice.h>

struct my_socket_like_struct {
    struct sk_buff_head  receive_queue;   /* ready-to-read skbs */
    struct sk_buff_head  backlog_queue;   /* skbs received while locked */
    spinlock_t           lock;
    wait_queue_head_t    wait;            /* sleeping readers */
};

/* Initialize */
static void my_socket_init(struct my_socket_like_struct *s)
{
    skb_queue_head_init(&s->receive_queue);
    skb_queue_head_init(&s->backlog_queue);
    spin_lock_init(&s->lock);
    init_waitqueue_head(&s->wait);
}

/*
 * Called from softirq (RX path) — must use spin_lock_bh or atomic ops.
 * This is equivalent to how sk_add_backlog() works in net/core/sock.c
 */
static void my_enqueue(struct my_socket_like_struct *s,
                        struct sk_buff *skb)
{
    /* Check receive buffer limit */
    if (skb_queue_len(&s->receive_queue) > 1000) {
        kfree_skb(skb);  /* drop: queue full */
        return;
    }

    /* Lock: protects the queue from concurrent readers (recv syscall) */
    spin_lock(&s->receive_queue.lock);
    __skb_queue_tail(&s->receive_queue, skb);
    spin_unlock(&s->receive_queue.lock);

    /* Wake up any thread sleeping in recv() */
    wake_up_interruptible(&s->wait);
}

/*
 * Called from process context (recv syscall)
 * Equivalent to how tcp_recvmsg() dequeues.
 */
static int my_recv(struct my_socket_like_struct *s,
                   char __user *ubuf, size_t len)
{
    struct sk_buff *skb;
    int copied = 0;

    /* Sleep until data arrives (simplified: no timeout) */
    wait_event_interruptible(s->wait,
        !skb_queue_empty(&s->receive_queue));

    while (copied < len) {
        /* Peek at head of queue (don't dequeue yet) */
        skb = skb_peek(&s->receive_queue);
        if (!skb)
            break;

        unsigned int to_copy = min((size_t)skb->len, len - copied);

        /* Copy to userspace: handles linear + fragment data */
        if (skb_copy_datagram_iter(skb, 0,
                                    &iter,      /* iov_iter pointing to ubuf */
                                    to_copy)) {
            break;  /* error */
        }
        copied += to_copy;

        if (to_copy == skb->len) {
            /* Consumed entire skb: dequeue and free */
            skb_dequeue(&s->receive_queue);
            consume_skb(skb);
        } else {
            /* Partial read: update offset (simplified here) */
            skb_pull(skb, to_copy);
        }
    }

    return copied;
}
```

### Sample 4: Manually Walking sk_buff Data (including fragments)

```c
/*
 * skb_iterate.c — Correctly iterating over ALL sk_buff data
 * (linear + paged fragments + frag_list)
 */

#include <linux/skbuff.h>

/*
 * skb_copy_bits() is the safe API, but let's see what it does internally.
 * This demonstrates the three data regions of an sk_buff.
 */
static int inspect_skb_data(const struct sk_buff *skb)
{
    struct skb_shared_info *shinfo = skb_shinfo(skb);
    int total = 0;

    /* ── Region 1: Linear data (skb->data..skb->tail) ── */
    unsigned int linear_len = skb_headlen(skb);  /* skb->len - skb->data_len */
    pr_info("Linear: %u bytes at %px\n", linear_len, skb->data);
    /* Direct pointer dereference is safe here */
    for (int i = 0; i < min(linear_len, 16u); i++)
        pr_cont("%02x ", skb->data[i]);
    pr_cont("\n");
    total += linear_len;

    /* ── Region 2: Page fragments (shinfo->frags[]) ── */
    for (int i = 0; i < shinfo->nr_frags; i++) {
        const skb_frag_t *frag = &shinfo->frags[i];
        struct page *page = skb_frag_page(frag);
        unsigned int off = skb_frag_off(frag);
        unsigned int sz  = skb_frag_size(frag);

        pr_info("Frag[%d]: page=%px offset=%u size=%u\n",
                i, page, off, sz);

        /* Must kmap to access page data */
        void *kaddr = kmap_local_page(page);
        /* ... access kaddr + off ... */
        kunmap_local(kaddr);

        total += sz;
    }

    /* ── Region 3: frag_list (chained sk_buffs) ── */
    struct sk_buff *frag_skb;
    skb_walk_frags(skb, frag_skb) {
        pr_info("frag_list skb: %u bytes\n", frag_skb->len);
        /* Recursion would handle frag_skb's own fragments */
        total += frag_skb->len;
    }

    pr_info("Total: %u bytes (skb->len=%u)\n", total, skb->len);
    BUG_ON(total != skb->len);
    return 0;
}

/*
 * The RIGHT way: use skb_copy_bits() for arbitrary byte access
 * Works transparently across all three regions.
 */
static void safe_read_bytes(const struct sk_buff *skb,
                             int offset, int len)
{
    unsigned char buf[256];
    if (len > sizeof(buf)) return;

    /*
     * skb_copy_bits(skb, offset_from_data, dst, len)
     * offset: bytes from skb->data (can be negative to go into headroom)
     */
    if (skb_copy_bits(skb, offset, buf, len) < 0) {
        pr_err("skb_copy_bits failed: offset=%d len=%d skb->len=%u\n",
               offset, len, skb->len);
        return;
    }
    /* buf[] now has the bytes regardless of where they physically live */
}
```

### Sample 5: Netfilter Hook — Observing sk_buff in the Stack

```c
/*
 * nf_hook_demo.c — Hooking into the netfilter framework
 * to inspect sk_buff at NF_INET_PRE_ROUTING
 */

#include <linux/module.h>
#include <linux/netfilter.h>
#include <linux/netfilter_ipv4.h>
#include <linux/skbuff.h>
#include <linux/ip.h>
#include <linux/tcp.h>

static unsigned int my_nf_hook(void *priv,
                                 struct sk_buff *skb,
                                 const struct nf_hook_state *state)
{
    struct iphdr *iph;
    struct tcphdr *tcph;

    /* Ensure IP header is in linear data */
    if (!pskb_may_pull(skb, sizeof(struct iphdr)))
        return NF_ACCEPT;

    iph = ip_hdr(skb);  /* = (struct iphdr *)(skb->head + skb->network_header) */

    if (iph->protocol != IPPROTO_TCP)
        return NF_ACCEPT;

    /* Ensure IP header + TCP header are in linear data */
    if (!pskb_may_pull(skb, iph->ihl * 4 + sizeof(struct tcphdr)))
        return NF_ACCEPT;

    /* Re-read iph after pskb_may_pull (may have reallocated) */
    iph  = ip_hdr(skb);
    tcph = (struct tcphdr *)((unsigned char *)iph + iph->ihl * 4);

    pr_info("TCP: %pI4:%u → %pI4:%u flags=%c%c%c\n",
            &iph->saddr, ntohs(tcph->source),
            &iph->daddr, ntohs(tcph->dest),
            tcph->syn ? 'S' : '.',
            tcph->ack ? 'A' : '.',
            tcph->fin ? 'F' : '.');

    /*
     * Modify packet (e.g., DNAT): must cow the header first
     * because skb may be cloned (shared with AF_PACKET capture)
     */
    if (skb_cow(skb, 0)) {
        /* allocation failed */
        return NF_DROP;
    }

    /* Now safe to modify: iph->daddr = new_addr; */

    return NF_ACCEPT;
}

static struct nf_hook_ops my_nf_ops = {
    .hook     = my_nf_hook,
    .pf       = PF_INET,
    .hooknum  = NF_INET_PRE_ROUTING,
    .priority = NF_IP_PRI_FIRST,
};

static int __init my_nf_init(void)
{
    return nf_register_net_hook(&init_net, &my_nf_ops);
}

static void __exit my_nf_exit(void)
{
    nf_unregister_net_hook(&init_net, &my_nf_ops);
}

module_init(my_nf_init);
module_exit(my_nf_exit);
MODULE_LICENSE("GPL");
```

---

## 16. Rust Implementation Samples

The Linux kernel has been gaining Rust support since 6.1. The `rust/kernel/net/` module provides safe abstractions over sk_buff. As of 6.9+, these bindings are still evolving but demonstrate the safety model.

### Sample 1: Rust sk_buff Wrapper (kernel abstraction)

```rust
// rust/kernel/net/skb.rs  (simplified from actual kernel source)
//
// The Rust bindings wrap the unsafe C sk_buff in safe types.

use crate::bindings;
use crate::error::{Error, Result};
use core::ptr::NonNull;

/// An owned reference to an sk_buff (drops/frees on Drop).
/// Invariant: ptr is always valid and non-null.
pub struct SkBuff {
    ptr: NonNull<bindings::sk_buff>,
}

impl SkBuff {
    /// # Safety
    /// Caller must ensure `ptr` is a valid, owned sk_buff pointer.
    pub unsafe fn from_raw(ptr: *mut bindings::sk_buff) -> Option<Self> {
        NonNull::new(ptr).map(|p| SkBuff { ptr: p })
    }

    /// Access the raw pointer (for FFI).
    pub fn as_raw(&self) -> *mut bindings::sk_buff {
        self.ptr.as_ptr()
    }

    /// Length of valid data in the sk_buff.
    pub fn len(&self) -> u32 {
        // SAFETY: ptr is always valid (invariant)
        unsafe { (*self.ptr.as_ptr()).len }
    }

    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    /// Length of the linear portion only.
    pub fn head_len(&self) -> u32 {
        let skb = unsafe { &*self.ptr.as_ptr() };
        // head_len = len - data_len
        skb.len - skb.data_len
    }

    /// Pull (consume) len bytes from the front.
    /// Returns error if len > self.len().
    pub fn pull(&mut self, len: u32) -> Result<()> {
        if len > self.len() {
            return Err(Error::EINVAL);
        }
        // SAFETY: we checked len <= skb->len
        unsafe {
            bindings::skb_pull(self.ptr.as_ptr(), len);
        }
        Ok(())
    }

    /// Get a read-only slice of the linear data.
    /// Only safe for linear portion; use copy_bits for full access.
    pub fn linear_data(&self) -> &[u8] {
        let skb = unsafe { &*self.ptr.as_ptr() };
        let data = skb.data;
        let head_len = self.head_len() as usize;
        // SAFETY: data..data+head_len is valid linear memory
        unsafe { core::slice::from_raw_parts(data, head_len) }
    }

    /// Copy bytes from any offset (handles linear + fragments).
    pub fn copy_bits(&self, offset: i32, dst: &mut [u8]) -> Result<()> {
        let len = dst.len() as i32;
        // SAFETY: dst is valid for len bytes; offset is caller-validated
        let ret = unsafe {
            bindings::skb_copy_bits(
                self.ptr.as_ptr(),
                offset,
                dst.as_mut_ptr() as *mut _,
                len,
            )
        };
        if ret < 0 {
            Err(Error::from_kernel_errno(ret))
        } else {
            Ok(())
        }
    }

    /// Get the network-layer protocol (ETH_P_IP, etc.)
    pub fn protocol(&self) -> u16 {
        // SAFETY: ptr valid
        u16::from_be(unsafe { (*self.ptr.as_ptr()).protocol })
    }

    /// Get the receiving network device name.
    pub fn dev_name(&self) -> Option<&[u8]> {
        let skb = unsafe { &*self.ptr.as_ptr() };
        let dev = skb.dev;
        if dev.is_null() {
            return None;
        }
        // SAFETY: dev is valid (not null)
        let name = unsafe { &(*dev).name };
        // name is [u8; IFNAMSIZ], null-terminated
        let len = name.iter().position(|&b| b == 0).unwrap_or(name.len());
        Some(&name[..len])
    }
}

impl Drop for SkBuff {
    fn drop(&mut self) {
        // SAFETY: We own this sk_buff (from_raw contract)
        unsafe {
            bindings::kfree_skb(self.ptr.as_ptr());
        }
    }
}

/// A borrowed reference to an sk_buff.
/// Does NOT free on drop. Used when the skb is owned by the kernel
/// (e.g., passed to a hook function).
pub struct SkBuffRef<'a> {
    ptr: NonNull<bindings::sk_buff>,
    _marker: core::marker::PhantomData<&'a bindings::sk_buff>,
}

impl<'a> SkBuffRef<'a> {
    /// # Safety
    /// ptr must be valid for at least the lifetime 'a.
    pub unsafe fn from_raw(ptr: *mut bindings::sk_buff) -> Option<Self> {
        NonNull::new(ptr).map(|p| SkBuffRef {
            ptr: p,
            _marker: core::marker::PhantomData,
        })
    }

    pub fn len(&self) -> u32 {
        unsafe { (*self.ptr.as_ptr()).len }
    }

    pub fn linear_data(&self) -> &'a [u8] {
        let skb = unsafe { &*self.ptr.as_ptr() };
        let head_len = (skb.len - skb.data_len) as usize;
        unsafe { core::slice::from_raw_parts(skb.data, head_len) }
    }
}
// Note: no Drop impl — we don't own it
```

### Sample 2: Rust Network Filter Driver Module

```rust
// rust_net_filter.rs — A Rust kernel module using sk_buff
//
// Demonstrates how to write a packet filter using Rust
// (requires Linux 6.8+ with CONFIG_RUST=y)

#![no_std]
#![feature(allocator_api)]

use kernel::prelude::*;
use kernel::net::filter::{NfHook, NfHookOps, NfVerdict};
use kernel::net::skb::SkBuffRef;
use kernel::net::ip::IpHdr;

module! {
    type: RustFilter,
    name: "rust_filter",
    author: "example",
    description: "Rust sk_buff inspection module",
    license: "GPL",
}

struct RustFilter {
    _hook: NfHookOps,
}

/// Our packet processing function.
/// Called for every IPv4 packet at PRE_ROUTING.
fn inspect_packet(skb: &SkBuffRef<'_>) -> NfVerdict {
    // Get the linear data slice
    let data = skb.linear_data();

    // Check minimum IP header size
    if data.len() < 20 {
        return NfVerdict::Accept;
    }

    // Parse IP header from raw bytes
    // In real kernel Rust, you'd use the IpHdr binding
    let version_ihl = data[0];
    let version = version_ihl >> 4;
    let ihl = (version_ihl & 0x0f) as usize * 4;

    if version != 4 || ihl < 20 || data.len() < ihl {
        return NfVerdict::Accept;
    }

    let protocol = data[9];
    let src_addr = u32::from_be_bytes([data[12], data[13], data[14], data[15]]);
    let dst_addr = u32::from_be_bytes([data[16], data[17], data[18], data[19]]);

    pr_debug!(
        "rust_filter: proto={} {:08x} -> {:08x}\n",
        protocol, src_addr, dst_addr
    );

    // Example: drop all ICMP (protocol=1) from specific source
    if protocol == 1 && src_addr == 0xc0a80101 {  /* 192.168.1.1 */
        pr_info!("rust_filter: dropping ICMP from 192.168.1.1\n");
        return NfVerdict::Drop;
    }

    NfVerdict::Accept
}

impl kernel::Module for RustFilter {
    fn init(_module: &'static ThisModule) -> Result<Self> {
        pr_info!("rust_filter: loading\n");

        let hook = NfHookOps::register_ipv4_pre_routing(inspect_packet)?;

        Ok(RustFilter { _hook: hook })
    }
}

impl Drop for RustFilter {
    fn drop(&mut self) {
        pr_info!("rust_filter: unloading\n");
        // _hook's Drop unregisters the netfilter hook
    }
}
```

### Sample 3: Rust sk_buff Queue (Safe Abstraction)

```rust
// skb_queue.rs — Safe Rust wrapper around sk_buff_head

use kernel::bindings;
use kernel::sync::SpinLock;
use core::ptr::NonNull;

/// A safe wrapper around sk_buff_head.
/// Uses Rust's ownership model to enforce correct use.
pub struct SkBuffQueue {
    inner: SpinLock<SkBuffQueueInner>,
}

struct SkBuffQueueInner {
    head: bindings::sk_buff_head,
}

// SAFETY: sk_buff_head with its spinlock is Send + Sync
unsafe impl Send for SkBuffQueue {}
unsafe impl Sync for SkBuffQueue {}

impl SkBuffQueue {
    pub fn new() -> Self {
        let mut head: bindings::sk_buff_head = unsafe { core::mem::zeroed() };
        // SAFETY: zeroed sk_buff_head is safe to initialize
        unsafe { bindings::skb_queue_head_init(&mut head) };
        SkBuffQueue {
            inner: unsafe { SpinLock::new(SkBuffQueueInner { head }) },
        }
    }

    /// Enqueue an owned sk_buff at the tail.
    /// Takes ownership: the queue now owns the sk_buff.
    pub fn enqueue(&self, mut skb: OwnedSkBuff) {
        let raw = skb.take_raw();  // take ownership, skb won't Drop
        let mut guard = self.inner.lock();
        // SAFETY: raw is valid, head is valid (locked)
        unsafe {
            bindings::skb_queue_tail(
                &mut guard.head as *mut _,
                raw,
            );
        }
    }

    /// Dequeue from head. Returns None if empty.
    pub fn dequeue(&self) -> Option<OwnedSkBuff> {
        let mut guard = self.inner.lock();
        // SAFETY: head is valid (locked)
        let raw = unsafe {
            bindings::skb_dequeue(&mut guard.head as *mut _)
        };
        if raw.is_null() {
            None
        } else {
            // SAFETY: dequeued skb is valid and we now own it
            Some(unsafe { OwnedSkBuff::from_raw(raw) })
        }
    }

    pub fn len(&self) -> u32 {
        let guard = self.inner.lock();
        guard.head.qlen
    }
}

impl Drop for SkBuffQueue {
    fn drop(&mut self) {
        // Free all remaining sk_buffs
        let mut guard = self.inner.lock();
        unsafe {
            bindings::skb_queue_purge(&mut guard.head as *mut _);
        }
    }
}

/// An owned sk_buff that calls kfree_skb on Drop.
pub struct OwnedSkBuff {
    ptr: NonNull<bindings::sk_buff>,
}

impl OwnedSkBuff {
    pub unsafe fn from_raw(ptr: *mut bindings::sk_buff) -> Self {
        OwnedSkBuff {
            ptr: NonNull::new_unchecked(ptr),
        }
    }

    fn take_raw(&mut self) -> *mut bindings::sk_buff {
        // This is only safe if caller ensures drop won't run
        // (used by enqueue to transfer ownership to queue)
        self.ptr.as_ptr()
    }

    pub fn data(&self) -> &[u8] {
        let skb = unsafe { &*self.ptr.as_ptr() };
        let head_len = (skb.len - skb.data_len) as usize;
        unsafe { core::slice::from_raw_parts(skb.data, head_len) }
    }
}

impl Drop for OwnedSkBuff {
    fn drop(&mut self) {
        unsafe { bindings::kfree_skb(self.ptr.as_ptr()) }
    }
}
```

### Sample 4: XDP BPF Program (C, but crucial for the full sk_buff picture)

```c
/*
 * xdp_drop_demo.c — XDP program that runs BEFORE sk_buff allocation.
 * This shows the zero-overhead path that bypasses sk_buff entirely.
 *
 * Compile: clang -O2 -target bpf -c xdp_drop_demo.c -o xdp_drop.o
 * Load:    ip link set dev eth0 xdp obj xdp_drop.o sec xdp
 */

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

/*
 * XDP context: xdp_md points to the raw DMA'd bytes.
 * NO sk_buff is created yet. This is the earliest possible intervention.
 */
SEC("xdp")
int xdp_filter(struct xdp_md *ctx)
{
    /*
     * ctx->data and ctx->data_end are offsets into the DMA page.
     * The verifier enforces bounds checking.
     */
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    /* Bounds check for Ethernet header (verifier requires explicit check) */
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_PASS;

    /* Only handle IPv4 */
    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return XDP_PASS;

    /* Bounds check for IP header */
    struct iphdr *iph = (void *)(eth + 1);
    if ((void *)(iph + 1) > data_end)
        return XDP_PASS;

    /* Drop all ICMP (protocol 1) */
    if (iph->protocol == IPPROTO_ICMP) {
        /* Drop happens here: NO sk_buff was ever allocated.
         * This is ~5-10x faster than drop in netfilter. */
        return XDP_DROP;
    }

    /* For TCP: could redirect to a specific CPU queue */
    if (iph->protocol == IPPROTO_TCP) {
        /* bpf_redirect_map() or just pass */
        return XDP_PASS;
    }

    return XDP_PASS;
    /* When XDP_PASS is returned:
     * Driver calls build_skb() → sk_buff is created → normal stack */
}

char _license[] SEC("license") = "GPL";
```

---

## 17. Mental Model Summary

### The Lifecycle in One Picture

```
═══════════════════════════════════════════════════════════════════════
                  sk_buff COMPLETE LIFECYCLE (RX)
═══════════════════════════════════════════════════════════════════════

[Wire arrives]
     │
     ▼
[DMA: NIC writes to pre-alloc page, no CPU]
     │
     ▼  
[Hard IRQ fires: napi_schedule()]
     │
     ▼
[Softirq: NAPI poll, budget=64]
     │
     ├── build_skb(dma_page)         ← sk_buff BORN: users=1, dataref=1
     │   skb_put(skb, pkt_len)       ← tail advances
     │   skb->protocol = ETH_P_IP    ← L3 type determined
     │
     ▼
[netif_receive_skb()]
     │
     ├── tcpdump hook: skb_clone()   ← dataref=2, clone goes to AF_PACKET
     │                                   original continues
     ▼
[ip_rcv()]
     │
     ├── NETFILTER: NF_INET_PRE_ROUTING (iptables runs)
     ├── Route: this host → ip_local_deliver()
     ├── skb_pull(ip_hdr_len)        ← data ptr advances past IP hdr
     │
     ▼
[tcp_v4_rcv()]
     │
     ├── Socket lookup (hash table)
     ├── skb_set_owner_r()           ← sk_rmem_alloc += skb->truesize
     │   skb->destructor = sock_rfree
     ├── TCP in-order: skb_queue_tail(&sk->sk_receive_queue, skb)
     │
     ▼
[Process wakes up: recv() syscall]
     │
     ├── skb_dequeue()
     ├── copy_to_user(skb->data, user_buf, len)  ← ONLY copy in the path
     ├── consume_skb(skb)            ← users: 1→0
     │     └── sock_rfree()          ← sk_rmem_alloc -= truesize
     │     └── skb_release_data()    ← dataref: 1→0 (or 2→1 if cloned)
     │         └── kfree(skb->head)  ← data buffer freed
     │     └── kmem_cache_free()     ← sk_buff struct returned to slab
     │
     ▼
[sk_buff DEAD: memory returned to slab/allocator]

═══════════════════════════════════════════════════════════════════════
```

### Key Invariants to Keep in Mind

1. **sk_buff is a view, not a copy.** Moving headers is pointer arithmetic, not memcpy.

2. **head never moves.** Only `data` and `tail` move within [head, end].

3. **skb->len = headlen + data_len.** Always. headlen = skb->tail - skb->data.

4. **dataref ≠ users.** `users` protects the sk_buff struct. `dataref` protects the data buffer. A clone increments dataref but starts with users=1.

5. **cb[] is per-layer scratch space.** Each layer overwrites it. Never read another layer's cb[].

6. **pskb_may_pull() before dereferencing headers.** Non-linear skbs may have the header in a page fragment, not in linear memory. pskb_may_pull() ensures (possibly copying) that `n` bytes are in the linear portion.

7. **skb_cow() before modifying shared data.** If `skb_cloned(skb)` is true, writing to headers corrupts the clone's view. Always call `skb_cow()` first.

8. **TX adds headers downward (skb_push), RX removes headers upward (skb_pull).** The directions are opposite and symmetric.

9. **fclone is a performance optimization.** The clone slot is pre-allocated next to the original in the slab cache — no separate allocation needed for the first clone.

10. **GFP_ATOMIC in softirq, GFP_KERNEL in process context.** alloc_skb() in NAPI poll must use GFP_ATOMIC (cannot sleep). Process-context allocation (sendmsg) can use GFP_KERNEL.

### The Algorithm Behind Protocol Stack Dispatch

```
L3 dispatch (from __netif_receive_skb_core):

  Protocol Type (ETH_P_*)
        │
        ▼
  ptype_base[] — hash table, 16 buckets, keyed on protocol
  ┌──────┬──────┬──────┬──────┬─────┬──────┐
  │ 0x00 │ 0x01 │ 0x02 │ ... │0x0a │ ...  │
  └──────┴──────┴──────┴──────┴─────┴──────┘
      │                            │
      ▼                            ▼
  ip_packet_type             ipv6_packet_type
  (.type=ETH_P_IP,           (.type=ETH_P_IPV6,
   .func=ip_rcv)              .func=ipv6_rcv)

L4 dispatch (from ip_local_deliver_finish):

  IP Protocol Number
        │
        ▼
  inet_protos[] — 256-entry array indexed by protocol number
  ┌────┬────┬────┬────┬────┬────┐
  │ 0  │ 1  │ 2  │ ...│ 6  │...│ 17 │...
  └────┴────┴────┴────┴────┴────┘
                           │          │
                           ▼          ▼
                      tcp_v4_rcv   udp_rcv
```

### Performance Characteristics

| Operation | Cost | Why |
|-----------|------|-----|
| skb_push(n) | O(1) | pointer decrement |
| skb_pull(n) | O(1) | pointer increment |
| skb_put(n)  | O(1) | pointer increment |
| skb_clone() | O(1) + atomic_inc | no data copy |
| skb_copy()  | O(len) | full data copy |
| alloc_skb() | O(1) amortized | slab cache hit |
| kfree_skb() | O(nr_frags) | release pages |
| pskb_may_pull(n) | O(1) if linear, O(n) if not | may copy fragment to linear |
| skb_copy_bits() | O(len) | may cross fragment boundaries |
| Protocol dispatch | O(1) | hash table + array lookup |

---

*This guide covers the sk_buff as found in Linux kernel 6.x. Specific field names, function signatures, and paths may vary slightly between kernel versions, but the fundamental architecture has been stable since 2.6.*
