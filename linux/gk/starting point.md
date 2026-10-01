# Linux Network Development Roadmap for Beginners

Great starting point! Here's a structured path from C basics → Linux net dev.

---

## Stage 1 — Strengthen Your C First
*Don't skip this. Kernel code will punish weak C knowledge.*

| Topic | Why It Matters |
|---|---|
| Pointers & pointer arithmetic | Everything in kernel is pointer-based |
| Structs & function pointers | Driver APIs are struct-of-function-pointers |
| Memory management (malloc/free) | Prepares you for kmalloc/kfree mindset |
| Bitwise operations | Network headers are bit-packed |
| `sizeof`, `offsetof`, alignment | Packet parsing requires exact sizes |

**Practice:** Implement a linked list, a simple memory allocator, a byte-level packet parser in plain C.

---

## Stage 2 — Linux & System Basics
*Before touching the kernel, get comfortable with Linux itself.*

```bash
# Things you must be fluent with
- File system navigation, permissions
- gcc, make, Makefiles
- gdb (debugger)
- strace, ltrace (syscall tracing)
- ip, ss, tcpdump, netstat (network tools)
- dmesg (kernel logs — your printk output goes here)
```

---

## Stage 3 — Socket Programming (Userspace First)
*This is where most people start — and it's the right place.*

```c
// Your first goal: write a TCP server + client from scratch
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>

// Learn these syscalls deeply:
// socket() → bind() → listen() → accept() → read()/write() → close()
// socket() → connect() → read()/write()  (client side)
```

**Projects at this stage:**
- Echo server (TCP)
- Simple HTTP GET client
- UDP ping-pong
- Port scanner

---

## Stage 4 — Raw Sockets & Packet Crafting
*This bridges userspace networking with how the kernel actually works.*

```c
// Raw socket — you build the entire packet yourself
int sock = socket(AF_PACKET, SOCK_RAW, htons(ETH_P_ALL));

// You'll manually construct:
// [ Ethernet header | IP header | TCP/UDP header | Payload ]
```

**Learn:**
- Ethernet frame structure
- IP header fields (TTL, protocol, checksum)
- TCP/UDP headers
- `struct iphdr`, `struct tcphdr`, `struct ethhdr` from `<linux/ip.h>`, `<linux/tcp.h>`

---

## Stage 5 — Kernel Module Basics
*Now you step into kernel space.*

```c
#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>

static int __init hello_init(void) {
    printk(KERN_INFO "Hello, kernel!\n");
    return 0;
}

static void __exit hello_exit(void) {
    printk(KERN_INFO "Goodbye, kernel!\n");
}

module_init(hello_init);
module_exit(hello_exit);
MODULE_LICENSE("GPL");
```

**Key skills:**
- Writing a `Makefile` for modules
- `insmod` / `rmmod` / `lsmod`
- Reading `dmesg` output
- Kernel memory (`kmalloc`, `kfree`, `GFP_KERNEL`)

---

## Stage 6 — Linux Network Kernel Internals
*The actual destination.*

```
Network Stack (top → bottom):
─────────────────────────────
  Application (userspace)
       ↕ syscalls
  Socket layer       → struct socket, struct sock
       ↕
  Protocol layer     → TCP: tcp_sendmsg(), UDP: udp_sendmsg()
       ↕
  IP layer           → ip_output(), ip_rcv()
       ↕
  Netfilter hooks    → iptables, nftables live here
       ↕
  Network device     → struct net_device, struct net_device_ops
       ↕
  Driver (hardware)  → NIC driver, NAPI polling
─────────────────────────────
```

**Things to build at this stage:**
- A **Netfilter hook** (intercept/drop/modify packets)
- A simple **virtual network device** (`net_device`)
- A custom **socket option** via `setsockopt`

---

## Learning Resources (in order)

| Resource | What You Get |
|---|---|
| **Beej's Guide to Network Programming** | Best socket programming resource, free online |
| **"Linux Kernel Development" — Robert Love** | Gentle intro to kernel internals |
| **"Understanding Linux Network Internals" — Benvenuti** | Deep dive into the net stack |
| **"Linux Device Drivers" (LDD3)** | Free online, classic kernel module guide |
| **Kernel source on Elixir** (elixir.bootlin.com) | Browse + search kernel source online |

---

## Your First 30-Day Plan

```
Week 1 → C pointers, structs, bitwise ops — code every day
Week 2 → TCP echo server + UDP client in userspace sockets
Week 3 → Raw socket packet sniffer (like mini-tcpdump)
Week 4 → First kernel module → Hello World → then a Netfilter hook
```

---

## Setup Advice

> **Always develop kernel modules in a VM** (QEMU or VirtualBox).
> A bad module can **kernel panic** and corrupt your system.

```bash
# Minimal setup
sudo apt install build-essential linux-headers-$(uname -r) qemu
```

---

Want me to give you a working **raw socket packet sniffer** or a **Netfilter hook** as your first real project?