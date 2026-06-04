I am learning linux kernel net. so which is the very basic net protocol that I can learn kernel level? I need to understand data structure and algorithms using, how to relate with kernel docs - for example how to write each line of code by refer the kernel docs, for example using inbuild modules, or .h files, functions. how to understand rfc, official docs, language specific knowledge, for example if I use c or rust, how to get to know which kind of things I use when code each line?

## Start Here: ICMP (RFC 792)

Here is my reasoning. You need a protocol that is stateless, has a short RFC, has a small codebase in the kernel, and is immediately testable with standard tools. ICMP satisfies all four. There is no connection state machine, no session management, and no complex congestion control. You can trigger the exact code path with a single `ping` command and observe it live. Everything else — TCP, UDP, BGP — builds on the same foundational mechanics you will learn here.

The kernel file is `net/ipv4/icmp.c`. It is roughly 1,500 lines. That is readable.

---

## The Core Learning Method: RFC → Header → Code

This is the workflow you should internalize for every protocol you ever study. It has three layers and they map directly onto each other.

**Layer 1 — RFC: What the protocol must do (behavior)**

Open RFC 792. Find the Echo Request message format. It looks like this:

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     Type      |     Code      |          Checksum             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|           Identifier          |        Sequence Number        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     Data ...
```

The RFC says: Type is 8 for echo request, 0 for echo reply. Code is 0. Checksum covers the ICMP header and data. The identifier and sequence number are used by the sender to match replies.

That is the specification. Now go find the kernel's data model for it.

**Layer 2 — Kernel Header: How the kernel models the RFC in C**

Open `include/linux/icmp.h`:

```c
struct icmphdr {
    __u8        type;
    __u8        code;
    __sum16     checksum;
    union {
        struct {
            __be16  id;
            __be16  sequence;
        } echo;
        __be32  gateway;
        struct {
            __be16  __unused;
            __be16  mtu;
        } frag;
        __u8    reserved[4];
    } un;
};
```

Notice the direct RFC-to-struct mapping. `type` → Type field. `code` → Code field. `un.echo.id` → Identifier. `un.echo.sequence` → Sequence Number. The union is the kernel's way of saying "the remaining 4 bytes mean different things for different ICMP types."

Now notice the types. `__be16` is not `uint16_t`. `__sum16` is not `uint16_t`. These are semantic type annotations the kernel uses to catch byte-order bugs at compile time. `__be16` means "big-endian 16-bit" — which is network byte order per RFC. The kernel enforces this distinction because a common bug class is reading a big-endian field directly as a native integer on a little-endian machine.

**Layer 3 — Kernel Source: How the kernel implements the RFC behavior**

Open `net/ipv4/icmp.c`. Find `icmp_rcv()`. This is the function called for every incoming ICMP packet. Look at the first few meaningful lines:

```c
static enum skb_drop_reason icmp_rcv(struct sk_buff *skb)
{
    struct icmphdr *icmph;
    struct rtable *rt;
    struct net *net;
    
    /* ... early validation ... */
    
    icmph = icmp_hdr(skb);
    
    /* ... checksum verification ... */
    
    switch (icmph->type) {
    case ICMP_ECHOREPLY:
        ping_rcv(skb);
        goto drop_no_count;
    case ICMP_ECHO:
        if (!sysctl_icmp_echo_ignore_all)
            icmp_echo(skb);
        break;
    /* ... other types ... */
    }
}
```

`icmp_hdr(skb)` is a macro. Go look at it. It expands to a cast of the transport header pointer inside `sk_buff`. This brings us to the most important data structure in the entire kernel network stack.

---

## The Most Critical Data Structure: `sk_buff`

Before you understand any protocol, you must understand `sk_buff`. It is defined in `include/linux/skbuff.h` and it is the universal packet container. Every packet in the kernel — incoming, outgoing, being processed — lives inside an `sk_buff`.

The key design insight is this: **the kernel does not copy packet data as it passes through layers.** Instead, it maintains a set of pointers into a single buffer and adjusts them as headers are added or removed.

```
skb->head  --------> [headroom | mac_header | network_header | transport_header | data | tailroom]
                                     ^                ^                  ^
                               skb->mac_header  skb->network_header  skb->transport_header
                                                                           ^
                                                                       skb->data
```

When the IP layer hands off to ICMP, it calls `skb_pull()` which simply advances `skb->data` past the IP header. ICMP now sees its header at `skb->data`. No copy happened. The `icmp_hdr(skb)` macro returns a pointer directly into this buffer.

This is the zero-copy architecture of the kernel network stack. The algorithm is pointer manipulation, not data movement.

The fields you need to understand first in `struct sk_buff` (from `include/linux/skbuff.h`):

```c
struct sk_buff {
    /* ... */
    struct net_device   *dev;       /* the network device this came from */
    sk_buff_data_t      transport_header; /* offset to L4 header */
    sk_buff_data_t      network_header;   /* offset to L3 header */
    sk_buff_data_t      mac_header;       /* offset to L2 header */
    sk_buff_data_t      tail;
    sk_buff_data_t      end;
    unsigned char       *head;
    unsigned char       *data;
    unsigned int        len;        /* length of actual data */
    /* ... */
};
```

---

## How ICMP Registers Itself With the IP Layer

The kernel needs a dispatch mechanism. When an IP packet arrives, how does `ip_input.c` know to call `icmp_rcv()` for protocol number 1?

Look in `net/ipv4/icmp.c` for:

```c
static const struct net_protocol icmp_protocol = {
    .handler     = icmp_rcv,
    .err_handler = icmp_err,
    .no_policy   = 1,
    .icmp_strict_tag_validation = 1,
};
```

And in `icmp_init()`:

```c
inet_add_protocol(&icmp_protocol, IPPROTO_ICMP);
```

`inet_add_protocol()` is defined in `net/ipv4/protocol.c`. It registers a handler in a global table keyed by IP protocol number. When `ip_local_deliver_finish()` processes a packet, it looks up protocol number in that table and calls the handler. That is the dispatch algorithm.

This pattern repeats for every IP protocol. TCP registers `tcp_protocol`. UDP registers `udp_protocol`. Go look at `net/ipv4/af_inet.c` and `net/ipv4/protocol.c` to see the full registration infrastructure.

---

## Tools for Navigating Kernel Source

**Bootlin Elixir** (`elixir.bootlin.com`) — use this first. It is a cross-reference browser for the kernel source. You can click on any symbol and immediately see every definition, declaration, and usage across the entire tree. This is faster than `grep` for exploration.

**`cscope`** — for local navigation when you have the source checked out. Run `make cscope` in the kernel tree to generate the database. Then use it with vim or emacs to jump between callers and definitions.

**`git log --follow -p net/ipv4/icmp.c`** — see the full history of changes to a file. Commit messages written by maintainers explain *why* changes were made. These are better documentation than comments.

**`Documentation/networking/`** — start with `Documentation/networking/kapi.rst` and `Documentation/networking/skbuff.rst`. These are the canonical references for the data structures we discussed.

**`scripts/decode_stacktrace.sh`** — you will need this later when kernel oops messages appear.

---

## Guiding Questions — Think Through These

These are not rhetorical. Work through each one before the next conversation.

1. A `ping` packet arrives at your NIC. Name the exact call chain from the driver's interrupt handler to `icmp_rcv()`. What are the function names at each layer? Use Bootlin or `grep` to trace it. Start from `netif_receive_skb()`.

2. `icmp_hdr(skb)` gives you a pointer directly into the packet buffer. What happens if you modify a field through that pointer on a received packet? Is that safe? What about shared `sk_buff` clones?

3. Why does `struct icmphdr` use `__be16` for `id` and `sequence` but `__u8` for `type` and `code`? What is the difference and why does byte order only matter for multi-byte fields?

4. The checksum in `struct icmphdr` is `__sum16`. The RFC says the checksum is computed over the entire ICMP message. Find the kernel function that computes this checksum. What algorithm does it use? Why is Internet checksum faster than CRC for this purpose?

5. When `icmp_echo()` builds the reply, does it allocate a new `sk_buff` or reuse the incoming one? Look at the code. What are the implications of each choice for performance?

6. What is `struct net_device`? When `sk_buff` carries a pointer to `dev`, what information does that give the ICMP layer?

Work through question 1 first. Tracing the call chain from interrupt to ICMP handler will give you the mental map of every layer the packet crosses, and every data structure will make sense in context after that.

