Great context from your workflow notes. Based on what you're doing — SSH-filtering `pr_info` trace points across `net/ipv4/` files — here's the **standard Linux kernel convention** for this:

---

## Where to put it

Since this header is only used **inside `net/ipv4/`** (by `tcp_output.c`, `ip_output.c`, `ip_input.c`), the correct place is:

```
linux-7.0.6/net/ipv4/net_nl_debug.h
```

This is the **standard pattern** for subsystem-private helpers in the kernel — the header lives alongside the `.c` files that use it, not in `include/`. Only headers shared across subsystems go into `include/net/`.

---

## How to include it

In each file you're instrumenting, add this near the top, after existing `#include` lines:

**`net/ipv4/tcp_output.c`**, **`net/ipv4/ip_output.c`**:
```c
#include "net_nl_debug.h"
```

**`net/ipv4/ip_input.c`** (for the `ip_rcv` path):
```c
#include "net_nl_debug.h"
```

---

## How to use it in your trace guards

Replace the raw port checks you had before with the helpers. This makes each trace point cleaner and consistent:

**NL-3 in `tcp_output.c`:**
```c
if (!skb_is_ssh(skb)) {
    pr_info("[NL-3] tcp_transmit_skb: skb=%px len=%u sport=%u dport=%u seq=%u\n",
            skb, skb->len,
            ntohs(inet_sk(sk)->inet_sport),
            ntohs(inet_sk(sk)->inet_dport),
            TCP_SKB_CB(skb)->seq);
}
```

**NL-4 in `ip_output.c`** (`ip_queue_xmit` / `ip_finish_output2`):
```c
if (!skb_is_ssh(skb)) {
    pr_info("[NL-5] ip_finish_output2: skb=%px len=%u dev=%s\n",
            skb, skb->len, dev->name);
}
```

**NL in `ip_input.c`** (`ip_rcv` — transport header not yet pulled):
```c
if (!ip_rcv_is_ssh(skb)) {
    pr_info("[NL-RX] ip_rcv: skb=%px len=%u\n", skb, skb->len);
}
```

---

## Why `ip_rcv_is_ssh()` is separate

`ip_rcv` runs **before** `skb->transport_header` is set, so `tcp_hdr(skb)` would give garbage. Your `ip_rcv_is_ssh()` manually peeks past the IP header with a bounds check — that's exactly the right approach and mirrors what `tcp_v4_rcv()` itself does before it pulls headers.

---

## Summary

| File | Include | Guard to use |
|---|---|---|
| `net/ipv4/tcp_output.c` | `#include "net_nl_debug.h"` | `skb_is_ssh(skb)` |
| `net/ipv4/ip_output.c` | `#include "net_nl_debug.h"` | `skb_is_ssh(skb)` |
| `net/ipv4/ip_input.c` | `#include "net_nl_debug.h"` | `ip_rcv_is_ssh(skb)` |
| `net/ipv4/net_nl_debug.h` | — | defines both helpers |

No `Makefile` changes needed — `.h` files in the same directory are picked up automatically since `net/ipv4/` is already in the compiler's include search path for files built from that directory.

CLion generated a generic guard. For a Linux kernel header, the convention is different. Here's what to change and why:

---

## Standard Linux kernel header guard style

```c
/* SPDX-License-Identifier: GPL-2.0 */
#ifndef _NET_NL_DEBUG_H
#define _NET_NL_DEBUG_H

/* your helpers here */

#endif /* _NET_NL_DEBUG_H */
```

---

## What CLion got wrong

| | CLion generated | Linux convention |
|---|---|---|
| **Prefix** | `CLION_` | `_` (single underscore) |
| **SPDX line** | missing | required for all kernel files |
| **Closing comment** | none | `/* _NET_NL_DEBUG_H */` — tells you what the `#endif` closes |
| **License** | none | `GPL-2.0` (must match the kernel) |

---

## Final file — `net/ipv4/net_nl_debug.h`

```c
/* SPDX-License-Identifier: GPL-2.0 */
#ifndef _NET_NL_DEBUG_H
#define _NET_NL_DEBUG_H

#include <linux/skbuff.h>
#include <linux/tcp.h>
#include <linux/ip.h>

static inline bool skb_is_ssh(const struct sk_buff *skb)
{
    const struct iphdr *iph = ip_hdr(skb);
    if (iph->protocol == IPPROTO_TCP) {
        const struct tcphdr *th = tcp_hdr(skb);
        return ntohs(th->source) == 22 || ntohs(th->dest) == 22;
    }
    return false;
}

static inline bool ip_rcv_is_ssh(const struct sk_buff *skb)
{
    const struct iphdr *iph = ip_hdr(skb);
    const __be16 *ports;
    if (iph->protocol != IPPROTO_TCP)
        return false;
    ports = (const __be16 *)((const u8 *)iph + iph->ihl * 4);
    if (!pskb_may_pull((struct sk_buff *)skb,
                       skb_network_offset(skb) + iph->ihl * 4 + 4))
        return false;
    return ntohs(ports[0]) == 22 || ntohs(ports[1]) == 22;
}

#endif /* _NET_NL_DEBUG_H */
```

---

You can configure CLion to use this style automatically under **Settings → Editor → Code Style → C/C++ → Header Guards** — set the pattern to `_${FILE_NAME_WITHOUT_EXT_UP}` to match kernel convention going forward.

Since it now spans **L2, L4, XDP, VXLAN, GENEVE** — it crosses multiple subsystems (`net/ipv4/`, `net/core/`, drivers, `net/vxlan/`). That changes the file location too.

---

## New location: `include/net/nl_debug.h`

`net/ipv4/` was fine when it was only L4. Multi-subsystem headers belong in `include/net/` — that's where `ip.h`, `tcp.h`, `vxlan.h` etc. all live. Then any `.c` file across the tree includes it with `#include <net/nl_debug.h>`.

---

## Full header — `include/net/nl_debug.h`

```c
/* SPDX-License-Identifier: GPL-2.0 */
/*
 * include/net/nl_debug.h — per-layer debug/trace helpers for netstack study
 *
 * Sections:
 *   §1  L2     — Ethernet frame classification
 *   §2  L4     — TCP / UDP port helpers
 *   §3  XDP    — xdp_buff frame parsing (kernel/driver side)
 *   §4  VXLAN  — outer + inner frame helpers  (UDP/4789)
 *   §5  GENEVE — outer + inner frame helpers  (UDP/6081)
 *
 * NOTE on eBPF:
 *   BPF programs loaded via bpf() syscall run inside the verifier sandbox
 *   and cannot call these helpers directly. Use bpf_skb_load_bytes(),
 *   bpf_xdp_load_bytes() etc. from your BPF C source instead.
 *   These helpers are for the *kernel C side* only — kprobes, kfuncs,
 *   native XDP hooks in drivers, and your pr_info trace points.
 *
 * All functions are static inline — zero cost when condition is false;
 * the compiler eliminates dead branches entirely.
 *
 * Usage:  #include <net/nl_debug.h>
 */
#ifndef _NET_NL_DEBUG_H
#define _NET_NL_DEBUG_H

#include <linux/skbuff.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/if_vlan.h>
#include <net/xdp.h>

/* ================================================================== */
/* §1  L2 — Ethernet                                                   */
/* ================================================================== */

/*
 * skb_eth_type - ethertype in network byte order.
 * skb->protocol is set by eth_type_trans() before netif_receive_skb;
 * valid from the rx handler downward.
 */
static inline __be16 skb_eth_type(const struct sk_buff *skb)
{
    return skb->protocol;
}

/*
 * skb_is_vlan_tagged - true if 802.1Q/802.1ad tag is present.
 *
 * Two cases to handle:
 *   a) Driver stripped the tag into skb->vlan_tci (most NICs with VLAN offload).
 *   b) Tag is still in the wire bytes and skb->protocol reflects it.
 */
static inline bool skb_is_vlan_tagged(const struct sk_buff *skb)
{
    return skb_vlan_tag_present(skb)              ||
           skb->protocol == htons(ETH_P_8021Q)   ||
           skb->protocol == htons(ETH_P_8021AD);
}

/*
 * skb_is_broadcast - true if dst MAC is FF:FF:FF:FF:FF:FF.
 * Requires mac_header set (eth_type_trans and below on RX).
 */
static inline bool skb_is_broadcast(const struct sk_buff *skb)
{
    return is_broadcast_ether_addr(eth_hdr(skb)->h_dest);
}

/*
 * skb_is_multicast_eth - true if dst MAC has the multicast bit set.
 * Includes broadcast. This is L2 multicast, distinct from IP multicast
 * (224.0.0.0/4) even though IP multicast maps to L2 multicast MACs.
 */
static inline bool skb_is_multicast_eth(const struct sk_buff *skb)
{
    return is_multicast_ether_addr(eth_hdr(skb)->h_dest);
}

/* ================================================================== */
/* §2  L4 — TCP / UDP                                                  */
/* ================================================================== */

/*
 * skb_is_ssh - true if skb carries TCP port 22 traffic.
 * Requires ip_hdr() and tcp_hdr() both valid.
 * Safe from: tcp_v4_rcv, ip_finish_output2 and below.
 * For ip_rcv (transport header not yet set), use ip_rcv_is_ssh().
 */
static inline bool skb_is_ssh(const struct sk_buff *skb)
{
    const struct iphdr *iph = ip_hdr(skb);
    if (iph->protocol == IPPROTO_TCP) {
        const struct tcphdr *th = tcp_hdr(skb);
        return ntohs(th->source) == 22 || ntohs(th->dest) == 22;
    }
    return false;
}

/*
 * ip_rcv_is_ssh - variant for ip_rcv() where transport_header
 * has NOT been set yet. Peeks manually past the IP header.
 */
static inline bool ip_rcv_is_ssh(const struct sk_buff *skb)
{
    const struct iphdr *iph = ip_hdr(skb);
    const __be16 *ports;
    if (iph->protocol != IPPROTO_TCP)
        return false;
    ports = (const __be16 *)((const u8 *)iph + iph->ihl * 4);
    if (!pskb_may_pull((struct sk_buff *)skb,
                       skb_network_offset(skb) + iph->ihl * 4 + 4))
        return false;
    return ntohs(ports[0]) == 22 || ntohs(ports[1]) == 22;
}

/*
 * skb_is_tcp_port - generic TCP src-or-dst port check.
 *
 * Examples:
 *   skb_is_tcp_port(skb, 80)    HTTP
 *   skb_is_tcp_port(skb, 443)   HTTPS
 *   skb_is_tcp_port(skb, 8080)  alt-HTTP
 */
static inline bool skb_is_tcp_port(const struct sk_buff *skb, u16 port)
{
    const struct iphdr *iph = ip_hdr(skb);
    if (iph->protocol == IPPROTO_TCP) {
        const struct tcphdr *th = tcp_hdr(skb);
        return ntohs(th->source) == port || ntohs(th->dest) == port;
    }
    return false;
}

/*
 * skb_is_udp_port - generic UDP src-or-dst port check.
 *
 * Examples:
 *   skb_is_udp_port(skb, 53)    DNS
 *   skb_is_udp_port(skb, 4789)  VXLAN
 *   skb_is_udp_port(skb, 6081)  GENEVE
 */
static inline bool skb_is_udp_port(const struct sk_buff *skb, u16 port)
{
    const struct iphdr *iph = ip_hdr(skb);
    if (iph->protocol == IPPROTO_UDP) {
        const struct udphdr *uh = udp_hdr(skb);
        return ntohs(uh->source) == port || ntohs(uh->dest) == port;
    }
    return false;
}

/* ================================================================== */
/* §3  XDP — xdp_buff helpers (kernel / driver side)                  */
/* ================================================================== */

/*
 * XDP runs before the skb is allocated. You work with raw pointers:
 *   xdp->data     — start of the Ethernet frame
 *   xdp->data_end — one byte past the end (bounds-check fence)
 *
 * Every pointer dereference MUST be bounds-checked against data_end.
 * This is enforced by the BPF verifier for BPF-side XDP programs,
 * and is equally required here for correctness and safety.
 *
 * These helpers are for kernel-side code only (native XDP hooks,
 * kfuncs). For BPF-side XDP programs use bpf_xdp_load_bytes().
 */

/*
 * xdp_ip_hdr - pointer to IPv4 header in an XDP frame.
 * Returns NULL if the frame is too short or not IPv4.
 */
static inline const struct iphdr *xdp_ip_hdr(const struct xdp_buff *xdp)
{
    const struct ethhdr *eth = xdp->data;
    const struct iphdr  *iph;

    if ((void *)(eth + 1) > xdp->data_end)
        return NULL;
    if (eth->h_proto != htons(ETH_P_IP))
        return NULL;
    iph = (const struct iphdr *)(eth + 1);
    if ((void *)(iph + 1) > xdp->data_end)
        return NULL;
    return iph;
}

/*
 * xdp_is_ssh - true if the XDP frame carries TCP port 22.
 */
static inline bool xdp_is_ssh(const struct xdp_buff *xdp)
{
    const struct iphdr  *iph = xdp_ip_hdr(xdp);
    const struct tcphdr *th;

    if (!iph || iph->protocol != IPPROTO_TCP)
        return false;
    th = (const struct tcphdr *)((const u8 *)iph + iph->ihl * 4);
    if ((void *)(th + 1) > xdp->data_end)
        return false;
    return ntohs(th->source) == 22 || ntohs(th->dest) == 22;
}

/*
 * xdp_is_udp_port - true if the XDP frame is UDP to/from given port.
 */
static inline bool xdp_is_udp_port(const struct xdp_buff *xdp, u16 port)
{
    const struct iphdr  *iph = xdp_ip_hdr(xdp);
    const struct udphdr *uh;

    if (!iph || iph->protocol != IPPROTO_UDP)
        return false;
    uh = (const struct udphdr *)((const u8 *)iph + iph->ihl * 4);
    if ((void *)(uh + 1) > xdp->data_end)
        return false;
    return ntohs(uh->source) == port || ntohs(uh->dest) == port;
}

/* ================================================================== */
/* §4  VXLAN — UDP/4789                                                */
/* ================================================================== */

/*
 * Wire layout (outer → inner):
 *
 *  ┌──────────┬──────────┬────────────────┬───────────┬─────────────────────┐
 *  │ Outer Eth│ Outer IP │ Outer UDP/4789 │ VXLAN(8B) │ Inner Eth + payload │
 *  └──────────┴──────────┴────────────────┴───────────┴─────────────────────┘
 *
 * VXLAN header (8 bytes):
 *   byte 0    : flags  (bit 3 = VNI valid)
 *   bytes 1-3 : reserved
 *   bytes 4-6 : VNI (24-bit, MSB first)
 *   byte 7    : reserved
 *
 * After udp_rcv decap, skb->data points at the VXLAN header.
 * The inner Ethernet frame starts at skb->data + 8.
 */

#define NL_VXLAN_PORT  4789

/*
 * skb_is_vxlan - true if outer UDP dst port is 4789.
 * Call before decap (e.g. from udp_rcv or ip_finish_output2).
 */
static inline bool skb_is_vxlan(const struct sk_buff *skb)
{
    return skb_is_udp_port(skb, NL_VXLAN_PORT);
}

/*
 * skb_vxlan_vni - extract the 24-bit VNI.
 * Only valid when skb->data points at the VXLAN header (post UDP pull).
 */
static inline u32 skb_vxlan_vni(const struct sk_buff *skb)
{
    const u8 *vxh = skb->data;
    return ((u32)vxh[4] << 16) | ((u32)vxh[5] << 8) | vxh[6];
}

/*
 * skb_vxlan_inner_eth - pointer to inner Ethernet header.
 * Returns NULL if the skb is too short.
 */
static inline const struct ethhdr *skb_vxlan_inner_eth(const struct sk_buff *skb)
{
    if (!pskb_may_pull((struct sk_buff *)skb, 8 + ETH_HLEN))
        return NULL;
    return (const struct ethhdr *)(skb->data + 8);
}

/* ================================================================== */
/* §5  GENEVE — UDP/6081                                               */
/* ================================================================== */

/*
 * Wire layout (outer → inner):
 *
 *  ┌──────────┬──────────┬─────────────────┬────────────────────┬──────────────────┐
 *  │ Outer Eth│ Outer IP │ Outer UDP/6081  │ GENEVE hdr(8+opts) │ Inner Eth/IP/... │
 *  └──────────┴──────────┴─────────────────┴────────────────────┴──────────────────┘
 *
 * GENEVE base header (8 bytes):
 *   byte 0    : version(2b) + opt_len(6b)   ← opt_len in 4-byte words
 *   byte 1    : flags
 *   bytes 2-3 : protocol type (inner ethertype)
 *   bytes 4-6 : VNI (24-bit)
 *   byte 7    : reserved
 *
 * Total header = 8 + (opt_len * 4) bytes. Options follow the base header.
 * Inner frame starts immediately after all options.
 */

#define NL_GENEVE_PORT  6081

/*
 * skb_is_geneve - true if outer UDP dst port is 6081.
 */
static inline bool skb_is_geneve(const struct sk_buff *skb)
{
    return skb_is_udp_port(skb, NL_GENEVE_PORT);
}

/*
 * skb_geneve_hdr_len - total GENEVE header length in bytes.
 * Returns 0 on pull failure.
 */
static inline unsigned int skb_geneve_hdr_len(const struct sk_buff *skb)
{
    const u8 *gnvh;
    if (!pskb_may_pull((struct sk_buff *)skb, 8))
        return 0;
    gnvh = skb->data;
    return 8 + ((gnvh[0] & 0x3f) * 4); /* base + options */
}

/*
 * skb_geneve_vni - 24-bit VNI from GENEVE header (bytes 4-6).
 */
static inline u32 skb_geneve_vni(const struct sk_buff *skb)
{
    const u8 *gnvh = skb->data;
    return ((u32)gnvh[4] << 16) | ((u32)gnvh[5] << 8) | gnvh[6];
}

/*
 * skb_geneve_inner_eth - pointer to inner Ethernet frame.
 * Returns NULL if header (including options) is truncated.
 */
static inline const struct ethhdr *skb_geneve_inner_eth(const struct sk_buff *skb)
{
    unsigned int hlen = skb_geneve_hdr_len(skb);
    if (!hlen || !pskb_may_pull((struct sk_buff *)skb, hlen + ETH_HLEN))
        return NULL;
    return (const struct ethhdr *)(skb->data + hlen);
}

#endif /* _NET_NL_DEBUG_H */
```

---

## Where each helper is called from

```
Layer       File you're patching          Helper to use
─────────── ──────────────────────────── ──────────────────────────
L2 RX       net/core/dev.c               skb_is_broadcast()
                                         skb_is_vlan_tagged()
L4 RX       net/ipv4/ip_input.c          ip_rcv_is_ssh()
L4 RX/TX    net/ipv4/tcp_output.c        skb_is_ssh()
            net/ipv4/ip_output.c         skb_is_tcp_port(skb, 80)
XDP         drivers/net/<nic>/...        xdp_is_ssh()
                                         xdp_is_udp_port(xdp, 4789)
VXLAN RX    net/vxlan/vxlan_core.c       skb_is_vxlan()
                                         skb_vxlan_vni()
                                         skb_vxlan_inner_eth()
GENEVE RX   drivers/net/geneve.c         skb_is_geneve()
                                         skb_geneve_vni()
                                         skb_geneve_inner_eth()
```

## One important thing about eBPF

eBPF programs run inside the **verifier sandbox** and cannot call any of these kernel C functions. For the BPF side you write separate `.bpf.c` files using `bpf_skb_load_bytes()` / `bpf_xdp_load_bytes()`. This header gets you the kernel C instrumentation; the BPF side needs its own set of helpers — worth doing as a separate file when you get there.

Good timing to expand into this. Let me cover the full picture — the BPF file structure, helpers, and the toolchain around it.

---

## The two worlds and why they're separate

```
┌─────────────────────────────────────────────────────────────┐
│                    Your kernel C code                        │
│   nl_debug.h helpers, pr_info trace points, kprobes         │
│   Compiled into the kernel. Can call anything.              │
└─────────────────────────────────────────────────────────────┘
              ↕  boundary: BPF syscall + verifier
┌─────────────────────────────────────────────────────────────┐
│                  BPF program (.bpf.c)                        │
│   Runs inside verifier sandbox. Restricted instruction set.  │
│   Cannot call arbitrary kernel functions.                    │
│   Can only call bpf_helpers — whitelisted by the kernel.    │
└─────────────────────────────────────────────────────────────┘
```

The BPF verifier is why you need separate helpers. Every pointer dereference must be explicitly bounds-checked or the verifier rejects your program at load time.

---

## File layout for your project

```
linux_kernel_net_playground/
│
├── kernel/                        ← your existing kernel C work
│   └── include/net/nl_debug.h
│
└── bpf/                           ← new: BPF side
    ├── nl_bpf_debug.h             ← shared BPF-side helpers (like nl_debug.h but for BPF)
    ├── nl_ssh_filter.bpf.c        ← XDP/TC program: SSH traffic tracer
    ├── nl_vxlan_inspect.bpf.c     ← XDP: VXLAN VNI extractor
    ├── nl_geneve_inspect.bpf.c    ← XDP: GENEVE inspector
    ├── nl_loader.c                ← userspace loader (libbpf)
    └── Makefile
```

---

## `bpf/nl_bpf_debug.h` — BPF-side equivalent of nl_debug.h

```c
/* SPDX-License-Identifier: GPL-2.0 */
/*
 * bpf/nl_bpf_debug.h — BPF-side packet parsing helpers
 *
 * Mirror of include/net/nl_debug.h but for .bpf.c programs.
 * These run inside the BPF verifier sandbox — every pointer
 * dereference is bounds-checked against ctx->data_end.
 *
 * Sections:
 *   §1  L2     — Ethernet parsing
 *   §2  L4     — TCP / UDP helpers
 *   §3  VXLAN  — outer + VNI extraction
 *   §4  GENEVE — outer + VNI extraction
 *
 * Include in your .bpf.c files:
 *   #include "nl_bpf_debug.h"
 */
#ifndef _NL_BPF_DEBUG_H
#define _NL_BPF_DEBUG_H

#include "vmlinux.h"          /* BTF-derived kernel types — generated once */
#include <bpf/bpf_helpers.h>  /* bpf_printk, bpf_skb_load_bytes, etc.     */
#include <bpf/bpf_endian.h>   /* bpf_ntohs, bpf_htons                     */

/* ================================================================== */
/* §1  L2 — Ethernet                                                   */
/* ================================================================== */

/*
 * nl_parse_eth - pull Ethernet header from XDP context.
 *
 * Returns pointer to ethhdr, advances *offset past it.
 * Returns NULL if frame is too short — verifier requires this check.
 *
 *  void *data     = (void *)(long)ctx->data;
 *  void *data_end = (void *)(long)ctx->data_end;
 */
static __always_inline struct ethhdr *
nl_parse_eth(void *data, void *data_end, int *offset)
{
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return NULL;
    *offset = sizeof(*eth);
    return eth;
}

/* ================================================================== */
/* §2  L4 — TCP / UDP                                                  */
/* ================================================================== */

static __always_inline struct iphdr *
nl_parse_ip(void *data, void *data_end, int offset)
{
    struct iphdr *iph = data + offset;
    if ((void *)(iph + 1) > data_end)
        return NULL;
    return iph;
}

static __always_inline struct tcphdr *
nl_parse_tcp(void *data, void *data_end, struct iphdr *iph, int ip_offset)
{
    struct tcphdr *th;
    int transport_offset = ip_offset + iph->ihl * 4;
    th = data + transport_offset;
    if ((void *)(th + 1) > data_end)
        return NULL;
    return th;
}

static __always_inline struct udphdr *
nl_parse_udp(void *data, void *data_end, struct iphdr *iph, int ip_offset)
{
    struct udphdr *uh;
    int transport_offset = ip_offset + iph->ihl * 4;
    uh = data + transport_offset;
    if ((void *)(uh + 1) > data_end)
        return NULL;
    return uh;
}

static __always_inline bool nl_is_ssh_xdp(void *data, void *data_end)
{
    int offset = 0;
    struct ethhdr *eth = nl_parse_eth(data, data_end, &offset);
    if (!eth || eth->h_proto != bpf_htons(0x0800))  /* ETH_P_IP */
        return false;
    struct iphdr *iph = nl_parse_ip(data, data_end, offset);
    if (!iph || iph->protocol != 6)                 /* IPPROTO_TCP */
        return false;
    struct tcphdr *th = nl_parse_tcp(data, data_end, iph, offset);
    if (!th)
        return false;
    return bpf_ntohs(th->source) == 22 || bpf_ntohs(th->dest) == 22;
}

/* ================================================================== */
/* §3  VXLAN — UDP/4789                                                */
/* ================================================================== */

#define NL_BPF_VXLAN_PORT  4789

/*
 * VXLAN header layout (8 bytes):
 *   [0]   flags  (bit3 = VNI valid)
 *   [1-3] reserved
 *   [4-6] VNI (24-bit, big-endian)
 *   [7]   reserved
 *
 * Followed immediately by inner Ethernet frame.
 */
struct nl_vxlanhdr {
    __u8  flags;
    __u8  rsvd1[3];
    __u8  vni[3];
    __u8  rsvd2;
} __attribute__((packed));

static __always_inline __u32
nl_vxlan_vni(void *data, void *data_end, int vxlan_offset)
{
    struct nl_vxlanhdr *vxh = data + vxlan_offset;
    if ((void *)(vxh + 1) > data_end)
        return 0;
    return ((__u32)vxh->vni[0] << 16) |
           ((__u32)vxh->vni[1] <<  8) |
            (__u32)vxh->vni[2];
}

static __always_inline bool nl_is_vxlan(struct udphdr *uh)
{
    return bpf_ntohs(uh->dest) == NL_BPF_VXLAN_PORT;
}

/* ================================================================== */
/* §4  GENEVE — UDP/6081                                               */
/* ================================================================== */

#define NL_BPF_GENEVE_PORT  6081

/*
 * GENEVE base header (8 bytes):
 *   [0]   version(2b) + opt_len(6b)   ← opt_len in 4-byte units
 *   [1]   flags
 *   [2-3] protocol type (inner ethertype)
 *   [4-6] VNI (24-bit)
 *   [7]   reserved
 *
 * Total header = 8 + (opt_len * 4). Inner frame follows.
 */
struct nl_genevehdr {
    __u8  opt_len_ver;   /* upper 2 bits = version, lower 6 = opt_len */
    __u8  flags;
    __be16 proto_type;
    __u8  vni[3];
    __u8  rsvd;
} __attribute__((packed));

static __always_inline bool nl_is_geneve(struct udphdr *uh)
{
    return bpf_ntohs(uh->dest) == NL_BPF_GENEVE_PORT;
}

static __always_inline __u32
nl_geneve_vni(void *data, void *data_end, int geneve_offset)
{
    struct nl_genevehdr *gnvh = data + geneve_offset;
    if ((void *)(gnvh + 1) > data_end)
        return 0;
    return ((__u32)gnvh->vni[0] << 16) |
           ((__u32)gnvh->vni[1] <<  8) |
            (__u32)gnvh->vni[2];
}

#endif /* _NL_BPF_DEBUG_H */
```

---

## `bpf/nl_ssh_filter.bpf.c` — example XDP program using the header

```c
/* SPDX-License-Identifier: GPL-2.0 */
#include "nl_bpf_debug.h"

/*
 * XDP hook: trace non-SSH TCP and flag VXLAN/GENEVE tunnels.
 * Attach to an interface:
 *   sudo ip link set dev eth0 xdp obj nl_ssh_filter.bpf.o sec xdp
 */
SEC("xdp")
int nl_xdp_trace(struct xdp_md *ctx)
{
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;
    int   offset   = 0;

    struct ethhdr *eth = nl_parse_eth(data, data_end, &offset);
    if (!eth)
        return XDP_PASS;

    if (eth->h_proto != bpf_htons(0x0800))   /* not IPv4 */
        return XDP_PASS;

    struct iphdr *iph = nl_parse_ip(data, data_end, offset);
    if (!iph)
        return XDP_PASS;

    /* ── TCP ── */
    if (iph->protocol == 6) {
        struct tcphdr *th = nl_parse_tcp(data, data_end, iph, offset);
        if (th && bpf_ntohs(th->source) != 22 && bpf_ntohs(th->dest) != 22) {
            bpf_printk("[NL-XDP] TCP sport=%u dport=%u\n",
                       bpf_ntohs(th->source), bpf_ntohs(th->dest));
        }
    }

    /* ── UDP: detect VXLAN / GENEVE ── */
    if (iph->protocol == 17) {
        struct udphdr *uh = nl_parse_udp(data, data_end, iph, offset);
        if (!uh)
            return XDP_PASS;

        int udp_payload_offset = offset + iph->ihl * 4 + sizeof(*uh);

        if (nl_is_vxlan(uh)) {
            __u32 vni = nl_vxlan_vni(data, data_end, udp_payload_offset);
            bpf_printk("[NL-XDP] VXLAN VNI=%u\n", vni);
        }

        if (nl_is_geneve(uh)) {
            __u32 vni = nl_geneve_vni(data, data_end, udp_payload_offset);
            bpf_printk("[NL-XDP] GENEVE VNI=%u\n", vni);
        }
    }

    return XDP_PASS;
}

char LICENSE[] SEC("license") = "GPL";
```

---

## Tools and frameworks — the full ecosystem

```
┌──────────────────────────────────────────────────────────────────────┐
│  WRITE                                                               │
│                                                                      │
│  libbpf  (C, low-level)  ← what you use; direct, minimal, standard  │
│  bpftool               ← inspect maps, programs, BTF from CLI        │
│  BCC  (Python/Lua frontend) ← rapid prototyping, not for production │
│  cilium/ebpf  (Go)     ← if you ever want a Go loader               │
└──────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────────────┐
│  BUILD                                                               │
│                                                                      │
│  clang + llvm   ← compile .bpf.c → .bpf.o (BPF bytecode ELF)       │
│  bpftool btf    ← generate vmlinux.h (one-time per kernel build)     │
│  bpf2go  (Go)  ← auto-generates Go bindings from .bpf.o             │
└──────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────────────┐
│  OBSERVE                                                             │
│                                                                      │
│  cat /sys/kernel/debug/tracing/trace_pipe  ← read bpf_printk output │
│  bpftool prog show                         ← list loaded BPF progs  │
│  bpftool map dump id <N>                   ← inspect BPF maps        │
└──────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────────────┐
│  HIGHER-LEVEL FRAMEWORKS  (build on top of libbpf)                  │
│                                                                      │
│  Cilium    ← production eBPF CNI, Kubernetes networking              │
│  Katran    ← Facebook's L4 load balancer in XDP                     │
│  Falco     ← security observability via BPF                          │
│  Pixie     ← Kubernetes observability                                │
└──────────────────────────────────────────────────────────────────────┘
```

For your learning path — **libbpf + bpftool** is the right choice. BCC is easier to start with but hides too much of what you're trying to learn.

---

## `bpf/Makefile`

```makefile
# Requires: clang, llvm, libbpf-dev, bpftool

CLANG   := clang
BPFTOOL := bpftool
CFLAGS  := -O2 -g -target bpf -D__TARGET_ARCH_x86

# Generate vmlinux.h once (run manually: make vmlinux.h)
vmlinux.h:
	$(BPFTOOL) btf dump file /sys/kernel/btf/vmlinux format c > $@

%.bpf.o: %.bpf.c nl_bpf_debug.h vmlinux.h
	$(CLANG) $(CFLAGS) -c $< -o $@

all: nl_ssh_filter.bpf.o nl_vxlan_inspect.bpf.o nl_geneve_inspect.bpf.o

clean:
	rm -f *.bpf.o vmlinux.h
```

---

## Workflow to see output

```bash
# 1. Generate vmlinux.h from your running custom kernel
make vmlinux.h

# 2. Compile
make

# 3. Attach to interface
sudo ip link set dev enp1s0 xdp obj nl_ssh_filter.bpf.o sec xdp

# 4. Watch output (equivalent of dmesg for BPF)
sudo cat /sys/kernel/debug/tracing/trace_pipe

# 5. Detach when done
sudo ip link set dev enp1s0 xdp off
```

The `bpf_printk` output goes to `trace_pipe`, while your kernel-side `pr_info` goes to `dmesg` — you can watch both simultaneously in separate terminals to see the same packet from both vantage points.