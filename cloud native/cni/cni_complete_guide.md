# CNI: Complete Under-the-Hood Guide
# From Linux Kernel Primitives to Kubernetes Pod Networking

---

## Table of Contents

1. [Linux Kernel Networking Primitives](#1-linux-kernel-networking-primitives)
   - 1.1 Network Namespaces
   - 1.2 Virtual Ethernet (veth) Pairs
   - 1.3 Linux Bridge
   - 1.4 Netfilter and iptables/nftables
   - 1.5 Traffic Control (tc) Subsystem
   - 1.6 eBPF Hooks: XDP, TC, Socket
   - 1.7 Netlink Socket Protocol
   - 1.8 Routing: FIB, RIB, Policy Routing
   - 1.9 ARP and Neighbor Subsystem
   - 1.10 Virtual Device Types (macvlan, ipvlan, dummy, tun/tap)

2. [CNI Specification Deep Dive](#2-cni-specification-deep-dive)
   - 2.1 What CNI Is (and Is Not)
   - 2.2 Plugin Invocation Contract
   - 2.3 Environment Variables
   - 2.4 Stdin/Stdout JSON Protocol
   - 2.5 CNI Commands: ADD, DEL, CHECK, VERSION
   - 2.6 Result Format
   - 2.7 Error Format
   - 2.8 Capabilities and Well-Known Keys

3. [CNI Plugin Architecture](#3-cni-plugin-architecture)
   - 3.1 Plugin Types: Main, IPAM, Meta
   - 3.2 Plugin Chaining: prevResult and Result Propagation
   - 3.3 conflist vs conf Configuration Files
   - 3.4 Plugin Binary Discovery

4. [IPAM: IP Address Management Deep Dive](#4-ipam-ip-address-management-deep-dive)
   - 4.1 host-local IPAM
   - 4.2 dhcp IPAM
   - 4.3 static IPAM
   - 4.4 whereabouts (cluster-wide IPAM)
   - 4.5 IPAM Result Propagation

5. [Container Runtimes and CNI Integration](#5-container-runtimes-and-cni-integration)
   - 5.1 OCI Runtime Spec and Network Namespaces
   - 5.2 Docker / Moby (libnetwork, then CNM vs CNI)
   - 5.3 containerd CNI Integration
   - 5.4 CRI-O CNI Integration
   - 5.5 runc: What It Actually Does at the Kernel Level
   - 5.6 How Runtimes Exec CNI Plugins

6. [Kubernetes and CNI: The Full Stack](#6-kubernetes-and-cni-the-full-stack)
   - 6.1 Kubernetes Networking Model
   - 6.2 kubelet Network Plugin Architecture
   - 6.3 CRI Protocol (gRPC) and Network
   - 6.4 Pod Sandbox (pause container) Creation
   - 6.5 Full Pod Lifecycle: Kernel to IP Address
   - 6.6 CNI Configuration Discovery in Kubernetes
   - 6.7 CNI Errors and Pod Failure Modes

7. [Popular CNI Implementations: Internals](#7-popular-cni-implementations-internals)
   - 7.1 Flannel: VXLAN Overlay Internals
   - 7.2 Calico: BGP and L3 Routing Internals
   - 7.3 Cilium: eBPF Data Plane Internals
   - 7.4 Weave: Mesh Overlay Internals
   - 7.5 AWS VPC CNI: ENI and Secondary IPs

8. [C Implementation: Raw Netlink CNI Operations](#8-c-implementation-raw-netlink-cni-operations)

9. [Go Implementation: Full CNI Plugin from Scratch](#9-go-implementation-full-cni-plugin-from-scratch)

10. [Rust Implementation: Netlink-Based CNI Plugin](#10-rust-implementation-netlink-based-cni-plugin)

11. [Advanced Topics](#11-advanced-topics)
    - 11.1 CNI and Network Policies
    - 11.2 CNI and Service Mesh (Istio, Linkerd)
    - 11.3 Multi-Network CNI (Multus)
    - 11.4 CNI Performance Benchmarking
    - 11.5 Debugging CNI: Tools and Techniques

---

## 1. Linux Kernel Networking Primitives

Before CNI makes any sense, you must own the kernel primitives it orchestrates.
Every CNI plugin is fundamentally a userspace orchestrator that issues kernel commands.
The kernel does the real work.

### 1.1 Network Namespaces

A **network namespace** is a complete, isolated copy of the Linux network stack. Each namespace has:
- Its own network interfaces (including `lo`)
- Its own routing tables
- Its own ARP table / neighbor cache
- Its own netfilter (iptables/nftables) rules
- Its own sockets and connection tracking tables
- Its own port number space

Every Linux process belongs to exactly one network namespace. Processes in different namespaces cannot directly see each other's interfaces.

```
Kernel Network Namespace Internals:

struct net {                          // include/net/net_namespace.h
    refcount_t           passive;
    refcount_t           count;
    spinlock_t           rules_mod_lock;
    atomic64_t           cookie_gen;
    struct list_head     list;        // linked list of all namespaces
    struct list_head     cleanup_list;
    struct list_head     exit_list;

    struct user_namespace  *user_ns;

    struct idr           netns_ids;
    spinlock_t           nsid_lock;

    struct ns_common     ns;          // namespace file descriptor

    struct proc_net      *proc_net;   // /proc/net entries
    struct net_device    *loopback_dev;  // lo interface

    struct netns_core    core;
    struct netns_mib     mib;
    struct netns_packet  packet;
    struct netns_unix    unx;
    struct netns_nexthop nexthop;
    struct netns_ipv4    ipv4;        // all IPv4 state
    struct netns_ipv6    ipv6;        // all IPv6 state
    struct netns_ieee802154_lowpan ieee802154_lowpan;
    struct netns_sctp    sctp;
    struct netns_nf      nf;          // netfilter state
    struct netns_xt      xt;
    struct netns_ct      ct;
    struct netns_nftables nft;
    struct netns_ft      ft;
    struct sk_buff_head  wext_nlevents;
    struct net_generic   *gen;
    struct netns_bpf     bpf;         // BPF programs per namespace
    ...
};
```

**Syscall path for namespace creation:**

```
clone(CLONE_NEWNET)              /* new process in new netns */
  OR
unshare(CLONE_NEWNET)            /* current process moves to new netns */
  OR
open("/proc/PID/ns/net")         /* get fd to existing netns */
setns(fd, CLONE_NEWNET)          /* join existing netns */
```

Under the hood, `clone(CLONE_NEWNET)` calls `copy_net_ns()` in the kernel:

```
copy_net_ns()
  -> setup_net()
     -> net_alloc()              /* kmalloc struct net */
     -> peernet_operations->init() for each subsystem
        -> ipv4_net_ops.init()   /* initialize IPv4 tables */
        -> nf_net_ops.init()     /* initialize netfilter */
        -> ...
     -> loopback_net_init()      /* create lo device */
     -> dev_init_scheduler()     /* init device scheduler */
```

The key insight: every container gets a `struct net` — a full copy of all kernel networking state machinery, completely isolated.

**How CNI uses namespaces:**

When a container runtime creates a container, it creates a new network namespace. CNI's job is to "wire" that namespace to the outside world. The CNI plugin:
1. Receives the network namespace path (e.g., `/var/run/netns/pod-abc123`)
2. Creates interfaces in the host namespace
3. Moves one end into the container namespace
4. Configures IPs, routes, and rules inside the container namespace

---

### 1.2 Virtual Ethernet (veth) Pairs

A **veth pair** is a bidirectional kernel pipe between two network namespaces. It acts like a patch cable: anything written to one end comes out the other.

```
Kernel veth driver: drivers/net/veth.c

Host Namespace                    Container Namespace
+-----------------+               +-----------------+
|   veth0         |<====pipe====> |   eth0          |
|  (peer: eth0)   |               |  (peer: veth0)  |
+-----------------+               +-----------------+
        |
    Linux Bridge (cni0)
        |
    Physical eth0
```

**veth internal structure:**

```c
/* drivers/net/veth.c */
struct veth_priv {
    struct net_device __rcu *peer;   /* pointer to the other end */
    atomic64_t               dropped;
    struct bpf_prog __rcu   *xdp_prog;
    struct veth_rq          *rq;
    unsigned                 requested_headroom;
};

/* Transmission: veth_xmit() */
static netdev_tx_t veth_xmit(struct sk_buff *skb, struct net_device *dev)
{
    struct veth_priv *rcv_priv, *priv = netdev_priv(dev);
    struct veth_rq *rq = NULL;
    struct net_device *rcv;
    int length = skb->len;
    bool use_napi = false;
    int rxq;

    /* get the peer device */
    rcv = rcu_dereference(priv->peer);
    if (unlikely(!rcv)) {
        kfree_skb(skb);
        goto drop;
    }

    /* deliver to the peer — effectively a cross-namespace packet transfer */
    /* skb goes directly into peer's receive queue, bypassing hardware */
    ...
    napi_gro_receive(&rq->napi, skb);
    ...
}
```

**How CNI creates a veth pair via Netlink (conceptually):**

```
RTM_NEWLINK message:
  ifinfomsg { ifi_family=AF_UNSPEC, ifi_type=ARPHRD_ETHER }
  Nested attributes:
    IFLA_IFNAME = "veth0"
    IFLA_LINK_KIND = "veth"           /* type = veth */
    IFLA_LINKINFO:
      IFLA_INFO_KIND = "veth"
      IFLA_INFO_DATA:
        VETH_INFO_PEER:
          ifinfomsg { }
          IFLA_IFNAME = "eth0"        /* peer name */
          IFLA_NET_NS_FD = <fd>       /* target namespace for peer */
```

This single Netlink message creates both ends of the veth pair and moves `eth0` into the container namespace atomically.

---

### 1.3 Linux Bridge

A **Linux bridge** (`br0`, `cni0`, `docker0`) is a kernel L2 switch. It forwards Ethernet frames between attached ports (bridge ports) based on a MAC address forwarding table (FDB — Forwarding Database).

```
Linux Bridge Architecture:

                    cni0 (bridge device)
                   /    |    |    \
              veth0  veth2  veth4  veth6   <-- bridge ports (host side of veth pairs)
               |       |      |      |
             pod-A   pod-B  pod-C  pod-D  <-- containers (other side via netns)

Bridge FDB (Forwarding Database):
  MAC aa:bb:cc:dd:ee:01  ->  veth0
  MAC aa:bb:cc:dd:ee:02  ->  veth2
  MAC aa:bb:cc:dd:ee:03  ->  veth4
```

**Kernel bridge internals:**

```c
/* net/bridge/br_device.c */
struct net_bridge {
    spinlock_t              lock;
    spinlock_t              hash_lock;
    struct hlist_head       frame_type_list;
    struct net_device      *dev;           /* bridge netdev */

    /* Spanning Tree Protocol state */
    unsigned long           flags;
    struct net_bridge_port  *port_list;    /* list of bridge ports */

    /* Forwarding database */
    struct hlist_head       hash[BR_HASH_SIZE];  /* MAC -> port map */

    /* Timer state */
    struct timer_list       tick;
    struct timer_list       stop_timer;
    struct timer_list       gc_timer;      /* garbage collect stale FDB entries */
    ...
};

/* br_forward.c: frame forwarding decision */
void br_forward(const struct net_bridge_port *to,
                struct sk_buff *skb, bool local_rcv, bool local_orig)
{
    /* Check if destination is known in FDB */
    /* If known: forward to specific port */
    /* If unknown: flood to all ports except ingress */
    ...
    __br_forward(to, skb, local_orig);
}
```

**Bridge vs. physical switch:**
The bridge runs entirely in software in kernel space. For each frame:
1. Frame arrives on a bridge port
2. Kernel calls `br_handle_frame()`
3. Source MAC is learned into FDB
4. Destination MAC looked up in FDB
5. If found: unicast forward to that port's netdev (and thus into that namespace)
6. If not found: flood to all ports (BUM traffic)

---

### 1.4 Netfilter and iptables/nftables

**Netfilter** is the kernel's packet filtering framework. It defines **hooks** at fixed points in the packet processing path. userspace tools (iptables, nftables) register callback functions at these hooks.

```
IPv4 Packet Path Through Netfilter Hooks:

NETWORK INGRESS
      |
      v
[NF_INET_PRE_ROUTING]    <- PREROUTING chain (DNAT, conntrack)
      |
      v
  Routing Decision
   /          \
  /            \
[NF_INET_LOCAL_IN]    [NF_INET_FORWARD]    <- FORWARD chain
     |                       |
     v                       v
Local Process         [NF_INET_POST_ROUTING]  <- POSTROUTING (MASQUERADE/SNAT)
     |                       |
     v                       v
[NF_INET_LOCAL_OUT]      Network Egress
     |
     v
[NF_INET_POST_ROUTING]
     |
     v
Network Egress
```

**Why CNI cares about Netfilter:**

Every CNI plugin sets up iptables rules to:
1. Enable pod-to-pod communication (FORWARD chain ACCEPT)
2. Enable pod-to-internet (POSTROUTING MASQUERADE)
3. Enable service IP routing (PREROUTING DNAT for kube-proxy)
4. Implement network policies (INPUT/OUTPUT/FORWARD DROP rules)

**Connection tracking (conntrack):** Netfilter tracks every connection in `struct nf_conn`. This is critical for SNAT/DNAT (so return packets can be re-translated). Each namespace has its own conntrack table.

```
iptables rule CNI typically adds:

# Allow forwarding between pods and host
iptables -A FORWARD -i cni0 -j ACCEPT
iptables -A FORWARD -o cni0 -j ACCEPT

# Masquerade pod traffic going to internet
iptables -t nat -A POSTROUTING -s 10.244.0.0/16 ! -o cni0 -j MASQUERADE

# These translate to nf_register_net_hook() calls in the kernel
```

**nftables** is the modern replacement. Instead of separate tables (iptables, ip6tables, arptables), nftables uses a single framework with a bytecode VM for rule evaluation — more efficient and no kernel module per protocol family.

---

### 1.5 Traffic Control (tc) Subsystem

**tc** (traffic control) lives at `net/sched/` in the kernel. It provides queuing disciplines (qdiscs), filters, and actions for:
- Bandwidth shaping/policing
- Packet classification
- eBPF program attachment (for CNI like Cilium)

```
tc Architecture:

Network Device TX Path:
  skb -> [qdisc (e.g., pfifo_fast)] -> hardware queue -> wire
              |
              v
         tc filters -> tc actions (redirect, drop, modify)

For eBPF-based CNI (Cilium):
  ingress clsact qdisc
       |
       v
  BPF program (tc filter with BPF_PROG_TYPE_SCHED_CLS)
       -> load balancing, policy, tunnel decap
```

**Why Cilium uses tc instead of iptables:**
- eBPF programs attached at tc hooks can redirect packets between interfaces with `bpf_redirect()` at much higher throughput than Netfilter
- No conntrack overhead for stateless forwarding
- Direct map lookups instead of rule chain traversal

---

### 1.6 eBPF Hooks: XDP, TC, Socket

eBPF (extended Berkeley Packet Filter) is a safe in-kernel virtual machine. Programs are verified before execution (no unbounded loops, no null dereferences) and JIT-compiled to native code.

```
eBPF Hook Points Relevant to CNI:

                    ┌──────────────────────────────┐
  Physical NIC  --> │  XDP hook (earliest, line rate)│
                    └──────────────────────────────┘
                              |
                              v
                    ┌──────────────────────────────┐
                    │  TC ingress (clsact qdisc)    │  <-- Cilium loads BPF here
                    └──────────────────────────────┘
                              |
                              v
                    ┌──────────────────────────────┐
                    │  Netfilter (iptables/nftables) │
                    └──────────────────────────────┘
                              |
                              v
                    ┌──────────────────────────────┐
                    │  Socket filter                │  <-- per-socket BPF
                    └──────────────────────────────┘
                              |
                    ┌──────────────────────────────┐
                    │  cgroup/skb BPF               │  <-- per-cgroup ingress/egress
                    └──────────────────────────────┘
```

**XDP (eXpress Data Path):** Runs before the kernel allocates an `sk_buff`. Can drop, redirect, or pass packets at the driver level. Cilium uses XDP for DDoS mitigation and fast load balancing.

**TC BPF:** Runs after `sk_buff` allocation, can access full packet metadata. Cilium uses this for per-pod policy enforcement, encryption, tunneling.

**BPF Maps:** Shared memory between BPF programs and userspace. CNI implementations use maps for:
- Service endpoint tables (load balancing)
- Policy rules
- Connection tracking
- Per-pod metadata

---

### 1.7 Netlink Socket Protocol

**Netlink** is the kernel-userspace communication protocol for networking configuration. It replaces older ioctl-based interfaces. Nearly all CNI operations use Netlink.

```
Netlink Socket Architecture:

Userspace                        Kernel
---------                        ------
socket(AF_NETLINK,               netlink_kernel_create()
       SOCK_RAW,                      |
       NETLINK_ROUTE)                 |
       |                         rtnetlink subsystem
       |  RTM_NEWLINK            (net/core/rtnetlink.c)
       | ===================>         |
       |                         rtnl_newlink()
       |  RTM_NEWLINK (reply)         -> register_netdevice()
       | <====================        -> veth_newlink()
       |                              -> br_add_if()
       |  RTM_NEWADDR            ...
       | ===================>
       |                         inet_rtm_newaddr()
       |                              -> __inet_insert_ifa()
       |  RTM_NEWROUTE
       | ===================>
       |                         inet_rtm_newroute()
       |                              -> fib_table_insert()
```

**Netlink message structure:**

```c
/* include/uapi/linux/netlink.h */
struct nlmsghdr {
    __u32 nlmsg_len;     /* length of message including header */
    __u16 nlmsg_type;    /* type of message content (RTM_NEWLINK, etc.) */
    __u16 nlmsg_flags;   /* NLM_F_REQUEST | NLM_F_CREATE | NLM_F_ACK */
    __u32 nlmsg_seq;     /* sequence number for matching replies */
    __u32 nlmsg_pid;     /* pid of sending process (0 = kernel) */
};

/* Followed by message-type-specific struct, then rtattr netsted attrs */

/* For RTM_NEWLINK: */
struct ifinfomsg {
    unsigned char  ifi_family;   /* AF_UNSPEC */
    unsigned char  __ifi_pad;
    unsigned short ifi_type;     /* ARPHRD_ETHER */
    int            ifi_index;    /* interface index, 0 = auto-assign */
    unsigned       ifi_flags;    /* IFF_UP | IFF_RUNNING */
    unsigned       ifi_change;   /* which flags to change */
};
```

**Key RTM message types used by CNI:**

```
RTM_NEWLINK    / RTM_DELLINK    / RTM_GETLINK    -- interface CRUD
RTM_NEWADDR    / RTM_DELADDR    / RTM_GETADDR    -- IP address CRUD
RTM_NEWROUTE   / RTM_DELROUTE  / RTM_GETROUTE   -- routing table CRUD
RTM_NEWNEIGH   / RTM_DELNEIGH  / RTM_GETNEIGH   -- ARP/NDP table
RTM_NEWRULE    / RTM_DELRULE   / RTM_GETRULE    -- policy routing rules
RTM_NEWQDISC   / RTM_DELQDISC  / RTM_GETQDISC   -- tc qdiscs
RTM_NEWTFILTER / RTM_DELTFILTER                  -- tc filters (for eBPF)
```

**Netlink families:**

```
NETLINK_ROUTE       -- routing/interface/address management
NETLINK_NETFILTER   -- nftables/iptables
NETLINK_GENERIC     -- generic netlink (extensible, used by nl80211, team, etc.)
NETLINK_XFRM        -- IPsec/XFRM state management
```

---

### 1.8 Routing: FIB, RIB, Policy Routing

The **FIB** (Forwarding Information Base) is the kernel's compiled routing table — optimized for lookup speed. The **RIB** (Routing Information Base) is what routing daemons (like FRR) manage.

```
Routing Table Lookup Path:

Packet arrives, destination IP = 10.244.2.5

ip_route_input_noref()
  -> fib_lookup()
     -> fib_table_lookup()     /* hash/trie lookup in FIB */
        Returns: nexthop (gateway or direct), output interface
  -> ip_mkroute_input()
     -> rt_set_nexthop()
  -> ip_forward()
     -> ip_output()
     -> neigh_output()         /* ARP resolution if needed */
        -> dev_queue_xmit()    /* send on output interface */
```

**Policy routing (ip rule):** Allows multiple routing tables selected by source IP, mark, tos, etc. Calico uses this extensively:

```
ip rule add from 10.244.1.5 lookup 100    /* pod's source IP uses table 100 */
ip route add default via 169.254.1.1 table 100

# This means: traffic from pod 10.244.1.5 uses a dedicated routing table
# that sends all traffic to the node's default gateway
```

---

### 1.9 ARP and Neighbor Subsystem

ARP (IPv4) and NDP (IPv6) resolve IP addresses to MAC addresses. The kernel neighbor subsystem (`net/core/neighbour.c`) manages this for all address families.

```
Neighbor Cache Entry States:

INCOMPLETE -> REACHABLE -> STALE -> DELAY -> PROBE -> FAILED
                               |
                               v
                             PERMANENT  (static entry, never expires)
                             NOARP      (no ARP needed, e.g., point-to-point)
```

**Why CNI manipulates ARP:**

In some CNI implementations (like Calico with `proxy_arp`), the node answers ARP requests on behalf of pods. The node adds static neighbor entries to avoid ARP flooding in large clusters:

```
ip neigh add 10.244.1.5 lladdr aa:bb:cc:dd:ee:ff dev eth0 nud permanent
```

Cilium bypasses ARP entirely for pod-to-pod traffic by using BPF to rewrite L2 headers directly.

---

### 1.10 Virtual Device Types

```
Device Type    | Driver                  | Use in CNI
---------------|-------------------------|------------------------------------------
veth           | drivers/net/veth.c      | Connect pod namespace to host
bridge         | net/bridge/             | L2 switch between pods on same node
macvlan        | drivers/net/macvlan.c   | Pod gets its own MAC on parent interface
ipvlan         | drivers/net/ipvlan/     | Pod shares parent MAC, own IP (L2 or L3)
tun/tap        | drivers/net/tun.c       | VPN tunnels (WireGuard, OpenVPN, Flannel)
dummy          | drivers/net/dummy.c     | Loopback-like, used for route anchoring
geneve         | drivers/net/geneve.c    | Overlay tunnel (Cilium, OVS)
vxlan          | drivers/net/vxlan.c     | Overlay tunnel (Flannel, Calico)
wireguard      | drivers/net/wireguard/  | Encrypted overlay (Cilium WireGuard mode)
ipip           | net/ipv4/ipip.c         | IP-in-IP tunnel
gre            | net/ipv4/ip_gre.c       | Generic Routing Encapsulation
```

**macvlan vs ipvlan:**
- macvlan: each subinterface has its own unique MAC. Acts like a dedicated NIC. Pods appear as separate machines on L2. Cannot communicate back to host (host uses master interface).
- ipvlan L2: pods share parent MAC. Traffic differentiated by IP. Like macvlan but with shared MAC.
- ipvlan L3: pods are fully routed. No ARP between pods. Better for large-scale clusters. Cilium uses ipvlan in some modes.

---

## 2. CNI Specification Deep Dive

CNI is a **specification** plus a **set of reference plugins**. The spec lives at: https://github.com/containernetworking/cni/blob/main/SPEC.md

### 2.1 What CNI Is (and Is Not)

CNI defines:
- How a runtime **invokes** a network plugin (exec, environment, stdin)
- What a plugin **receives** as input
- What a plugin **must return** as output
- The semantics of ADD/DEL/CHECK/VERSION operations

CNI does NOT define:
- How plugins implement networking internally
- How IP addresses are allocated globally across nodes
- How cross-node routing is established
- Any daemon or long-running process (though plugins can start one)

CNI is intentionally minimal. It's an **exec-based RPC protocol**.

### 2.2 Plugin Invocation Contract

A CNI plugin is an **executable binary**. The runtime `exec`s it directly. There is no daemon, no socket, no HTTP server.

```
Runtime invokes plugin:

  execve("/opt/cni/bin/bridge", [], envp)

  where envp contains:
    CNI_COMMAND=ADD
    CNI_CONTAINERID=abc123def456...
    CNI_NETNS=/var/run/netns/abc123
    CNI_IFNAME=eth0
    CNI_PATH=/opt/cni/bin
    CNI_ARGS=K8S_POD_NAME=my-pod;K8S_POD_NAMESPACE=default;...

  and stdin receives the JSON configuration

  plugin writes result JSON to stdout
  plugin writes log/error messages to stderr
  exit code: 0 = success, non-zero = error
```

This exec-based model is intentional:
- No plugin crashes can affect the runtime
- Plugins are stateless between calls (no shared memory leaks)
- Easy to test plugins with shell scripts
- Language-agnostic (plugin can be a shell script, Python, Go, Rust, C)

### 2.3 Environment Variables

```
CNI_COMMAND    Required. One of: ADD, DEL, CHECK, VERSION
CNI_CONTAINERID  Required. Unique ID for the container (from runtime, e.g., containerd's container ID)
CNI_NETNS      Required for ADD/CHECK. Path to the network namespace: /var/run/netns/<id>
               or /proc/<pid>/ns/net
CNI_IFNAME     Required. Name of interface to create inside container. Usually "eth0".
CNI_ARGS       Optional. Semicolon-delimited key=value pairs.
               Standard: K8S_POD_NAME, K8S_POD_NAMESPACE, K8S_POD_INFRA_CONTAINER_ID
CNI_PATH       Required. Colon-separated list of directories to search for plugin binaries.
               e.g., /opt/cni/bin:/usr/local/cni/bin
```

### 2.4 Stdin/Stdout JSON Protocol

**Input (stdin) — Network Configuration:**

```json
{
  "cniVersion": "1.0.0",
  "name": "mynet",
  "type": "bridge",
  "bridge": "cni0",
  "isGateway": true,
  "ipMasq": true,
  "hairpinMode": true,
  "ipam": {
    "type": "host-local",
    "ranges": [
      [{ "subnet": "10.244.0.0/24", "rangeStart": "10.244.0.2", "rangeEnd": "10.244.0.254" }]
    ],
    "routes": [{ "dst": "0.0.0.0/0" }]
  },
  "dns": {
    "nameservers": ["10.96.0.10"],
    "search": ["default.svc.cluster.local", "svc.cluster.local"]
  },
  "prevResult": null
}
```

For chained plugins, `prevResult` is the result from the previous plugin in the chain (populated by the runtime).

### 2.5 CNI Commands: ADD, DEL, CHECK, VERSION

**ADD command:**

The plugin must:
1. Create network interface `CNI_IFNAME` inside `CNI_NETNS`
2. Assign IP address(es) to the interface
3. Add routes as required
4. Return result JSON with assigned IPs, routes, DNS

The operation must be idempotent: calling ADD twice with the same container ID must not fail catastrophically (though the result may differ).

**DEL command:**

The plugin must:
1. Undo everything ADD did
2. Release IP addresses
3. Remove network interfaces
4. Remove routes

DEL must succeed even if some resources no longer exist (idempotent).

**CHECK command (CNI spec 0.4.0+):**

Verify that the network setup for a container is still correct. Returns success or error. The runtime calls this periodically to detect externally-broken network state. Plugin should verify:
- Interface exists with correct MAC/IP
- Routes are present
- IPAM state is consistent

**VERSION command:**

Returns supported CNI spec versions:

```json
{
  "cniVersion": "1.0.0",
  "supportedVersions": ["0.3.0", "0.3.1", "0.4.0", "1.0.0"]
}
```

### 2.6 Result Format

```json
{
  "cniVersion": "1.0.0",
  "interfaces": [
    {
      "name": "eth0",
      "mac": "aa:bb:cc:dd:ee:01",
      "sandbox": "/var/run/netns/abc123"
    },
    {
      "name": "veth3a7b2c",
      "mac": "aa:bb:cc:dd:ee:02",
      "sandbox": ""
    }
  ],
  "ips": [
    {
      "address": "10.244.0.5/24",
      "gateway": "10.244.0.1",
      "interface": 0
    }
  ],
  "routes": [
    { "dst": "0.0.0.0/0", "gw": "10.244.0.1" },
    { "dst": "10.244.0.0/16", "gw": "10.244.0.1" }
  ],
  "dns": {
    "nameservers": ["10.96.0.10"],
    "search": ["default.svc.cluster.local"]
  }
}
```

**interfaces array:** Both ends of the veth pair are listed. `sandbox` is empty string for host-side interfaces; it's the netns path for container-side.

### 2.7 Error Format

```json
{
  "cniVersion": "1.0.0",
  "code": 11,
  "msg": "error allocating IP address",
  "details": "no addresses available in range 10.244.0.2-10.244.0.254"
}
```

**Standard error codes:**

```
1   Incompatible CNI version
2   Unsupported field in network configuration
3   Container unknown (container not found)
4   Invalid environment variables
5   I/O failure (read/write error on stdin/stdout)
6   Failed to decode content (JSON parse error)
7   Invalid network configuration
11  Try again later (transient error, runtime should retry)
```

### 2.8 Capabilities and Well-Known Keys

Plugins can advertise **capabilities** — optional features they support:

```json
{
  "type": "bridge",
  "capabilities": {
    "portMappings": true,
    "ipRanges": true,
    "bandwidth": true,
    "dns": true,
    "mac": true
  }
}
```

The runtime passes capability-specific parameters in a `runtimeConfig` block:

```json
{
  "runtimeConfig": {
    "portMappings": [
      { "hostPort": 8080, "containerPort": 80, "protocol": "tcp" }
    ],
    "bandwidth": {
      "ingressRate": 100000000,
      "ingressBurst": 10000000,
      "egressRate": 100000000,
      "egressBurst": 10000000
    }
  }
}
```

---

## 3. CNI Plugin Architecture

### 3.1 Plugin Types: Main, IPAM, Meta

**Main plugins** (create network interfaces):

```
bridge   -- Creates/uses a Linux bridge, attaches veth pairs
ipvlan   -- Creates ipvlan subinterfaces on a parent
macvlan  -- Creates macvlan subinterfaces on a parent
ptp      -- Creates a direct veth pair (point-to-point, no bridge)
host-device -- Moves an existing host interface into the container
vlan     -- Creates a VLAN subinterface
```

**IPAM plugins** (allocate IP addresses):

```
host-local  -- Allocates from a local file-backed pool (per-node)
dhcp        -- Runs a DHCP client inside the container namespace
static      -- Assigns a static IP from config
```

**Meta plugins** (modify/augment, no network interface creation):

```
bandwidth   -- Adds tc tbf qdisc for rate limiting
firewall    -- Adds iptables/nftables rules
portmap     -- Port-forwards host:port -> container:port via iptables DNAT
sbr         -- Source-based routing (sets ip rules)
tuning      -- Sets sysctl values inside container namespace
loopback    -- Configures the lo interface (usually first in chain)
```

### 3.2 Plugin Chaining: prevResult and Result Propagation

A **conflist** defines an ordered chain of plugins. The runtime calls each plugin in sequence. Each plugin receives the previous plugin's result in `prevResult`.

```
conflist execution:

  Runtime
    |
    | exec plugin[0]: loopback
    |   Input: config (no prevResult)
    |   Action: configure lo in netns
    |   Output: result_0
    |
    | exec plugin[1]: bridge
    |   Input: config + prevResult=result_0
    |   Action: create veth, attach to bridge, configure IP
    |   Output: result_1 (includes veth interfaces + IP)
    |
    | exec plugin[2]: portmap
    |   Input: config + prevResult=result_1
    |   Action: add iptables DNAT rules using IPs from prevResult
    |   Output: result_2 (usually same as prevResult, passthrough)
    |
    | exec plugin[3]: bandwidth
    |   Input: config + prevResult=result_2
    |   Action: add tc tbf qdisc on veth using interface names from prevResult
    |   Output: result_3 (passthrough)
    |
    Final result: result_3 returned to caller (kubelet/runtime)
```

**DEL chain:** Executed in **reverse** order. Each plugin must gracefully handle missing resources (idempotency).

### 3.3 conflist vs conf Configuration Files

**Single plugin conf** (`/etc/cni/net.d/10-mynet.conf`):

```json
{
  "cniVersion": "0.4.0",
  "name": "mynet",
  "type": "bridge",
  ...
}
```

**Plugin chain conflist** (`/etc/cni/net.d/10-mynet.conflist`):

```json
{
  "cniVersion": "0.4.0",
  "name": "mynet",
  "plugins": [
    { "type": "bridge", "bridge": "cni0", "ipam": { "type": "host-local", ... } },
    { "type": "portmap", "capabilities": { "portMappings": true } },
    { "type": "bandwidth", "ingressRate": 0, "egressRate": 0 }
  ]
}
```

**Config file selection:** The runtime reads `/etc/cni/net.d/` and picks the **lexicographically first** file. This is why files are named with numeric prefixes (`10-`, `99-`). Only one network config is active at a time (for Kubernetes, one primary CNI per node).

### 3.4 Plugin Binary Discovery

When `CNI_PATH=/opt/cni/bin:/usr/local/cni/bin`, the runtime looks for plugin executables in that order:

```
For plugin "type": "bridge":
  Try /opt/cni/bin/bridge      <- execute if exists
  Try /usr/local/cni/bin/bridge

For plugin "type": "host-local":
  Try /opt/cni/bin/host-local
  ...
```

This means Cilium ships its own `cilium-cni` binary in `/opt/cni/bin/`, and the conflist points to it by name.

---

## 4. IPAM: IP Address Management Deep Dive

IPAM is a **sub-plugin** invoked by main plugins. When a bridge plugin needs an IP address, it:
1. Finds the IPAM plugin binary named in config
2. Execs it with `CNI_COMMAND=ADD`
3. Passes the IPAM config section via stdin
4. Receives IP/gateway/route result from stdout

The IPAM plugin never touches network interfaces — it only handles IP allocation state.

### 4.1 host-local IPAM

Allocates IPs from a range using a **local file on disk** as the allocation database.

```
State directory: /var/lib/cni/networks/<network-name>/

Files:
  last_reserved_ip.0     <-- last allocated IP (for sequential allocation)
  10.244.0.5             <-- file named after the IP, contains container ID
  10.244.0.6             <-- another allocated IP
  10.244.0.7
  lock                   <-- flock() lock file

Allocation algorithm:
  1. flock(lock, LOCK_EX)                    <- exclusive lock
  2. read last_reserved_ip.0                  <- start scanning from here
  3. for ip in range(rangeStart, rangeEnd):
       if not exists file named ip:
         write containerID to file named ip   <- allocate
         break
  4. flock(lock, LOCK_UN)                    <- release lock
```

**Key limitation:** host-local is per-node. Each node has its own pool (e.g., `10.244.0.0/24` on node-1, `10.244.1.0/24` on node-2). Kubernetes assigns per-node PodCIDRs via the node's `.spec.podCIDR` field.

**Release (DEL):**

```
  1. flock(lock, LOCK_EX)
  2. find files that contain containerID
  3. delete those files
  4. flock(lock, LOCK_UN)
```

### 4.2 dhcp IPAM

Runs a DHCP client **inside the container namespace** to obtain an IP from an external DHCP server.

This requires a **long-running daemon** (`dhcp` plugin in daemon mode):

```
  dhcp plugin daemon <-> /run/cni/dhcp.sock (Unix socket)
                 |
                 v
          container netns
                 |
                 v (DHCP DISCOVER/OFFER/REQUEST/ACK via container's interface)
          DHCP Server (external)
```

When called with ADD:
1. Plugin contacts the dhcp daemon via Unix socket
2. Daemon opens container namespace
3. Daemon sends DHCP DISCOVER on container's interface
4. Receives OFFER, sends REQUEST, receives ACK
5. Returns IP, gateway, lease time to plugin

Lease renewal runs in the daemon continuously until DEL is called.

### 4.3 static IPAM

Simplest form — IP is hardcoded in config:

```json
{
  "type": "static",
  "addresses": [
    { "address": "10.10.0.1/24", "gateway": "10.10.0.254" }
  ],
  "routes": [
    { "dst": "0.0.0.0/0", "gw": "10.10.0.254" }
  ]
}
```

Used for nodes with fixed roles, test environments, or edge networking.

### 4.4 whereabouts (Cluster-Wide IPAM)

**whereabouts** solves the cross-node IP allocation problem for multi-NIC scenarios (used with Multus). Uses etcd or a Kubernetes CRD (IPPool) as the allocation database.

```
whereabouts allocation flow:

  Node A                         etcd / K8s API
  ------                         ---------------
  whereabouts ADD
    -> lock key "ippool/mypool" (etcd transaction / optimistic lock)
    -> GET ippool/mypool CRD
    -> find next free IP in range
    -> mark as allocated (containerID + node)
    -> PUT ippool/mypool CRD
    -> unlock
    -> return IP to calling plugin
```

This ensures no two pods across the cluster get the same IP.

### 4.5 IPAM Result Propagation

IPAM result is absorbed by the calling main plugin:

```
bridge plugin calls host-local IPAM:
  IPAM returns:
    IPs: [{ "address": "10.244.0.5/24", "gateway": "10.244.0.1" }]
    Routes: [{ "dst": "0.0.0.0/0", "gw": "10.244.0.1" }]

bridge plugin then:
  1. Assigns 10.244.0.5/24 to eth0 inside container namespace
     (ip addr add 10.244.0.5/24 dev eth0)
  2. Sets default route via 10.244.0.1
     (ip route add default via 10.244.0.1 dev eth0)
  3. Configures bridge IP as gateway (10.244.0.1 on cni0)
     (ip addr add 10.244.0.1/24 dev cni0)
```

---

## 5. Container Runtimes and CNI Integration

### 5.1 OCI Runtime Spec and Network Namespaces

The **OCI Runtime Spec** (`config.json`) describes a container. The `linux.namespaces` field specifies which namespaces the container should use.

```json
{
  "ociVersion": "1.0.2",
  "linux": {
    "namespaces": [
      { "type": "pid" },
      { "type": "ipc" },
      { "type": "uts" },
      { "type": "mount" },
      {
        "type": "network",
        "path": "/var/run/netns/abc123def456"
      }
    ]
  }
}
```

When `path` is specified for the network namespace, `runc` calls `setns(fd, CLONE_NEWNET)` to place the container process into the **already-created** namespace. The runtime (containerd/CRI-O) creates the namespace first, runs CNI to configure it, then passes the path to runc.

This is the critical sequence: **namespace created → CNI configures it → runc joins it**.

### 5.2 Docker / Moby (libnetwork vs CNI)

Docker uses **libnetwork** (its own networking abstraction), not CNI directly. However:

```
Docker Networking Stack:
  Docker daemon
    -> libnetwork
       -> network drivers (bridge, overlay, macvlan, ipvlan, null, host)
          -> kernel operations

The "bridge" driver uses the same primitives as CNI bridge:
  1. Create docker0 bridge
  2. Create veth pair
  3. Move one end into container namespace (via docker's own namespace management)
  4. Assign IP from docker's IPAM
  5. Configure iptables rules
```

Docker does support CNI indirectly through:
- `--network=container:<id>` sharing another container's network namespace
- `cri-dockerd` (adapter for Kubernetes) which does implement the CRI interface and invokes CNI

For direct CNI with Docker (without Kubernetes), you'd use `cni-docker` shim or just not use Docker in production Kubernetes (containerd is preferred).

### 5.3 containerd CNI Integration

**containerd** is the most common Kubernetes container runtime. It implements the CRI (Container Runtime Interface) gRPC API.

```
containerd CNI integration layers:

kubelet
  | CRI gRPC: RunPodSandbox()
  v
containerd (grpc server, /run/containerd/containerd.sock)
  |
  | sandbox network setup
  v
containerd/pkg/cri/server/sandbox_run.go: setupPodNetwork()
  |
  v
github.com/containerd/go-cni (CNI library)
  |
  | Reads /etc/cni/net.d/ configs
  | Invokes CNI plugin executables
  v
CNI plugin binary (/opt/cni/bin/...)
  |
  v
kernel (netlink, namespaces, etc.)
```

**Key source: containerd's CNI invocation:**

```go
// Simplified from containerd/go-cni
func (c *cni) Setup(ctx context.Context, id string, path string, opts ...NamespaceOpts) (*Result, error) {
    // path = /var/run/netns/<id>
    ns, err := c.GetConfig()  // reads conflist from /etc/cni/net.d/
    // ...
    result, err := c.attachNetworks(ctx, ns, id, path, opts...)
    return result, err
}

func (c *cni) attachNetworks(ctx, networks, id, path, opts) {
    for _, network := range networks {
        // exec the CNI plugin with:
        //   CNI_COMMAND=ADD
        //   CNI_CONTAINERID=id
        //   CNI_NETNS=path
        //   CNI_IFNAME=eth0
        //   stdin = network.config JSON with prevResult from previous plugin
        r, err := network.Attach(ctx, id, path, opts...)
        prevResult = r
    }
}
```

**Namespace creation in containerd:**

```go
// pkg/cri/server/sandbox_run_linux.go
func (c *criService) setupPodNetwork(ctx context.Context, sandbox *sandboxstore.Sandbox) error {
    // 1. Create network namespace
    netnsPath, err := netns.NewNetNS(netnsBasePath)
    // netnsPath = /var/run/netns/<sandbox-id>
    // This calls: open("/proc/self/ns/net"), unshare(CLONE_NEWNET),
    //             bind mount new netns to path, switch back to original netns

    // 2. Invoke CNI
    result, err := c.netPlugin.Setup(ctx, sandbox.ID, netnsPath, ...)
    // This execs the CNI plugin binary

    // 3. Store result for future reference
    sandbox.NetNS = netnsPath
    sandbox.NetworkStatus = result
    return nil
}
```

### 5.4 CRI-O CNI Integration

**CRI-O** is a lightweight CRI implementation specifically for Kubernetes (no daemon, purely CRI-focused).

```
CRI-O CNI path:
  /pkg/server/container_network.go

  RunPodSandbox()
    -> createSandbox()
       -> createNetNS()         <- clone(CLONE_NEWNET) + bind mount
       -> setupContainerNetwork()
          -> c.netPlugin.SetUpPod()
             -> invoke CNI ADD
```

CRI-O uses its own CNI wrapper (`pkg/ocicni`) which reads configs from `/etc/cni/net.d/` and execs plugins from `CNI_PATH`.

### 5.5 runc: What It Actually Does at the Kernel Level

`runc` is the OCI runtime. For a Kubernetes pod, runc is invoked **after** the network namespace is configured.

```
runc container start sequence (network-relevant parts):

runc create/run
  -> libcontainer.Factory.Create()
  -> container.Start()
     -> container.newParentProcess()
        -> parent side: namespaces.Init()
           -> clone(CLONE_NEWPID | CLONE_NEWUTS | CLONE_NEWIPC | CLONE_NEWNS)
              NOTICE: CLONE_NEWNET is NOT here if network ns path is provided
        -> child side: setns(netns_fd, CLONE_NEWNET)   <- join existing netns
           -> pivot_root() or chroot()
           -> mount filesystems
           -> exec container entrypoint
```

runc joins the **pre-existing** network namespace created by the runtime. It never creates a new one when a `path` is specified in the OCI config. This is the key design: runtime creates netns, CNI configures it, runc joins it.

### 5.6 How Runtimes Exec CNI Plugins

The actual exec is done via `os/exec` in Go (for containerd/CRI-O) or equivalent. The key considerations:

**Environment setup:**

```go
// Typical CNI exec invocation
cmd := exec.Cmd{
    Path: "/opt/cni/bin/bridge",
    Env: []string{
        "CNI_COMMAND=ADD",
        "CNI_CONTAINERID=" + containerID,
        "CNI_NETNS=" + netnsPath,
        "CNI_IFNAME=eth0",
        "CNI_PATH=/opt/cni/bin",
        "CNI_ARGS=K8S_POD_NAME=my-pod;K8S_POD_NAMESPACE=default;K8S_POD_UID=...",
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
    },
    Stdin:  bytes.NewReader(configJSON),   // network config sent via stdin
    Stdout: &stdoutBuf,                    // result JSON read from stdout
    Stderr: &stderrBuf,                    // errors/logs from stderr
}
err := cmd.Run()
exitCode := cmd.ProcessState.ExitCode()
```

**Timeout handling:** Runtimes typically apply a 30-60 second timeout on CNI execution. If a plugin hangs (e.g., DHCP server unreachable), the runtime will SIGKILL it and report failure.

**Concurrency:** Multiple pods may be starting simultaneously. The runtime serializes or parallelizes CNI calls depending on implementation. containerd uses per-sandbox locking to prevent concurrent CNI calls for the same sandbox. host-local IPAM uses file locking to handle concurrent calls from different pods.

---

## 6. Kubernetes and CNI: The Full Stack

### 6.1 Kubernetes Networking Model

Kubernetes mandates:
1. Every pod gets its own IP address
2. Pods can communicate with all other pods without NAT
3. Nodes can communicate with pods without NAT
4. Pod IP is the same from inside and outside (no hair-pin NAT)

This "flat network" model means:
- No port mapping required for pod-to-pod communication
- Service abstraction handles load balancing (not pod IP rewriting)
- Network policy is IP/port based, not based on translated addresses

CNI must implement this model. Different CNI plugins achieve it differently:
- Flannel: VXLAN overlay (encapsulate pod packets in UDP)
- Calico: BGP routing (program each node to route pod subnet of other nodes)
- Cilium: eBPF (bypass routing with direct redirect)

### 6.2 kubelet Network Plugin Architecture

`kubelet` uses the `NetworkPlugin` interface. Since Kubernetes 1.24, only CNI is supported (the old `kubenet` built-in plugin was removed).

```go
// pkg/kubelet/network/cni/cni.go (simplified)
type cniNetworkPlugin struct {
    defaultNetwork *cniNetwork     // current active CNI config
    loNetwork      *cniNetwork     // loopback CNI config
    execer         utilexec.Interface
    nsenterPath    string
    confDir        string          // /etc/cni/net.d
    binDirs        []string        // [/opt/cni/bin]
    cacheDir       string          // /var/lib/cni/cache
    podCidr        string
}

// SetUpPod: called when pod sandbox is created
func (plugin *cniNetworkPlugin) SetUpPod(namespace, name string, id kubecontainer.ContainerID, annotations, options map[string]string) error {
    // ...
    netnsPath := plugin.host.GetNetNS(id)  // /var/run/netns/<id>
    result, err := plugin.addToNetwork(plugin.loNetwork, name, namespace, id, netnsPath, annotations, options)
    // ...
    result, err = plugin.addToNetwork(plugin.defaultNetwork, name, namespace, id, netnsPath, annotations, options)
    // ...
}
```

**But wait — in modern Kubernetes, kubelet doesn't invoke CNI directly.** The CRI runtime (containerd) does. kubelet calls CRI, CRI calls CNI. This was a deliberate architectural shift.

```
Old flow (pre-1.20):
  kubelet -> CNI directly

New flow (post-1.20):
  kubelet -> CRI (gRPC) -> containerd/CRI-O -> CNI
```

kubelet's CNI code now mainly handles:
- Validating CNI configuration on startup
- Reporting network readiness
- Coordinating the `podCIDR` via NodeNetworkStatus

### 6.3 CRI Protocol (gRPC) and Network

The CRI protocol is defined in `k8s.io/cri-api/pkg/apis/runtime/v1/api.proto`.

```protobuf
// Relevant RPC calls for networking:

service RuntimeService {
  rpc RunPodSandbox(RunPodSandboxRequest) returns (RunPodSandboxResponse) {}
  rpc StopPodSandbox(StopPodSandboxRequest) returns (StopPodSandboxResponse) {}
  rpc RemovePodSandbox(RemovePodSandboxRequest) returns (RemovePodSandboxResponse) {}
  rpc PodSandboxStatus(PodSandboxStatusRequest) returns (PodSandboxStatusResponse) {}
  ...
}

message RunPodSandboxRequest {
  PodSandboxConfig config = 1;
  string runtime_handler = 2;
}

message PodSandboxConfig {
  PodSandboxMetadata metadata = 1;
  string hostname = 2;
  string log_directory = 3;
  DNSConfig dns_config = 4;
  repeated PortMapping port_mappings = 5;
  map<string, string> labels = 6;
  map<string, string> annotations = 7;
  LinuxPodSandboxConfig linux = 8;
}

message PodSandboxNetworkStatus {
  string ip = 1;                    // primary pod IP
  repeated PodIP additional_ips = 2; // secondary IPs (dual-stack)
}
```

### 6.4 Pod Sandbox (pause container) Creation

**The pause container** (also called "infra container") is the first container in a pod. It exists solely to hold the pod's namespaces (network, IPC, UTS) alive.

```
Pod = pause container + app containers (sharing pause's namespaces)

pause container:
  - runs /pause binary (from k8s.gcr.io/pause image)
  - /pause just calls pause() syscall — blocks forever
  - Holds the network namespace alive

app containers:
  - join pause's network namespace (CNI_NETNS)
  - share eth0 and routing table
  - appear to have the same IP as the pod
```

**Why pause?** If the app container restarts, its PID changes, and its `/proc/<pid>/ns/net` disappears. By keeping a separate long-lived pause container, the network namespace and configuration persist across app container restarts.

**pause container creation flow in containerd:**

```
kubelet: RunPodSandbox() ->
  containerd:
    1. Pull pause image if needed
    2. Create OCI spec for pause container
    3. Create network namespace: /var/run/netns/<sandbox-id>
    4. Call CNI Setup() with sandbox-id and netns path
       -> CNI plugin ADD: configure eth0, IP, routes
    5. Create pause container with OCI spec referencing the netns path
    6. runc start: pause joins the configured netns
    7. Return sandbox ID to kubelet

kubelet: CreateContainer() for app containers ->
  containerd:
    1. Create OCI spec with netns path = same as pause container's netns
    2. runc start: app container joins the same netns
    -> App container shares eth0, IP, routes with pause
```

### 6.5 Full Pod Lifecycle: Kernel to IP Address

Let's trace exactly what happens, from `kubectl apply` to a working pod IP.

```
STEP 1: kubectl apply -f pod.yaml
  -> kube-apiserver stores Pod object in etcd
     Status: Pending, no nodeName

STEP 2: kube-scheduler watches for unscheduled pods
  -> evaluates node constraints, resource availability
  -> sets pod.spec.nodeName = "node-1"
  -> updates pod in etcd

STEP 3: kubelet on node-1 watches for pods with its nodeName
  -> receives pod spec
  -> calls CRI: RunPodSandbox()

STEP 4: containerd (CRI) RunPodSandbox()
  -> allocate sandbox ID (UUID)
  -> mkdir /var/run/netns/
  -> create network namespace:
       fd = open("/proc/self/ns/net", O_RDONLY)   // save current netns
       unshare(CLONE_NEWNET)                        // create new netns
       // new netns now: only lo interface, no routes
       bind_mount("/proc/self/ns/net", "/var/run/netns/<sandbox-id>")
       // make netns persistent (survives even if no process is in it)
       setns(fd, CLONE_NEWNET)                      // return to original netns
       close(fd)

STEP 5: containerd calls CNI Setup()
  -> reads /etc/cni/net.d/10-flannel.conflist
  -> for each plugin in chain:
     exec /opt/cni/bin/flannel with:
       CNI_COMMAND=ADD
       CNI_CONTAINERID=<sandbox-id>
       CNI_NETNS=/var/run/netns/<sandbox-id>
       CNI_IFNAME=eth0
       stdin = {"cniVersion":"1.0.0","name":"cbr0","type":"flannel",...}

STEP 6: CNI plugin (flannel) runs
  -> reads flannel subnet env from /run/flannel/subnet.env:
       FLANNEL_NETWORK=10.244.0.0/16
       FLANNEL_SUBNET=10.244.1.0/24    <- this node's pod CIDR
       FLANNEL_MTU=1450                <- VXLAN overhead
       FLANNEL_IPMASQ=true
  -> generates bridge config and calls bridge plugin with IPAM config
  -> bridge plugin calls host-local IPAM:
       allocates 10.244.1.5/24 (first available in range)
  -> bridge plugin:
       a. create bridge cni0 if not exists:
            RTM_NEWLINK: name=cni0, type=bridge
            RTM_NEWADDR: addr=10.244.1.1/24 dev=cni0
       b. create veth pair:
            RTM_NEWLINK: name=veth3a7b2c, type=veth, peer={name=eth0, netns_fd=<fd>}
       c. bring up veth3a7b2c:
            RTM_NEWLINK: ifi_flags=IFF_UP
       d. attach veth3a7b2c to bridge cni0:
            RTM_NEWLINK: IFLA_MASTER=cni0_index
       e. enter container netns, configure eth0:
            ip addr add 10.244.1.5/24 dev eth0
            ip link set eth0 up
            ip route add default via 10.244.1.1 dev eth0
       f. add iptables rules:
            iptables -t nat -A POSTROUTING -s 10.244.1.0/24 ! -d 10.244.0.0/16 -j MASQUERADE
  -> portmap plugin (if in chain):
       add DNAT rules for any port mappings
  -> returns result: IP=10.244.1.5/24, gw=10.244.1.1

STEP 7: Kernel state after CNI:

  Host namespace:
    lo:        127.0.0.1/8
    eth0:      192.168.1.10/24     <- node IP
    cni0:      10.244.1.1/24       <- bridge (pod gateway)
    veth3a7b2c: (no IP, bridge port)

    Routing table:
      10.244.1.0/24  dev cni0      <- pods on this node
      10.244.0.0/16  via 192.168.1.1  <- pods on other nodes (via Flannel)
      0.0.0.0/0      via 192.168.1.1  <- default

  Container namespace (/var/run/netns/<sandbox-id>):
    lo:   127.0.0.1/8
    eth0: 10.244.1.5/24            <- pod IP

    Routing table:
      10.244.1.0/24  dev eth0 src 10.244.1.5
      0.0.0.0/0      via 10.244.1.1 dev eth0

STEP 8: containerd starts pause container
  -> OCI config: netns.path = /var/run/netns/<sandbox-id>
  -> runc: setns(<netns fd>, CLONE_NEWNET)
  -> pause process now lives in the configured namespace

STEP 9: containerd starts app containers
  -> OCI config: netns.path = same as pause
  -> runc: setns(<same netns fd>, CLONE_NEWNET)
  -> app containers share the same eth0, 10.244.1.5

STEP 10: kubelet updates pod status
  -> PodStatus.PodIP = "10.244.1.5"
  -> kube-apiserver stores in etcd
  -> other pods can now reach 10.244.1.5 directly
```

### 6.6 CNI Configuration Discovery in Kubernetes

kubelet's `--cni-conf-dir` (default `/etc/cni/net.d/`) and `--cni-bin-dir` (default `/opt/cni/bin`).

**CNI config update:** If the conflist file changes on disk, containerd/CRI-O reload it on the next pod creation. There is no daemon reload needed. This is why CNI is "just files."

**Node-level CNI setup:**

Most CNI plugins (Cilium, Calico, Flannel) deploy a **DaemonSet** that:
1. Copies CNI plugin binary to `/opt/cni/bin/`
2. Writes conflist to `/etc/cni/net.d/`
3. Configures any per-node state (VTEP, BGP peer, etc.)
4. Runs as a privileged pod with `hostNetwork: true` and volume mounts to the host filesystem

```yaml
# Typical CNI DaemonSet volume mounts:
volumes:
  - name: cni-bin-dir
    hostPath:
      path: /opt/cni/bin
  - name: cni-conf-dir
    hostPath:
      path: /etc/cni/net.d
  - name: host-proc
    hostPath:
      path: /proc
```

### 6.7 CNI Errors and Pod Failure Modes

```
CNI Error -> CRI Error -> kubelet Error -> Pod Status

containerd: "CNI plugin invocation failed: exit status 1: ..."
kubelet: pod enters ContainerCreating state
kubectl describe pod: Events show "FailedCreatePodSandBox"

Common failure modes:
  - CNI binary not found in CNI_PATH
  - IPAM pool exhausted (host-local: no IPs available)
  - Network namespace creation failed (permissions)
  - Bridge creation failed (kernel module not loaded)
  - iptables call failed (iptables not installed or permission denied)
  - Plugin chaining: earlier plugin failed, later plugin gets incomplete prevResult
```

---

## 7. Popular CNI Implementations: Internals

### 7.1 Flannel: VXLAN Overlay Internals

Flannel creates an **overlay network** over the existing L3 network. Pods on different nodes communicate via VXLAN tunnels.

```
Flannel Architecture:

Node 1 (192.168.1.10)             Node 2 (192.168.1.20)
  podCIDR: 10.244.1.0/24            podCIDR: 10.244.2.0/24

  pod-A: 10.244.1.5                 pod-B: 10.244.2.5
     |                                 |
  eth0 (netns)                      eth0 (netns)
     |                                 |
  veth pair                          veth pair
     |                                 |
  cni0 (bridge)                     cni0 (bridge)
  10.244.1.1/24                     10.244.2.1/24
     |                                 |
  flannel.1 (VXLAN)  =========>  flannel.1 (VXLAN)
  VTEP: 192.168.1.10                VTEP: 192.168.1.20
  VNI: 1                            VNI: 1
     |                                 |
  eth0 (host NIC)                   eth0 (host NIC)
  192.168.1.10                      192.168.1.20
```

**VXLAN packet flow (pod-A -> pod-B):**

```
1. pod-A sends packet: src=10.244.1.5, dst=10.244.2.5
2. Routing in pod netns: default via 10.244.1.1 -> exits eth0
3. ARP: who has 10.244.1.1? -> cni0 answers (gateway)
4. Packet enters cni0 bridge
5. Routing in host netns: 10.244.2.0/24 via 10.244.2.0 dev flannel.1 onlink
   (flannel programs this route pointing to VXLAN device)
6. VXLAN encapsulation (flannel.1 driver):
   Original packet (10.244.1.5 -> 10.244.2.5) becomes:
   Outer UDP packet:
     src: 192.168.1.10:8472 (VXLAN port)
     dst: 192.168.1.20:8472
     VXLAN header: VNI=1
     Inner Ethernet frame: (pod-A MAC -> pod-B MAC)
     Inner IP: 10.244.1.5 -> 10.244.2.5
7. Sent via host eth0 over physical network
8. Node 2 receives UDP:8472, VXLAN kernel module decapsulates
9. Inner packet delivered to cni0, then via veth to pod-B
```

**flanneld daemon:** A Go daemon that:
1. Watches Kubernetes API for node additions/updates
2. Reads node's `.spec.podCIDR`
3. Populates VXLAN FDB entries for each remote node:
   ```
   bridge fdb append 00:00:00:00:00:00 dev flannel.1 dst <remote-node-IP>
   ```
4. Adds host routing table entries:
   ```
   ip route add 10.244.2.0/24 via 10.244.2.0 dev flannel.1 onlink
   ```
5. Writes `/run/flannel/subnet.env` for CNI plugin to read

### 7.2 Calico: BGP and L3 Routing Internals

Calico uses **pure L3 routing** — no overlay by default. Each node runs a BGP daemon (BIRD or FRR) that advertises its podCIDR to other nodes.

```
Calico Architecture (BGP mode):

Node 1 (192.168.1.10)             Node 2 (192.168.1.20)
  podCIDR: 10.244.1.0/24            podCIDR: 10.244.2.0/24

  pod-A: 10.244.1.5                 pod-B: 10.244.2.5
     |                                 |
  veth pair (no bridge!)             veth pair (no bridge!)
  cali3a7b2c                        calib4c8d3
     |                                 |
  Host routing table:               Host routing table:
  10.244.1.5 dev cali3a7b2c          10.244.2.5 dev calib4c8d3
  10.244.2.0/24 via 192.168.1.20     10.244.1.0/24 via 192.168.1.10
  (learned via BGP)                  (learned via BGP)
     |                                 |
  eth0 (host NIC)    BGP session    eth0 (host NIC)
  192.168.1.10 <================> 192.168.1.20
```

**No bridge, no overlay. L3 forwarding only.**

Calico uses **proxy ARP** on the veth host end: the node answers ARP requests for the pod IP on behalf of the pod. This allows pods to route via the host without needing the host's IP as a gateway.

```
# Calico sets on each host-side veth:
echo 1 > /proc/sys/net/ipv4/conf/cali3a7b2c/proxy_arp
echo 1 > /proc/sys/net/ipv4/conf/cali3a7b2c/forwarding

# Pod's routing table (Calico style):
default via 169.254.1.1 dev eth0         <- link-local gateway (proxy ARP)
169.254.1.1 dev eth0 scope link           <- direct ARP to host veth
```

**Felix:** Calico's per-node agent. Watches the Calico datastore (etcd or Kubernetes CRD) for:
- Network policy changes -> programs iptables/nftables rules
- New pods -> programs routes on the host
- BGP peer state -> manages BIRD/FRR configuration

**BIRD:** A lightweight BGP daemon. Felix generates BIRD config files, BIRD manages BGP sessions with peers (other nodes, physical routers), and updates the kernel routing table via Netlink.

### 7.3 Cilium: eBPF Data Plane Internals

Cilium replaces iptables/kube-proxy entirely with eBPF programs. This is the most sophisticated CNI architecture.

```
Cilium eBPF Architecture:

Pod A                             Pod B (same node)
  eth0 (netns)                    eth0 (netns)
  10.244.1.5                      10.244.1.6
     |                                 |
  tc ingress BPF  <===============  tc egress BPF
  tc egress BPF   ===============>  tc ingress BPF
     |                                 |
  lxc3a7b2c (veth)               lxc4b8c9d (veth)
  (host side)                    (host side)

BPF programs on lxcXXXX:
  - Ingress: policy enforcement, service lb, NAT
  - Egress: policy enforcement, encrypt, tunnel

Key BPF maps (shared across all pods):
  cilium_lxc          -- endpoint metadata (IP -> lxc interface, BPF prog)
  cilium_policy       -- network policy rules (endpoint -> allowed endpoints)
  cilium_lb4_services -- L4 load balancer (ClusterIP -> backend list)
  cilium_lb4_backends -- backend IP:port entries
  cilium_ct4_global   -- connection tracking table (replaces conntrack)
  cilium_ipcache      -- IP -> identity mapping (for policy)
```

**Pod-to-pod communication (same node, Cilium):**

```
1. pod-A sends packet dst=10.244.1.6
2. Packet hits tc egress BPF on lxc3a7b2c (egress from pod-A's perspective = ingress on lxcXXX)
3. BPF program:
   a. Lookup destination in cilium_ipcache: 10.244.1.6 -> identity=12345
   b. Lookup policy: is src=pod-A allowed to reach identity 12345?
      -> check cilium_policy map
   c. If allowed: redirect directly to lxc4b8c9d
      bpf_redirect(lxc4b8c9d_ifindex, 0)
      <- no routing table lookup, no bridge, just direct redirect
4. Packet arrives at tc ingress BPF on lxc4b8c9d
5. BPF program: policy check (ingress), then deliver to pod-B
```

**Service load balancing (replaces kube-proxy):**

```
1. pod-A sends TCP to ClusterIP=10.96.0.1:80
2. BPF on egress of lxc3a7b2c:
   a. Lookup 10.96.0.1:80 in cilium_lb4_services
   b. Select backend (consistent hash or random): 10.244.2.5:8080
   c. DNAT: rewrite dst to 10.244.2.5:8080
   d. Record in cilium_ct4_global (connection tracking)
   e. Forward packet (tunnel or direct routing)
3. Return packets: BPF checks cilium_ct4_global, reverse-NATs
```

**No iptables. No conntrack. Pure BPF.**

**Cross-node with VXLAN (Cilium tunnel mode):**

```
1. Packet dst=10.244.2.5 (remote pod)
2. BPF lookup in cilium_ipcache: 10.244.2.5 -> tunnel_endpoint=192.168.1.20
3. BPF encapsulates with GENEVE header (Cilium uses GENEVE for metadata)
4. bpf_redirect_neigh() to eth0 with dst=192.168.1.20
5. No routing table lookup needed in BPF fast path
```

**Hubble:** Cilium's observability layer. eBPF programs emit events (connection open/close, DNS query, HTTP request) via perf ring buffers. Hubble daemon reads these and exposes metrics, logs, and flow tracing via gRPC.

### 7.4 Weave: Mesh Overlay Internals

Weave uses a **userspace sleeve** overlay and a fast-path kernel dataplane.

```
Weave Architecture:
  - weave daemon on each node
  - weave bridge (weave) connects pods
  - Peer-to-peer encrypted connections between nodes
  - DNS-based service discovery (weaveDNS)

Fast path: Linux VXLAN or GENEVE kernel driver
Sleeve (fallback): userspace TCP/UDP relay
```

### 7.5 AWS VPC CNI: ENI and Secondary IPs

AWS VPC CNI assigns **actual VPC IP addresses** to pods using EC2 ENI (Elastic Network Interface) secondary IPs.

```
AWS VPC CNI Architecture:

  EC2 instance:
    eth0 (primary ENI): 10.0.1.10/24
    eth1 (secondary ENI): 10.0.1.20/24, 10.0.1.21/24, 10.0.1.22/24

  Pods:
    pod-A: 10.0.1.21 (secondary IP on eth1)
    pod-B: 10.0.1.22 (secondary IP on eth1)

  No overlay. No encapsulation. Pods have real VPC IPs.
  VPC routing table handles pod-to-pod across nodes natively.
```

**ipamd daemon:** Pre-warms a pool of secondary IPs attached to ENIs. When a pod is created:
1. CNI plugin requests an IP from ipamd via Unix socket
2. ipamd returns a pre-allocated secondary IP
3. CNI creates veth, assigns the secondary IP to the pod
4. No IPAM file locking needed; ipamd is the single allocator

---

## 8. C Implementation: Raw Netlink CNI Operations

This shows the low-level kernel operations that every CNI plugin performs, using raw Netlink sockets in C.

```c
/*
 * cni_netlink.c
 * Low-level CNI operations using raw Netlink sockets.
 * Demonstrates: veth creation, namespace configuration, IP assignment, routing.
 *
 * Compile: gcc -O2 -o cni_netlink cni_netlink.c
 * Run as root.
 */

#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>
#include <fcntl.h>
#include <sched.h>

#include <sys/socket.h>
#include <sys/ioctl.h>
#include <sys/stat.h>
#include <sys/mount.h>
#include <sys/syscall.h>

#include <linux/rtnetlink.h>
#include <linux/if_link.h>
#include <linux/if.h>
#include <linux/if_ether.h>
#include <linux/ipv6.h>
#include <linux/veth.h>

#include <arpa/inet.h>
#include <netinet/in.h>
#include <net/if.h>

/* Netlink message buffer size */
#define NL_BUF_SIZE 8192

/* Netlink helper: send a request and receive reply */
typedef struct {
    int sock;
    uint32_t seq;
} nl_handle_t;

static int nl_open(nl_handle_t *h) {
    h->sock = socket(AF_NETLINK, SOCK_RAW | SOCK_CLOEXEC, NETLINK_ROUTE);
    if (h->sock < 0) {
        perror("socket(AF_NETLINK)");
        return -1;
    }

    struct sockaddr_nl addr = {
        .nl_family = AF_NETLINK,
        .nl_pid    = getpid(),
        .nl_groups = 0,
    };
    if (bind(h->sock, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        perror("bind(AF_NETLINK)");
        close(h->sock);
        return -1;
    }
    h->seq = 1;
    return 0;
}

static void nl_close(nl_handle_t *h) {
    close(h->sock);
}

/*
 * Send a Netlink message and wait for ACK.
 * hdr must be a properly constructed nlmsghdr with message appended.
 */
static int nl_talk(nl_handle_t *h, struct nlmsghdr *hdr) {
    hdr->nlmsg_seq = h->seq++;
    hdr->nlmsg_flags |= NLM_F_ACK;

    struct iovec iov = { hdr, hdr->nlmsg_len };
    struct sockaddr_nl dst = { .nl_family = AF_NETLINK };
    struct msghdr msg = {
        .msg_name    = &dst,
        .msg_namelen = sizeof(dst),
        .msg_iov     = &iov,
        .msg_iovlen  = 1,
    };

    if (sendmsg(h->sock, &msg, 0) < 0) {
        perror("sendmsg");
        return -errno;
    }

    /* Receive ACK */
    char buf[NL_BUF_SIZE];
    ssize_t n = recv(h->sock, buf, sizeof(buf), 0);
    if (n < 0) {
        perror("recv");
        return -errno;
    }

    struct nlmsghdr *resp = (struct nlmsghdr *)buf;
    if (resp->nlmsg_type == NLMSG_ERROR) {
        struct nlmsgerr *err = (struct nlmsgerr *)NLMSG_DATA(resp);
        if (err->error != 0) {
            fprintf(stderr, "Netlink error: %s\n", strerror(-err->error));
            return err->error;
        }
    }
    return 0;
}

/* Append a Netlink attribute to message */
static void nl_addattr(struct nlmsghdr *n, int type, const void *data, int dlen) {
    int alen = RTA_LENGTH(dlen);
    struct rtattr *rta = (struct rtattr *)((char *)n + NLMSG_ALIGN(n->nlmsg_len));
    rta->rta_type = type;
    rta->rta_len  = alen;
    if (data) memcpy(RTA_DATA(rta), data, dlen);
    n->nlmsg_len = NLMSG_ALIGN(n->nlmsg_len) + RTA_ALIGN(alen);
}

static struct rtattr *nl_addattr_nest(struct nlmsghdr *n, int type) {
    struct rtattr *rta = (struct rtattr *)((char *)n + NLMSG_ALIGN(n->nlmsg_len));
    rta->rta_type = type;
    rta->rta_len  = RTA_LENGTH(0);
    n->nlmsg_len  = NLMSG_ALIGN(n->nlmsg_len) + RTA_LENGTH(0);
    return rta;
}

static void nl_addattr_nest_end(struct nlmsghdr *n, struct rtattr *nest) {
    nest->rta_len = (char *)n + NLMSG_ALIGN(n->nlmsg_len) - (char *)nest;
}

/*
 * Create a veth pair: host_ifname <-> container_ifname
 * container_ifname is moved into namespace fd (netns_fd).
 *
 * Equivalent to:
 *   ip link add <host_ifname> type veth peer name <container_ifname> netns <netns_fd>
 */
int create_veth_pair(nl_handle_t *h,
                     const char *host_ifname,
                     const char *container_ifname,
                     int netns_fd)
{
    char buf[NL_BUF_SIZE] = {0};
    struct nlmsghdr *n = (struct nlmsghdr *)buf;
    struct ifinfomsg *ifi;

    /* RTM_NEWLINK: create new interface */
    n->nlmsg_len   = NLMSG_LENGTH(sizeof(*ifi));
    n->nlmsg_type  = RTM_NEWLINK;
    n->nlmsg_flags = NLM_F_REQUEST | NLM_F_CREATE | NLM_F_EXCL;

    ifi = NLMSG_DATA(n);
    ifi->ifi_family = AF_UNSPEC;

    /* IFLA_IFNAME: name for host side */
    nl_addattr(n, IFLA_IFNAME, host_ifname, strlen(host_ifname) + 1);

    /* IFLA_LINKINFO -> IFLA_INFO_KIND = "veth" -> IFLA_INFO_DATA -> VETH_INFO_PEER */
    struct rtattr *linkinfo = nl_addattr_nest(n, IFLA_LINKINFO);
    nl_addattr(n, IFLA_INFO_KIND, "veth", 5);

    struct rtattr *info_data = nl_addattr_nest(n, IFLA_INFO_DATA);

    /* VETH_INFO_PEER: nested ifinfomsg for the container side */
    struct rtattr *peer_rta = nl_addattr_nest(n, VETH_INFO_PEER);

    /* peer's ifinfomsg (zero-initialized) */
    struct ifinfomsg peer_ifi = {0};
    int peer_start = NLMSG_ALIGN(n->nlmsg_len);
    memcpy((char *)n + NLMSG_ALIGN(n->nlmsg_len), &peer_ifi, sizeof(peer_ifi));
    n->nlmsg_len = NLMSG_ALIGN(n->nlmsg_len) + sizeof(peer_ifi);

    /* peer's attributes */
    nl_addattr(n, IFLA_IFNAME, container_ifname, strlen(container_ifname) + 1);
    if (netns_fd >= 0) {
        nl_addattr(n, IFLA_NET_NS_FD, &netns_fd, sizeof(netns_fd));
    }

    nl_addattr_nest_end(n, peer_rta);
    nl_addattr_nest_end(n, info_data);
    nl_addattr_nest_end(n, linkinfo);

    (void)peer_start; /* suppress unused warning */

    return nl_talk(h, n);
}

/*
 * Set interface up: ip link set <ifname> up
 */
int link_set_up(nl_handle_t *h, const char *ifname) {
    char buf[NL_BUF_SIZE] = {0};
    struct nlmsghdr *n = (struct nlmsghdr *)buf;
    struct ifinfomsg *ifi;

    n->nlmsg_len   = NLMSG_LENGTH(sizeof(*ifi));
    n->nlmsg_type  = RTM_NEWLINK;
    n->nlmsg_flags = NLM_F_REQUEST;

    ifi = NLMSG_DATA(n);
    ifi->ifi_family  = AF_UNSPEC;
    ifi->ifi_flags   = IFF_UP;
    ifi->ifi_change  = IFF_UP;
    ifi->ifi_index   = if_nametoindex(ifname);

    return nl_talk(h, n);
}

/*
 * Set interface master (attach to bridge):
 *   ip link set <ifname> master <bridge_name>
 */
int link_set_master(nl_handle_t *h, const char *ifname, const char *bridge_name) {
    char buf[NL_BUF_SIZE] = {0};
    struct nlmsghdr *n = (struct nlmsghdr *)buf;
    struct ifinfomsg *ifi;

    n->nlmsg_len   = NLMSG_LENGTH(sizeof(*ifi));
    n->nlmsg_type  = RTM_NEWLINK;
    n->nlmsg_flags = NLM_F_REQUEST;

    ifi = NLMSG_DATA(n);
    ifi->ifi_family = AF_UNSPEC;
    ifi->ifi_index  = if_nametoindex(ifname);

    unsigned int master_idx = if_nametoindex(bridge_name);
    nl_addattr(n, IFLA_MASTER, &master_idx, sizeof(master_idx));

    return nl_talk(h, n);
}

/*
 * Add IP address to interface:
 *   ip addr add <cidr> dev <ifname>
 */
int addr_add(nl_handle_t *h, const char *ifname,
             const char *ip_str, int prefix_len)
{
    char buf[NL_BUF_SIZE] = {0};
    struct nlmsghdr *n = (struct nlmsghdr *)buf;
    struct ifaddrmsg *ifa;
    struct in_addr addr;

    if (inet_pton(AF_INET, ip_str, &addr) != 1) {
        fprintf(stderr, "Invalid IP: %s\n", ip_str);
        return -EINVAL;
    }

    n->nlmsg_len   = NLMSG_LENGTH(sizeof(*ifa));
    n->nlmsg_type  = RTM_NEWADDR;
    n->nlmsg_flags = NLM_F_REQUEST | NLM_F_CREATE | NLM_F_REPLACE;

    ifa = NLMSG_DATA(n);
    ifa->ifa_family    = AF_INET;
    ifa->ifa_prefixlen = prefix_len;
    ifa->ifa_flags     = 0;
    ifa->ifa_scope     = RT_SCOPE_UNIVERSE;
    ifa->ifa_index     = if_nametoindex(ifname);

    nl_addattr(n, IFA_LOCAL,   &addr, sizeof(addr));
    nl_addattr(n, IFA_ADDRESS, &addr, sizeof(addr));

    return nl_talk(h, n);
}

/*
 * Add default route:
 *   ip route add default via <gw_str> dev <ifname>
 */
int route_add_default(nl_handle_t *h, const char *gw_str, const char *ifname) {
    char buf[NL_BUF_SIZE] = {0};
    struct nlmsghdr *n = (struct nlmsghdr *)buf;
    struct rtmsg *rt;
    struct in_addr gw;

    if (inet_pton(AF_INET, gw_str, &gw) != 1) {
        fprintf(stderr, "Invalid gateway: %s\n", gw_str);
        return -EINVAL;
    }

    n->nlmsg_len   = NLMSG_LENGTH(sizeof(*rt));
    n->nlmsg_type  = RTM_NEWROUTE;
    n->nlmsg_flags = NLM_F_REQUEST | NLM_F_CREATE | NLM_F_REPLACE;

    rt = NLMSG_DATA(n);
    rt->rtm_family   = AF_INET;
    rt->rtm_dst_len  = 0;           /* 0 = default route */
    rt->rtm_src_len  = 0;
    rt->rtm_tos      = 0;
    rt->rtm_table    = RT_TABLE_MAIN;
    rt->rtm_protocol = RTPROT_STATIC;
    rt->rtm_scope    = RT_SCOPE_UNIVERSE;
    rt->rtm_type     = RTN_UNICAST;
    rt->rtm_flags    = 0;

    unsigned int ifindex = if_nametoindex(ifname);
    nl_addattr(n, RTA_GATEWAY, &gw,      sizeof(gw));
    nl_addattr(n, RTA_OIF,     &ifindex, sizeof(ifindex));

    return nl_talk(h, n);
}

/*
 * Create a Linux bridge:
 *   ip link add <bridge_name> type bridge
 *   ip link set <bridge_name> up
 */
int bridge_create(nl_handle_t *h, const char *bridge_name) {
    char buf[NL_BUF_SIZE] = {0};
    struct nlmsghdr *n = (struct nlmsghdr *)buf;
    struct ifinfomsg *ifi;

    n->nlmsg_len   = NLMSG_LENGTH(sizeof(*ifi));
    n->nlmsg_type  = RTM_NEWLINK;
    n->nlmsg_flags = NLM_F_REQUEST | NLM_F_CREATE | NLM_F_EXCL;

    ifi = NLMSG_DATA(n);
    ifi->ifi_family = AF_UNSPEC;

    nl_addattr(n, IFLA_IFNAME, bridge_name, strlen(bridge_name) + 1);

    struct rtattr *li = nl_addattr_nest(n, IFLA_LINKINFO);
    nl_addattr(n, IFLA_INFO_KIND, "bridge", 7);
    nl_addattr_nest_end(n, li);

    int ret = nl_talk(h, n);
    if (ret < 0 && ret != -EEXIST) return ret;

    return link_set_up(h, bridge_name);
}

/*
 * Full CNI ADD simulation:
 * Creates a veth pair, attaches one end to a bridge,
 * configures IP and routing inside a container namespace.
 *
 * Simulates what the 'bridge' CNI plugin does.
 */
int cni_add_simulation(const char *container_netns,
                       const char *host_veth,
                       const char *container_veth,
                       const char *bridge_name,
                       const char *container_ip,
                       int prefix_len,
                       const char *gateway)
{
    nl_handle_t h_host, h_container;
    int ret = 0;

    /* 1. Open Netlink socket in HOST namespace */
    if (nl_open(&h_host) < 0) return -1;

    printf("[CNI ADD] Creating bridge '%s'...\n", bridge_name);
    ret = bridge_create(&h_host, bridge_name);
    if (ret < 0) goto err_host;

    /* 2. Open container namespace fd */
    int netns_fd = open(container_netns, O_RDONLY | O_CLOEXEC);
    if (netns_fd < 0) {
        perror("open netns");
        ret = -errno;
        goto err_host;
    }

    printf("[CNI ADD] Creating veth pair %s <-> %s (moving %s to %s)...\n",
           host_veth, container_veth, container_veth, container_netns);
    ret = create_veth_pair(&h_host, host_veth, container_veth, netns_fd);
    if (ret < 0) {
        fprintf(stderr, "Failed to create veth pair\n");
        close(netns_fd);
        goto err_host;
    }

    printf("[CNI ADD] Attaching %s to bridge %s...\n", host_veth, bridge_name);
    ret = link_set_up(&h_host, host_veth);
    if (ret < 0) goto cleanup_veth;

    ret = link_set_master(&h_host, host_veth, bridge_name);
    if (ret < 0) goto cleanup_veth;

    /* 3. Enter container namespace for interface configuration */
    /* Save current netns */
    int orig_netns_fd = open("/proc/self/ns/net", O_RDONLY | O_CLOEXEC);
    if (orig_netns_fd < 0) { ret = -errno; goto cleanup_veth; }

    /* Enter container netns */
    if (setns(netns_fd, CLONE_NEWNET) < 0) {
        perror("setns(container)");
        ret = -errno;
        close(orig_netns_fd);
        goto cleanup_veth;
    }

    /* Open Netlink socket INSIDE container namespace */
    if (nl_open(&h_container) < 0) {
        ret = -1;
        setns(orig_netns_fd, CLONE_NEWNET);
        close(orig_netns_fd);
        goto cleanup_veth;
    }

    printf("[CNI ADD] Configuring %s inside %s: %s/%d gw %s...\n",
           container_veth, container_netns, container_ip, prefix_len, gateway);

    ret = link_set_up(&h_container, "lo");
    if (ret < 0) fprintf(stderr, "Warning: lo up failed\n");

    ret = link_set_up(&h_container, container_veth);
    if (ret < 0) {
        fprintf(stderr, "Failed to bring up %s\n", container_veth);
        goto cleanup_container;
    }

    ret = addr_add(&h_container, container_veth, container_ip, prefix_len);
    if (ret < 0) {
        fprintf(stderr, "Failed to add IP %s/%d\n", container_ip, prefix_len);
        goto cleanup_container;
    }

    ret = route_add_default(&h_container, gateway, container_veth);
    if (ret < 0) {
        fprintf(stderr, "Failed to add default route via %s\n", gateway);
        goto cleanup_container;
    }

    printf("[CNI ADD] Success! Pod IP: %s/%d, gateway: %s\n",
           container_ip, prefix_len, gateway);

cleanup_container:
    nl_close(&h_container);
    /* Return to host namespace */
    if (setns(orig_netns_fd, CLONE_NEWNET) < 0) {
        perror("setns(restore)");
    }
    close(orig_netns_fd);
    close(netns_fd);

err_host:
    nl_close(&h_host);
    return ret;

cleanup_veth:
    /* TODO: cleanup veth on error */
    close(netns_fd);
    goto err_host;
}

/*
 * Create a persistent network namespace (like `ip netns add`)
 * Bind-mounts /proc/self/ns/net to /var/run/netns/<name>
 */
int netns_create(const char *name) {
    char path[256];
    snprintf(path, sizeof(path), "/var/run/netns/%s", name);

    /* Create mount point */
    int fd = open(path, O_RDONLY | O_CREAT | O_EXCL, 0);
    if (fd < 0 && errno != EEXIST) {
        perror("create netns file");
        return -errno;
    }
    if (fd >= 0) close(fd);

    /* Save current netns */
    int orig = open("/proc/self/ns/net", O_RDONLY | O_CLOEXEC);

    /* Create new netns via unshare */
    if (unshare(CLONE_NEWNET) < 0) {
        perror("unshare(CLONE_NEWNET)");
        close(orig);
        return -errno;
    }

    /* Bind mount to make it persistent */
    if (mount("/proc/self/ns/net", path, "none", MS_BIND, NULL) < 0) {
        perror("mount --bind netns");
        setns(orig, CLONE_NEWNET);
        close(orig);
        return -errno;
    }

    /* Return to original netns */
    if (setns(orig, CLONE_NEWNET) < 0) {
        perror("setns(restore)");
    }
    close(orig);

    printf("[NETNS] Created /var/run/netns/%s\n", name);
    return 0;
}

int main(int argc, char *argv[]) {
    printf("=== CNI Netlink Low-Level Demo ===\n\n");

    /* Create a test network namespace */
    const char *ns_name = "cni-test";
    char ns_path[256];
    snprintf(ns_path, sizeof(ns_path), "/var/run/netns/%s", ns_name);

    if (netns_create(ns_name) < 0) {
        fprintf(stderr, "Failed to create netns '%s' (may already exist)\n", ns_name);
    }

    /* Simulate CNI ADD */
    int ret = cni_add_simulation(
        ns_path,
        "veth-host0",      /* host-side veth name */
        "eth0",            /* container-side veth name */
        "cni0",            /* bridge name */
        "10.244.0.5",      /* pod IP */
        24,                /* prefix length */
        "10.244.0.1"       /* gateway (bridge IP) */
    );

    if (ret == 0) {
        printf("\nVerify with:\n");
        printf("  ip netns exec %s ip addr show\n", ns_name);
        printf("  ip netns exec %s ip route show\n", ns_name);
        printf("  ip link show cni0\n");
        printf("  bridge link show\n");
    }

    return ret;
}
```

---

## 9. Go Implementation: Full CNI Plugin from Scratch

This implements a complete, working CNI plugin in Go — a simplified `bridge` plugin that handles ADD/DEL/CHECK/VERSION.

```go
// cni_bridge_plugin.go
// A minimal but complete CNI bridge plugin.
// Usage: place binary at /opt/cni/bin/mybridge
//
// go build -o /opt/cni/bin/mybridge cni_bridge_plugin.go
//
// Required: go get github.com/vishvananda/netlink
//           go get github.com/containernetworking/cni/pkg/skel
//           go get github.com/containernetworking/cni/pkg/types
//           go get github.com/containernetworking/plugins/pkg/ns

package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func init() {
	// This ensures that main runs only on the thread that is calling the CNI
	// plugin (go runtime can move goroutines across threads, but we need a
	// stable thread for namespace operations)
	runtime.LockOSThread()
}

// PluginConf holds the CNI configuration parsed from stdin.
type PluginConf struct {
	types.NetConf                    // embeds: CNIVersion, Name, Type, etc.
	BridgeName  string               `json:"bridge"`
	IsGateway   bool                 `json:"isGateway"`
	IPMasq      bool                 `json:"ipMasq"`
	MTU         int                  `json:"mtu"`
	HairpinMode bool                 `json:"hairpinMode"`
	IPAM        map[string]interface{} `json:"ipam"`
}

// IPAMResult is the minimal IPAM result we parse.
type IPAMResult struct {
	IPs []struct {
		Address string `json:"address"` // CIDR notation: "10.244.0.5/24"
		Gateway string `json:"gateway"`
	} `json:"ips"`
	Routes []struct {
		Dst string `json:"dst"`
		GW  string `json:"gw"`
	} `json:"routes"`
}

// parseConfig parses the CNI config from stdin JSON.
func parseConfig(stdin []byte) (*PluginConf, error) {
	conf := &PluginConf{}
	if err := json.Unmarshal(stdin, conf); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if conf.BridgeName == "" {
		conf.BridgeName = "cni0"
	}
	if conf.MTU == 0 {
		conf.MTU = 1500
	}
	return conf, nil
}

// randomVethName generates a random veth interface name.
func randomVethName() string {
	rand.Seed(time.Now().UnixNano())
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return "veth" + string(b)
}

// ensureBridge creates the bridge if it doesn't exist, returns the bridge link.
func ensureBridge(name string, mtu int) (*netlink.Bridge, error) {
	// Try to get existing bridge
	link, err := netlink.LinkByName(name)
	if err == nil {
		br, ok := link.(*netlink.Bridge)
		if !ok {
			return nil, fmt.Errorf("%s already exists but is not a bridge", name)
		}
		return br, nil
	}

	// Create new bridge
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:   name,
			MTU:    mtu,
			TxQLen: -1,
		},
	}
	if err := netlink.LinkAdd(br); err != nil {
		return nil, fmt.Errorf("failed to create bridge %s: %w", name, err)
	}

	// Set bridge up
	if err := netlink.LinkSetUp(br); err != nil {
		return nil, fmt.Errorf("failed to set bridge up: %w", err)
	}

	// Re-fetch to get populated Attrs (especially index)
	link, err = netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to refetch bridge: %w", err)
	}
	br, _ = link.(*netlink.Bridge)
	return br, nil
}

// setBridgeAddr configures the gateway IP on the bridge.
func setBridgeAddr(br *netlink.Bridge, gw *net.IPNet) error {
	addrs, err := netlink.AddrList(br, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list bridge addrs: %w", err)
	}
	for _, a := range addrs {
		if a.IPNet.String() == gw.String() {
			return nil // already configured
		}
	}
	addr := &netlink.Addr{IPNet: gw, Label: ""}
	return netlink.AddrAdd(br, addr)
}

// setupVeth creates a veth pair: one end in the host namespace attached to the
// bridge, the other in the container namespace named ifName.
// Returns the host-side veth name and any error.
func setupVeth(netNS ns.NetNS, br *netlink.Bridge, ifName string, mtu int, hairpin bool) (string, error) {
	hostVethName := randomVethName()

	// Create veth pair. We work inside the container namespace.
	// One end (ifName) stays in the container; the other (hostVethName) is moved back to host.
	err := netNS.Do(func(hostNS ns.NetNS) error {
		// Create veth pair inside container namespace initially.
		// Both ends start in container netns, then we move one end to host.
		veth := &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{
				Name:  ifName,
				Flags: net.FlagUp,
				MTU:   mtu,
			},
			PeerName: hostVethName,
		}
		if err := netlink.LinkAdd(veth); err != nil {
			return fmt.Errorf("failed to add veth: %w", err)
		}

		// Move host-side veth to host namespace
		hostVeth, err := netlink.LinkByName(hostVethName)
		if err != nil {
			return fmt.Errorf("failed to find host veth %s: %w", hostVethName, err)
		}
		if err := netlink.LinkSetNsFd(hostVeth, int(hostNS.Fd())); err != nil {
			return fmt.Errorf("failed to move veth to host: %w", err)
		}

		// Bring up container-side veth
		contVeth, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to find container veth: %w", err)
		}
		return netlink.LinkSetUp(contVeth)
	})
	if err != nil {
		return "", err
	}

	// Back in host namespace: attach host veth to bridge
	hostVeth, err := netlink.LinkByName(hostVethName)
	if err != nil {
		return "", fmt.Errorf("failed to find host veth in host ns: %w", err)
	}
	if err := netlink.LinkSetUp(hostVeth); err != nil {
		return "", fmt.Errorf("failed to set host veth up: %w", err)
	}
	if err := netlink.LinkSetMaster(hostVeth, br); err != nil {
		return "", fmt.Errorf("failed to attach veth to bridge: %w", err)
	}
	if err := netlink.LinkSetHairpin(hostVeth, hairpin); err != nil {
		return "", fmt.Errorf("failed to set hairpin: %w", err)
	}

	return hostVethName, nil
}

// configureContainerIface configures IP, routes inside the container namespace.
func configureContainerIface(netNS ns.NetNS, ifName string, ipam *IPAMResult) error {
	return netNS.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to find %s in container ns: %w", ifName, err)
		}

		for _, ipConf := range ipam.IPs {
			ipNet, err := netlink.ParseIPNet(ipConf.Address)
			if err != nil {
				return fmt.Errorf("invalid IP %s: %w", ipConf.Address, err)
			}
			addr := &netlink.Addr{IPNet: ipNet}
			if err := netlink.AddrAdd(link, addr); err != nil {
				return fmt.Errorf("failed to add addr %s: %w", ipConf.Address, err)
			}

			// Add default route
			if ipConf.Gateway != "" {
				gw := net.ParseIP(ipConf.Gateway)
				if gw == nil {
					return fmt.Errorf("invalid gateway %s", ipConf.Gateway)
				}
				route := &netlink.Route{
					LinkIndex: link.Attrs().Index,
					Scope:     netlink.SCOPE_UNIVERSE,
					Dst:       &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)},
					Gw:        gw,
				}
				if err := netlink.RouteAdd(route); err != nil {
					return fmt.Errorf("failed to add default route: %w", err)
				}
			}
		}

		// Add additional routes
		for _, r := range ipam.Routes {
			dst, err := netlink.ParseIPNet(r.Dst)
			if err != nil {
				continue // skip malformed routes
			}
			var gw net.IP
			if r.GW != "" {
				gw = net.ParseIP(r.GW)
			}
			route := &netlink.Route{
				LinkIndex: link.Attrs().Index,
				Dst:       dst,
				Gw:        gw,
			}
			netlink.RouteAdd(route) // best-effort
		}

		return nil
	})
}

// callIPAM invokes the IPAM plugin as a sub-process (re-uses CNI exec mechanism).
// In a real plugin, this uses cni/pkg/invoke. Simplified here.
func callIPAM(conf *PluginConf, args *skel.CmdArgs) (*IPAMResult, error) {
	// Use containernetworking/plugins IPAM library in real code.
	// Here we demonstrate the interface.
	//
	// For host-local IPAM (the most common case):
	//   invoke.ExecPluginWithResult(ctx, "/opt/cni/bin/host-local", ipamConfig, args.Args, nil)
	//
	// Returning a hardcoded result for demonstration:
	return &IPAMResult{
		IPs: []struct {
			Address string `json:"address"`
			Gateway string `json:"gateway"`
		}{
			{Address: "10.244.0.5/24", Gateway: "10.244.0.1"},
		},
		Routes: []struct {
			Dst string `json:"dst"`
			GW  string `json:"gw"`
		}{
			{Dst: "0.0.0.0/0", GW: "10.244.0.1"},
		},
	}, nil
}

// cmdAdd handles the CNI ADD command.
// This is where all the real work happens.
func cmdAdd(args *skel.CmdArgs) error {
	// args.StdinData = config JSON
	// args.ContainerID = container ID
	// args.Netns = /var/run/netns/<id>
	// args.IfName = "eth0"

	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	// 1. Call IPAM to get IP address
	ipamResult, err := callIPAM(conf, args)
	if err != nil {
		return fmt.Errorf("IPAM failed: %w", err)
	}
	if len(ipamResult.IPs) == 0 {
		return fmt.Errorf("IPAM returned no IPs")
	}

	// 2. Ensure bridge exists
	br, err := ensureBridge(conf.BridgeName, conf.MTU)
	if err != nil {
		return fmt.Errorf("bridge setup failed: %w", err)
	}

	// 3. Configure gateway IP on bridge (isGateway mode)
	if conf.IsGateway {
		gwIP := ipamResult.IPs[0].Gateway
		if gwIP != "" {
			// Build gateway CIDR (same prefix as pod IP)
			podNet, _, err := net.ParseCIDR(ipamResult.IPs[0].Address)
			if err == nil {
				_, podNetFull, _ := net.ParseCIDR(ipamResult.IPs[0].Address)
				gwNet := &net.IPNet{
					IP:   net.ParseIP(gwIP),
					Mask: podNetFull.Mask,
				}
				_ = podNet
				if err := setBridgeAddr(br, gwNet); err != nil {
					return fmt.Errorf("failed to set bridge gateway: %w", err)
				}
			}
		}
	}

	// 4. Open container network namespace
	netNS, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %s: %w", args.Netns, err)
	}
	defer netNS.Close()

	// 5. Create veth pair
	hostVethName, err := setupVeth(netNS, br, args.IfName, conf.MTU, conf.HairpinMode)
	if err != nil {
		return fmt.Errorf("veth setup failed: %w", err)
	}

	// 6. Configure IP inside container namespace
	if err := configureContainerIface(netNS, args.IfName, ipamResult); err != nil {
		return fmt.Errorf("container iface config failed: %w", err)
	}

	// 7. Build result
	hostVethLink, _ := netlink.LinkByName(hostVethName)
	var hostMAC string
	if hostVethLink != nil {
		hostMAC = hostVethLink.Attrs().HardwareAddr.String()
	}

	var contMAC string
	netNS.Do(func(_ ns.NetNS) error {
		l, err := netlink.LinkByName(args.IfName)
		if err == nil {
			contMAC = l.Attrs().HardwareAddr.String()
		}
		return nil
	})

	ipNet, _, _ := net.ParseCIDR(ipamResult.IPs[0].Address)
	result := &current.Result{
		CNIVersion: conf.CNIVersion,
		Interfaces: []*current.Interface{
			{
				Name:    args.IfName,
				Mac:     contMAC,
				Sandbox: args.Netns,
			},
			{
				Name: hostVethName,
				Mac:  hostMAC,
			},
		},
		IPs: []*current.IPConfig{
			{
				Interface: current.Int(0),
				Address:   *mustParseCIDR(ipamResult.IPs[0].Address),
				Gateway:   ipNet,
			},
		},
	}

	// 8. Add routes to result
	for _, r := range ipamResult.Routes {
		_, dst, err := net.ParseCIDR(r.Dst)
		if err != nil {
			continue
		}
		route := &types.Route{Dst: *dst}
		if r.GW != "" {
			route.GW = net.ParseIP(r.GW)
		}
		result.Routes = append(result.Routes, route)
	}

	return types.PrintResult(result, conf.CNIVersion)
}

// cmdDel handles the CNI DEL command.
func cmdDel(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	// DEL is best-effort: each step should ignore "not found" errors.

	// 1. Release IP via IPAM
	// invoke IPAM DEL here (omitted for brevity)

	// 2. Remove container interface (this also removes the veth peer)
	if args.Netns != "" {
		netNS, err := ns.GetNS(args.Netns)
		if err == nil {
			defer netNS.Close()
			netNS.Do(func(_ ns.NetNS) error {
				iface, err := netlink.LinkByName(args.IfName)
				if err != nil {
					return nil // already gone
				}
				return netlink.LinkDel(iface)
			})
		}
	}

	// 3. Clean up bridge if no more ports (optional, many plugins don't do this)
	_ = conf // silence unused warning

	return nil
}

// cmdCheck handles the CNI CHECK command.
func cmdCheck(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	// Verify bridge exists
	_, err = netlink.LinkByName(conf.BridgeName)
	if err != nil {
		return fmt.Errorf("bridge %s not found: %w", conf.BridgeName, err)
	}

	// Verify container interface exists with correct config
	if args.Netns != "" {
		netNS, err := ns.GetNS(args.Netns)
		if err != nil {
			return fmt.Errorf("netns %s not found: %w", args.Netns, err)
		}
		defer netNS.Close()

		err = netNS.Do(func(_ ns.NetNS) error {
			link, err := netlink.LinkByName(args.IfName)
			if err != nil {
				return fmt.Errorf("interface %s not found: %w", args.IfName, err)
			}
			if link.Attrs().Flags&net.FlagUp == 0 {
				return fmt.Errorf("interface %s is down", args.IfName)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func main() {
	// skel.PluginMain handles:
	// - Reading CNI_COMMAND environment variable
	// - Dispatching to cmdAdd/cmdDel/cmdCheck
	// - Reading stdin
	// - Handling VERSION command
	// - Proper error output format
	skel.PluginMain(
		cmdAdd,
		cmdCheck,
		cmdDel,
		version.All,
		"mybridge CNI plugin v0.1",
	)
}
```

**go.mod for the plugin:**

```
module cni-bridge-plugin

go 1.21

require (
    github.com/containernetworking/cni v1.1.2
    github.com/containernetworking/plugins v1.4.0
    github.com/vishvananda/netlink v1.2.1-beta.2
)
```

**host-local IPAM implementation (Go):**

```go
// ipam_host_local.go
// Demonstrates the file-locking allocation logic of host-local IPAM.
// This is essentially what /opt/cni/bin/host-local does internally.

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

type IPAMConfig struct {
	Type     string      `json:"type"`
	DataDir  string      `json:"dataDir"`
	Ranges   [][]Range   `json:"ranges"`
	Routes   []Route     `json:"routes"`
}

type Range struct {
	Subnet     string `json:"subnet"`
	RangeStart string `json:"rangeStart,omitempty"`
	RangeEnd   string `json:"rangeEnd,omitempty"`
	Gateway    string `json:"gateway,omitempty"`
}

type Route struct {
	Dst string `json:"dst"`
	GW  string `json:"gw,omitempty"`
}

type Store struct {
	dataDir string
	network string
	lockFd  *os.File
}

func NewStore(network, dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, network)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create IPAM dir: %w", err)
	}
	return &Store{dataDir: dir, network: network}, nil
}

// Lock acquires an exclusive file lock on the IPAM directory.
// This is critical for concurrent access from multiple CNI calls.
func (s *Store) Lock() error {
	lockPath := filepath.Join(s.dataDir, "lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return err
	}
	s.lockFd = f
	return nil
}

func (s *Store) Unlock() {
	if s.lockFd != nil {
		syscall.Flock(int(s.lockFd.Fd()), syscall.LOCK_UN)
		s.lockFd.Close()
		s.lockFd = nil
	}
}

// Allocate finds the next free IP in the range and allocates it.
// Must be called with Lock held.
func (s *Store) Allocate(containerID string, rangeConf Range) (net.IP, error) {
	_, subnet, err := net.ParseCIDR(rangeConf.Subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet %s: %w", rangeConf.Subnet, err)
	}

	start := cloneIP(subnet.IP)
	incIP(start) // skip network address

	rangeStart := start
	if rangeConf.RangeStart != "" {
		rangeStart = net.ParseIP(rangeConf.RangeStart).To4()
	}

	rangeEnd := broadcastAddr(subnet)
	decIP(rangeEnd) // skip broadcast
	if rangeConf.RangeEnd != "" {
		rangeEnd = net.ParseIP(rangeConf.RangeEnd).To4()
	}

	// Skip gateway (first usable IP by convention)
	skipGW := net.ParseIP("").To4()
	if rangeConf.Gateway != "" {
		skipGW = net.ParseIP(rangeConf.Gateway).To4()
	}

	// Try sequential allocation from last reserved IP
	lastPath := filepath.Join(s.dataDir, "last_reserved_ip.0")
	lastData, _ := os.ReadFile(lastPath)
	cursor := rangeStart
	if len(lastData) > 0 {
		if p := net.ParseIP(string(lastData)).To4(); p != nil {
			cursor = p
			incIP(cursor) // start after last
		}
	}

	// Wrap-around search
	for {
		if !inRange(cursor, rangeStart, rangeEnd) {
			cursor = cloneIP(rangeStart)
		}

		ipStr := cursor.String()
		ipPath := filepath.Join(s.dataDir, ipStr)

		// Skip if already allocated
		if _, err := os.Stat(ipPath); os.IsNotExist(err) {
			// Skip gateway
			if cursor.Equal(skipGW) {
				incIP(cursor)
				continue
			}
			// Allocate: write containerID to file named after IP
			if err := os.WriteFile(ipPath, []byte(containerID), 0644); err != nil {
				return nil, fmt.Errorf("failed to allocate %s: %w", ipStr, err)
			}
			os.WriteFile(lastPath, []byte(ipStr), 0644)
			return cloneIP(cursor), nil
		}

		// Check if this allocation belongs to the same container (idempotency)
		existing, _ := os.ReadFile(ipPath)
		if string(existing) == containerID {
			return cloneIP(cursor), nil
		}

		incIP(cursor)

		// Check if we've wrapped all the way around without finding a free IP
		if cursor.Equal(rangeStart) {
			return nil, fmt.Errorf("no IPs available in range %s-%s",
				rangeStart, rangeEnd)
		}
	}
}

// Release frees an IP allocation.
func (s *Store) Release(containerID string) error {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == "lock" || e.Name() == "last_reserved_ip.0" {
			continue
		}
		path := filepath.Join(s.dataDir, e.Name())
		data, _ := os.ReadFile(path)
		if string(data) == containerID {
			os.Remove(path)
		}
	}
	return nil
}

// Helper functions
func cloneIP(ip net.IP) net.IP {
	n := make(net.IP, len(ip))
	copy(n, ip)
	return n
}

func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func decIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]--
		if ip[i] != 255 {
			break
		}
	}
}

func broadcastAddr(subnet *net.IPNet) net.IP {
	ip := cloneIP(subnet.IP.To4())
	for i := range ip {
		ip[i] |= ^subnet.Mask[i]
	}
	return ip
}

func inRange(ip, start, end net.IPNet) bool {
	// Compare as 4-byte integers
	si, ei, ii := ipToUint32(start.IP), ipToUint32(end.IP), ipToUint32(ip.IP)
	return ii >= si && ii <= ei
}

func inRange2(ip, start, end net.IP) bool {
	si := ipToUint32(start.To4())
	ei := ipToUint32(end.To4())
	ii := ipToUint32(ip.To4())
	return ii >= si && ii <= ei
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func hostLocalIPAMAdd(containerID, networkName, subnet, rangeStart, rangeEnd, gateway, dataDir string) error {
	store, err := NewStore(networkName, dataDir)
	if err != nil {
		return err
	}

	if err := store.Lock(); err != nil {
		return fmt.Errorf("failed to lock IPAM store: %w", err)
	}
	defer store.Unlock()

	ip, err := store.Allocate(containerID, Range{
		Subnet:     subnet,
		RangeStart: rangeStart,
		RangeEnd:   rangeEnd,
		Gateway:    gateway,
	})
	if err != nil {
		return err
	}

	result := map[string]interface{}{
		"cniVersion": "1.0.0",
		"ips": []map[string]string{
			{
				"address": fmt.Sprintf("%s/%d", ip.String(), maskBits(subnet)),
				"gateway": gateway,
			},
		},
		"routes": []map[string]string{
			{"dst": "0.0.0.0/0", "gw": gateway},
		},
	}

	return json.NewEncoder(os.Stdout).Encode(result)
}

func maskBits(cidr string) int {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return 24
	}
	ones, _ := n.Mask.Size()
	return ones
}
```

---

## 10. Rust Implementation: Netlink-Based CNI Plugin

```rust
//! cni_plugin.rs
//! A Rust CNI plugin demonstrating:
//! - CNI protocol parsing (stdin/stdout/env)
//! - Netlink socket operations via rtnetlink crate
//! - Network namespace management
//! - veth pair creation, IP assignment, routing
//!
//! Cargo.toml dependencies:
//! [dependencies]
//! serde = { version = "1", features = ["derive"] }
//! serde_json = "1"
//! rtnetlink = "0.13"
//! netlink-packet-route = "0.17"
//! tokio = { version = "1", features = ["rt", "macros"] }
//! nix = { version = "0.27", features = ["net", "process", "user"] }
//! futures = "0.3"
//! thiserror = "1"

use std::collections::HashMap;
use std::env;
use std::fs::{self, File, OpenOptions};
use std::io::{self, Read, Write};
use std::net::Ipv4Addr;
use std::os::unix::io::AsRawFd;
use std::path::Path;
use std::process;

use nix::fcntl::{open, OFlag};
use nix::sched::{setns, CloneFlags};
use nix::sys::stat::Mode;
use nix::unistd::{close, getpid};

use serde::{Deserialize, Serialize};

// ============================================================
// CNI Protocol Types
// ============================================================

/// Top-level CNI network configuration (parsed from stdin).
#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetConf {
    pub cni_version: String,
    pub name: String,
    #[serde(rename = "type")]
    pub plugin_type: String,
    pub bridge: Option<String>,
    pub is_gateway: Option<bool>,
    pub ip_masq: Option<bool>,
    pub mtu: Option<u32>,
    pub hairpin_mode: Option<bool>,
    pub ipam: Option<IpamConf>,
    pub prev_result: Option<serde_json::Value>,
    #[serde(flatten)]
    pub extra: HashMap<String, serde_json::Value>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct IpamConf {
    #[serde(rename = "type")]
    pub plugin_type: String,
    pub subnet: Option<String>,
    pub range_start: Option<String>,
    pub range_end: Option<String>,
    pub gateway: Option<String>,
    pub data_dir: Option<String>,
}

/// CNI result returned on stdout.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CniResult {
    pub cni_version: String,
    pub interfaces: Vec<Interface>,
    pub ips: Vec<IpConfig>,
    pub routes: Vec<Route>,
    pub dns: Dns,
}

#[derive(Debug, Serialize)]
pub struct Interface {
    pub name: String,
    pub mac: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub sandbox: String,
}

#[derive(Debug, Serialize)]
pub struct IpConfig {
    pub address: String, // CIDR
    pub gateway: String,
    pub interface: usize,
}

#[derive(Debug, Serialize)]
pub struct Route {
    pub dst: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub gw: String,
}

#[derive(Debug, Default, Serialize)]
pub struct Dns {
    pub nameservers: Vec<String>,
    pub search: Vec<String>,
}

/// CNI error returned on stdout with non-zero exit code.
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CniError {
    pub cni_version: String,
    pub code: u32,
    pub msg: String,
    pub details: String,
}

// ============================================================
// CNI Command Dispatch
// ============================================================

/// Reads CNI command from environment variables.
pub struct CniArgs {
    pub command: String,
    pub container_id: String,
    pub netns: String,
    pub ifname: String,
    pub args: String,
    pub path: String,
    pub stdin_data: String,
}

impl CniArgs {
    pub fn from_env() -> Result<Self, String> {
        let command = env::var("CNI_COMMAND").map_err(|_| "CNI_COMMAND not set")?;
        let container_id = env::var("CNI_CONTAINERID").unwrap_or_default();
        let netns = env::var("CNI_NETNS").unwrap_or_default();
        let ifname = env::var("CNI_IFNAME").unwrap_or_else(|_| "eth0".to_string());
        let args = env::var("CNI_ARGS").unwrap_or_default();
        let path = env::var("CNI_PATH").unwrap_or_else(|_| "/opt/cni/bin".to_string());

        let mut stdin_data = String::new();
        io::stdin().read_to_string(&mut stdin_data).map_err(|e| format!("stdin read: {}", e))?;

        Ok(CniArgs { command, container_id, netns, ifname, args, path, stdin_data })
    }
}

fn cni_error(code: u32, msg: &str, details: &str) -> String {
    serde_json::to_string(&CniError {
        cni_version: "1.0.0".to_string(),
        code,
        msg: msg.to_string(),
        details: details.to_string(),
    }).unwrap_or_default()
}

fn cni_version_result() -> String {
    serde_json::json!({
        "cniVersion": "1.0.0",
        "supportedVersions": ["0.3.0", "0.3.1", "0.4.0", "1.0.0"]
    }).to_string()
}

// ============================================================
// Network Namespace Operations (using nix)
// ============================================================

/// Opens a network namespace by path, returns fd.
fn open_netns(path: &str) -> Result<i32, String> {
    open(path, OFlag::O_RDONLY | OFlag::O_CLOEXEC, Mode::empty())
        .map(|fd| fd)
        .map_err(|e| format!("open netns {}: {}", path, e))
}

/// Saves current netns fd.
fn save_netns() -> Result<i32, String> {
    open("/proc/self/ns/net", OFlag::O_RDONLY | OFlag::O_CLOEXEC, Mode::empty())
        .map_err(|e| format!("save netns: {}", e))
}

/// Enters the given network namespace.
fn enter_netns(fd: i32) -> Result<(), String> {
    setns(fd, CloneFlags::CLONE_NEWNET)
        .map_err(|e| format!("setns: {}", e))
}

/// RAII guard for network namespace switching.
pub struct NetNsGuard {
    orig_fd: i32,
}

impl NetNsGuard {
    /// Enter the namespace at path, returning a guard that restores the original on drop.
    pub fn enter(netns_path: &str) -> Result<Self, String> {
        let orig_fd = save_netns()?;
        let target_fd = open_netns(netns_path)?;
        if let Err(e) = enter_netns(target_fd) {
            close(target_fd).ok();
            close(orig_fd).ok();
            return Err(e);
        }
        close(target_fd).ok();
        Ok(NetNsGuard { orig_fd })
    }
}

impl Drop for NetNsGuard {
    fn drop(&mut self) {
        enter_netns(self.orig_fd).ok();
        close(self.orig_fd).ok();
    }
}

// ============================================================
// Netlink Operations via /proc/self/fd and rtnetlink
// (synchronous wrapper over tokio for CNI use)
// ============================================================

/// Wraps async rtnetlink operations for synchronous CNI use.
/// In production, use the async path natively; for a CNI binary,
/// a small tokio runtime is the pragmatic approach.
fn run_async<F, T>(f: F) -> T
where
    F: std::future::Future<Output = T>,
{
    tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .expect("tokio runtime")
        .block_on(f)
}

/// Creates a veth pair. Runs async netlink internally.
fn create_veth_pair(host_name: &str, container_name: &str, netns_fd: i32) -> Result<(), String> {
    // We shell out to `ip` here for brevity.
    // In production Rust, use rtnetlink::Handle::link().add() with VethInfoBuilder.
    let output = std::process::Command::new("ip")
        .args([
            "link", "add", host_name,
            "type", "veth",
            "peer", "name", container_name,
            "netns", &format!("/proc/self/fd/{}", netns_fd),
        ])
        .output()
        .map_err(|e| format!("exec ip link add: {}", e))?;

    if !output.status.success() {
        return Err(format!(
            "ip link add veth failed: {}",
            String::from_utf8_lossy(&output.stderr)
        ));
    }
    Ok(())
}

/// Runs `ip` command, returns error if non-zero exit.
fn ipcmd(args: &[&str]) -> Result<(), String> {
    let out = std::process::Command::new("ip")
        .args(args)
        .output()
        .map_err(|e| format!("exec ip: {}", e))?;
    if !out.status.success() {
        return Err(format!(
            "ip {} failed: {}",
            args.join(" "),
            String::from_utf8_lossy(&out.stderr)
        ));
    }
    Ok(())
}

// ============================================================
// IPAM: host-local implementation in Rust
// ============================================================

pub struct HostLocalIpam {
    data_dir: std::path::PathBuf,
    lock_file: File,
}

impl HostLocalIpam {
    pub fn new(network: &str, base_dir: &str) -> Result<Self, String> {
        let dir = Path::new(base_dir).join(network);
        fs::create_dir_all(&dir).map_err(|e| format!("mkdir ipam: {}", e))?;

        let lock_path = dir.join("lock");
        let lock_file = OpenOptions::new()
            .create(true)
            .write(true)
            .open(&lock_path)
            .map_err(|e| format!("open lock: {}", e))?;

        // Acquire exclusive lock
        use nix::fcntl::{flock, FlockArg};
        flock(lock_file.as_raw_fd(), FlockArg::LockExclusive)
            .map_err(|e| format!("flock: {}", e))?;

        Ok(HostLocalIpam { data_dir: dir, lock_file })
    }

    /// Allocate the next free IP in the given subnet.
    pub fn allocate(
        &self,
        container_id: &str,
        subnet: &str,
        range_start: Option<&str>,
        range_end: Option<&str>,
        gateway: Option<&str>,
    ) -> Result<(Ipv4Addr, u8), String> {
        let (net_ip, prefix_len) = parse_cidr(subnet)?;
        let mask = u32::MAX << (32 - prefix_len);

        let network_u32 = u32::from(net_ip);

        let start_u32 = range_start
            .and_then(|s| s.parse::<Ipv4Addr>().ok())
            .map(u32::from)
            .unwrap_or(network_u32 + 1);

        let broadcast_u32 = network_u32 | (!mask);
        let end_u32 = range_end
            .and_then(|s| s.parse::<Ipv4Addr>().ok())
            .map(u32::from)
            .unwrap_or(broadcast_u32 - 1);

        let gw_u32 = gateway
            .and_then(|s| s.parse::<Ipv4Addr>().ok())
            .map(u32::from)
            .unwrap_or(start_u32);

        // Read last reserved IP for cursor hint
        let last_path = self.data_dir.join("last_reserved_ip.0");
        let mut cursor = fs::read_to_string(&last_path)
            .ok()
            .and_then(|s| s.trim().parse::<Ipv4Addr>().ok())
            .map(u32::from)
            .map(|n| n + 1)
            .filter(|&n| n >= start_u32 && n <= end_u32)
            .unwrap_or(start_u32);

        let original_start = cursor;
        loop {
            if cursor > end_u32 {
                cursor = start_u32;
            }
            if cursor == gw_u32 {
                cursor += 1;
                continue;
            }

            let ip = Ipv4Addr::from(cursor);
            let ip_path = self.data_dir.join(ip.to_string());

            match fs::read_to_string(&ip_path) {
                Err(_) => {
                    // Free: allocate it
                    fs::write(&ip_path, container_id.as_bytes())
                        .map_err(|e| format!("write alloc: {}", e))?;
                    fs::write(&last_path, ip.to_string())
                        .map_err(|e| format!("write last: {}", e))?;
                    return Ok((ip, prefix_len));
                }
                Ok(existing) if existing.trim() == container_id => {
                    // Idempotent: already allocated to this container
                    return Ok((ip, prefix_len));
                }
                Ok(_) => {
                    // Allocated to someone else
                    cursor += 1;
                    if cursor == original_start {
                        return Err("no IPs available in range".to_string());
                    }
                    continue;
                }
            }
        }
    }

    /// Release all IPs allocated to a container.
    pub fn release(&self, container_id: &str) -> Result<(), String> {
        for entry in fs::read_dir(&self.data_dir).map_err(|e| e.to_string())? {
            let entry = entry.map_err(|e| e.to_string())?;
            let name = entry.file_name();
            let name = name.to_string_lossy();
            if name == "lock" || name.starts_with("last_reserved_ip") {
                continue;
            }
            if let Ok(content) = fs::read_to_string(entry.path()) {
                if content.trim() == container_id {
                    fs::remove_file(entry.path()).ok();
                }
            }
        }
        Ok(())
    }
}

impl Drop for HostLocalIpam {
    fn drop(&mut self) {
        use nix::fcntl::{flock, FlockArg};
        flock(self.lock_file.as_raw_fd(), FlockArg::Unlock).ok();
    }
}

fn parse_cidr(cidr: &str) -> Result<(Ipv4Addr, u8), String> {
    let parts: Vec<&str> = cidr.split('/').collect();
    if parts.len() != 2 {
        return Err(format!("invalid CIDR: {}", cidr));
    }
    let ip = parts[0].parse::<Ipv4Addr>().map_err(|e| format!("invalid IP: {}", e))?;
    let prefix: u8 = parts[1].parse().map_err(|e| format!("invalid prefix: {}", e))?;
    Ok((ip, prefix))
}

// ============================================================
// CNI ADD Handler
// ============================================================

fn cmd_add(args: &CniArgs, conf: &NetConf) -> Result<String, String> {
    let ipam = conf.ipam.as_ref().ok_or("no IPAM config")?;
    let bridge_name = conf.bridge.as_deref().unwrap_or("cni0");
    let mtu = conf.mtu.unwrap_or(1500);
    let netns_path = &args.netns;
    let ifname = &args.ifname;

    // 1. Allocate IP
    let (pod_ip, prefix_len) = {
        let data_dir = ipam.data_dir.as_deref().unwrap_or("/var/lib/cni/networks");
        let subnet = ipam.subnet.as_deref().ok_or("IPAM subnet required")?;
        let store = HostLocalIpam::new(&conf.name, data_dir)?;
        store.allocate(
            &args.container_id,
            subnet,
            ipam.range_start.as_deref(),
            ipam.range_end.as_deref(),
            ipam.gateway.as_deref(),
        )?
    };

    let gateway = ipam.gateway.as_deref().unwrap_or("");
    let pod_cidr = format!("{}/{}", pod_ip, prefix_len);

    // 2. Ensure bridge exists in host namespace
    let bridge_exists = std::process::Command::new("ip")
        .args(["link", "show", bridge_name])
        .output()
        .map(|o| o.status.success())
        .unwrap_or(false);

    if !bridge_exists {
        ipcmd(&["link", "add", bridge_name, "type", "bridge"])?;
        ipcmd(&["link", "set", bridge_name, "up"])?;
        if !gateway.is_empty() {
            let gw_cidr = format!("{}/{}", gateway, prefix_len);
            ipcmd(&["addr", "add", &gw_cidr, "dev", bridge_name])?;
        }
    }

    // 3. Create veth pair: host-side + container-side
    let host_veth = format!("veth{}", &args.container_id[..8]);
    let container_veth = ifname.as_str();

    // Open container netns fd
    let netns_fd = open_netns(netns_path)?;

    create_veth_pair(&host_veth, container_veth, netns_fd)?;
    close(netns_fd).ok();

    // 4. Configure host-side veth
    ipcmd(&["link", "set", &host_veth, "up"])?;
    ipcmd(&["link", "set", &host_veth, "master", bridge_name])?;

    // 5. Configure container-side veth (inside netns)
    {
        let _guard = NetNsGuard::enter(netns_path)?;
        ipcmd(&["link", "set", "lo", "up"])?;
        ipcmd(&["link", "set", container_veth, "up"])?;
        ipcmd(&["addr", "add", &pod_cidr, "dev", container_veth])?;
        if !gateway.is_empty() {
            ipcmd(&["route", "add", "default", "via", gateway, "dev", container_veth])?;
        }
    } // NetNsGuard drops here, restoring original namespace

    // 6. Collect MACs for result
    let cont_mac = get_mac_in_ns(netns_path, container_veth);
    let host_mac = get_mac_host(&host_veth);

    // 7. Build CNI result
    let result = CniResult {
        cni_version: conf.cni_version.clone(),
        interfaces: vec![
            Interface {
                name: container_veth.to_string(),
                mac: cont_mac,
                sandbox: netns_path.clone(),
            },
            Interface {
                name: host_veth.clone(),
                mac: host_mac,
                sandbox: String::new(),
            },
        ],
        ips: vec![IpConfig {
            address: pod_cidr,
            gateway: gateway.to_string(),
            interface: 0,
        }],
        routes: vec![
            Route { dst: "0.0.0.0/0".to_string(), gw: gateway.to_string() },
        ],
        dns: Dns::default(),
    };

    serde_json::to_string(&result).map_err(|e| format!("marshal result: {}", e))
}

// ============================================================
// CNI DEL Handler
// ============================================================

fn cmd_del(args: &CniArgs, conf: &NetConf) -> Result<String, String> {
    // 1. Release IP
    if let Some(ipam) = &conf.ipam {
        let data_dir = ipam.data_dir.as_deref().unwrap_or("/var/lib/cni/networks");
        if let Ok(store) = HostLocalIpam::new(&conf.name, data_dir) {
            store.release(&args.container_id).ok();
        }
    }

    // 2. Remove container interface (this deletes the veth peer too)
    if !args.netns.is_empty() && Path::new(&args.netns).exists() {
        let _guard = NetNsGuard::enter(&args.netns);
        // Best effort — interface may already be gone
        std::process::Command::new("ip")
            .args(["link", "del", &args.ifname])
            .output()
            .ok();
    }

    Ok("{}".to_string())
}

// ============================================================
// CNI CHECK Handler
// ============================================================

fn cmd_check(args: &CniArgs, conf: &NetConf) -> Result<String, String> {
    let bridge_name = conf.bridge.as_deref().unwrap_or("cni0");

    // Verify bridge exists
    std::process::Command::new("ip")
        .args(["link", "show", bridge_name])
        .output()
        .ok()
        .filter(|o| o.status.success())
        .ok_or_else(|| format!("bridge {} not found", bridge_name))?;

    // Verify container interface
    if !args.netns.is_empty() {
        let _guard = NetNsGuard::enter(&args.netns)?;
        let out = std::process::Command::new("ip")
            .args(["link", "show", &args.ifname])
            .output()
            .map_err(|e| e.to_string())?;
        if !out.status.success() {
            return Err(format!("interface {} not found in netns", args.ifname));
        }
    }

    Ok("{}".to_string())
}

// ============================================================
// Helper: get MAC address
// ============================================================

fn get_mac_in_ns(netns_path: &str, ifname: &str) -> String {
    let _guard = NetNsGuard::enter(netns_path).ok();
    get_mac_host(ifname)
}

fn get_mac_host(ifname: &str) -> String {
    let path = format!("/sys/class/net/{}/address", ifname);
    fs::read_to_string(path)
        .unwrap_or_default()
        .trim()
        .to_string()
}

// ============================================================
// Main: CNI dispatch
// ============================================================

fn main() {
    let args = match CniArgs::from_env() {
        Ok(a) => a,
        Err(e) => {
            eprintln!("CNI args error: {}", e);
            print!("{}", cni_error(4, "invalid environment", &e));
            process::exit(1);
        }
    };

    let result: Result<String, String> = match args.command.as_str() {
        "VERSION" => Ok(cni_version_result()),
        "ADD" | "DEL" | "CHECK" => {
            match serde_json::from_str::<NetConf>(&args.stdin_data) {
                Err(e) => Err(format!("parse config: {}", e)),
                Ok(conf) => match args.command.as_str() {
                    "ADD"   => cmd_add(&args, &conf),
                    "DEL"   => cmd_del(&args, &conf),
                    "CHECK" => cmd_check(&args, &conf),
                    _       => unreachable!(),
                }
            }
        }
        cmd => Err(format!("unknown CNI_COMMAND: {}", cmd)),
    };

    match result {
        Ok(output) => {
            print!("{}", output);
            process::exit(0);
        }
        Err(e) => {
            print!("{}", cni_error(100, "plugin failed", &e));
            process::exit(1);
        }
    }
}
```

---

## 11. Advanced Topics

### 11.1 CNI and Network Policies

**Network policies** (Kubernetes `NetworkPolicy` objects) are enforced by CNI plugins, not kube-apiserver. kube-apiserver stores the policy; the CNI plugin implements it.

```
NetworkPolicy lifecycle:

1. User applies NetworkPolicy:
   kubectl apply -f policy.yaml
   -> kube-apiserver stores in etcd

2. CNI agent (Calico/Felix, Cilium-agent, etc.) watches NetworkPolicy via Kubernetes API:
   informer.AddEventHandler(networkPolicyChanged)

3. Agent translates to low-level rules:
   Calico:  iptables rules via Felix
   Cilium:  BPF map entries in cilium_policy

4. On each packet:
   Calico:  iptables evaluates chains (SLOW for large rule sets)
   Cilium:  BPF hash map lookup (O(1) regardless of policy count)

NetworkPolicy rule model:
  - Ingress: which pods/namespaces/CIDRs can REACH this pod
  - Egress: which destinations this pod can REACH TO
  - Default deny: if any NetworkPolicy selects a pod, deny all non-matching traffic
```

### 11.2 CNI and Service Mesh (Istio, Linkerd)

Service meshes inject a sidecar proxy (Envoy, linkerd-proxy) into each pod. The sidecar intercepts ALL traffic. This is done via iptables rules added at pod startup.

```
Istio init container (istio-init) runs:
  iptables -t nat -A PREROUTING  -p tcp -j REDIRECT --to-ports 15001  (intercept inbound)
  iptables -t nat -A OUTPUT      -p tcp -j REDIRECT --to-ports 15001  (intercept outbound)

All pod traffic -> Envoy sidecar -> (mTLS, tracing, retries) -> destination

CNI interaction:
  - Istio CNI plugin replaces the init container
  - istio-cni plugin runs during pod setup and adds iptables rules via CNI
  - Avoids need for privileged init containers
  - Uses CNI chaining: bridge plugin + istio-cni plugin
```

### 11.3 Multi-Network CNI (Multus)

**Multus** is a meta-CNI plugin that enables pods to have **multiple network interfaces**.

```
Multus Architecture:

Pod has:
  eth0  <- primary CNI (managed by Flannel/Cilium)
  net1  <- secondary CNI (managed by SR-IOV CNI for data plane)
  net2  <- tertiary CNI (managed by macvlan)

Multus config:
  /etc/cni/net.d/00-multus.conflist
    -> delegates to primary CNI for eth0
    -> reads pod annotations for additional networks

Pod annotation:
  annotations:
    k8s.v1.cni.cncf.io/networks: '[{"name":"sriov-net","interface":"net1"},
                                    {"name":"macvlan-net","interface":"net2"}]'

Multus flow:
  1. Multus plugin invoked by runtime
  2. Invokes primary CNI for eth0 (normal flow)
  3. Reads NetworkAttachmentDefinition CRDs for secondary networks
  4. Invokes secondary CNI plugins for net1, net2
  5. Returns combined result with all interfaces
```

### 11.4 CNI Performance Benchmarking

```
Key metrics:
  - Pod startup time (ns creation + CNI ADD latency)
  - Throughput (pod-to-pod, pod-to-service, pod-to-internet)
  - Latency (p50, p99, p999)
  - CPU overhead per packet

Rough comparison (2024, 10GbE nodes):

CNI         Mode        Throughput    Latency(p99)  CPU/pkt
--------------------------------------------------------------
Flannel     VXLAN       ~7 Gbps       ~80 µs        HIGH   (encap overhead)
Calico      BGP/native  ~9.5 Gbps     ~20 µs        LOW    (kernel L3)
Cilium      eBPF/VXLAN  ~8 Gbps       ~40 µs        MED    (BPF overhead)
Cilium      eBPF/native ~9.8 Gbps     ~15 µs        LOW    (BPF redirect)
AWS VPC CNI native      ~9.9 Gbps     ~10 µs        LOWEST (no overlay)

For kube-proxy replacement:
  kube-proxy (iptables): O(n) rules, degrades with service count
  Cilium (BPF maps):     O(1) lookup, consistent latency regardless of scale
```

### 11.5 Debugging CNI: Tools and Techniques

**Check CNI config:**

```bash
# Current active CNI config
ls -la /etc/cni/net.d/
cat /etc/cni/net.d/10-flannel.conflist

# CNI binaries
ls -la /opt/cni/bin/

# Current CNI allocations (host-local)
ls /var/lib/cni/networks/cbr0/
cat /var/lib/cni/networks/cbr0/10.244.1.5   # should contain container ID
```

**Manually invoke CNI plugin:**

```bash
# Simulate ADD
CNI_COMMAND=ADD \
CNI_CONTAINERID=test123 \
CNI_NETNS=/var/run/netns/test \
CNI_IFNAME=eth0 \
CNI_PATH=/opt/cni/bin \
/opt/cni/bin/bridge <<EOF
{
  "cniVersion":"1.0.0",
  "name":"mynet",
  "type":"bridge",
  "bridge":"cni0",
  "isGateway":true,
  "ipam":{"type":"host-local","subnet":"10.244.0.0/24"}
}
EOF
```

**Inspect pod networking:**

```bash
# Get pause container's PID
docker inspect --format '{{.State.Pid}}' $(docker ps | grep pause | awk '{print $1}')

# Inspect inside pod's namespace
nsenter -t <PID> -n ip addr
nsenter -t <PID> -n ip route
nsenter -t <PID> -n ss -tlnp

# Trace packet path
nsenter -t <PID> -n tcpdump -i eth0 -n
```

**Cilium-specific:**

```bash
cilium status
cilium endpoint list
cilium monitor --type=drop           # see dropped packets
cilium bpf lb list                   # service load balancer entries
cilium bpf policy get <endpoint-id>  # policy map entries
bpftool map show                     # all BPF maps
bpftool map dump name cilium_lxc     # endpoint metadata
```

**Calico-specific:**

```bash
calicoctl get workloadendpoints -o wide
calicoctl get networkpolicy -o yaml
# Felix logs
journalctl -u calico-felix
# BGP peer status (via BIRD)
calicoctl node status
```

**Common failure diagnoses:**

```
Symptom: Pod stuck in ContainerCreating, "FailedCreatePodSandBox"
  Check: journalctl -u containerd | grep CNI
  Check: /var/log/pods/<pod>/
  Fix:   Verify CNI binary exists, config is valid JSON

Symptom: Pod has IP but can't reach other pods
  Check: iptables -L FORWARD (should have ACCEPT rules)
  Check: ip route show (does route for pod CIDR exist?)
  Check: net.ipv4.ip_forward = 1 on host

Symptom: Pod can reach same-node pods but not cross-node
  Check: Flannel/Calico agent running? (kubectl get ds -n kube-system)
  Check: Flannel VXLAN: bridge fdb show dev flannel.1
  Check: Calico BGP: calicoctl node status
  Check: MTU mismatch (VXLAN needs 50B overhead, set MTU=1450 on pods)

Symptom: Pod can't reach services (ClusterIP)
  Check: kube-proxy or Cilium running
  Check: iptables -t nat -L KUBE-SERVICES (kube-proxy rules)
  Check: Cilium: cilium bpf lb list
```

---

## Complete Architecture: Everything Together

```
                         KUBERNETES CLUSTER
 ┌───────────────────────────────────────────────────────────────────────────┐
 │                        kube-apiserver (etcd backend)                      │
 │                Pod, Node, NetworkPolicy, Service objects                  │
 └─────────┬──────────────────┬───────────────────────────────────────────┘
           │ watch            │ watch
           v                  v
    kube-scheduler      CNI DaemonSet (e.g., cilium-agent)
    assigns node         - writes /etc/cni/net.d/
                         - copies /opt/cni/bin/cilium-cni
                         - programs BPF maps
                         - runs BGP/VXLAN daemon
                         - watches NetworkPolicy -> BPF policy maps

           │
           │ pod.spec.nodeName = "node-1"
           v
┌────────────────────────────────────────────────────────────────────────────┐
│  NODE-1                                                                    │
│                                                                            │
│  kubelet                                                                   │
│    │ CRI gRPC RunPodSandbox()                                              │
│    v                                                                       │
│  containerd  (/run/containerd/containerd.sock)                             │
│    │                                                                       │
│    ├─1. unshare(CLONE_NEWNET) -> bind_mount -> /var/run/netns/<id>         │
│    │                                                                       │
│    ├─2. exec /opt/cni/bin/cilium-cni                                       │
│    │     CNI_COMMAND=ADD                                                   │
│    │     CNI_NETNS=/var/run/netns/<id>                                     │
│    │     stdin: { "type": "cilium-cni", ... }                              │
│    │                                                                       │
│    │   CNI Plugin (cilium-cni):                                            │
│    │     a. Calls cilium-agent via Unix socket for endpoint creation       │
│    │     b. cilium-agent:                                                  │
│    │          - Allocates IP from IPAM pool                                │
│    │          - Creates lxcXXXX veth interface                             │
│    │          - Attaches BPF program to lxcXXXX tc hook                   │
│    │          - Updates cilium_lxc BPF map (IP -> endpoint)                │
│    │          - Updates cilium_ipcache BPF map                             │
│    │     c. Plugin configures eth0 inside container netns                  │
│    │     d. Returns result: IP=10.244.1.5/32, gw=10.244.1.1                │
│    │                                                                       │
│    ├─3. runc: setns(/var/run/netns/<id>)                                   │
│    │          exec /pause (holds netns alive)                              │
│    │                                                                       │
│    └─4. runc for app containers: setns(same netns)                         │
│              exec /app (shares eth0 and 10.244.1.5)                        │
│                                                                            │
│  Kernel Networking State:                                                  │
│                                                                            │
│  HOST NAMESPACE:                                                           │
│    eth0     192.168.1.10/24    <- physical NIC                             │
│    lxc3a7b  (no IP, veth)      <- host side of pod-A veth                 │
│    lxc4b8c  (no IP, veth)      <- host side of pod-B veth                 │
│    cilium_vxlan (if tunnel)    <- VXLAN device                             │
│                                                                            │
│    BPF Programs:                                                           │
│      lxc3a7b tc ingress: policy check, LB, tunnel                         │
│      lxc3a7b tc egress:  policy check, identity lookup                    │
│      eth0    XDP:         fast drop/redirect (DDoS mitigation)             │
│                                                                            │
│    BPF Maps:                                                               │
│      cilium_lxc:       10.244.1.5 -> lxc3a7b, endpoint-id=100             │
│      cilium_policy:    ep-100 -> allowed: [ep-200, ns=frontend]            │
│      cilium_lb4_svc:   10.96.0.1:80 -> [10.244.1.5:8080, 10.244.2.3:8080] │
│                                                                            │
│  CONTAINER NAMESPACE (/var/run/netns/<id>):                                │
│    lo       127.0.0.1/8                                                    │
│    eth0     10.244.1.5/32                                                  │
│    routes:  default via 10.244.1.1, 10.244.1.1 dev eth0 scope link        │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘

Packet Flow: pod-A (10.244.1.5) -> Service ClusterIP (10.96.0.1:80)

1. pod-A writes to socket: dst=10.96.0.1:80
2. TCP SYN exits eth0 (in container netns)
3. Enters lxc3a7b (host side veth)
4. BPF tc ingress on lxc3a7b executes:
   a. Lookup 10.96.0.1:80 in cilium_lb4_svc  -> backend: 10.244.2.3:8080
   b. DNAT: rewrite dst IP:port
   c. Record in cilium_ct4_global (connection tracking)
   d. Lookup 10.244.2.3 in cilium_ipcache    -> tunnel_endpoint=192.168.1.20
   e. Encapsulate with GENEVE: outer dst=192.168.1.20
   f. bpf_redirect(eth0) -> packet exits node
5. Node-2 receives GENEVE packet, BPF on eth0 decapsulates
6. Delivers to pod-B via lxc4b8c
7. Return packets: BPF checks cilium_ct4_global, reverse NAT

No iptables. No kernel routing table lookups in fast path.
Pure BPF map lookups at O(1).
```

---

## Mental Model Summary

```
ABSTRACTION LAYERS:

Layer 5: Kubernetes (Pod, Service, NetworkPolicy)
          - Declares intent ("this pod needs IP connectivity")
          - No networking code itself

Layer 4: Container Runtime (containerd, CRI-O)
          - Creates network namespaces
          - Invokes CNI plugins
          - Manages pause container lifecycle

Layer 3: CNI Specification
          - exec-based RPC: environment vars + stdin JSON + stdout JSON
          - ADD/DEL/CHECK/VERSION commands
          - Plugin chaining (conflist)
          - IPAM sub-plugin protocol

Layer 2: CNI Plugins (bridge, Flannel, Calico, Cilium)
          - Translate CNI config into kernel operations
          - Use Netlink, iptables, tc, eBPF to program kernel

Layer 1: Linux Kernel Primitives
          - Network namespaces (struct net)
          - Virtual devices (veth, bridge, vxlan, geneve)
          - Routing (FIB, policy routing)
          - Netfilter (Netfilter hooks, conntrack)
          - eBPF (verifier, JIT, maps, hooks)
          - Netlink (rtnetlink, NETLINK_ROUTE)

THE KEY INSIGHT:
  CNI is not a networking system. CNI is a protocol for orchestrating
  the Linux kernel's networking system on behalf of container runtimes.
  Every byte of actual data plane work happens in the kernel.
  CNI is the control plane glue between Kubernetes and the kernel.

THE PERFORMANCE HIERARCHY:
  Userspace (app) -> Socket -> Kernel TCP/IP stack -> Netfilter
  -> Traffic Control -> eBPF (tc/XDP) -> Network Driver -> Hardware

  eBPF at TC:  ~500ns overhead per packet
  iptables:    ~5µs overhead for large rule sets (O(n) chain traversal)
  Kernel route: ~1µs FIB lookup
  XDP:         ~100ns (before sk_buff allocation)
  DPDK (bypass): ~50ns (bypass kernel entirely, only for special use cases)

UNDERSTANDING = ASKING: "AT WHICH LAYER DOES THIS HAPPEN?"
  Service VIP resolution?     -> kube-proxy (iptables) or Cilium (BPF map)
  Pod IP allocation?          -> CNI IPAM plugin (host-local file / etcd)
  Pod isolation?              -> Linux network namespace
  Cross-pod routing?          -> Kernel FIB + CNI-programmed routes OR BPF redirect
  Network policy enforcement? -> CNI plugin iptables/nftables/BPF
  Packet forwarding?          -> Linux bridge / kernel router / BPF
```
