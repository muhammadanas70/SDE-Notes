# io_uring: A Complete Architectural and Implementation Guide

> A production-grade reference on Linux's `io_uring` asynchronous I/O interface — kernel internals, memory layout, syscall ABI, C and Rust implementations, and the expert mental model needed to reason about it correctly.

---

## Table of Contents

1. Motivation: Why io_uring Exists
2. Historical Timeline
3. Core Architecture Overview
4. The Two Rings: SQ and CQ
5. Memory Layout and mmap Regions
6. Kernel Data Structures
7. The `io_uring_setup` Syscall
8. Submission Queue Entry (SQE) Deep Dive
9. Completion Queue Entry (CQE) Deep Dive
10. The `io_uring_enter` Syscall and Submission/Completion Flow
11. Polling Modes: IOPOLL and SQPOLL
12. Registered Resources: Fixed Files and Fixed Buffers
13. Provided Buffers and Buffer Rings (bufring)
14. Linked SQEs, Chains, and Drain Semantics
15. Multishot Operations
16. Opcode Catalog
17. liburing: The Userspace Library
18. C Implementation: Real-World Examples
19. Rust Implementation: Raw Syscalls
20. Rust Implementation: The `io-uring` Crate
21. Rust Implementation: `tokio-uring`
22. Security Model and Notable CVEs
23. Performance Characteristics and Benchmarking
24. Comparison: io_uring vs epoll vs AIO vs io_submit
25. Debugging, Tracing, and Observability
26. Expert Mental Model

---

## 1. Motivation: Why io_uring Exists

Before io_uring, Linux offered three broad asynchronous/efficient I/O strategies, and each had a structural flaw that io_uring was purpose-built to eliminate.

**`epoll`** solves the "which fds are ready" problem for readiness-based I/O (sockets, pipes), but it is fundamentally a *notification* mechanism, not a completion mechanism. You still issue a blocking (or would-block) `read()`/`write()` syscall per ready fd. Every I/O operation costs at least one syscall, and syscalls are expensive: each entry crosses the user/kernel boundary, invalidates speculative execution state, and — post-Spectre/Meltdown — pays the price of kernel page table isolation (KPTI) switches. At high I/O rates (hundreds of thousands to millions of ops/sec), syscall overhead dominates.

**POSIX AIO (`aio_read`/`aio_write`, glibc)** is implemented almost entirely in userspace using a thread pool that performs blocking syscalls behind your back. It's "asynchronous" in name only — you get thread-pool concurrency, not true kernel-level async I/O. It does not support arbitrary operations well, has inconsistent behavior across filesystems, and its glibc implementation is widely regarded as unreliable in production.

**Linux native AIO (`io_setup`/`io_submit`/`io_getevents`, the `libaio` interface)** is a genuine kernel-level async interface, but with severe limitations:
- Only supports `O_DIRECT` file I/O reliably; buffered I/O frequently falls back to blocking behavior inside the syscall.
- No support for sockets in any meaningful, general way.
- `io_submit()` is still a syscall per batch, and in practice many workloads end up issuing one syscall per operation because batching is awkward.
- The completion side (`io_getevents`) requires another syscall to reap events.
- The ABI is notoriously difficult to extend; it has essentially been frozen for years.

io_uring, introduced by Jens Axboe in **Linux 5.1 (May 2019)**, was designed from scratch to solve the actual problem: **eliminate syscalls from the hot path entirely**, support **any** kind of I/O (file, block, network, and eventually much more — `openat`, `accept`, `connect`, `splice`, `poll`, timeouts, cancellation, even non-I/O syscalls like `close` and `statx`), and do it with a **stable, extensible binary ABI**.

The key architectural insight: instead of one syscall per operation, use two **lock-free ring buffers shared between userspace and the kernel via `mmap`**. Userspace writes submission descriptors into a ring; the kernel writes completions into another ring. Under the right configuration (`SQPOLL`), **zero syscalls** are needed for steady-state operation — a kernel thread polls the submission ring itself.


---

## 2. Historical Timeline

| Kernel Version | Date | Milestone |
|---|---|---|
| 5.1 | May 2019 | Initial merge: `io_uring_setup`, `io_uring_enter`, basic read/write, fixed files/buffers |
| 5.2 | Jul 2019 | `IORING_OP_POLL_ADD`, `IORING_OP_POLL_REMOVE` |
| 5.3 | Sep 2019 | `SQPOLL` improvements, `IORING_OP_SYNC_FILE_RANGE` |
| 5.4 | Nov 2019 | Linked SQEs (`IOSQE_IO_LINK`), timeouts (`IORING_OP_TIMEOUT`) |
| 5.5 | Jan 2020 | `IORING_OP_ACCEPT`, `IORING_OP_CONNECT`, `IORING_OP_ASYNC_CANCEL`, `IORING_OP_RECVMSG`/`SENDMSG` |
| 5.6 | Mar 2020 | `IORING_OP_OPENAT2`, `IORING_OP_EPOLL_CTL`, buffer selection groundwork |
| 5.7 | May 2020 | `IORING_OP_STATX`, splice/tee ops, io_uring worker pools (`io-wq`) replacing plain kthreads |
| 5.8 | Aug 2020 | Restricted `io_uring` via `IORING_REGISTER_RESTRICTIONS`, `IORING_OP_PROVIDE_BUFFERS` |
| 5.9 | Oct 2020 | `IORING_OP_SHUTDOWN`, better cancellation |
| 5.11 | Feb 2021 | Zero-copy `IORING_OP_SENDMSG_ZC` groundwork, `IORING_REGISTER_RING_FDS` |
| 5.13 | Jun 2021 | `IORING_OP_SOCKET`, ring-mapped SQ, better multishot support |
| 5.15 | Nov 2021 | Multishot `IORING_OP_ACCEPT`, multishot `IORING_OP_POLL_ADD` |
| 5.19 | Aug 2022 | Multishot `IORING_OP_RECV`/`RECVMSG`, ring-provided buffers (`IOU_PBUF_RING`) |
| 6.0 | Oct 2022 | `IORING_OP_URING_CMD` for device-specific passthrough (NVMe) |
| 6.1 | Dec 2022 | io_uring/NVMe passthrough hardened |
| 6.4 | Jun 2023 | Zero-copy `IORING_OP_SEND_ZC`, register clone for napi |
| 6.7+ | 2024 | `IORING_OP_WAITID`, futex ops, continued zero-copy networking work |

The design has consistently followed one rule since day one: **the ABI never breaks**. New opcodes and features are additive, gated by `IORING_FEAT_*` and `IORING_SETUP_*` flags detected via `io_uring_setup`'s returned `io_uring_params` and `IORING_REGISTER_PROBE`.

---

## 3. Core Architecture Overview

At the highest level, io_uring is **two circular buffers plus an array of fixed-size command slots**, all living in memory that is `mmap`'d into both the process and accessible to the kernel:

- **Submission Queue (SQ)**: a ring of *indices* into an SQE array. Userspace writes.
- **SQE array**: the actual submission entries (64 bytes each, or 128 in "big SQE" mode). Userspace writes.
- **Completion Queue (CQ)**: a ring of CQEs (completion entries) written directly by the kernel. Userspace reads.

```
                         USER SPACE                    KERNEL SPACE
                    ┌───────────────────┐          ┌──────────────────────┐
                    │   Application      │          │   io_uring subsystem  │
                    │                    │          │                       │
   write SQE  ─────►│  SQE[] array       │◄────────►│  fetches SQE by index │
   push index ─────►│  SQ ring (indices) │          │  from SQ ring         │
                    │                    │          │                       │
                    │  CQ ring (CQEs)    │◄─────────│  writes CQE on        │
   read CQE  ◄───────│                    │          │  completion           │
                    └───────────────────┘          └──────────────────────┘
                              ▲                                │
                              │        io_uring_enter(2)        │
                              └────────────────────────────────┘
                         (optional in SQPOLL mode — kernel thread
                          polls the SQ ring without syscalls)
```

Both rings are **single-producer/single-consumer lock-free ring buffers** using monotonically increasing 32-bit head/tail indices (wrapping is handled via masking with `ring_size - 1`, since ring sizes are always powers of two). This is the same fundamental design as a classic SPSC ring buffer used in high-performance networking (DPDK, lock-free queues) — no kernel lock is needed for the steady-state push/pop path, only memory barriers.

- **SQ**: userspace is the producer (writes new submissions), kernel is the consumer (reads and executes them).
- **CQ**: kernel is the producer (writes completions), userspace is the consumer (reads results).

This is the single most important mental model to internalize: **io_uring turns I/O into a message-passing problem between two lock-free SPSC queues**, rather than a syscall-per-operation model.


---

## 4. The Two Rings: SQ and CQ

### 4.1 Submission Queue Ring

The SQ ring itself does **not** contain SQEs. It contains `uint32_t` **indices** into a separate SQE array. This indirection exists so the kernel can support features like SQE reuse and so userspace can prepare SQEs out of order and then commit them in any order by writing indices into the ring.

Flow for submitting one operation:

1. Application computes `tail = *sq.ktail` (local cached copy, not re-read from shared memory each time for hot loops).
2. Application picks a free slot in the SQE array: `index = tail & sq_mask`.
3. Application fills `sqes[index]` with operation parameters (opcode, fd, buffer, offset, flags, user_data...).
4. Application writes `index` into `sq.array[tail & sq_mask]`.
5. Application increments its local `tail` and stores it with a **release memory-barrier write** into `*sq.ktail` (`io_uring_smp_store_release`), making it visible to the kernel.
6. Application calls `io_uring_enter()` (unless SQPOLL is active and the kernel thread is already polling), which tells the kernel "new entries are available, go consume them" and/or "wait for N completions."

### 4.2 Completion Queue Ring

The CQ ring **does** contain the actual completion entries inline (not indices) — CQEs are small (16 bytes in the default mode) so there's no indirection benefit.

Flow for reaping one completion:

1. Application reads `head = *cq.khead` and `tail = io_uring_smp_load_acquire(cq.ktail)` (acquire barrier — must happen *after* checking tail to see fully-written CQE data).
2. While `head != tail`: read `cqes[head & cq_mask]`, process it (dispatch based on `cqe->user_data`), increment local `head`.
3. Write updated `head` back to `*cq.khead` with a release barrier, telling the kernel these slots are now free for reuse.

### 4.3 Why Memory Barriers Matter Here

Because there is no kernel lock guarding these rings in the hot path, correctness depends entirely on x86/ARM memory ordering rules being respected explicitly:

- **Producer** (writer of head/tail) must use a **store-release** so all writes to the ring slot become visible to the consumer *before* the index update is visible.
- **Consumer** (reader of head/tail) must use a **load-acquire** so it doesn't observe the updated index before the corresponding slot data.

This is exactly the same barrier pattern used in classic SPSC ring buffers in lock-free systems programming — if you've built one in Rust with `AtomicUsize` and `Ordering::Release`/`Ordering::Acquire`, you already have the correct mental model. `liburing`'s `io_uring_smp_store_release()`/`io_uring_smp_load_acquire()` macros compile down to exactly `atomic_store_explicit(..., memory_order_release)` / `atomic_load_explicit(..., memory_order_acquire)` semantics (implemented with explicit compiler+CPU barriers for portability across architectures with weaker memory models, e.g. ARM).

```
   ASCII: SPSC ring buffer index arithmetic

   ring_size = 8  (must be power of two)
   mask      = ring_size - 1 = 0b111

   tail=13 (0b1101)  → slot = 13 & 7 = 5
   head=9  (0b1001)  → slot = 9  & 7 = 1

        0   1   2   3   4   5   6   7
      ┌───┬───┬───┬───┬───┬───┬───┬───┐
      │   │ H │   │   │   │ T │   │   │
      └───┴───┴───┴───┴───┴───┴───┴───┘
            ▲                   ▲
          head&mask          tail&mask

   Occupied slots = tail - head = 4 (entries 9,10,11,12 consumed
   already reused space; entries between head and tail are "in flight")
```

Note that `head` and `tail` are **not** masked in storage — they are free-running 32-bit counters that monotonically increase (and wrap via integer overflow, which is fine because the difference `tail - head` is computed mod 2^32 and ring sizes are always ≤ 2^31). Masking only happens when computing the *array index*. This is the standard trick that lets an SPSC ring buffer distinguish "empty" from "full" without wasting a slot or needing a separate counter.


---

## 5. Memory Layout and mmap Regions

`io_uring_setup()` returns a file descriptor. Userspace then calls `mmap()` on that fd **three times** (or two, in newer "single mmap" mode) with specific offsets defined by constants in `<linux/io_uring.h>`:

```c
#define IORING_OFF_SQ_RING   0x00000000ULL   /* SQ ring header (head/tail/flags/array) */
#define IORING_OFF_CQ_RING   0x08000000ULL   /* CQ ring header + CQE array */
#define IORING_OFF_SQES      0x10000000ULL   /* the actual io_uring_sqe[] array */
```

```
                     Process Virtual Address Space
   ┌─────────────────────────────────────────────────────────────────┐
   │                                                                   │
   │   mmap(fd, IORING_OFF_SQ_RING)                                   │
   │   ┌───────────────────────────────────────────┐                 │
   │   │ struct io_sqring_offsets region             │                 │
   │   │  ┌────────┬────────┬────────┬────────┐     │                 │
   │   │  │ khead  │ ktail  │ kflags │ kdropped│     │                 │
   │   │  ├────────┴────────┴────────┴────────┤     │                 │
   │   │  │  array[] -- indices into SQE array │     │                 │
   │   │  └─────────────────────────────────────┘     │                 │
   │   └───────────────────────────────────────────┘                 │
   │                                                                   │
   │   mmap(fd, IORING_OFF_SQES)                                      │
   │   ┌───────────────────────────────────────────┐                 │
   │   │ struct io_uring_sqe sqes[sq_entries]        │                 │
   │   │   [sqe 0][sqe 1][sqe 2] ... [sqe N-1]        │                 │
   │   │   (64 bytes each, or 128B if SETUP_SQE128)   │                 │
   │   └───────────────────────────────────────────┘                 │
   │                                                                   │
   │   mmap(fd, IORING_OFF_CQ_RING)                                   │
   │   ┌───────────────────────────────────────────┐                 │
   │   │ struct io_cqring_offsets region             │                 │
   │   │  ┌────────┬────────┬────────┐              │                 │
   │   │  │ khead  │ ktail  │ kflags │              │                 │
   │   │  ├────────┴────────┴────────┤              │                 │
   │   │  │ cqes[] -- actual CQEs     │              │                 │
   │   │  │  (16B each, 32B if CQE32) │              │                 │
   │   │  └───────────────────────────┘              │                 │
   │   └───────────────────────────────────────────┘                 │
   │                                                                   │
   └─────────────────────────────────────────────────────────────────┘
```

Since Linux 5.4, if `IORING_FEAT_SINGLE_MMAP` is reported, the SQ ring header and CQ ring can be mapped with a **single `mmap()` call** because the kernel allocates them contiguously — this reduces the number of mmap syscalls from 3 to 2, which matters because `mmap` itself is not cheap (page table setup, TLB shootdown consideration) and this only happens once at ring init, but it's part of io_uring's general philosophy of minimizing syscall count everywhere, not just per-I/O.

All three regions are backed by kernel memory allocated with `remap_pfn_range`-style mapping (or `vm_insert_page` for the SQE array), pinned (non-swappable) so the kernel can safely dereference these pages without faulting — this is critical because SQPOLL kernel threads and IRQ-context completion producers cannot tolerate page faults.

### 5.1 `io_uring_setup` parameters that affect layout

```c
struct io_uring_params {
    __u32 sq_entries;   /* filled by kernel: actual SQ ring size (rounded to pow2) */
    __u32 cq_entries;   /* filled by kernel: actual CQ ring size */
    __u32 flags;        /* input: IORING_SETUP_* flags */
    __u32 sq_thread_cpu;    /* SQPOLL: which CPU to pin the poll thread to */
    __u32 sq_thread_idle;   /* SQPOLL: ms of idle before thread sleeps */
    __u32 features;     /* output: IORING_FEAT_* supported by this kernel */
    __u32 wq_fd;         /* input: share io-wq worker pool with another ring */
    __u32 resv[3];
    struct io_sqring_offsets sq_off;   /* output: byte offsets within SQ mmap */
    struct io_cqring_offsets cq_off;   /* output: byte offsets within CQ mmap */
};
```

Userspace never hardcodes offsets — it always reads `sq_off`/`cq_off` back from the kernel after `io_uring_setup()` and computes pointers as `mmap_base + offset`. This is what makes the ABI forward-compatible: the kernel can rearrange internal layout across versions and old binaries keep working because they trust the returned offsets rather than assuming a fixed struct layout.


---

## 6. Kernel Data Structures

While userspace only sees the mmap'd rings, it's essential to understand what's happening on the kernel side (`fs/io_uring.c` historically, split into `io_uring/*.c` since 5.19's major refactor) to reason about performance and failure modes correctly.

### 6.1 `io_ring_ctx` — the per-ring context

This is the central kernel object, one per `io_uring_setup()` call, roughly:

```c
struct io_ring_ctx {
    struct {
        struct percpu_ref   refs;
        unsigned int         flags;
        unsigned int         drain_next: 1;
        struct io_rings      *rings;        /* the mmap'd SQ/CQ header region */
        struct task_struct    *submitter_task;
        ...
    } ____cacheline_aligned_in_smp;

    /* submission side, only touched by the submitting task or sqpoll thread */
    struct {
        struct io_uring_sqe *sq_sqes;
        ...
        struct io_sq_data     *sq_data;   /* SQPOLL thread state */
    } ____cacheline_aligned_in_smp;

    /* completion side */
    struct {
        struct wait_queue_head cq_wait;
        ...
    } ____cacheline_aligned_in_smp;

    struct io_wq_work_list   defer_list;   /* linked ops waiting on drain/deps */
    struct io_alloc_cache     apoll_cache; /* async poll allocation cache */
    struct xarray             io_bl_xa;    /* buffer lists (provided buffers) */
    struct io_rsrc_data       *file_data;  /* registered/fixed files */
    struct io_rsrc_data       *buf_data;   /* registered/fixed buffers */
    ...
};
```

The struct is deliberately organized with `____cacheline_aligned_in_smp` sections separating submission-path-only fields, completion-path-only fields, and shared fields — this is a **false-sharing avoidance pattern**: the CPU running the submitting thread and the CPU running IRQ-context completion (e.g., NVMe interrupt handler marking an I/O done) should never contend on the same cache line. This is the same discipline you'd apply designing a high-throughput Rust `struct` with `#[repr(align(64))]` fields to avoid cache-line ping-pong between producer and consumer cores.

### 6.2 `io_kiocb` — one in-flight operation

Every submitted SQE becomes an `io_kiocb` inside the kernel — the "request" object tracked through its entire lifecycle:

```c
struct io_kiocb {
    union {
        struct file *file;
        ...
    };
    u8 opcode;
    u8 iopoll_completed;
    u16 flags;            /* REQ_F_* runtime state flags */

    struct io_ring_ctx *ctx;
    struct task_struct  *task;

    u64 user_data;        /* opaque value from the SQE, echoed in the CQE */

    /* opcode-specific data lives in a union of small structs, e.g. */
    union {
        struct io_read     read;
        struct io_write    write;
        struct io_accept   accept;
        struct io_connect  connect;
        struct io_poll     poll;
        struct io_timeout  timeout;
        ...
    };

    struct io_kiocb *link;    /* next request in a link chain, or NULL */
    ...
};
```

`io_kiocb` objects are allocated from a **per-context slab/allocation cache** (`io_alloc_cache`) to avoid `kmalloc`/`kfree` churn on the hot path — this is analogous to an object pool pattern you'd hand-roll in Rust with a `Vec<Option<T>>` free-list to avoid allocator pressure in a hot loop.

### 6.3 `io-wq` — the async worker pool

Not every operation can complete inline in the `io_uring_enter()` syscall context. Some (e.g., buffered reads that would block on disk I/O without `O_DIRECT`, or blocking-prone operations) get punted to a **worker thread pool called `io-wq`**. This is conceptually similar to a Rust async runtime's `spawn_blocking` — an escape hatch from the non-blocking fast path onto a thread pool for genuinely blocking work, while keeping the fast path syscall-free for I/O that supports true async completion (sockets, `O_DIRECT` files, NVMe passthrough, etc.).

```
   io_uring_enter()
         │
         ▼
   ┌─────────────┐   can complete without blocking?
   │  dispatch    │────────────Yes───────────► do it inline, write CQE
   │  by opcode   │
   └─────────────┘
         │ No (would block)
         ▼
   ┌─────────────┐
   │  io-wq pool   │  worker thread performs blocking syscall,
   │  (bounded,    │  then posts CQE via task_work / IRQ-safe path
   │  per-NUMA)    │
   └─────────────┘
```


---

## 7. The `io_uring_setup` Syscall

```c
int io_uring_setup(u32 entries, struct io_uring_params *p);
```

- `entries`: requested SQ ring size (rounded up to the next power of two, capped by `/proc/sys/kernel/io_uring_disabled` policy and `IORING_MAX_ENTRIES`, historically 32768, extendable to 2^15 unless `IORING_SETUP_CLAMP` is set to silently clamp rather than error).
- `p`: in/out parameter struct (see §5.1). `p->flags` on input selects setup mode.

Key `IORING_SETUP_*` flags:

| Flag | Effect |
|---|---|
| `IORING_SETUP_IOPOLL` | Busy-poll for completions instead of interrupt-driven; requires `O_DIRECT` block device files |
| `IORING_SETUP_SQPOLL` | Kernel spawns a dedicated polling thread that consumes the SQ ring without `io_uring_enter()` calls |
| `IORING_SETUP_SQ_AFF` + `sq_thread_cpu` | Pin the SQPOLL thread to a specific CPU |
| `IORING_SETUP_CQSIZE` | Explicitly size the CQ ring independent of SQ ring (default CQ = 2× SQ) |
| `IORING_SETUP_CLAMP` | Clamp entries to max instead of failing |
| `IORING_SETUP_ATTACH_WQ` | Share the `io-wq` worker pool with another ring (reduces thread count when an app owns many rings) |
| `IORING_SETUP_R_DISABLED` | Ring starts disabled; must call `IORING_REGISTER_ENABLE_RINGS` — lets you register resources before any submission is possible |
| `IORING_SETUP_SUBMIT_ALL` | On error mid-batch, continue submitting rest instead of stopping |
| `IORING_SETUP_COOP_TASKRUN` | Don't send an inter-processor interrupt to wake the task for every completion; batch notification (big latency/throughput win under high completion rates) |
| `IORING_SETUP_SINGLE_ISSUER` | Assert only one thread will ever submit — enables internal lock elision |
| `IORING_SETUP_DEFER_TASKRUN` | Defer task_work running until the app explicitly waits — pairs with `SINGLE_ISSUER` for the lowest-overhead single-threaded event loop mode |

The returned `p.features` bitmask tells userspace what the running kernel actually supports (`IORING_FEAT_SINGLE_MMAP`, `IORING_FEAT_NODROP`, `IORING_FEAT_SUBMIT_STABLE`, `IORING_FEAT_FAST_POLL`, `IORING_FEAT_EXT_ARG`, `IORING_FEAT_NATIVE_WORKERS`, `IORING_FEAT_RSRC_TAGS`, `IORING_FEAT_CQE_SKIP`, `IORING_FEAT_LINKED_FILE`, `IORING_FEAT_REG_REG_RING`...) — this is how liburing and applications do runtime feature detection instead of compile-time `#ifdef` on kernel version, since io_uring capability depends on the *running* kernel, not the build environment.


---

## 8. Submission Queue Entry (SQE) Deep Dive

The `io_uring_sqe` struct is the ABI-critical, most important struct in the entire interface. Its layout (64 bytes, `<linux/io_uring.h>`):

```c
struct io_uring_sqe {
    __u8    opcode;         /* IORING_OP_* */
    __u8    flags;          /* IOSQE_* per-request modifier flags */
    __u16   ioprio;         /* I/O priority, like ioprio_set(2) */
    __s32   fd;              /* file descriptor, or index if IOSQE_FIXED_FILE */
    union {
        __u64 off;           /* file offset */
        __u64 addr2;
        struct { __u32 cmd_op; __u32 __pad1; };
    };
    union {
        __u64 addr;           /* buffer pointer, or msghdr*, or ... (opcode-dependent) */
        __u64 splice_off_in;
    };
    __u32   len;              /* buffer length, opcode-dependent */
    union {
        __kernel_rwf_t rw_flags;
        __u32           fsync_flags;
        __u16           poll_events;
        __u32           poll32_events;
        __u32           sync_range_flags;
        __u32           msg_flags;
        __u32           timeout_flags;
        __u32           accept_flags;
        __u32           cancel_flags;
        __u32           open_flags;
        __u32           statx_flags;
        __u32           fadvise_advice;
        __u32           splice_flags;
        __u32           rename_flags;
        __u32           unlink_flags;
        ...                              /* opcode determines interpretation */
    };
    __u64   user_data;         /* opaque, echoed verbatim into the matching CQE */
    union {
        __u16 buf_index;        /* IOSQE_BUFFER_SELECT: which buffer group */
        __u16 buf_group;
    };
    __u16   personality;        /* credential set to use, via IORING_REGISTER_PERSONALITY */
    union {
        __s32 splice_fd_in;
        __u32 file_index;        /* IOSQE_FIXED_FILE / auto-alloc fixed slot */
        __u32 optlen;             /* IORING_OP_URING_CMD, socket opts */
    };
    union {
        __u64 addr3;
        struct { __u64 attr_ptr; __u64 attr_type_mask; };
        __u64 optval;
    };
    __u64   __pad2[1];
};
```

Every field's meaning is **opcode-dependent** via the unions — this is the trickiest part of the ABI to internalize. There is no single canonical "meaning" of `off`/`addr`/`len`; you must always cross-reference against the specific `IORING_OP_*` you're issuing.

### 8.1 `IOSQE_*` flags (per-SQE behavior modifiers)

| Flag | Meaning |
|---|---|
| `IOSQE_FIXED_FILE` | `fd` is interpreted as an index into the pre-registered fixed-file table, not a raw fd |
| `IOSQE_IO_DRAIN` | Don't start this SQE until all prior SQEs have completed (a full serialization barrier) |
| `IOSQE_IO_LINK` | This SQE is linked to the next one — next won't start until this one completes (soft dependency; failure short-circuits the rest of the chain) |
| `IOSQE_IO_HARDLINK` | Like `IO_LINK` but continues the chain even if this SQE fails |
| `IOSQE_ASYNC` | Force async (io-wq) execution instead of attempting inline completion — useful to avoid a slow op blocking the submitter |
| `IOSQE_BUFFER_SELECT` | Kernel picks a buffer from a registered provided-buffer pool instead of using `addr`/`len` |
| `IOSQE_CQE_SKIP_SUCCESS` | Don't post a CQE if this op succeeds (useful mid-link-chain to reduce CQ pressure) |

### 8.2 Filling an SQE (conceptual, opcode-agnostic sequence)

```
struct io_uring_sqe *sqe = io_uring_get_sqe(&ring);   /* claim next free slot */
io_uring_prep_read(sqe, fd, buf, len, offset);          /* fills opcode+fields */
sqe->user_data = (uint64_t)(uintptr_t)my_request_ctx;    /* your own correlation id */
io_uring_sqe_set_flags(sqe, IOSQE_FIXED_FILE);            /* optional modifiers */
/* not yet submitted to kernel -- just written into the mmap'd array */
```

`io_uring_get_sqe()` returns `NULL` if the SQ ring is full (`tail - head == sq_entries`) — the application must either flush (`io_uring_submit()`) or drop/backoff. This is the single most common source of subtle bugs in naive io_uring code: not checking for a full ring under high submission rate.


---

## 9. Completion Queue Entry (CQE) Deep Dive

```c
struct io_uring_cqe {
    __u64  user_data;     /* echoed verbatim from the originating SQE */
    __s32  res;            /* result: >=0 success (opcode-defined meaning,
                               e.g. bytes transferred), <0 = -errno */
    __u32  flags;           /* IORING_CQE_F_* */
    /* only present if ring created with IORING_SETUP_CQE32: */
    __u64  big_cqe[];
};
```

`res` follows the **negative-errno convention** identical to raw Linux syscalls (`read()` returning `-EAGAIN` maps to `cqe->res == -EAGAIN`) — this is intentional, so existing errno-handling code translates almost directly.

### 9.1 `IORING_CQE_F_*` flags

| Flag | Meaning |
|---|---|
| `IORING_CQE_F_BUFFER` | The high 16 bits of `flags` encode which provided-buffer index was used (`IOSQE_BUFFER_SELECT` responses) |
| `IORING_CQE_F_MORE` | More CQEs will follow for this same request (multishot operations — the SQE stays "active") |
| `IORING_CQE_F_SOCK_NONEMPTY` | Hint: socket recv queue still has more data pending after this completion |
| `IORING_CQE_F_NOTIF` | This CQE is a zero-copy-send *notification* completion (buffer now safe to reuse), not the send completion itself |

### 9.2 CQ Overflow

If the CQ ring fills up (kernel produces completions faster than userspace consumes them) and `IORING_FEAT_NODROP` is supported (default on modern kernels), the kernel does **not** silently drop CQEs — it backs them up in an internal overflow list and flags `IORING_SQ_CQ_OVERFLOW` in the SQ ring's `flags` field, which forces the next `io_uring_enter()` call (even a pure-submit call) to first drain the overflow list into the ring before doing anything else. On very old kernels lacking `IORING_FEAT_NODROP`, CQEs *could* be lost under sustained overflow, which is why an application must always size the CQ ring generously (`IORING_SETUP_CQSIZE`, typically 2-4x the SQ ring) and drain promptly.


---

## 10. The `io_uring_enter` Syscall and Submission/Completion Flow

```c
int io_uring_enter(unsigned int fd, unsigned int to_submit,
                    unsigned int min_complete, unsigned int flags,
                    sigset_t *sig, size_t sigsz);
/* io_uring_enter2 adds: struct __kernel_timespec *ts variant via IORING_ENTER_EXT_ARG */
```

- `to_submit`: how many new SQEs (starting from the ring's current tail region) to process this call.
- `min_complete`: block until at least this many CQEs are available (or the call would return immediately if already satisfied) — only honored if `IORING_ENTER_GETEVENTS` is set in `flags`.
- `flags`: `IORING_ENTER_GETEVENTS` (wait for completions too, combining submit+wait in one syscall), `IORING_ENTER_SQ_WAKEUP` (SQPOLL: wake the polling thread if it went idle), `IORING_ENTER_EXT_ARG` (use the timeout/sigmask extended arg struct).

### 10.1 Full round-trip, single syscall

The headline feature: **one syscall can both submit new work and reap completions**, because `io_uring_enter(fd, to_submit=N, min_complete=M, IORING_ENTER_GETEVENTS)` does both atomically from the kernel's perspective. This is fundamentally different from the old `io_submit()`/`io_getevents()` pair, which always cost (at least) two syscalls per round-trip.

```
   Application event loop (non-SQPOLL, default mode)
   ┌───────────────────────────────────────────────────────────┐
   │  loop {                                                     │
   │    // 1. Prepare N operations                                │
   │    for op in pending_ops {                                    │
   │        sqe = io_uring_get_sqe(ring)                            │
   │        io_uring_prep_XXX(sqe, ...)                             │
   │    }                                                            │
   │                                                                  │
   │    // 2. ONE syscall: submit N, block until >=1 completion       │
   │    io_uring_enter(fd, N, 1, IORING_ENTER_GETEVENTS)                │
   │                                                                     │
   │    // 3. Drain CQ ring (pure userspace, no syscall)                 │
   │    while (cqe = io_uring_peek_cqe(ring)) {                            │
   │        handle(cqe->user_data, cqe->res)                                │
   │        io_uring_cqe_seen(ring, cqe)   // advance head                  │
   │    }                                                                    │
   │  }                                                                       │
   └───────────────────────────────────────────────────────────────────────┘
```

### 10.2 SQPOLL mode: zero syscalls in steady state

With `IORING_SETUP_SQPOLL`, the kernel spawns a dedicated thread (`io_sq_thread`, visible in `ps`/`top` as `iou-sqp-<pid>`) that continuously polls the SQ ring's tail for new entries, exactly like a userspace busy-poll loop but running in kernel context. As long as the SQPOLL thread hasn't gone idle (controlled by `sq_thread_idle`, default a few hundred ms), the application **never needs to call `io_uring_enter()` to submit** — it just writes SQEs and bumps the ring tail with the release-store, and the kernel thread picks them up on its own polling cadence.

```
   Application                         Kernel SQPOLL thread (dedicated core)
   ┌────────────────┐                  ┌───────────────────────────────┐
   │ write SQE        │                │  loop {                          │
   │ bump sq.tail  ───┼───(shared mem)─┼─►  if sq.tail changed:              │
   │  (no syscall!)    │                │       consume new SQEs               │
   │                    │                │       execute / dispatch              │
   │                    │◄───(shared mem)┼─  write CQEs, bump cq.tail          │
   │ read CQE           │                │    if idle > sq_thread_idle:          │
   │  (no syscall!)      │                │       set IORING_SQ_NEED_WAKEUP flag,  │
   └────────────────────┘                │       go to sleep                       │
                                          │  }                                       │
                                          └─────────────────────────────────────────┘
```

If the SQPOLL thread *has* gone idle, it sets `IORING_SQ_NEED_WAKEUP` in the SQ ring flags before sleeping; the application must check for this flag and, if set, make one `io_uring_enter(..., IORING_ENTER_SQ_WAKEUP)` call to rouse it. This gives you the best of both worlds: zero syscalls under sustained load, graceful fallback to a wakeup syscall under bursty/idle load. The trade-off is a dedicated CPU core spinning (or nearly so) for the SQPOLL thread — this is a throughput-for-a-core trade, appropriate for dedicated I/O-heavy services (proxies, storage engines, DB servers) but wasteful for general-purpose or low-QPS applications. SQPOLL requires elevated privilege (`CAP_SYS_NICE` historically, root-equivalent policy in many distros) specifically because a misbehaving unprivileged SQPOLL thread could be used to pin a CPU core.


---

## 11. Polling Modes: IOPOLL and SQPOLL

Don't confuse these — they poll **different things**.

- **`SQPOLL`** (§10.2): a kernel thread polls the *submission* side, so userspace avoids submit syscalls.
- **`IOPOLL`** (`IORING_SETUP_IOPOLL`): completions are reaped via **busy-polling the device**, bypassing interrupts entirely, analogous to NVMe polled-mode queues. This trades CPU cycles for lower tail latency and higher IOPS on very fast storage (NVMe SSDs) where interrupt/IRQ-handling overhead is a larger fraction of the total I/O latency than the poll cost itself.

Constraints on `IOPOLL`:
- Only works with files opened `O_DIRECT` on block devices exposing polled-queue support.
- The application must call `io_uring_enter(..., IORING_ENTER_GETEVENTS)` (or equivalent liburing wait call) to actually drive the polling — completions are **not** asynchronously pushed by an interrupt handler; they only get discovered when something calls into the poll path.
- Cannot be combined with ops that don't support polling (sockets, most non-block-device operations) on the same ring.

`SQPOLL` and `IOPOLL` **can** be combined for an extreme low-latency, all-kernel-thread-driven NVMe storage engine: the SQPOLL thread submits, and the same or another polling mechanism busy-waits on completions, achieving sub-10-microsecond round trips on modern NVMe hardware with essentially zero syscalls and zero interrupts in steady state.


---

## 12. Registered Resources: Fixed Files and Fixed Buffers

Every raw `read(2)`/`write(2)` pays hidden per-call costs beyond the syscall trap itself: `fdget()`/`fdput()` to look up the `struct file*` from the fd table (which involves an RCU read-side critical section and an atomic refcount bump/decrement), and for buffers, the kernel must pin user pages on every single I/O (`get_user_pages`) if using certain paths, or at minimum touch/validate the iovec.

io_uring lets you **pre-register** both to amortize this cost across many operations:

### 12.1 Fixed Files

```c
int io_uring_register(int ring_fd, IORING_REGISTER_FILES, int *fds, unsigned nr_fds);
```

Registers an array of file descriptors once. Afterwards, SQEs set `IOSQE_FIXED_FILE` and put the **index into that array** in `sqe->fd`, not a raw fd. The kernel keeps a stable reference (an elevated `struct file` refcount) for the lifetime of the registration, so per-I/O `fdget`/`fdput` RCU+atomic overhead is eliminated — replaced by direct array indexing. Measured savings are typically in the range of several percent of total CPU time on fd-lookup-heavy workloads (many small I/Os), and matter more as raw I/O latency shrinks (NVMe, `IOPOLL`) since fixed overhead becomes a larger relative fraction.

Since 5.13, `IORING_REGISTER_FILES_UPDATE` allows incremental add/remove without re-registering the whole table, and auto-allocated slots (`sqe->file_index` with `IORING_FILE_INDEX_ALLOC`) let the kernel pick a free slot itself (useful for `accept`-heavy servers that register each accepted socket as a fixed file for subsequent fixed-file reads/writes on that connection).

### 12.2 Fixed Buffers

```c
int io_uring_register(int ring_fd, IORING_REGISTER_BUFFERS, struct iovec *iovs, unsigned nr_iovs);
```

Registers userspace memory regions once; the kernel pins the pages (`pin_user_pages`) and, critically, can build the required DMA mapping/scatter-gather list **once at registration time** rather than on every I/O. Operations then use `IORING_OP_READ_FIXED`/`IORING_OP_WRITE_FIXED` referencing a `buf_index` instead of a raw pointer. This is the single biggest win for `O_DIRECT` block I/O workloads, where per-I/O `get_user_pages`+SG-list construction is a measurable fraction of total latency at high IOPS.

```
   Without registration:              With registered (fixed) buffer:
   ┌────────────────────┐             ┌────────────────────────────┐
   │ every read/write:    │             │ once, at setup:               │
   │  - get_user_pages()   │             │  - pin_user_pages()             │
   │  - build SG list        │             │  - build SG list once            │
   │  - do I/O                │             │                                    │
   │  - put_user_pages()       │             │ every read/write:                   │
   └────────────────────────┘             │  - lookup pre-built SG list (O(1))    │
                                          │  - do I/O                                │
                                          └──────────────────────────────────────────┘
```


---

## 13. Provided Buffers and Buffer Rings (bufring)

For **receive-side** networking (and other read-like ops) where you don't know in advance which connection will have data ready, pre-allocating a dedicated buffer per outstanding request wastes memory (imagine 100k idle connections each holding a reserved 64KB buffer "just in case"). **Provided buffers** flip the model: you give the kernel a *pool* of buffers up front, and the kernel picks one at completion time, telling you which one it used via `IORING_CQE_F_BUFFER` in the CQE flags.

### 13.1 Classic provided buffers (`IORING_OP_PROVIDE_BUFFERS`)

You submit an SQE that hands the kernel a batch of same-sized buffers tagged with a `buf_group` id. Subsequent read/recv SQEs set `IOSQE_BUFFER_SELECT` and the same `buf_group` instead of an explicit `addr`; the kernel consumes one buffer from the group per completion.

### 13.2 Ring-mapped provided buffers (`IORING_REGISTER_PBUF_RING`, 5.19+)

The classic scheme required an SQE (and CQE) just to *replenish* the buffer pool, which itself burns ring slots. The newer buffer-ring mechanism registers a **separate small ring** (its own mmap'd region) purely for buffer bookkeeping — replenishing buffers becomes pure userspace ring manipulation, no SQE/CQE round trip required at all:

```
   buf ring (per buffer group, mmap'd)
   ┌───────────────────────────────────────────┐
   │  entries: [ {addr, len, bid}, {addr,len,bid}, ... ] │
   │  tail pointer (userspace-managed)             │
   └───────────────────────────────────────────┘
        ▲                                    │
        │ app adds buffers back              │ kernel consumes on recv,
        │ after processing (no syscall)       │ advances internal cursor,
        │                                       │ echoes bid in CQE flags
```

This is essential for high-connection-count network servers (proxies, load balancers) using multishot receive (§15) — the combination of buffer rings + multishot recv gets extremely close to the theoretical minimum overhead per received network packet, with no per-packet buffer allocation and no per-packet SQE submission.


---

## 14. Linked SQEs, Chains, and Drain Semantics

### 14.1 `IOSQE_IO_LINK`

Marking an SQE with `IOSQE_IO_LINK` says "the next submitted SQE depends on this one." The kernel builds a linked list of `io_kiocb`s (`io_kiocb->link`) and enforces sequential execution: link[1] only starts after link[0] completes successfully; if link[0] fails, the rest of the chain is cancelled (each remaining link gets a CQE with `res = -ECANCELED`), unless `IOSQE_IO_HARDLINK` was used, which continues the chain regardless of prior failure.

This is how you express a dependent sequence of I/O operations **without a round trip to userspace between each one** — e.g., `openat → read → close` as a single link chain submitted together:

```
   SQE[0]: openat("/path")   [IOSQE_IO_LINK]
   SQE[1]: read(fd_from_0)    [IOSQE_IO_LINK]
   SQE[2]: close(fd_from_0)    (no link flag: end of chain)

   Execution: strictly sequential, kernel-internal, one CQE per SQE
              (3 CQEs total), but only ONE io_uring_enter() call needed
              to submit all three.
```

Note the subtlety: the *fd* produced by `openat` in SQE[0] isn't known to userspace at submission time. io_uring handles this internally for linked chains by passing the just-opened fd directly into the next link's execution context — this only works within a link chain, which is precisely why link chains exist as a primitive rather than just "submit 3 independent SQEs."

### 14.2 `IOSQE_IO_DRAIN`

A full **serialization barrier**: an SQE marked `IOSQE_IO_DRAIN` will not begin execution until *every* previously submitted SQE (drained or not) has completed, and no SQE submitted after it will begin until it itself completes. This is heavier than a link — it's a barrier across the *entire* submission history, not just a chain of two ops. Used sparingly (e.g., ensuring a batch of writes is durable before issuing an `fsync`), since it defeats the parallelism that's the entire point of io_uring if overused.


---

## 15. Multishot Operations

Historically, one SQE produced exactly one CQE, then the request was "done" — for a TCP server calling `accept()` repeatedly, this meant submitting a fresh `IORING_OP_ACCEPT` SQE after every single accepted connection, a syscall-adjacent overhead per connection.

**Multishot** operations (`IORING_ACCEPT_MULTISHOT` flag on accept, multishot poll, multishot recv since 5.19) break this 1:1 relationship: **one SQE stays "armed" and produces a continuous stream of CQEs** until explicitly cancelled or an error terminates it. Each CQE in the stream is flagged `IORING_CQE_F_MORE` to signal "expect more from this request"; the final one omits that flag.

```
   Traditional accept loop:              Multishot accept:
   ┌─────────────────────┐               ┌──────────────────────┐
   │ submit ACCEPT SQE      │               │ submit ACCEPT SQE       │
   │ wait for CQE (conn 1)    │               │  [MULTISHOT flag]         │
   │ submit ACCEPT SQE again   │               │                             │
   │ wait for CQE (conn 2)       │               │ wait for CQE (conn 1)         │
   │ submit ACCEPT SQE again       │               │   [F_MORE set]                 │
   │ wait for CQE (conn 3)           │               │ wait for CQE (conn 2)            │
   │ ... one SQE per connection        │               │   [F_MORE set]                     │
   └─────────────────────────────────┘               │ wait for CQE (conn 3)                │
                                                       │   [F_MORE set]  -- same SQE!            │
                                                       │ ... unlimited connections, ONE SQE       │
                                                       └──────────────────────────────────────────┘
```

This dramatically reduces submission-side overhead for high-connection-churn network servers, and pairs naturally with ring-provided buffers (§13.2) for multishot `recv` — a single armed SQE per socket can deliver an unbounded stream of received-data CQEs, each pulling its buffer from the shared ring-managed pool, with **zero additional submissions** for the life of the connection.


---

## 16. Opcode Catalog

A representative (not exhaustive) survey of `IORING_OP_*` values, grouped by domain — the ABI has grown to cover nearly the full surface of blocking Linux syscalls:

**File I/O**: `NOP`, `READV`, `WRITEV`, `READ`, `WRITE`, `READ_FIXED`, `WRITE_FIXED`, `FSYNC`, `SYNC_FILE_RANGE`, `FALLOCATE`, `FADVISE`, `MADVISE`, `SPLICE`, `TEE`, `OPENAT`, `OPENAT2`, `CLOSE`, `STATX`, `UNLINKAT`, `RENAMEAT`, `MKDIRAT`, `SYMLINKAT`, `LINKAT`.

**Networking**: `ACCEPT` (+ multishot), `CONNECT`, `SEND`, `RECV` (+ multishot), `SENDMSG`, `RECVMSG` (+ multishot), `SEND_ZC` / `SENDMSG_ZC` (zero-copy send), `SOCKET`, `SHUTDOWN`, `BIND`, `LISTEN` (5.19+ direct socket ops without extra syscalls).

**Polling / Timing / Sync primitives**: `POLL_ADD` (+ multishot), `POLL_REMOVE`, `POLL_UPDATE`, `TIMEOUT`, `TIMEOUT_REMOVE`, `LINK_TIMEOUT` (attach a timeout to a link chain), `WAITID` (async `waitid(2)`), `FUTEX_WAIT`/`FUTEX_WAKE` (6.7+).

**Buffer / resource management**: `PROVIDE_BUFFERS`, `REMOVE_BUFFERS`.

**Misc / control**: `ASYNC_CANCEL`, `FILES_UPDATE`, `EPOLL_CTL` (drive an epoll instance from within io_uring — useful for gradual migration of an epoll-based app), `URING_CMD` (opaque device-specific passthrough — the mechanism NVMe passthrough and other drivers use to expose custom async commands through io_uring without inventing a bespoke syscall each time).

Each opcode has a corresponding `io_uring_prep_<name>()` helper in liburing that fills the SQE's opcode-dependent unions correctly — hand-filling SQEs for anything beyond `NOP`/basic read/write is error-prone and discouraged; always prefer the prep helpers.


---

## 17. liburing: The Userspace Library

Writing directly against the raw syscalls (`io_uring_setup`, `io_uring_enter`, `io_uring_register` — none of which even have glibc wrappers, so raw programs use `syscall(2)` directly) is possible but tedious and error-prone (manual mmap offset math, manual barrier placement). **liburing** (maintained by Jens Axboe, the io_uring author himself) is the de facto standard C library wrapping all of this:

```
liburing responsibilities:
  - io_uring_queue_init() / io_uring_queue_init_params(): setup() + all mmaps in one call
  - io_uring_get_sqe(): claim next SQE slot (or NULL if full)
  - io_uring_prep_*(): ~60 helpers, one per opcode, correctly filling SQE unions
  - io_uring_submit() / io_uring_submit_and_wait(): wraps io_uring_enter() with correct flags
  - io_uring_peek_cqe() / io_uring_wait_cqe() / io_uring_for_each_cqe(): CQ consumption
  - io_uring_cqe_seen() / io_uring_cq_advance(): advance CQ head correctly (barriers handled)
  - io_uring_register_*(): fixed files/buffers/personalities/restrictions/probes
```

Almost every serious C project using io_uring (QEMU, Ceph, RocksDB's experimental io_uring backend, Envoy, nginx's io_uring support, PostgreSQL's io_uring AIO work) links against liburing rather than hand-rolling the syscall layer. It is packaged as `liburing-dev`/`liburing-devel` on major distros and its header `<liburing.h>` also re-exports the raw `<linux/io_uring.h>` ABI structs.


---

## 18. C Implementation: Real-World Examples

### 18.1 Minimal file-read example (liburing)

```c
/* file: uring_read.c -- read a file with io_uring and liburing
 * build: gcc -O2 -o uring_read uring_read.c -luring
 */
#include <liburing.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#define QUEUE_DEPTH 8
#define READ_SZ     4096

int main(int argc, char *argv[]) {
    if (argc < 2) { fprintf(stderr, "usage: %s <file>\n", argv[0]); return 1; }

    struct io_uring ring;
    /* one call: does io_uring_setup() + all mmaps */
    int ret = io_uring_queue_init(QUEUE_DEPTH, &ring, 0);
    if (ret < 0) { fprintf(stderr, "queue_init: %s\n", strerror(-ret)); return 1; }

    int fd = open(argv[1], O_RDONLY);
    if (fd < 0) { perror("open"); return 1; }

    char *buf = malloc(READ_SZ);

    /* --- submission --- */
    struct io_uring_sqe *sqe = io_uring_get_sqe(&ring);
    if (!sqe) { fprintf(stderr, "SQ ring full\n"); return 1; }

    io_uring_prep_read(sqe, fd, buf, READ_SZ, 0 /* offset */);
    sqe->user_data = 42;  /* arbitrary correlation id, echoed back in the CQE */

    ret = io_uring_submit(&ring);           /* one io_uring_enter() call */
    if (ret < 0) { fprintf(stderr, "submit: %s\n", strerror(-ret)); return 1; }

    /* --- completion --- */
    struct io_uring_cqe *cqe;
    ret = io_uring_wait_cqe(&ring, &cqe);   /* blocks (via io_uring_enter GETEVENTS) */
    if (ret < 0) { fprintf(stderr, "wait_cqe: %s\n", strerror(-ret)); return 1; }

    if (cqe->res < 0) {
        fprintf(stderr, "async read failed: %s\n", strerror(-cqe->res));
    } else {
        printf("read %d bytes (user_data=%llu):\n%.*s\n",
               cqe->res, (unsigned long long)cqe->user_data, cqe->res, buf);
    }

    io_uring_cqe_seen(&ring, cqe);  /* advance CQ head -- MUST be called per cqe */

    free(buf);
    close(fd);
    io_uring_queue_exit(&ring);     /* unmaps rings, closes ring fd */
    return 0;
}
```

### 18.2 Batched submission pattern (multiple concurrent reads)

```c
/* Submit N reads concurrently, then reap all N completions.
 * Demonstrates the core throughput pattern: prepare many, submit once. */
#include <liburing.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

struct read_ctx {
    int fd;
    char *buf;
    size_t len;
};

int submit_batch_reads(struct io_uring *ring, struct read_ctx *ctxs, int n) {
    for (int i = 0; i < n; i++) {
        struct io_uring_sqe *sqe = io_uring_get_sqe(ring);
        if (!sqe) {
            /* ring full mid-batch: flush what we have, then keep going */
            io_uring_submit(ring);
            sqe = io_uring_get_sqe(ring);
            if (!sqe) return -1; /* still full, truly saturated */
        }
        io_uring_prep_read(sqe, ctxs[i].fd, ctxs[i].buf, ctxs[i].len, 0);
        io_uring_sqe_set_data(sqe, &ctxs[i]);   /* typed pointer, not raw u64 */
    }
    return io_uring_submit(ring);  /* ONE syscall submits everything queued */
}

void reap_batch(struct io_uring *ring, int expected) {
    int reaped = 0;
    while (reaped < expected) {
        struct io_uring_cqe *cqe;
        int ret = io_uring_wait_cqe(ring, &cqe);
        if (ret < 0) { fprintf(stderr, "wait_cqe: %s\n", strerror(-ret)); break; }

        struct read_ctx *ctx = io_uring_cqe_get_data(cqe);
        if (cqe->res < 0)
            fprintf(stderr, "fd=%d failed: %s\n", ctx->fd, strerror(-cqe->res));
        else
            printf("fd=%d read %d bytes\n", ctx->fd, cqe->res);

        io_uring_cqe_seen(ring, cqe);
        reaped++;
    }
}
```

### 18.3 High-throughput echo server using multishot accept + multishot recv + provided buffer ring

This is the pattern real production network servers (proxies, load balancers) converge on — it minimizes both syscalls and per-connection buffer allocation.

```c
/* Sketch (elided error handling for brevity) of an io_uring echo server
 * using: multishot accept, ring-provided buffers, multishot recv. */
#include <liburing.h>
#include <netinet/in.h>
#include <string.h>
#include <stdio.h>
#include <unistd.h>

#define BUF_GROUP_ID 1
#define BUF_COUNT    4096
#define BUF_SIZE     2048

enum { OP_ACCEPT, OP_RECV, OP_SEND };

struct req_data { int type; int fd; };

static void setup_buffer_ring(struct io_uring *ring, struct io_uring_buf_ring **br,
                               void *buf_base) {
    struct io_uring_buf_reg reg = {
        .ring_addr = 0, /* filled by io_uring_setup_buf_ring */
        .ring_entries = BUF_COUNT,
        .bgid = BUF_GROUP_ID,
    };
    int ret;
    *br = io_uring_setup_buf_ring(ring, BUF_COUNT, BUF_GROUP_ID, 0, &ret);

    /* seed every slot with a chunk of buf_base */
    struct io_uring_buf_ring *bring = *br;
    io_uring_buf_ring_init(bring);
    for (int i = 0; i < BUF_COUNT; i++) {
        void *addr = (char *)buf_base + i * BUF_SIZE;
        io_uring_buf_ring_add(bring, addr, BUF_SIZE, i,
                               io_uring_buf_ring_mask(BUF_COUNT), i);
    }
    io_uring_buf_ring_advance(bring, BUF_COUNT);
}

int main(void) {
    struct io_uring ring;
    io_uring_queue_init(4096, &ring, 0);

    int listen_fd = socket(AF_INET, SOCK_STREAM, 0);
    int opt = 1;
    setsockopt(listen_fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
    struct sockaddr_in addr = { .sin_family = AF_INET, .sin_port = htons(8080),
                                 .sin_addr.s_addr = INADDR_ANY };
    bind(listen_fd, (struct sockaddr *)&addr, sizeof(addr));
    listen(listen_fd, 1024);

    void *buf_base = malloc((size_t)BUF_COUNT * BUF_SIZE);
    struct io_uring_buf_ring *br;
    setup_buffer_ring(&ring, &br, buf_base);

    /* arm ONE multishot accept for the whole server lifetime */
    struct req_data *accept_ctx = malloc(sizeof(*accept_ctx));
    accept_ctx->type = OP_ACCEPT; accept_ctx->fd = listen_fd;
    struct io_uring_sqe *sqe = io_uring_get_sqe(&ring);
    io_uring_prep_multishot_accept(sqe, listen_fd, NULL, NULL, 0);
    io_uring_sqe_set_data(sqe, accept_ctx);
    io_uring_submit(&ring);

    for (;;) {
        struct io_uring_cqe *cqe;
        io_uring_wait_cqe(&ring, &cqe);

        struct req_data *rd = io_uring_cqe_get_data(cqe);

        if (rd->type == OP_ACCEPT) {
            int client_fd = cqe->res;
            if (client_fd >= 0) {
                struct req_data *rctx = malloc(sizeof(*rctx));
                rctx->type = OP_RECV; rctx->fd = client_fd;
                struct io_uring_sqe *s2 = io_uring_get_sqe(&ring);
                /* multishot recv: stays armed, pulls from buffer ring per packet */
                io_uring_prep_recv_multishot(s2, client_fd, NULL, 0, 0);
                s2->flags |= IOSQE_BUFFER_SELECT;
                s2->buf_group = BUF_GROUP_ID;
                io_uring_sqe_set_data(s2, rctx);
                io_uring_submit(&ring);
            }
            /* IORING_CQE_F_MORE means the multishot accept is still armed --
               no need to re-submit accept */
        } else if (rd->type == OP_RECV) {
            if (cqe->res <= 0) {
                close(rd->fd);
                free(rd);
            } else {
                int buf_id = cqe->flags >> IORING_CQE_BUFFER_SHIFT;
                void *data = (char *)buf_base + buf_id * BUF_SIZE;
                int len = cqe->res;

                /* echo it back */
                struct req_data *sctx = malloc(sizeof(*sctx));
                sctx->type = OP_SEND; sctx->fd = rd->fd;
                struct io_uring_sqe *s3 = io_uring_get_sqe(&ring);
                io_uring_prep_send(s3, rd->fd, data, len, 0);
                io_uring_sqe_set_data(s3, sctx);
                io_uring_submit(&ring);

                /* return the buffer to the ring for reuse once send completes
                   (elided: track completion, then io_uring_buf_ring_add again) */
            }
        } else if (rd->type == OP_SEND) {
            free(rd);
        }

        io_uring_cqe_seen(&ring, cqe);
    }
}
```

Key production patterns visible above: **one armed accept for the server's entire lifetime**, **one armed recv per connection** (rather than resubmitting after every packet), and **kernel-selected buffers** (no per-packet `malloc`). This is structurally how `io_uring`-based proxies (e.g., parts of Cloudflare's and Netflix's infrastructure, and newer nginx/HAProxy experimental backends) are built.


---

## 19. Rust Implementation: Raw Syscalls

Understanding what the `io-uring` crate does for you requires seeing the raw layer once. Here's a minimal, dependency-free (beyond `libc`) Rust program issuing `io_uring_setup` and `io_uring_enter` directly via `libc::syscall`, since glibc provides no wrappers for these three syscalls.

```rust
// Cargo.toml: [dependencies] libc = "0.2"
use std::ffi::c_void;
use std::os::unix::io::RawFd;
use std::ptr;

const SYS_IO_URING_SETUP: i64 = 425;
const SYS_IO_URING_ENTER: i64 = 426;

const IORING_OFF_SQ_RING: i64 = 0x0;
const IORING_OFF_CQ_RING: i64 = 0x8000000;
const IORING_OFF_SQES: i64 = 0x10000000;

const IORING_ENTER_GETEVENTS: u32 = 1 << 0;

#[repr(C)]
#[derive(Default)]
struct IoSqringOffsets {
    head: u32, tail: u32, ring_mask: u32, ring_entries: u32,
    flags: u32, dropped: u32, array: u32, resv1: u32, resv2: u64,
}

#[repr(C)]
#[derive(Default)]
struct IoCqringOffsets {
    head: u32, tail: u32, ring_mask: u32, ring_entries: u32,
    overflow: u32, cqes: u32, flags: u32, resv1: u32, resv2: u64,
}

#[repr(C)]
#[derive(Default)]
struct IoUringParams {
    sq_entries: u32, cq_entries: u32, flags: u32,
    sq_thread_cpu: u32, sq_thread_idle: u32, features: u32,
    wq_fd: u32, resv: [u32; 3],
    sq_off: IoSqringOffsets,
    cq_off: IoCqringOffsets,
}

#[repr(C)]
struct IoUringSqe {
    opcode: u8, flags: u8, ioprio: u16, fd: i32,
    off: u64, addr: u64, len: u32,
    op_flags: u32,          // union: rw_flags/etc.
    user_data: u64,
    buf_index_or_group: u16, personality: u16, splice_fd_in_or_file_index: i32,
    addr3: u64, pad2: u64,
}

const IORING_OP_READ: u8 = 22;

unsafe fn io_uring_setup(entries: u32, params: *mut IoUringParams) -> i64 {
    libc::syscall(SYS_IO_URING_SETUP, entries as i64, params as i64)
}

unsafe fn io_uring_enter(fd: RawFd, to_submit: u32, min_complete: u32, flags: u32) -> i64 {
    libc::syscall(
        SYS_IO_URING_ENTER,
        fd as i64, to_submit as i64, min_complete as i64, flags as i64,
        ptr::null::<c_void>() as i64, 0i64,
    )
}

fn main() -> std::io::Result<()> {
    unsafe {
        let mut params = IoUringParams::default();
        let ring_fd = io_uring_setup(8, &mut params as *mut _);
        if ring_fd < 0 {
            return Err(std::io::Error::last_os_error());
        }
        let ring_fd = ring_fd as RawFd;

        // mmap the SQ ring header + array
        let sq_ring_sz =
            params.sq_off.array as usize + params.sq_entries as usize * 4;
        let sq_ptr = libc::mmap(
            ptr::null_mut(), sq_ring_sz, libc::PROT_READ | libc::PROT_WRITE,
            libc::MAP_SHARED | libc::MAP_POPULATE, ring_fd, IORING_OFF_SQ_RING,
        );
        if sq_ptr == libc::MAP_FAILED {
            return Err(std::io::Error::last_os_error());
        }

        // mmap the SQE array
        let sqes_sz = params.sq_entries as usize * std::mem::size_of::<IoUringSqe>();
        let sqes_ptr = libc::mmap(
            ptr::null_mut(), sqes_sz, libc::PROT_READ | libc::PROT_WRITE,
            libc::MAP_SHARED | libc::MAP_POPULATE, ring_fd, IORING_OFF_SQES,
        ) as *mut IoUringSqe;
        if sqes_ptr.is_null() {
            return Err(std::io::Error::last_os_error());
        }

        // mmap the CQ ring
        let cq_ring_sz = params.cq_off.cqes as usize
            + params.cq_entries as usize * 16; // sizeof(io_uring_cqe) default
        let cq_ptr = libc::mmap(
            ptr::null_mut(), cq_ring_sz, libc::PROT_READ | libc::PROT_WRITE,
            libc::MAP_SHARED | libc::MAP_POPULATE, ring_fd, IORING_OFF_CQ_RING,
        );
        if cq_ptr == libc::MAP_FAILED {
            return Err(std::io::Error::last_os_error());
        }

        // --- compute pointers from offsets (raw pointer arithmetic, as the
        // kernel ABI intends -- this is exactly what liburing does internally) ---
        let sq_tail_ptr = sq_ptr.add(params.sq_off.tail as usize) as *mut u32;
        let sq_array_ptr = sq_ptr.add(params.sq_off.array as usize) as *mut u32;

        // fill one SQE: a NOP for demonstration (opcode 0)
        let sqe = &mut *sqes_ptr.add(0);
        ptr::write_bytes(sqe as *mut IoUringSqe, 0, 1);
        sqe.opcode = 0; // IORING_OP_NOP
        sqe.user_data = 0xdead_beef;

        // publish it: array[tail & mask] = 0; tail += 1 (release semantics)
        *sq_array_ptr.add(0) = 0;
        std::sync::atomic::fence(std::sync::atomic::Ordering::Release);
        *sq_tail_ptr = 1;

        let ret = io_uring_enter(ring_fd, 1, 1, IORING_ENTER_GETEVENTS);
        if ret < 0 {
            return Err(std::io::Error::last_os_error());
        }

        // read back the CQE
        let cq_head_ptr = cq_ptr.add(params.cq_off.head as usize) as *mut u32;
        let cqes_ptr = cq_ptr.add(params.cq_off.cqes as usize) as *mut u8;
        std::sync::atomic::fence(std::sync::atomic::Ordering::Acquire);
        let res = *(cqes_ptr.add(8) as *const i32); // offset 8 = cqe->res
        println!("NOP completed, res={res}");
        *cq_head_ptr = 1;

        Ok(())
    }
}
```

This raw version makes explicit what libraries hide: manual offset computation from `io_uring_params`, manual barrier placement (`Ordering::Release`/`Ordering::Acquire` map directly onto the C `smp_store_release`/`smp_load_acquire` macros), and manual mmap lifetime management. It should reinforce why nobody ships production code at this layer — but understanding it is what lets you *read* what `io-uring`/`tokio-uring` are doing under the hood, and debug them when something goes wrong.


---

## 20. Rust Implementation: The `io-uring` Crate

[`io-uring`](https://crates.io/crates/io-uring) (maintained by tokio-rs / Quentin de Longraye et al., closely tracking liburing's feature set) is the standard low-level-but-safe(r) Rust binding — analogous in scope to liburing, not to tokio-uring's higher-level async model. It gives you the `Sq`/`Cq`/`Submitter` split with typed opcode builders, while still exposing raw completion handling (you write your own event loop and `user_data` correlation).

```rust
// Cargo.toml: [dependencies] io-uring = "0.6"
use io_uring::{opcode, types, IoUring};
use std::fs::File;
use std::os::unix::io::AsRawFd;

fn main() -> std::io::Result<()> {
    // Builder pattern mirrors io_uring_queue_init_params: allows setting
    // SQPOLL, IOPOLL, ring sizes, etc. before construction.
    let mut ring = IoUring::builder()
        .setup_sqpoll(1000)     // enable SQPOLL, 1000ms idle timeout
        .build(256)?;            // 256-entry SQ ring

    let file = File::open("/etc/hostname")?;
    let fd = types::Fd(file.as_raw_fd());

    let mut buf = vec![0u8; 4096];

    // opcode builders mirror liburing's io_uring_prep_* helpers, but
    // produce an owned `Entry` value rather than mutating an SQE slot directly.
    let read_e = opcode::Read::new(fd, buf.as_mut_ptr(), buf.len() as _)
        .offset(0)
        .build()
        .user_data(0x42);

    unsafe {
        // push() is the Rust equivalent of io_uring_get_sqe + write;
        // returns Err if the SQ ring is full (mirrors liburing's NULL check)
        ring.submission()
            .push(&read_e)
            .expect("submission queue full");
    }

    ring.submit_and_wait(1)?;   // wraps io_uring_enter with GETEVENTS, min_complete=1

    let cqe = ring.completion().next().expect("completion queue empty");
    assert_eq!(cqe.user_data(), 0x42);

    let n = cqe.result();
    if n < 0 {
        eprintln!("read failed: {}", std::io::Error::from_raw_os_error(-n));
    } else {
        println!("read {} bytes: {:?}", n, String::from_utf8_lossy(&buf[..n as usize]));
    }

    Ok(())
}
```

### 20.1 Batched submission with the `io-uring` crate

```rust
use io_uring::{opcode, types, IoUring, squeue::Entry};
use std::fs::File;
use std::os::unix::io::AsRawFd;

struct PendingRead {
    fd: File,
    buf: Vec<u8>,
}

fn submit_batch(ring: &mut IoUring, files: Vec<File>) -> std::io::Result<Vec<PendingRead>> {
    let mut pending: Vec<PendingRead> = Vec::with_capacity(files.len());

    for (i, file) in files.into_iter().enumerate() {
        let mut buf = vec![0u8; 8192];
        let fd = types::Fd(file.as_raw_fd());
        let entry: Entry = opcode::Read::new(fd, buf.as_mut_ptr(), buf.len() as _)
            .build()
            .user_data(i as u64);

        pending.push(PendingRead { fd: file, buf });

        unsafe {
            // On SQ-full mid-loop: flush and retry once, mirroring the C pattern
            if ring.submission().push(&entry).is_err() {
                ring.submit()?;
                ring.submission().push(&entry)
                    .expect("SQ ring too small even after flush");
            }
        }
    }

    ring.submit_and_wait(pending.len())?;

    for cqe in ring.completion() {
        let idx = cqe.user_data() as usize;
        let res = cqe.result();
        if res < 0 {
            eprintln!("read[{idx}] failed: {}", std::io::Error::from_raw_os_error(-res));
        } else {
            println!("read[{idx}] got {res} bytes");
        }
    }

    Ok(pending)
}
```

The crate deliberately keeps the `unsafe` boundary exactly where the kernel ABI demands it: `push()` is `unsafe` because the caller must guarantee any pointers embedded in the `Entry` (buffer addresses, iovec arrays) remain valid **until the corresponding CQE is reaped** — the kernel holds no borrow-checker-visible reference to that memory, so use-after-free/dangling-buffer bugs are a real risk class specific to io_uring-based Rust code, not eliminated by the crate. This is the same "the kernel is now a concurrent reader/writer of your memory with a lifetime the type system can't see" hazard that motivates `tokio-uring`'s ownership-passing design (§21).


---

## 21. Rust Implementation: `tokio-uring`

[`tokio-uring`](https://crates.io/crates/tokio-uring) builds a `Future`-based API on top of the `io-uring` crate, running its own single-threaded io_uring-backed executor per thread (it is **not** a drop-in replacement for tokio's default epoll-based reactor; it's a separate runtime you opt into for the I/O-heavy portion of an application, often via `tokio_uring::start(...)`).

The core design problem tokio-uring solves: `Future::poll` in ordinary Rust async can be **cancelled at any `.await` point** by simply dropping the future. But io_uring operations, once submitted, are executing **in the kernel** — you cannot "cancel" a submitted read by just dropping a Rust value; the kernel doesn't know your future was dropped, and if you dropped the buffer along with it, the kernel would be writing into freed memory. tokio-uring's answer is **ownership-passing I/O**: you hand owned buffers (`Vec<u8>`) *into* the operation, and get them back (wrapped in the result) *after* completion — the buffer's lifetime is tied to the operation's lifetime by the type system, not by a borrow.

```rust
// Cargo.toml: [dependencies] tokio-uring = "0.5"
use tokio_uring::fs::File;

fn main() -> std::io::Result<()> {
    tokio_uring::start(async {
        let file = File::open("/etc/hostname").await?;

        // Note the signature: read_at takes ownership of `buf` and returns
        // it back inside the result tuple -- this is the ownership-passing
        // pattern that makes io_uring cancellation-safe in async Rust.
        let buf = vec![0u8; 4096];
        let (res, buf) = file.read_at(buf, 0).await;
        let n = res?;

        println!("read {n} bytes: {}", String::from_utf8_lossy(&buf[..n]));

        file.close().await?;
        Ok(())
    })
}
```

### 21.1 Concurrent operations with `tokio_uring::spawn`

```rust
use tokio_uring::fs::File;

async fn read_file(path: &str) -> std::io::Result<Vec<u8>> {
    let file = File::open(path).await?;
    let buf = vec![0u8; 65536];
    let (res, buf) = file.read_at(buf, 0).await;
    let n = res?;
    file.close().await?;
    Ok(buf[..n].to_vec())
}

fn main() {
    tokio_uring::start(async {
        let paths = ["/etc/hostname", "/etc/os-release", "/proc/version"];

        // Each spawned task submits its own io_uring ops against the
        // same thread-local ring; the tokio-uring executor multiplexes
        // completions back to the right task, analogous to how epoll-based
        // tokio multiplexes wakeups -- except the "readiness" signal here
        // IS the completion itself (completion-based, not readiness-based).
        let handles: Vec<_> = paths
            .iter()
            .map(|p| tokio_uring::spawn(read_file(p)))
            .collect();

        for (path, h) in paths.iter().zip(handles) {
            match h.await.unwrap() {
                Ok(data) => println!("{path}: {} bytes", data.len()),
                Err(e) => eprintln!("{path}: error {e}"),
            }
        }
    });
}
```

### 21.2 Why this matters architecturally

```
   epoll-based async I/O (readiness model):        io_uring async I/O (completion model):
   ┌───────────────────────────────────┐            ┌────────────────────────────────────┐
   │ 1. register interest (EPOLLIN)       │            │ 1. submit operation w/ OWNED buffer    │
   │ 2. epoll_wait() says "readable"        │            │ 2. kernel performs I/O directly into    │
   │ 3. call read() yourself -- another       │            │    that buffer -- no separate "you do    │
   │    syscall, kernel copies into YOUR       │            │    the read" step                          │
   │    buffer during THIS syscall              │            │ 3. CQE arrives -- buffer handed back       │
   │ 4. buffer lifetime = trivial (you            │            │    to you, now containing the data           │
   │    own it the whole time, sync call)          │            │ 4. buffer lifetime = must survive UNTIL     │
   └───────────────────────────────────────────────┘            │    the kernel is done with it -- this is     │
                                                                  │    what forces the ownership-passing API      │
                                                                  └──────────────────────────────────────────────┘
```

This is the deepest conceptual difference between io_uring and every prior Linux async I/O model, and it's why bolting io_uring underneath an existing `Future`-based, borrow-friendly async ecosystem (like mainline tokio's `AsyncRead`/`AsyncWrite` traits, which assume a borrowed `&mut [u8]` buffer usable only during the `poll` call) required a **separate runtime** rather than a drop-in reactor swap. Work exists (e.g. `glommio`, `monoio`) building entire async runtimes around this completion-based, ownership-passing model natively rather than adapting it onto a readiness-shaped API — worth knowing about if you go deeper into this space.


---

## 22. Security Model and Notable CVEs

io_uring has had an outsized share of Linux kernel security scrutiny relative to its age, for structural reasons worth understanding rather than dismissing as "io_uring is just buggy":

1. **Enormous new attack surface, fast.** It exposes dozens of new opcodes, each with its own kernel-side handling code, added at a rapid pace during 2019-2023. New code paths handling untrusted-ish input (SQE fields fully controlled by userspace) are exactly where memory-safety bugs concentrate.
2. **Bypasses traditional syscall-auditing/LSM hooks.** Security tooling (seccomp-bpf filters, many LSM hooks, auditd rules) historically assumed one syscall = one action, filterable by syscall number. io_uring operations are dispatched from *inside* `io_uring_enter`, so a `seccomp` filter blocking `connect(2)` does **not** block `IORING_OP_CONNECT` unless specifically updated to understand io_uring — this was a well-publicized gap that container security tooling (gVisor, Docker's default seccomp profiles, Kubernetes security contexts) had to explicitly address. Several major cloud providers and container platforms **disabled io_uring by default** for a period (Google's GKE, and briefly ChromeOS/Android) specifically because of this audit/filtering gap, not because of a single exploit.
3. **Reference counting and lifetime complexity.** The completion-based, kernel-holds-a-reference-until-done model (§21.2) creates exactly the kind of complex object-lifetime graph (`io_kiocb` linked chains, deferred task_work, cross-thread completion posting) where use-after-free and double-free bugs are historically common in kernel code.

Notable examples (illustrative, not exhaustive — always check current CVE databases for a live list):
- Multiple **use-after-free** bugs in link-chain cancellation and `io-wq` worker teardown paths (various CVEs across 5.10–5.15 era), several of which had public proof-of-concept **local privilege escalation** exploits — io_uring bugs were, for a period, one of the more popular Linux kernel LPE primitives in CTF and real-world exploit research (Google Project Zero and independent researchers published several).
- Provided-buffer and buffer-ring **reference counting** issues.
- Issues in the **fixed-file table** allowing type confusion between file kinds under certain registration/update races.

**Mitigations that exist today:**
- `/proc/sys/kernel/io_uring_disabled`: a sysctl (0 = enabled for all, 1 = disabled unless `CAP_SYS_ADMIN` or a legacy per-process opt-in, 2 = fully disabled) — many hardened distros and container base images now default this to `1`.
- `IORING_REGISTER_RESTRICTIONS`: lets a privileged setup process lock down *which* opcodes and `IOSQE_*`/`IORING_ENTER_*` flags a ring may ever use, before dropping privileges — the standard hardening pattern for services that need only a small opcode subset (e.g., only file reads, never `IORING_OP_URING_CMD`).
- Ongoing upstream hardening work specifically targeting the reference-counting and worker-lifecycle classes of bugs.

The practical takeaway for anyone shipping io_uring-based software: **stay current on kernel patch levels**, use `IORING_REGISTER_RESTRICTIONS` to minimize the opcode surface your process actually needs, and be aware that container/sandbox security policies must be io_uring-aware, not just syscall-number-aware.


---

## 23. Performance Characteristics and Benchmarking

### 23.1 Where the wins actually come from

It's easy to over-attribute io_uring's speedups to "no syscalls," but the real picture has several independent contributors, roughly in order of typical impact for a well-optimized workload:

1. **Batching**: submitting N operations per `io_uring_enter()` amortizes the fixed per-syscall cost (context switch, KPTI page table swap if mitigations enabled, cache/TLB pollution) across N operations instead of paying it N times.
2. **True kernel-native async for more operation types than old AIO**: sockets, buffered file I/O, and many syscalls that had no prior async kernel path at all now get one, avoiding thread-pool-based fake-async overhead (context switches, thread wake latency) entirely.
3. **Fixed files/buffers**: eliminates `fdget`/`fdput` and `get_user_pages` per-op overhead (§12).
4. **SQPOLL/IOPOLL**: eliminates syscalls/interrupts from the steady-state path entirely, at the cost of a dedicated spinning core.
5. **Multishot + provided buffers**: eliminates per-event submission and per-event buffer allocation for high-frequency event streams (network receive, accept).

### 23.2 Realistic expectations

Public benchmarks (from the io_uring authors, and independent reproductions by projects like ScyllaDB, RocksDB, and academic papers analyzing io_uring vs epoll/AIO) generally show:
- For **small, high-IOPS `O_DIRECT` random reads on NVMe**, `IOPOLL` + fixed buffers + `SQPOLL` io_uring configurations can reach several million IOPS on a single core where `libaio` plateaus lower — this is the workload io_uring's storage-engine adopters (RocksDB, ScyllaDB) target.
- For **networking**, multishot recv + provided buffers shows the largest relative gains under **high connection counts with small messages** (the regime where epoll's per-fd `read()` syscall overhead dominates) — gains shrink for workloads already bottlenecked on large-payload throughput (network bandwidth, not syscall count) rather than syscall rate.
- For **low-concurrency, latency-insensitive workloads**, io_uring's advantage over epoll or even blocking I/O with enough threads is often marginal or even negative once you account for the CPU cost of SQPOLL's dedicated core — it is not a universal free lunch, and naive non-batched, non-SQPOLL usage (one `io_uring_enter()` per single operation) captures little of the benefit over plain synchronous syscalls, since you've just added a ring-management layer around the same one-syscall-per-op pattern.

**Rule of thumb**: io_uring's benefit is proportional to how much you can *batch* and how *hot* your I/O path is. A CLI tool doing a handful of file reads gets essentially nothing from io_uring. A database engine issuing hundreds of thousands of concurrent I/Os per second, or a proxy juggling 100k+ connections, is exactly the target workload.

### 23.3 Benchmarking methodology notes

When measuring io_uring workloads yourself:
- Always compare against a *properly tuned* baseline (multi-threaded epoll with `SO_REUSEPORT`, or `libaio` with adequate queue depth) — comparing against a naive single-threaded blocking baseline overstates io_uring's advantage.
- Separate "syscalls eliminated" from "actual wall-clock/CPU-time improvement" — use `perf stat -e syscalls:sys_enter` or `strace -c` to directly verify syscall counts dropped as expected, then separately measure throughput/latency/CPU utilization.
- Account for the SQPOLL thread's core when computing "CPU cost per request" — it's easy to show a great per-thread number that ignores an entire pinned core running at high utilization.
- Use `fio`'s `io_uring` ioengine (`--ioengine=io_uring`) for storage benchmarking; it exposes `fixedbufs`, `hipri` (IOPOLL), `sqthread_poll` (SQPOLL) as direct flags for apples-to-apples comparison against its `libaio` ioengine.


---

## 24. Comparison: io_uring vs epoll vs AIO vs io_submit

| Dimension | `epoll` | POSIX AIO (glibc) | Linux native AIO (`io_submit`) | `io_uring` |
|---|---|---|---|---|
| Model | Readiness notification | Fake async (thread pool) | True async, kernel-level | True async, kernel-level |
| File I/O support | No (readiness doesn't apply) | Yes (via threads) | `O_DIRECT` only, unreliable buffered | Full: buffered, `O_DIRECT`, any fs |
| Socket support | Excellent (its purpose) | Poor | Effectively none | Excellent, incl. zero-copy |
| Syscalls per op (steady state) | ≥2 (`epoll_wait` + `read`/`write`) | 0 direct, but thread wake/sched overhead | ≥2 (`io_submit` + `io_getevents`) | 0 (SQPOLL) to 1 (batched enter) |
| Batching | No (one fd worth of work per wakeup) | No | Partial (`io_submit` takes an array) | Yes, fully (N ops per `enter`) |
| Extensibility | Fixed (epoll events) | N/A | Frozen ABI, effectively unmaintained | Actively extended, versioned via `IORING_FEAT_*` |
| Zero-copy | No | No | Partial (`O_DIRECT` DMA) | Yes (fixed buffers, `SEND_ZC`) |
| Non-I/O ops (openat, statx, etc.) | No | No | No | Yes |
| Kernel version needed | Ancient (2.6+) | glibc, any kernel | 2.6+ | 5.1+ (full feature set: 5.19+/6.x) |
| Security surface | Mature, well-audited | Userspace, low kernel risk | Small, mature | Large, actively hardened |
| Best for | Moderate-connection-count network servers, general event loops | Legacy compatibility only — avoid in new code | Legacy `O_DIRECT`-only storage engines | New high-performance storage and network code |

The practical migration story: `io_uring` is a strict superset of capability over all three predecessors, and its `IORING_OP_EPOLL_CTL` and `IORING_OP_POLL_ADD` opcodes even let you drive an existing epoll-shaped design through io_uring incrementally rather than requiring a big-bang rewrite. Native Linux AIO (`io_submit`) is effectively legacy at this point; new storage-engine code targeting modern kernels should default to io_uring.


---

## 25. Debugging, Tracing, and Observability

### 25.1 `/proc` and `/sys` visibility

- `/proc/<pid>/fdinfo/<ring_fd>` — for a process holding an io_uring fd, this exposes live ring state: `SqMask`, `SqHead`, `SqTail`, `CqMask`, `CqHead`, `CqTail`, `UserFiles`, `UserBufs`, and per-opcode operation counters (`Ops: ...`) — invaluable for checking whether a ring is actually saturated, leaking unreaped CQEs, or has an unexpectedly large registered-file table.
- `/proc/sys/kernel/io_uring_disabled` and `/proc/sys/kernel/io_uring_group` — the security policy sysctls (§22).
- `ps`/`top`/`/proc/<pid>/comm` for SQPOLL threads — they appear as separate kernel threads named `iou-sqp-<owner_pid>`, and `io-wq` workers appear as `iou-wrk-<owner_pid>`. Seeing unexpectedly many `iou-wrk-*` threads is a strong signal that operations are falling back to the async worker pool instead of completing inline — often because buffered (non-`O_DIRECT`) file I/O is being used, which frequently can't complete inline.

### 25.2 Tracepoints

The kernel exposes a rich set of io_uring tracepoints under `/sys/kernel/debug/tracing/events/io_uring/` (requires `debugfs` mounted, `CAP_SYS_ADMIN` or perf_event access):

```bash
# List available io_uring tracepoints
sudo ls /sys/kernel/debug/tracing/events/io_uring/
# io_uring_create, io_uring_queue_async_work, io_uring_submit_sqe,
# io_uring_complete, io_uring_fail_link, io_uring_link, io_uring_cqring_wait, ...

# Trace live submission/completion activity with perf
sudo perf trace -e io_uring:* -p <pid>

# Or via ftrace directly
echo 1 | sudo tee /sys/kernel/debug/tracing/events/io_uring/enable
sudo cat /sys/kernel/debug/tracing/trace_pipe
```

`io_uring_submit_sqe` and `io_uring_complete` tracepoints include `user_data`, letting you correlate exactly which submitted operation produced which completion and how long it took — essential for diagnosing "why is this one request slow" issues in a ring handling thousands of concurrent operations.

### 25.3 `strace` limitations

`strace` still works for the syscall-level view (`io_uring_setup`, `io_uring_enter`, `io_uring_register` calls are visible and their arguments decoded in reasonably recent `strace` versions), but it is structurally blind to **what happened inside the ring** between `enter()` calls — with `SQPOLL`, you may see almost no `io_uring_enter` calls at all despite heavy I/O activity, since the kernel thread is polling independently. Use `/proc/<pid>/fdinfo` and tracepoints, not `strace`, as the primary tool for understanding steady-state io_uring behavior.

### 25.4 `liburing`'s own diagnostic helpers

- `IORING_REGISTER_PROBE`: query which opcodes the running kernel actually supports before use — `io_uring_opcode_supported()` in liburing wraps this. Always probe rather than assuming based on kernel version strings, since distro backports vary.
- Build liburing's bundled `liburing-test` / example tools (`fio`'s io_uring engine, or liburing's own `examples/` directory containing `io_uring-cp`, a full copy utility, and `link-cp`, demonstrating linked SQEs) as reference implementations to diff your own code against when something misbehaves.


---

## 26. Expert Mental Model

Condensing everything above into the handful of principles an expert actually keeps in their head when designing or debugging io_uring-based systems:

1. **Two lock-free SPSC rings, mmap-shared, barrier-disciplined.** Everything else is built on top of this. If you understand how a single-producer/single-consumer ring buffer needs release-stores on the producer side and acquire-loads on the consumer side to be correct without locks, you understand io_uring's foundation — it is not a novel synchronization primitive, it's a very carefully productionized instance of a pattern you likely already know from lock-free systems programming.

2. **The completion model inverts buffer ownership.** In readiness-based I/O (epoll), you own your buffer the whole time and the kernel only touches it synchronously inside a `read()`/`write()` call you control. In io_uring, **the kernel holds a live, unsupervised reference to your buffer from submission until completion** — this is the source of essentially every subtle correctness bug in io_uring code (dropping a buffer too early, reusing a buffer for a second operation before the first completes, stack-allocated buffers going out of scope). tokio-uring's ownership-passing API design exists entirely to make this hazard a compile-time impossibility rather than a runtime footgun.

3. **Syscall elimination is a spectrum, not a boolean.** Default mode: still 1 syscall per `io_uring_enter()`, but batched across N ops. SQPOLL: 0 syscalls in steady state, 1 core dedicated. IOPOLL: 0 interrupts, CPU spent polling instead. Multishot: 0 *submissions* per event after the first. Choose the point on this spectrum that matches your actual bottleneck (syscall rate vs. CPU budget vs. latency tail) — don't reach for SQPOLL by default; it costs a core.

4. **Fixed resources trade setup cost for per-op cost.** Registering files/buffers is itself work (pinning pages, building SG lists) — it only pays off when the *same* files/buffers are reused across *many* operations. For one-shot or rarely-reused resources, registration is pure overhead.

5. **The ABI is a contract of offsets, not a fixed struct layout.** Always read `sq_off`/`cq_off` back from the kernel; never hardcode mmap byte offsets. This is what has let the kernel evolve the internal ring layout across six years of releases without breaking a single existing binary.

6. **Opcode-dependent field reuse is the ABI's biggest ergonomic wart.** The SQE's unions mean `sqe->addr` might be a buffer pointer, a `struct msghdr*`, or a `splice_off_in` depending entirely on `opcode`. Always go through `io_uring_prep_*()` helpers (liburing) or their typed equivalents (`opcode::*::new()` in the Rust crate) rather than hand-filling fields — the union layout is not self-documenting and hand-filling is a well-known source of subtle bugs even among experienced kernel engineers.

7. **Security posture must be io_uring-aware, not syscall-aware.** If you operate in a multi-tenant, sandboxed, or otherwise adversarial-input environment, treat io_uring as a distinct capability surface requiring its own explicit allow-listing (`IORING_REGISTER_RESTRICTIONS`) — assuming your existing `seccomp` policy "covers" a process just because it restricts raw syscalls is a documented, exploited gap.

8. **Measure batching and inline-completion rate before optimizing further.** The two questions that predict whether an io_uring redesign will actually help: "how many operations do I typically have in flight/preparable at once?" (low numbers mean little batching benefit) and "how many of my operations complete inline vs. get punted to `io-wq`?" (a high `io-wq` punt rate, visible via `iou-wrk-*` thread activity, means you're not getting the syscall-elimination benefit you might expect, and the underlying operation type — often buffered I/O on certain filesystems — may need `O_DIRECT` or a different opcode to actually benefit).

