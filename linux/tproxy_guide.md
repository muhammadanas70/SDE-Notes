# TPROXY: The Complete In-Depth Guide
## Transparent Proxying in Linux — Kernel, Userspace, C, Rust, Go, and Cloud

---

# Table of Contents

1. [What Is a Transparent Proxy?](#1-what-is-a-transparent-proxy)
2. [History and Motivation](#2-history-and-motivation)
3. [Linux Networking Fundamentals](#3-linux-networking-fundamentals)
   - 3.1 Network Stack Overview
   - 3.2 Netfilter Framework
   - 3.3 Hooks and Priority
   - 3.4 Routing Subsystem
   - 3.5 Policy Routing and Rules
   - 3.6 Traffic Control (tc)
4. [How TPROXY Works — Core Mechanics](#4-how-tproxy-works--core-mechanics)
   - 4.1 The Problem TPROXY Solves
   - 4.2 IP_TRANSPARENT Socket Option
   - 4.3 The TPROXY Netfilter Target
   - 4.4 Mark-Based Routing
   - 4.5 The Full Packet Journey
5. [Socket and Kernel Internals](#5-socket-and-kernel-internals)
   - 5.1 Socket Lookup Mechanism
   - 5.2 sk_assign and skb->sk
   - 5.3 CAP_NET_ADMIN Requirement
   - 5.4 Kernel Source Walkthrough
6. [iptables TPROXY Configuration](#6-iptables-tproxy-configuration)
   - 6.1 Required Kernel Modules
   - 6.2 Mangle Table and PREROUTING
   - 6.3 Mark and Routing Rules
   - 6.4 Full iptables Setup
   - 6.5 Handling Both IPv4 and IPv6
7. [nftables TPROXY Configuration](#7-nftables-tproxy-configuration)
   - 7.1 nftables vs iptables
   - 7.2 nftables tproxy Syntax
   - 7.3 Full nftables Ruleset
8. [TCP vs UDP TPROXY](#8-tcp-vs-udp-tproxy)
   - 8.1 TCP TPROXY
   - 8.2 UDP TPROXY — Stateless Challenges
   - 8.3 UDP with SO_REUSEPORT
9. [Implementation in C](#9-implementation-in-c)
   - 9.1 Minimal TCP TPROXY Server
   - 9.2 Retrieving Original Destination
   - 9.3 Full TCP + UDP C Implementation
   - 9.4 Getting Original Destination via getsockname()
10. [Implementation in Rust](#10-implementation-in-rust)
    - 10.1 Rust Async with tokio
    - 10.2 Setting IP_TRANSPARENT in Rust
    - 10.3 Full Rust TCP TPROXY
    - 10.4 Full Rust UDP TPROXY
11. [Implementation in Go](#11-implementation-in-go)
    - 11.1 Go syscall Interface
    - 11.2 TCP Transparent Proxy in Go
    - 11.3 UDP Transparent Proxy in Go
    - 11.4 Getting Original Destination in Go
12. [TPROXY for TLS/HTTPS Interception](#12-tproxy-for-tlshttps-interception)
    - 12.1 SNI-Based Routing
    - 12.2 Full TLS Termination
    - 12.3 Certificate Pinning Challenges
13. [Architecture Patterns](#13-architecture-patterns)
    - 13.1 Inline Bump-in-the-Wire
    - 13.2 Policy-Based Routing with TPROXY
    - 13.3 Kubernetes / Container Network TPROXY
    - 13.4 Service Mesh Sidecar Pattern
14. [Cloud Deployments](#14-cloud-deployments)
    - 14.1 AWS — VPC Traffic Mirroring and Gateway Load Balancer
    - 14.2 GCP — Transparent Proxy via Packet Mirroring
    - 14.3 Azure — NVA (Network Virtual Appliance) with TPROXY
    - 14.4 Cloud-Native Constraints and Workarounds
15. [Performance Tuning](#15-performance-tuning)
    - 15.1 Zero-Copy Techniques
    - 15.2 SO_REUSEPORT and CPU Affinity
    - 15.3 eBPF-Accelerated TPROXY
    - 15.4 NUMA and IRQ Affinity
16. [Debugging and Troubleshooting](#16-debugging-and-troubleshooting)
    - 16.1 conntrack and TPROXY
    - 16.2 tcpdump and Packet Capture
    - 16.3 strace and Socket Inspection
    - 16.4 Common Pitfalls
17. [Security Considerations](#17-security-considerations)
18. [Real-World Projects Using TPROXY](#18-real-world-projects-using-tproxy)
19. [Complete Reference Cheatsheet](#19-complete-reference-cheatsheet)

---

# 1. What Is a Transparent Proxy?

A **proxy** is a process that sits between a client and a server, receiving traffic on the client's behalf and forwarding it toward the destination. In a traditional (explicit) proxy, the client is **aware** of the proxy — it is configured to send traffic to a specific proxy IP and port (e.g., HTTP `CONNECT` or SOCKS5).

A **transparent proxy** intercepts traffic **without the client's knowledge or configuration**. The client sends traffic toward its intended destination (say, `93.184.216.34:443`). The proxy intercepts that packet, accepts it on a local socket, and then either inspects, modifies, or forwards it. From the client's perspective, nothing unusual happened — it was just talking to the destination server.

```
  EXPLICIT PROXY                        TRANSPARENT PROXY
  ─────────────────────────────         ─────────────────────────────────────
  Client knows about proxy              Client knows NOTHING about proxy

  Client ──► Proxy:8080 ──► Server      Client ──► [invisible interception] ──► Server
              │                                          │
              │ configured in browser/OS                 │ kernel redirects the packet
```

The kernel-level mechanism that makes this possible on Linux is **TPROXY** — a Netfilter target that assigns an incoming packet to a local socket *without changing the destination IP address in the packet headers*. This is the distinguishing characteristic: the proxy socket can call `getsockname()` on the accepted connection and receive the **original destination** (e.g., `93.184.216.34:443`), not the proxy's own address.

### Key Properties of TPROXY

| Property | Explicit Proxy | TPROXY / Transparent Proxy |
|---|---|---|
| Client configuration needed | Yes | No |
| Destination IP preserved | No (proxy is destination) | Yes (original IP visible to proxy) |
| Works for all protocols | No (protocol-specific) | Yes (TCP, UDP, even ICMP) |
| Kernel support required | No | Yes (Netfilter TPROXY target) |
| Routing rule required | No | Yes (policy routing) |
| Privileged socket needed | No | Yes (`CAP_NET_ADMIN` + `IP_TRANSPARENT`) |

---

# 2. History and Motivation

## 2.1 Why TPROXY Was Needed

Before TPROXY, transparent proxying was done with a hack: `iptables REDIRECT`. The REDIRECT target rewrites the destination IP of a packet to `127.0.0.1` (the loopback address) and the destination port to a local port. While this works for simple use cases, it **destroys the original destination information** — the accepting socket only sees `127.0.0.1:PORT`, not the original destination. A workaround was `SO_ORIGINAL_DST` — a socket option to query `conntrack` for the pre-NAT destination. But this only works with `conntrack` (connection tracking) and only for TCP.

```
  REDIRECT approach (old, lossy):
  ──────────────────────────────────────────────────────
  Packet: src=10.0.0.1:54321  dst=93.184.216.34:443
                │
                ▼ iptables REDIRECT --to-port 8080
  Packet: src=10.0.0.1:54321  dst=127.0.0.1:8080
                │
                ▼ proxy calls accept()
  Proxy sees: peer=10.0.0.1:54321, local=127.0.0.1:8080
              (original destination LOST — must query conntrack)

  TPROXY approach (modern, lossless):
  ──────────────────────────────────────────────────────
  Packet: src=10.0.0.1:54321  dst=93.184.216.34:443
                │
                ▼ iptables TPROXY --on-port 8080
  Packet: src=10.0.0.1:54321  dst=93.184.216.34:443
                │                (HEADERS UNCHANGED)
                ▼ proxy calls accept()
  Proxy sees: peer=10.0.0.1:54321, local=93.184.216.34:443
              (original destination PRESERVED natively)
```

## 2.2 Origins in the Kernel

TPROXY was developed by Balázs Scheidler and Krisztián Kovács and was merged into the Linux kernel in **version 2.6.28** (released December 2008). It was originally developed for Balabit's Zorp firewall (an enterprise-grade application-layer gateway). The implementation lives in:

```
net/netfilter/xt_TPROXY.c        — netfilter target module
include/linux/netfilter/xt_TPROXY.h — uAPI header
net/ipv4/netfilter/nf_tproxy_ipv4.c — IPv4 helpers
net/ipv6/netfilter/nf_tproxy_ipv6.c — IPv6 helpers
include/net/netfilter/nf_tproxy.h  — common helpers
```

## 2.3 Common Use Cases

- **Web filtering / parental control**: Intercept all HTTP/HTTPS without browser configuration
- **DLP (Data Loss Prevention)**: Inspect outbound content at the network layer
- **Security appliances / NGFW**: Deep Packet Inspection on all traffic
- **Service meshes** (e.g., Istio/Envoy): Intercept pod traffic in Kubernetes without application changes
- **Corporate proxies**: Force all traffic through an inspection proxy
- **CDN / load balancers**: Preserve original client IP while terminating connections locally
- **Protocol translation gateways**: Transparently rewrite or upgrade protocols

---

# 3. Linux Networking Fundamentals

Understanding TPROXY requires a solid mental model of how the Linux network stack processes packets.

## 3.1 Network Stack Overview

```
  NIC Hardware
      │
      ▼
  ┌──────────────────────────────────────┐
  │  Driver / Ring Buffer                │
  └──────────────────┬───────────────────┘
                     │  sk_buff (skb) allocated
                     ▼
  ┌──────────────────────────────────────┐
  │  L2 — Ethernet / Link Layer          │
  │  (MAC address processing,            │
  │   VLAN tagging, bridging)            │
  └──────────────────┬───────────────────┘
                     │
                     ▼
  ┌──────────────────────────────────────┐
  │  Netfilter PREROUTING hook           │  ◄── TPROXY fires here
  └──────────────────┬───────────────────┘
                     │
                     ▼
  ┌──────────────────────────────────────┐
  │  L3 Routing Decision                 │
  │  (is dst local? forward? drop?)      │
  └──────┬──────────────────┬────────────┘
         │ for local         │ for forwarded
         ▼                   ▼
  ┌──────────────┐   ┌───────────────────┐
  │  LOCAL_IN    │   │  FORWARD hook     │
  │  hook        │   └───────────────────┘
  └──────┬───────┘
         │
         ▼
  ┌──────────────────────────────────────┐
  │  L4 Transport Layer                  │
  │  TCP/UDP demultiplexing              │
  │  Socket lookup by (src,dst,proto)    │
  └──────────────────┬───────────────────┘
                     │
                     ▼
  ┌──────────────────────────────────────┐
  │  Socket Receive Buffer               │
  │  Application calls recv() / accept() │
  └──────────────────────────────────────┘
```

The kernel represents every packet as an `sk_buff` (`skb`) — a structure containing the packet data plus metadata (timestamps, marks, netfilter decisions, etc.).

## 3.2 Netfilter Framework

Netfilter is the packet-filtering framework built into the Linux kernel. It provides **hooks** at strategic points in the network stack where kernel modules can register callbacks (called **targets** in iptables terminology) to inspect, modify, accept, or drop packets.

```c
/* Simplified — actual registration is via nf_register_net_hook() */
struct nf_hook_ops {
    nf_hookfn       *hook;       /* callback function */
    struct net_device *dev;
    u_int8_t        pf;          /* protocol family: NFPROTO_IPV4, etc. */
    unsigned int    hooknum;     /* which hook point */
    int             priority;    /* execution order among registered hooks */
};
```

### Hook Points (IPv4)

```
  Packet IN ──► [PREROUTING] ──► routing ──► [FORWARD] ──► [POSTROUTING] ──► OUT
                                    │
                                    └──► [LOCAL_IN] ──► local socket
                                    
  Packet OUT ◄──────────────────────────────────────────────────────────────────
  from socket ──► [LOCAL_OUT] ──► routing ──► [POSTROUTING] ──► NIC
```

| Hook | When It Fires |
|---|---|
| `NF_INET_PRE_ROUTING` | Before routing decision, on ingress |
| `NF_INET_LOCAL_IN` | After routing, for packets destined to local system |
| `NF_INET_FORWARD` | For packets being forwarded (not local) |
| `NF_INET_LOCAL_OUT` | Locally generated packets, before routing |
| `NF_INET_POST_ROUTING` | After routing, just before going out on the wire |

**TPROXY fires at `NF_INET_PRE_ROUTING`** — before the routing decision. This is critical because TPROXY needs to *influence* the routing decision that follows.

## 3.3 The mangle Table

iptables organizes rules into **tables** based on purpose:

| Table | Purpose |
|---|---|
| `raw` | Before conntrack; earliest hook |
| `mangle` | Modify packet fields (TTL, TOS, MARK) |
| `nat` | Address translation (DNAT, SNAT, MASQUERADE) |
| `filter` | Allow/block decisions |
| `security` | SELinux/seccomp marks |

TPROXY rules go in the **`mangle` table**, `PREROUTING` chain. The `nat` table would run conntrack and could interfere with TPROXY's operation. Mangle runs before nat in PREROUTING.

## 3.4 Routing Subsystem

Linux uses a hierarchical routing system:

```
                   ┌─────────────────────────────┐
                   │    Policy Routing Rules       │
                   │  (ip rule list)               │
                   │                               │
                   │  priority 0:  from all lookup local   │
                   │  priority 32766: from all lookup main │
                   │  priority 32767: from all lookup default│
                   └──────────────┬──────────────┘
                                  │ matches rule
                                  ▼
                   ┌─────────────────────────────┐
                   │    Routing Table             │
                   │  (ip route show table N)     │
                   │                              │
                   │  default via 10.0.0.1        │
                   │  10.0.0.0/8 dev eth0         │
                   │  local ...                   │
                   └─────────────────────────────┘
```

### Policy Routing (ip rule)

Policy routing allows routing decisions based on more than just the destination IP. Rules can match:

- Source address (`from`)
- Destination address (`to`)
- Incoming interface (`iif`)
- **Firewall mark** (`fwmark`) ← this is what TPROXY uses

When a packet's `fwmark` (a 32-bit field in `skb->mark`) matches an `ip rule`, Linux consults the specified routing table for that packet.

```
ip rule add fwmark 1 lookup 100
# "Packets marked with fwmark=1 → look up routing table 100"

ip route add local 0.0.0.0/0 dev lo table 100
# "In table 100: route ALL destinations as local (loopback)"
```

This is the magic that makes TPROXY work: packets destined for external IPs get marked and then routed as if they're destined for localhost — so the local proxy socket receives them.

## 3.5 Connection Tracking (conntrack)

Netfilter's conntrack module tracks the state of network connections. Each packet belonging to a tracked connection has a `nf_conn` structure attached. TPROXY must interact carefully with conntrack:

```
  States conntrack tracks:
  ─────────────────────────
  NEW       — first packet of a new connection
  ESTABLISHED — reply packets seen
  RELATED   — related connection (e.g., FTP data)
  INVALID   — doesn't match any known connection
```

**Important:** When using TPROXY, you typically want to **bypass conntrack** for intercepted traffic, or ensure conntrack marks are applied correctly. If conntrack NATs the packet before TPROXY sees it, the original destination is lost. That's why TPROXY rules are in `mangle` (which runs before `nat`) and often paired with conntrack bypass rules in the `raw` table.

---

# 4. How TPROXY Works — Core Mechanics

## 4.1 The Problem TPROXY Solves

When a packet arrives at a Linux box destined for `93.184.216.34:443`:

1. The kernel checks: is `93.184.216.34` a local address? No.
2. The kernel routes it toward the gateway.
3. No local socket ever sees it.

For a transparent proxy to intercept this, we need to:

1. Intercept the packet at PREROUTING.
2. Assign it to a local socket **without changing the destination address**.
3. Route the packet to be handled locally (not forwarded).
4. Have the proxy socket bind to a non-local address (or accept the connection on that socket).

The TPROXY target and `IP_TRANSPARENT` socket option together solve all four requirements.

## 4.2 IP_TRANSPARENT Socket Option

`IP_TRANSPARENT` is a socket option (`SOL_IP`, option `19`) that allows a socket to:

1. **Bind to any IP address**, even non-local ones (normally, binding to a non-local address fails with `EADDRNOTAVAIL`)
2. **Receive packets** originally destined for non-local addresses

This requires `CAP_NET_ADMIN` capability (root or `CAP_NET_ADMIN` in the process's capability set).

```c
int sockfd = socket(AF_INET, SOCK_STREAM, 0);
int val = 1;
setsockopt(sockfd, SOL_IP, IP_TRANSPARENT, &val, sizeof(val));
// Now this socket can bind to 93.184.216.34 even though that IP
// is not assigned to any local interface
```

When a packet is delivered to a socket with `IP_TRANSPARENT`, calling `getsockname()` on the accepted connection returns the **original destination** address and port — exactly what the proxy needs to know where to forward the traffic.

## 4.3 The TPROXY Netfilter Target

The TPROXY target is a Netfilter extension in the `mangle` table. When applied to a packet, it:

1. **Looks up** a listening socket on the local machine at the specified port (or at the original destination port).
2. **Assigns** the packet to that socket by setting `skb->sk` (the `sk_buff`'s socket pointer).
3. Sets `skb->destructor` to `sock_pfree` so the socket reference is properly released.
4. **Does NOT rewrite** the packet headers — the destination IP:port remains unchanged.

```
iptables -t mangle -A PREROUTING \
    -p tcp --dport 80 \
    -j TPROXY --tproxy-mark 0x1/0x1 --on-port 8080
```

This says: "For TCP packets to port 80, assign them to the local socket listening on port 8080, and mark them with 0x1."

### TPROXY Target Parameters

| Parameter | Meaning |
|---|---|
| `--on-port PORT` | The local port to redirect to |
| `--on-ip IP` | The local IP to redirect to (optional) |
| `--tproxy-mark VALUE/MASK` | Firewall mark to apply to the packet |

## 4.4 Mark-Based Routing

After TPROXY assigns a socket to the packet, the routing decision still needs to happen. The packet's destination is still (e.g.) `93.184.216.34` — which is not a local address. Without additional configuration, the routing subsystem would try to forward it.

The **firewall mark** set by `--tproxy-mark` tells the routing subsystem: "Route this packet using a special routing table." That table has one entry: route everything via `lo` as a local destination.

```
  ┌─────────────────────────────────────────────────────────────┐
  │  ip rule add fwmark 1 lookup 100                           │
  │  # Packets with mark=1 → consult routing table 100         │
  └──────────────────────────────┬──────────────────────────────┘
                                 │
                                 ▼
  ┌─────────────────────────────────────────────────────────────┐
  │  ip route add local 0.0.0.0/0 dev lo table 100             │
  │  # Table 100: all destinations are "local"                  │
  │  # This causes the packet to be delivered locally          │
  └──────────────────────────────────────────────────────────────┘
```

The routing table entry `local 0.0.0.0/0 dev lo` tells the kernel: treat ALL destination addresses as local (loopback). This is the routing trick that causes the packet to be delivered to the TPROXY-assigned socket instead of being forwarded.

## 4.5 The Full Packet Journey

```
  CLIENT (10.0.0.5:54321) sends packet to SERVER (93.184.216.34:443)
  
  ┌─────────────────────────────────────────────────────────────────────┐
  │  STEP 1: Packet arrives at PROXY BOX NIC                           │
  │  skb: src=10.0.0.5:54321  dst=93.184.216.34:443                   │
  │  skb->mark = 0, skb->sk = NULL                                     │
  └──────────────────────────────┬──────────────────────────────────────┘
                                  │
                                  ▼
  ┌─────────────────────────────────────────────────────────────────────┐
  │  STEP 2: Netfilter PREROUTING / mangle table                       │
  │  Rule: -p tcp --dport 443 -j TPROXY --tproxy-mark 1 --on-port 8443│
  │                                                                     │
  │  TPROXY target executes:                                            │
  │  1. Finds local socket listening on *:8443 with IP_TRANSPARENT     │
  │  2. Sets skb->sk = &that_socket                                    │
  │  3. Sets skb->mark = 1                                             │
  │  Headers still: src=10.0.0.5:54321  dst=93.184.216.34:443         │
  └──────────────────────────────┬──────────────────────────────────────┘
                                  │
                                  ▼
  ┌─────────────────────────────────────────────────────────────────────┐
  │  STEP 3: Routing decision                                          │
  │  Kernel checks ip rules: mark=1 → use table 100                    │
  │  Table 100: local 0.0.0.0/0 dev lo                                 │
  │  Decision: DELIVER LOCALLY (as if dst was 127.0.0.1)               │
  └──────────────────────────────┬──────────────────────────────────────┘
                                  │
                                  ▼
  ┌─────────────────────────────────────────────────────────────────────┐
  │  STEP 4: Netfilter LOCAL_IN hook                                   │
  │  Packet is processed as a locally-destined packet                  │
  └──────────────────────────────┬──────────────────────────────────────┘
                                  │
                                  ▼
  ┌─────────────────────────────────────────────────────────────────────┐
  │  STEP 5: TCP layer / socket demultiplexing                         │
  │  Kernel delivers packet to socket already assigned (skb->sk)       │
  │  The listening socket accepts the connection                        │
  │  Proxy calls accept() → new fd                                     │
  │  Proxy calls getsockname() → returns 93.184.216.34:443            │
  │  (original destination, PRESERVED!)                                │
  │  Proxy calls getpeername() → returns 10.0.0.5:54321               │
  └──────────────────────────────────────────────────────────────────────┘
```

---

# 5. Socket and Kernel Internals

## 5.1 Socket Lookup Mechanism

Normally, when a TCP SYN arrives, the kernel's TCP layer runs a socket lookup (`__inet_lookup()`) to find which socket owns this connection:

```c
/* net/ipv4/tcp_ipv4.c — simplified */
struct sock *__inet_lookup(struct net *net,
                           struct inet_hashinfo *hashinfo,
                           __be32 saddr, __be16 sport,
                           __be32 daddr, __be16 dport,
                           int dif, int sdif)
{
    /* 1. Check established sockets (4-tuple match) */
    sk = __inet_lookup_established(net, hashinfo, saddr, sport, daddr, dport, dif, sdif);
    if (sk) return sk;
    
    /* 2. Check listening sockets (2-tuple: local addr+port) */
    return __inet_lookup_listener(net, hashinfo, skb, doff,
                                  saddr, sport, daddr, dport, dif, sdif);
}
```

For TPROXY, the lookup in step 2 would fail because no socket is listening on `93.184.216.34:443` locally. However, **when `skb->sk` is already set** (by the TPROXY target), the TCP layer skips the lookup and uses the pre-assigned socket directly.

```c
/* net/ipv4/tcp_ipv4.c — tcp_v4_rcv() */
sk = skb_steal_sock(skb, &refcounted);
if (!sk) {
    /* normal lookup */
    sk = __inet_lookup_skb(/* ... */);
}
/* if sk was set by TPROXY, we go straight here: */
if (sk->sk_state == TCP_LISTEN) {
    /* deliver to accept queue */
}
```

## 5.2 sk_assign and skb->sk

The kernel function that assigns a socket to a packet is `skb_steal_sock()` (and its counterpart `skb_orphan()`). In the TPROXY target code:

```c
/* net/netfilter/xt_TPROXY.c — simplified */
static unsigned int
tproxy_tg4(struct sk_buff *skb, const struct xt_action_param *par)
{
    const struct xt_tproxy_target_info_v1 *tgi = par->targinfo;
    /* ... */
    
    /* Find the local socket matching the tproxy port */
    sk = nf_tproxy_get_sock_v4(dev_net(par->state->in), skb,
                                iph->protocol,
                                iph->saddr, iph->daddr,
                                hp->source, tgi->lport ? tgi->lport : hp->dest,
                                par->state->in, NFT_LOOKUP_LISTENER);
    
    if (sk) {
        /* Assign the socket to the packet */
        skb->sk = sk;
        skb->destructor = sock_pfree;
        
        /* Apply the mark */
        skb->mark = (skb->mark & ~tgi->mark_mask) | tgi->mark_value;
        
        return NF_ACCEPT;
    }
    return NF_DROP; /* or NF_ACCEPT to pass through */
}
```

## 5.3 CAP_NET_ADMIN Requirement

The `IP_TRANSPARENT` option requires `CAP_NET_ADMIN`. This is enforced in `net/ipv4/ip_sockglue.c`:

```c
case IP_TRANSPARENT:
    if (!!val && !ns_capable(sock_net(sk)->user_ns, CAP_NET_ADMIN) &&
        !ns_capable(sock_net(sk)->user_ns, CAP_NET_RAW)) {
        err = -EPERM;
        break;
    }
    /* ... set sock flag SOCK_TRANSPARENT ... */
```

The flag `SOCK_TRANSPARENT` is set on the socket. This flag is later checked in:

- `inet_bind()` — allows binding to non-local IPs
- Socket lookup — allows matching packets destined for non-local IPs

## 5.4 Kernel Source Walkthrough

Here is a breadcrumb trail through the TPROXY-related kernel source:

```
net/netfilter/xt_TPROXY.c
  tproxy_tg4()                        — IPv4 TPROXY target handler
  tproxy_tg6()                        — IPv6 TPROXY target handler
  nf_tproxy_get_sock_v4()             — find matching local socket

net/ipv4/netfilter/nf_tproxy_ipv4.c
  nf_tproxy_get_sock_v4()             — socket lookup for IPv4

include/net/netfilter/nf_tproxy.h
  nf_tproxy_laddr4()                  — get the listen address
  nf_tproxy_assign_sock()             — assign socket to skb

net/ipv4/inet_connection_sock.c
  inet_csk_get_port()                 — port binding with TRANSPARENT flag

net/ipv4/ip_sockglue.c
  ip_setsockopt() → case IP_TRANSPARENT  — set transparent flag

net/ipv4/tcp_ipv4.c
  tcp_v4_rcv()                        — packet reception, checks skb->sk

include/linux/skbuff.h
  skb_steal_sock()                    — extract pre-assigned socket from skb
```

---

# 6. iptables TPROXY Configuration

## 6.1 Required Kernel Modules

```bash
# Load required modules
modprobe xt_TPROXY          # The TPROXY netfilter target
modprobe xt_socket          # Socket matching (optional but common)
modprobe xt_mark            # Mark matching
modprobe nf_tproxy_core     # Core TPROXY helpers
modprobe nf_tproxy_ipv4     # IPv4 TPROXY helpers
modprobe nf_tproxy_ipv6     # IPv6 TPROXY helpers (if needed)
modprobe xt_conntrack       # Connection tracking match

# Verify they are loaded
lsmod | grep -E 'TPROXY|tproxy|xt_socket'
```

## 6.2 The Routing Setup (Critical Step)

Before any iptables rules, set up policy routing:

```bash
# Mark-based routing: packets with mark 1 use routing table 100
ip rule add fwmark 1 lookup 100

# In routing table 100: all destinations are "local"
# This makes the kernel treat 93.184.216.34 as a local address
ip route add local 0.0.0.0/0 dev lo table 100

# For IPv6:
ip -6 rule add fwmark 1 lookup 100
ip -6 route add local ::/0 dev lo table 100
```

**Verify the routing rules:**

```bash
ip rule list
# 0:      from all lookup local
# 32764:  from all fwmark 0x1 lookup 100    ← our rule
# 32766:  from all lookup main
# 32767:  from all lookup default

ip route show table 100
# local 0.0.0.0/0 dev lo scope host          ← our route
```

## 6.3 Full iptables Setup — TCP

```bash
#!/bin/bash
# tproxy_setup.sh — Full TCP TPROXY setup

PROXY_PORT=8080
PROXY_MARK=1
PROXY_TABLE=100

# ── Routing setup ─────────────────────────────────────────────────────────────
ip rule add fwmark $PROXY_MARK lookup $PROXY_TABLE 2>/dev/null || true
ip route add local 0.0.0.0/0 dev lo table $PROXY_TABLE 2>/dev/null || true

# ── iptables TPROXY rules ──────────────────────────────────────────────────────

# Flush existing rules in mangle PREROUTING
iptables -t mangle -F PREROUTING

# Don't intercept traffic from the proxy itself (avoid loops!)
# The proxy process uid is 'proxy' in this example
iptables -t mangle -A PREROUTING -m owner --uid-owner proxy -j RETURN

# Don't intercept traffic already marked (prevents loops via lo)
iptables -t mangle -A PREROUTING -m mark --mark $PROXY_MARK -j RETURN

# Intercept HTTP (port 80)
iptables -t mangle -A PREROUTING \
    -p tcp --dport 80 \
    -j TPROXY \
    --tproxy-mark $PROXY_MARK/0xffffffff \
    --on-port $PROXY_PORT

# Intercept HTTPS (port 443)
iptables -t mangle -A PREROUTING \
    -p tcp --dport 443 \
    -j TPROXY \
    --tproxy-mark $PROXY_MARK/0xffffffff \
    --on-port $PROXY_PORT

echo "TPROXY rules installed"
iptables -t mangle -L PREROUTING -v -n
```

## 6.4 Full iptables Setup — UDP

UDP is trickier because it's connectionless. The TPROXY target still works, but each datagram is intercepted independently:

```bash
# UDP TPROXY — intercept DNS (port 53)
iptables -t mangle -A PREROUTING \
    -p udp --dport 53 \
    -j TPROXY \
    --tproxy-mark 1/1 \
    --on-port 15353    # Local DNS proxy port
```

For UDP with TPROXY, the proxy socket must use `IP_TRANSPARENT` + `IP_RECVORIGDSTADDR` to receive the original destination per datagram via `recvmsg()` with `cmsg` (ancillary data).

## 6.5 Handling Both IPv4 and IPv6

```bash
# IPv4 rules (using iptables)
iptables -t mangle -A PREROUTING -p tcp --dport 443 \
    -j TPROXY --tproxy-mark 1 --on-port 8443

# IPv6 rules (using ip6tables)
ip6tables -t mangle -A PREROUTING -p tcp --dport 443 \
    -j TPROXY --tproxy-mark 1 --on-port 8443

# IPv6 routing
ip -6 rule add fwmark 1 lookup 100
ip -6 route add local ::/0 dev lo table 100
```

On the socket side, to accept both IPv4 and IPv6, use `AF_INET6` with `IPV6_V6ONLY=0`, or create two separate sockets.

## 6.6 Preventing Routing Loops

A common pitfall is the proxy's own outbound traffic getting intercepted again. Prevention strategies:

```bash
# Strategy 1: Skip by UID (if proxy runs as specific user)
iptables -t mangle -A PREROUTING \
    -m owner --uid-owner proxy_user -j RETURN

# Strategy 2: Skip by outgoing interface (proxy connects via eth1, intercept on eth0)
iptables -t mangle -A PREROUTING -i lo -j RETURN
iptables -t mangle -A PREROUTING -i eth1 -j RETURN

# Strategy 3: Mark outbound proxy traffic and skip marked packets
# (proxy sets SO_MARK on its outbound sockets)
iptables -t mangle -A PREROUTING \
    -m mark --mark 2 -j RETURN   # mark 2 = proxy's outbound
```

---

# 7. nftables TPROXY Configuration

## 7.1 nftables vs iptables

nftables is the successor to iptables, integrated into the kernel since 4.0 and the default in most modern distributions (Debian 10+, RHEL 8+). nftables provides a unified framework for IPv4, IPv6, and ARP rules in a single ruleset, with better performance (single rule traversal vs. separate IPv4/IPv6 tables) and a cleaner syntax.

```
  iptables                    nftables
  ──────────────────────      ──────────────────────────────
  iptables / ip6tables        nft (single command)
  separate table per proto    families: ip, ip6, inet, arp, bridge
  chain = fixed (PREROUTING)  chain = user-defined with hook
  target = TPROXY             statement = tproxy
  rule format verbose         rule format compact
```

## 7.2 nftables tproxy Syntax

```
nft add rule [family] [table] [chain] [match] tproxy to [addr:]port [meta mark set value]
```

The `tproxy` statement accepts:

- `to PORT` — redirect to local port, keep original IP
- `to ADDR:PORT` — redirect to specific address:port
- `to :PORT` — keep original IP, redirect to port

## 7.3 Full nftables Ruleset

```nft
#!/usr/sbin/nft -f
# /etc/nftables-tproxy.conf

flush ruleset

# ── Transparent proxy table ───────────────────────────────────────────────────
table inet tproxy_table {

    # Chain for TCP proxy decisions
    chain prerouting {
        type filter hook prerouting priority mangle; policy accept;

        # Skip loopback traffic (prevents loops)
        iif lo accept

        # Skip already-marked traffic
        meta mark 1 accept

        # TCP TPROXY: intercept HTTP and HTTPS
        ip  protocol tcp tcp dport { 80, 443 } \
            tproxy ip to 127.0.0.1:8080 \
            meta mark set 1

        ip6 nexthdr tcp tcp dport { 80, 443 } \
            tproxy ip6 to [::1]:8080 \
            meta mark set 1

        # UDP TPROXY: intercept DNS
        ip  protocol udp udp dport 53 \
            tproxy ip to 127.0.0.1:15353 \
            meta mark set 1

        ip6 nexthdr udp udp dport 53 \
            tproxy ip6 to [::1]:15353 \
            meta mark set 1
    }
}
```

Load with:

```bash
nft -f /etc/nftables-tproxy.conf

# Verify
nft list ruleset
nft list table inet tproxy_table
```

### nftables: Using inet family for dual-stack

The `inet` family processes both IPv4 and IPv6 in one chain. However, the `tproxy` statement requires specifying the address family (`ip` or `ip6`) for the target address:

```nft
chain prerouting {
    type filter hook prerouting priority mangle;
    
    # This single rule handles both IPv4 and IPv6 TCP:443
    meta l4proto tcp th dport 443 tproxy to :8443 meta mark set 1
    # "tproxy to :8443" without ip/ip6 uses the packet's own family
}
```

---

# 8. TCP vs UDP TPROXY

## 8.1 TCP TPROXY

TCP TPROXY is straightforward because TCP is connection-oriented:

1. The SYN packet arrives, TPROXY assigns it to the listening socket.
2. The listening socket's accept queue receives the connection.
3. `accept()` returns a new socket for the connection.
4. `getsockname()` on the new socket returns the **original destination** (`93.184.216.34:443`).
5. `getpeername()` returns the client's address.

```
  Listening socket: bound to 0.0.0.0:8080 with IP_TRANSPARENT
                    │
         accept()   │
                    ▼
  Accepted socket:  local  = 93.184.216.34:443  (original dst)
                    remote = 10.0.0.5:54321      (client src)
```

## 8.2 UDP TPROXY — Stateless Challenges

UDP has no connections, so every datagram is independent. The TPROXY target assigns each UDP datagram to a socket, but the concept of "accepting a connection" doesn't exist. Instead:

- The proxy creates a `SOCK_DGRAM` socket with `IP_TRANSPARENT`.
- It binds to `0.0.0.0:PORT` (or a specific port).
- It uses `recvmsg()` with `IP_RECVORIGDSTADDR` to receive the original destination per datagram as ancillary (cmsg) data.

```c
/* Enable receiving original destination */
int on = 1;
setsockopt(fd, SOL_IP, IP_RECVORIGDSTADDR, &on, sizeof(on));

/* Receive a datagram */
char buf[4096];
struct iovec iov = { buf, sizeof(buf) };
char cmsg_buf[CMSG_SPACE(sizeof(struct sockaddr_in))];
struct msghdr msg = {
    .msg_iov = &iov, .msg_iovlen = 1,
    .msg_control = cmsg_buf, .msg_controllen = sizeof(cmsg_buf),
};
recvmsg(fd, &msg, 0);

/* Extract original destination from cmsg */
for (struct cmsghdr *cm = CMSG_FIRSTHDR(&msg); cm;
     cm = CMSG_NXTHDR(&msg, cm)) {
    if (cm->cmsg_level == SOL_IP &&
        cm->cmsg_type  == IP_ORIGDSTADDR) {
        struct sockaddr_in *orig = (struct sockaddr_in *)CMSG_DATA(cm);
        /* orig->sin_addr = original destination IP */
        /* orig->sin_port = original destination port */
    }
}
```

## 8.3 UDP Reply Path

For UDP TPROXY, replying to the client is also non-trivial. The reply must appear to come from the original destination IP. This requires:

1. Creating a new UDP socket with `IP_TRANSPARENT`.
2. Binding it to the **original destination IP:port** (allowed because of `IP_TRANSPARENT`).
3. `sendto()` targeting the client.

```c
/* Create reply socket */
int reply_fd = socket(AF_INET, SOCK_DGRAM, 0);
int on = 1;
setsockopt(reply_fd, SOL_IP, IP_TRANSPARENT, &on, sizeof(on));

/* Bind to original destination (non-local IP is OK with IP_TRANSPARENT) */
struct sockaddr_in orig_dst = { /* 93.184.216.34:53 */ };
bind(reply_fd, (struct sockaddr *)&orig_dst, sizeof(orig_dst));

/* Send reply to client — appears to come from 93.184.216.34:53 */
sendto(reply_fd, reply_data, reply_len, 0,
       (struct sockaddr *)&client_addr, sizeof(client_addr));
```

This works because:
- `IP_TRANSPARENT` allows binding to the non-local IP.
- The routing must allow this socket's outbound packets — they have src IP `93.184.216.34`, which the kernel must route out via the correct interface.

---

# 9. Implementation in C

## 9.1 Socket Setup Helper

```c
/* tproxy_socket.h */
#ifndef TPROXY_SOCKET_H
#define TPROXY_SOCKET_H

#include <sys/socket.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <arpa/inet.h>
#include <unistd.h>
#include <string.h>
#include <stdio.h>
#include <errno.h>

/*
 * create_tproxy_listener - create a TCP socket for TPROXY interception
 * @port: local port to listen on
 *
 * The socket is created with:
 *   - IP_TRANSPARENT: accept packets for non-local IPs
 *   - SO_REUSEADDR: fast port reuse after restart
 *   - SO_REUSEPORT: allow multiple processes/threads to share the port
 *
 * Returns: fd on success, -1 on error
 */
int create_tproxy_listener(uint16_t port)
{
    int fd = socket(AF_INET6, SOCK_STREAM, 0);
    if (fd < 0) { perror("socket"); return -1; }

    /* Allow dual-stack: accept both IPv4 and IPv6 */
    int off = 0;
    setsockopt(fd, IPPROTO_IPV6, IPV6_V6ONLY, &off, sizeof(off));

    int on = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &on, sizeof(on));
    setsockopt(fd, SOL_SOCKET, SO_REUSEPORT, &on, sizeof(on));

    /* THE critical option: allow binding to non-local addresses */
    if (setsockopt(fd, SOL_IP, IP_TRANSPARENT, &on, sizeof(on)) < 0) {
        perror("setsockopt IP_TRANSPARENT (need CAP_NET_ADMIN)");
        close(fd);
        return -1;
    }

    /* Set TCP_NODELAY for lower latency */
    setsockopt(fd, IPPROTO_TCP, TCP_NODELAY, &on, sizeof(on));

    struct sockaddr_in6 addr = {
        .sin6_family = AF_INET6,
        .sin6_addr   = in6addr_any,
        .sin6_port   = htons(port),
    };

    if (bind(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        perror("bind"); close(fd); return -1;
    }
    if (listen(fd, 4096) < 0) {
        perror("listen"); close(fd); return -1;
    }

    return fd;
}

/*
 * get_original_dst - retrieve original destination from accepted socket
 *
 * For IPv4 connections, this is just getsockname().
 * For IPv6, use IPV6_RECVORIGDSTADDR or getsockname() depending on setup.
 */
int get_original_dst(int client_fd,
                     struct sockaddr_storage *orig_dst,
                     socklen_t *orig_dst_len)
{
    *orig_dst_len = sizeof(*orig_dst);
    /* With TPROXY, getsockname() returns the original destination */
    return getsockname(client_fd, (struct sockaddr *)orig_dst, orig_dst_len);
}

#endif /* TPROXY_SOCKET_H */
```

## 9.2 Full TCP TPROXY Server in C

```c
/* tproxy_server.c
 *
 * Minimal transparent TCP proxy in C.
 * Intercepts traffic, logs original destination, and optionally
 * forwards to the real server.
 *
 * Compile: gcc -O2 -o tproxy_server tproxy_server.c -lpthread
 * Run:     sudo ./tproxy_server 8080
 */

#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>
#include <pthread.h>
#include <sys/socket.h>
#include <sys/epoll.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <arpa/inet.h>
#include <fcntl.h>
#include <signal.h>

#define BACKLOG        4096
#define MAX_EVENTS     1024
#define BUFFER_SIZE    65536

/* ── Utility ──────────────────────────────────────────────────────────────── */

static void sockaddr_to_str(const struct sockaddr_storage *ss,
                             char *buf, size_t len)
{
    if (ss->ss_family == AF_INET) {
        const struct sockaddr_in *s4 = (const struct sockaddr_in *)ss;
        char ip[INET_ADDRSTRLEN];
        inet_ntop(AF_INET, &s4->sin_addr, ip, sizeof(ip));
        snprintf(buf, len, "%s:%d", ip, ntohs(s4->sin_port));
    } else if (ss->ss_family == AF_INET6) {
        const struct sockaddr_in6 *s6 = (const struct sockaddr_in6 *)ss;
        char ip[INET6_ADDRSTRLEN];
        inet_ntop(AF_INET6, &s6->sin6_addr, ip, sizeof(ip));
        snprintf(buf, len, "[%s]:%d", ip, ntohs(s6->sin6_port));
    } else {
        snprintf(buf, len, "<unknown>");
    }
}

static int set_nonblocking(int fd)
{
    int flags = fcntl(fd, F_GETFL, 0);
    if (flags < 0) return -1;
    return fcntl(fd, F_SETFL, flags | O_NONBLOCK);
}

/* ── Proxy Connection Handler ─────────────────────────────────────────────── */

struct conn_ctx {
    int client_fd;
    struct sockaddr_storage client_addr;  /* where client connected FROM */
    struct sockaddr_storage orig_dst;     /* original destination (TPROXY magic) */
};

/*
 * connect_to_real_server - open a connection to the original destination
 *
 * With IP_TRANSPARENT on the outbound socket, we can set the source IP
 * to the client's IP so the server sees the real client (full transparency).
 * Without this, the server sees the proxy's IP as the source.
 */
static int connect_to_real_server(const struct sockaddr_storage *orig_dst,
                                   const struct sockaddr_storage *client_src)
{
    int fd = socket(orig_dst->ss_family, SOCK_STREAM, 0);
    if (fd < 0) { perror("socket (upstream)"); return -1; }

    int on = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &on, sizeof(on));
    setsockopt(fd, IPPROTO_TCP, TCP_NODELAY, &on, sizeof(on));

    /*
     * Optional: bind with IP_TRANSPARENT so the server sees the
     * original client IP. This requires that return traffic is
     * also routed through this proxy box.
     */
#ifdef TPROXY_FULL_TRANSPARENT
    if (setsockopt(fd, SOL_IP, IP_TRANSPARENT, &on, sizeof(on)) == 0) {
        /* Bind to client's source address — server sees real client IP */
        if (bind(fd, (struct sockaddr *)client_src,
                 client_src->ss_family == AF_INET
                     ? sizeof(struct sockaddr_in)
                     : sizeof(struct sockaddr_in6)) < 0) {
            /* Non-fatal: binding to a non-local client IP might fail */
            perror("bind (client IP spoofing, non-fatal)");
        }
    }
#else
    (void)client_src;
#endif

    socklen_t addrlen = orig_dst->ss_family == AF_INET
                        ? sizeof(struct sockaddr_in)
                        : sizeof(struct sockaddr_in6);

    if (connect(fd, (const struct sockaddr *)orig_dst, addrlen) < 0) {
        char dst_str[64];
        sockaddr_to_str(orig_dst, dst_str, sizeof(dst_str));
        fprintf(stderr, "connect to %s failed: %s\n", dst_str, strerror(errno));
        close(fd);
        return -1;
    }

    return fd;
}

/*
 * splice_loop - bidirectional data forwarding using splice(2)
 *
 * splice() moves data between file descriptors via the kernel's pipe
 * buffer — avoiding userspace copies (zero-copy between sockets).
 */
static void splice_loop(int fd_a, int fd_b)
{
    int pipes_ab[2], pipes_ba[2];

    if (pipe(pipes_ab) < 0 || pipe(pipes_ba) < 0) {
        perror("pipe");
        goto cleanup;
    }

    while (1) {
        /* Forward: client → server */
        ssize_t n = splice(fd_a, NULL, pipes_ab[1], NULL,
                           65536, SPLICE_F_MOVE | SPLICE_F_NONBLOCK);
        if (n < 0 && errno != EAGAIN) break;
        if (n > 0) {
            ssize_t m = splice(pipes_ab[0], NULL, fd_b, NULL,
                               n, SPLICE_F_MOVE);
            if (m < 0) break;
        }

        /* Forward: server → client */
        n = splice(fd_b, NULL, pipes_ba[1], NULL,
                   65536, SPLICE_F_MOVE | SPLICE_F_NONBLOCK);
        if (n < 0 && errno != EAGAIN) break;
        if (n > 0) {
            ssize_t m = splice(pipes_ba[0], NULL, fd_a, NULL,
                               n, SPLICE_F_MOVE);
            if (m < 0) break;
        }
    }

    close(pipes_ab[0]); close(pipes_ab[1]);
    close(pipes_ba[0]); close(pipes_ba[1]);
cleanup:
    return;
}

static void *handle_connection(void *arg)
{
    struct conn_ctx *ctx = (struct conn_ctx *)arg;

    char client_str[64], orig_dst_str[64];
    sockaddr_to_str(&ctx->client_addr, client_str, sizeof(client_str));
    sockaddr_to_str(&ctx->orig_dst,    orig_dst_str, sizeof(orig_dst_str));

    printf("[+] Connection: client=%s  original_dst=%s\n",
           client_str, orig_dst_str);

    /* Connect to the real server */
    int server_fd = connect_to_real_server(&ctx->orig_dst, &ctx->client_addr);
    if (server_fd < 0) {
        fprintf(stderr, "[-] Could not connect to original server\n");
        close(ctx->client_fd);
        free(ctx);
        return NULL;
    }

    /* Bidirectional forwarding (could inspect/modify here) */
    splice_loop(ctx->client_fd, server_fd);

    printf("[-] Connection closed: %s → %s\n", client_str, orig_dst_str);

    close(ctx->client_fd);
    close(server_fd);
    free(ctx);
    return NULL;
}

/* ── Main Accept Loop ─────────────────────────────────────────────────────── */

int main(int argc, char *argv[])
{
    if (argc < 2) {
        fprintf(stderr, "Usage: %s <port>\n", argv[0]);
        return 1;
    }

    uint16_t port = (uint16_t)atoi(argv[1]);

    /* Ignore SIGPIPE — handle connection errors via errno */
    signal(SIGPIPE, SIG_IGN);

    /* Create TPROXY listening socket */
    int listen_fd = socket(AF_INET6, SOCK_STREAM, 0);
    if (listen_fd < 0) { perror("socket"); return 1; }

    int on = 1, off = 0;
    setsockopt(listen_fd, IPPROTO_IPV6, IPV6_V6ONLY,  &off, sizeof(off));
    setsockopt(listen_fd, SOL_SOCKET,   SO_REUSEADDR,  &on,  sizeof(on));
    setsockopt(listen_fd, SOL_SOCKET,   SO_REUSEPORT,  &on,  sizeof(on));
    setsockopt(listen_fd, IPPROTO_TCP,  TCP_NODELAY,   &on,  sizeof(on));

    /* THE key option */
    if (setsockopt(listen_fd, SOL_IP, IP_TRANSPARENT, &on, sizeof(on)) < 0) {
        perror("IP_TRANSPARENT (run as root or with CAP_NET_ADMIN)");
        return 1;
    }

    struct sockaddr_in6 addr = {
        .sin6_family = AF_INET6,
        .sin6_addr   = in6addr_any,
        .sin6_port   = htons(port),
    };

    if (bind(listen_fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        perror("bind"); return 1;
    }
    if (listen(listen_fd, BACKLOG) < 0) {
        perror("listen"); return 1;
    }

    printf("[*] TPROXY listening on port %d\n", port);
    printf("[*] Waiting for intercepted connections...\n\n");

    while (1) {
        struct sockaddr_storage client_addr;
        socklen_t client_len = sizeof(client_addr);

        int client_fd = accept4(listen_fd,
                                (struct sockaddr *)&client_addr,
                                &client_len,
                                SOCK_CLOEXEC);
        if (client_fd < 0) {
            if (errno == EINTR || errno == EAGAIN) continue;
            perror("accept4");
            break;
        }

        /* Get the original destination (preserved by TPROXY) */
        struct conn_ctx *ctx = calloc(1, sizeof(*ctx));
        ctx->client_fd   = client_fd;
        ctx->client_addr = client_addr;

        socklen_t orig_len = sizeof(ctx->orig_dst);
        getsockname(client_fd, (struct sockaddr *)&ctx->orig_dst, &orig_len);
        /*
         * getsockname() on a TPROXY-accepted socket returns the
         * ORIGINAL destination IP:port, not the proxy's IP:port.
         * This is the core feature of TPROXY.
         */

        pthread_t tid;
        pthread_attr_t attr;
        pthread_attr_init(&attr);
        pthread_attr_setdetachstate(&attr, PTHREAD_CREATE_DETACHED);
        pthread_create(&tid, &attr, handle_connection, ctx);
        pthread_attr_destroy(&attr);
    }

    close(listen_fd);
    return 0;
}
```

## 9.3 UDP TPROXY in C

```c
/* udp_tproxy.c — UDP transparent proxy
 *
 * Compile: gcc -O2 -o udp_tproxy udp_tproxy.c
 * Run:     sudo ./udp_tproxy 15353
 */

#define _GNU_SOURCE
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>
#include <errno.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <netinet/ip.h>
#include <arpa/inet.h>

#define BUF_SIZE 65536

int create_udp_tproxy_socket(uint16_t port)
{
    int fd = socket(AF_INET, SOCK_DGRAM, 0);
    if (fd < 0) { perror("socket"); return -1; }

    int on = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &on, sizeof(on));
    setsockopt(fd, SOL_SOCKET, SO_REUSEPORT, &on, sizeof(on));

    /* IP_TRANSPARENT: allow receiving packets for non-local IPs */
    if (setsockopt(fd, SOL_IP, IP_TRANSPARENT, &on, sizeof(on)) < 0) {
        perror("IP_TRANSPARENT"); close(fd); return -1;
    }

    /* IP_RECVORIGDSTADDR: receive original dst in ancillary data */
    if (setsockopt(fd, SOL_IP, IP_RECVORIGDSTADDR, &on, sizeof(on)) < 0) {
        perror("IP_RECVORIGDSTADDR"); close(fd); return -1;
    }

    struct sockaddr_in addr = {
        .sin_family = AF_INET,
        .sin_addr.s_addr = INADDR_ANY,
        .sin_port = htons(port),
    };

    if (bind(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        perror("bind"); close(fd); return -1;
    }

    return fd;
}

int main(int argc, char *argv[])
{
    if (argc < 2) { fprintf(stderr, "Usage: %s <port>\n", argv[0]); return 1; }

    int fd = create_udp_tproxy_socket((uint16_t)atoi(argv[1]));
    if (fd < 0) return 1;

    printf("[*] UDP TPROXY listening on port %s\n", argv[1]);

    char buf[BUF_SIZE];
    char cmsg_buf[CMSG_SPACE(sizeof(struct sockaddr_in))];

    while (1) {
        struct sockaddr_in client_addr;
        struct iovec iov = { buf, sizeof(buf) };
        struct msghdr msg = {
            .msg_name       = &client_addr,
            .msg_namelen    = sizeof(client_addr),
            .msg_iov        = &iov,
            .msg_iovlen     = 1,
            .msg_control    = cmsg_buf,
            .msg_controllen = sizeof(cmsg_buf),
        };

        ssize_t n = recvmsg(fd, &msg, 0);
        if (n < 0) { perror("recvmsg"); continue; }

        /* Extract original destination from cmsg */
        struct sockaddr_in orig_dst = {0};
        for (struct cmsghdr *cm = CMSG_FIRSTHDR(&msg); cm;
             cm = CMSG_NXTHDR(&msg, cm)) {
            if (cm->cmsg_level == SOL_IP &&
                cm->cmsg_type  == IP_ORIGDSTADDR) {
                memcpy(&orig_dst, CMSG_DATA(cm), sizeof(orig_dst));
                break;
            }
        }

        char client_str[64], orig_dst_str[64];
        inet_ntop(AF_INET, &client_addr.sin_addr,
                  client_str, sizeof(client_str));
        inet_ntop(AF_INET, &orig_dst.sin_addr,
                  orig_dst_str, sizeof(orig_dst_str));

        printf("[UDP] %s:%d → %s:%d  (%zd bytes)\n",
               client_str, ntohs(client_addr.sin_port),
               orig_dst_str, ntohs(orig_dst.sin_port),
               n);

        /*
         * To forward to real server and send reply back:
         * 1. Create upstream UDP socket, sendto orig_dst
         * 2. Receive reply
         * 3. Create reply socket, bind to orig_dst with IP_TRANSPARENT
         * 4. sendto client_addr (appears to come from orig_dst)
         */

        /* For demonstration, just echo back from original destination */
        int reply_fd = socket(AF_INET, SOCK_DGRAM, 0);
        int tr = 1;
        setsockopt(reply_fd, SOL_IP, IP_TRANSPARENT, &tr, sizeof(tr));
        /* Bind to original destination — spoof source IP */
        bind(reply_fd, (struct sockaddr *)&orig_dst, sizeof(orig_dst));
        /* Send reply to client — client sees it as coming from orig_dst */
        sendto(reply_fd, buf, n, 0,
               (struct sockaddr *)&client_addr, sizeof(client_addr));
        close(reply_fd);
    }

    return 0;
}
```

---

# 10. Implementation in Rust

## 10.1 IP_TRANSPARENT in Rust

Rust's standard library doesn't expose `IP_TRANSPARENT` directly. We use `nix` or raw `libc` syscalls via `std::os::unix::io::AsRawFd`.

**Cargo.toml dependencies:**
```toml
[dependencies]
tokio = { version = "1", features = ["full"] }
nix   = { version = "0.29", features = ["net", "socket"] }
libc  = "0.2"
```

## 10.2 Setting IP_TRANSPARENT via libc

```rust
use std::os::unix::io::AsRawFd;
use libc::{setsockopt, SOL_IP, c_int};

/// IP_TRANSPARENT socket option (not in libc crate, define manually)
const IP_TRANSPARENT: c_int = 19;

pub fn set_ip_transparent(fd: std::os::unix::io::RawFd) -> std::io::Result<()> {
    let val: c_int = 1;
    let ret = unsafe {
        setsockopt(
            fd,
            SOL_IP,
            IP_TRANSPARENT,
            &val as *const c_int as *const libc::c_void,
            std::mem::size_of::<c_int>() as libc::socklen_t,
        )
    };
    if ret == 0 {
        Ok(())
    } else {
        Err(std::io::Error::last_os_error())
    }
}
```

## 10.3 Full Rust TCP TPROXY with Tokio

```rust
// src/main.rs
//
// Async transparent TCP proxy using Tokio.
// Intercepts connections, logs original destination,
// forwards traffic to the real server.
//
// Run: sudo ./tproxy_rust 8080

use std::net::SocketAddr;
use std::sync::Arc;
use tokio::net::{TcpListener, TcpStream};
use tokio::io::{AsyncReadExt, AsyncWriteExt, copy_bidirectional};
use tokio::time::{timeout, Duration};
use libc::c_int;

const IP_TRANSPARENT: c_int = 19;

/// Apply socket options required for TPROXY to a raw file descriptor.
fn configure_tproxy_socket(fd: std::os::unix::io::RawFd) -> std::io::Result<()> {
    let val: c_int = 1;
    let ret = unsafe {
        libc::setsockopt(
            fd,
            libc::SOL_IP,
            IP_TRANSPARENT,
            &val as *const _ as *const libc::c_void,
            std::mem::size_of::<c_int>() as libc::socklen_t,
        )
    };
    if ret != 0 {
        return Err(std::io::Error::last_os_error());
    }
    Ok(())
}

fn configure_so_reuseport(fd: std::os::unix::io::RawFd) -> std::io::Result<()> {
    let val: c_int = 1;
    let ret = unsafe {
        libc::setsockopt(
            fd,
            libc::SOL_SOCKET,
            libc::SO_REUSEPORT,
            &val as *const _ as *const libc::c_void,
            std::mem::size_of::<c_int>() as libc::socklen_t,
        )
    };
    if ret != 0 { Err(std::io::Error::last_os_error()) } else { Ok(()) }
}

/// Create a TCP listener socket configured for TPROXY interception.
fn create_tproxy_listener(addr: &str) -> std::io::Result<std::net::TcpListener> {
    use std::net::TcpListener;
    use std::os::unix::io::AsRawFd;

    // Use a raw std listener first so we can configure it before binding
    let socket = socket2::Socket::new(
        socket2::Domain::IPV6,
        socket2::Type::STREAM,
        None,
    )?;

    socket.set_only_v6(false)?;          // dual-stack
    socket.set_reuse_address(true)?;

    let fd = socket.as_raw_fd();
    configure_tproxy_socket(fd)?;
    configure_so_reuseport(fd)?;

    socket.set_nonblocking(true)?;

    let addr: std::net::SocketAddr = addr.parse().expect("invalid address");
    socket.bind(&addr.into())?;
    socket.listen(4096)?;

    Ok(socket.into())
}

/// Get the original destination address from a TPROXY-accepted connection.
/// With TPROXY, getsockname() returns the original destination.
fn get_original_dst(stream: &std::net::TcpStream) -> std::io::Result<SocketAddr> {
    use std::os::unix::io::AsRawFd;
    let fd = stream.as_raw_fd();

    let mut addr: libc::sockaddr_in6 = unsafe { std::mem::zeroed() };
    let mut len = std::mem::size_of::<libc::sockaddr_in6>() as libc::socklen_t;

    let ret = unsafe {
        libc::getsockname(
            fd,
            &mut addr as *mut _ as *mut libc::sockaddr,
            &mut len,
        )
    };

    if ret != 0 {
        return Err(std::io::Error::last_os_error());
    }

    // Convert to Rust SocketAddr
    let ip = std::net::Ipv6Addr::from(addr.sin6_addr.s6_addr);
    let port = u16::from_be(addr.sin6_port);

    let sock_addr = if let Some(v4) = ip.to_ipv4_mapped() {
        SocketAddr::new(std::net::IpAddr::V4(v4), port)
    } else {
        SocketAddr::new(std::net::IpAddr::V6(ip), port)
    };

    Ok(sock_addr)
}

/// Handle one intercepted client connection.
async fn handle_connection(
    client_stream: TcpStream,
    client_addr: SocketAddr,
    orig_dst: SocketAddr,
) {
    println!("[+] {} -> {} (intercepted)", client_addr, orig_dst);

    // Connect to the real server
    let server_stream = match timeout(
        Duration::from_secs(10),
        TcpStream::connect(orig_dst),
    ).await {
        Ok(Ok(s)) => s,
        Ok(Err(e)) => {
            eprintln!("[-] Failed to connect to {}: {}", orig_dst, e);
            return;
        }
        Err(_) => {
            eprintln!("[-] Timeout connecting to {}", orig_dst);
            return;
        }
    };

    // Bidirectional forwarding (zero-allocation, splice-like via tokio)
    let (mut cr, mut cw) = client_stream.into_split();
    let (mut sr, mut sw) = server_stream.into_split();

    let client_to_server = tokio::spawn(async move {
        let _ = tokio::io::copy(&mut cr, &mut sw).await;
        let _ = sw.shutdown().await;
    });

    let server_to_client = tokio::spawn(async move {
        let _ = tokio::io::copy(&mut sr, &mut cw).await;
        let _ = cw.shutdown().await;
    });

    let _ = tokio::join!(client_to_server, server_to_client);
    println!("[-] {} -> {} closed", client_addr, orig_dst);
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let port: u16 = std::env::args()
        .nth(1)
        .unwrap_or_else(|| "8080".to_string())
        .parse()
        .expect("invalid port");

    let bind_addr = format!(":::{}",  port);

    println!("[*] Creating TPROXY listener on {}", bind_addr);

    // Create the raw listener with IP_TRANSPARENT
    let std_listener = create_tproxy_listener(&bind_addr)?;
    // Convert to tokio listener
    let listener = TcpListener::from_std(std_listener)?;

    println!("[*] Listening. Waiting for intercepted connections...");

    loop {
        match listener.accept().await {
            Ok((stream, client_addr)) => {
                // Extract original destination BEFORE converting to tokio stream
                // We need the raw std stream for getsockname()
                let orig_dst = {
                    use std::os::unix::io::AsRawFd;
                    let fd = stream.as_raw_fd();
                    let mut addr: libc::sockaddr_in6 = unsafe { std::mem::zeroed() };
                    let mut len = std::mem::size_of::<libc::sockaddr_in6>()
                        as libc::socklen_t;
                    unsafe {
                        libc::getsockname(
                            fd,
                            &mut addr as *mut _ as *mut libc::sockaddr,
                            &mut len,
                        )
                    };
                    let ip = std::net::Ipv6Addr::from(addr.sin6_addr.s6_addr);
                    let port = u16::from_be(addr.sin6_port);
                    if let Some(v4) = ip.to_ipv4_mapped() {
                        SocketAddr::new(std::net::IpAddr::V4(v4), port)
                    } else {
                        SocketAddr::new(std::net::IpAddr::V6(ip), port)
                    }
                };

                tokio::spawn(handle_connection(stream, client_addr, orig_dst));
            }
            Err(e) => {
                eprintln!("accept error: {}", e);
            }
        }
    }
}
```

**Cargo.toml:**
```toml
[package]
name = "tproxy-rust"
version = "0.1.0"
edition = "2021"

[dependencies]
tokio   = { version = "1", features = ["full"] }
libc    = "0.2"
socket2 = "0.5"
```

## 10.4 Rust UDP TPROXY

```rust
// UDP TPROXY in Rust — receive datagrams with original destination

use libc::{
    recvmsg, msghdr, iovec, cmsghdr,
    sockaddr_in, CMSG_FIRSTHDR, CMSG_NXTHDR, CMSG_DATA,
    SOL_IP, IP_ORIGDSTADDR, AF_INET,
    c_void, c_int,
};
use std::os::unix::io::AsRawFd;
use tokio::net::UdpSocket;

const IP_TRANSPARENT: c_int   = 19;
const IP_RECVORIGDSTADDR: c_int = 20;

pub struct UdpTproxySocket {
    inner: UdpSocket,
}

impl UdpTproxySocket {
    pub async fn bind(port: u16) -> std::io::Result<Self> {
        let sock = socket2::Socket::new(
            socket2::Domain::IPV4,
            socket2::Type::DGRAM,
            None,
        )?;

        let val: c_int = 1;
        // IP_TRANSPARENT
        unsafe {
            libc::setsockopt(
                sock.as_raw_fd(), SOL_IP, IP_TRANSPARENT,
                &val as *const _ as *const c_void,
                std::mem::size_of::<c_int>() as libc::socklen_t,
            );
            // IP_RECVORIGDSTADDR: attach original dst to every recvmsg
            libc::setsockopt(
                sock.as_raw_fd(), SOL_IP, IP_RECVORIGDSTADDR,
                &val as *const _ as *const c_void,
                std::mem::size_of::<c_int>() as libc::socklen_t,
            );
        }

        sock.set_reuse_address(true)?;
        sock.set_nonblocking(true)?;
        sock.bind(&format!("0.0.0.0:{}", port).parse::<std::net::SocketAddr>()
            .unwrap().into())?;

        let std_sock: std::net::UdpSocket = sock.into();
        Ok(Self { inner: UdpSocket::from_std(std_sock)? })
    }

    /// Receive a datagram, returning (data, client_addr, original_dst)
    pub async fn recv_from_transparent(
        &self,
    ) -> std::io::Result<(Vec<u8>, std::net::SocketAddr, std::net::SocketAddr)>
    {
        // Wait for data to be available (async)
        self.inner.readable().await?;

        let fd = self.inner.as_raw_fd();
        let mut buf = vec![0u8; 65536];
        let mut client_addr: sockaddr_in = unsafe { std::mem::zeroed() };
        let mut iov = iovec {
            iov_base: buf.as_mut_ptr() as *mut c_void,
            iov_len:  buf.len(),
        };
        let mut cmsg_buf = vec![0u8; 1024];
        let mut msg: msghdr = unsafe { std::mem::zeroed() };
        msg.msg_name       = &mut client_addr as *mut _ as *mut c_void;
        msg.msg_namelen    = std::mem::size_of::<sockaddr_in>() as u32;
        msg.msg_iov        = &mut iov;
        msg.msg_iovlen     = 1;
        msg.msg_control    = cmsg_buf.as_mut_ptr() as *mut c_void;
        msg.msg_controllen = cmsg_buf.len();

        let n = unsafe { recvmsg(fd, &mut msg, 0) };
        if n < 0 {
            return Err(std::io::Error::last_os_error());
        }

        // Parse original destination from cmsg
        let mut orig_dst: sockaddr_in = unsafe { std::mem::zeroed() };
        let mut cm = unsafe { CMSG_FIRSTHDR(&msg) };
        while !cm.is_null() {
            let cm_ref = unsafe { &*cm };
            if cm_ref.cmsg_level == SOL_IP && cm_ref.cmsg_type == IP_ORIGDSTADDR {
                unsafe {
                    std::ptr::copy_nonoverlapping(
                        CMSG_DATA(cm),
                        &mut orig_dst as *mut _ as *mut u8,
                        std::mem::size_of::<sockaddr_in>(),
                    );
                }
                break;
            }
            cm = unsafe { CMSG_NXTHDR(&msg, cm) };
        }

        buf.truncate(n as usize);

        let client = sockaddr_in_to_socketaddr(&client_addr);
        let original = sockaddr_in_to_socketaddr(&orig_dst);

        Ok((buf, client, original))
    }
}

fn sockaddr_in_to_socketaddr(s: &sockaddr_in) -> std::net::SocketAddr {
    let ip = std::net::Ipv4Addr::from(u32::from_be(s.sin_addr.s_addr));
    let port = u16::from_be(s.sin_port);
    std::net::SocketAddr::new(std::net::IpAddr::V4(ip), port)
}
```

---

# 11. Implementation in Go

## 11.1 Go syscall Interface

Go provides `syscall` and `golang.org/x/sys/unix` packages for low-level socket operations. `IP_TRANSPARENT` is not in the standard library, so we define it ourselves or use the `unix` package.

```go
// go.mod
module tproxy-go

go 1.22

require golang.org/x/sys v0.21.0
```

## 11.2 TCP Transparent Proxy in Go

```go
// main.go — TCP transparent proxy in Go
// Run: sudo ./tproxy-go -port 8080

package main

import (
    "context"
    "flag"
    "fmt"
    "io"
    "log"
    "net"
    "syscall"
    "time"

    "golang.org/x/sys/unix"
)

const (
    IPTransparent     = 19 // IP_TRANSPARENT
    SOL_IP            = 0
)

// setIPTransparent enables IP_TRANSPARENT on a socket file descriptor.
func setIPTransparent(fd int) error {
    return syscall.SetsockoptInt(fd, SOL_IP, IPTransparent, 1)
}

// ListenTProxy creates a TCP listener socket with IP_TRANSPARENT enabled.
// The bind address should be 0.0.0.0:PORT or [::]:PORT.
func ListenTProxy(network, address string) (net.Listener, error) {
    // Use a custom control function to set socket options before bind
    lc := net.ListenConfig{
        Control: func(network, address string, c syscall.RawConn) error {
            var innerErr error
            err := c.Control(func(fd uintptr) {
                // SO_REUSEADDR
                if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET,
                    syscall.SO_REUSEADDR, 1); err != nil {
                    innerErr = fmt.Errorf("SO_REUSEADDR: %w", err)
                    return
                }
                // SO_REUSEPORT
                if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET,
                    unix.SO_REUSEPORT, 1); err != nil {
                    innerErr = fmt.Errorf("SO_REUSEPORT: %w", err)
                    return
                }
                // IP_TRANSPARENT — THE key option
                if err := setIPTransparent(int(fd)); err != nil {
                    innerErr = fmt.Errorf("IP_TRANSPARENT: %w", err)
                    return
                }
            })
            if err != nil {
                return err
            }
            return innerErr
        },
    }

    return lc.Listen(context.Background(), network, address)
}

// GetOriginalDst returns the original destination address of a TPROXY
// connection. With TPROXY, conn.LocalAddr() returns the original dst.
func GetOriginalDst(conn net.Conn) net.Addr {
    // With TPROXY, the "local" address IS the original destination.
    // getsockname() is what net.Conn.LocalAddr() calls internally.
    return conn.LocalAddr()
}

// dialWithoutTProxy connects to addr without IP_TRANSPARENT.
// Used for connecting to the real upstream server.
func dialWithoutTProxy(ctx context.Context, network, addr string) (net.Conn, error) {
    dialer := &net.Dialer{Timeout: 10 * time.Second}
    return dialer.DialContext(ctx, network, addr)
}

// proxyConn handles a single intercepted connection.
func proxyConn(clientConn net.Conn) {
    defer clientConn.Close()

    origDst := GetOriginalDst(clientConn)
    log.Printf("[+] Intercepted: %s -> %s", clientConn.RemoteAddr(), origDst)

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    serverConn, err := dialWithoutTProxy(ctx, "tcp", origDst.String())
    if err != nil {
        log.Printf("[-] Failed to connect to %s: %v", origDst, err)
        return
    }
    defer serverConn.Close()

    // Bidirectional copy
    errCh := make(chan error, 2)

    go func() {
        _, err := io.Copy(serverConn, clientConn)
        // Signal half-close
        if tc, ok := serverConn.(*net.TCPConn); ok {
            tc.CloseWrite()
        }
        errCh <- err
    }()

    go func() {
        _, err := io.Copy(clientConn, serverConn)
        if tc, ok := clientConn.(*net.TCPConn); ok {
            tc.CloseWrite()
        }
        errCh <- err
    }()

    // Wait for both directions to finish
    <-errCh
    <-errCh

    log.Printf("[-] Closed: %s -> %s", clientConn.RemoteAddr(), origDst)
}

func main() {
    port := flag.Int("port", 8080, "TPROXY listen port")
    flag.Parse()

    addr := fmt.Sprintf("0.0.0.0:%d", *port)
    ln, err := ListenTProxy("tcp", addr)
    if err != nil {
        log.Fatalf("ListenTProxy: %v", err)
    }
    defer ln.Close()

    log.Printf("[*] TPROXY listening on %s", addr)
    log.Printf("[*] Waiting for intercepted connections...")

    for {
        conn, err := ln.Accept()
        if err != nil {
            log.Printf("accept: %v", err)
            continue
        }
        go proxyConn(conn)
    }
}
```

## 11.3 UDP Transparent Proxy in Go

```go
// udp_tproxy.go — UDP TPROXY in Go
// Uses recvmsg with IP_RECVORIGDSTADDR to get original destination

package main

import (
    "encoding/binary"
    "fmt"
    "log"
    "net"
    "syscall"
    "unsafe"

    "golang.org/x/sys/unix"
)

const (
    IPTransparent      = 19
    IPRecvOrigDstAddr  = 20
    IPOrigDstAddr      = 20
    SOL_IP             = 0
)

type UDPTProxyConn struct {
    fd   int
    file *net.UDPConn
}

// NewUDPTProxy creates a UDP socket configured for TPROXY.
func NewUDPTProxy(port int) (*UDPTProxyConn, error) {
    fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
    if err != nil {
        return nil, fmt.Errorf("socket: %w", err)
    }

    // SO_REUSEADDR
    syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
    // IP_TRANSPARENT
    if err := syscall.SetsockoptInt(fd, SOL_IP, IPTransparent, 1); err != nil {
        syscall.Close(fd)
        return nil, fmt.Errorf("IP_TRANSPARENT: %w", err)
    }
    // IP_RECVORIGDSTADDR — causes kernel to append original dst to recvmsg
    if err := syscall.SetsockoptInt(fd, SOL_IP, IPRecvOrigDstAddr, 1); err != nil {
        syscall.Close(fd)
        return nil, fmt.Errorf("IP_RECVORIGDSTADDR: %w", err)
    }

    addr := syscall.SockaddrInet4{Port: port}
    if err := syscall.Bind(fd, &addr); err != nil {
        syscall.Close(fd)
        return nil, fmt.Errorf("bind: %w", err)
    }

    return &UDPTProxyConn{fd: fd}, nil
}

// RecvMsg receives a UDP datagram, extracting client and original destination.
func (u *UDPTProxyConn) RecvMsg() (data []byte, client, origDst *net.UDPAddr, err error) {
    buf := make([]byte, 65536)
    oob := make([]byte, 1024)

    var msg unix.Msghdr
    var clientSA unix.RawSockaddrInet4

    iov := unix.Iovec{
        Base: &buf[0],
        Len:  uint64(len(buf)),
    }
    msg.Name    = (*byte)(unsafe.Pointer(&clientSA))
    msg.Namelen = uint32(unix.SizeofSockaddrInet4)
    msg.Iov     = &iov
    msg.Iovlen  = 1
    msg.Control = &oob[0]
    msg.Controllen = uint64(len(oob))

    n, _, e := syscall.Syscall(syscall.SYS_RECVMSG,
        uintptr(u.fd),
        uintptr(unsafe.Pointer(&msg)),
        0)
    if e != 0 {
        return nil, nil, nil, e
    }

    data = buf[:n]

    // Parse client address
    client = &net.UDPAddr{
        IP:   net.IP(clientSA.Addr[:]),
        Port: int(binary.BigEndian.Uint16(
            []byte{byte(clientSA.Port >> 8), byte(clientSA.Port)},
        )),
    }

    // Parse original destination from cmsg
    msgs, _ := syscall.ParseSocketControlMessage(oob[:msg.Controllen])
    for _, m := range msgs {
        if m.Header.Level == SOL_IP && m.Header.Type == IPOrigDstAddr {
            if len(m.Data) >= 8 {
                port := binary.BigEndian.Uint16(m.Data[2:4])
                ip   := net.IP(m.Data[4:8])
                origDst = &net.UDPAddr{IP: ip, Port: int(port)}
            }
        }
    }

    return data, client, origDst, nil
}

// SendReplyFromOrigDst sends a UDP reply that appears to originate
// from the original destination address (using IP_TRANSPARENT bind).
func SendReplyFromOrigDst(origDst, client *net.UDPAddr, data []byte) error {
    fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
    if err != nil {
        return err
    }
    defer syscall.Close(fd)

    syscall.SetsockoptInt(fd, SOL_IP, IPTransparent, 1)
    syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)

    // Bind to original destination (non-local with IP_TRANSPARENT)
    var src syscall.SockaddrInet4
    copy(src.Addr[:], origDst.IP.To4())
    src.Port = origDst.Port
    syscall.Bind(fd, &src)

    // Send to client — appears to come from origDst
    var dst syscall.SockaddrInet4
    copy(dst.Addr[:], client.IP.To4())
    dst.Port = client.Port
    return syscall.Sendto(fd, data, 0, &dst)
}
```

## 11.4 Getting Original Destination — SO_ORIGINAL_DST vs TPROXY

```go
// For comparison: the OLD way (REDIRECT + SO_ORIGINAL_DST) vs TPROXY

// OLD WAY: SO_ORIGINAL_DST (requires conntrack, only IPv4 TCP)
func getOrigDstViaConntrack(conn *net.TCPConn) (*net.TCPAddr, error) {
    f, err := conn.File()
    if err != nil {
        return nil, err
    }
    defer f.Close()

    const SO_ORIGINAL_DST = 80
    mtuinfo, err := syscall.GetsockoptIPv6MTUInfo(
        int(f.Fd()), syscall.SOL_IP, SO_ORIGINAL_DST)
    // ... parse mtuinfo
    _ = mtuinfo
    return nil, err
}

// NEW WAY: TPROXY — just use LocalAddr()
func getOrigDstViaTProxy(conn net.Conn) net.Addr {
    return conn.LocalAddr() // That's it. Original dst is preserved natively.
}
```

---

# 12. TPROXY for TLS/HTTPS Interception

## 12.1 TLS Interception Architecture

When intercepting HTTPS, TPROXY gives us the TCP connection. To inspect the content, we must terminate TLS:

```
  CLIENT
    │  TLS ClientHello → SNI: "example.com"
    │
    ▼
  TPROXY PROXY
    │  1. Accept TCP connection (original dst = 93.184.216.34:443)
    │  2. Peek at TLS ClientHello → extract SNI ("example.com")
    │  3. Decide: intercept or pass through?
    │  4. If intercept: generate cert for "example.com" signed by local CA
    │  5. Complete TLS handshake with client (present fake cert)
    │  6. Open new TLS connection to 93.184.216.34:443
    │  7. Forward decrypted application data
    │
    ▼
  REAL SERVER (93.184.216.34:443)
```

## 12.2 SNI Extraction Without Termination

```c
/* Peek at TLS ClientHello to extract SNI without consuming data */

#include <openssl/ssl.h>

/* Peek (MSG_PEEK) at the first bytes to parse ClientHello */
ssize_t peek_sni(int fd, char *sni_buf, size_t sni_len)
{
    unsigned char buf[4096];
    ssize_t n = recv(fd, buf, sizeof(buf), MSG_PEEK);
    if (n < 5) return -1;

    /* TLS record header: type(1) version(2) length(2) */
    if (buf[0] != 0x16) return -1;  /* not Handshake */
    
    /* Handshake header: type(1) length(3) */
    if (buf[5] != 0x01) return -1;  /* not ClientHello */

    /* Skip to extensions in ClientHello... */
    /* (Full TLS ClientHello parsing omitted for brevity) */
    /* Use a library like mbedTLS's ssl_parse_client_hello */
    
    return 0;
}
```

## 12.3 Dynamic Certificate Generation

```go
// cert_gen.go — Generate per-domain TLS certificates signed by local CA

package tls_intercept

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/tls"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/pem"
    "math/big"
    "time"
)

type CertCache struct {
    ca      *x509.Certificate
    caKey   *rsa.PrivateKey
    cache   map[string]*tls.Certificate
}

func (cc *CertCache) CertForHost(hostname string) (*tls.Certificate, error) {
    if cert, ok := cc.cache[hostname]; ok {
        return cert, nil
    }

    privKey, _ := rsa.GenerateKey(rand.Reader, 2048)

    template := &x509.Certificate{
        SerialNumber: big.NewInt(time.Now().UnixNano()),
        Subject: pkix.Name{
            CommonName:   hostname,
            Organization: []string{"TProxy Intercept"},
        },
        DNSNames:  []string{hostname},
        NotBefore: time.Now().Add(-time.Hour),
        NotAfter:  time.Now().Add(24 * time.Hour),
        KeyUsage:  x509.KeyUsageDigitalSignature,
        ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
    }

    certDER, err := x509.CreateCertificate(rand.Reader, template, cc.ca,
        &privKey.PublicKey, cc.caKey)
    if err != nil {
        return nil, err
    }

    tlsCert := &tls.Certificate{
        Certificate: [][]byte{certDER},
        PrivateKey:  privKey,
    }
    cc.cache[hostname] = tlsCert
    return tlsCert, nil
}
```

---

# 13. Architecture Patterns

## 13.1 Inline Bump-in-the-Wire

The most common TPROXY architecture: the proxy box sits physically (or logically) in the path of all traffic.

```
  ┌──────────┐       ┌─────────────────────────────────────┐       ┌──────────┐
  │  CLIENT  │       │           PROXY BOX                  │       │ INTERNET │
  │ NETWORK  │       │                                     │       │          │
  │          │       │  eth0 (LAN)    eth1 (WAN)           │       │          │
  │  Clients ├──────►│  10.0.0.1/24   203.0.113.1          ├──────►│  Servers │
  │          │       │                                     │       │          │
  └──────────┘       │  ┌─────────────────────────────┐   │       └──────────┘
                     │  │  iptables / nftables         │   │
                     │  │  PREROUTING: TPROXY          │   │
                     │  │  mark 1 → table 100 → lo     │   │
                     │  └────────────┬────────────────┘   │
                     │               │                     │
                     │  ┌────────────▼────────────────┐   │
                     │  │  Proxy Daemon (port 8080)    │   │
                     │  │  IP_TRANSPARENT socket       │   │
                     │  │  reads original dst          │   │
                     │  │  forwards to eth1            │   │
                     │  └─────────────────────────────┘   │
                     └─────────────────────────────────────┘
```

**Routing on the proxy box:**
```bash
# Enable IP forwarding for non-intercepted traffic
echo 1 > /proc/sys/net/ipv4/ip_forward

# All traffic from LAN to WAN → intercept via TPROXY
iptables -t mangle -A PREROUTING -i eth0 -p tcp --dport 443 \
    -j TPROXY --tproxy-mark 1 --on-port 8080
```

## 13.2 Policy-Based TPROXY (Selective Interception)

Not all traffic needs to be intercepted. Policy rules can select specific sources/destinations:

```bash
# Only intercept traffic from specific VLAN (10.10.5.0/24)
iptables -t mangle -A PREROUTING \
    -s 10.10.5.0/24 -p tcp --dport 443 \
    -j TPROXY --tproxy-mark 1 --on-port 8443

# Whitelist specific destinations (skip bank, healthcare sites)
iptables -t mangle -A PREROUTING \
    -p tcp --dport 443 -d 192.0.2.10 \
    -j RETURN   # don't intercept

# Intercept rest
iptables -t mangle -A PREROUTING \
    -p tcp --dport 443 \
    -j TPROXY --tproxy-mark 1 --on-port 8443
```

## 13.3 Kubernetes / Container Network TPROXY

In Kubernetes, the service mesh (e.g., Istio) uses TPROXY to intercept all pod traffic transparently using an **init container** and a sidecar (Envoy):

```
  ┌─────────────────────────────────────────────────────────────┐
  │  POD                                                        │
  │                                                             │
  │  ┌──────────────┐       ┌──────────────────────────────┐  │
  │  │ App Container│◄─────►│ Envoy Sidecar                │  │
  │  │ (port 8080)  │       │ (TPROXY listener :15001)     │  │
  │  └──────────────┘       └──────────────────────────────┘  │
  │                                    ▲                        │
  │  ┌─────────────────────────────────┤────────────────────┐  │
  │  │ Network Namespace               │                    │  │
  │  │                                 │                    │  │
  │  │  iptables (set by init container):                   │  │
  │  │  -t mangle -A PREROUTING                             │  │
  │  │      -p tcp ! --dport 15001                          │  │
  │  │      -j TPROXY --on-port 15001 --tproxy-mark 1      │  │
  │  │                                                      │  │
  │  │  -t mangle -A OUTPUT                                 │  │
  │  │      -p tcp ! --uid-owner 1337 (envoy)               │  │
  │  │      -j MARK --set-mark 1                            │  │
  │  └──────────────────────────────────────────────────────┘  │
  └─────────────────────────────────────────────────────────────┘
```

The init container (runs before the app) installs the iptables rules. The Envoy sidecar binds with `IP_TRANSPARENT` and intercepts all inbound/outbound traffic, applying mTLS, circuit breaking, load balancing, etc.

**Istio iptables init logic (simplified):**
```bash
# Intercept inbound (to app containers)
iptables -t mangle -A PREROUTING \
    -p tcp ! --dport 15090 ! --dport 15021 \
    -j TPROXY --on-port 15001 --tproxy-mark 1

# Redirect outbound (from app containers)
iptables -t nat -A OUTPUT \
    -p tcp ! -o lo ! -m owner --uid-owner 1337 \
    -j REDIRECT --to-port 15001
```

Note: Istio actually uses REDIRECT (not TPROXY) by default for outbound, but uses TPROXY for inbound since 1.12+ and for CNI (container network interface) scenarios.

## 13.4 eBPF-Accelerated TPROXY Pattern

Modern kernels (5.7+) allow replacing netfilter rules with eBPF programs attached at `BPF_PROG_TYPE_CGROUP_SOCK_OPS` or `BPF_PROG_TYPE_SK_SKB`. Cilium and other CNI plugins use this approach:

```
  Packet arrives
      │
      ▼
  eBPF at TC (Traffic Control) ingress hook
      │
      ├─ bpf_sk_redirect_map() — redirect to proxy socket
      │   (replaces iptables TPROXY + routing rules)
      │
      ▼
  Proxy socket receives packet
  (no netfilter, no routing table lookup)
```

eBPF TPROXY is significantly faster (~30-50% lower latency) because it bypasses the full netfilter traversal.

---

# 14. Cloud Deployments

## 14.1 AWS — Gateway Load Balancer (GWLB)

AWS provides Gateway Load Balancer specifically for "bump-in-the-wire" network appliances. GWLB uses the **GENEVE** tunnel protocol to preserve original packet headers (similar in spirit to TPROXY at the IP level):

```
  VPC Traffic Flow with GWLB:
  ────────────────────────────────────────────────────────────────

  Internet Gateway
       │
       ▼
  GWLB Endpoint (in workload VPC)
  [Traffic intercepted by GWLB; original headers preserved in GENEVE]
       │
       │ GENEVE tunnel (UDP/6081)
       ▼
  GWLB (load balancer)
       │
       ▼
  NVA EC2 Instances (your TPROXY proxy running on each instance)
  [EC2 unwraps GENEVE, sees original src/dst, processes packet]
       │
       │ GENEVE tunnel (response)
       ▼
  GWLB
       │
       ▼
  Workload VPC continues routing
```

**On the EC2 NVA instance:** Run a userspace GENEVE tunnel terminator (or use the kernel's `geneve` module), then run TPROXY rules on the inner packet:

```bash
# Add GENEVE tunnel interface
ip link add geneve0 type geneve id 1234 remote 10.0.0.1
ip link set geneve0 up
ip addr add 169.254.0.1/28 dev geneve0

# TPROXY on decapsulated traffic
iptables -t mangle -A PREROUTING -i geneve0 -p tcp -j TPROXY \
    --tproxy-mark 1 --on-port 8080
```

**Terraform for GWLB:**
```hcl
resource "aws_lb" "inspection_gwlb" {
  name               = "inspection-gwlb"
  load_balancer_type = "gateway"
  subnets            = [aws_subnet.inspection.id]
}

resource "aws_lb_target_group" "nva" {
  name     = "nva-targets"
  port     = 6081          # GENEVE port
  protocol = "GENEVE"
  vpc_id   = aws_vpc.main.id

  health_check {
    protocol = "TCP"
    port     = "80"
  }
}

resource "aws_vpc_endpoint_service" "gwlb" {
  acceptance_required        = false
  gateway_load_balancer_arns = [aws_lb.inspection_gwlb.arn]
}
```

## 14.2 GCP — Packet Mirroring + IDS

GCP Packet Mirroring mirrors traffic to a collector (useful for IDS, not for inline proxy). For inline TPROXY on GCP:

```
  GCP Network — Inline Inspection via Routing:
  ──────────────────────────────────────────────────────

  VPC Route Table:
  0.0.0.0/0 → Next Hop: Inspection VM Instance (NVA)
                │
                │ NVA runs TPROXY:
                │   - ip_forward enabled
                │   - iptables TPROXY rules
                │   - Proxy process (Go/Rust)
                │
                ▼
  Cloud NAT / Internet Gateway
```

**GCP custom route to force traffic through NVA:**
```bash
# Using gcloud
gcloud compute routes create force-through-nva \
  --network=my-vpc \
  --destination-range=0.0.0.0/0 \
  --next-hop-instance=nva-vm-1 \
  --next-hop-instance-zone=us-central1-a \
  --priority=900  # higher priority than default route

# On the NVA VM (GCP requires disabling source/dest check):
# Via gcloud:
gcloud compute instances add-metadata nva-vm-1 \
  --metadata=serial-port-enable=true
# Actually disable src/dst check via API (can_ip_forward in TF)
```

**Terraform for GCP NVA:**
```hcl
resource "google_compute_instance" "nva" {
  name         = "tproxy-nva"
  machine_type = "n2-standard-4"
  zone         = "us-central1-a"

  # CRITICAL: disable source/destination check
  can_ip_forward = true

  network_interface {
    network    = google_compute_network.vpc.id
    subnetwork = google_compute_subnetwork.nva.id
    access_config {}
  }

  metadata_startup_script = file("nva_startup.sh")
}
```

## 14.3 Azure — NVA (Network Virtual Appliance)

Azure uses User Defined Routes (UDR) to force traffic through an NVA VM:

```
  Azure VNET Architecture:
  ──────────────────────────────────────────────────────────────

  ┌─────────────────────────────────────────────────────────────┐
  │  VNET (10.0.0.0/16)                                        │
  │                                                             │
  │  ┌──────────────────┐    ┌──────────────────────────────┐  │
  │  │  Workload Subnet  │    │  NVA Subnet (10.0.1.0/24)   │  │
  │  │  10.0.2.0/24     │    │                              │  │
  │  │                  │    │  ┌──────────────────────┐   │  │
  │  │  App VMs         │    │  │  NVA VM               │   │  │
  │  │                  │    │  │  (Ubuntu + TPROXY)    │   │  │
  │  └───────┬──────────┘    │  │  IP Forwarding: ON    │   │  │
  │          │               │  └──────────────────────┘   │  │
  │          │ UDR: 0.0.0.0/0│                              │  │
  │          │ → NVA VM NIC  │                              │  │
  │          └───────────────►                              │  │
  │                           └──────────────────────────────┘  │
  │                                         │                    │
  │                                    Internet                  │
  └─────────────────────────────────────────────────────────────┘
```

```bash
# Azure CLI: create UDR to route through NVA
az network route-table create \
    --name force-via-nva \
    --resource-group my-rg \
    --location eastus

az network route-table route create \
    --route-table-name force-via-nva \
    --resource-group my-rg \
    --name default-to-nva \
    --address-prefix 0.0.0.0/0 \
    --next-hop-type VirtualAppliance \
    --next-hop-ip-address 10.0.1.4  # NVA's private IP

# Associate route table with workload subnet
az network vnet subnet update \
    --vnet-name my-vnet \
    --name workload-subnet \
    --resource-group my-rg \
    --route-table force-via-nva
```

## 14.4 Cloud-Native Constraints

| Issue | Cause | Solution |
|---|---|---|
| NIC src/dst check | Cloud hypervisor drops packets with non-NIC IPs | Disable per-NIC or enable IP forwarding flag |
| MTU mismatch | GENEVE/VxLAN encapsulation adds overhead | Adjust MTU on tunnel interfaces; TCP MSS clamping |
| Asymmetric routing | Return traffic bypasses proxy | Ensure both ingress and egress traverse NVA; use per-flow routing |
| BGP/ECMP load balancing | Traffic may split across NVA instances | Use flow-based ECMP; sticky sessions; GWLB handles this |
| Ephemeral IPs | Cloud VMs may have different IPs after restart | Use Elastic IPs (AWS) or static internal IPs |

---

# 15. Performance Tuning

## 15.1 Kernel Network Buffer Tuning

```bash
# /etc/sysctl.d/99-tproxy-perf.conf

# Socket receive/send buffer sizes
net.core.rmem_max          = 134217728   # 128MB max receive buffer
net.core.wmem_max          = 134217728   # 128MB max send buffer
net.core.rmem_default      = 31457280    # 30MB default
net.core.wmem_default      = 31457280

# TCP-specific buffers
net.ipv4.tcp_rmem          = 4096 87380 134217728
net.ipv4.tcp_wmem          = 4096 65536 134217728

# Connection backlog
net.core.somaxconn         = 65535
net.core.netdev_max_backlog = 65535

# TCP SYN backlog
net.ipv4.tcp_max_syn_backlog = 65535

# Allow more local ports (for upstream connections from proxy)
net.ipv4.ip_local_port_range = 1024 65535

# TIME_WAIT optimization
net.ipv4.tcp_tw_reuse      = 1
net.ipv4.tcp_fin_timeout   = 10

# Increase conntrack table (if using conntrack)
net.netfilter.nf_conntrack_max = 1048576
net.netfilter.nf_conntrack_buckets = 262144

# Apply
sysctl -p /etc/sysctl.d/99-tproxy-perf.conf
```

## 15.2 splice() for Zero-Copy Forwarding

The `splice()` system call transfers data between file descriptors via a kernel pipe buffer, avoiding userspace copies:

```c
/*
 * splice_data - move up to `len` bytes from in_fd to out_fd
 * via a kernel pipe (zero userspace copies)
 */
ssize_t splice_data(int in_fd, int out_fd, size_t len)
{
    int pipefd[2];
    if (pipe(pipefd) < 0) return -1;

    /* Move data from in_fd into pipe (kernel buffer) */
    ssize_t spliced = splice(in_fd, NULL, pipefd[1], NULL,
                             len, SPLICE_F_MOVE | SPLICE_F_MORE);
    if (spliced <= 0) {
        close(pipefd[0]); close(pipefd[1]);
        return spliced;
    }

    /* Move data from pipe to out_fd */
    ssize_t written = splice(pipefd[0], NULL, out_fd, NULL,
                             spliced, SPLICE_F_MOVE | SPLICE_F_MORE);

    close(pipefd[0]); close(pipefd[1]);
    return written;
}
```

For optimal performance, use `splice()` in a loop with `SPLICE_F_NONBLOCK` and `epoll`.

## 15.3 SO_REUSEPORT and Worker Scaling

`SO_REUSEPORT` allows multiple sockets to bind to the same port. The kernel distributes incoming connections/datagrams across all sockets (with CPU-affinity awareness in recent kernels):

```c
/* Start N worker processes/threads, each binding to the same port */
for (int i = 0; i < num_cpus; i++) {
    pid_t pid = fork();
    if (pid == 0) {
        int fd = socket(AF_INET, SOCK_STREAM, 0);
        int on = 1;
        setsockopt(fd, SOL_SOCKET, SO_REUSEPORT,     &on, sizeof(on));
        setsockopt(fd, SOL_IP,     IP_TRANSPARENT,    &on, sizeof(on));
        /* bind, listen, accept loop... */
        worker_loop(fd);  /* each worker handles its own connections */
        exit(0);
    }
}
```

This eliminates the single-threaded `accept()` bottleneck. Each core runs a worker, and the kernel distributes connections.

## 15.4 eBPF TPROXY (Bypassing Netfilter)

Using `BPF_PROG_TYPE_SCHED_CLS` + `bpf_sk_redirect_map`:

```c
/* eBPF program: redirect packets to proxy socket without netfilter */
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

/* Map from port -> proxy socket */
struct {
    __uint(type, BPF_MAP_TYPE_SOCKHASH);
    __uint(max_entries, 65536);
    __type(key, __u32);    /* destination port */
    __type(value, __u64);  /* socket cookie */
} proxy_sockets SEC(".maps");

SEC("classifier/ingress")
int tproxy_redirect(struct __sk_buff *skb)
{
    void *data     = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;

    struct iphdr  *ip;
    struct tcphdr *tcp;

    ip = data;
    if ((void *)(ip + 1) > data_end) return TC_ACT_OK;
    if (ip->protocol != IPPROTO_TCP)  return TC_ACT_OK;

    tcp = (void *)ip + (ip->ihl * 4);
    if ((void *)(tcp + 1) > data_end) return TC_ACT_OK;

    __u32 dport = bpf_ntohs(tcp->dest);

    /* Only intercept HTTP and HTTPS */
    if (dport != 80 && dport != 443) return TC_ACT_OK;

    /* Redirect to proxy socket */
    return bpf_sk_redirect_map(skb, &proxy_sockets, dport, 0);
}
```

---

# 16. Debugging and Troubleshooting

## 16.1 Verify iptables Rules Are Applied

```bash
# Check mangle PREROUTING rules
iptables -t mangle -L PREROUTING -v -n --line-numbers
# Look for non-zero packet/byte counters to confirm traffic hits the rules

# If counters are zero but traffic is flowing:
# - Check interface: are you intercepting on the right interface?
# - Check IP version: are you using ip6tables for IPv6?
# - Check conntrack: is conntrack marking packets ESTABLISHED before mangle sees them?
```

## 16.2 Verify Routing Rules

```bash
# Policy routing rules
ip rule list
# Expected output:
# 0:      from all lookup local
# 32764:  from all fwmark 0x1 lookup 100
# 32766:  from all lookup main
# 32767:  from all lookup default

# Routing table 100
ip route show table 100
# Expected: local 0.0.0.0/0 dev lo scope host

# Test: route lookup for an external IP with mark
ip route get 8.8.8.8 mark 1
# Expected: local 8.8.8.8 dev lo src 127.0.0.1 uid 0
#                         ^^^^ "local" — means it will be delivered locally
```

## 16.3 Test IP_TRANSPARENT is Working

```bash
# Quick test: try to bind to a non-local IP WITHOUT CAP_NET_ADMIN
python3 -c "
import socket
s = socket.socket()
s.setsockopt(socket.IPPROTO_IP, 19, 1)  # IP_TRANSPARENT = 19
s.bind(('8.8.8.8', 0))
print('Bind succeeded (has CAP_NET_ADMIN)')
"
# If you see "OSError: [Errno 13] Permission denied" → need root/capability
# If you see "OSError: [Errno 99] Cannot assign requested address" → IP_TRANSPARENT not set
```

## 16.4 tcpdump to Verify Interception

```bash
# Before interception: capture on external interface
tcpdump -i eth0 'tcp and port 443' -n

# After iptables TPROXY rule: capture on loopback
tcpdump -i lo 'tcp and port 8080' -n
# If you see traffic here, TPROXY is working

# Capture the TPROXY socket specifically
tcpdump -i any 'tcp and (port 443 or port 8080)' -n

# Check with conntrack
conntrack -L | grep "443"
# Look for ORIGINAL src/dst being preserved
```

## 16.5 strace to Debug Socket Issues

```bash
# Trace socket calls in your proxy to see what's happening
strace -e trace=socket,bind,accept,getsockname,setsockopt \
    -f ./your_tproxy_process

# Key things to look for:
# setsockopt(fd, SOL_IP, IP_TRANSPARENT, [1]) = 0   ← should succeed
# getsockname(client_fd, {sa_family=AF_INET,          ← after accept
#             sin_addr=93.184.216.34,                  ← original dst preserved!
#             sin_port=443}) = 0
```

## 16.6 Common Pitfalls and Fixes

```
  PROBLEM 1: getsockname() returns 0.0.0.0 or ::, not original dst
  ────────────────────────────────────────────────────────────────
  CAUSE:   TPROXY rules not installed, or routing not set up.
           Packet wasn't actually TPROXY'd.
  FIX:     Check iptables -t mangle -L, check ip rule list, check ip route table 100.
           Also check: are you binding the listening socket with IP_TRANSPARENT?

  PROBLEM 2: "Cannot assign requested address" when binding
  ────────────────────────────────────────────────────────────────
  CAUSE:   IP_TRANSPARENT not set on the socket.
  FIX:     Call setsockopt(fd, SOL_IP, IP_TRANSPARENT, 1) BEFORE bind().

  PROBLEM 3: "Operation not permitted" on setsockopt(IP_TRANSPARENT)
  ────────────────────────────────────────────────────────────────
  CAUSE:   Process lacks CAP_NET_ADMIN.
  FIX:     Run as root, or grant capability:
           setcap cap_net_admin+ep ./your_proxy
           # or in a container: securityContext.capabilities.add: [NET_ADMIN]

  PROBLEM 4: Proxy traffic loops (proxy's own traffic intercepted)
  ────────────────────────────────────────────────────────────────
  CAUSE:   TPROXY rules intercept the proxy's own outbound connections.
  FIX:     Add iptables rule to skip proxy user's traffic:
           iptables -t mangle -I PREROUTING 1 \
               -m owner --uid-owner proxy -j RETURN
           Or skip by firewall mark (proxy sets SO_MARK on outbound sockets).

  PROBLEM 5: TPROXY drops UDP packets with EINVAL
  ────────────────────────────────────────────────────────────────
  CAUSE:   UDP TPROXY requires IP_TRANSPARENT on receive socket
           AND IP_RECVORIGDSTADDR.
  FIX:     Set both socket options.

  PROBLEM 6: conntrack interferes — original dst is NAT'd address
  ────────────────────────────────────────────────────────────────
  CAUSE:   conntrack is NATing packets before TPROXY sees them.
  FIX:     Bypass conntrack for TPROXY'd traffic:
           iptables -t raw -A PREROUTING -p tcp --dport 443 -j NOTRACK
           (or ACCEPT in raw to skip conntrack entirely for these packets)

  PROBLEM 7: IPv6 not working
  ────────────────────────────────────────────────────────────────
  CAUSE:   Using ip6tables but not ip -6 rules, or vice versa.
  FIX:     ip -6 rule add fwmark 1 lookup 100
           ip -6 route add local ::/0 dev lo table 100
           Use IPV6_TRANSPARENT (setsockopt on SOL_IPV6) for IPv6 sockets.

  PROBLEM 8: Performance issues — high latency
  ────────────────────────────────────────────────────────────────
  CAUSE:   Netfilter traversal overhead; small socket buffers; context switches.
  FIX:     Tune kernel buffers (see §15.1), use SO_REUSEPORT workers,
           consider eBPF replacement for netfilter rules,
           use splice() for forwarding.
```

---

# 17. Security Considerations

## 17.1 IP Spoofing Risk

`IP_TRANSPARENT` combined with bind allows a process to send packets appearing to come from any IP. This is a privilege, not a bug — hence `CAP_NET_ADMIN` requirement. Mitigations:

- Run the proxy with minimal capabilities: `cap_net_admin` only, drop all others.
- Use network namespaces to isolate the proxy.
- Implement egress filtering to prevent the proxy from spoofing arbitrary IPs.

```bash
# Run proxy with only CAP_NET_ADMIN (not full root)
setcap 'cap_net_admin=+ep' /usr/local/bin/tproxy

# Or in a systemd service:
# [Service]
# AmbientCapabilities=CAP_NET_ADMIN
# CapabilityBoundingSet=CAP_NET_ADMIN
# User=tproxy
# Group=tproxy
```

## 17.2 Certificate Trust in TLS Interception

TLS interception is a form of MITM. Clients will reject certificates unless:

1. The proxy CA certificate is trusted by clients (installed in OS/browser cert store).
2. Certificate pinning is not in use (pinning defeats MITM).

For corporate environments, the CA cert is distributed via MDM or Group Policy. For hostile interception, this is a serious ethical/legal concern.

## 17.3 Avoiding Bypass

Clients may attempt to bypass a transparent proxy by:

- Using non-standard ports → intercept all ports, not just 80/443
- Using UDP (QUIC/HTTP3) → TPROXY UDP rules for port 443/UDP
- Using encrypted DNS (DoH) → intercept port 443 entirely or use encrypted DNS resolvers on the proxy
- Using a VPN → intercept VPN protocols or force split-tunnel at the network level

---

# 18. Real-World Projects Using TPROXY

| Project | Language | How TPROXY Is Used |
|---|---|---|
| **Envoy Proxy** (Istio) | C++ | Sidecar interception in Kubernetes pods |
| **redsocks** | C | Redirect any TCP connection to a SOCKS proxy |
| **tun2socks** | Go/C | Tunnel all traffic through a SOCKS5/VLESS proxy |
| **Clash** | Go | Feature-rich proxy client with TPROXY mode |
| **Xray-core** | Go | V2Ray successor; TPROXY mode for transparent proxying |
| **sing-box** | Go | Universal proxy platform with TPROXY listener |
| **gVisor** | Go | Container sandbox with TPROXY-like interception |
| **Zorp** | C | Original TPROXY creator's enterprise firewall |
| **Squid** | C | Enterprise web cache with TPROXY (WCCP integration) |
| **mitmproxy** | Python | Transparent HTTPS interception for debugging |
| **Cilium** | Go/eBPF | eBPF-based TPROXY replacement in Kubernetes |

### redsocks Configuration (Example of TPROXY in Practice)

```
# /etc/redsocks.conf
base {
    log_debug = on;
    log_info  = on;
    daemon    = on;
    redirector = tproxy;  // use TPROXY instead of REDIRECT
}

redsocks {
    local_ip   = 0.0.0.0;
    local_port = 12345;
    ip         = 192.168.1.100;  // SOCKS proxy IP
    port       = 1080;
    type       = socks5;
}
```

```bash
# iptables rules for redsocks with TPROXY
iptables -t mangle -N REDSOCKS
iptables -t mangle -A REDSOCKS \
    -d 0.0.0.0/8 -j RETURN
iptables -t mangle -A REDSOCKS \
    -d 127.0.0.0/8 -j RETURN
iptables -t mangle -A REDSOCKS \
    -d 192.168.0.0/16 -j RETURN
iptables -t mangle -A REDSOCKS \
    -p tcp -j TPROXY --on-port 12345 --tproxy-mark 0x1/0x1
iptables -t mangle -A PREROUTING \
    -p tcp -j REDSOCKS
```

---

# 19. Complete Reference Cheatsheet

## System Setup Commands

```bash
# ── Kernel modules ────────────────────────────────────────────────────────
modprobe xt_TPROXY nf_tproxy_core nf_tproxy_ipv4 nf_tproxy_ipv6 xt_socket

# ── Policy routing (required) ─────────────────────────────────────────────
ip rule  add fwmark 1 lookup 100
ip route add local 0.0.0.0/0 dev lo table 100
ip -6 rule  add fwmark 1 lookup 100
ip -6 route add local ::/0 dev lo table 100

# ── iptables TPROXY rules ─────────────────────────────────────────────────
# TCP port 80
iptables -t mangle -A PREROUTING -p tcp --dport 80 \
    -j TPROXY --tproxy-mark 1 --on-port 8080

# TCP port 443
iptables -t mangle -A PREROUTING -p tcp --dport 443 \
    -j TPROXY --tproxy-mark 1 --on-port 8080

# UDP port 53 (DNS)
iptables -t mangle -A PREROUTING -p udp --dport 53 \
    -j TPROXY --tproxy-mark 1 --on-port 15353

# ── nftables equivalent ───────────────────────────────────────────────────
nft add table inet tproxy
nft add chain inet tproxy prerouting \
    '{ type filter hook prerouting priority mangle; }'
nft add rule inet tproxy prerouting \
    'ip protocol tcp tcp dport { 80, 443 } tproxy ip to :8080 meta mark set 1'
nft add rule inet tproxy prerouting \
    'ip protocol udp udp dport 53 tproxy ip to :15353 meta mark set 1'

# ── Socket options (setsockopt values) ───────────────────────────────────
# IP_TRANSPARENT       = 19  (SOL_IP/IPPROTO_IP)
# IPV6_TRANSPARENT     = 75  (SOL_IPV6/IPPROTO_IPV6)
# IP_RECVORIGDSTADDR   = 20  (SOL_IP) — for UDP original dst
# SO_ORIGINAL_DST      = 80  (SOL_IP) — old conntrack method (not TPROXY)

# ── Grant capability without root ────────────────────────────────────────
setcap cap_net_admin+ep ./proxy_binary

# ── Verify setup ──────────────────────────────────────────────────────────
iptables -t mangle -L PREROUTING -v -n
ip rule list
ip route show table 100
ip route get 8.8.8.8 mark 1   # should show "local"
```

## TPROXY Key Concepts Summary

```
  TPROXY = Netfilter target (xt_TPROXY module) in mangle/PREROUTING
         + IP_TRANSPARENT socket option
         + Policy routing (fwmark → local routing table)

  Flow:
  1. Packet arrives at PREROUTING
  2. TPROXY target assigns packet to listening socket (skb->sk = socket)
  3. TPROXY target sets firewall mark on packet (skb->mark = N)
  4. ip rule: fwmark N → route table 100
  5. Route table 100: local 0.0.0.0/0 → deliver locally
  6. TCP/UDP layer delivers to the pre-assigned socket
  7. Proxy calls accept() or recvmsg()
  8. getsockname() returns original destination IP:port

  Original destination retrieval:
  TCP: getsockname(accepted_fd) → original dst
  UDP: recvmsg() + IP_RECVORIGDSTADDR cmsg → original dst per datagram

  Requirements:
  - CAP_NET_ADMIN capability
  - IP_TRANSPARENT on the listening socket
  - Routing: ip rule + ip route for fwmark
  - Kernel modules: xt_TPROXY, nf_tproxy_ipv4/ipv6
```

## Socket Option Quick Reference

```c
/* TCP TPROXY listener setup — minimal viable example */
int fd = socket(AF_INET6, SOCK_STREAM, 0);
int on = 1, off = 0;

setsockopt(fd, IPPROTO_IPV6, IPV6_V6ONLY,    &off, sizeof off); // dual-stack
setsockopt(fd, SOL_SOCKET,   SO_REUSEADDR,    &on,  sizeof on);
setsockopt(fd, SOL_SOCKET,   SO_REUSEPORT,    &on,  sizeof on);
setsockopt(fd, SOL_IP,       IP_TRANSPARENT,  &on,  sizeof on); // KEY
setsockopt(fd, IPPROTO_TCP,  TCP_NODELAY,     &on,  sizeof on); // performance

struct sockaddr_in6 addr = {AF_INET6, htons(PORT), 0, in6addr_any, 0};
bind(fd, (struct sockaddr *)&addr, sizeof addr);
listen(fd, 4096);

/* After accept(fd, ...) → client_fd: */
struct sockaddr_storage orig_dst;
socklen_t len = sizeof orig_dst;
getsockname(client_fd, (struct sockaddr *)&orig_dst, &len);
/* orig_dst = the original destination the client intended to reach */

/* ── UDP TPROXY listener setup ── */
int ufd = socket(AF_INET, SOCK_DGRAM, 0);
setsockopt(ufd, SOL_IP, IP_TRANSPARENT,    &on, sizeof on); // KEY
setsockopt(ufd, SOL_IP, IP_RECVORIGDSTADDR,&on, sizeof on); // per-datagram orig dst
struct sockaddr_in uaddr = {AF_INET, htons(PORT), {INADDR_ANY}};
bind(ufd, (struct sockaddr *)&uaddr, sizeof uaddr);
/* Use recvmsg() + parse cmsg for IP_ORIGDSTADDR */
```

---

*End of TPROXY Complete Guide*

---

## Further Reading

- Linux kernel source: `net/netfilter/xt_TPROXY.c`
- Linux kernel docs: `Documentation/networking/tproxy.rst`
- RFC 793 (TCP), RFC 768 (UDP)
- Netfilter project: https://www.netfilter.org/
- iproute2 documentation: `man ip-rule`, `man ip-route`
- Original TPROXY paper by Balázs Scheidler (Balabit)
- Envoy proxy TPROXY source: `source/common/network/socket_interface_impl.cc`
- Cilium eBPF TPROXY: `bpf/bpf_sock.h`, `cilium/pkg/datapath`
