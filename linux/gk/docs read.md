# Reading Docs for `linux/<header>` Files

Short answer: **man pages don't cover kernel headers directly.** Here's what actually works:

---

## 1. `man 2` — System Calls (closest thing)

Man pages cover **syscalls**, not headers. But syscall pages mention the headers you need:

```bash
man 2 socket       # documents socket(), mentions <sys/socket.h>
man 2 send         # send(), recv(), flags
man 2 ioctl        # ioctl() — heavily used in net drivers
man 7 ip           # IP socket options
man 7 tcp          # TCP socket options
man 7 packet       # AF_PACKET raw sockets ← very useful for net dev
man 7 netdevice    # network device ioctls
```

```bash
# Section numbers
man 1 = commands
man 2 = syscalls       ← your main reference
man 3 = libc functions
man 7 = protocols, conventions  ← also very useful
```

---

## 2. Kernel Source Directly — The Real Docs

```bash
# Install kernel source / headers
sudo apt install linux-source
sudo apt install linux-headers-$(uname -r)

# Headers are here:
/usr/src/linux-headers-$(uname -r)/include/linux/
```

```bash
# Just read the header file itself — they're well commented
cat /usr/src/linux-headers-$(uname -r)/include/linux/skbuff.h
cat /usr/src/linux-headers-$(uname -r)/include/linux/netdevice.h
```

---

## 3. Elixir Cross-Referencer (Best Online Tool)

**https://elixir.bootlin.com/linux/latest/source/include/linux**

- Browse any kernel header online
- **Click any struct/function** → jumps to definition
- See **where it's used** across the entire kernel
- No setup needed

```
# Example URLs:
https://elixir.bootlin.com/linux/latest/source/include/linux/skbuff.h
https://elixir.bootlin.com/linux/latest/source/include/linux/netdevice.h
https://elixir.bootlin.com/linux/latest/source/include/linux/tcp.h
```

---

## 4. Kernel Docs Site

**https://docs.kernel.org**

```
https://docs.kernel.org/networking/         ← networking subsystem docs
https://docs.kernel.org/driver-api/         ← driver API reference
https://docs.kernel.org/core-api/           ← core kernel APIs
```

---

## 5. `kernel-doc` Comments in Source

Kernel developers write Javadoc-style comments above functions:

```bash
# Search documentation comments in source
grep -r "* @" /usr/src/linux-headers-$(uname -r)/include/linux/skbuff.h
```

They look like:
```c
/**
 * skb_put - add data to a buffer
 * @skb: buffer to use
 * @len: amount of data to add
 *
 * This function extends the used data area of the buffer.
 * Returns: pointer to the first byte of the extra data.
 */
```

---

## Quick Reference Table

| What you want | Where to look |
|---|---|
| Syscall usage | `man 2 <syscall>` |
| Protocol behavior | `man 7 <protocol>` |
| Struct/function definition | elixir.bootlin.com |
| Conceptual guides | docs.kernel.org/networking |
| Raw header + comments | `/usr/src/linux-headers-.../include/linux/` |
| Real usage examples | grep the kernel source itself |

---

## Most Useful `man 7` Pages for Net Dev

```bash
man 7 socket
man 7 ip
man 7 ipv6
man 7 tcp
man 7 udp
man 7 packet      # raw sockets / AF_PACKET
man 7 netlink     # kernel↔userspace communication
man 7 netdevice   # ioctl-based device control
```

> **Bookmark elixir.bootlin.com** — it will be your most-used tool as a kernel net developer.