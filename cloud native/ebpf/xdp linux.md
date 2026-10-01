# eBPF + XDP: the working basics

**Analogy:** eBPF is like a safe plugin system inside the Linux kernel. You write a small program, the kernel's **verifier** inspects it to prove it can't crash anything, and then it runs at a hook point. XDP is the earliest hook on the network receive path, a bouncer at the door of the NIC driver that decides what happens to each packet before the kernel spends any effort on it.

## Where XDP sits

```
 NIC ──► driver RX ──► [ XDP hook ] ──► skb alloc ──► TC ──► netfilter ──► socket
                           │
                           ├─ XDP_DROP      discard here (cheapest)
                           ├─ XDP_PASS      continue to normal stack
                           ├─ XDP_TX        bounce out the same NIC
                           ├─ XDP_REDIRECT  send to another NIC/CPU/AF_XDP socket
                           └─ XDP_ABORTED   drop + trace error
```

## The workflow

```
 prog.bpf.c ──clang -target bpf──► prog.bpf.o ──load──► VERIFIER ──► JIT ──► attached to iface
                                                           │
                                         rejects unsafe code (bad pointers,
                                         unbounded loops, unchecked reads)
```

## 1. Setup (Ubuntu/Debian)

```bash
sudo apt install clang llvm libbpf-dev linux-tools-$(uname -r) \
                 linux-headers-$(uname -r) xdp-tools
```

You need a kernel of roughly 5.x or newer. BTF enabled (`/sys/kernel/btf/vmlinux` exists) makes life easier.

## 2. A minimal XDP program

`drop_udp.bpf.c`: drops all UDP packets, passes everything else.

```c
#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

SEC("xdp")
int drop_udp(struct xdp_md *ctx)
{
    void *data     = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)          // bounds check: REQUIRED
        return XDP_PASS;

    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return XDP_PASS;

    struct iphdr *ip = (void *)(eth + 1);
    if ((void *)(ip + 1) > data_end)           // bounds check again
        return XDP_PASS;

    if (ip->protocol == IPPROTO_UDP)
        return XDP_DROP;

    return XDP_PASS;
}

char LICENSE[] SEC("license") = "GPL";
```

## 3. Compile, load, inspect

```bash
# compile
clang -O2 -g -target bpf -c drop_udp.bpf.c -o drop_udp.bpf.o

# attach (generic/skb mode works anywhere; use `xdp` for native driver mode)
sudo ip link set dev eth0 xdpgeneric obj drop_udp.bpf.o sec xdp

# verify
ip link show eth0            # shows "xdpgeneric" and a prog id
sudo bpftool prog show
sudo bpftool net

# detach
sudo ip link set dev eth0 xdpgeneric off
```

**Tip:** test on a veth pair or VM first. A bad program on your only NIC can cut your SSH session.

## 4. XDP attach modes

| Mode | Flag | Speed | Requirement |
|---|---|---|---|
| Generic | `xdpgeneric` | Slowest (after skb alloc) | Any NIC |
| Native | `xdp` / `xdpdrv` | Fast | Driver support |
| Offload | `xdpoffload` | Fastest (runs on NIC) | SmartNIC |

## 5. Maps: talking to userspace

Programs are stateless between packets. **Maps** are the shared memory between kernel programs and userspace.

```
  XDP prog ──write──► [ BPF map ] ◄──read── userspace (counters, config, flow tables)
```

Common types: `BPF_MAP_TYPE_ARRAY`, `HASH`, `PERCPU_ARRAY` (lock-free counters), `LRU_HASH`, `RINGBUF` (events to userspace), `DEVMAP` (for redirect).

```c
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u64);
} pkt_count SEC(".maps");

// inside the program:
__u32 key = 0;
__u64 *val = bpf_map_lookup_elem(&pkt_count, &key);
if (val) (*val)++;                      // NULL check is required by the verifier
```

Read it with `sudo bpftool map dump name pkt_count`.

## 6. Tooling choices for a real project

| Approach | Best for |
|---|---|
| **libbpf + skeleton** (C) | Production, small binaries, CO-RE portability |
| **xdp-tools / xdp-tutorial** | Learning XDP specifically |
| **Cilium ebpf** (Go) | Go userspace loaders |
| **Aya** (Rust) | Rust end to end |
| **bpftrace / BCC** | Tracing and quick experiments (not XDP-focused) |

With libbpf, `bpftool gen skeleton prog.bpf.o > prog.skel.h` generates a header so your userspace C code can open, load, and attach with a few calls.

## 7. Common verifier pain points

- **Every packet pointer access needs a bounds check** against `data_end`, immediately before use.
- **Loops must be provably bounded.** Use constant bounds or `bpf_loop()` on newer kernels.
- **Map lookups return nullable pointers**, so check for NULL.
- **Stack is only 512 bytes**, so use per-CPU maps for big scratch buffers.
- **No arbitrary function calls**, only BPF helpers and `static __always_inline` functions.
- Read the verifier log when it rejects you: `bpftool -d prog load ...` or `LIBBPF_DEBUG`.

## 8. Debugging

```bash
sudo bpftool prog tracelog          # shows bpf_printk() output
sudo xdpdump -i eth0                # packet capture at XDP level
sudo bpftool prog profile id <ID> duration 5 cycles instructions
```

`bpf_printk("proto=%d", ip->protocol);` is your printf, but it's slow, so use it only for debugging.

## Suggested learning path

1. Do the **xdp-tutorial** repo (github.com/xdp-project/xdp-tutorial). It's the best hands-on course.
2. Build a packet counter with a per-CPU map and a userspace reader.
3. Parse further (TCP/UDP ports) and implement a simple blocklist with a hash map.
4. Try `XDP_REDIRECT` and AF_XDP for userspace packet processing.

Want me to go deeper on one piece next, such as the libbpf skeleton workflow, map types, or how the verifier reasons about pointers?

# Linux Kernel → eBPF → XDP: Complete Foundations

This is a reference map in the order you should learn things. Each part builds on the previous one. Don't skip Part 1 and 2; eBPF only makes sense once you know what the kernel is.

```
PART 1  Kernel basics            what the kernel is, what it does
PART 2  Networking basics        how a packet travels through Linux
PART 3  eBPF concepts            what eBPF is and how it works
PART 4  eBPF building blocks     program types, maps, helpers, verifier
PART 5  XDP                      eBPF at the NIC driver
PART 6  Toolchain                compile, load, inspect
PART 7  Writing programs         C, bounds checks, restrictions
PART 8  Advanced topics          CO-RE, tail calls, AF_XDP, etc.
PART 9  Debugging & security
PART 10 Learning roadmap
```

---

# PART 1: Linux kernel basics

## 1.1 What is the kernel?

**Analogy:** The kernel is the building manager. Programs (tenants) never touch the electricity, water, or locks directly. They ask the manager, and the manager decides.

```
 ┌───────────────────────────────────────────┐
 │  User space   (apps: nginx, bash, python) │  restricted, can't touch hardware
 ├───────────────────────────────────────────┤
 │        System call interface (syscalls)   │  the "front desk"
 ├───────────────────────────────────────────┤
 │  Kernel space                             │  full power
 │  ┌─────────┬─────────┬─────────┬───────┐  │
 │  │ Process │ Memory  │ File    │Network│  │
 │  │ sched.  │ mgmt    │ systems │ stack │  │
 │  └─────────┴─────────┴─────────┴───────┘  │
 │            Device drivers                 │
 ├───────────────────────────────────────────┤
 │  Hardware (CPU, RAM, disk, NIC)           │
 └───────────────────────────────────────────┘
```

## 1.2 Core concepts

| Concept | Plain English |
|---|---|
| **User space vs kernel space** | Two privilege zones. Kernel code can do anything; user code is fenced in. |
| **Privilege rings** | CPU hardware enforces it (x86: ring 0 = kernel, ring 3 = user). |
| **System call (syscall)** | The only legal way for user code to ask the kernel for something (`read`, `write`, `socket`, `bpf`). |
| **Process / task** | A running program. The kernel tracks each as a `task_struct`. |
| **Thread** | A process-like unit sharing memory with siblings. |
| **Scheduler** | Decides which task runs on which CPU and for how long. |
| **Virtual memory** | Each process sees its own private address space; the kernel maps it to real RAM via page tables. |
| **Interrupt** | Hardware tapping the CPU on the shoulder ("a packet arrived!"). |
| **Softirq / NAPI** | Deferred interrupt work; how the network stack batches packet processing. |
| **Context switch** | CPU stops one task and starts another. Costly. |
| **Kernel module** | Code loaded into the running kernel (`.ko`). Powerful but dangerous: a bug crashes the whole machine. |
| **Device driver** | Kernel code that talks to a specific piece of hardware (e.g. your NIC). |
| **procfs / sysfs / debugfs** | Virtual filesystems exposing kernel state (`/proc`, `/sys`, `/sys/kernel/debug`). |
| **Kernel version / config** | `uname -r`; features depend on `CONFIG_*` options (e.g. `CONFIG_BPF_SYSCALL`). |

## 1.3 The problem eBPF solves

Historically, to add behavior to the kernel (tracing, filtering, custom networking) you had two options:

```
 Option A: Change kernel source   → slow: upstream review, wait years, rebuild everywhere
 Option B: Write a kernel module  → fast, but UNSAFE: one bug = kernel panic
```

**eBPF is Option C:** load small, *verified-safe* programs into the running kernel, no reboot, no module.

---

# PART 2: Networking basics (needed for XDP)

## 2.1 The layers

```
 Layer 7  Application   HTTP, DNS, TLS payload
 Layer 4  Transport     TCP, UDP  (ports)
 Layer 3  Network       IP        (addresses)
 Layer 2  Data link     Ethernet  (MAC addresses)
 Layer 1  Physical      cable, radio
```

## 2.2 What a packet looks like in memory

```
 low address ─────────────────────────────────────────────► high address
 ┌──────────────┬──────────────┬──────────────┬─────────────┐
 │ Ethernet hdr │   IPv4 hdr   │ TCP/UDP hdr  │   payload   │
 │   14 bytes   │ 20+ bytes    │ 8 / 20+ bytes│             │
 └──────────────┴──────────────┴──────────────┴─────────────┘
 ▲                                                           ▲
 data                                                    data_end
```

XDP gives you exactly these two pointers, `data` and `data_end`. You walk the packet by moving a pointer forward and checking you haven't passed `data_end`.

## 2.3 Key networking terms

| Term | Meaning |
|---|---|
| **NIC** | Network Interface Card (hardware). |
| **RX / TX** | Receive / transmit. |
| **RX ring (queue)** | Circular buffer where the NIC places incoming packets via DMA. |
| **DMA** | NIC writes packet bytes straight to RAM without the CPU copying. |
| **Multi-queue / RSS** | NIC spreads flows across several RX queues, one per CPU. |
| **Network byte order** | Big-endian on the wire; x86 is little-endian, so use `htons/ntohs`. |
| **MTU** | Max packet size (usually 1500 bytes). |
| **Checksum** | Integrity value in IP/TCP/UDP headers; must be recomputed if you modify packets. |
| **NAT, firewall, load balancer** | Typical things people build with XDP/eBPF. |

## 2.4 How Linux normally receives a packet

```
 1. Packet arrives at NIC
 2. NIC DMA-writes it to an RX ring buffer in RAM
 3. NIC raises interrupt → driver schedules NAPI poll (softirq)
 4. Driver allocates an  sk_buff  (skb)   ← expensive: metadata + memory
 5. skb goes up: tc ingress → netfilter/iptables → routing → TCP/UDP → socket
 6. App calls recv() and finally gets the data
```

**`sk_buff` (skb)** is the kernel's big per-packet struct (headers, pointers, metadata, refcounts). Allocating it per packet is what costs CPU at millions of packets per second. **XDP runs before step 4**, which is why it's so fast.

```
 Normal path:  NIC → driver → [alloc skb] → tc → netfilter → routing → socket
 XDP path:     NIC → driver → [XDP prog] → drop/forward immediately, no skb
```

---

# PART 3: eBPF core concepts

## 3.1 Name and history

| Term | Meaning |
|---|---|
| **BPF** (classic) | Berkeley Packet Filter (1992): tiny VM for `tcpdump` filters. |
| **eBPF** | "Extended BPF" (Linux 3.18+, ~2014): a general-purpose in-kernel VM. Today people just say "BPF". |
| **cBPF** | Classic BPF, the old one. |

## 3.2 What eBPF actually is

```
 ┌─────────────────────────────────────────────────────────┐
 │ 1. A small instruction set (virtual CPU)                │
 │    11 registers (R0–R10), 64-bit, 512-byte stack        │
 │ 2. A verifier    (proves the program is safe)           │
 │ 3. A JIT compiler (bytecode → native machine code)      │
 │ 4. Hooks         (places in the kernel to attach)       │
 │ 5. Maps          (shared data structures)               │
 │ 6. Helpers       (approved kernel functions to call)    │
 └─────────────────────────────────────────────────────────┘
```

## 3.3 Register conventions

| Reg | Role |
|---|---|
| R0 | Return value |
| R1–R5 | Function arguments (R1 = context pointer on entry) |
| R6–R9 | Callee-saved |
| R10 | Read-only frame (stack) pointer |

You rarely write bytecode by hand, because you write C and clang compiles it. But this explains why the verifier cares so much about types and pointers.

## 3.4 Program lifecycle

```
 write C  ──►  clang -target bpf  ──►  ELF object (.o) with BPF bytecode + BTF
                                              │
                          bpf() syscall  ◄────┘   (loader: libbpf, bpftool, ip)
                                │
                          ┌─────▼──────┐
                          │  VERIFIER  │ ── reject ──► error + log
                          └─────┬──────┘
                                │ accept
                          ┌─────▼──────┐
                          │    JIT     │ → native code
                          └─────┬──────┘
                                │
                          attach to HOOK ──► runs on every event
```

## 3.5 Event-driven model

eBPF programs don't run on their own. They run **when an event fires** at a hook:

```
 packet arrives ─────► XDP / tc program runs
 syscall entered ────► tracepoint / kprobe program runs
 function called ────► kprobe / fentry program runs
 cgroup socket op ───► cgroup program runs
 timer / perf event ─► perf_event program runs
```

Each run is short, bounded, and returns a value to the kernel.

## 3.6 The `bpf()` syscall

Everything goes through one syscall: `bpf(cmd, attr, size)`.

| Command | Purpose |
|---|---|
| `BPF_PROG_LOAD` | Load + verify a program |
| `BPF_MAP_CREATE` | Create a map |
| `BPF_MAP_LOOKUP_ELEM` / `UPDATE` / `DELETE` | Map operations from userspace |
| `BPF_PROG_ATTACH` / `BPF_LINK_CREATE` | Attach to a hook |
| `BPF_OBJ_PIN` / `GET` | Pin objects to `/sys/fs/bpf` so they outlive the process |

Libraries (libbpf, cilium/ebpf, Aya) wrap this syscall.

---

# PART 4: eBPF building blocks

## 4.1 Program types (where programs can attach)

| Category | Types | Used for |
|---|---|---|
| **Networking** | `XDP`, `SCHED_CLS` (tc), `SK_SKB`, `SK_MSG`, `SOCKET_FILTER`, `SK_LOOKUP`, `SOCK_OPS` | Packet filtering, LB, steering |
| **Tracing** | `KPROBE`, `KRETPROBE`, `UPROBE`, `TRACEPOINT`, `RAW_TRACEPOINT`, `FENTRY/FEXIT`, `PERF_EVENT` | Observability, profiling |
| **Security** | `LSM`, `SECCOMP` (via cBPF), `CGROUP_DEVICE` | Policy enforcement |
| **Cgroup** | `CGROUP_SKB`, `CGROUP_SOCK`, `CGROUP_SOCK_ADDR`, `CGROUP_SYSCTL` | Per-container control |
| **Scheduler / other** | `STRUCT_OPS`, `SCHED_EXT` (sched_ext), `LWT_*` | Custom kernel behavior |

Each type has its own **context** struct (what R1 points to) and its own list of allowed helpers and return values.

| Type | Context | Returns |
|---|---|---|
| XDP | `struct xdp_md` | `XDP_PASS/DROP/TX/REDIRECT/ABORTED` |
| tc | `struct __sk_buff` | `TC_ACT_OK/SHOT/REDIRECT` |
| kprobe | `struct pt_regs` | 0 |
| tracepoint | event-specific struct | 0 |

## 4.2 Maps

**Analogy:** Programs are goldfish (no memory between runs). Maps are a whiteboard they all share with userspace.

```
 ┌─────────────┐      ┌───────────┐      ┌─────────────┐
 │ BPF prog A  │◄────►│           │◄────►│ userspace   │
 ├─────────────┤      │  BPF MAP  │      │ (read/write)│
 │ BPF prog B  │◄────►│           │      └─────────────┘
 └─────────────┘      └───────────┘
```

| Map type | Use |
|---|---|
| `ARRAY` | Fixed-size, index = key. Fast. |
| `HASH` | Key→value lookup (flow tables, blocklists). |
| `PERCPU_ARRAY` / `PERCPU_HASH` | Each CPU gets its own copy → lock-free counters. |
| `LRU_HASH` | Hash that evicts least-recently-used. |
| `LPM_TRIE` | Longest-prefix match (IP routing/CIDR blocklists). |
| `RINGBUF` | Efficient kernel→user event stream. |
| `PERF_EVENT_ARRAY` | Older event stream. |
| `PROG_ARRAY` | Table of programs for **tail calls**. |
| `DEVMAP` / `CPUMAP` / `XSKMAP` | Targets for `XDP_REDIRECT` (NICs / CPUs / AF_XDP sockets). |
| `ARRAY_OF_MAPS` / `HASH_OF_MAPS` | Maps containing maps. |
| `STACK_TRACE`, `SOCKMAP`, `SOCKHASH`, `CGROUP_STORAGE`, `TASK_STORAGE` | Specialized. |

Map definition (modern BTF style):

```c
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u32);       // e.g. source IPv4
    __type(value, __u64);     // e.g. packet count
} flows SEC(".maps");
```

## 4.3 Helper functions

Programs cannot call arbitrary kernel functions. They call **helpers**, a fixed, versioned API.

| Helper | What it does |
|---|---|
| `bpf_map_lookup_elem / update_elem / delete_elem` | Map access |
| `bpf_ktime_get_ns` | Timestamp |
| `bpf_get_smp_processor_id` | Current CPU |
| `bpf_printk` | Debug print to trace pipe |
| `bpf_xdp_adjust_head / tail` | Grow/shrink packet (encap/decap) |
| `bpf_redirect / redirect_map` | Forward packet |
| `bpf_csum_diff` | Checksum math |
| `bpf_get_current_pid_tgid`, `bpf_probe_read_*` | Tracing helpers |
| `bpf_tail_call` | Jump to another program |
| `bpf_loop` | Bounded loop (5.17+) |
| `bpf_spin_lock` | Lock inside map values |

Newer kernels also offer **kfuncs** (direct kernel functions exposed to BPF with BTF) in addition to helpers.

## 4.4 The verifier (most important concept)

**Analogy:** An airport security scanner. Nothing boards the plane (the kernel) until it's proven harmless.

What it checks:

```
 ✔ Program terminates (no unbounded loops; max instruction count)
 ✔ No out-of-bounds memory access (every packet read vs data_end)
 ✔ No use of uninitialized registers/stack
 ✔ Pointer types are tracked (packet ptr, map value ptr, ctx ptr...)
 ✔ Map lookups NULL-checked before dereference
 ✔ Only allowed helpers for this program type
 ✔ No pointer leaks to userspace
 ✔ Stack ≤ 512 bytes
 ✔ License compatibility for GPL-only helpers
```

It simulates **every possible execution path**, tracking the range of values in each register. That's why this works:

```c
if ((void *)(eth + 1) > data_end) return XDP_PASS;   // verifier learns: eth is safe to read
```

After that line, the verifier knows `eth` is within bounds on the surviving path.

## 4.5 JIT compiler

Converts BPF bytecode to native machine code at load time → near-native speed. Enabled via `net.core.bpf_jit_enable=1` (default on modern distros).

## 4.6 BTF and CO-RE

| Term | Meaning |
|---|---|
| **BTF** (BPF Type Format) | Compact debug info describing kernel & program types. Lives at `/sys/kernel/btf/vmlinux`. |
| **CO-RE** (Compile Once, Run Everywhere) | Program records which struct fields it uses; libbpf patches offsets at load time to match the running kernel. Solves "kernel struct changed between versions." |
| **`vmlinux.h`** | Header generated from BTF with all kernel types: `bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h` |

## 4.7 Other concepts

| Concept | Meaning |
|---|---|
| **BPF links** | Handle object for an attachment; auto-detaches when closed. |
| **Pinning (bpffs)** | Persist programs/maps at `/sys/fs/bpf/...`. |
| **Tail calls** | Jump to another BPF program (no return); chains programs past size limits. |
| **BPF-to-BPF calls** | Real function calls between `static` functions. |
| **Global / static data** | `.data`, `.rodata`, `.bss` sections, acting like config variables. |
| **Spin locks, atomics** | Concurrency within map values. |
| **Capabilities** | Loading needs `CAP_BPF` (+ `CAP_NET_ADMIN`/`CAP_PERFMON` depending on type) or root. |

---

# PART 5: XDP (eXpress Data Path)

## 5.1 Definition

XDP = an eBPF program type that runs **inside the NIC driver's receive path, before an `sk_buff` exists**. It gets raw packet memory and returns a verdict.

## 5.2 Why it's fast

```
 ✔ Runs before skb allocation
 ✔ Runs on the same CPU that received the packet (no context switch)
 ✔ No locks needed with per-CPU maps
 ✔ JIT-compiled
 ✔ Batching via NAPI
 → Millions of packets/sec per core
```

## 5.3 The context: `struct xdp_md`

```c
struct xdp_md {
    __u32 data;            // start of packet
    __u32 data_end;        // end of packet
    __u32 data_meta;       // metadata area before packet (for passing info)
    __u32 ingress_ifindex; // receiving interface
    __u32 rx_queue_index;  // which RX queue
    __u32 egress_ifindex;  // for devmap programs
};
```

```
 data_meta         data                          data_end
    │                │                              │
    ▼                ▼                              ▼
 ┌────────┬──────────────────────────────────────────┐
 │  meta  │  Eth │ IP │ TCP │ payload ...            │
 └────────┴──────────────────────────────────────────┘
 ◄── headroom (bpf_xdp_adjust_head can grow into it)
```

## 5.4 Return codes (verdicts)

| Code | Effect |
|---|---|
| `XDP_PASS` | Continue to the normal kernel stack |
| `XDP_DROP` | Discard immediately (DDoS mitigation) |
| `XDP_TX` | Send back out the **same** interface (hairpin; load balancers) |
| `XDP_REDIRECT` | Send to another NIC, CPU, or AF_XDP socket |
| `XDP_ABORTED` | Drop + fire `xdp_exception` tracepoint (error path) |

## 5.5 Attach modes

| Mode | Where it runs | Notes |
|---|---|---|
| **Native (driver)** | Inside driver RX | Fast; needs driver support |
| **Generic (skb)** | After skb allocation | Works on any NIC; slow; for development |
| **Offload** | On the NIC hardware | SmartNICs (Netronome); limited features |

## 5.6 What you can do inside XDP

| Action | How |
|---|---|
| Parse headers | Pointer arithmetic + bounds checks |
| Filter/firewall | Match fields → `XDP_DROP` |
| Count / sample | Per-CPU maps |
| Rewrite headers (NAT) | Modify fields, recompute checksums |
| Encapsulate / decapsulate (IP-in-IP, VXLAN, GRE) | `bpf_xdp_adjust_head` |
| Load balance | Hash flow → pick backend → `XDP_TX`/`REDIRECT` |
| Forward | `bpf_redirect_map` with DEVMAP |
| Steer to userspace | AF_XDP via XSKMAP |
| Pass metadata | `data_meta` area, readable by tc programs |

## 5.7 What XDP can't do (limitations)

- Only **ingress** (RX); no egress hook (use tc for egress)
- No `sk_buff`, so no skb-based helpers or socket info
- Limited by driver support, MTU and multi-buffer/fragmentation handling (XDP multi-buffer exists in newer kernels; programs must opt in)
- No sleeping, no blocking, no arbitrary allocation
- Packet mutation requires manual checksum fixes

## 5.8 XDP vs related tools

| Tool | Layer | Speed | Strength |
|---|---|---|---|
| **iptables/nftables** | netfilter, after skb | Slower | Mature, rich rules |
| **tc (cls_bpf)** | After skb alloc, ingress + egress | Medium | Full skb access, egress |
| **XDP** | Driver, before skb | Fastest | Raw speed |
| **DPDK** | Userspace, bypasses kernel | Very fast | Full userspace control, but gives up the kernel stack |
| **AF_XDP** | XDP → userspace socket | Near-DPDK | Keeps kernel driver, zero-copy |

---

# PART 6: Toolchain

| Tool | Purpose |
|---|---|
| **clang/LLVM** | Compiles C → BPF bytecode (`-target bpf`) |
| **libbpf** | C library for loading/attaching; the standard |
| **bpftool** | Swiss-army CLI: list progs/maps, load, dump, generate skeletons |
| **iproute2 (`ip`, `tc`)** | Attach XDP/tc programs |
| **xdp-tools** | `xdp-loader`, `xdpdump`, `xdp-filter`, `xdp-bench` |
| **bpftrace** | One-liner tracing language |
| **BCC** | Python/C toolkit for tracing |
| **cilium/ebpf (Go), Aya (Rust), libbpf-rs** | Language-specific userspace libraries |
| **veth, netns, `xdp-trafficgen`, `pktgen`, `scapy`, `tcpreplay`** | Test traffic and isolated environments |
| **perf, `bpftool prog profile`** | Performance measurement |

Useful commands:

```bash
bpftool prog show            # loaded programs
bpftool map show             # maps
bpftool map dump id <ID>
bpftool net show             # XDP/tc attachments
bpftool feature probe        # what your kernel supports
bpftool btf list
ip -d link show dev eth0     # shows attached XDP prog
```

---

# PART 7: Writing eBPF programs

## 7.1 Program anatomy (libbpf style)

```c
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct { /* map definitions */ } mymap SEC(".maps");

SEC("xdp")                        // section name decides program type
int my_prog(struct xdp_md *ctx)   // entry function takes the context
{
    /* logic */
    return XDP_PASS;              // verdict
}

char LICENSE[] SEC("license") = "GPL";   // required for GPL helpers
```

## 7.2 Common `SEC()` names

| Section | Program type |
|---|---|
| `xdp` | XDP |
| `tc` / `classifier` | tc |
| `kprobe/<func>` | kprobe |
| `tracepoint/<cat>/<name>` | tracepoint |
| `fentry/<func>` | fentry |
| `lsm/<hook>` | LSM |
| `cgroup/...` | cgroup |

## 7.3 The packet-parsing pattern

```
 cursor = data
 ┌───────┐   check cursor+sizeof(hdr) ≤ data_end?
 │ eth   │──► yes → read it, advance cursor
 └───────┘    no  → return XDP_PASS
     │
 ┌───────┐   same pattern
 │  ip   │──► (IP header length is variable: use ip->ihl * 4)
 └───────┘
     │
 ┌───────┐
 │tcp/udp│
 └───────┘
```

## 7.4 Coding restrictions checklist

| Rule | Why |
|---|---|
| Bounds-check before every packet read | Verifier requirement |
| NULL-check every map lookup | Verifier requirement |
| Loops must be bounded (`#pragma unroll`, constant bounds, `bpf_loop`) | Termination proof |
| Use `static __always_inline` for helpers | Avoid call overhead/limits |
| No libc (`printf`, `malloc`, `memcpy` → use `__builtin_memcpy`) | No standard library in kernel |
| No global mutable state except via maps/`.data` | Safety |
| Stack ≤ 512 B | Fixed |
| Instruction limit: 1M verified instructions (4096 for unprivileged) | Verifier complexity bound |
| Use `-O2` | Required; unoptimized code often fails verification |
| Use `bpf_htons/ntohs` for byte order | Wire = big-endian |

## 7.5 Userspace side

```
 userspace loader (C/Go/Rust)
   1. open object file
   2. (optionally) set config / resize maps
   3. load  → verifier runs
   4. attach to interface
   5. loop: read maps / ring buffer
   6. on exit: detach (or pin to keep running)
```

With libbpf skeletons:

```c
struct prog_bpf *skel = prog_bpf__open_and_load();
bpf_xdp_attach(ifindex, bpf_program__fd(skel->progs.my_prog), 0, NULL);
/* read skel->maps.mymap via bpf_map_lookup_elem(...) */
prog_bpf__destroy(skel);
```

---

# PART 8: Advanced topics

| Topic | Summary |
|---|---|
| **Tail calls** | Chain up to 33 programs via `PROG_ARRAY`. |
| **XDP multi-buffer (frags)** | Handles jumbo/fragmented packets; use `bpf_xdp_load_bytes`. |
| **AF_XDP** | Zero-copy path to userspace via UMEM + 4 rings (fill, completion, RX, TX). |
| **XDP metadata / hints** | Pass info to tc/skb; NIC hardware hints (timestamp, hash) via kfuncs. |
| **XDP + tc cooperation** | XDP early filtering, tc for egress/skb features. |
| **Chaining multiple XDP progs** | `libxdp` dispatcher (multiple programs on one interface). |
| **Checksum handling** | Incremental updates with `bpf_csum_diff`. |
| **Connection tracking** | Own flow table in LRU hash maps. |
| **Load balancer designs** | Maglev hashing, DSR, encapsulation (Katran, Cilium). |
| **Ring buffer vs perf buffer** | Prefer `RINGBUF` (shared, ordered, efficient). |
| **bpf_loop / iterators / timers** | Newer bounded-loop and deferred-work features. |
| **BPF arena, kptrs, dynptrs** | Newer memory features for complex programs. |
| **sched_ext** | Write CPU schedulers in BPF. |
| **Real-world projects** | Cilium, Katran (Meta), Cloudflare DDoS mitigation, Falco, Tetragon, Pixie. |

---

# PART 9: Debugging, testing, security

## 9.1 Debugging

| Problem | Tool |
|---|---|
| Verifier rejection | Read the log (`bpftool -d prog load`); look for the failing instruction and register state |
| Runtime values | `bpf_printk` → `cat /sys/kernel/debug/tracing/trace_pipe` |
| Packet inspection | `xdpdump`, `tcpdump` (note: tcpdump doesn't see XDP-dropped packets) |
| Counters | Maps + `bpftool map dump` |
| Performance | `bpftool prog profile`, `perf`, `xdp-bench` |
| Unit testing | `BPF_PROG_TEST_RUN` (feed fake packets to a program, no NIC needed) |
| Inspect compiled code | `llvm-objdump -d prog.bpf.o`, `bpftool prog dump xlated/jited id N` |

## 9.2 Security model

- Loading requires privileges (`CAP_BPF` etc.); unprivileged BPF is usually disabled (`kernel.unprivileged_bpf_disabled`).
- The verifier is the security boundary, and bugs in it have been exploited (CVEs).
- Spectre mitigations restrict some pointer patterns.
- Hardening: `bpf_jit_harden`, signed programs (newer kernels).

## 9.3 Common beginner mistakes

```
 ✗ Forgetting bounds checks               → verifier rejects
 ✗ Forgetting NULL check after lookup     → verifier rejects
 ✗ Compiling without -O2                  → verifier rejects
 ✗ Wrong byte order                       → filters silently never match
 ✗ Testing on your only SSH interface     → lock yourself out
 ✗ Assuming tcpdump sees dropped packets  → it doesn't
 ✗ Expecting XDP to see egress            → ingress only
 ✗ Modifying packet without fixing csum   → packets dropped downstream
```

---

# PART 10: Learning roadmap

```
 Stage 1  Linux basics        syscalls, processes, /proc, namespaces, cgroups
 Stage 2  C for the kernel    pointers, structs, bit ops, endianness, memory layout
 Stage 3  Networking          Ethernet/IP/TCP/UDP headers, tcpdump, ip, ss
 Stage 4  eBPF tracing first  bpftrace one-liners, then libbpf tracepoint/kprobe
 Stage 5  First XDP           drop/pass, counters with PERCPU_ARRAY
 Stage 6  Parse + maps        TCP/UDP ports, hash-map blocklist, LPM trie
 Stage 7  Modify packets      NAT, encap/decap, checksums
 Stage 8  Forwarding          XDP_TX / REDIRECT / DEVMAP, load balancer
 Stage 9  AF_XDP              zero-copy userspace processing
 Stage 10 Production          CO-RE, pinning, testing, perf tuning
```

**Recommended resources**

| Resource | Why |
|---|---|
| `xdp-project/xdp-tutorial` (GitHub) | Hands-on XDP course (best starting point) |
| `ebpf.io` | Concepts and landscape |
| Kernel docs: `Documentation/bpf/` and `networking/af_xdp.rst` | Authoritative reference |
| `man bpf`, `man bpf-helpers` | Syscall and helper reference |
| libbpf-bootstrap (GitHub) | Minimal working project templates |
| *Learning eBPF* (Liz Rice) | Beginner-friendly book |
| *BPF Performance Tools* (Brendan Gregg) | Tracing-focused reference |
| Brendan Gregg's eBPF pages, Cilium BPF reference guide | Deep dives |

---

## Where to go next

Since you're early in the journey, I'd suggest taking it one piece at a time. Which would you like first?

1. **Part 1 in depth**: syscalls, processes, and memory, with code examples
2. **Part 2 in depth**: walking a packet through the kernel step by step
3. **Part 3 in depth**: eBPF bytecode and how the verifier thinks

# Workflow: machine setup and network monitoring

The plan: build a safe lab, learn to see normal traffic first, then add XDP and watch what it changes.

```
 Phase 0  Choose environment      VM, not your only machine
 Phase 1  Verify the kernel       features, BTF, mounts
 Phase 2  Install tools           compiler, loader, monitors
 Phase 3  Language toolchains     C, Go, Rust
 Phase 4  Build a test network    netns + veth (isolated, safe)
 Phase 5  Baseline monitoring     see normal traffic before touching it
 Phase 6  First XDP loop          build, attach, observe, detach
 Phase 7  Monitor XDP itself      counters, drops, run time
 Phase 8  Troubleshooting
```

---

## Phase 0: Choose the environment

| Option | Verdict |
|---|---|
| **VM** (QEMU/KVM, Multipass, VirtualBox, cloud VM) | Best. Crash it, snapshot it, rebuild it. |
| Bare-metal Linux | OK, but mistakes can cut your SSH session or the NIC. |
| WSL2 | Awkward, since it needs a custom kernel for many BPF features. |
| Docker | Shares the host kernel and needs privileged mode. Not a good first lab. |

**Recommendation:** Ubuntu 24.04 LTS (or Debian 12 or later, or Fedora) in a VM, with 2+ vCPUs, 4 GB RAM, and 20 GB disk. A recent kernel (6.x) gives you the most features. Use `virtio-net` as the VM NIC, which supports native XDP in many setups.

**Analogy:** The VM is a flight simulator. Crashing is free.

---

## Phase 1: Verify the kernel

```bash
uname -r                                   # kernel version (want 5.15+, ideally 6.x)

# Required config options
zgrep -E 'CONFIG_(BPF|BPF_SYSCALL|BPF_JIT|BPF_EVENTS|XDP_SOCKETS|DEBUG_INFO_BTF|NET_CLS_BPF|NET_SCH_INGRESS|CGROUP_BPF)=' \
     /boot/config-$(uname -r)

ls /sys/kernel/btf/vmlinux                 # BTF present = CO-RE works
mount | grep -E 'bpf|tracefs|debugfs'      # bpffs and tracefs mounted?
sysctl net.core.bpf_jit_enable             # want 1
```

Mount missing filesystems:

```bash
sudo mount -t bpf bpf /sys/fs/bpf
sudo mount -t tracefs nodev /sys/kernel/tracing
```

What each requirement means:

| Item | Why |
|---|---|
| `CONFIG_BPF_SYSCALL` | The `bpf()` syscall exists |
| `CONFIG_BPF_JIT` | Native-speed execution |
| `CONFIG_DEBUG_INFO_BTF` | CO-RE and `vmlinux.h` |
| `CONFIG_XDP_SOCKETS` | AF_XDP |
| `/sys/fs/bpf` | Pin maps and programs |
| tracefs | `bpf_printk` output, tracepoints |

---

## Phase 2: Install the tools

### Ubuntu/Debian

```bash
sudo apt update
sudo apt install -y \
  build-essential clang llvm lld libbpf-dev libelf-dev zlib1g-dev \
  linux-headers-$(uname -r) linux-tools-$(uname -r) linux-tools-common \
  bpftool dwarves xdp-tools bpftrace \
  iproute2 ethtool tcpdump iperf3 hping3 conntrack \
  dropwatch trace-cmd git curl pkg-config \
  python3-scapy
```

Some packages vary by release. If `bpftool` isn't a separate package, it ships in `linux-tools-*`. Check each with `which`.

### Tool map: which tool does what

| Purpose | Tools |
|---|---|
| Compile BPF C | `clang`, `llvm` |
| Load/attach/inspect BPF | `bpftool`, `ip`, `xdp-loader` |
| Kernel type info | `pahole` (from `dwarves`), `bpftool btf` |
| Packet capture | `tcpdump`, `xdpdump` |
| Traffic generation | `iperf3`, `hping3`, `ping`, `scapy`, `xdp-trafficgen` |
| Interface and NIC stats | `ip -s`, `ethtool`, `/proc/net/dev` |
| Socket and protocol stats | `ss`, `nstat` |
| Tracing | `bpftrace`, `perf`, `trace-cmd` |
| Packet drop hunting | `dropwatch`, `bpftrace` on `kfree_skb` |
| Benchmarking XDP | `xdp-bench` |

### Verify the install

```bash
clang --version
bpftool version
bpftrace --version
xdp-loader --help | head -3
sudo bpftool feature probe kernel | head -30   # what your kernel supports
```

---

## Phase 3: Language toolchains

You work with C, Go, and Rust. The **kernel-side program** is always restricted C or Rust (Aya). The **userspace loader** can be any of them.

```
 ┌──────────────────────────┐        ┌───────────────────────────┐
 │ KERNEL SIDE (BPF prog)   │        │ USERSPACE SIDE (loader)   │
 │ C  (clang)  or  Rust(Aya)│◄──maps─►│ C/libbpf, Go/cilium, Rust │
 └──────────────────────────┘        └───────────────────────────┘
```

| Stack | Kernel side | Userspace | Setup |
|---|---|---|---|
| **C** | C + clang | libbpf + skeleton | Already installed in Phase 2 |
| **Go** | C + clang | `cilium/ebpf` + `bpf2go` | see below |
| **Rust** | Rust (Aya) | Aya | see below |

**Go**

```bash
go install github.com/cilium/ebpf/cmd/bpf2go@latest
# in your module:
go get github.com/cilium/ebpf
# bpf2go compiles your .c and generates Go bindings via //go:generate
```

**Rust (Aya)**

```bash
rustup toolchain install nightly --component rust-src
cargo install bpf-linker
cargo install cargo-generate
cargo generate https://github.com/aya-rs/aya-template   # scaffold a project
```

**C (libbpf-bootstrap)**

```bash
git clone --recurse-submodules https://github.com/libbpf/libbpf-bootstrap
```

Start with C. The tutorials and kernel docs all use it, and the Go and Rust stacks build on the same concepts.

---

## Phase 4: Build a safe test network

Never test on your real NIC first. Use a **network namespace** (a separate network stack) joined to your host by a **veth pair** (a virtual cable).

```
  ┌───────────── host (root netns) ─────────────┐   ┌──── netns "ns1" ────┐
  │                                              │   │                     │
  │   veth0 (10.0.0.1/24) ◄════ virtual cable ═══════► veth1 (10.0.0.2/24) │
  │      ▲                                       │   │                     │
  │      └── attach XDP programs here            │   │  traffic source     │
  └──────────────────────────────────────────────┘   └─────────────────────┘
```

Setup script:

```bash
sudo ip netns add ns1
sudo ip link add veth0 type veth peer name veth1
sudo ip link set veth1 netns ns1

sudo ip addr add 10.0.0.1/24 dev veth0
sudo ip link set veth0 up

sudo ip netns exec ns1 ip addr add 10.0.0.2/24 dev veth1
sudo ip netns exec ns1 ip link set veth1 up
sudo ip netns exec ns1 ip link set lo up

# Disable offloads on veth so checksums/GRO don't confuse XDP experiments
sudo ethtool -K veth0 tx off rx off gro off 2>/dev/null
sudo ip netns exec ns1 ethtool -K veth1 tx off rx off gro off 2>/dev/null

# Test
ping -c 3 10.0.0.2
```

Teardown (one command cleans everything):

```bash
sudo ip netns del ns1 && sudo ip link del veth0 2>/dev/null
```

Run commands inside the namespace with `sudo ip netns exec ns1 <cmd>`.

Generating traffic:

```bash
# Terminal A (inside ns1): server
sudo ip netns exec ns1 iperf3 -s

# Terminal B (host): TCP load
iperf3 -c 10.0.0.2 -t 10

# UDP flood example
iperf3 -c 10.0.0.2 -u -b 100M -t 10

# Custom packets
sudo hping3 -2 -p 5000 --flood 10.0.0.2    # UDP flood (lab only!)
```

---

## Phase 5: Baseline monitoring (before any XDP)

Learn what normal looks like. Monitoring works in layers, from hardware to application:

```
 Layer                What to watch                      Tools
 ─────────────────────────────────────────────────────────────────────────
 NIC hardware         RX/TX counters, ring size, drops   ethtool -S/-g/-l
 Driver / interrupts  IRQ distribution, queues           /proc/interrupts
 Softirq / NAPI       backlog drops, budget exhaustion   /proc/softnet_stat
 Interface            packets, bytes, errors, drops      ip -s link
 IP / protocol        retransmits, resets, IP errors     nstat, netstat -s
 Sockets              state, queues, RTT                 ss
 Packet content       actual packets                     tcpdump
 Kernel drop points   where/why skbs are freed           bpftrace, dropwatch
 Netfilter state      connection tracking                conntrack
```

### 5.1 Interface level

```bash
ip -s link show dev veth0          # RX/TX packets, bytes, errors, dropped
ip -s -s link show dev veth0       # double -s = detailed error breakdown
ip -br addr                        # compact address overview
ip route                           # routing table
watch -n1 'ip -s link show dev veth0'   # live view
cat /proc/net/dev                  # raw counters
```

### 5.2 NIC and driver level

```bash
ethtool -i eth0                    # driver name (needed to know XDP native support)
ethtool -S eth0                    # NIC-specific stats (names vary per driver)
ethtool -l eth0                    # number of queues (channels)
ethtool -g eth0                    # RX/TX ring sizes
ethtool -k eth0                    # offload features (GRO, LRO, checksum...)
cat /proc/interrupts | grep -i eth0
```

### 5.3 Softirq / per-CPU path

```bash
cat /proc/softirqs                 # NET_RX / NET_TX per CPU
cat /proc/net/softnet_stat         # one row per CPU, hex columns
```

Key columns in `softnet_stat`:

| Column | Meaning |
|---|---|
| 1 | Packets processed |
| 2 | **Dropped** (backlog full) |
| 3 | **time_squeeze** (NAPI ran out of budget or time) |

Rising column 2 or 3 means the CPU can't keep up.

### 5.4 Protocol and socket level

```bash
nstat -az | grep -Ei 'drop|retrans|err'     # protocol counters (absolute)
nstat                                        # delta since last call
ss -s                                        # summary
ss -tunap                                    # all TCP/UDP sockets + processes
ss -ti                                       # TCP internals: rtt, cwnd, retrans
ss -ltn                                      # listening TCP sockets
```

### 5.5 Packet capture

```bash
sudo tcpdump -ni veth0                                   # all packets
sudo tcpdump -ni veth0 -c 20 'udp and port 5000'         # filtered, 20 packets
sudo tcpdump -ni veth0 -w cap.pcap                       # save for Wireshark
sudo tcpdump -ni veth0 -e                                # show Ethernet headers
sudo tcpdump -ni veth0 -XX -c 1                          # hex dump (matches what XDP sees)
```

**Important:** `tcpdump` captures after XDP, so packets dropped by XDP never appear. Use `xdpdump` to see them:

```bash
sudo xdpdump -i veth0 -w xdp.pcap      # captures at the XDP hook
```

### 5.6 Finding kernel packet drops

```bash
# Where in the kernel are packets being freed (dropped)?
sudo bpftrace -e 'tracepoint:skb:kfree_skb { @[ksym(args->location)] = count(); }'

# Interactive drop monitor
sudo dropwatch -l kas

# TCP retransmits
sudo bpftrace -e 'tracepoint:tcp:tcp_retransmit_skb { @[comm] = count(); }'

# Conntrack stats (if netfilter conntrack is active)
sudo conntrack -S
```

Press `Ctrl+C` to print bpftrace maps.

### 5.7 Quick all-in-one view

```bash
sudo apt install -y bmon iftop nload     # optional live dashboards
bmon                                     # per-interface bandwidth
sudo iftop -i eth0                       # per-flow bandwidth
```

---

## Phase 6: The first XDP loop

This is the loop you'll repeat constantly:

```
        ┌────────────────────────────────────────────────┐
        │                                                │
   1. edit .bpf.c                                        │
        │                                                │
   2. compile  ──► error? fix C                          │
        │                                                │
   3. load/attach ──► verifier reject? read log, fix     │
        │                                                │
   4. generate traffic (ping/iperf/hping3)               │
        │                                                │
   5. observe (maps, ip -s, tcpdump/xdpdump, tracepoints)│
        │                                                │
   6. detach  ──────────────────────────────────────────►┘
```

### Step-by-step

```bash
# 1-2. Compile (use the drop_udp.bpf.c from earlier)
clang -O2 -g -target bpf -c drop_udp.bpf.c -o drop_udp.bpf.o

# 3. Attach to the veth (generic mode works everywhere)
sudo ip link set dev veth0 xdpgeneric obj drop_udp.bpf.o sec xdp

# Confirm
ip -d link show dev veth0 | grep -i xdp
sudo bpftool net show

# 4. Generate traffic from the namespace
sudo ip netns exec ns1 ping -c 3 10.0.0.1                 # ICMP passes
sudo ip netns exec ns1 hping3 -2 -p 5000 -c 5 10.0.0.1    # UDP gets dropped

# 5. Observe (see Phase 7)

# 6. Detach
sudo ip link set dev veth0 xdpgeneric off
```

Alternative loader with a cleaner status view:

```bash
sudo xdp-loader load -m skb veth0 drop_udp.bpf.o      # skb = generic mode
sudo xdp-loader status                                # shows what's attached
sudo xdp-loader unload veth0 --all
```

---

## Phase 7: Monitor the XDP program itself

### 7.1 Enable run-time statistics

```bash
sudo sysctl -w kernel.bpf_stats_enabled=1
sudo bpftool prog show           # now shows run_time_ns and run_cnt
```

Average cost per packet is `run_time_ns / run_cnt`. Turn it off after measuring (`=0`), since it adds small overhead.

### 7.2 Inspect programs and maps

```bash
sudo bpftool prog show                       # all loaded programs
sudo bpftool prog show id <ID> --pretty
sudo bpftool prog dump xlated id <ID>        # BPF instructions after verifier
sudo bpftool prog dump jited id <ID>         # native machine code
sudo bpftool map show
sudo bpftool map dump name pkt_count         # contents of your counter map
sudo bpftool net show                        # attachments per interface
```

### 7.3 Live debug prints

```bash
# In C: bpf_printk("proto=%d\n", ip->protocol);
sudo bpftool prog tracelog
# or
sudo cat /sys/kernel/tracing/trace_pipe
```

### 7.4 XDP verdict tracing

```bash
# XDP_ABORTED / errors
sudo bpftrace -e 'tracepoint:xdp:xdp_exception { @[args->ifindex, args->act] = count(); }'

# Redirect events
sudo bpftrace -e 'tracepoint:xdp:xdp_redirect* { @[probe] = count(); }'

# List all XDP tracepoints available
sudo bpftrace -l 'tracepoint:xdp:*'
```

### 7.5 Throughput and performance

```bash
sudo xdp-bench drop veth0                    # built-in drop benchmark with live pps
sudo perf stat -a -e cycles,instructions sleep 5
sudo perf top                                # CPU hotspots (look for your BPF prog / driver)
sudo bpftool prog profile id <ID> duration 5 cycles instructions
mpstat -P ALL 1                              # per-CPU usage (package: sysstat)
```

### 7.4 What to watch while running XDP

| Symptom | Where to look |
|---|---|
| Is my program attached? | `bpftool net`, `ip -d link` |
| Is it seeing packets? | `bpftool prog show` → `run_cnt` rising |
| What verdicts is it returning? | Counter map per verdict (`PERCPU_ARRAY`) |
| Are packets dropped *by me*? | `xdpdump`, your counter map |
| Are packets dropped *elsewhere*? | `kfree_skb` bpftrace, `nstat`, `ip -s` |
| Is a CPU saturated? | `mpstat`, `perf top`, `/proc/net/softnet_stat` |
| Is traffic landing on one queue only? | `ethtool -S`, `/proc/interrupts` (RSS/hash issue) |

---

## Phase 8: Troubleshooting

| Problem | Likely cause | Fix |
|---|---|---|
| `Permission denied` / `Operation not permitted` | Not root, or missing `CAP_BPF` | Use `sudo` |
| Verifier: `invalid access to packet` | Missing bounds check | Check against `data_end` before reading |
| Verifier: `R0 invalid mem access 'map_value_or_null'` | No NULL check after lookup | Add `if (!val) return ...;` |
| Verifier: `back-edge` / `loop not bounded` | Unbounded loop | Constant bounds, `#pragma unroll`, or `bpf_loop` |
| `Error: native XDP not supported` | Driver lacks support | Use `xdpgeneric` (dev) or a driver that supports it |
| Program attached but `run_cnt` is 0 | Traffic isn't hitting this interface | Check interface, direction (XDP is ingress only) |
| Filter never matches | Byte-order bug | Use `bpf_htons()` and `bpf_ntohs()` |
| Packets vanish after modification | Bad checksum or length | Fix checksums, check `ip->tot_len` |
| `libbpf: failed to find BTF` | Kernel lacks BTF | Install a kernel with `CONFIG_DEBUG_INFO_BTF=y` |
| SSH died after attach | XDP dropped your session | Reboot VM; always test on veth first |
| `tcpdump` shows nothing | XDP dropped it first | Use `xdpdump` |

Reading the verifier log:

```bash
sudo bpftool -d prog load prog.bpf.o /sys/fs/bpf/test 2>&1 | tail -50
# Read BOTTOM-UP: the last lines show the rejected instruction and register state
```

---

## Cheat sheet

```
 SETUP    uname -r · zgrep CONFIG_BPF /boot/config-$(uname -r) · ls /sys/kernel/btf/vmlinux
 LAB      ip netns add · ip link add veth0 type veth peer name veth1
 BUILD    clang -O2 -g -target bpf -c x.bpf.c -o x.bpf.o
 ATTACH   ip link set dev veth0 xdpgeneric obj x.bpf.o sec xdp
 DETACH   ip link set dev veth0 xdpgeneric off
 INSPECT  bpftool prog|map|net show · bpftool map dump name X
 WATCH    ip -s link · ethtool -S · nstat · ss -s · /proc/net/softnet_stat
 CAPTURE  tcpdump -ni IF (post-XDP) · xdpdump -i IF (at XDP)
 DROPS    bpftrace tracepoint:skb:kfree_skb · dropwatch -l kas
 PERF     sysctl kernel.bpf_stats_enabled=1 · xdp-bench · perf top
```

---

## Suggested first-day exercise

```
 1. Build the VM, run Phase 1 checks                      (30 min)
 2. Install tools, build the veth lab                     (30 min)
 3. Generate iperf3 traffic, watch with ip -s / nstat / ss (30 min)
 4. Attach drop_udp, confirm with hping3 + xdpdump        (30 min)
 5. Add a PERCPU_ARRAY counter, read via bpftool map dump (60 min)
```

Do you want me to go deeper on one phase next? I'd suggest either a complete C XDP packet counter with a libbpf userspace loader, or a walkthrough of the `ethtool -S` / `softnet_stat` output so you can read NIC-level metrics confidently.

# Short answer

Yes, several companies and projects already work in this space. The most important finding is that the market is mostly **open source plus support contracts**, so the business model matters more than the technology.

First, a terminology correction that affects how you position the company.

## "XDP/eBPF device drivers" is not quite a category

```
 NIC hardware
     │
 ┌───▼────────────────────────────┐
 │ Device driver (C kernel module │  ← written by NIC vendors (Intel, NVIDIA,
 │ e.g. i40e, mlx5, ena, virtio)  │    Broadcom, AWS) and upstreamed in Linux
 │                                │
 │   ┌────────────────────────┐   │
 │   │ XDP hook  ◄────────────┼───┼── your eBPF program runs HERE
 │   └────────────────────────┘   │
 └────────────────────────────────┘
```

| Thing | Who writes it | Language |
|---|---|---|
| **Device driver** | NIC vendor, upstream kernel | Kernel C |
| **XDP/eBPF program** | You (or the customer) | Restricted C or Rust |
| **Userspace control plane** | You | Go, Rust, C |

An eBPF program is not a driver. It is a packet-processing function that runs inside the driver's receive path. Customers will hear "driver" and think of hardware enablement, which is a different business (and the NIC vendors already own it). What you can sell is closer to **XDP-based network functions, data planes, or SDKs**: firewalls, DDoS mitigation, load balancers, NAT, telemetry, and so on.

## Who is already doing it

### 1. Open source projects (free competitors)

| Project | Backer | What it does |
|---|---|---|
| **Cilium** | Isovalent (acquired by Cisco, per my background knowledge) | Kubernetes networking, security, observability |
| **Katran** | Meta | XDP L4 load balancer |
| **Calico eBPF data plane** | Tigera | Kubernetes networking data plane |
| **xdp-tools / xdp-project** | Red Hat and community | Loader, filter, and benchmark tools |
| **Aya, libbpf, cilium/ebpf** | Community | The development libraries you'll use |

Cilium is described as the most widely deployed eBPF-based networking solution for Kubernetes, replacing kube-proxy and avoiding iptables overhead. Cilium is the most widely deployed eBPF-based networking solution for Kubernetes. Katran uses XDP with a DEVMAP for packet steering, as noted in the same guide.

### 2. Big companies with in-house eBPF/XDP

- Cloudflare uses eBPF for DDoS mitigation and load balancing, and Meta uses it for network instrumentation at scale. Cloudflare uses eBPF for DDoS mitigation and load balancing, while Facebook uses it for network instrumentation at massive scale.
- Naver replaced a Netfilter-based NAT system with an eBPF/XDP one after hitting performance problems. Naver moved to eBPF after iptables-based NAT showed performance issues.
- Cloud providers ship eBPF networking in managed products. AWS, GCP (GKE Dataplane V2), and Azure (AKS) all offer eBPF-based networking.

These companies are mostly **not customers**, since they build it themselves. They are also your hiring competition.

### 3. Commercial vendors

| Vendor | Angle |
|---|---|
| **Cisco (Isovalent)** | Enterprise Cilium, Tetragon (security) |
| **Tigera** | Calico commercial and enterprise |
| **Juniper** | Contrail CN2 with an eBPF-XDP data plane, validated on drivers like ENA, veth, virtio, and i40e Juniper's Contrail Networking CN2 supports an eBPF data plane built on XDP, with validated drivers including ENA, veth, virtio, and i40e. |
| **Splunk** | OpenTelemetry eBPF instrumentation, commercial support Splunk introduced OpenTelemetry eBPF Instrumentation at KubeCon EU 2026, now in beta. |
| **Arista** | Data center switching on Linux (EOS), a hardware-centric competitor for the same budget Arista's EOS is a networking stack built on an unmodified Linux kernel. |
| **Solo.io, Isovalent partners, Datadog, Grafana, Pixie/New Relic** | Observability and service mesh with eBPF |
| **NVIDIA (Mellanox/DOCA), Intel, Corigine (ex-Netronome), AMD Pensando** | SmartNIC and DPU offload, a hardware-level alternative |

I did not verify every vendor above in this search, so confirm each before you rely on it. Check each vendor's current site.

### 4. The adoption signal

According to a CNCF survey cited in one 2026 article, 67 percent of Kubernetes teams already use at least one eBPF-based tool in production. That shows demand is real, but it also means the "standard" use cases are already covered by free tools.

## What this means for Wyvernetworks

```
 CROWDED (hard to win)                    LESS CROWDED (opportunity)
 ─────────────────────────────────        ────────────────────────────────────
 Generic K8s CNI / network policy         Custom XDP network functions for a
 Generic L4 load balancer                   specific industry (telco, finance,
 Generic observability                      gaming, CDN edge)
 Basic DDoS filtering                     Performance engineering and tuning
                                          Porting/hardening XDP on specific
                                            NICs, DPUs, or ARM servers
                                          AF_XDP / userspace data planes
                                          Compliance-grade firewalls with
                                            support SLAs
                                          Training, audits, consulting
```

Realistic business models, roughly in order of how fast they can earn revenue:

| Model | Pros | Cons |
|---|---|---|
| **Consulting / custom development** | Fast revenue, learn real problems | Doesn't scale, tied to your hours |
| **Support and SLAs on a tuned product** | Recurring revenue | Needs a credible product first |
| **Open-core product** (open engine, paid enterprise features) | The proven model (Cilium, Calico) | Slow start, needs community |
| **Licensed library / SDK** | Clear product | Hard to sell against free open source |
| **Hardware-specific XDP data plane** (SmartNIC/DPU) | Defensible via expertise | Small market, vendor dependence |

## Before you start: checks I'd do

1. **Talk to 10 prospective customers** (platform and network engineers at cloud and data center companies). Ask what they pay for today and what they can't solve with Cilium, Katran, or their NIC vendor's tools.
2. **Check licensing.** BPF programs that use GPL-only helpers must be GPL. Your product has to be designed around that. Get a lawyer to advise on your business model.
3. **Pick one narrow problem** and be the best at it, instead of a general "XDP platform."
4. **Trademark and name search** for "Wyvernetworks" in your jurisdiction.
5. **Verify the competitors yourself.** The search results above are a starting point, not a market study, and acquisitions and pricing change quickly.

I'm not a business or legal advisor, so treat this as a map rather than a recommendation.

Do you want me to go further on one of these: a competitor comparison table (features, pricing, licensing) for Cilium, Katran, Calico, and the commercial vendors, or a list of candidate niche products Wyvernetworks could start with?

# Yes, it's possible and common, if you follow each license

Most commercial infrastructure companies are built on open source. The real question is **which license each project uses and what it requires of you**. I'm not a lawyer, so treat this as a map and have a lawyer review your final design.

**Analogy:** Open source is a free ingredient supplier. Some suppliers let you cook anything and sell it (permissive). Some say "if you serve a dish made with my sauce, you must also share your recipe" (copyleft).

```
 Your product
   ├── your own code               ← you choose the license
   ├── permissive dependencies     ← use freely, keep notices
   ├── weak copyleft libraries     ← OK to link, share changes to THEM only
   └── GPL code                    ← obligations if you distribute it
```

## 1. License families

| Family | Examples | Can you sell a closed product built on it? | Main obligation |
|---|---|---|---|
| **Permissive** | MIT, BSD-2/3, Apache-2.0 | Yes | Keep copyright and license notices (Apache also covers patents and requires noting changes) |
| **Weak copyleft** | LGPL-2.1 | Yes, if you link properly | Share modifications to the LGPL library itself; allow users to relink |
| **Strong copyleft** | GPL-2.0, GPL-3.0 | Only if you comply (usually means sharing source of the combined work *when you distribute*) | Provide source of derived work to recipients |
| **Network copyleft** | AGPL-3.0 | Risky for SaaS | Source obligation triggers even when users only interact over a network |
| **Source-available** | BSL, SSPL, Elastic License | Often restricted (e.g. no competing hosted service) | Read each one carefully |

## 2. The eBPF-specific twist: kernel side vs userspace side

An eBPF product always has two halves, and **they can have different licenses**.

```
 ┌──────────────────────────┐        ┌──────────────────────────────┐
 │ KERNEL SIDE              │        │ USERSPACE SIDE               │
 │ BPF program (.bpf.c)     │◄─maps─►│ loader, control plane, CLI,  │
 │                          │        │ API, UI                      │
 │ GPL-only helpers require │        │ Your license choice, subject │
 │ the program to declare a │        │ to the libraries you link    │
 │ GPL-compatible license   │        │                              │
 └──────────────────────────┘        └──────────────────────────────┘
```

| Question | Answer |
|---|---|
| Must my BPF program be GPL? | Only if it uses GPL-only helpers (many useful ones are, e.g. `bpf_trace_printk`, many tracing helpers). The `LICENSE` string in the program (`char LICENSE[] SEC("license")`) is checked by the kernel at load time. |
| Can I keep the BPF programs proprietary? | Possible if you limit yourself to helpers available to non-GPL programs, but you lose access to a lot. Check each helper's license requirement. |
| Is the userspace control plane affected by the BPF license? | Generally treated as a separate program that talks to the kernel through the `bpf()` syscall, but the boundary is a legal question to confirm with a lawyer. |
| Is the Linux kernel itself a problem? | The kernel is GPL-2.0 with a syscall exception, so userspace programs calling it are not made GPL. |

Common industry pattern: **BPF code under GPL (or dual BSD/GPL), userspace under Apache-2.0 or proprietary.** Many projects use the dual form precisely so both worlds can use it.

## 3. Licenses of the projects you're likely to build on

Verify these against each repo's `LICENSE` file before you commit, since licenses can change and subprojects can differ.

| Project | Typical license (check current) | Practical implication |
|---|---|---|
| **libbpf** | LGPL-2.1 OR BSD-2-Clause | Very friendly; choose BSD-2 and link freely |
| **cilium/ebpf** (Go library) | MIT | Fully permissive |
| **Aya** (Rust) | MIT / Apache-2.0 (userspace); eBPF side in a dual form | Permissive |
| **Cilium** | Apache-2.0 for userspace; BPF code is dual-licensed (GPL/BSD) | You can build on it, but see trademark below |
| **Calico** | Apache-2.0 | Permissive; commercial editions are separate |
| **Katran** | GPL-2.0 | Strong copyleft: distributing a derived work triggers source obligations |
| **xdp-tools / libxdp** | Mix of GPL-2.0, LGPL-2.1, BSD-2 by component | Check per-component |
| **bpftool** | GPL-2.0 or BSD-2 (dual) | Use as a tool; the dual license helps |
| **bpftrace / BCC** | Apache-2.0 | Permissive |
| **Linux kernel** | GPL-2.0 (+ syscall exception) | Not a concern for userspace products |

## 4. What "using existing projects" can mean

```
 Level 1  USE as a dependency      link a library (libbpf, cilium/ebpf, Aya)
 Level 2  BUNDLE / redistribute    ship Cilium/Katran/Calico inside your product
 Level 3  FORK and modify          maintain your own version
 Level 4  OPERATE as a service     run it for customers (managed offering)
 Level 5  CONTRIBUTE upstream      fix bugs, add features to the main project
```

| Level | Permissive (Apache/MIT/BSD) | GPL-2.0 | AGPL |
|---|---|---|---|
| 1 Use library | OK | Depends on linking; high risk if linked statically into a proprietary binary | High risk |
| 2 Bundle | OK with notices | Must offer source of the GPL parts (separate programs can often coexist) | Must offer source |
| 3 Fork and sell | OK; may keep changes private | Changes must be shared with recipients | Shared even over network |
| 4 Run as a service | OK | Usually no distribution, so typically no source obligation | **Source obligation applies** |
| 5 Upstream | Good for reputation and cost | Same | Same |

## 5. Business models that work on top of open source

| Model | How it works | Real examples (from my background knowledge) |
|---|---|---|
| **Support and SLA** | Customer runs open source; you provide 24/7 support, patches, and guarantees | Red Hat model; Isovalent/Tigera enterprise support |
| **Open core** | Core is open; paid enterprise features are proprietary add-ons | Tigera Calico Enterprise |
| **Hardened, certified distribution** | You package, test, and certify a version for specific hardware/OS/compliance needs | Enterprise Linux distributions |
| **Managed service** | You run it for customers | Cloud providers |
| **Professional services** | Integration, tuning, migration, custom XDP functions | Consulting firms |
| **Hardware-specific optimization** | Tuned data plane for particular NICs or DPUs | Vendor partnerships |
| **Training and audits** | Courses, performance reviews | Many small firms |

Fit for Wyvernetworks: a small team usually starts with **services + support**, then packages repeated work into a **product** (open core or a proprietary add-on layer).

## 5b. Where you add value so customers pay

Customers can already download Cilium or Katran for free, so you must be worth paying for. Typical differentiators:

```
 ┌───────────────────────────────────────────────┐
 │ WHAT THE CUSTOMER BUYS                        │
 ├───────────────────────────────────────────────┤
 │ • Someone accountable when it breaks (SLA)    │
 │ • Tested on THEIR hardware/kernel/cloud       │
 │ • Integration with THEIR systems              │
 │ • Performance tuning for THEIR traffic        │
 │ • Features the open project doesn't have      │
 │ • Compliance evidence (audits, certifications)│
 │ • Easy management UI/API/automation           │
 │ • Faster security patches                     │
 └───────────────────────────────────────────────┘
```

## 6. Pitfalls to avoid

| Pitfall | Why it matters |
|---|---|
| **Trademarks** | Apache/MIT licenses don't grant rights to use names. Cilium and Calico are trademarked; you can't call your product "Cilium Pro" or imply endorsement. Read each project's trademark policy. |
| **Copying GPL code into a proprietary codebase** | Triggers source-sharing obligations on distribution. |
| **Mixed licenses in one binary** | Some combinations are incompatible (e.g. Apache-2.0 with GPL-2.0-only). |
| **Unreviewed transitive dependencies** | A permissive library can pull in a copyleft one. Use a scanner (e.g. `go-licenses`, `cargo-deny`, `cargo-about`, FOSSA, ScanCode). |
| **Contributor license agreements** | If you contribute upstream, check whether it requires a CLA. |
| **Upstream license changes** | Some projects have moved from open to source-available licenses; pin versions and watch announcements. |
| **Patent grants** | Apache-2.0 includes a patent grant, which helps you, but also revokes it if you sue contributors. |
| **Customers' own policies** | Some enterprises ban AGPL or GPL components in vendor products. Check early. |
| **Kernel dependency** | Your product depends on specific kernel versions and features; support matrix is part of your promise. |
| **Export and sanctions rules** | Cryptography or networking software can be regulated depending on countries; get advice. |

## 7. Practical checklist

```
 □ 1. List every component you plan to use (libs, projects, tools)
 □ 2. Record each one's license and exact version
 □ 3. Classify: permissive / weak copyleft / GPL / AGPL / source-available
 □ 4. Decide the architecture to keep license boundaries clean
        (BPF side: GPL or dual; userspace: separate process, own license)
 □ 5. Choose your business model (support / open core / service)
 □ 6. Check trademark policy of every project you name
 □ 7. Set up automated license scanning in CI
 □ 8. Generate a Software Bill of Materials (SBOM): customers will ask
 □ 9. Write customer terms (license, SLA, warranty, liability limits)
 □ 10. Get a lawyer to review before the first paid contract
```

## 8. A sample clean architecture

```
 Customer's servers
 ┌───────────────────────────────────────────────────────────────┐
 │                                                               │
 │  [Wyvernetworks agent]  (your code, your license)             │
 │     │   uses libbpf (BSD-2) / cilium/ebpf (MIT) / Aya        │
 │     │                                                         │
 │     ▼ bpf() syscall                                           │
 │  [BPF programs]  (GPL / dual BSD-GPL, source provided)        │
 │                                                               │
 │  [Management API / dashboard]  (proprietary or open core)     │
 │                                                               │
 └───────────────────────────────────────────────────────────────┘
        ▲
        └── Optional: open-source building blocks (Cilium, Calico,
            xdp-tools) used as separate components, unmodified,
            with notices and source offers where needed
```

## Summary

| Question | Answer |
|---|---|
| Can you build a B2B product on existing open source? | **Yes**, it's the standard path. |
| Can you sell it? | **Yes**, for permissive and properly handled copyleft code. |
| Biggest risks | GPL/AGPL obligations, trademarks, license changes |
| Safest starting stack | libbpf + cilium/ebpf or Aya, with BPF code under a GPL-compatible license |
| Best early business model | Services and support, evolving into open core |
| Must-do before selling | Component license audit, trademark check, lawyer review |

Would you like a worked example next, such as how to license-audit a specific stack (say Go + cilium/ebpf + libbpf), or a step-by-step plan for turning a consulting engagement into a repeatable product?