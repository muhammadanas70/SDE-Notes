# Packet Metadata: A Comprehensive Technical Guide

## Table of Contents

1. [Foundational Concepts](#foundational-concepts)
2. [Memory Models and Architecture](#memory-models-and-architecture)
3. [Hardware Layer Metadata](#hardware-layer-metadata)
4. [Linux Kernel Data Structures](#linux-kernel-data-structures)
5. [Data Link Layer Metadata](#data-link-layer-metadata)
6. [Network Layer Metadata](#network-layer-metadata)
7. [Transport Layer Metadata](#transport-layer-metadata)
8. [Application Layer Metadata](#application-layer-metadata)
9. [Virtualization and Cloud Metadata](#virtualization-and-cloud-metadata)
10. [eBPF and Advanced Metadata Handling](#ebpf-and-advanced-metadata-handling)
11. [C Implementation Examples](#c-implementation-examples)
12. [Rust Implementation Examples](#rust-implementation-examples)
13. [Performance Optimization](#performance-optimization)

---

## 1. Foundational Concepts

### 1.1 What is Packet Metadata?

Packet metadata encompasses all contextual information associated with a network packet beyond the payload data. This includes:

- **Temporal metadata**: arrival time, timestamp, delay measurements
- **Spatial metadata**: source/destination ports, addresses, interface information
- **Structural metadata**: protocol type, packet length, fragment info
- **State metadata**: TCP state, connection tracking, qdisc information
- **Device metadata**: NIC stats, queue depth, NUMA affinity
- **Security metadata**: classification marks (skb->mark), QoS tags

### 1.2 Why Metadata Matters

```
Network Packet Processing Pipeline
────────────────────────────────────────────────────────────────

Hardware Interrupt
       ↓
[DMA Engine] → writes packet to ring buffer
       ↓
[NAPI Poll Handler] → reads packet from DMA ring
       ↓
[sk_buff Allocation] → metadata structure created
       ↓
[Driver Processing] → populates hardware metadata
       ↓
[GRO/LRO] → aggregates metadata for batching
       ↓
[Protocol Handler] → extracts protocol-specific metadata
       ↓
[Netfilter/XDP] → reads/modifies metadata
       ↓
[Socket Receive] → delivers metadata to userspace
```

Metadata enables:
1. **Routing decisions** without payload inspection
2. **QoS classification** for traffic management
3. **Connection tracking** for stateful processing
4. **DDoS mitigation** via early packet filtering
5. **Performance optimization** via batching and coalescing
6. **Debugging and monitoring** via instrumentation

---

## 2. Memory Models and Architecture

### 2.1 The sk_buff Structure: Linux Kernel Heart

The `sk_buff` (socket buffer) is the universal metadata container in the Linux kernel. Every packet processed exists within an sk_buff.

```
sk_buff Memory Layout
──────────────────────────────────────────────────────────────

┌─────────────────────────────────────────────────┐
│ struct sk_buff {                                │
│                                                 │
│  /* Pointers to data regions */                 │
│  unsigned char *head;        ────────┐          │
│  unsigned char *data;        ────────┼──┐       │
│  unsigned char *tail;        ────────┼──┼──┐    │
│  unsigned char *end;         ────────┼──┼──┼──┐ │
│                                      │  │  │  │ │
│  /* List management */               │  │  │  │ │
│  struct sk_buff *next;               │  │  │  │ │
│  struct sk_buff *prev;               │  │  │  │ │
│  struct sock *sk;                    │  │  │  │ │
│                                      │  │  │  │ │
│  /* Timestamps */                    │  │  │  │ │
│  ktime_t tstamp;          /* arrival time */  │ │
│  ktime_t skb_mstamp;      /* monotonic */     │ │
│                                      │  │  │  │ │
│  /* Device info */                   │  │  │  │ │
│  struct net_device *dev;   ← incoming NIC     │ │
│                                      │  │  │  │ │
│  /* Control metadata */              │  │  │  │ │
│  __u16 len;                ← total packet len │ │
│  __u16 data_len;           ← payload length   │ │
│  __u16 mac_len;            ← Ethernet hdr len │ │
│  __u16 transport_header;   ← offset to L4     │ │
│  __u16 network_header;     ← offset to L3     │ │
│  __u16 mac_header;         ← offset to L2     │ │
│                                      │  │  │  │ │
│  /* Protocol and forwarding */       │  │  │  │ │
│  __u8 pkt_type;            ← PACKET_HOST,etc  │ │
│  __u8 ip_summed;           ← CHECKSUM_* enum  │ │
│  __u8 priority;                         │  │  │ │
│                                      │  │  │  │ │
│  /* Offload capabilities */          │  │  │  │ │
│  struct skb_shared_info *shinfo; (in tail)    │ │
│                                      │  │  │  │ │
│  /* Classification */                │  │  │  │ │
│  __u32 mark;               ← netfilter mark   │ │
│  __u16 tc_index;           ← QDisc class      │ │
│                                      │  │  │  │ │
│  /* Connection tracking */           │  │  │  │ │
│  struct nf_conntrack *nfct; ← ct info      │  │ │
│                                      │  │  │  │ │
│  /* GSO/TSO metadata */              │  │  │  │ │
│  struct skb_shared_info {            │  │  │  │ │
│    unsigned short gso_size;          │  │  │  │ │
│    unsigned short gso_type;          │  │  │  │ │
│    struct skb_frag_struct frags[];   │  │  │  │ │
│  }                                   │  │  │  │ │
│                                      │  │  │  │ │
│ } /* 192-240 bytes depending on config */     │ │
└─────────────────────────────────────────────────┘

         ↓
    Memory Layout
         ↓

┌──────────────────────────────────────────────────┐
│ sk_buff structure (192-240 bytes)                │
├──────────────────────────────────────────────────┤
│ head ──→                                         │
│         ┌──────────────────────────────────────┐ │
│         │ Headroom (configurable, typically    │ │
│         │  64-320 bytes for headers)           │ │
│         ├──────────────────────────────────────┤ │
│ data ──→│ Ethernet Frame (14 bytes)            │ │
│         │ ┌────────────────────────────────────┤ │
│         │ │ IPv4/IPv6 Header (20-60 bytes)     │ │
│         │ │ ┌───────────────────────────────── ┤ │
│         │ │ │ TCP/UDP Header (20-60 bytes)     │ │
│         │ │ │ ┌─────────────────────────────  ─┤ │
│         │ │ │ │ Payload Data (variable)        │ │
│ tail ──→│ │ │ │                                │ │
│         │ │ │ └─────────────────────────────  ─┤ │
│         │ │ └────────────────────────── ───────┤ │
│         │ └────────────────────────────────────┤ │
│         └──────────────────────────────────────┘ │
│ end ──→  (actual end of allocated buffer)        │
│                                                  │
│ Tailroom (padding/fragments)                     │
└──────────────────────────────────────────────────┘
```

### 2.2 Header Offset Architecture

The Linux kernel uses offsets rather than pointers for protocol headers to save memory:

```c
/* Header offsets in sk_buff */

struct sk_buff {
    unsigned char *data;
    __u16 mac_header;       /* offset from head to MAC start */
    __u16 network_header;   /* offset from head to IP start */
    __u16 transport_header; /* offset from head to TCP/UDP start */
};

/* Access pattern: */
/* To get actual pointer: data + offset */

#define skb_mac_header(SKB)         ((SKB)->head + (SKB)->mac_header)
#define skb_network_header(SKB)     ((SKB)->head + (SKB)->network_header)
#define skb_transport_header(SKB)   ((SKB)->head + (SKB)->transport_header)

/* Advantages: 
   - Offset is 16-bit (0-65535) vs 64-bit pointer
   - Offset doesn't change if head pointer moves (during prepending)
   - Saves 48 bits per sk_buff × millions of packets = megabytes saved
   - Enables zero-copy on certain operations
*/
```

### 2.3 sk_buff Growth and Shrinkage

```
Packet Processing Life Cycle
────────────────────────────────────────────────────

1. Initial Allocation (RX)
   ┌──────────────────┐
   │ skb_alloc()      │ Allocates sk_buff + data region
   │ Order 0 (~4KB)   │
   └────────┬─────────┘
            ↓
   ┌──────────────────┐
   │ sk_buff + data   │
   │ [headroom][data] │
   └────────┬─────────┘
            ↓

2. Header Prepending (Forward to wire)
   ┌──────────────────────────────────────┐
   │ skb_push(skb, hdr_len)               │
   │ Moves data pointer backward          │
   │ Increases available headroom space   │
   └────────┬─────────────────────────────┘
            ↓
   ┌──────────────────────────────────────┐
   │ [headroom][NEW_HDR][old_data]        │
   │ ↑                                    │
   │ data pointer moved left              │
   └────────┬─────────────────────────────┘
            ↓

3. Header Removal (Parse)
   ┌──────────────────────────────────────┐
   │ skb_pull(skb, hdr_len)               │
   │ Moves data pointer forward           │
   │ Returns old data pointer (header)    │
   └────────┬─────────────────────────────┘
            ↓
   ┌──────────────────────────────────────┐
   │ [consumed_hdr][remaining_data]       │
   │                ↑                     │
   │                data pointer moved    │
   │                right (past header)   │
   └──────────────────────────────────────┘

4. Data Trimming (skb_trim)
   Moves tail pointer, reduces len field

5. Clone Operations (skb_clone)
   - Shares data buffer
   - Duplicates sk_buff metadata structure
   - Reference counting via skb_shared_info
```

### 2.4 Fragment Handling: skb_shared_info

For packets > ~4KB or segmented packets:

```c
struct skb_shared_info {
    atomic_t    dataref;            /* Reference count for data */
    unsigned short  gso_size;       /* Bytes per segment (TSO/GSO) */
    unsigned short  gso_type;       /* SKB_GSO_TCPV4, etc */
    unsigned short  nr_frags;       /* Number of fragments */
    __u8 tx_flags;
    struct sk_buff *frag_list;      /* Linked list of fragments */
    
    struct skb_frag_struct {
        struct page *page;
        __u32 page_offset;
        __u32 size;
    } frags[MAX_SKB_FRAGS];         /* Array of page fragments */
};

/*
Multi-page packet layout:
┌──────────────────┐
│ sk_buff head     │ (main structure, ~240 bytes)
│ (linear data)    │ (0-4KB typical)
└─────────┬────────┘
          │
          ↓
┌──────────────────────────────────┐
│ skb_shared_info                  │
├──────────────────────────────────┤
│ frags[0] → page pointer 0        │ ← Next 4KB of data
│ frags[1] → page pointer 1        │ ← Next 4KB of data
│ frags[2] → page pointer 2        │ ← Next 4KB of data
│ ...                              │
└──────────────────────────────────┘

This allows packets of arbitrary size without continuous
physical memory, crucial for modern high-speed networking.
*/
```

---

## 3. Hardware Layer Metadata

### 3.1 NIC DMA Ring Buffers and Descriptors

Every network interface card (NIC) maintains DMA ring buffers for packet reception and transmission.

```
NIC RX Ring Buffer Architecture
────────────────────────────────────────────────────────

NIC Hardware (PCIe Device)
┌─────────────────────────────────────────┐
│ MAC/PHY Layer                           │
│ Receives packets from physical wire     │
└────────────┬────────────────────────────┘
             │ DMA
             ↓
┌───────────────────────────────────────────────────┐
│ DMA RX Ring (in host memory, written by NIC)      │
│                                                   │
│ ┌────────────────────────────────────────────┐    │
│ │ RX Descriptor 0                            │    │
│ │ ┌────────────────────────────────────────┐ │    │
│ │ │ Buffer Address (physical)      [64-bit]│ │    │
│ │ ├────────────────────────────────────────┤ │    │
│ │ │ Status Flags                   [16-bit]│ │    │
│ │ │   - DD (Done bit)                      │ │    │
│ │ │   - EOP (End of Packet)                │ │    │
│ │ │   - IXSM (IP checksum done)            │ │    │
│ │ │   - L4E (L4 error)                     │ │    │
│ │ │   - VLAN (VLAN present)                │ │    │
│ │ │   - TS (Timestamp available)           │ │    │
│ │ ├────────────────────────────────────────┤ │    │
│ │ │ Packet Length                  [16-bit]│ │    │
│ │ ├────────────────────────────────────────┤ │    │
│ │ │ Error Bits                     [16-bit]│ │    │
│ │ │   - RXE (RX error)                     │ │    │
│ │ │   - IPE (IP payload error)             │ │    │
│ │ │   - TCPE (TCP/UDP payload error)       │ │    │
│ │ ├────────────────────────────────────────┤ │    │
│ │ │ VLAN Tag / Extended Info       [32-bit]│ │    │
│ │ │   - VID (VLAN ID)                      │ │    │
│ │ │   - CFI (Canonical Form Indicator)     │ │    │
│ │ │   - PRI (Priority)                     │ │    │
│ │ │   - EXTENDED: RSS Hash / Timestamp     │ │    │
│ │ └────────────────────────────────────────┘ │    │
│ └────────────────────────────────────────────┘    │
│                                                   │
│ ┌────────────────────────────────────────────┐    │
│ │ RX Descriptor 1                            │    │
│ │ [Same structure as above]                  │    │
│ └────────────────────────────────────────────┘    │
│                                                   │
│ ┌────────────────────────────────────────────┐    │
│ │ RX Descriptor 2                            │    │
│ │ [Same structure as above]                  │    │
│ └────────────────────────────────────────────┘    │
│                                                   │
│ ... (typically 256-4096 descriptors per ring)     │
│                                                   │
└───────────────────────────────────────────────────┘

Intel 82599 10G NIC RX Descriptor Format (Advanced)
────────────────────────────────────────────────────

Byte Offset    Field              Size    Description
────────────────────────────────────────────────────
0-7            Buffer Address     64-bit  DMA address of data buffer
8-9            Length             16-bit  Packet length
10-11          Packet Checksum    16-bit  L3/L4 checksum from NIC
12-13          Status             16-bit  Descriptor status flags
14-15          Error              16-bit  Error bits
16-19          RSS Hash/Status    32-bit  RSS info or extended status
20-21          VLAN Tag           16-bit  802.1Q tag
22-23          Extended Status    16-bit  Filter match, last descriptor
24-31          Timestamp          64-bit  PTP timestamp (optional)

Status Bits (12-13):
Bit 0 (DD)    - Descriptor Done (written by NIC, read by driver)
Bit 1 (EOP)   - End of Packet
Bit 2 (IXSM)  - Ignore Checksum (partial checksums)
Bit 3 (VP)    - VLAN Packet
Bit 4 (UDPCS) - UDP checksum
Bit 5 (TCPCS) - TCP checksum
Bit 6 (IPCS)  - IP checksum
Bit 7 (PIF)   - Passed in-exact filter
Bit 8 (CRCV)  - CRC valid
Bit 9 (PV)    - Packet is valid
Bit 10 (TSIP) - Timestamp in packet
Bit 11 (FLM)  - FDir Filter Match
Bit 12 (FEO)  - FDir error or FLEX
Bit 13-15     - Reserved
```

### 3.2 Hardware Offload Metadata

```
Hardware Offloading Information
────────────────────────────────────────────────────

Checksum Offload:
┌──────────────────────────────────────┐
│ sk_buff→ip_summed field              │
├──────────────────────────────────────┤
│ CHECKSUM_NONE = 0                    │
│   - No checksum information present  │
│   - Stack must compute checksums     │
│                                      │
│ CHECKSUM_UNNECESSARY = 1             │
│   - Checksum already verified by HW  │
│   - Stack trusts this checksum       │
│                                      │
│ CHECKSUM_PARTIAL = 2                 │
│   - Pseudo-header checksum computed  │
│   - IP payload checksum needs update │
│   - Transport-layer-specific handling│
│                                      │
│ CHECKSUM_COMPLETE = 3                │
│   - Checksum (sk_buff→csum) covers   │
│   - From first NIC byte to skb→tail  │
│   - May not align with L3/L4 payload │
└──────────────────────────────────────┘

Segmentation Offload Metadata:
┌────────────────────────────────────────────────┐
│ struct skb_shared_info {                       │
│   u16 gso_size;       ← bytes per segment      │
│   u16 gso_type;       ← offload type           │
│ }                                              │
│                                                │
│ gso_type flags:                                │
│   SKB_GSO_TCPV4        ← TCP over IPv4         │
│   SKB_GSO_TCPV6        ← TCP over IPv6         │
│   SKB_GSO_UDPV4        ← UDP segmentation      │
│   SKB_GSO_UDPV6        ← UDP over IPv6         │
│   SKB_GSO_PARTIAL      ← partial offload       │
│   SKB_GSO_GRE          ← GRE tunnel            │
│   SKB_GSO_GRE_CSUM     ← GRE with checksum     │
│   SKB_GSO_IPXIP4/6     ← IP-in-IP tunneling    │
│   SKB_GSO_UDP_TUNNEL   ← UDP tunnel (VXLAN)    │
│   SKB_GSO_UDP_TUNNEL_CSUM ← with checksum      │
│   SKB_GSO_PARTIAL      ← partial HW offload    │
│                                                │
│ Example: 64KB payload with TSO                 │
│   - gso_size = 1460 (MSS for TCP)              │
│   - gso_type = SKB_GSO_TCPV4                   │
│   - NIC segments into ~44 packets              │
│   - Each 1460 + headers in size                │
│   - Stack only prepares one sk_buff            │
│   - NIC generates all TCP sequence numbers     │
│   - Huge CPU savings: 1 → 44 packets           │
└────────────────────────────────────────────────┘

Receive Side Scaling (RSS) Metadata:
┌──────────────────────────────────────────────────┐
│ NIC distributes packets across RX queues         │
│ based on a hash of packet 5-tuple                │
│                                                  │
│ Metadata passed to stack:                        │
│                                                  │
│ sk_buff→hash              ← RSS hash value       │
│ sk_buff→l4_hash           ← L4 hash indicator    │
│ sk_buff→sw_hash           ← software computed    │
│                                                  │
│ Hash includes:                                   │
│   [Source IP][Dest IP][Source Port][Dest Port]   │
│   + [Protocol] (TCP/UDP)                         │
│                                                  │
│ Benefits:                                        │
│   - Packets from same flow always on same queue  │
│   - Each CPU handles isolated flows              │
│   - Eliminates locking on per-flow state         │
│   - Scales linearly with CPU cores               │
│   - Crucial for 10G+ NICs                        │
│                                                  │
│ Typical setup (8-core system):                   │
│   CPU0 ← RX Queue 0 ← hash%8 = 0                 │
│   CPU1 ← RX Queue 1 ← hash%8 = 1                 │
│   ...                                            │
│   CPU7 ← RX Queue 7 ← hash%8 = 7                 │
└──────────────────────────────────────────────────┘
```

### 3.3 Timestamp Metadata

```
PTP (Precision Time Protocol) and Hardware Timestamps
──────────────────────────────────────────────────────

struct skb_shared_info {
    ...
    ktime_t hwtstamp;       ← Hardware timestamp (PTP nanoseconds)
};

Timestamp Collection Points:
┌────────────────────────────────────────────────┐
│                                                │
│ 1. RX Timestamping (First-In Timestamping)     │
│    ┌─────────────────────────────────────────┐ │
│    │ Packet arrives at PHY (wire)            │ │
│    │         ↓ (nanosecond precision)        │ │
│    │ PTP clock captures timestamp            │ │
│    │         ↓                                 │
│    │ Stored in RX descriptor                 │ │
│    │         ↓                               │
│    │ Driver reads and stores in sk_buff      │ │
│    └─────────────────────────────────────────┘ │
│                                                │
│ 2. TX Timestamping (Last-Out Timestamping)     │
│    ┌─────────────────────────────────────────┐ │
│    │ Packet queued for transmission          │ │
│    │         ↓ (captured optionally)         │
│    │ Packet leaves MAC (at wire)             │ │
│    │         ↓                                 │
│    │ PTP clock captures timestamp            │ │
│    │         ↓                                 │
│    │ NIC generates completion descriptor     │ │
│    │         ↓                                 │
│    │ Driver sends to userspace via CMSG      │ │
│    └─────────────────────────────────────────┘ │
│                                                │
│ 3. Software Timestamps (CPU clock)             │
│    ┌─────────────────────────────────────────┐ │
│    │ CPU reads clock (ktime_t) at:           │ │
│    │   - Interrupt arrival (tstamp)          │ │
│    │   - NAPI poll start (skb_mstamp)        │ │
│    │ Uses monotonic clock to avoid           │ │
│    │ wall-clock adjustments                  │ │
│    └─────────────────────────────────────────┘ │
│                                                │
└────────────────────────────────────────────────┘

Timestamp Types:
HWTSTAMP_FILTER_NONE            - No timestamping
HWTSTAMP_FILTER_ALL             - All packets
HWTSTAMP_FILTER_PTP_V1_L4_SYNC  - PTP v1 over UDP (port 319)
HWTSTAMP_FILTER_PTP_V1_L4_DELAY_REQ
HWTSTAMP_FILTER_PTP_V2_L4_EVENT - PTP v2 over UDP (port 319-320)
HWTSTAMP_FILTER_PTP_V2_L4_SYNC
HWTSTAMP_FILTER_PTP_V2_L2_EVENT - PTP v2 over Ethernet (0x88F7)
HWTSTAMP_FILTER_PTP_V2_SYNC

Access from userspace:
┌─────────────────────────────────────────────────────┐
│ int fd = socket(AF_INET, SOCK_DGRAM, 0);            │
│                                                     │
│ struct hwtstamp_config config = {                   │
│     .flags = 0,                                     │
│     .tx_type = HWTSTAMP_TX_ON,                      │
│     .rx_filter = HWTSTAMP_FILTER_ALL,               │
│ };                                                  │
│                                                     │
│ ioctl(fd, SIOCSHWTSTAMP, &ifreq);  ← Enable         │
│                                                     │
│ /* Receive timestamp via cmsg */                    │
│ struct cmsghdr *cmsg;                               │
│ struct timespec64 ts;                               │
│                                                     │
│ recvmsg(fd, &msg, 0);                               │
│                                                     │
│ for (cmsg = CMSG_FIRSTHDR(&msg); cmsg;             │
│      cmsg = CMSG_NXTHDR(&msg, cmsg)) {             │
│     if (cmsg->cmsg_level == SOL_SOCKET &&          │
│         cmsg->cmsg_type == SCM_TIMESTAMPNS) {      │
│         memcpy(&ts, CMSG_DATA(cmsg), sizeof(ts));  │
│         /* ts now contains hardware timestamp */   │
│     }                                              │
│ }                                                  │
└─────────────────────────────────────────────────────┘
```

---

## 4. Linux Kernel Data Structures

### 4.1 Complete sk_buff Structure Reference

```c
/* Linux kernel net/core/skbuff.h (simplified/annotated) */

struct sk_buff {
    /* These two members must be first for skb_unlink to work */
    struct sk_buff *next;
    struct sk_buff *prev;

    union {
        struct {
            /* These members are written at each packet arrival so
             * keep them in a separate cache line from everything else.
             */
            ktime_t tstamp;              /* Time stamp (64-bit) */
            struct sock *sk;             /* Socket backpointer (64-bit) */
        };
        struct rb_node rbnode;           /* Used in netem, ip4_frag, etc */
        struct list_head list;           /* Linked list node */
    };

    union {
        struct net_device *dev;
        /* Some protocols might use the previous __dev field */
    };

    /*
     * This is the control buffer. It is free to use for every
     * layer. Please put your private variables there. If you
     * want to keep them across layers you have to do a skb_clone()
     * first. This is owned by whoever has the skb queued ATM.
     */
    char cb[48] __aligned(8);

    unsigned long _skb_refdst;           /* Destination cache (DST) */
    void (*destructor)(struct sk_buff *skb);

    struct sec_path *sp;                 /* Security path extension */
#ifdef CONFIG_SKB_EXTENSIONS
    struct skb_ext *extensions;          /* Extensible fields via skb_ext */
#endif
};

/* Control buffer usage by layers */
#define skb_shinfo(SKB) ((struct skb_shared_info *)(skb_end_pointer(SKB)))
#define skb_hwtstamps(SKB) (&(SKB)->tstamp_type)
#define skb_queue_mapping(SKB) ((SKB)->queue_mapping)

struct sk_buff {
    /* ... previous fields ... */

    struct {
        __u16 transport_header;          /* Offset to L4 header */
        __u16 network_header;            /* Offset to L3 header */
        __u16 mac_header;                /* Offset to L2 header */
        __u16 inner_transport_header;    /* Inner tunnel L4 */
        __u16 inner_network_header;      /* Inner tunnel L3 */
        __u16 inner_mac_header;          /* Inner tunnel L2 */
    };

    __u32 inner_ipproto;                 /* Inner IP protocol */

    __u16 len,                           /* Total packet length */
          data_len;                      /* Payload length (non-linear) */
    __u16 mac_len,                       /* MAC header length */
          hdr_len;                       /* Hardware header length */
    __u32 priority;                      /* QoS priority (0-15) */
    __u8  ignore_df : 1,                 /* Do not fragment */
        cloned : 1,                      /* Is a cloned packet */
        ip_summed : 2,                   /* IP checksum type */
        nohdr : 1,                       /* Raw socket, no header */
        nfctinfo : 3;                    /* netfilter connection track */

    __u8  pkt_type;                      /* Packet type
                                          * PACKET_HOST - destined for us
                                          * PACKET_BROADCAST
                                          * PACKET_MULTICAST
                                          * PACKET_OTHERHOST - promiscuous
                                          * PACKET_OUTGOING
                                          */

    __u16 frag_off;                      /* Fragment offset (IPv4) */
    __u16 csum;                          /* Transport checksum value */
    __u32 mark;                          /* netfilter mark value
                                          * (conn track, routing, etc)
                                          */
    __u16 vlan_proto;                    /* VLAN protocol: ETH_P_8021Q, etc */
    __u16 vlan_tci;                      /* VLAN TCI with priority + VLAN ID */

    union {
        struct {
            __u16 tc_index;              /* qdisc class / filter index */
            __u16 tc_verd;               /* traffic control verdict */
        };
        __u32 secmark;                   /* SELinux security label */
    };

    union {
        __u32 hash;                      /* RSS hash */
        __u32 swhash;                    /* Software hash (flow dissector) */
    };

    __u32 flow_dissector_key;            /* Dissector key for classification */

    __u32 rxhash;                        /* Rx hash */
    
    __u8 l4_hash : 1,                    /* Hash covers L4 */
         l4_hash_noref : 1,              /* Hash computed by remote */
         is_hijacked : 1;                /* Hijacked by netfilter */
    
    __u8 pfmemalloc : 1;                 /* Packet from pfmemalloc skb */
    __u8 active_extensions;              /* Number of active extensions */

    __u32 csum_level : 2,                /* Checksum level 0-2 */
          csum_bad : 1,                  /* Checksum invalid */
          dst_pending_confirm : 1,       /* Confirmation needed for DST */
          decrypted : 1,                 /* Decrypted in-place by xfrm */
          slow_gro : 1,                  /* GRO packet marked for slow path */
          mono_delivery_time : 1;        /* Represents mono delivery time */

    __u16 inner_ipproto : 8,             /* Inner protocol */
          inner_transport_header : 16;   /* Inner L4 offset */

    struct {
        __u8 __pkt_type_offset[0];
        struct {
            __u8 __cloned : 1;
            __u8 __pkt_vlan_present_offset[0];
            struct {
                __u16 vlan_present : 1;  /* VLAN present in skb */
            };
        };
    };

    /* These are the bytes in the CB that are allocated for netfilter */
    unsigned long nfmark;                /* Old netfilter mark (32-bit) */

    struct nf_conntrack *nfct;           /* netfilter connection tracking
                                          * Can be NULL
                                          */
#if IS_ENABLED(CONFIG_NF_CONNTRACK)
    struct hlist_node nfct_list;         /* Connection track hash list */
#endif
    nf_conntrack_get_hook_t nfct_get;    /* Getter function */

    struct nf_bridge_info *nf_bridge;    /* netfilter bridge extension */

    unsigned int len;                    /* Length of actual data */
    unsigned int data_len;               /* Data fragment length */
    __u16 mac_len;                       /* MAC header length */
    __u16 hdr_len;                       /* Hardware header length */
    __u32 priority;                      /* Packet priority (0-15) */

    __u8  pkt_type;                      /* See PACKET_* above */
    __u8  __pkt_type_offset[0];

    __u32 mark;                          /* netfilter mark + connection tracking */

    __u32 secmark;                       /* SELinux security */

    __u32 napi_id;                       /* NAPI context for GRO */

    __u32 hash;                          /* Flow hash / RSS hash */

    __u16 queue_mapping;                 /* Queue mapping for MQ devices
                                          * skb_set_queue_mapping(skb, queue)
                                          * Determines which NIC TX queue
                                          */

    __u8  xmit_more : 1,                 /* More frames follow */
        inner_ipproto : 8,
        nf_trace : 1,                   /* Packet marked for tracing */
        ip_vlan_tag : 1,                /* VLAN tag present */
        nfctinfo : 3;                   /* Netfilter connection tracking info */

    __u8  vlan_present : 1,              /* VLAN tag in skb (802.1Q/802.1ad) */
        vlan_proto : 16;                 /* VLAN protocol (htons value) */
    __u16 vlan_tci;                      /* VLAN TCI with priority */

    /* Transport layer payload offset/info */
    union {
        __u32 inner_ipproto;
        __u8 gso_type;                   /* Segmentation offload type */
    };

    __u16 gso_segs;                      /* Number of GSO segments */
    __u16 gso_size;                      /* Segment size for GSO */

    struct skb_shared_info *shinfo;      /* Shared info (frags, etc) */

    /* End of first cache line marker - DO NOT ADD NEW FIELDS BELOW THIS */
};

struct skb_shared_info {
    __u8 __pkt_vlan_present_offset[0];
    struct {
        unsigned short vlan_present:1;
        unsigned short vlan_proto:16;
    };
    unsigned short gso_type;
    unsigned short gso_segs;
    unsigned short gso_size;

    struct sk_buff *frag_list;           /* Multi-buffer packets (GRO, GRE, etc) */

    struct skb_frag_struct frags[MAX_SKB_FRAGS];  /* Fragment array */

    atomic_t dataref;                    /* Reference count for data */

    /* Timestamps and hardware info */
    unsigned long ts_jiffies;
    ktime_t skb_mstamp;
    struct skb_hwtstamps {
        ktime_t hwtstamp;                /* Hardware timestamp */
        ktime_t syststamp;               /* System timestamp */
        u32 netdev_data;                 /* Device specific */
    };

    /* GSO context information */
    struct gso_context {
        __u32 segs_remaining;
        struct gso_seg {
            __u32 size;
            __u32 offset;
        };
    };
};

#define SKB_DATA_ALIGN(X)       (((X) + (SMP_CACHE_BYTES - 1)) & \
                                 ~(SMP_CACHE_BYTES - 1))
#define SKB_WITH_OVERHEAD(X)    (((X) - sizeof(struct skb_shared_info)) & \
                                 ~(SMP_CACHE_BYTES - 1))

/* Handy accessors */
#define skb_headroom(SKB)       ((SKB)->data - (SKB)->head)
#define skb_tailroom(SKB)       ((SKB)->end - (SKB)->tail)
#define skb_availroom(SKB)      (skb_is_nonlinear(SKB) ? 0 : \
                                 (SKB)->end - (SKB)->tail)
#define skb_reserve(SKB, LEN)   ((SKB)->data += (LEN), (SKB)->tail += (LEN))

#define skb_mac_header(SKB)     ((SKB)->head + (SKB)->mac_header)
#define skb_network_header(SKB) ((SKB)->head + (SKB)->network_header)
#define skb_transport_header(SKB) ((SKB)->head + (SKB)->transport_header)
```

### 4.2 Extended Attributes (skb_ext)

For rarely-used metadata that would bloat sk_buff:

```c
struct skb_ext {
    refcount_t refcnt;
    u8 offset[SKB_EXT_NUM];              /* Offsets of extensions */
    u8 chunks;                           /* Number of chunks */
    char data[0] __aligned(8);           /* Extension data */
};

enum skb_ext_id {
    SKB_EXT_BRIDGE_NF_RESET_MASK,        /* Bridge netfilter reset mask */
    SKB_EXT_SECPATH,                     /* Security context (IPSec) */
#if IS_ENABLED(CONFIG_NET_TC_SKB_EXT)
    SKB_EXT_TC,                          /* Traffic control metadata */
#endif
    SKB_EXT_MPTCP,                       /* Multipath TCP */
    SKB_EXT_NUM,
};

/*
Overhead calculation:
- Without extensions: skb_ext = NULL (just a pointer, ~8 bytes)
- With extensions: skb_ext allocated on-demand
- Extension payload stored in data[] array
- Multiple extensions can coexist in single skb_ext structure
- Only allocated when needed
*/

/* Create extension */
void *__skb_ext_alloc(unsigned int id)
{
    struct skb_ext *ext;
    size_t sz;
    
    /* Calculate required size for extension */
    sz = ALIGN(sizeof(*ext), 8) + /* base structure */
         ext->chunks * sizeof(u8) + /* offsets */
         ext->data_size;            /* extension data */
    
    ext = kmem_cache_alloc(...);    /* Allocate from slab cache */
    return ext->data + ext->offset[id];
}

/* Access extension */
#define skb_ext_find(SKB, ID) ({
    struct skb_ext *ext = (SKB)->extensions;
    if (ext && ext->offset[ID])
        ext->data + ext->offset[ID];
    else
        NULL;
})
```

### 4.3 Buffer Management and Memory Zones

```
sk_buff Allocation Strategy
────────────────────────────────────────────────────

When a packet arrives, the driver allocates memory strategically
to minimize memory fragmentation and cache misses:

1. Linear Data Zone
   ┌─────────────────────────────────────────────┐
   │sk_buff + head + data (one memory allocation)│
   │                                             │
   │ Typical size: 2048 - 4096 bytes             │
   │ Covers: sk_buff structure + headroom +      │
   │         Ethernet + IP + TCP headers +       │
   │         up to 512 bytes of payload          │
   │                                             │
   │ Allocated via: __netdev_alloc_skb_ip_align()│
   │                or kmem_cache (size ~2-4K)   │
   └─────────────────────────────────────────────┘
   
   Advantages:
   - Single cache-line friendly access pattern
   - Headers are usually accessed sequentially
   - Typical packets fit entirely here
   - Fast allocation from slab cache

2. Fragment Pages (for large packets)
   ┌──────────────────────────────────────────────────┐
   │ skb_shared_info→frags[0..15] (order-0 pages)     │
   │                                                  │
   │ frags[0] ──→ Page 0 (4096 bytes)                 │
   │ frags[1] ──→ Page 1 (4096 bytes)                 │
   │ frags[2] ──→ Page 2 (4096 bytes)                 │
   │ ...                                              │
   │                                                  │
   │ Used for: TSO/GSO packets, jumbo frames,         │
   │           GRO/LRO aggregated packets             │
   │                                                  │
   │ Allocated via: alloc_page(GFP_ATOMIC)            │
   │ or pre-allocated ring buffers (best practice)    │
   └──────────────────────────────────────────────────┘
   
   Advantages:
   - Packets can exceed typical page size
   - Avoids large contiguous allocations
   - Can reuse pages across multiple skbs
   - Enables page recycling (recent improvement)

Memory Zones (Linux SLUB allocator):
┌────────────────────────────────────────────────────────┐
│ /proc/slabinfo (kmalloc zone examples)                 │
│                                                        │
│ kmalloc-256    48 slab objects (12.2 MB)               │
│   - Used by: struct skb_shared_info if needed          │
│                                                        │
│ kmalloc-512    96 slab objects (24.4 MB)               │
│   - Used by: small sk_buff + small payloads            │
│                                                        │
│ kmalloc-1024   192 slab objects (48.8 MB)              │
│   - Used by: most common case                          │
│                                                        │
│ kmalloc-2048   384 slab objects (97.6 MB)              │
│   - Used by: larger packets, high-speed NICs           │
│                                                        │
│ kmalloc-4096   768 slab objects (195.2 MB)             │
│   - Used by: jumbo frames, aggregated packets          │
│                                                        │
└────────────────────────────────────────────────────────┘

Allocation Patterns for Different Hardware:
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│ 1. Gigabit Ethernet (1Gbps)                                  │
│    Typical packet: 1500 bytes                                │
│    MTU 1500 + 14 (Eth) + 20 (IP) + 20 (TCP) = 1554           │
│    → Allocated from kmalloc-2048 zone                        │
│    Headroom: 64 bytes for potential prepends                 │
│                                                              │
│ 2. 10G Ethernet                                              │
│    Mix of:                                                   │
│    - Regular packets (1500 MTU) → kmalloc-2048               │
│    - Large packets with GSO (64KB) → multiple frags          │
│    - GRO packets (64KB aggregate) → skb_shared_info          │
│    Headroom: 256+ bytes (tunneling headers)                  │
│                                                              │
│ 3. SmartNIC / DPU offloading                                 │
│    Jumbo frames (9000 bytes)                                 │
│    - Linear: 4096 bytes (single page)                        │
│    - Frags: pages for overflow                               │
│    Header prepend space: 320+ bytes                          │
│                                                              │
│ 4. Embedded / IoT                                            │
│    Memory constrained                                        │
│    - Smaller headroom (32 bytes)                             │
│    - Minimal fragmentation (single allocation)               │
│    - sk_buff pool pre-allocated                              │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

---

## 5. Data Link Layer Metadata

### 5.1 Ethernet Frame Structure and Metadata

```
Ethernet Frame with Metadata
──────────────────────────────────────────────────

Physical Frame on Wire:
┌───────────────────────────────────────────────────┐
│ Preamble (7 bytes) + SFD (1 byte) [removed by PHY]│
├───────────────────────────────────────────────────┤
│                                                   │
│ Destination MAC    48-bit (6 bytes)               │
│ ┌─────────────────────────────────────────────┐   │
│ │ DA: FF:FF:FF:FF:FF:FF (Broadcast example)   │   │
│ │                                             │   │
│ │DA[0] bits: |U/L|I/G| (U/L=0 unicast,1 mcast)│   │ 
│ │             |I/G=0 individual, 1 group      │   │
│ └─────────────────────────────────────────────┘   │
│                                                   │
│ Source MAC         48-bit (6 bytes)              │
│ ┌─────────────────────────────────────────────┐  │
│ │ SA: 08:00:27:C4:D2:9B (NIC hardware addr)   │  │
│ │ SA[0] bit 1: Locally administered           │  │
│ │ SA[0] bit 0: Unicast                        │  │
│ └─────────────────────────────────────────────┘  │
│                                                  │
│ EtherType        16-bit (2 bytes)                │
│ ┌────────────────────────────────────────────┐  │
│ │ 0x0800 = IPv4                              │  │
│ │ 0x0806 = ARP                               │  │
│ │ 0x86DD = IPv6                              │  │
│ │ 0x8100 = 802.1Q VLAN                       │  │
│ │ 0x8847 = MPLS unicast                      │  │
│ │ 0x8848 = MPLS multicast                    │  │
│ │ 0x8864 = PPPoE Discovery                   │  │
│ │ 0x8865 = PPPoE Session                     │  │
│ │ 0x88A8 = 802.1ad (QinQ provider)           │  │
│ │ 0x88CC = LLDP                              │  │
│ │ 0x88E5 = MAC Security (MACsec)             │  │
│ └────────────────────────────────────────────┘  │
│                                                 │
│ Payload (or VLAN tag + Payload)                 │
│ ┌────────────────────────────────────────────┐ │
│ │ 46-1500 bytes (or 42-1496 if VLAN/QinQ)    │ │
│ │ (Note: Jumbo frames extend to 9000+ bytes) │ │
│ └────────────────────────────────────────────┘ │
│                                                │
│ Frame Check Sequence (FCS)   32-bit (4 bytes)  │
│ ┌───────────────────────────────────────────┐  │
│ │ CRC32 checksum of entire frame            │  │
│ │ Usually offloaded to NIC hardware         │  │
│ │ Computed at transmit, verified at receive │  │
│ └───────────────────────────────────────────┘  │
│                                                │
└────────────────────────────────────────────────┘

Total minimum frame: 14 (Eth header) + 46 (payload) + 4 (FCS) = 64 bytes
Total maximum frame: 14 (Eth header) + 1500 (payload) + 4 (FCS) = 1518 bytes
Jumbo frames: 14 + 9000+ + 4 = 9018+ bytes

Linux sk_buff Metadata for Ethernet:
┌───────────────────────────────────────────────────┐
│ struct ethhdr {                                   │
│     unsigned char h_dest[ETH_ALEN];   /* 6 bytes*/│
│     unsigned char h_source[ETH_ALEN]; /* 6 bytes*/│
│     __be16 h_proto;                   /* 2 bytes*/│
│ };                                                │
│                                                   │
│ sk_buff fields:                                   │
│   skb->dev            ← incoming/outgoing device│
│   skb->mac_header     ← offset to ethhdr        │
│   skb->protocol       ← htons(h_proto)          │
│   skb->pkt_type       ← reception classification│
│   skb->skb_iif        ← ingress interface index │
│ └───────────────────────────────────────────────┘

pkt_type Classification:
PACKET_HOST         (0) Destined for this host (via MAC address match)
PACKET_BROADCAST    (1) Broadcast frame (FF:FF:FF:FF:FF:FF)
PACKET_MULTICAST    (2) Multicast frame (01:00:5E:* for IPv4)
PACKET_OTHERHOST    (3) Frame for another host (promiscuous mode)
PACKET_INCOMING     (4) Sent to us (locally originated loopback)
PACKET_LOOPBACK     (5) MC/BC for local host
PACKET_FASTROUTE    (6) Usually not set (legacy)
PACKET_OUTGOING     (7) Locally originated outgoing
```

### 5.2 VLAN (802.1Q) Metadata

```
VLAN Tag Structure
──────────────────────────────────────────────────

Legacy Frame Format (802.1Q):
┌───────────────────────────────────────────┐
│ Destination MAC       (6 bytes)           │
├───────────────────────────────────────────┤
│ Source MAC            (6 bytes)           │
├───────────────────────────────────────────┤
│ EtherType = 0x8100    (2 bytes)           │
├───────────────────────────────────────────┤
│ VLAN Tag (4 bytes)                        │
│ ┌───────────────────────────────────────┐ │
│ │ Priority    (3 bits)  PCP             │ │
│ │ ├ 0-7(0=best effort, 7=network control) │
│ │                                       │ │
│ │ DEI         (1 bit)   Drop Eligible   │ │
│ │ ├─ 0 = frame not eligible for drop    │ │
│ │ ├─ 1 = frame is drop eligible         │ │
│ │                                       │ │
│ │ VID         (12 bits) VLAN ID         │ │
│ │ ├─ 0000 = no VLAN tag (reserved)      │ │
│ │ ├─ 1 to 4094 = valid VLAN IDs         │ │
│ │ ├─ 4095 = reserved (reserved)         │ │
│ └───────────────────────────────────────┘ │
├───────────────────────────────────────────┤
│ Original EtherType    (2 bytes)           │
│ (e.g., 0x0800 for IPv4)                   │
├───────────────────────────────────────────┤
│ Payload               (variable)          │
├───────────────────────────────────────────┤
│ FCS                   (4 bytes)           │
└───────────────────────────────────────────┘

QinQ (802.1ad) Format (double-tagged):
┌───────────────────────────────────────────┐
│ Destination MAC       (6 bytes)           │
├───────────────────────────────────────────┤
│ Source MAC            (6 bytes)           │
├───────────────────────────────────────────┤
│ Outer VLAN EtherType = 0x88A8 (2 bytes)   │
├───────────────────────────────────────────┤
│ Outer VLAN Tag        (4 bytes)           │
│ (PCP[3] | DEI[1] | VLAN_ID[12])           │
├───────────────────────────────────────────┤
│ Inner VLAN EtherType = 0x8100 (2 bytes)   │
├───────────────────────────────────────────┤
│ Inner VLAN Tag        (4 bytes)           │
│ (PCP[3] | DEI[1] | VLAN_ID[12])           │
├───────────────────────────────────────────┤
│ EtherType            (2 bytes) (IPv4, etc)│
├───────────────────────────────────────────┤
│ Payload               (variable)          │
├───────────────────────────────────────────┤
│ FCS                   (4 bytes)           │
└───────────────────────────────────────────┘

Linux sk_buff VLAN Metadata:
┌──────────────────────────────────────────┐
│ skb->vlan_proto                          │
│  ├─ htons(0x8100) = standard 802.1Q      │
│  ├─ htons(0x88A8) = provider 802.1ad     │
│  ├─ htons(0x9100) = alternate provider   │
│  └─ htons(0x9200) = alternate alternate  │
│                                          │
│ skb->vlan_tci (VLAN Tag Control Info)    │
│  ├─ [15:13] = PCP (Priority)             │
│  ├─ [12] = DEI (Drop Eligible)           │
│  └─ [11:0] = VID (VLAN ID)               │
│                                          │
│ VLAN_TAG_PRESENT flag bit                │
│  └─ Indicates valid VLAN tag in skb      │
│                                          │
└──────────────────────────────────────────┘

Accessing VLAN data from sk_buff:
#define vlan_tx_tag_present(__skb)  (((__skb)->vlan_tci) & VLAN_TAG_PRESENT)
#define vlan_tx_tag_get(__skb)      (((__skb)->vlan_tci) & ~VLAN_TAG_PRESENT)
#define vlan_tx_tag_get_id(__skb)   (((__skb)->vlan_tci) & VLAN_VID_MASK)
#define vlan_tx_tag_get_prio(__skb) ((((__skb)->vlan_tci) & VLAN_PRIO_MASK) >> VLAN_PRIO_SHIFT)
```

### 5.3 MACsec Metadata

```
MACsec (IEEE 802.1AE) - MAC Layer Security
────────────────────────────────────────────

Frame Structure with MACsec:
┌─────────────────────────────────────────────────┐
│ Destination MAC       (6 bytes)                 │
├─────────────────────────────────────────────────┤
│ Source MAC            (6 bytes)                 │
├─────────────────────────────────────────────────┤
│ 802.1Q Tag (optional) (4 bytes)                 │
├─────────────────────────────────────────────────┤
│ MACsec EtherType = 0x88E5 (2 bytes)             │
├─────────────────────────────────────────────────┤
│ MACsec Header        (2 bytes)                  │
│ ┌─────────────────────────────────────────────┐ │
│ │ TCI (Tag Control Info) [1 byte]             │ │
│ │ ├─ V (Version)[1]: Always 0 (current spec)  │ │
│ │ ├─ ES (End Station)[1]: Endpoint device     │ │
│ │ ├─ SC (SCI Present)[1]: SecTag valid        │ │
│ │ ├─ SCB (SCI Bypass)[1]: Skip SecTag         │ │
│ │ ├─ E (Encrypted)[1]: Frame encrypted        │ │
│ │ ├─ C (Integrity Check)[1]: ICV present      │ │
│ │ ├─ AN (Association Number)[2]: SA index     │ │
│ │                                             │ │
│ │ SAL (Short Packet Number)[1 byte]         │ │
│ │ └─ Upper 8 bits of 32-bit packet number   │ │
│ └─────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────┤
│ SCI (Secure Channel ID) (8 bytes, optional)     │
│ ┌─────────────────────────────────────────────┐ │
│ │ MAC Address [6 bytes] + Port ID [2 bytes]   │ │
│ │ Identifies the transmitting SC              │ │
│ └─────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────┤
│ Packet Number         (4 bytes)                 │
│ (full 32-bit sequence number)                   │
├─────────────────────────────────────────────────┤
│ Encrypted Payload     (variable)                │
│ (AES-GCM encryption, GCM-AES-128 typical)       │
├─────────────────────────────────────────────────┤
│ ICV (Integrity Check Value) (16 bytes)          │
│ (GCM authentication tag)                        │
├─────────────────────────────────────────────────┤
│ FCS                   (4 bytes)                 │
└─────────────────────────────────────────────────┘

Linux sk_buff MACsec Support:
struct sk_buff {
    ...
    struct macsec_info {
        bool valid;              /* MACSEC processed */
        u32 pn;                  /* Packet number */
        u8 sci[8];               /* Secure Channel ID */
        u16 an;                  /* Association number */
    } macsec;
    
    /* Also uses:
       skb->mark for classification
       skb->tc_index for MACsec rule matching
    */
}

Key Points:
- MACsec provides hop-by-hop encryption
- Unlike IPSec, operates at MAC layer (before IP routing)
- Every switch/router can have different MACsec key
- Symmetric keys (pre-shared or RADIUS/802.1X)
- GCM-AES provides both encryption + authentication
- Typical implementation in hardware (switch ASIC)
- Software implementation via ksecure (kernel module)
```

---

## 6. Network Layer Metadata

### 6.1 IPv4 Header Metadata

```
IPv4 Header with Metadata Analysis
────────────────────────────────────────────────────

On-Wire IPv4 Header (20 bytes minimum):
┌───────────────────────────────────────────────┐
│ Version (4 bits) | IHL (4 bits)               │ Byte 0
│ ├─ Version: Always 4 for IPv4                 │
│ ├─ IHL: Header Length (5-15, in 32-bit words) │
│ │   5 = 20 bytes (minimum, no options)        │
│ │   >5 = options present                      │
│                                               │
│ DSCP (6 bits) | ECN (2 bits)                  │ Byte 1
│ ├─ DSCP: Differentiated Services Code Point   │
│ │   (formerly ToS - Type of Service)         │
│ │                                            │
│ │   000000 = BE (Best Effort, default)       │
│ │   001000 = CS1 (Class Selector 1)          │
│ │   010000 = CS2                              │
│ │   011000 = CS3                              │
│ │   100000 = CS4                              │
│ │   101000 = CS5 (Expedited Forwarding)       │
│ │   110000 = CS6                              │
│ │   111000 = CS7 (Network Control)           │
│ │   101110 = EF (Expedited Forwarding)       │
│ │   010010 = AF11 (Assured Forwarding Class) │
│ │                                            │
│ ├─ ECN: Explicit Congestion Notification     │
│ │   00 = Not ECT (not ECN capable)           │
│ │   01 = ECT(1)                              │
│ │   10 = ECT(0)                              │
│ │   11 = CE (Congestion Experienced)         │
│                                              │
├──────────────────────────────────────────────┤
│ Total Length               (2 bytes)         │ Bytes 2-3
│ ├─ Length of IP header + payload             │
│ ├─ Range: 20 (header only) to 65535 bytes    │
│ └─ Used by driver to know payload length      │
│                                               │
├───────────────────────────────────────────────┤
│ Identification             (2 bytes)          │ Bytes 4-5
│ ├─ Unique identifier for each packet          │
│ ├─ Used to reassemble fragmented packets      │
│ └─ Linux: Usually incremented per packet      │
│                                               │
├───────────────────────────────────────────────┤
│ Flags (3 bits) | Fragment Offset (13 bits)    │ Bytes 6-7
│ ├─ Bit 0: Reserved (must be 0)               │
│ ├─ Bit 1: DF (Don't Fragment)                │
│ │   0 = OK to fragment if needed             │
│ │   1 = Do not fragment (PMTUD reliant)      │
│ ├─ Bit 2: MF (More Fragments)                │
│ │   0 = Last fragment                        │
│ │   1 = More fragments follow                │
│ ├─ Fragment Offset (13 bits)                 │
│ │   Offset of this fragment in original      │
│ │   packet (in 8-byte units, 0-8191)         │
│ │   0 = First fragment                       │
│ │   >0 = Subsequent fragments                │
│                                               │
├───────────────────────────────────────────────┤
│ TTL (Time To Live)         (1 byte)           │ Byte 8
│ ├─ Hop limit (typically 64 for initial)      │
│ ├─ Decremented by 1 at each router           │
│ ├─ Packet discarded if TTL reaches 0         │
│ ├─ 0 = not valid for transmission            │
│ └─ Used by traceroute                        │
│                                               │
├───────────────────────────────────────────────┤
│ Protocol                   (1 byte)           │ Byte 9
│ ├─ 1 = ICMP (Internet Control Message Proto) │
│ ├─ 6 = TCP (Transmission Control Protocol)   │
│ ├─ 17 = UDP (User Datagram Protocol)         │
│ ├─ 47 = GRE (Generic Routing Encapsulation) │
│ ├─ 50 = ESP (IPSec Encapsulating Security) │
│ ├─ 51 = AH (IPSec Authentication Header)    │
│ ├─ 132 = SCTP (Stream Control Trans Proto)  │
│ └─ 255 = reserved                            │
│                                               │
├───────────────────────────────────────────────┤
│ Header Checksum            (2 bytes)           │ Bytes 10-11
│ ├─ Checksum of IPv4 header only (not payload)│
│ ├─ Usually offloaded to NIC hardware         │
│ ├─ Recomputed by each router (TTL change)    │
│ └─ Not used in newer protocols (IPv6)        │
│                                               │
├───────────────────────────────────────────────┤
│ Source IP Address          (4 bytes)           │ Bytes 12-15
│ └─ Sender's IPv4 address (dotted decimal)     │
│                                               │
├───────────────────────────────────────────────┤
│ Destination IP Address     (4 bytes)           │ Bytes 16-19
│ └─ Recipient's IPv4 address (dotted decimal)  │
│                                               │
├───────────────────────────────────────────────┤
│ Options (optional)         (variable)          │ Bytes 20+
│ ├─ Only if IHL > 5                           │
│ ├─ Padded to 32-bit boundary                 │
│ ├─ RR (Record Route): log routers traversed  │
│ ├─ TS (Timestamp): record timestamps         │
│ ├─ SRR (Strict Source Route): specify route  │
│ └─ Rarely used in modern networks            │
│                                               │
└───────────────────────────────────────────────┘

Linux sk_buff IPv4 Metadata:
┌─────────────────────────────────────────────────┐
│ struct iphdr *ip_hdr(const struct sk_buff *s) │
│ {                                              │
│     return (struct iphdr *)skb_network_header()│
│ }                                              │
│                                                │
│ Key fields in sk_buff for IPv4:               │
│   skb->network_header  ← offset to IP start   │
│   skb->transport_header ← offset to L4 hdr    │
│   skb->len            ← total packet length   │
│   skb->data_len       ← payload (frag) length │
│   skb->frag_off       ← fragment offset + flags│
│   skb->ttl            ← cached TTL value      │
│   skb->priority       ← mapped from DSCP      │
│   skb->mark           ← policy mark           │
│   skb->tc_index       ← qdisc classification  │
│                                                │
└─────────────────────────────────────────────────┘

Fragmentation Handling:
┌────────────────────────────────────────────────┐
│                                                │
│ Original Packet (1500 bytes)                  │
│ ┌──────────────────────────────────────────┐  │
│ │ IPv4 Header (20 bytes)                   │  │
│ │ Payload (1480 bytes)                     │  │
│ └──────────────────────────────────────────┘  │
│                                                │
│ Network path MTU = 500 bytes → Fragmentation │
│                                                │
│ Fragment 1 (520 bytes)                        │
│ ┌──────────────────────────────────────────┐  │
│ │ IPv4 Header: ID=12345, Offset=0, MF=1   │  │
│ │ Payload (500 bytes)                      │  │
│ └──────────────────────────────────────────┘  │
│                                                │
│ Fragment 2 (520 bytes)                        │
│ ┌──────────────────────────────────────────┐  │
│ │ IPv4 Header: ID=12345, Offset=62.5*8=500│  │ (offset in 8-byte units)
│ │ Payload (500 bytes)                      │  │
│ │ MF=1 (more fragments)                    │  │
│ └──────────────────────────────────────────┘  │
│                                                │
│ Fragment 3 (480 bytes)                        │
│ ┌──────────────────────────────────────────┐  │
│ │ IPv4 Header: ID=12345, Offset=125*8=1000│  │
│ │ Payload (480 bytes)                      │  │
│ │ MF=0 (last fragment)                     │  │
│ └──────────────────────────────────────────┘  │
│                                                │
│ Reassembly at destination:                    │
│   - IP reassembly buffer holds fragments      │
│   - Identified by (source, dest, ID, proto)   │
│   - Timeout after 15 seconds (configurable)  │
│   - Linux: /proc/sys/net/ipv4/ipfrag_time    │
│   - When all fragments received, pass to L4   │
│   - Reassembled sk_buff looks normal          │
│                                                │
└────────────────────────────────────────────────┘

Linux IPv4 Fragmentation Handler Registration:
struct net_protocol {
    int (*handler)(struct sk_buff *skb);  ← UDP, TCP, etc.
    void (*err_handler)(struct sk_buff *skb, u32 info);  ← Error handling
    ...
};

/* In ip_input.c: */
static const struct net_protocol tcp_protocol = {
    .handler = tcp_v4_rcv,
    .err_handler = tcp_v4_err,
    ...
};

static const struct net_protocol udp_protocol = {
    .handler = udp_rcv,
    .err_handler = udp_err,
    ...
};

/* Called from ip_local_deliver() */
if (ipprot->handler(skb) > 0)
    goto out;  /* handler consumed packet */
```

### 6.2 IPv6 Header Metadata

```
IPv6 Header Structure
──────────────────────────────────────────────────

Fixed IPv6 Header (40 bytes):
┌──────────────────────────────────────────────┐
│ Version (4) | Traffic Class (8) | Flow Label (20)│ Bytes 0-3
│                                              │
│ Version: 6 (4 bits)                         │
│                                              │
│ Traffic Class: 8 bits (like IPv4 DSCP+ECN) │
│ ├─ DSCP (6 bits): Differentiated Services  │
│ └─ ECN (2 bits): Explicit Congestion Notif │
│                                              │
│ Flow Label: 20 bits                         │
│ └─ Identifies flow for QoS treatment        │
│    All packets of same source-dest pair     │
│    with same flow label get same handling   │
│    Assigned by source or middlebox          │
│                                              │
├──────────────────────────────────────────────┤
│ Payload Length                (2 bytes)      │ Bytes 4-5
│ ├─ Length of extension headers + payload    │
│ ├─ Does NOT include IPv6 fixed header (40B)│
│ ├─ Max 65535 bytes (jumbograms: >65535)    │
│ └─ Jumbograms require jumbo payload option  │
│                                              │
├──────────────────────────────────────────────┤
│ Next Header                    (1 byte)      │ Byte 6
│ ├─ Type of next header (TCP, UDP, etc)      │
│ ├─ IPv6 allows chaining via extension hdrs  │
│ ├─ Values same as IPv4 Protocol field       │
│ ├─ Special: 0 = Hop-by-hop options         │
│ │            60 = Destination options       │
│ │            43 = Routing header            │
│ │            44 = Fragment header           │
│ │            59 = No Next Header            │
│ └─ Can form chain: IPv6 → Hop-by-hop →    │
│    Routing → Fragment → TCP/UDP             │
│                                              │
├──────────────────────────────────────────────┤
│ Hop Limit                      (1 byte)      │ Byte 7
│ ├─ Like IPv4 TTL (usually 64 or 255)       │
│ ├─ Decremented at each hop                  │
│ ├─ When 0, packet discarded                 │
│ └─ Used for traceroute (ICMPv6)             │
│                                              │
├──────────────────────────────────────────────┤
│ Source Address                 (16 bytes)    │ Bytes 8-23
│ ├─ 128-bit IPv6 address                     │
│ ├─ Format: 8 groups of 4 hex digits         │
│ ├─ Example: 2001:0db8:85a3::8a2e:0370:7334│
│ ├─ :: means consecutive zero groups         │
│ └─ Can be global, link-local, etc           │
│                                              │
├──────────────────────────────────────────────┤
│ Destination Address            (16 bytes)    │ Bytes 24-39
│ ├─ 128-bit IPv6 address                     │
│ ├─ Can be unicast, multicast, or anycast    │
│ └─ Same format as source address            │
│                                              │
├──────────────────────────────────────────────┤
│ Extension Headers (optional, variable)      │
│ ├─ Hop-by-hop Options Header               │
│ ├─ Destination Options Header              │
│ ├─ Routing Header (specify route)          │
│ ├─ Fragment Header                         │
│ ├─ Authentication Header (AH)              │
│ ├─ Encapsulating Security Payload (ESP)    │
│ └─ Destination Options (for end-to-end)    │
│                                              │
│ Each extension header format:               │
│ ┌──────────────────────────────────────┐   │
│ │ Next Header (1 byte) - what follows  │   │
│ ├──────────────────────────────────────┤   │
│ │ Header Extension Length (1 byte)     │   │
│ │ ├─ In 8-byte units (excluding first) │   │
│ │ ├─ Length 0 = 8 bytes total          │   │
│ │ ├─ Length 1 = 16 bytes total         │   │
│ │ └─ Must account for alignment        │   │
│ ├──────────────────────────────────────┤   │
│ │ Type-specific data (variable)        │   │
│ │ └─ Padded to 8-byte alignment        │   │
│ └──────────────────────────────────────┘   │
│                                              │
└──────────────────────────────────────────────┘

IPv6 Fragmentation (via Fragment Header):
┌────────────────────────────────────────────────────┐
│ Fragment Header (8 bytes)                          │
├────────────────────────────────────────────────────┤
│ Next Header         (1 byte)                       │
├────────────────────────────────────────────────────┤
│ Reserved            (1 byte) - must be 0           │
├────────────────────────────────────────────────────┤
│ Fragment Offset (13 bits) | Reserved (2) | M (1)  │
│ ├─ Offset: Fragment position in 8-byte units     │
│ ├─ M (More Fragments): 0=last, 1=more             │
│ └─ Range: 0-8191 (max packet 65536 bytes)        │
├────────────────────────────────────────────────────┤
│ Identification      (4 bytes)                      │
│ ├─ Unique ID for all fragments of same packet    │
│ ├─ Determined by source, dest, identification    │
│ └─ Helps prevent stale fragment reassembly       │
└────────────────────────────────────────────────────┘

Unlike IPv4:
- IPv6 fragmentation only at source (no router frags)
- Requires PMTUD (Path MTU Discovery)
- ICMPv6 "Packet Too Big" triggers retransmission
- Simpler than IPv4 but requires PMTUD to work

Linux sk_buff IPv6 Metadata:
struct ipv6hdr {
    __u8 version:4,
         prio:4;
    __u8 flow_lbl[3];
    __be16 payload_len;
    __u8 nexthdr;
    __u8 hop_limit;
    struct in6_addr saddr;
    struct in6_addr daddr;
};

sk_buff fields:
  skb->network_header  ← offset to IPv6 header
  skb->transport_header ← offset to TCP/UDP/etc
  skb->priority        ← mapped from Traffic Class
  skb->flow_dissector_key ← 5-tuple for routing
  skb->hash            ← IPv6 flow hash (RSS)
  skb->rxhash          ← hardware hash from NIC
```

---

## 7. Transport Layer Metadata

### 7.1 TCP Header and State Metadata

```
TCP Segment Structure
──────────────────────────────────────────────────

TCP Header (20 bytes minimum, up to 60 with options):
┌────────────────────────────────────────────────┐
│ Source Port                    (2 bytes)        │ Bytes 0-1
│ ├─ 0-65535 range                              │
│ ├─ 0-1023: Well-known ports (reserved)        │
│ │   22 = SSH, 80 = HTTP, 443 = HTTPS         │
│ ├─ 1024-49151: Registered ports               │
│ └─ 49152-65535: Ephemeral/dynamic ports       │
│                                                │
├────────────────────────────────────────────────┤
│ Destination Port               (2 bytes)        │ Bytes 2-3
│ └─ Server listening port                       │
│                                                │
├────────────────────────────────────────────────┤
│ Sequence Number                (4 bytes)        │ Bytes 4-7
│ ├─ Initial value chosen by sender              │
│ ├─ Incremented by number of bytes sent         │
│ ├─ Used for reliable ordered delivery          │
│ ├─ Wraps at 2^32 (sequence number space)       │
│ └─ TCP in kernel tracks ISS (Initial Seq#)     │
│                                                │
├────────────────────────────────────────────────┤
│ Acknowledgment Number          (4 bytes)        │ Bytes 8-11
│ ├─ Sequence number of next expected byte       │
│ ├─ Valid only if ACK flag is set               │
│ ├─ Acknowledges all data up to (ACK - 1)       │
│ └─ Cumulative: only highest ACK sent           │
│                                                │
├────────────────────────────────────────────────┤
│ Data Offset (4) | Reserved (3) | Flags (9)    │ Bytes 12-13
│ ├─ Data Offset: TCP header length in 32-bit   │
│ │   Minimum 5 (20 bytes), Max 15 (60 bytes)   │
│ │   Specifies where payload begins             │
│ │                                              │
│ └─ Flags (9 bits):                            │
│    ├─ CWR (Congestion Window Reduced) [bit 7] │
│    │   Set by sender to indicate ECE received │
│    ├─ ECE (ECN-Echo) [bit 6]                 │
│    │   Indicates congestion (if SYN: ECT)     │
│    ├─ URG (Urgent Pointer Valid) [bit 5]     │
│    │   Urgent data present (rarely used)      │
│    ├─ ACK (Acknowledgment Valid) [bit 4]     │
│    │   ACK number field is valid              │
│    ├─ PSH (Push) [bit 3]                    │
│    │   Recipient should push data to app      │
│    ├─ RST (Reset) [bit 2]                    │
│    │   Abruptly close connection              │
│    ├─ SYN (Synchronize) [bit 1]              │
│    │   Initiate connection, ISS in seq#       │
│    ├─ FIN (Finish) [bit 0]                   │
│    │   Initiator is done sending              │
│    │   Packets may still arrive after FIN     │
│    └─ Reserved bits [bits 8-0]: must be 0    │
│                                                │
├────────────────────────────────────────────────┤
│ Window Size                    (2 bytes)        │ Bytes 14-15
│ ├─ Sliding window: how much sender can send    │
│ ├─ Number of bytes receiver is willing to get  │
│ ├─ 0-65535 range                              │
│ ├─ Can be scaled up to 1GB via option          │
│ └─ Flow control mechanism (prevent overflow)   │
│                                                │
├────────────────────────────────────────────────┤
│ Checksum                       (2 bytes)        │ Bytes 16-17
│ ├─ Covers TCP header + payload + pseudo-hdr    │
│ ├─ Pseudo-header includes:                     │
│ │   Source IP, Dest IP, Protocol, Length      │
│ ├─ Required (never 0, unlike UDP)              │
│ └─ Usually offloaded to NIC (CHECKSUM_PARTIAL)│
│                                                │
├────────────────────────────────────────────────┤
│ Urgent Pointer                 (2 bytes)        │ Bytes 18-19
│ ├─ Offset to urgent data (if URG flag set)    │
│ ├─ Rarely used in modern applications          │
│ └─ Used by telnet for CTRL-C signal           │
│                                                │
├────────────────────────────────────────────────┤
│ Options (if Data Offset > 5)   (variable)      │
│ ├─ Padded to 32-bit boundary                   │
│ ├─ Maximum 40 bytes (60 total - 20 fixed)     │
│ │                                              │
│ ├─ Common Options:                            │
│ │   MSS (Maximum Segment Size) [4 bytes]      │
│ │   ├─ Negotiated during SYN                   │
│ │   ├─ Typical: 1460 (1500 MTU - 40 hdr)     │
│ │   ├─ Prevents excessive fragmentation        │
│ │   └─ Linux default: /proc/.../tcp_mss_       │
│ │                                              │
│ │   Window Scale [4 bytes]                     │
│ │   ├─ Multiplies window size by 2^N           │
│ │   ├─ N = 0-14 (1, 2, 4, 8, ... 16384)       │
│ │   ├─ Allows window >64KB                     │
│ │   └─ Only in SYN/SYN-ACK                    │
│ │                                              │
│ │   SACK (Selective Acknowledgment) [2 bytes] │
│ │   ├─ Indicates support for SACK              │
│ │   ├─ Allows receiver to ACK non-contiguous  │
│ │   ├─ Efficient loss recovery                 │
│ │   └─ SACK blocks in separate packets        │
│ │                                              │
│ │   Timestamps [10 bytes]                      │
│ │   ├─ TSval (Timestamp Value): sender clock  │
│ │   ├─ TSecr (Timestamp Echo Reply): peer ts  │
│ │   ├─ Used for RTT calculation                │
│ │   ├─ Protects against wrapped seq numbers   │
│ │   └─ Virtually standard in modern TCP       │
│ │                                              │
│ └─ Less common:                               │
│    TCP Fast Open (TFO): Bypass 3-way handshake│
│    MD5 Authentication: Pre-shared key auth    │
│    UMP (User Mapping): Legacy                 │
│                                                │
└────────────────────────────────────────────────┘

Linux TCP Connection State Metadata:
┌──────────────────────────────────────────────────┐
│ struct tcp_sock {                               │
│     /* TCP-specific control block */            │
│     u32 pred_flags;    ← Fast path predictable │
│     u32 rcv_nxt;       ← Next expected seq#    │
│     u32 rcv_wup;       ← Receive window update │
│     u32 snd_una;       ← Send unacknowledged   │
│     u32 snd_sml;       ← Send small            │
│     u32 rcv_tstamp;    ← Last rcv timestamp    │
│     u32 lsndtime;      ← Last send time        │
│     u32 last_oow_ack_time;                      │
│     u16 rcv_wnd;       ← Advertised window    │
│     u16 mss_cache;     ← Cached MSS value     │
│                                                │
│     /* SACK blocks for reordering */           │
│     u16 num_sacks;     ← # of SACK blocks     │
│     u16 num_dupack;    ← # of duplicate acks  │
│                                                │
│     /* Congestion control state */            │
│     u32 snd_ssthresh;  ← Slow start threshold │
│     u32 snd_cwnd;      ← Congestion window    │
│     u32 snd_cwnd_cnt;  ← Counter for cwnd     │
│     u32 snd_cwnd_clamp;← Max congestion window│
│                                                │
│     /* Round trip time estimation */          │
│     u32 srtt_us;       ← Smoothed RTT (us)    │
│     u32 mdev_us;       ← Mean deviation        │
│     u32 rttvar_us;     ← RTT variance         │
│     u32 rtt_min;       ← Minimum RTT           │
│                                                │
│     /* Loss detection */                      │
│     u32 prior_ssthresh;← SSThresh at loss     │
│     u32 prior_cwnd;    ← Cwnd at loss         │
│     u32 undo_marker;   ← Undo point           │
│     u32 undo_retrans;  ← Retrans to undo      │
│                                                │
│     /* Fast retransmit / fast recovery */     │
│     u16 dupacks;       ← Duplicate ACK count  │
│     u8 tlp_high_seq;   ← TLP high seq         │
│     u8 pending_retrans;← Pending retransmits  │
│                                                │
│     /* Send buffer management */              │
│     u32 write_seq;     ← Write pointer        │
│     u32 pushed_seq;    ← Pushed to network    │
│     u32 lost_out;      ← # lost segments      │
│     u32 sacked_out;    ← # SACK'd segments    │
│     u32 retrans_out;   ← # retransmitted      │
│                                                │
│ } __aligned(8);                                │
│                                                │
│ Typically ~3KB per active TCP connection       │
│ × millions = significant memory in high        │
│ connection count servers (need tcp_tw_reuse)   │
│                                                │
└──────────────────────────────────────────────────┘

TCP Connection States:
TCP_ESTABLISHED    - Connection active
TCP_SYN_SENT       - Client initiated, awaiting ACK
TCP_SYN_RECV       - Server received SYN, sent SYN-ACK
TCP_FIN_WAIT1      - Sent FIN, awaiting ACK
TCP_FIN_WAIT2      - FIN acked, awaiting peer FIN
TCP_TIME_WAIT      - Waiting 2*MSL after FIN
TCP_CLOSE          - Connection closed (not used much)
TCP_CLOSE_WAIT     - Peer sent FIN, awaiting app close
TCP_LAST_ACK       - Sent FIN, awaiting final ACK
TCP_LISTEN         - Listening for incoming (server)
TCP_CLOSING        - FIN sent & received (simultaneous)

State Machine Diagram:
(see detailed ASCII diagram in appendix)
```

### 7.2 UDP Header and Metadata

```
UDP Header Structure
───────────────────────────────────────────────

Fixed 8-byte Header (minimal):
┌────────────────────────────────────┐
│ Source Port              (2 bytes)  │ Bytes 0-1
│ ├─ 0-65535 range                  │
│ ├─ Can be 0 (no reply expected)    │
│ └─ Client ephemeral or server      │
│                                    │
├────────────────────────────────────┤
│ Destination Port         (2 bytes)  │ Bytes 2-3
│ ├─ Server listening port            │
│ ├─ 53 = DNS                         │
│ ├─ 67 = DHCP                        │
│ ├─ 123 = NTP                        │
│ ├─ 161 = SNMP                       │
│ ├─ 514 = Syslog                     │
│ └─ 5353 = mDNS                      │
│                                    │
├────────────────────────────────────┤
│ Length                   (2 bytes)  │ Bytes 4-5
│ ├─ UDP header + payload             │
│ ├─ Minimum: 8 bytes (no payload)   │
│ ├─ Maximum: 65535 bytes             │
│ └─ Usually matches (IP total - IP) │
│                                    │
├────────────────────────────────────┤
│ Checksum                 (2 bytes)  │ Bytes 6-7
│ ├─ Covers pseudo-hdr + UDP + data   │
│ ├─ Can be 0 (means no checksum)    │
│ ├─ IPv6 requires checksum (no 0)    │
│ └─ Usually offloaded to NIC         │
│                                    │
├────────────────────────────────────┤
│ Payload (variable, 0-65507 bytes)   │
│ └─ Application data                 │
│                                    │
└────────────────────────────────────┘

Advantages of UDP:
- No connection state (stateless)
- Lower latency (no handshake)
- Lower overhead (8-byte header vs 20+ TCP)
- Multicast/broadcast capable
- No flow control (applications handle it)
- Simpler in-kernel implementation

Linux UDP Socket Metadata:
struct udp_sock {
    struct inet_sock inet;          /* IP layer info */
    
    int encap_type;                 /* UDP encapsulation */
    struct udp_encap_info *encap_rcv;
    
    unsigned int nosock : 1,        /* UDP socket from NF */
                convert_csum : 1,   /* Convert checksum */
                gso_segment : 1,    /* UDP GSO enabled */
                accept_udplite : 1; /* Accept UDPLite */
};

Key differences from TCP:
- No retransmission (application's responsibility)
- No sequence numbers (out-of-order delivery OK)
- No congestion control (must implement in app)
- No ACKs (no backpressure from receiver)
- Connectionless (packet-oriented)
- Less state per "flow"
```

### 7.3 SCTP Header Metadata

```
SCTP (Stream Control Transmission Protocol)
────────────────────────────────────────────────

SCTP Common Header (12 bytes):
┌──────────────────────────────────────┐
│ Source Port                (2 bytes) │ Bytes 0-1
│ ├─ 0-65535, similar to TCP/UDP       │
│ └─ Server listening port              │
│                                      │
├──────────────────────────────────────┤
│ Destination Port           (2 bytes) │ Bytes 2-3
│ └─ Destination listening port         │
│                                      │
├──────────────────────────────────────┤
│ Verification Tag           (4 bytes) │ Bytes 4-7
│ ├─ Endpoint identification            │
│ ├─ Sent by peer in INIT-ACK           │
│ ├─ Must be echoed in every packet     │
│ └─ Protects against connection theft  │
│                                      │
├──────────────────────────────────────┤
│ Checksum                   (4 bytes) │ Bytes 8-11
│ ├─ Adler-32 (not CRC32 like TCP/UDP) │
│ ├─ Covers entire SCTP packet          │
│ ├─ Sometimes optional (app decision)  │
│ └─ Different algorithm for hardware   │
│                                      │
├──────────────────────────────────────┤
│ Chunks (variable, 16+ bytes each)   │
│ └─ Variable-length chunks follow     │
│                                      │
└──────────────────────────────────────┘

SCTP Chunks (within SCTP packet):
Each chunk has:
┌─────────────────────────────────────┐
│ Type (1 byte)                       │ Chunk type
│ ├─ 0 = DATA                         │
│ ├─ 1 = INIT                         │
│ ├─ 2 = INIT-ACK                     │
│ ├─ 3 = SACK                         │
│ ├─ 4 = HEARTBEAT                    │
│ ├─ 5 = HEARTBEAT-ACK                │
│ ├─ 6 = ABORT                        │
│ ├─ 7 = SHUTDOWN                     │
│ ├─ 8 = SHUTDOWN-ACK                 │
│ ├─ 9 = ERROR                        │
│ ├─ 10 = COOKIE-ECHO                 │
│ ├─ 11 = COOKIE-ACK                  │
│ ├─ 12 = ECNE                        │
│ ├─ 13 = CWR                         │
│ └─ 14 = SHUTDOWN-COMPLETE           │
│                                     │
├─────────────────────────────────────┤
│ Flags (1 byte)                      │
│ ├─ E (beginning), B (beginning),    │
│ ├─ E (end) for fragmented streams   │
│ └─ U (unordered delivery)           │
│                                     │
├─────────────────────────────────────┤
│ Length (2 bytes)                    │
│ ├─ Chunk size (header + data)       │
│ └─ Padded to 4-byte boundary        │
│                                     │
├─────────────────────────────────────┤
│ Value (variable)                    │
│ └─ Chunk-type-specific data         │
│                                     │
└─────────────────────────────────────┘

Key Advantages over TCP:
- Message-oriented (not byte-stream)
- Multi-streaming (independent streams in one association)
- Multi-homing (multiple IP addresses per endpoint)
- Association concept (like connection, but more flexible)
- Faster failure detection (heartbeat)
- Better for telecom, signaling (SS7, SIGTRAN)

Linux SCTP Metadata (kernel/net/sctp/):
- Maintained in sctp_association structure
- Per-association state management
- Chunk reassembly buffer
- Out-of-order message handling
- Stream sequence number tracking
```

---

## 8. Application Layer Metadata

### 8.1 HTTP Header Metadata

```
HTTP Request/Response Metadata
────────────────────────────────────────────────

HTTP/1.1 Request Line + Headers:
┌──────────────────────────────────────────────┐
│ Request Line (variable)                      │
│ METHOD /path?query HTTP/1.1\r\n              │
│ ├─ METHOD: GET, POST, PUT, DELETE, etc       │
│ ├─ /path: Request URI path                   │
│ ├─ ?query: Query string (optional)           │
│ └─ HTTP/1.1: Protocol version                │
│                                              │
├──────────────────────────────────────────────┤
│ Headers (variable, key: value\r\n)          │
│                                              │
│ Host: example.com:80                         │
│ ├─ Server identity                           │
│ ├─ Required in HTTP/1.1                      │
│ └─ Used for virtual hosting                  │
│                                              │
│ User-Agent: Mozilla/5.0 ...                  │
│ ├─ Client software identification            │
│ ├─ Used for content negotiation              │
│ └─ Popular for bot detection                 │
│                                              │
│ Content-Type: application/json               │
│ ├─ MIME type of request/response body        │
│ ├─ charset parameter for encoding            │
│ └─ Server uses to deserialize                │
│                                              │
│ Content-Length: 1234                         │
│ ├─ Size of request/response body in bytes    │
│ ├─ Required for determining message boundary │
│ └─ Helps with Content-Encoding               │
│                                              │
│ Connection: keep-alive                       │
│ ├─ keep-alive or close                       │
│ ├─ keep-alive reuses TCP connection          │
│ ├─ Crucial for HTTP/1.1 performance          │
│ └─ Default in HTTP/1.1 (close in HTTP/1.0)   │
│                                              │
│ Cookie: session_id=abc123; path=/            │
│ ├─ Client-side state                         │
│ ├─ Sent with every request to same domain    │
│ ├─ Can impact caching and performance        │
│ └─ Privacy concerns (tracking)                │
│                                              │
│ Accept: text/html, application/xhtml+xml    │
│ ├─ Client acceptable content types           │
│ ├─ With quality factors (q=0.9)              │
│ └─ Server negotiates content                 │
│                                              │
│ Authorization: Bearer <token>                │
│ ├─ Authentication credentials                │
│ ├─ Bearer: OAuth 2.0 token                   │
│ ├─ Basic: Base64 encoded user:pass           │
│ └─ Digest: MD5 challenge-response            │
│                                              │
│ Cache-Control: max-age=3600                  │
│ ├─ Caching directives                        │
│ ├─ public: shareable across clients          │
│ ├─ private: only for single user             │
│ ├─ no-cache: revalidate before use           │
│ ├─ no-store: don't store at all              │
│ ├─ max-age: seconds until expiration         │
│ └─ Crucial for web performance               │
│                                              │
│ X-Forwarded-For: 203.0.113.50               │
│ ├─ Client IP address (if behind proxy)       │
│ ├─ Comma-separated list of IPs               │
│ ├─ Added by load balancers, CDNs             │
│ └─ Applications need for rate limiting       │
│                                              │
│ X-Request-ID: f058ebd6-02f7-4d3f-942e-...   │
│ ├─ Request tracing identifier                │
│ ├─ UUID format typical                       │
│ ├─ Enables distributed tracing               │
│ └─ Visible in logs across services           │
│                                              │
│ Accept-Encoding: gzip, deflate               │
│ ├─ Compression algorithms client accepts     │
│ ├─ gzip: standard compression                │
│ ├─ deflate: zlib (less common)               │
│ ├─ br: Brotli (newer, better compression)    │
│ └─ Server responds with Content-Encoding     │
│                                              │
│ \r\n (blank line terminates headers)        │
│                                              │
├──────────────────────────────────────────────┤
│ Body (optional, variable)                   │
│ ├─ Content depends on method & content-type │
│ ├─ POST/PUT: form data or JSON               │
│ ├─ GET: no body                              │
│ └─ Size must match Content-Length            │
│                                              │
└──────────────────────────────────────────────┘

HTTP/2 and HTTP/3 Metadata (no headers text):
┌────────────────────────────────────────────────┐
│ HTTP/2 (HPACK compression)                     │
│                                                │
│ Frame-based (not text-based):                 │
│ ┌──────────────────────────────────────────┐  │
│ │ Frame Type:                              │  │
│ │  - HEADERS: encoded header block         │  │
│ │  - DATA: payload body                    │  │
│ │  - SETTINGS: connection settings         │  │
│ │  - GOAWAY: terminate connection          │  │
│ │  - PING: keepalive                       │  │
│ │  - WINDOW_UPDATE: flow control           │  │
│ └──────────────────────────────────────────┘  │
│                                                │
│ Stream ID: Multiplexes multiple requests      │
│   - Each request in separate stream            │
│   - Streams can arrive out-of-order           │
│   - Server pushes resources proactively       │
│                                                │
│ Flow Control: Per-stream and connection-wide   │
│   - Window-based (credit system)              │
│   - Prevents buffer overflow                   │
│   - Better congestion handling                │
│                                                │
│ Header Compression: HPACK algorithm            │
│   - Dynamic table of previously-seen headers  │
│   - ~30-50% reduction in header size          │
│   - Stateful (must maintain context)          │
│                                                │
└────────────────────────────────────────────────┘

┌────────────────────────────────────────────────┐
│ HTTP/3 (QUIC-based)                           │
│                                                │
│ Built on QUIC:                                │
│  - UDP-based (not TCP)                         │
│  - 0-RTT connection establishment              │
│  - Multiplexing at protocol layer              │
│  - Better congestion control                   │
│  - Connection migration (IP change OK)         │
│                                                │
│ Metadata changes:                             │
│  - Streams over QUIC packets (not TCP packets)│
│  - Encryption mandatory (TLS 1.3)              │
│  - Loss detection different (QUIC level)       │
│  - ACKs only at QUIC layer                     │
│  - Stream IDs: even=client, odd=server-push   │
│                                                │
└────────────────────────────────────────────────┘

Linux HTTP Handling:
- Usually not in kernel (userspace apps)
- Some HTTP parsing in modules (mod_http)
- Metadata mostly in application layer
- sk_buff carries IP/TCP headers only
- Application responsible for HTTP parsing/headers
- Exceptions: 
  - XDP programs can do HTTP parsing
  - BPF can inspect application-level data
  - Kernel HTTP accelerators (HAProxy, nginx in kernel)
```

### 8.2 DNS Metadata

```
DNS Query/Response Structure
─────────────────────────────────────────────────

DNS Header (12 bytes, fixed):
┌─────────────────────────────────────┐
│ Transaction ID           (2 bytes)   │ Bytes 0-1
│ ├─ Client generates random ID        │
│ ├─ Server echoes in response         │
│ ├─ Matches query to response         │
│ └─ 16-bit space (0-65535)             │
│                                     │
├─────────────────────────────────────┤
│ Flags                    (2 bytes)   │ Bytes 2-3
│                                     │
│ QR (Query/Response):                │
│   0 = query (client → server)        │
│   1 = response (server → client)     │
│                                     │
│ Opcode (4 bits):                    │
│   0 = QUERY (standard)              │
│   1 = IQUERY (inverse, obsolete)    │
│   2 = STATUS                        │
│   3 = NOTIFY                        │
│   4 = UPDATE                        │
│   5 = QUERY (same as 0)             │
│                                     │
│ AA (Authoritative Answer):          │
│   1 = response from authoritative ns │
│   0 = non-authoritative             │
│                                     │
│ TC (Truncation):                    │
│   1 = response too large for UDP     │
│   0 = response not truncated        │
│   Indicates TCP retry needed        │
│                                     │
│ RD (Recursion Desired):             │
│   1 = client requests recursion     │
│   0 = client wants referral         │
│                                     │
│ RA (Recursion Available):           │
│   1 = server supports recursion     │
│   0 = server doesn't support        │
│                                     │
│ Z (Reserved):                       │
│   Must be 0                         │
│                                     │
│ AD (Authenticated Data):            │
│   1 = DNSSEC signature validated    │
│   0 = not authenticated             │
│                                     │
│ CD (Checking Disabled):             │
│   1 = don't validate DNSSEC         │
│   0 = validate DNSSEC               │
│                                     │
│ RCODE (Response Code, 4 bits):      │
│   0 = NOERROR (success)             │
│   1 = FORMERR (format error)        │
│   2 = SERVFAIL (server failure)     │
│   3 = NXDOMAIN (no such domain)     │
│   4 = NOTIMP (not implemented)      │
│   5 = REFUSED (query refused)       │
│   6 = YXDOMAIN (name exists)        │
│   7 = YXRRSET (RR set exists)       │
│   8 = NXRRSET (RR set doesn't)      │
│   9 = NOTAUTH (not authoritative)   │
│   10 = NOTZONE (not in zone)        │
│                                     │
├─────────────────────────────────────┤
│ QDCOUNT (Question Section)  (2 B)   │ Bytes 4-5
│ ├─ Number of queries (usually 1)     │
│ ├─ Can be 0 in zone transfer         │
│ └─ Range: 0-65535                   │
│                                     │
├─────────────────────────────────────┤
│ ANCOUNT (Answer Section)    (2 B)   │ Bytes 6-7
│ ├─ Number of answer RRs              │
│ └─ Range: 0-65535                   │
│                                     │
├─────────────────────────────────────┤
│ NSCOUNT (Authority Section) (2 B)   │ Bytes 8-9
│ ├─ Number of authority RRs           │
│ └─ Usually NS records                │
│                                     │
├─────────────────────────────────────┤
│ ARCOUNT (Additional Section)(2 B)   │ Bytes 10-11
│ ├─ Number of additional RRs          │
│ ├─ Glue records, EDNS0               │
│ └─ Range: 0-65535                   │
│                                     │
├─────────────────────────────────────┤
│ Question Section (variable)         │
│ For each question:                  │
│ ┌─────────────────────────────────┐ │
│ │ QNAME (domain name)             │ │
│ │ ├─ Compressed using offset refs │ │
│ │ ├─ e.g., \x03www\x07example\x03com\x00 │
│ │ └─ Null-terminated               │ │
│ │                                 │ │
│ │ QTYPE (2 bytes):                │ │
│ │ ├─ 1 = A (IPv4 address)         │ │
│ │ ├─ 2 = NS (name server)         │ │
│ │ ├─ 5 = CNAME (canonical name)   │ │
│ │ ├─ 6 = SOA (start of authority) │ │
│ │ ├─ 15 = MX (mail exchange)      │ │
│ │ ├─ 16 = TXT (text record)       │ │
│ │ ├─ 28 = AAAA (IPv6 address)     │ │
│ │ ├─ 33 = SRV (service)           │ │
│ │ ├─ 34 = NAPTR (naming auth ptr) │ │
│ │ ├─ 35 = DS (delegation signer)  │ │
│ │ ├─ 43 = DNSKEY (DNSSEC key)     │ │
│ │ ├─ 46 = RRSIG (DNSSEC signature)│ │
│ │ ├─ 47 = NSEC (DNSSEC proof)     │ │
│ │ ├─ 48 = DNSKEY                  │ │
│ │ ├─ 249 = TKEY (transaction key) │ │
│ │ ├─ 250 = TSIG (transaction sig) │ │
│ │ ├─ 255 = ANY (all types)        │ │
│ │ └─ Value is 16-bit              │ │
│ │                                 │ │
│ │ QCLASS (2 bytes):               │ │
│ │ ├─ 1 = IN (Internet)            │ │
│ │ ├─ 3 = CH (Chaosnet, obsolete)  │ │
│ │ ├─ 4 = HS (Hesiod, obsolete)    │ │
│ │ └─ 255 = ANY (all classes)      │ │
│ │                                 │ │
│ └─────────────────────────────────┘ │
│                                     │
├─────────────────────────────────────┤
│ Answer Section (variable)           │
│ For each answer RR:                 │
│ ┌─────────────────────────────────┐ │
│ │ NAME (offset to domain name)    │ │
│ │ TYPE (2 bytes, like QTYPE)      │ │
│ │ CLASS (2 bytes, like QCLASS)    │ │
│ │ TTL (4 bytes, Time To Live)     │ │
│ │ ├─ Seconds until RR expires     │ │
│ │ ├─ Resolver caches for this     │ │
│ │ ├─ 0 = don't cache              │ │
│ │ ├─ 86400 = 24 hours (1 day)     │ │
│ │ └─ Crucial for caching behavior │ │
│ │                                 │ │
│ │ RDLENGTH (2 bytes)              │ │
│ │ └─ Length of RDATA              │ │
│ │                                 │ │
│ │ RDATA (variable)                │ │
│ │ ├─ For A record: 4-byte IPv4    │ │
│ │ ├─ For AAAA: 16-byte IPv6       │ │
│ │ ├─ For CNAME: domain name       │ │
│ │ ├─ For MX: priority + mail host │ │
│ │ ├─ For TXT: arbitrary bytes     │ │
│ │ └─ Format depends on TYPE       │ │
│ │                                 │ │
│ └─────────────────────────────────┘ │
│                                     │
├─────────────────────────────────────┤
│ Authority Section (variable)        │
│ ├─ Typically NS records              │
│ ├─ Same format as Answer RRs        │
│ └─ Tells where to query next        │
│                                     │
├─────────────────────────────────────┤
│ Additional Section (variable)       │
│ ├─ Glue records (A/AAAA for NS)    │
│ ├─ EDNS0 pseudo-records             │
│ └─ Performance optimization         │
│                                     │
└─────────────────────────────────────┘

EDNS0 (Extension Mechanisms for DNS)
─────────────────────────────────────

Pseudo-record in Additional section:
┌──────────────────────────────────────┐
│ NAME: root label (\x00)              │ (no name)
│ TYPE: 41 (OPT, extended record)      │
│ CLASS: UDP Payload Size              │
│ ├─ Max UDP packet size client accepts│
│ ├─ 512 bytes (default)               │
│ ├─ Up to 4096 or higher modern       │
│ └─ Allows large responses            │
│                                      │
│ TTL: Extended RCODE + Flags          │
│ ├─ Upper 8 bits: RCODE (DNSSEC info)│
│ ├─ Lower 24 bits: flags              │
│ │   ├─ DO (DNSSEC OK) bit 15        │
│ │   └─ Other flags for features     │
│                                      │
│ RDLENGTH + RDATA: Options            │
│ ├─ Cookie: DNS transaction security  │
│ ├─ Client Subnet: GeoIP for queries  │
│ ├─ Keepalive: TCP keepalive hint     │
│ ├─ Padding: QNAME padding (privacy) │
│ └─ More options standardized         │
│                                      │
└──────────────────────────────────────┘

DNS Metadata in Linux:
- Handled by libresolv (userspace library)
- Kernel has no DNS parsing (except BPF)
- UDP port 53 or TCP 53 (zone transfers)
- sk_buff carries raw UDP/IP headers
- Application responsible for DNS parsing
- systemd-resolved daemon (modern systems)
- Can use XDP for DNS acceleration
```

---

## 9. Virtualization and Cloud Metadata

### 9.1 Virtual Network Interface (vNIC) Metadata

```
Virtual NIC in Hypervisor/Container
────────────────────────────────────────────────────

vNIC (virtio-net, e1000 emulation, etc.):
┌────────────────────────────────────────────────┐
│ Guest VM                                       │
│ ┌──────────────────────────────────────────┐  │
│ │ Guest OS Linux                           │  │
│ │ ┌──────────────────────────────────────┐ │  │
│ │ │ eth0 virtual device                  │ │  │
│ │ │ (driver: e1000, virtio-net, vmxnet3) │ │  │
│ │ │ ┌────────────────────────────────────┤ │  │
│ │ │ │ skb created on "RX"               │ │  │
│ │ │ │ skb→dev = eth0 (virtual device)   │ │  │
│ │ │ │ skb→mark = 0x0 (no vNIC marking)  │ │  │
│ │ │ │ ...normal processing...           │ │  │
│ │ │ └────────────────────────────────────┤ │  │
│ │ │ ┌────────────────────────────────────┤ │  │
│ │ │ │ Sending packet:                   │ │  │
│ │ │ │ xmit_skb(skb):                    │ │  │
│ │ │ │   - Prepare packet                │ │  │
│ │ │ │   - Give to hypervisor via ring   │ │  │
│ │ │ └────────────────────────────────────┤ │  │
│ │ └──────────────────────────────────────┘ │  │
│ │                                           │  │
│ │ virtio ring (memory shared with host)     │  │
│ │ [virtio descriptor chain pointing to skb] │  │
│ └──────────────────────────────────────────┘  │
│                                               │
│  ─────────────────────────────────────────    │
│  VMCALL / Hypervisor Trap (KVM, Xen)        │
│  ─────────────────────────────────────────    │
│                                               │
│ ┌──────────────────────────────────────────┐  │
│ │ Hypervisor / VMM                         │  │
│ │ ┌──────────────────────────────────────┐ │  │
│ │ │ Virtio backend (qemu-system-x86)    │ │  │
│ │ │ ┌────────────────────────────────────┤ │  │
│ │ │ │ Read guest memory via DMA pointer  │ │  │
│ │ │ │ Extract packet from guest memory   │ │  │
│ │ │ │ Create host-side skb               │ │  │
│ │ │ │ Queue to virtual switch / bridge   │ │  │
│ │ │ │                                     │ │  │
│ │ │ │ Metadata mapping:                  │ │  │
│ │ │ │ ├─ vNIC index → Host bridge port   │ │  │
│ │ │ │ ├─ Guest MAC → mapping entry      │ │  │
│ │ │ │ ├─ Packet headers preserved       │ │  │
│ │ │ │ └─ Add hypervisor-specific marks  │ │  │
│ │ │ │                                     │ │  │
│ │ │ │ Host-side sk_buff created:         │ │  │
│ │ │ │ ├─ skb→dev = br0 / host_eth0      │ │  │
│ │ │ │ ├─ skb→mark = vNIC_ID | VM_ID     │ │  │
│ │ │ │ ├─ skb→skb_iif = vNIC index       │ │  │
│ │ │ │ ├─ Original packet headers intact  │ │  │
│ │ │ │ └─ Processing continues in host   │ │  │
│ │ │ │                                     │ │  │
│ │ │ └────────────────────────────────────┤ │  │
│ │ │ ┌────────────────────────────────────┤ │  │
│ │ │ │ If destined for another vNIC:    │ │  │
│ │ │ │   ├─ Lookup MAC in forwarding DB  │ │  │
│ │ │ │   ├─ Find target vNIC ID          │ │  │
│ │ │ │   ├─ Route through virtual bridge │ │  │
│ │ │ │   ├─ Write back to target VM      │ │  │
│ │ │ │   └─ Raise virtual IRQ in guest   │ │  │
│ │ │ │                                     │ │  │
│ │ │ │ If destined for physical NIC:    │ │  │
│ │ │ │   ├─ Transform TAP filters        │ │  │
│ │ │ │   ├─ Apply egress qdisc           │ │  │
│ │ │ │   └─ DMA to real NIC              │ │  │
│ │ │ │                                     │ │  │
│ │ │ └────────────────────────────────────┤ │  │
│ │ └──────────────────────────────────────┘ │  │
│ │                                           │  │
│ │ Virtual Bridge (br0)                      │  │
│ │ ├─ Forwards packets between vNICs        │  │
│ │ ├─ Same as physical bridge switching      │  │
│ │ ├─ Supports VLAN filtering / 802.1Q      │  │
│ │ ├─ Can mirror to TAP devices             │  │
│ │ └─ QDisc applies per vNIC egress         │  │
│ │                                           │  │
│ └──────────────────────────────────────────┘  │
│                                               │
│ Physical Network Interface (eth0)             │
│ ├─ Real NIC hardware                         │
│ ├─ Packets forwarded by bridge                │
│ ├─ Hardware addresses may be rewritten       │
│ └─ vNIC MAC → Physical NIC MAC mapping       │
│                                               │
└────────────────────────────────────────────────┘

vNIC Metadata Fields in Host sk_buff:
┌──────────────────────────────────────┐
│ skb→dev                              │
│ ├─ Points to virtual bridge device   │
│ └─ Example: br0, vnet0, vxlan0       │
│                                      │
│ skb→mark (custom field)              │
│ ├─ Bit 31-24: Hypervisor ID          │
│ ├─ Bit 23-16: VM ID / Container ID   │
│ ├─ Bit 15-8: vNIC ID                 │
│ └─ Bit 7-0: Traffic type/priority    │
│                                      │
│ skb→skb_iif (input interface index)  │
│ ├─ Virtual NIC index (vnet0, etc)    │
│ └─ Used for reverse path forwarding  │
│                                      │
│ skb→vlan_tci                         │
│ ├─ VLAN ID if vNIC uses VLAN         │
│ └─ Virtual LANs within VM network    │
│                                      │
│ skb→priority                         │
│ ├─ QoS class (0-15)                  │
│ ├─ Set by guest or hypervisor        │
│ └─ Used for qdisc scheduling         │
│                                      │
│ skb→hash / skb→rxhash                │
│ ├─ Flow hash for load balancing      │
│ ├─ Used across vNICs                 │
│ └─ Ensures same flow stays on CPU    │
│                                      │
│ /* Custom metadata (via skb_ext) */  │
│ struct vnic_metadata {               │
│     u32 vm_id;     ← Guest VM ID     │
│     u32 vnic_id;   ← Virtual NIC ID  │
│     u8 vnic_mac[6];← vNIC MAC addr   │
│     u8 vm_uuid[16];← VM UUID         │
│ };                                   │
│                                      │
└──────────────────────────────────────┘
```

### 9.2 VXLAN (Virtual eXtensible LAN) Metadata

```
VXLAN Encapsulation and Metadata
──────────────────────────────────────────────────

VXLAN Frame Format (Tunnel):
┌──────────────────────────────────────────────────┐
│ Outer Ethernet Frame                            │
│ ┌──────────────────────────────────────────────┐│
│ │ Outer Destination MAC  │ Physical NIC MAC    ││
│ │ Outer Source MAC       │ Encapsulating host  ││
│ │ Outer EtherType: 0x0800│ (IPv4) or 0x86DD   ││
│ │                        │ (IPv6)              ││
│ └──────────────────────────────────────────────┘│
│                                                  │
│ Outer IP Header (IPv4 or IPv6)                 │
│ ├─ Source: Physical host IP                    │
│ ├─ Destination: VXLAN tunnel endpoint IP       │
│ └─ Protocol: 17 (UDP)                          │
│                                                  │
│ Outer UDP Header                               │
│ ├─ Source Port: 51234 (ephemeral, for flow)   │
│ │  └─ Often hash-based to spread load          │
│ ├─ Destination Port: 4789 (IANA assigned)      │
│ │  └─ Can be customized per deployment         │
│ └─ Checksum: Usually 0 (optional for UDP)      │
│                                                  │
│ ┌──────────────────────────────────────────────┐│
│ │ VXLAN Header (8 bytes)                       ││
│ │                                              ││
│ │ Flags (1 byte)                               ││
│ │ ├─ Bit 3 (I): VXLAN Network ID valid         ││
│ │ │  └─ 1 = VXLAN ID present (required)        ││
│ │ ├─ Bit 2-0: Reserved (must be 0)            ││
│ │ └─ Bits 7-4: Reserved (must be 0)           ││
│ │                                              ││
│ │ Reserved (3 bytes): must be 0x000000        ││
│ │                                              ││
│ │ VXLAN Network ID (3 bytes)                   ││
│ │ ├─ Identifies virtual network segment        ││
│ │ ├─ 24-bit space: 0 - 16,777,215             ││
│ │ ├─ VNI 0 reserved (management)              ││
│ │ ├─ Supports 16M networks (vs 4K VLANs)      ││
│ │ └─ VLAN ID mapped to VNI by controller      ││
│ │                                              ││
│ └──────────────────────────────────────────────┘│
│                                                  │
│ Inner Ethernet Frame (original VM traffic)     │
│ ├─ Destination MAC: Original dest              │
│ ├─ Source MAC: Original source                 │
│ ├─ EtherType: Original (0x0800, 0x86DD, etc)  │
│ └─ Frame: Unchanged from VM perspective        │
│                                                  │
│ Inner IP/TCP/UDP/Application Data              │
│ └─ Original packet payload                     │
│                                                  │
│ FCS (4 bytes)                                  │
│ └─ Frame check sequence (outer frame only)     │
│                                                  │
└──────────────────────────────────────────────────┘

VXLAN Tunnel Endpoint (VTEP) Processing:

RX Path (Incoming VXLAN packet from network):
┌─────────────────────────────────────────────────┐
│ Physical NIC receives packet                    │
│ Outer IP/UDP headers processed normally         │
│ UDP port 4789 match triggers VXLAN decap       │
│                                                 │
│ VXLAN driver (linux/drivers/net/vxlan.c)       │
│ ├─ Parse VXLAN header                          │
│ ├─ Extract VNI (virtual network ID)            │
│ │  └─ Lookup tunnel device by VNI              │
│ ├─ Remove outer IP/UDP/VXLAN headers           │
│ ├─ Keep inner frame + sk_buff intact           │
│ ├─ Update skb→dev to vxlan interface           │
│ │  └─ Example: skb→dev = vxlan100 (VNI 100)    │
│ ├─ Add metadata to sk_buff                     │
│ │  ├─ skb→vxlan_vni = 100                      │
│ │  ├─ skb→tunnel_metadata[src_vtep_ip]         │
│ │  ├─ skb→dst_metadata[remote_VTEP_port]       │
│ │  └─ skb→mark with VNI info                   │
│ ├─ Pass to virtual bridge / switch             │
│ │  └─ Bridge forwards based on inner MAC       │
│ └─ Eventually reaches destination vNIC/VM      │
│                                                 │
└─────────────────────────────────────────────────┘

TX Path (Outgoing packet to tunnel):
┌─────────────────────────────────────────────────┐
│ VM sends packet via vNIC (eth0)                │
│ Packet arrives at virtual switch (br0)          │
│                                                 │
│ Switch learns mapping:                         │
│ ├─ Inner source MAC → vNIC (local)             │
│ ├─ Looks up inner dest MAC in FDB              │
│ │  └─ FDB = Forwarding Database                │
│ ├─ If unknown or multicast → flood             │
│ │  └─ Sends to all vNICs + VXLAN tunnels       │
│ └─ If known remote → VXLAN tunnel              │
│    └─ Lookup remote VTEP IP from MAC           │
│                                                 │
│ VXLAN TX Processing:                          │
│ ├─ Read packet (inner frame)                   │
│ ├─ Prepend VXLAN header (8 bytes)              │
│ │  ├─ Set VNI from tunnel device               │
│ │  ├─ Set I flag (VNI valid)                   │
│ │  └─ Zero reserved bits                       │
│ ├─ Prepend UDP header                          │
│ │  ├─ Source port: hash(inner_5tuple)          │
│ │  ├─ Dest port: 4789 (or custom)              │
│ │  ├─ Checksum: 0 (usually)                    │
│ │  └─ Spreads flows across ECMP paths          │
│ ├─ Prepend IP header                           │
│ │  ├─ Source IP: Local VTEP IP                 │
│ │  ├─ Dest IP: Remote VTEP IP (from FDB)       │
│ │  ├─ TTL: 64 (configurable)                   │
│ │  └─ Protocol: 17 (UDP)                       │
│ ├─ Prepend outer Ethernet header               │
│ │  ├─ Source MAC: Encapsulating interface MAC  │
│ │  ├─ Dest MAC: Next-hop router MAC (via ARP) │
│ │  ├─ EtherType: 0x0800 (IPv4)                │
│ │  └─ Or 0x86DD (IPv6)                        │
│ ├─ Add sk_buff metadata                        │
│ │  ├─ skb→mark = VNI                           │
│ │  ├─ skb→pkt_type = PACKET_OUTGOING           │
│ │  ├─ skb→priority = from inner QoS            │
│ │  └─ skb→tstamp = current time                │
│ ├─ Send to physical NIC for transmission       │
│ └─ Physical network delivers to remote VTEP    │
│                                                 │
└─────────────────────────────────────────────────┘

VXLAN Learning and MAC Table:

FDB (Forwarding Database) Entry Format:
┌──────────────────────────────────────┐
│ Inner Source MAC: 08:00:27:c4:d2:9b │
│ VXLAN VNI: 100                      │
│ Remote VTEP IP: 192.168.1.200       │
│ Remote VTEP Port: 4789               │
│ Learned timestamp: current jiffies   │
│ Learned on VXLAN tunnel device      │
│                                      │
│ Aging:                               │
│ ├─ Default timeout: 300 seconds      │
│ ├─ Static entries (admin configured) │
│ ├─ Dynamic entries (learned)         │
│ └─ Refresh on each packet from MAC   │
│                                      │
└──────────────────────────────────────┘

Metadata in sk_buff for VXLAN:
┌──────────────────────────────────────────────┐
│ skb→dev = vxlan100 (or similar)              │
│ skb→vxlan_vni = 100                         │
│                                              │
│ skb_dst(skb)→dst_metadata                    │
│ ├─ Contains tunnel source/dest IPs           │
│ ├─ Remote VTEP port                          │
│ ├─ Flags (checksum, inherit DSCP, etc)       │
│ └─ Used for outbound tunnel encap            │
│                                              │
│ skb→mark (custom):                          │
│ ├─ VNI in upper bits                        │
│ ├─ Tenant ID / Segment ID                    │
│ └─ Used for policy routing / qdisc          │
│                                              │
│ skb→flow_dissector_key                       │
│ ├─ Inner 5-tuple (for load balancing)       │
│ └─ Ensures same flow hashes to same path     │
│                                              │
└──────────────────────────────────────────────┘

VXLAN Advantages:
- Overlay network (independent of underlay)
- 24-bit VNI (16M networks vs 4K VLANs)
- Works over existing IP infrastructure
- VM migration without network config change
- Supports multi-tenant networks
- Can span data centers (with proper routing)

VXLAN Limitations:
- Extra encapsulation overhead (~50 bytes)
- CPU cost of encap/decap (improving with HW)
- MTU reduction (tunnel header eats into payload)
- Multicast learning (can be intensive)
- Complexity in debugging (nested headers)
```

### 9.3 Kubernetes Pod-to-Pod Networking Metadata

```
Kubernetes Network Model and Metadata
──────────────────────────────────────────────────

Pod Networking Architecture:
┌────────────────────────────────────────────────┐
│ Kubernetes Cluster                            │
│                                                │
│ ┌──────────────────┐  ┌──────────────────┐   │
│ │ Worker Node 1    │  │ Worker Node 2    │   │
│ │ ┌──────────────┐ │  │ ┌──────────────┐ │   │
│ │ │ Pod A        │ │  │ │ Pod B        │ │   │
│ │ │ IP: 10.1.1.5 │ │  │ │ IP: 10.2.1.5 │ │   │
│ │ │ eth0         │ │  │ │ eth0         │ │   │
│ │ │ (veth pair)  │ │  │ │ (veth pair)  │ │   │
│ │ │              │ │  │ │              │ │   │
│ │ │ Namespace:   │ │  │ │ Namespace:   │ │   │
│ │ │ cni-f74a     │ │  │ │ cni-4bde     │ │   │
│ │ └──────┬───────┘ │  │ └──────┬───────┘ │   │
│ │        │         │  │        │         │   │
│ │   cni0 (bridge)  │  │   cni0 (bridge) │   │
│ │   10.1.0.1/24    │  │   10.2.0.1/24   │   │
│ │        │         │  │        │         │   │
│ │        │ veth-pod│  │        │ veth-pod│   │
│ │        ↓         │  │        ↓         │   │
│ │   eth0 (host)    │  │   eth0 (host)   │   │
│ │   192.168.1.10   │  │   192.168.1.11  │   │
│ │        ↓         │  │        ↓         │   │
│ └────────┼─────────┘  └────────┼────────┘   │
│          │                      │             │
│          └──────────┬───────────┘             │
│                     │                         │
│              Physical Network                 │
│         (e.g., VPC overlay or               │
│          underlay routing)                    │
│                                                │
└────────────────────────────────────────────────┘

Pod Network Metadata:

1. Pod-to-Pod on Same Node:
   ┌──────────────────────────────────────────┐
   │ Pod A (10.1.1.5) sends to Pod B (10.2.1.5) │
   │                                           │
   │ Packet:                                  │
   │ ├─ Src IP: 10.1.1.5 (Pod A)              │
   │ ├─ Dest IP: 10.2.1.5 (Pod B)             │
   │ ├─ Src MAC: AA:BB:CC:DD:EE:11 (eth0)    │
   │ ├─ Dest MAC: AA:BB:CC:DD:EE:22 (eth0)   │
   │                                           │
   │ Processing:                               │
   │ ├─ Pod A veth sends to cni0 (bridge)     │
   │ ├─ Bridge sees src MAC AA:BB:..:EE:11   │
   │ ├─ Bridge does MAC lookup                │
   │ │  └─ Learned which veth port earlier    │
   │ ├─ Bridge forwards out Pod B's veth      │
   │ ├─ veth pair delivers to Pod B's eth0    │
   │ └─ Pod B receives packet                 │
   │                                           │
   │ sk_buff metadata:                        │
   │ ├─ skb→dev = eth0 (Pod B's iface)       │
   │ ├─ skb→pkt_type = PACKET_HOST            │
   │ ├─ skb→mark = 0 (no classification)      │
   │ └─ No tunneling needed (same L2)        │
   │                                           │
   └──────────────────────────────────────────┘

2. Pod-to-Pod on Different Nodes (CNI Plugin):

   a) Calico (IP-in-IP or BGP routing):
   ┌──────────────────────────────────────────┐
   │ Pod A (10.1.1.5, Node 1)                │
   │ Sends to Pod B (10.2.1.5, Node 2)       │
   │                                          │
   │ Original Packet:                        │
   │ ├─ Src IP: 10.1.1.5                     │
   │ ├─ Dest IP: 10.2.1.5                    │
   │ └─ Inner Ethernet: Pod→Pod MAC          │
   │                                          │
   │ If IP-in-IP Encapsulation:              │
   │ ├─ Outer Src IP: Node1 IP (192.168.1.10)│
   │ ├─ Outer Dest IP: Node2 IP (192.168.1.11)│
   │ ├─ Protocol: 4 (IP-in-IP)               │
   │ ├─ Inner Packet: Original 10.1→10.2     │
   │ └─ MTU penalty: ~20 bytes                │
   │                                          │
   │ If BGP Routing (no encap):              │
   │ ├─ Original packet unchanged            │
   │ ├─ Routed via BGP routes                │
   │ ├─ Each node announces Pod CIDR via BGP │
   │ │  └─ Node1: 10.1.0.0/24 via 192.168.1.10 │
   │ │  └─ Node2: 10.2.0.0/24 via 192.168.1.11 │
   │ ├─ Network devices route based on dst  │
   │ └─ No extra headers (better performance)│
   │                                          │
   │ sk_buff metadata (IP-in-IP):            │
   │ ├─ skb→network_header = outer IP        │
   │ ├─ skb→inner_network_header = inner IP │
   │ ├─ skb→mark = VRF / Routing Mark        │
   │ └─ CNI-specific classification data     │
   │                                          │
   └──────────────────────────────────────────┘

   b) Flannel (VXLAN or UDP tunneling):
   ┌──────────────────────────────────────────┐
   │ Uses VXLAN (see VXLAN section)           │
   │                                          │
   │ Pod Subnet → VXLAN VNI Mapping:         │
   │ ├─ 10.1.0.0/24 (Node1) → VXLAN VNI 1   │
   │ ├─ 10.2.0.0/24 (Node2) → VXLAN VNI 2   │
   │ └─ Flannel daemon manages mapping       │
   │                                          │
   │ Encapsulation: standard VXLAN format    │
   │ Remote VTEP IP learned from etcd        │
   │                                          │
   └──────────────────────────────────────────┘

   c) Weave (IP-in-IP with encryption):
   ┌──────────────────────────────────────────┐
   │ IP-in-IP encapsulation:                 │
   │ ├─ Outer Src: Node1 Management IP      │
   │ ├─ Outer Dest: Node2 Management IP     │
   │ └─ Inner: Original Pod packet          │
   │                                          │
   │ Optional IPSec encryption:              │
   │ ├─ Encrypted using pre-shared keys      │
   │ ├─ Each node pair has shared secret      │
   │ ├─ AES encryption applied to tunnel     │
   │ └─ Higher CPU cost but secure           │
   │                                          │
   │ Metadata:                               │
   │ ├─ skb→mark = encryption context       │
   │ ├─ skb→dst_metadata = tunnel info      │
   │ └─ xfrm_state = IPSec security context │
   │                                          │
   └──────────────────────────────────────────┘

3. Service-to-Pod Routing (kube-proxy metadata):
   ┌──────────────────────────────────────────┐
   │ Client Pod sends to Service IP          │
   │ ├─ Dest IP: 10.96.1.1 (Service VIP)     │
   │ └─ Port: 443 (service port)             │
   │                                          │
   │ kube-proxy iptables rules (or IPVS):   │
   │ ├─ Match: Dest IP 10.96.1.1:443         │
   │ ├─ Action: DNAT to Pod IP               │
   │ │  └─ Select one of: [10.1.1.5:8443,   │
   │ │      10.2.1.5:8443, ...]              │
   │ ├─ Metadata: Mark rule with service ID │
   │ │  └─ skb→mark = 0x10001 (service-id)  │
   │ └─ Return: SNAT to original node IP     │
   │                                          │
   │ Tracking in sk_buff:                   │
   │ ├─ skb→mark = Service ID + endpoint   │
   │ ├─ skb_dst(skb)->tclassid = Service  │
   │ └─ nf_conntrack tracks the mapping    │
   │                                          │
   │ Example flow:                          │
   │ ├─ Client pod 10.1.1.10 → 10.96.1.1:443│
   │ ├─ kube-proxy matches rule              │
   │ ├─ DNAT: Dest IP → 10.2.1.5:8443       │
   │ ├─ SNAT: Src IP → Node1 IP              │
   │ ├─ Forward to Pod B                     │
   │ ├─ Pod B reply: 10.2.1.5 → 10.1.1.10   │
   │ ├─ Reverse SNAT: Src → 10.96.1.1       │
   │ └─ Back to client (connection tracking) │
   │                                          │
   └──────────────────────────────────────────┘

Network Policy Metadata:
┌──────────────────────────────────────────────┐
│ NetworkPolicy resource defines traffic rules │
│ CNI plugin enforces via iptables/BPF        │
│                                              │
│ Example Policy:                             │
│ ├─ Allow traffic from Pod labeled           │
│ │  "role=frontend" to "role=backend"        │
│ ├─ Deny all other traffic                   │
│ └─ Enforce on ingress port 8080            │
│                                              │
│ Implementation marks packets:                │
│ ├─ skb→mark = Policy ID / Rule ID           │
│ ├─ iptables rules check mark                │
│ ├─ BPF programs filter at earlier stage     │
│ └─ Decisions cached in conntrack            │
│                                              │
│ sk_buff Metadata:                           │
│ ├─ skb→mark = Policy match result           │
│ ├─ skb→nfmark = Full netfilter mark        │
│ ├─ nf_conntrack→mark = Connection mark     │
│ └─ Labels encoded in mark (namespace, pod) │
│                                              │
└──────────────────────────────────────────────┘

Observability Metadata (Cilium eBPF):
┌──────────────────────────────────────────────┐
│ Cilium uses eBPF instead of iptables        │
│                                              │
│ Enhanced metadata in sk_buff:               │
│ ├─ skb→cb[0-47]: Cilium-specific data      │
│ │  ├─ Pod ID / Endpoint ID                 │
│ │  ├─ Security Identity                    │
│ │  ├─ Service backend ID                   │
│ │  └─ Traffic direction                    │
│                                              │
│ Unified tracing:                            │
│ ├─ Trace ID in packet (via BPF tail call)  │
│ ├─ Visible across all network layers       │
│ ├─ Enables full packet capture              │
│ └─ Enhanced observability (vs iptables)     │
│                                              │
└──────────────────────────────────────────────┘
```

---

## 10. eBPF and Advanced Metadata Handling

### 10.1 eBPF/BPF Metadata Inspection and Modification

```
eBPF Programs and Packet Metadata
────────────────────────────────────────────────

eBPF Hook Points for Network Metadata:

1. XDP (eXpress Data Path) - Earliest (driver level)
┌──────────────────────────────────────────────────┐
│ Triggered before NAPI/softirq processing         │
│                                                  │
│ Access:                                         │
│ ├─ ctx→data: packet start pointer               │
│ ├─ ctx→data_end: packet end pointer             │
│ └─ ctx→(no sk_buff available)                   │
│                                                  │
│ What metadata is available:                     │
│ ├─ Physical packet bytes (Ethernet onwards)     │
│ ├─ NIC-provided hardware metadata               │
│ ├─ Interface index (ctx→ingress_ifindex)        │
│ ├─ Device queue (ctx→rx_queue_index)            │
│ ├─ Namespace ID (optional)                      │
│ └─ Hardware timestamp (if enabled)              │
│                                                  │
│ Metadata modification:                         │
│ ├─ Rewrite packet bytes (packet contents)       │
│ ├─ Set XDP return action codes                  │
│ │  ├─ XDP_DROP: discard packet                 │
│ │  ├─ XDP_PASS: continue to kernel             │
│ │  ├─ XDP_TX: transmit on same interface       │
│ │  ├─ XDP_REDIRECT: forward to another iface   │
│ │  └─ XDP_ABORTED: error, drop packet          │
│ └─ Cannot set sk_buff fields (not created yet) │
│                                                  │
│ Use cases:                                     │
│ ├─ L3/L4 load balancing (before kernel)         │
│ ├─ DDoS mitigation (drop early)                 │
│ ├─ Traffic steering (REDIRECT for AF_XDP)       │
│ └─ Packet sampling (SAMPLE via BTF)             │
│                                                  │
└──────────────────────────────────────────────────┘

2. tc-bpf (traffic classifier / qdisc)
┌──────────────────────────────────────────────────┐
│ Hooked in qdisc, after sk_buff created           │
│                                                  │
│ Ingress path (before routing):                  │
│ ├─ Early classification stage                   │
│ ├─ Access sk_buff metadata                      │
│ ├─ Can inspect all headers                      │
│ └─ Modify skb→mark, skb→priority, etc          │
│                                                  │
│ Egress path (before transmission):              │
│ ├─ After routing decision                       │
│ ├─ Can rewrite headers                          │
│ ├─ Apply per-interface policy                   │
│ └─ Shape traffic via qdisc interaction          │
│                                                  │
│ BPF Context (sk_buff available):               │
│ ├─ skb→data, skb→data_end                       │
│ ├─ skb→len, skb→data_len                        │
│ ├─ skb→dev (ingress/egress interface)           │
│ ├─ skb→protocol, skb→pkt_type                   │
│ ├─ skb→mark, skb→priority                       │
│ ├─ skb→queue_mapping                            │
│ └─ skb→ifindex, skb→skb_iif                     │
│                                                  │
│ Metadata modification:                         │
│ ├─ Rewrite packet headers                       │
│ ├─ Set skb→mark (for routing/policy)            │
│ ├─ Set skb→priority (QoS)                       │
│ ├─ Set skb→queue_mapping (NIC tx queue)         │
│ ├─ Set return actions:                          │
│ │  ├─ TC_ACT_OK (pass forward)                 │
│ │  ├─ TC_ACT_SHOT (drop packet)                │
│ │  ├─ TC_ACT_REDIRECT (send to device)         │
│ │  ├─ TC_ACT_MIRRED (mirror copy)              │
│ │  └─ TC_ACT_STOLEN (consumed)                 │
│ └─ Call kfunc for common operations             │
│                                                  │
│ Use cases:                                     │
│ ├─ Traffic classification (mark packets)        │
│ ├─ Policy enforcement (filter by 5-tuple)       │
│ ├─ QoS application (set priority bits)          │
│ ├─ Header manipulation (rewrite addresses)      │
│ ├─ Load balancing (distribute flows)            │
│ └─ Accounting/metering (sample packets)         │
│                                                  │
└──────────────────────────────────────────────────┘

3. Socket-level BPF (kernel >= 5.0)
┌──────────────────────────────────────────────────┐
│ Attached to sockets (struct sock)                │
│                                                  │
│ BPF_PROG_TYPE_SK_SKB (sockmap/devmap):          │
│ ├─ Intercepts socket operations                 │
│ ├─ Access to sock structure                     │
│ ├─ Can redirect packets (accelerated path)      │
│ ├─ No sk_buff (direct data)                     │
│ └─ Used for userspace packet steering           │
│                                                  │
│ BPF_PROG_TYPE_SK_REUSEPORT:                    │
│ ├─ Multi-program socket load balancing          │
│ ├─ Receive side socket selection                │
│ ├─ Return socket index for packet               │
│ ├─ Access sock structure metadata               │
│ └─ Useful for UDP load balancing                │
│                                                  │
└──────────────────────────────────────────────────┘

4. Netfilter (nf_tables, nf_conntrack)
┌──────────────────────────────────────────────────┐
│ Hooked into netfilter chain                      │
│                                                  │
│ BPF_PROG_TYPE_NETFILTER:                       │
│ ├─ Attached to nf_tables hooks                  │
│ ├─ Pre/post routing, forward, local input/out   │
│ ├─ Full sk_buff access                          │
│ ├─ Access to nf_conn (connection tracking)      │
│ ├─ Can modify marks, nfct state                 │
│ └─ Return verdict (ACCEPT, DROP, QUEUE, etc)    │
│                                                  │
│ Metadata available:                             │
│ ├─ Full sk_buff structure                       │
│ ├─ skb→nfmark, skb→nfct (conntrack)             │
│ ├─ Parsing headers (L3/L4)                      │
│ ├─ Connection state tracking info               │
│ └─ Netfilter match/target state                 │
│                                                  │
│ Modification capabilities:                      │
│ ├─ NAT (SNAT/DNAT address rewriting)            │
│ ├─ Packet rewriting (any header)                │
│ ├─ Connection state manipulation                │
│ ├─ Mark setting for downstream rules            │
│ └─ Return verdicts (NF_ACCEPT, NF_DROP, etc)    │
│                                                  │
│ Use cases:                                     │
│ ├─ Stateful firewall rules                      │
│ ├─ NAT and port forwarding                      │
│ ├─ DDoS/load balancing with state               │
│ ├─ Connection-aware filtering                   │
│ └─ Dynamic rule evaluation (vs static tables)   │
│                                                  │
└──────────────────────────────────────────────────┘

5. LSM (Linux Security Module) - BPF LSM
┌──────────────────────────────────────────────────┐
│ Security policy enforcement                      │
│                                                  │
│ BPF_PROG_TYPE_LSM:                             │
│ ├─ Attached to LSM hooks (not network-specific) │
│ ├─ Can hook socket operations                   │
│ ├─ Access to task_struct, socket, sk_buff      │
│ ├─ Fine-grained capability checks               │
│ └─ RBAC-like policy enforcement                 │
│                                                  │
│ Network-related hooks:                         │
│ ├─ socket_create (new socket)                   │
│ ├─ socket_bind (binding to port)                │
│ ├─ socket_connect (outbound connection)         │
│ ├─ socket_listen (server listening)             │
│ ├─ socket_sendmsg (sending data)                │
│ ├─ socket_recvmsg (receiving data)              │
│ └─ Return allow (0) or deny (error)             │
│                                                  │
└──────────────────────────────────────────────────┘

Example BPF Programs for Metadata:

1. tc-bpf: Mark by IP 5-tuple
┌──────────────────────────────────────────────────┐
│ #include <uapi/linux/bpf.h>                      │
│ #include <linux/in.h>                            │
│ #include <linux/ip.h>                            │
│ #include <linux/udp.h>                           │
│                                                  │
│ SEC("classifier")                               │
│ int mark_flow(struct __sk_buff *skb)            │
│ {                                                │
│     void *data_end = (void *)(long)skb→data_end;│
│     void *data = (void *)(long)skb→data;        │
│     struct ethhdr *eth = data;                   │
│     struct iphdr *ip;                            │
│     struct udphdr *udp;                          │
│                                                  │
│     if ((void *)(eth + 1) > data_end)           │
│         return TC_ACT_OK;  /* Packet too small */ │
│                                                  │
│     if (eth→h_proto != htons(ETH_P_IP))        │
│         return TC_ACT_OK;  /* Not IPv4 */       │
│                                                  │
│     ip = (void *)(eth + 1);                     │
│     if ((void *)(ip + 1) > data_end)            │
│         return TC_ACT_OK;                        │
│                                                  │
│     if (ip→protocol != IPPROTO_UDP)              │
│         return TC_ACT_OK;  /* Not UDP */        │
│                                                  │
│     udp = (void *)(ip + 1);                     │
│     if ((void *)(udp + 1) > data_end)           │
│         return TC_ACT_OK;                        │
│                                                  │
│     /* Compute hash from 5-tuple */             │
│     __u32 sip = ip→saddr;                       │
│     __u32 dip = ip→daddr;                       │
│     __u32 sport = udp→source;                   │
│     __u32 dport = udp→dest;                     │
│                                                  │
│     /* Create mark: combine hash of 5-tuple */  │
│     __u32 mark = ((sip ^ dip) * 2654435761UL) ┌│
│                  ^ ((sport ^ dport) * 101);     │
│                                                  │
│     skb→mark = mark;  /* Set classification */  │
│     return TC_ACT_OK;                           │
│ }                                                │
│                                                  │
│ Loading:                                        │
│ $ clang -O2 -target bpf -c mark_flow.c -o mark│
│ $ tc qdisc add dev eth0 root handle 1: prio   │
│ $ tc filter add dev eth0 parent 1: protocol   │
│    ip prio 1 bpf obj mark_flow.o sec classifier│
│                                                  │
└──────────────────────────────────────────────────┘

2. XDP: DDoS filter (drop by rate)
┌──────────────────────────────────────────────────┐
│ #include <linux/bpf.h>                           │
│ #include <linux/ip.h>                            │
│ #include <linux/in.h>                            │
│ #include <bpf/bpf_helpers.h>                    │
│                                                  │
│ struct {                                        │
│     __uint(type, BPF_MAP_TYPE_ARRAY);           │
│     __type(key, __u32);                         │
│     __type(value, __u64);                       │
│     __uint(max_entries, 256);                   │
│ } pkt_counts SEC(".maps");                      │
│                                                  │
│ SEC("xdp")                                      │
│ int ddos_filter(struct xdp_md *ctx)             │
│ {                                                │
│     void *data_end = (void *)(long)ctx→data_end;│
│     void *data = (void *)(long)ctx→data;        │
│     struct ethhdr *eth = data;                   │
│     struct iphdr *ip = (void *)(eth + 1);       │
│                                                  │
│     if ((void *)(ip + 1) > data_end)            │
│         return XDP_PASS;                        │
│                                                  │
│     /* Get last octet of destination IP */     │
│     __u32 bucket = ip→daddr & 0xFF;             │
│                                                  │
│     /* Increment counter for this bucket */    │
│     __u64 *count = bpf_map_lookup_elem(&pkt..);│
│     if (count) {                                │
│         __sync_fetch_and_add(count, 1);         │
│         if (*count > 100000) {  /* Per sec */  │
│             return XDP_DROP;  /* Block IP */   │
│         }                                       │
│     }                                           │
│                                                  │
│     return XDP_PASS;                            │
│ }                                                │
│                                                  │
└──────────────────────────────────────────────────┘

BPF Maps for Metadata Storage:
┌──────────────────────────────────────────────────┐
│ BPF programs can store metadata in maps          │
│                                                  │
│ Common map types:                               │
│                                                  │
│ BPF_MAP_TYPE_ARRAY                             │
│ ├─ Fast fixed-size lookup                      │
│ ├─ No hash cost                                 │
│ ├─ Key = 0-indexed (0 to max_entries-1)        │
│ └─ Used for counters, stats                    │
│                                                  │
│ BPF_MAP_TYPE_HASH_MAP                          │
│ ├─ Generic key-value storage                   │
│ ├─ Hash lookup (O(1) average)                  │
│ ├─ Used for flow tables, connection data      │
│ └─ 5-tuple as key → metadata as value         │
│                                                  │
│ BPF_MAP_TYPE_LRU_HASH                          │
│ ├─ Hash map with LRU eviction                  │
│ ├─ Automatic size management                   │
│ ├─ Perfect for per-flow data                   │
│ └─ Evicts least-recently-used entries          │
│                                                  │
│ BPF_MAP_TYPE_RINGBUF                           │
│ ├─ Ring buffer for event output                │
│ ├─ Zero-copy pushes to userspace               │
│ ├─ Used for exporting metadata                 │
│ └─ Events = packets, traffic, metadata         │
│                                                  │
│ Example: Flow metadata storage
│ ┌──────────────────────────────────────────┐  │
│ │ struct flow_key {                         │  │
│ │     __u32 sip, dip;                      │  │
│ │     __u16 sport, dport;                  │  │
│ │     __u8 proto;                          │  │
│ │ };                                        │  │
│ │                                          │  │
│ │ struct flow_data {                       │  │
│ │     __u64 packets;                       │  │
│ │     __u64 bytes;                         │  │
│ │     __u32 last_seen;                     │  │
│ │     __u8 state;  ← TCP state             │  │
│ │ };                                        │  │
│ │                                          │  │
│ │ BPF_MAP_TYPE_LRU_HASH flow_table = {    │  │
│ │     .key_size = sizeof(struct flow_key), │  │
│ │     .value_size = sizeof(struct flow... │  │
│ │ };                                        │  │
│ │                                          │  │
│ │ On every packet:                        │  │
│ │ ├─ Extract 5-tuple                      │  │
│ │ ├─ Lookup in flow_table                 │  │
│ │ ├─ If exists: update counters, state    │  │
│ │ ├─ If new: create entry                 │  │
│ │ └─ Use for rate limiting, accounting    │  │
│ │                                          │  │
│ └──────────────────────────────────────────┘  │
│                                                  │
└──────────────────────────────────────────────────┘
```

---

## 11. C Implementation Examples

### 11.1 sk_buff Packet Walking in C

```c
/*
 * Kernel module: Walk through packet headers in sk_buff
 * Demonstrates metadata extraction at each layer
 */

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/skbuff.h>
#include <linux/netdevice.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/etherdevice.h>
#include <net/ip.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Example");
MODULE_DESCRIPTION("sk_buff packet walking demonstration");

/* Function to safely get a pointer to data in sk_buff */
static void *get_pointer_safe(struct sk_buff *skb, size_t offset, size_t size)
{
    /*
     * Verify we're not reading past the end of the packet
     * This prevents buffer overflows in kernel code
     */
    if (offset + size > skb->len) {
        printk(KERN_WARNING "Attempted read past packet boundary\n");
        return NULL;
    }
    return skb->data + offset;
}

/* Walk through packet layers and print metadata */
static void analyze_packet(struct sk_buff *skb)
{
    struct ethhdr *eth;
    struct iphdr *ip_hdr;
    struct ipv6hdr *ipv6_hdr;
    struct tcphdr *tcp_hdr;
    struct udphdr *udp_hdr;
    
    unsigned int offset = 0;
    __u8 ip_protocol;
    
    printk(KERN_INFO "\n=== Packet Analysis (skb=%p) ===\n", skb);
    
    /* ===== LAYER 2: Ethernet ===== */
    eth = (struct ethhdr *)skb->data;
    
    if (!eth || (skb->data + sizeof(*eth) > skb->data + skb->len)) {
        printk(KERN_ERR "Packet too small for Ethernet header\n");
        return;
    }
    
    printk(KERN_INFO "=== Layer 2: Ethernet ===\n");
    printk(KERN_INFO "  Src MAC: %pM\n", eth->h_source);
    printk(KERN_INFO "  Dst MAC: %pM\n", eth->h_dest);
    printk(KERN_INFO "  EtherType: 0x%04x (%s)\n", 
           ntohs(eth->h_proto),
           (ntohs(eth->h_proto) == ETH_P_IP) ? "IPv4" :
           (ntohs(eth->h_proto) == ETH_P_IPV6) ? "IPv6" : 
           (ntohs(eth->h_proto) == ETH_P_ARP) ? "ARP" : "Other");
    
    /* sk_buff metadata for L2 */
    printk(KERN_INFO "  skb→dev: %s\n", skb->dev ? skb->dev->name : "NULL");
    printk(KERN_INFO "  skb→pkt_type: %u (%s)\n", 
           skb->pkt_type,
           (skb->pkt_type == PACKET_HOST) ? "PACKET_HOST" :
           (skb->pkt_type == PACKET_BROADCAST) ? "PACKET_BROADCAST" :
           (skb->pkt_type == PACKET_MULTICAST) ? "PACKET_MULTICAST" : "Other");
    
    offset = sizeof(*eth);
    
    /* Handle VLAN tagging */
    if (ntohs(eth->h_proto) == ETH_P_8021Q) {
        printk(KERN_INFO "  [VLAN tag present]\n");
        printk(KERN_INFO "  VLAN ID: %d\n", skb->vlan_tci & VLAN_VID_MASK);
        printk(KERN_INFO "  VLAN Priority: %d\n", 
               (skb->vlan_tci >> VLAN_PRIO_SHIFT) & 0x7);
        offset += 4;  /* Skip VLAN tag, then re-read EtherType */
    }
    
    /* ===== LAYER 3: IP ===== */
    ip_hdr = (struct iphdr *)(skb->data + offset);
    ipv6_hdr = (struct ipv6hdr *)(skb->data + offset);
    
    /* Check IP version */
    if (ip_hdr->version == 4) {
        /* IPv4 */
        if ((skb->data + offset + sizeof(*ip_hdr)) > skb->data + skb->len) {
            printk(KERN_ERR "Packet too small for IPv4 header\n");
            return;
        }
        
        printk(KERN_INFO "\n=== Layer 3: IPv4 ===\n");
        printk(KERN_INFO "  Src IP: %pI4\n", &ip_hdr->saddr);
        printk(KERN_INFO "  Dst IP: %pI4\n", &ip_hdr->daddr);
        printk(KERN_INFO "  TTL: %u\n", ip_hdr->ttl);
        printk(KERN_INFO "  Protocol: %u (%s)\n", 
               ip_hdr->protocol,
               (ip_hdr->protocol == IPPROTO_TCP) ? "TCP" :
               (ip_hdr->protocol == IPPROTO_UDP) ? "UDP" :
               (ip_hdr->protocol == IPPROTO_ICMP) ? "ICMP" : "Other");
        printk(KERN_INFO "  DSCP: %u (QoS class)\n", 
               (ip_hdr->tos >> 2) & 0x3F);
        printk(KERN_INFO "  ECN: %u\n", ip_hdr->tos & 0x3);
        
        /* Fragmentation info */
        if (ip_hdr->frag_off & htons(IP_MF)) {
            printk(KERN_INFO "  [More Fragments]\n");
        }
        if (ip_hdr->frag_off & htons(IP_DF)) {
            printk(KERN_INFO "  [Don't Fragment]\n");
        }
        printk(KERN_INFO "  Fragment Offset: %u\n", 
               ntohs(ip_hdr->frag_off) & IP_OFFSET);
        
        ip_protocol = ip_hdr->protocol;
        offset += (ip_hdr->ihl * 4);  /* IHL in 32-bit words */
        
        /* sk_buff metadata for L3 */
        printk(KERN_INFO "  skb→priority (mapped from DSCP): %u\n", 
               skb->priority);
        printk(KERN_INFO "  skb→mark: 0x%08x\n", skb->mark);
        
    } else if (ipv6_hdr->version == 6) {
        /* IPv6 */
        if ((skb->data + offset + sizeof(*ipv6_hdr)) > skb->data + skb->len) {
            printk(KERN_ERR "Packet too small for IPv6 header\n");
            return;
        }
        
        printk(KERN_INFO "\n=== Layer 3: IPv6 ===\n");
        printk(KERN_INFO "  Src IP: %pI6\n", &ipv6_hdr->saddr);
        printk(KERN_INFO "  Dst IP: %pI6\n", &ipv6_hdr->daddr);
        printk(KERN_INFO "  Hop Limit: %u\n", ipv6_hdr->hop_limit);
        printk(KERN_INFO "  Next Header: %u\n", ipv6_hdr->nexthdr);
        printk(KERN_INFO "  Traffic Class (DSCP): %u\n", 
               (ipv6_hdr->priority << 4) | (ipv6_hdr->flow_lbl[0] >> 4));
        printk(KERN_INFO "  Flow Label: 0x%05x\n", 
               ntohl(*((__u32 *)ipv6_hdr)) & 0xFFFFF);
        
        ip_protocol = ipv6_hdr->nexthdr;
        offset += sizeof(*ipv6_hdr);
        
    } else {
        printk(KERN_WARNING "Unknown IP version: %u\n", ip_hdr->version);
        return;
    }
    
    /* ===== LAYER 4: Transport ===== */
    switch (ip_protocol) {
        case IPPROTO_TCP:
            tcp_hdr = (struct tcphdr *)(skb->data + offset);
            if ((skb->data + offset + sizeof(*tcp_hdr)) > skb->data + skb->len) {
                printk(KERN_ERR "Packet too small for TCP header\n");
                return;
            }
            
            printk(KERN_INFO "\n=== Layer 4: TCP ===\n");
            printk(KERN_INFO "  Src Port: %u\n", ntohs(tcp_hdr->source));
            printk(KERN_INFO "  Dst Port: %u\n", ntohs(tcp_hdr->dest));
            printk(KERN_INFO "  Sequence: %u\n", ntohl(tcp_hdr->seq));
            printk(KERN_INFO "  Acknowledgment: %u\n", ntohl(tcp_hdr->ack_seq));
            printk(KERN_INFO "  Window Size: %u\n", ntohs(tcp_hdr->window));
            
            /* TCP flags */
            printk(KERN_INFO "  Flags: %s%s%s%s%s%s (0x%02x)\n",
                   tcp_hdr->syn ? "SYN " : "",
                   tcp_hdr->ack ? "ACK " : "",
                   tcp_hdr->fin ? "FIN " : "",
                   tcp_hdr->rst ? "RST " : "",
                   tcp_hdr->psh ? "PSH " : "",
                   tcp_hdr->urg ? "URG " : "",
                   tcp_hdr->th_flags);
            break;
        case IPPROTO_UDP:
            udp_hdr = (struct udphdr *)(skb->data + offset);
            if ((skb->data + offset + sizeof(*udp_hdr)) > skb->data + skb->len) {
                printk(KERN_ERR "Packet too small for UDP header\n");
                return;
            }
            printk(KERN_INFO "\n=== Layer 4: UDP ===\n");
            printk(KERN_INFO "  Src Port: %u\n", ntohs(udp_hdr->source));
            printk(KERN_INFO "  Dst Port: %u\n", ntohs(udp_hdr->dest));
            printk(KERN_INFO "  Length: %u\n", ntohs(udp_hdr->len));
            break;
        case IPPROTO_ICMP:
            printk(KERN_INFO "\n=== Layer 4: ICMP ===\n");
            /* ICMP header parsing would go here */
            break;
        default:
            printk(KERN_INFO "\n=== Layer 4: Other Protocol (%u) ===\n", ip_protocol);
            break;
    }
}
```