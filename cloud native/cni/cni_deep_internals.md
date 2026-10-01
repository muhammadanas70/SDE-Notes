# CNI Deep Internals: The Complete Under-the-Hood Reference
## Volume II — Kernel Packet Path, eBPF Engine, OVS/OVN, SR-IOV, Multi-Runtime, Multi-Cluster

> **Prerequisite**: This guide assumes familiarity with the fundamentals covered in the companion
> guide (network namespaces, veth, bridge, basic CNI spec, kubelet/CRI integration, Flannel/Calico/Cilium overview).
> This volume goes deeper into every layer and covers topics not addressed there.

---

## Table of Contents

1. [sk_buff: The Kernel's Packet Representation](#1-sk_buff-the-kernels-packet-representation)
2. [The Full RX/TX Packet Path Through the Linux Kernel](#2-the-full-rxtx-packet-path-through-the-linux-kernel)
3. [NAPI, GRO, GSO, and TSO — Kernel Throughput Mechanisms](#3-napi-gro-gso-and-tso--kernel-throughput-mechanisms)
4. [eBPF Engine Deep Dive: Verifier, JIT, BTF, CO-RE](#4-ebpf-engine-deep-dive-verifier-jit-btf-co-re)
5. [eBPF Map Types and Their Use in CNI](#5-ebpf-map-types-and-their-use-in-cni)
6. [XDP: eXpress Data Path — Full Internals](#6-xdp-express-data-path--full-internals)
7. [AF_XDP: Zero-Copy Userspace Networking](#7-af_xdp-zero-copy-userspace-networking)
8. [Traffic Control (tc) Deep Dive: qdiscs, Classes, BPF Filters](#8-traffic-control-tc-deep-dive-qdiscs-classes-bpf-filters)
9. [Open vSwitch (OVS) and OVN Internals](#9-open-vswitch-ovs-and-ovn-internals)
10. [WireGuard Kernel Internals and CNI Integration](#10-wireguard-kernel-internals-and-cni-integration)
11. [VRF, Policy Routing, and ECMP Deep Dive](#11-vrf-policy-routing-and-ecmp-deep-dive)
12. [IPv6 Dual-Stack CNI](#12-ipv6-dual-stack-cni)
13. [SR-IOV, DPDK, and RDMA for High-Performance Pod Networking](#13-sr-iov-dpdk-and-rdma-for-high-performance-pod-networking)
14. [CNI in Non-Kubernetes Runtimes](#14-cni-in-non-kubernetes-runtimes)
15. [Sandboxed Runtimes: gVisor and Kata Containers CNI](#15-sandboxed-runtimes-gvisor-and-kata-containers-cni)
16. [Windows CNI: HNS and HCN Architecture](#16-windows-cni-hns-and-hcn-architecture)
17. [Multi-Cluster Networking](#17-multi-cluster-networking)
18. [Conntrack: Deep Internals and Tuning](#18-conntrack-deep-internals-and-tuning)
19. [IPVS Mode: kube-proxy and CNI Interaction](#19-ipvs-mode-kube-proxy-and-cni-interaction)
20. [Network Policy: Implementation Internals](#20-network-policy-implementation-internals)
21. [MTU, PMTU Discovery, and MSS Clamping](#21-mtu-pmtu-discovery-and-mss-clamping)
22. [CNI Security: Privilege Model and Attack Surface](#22-cni-security-privilege-model-and-attack-surface)
23. [Observability: eBPF-Based Network Telemetry](#23-observability-ebpf-based-network-telemetry)
24. [Full Rust CNI Plugin Implementation](#24-full-rust-cni-plugin-implementation)
25. [eBPF CNI Data Plane in C: XDP + TC Program](#25-ebpf-cni-data-plane-in-c-xdp--tc-program)
26. [CNI Performance: Benchmarks, Bottlenecks, Tuning](#26-cni-performance-benchmarks-bottlenecks-tuning)

---

## 1. sk_buff: The Kernel's Packet Representation

Every packet in the Linux kernel is an `sk_buff` (socket buffer). Understanding this struct is
foundational to understanding how CNI data plane operations actually work. When a CNI plugin
redirects, drops, or modifies a packet — it is operating on `sk_buff` instances.

### 1.1 The sk_buff Structure

```
include/linux/skbuff.h — simplified layout:

struct sk_buff {
    /* === TRANSPORT LAYER === */
    union {
        struct sock     *sk;       /* owning socket (NULL for forwarded pkts) */
        int              ip_defrag_offset;
    };
    union {
        ktime_t          tstamp;   /* receive timestamp */
        u64              skb_mstamp_ns;
    };

    /* === DEVICE/INTERFACE === */
    struct net_device   *dev;      /* device this packet arrived on / will go out on */
    unsigned int         len;      /* total packet length (data + frags) */
    unsigned int         data_len; /* length of paged (fragmented) data */
    __u16                mac_len;  /* length of MAC header */
    __u16                hdr_len;  /* writable header length (for clones) */

    /* === CHECKSUM === */
    __wsum               csum;
    union {
        struct { __u16 csum_start; __u16 csum_offset; };
        __wsum    csum_data;
    };

    /* === ROUTING / POLICY === */
    __u32                priority;  /* packet priority (TC qdisc) */
    __u32                mark;      /* fwmark: used by ip rule/iptables MARK */
    __u32                hash;      /* packet hash (RSS, ECMP) */
    __be16               protocol;  /* L3 protocol: ETH_P_IP, ETH_P_IPV6, ... */

    /* === NETFILTER / CONNTRACK === */
    #if defined(CONFIG_NF_CONNTRACK)
    struct nf_conntrack *nfct;     /* conntrack entry this packet belongs to */
    #endif
    __u8                 pkt_type; /* PACKET_HOST, PACKET_BROADCAST, etc. */

    /* === OFFLOAD === */
    __u8                 ip_summed:2;   /* checksum offload state */
    __u8                 ooo_okay:1;    /* TCP out-of-order reorder OK */
    __u8                 gso_type;      /* GSO type (TCP, UDP, etc.) */
    unsigned int         gso_size;      /* segment size for GSO */
    unsigned int         gso_segs;      /* number of segments */

    /* === eBPF METADATA === */
    struct bpf_flow_keys *flow_keys;
    #ifdef CONFIG_BPF_SYSCALL
    union { u32 tc_index; };       /* traffic control index */
    union { u32 tc_classid; };     /* tc class ID (set by BPF or tc filter) */
    #endif

    /* === MEMORY LAYOUT POINTERS (critical for packet parsing) === */
    sk_buff_data_t       tail;     /* pointer to end of actual data */
    sk_buff_data_t       end;      /* pointer to end of buffer (max tail) */
    unsigned char       *head;     /* start of allocated buffer */
    unsigned char       *data;     /* start of actual packet data */

    /* === FRAGMENTATION (scatter-gather I/O) === */
    skb_frag_t          frags[MAX_SKB_FRAGS];  /* page fragments */
    struct sk_buff      *next;     /* for sk_buff lists */
    struct sk_buff      *prev;
};
```

### 1.2 sk_buff Memory Layout

The genius of sk_buff is its **headroom/tailroom** design. Headers are prepended (not inserted) as
the packet travels down the stack, avoiding memcpy:

```
Allocated buffer (kmalloc or page):

head                data                             tail        end
 |                   |                                |           |
 v                   v                                v           v
 +-------------------+--------------------------------+-----------+
 |    headroom       |        packet data             | tailroom  |
 |  (reserved for    |  (grows left as headers added) | (unused)  |
 |   header growth)  |                                |           |
 +-------------------+--------------------------------+-----------+

As packet goes DOWN the stack (transmit path):
  User data:         [TCP payload]
  TCP adds header:   [TCP hdr][TCP payload]        skb_push(skb, sizeof(tcp_hdr))
  IP adds header:    [IP hdr][TCP hdr][payload]    skb_push(skb, sizeof(ip_hdr))
  Ethernet adds hdr: [ETH][IP][TCP][payload]       skb_push(skb, ETH_HLEN)
  -> data pointer moves LEFT (into headroom)

As packet goes UP the stack (receive path):
  Driver delivers:   [ETH][IP][TCP][payload]
  ETH strips header: [IP][TCP][payload]            skb_pull(skb, ETH_HLEN)
  IP strips header:  [TCP][payload]                skb_pull(skb, ip_hdrlen)
  TCP strips header: [payload]                     skb_pull(skb, tcp_hdrlen)
  -> data pointer moves RIGHT

Key functions:
  skb_push(skb, len)  -- moves data LEFT by len (prepend header)
  skb_pull(skb, len)  -- moves data RIGHT by len (consume header)
  skb_put(skb, len)   -- extends tail RIGHT by len (append data)
  skb_reserve(skb, len) -- moves both head and data right (reserve headroom)
```

### 1.3 sk_buff Cloning and Copying

CNI data plane operations often need to clone or copy sk_buffs:

```c
/* Clone: shared data, independent metadata (refcount on data) */
struct sk_buff *clone = skb_clone(skb, GFP_ATOMIC);
/* clone->data points to same memory as skb->data
   Modifying headers in clone requires skb_copy_expand or skb_unshare */

/* Copy: completely independent copy */
struct sk_buff *copy = skb_copy(skb, GFP_ATOMIC);

/* For BPF redirect (Cilium): packet is moved, not copied */
/* bpf_redirect() just changes dev pointer, no copy needed */
```

**Why this matters for CNI:**
- Cilium's BPF `bpf_redirect()` is fast because it just changes the destination interface index;
  the sk_buff is not copied.
- VXLAN encapsulation (`vxlan_xmit()`) does a `skb_copy_expand()` to add outer header space.
- Port mapping (DNAT) modifies IP/port headers in-place after `skb_make_writable()`.

### 1.4 sk_buff and Network Namespaces

```c
/* Every sk_buff that is forwarded across namespace boundaries:
   - The sk_buff itself is namespace-agnostic (just a buffer)
   - The routing decision (dev, sk) is namespace-specific

   When a veth delivers a packet cross-namespace:
   veth_xmit() on host-side:
     1. Gets peer device (container-side veth)
     2. Calls napi_gro_receive() on peer's NAPI instance
        -> This runs in the peer device's context
        -> Subsequent processing uses the PEER DEVICE'S namespace
           (found via peer->nd_net / read_pnet(&peer->nd_net))
     3. skb->dev = peer_dev (now in container namespace context)
     4. netif_receive_skb() -> IPv4/IPv6 input -> socket lookup in container ns
*/
```

---

## 2. The Full RX/TX Packet Path Through the Linux Kernel

This is the complete journey of a packet. Understanding this is essential for knowing WHERE eBPF,
tc, and CNI hooks intercept packets.

### 2.1 RX Path: NIC to Socket

```
HARDWARE                     KERNEL                          USERSPACE
--------                     ------                          ---------

                             CPU: interrupt disabled
NIC DMA -->  ring buffer     NAPI poll scheduled
(packet lands in             (net/core/dev.c: napi_schedule)
 RX descriptor ring)                |
                                    v
                             netif_receive_skb()
                             net/core/dev.c
                                    |
                         +---------+----------+
                         |                    |
                    [RX BPF hook]       [no BPF: direct]
                    (skb type)
                         |                    |
                         v                    v
                    __netif_receive_skb_core()
                         |
                    protocol_type dispatch
                    (ETH_P_IP -> ip_rcv)
                    (ETH_P_ARP -> arp_rcv)
                    (ETH_P_8021Q -> vlan handler)
                         |
                         v
                    [NF_INET_PRE_ROUTING]        <- PREROUTING iptables/nftables
                    (net/netfilter/nf_hook.c)
                         |
                    conntrack: nf_conntrack_in()  <- connection tracking
                    (net/netfilter/nf_conntrack_core.c)
                         |
                    DNAT (kube-proxy/Cilium LB)   <- PREROUTING DNAT
                         |
                         v
                    ip_rcv_finish()
                    fib_lookup()                  <- routing decision
                         |
              +----------+----------+
              |                     |
         LOCAL delivery        FORWARD path
              |                     |
              v                     v
         [NF_INET_LOCAL_IN]    [NF_INET_FORWARD]   <- FORWARD chain
         (INPUT chain)              |
              |               [NF_INET_POST_ROUTING] <- POSTROUTING
              v                     |                   (MASQUERADE/SNAT)
         ip_local_deliver()         v
              |               ip_output()
         TCP/UDP demux              |
         (inet_lookup)         dev_queue_xmit()
              |               qdisc (tc)
              v               NIC TX ring
         socket recv queue    DMA to wire
              |
              v
         userspace read()
         (recvmsg syscall)
```

### 2.2 TX Path: Socket to Wire

```
USERSPACE                    KERNEL
---------                    ------

sendmsg() syscall
       |
       v
sock_sendmsg() -> inet_sendmsg()
       |
       v
TCP: tcp_sendmsg()
  -> tcp_push() -> tcp_write_xmit()
  -> skb = alloc_skb(headroom + data + tailroom)
  -> copy user data: skb_copy_datagram_from_iter()
  -> tcp_transmit_skb()
       |
       v
IP: ip_queue_xmit() / ip_send_skb()
  -> fib_lookup() (routing)
  -> ip_options_build()
  -> skb_push(skb, sizeof(iphdr))
  -> ip_select_ident()
  -> ip_local_out()
       |
       v
  [NF_INET_LOCAL_OUT]      <- OUTPUT chain
  [NF_INET_POST_ROUTING]   <- POSTROUTING (SNAT/MASQUERADE)
       |
       v
  dst_output() -> ip_output()
  -> skb_push(skb, ETH_HLEN)  <- Ethernet header
  -> neigh_output()            <- ARP resolution
       |
       v
  dev_queue_xmit()
       |
       v
  sch_handle_egress()      <- tc egress hook (BPF programs here!)
       |
       v
  qdisc enqueue
  (pfifo_fast, tbf, htb, fq_codel, mq...)
       |
       v
  qdisc dequeue -> driver TX
  -> ndo_start_xmit()      <- NIC driver
  -> DMA to hardware ring buffer
  -> PCI / PCIe transfer to NIC
  -> NIC sends on wire
```

### 2.3 Cross-Namespace Packet Flow (veth)

This is the specific path for pod-to-pod traffic on the same node:

```
Pod A (netns A)                Host netns                   Pod B (netns B)
--------------                 ----------                   --------------

sendmsg(dst=10.244.1.6)
    |
ip_output()
    |
eth0 TX (veth-container-A)
    |
    | [veth driver: veth_xmit()]
    | skb->dev = peer (lxcXXX in host ns)
    |
    +---------> netif_rx(skb) in host netns
                    |
               [TC ingress BPF on lxcXXX] <- Cilium policy check here
                    |
               netif_receive_skb()
                    |
               ip_rcv() -> fib_lookup()
                    |
               [TC egress BPF on lxcYYY] <- Cilium LB, encrypt, etc.
                    |
               dev_queue_xmit() on lxcYYY (host-side of pod-B's veth)
                    |
                    | [veth driver: veth_xmit() on lxcYYY]
                    | skb->dev = peer (eth0 in pod-B netns)
                    |
                    +---------> netif_rx(skb) in pod-B netns
                                    |
                               ip_rcv() -> tcp_rcv()
                                    |
                               recvmsg() in pod-B
```

**Cilium shortcut (same-node, BPF redirect):**

```
Instead of full routing, Cilium does:
  TC ingress BPF on lxcXXX:
    bpf_redirect_peer(lxcYYY_ifindex, 0)
    -> packet goes DIRECTLY to lxcYYY's peer (eth0 in pod-B netns)
    -> SKIPS the host routing stack entirely
    -> ~30% faster than full routing path
```

---

## 3. NAPI, GRO, GSO, and TSO — Kernel Throughput Mechanisms

These mechanisms dramatically affect CNI performance and must be understood for tuning.

### 3.1 NAPI (New API) — Interrupt Mitigation

```
Problem: At 10Gbps, NIC generates ~14.88M interrupts/sec (64-byte packets)
         Interrupt overhead overwhelms CPUs

NAPI Solution:
  1. First packet: hardware interrupt fires
  2. Interrupt handler: disable further NIC interrupts, schedule NAPI poll
  3. NAPI poll (softirq NET_RX_SOFTIRQ): process up to budget packets
     (default budget=300 packets per poll cycle)
  4. If ring drained: re-enable interrupts
  5. If not drained: keep polling (no interrupt, just softirq)

struct napi_struct {
    struct list_head    poll_list;     /* list of active NAPI instances */
    unsigned long       state;         /* NAPI_STATE_SCHED, etc. */
    int                 weight;        /* poll budget */
    int                 (*poll)(struct napi_struct *, int); /* driver's poll fn */
    ...
};

/* Driver receives packet: */
napi_schedule(&dev->napi);              /* schedules softirq */
/* Softirq runs driver->poll(): */
int driver_poll(struct napi_struct *napi, int budget) {
    int work = 0;
    while (work < budget && ring_not_empty()) {
        skb = build_skb(ring_entry);   /* map DMA buffer to sk_buff */
        netif_receive_skb(skb);        /* deliver to network stack */
        work++;
    }
    if (work < budget)
        napi_complete(napi);           /* done, re-enable interrupts */
    return work;
}
```

**Why CNI plugins care about NAPI:**
- veth pairs have their own NAPI instance per direction
- Cilium's veth-based data plane uses `napi_gro_receive()` for delivering to peers
- Tuning: `ethtool -C eth0 rx-usecs 50` adjusts interrupt coalescing

### 3.2 GRO (Generic Receive Offload)

GRO coalesces many small incoming packets into fewer large ones before delivering to the stack.
This amortizes per-packet overhead.

```
Without GRO:
  NIC delivers 100 × 1460-byte TCP segments
  -> 100 calls to ip_rcv(), tcp_rcv()
  -> 100 socket buffer allocations
  -> 100 calls to userspace (batched by TCP, but still)

With GRO:
  NIC delivers 100 × 1460-byte segments
  -> GRO engine coalesces into 1 × ~146KB super-segment
  -> 1 call through ip_rcv(), tcp_rcv()
  -> tcp_rcv() segments it back for socket delivery
  -> Much less per-packet overhead

GRO Implementation (net/core/gro.c):
  napi_gro_receive(napi, skb):
    -> dev_gro_receive():
       Check if skb can be merged with existing GRO flow:
       - Same src/dst IP, same TCP flow
       - Contiguous TCP sequence numbers
       - Same GSO parameters
    -> If yes: skb_gro_receive() merges into existing GRO skb
    -> If no (timeout or flush): deliver accumulated super-segment

GRO and tunnels:
  GRO understands VXLAN, GENEVE, GRE tunnels
  -> Can coalesce inner packets even through overlay headers
  -> Critical for CNI overlay performance (Flannel, Cilium tunnel mode)
```

### 3.3 GSO/TSO — Segmentation Offload

```
GSO (Generic Segmentation Offload) — software:
  Application writes 64KB buffer
  TCP: don't segment yet, create 1 large skb with gso_size=1460
  -> Segmentation happens AFTER routing, just before driver
  -> net/core/dev.c: skb_gso_segment()

TSO (TCP Segmentation Offload) — hardware:
  NIC supports segmentation in hardware
  -> Kernel sends 1 large skb, NIC segments to MTU-sized frames
  -> Dramatically reduces CPU cycles for large transfers

For CNI:
  Overlay tunnels complicate TSO:
    Inner TCP segment: up to 1500 bytes
    VXLAN header: +50 bytes
    -> Outer packet > MTU (1550 > 1500) = fragmentation or MTU reduction
  
  Solutions:
    1. Reduce MTU in container (1450 for VXLAN, 1430 for WireGuard+VXLAN)
    2. Use NIC with encapsulation offload (VXLAN TSO, GENEVE TSO)
    3. Cilium: uses native routing or IPIP which has smaller overhead

  Check offload support:
    ethtool -k eth0 | grep -E "tx-udp_tnl|tx-vxlan"
```

---

## 4. eBPF Engine Deep Dive: Verifier, JIT, BTF, CO-RE

Understanding the eBPF engine is critical for understanding how Cilium and other eBPF-based CNI
plugins work without crashing the kernel.

### 4.1 eBPF Program Lifecycle

```
Developer writes BPF program (C)
       |
       v
Clang/LLVM compiles to BPF bytecode (ELF .o file)
       |
       v
Userspace loader (libbpf):
  bpf_object__open()    -> parse ELF, load maps, programs
  bpf_object__load()    -> sys_bpf(BPF_PROG_LOAD, ...)
       |
       v
Kernel: bpf_prog_load() syscall handler
       |
       v
  [VERIFIER]  <-- safety guarantee
       |
       v
  [JIT compiler]  <-- performance (optional, can interpret)
       |
       v
Program loaded, fd returned to userspace
       |
       v
Userspace attaches program to hook:
  TC:    bpf_program__attach_tc()
  XDP:   bpf_program__attach_xdp()
  cgroup: bpf_program__attach_cgroup()
```

### 4.2 The eBPF Verifier

The verifier is what makes eBPF safe. It runs before any program executes.

```
Verifier checks (kernel/bpf/verifier.c):

1. CONTROL FLOW ANALYSIS:
   - Build CFG (Control Flow Graph)
   - Detect unreachable instructions
   - Ensure NO UNBOUNDED LOOPS (pre-5.3 kernels)
   - Post-5.3: bounded loops allowed if verifier can prove termination
   - NO BACKWARD JUMPS that could create infinite loops (enforced)

2. TYPE CHECKING (register state tracking):
   The verifier tracks the "type" of every register at every instruction:
   
   enum bpf_reg_type {
     NOT_INIT = 0,           /* register has not been set */
     SCALAR_VALUE,           /* integer, not a pointer */
     PTR_TO_CTX,             /* pointer to program context (skb, xdp_md) */
     PTR_TO_MAP_KEY,         /* pointer to map key (stack memory) */
     PTR_TO_MAP_VALUE,       /* pointer to map value */
     PTR_TO_STACK,           /* pointer to stack frame */
     PTR_TO_PACKET,          /* pointer into skb->data */
     PTR_TO_PACKET_END,      /* pointer to skb->data_end */
     PTR_TO_FLOW_KEYS,       /* pointer to flow keys */
     PTR_TO_SOCKET,          /* pointer to struct bpf_sock */
     ...
   };

   Example: bounds checking enforcement:
   
   /* This BPF C code: */
   void *data = (void *)(long)skb->data;
   void *data_end = (void *)(long)skb->data_end;
   struct ethhdr *eth = data;
   
   /* MUST check bounds or verifier rejects: */
   if (data + sizeof(*eth) > data_end)
       return XDP_DROP;
   /* After this check, eth->h_proto is safe to access */
   
   /* Without the check, verifier says:
      "invalid access to packet, off=14 size=2, R1 offset from packet start
       is unbounded - cannot guarantee packet is large enough" */

3. HELPER FUNCTION VALIDATION:
   - Only whitelisted helpers allowed per program type
   - Argument types validated against helper prototype
   - bpf_skb_store_bytes(): verified caller has PTR_TO_SKB context

4. STACK DEPTH LIMIT:
   - Max 512 bytes of stack per BPF program
   - No recursion (tail calls allowed, up to 33 tail calls deep)

5. INSTRUCTION COUNT LIMIT:
   - 1 million instructions max (was 4096 in early kernels)
   - Cilium's complex programs stay within this with tail calls

VERIFIER COMPLEXITY:
  State space explosion: at each branch point, verifier tracks BOTH paths
  If paths rejoin with different register states -> verifier must merge states
  Cilium workaround: compile separate BPF programs for each policy rule
```

### 4.3 JIT Compilation

After verification, eBPF bytecode is JIT-compiled to native machine code:

```
eBPF ISA (Instruction Set Architecture):
  - 64-bit registers: r0 (return value), r1-r5 (args), r6-r9 (callee-saved), r10 (FP)
  - 10 64-bit registers total
  - Load/store with bounds checking (implicit via verifier)
  - 8 arithmetic/logical ops
  - Call instruction (helper functions)
  - Conditional and unconditional jumps

JIT mapping (x86_64):
  BPF r0  -> rax (return value)
  BPF r1  -> rdi (arg 1)
  BPF r2  -> rsi (arg 2)
  BPF r3  -> rdx (arg 3)
  BPF r4  -> rcx (arg 4)
  BPF r5  -> r8  (arg 5)
  BPF r6  -> rbx (callee-saved)
  BPF r7  -> r13
  BPF r8  -> r14
  BPF r9  -> r15
  BPF r10 -> rbp (frame pointer)

JIT output: native x86_64 code, directly callable
Performance: ~equal to native C code compiled with -O2

Enable JIT:
  echo 1 > /proc/sys/net/core/bpf_jit_enable
  echo 2 > /proc/sys/net/core/bpf_jit_enable  # also log JIT output
  
  sysctl net.core.bpf_jit_harden=0   # 0=off, 1=jit harden for unprivileged, 2=always
```

### 4.4 BTF (BPF Type Format)

BTF encodes C type information alongside BPF programs. It enables CO-RE and is required for
modern eBPF development.

```
BTF is like DWARF debug info but compact and embedded in the kernel:
  - Type descriptions (structs, unions, enums, typedefs)
  - Function signatures
  - Line number info (for error messages)

Kernel BTF:
  The Linux kernel ships with BTF for all its types:
  /sys/kernel/btf/vmlinux  <- BTF for the running kernel
  
  bpftool btf dump file /sys/kernel/btf/vmlinux | grep -A10 "struct sk_buff"
  
  Output shows exact field offsets for the running kernel version.

vmlinux.h (generated):
  bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h
  
  This gives you ALL kernel structs as C headers without kernel headers.
  Cilium uses this extensively.

BPF map with BTF:
  struct {
      __uint(type, BPF_MAP_TYPE_HASH);
      __type(key, struct endpoint_key);    /* BTF encodes key type */
      __type(value, struct endpoint_info); /* BTF encodes value type */
      __uint(max_entries, 65536);
  } cilium_lxc SEC(".maps");
  
  BTF allows bpftool and Hubble to decode map entries with full type info.
```

### 4.5 CO-RE (Compile Once, Run Everywhere)

CO-RE solves the portability problem: struct field offsets differ between kernel versions.

```
Problem without CO-RE:
  struct sk_buff {
    /* Kernel 5.15: sk is at offset 0 */
    struct sock *sk;
    ...
  };
  struct sk_buff {
    /* Kernel 5.10: sk is at offset 0 but with different padding */
    ...
  };
  
  Hard-coded offset access BREAKS across kernels.

CO-RE Solution:
  1. BPF program uses BTF-aware field access:
     
     struct task_struct *task = bpf_get_current_task_btf();
     pid_t pid = BPF_CORE_READ(task, pid);
     /* BPF_CORE_READ macro records the access in BTF relocations */
  
  2. Compiler emits BTF relocation records in ELF
  
  3. libbpf loader reads /sys/kernel/btf/vmlinux at load time
  
  4. libbpf patches bytecode with CORRECT offset for the running kernel
  
  5. Same compiled .o file works on kernel 5.4, 5.10, 5.15, 6.x
  
  For Cilium:
    Cilium ships pre-compiled BPF objects
    cilium-agent loads them via libbpf with CO-RE relocations
    No kernel header dependency at runtime

BPF_CORE_READ internals:
  #define BPF_CORE_READ(src, a, ...) ({         \
    ___type(src, a, ##__VA_ARGS__) __r;         \
    BPF_CORE_READ_INTO(&__r, src, a, ##__VA_ARGS__); \
    __r;                                         \
  })
  /* Expands to a load instruction with BTF type annotation */
  /* libbpf sees the annotation and patches offset at load time */
```

---

## 5. eBPF Map Types and Their Use in CNI

eBPF maps are the shared memory between BPF programs and userspace. CNI plugins (especially
Cilium) use dozens of map types for different purposes.

### 5.1 Core Map Types

```
BPF_MAP_TYPE_HASH (generic hash map):
  - Key: arbitrary bytes (exact match)
  - Value: arbitrary bytes
  - O(1) average lookup, O(n) worst case
  - Cilium use: cilium_ipcache (IP -> security identity)
  
  struct bpf_map_def SEC("maps") cilium_ipcache = {
      .type        = BPF_MAP_TYPE_HASH,
      .key_size    = sizeof(struct endpoint_key),   /* IP + family */
      .value_size  = sizeof(struct remote_endpoint_info), /* identity, key */
      .max_entries = IPCACHE_MAP_SIZE,
  };

BPF_MAP_TYPE_LRU_HASH:
  - Like HASH but automatically evicts least recently used entries
  - No manual GC needed
  - Cilium use: connection tracking tables (replace kernel conntrack)
  
  /* LRU is critical for high-connection-count workloads:
     no need to explicitly delete stale entries */

BPF_MAP_TYPE_PERCPU_HASH:
  - Each CPU has its own hash table
  - No lock contention (reads/writes on current CPU only)
  - Values must be aggregated in userspace
  - Cilium use: per-CPU drop/forward counters
  
  /* Reading per-CPU map: userspace reads ALL CPU copies and sums */

BPF_MAP_TYPE_ARRAY:
  - Fixed-size array, key is index (u32)
  - Always in memory (no hash overhead)
  - Cannot delete entries (just zero them)
  - Cilium use: config maps, flags, global state
  
  struct {
      __uint(type, BPF_MAP_TYPE_ARRAY);
      __type(key, __u32);
      __type(value, struct bpf_elf_map);
      __uint(max_entries, 1);
  } cilium_config SEC(".maps");

BPF_MAP_TYPE_PERCPU_ARRAY:
  - Per-CPU array (no lock, per-CPU scratch space)
  - Cilium use: per-packet scratch buffers (avoid stack limits)
  
  /* Trick: allocate large structs in per-CPU array instead of stack
     (stack limit: 512 bytes; per-CPU array: up to 32KB per entry) */

BPF_MAP_TYPE_PROG_ARRAY:
  - Array of BPF program file descriptors
  - Used for tail calls (bpf_tail_call)
  - Cilium splits complex programs across multiple entries
  
  /* tail call: effectively a goto to another BPF program
     Does NOT return to caller (like exec, not call)
     Stack does not grow
     Max 33 tail calls per packet */
  
  bpf_tail_call(ctx, &cilium_calls, CILIUM_CALL_IPV4_FROM_LXC);

BPF_MAP_TYPE_DEVMAP / BPF_MAP_TYPE_DEVMAP_HASH:
  - Maps index -> net_device (ifindex)
  - Used with bpf_redirect_map() for bulk packet redirect
  - XDP programs use this for multi-port forwarding
  
  /* Bulk redirect: up to 64 packets sent to device at once */
  /* Used by Cilium for efficient packet forwarding between pods */

BPF_MAP_TYPE_SOCKMAP / BPF_MAP_TYPE_SOCKHASH:
  - Maps to socket structs
  - bpf_sk_redirect_map() for socket-level forwarding
  - Cilium uses for local service acceleration (bypass IP stack)
  
  /* When pod A accesses ClusterIP that resolves to pod B (same node):
     Traffic can bypass IP routing entirely via sockmap
     sk_msg BPF program redirects at socket level
     Latency: ~2µs instead of ~10µs for full IP stack path */

BPF_MAP_TYPE_RINGBUF:
  - Single-producer, single-consumer ring buffer (kernel to userspace)
  - Replaces BPF_MAP_TYPE_PERF_EVENT_ARRAY for events
  - Memory-efficient: no wasted per-CPU copies
  - Cilium/Hubble use: flow events, DNS events, drop notifications
  
  /* BPF program: */
  struct flow_event *ev = bpf_ringbuf_reserve(&events, sizeof(*ev), 0);
  if (!ev) return 0;  /* ring full, drop event */
  ev->src_ip = src; ev->dst_ip = dst; ev->port = port;
  bpf_ringbuf_submit(ev, 0);
  
  /* Userspace (Hubble): */
  ring_buffer__consume(rb);  /* calls callback for each event */

BPF_MAP_TYPE_SK_STORAGE:
  - Per-socket storage (like an inode cache)
  - Attached to socket lifetime, GC'd automatically when socket closes
  - No global lock, each socket is independent
  - Use: per-connection metadata (policy result, encryption state)
```

### 5.2 Map Pinning and Sharing Between Programs

```
Pin a map to BPF virtual filesystem:
  bpf_obj_pin(map_fd, "/sys/fs/bpf/cilium/maps/cilium_lxc")
  
  -> Map survives program reload (program can be unloaded/reloaded without losing state)
  -> Multiple programs can share the same map by path

Cilium map hierarchy:
  /sys/fs/bpf/
  └── tc/
      └── globals/
          ├── cilium_lxc           <- endpoint metadata
          ├── cilium_ipcache       <- IP to identity
          ├── cilium_policy        <- policy rules (per-endpoint submaps)
          ├── cilium_ct4_global    <- IPv4 connection tracking
          ├── cilium_ct6_global    <- IPv6 connection tracking
          ├── cilium_lb4_services  <- L4 LB service table
          ├── cilium_lb4_backends  <- LB backend table
          ├── cilium_lb4_state     <- connection affinity state
          └── cilium_calls         <- tail call jump table (PROG_ARRAY)

Map-in-map (BPF_MAP_TYPE_HASH_OF_MAPS):
  Cilium uses this for per-endpoint policy maps:
  - Outer map: endpoint ID -> inner map fd
  - Inner map: policy rules for that specific endpoint
  - Allows atomic policy updates per endpoint without touching others
```

---

## 6. XDP: eXpress Data Path — Full Internals

XDP is the earliest point in the Linux network stack where you can intercept packets. It runs
in the NIC driver's NAPI poll loop, before sk_buff allocation.

### 6.1 XDP Program Types and Hook Points

```
Three XDP attachment modes:

1. NATIVE XDP (offloaded to driver):
   +-----------+    +------------------+    +------------+
   |  NIC HW   | -> | DMA ring buffer  | -> | XDP hook   |
   +-----------+    +------------------+    | (in driver)|
                                            +------------+
   Driver calls bpf_prog_run_xdp() BEFORE sk_buff allocation
   Supported by: mlx5, i40e, ixgbe, bnxt, qede, virtio_net, veth
   Fastest: ~14 Mpps on 10GbE with simple drop rule
   
2. SKBUFF-MODE XDP (generic, fallback):
   All drivers: XDP runs after sk_buff allocation
   Slower (~3x) but works everywhere
   Not suitable for DDoS mitigation (sk_buff already allocated = memory pressure)
   
3. HW OFFLOAD XDP (NIC does it in silicon):
   NIC executes BPF bytecode ON THE NIC
   Supported by: Netronome SmartNICs only
   Fastest possible: zero CPU cycles for dropped packets

XDP return codes:
  XDP_DROP    -> drop packet immediately, free DMA buffer
  XDP_PASS    -> pass packet to normal kernel stack (allocate sk_buff)
  XDP_TX      -> send packet back OUT the same interface (useful for reflection)
  XDP_REDIRECT -> redirect to another interface or userspace (AF_XDP)
  XDP_ABORTED -> BPF program error (logs trace)
```

### 6.2 XDP Program Structure and Context

```c
/* XDP program context: minimal struct, no sk_buff */
struct xdp_md {
    __u32 data;          /* pointer to packet data start */
    __u32 data_end;      /* pointer to packet data end */
    __u32 data_meta;     /* pointer to metadata area (before data) */
    __u32 ingress_ifindex; /* interface index packet arrived on */
    __u32 rx_queue_index;  /* hardware RX queue index */
    __u32 egress_ifindex;  /* for XDP_REDIRECT: target interface */
};

/* XDP metadata area (between data_meta and data):
   XDP programs can store metadata here that TC BPF programs can read
   Used for communication between XDP (DDoS filter) and TC (forwarding)
   
   xdp_adjust_meta(ctx, -sizeof(struct my_metadata)):
   struct my_metadata *meta = (void *)(long)ctx->data_meta;
   meta->rx_queue = ctx->rx_queue_index;
   /* TC program can read this via skb->data_meta */
*/
```

### 6.3 XDP for CNI: Full C Implementation

```c
/* xdp_cni_filter.c
 * XDP program for a CNI plugin:
 * - Fast packet classification
 * - DDoS protection at line rate
 * - Statistics collection
 * 
 * Load with: ip link set dev eth0 xdp obj xdp_cni_filter.o sec xdp_filter
 */

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

/* ===== Map Definitions ===== */

/* Blocked source IPs (DDoS blocklist) */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key, __be32);          /* IPv4 source address */
    __type(value, __u64);         /* block timestamp (ns) */
    __uint(max_entries, 100000);
} blocklist SEC(".maps");

/* Rate limiting: per-source packet counters */
struct rate_limit_entry {
    __u64 packets;
    __u64 last_reset_ns;
};
struct {
    __uint(type, BPF_MAP_TYPE_LRU_PERCPU_HASH);
    __type(key, __be32);
    __type(value, struct rate_limit_entry);
    __uint(max_entries, 100000);
} rate_limits SEC(".maps");

/* Per-CPU statistics */
struct xdp_stats {
    __u64 rx_packets;
    __u64 rx_bytes;
    __u64 dropped_blocklist;
    __u64 dropped_ratelimit;
    __u64 passed;
};
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, __u32);
    __type(value, struct xdp_stats);
    __uint(max_entries, 1);
} xdp_stats_map SEC(".maps");

/* Pod IP to egress interface (for redirect) */
struct {
    __uint(type, BPF_MAP_TYPE_DEVMAP_HASH);
    __type(key, __be32);           /* pod IP */
    __type(value, __u32);          /* ifindex */
    __uint(max_entries, 1024);
} pod_redirect_map SEC(".maps");

/* ===== Helper Functions ===== */

static __always_inline struct xdp_stats *get_stats(void) {
    __u32 key = 0;
    return bpf_map_lookup_elem(&xdp_stats_map, &key);
}

#define RATE_LIMIT_PPS 10000    /* 10k packets/sec per source */
#define RATE_LIMIT_WINDOW_NS (1000000000ULL)  /* 1 second */

static __always_inline int is_rate_limited(__be32 src_ip) {
    struct rate_limit_entry *entry;
    struct rate_limit_entry new_entry = {};
    __u64 now = bpf_ktime_get_ns();
    
    entry = bpf_map_lookup_elem(&rate_limits, &src_ip);
    if (!entry) {
        new_entry.packets = 1;
        new_entry.last_reset_ns = now;
        bpf_map_update_elem(&rate_limits, &src_ip, &new_entry, BPF_ANY);
        return 0;
    }
    
    /* Reset window if expired */
    if (now - entry->last_reset_ns > RATE_LIMIT_WINDOW_NS) {
        entry->packets = 1;
        entry->last_reset_ns = now;
        return 0;
    }
    
    entry->packets++;
    return entry->packets > RATE_LIMIT_PPS;
}

/* ===== Main XDP Program ===== */

SEC("xdp_filter")
int xdp_cni_program(struct xdp_md *ctx) {
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;
    struct xdp_stats *stats = get_stats();
    int action = XDP_PASS;
    
    if (!stats) return XDP_PASS;
    
    /* Bounds check: Ethernet header */
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) {
        action = XDP_DROP;
        goto out;
    }
    
    /* Only process IPv4 for now */
    if (eth->h_proto != bpf_htons(ETH_P_IP)) {
        goto out_pass;
    }
    
    /* Bounds check: IP header */
    struct iphdr *iph = (void *)(eth + 1);
    if ((void *)(iph + 1) > data_end) {
        action = XDP_DROP;
        goto out;
    }
    
    __be32 src_ip = iph->saddr;
    __u32 pkt_len = (long)data_end - (long)data;
    
    /* Check blocklist */
    __u64 *blocked = bpf_map_lookup_elem(&blocklist, &src_ip);
    if (blocked) {
        stats->dropped_blocklist++;
        action = XDP_DROP;
        goto out;
    }
    
    /* Rate limiting */
    if (is_rate_limited(src_ip)) {
        stats->dropped_ratelimit++;
        /* Add to blocklist for next 60 seconds */
        __u64 ts = bpf_ktime_get_ns();
        bpf_map_update_elem(&blocklist, &src_ip, &ts, BPF_ANY);
        action = XDP_DROP;
        goto out;
    }
    
    /* Fast-path redirect for known pod destinations */
    __be32 dst_ip = iph->daddr;
    struct bpf_devmap_val *redir_dev = bpf_map_lookup_elem(&pod_redirect_map, &dst_ip);
    if (redir_dev) {
        /* Redirect to pod's interface without going through routing */
        action = bpf_redirect_map(&pod_redirect_map, dst_ip, XDP_PASS);
        goto out;
    }

out_pass:
    action = XDP_PASS;
out:
    if (action == XDP_PASS) {
        stats->rx_packets++;
        stats->rx_bytes += pkt_len;
        stats->passed++;
    }
    return action;
}

char _license[] SEC("license") = "GPL";
```

### 6.4 XDP and VXLAN Decapsulation

```c
/* XDP can decapsulate VXLAN before sk_buff allocation,
   dramatically reducing overhead for overlay networks */

SEC("xdp_vxlan_decap")
int xdp_vxlan(struct xdp_md *ctx) {
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;
    
    /* VXLAN packet structure:
       [ETH][IP (outer)][UDP (dst=4789)][VXLAN hdr][ETH (inner)][IP (inner)][payload] */
    
    struct ethhdr *outer_eth = data;
    if ((void *)(outer_eth + 1) > data_end) return XDP_PASS;
    if (outer_eth->h_proto != bpf_htons(ETH_P_IP)) return XDP_PASS;
    
    struct iphdr *outer_ip = (void *)(outer_eth + 1);
    if ((void *)(outer_ip + 1) > data_end) return XDP_PASS;
    if (outer_ip->protocol != IPPROTO_UDP) return XDP_PASS;
    
    struct udphdr *udp = (void *)outer_ip + (outer_ip->ihl * 4);
    if ((void *)(udp + 1) > data_end) return XDP_PASS;
    if (udp->dest != bpf_htons(4789)) return XDP_PASS; /* VXLAN port */
    
    /* 8-byte VXLAN header */
    __u8 *vxlan_hdr = (void *)(udp + 1);
    if ((void *)(vxlan_hdr + 8) > data_end) return XDP_PASS;
    
    /* Inner ethernet frame starts after VXLAN header */
    struct ethhdr *inner_eth = (void *)(vxlan_hdr + 8);
    if ((void *)(inner_eth + 1) > data_end) return XDP_PASS;
    
    /* Remove outer headers: ETH(14) + IP(20) + UDP(8) + VXLAN(8) = 50 bytes */
    if (bpf_xdp_adjust_head(ctx, 14 + 20 + 8 + 8) != 0)
        return XDP_PASS;
    
    /* Now ctx->data points to inner ethernet frame */
    /* Pass to kernel stack for inner packet processing */
    return XDP_PASS;
}
```

---

## 7. AF_XDP: Zero-Copy Userspace Networking

AF_XDP allows userspace programs to receive packets with zero kernel copies. The NIC DMA
writes directly to userspace memory. Used by DPDK alternatives and high-performance CNI
data planes.

### 7.1 AF_XDP Architecture

```
Traditional networking (multiple copies):
  NIC -> DMA -> kernel ring -> sk_buff -> socket recv buffer -> userspace
  (3-4 copies, kernel involvement at each step)

AF_XDP (zero copy with supported NICs):
  NIC -> DMA -> UMEM (userspace memory) -> userspace application
  (0 copies, kernel only sets up the mapping)

UMEM (User Memory):
  A region of memory registered with the kernel via setsockopt(SOL_XDP, XDP_UMEM_REG)
  Divided into chunks (frames) of equal size
  NIC DMA writes into UMEM chunks directly

AF_XDP Rings:
  FILL ring:    Userspace gives kernel free UMEM frames (for RX)
  RX ring:      Kernel tells userspace which frames have received packets
  TX ring:      Userspace puts frames to transmit
  COMPLETION ring: Kernel tells userspace TX frames are done (free to reuse)

                  Userspace                  Kernel
                  ---------                  ------
  
  RX path:
  FILL ring: push free frames --------->  kernel receives packet
  RX ring:   pull filled frames <--------- DMA to frame, update ring
  
  TX path:
  TX ring: push frames to send --------->  kernel dequeues, sends
  COMPLETION: pull sent frames <-----------  DMA complete, frame freed
```

### 7.2 AF_XDP C Implementation

```c
/* af_xdp_cni.c - zero-copy packet processing for CNI data plane
 * 
 * This shows how a high-performance CNI control plane can receive
 * packets at line rate using AF_XDP.
 *
 * Simplified for clarity. Full implementation uses libbpf + libxdp.
 */

#include <linux/if_xdp.h>
#include <sys/mman.h>
#include <sys/socket.h>
#include <linux/if_link.h>

#define FRAME_SIZE   4096
#define NUM_FRAMES   4096
#define BATCH_SIZE   64

struct xsk_umem_info {
    void    *buffer;
    size_t   size;
    struct xdp_umem_reg reg;
};

struct xsk_socket_info {
    int      fd;
    __u32    outstanding_tx;
    
    /* Ring pointers (mmaped from kernel) */
    struct xdp_ring_prod fill_ring;
    struct xdp_ring_cons rx_ring;
    struct xdp_ring_prod tx_ring;
    struct xdp_ring_cons comp_ring;
};

/* Setup UMEM: register userspace memory with kernel */
struct xsk_umem_info *xsk_alloc_umem(void) {
    struct xsk_umem_info *umem = calloc(1, sizeof(*umem));
    
    umem->size = NUM_FRAMES * FRAME_SIZE;
    
    /* Allocate hugepage-backed memory for DMA efficiency */
    umem->buffer = mmap(NULL, umem->size,
                        PROT_READ | PROT_WRITE,
                        MAP_PRIVATE | MAP_ANONYMOUS | MAP_HUGETLB,
                        -1, 0);
    if (umem->buffer == MAP_FAILED) {
        /* Fall back to regular pages */
        umem->buffer = mmap(NULL, umem->size,
                            PROT_READ | PROT_WRITE,
                            MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
    }
    
    /* Register with kernel */
    int sock = socket(AF_XDP, SOCK_RAW, 0);
    umem->reg.addr    = (unsigned long)umem->buffer;
    umem->reg.len     = umem->size;
    umem->reg.chunk_size = FRAME_SIZE;
    umem->reg.headroom   = 0;
    
    setsockopt(sock, SOL_XDP, XDP_UMEM_REG, &umem->reg, sizeof(umem->reg));
    
    /* Mmap the fill and completion rings */
    /* (offset from XDP_UMEM_PGOFF_FILL_RING, XDP_UMEM_PGOFF_COMPLETION_RING) */
    
    return umem;
}

/* Main receive loop: zero-copy, processes packets directly in UMEM */
void xsk_receive_loop(struct xsk_socket_info *xsk, struct xsk_umem_info *umem) {
    struct pollfd fds = { .fd = xsk->fd, .events = POLLIN };
    
    while (1) {
        poll(&fds, 1, -1);  /* wait for packets */
        
        /* Check RX ring for received packets */
        __u32 idx_rx = 0, idx_fq = 0;
        __u32 rcvd = xsk_ring_cons__peek(&xsk->rx_ring, BATCH_SIZE, &idx_rx);
        if (!rcvd) continue;
        
        /* Reserve fill ring space for next batch */
        xsk_ring_prod__reserve(&xsk->fill_ring, rcvd, &idx_fq);
        
        for (__u32 i = 0; i < rcvd; i++) {
            const struct xdp_desc *desc = xsk_ring_cons__rx_desc(&xsk->rx_ring, idx_rx++);
            
            /* desc->addr is offset into UMEM buffer */
            /* desc->len  is packet length */
            void *pkt = (char *)umem->buffer + desc->addr;
            
            /* Zero-copy packet access! No memcpy, no sk_buff */
            struct ethhdr *eth = pkt;
            /* Process packet directly in UMEM memory */
            process_packet(pkt, desc->len);
            
            /* Return frame to fill ring */
            *xsk_ring_prod__fill_addr(&xsk->fill_ring, idx_fq++) = desc->addr;
        }
        
        xsk_ring_cons__release(&xsk->rx_ring, rcvd);
        xsk_ring_prod__submit(&xsk->fill_ring, rcvd);
    }
}
```

---

## 8. Traffic Control (tc) Deep Dive: qdiscs, Classes, BPF Filters

tc is not just rate limiting. It's a full packet scheduling framework that eBPF-based CNI plugins
use as their primary hook point.

### 8.1 qdisc (Queuing Discipline) Architecture

```
Network Device TX path:

dev_queue_xmit(skb)
       |
       v
sch_handle_egress()          <- TC EGRESS hook (clsact qdisc)
       |
       v
qdisc->enqueue(skb, qdisc)  <- main qdisc (pfifo_fast, htb, fq_codel, etc.)
       |
       v
__qdisc_run()               <- dequeue loop
       |
       v
qdisc->dequeue(qdisc)
       |
       v
dev_hard_start_xmit()       <- driver

For INGRESS:
  packet arrives ->
  sch_handle_ingress()       <- TC INGRESS hook (clsact only)
  (ingress qdisc is special: no queuing, just filter+action)
```

### 8.2 clsact qdisc (Cilium's Main Attachment Point)

```
clsact is a special pseudo-qdisc added with:
  tc qdisc add dev eth0 clsact

It provides TWO independent hook points:
  1. INGRESS: before routing decision (after driver recv)
  2. EGRESS:  after routing decision (before actual send)

Both are "classless" — no queuing happens, only classify+act.

Attach BPF program to clsact:
  tc filter add dev lxcXXX ingress bpf direct-action obj cilium.o sec from-container
  tc filter add dev lxcXXX egress  bpf direct-action obj cilium.o sec to-container

"direct-action" mode:
  The BPF program return value IS the tc action (no separate action needed):
  TC_ACT_OK      -> pass packet through
  TC_ACT_SHOT    -> drop packet
  TC_ACT_REDIRECT -> redirect (after bpf_redirect() call)
  TC_ACT_UNSPEC  -> use parent qdisc's default action

Cilium's tc attachment (per interface on each node):
  lxcXXX (host-side veth for each pod):
    ingress BPF: from-container   <- packets leaving pod
    egress  BPF: to-container     <- packets entering pod
  
  eth0 (physical NIC):
    ingress BPF: from-netdev      <- packets from external network
    egress  BPF: to-netdev        <- packets to external network

  cilium_host:
    ingress BPF: from-host        <- packets from host process
```

### 8.3 Hierarchical Token Bucket (HTB) — Bandwidth Guarantees

```
HTB implements hierarchical fair queuing with guaranteed bandwidths.
Used by CNI bandwidth plugin for pod rate limiting.

HTB tree structure (example):

  root qdisc (htb)
     |
     +-- class 1:1  (rate=1Gbit, ceil=1Gbit)  <- root class
          |
          +-- class 1:10 (rate=100Mbit, ceil=200Mbit)  <- pod-A
          |    |
          |    +-- pfifo leaf qdisc
          |
          +-- class 1:20 (rate=200Mbit, ceil=500Mbit)  <- pod-B
               |
               +-- pfifo leaf qdisc

CNI bandwidth plugin implementation:
  1. Create HTB root qdisc:
     tc qdisc add dev lxcXXX root handle 1: htb default 30
  
  2. Add class with rate limit:
     tc class add dev lxcXXX parent 1: classid 1:1 htb \
         rate 100mbit ceil 100mbit burst 15k
  
  3. Filter to classify packets to class:
     tc filter add dev lxcXXX parent 1:0 protocol ip u32 match u32 0 0 \
         flowid 1:1
  
  4. Repeat for ingress:
     tc qdisc add dev lxcXXX handle ffff: ingress
     tc filter add dev lxcXXX parent ffff: protocol ip u32 match u32 0 0 \
         action police rate 100mbit burst 15k drop

The CNI bandwidth plugin automates this via the CNI_IFNAME from prevResult.
```

### 8.4 Token Bucket Filter (TBF) — Simple Rate Limiting

```
TBF is simpler than HTB: single rate limit for all traffic.

   Tokens added at rate R         Bucket depth B
   ──────────────────────>  [token bucket]  <── packet consumes tokens equal to its size
                                  |
                             If enough tokens: packet passes
                             If not enough tokens: packet queued or dropped

CNI TBF configuration:
  tc qdisc replace dev lxcXXX root tbf \
      rate 100mbit          \  # token fill rate (sustained rate)
      burst 15k             \  # burst bucket size (tokens)
      latency 400ms            # max latency (determines queue size)

For bidirectional limiting (ingress is trickier — needs ifb):
  # Create IFB (Intermediate Functional Block) device
  ip link add ifb0 type ifb
  ip link set dev ifb0 up
  
  # Redirect ingress to IFB (egress of IFB is easy to police)
  tc qdisc add dev lxcXXX handle ffff: ingress
  tc filter add dev lxcXXX parent ffff: protocol ip u32 match u32 0 0 \
      action mirred egress redirect dev ifb0
  
  # Rate limit on IFB egress
  tc qdisc add dev ifb0 root tbf rate 100mbit burst 15k latency 400ms
```

---

## 9. Open vSwitch (OVS) and OVN Internals

OVS is a high-performance virtual switch used by OpenShift's default CNI (OVN-Kubernetes)
and by OpenStack Neutron. Understanding OVS is essential for understanding how OpenShift
pod networking works.

### 9.1 OVS Architecture

```
OVS Component Architecture:

Userspace:
  +------------------+     +------------------+     +-----------------+
  |   ovs-vswitchd   |     |    ovsdb-server  |     |   ovn-controller|
  | (switch daemon)  |<--->| (config database)|<--->| (OVN agent)     |
  +------------------+     +------------------+     +-----------------+
           |                        ^                       |
           | Netlink/upcall         | OVSDB protocol        | Southbound DB
           v                        |                       v
  +------------------+     +------------------+     +-----------------+
  |  openvswitch.ko  |     |  ovn-northd      |     |  OVN Southbound |
  | (kernel module)  |     | (translator)     |     |  DB (etcd-like) |
  +------------------+     +------------------+     +-----------------+
  Flow table (kernel)              ^
                           +------------------+
                           |  OVN Northbound  |
                           |  DB (K8s state)  |
                           +------------------+
                                   ^
                           +------------------+
                           |  ovn-kubernetes  |
                           | (K8s controller) |
                           +------------------+

Data plane:
  Kernel module (openvswitch.ko):
    - Maintains OpenFlow flow tables
    - Fast path: lookup in kernel hash table, action directly
    - Slow path: upcall to ovs-vswitchd for unknown flows
    - Actions: output, drop, push/pop VLAN, push/pop MPLS,
               encap/decap (VXLAN/Geneve/GRE), NAT, set fields

  Megaflow cache:
    Instead of re-looking up the full OpenFlow pipeline for every packet,
    OVS caches the RESULT of the pipeline for a particular 5-tuple.
    First packet: slow path (kernel->userspace->action sequence)
    Subsequent packets of same flow: megaflow hit, direct kernel action
    -> ~10Mpps for cached flows, ~200Kpps for uncached
```

### 9.2 OpenFlow Pipeline in OVN-Kubernetes

```
OVN-Kubernetes programs OVS with OpenFlow rules via OVN.
The pipeline has multiple tables:

Table 0: Ingress Port Security (MAC/IP validation)
Table 8: L2 Lookup (destination MAC learning/routing)
Table 11: L3 Routing (IP routing between logical networks)
Table 16: Pre-stateful (conntrack zone setup)
Table 17: Stateful (NAT, load balancing)
Table 19: Egress Port Security

For a pod-to-service packet:
  1. Table 0: Ingress port security check (valid src MAC/IP)
  2. Table 16: Set conntrack zone (per logical network)
  3. ct_commit: commit to conntrack, DNAT service IP -> pod IP
  4. Table 11: Route to destination pod subnet
  5. Table 8: L2 output to destination logical port (pod's veth)
  6. Encapsulate if remote node (Geneve tunnel)
  7. Output to physical port

Packet encapsulation in OVN:
  Geneve header (instead of VXLAN):
  - Metadata: logical network ID, logical port ID, flags
  - These allow the remote node to make routing decisions
    without knowing the physical address of every pod
  - Flexible TLV options vs fixed VXLAN VNI
```

### 9.3 OVN-Kubernetes CNI Plugin

```
OVN-Kubernetes pod networking setup:

1. ovn-kubernetes DaemonSet runs on each node
   - Implements CNI (ovn-k8s-cni-overlay binary)
   - Watches pod creation events

2. On pod creation:
   ovn-cni:
     a. Create OVS internal port (pod's interface):
        ovs-vsctl add-port br-int <pod-iface> -- set Interface <pod-iface> type=internal
     b. Assign MAC address and IP (from OVN IPAM)
     c. Move interface into pod namespace
     d. Configure IP, routes in pod namespace
     e. Register logical port with OVN northbound DB
     
3. OVN northbound -> ovn-northd -> OVN southbound -> ovn-controller
   -> Programs OVS flow tables on this node
   -> Programs OVS flow tables on remote nodes (for this pod's subnet)

OVS bridge topology:
  br-int (integration bridge):
    OVS internal port: pod_if_<pod-id>  <- pod connects here
    OVS patch port: patch-br-int-to-br-local
    OVS patch port: patch-br-int-to-br-ex  (external traffic)
  
  br-local:
    Physical NIC: eth0
    Tunnel port: ovn-<remote-node>  (GENEVE to each remote node)
  
  br-ex:
    For north-south (internet) traffic
    Physical NIC or bond
```

---

## 10. WireGuard Kernel Internals and CNI Integration

WireGuard provides encrypted tunnels. Cilium uses WireGuard as its transparent encryption layer.

### 10.1 WireGuard Architecture

```
WireGuard design philosophy:
  - Kernel module (drivers/net/wireguard/)
  - Simple, auditable codebase (~4000 lines vs OpenVPN's ~100K)
  - Uses modern cryptography: Curve25519, ChaCha20, Poly1305, BLAKE2s, SipHash
  - No handshake states exposed to the network (silent discard of invalid packets)
  - Roaming: peers identified by public key, not IP

WireGuard components:
  wg0 (virtual network interface)
    - Appears as a normal netdev
    - TX: encrypt and encapsulate into UDP
    - RX: receive UDP, decrypt, inject into kernel as plain IP packets

Cryptokey Routing:
  - Each peer has a public key
  - Each peer is associated with allowed-ips (CIDRs)
  - Routing decision: which peer's key to use = which allowed-ip matches dst

Handshake (Noise_IKpsk2):
  Initiator                              Responder
  --------                               ---------
  ephemeral keypair (generate)
  msg1 = {ephemeral, AEAD(static, timestamp)}
         ─────────────────────────────────>
                                          verify static (public key match)
                                          msg2 = {ephemeral, AEAD(empty)}
         <─────────────────────────────────
  derive session keys
  (initiator_key, responder_key)
                                          derive session keys
  
  Keys rotate every 3 minutes (rekeying) or 2^64 messages
  Session keys never touch the wire (ECDH via Curve25519)

Packet structure (WireGuard data packet):
  +--------+--------+---------------------------+--------+
  | type   | key    |   nonce (counter)          |AEAD    |
  | (u32)  | index  |   (u64)                    |ciphert |
  +--------+--------+---------------------------+--------+
  Outer UDP: node1_ip:port -> node2_ip:port
```

### 10.2 Cilium + WireGuard Integration

```
Cilium WireGuard mode architecture:

Node 1 (10.0.1.10)                    Node 2 (10.0.1.20)
  Pod A: 10.244.1.5                     Pod B: 10.244.2.5
     |                                      |
  lxcXXX (tc BPF: policy + route)       lxcYYY (tc BPF)
     |                                      |
  cilium_host                           cilium_host
     |                                      |
  [WireGuard wg0]  ==================>  [WireGuard wg0]
  Public key: AAAA...                   Public key: BBBB...
  Listen: :51871                        Listen: :51871
     |                                      |
  eth0 (10.0.1.10)                     eth0 (10.0.1.20)
  
Setup by cilium-agent:
  1. Generate WireGuard keypair on node startup
  2. Store public key in Kubernetes Node annotation:
     node.cilium.io/wg-pub-key: AAAA...
  3. Watch other nodes' pub-keys
  4. Configure wg0 with peer entries for each remote node:
     wg set wg0 \
       peer BBBB... \
       allowed-ips 10.244.2.0/24 \  <- pod CIDR of remote node
       endpoint 10.0.1.20:51871
  5. Route pod traffic through wg0:
     ip route add 10.244.2.0/24 dev wg0

BPF + WireGuard interaction:
  Cilium's BPF programs run BEFORE WireGuard:
    Pod A sends to Pod B:
    1. tc egress BPF on lxcXXX:
       - Policy check: allowed?
       - Route: dst=10.244.2.5, nexthop=wg0
    2. Packet routed to wg0 (WireGuard interface)
    3. WireGuard: encrypt, encapsulate, send UDP
    4. eth0 transmits encrypted UDP
  
  Return path:
    1. eth0 receives UDP
    2. WireGuard: decrypt, inject as plain IP into kernel
    3. Packet arrives at cilium_host
    4. BPF routes to lxcYYY (Pod B's veth)
    5. tc ingress BPF on lxcYYY: policy check
    6. Delivered to Pod B
```

---

## 11. VRF, Policy Routing, and ECMP Deep Dive

### 11.1 VRF (Virtual Routing and Forwarding)

VRF provides per-interface routing tables at the kernel level. Used for advanced multi-tenant
CNI scenarios.

```
VRF architecture:

  VRF device "blue":
    enslaved interfaces: eth1, veth-pod-a, veth-pod-b
    routing table: 10 (dedicated to this VRF)
  
  VRF device "red":
    enslaved interfaces: eth2, veth-pod-c
    routing table: 20
  
  Packets arriving on eth1 -> routed in table 10 (not main table)
  Packets arriving on eth2 -> routed in table 20
  
  VRF kernel implementation:
    struct vrf_dev_info { u32 tb_id; }  <- routing table ID
    vrf_rcv() called for all ingress packets on enslaved interfaces
      -> calls vrf_fib_rule()
      -> sets skb->vrf_device
      -> forces route lookup in tb_id table
  
  Create VRF:
    ip link add blue type vrf table 10
    ip link set blue up
    ip link set eth1 master blue
    ip link set veth-pod-a master blue
  
  VRF + CNI (advanced multi-tenant):
    Each tenant's pods enslaved to their VRF
    Network policies enforced per-VRF
    Calico uses this for BGP VRF isolation

ip vrf exec:
  Run a process in a specific VRF context:
    ip vrf exec blue ping 10.1.1.1
  Equivalent to: bind-to-device before socket creation
```

### 11.2 Policy Routing Deep Dive

```
Linux policy routing: multiple routing tables selected by rules.

Routing table lookup order:
  1. ip rule list (rules in priority order)
  2. Each rule has: match conditions + routing table to use
  3. First matching rule -> look up packet in that table
  4. If route found: use it
  5. If not: continue to next rule

Default rules (always present):
  Priority 0:    lookup local (loopback, broadcast)
  Priority 32766: lookup main (normal routes)
  Priority 32767: lookup default (last resort, usually empty)

Calico policy routing (per-pod routing):
  # For each pod:
  ip rule add from 10.244.1.5 lookup 100 priority 10
  ip route add 169.254.1.1 dev cali3a7b2c scope link table 100
  ip route add default via 169.254.1.1 table 100
  
  # Effect:
  # Pod traffic (src=10.244.1.5) uses table 100
  # Table 100 routes everything via link-local gateway on pod's veth
  # Host answers ARP for 169.254.1.1 (proxy_arp)
  # Traffic enters host, gets normal routing to destination

Source-based routing for multi-NIC pods (Multus):
  # Pod has eth0 (10.244.1.5) and net1 (192.168.100.5)
  ip rule add from 10.244.1.5   lookup 10  # traffic from eth0 -> table 10
  ip rule add from 192.168.100.5 lookup 20  # traffic from net1 -> table 20
  
  ip route add default via 10.244.1.1 table 10
  ip route add default via 192.168.100.1 table 20
  
  # This prevents asymmetric routing: responses go back the same NIC they came in on

fwmark-based routing (Cilium):
  # iptables MARK or BPF mark:
  iptables -t mangle -A OUTPUT -m owner --uid-owner 1000 -j MARK --set-mark 0x1
  ip rule add fwmark 0x1 lookup 50
  ip route add default via 10.244.0.1 table 50
  # -> specific users/processes route differently
```

### 11.3 ECMP (Equal-Cost Multi-Path) Routing

```
ECMP allows traffic to be load-balanced across multiple next-hops.
Used by Calico for multi-path BGP and by Cilium in native routing mode.

Kernel ECMP (ip route add ... nexthop ... nexthop ...):
  ip route add 10.244.2.0/24 \
    nexthop via 192.168.1.20 weight 1 \
    nexthop via 192.168.1.21 weight 1 \
    nexthop via 192.168.1.22 weight 1

  Load balancing: kernel hashes (src_ip, dst_ip, protocol, src_port, dst_port)
  Result: same flow always goes to same nexthop (flow affinity)
  Different flows distributed across nexthops

Calico BGP ECMP:
  BGP advertises same prefix from multiple nodes
  -> Core router installs ECMP route to pod subnet
  -> Traffic load-balanced across nodes
  -> No single node bottleneck for ingress traffic

  # With FRR (Free Range Routing) / BIRD:
  router bgp 65001
    neighbor 192.168.1.1 remote-as 65000
    address-family ipv4 unicast
      network 10.244.1.0/24     <- this node's pod CIDR
      maximum-paths 8           <- ECMP up to 8 paths

Resilient ECMP (kernel 5.12+):
  Problem: adding/removing a nexthop reshuffles ALL flows (re-hashing)
  Solution: resilient hashing bucket table
    ip route add ... nhid 10 resilient buckets 512
  Effect: only flows that used the removed nexthop are affected
  Critical for node maintenance without disrupting all active connections

BFD (Bidirectional Forwarding Detection):
  Fast failure detection for BGP peers (< 100ms vs BGP timer ~30s)
  BIRD/FRR support BFD sessions with each peer
  On link failure: BFD detects, notifies BGP, BGP withdraws route
  -> ECMP converges in < 1 second instead of 30-90 seconds
```

---

## 12. IPv6 Dual-Stack CNI

Kubernetes supports IPv4/IPv6 dual-stack since 1.21 (stable). Every pod gets both an IPv4 and
IPv6 address.

### 12.1 Dual-Stack Architecture

```
Dual-stack pod networking:

  Pod IP: 10.244.1.5/24 (IPv4) AND fd00::1:5/48 (IPv6)
  
  Container namespace:
    lo:   127.0.0.1/8 + ::1/128
    eth0: 10.244.1.5/24 + fd00::1:5/48  (same interface!)
    
    Routes:
      0.0.0.0/0       via 10.244.1.1 dev eth0      # IPv4 default
      ::/0            via fd00::1:1   dev eth0      # IPv6 default
      10.244.1.0/24   dev eth0                      # IPv4 local
      fd00::1:0/48    dev eth0                      # IPv6 local

Kubernetes dual-stack config:
  kube-apiserver:  --service-cluster-ip-range=10.96.0.0/12,fd00:10:96::/112
  kube-controller: --cluster-cidr=10.244.0.0/16,fd00:10:244::/56
                   --node-cidr-mask-size-ipv4=24
                   --node-cidr-mask-size-ipv6=64

Node podCIDRs:
  node-1: spec.podCIDRs: ["10.244.1.0/24", "fd00:10:244:1::/64"]
  node-2: spec.podCIDRs: ["10.244.2.0/24", "fd00:10:244:2::/64"]

Services dual-stack:
  ClusterIP: 10.96.0.1 AND fd00:10:96::1
  -> kube-proxy/Cilium programs DNAT for BOTH addresses
```

### 12.2 CNI Dual-Stack Implementation

```
CNI dual-stack result format:
  {
    "ips": [
      { "address": "10.244.1.5/24",  "gateway": "10.244.1.1"  },
      { "address": "fd00::1:5/64",   "gateway": "fd00::1:1"   }
    ],
    "routes": [
      { "dst": "0.0.0.0/0", "gw": "10.244.1.1" },
      { "dst": "::/0",      "gw": "fd00::1:1"  }
    ]
  }

NDP (IPv6 Neighbor Discovery) vs ARP:
  IPv6 uses NDP (ICMPv6) instead of ARP:
    Neighbor Solicitation (multicast):  like ARP Request
    Neighbor Advertisement (unicast):  like ARP Reply
  
  Calico proxy NDP (like proxy ARP for IPv4):
    echo 1 > /proc/sys/net/ipv6/conf/cali3a7b2c/proxy_ndp
    ip -6 neigh add proxy fd00::1:5 dev eth0

  Cilium handles NDP in BPF:
    Answers Neighbor Solicitations for pod IPs in BPF program
    No userspace daemon, no NDP proxy sysctl needed

SLAAC (Stateless Address AutoConfiguration):
  IPv6 can auto-configure addresses from RA (Router Advertisement)
  In Kubernetes pods: disable SLAAC to avoid unexpected addresses
    echo 0 > /proc/sys/net/ipv6/conf/eth0/accept_ra
    echo 0 > /proc/sys/net/ipv6/conf/eth0/autoconf

ip6tables for dual-stack:
  Everything done for IPv4 with iptables must be mirrored for IPv6 with ip6tables
  Cilium: handles both in the same BPF program (AF_INET vs AF_INET6 dispatch)
```

---

## 13. SR-IOV, DPDK, and RDMA for High-Performance Pod Networking

For latency-sensitive workloads (telco 5G, financial trading, HPC), standard veth-based CNI
is too slow. SR-IOV, DPDK, and RDMA bypass most of the kernel.

### 13.1 SR-IOV (Single Root I/O Virtualization)

```
SR-IOV architecture:

Physical NIC (PF: Physical Function)
  |
  +-- VF 0 (Virtual Function 0) -> Pod A
  +-- VF 1 (Virtual Function 1) -> Pod B
  +-- VF 2 (Virtual Function 2) -> Pod C
  ...

Each VF appears as a separate PCI device to the OS.
VF can be moved into a pod's namespace directly.
No kernel network stack involvement for data path.

SR-IOV vs veth:
  veth:  CPU must copy every packet, netfilter, routing, tc overhead
  SR-IOV: NIC hardware handles queuing, DMA directly to pod's memory

SR-IOV setup:
  1. Enable VFs on PF:
     echo 4 > /sys/class/net/eth0/device/sriov_numvfs
  
  2. Verify VFs created:
     ip link show eth0
     # eth0: ... vf 0 MAC aa:bb:cc:dd:ee:01, vlan <none>, ...
  
  3. CNI moves VF into pod namespace:
     # Get VF PCI address
     PCI=$(ls /sys/class/net/eth0/device/virtfn0/net/ | head -1)
     VF_IFACE=$(ls /sys/class/net/eth0/device/virtfn0/net/)
     
     # Move VF interface into pod namespace
     ip link set $VF_IFACE netns /var/run/netns/<pod-id>
     
     # Configure in namespace
     ip netns exec <pod-id> ip link set $VF_IFACE name eth0
     ip netns exec <pod-id> ip addr add <pod-ip>/24 dev eth0
     ip netns exec <pod-id> ip link set eth0 up

SR-IOV CNI (sriov-cni):
  Plugin in /opt/cni/bin/sriov
  Reads SR-IOV device plugin allocated resource (Kubernetes Device Plugin framework)
  Assigns pre-allocated VF to pod
  Configures MAC, VLAN, trust mode on VF via netlink

Multus + SR-IOV (common pattern for telco):
  eth0: standard CNI (Flannel/Calico) for control plane traffic
  net1: SR-IOV VF for high-performance data plane
  net2: SR-IOV VF for another data plane interface
```

### 13.2 DPDK (Data Plane Development Kit)

```
DPDK bypasses the kernel network stack entirely:

Standard stack:     NIC -> kernel driver -> sk_buff -> netfilter -> socket -> app
DPDK:               NIC -> DPDK PMD (Poll Mode Driver) -> app (userspace)

Key DPDK concepts:
  PMD: Poll Mode Driver
    - Runs in dedicated core, busy-polling NIC ring
    - No interrupts (zero interrupt latency)
    - 0 syscalls per packet
  
  Hugepages: NIC DMA into 1GB or 2MB pages
    - Reduces TLB pressure
    - Contiguous memory for large rings
    
  RSS (Receive Side Scaling): per-core RX queues
    - NIC hashes flows to specific queues
    - Each core processes its own queue (no contention)

DPDK + CNI (vhost-user approach):
  Pod A <-> virtio <-> vhost-user socket <-> DPDK app (OVS-DPDK or VPP)
  
  OVS-DPDK:
    ovs-vswitchd uses DPDK for its data plane
    vSwitch ports: vhost-user sockets (one per pod)
    Pod connects via virtio-user library
    Throughput: 20-40 Mpps vs 2-5 Mpps for kernel OVS

  VPP (Vector Packet Processing) + CNI:
    VPP is a high-performance packet processing stack
    Used in Calico Enterprise, Cisco telco solutions
    Plugins: CNI plugin configures VPP interfaces for each pod

Intel DPDK performance (Xeon, 10GbE):
  64-byte packets:   ~14.88 Mpps (line rate)
  1518-byte packets: ~  812 Kpps (line rate)
  VPP/DPDK achieves >90% of line rate with complex pipelines
```

### 13.3 RDMA (Remote Direct Memory Access) for HPC Pods

```
RDMA allows pod to pod memory access WITHOUT CPU involvement:

Traditional network:
  Pod A: write to buffer -> OS copies -> NIC sends
  Pod B: NIC receives -> OS copies -> Pod B reads

RDMA:
  Pod A: post a work request to HCA (Host Channel Adapter)
  HCA: DMA from Pod A's memory, sends via InfiniBand/RoCE
  Pod B's HCA: receives, DMA directly to Pod B's registered memory
  No CPU, no OS copies, no system calls for data transfer

Latency: ~1µs vs ~100µs for TCP sockets

RDMA CNI (rdma-cni):
  Manages RDMA device assignment to pods
  Works with SR-IOV (RDMA VF) or dedicated HCA per pod
  Uses rdma tool for device management:
    rdma link show            <- show RDMA links
    rdma dev show             <- show RDMA devices
  
  macvlan or ipvlan + RDMA:
    Pod gets a macvlan interface for IP connectivity
    Pod also gets an RDMA device for direct memory transfers
    Two interfaces, two data paths, one pod
```

---

## 14. CNI in Non-Kubernetes Runtimes

CNI is not Kubernetes-specific. Understanding these use cases reveals how flexible the spec is.

### 14.1 Podman and Netavark

```
Podman (rootless and rootful containers):
  Legacy: used CNI plugins directly (same as Kubernetes)
  Modern (Podman 4.0+): uses Netavark (Rust) as default network stack

Netavark (github.com/containers/netavark):
  Written in Rust, replaces CNI for Podman
  NOT CNI-compatible (Podman has its own network abstraction)
  Uses the same kernel primitives (netlink, namespaces) as CNI plugins
  
  Key difference from CNI:
    Podman has long-running network state (unlike Kubernetes which is ephemeral)
    Netavark manages network configurations as a daemon-less state machine
    State stored in JSON files under /run/containers/networks/
  
  Aardvark-dns:
    Container DNS server (written in Rust)
    Provides name resolution for container-to-container communication
    Replaces dnsmasq

Podman still supports CNI plugins (legacy mode):
  podman network create --driver bridge  <- uses CNI bridge plugin
  /etc/cni/net.d/podman-<network>.conflist created
  
Rootless Podman and namespaces:
  No root -> cannot configure real network namespaces
  Uses slirp4netns or pasta for networking:
    slirp4netns: userspace TCP/IP stack, SLIRP protocol
    pasta: similar but newer, better performance
    
  slirp4netns architecture:
    Container namespace:
      tap0 interface (tun/tap)
      IP: 10.0.2.15/24 (hardcoded SLIRP address)
    Host process (slirp4netns):
      Reads from tap0 -> implements TCP/IP -> connects to host network
    No privileged network setup needed
    Performance: ~1 Gbps (vs ~10 Gbps for real networking)
```

### 14.2 Nomad CNI Integration

```
HashiCorp Nomad uses CNI for task group networking:

Nomad network modes:
  bridge: task group shares a network namespace (like K8s pod)
  host:   tasks use host networking
  none:   tasks have isolated but unconfigured namespaces

Nomad CNI invocation (similar to Kubernetes):
  1. Nomad allocates a task group
  2. Creates network namespace
  3. Reads CNI config from /opt/cni/config/
  4. Invokes CNI ADD with:
     CNI_CONTAINERID=<alloc-id>
     CNI_NETNS=/var/run/netns/<alloc-id>
     CNI_IFNAME=eth0
  5. Starts pause-like task (nomad_init) to hold namespace
  6. Other tasks in group join the namespace

Nomad bridge mode:
  Task group:
    - pause-like init process holds namespace
    - Application task: shares namespace
    - All tasks appear on same IP (like K8s pod)
  
  Port mapping in Nomad:
    Nomad maps service ports automatically:
    job spec: port "http" { to = 8080 }
    CNI portmap plugin adds DNAT rule: host_port -> 8080 in namespace

Consul Connect (service mesh) with Nomad CNI:
  Task group gets:
    eth0: application traffic (via CNI)
    envoy proxy sidecar: shares same namespace
    mTLS: all inter-service traffic encrypted by Envoy
  
  CNI plugin choice: same as K8s (Calico, Cilium work with Nomad)
```

### 14.3 Plain runc/crun with CNI

```
Using CNI without a higher-level runtime:

1. Create network namespace:
   ip netns add mycontainer

2. Create OCI bundle:
   mkdir -p /tmp/mycontainer/{rootfs,}
   # Create config.json with:
   {
     "linux": {
       "namespaces": [
         { "type": "network", "path": "/var/run/netns/mycontainer" }
       ]
     }
   }

3. Invoke CNI directly (from shell):
   CNI_COMMAND=ADD \
   CNI_CONTAINERID=mycontainer \
   CNI_NETNS=/var/run/netns/mycontainer \
   CNI_IFNAME=eth0 \
   CNI_PATH=/opt/cni/bin \
   /opt/cni/bin/bridge < /etc/cni/net.d/10-bridge.conf

4. Run container:
   runc run --bundle /tmp/mycontainer mycontainer

5. Cleanup:
   CNI_COMMAND=DEL \
   CNI_CONTAINERID=mycontainer \
   CNI_NETNS=/var/run/netns/mycontainer \
   CNI_IFNAME=eth0 \
   CNI_PATH=/opt/cni/bin \
   /opt/cni/bin/bridge < /etc/cni/net.d/10-bridge.conf
   
   ip netns del mycontainer

CNI reference library (Go):
  github.com/containernetworking/cni/libcni
  
  cniConfig := libcni.NewCNIConfig([]string{"/opt/cni/bin"}, nil)
  netConfig, _ := libcni.LoadConfList("/etc/cni/net.d/10-mynet.conflist")
  runtimeConf := &libcni.RuntimeConf{
      ContainerID: "mycontainer",
      NetNS:       "/var/run/netns/mycontainer",
      IfName:      "eth0",
  }
  result, _ := cniConfig.AddNetworkList(ctx, netConfig, runtimeConf)
```

---

## 15. Sandboxed Runtimes: gVisor and Kata Containers CNI

Sandboxed runtimes add an extra isolation layer. CNI must work with their unique architectures.

### 15.1 gVisor Networking

```
gVisor (runsc) is an application kernel in Go.
It intercepts all system calls, including network syscalls.

gVisor network stack options:

1. SANDBOX mode (default):
   Application -> gVisor syscall interceptor -> gVisor netstack (gonet)
   -> tun/tap device -> host kernel network stack -> real NIC
   
   gonet is a complete Go implementation of TCP/IP (from scratch):
   - No kernel TCP involvement for application traffic
   - Advantages: kernel TCP vulnerabilities don't affect sandboxed apps
   - Disadvantages: ~30-50% throughput vs native, no eBPF, no XDP
   
   CNI interaction:
     CNI runs NORMALLY (configures host namespace + tun/tap)
     gVisor connects to the tun/tap
     Traffic: App -> gonet -> tun/tap -> host veth -> bridge/routing

2. PASSTHROUGH mode (for performance):
   Application -> gVisor syscall interceptor -> host kernel network
   Direct: gVisor passes network syscalls to host kernel
   Advantage: full performance, eBPF works
   Disadvantage: less network isolation

gVisor CNI setup:
  RuntimeClass: gvisor
  Kubernetes creates pod -> containerd -> runsc (gVisor OCI runtime)
  runsc creates sandbox process -> sets up tun/tap in netns
  CNI (same as normal) configures eth0 (actually a tap) in pod netns
  gVisor's sandbox connects to tap device

Pod network namespace with gVisor:
  /var/run/netns/<pod-id>:
    lo:   configured by loopback CNI plugin
    eth0: this is a TAP device (not veth!)
          CNI configured it with IP/routes normally
          gVisor sandbox reads/writes raw Ethernet frames on it
```

### 15.2 Kata Containers CNI

```
Kata Containers runs each pod in a lightweight VM (QEMU, Cloud Hypervisor, Firecracker).
The VM needs to get the CNI-configured network "passed in".

Kata Containers network architecture:

Host namespace:
  eth0 (host NIC)
  cni0 (bridge)
  veth3a7b2c  <- CNI configures this as normal (IP: 10.244.1.5/24)

BUT the container is a VM! The VM needs network access.
Solution: macvtap or tcfilter

Option A: macvtap (default Kata):
  1. CNI plugin creates veth pair (as normal)
  2. Kata agent detects it's a VM container
  3. Creates a macvtap device on top of the veth
  4. QEMU attaches macvtap as a virtio-net device
  5. VM kernel sees virtio-net interface
  6. Kata agent configures IP/routes INSIDE the VM (same as CNI would in container)

  Host:
    veth3a7b2c (bridge port, no IP) 
      |
    macvtap3a7b2c
      |
    QEMU virtio-net
      |
    VM: eth0 (10.244.1.5/24) <- kata-agent configures this

Option B: VETH + TC redirect (Kata with CNI transparency):
  1. CNI configures the namespace normally
  2. Kata detects namespace, creates TC redirect rules
  3. All traffic from VM is redirected via TC to/from the pre-configured veth
  4. No re-configuration needed inside VM

Option C: SR-IOV VF direct assignment to VM:
  High-performance: VF passed directly into QEMU
  VM kernel uses VF driver directly
  No virtio, no host CPU involvement for data path

Firecracker + Kata:
  Firecracker uses tap devices (not macvtap)
  More restrictive security model (no arbitrary PCI passthrough)
  Kata creates tap, passes to Firecracker as network backend
  VM sees a virtio-net backed by the tap
  tap connects to host via bridge or macvlan
```

---

## 16. Windows CNI: HNS and HCN Architecture

### 16.1 Windows Container Networking

```
Windows does not have Linux network namespaces.
The equivalent is the Host Compute Network (HCN) and Host Network Service (HNS).

Windows container networking stack:

  Container (Hyper-V isolated or process isolated)
       |
  Virtual NIC (VMNIC) or process compartment NIC
       |
  Hyper-V Virtual Switch (vSwitch)  <- equivalent to Linux bridge
       |
  Physical NIC
       |
  External network

HNS (Host Network Service):
  - Windows service managing virtual network infrastructure
  - Creates: HNS networks (bridge, overlay, l2tunnel, l2bridge, nat, transparent)
  - Creates: HNS endpoints (equivalent to pod's network config)
  
  HNS REST API (localhost):
    POST /networks  <- create network
    POST /endpoints <- create endpoint (assign to container)
    GET  /networks/stats
  
  HCN (Host Compute Network) - modern API:
    Windows 1809+: HCN JSON schema replaces old HNS API
    Powershell: New-HnsNetwork, New-HnsEndpoint
    Go: hcsshim library

Windows CNI plugin (win-bridge, win-overlay):
  Instead of netlink: calls HNS API via named pipe \\.\pipe\HNS
  
  win-bridge plugin ADD flow:
  1. Check if HNS network "cbr0" exists, create if not
     hcsshim.NewHNSNetwork({Type: "L2Bridge", Subnets: [...]})
  2. Create HNS endpoint for the container:
     hcsshim.NewHNSEndpoint({VirtualNetwork: networkID, IPAddress: podIP})
  3. Attach endpoint to container namespace:
     endpoint.GrantEndpointAccess(containerID)
     endpoint.HotAttachEndpoint(containerID)
  4. Configure IP/routes inside container (via WinAPI, not netlink)

Overlay networking on Windows:
  win-overlay plugin: uses VFP (Virtual Filtering Platform) in Hyper-V
  VXLAN encapsulation handled by VFP at vSwitch level
  Flannel: deploys win-overlay CNI on Windows nodes
```

### 16.2 Windows + Linux Hybrid Clusters

```
Kubernetes supports mixed Linux/Windows node clusters:

  Linux nodes: use Linux CNI (Flannel, Calico, Cilium)
  Windows nodes: use Windows CNI (win-overlay, win-bridge)
  
  Same VXLAN VNI can span Linux and Windows nodes:
  - Flannel on Linux: kernel vxlan driver (flannel.1)
  - Flannel on Windows: VFP VXLAN implementation
  - VTEP MAC/IP discovery: same etcd/Kubernetes-backed mechanism

  Cross-OS pod communication:
    Linux pod (10.244.1.5) <-> Windows pod (10.244.2.5)
    VXLAN packet format is the same -> interoperable
    kube-proxy on Windows: HNS policies for service DNAT
```

---

## 17. Multi-Cluster Networking

### 17.1 Submariner Architecture

```
Submariner connects pod networks across multiple Kubernetes clusters.

Cluster A (10.244.0.0/16)         Cluster B (10.245.0.0/16)
  Gateway Node A                    Gateway Node B
  +------------------+              +------------------+
  | routeagent       |              | routeagent       |
  | (on each node)   |              | (on each node)   |
  +------------------+              +------------------+
  | submarine-gateway|              | submarine-gateway|
  | (cable driver)   |              | (cable driver)   |
  | IPsec/WireGuard  | <==========> | IPsec/WireGuard  |
  | tunnel           |              | tunnel           |
  +------------------+              +------------------+
         |                                  |
  [Broker Cluster] <-- CRD sync --> [Broker Cluster]
  (or same cluster)                 Endpoint, Cluster CRDs

Submariner cable drivers:
  libreswan:  IKEv2/IPsec (most compatible)
  WireGuard:  Modern, fast, simple
  VXLAN:      No encryption, low overhead
  
Route agent (DaemonSet on each node):
  - Manages routes to remote cluster pod CIDRs via gateway node
  - Adds host route: 10.245.0.0/16 via <gateway-node-IP>
  - Gateway node handles actual tunnel encapsulation
  
GlobalCIDR (for overlapping pod CIDRs):
  Problem: Cluster A and Cluster B both use 10.244.0.0/16
  Solution: GlobalCIDR (242.0.0.0/8 space)
    Cluster A pods: 10.244.x.x -> globalIP 242.0.x.x
    Cluster B pods: 10.244.x.x -> globalIP 242.1.x.x
  
  GlobalIP assignment:
    CRD GlobalIngressIP: maps clusterIP -> globalIP
    CRD GlobalEgressIP: maps pod -> globalIP for egress
  
  NAT at gateway: pods use global IPs when crossing clusters
```

### 17.2 Cilium Cluster Mesh

```
Cluster Mesh: Cilium's multi-cluster solution (no overlay tunnel)

Architecture:
  Cluster A                          Cluster B
  +-----------+                      +-----------+
  | cilium-   |                      | cilium-   |
  | agent     |                      | agent     |
  +-----------+                      +-----------+
       |                                  |
  +-----------+                      +-----------+
  | clustermesh|  <-- TLS connection --> | clustermesh|
  | apiserver  |  (Cluster Mesh API)  | apiserver  |
  +-----------+                      +-----------+
       |                                  |
  [etcd cluster A]                  [etcd cluster B]
  
  Shared services (example):
    Cluster A: Service "redis" -> endpoints [10.244.1.5, 10.244.1.6]
    Cluster B: Service "redis" -> endpoints [10.245.1.5]
    
    With Cluster Mesh + Global service:
    Any pod in A or B reaching "redis" can get any of the 3 endpoints
    Cross-cluster load balancing in eBPF (no extra hop)

Cluster Mesh network requirements:
  Pod CIDRs: MUST NOT overlap between clusters
  Direct IP routing between nodes across clusters (or VPN)
  No overlay needed (unlike Submariner)
  
Identity-aware security:
  Each cluster has a unique ClusterID (1-255)
  Pod security identity: (ClusterID << 16) | local-identity
  Policy: "allow cluster-A pods to reach cluster-B redis"
  Cilium encodes ClusterID in BPF identity tables
```

---

## 18. Conntrack: Deep Internals and Tuning

Connection tracking (conntrack) is a kernel subsystem that ALL iptables-based CNI plugins
depend on. Mistuning conntrack causes mysterious packet drops in large clusters.

### 18.1 Conntrack Data Structures

```c
/* net/netfilter/nf_conntrack_core.c */
struct nf_conn {
    /* Reference count */
    struct nf_conntrack ct_general;
    
    /* Spinlock for this connection */
    spinlock_t  lock;
    
    /* Connection tuple: (src_ip, dst_ip, src_port, dst_port, proto, zone) */
    struct nf_conntrack_tuple_hash tuplehash[IP_CT_DIR_MAX];
    /* [0] = ORIGINAL direction (initiator -> server)
       [1] = REPLY    direction (server -> initiator, with NAT applied) */
    
    /* State tracking */
    unsigned long status;   /* IPS_CONFIRMED, IPS_SRC_NAT, IPS_DST_NAT, ... */
    
    /* Timeout (when to expire this entry) */
    u32 timeout;
    
    /* Layer 4 protocol-specific state */
    union nf_conntrack_proto proto;
    /* For TCP: struct nf_ct_tcp_state { state, seen_guaranteed_rst, ... } */
    /* For UDP: timeout counter */
    
    /* NAT transformation (if applicable) */
    struct nf_ct_ext *ext;  /* extensions: NAT, helper, labels, etc. */
};

/* The global conntrack table: a hash table of all active connections */
struct nf_conntrack_net {
    struct hlist_nulls_head *hash;  /* hash table: tuple -> nf_conn */
    struct hlist_head        unconfirmed;
    struct hlist_head        dying;
    ...
};
```

### 18.2 Conntrack Lookup Path

```
Incoming packet: src=10.244.1.5:50000, dst=10.96.0.1:80

1. nf_conntrack_in() called at PREROUTING hook
2. nf_ct_get_tuple(): build 5-tuple from packet headers
3. __nf_conntrack_find_get(): hash lookup
   hash = hash_conntrack(tuple, zone) % hash_size
   Scan linked list at hash bucket
   Compare each entry's tuple with packet tuple
   
4a. FOUND (established connection):
    ct = found entry
    ct->timeout = now + proto_timeout  <- refresh timeout
    NF_CT_STAT_INC(net, found)
    return CT_NEW? No -> return CT_ESTABLISHED

4b. NOT FOUND (new connection):
    nf_ct_alloc_hashtable_entry()
    nf_conntrack_alloc()
    Add to unconfirmed list
    Return CT_NEW
    
5. If DNAT (kube-proxy):
   nf_nat_packet(): apply NAT transformation
   Modify skb: dst IP/port rewritten
   Record NAT in ct->ext (NAT extension)
   
6. ct_confirm() at POSTROUTING (after routing decision):
   Move from unconfirmed -> confirmed hash table
   Insert reply tuple (with NAT inverse mapping)
```

### 18.3 Conntrack Tuning for Large Clusters

```
Default conntrack limits (often too small for production):

/proc/sys/net/netfilter/nf_conntrack_max      = 131072
/proc/sys/net/netfilter/nf_conntrack_buckets  = 65536

For a cluster with:
  - 100 nodes
  - 100 pods/node = 10,000 pods
  - 1000 concurrent connections/pod = 10M connections
  -> Need nf_conntrack_max >= 10,000,000

Tuning:
  # Scale table size (must be power of 2, min 1/8 of max)
  sysctl -w net.netfilter.nf_conntrack_max=2000000
  sysctl -w net.netfilter.nf_conntrack_buckets=500000
  
  # Each entry uses ~300-400 bytes
  # 2M entries = ~600-800MB RAM
  
  # Aggressive timeout reduction for Kubernetes:
  sysctl -w net.netfilter.nf_conntrack_tcp_timeout_established=86400  # was 432000
  sysctl -w net.netfilter.nf_conntrack_tcp_timeout_time_wait=30       # was 120
  sysctl -w net.netfilter.nf_conntrack_tcp_timeout_close_wait=15      # was 60
  sysctl -w net.netfilter.nf_conntrack_udp_timeout=30                 # was 30
  sysctl -w net.netfilter.nf_conntrack_udp_timeout_stream=180         # was 180

Conntrack zones (for overlay networks):
  Problem: Same 5-tuple can exist in outer AND inner packet
  (Outer: node1:4789 -> node2:4789; Inner: pod-A:50000 -> pod-B:80)
  Both share same conntrack table -> collision!
  
  Solution: conntrack zones (each zone has independent tuple space)
  Flannel/Calico assign different zones to outer and inner:
    iptables -t raw -A PREROUTING -i flannel.1 -j CT --zone 1
    iptables -t raw -A OUTPUT -o flannel.1 -j CT --zone 1
  
  Zone 0: host traffic (normal)
  Zone 1: VXLAN inner traffic

Conntrack and Cilium:
  Cilium replaces conntrack with its own BPF CT tables:
    cilium_ct4_global: IPv4 connections
    cilium_ct6_global: IPv6 connections
  
  Cilium disables conntrack for pod traffic:
    iptables -t raw -A PREROUTING -i lxc+ -j NOTRACK
    iptables -t raw -A OUTPUT -o lxc+ -j NOTRACK
  
  BPF CT advantages:
    - Per-CPU LRU maps (no lock contention)
    - Automatic expiry (LRU eviction vs kernel GC timer)
    - ~3x faster lookup for large connection tables
    - No /proc/sys tuning needed

Conntrack table exhaustion symptoms:
  dmesg: "nf_conntrack: table full, dropping packet"
  Mysterious TCP connection failures, DNS resolution failures
  Check: cat /proc/sys/net/netfilter/nf_conntrack_count
  Monitor: conntrack -L | wc -l
```

---

## 19. IPVS Mode: kube-proxy and CNI Interaction

kube-proxy can run in three modes: iptables (default), IPVS, and eBPF (via Cilium). IPVS mode
has important interactions with CNI.

### 19.1 IPVS Architecture

```
IPVS (IP Virtual Server) is a kernel L4 load balancer:
  - Part of netfilter infrastructure
  - Hash table for VIP (Virtual IP) -> real servers
  - Much faster than iptables for large service counts
  - Supports multiple scheduling algorithms: rr, wrr, lc, wlc, sh, sed, nq

iptables for 10,000 services: O(n) rule traversal per packet
IPVS for 10,000 services: O(1) hash lookup per packet

IPVS data structures:
  ip_vs_service (VIP:port):
    - ClusterIP:port -> list of ip_vs_dest (real servers/pod IPs)
    - scheduler (round-robin, least-conn, etc.)
    - connection table (for persistence/affinity)
  
  ip_vs_dest (real server):
    - Pod IP:port
    - weight
    - active/inactive connections count

How kube-proxy IPVS mode works:
  1. For each Service: create IPVS virtual service (VIP:port)
  2. For each Endpoint: add IPVS real server (pod-IP:port)
  3. Add dummy interface kube-ipvs0 with ALL ClusterIPs
     (so kernel accepts packets for service IPs on this node)
     ip addr add 10.96.0.1/32 dev kube-ipvs0
     ip addr add 10.96.0.2/32 dev kube-ipvs0
     ... (one per service)
  4. iptables rules: minimal (only for MASQUERADE and node-port handling)
  
  Packet flow:
    Pod -> ClusterIP:80 ->
    -> Routed to kube-ipvs0 (local delivery) ->
    -> ip_vs_in(): DNAT (ClusterIP:80 -> pod-IP:8080) ->
    -> Forwarded to pod-IP:8080 (actual pod)
    -> SNAT on return path (connection tracking via ip_vs_conn)
```

### 19.2 IPVS + CNI Interaction

```
IPVS and CNI interact in these ways:

1. kube-ipvs0 interface:
   kube-proxy creates this dummy interface
   CNI doesn't manage it (kube-proxy manages it directly)
   All ClusterIPs added to it -> kernel routes service traffic to local
   
2. Conntrack interaction:
   IPVS has its own connection table (ip_vs_conn hash)
   Conntrack ALSO tracks the DNAT'd connection
   Double tracking: ip_vs_conn + nf_conn
   
   ip_vs_conn: remembers which pod was selected for which client
   nf_conn: tracks the post-DNAT TCP state
   
   Tuning: ip_vs_conn table size:
     sysctl -w net.ipv4.vs.conn_reuse_mode=0  # performance
     sysctl -w net.ipv4.vs.expire_nodest_conn=1
   
3. NodePort interaction:
   kube-proxy adds iptables rule for NodePort:
     iptables -t nat -A PREROUTING -p tcp --dport 30080 -j MARK --set-mark 0x4000
     iptables -t nat -A PREROUTING -m mark --mark 0x4000 -j KUBE-CLUSTER-IP
   Then IPVS handles the DNAT
   
4. CNI MASQUERADE + IPVS:
   If IPVS performs DNAT, the packet's source IP is the pod IP
   If pod is on a different node, the receiving node must route back
   Without MASQUERADE: return path goes direct (may work or not depending on CNI)
   With MASQUERADE: return path always goes through originating node
   
   Cilium with IPVS: Cilium handles LB in BPF, bypasses IPVS entirely
     Set: kube-proxy-replacement: strict in Cilium config
     This makes Cilium fully replace kube-proxy (IPVS not used)
```

---

## 20. Network Policy: Implementation Internals

Kubernetes NetworkPolicy is implemented by the CNI plugin, NOT by Kubernetes itself.

### 20.1 NetworkPolicy Spec to Kernel Rules

```
NetworkPolicy example:
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata: { name: "allow-only-frontend", namespace: "default" }
  spec:
    podSelector:
      matchLabels: { app: backend }
    ingress:
    - from:
      - podSelector:
          matchLabels: { app: frontend }
      ports:
      - protocol: TCP
        port: 8080
    policyTypes: ["Ingress"]

Meaning:
  - Pods with label app=backend:
    - Allow ingress TCP:8080 from pods with label app=frontend
    - Deny all other ingress (implicit)
    - No egress restriction (Egress not in policyTypes)

How Calico implements this (Felix):
  1. Watch NetworkPolicy CRD
  2. Translate to Felix internal policy model
  3. Program iptables rules per-pod:
  
  For backend pod (IP 10.244.1.5) with cali3a7b2c veth:
  
  Chain cali-fw-cali3a7b2c (forward into pod):
    -m comment --comment "Policy: allow-only-frontend"
    -m set --match-set cali40s:frontend src     <- source IP set for frontend pods
    -p tcp --dport 8080 -j ACCEPT
    -j DROP                                      <- default deny
  
  IP sets (ipset):
    cali40s:frontend = {10.244.2.3, 10.244.2.7}   <- all frontend pod IPs
    
    Updated dynamically as pods are added/removed:
    ipset add cali40s:frontend 10.244.2.3
    ipset del cali40s:frontend 10.244.2.3   (on pod removal)
  
  ipset lookup: O(1) hash lookup (vs O(n) iptables rule traversal)
  For 1000 frontend pods: ipset is identical speed to 1 pod
  For 1000 frontend pods with iptables rules: O(1000) per packet
```

### 20.2 Cilium Network Policy (Identity-Based)

```
Cilium uses IDENTITY instead of IP addresses for policy:

Identity = label hash (e.g., app=frontend, namespace=default)
         -> numeric identity (e.g., 12345)
         -> applies to all pods with that label set

Advantages:
  - Policy evaluation is O(1) regardless of number of pods
  - Handles pod restart (IP changes) transparently (identity doesn't change)
  - Works across nodes without IP coordination

Identity assignment:
  1. cilium-agent watches Kubernetes for pod label changes
  2. Computes identity from label set (hash)
  3. Stores in cilium_ipcache BPF map: IP -> identity
  4. Propagates identity to all nodes (via KVStore/CRD)

Policy enforcement in BPF:
  /* On egress from pod A (src=12345) to pod B's IP: */
  struct remote_endpoint_info *info = 
      bpf_map_lookup_elem(&cilium_ipcache, &dst_ip);
  if (!info) return DROP;  /* unknown destination */
  
  __u32 dst_identity = info->sec_label;  /* e.g., 67890 (backend identity) */
  
  /* Check policy: can identity 12345 reach identity 67890 on port 8080? */
  struct policy_entry *policy =
      cilium_policy_lookup(src_identity=12345, dst_identity=67890, 
                           protocol=TCP, port=8080);
  
  if (!policy || policy->deny) return DROP;
  return REDIRECT;

DNS-aware network policy (Cilium L7):
  Allow: pods can reach *.api.example.com on port 443
  
  Cilium intercepts DNS at socket level (BPF_PROG_TYPE_SOCK_OPS)
  DNS resolver queries -> Cilium observes replies -> extracts IPs -> updates policy
  Policy: instead of "allow IP X", allow "IPs that DNS resolved to api.example.com"
  Dynamic: new IPs for the same domain are automatically allowed
```

### 20.3 Calico eBPF Network Policy

```
Calico eBPF mode (alternative to iptables):
  Same policy model, but implemented in BPF instead of iptables

Calico eBPF programs:
  tc filter: attached to each workload interface (cali+)
  Programs:
    from_workload: BPF for egress from pod
    to_workload:   BPF for ingress to pod
    from_host:     BPF for traffic from host
    to_host:       BPF for traffic to host

Policy representation in BPF maps:
  /* jump map: endpoint ID -> policy program */
  cali_policy_jump_map[endpoint_id] = bpf_prog_fd
  
  /* Each endpoint gets its own compiled policy program */
  /* Policy update = recompile and reload BPF program */
  /* No rule traversal: policy is compiled into the program itself */

Calico eBPF vs iptables:
  iptables: O(n) rules, conntrack overhead, global lock
  eBPF:     O(1) compiled policy, no global lock, optional conntrack bypass
  
  Performance: 2-3x throughput improvement for policy-heavy workloads
```

---

## 21. MTU, PMTU Discovery, and MSS Clamping

MTU misconfiguration is one of the most common causes of mysterious connectivity failures in
Kubernetes clusters.

### 21.1 MTU Layering in CNI

```
MTU layers (each adds overhead reducing effective payload):

Physical Ethernet:        1500 bytes MTU
  - 14 bytes Ethernet header
  - 4  bytes VLAN tag (if used)
  Net payload: 1500 bytes

IPv4 header:              20 bytes
IPv6 header:              40 bytes
TCP header:               20-60 bytes
UDP header:               8 bytes

Overlay headers (cumulative):
  VXLAN:   14 (inner ETH) + 8 (VXLAN) + 8 (UDP) + 20 (outer IP) = 50 bytes
  GENEVE:  14 (inner ETH) + 8+ (GENEVE) + 8 (UDP) + 20 (outer IP) = 50+ bytes
  WireGuard: 20 (IP) + 8 (UDP) + 32 (nonce) + 16 (AEAD tag) = 76 bytes
  IPsec ESP: 20 (IP) + 8 (ESP) + 16 (IV) + 12 (HMAC) + 2 (pad) = 58 bytes
  IP-in-IP:  20 bytes outer IP header = 20 bytes
  GRE:       20 (IP) + 4+ (GRE) = 24+ bytes

Recommended pod MTU by overlay type:
  No overlay (direct routing): 1500 (or match physical MTU)
  VXLAN:         1450 (1500 - 50)
  GENEVE:        1450 (1500 - 50, variable)
  WireGuard:     1420 (1500 - 76 - 4 safety margin)
  WireGuard+VXLAN: 1370 (1500 - 76 - 50 - 4)
  IPsec:         1422 (1500 - 58 - 20)
  IP-in-IP:      1480 (1500 - 20)

CNI MTU configuration:
  Flannel:
    /run/flannel/subnet.env: FLANNEL_MTU=1450
    Flannel CNI plugin reads this and sets veth MTU accordingly
  
  Calico:
    Felix auto-detects physical MTU and sets pod MTU = phys_MTU - overlay_overhead
    calico_mtu_iface_pattern: pattern to select physical interface
    veth_mtu: auto, or explicit override in FelixConfiguration CRD
  
  Cilium:
    auto-direct-node-routes: true -> no overlay -> 1500 MTU pods
    tunnel: vxlan -> sets pod MTU to physical_mtu - 50
    tunnel: geneve -> same
    Can query from node: ip link show eth0 | grep mtu
```

### 21.2 PMTU Discovery

```
PMTU (Path MTU Discovery): mechanism to discover the smallest MTU on a path.

How it works (RFC 1191):
  1. Sender sends a large packet (DF bit set: Don't Fragment)
  2. Intermediate router: packet > its MTU
     -> Drops packet
     -> Sends ICMP "Fragmentation Needed, MTU=X" back to sender
  3. Sender reduces packet size to X
  4. Repeat until no more ICMP messages (found minimum MTU)

PMTU in Kubernetes clusters:
  Problem: ICMP "Fragmentation Needed" messages often BLOCKED by firewalls/security groups
  -> Silent drops for packets > overlay MTU
  -> TCP connections time out mysteriously for large payloads
  -> Symptoms: small requests work, large requests fail, ping works, curl fails
  
  Diagnose:
    # From pod, try sending large packets with DF bit:
    ping -M do -s 1400 10.244.2.5      <- if fails, MTU issue
    ping -M do -s 1400 google.com      <- if fails but above works, PMTU blackhole
    
    # Check effective PMTU on a socket:
    tracepath 10.244.2.5               <- shows per-hop MTU

ICMP black hole detection (kernel mechanism):
  net.ipv4.tcp_mtu_probing = 0  (disabled, default)
  net.ipv4.tcp_mtu_probing = 1  (enabled when black hole detected)
  net.ipv4.tcp_mtu_probing = 2  (always enabled)
  
  With probing=1: TCP detects PMTU black hole and reduces MSS

When to enable in Kubernetes:
  sysctl -w net.ipv4.tcp_mtu_probing=1
  -> Recommended for cloud environments where ICMP may be filtered
```

### 21.3 MSS Clamping

```
MSS (Maximum Segment Size): TCP option that limits segment size.
MSS clamping adjusts MSS during TCP handshake to prevent oversized segments.

TCP MSS exchange:
  Client SYN:  MSS=1460 (1500 - 20 IP - 20 TCP)
  Server SYN-ACK: MSS=1460
  
  If overlay reduces effective MTU to 1450:
  Without clamping: segments can be 1460 bytes -> 1510 bytes with headers > 1450 MTU -> dropped
  With clamping: iptables/nftables rewrites MSS to 1410 during SYN/SYN-ACK:
    iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN \
        -j TCPMSS --clamp-mss-to-pmtu
  
  Or explicit value:
    iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN \
        -j TCPMSS --set-mss 1410

Flannel MSS clamping:
  "ipMasq": true in conflist -> flannel adds MSS clamp rule automatically
  Also: "mtu": 1450 in subnet.env sets interface MTU (kernel enforces fragmentation)

Cilium MSS clamping (BPF):
  Cilium's TC BPF programs rewrite MSS for SYN packets:
  enable-bpf-masquerade: true -> includes MSS clamping in masquerade BPF
  
  BPF MSS rewrite:
  /* In TC BPF program for egress: */
  if (is_tcp_syn(skb)) {
      struct tcphdr *tcp = ...;
      /* Find TCP MSS option */
      __u8 *opt = (void *)tcp + sizeof(*tcp);
      while (opt < data_end && *opt != TCPOPT_EOL) {
          if (*opt == TCPOPT_MSS && opt[1] == TCPOLEN_MSS) {
              __u16 mss = (opt[2] << 8) | opt[3];
              if (mss > MAX_MTU - 40) {
                  __u16 new_mss = MAX_MTU - 40;
                  opt[2] = new_mss >> 8;
                  opt[3] = new_mss & 0xff;
                  /* Recalculate TCP checksum */
                  bpf_l4_csum_replace(skb, ...);
              }
          }
          opt += opt[1];
      }
  }
```

---

## 22. CNI Security: Privilege Model and Attack Surface

### 22.1 CNI Plugin Privilege Requirements

```
CNI plugins run with elevated privileges. Understanding what they need (and don't need)
helps minimize attack surface.

Required capabilities for CNI plugins:
  CAP_NET_ADMIN:   create/modify network interfaces, set IPs, routing
  CAP_NET_RAW:     send raw packets (some plugins for ARP/NDP)
  CAP_SYS_ADMIN:   mount operations (for namespace manipulation)
  CAP_SYS_PTRACE:  read /proc/<pid>/ns/net (in some implementations)

CNI plugin as DaemonSet:
  Always runs as privileged: true
  Mounts:
    /proc    (read network namespaces of other processes)
    /run     (CNI state files, network namespace files)
    /opt/cni (drop plugin binaries)
    /etc/cni (write config files)
    /sys     (configure sysctl, manage network devices)
  
  This is why CNI DaemonSets are high-value attack targets.

Attack scenarios:
  1. Malicious CNI plugin: DaemonSet runs with host privileges
     -> Can modify routing tables of ALL namespaces
     -> Can sniff ALL pod traffic via bridge/veth
     -> Can inject packets into any pod
     -> Full node compromise

  2. CNI binary replacement:
     Attacker writes to /opt/cni/bin/
     Next pod creation executes attacker's binary with host privileges
     Mitigation: Read-only filesystem, binary signing verification
  
  3. Config file poisoning:
     Attacker writes to /etc/cni/net.d/
     Next pod creation uses attacker's CNI config
     Mitigation: Admission webhook validating CNI config

  4. Race condition: TOCTOU on network namespace path
     Between namespace creation and CNI invocation:
     Another process could mount something at the netns path
     Mitigation: Open fd first, then validate it's a netns
```

### 22.2 Securing the CNI Data Path

```
Defense-in-depth for CNI:

1. Mutual TLS between pods (service mesh):
   Istio/Linkerd inject sidecar proxy (Envoy/linkerd-proxy)
   All inter-pod traffic encrypted even within cluster
   CNI handles L3 routing; mTLS handles L7 security
   
2. Network policies (mandatory baseline):
   Default deny all ingress:
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata: { name: deny-all }
   spec:
     podSelector: {}  # applies to ALL pods in namespace
     policyTypes: ["Ingress", "Egress"]
   # No ingress/egress rules = deny all
   
3. Cilium's encryption:
   WireGuard: all node-to-node traffic encrypted
   IPsec: alternative, hardware offload available
   
4. Cilium Tetragon (security observability):
   eBPF-based security monitoring
   Detects: privilege escalation, network anomalies, file access
   Can KILL processes in real-time based on policy
   
5. Preventing pod IP spoofing:
   "Source IP validation" in network policy
   Calico: iptables rule validates src IP matches pod IP
     iptables -A cali-from-wl-dispatch -i cali+ \
         ! --source <expected-pod-CIDR> -j DROP
   
   BPF approach (Cilium):
     TC ingress program on lxcXXX:
     if (skb->src_ip != expected_pod_ip) return DROP;

6. eBPF program pinning and integrity:
   Cilium pins all BPF programs to /sys/fs/bpf/
   Monitor for unauthorized changes: inotify on /sys/fs/bpf/
   BPF program IDs logged by cilium-agent on load
```

---

## 23. Observability: eBPF-Based Network Telemetry

### 23.1 Hubble (Cilium's Observability Layer)

```
Hubble architecture:

Cilium BPF programs:
  TC programs emit flow events to perf ring buffers / ring buffers
  
  struct flow_event {
      __u32 src_ip;
      __u32 dst_ip;
      __u16 src_port;
      __u16 dst_port;
      __u8  protocol;
      __u8  verdict;      /* forwarded, dropped, redirected */
      __u32 dst_identity; /* Cilium security identity */
      char  drop_reason[32];
  };
  
  bpf_ringbuf_submit(&flow, 0);

Hubble Daemon (per node):
  Reads BPF ring buffers from all Cilium programs
  Enriches: IP -> pod name (via Kubernetes API), port -> service name
  Stores: in-memory ring buffer (configurable retention, e.g., 4096 flows)
  Exposes: gRPC API for Hubble CLI and UI

Hubble Relay (cluster-wide):
  Connects to all per-node Hubble daemons
  Aggregates flow data
  Provides single gRPC endpoint for cluster-wide visibility

Hubble queries:
  hubble observe --namespace default --verdict DROPPED
  hubble observe --from-pod default/frontend --to-pod default/backend
  hubble observe --protocol TCP --port 5432   # PostgreSQL traffic
  
  # Real-time L7 HTTP visibility (Cilium L7 policy):
  hubble observe --protocol HTTP --http-method GET

DNS observability:
  Cilium intercepts DNS at socket level (BPF_PROG_TYPE_SOCK_ADDR)
  Every DNS query/response logged with:
    - Querying pod
    - DNS name queried
    - Response IPs
    - Latency
  hubble observe --protocol DNS
```

### 23.2 Falco for CNI Security

```
Falco uses eBPF (or kernel module) to detect runtime security threats
including network anomalies:

Relevant Falco rules for CNI security:

- Network namespace manipulation:
  condition: evt.type = unshare and evt.arg.flags contains CLONE_NEWNET
  output: "Network namespace created by non-container process (user=%user.name cmd=%proc.cmdline)"

- Unexpected outbound connection from pod:
  condition: outbound and not proc.name in (expected_network_processes)
  output: "Unexpected outbound connection (pod=%k8s.pod.name dest=%fd.rip:%fd.rport)"

- Container escaping via network:
  condition: container.id != host and
             (fd.sip = "169.254.169.254" or fd.sip = "169.254.0.0/16")
  output: "Potential metadata service access from container (SSRF?)"

Falco integration with CNI:
  Falco doesn't replace CNI, it watches it
  BPF probe: sys_enter_connect, sys_enter_bind, sys_enter_accept
  Enriches with Kubernetes pod labels, namespace, container name
```

---

## 24. Full Rust CNI Plugin Implementation

```rust
// cni_plugin_rust/src/main.rs
// Complete Rust CNI plugin: handles ADD, DEL, CHECK, VERSION
// Uses netlink-sys and rtnetlink crates for kernel operations
//
// Cargo.toml dependencies:
// [dependencies]
// serde = { version = "1", features = ["derive"] }
// serde_json = "1"
// rtnetlink = "0.13"
// netlink-packet-route = "0.17"
// tokio = { version = "1", features = ["full"] }
// futures = "0.3"
// ipnetwork = "0.20"
// nix = { version = "0.27", features = ["sched", "mount", "net"] }
// anyhow = "1"
// thiserror = "1"

use std::collections::HashMap;
use std::env;
use std::io::{self, Read, Write};
use std::net::Ipv4Addr;
use std::os::unix::io::RawFd;

use anyhow::{anyhow, Context, Result};
use futures::stream::TryStreamExt;
use ipnetwork::Ipv4Network;
use nix::sched::{setns, CloneFlags};
use rtnetlink::{new_connection, Handle, IpVersion, NetworkNamespace};
use serde::{Deserialize, Serialize};
use tokio::runtime::Runtime;

// ===================== CNI Data Types =====================

#[derive(Debug, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
struct NetConf {
    cni_version: String,
    name: String,
    #[serde(rename = "type")]
    plugin_type: String,
    bridge: Option<String>,
    is_gateway: Option<bool>,
    ip_masq: Option<bool>,
    mtu: Option<u32>,
    ipam: Option<HashMap<String, serde_json::Value>>,
    prev_result: Option<serde_json::Value>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct CniResult {
    cni_version: String,
    interfaces: Vec<CniInterface>,
    ips: Vec<CniIP>,
    routes: Vec<CniRoute>,
    dns: CniDNS,
}

#[derive(Debug, Serialize)]
struct CniInterface {
    name: String,
    mac: String,
    sandbox: String,
}

#[derive(Debug, Serialize)]
struct CniIP {
    address: String,   // CIDR: "10.244.0.5/24"
    gateway: String,
    interface: u32,
}

#[derive(Debug, Serialize)]
struct CniRoute {
    dst: String,
    gw: Option<String>,
}

#[derive(Debug, Serialize, Default)]
struct CniDNS {
    nameservers: Vec<String>,
    search: Vec<String>,
}

#[derive(Debug, Serialize)]
struct CniError {
    cni_version: String,
    code: u32,
    msg: String,
    details: String,
}

impl CniError {
    fn new(code: u32, msg: impl Into<String>, details: impl Into<String>) -> Self {
        CniError {
            cni_version: "1.0.0".to_string(),
            code,
            msg: msg.into(),
            details: details.into(),
        }
    }
}

// ===================== CNI Commands =====================

struct CniArgs {
    command: String,
    container_id: String,
    netns: String,
    ifname: String,
    path: Vec<String>,
    args: HashMap<String, String>,
}

fn parse_cni_args() -> Result<CniArgs> {
    let command = env::var("CNI_COMMAND")
        .context("CNI_COMMAND not set")?;
    let container_id = env::var("CNI_CONTAINERID")
        .context("CNI_CONTAINERID not set")?;
    let netns = env::var("CNI_NETNS").unwrap_or_default();
    let ifname = env::var("CNI_IFNAME")
        .context("CNI_IFNAME not set")?;
    let path_str = env::var("CNI_PATH")
        .context("CNI_PATH not set")?;
    let path = path_str.split(':').map(String::from).collect();
    
    // Parse CNI_ARGS: "K8S_POD_NAME=my-pod;K8S_POD_NAMESPACE=default"
    let args_str = env::var("CNI_ARGS").unwrap_or_default();
    let args: HashMap<String, String> = args_str
        .split(';')
        .filter_map(|kv| {
            let mut parts = kv.splitn(2, '=');
            let k = parts.next()?.to_string();
            let v = parts.next()?.to_string();
            Some((k, v))
        })
        .collect();
    
    Ok(CniArgs { command, container_id, netns, ifname, path, args })
}

fn read_config() -> Result<NetConf> {
    let mut buf = String::new();
    io::stdin().read_to_string(&mut buf)?;
    let conf: NetConf = serde_json::from_str(&buf)
        .context("failed to parse CNI config JSON")?;
    Ok(conf)
}

// ===================== Namespace Operations =====================

/// Open a network namespace file descriptor
fn open_netns(path: &str) -> Result<RawFd> {
    use std::os::unix::fs::OpenOptionsExt;
    use std::fs::OpenOptions;
    let file = OpenOptions::new()
        .read(true)
        .custom_flags(libc::O_CLOEXEC)
        .open(path)
        .context(format!("failed to open netns {}", path))?;
    Ok(std::os::unix::io::IntoRawFd::into_raw_fd(file))
}

/// Open current network namespace
fn open_current_netns() -> Result<RawFd> {
    open_netns("/proc/self/ns/net")
}

/// Set current thread's network namespace
fn set_netns(fd: RawFd) -> Result<()> {
    setns(unsafe { std::os::fd::BorrowedFd::borrow_raw(fd) }, CloneFlags::CLONE_NEWNET)
        .context("setns failed")?;
    Ok(())
}

// ===================== Netlink Operations via rtnetlink =====================

struct NetlinkOps {
    handle: Handle,
}

impl NetlinkOps {
    async fn new() -> Result<Self> {
        let (connection, handle, _) = new_connection()?;
        tokio::spawn(connection);
        Ok(NetlinkOps { handle })
    }
    
    /// Create a Linux bridge with given name and MTU
    async fn ensure_bridge(&self, name: &str, mtu: u32) -> Result<u32> {
        // Check if bridge already exists
        let mut links = self.handle.link().get().match_name(name.to_string()).execute();
        if let Ok(Some(link)) = links.try_next().await {
            return Ok(link.header.index);
        }
        
        // Create bridge
        self.handle.link()
            .add()
            .bridge(name.to_string())
            .execute()
            .await
            .context(format!("failed to create bridge {}", name))?;
        
        // Get its index
        let mut links = self.handle.link().get().match_name(name.to_string()).execute();
        let link = links.try_next().await?
            .ok_or_else(|| anyhow!("bridge {} not found after creation", name))?;
        let idx = link.header.index;
        
        // Set MTU
        self.handle.link().set(idx).mtu(mtu).execute().await?;
        
        // Bring up
        self.handle.link().set(idx).up().execute().await?;
        
        eprintln!("[CNI] Created bridge {} (idx={})", name, idx);
        Ok(idx)
    }
    
    /// Add IP address to an interface
    async fn add_addr(&self, ifindex: u32, addr: Ipv4Network) -> Result<()> {
        self.handle
            .address()
            .add(ifindex, addr.ip().into(), addr.prefix())
            .execute()
            .await
            .context(format!("failed to add addr {} to if {}", addr, ifindex))
    }
    
    /// Create veth pair, returning host-side ifindex
    /// peer_name is moved into netns_fd
    async fn create_veth(
        &self,
        host_name: &str,
        peer_name: &str,
        peer_netns_fd: RawFd,
        mtu: u32,
    ) -> Result<u32> {
        self.handle
            .link()
            .add()
            .veth(host_name.to_string(), peer_name.to_string())
            .execute()
            .await
            .context("failed to create veth pair")?;
        
        // Get host-side index
        let mut links = self.handle.link().get().match_name(host_name.to_string()).execute();
        let link = links.try_next().await?
            .ok_or_else(|| anyhow!("veth {} not found", host_name))?;
        let host_idx = link.header.index;
        
        // Get peer index (currently in our namespace, before moving)
        let mut peer_links = self.handle.link().get().match_name(peer_name.to_string()).execute();
        let peer_link = peer_links.try_next().await?
            .ok_or_else(|| anyhow!("veth peer {} not found", peer_name))?;
        let peer_idx = peer_link.header.index;
        
        // Move peer into target namespace
        self.handle
            .link()
            .set(peer_idx)
            .setns_by_fd(peer_netns_fd)
            .execute()
            .await
            .context("failed to move veth peer to container namespace")?;
        
        // Set MTU on host side
        self.handle.link().set(host_idx).mtu(mtu).execute().await?;
        
        // Bring up host side
        self.handle.link().set(host_idx).up().execute().await?;
        
        eprintln!("[CNI] Created veth pair {}<->{} (host_idx={})", host_name, peer_name, host_idx);
        Ok(host_idx)
    }
    
    /// Attach interface to bridge (set master)
    async fn set_master(&self, ifindex: u32, master_idx: u32) -> Result<()> {
        self.handle
            .link()
            .set(ifindex)
            .master(master_idx)
            .execute()
            .await
            .context("failed to set bridge master")
    }
    
    /// Add default IPv4 route
    async fn add_default_route(&self, ifindex: u32, gateway: Ipv4Addr) -> Result<()> {
        use netlink_packet_route::route::{RouteAttribute, RouteScope, RouteType};
        use std::net::IpAddr;
        
        self.handle
            .route()
            .add()
            .v4()
            .destination_prefix(Ipv4Addr::UNSPECIFIED, 0)
            .gateway(gateway)
            .output_interface(ifindex)
            .execute()
            .await
            .context(format!("failed to add default route via {}", gateway))
    }
    
    /// Get MAC address of interface
    async fn get_mac(&self, ifindex: u32) -> Result<String> {
        use netlink_packet_route::link::LinkAttribute;
        
        let mut links = self.handle.link().get().match_index(ifindex).execute();
        let link = links.try_next().await?
            .ok_or_else(|| anyhow!("interface {} not found", ifindex))?;
        
        for attr in &link.attributes {
            if let LinkAttribute::Address(addr) = attr {
                return Ok(addr.iter()
                    .map(|b| format!("{:02x}", b))
                    .collect::<Vec<_>>()
                    .join(":"));
            }
        }
        Ok("00:00:00:00:00:00".to_string())
    }
    
    /// Bring up interface by index
    async fn link_up(&self, ifindex: u32) -> Result<()> {
        self.handle.link().set(ifindex).up().execute().await
            .context(format!("failed to bring up interface {}", ifindex))
    }
    
    /// Get interface index by name
    async fn get_ifindex(&self, name: &str) -> Result<u32> {
        let mut links = self.handle.link().get().match_name(name.to_string()).execute();
        let link = links.try_next().await?
            .ok_or_else(|| anyhow!("interface {} not found", name))?;
        Ok(link.header.index)
    }
}

// ===================== IPAM =====================

struct IpamResult {
    address: Ipv4Network,
    gateway: Ipv4Addr,
}

/// Simple hardcoded IPAM for demonstration.
/// Real implementation: exec host-local IPAM plugin, parse its stdout.
fn call_ipam(_conf: &NetConf, _container_id: &str) -> Result<IpamResult> {
    // In production: exec /opt/cni/bin/host-local with proper env and stdin
    // Parse the JSON result from its stdout
    Ok(IpamResult {
        address: "10.244.0.5/24".parse().unwrap(),
        gateway: "10.244.0.1".parse().unwrap(),
    })
}

fn random_veth_name() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let ts = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .subsec_nanos();
    format!("veth{:08x}", ts)
}

// ===================== CMD_ADD =====================

async fn cmd_add(args: &CniArgs, conf: &NetConf) -> Result<CniResult> {
    let bridge_name = conf.bridge.as_deref().unwrap_or("cni0");
    let mtu = conf.mtu.unwrap_or(1500);
    let is_gateway = conf.is_gateway.unwrap_or(true);
    
    // 1. Call IPAM
    let ipam = call_ipam(conf, &args.container_id)?;
    eprintln!("[CNI ADD] IPAM assigned: {} gw={}", ipam.address, ipam.gateway);
    
    // 2. Open container namespace fd (before entering it)
    let netns_fd = open_netns(&args.netns)?;
    let orig_netns_fd = open_current_netns()?;
    
    // 3. Setup in HOST namespace
    let host_nl = NetlinkOps::new().await?;
    
    // Ensure bridge exists
    let bridge_idx = host_nl.ensure_bridge(bridge_name, mtu).await?;
    
    // Configure bridge gateway IP
    if is_gateway {
        let gw_net = Ipv4Network::new(ipam.gateway, ipam.address.prefix())
            .context("invalid gateway network")?;
        // Ignore error if already configured (EEXIST)
        let _ = host_nl.add_addr(bridge_idx, gw_net).await;
    }
    
    // Create veth pair (peer moves to container netns)
    let host_veth_name = random_veth_name();
    let host_veth_idx = host_nl.create_veth(
        &host_veth_name,
        &args.ifname,
        netns_fd,
        mtu,
    ).await?;
    
    // Attach host-side veth to bridge
    host_nl.set_master(host_veth_idx, bridge_idx).await?;
    
    let host_mac = host_nl.get_mac(host_veth_idx).await?;
    
    // 4. Enter container namespace and configure
    set_netns(netns_fd)?;
    
    // Create new connection in container namespace
    let container_nl = NetlinkOps::new().await?;
    
    // Bring up lo
    if let Ok(lo_idx) = container_nl.get_ifindex("lo").await {
        let _ = container_nl.link_up(lo_idx).await;
    }
    
    // Get container-side veth index and configure
    let container_if_idx = container_nl.get_ifindex(&args.ifname).await
        .context(format!("interface {} not found in container ns", args.ifname))?;
    
    // Bring up container interface
    container_nl.link_up(container_if_idx).await?;
    
    // Add IP address
    container_nl.add_addr(container_if_idx, ipam.address).await?;
    
    // Add default route
    container_nl.add_default_route(container_if_idx, ipam.gateway).await?;
    
    let container_mac = container_nl.get_mac(container_if_idx).await?;
    
    // 5. Return to host namespace
    set_netns(orig_netns_fd)?;
    
    unsafe {
        libc::close(netns_fd);
        libc::close(orig_netns_fd);
    }
    
    eprintln!("[CNI ADD] Success: pod IP={}, veth={}", ipam.address, host_veth_name);
    
    // 6. Build CNI result
    Ok(CniResult {
        cni_version: conf.cni_version.clone(),
        interfaces: vec![
            CniInterface {
                name: args.ifname.clone(),
                mac: container_mac,
                sandbox: args.netns.clone(),
            },
            CniInterface {
                name: host_veth_name,
                mac: host_mac,
                sandbox: String::new(),
            },
        ],
        ips: vec![
            CniIP {
                address: ipam.address.to_string(),
                gateway: ipam.gateway.to_string(),
                interface: 0,  // index into interfaces array (container side)
            },
        ],
        routes: vec![
            CniRoute { dst: "0.0.0.0/0".to_string(), gw: Some(ipam.gateway.to_string()) },
        ],
        dns: CniDNS::default(),
    })
}

// ===================== CMD_DEL =====================

async fn cmd_del(args: &CniArgs, conf: &NetConf) -> Result<()> {
    // DEL must be idempotent: succeed even if already cleaned up
    
    // 1. Release IPAM (delete allocation file for container_id)
    // In production: exec host-local IPAM plugin with CNI_COMMAND=DEL
    eprintln!("[CNI DEL] Releasing IPAM for {}", args.container_id);
    
    if args.netns.is_empty() {
        eprintln!("[CNI DEL] No netns provided, skipping interface cleanup");
        return Ok(());
    }
    
    // 2. Enter container namespace and remove interface
    let netns_fd = match open_netns(&args.netns) {
        Ok(fd) => fd,
        Err(e) => {
            eprintln!("[CNI DEL] Netns {} not found (already gone?): {}", args.netns, e);
            return Ok(());  // idempotent: netns already gone = success
        }
    };
    let orig_netns_fd = open_current_netns()?;
    
    set_netns(netns_fd)?;
    let container_nl = NetlinkOps::new().await?;
    
    // Delete container interface (this also deletes the veth pair)
    if let Ok(idx) = container_nl.get_ifindex(&args.ifname).await {
        container_nl.handle.link().del(idx).execute().await
            .unwrap_or_else(|e| eprintln!("[CNI DEL] Warning deleting {}: {}", args.ifname, e));
    }
    
    set_netns(orig_netns_fd)?;
    
    unsafe {
        libc::close(netns_fd);
        libc::close(orig_netns_fd);
    }
    
    eprintln!("[CNI DEL] Done");
    Ok(())
}

// ===================== CMD_CHECK =====================

async fn cmd_check(args: &CniArgs, conf: &NetConf) -> Result<()> {
    if args.netns.is_empty() {
        return Err(anyhow!("CNI_NETNS required for CHECK"));
    }
    
    let netns_fd = open_netns(&args.netns)?;
    let orig_netns_fd = open_current_netns()?;
    
    set_netns(netns_fd)?;
    let container_nl = NetlinkOps::new().await?;
    
    // Verify interface exists
    container_nl.get_ifindex(&args.ifname).await
        .context(format!("interface {} missing in container netns", args.ifname))?;
    
    // TODO: verify IP addresses, routes match expected from config/prevResult
    
    set_netns(orig_netns_fd)?;
    
    unsafe {
        libc::close(netns_fd);
        libc::close(orig_netns_fd);
    }
    
    Ok(())
}

// ===================== Main Entry Point =====================

fn output_error(code: u32, msg: &str, details: &str) {
    let err = CniError::new(code, msg, details);
    println!("{}", serde_json::to_string(&err).unwrap());
    std::process::exit(1);
}

fn main() {
    // CNI plugins MUST NOT print anything to stdout except the result JSON
    // (or error JSON). Use stderr for logging.
    
    let rt = Runtime::new().expect("tokio runtime");
    
    let args = match parse_cni_args() {
        Ok(a) => a,
        Err(e) => {
            output_error(4, "invalid environment variables", &e.to_string());
            return;
        }
    };
    
    match args.command.as_str() {
        "VERSION" => {
            let version = serde_json::json!({
                "cniVersion": "1.0.0",
                "supportedVersions": ["0.3.0", "0.3.1", "0.4.0", "1.0.0"]
            });
            println!("{}", version);
        }
        
        "ADD" => {
            let conf = match read_config() {
                Ok(c) => c,
                Err(e) => {
                    output_error(6, "failed to parse config", &e.to_string());
                    return;
                }
            };
            match rt.block_on(cmd_add(&args, &conf)) {
                Ok(result) => {
                    println!("{}", serde_json::to_string(&result).unwrap());
                }
                Err(e) => {
                    output_error(11, "ADD failed", &e.to_string());
                }
            }
        }
        
        "DEL" => {
            let conf = match read_config() {
                Ok(c) => c,
                Err(e) => {
                    output_error(6, "failed to parse config", &e.to_string());
                    return;
                }
            };
            match rt.block_on(cmd_del(&args, &conf)) {
                Ok(()) => {
                    // DEL returns empty result on success
                    println!("{{}}");
                }
                Err(e) => {
                    output_error(11, "DEL failed", &e.to_string());
                }
            }
        }
        
        "CHECK" => {
            let conf = match read_config() {
                Ok(c) => c,
                Err(e) => {
                    output_error(6, "failed to parse config", &e.to_string());
                    return;
                }
            };
            match rt.block_on(cmd_check(&args, &conf)) {
                Ok(()) => {
                    println!("{{}}");
                }
                Err(e) => {
                    output_error(11, "CHECK failed", &e.to_string());
                }
            }
        }
        
        cmd => {
            output_error(4, "unknown command", &format!("unknown CNI command: {}", cmd));
        }
    }
}
```

---

## 25. eBPF CNI Data Plane in C: XDP + TC Program

This is a complete, real eBPF program for a CNI data plane — XDP for ingress filtering and TC
for pod policy enforcement. This is the style that Cilium uses internally.

```c
/* cni_dataplane.bpf.c
 * Complete BPF program for a CNI data plane:
 * - XDP: fast ingress filtering and VXLAN decap
 * - TC:  per-pod policy enforcement and routing
 *
 * Compile:
 *   clang -O2 -g -target bpf -D__TARGET_ARCH_x86 \
 *         -I/usr/include/bpf \
 *         -c cni_dataplane.bpf.c -o cni_dataplane.bpf.o
 *
 * Load:
 *   ip link set dev eth0 xdp obj cni_dataplane.bpf.o sec xdp_ingress
 *   tc qdisc add dev lxcXXX clsact
 *   tc filter add dev lxcXXX ingress bpf direct-action obj cni_dataplane.bpf.o sec tc_from_pod
 *   tc filter add dev lxcXXX egress  bpf direct-action obj cni_dataplane.bpf.o sec tc_to_pod
 */

#include "vmlinux.h"           /* auto-generated from BTF */
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_core_read.h>

/* Protocol constants (from linux/if_ether.h, linux/ip.h etc.) */
#define ETH_P_IP    0x0800
#define ETH_P_IPV6  0x86DD
#define ETH_P_ARP   0x0806
#define IPPROTO_TCP 6
#define IPPROTO_UDP 17
#define IPPROTO_ICMP 1
#define VXLAN_PORT  4789

/* tc return codes */
#define TC_ACT_OK       0
#define TC_ACT_SHOT     2
#define TC_ACT_REDIRECT 7

/* ==================== BPF Maps ==================== */

/* Endpoint table: pod IP -> interface index for direct redirect */
struct endpoint_entry {
    __u32 ifindex;          /* lxcXXX interface index (host-side veth) */
    __u32 lxc_id;           /* endpoint/container ID */
    __u8  mac[6];           /* pod MAC address */
    __u8  pad[2];
    __u32 identity;         /* security identity */
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __be32);                  /* IPv4 address */
    __type(value, struct endpoint_entry);
    __uint(max_entries, 65536);
    __uint(pinning, LIBBPF_PIN_BY_NAME);  /* pin to /sys/fs/bpf/tc/globals/ */
} cni_endpoints SEC(".maps");

/* Policy map: (src_identity, dst_identity, port, proto) -> verdict */
struct policy_key {
    __u32 src_identity;
    __u32 dst_identity;
    __u16 dst_port;
    __u8  protocol;
    __u8  pad;
};
struct policy_value {
    __u8  allow;  /* 1=allow, 0=deny */
    __u8  pad[3];
};
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct policy_key);
    __type(value, struct policy_value);
    __uint(max_entries, 65536);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} cni_policy SEC(".maps");

/* Per-CPU counters */
struct cni_stats {
    __u64 rx_pkts;
    __u64 rx_bytes;
    __u64 tx_pkts;
    __u64 tx_bytes;
    __u64 policy_drops;
    __u64 redirects;
};
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, __u32);
    __type(value, struct cni_stats);
    __uint(max_entries, 1);
} cni_stats_map SEC(".maps");

/* Connection tracking (simplified) */
struct ct_key {
    __be32 src_ip;
    __be32 dst_ip;
    __be16 src_port;
    __be16 dst_port;
    __u8   protocol;
    __u8   pad[3];
};
struct ct_value {
    __u64  last_seen_ns;
    __u32  rx_packets;
    __u32  tx_packets;
    __u8   state;   /* 0=new, 1=established, 2=closing */
    __u8   pad[3];
};
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key, struct ct_key);
    __type(value, struct ct_value);
    __uint(max_entries, 524288);  /* 512K connections */
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} cni_ct4 SEC(".maps");

/* Notification ring buffer (for Hubble-like observability) */
struct flow_event {
    __u64  timestamp_ns;
    __be32 src_ip;
    __be32 dst_ip;
    __be16 src_port;
    __be16 dst_port;
    __u8   protocol;
    __u8   verdict;   /* 0=pass, 1=drop, 2=redirect */
    __u8   drop_reason;
    __u8   pad;
    __u32  src_identity;
    __u32  dst_identity;
};
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 22);  /* 4MB ring buffer */
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} cni_events SEC(".maps");

/* ==================== Helpers ==================== */

static __always_inline struct cni_stats *get_stats(void) {
    __u32 key = 0;
    return bpf_map_lookup_elem(&cni_stats_map, &key);
}

static __always_inline void emit_flow_event(
    __be32 src_ip, __be32 dst_ip,
    __be16 src_port, __be16 dst_port,
    __u8 proto, __u8 verdict, __u8 drop_reason,
    __u32 src_id, __u32 dst_id)
{
    struct flow_event *ev = bpf_ringbuf_reserve(&cni_events, sizeof(*ev), 0);
    if (!ev) return;  /* ring full, drop telemetry (not packet) */
    
    ev->timestamp_ns  = bpf_ktime_get_ns();
    ev->src_ip        = src_ip;
    ev->dst_ip        = dst_ip;
    ev->src_port      = src_port;
    ev->dst_port      = dst_port;
    ev->protocol      = proto;
    ev->verdict       = verdict;
    ev->drop_reason   = drop_reason;
    ev->src_identity  = src_id;
    ev->dst_identity  = dst_id;
    
    bpf_ringbuf_submit(ev, 0);
}

/* Parse L3/L4 headers from skb */
static __always_inline int parse_headers(
    struct __sk_buff *skb,
    __be32 *src_ip, __be32 *dst_ip,
    __be16 *src_port, __be16 *dst_port,
    __u8 *proto)
{
    void *data     = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;
    
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) return -1;
    if (eth->h_proto != bpf_htons(ETH_P_IP)) return -1;
    
    struct iphdr *iph = (void *)(eth + 1);
    if ((void *)(iph + 1) > data_end) return -1;
    
    *src_ip = iph->saddr;
    *dst_ip = iph->daddr;
    *proto  = iph->protocol;
    *src_port = 0;
    *dst_port = 0;
    
    __u32 ip_hdr_len = iph->ihl * 4;
    void *l4 = (void *)iph + ip_hdr_len;
    
    if (iph->protocol == IPPROTO_TCP) {
        struct tcphdr *tcp = l4;
        if ((void *)(tcp + 1) > data_end) return -1;
        *src_port = tcp->source;
        *dst_port = tcp->dest;
    } else if (iph->protocol == IPPROTO_UDP) {
        struct udphdr *udp = l4;
        if ((void *)(udp + 1) > data_end) return -1;
        *src_port = udp->source;
        *dst_port = udp->dest;
    }
    
    return 0;
}

/* Connection tracking lookup/update */
static __always_inline int ct_lookup_or_create(
    __be32 src_ip, __be32 dst_ip,
    __be16 src_port, __be16 dst_port, __u8 proto)
{
    struct ct_key key = {
        .src_ip   = src_ip, .dst_ip   = dst_ip,
        .src_port = src_port, .dst_port = dst_port,
        .protocol = proto,
    };
    
    struct ct_value *ctv = bpf_map_lookup_elem(&cni_ct4, &key);
    if (ctv) {
        /* Existing connection: update */
        ctv->last_seen_ns = bpf_ktime_get_ns();
        ctv->rx_packets++;
        return ctv->state;  /* 0=new, 1=established */
    }
    
    /* New connection: create entry */
    struct ct_value new_ctv = {
        .last_seen_ns = bpf_ktime_get_ns(),
        .rx_packets   = 1,
        .state        = 1,  /* established (simplified: no SYN tracking) */
    };
    bpf_map_update_elem(&cni_ct4, &key, &new_ctv, BPF_ANY);
    return 0;
}

/* ==================== XDP Program (ingress, physical NIC) ==================== */

SEC("xdp_ingress")
int xdp_cni_ingress(struct xdp_md *ctx) {
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;
    
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) return XDP_DROP;
    
    /* Pass ARP to kernel (needed for ARP resolution) */
    if (eth->h_proto == bpf_htons(ETH_P_ARP)) return XDP_PASS;
    
    /* Only handle IPv4 in XDP for now */
    if (eth->h_proto != bpf_htons(ETH_P_IP)) return XDP_PASS;
    
    struct iphdr *iph = (void *)(eth + 1);
    if ((void *)(iph + 1) > data_end) return XDP_DROP;
    
    /* VXLAN decapsulation: if UDP dst=4789, strip outer headers */
    if (iph->protocol == IPPROTO_UDP) {
        __u32 ip_hdr_len = iph->ihl * 4;
        struct udphdr *udp = (void *)iph + ip_hdr_len;
        if ((void *)(udp + 1) > data_end) return XDP_PASS;
        
        if (udp->dest == bpf_htons(VXLAN_PORT)) {
            /* VXLAN: strip outer ETH+IP+UDP+VXLAN header */
            __u32 strip_len = sizeof(*eth) + ip_hdr_len + sizeof(*udp) + 8 /* vxlan hdr */;
            if (bpf_xdp_adjust_head(ctx, strip_len) != 0) return XDP_PASS;
            /* Now packet starts with inner ETH frame */
            /* Fall through to XDP_PASS (let kernel handle inner packet) */
            return XDP_PASS;
        }
    }
    
    /* Fast-path redirect for known pod destinations */
    __be32 dst_ip = iph->daddr;
    struct endpoint_entry *ep = bpf_map_lookup_elem(&cni_endpoints, &dst_ip);
    if (ep) {
        /* Packet destined for a local pod: redirect to its veth */
        /* This bypasses routing table lookup */
        return bpf_redirect(ep->ifindex, 0);
    }
    
    return XDP_PASS;
}

/* ==================== TC Program: from pod (egress from pod's perspective) ==================== */

SEC("tc_from_pod")
int tc_egress_from_pod(struct __sk_buff *skb) {
    struct cni_stats *stats = get_stats();
    if (stats) {
        stats->tx_pkts++;
        stats->tx_bytes += skb->len;
    }
    
    __be32 src_ip, dst_ip;
    __be16 src_port, dst_port;
    __u8 proto;
    
    if (parse_headers(skb, &src_ip, &dst_ip, &src_port, &dst_port, &proto) < 0)
        return TC_ACT_OK;  /* pass non-IPv4 traffic */
    
    /* Look up destination pod/identity */
    struct endpoint_entry *dst_ep = bpf_map_lookup_elem(&cni_endpoints, &dst_ip);
    __u32 dst_identity = dst_ep ? dst_ep->identity : 0;
    
    /* Look up source pod identity (injected by agent via skb->mark or dedicated map) */
    /* Here simplified: src_identity = skb->mark (set by agent when attaching program) */
    __u32 src_identity = skb->mark;
    
    /* Policy enforcement: check if this src_identity can reach dst_identity:dst_port */
    struct policy_key pkey = {
        .src_identity = src_identity,
        .dst_identity = dst_identity,
        .dst_port     = dst_port,
        .protocol     = proto,
    };
    struct policy_value *pval = bpf_map_lookup_elem(&cni_policy, &pkey);
    
    if (!pval || !pval->allow) {
        /* Default deny if no policy matches */
        if (stats) stats->policy_drops++;
        emit_flow_event(src_ip, dst_ip, src_port, dst_port, proto,
                        1, 0, src_identity, dst_identity);  /* verdict=1 (drop) */
        return TC_ACT_SHOT;
    }
    
    /* Update connection tracking */
    ct_lookup_or_create(src_ip, dst_ip, src_port, dst_port, proto);
    
    /* If destination is a local pod: redirect directly to its interface */
    if (dst_ep) {
        if (stats) stats->redirects++;
        emit_flow_event(src_ip, dst_ip, src_port, dst_port, proto,
                        2, 0, src_identity, dst_identity);  /* verdict=2 (redirect) */
        return bpf_redirect(dst_ep->ifindex, BPF_F_INGRESS);
    }
    
    emit_flow_event(src_ip, dst_ip, src_port, dst_port, proto,
                    0, 0, src_identity, dst_identity);  /* verdict=0 (pass) */
    return TC_ACT_OK;
}

/* ==================== TC Program: to pod (ingress to pod's perspective) ==================== */

SEC("tc_to_pod")
int tc_ingress_to_pod(struct __sk_buff *skb) {
    struct cni_stats *stats = get_stats();
    if (stats) {
        stats->rx_pkts++;
        stats->rx_bytes += skb->len;
    }
    
    __be32 src_ip, dst_ip;
    __be16 src_port, dst_port;
    __u8 proto;
    
    if (parse_headers(skb, &src_ip, &dst_ip, &src_port, &dst_port, &proto) < 0)
        return TC_ACT_OK;
    
    /* Check connection tracking: if established, skip full policy check */
    /* This is the "fast path" for reply packets */
    struct ct_key reply_key = {
        .src_ip   = dst_ip,   /* reversed */
        .dst_ip   = src_ip,
        .src_port = dst_port,
        .dst_port = src_port,
        .protocol = proto,
    };
    struct ct_value *ctv = bpf_map_lookup_elem(&cni_ct4, &reply_key);
    if (ctv && ctv->state == 1) {
        /* Reply to established connection: pass without policy check */
        ctv->last_seen_ns = bpf_ktime_get_ns();
        ctv->tx_packets++;
        return TC_ACT_OK;
    }
    
    /* New ingress connection: check ingress policy */
    /* (src_identity must be looked up from cilium_ipcache) */
    struct endpoint_entry *src_ep = bpf_map_lookup_elem(&cni_endpoints, &src_ip);
    __u32 src_identity = src_ep ? src_ep->identity : 0;
    __u32 dst_identity = skb->mark;  /* set when attaching tc filter to this endpoint */
    
    struct policy_key pkey = {
        .src_identity = src_identity,
        .dst_identity = dst_identity,
        .dst_port     = dst_port,
        .protocol     = proto,
    };
    struct policy_value *pval = bpf_map_lookup_elem(&cni_policy, &pkey);
    
    if (!pval || !pval->allow) {
        if (stats) stats->policy_drops++;
        emit_flow_event(src_ip, dst_ip, src_port, dst_port, proto,
                        1, 0, src_identity, dst_identity);
        return TC_ACT_SHOT;
    }
    
    ct_lookup_or_create(dst_ip, src_ip, dst_port, src_port, proto);  /* store forward direction */
    
    return TC_ACT_OK;
}

char __license[] SEC("license") = "GPL";
```

---

## 26. CNI Performance: Benchmarks, Bottlenecks, Tuning

### 26.1 Latency Benchmarks (Representative, 2024 Hardware)

```
Test setup: bare-metal, kernel 6.x, 10GbE NIC, same node
Measurement: TCP RR latency (request-response, 1-byte messages)

+--------------------------+------------------+------------------+
| CNI Mode                 | Avg Latency (µs) | P99 Latency (µs) |
+--------------------------+------------------+------------------+
| Host <-> Host (baseline) | 6                | 8                |
| Cilium BPF redirect      | 17               | 22               |
| Calico eBPF              | 19               | 25               |
| Flannel VXLAN (kernel)   | 28               | 35               |
| Calico iptables          | 22               | 30               |
| OVN-Kubernetes           | 32               | 42               |
| Flannel host-gw (no ovrly)| 18              | 23               |
| SR-IOV (macvtap)         | 8                | 10               |
| DPDK vhost-user          | 2                | 3                |
+--------------------------+------------------+------------------+

Cross-node latency (add ~0.1-0.5ms physical network baseline):
  Direct routing (Calico BGP): +0.1ms
  VXLAN overlay:               +0.2ms (VXLAN encap/decap overhead)
  WireGuard encrypted:         +0.3ms (crypto overhead)
  IPsec:                       +0.25ms
```

### 26.2 Throughput Benchmarks

```
Test setup: iperf3 TCP, single stream, 64KB window, same node

+--------------------------+------------+------------+
| CNI Mode                 | Throughput | CPU Usage  |
+--------------------------+------------+------------+
| Host baseline            | 40 Gbps    | 25%        |
| Cilium BPF redirect      | 38 Gbps    | 28%        |
| Calico eBPF              | 36 Gbps    | 30%        |
| Flannel VXLAN            | 18 Gbps    | 65%        |  <- GSO helps but still high
| Calico iptables          | 32 Gbps    | 35%        |
| SR-IOV                   | 40 Gbps    | 10%        |
+--------------------------+------------+------------+

Key observation:
  VXLAN halves throughput because:
  1. CPU must encap/decap VXLAN headers
  2. GRO/GSO don't fully coalesce VXLAN (must understand inner packets)
  3. MTU reduction increases packet count for same data

Cilium vs Flannel:
  Cilium BPF redirect avoids routing table, bridge, conntrack
  Result: ~2x better throughput for pod-to-pod same node
```

### 26.3 Rule Scalability

```
kube-proxy iptables DNAT rule count vs latency:
  100 services   -> 2,000 iptables rules   -> 0.5µs overhead
  1,000 services -> 20,000 iptables rules  -> 2µs overhead
  10,000 services-> 200,000 iptables rules -> 20µs overhead (!)
  
  Each iptables rule: O(n) traversal
  iptables is O(n) per packet = death by 10k services

kube-proxy IPVS:
  10,000 services -> O(1) hash lookup -> <1µs
  No performance degradation with scale

Cilium eBPF:
  10,000 services -> O(1) BPF map lookup -> <1µs
  Plus: no conntrack (own CT) = additional performance win
  Plus: direct redirect (no routing table) = even faster

Calico network policy rules:
  iptables mode: O(n) policies -> degrades with many policies
  ipset mode:    O(1) per IP match (ipset hash lookup)
  eBPF mode:     O(1) identity map lookup (no ipset at all)
```

### 26.4 Key Tuning Parameters

```
/proc/sys/net/ tuning for high-performance CNI:

# Increase socket buffer sizes (more in-flight data):
sysctl -w net.core.rmem_max=134217728
sysctl -w net.core.wmem_max=134217728
sysctl -w net.ipv4.tcp_rmem="4096 87380 67108864"
sysctl -w net.ipv4.tcp_wmem="4096 65536 67108864"

# TCP backlog (more connections in accept queue):
sysctl -w net.ipv4.tcp_max_syn_backlog=8192
sysctl -w net.core.somaxconn=65535

# Netdev backlog (more packets before DROP):
sysctl -w net.core.netdev_max_backlog=250000
sysctl -w net.core.netdev_budget=600  # NAPI poll budget

# VXLAN/overlay optimizations:
sysctl -w net.ipv4.udp_rmem_min=16384
sysctl -w net.ipv4.udp_wmem_min=16384

# IP forwarding (required for pod routing):
sysctl -w net.ipv4.ip_forward=1
sysctl -w net.ipv6.conf.all.forwarding=1

# RP filter (reverse path: relaxed for asymmetric routing):
sysctl -w net.ipv4.conf.all.rp_filter=0
sysctl -w net.ipv4.conf.default.rp_filter=0
# (rp_filter=1 drops packets where return path != ingress interface)
# This breaks Flannel/Calico overlay where routing is asymmetric

# ARP tuning (for large cluster bridge mode):
sysctl -w net.ipv4.neigh.default.gc_thresh1=4096
sysctl -w net.ipv4.neigh.default.gc_thresh2=8192
sysctl -w net.ipv4.neigh.default.gc_thresh3=16384
# Without this: ARP table fills up, gc_thresh1=128 is the default

# Bridge netfilter (required for kube-proxy iptables to see bridge traffic):
sysctl -w net.bridge.bridge-nf-call-iptables=1
sysctl -w net.bridge.bridge-nf-call-ip6tables=1
# Load: modprobe br_netfilter

# Disable IPv6 if not used (saves overhead):
sysctl -w net.ipv6.conf.all.disable_ipv6=1

# TCP keepalive (detect dead connections faster):
sysctl -w net.ipv4.tcp_keepalive_time=300
sysctl -w net.ipv4.tcp_keepalive_probes=5
sysctl -w net.ipv4.tcp_keepalive_intvl=30

NIC tuning (ethtool):
# Increase RX/TX ring buffers:
ethtool -G eth0 rx 4096 tx 4096

# Interrupt coalescing (adaptive, or explicit):
ethtool -C eth0 adaptive-rx on adaptive-tx on

# RSS queues (match CPU count for NUMA):
ethtool -L eth0 combined 8

# GRO/GSO/TSO (should be on by default):
ethtool -K eth0 gro on gso on tso on

# For VXLAN: check hw offload support:
ethtool -k eth0 | grep -E "tx-udp_tnl|rx-udp_tnl|tx-vxlan|rx-vxlan"
```

### 26.5 Profiling CNI Data Plane Bottlenecks

```
Tools for finding CNI performance issues:

1. perf (CPU profiling):
   perf record -g -a sleep 30
   perf report
   # Look for: ip_rcv, nf_hook_slow, __br_forward, nf_nat_packet
   # These are the hot paths in iptables-based CNI

2. BCC tools (eBPF-based, zero overhead profiling):
   tcptop      <- TCP throughput by connection
   tcpretrans  <- TCP retransmissions (MTU/PMTU issues show here)
   netlatency  <- per-packet latency histogram
   runqlat     <- scheduler latency (not CNI, but affects pod performance)
   
   Example: trace conntrack lookups:
   trace 'nf_conntrack_find_get' -tKU

3. eBPF trace for CNI operations:
   bpftrace -e '
   kprobe:veth_xmit {
       @bytes = hist(args->skb->len);
   }'
   # Shows packet size distribution through veth

4. XDP trace (CNI drop monitoring):
   bpftrace -e '
   tracepoint:xdp:xdp_exception {
       printf("XDP exception: ifindex=%d action=%d prog=%d\n",
              args->ifindex, args->act, args->prog_id);
   }'

5. iptables counters:
   watch -n1 "iptables -t nat -L -n -v | grep -v '0     0'"
   # Shows active NAT rules being hit
   # Unexpected high counts on conntrack rules = table exhaustion approaching

6. Bridge forwarding stats:
   bridge fdb show                     # MAC table
   bridge link show                    # bridge port states
   ip -s link show cni0                # packet counters per interface
   cat /proc/net/dev                   # bytes/packets/errors/drops per iface
```
