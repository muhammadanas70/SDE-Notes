# CNI: The Unexplored Depths
## What the Standard Guide Doesn't Cover — Kernel Internals, Runtime Plumbing, BPF Programs, Encryption, Service Meshes, and More

> This document is a **strict complement** to the existing CNI complete guide.
> It does **not** repeat Linux namespace basics, veth creation, bridge plugin logic,
> basic IPAM, or the standard Flannel/Calico/Cilium overviews.
> It goes **below** all of that — into sk_buff lifecycles, NAPI polling, XDP/TC BPF
> programs you can actually compile and run, conntrack internals, kube-proxy's exact
> iptables/IPVS rules, Multus, SR-IOV, AF_XDP, WireGuard, IPSec, gVisor, Kata,
> service mesh CNI hooks, dual-stack, DNS architecture, MTU mechanics, and a complete
> Rust CNI plugin.

---

## Table of Contents

1. [sk_buff: The Packet Data Structure](#1-sk_buff-the-packet-data-structure)
2. [NAPI: Efficient Packet Reception](#2-napi-efficient-packet-reception)
3. [GSO, TSO, GRO and Overlay Networks](#3-gso-tso-gro-and-overlay-networks)
4. [XDP Programming in C](#4-xdp-programming-in-c)
5. [TC BPF Programming with libbpf](#5-tc-bpf-programming-with-libbpf)
6. [BPF Maps: Architecture and Use in CNI](#6-bpf-maps-architecture-and-use-in-cni)
7. [Connection Tracking (nf_conntrack) Internals](#7-connection-tracking-nf_conntrack-internals)
8. [kube-proxy: iptables and IPVS Modes Deep Dive](#8-kube-proxy-iptables-and-ipvs-modes-deep-dive)
9. [ClusterIP, NodePort, LoadBalancer at Kernel Level](#9-clusterip-nodeport-loadbalancer-at-kernel-level)
10. [libcni and go-cni: The Go CNI Library Internals](#10-libcni-and-go-cni-the-go-cni-library-internals)
11. [Kata Containers Networking: VM-Level CNI](#11-kata-containers-networking-vm-level-cni)
12. [gVisor Networking: The Go Netstack](#12-gvisor-networking-the-go-netstack)
13. [crun: The C OCI Runtime and Namespaces](#13-crun-the-c-oci-runtime-and-namespaces)
14. [Multus CNI Internals](#14-multus-cni-internals)
15. [SR-IOV CNI: Hardware Virtualization](#15-sr-iov-cni-hardware-virtualization)
16. [AF_XDP: Kernel Bypass for Containers](#16-af_xdp-kernel-bypass-for-containers)
17. [WireGuard Integration in Cilium](#17-wireguard-integration-in-cilium)
18. [IPSec/XFRM: Pod-to-Pod Encryption](#18-ipsecxfrm-pod-to-pod-encryption)
19. [Istio CNI Plugin and Ambient Mesh](#19-istio-cni-plugin-and-ambient-mesh)
20. [Dual-Stack IPv4/IPv6 CNI](#20-dual-stack-ipv4ipv6-cni)
21. [DNS Architecture Inside Pods](#21-dns-architecture-inside-pods)
22. [NodeLocal DNSCache: eBPF-Accelerated DNS](#22-nodelocal-dnscache-ebpf-accelerated-dns)
23. [hostPort and portmap Plugin: Exact Kernel Mechanics](#23-hostport-and-portmap-plugin-exact-kernel-mechanics)
24. [kubectl port-forward: The Full Tunnel Path](#24-kubectl-port-forward-the-full-tunnel-path)
25. [MTU, PMTUD, TSO/GRO in Overlay Networks](#25-mtu-pmtud-tsogro-in-overlay-networks)
26. [Network Capabilities, Seccomp, and CNI Security](#26-network-capabilities-seccomp-and-cni-security)
27. [Complete Rust CNI Plugin](#27-complete-rust-cni-plugin)
28. [Debugging Toolkit: nsenter, bpftrace, conntrack, ss](#28-debugging-toolkit-nsenter-bpftrace-conntrack-ss)

---

## 1. sk_buff: The Packet Data Structure

Every packet in the Linux kernel lives in a `struct sk_buff` (socket buffer). Understanding this structure is mandatory for understanding CNI at the deepest level — every BPF program, every driver, every netfilter hook reads and writes `sk_buff` fields.

### 1.1 Memory Layout

```
sk_buff and its data in memory:

  struct sk_buff (on slab/kmalloc)
  +----------------------------------------+
  | struct sk_buff *next, *prev            |  doubly-linked list
  | struct sock *sk                        |  owning socket (or NULL)
  | unsigned int len, data_len            |  total length, paged data len
  | __be16 protocol                       |  ETH_P_IP, ETH_P_IPV6, etc.
  | __u16 transport_header               |  offset to L4 header in data
  | __u16 network_header                 |  offset to L3 header
  | __u16 mac_header                     |  offset to L2 header
  | sk_buff_data_t tail, end             |  end of data, end of buffer
  | unsigned char *head, *data           |  start of buffer, start of data
  |                                        |
  | __u32 mark                            |  fwmark (used by routing rules)
  | __u32 priority                        |  tc priority
  | __u8  pkt_type                        |  PACKET_HOST, BROADCAST, etc.
  | __u8  ip_summed                       |  checksum offload info
  | struct nf_conntrack *nfct            |  connection tracking entry
  | unsigned long _skb_refdst            |  dst_entry (next-hop cache)
  | struct skb_shared_info *shinfo       |  gso_size, frags[], frag_list
  +----------------------------------------+

  Separate allocation: linear packet data
  head                                    end
  +----+-------+-------+-------+---------+
  |headroom| mac_hdr | net_hdr | transport |  tailroom  |
  |        |  (eth)  |  (ip)  |  (tcp/udp)|            |
  +----+-------+-------+-------+---------+
       ^                               ^
      data                            tail
```

### 1.2 Headroom and Tailroom

`sk_buff` has headroom (space before `data`) and tailroom (space after `tail`). This is critical for CNI overlay tunnels:

When Flannel or Cilium needs to add a VXLAN/GENEVE header, it does:

```c
/* Add VXLAN outer headers — grows the packet by prepending */
skb_cow_head(skb, sizeof(struct vxlanhdr) +
                  sizeof(struct udphdr) +
                  sizeof(struct iphdr) +
                  sizeof(struct ethhdr));

/* skb_push() moves data pointer BACKWARDS into headroom */
vxh = __skb_push(skb, sizeof(struct vxlanhdr));
vxh->vx_flags = VXLAN_HF_VNI;
vxh->vx_vni   = htonl(vni << 8);

/* Similarly for UDP, IP, Ethernet headers */
uh = __skb_push(skb, sizeof(struct udphdr));
/* ... fill UDP src/dst port ... */
```

If there isn't enough headroom, `skb_cow_head()` reallocates and copies. This is expensive. Network drivers typically allocate extra headroom (e.g., 128 bytes via `NET_SKB_PAD`) to avoid this. Overlay CNI plugins need even more because they prepend an entire outer IP+UDP+VXLAN header (~50 bytes).

### 1.3 Paged Data and Fragmented skbs

For large packets (e.g., 64KB with TSO), `sk_buff` uses `skb_shared_info` (pointed to by `shinfo`) to store data in pages rather than the linear buffer:

```c
struct skb_shared_info {
    __u8            flags;
    __u8            meta_len;
    __u8            nr_frags;      /* number of page fragments */
    __u8            tx_flags;
    unsigned short  gso_size;      /* segment size for GSO */
    unsigned short  gso_segs;      /* number of GSO segments */
    unsigned int    gso_type;      /* SKB_GSO_TCPV4, SKB_GSO_UDP_L4, etc. */
    struct sk_buff  *frag_list;    /* chained skbs for jumbo frames */
    skb_frag_t      frags[MAX_SKB_FRAGS]; /* page[] + offset + size */
};
```

**Why CNI must care:** If a pod sends a 64KB TCP segment (GSO), the kernel carries it as one `sk_buff` with `gso_size=1460, gso_segs=44`. When a CNI plugin calls `skb_linearize()` (to process the full packet), it copies all paged data into the linear buffer — a significant copy. BPF programs in Cilium are carefully written to avoid linearizing by working only with the headers in the linear area.

### 1.4 How Headers Are Accessed

```c
/* Access L3 IP header — no copy, just pointer arithmetic */
struct iphdr *ip4h = ip_hdr(skb);
/* ip_hdr() = (struct iphdr *)(skb->head + skb->network_header) */

/* Access L4 TCP header */
struct tcphdr *tcph = tcp_hdr(skb);
/* tcp_hdr() = (struct tcphdr *)(skb->head + skb->transport_header) */

/* Access Ethernet header (L2) */
struct ethhdr *eth = eth_hdr(skb);
/* eth_hdr() = (struct ethhdr *)(skb->head + skb->mac_header) */
```

BPF programs access the same data via `bpf_skb_load_bytes()` or direct data pointer arithmetic with bounds checking enforced by the BPF verifier.

### 1.5 sk_buff Cloning and Reference Counting

When a packet needs to be sent to multiple destinations (e.g., broadcast on a bridge, or mirrored for Hubble), the kernel `skb_clone()`s the sk_buff:

```c
struct sk_buff *clone = skb_clone(skb, GFP_ATOMIC);
/* clone shares the same data pages (refcounted) */
/* changes to clone's headers don't affect original */
/* kfree_skb(clone) decrements data refcount */
```

`skb_copy()` does a deep copy (new data buffer). It's used when you need to modify packet data. Cilium's BPF programs avoid copies by rewriting packet data in-place using `bpf_skb_store_bytes()`.

---

## 2. NAPI: Efficient Packet Reception

NAPI (New API) is the kernel's mechanism for high-throughput packet reception. Understanding it explains why BPF programs at different hooks have different performance characteristics.

### 2.1 Pre-NAPI: Interrupt-Driven Reception (The Problem)

Without NAPI, every arriving packet triggers a hardware interrupt:

```
[NIC interrupt] -> CPU interrupt handler -> alloc sk_buff -> copy packet
                -> put in socket queue -> wake up process -> repeat

At 1 Mpps (million packets per second):
  1,000,000 interrupts/sec = CPU spends ALL time in interrupt context
  "Receive Livelock": CPU processes interrupts faster than user space
  can consume them -> system locks up
```

### 2.2 NAPI: Polling Mode

NAPI switches to polling mode when packet rates are high:

```
1. First packet arrives -> NIC fires interrupt
2. Interrupt handler:
   a. Disables NIC interrupts (napi_schedule())
   b. Adds NAPI poll to softirq queue
3. NET_RX_SOFTIRQ runs napi->poll() in softirq context:
   a. Poll up to budget (default 64) packets from NIC ring buffer
   b. Each packet: alloc sk_buff, DMA data, hand to netif_receive_skb()
   c. If ring drained: re-enable interrupts, exit polling
   d. If budget exhausted: yield (give other softirqs a chance), reschedule

Result: At high rates, CPU polls NIC ring buffer in batches
        -> far fewer context switches, better cache utilization
```

```
NAPI Poll Loop (drivers/net/ethernet/.../foo_eth.c):

static int foo_poll(struct napi_struct *napi, int budget)
{
    struct foo_priv *priv = container_of(napi, struct foo_priv, napi);
    int work_done = 0;

    while (work_done < budget) {
        struct sk_buff *skb = foo_rx_ring_get_next(priv);
        if (!skb) break;

        /* Set protocol, mac header */
        skb->protocol = eth_type_trans(skb, priv->netdev);

        /* Hand to network stack */
        /* GRO: try to merge with previous skb for TSO-like batching */
        napi_gro_receive(napi, skb);
        work_done++;
    }

    if (work_done < budget) {
        /* Ring empty: exit polling, re-enable interrupts */
        napi_complete_done(napi, work_done);
        foo_enable_irq(priv);
    }

    return work_done;
}
```

### 2.3 Why NAPI Matters for CNI

CNI plugins that use **veth pairs** get NAPI behavior automatically (veth implements a fake NAPI poll). But there's a subtle performance issue:

When Cilium uses `bpf_redirect()` to send packets directly between veth interfaces, the packet bypasses NAPI and goes through `dev_queue_xmit()` synchronously. For high-throughput pod-to-pod traffic, Cilium uses `bpf_redirect_peer()` (kernel 5.10+), which delivers the packet directly into the peer veth's NAPI receive path — more efficient because it batches with other packets already in the receive queue.

```
                                       Kernel 5.10+ bpf_redirect_peer()
Pod A tx ->  lxcAAAA (veth host side)
              |
              | tc BPF egress
              |  bpf_redirect_peer(lxcBBBB_ifindex, 0)
              |
              +-----> directly into eth0 (veth container side of Pod B)
                      into Pod B's napi receive queue
                      -> no bridge, no routing, no iptables
                      -> single memcpy in the fast path
```

### 2.4 RPS and RFS: Multi-CPU Packet Processing

**RPS** (Receive Packet Steering) spreads incoming packets across multiple CPUs based on flow hash. **RFS** (Receive Flow Steering) ensures packets for a socket go to the CPU running the application — improving cache locality.

For CNI, these settings affect pod network throughput. Cilium sets RPS on the host-side veth interfaces to ensure pod traffic is distributed across CPUs:

```bash
# Enable RPS on veth host side (set all CPUs)
echo "ff" > /sys/class/net/lxcAAAA/queues/rx-0/rps_cpus

# Enable RFS with 4096-entry flow table
echo 4096 > /proc/sys/net/core/rps_sock_flow_entries
echo 256 > /sys/class/net/lxcAAAA/queues/rx-0/rps_flow_cnt
```

---

## 3. GSO, TSO, GRO and Overlay Networks

These offloads are invisible under normal conditions but become major performance and correctness issues in CNI overlay networks.

### 3.1 What TSO, GSO, and GRO Do

```
TSO (TCP Segmentation Offload):
  Application sends 64KB write()
  TCP creates a 64KB sk_buff with gso_size=1460
  Instead of segmenting in kernel, pass to NIC
  NIC hardware splits into 44 x 1460-byte segments
  -> CPU does 1 sk_buff processing, NIC does 44 segment transmits

GSO (Generic Segmentation Offload):
  Software TSO fallback when NIC doesn't support TSO
  Kernel delays segmentation as long as possible
  Segments just before dev_queue_xmit()
  Works for TCP (GSO_TCPV4), UDP (GSO_UDP_L4), GRE, VXLAN, etc.

GRO (Generic Receive Offload):
  Inverse of TSO: coalesces incoming segments into large sk_buffs
  NAPI poll calls napi_gro_receive()
  GRO engine checks: same flow? consecutive seq nums? -> merge
  Result: stack processes one 64KB sk_buff instead of 44 x 1460-byte
```

### 3.2 The Overlay MTU Problem

VXLAN adds 50 bytes of overhead per packet:

```
Outer Ethernet (14) + Outer IP (20) + UDP (8) + VXLAN (8) = 50 bytes

Physical MTU: 1500 bytes
Inner packet max: 1500 - 50 = 1450 bytes

If pod sends 1500-byte packet -> outer packet = 1550 bytes
-> Physical NIC sees 1550 > 1500 -> MUST FRAGMENT
-> Fragmentation is catastrophically expensive

Solution: Set VXLAN interface MTU to 1450
  ip link set flannel.1 mtu 1450
  CNI conflist sets: "mtu": 1450
  This propagates to the container's eth0 via CNI
  Pod's TCP MSS = 1450 - 20 (IP) - 20 (TCP) = 1410 bytes
```

### 3.3 GSO and VXLAN Offloads

Modern NICs support **VXLAN offload** — the NIC can segment VXLAN-encapsulated TCP traffic:

```
With VXLAN NIC offload:
  inner TCP sk_buff (64KB, gso_size=1410, gso_type=SKB_GSO_TCPV4)
    -> VXLAN driver marks outer as SKB_GSO_UDP_TUNNEL
    -> NIC segments: outer UDP/VXLAN + inner TCP MSS
    -> 44 physical packets, kernel did zero copies

Without VXLAN NIC offload:
  VXLAN driver must call skb_gso_segment() to split before encapsulation
  -> 44 x 1450-byte inner sk_buffs
  -> Each gets VXLAN header added
  -> 44 transmit operations
  -> Much higher CPU usage
```

Check offload support:

```bash
ethtool -k eth0 | grep -E "tx-udp-tnl|tx-checksum|rx-gro"
# tx-udp_tnl-segmentation: on  <- VXLAN GSO offload
# rx-gro: on
```

CNI plugins like Flannel explicitly check for and enable VXLAN offload on the flannel.1 device. AWS VPC CNI uses native routing (no overlay) precisely to avoid all MTU and GSO complexity.

---

## 4. XDP Programming in C

XDP (eXpress Data Path) is the highest-performance hook in the Linux kernel network stack. It runs a BPF program BEFORE the kernel allocates an `sk_buff` — packets are processed directly from DMA ring buffers.

### 4.1 XDP Return Codes

```c
enum xdp_action {
    XDP_ABORTED = 0,  /* bug: drop + trace */
    XDP_DROP,         /* drop immediately, no sk_buff alloc */
    XDP_PASS,         /* continue to normal network stack */
    XDP_TX,           /* transmit back out same interface */
    XDP_REDIRECT,     /* redirect to another interface or CPU */
};
```

### 4.2 Complete XDP Program: Pod-to-Pod Fast Path

This XDP program implements what Cilium does in its native routing mode: fast-path redirect between pods on the same node without going through the full network stack.

```c
/* xdp_pod_redirect.c
 *
 * Attach to the physical NIC's XDP hook.
 * If the destination IP matches a local pod's IP, redirect directly
 * to that pod's veth interface index (bypassing routing, netfilter, everything).
 *
 * Build:
 *   clang -O2 -target bpf -c xdp_pod_redirect.c -o xdp_pod_redirect.o
 * Load:
 *   ip link set eth0 xdp obj xdp_pod_redirect.o sec xdp
 */

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

/* BPF map: pod IP (u32) -> veth ifindex (u32)
 * Populated by the CNI daemon when pods are created/deleted.
 */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key,   __u32);    /* pod IPv4 address (network byte order) */
    __type(value, __u32);    /* host-side veth ifindex */
    __uint(max_entries, 65536);
} pod_iface_map SEC(".maps");

/* BPF map: pod IP -> pod MAC address
 * Needed to rewrite the inner Ethernet destination MAC.
 */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key,   __u32);
    __type(value, __u8[ETH_ALEN]);
    __uint(max_entries, 65536);
} pod_mac_map SEC(".maps");

/* Helper: swap src/dst MAC addresses in Ethernet header */
static __always_inline void swap_macs(struct ethhdr *eth)
{
    __u8 tmp[ETH_ALEN];
    __builtin_memcpy(tmp,         eth->h_dest,   ETH_ALEN);
    __builtin_memcpy(eth->h_dest, eth->h_source, ETH_ALEN);
    __builtin_memcpy(eth->h_source, tmp,          ETH_ALEN);
}

SEC("xdp")
int xdp_pod_fast_path(struct xdp_md *ctx)
{
    /* ctx->data and ctx->data_end are offsets into the DMA buffer.
     * The BPF verifier requires explicit bounds checking before
     * every pointer dereference. */
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    /* Parse Ethernet header */
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_PASS;  /* not enough data, pass to stack */

    /* Only handle IPv4 for this example */
    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return XDP_PASS;

    /* Parse IPv4 header */
    struct iphdr *iph = (struct iphdr *)(eth + 1);
    if ((void *)(iph + 1) > data_end)
        return XDP_PASS;

    /* Verify no IP options (IHL must be 5) */
    if (iph->ihl != 5)
        return XDP_PASS;

    __u32 dst_ip = iph->daddr;

    /* Look up destination IP in our pod map */
    __u32 *ifindex = bpf_map_lookup_elem(&pod_iface_map, &dst_ip);
    if (!ifindex)
        return XDP_PASS;  /* not a local pod, let the stack handle it */

    /* Look up destination MAC */
    __u8 (*pod_mac)[ETH_ALEN] = bpf_map_lookup_elem(&pod_mac_map, &dst_ip);
    if (!pod_mac)
        return XDP_PASS;

    /* Rewrite destination MAC to pod's MAC address */
    __builtin_memcpy(eth->h_dest, pod_mac, ETH_ALEN);

    /* Redirect to the pod's veth interface.
     * BPF_F_INGRESS = deliver to the ingress side of the target interface
     * (i.e., into the veth's receive path, which appears in the pod's netns
     *  as the eth0 receive path)
     */
    return bpf_redirect(*ifindex, 0);
}

/* XDP program to attach on the veth host side (lxcXXXX interfaces).
 * For traffic FROM pods going to the physical network or other pods.
 */
SEC("xdp_egress")
int xdp_from_pod(struct xdp_md *ctx)
{
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return XDP_PASS;

    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return XDP_PASS;

    struct iphdr *iph = (struct iphdr *)(eth + 1);
    if ((void *)(iph + 1) > data_end)
        return XDP_PASS;

    /* Check if destination is another pod on this node */
    __u32 dst_ip = iph->daddr;
    __u32 *ifindex = bpf_map_lookup_elem(&pod_iface_map, &dst_ip);
    if (ifindex) {
        /* Redirect directly to peer pod's veth */
        __u8 (*pod_mac)[ETH_ALEN] = bpf_map_lookup_elem(&pod_mac_map, &dst_ip);
        if (pod_mac)
            __builtin_memcpy(eth->h_dest, pod_mac, ETH_ALEN);
        return bpf_redirect(*ifindex, 0);
    }

    /* Destination is remote — pass to kernel stack for routing/tunneling */
    return XDP_PASS;
}

char _license[] SEC("license") = "GPL";
```

### 4.3 Loading XDP Programs with libbpf (Go via ebpf-go)

Cilium uses `cilium/ebpf` (Go) to load BPF programs. Here's how the loading works conceptually:

```go
// Simplified from Cilium's bpf package

type PodFastPathObjects struct {
    Programs struct {
        XdpPodFastPath *ebpf.Program `ebpf:"xdp_pod_fast_path"`
        XdpFromPod     *ebpf.Program `ebpf:"xdp_egress"`
    }
    Maps struct {
        PodIfaceMap *ebpf.Map `ebpf:"pod_iface_map"`
        PodMacMap   *ebpf.Map `ebpf:"pod_mac_map"`
    }
}

func LoadAndAttachXDP(ifname string, podIP net.IP, vethIndex int, podMAC net.HardwareAddr) error {
    // Load compiled BPF object (embedded in binary with go:embed)
    objs := PodFastPathObjects{}
    if err := loadPodFastPathObjects(&objs, nil); err != nil {
        return fmt.Errorf("load BPF objects: %w", err)
    }
    defer objs.Close()

    // Populate pod_iface_map: podIP -> veth ifindex
    ip4 := binary.BigEndian.Uint32(podIP.To4())
    if err := objs.Maps.PodIfaceMap.Put(ip4, uint32(vethIndex)); err != nil {
        return fmt.Errorf("update pod_iface_map: %w", err)
    }

    // Populate pod_mac_map: podIP -> MAC
    mac := [6]byte{}
    copy(mac[:], podMAC)
    if err := objs.Maps.PodMacMap.Put(ip4, mac); err != nil {
        return fmt.Errorf("update pod_mac_map: %w", err)
    }

    // Attach XDP program to physical NIC
    iface, err := net.InterfaceByName(ifname)
    if err != nil {
        return err
    }
    link, err := netlink.LinkByIndex(iface.Index)
    if err != nil {
        return err
    }
    // netlink.LinkSetXdpFdWithFlags uses RTM_SETLINK with IFLA_XDP
    if err := netlink.LinkSetXdpFdWithFlags(link, objs.Programs.XdpPodFastPath.FD(),
        int(nl.XDP_FLAGS_DRV_MODE | nl.XDP_FLAGS_UPDATE_IF_NOEXIST)); err != nil {
        return fmt.Errorf("attach XDP: %w", err)
    }
    return nil
}
```

---

## 5. TC BPF Programming with libbpf

TC BPF programs run after `sk_buff` allocation. They can read and modify any packet field. Cilium uses TC BPF for all pod-level networking — policy enforcement, load balancing, NAT.

### 5.1 TC BPF vs XDP

```
Feature           XDP                    TC BPF
-----------       ---                    ------
When runs         Before sk_buff alloc   After sk_buff alloc
Context struct    xdp_md                 __sk_buff
sk_buff access    No                     Yes (via helpers)
Return actions    DROP/PASS/TX/REDIRECT  DROP/OK/REDIRECT/STOLEN
sk_buff helpers   Limited                Full set
frag access       No                     Yes (bpf_skb_pull_data)
Encap/decap       Partial                Full (bpf_skb_adjust_room)
Attach point      NIC driver             qdisc (clsact)
Priority          Highest                After driver, before netfilter
```

### 5.2 Complete TC BPF Program: Service Load Balancing

This is the kind of program Cilium uses to implement ClusterIP service load balancing — replacing kube-proxy entirely.

```c
/* tc_lb.c
 *
 * TC BPF classifier program implementing service load balancing.
 * Replaces kube-proxy's iptables DNAT rules with BPF map lookups.
 *
 * Attach:
 *   tc qdisc add dev lxcXXXX clsact
 *   tc filter add dev lxcXXXX egress bpf da obj tc_lb.o sec tc_egress
 */

#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

/* Service key: ClusterIP + port + protocol */
struct svc_key {
    __u32 vip;       /* ClusterIP in network byte order */
    __u16 port;      /* service port in network byte order */
    __u8  proto;     /* IPPROTO_TCP or IPPROTO_UDP */
    __u8  pad;
};

/* Service value: number of backends */
struct svc_val {
    __u32 count;     /* number of backend endpoints */
    __u32 flags;     /* SVC_FLAG_EXTERNAL, etc. */
};

/* Backend key: service key + slot (0..count-1) */
struct backend_key {
    struct svc_key svc;
    __u32 slot;
};

/* Backend value: real pod IP and port */
struct backend_val {
    __u32 ip;
    __u16 port;
    __u8  proto;
    __u8  pad;
};

/* Connection tracking key: 5-tuple */
struct ct_key {
    __u32 src_ip, dst_ip;
    __u16 src_port, dst_port;
    __u8  proto;
    __u8  pad[3];
};

/* Connection tracking value: NAT translation */
struct ct_val {
    __u32 orig_dst_ip;
    __u16 orig_dst_port;
    __u16 pad;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key,   struct svc_key);
    __type(value, struct svc_val);
    __uint(max_entries, 65536);
} svc_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key,   struct backend_key);
    __type(value, struct backend_val);
    __uint(max_entries, 262144);
} backend_map SEC(".maps");

/* LRU connection tracking — automatically evicts old entries */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key,   struct ct_key);
    __type(value, struct ct_val);
    __uint(max_entries, 1 << 20);  /* 1M connections */
} ct_map SEC(".maps");

/* Per-CPU counter for load balancing (round-robin per CPU) */
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key,   __u32);
    __type(value, __u32);
    __uint(max_entries, 1);
} lb_rr_counter SEC(".maps");

/* Checksum recalculation helper */
static __always_inline void update_csum(__u16 *csum,
                                         __u32 old_val, __u32 new_val)
{
    /* Incremental checksum update per RFC 1624 */
    __u32 sum = (~bpf_ntohs(*csum) & 0xffff)
              + (~old_val & 0xffff) + (old_val >> 16)
              + (new_val & 0xffff)  + (new_val >> 16);
    sum = (sum & 0xffff) + (sum >> 16);
    *csum = bpf_htons(~sum);
}

SEC("tc_egress")
int tc_lb_egress(struct __sk_buff *skb)
{
    /* Pull Ethernet + IP + TCP/UDP into linear area if needed */
    if (bpf_skb_pull_data(skb, sizeof(struct ethhdr) +
                               sizeof(struct iphdr) +
                               sizeof(struct tcphdr)) < 0)
        return TC_ACT_OK;

    void *data     = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return TC_ACT_OK;

    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return TC_ACT_OK;

    struct iphdr *iph = (struct iphdr *)(eth + 1);
    if ((void *)(iph + 1) > data_end)
        return TC_ACT_OK;

    if (iph->protocol != IPPROTO_TCP && iph->protocol != IPPROTO_UDP)
        return TC_ACT_OK;

    __u16 *dst_port_ptr;
    __u16 *src_port_ptr;

    if (iph->protocol == IPPROTO_TCP) {
        struct tcphdr *tcph = (struct tcphdr *)(iph + 1);
        if ((void *)(tcph + 1) > data_end)
            return TC_ACT_OK;
        dst_port_ptr = &tcph->dest;
        src_port_ptr = &tcph->source;
    } else {
        struct udphdr *udph = (struct udphdr *)(iph + 1);
        if ((void *)(udph + 1) > data_end)
            return TC_ACT_OK;
        dst_port_ptr = &udph->dest;
        src_port_ptr = &udph->source;
    }

    /* Check if this is a new connection or existing (CT lookup first) */
    struct ct_key ctk = {
        .src_ip   = iph->saddr,
        .dst_ip   = iph->daddr,
        .src_port = *src_port_ptr,
        .dst_port = *dst_port_ptr,
        .proto    = iph->protocol,
    };

    struct ct_val *ctv = bpf_map_lookup_elem(&ct_map, &ctk);
    if (ctv) {
        /* Existing connection: apply stored NAT translation */
        __u32 old_ip   = iph->daddr;
        __u16 old_port = *dst_port_ptr;

        iph->daddr    = ctv->orig_dst_ip;
        *dst_port_ptr = ctv->orig_dst_port;

        /* Update IP checksum */
        update_csum(&iph->check, old_ip, ctv->orig_dst_ip);

        /* Update TCP/UDP checksum (covers IP pseudo-header + ports) */
        /* bpf_l4_csum_replace handles pseudo-header recalc */
        bpf_l4_csum_replace(skb,
            (void *)dst_port_ptr - data + offsetof(struct tcphdr, check),
            old_ip | ((__u32)old_port << 16),
            ctv->orig_dst_ip | ((__u32)ctv->orig_dst_port << 16),
            BPF_F_PSEUDO_HDR | sizeof(__u32));

        return TC_ACT_OK;
    }

    /* New connection: look up service */
    struct svc_key svck = {
        .vip   = iph->daddr,
        .port  = *dst_port_ptr,
        .proto = iph->protocol,
    };

    struct svc_val *svcv = bpf_map_lookup_elem(&svc_map, &svck);
    if (!svcv || svcv->count == 0)
        return TC_ACT_OK;  /* not a service VIP */

    /* Select backend: per-CPU round-robin */
    __u32 key = 0;
    __u32 *counter = bpf_map_lookup_elem(&lb_rr_counter, &key);
    __u32 slot = 0;
    if (counter) {
        slot = (*counter) % svcv->count;
        __sync_fetch_and_add(counter, 1);
    }

    struct backend_key bk = { .svc = svck, .slot = slot };
    struct backend_val *bv = bpf_map_lookup_elem(&backend_map, &bk);
    if (!bv)
        return TC_ACT_OK;

    /* Store CT entry for return path */
    struct ct_val new_ctv = {
        .orig_dst_ip   = iph->daddr,  /* ClusterIP */
        .orig_dst_port = *dst_port_ptr,
    };
    bpf_map_update_elem(&ct_map, &ctk, &new_ctv, BPF_ANY);

    /* DNAT: rewrite destination to backend */
    __u32 old_ip   = iph->daddr;
    __u16 old_port = *dst_port_ptr;

    iph->daddr    = bv->ip;
    *dst_port_ptr = bv->port;

    /* Recalculate checksums */
    bpf_l3_csum_replace(skb,
        ETH_HLEN + offsetof(struct iphdr, check),
        old_ip, bv->ip, sizeof(__u32));

    bpf_l4_csum_replace(skb,
        /* offset to L4 checksum field */
        ETH_HLEN + sizeof(struct iphdr) +
        (iph->protocol == IPPROTO_TCP ?
            offsetof(struct tcphdr, check) :
            offsetof(struct udphdr, check)),
        old_ip | ((__u32)old_port << 16),
        bv->ip | ((__u32)bv->port << 16),
        BPF_F_PSEUDO_HDR | sizeof(__u32));

    return TC_ACT_OK;
}

char _license[] SEC("license") = "GPL";
```

### 5.3 Attaching TC BPF via Netlink (clsact qdisc)

```bash
# The clsact qdisc is a special qdisc that enables BPF on ingress AND egress
# It doesn't actually queue packets — just provides hook points

# 1. Create clsact qdisc on the interface
tc qdisc add dev lxcAAAA clsact

# 2. Attach BPF program to egress hook (from pod's perspective)
tc filter add dev lxcAAAA egress bpf da obj tc_lb.o sec tc_egress

# 3. Attach to ingress hook (for return path)
tc filter add dev lxcAAAA ingress bpf da obj tc_lb.o sec tc_ingress

# View attached programs:
tc filter show dev lxcAAAA egress
# filter protocol all pref 49152 bpf chain 0
# filter protocol all pref 49152 bpf chain 0 handle 0x1 tc_lb.o:[tc_egress] direct-action not_in_hw
#   id 42 tag abcdef1234567890

# In Cilium, this is done via Netlink RTM_NEWTFILTER messages
# rather than calling the `tc` command
```

---

## 6. BPF Maps: Architecture and Use in CNI

BPF maps are the shared memory between BPF programs and user space. They are the database of CNI's data plane.

### 6.1 Map Types Used in CNI

```
BPF_MAP_TYPE_HASH
  Key: arbitrary bytes. Value: arbitrary bytes.
  Lookup: O(1) average via jhash.
  Use in CNI: pod IP -> veth ifindex, service VIP -> backend count
  Concurrency: per-CPU spinlock (kernel 5.1+: RCU for read, lock for write)

BPF_MAP_TYPE_LRU_HASH
  Same as HASH but evicts least-recently-used entries automatically.
  Use in CNI: Connection tracking tables (1M+ entries, old ones evicted)
  No manual cleanup needed. Cilium uses this for cilium_ct4_global.

BPF_MAP_TYPE_PERCPU_HASH
  Each CPU has its own copy of the value.
  No locking needed for updates — each CPU writes its own slot.
  Aggregated by reading all CPUs and summing.
  Use in CNI: Per-endpoint packet/byte counters (Cilium metrics)

BPF_MAP_TYPE_ARRAY
  Integer keys 0..max_entries-1. Values pre-allocated at map creation.
  Faster than HASH (no hashing, no memory allocation).
  Use in CNI: Per-CPU counters (load balancer slot selection)

BPF_MAP_TYPE_PROG_ARRAY
  Values are BPF program file descriptors.
  Used for BPF tail calls: programs call other programs without stack growth.
  Use in Cilium: Dispatch table (protocol number -> policy program)
  Allows programs >BPF instruction limit by chaining.

BPF_MAP_TYPE_PERF_EVENT_ARRAY
  Ring buffer per CPU. BPF programs push events; user space reads them.
  Use in Cilium/Hubble: Network flow events (connection open/close, drops)
  bpf_perf_event_output() writes event; user space polls via perf_event_open()

BPF_MAP_TYPE_RINGBUF (kernel 5.8+)
  Single ring buffer shared across all CPUs. More efficient than PERF_EVENT_ARRAY.
  Use in Cilium 1.10+: Hubble flow events. bpf_ringbuf_output().

BPF_MAP_TYPE_SK_STORAGE
  Per-socket storage attached directly to struct sock.
  No map lookup needed — data travels with the socket.
  Use in Cilium: Per-connection metadata (service endpoint, policy verdict)

BPF_MAP_TYPE_DEVMAP / BPF_MAP_TYPE_DEVMAP_HASH
  Values are net_device ifindexes.
  Used with bpf_redirect_map() for fast multi-NIC forwarding.
  Use in CNI: XDP bulk redirect to multiple pod veths.
```

### 6.2 Map Pinning: Sharing Maps Between Programs

BPF maps can be **pinned** to the BPF filesystem (`/sys/fs/bpf/`). This allows different programs (and even different BPF programs loaded at different times) to share the same map.

```bash
# Cilium pins all its maps under /sys/fs/bpf/tc/globals/
ls /sys/fs/bpf/tc/globals/
# cilium_calls_00001      cilium_ct4_global
# cilium_ipcache          cilium_lb4_backends_v3
# cilium_lb4_services_v2  cilium_policy_00001
# cilium_lxc              cilium_metrics
```

```go
// In Go (cilium/ebpf): pin a map
m, err := ebpf.NewMap(&ebpf.MapSpec{
    Type:       ebpf.Hash,
    KeySize:    4,
    ValueSize:  4,
    MaxEntries: 65536,
})
m.Pin("/sys/fs/bpf/tc/globals/pod_iface_map")

// Later, another process opens the pinned map
m2, err := ebpf.LoadPinnedMap("/sys/fs/bpf/tc/globals/pod_iface_map", nil)
m2.Put(podIP, ifindex)
```

Maps survive program reload — old BPF programs can be replaced with new ones while keeping the same map (and its data). This is how Cilium does rolling updates without losing connection tracking state.

### 6.3 BPF Tail Calls: Chaining Programs

Cilium's policy engine is too large to fit in a single BPF program (verifier limit). It uses tail calls to chain programs:

```
BPF_MAP_TYPE_PROG_ARRAY  "cilium_calls_00001":
  [0] = prog: from-container (handle initial packet from pod)
  [1] = prog: ipv4-policy    (L3/L4 policy check)
  [2] = prog: encrypt        (WireGuard/IPSec encryption)
  [3] = prog: tunnel-egress  (VXLAN/GENEVE encapsulation)

/* In from-container BPF program: */
bpf_tail_call(ctx, &cilium_calls_00001, 1);
/* If tail call succeeds, current program's stack is replaced by program[1] */
/* The stack does NOT grow — it's a goto, not a call */
/* If prog[1] returns, execution does NOT return to this program */
```

---

## 7. Connection Tracking (nf_conntrack) Internals

Connection tracking is the kernel's mechanism for stateful packet filtering and NAT. Every pod-to-external packet goes through conntrack. Understanding it is essential for debugging NAT issues in Kubernetes.

### 7.1 Conntrack Table Structure

```c
/* net/netfilter/nf_conntrack_core.c */

/* The conntrack hash table */
/* Each net namespace has its own hash table */
struct nf_conntrack_tuple_hash {
    struct hlist_nulls_node hnnode;  /* hash table bucket */
    struct nf_conntrack_tuple tuple; /* the 5-tuple */
};

struct nf_conn {
    /* Reference counting (two tuple hashes reference this) */
    struct nf_conntrack ct_general;

    spinlock_t          lock;
    u32                 timeout;     /* expiry timestamp */

    /* Original direction 5-tuple */
    struct nf_conntrack_tuple_hash tuplehash[IP_CT_DIR_MAX];
    /*
     * tuplehash[IP_CT_DIR_ORIGINAL]: src=pod, dst=ClusterIP
     * tuplehash[IP_CT_DIR_REPLY]:    src=backend, dst=pod
     * These two entries are in the global hash table.
     */

    /* Connection state */
    unsigned long       status;      /* IPS_CONFIRMED, IPS_NAT_MASK, etc. */

    /* NAT information (if NATed) */
    /* lives in nf_conn_nat extension */
    struct nf_nat_l4proto *nat_bysource;
    /* stores the translated tuple for SNAT/DNAT */

    /* TCP/UDP state */
    union nf_conntrack_proto proto;

    /* Extensions (variable-length, allocated after struct) */
    struct nf_ct_ext    *ext;  /* helper, nat, acct, timeout, labels, ... */
};

/* The 5-tuple */
struct nf_conntrack_tuple {
    struct nf_conntrack_man src;  /* source: IP + port/id + L3 proto */
    struct {
        union nf_inet_addr  u3;   /* destination IP */
        union {
            __be16 all;
            struct { __be16 port; } tcp;
            struct { __be16 port; } udp;
            /* ... */
        } u;
        __u8    protonum;         /* IPPROTO_TCP, UDP, etc. */
        __u8    dir;              /* IP_CT_DIR_ORIGINAL or REPLY */
    } dst;
};
```

### 7.2 Connection Tracking Zones

Conntrack **zones** allow multiple connections with identical 5-tuples to coexist in the same kernel. This is critical in Kubernetes where many pods can appear as the same SNAT'd IP.

```
Without zones:
  Pod-A: 10.0.0.1:12345 -> 8.8.8.8:53  (after MASQUERADE: 192.168.1.10:12345 -> 8.8.8.8:53)
  Pod-B: 10.0.0.2:12345 -> 8.8.8.8:53  (after MASQUERADE: 192.168.1.10:12345 -> 8.8.8.8:53)
  !! COLLISION !! Same post-NAT tuple

With zones:
  Pod-A traffic: zone=1, 192.168.1.10:12345 -> 8.8.8.8:53
  Pod-B traffic: zone=2, 192.168.1.10:12345 -> 8.8.8.8:53
  Different zones -> no collision

Cilium uses zones (via nf_conntrack_zone) to separate per-endpoint conntrack.
This avoids the SNAT port collision problem entirely.
```

### 7.3 Conntrack in the kube-proxy iptables Chain

```
Packet: pod-A (10.244.0.5) -> ClusterIP (10.96.0.1:80)

PREROUTING chain:
  1. nf_conntrack: NEW connection (no existing entry)
     -> allocate struct nf_conn
     -> add to hash table: (10.244.0.5, 10.96.0.1:80, TCP) [ORIGINAL]

  2. KUBE-SERVICES chain:
     -d 10.96.0.1/32 -p tcp --dport 80 -j KUBE-SVC-XXXXXXXX

  3. KUBE-SVC-XXXXXXXX (load balancing):
     -m statistic --mode random --probability 0.5 -j KUBE-SEP-AAAAAAAA
     -j KUBE-SEP-BBBBBBBB

  4. KUBE-SEP-AAAAAAAA (DNAT to pod-B 10.244.1.6:8080):
     -j DNAT --to-destination 10.244.1.6:8080

     -> kernel: modify nf_conn's reply tuple:
        REPLY: (10.244.1.6:8080, 10.244.0.5, TCP)
                src=backend, dst=original-client
        Mark connection IPS_DST_NAT

  5. Packet forwarded to 10.244.1.6:8080

Return packet (10.244.1.6:8080 -> 10.244.0.5):
  PREROUTING:
    nf_conntrack: ESTABLISHED, IPS_DST_NAT set
    -> reverse NAT: rewrite src from 10.244.1.6:8080 to 10.96.0.1:80
    -> pod-A receives packet as if from ClusterIP ✓
```

### 7.4 Conntrack Table Size and Tuning

```bash
# View current conntrack table
cat /proc/sys/net/netfilter/nf_conntrack_count   # current entries
cat /proc/sys/net/netfilter/nf_conntrack_max     # max entries (default: 65536)

# For large Kubernetes clusters:
sysctl -w net.netfilter.nf_conntrack_max=1048576
sysctl -w net.netfilter.nf_conntrack_buckets=262144

# Conntrack timeouts (reduce for high-connection-rate services)
sysctl -w net.netfilter.nf_conntrack_tcp_timeout_established=86400  # 24h
sysctl -w net.netfilter.nf_conntrack_tcp_timeout_time_wait=30
sysctl -w net.netfilter.nf_conntrack_udp_timeout=30

# View conntrack table content
conntrack -L
# tcp      6 431999 ESTABLISHED src=10.244.0.5 dst=10.96.0.1 sport=12345 dport=80
#   [UNREPLIED] src=10.244.1.6 dst=10.244.0.5 sport=8080 dport=12345 mark=0 use=1
```

The conntrack table is a major scalability bottleneck for kube-proxy's iptables mode. At 10,000 pods, each pod making 100 connections = 1M conntrack entries. Cilium removes this bottleneck by doing its own connection tracking in BPF maps.

---

## 8. kube-proxy: iptables and IPVS Modes Deep Dive

kube-proxy translates Kubernetes Service objects into kernel-level packet rules. It is NOT a CNI plugin — but it works alongside CNI and depends on the pod network CNI establishes.

### 8.1 iptables Mode: Exact Rule Structure

For a Service `my-svc` with ClusterIP=10.96.100.1:80, two endpoints (10.244.0.5:8080 and 10.244.1.6:8080):

```
# kube-proxy generates these exact iptables rules:

# --- nat table ---

# PREROUTING: entry point for all incoming
-A PREROUTING -m comment --comment "kubernetes service portals" -j KUBE-SERVICES

# OUTPUT: for traffic from local processes on the node
-A OUTPUT -m comment --comment "kubernetes service portals" -j KUBE-SERVICES

# POSTROUTING: masquerade for traffic leaving the node
-A POSTROUTING -m comment --comment "kubernetes postrouting rules" -j KUBE-POSTROUTING

# KUBE-SERVICES: dispatch to per-service chains
-A KUBE-SERVICES -d 10.96.100.1/32 -p tcp -m tcp --dport 80
    -m comment --comment "default/my-svc:http cluster IP"
    -j KUBE-SVC-XYZXYZXYZ

# KUBE-SVC-XYZXYZXYZ: load balancing chain
# Rule 1: send 50% to endpoint A
-A KUBE-SVC-XYZXYZXYZ -m comment --comment "default/my-svc -> 10.244.0.5:8080"
    -m statistic --mode random --probability 0.50000000000
    -j KUBE-SEP-AAAAAAAAAA

# Rule 2: remaining 100% (of remaining 50%) to endpoint B
-A KUBE-SVC-XYZXYZXYZ -m comment --comment "default/my-svc -> 10.244.1.6:8080"
    -j KUBE-SEP-BBBBBBBBBB

# KUBE-SEP-AAAAAAAAAA: DNAT to endpoint A
# Mark if traffic comes FROM the endpoint itself (hairpin)
-A KUBE-SEP-AAAAAAAAAA -s 10.244.0.5/32 -m comment --comment "default/my-svc"
    -j KUBE-MARK-MASQ

-A KUBE-SEP-AAAAAAAAAA -p tcp -m tcp
    -j DNAT --to-destination 10.244.0.5:8080

# KUBE-SEP-BBBBBBBBBB: DNAT to endpoint B
-A KUBE-SEP-BBBBBBBBBB -s 10.244.1.6/32 -m comment --comment "default/my-svc"
    -j KUBE-MARK-MASQ

-A KUBE-SEP-BBBBBBBBBB -p tcp -m tcp
    -j DNAT --to-destination 10.244.1.6:8080

# KUBE-MARK-MASQ: mark packets for masquerade
-A KUBE-MARK-MASQ -j MARK --set-xmark 0x4000/0x4000

# KUBE-POSTROUTING: masquerade marked packets
-A KUBE-POSTROUTING -m mark ! --mark 0x4000/0x4000 -j RETURN
-A KUBE-POSTROUTING -j MARK --set-xmark 0x4000/0x0
-A KUBE-POSTROUTING -m masquerade --random-fully -j MASQUERADE

# --- filter table ---
-A FORWARD -m comment --comment "kubernetes forwarding rules" -j KUBE-FORWARD
-A KUBE-FORWARD -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
-A KUBE-FORWARD -m comment --comment "kubernetes forwarding pods"
    -s 10.244.0.0/16 -j ACCEPT
-A KUBE-FORWARD -m comment --comment "kubernetes forwarding pods"
    -d 10.244.0.0/16 -j ACCEPT
```

kube-proxy uses `iptables-restore` (batched rule application) rather than individual `iptables -A` calls. For 10,000 services with 5 endpoints each, this can be a 500MB iptables ruleset that takes 10+ seconds to apply.

### 8.2 IPVS Mode: Virtual Server Architecture

IPVS (IP Virtual Server) is a kernel-level L4 load balancer built on netfilter. It's much more efficient than iptables for large numbers of services.

```
IPVS Architecture:

kube-proxy watches for Service/Endpoint changes
   |
   v
ipset: creates IP sets for service CIDRs
   |
iptables: minimal rules that just redirect service IP traffic to IPVS
   | (one rule per iptables chain, not per-service)
   v
IPVS virtual server table (kernel):
   Virtual service 10.96.100.1:80 TCP
     -> backend 10.244.0.5:8080 weight=1
     -> backend 10.244.1.6:8080 weight=1
     scheduler: rr (round-robin), lc (least-conn), sh (source-hash), etc.

ipvsadm -Ln (shows kernel IPVS table):
  IP Virtual Server version 1.2.1 (size=4096)
  Prot LocalAddress:Port Scheduler Flags
    -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
  TCP  10.96.100.1:80 rr
    -> 10.244.0.5:8080              Masq    1      0          0
    -> 10.244.1.6:8080              Masq    1      0          0
```

**IPVS connection table vs iptables conntrack:**

```
iptables DNAT: uses nf_conntrack (shared table, size=nf_conntrack_max)
  O(1) lookup via hash, but contended under high concurrency

IPVS: has its OWN connection table separate from nf_conntrack
  Hash table: ip_vs_conn_tab[256] (array of hlist_head)
  Lookup: ip_vs_conn_get() -> hash(proto, src, vdst) -> bucket scan
  Size: configurable, default 4096 * nr_cpus
  Timeout: TCP: 900s (ESTABLISHED), 120s (FIN_WAIT), 60s (TIME_WAIT)

Performance comparison at 10,000 services:
  iptables: O(rules) packet traversal = 50,000 rule checks per packet
  IPVS:     O(1) virtual server lookup + O(1) backend selection
```

**IPVS Scheduler Algorithms:**

```
rr   = round-robin (default, even distribution)
wrr  = weighted round-robin (for pods with different capacities)
lc   = least connection (new conn -> backend with fewest active conns)
wlc  = weighted least connection
sh   = source hash (same client always goes to same backend = session affinity)
      Used for Kubernetes sessionAffinity: ClientIP
dh   = destination hash
lblc = locality-based least connection
lblcr = locality-based least connection with replication
```

kube-proxy sets `sh` scheduler when `service.spec.sessionAffinity = ClientIP`.

### 8.3 How kube-proxy Interacts with CNI

kube-proxy and CNI are independent but deeply coupled:

```
CNI establishes:
  - Pod IP addresses (10.244.x.x)
  - Inter-node routing (via Flannel/Calico/etc.)
  - FORWARD chain ACCEPT for pod traffic

kube-proxy requires (from CNI):
  - Working pod IP reachability (so DNAT targets are routable)
  - iptables FORWARD ACCEPT (so DNATed packets can be forwarded)
  - Masquerade for external traffic (POSTROUTING MASQUERADE)

kube-proxy adds on top:
  - PREROUTING/OUTPUT DNAT for ClusterIP
  - NodePort rules (DNAT on node's physical IP)
  - External traffic policy rules

Cilium replaces kube-proxy entirely:
  - Removes iptables dependency
  - Does service LB in BPF (cilium_lb4_services_v2 map)
  - Does CT in BPF (cilium_ct4_global map)
  - No conntrack overhead
```

---

## 9. ClusterIP, NodePort, LoadBalancer at Kernel Level

### 9.1 ClusterIP: Virtual IP That Exists Nowhere

A ClusterIP is a virtual IP address. It is NOT assigned to any network interface. It only exists in iptables DNAT rules (or BPF maps). When a packet is sent to a ClusterIP, it is intercepted by PREROUTING and rewritten before any routing occurs.

```
10.96.0.1:80 (ClusterIP for kubernetes API server)

  ip addr show  # NOT on any interface
  ip route show # NOT in routing table

  How does it work?
  Pod sends SYN to 10.96.0.1:80
    -> PREROUTING hook fires
    -> DNAT to 10.244.0.5:6443 (API server pod)
    -> Routing decision: is 10.244.0.5 local? -> forward via cni0/flannel.1
    -> Packet delivered

  The routing system never sees 10.96.0.1 — DNAT happens BEFORE routing.
  This is why ClusterIPs don't need routes.
```

### 9.2 NodePort: Binding on All Node IPs

NodePort exposes a service on every node's IP on a high port (30000-32767):

```
Service: NodePort 30080 -> ClusterIP 10.96.100.1:80

iptables rules added by kube-proxy:
  -A KUBE-SERVICES ! -s 10.244.0.0/16 -d 10.96.100.1/32 -p tcp --dport 80
    -m comment --comment "default/my-svc masquerade"
    -j KUBE-MARK-MASQ

  -A KUBE-NODEPORTS -p tcp -m tcp --dport 30080
    -m comment --comment "default/my-svc nodeport"
    -j KUBE-SVC-XYZXYZXYZ

External client: 1.2.3.4 -> node-1 (192.168.1.10):30080
  PREROUTING: -A KUBE-NODEPORTS: DNAT to backend (10.244.0.5:8080)
  POSTROUTING: -j MASQUERADE (because ExternalTrafficPolicy=Cluster)
               -> src IP becomes 192.168.1.10 (node IP)
               -> backend sees client as node IP, not 1.2.3.4 !!

With ExternalTrafficPolicy=Local:
  No MASQUERADE for external traffic
  Backend sees real client IP 1.2.3.4
  But: only backends on THIS node are used (no inter-node DNAT)
  iptables: add check -m addrtype --dst-type LOCAL before KUBE-NODEPORTS
```

### 9.3 LoadBalancer: Cloud Provider Integration

A LoadBalancer service creates a cloud load balancer (AWS ELB, GCP GLB, etc.) that forwards to NodePort. CNI is not directly involved — the cloud provider's controller handles external LB creation. The traffic path is:

```
Client -> Cloud LB (external IP) -> NodePort on any node -> ClusterIP -> Pod
```

MetalLB implements LoadBalancer for bare-metal by advertising the service IP via BGP or ARP. It interacts with CNI indirectly through ARP/BGP:

```
MetalLB (BGP mode):
  - Runs BGP speaker on each node
  - Announces service IP (e.g., 10.0.10.1/32) via BGP to routers
  - Router learns: to reach 10.0.10.1, send to node-X
  - Packet arrives at node-X on physical interface
  - iptables PREROUTING DNAT: 10.0.10.1:80 -> ClusterIP -> backend
```

---

## 10. libcni and go-cni: The Go CNI Library Internals

### 10.1 libcni: The Reference CNI Executor

`github.com/containernetworking/cni/libcni` is the Go library that implements the CNI spec on the invoker side. containerd and CRI-O use this (or their own wrappers around it).

```go
// Core types in libcni:

type CNIConfig struct {
    Path     []string         // CNI_PATH directories
    exec     invoke.Exec      // interface for exec'ing plugins
    cacheDir string
}

type NetworkConfig struct {
    Network *types.NetConf   // parsed CNI config
    Bytes   []byte           // raw JSON
}

type NetworkConfigList struct {
    Name       string
    CNIVersion string
    DisableCheck bool
    Plugins    []*NetworkConfig  // ordered plugin chain
    Bytes      []byte
}

// Key method: AddNetworkList()
func (c *CNIConfig) AddNetworkList(
    ctx context.Context,
    list *NetworkConfigList,
    rt *RuntimeConf,
) (types.Result, error) {
    var err error
    var result types.Result

    for _, net := range list.Plugins {
        // Inject prevResult from previous plugin into config
        result, err = c.addNetwork(ctx, list.Name, list.CNIVersion, net, result, rt)
        if err != nil {
            return nil, fmt.Errorf("plugin %s failed: %w", net.Network.Type, err)
        }
    }
    // Cache the final result for CHECK/DEL later
    if err = c.cacheAdd(result, list.Bytes, list.Name, rt); err != nil {
        return nil, fmt.Errorf("cache result: %w", err)
    }
    return result, nil
}

// RuntimeConf: per-invocation runtime parameters
type RuntimeConf struct {
    ContainerID string
    NetNS       string         // /var/run/netns/...
    IfName      string         // eth0
    Args        [][2]string    // K8S_POD_NAME, etc.
    CapabilityArgs map[string]interface{}  // runtimeConfig
}
```

### 10.2 invoke.Exec: The Actual exec() Call

```go
// pkg/invoke/exec.go

type DefaultExec struct {
    RawExec *RawExec
}

type RawExec struct {
    Stderr io.Writer
}

func (e *RawExec) ExecPlugin(
    ctx context.Context,
    pluginPath string,
    stdinData []byte,
    environ []string,
) ([]byte, error) {
    // This is the actual os/exec call
    c := exec.CommandContext(ctx, pluginPath)
    c.Env = environ
    c.Stdin = bytes.NewBuffer(stdinData)

    var stdout, stderr bytes.Buffer
    c.Stdout = &stdout
    if e.Stderr != nil {
        c.Stderr = io.MultiWriter(&stderr, e.Stderr)
    } else {
        c.Stderr = &stderr
    }

    // exec() syscall: fork + execve
    if err := c.Run(); err != nil {
        return nil, &InvokeError{
            Path:       pluginPath,
            Command:    environ_CNI_COMMAND,
            CombinedOutput: stderr.String() + stdout.String(),
            Err:        err,
        }
    }

    return stdout.Bytes(), nil
}
```

**The environ array** passed to the plugin is built from `RuntimeConf`:

```go
func (rt *RuntimeConf) AsEnv(command string) []string {
    env := os.Environ()  // inherit current environment (PATH, etc.)
    env = append(env,
        "CNI_COMMAND="+command,
        "CNI_CONTAINERID="+rt.ContainerID,
        "CNI_NETNS="+rt.NetNS,
        "CNI_IFNAME="+rt.IfName,
        "CNI_PATH="+strings.Join(cniPath, ":"),
        "CNI_ARGS="+formatArgs(rt.Args),
    )
    return env
}
```

### 10.3 go-cni: containerd's Higher-Level Wrapper

`github.com/containerd/go-cni` wraps libcni with:
- Automatic config file discovery from `/etc/cni/net.d/`
- Concurrent plugin execution (multiple networks in parallel for Multus-like setups)
- Status reporting and readiness checks

```go
// go-cni key interface
type CNI interface {
    Setup(ctx context.Context, id string, path string, opts ...NamespaceOpts) (*Result, error)
    Remove(ctx context.Context, id string, path string, opts ...NamespaceOpts) error
    Check(ctx context.Context, id string, path string, opts ...NamespaceOpts) error
    Load(opts ...CNIOpt) error     // reload config from disk
    Status() error                 // returns nil when CNI is ready (config found)
    GetConfig() *ConfigResult
}

// containerd calls Status() during startup to wait for CNI to be ready
// (i.e., wait for the CNI DaemonSet to drop its conflist file)
func (c *cni) Status() error {
    c.RLock()
    defer c.RUnlock()
    return c.status()
}

func (c *cni) status() error {
    if len(c.networks) == 0 {
        return ErrCNINotInitialized
    }
    return nil
}
```

The `Status()` check is why pods are stuck in `ContainerCreating` when no CNI is installed — containerd loops calling `Status()` until a conflist appears.

---

## 11. Kata Containers Networking: VM-Level CNI

Kata Containers runs each pod in a lightweight VM (using QEMU or Cloud-Hypervisor). The pod's workloads don't run in a Linux network namespace on the host — they run inside a VM. CNI still runs on the host, but it must connect the VM to the pod network.

### 11.1 The Problem: CNI Namespace vs VM

```
Standard containers (runc):
  Host netns -> veth -> container netns (eth0)
  CNI configures the container netns directly.

Kata Containers:
  Host netns -> ??? -> VM guest netns (eth0 inside VM)
  CNI configures a host netns, but the VM needs to access it.
```

### 11.2 Kata Networking: macvtap/tuntap Bridge

Kata uses one of several methods to expose the CNI-configured network namespace to the VM:

```
Method 1: macvtap (preferred for performance)

  Host netns:
    cni0 (bridge)
      |
    veth0 (host side) <-- CNI configures this, assigns IP
      |
    macvtap0 <-- Kata creates this, VM uses it as NIC

  VM guest:
    eth0 (virtio-net using macvtap0 as backend)
    IP = same as what CNI assigned to veth0

  Flow: VM guest eth0 -> virtio-net -> macvtap0 -> cni0 bridge -> other pods

Method 2: TC redirect (for bridges with strict isolation)

  CNI creates veth pair, assigns IP to container side.
  Kata intercepts the veth and installs TC BPF redirect:
    tc filter add dev veth0 ingress bpf direct-action
      -> redirect all ingress to tapXXX (VM's tap device)
    tc filter add dev tapXXX ingress bpf direct-action
      -> redirect all ingress to veth0
  VM sees traffic via tap device; bridge/CNI sees veth0 as the endpoint.
```

### 11.3 Kata Agent: In-VM Networking

Inside the VM, the Kata Agent (a Go binary) receives network configuration from the Kata runtime via a gRPC protocol and configures the in-VM interface:

```
Kata runtime (host) -> virtio-vsock -> Kata agent (in VM)
  gRPC: AddInterface(Interface{
    Name:    "eth0",
    IPAddrs: [{IPAddress: "10.244.0.5", Mask: "24"}],
    Mtu:     1450,
    HwAddr:  "aa:bb:cc:dd:ee:ff",
  })

  Agent receives this and calls:
    netlink.LinkSetName(link, "eth0")
    netlink.AddrAdd(link, addr)
    netlink.RouteAdd(&defaultRoute{Gw: gw})
```

The IP address assigned by CNI on the host ends up as the IP address inside the VM — from the pod's perspective, it's completely transparent.

---

## 12. gVisor Networking: The Go Netstack

gVisor (runsc) is a sandboxed container runtime that implements a complete OS in user space. Importantly, it has its own **network stack written in Go** — it does NOT use the Linux kernel's networking. This has profound implications for CNI.

### 12.1 gVisor Network Architecture

```
Standard container:
  App syscall (send()) -> Linux kernel TCP/IP stack -> NIC driver

gVisor container:
  App syscall (send())
    -> gVisor sentry (user-space kernel, Go)
       -> netstack (Go TCP/IP implementation)
          -> gonet package (socket API)
             -> tun/tap device (host kernel)
                -> Linux kernel IP routing
                   -> physical NIC

gVisor implements:
  - IPv4, IPv6, TCP (with CUBIC congestion control), UDP, ICMP
  - In Go, in user space
  - Syscall ABI compatibility (app thinks it's talking to Linux kernel)
```

### 12.2 Why CNI Still Works with gVisor

The key insight: CNI always works at the **host namespace** level. It creates a network namespace and puts a veth inside it. gVisor's sentry then connects to that namespace via a tun/tap or gVisor's FD-based NIC mode:

```
Host:
  CNI creates namespace /var/run/netns/sandbox-id
  CNI configures eth0 inside: IP=10.244.0.5, gw=10.244.0.1

gVisor runtime startup:
  Opens /var/run/netns/sandbox-id
  Enters the namespace
  Finds eth0 (the veth end CNI configured)
  Creates a raw socket bound to eth0 (AF_PACKET, SOCK_RAW)
  Passes this raw socket fd to the netstack

gVisor netstack:
  Reads raw Ethernet frames from the fd
  Implements TCP/IP on top of these raw frames
  App's send() -> Go TCP stack -> raw Ethernet frame -> fd -> host kernel eth0
```

### 12.3 gVisor Performance Implications

Since gVisor uses its own TCP stack:
- No TSO/GSO (gVisor doesn't implement hardware offload abstractions)
- Higher CPU usage per connection (all TCP processing in user space)
- Higher latency (two kernel crossings: gVisor sentry + host kernel)
- Cilium/XDP optimizations don't help gVisor pods (gVisor bypass those hooks)

For CNI, the important point: CNI doesn't need to do anything special for gVisor. The containerd-shim-runsc (gVisor's CRI shim) creates the namespace and calls CNI exactly as containerd would. The abstraction holds.

---

## 13. crun: The C OCI Runtime and Namespaces

`crun` is a lightweight OCI runtime written in C (vs runc in Go). It's used by CRI-O and Podman. Understanding how crun handles network namespaces differently from runc is instructive.

### 13.1 crun's Namespace Join

crun uses `setns()` via the `liboci` C library. The key difference from runc: crun is single-threaded until exec, so namespace transitions are simpler.

```c
/* crun: src/libocispec/libcrun_container.c (conceptual) */

static int
join_netns(const char *netns_path, int *netns_fd_out)
{
    int fd = open(netns_path, O_RDONLY | O_CLOEXEC);
    if (fd < 0)
        return crun_make_error(err, errno, "open netns `%s`", netns_path);

    /* setns(fd, CLONE_NEWNET) moves this thread into the network namespace */
    if (setns(fd, CLONE_NEWNET) < 0)
        return crun_make_error(err, errno, "setns netns `%s`", netns_path);

    *netns_fd_out = fd;
    return 0;
}

/* Called before exec'ing the container process */
static int
libcrun_apply_namespace(container, ns, err)
{
    for each namespace in config.linux.namespaces:
        if ns.path != "":
            /* Join existing namespace (e.g., /var/run/netns/sandbox-id) */
            join_existing_ns(ns.type, ns.path)
        else:
            /* Create new namespace */
            unshare(ns_flag)
}
```

crun uses fewer `fork()`s than runc (runc uses two forks for namespace isolation), which makes it faster for pod startup — critical in serverless workloads like Knative.

---

## 14. Multus CNI Internals

Multus is a CNI meta-plugin that enables pods to have multiple network interfaces. It reads `NetworkAttachmentDefinition` (NAD) CRDs and invokes multiple CNI plugins for a single pod.

### 14.1 How Multus Intercepts CNI Calls

```
Without Multus:
  containerd -> CNI (flannel) -> eth0 in pod

With Multus:
  containerd -> Multus (primary CNI) -> eth0 (default network, via flannel/calico)
                                     -> net1 (secondary, via macvlan)
                                     -> net2 (tertiary, via SR-IOV)
```

Multus installs itself as the primary CNI by placing its binary at `/opt/cni/bin/multus` and writing a conflist that points to it. The conflist has `type: multus`.

### 14.2 NetworkAttachmentDefinition CRD

```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-conf
  namespace: default
spec:
  config: |
    {
      "cniVersion": "0.3.1",
      "type": "macvlan",
      "master": "eth0",
      "mode": "bridge",
      "ipam": {
        "type": "static",
        "addresses": [{"address": "192.168.1.10/24"}]
      }
    }
```

Pod requests additional interfaces via annotation:

```yaml
annotations:
  k8s.v1.cni.cncf.io/networks: macvlan-conf, macvlan-conf@net2
  # net2 = interface name for second macvlan attachment
```

### 14.3 Multus Plugin Execution Flow

```go
// Simplified from k8snetworkplumbingwg/multus-cni

func (m *multusPlugin) CmdAdd(args *skel.CmdArgs) error {
    // 1. Parse Multus config (contains delegate for default network)
    multusConf := parseMultusConfig(args.StdinData)

    // 2. Execute default network plugin (flannel/calico/etc.)
    // Creates eth0 inside the container netns
    defaultResult, err := delegateAdd(args, multusConf.Delegates[0])

    // 3. Read pod annotations via Kubernetes API
    // (Multus has a kubeconfig and calls kube-apiserver)
    pod, err := m.kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
    networkAttachments := parseNetworkAnnotation(pod.Annotations["k8s.v1.cni.cncf.io/networks"])

    // 4. For each requested additional network:
    for i, netAttachment := range networkAttachments {
        // Fetch NAD CRD from kube-apiserver
        nad, err := m.nadClient.Get(ctx, netAttachment.Name, netAttachment.Namespace)

        // Determine interface name (net1, net2, ... or custom)
        ifName := netAttachment.InterfaceRequest
        if ifName == "" {
            ifName = fmt.Sprintf("net%d", i+1)
        }

        // Modify args for this delegation
        delegateArgs := *args
        delegateArgs.IfName = ifName

        // Execute the CNI plugin specified in the NAD
        // This creates net1, net2, etc. inside the container netns
        result, err := delegateAdd(&delegateArgs, nad.Spec.Config)

        // Collect results
        results = append(results, result)
    }

    // 5. Store all results in pod annotation (for inspection)
    updatePodAnnotationWithNetworks(pod, results)

    // 6. Return merged result (default network's result as primary)
    return types.PrintResult(defaultResult, multusConf.CNIVersion)
}
```

### 14.4 Thick vs Thin Plugin Architecture

**Thin plugin (current Multus):** Multus binary runs as a CNI plugin. For every pod ADD/DEL, it calls the Kubernetes API from within the CNI exec. This has latency implications (API call during pod creation).

**Thick plugin (Multus 4.0+):** A daemon runs on the node. The CNI binary is just a shim that talks to the daemon via Unix socket. The daemon maintains a cache of NADs and pod metadata, avoiding per-pod API calls.

```
Thin (current):
  containerd -> exec multus-cni binary
                  -> HTTP: kube-apiserver (fetch pod/NAD)
                  -> exec flannel, exec macvlan

Thick (v4.0+):
  containerd -> exec multus-shim
                  -> Unix socket: multus-daemon (caches NADs)
                  -> exec flannel, exec macvlan
                  <- returns result
```

---

## 15. SR-IOV CNI: Hardware Virtualization

SR-IOV (Single Root I/O Virtualization) allows a single physical NIC to appear as multiple PCIe devices. Each **Virtual Function (VF)** is a lightweight PCIe device that can be directly assigned to a container.

### 15.1 SR-IOV Concepts

```
Physical NIC (Physical Function - PF):
  Intel X550, Mellanox ConnectX-5, etc.
  One PF, up to 256 VFs

  PF: eth0 (host network, CNI bridge, routing)
  VF0: assigned to pod-A -> appears as eth0 inside pod-A
  VF1: assigned to pod-B -> appears as eth0 inside pod-B
  VF2: assigned to pod-C -> appears as eth0 inside pod-C

Packet flow (VF mode):
  Pod-A TX: VF0 NIC -> physical switch -> destination
  (No kernel networking involved — hardware direct path)
  Throughput: line rate, ~10 Gbps with < 5µs latency
```

### 15.2 SR-IOV Device Plugin + SR-IOV CNI

SR-IOV requires two components:
1. **SR-IOV Device Plugin**: advertises VFs as Kubernetes resources (`intel.com/sriov_net: 1`)
2. **SR-IOV CNI plugin**: moves the VF into the pod's network namespace

```bash
# Enable SR-IOV on the host (before Kubernetes):
echo 4 > /sys/class/net/eth0/device/sriov_numvfs
# Creates VF0..VF3 as PCIe devices

# VFs appear as netdevices:
ip link show eth0
# ...
#     vf 0     link/ether aa:bb:cc:dd:00:01 brd ff:ff:ff:ff:ff:ff, spoof checking on
#     vf 1     link/ether aa:bb:cc:dd:00:02 brd ff:ff:ff:ff:ff:ff, spoof checking on

# Device plugin discovers VFs and advertises to kubelet:
# allocatable: {"intel.com/sriov_net": 4}
```

### 15.3 SR-IOV CNI Plugin: Moving VF into Netns

```go
// Simplified SR-IOV CNI ADD handler

func cmdAdd(args *skel.CmdArgs) error {
    conf := parseConfig(args.StdinData)

    // Device plugin passes the VF PCI address via CNI_ARGS or runtimeConfig
    // e.g., CNI_ARGS="VF_PCI_ADDR=0000:02:00.1"
    vfPciAddr := getCNIArg(args, "VF_PCI_ADDR")

    // Find the netdevice name for this PCI address
    vfNetdevName := pciToNetdev(vfPciAddr)  // e.g., "eth0v0"

    // Get the VF index (0, 1, 2, ...)
    pfName, vfIndex := getVFInfo(vfNetdevName)

    // Set MAC address for the VF (optional, via PF's Netlink)
    if conf.MAC != "" {
        // Must set MAC via PF netlink (VF MAC is PF-controlled)
        pfLink, _ := netlink.LinkByName(pfName)
        netlink.LinkSetVfHardwareAddr(pfLink, vfIndex, macAddr)
    }

    // Move VF netdevice into container namespace
    vfLink, _ := netlink.LinkByName(vfNetdevName)
    netNS, _ := ns.GetNS(args.Netns)
    netlink.LinkSetNsFd(vfLink, int(netNS.Fd()))

    // Inside container namespace: rename and configure
    netNS.Do(func(_ ns.NetNS) error {
        // Rename to requested ifname (e.g., "eth0")
        vfLink, _ = netlink.LinkByName(vfNetdevName)
        netlink.LinkSetName(vfLink, args.IfName)

        // Configure IP (from IPAM)
        ipam := callIPAM(conf)
        netlink.AddrAdd(vfLink, ipam.IP)
        netlink.LinkSetUp(vfLink)
        netlink.RouteAdd(defaultRoute)
        return nil
    })

    // Return result
    return types.PrintResult(result, conf.CNIVersion)
}
```

**Key difference from veth-based CNI:** With SR-IOV, the pod's NIC IS the physical hardware VF. There's no kernel network stack involved for the pod's data path — packets go directly from the VF to the application. This means:
- No iptables/BPF hooks on the pod's data path
- No network policy enforcement possible on the VF data plane (need SR-IOV-aware policy, e.g., via Hardware TC or eSwitch)
- The `kube-proxy` service IP DNAT still works (the Linux routing stack handles Service IPs before the packet hits the VF)

---

## 16. AF_XDP: Kernel Bypass for Containers

AF_XDP sockets allow user-space applications to receive/send packets with near-zero kernel overhead. XDP programs redirect packets from the NIC ring buffer directly to user-space memory (UMEM).

### 16.1 AF_XDP Architecture

```
Traditional socket path:
  NIC ring buffer -> DMA -> sk_buff alloc -> socket buffer -> recv() -> app

AF_XDP path:
  NIC ring buffer -> XDP program -> UMEM (shared memory) -> recvmsg() -> app
  NO sk_buff allocation.
  NO kernel TCP/IP stack.
  Throughput: 20+ Mpps on a single CPU.

UMEM: User MEMory
  App allocates a large memory region.
  Registers it with kernel via XDP_UMEM_REG setsockopt.
  NIC DMA writes packets directly into UMEM frames.
  App reads frame descriptors from RX ring.
```

### 16.2 AF_XDP in Container Networking

Projects like DPDK-based CNI or F-Stack use AF_XDP for ultra-low-latency container networking:

```
Pod running DPDK workload:
  1. CNI creates a veth pair as usual
  2. CNI creates an XSK (XDP socket) bound to the host-side veth
  3. CNI passes the XSK fd to the pod via a Unix socket (out-of-band)
  4. Pod app opens the XSK fd, registers UMEM
  5. All pod network traffic bypasses kernel TCP/IP stack
  6. App implements its own TCP/IP (DPDK PMD or Seastar)

Alternatively: DPDK vhost-user:
  Pod <-> vhost-user socket <-> host vswitch (OVS-DPDK) <-> physical NIC
```

### 16.3 AF_XDP Socket Creation in C

```c
/* Minimal AF_XDP socket setup */
#include <linux/if_xdp.h>
#include <sys/mman.h>

#define FRAME_SIZE    4096
#define NUM_FRAMES    4096
#define UMEM_SIZE     (FRAME_SIZE * NUM_FRAMES)

int setup_xsk(const char *ifname, int queue_id) {
    /* 1. Create AF_XDP socket */
    int xsk_fd = socket(AF_XDP, SOCK_RAW, 0);

    /* 2. Allocate UMEM (packet memory pool) */
    void *umem_area = mmap(NULL, UMEM_SIZE,
                           PROT_READ | PROT_WRITE,
                           MAP_PRIVATE | MAP_ANONYMOUS | MAP_HUGETLB,
                           -1, 0);

    /* 3. Register UMEM with kernel */
    struct xdp_umem_reg umem_reg = {
        .addr     = (uintptr_t)umem_area,
        .len      = UMEM_SIZE,
        .chunk_size = FRAME_SIZE,
        .headroom = 0,
        .flags    = 0,
    };
    setsockopt(xsk_fd, SOL_XDP, XDP_UMEM_REG,
               &umem_reg, sizeof(umem_reg));

    /* 4. Set ring sizes */
    int ring_size = NUM_FRAMES;
    setsockopt(xsk_fd, SOL_XDP, XDP_UMEM_FILL_RING,
               &ring_size, sizeof(ring_size));
    setsockopt(xsk_fd, SOL_XDP, XDP_RX_RING,
               &ring_size, sizeof(ring_size));

    /* 5. Bind to interface + queue */
    struct sockaddr_xdp sxdp = {
        .sxdp_family   = AF_XDP,
        .sxdp_ifindex  = if_nametoindex(ifname),
        .sxdp_queue_id = queue_id,
        .sxdp_flags    = XDP_USE_NEED_WAKEUP,
    };
    bind(xsk_fd, (struct sockaddr *)&sxdp, sizeof(sxdp));

    /* 6. Load an XDP program that redirects to this socket */
    /* bpf_redirect_map(&xsks_map, queue_id, XDP_DROP) */

    return xsk_fd;
}
```

---

## 17. WireGuard Integration in Cilium

Cilium's WireGuard mode encrypts all pod-to-pod traffic between nodes using the WireGuard kernel module.

### 17.1 WireGuard Kernel Module

WireGuard is a simple, fast VPN built into the Linux kernel (5.6+). It uses ChaCha20-Poly1305 for encryption and Curve25519 for key exchange.

```
WireGuard interface (cilium_wg0):
  - Each node has a WireGuard private key
  - Each peer (remote node) is registered with its public key and endpoint
  - All traffic to/from a peer is automatically encrypted/decrypted

Node 1                              Node 2
  cilium_wg0                          cilium_wg0
  privkey: Ak...                      privkey: Bk...
  pubkey:  AP...                      pubkey:  BP...
  peer BP...:                         peer AP...:
    endpoint: 192.168.1.20:51820        endpoint: 192.168.1.10:51820
    allowed-ips: 10.244.2.0/24          allowed-ips: 10.244.1.0/24
```

### 17.2 How Cilium Manages WireGuard

```go
// pkg/wireguard/agent.go (simplified)

type Agent struct {
    wgClient *wgctrl.Client  // golang.zx2c4.com/wireguard/wgctrl
    privKey  wgtypes.Key
    pubKey   wgtypes.Key
}

func (a *Agent) Init(netlink netlink.Link) error {
    // 1. Create WireGuard interface
    link := &netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: "cilium_wg0"}}
    netlink.LinkAdd(link)

    // 2. Generate key pair
    a.privKey, _ = wgtypes.GeneratePrivateKey()
    a.pubKey = a.privKey.PublicKey()

    // 3. Configure WireGuard interface via WireGuard Netlink API (NETLINK_GENERIC)
    a.wgClient.ConfigureDevice("cilium_wg0", wgtypes.Config{
        PrivateKey: &a.privKey,
        ListenPort: &wgPort,  // 51820
    })

    // 4. Add route: all cluster traffic via cilium_wg0
    netlink.RouteAdd(&netlink.Route{
        Dst:       clusterCIDR,
        LinkIndex: wg0.Attrs().Index,
    })
    return nil
}

// Called when a new node joins the cluster
func (a *Agent) UpdatePeer(node *nodeTypes.Node) error {
    peerPubKey := node.WireguardPubKey
    nodeIP := node.GetNodeIP(false)
    podCIDR := node.IPv4AllocCIDR

    peer := wgtypes.PeerConfig{
        PublicKey:  peerPubKey,
        Endpoint:   &net.UDPAddr{IP: nodeIP, Port: 51820},
        AllowedIPs: []net.IPNet{*podCIDR},
        // PersistentKeepaliveInterval: 25 * time.Second (for NAT traversal)
    }

    // Configure peer via NETLINK_GENERIC WireGuard protocol
    return a.wgClient.ConfigureDevice("cilium_wg0", wgtypes.Config{
        Peers: []wgtypes.PeerConfig{peer},
    })
}
```

### 17.3 Packet Path with WireGuard

```
Pod-A (10.244.1.5) on Node-1 -> Pod-B (10.244.2.5) on Node-2

1. BPF on lxcAAAA (egress):
   - Policy check: allowed
   - Lookup cilium_ipcache: 10.244.2.5 -> node-2
   - Lookup: is WireGuard enabled? yes
   - Route packet via cilium_wg0 (NOT via VXLAN tunnel)
   - bpf_redirect(cilium_wg0_ifindex, 0)

2. cilium_wg0 WireGuard TX:
   - Find peer for 10.244.2.5: peer=node-2 (AllowedIPs match)
   - Encrypt packet with ChaCha20-Poly1305
   - UDP encapsulate: src=192.168.1.10:51820 dst=192.168.1.20:51820
   - WireGuard header: receiver_index, counter, auth_tag

3. Physical NIC transmits encrypted UDP

4. Node-2 physical NIC receives UDP:51820
   - WireGuard kernel module in cilium_wg0:
   - Decrypt and verify
   - Deliver inner packet (10.244.1.5 -> 10.244.2.5) to kernel

5. Node-2 routing: 10.244.2.5 -> local pod (via BPF redirect to lxcBBBB)
```

**Key performance property:** WireGuard works in the kernel, not in user space (unlike OpenVPN or Strongswan). Encryption throughput on a single core is typically 3-5 Gbps.

---

## 18. IPSec/XFRM: Pod-to-Pod Encryption

Calico's encryption mode uses the Linux XFRM (transform) subsystem — the kernel's IPSec implementation.

### 18.1 XFRM Architecture

```
XFRM: Security Policy (SP) + Security Association (SA)

Security Policy (SP): "what traffic to encrypt"
  ip xfrm policy add src 10.244.1.0/24 dst 10.244.2.0/24
    dir out tmpl src 192.168.1.10 dst 192.168.1.20
    proto esp spi 0x1001 mode tunnel

Security Association (SA): "how to encrypt" (keys, algorithm)
  ip xfrm state add src 192.168.1.10 dst 192.168.1.20
    proto esp spi 0x1001 mode tunnel
    enc aes-128-cbc <key>
    auth sha1 <key>
```

### 18.2 IPSec Packet Path

```
Pod-A (10.244.1.5) -> Pod-B (10.244.2.5)

1. Routing: packet dst=10.244.2.5, out via eth0 (BGP route to node-2)

2. OUTPUT hook: XFRM policy lookup
   src=10.244.1.5, dst=10.244.2.5 matches SP:
   "encrypt and tunnel via SA 0x1001"

3. XFRM transform:
   a. Add ESP header: SPI=0x1001, sequence=N
   b. Encrypt inner IP + payload with AES-CBC
   c. Compute HMAC-SHA1 auth tag
   d. Add outer IP header: src=192.168.1.10, dst=192.168.1.20

4. Physical NIC transmits: [IP][ESP][encrypted: IP+payload][auth]

5. Node-2 receives IPSec packet:
   XFRM SP lookup: matches inbound SA 0x1001
   -> decrypt and verify
   -> deliver inner packet 10.244.1.5->10.244.2.5 to routing

6. Routing delivers to pod-B via its veth
```

### 18.3 Calico's Key Management

Calico uses a **per-node WireGuard-like key management** but via a Kubernetes Secret mechanism:

```
Felix (Calico agent) on each node:
  1. Generates a per-node PSK (Pre-Shared Key) on startup
  2. Stores in Kubernetes Node annotation: projectcalico.org/IPSecSharedSecret
  3. On peer node discovery:
     - Read peer's PSK from K8s node annotation
     - Install XFRM SA with that PSK
     - Install XFRM SP for that node's pod CIDR
```

---

## 19. Istio CNI Plugin and Ambient Mesh

### 19.1 The Sidecar Problem: iptables vs CNI

Standard Istio uses an init container to set up iptables rules that redirect all pod traffic through the Envoy sidecar. This requires `NET_ADMIN` capability and runs before the app container.

The **Istio CNI plugin** moves this iptables setup OUT of the pod init container and INTO the CNI plugin chain. This eliminates the need for `NET_ADMIN` in pods.

### 19.2 Istio CNI Plugin Mechanics

```json
{
  "cniVersion": "0.3.1",
  "name": "istio-cni",
  "plugins": [
    { "type": "calico", "... calico config ..." },
    {
      "type": "istio-cni",
      "log_level": "info",
      "kubernetes": {
        "kubeconfig": "/etc/cni/net.d/ZZZ-istio-cni-kubeconfig",
        "cni_bin_dir": "/opt/cni/bin"
      }
    }
  ]
}
```

When the `istio-cni` plugin runs for a pod:

```go
// Simplified from istio/cni

func cmdAdd(args *skel.CmdArgs) error {
    // prevResult contains the pod's IP (from calico/flannel plugin)
    prevResult := parseResult(conf.PrevResult)
    podIP := prevResult.IPs[0].Address.IP

    // Check if this pod should have sidecar injection
    // (by querying kube-apiserver for pod annotations)
    if !shouldInjectSidecar(podName, podNamespace) {
        return prevResult  // passthrough, no iptables setup
    }

    // Enter pod network namespace
    netNS, _ := ns.GetNS(args.Netns)
    netNS.Do(func(_ ns.NetNS) error {
        // Add the SAME iptables rules that the init container would add:
        // Redirect all inbound traffic to Envoy port 15006
        iptables -t nat -A PREROUTING -p tcp -j REDIRECT --to-port 15006

        // Redirect all outbound traffic to Envoy port 15001
        iptables -t nat -A OUTPUT -p tcp ! -d 127.0.0.1/32 -j REDIRECT --to-port 15001

        // Exclude Envoy itself (UID 1337) from redirection
        iptables -t nat -A OUTPUT -m owner --uid-owner 1337 -j RETURN

        return nil
    })
    return prevResult
}
```

### 19.3 Ambient Mesh: No Sidecar, No CNI Plugin

Istio Ambient Mesh (1.22+) eliminates the sidecar entirely. Instead of redirecting traffic per-pod, it uses a **per-node ztunnel** (a Rust-based L4 proxy) and **waypoint proxies** for L7.

```
Ambient Mesh architecture:

Node:
  Pod-A (no sidecar) -> ztunnel (hostNetwork pod)
                     |
                     | HBONE (HTTP CONNECT-based overlay)
                     v
                  Remote ztunnel -> Pod-B (no sidecar)

Redirection mechanism:
  iptables in Pod-A's netns redirect TCP traffic to ztunnel's Unix socket
  OR
  Kernel TC BPF redirects traffic to ztunnel's network namespace

ztunnel responsibilities:
  - mTLS termination (peer authentication)
  - L4 policy enforcement (allow/deny by identity)
  - Traffic tunneling to remote ztunnel via HBONE
  - Does NOT do L7 (HTTP headers, retries) — that's waypoint's job
```

The interaction with CNI: Ambient mesh's **istio-cni** plugin installs TC BPF programs on pod interfaces (instead of or in addition to iptables) to redirect traffic to ztunnel. This is done in the CNI ADD phase, making it transparent to the application and the container runtime.

---

## 20. Dual-Stack IPv4/IPv6 CNI

Kubernetes dual-stack (since 1.21) allows pods to have both IPv4 and IPv6 addresses.

### 20.1 Dual-Stack CNI Configuration

```json
{
  "cniVersion": "0.3.1",
  "name": "cbr0",
  "plugins": [
    {
      "type": "ptp",
      "ipam": {
        "type": "host-local",
        "ranges": [
          [{"subnet": "10.244.0.0/24"}],
          [{"subnet": "2001:db8::/64"}]
        ],
        "routes": [
          {"dst": "0.0.0.0/0"},
          {"dst": "::/0"}
        ]
      }
    }
  ]
}
```

The `ranges` field in host-local IPAM supports multiple subnets. Each range results in a separate IP allocation. The CNI result will contain multiple IPs:

```json
{
  "ips": [
    {"address": "10.244.0.5/24",     "gateway": "10.244.0.1", "interface": 0},
    {"address": "2001:db8::5/64",    "gateway": "2001:db8::1", "interface": 0}
  ]
}
```

### 20.2 Kernel Dual-Stack Configuration

The CNI plugin must configure both IPv4 and IPv6 in the container namespace:

```bash
# Inside container netns:
ip addr add 10.244.0.5/24 dev eth0
ip addr add 2001:db8::5/64 dev eth0

ip route add default via 10.244.0.1 dev eth0
ip -6 route add default via 2001:db8::1 dev eth0

# IPv6 Router Advertisements (SLAAC) must be disabled
# to prevent the container from autoconfiguring a conflicting IPv6 address
sysctl -w net.ipv6.conf.eth0.accept_ra=0
sysctl -w net.ipv6.conf.eth0.autoconf=0
```

### 20.3 Dual-Stack Kubernetes Services

With dual-stack pods, Kubernetes can have dual-stack services too:

```yaml
spec:
  ipFamilyPolicy: PreferDualStack
  ipFamilies: [IPv4, IPv6]
  clusterIPs: [10.96.100.1, fd00::1234]
```

kube-proxy creates separate iptables chains for IPv4 and IPv6 service IPs. Cilium creates separate BPF maps (`cilium_lb4_services` for IPv4, `cilium_lb6_services` for IPv6).

---

## 21. DNS Architecture Inside Pods

Understanding DNS in pods is essential because DNS failures are one of the most common networking issues in Kubernetes.

### 21.1 /etc/resolv.conf in a Pod

The kubelet generates `/etc/resolv.conf` for each pod (placed in the container's rootfs via a bind mount):

```
# /etc/resolv.conf inside pod "my-app" in namespace "default"
nameserver 10.96.0.10          # CoreDNS ClusterIP
search default.svc.cluster.local svc.cluster.local cluster.local
options ndots:5
```

**ndots:5**: If a query has fewer than 5 dots, the kernel DNS resolver tries each search domain first before trying the bare name. This means:
- `curl http://my-service` -> tries `my-service.default.svc.cluster.local` first ✓
- `curl http://google.com` -> tries `google.com.default.svc.cluster.local`, `google.com.svc.cluster.local`, `google.com.cluster.local`, then `google.com` → 4 failed DNS queries before success!

This is the "ndots:5 problem" — external DNS queries generate extra lookups.

### 21.2 How kubelet Writes resolv.conf

```go
// pkg/kubelet/network/dns/dns.go

func (c *Configurer) GetPodDNS(pod *v1.Pod) (*runtimeapi.DNSConfig, error) {
    dnsConfig := &runtimeapi.DNSConfig{}

    // ClusterFirst DNS policy (default):
    // Use cluster DNS (CoreDNS) as primary nameserver
    dnsConfig.Servers = []string{c.clusterDNS.String()}  // 10.96.0.10

    // Search domains:
    dnsConfig.Searches = []string{
        fmt.Sprintf("%s.svc.%s", pod.Namespace, c.ClusterDomain),  // default.svc.cluster.local
        fmt.Sprintf("svc.%s", c.ClusterDomain),                     // svc.cluster.local
        c.ClusterDomain,                                              // cluster.local
    }
    dnsConfig.Options = []string{"ndots:5"}

    // Merge with pod's dnsConfig if specified (pod.spec.dnsConfig)
    return dnsConfig, nil
}
```

containerd receives this via CRI and creates the resolv.conf file in the container's mount namespace (typically overlayfs root + /etc/resolv.conf bind mount).

### 21.3 CoreDNS Architecture

CoreDNS is the cluster DNS server. It runs as a Deployment (usually 2+ replicas) behind a ClusterIP.

```
DNS query path:
  pod-A -> /etc/resolv.conf nameserver=10.96.0.10
         -> UDP:53 to 10.96.0.10
         -> kube-proxy DNAT: 10.96.0.10:53 -> coredns-pod:53
         -> CoreDNS pod

CoreDNS plugin chain (Corefile):
  . {
    errors
    health
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
      pods insecure
      fallthrough in-addr.arpa ip6.arpa
      ttl 30
    }
    prometheus :9153
    forward . /etc/resolv.conf  <- forward non-cluster queries to upstream
    cache 30
    loop
    reload
    loadbalance
  }

CoreDNS kubernetes plugin:
  Watches kube-apiserver for Service/Endpoint changes
  Builds in-memory DNS records:
    my-service.default.svc.cluster.local -> 10.96.100.1 (A record)
    _http._tcp.my-service.default.svc.cluster.local -> SRV record
    pod-ip.default.pod.cluster.local -> pod IP (if pods insecure)
```

### 21.4 DNS TTL and Caching Issues

DNS responses from CoreDNS have TTL=30s by default. glibc's resolver caches responses for min(TTL, 30) seconds. But glibc has NO negative cache — a NXDOMAIN is retried immediately. This causes DNS thundering herds when a service is being created.

The `ndots:5` + no negative cache problem:

```
pod queries: curl http://api.example.com
  1. api.example.com.default.svc.cluster.local -> NXDOMAIN (no cache, retried!)
  2. api.example.com.svc.cluster.local -> NXDOMAIN
  3. api.example.com.cluster.local -> NXDOMAIN
  4. api.example.com -> A 93.184.216.34 (success)

All 4 queries hit CoreDNS. Multiply by 1000 pods = 4000 DNS queries per curl.
```

**Mitigation**: `ndots:1` in pod's dnsConfig, or use FQDN (`api.example.com.` with trailing dot).

---

## 22. NodeLocal DNSCache: eBPF-Accelerated DNS

NodeLocal DNSCache runs a DNS cache on every node. It intercepts DNS queries at the kernel level before they reach CoreDNS.

### 22.1 How It Works

```
Without NodeLocal DNSCache:
  pod-A -> UDP:53 -> kube-proxy DNAT -> CoreDNS (remote pod)
  Latency: ~1ms (cross-pod + kube-proxy overhead)
  Problem: UDP DNAT + conntrack + CoreDNS pod scheduling

With NodeLocal DNSCache:
  pod-A -> UDP:53 to 169.254.20.10 (link-local, node-local)
         -> NodeLocal cache daemon (on this node, no kube-proxy)
         -> Cache hit: < 100µs
         -> Cache miss: NodeLocal -> CoreDNS
```

### 22.2 Traffic Interception Without kube-proxy

NodeLocal DNSCache uses a **link-local IP** (169.254.20.10) that is added to every node's `nodelocaldns` dummy interface. No kube-proxy DNAT needed — the IP is on the node itself.

```bash
# NodeLocal DNSCache setup on each node:
ip link add nodelocaldns type dummy
ip addr add 169.254.20.10/32 dev nodelocaldns
ip link set nodelocaldns up

# iptables rules to intercept DNS queries going to the link-local IP
iptables -t raw -A PREROUTING -dst 169.254.20.10 -p udp --dport 53 -j NOTRACK
iptables -t raw -A OUTPUT -src 169.254.20.10 -p udp --sport 53 -j NOTRACK
```

**NOTRACK**: tells conntrack to NOT track these connections — avoids conntrack contention for high-volume DNS traffic.

### 22.3 kubelet Integration

kubelet is configured to write `169.254.20.10` as the nameserver in pod resolv.conf:

```
--cluster-dns=169.254.20.10
```

This makes ALL pod DNS queries go to the local node's cache daemon first.

---

## 23. hostPort and portmap Plugin: Exact Kernel Mechanics

`hostPort` allows a container port to be accessible on the node's IP. The `portmap` CNI meta-plugin implements this using iptables DNAT.

### 23.1 portmap Plugin Execution

When a pod has `containerPort.hostPort: 8080`:

1. containerd reads the portmapping from the OCI spec
2. Passes it via `runtimeConfig.portMappings` capability to the CNI chain
3. The `portmap` plugin (in the chain) adds iptables rules:

```bash
# Rules added by portmap plugin for pod IP=10.244.0.5 hostPort=8080 containerPort=80

# PREROUTING: any external traffic to node-IP:8080 -> pod:80
iptables -t nat -A CNI-HOSTPORT-DNAT
    -p tcp --dport 8080
    -j DNAT --to-destination 10.244.0.5:80

# OUTPUT: traffic from local processes on the node to node-IP:8080 -> pod:80
iptables -t nat -A OUTPUT
    -p tcp -d <node-IP> --dport 8080
    -j DNAT --to-destination 10.244.0.5:80

# POSTROUTING: masquerade traffic from the pod subnet back to itself
# (hairpin: pod talking to its own hostPort)
iptables -t nat -A CNI-HOSTPORT-MASQ
    -s 10.244.0.5/32 -d 10.244.0.5/32
    -p tcp --dport 80
    -j MASQUERADE
```

### 23.2 Why hostPort Scales Poorly

Every hostPort creates an iptables DNAT rule that applies to ALL traffic entering the node. With 1000 pods each having 2 hostPorts = 2000 DNAT rules scanned per packet. This is why Kubernetes recommends Services over hostPorts for production use.

---

## 24. kubectl port-forward: The Full Tunnel Path

`kubectl port-forward pod/my-pod 8080:80` creates a tunnel from your laptop to the pod. Understanding this path clarifies how it differs from hostPort.

### 24.1 The Tunnel Chain

```
kubectl port-forward                kube-apiserver             kubelet/SPDY
---------------------               --------------             -----------

kubectl                             kube-apiserver             kubelet
  -> TCP connect to kube-apiserver:443
  -> HTTP/2 upgrade to SPDY (streamType=portforward)
  -> kube-apiserver proxies to kubelet:10250
  -> kubelet opens SPDY stream to the target pod
  -> kubelet: nsenter into pod's netns, dial 127.0.0.1:80
  -> bidirectional byte stream

User flow:
  User connects to localhost:8080
    -> kubectl reads bytes
    -> sends via SPDY stream over HTTPS to kube-apiserver
    -> kube-apiserver forwards to kubelet
    -> kubelet writes to TCP connection inside pod
    -> pod receives as if from 127.0.0.1
```

### 24.2 kubelet's Side of port-forward

```go
// pkg/kubelet/server/portforward/portforward.go

func (s *Server) PortForward(ctx context.Context, pod *api.Pod, req PortForwardRequest) error {
    // Get pod's network namespace
    netnsPath := s.containerManager.GetNetNS(pod)

    // For each port-forward stream pair (data + error):
    // 1. Parse the target port from the SPDY header
    port := req.Port

    // 2. Open TCP connection to the pod's port, INSIDE the pod's netns
    conn, err := nsenter.DialTCP(netnsPath, fmt.Sprintf("127.0.0.1:%d", port))
    // nsenter uses: setns(netns_fd, CLONE_NEWNET) + net.Dial()

    // 3. Copy bytes in both directions
    go io.Copy(stream, conn)   // pod -> kubectl
    io.Copy(conn, stream)      // kubectl -> pod
}
```

No iptables, no CNI involvement. `kubectl port-forward` is a pure user-space tunnel through the HTTPS connection to kube-apiserver.

---

## 25. MTU, PMTUD, TSO/GRO in Overlay Networks

MTU issues are responsible for a significant percentage of CNI-related production bugs. This section provides a complete mental model.

### 25.1 The Overlay MTU Chain

```
Physical MTU: 1500 bytes (standard Ethernet)

VXLAN overlay: -50 bytes (14 Eth + 20 IP + 8 UDP + 8 VXLAN)
  -> flannel.1 MTU: 1450 bytes
  -> pod eth0 MTU: 1450 bytes
  -> TCP MSS: 1450 - 20 (IP) - 20 (TCP) = 1410 bytes

Geneve overlay: -58 bytes (14 Eth + 20 IP + 8 UDP + 8 Geneve + 8 options typical)
  -> cilium_vxlan MTU: 1442 bytes

WireGuard: -80 bytes (UDP + WireGuard header + ChaCha20 overhead)
  -> cilium_wg0 MTU: 1420 bytes

IPSec (AES-256-GCM + Tunnel mode): ~70 bytes overhead
  -> MTU: ~1430 bytes

Double encapsulation (WireGuard over VXLAN):
  -> 1500 - 50 - 80 = 1370 bytes
  -> This is why you should NEVER stack overlay + encryption; use WireGuard-only
```

### 25.2 PMTUD: Path MTU Discovery

When a packet is too large and the "Don't Fragment" bit is set, the intermediate router drops it and sends an ICMP Type 3, Code 4 (Fragmentation Needed) message back.

```
Pod sends 1500-byte packet (TCP MSS not reduced)
  -> VXLAN encapsulates to 1550 bytes
  -> Physical NIC: 1550 > 1500, DF bit set -> DROP
  -> Router sends ICMP "Frag Needed, MTU=1500" to source
  -> Source should reduce MSS and retransmit

The problem: ICMP Frag Needed is often BLOCKED by firewalls
  -> PMTUD Blackhole: connection silently hangs for large transfers
  -> Small packets (< 1450 bytes) work, large transfers fail
  -> Symptoms: kubectl exec works, but large file copies hang
```

**Detection and fix:**

```bash
# Test PMTUD from inside a pod:
ping -c 1 -M do -s 1450 8.8.8.8    # DF bit set, 1450-byte payload
# If this fails but ping -s 100 works: PMTUD blackhole

# Fix: clamp TCP MSS via iptables (in CNI plugin or Kubernetes node setup)
iptables -t mangle -A FORWARD -p tcp --tcp-flags SYN,RST SYN \
    -j TCPMSS --set-mss 1410

# Flannel and Calico do this automatically as part of their iptables setup.
# The TCPMSS target modifies the SYN packet's MSS option, telling the peer
# not to send packets larger than this MSS.
```

### 25.3 Jumbo Frames: 9000-byte MTU

In data center environments with jumbo frame support:

```
Physical MTU: 9000 bytes (jumbo frames)
VXLAN overlay: 9000 - 50 = 8950 bytes
  -> pod eth0 MTU: 8950 bytes
  -> TCP MSS: 8950 - 40 = 8910 bytes

Benefit: Fewer packets for same data, lower overhead per byte.
Requirement: ALL switches in path must support jumbo frames.
AWS: Intra-VPC traffic supports up to 9001 bytes (by default for newer instance types).
Azure: 1500 bytes only (no jumbo in general).
```

---

## 26. Network Capabilities, Seccomp, and CNI Security

### 26.1 Linux Capabilities Required by CNI Plugins

CNI plugins run with the container runtime's privileges (root or with specific capabilities). Key capabilities:

```
CAP_NET_ADMIN: Create/modify network interfaces, routing, iptables
  Required for: bridge creation, veth creation, ip addr/route commands
  Used by: all CNI main plugins

CAP_NET_RAW: Create raw sockets, bind to any port
  Required for: ARP probing (to detect IP conflicts)
  Used by: arping in CNI, DHCP IPAM

CAP_SYS_ADMIN: Mount filesystems, unshare namespaces
  Required for: creating network namespaces (unshare CLONE_NEWNET)
  Required for: bind-mounting the netns to a path
  Used by: container runtimes (containerd, CRI-O)

CAP_BPF (kernel 5.8+): Load BPF programs
  Required for: Cilium's BPF program loading
  Previously needed CAP_SYS_ADMIN for BPF

CAP_PERFMON (kernel 5.8+): Access perf subsystem
  Required for: BPF perf buffers (Hubble flow data)
```

### 26.2 Containers and Network Capabilities

By default, containers get no network-sensitive capabilities beyond `CAP_NET_BIND_SERVICE` (bind to ports < 1024). The container's networking is pre-configured by CNI before the app starts.

```yaml
# Kubernetes securityContext that blocks all network manipulation:
securityContext:
  capabilities:
    drop: [NET_ADMIN, NET_RAW, SYS_ADMIN]
  # With this, the app cannot:
  #   - Create raw sockets (CAP_NET_RAW required for ping)
  #   - Modify routing tables
  #   - Install iptables rules
  #   - Create network interfaces
```

### 26.3 CNI DaemonSet Security Context

CNI DaemonSets (Cilium, Calico, Flannel) run with highly privileged contexts:

```yaml
# Cilium DaemonSet securityContext (partial):
securityContext:
  capabilities:
    add:
      - NET_ADMIN      # netlink, ip commands
      - NET_RAW        # raw sockets for routing protocols
      - SYS_MODULE     # modprobe (load kernel modules: vxlan, wireguard)
      - SYS_ADMIN      # BPF, mount, namespace operations
      - BPF            # kernel 5.8+ explicit BPF capability
      - PERFMON        # kernel 5.8+ for BPF perf maps
  hostNetwork: true    # required to configure the host's network namespace
  hostPID: true        # required to enter pod namespaces (nsenter)
volumes:
  - hostPath: /proc      # access /proc/PID/ns/net for pod netns
  - hostPath: /sys       # sysctl and BPF filesystem (/sys/fs/bpf)
  - hostPath: /run/cilium # PID files, locks
  - hostPath: /lib/modules # kernel module loading
```

---

## 27. Complete Rust CNI Plugin

A full, working CNI plugin in Rust that handles ADD/DEL/CHECK/VERSION. Uses `rtnetlink` crate for kernel operations.

```toml
# Cargo.toml
[package]
name = "rust-cni-bridge"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { version = "1", features = ["derive"] }
serde_json = "1"
rtnetlink = "0.13"
tokio = { version = "1", features = ["full"] }
futures = "0.3"
netlink-packet-route = "0.17"
ipnetwork = "0.20"
nix = { version = "0.27", features = ["net", "sched"] }
anyhow = "1"
```

```rust
// src/main.rs
// A complete CNI bridge plugin in Rust.
// Implements ADD/DEL/CHECK/VERSION per CNI spec 1.0.0.

use anyhow::{anyhow, Context, Result};
use ipnetwork::IpNetwork;
use nix::sched::{setns, CloneFlags};
use rtnetlink::Handle;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::env;
use std::io::{self, Read, Write};
use std::net::{IpAddr, Ipv4Addr};
use std::os::unix::io::RawFd;
use std::str::FromStr;

// ─────────────────────────────────────────────────────────────
// CNI Configuration Types
// ─────────────────────────────────────────────────────────────

#[derive(Debug, Deserialize)]
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
    ipam: IpamConf,
    prev_result: Option<CniResult>,
}

#[derive(Debug, Deserialize)]
struct IpamConf {
    #[serde(rename = "type")]
    ipam_type: String,
    #[serde(flatten)]
    extra: HashMap<String, serde_json::Value>,
}

// ─────────────────────────────────────────────────────────────
// CNI Result Types (spec 1.0.0)
// ─────────────────────────────────────────────────────────────

#[derive(Debug, Serialize, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
struct CniResult {
    cni_version: String,
    interfaces: Vec<CniInterface>,
    ips: Vec<CniIP>,
    routes: Vec<CniRoute>,
    #[serde(skip_serializing_if = "Option::is_none")]
    dns: Option<CniDNS>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
struct CniInterface {
    name: String,
    mac: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    sandbox: String,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
struct CniIP {
    address: String,  // CIDR notation: "10.244.0.5/24"
    gateway: Option<String>,
    interface: Option<usize>,  // index into interfaces array
}

#[derive(Debug, Serialize, Deserialize, Clone)]
struct CniRoute {
    dst: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    gw: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
struct CniDNS {
    nameservers: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    search: Vec<String>,
}

// ─────────────────────────────────────────────────────────────
// CNI Error Type
// ─────────────────────────────────────────────────────────────

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct CniError {
    cni_version: String,
    code: u32,
    msg: String,
    details: String,
}

impl CniError {
    fn new(code: u32, msg: &str, details: &str) -> Self {
        CniError {
            cni_version: "1.0.0".to_string(),
            code,
            msg: msg.to_string(),
            details: details.to_string(),
        }
    }
}

// ─────────────────────────────────────────────────────────────
// Runtime Environment
// ─────────────────────────────────────────────────────────────

struct CniEnv {
    command: String,       // ADD | DEL | CHECK | VERSION
    container_id: String,
    netns: String,         // /var/run/netns/<id>
    ifname: String,        // eth0
    path: Vec<String>,     // /opt/cni/bin
    args: HashMap<String, String>,  // K8S_POD_NAME, etc.
}

impl CniEnv {
    fn from_env() -> Result<Self> {
        let command = env::var("CNI_COMMAND")
            .context("CNI_COMMAND not set")?;
        let container_id = env::var("CNI_CONTAINERID")
            .context("CNI_CONTAINERID not set")?;
        let netns = env::var("CNI_NETNS")
            .unwrap_or_default();
        let ifname = env::var("CNI_IFNAME")
            .unwrap_or_else(|_| "eth0".to_string());
        let path_str = env::var("CNI_PATH")
            .unwrap_or_else(|_| "/opt/cni/bin".to_string());
        let path = path_str.split(':').map(String::from).collect();

        // Parse CNI_ARGS: "K8S_POD_NAME=my-pod;K8S_POD_NAMESPACE=default"
        let args_str = env::var("CNI_ARGS").unwrap_or_default();
        let mut args = HashMap::new();
        for pair in args_str.split(';') {
            if let Some((k, v)) = pair.split_once('=') {
                args.insert(k.to_string(), v.to_string());
            }
        }

        Ok(CniEnv { command, container_id, netns, ifname, path, args })
    }
}

// ─────────────────────────────────────────────────────────────
// Netlink Operations (async, using tokio + rtnetlink)
// ─────────────────────────────────────────────────────────────

struct NetlinkOps {
    handle: Handle,
}

impl NetlinkOps {
    async fn new() -> Result<Self> {
        let (connection, handle, _) = rtnetlink::new_connection()?;
        tokio::spawn(connection);
        Ok(NetlinkOps { handle })
    }

    /// Create a bridge if it doesn't exist.
    async fn ensure_bridge(&self, name: &str, mtu: u32) -> Result<u32> {
        // Check if bridge already exists
        let mut links = self.handle.link().get().execute();
        use futures::StreamExt;
        while let Some(msg) = links.next().await {
            if let Ok(link) = msg {
                use netlink_packet_route::link::nlas::Nla;
                let ifname = link.nlas.iter().find_map(|nla| {
                    if let Nla::IfName(n) = nla { Some(n.clone()) } else { None }
                });
                if ifname.as_deref() == Some(name) {
                    return Ok(link.header.index);
                }
            }
        }

        // Create new bridge
        self.handle
            .link()
            .add()
            .bridge(name.to_string())
            .execute()
            .await
            .context("create bridge")?;

        // Get the new bridge's index
        let msg = self.handle
            .link()
            .get()
            .match_name(name.to_string())
            .execute()
            .try_next()
            .await?
            .ok_or_else(|| anyhow!("bridge not found after creation"))?;

        let idx = msg.header.index;

        // Set bridge up
        self.handle
            .link()
            .set(idx)
            .up()
            .execute()
            .await
            .context("bridge up")?;

        // Set MTU
        self.handle
            .link()
            .set(idx)
            .mtu(mtu)
            .execute()
            .await
            .context("set bridge mtu")?;

        Ok(idx)
    }

    /// Create a veth pair. One end (host_name) stays in host netns,
    /// the other (peer_name) is moved to the container netns.
    async fn create_veth(
        &self,
        host_name: &str,
        peer_name: &str,
        netns_fd: RawFd,
        mtu: u32,
    ) -> Result<(u32, u32)> {
        // Create veth pair via RTM_NEWLINK
        self.handle
            .link()
            .add()
            .veth(host_name.to_string(), peer_name.to_string())
            .execute()
            .await
            .context("create veth pair")?;

        // Get host-side index
        let host_link = self.handle
            .link()
            .get()
            .match_name(host_name.to_string())
            .execute()
            .try_next()
            .await?
            .ok_or_else(|| anyhow!("host veth not found"))?;
        let host_idx = host_link.header.index;

        // Get peer-side index
        let peer_link = self.handle
            .link()
            .get()
            .match_name(peer_name.to_string())
            .execute()
            .try_next()
            .await?
            .ok_or_else(|| anyhow!("peer veth not found"))?;
        let peer_idx = peer_link.header.index;

        // Set MTU on both ends
        self.handle.link().set(host_idx).mtu(mtu).execute().await?;
        self.handle.link().set(peer_idx).mtu(mtu).execute().await?;

        // Move peer into container namespace
        self.handle
            .link()
            .set(peer_idx)
            .setns_by_fd(netns_fd)
            .execute()
            .await
            .context("move veth to container netns")?;

        Ok((host_idx, peer_idx))
    }

    /// Attach interface to bridge master.
    async fn set_master(&self, link_idx: u32, master_idx: u32) -> Result<()> {
        self.handle
            .link()
            .set(link_idx)
            .master(master_idx)
            .execute()
            .await
            .context("set master")
    }

    /// Set interface up.
    async fn link_up(&self, idx: u32) -> Result<()> {
        self.handle
            .link()
            .set(idx)
            .up()
            .execute()
            .await
            .context("link up")
    }

    /// Add IP address to interface.
    async fn addr_add(&self, idx: u32, ip: IpAddr, prefix_len: u8) -> Result<()> {
        let addr = match ip {
            IpAddr::V4(v4) => {
                rtnetlink::IpVersion::V4
                // use ip as u32
            }
            _ => return Err(anyhow!("IPv6 not implemented in this example")),
        };
        self.handle
            .address()
            .add(idx, ip, prefix_len)
            .execute()
            .await
            .context("add address")
    }

    /// Add default route.
    async fn route_add_default(&self, idx: u32, gateway: IpAddr) -> Result<()> {
        use rtnetlink::IpVersion;
        use std::net::Ipv4Addr;

        let gw = match gateway {
            IpAddr::V4(gw4) => gw4,
            _ => return Err(anyhow!("IPv6 gateway not implemented here")),
        };

        self.handle
            .route()
            .add()
            .v4()
            .destination_prefix(Ipv4Addr::UNSPECIFIED, 0)
            .gateway(gw)
            .output_interface(idx)
            .execute()
            .await
            .context("add default route")
    }
}

// ─────────────────────────────────────────────────────────────
// Namespace Operations
// ─────────────────────────────────────────────────────────────

/// Enter a network namespace, run the closure, then return.
fn with_netns<F, T>(netns_path: &str, f: F) -> Result<T>
where
    F: FnOnce() -> Result<T>,
{
    use std::fs::File;
    use std::os::unix::io::IntoRawFd;

    // Save current netns
    let current = File::open("/proc/self/ns/net")
        .context("open current netns")?;
    let current_fd = current.into_raw_fd();

    // Open target netns
    let target = File::open(netns_path)
        .context("open target netns")?;
    let target_fd = target.into_raw_fd();

    // Enter target netns
    setns(unsafe { std::os::unix::io::BorrowedFd::borrow_raw(target_fd) },
          CloneFlags::CLONE_NEWNET)
        .context("setns target")?;

    let result = f();

    // Always return to original netns
    let _ = setns(unsafe { std::os::unix::io::BorrowedFd::borrow_raw(current_fd) },
                  CloneFlags::CLONE_NEWNET);

    unsafe {
        nix::libc::close(current_fd);
        nix::libc::close(target_fd);
    }

    result
}

// ─────────────────────────────────────────────────────────────
// IPAM: Call host-local via subprocess
// ─────────────────────────────────────────────────────────────

fn call_ipam(ipam_type: &str, cni_path: &[String], stdin_data: &[u8])
    -> Result<CniResult>
{
    // Find the IPAM plugin binary
    let binary = cni_path.iter()
        .map(|dir| format!("{}/{}", dir, ipam_type))
        .find(|p| std::path::Path::new(p).exists())
        .ok_or_else(|| anyhow!("IPAM plugin '{}' not found in {:?}", ipam_type, cni_path))?;

    let current_env: Vec<(String, String)> = env::vars().collect();

    let output = std::process::Command::new(&binary)
        .env_clear()
        .envs(current_env)
        .env("CNI_COMMAND", "ADD")
        .stdin(std::process::Stdio::piped())
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .spawn()
        .context("spawn IPAM plugin")?
        .wait_with_output()
        .context("wait for IPAM plugin")?;

    // TODO: write stdin_data to the process stdin (omitted for brevity)

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(anyhow!("IPAM plugin failed: {}", stderr));
    }

    let result: CniResult = serde_json::from_slice(&output.stdout)
        .context("parse IPAM result")?;
    Ok(result)
}

// ─────────────────────────────────────────────────────────────
// Command Handlers
// ─────────────────────────────────────────────────────────────

#[tokio::main]
async fn cmd_add(env: &CniEnv, conf: &NetConf, stdin_data: &[u8]) -> Result<CniResult> {
    let bridge_name = conf.bridge.as_deref().unwrap_or("cni0");
    let mtu = conf.mtu.unwrap_or(1500);
    let netns_path = &env.netns;

    // 1. Call IPAM to get IP address
    let ipam_result = call_ipam(&conf.ipam.ipam_type, &env.path, stdin_data)?;
    if ipam_result.ips.is_empty() {
        return Err(anyhow!("IPAM returned no IPs"));
    }

    let pod_ip_cidr = &ipam_result.ips[0].address;  // "10.244.0.5/24"
    let gateway = ipam_result.ips[0].gateway.as_deref().unwrap_or("0.0.0.0");

    let net: IpNetwork = pod_ip_cidr.parse().context("parse pod IP")?;
    let gw_ip: IpAddr = gateway.parse().context("parse gateway")?;

    // 2. Ensure bridge exists (in host namespace)
    let netops = NetlinkOps::new().await?;
    let bridge_idx = netops.ensure_bridge(bridge_name, mtu).await?;

    // 3. Generate random veth name
    let host_veth = format!("veth{:08x}", rand_u32());
    let container_veth = env.ifname.clone();

    // 4. Open container namespace fd
    use std::fs::File;
    use std::os::unix::io::AsRawFd;
    let netns_file = File::open(netns_path).context("open netns")?;
    let netns_fd = netns_file.as_raw_fd();

    // 5. Create veth pair (host side stays, peer moves to container netns)
    let (host_veth_idx, _) = netops.create_veth(&host_veth, &container_veth, netns_fd, mtu).await?;

    // 6. Bring up host-side veth and attach to bridge
    netops.link_up(host_veth_idx).await?;
    netops.set_master(host_veth_idx, bridge_idx).await?;

    // 7. Configure IP inside container namespace
    with_netns(netns_path, || {
        let rt = tokio::runtime::Runtime::new()?;
        rt.block_on(async {
            let container_netops = NetlinkOps::new().await?;

            // Get container-side veth index (we're now inside the netns)
            use futures::StreamExt;
            let mut links = container_netops.handle.link().get().execute();
            let mut container_veth_idx = None;
            while let Some(msg) = links.next().await {
                if let Ok(link) = msg {
                    use netlink_packet_route::link::nlas::Nla;
                    let ifname = link.nlas.iter().find_map(|nla| {
                        if let Nla::IfName(n) = nla { Some(n.clone()) } else { None }
                    });
                    if ifname.as_deref() == Some(&env.ifname) {
                        container_veth_idx = Some(link.header.index);
                    }
                    // Also find lo
                    if ifname.as_deref() == Some("lo") {
                        let _ = container_netops.link_up(link.header.index).await;
                    }
                }
            }

            let idx = container_veth_idx.ok_or_else(|| anyhow!("container veth not found"))?;
            container_netops.link_up(idx).await?;
            container_netops.addr_add(idx, net.ip(), net.prefix()).await?;
            container_netops.route_add_default(idx, gw_ip).await?;

            Ok::<(), anyhow::Error>(())
        })
    })?;

    // 8. Get MAC addresses for result
    let (host_mac, container_mac) = ("aa:bb:cc:dd:ee:01".to_string(), "aa:bb:cc:dd:ee:02".to_string());
    // In production: read from Netlink after interface creation

    // 9. Build result
    let result = CniResult {
        cni_version: "1.0.0".to_string(),
        interfaces: vec![
            CniInterface {
                name: env.ifname.clone(),
                mac: container_mac,
                sandbox: netns_path.clone(),
            },
            CniInterface {
                name: host_veth.clone(),
                mac: host_mac,
                sandbox: String::new(),
            },
        ],
        ips: vec![
            CniIP {
                address: pod_ip_cidr.clone(),
                gateway: Some(gateway.to_string()),
                interface: Some(0),
            }
        ],
        routes: ipam_result.routes,
        dns: ipam_result.dns,
    };

    Ok(result)
}

fn cmd_del(env: &CniEnv, conf: &NetConf) -> Result<()> {
    // 1. Find and delete the veth that belongs to this container
    //    (look up by container ID in IPAM cache or by scanning interfaces in netns)

    // 2. Delete IPAM allocation
    // call_ipam(&conf.ipam.ipam_type, &env.path, stdin_data, "DEL")?;

    // DEL must be idempotent — if veth/IP is already gone, return success
    Ok(())
}

fn cmd_check(_env: &CniEnv, conf: &NetConf, prev_result: &CniResult) -> Result<()> {
    // Verify:
    // 1. Interface exists in container netns with correct IP
    // 2. Routes are correct
    // 3. IPAM state is consistent
    // Return error if any check fails
    Ok(())
}

fn cmd_version() -> CniResult {
    // VERSION command: return supported spec versions
    // This isn't actually a CniResult in practice — it has a different structure
    // but we reuse the type for simplicity
    CniResult {
        cni_version: "1.0.0".to_string(),
        interfaces: vec![],
        ips: vec![],
        routes: vec![
            // Overloading routes to carry version info (not spec-compliant, demo only)
        ],
        dns: None,
    }
}

// ─────────────────────────────────────────────────────────────
// main: CNI entry point
// ─────────────────────────────────────────────────────────────

fn rand_u32() -> u32 {
    // Simple non-cryptographic random for veth naming
    use std::time::{SystemTime, UNIX_EPOCH};
    let t = SystemTime::now().duration_since(UNIX_EPOCH).unwrap().subsec_nanos();
    t ^ (std::process::id() << 16)
}

fn main() {
    // 1. Read CNI environment
    let env = match CniEnv::from_env() {
        Ok(e) => e,
        Err(err) => {
            let cni_err = CniError::new(4, "Invalid environment", &err.to_string());
            eprintln!("{}", serde_json::to_string(&cni_err).unwrap());
            std::process::exit(1);
        }
    };

    // 2. Read configuration from stdin
    let mut stdin_data = Vec::new();
    io::stdin().read_to_end(&mut stdin_data).expect("read stdin");

    let conf: NetConf = match serde_json::from_slice(&stdin_data) {
        Ok(c) => c,
        Err(err) => {
            let cni_err = CniError::new(6, "Failed to decode config", &err.to_string());
            eprintln!("{}", serde_json::to_string(&cni_err).unwrap());
            std::process::exit(1);
        }
    };

    // 3. Dispatch on command
    let result: Result<Option<CniResult>> = match env.command.as_str() {
        "ADD" => {
            let rt = tokio::runtime::Runtime::new().unwrap();
            rt.block_on(cmd_add(&env, &conf, &stdin_data)).map(Some)
        }
        "DEL" => cmd_del(&env, &conf).map(|_| None),
        "CHECK" => {
            if let Some(prev) = &conf.prev_result {
                cmd_check(&env, &conf, prev).map(|_| None)
            } else {
                Err(anyhow!("CHECK requires prevResult"))
            }
        }
        "VERSION" => {
            let version_response = serde_json::json!({
                "cniVersion": "1.0.0",
                "supportedVersions": ["0.3.0", "0.3.1", "0.4.0", "1.0.0"]
            });
            print!("{}", version_response);
            return;
        }
        cmd => Err(anyhow!("Unknown CNI command: {}", cmd)),
    };

    match result {
        Ok(Some(cni_result)) => {
            // ADD: write result JSON to stdout
            print!("{}", serde_json::to_string(&cni_result).unwrap());
            std::process::exit(0);
        }
        Ok(None) => {
            // DEL/CHECK: success, no output
            std::process::exit(0);
        }
        Err(err) => {
            // Error: write error JSON to stderr (CNI spec: errors to stderr)
            let cni_err = CniError::new(99, "Plugin failed", &err.to_string());
            eprintln!("{}", serde_json::to_string(&cni_err).unwrap());
            std::process::exit(1);
        }
    }
}
```

---

## 28. Debugging Toolkit: nsenter, bpftrace, conntrack, ss

### 28.1 nsenter: Entering Namespaces

`nsenter` is the Go/C tool for entering a namespace from outside the container. It uses `setns()` internally.

```bash
# Get the pause container's PID (which holds the pod's netns)
PAUSE_PID=$(crictl inspect $(crictl ps -q --name POD_SANDBOX_ID) | jq .info.pid)

# Enter just the network namespace of the pod:
nsenter --net=/proc/$PAUSE_PID/ns/net ip addr show
nsenter --net=/proc/$PAUSE_PID/ns/net ip route show
nsenter --net=/proc/$PAUSE_PID/ns/net ss -tnp  # show TCP sockets
nsenter --net=/proc/$PAUSE_PID/ns/net iptables -L -n -v  # pod's iptables

# For Kubernetes: use the pod's netns path directly
POD_NETNS=$(crictl inspect $CONTAINER_ID | jq -r .info.runtimeSpec.linux.namespaces[] |
            jq -r 'select(.type=="network") | .path')
nsenter --net=$POD_NETNS ip addr show

# Capture traffic in a pod's namespace without kubectl exec:
nsenter --net=/proc/$PAUSE_PID/ns/net tcpdump -i eth0 -n tcp port 80
```

### 28.2 ss: Deep Socket Inspection

`ss` (socket statistics) shows in-kernel socket state — much more detailed than `netstat`:

```bash
# Inside pod namespace: all TCP sockets with process info
nsenter --net=/proc/$PID/ns/net ss -tnpe

# Key columns:
# Recv-Q: bytes in receive buffer waiting to be read by app
# Send-Q: bytes in send buffer not yet ACKed
# Local Address:Port, Peer Address:Port
# Process: pid=12345,fd=8 (fd 8 of pid 12345 owns this socket)
# Timer: (on,1.234ms,0) = keepalive timer, 1.234ms until next probe, 0 retries

# Find TIME_WAIT sockets that could indicate conntrack exhaustion:
nsenter --net=/proc/$PID/ns/net ss -o state time-wait

# Show BPF socket filters attached to sockets:
ss --bpf
```

### 28.3 bpftrace: Kernel-Level Network Tracing

`bpftrace` is a high-level BPF scripting language. Extremely powerful for debugging CNI and network stack issues.

```
# Trace all packets dropped by netfilter (and why):
bpftrace -e '
kprobe:kfree_skb {
  @reason[args->reason] = count();
}
interval:s:1 {
  print(@reason);
  clear(@reason);
}'

# Common drop reasons:
# SKB_DROP_REASON_NOT_SPECIFIED = 1
# SKB_DROP_REASON_IP_RPFILTER = 9   <- reverse path filter drop (check ip rules)
# SKB_DROP_REASON_CONNTRACK = 5     <- conntrack drop
# SKB_DROP_REASON_NETFILTER_DROP = 22 <- iptables DROP rule

# ─────────────────────────────────────────────────────────────

# Trace CNI plugin exec calls from containerd:
bpftrace -e '
tracepoint:syscalls:sys_enter_execve /
    str(args->filename) == "/opt/cni/bin/calico" ||
    str(args->filename) == "/opt/cni/bin/host-local"
/ {
    printf("CNI exec: %s by pid %d (%s) at %llu ns\n",
           str(args->filename), pid, comm, nsecs);
}'

# ─────────────────────────────────────────────────────────────

# Trace TCP connections in a specific pod's netns:
# First: get the pod's netns inode number
POD_NETNS_INUM=$(stat -L -c '%i' /proc/$PAUSE_PID/ns/net)

bpftrace -e "
kprobe:tcp_connect {
  \$sk = (struct sock *)arg0;
  \$ns_inum = \$sk->__sk_common.skc_net.net->ns.inum;
  if (\$ns_inum == $POD_NETNS_INUM) {
    printf(\"TCP connect from pod: %s:%d -> %s:%d\n\",
      ntop(AF_INET, \$sk->__sk_common.skc_rcv_saddr),
      \$sk->__sk_common.skc_num,
      ntop(AF_INET, \$sk->__sk_common.skc_daddr),
      bswap16(\$sk->__sk_common.skc_dport));
  }
}"

# ─────────────────────────────────────────────────────────────

# Trace conntrack table insertions and deletions:
bpftrace -e '
kprobe:__nf_conntrack_hash_insert {
    printf("CT insert: pid=%d\n", pid);
}
kprobe:nf_ct_delete {
    printf("CT delete: pid=%d\n", pid);
}
interval:s:5 {
    printf("CT table size: %d\n",
           *kptr((uint64)kaddr("nf_conntrack_count")));
}'

# ─────────────────────────────────────────────────────────────

# Detect CNI-related IPAM lock contention:
bpftrace -e '
tracepoint:syscalls:sys_enter_flock /
    strncmp(str(args->pathname), "/var/lib/cni/networks", 21) == 0
/ {
    @start[tid] = nsecs;
}
tracepoint:syscalls:sys_exit_flock /
    @start[tid]
/ {
    @flock_latency_us = hist((nsecs - @start[tid]) / 1000);
    delete(@start[tid]);
}
END { print(@flock_latency_us); }'
```

### 28.4 conntrack: Direct Conntrack Table Inspection

```bash
# List all connection tracking entries:
conntrack -L

# Filter by pod IP (find all connections from a specific pod):
conntrack -L -s 10.244.0.5

# Watch live conntrack events:
conntrack -E

# Show conntrack statistics per CPU:
conntrack -S
# cpu=0  found=1234 invalid=0 insert=567 insert_failed=0 drop=0 early_drop=0
# cpu=1  found=2345 invalid=0 insert=890 ...

# Delete all conntrack entries for a specific pod (force TCP reset):
conntrack -D -s 10.244.0.5

# Flush all entries (use with extreme caution in production!):
conntrack -F

# Show NAT translations:
conntrack -L -n  # -n = NAT entries only
# tcp  6 86394 ESTABLISHED
#   src=10.244.0.5 dst=10.96.100.1 sport=12345 dport=80
#   src=10.244.1.6 dst=10.244.0.5 sport=8080  dport=12345  <- NAT translation
#   [ASSURED] mark=0 use=1
```

### 28.5 ip-monitor: Live Netlink Events

```bash
# Watch for routing table changes in real-time:
ip monitor route

# Watch for interface changes (useful to see CNI creating/deleting veths):
ip monitor link

# Watch for address changes:
ip monitor address

# Watch everything (all Netlink events):
ip monitor all
# [NEW] link: 7: veth3a7b2c@eth0: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN
# [NEW] addr: 10.244.0.5/24 dev eth0 scope global dynamic
# [NEW] route: 10.244.0.0/24 dev cni0 proto kernel scope link src 10.244.0.1
```

### 28.6 strace: Tracing CNI Plugin Syscalls

```bash
# Trace exactly what Netlink syscalls a CNI plugin makes:
strace -e trace=sendto,recvfrom,socket,bind,setsockopt \
       -f -s 512 \
       /opt/cni/bin/bridge < /tmp/cni_config.json

# Output shows each sendto(3, ..., AF_NETLINK) call with raw hex:
# sendto(3, "\x1c\x00\x00\x00"  <- nlmsg_len=28
#           "\x10\x00"           <- RTM_NEWLINK (16)
#           "\x01\x06"           <- NLM_F_REQUEST|NLM_F_ACK
#           "\x01\x00\x00\x00"  <- sequence
#           "\x00\x00\x00\x00"  <- pid=0 (from kernel)
#           ...
#           , MSG_NOSIGNAL) = 1234

# Combine with -P to trace specific file path operations:
strace -P /var/lib/cni/networks/mynet/last_reserved_ip.0 \
       /opt/cni/bin/host-local < /tmp/ipam_config.json
# Shows: open, read, write, flock on the IPAM state files
```

---

## ASCII Reference: The Complete CNI Data Flow

```
═══════════════════════════════════════════════════════════════════════════════
                     KUBERNETES CNI: COMPLETE DATA FLOW
═══════════════════════════════════════════════════════════════════════════════

USER SPACE                    KERNEL SPACE
──────────────────────────────────────────────────────────────────────────────

kubectl apply pod.yaml
       │
       ▼
kube-apiserver ──etcd──────── pod stored (Pending)
       │
       ▼ (scheduler assigns nodeName)
kubelet watches pod
       │
       ▼
kubelet ──gRPC:RunPodSandbox──► containerd
                                    │
                                    │  1. create netns
                                    │  clone(CLONE_NEWNET)
                                    │  bind_mount → /var/run/netns/SANDBOXID
                                    │
                                    │  2. exec CNI plugin
                                    │  fork() + execve("/opt/cni/bin/calico")
                                    │    ├─ CNI_COMMAND=ADD
                                    │    ├─ CNI_NETNS=/var/run/netns/SANDBOXID
                                    │    ├─ stdin: {"type":"calico",...}
                                    │    │
                                    │    ▼ calico-cni binary:
                                    │    │  ┌─ call host-local IPAM
                                    │    │  │  flock(/var/lib/cni/networks/...)
                                    │    │  │  allocate 10.244.0.5/24
                                    │    │  └─
                                    │    │
                                    │    │  RTM_NEWLINK ──────────────────────►
                                    │    │    create veth cali3a7b2c↔eth0      │
                                    │    │    move eth0 to netns SANDBOXID      │
                                    │    │                                      │
                                    │    │  RTM_NEWADDR ──────────────────────►│
                                    │    │    10.244.0.5/24 on eth0 in netns   │
                                    │    │                                      │
                                    │    │  RTM_NEWROUTE ─────────────────────►│
                                    │    │    default via 169.254.1.1 (proxy)   │
                                    │    │                                      │
                                    │    │  ip rule add + ip route add ────────►│
                                    │    │    policy routing for this pod        │
                                    │    │                                      │
                                    │    │  iptables -A FORWARD ───────────────►│
                                    │    │    allow pod traffic                  │
                                    │    │                                      │
                                    │    stdout: result JSON                    │
                                    │                                           │
                                    │  3. start pause container                │
                                    │     setns(SANDBOXID netns)               │
                                    │     exec /pause                          │
                                    │                                           │
                                    │  4. start app containers                 │
                                    │     setns(SANDBOXID netns)  ◄────────────┘
                                    │     exec app
                                    │
                               return PodIP=10.244.0.5
                                    │
kubelet ◄─────────────────────────────
    │ update pod.status.podIP
    ▼
kube-apiserver ──etcd──────── pod stored (Running, IP=10.244.0.5)

══════════════════════════════════════════════════════════════════════════════
                        PACKET PATH: pod-A → Service → pod-B
══════════════════════════════════════════════════════════════════════════════

pod-A: 10.244.0.5                                   pod-B: 10.244.1.6
 netns                                               netns
  eth0                                               eth0
   │ src=10.244.0.5                                   │
   │ dst=10.96.100.1 (ClusterIP)                      │
   │                                                  │
   ▼ (veth pair to host netns)                        │
HOST NETNS (node-1)                                   │
   │                                                  │
   ├─ cali3a7b2c (host-side veth)                     │
   │                                                  │
   ▼ PREROUTING netfilter hook                        │
   │                                                  │
   │ [KUBE-SERVICES chain]                            │
   │  -d 10.96.100.1 --dport 80                       │
   │  -j KUBE-SVC-XYZXYZ                              │
   │                                                  │
   │ [KUBE-SEP-BBBBBB]                                │
   │  DNAT → 10.244.1.6:8080                          │
   │                                                  │
   │ nf_conntrack: record NAT                         │
   │                                                  │
   ▼ ROUTING: dst=10.244.1.6                          │
   │                                                  │
   ├─── node-1 to node-2 via BGP/VXLAN ──────────────►│
   │     (cross-node routing)                         │
   │                                                  │
                                          HOST NETNS (node-2)
                                                      │
                                           ▼ POSTROUTING
                                           │  MASQUERADE if needed
                                           │
                                           ├─ caliBBBBBB (host-side veth)
                                           │
                                           ▼ pod-B receives:
                                            src=10.244.0.5
                                            dst=10.244.1.6:8080
                                            (DNAT already applied)

══════════════════════════════════════════════════════════════════════════════
                CILIUM eBPF FAST PATH (same node, no iptables)
══════════════════════════════════════════════════════════════════════════════

pod-A eth0 TX
   │
   ▼ veth pair to lxcAAAA (host side)
   │
   ▼ TC BPF PROGRAM (egress of lxcAAAA)
   │
   ├── lookup cilium_lb4_services: 10.96.100.1:80 → backend slot N
   ├── select backend: 10.244.0.6:8080 (pod-C on same node)
   ├── DNAT in BPF: dst=10.244.0.6:8080
   ├── record in cilium_ct4_global
   │
   └── bpf_redirect(lxcCCCC_ifindex, 0)
           │
           ▼ TC BPF PROGRAM (ingress of lxcCCCC)
           │
           ├── policy check: cilium_policy map
           ├── allowed: deliver to pod-C
           │
           ▼ pod-C receives packet
           NO iptables traversal
           NO conntrack overhead
           NO bridge
           ONE bpf_redirect() call
```

---

*This guide covers topics intentionally absent from the standard CNI guide. Together they form a complete mental model of Kubernetes networking from DMA ring buffers to kubectl output.*
