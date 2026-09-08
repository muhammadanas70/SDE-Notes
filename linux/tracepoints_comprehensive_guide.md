# Tracepoints: A Comprehensive, In-Depth Guide

> *From kernel internals to userspace USDT, from C macros to Rust abstractions, from static hooks to eBPF consumers — everything you need to build a complete mental model.*

---

## Table of Contents

1. [What Is a Tracepoint?](#1-what-is-a-tracepoint)
2. [Mental Model: The Big Picture](#2-mental-model-the-big-picture)
3. [History and Motivation](#3-history-and-motivation)
4. [Static vs Dynamic Tracing](#4-static-vs-dynamic-tracing)
5. [Linux Kernel Tracepoint Infrastructure](#5-linux-kernel-tracepoint-infrastructure)
   - 5.1 [Core Data Structures](#51-core-data-structures)
   - 5.2 [The TRACE_EVENT Macro System](#52-the-trace_event-macro-system)
   - 5.3 [Tracepoint Registration and Probing](#53-tracepoint-registration-and-probing)
   - 5.4 [The RCU Mechanism and Call Sites](#54-the-rcu-mechanism-and-call-sites)
   - 5.5 [Jump Labels / Static Keys Optimization](#55-jump-labels--static-keys-optimization)
6. [Tracepoint Consumers](#6-tracepoint-consumers)
   - 6.1 [ftrace Ring Buffer](#61-ftrace-ring-buffer)
   - 6.2 [perf_events](#62-perf_events)
   - 6.3 [eBPF Programs](#63-ebpf-programs)
   - 6.4 [LTTng](#64-lttng)
7. [The tracefs / debugfs Interface](#7-the-tracefs--debugfs-interface)
8. [TRACE_EVENT Macro Deep Dive](#8-trace_event-macro-deep-dive)
9. [Writing Your Own Kernel Tracepoint in C](#9-writing-your-own-kernel-tracepoint-in-c)
10. [User-Space Tracepoints (USDT)](#10-user-space-tracepoints-usdt)
    - 10.1 [What Is USDT?](#101-what-is-usdt)
    - 10.2 [SystemTap SDT Mechanism](#102-systemtap-sdt-mechanism)
    - 10.3 [ELF Note Sections and .probes](#103-elf-note-sections-and-probes)
    - 10.4 [Uprobes: The Kernel Side of USDT](#104-uprobes-the-kernel-side-of-usdt)
11. [Implementing USDT in C](#11-implementing-usdt-in-c)
12. [Tracepoints in Rust](#12-tracepoints-in-rust)
    - 12.1 [Kernel Tracepoints via rust/kernel Crate](#121-kernel-tracepoints-via-rustkernel-crate)
    - 12.2 [USDT in Rust with probe-rs / tracing crate](#122-usdt-in-rust-with-probe-rs--tracing-crate)
    - 12.3 [eBPF Tracepoint Consumer in Rust (Aya)](#123-ebpf-tracepoint-consumer-in-rust-aya)
13. [eBPF and Tracepoints: Deep Integration](#13-ebpf-and-tracepoints-deep-integration)
    - 13.1 [BPF_PROG_TYPE_TRACEPOINT](#131-bpf_prog_type_tracepoint)
    - 13.2 [Raw Tracepoints](#132-raw-tracepoints)
    - 13.3 [BTF and Type Information](#133-btf-and-type-information)
14. [Performance Model and Overhead Analysis](#14-performance-model-and-overhead-analysis)
15. [Tracing Data Flow End-to-End](#15-tracing-data-flow-end-to-end)
16. [Filtering and Triggering](#16-filtering-and-triggering)
17. [Tracepoint vs Kprobe vs Uprobe vs Ftrace](#17-tracepoint-vs-kprobe-vs-uprobe-vs-ftrace)
18. [Real-World Use Cases and Patterns](#18-real-world-use-cases-and-patterns)
19. [Security Considerations](#19-security-considerations)
20. [Complete ASCII Architecture Reference](#20-complete-ascii-architecture-reference)
21. [Quick Reference Cheatsheet](#21-quick-reference-cheatsheet)

---

## 1. What Is a Tracepoint?

A **tracepoint** is a **statically defined, strategically placed hook** inside source code — kernel or userspace — that can be activated at runtime to emit structured diagnostic data without modifying, recompiling, or restarting the program being observed.

The key properties that distinguish tracepoints from all other instrumentation mechanisms:

| Property | Meaning |
|---|---|
| **Static** | The hook site is baked into the binary at compile time |
| **Stable** | The name and argument semantics are part of the ABI |
| **Conditional** | When no consumer is attached, cost is essentially zero |
| **Structured** | Events have a schema: named fields, typed arguments |
| **Multi-consumer** | Multiple tools (perf, ftrace, eBPF) can attach simultaneously |

A tracepoint is not a `printk`. It is not a log. It is a **probe site with a guaranteed calling convention** — a stable, well-typed event emission interface embedded directly in the code path.

```
                      Source Code
                          |
               [TRACE_EVENT / DTRACE_PROBE]
                          |
                   compile-time hook
                          |
         ┌────────────────┴────────────────┐
         │           runtime check          │
         │   enabled?  NO ──► NOP (fast)   │
         │            YES ──► call probes   │
         └──────────────────────────────────┘
```

---

## 2. Mental Model: The Big Picture

Before diving into implementation details, build this layered mental model:

```
┌─────────────────────────────────────────────────────────────────────┐
│                      LAYER 5: CONSUMERS / TOOLS                     │
│  perf(1)   trace-cmd   bpftrace   BCC   LTTng   SystemTap   strace  │
└───────────────────────────────┬─────────────────────────────────────┘
                                │  reads ring buffers / perf events
┌───────────────────────────────▼─────────────────────────────────────┐
│                    LAYER 4: SUBSYSTEM INTERFACES                     │
│   tracefs (/sys/kernel/tracing)    perf_event_open(2)   bpf(2)      │
└───────────────────────────────┬─────────────────────────────────────┘
                                │  dispatch to handlers
┌───────────────────────────────▼─────────────────────────────────────┐
│                     LAYER 3: EVENT SUBSYSTEM                         │
│   struct trace_event_class   struct trace_event_call                 │
│   trace_event_reg()          event_filter engine                     │
└───────────────────────────────┬─────────────────────────────────────┘
                                │  calls registered probe functions
┌───────────────────────────────▼─────────────────────────────────────┐
│                    LAYER 2: TRACEPOINT CORE                          │
│   struct tracepoint            tracepoint_probe_register/unregister  │
│   tracepoint_funcs[]           RCU-protected probe list              │
│   static_key (jump label)      __tracepoint_* symbols                │
└───────────────────────────────┬─────────────────────────────────────┘
                                │  executed at call sites
┌───────────────────────────────▼─────────────────────────────────────┐
│                    LAYER 1: CALL SITES IN CODE                       │
│   trace_sched_switch()   trace_net_dev_xmit()   trace_sys_enter()   │
│   These are macros expanding to: if(enabled) { call probes }        │
└─────────────────────────────────────────────────────────────────────┘
```

The key insight: **call sites are the producers**, the **tracepoint core is the dispatcher**, and **ftrace/perf/eBPF are consumers**. The tracepoint mechanism decouples producers from consumers entirely.

---

## 3. History and Motivation

### The Pre-Tracepoint Problem

Before tracepoints (~2009), kernel developers faced a hard trilemma:

1. **`printk`** — safe but extremely slow, floods consoles, unusable for hot paths
2. **`kprobes`** — dynamic, but unstable (ABI can change any kernel version), INT3 overhead
3. **`SystemTap`** — powerful but requires kernel-devel, DWARF info, interpreter overhead

The core problems with older approaches:

- **No stable ABI**: if you probe `do_sys_open+0x12`, a new kernel compilation moves that offset
- **No structure**: raw register dumps require DWARF to decode
- **All-or-nothing cost**: even disabled probes had overhead with some mechanisms
- **Single-consumer**: you couldn't have `perf` and `ftrace` watching the same event

### Ingo Molnár's TRACE_EVENT Framework (2008–2009)

Ingo Molnár, Mathieu Desnoyers, and Steven Rostedt designed the modern tracepoint system. The goals:

1. Tracepoints should be **zero-cost when disabled** (jump labels)
2. Tracepoints should have a **stable, versioned ABI** (part of the kernel's stable interface)
3. Tracepoints should carry **structured, typed data** (format strings in tracefs)
4. **Multiple consumers** can attach simultaneously via probe function lists
5. A **single macro** (`TRACE_EVENT`) should generate all needed code automatically

This resulted in the system described throughout this guide.

### USDT (User Statically Defined Tracing)

USDT originated in Solaris DTrace (Sun Microsystems, 2003) as a way to add tracepoints to userspace applications with the same zero-cost-when-disabled guarantees as kernel tracepoints.

Linux adopted USDT through SystemTap's `<sys/sdt.h>` header, which uses `asm` directives to embed NOP instructions at probe sites and note sections in ELF binaries to describe them.

---

## 4. Static vs Dynamic Tracing

Understanding the fundamental distinction is essential:

```
TRACING TAXONOMY
════════════════

STATIC                              DYNAMIC
────────────────────────────────    ────────────────────────────────
Site defined at compile time        Site defined at runtime
Stable ABI                          Unstable (offset-dependent)
Near-zero disabled overhead         Always some overhead
Structured data (typed fields)      Raw data (register state + DWARF)
Requires source modification        Works on any compiled binary
Examples:                           Examples:
  - Kernel tracepoints                - kprobes
  - USDT                              - uprobes (without USDT)
  - DTrace probes                     - ftrace function tracer
                                      - perf dynamic events
```

Tracepoints are **static** tracing — the sites are permanently embedded. But the **handlers** (what happens when a tracepoint fires) are dynamic — registered and unregistered at runtime.

This combination gives you:
- Compile-time stability (site won't move between kernel updates)
- Runtime flexibility (attach/detach observers without rebooting)
- Production safety (disabled tracepoints are nearly free)

---

## 5. Linux Kernel Tracepoint Infrastructure

### 5.1 Core Data Structures

The tracepoint system is built on a small set of fundamental structures. Let's examine them with full detail:

```c
/* include/linux/tracepoint-defs.h */

struct tracepoint_func {
    void        *func;      /* Pointer to the probe callback function */
    void        *data;      /* Opaque data passed to probe at call time */
    int          prio;      /* Priority (higher = called first) */
};

struct tracepoint {
    const char              *name;      /* Tracepoint name: "sched_switch" */
    struct static_key        key;       /* Jump label: enabled/disabled */
    struct static_call_key  *static_call_key;  /* Static call optimization */
    void                    *static_call_tramp; /* Trampo for static calls */
    void                    *iterator;  /* Multi-probe iterator function */
    void                    *probestub; /* Stub when no probes */
    int (*regfunc)(void);               /* Called when first probe attaches */
    void (*unregfunc)(void);            /* Called when last probe detaches */
    struct tracepoint_func __rcu *funcs; /* RCU-protected probe function list */
};
```

The `struct tracepoint` is the **central control structure**. It lives in the `.data` section of the kernel binary. Each tracepoint declaration creates exactly one instance.

The `struct static_key` (also called a "jump label") is the performance-critical field — it controls whether the tracepoint's code path is executed. More on this in section 5.5.

The `funcs` field is an RCU-protected, NULL-terminated array of `tracepoint_func` entries. When a consumer (ftrace, perf, eBPF) attaches to a tracepoint, it appends an entry here. When it detaches, the entry is removed.

```
struct tracepoint layout in memory:
═══════════════════════════════════

offset  field               size
──────  ──────────────────  ────
  0     name (ptr)          8
  8     key (static_key)    8
 16     static_call_key     8
 24     static_call_tramp   8
 32     iterator            8
 40     probestub           8
 48     regfunc             8
 56     unregfunc           8
 64     funcs (RCU ptr)     8
──────
 72     total bytes
```

### The `__tracepoint_*` and `__traceiter_*` symbols

For every `DEFINE_TRACE(name)` or `TRACE_EVENT(name, ...)`, the preprocessor generates:

```c
/* Actual tracepoint struct */
struct tracepoint __tracepoint_##name;

/* The iterator function: calls all registered probes */
void __traceiter_##name(void *__data, proto...);

/* The call-site check macro */
#define trace_##name(args...) \
    do { \
        if (static_key_false(&__tracepoint_##name.key)) \
            __DO_TRACE(...); \
    } while (0)
```

The split between `__tracepoint_*` (the struct) and `__traceiter_*` (the function) matters because:
- The struct holds the probe list and metadata
- The iterator function is what's actually called when the tracepoint fires
- Static call optimization can replace an indirect call through the struct with a direct call

### 5.2 The TRACE_EVENT Macro System

`TRACE_EVENT` is the highest-level API. It's a **macro that expands into multiple definitions** across multiple header inclusions via a technique called "X-macros" or "macro magic."

The full expansion happens through a multi-pass trick using `#include` with different `#define` values for `TRACE_EVENT`.

Here is the macro signature:

```c
TRACE_EVENT(name,
    TP_PROTO(proto...),       /* C prototype of trace call */
    TP_ARGS(args...),         /* Argument list */
    TP_STRUCT__entry(         /* Fields in the ring buffer entry */
        __field(type, name)
        __array(type, name, len)
        __string(name, src)
        __dynamic_array(type, name, len)
        __bitmask(name, nr_bits)
    ),
    TP_fast_assign(           /* How to fill ring buffer fields */
        __entry->field = value;
    ),
    TP_printk(fmt, args...)   /* How to format for human reading */
);
```

Let's trace the full expansion. The header `include/trace/define_trace.h` is included *six times* with a different definition of `TRACE_EVENT` each time:

```
Pass 1: DECLARE_EVENT_CLASS      → struct trace_event_data_offsets_##name
Pass 2: DEFINE_EVENT             → extern struct tracepoint __tracepoint_##name
Pass 3: TRACE_INCLUDE            → struct trace_entry layout
Pass 4: CREATE_TRACE_POINTS      → actual struct definitions + functions
Pass 5: PERF_TRACE_EVENT         → perf_trace_##name() function
Pass 6: TRACE_SYSCALL_TABLE      → syscall table entries (for syscall events)
```

The most important passes are 1 and 4. Pass 4 generates:

```c
/* The ring buffer entry struct */
struct trace_event_raw_##name {
    struct trace_entry  ent;    /* Common header: type, flags, preempt_count, pid */
    /* ... fields from TP_STRUCT__entry ... */
    char                __data[0];  /* Variable-length area */
};

/* The class descriptor */
struct trace_event_class event_class_##name = {
    .system         = TRACE_SYSTEM,  /* subsystem name e.g. "sched" */
    .fields_array   = trace_event_fields_##name,
    .fields         = LIST_HEAD_INIT(...),
    .raw_init       = trace_event_raw_init,
    .probe          = trace_event_raw_event_##name,   /* ftrace handler */
    .reg            = trace_event_reg,
    .perf_probe     = perf_trace_##name,              /* perf handler */
};

/* The event call descriptor */
struct trace_event_call event_##name = {
    .class          = &event_class_##name,
    .name           = #name,
    .print_fmt      = TP_printk_str,
    .flags          = TRACE_EVENT_FL_TRACEPOINT,
    .tp             = &__tracepoint_##name,
};

/* The probe function called by the tracepoint */
static void trace_event_raw_event_##name(void *__data, proto...)
{
    struct trace_event_file *trace_file = __data;
    struct trace_event_data_offsets_##name __data_offsets;
    struct trace_event_buffer fbuffer;
    struct trace_event_raw_##name *entry;
    int __data_size;

    /* ... allocate ring buffer space ... */
    /* ... fill fields via TP_fast_assign ... */
    /* ... commit entry ... */
}
```

This is a lot. The key takeaway: **one `TRACE_EVENT(...)` macro generates a complete, self-contained event subsystem** — the struct layout, the probe functions for both ftrace and perf, the format descriptors, and the tracepoint itself.

### 5.3 Tracepoint Registration and Probing

When a consumer (say, ftrace) wants to enable a tracepoint, it calls:

```c
int tracepoint_probe_register(struct tracepoint *tp,
                              void *probe,
                              void *data);

int tracepoint_probe_unregister(struct tracepoint *tp,
                                void *probe,
                                void *data);
```

Internally, this does:

```
tracepoint_probe_register()
        │
        ├─ mutex_lock(&tracepoints_mutex)
        │
        ├─ Allocate new tracepoint_func[] array
        │   (old_array + new entry, RCU-safe copy)
        │
        ├─ rcu_assign_pointer(tp->funcs, new_array)
        │   (memory barrier, new probes visible to readers)
        │
        ├─ If first probe: call tp->regfunc()
        │   (allows subsystem to set up state)
        │
        ├─ static_key_enable(&tp->key)
        │   (NOW call sites take the "enabled" branch)
        │
        └─ synchronize_rcu()
            (wait for all pre-update readers to finish)
```

The RCU (Read-Copy-Update) mechanism is crucial here. Call sites read `tp->funcs` under RCU read lock. The registration path uses `rcu_assign_pointer` to atomically publish the new probe list. Old readers finish their current probe invocations using the old list, and only after `synchronize_rcu()` returns can the old list be freed.

### 5.4 The RCU Mechanism and Call Sites

The generated call-site code (what `trace_sched_switch()` expands to) looks like:

```c
static inline void trace_sched_switch(bool preempt,
                                      struct task_struct *prev,
                                      struct task_struct *next,
                                      unsigned int prev_state)
{
    /* Jump label check — see 5.5 for why this is a NOP when disabled */
    if (static_key_false(&__tracepoint_sched_switch.key)) {
        struct tracepoint_func *it_func_ptr;
        void *__data;

        /* Enter RCU read-side critical section */
        rcu_read_lock_sched_notrace();

        /* Load probe function list atomically */
        it_func_ptr =
            rcu_dereference_sched(__tracepoint_sched_switch.funcs);

        if (it_func_ptr) {
            /* Call each registered probe */
            do {
                __data = (it_func_ptr)->data;
                ((void(*)(void *, bool, struct task_struct *,
                          struct task_struct *, unsigned int))
                 (it_func_ptr)->func)(__data, preempt, prev,
                                      next, prev_state);
                ++it_func_ptr;
            } while ((it_func_ptr)->func);
        }

        rcu_read_unlock_sched_notrace();
    }
}
```

The complete call sequence when a tracepoint fires with one probe attached:

```
CPU executes trace_sched_switch()
        │
        ├─ static_key_false() → evaluates to false (branch NOT taken) when DISABLED
        │   evaluates to true (branch taken) when ENABLED
        │
        ├─ [ENABLED PATH]
        │   rcu_read_lock_sched_notrace()
        │       preemption disabled, memory barriers
        │   │
        │   rcu_dereference_sched(tp->funcs)
        │       load pointer with acquire semantics
        │   │
        │   Loop: call each probe function
        │       probe(data, preempt, prev, next, prev_state)
        │       │
        │       └─ e.g., trace_event_raw_event_sched_switch()
        │           │
        │           ├─ ring_buffer_lock_reserve()  ← allocate space
        │           ├─ fill entry fields           ← TP_fast_assign
        │           └─ ring_buffer_unlock_commit() ← publish to reader
        │   │
        │   rcu_read_unlock_sched_notrace()
        │
        └─ [DISABLED PATH] → immediate return, ~0 cycles overhead
```

### 5.5 Jump Labels / Static Keys Optimization

This is the mechanism that makes disabled tracepoints essentially free. It is one of the most clever pieces of engineering in the Linux kernel.

**The problem**: a naive `if (enabled)` check costs ~3–5 cycles per tracepoint invocation even when disabled — the branch predictor handles the unidirectional branch well, but there's still a comparison, a potential pipeline stall, and the branch prediction table entry to maintain.

**The solution**: **self-modifying code**. When a tracepoint is disabled, the `if (static_key_false(...))` check is compiled to an **unconditional NOP** instruction. When enabled, that NOP is patched in-place to an **unconditional JMP** to the slow path.

```
x86_64 instruction stream:

DISABLED state:
  0x00:  0F 1F 44 00 00    NOP DWORD PTR [RAX+RAX*1+0x0]  (5-byte NOP)
  0x05:  <next instruction>

ENABLED state (after patching):
  0x00:  E9 XX XX XX XX    JMP rel32 → slow_path
  0x05:  <next instruction>  (never reached when enabled)
```

The patching happens via `text_poke_bp()` which uses an INT3 breakpoint trick to safely modify live kernel text:

```
text_poke_bp() sequence:
═══════════════════════

Step 1: Write INT3 to first byte of target instruction
        (any CPU hitting this will trap to INT3 handler,
         which emulates the original instruction)

Step 2: smp_wmb() + IPI to all CPUs to sync instruction cache

Step 3: Write remaining bytes of new instruction
        (safe because first byte is INT3, not partial NOP/JMP)

Step 4: Write correct first byte (NOP or E9)

Step 5: IPI to all CPUs again to flush instruction cache
```

The result: **disabled tracepoints cost exactly the same as the NOP instruction** — on modern x86 CPUs, a 5-byte NOP takes 0–1 cycles and is eliminated by the decoder. The overhead is genuinely near-zero (measured at ~0.2ns on modern hardware).

**The `__jump_table` section**

Jump labels work because the compiler emits a special relocation table entry for each `static_key` usage:

```c
/* Each static_key usage generates an entry in __jump_table: */
struct jump_entry {
    jump_label_t  code;    /* Address of the NOP/JMP in text */
    jump_label_t  target;  /* Address of the slow path */
    jump_label_t  key;     /* Address of the static_key struct */
};
```

This table lives in the `__jump_table` ELF section. When `static_key_enable()` is called, the kernel:
1. Looks up all `jump_entry` records with `key == &this_key`
2. For each one, patches the instruction at `code` from NOP→JMP or JMP→NOP

---

## 6. Tracepoint Consumers

Multiple subsystems can consume the same tracepoints simultaneously. Each registers its own probe function pointer.

### 6.1 ftrace Ring Buffer

The **ftrace ring buffer** is the primary storage mechanism for tracepoint data. It's a per-CPU, lock-free, overwrite-capable ring buffer optimized for low-overhead tracing.

```
ftrace ring buffer architecture:
═════════════════════════════════

Per-CPU layout:
  CPU 0:  [head_page] → [page] → [page] → ... → [tail_page] → (wraps to head)
  CPU 1:  [head_page] → [page] → [page] → ... → [tail_page] → (wraps to head)
  CPU 2:  ...
  CPU N:  ...

Each page (typically 4KB):
  ┌─────────────────────────────────────────────────┐
  │ struct buffer_page header (commit, write, read) │
  ├─────────────────────────────────────────────────┤
  │ event_entry_0: [type|len][data...]               │
  │ event_entry_1: [type|len][data...]               │
  │ event_entry_2: [type|len][data...]               │
  │ ...                                             │
  │ (free space)                                    │
  └─────────────────────────────────────────────────┘

Time stamp compression:
  - Absolute timestamp on first event per page
  - Delta timestamps (27-bit) for subsequent events
  - Saves 5 bytes per event on average
```

The ring buffer write path (extremely optimized):

```c
/* Simplified write path */
static struct ring_buffer_event *
rb_reserve_next_event(struct ring_buffer_per_cpu *cpu_buffer,
                      unsigned long length)
{
    /* 1. Disable preemption (caller already in NMI/IRQ context or has
          preemption disabled) */

    /* 2. Get write position atomically */
    tail = local_read(&cpu_buffer->tail);

    /* 3. Check if current page has space */
    if (tail + length > BUF_PAGE_SIZE) {
        /* Move to next page (may overwrite oldest data) */
        rb_move_tail(cpu_buffer);
    }

    /* 4. Bump write pointer atomically */
    local_add(length, &cpu_buffer->tail);

    /* 5. Return pointer to reserved space */
    return (void *)(cpu_buffer->tail_page->page + tail);
    /* Note: no lock acquired — the local_add is the "lock" */
}
```

Key design: `local_read/local_add` are **per-CPU atomic operations** that don't require cache-line bouncing between CPUs. This is why ftrace can trace at millions of events per second per CPU.

### 6.2 perf_events

The `perf` subsystem attaches to tracepoints via `perf_event_open(2)` with `attr.type = PERF_TYPE_TRACEPOINT` and `attr.config = event_id`.

The event ID comes from `/sys/kernel/tracing/events/subsystem/event/id`.

```c
/* perf_event_open example: attach to sched_switch */
struct perf_event_attr attr = {
    .type           = PERF_TYPE_TRACEPOINT,
    .config         = 316,   /* ID from tracefs */
    .sample_type    = PERF_SAMPLE_RAW | PERF_SAMPLE_TIME | PERF_SAMPLE_CPU,
    .sample_period  = 1,     /* Every occurrence */
    .wakeup_events  = 100,   /* Wake reader every 100 events */
    .size           = sizeof(attr),
};
int fd = perf_event_open(&attr, -1, 0, -1, 0);
/* Then mmap the fd to get the ring buffer */
```

The `perf_trace_##name` function (generated by TRACE_EVENT) is registered as the probe. It packages the event data into a `perf_raw_record` and calls `perf_trace_buf_submit()`.

### 6.3 eBPF Programs

eBPF is now the dominant way to consume tracepoints. An eBPF program of type `BPF_PROG_TYPE_TRACEPOINT` is attached to a tracepoint and executed synchronously in the kernel when the tracepoint fires.

The eBPF program receives a pointer to the trace event data as its context argument. The verifier ensures the program is safe (no unbounded loops, no invalid memory access).

```c
/* BPF program attaching to sched:sched_switch */
SEC("tracepoint/sched/sched_switch")
int trace_sched_switch(struct trace_event_raw_sched_switch *ctx)
{
    /* ctx->prev_comm, ctx->prev_pid, ctx->next_pid etc. are directly
       accessible — BTF provides the type information */
    bpf_printk("switch: %s → %s\n", ctx->prev_comm, ctx->next_comm);
    return 0;
}
```

See section 13 for full eBPF+tracepoint details.

### 6.4 LTTng

LTTng (Linux Trace Toolkit next generation) is a high-throughput tracing framework that also hooks into kernel tracepoints. It uses its own lockless ring buffer implementation (separate from ftrace) optimized for sustained high-throughput tracing with minimal data loss.

LTTng modules register themselves as tracepoint probes using the same `tracepoint_probe_register()` API, but write into its own per-CPU subbuffers that are managed by the LTTng kernel module.

---

## 7. The tracefs / debugfs Interface

The userspace interface to tracepoints is through the **tracefs** virtual filesystem, typically mounted at `/sys/kernel/tracing` (or `/sys/kernel/debug/tracing` on older systems).

```
/sys/kernel/tracing/
├── available_events          # List of all registered tracepoints
├── enabled_events            # Currently enabled events
├── tracing_on                # Master switch (echo 1 > to enable)
├── trace                     # Human-readable ring buffer output
├── trace_pipe                # Streaming output (blocks until data)
├── trace_clock               # Clock source for timestamps
├── buffer_size_kb            # Per-CPU ring buffer size
├── current_tracer            # Active function tracer (nop, function, ...)
│
├── events/                   # Per-event control directory
│   ├── sched/
│   │   ├── sched_switch/
│   │   │   ├── enable        # Echo 1/0 to enable/disable
│   │   │   ├── filter        # Boolean filter expression
│   │   │   ├── trigger       # Actions when event fires
│   │   │   ├── format        # Field descriptions (type, offset, size)
│   │   │   ├── id            # Numeric event ID (for perf_event_open)
│   │   │   └── hist          # Histogram triggers
│   │   ├── sched_wakeup/
│   │   └── ...
│   ├── net/
│   ├── block/
│   ├── ext4/
│   └── ...
│
├── per_cpu/
│   ├── cpu0/
│   │   ├── trace             # Per-CPU trace buffer
│   │   ├── trace_pipe        # Per-CPU streaming output
│   │   └── stats             # Buffer statistics
│   └── cpu1/ ...
│
└── options/                  # Output formatting options
    ├── timestamp
    ├── sym-addr
    ├── stacktrace
    └── ...
```

### The `format` file — The Event Schema

The `format` file is essential — it describes the exact binary layout of each event in the ring buffer:

```
$ cat /sys/kernel/tracing/events/sched/sched_switch/format

name: sched_switch
ID: 316
format:
	field:unsigned short common_type;         offset:0;  size:2; signed:0;
	field:unsigned char common_flags;         offset:2;  size:1; signed:0;
	field:unsigned char common_preempt_count; offset:3;  size:1; signed:0;
	field:int common_pid;                     offset:4;  size:4; signed:1;

	field:char prev_comm[16];    offset:8;  size:16; signed:1;
	field:pid_t prev_pid;        offset:24; size:4;  signed:1;
	field:int prev_prio;         offset:28; size:4;  signed:1;
	field:long prev_state;       offset:32; size:8;  signed:1;
	field:char next_comm[16];    offset:40; size:16; signed:1;
	field:pid_t next_pid;        offset:56; size:4;  signed:1;
	field:int next_prio;         offset:60; size:4;  signed:1;

print fmt: "prev_comm=%s prev_pid=%d prev_prio=%d prev_state=%s%s ==> next_comm=%s next_pid=%d next_prio=%d",
           REC->prev_comm, REC->prev_pid, REC->prev_prio, ...
```

This format file is used by:
- `trace-cmd` to decode binary logs
- `perf` to decode raw samples
- BCC/bpftrace to know field offsets
- Any userspace tool reading the ring buffer

---

## 8. TRACE_EVENT Macro Deep Dive

Let's trace through a complete, real example to see exactly what code is generated.

The `sched_switch` tracepoint is defined in `include/trace/events/sched.h`:

```c
TRACE_EVENT(sched_switch,

    TP_PROTO(bool preempt,
             struct task_struct *prev,
             struct task_struct *next,
             unsigned int prev_state),

    TP_ARGS(preempt, prev, next, prev_state),

    TP_STRUCT__entry(
        __array(     char,   prev_comm,  TASK_COMM_LEN  )
        __field(     pid_t,  prev_pid                   )
        __field(     int,    prev_prio                  )
        __field(     long,   prev_state                 )
        __array(     char,   next_comm,  TASK_COMM_LEN  )
        __field(     pid_t,  next_pid                   )
        __field(     int,    next_prio                  )
    ),

    TP_fast_assign(
        memcpy(__entry->next_comm, next->comm, TASK_COMM_LEN);
        __entry->prev_pid   = prev->pid;
        __entry->prev_prio  = prev->prio;
        __entry->prev_state = __trace_sched_switch_state(preempt,
                                                          prev_state,
                                                          prev);
        memcpy(__entry->prev_comm, prev->comm, TASK_COMM_LEN);
        __entry->next_pid   = next->pid;
        __entry->next_prio  = next->prio;
    ),

    TP_printk("prev_comm=%s prev_pid=%d prev_prio=%d "
              "prev_state=%s%s ==> "
              "next_comm=%s next_pid=%d next_prio=%d",
              __entry->prev_comm, __entry->prev_pid, __entry->prev_prio,
              (__entry->prev_state & (TASK_REPORT_MAX - 1))
                ? __print_flags(__entry->prev_state & (TASK_REPORT_MAX-1),
                                "|", { TASK_INTERRUPTIBLE, "S" }, ...)
                : "R",
              __entry->prev_state & TASK_REPORT_MAX ? "+" : "",
              __entry->next_comm, __entry->next_pid, __entry->next_prio)
);
```

### What the macro generates (simplified):

**1. The raw entry structure:**

```c
struct trace_event_raw_sched_switch {
    struct trace_entry  ent;           /* 8 bytes: type, flags, pcount, pid */
    char                prev_comm[16]; /* TASK_COMM_LEN */
    pid_t               prev_pid;
    int                 prev_prio;
    long                prev_state;
    char                next_comm[16];
    pid_t               next_pid;
    int                 next_prio;
    char                __data[0];     /* Variable area (empty here) */
};
```

**2. The tracepoint declaration:**

```c
extern struct tracepoint __tracepoint_sched_switch;

static inline void trace_sched_switch(bool preempt,
                                       struct task_struct *prev,
                                       struct task_struct *next,
                                       unsigned int prev_state)
{
    if (static_key_false(&__tracepoint_sched_switch.key))
        __DO_TRACE(&__tracepoint_sched_switch,
                   TP_PROTO(bool preempt, ...),
                   TP_ARGS(preempt, prev, next, prev_state),
                   TP_CONDITION(1), 0);
    if (IS_ENABLED(CONFIG_LOCKDEP) && (cpu_online(raw_smp_processor_id())))
        rcu_read_lock_sched_notrace();
}
```

**3. The probe function (called by ftrace):**

```c
static void trace_event_raw_event_sched_switch(void *__data,
                                                bool preempt,
                                                struct task_struct *prev,
                                                struct task_struct *next,
                                                unsigned int prev_state)
{
    struct trace_event_file *trace_file = __data;
    struct trace_event_raw_sched_switch *entry;
    struct trace_event_buffer fbuffer;
    int __data_size = 0;  /* No dynamic fields */

    /* Check event-level filters */
    if (trace_trigger_soft_disabled(trace_file))
        return;

    /* Allocate space in ring buffer */
    entry = trace_event_buffer_reserve(&fbuffer, trace_file,
                                        sizeof(*entry) + __data_size);
    if (!entry)
        return;

    /* TP_fast_assign expansion: fill fields */
    memcpy(__entry->next_comm, next->comm, TASK_COMM_LEN);
    __entry->prev_pid   = prev->pid;
    __entry->prev_prio  = prev->prio;
    __entry->prev_state = __trace_sched_switch_state(preempt, prev_state, prev);
    memcpy(__entry->prev_comm, prev->comm, TASK_COMM_LEN);
    __entry->next_pid   = next->pid;
    __entry->next_prio  = next->prio;

    /* Commit to ring buffer */
    trace_event_buffer_commit(&fbuffer);
}
```

**4. The field descriptors (used by `format` file and eBPF BTF):**

```c
static struct btf_field_info trace_event_fields_sched_switch[] = {
    { .type = TRACE_FIELD_PADDING, .offset = 0, .size = 8 },  /* common */
    { .name = "prev_comm", .type = TRACE_FIELD_CHAR_ARR,
      .offset = offsetof(struct trace_event_raw_sched_switch, prev_comm) },
    { .name = "prev_pid", .type = TRACE_FIELD_INT,
      .offset = offsetof(struct trace_event_raw_sched_switch, prev_pid) },
    /* ... */
};
```

### The `__field`, `__array`, `__string`, `__dynamic_array` helpers

These are sub-macros that expand differently depending on context:

```c
/* __field(type, name) expands to: */
// In TP_STRUCT__entry:   type name;
// In TP_fast_assign:     (used as __entry->name = ...)
// In format descriptor:  .type=FIELD_TYPE, .size=sizeof(type), ...

/* __array(type, name, len) expands to: */
// In TP_STRUCT__entry:   type name[len];

/* __string(name, src) — variable length string */
// In TP_STRUCT__entry:   __data_loc char[] name;  (offset+len word)
// In TP_fast_assign:     __assign_str(name, src)
//   → memcpy into dynamic data area, store offset in __data_loc field
// In TP_printk:          __get_str(name)
//   → (char *)((char *)__entry + (__entry->name & 0xffff))

/* __dynamic_array(type, name, len_expr) — runtime-sized array */
// Allocates len_expr * sizeof(type) bytes in the dynamic area
// __get_dynamic_array(name) retrieves the pointer
// __get_dynamic_array_len(name) retrieves the length
```

The distinction between `__array` (fixed at compile time) and `__dynamic_array` (determined at trace time) is critical for variable-length events like network packet traces.

---

## 9. Writing Your Own Kernel Tracepoint in C

Here is a complete, working example of adding a tracepoint to a kernel module.

### File structure:

```
my_module/
├── my_module.c
├── trace_my_module.h    ← tracepoint definitions
└── Makefile
```

### Step 1: `trace_my_module.h` — Define the tracepoint

```c
/* trace_my_module.h */
#undef TRACE_SYSTEM
#define TRACE_SYSTEM my_module

/* Guard against multiple inclusion */
#if !defined(_TRACE_MY_MODULE_H) || defined(TRACE_HEADER_MULTI_READ)
#define _TRACE_MY_MODULE_H

#include <linux/tracepoint.h>

/*
 * Tracepoint: my_module_request
 * Fired when our module processes a request.
 *
 * Args:
 *   @cpu:     CPU where request arrived
 *   @req_id:  64-bit request identifier
 *   @cmd:     command string (variable length)
 *   @size:    payload size in bytes
 */
TRACE_EVENT(my_module_request,

    TP_PROTO(int cpu, u64 req_id, const char *cmd, size_t size),

    TP_ARGS(cpu, req_id, cmd, size),

    TP_STRUCT__entry(
        __field(    int,    cpu             )
        __field(    u64,    req_id          )
        __string(   cmd,    cmd             )  /* variable-length */
        __field(    size_t, size            )
    ),

    TP_fast_assign(
        __entry->cpu    = cpu;
        __entry->req_id = req_id;
        __assign_str(cmd, cmd);              /* copies string into ring buffer */
        __entry->size   = size;
    ),

    TP_printk("cpu=%d req_id=%llu cmd=%s size=%zu",
              __entry->cpu,
              __entry->req_id,
              __get_str(cmd),              /* retrieves string from ring buffer */
              __entry->size)
);

/*
 * Tracepoint: my_module_complete
 * Fired when a request completes.
 */
TRACE_EVENT(my_module_complete,

    TP_PROTO(u64 req_id, int error, u64 latency_ns),

    TP_ARGS(req_id, error, latency_ns),

    TP_STRUCT__entry(
        __field(    u64,    req_id          )
        __field(    int,    error           )
        __field(    u64,    latency_ns      )
    ),

    TP_fast_assign(
        __entry->req_id     = req_id;
        __entry->error      = error;
        __entry->latency_ns = latency_ns;
    ),

    TP_printk("req_id=%llu error=%d latency_ns=%llu",
              __entry->req_id, __entry->error, __entry->latency_ns)
);

#endif /* _TRACE_MY_MODULE_H */

/* This must be outside the header guard! */
#include <trace/define_trace.h>
```

### Step 2: `my_module.c` — Use the tracepoints

```c
/* my_module.c */

/* IMPORTANT: Define this ONLY in ONE .c file in your module,
   before including the trace header. This triggers code generation. */
#define CREATE_TRACE_POINTS
#include "trace_my_module.h"

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>
#include <linux/ktime.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Your Name");
MODULE_DESCRIPTION("Tracepoint example module");

struct request {
    u64         id;
    const char *cmd;
    size_t      size;
};

static int process_request(struct request *req)
{
    ktime_t start = ktime_get();
    int cpu = raw_smp_processor_id();

    /* Fire the entry tracepoint */
    trace_my_module_request(cpu, req->id, req->cmd, req->size);

    /* Simulate processing */
    udelay(10);

    ktime_t end = ktime_get();
    u64 latency = ktime_to_ns(ktime_sub(end, start));

    /* Fire the completion tracepoint */
    trace_my_module_complete(req->id, 0, latency);

    return 0;
}

static int __init my_module_init(void)
{
    struct request req = { .id = 1, .cmd = "read", .size = 4096 };
    pr_info("my_module: loaded\n");
    process_request(&req);
    return 0;
}

static void __exit my_module_exit(void)
{
    pr_info("my_module: unloaded\n");
}

module_init(my_module_init);
module_exit(my_module_exit);
```

### Step 3: `Makefile`

```makefile
obj-m := my_module.o

# Tell the build system where to find our trace header
CFLAGS_my_module.o := -I$(src)

KDIR := /lib/modules/$(shell uname -r)/build
PWD  := $(shell pwd)

default:
	$(MAKE) -C $(KDIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean
```

### Step 4: Using it

```bash
# Build and load
make
sudo insmod my_module.ko

# Enable the tracepoints
echo 1 > /sys/kernel/tracing/events/my_module/enable

# Read output
cat /sys/kernel/tracing/trace

# Or use trace-cmd
trace-cmd record -e 'my_module:*' sleep 5
trace-cmd report

# Or use perf
perf record -e 'my_module:my_module_request' -a sleep 5
perf script

# Or use bpftrace
bpftrace -e 'tracepoint:my_module:my_module_request {
    printf("req_id=%llu cmd=%s size=%lu\n",
           args->req_id, str(args->cmd), args->size);
}'
```

### The `#define CREATE_TRACE_POINTS` trick

This macro must appear in **exactly one** `.c` file per tracepoint header. Without it, you get `extern` declarations only (no actual definitions). With it, the header generates all the code: struct definitions, probe functions, event class descriptors, the `struct tracepoint` instance itself.

If you include the trace header in multiple `.c` files without `CREATE_TRACE_POINTS`, you correctly get "undefined reference" link errors — the linker catches the missing definitions.

---

## 10. User-Space Tracepoints (USDT)

### 10.1 What Is USDT?

USDT (User Statically Defined Tracing) brings the same static tracepoint concept to userspace programs. Like kernel tracepoints, USDT probes:

- Are defined at compile time as NOP instructions
- Have near-zero overhead when no consumer is attached
- Provide structured, stable probe sites
- Can be activated at runtime without recompilation

The mechanism differs from kernel tracepoints because userspace can't use jump labels (no kernel text patching) and can't use RCU. Instead, USDT uses a simpler approach:

1. Probe site: a `NOP` instruction in the `.text` section
2. Probe description: an ELF note in the `.note.stapsdt` section
3. Activation: the kernel patches the NOP to a breakpoint (INT3) using `uprobes`

### 10.2 SystemTap SDT Mechanism

The `<sys/sdt.h>` header from SystemTap is the de-facto USDT implementation on Linux:

```c
#include <sys/sdt.h>

void process_request(int req_id, size_t size) {
    /* USDT probe: provider=myapp, name=request_start */
    DTRACE_PROBE2(myapp, request_start, req_id, size);

    /* or equivalently: */
    STAP_PROBE2(myapp, request_start, req_id, size);

    do_work();

    DTRACE_PROBE1(myapp, request_done, req_id);
}
```

The `DTRACE_PROBE2(provider, name, arg1, arg2)` macro expands to:

```c
/* Simplified expansion of DTRACE_PROBE2 */
#define DTRACE_PROBE2(provider, name, arg1, arg2)               \
do {                                                              \
    asm volatile (                                                \
        /* 1. The probe NOP — assembler marks this as target */  \
        "990: nop\n\t"                                           \
        /* 2. Note section entry describing this probe */        \
        ".pushsection .note.stapsdt,\"?\",\"note\"\n\t"         \
        ".balign 4\n\t"                                          \
        ".4byte 992f-991f\n\t"        /* name length */          \
        ".4byte 994f-993f\n\t"        /* desc length */          \
        ".4byte 3\n\t"                /* NT_STAPSDT type */      \
        "991: .asciz \"stapsdt\"\n\t" /* name */                 \
        "992: .balign 4\n\t"                                     \
        "993: .8byte 990b\n\t"        /* probe addr (the NOP) */ \
        ".8byte _.stapsdt.base\n\t"   /* base address */         \
        ".8byte 0\n\t"                /* semaphore addr */       \
        ".asciz \"" #provider "\"\n\t"                           \
        ".asciz \"" #name "\"\n\t"                               \
        /* Argument descriptions: location, size, type */        \
        ".asciz \"-4@%0 -4@%1\"\n\t"  /* arg1 and arg2 */       \
        "994: .balign 4\n\t"                                     \
        ".popsection\n\t"                                        \
        : : "nor"(arg1), "nor"(arg2)                             \
    );                                                            \
} while (0)
```

This inline assembly does two things:
1. Emits a `NOP` at the probe site in `.text`
2. Emits a note entry in `.note.stapsdt` describing the probe location and argument types

### 10.3 ELF Note Sections and .probes

The `.note.stapsdt` section survives stripping (it's in a special note section, not debug info). Each probe entry contains:

```
ELF Note Entry Structure (.note.stapsdt):
══════════════════════════════════════════

Offset  Size  Field
──────  ────  ─────────────────────────────────────────
  0      4    namesz  = 8 (length of "stapsdt\0")
  4      4    descsz  = variable
  8      4    type    = 3 (NT_STAPSDT)
 12      8    "stapsdt\0" padded to 4-byte align
 20      8    probe_addr    — VA of the NOP in .text
 28      8    base_addr     — base address of object
 36      8    semaphore_addr — optional semaphore (0 if unused)
 44      var  provider\0   — null-terminated provider name
  ?      var  name\0       — null-terminated probe name
  ?      var  args\0       — argument format string
          e.g. "-4@%rdi -4@%rsi" means:
               arg0: 4-byte signed, in register rdi
               arg1: 4-byte signed, in register rsi
               Or:   "8@16(%rbp)" means 8-byte at rbp+16
```

**Argument format string decoding:**

```
Format: [±]SIZE@LOCATION

SIZE:      1, 2, 4, 8 bytes
SIGN:      + = unsigned, - = signed
LOCATION:  register:   %rdi, %rsi, %rdx, %rcx, %r8, %r9
           memory:     OFFSET(BASEREG)
           immediate:  $VALUE
           indirect:   *OFFSET(BASEREG)

Examples:
  -4@%edi        → int32 in edi register
   8@-24(%rbp)   → uint64 at [rbp-24] (stack variable)
  -8@(%rax)      → int64 at address in rax (pointer deref)
   4@$42         → constant value 42 (compile-time constant)
```

This format allows probe consumers to read argument values at runtime without DWARF — just registers and stack offsets.

### 10.4 Uprobes: The Kernel Side of USDT

When a tool wants to activate a USDT probe, it uses the kernel's **uprobe** mechanism:

```
USDT Activation Sequence:
═══════════════════════════

Tool (bpftrace/perf/BCC)
        │
        ├─ Read /proc/PID/maps to find library/binary VA
        ├─ Parse .note.stapsdt to find NOP address
        ├─ Calculate actual VA = load_bias + probe_addr
        │
        └─ inotify_add_watch / inotify on file, or:
           write "p:myprobe /path/to/binary:0xADDR" to
           /sys/kernel/tracing/uprobe_events
               │
               └─ Kernel uprobe_register()
                   │
                   ├─ Find all mappings of this inode at this offset
                   │
                   ├─ For each mapping:
                   │   get_user_pages() to pin the page
                   │   Copy original instruction bytes (the NOP)
                   │   Write INT3 (0xCC) at probe address
                   │   flush_dcache_page() + flush_icache_range()
                   │
                   └─ Add to uprobe_table hash
```

When the INT3 is hit at runtime:

```
Userspace process hits INT3
        │
        ├─ CPU → exception #BP
        ├─ Kernel: do_int3() / exc_int3()
        ├─ uprobe_pre_sstep_notifier() fires
        │   │
        │   ├─ Find uprobe in hash by (inode, offset)
        │   ├─ Call all registered handlers (BPF, perf, etc.)
        │   │   Handlers read argument values from user stack/regs
        │   │
        │   └─ Execute original instruction via single-step
        │       (place original bytes in an XOL "execute out of line" area,
        │        set trap flag, resume at XOL area)
        │
        ├─ CPU executes original instruction (NOP), hits TF trap
        ├─ uprobe_post_sstep_notifier() 
        ├─ Resume at original address + instruction_length
        └─ Return to userspace, application continues
```

The **XOL (eXecute Out of Line)** mechanism is crucial: rather than executing the original instruction at its original location (which would require unpatching and repatching), the kernel copies it to a private "XOL area" in the process's address space and sets the CPU's trap flag (TF) to single-step. This avoids a race condition where two threads could hit the probe simultaneously during unpatching.

---

## 11. Implementing USDT in C

### Full example with semaphores

```c
/* my_app.c */
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <sys/sdt.h>   /* from systemtap-sdt-dev package */

/*
 * Optional: USDT semaphore.
 * When a consumer attaches, the kernel increments this counter.
 * We can check it to skip expensive argument preparation.
 */
volatile unsigned short my_probe_semaphore __attribute__((unused));
#define MY_PROBE_ENABLED (my_probe_semaphore > 0)

struct request {
    uint64_t    id;
    const char *path;
    size_t      size;
    int         flags;
};

/*
 * Expensive argument preparation — only do it if a probe is attached.
 * Without semaphores, the arguments are always evaluated (but the NOP
 * is still cheap since they're just register loads).
 */
static char *build_path_copy(const char *path) {
    /* Expensive: copies and normalizes the path */
    return strdup(path);
}

void handle_request(struct request *req) {
    /* Simple probe — args are cheap (just passing existing values) */
    DTRACE_PROBE3(myapp, request_start,
                  req->id,
                  req->size,
                  req->flags);

    /* Semaphore-guarded probe — args are expensive to prepare */
    if (MY_PROBE_ENABLED) {
        char *normalized = build_path_copy(req->path);
        DTRACE_PROBE2(myapp, request_path,
                      req->id,
                      normalized);
        free(normalized);
    }

    /* ... actual work ... */

    DTRACE_PROBE2(myapp, request_done, req->id, 0 /* error */);
}

int main(void) {
    struct request reqs[] = {
        { 1, "/etc/passwd",  256, O_RDONLY },
        { 2, "/tmp/test",   4096, O_RDWR   },
        { 3, "/dev/null",      0, O_WRONLY  },
    };

    for (int i = 0; i < 3; i++)
        handle_request(&reqs[i]);

    return 0;
}
```

Compile with:

```bash
gcc -o my_app my_app.c
```

No special flags needed! The probes are entirely in inline assembly.

### Inspecting the ELF notes:

```bash
# List all USDT probes in a binary
readelf -n my_app | grep -A4 "stapsdt"

# Or use the dedicated tool from systemtap:
stap -L 'process("./my_app").mark("*")'

# Using bpftrace:
bpftrace -l 'usdt:./my_app:*'

# Output:
usdt:./my_app:myapp:request_start
usdt:./my_app:myapp:request_path
usdt:./my_app:myapp:request_done
```

### Consuming USDT probes:

```bash
# bpftrace
bpftrace -e '
usdt:/path/to/my_app:myapp:request_start {
    printf("request %lld, size=%lld, flags=%d\n",
           arg0, arg1, arg2);
}
usdt:/path/to/my_app:myapp:request_done {
    printf("done %lld, error=%d\n", arg0, arg1);
}
'

# perf
perf buildid-cache --add my_app
perf record -e 'sdt_myapp:request_start' -p $(pgrep my_app) sleep 10
perf script

# SystemTap
stap -e '
probe process("my_app").mark("request_start") {
    printf("req_id=%d size=%d\n", $arg1, $arg2);
}
'
```

### USDT with libstapsdt (runtime probe creation)

You can also add USDT probes at runtime (useful for interpreted languages — Node.js, Python, Ruby):

```c
/* Runtime USDT with libstapsdt */
#include <stapsdt.h>

int main(void) {
    SDTProvider_t *provider = sdtProviderInit("myprovider");

    /* Create probes at runtime */
    SDTProbe_t *probe = sdtProviderAddProbe(provider, "myprobe",
                                             2,       /* 2 arguments */
                                             uint64,  /* arg1 type */
                                             uint64); /* arg2 type */

    /* "Load" the provider: patches ELF notes into a temp shared lib,
       maps it into our address space, making probes discoverable */
    if (sdtProviderLoad(provider) != 0) {
        fprintf(stderr, "Failed to load provider\n");
        return 1;
    }

    /* Fire probe */
    sdtProbeFireArgs(probe, 42ULL, 1024ULL);

    sdtProviderUnload(provider);
    sdtProviderDestroy(provider);
    return 0;
}
```

---

## 12. Tracepoints in Rust

### 12.1 Kernel Tracepoints via rust/kernel Crate

The Linux kernel's Rust support (merged in 6.1) provides abstractions for tracepoints. The `rust/kernel` crate is the canonical interface:

```rust
// In a Rust kernel module: src/my_module.rs

use kernel::prelude::*;
use kernel::trace;

// Declare use of a kernel tracepoint defined in C headers
// (Rust kernel modules can call tracepoints defined anywhere in the kernel)
kernel::module! {
    type: MyModule,
    name: "my_module",
    author: "Author",
    description: "Rust tracepoint example",
    license: "GPL",
}

struct MyModule;

impl kernel::Module for MyModule {
    fn init(_module: &'static ThisModule) -> Result<Self> {
        // Call a tracepoint defined elsewhere in the kernel
        // trace_sched_switch equivalent via the trace! macro
        pr_info!("Rust module loaded\n");
        Ok(MyModule)
    }
}
```

For custom tracepoints in Rust kernel modules, the current approach uses FFI bindings to the C tracepoint headers. Since `TRACE_EVENT` is a macro system, Rust calls the generated C inline functions:

```rust
// Calling a C-defined tracepoint from Rust
// The bindgen-generated bindings expose the trace_* inline functions

extern "C" {
    // Generated by bindgen from the C tracepoint headers
    fn trace_my_module_request(cpu: i32, req_id: u64, cmd: *const i8, size: usize);
    fn trace_my_module_complete(req_id: u64, error: i32, latency_ns: u64);
}

fn process_request(req_id: u64, cmd: &CStr, size: usize) {
    unsafe {
        trace_my_module_request(
            // raw_smp_processor_id() equivalent
            kernel::cpu::current(),
            req_id,
            cmd.as_ptr(),
            size,
        );
    }

    // ... do work ...

    unsafe {
        trace_my_module_complete(req_id, 0, 12345);
    }
}
```

The Rust kernel tracepoint story is evolving. The longer-term plan involves procedural macros that generate the equivalent of `TRACE_EVENT` in Rust:

```rust
// Future API (proposed, not yet merged):
use kernel::trace_event;

trace_event! {
    name: my_module_request,
    system: my_module,
    proto: (cpu: i32, req_id: u64, size: usize),
    struct_entry: {
        cpu: i32,
        req_id: u64,
        size: usize,
    },
    assign: |entry, cpu, req_id, size| {
        entry.cpu = cpu;
        entry.req_id = req_id;
        entry.size = size;
    },
    printk: "cpu={cpu} req_id={req_id} size={size}",
}
```

### 12.2 USDT in Rust with probe-rs / tracing crate

For userspace Rust programs, there are several approaches to USDT:

#### Option 1: `probe` crate (static USDT via inline asm)

```toml
# Cargo.toml
[dependencies]
probe = "0.5"
```

```rust
use probe::probe;

fn handle_request(req_id: u64, size: usize) {
    // Fire USDT probe: provider=myapp, name=request_start
    // args are passed as usizes
    probe!(myapp, request_start, req_id, size as u64);

    // ... work ...

    probe!(myapp, request_done, req_id, 0u64 /* error */);
}
```

The `probe!` macro expands to inline assembly similar to `DTRACE_PROBE`:

```rust
// Simplified expansion of probe!(myapp, request_start, arg0, arg1)
unsafe {
    core::arch::asm!(
        "990: nop",
        ".pushsection .note.stapsdt,\"?\",\"note\"",
        // ... note section content ...
        ".popsection",
        in(reg) arg0,
        in(reg) arg1,
        options(nostack)
    );
}
```

#### Option 2: `tracing` crate + USDT backend

The `tracing` crate (Tokio's structured logging/tracing library) can be configured with a USDT subscriber:

```toml
[dependencies]
tracing = "0.1"
tracing-subscriber = "0.3"
# For USDT backend:
tracing-stap = "0.1"   # hypothetical — actual crate names vary
```

```rust
use tracing::{instrument, event, Level, span};

#[instrument(fields(req_id = req_id, size = size))]
fn handle_request(req_id: u64, size: usize) {
    event!(Level::INFO, req_id, size, "request started");
    
    let span = span!(Level::TRACE, "process", req_id);
    let _guard = span.enter();
    
    // ... work ...
    
    event!(Level::INFO, req_id, status = 0, "request complete");
}

fn main() {
    // Install a USDT subscriber that routes tracing events to USDT probes
    let subscriber = tracing_subscriber::registry()
        .with(tracing_stap::UsdtLayer::new("myapp"));
    tracing::subscriber::set_global_default(subscriber).unwrap();

    handle_request(1, 4096);
}
```

#### Option 3: Direct inline asm for maximum control

```rust
#[macro_export]
macro_rules! usdt_probe {
    ($provider:ident, $name:ident) => {
        unsafe {
            core::arch::asm!(
                concat!("990: nop\n\t",
                        ".pushsection .note.stapsdt,\"?\",\"note\"\n\t",
                        ".balign 4\n\t",
                        ".4byte 8\n\t",          // namesz
                        ".4byte 24\n\t",         // descsz (no args)
                        ".4byte 3\n\t",          // NT_STAPSDT
                        ".ascii \"stapsdt\"\n\t",
                        ".byte 0\n\t",
                        ".balign 4\n\t",
                        ".8byte 990b\n\t",       // probe addr
                        ".8byte 0\n\t",          // base
                        ".8byte 0\n\t",          // semaphore
                        ".asciz \"", stringify!($provider), "\"\n\t",
                        ".asciz \"", stringify!($name), "\"\n\t",
                        ".asciz \"\"\n\t",       // no args
                        ".balign 4\n\t",
                        ".popsection"),
                options(nomem, nostack, preserves_flags)
            );
        }
    };
    ($provider:ident, $name:ident, $arg0:expr) => {
        unsafe {
            let _a0 = $arg0 as u64;
            core::arch::asm!(
                "990: nop",
                // Note section with arg descriptor "-8@{0}"
                // ...
                in(reg) _a0,
                options(nomem, nostack, preserves_flags)
            );
        }
    };
}

fn process() {
    usdt_probe!(myapp, process_start);
    // work
    usdt_probe!(myapp, process_done);
}
```

### 12.3 eBPF Tracepoint Consumer in Rust (Aya)

Aya is the premier pure-Rust eBPF framework. It lets you write both the eBPF program (running in kernel) and the userspace loader/reader in Rust.

#### The eBPF program (runs in kernel):

```rust
// src/bpf/tracepoint_prog.rs
// This compiles to BPF bytecode

#![no_std]
#![no_main]

use aya_bpf::{
    macros::tracepoint,
    programs::TracePointContext,
    helpers::bpf_get_current_pid_tgid,
};
use aya_log_ebpf::info;

// Context struct matching sched_switch's format file layout
// Must match the kernel's trace_event_raw_sched_switch exactly
#[repr(C)]
pub struct SchedSwitchArgs {
    // Common fields (8 bytes total: type[2], flags[1], preempt_count[1], pid[4])
    common_type:          u16,
    common_flags:         u8,
    common_preempt_count: u8,
    common_pid:           i32,
    // Event-specific fields
    prev_comm:  [u8; 16],
    prev_pid:   i32,
    prev_prio:  i32,
    prev_state: i64,
    next_comm:  [u8; 16],
    next_pid:   i32,
    next_prio:  i32,
}

#[tracepoint(name = "sched_switch", category = "sched")]
pub fn sched_switch_handler(ctx: TracePointContext) -> i32 {
    match unsafe { try_sched_switch(&ctx) } {
        Ok(ret) => ret,
        Err(_)  => 1,
    }
}

unsafe fn try_sched_switch(ctx: &TracePointContext) -> Result<i32, i64> {
    // Read the tracepoint args — Aya generates safe accessors
    let prev_pid = ctx.read_at::<i32>(24)?;  // offset of prev_pid
    let next_pid = ctx.read_at::<i32>(56)?;  // offset of next_pid

    let prev_comm_ptr = ctx.as_ptr().add(8) as *const [u8; 16];
    let prev_comm = core::ptr::read_unaligned(prev_comm_ptr);
    let next_comm_ptr = ctx.as_ptr().add(40) as *const [u8; 16];
    let next_comm = core::ptr::read_unaligned(next_comm_ptr);

    info!(ctx, "switch: {} ({}) -> {} ({})",
          core::str::from_utf8_unchecked(&prev_comm),
          prev_pid,
          core::str::from_utf8_unchecked(&next_comm),
          next_pid);

    Ok(0)
}

#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! { loop {} }
```

#### The userspace loader (also Rust):

```rust
// src/main.rs
use aya::{Bpf, include_bytes_aligned, programs::TracePoint};
use aya_log::BpfLogger;
use tokio::signal;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    // Load the compiled BPF object
    let mut bpf = Bpf::load(include_bytes_aligned!(
        "../../target/bpfel-unknown-none/release/tracepoint_prog"
    ))?;

    // Initialize BPF logging
    BpfLogger::init(&mut bpf)?;

    // Get the tracepoint program
    let program: &mut TracePoint = bpf
        .program_mut("sched_switch_handler")
        .unwrap()
        .try_into()?;

    // Load (verify) and attach to sched:sched_switch
    program.load()?;
    program.attach("sched", "sched_switch")?;

    println!("Tracing sched_switch... Press Ctrl-C to stop");
    signal::ctrl_c().await?;

    Ok(())
}
```

---

## 13. eBPF and Tracepoints: Deep Integration

### 13.1 BPF_PROG_TYPE_TRACEPOINT

When an eBPF program attaches to a tracepoint, the attachment mechanism is:

```
Attachment flow:
════════════════

bpf(BPF_PROG_LOAD, ...)
    │  type = BPF_PROG_TYPE_TRACEPOINT
    │  insns = <BPF bytecode>
    └─ Returns prog_fd

perf_event_open(...)
    │  attr.type   = PERF_TYPE_TRACEPOINT
    │  attr.config = event_id  (from tracefs id file)
    └─ Returns perf_fd

ioctl(perf_fd, PERF_EVENT_IOC_SET_BPF, prog_fd)
    └─ Attaches prog_fd to perf_fd

ioctl(perf_fd, PERF_EVENT_IOC_ENABLE, 0)
    └─ Activates — now tracepoint fires BPF program
```

Modern libbpf and Aya hide this complexity behind `bpf_program__attach_tracepoint()`.

### 13.2 Raw Tracepoints

`BPF_PROG_TYPE_RAW_TRACEPOINT` was added in Linux 4.17 to reduce overhead further.

**Regular tracepoint**: The tracepoint fires → `trace_event_raw_event_*()` runs → packages arguments into a struct → BPF program reads from the struct.

**Raw tracepoint**: The tracepoint fires → BPF program runs immediately with the raw arguments (before any packaging).

```c
/* Regular tracepoint BPF — receives fully formatted struct */
SEC("tracepoint/sched/sched_switch")
int prog(struct trace_event_raw_sched_switch *ctx) {
    /* ctx->prev_pid etc. — already typed */
    return 0;
}

/* Raw tracepoint BPF — receives raw args array */
SEC("raw_tracepoint/sched_switch")
int prog(struct bpf_raw_tracepoint_args *ctx) {
    /* ctx->args[0] = preempt (bool) */
    /* ctx->args[1] = prev task_struct* */
    /* ctx->args[2] = next task_struct* */
    /* ctx->args[3] = prev_state */

    struct task_struct *prev = (struct task_struct *)ctx->args[1];
    pid_t prev_pid;
    /* Use BPF CO-RE (Compile Once - Run Everywhere) to read: */
    bpf_core_read(&prev_pid, sizeof(prev_pid), &prev->pid);
    return 0;
}
```

Raw tracepoints are faster because:
1. No struct allocation for the formatted event
2. No field copying
3. Direct access to kernel data structures (with BTF safety)

The tradeoff: you must know the raw argument layout, which can change between kernel versions. BTF CO-RE mitigates this.

### 13.3 BTF and Type Information

**BTF (BPF Type Format)** is a compact type system embedded in the kernel and BPF objects. It enables:

1. **CO-RE (Compile Once – Run Everywhere)**: BPF programs compiled against one kernel version can run on another by using BTF to find the correct field offsets at load time.

2. **Type-safe field access**: The verifier can validate that your BPF program only reads valid fields.

3. **Automatic format file generation**: The `format` file in tracefs is generated from BTF.

```
BTF for sched_switch tracepoint:
══════════════════════════════════

BTF type entry (simplified):
  struct trace_event_raw_sched_switch {
    .members = [
      { "common_type",          type=u16,   offset=0  },
      { "common_flags",         type=u8,    offset=16 },
      { "common_preempt_count", type=u8,    offset=24 },
      { "common_pid",           type=i32,   offset=32 },
      { "prev_comm",            type=char[16], offset=64 },
      { "prev_pid",             type=i32,   offset=192 },
      ...
    ]
  }
```

With CO-RE, offset 192 is not hard-coded — the BPF loader reads the BTF at runtime and relocates the field access to the correct offset for the running kernel.

---

## 14. Performance Model and Overhead Analysis

Understanding the exact performance characteristics of tracepoints is essential for using them in production.

### Disabled tracepoint overhead

```
Mechanism                    Overhead per call site
─────────────────────────────────────────────────────
Jump label NOP (disabled)    ~0 cycles (eliminated by decoder)
                             ~0.2ns measured on x86
Naive if(atomic_read(key))   ~3-5 cycles (comparison + branch)
                             ~1.5-2.5ns

Advantage of jump labels:   ~5-15x faster disabled path
```

### Enabled tracepoint overhead

The overhead of a fired tracepoint depends on what's attached:

```
Component                            Typical Cost
──────────────────────────────────   ────────────
Jump label branch                    ~1 cycle
RCU read lock (sched version)        ~2-5 cycles
Load probe function list             ~2 cycles (L1 cache hit)
Function call overhead               ~3 cycles
Ring buffer reservation (ftrace)     ~15-30 cycles (per-CPU local ops)
Memory copy (TP_fast_assign)         ~10-50 cycles (data dependent)
Ring buffer commit                   ~5 cycles
RCU read unlock                      ~2 cycles
─────────────────────────────────
Total (typical, ftrace enabled):     ~40-100 cycles ≈ 15-40ns

eBPF program execution:              ~50-500 cycles additional
  (depends on program complexity)
```

### Throughput limits

```
Ring buffer throughput:
  Per-CPU ring buffer write: ~10-50M events/second per CPU
  perf ring buffer:          ~5-20M events/second per CPU
  eBPF (map update):         ~5-10M events/second per CPU

Memory bandwidth:
  Each sched_switch event:  ~64 bytes
  At 10M events/sec:        ~640 MB/s per CPU — this is the real bottleneck
  Modern DRAM:              ~50 GB/s
  L3 cache:                 ~300 GB/s
  → For in-cache working set (small ring buffer): nearly unlimited
```

### Sizing the ring buffer

```bash
# Check current size (KB per CPU)
cat /sys/kernel/tracing/buffer_size_kb

# Set to 64MB per CPU
echo 65536 > /sys/kernel/tracing/buffer_size_kb

# Check stats (dropped events = ring buffer too small)
cat /sys/kernel/tracing/per_cpu/cpu0/stats
```

---

## 15. Tracing Data Flow End-to-End

```
Complete data flow for a traced sched_switch event:

CPU 3: process A → process B
            │
    ┌───────▼───────────────────────────────────┐
    │  __schedule() calls trace_sched_switch()  │
    └───────┬───────────────────────────────────┘
            │
    ┌───────▼────────────────────────────────────────────────────┐
    │  static_key_false() → patched to JMP (enabled state)       │
    └───────┬────────────────────────────────────────────────────┘
            │
    ┌───────▼────────────────────────────────────────────────────┐
    │  rcu_read_lock_sched_notrace()                              │
    │  it_func_ptr = rcu_dereference(tp->funcs)                  │
    └───────┬────────────────────────────────────────────────────┘
            │
            ├─────────────────────┬──────────────────────────────┐
            │                     │                              │
    ┌───────▼──────┐    ┌─────────▼────────┐    ┌───────────────▼──────┐
    │ ftrace probe │    │  perf probe       │    │  eBPF probe           │
    │              │    │                  │    │                       │
    │ reserve 72B  │    │ perf_raw_record  │    │ BPF prog executes     │
    │ in ring buf  │    │ → perf ring buf  │    │ (verified bytecode)   │
    │ fill fields  │    │ → wake reader if │    │ reads ctx fields      │
    │ commit entry │    │   watermark hit  │    │ updates BPF maps      │
    └───────┬──────┘    └─────────┬────────┘    └───────────────┬──────┘
            │                     │                              │
    ┌───────▼────────────────────────────────────────────────────▼──────┐
    │  rcu_read_unlock_sched_notrace()    ← back to normal execution    │
    └───────────────────────────────────────────────────────────────────┘
            │
    User space reads data:
            │
    ┌───────▼──────────┐    ┌──────────────────┐    ┌──────────────────┐
    │ cat trace_pipe   │    │ perf read mmap   │    │ bpf map lookup   │
    │ trace-cmd report │    │ perf script      │    │ bpftrace output  │
    │                  │    │                  │    │                  │
    │ ASCII output:    │    │ struct sample:   │    │ Aggregated stats │
    │  <...>-PID [003] │    │  time, cpu, raw  │    │ or per-event log │
    │  prev_comm=A ... │    │  → decode via    │    │                  │
    │                  │    │    format file   │    │                  │
    └──────────────────┘    └──────────────────┘    └──────────────────┘
```

---

## 16. Filtering and Triggering

### Filters (predicate pushdown)

Tracepoint filtering pushes predicates into the kernel, so events that don't match are not copied to the ring buffer at all.

```bash
# Filter sched_switch to only show events involving PID 1234
echo 'prev_pid == 1234 || next_pid == 1234' > \
    /sys/kernel/tracing/events/sched/sched_switch/filter

# String comparison (note: uses glob, not regex)
echo 'prev_comm == "nginx"' > \
    /sys/kernel/tracing/events/sched/sched_switch/filter

# Numeric comparisons, bitwise, logical
echo 'prev_state & 1 && prev_prio < 100' > \
    /sys/kernel/tracing/events/sched/sched_switch/filter

# Clear filter
echo '0' > /sys/kernel/tracing/events/sched/sched_switch/filter
```

Filter evaluation happens inside `trace_event_raw_event_*()` via the **filter engine**. The engine compiles filter strings into a bytecode-like representation (`prog_entry` array) that is evaluated against each event's raw struct.

The filter types and operators:

```
Field types that can be filtered:
  integer types: ==, !=, <, <=, >, >=, &
  strings:       ==, != (glob comparison with * wildcard)
  bitmask:       &  (bitwise AND, tests if bits set)

Logical operators:
  &&   (AND)
  ||   (OR)
  !    (NOT — only at field level, not compound)

Examples:
  "size > 4096 && size < 65536"
  "comm == \"nginx\" || comm == \"apache\""
  "flags & 0x04"    (test specific bit)
```

### Triggers

Triggers are **actions** that fire when an event occurs, optionally with a filter condition. Configured via the `trigger` file:

```bash
# Syntax: ACTION[:ARG][/FILTER][if CONDITION]

# Dump a stack trace when sys_enter fires with id=2 (open syscall)
echo 'stacktrace if id == 2' > \
    /sys/kernel/tracing/events/syscalls/sys_enter/trigger

# Snapshot the ring buffer when a specific process calls exit
echo 'snapshot if pid == 1234' > \
    /sys/kernel/tracing/events/syscalls/sys_exit/trigger

# Enable/disable another event as a trigger action
echo 'enable_event:net:net_dev_xmit' > \
    /sys/kernel/tracing/events/sched/sched_switch/trigger

# Count occurrences with hist trigger (histogram)
echo 'hist:key=prev_comm:val=hitcount' > \
    /sys/kernel/tracing/events/sched/sched_switch/trigger

# Read histogram
cat /sys/kernel/tracing/events/sched/sched_switch/hist
```

### Histogram triggers (hist triggers)

Hist triggers are powerful in-kernel aggregation:

```bash
# Count task switches per comm (process name)
echo 'hist:key=prev_comm:val=hitcount:sort=hitcount:size=64' > \
    /sys/kernel/tracing/events/sched/sched_switch/trigger

# Measure scheduling latency (wakeup to switch)
# This requires two correlated events using variables
echo 'hist:key=pid:val=ts=common_timestamp.usecs' > \
    /sys/kernel/tracing/events/sched/sched_wakeup/trigger

echo 'hist:key=next_pid:val=lat=common_timestamp.usecs-$ts:
      onmatch(sched.sched_wakeup).trace(sched_lat,$lat)' > \
    /sys/kernel/tracing/events/sched/sched_switch/trigger
```

---

## 17. Tracepoint vs Kprobe vs Uprobe vs Ftrace

Understanding when to choose each mechanism:

```
COMPARISON TABLE
════════════════════════════════════════════════════════════════════════

                    Tracepoint    Kprobe      Uprobe      Ftrace fn
─────────────────   ──────────    ──────────  ──────────  ──────────
ABI stability       STABLE        UNSTABLE    UNSTABLE    STABLE*
Placement           Source-time   Runtime     Runtime     Per-function
Requires source     YES           NO          NO          NO
Arguments           Typed struct  Raw regs    Raw regs    Raw regs
  + DWARF/BTF       (native)      optional    optional    no
Disabled overhead   ~0 (NOP)      ~0*         0           0**
Enabled overhead    40-100 ns     ~100 ns     ~1 µs       ~30 ns
Multi-consumer      YES           YES         YES         NO (single)
Works on any code   NO            YES         YES         YES
Works in NMI ctx    YES           Maybe       NO          YES
String args         YES (field)   NO (raw)    NO (raw)    NO
Production safe     YES           Careful     YES         Careful

* kprobes: disabled = breakpoint removed = 0 overhead, but placement is by
  address which changes between kernel versions unless using symbols
** ftrace: disabled = NOP (also uses jump labels for function tracer)
```

### Decision flowchart:

```
Do you need to observe a specific, known event
with a stable interface? (scheduler, syscall, network...)
        │
        YES ──► Use TRACEPOINT
        │       - Zero overhead when disabled
        │       - Structured typed data
        │       - Stable across kernel versions
        │
        NO
        │
Is it in the kernel?
        │
        YES ──► Is it a function entry/return?
        │               │
        │               YES ──► Use KPROBE/KRETPROBE
        │               │       (or ftrace function tracer)
        │               │
        │               NO ──── Any address? Use KPROBE
        │
        NO (userspace)
        │
Does the binary have USDT probes?
        │
        YES ──► Use USDT/UPROBE with USDT
        │       - Stable, semaphore-guarded
        │       - Typed arguments
        │
        NO ──── Use UPROBE
                - Raw: need DWARF or manual offset
                - Higher overhead (~1 µs for INT3+singlestep)
```

---

## 18. Real-World Use Cases and Patterns

### Pattern 1: Latency Distribution

Measure latency between two tracepoints using eBPF:

```c
/* BPF program: measure sys_read latency */
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, u32);      /* pid */
    __type(value, u64);    /* timestamp */
} start SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 100);   /* 100 histogram buckets */
    __type(key, u32);
    __type(value, u64);
} latency_hist SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_read")
int sys_enter_read(struct trace_event_raw_sys_enter *ctx)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&start, &pid, &ts, BPF_ANY);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_read")
int sys_exit_read(struct trace_event_raw_sys_exit *ctx)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    u64 *tsp = bpf_map_lookup_elem(&start, &pid);
    if (!tsp) return 0;

    u64 latency_ns = bpf_ktime_get_ns() - *tsp;
    bpf_map_delete_elem(&start, &pid);

    /* Log2 bucketing: bucket = log2(latency_ns) */
    u32 bucket = 0;
    u64 lat = latency_ns;
    while (lat > 1 && bucket < 99) { lat >>= 1; bucket++; }

    u64 *count = bpf_map_lookup_elem(&latency_hist, &bucket);
    if (count) __sync_fetch_and_add(count, 1);

    return 0;
}
```

### Pattern 2: Syscall auditing with filters

```bash
# Watch all openat() calls by nginx, showing filename
echo 'tracepoint:syscalls:sys_enter_openat' | \
bpftrace -e '
tracepoint:syscalls:sys_enter_openat
/comm == "nginx"/ {
    printf("nginx openat: %s flags=%d\n",
           str(args->filename), args->flags);
}'
```

### Pattern 3: Network packet tracing

```bash
# Trace packets dropped by the kernel
bpftrace -e '
tracepoint:skb:kfree_skb {
    @drops[args->reason] = count();
}
interval:s:5 {
    print(@drops);
    clear(@drops);
}'
```

### Pattern 4: Memory allocation profiling

```bash
# Track kmalloc allocations by call site
bpftrace -e '
tracepoint:kmem:kmalloc {
    @allocs[args->call_site, args->bytes_req] = count();
    @bytes[args->call_site] = sum(args->bytes_req);
}
END {
    printf("Top allocations by site:\n");
    print(@bytes, 10);
}'
```

### Pattern 5: Scheduler runqueue latency (using trace-cmd)

```bash
# Record sched events and analyze with trace-cmd
trace-cmd record -e sched:sched_wakeup -e sched:sched_switch \
                 -e sched:sched_migrate_task \
                 -p nop sleep 10

# Analyze with kernelshark (GUI) or:
trace-cmd report | grep -E "sched_wakeup|sched_switch" | head -50
```

---

## 19. Security Considerations

### Who can attach to tracepoints?

```
Capability requirements:
════════════════════════

tracefs access:
  - Mounting tracefs: requires CAP_SYS_ADMIN or root
  - Reading /sys/kernel/tracing: group ownership = tracing
    (add user to 'tracing' group for non-root access)
  - Writing enable/filter files: CAP_SYS_ADMIN or tracing group

perf_event_open for tracepoints:
  - /proc/sys/kernel/perf_event_paranoid controls access:
    -1: No restrictions
     0: Allow user-space measurements (non-root)
     1: Allow kernel measurements (CAP_SYS_ADMIN or owner)
     2: Allow only self-profiling without CAP_PERFMON
     3: Deny all access to non-privileged users
  - CAP_PERFMON (Linux 5.8+): new capability just for perf/BPF

BPF programs on tracepoints:
  - Require CAP_BPF (Linux 5.8+) or CAP_SYS_ADMIN
  - Verifier prevents memory safety violations
  - No capability to modify kernel data via tracepoints
    (they are read-only observers by design)

USDT:
  - Activating USDT for your own process: no special caps
  - Activating USDT for another process: CAP_SYS_PTRACE
```

### Information disclosure

Tracepoints can expose sensitive information:

- `syscalls:sys_enter_openat` reveals filenames opened by all processes
- `sched:sched_switch` reveals all process names and PIDs
- `net:*` events can expose network traffic metadata
- eBPF programs can read arbitrary kernel memory (within verifier constraints)

The kernel prevents truly privileged information (encryption keys, raw packet payloads in some cases) from being exposed through careful tracepoint design.

### The verifier as a security boundary

For eBPF programs attached to tracepoints, the BPF verifier is the security boundary:

```
BPF Verifier Guarantees:
  - No unbounded loops (program always terminates)
  - No invalid memory accesses (bounds-checked)
  - No kernel function calls except allowed helpers
  - No modifying kernel structures (read-only access to context)
  - Stack size limited (512 bytes)
  - Instruction count limited (~1M instructions)
```

---

## 20. Complete ASCII Architecture Reference

### Full Linux Tracing Stack

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║                         LINUX TRACING ARCHITECTURE                           ║
╠═══════════════════════════════════════════════════════════════════════════════╣
║                                                                               ║
║  USER SPACE                                                                   ║
║  ┌──────────┐ ┌───────────┐ ┌──────────┐ ┌────────────┐ ┌────────────────┐  ║
║  │  perf(1) │ │ trace-cmd │ │bpftrace  │ │  LTTng ctl │ │  SystemTap     │  ║
║  └────┬─────┘ └─────┬─────┘ └─────┬────┘ └──────┬─────┘ └───────┬────────┘  ║
║       │              │              │              │               │           ║
║  ─────┼──────────────┼──────────────┼──────────────┼───────────────┼─────────  ║
║       │  SYSTEM      │    CALLS     │              │               │           ║
║  ─────┼──────────────┼──────────────┼──────────────┼───────────────┼─────────  ║
║       │              │              │              │               │           ║
║  KERNEL SPACE         │              │              │               │           ║
║       │              │              │              │               │           ║
║  ┌────▼──────────┐   │   ┌──────────▼──┐  ┌───────▼────┐  ┌──────▼───────┐  ║
║  │perf_event_open│   │   │  bpf(2)     │  │lttng-ctl.ko│  │ stap kernel  │  ║
║  └────┬──────────┘   │   └──────┬──────┘  └─────┬──────┘  └──────┬───────┘  ║
║       │              │          │                │                 │           ║
║  ┌────▼──────────────▼──────────▼────────────────▼─────────────────▼───────┐  ║
║  │                                                                          │  ║
║  │                    TRACEPOINT SUBSYSTEM CORE                             │  ║
║  │                                                                          │  ║
║  │  struct tracepoint {                                                     │  ║
║  │    .key   = static_key (jump label)                                      │  ║
║  │    .funcs = RCU-protected probe list ──→ [probe0|data0][probe1|data1]... │  ║
║  │  }                                          │           │                │  ║
║  │                                             │           │                │  ║
║  │  Probe slots:                               │           │                │  ║
║  │    [ftrace handler] ────────────────────────┘           │                │  ║
║  │    [perf handler]  ─────────────────────────────────────┘                │  ║
║  │    [BPF handler]   (via perf attachment)                                  │  ║
║  │    [LTTng handler]                                                        │  ║
║  │                                                                          │  ║
║  └──────────────────────────────────────────────────────────────────────────┘  ║
║           │                   │                  │               │              ║
║  ┌────────▼──────┐  ┌─────────▼────┐  ┌─────────▼──────┐  ┌────▼──────────┐  ║
║  │  ftrace ring  │  │  perf ring   │  │  BPF maps/     │  │  LTTng ring   │  ║
║  │  buffer       │  │  buffer      │  │  ringbuf       │  │  buffer       │  ║
║  │  (per-CPU)    │  │  (mmap'd)    │  │                │  │  (per-CPU)    │  ║
║  └───────────────┘  └──────────────┘  └────────────────┘  └───────────────┘  ║
║                                                                               ║
║  CALL SITES (examples):                                                       ║
║  kernel/sched/core.c:   trace_sched_switch(preempt, prev, next, state)       ║
║  net/core/dev.c:        trace_net_dev_xmit(skb, rc, dev, skb_len)            ║
║  mm/filemap.c:          trace_mm_filemap_add_to_page_cache(page)             ║
║  fs/open.c:             trace_do_sys_openat2(dfd, filename, op)              ║
╚═══════════════════════════════════════════════════════════════════════════════╝
```

### Jump Label State Machine

```
JUMP LABEL STATE MACHINE
═════════════════════════

Source (compile time):
  if (static_key_false(&tp->key)) { slow_path(); }
         │
         └─ Compiler emits NOP at this branch + entry in __jump_table

Runtime states:
               DISABLED                        ENABLED
               ─────────                       ───────
text:    NOP (0F 1F 44 00 00)         JMP rel32 (E9 xx xx xx xx)
         │                                     │
         │                                     └─ jumps to slow_path
         └─ falls through to next instr

state transitions:
  DISABLED ──[static_key_enable()]──► ENABLED
             │
             ├─ find all jump_table entries for this key
             ├─ for each: text_poke_bp(code, JMP, NOP)
             │   ├─ write INT3 to byte 0 (safe start)
             │   ├─ IPI flush
             │   ├─ write bytes 1-4
             │   └─ write byte 0 (JMP opcode)
             └─ all CPUs now take the JMP to slow_path

  ENABLED ──[static_key_disable()]──► DISABLED
            └─ text_poke_bp(code, NOP, JMP) — reverse process
```

### USDT ELF Layout

```
ELF BINARY WITH USDT PROBES
═════════════════════════════

ELF File:
┌─────────────────────────────────────────────────────────────────┐
│  ELF Header                                                      │
├─────────────────────────────────────────────────────────────────┤
│  .text section                                                   │
│  ...                                                             │
│  0x1234:  48 89 FE          mov rsi, rdi                        │
│  0x1237:  90                NOP  ◄── USDT probe site            │
│  0x1238:  48 8B 45 F8       mov rax, [rbp-8]                    │
│  ...                                                             │
├─────────────────────────────────────────────────────────────────┤
│  .note.stapsdt section                                           │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ namesz = 8                                               │   │
│  │ descsz = 56                                              │   │
│  │ type   = 3 (NT_STAPSDT)                                  │   │
│  │ name   = "stapsdt\0"                                     │   │
│  │ desc:                                                    │   │
│  │   probe_addr  = 0x1237  ← points to the NOP             │   │
│  │   base_addr   = 0x0     ← ASLR base (filled at load)    │   │
│  │   sem_addr    = 0x0     ← semaphore (if used)           │   │
│  │   provider    = "myapp\0"                                │   │
│  │   name        = "request_start\0"                       │   │
│  │   args        = "-8@%rdi -8@%rsi\0"                     │   │
│  └──────────────────────────────────────────────────────────┘   │
│  (more entries for each probe...)                                │
├─────────────────────────────────────────────────────────────────┤
│  .stapsdt.base section (1 byte, just provides a reference VA)    │
└─────────────────────────────────────────────────────────────────┘

At runtime when consumer attaches:
  Kernel uprobes: patch 0x1237 from NOP (0x90) to INT3 (0xCC)
  
  On INT3 hit:
    → probe handler reads rdi, rsi per the args format string
    → probe handler restores NOP into XOL area, single-steps
```

### Tracepoint Registration Timeline

```
TRACEPOINT LIFECYCLE
═════════════════════

Boot time:
  vmlinux link ──► __tracepoints section contains all struct tracepoint
  early init   ──► tracepoint_iter_reset() initializes iterators
  module load  ──► tracepoint_module_notify() registers module tracepoints

Consumer attach (e.g., echo 1 > events/sched/sched_switch/enable):
  
  time ──────────────────────────────────────────────────────────────►
  
  t0: echo 1 → ft_event_enable_disable()
      │
  t1: trace_array_set_clr_event() → event_enable_func()
      │
  t2: tracepoint_probe_register(tp, probe_fn, data)
      │
      ├── mutex_lock(tracepoints_mutex)
      ├── alloc new tracepoint_func[] array
      ├── rcu_assign_pointer(tp->funcs, new_array)     ← probes now "visible"
      │                                                   but key still disabled
      ├── synchronize_rcu()                             ← wait for old readers
      ├── static_key_enable(&tp->key)                  ← text_poke! NOP→JMP
      └── mutex_unlock(tracepoints_mutex)
      
  t3: ANY new call site execution hits the JMP (tracepoint fires)
  
  t4: Old call sites (between t2 and t3) may still see disabled state
      (they were in the NOP path when the IPI arrived)
      → This is safe: they miss this one firing, next time they'll catch it

Consumer detach (echo 0 > enable):
  
  t5: tracepoint_probe_unregister(tp, probe_fn, data)
      │
      ├── mutex_lock
      ├── [if last probe] static_key_disable(&tp->key) ← JMP→NOP
      ├── synchronize_rcu()                            ← wait for in-flight probes
      ├── rcu_assign_pointer(tp->funcs, new_array_without_probe)
      ├── synchronize_rcu()                            ← wait for readers of old ptr
      └── kfree(old_array)
  
  t6: Zero overhead state restored
```

---

## 21. Quick Reference Cheatsheet

### Enabling tracepoints

```bash
# List all available tracepoints
cat /sys/kernel/tracing/available_events | grep sched

# Enable a single event
echo 1 > /sys/kernel/tracing/events/sched/sched_switch/enable

# Enable all events in a subsystem
echo 1 > /sys/kernel/tracing/events/sched/enable

# Enable all events
echo 1 > /sys/kernel/tracing/events/enable

# Enable tracing (master switch)
echo 1 > /sys/kernel/tracing/tracing_on

# Read output
cat /sys/kernel/tracing/trace_pipe

# Disable and clear
echo 0 > /sys/kernel/tracing/tracing_on
echo 0 > /sys/kernel/tracing/events/enable
echo > /sys/kernel/tracing/trace
```

### trace-cmd

```bash
# Record all sched events for 5 seconds
trace-cmd record -e 'sched:*' sleep 5

# Record with a filter
trace-cmd record -e sched:sched_switch --filter 'prev_pid > 100' sleep 5

# Report
trace-cmd report trace.dat

# Live stream
trace-cmd stream -e sched:sched_switch
```

### bpftrace one-liners

```bash
# Count sched switches by process
bpftrace -e 'tracepoint:sched:sched_switch { @[args->prev_comm] = count(); }'

# Trace all syscalls by a specific PID
bpftrace -e 'tracepoint:syscalls:sys_enter_* /pid == 1234/ { @[probe] = count(); }'

# List syscall frequency
bpftrace -e 'tracepoint:raw_syscalls:sys_enter { @[comm] = count(); }'

# Measure block I/O latency
bpftrace -e '
tracepoint:block:block_rq_issue { @ts[args->dev, args->sector] = nsecs; }
tracepoint:block:block_rq_complete {
    $key = (args->dev, args->sector);
    @usecs = hist((nsecs - @ts[$key]) / 1000);
    delete(@ts[$key]);
}'
```

### perf

```bash
# List available tracepoints
perf list 2>&1 | grep Tracepoint

# Count sched switches for 5 seconds
perf stat -e sched:sched_switch -a sleep 5

# Record with sampling
perf record -e sched:sched_switch -a sleep 5
perf script

# With callchains
perf record -e sched:sched_switch -g -a sleep 5
perf report
```

### Key kernel source files

```
include/linux/tracepoint.h          Core tracepoint macros
include/linux/tracepoint-defs.h     struct tracepoint, tracepoint_func
include/trace/trace_events.h        TRACE_EVENT macro expansion passes
kernel/tracepoint.c                 tracepoint_probe_register/unregister
kernel/trace/trace_events.c         tracefs interface, event enable/disable
kernel/trace/ring_buffer.c          ftrace ring buffer implementation
kernel/events/core.c                perf_event_open, tracepoint integration
include/trace/events/sched.h        Scheduler tracepoint definitions
include/trace/events/net.h          Network tracepoint definitions
include/trace/events/block.h        Block I/O tracepoint definitions
include/trace/events/syscalls.h     Syscall tracepoint definitions
kernel/trace/bpf_trace.c            BPF tracepoint integration
kernel/trace/trace_uprobe.c         Uprobe/USDT implementation
```

### TRACE_EVENT helper macros reference

```c
/* In TP_STRUCT__entry: */
__field(type, name)                 Simple scalar field
__field_struct(type, name)          Struct field (copied by value)
__array(type, name, len)            Fixed-size array
__string(name, src)                 Variable-length string (NUL-terminated)
__dynamic_array(type, name, len)    Variable-length typed array
__bitmask(name, nr_bits)           Bitmask (for __print_flags display)
__sockaddr(name, len)              Socket address

/* In TP_fast_assign: */
__assign_str(name, src)            Copy string src → __string field
__assign_bitmask(name, src, nb)    Copy bitmask
__get_dynamic_array(name)          Pointer to dynamic array data
__get_dynamic_array_len(name)      Number of elements in dynamic array

/* In TP_printk: */
__get_str(name)                    Retrieve string from __string field
__get_bitmask(name)                Retrieve bitmask for printing
__print_flags(val, delim, flags)   Print set bits as strings
__print_symbolic(val, symbols)     Print value as symbolic name
__print_array(arr, len, el_size)   Format array contents
```

---

## Conclusion and Mental Model Summary

After working through this guide, your mental model for tracepoints should be:

**A tracepoint is a stable, statically-defined event emission point** with these invariants:

1. **Cost = 0 when disabled**: jump labels turn the entire call path into a single NOP, indistinguishable from no instrumentation at all.

2. **The producer/consumer separation**: call sites (producers) know nothing about who is consuming. They fire into a probe function list. Consumers (ftrace, perf, eBPF, LTTng) register probe functions dynamically.

3. **RCU is the synchronization primitive**: probe lists are RCU-protected for lock-free reading at call sites, with safe modification on the registration side.

4. **TRACE_EVENT is a macro machine**: one macro generates the ring buffer struct, the probe functions, the format descriptors, and the tracepoint itself — a complete, self-contained event subsystem.

5. **USDT extends this to userspace**: via ELF note sections (`.note.stapsdt`) and kernel uprobes, giving userspace programs the same near-zero-cost static probe semantics.

6. **eBPF is the modern consumer**: raw tracepoints + BTF CO-RE give you type-safe, version-portable, programmable observation of any tracepoint in the kernel or userspace.

The entire system is designed around one core principle: **observation must never significantly alter the thing being observed**. Every design decision — jump labels, per-CPU ring buffers, RCU probe lists, semaphore-guarded USDT — flows from that principle.
