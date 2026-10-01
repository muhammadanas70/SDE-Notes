# NAPI — The Linux Kernel Network Receive Interface: A Complete In-Depth Guide

> **Scope**: Linux Kernel NAPI subsystem — architecture, internals, driver implementation, Rust abstractions, cloud networking, XDP, busy polling, threaded NAPI, performance tuning, and the mental models that make you reason about packet I/O efficiently.

---

## Table of Contents

1. [Introduction & Motivation](#1-introduction--motivation)
2. [Historical Background: Before NAPI](#2-historical-background-before-napi)
3. [The Interrupt Storm Problem — Deep Analysis](#3-the-interrupt-storm-problem--deep-analysis)
4. [NAPI Architecture — How It Solves the Problem](#4-napi-architecture--how-it-solves-the-problem)
5. [Linux Networking Stack — Where NAPI Lives](#5-linux-networking-stack--where-napi-lives)
6. [Core Data Structures — Complete Walkthrough](#6-core-data-structures--complete-walkthrough)
7. [NAPI State Machine](#7-napi-state-machine)
8. [NAPI Lifecycle Functions — Every API Explained](#8-napi-lifecycle-functions--every-api-explained)
9. [Softirq Integration — net_rx_action Deep Dive](#9-softirq-integration--net_rx_action-deep-dive)
10. [Budget and Weight System](#10-budget-and-weight-system)
11. [GRO — Generic Receive Offload](#11-gro--generic-receive-offload)
12. [Multi-Queue NAPI and Per-CPU Architecture](#12-multi-queue-napi-and-per-cpu-architecture)
13. [RSS, RPS, and RFS — Packet Steering](#13-rss-rps-and-rfs--packet-steering)
14. [Complete C Driver Implementation with NAPI](#14-complete-c-driver-implementation-with-napi)
15. [Rust for Linux — NAPI Abstractions and Driver](#15-rust-for-linux--napi-abstractions-and-driver)
16. [XDP Integration with NAPI](#16-xdp-integration-with-napi)
17. [Busy Polling — SO_BUSY_POLL and Low-Latency Path](#17-busy-polling--so_busy_poll-and-low-latency-path)
18. [Threaded NAPI Mode](#18-threaded-napi-mode)
19. [AF_XDP and Zero-Copy with NAPI](#19-af_xdp-and-zero-copy-with-napi)
20. [Performance Tuning — Complete Reference](#20-performance-tuning--complete-reference)
21. [Monitoring, Debugging, and Observability](#21-monitoring-debugging-and-observability)
22. [Cloud Networking and NAPI](#22-cloud-networking-and-napi)
23. [Kernel Source Navigation](#23-kernel-source-navigation)
24. [Mental Models — Thinking Efficiently About NAPI](#24-mental-models--thinking-efficiently-about-napi)

---

## 1. Introduction & Motivation

NAPI (New API) is the Linux kernel's solution to a fundamental problem in high-speed networking: how do you receive millions of packets per second without letting interrupt handling consume the entire CPU?

The name is historical — when it was introduced around the Linux 2.5/2.6 era (roughly 2002), it was the "new" way to write receive-side network drivers. Today, all modern network drivers use NAPI, and the name just sticks.

**What NAPI does in one sentence**: Instead of generating a hardware interrupt for every single received packet, NAPI switches the NIC into a polling mode where the kernel actively drains the receive queue in batches, then re-enables interrupts when the queue is empty.

**Why this is critical**: At 10 Gbps with minimum-sized (64-byte) Ethernet frames:

```
10,000,000,000 bits/second
÷ (64 bytes + 20 bytes overhead) × 8 bits/byte
= ~14,880,952 packets/second
```

Without NAPI, that would be ~14.8 million hardware interrupts per second per NIC. Each interrupt:
- Saves CPU register state (context switch overhead)
- Invalidates speculative execution
- Flushes portions of instruction/data TLB
- Wakes the interrupt handler
- Returns to interrupted code

At 14.8 million/second, interrupt overhead alone would saturate a modern CPU core. NAPI eliminates most of those interrupts and replaces them with a single polling loop run in softirq context.

At 100 Gbps (modern data center NICs), the numbers are even more extreme:
- ~148 million packets/second per port
- Multiple ports per NIC
- NAPI (combined with multi-queue, RSS, and per-CPU processing) is the only reason these rates are achievable in the Linux kernel.

---

## 2. Historical Background: Before NAPI

### The Old `netif_rx()` Path (pre-2.5 kernel)

Before NAPI, the receive path worked as follows:

1. **NIC raises hardware interrupt** for every received frame
2. **IRQ handler** calls `netif_rx(skb)` 
3. `netif_rx()` enqueues the `sk_buff` onto a per-CPU `input_pkt_queue`
4. Raises `NET_RX_SOFTIRQ` 
5. Softirq handler drains the queue and passes SKBs up the stack

```
  [NIC Hardware]
       |
       | (interrupt per packet)
       v
  [IRQ Handler]
       |
       | netif_rx(skb)
       v
  [per-CPU input_pkt_queue] ← queue fills up here
       |
       | softirq drain
       v
  [Protocol Stack: IP → TCP/UDP]
```

### Why This Broke at High Speeds

**Problem 1 — Interrupt rate explosion**: Each packet = one interrupt. The CPU spent more time managing interrupt overhead than processing packets.

**Problem 2 — Queue drop policy**: `input_pkt_queue` had a fixed maximum length (`netdev_max_backlog`, default 1000). When the queue filled:
- New packets were **dropped silently**
- The counter `softnet_data.dropped` incremented
- The NIC kept interrupting even though the kernel was dropping everything

**Problem 3 — No backpressure to the hardware**: The kernel had no mechanism to tell the NIC "stop sending interrupts until we've caught up." The hardware queue would also overflow.

**Problem 4 — Per-CPU queue was a bottleneck**: Single queue per CPU meant receive processing was serialized, even as multi-queue NICs emerged.

**Problem 5 — No hardware queue draining**: The `netif_rx()` model processed one packet per interrupt. The NIC's hardware FIFO queue was never drained efficiently.

Jamal Hadi Salim proposed NAPI in 2002 to address all of these by fundamentally changing the model: **disable interrupts, poll the hardware queue directly**.

---

## 3. The Interrupt Storm Problem — Deep Analysis

Understanding the interrupt storm is critical to deeply grasping why NAPI's design choices are what they are.

### What Actually Happens in an Interrupt Storm

```
Timeline without NAPI at 10Gbps:

t=0      NIC receives pkt1 → IRQ fires → handler runs → 1μs
t=0.067  NIC receives pkt2 → IRQ fires → handler runs → 1μs
t=0.134  NIC receives pkt3 → IRQ fires → handler runs → 1μs
... (14.88 million times per second)

CPU utilization breakdown:
  - IRQ entry/exit overhead:          ~60%
  - Actual packet processing:         ~10%
  - Other work (TCP ACKs, apps):      ~30%   ← starvation
```

### The Livelock Scenario

The most pathological form of interrupt storm is **receive livelock**:

1. Attacker sends high-rate UDP flood to server
2. Server CPU is 100% in interrupt handlers processing the flood
3. CPU never returns to user space
4. Legitimate requests pile up unserviced
5. Server appears completely unresponsive — but it's not crashed, it's just stuck in IRQs

This is a **denial of service via interrupt saturation**, and it was a real attack vector before NAPI. Even non-malicious high traffic could trigger it.

### Cache Effects

Beyond raw interrupt count, interrupts destroy CPU caches:

```
Without NAPI:
  - Each IRQ: saves ~150 bytes of register state on stack
  - Handler runs: touches packet descriptor rings, DMA areas
  - Returns: restores register state
  - Re-enters application code: cache is partially cold
  - L1 cache miss rate: high
  - Branch predictor: disrupted by indirect jumps in IRQ vector

With NAPI:
  - One IRQ to initiate
  - Softirq runs: stays "warm" in cache (same data, same code)
  - Poll loop: tight, cache-friendly, branch predictor learns pattern
  - L1 cache miss rate: dramatically lower
```

### NUMA and Interrupt Affinity

On NUMA systems (multiple CPU sockets), the interrupt storm problem is compounded:

```
      Socket 0                    Socket 1
   ┌──────────────┐           ┌──────────────┐
   │  CPU 0-15    │           │  CPU 16-31   │
   │  L3 Cache    │           │  L3 Cache    │
   │  RAM Bank 0  │           │  RAM Bank 1  │
   └──────┬───────┘           └──────────────┘
          │
          │ QPI/UPI link (slow)
          │
   ┌──────┴───────┐
   │  NIC PCIe    │
   │  DMA → RAM0  │
   └──────────────┘

If NIC DMA goes to RAM Bank 0, but IRQ is handled by CPU 16-31 (Socket 1):
  - Every packet access crosses QPI link
  - QPI latency: ~100ns vs local RAM ~50ns
  - At 14.8M pps: 14.8M × 50ns extra = 0.74 seconds of QPI latency/second
  - Effectively halves throughput for no reason
```

NAPI's per-CPU design allows pinning: IRQ → CPU → NAPI poll → all on the same NUMA node, accessing NIC DMA memory through the local memory bus.

---

## 4. NAPI Architecture — How It Solves the Problem

### The Core NAPI Algorithm

```
NAPI Receive Algorithm:

  [Hardware Interrupt Fires]
         │
         ▼
  ┌─────────────────────────────────┐
  │  IRQ Handler                    │
  │  1. ACK interrupt               │
  │  2. DISABLE NIC interrupts      │  ← Key: stop further IRQs
  │  3. Schedule NAPI (softirq)     │
  │  4. Return from IRQ             │
  └─────────────────────────────────┘
         │
         │ (CPU returns to interrupted work briefly,
         │  then softirq fires)
         ▼
  ┌─────────────────────────────────┐
  │  Softirq: NET_RX_SOFTIRQ        │
  │  (net_rx_action)                │
  │                                 │
  │  while (budget > 0) {           │
  │    pkt = poll_hw_rx_ring()      │  ← Direct hardware read
  │    if (!pkt) break              │  ← Queue empty: stop
  │    process_packet(pkt)          │
  │    budget--                     │
  │  }                              │
  │                                 │
  │  if (queue_empty) {             │
  │    ENABLE NIC interrupts        │  ← Re-arm: next pkt = new IRQ
  │    napi_complete()              │
  │  } else {                       │
  │    reschedule softirq           │  ← Budget exhausted: yield
  │  }                              │
  └─────────────────────────────────┘
```

### State Transitions in Plain Language

```
NAPI States:
                                    ┌──────────────── ──┐
                                    │    DISABLED       │
                                    │  (not available)  │
                                    └────────┬──────────┘
                                             │ napi_enable()
                                             ▼
                                    ┌─────────────── ───┐
                          ┌────────►│    IDLE           │◄────────────┐
                          │         │  (interrupts on)  │             │
                          │         └────────┬──────────┘             │
                          │                  │                        │
                          │                  │ IRQ fires,             │
                          │                  │ napi_schedule()        │
                          │                  ▼                        │
                          │         ┌──────────────────┐              │
                          │         │    SCHEDULED     │              │
                          │         │  (in poll_list)  │              │
                          │         └────────┬─────────┘              │
                          │                  │                        │
                          │                  │softirq runs napi_poll()│
                          │                  ▼                        │
                          │         ┌───────────────────┐             │
                          │         │    POLLING        │             │
                          │         │  (consuming pkts) │             │
                          │         └────────┬──────────┘             │
                          │                  │                        │
                          │         ┌────────┴──────────┐             │
                          │         │                   │             │
                          │         │ budget            │ queue       │
                          │         │ exhausted         │ empty       │
                          │         ▼                   ▼             │
                          │  ┌────────────┐    ┌────────────────┐     │
                          │  │ RESCHEDULED│    │  COMPLETE      │     │
                          │  │(re-queued) │    │ re-enable IRQs │─────┘
                          │  └──────┬─────┘    └────────────────┘
                          │         │ next softirq
                          └─────────┘
```

### The Key Insight: Interrupt Coalescing Without Hardware Support

Traditional interrupt coalescing requires NIC firmware support (interrupt moderation timers). NAPI achieves software-based coalescing:

- **First packet**: Hardware interrupt fires → NAPI scheduled
- **All subsequent packets** while polling: Processed by poll loop, **no interrupts** 
- **Interrupt re-enabled** only when queue is truly empty

This is elegant because it adapts automatically:
- **Low traffic**: One packet → one IRQ → poll → queue empty → re-enable. Low latency.
- **High traffic**: One IRQ → poll entire burst → queue never empty → reschedule → minimal IRQs.
- **Overload**: Budget exhausted → yield CPU to other work → come back. Fair scheduling.

---

## 5. Linux Networking Stack — Where NAPI Lives

Understanding NAPI requires seeing the full picture of where it sits in the stack.

```
  User Space
  ─────────────────────────────────────────────────────────────
  Application (read/recv/recvmsg)
       │
       │ syscall
       ▼
  ─────────────────────────────────────────────────────────────
  Kernel Space — Socket Layer (net/socket.c)
       │
       ▼
  Transport Layer (TCP: net/ipv4/tcp.c / UDP: net/ipv4/udp.c)
       │
       ▼
  Network Layer (IP: net/ipv4/ip_input.c)
       │
       ▼
  ─────────────────────────────────────────────────────────────
  NETIF_RECEIVE_SKB LAYER
  (net/core/dev.c: __netif_receive_skb)
       │
       │ ← Packet enters here from NAPI poll
       │
       ├─► netfilter hooks (iptables/nftables)
       ├─► traffic control (TC ingress)
       ├─► bridge/VLAN processing
       └─► protocol demux → IP, ARP, etc.
  ─────────────────────────────────────────────────────────────
  NAPI LAYER  ◄════════ WE ARE HERE
  (net/core/dev.c: napi_poll, net_rx_action)
       │
       │ napi_gro_receive() or netif_receive_skb()
       │ (called by driver poll function)
       │
       ▼
  ─────────────────────────────────────────────────────────────
  GRO ENGINE (net/core/gro.c)
  Aggregates packets before passing up
       │
       ▼
  ─────────────────────────────────────────────────────────────
  XDP HOOK (if XDP program loaded)
  Runs before sk_buff allocation
       │
       ▼
  ─────────────────────────────────────────────────────────────
  DEVICE DRIVER (drivers/net/ethernet/...)
  Poll function: reads NIC descriptor ring
       │
       │ DMA
       ▼
  NIC Hardware RX Ring Buffer
  ─────────────────────────────────────────────────────────────
  Physical / Virtual NIC
```

### Softirq Context — The Execution Environment

NAPI poll runs in **softirq context** (specifically `NET_RX_SOFTIRQ`). This is critical to understand:

```
Linux Interrupt Subsystem:

  ┌────────────────────────────────────────────────────────┐
  │ Hardware Interrupt (hardirq)                           │
  │  - Highest priority                                    │
  │  - Interrupts preemptible kernel code                  │
  │  - Cannot sleep                                        │
  │  - Cannot be preempted by softirq                      │
  │  - Runs with local IRQs disabled                       │
  └────────────────────────────────────────────────────────┘
           │
           │ (raises softirq flag)
           ▼
  ┌─────────────────────────────────────────────────────── ─┐
  │ Softirq (software interrupt)                            │
  │  - Lower priority than hardirq                          │
  │  - Runs with interrupts ENABLED                         │
  │  - Cannot sleep                                         │
  │  - Can be preempted by hardirq                          │
  │  - Runs in ksoftirqd or at hardirq return               │
  │  - Per-CPU execution                                    │
  │  - NET_RX_SOFTIRQ = index 3 in softirq table            │
  └─────────────────────────────────────────────────────── ─┘
           │
           │ (if softirq runs too long → ksoftirqd)
           ▼
  ┌────────────────────────────────────────────────────────┐
  │ ksoftirqd/N (kernel thread per CPU)                    │
  │  - Runs when softirq load is high                      │
  │  - Schedulable: can be deprioritized                   │
  │  - Visible in 'ps aux' as [ksoftirqd/0], [ksoftirqd/1] │
  └────────────────────────────────────────────────────────┘
```

**What "cannot sleep" means for drivers**: Your NAPI poll function runs in softirq context. You cannot:
- Call `schedule()` or `msleep()`
- Use `mutex_lock()` (use `spin_lock()`)
- Call any function that might sleep

You can:
- Use `spin_lock_bh()` (disables bottom-half / softirq on local CPU)
- Allocate memory with `GFP_ATOMIC`
- Use `napi_alloc_skb()` which uses the NAPI per-CPU cache

---

## 6. Core Data Structures — Complete Walkthrough

### 6.1 `struct napi_struct` — The Heart of NAPI

Location: `include/linux/netdevice.h`

```c
struct napi_struct {
    /*
     * poll_list: links this napi into the per-CPU softnet_data.poll_list.
     * When napi_schedule() is called, this napi is added to that list.
     * net_rx_action() iterates this list to find work to do.
     */
    struct list_head        poll_list;

    /*
     * state: bitmask of NAPI_STATE_* flags.
     * Atomic operations are used because IRQ handlers and softirq
     * can race when scheduling/completing NAPI.
     */
    unsigned long           state;

    /*
     * weight: maximum number of packets to process per poll() call.
     * Default is NAPI_POLL_WEIGHT = 64.
     * Setting it higher means fewer context switches but longer latency.
     * Setting it lower means more fairness but more overhead.
     */
    int                     weight;

    /*
     * defer_hard_irqs_count: used by interrupt coalescing.
     * If > 0, NAPI will defer re-enabling IRQs and instead
     * use a hrtimer to trigger the next poll.
     */
    int                     defer_hard_irqs_count;

    /*
     * gro_bitmask: bitmask indicating which GRO hash buckets have data.
     * Used to quickly check if GRO flush is needed without iterating
     * all GRO_HASH_BUCKETS.
     */
    unsigned long           gro_bitmask;

    /*
     * poll: the driver-provided callback function.
     * Called by net_rx_action() to drain the receive queue.
     * Arguments: (napi_struct*, budget)
     * Returns: number of packets processed (0..budget)
     */
    int                     (*poll)(struct napi_struct *, int);

#ifdef CONFIG_NETPOLL
    /* For netpoll: network console / crash dump over network */
    int                     poll_owner;
    struct netpoll_info     *npinfo;
#endif

    /*
     * dev: the net_device this NAPI belongs to.
     * Used for error reporting, weight lookup, and statistics.
     */
    struct net_device       *dev;

    /*
     * GRO state: hash table for in-progress GRO aggregation.
     * GRO_HASH_BUCKETS = 8 (as of kernel 5.x)
     * Each bucket has a list of partially-aggregated sk_buffs.
     */
    struct gro_list         gro_hash[GRO_HASH_BUCKETS];

    /*
     * skb: scratch SKB for napi_get_frags() / napi_gro_frags() path.
     * Drivers using page-based receive (page pool) use this.
     */
    struct sk_buff          *skb;

    /*
     * rx_list, rx_count: used by napi_consume_skb() and the
     * bulk-free path. SKBs are collected here for bulk kfree.
     */
    struct list_head        rx_list;
    int                     rx_count;

    /*
     * napi_id: globally unique ID for this NAPI instance.
     * Used by SO_INCOMING_NAPI_ID socket option to identify
     * which NAPI handled the last received packet.
     * Also used by busy polling to target the right NAPI.
     */
    unsigned int            napi_id;

    /*
     * timer: high-resolution timer for deferred IRQ re-enable.
     * When defer_hard_irqs is used, this timer fires to
     * re-enable NIC interrupts after a configurable delay.
     */
    struct hrtimer          timer;

    /*
     * thread: for threaded NAPI mode (Linux 5.12+).
     * Instead of running in softirq, poll() runs in this kthread.
     */
    struct task_struct      *thread;

    /*
     * list: used by netdev_napi_list to enumerate all NAPI
     * instances of a device (for ethtool, etc.)
     */
    struct list_head        list;
};
```

### NAPI State Flags

```c
enum {
    NAPI_STATE_SCHED,           /* bit 0: In poll_list, scheduled for polling */
    NAPI_STATE_MISSED,          /* bit 1: Received pkts while being polled
                                 * Driver called napi_schedule() during poll.
                                 * napi_complete_done() will reschedule. */
    NAPI_STATE_DISABLE,         /* bit 2: Disabled via napi_disable() */
    NAPI_STATE_NPSVC,           /* bit 3: Serving netpoll request */
    NAPI_STATE_LISTED,          /* bit 4: In netdev->napi_list */
    NAPI_STATE_NO_BUSY_POLL,    /* bit 5: Busy polling disabled for this NAPI */
    NAPI_STATE_IN_BUSY_POLL,    /* bit 6: Currently in busy poll loop */
    NAPI_STATE_PREFER_BUSY_POLL,/* bit 7: Prefer busy poll (set by epoll) */
    NAPI_STATE_THREADED,        /* bit 8: Threaded NAPI mode enabled */
    NAPI_STATE_SCHED_THREADED,  /* bit 9: Threaded NAPI scheduled */
};
```

### 6.2 `struct softnet_data` — Per-CPU Receive State

Location: `include/linux/netdevice.h`

Each CPU has one `softnet_data`. This is the glue between hardware IRQs and NAPI polling.

```c
struct softnet_data {
    /*
     * poll_list: the list of napi_struct instances ready to be polled.
     * net_rx_action() drains this list.
     * Protected by local softirq disabling (no lock needed since
     * this is per-CPU and softirqs don't preempt each other on the same CPU).
     */
    struct list_head        poll_list;

    /*
     * output_queue: TX queue of net_device instances waiting to
     * transmit packets. Separate from RX NAPI.
     */
    struct Qdisc            *output_queue;
    struct Qdisc            **output_queue_tailp;

    /*
     * completion_queue: SKBs that have been fully transmitted and
     * can be freed. Deferred to avoid freeing in hardirq context.
     */
    struct sk_buff          *completion_queue;

    /*
     * For the old netif_rx() path (legacy, rarely used now):
     * input_pkt_queue holds packets before softirq processes them.
     */
    struct sk_buff_head     input_pkt_queue;

    /* backlog: legacy NAPI instance used for the old netif_rx() path */
    struct napi_struct      backlog;

    /* tthrottle_jiffies: timestamp of last throttle for rate limiting */
    unsigned long           throttle_jiffies;

    /*
     * Statistics: these are per-CPU, so no locking needed.
     * Visible in /proc/net/softnet_stat.
     */
    unsigned int            total_rx_packets;  /* packets received */
    unsigned int            total_rx_bytes;    /* bytes received */
    unsigned int            dropped;           /* dropped: input queue full */
    unsigned int            time_squeeze;      /* budget/time limit hit */
    unsigned int            throttled;         /* throttled: budget pressure */
    unsigned int            received_rps;      /* packets via RPS */

    /* CPU ID: for multi-queue affinity checking */
    unsigned int            cpu;

#ifdef CONFIG_RPS
    /* RPS (Receive Packet Steering) state */
    struct rps_dev_flow_table __rcu *rps_flow_table;
#endif
};

/* Access pattern: always via this_cpu_ptr() */
DECLARE_PER_CPU_ALIGNED(struct softnet_data, softnet_data);
```

### 6.3 `struct sk_buff` — The Packet Descriptor

The `sk_buff` (socket buffer) is not unique to NAPI, but it's what NAPI allocates and passes upward. Key fields relevant to the NAPI path:

```c
struct sk_buff {
    /* 
     * Doubly-linked list pointers.
     * In receive path: queued in per-CPU input_pkt_queue or
     * in protocol receive queues.
     */
    struct sk_buff          *next;
    struct sk_buff          *prev;

    /* Timestamps: filled during NAPI processing */
    ktime_t                 tstamp;

    /* Network device that received this packet */
    struct net_device       *dev;

    /* 
     * Data pointers: the actual packet data lives in a linear
     * buffer plus optional page fragments.
     *
     * Memory layout:
     *
     *  head                               end
     *   ┌────────────────────────────── ──────┐
     *   │ headroom │   data   │    tailroom   │
     *   └──────────┬──────────┬───────────────┘
     *              │          │
     *             data       tail
     *
     * len = tail - data (linear data length)
     * data_len = length in page fragments
     * truesize = sizeof(sk_buff) + allocated head + frags
     */
    unsigned char           *head, *data;
    sk_buff_data_t          tail;
    sk_buff_data_t          end;
    unsigned int            len;       /* total length: linear + frags */
    unsigned int            data_len;  /* length in page frags */
    unsigned int            truesize;  /* memory charged to socket */

    /* 
     * napi_id: ID of the NAPI instance that received this packet.
     * Used by SO_INCOMING_NAPI_ID and busy polling.
     */
    unsigned int            napi_id;

    /* Protocol, VLAN, checksum state... */
    __be16                  protocol;
    __u16                   vlan_tci;
    __u8                    ip_summed;  /* checksum offload state */

    /* Page fragments for scatter-gather receive */
    skb_frag_t              frags[MAX_SKB_FRAGS];
    unsigned int            nr_frags;
};
```

### 6.4 `struct net_device` — NAPI-relevant Fields

```c
struct net_device {
    /* ... many fields ... */

    /*
     * napi_list: list of all napi_struct instances for this device.
     * Iterated during device up/down and by ethtool.
     */
    struct list_head        napi_list;

    /*
     * weight: per-device NAPI polling weight.
     * Can be set per-queue by drivers.
     * Overrides NAPI_POLL_WEIGHT for this device.
     */
    int                     weight;

    /*
     * Features: what the hardware can do (offloads).
     * NETIF_F_GRO: hardware supports GRO
     * NETIF_F_RXCSUM: hardware checksums
     * NETIF_F_LRO: hardware Large Receive Offload
     */
    netdev_features_t       features;

    /* ... */
};
```

### 6.5 Memory Layout: DMA Ring vs. SKB

```
NIC Hardware                    Kernel Memory (DMA-accessible)
───────────────────────────────────────────────────────────────

  NIC RX Descriptor Ring (in BAR or shared memory):
  ┌──────┬──────┬──────┬──────┬──────┬──────┐
  │ Desc │ Desc │ Desc │ Desc │ Desc │ Desc │  ← hardware writes status here
  │  [0] │  [1] │  [2] │  [3] │  [4] │  [5] │
  └──┬───┴──┬───┴──────┴──────┴──────┴──────┘
     │      │
     │      │ each descriptor points to a DMA buffer
     ▼      ▼
  ┌──────┐ ┌──────┐
  │ Buf0 │ │ Buf1 │  ← DMA buffers pre-allocated by driver,
  │(pkt) │ │(pkt) │    physical addresses written to descriptors
  └──────┘ └──────┘

During NAPI poll():
  1. Driver reads descriptor ring head pointer
  2. Checks status bit (DD = Descriptor Done)
  3. If DD set: packet ready
  4. Build sk_buff pointing to Buf0 (zero-copy if possible)
     or copy Buf0 into new sk_buff allocation (copy path)
  5. Refill: allocate new DMA buffer, write to Desc[0]
  6. Advance head pointer
  7. Pass sk_buff to napi_gro_receive()
```

---

## 7. NAPI State Machine

The NAPI state machine controls the synchronization between IRQ handlers, softirq, and napi_disable(). Getting this right prevents races where packets are dropped or double-processed.

```
                         ┌─────────────────────────────────────────────┐
                         │          NAPI State Transitions             │
                         └─────────────────────────────────────────────┘

  ┌──────────────┐
  │ UNREGISTERED │
  └──────┬───────┘
         │ netif_napi_add()
         │ Sets: NAPI_STATE_LISTED
         ▼
  ┌──────────────┐
  │  DISABLED    │  ← napi_disable() sets NAPI_STATE_DISABLE
  │              │    and waits for poll to complete
  └──────┬───────┘
         │ napi_enable()
         │ Clears: NAPI_STATE_DISABLE, NAPI_STATE_SCHED
         ▼
  ┌──────────────┐
  │    IDLE      │  ← NIC interrupts are ENABLED
  │              │    no entry in poll_list
  └──────┬───────┘
         │
         │ IRQ fires → irq_handler() calls napi_schedule()
         │
         │  napi_schedule_prep(): test_and_set_bit(NAPI_STATE_SCHED, state)
         │  Returns false if already SCHED → no double-scheduling
         │  If returns true: __napi_schedule() adds to poll_list
         │
         ▼
  ┌──────────────┐
  │  SCHEDULED   │  ← In softnet_data.poll_list
  │              │    NIC interrupts DISABLED
  └──────┬───────┘
         │ net_rx_action() picks this napi → calls napi->poll()
         ▼
  ┌──────────────┐
  │   POLLING    │  ← Actively draining RX ring
  │              │
  │ Can receive  │
  │ new schedule │
  │ request here │  ← Sets NAPI_STATE_MISSED
  └──────┬────┬──┘
         │    │
         │    └────────────────────────────────────────────┐
         │ queue empty                                     │ budget exhausted
         │ (work < budget)                                 │ (work == budget)
         ▼                                                 ▼
  ┌──────────── ──┐                               ┌──────────────────────┐
  │  napi_complete│                               │   Re-queue to        │
  │  _done()      │                               │   poll_list          │
  │               │                               │   (reschedule)       │
  │  clear SCHED  │                               └──────────────────────┘
  │  Re-enable    │
  │  NIC IRQs     │
  │               │
  │  if MISSED:   │  ← napi_schedule_prep() again
  │    reschedule │    if pkts arrived during poll
  └──────┬────────┘
         │
         ▼
       IDLE (if no MISSED)
    or SCHEDULED (if MISSED)
```

### The MISSED Flag Race Prevention

```c
/*
 * Scenario: IRQ fires while poll() is running
 *
 * Without MISSED flag:
 *   1. poll() running, clears SCHED flag, re-enables IRQ
 *   2. New packet arrives, IRQ fires
 *   3. napi_schedule() called: sets SCHED, adds to list
 *   4. WINDOW: between step 1 (re-enable IRQ) and step 3,
 *      if poll() sees empty ring and exits FIRST,
 *      the new packet waits for the next IRQ.
 *      This is fine and expected.
 *
 * The MISSED flag handles a different race:
 *   1. poll() is draining ring (SCHED is set)
 *   2. New IRQ fires: napi_schedule() sees SCHED already set
 *   3. Does NOT add to poll_list (already there)
 *   4. BUT sets NAPI_STATE_MISSED
 *   5. poll() finishes "all" packets
 *   6. napi_complete_done() checks MISSED
 *   7. Re-schedules instead of re-enabling IRQ
 *   → No packet left behind
 */
static inline bool napi_complete_done(struct napi_struct *n, int work_done)
{
    unsigned long flags, val, new;

    if (n->state & (NAPI_STATE_MISSED | ...)) {
        /* Clear MISSED and SCHED, then re-schedule */
        ...
        return false;  /* caller should NOT re-enable interrupts */
    }
    /* Clear SCHED, caller re-enables IRQs */
    return true;
}
```

---

## 8. NAPI Lifecycle Functions — Every API Explained

### 8.1 Registration

```c
/*
 * netif_napi_add() — register a NAPI instance with a net_device
 *
 * @dev:    the net_device (struct net_device *)
 * @napi:   the napi_struct to initialize (embedded in driver's priv struct)
 * @poll:   driver's poll callback function
 * @weight: max packets per poll call (use NAPI_POLL_WEIGHT = 64 as default)
 *
 * Call during driver probe() or when the queue is created.
 * Does NOT enable NAPI — call napi_enable() separately.
 */
void netif_napi_add(struct net_device *dev, struct napi_struct *napi,
                    int (*poll)(struct napi_struct *, int), int weight);

/*
 * Variant for TX completion NAPI (weight = 64, no GRO):
 */
void netif_napi_add_tx(struct net_device *dev, struct napi_struct *napi,
                       int (*poll)(struct napi_struct *, int));

/*
 * netif_napi_del() — unregister and clean up a NAPI instance
 * Call during driver remove() or queue destruction.
 * Must call napi_disable() BEFORE netif_napi_del().
 */
void netif_napi_del(struct napi_struct *napi);
```

### 8.2 Enable / Disable

```c
/*
 * napi_enable() — allow scheduling of this NAPI instance
 *
 * Clears NAPI_STATE_DISABLE. After this, IRQ handlers can call
 * napi_schedule(). Called during net_device open (ndo_open).
 */
void napi_enable(struct napi_struct *n);

/*
 * napi_disable() — prevent scheduling and wait for poll to complete
 *
 * 1. Sets NAPI_STATE_DISABLE atomically
 * 2. Waits (busy-loop with yield) until NAPI_STATE_SCHED is clear
 *    → guarantees poll() is not running when this returns
 *
 * Use before: device close (ndo_stop), queue reconfiguration,
 *             driver removal.
 *
 * Caution: If you hold a lock that the poll function also needs,
 *          this will deadlock. Release the lock before calling.
 */
void napi_disable(struct napi_struct *n);

/*
 * napi_synchronize() — wait for poll to complete (without disabling)
 * Less aggressive than napi_disable(): just ensures poll isn't running.
 * Useful for reconfiguring per-packet state that poll() reads.
 */
static inline void napi_synchronize(const struct napi_struct *n)
{
    /* on SMP: loop until SCHED flag is clear */
    while (test_bit(NAPI_STATE_SCHED, &n->state))
        msleep(1);
}
```

### 8.3 Scheduling (Called from IRQ Handler)

```c
/*
 * napi_schedule() — schedule this NAPI for polling from any context
 *
 * Typically called from the hardware IRQ handler.
 * Internally: calls napi_schedule_prep() + __napi_schedule()
 *
 * napi_schedule_prep(): atomically test_and_set NAPI_STATE_SCHED
 *   Returns true if we're the one who set it (we should schedule)
 *   Returns false if already set (don't double-add to list)
 *
 * __napi_schedule(): disables local softirq, adds napi to
 *   this_cpu_ptr(&softnet_data)->poll_list, raises NET_RX_SOFTIRQ
 */
static inline void napi_schedule(struct napi_struct *n)
{
    if (napi_schedule_prep(n))
        __napi_schedule(n);
}

/*
 * napi_schedule_irqoff() — like napi_schedule() but caller has IRQs disabled
 *
 * More efficient: skips local_irq_save/restore that napi_schedule() does.
 * Use this from hardirq handlers (where IRQs are already off).
 */
static inline void napi_schedule_irqoff(struct napi_struct *n)
{
    if (napi_schedule_prep(n))
        __napi_schedule_irqoff(n);
}

/*
 * napi_reschedule() — reschedule from within poll (budget exhausted)
 *
 * Called when poll() returns work_done == budget.
 * Adds back to poll_list for immediate re-polling.
 * Note: net_rx_action() handles this case automatically.
 * Drivers rarely call this directly.
 */
static inline bool napi_reschedule(struct napi_struct *napi)
{
    if (napi_schedule_prep(napi)) {
        __napi_schedule(napi);
        return true;
    }
    return false;
}
```

### 8.4 Completion (Called from Poll Function)

```c
/*
 * napi_complete_done() — declare poll cycle complete
 *
 * @n:         the napi_struct
 * @work_done: number of packets processed this cycle
 *
 * Call at the end of your poll() function when:
 *   a) The RX ring is empty (work_done < budget)
 *   b) You want to re-enable hardware interrupts
 *
 * Returns true if IRQs should be re-enabled.
 * Returns false if napi was re-scheduled (MISSED flag was set
 * because packets arrived during poll; don't re-enable IRQs).
 *
 * Usage pattern:
 *
 *   int my_poll(struct napi_struct *napi, int budget) {
 *       int work_done = 0;
 *       while (work_done < budget) {
 *           pkt = read_rx_ring();
 *           if (!pkt) break;
 *           process(pkt);
 *           work_done++;
 *       }
 *       if (work_done < budget) {
 *           if (napi_complete_done(napi, work_done))
 *               enable_irq(priv->irq); // re-arm
 *       }
 *       return work_done;
 *   }
 */
bool napi_complete_done(struct napi_struct *n, int work_done);

/*
 * napi_complete() — legacy version (work_done = 0, GRO not flushed)
 * Prefer napi_complete_done() for all new code.
 */
static inline bool napi_complete(struct napi_struct *n)
{
    return napi_complete_done(n, 0);
}
```

### 8.5 Packet Delivery Functions

```c
/*
 * napi_gro_receive() — deliver a packet through the GRO engine
 *
 * Main function for passing received packets from poll() to the stack.
 * GRO may hold the packet for aggregation or deliver immediately.
 *
 * Returns: GRO_NORMAL, GRO_HELD, GRO_MERGED, GRO_DROP
 * Driver doesn't need to check the return value for basic operation.
 */
gro_result_t napi_gro_receive(struct napi_struct *napi, struct sk_buff *skb);

/*
 * napi_gro_frags() — for scatter-gather (page fragment) receive
 *
 * Used when the driver builds skb using page fragments (page pool path).
 * The napi->skb is used as scratch space.
 * frags must be assembled via skb_add_rx_frag() before calling this.
 */
gro_result_t napi_gro_frags(struct napi_struct *napi);

/*
 * netif_receive_skb() — bypass GRO, deliver directly
 *
 * Use when you don't want GRO, or for non-GRO-able packets
 * (e.g., non-TCP traffic on a specialized path).
 * Can be called from any context (napi_receive_skb() is the napi variant).
 */
int netif_receive_skb(struct sk_buff *skb);

/*
 * napi_build_skb() — efficiently build sk_buff from pre-allocated buffer
 *
 * For XDP-to-stack fallback and page-pool-based drivers.
 * @data: pointer to the received data (must include headroom)
 * @frag_size: size of the allocation
 *
 * Returns skb with head pointing to @data.
 */
struct sk_buff *napi_build_skb(void *data, unsigned int frag_size);

/*
 * napi_alloc_skb() — allocate skb optimized for NAPI context
 *
 * Uses per-CPU NAPI fragmentation cache.
 * @napi: the current napi_struct
 * @length: desired linear data length
 *
 * Uses GFP_ATOMIC internally. Returns NULL on allocation failure.
 */
struct sk_buff *napi_alloc_skb(struct napi_struct *napi, unsigned int length);

/*
 * napi_consume_skb() — free skb in NAPI poll context (bulk-friendly)
 *
 * In NAPI poll context, collects skbs for bulk freeing.
 * budget != 0 means we're in NAPI poll → use bulk path.
 * budget == 0 means we're not in poll → immediate free.
 */
void napi_consume_skb(struct sk_buff *skb, int budget);
```

---

## 9. Softirq Integration — `net_rx_action` Deep Dive

This is the function that drives all NAPI polling. Understanding it is essential.

```c
/*
 * net_rx_action() — the NET_RX_SOFTIRQ handler
 * Location: net/core/dev.c
 *
 * Runs when NET_RX_SOFTIRQ is raised (by __napi_schedule).
 * Runs on the CPU that raised the softirq (the IRQ-handling CPU).
 *
 * Critical invariants:
 *   - Runs with softirqs disabled locally (no recursion)
 *   - Runs with local IRQs ENABLED (can be preempted by hardirq)
 *   - Per-CPU: no locking needed for softnet_data access
 *   - Time and budget limited: cannot starve other work
 */
static __latent_entropy void net_rx_action(struct softirq_action *h)
{
    struct softnet_data *sd = this_cpu_ptr(&softnet_data);

    /*
     * Budget: total packets across all NAPI instances.
     * Controlled by: sysctl net.core.netdev_budget (default: 300)
     *
     * This means: across all NICs on this CPU, process at most
     * 300 packets per softirq invocation.
     */
    int budget = netdev_budget;

    /*
     * Time limit: don't run longer than netdev_budget_usecs.
     * Default: 8000 microseconds (8ms).
     * Prevents softirq from monopolizing CPU.
     */
    unsigned long time_limit = jiffies +
        usecs_to_jiffies(netdev_budget_usecs);

    LIST_HEAD(list);    /* working list of napi instances */
    LIST_HEAD(repoll);  /* instances that need re-polling */

    /*
     * Atomically move poll_list to local list.
     * New additions go to sd->poll_list; we work from 'list'.
     * This prevents the endless loop of adding back while processing.
     */
    spin_lock_irq(&sd->poll_wait.lock);   /* in older kernels: not needed */
    list_splice_init(&sd->poll_list, &list);
    /* ... */

    for (;;) {
        struct napi_struct *n;

        /*
         * If our working list is empty, check if anything was
         * added to sd->poll_list while we were working.
         */
        if (list_empty(&list)) {
            /* check sd->poll_list for new additions */
            if (list_empty(&sd->poll_list))
                break;
            list_splice_init(&sd->poll_list, &list);
        }

        /* Take the first NAPI from the working list */
        n = list_first_entry(&list, struct napi_struct, poll_list);
        list_del_init(&n->poll_list);   /* remove from list */

        /*
         * napi_poll(): calls n->poll(n, weight) and handles
         * the state machine (MISSED flag, rescheduling logic).
         * Returns number of packets processed.
         */
        budget -= napi_poll(n, &repoll);

        /*
         * Time or budget exceeded: we can't keep going.
         * Merge repoll back to sd->poll_list, raise softirq again.
         * The softirq will run again after higher priority work
         * (or via ksoftirqd if load is extreme).
         */
        if (unlikely(budget <= 0 ||
                     time_after_eq(jiffies, time_limit))) {
            sd->time_squeeze++;         /* stat: budget/time hit */
            __raise_softirq_irqoff(NET_RX_SOFTIRQ);
            break;
        }
    }

    /* Move re-poll list back to sd->poll_list */
    list_splice_tail_init(&sd->poll_list, &repoll);
    list_splice(&repoll, &sd->poll_list);
    /* ... */
}

/*
 * napi_poll() — call driver's poll(), handle state
 * This wraps the driver's poll callback with state machine logic.
 */
static int napi_poll(struct napi_struct *n, struct list_head *repoll)
{
    void *have;
    int work, weight;

    /* Lock NAPI for exclusive polling (prevent concurrent poll from
     * another CPU — e.g., netpoll can call from different CPU) */
    have = netpoll_poll_lock(n);

    weight = n->weight;

    /* Call the driver's poll function */
    work = n->poll(n, weight);

    if (unlikely(work > weight))
        pr_err_once("NAPI poll function %pS returned %d, "
                    "exceeding its budget of %d.\n",
                    n->poll, work, weight);

    if (likely(work < weight))
        goto out_unlock;    /* queue empty: napi_complete_done was called */

    /*
     * work == weight: budget exhausted.
     * Check MISSED flag: was napi_schedule() called during poll?
     */
    if (unlikely(napi_disable_pending(n))) {
        /* napi_disable() was called: don't reschedule */
        napi_complete(n);
        goto out_unlock;
    }

    /* Re-add to repoll list: will be processed in next softirq round */
    if (n->gro_bitmask) {
        /* flush GRO if we're about to reschedule */
        ...
    }
    list_add_tail(&n->poll_list, repoll);

out_unlock:
    netpoll_poll_unlock(have);
    return work;
}
```

### The Budget Flow Visualized

```
net_rx_action() with 3 NAPI instances, budget=300:

  budget=300
  ┌──────────────────────────────────────────────────────┐
  │  poll_list: [napi_eth0] [napi_eth1] [napi_eth2]      │
  └──────────────────────────────────────────────────────┘

  Round 1: napi_eth0.poll(64) → returns 64 (full weight)
    budget = 300 - 64 = 236
    → added to repoll (budget exhausted for this instance)

  Round 2: napi_eth1.poll(64) → returns 30 (queue drained)
    budget = 236 - 30 = 206
    → napi_complete_done(30): IRQ re-enabled, removed from list

  Round 3: napi_eth2.poll(64) → returns 64 (full weight)
    budget = 206 - 64 = 142
    → added to repoll

  Round 4: napi_eth0.poll(64) → returns 10 (queue drained now)
    budget = 142 - 10 = 132
    → napi_complete_done(10): IRQ re-enabled

  Round 5: napi_eth2.poll(64) → returns 5 (queue drained)
    budget = 132 - 5 = 127
    → napi_complete_done(5): IRQ re-enabled

  Final: poll_list empty, budget=127, done.

  If at any point budget ≤ 0 or time limit hit:
    → raise NET_RX_SOFTIRQ again
    → sd->time_squeeze++
    → remaining napi instances stay in poll_list
```

---

## 10. Budget and Weight System

### Terminology Clarification

These terms are often confused:

| Term | What it controls | Scope | Default | Tunable |
|------|-----------------|-------|---------|---------|
| `netdev_budget` | Total packets across all NAPIs per softirq run | Per-CPU, per-softirq invocation | 300 | `sysctl net.core.netdev_budget` |
| `netdev_budget_usecs` | Time limit per softirq run | Per-CPU, per-softirq invocation | 8000 μs | `sysctl net.core.netdev_budget_usecs` |
| `napi->weight` | Max packets for one NAPI instance per poll call | Per-NAPI-instance | 64 (`NAPI_POLL_WEIGHT`) | `dev_weight` or driver-set |
| `dev_weight` | Default NAPI weight assigned to new instances | System-wide | 64 | `sysctl net.core.dev_weight` |
| `dev_weight_rx_bias` | Multiplier for RX weight | System-wide | 1 | `sysctl net.core.dev_weight_rx_bias` |

### Why 64 is the Default Weight

64 packets per poll call was chosen empirically as a balance:
- **Too small (e.g., 16)**: Frequent interrupts between poll cycles, high overhead
- **Too large (e.g., 256)**: Long polling latency for other NAPIs waiting in poll_list
- **64**: Each poll cycle takes roughly constant time, CPU cache footprint is reasonable

### The Budget Math — Fairness

With budget=300 and 8 NAPI instances each with weight=64:

```
Worst case: all 8 are busy
  300 / 64 ≈ 4.6 → ~4-5 NAPI instances can run per softirq invocation

  Round-robin is NOT guaranteed: first NAPI in list gets priority.
  This is why IRQ affinity and queue distribution matters.

Actual behavior with time limit:
  If packets are large (expensive to process), time limit hits first.
  If packets are small (cheap), budget count hits first.
  The minimum of {budget, time_limit} determines when to stop.
```

### Tuning Budget for High-Speed Links

```bash
# For 25Gbps+ links, increase budget significantly:
sysctl -w net.core.netdev_budget=600
sysctl -w net.core.netdev_budget_usecs=20000  # 20ms

# For 100Gbps:
sysctl -w net.core.netdev_budget=1000
sysctl -w net.core.netdev_budget_usecs=50000  # 50ms

# Increase per-device weight for all new NAPI instances:
sysctl -w net.core.dev_weight=128

# Note: very high budgets delay other softirq work.
# Monitor ksoftirqd CPU usage and application latency after changes.
```

---

## 11. GRO — Generic Receive Offload

### What GRO Does

GRO (Generic Receive Offload) is a software technique that merges multiple received packets into a single larger one before passing it up the network stack. This amortizes the per-packet processing overhead of the IP and TCP layers.

Without GRO:
```
100 × 1500-byte TCP segments → 100 separate sk_buffs → 100 IP header checks
                              → 100 TCP sequence number checks
                              → 100 socket buffer copies
                              → TCP receive window updates × 100
```

With GRO:
```
100 × 1500-byte TCP segments → aggregated into 3-4 × ~46KB "super-SKBs"
                              → 3-4 IP header checks
                              → 3-4 TCP sequence checks
                              → 3-4 socket buffer copies
```

### GRO Architecture

```
NAPI poll() calls napi_gro_receive(napi, skb) for each received packet:

  napi_gro_receive()
       │
       ▼
  dev_gro_receive()        ← protocol-specific GRO handler
       │
       │ checks:
       │  - Is this packet compatible with any in-progress flow?
       │  - IP: same src/dst, same proto
       │  - TCP: same 4-tuple, contiguous sequence numbers
       │  - Headers not modified (no options, standard TTL)
       │
       ├─► GRO_HELD     → packet merged into existing flow,
       │                   held in napi->gro_hash table
       │
       ├─► GRO_MERGED   → packet merged, skb freed
       │
       ├─► GRO_NORMAL   → can't aggregate, pass immediately
       │                   to netif_receive_skb()
       │
       └─► GRO_DROP     → discard (checksum error, etc.)

  At end of poll() (or budget exhaustion):
  napi_gro_flush(napi, flush_old)
       │
       ▼
  Flushes all held packets in gro_hash to netif_receive_skb()
  (merged as large sk_buffs)
```

### GRO Hash Table

```c
/*
 * GRO uses a hash table to quickly find matching flows.
 * GRO_HASH_BUCKETS = 8 (powers of 2 for fast modulo)
 *
 * Each bucket contains a list of in-progress aggregations:
 */
struct gro_list {
    struct list_head    list;
    int                 count;
};

/*
 * Hash key: based on packet headers.
 * For TCP/IPv4: hash(src_ip, dst_ip, src_port, dst_port, proto)
 * GRO is per-NAPI: flows are only merged within the same NAPI instance.
 * This is why each CPU/queue should ideally see the same flows (RSS).
 */
```

### GRO Data Path in Detail

```c
/*
 * When a TCP/IPv4 packet arrives at dev_gro_receive():
 *
 * 1. Compute hash of the 5-tuple
 * 2. Look up gro_hash[hash % GRO_HASH_BUCKETS]
 * 3. Walk the list looking for a matching in-progress aggregation
 *
 * If found (matching flow):
 *   Check: is this the next expected sequence number?
 *     Yes → merge:
 *       a. Extend the existing "head" SKB's data_len
 *       b. Add new skb to skb_shinfo(head)->frag_list
 *          OR merge into page fragments (if page pool)
 *       c. Update TCP/IP headers in head SKB
 *       d. Increment gro_count in head
 *       e. Return GRO_MERGED
 *     No → flush existing, start new (return GRO_NORMAL for old)
 *
 * If not found (new flow):
 *   Clone or reference the SKB, add to gro_hash list
 *   Return GRO_HELD
 *
 * Flushing triggers (to prevent excessive holding):
 *   1. End of napi poll() via napi_gro_flush()
 *   2. GRO list count exceeds MAX_GRO_SKBS (8 in same bucket)
 *   3. flush_old=true after budget exhaustion: flush old flows
 *   4. Timeout: napi_skb_finish() checks age
 */

/*
 * Result from the application's perspective:
 * Instead of recv() returning 1460 bytes, it returns 43800 bytes
 * (30 × 1460-byte segments merged)
 * recv() is called 30× fewer times
 * Context switch overhead: 30× lower
 */
```

### GRO and Checksum Offload Interaction

```c
/*
 * GRO works best when hardware computes checksums.
 * When ip_summed == CHECKSUM_UNNECESSARY:
 *   GRO can skip per-segment checksum verification
 *
 * When ip_summed == CHECKSUM_COMPLETE:
 *   GRO accumulates checksum across merged segments
 *   Single checksum verify for the entire aggregation
 *
 * When ip_summed == CHECKSUM_NONE (no offload):
 *   GRO still works but performs per-segment checksum verify
 *   before merging → slightly higher CPU cost
 */
```

---

## 12. Multi-Queue NAPI and Per-CPU Architecture

### Single-Queue vs. Multi-Queue

```
Single-Queue NIC (old model):
┌────────────────────────────────────────────┐
│ NIC Hardware                               │
│  ┌──────────────────────────────────────┐  │
│  │ Single RX Queue (ring buffer)        │  │
│  │  [desc0][desc1][desc2]...[descN]     │  │
│  └──────────────────────────────────────┘  │
│  Single IRQ → CPU 0 only                   │
└────────────────────────────────────────────┘

Problem: CPU 0 handles all traffic. CPUs 1-N idle.
         Single NAPI instance. Bottleneck at ~1-2 Mpps.

Multi-Queue NIC (modern):
┌────────────────────────────────────────────────────────────┐
│ NIC Hardware                                               │
│  Queue 0: [desc0..descN] → IRQ 0 → CPU 0 → napi[0]         │
│  Queue 1: [desc0..descN] → IRQ 1 → CPU 1 → napi[1]         │
│  Queue 2: [desc0..descN] → IRQ 2 → CPU 2 → napi[2]         │
│  Queue 3: [desc0..descN] → IRQ 3 → CPU 3 → napi[3]         │
│  ...                                                       │
│  Queue N: [desc0..descN] → IRQ N → CPU N → napi[N]         │
└────────────────────────────────────────────────────────────┘

Each queue has its own:
  - DMA ring buffer (RX descriptors)
  - IRQ vector (MSI-X)
  - NAPI instance
  - GRO hash table
  - Per-CPU softnet_data binding
```

### Driver Implementation for Multi-Queue

```c
struct my_driver_priv {
    struct net_device   *dev;
    int                 num_queues;
    struct my_rx_queue  *rx_queues;  /* array of per-queue state */
};

struct my_rx_queue {
    struct napi_struct  napi;
    struct my_desc      *desc_ring;     /* DMA descriptor ring */
    dma_addr_t          desc_dma;       /* physical address */
    struct sk_buff      **skb_pool;     /* SKB pointer array */
    dma_addr_t          *buf_dma;       /* DMA addresses of buffers */
    int                 head;           /* consumer index */
    int                 count;          /* ring size */
    int                 irq;            /* MSI-X vector number */
    int                 cpu;            /* pinned CPU */
    /* ... */
};

/* During probe: */
static int my_driver_probe(struct pci_dev *pdev, ...) {
    int num_queues = min(pci_msix_vec_count(pdev), num_online_cpus());

    priv->rx_queues = kcalloc(num_queues, sizeof(*priv->rx_queues), GFP_KERNEL);

    for (int i = 0; i < num_queues; i++) {
        struct my_rx_queue *q = &priv->rx_queues[i];

        /* Register a separate NAPI instance per queue */
        netif_napi_add(dev, &q->napi, my_napi_poll, NAPI_POLL_WEIGHT);

        /* Allocate DMA ring, SKB pool, etc. */
        my_alloc_rx_ring(q, RX_RING_SIZE);
    }
}
```

### NUMA-Aware Queue Assignment

```
NUMA topology for a dual-socket server:

  Socket 0 (CPUs 0-15, RAM Bank 0)    Socket 1 (CPUs 16-31, RAM Bank 1)
  ┌───────────────────────────┐        ┌───────────────────────────┐
  │  CPU 0    CPU 1           │        │  CPU 16   CPU 17          │
  │  CPU 2    CPU 3           │        │  CPU 18   CPU 19          │
  │  ...      ...             │        │  ...      ...             │
  │                           │        │                           │
  │  L3 Cache                 │        │  L3 Cache                 │
  │  DIMM 0-3 (RAM Bank 0)    │        │  DIMM 4-7 (RAM Bank 1)    │
  └───────────┬───────────────┘        └───────────────────────────┘
              │
       PCIe slot (NIC) — NUMA node 0

Optimal assignment:
  NIC queues 0-15 → CPUs 0-15 (socket 0, local NUMA)
  DMA buffers allocated from NODE 0 memory:
    alloc_pages_node(dev_to_node(&pdev->dev), GFP_KERNEL, 0)

If NIC is on PCIe attached to socket 0:
  Queues pinned to socket 1 CPUs → all DMA reads cross QPI/UPI
  Performance penalty: 40-60% throughput reduction
```

---

## 13. RSS, RPS, and RFS — Packet Steering

These three mechanisms work together with NAPI to distribute load across CPUs:

### RSS — Receive Side Scaling (Hardware)

```
RSS: The NIC hashes packet headers to determine which queue receives each packet.

┌─────────────────────────────────────────────────────────────────────┐
│  NIC Hardware RSS Engine                                            │
│                                                                     │
│  Packet arrives →  5-tuple hash: {src_ip, dst_ip, proto,            │
│                                    src_port, dst_port}              │
│                   OR 2-tuple: {src_ip, dst_ip} (for ICMP)           │
│                                                                     │
│  Toeplitz hash function (standard) applied to the 5-tuple:          │
│                                                                     │
│  hash = Toeplitz(key, tuple)  →  hash % num_queues  =  queue_id     │
│                                                                     │
│  Indirection table (RETA): 128-512 entries mapping hash → queue:    │
│  [0] → queue 0   [1] → queue 1   [2] → queue 0   [3] → queue 3      │
│  ...                                                                │
│                                                                     │
│  Result: All packets of the same TCP connection go to SAME queue    │
│  → Same NAPI instance → Same GRO context → Correct aggregation      │
└─────────────────────────────────────────────────────────────────────┘

Configure RSS:
  ethtool -L eth0 combined 8          # set 8 combined queues
  ethtool -X eth0 equal 8             # distribute equally
  ethtool -x eth0                     # show RETA
  # Use custom Toeplitz key for specific flow distribution:
  ethtool -X eth0 hkey <key>
```

### RPS — Receive Packet Steering (Software, for single-queue NICs)

```
RPS: Software implementation of RSS for NICs without hardware multi-queue.

  NIC → Single RX Queue → Single NAPI (CPU 0) → receives ALL packets
                                │
                                │ __netif_receive_skb_core()
                                │ RPS is applied here (before protocol dispatch)
                                ▼
  get_rps_cpu(dev, skb, &rflow):
    1. Hash skb (same Toeplitz-like hash on 5-tuple)
    2. Map hash to CPU via rps_map
    3. If different from current CPU:
       a. Enqueue skb to target CPU's backlog queue
       b. Send IPI (inter-processor interrupt) to wake target CPU
    4. Target CPU processes via its backlog NAPI

Configure RPS:
  # Enable RPS on eth0 queue 0, all CPUs (bitmask):
  echo f > /sys/class/net/eth0/queues/rx-0/rps_cpus
  # f = 0b1111 = CPUs 0,1,2,3

  # Set flow table size (for cache locality):
  echo 4096 > /sys/class/net/eth0/queues/rx-0/rps_flow_cnt
```

### RFS — Receive Flow Steering (Software, improves locality)

```
RFS: Extends RPS to steer flows to the CPU running the application thread
     that will consume the data. Improves cache locality:
     NIC → CPU 0 (NAPI) → CPU 3 (application) = cache-cold on CPU 3
     RFS: NIC → CPU 0 (NAPI) → CPU 3 (app, steered) = cache-warm

How it works:
  1. When application calls recv() on CPU 3, kernel records:
     rps_sock_flow_table[flow_hash] = 3   (CPU 3 is processing this flow)
  2. When NIC receives packet for same flow on CPU 0:
     RPS maps flow_hash → checks sock_flow_table → routes to CPU 3
  3. CPU 3 receives packet in its backlog, processes in its softirq
     Application calls recv(), data is in CPU 3's L1/L2 cache

Configure RFS:
  sysctl -w net.core.rps_sock_flow_entries=32768
  echo 32768 > /sys/class/net/eth0/queues/rx-0/rps_flow_cnt

Combined RSS + RFS:
  RSS + RFS together = "Accelerated RFS" (aRFS):
  NIC can use socket flow table to steer to correct queue via
  ethtool ntuple filters. Requires NIC support.
  ethtool -K eth0 ntuple on
```

---

## 14. Complete C Driver Implementation with NAPI

This is a complete, commented, compilable Linux kernel module that implements a minimal virtual NIC driver using NAPI. It exercises the full NAPI lifecycle.

```c
// SPDX-License-Identifier: GPL-2.0-only
/*
 * demo_napi.c — Minimal NAPI-enabled network driver demo
 *
 * This creates a virtual NIC (demnapi0) that loops transmitted
 * packets back to the receive path, demonstrating the complete
 * NAPI lifecycle.
 *
 * Build: Add to drivers/net/ and add to Makefile, or build
 * as external module with:
 *   make -C /lib/modules/$(uname -r)/build M=$(pwd) modules
 */

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/netdevice.h>
#include <linux/etherdevice.h>
#include <linux/skbuff.h>
#include <linux/interrupt.h>
#include <linux/workqueue.h>
#include <linux/spinlock.h>
#include <linux/ethtool.h>

MODULE_LICENSE("GPL v2");
MODULE_AUTHOR("Demo");
MODULE_DESCRIPTION("NAPI driver demonstration");

#define DRV_NAME        "demo_napi"
#define RX_RING_SIZE    256          /* must be power of 2 */
#define TX_RING_SIZE    256

/*
 * Simulated hardware descriptor.
 * In a real driver, this matches the NIC's hardware descriptor format.
 * The NIC DMA engine writes packet data here and sets the 'done' flag.
 */
struct demo_desc {
    volatile uint32_t   status;      /* bit 0: done (DD) */
    uint32_t            length;      /* packet length in bytes */
    uint8_t             data[1600];  /* packet data (simulated DMA buffer) */
};

#define DESC_DONE   BIT(0)           /* descriptor done bit */

/*
 * Per-queue state.
 * In a real multi-queue driver, there are multiple of these.
 */
struct demo_rx_ring {
    struct demo_desc    *descs;      /* ring of descriptors */
    int                 head;        /* next descriptor to process */
    int                 count;       /* ring size */
    spinlock_t          lock;        /* protect head/tail */
};

struct demo_tx_ring {
    struct demo_desc    *descs;
    int                 head;        /* next to transmit */
    int                 tail;        /* next free */
    int                 count;
    spinlock_t          lock;
};

/*
 * Driver private structure.
 * Embedded in net_device via netdev_priv().
 */
struct demo_priv {
    struct net_device   *dev;
    struct napi_struct  napi;           /* one NAPI instance */
    struct demo_rx_ring rx;
    struct demo_tx_ring tx;

    /* Statistics (64-bit for accuracy, read atomically) */
    u64                 rx_packets;
    u64                 rx_bytes;
    u64                 tx_packets;
    u64                 tx_bytes;
    u64                 rx_dropped;

    /* Simulated IRQ: in a real driver this comes from the NIC */
    struct hrtimer      fake_irq_timer;  /* fires to simulate packet arrival */
    bool                generating;
    spinlock_t          stats_lock;
};

/* ─────────────────────────────────────────────────────────────────────
 * Receive Descriptor Ring Management
 * ─────────────────────────────────────────────────────────────────── */

static int demo_rx_ring_alloc(struct demo_rx_ring *ring, int count)
{
    ring->descs = vzalloc(count * sizeof(struct demo_desc));
    if (!ring->descs)
        return -ENOMEM;
    ring->count = count;
    ring->head = 0;
    spin_lock_init(&ring->lock);
    return 0;
}

static void demo_rx_ring_free(struct demo_rx_ring *ring)
{
    vfree(ring->descs);
    ring->descs = NULL;
}

/*
 * Simulate packet arrival: write fake packet data into descriptor.
 * In a real driver, the hardware DMA engine does this.
 */
static void demo_inject_rx_packet(struct demo_priv *priv, const char *payload,
                                  int len)
{
    struct demo_rx_ring *ring = &priv->rx;
    struct demo_desc *desc;
    unsigned long flags;
    int next;

    spin_lock_irqsave(&ring->lock, flags);

    /* Find next free descriptor (using tail, skipping head) */
    next = (ring->head + 1) % ring->count;
    desc = &ring->descs[ring->head];

    if (desc->status & DESC_DONE) {
        /* Ring full: drop */
        spin_unlock_irqrestore(&ring->lock, flags);
        return;
    }

    /* Build an Ethernet frame */
    memcpy(desc->data, payload, min(len, 1600));
    desc->length = len;

    /* Set DONE flag — hardware would do this via DMA */
    wmb();  /* write memory barrier: ensure data written before status */
    desc->status = DESC_DONE;

    spin_unlock_irqrestore(&ring->lock, flags);

    /*
     * Trigger NAPI: in a real driver, the NIC raises an IRQ here.
     * We simulate it by scheduling NAPI directly.
     */
    napi_schedule(&priv->napi);
}

/* ─────────────────────────────────────────────────────────────────────
 * NAPI Poll Function — Core of the NAPI Driver
 * ─────────────────────────────────────────────────────────────────── */

/*
 * demo_napi_poll() — called by net_rx_action() to drain RX ring
 *
 * @napi:   our napi_struct (use container_of to get priv)
 * @budget: maximum packets to process this call
 *
 * Returns: number of packets processed (MUST be <= budget)
 *
 * Contract with the NAPI core:
 *   - Return < budget AND call napi_complete_done() → queue empty, IRQ re-armed
 *   - Return == budget → budget exhausted, NAPI core will reschedule
 *   - MUST NOT call napi_complete_done() if returning == budget
 */
static int demo_napi_poll(struct napi_struct *napi, int budget)
{
    /*
     * Get our private state from the napi pointer.
     * container_of() uses the offset of 'napi' within demo_priv.
     */
    struct demo_priv *priv = container_of(napi, struct demo_priv, napi);
    struct demo_rx_ring *ring = &priv->rx;
    int work_done = 0;

    /*
     * Main polling loop: drain descriptors up to budget.
     *
     * IMPORTANT: Check work_done < budget FIRST.
     * If we check ring empty first, we might miss the budget check
     * and return work_done == budget incorrectly.
     */
    while (work_done < budget) {
        struct demo_desc *desc;
        struct sk_buff *skb;
        int head;

        /* Read and check current descriptor */
        head = ring->head;
        desc = &ring->descs[head];

        /*
         * Read memory barrier: ensure we see the hardware's DONE bit.
         * Without rmb(), CPU might reorder the status read before data.
         */
        rmb();

        if (!(desc->status & DESC_DONE))
            break;  /* No more packets: ring empty */

        /*
         * Allocate an sk_buff to hold this packet.
         * napi_alloc_skb() uses the per-CPU NAPI frag cache,
         * which is more efficient than alloc_skb() in this context.
         */
        skb = napi_alloc_skb(napi, desc->length);
        if (unlikely(!skb)) {
            /* Allocation failure: drop and move on */
            priv->rx_dropped++;
            goto next_desc;
        }

        /*
         * Copy packet data into SKB.
         *
         * In a real driver with DMA, this is zero-copy:
         *   build_skb(dma_buf, len) or napi_build_skb()
         * We copy here because this is a simulation without real DMA.
         */
        skb_put_data(skb, desc->data, desc->length);

        /*
         * Set SKB metadata:
         */

        /* Which NIC received this packet */
        skb->dev = priv->dev;

        /* Protocol: parse from Ethernet type field.
         * eth_type_trans() sets skb->protocol and skb->pkt_type,
         * and advances skb->data past the Ethernet header.
         */
        skb->protocol = eth_type_trans(skb, priv->dev);

        /*
         * Checksum: tell the stack our hardware verified it.
         * CHECKSUM_UNNECESSARY = no need to verify in software.
         * For a real driver, only set this if NIC confirmed it.
         */
        skb->ip_summed = CHECKSUM_UNNECESSARY;

        /*
         * Record receive timestamp.
         * netif_receive_skb() will use this if SO_TIMESTAMPING is active.
         */
        skb->tstamp = ktime_get_real();

        /* Update statistics */
        priv->rx_packets++;
        priv->rx_bytes += skb->len;
        work_done++;

        /*
         * Deliver packet to the network stack via GRO.
         * napi_gro_receive() may hold the packet for GRO aggregation,
         * or pass it up immediately.
         *
         * Alternative: netif_receive_skb(skb) — bypass GRO.
         * Use netif_receive_skb() for non-GRO-able traffic or
         * when you've already done aggregation yourself.
         */
        napi_gro_receive(napi, skb);

next_desc:
        /* Clear descriptor so hardware (or simulation) can reuse it */
        desc->status = 0;
        desc->length = 0;

        /* Advance ring head (modular) */
        ring->head = (ring->head + 1) % ring->count;
    }

    /*
     * Work_done < budget means: the ring is empty.
     * We can re-enable hardware interrupts and exit NAPI mode.
     *
     * napi_complete_done():
     *   - Removes napi from poll_list
     *   - Clears NAPI_STATE_SCHED
     *   - If NAPI_STATE_MISSED: re-schedules (returns false)
     *   - Otherwise: returns true → we re-enable IRQ
     *
     * If we do NOT call napi_complete_done() when returning < budget,
     * NAPI will be stuck scheduled forever. This is a driver bug.
     */
    if (work_done < budget) {
        if (napi_complete_done(napi, work_done)) {
            /*
             * Re-enable hardware interrupts.
             * In a real driver: write to NIC interrupt mask register.
             * Our simulation: let the hrtimer fire again.
             */
            /* iowrite32(INTR_ENABLE, priv->hw_base + INTR_MASK_REG); */
        }
    }

    return work_done;
}

/* ─────────────────────────────────────────────────────────────────────
 * Transmit Path
 * ─────────────────────────────────────────────────────────────────── */

static netdev_tx_t demo_start_xmit(struct sk_buff *skb,
                                    struct net_device *dev)
{
    struct demo_priv *priv = netdev_priv(dev);

    /*
     * For this loopback demo: inject the transmitted packet
     * back as a received packet (simulates a loopback NIC).
     *
     * In a real driver:
     *   1. Map skb data for DMA: dma_map_single()
     *   2. Write TX descriptor with DMA address and length
     *   3. Ring TX doorbell register to notify NIC
     *   4. NIC DMA-reads the data and transmits
     *   5. NIC signals TX completion via IRQ
     *   6. TX completion handler: dma_unmap_single(), dev_kfree_skb()
     */
    priv->tx_packets++;
    priv->tx_bytes += skb->len;

    /* Loopback: re-inject as RX */
    demo_inject_rx_packet(priv, skb->data, skb->len);

    /* Free the transmitted SKB */
    dev_kfree_skb(skb);

    return NETDEV_TX_OK;
}

/* ─────────────────────────────────────────────────────────────────────
 * Device Open / Close
 * ─────────────────────────────────────────────────────────────────── */

static int demo_open(struct net_device *dev)
{
    struct demo_priv *priv = netdev_priv(dev);

    netdev_info(dev, "opening device\n");

    /*
     * napi_enable(): clears NAPI_STATE_DISABLE flag.
     * After this, IRQ handlers can call napi_schedule().
     * Must be called before requesting the IRQ or starting DMA.
     */
    napi_enable(&priv->napi);

    /*
     * In a real driver, here you would:
     *   request_irq(priv->irq, irq_handler, ...)
     *   setup DMA, enable NIC TX/RX
     *   enable NIC interrupts
     */

    /* Start the TX queue so the stack can send packets */
    netif_start_queue(dev);

    return 0;
}

static int demo_close(struct net_device *dev)
{
    struct demo_priv *priv = netdev_priv(dev);

    netdev_info(dev, "closing device\n");

    /* Stop the TX queue first: no new transmits */
    netif_stop_queue(dev);

    /*
     * napi_disable(): waits until any running poll() completes,
     * then sets NAPI_STATE_DISABLE to prevent future scheduling.
     *
     * MUST call before:
     *   - free_irq() (real driver)
     *   - Freeing DMA buffers
     *   - Removing the device
     *
     * Cannot hold any lock that poll() also acquires!
     */
    napi_disable(&priv->napi);

    /*
     * In a real driver:
     *   disable NIC interrupts
     *   free_irq(priv->irq, priv)
     *   stop DMA
     */

    return 0;
}

/* ─────────────────────────────────────────────────────────────────────
 * Statistics and ethtool
 * ─────────────────────────────────────────────────────────────────── */

static void demo_get_stats64(struct net_device *dev,
                              struct rtnl_link_stats64 *stats)
{
    struct demo_priv *priv = netdev_priv(dev);

    stats->rx_packets   = priv->rx_packets;
    stats->rx_bytes     = priv->rx_bytes;
    stats->tx_packets   = priv->tx_packets;
    stats->tx_bytes     = priv->tx_bytes;
    stats->rx_dropped   = priv->rx_dropped;
}

static const struct net_device_ops demo_netdev_ops = {
    .ndo_open           = demo_open,
    .ndo_stop           = demo_close,
    .ndo_start_xmit     = demo_start_xmit,
    .ndo_get_stats64    = demo_get_stats64,
    .ndo_set_mac_address = eth_mac_addr,
    .ndo_validate_addr  = eth_validate_addr,
};

/* ─────────────────────────────────────────────────────────────────────
 * Module Init / Exit
 * ─────────────────────────────────────────────────────────────────── */

static struct net_device *demo_dev;

static int __init demo_napi_init(void)
{
    struct demo_priv *priv;
    int ret;

    /*
     * alloc_etherdev() allocates net_device + sizeof(demo_priv)
     * and sets up Ethernet defaults.
     * Alternative: alloc_netdev(sizeof(demo_priv), "demnapi%d", ...)
     */
    demo_dev = alloc_etherdev(sizeof(struct demo_priv));
    if (!demo_dev)
        return -ENOMEM;

    /* Set device name */
    strscpy(demo_dev->name, "demnapi%d", IFNAMSIZ);

    priv = netdev_priv(demo_dev);
    priv->dev = demo_dev;
    spin_lock_init(&priv->stats_lock);

    /* Allocate RX ring */
    ret = demo_rx_ring_alloc(&priv->rx, RX_RING_SIZE);
    if (ret) {
        free_netdev(demo_dev);
        return ret;
    }

    /*
     * netif_napi_add() — register NAPI instance.
     *
     * Arguments:
     *   dev:    the net_device
     *   napi:   our napi_struct (embedded in priv)
     *   poll:   our poll function
     *   weight: NAPI_POLL_WEIGHT (64) — use this default
     *           unless you have a reason to change it
     *
     * Effect:
     *   - Initializes napi->state, napi->weight, napi->dev
     *   - Sets napi->poll = demo_napi_poll
     *   - Assigns a unique napi->napi_id
     *   - Sets NAPI_STATE_LISTED
     *   - Adds to dev->napi_list
     *   - Does NOT enable NAPI (call napi_enable() in ndo_open)
     */
    netif_napi_add(demo_dev, &priv->napi, demo_napi_poll, NAPI_POLL_WEIGHT);

    /* Set net_device ops */
    demo_dev->netdev_ops = &demo_netdev_ops;

    /* 
     * Set hardware features this (fake) NIC supports:
     * NETIF_F_GRO: enable GRO (we set CHECKSUM_UNNECESSARY so it works well)
     */
    demo_dev->features |= NETIF_F_GRO;
    demo_dev->hw_features = demo_dev->features;

    /* Assign a random MAC address */
    eth_hw_addr_random(demo_dev);

    /* Register with the kernel */
    ret = register_netdev(demo_dev);
    if (ret) {
        netif_napi_del(&priv->napi);
        demo_rx_ring_free(&priv->rx);
        free_netdev(demo_dev);
        return ret;
    }

    pr_info("demo_napi: registered %s\n", demo_dev->name);
    return 0;
}

static void __exit demo_napi_exit(void)
{
    struct demo_priv *priv = netdev_priv(demo_dev);

    unregister_netdev(demo_dev);  /* calls ndo_stop → napi_disable() */

    /*
     * netif_napi_del(): unlinks napi from dev->napi_list,
     * frees GRO resources.
     * Call AFTER napi_disable() and AFTER unregister_netdev().
     */
    netif_napi_del(&priv->napi);

    demo_rx_ring_free(&priv->rx);
    free_netdev(demo_dev);
    pr_info("demo_napi: unregistered\n");
}

module_init(demo_napi_init);
module_exit(demo_napi_exit);
```

### The IRQ Handler Pattern (Real Hardware)

```c
/*
 * In a real PCI NIC driver, the IRQ handler looks like this:
 */
static irqreturn_t my_irq_handler(int irq, void *data)
{
    struct my_priv *priv = data;
    u32 status;

    /*
     * Read interrupt status register.
     * This also ACKs the interrupt on most NICs.
     */
    status = ioread32(priv->hw_base + INTR_STATUS_REG);

    if (!(status & (INTR_RX | INTR_TX | INTR_ERR)))
        return IRQ_NONE;  /* Not our interrupt (shared IRQ) */

    if (status & INTR_RX) {
        /*
         * DISABLE RX interrupts in hardware.
         * This is CRITICAL: if we don't do this, the NIC will keep
         * raising IRQs for every received packet, defeating NAPI.
         *
         * How to disable depends on NIC:
         *   Option 1: Write to interrupt mask register
         *   Option 2: Write to interrupt clear register
         *   Option 3: Some NICs auto-disable on reading status
         */
        iowrite32(ioread32(priv->hw_base + INTR_MASK_REG) & ~INTR_RX,
                  priv->hw_base + INTR_MASK_REG);

        /*
         * Schedule NAPI for this queue.
         * napi_schedule_irqoff() is more efficient here because
         * we're in hardirq context (IRQs are already disabled).
         */
        napi_schedule_irqoff(&priv->napi);
    }

    if (status & INTR_TX) {
        /* TX completion: free transmitted SKBs, unmap DMA */
        my_tx_complete(priv);
    }

    return IRQ_HANDLED;
}

/*
 * In the poll function, when re-enabling interrupts:
 *
 * if (napi_complete_done(napi, work_done)) {
 *     // Re-enable RX interrupts:
 *     iowrite32(ioread32(priv->hw_base + INTR_MASK_REG) | INTR_RX,
 *               priv->hw_base + INTR_MASK_REG);
 *
 *     // Potential race window: new packet arrives here, before
 *     // interrupt enable write completes. NIC should handle this
 *     // gracefully by raising IRQ for the pending packet.
 *     // If NIC doesn't: check ring one more time after re-enable.
 * }
 */
```

---

## 15. Rust for Linux — NAPI Abstractions and Driver

Linux 6.1+ includes Rust support via the `rust/` directory. The Rust for Linux (R4L) project provides safe abstractions over the C kernel APIs.

### Rust Networking Abstractions (R4L)

```rust
// rust/kernel/net/phy.rs and future rust/kernel/net/dev.rs
// As of Linux 6.8, full NAPI Rust abstractions are still being developed.
// Below shows the architectural pattern with existing and in-progress code.

// File: rust/kernel/net/dev.rs (conceptual — matches R4L direction)

use kernel::prelude::*;
use kernel::net::device::{Device, Registration};
use kernel::sync::SpinLock;

/// Safe Rust wrapper around `struct napi_struct`
pub struct NapiStruct {
    /// Raw pointer to the C napi_struct (allocated with the net_device)
    napi: *mut bindings::napi_struct,
}

// SAFETY: napi_struct is pinned and per-device. We ensure
// exclusive access through the NAPI state machine.
unsafe impl Send for NapiStruct {}
unsafe impl Sync for NapiStruct {}

impl NapiStruct {
    /// Register a NAPI instance with a net_device.
    /// Corresponds to: netif_napi_add()
    pub fn add(
        dev: &Device,
        poll: unsafe extern "C" fn(*mut bindings::napi_struct, i32) -> i32,
        weight: i32,
    ) -> Result<Self> {
        let napi = unsafe {
            // Allocate within the net_device's private area
            let ptr = bindings::netdev_priv(dev.as_ptr()) as *mut bindings::napi_struct;
            bindings::netif_napi_add(dev.as_ptr(), ptr, Some(poll), weight);
            ptr
        };
        Ok(NapiStruct { napi })
    }

    /// Enable NAPI polling. Call in ndo_open().
    pub fn enable(&self) {
        // SAFETY: napi is valid and properly initialized.
        unsafe { bindings::napi_enable(self.napi) }
    }

    /// Disable NAPI polling. Call in ndo_stop().
    pub fn disable(&self) {
        // SAFETY: napi is valid; we wait for poll to complete.
        unsafe { bindings::napi_disable(self.napi) }
    }

    /// Schedule NAPI from an IRQ handler (IRQs disabled).
    pub fn schedule_irqoff(&self) {
        // SAFETY: called in hardirq context where IRQs are disabled.
        unsafe {
            if bindings::napi_schedule_prep(self.napi) {
                bindings::__napi_schedule_irqoff(self.napi);
            }
        }
    }

    /// Complete poll cycle when queue is empty.
    /// Returns true if IRQs should be re-enabled.
    pub fn complete_done(&self, work_done: i32) -> bool {
        // SAFETY: called from within poll(), napi is valid.
        unsafe { bindings::napi_complete_done(self.napi, work_done) }
    }

    /// Deliver packet through GRO.
    pub fn gro_receive(&self, skb: SkBuff) -> GroResult {
        // SAFETY: skb ownership transferred to GRO engine.
        let result = unsafe {
            bindings::napi_gro_receive(self.napi, skb.into_raw())
        };
        GroResult::from(result)
    }
}

impl Drop for NapiStruct {
    fn drop(&mut self) {
        // SAFETY: called during device cleanup, after napi_disable().
        unsafe { bindings::netif_napi_del(self.napi) }
    }
}

#[repr(i32)]
pub enum GroResult {
    Merged  = bindings::gro_result_GRO_MERGED as i32,
    MergedFree = bindings::gro_result_GRO_MERGED_FREE as i32,
    Held    = bindings::gro_result_GRO_HELD as i32,
    Normal  = bindings::gro_result_GRO_NORMAL as i32,
    Drop    = bindings::gro_result_GRO_DROP as i32,
}
```

### Complete Rust Loopback Driver

```rust
// SPDX-License-Identifier: GPL-2.0
//! Rust NAPI demonstration driver.
//! Mirrors the C demo_napi.c functionality using R4L abstractions.

use kernel::{
    bindings,
    net::{
        device::{self, Device, NetdevTx},
        skb::SkBuff,
    },
    prelude::*,
    sync::{Arc, Mutex, SpinLock},
};

module! {
    type: RustNapiDriver,
    name: "rust_demo_napi",
    author: "Demo",
    description: "Rust NAPI driver demonstration",
    license: "GPL v2",
}

// ────────────────────────────────────────────────────────────────
// RX Ring
// ────────────────────────────────────────────────────────────────

const RX_RING_SIZE: usize = 256;

/// Simulated hardware RX descriptor
struct RxDesc {
    done:   bool,
    length: usize,
    data:   [u8; 1600],
}

impl Default for RxDesc {
    fn default() -> Self {
        Self { done: false, length: 0, data: [0u8; 1600] }
    }
}

struct RxRing {
    descs: Vec<RxDesc>,
    head:  usize,
}

impl RxRing {
    fn new() -> Result<Self> {
        let mut descs = Vec::try_with_capacity(RX_RING_SIZE)?;
        for _ in 0..RX_RING_SIZE {
            descs.try_push(RxDesc::default())?;
        }
        Ok(RxRing { descs, head: 0 })
    }

    fn inject(&mut self, payload: &[u8]) {
        let desc = &mut self.descs[self.head];
        if desc.done { return; } // ring full
        let len = payload.len().min(1600);
        desc.data[..len].copy_from_slice(&payload[..len]);
        desc.length = len;
        // Simulate DMA: set done flag (hardware does this)
        core::sync::atomic::fence(core::sync::atomic::Ordering::Release);
        desc.done = true;
    }

    fn next_ready(&self) -> Option<(usize, usize)> {
        let desc = &self.descs[self.head];
        if desc.done {
            Some((self.head, desc.length))
        } else {
            None
        }
    }

    fn consume(&mut self, idx: usize) -> &[u8] {
        let desc = &mut self.descs[idx];
        let len = desc.length;
        desc.done = false;
        self.head = (self.head + 1) % RX_RING_SIZE;
        &desc.data[..len]
    }
}

// ────────────────────────────────────────────────────────────────
// Device Private State
// ────────────────────────────────────────────────────────────────

struct DeviceStats {
    rx_packets: u64,
    rx_bytes:   u64,
    tx_packets: u64,
    tx_bytes:   u64,
    rx_dropped: u64,
}

/// Private state embedded in net_device via netdev_priv().
/// In R4L, this is typically registered with Box<dyn NetDevice>.
struct RustNapiPriv {
    rx_ring:    SpinLock<RxRing>,
    stats:      SpinLock<DeviceStats>,
    // In a complete implementation, napi_struct would be here.
    // R4L is still developing the full NAPI Rust abstraction.
}

// ────────────────────────────────────────────────────────────────
// NetDevice trait implementation
// ────────────────────────────────────────────────────────────────

impl device::Operations for RustNapiPriv {
    type Data = ();

    /// ndo_open: called when `ip link set dev up`
    fn open(dev: &Device<Self>) -> Result {
        // In a full implementation: napi_enable(), request_irq(), etc.
        pr_info!("rust_demo_napi: {} opened\n", dev.name());
        device::start_queue(dev);
        Ok(())
    }

    /// ndo_stop: called when `ip link set dev down`
    fn stop(dev: &Device<Self>) -> Result {
        device::stop_queue(dev);
        // napi_disable() would go here
        pr_info!("rust_demo_napi: {} stopped\n", dev.name());
        Ok(())
    }

    /// ndo_start_xmit: transmit a packet
    fn start_xmit(skb: SkBuff, dev: &Device<Self>) -> NetdevTx {
        let data = skb.data();
        let len = skb.len() as u64;

        // Update stats
        {
            let priv_data = device::priv_data(dev);
            let mut stats = priv_data.stats.lock();
            stats.tx_packets += 1;
            stats.tx_bytes += len;
        }

        // Loopback: re-inject packet as RX
        {
            let priv_data = device::priv_data(dev);
            let mut ring = priv_data.rx_ring.lock();
            ring.inject(data);
        }

        // In a real driver, we'd trigger the NAPI here via napi_schedule().
        // For the loopback demo, we skip the IRQ step and note that
        // a real implementation would call:
        //   napi.schedule_irqoff()
        //   from within the hardirq handler

        // skb is dropped here: Rust's ownership ensures kfree_skb() is called.
        drop(skb);
        NetdevTx::Ok
    }
}

// ────────────────────────────────────────────────────────────────
// The NAPI poll function — must be unsafe extern "C" for FFI
// ────────────────────────────────────────────────────────────────

/// NAPI poll callback.
///
/// This is the function registered with netif_napi_add().
/// It runs in softirq context.
///
/// # Safety
/// Called by the kernel with valid napi and budget. We reconstruct
/// the device private pointer from napi using container_of logic.
unsafe extern "C" fn rust_napi_poll(
    napi: *mut bindings::napi_struct,
    budget: i32,
) -> i32 {
    // SAFETY: napi is valid, embedded in our device private area.
    let budget = budget as usize;
    let mut work_done: usize = 0;

    // In a real R4L driver, we'd use container_of! macro to get priv.
    // Conceptual loop:
    //
    // while work_done < budget {
    //     match priv.rx_ring.lock().next_ready() {
    //         None => break,  // ring empty
    //         Some((idx, len)) => {
    //             // Allocate sk_buff using NAPI allocator
    //             let skb = napi_alloc_skb(napi, len as u32);
    //             if skb.is_null() {
    //                 priv.stats.lock().rx_dropped += 1;
    //             } else {
    //                 // Copy data, set metadata, deliver via GRO
    //                 let data = priv.rx_ring.lock().consume(idx);
    //                 skb_put_data(skb, data.as_ptr(), len as u32);
    //                 (*skb).protocol = eth_type_trans(skb, dev);
    //                 napi_gro_receive(napi, skb);
    //                 work_done += 1;
    //             }
    //         }
    //     }
    // }
    //
    // if work_done < budget {
    //     napi_complete_done(napi, work_done as i32);
    //     // re-enable IRQ
    // }

    work_done as i32
}

// ────────────────────────────────────────────────────────────────
// Module init / exit
// ────────────────────────────────────────────────────────────────

struct RustNapiDriver {
    _dev: Pin<Box<device::Registration<RustNapiPriv>>>,
}

impl kernel::Module for RustNapiDriver {
    fn init(_module: &'static ThisModule) -> Result<Self> {
        pr_info!("rust_demo_napi: initializing\n");

        let priv_data = RustNapiPriv {
            rx_ring: SpinLock::new(RxRing::new()?),
            stats: SpinLock::new(DeviceStats {
                rx_packets: 0,
                rx_bytes: 0,
                tx_packets: 0,
                tx_bytes: 0,
                rx_dropped: 0,
            }),
        };

        // Register net_device
        let registration = device::Registration::try_new_ethernet::<RustNapiPriv>(
            c_str!("rstnapi%d"),
            priv_data,
            &THIS_MODULE,
        )?;

        // In a complete implementation:
        //   netif_napi_add(dev, &priv.napi, rust_napi_poll, NAPI_POLL_WEIGHT);

        Ok(RustNapiDriver {
            _dev: registration,
        })
    }
}

impl Drop for RustNapiDriver {
    fn drop(&mut self) {
        pr_info!("rust_demo_napi: exiting\n");
        // Registration Drop impl calls unregister_netdev() automatically
        // netif_napi_del() would be called during cleanup
    }
}
```

### Rust Memory Safety Benefits for NAPI Drivers

```rust
/*
 * Key safety improvements Rust brings to NAPI drivers:
 *
 * 1. LIFETIME TRACKING OF SKBs:
 *
 *    In C:                         In Rust:
 *    struct sk_buff *skb = ...;    let skb = napi_alloc_skb(...);
 *    napi_gro_receive(napi, skb);  napi_gro_receive(napi, skb);
 *    // skb is now owned by GRO   // compiler enforces: skb moved,
 *    kfree_skb(skb);  // BUG!     // cannot use after move
 *
 * 2. RING BUFFER ACCESS:
 *
 *    In C: manual index arithmetic, easy off-by-one, no bounds check
 *    In Rust: Vec/slice access with panic on out-of-bounds in debug,
 *             wrapping arithmetic checked by clippy
 *
 * 3. SPINLOCK DISCIPLINE:
 *
 *    In C: easy to forget unlock, hard to detect across branches
 *    In Rust: SpinLock<T> = MutexGuard automatically unlocks on drop
 *
 *    let _guard = priv.rx_ring.lock(); // auto-unlocks when _guard drops
 *
 * 4. DMA BUFFER SAFETY:
 *
 *    In C: possible to free DMA buffer while NIC is still using it
 *    In Rust: DmaMappedBuffer type ensures unmap before free via Drop
 *
 * 5. NULL POINTER:
 *    Rust Option<NonNull<T>> makes null impossible to dereference silently.
 */
```

---

## 16. XDP Integration with NAPI

XDP (eXpress Data Path) is the fastest packet processing path in Linux. It integrates directly into NAPI at the point just before `sk_buff` allocation.

### XDP Hook Position in NAPI

```
NAPI poll() execution flow with XDP:

  Driver reads descriptor from NIC ring
         │
         │ (DMA data is in page, not yet sk_buff)
         ▼
  ┌─────────────────────────────────────────────────────┐
  │  XDP HOOK  (runs here, before any skb allocation)   │
  │                                                     │
  │  struct xdp_buff xdp = {                            │
  │      .data     = page_addr + headroom,              │
  │      .data_end = page_addr + headroom + pkt_len,    │
  │      .data_meta = page_addr + headroom,             │
  │      .rxq     = &rx_queue->xdp_rxq,                 │
  │  };                                                 │
  │                                                     │
  │  act = bpf_prog_run_xdp(prog, &xdp);                │
  │                                                     │
  │  switch (act) {                                     │
  │    XDP_PASS   → continue to sk_buff allocation      │
  │    XDP_DROP   → free page, no further processing    │
  │    XDP_TX     → transmit back on same interface     │
  │    XDP_REDIRECT → redirect to another iface/CPU     │
  │    XDP_ABORTED → drop + trace                       │
  │  }                                                  │
  └─────────────────────────────────────────────────────┘
         │ (XDP_PASS only)
         ▼
  sk_buff allocation (napi_build_skb or napi_alloc_skb)
         │
         ▼
  napi_gro_receive(napi, skb) → normal stack
```

### XDP Driver Implementation

```c
/*
 * To support XDP in a NAPI driver, add to poll():
 */
static int my_napi_poll_with_xdp(struct napi_struct *napi, int budget)
{
    struct my_priv *priv = container_of(napi, struct my_priv, napi);
    struct bpf_prog *xdp_prog;
    int work_done = 0;

    /*
     * Read XDP program under RCU.
     * The program can be changed at runtime via ip link set xdpdrv.
     * We hold RCU read lock across the entire poll to ensure
     * the program isn't freed while we're using it.
     */
    rcu_read_lock();
    xdp_prog = READ_ONCE(priv->xdp_prog);

    while (work_done < budget) {
        struct my_desc *desc = &priv->rx.descs[priv->rx.head];
        struct xdp_buff xdp;
        struct sk_buff *skb;
        u32 act;

        if (!(desc->status & DESC_DONE))
            break;

        /*
         * Build xdp_buff pointing directly at the DMA page.
         * NO sk_buff allocation yet — this is the key to XDP's speed.
         *
         * xdp_prepare_buff() initializes xdp_buff fields:
         */
        xdp_prepare_buff(&xdp, page_address(priv->rx.pages[priv->rx.head]),
                         priv->rx.page_offset,   /* headroom */
                         desc->length,           /* packet length */
                         false);                 /* no frags */

        if (xdp_prog) {
            act = bpf_prog_run_xdp(xdp_prog, &xdp);

            switch (act) {
            case XDP_PASS:
                /* Fall through to sk_buff allocation */
                break;

            case XDP_DROP:
                /*
                 * Drop: just free the page and move on.
                 * No skb allocated, no stack overhead.
                 * This is how XDP achieves line-rate dropping:
                 * only memory barrier + descriptor update.
                 */
                priv->stats.xdp_dropped++;
                goto recycle;

            case XDP_TX:
                /*
                 * Transmit back on the same interface.
                 * The driver calls my_xdp_xmit() which writes
                 * the page to a TX descriptor.
                 */
                if (my_xdp_xmit(priv, &xdp) < 0) {
                    priv->stats.xdp_tx_fail++;
                    goto recycle;
                }
                priv->stats.xdp_tx++;
                work_done++;
                goto next_desc;

            case XDP_REDIRECT:
                /*
                 * Redirect to another interface or CPU queue.
                 * xdp_do_redirect() handles the logic.
                 * Pages are consumed by the redirect target.
                 */
                if (xdp_do_redirect(priv->dev, &xdp, xdp_prog) < 0) {
                    priv->stats.xdp_redirect_fail++;
                    goto recycle;
                }
                priv->stats.xdp_redirect++;
                work_done++;
                goto next_desc;

            default:
                bpf_warn_invalid_xdp_action(priv->dev, xdp_prog, act);
                fallthrough;
            case XDP_ABORTED:
                trace_xdp_exception(priv->dev, xdp_prog, act);
                goto recycle;
            }
        }

        /*
         * XDP_PASS: build sk_buff from the XDP buffer.
         * napi_build_skb() uses the same page, zero-copy.
         */
        skb = napi_build_skb(xdp.data_hard_start,
                             priv->rx.page_size);
        if (!skb) {
            priv->stats.rx_alloc_fail++;
            goto recycle;
        }

        /* Set data/tail pointers based on xdp_buff */
        skb_reserve(skb, xdp.data - xdp.data_hard_start);
        __skb_put(skb, xdp.data_end - xdp.data);

        skb->dev = priv->dev;
        skb->protocol = eth_type_trans(skb, priv->dev);
        skb->ip_summed = CHECKSUM_UNNECESSARY;

        napi_gro_receive(napi, skb);
        work_done++;
        goto next_desc;

recycle:
        /* Return page to page pool for reuse */
        page_pool_recycle_direct(priv->rx.pool, priv->rx.pages[priv->rx.head]);
next_desc:
        /* Refill descriptor with new page from page pool */
        my_refill_rx_desc(priv, priv->rx.head);
        priv->rx.head = (priv->rx.head + 1) % RX_RING_SIZE;
    }

    /* Flush XDP redirect queue */
    xdp_do_flush();

    rcu_read_unlock();

    if (work_done < budget) {
        if (napi_complete_done(napi, work_done))
            my_enable_rx_irq(priv);
    }

    return work_done;
}
```

### XDP Performance Numbers (Context)

```
Benchmark: packet drop rate (raw numbers for mental model)

  Method                    Mpps (million packets/sec, per core)
  ─────────────────────────────────────────────────────────────
  iptables DROP              ~2-5 Mpps
  nftables DROP              ~3-6 Mpps
  tc filter DROP             ~5-8 Mpps
  XDP_DROP (skb mode)        ~10-15 Mpps
  XDP_DROP (driver mode)     ~20-40 Mpps   ← in NAPI, pre-skb
  XDP_DROP (offload mode)    Line rate      ← in NIC hardware

  XDP in driver mode is 4-10× faster than iptables because:
  1. Runs in NAPI before sk_buff allocation (~200 bytes avoided)
  2. No netfilter hook traversal
  3. No route lookup
  4. BPF JIT: near-native code execution
```

---

## 17. Busy Polling — `SO_BUSY_POLL` and Low-Latency Path

### The Latency Problem with Normal NAPI

```
Normal NAPI receive latency:

  Packet arrives at NIC
        │
        │ ~1-10μs (NIC DMA + interrupt propagation)
        ▼
  IRQ fires → handler → napi_schedule()
        │
        │ ~5-50μs (waiting for softirq to run)
        │ (softirq runs when current process voluntarily yields
        │  or is preempted, or when IRQ returns to user space)
        ▼
  NET_RX_SOFTIRQ → napi_poll() → packet delivered
        │
        │ ~1-5μs (scheduler wakes sleeping recv())
        ▼
  Application's recv() returns

  Total: 7μs - 65μs typical
  With interrupt coalescing: can be 100-500μs

For latency-sensitive applications (HFT, HPC, real-time):
  This is unacceptable. They need sub-10μs end-to-end.
```

### SO_BUSY_POLL Mechanism

Busy polling allows the application to directly drive NAPI from its own thread, bypassing the interrupt and softirq machinery entirely:

```
Busy Polling Path:

  Application calls epoll_wait() / poll() / select()
        │
        │ epoll_wait with EPOLLERR | napi_id attached
        │ or: setsockopt(fd, SOL_SOCKET, SO_BUSY_POLL, 50)
        ▼
  Kernel enters sk_busy_loop():
        │
        │  while (timeout not elapsed) {
        │      // Directly call NAPI poll function
        │      napi_busy_loop(sk->sk_napi_id, ...)
        │         │
        │         └─► n->poll(n, BUSY_POLL_BUDGET)
        │  }
        │
        ▼
  If packet received: return immediately to application
  If timeout: fall back to normal blocking wait

  Latency: 1-5μs (CPU spin, no context switch, no softirq wait)
```

### Configuration

```c
/* Option 1: Per-socket busy polling */
int timeout_us = 50;  /* poll for up to 50 microseconds */
setsockopt(fd, SOL_SOCKET, SO_BUSY_POLL, &timeout_us, sizeof(timeout_us));

/* Option 2: System-wide default */
// sysctl net.core.busy_poll = 50   (microseconds, 0 = disabled)
// sysctl net.core.busy_read = 50   (for recv() calls)

/* Option 3: Prefer busy poll (for epoll) */
// SO_PREFER_BUSY_POLL: hint that this socket wants busy polling
int val = 1;
setsockopt(fd, SOL_SOCKET, SO_PREFER_BUSY_POLL, &val, sizeof(val));
// Then set NAPI budget per socket:
// SO_BUSY_POLL_BUDGET = number of packets to process per poll
```

### NAPI ID Tracking

```c
/*
 * When a packet arrives, the kernel records which NAPI instance
 * processed it in the socket's sk_napi_id field:
 *
 * In napi_gro_receive():
 *   skb->napi_id = napi->napi_id
 *
 * In tcp_rcv_established() or udp_rcv():
 *   sk->sk_napi_id = skb->napi_id
 *
 * When application calls busy_poll:
 *   napi_busy_loop(sk->sk_napi_id, ...)
 *   → finds napi_struct by ID
 *   → calls napi->poll() directly
 *
 * This creates a direct path:
 * Application CPU → specific NAPI instance → specific NIC queue
 *
 * For best results: pin application thread + NIC queue to same CPU.
 */

/* Check which NAPI ID is associated with a socket: */
// getsockopt(fd, SOL_SOCKET, SO_INCOMING_NAPI_ID, &id, &len);
```

### The napi_id Global Registry

```c
/*
 * The kernel maintains a global (per-CPU) hashtable:
 * napi_hash[]: maps napi_id → napi_struct *
 *
 * napi_id values:
 *   0: invalid
 *   1..NAPI_ID_HASH_BUCKETS: IDs for NAPI instances
 *   NAPI_ID_HASH_BUCKETS+1..: IDs for backlog NAPIs
 *
 * Fast lookup: napi_by_id(napi_id) → napi_struct *
 * Used by busy polling to find the right NAPI without scanning.
 */
```

---

## 18. Threaded NAPI Mode

Introduced in Linux 5.12. Allows NAPI poll to run in a dedicated kernel thread instead of softirq context.

### Why Threaded NAPI?

```
Problem with softirq-based NAPI:

1. REAL-TIME KERNELS (PREEMPT_RT):
   In RT kernels, softirqs are elevated to kernel threads.
   But the priority isn't controllable per-NAPI.
   NAPI poll running at the wrong priority can cause:
   - RT tasks starved by network receive processing
   - Or network not getting CPU time at needed rates

2. CPU ISOLATION:
   With nohz_full + isolcpus, softirqs on isolated CPUs
   prevent true isolation (ksoftirqd still runs).
   Threaded NAPI can be migrated off isolated CPUs.

3. PRIORITY CONTROL:
   Softirq: fixed priority, not schedulable.
   Thread: can set via chrt/nice, pinnable via taskset/cgroups.
   Allows: NAPI thread at SCHED_FIFO priority for latency,
           or SCHED_IDLE for background processing.

4. ACCOUNTING:
   Softirq CPU time attributed to interrupted process.
   Thread: accurately attributed to NAPI thread,
           visible in 'top' as [napi/0], [napi/eth0-0], etc.
```

### Threaded NAPI Architecture

```
Traditional NAPI:                  Threaded NAPI:
┌────────────────────┐            ┌────────────────────┐
│ IRQ Handler        │            │ IRQ Handler        │
│ napi_schedule()    │            │ napi_schedule()    │
└────────┬───────────┘            └────────┬───────────┘
         │                                 │
         ▼                                 ▼
┌────────────────────┐            ┌────────────────────┐
│ NET_RX_SOFTIRQ     │            │ napi->thread       │
│ (ksoftirqd/N or    │            │ [napi/0] kthread   │
│  returned from IRQ)│            │ Priority: RT or    │
│ Runs poll()        │            │  configurable      │
└────────────────────┘            │ Runs poll()        │
                                  │ Sleepable context! │
                                  └────────────────────┘
```

### Enabling and Using Threaded NAPI

```c
/*
 * Method 1: Per-device threaded NAPI
 */
/* In driver probe or sysfs: */
netdev_set_threaded(dev, true);
/* OR via sysfs after device creation: */
// echo 1 > /sys/class/net/eth0/threaded

/*
 * Method 2: Per-NAPI instance (kernel internal)
 */
/* In newer kernels, napi->thread is allocated by netif_napi_add()
 * if NAPI_STATE_THREADED is set. The thread runs napi_threaded_poll(). */

/*
 * napi_threaded_poll() — the thread function:
 */
static int napi_threaded_poll(void *data)
{
    struct napi_struct *napi = data;
    void *have;

    while (!napi_thread_wait(napi)) {
        /* napi_thread_wait(): uses wait_event() — SLEEPABLE! */
        /* This is the key difference from softirq: can sleep */

        for (;;) {
            bool repoll = false;

            have = netpoll_poll_lock(napi);
            __napi_poll(napi, &repoll);
            netpoll_poll_unlock(have);

            if (!repoll)
                break;

            /* Can yield between polls: RT-safe */
            cond_resched();
        }
    }
    return 0;
}

/*
 * Scheduling in threaded mode:
 * napi_schedule() still works the same from IRQ handler.
 * But instead of raising NET_RX_SOFTIRQ, it wakes the NAPI thread:
 *   __napi_schedule_irqoff() → wake_up_process(napi->thread)
 */

/*
 * In threaded NAPI, poll() CAN do things softirq cannot:
 *   - cond_resched() to yield CPU cooperatively
 *   - Technically: GFP_KERNEL allocations (page faults possible)
 *   - Wait for resources (carefully, to avoid starvation)
 *
 * Caution: spinlocks still can't sleep. mutex_lock() is possible
 * but discouraged in the tight poll loop.
 */
```

### Priority Tuning for Threaded NAPI

```bash
# List NAPI threads (visible after enabling threaded mode):
ps aux | grep napi
# Output: [napi/eth0-0], [napi/eth0-1], etc.

# Set real-time priority for low-latency networking:
PID=$(pgrep -f "napi/eth0-0")
chrt -f -p 50 $PID    # SCHED_FIFO priority 50

# Or set to idle for background:
chrt -i -p 0 $PID     # SCHED_IDLE

# Pin NAPI thread to specific CPU (matches IRQ affinity):
taskset -p 0x1 $PID   # CPU 0 only

# For RT kernel: prevents RT task starvation
chrt -f -p 99 $PID    # Highest priority: network first
```

---

## 19. AF_XDP and Zero-Copy with NAPI

AF_XDP (Address Family XDP) extends XDP to allow user-space applications to receive packets with true zero-copy, using shared memory queues between kernel NAPI and user space.

### Architecture

```
AF_XDP Zero-Copy Architecture:

  NIC Hardware RX Ring
  [desc0: page PA0] [desc1: page PA1] [desc2: page PA2] ...
                │
                │ DMA (NIC writes packet directly to page)
                ▼
  UMEM (User Memory Region):
  ┌─────────────────────────────────────────────────────────┐
  │User-space memory mmap'd into both kernel and user space │
  │ [Frame 0: 4096 bytes] [Frame 1: 4096 bytes] ...         │
  │ NIC DMA physically writes into these frames             │
  └─────────────────────────────────────────────────────────┘
                │
  ┌─────────────┴──────────────────────────────────────┐
  │              XSK (XDP Socket) Rings                │
  │                                                    │
  │  Fill Ring:    Kernel ← User  (user gives buffers) │
  │  [addr0][addr1][addr2]...                          │
  │                                                    │
  │  Completion Ring: Kernel → User (TX done)          │
  │  [addr0][addr1]...                                 │
  │                                                    │
  │  RX Ring:  Kernel → User (received packets)        │
  │  [addr + len][addr + len]...                       │
  │                                                    │
  │  TX Ring:  User → Kernel (packets to send)         │
  │  [addr + len][addr + len]...                       │
  └────────────────────────────────────────────────────┘
                │
  NAPI poll() with XDP_REDIRECT to AF_XDP socket:
  - xdp_prog receives packet → XDP_REDIRECT to xsk
  - xdp_do_redirect() puts frame address in RX ring
  - User-space polls RX ring with poll() or busy_loop
  - User reads packet directly from UMEM frame (zero-copy)
  - User puts frame address in Fill ring (recycle)
```

### AF_XDP User-Space Code

```c
/*
 * User-space AF_XDP receive loop (simplified).
 * Uses libxdp (or hand-rolled xsk_ring operations).
 */
#include <linux/if_xdp.h>
#include <sys/socket.h>

struct xsk_umem_info {
    struct xsk_ring_prod fq;  /* fill ring */
    struct xsk_ring_cons cq;  /* completion ring */
    struct xsk_umem    *umem;
    void               *buffer; /* mmap'd memory */
};

struct xsk_socket_info {
    struct xsk_ring_cons rx;  /* receive ring */
    struct xsk_ring_prod tx;  /* transmit ring */
    struct xsk_umem_info *umem;
    struct xsk_socket    *xsk;
};

void rx_loop(struct xsk_socket_info *xsk)
{
    uint32_t idx_rx = 0, idx_fq = 0;
    unsigned int rcvd;

    for (;;) {
        /*
         * Poll for new packets (or busy-spin: no poll() call).
         * xsk_ring_cons__peek() checks if RX ring has entries.
         *
         * Zero-copy: no data movement, just ring index updates.
         */
        rcvd = xsk_ring_cons__peek(&xsk->rx, BATCH_SIZE, &idx_rx);
        if (!rcvd) {
            /* Nothing received: poll() to sleep until data arrives */
            struct pollfd fds = {
                .fd     = xsk_socket__fd(xsk->xsk),
                .events = POLLIN,
            };
            poll(&fds, 1, -1);
            continue;
        }

        /* Process received packets */
        for (unsigned int i = 0; i < rcvd; i++) {
            const struct xdp_desc *desc =
                xsk_ring_cons__rx_desc(&xsk->rx, idx_rx++);

            /*
             * desc->addr: offset into UMEM where packet data sits.
             * desc->len:  packet length.
             *
             * TRUE ZERO-COPY: packet was written by NIC DMA
             * directly into this address. No copy ever happened.
             */
            uint8_t *pkt = xsk_umem__get_data(xsk->umem->buffer,
                                               desc->addr);

            /* Process packet at pkt[0..desc->len] */
            process_packet(pkt, desc->len);

            /* Recycle: give address back to fill ring */
            *xsk_ring_prod__fill_addr(&xsk->umem->fq, idx_fq++) =
                desc->addr;
        }

        /* Release consumed RX ring entries */
        xsk_ring_cons__release(&xsk->rx, rcvd);

        /* Submit recycled addresses to fill ring */
        xsk_ring_prod__submit(&xsk->umem->fq, rcvd);
    }
}
```

---

## 20. Performance Tuning — Complete Reference

### System-Level Tuning

```bash
# ─────────────────────────────────────────────────────────────
# NAPI / Softirq Tuning
# ─────────────────────────────────────────────────────────────

# Total packets per softirq invocation (default: 300)
# Increase for high-speed links; monitor time_squeeze in softnet_stat
sysctl -w net.core.netdev_budget=600

# Time limit per softirq run in microseconds (default: 8000)
sysctl -w net.core.netdev_budget_usecs=20000

# NAPI poll weight for new instances (default: 64)
sysctl -w net.core.dev_weight=128

# ─────────────────────────────────────────────────────────────
# IRQ Affinity — Pin NIC queues to CPUs
# ─────────────────────────────────────────────────────────────

# Show IRQ → CPU mapping:
cat /proc/irq/*/smp_affinity_list

# Pin IRQ 64 (NIC queue 0) to CPU 0:
echo 1 > /proc/irq/64/smp_affinity_list

# Pin IRQ 65 (NIC queue 1) to CPU 1:
echo 2 > /proc/irq/65/smp_affinity_list

# Automated: use irqbalance or set_irq_affinity scripts
# Many NIC vendors provide: set_irq_affinity_bynode.sh

# ─────────────────────────────────────────────────────────────
# NIC Queue Tuning (ethtool)
# ─────────────────────────────────────────────────────────────

# Show current queue configuration:
ethtool -l eth0

# Set queues = number of CPUs (or NUMA-local CPUs):
ethtool -L eth0 combined 16

# Show and set ring buffer sizes:
ethtool -g eth0
ethtool -G eth0 rx 4096 tx 4096   # larger ring = more buffering

# Interrupt coalescing: batching IRQs to reduce rate
# (complements NAPI, but adds latency):
ethtool -c eth0                   # show current
ethtool -C eth0 rx-usecs 50       # coalesce 50μs or...
ethtool -C eth0 rx-frames 64      # ...64 frames, whichever first
# adaptive: NIC auto-tunes:
ethtool -C eth0 adaptive-rx on

# GRO/LRO control:
ethtool -K eth0 gro on            # enable GRO (usually default on)
ethtool -K eth0 lro off           # disable hardware LRO (interferes with GRO)

# ─────────────────────────────────────────────────────────────
# Socket Buffer Tuning
# ─────────────────────────────────────────────────────────────

# Receive buffer size: affects socket backlog depth
sysctl -w net.core.rmem_max=134217728        # 128MB
sysctl -w net.core.rmem_default=16777216     # 16MB
sysctl -w net.ipv4.tcp_rmem="4096 87380 134217728"

# Backlog queue (for pre-accept connections):
sysctl -w net.core.netdev_max_backlog=5000

# ─────────────────────────────────────────────────────────────
# CPU Frequency and Power States
# ─────────────────────────────────────────────────────────────

# Disable CPU frequency scaling (prevents idle states):
for cpu in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
    echo performance > $cpu
done

# Disable C-states for consistent low latency:
# (add to kernel cmdline):
# intel_idle.max_cstate=1 processor.max_cstate=1

# ─────────────────────────────────────────────────────────────
# NUMA Optimization
# ─────────────────────────────────────────────────────────────

# Disable NUMA memory balancing (prevents page migration):
sysctl -w kernel.numa_balancing=0

# Verify NIC NUMA node:
cat /sys/class/net/eth0/device/numa_node

# Pin application to same NUMA node as NIC:
numactl --cpunodebind=0 --membind=0 ./my_server

# ─────────────────────────────────────────────────────────────
# Busy Polling (for latency-sensitive apps)
# ─────────────────────────────────────────────────────────────

# Global busy poll timeout (microseconds):
sysctl -w net.core.busy_poll=50
sysctl -w net.core.busy_read=50

# Per-socket (in application code):
# setsockopt(fd, SOL_SOCKET, SO_BUSY_POLL, &us, sizeof(us))
```

### Identifying Bottlenecks with Tuning

```
Performance Bottleneck Decision Tree:

Is time_squeeze high in /proc/net/softnet_stat?
  YES → budget too small OR CPU too slow for packet rate
        Actions:
          1. Increase netdev_budget (more packets per softirq)
          2. Increase netdev_budget_usecs
          3. Add more NIC queues + more CPUs
          4. Enable GRO to reduce packet count
          5. Use XDP to drop unwanted traffic before softirq

Are rx_dropped high?
  YES → input queue overflowing
        Actions:
          1. Increase netdev_max_backlog
          2. Increase per-queue ring size (ethtool -G rx 4096)
          3. Optimize application receive path (faster recv())
          4. Use RPS/RSS to distribute across CPUs

Is CPU utilization pinned to single core?
  YES → single-queue NIC or IRQ not distributed
        Actions:
          1. Enable multi-queue (ethtool -L combined N)
          2. Set IRQ affinity (irqbalance or manual)
          3. Enable RSS on NIC

Is latency high but throughput OK?
  YES → interrupt coalescing or softirq delay
        Actions:
          1. Reduce ethtool -C rx-usecs to 0
          2. Enable SO_BUSY_POLL
          3. Enable threaded NAPI with RT priority
          4. Disable adaptive interrupt coalescing
```

---

## 21. Monitoring, Debugging, and Observability

### `/proc/net/softnet_stat`

```bash
cat /proc/net/softnet_stat
# Each line = one CPU
# Format (hex columns):
# total  dropped  time_sq  0  0  0  0  0  0  cpu_collis  rps_recv  flow_lim_cnt

# Parse and display:
awk 'BEGIN {printf "CPU  Total      Dropped    TimeSqueeze  RPS\n"}
     { printf "%-4d %-10d %-10d %-12d  %d\n",
       NR-1, strtonum("0x"$1), strtonum("0x"$2),
             strtonum("0x"$3), strtonum("0x"$10) }' \
     /proc/net/softnet_stat
```

**Column meanings**:

| Column | Name | Meaning |
|--------|------|---------|
| 1 | `total_rx_packets` | Total packets received by softirq |
| 2 | `dropped` | Packets dropped (input queue full) |
| 3 | `time_squeeze` | softirq budget or time limit hit |
| 9 | `cpu_collision` | Hash collision in flow table |
| 10 | `received_rps` | Packets received via RPS |
| 11 | `flow_limit_count` | Flow limit activations |

### `ethtool -S` — NIC Statistics

```bash
# Full NIC statistics (driver-specific):
ethtool -S eth0 | head -50

# Key NAPI-related stats (Intel i40e example):
# rx_0_packets: packets received on queue 0
# rx_0_bytes:   bytes received on queue 0
# rx_0_dropped: packets dropped at queue 0
# rx_missed_errors: NIC overflow (hardware ring full)
# rx_no_buffer_count: descriptor ring exhausted

# Watch per-queue stats in real time:
watch -n1 "ethtool -S eth0 | grep -E 'rx_[0-9]+_packets'"

# Monitor queue distribution (uneven = RSS misconfigured):
ethtool -S eth0 | awk '/rx_[0-9]+_packets/ {print $1, $2}' | sort -t: -k2 -n
```

### `perf` Tracing for NAPI

```bash
# Profile net_rx_action and NAPI poll:
perf record -e 'net:*' -a -g sleep 5
perf report

# Specific NAPI events:
perf record -e 'net:napi_poll' -a sleep 5
perf report

# Flame graph of receive path:
perf record -F 999 -ag -- sleep 10
perf script | stackcollapse-perf.pl | flamegraph.pl > flame.svg

# Count function calls:
perf stat -e 'net:napi_poll,net:netif_receive_skb' -a sleep 1
```

### `ftrace` — Kernel Function Tracing

```bash
# Enable function tracing for NAPI:
cd /sys/kernel/debug/tracing

echo function > current_tracer
echo net_rx_action > set_ftrace_filter
echo 1 > tracing_on
sleep 1
echo 0 > tracing_on
cat trace

# Dynamic events on napi_poll:
echo 'p:napi_poll_entry net/core/dev.c:napi_poll napi=%ax budget=%dx' \
    > kprobe_events
echo 'r:napi_poll_exit net/core/dev.c:napi_poll ret=$retval' \
    >> kprobe_events
echo 1 > events/kprobes/enable
cat trace_pipe
```

### BPF/bpftrace for NAPI Instrumentation

```c
// trace_napi.bt — bpftrace script for NAPI analysis
//
// Usage: bpftrace trace_napi.bt

BEGIN {
    printf("Tracing NAPI polls... Hit Ctrl-C to end.\n");
}

// Probe net_rx_action to measure softirq duration:
kprobe:net_rx_action
{
    @start[tid] = nsecs;
}

kretprobe:net_rx_action
{
    if (@start[tid]) {
        @softirq_lat = hist(nsecs - @start[tid]);
        delete(@start[tid]);
    }
}

// Track per-NAPI poll work:
kprobe:napi_poll
{
    @napi_calls = count();
}

// Track budget exhaustion (time_squeeze):
kprobe:__raise_softirq_irqoff / arg0 == 3 /  // NET_RX_SOFTIRQ = 3
{
    @budget_exhausted = count();
}

// GRO flush frequency:
kprobe:napi_gro_flush
{
    @gro_flush = count();
}

END {
    printf("\nSoftirq duration histogram:\n"); print(@softirq_lat);
    printf("\nNAPI polls: %d\n", @napi_calls);
    printf("Budget exhaustion events: %d\n", @budget_exhausted);
    printf("GRO flushes: %d\n", @gro_flush);
}
```

### `ss` and Socket-Level NAPI Visibility

```bash
# Show socket's associated NAPI ID:
# (after setting SO_INCOMING_NAPI_ID)
ss -enp | grep napi_id

# Show socket receive queue depths:
ss -m

# Watch for receive buffer overflows:
netstat -s | grep -i "receive buffer errors\|receive errors"
# Or: ss --stats

# Monitor dropped packets system-wide:
watch -n1 "netstat -s | grep -E 'errors|dropped|overrun'"
```

---

## 22. Cloud Networking and NAPI

Understanding NAPI in cloud environments requires knowing which virtual and paravirtual NICs use it and how they differ from bare metal.

### virtio-net (QEMU/KVM, OpenStack)

```
virtio-net Architecture:

  Guest OS                         Host OS (QEMU/KVM)
  ─────────────────────────────────────────────────────
  virtio-net driver                vhost-net / kernel virtio-net backend
  ┌─────────────────┐              ┌────────────────────────────────────┐
  │ RX virtqueue:   │              │ vhost-net: kernel thread per queue │
  │ avail ring ────►│──────────────►─────────────────────────────────►  │
  │ used ring  ◄────│◄──────────────◄────────────────────────────────   │
  │                 │   virtqueue  │ TAP device or macvtap              │
  │ NAPI instance   │   (shared    │                                    │
  │ per RX queue    │   memory)    │ Host physical NIC                  │
  └─────────────────┘              └────────────────────────────────────┘

Guest virtio-net NAPI:
  - One NAPI instance per RX virtqueue
  - IRQ from host (kick) → guest IRQ handler → napi_schedule()
  - Guest NAPI poll: drains virtqueue used ring
    for each entry in used ring:
      - Get buffer from used_elem->id
      - Build sk_buff from buffer
      - napi_gro_receive()
  - Complete: re-fill avail ring with new buffers
              virtio_napi_complete_done()

Multi-queue virtio-net:
  ethtool -L eth0 combined 4  (up to num_vcpus queues)
  Each queue: separate MSI-X vector, separate NAPI, pinnable to vCPU
```

### AWS ENA (Elastic Network Adapter)

```
ENA is AWS's custom high-performance NIC for EC2 instances.

Key NAPI properties:
  - Up to 32 RX queues (instance-type dependent)
  - Each queue: separate MSI-X + NAPI instance
  - NAPI weight: NAPI_POLL_WEIGHT = 64 (ENA default)
  - Page pool for RX buffer management
  - LLQ (Low Latency Queue): TX uses WC (write-combining) BAR
    for fast doorbell writes without PCIe read-modify-write

ENA NAPI source: drivers/net/ethernet/amazon/ena/
  ena_netdev.c: ena_io_poll() — the NAPI poll function

ENA poll function pattern:
  static int ena_io_poll(struct napi_struct *napi, int budget)
  {
      struct ena_ring *tx_ring = ...;  // ENA uses combined TX+RX NAPI
      struct ena_ring *rx_ring = ...;
      int tx_work_done, rx_work_done;

      // TX completions first (free SKBs):
      tx_work_done = ena_clean_tx_irq(tx_ring, ENA_TX_BUDGET);

      // RX receives:
      rx_work_done = ena_clean_rx_irq(rx_ring, napi, budget);

      if (rx_work_done < budget) {
          napi_complete_done(napi, rx_work_done);
          ena_unmask_interrupt(tx_ring, rx_ring);  // re-enable IRQ
      }
      return rx_work_done;
  }

Cloud-specific consideration for ENA:
  - Interrupt moderation is done by ENA firmware (not just software NAPI)
  - ethtool -C adjusts ENA's built-in coalescing timer
  - ENA uses page pool for RX buffers: more efficient allocation
  - With SR-IOV: ENA VFs have same NAPI structure as PF
```

### Azure — MANA (Microsoft Azure Network Adapter) and MLX5

```
Azure Accelerated Networking uses two paths:

1. SR-IOV with MLX5 (Mellanox/NVIDIA):
   - Physical NIC is Mellanox ConnectX-5/6
   - Azure assigns VF (virtual function) to VM
   - Guest driver: mlx5_core (same as bare metal)
   - NAPI: identical to bare metal MLX5 NAPI
   - Key: RSS operates on real hardware
   - Up to 8 RX queues per VF (Azure policy limit)

2. MANA (newer Azure VMs):
   - drivers/net/ethernet/microsoft/mana/
   - Uses gdma_queue per CPU
   - NAPI weight: MANA_RX_BUDGET = 64
   - Each queue: separate completion queue event → NAPI schedule

Azure Accelerated Networking NAPI tuning:
  ethtool -L eth0 combined $(nproc)  # up to 8 for most VM sizes
  # D-series: up to 8 queues
  # E-series: up to 8 queues
  # HB-series (HPC): up to 32 queues (Infiniband path)
```

### GCP — gVNIC (Google Virtual NIC)

```
GCP gVNIC:
  - driver: drivers/net/ethernet/google/gve/
  - gve_main.c: gve_rx_poll() — NAPI poll function
  - Two architectures:
    a. GQI (GVE Queue Interface) — older, for non-nested VMs
    b. DQO (Descriptor Queue Ordering) — newer, higher performance

gVNIC DQO NAPI:
  - Page pool RX path
  - Hardware GRO offload (HW aggregates segments before NAPI)
  - Up to 16 RX queues
  - IRQ: MSI-X per queue

Key gVNIC NAPI config:
  sysctl -w net.core.dev_weight=64    # default works well
  ethtool -G eth0 rx 1024             # gVNIC supports up to 1024
  ethtool -C eth0 rx-usecs 0          # for lowest latency
```

### Cloud Considerations Summary

```
VM Type → NIC Driver → NAPI Considerations

  KVM/OpenStack → virtio-net     → standard NAPI, multi-queue
                                   page pool support added in 5.x
  AWS EC2       → ENA            → page pool, LLQ TX, up to 32 queues
  Azure         → MLX5 VF / MANA → MLX5: bare metal-like NAPI
  GCP           → gVNIC DQO      → HW GRO, page pool
  VMware        → vmxnet3        → 16 RX queues, standard NAPI
  Hyper-V       → hv_netvsc      → channels = queues, NAPI per channel

Cloud Networking Anti-Patterns:
  1. Single queue on multi-core VM: all traffic to CPU 0
     Fix: ethtool -L eth0 combined $(nproc)

  2. IRQ not distributed: all queues to CPU 0
     Fix: service irqbalance OR manual /proc/irq/*/smp_affinity

  3. Huge RX ring on small VM: wastes memory, same latency
     Tune: ethtool -G rx 256 for small VMs

  4. Enabled adaptive coalescing on latency-sensitive workload
     Fix: ethtool -C eth0 adaptive-rx off rx-usecs 50

  5. Not using NUMA-aware placement
     Fix: numactl, check /sys/class/net/eth0/device/numa_node
```

---

## 23. Kernel Source Navigation

The NAPI implementation spans several key files. Here's a guided map:

```
Linux kernel source tree — NAPI-relevant files:

include/linux/netdevice.h
  ├── struct napi_struct        (definition)
  ├── struct softnet_data       (per-CPU state)
  ├── struct net_device (partial) (napi_list, weight)
  ├── netif_napi_add()          (declaration)
  ├── napi_schedule()           (inline implementation)
  ├── napi_schedule_irqoff()    (inline)
  ├── napi_complete_done()      (declaration)
  └── NAPI_STATE_* constants

net/core/dev.c
  ├── net_rx_action()           (NET_RX_SOFTIRQ handler)
  ├── napi_poll()               (calls driver poll())
  ├── napi_complete_done()      (MISSED flag logic)
  ├── __napi_schedule()         (adds to poll_list, raises softirq)
  ├── napi_enable()             
  ├── napi_disable()            (wait-for-poll logic)
  ├── netif_napi_add()          (registration)
  ├── netif_napi_del()          (unregistration)
  ├── netif_receive_skb()       (protocol dispatch)
  └── napi_threaded_poll()      (threaded NAPI thread func)

net/core/gro.c (Linux 5.x+, was in dev.c)
  ├── napi_gro_receive()        
  ├── dev_gro_receive()         (protocol GRO dispatch)
  ├── napi_gro_flush()          
  └── skb_gro_receive()         (SKB merging)

net/core/skbuff.c
  ├── napi_alloc_skb()          
  ├── napi_build_skb()          
  └── napi_consume_skb()        

net/core/filter.c + kernel/bpf/
  └── XDP hooks: bpf_prog_run_xdp()

net/ipv4/gro.c
  └── tcp4_gro_receive()        (TCP/IPv4 GRO merge logic)

drivers/net/ethernet/intel/igb/   (Good reference driver)
  ├── igb_main.c: igb_poll()    (igb NAPI poll)
  └── igb_intr(): napi_schedule_irqoff()

drivers/net/ethernet/intel/i40e/  (Multi-queue reference)
  ├── i40e_main.c: i40e_napi_poll()
  └── MSI-X per-queue setup

drivers/net/ethernet/amazon/ena/  (Cloud reference)
  └── ena_netdev.c: ena_io_poll()

drivers/net/virtio_net.c          (VM reference)
  └── virtnet_poll(): virtio NAPI

drivers/net/ethernet/google/gve/  (GCP gVNIC)
  └── gve_main.c: gve_rx_poll()
```

### Reading the Code — Key Patterns to Spot

```c
/* Pattern 1: NAPI registration in probe() */
netif_napi_add(dev, &priv->napi[i], driver_poll, NAPI_POLL_WEIGHT);

/* Pattern 2: IRQ handler schedules NAPI */
irqreturn_t handler(int irq, void *data) {
    disable_hw_irq();
    napi_schedule_irqoff(&priv->napi);
    return IRQ_HANDLED;
}

/* Pattern 3: poll() returns work_done */
int driver_poll(struct napi_struct *napi, int budget) {
    int work = 0;
    while (work < budget && packet_ready()) {
        process_packet();
        work++;
    }
    if (work < budget)
        if (napi_complete_done(napi, work))
            enable_hw_irq();
    return work;
}

/* Pattern 4: Correct ndo_open/ndo_stop */
int open(struct net_device *dev) {
    napi_enable(&priv->napi);
    request_irq(...);
    return 0;
}
int stop(struct net_device *dev) {
    disable_irq(priv->irq);    /* must come before napi_disable */
    napi_disable(&priv->napi);
    free_irq(priv->irq, priv);
    return 0;
}
```

---

## 24. Mental Models — Thinking Efficiently About NAPI

### Model 1: The "Doorbell" Model

Think of NAPI as a two-phase system:

```
Phase 1: The Doorbell (Hardware Interrupt)
  Hardware rings the doorbell ONCE to say "I have mail."
  The kernel answers the door (IRQ handler), takes note,
  and tells the postman "I'll pick it up myself — stop ringing."

Phase 2: The Pickup (NAPI Poll)
  The kernel goes to the mailbox (RX ring) and empties it
  completely (up to budget). When the mailbox is empty,
  it tells the postman "OK, ring again if more mail arrives."

Key insight: You separate "notification" (IRQ) from "retrieval" (poll).
One notification → batch retrieval → re-enable notification.
This is the same pattern as:
  - Event loops (libuv, epoll)
  - Message queue draining (Kafka consumer)
  - OS scheduler quantum
```

### Model 2: The Budget Governor

```
Think of net_rx_action as a CPU time governor:

  Budget (300 packets) = how much networking work you can do in one run.
  Time limit (8ms)     = hard wall clock limit.

  If you hit budget: "You've had enough time, let other CPUs and processes run.
                      Come back in the next softirq cycle."

  If you finish early: "Great, you're done. Re-arm the hardware.
                        No wasted CPU cycles."

  This is fair scheduling for the network subsystem.
  Compare to: CFS scheduler quantum, GC pauses, task budgets.
```

### Model 3: The State Machine Invariants

```
Always know: at any moment, a NAPI is in exactly ONE state.
The state machine guarantees:

1. IDLE → only one thread can SCHED it (test_and_set_bit atomicity)
2. SCHEDULED → only one CPU polls it at a time (NAPI poll lock)
3. POLLING → if new packet arrives, MISSED is set (not double-scheduled)
4. COMPLETE → MISSED check prevents losing the "last packet" race

If you're debugging a driver and packets are lost:
  - Check: is napi_complete_done() called correctly?
  - Check: is IRQ re-enabled in napi_complete_done()?
  - Check: is MISSED handled? (it is, automatically)

If CPU is spinning at 100%:
  - Check: is poll() returning == budget every time?
  - Check: is the RX ring actually draining?
  - Check: budget too small for packet rate?
```

### Model 4: Data Flow as a Pipeline

```
Think of the receive path as a production pipeline:

NIC Hardware      →   NAPI Poll     →   GRO Engine  →   Protocol Stack
(raw frames)          (batch fetch)     (aggregation)    (IP/TCP)
                                                          ↓
                                                      Application

Each stage has:
  Throughput:   limited by hardware rate / budget / GRO ratio / CPU
  Latency:      IRQ delay + softirq delay + GRO hold time + scheduler
  Buffering:    NIC ring / poll_list / GRO hash table / socket recv buffer

Optimize throughput: increase batch sizes, enable GRO, multi-queue
Optimize latency:    reduce batch sizes, disable GRO for RT, busy polling
```

### Model 5: NAPI as an Abstraction Layer

```
NAPI hides the difference between:

  Hardware polling mode NICs (no interrupts at all):
    → poll() is called periodically by kernel timer
    → Never goes through interrupt path

  Interrupt-driven NICs (traditional):
    → interrupt → disable → schedule → poll → re-enable

  Mixed mode (modern):
    → IRQ for first packet → polling for burst → re-arm when quiet

From the driver writer's perspective:
  Always implement poll() the same way.
  NAPI handles the interrupt/poll mode transition.
  This is the power of the abstraction.
```

### Model 6: Thinking in Packet Rates

```
Always convert bandwidth to packets/second:

  1 Gbps  (64B frames):  ~1.5 Mpps
  10 Gbps (64B frames):  ~15 Mpps
  25 Gbps (64B frames):  ~37 Mpps
  100 Gbps (64B frames): ~149 Mpps

Budget = 300 packets/softirq, 10ms budget window
Max theoretical: 300 / 0.00001s = 30 Mpps per core

  → At 10 Gbps: budget comfortably handles one core.
  → At 25 Gbps: need tuning or multi-queue (multiple cores).
  → At 100 Gbps: need 4-8 cores minimum, even with XDP.

For realistic (mixed frame sizes ~500 bytes):
  Each Gbps ≈ 250Kpps
  budget=300 per 8ms = 37.5Kpps per core
  → One core handles ~150Mbps of realistic traffic

Reality check: XDP_DROP can do 40Mpps per core.
Normal stack (NAPI + GRO + TCP): ~3-5 Mpps per core practical limit.
```

---

## Summary: NAPI's Core Design Principles

| Principle | Mechanism | Why |
|-----------|-----------|-----|
| Interrupt coalescing | Disable IRQ during poll | Prevent interrupt storm |
| Batch processing | Poll loop with budget | Amortize per-packet overhead |
| Cache efficiency | Per-CPU softnet_data, no locks | Avoid false sharing |
| Fairness | Budget system, time limit | Prevent starvation |
| Adaptability | IRQ when quiet, poll when busy | Minimize latency AND CPU |
| Extensibility | XDP hook, GRO, busy poll | Multiple fast paths |
| Driver abstraction | poll() callback, napi_gro_receive() | Uniform driver interface |

---

*References:*
*- Linux kernel source: `net/core/dev.c`, `net/core/gro.c`, `include/linux/netdevice.h`*
*- NAPI LWN article series: https://lwn.net/Articles/30107/*
*- Rust for Linux networking: https://rust-for-linux.com*
*- XDP documentation: https://www.kernel.org/doc/html/latest/networking/af_xdp.html*
*- NIC vendor docs: Intel i40e, AWS ENA, Google gVNIC, Microsoft MANA*
