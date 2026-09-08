## Data Race and the Four Pillars That Prevent It

Let me build this layer by layer. Start from hardware, reason upward to language. This is the only way it sticks.

---

## 1. Data Race — Precise Definition

A data race is not about "multiple writers." It is about **concurrent, unsynchronized, conflicting memory accesses.**

Formally — three conditions, all three must hold simultaneously:

```
Condition 1: Two or more execution contexts access the same memory location
Condition 2: At least one access is a WRITE
Condition 3: There is NO synchronization ordering them (no happens-before)
```

Miss even one condition → no data race.
All three present → **undefined behavior.** The program is wrong at a foundational level.

**Why undefined behavior and not just "wrong value"?**

Because the CPU and the compiler are **both** allowed to transform your code under the assumption that data races do not exist. Once you have a race, those transformations produce effects you cannot predict. You lose all ability to reason about what the code does.

---

## 2. Memory Barriers

### The Problem They Solve

You assume CPUs execute instructions in the order you wrote them. They do not.

Modern CPUs use:

```
Store Buffer:
  CPU does not write directly to cache.
  Write goes into a store buffer first.
  CPU continues executing other instructions.
  Buffer drains to cache/memory later, asynchronously.

Load Buffer (Speculative Loads):
  CPU reads from cache speculatively, before prior stores are complete.
  It may serve a load from its own store buffer or from a stale cache line.

Out-of-Order Execution:
  If instruction N is waiting on a cache miss,
  the CPU executes instruction N+3, N+4 out of order,
  commits results in order only if no exceptions.
```

Result: **from another core's perspective, your writes may appear in a completely different order than your source code implies.**

### What a Memory Barrier Does

A memory barrier is a CPU instruction that enforces ordering constraints on memory operations.

```
Types:

  Store Barrier (wmb, sfence):
    All stores BEFORE this barrier complete
    before any store AFTER it begins.

  Load Barrier (rmb, lfence):
    All loads BEFORE this barrier complete
    before any load AFTER it begins.

  Full Barrier (mb, mfence):
    Both store and load ordering enforced in both directions.
```

### In the Linux Kernel

The kernel defines architecture-independent wrappers:

```c
mb();      /* full barrier */
rmb();     /* read/load barrier */
wmb();     /* write/store barrier */

smp_mb();  /* full barrier, but only compiled in on SMP builds */
smp_rmb(); /* SMP read barrier */
smp_wmb(); /* SMP write barrier */
```

`smp_*` variants compile down to `barrier()` (compiler fence only, no CPU instruction) on UP (uniprocessor) builds, because reordering across cores is irrelevant when there is only one core. This is a production optimization.

### A Real Pattern: Ring Buffer in a Network Driver

```c
/* Producer (NIC driver, filling descriptors) */
desc->addr   = dma_addr;
desc->length = len;
wmb();               /* ensure desc fields visible before we advance tail */
ring->tail = new_tail;

/* Consumer (NAPI poll, reading descriptors) */
head = ring->head;
rmb();               /* ensure we read head before reading desc fields */
addr = desc->addr;
len  = desc->length;
```

Without `wmb()`, the consumer might see the updated tail but stale descriptor fields. The write to `ring->tail` could have been reordered before the writes to `desc->addr` and `desc->length` by the CPU. **This is a real class of bug in network driver code.**

---

## 3. Cache Coherency

### The Hardware Problem

Each CPU core has its own private L1 and L2 cache. They all see the same physical memory, but they cache different views of it.

```
Core 0        Core 1        Core 2
  L1            L1            L1
  L2            L2            L2
        L3 (shared, or per-socket)
              DRAM
```

If Core 0 writes to address X, Core 1's L1 still holds the old value until the coherency protocol updates it. The window between "Core 0 wrote" and "Core 1 sees the write" is where races live.

### MESI Protocol

Hardware cache coherency is maintained by a protocol. MESI is the classic one:

```
M — Modified:
    This core has the only valid copy.
    It differs from DRAM. Others have Invalid.

E — Exclusive:
    This core has the only valid copy.
    Matches DRAM. Others have Invalid.
    Can promote to M without bus transaction.

S — Shared:
    Multiple cores hold valid copies.
    Matches DRAM. Any core can read.
    No core can write without first invalidating others.

I — Invalid:
    This core's copy is stale. Must fetch before use.
```

**State transition when you write:**

```
Core 0 wants to write to address X, currently in S state:
  → Core 0 sends "Request for Ownership" (RFO) on the bus
  → All other cores with X in S state transition to I (Invalid)
  → Core 0 transitions to M (Modified)
  → Now Core 0 can write
  → Core 1 reads X → cache miss → fetches from Core 0 (flush) or DRAM
```

### What Coherency Gives You — and Does NOT Give You

**Gives you:** Eventually, every core will see a consistent value. No core sees a value that was never written.

**Does NOT give you:** Ordering. MESI ensures coherency of individual cache lines, but does not tell Core 1 in what **order** Core 0's writes to different addresses became visible.

That is why you still need **memory barriers** even with full cache coherency. They are solving different problems.

### Device DMA — Coherency Does Not Apply

This is critical for driver work. The NIC (or any DMA-capable device) accesses memory through its own bus master transactions. **The device is NOT part of the CPU coherency domain.** MESI does not cover device-to-memory transactions.

If your driver writes a descriptor in cache and then tells the NIC to read that memory, the NIC may read stale DRAM because the write is still in L1 cache. This is why you have:

```c
dma_sync_single_for_device(dev, dma_addr, size, DMA_TO_DEVICE);
/* After this, the device sees the data. */

dma_sync_single_for_cpu(dev, dma_addr, size, DMA_FROM_DEVICE);
/* After this, the CPU sees what the device wrote. */
```

These functions issue the correct cache flush or invalidate operations depending on architecture.

---

## 4. `volatile`

### What It Actually Means in C

`volatile` is a promise to the compiler: **do not optimize this access away. Re-read from memory every time. Do not cache in a register. Do not reorder with other volatile accesses.**

```c
volatile uint32_t *reg = (volatile uint32_t *)0xFEA00000; /* MMIO address */

uint32_t a = *reg;  /* compiler MUST emit a real load */
uint32_t b = *reg;  /* compiler MUST emit another real load, cannot reuse a */
```

Without `volatile`, the compiler sees two reads from the same address with no writes in between, and may legally optimize the second read away, returning the cached value from the register.

For MMIO (Memory-Mapped I/O, device registers), this is fatal. Reading a device status register twice may have side effects on the device. Both reads must actually hit the bus.

### What `volatile` Does NOT Give You

This is where most engineers have the wrong mental model:

```
volatile does NOT give you atomicity.
volatile does NOT give you memory ordering guarantees.
volatile does NOT make multi-threaded code correct.
```

In C11 and later, concurrent access to a `volatile` non-atomic variable from multiple threads is still undefined behavior if one access is a write.

**In the Linux kernel**, MMIO accessors use `volatile` AND a compiler barrier:

```c
/* arch/x86/include/asm/io.h simplified */
static inline u32 readl(const volatile void __iomem *addr)
{
    u32 val;
    asm volatile("mov %1, %0" : "=r"(val) : "m"(*(volatile u32 *)addr));
    return val;
}
```

The `volatile` here is specifically for preventing the compiler from caching the MMIO register read. The `__iomem` annotation is for sparse (the kernel static analyzer) to catch incorrect pointer usage.

### Java/C# `volatile` — A Completely Different Animal

Do not conflate C `volatile` with Java `volatile`. In Java, `volatile` additionally provides **happens-before guarantees** and **memory visibility** across threads. In C, it does none of that. They share a keyword and nothing else conceptually.

---

## 5. Atomics

### The Core Problem Atomics Solve

Even if cache coherency ensures all cores eventually see the same value, a **read-modify-write** operation is not atomic by default:

```c
counter++;
/* This is three separate operations:
   1. Load counter from memory to register
   2. Increment register
   3. Store register back to memory

   Between steps 1 and 3, another core can also
   load, increment, and store. Both start from the
   same value. Both write value+1. You lose an increment.
*/
```

This is the classic lost-update problem. The fix requires the load-modify-store to be **indivisible** — no other core can observe or modify the value in between.

### Hardware Mechanisms

**x86: LOCK prefix**

```
LOCK XADD [mem], reg    ; atomic fetch-and-add
LOCK CMPXCHG [mem], reg ; atomic compare-and-swap
LOCK INC [mem]          ; atomic increment
```

The LOCK prefix on x86 asserts the cache line as Exclusive (MESI M state) for the entire duration of the read-modify-write. No other core can touch it.

**ARM: Load-Link / Store-Conditional (LL/SC)**

```
LDXR  x0, [x1]   ; Load Exclusive — marks address as monitored
; ... compute new value ...
STXR  w2, x0, [x1]  ; Store Exclusive — fails (w2=1) if anyone else wrote to [x1] since LDXR
CBNZ  w2, retry  ; if failed, retry the whole loop
```

ARM does not lock the bus. It uses a reservation mechanism. If another core modifies the address between LDXR and STXR, the store fails and you retry. This is optimistic concurrency — cheaper when contention is low.

### Memory Ordering in Atomics

Atomics do not just give you atomicity. They also carry **memory ordering semantics.** This is where Rust and C11 express them precisely:

```rust
// Rust
use std::sync::atomic::{AtomicU64, Ordering};

let x = AtomicU64::new(0);

x.store(1, Ordering::Relaxed);
// Atomic store. No ordering guarantee relative to other memory ops.
// Only guarantees: this store is atomic (no torn write).

x.store(1, Ordering::Release);
// All memory writes BEFORE this point are visible to any thread
// that subsequently does an Acquire load on the same variable.

x.load(Ordering::Acquire);
// All memory reads/writes AFTER this point see all writes made
// before a corresponding Release store.

x.store(1, Ordering::SeqCst);
// Total sequential consistency. Most expensive. Only use when needed.
```

**In the Linux kernel (C):**

```c
atomic_t counter = ATOMIC_INIT(0);

atomic_inc(&counter);                   /* atomic increment */
atomic_add(5, &counter);               /* atomic add */
int old = atomic_cmpxchg(&counter, expected, new); /* CAS */

/* With explicit ordering: */
smp_store_release(&flag, 1);           /* release store */
int val = smp_load_acquire(&flag);     /* acquire load */
```

### Relaxed vs Acquire/Release — Why It Matters

```c
/* Thread A */
data = 42;                       /* (1) plain write */
smp_store_release(&ready, 1);    /* (2) release store */

/* Thread B */
while (!smp_load_acquire(&ready)); /* (3) acquire load — spins until 1 */
use(data);                          /* (4) sees 42, guaranteed */
```

The acquire on step (3) creates a happens-before edge with the release on step (2). This guarantees step (4) sees everything written before step (2), including step (1). If you used `Relaxed` for both, you have no such guarantee. Thread B's load of `data` could see a stale value even after seeing `ready == 1`.

---

## 6. Why Rust's Ownership Model Reflects Hardware

This is the synthesis. Rust's borrow checker is not a language designer's nicety. It is a **compile-time enforcement of the MESI coherency protocol's core invariant.**

```
MESI hardware invariant:
  A cache line can be:
    Modified (one owner, exclusive write access), OR
    Shared (many readers, no writers)
  Never both simultaneously.

Rust borrow checker invariant:
  A value can have:
    One mutable reference (&mut T) — exclusive, like Modified, OR
    Many immutable references (&T) — shared reads, like Shared
  Never both simultaneously.
```

This is not a coincidence. It is the same invariant enforced at two different levels of the stack.

The MESI hardware cannot enforce this at the language level — it only handles what actually executes. The borrow checker enforces it at compile time, before execution, eliminating the entire class of data races that would require MESI, barriers, and atomics to handle at runtime.

When you reach for raw pointers (`*mut T`, `*const T`), you are stepping outside this guarantee. You are now responsible for manually providing what MESI + barriers + atomics provide in hardware. That is exactly what `unsafe` means in Rust — not "dangerous arbitrary code" but "the programmer is taking on the proof obligation that the compiler can no longer verify."

---

## The Relationship — One Sentence Each

```
Cache Coherency:  Hardware ensures all cores eventually see a consistent value.
Memory Barriers:  You control WHEN that consistency becomes visible and in what ORDER.
Volatile:         You tell the compiler to not optimize away accesses to hardware registers.
Atomics:          You make read-modify-write indivisible AND carry memory ordering semantics.
Rust Ownership:   Compile-time enforcement of the exclusive-or-shared invariant that makes
                  all of the above unnecessary when your access patterns fit the model.
```

---

## Guiding Questions for You

1. MESI gives you cache coherency, so why does a store barrier still need to exist on an x86 SMP machine?

2. In a Linux NIC driver, you write a DMA descriptor and then ring the doorbell register. What can go wrong if you do those two operations without a barrier between them — and which type of barrier?

3. In Rust, `AtomicU64::fetch_add(1, Ordering::Relaxed)` is atomic but uses `Relaxed` ordering. Name a real case where `Relaxed` is correct and sufficient, and a real case where it is dangerously wrong.

4. The kernel uses `READ_ONCE()` and `WRITE_ONCE()` macros. What problem are they solving that neither `volatile` alone nor atomics alone solve?

These four questions span the entire chain from hardware to Rust. Work through them.

Your existing notes cover the general case well. For **net subsystem development specifically**, the answer is more nuanced — the right config strategy depends on a 3-axis decision:

```
Axis 1: Build speed vs hardware fidelity
Axis 2: QEMU iteration vs real hardware testing
Axis 3: Subsystem coverage vs full distro parity
```

Let me give you the expert answer for each scenario, then the recommended layered approach.

---

## The Core Problem With Generic Advice

`cp /boot/config-$(uname -r) .config && make olddefconfig` is the **safe beginner choice** — but for net subsystem dev it has two real problems:

```
Problem 1: Ubuntu's distro config enables ~12,000 options.
           Build time: 45-60 min on 8 cores.
           For net subsystem, you touch ~200 files.
           You're compiling 11,800 things you never touch.

Problem 2: Distro config DISABLES most net debug options.
           CONFIG_NET_DROP_MONITOR, CONFIG_DYNAMIC_DEBUG,
           CONFIG_DEBUG_NET — all off by default in Ubuntu kernels.
           You need these. You won't know they're missing until
           you're blind to a kernel bug.
```

---

## The Recommended Strategy: Layered Config

Think of it as three layers applied in sequence:

```
Layer 1: Minimal bootable QEMU base
         (fast builds, deterministic environment)
         
Layer 2: Net subsystem essentials
         (everything net/socket/protocol related)
         
Layer 3: Debug and observability
         (what kernel devs actually use to find bugs)
```

---

## Step-by-Step: The Expert Net Subsystem Config

### Phase 1 — Start from QEMU-optimized base

```bash
cd ~/path/to/linux

# Start with the x86_64 KVM/QEMU-tuned config
# This is the RIGHT starting point for subsystem dev — not your distro config
make x86_64_defconfig
make kvm_guest.config
```

`kvm_guest.config` is a **config fragment** maintained by the kernel developers at `kernel/configs/kvm_guest.config` specifically for running kernels under KVM. It enables virtio, serial console, 9P — exactly what you need.

```bash
# Verify these are now set:
grep -E "CONFIG_(VIRTIO|KVM_GUEST|9P|SERIAL_8250_CONSOLE)" .config
```

### Phase 2 — Apply the net subsystem config fragment

```bash
# The kernel ships a network-specific config fragment too
# It lives at: net/Kconfig fragments — but easier via scripts/config

# Core networking stack (probably already in defconfig, verify):
scripts/config --enable CONFIG_NET
scripts/config --enable CONFIG_INET
scripts/config --enable CONFIG_IPV6
scripts/config --enable CONFIG_PACKET
scripts/config --enable CONFIG_UNIX

# Network namespaces — CRITICAL for testing isolation
scripts/config --enable CONFIG_NET_NS
scripts/config --enable CONFIG_USER_NS    # needed for unpriv netns

# Virtual devices — your test lab inside QEMU
scripts/config --enable CONFIG_VETH       # veth pairs (backbone of container net)
scripts/config --enable CONFIG_DUMMY      # dummy interfaces
scripts/config --enable CONFIG_TUN        # TUN/TAP
scripts/config --enable CONFIG_BRIDGE     # L2 bridging
scripts/config --enable CONFIG_VLAN_8021Q # 802.1Q VLAN

# Tunneling protocols — your Lumen background
scripts/config --enable CONFIG_VXLAN
scripts/config --enable CONFIG_GENEVE
scripts/config --enable CONFIG_GRE
scripts/config --enable CONFIG_IP_GRE
scripts/config --enable CONFIG_IPIP
scripts/config --enable CONFIG_IP6_GRE

# eBPF/XDP — non-negotiable for modern net subsystem work
scripts/config --enable CONFIG_BPF_SYSCALL
scripts/config --enable CONFIG_XDP_SOCKETS
scripts/config --enable CONFIG_BPF_JIT
scripts/config --enable CONFIG_BPF_JIT_ALWAYS_ON
scripts/config --enable CONFIG_CGROUP_BPF
scripts/config --enable CONFIG_BPF_EVENTS
scripts/config --enable CONFIG_BPF_STREAM_PARSER

# TC (Traffic Control) — needed for BPF tc programs
scripts/config --enable CONFIG_NET_SCHED
scripts/config --enable CONFIG_NET_SCH_INGRESS
scripts/config --enable CONFIG_NET_CLS_BPF
scripts/config --enable CONFIG_NET_ACT_BPF

# BGP/routing infrastructure
scripts/config --enable CONFIG_IP_ADVANCED_ROUTER
scripts/config --enable CONFIG_IP_MULTIPLE_TABLES
scripts/config --enable CONFIG_IPV6_MULTIPLE_TABLES
scripts/config --enable CONFIG_IP_ROUTE_MULTIPATH

# IPsec (your mTLS/IPsec background)
scripts/config --enable CONFIG_XFRM
scripts/config --enable CONFIG_XFRM_USER
scripts/config --enable CONFIG_XFRM_STATISTICS
scripts/config --enable CONFIG_NET_KEY
scripts/config --enable CONFIG_INET_ESP
scripts/config --enable CONFIG_INET_AH
```

### Phase 3 — Debug and observability (most missed, most critical)

```bash
# BTF — mandatory for bpftool, libbpf, Cilium debugging
scripts/config --enable CONFIG_DEBUG_INFO
scripts/config --enable CONFIG_DEBUG_INFO_BTF
scripts/config --enable CONFIG_DEBUG_INFO_DWARF5

# Dynamic debug — lets you enable pr_debug() at runtime with no recompile
scripts/config --enable CONFIG_DYNAMIC_DEBUG
scripts/config --enable CONFIG_DYNAMIC_DEBUG_CORE

# Net drop monitor — see WHERE in the stack packets are dropped
scripts/config --enable CONFIG_NET_DROP_MONITOR

# Socket monitoring — ss/iproute2 advanced stats
scripts/config --enable CONFIG_INET_DIAG
scripts/config --enable CONFIG_INET_UDP_DIAG
scripts/config --enable CONFIG_INET_RAW_DIAG

# Tracing infrastructure (ftrace, perf events for net functions)
scripts/config --enable CONFIG_FTRACE
scripts/config --enable CONFIG_FUNCTION_TRACER
scripts/config --enable CONFIG_DYNAMIC_FTRACE
scripts/config --enable CONFIG_KPROBES
scripts/config --enable CONFIG_UPROBES
scripts/config --enable CONFIG_PERF_EVENTS

# KASAN — catches memory bugs in kernel (priceless for net buf handling)
# WARNING: 2-3x slowdown, only enable when hunting bugs
# scripts/config --enable CONFIG_KASAN
# scripts/config --enable CONFIG_KASAN_INLINE

# Lockdep — catches deadlocks in net locking paths
# Also a slowdown, enable targeted:
# scripts/config --enable CONFIG_PROVE_LOCKING
# scripts/config --enable CONFIG_LOCKDEP

# Netfilter (even if not your focus — many tests assume it)
scripts/config --enable CONFIG_NETFILTER
scripts/config --enable CONFIG_NF_CONNTRACK
scripts/config --enable CONFIG_NF_TABLES
```

### Phase 4 — Resolve and finalize

```bash
# After all scripts/config changes, always run this:
# It resolves dependencies (enabling one option may require enabling others)
make olddefconfig

# Sanity check — verify your key options survived:
for opt in BPF_SYSCALL XDP_SOCKETS VXLAN GENEVE NET_NS DEBUG_INFO_BTF; do
    val=$(grep "CONFIG_${opt}" .config | head -1)
    printf "%-35s %s\n" "CONFIG_${opt}" "${val:-NOT FOUND}"
done
```

---

## Expected Config State After This Process

```
CONFIG_BPF_SYSCALL=y
CONFIG_XDP_SOCKETS=y
CONFIG_BPF_JIT=y
CONFIG_DEBUG_INFO_BTF=y
CONFIG_VXLAN=m
CONFIG_GENEVE=m
CONFIG_NET_NS=y
CONFIG_VETH=m
CONFIG_DYNAMIC_DEBUG=y
CONFIG_NET_DROP_MONITOR=m
CONFIG_KPROBES=y
...
```

---

## Build Time Comparison

```
Strategy                       Approx Build Time (8 cores)
─────────────────────────────────────────────────────────
Ubuntu distro config           45-60 min
defconfig + kvm + net layers   8-12 min      ← your target
defconfig alone                5-7 min
Single subsystem (net/ only)   ~30 seconds   ← for fast checks
```

The 8-12 min build is your **daily driver**. For a single-file change in `net/ipv4/tcp.c`, incremental rebuild after that is 3-10 seconds.

---

## One More Weapon: `localmodconfig`

This is an underused tool that many professional kernel devs rely on:

```bash
# First, make sure all the kernel modules you use daily are LOADED
# on your running system:
lsmod

# Then run:
make LSMOD=/tmp/my-modules.txt localmodconfig
# It reads the currently loaded modules and generates a config that
# includes EXACTLY those modules — nothing more.

# Steps:
lsmod > /tmp/my-modules.txt
make LSMOD=/tmp/my-modules.txt localmodconfig
# Result: very lean config tuned to YOUR machine's actual hardware
```

The catch: `localmodconfig` only captures what's loaded NOW. If you haven't exercised your VXLAN/GRE interfaces, those modules won't be in `lsmod` and will be excluded. Fix: load all relevant modules before running it:

```bash
sudo modprobe vxlan geneve gre ip_gre veth dummy tun
sudo modprobe xfrm_user esp4 ah4
lsmod > /tmp/my-modules.txt
make LSMOD=/tmp/my-modules.txt localmodconfig

# Then add the net debug options on top via scripts/config (Phase 3 above)
make olddefconfig
```

---

## Decision Map for Your Specific Case

```
You are developing Linux net subsystem:

Are you iterating on kernel logic (socket, tcp, xdp code paths)?
  YES → x86_64_defconfig + kvm_guest.config + net layers (above)
        Build in QEMU. 8-12 min first build, ~5 sec incremental.

Are you debugging a specific crash/memory bug?
  YES → Same config + enable CONFIG_KASAN + CONFIG_PROVE_LOCKING
        Accept 2-3x slowdown. Worth it.

Are you testing with real tools (iproute2, bpftool, Cilium)?
  YES → Ensure CONFIG_DEBUG_INFO_BTF=y. Without BTF, bpftool is blind.

Do you need to test on real hardware (not QEMU)?
  YES → cp /boot/config-$(uname -r) .config + olddefconfig
        Then layer your net debug options on top.
        Don't reboot until QEMU confirms it works.
```

---

The mental model to carry forward: **your `.config` is a test harness contract, not just a compilation manifest.** Every `CONFIG_DEBUG_*` and `CONFIG_NET_DROP_MONITOR` you enable is an instrumentation point — the kernel's way of exposing its internal state to you. Kernel subsystem devs who miss this work blind. The time cost of the extra options is trivial compared to the observability they unlock.

Good — now the picture is clear. Let me map your exact topology first, then give you the precise config strategy for it.

---

## Your Actual Topology

```
┌─────────────────────────────────────────────────┐
│  HOST: Dell G3 3500 — Ubuntu Desktop (Wayland)  │
│                                                  │
│  ┌─────────────────────────────────────────┐    │
│  │  KVM VM: Ubuntu Server                  │    │
│  │                                         │    │
│  │  - systemd, udev, real userspace        │    │
│  │  - iproute2, bpftool, clang/llvm        │    │
│  │  - virtio_net (your XDP test NIC)       │    │
│  │  - serial console: ttyS0               │    │
│  └─────────────────────────────────────────┘    │
│         ↑                        ↑              │
│   build kernel here       virsh console         │
│   copy bzImage + modules → install in VM        │
└─────────────────────────────────────────────────┘
```

This is a **build-on-host, run-in-VM** workflow. The critical insight this changes:

> You must NOT use your host's `/boot/config-$(uname -r)` as the base. That is the host kernel config — it knows nothing about virtio, the VM's hardware profile, or Ubuntu Server's init requirements.

---

## The Right Config Source: Pull From the VM Itself

```bash
# Get the config that currently boots your Ubuntu Server VM
# (run this on your HOST)
scp user@<vm-ip>:/boot/config-$(uname -r) ~/kernel-dev/vm-base.config

# Copy into your kernel source tree
cp ~/kernel-dev/vm-base.config /path/to/linux/.config

# Resolve any new options from your kernel version vs VM's kernel version
make olddefconfig
```

This gives you a config that:
- Already has virtio_net, virtio_blk, virtio_pci correctly set
- Already satisfies systemd/udev's kernel feature requirements
- Already has serial console enabled (Ubuntu Server sets this)
- Will boot the VM with real userspace intact

If you can't SCP (VM not running), use `virsh`:

```bash
# Alternative: copy config from VM disk via virsh
virsh start <vm-name>
virsh console <vm-name>
# inside VM:
cat /boot/config-$(uname -r) > /tmp/vm.config
# back on host via scp
```

---

## Layer XDP/eBPF Development Options On Top

After the base config is in place:

```bash
cd /path/to/linux

# ── Core BPF ────────────────────────────────────────────────
scripts/config --enable  CONFIG_BPF_SYSCALL
scripts/config --enable  CONFIG_BPF_JIT
scripts/config --enable  CONFIG_BPF_JIT_ALWAYS_ON
scripts/config --enable  CONFIG_BPF_EVENTS
scripts/config --enable  CONFIG_BPF_STREAM_PARSER
scripts/config --enable  CONFIG_CGROUP_BPF

# ── XDP ─────────────────────────────────────────────────────
scripts/config --enable  CONFIG_XDP_SOCKETS
scripts/config --enable  CONFIG_XDP_SOCKETS_DIAG      # AF_XDP diagnostics

# ── virtio_net XDP — THIS IS THE CRITICAL ONE ────────────────
# virtio_net supports XDP but the config must be right
scripts/config --enable  CONFIG_VIRTIO_NET
# XDP on virtio_net requires XDP_SOCKETS — already done above
# Also requires: kernel ≥ 4.10 for basic XDP, ≥ 5.1 for AF_XDP zero-copy

# ── TC BPF (tc-bpf programs, cls_bpf) ───────────────────────
scripts/config --enable  CONFIG_NET_SCHED
scripts/config --enable  CONFIG_NET_SCH_INGRESS        # needed for tc redirect
scripts/config --enable  CONFIG_NET_CLS_BPF            # cls_bpf classifier
scripts/config --enable  CONFIG_NET_ACT_BPF            # act_bpf action
scripts/config --enable  CONFIG_NET_CLS_ACT

# ── Your specific domain: VXLAN, GRE, GENEVE, IPsec ─────────
scripts/config --enable  CONFIG_VXLAN
scripts/config --enable  CONFIG_GENEVE
scripts/config --enable  CONFIG_GRE
scripts/config --enable  CONFIG_IP_GRE
scripts/config --enable  CONFIG_IP6_GRE
scripts/config --enable  CONFIG_XFRM
scripts/config --enable  CONFIG_XFRM_USER
scripts/config --enable  CONFIG_INET_ESP
scripts/config --enable  CONFIG_INET_AH

# ── Network namespaces (your test isolation layer) ───────────
scripts/config --enable  CONFIG_NET_NS
scripts/config --enable  CONFIG_USER_NS
scripts/config --enable  CONFIG_VETH

# ── BTF — bpftool/libbpf/Cilium are BLIND without this ──────
scripts/config --enable  CONFIG_DEBUG_INFO
scripts/config --enable  CONFIG_DEBUG_INFO_BTF
scripts/config --set-val CONFIG_DEBUG_INFO_REDUCED  n    # BTF needs full debug info
scripts/config --enable  CONFIG_DEBUG_INFO_DWARF5

# ── Serial console — confirm it survives the config merge ────
scripts/config --enable  CONFIG_SERIAL_8250
scripts/config --enable  CONFIG_SERIAL_8250_CONSOLE
scripts/config --enable  CONFIG_TTY

# ── Tracing/observability ────────────────────────────────────
scripts/config --enable  CONFIG_KPROBES
scripts/config --enable  CONFIG_KPROBE_EVENTS
scripts/config --enable  CONFIG_UPROBES
scripts/config --enable  CONFIG_UPROBE_EVENTS
scripts/config --enable  CONFIG_FTRACE
scripts/config --enable  CONFIG_FUNCTION_TRACER
scripts/config --enable  CONFIG_DYNAMIC_FTRACE
scripts/config --enable  CONFIG_DYNAMIC_DEBUG
scripts/config --enable  CONFIG_NET_DROP_MONITOR
scripts/config --enable  CONFIG_TRACEPOINTS

# ── Resolve all dependencies ─────────────────────────────────
make olddefconfig
```

---

## The Transfer-Install-Boot Cycle

This is your core iteration loop. Automate it from day one:

```bash
# On HOST — save as ~/bin/kdeploy
#!/bin/bash
set -e

VM_USER="your-user"
VM_IP="192.168.122.x"          # get with: virsh domifaddr <vm-name>
KDIR="/path/to/linux"
VM_KERNEL_VERSION=$(make -s -C "$KDIR" kernelversion)

echo "── [1/4] Building kernel ──"
cd "$KDIR"
make -j$(nproc) 2>&1 | tail -10

echo "── [2/4] Copying bzImage ──"
scp arch/x86/boot/bzImage ${VM_USER}@${VM_IP}:/tmp/bzImage-dev

echo "── [3/4] Installing modules ──"
# installs to /lib/modules/<version>/ inside VM
make INSTALL_MOD_PATH=/tmp/kmod-staging modules_install
tar -czf /tmp/kmod.tar.gz -C /tmp/kmod-staging .
scp /tmp/kmod.tar.gz ${VM_USER}@${VM_IP}:/tmp/

echo "── [4/4] Installing on VM ──"
ssh ${VM_USER}@${VM_IP} << 'REMOTE'
  sudo cp /tmp/bzImage-dev /boot/vmlinuz-dev
  sudo tar -xzf /tmp/kmod.tar.gz -C /
  sudo depmod -a
  # Update GRUB — add entry for dev kernel
  sudo update-grub
  echo "Ready. Run: sudo grub-reboot 'Advanced options > dev kernel'"
  sudo reboot
REMOTE

echo "── Connecting serial console in 5s ──"
sleep 5
virsh console <vm-name>
```

```bash
chmod +x ~/bin/kdeploy
kdeploy
```

---

## Serial Console — Verify It's Configured Correctly

On the VM's GRUB config, the kernel cmdline must have:

```bash
# Inside VM: check current cmdline
cat /proc/cmdline
# Should contain: console=tty0 console=ttyS0,115200n8
# Both tty0 AND ttyS0 — tty0 for local VGA, ttyS0 for your serial access
```

If it's missing, edit `/etc/default/grub` on the VM:

```bash
# In VM:
sudo vim /etc/default/grub

# Change this line:
GRUB_CMDLINE_LINUX_DEFAULT="quiet splash"

# To this:
GRUB_CMDLINE_LINUX_DEFAULT="console=tty0 console=ttyS0,115200n8 nokaslr"
#                                                                  ↑
#                                              nokaslr: fixed kernel addresses
#                                              helps bpftool/perf symbol resolution

sudo update-grub
```

On the HOST, connect:

```bash
virsh console <vm-name>
# Disconnect: Ctrl+]
```

---

## Verify XDP Works After Boot

Once you're in the VM via serial console:

```bash
# Verify BPF subsystem loaded
bpftool version
# output should show: libbpf version x.x, features: ...

# Verify BTF is present (critical)
bpftool btf show
# should list vmlinux BTF

# Verify XDP socket support
ls /proc/net/xdp    # should exist

# Verify virtio_net supports XDP
ethtool -i eth0     # or ens3, whatever your VM's NIC is
# driver: virtio_net

# Attach a trivial XDP pass program to confirm the pipeline works
ip link set dev eth0 xdp obj xdp_pass.o sec xdp
ip link show eth0   # should show xdp/id: <N>
ip link set dev eth0 xdp off
```

---

## One Hidden Detail Most People Miss

`CONFIG_DEBUG_INFO_BTF` requires `pahole` ≥ 1.16 installed on the **build host**, not the VM:

```bash
# On HOST:
pahole --version
# If < 1.16 or missing:
sudo apt install dwarves    # Ubuntu 20.04+

# If apt version is too old (Ubuntu 18.04):
git clone https://github.com/acmel/dwarves
cd dwarves && mkdir build && cd build
cmake -D__LIB=lib .. && make && sudo make install
```

Without this, the build will silently disable BTF even though you set `CONFIG_DEBUG_INFO_BTF=y` — and `bpftool` will be unable to do CO-RE (Compile Once Run Everywhere) relocations, which kills modern libbpf programs.

---

## The Mental Model for This Setup

```
You have two distinct kernel roles:

HOST kernel     = not your concern. Ubuntu manages it.
                  Never install your dev kernel here.

VM kernel       = your laboratory. Crash it freely.
                  Config must satisfy: Ubuntu Server userspace
                  + virtio hardware + XDP/BPF development needs.

Serial console  = your lifeline when the VM kernel panics.
                  Always confirm it works BEFORE installing a new kernel.
                  A kernel that kills the serial console = blind debugging.
```

The serial console test before every new kernel install:

```bash
# Before rebooting into new kernel — confirm serial is alive:
virsh console <vm-name>
# press Enter — you should see a login prompt or systemd output
# If dead → your new kernel has a console config problem → fix before rebooting
```

This is a full runbook. I'll go phase by phase — no gaps.

---

## The Complete Picture Before You Touch Anything

```
PHASE 1: Prepare host (KVM tools, kernel build deps)
PHASE 2: Create VM + install Ubuntu Server from ISO
PHASE 3: Configure VM for kernel dev (serial, tools, SSH)
PHASE 4: Extract VM's kernel config → build kernel 7.0.6 on host
PHASE 5: Deploy built kernel into VM
PHASE 6: Boot custom kernel, verify XDP/eBPF stack
```

```
HOST (Ubuntu Desktop)                    VM (Ubuntu Server)
─────────────────────                    ──────────────────
libvirt/KVM/QEMU          manages →     virtio-net (XDP target)
kernel source tree                       systemd + real userspace
build tools (gcc, pahole)               bpftool, clang, iproute2
kdeploy script            ssh/scp →     /boot/vmlinuz-dev
                          serial  →     ttyS0 (your console)
```

---

## PHASE 1 — Prepare the Host

### 1.1 Install KVM stack

```bash
sudo apt update
sudo apt install -y \
    qemu-kvm libvirt-daemon-system libvirt-clients \
    bridge-utils virtinst virt-manager \
    cpu-checker

# Verify hardware virtualization is available
kvm-ok
# Must see: INFO: /dev/kvm exists — KVM acceleration can be used
# If not: enter BIOS → enable Intel VT-x or AMD-V

# Add yourself to both groups (logout+login after this)
sudo usermod -aG kvm,libvirt $USER
newgrp libvirt
newgrp kvm

# Verify libvirt daemon is running
sudo systemctl enable --now libvirtd
virsh version
# Should print: libvirt version x.x.x
```

### 1.2 Install kernel build dependencies on host

```bash
sudo apt install -y \
    build-essential gcc g++ make \
    libssl-dev libelf-dev \
    flex bison bc \
    pahole dwarves \
    libncurses-dev \
    git fakeroot \
    cpio xz-utils

# Verify pahole version — must be >= 1.16 for BTF support
pahole --version
# If < 1.16 or missing on older Ubuntu:
# sudo apt install -y dwarves
```

### 1.3 Get kernel 7.0.6 source on host

```bash
mkdir -p ~/kernel-dev && cd ~/kernel-dev

# Download from kernel.org
wget https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.0.6.tar.xz
wget https://cdn.kernel.org/pub/linux/kernel/v7.x/linux-7.0.6.tar.sign

# Extract
tar -xf linux-7.0.6.tar.xz
cd linux-7.0.6

# Confirm tree structure
ls
# arch  block  certs  crypto  Documentation  drivers
# fs    include  init  ipc  kernel  lib  mm  net  ...
```

---

## PHASE 2 — Create VM and Install Ubuntu Server

### 2.1 Create the VM with the right hardware profile

The NIC model matters: `virtio` is the only one that supports XDP. Add **two NICs** — one for management (SSH), one dedicated as your XDP test interface.

```bash
# Create disk image — 40GB is enough
qemu-img create -f qcow2 ~/kernel-dev/ubuntu-server-dev.qcow2 40G

# Create the VM (virt-install)
virt-install \
    --name kernel-dev \
    --ram 4096 \
    --vcpus 4 \
    --cpu host-passthrough \
    --os-variant ubuntu22.04 \
    --disk path=~/kernel-dev/ubuntu-server-dev.qcow2,bus=virtio \
    --cdrom /path/to/ubuntu-server.iso \
    --network network=default,model=virtio \
    --network network=default,model=virtio \
    --graphics none \
    --console pty,target.type=virtio \
    --serial pty \
    --extra-args "console=tty0 console=ttyS0,115200n8" \
    --boot cdrom,hd

# --cpu host-passthrough  : exposes host CPU flags to guest
#                           required for some eBPF JIT features
# Two --network lines     : ens3 = mgmt, ens4 = XDP test NIC
# --graphics none         : headless, serial only
# --serial pty            : creates /dev/pts/X on host for virsh console
```

### 2.2 Ubuntu Server install — what to choose

The installer will run in the terminal via serial. Key choices:

```
Profile setup:
  Your name:     dev
  Server name:   kernel-dev
  Username:      dev
  Password:      (set one)

Network:
  ens3 → configure with DHCP (management)
  ens4 → leave unconfigured for now (XDP test NIC)

Storage:
  Use entire disk
  No LVM (simpler for kernel dev)

SSH:
  ✓ Install OpenSSH server   ← MANDATORY

Snaps:
  Skip all (press Done)
```

After install completes, the VM reboots. Connect:

```bash
virsh console kernel-dev
# Press Enter → Ubuntu Server login prompt
# Login with dev / your password
```

---

## PHASE 3 — Configure VM for Kernel Development

Do this INSIDE the VM via serial console or SSH.

### 3.1 Get VM IP and switch to SSH (more comfortable than serial)

```bash
# Inside VM via virsh console:
ip addr show ens3
# Note the IP — something like 192.168.122.x

# From host, SSH in (easier to paste commands):
ssh dev@192.168.122.x
```

### 3.2 Configure GRUB for serial console + kernel dev flags

```bash
# Inside VM:
sudo nano /etc/default/grub
```

Set these exact values:

```bash
GRUB_DEFAULT=0
GRUB_TIMEOUT=5
GRUB_TIMEOUT_STYLE=menu           # show menu — lets you pick kernel on reboot

# Serial console: tty0 = local VGA, ttyS0 = your virsh console
GRUB_CMDLINE_LINUX_DEFAULT=""
GRUB_CMDLINE_LINUX="console=tty0 console=ttyS0,115200n8 nokaslr"
#                                                         ↑
#                                    nokaslr: fixed kernel symbols
#                                    bpftool/perf work better with this

# Enable serial in GRUB menu itself
GRUB_TERMINAL="serial console"
GRUB_SERIAL_COMMAND="serial --speed=115200 --unit=0 --word=8 --parity=no --stop=1"
```

```bash
sudo update-grub
```

### 3.3 Install XDP/eBPF test tools inside VM

```bash
sudo apt update
sudo apt install -y \
    clang llvm \
    libbpf-dev \
    linux-tools-common linux-tools-generic \
    bpftool \
    iproute2 \
    iputils-ping \
    tcpdump \
    netcat-openbsd \
    python3 \
    strace \
    perf-tools-unstable

# Verify bpftool works
bpftool version
```

### 3.4 Extract the VM's kernel config — this is your build baseline

```bash
# Inside VM:
uname -r
# e.g.: 6.8.0-57-server

# Copy it to host
# (run from HOST):
scp dev@192.168.122.x:/boot/config-$(ssh dev@192.168.122.x uname -r) \
    ~/kernel-dev/linux-7.0.6/.config

# Verify it arrived
ls -la ~/kernel-dev/linux-7.0.6/.config
# -rw-r--r-- 1 dev dev 240K  .config
```

---

## PHASE 4 — Configure and Build Kernel 7.0.6

### 4.1 Apply the VM config as base and resolve new options

```bash
cd ~/kernel-dev/linux-7.0.6

# .config is already there from scp above
# Resolve: fills new 7.x options with defaults, keeps your VM's options
make olddefconfig

# Handle Ubuntu-specific cert options that cause build failures:
scripts/config --disable SYSTEM_TRUSTED_KEYS
scripts/config --disable SYSTEM_REVOCATION_KEYS
scripts/config --disable MODULE_SIG_KEY
```

### 4.2 Apply XDP/eBPF development config layers

```bash
# ── BPF core ────────────────────────────────────────────────
scripts/config --enable  CONFIG_BPF_SYSCALL
scripts/config --enable  CONFIG_BPF_JIT
scripts/config --enable  CONFIG_BPF_JIT_ALWAYS_ON
scripts/config --enable  CONFIG_BPF_EVENTS
scripts/config --enable  CONFIG_BPF_STREAM_PARSER
scripts/config --enable  CONFIG_CGROUP_BPF

# ── XDP ─────────────────────────────────────────────────────
scripts/config --enable  CONFIG_XDP_SOCKETS
scripts/config --enable  CONFIG_XDP_SOCKETS_DIAG

# ── virtio_net (your XDP target NIC inside KVM) ──────────────
scripts/config --enable  CONFIG_VIRTIO
scripts/config --enable  CONFIG_VIRTIO_NET
scripts/config --enable  CONFIG_VIRTIO_PCI

# ── TC/BPF pipeline ─────────────────────────────────────────
scripts/config --enable  CONFIG_NET_SCHED
scripts/config --enable  CONFIG_NET_SCH_INGRESS
scripts/config --enable  CONFIG_NET_CLS_BPF
scripts/config --enable  CONFIG_NET_ACT_BPF
scripts/config --enable  CONFIG_NET_CLS_ACT

# ── Tunnels (your domain) ────────────────────────────────────
scripts/config --enable  CONFIG_VXLAN
scripts/config --enable  CONFIG_GENEVE
scripts/config --enable  CONFIG_GRE
scripts/config --enable  CONFIG_IP_GRE
scripts/config --enable  CONFIG_IP6_GRE
scripts/config --enable  CONFIG_IPIP

# ── IPsec ───────────────────────────────────────────────────
scripts/config --enable  CONFIG_XFRM
scripts/config --enable  CONFIG_XFRM_USER
scripts/config --enable  CONFIG_XFRM_STATISTICS
scripts/config --enable  CONFIG_INET_ESP
scripts/config --enable  CONFIG_INET_AH

# ── Network namespaces + virtual devices ─────────────────────
scripts/config --enable  CONFIG_NET_NS
scripts/config --enable  CONFIG_USER_NS
scripts/config --enable  CONFIG_VETH
scripts/config --enable  CONFIG_DUMMY
scripts/config --enable  CONFIG_TUN
scripts/config --enable  CONFIG_BRIDGE

# ── BTF — bpftool / libbpf / CO-RE require this ─────────────
scripts/config --enable  CONFIG_DEBUG_INFO
scripts/config --set-val CONFIG_DEBUG_INFO_REDUCED  n
scripts/config --enable  CONFIG_DEBUG_INFO_BTF
scripts/config --enable  CONFIG_DEBUG_INFO_DWARF5

# ── Serial console — NEVER disable this ─────────────────────
scripts/config --enable  CONFIG_SERIAL_8250
scripts/config --enable  CONFIG_SERIAL_8250_CONSOLE
scripts/config --enable  CONFIG_TTY

# ── Observability / tracing ──────────────────────────────────
scripts/config --enable  CONFIG_KPROBES
scripts/config --enable  CONFIG_KPROBE_EVENTS
scripts/config --enable  CONFIG_UPROBES
scripts/config --enable  CONFIG_UPROBE_EVENTS
scripts/config --enable  CONFIG_FTRACE
scripts/config --enable  CONFIG_FUNCTION_TRACER
scripts/config --enable  CONFIG_DYNAMIC_FTRACE
scripts/config --enable  CONFIG_DYNAMIC_DEBUG
scripts/config --enable  CONFIG_NET_DROP_MONITOR
scripts/config --enable  CONFIG_TRACEPOINTS
scripts/config --enable  CONFIG_PERF_EVENTS

# ── Resolve all dependencies ─────────────────────────────────
make olddefconfig
```

### 4.3 Verify critical options before building

```bash
echo "=== Critical options check ==="
for opt in \
    BPF_SYSCALL BPF_JIT XDP_SOCKETS VIRTIO_NET \
    DEBUG_INFO_BTF NET_NS VXLAN SERIAL_8250_CONSOLE \
    KPROBES DYNAMIC_DEBUG NET_DROP_MONITOR; do
    val=$(grep "CONFIG_${opt}=" .config | head -1)
    printf "  %-35s %s\n" "CONFIG_${opt}" "${val:-⚠ NOT SET}"
done
```

Expected output:
```
  CONFIG_BPF_SYSCALL                CONFIG_BPF_SYSCALL=y
  CONFIG_BPF_JIT                    CONFIG_BPF_JIT=y
  CONFIG_XDP_SOCKETS                CONFIG_XDP_SOCKETS=y
  CONFIG_VIRTIO_NET                 CONFIG_VIRTIO_NET=y
  CONFIG_DEBUG_INFO_BTF             CONFIG_DEBUG_INFO_BTF=y
  CONFIG_NET_NS                     CONFIG_NET_NS=y
  CONFIG_VXLAN                      CONFIG_VXLAN=m
  CONFIG_SERIAL_8250_CONSOLE        CONFIG_SERIAL_8250_CONSOLE=y
  CONFIG_KPROBES                    CONFIG_KPROBES=y
  CONFIG_DYNAMIC_DEBUG              CONFIG_DYNAMIC_DEBUG=y
  CONFIG_NET_DROP_MONITOR           CONFIG_NET_DROP_MONITOR=m
```

### 4.4 Build

```bash
# Full build — first time takes 10-20 min depending on your machine
time make -j$(nproc)

# Watch for these at the end — means success:
#   Kernel: arch/x86/boot/bzImage is ready
#   BUILD  arch/x86/boot/bzImage
```

---

## PHASE 5 — Deploy Kernel Into VM

### 5.1 Create the deploy script

Save as `~/bin/kdeploy` on host:

```bash
#!/bin/bash
set -e

VM_USER="dev"
VM_IP="192.168.122.x"          # replace with your VM's actual IP
KDIR="$HOME/kernel-dev/linux-7.0.6"
KVER=$(make -s -C "$KDIR" kernelversion)

echo "━━━ Deploying kernel ${KVER} ━━━"

# 1. Copy bzImage
echo "[1/4] Copying bzImage..."
scp "$KDIR/arch/x86/boot/bzImage" \
    "${VM_USER}@${VM_IP}:/tmp/bzImage-${KVER}"

# 2. Install modules into a staging dir, tar it, copy to VM
echo "[2/4] Packaging modules..."
STAGING=$(mktemp -d)
make -C "$KDIR" INSTALL_MOD_PATH="$STAGING" modules_install 2>/dev/null
tar -czf /tmp/kmod-${KVER}.tar.gz -C "$STAGING" .
scp /tmp/kmod-${KVER}.tar.gz "${VM_USER}@${VM_IP}:/tmp/"
rm -rf "$STAGING"

# 3. Install on VM
echo "[3/4] Installing on VM..."
ssh "${VM_USER}@${VM_IP}" "bash -s" << REMOTE
  set -e
  sudo cp /tmp/bzImage-${KVER} /boot/vmlinuz-${KVER}
  
  # Extract modules
  sudo tar -xzf /tmp/kmod-${KVER}.tar.gz -C /
  sudo depmod ${KVER}
  
  # Create initramfs for the new kernel
  sudo update-initramfs -c -k ${KVER}
  
  # Add GRUB entry
  sudo update-grub
  
  # Set new kernel as next boot (once — fallback safe)
  GRUB_ENTRY=\$(grep -i "menuentry.*${KVER}" /boot/grub/grub.cfg | head -1 | \
               sed "s/menuentry '\\([^']*\\)'.*/\\1/")
  echo "GRUB entry: \$GRUB_ENTRY"
  sudo grub-reboot "\$GRUB_ENTRY"
  
  echo "━━━ Rebooting into ${KVER} ━━━"
  sudo reboot
REMOTE

# 4. Attach serial console after reboot
echo "[4/4] Connecting serial console (10s delay for boot)..."
sleep 10
virsh console kernel-dev
```

```bash
chmod +x ~/bin/kdeploy
```

### 5.2 Deploy

```bash
kdeploy
```

You'll see the boot messages stream over serial. Watch for:

```
[    0.000000] Linux version 7.0.6 ...
[    0.000000] Command line: console=tty0 console=ttyS0,115200n8 nokaslr
...
[    2.xxxxxx] bpf: BPF JIT compiler loaded
[    2.xxxxxx] NET: Registered PF_XDP protocol family
```

---

## PHASE 6 — Verify the XDP/eBPF Stack

Once logged in via serial:

### 6.1 Confirm you're running your kernel

```bash
uname -r
# 7.0.6
```

### 6.2 Verify BPF/XDP subsystem

```bash
# BTF loaded (critical)
bpftool btf show | head -5
# Should show vmlinux BTF entry

# BPF JIT enabled
cat /proc/sys/net/core/bpf_jit_enable
# 1

# XDP socket support
ls /proc/net/xdp
# should exist (empty file is fine)

# Your NICs
ip link show
# 1: lo
# 2: ens3   (management)
# 3: ens4   (XDP test NIC — this is your target)
```

### 6.3 Write and attach a minimal XDP program to confirm pipeline

On the VM, create `xdp_pass.c`:

```c
// xdp_pass.c — simplest possible XDP program
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

SEC("xdp")
int xdp_pass_fn(struct xdp_md *ctx)
{
    return XDP_PASS;   // let all packets through
}

char _license[] SEC("license") = "GPL";
```

```bash
# Compile
clang -O2 -target bpf -c xdp_pass.c -o xdp_pass.o

# Attach to ens4 (your XDP test NIC)
sudo ip link set dev ens4 xdp obj xdp_pass.o sec xdp

# Verify it loaded
ip link show ens4
# Should show: xdp/id: 1  or  prog/xdp id 1

# Check via bpftool
sudo bpftool prog show
# Should list your xdp_pass program with id, jited: yes

# Detach
sudo ip link set dev ens4 xdp off
```

---

## Day-to-Day Iteration After This Point

```
You make a change in net/ or kernel/bpf/ on HOST
    ↓
make -j$(nproc)                  # incremental: 5-30 seconds
    ↓
kdeploy                          # copies + reboots VM
    ↓
virsh console kernel-dev         # serial: watch boot, run tests
    ↓
dmesg | tail -30                 # check your printk() output
bpftool prog show                # check BPF programs
ip link show                     # check XDP attachment
```

Save this reference for your notes — this is the complete path from zero to a working XDP/eBPF kernel dev environment.

Got the full context. You have an existing KVM VM (`kernel-dev`) that was set up during this process, and you want to wipe it and start fresh. Here's the clean teardown + fresh start procedure:

---

## Teardown the Old VM

### 1. Stop and remove the VM completely

```bash
# Stop the VM if it's running
virsh destroy kernel-dev

# Undefine (removes VM definition from libvirt)
virsh undefine kernel-dev --remove-all-storage
# --remove-all-storage  ← deletes the .qcow2 disk image too

# Verify it's gone
virsh list --all
# kernel-dev should not appear
```

If the disk image was created manually (not managed by libvirt), delete it explicitly:

```bash
rm -f ~/kernel-dev/ubuntu-server-dev.qcow2
```

### 2. Clean up any leftover build artifacts on the host (optional)

```bash
# If you want a fully clean kernel build too:
cd ~/kernel-dev/linux-7.0.6
make mrproper       # removes .config + all build output

# Or nuke the whole staging and module tarballs:
rm -f /tmp/kmod-*.tar.gz /tmp/bzImage-*
```

---

## Fresh Start

From Phase 2 in your notes — just re-run from the VM creation step:

```bash
# 1. Fresh disk
qemu-img create -f qcow2 ~/kernel-dev/ubuntu-server-dev.qcow2 40G

# 2. Re-create VM (same virt-install command from PHASE 2)
virt-install \
    --name kernel-dev \
    --ram 4096 \
    --vcpus 4 \
    --cpu host-passthrough \
    --os-variant ubuntu22.04 \
    --disk path=~/kernel-dev/ubuntu-server-dev.qcow2,bus=virtio \
    --cdrom /path/to/ubuntu-server.iso \
    --network network=default,model=virtio \
    --network network=default,model=virtio \
    --graphics none \
    --console pty,target.type=virtio \
    --serial pty \
    --extra-args "console=tty0 console=ttyS0,115200n8" \
    --boot cdrom,hd
```

Then continue from **PHASE 3** (GRUB config, tools install, scp the VM's kernel config) as documented.

---

## Quick Reference: Common Virsh Commands

| Task | Command |
|---|---|
| List all VMs | `virsh list --all` |
| Stop (force) | `virsh destroy kernel-dev` |
| Delete VM + disk | `virsh undefine kernel-dev --remove-all-storage` |
| Delete VM only | `virsh undefine kernel-dev` |
| Start VM | `virsh start kernel-dev` |
| Serial console | `virsh console kernel-dev` |
| Exit serial | `Ctrl + ]` |

The key flag is `--remove-all-storage` — without it, `undefine` only removes the VM definition but leaves the disk image on disk.

## Incremental Builds in the Linux Kernel

The kernel build system (kbuild) is already incremental by default — `make` tracks timestamps and only recompiles what changed. The key is to never run `make mrproper` or `make clean` between iterations during active development.

### The Basic Incremental Build

```bash
cd ~/kernel-dev/linux-7.0.6

# After editing any file, just run this — kbuild figures out what changed
make -j$(nproc)
```

If you edited `net/ipv4/tcp.c`, only `tcp.o` and the affected link steps re-run. A single file change typically takes 5–30 seconds.

### Build Only a Specific Subsystem Directory

```bash
# Only rebuild the net/ subtree
make -j$(nproc) M=net/

# Only rebuild a specific subdirectory
make -j$(nproc) M=net/ipv4/
make -j$(nproc) M=net/core/
make -j$(nproc) M=kernel/bpf/
```

This is the fastest path when you're working within a known subsystem. It skips all other directories entirely.

### Build a Single .o File

```bash
# Rebuilds just that one translation unit
make net/ipv4/tcp.o
make kernel/bpf/verifier.o
```

Useful for checking if your file even compiles before doing a full build.

### Build Only Modules (skip vmlinux relink if unchanged)

```bash
# Only rebuild kernel modules — skips vmlinux if core is untouched
make -j$(nproc) modules
```

### Check What Would Be Rebuilt (dry run)

```bash
make -j$(nproc) --dry-run 2>&1 | head -40
# Shows exactly which files kbuild would recompile without doing it
```

### Tracking What You Changed (git-based)

Since you're working in a git tree:

```bash
# See what files you've modified vs upstream
git diff --name-only

# Build only the directories containing your changes
git diff --name-only | sed 's|/[^/]*$||' | sort -u
# Use the output to target M= builds

# Example: if git says you touched net/ipv4/tcp.c and net/core/sock.c
make -j$(nproc) M=net/
```

### The compile_commands.json for Editor Intelligence

```bash
# Generate compile_commands.json — used by clangd, nvim LSP, VS Code
make -j$(nproc) compile_commands.json
# or
scripts/clang-tools/gen_compile_commands.py
```

This lets your editor know exactly how each file is compiled (flags, includes), so you get accurate jump-to-definition and error highlighting without running the full build.

---

## Ways to Load the Built Kernel Into the KVM VM

There are four distinct methods, each with different tradeoffs.

---

### Method 1: SCP + GRUB (Standard, Most Reliable)

This is what your `kdeploy` script does. It's the closest to how production kernel upgrades work.

```bash
KVER=$(make -s kernelversion)   # e.g. 7.0.6
VM_USER=dev
VM_IP=192.168.122.x

# 1. Copy the kernel image
scp arch/x86/boot/bzImage ${VM_USER}@${VM_IP}:/tmp/bzImage-${KVER}

# 2. Install modules (modules_install writes to /lib/modules/<ver>/)
STAGING=$(mktemp -d)
make INSTALL_MOD_PATH=$STAGING modules_install
tar -czf /tmp/kmod-${KVER}.tar.gz -C $STAGING .
scp /tmp/kmod-${KVER}.tar.gz ${VM_USER}@${VM_IP}:/tmp/
rm -rf $STAGING

# 3. On the VM:
ssh ${VM_USER}@${VM_IP} "bash -s" << REMOTE
  sudo cp /tmp/bzImage-${KVER} /boot/vmlinuz-${KVER}
  sudo tar -xzf /tmp/kmod-${KVER}.tar.gz -C /
  sudo depmod ${KVER}
  sudo update-initramfs -c -k ${KVER}
  sudo update-grub
  sudo grub-reboot "$(grep -i 'menuentry.*${KVER}' /boot/grub/grub.cfg | head -1 | sed "s/menuentry '\\([^']*\\)'.*/\\1/")"
  sudo reboot
REMOTE

sleep 10
virsh console kernel-dev
```

**When to use:** Reliable daily driver. The VM boots with a proper initramfs, all modules are in place, systemd is happy. Crash recovery is safe because GRUB only uses the new kernel once (`grub-reboot`) — a crash falls back to the previous kernel automatically.

---

### Method 2: QEMU Direct Kernel Boot (Fastest Iteration, No GRUB)

QEMU can boot a kernel directly without it being installed inside the VM at all. You pass `bzImage` straight to QEMU from the host.

First, get your VM's current domain XML:

```bash
virsh dumpxml kernel-dev > /tmp/kernel-dev.xml
```

Then boot with a direct kernel override:

```bash
virsh start kernel-dev \
    --pass-fds 0 \
    -- \
    -kernel ~/kernel-dev/linux-7.0.6/arch/x86/boot/bzImage \
    -append "root=/dev/vda1 console=tty0 console=ttyS0,115200n8 nokaslr"
```

Or with `qemu-kvm` directly (if you started the VM manually):

```bash
qemu-kvm \
    -m 4096 -smp 4 \
    -cpu host \
    -drive file=~/kernel-dev/ubuntu-server-dev.qcow2,if=virtio \
    -kernel arch/x86/boot/bzImage \
    -append "root=/dev/vda1 rw console=tty0 console=ttyS0,115200n8 nokaslr" \
    -nographic \
    -serial mon:stdio \
    -net nic,model=virtio -net user
```

For modules, you still need to either install them into the qcow2 disk or use a 9P virtio share:

```bash
# Mount host directory inside VM via 9P (no SCP needed for modules)
# Add to qemu command:
    -virtfs local,path=$STAGING,mount_tag=kmod_share,security_model=passthrough

# Inside VM after boot:
sudo mount -t 9p -o trans=virtio kmod_share /mnt/kmod
sudo cp -r /mnt/kmod/lib/modules/${KVER} /lib/modules/
sudo depmod ${KVER}
```

**When to use:** When you're iterating rapidly on early boot code, driver init, or anything that causes a panic before SSH is available. No need to touch GRUB or initramfs. The disk image is never written to — the VM's on-disk kernel is untouched.

---

### Method 3: kexec (Zero-Reboot Kernel Switch, Fastest Boot)

`kexec` loads a new kernel into memory and jumps to it directly, skipping BIOS/UEFI and GRUB entirely. Boot time drops from ~15 seconds to ~2 seconds.

```bash
# On the VM — install kexec-tools once
sudo apt install kexec-tools

# On HOST — copy bzImage and modules as usual (scp or method 1 steps 1-2)
# Then on VM:
KVER=7.0.6

sudo kexec \
    --load /boot/vmlinuz-${KVER} \
    --initrd /boot/initrd.img-${KVER} \
    --append "root=/dev/vda1 rw console=tty0 console=ttyS0,115200n8 nokaslr"

# Execute the switch immediately:
sudo kexec -e
```

You can also do it in one shot with systemd:

```bash
sudo systemctl kexec
# systemd detects a kexec-loaded kernel and switches to it cleanly
```

**When to use:** When you're iterating on net/ or bpf/ code that doesn't affect early boot — the common case. The 2-second boot vs 15-second boot adds up across dozens of test cycles per day. The downside: no GRUB fallback. If the new kernel panics immediately, you need `virsh reset kernel-dev` from the host.

---

### Method 4: Live Module Reload (No Reboot At All)

If your change is in a part of the kernel that's built as a module (`=m`), you can reload just that module without rebooting at all.

```bash
# On HOST — build only the changed module
make -j$(nproc) M=net/ipv4/

# Find the resulting .ko file
find net/ipv4/ -name "*.ko" -newer arch/x86/boot/bzImage

# Copy just the module to VM
scp net/ipv4/foo.ko ${VM_USER}@${VM_IP}:/tmp/

# On VM — unload old, load new
sudo rmmod foo
sudo insmod /tmp/foo.ko

# Or use modprobe if the module has dependencies:
sudo cp /tmp/foo.ko /lib/modules/$(uname -r)/kernel/net/ipv4/
sudo depmod
sudo modprobe -r foo
sudo modprobe foo

# Check it loaded correctly
lsmod | grep foo
dmesg | tail -20
```

For VXLAN, GENEVE, GRE etc. — these are almost always modules. When you're tuning encapsulation behavior, this is the fastest path: edit → `make M=net/` → scp one `.ko` → `rmmod`/`insmod`. No reboot at all.

**When to use:** Net tunnel drivers, BPF helper additions, anything where the changed code lives in a module. Check with `grep CONFIG_VXLAN .config` — if it says `=m`, you can use this method.

---

## Decision Map

```
What did you change?
│
├── Core net stack (tcp.c, sock.c, skbuff.c) — built-in, not a module
│   └── kexec (Method 3) for speed, or SCP+GRUB (Method 1) for safety
│
├── A tunnel driver (vxlan, geneve, gre) — likely =m
│   └── Live module reload (Method 4) — no reboot at all
│
├── BPF verifier / helpers — built-in
│   └── kexec (Method 3)
│
├── Early boot / init / panic handler
│   └── QEMU direct boot (Method 2) — serial from first byte
│
└── Anything that caused a hang/panic last boot
    └── SCP+GRUB with grub-reboot (Method 1) — safe fallback guaranteed
```

---

## Testing Checklist After Each Boot

```bash
# 1. Confirm you're on your kernel
uname -r

# 2. Check for any boot errors
dmesg | grep -E "(BUG|WARNING|ERROR|panic)" | head -20

# 3. BPF/XDP stack
bpftool btf show | head -3        # BTF present?
cat /proc/sys/net/core/bpf_jit_enable  # JIT on?
ls /proc/net/xdp                  # AF_XDP available?

# 4. Your modules loaded
lsmod | grep -E "vxlan|geneve|veth"

# 5. Net namespaces work
sudo ip netns add test-ns
sudo ip netns exec test-ns ip link show
sudo ip netns del test-ns

# 6. XDP attach on test NIC
sudo ip link set dev ens4 xdp obj xdp_pass.o sec xdp
sudo bpftool prog show
sudo ip link set dev ens4 xdp off

# 7. Drop monitor (if you enabled it)
sudo modprobe drop_monitor
sudo dropwatch -l kas
```

Yes, that command works correctly, but it has a subtle behavior worth understanding — and a couple of things to watch out for.

## What That Command Actually Does

```bash
scp dev@192.168.122.x:/boot/config-$(ssh dev@192.168.122.x uname -r) \
    ~/kernel-dev/linux-7.0.6/.config
```

The `$(ssh dev@192.168.122.x uname -r)` is evaluated **on the host** first — it opens a separate SSH connection to the VM, runs `uname -r`, substitutes the result (e.g. `6.8.0-57-generic`) into the path, then `scp` opens a second connection to copy the file. So it makes **two SSH connections**, which is harmless but slightly inefficient. A cleaner equivalent:

```bash
# Cleaner: get the version once, reuse it
VM_KVER=$(ssh dev@192.168.122.x uname -r)
scp dev@192.168.122.x:/boot/config-${VM_KVER} ~/kernel-dev/linux-7.0.6/.config
```

## The Real Issues to Watch For

**1. Ubuntu signing keys will break your build**

Ubuntu's kernel config has certificate paths baked in that don't exist on your host. Without disabling them, the build will fail. Always run these right after the scp:

```bash
cd ~/kernel-dev/linux-7.0.6

# Resolve new options from 6.8 → 7.0.6
make olddefconfig

# Disable Ubuntu-specific signing config
scripts/config --disable SYSTEM_TRUSTED_KEYS
scripts/config --disable SYSTEM_REVOCATION_KEYS
scripts/config --disable MODULE_SIG_KEY
scripts/config --set-str SYSTEM_TRUSTED_KEYS ""
scripts/config --set-str SYSTEM_REVOCATION_KEYS ""

make olddefconfig   # run again after changes
```

**2. You're using a 6.8 config to build 7.0.6**

`make olddefconfig` handles this — it keeps all your existing options and sets new 7.0.6-specific options to their safe defaults. But verify nothing critical was silently set wrong:

```bash
# Quick sanity check after olddefconfig
for opt in BPF_SYSCALL XDP_SOCKETS VIRTIO_NET DEBUG_INFO_BTF SERIAL_8250_CONSOLE NET_NS; do
    printf "%-35s %s\n" "CONFIG_${opt}" "$(grep CONFIG_${opt}= .config | head -1)"
done
```

**3. The config is correct for this VM specifically**

This is actually the *right* approach — pulling from the VM means virtio drivers, serial console, and systemd requirements are already satisfied. Using `make x86_64_defconfig` would miss VM-specific things and likely fail to boot.

## Summary

| Step | Status |
|---|---|
| Command works | Yes |
| Two SSH connections | Harmless, just be aware |
| `make olddefconfig` after scp | Mandatory |
| Disable signing keys | Mandatory |
| Good base for this VM | Yes — better than defconfig |

This is actually great news for your setup. Here's what it means:

## Ubuntu 24.04 LTS vs 26.04 LTS — What Changes

| | Ubuntu 24.04 | Ubuntu 26.04 |
|---|---|---|
| Ships with kernel | 6.8.x | **7.0** |
| Config gap to build 7.0.6 | Large (6.8 → 7.0.6) | **Minimal (7.0 → 7.0.6)** |
| `make olddefconfig` work | Fills many new options with defaults | Almost nothing new to fill |
| Risk of missing options | Higher | Lower |

---

## If You're on Ubuntu 26.04 (recommended)

Ubuntu 26.04 ships with Linux Kernel 7.0 by default, which means your VM's `/boot/config-*` is already a 7.0 config. When you scp it and run `make olddefconfig` against kernel 7.0.6, the config gap is tiny — just patch-level changes between 7.0 and 7.0.6. Almost nothing gets filled with unknown defaults.

```bash
# On VM — confirm your kernel version
uname -r
# 7.0.x-xx-generic   ← if 26.04

# The scp command works perfectly and the config is nearly identical to 7.0.6
VM_KVER=$(ssh dev@192.168.122.x uname -r)
scp dev@192.168.122.x:/boot/config-${VM_KVER} ~/kernel-dev/linux-7.0.6/.config

make olddefconfig   # almost no new options to fill — clean
```

Also worth noting: Ubuntu 26.04 ships with sched_ext support — the eBPF-based scheduling system — and crash dumps (kdump) enabled by default, so those will already be in the config you pull.

---

## If You're on Ubuntu 24.04 LTS

The config ships with kernel 6.8, so the gap to 7.0.6 is larger. `make olddefconfig` handles it, but you should double-check critical options manually after:

```bash
# These are the areas where 6.8 → 7.0 added new Kconfig options
# Verify they got set how you want after olddefconfig:
for opt in BPF_SYSCALL XDP_SOCKETS DEBUG_INFO_BTF VIRTIO_NET \
           SERIAL_8250_CONSOLE NET_NS SCHED_CLASS_EXT; do
    printf "%-35s %s\n" "CONFIG_${opt}" "$(grep CONFIG_${opt}= .config | head -1)"
done
```

`CONFIG_SCHED_CLASS_EXT` (sched_ext) is a notable 7.0 addition — it won't exist in the 24.04 config and `olddefconfig` will set it to a default you should verify.

---

## Recommendation

**Use 26.04** if you haven't installed the VM yet. The kernel version match alone eliminates an entire class of config problems, and you get a better eBPF/XDP environment out of the box.