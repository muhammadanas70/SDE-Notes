# Linux Kernel: End-to-End Packet Processing Pipeline
## From NIC Hardware Interrupt to Upper Layer Protocol — Code Level + Kernel Concept Level

> **Context:** This is a follow-up to the ICMP Development notes. That document ended with a
> guiding question: *"Name the exact call chain from the driver's interrupt handler to
> `icmp_rcv()`."* This document answers that question in full — for every layer, with code,
> with memory mechanics, and with the reasoning behind every design decision.

---

## Table of Contents

1. [The Big Picture: Five Stages](#1-the-big-picture-five-stages)
2. [Stage 1 — Hardware: NIC DMA and the Interrupt](#2-stage-1--hardware-nic-dma-and-the-interrupt)
3. [Stage 2 — Driver Layer: `sk_buff` Birth and NAPI](#3-stage-2--driver-layer-sk_buff-birth-and-napi)
4. [Stage 3 — Network Core: `netif_receive_skb` and Protocol Dispatch](#4-stage-3--network-core-netif_receive_skb-and-protocol-dispatch)
5. [Stage 4 — IP Layer: `ip_rcv` to `ip_local_deliver_finish`](#5-stage-4--ip-layer-ip_rcv-to-ip_local_deliver_finish)
6. [Stage 5 — Transport/ICMP Layer: Protocol Handler Execution](#6-stage-5--transporticmp-layer-protocol-handler-execution)
7. [Pointer Mechanics: How Data Is Read Without Copying](#7-pointer-mechanics-how-data-is-read-without-copying)
8. [Memory Lifecycle: Who Allocates, Who Frees](#8-memory-lifecycle-who-allocates-who-frees)
9. [The Full Call Chain in One View](#9-the-full-call-chain-in-one-view)
10. [Annotated Code Walkthrough](#10-annotated-code-walkthrough)

---

## 1. The Big Picture: Five Stages

A packet arriving at your NIC travels through five distinct stages before any user-space
application sees it. Each stage has a clear owner, a clear mechanism, and a clear handoff point.

```
 ┌──────────────────────────────────────────────────────────────────────┐
 │  HARDWARE                                                            │
 │  NIC receives frame → DMA into kernel RAM → fires hardware interrupt │
 └───────────────────────────────┬──────────────────────────────────────┘
                                 │ IRQ
                                 ▼
 ┌──────────────────────────────────────────────────────────────────────┐
 │  DRIVER LAYER  (e.g., e1000, igb, ixgbe)                            │
 │  ISR disables interrupts → schedules NAPI poll → poll() reads DMA   │
 │  ring → allocates sk_buff → wraps DMA memory → calls               │
 │  netif_receive_skb()                                                 │
 └───────────────────────────────┬──────────────────────────────────────┘
                                 │ sk_buff *
                                 ▼
 ┌──────────────────────────────────────────────────────────────────────┐
 │  NETWORK CORE  (net/core/dev.c)                                      │
 │  __netif_receive_skb() → deliver to packet_type handlers            │
 │  Ethernet type field → which L3 protocol?                            │
 └───────────────────────────────┬──────────────────────────────────────┘
                                 │ ethertype == 0x0800 (IPv4)
                                 ▼
 ┌──────────────────────────────────────────────────────────────────────┐
 │  IP LAYER  (net/ipv4/ip_input.c)                                     │
 │  ip_rcv() → validate → ip_rcv_finish() → routing decision           │
 │  ip_local_deliver() → ip_local_deliver_finish()                     │
 │  protocol field lookup → dispatch to registered handler             │
 └───────────────────────────────┬──────────────────────────────────────┘
                                 │ protocol == 1 (ICMP)
                                 ▼
 ┌──────────────────────────────────────────────────────────────────────┐
 │  PROTOCOL LAYER  (net/ipv4/icmp.c / tcp_input.c / udp.c ...)        │
 │  icmp_rcv() → parse ICMP header → dispatch by type                  │
 │  icmp_echo() → build reply → kfree_skb() on original               │
 └──────────────────────────────────────────────────────────────────────┘
```

The critical insight before reading further: **the packet data is never copied** between
these stages. Every layer manipulates pointers into the same buffer the NIC DMA'd into RAM.
This is the zero-copy architecture of the kernel network stack.

---

## 2. Stage 1 — Hardware: NIC DMA and the Interrupt

### What the NIC Does (No Kernel Code Yet)

Modern NICs do not wait for the CPU to come read a packet. They use **Direct Memory Access
(DMA)**: they write the incoming frame bytes directly into a region of kernel RAM that the
driver set up in advance, without CPU involvement.

The driver, during initialization, creates a **ring buffer** — a circular array of
descriptors. Each descriptor holds the physical address of a pre-allocated memory region
(a DMA buffer). When the NIC receives a frame it picks the next available descriptor,
copies the frame data into that buffer via DMA, marks the descriptor "used", and then
fires a hardware interrupt.

```
 NIC DMA Ring (set up by driver at init time):
 ┌─────────────────────────────────────────────────────────┐
 │  desc[0]: phys_addr=0xABC000, len=1518, status=DONE    │ ← packet arrived here
 │  desc[1]: phys_addr=0xABC600, len=0,    status=EMPTY   │
 │  desc[2]: phys_addr=0xABCC00, len=0,    status=EMPTY   │
 │  ...                                                    │
 └─────────────────────────────────────────────────────────┘
```

The physical address `0xABC000` points to kernel RAM that already has the raw Ethernet
frame bytes. The interrupt is just a signal: *"Go look at the ring, there is work."*

### Relevant Kernel Reference

The ring buffer setup is visible in drivers like `drivers/net/ethernet/intel/igb/igb_main.c`
in `igb_setup_rx_resources()`. The DMA API used is `dma_alloc_coherent()` from
`include/linux/dma-mapping.h`.

---

## 3. Stage 2 — Driver Layer: `sk_buff` Birth and NAPI

### The Interrupt Service Routine (ISR)

When the NIC fires its interrupt, the CPU stops whatever it is doing and jumps to the
driver's registered ISR. For the `igb` driver this is `igb_intr()`. But here is the
crucial design: **the ISR does almost nothing except schedule work and disable the
interrupt.**

```c
/* drivers/net/ethernet/intel/igb/igb_main.c  (simplified) */
static irqreturn_t igb_intr(int irq, void *data)
{
    struct igb_adapter *adapter = data;

    /* Tell the NIC: stop generating this interrupt for now */
    igb_irq_disable(adapter);

    /* Schedule the NAPI poll — actual work happens there */
    napi_schedule(&adapter->q_vector[0]->napi);

    return IRQ_HANDLED;
}
```

Why disable the interrupt immediately? If the NIC fires thousands of interrupts per second
(a traffic flood), the CPU spends all its time context-switching into ISRs and does no real
work. This is called **interrupt liveness** or the receive livelock problem.

### NAPI: New API for Receive Processing

NAPI solves the livelock problem. Instead of one interrupt per packet, the ISR fires once,
disables further interrupts, and schedules a **poll function**. The poll function runs in
softirq context and processes as many packets as available, up to a budget (typically 64).
Only after the ring is drained does NAPI re-enable interrupts.

```c
/* Generic NAPI poll — driver implements .poll = igb_poll */
static int igb_poll(struct napi_struct *napi, int budget)
{
    struct igb_q_vector *q_vector =
        container_of(napi, struct igb_q_vector, napi);
    int work_done = 0;

    /* Process up to `budget` descriptors from the DMA ring */
    igb_clean_rx_irq(q_vector, budget, &work_done);

    /* If we drained the ring, re-enable interrupts */
    if (work_done < budget) {
        napi_complete_done(napi, work_done);
        igb_ring_irq_enable(q_vector);
    }

    return work_done;
}
```

### How `sk_buff` Is Allocated and Filled

Inside `igb_clean_rx_irq()` → `igb_fetch_rx_buffer()`, the driver does the key work:

```c
/* Conceptual version — actual code spread across igb_main.c */

struct sk_buff *skb;
struct igb_rx_buffer *rx_buffer; /* points to pre-mapped DMA page */
union e1000_adv_rx_desc *rx_desc; /* descriptor from the ring */

/* 1. Get the DMA descriptor that the NIC filled */
rx_desc = IGB_RX_DESC(ring, ring->next_to_clean);

/* 2. Allocate the sk_buff shell */
skb = napi_alloc_skb(&q_vector->napi, IGB_RX_HDR_LEN);
/*
 * napi_alloc_skb() calls __alloc_skb() internally.
 * This gives us the sk_buff control structure and a small headroom
 * buffer. For larger packets, the actual data stays in the DMA page.
 */

/* 3. Map the DMA buffer into the skb's data area */
/*
 * For large packets the driver uses page fragments:
 * skb_add_rx_frag() attaches the DMA page directly to the skb
 * without copying. The page is reference-counted.
 */
skb_add_rx_frag(skb,
                skb_shinfo(skb)->nr_frags,
                rx_buffer->page,
                rx_buffer->page_offset,
                size,
                truesize);

/* 4. Set the device and determine L3 protocol from the Ethernet header */
skb->dev = rx_ring->netdev;
skb->protocol = eth_type_trans(skb, rx_ring->netdev);
/*
 * eth_type_trans() does two things:
 *   a) Reads the 2-byte EtherType from the Ethernet header
 *   b) Calls skb_pull() to advance skb->data past the Ethernet header
 *      so the next layer sees the IP header at skb->data
 * It returns htons(ETH_P_IP) for IPv4, htons(ETH_P_IPV6) for IPv6, etc.
 */

/* 5. Hand off to the network core */
napi_gro_receive(&q_vector->napi, skb);
/* GRO = Generic Receive Offload: coalesces small TCP segments.
 * Eventually calls netif_receive_skb() for non-coalesced packets. */
```

**After this point the driver's job is done.** It owns nothing. The `sk_buff` is now in
the kernel network core's hands.

---

## 4. Stage 3 — Network Core: `netif_receive_skb` and Protocol Dispatch

### Source: `net/core/dev.c`

```c
int netif_receive_skb(struct sk_buff *skb)
{
    int ret;

    /* RPS (Receive Packet Steering): optionally redirect to
     * a specific CPU's queue for processing. On SMP systems
     * this avoids cache thrashing. */
    if (static_branch_unlikely(&rps_needed)) {
        struct rps_dev_flow voidflow, *rflow = &voidflow;
        int cpu = get_rps_cpu(skb->dev, skb, &rflow);
        if (cpu >= 0) {
            ret = enqueue_to_backlog(skb, cpu, &rflow->last_qtail);
            return ret;
        }
    }

    return __netif_receive_skb(skb);
}
```

### The Real Work: `__netif_receive_skb_core()`

```c
static int __netif_receive_skb_core(struct sk_buff **pskb,
                                    bool pfmemalloc,
                                    struct packet_type **ppt_prev)
{
    struct sk_buff *skb = *pskb;
    struct packet_type *ptype, *pt_prev;
    struct net_device *orig_dev;
    __be16 type;

    /* skb->protocol was set by eth_type_trans() in the driver.
     * This is the EtherType: 0x0800 for IPv4. */
    type = skb->protocol;

    /* Walk the list of registered packet handlers for this EtherType.
     * These are registered via dev_add_pack(). The IPv4 handler is
     * registered in net/ipv4/af_inet.c */
    list_for_each_entry_rcu(ptype, &ptype_base[ntohs(type) & 15], list) {
        if (ptype->type == type && ptype->dev == NULL) {
            /* Found the handler. But don't call it yet — defer
             * until we know there isn't a higher-priority sniffer. */
            if (pt_prev)
                ret = deliver_skb(skb, pt_prev, orig_dev);
            pt_prev = ptype;
        }
    }

    /* Call the last matched handler — for IPv4 this is ip_rcv() */
    if (pt_prev) {
        ret = pt_prev->func(skb, skb->dev, pt_prev, orig_dev);
    }
    /* ...snip error handling... */
}
```

### The `packet_type` Registration Mechanism

The IPv4 protocol registers itself at boot via `inet_init()` in `net/ipv4/af_inet.c`:

```c
/* net/ipv4/af_inet.c */
static struct packet_type ip_packet_type __read_mostly = {
    .type = cpu_to_be16(ETH_P_IP),   /* 0x0800 */
    .func = ip_rcv,                   /* ← this is called for every IPv4 pkt */
};

static int __init inet_init(void)
{
    /* ... */
    dev_add_pack(&ip_packet_type);   /* registers in ptype_base hash table */
    /* ... */
}
```

The `ptype_base` is a hash table indexed by EtherType. When `__netif_receive_skb_core()`
walks the list for `ETH_P_IP`, it finds `ip_packet_type` and calls `ip_rcv()`.

**This is the L2→L3 handoff.** The caller is the network core. The callee is the IP layer.

---

## 5. Stage 4 — IP Layer: `ip_rcv` to `ip_local_deliver_finish`

### Source: `net/ipv4/ip_input.c`

```c
/*
 * ip_rcv - entry point for all incoming IPv4 packets.
 * Called by: __netif_receive_skb_core() via the packet_type dispatch.
 * Returns: NET_RX_SUCCESS or NET_RX_DROP
 */
int ip_rcv(struct sk_buff *skb, struct net_device *dev,
           struct packet_type *pt, struct net_device *orig_dev)
{
    struct net *net = dev_net(dev);

    /* NF_INET_PRE_ROUTING: netfilter hook point.
     * iptables/nftables rules run here before we even look at
     * the IP header. ip_rcv_finish() is the continuation callback. */
    return NF_HOOK(NFPROTO_IPV4, NF_INET_PRE_ROUTING,
                   net, NULL, skb, dev, NULL, ip_rcv_finish);
}
```

### `ip_rcv_finish()` — After Netfilter

```c
static int ip_rcv_finish(struct net *net, struct sock *sk,
                         struct sk_buff *skb)
{
    const struct iphdr *iph = ip_hdr(skb);
    /*
     * ip_hdr(skb) is a macro:
     *   #define ip_hdr(skb)  ((struct iphdr *)skb_network_header(skb))
     * skb_network_header() returns skb->head + skb->network_header.
     * This is a direct pointer into the DMA buffer — no copy.
     */

    /* Make a routing decision: is this packet for us, or to be forwarded? */
    if (!skb_valid_dst(skb)) {
        int err = ip_route_input_noref(skb, iph->daddr, iph->saddr,
                                       iph->tos, skb->dev);
        if (err)
            goto drop;
    }

    /* skb->_skb_refdst now holds the routing table entry.
     * dst->input is either ip_local_deliver (for us) or
     * ip_forward (packet needs to be forwarded to another host). */
    return dst_input(skb);
    /* dst_input expands to: skb_dst(skb)->input(skb) */
}
```

### `ip_local_deliver()` — Packet Is For This Host

```c
int ip_local_deliver(struct sk_buff *skb)
{
    struct net *net = dev_net(skb->dev);

    /* Reassemble IP fragments if needed */
    if (ip_is_fragment(ip_hdr(skb))) {
        if (ip_defrag(net, skb, IP_DEFRAG_LOCAL_DELIVER))
            return 0;  /* more fragments pending, skb consumed */
    }

    /* NF_INET_LOCAL_IN: another netfilter hook.
     * Continuation is ip_local_deliver_finish(). */
    return NF_HOOK(NFPROTO_IPV4, NF_INET_LOCAL_IN,
                   net, NULL, skb, skb->dev, NULL,
                   ip_local_deliver_finish);
}
```

### `ip_local_deliver_finish()` — The L3→L4 Dispatch

This is where the protocol number in the IP header (`iph->protocol`) is used to find the
correct upper-layer handler. This is the exact same registration pattern you saw with ICMP
in the previous notes.

```c
static int ip_local_deliver_finish(struct net *net, struct sock *sk,
                                   struct sk_buff *skb)
{
    __skb_pull(skb, skb_network_header_len(skb));
    /*
     * __skb_pull advances skb->data past the IP header.
     * Now skb->data points at the start of the L4 payload
     * (the ICMP/TCP/UDP header).
     * Again: no copy. Pointer advancement only.
     */

    rcu_read_lock();
    {
        int protocol = ip_hdr(skb)->protocol; /* e.g., IPPROTO_ICMP = 1 */

        /* Look up the handler in the global inet_protos[] table.
         * This table is populated by inet_add_protocol(). */
        const struct net_protocol *ipprot =
            rcu_dereference(inet_protos[protocol]);

        if (ipprot) {
            int ret = ipprot->handler(skb);
            /* For ICMP: this calls icmp_rcv(skb) */
        } else {
            /* No handler registered — send ICMP "Protocol Unreachable" */
            icmp_send(skb, ICMP_DEST_UNREACH, ICMP_PROT_UNREACH, 0);
            kfree_skb(skb);
        }
    }
    rcu_read_unlock();
    return 0;
}
```

The `inet_protos[]` table is a fixed-size array (256 entries, one per protocol number).
ICMP registered itself in this table at boot:

```c
/* net/ipv4/icmp.c */
static const struct net_protocol icmp_protocol = {
    .handler     = icmp_rcv,
    .err_handler = icmp_err,
    .no_policy   = 1,
};

static int __init icmp_init(void)
{
    return inet_add_protocol(&icmp_protocol, IPPROTO_ICMP); /* slot 1 */
}
```

`inet_add_protocol()` in `net/ipv4/protocol.c` does:
```c
int inet_add_protocol(const struct net_protocol *prot, unsigned char protocol)
{
    return !cmpxchg((const struct net_protocol **)&inet_protos[protocol],
                    NULL, prot) ? 0 : -1;
}
```

A single atomic `cmpxchg` into `inet_protos[1]`. From this point forward, any packet with
`iph->protocol == 1` goes to `icmp_rcv()`.

---

## 6. Stage 5 — Transport/ICMP Layer: Protocol Handler Execution

### Source: `net/ipv4/icmp.c`

```c
/*
 * icmp_rcv - Entry point for all received ICMP packets.
 * Called by: ip_local_deliver_finish() via inet_protos[IPPROTO_ICMP].
 *
 * At this point:
 *   - skb->data points at the ICMP header (IP header has been pulled)
 *   - skb->network_header still records the IP header offset
 *   - The raw bytes in memory are untouched since the NIC DMA'd them
 */
static enum skb_drop_reason icmp_rcv(struct sk_buff *skb)
{
    struct icmphdr *icmph;
    struct rtable *rt;
    struct net *net;
    SKB_DR(reason);    /* drop reason tracking */

    /* Basic sanity: does the skb have enough bytes for an ICMP header? */
    if (!pskb_may_pull(skb, sizeof(struct icmphdr)))
        goto drop;

    /*
     * icmp_hdr(skb) is defined as:
     *   static inline struct icmphdr *icmp_hdr(const struct sk_buff *skb)
     *   { return (struct icmphdr *)skb_transport_header(skb); }
     *
     * skb_transport_header returns skb->head + skb->transport_header.
     * This is a CAST — the same bytes the NIC wrote, reinterpreted
     * as a struct icmphdr. Zero copy. Zero allocation.
     */
    icmph = icmp_hdr(skb);

    /* Verify checksum: RFC 792 requires checksum over ICMP header + data */
    if (skb->ip_summed != CHECKSUM_UNNECESSARY) {
        if (__skb_checksum_complete(skb)) {
            /* bad checksum — drop silently */
            SKB_DR_SET(reason, ICMP_CSUM);
            goto drop;
        }
    }

    net = dev_net(skb->dev);

    /*
     * Dispatch by ICMP type field.
     * icmph->type is a single byte, so no byte-order conversion needed.
     */
    switch (icmph->type) {
    case ICMP_ECHO:                     /* type 8: ping request */
        if (!net->ipv4.sysctl_icmp_echo_ignore_all)
            icmp_echo(skb);             /* build and send a reply */
        break;

    case ICMP_ECHOREPLY:                /* type 0: ping reply */
        ping_rcv(skb);
        break;

    case ICMP_DEST_UNREACH:
    case ICMP_TIME_EXCEEDED:
    case ICMP_PARAMETERPROB:
        /* These carry the offending IP header — forward to
         * the affected transport protocol's error handler */
        icmp_unreach(skb);
        break;

    case ICMP_REDIRECT:
        icmp_redirect(skb);
        break;

    /* ... other types ... */

    default:
        /* Unknown type — just consume the packet */
        break;
    }

    /* Whoever reaches here drops their reference */
    kfree_skb(skb);
    return 0;

drop:
    kfree_skb_reason(skb, reason);
    return 0;
}
```

### Building the Reply: `icmp_echo()` → `icmp_reply()`

```c
static bool icmp_echo(struct sk_buff *skb)
{
    struct icmp_bxm icmp_param;

    /* Build the reply parameters */
    icmp_param.data.icmph       = *icmp_hdr(skb);  /* copy header struct */
    icmp_param.data.icmph.type  = ICMP_ECHOREPLY;  /* flip type: 8 → 0 */
    icmp_param.data.icmph.code  = 0;
    icmp_param.skb              = skb;
    icmp_param.offset           = 0;
    icmp_param.data_len         = skb->len;
    icmp_param.head_len         = sizeof(struct icmphdr);

    icmp_reply(&icmp_param, skb);
    return true;
}

static void icmp_reply(struct icmp_bxm *icmp_param, struct sk_buff *skb)
{
    /* icmp_send() allocates a NEW sk_buff for the outgoing reply.
     * The incoming skb is not modified — it will be freed by icmp_rcv(). */
    struct ipcm_cookie ipc;
    struct rtable *rt = skb_rtable(skb);
    struct net *net   = dev_net(rt->dst.dev);
    struct sock *sk   = net->ipv4.icmp_sk; /* per-CPU ICMP socket */

    /* ... routing setup, source address selection ... */

    ip_send_skb(net, icmp_param->skb_out);
    /* ip_send_skb() → ip_local_out() → ip_output() → dev_queue_xmit() */
}
```

---

## 7. Pointer Mechanics: How Data Is Read Without Copying

This section explains the exact C mechanics behind the zero-copy architecture.

### The `sk_buff` Memory Layout

```
 Physical memory (one contiguous allocation from kmalloc or page allocator):
 ┌──────────────────────────────────────────────────────────────────────────────┐
 │  headroom │ Eth hdr (14B) │ IP hdr (20B) │ ICMP hdr (8B) │ payload │ tail  │
 └──────────────────────────────────────────────────────────────────────────────┘
 ▲                           ▲              ▲               ▲
 skb->head               skb->data      (after ip_pull) (after icmp_pull)
                         (set by driver
                          after eth_type_trans)
```

The fields in `struct sk_buff` that track these positions:

```c
struct sk_buff {
    unsigned char   *head;         /* start of allocated buffer */
    unsigned char   *data;         /* start of current payload (moves!) */
    sk_buff_data_t   tail;         /* end of current payload */
    sk_buff_data_t   end;          /* end of allocated buffer */

    sk_buff_data_t   mac_header;   /* offset from head to Ethernet header */
    sk_buff_data_t   network_header;   /* offset from head to IP header */
    sk_buff_data_t   transport_header; /* offset from head to L4 header */
};
```

`sk_buff_data_t` is `unsigned int` (an offset, not a pointer) on 64-bit systems. The
accessor macros convert it:

```c
/* include/linux/skbuff.h */
static inline unsigned char *skb_network_header(const struct sk_buff *skb)
{
    return skb->head + skb->network_header;
    /* Returns: pointer to IP header bytes */
}

static inline unsigned char *skb_transport_header(const struct sk_buff *skb)
{
    return skb->head + skb->transport_header;
    /* Returns: pointer to ICMP/TCP/UDP header bytes */
}
```

### How Each Layer "Sees" Its Header

Each layer macro does nothing but cast the appropriate offset into the layer's header
struct:

```c
/* L2: Ethernet header */
#define eth_hdr(skb)   ((struct ethhdr *)skb_mac_header(skb))

/* L3: IP header */
#define ip_hdr(skb)    ((struct iphdr *)skb_network_header(skb))

/* L4: ICMP header */
static inline struct icmphdr *icmp_hdr(const struct sk_buff *skb)
{
    return (struct icmphdr *)skb_transport_header(skb);
}
```

All three point into the **same buffer**. Reading `ip_hdr(skb)->protocol` reads one byte
at `skb->head + skb->network_header + offsetof(struct iphdr, protocol)`. The CPU never
fetches this byte from anywhere other than its L1/L2 cache line after the first access.

### How `skb_pull()` Advances the View

As the packet moves up the stack, `skb_pull()` advances `skb->data`:

```c
static inline void *skb_pull(struct sk_buff *skb, unsigned int len)
{
    skb->len  -= len;
    return skb->data += len;   /* advance the data pointer */
}
```

After `eth_type_trans()` calls `skb_pull(skb, ETH_HLEN)`, `skb->data` points at the IP
header. After `ip_local_deliver_finish()` calls `__skb_pull()`, `skb->data` points at the
ICMP header. **The Ethernet and IP bytes are still in memory** — only the pointer moved.

---

## 8. Memory Lifecycle: Who Allocates, Who Frees

### Allocation

| Where | Function | What it creates |
|---|---|---|
| Driver init | `dma_alloc_coherent()` | DMA ring buffers (NIC writes here) |
| Driver poll | `napi_alloc_skb()` | `sk_buff` control structure + headroom |
| Driver poll | `skb_add_rx_frag()` | Attaches DMA page to skb as a fragment |
| ICMP reply | `alloc_skb()` / `ip_reply_glue_bits()` | New `sk_buff` for the outgoing reply |

`napi_alloc_skb()` calls `__alloc_skb()` which does:
1. `kmem_cache_alloc(skbuff_head_cache)` — allocates the `struct sk_buff` itself from a
   dedicated slab cache
2. `kmalloc()` or `__netdev_alloc_frag()` — allocates the data buffer

### The Reference Count

Every `sk_buff` has a reference count: `skb->users` (an `atomic_t`). When the count
reaches zero, the buffer is freed.

```c
/* include/linux/skbuff.h */
static inline void kfree_skb(struct sk_buff *skb)
{
    if (!skb_unref(skb))   /* atomic_dec_and_test(&skb->users) */
        return;
    __kfree_skb(skb);       /* actually free it */
}
```

`__kfree_skb()` calls `skb_release_all()` which:
1. Calls `skb_release_data()` — releases the data buffer (decrements page refcount for
   page-based fragments, or `kfree()` for linear allocations)
2. Calls `kfree_skbmem()` — releases the `struct sk_buff` back to the slab cache

### Who Frees What

The rule in the kernel is: **the function that consumes an `sk_buff` is responsible for
freeing it.** The "protocol contract" is:

- If your handler returns success (`NET_RX_SUCCESS`, `0`), you consumed the skb. You must
  free it (or have passed ownership to someone else who will).
- If your handler returns an error, the caller may or may not free it — check the specific
  API.

Tracing our ICMP example:

```
icmp_rcv(skb)
  ├─ icmp_echo(skb)          ← reads data, does NOT free (caller will)
  │    └─ icmp_reply(...)    ← allocates a NEW skb for the reply
  │                             frees that new skb via ip_send_skb → ...
  │                             the ORIGINAL skb is untouched
  │
  └─ kfree_skb(skb)          ← icmp_rcv() frees the original incoming skb
```

For the reply path, the new `sk_buff` allocated by `icmp_reply()` travels down through:
```
ip_send_skb() → ip_local_out() → ip_output() → dev_queue_xmit()
  → sch_direct_xmit() → dev_hard_start_xmit()
  → driver's .ndo_start_xmit() [e.g., igb_xmit_frame()]
  → after DMA completes: dev_kfree_skb_irq(skb)   ← TX completion ISR frees it
```

### `kfree_skb` vs `consume_skb` vs `dev_kfree_skb`

These are not interchangeable. They carry semantic meaning:

| Function | Use case | Drop tracking |
|---|---|---|
| `kfree_skb(skb)` | Packet dropped (error path, bad checksum, etc.) | Records a drop in netstat |
| `consume_skb(skb)` | Packet successfully consumed (normal path) | No drop recorded |
| `dev_kfree_skb(skb)` | Driver freeing a TX skb after successful send | Calls `consume_skb` |
| `kfree_skb_reason(skb, reason)` | Drop with an explicit reason code | Used by modern code for debugging |

Using `kfree_skb()` on a successfully processed packet inflates your drop counters.
Using `consume_skb()` on a dropped packet hides errors. The distinction matters for
`/proc/net/dev` and `ss -s` statistics.

---

## 9. The Full Call Chain in One View

Starting from the NIC interrupt to `icmp_rcv()` completion and memory free:

```
[NIC hardware]
  └─ DMA frame to ring buffer → fire IRQ

[Driver ISR]  igb_intr()
  └─ igb_irq_disable()
  └─ napi_schedule()

[SoftIRQ]  net_rx_action()
  └─ igb_poll()
       └─ igb_clean_rx_irq()
            └─ igb_fetch_rx_buffer()     ← sk_buff allocated here
            └─ eth_type_trans()          ← EtherType read, skb->data advanced
            └─ napi_gro_receive()
                 └─ netif_receive_skb()  ← handoff to network core

[Network Core]  net/core/dev.c
  └─ __netif_receive_skb()
       └─ __netif_receive_skb_core()
            └─ ptype_base[ETH_P_IP] lookup
            └─ ip_packet_type.func(skb)  → ip_rcv()

[IP Layer]  net/ipv4/ip_input.c
  └─ ip_rcv()
       └─ NF_HOOK(NF_INET_PRE_ROUTING, ..., ip_rcv_finish)
  └─ ip_rcv_finish()
       └─ ip_route_input_noref()         ← routing decision
       └─ dst_input(skb)                 → ip_local_deliver()
  └─ ip_local_deliver()
       └─ NF_HOOK(NF_INET_LOCAL_IN, ..., ip_local_deliver_finish)
  └─ ip_local_deliver_finish()
       └─ __skb_pull()                   ← advance past IP header
       └─ inet_protos[IPPROTO_ICMP]      ← protocol lookup
       └─ ipprot->handler(skb)           → icmp_rcv()

[ICMP Layer]  net/ipv4/icmp.c
  └─ icmp_rcv()
       └─ pskb_may_pull()               ← validate skb has enough bytes
       └─ icmp_hdr(skb)                 ← cast pointer to icmphdr, zero-copy
       └─ __skb_checksum_complete()     ← verify checksum
       └─ switch(icmph->type)
            └─ ICMP_ECHO → icmp_echo()
                 └─ icmp_reply()        ← allocates NEW sk_buff for reply
                      └─ ip_send_skb() → [TX path] → dev_kfree_skb() [free reply]
       └─ kfree_skb(skb)               ← FREE original incoming packet
```

---

## 10. Annotated Code Walkthrough

This section is a compressed but complete read-through of the key functions in order.
Use it alongside `elixir.bootlin.com` with the latest stable kernel selected.

### How to Follow Along in Bootlin

1. Go to `https://elixir.bootlin.com/linux/latest/source`
2. For each function below, use the **search box** (top right) to find it
3. Click a function name in the source to see all its callers and callees

### Key Functions by File

**`drivers/net/ethernet/intel/igb/igb_main.c`**
- `igb_intr()` — ISR entry point
- `igb_poll()` — NAPI poll function
- `igb_clean_rx_irq()` — processes the DMA ring

**`net/core/dev.c`**
- `netif_receive_skb()` — public receive entry
- `__netif_receive_skb_core()` — does the actual EtherType dispatch

**`net/ipv4/ip_input.c`**
- `ip_rcv()` — IPv4 entry, attaches to `NF_INET_PRE_ROUTING`
- `ip_rcv_finish()` — after netfilter, does routing
- `ip_local_deliver()` — for locally destined packets
- `ip_local_deliver_finish()` — dispatches to L4 via `inet_protos[]`

**`net/ipv4/protocol.c`**
- `inet_add_protocol()` — registers a handler in `inet_protos[]`
- `inet_del_protocol()` — unregisters

**`net/ipv4/icmp.c`**
- `icmp_init()` — calls `inet_add_protocol(&icmp_protocol, IPPROTO_ICMP)`
- `icmp_rcv()` — entry point, validates, dispatches by type
- `icmp_echo()` — handles Echo Request (type 8)
- `icmp_reply()` — builds and sends Echo Reply

**`include/linux/skbuff.h`**
- `struct sk_buff` — the universal packet structure
- `skb_pull()`, `skb_push()`, `skb_put()` — pointer manipulation
- `kfree_skb()`, `consume_skb()` — reference-counted free

**`net/core/skbuff.c`**
- `__alloc_skb()` — real allocator (called by `napi_alloc_skb`)
- `__kfree_skb()` — real free (called by `kfree_skb` when refcount hits zero)
- `skb_clone()` — creates a reference to the same data (increments page refcount)
- `skb_copy()` — creates a true independent copy

---

## Summary: The Responsibilities at Each Stage

| Stage | File | Trigger Mechanism | Responsible For |
|---|---|---|---|
| NIC Hardware | — | DMA complete → IRQ | Writing bytes to RAM |
| Driver ISR | `igb_main.c` | Hardware interrupt | Scheduling NAPI poll |
| Driver Poll | `igb_main.c` | SoftIRQ (`net_rx_action`) | Allocating `sk_buff`, calling `netif_receive_skb` |
| Network Core | `net/core/dev.c` | Function call from driver | EtherType dispatch via `ptype_base` |
| IP Layer | `net/ipv4/ip_input.c` | `ip_packet_type.func` | Validation, routing, fragment reassembly, protocol dispatch via `inet_protos` |
| ICMP Layer | `net/ipv4/icmp.c` | `inet_protos[1].handler` | Checksum check, type dispatch, reply generation, **freeing the original `sk_buff`** |

---

*Generated as a follow-up to the ICMP Development notes.*
*Source references target Linux kernel 6.x. Use `elixir.bootlin.com` with a specific
version tag for exact line numbers.*
