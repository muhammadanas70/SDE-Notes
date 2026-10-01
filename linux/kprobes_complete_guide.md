# Kprobes: A Complete In-Depth Guide
## Dynamic Kernel Instrumentation from First Principles

---

## Table of Contents

1. [What Are Kprobes?](#1-what-are-kprobes)
2. [Historical Context and Motivation](#2-historical-context-and-motivation)
3. [Mental Model: The Execution Interception Model](#3-mental-model-the-execution-interception-model)
4. [Architecture Deep Dive](#4-architecture-deep-dive)
5. [CPU-Level Mechanics: x86_64 Internals](#5-cpu-level-mechanics-x86_64-internals)
6. [ARM64 / AArch64 Internals](#6-arm64--aarch64-internals)
7. [Types of Probes](#7-types-of-probes)
8. [Kernel Data Structures](#8-kernel-data-structures)
9. [The kprobe Lifecycle](#9-the-kprobe-lifecycle)
10. [Pre-handler and Post-handler Execution](#10-pre-handler-and-post-handler-execution)
11. [kretprobes: Function Return Tracing](#11-kretprobes-function-return-tracing)
12. [Probe Optimizations: Jump Optimization](#12-probe-optimizations-jump-optimization)
13. [Blacklisted and Non-probeable Functions](#13-blacklisted-and-non-probeable-functions)
14. [The kprobes Notifier Chain](#14-the-kprobes-notifier-chain)
15. [C Implementation: Full Examples](#15-c-implementation-full-examples)
16. [Rust Implementation in the Linux Kernel](#16-rust-implementation-in-the-linux-kernel)
17. [kprobes and eBPF](#17-kprobes-and-ebpf)
18. [kprobes via ftrace](#18-kprobes-via-ftrace)
19. [kprobes via perf_events](#19-kprobes-via-perf_events)
20. [The /sys/kernel/debug/kprobes Interface](#20-the-syskerneldebugkprobes-interface)
21. [kprobes and KASLR](#21-kprobes-and-kaslr)
22. [kprobes and Kernel Live Patching](#22-kprobes-and-kernel-live-patching)
23. [Multi-probe Scenarios and Aggregation](#23-multi-probe-scenarios-and-aggregation)
24. [Safety, Reentrancy, and Locking](#24-safety-reentrancy-and-locking)
25. [Performance Impact and Overhead Analysis](#25-performance-impact-and-overhead-analysis)
26. [Real-World Use Cases](#26-real-world-use-cases)
27. [Comparison with Other Tracing Mechanisms](#27-comparison-with-other-tracing-mechanisms)
28. [Limitations and Caveats](#28-limitations-and-caveats)
29. [Debugging kprobes Themselves](#29-debugging-kprobes-themselves)
30. [Complete Reference Summary](#30-complete-reference-summary)

---

## 1. What Are Kprobes?

**Kprobes** (Kernel Probes) is a dynamic tracing framework built into the Linux kernel that allows you to insert breakpoints into virtually any kernel instruction — *without rebooting, recompiling, or modifying the kernel source*. It provides a safe, structured mechanism to intercept kernel execution, inspect CPU registers, examine kernel memory, gather statistics, and even alter execution flow.

The core idea: you identify a kernel address (by symbol name or raw address), and kprobes will ensure that whenever execution reaches that point, your callback function runs — before the instruction executes, after it executes, or at function return.

### What kprobes gives you:
- **Arbitrary instruction-level probing** — not just function entries/exits
- **Register access** — full `pt_regs` state at the probe point
- **Stack inspection** — access to the call stack
- **Conditional probing** — your handler decides what to record
- **Zero overhead when disabled** — no-op when the probe is not active (with optimization)
- **No kernel recompile needed** — entirely runtime

### What kprobes is NOT:
- Not a debugger (no interactive stepping)
- Not a fault injector by default (though it can be used as one)
- Not a way to replace code (use live patching for that)
- Not zero-overhead when active

---

## 2. Historical Context and Motivation

### Before Kprobes (The Dark Ages)

Prior to kprobes, instrumenting the Linux kernel required:

1. **Adding `printk()` calls** and recompiling — intrusive, slow, requires reboot
2. **Using `kgdb`** — requires a serial/network debug connection, stops execution
3. **Hardware breakpoints** (DR0–DR3 on x86) — limited to 4, no post-processing
4. **SystemTap scripts calling kernel modules** — complex toolchain
5. **Custom kernel patches** — not deployable on production systems

### Origins

Kprobes was originally developed at **IBM Research** (Vamsi Krishna, Ananth Mavinakayanahalli, et al.) and merged into the mainline Linux kernel in **version 2.6.9 (2004)**. The design was inspired by DProbes (Dynamic Probes), an earlier IBM project.

The design goals were:
- Work on production kernels without source modification
- Minimal overhead when probes are not firing
- Safe for use in multi-CPU environments
- Extensible for higher-level tools (SystemTap, perf, eBPF)

### Evolution Timeline

```
2004  - Linux 2.6.9:  kprobes merged (x86 only)
2005  - Linux 2.6.11: kretprobes added
2006  - Linux 2.6.17: ARM support
2007  - Linux 2.6.22: powerpc, s390 support  
2008  - Linux 2.6.26: MIPS support
2012  - Linux 3.5:    Jump optimization (boosts performance significantly)
2014  - Linux 3.19:   kprobes + eBPF integration begins
2015  - Linux 4.1:    kprobe_events via tracefs (perf-style)
2016  - Linux 4.6:    BPF_PROG_TYPE_KPROBE
2020  - Linux 5.7:    fprobe (function-level fast kprobes via ftrace)
2021  - Linux 5.15:   Rust in-tree (kprobe Rust bindings begin)
2022  - Linux 6.1:    Rust officially in-tree (kprobes Rust API)
```

---

## 3. Mental Model: The Execution Interception Model

Before diving into mechanics, build this mental model:

### The Intercept-Execute-Resume Model

```
Normal Execution:
  ┌──────────────────────────────────────────────────┐
  │ ... → instr_N-1 → [TARGET_INSTR] → instr_N+1 → ..│
  └──────────────────────────────────────────────────┘
                              ↓ (linear, no interruption)

With kprobe Installed:
  ┌──────────────────────────────────────────────────────────────────┐
  │                                                                  │
  │  ... → instr_N-1 → [INT3/BRK] → pre_handler()                  │
  │                                      ↓                          │
  │                                 [single-step                     │
  │                                  original instr                  │
  │                                  in safe slot]                   │
  │                                      ↓                          │
  │                                 post_handler()                   │
  │                                      ↓                          │
  │                               → instr_N+1 → ...                 │
  └──────────────────────────────────────────────────────────────────┘
```

Think of it as: the kernel **hijacks** the instruction stream at that point, **calls your code**, **re-executes the original instruction** in a safe side-channel, then **resumes** normal execution — all transparently.

### The Three-Phase Model

```
Phase 1: INSTALL TIME
  ┌────────────────────────────────────────────┐
  │  register_kprobe()                         │
  │    ├── Resolve symbol → address            │
  │    ├── Save original byte(s) at address    │
  │    ├── Copy original instr to slot buffer  │
  │    ├── Write INT3 (0xCC) at address        │
  │    └── Add to kprobe hash table            │
  └────────────────────────────────────────────┘

Phase 2: HIT TIME (per execution)
  ┌────────────────────────────────────────────┐
  │  CPU hits INT3                             │
  │    ├── Exception → kprobe_int3_handler()   │
  │    ├── Call pre_handler() [your code]      │
  │    ├── Set up single-step                  │
  │    ├── Execute original instruction        │
  │    └── Call post_handler() [your code]     │
  └────────────────────────────────────────────┘

Phase 3: REMOVE TIME
  ┌────────────────────────────────────────────┐
  │  unregister_kprobe()                       │
  │    ├── Restore original byte(s)            │
  │    ├── Remove from hash table              │
  │    └── Synchronize CPUs (ensure no        │
  │         CPU is mid-probe on this site)     │
  └────────────────────────────────────────────┘
```

---

## 4. Architecture Deep Dive

### The Full System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         USER SPACE                                          │
│                                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │  perf tool   │  │  BCC/bpftrace│  │  SystemTap   │  │ Custom Module │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └───────┬───────┘  │
│         │                 │                  │                   │          │
└─────────┼─────────────────┼──────────────────┼───────────────────┼──────────┘
          │                 │                  │                   │
          │  perf_event_open│  BPF syscall      │  staprun         │  insmod
          │                 │                  │                   │
┌─────────┼─────────────────┼──────────────────┼───────────────────┼──────────┐
│         │         KERNEL SPACE                │                   │          │
│         ▼                 ▼                  ▼                   ▼          │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐  ┌───────────────┐   │
│  │ perf_events │  │  eBPF subsys │  │  Staprun     │  │ kernel module │   │
│  │  subsystem  │  │              │  │  kernel mod  │  │               │   │
│  └──────┬──────┘  └──────┬───────┘  └──────┬───────┘  └───────┬───────┘   │
│         │                │                  │                   │           │
│         └────────────────┴──────────────────┴───────────────────┘           │
│                                    │                                         │
│                                    ▼                                         │
│         ┌──────────────────────────────────────────────────────────┐        │
│         │                  KPROBES CORE LAYER                      │        │
│         │                                                          │        │
│         │  ┌────────────┐  ┌────────────┐  ┌───────────────────┐  │        │
│         │  │  kprobes   │  │ kretprobes │  │   kprobe_events   │  │        │
│         │  │  API       │  │   API      │  │   (tracefs)       │  │        │
│         │  └─────┬──────┘  └─────┬──────┘  └────────┬──────────┘  │        │
│         │        │               │                   │             │        │
│         │        └───────────────┴───────────────────┘             │        │
│         │                        │                                 │        │
│         │              ┌─────────▼──────────┐                      │        │
│         │              │   kprobes engine   │                      │        │
│         │              │ (kernel/kprobes.c) │                      │        │
│         │              └─────────┬──────────┘                      │        │
│         │                        │                                 │        │
│         │      ┌─────────────────┼──────────────────┐             │        │
│         │      │                 │                  │             │        │
│         │      ▼                 ▼                  ▼             │        │
│         │  ┌─────────┐    ┌────────────┐    ┌────────────┐       │        │
│         │  │  Hash   │    │  Arch-spec │    │  Optimize  │       │        │
│         │  │  Table  │    │  handlers  │    │  (JMP opt) │       │        │
│         │  └─────────┘    └─────┬──────┘    └────────────┘       │        │
│         └────────────────────── │ ─────────────────────────────── ┘        │
│                                 │                                           │
│              ┌──────────────────┼──────────────────────┐                    │
│              │                  │                      │                    │
│              ▼                  ▼                      ▼                    │
│     ┌─────────────────┐  ┌────────────────┐  ┌───────────────────┐         │
│     │ arch/x86/kernel │  │ arch/arm64/    │  │ arch/powerpc/     │         │
│     │ kprobes.c       │  │ probes/        │  │ kprobes.c         │         │
│     │                 │  │ kprobes.c      │  │                   │         │
│     │ - INT3 handler  │  │ - BRK handler  │  │ - TRAP handler    │         │
│     │ - single-step   │  │ - single-step  │  │ - single-step     │         │
│     └─────────────────┘  └────────────────┘  └───────────────────┘         │
│                                 │                                           │
│                                 ▼                                           │
│                    ┌─────────────────────────┐                              │
│                    │   PHYSICAL KERNEL TEXT  │                              │
│                    │                         │                              │
│                    │  func_foo:              │                              │
│                    │    0xffffffff812345ab:  │                              │
│                    │      [INT3] ← was: PUSH │                              │
│                    │      RBP   ← original   │                              │
│                    │      ...                │                              │
│                    └─────────────────────────┘                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Source File Map (Linux Kernel)

```
kernel/
├── kprobes.c              ← Core: registration, hash table, generic logic
├── trace/
│   ├── trace_kprobe.c     ← kprobe_events (tracefs interface)
│   └── bpf_trace.c        ← eBPF + kprobe integration

arch/x86/kernel/
├── kprobes/
│   ├── core.c             ← x86 breakpoint insertion, single-step
│   ├── opt.c              ← Jump optimization for x86
│   └── ftrace.c           ← ftrace-backed kprobes for x86

arch/arm64/kernel/probes/
├── kprobes.c              ← arm64 BRK-based probing
├── kprobes-arm64.h
└── simulate-insn.c        ← Instruction simulation (no single-step HW)

include/linux/
├── kprobes.h              ← Public API, data structures
└── kretprobes.h           ← kretprobe structures
```

---

## 5. CPU-Level Mechanics: x86_64 Internals

### How INT3 Works

On x86, the breakpoint instruction is `INT3` (opcode `0xCC`), a single-byte instruction that triggers interrupt vector 3. This is the foundation of kprobes on x86.

```
Normal instruction stream (before probe):
  Address           Bytes          Assembly
  ffffffff81200000: 55             push rbp
  ffffffff81200001: 48 89 e5       mov  rbp, rsp
  ffffffff81200004: 48 83 ec 10    sub  rsp, 0x10

After kprobe at ffffffff81200000:
  Address           Bytes          Assembly
  ffffffff81200000: CC             int3          ← replaced by kprobes
  ffffffff81200001: 48 89 e5       mov  rbp, rsp
  ffffffff81200004: 48 83 ec 10    sub  rsp, 0x10

Slot buffer (kprobe_insn_cache):
  [slot+0]:         55             push rbp      ← copy of original instr
  [slot+1]:         e9 xx xx xx xx jmp  <back>   ← trampoline back
```

### The INT3 Exception Flow

```
CPU executes INT3 at probe address
         │
         ▼
 ┌───────────────────────────────┐
 │  CPU: save registers to stack │
 │  push rflags, cs, rip, ss, rsp│
 │  (rip = probe_addr + 1)       │
 └───────────────┬───────────────┘
                 │
                 ▼
 ┌───────────────────────────────┐
 │  IDT[3] → do_int3()           │
 │  (arch/x86/kernel/traps.c)    │
 └───────────────┬───────────────┘
                 │
                 ▼
 ┌───────────────────────────────┐
 │  kprobe_int3_handler()        │
 │  (arch/x86/kernel/kprobes/   │
 │   core.c)                     │
 │                               │
 │  1. Disable preemption        │
 │  2. Lookup kprobe at addr-1   │
 │  3. Verify it's a kprobe INT3 │
 └───────────────┬───────────────┘
                 │
                 ▼
 ┌───────────────────────────────┐
 │  Call pre_handler(kp, regs)   │
 │  [YOUR CODE HERE]             │
 └───────────────┬───────────────┘
                 │
                 ▼
 ┌───────────────────────────────┐
 │  Setup Single-Step (SS):      │
 │  - Set TF (Trap Flag) in     │
 │    EFLAGS in saved regs       │
 │  - Set rip = slot_buffer      │
 │    (where copy of original   │
 │     instruction lives)        │
 └───────────────┬───────────────┘
                 │
                 ▼ (iret — resumes at slot)
 ┌───────────────────────────────┐
 │  CPU executes ORIGINAL INSTR  │
 │  (in slot buffer, e.g. PUSH)  │
 │  TF set → fires debug fault   │
 └───────────────┬───────────────┘
                 │
                 ▼
 ┌───────────────────────────────┐
 │  IDT[1] → do_debug()         │
 │  kprobe_debug_handler()       │
 │                               │
 │  1. Detect this is our SS     │
 │  2. Call post_handler()       │
 │     [YOUR CODE HERE]          │
 │  3. Clear TF flag             │
 │  4. Fix up RIP: point back    │
 │     to instr_after_probe      │
 └───────────────┬───────────────┘
                 │
                 ▼ (iret)
 ┌───────────────────────────────┐
 │  Normal execution resumes     │
 │  at: probe_addr + instr_len   │
 └───────────────────────────────┘
```

### The Trap Flag (TF) Mechanism

The x86 EFLAGS register has a bit called **TF (Trap Flag)** at bit 8. When set, the CPU generates a debug exception (`#DB`, interrupt vector 1) after executing **every single instruction**. Kprobes uses this to single-step the original instruction:

1. Set TF in the EFLAGS saved on the stack (before iret)
2. Set RIP to the slot buffer containing the original instruction copy
3. iret → CPU executes original instruction → TF fires #DB immediately after
4. In #DB handler, clear TF, fix RIP → resume normally

### RIP Fixup Problem

Some instructions are **RIP-relative** on x86-64 (e.g., `mov rax, [rip+offset]`). When the instruction is copied to a slot buffer at a different address, the `rip+offset` resolves to the wrong address.

The solution:

```c
/* arch/x86/kernel/kprobes/core.c */

static void __kprobes
kprobe_emulate_insn(struct kprobe *p, struct pt_regs *regs)
{
    /*
     * RIP-relative instruction at probe site:
     *   target_addr = probe_rip + instr_len + offset
     *
     * In slot buffer:
     *   slot_rip = slot_addr
     *   target_addr = slot_rip + instr_len + offset  ← WRONG
     *
     * Fix: rewrite the offset field in the copied instruction
     *   new_offset = target_addr - (slot_addr + instr_len)
     */
}
```

Kprobes handles this by **rewriting the displacement** in the copied instruction to compensate for the address difference between the probe site and the slot buffer.

### Instruction Slot Buffer (kprobe_insn_cache)

```
┌──────────────────────────────────────────────────────┐
│              kprobe_insn_cache                       │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │  Page 0 (executable, allocated with            │  │
│  │          module_alloc() in module memory range)│  │
│  │                                                │  │
│  │  slot[0]: [INSTR_BYTES][JMP trampoline]        │  │
│  │  slot[1]: [INSTR_BYTES][JMP trampoline]        │  │
│  │  slot[2]: [INSTR_BYTES][JMP trampoline]        │  │
│  │  ...                                           │  │
│  └────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────┐  │
│  │  Page 1 ...                                    │  │
│  └────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘

Each slot:
  MAX_INSN_SIZE bytes for the instruction copy
  + MAX_STACK_SIZE bytes for the return trampoline
  = KPROBE_INSN_SLOT_SIZE total per slot
```

The slot pages are allocated from the module address range (within 2GB of the kernel) so that RIP-relative jumps can reach them.

---

## 6. ARM64 / AArch64 Internals

ARM64 does **not** have a single-byte breakpoint like x86's INT3. It also does **not** support the hardware Trap Flag for single-stepping. This requires a fundamentally different approach.

### ARM64 BRK Instruction

ARM64 uses the `BRK #imm` instruction (a 4-byte encoding) as a software breakpoint. Kprobes uses `BRK #0x004` (ESR_ELx value `0xF2000004`).

```
Normal ARM64 instruction (4 bytes, fixed-width):
  ffffffff80010000: a9bf7bfd  stp x29, x30, [sp, #-16]!

After kprobe installation:
  ffffffff80010000: d4200080  brk #0x4   ← overwritten (4 bytes, atomic)
```

Because ARM64 instructions are always 4 bytes (fixed-width ISA), the replacement is **atomic** from the CPU's perspective — no half-written instruction issue.

### ARM64 Single-Step Emulation

Since ARM64 has no hardware Trap Flag, kprobes **emulates** the original instruction in software rather than executing it in a slot buffer:

```
BRK #4 fires → kprobe_breakpoint_handler()
     │
     ├── Call pre_handler()
     │
     ├── Is instruction emulatable?
     │       ├── YES: emulate_ldr(), emulate_str(), etc.
     │       │        (arch/arm64/kernel/probes/simulate-insn.c)
     │       │
     │       └── NO:  use hardware single-step:
     │                - Set MDSCR_EL1.SS bit (Software Step)
     │                - Set ELR_EL1 = slot buffer with original instr
     │                - eret → execute 1 instruction → SSBS exception
     │
     └── Call post_handler()
```

The `simulate-insn.c` file contains pure software implementations of dozens of instruction behaviors (loads, stores, branches, etc.).

### Instructions That Cannot Be Probed on ARM64

- Instructions that modify `PC` in unpredictable ways (indirect branches to unknown targets)
- `SVC`, `HVC`, `SMC` (exception-generating)
- `MSR`/`MRS` for certain system registers
- Load/store exclusive (`ldxr`/`stxr`) — would break exclusive monitor

These are checked during `register_kprobe()` via `arch_check_kprobe()`.

---

## 7. Types of Probes

### 7.1 kprobe (Instruction-Level Probe)

The fundamental probe type. Fires at a specific kernel instruction.

```
                     pre_handler                   post_handler
                          │                              │
                          ▼                              ▼
... → prev_instr → [BREAKPOINT] ─────────────────→ [TARGET_INSTR] → next_instr → ...
                       INT3/BRK        single-step
                                      execution
```

**Use when**: You need to inspect state at a specific non-entry instruction, or when neither entry nor exit is the right point.

### 7.2 kretprobe (Return Probe)

Fires when a function **returns**. It intercepts the return address on the stack and redirects it through a trampoline.

```
  caller:                        callee (sys_read):
  ┌─────────────────┐            ┌──────────────────────────────┐
  │ call sys_read   │────────────►│ entry: kprobe fires (optl.)  │
  │                 │            │   ...                         │
  │  [return addr]  │◄───────────│   ret ← replaced by kretprobe│
  │   on stack      │            │         trampoline            │
  └─────────────────┘            └──────────────────────────────┘
                                              │
                                              ▼
                                    kretprobe trampoline:
                                      entry_handler() ← fires at entry (opt.)
                                      handler()       ← fires at return
                                      restore real return address
                                      jmp real_return_addr
```

**Use when**: You want to measure function duration, capture return values, or correlate entry arguments with return results.

### 7.3 jprobe (DEPRECATED and Removed)

`jprobe` was an older API (removed in Linux 4.15) that provided a more "C-like" probe handler with argument access. It was replaced by kprobes + `pt_regs` access or eBPF programs.

```c
/* HISTORICAL — do not use */
struct jprobe jp = {
    .entry = my_jprobe_handler,  /* must match target prototype */
    .kp.symbol_name = "do_fork",
};
/* Removed in 4.15 — use kprobe + pt_regs instead */
```

### 7.4 fprobe (Linux 5.18+)

`fprobe` is a newer, higher-performance mechanism built on **ftrace** rather than breakpoints. It uses ftrace's function hooks to provide function entry/return tracing with significantly lower overhead.

```
kprobe:  INT3 → exception → handler   (~100-300ns overhead)
fprobe:  ftrace hook → handler         (~10-50ns overhead)
```

fprobe does not support arbitrary instruction-level probing — only function entry/return.

---

## 8. Kernel Data Structures

### struct kprobe

```c
/* include/linux/kprobes.h */

struct kprobe {
    struct hlist_node hlist;        /* Hash table linkage (keyed by addr) */

    /* List of kprobes at the same address (multi-probe support) */
    struct list_head list;

    /* Count of temporarily disarmed breakpoints (for optimize/deoptimize) */
    unsigned long nmissed;

    /* Location of the probe point */
    kprobe_opcode_t *addr;          /* Exact address to probe */

    /* Allow user to indicate symbol name of the probe point */
    const char *symbol_name;        /* e.g. "do_sys_open" */
    unsigned int offset;            /* Byte offset into symbol */

    /* Called before target instruction executes */
    kprobe_pre_handler_t pre_handler;

    /* Called after target instruction executes (before resume) */
    kprobe_post_handler_t post_handler;

    /* Saved original instruction */
    kprobe_opcode_t opcode;         /* First byte (for INT3 detection) */

    /* Copy of the original instruction for single-stepping */
    struct arch_specific_insn ainsn;/* Arch-specific: slot + metadata */

    /* Probe status flags */
    u32 flags;
/*
 * Flags:
 *   KPROBE_FLAG_GONE        - unregistered but still allocated
 *   KPROBE_FLAG_DISABLED    - temporarily disabled
 *   KPROBE_FLAG_OPTIMIZED   - using jump optimization
 *   KPROBE_FLAG_FTRACE      - backed by ftrace
 */
};
```

### struct arch_specific_insn (x86)

```c
/* arch/x86/include/asm/kprobes.h */

struct arch_specific_insn {
    kprobe_opcode_t *insn;          /* Pointer to slot buffer copy */
    unsigned int boostable;         /* Can use boost (no single-step needed) */
    unsigned char size;             /* Instruction length in bytes */
    union {
        unsigned char opcode;
        struct {
            unsigned char type;     /* Instruction type (branch, etc.) */
        };
    };
};
```

### struct kretprobe

```c
/* include/linux/kprobes.h */

struct kretprobe {
    struct kprobe kp;               /* Embedded kprobe (for entry) */

    /* Handler called when function returns */
    kretprobe_handler_t handler;

    /* Optional: handler called at function entry */
    kretprobe_handler_t entry_handler;

    /* Maximum number of concurrent instances of this function */
    int maxactive;

    /* Number of instances currently active */
    int nmissed;

    /* Size of extra per-instance data (user can store state here) */
    size_t data_size;

    /* Freelist of kretprobe_instance structures */
    struct freelist_head freelist;

    /* RCU head for safe deallocation */
    struct rcu_head rcu;
};

struct kretprobe_instance {
    union {
        struct freelist_node freelist;
        struct rcu_head rcu;
    };
    struct llist_node llist;        /* Per-task list of active instances */
    struct kretprobe __rcu *rph;    /* Back-pointer to kretprobe */
    kprobe_opcode_t *ret_addr;      /* Saved real return address */
    void *fp;                       /* Frame pointer at entry */
    char data[];                    /* User data (data_size bytes) */
};
```

### The kprobe Hash Table

```c
/* kernel/kprobes.c */

#define KPROBE_HASH_BITS 6
#define KPROBE_TABLE_SIZE (1 << KPROBE_HASH_BITS)  /* 64 buckets */

static struct hlist_head kprobe_table[KPROBE_TABLE_SIZE];

/*
 * Hash function: hash the probe address into one of 64 buckets
 * Uses hash_ptr() which hashes based on the pointer value.
 *
 * Lookup: O(1) average, O(n) worst case (all probes at same hash)
 */

static inline struct hlist_head *kprobe_table_entry(unsigned long addr)
{
    return &kprobe_table[hash_ptr((void *)addr, KPROBE_HASH_BITS)];
}
```

### Per-CPU State

```c
/* arch/x86/include/asm/kprobes.h */

struct kprobe_ctlblk {
    unsigned long kprobe_status;          /* KPROBE_HIT_ACTIVE, etc. */
    unsigned long kprobe_old_flags;       /* Saved EFLAGS */
    unsigned long kprobe_saved_flags;     /* For nested probe handling */
    struct kprobe *cur_kprobe;            /* Currently firing probe */
    struct prev_kprobe prev_kprobe;       /* For nested probes */
};

DECLARE_PER_CPU(struct kprobe_ctlblk, kprobe_ctlblk);
```

This per-CPU state is critical: each CPU independently tracks which probe it's currently processing, preventing cross-CPU interference.

---

## 9. The kprobe Lifecycle

### Registration

```c
/* Simplified flow of register_kprobe() in kernel/kprobes.c */

int register_kprobe(struct kprobe *p)
{
    int ret;
    struct kprobe *old_p;

    /* 1. Resolve symbol name to address if needed */
    if (p->symbol_name) {
        p->addr = kprobe_lookup_name(p->symbol_name, p->offset);
        if (!p->addr)
            return -ENOENT;
    }

    /* 2. Apply KASLR offset (kernel address space layout randomization) */
    p->addr = (kprobe_opcode_t *)
        (((unsigned long)p->addr) + kaslr_offset());

    /* 3. Blacklist check: refuse to probe certain critical functions */
    if (within_kprobe_blacklist((unsigned long)p->addr))
        return -EINVAL;

    /* 4. Check if address is in kernel text (.text section) */
    if (!kernel_text_address((unsigned long)p->addr))
        return -EINVAL;

    /* 5. Architecture-specific checks (e.g., not in kprobe handler itself) */
    ret = arch_check_kprobe(p);
    if (ret)
        return ret;

    /* 6. Prepare the arch-specific instruction slot */
    ret = prepare_kprobe(p);  /* calls arch_prepare_kprobe() */
    if (ret)
        return ret;

    /* 7. Check if another probe exists at this address */
    old_p = get_kprobe(p->addr);
    if (old_p) {
        /* Aggregate: create or extend an aggregated kprobe */
        ret = register_aggr_kprobe(old_p, p);
        goto out;
    }

    /* 8. Add to hash table */
    hlist_add_head_rcu(&p->hlist,
        &kprobe_table[kprobe_hashfn(p->addr)]);

    /* 9. ARM the probe: write INT3/BRK at the target address */
    arm_kprobe(p);  /* calls arch_arm_kprobe() */

    /* 10. Optimization (may be async): try to replace INT3 with JMP */
    try_to_optimize_kprobe(p);

out:
    return ret;
}
```

### arch_prepare_kprobe() — x86

```c
/* arch/x86/kernel/kprobes/core.c */

int arch_prepare_kprobe(struct kprobe *p)
{
    /* 1. Allocate an instruction slot */
    p->ainsn.insn = get_insn_slot();
    if (!p->ainsn.insn)
        return -ENOMEM;

    /* 2. Copy the original instruction to the slot
     *    (handles RIP-relative fixup automatically) */
    __copy_instruction(p->ainsn.insn,  /* destination: slot */
                       p->addr,         /* source: probe site */
                       p->ainsn.insn,   /* for RIP fixup calc */
                       &p->ainsn.tp_t); /* for text patching */

    /* 3. Add a JMP after it to return to post-probe execution */
    synthesize_reljump(p->ainsn.insn + p->ainsn.size,
                       p->addr + p->ainsn.size);  /* jump to next instr */

    /* 4. Save the original first byte (for disarm) */
    p->opcode = *p->addr;

    return 0;
}
```

### arm_kprobe() — Writing the Breakpoint

```c
/* kernel/kprobes.c and arch/x86/kernel/kprobes/core.c */

static void arm_kprobe(struct kprobe *kp)
{
    /*
     * We must ensure the write is visible to all CPUs before any CPU
     * can execute it. Use text_poke_bp() which:
     *   1. Makes the page writable (if kernel text is RO)
     *   2. Writes the INT3 byte
     *   3. Issues appropriate memory barriers
     *   4. Makes the page read-only again
     */
    cpus_read_lock();
    arch_arm_kprobe(kp);
    cpus_read_unlock();
}

void arch_arm_kprobe(struct kprobe *p)
{
    u8 int3 = INT3_INSN_OPCODE;  /* 0xCC */

    /*
     * text_poke_bp() atomically replaces one byte with INT3.
     * It uses stop_machine() or IPI to pause other CPUs while
     * doing the patching, ensuring consistency.
     */
    text_poke_bp(p->addr, &int3, 1, NULL);
}
```

### Disarming and Unregistration

```c
void unregister_kprobe(struct kprobe *p)
{
    /* 1. Remove from aggregated probe list (if aggregated) */
    /* 2. If last probe at address: */

    /* 3. Disarm: restore original byte(s) */
    disarm_kprobe(p, true);
    /* This calls text_poke_bp(p->addr, &p->opcode, 1, NULL) */

    /* 4. Synchronize: wait for all CPUs to finish any in-flight
     *    execution of this probe's handler
     *    Uses synchronize_rcu() to wait for RCU grace period */
    synchronize_rcu();

    /* 5. Remove from hash table */
    hlist_del_rcu(&p->hlist);

    /* 6. Free instruction slot */
    free_insn_slot(p->ainsn.insn, ...);
}
```

---

## 10. Pre-handler and Post-handler Execution

### Handler Signatures

```c
/* Pre-handler: called BEFORE the probed instruction executes */
typedef int (*kprobe_pre_handler_t)(struct kprobe *p, struct pt_regs *regs);

/* Post-handler: called AFTER the probed instruction executes */
typedef void (*kprobe_post_handler_t)(struct kprobe *p,
                                      struct pt_regs *regs,
                                      unsigned long flags);
```

### The pt_regs Structure (x86_64)

This is the core data structure giving you access to CPU state:

```c
/* arch/x86/include/asm/ptrace.h */

struct pt_regs {
    /* General purpose registers (in reverse order for syscall ABI) */
    unsigned long r15;
    unsigned long r14;
    unsigned long r13;
    unsigned long r12;
    unsigned long rbp;
    unsigned long rbx;

    /* Caller-saved registers (syscall ABI) */
    unsigned long r11;
    unsigned long r10;
    unsigned long r9;
    unsigned long r8;
    unsigned long rax;
    unsigned long rcx;
    unsigned long rdx;
    unsigned long rsi;
    unsigned long rdi;

    /* On interrupt/exception entry (pushed by hardware) */
    unsigned long orig_rax;    /* Syscall number or error code */
    unsigned long rip;         /* Instruction pointer */
    unsigned long cs;          /* Code segment */
    unsigned long eflags;      /* EFLAGS register */
    unsigned long rsp;         /* Stack pointer */
    unsigned long ss;          /* Stack segment */
};
```

### Accessing Function Arguments via pt_regs

On x86-64 System V ABI, function arguments are passed in registers:

```
Arg 1: RDI
Arg 2: RSI
Arg 3: RDX
Arg 4: RCX (note: in kernel syscalls, R10 instead)
Arg 5: R8
Arg 6: R9
Return: RAX
```

So in a pre_handler probing function entry:

```c
static int my_pre_handler(struct kprobe *p, struct pt_regs *regs)
{
    /* If probing: void foo(int a, long b, char *c) */
    int a     = (int)regs->di;   /* arg1: RDI */
    long b    = (long)regs->si;  /* arg2: RSI */
    char *c   = (char *)regs->dx; /* arg3: RDX */

    printk(KERN_INFO "foo called: a=%d, b=%ld, c=%s\n", a, b, c);
    return 0;  /* 0 = continue; non-zero aborts further handling */
}
```

### Modifying Execution in the Handler

You can **change register values** in the pre_handler, and those changes take effect when the instruction executes:

```c
static int redirect_handler(struct kprobe *p, struct pt_regs *regs)
{
    /* Skip the probed instruction and jump elsewhere */
    regs->ip = (unsigned long)my_replacement_function;
    return 1;  /* Tell kprobes: don't single-step, we handled it */
}
```

**Warning**: Modifying `regs->ip` can corrupt the stack and crash the kernel. Use only when you fully understand the ABI.

### Handler Execution Context

```
┌─────────────────────────────────────────────────────────────┐
│  Handler Execution Context:                                 │
│                                                             │
│  - Interrupts: DISABLED (from INT3 exception entry)         │
│  - Preemption: DISABLED                                     │
│  - Running in: interrupt context (atomic)                   │
│                                                             │
│  FORBIDDEN in handlers:                                     │
│  - sleep() or any blocking operation                        │
│  - kmalloc(GFP_KERNEL) — must use GFP_ATOMIC               │
│  - mutex_lock() — use spin_lock() instead                   │
│  - copy_from_user() / copy_to_user()                        │
│  - schedule()                                               │
│                                                             │
│  ALLOWED:                                                   │
│  - Reading/writing kernel memory                            │
│  - Atomic operations                                        │
│  - spin_lock_irqsave()                                      │
│  - kmalloc(GFP_ATOMIC)                                      │
│  - printk() (but slow — use trace_printk() instead)        │
│  - Reading pt_regs                                          │
│  - Modifying pt_regs                                        │
└─────────────────────────────────────────────────────────────┘
```

---

## 11. kretprobes: Function Return Tracing

### The Return Address Hijacking Mechanism

```
Normal function call/return:
  caller:                         callee:
    call target ──────────────────► entry
                                    ...
    [return_addr on stack]◄──────── ret

With kretprobe:
    call target ──────────────────► entry (kprobe fires here optionally)
                                    ...
                                    ret  ← hijacked!
                                         return addr on stack replaced with
                                         kretprobe_trampoline addr

                              kretprobe_trampoline:
                                    call handler()   ← YOUR CODE
                                    jmp real_return_addr  ← restored
```

### The Trampoline Details

```c
/* kernel/kprobes.c */

/*
 * kretprobe_trampoline - the code jumped to when a kretprobe-modified
 *                        function returns.
 *
 * On x86-64, this is a small assembly stub:
 */
asm(
    ".global kretprobe_trampoline\n"
    "kretprobe_trampoline:\n"
    "    pushq $0\n"            /* placeholder for orig return addr */
    "    pushq %rax\n"          /* save all regs */
    "    pushq %rcx\n"
    "    ... (all regs) ...\n"
    "    call trampoline_handler\n"  /* C handler */
    "    ... (restore regs) ...\n"
    "    ret\n"                 /* returns to real caller */
);
```

### kretprobe Instance Management

Since a function can be called concurrently on multiple CPUs, kretprobe needs **one instance per concurrent call**:

```
kretprobe.maxactive = 20  ← allow up to 20 concurrent calls

freelist (pre-allocated):
  [inst_0] [inst_1] [inst_2] ... [inst_19]

When function is called:
  1. Pop inst_N from freelist
  2. Save real return address in inst_N->ret_addr
  3. Store entry args in inst_N->data (user data)
  4. Replace return address with kretprobe_trampoline
  5. Push inst_N onto current task's kretprobe list

When function returns (trampoline fires):
  1. Find inst_N for this task/frame
  2. Call handler(kri, regs) with inst_N
  3. User reads inst_N->data and regs->ax (return value)
  4. Restore real return address
  5. Return inst_N to freelist
```

### Accessing Entry Arguments at Return Time

```c
struct my_data {
    unsigned long entry_arg1;   /* arg captured at entry */
    ktime_t entry_time;         /* timestamp at entry */
};

static int entry_handler(struct kretprobe_instance *ri, struct pt_regs *regs)
{
    struct my_data *data = (struct my_data *)ri->data;

    data->entry_arg1 = regs->di;       /* 1st arg at function entry */
    data->entry_time = ktime_get();    /* capture entry timestamp */
    return 0;
}

static int ret_handler(struct kretprobe_instance *ri, struct pt_regs *regs)
{
    struct my_data *data = (struct my_data *)ri->data;
    long retval = regs_return_value(regs);  /* RAX on x86 */
    ktime_t duration = ktime_sub(ktime_get(), data->entry_time);

    pr_info("Function returned %ld after %lld ns (arg1 was %lu)\n",
            retval, ktime_to_ns(duration), data->entry_arg1);
    return 0;
}

static struct kretprobe my_kretprobe = {
    .handler        = ret_handler,
    .entry_handler  = entry_handler,
    .data_size      = sizeof(struct my_data),
    .maxactive      = 20,
    .kp.symbol_name = "vfs_read",
};
```

---

## 12. Probe Optimizations: Jump Optimization

### The Problem with INT3

Every probe hit requires:
1. CPU exception (INT3) → ~100 cycles
2. Kernel exception handler dispatch
3. Lookup in hash table
4. Call pre_handler
5. Single-step setup (set TF)
6. iret
7. Execute 1 instruction
8. Debug exception (#DB)
9. Call post_handler
10. Clear TF, fix RIP
11. iret

This is ~300-1000ns overhead per probe hit — substantial for hot paths.

### Jump Optimization (Linux 3.5+)

If a probe has **no post_handler**, kprobes can replace the INT3-based approach with a **direct JMP** to an "optimized handler":

```
INT3-based (normal):
  [INT3] → exception → hash lookup → handler → single-step → ...

JMP-based (optimized):
  [JMP <detour_buffer>] → detour_buffer:
                              call kprobe_handler()
                              execute original instruction
                              jmp back to next_instruction
```

```
Before optimization:
  addr+0:  CC           int3
  addr+1:  48 89 e5     mov  rbp, rsp   (remaining bytes of patched instr)

After optimization:
  addr+0:  E9 xx xx xx xx   jmp detour_buf
  addr+5:  ...

detour_buf:
  <NOP sled or frame setup>
  call kprobe_opt_handler  ← calls pre_handler
  55                       push rbp   ← original instruction
  E9 <back to addr+1>     jmp back
```

### Why 5 Bytes?

A relative JMP on x86-64 is `E9 + 4-byte offset` = 5 bytes. The original instruction at the probe site must be **at least 5 bytes long** for jump optimization to work. For shorter instructions, INT3 (1 byte) is used.

```c
/* arch/x86/kernel/kprobes/opt.c */

static int can_optimize(unsigned long paddr)
{
    /* Check instruction length >= 5 bytes */
    if (insn_len < JMP_OP_LEN)
        return 0;

    /* Check no branch targets land in the middle of our 5-byte patch */
    if (branch_into_range(paddr, paddr + JMP_OP_LEN))
        return 0;

    /* Check no exception table entries in the range */
    if (search_exception_tables(paddr) ||
        search_exception_tables(paddr + JMP_OP_LEN - 1))
        return 0;

    return 1;
}
```

### Optimization is Asynchronous

Optimization happens in a **work queue** (not during `register_kprobe()`) to avoid long critical sections. There's a brief window where the probe uses INT3 before being optimized to JMP.

```c
/* kernel/kprobes.c */

static void kprobe_optimizer(struct work_struct *work)
{
    mutex_lock(&kprobe_mutex);

    /* Process list of pending optimizations */
    do_optimize_kprobes();    /* Convert INT3 → JMP where possible */
    do_unoptimize_kprobes();  /* Revert JMP → INT3 where needed */

    mutex_unlock(&kprobe_mutex);
}

static DECLARE_DELAYED_WORK(optimizing_work, kprobe_optimizer);
```

---

## 13. Blacklisted and Non-probeable Functions

### Why Blacklisting Is Necessary

Probing certain functions would cause infinite recursion or deadlock:

```
If kprobe_int3_handler() itself were probed:
  CPU hits INT3 → kprobe_int3_handler() → 
    lookup probe → hits INT3 → kprobe_int3_handler() → 
      [infinite recursion → stack overflow → kernel panic]
```

### The Blacklist Mechanism

```c
/* kernel/kprobes.c */

/*
 * Functions marked with __kprobes or nokprobe_inline are added to the
 * kprobes blacklist at build time via the _kprobes section.
 */
static __initdata const char * const kprobe_blacklist_str[] = {
    "kprobe_int3_handler",
    "kprobe_debug_handler",
    "trampoline_handler",
    "kprobes_fault_handler",
    /* ... many more ... */
};
```

### Marking Functions as Non-probeable

In kernel code, you can mark functions to prevent probing:

```c
/* Method 1: __kprobes attribute (placed in .kprobes.text section) */
static int __kprobes my_critical_function(void)
{
    /* This function cannot be probed */
}

/* Method 2: nokprobe_inline (for inline functions) */
static nokprobe_inline int my_inline_helper(void)
{
    /* Also cannot be probed */
}

/* Method 3: NOKPROBE_SYMBOL macro */
NOKPROBE_SYMBOL(my_critical_function);
```

The linker script places `__kprobes` functions in a special section:

```
/* arch/x86/kernel/vmlinux.lds.S */
.kprobes.text : {
    *(.kprobes.text)   /* all __kprobes functions go here */
}
```

And `within_kprobe_blacklist()` checks if an address falls in this section or in the blacklist array.

### ftrace Infrastructure Conflict

Functions compiled with `-pg` for ftrace (or that use ftrace's `__fentry__`/`__mcount`) have a `CALL __fentry__` at their very first instruction. Kprobes must handle these carefully to avoid double-patching.

---

## 14. The kprobes Notifier Chain

Kprobes integrates with the kernel's `die_chain` notifier to intercept exceptions:

```c
/* arch/x86/kernel/kprobes/core.c */

static struct notifier_block kprobe_exceptions_nb = {
    .notifier_call  = kprobe_exceptions_notify,
    .priority       = 0x7fffffff,  /* Highest priority */
};

/* registered via: */
register_die_notifier(&kprobe_exceptions_nb);

/*
 * This ensures kprobes sees the exception FIRST, before any other
 * exception handler. It checks: "is this exception a kprobe hit?"
 * If yes, handle it. If no, pass it to the next handler.
 */
int kprobe_exceptions_notify(struct notifier_block *self,
                              unsigned long val, void *data)
{
    struct die_args *args = (struct die_args *)data;

    switch (val) {
    case DIE_INT3:
        /* x86: check if this INT3 is from a kprobe */
        if (kprobe_int3_handler(args->regs))
            return NOTIFY_STOP;  /* Handled by kprobes */
        break;
    case DIE_DEBUG:
        /* x86: check if this is our single-step debug exception */
        if (kprobe_debug_handler(args->regs))
            return NOTIFY_STOP;
        break;
    case DIE_GPF:
        /* Handle faults inside probe handlers */
        if (kprobe_fault_handler(args->regs, args->trapnr))
            return NOTIFY_STOP;
        break;
    }

    return NOTIFY_DONE;  /* Not ours, let kernel handle it */
}
```

---

## 15. C Implementation: Full Examples

### Example 1: Basic Function Entry Probe

```c
/* kprobe_example.c
 * Kernel module to probe vfs_read() entry
 * Compile: make -C /lib/modules/$(uname -r)/build M=$(pwd) modules
 */

#include <linux/kernel.h>
#include <linux/module.h>
#include <linux/kprobes.h>
#include <linux/uaccess.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Example");
MODULE_DESCRIPTION("kprobe example: trace vfs_read");

/* Pre-handler: fires just before vfs_read executes */
static int handler_pre(struct kprobe *p, struct pt_regs *regs)
{
    /*
     * vfs_read prototype:
     *   ssize_t vfs_read(struct file *file, char __user *buf,
     *                    size_t count, loff_t *pos)
     *
     * x86-64 ABI:
     *   RDI = file (arg1)
     *   RSI = buf  (arg2)
     *   RDX = count (arg3)
     *   RCX = pos  (arg4)
     */
    struct file *file = (struct file *)regs->di;
    size_t count      = (size_t)regs->dx;

    if (file && file->f_path.dentry) {
        pr_info("[kprobe] vfs_read: file=%s count=%zu pid=%d\n",
                file->f_path.dentry->d_name.name,
                count,
                current->pid);
    }

    return 0;  /* 0: continue; non-zero: skip post_handler */
}

/* Post-handler: fires after the instruction is single-stepped */
static void handler_post(struct kprobe *p, struct pt_regs *regs,
                         unsigned long flags)
{
    pr_info("[kprobe] vfs_read post handler: eflags=0x%lx\n",
            regs->flags);
}

static struct kprobe kp = {
    .symbol_name    = "vfs_read",
    .pre_handler    = handler_pre,
    .post_handler   = handler_post,
};

static int __init kprobe_init(void)
{
    int ret;

    ret = register_kprobe(&kp);
    if (ret < 0) {
        pr_err("register_kprobe failed: %d\n", ret);
        return ret;
    }

    pr_info("Planted kprobe at %p\n", kp.addr);
    return 0;
}

static void __exit kprobe_exit(void)
{
    unregister_kprobe(&kp);
    pr_info("kprobe at %p unregistered\n", kp.addr);
}

module_init(kprobe_init);
module_exit(kprobe_exit);
```

### Example 2: kretprobe for Latency Measurement

```c
/* kretprobe_latency.c
 * Measure latency of do_sys_openat2() (the openat2 syscall implementation)
 */

#include <linux/kernel.h>
#include <linux/module.h>
#include <linux/kprobes.h>
#include <linux/time64.h>
#include <linux/sched.h>

MODULE_LICENSE("GPL");

struct open_data {
    ktime_t entry_time;
    pid_t   pid;
    char    filename[256];
};

/* Entry handler: capture entry timestamp and filename */
static int entry_handler(struct kretprobe_instance *ri, struct pt_regs *regs)
{
    struct open_data *data = (struct open_data *)ri->data;
    struct filename *name;

    data->entry_time = ktime_get();
    data->pid        = current->pid;

    /*
     * do_sys_openat2(int dfd, const char __user *filename,
     *                struct open_how *how)
     * RSI = filename (user pointer)
     */
    name = getname_uflags((const char __user *)regs->si, 0);
    if (!IS_ERR(name)) {
        strncpy(data->filename, name->name, sizeof(data->filename) - 1);
        putname(name);
    } else {
        strcpy(data->filename, "<unknown>");
    }

    return 0;
}

/* Return handler: compute and print latency */
static int ret_handler(struct kretprobe_instance *ri, struct pt_regs *regs)
{
    struct open_data *data = (struct open_data *)ri->data;
    long retval            = regs_return_value(regs);
    ktime_t latency        = ktime_sub(ktime_get(), data->entry_time);

    pr_info("[kretprobe] open: pid=%d file=%s fd=%ld latency=%lldus\n",
            data->pid,
            data->filename,
            retval,
            ktime_to_us(latency));

    return 0;
}

static struct kretprobe my_kretprobe = {
    .handler       = ret_handler,
    .entry_handler = entry_handler,
    .data_size     = sizeof(struct open_data),
    .maxactive     = 64,  /* up to 64 concurrent open() calls */
    .kp = {
        .symbol_name = "do_sys_openat2",
    },
};

static int __init krp_init(void)
{
    int ret = register_kretprobe(&my_kretprobe);
    if (ret < 0) {
        pr_err("register_kretprobe failed: %d\n", ret);
        return ret;
    }
    pr_info("Planted kretprobe at %p\n", my_kretprobe.kp.addr);
    return 0;
}

static void __exit krp_exit(void)
{
    unregister_kretprobe(&my_kretprobe);
    pr_info("kretprobe unregistered. Missed %d instances.\n",
            my_kretprobe.nmissed);
}

module_init(krp_init);
module_exit(krp_exit);
```

### Example 3: Multiple Probes at the Same Address

```c
/* Aggregated kprobes: multiple handlers on same function */

static struct kprobe kp1 = {
    .symbol_name = "tcp_connect",
    .pre_handler = log_connection,
};

static struct kprobe kp2 = {
    .symbol_name = "tcp_connect",
    .pre_handler = count_connections,
};

static struct kprobe kp3 = {
    .symbol_name = "tcp_connect",
    .pre_handler = security_check,
};

static struct kprobe *kps[] = { &kp1, &kp2, &kp3, NULL };

static int __init multi_probe_init(void)
{
    return register_kprobes(kps);
    /*
     * Internally, kprobes creates an "aggregator" kprobe at the address
     * and chains kp1, kp2, kp3. Only one INT3 is written.
     * All three pre_handlers are called in registration order.
     */
}
```

### Example 4: Probe by Exact Address

```c
/* Probing by raw kernel virtual address (use with extreme care) */

#include <linux/kallsyms.h>

static int __init addr_probe_init(void)
{
    unsigned long addr;
    struct kprobe kp = {};

    /* Method 1: kallsyms lookup */
    addr = kallsyms_lookup_name("__x64_sys_write");
    if (!addr) {
        pr_err("Symbol not found\n");
        return -ENOENT;
    }

    kp.addr        = (kprobe_opcode_t *)addr;
    kp.pre_handler = my_handler;

    return register_kprobe(&kp);
}
```

### Example 5: Fault Injection Probe

```c
/* Inject faults into memory allocation paths (testing/chaos) */

static int alloc_fail_handler(struct kprobe *p, struct pt_regs *regs)
{
    static atomic_t call_count = ATOMIC_INIT(0);
    int n = atomic_inc_return(&call_count);

    /* Fail every 100th call to kmalloc */
    if (n % 100 == 0) {
        pr_info("Injecting kmalloc failure at call %d\n", n);
        /* Make kmalloc return NULL by setting return value to 0
         * We can't directly set RAX here (pre_handler, not ret_handler)
         * Instead, redirect RIP to a fault-injection function
         */
        regs->ip = (unsigned long)return_null_func;
        return 1;  /* Skip original instruction */
    }

    return 0;
}
```

### Example 6: Per-CPU Histogram

```c
/* Track distribution of vfs_write() sizes using per-CPU counters */

#include <linux/percpu.h>

#define NUM_BUCKETS 32

static DEFINE_PER_CPU(u64[NUM_BUCKETS], write_size_hist);
static DEFINE_PER_CPU(u64, write_total);

static int write_pre_handler(struct kprobe *p, struct pt_regs *regs)
{
    /*
     * vfs_write(struct file *file, const char __user *buf,
     *           size_t count, loff_t *pos)
     * RDX = count
     */
    size_t count = regs->dx;
    int bucket   = min(fls(count), NUM_BUCKETS - 1); /* log2 bucket */

    this_cpu_inc(write_size_hist[bucket]);
    this_cpu_add(write_total, count);

    return 0;
}

/* Read histogram from debugfs or /proc */
static int hist_show(struct seq_file *m, void *v)
{
    int cpu, bucket;
    u64 total[NUM_BUCKETS] = {0};

    for_each_possible_cpu(cpu) {
        for (bucket = 0; bucket < NUM_BUCKETS; bucket++) {
            total[bucket] += per_cpu(write_size_hist[bucket], cpu);
        }
    }

    seq_printf(m, "vfs_write size distribution:\n");
    for (bucket = 0; bucket < NUM_BUCKETS; bucket++) {
        if (total[bucket])
            seq_printf(m, "  [2^%2d]: %llu\n", bucket, total[bucket]);
    }
    return 0;
}
```

### Makefile for Kernel Modules

```makefile
# Makefile for kprobe examples
obj-m := kprobe_example.o kretprobe_latency.o

KERNEL_DIR ?= /lib/modules/$(shell uname -r)/build
PWD := $(shell pwd)

all:
	$(MAKE) -C $(KERNEL_DIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KERNEL_DIR) M=$(PWD) clean

load:
	sudo insmod kprobe_example.ko

unload:
	sudo rmmod kprobe_example

dmesg:
	sudo dmesg | tail -50
```

---

## 16. Rust Implementation in the Linux Kernel

### Overview of Rust kprobes Support

As of Linux 6.1+, Rust is officially supported in the kernel. Rust kprobe bindings provide a safe, idiomatic API wrapping the C kprobes infrastructure.

### Rust Bindings Architecture

```
rust/kernel/kprobes.rs      ← Safe Rust API
rust/bindings/               ← Auto-generated bindings from C headers
                              (bindgen from include/linux/kprobes.h)
kernel/kprobes.c             ← Underlying C implementation (unchanged)
```

### The Rust kprobe API

```rust
// rust/kernel/kprobes.rs (simplified from actual kernel source)

use core::ffi::c_int;
use crate::bindings;
use crate::error::{Error, Result};

/// A kernel probe that fires before a specific instruction executes.
///
/// # Safety
/// The probe handler runs in interrupt context with preemption disabled.
/// No sleeping operations are allowed.
pub struct KProbe {
    kp: bindings::kprobe,
}

unsafe impl Send for KProbe {}
unsafe impl Sync for KProbe {}

impl KProbe {
    /// Create a new KProbe targeting a kernel symbol.
    pub fn new(symbol: &'static core::ffi::CStr) -> Self {
        let mut kp = bindings::kprobe::default();
        kp.symbol_name = symbol.as_ptr();
        KProbe { kp }
    }

    /// Register the probe with the kernel.
    ///
    /// The `handler` closure is called before each execution of the
    /// probed instruction, with access to CPU registers.
    pub fn register<F>(&mut self, handler: F) -> Result<()>
    where
        F: Fn(&mut bindings::pt_regs) + Send + Sync + 'static,
    {
        // Store handler as a trait object
        // (in practice, uses a static dispatch mechanism)
        self.kp.pre_handler = Some(kprobe_pre_trampoline::<F>);

        let ret = unsafe { bindings::register_kprobe(&mut self.kp) };
        if ret < 0 {
            Err(Error::from_kernel_errno(ret))
        } else {
            Ok(())
        }
    }

    /// Disable the probe temporarily.
    pub fn disable(&mut self) {
        unsafe { bindings::disable_kprobe(&mut self.kp) };
    }

    /// Re-enable a disabled probe.
    pub fn enable(&mut self) {
        unsafe { bindings::enable_kprobe(&mut self.kp) };
    }
}

impl Drop for KProbe {
    fn drop(&mut self) {
        unsafe { bindings::unregister_kprobe(&mut self.kp) };
    }
}

/// FFI trampoline: bridges C callback to Rust closure
unsafe extern "C" fn kprobe_pre_trampoline<F>(
    kp: *mut bindings::kprobe,
    regs: *mut bindings::pt_regs,
) -> c_int
where
    F: Fn(&mut bindings::pt_regs),
{
    let handler = /* extract closure from kp user data */ todo!();
    handler(&mut *regs);
    0
}
```

### Rust KRetProbe

```rust
/// A return probe that fires when a function returns.
pub struct KRetProbe {
    krp: bindings::kretprobe,
}

impl KRetProbe {
    pub fn new(
        symbol: &'static core::ffi::CStr,
        max_active: i32,
    ) -> Self {
        let mut krp = bindings::kretprobe::default();
        krp.kp.symbol_name = symbol.as_ptr();
        krp.maxactive = max_active;
        KRetProbe { krp }
    }

    pub fn register<H, E, D>(
        &mut self,
        ret_handler: H,
        entry_handler: E,
        data_size: usize,
    ) -> Result<()>
    where
        H: Fn(&bindings::kretprobe_instance, &bindings::pt_regs) + 'static,
        E: Fn(&bindings::kretprobe_instance, &bindings::pt_regs) + 'static,
    {
        self.krp.handler       = Some(kretprobe_ret_trampoline::<H>);
        self.krp.entry_handler = Some(kretprobe_entry_trampoline::<E>);
        self.krp.data_size     = data_size;

        let ret = unsafe { bindings::register_kretprobe(&mut self.krp) };
        if ret < 0 {
            Err(Error::from_kernel_errno(ret))
        } else {
            Ok(())
        }
    }
}

impl Drop for KRetProbe {
    fn drop(&mut self) {
        unsafe { bindings::unregister_kretprobe(&mut self.krp) };
    }
}
```

### Full Rust Kernel Module with kprobe

```rust
// samples/rust/rust_kprobe_example.rs

#![no_std]
#![feature(allocator_api)]

use kernel::prelude::*;
use kernel::kprobes::{KProbe, KProbeHandler};
use kernel::pt_regs::PtRegs;

module! {
    type: RustKprobeModule,
    name: "rust_kprobe_example",
    author: "Example",
    description: "kprobe example in Rust",
    license: "GPL",
}

struct VfsReadProbeHandler;

impl KProbeHandler for VfsReadProbeHandler {
    fn pre_handler(&self, regs: &PtRegs) -> i32 {
        // Access registers via safe wrapper
        let count = regs.rdx();    // arg3: count
        let pid   = kernel::current_pid();

        pr_info!("vfs_read: count={count} pid={pid}\n");
        0  // continue execution
    }

    fn post_handler(&self, regs: &PtRegs) {
        // Called after single-step
    }
}

struct RustKprobeModule {
    probe: KProbe<VfsReadProbeHandler>,
}

impl kernel::Module for RustKprobeModule {
    fn init(_module: &'static ThisModule) -> Result<Self> {
        pr_info!("Rust kprobe example: init\n");

        let mut probe = KProbe::new(
            c_str!("vfs_read"),
            VfsReadProbeHandler,
        )?;
        probe.register()?;

        Ok(RustKprobeModule { probe })
    }
}

impl Drop for RustKprobeModule {
    fn drop(&mut self) {
        pr_info!("Rust kprobe example: exit\n");
        // probe.unregister() called automatically via Drop
    }
}
```

### PtRegs Safe Wrapper (Rust)

```rust
// rust/kernel/pt_regs.rs

use crate::bindings;

/// Safe wrapper around pt_regs providing typed register access.
pub struct PtRegs {
    inner: *mut bindings::pt_regs,
}

impl PtRegs {
    /// Safety: `regs` must be a valid pt_regs from the kernel
    pub unsafe fn from_raw(regs: *mut bindings::pt_regs) -> &'static Self {
        &*(regs as *const Self)
    }

    // x86-64 register accessors
    pub fn rdi(&self) -> u64 { unsafe { (*self.inner).di } }
    pub fn rsi(&self) -> u64 { unsafe { (*self.inner).si } }
    pub fn rdx(&self) -> u64 { unsafe { (*self.inner).dx } }
    pub fn rcx(&self) -> u64 { unsafe { (*self.inner).cx } }
    pub fn rax(&self) -> u64 { unsafe { (*self.inner).ax } }
    pub fn rip(&self) -> u64 { unsafe { (*self.inner).ip } }
    pub fn rsp(&self) -> u64 { unsafe { (*self.inner).sp } }
    pub fn eflags(&self) -> u64 { unsafe { (*self.inner).flags } }

    // Return value (RAX after function returns)
    pub fn return_value(&self) -> i64 {
        self.rax() as i64
    }

    // Modify RIP (DANGEROUS: can corrupt stack)
    /// # Safety
    /// Setting rip to an invalid address will crash the kernel.
    pub unsafe fn set_rip(&mut self, addr: u64) {
        (*self.inner).ip = addr;
    }
}
```

---

## 17. kprobes and eBPF

### Architecture: BPF Programs Attached to kprobes

eBPF (extended Berkeley Packet Filter) can be attached to kprobes via `bpf()` syscall with `BPF_PROG_TYPE_KPROBE`. This is the most common production use of kprobes today.

```
User Space:
  bpf() syscall
    BPF_PROG_LOAD (prog_type=BPF_PROG_TYPE_KPROBE, license="GPL")
    BPF_LINK_CREATE (prog_fd, attach_type=BPF_PERF_EVENT)

        │
        ▼
Kernel:
  perf_event created for kprobe
  BPF program attached to perf_event
        │
        ▼
Execution Path:
  kprobe fires
    → kprobe_dispatcher()
    → perf_event_overflow()
    → bpf_prog_run(bpf_prog, ctx)  ← BPF program executes
        (ctx = struct bpf_kprobe_ctx)
```

### BPF Context for kprobes

```c
/* The BPF program receives a bpf_context that wraps pt_regs */

struct bpf_kprobe_multi_link {
    struct bpf_link    link;
    struct fprobe      fp;
    /* ... */
};

/* In BPF C program: */
SEC("kprobe/vfs_read")
int BPF_KPROBE(trace_vfs_read, struct file *file,
               char __user *buf, size_t count, loff_t *pos)
{
    /* BPF_KPROBE macro expands args from pt_regs automatically */
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    bpf_printk("vfs_read: pid=%d count=%lu\n", pid, count);
    return 0;
}

SEC("kretprobe/vfs_read")
int BPF_KRETPROBE(trace_vfs_read_ret, ssize_t ret)
{
    bpf_printk("vfs_read returned: %ld\n", ret);
    return 0;
}
```

### BPF Helpers Available in kprobe Programs

```c
/* Available BPF helpers in kprobe context */

bpf_get_current_pid_tgid()     // current task PID/TGID
bpf_get_current_comm()          // current task name
bpf_get_current_task()          // struct task_struct * (as u64)
bpf_probe_read_kernel()         // safe kernel memory read
bpf_probe_read_user()           // safe user memory read
bpf_probe_read_kernel_str()     // safe kernel string read
bpf_perf_event_output()         // output to perf ring buffer
bpf_ringbuf_output()            // output to BPF ring buffer
bpf_map_lookup_elem()           // map access
bpf_map_update_elem()           // map write
bpf_ktime_get_ns()              // nanosecond timestamp
bpf_get_stackid()               // capture stack trace
bpf_send_signal()               // send signal to current task
bpf_override_return()           // override return value (requires CONFIG_BPF_KPROBE_OVERRIDE)
```

### BCC (BPF Compiler Collection) Example

```python
#!/usr/bin/env python3
# trace_open.py: Trace all open() syscalls using kprobes via BCC

from bcc import BPF

bpf_code = """
#include <uapi/linux/ptrace.h>
#include <linux/fs.h>

struct data_t {
    u32 pid;
    char comm[TASK_COMM_LEN];
    char filename[256];
    long ret;
    u64 latency_ns;
};

BPF_HASH(start_ts, u64);
BPF_PERF_OUTPUT(events);

int trace_openat_entry(struct pt_regs *ctx,
                       int dfd, const char __user *filename) {
    u64 id = bpf_get_current_pid_tgid();
    u64 ts = bpf_ktime_get_ns();
    start_ts.update(&id, &ts);
    return 0;
}

int trace_openat_return(struct pt_regs *ctx) {
    u64 id    = bpf_get_current_pid_tgid();
    u64 *tsp  = start_ts.lookup(&id);
    if (!tsp) return 0;

    struct data_t data = {};
    data.pid        = id >> 32;
    data.latency_ns = bpf_ktime_get_ns() - *tsp;
    data.ret        = PT_REGS_RC(ctx);

    bpf_get_current_comm(&data.comm, sizeof(data.comm));
    events.perf_submit(ctx, &data, sizeof(data));

    start_ts.delete(&id);
    return 0;
}
"""

b = BPF(text=bpf_code)
b.attach_kprobe(event="do_sys_openat2",  fn_name="trace_openat_entry")
b.attach_kretprobe(event="do_sys_openat2", fn_name="trace_openat_return")

def print_event(cpu, data, size):
    event = b["events"].event(data)
    print(f"PID:{event.pid:6} COMM:{event.comm.decode():16} "
          f"RET:{event.ret:6} LAT:{event.latency_ns/1000:.1f}us")

b["events"].open_perf_buffer(print_event)
while True:
    b.perf_buffer_poll()
```

### bpftrace Example

```
# bpftrace inline kprobe examples

# Count vfs_read calls per process
bpftrace -e 'kprobe:vfs_read { @[comm] = count(); }'

# Measure vfs_read latency histogram
bpftrace -e '
kprobe:vfs_read    { @start[tid] = nsecs; }
kretprobe:vfs_read / @start[tid] / {
    @us = hist((nsecs - @start[tid]) / 1000);
    delete(@start[tid]);
}'

# Trace TCP connection attempts
bpftrace -e 'kprobe:tcp_connect {
    printf("TCP connect: pid=%d comm=%s\n", pid, comm);
}'

# Stack trace for kmalloc calls
bpftrace -e 'kprobe:__kmalloc { @[kstack] = count(); }'

# Watch for specific return value
bpftrace -e 'kretprobe:vfs_write / retval < 0 / {
    printf("write failed: pid=%d errno=%d\n", pid, -retval);
}'
```

---

## 18. kprobes via ftrace

### ftrace-backed kprobes

When CONFIG_KPROBES_ON_FTRACE is enabled and a function was compiled with ftrace instrumentation (`-pg` or `-mfentry`), kprobes uses the existing ftrace hook point instead of writing an INT3.

```
Function compiled with ftrace:
  do_sys_open:
    ffffffff81200000: e8 xx xx xx xx   call __fentry__  ← ftrace NOP slot
    ffffffff81200005: 55               push  rbp
    ffffffff81200006: 48 89 e5         mov   rbp, rsp
    ...

Normal (no probes, ftrace disabled):
  ffffffff81200000: 0f 1f 44 00 00   nop5  ← ftrace converts call to NOP

With ftrace-backed kprobe:
  ffffffff81200000: e8 xx xx xx xx   call ftrace_trampoline ← re-enabled
    ↓
  ftrace_trampoline:
    call kprobe_ftrace_handler()  ← kprobe fires via ftrace
```

This eliminates the need for INT3 and single-stepping, reducing overhead from ~300ns to ~50ns.

### Checking if a Probe Uses ftrace

```bash
# In /sys/kernel/debug/kprobes/list:
# The 'F' flag indicates ftrace-backed probe
cat /sys/kernel/debug/kprobes/list
# Output:
# ffffffff8120ab40  k  do_sys_open+0x0    [DISABLED]
# ffffffff81456780  k  tcp_connect+0x0    [FTRACE]    ← ftrace-backed
```

---

## 19. kprobes via perf_events

### The tracefs kprobe_events Interface

The Linux kernel exposes kprobes through the `tracefs` (usually mounted at `/sys/kernel/debug/tracing` or `/sys/kernel/tracing`) via the `kprobe_events` file. This is the **perf** and **ftrace** integration point.

```bash
# === Setting up kprobes via tracefs ===

# Mount tracefs (if not already)
mount -t tracefs tracefs /sys/kernel/tracing

# Add a kprobe event: trace do_sys_open with fd return value
echo 'p:my_open do_sys_open dfd=%di filename=%si:string' \
    > /sys/kernel/tracing/kprobe_events

# Add a kretprobe: trace return value of vfs_read
echo 'r:my_read_ret vfs_read retval=$retval' \
    >> /sys/kernel/tracing/kprobe_events

# See registered probes
cat /sys/kernel/tracing/kprobe_events
# p:kprobes/my_open do_sys_open dfd=%di filename=%si:string
# r:kprobes/my_read_ret vfs_read retval=$retval

# Enable tracing
echo 1 > /sys/kernel/tracing/events/kprobes/my_open/enable
echo 1 > /sys/kernel/tracing/events/kprobes/my_read_ret/enable

# Start tracing
echo 1 > /sys/kernel/tracing/tracing_on

# Read events
cat /sys/kernel/tracing/trace
#          <idle>-0       [000] d... 12345.678901: my_open: ...
#           bash-1234     [001] d... 12345.679012: my_read_ret: retval=128

# Cleanup
echo 0 > /sys/kernel/tracing/tracing_on
echo '-:my_open'     >> /sys/kernel/tracing/kprobe_events
echo '-:my_read_ret' >> /sys/kernel/tracing/kprobe_events
```

### kprobe_events Syntax Reference

```
Probe definition syntax:
  p[:[GRP/]EVENT] SYMBOL[+OFFS]|MEMADDR [FETCHARGS]   (kprobe)
  r[MAXACTIVE][:[GRP/]EVENT] SYMBOL[+0] [FETCHARGS]   (kretprobe)

Fetch arguments (FETCHARGS):
  %REG                  CPU register (e.g., %ax, %di, %si)
  @ADDR                 Memory address (kernel)
  @SYM[+|-OFFS]         Symbol + offset
  $stackN               N-th entry on stack
  $stack                Current stack pointer
  $retval               Return value (kretprobe only)
  $comm                 Current task name
  +OFFS(FETCHARG)       Dereference: *(FETCHARG + OFFS)
  \IMM                  Immediate value

Type suffixes:
  :u8  :u16  :u32  :u64      Unsigned integer
  :s8  :s16  :s32  :s64      Signed integer
  :x8  :x16  :x32  :x64      Hex integer
  :string                    NULL-terminated string (user or kernel)
  :ustring                   User-space string
  :bitfield(W,O,S)           Bit field: Width, Offset, Size

Examples:
  p:myprobe vfs_write file=%di buf=%si count=%dx:u64
  p:myprobe do_sys_open+0x20 %rax:x64
  r:myret   tcp_sendmsg $retval:s64
  p:myprobe sys_read +0(%di):string  ← dereference arg
```

### Using perf with kprobes

```bash
# Record kprobe events with perf

# Add probe: trace do_sys_open with filename argument
perf probe --add 'do_sys_open filename:string'

# List available probes
perf probe --list

# Record with kprobe
perf record -e 'probe:do_sys_open' -a sleep 10

# Annotate
perf report

# Remove probe
perf probe --del probe:do_sys_open

# Probe at source line (requires debug symbols)
perf probe --add 'net/socket.c:234'

# Probe at function + offset
perf probe --add 'tcp_sendmsg+20'

# Record kretprobe
perf probe --add 'vfs_read%return retval=$retval'
perf record -e 'probe:vfs_read__return' -a sleep 5
perf script
```

---

## 20. The /sys/kernel/debug/kprobes Interface

```bash
# List all registered kprobes
cat /sys/kernel/debug/kprobes/list

# Output format:
# ADDRESS          TYPE  SYMBOL+OFFSET  [FLAGS]
# ffffffff81234567 k     vfs_read+0x0
# ffffffff81456789 r     tcp_connect+0x0  [DISABLED]
# ffffffff81678901 k     do_sys_open+0x0  [OPTIMIZED]

# Legend:
#   k  = kprobe
#   r  = kretprobe
#   j  = jprobe (removed in 4.15)
#   [DISABLED]   = probe registered but not active
#   [OPTIMIZED]  = using JMP optimization instead of INT3
#   [FTRACE]     = using ftrace hook instead of INT3

# Check if kprobes are enabled
cat /sys/kernel/debug/kprobes/enabled
# 1 = enabled, 0 = disabled

# Globally disable all kprobes
echo 0 > /sys/kernel/debug/kprobes/enabled

# Re-enable
echo 1 > /sys/kernel/debug/kprobes/enabled

# Blacklist (functions that cannot be probed)
cat /sys/kernel/debug/kprobes/blacklist
# ffffffff81001000-ffffffff81002000    ← address ranges
# kprobe_int3_handler
# kprobe_debug_handler
# ...
```

---

## 21. kprobes and KASLR

### The KASLR Challenge

KASLR (Kernel Address Space Layout Randomization) randomizes the base address at which the kernel is loaded on each boot. This means:

```
Boot 1: kernel base = 0xffffffff81000000
  vfs_read at:          0xffffffff81a23456

Boot 2: kernel base = 0xffffffff82000000
  vfs_read at:          0xffffffff82a23456
```

### How kprobes Handles KASLR

When you specify `symbol_name = "vfs_read"`, kprobes uses `kallsyms_lookup_name()` which already accounts for KASLR — it returns the **runtime address**, not the compile-time address.

```c
/* kernel/kprobes.c */

kprobe_opcode_t *kprobe_lookup_name(const char *name, unsigned int offset)
{
    kprobe_opcode_t *addr;

#ifdef CONFIG_FUNCTION_GRAPH_TRACER
    /* ftrace sets up function graph entries at offset 0 */
    /* Need special handling */
#endif

    addr = (kprobe_opcode_t *)kallsyms_lookup_name(name);
    if (addr)
        addr = (kprobe_opcode_t *)correct_address(addr);  /* +KASLR offset */

    return addr + offset;
}
```

### KASLR and Raw Address Probing

If you probe by raw address (not symbol name), you must account for KASLR yourself:

```c
/* WRONG: hardcoded address ignores KASLR */
kp.addr = (kprobe_opcode_t *)0xffffffff81200000;

/* RIGHT: use kallsyms at runtime */
kp.addr = (kprobe_opcode_t *)kallsyms_lookup_name("vfs_read");

/* OR: Read from /proc/kallsyms in userspace and pass via module parameter */
static ulong probe_addr = 0;
module_param(probe_addr, ulong, 0);

/* In init: */
kp.addr = (kprobe_opcode_t *)probe_addr;
/* User loads module with: insmod mymod.ko probe_addr=0xffffffff81200000 */
/* where 0xffffffff81200000 was read from /proc/kallsyms */
```

---

## 22. kprobes and Kernel Live Patching

### Interaction Between kprobes and livepatch

Kernel live patching (introduced in Linux 4.0) also modifies kernel text at runtime. Both kprobes and livepatch use `text_poke_bp()` and must coordinate.

```
Conflict scenario:
  livepatch patches function X → installs new function body
  kprobe at function X → INT3 at old body

Resolution:
  1. kprobes are aware of ftrace stubs
  2. livepatch works via ftrace (replaces ftrace trampoline target)
  3. kprobes at ftrace-instrumented functions use ftrace path
  → No direct conflict for ftrace-instrumented functions

For non-ftrace functions:
  kprobes registers first → writes INT3
  livepatch writes new body → may overwrite INT3
  This is a known limitation: probing live-patched functions is unsafe
```

### kprobes During Module Loading/Unloading

```
Problem: A kprobe is registered on a function in loadable module A.
         Module A gets unloaded.
         kprobe handler now calls into freed memory → CRASH

Solution implemented in kernel:
  1. kprobes tracks which module each probe is in
  2. On module unload (MODULE_STATE_GOING):
     - All kprobes in that module are forcefully unregistered
     - Handlers referencing that module's code are also disabled
  3. register_kprobe() checks MODULE_STATE at registration time
```

---

## 23. Multi-probe Scenarios and Aggregation

### Aggregated kprobes

When multiple kprobes are registered at the same address, the kernel creates an **aggregated kprobe** that acts as a dispatcher:

```
Address 0xffffffff812345ab:
  [INT3]  ← single breakpoint, shared

  On hit → aggr_pre_handler():
    for each sub-probe in list:
      if sub-probe is enabled:
        call sub_probe->pre_handler()

  After single-step → aggr_post_handler():
    for each sub-probe:
      call sub_probe->post_handler()
```

```c
/* kernel/kprobes.c — simplified */

static int aggr_pre_handler(struct kprobe *p, struct pt_regs *regs)
{
    struct kprobe *kp;

    list_for_each_entry_rcu(kp, &p->list, list) {
        if (kp->pre_handler && likely(!kprobe_disabled(kp))) {
            set_kprobe_instance(kp);
            if (kp->pre_handler(kp, regs))
                return 1;  /* halt further processing */
        }
    }
    return 0;
}
```

### probe_count and nmissed

```c
/*
 * kretprobe.nmissed:
 *   Incremented when a function call occurs but no free instance
 *   is available (all maxactive slots are in use).
 *   Indicates you need to increase maxactive.
 */

/*
 * kprobe.nmissed (for aggregated probes):
 *   Count of times the probe was hit while another CPU was
 *   already executing the handler (reentrancy guard fired).
 */
```

---

## 24. Safety, Reentrancy, and Locking

### Reentrancy Protection

A fundamental safety property: kprobes must not probe themselves. The per-CPU `kprobe_ctlblk` tracks whether we're already inside a kprobe handler:

```c
/* arch/x86/kernel/kprobes/core.c */

int kprobe_int3_handler(struct pt_regs *regs)
{
    struct kprobe_ctlblk *kcb = get_kprobe_ctlblk();

    /*
     * If we're already in a kprobe handler (nested kprobe),
     * we must handle this carefully to avoid infinite recursion.
     */
    if (kcb->kprobe_status == KPROBE_HIT_ACTIVE) {
        /* This is a re-entrant probe hit */
        save_previous_kprobe(kcb);       /* push current state */
        set_kprobe_instance(p);
        /* Handle with reduced safety */
        kp->nmissed++;
        return 1;
    }

    kcb->kprobe_status = KPROBE_HIT_ACTIVE;
    /* ... normal handling ... */
}
```

### Memory Access in Handlers

Accessing kernel memory from probe handlers is generally safe, but accessing **user memory** is dangerous because:

1. User pages might be swapped out → page fault → sleep → NOT ALLOWED in atomic context
2. Probing a pagefault handler → recursive fault

Solution: Use `probe_kernel_read()` / `probe_user_read()` which handle faults gracefully:

```c
static int my_handler(struct kprobe *p, struct pt_regs *regs)
{
    struct file *file = (struct file *)regs->di;
    char buf[256];
    long ret;

    /* Safe kernel memory read: returns -EFAULT on bad access */
    ret = probe_kernel_read(buf, file->f_path.dentry->d_name.name,
                            sizeof(buf));
    if (ret < 0) {
        pr_warn("probe_kernel_read failed: %ld\n", ret);
        return 0;
    }

    pr_info("filename: %s\n", buf);
    return 0;
}
```

### Locking Rules in Handlers

```
┌─────────────────────────────────────────────────────────────────┐
│                    Locking in kprobe Handlers                   │
│                                                                 │
│  Context: interrupts DISABLED, preemption DISABLED              │
│                                                                 │
│  Rule 1: Never acquire a mutex or semaphore                     │
│    BAD:  mutex_lock(&my_mutex);                                 │
│    GOOD: spin_lock(&my_spinlock);                               │
│                                                                 │
│  Rule 2: Use IRQ-safe spinlocks                                 │
│    GOOD: spin_lock_irqsave(&lock, flags);                       │
│    NOTE: irqsave is redundant (IRQs already off) but harmless   │
│                                                                 │
│  Rule 3: Avoid locks already held on probe path                 │
│    If probing tcp_sendmsg which holds sk_lock,                  │
│    don't try to acquire sk_lock in handler → DEADLOCK           │
│                                                                 │
│  Rule 4: Per-CPU variables are safe (CPU pinned, no migration)  │
│    GOOD: this_cpu_inc(my_counter);                              │
│                                                                 │
│  Rule 5: RCU read-side critical sections are safe               │
│    GOOD: rcu_read_lock(); ... rcu_read_unlock();                │
└─────────────────────────────────────────────────────────────────┘
```

### Exception Handling in Handlers

If your handler causes a fault (e.g., accessing invalid memory), kprobes has a fault handler:

```c
/* arch/x86/kernel/kprobes/core.c */

int kprobe_fault_handler(struct pt_regs *regs, int trapnr)
{
    struct kprobe *cur = kprobe_running();

    if (cur && cur->fault_handler) {
        /* Call user-provided fault handler */
        if (cur->fault_handler(cur, regs, trapnr))
            return 1;  /* Handled */
    }

    /* If fault in kprobe handler, use exception table to recover */
    if (kprobe_status == KPROBE_HIT_SS) {
        /* Fault during single-step: skip the instruction */
        resume_execution(cur, regs, kcb);
        return 1;
    }

    return 0;
}
```

---

## 25. Performance Impact and Overhead Analysis

### Overhead Breakdown

```
┌─────────────────────────────────────────────────────────────────┐
│                   kprobe Overhead Analysis                      │
│                                                                 │
│  Mechanism              Overhead per hit    Notes               │
│  ─────────────────────  ─────────────────   ──────────────────  │
│  INT3 + single-step     300-1000 ns         With handler        │
│  INT3 (no-op handler)   ~100-200 ns         Breakpoint only     │
│  JMP optimization       ~50-150 ns          With handler        │
│  ftrace-backed          ~30-80 ns           With handler        │
│  fprobe                 ~10-50 ns           Function entry/ret  │
│  eBPF via kprobe        ~100-400 ns         With BPF overhead   │
│                                                                 │
│  For comparison:                                                │
│  Regular function call  ~2-5 ns             No probe            │
│  L1 cache miss          ~4 ns               Reference           │
│  L3 cache miss          ~40 ns              Reference           │
└─────────────────────────────────────────────────────────────────┘
```

### Cache Effects

```
Without probe:
  CPU fetches instruction at addr → L1 I-cache hit → execute

With probe (INT3):
  CPU fetches 0xCC at addr → executes INT3 → exception
    ↓
  Exception handler path runs → ALL L1 D/I caches affected
  Hash table lookup → D-cache miss (cold probe table)
  Handler code runs → I-cache pollution
  Single-step path runs → D-cache dirty
    ↓
  Resume → return to caller → instruction cache partially invalidated
```

### Measuring kprobe Overhead

```c
/* Measure probe overhead with TSC */
static u64 before_tsc, total_overhead;
static u64 hit_count;

static int overhead_pre(struct kprobe *p, struct pt_regs *regs)
{
    before_tsc = rdtsc();
    return 0;
}

static void overhead_post(struct kprobe *p, struct pt_regs *regs,
                          unsigned long flags)
{
    u64 elapsed = rdtsc() - before_tsc;
    atomic64_add(elapsed, &total_overhead_atomic);
    atomic64_inc(&hit_count_atomic);
}
```

### When Probes Are Disabled

When a kprobe is disabled (via `disable_kprobe()`), the INT3 is replaced back with the original instruction. The probe exists in the hash table but is not armed:

```
Disabled probe: O(1) overhead = zero (original instruction executes normally)
```

### Profiling Rule of Thumb

```
Probe on function called:
  < 1,000/sec      → negligible overhead
  < 100,000/sec    → acceptable (trace_printk or BPF map)
  < 1,000,000/sec  → moderate (use per-CPU counters, no printk)
  > 1,000,000/sec  → significant overhead; use ftrace/fprobe instead
  > 10,000,000/sec → consider kernel-side aggregation only (histograms)
```

---

## 26. Real-World Use Cases

### Use Case 1: Security Monitoring (EDR/XDR)

```c
/* Monitor privilege escalation attempts */

static int setuid_monitor(struct kprobe *p, struct pt_regs *regs)
{
    /* __x64_sys_setuid(uid_t uid) */
    uid_t new_uid = (uid_t)regs->di;
    uid_t cur_uid = current_uid().val;

    if (cur_uid != 0 && new_uid == 0) {
        pr_alert("SECURITY: PID %d (%s) attempting setuid(0) from uid %d\n",
                 current->pid, current->comm, cur_uid);
        /* Could: send to audit, kill process, increment counter */
    }
    return 0;
}
```

### Use Case 2: Debugging Memory Leaks

```c
/* Track kmalloc/kfree pairs to find kernel memory leaks */

static DEFINE_HASHTABLE(alloc_table, 16);
DEFINE_SPINLOCK(alloc_lock);

struct alloc_entry {
    void *ptr;
    size_t size;
    unsigned long stack[8];
    struct hlist_node node;
};

static int kmalloc_ret_handler(struct kretprobe_instance *ri,
                                struct pt_regs *regs)
{
    void *ptr = (void *)regs_return_value(regs);
    if (!ptr) return 0;

    struct alloc_entry *entry = kmalloc(sizeof(*entry), GFP_ATOMIC);
    if (!entry) return 0;

    entry->ptr = ptr;
    /* save_stack_trace() to capture who allocated */

    spin_lock(&alloc_lock);
    hash_add(alloc_table, &entry->node, (u64)ptr);
    spin_unlock(&alloc_lock);
    return 0;
}

static int kfree_handler(struct kprobe *p, struct pt_regs *regs)
{
    void *ptr = (void *)regs->di;

    spin_lock(&alloc_lock);
    /* find and remove entry for ptr */
    spin_unlock(&alloc_lock);
    return 0;
}
```

### Use Case 3: System Call Auditing

```bash
# Using kprobe_events to audit all exec() calls
echo 'p:audit_exec do_execveat_common filename=+0(+16(%si)):string' \
    > /sys/kernel/tracing/kprobe_events
echo 1 > /sys/kernel/tracing/events/kprobes/audit_exec/enable
cat /sys/kernel/tracing/trace_pipe
```

### Use Case 4: Network Packet Tracing

```c
/* Trace every packet through the IP stack */
static struct kprobe ip_rcv_probe = {
    .symbol_name = "ip_rcv",
    .pre_handler = trace_ip_rcv,
};

static int trace_ip_rcv(struct kprobe *p, struct pt_regs *regs)
{
    struct sk_buff *skb = (struct sk_buff *)regs->di;
    struct iphdr *iph;

    /* probe_kernel_read for safety */
    iph = ip_hdr(skb);
    pr_info("IP: %pI4 → %pI4 proto=%d len=%d\n",
            &iph->saddr, &iph->daddr, iph->protocol,
            ntohs(iph->tot_len));
    return 0;
}
```

### Use Case 5: Performance Profiling

```bash
# Find the top 10 slowest vfs_write calls (using bpftrace)
bpftrace -e '
kprobe:vfs_write    { @start[tid] = nsecs; }
kretprobe:vfs_write / @start[tid] / {
    $lat = (nsecs - @start[tid]) / 1000;
    if ($lat > 1000) {  /* > 1ms */
        printf("SLOW write: pid=%d lat=%ldus stack:\n", pid, $lat);
        print(kstack);
    }
    delete(@start[tid]);
}
'
```

### Use Case 6: Filesystem I/O Tracing

```bash
# Complete I/O trace: who reads what, how big, how slow
bpftrace -e '
struct file;

kprobe:vfs_read {
    @fname[tid] = str(((struct file *)arg0)->f_path.dentry->d_name.name);
    @start[tid] = nsecs;
}
kretprobe:vfs_read /@start[tid]/ {
    printf("READ: %s pid=%d size=%d lat=%ldus\n",
           @fname[tid], pid, retval,
           (nsecs - @start[tid]) / 1000);
    delete(@start[tid]); delete(@fname[tid]);
}
'
```

---

## 27. Comparison with Other Tracing Mechanisms

### Complete Comparison Matrix

```
┌──────────────────┬──────────┬─────────┬──────────┬──────────────┬──────────┐
│ Mechanism        │ Granular │ Overhead│ Safety   │ Kernel recomp│ Scope    │
├──────────────────┼──────────┼─────────┼──────────┼──────────────┼──────────┤
│ kprobes          │ Instr-   │ Medium  │ Good     │ No           │ Any insn │
│                  │ level    │(~300ns) │          │              │          │
├──────────────────┼──────────┼─────────┼──────────┼──────────────┼──────────┤
│ kretprobes       │ Function │ Medium  │ Good     │ No           │ Fn exit  │
│                  │ return   │(~400ns) │          │              │          │
├──────────────────┼──────────┼─────────┼──────────┼──────────────┼──────────┤
│ fprobe           │ Function │ Low     │ Good     │ No           │ Fn entry │
│                  │ entry/ret│(~50ns)  │          │              │ /exit    │
├──────────────────┼──────────┼─────────┼──────────┼──────────────┼──────────┤
│ ftrace           │ Function │ Low     │ Good     │ No (needs -pg│ Fn entry │
│                  │ entry    │(~20ns)  │          │ compile flag)│          │
├──────────────────┼──────────┼─────────┼──────────┼──────────────┼──────────┤
│ tracepoints      │ Static   │ Minimal │ Excellent│ No (runtime  │ Fixed    │
│                  │ sites    │(~10ns)  │          │ enable/dis.) │ sites    │
├──────────────────┼──────────┼─────────┼──────────┼──────────────┼──────────┤
│ Hardware BPs     │ Instr/   │ Minimal │ Limited  │ No           │ 4 max    │
│ (DR0-DR3)        │ Data     │(~5ns)   │          │              │ globally │
├──────────────────┼──────────┼─────────┼──────────┼──────────────┼──────────┤
│ printk()         │ Explicit │ High    │ Good     │ YES (source  │ Where    │
│                  │ code     │(~10us)  │          │ change req.) │ inserted │
├──────────────────┼──────────┼─────────┼──────────┼──────────────┼──────────┤
│ KGDB             │ Debugger │ Halts   │ Poor     │ No           │ Full     │
│                  │ level    │ CPU     │ (stops   │              │ debug    │
│                  │          │         │  system) │              │ access   │
├──────────────────┼──────────┼─────────┼──────────┼──────────────┼──────────┤
│ eBPF (kprobe)    │ Instr-   │ Medium  │ Excellent│ No           │ Any fn   │
│                  │ level    │(~200ns) │ (verifier│              │ + instr  │
│                  │          │         │  sandbox)│              │          │
└──────────────────┴──────────┴─────────┴──────────┴──────────────┴──────────┘
```

### When to Choose What

```
Choose kprobes when:
  ✓ Need arbitrary instruction-level precision
  ✓ Need to probe a specific non-entry offset
  ✓ Building a kernel module (not user-space BPF tool)
  ✓ Need to modify register state on probe hit
  ✓ Prototype/debug scenario in development

Choose eBPF kprobes when:
  ✓ Production environment (eBPF verifier provides safety)
  ✓ Need user-space analysis/aggregation
  ✓ Need maps, ring buffers, multi-probe correlation
  ✓ Function entry/return is sufficient
  ✓ Team has BPF tooling (BCC, bpftrace, libbpf)

Choose fprobe/ftrace when:
  ✓ Function entry/return is sufficient
  ✓ Lowest possible overhead is critical
  ✓ High-frequency functions (>1M/sec)

Choose tracepoints when:
  ✓ The tracepoint already exists
  ✓ Need stable, API-stable instrumentation
  ✓ Lowest overhead
  ✓ Distributable (tracepoints are ABI-stable, kprobes are not)

Choose hardware breakpoints when:
  ✓ Need to catch memory access (watchpoints: read/write)
  ✓ Need absolute minimal overhead
  ✓ Need only 1-4 probes total
```

---

## 28. Limitations and Caveats

### 1. No API Stability Guarantee

kprobes probe **kernel internal functions** — not the stable syscall ABI. Any kernel update can:
- Rename the function
- Merge it into another function
- Inline it (making it disappear as a symbol)
- Change argument order or types
- Remove it entirely

**Implication**: kprobe-based tools must be versioned with kernel versions.

### 2. Inlined Functions Cannot Be Probed

```c
/* In kernel source: */
static inline int __must_check may_open(...)
{
    /* ... */
}

/* This is inlined — no symbol in kallsyms — CANNOT probe */
kallsyms_lookup_name("may_open");  /* Returns 0 (not found) */
```

**Workaround**: Probe the caller instead, or find a non-inlined callee.

### 3. __init Functions

Functions marked `__init` are freed after kernel initialization:

```c
static int __init my_init_function(void)
{
    /* Only runs once, then memory freed */
}
```

Probing `__init` functions only works during the boot window before initcall completion.

### 4. kprobes Cannot Probe All Instructions

The following cannot be probed on x86:

```
- The INT3 instruction itself (0xCC) — would cause infinite recursion
- iret (may be in exception path)
- hlt
- Instructions in kprobes' own handlers (__kprobes section)
- NMI handler code
- Some SMM-related code
- Instructions spanning page boundaries (architectural constraint)
```

### 5. No Guaranteed Consistency of Arguments

At a non-entry point in a function, registers may have been reused by the compiler. This makes probing mid-function unreliable for argument recovery:

```
void foo(int a, int b) {
    // RDI = a, RSI = b at entry
    int c = a + b;
    // Now RDI might hold c, RSI is gone
    bar(c);
    // Probe here: you cannot recover 'a' or 'b' from registers
}
```

### 6. maxactive and Missed Probes (kretprobe)

```
If kretprobe.maxactive = 20 and 21 simultaneous calls occur:
  21st call: kretprobe_instance unavailable
  → kretprobe.nmissed++
  → 21st call NOT traced

Fix: increase maxactive. Rule of thumb:
  maxactive = max_concurrent_calls * 2
  Default: max(10, 2 * NR_CPUS)
```

### 7. Probe of Probe-Patching Code

If you try to probe `text_poke()`, `patch_text()`, or similar code-patching functions, you risk:

1. Probing the function that installs probes
2. Creating a race between the probe installation and the probe hit
3. Potential corruption of the instruction stream

These are blacklisted.

### 8. Module Unload Race

```
Scenario:
  Thread 1: executing kprobe pre_handler() in module A
  Thread 2: rmmod module_A

  Without protection: Thread 1 continues into freed module code → UAF

Kernel protection:
  try_module_get() before handler
  module_put() after handler
  This prevents unload while handler is running
```

---

## 29. Debugging kprobes Themselves

### Verifying Probe Registration

```bash
# Check if probe is registered
cat /sys/kernel/debug/kprobes/list | grep my_symbol

# Check kernel log for errors
dmesg | grep -E "kprobe|register_kprobe"

# Check kallsyms to verify symbol exists
grep my_symbol /proc/kallsyms

# Verify symbol is in text (not __init, not inlined)
readelf -s /sys/kernel/vmlinux | grep my_symbol
# T = text (good), W = weak, t = static text (good)
# Absent = inlined or __init (gone)
```

### Common Error Codes

```
register_kprobe() return values:
  0           Success
  -ENOENT     Symbol not found (check spelling, check /proc/kallsyms)
  -EINVAL     Address invalid (blacklisted, not in .text, bad instruction)
  -EEXIST     Duplicate probe at address (use aggregation API)
  -ENOMEM     No memory for instruction slot
  -EOPNOTSUPP Architecture doesn't support probing at this location
  -ENOSPC     No instruction slots available (increase MAX_INSN_SLOTS)
```

### Enabling kprobes Debug Output

```bash
# Dynamic debug: enable kprobes debugging messages
echo "module kprobes +p" > /sys/kernel/debug/dynamic_debug/control

# Or via kernel boot parameter:
# dyndbg="module kprobes +p"
```

### Testing Your Probe

```c
/* Simple test: probe a function you can call yourself */

static atomic_t hit_count = ATOMIC_INIT(0);

static int test_pre_handler(struct kprobe *p, struct pt_regs *regs)
{
    atomic_inc(&hit_count);
    return 0;
}

static struct kprobe test_kp = {
    .symbol_name = "schedule",  /* Called frequently — easy to test */
    .pre_handler = test_pre_handler,
};

static int __init test_init(void)
{
    int ret = register_kprobe(&test_kp);
    if (ret)
        return ret;

    /* Force a schedule() call */
    schedule();
    msleep(100);  /* More schedules */

    pr_info("hit_count = %d (should be > 0)\n", atomic_read(&hit_count));
    return 0;
}
```

### Crash Analysis When a Probe Goes Wrong

If a kprobe causes a kernel panic, the oops message will show:

```
BUG: unable to handle kernel NULL pointer dereference at 0000000000000000
RIP: 0010:handler_pre+0x2a/0x50 [mymodule]
...
Call Trace:
 kprobe_int3_handler+0x8f/0x110
 do_int3+0x3e/0xa0
 int3+0x27/0x40
 vfs_read+0x0/0x170       ← the probed function
```

Key things to check:
1. `handler_pre+0x2a` — exact offset in your handler that crashed
2. The probed function in the call trace confirms which probe fired
3. Register dump shows values at crash time

---

## 30. Complete Reference Summary

### API Quick Reference

```c
/* === Registration / Unregistration === */

int  register_kprobe(struct kprobe *kp);
void unregister_kprobe(struct kprobe *kp);

int  register_kprobes(struct kprobe **kps);     /* array, NULL-terminated */
void unregister_kprobes(struct kprobe **kps, int num);

int  register_kretprobe(struct kretprobe *rp);
void unregister_kretprobe(struct kretprobe *rp);

int  register_kretprobes(struct kretprobe **rps, int num);
void unregister_kretprobes(struct kretprobe **rps, int num);

/* === Enable / Disable === */

int  enable_kprobe(struct kprobe *kp);
int  disable_kprobe(struct kprobe *kp);
int  enable_kretprobe(struct kretprobe *rp);
int  disable_kretprobe(struct kretprobe *rp);

/* === Query === */

struct kprobe *get_kprobe(void *addr);          /* NULL if none at addr */
bool kprobe_running(void);                       /* true if in handler */
bool kprobes_built_in(void);                     /* CONFIG_KPROBES set? */

/* === Utility === */

unsigned long kallsyms_lookup_name(const char *name);  /* symbol → addr */
void *kprobe_get_insn_slot(void);               /* allocate slot manually */
void kprobe_free_insn_slot(void *slot, int dirty);

/* === Blacklist Annotation === */

#define NOKPROBE_SYMBOL(fname) /* Add fname to kprobe blacklist */
__attribute__((__section__(".kprobes.text"))) __kprobes
nokprobe_inline
```

### kprobe Flags

```c
/* Flags for kprobe.flags field */
#define KPROBE_FLAG_GONE        BIT(0)  /* Unregistered, not freed yet */
#define KPROBE_FLAG_DISABLED    BIT(1)  /* Disabled via disable_kprobe() */
#define KPROBE_FLAG_OPTIMIZED   BIT(2)  /* JMP optimization active */
#define KPROBE_FLAG_FTRACE      BIT(3)  /* Backed by ftrace hook */
#define KPROBE_FLAG_ON_FUNC_ENTRY BIT(4) /* Probe is at function start */
```

### Kernel Config Options

```
CONFIG_KPROBES                  # Enable kprobes (mandatory)
CONFIG_KPROBES_ON_FTRACE        # Allow ftrace-backed kprobes
CONFIG_OPTPROBES                # Enable JMP optimization
CONFIG_KRETPROBES               # Enable kretprobes
CONFIG_HAVE_KPROBES_ON_FTRACE   # Arch support for ftrace kprobes
CONFIG_KPROBE_EVENTS            # tracefs kprobe_events interface
CONFIG_BPF_EVENTS               # eBPF + kprobe events
CONFIG_FPROBE                   # Enable fprobe (ftrace-based)
CONFIG_FUNCTION_TRACER          # ftrace (dependency for KPROBES_ON_FTRACE)
CONFIG_DEBUG_INFO               # DWARF debug info (for perf probe --add)
CONFIG_KALLSYMS                 # /proc/kallsyms (for symbol lookup)
CONFIG_KALLSYMS_ALL             # All symbols in kallsyms (not just exported)
```

### Registers Quick Reference (x86-64)

```
pt_regs field   Register   Linux syscall   C func arg   Notes
─────────────   ─────────  ──────────────  ────────────  ──────────────
regs->di        RDI        arg1 (rdi)      arg1          Dest index
regs->si        RSI        arg2 (rsi)      arg2          Src index
regs->dx        RDX        arg3 (rdx)      arg3          Data
regs->cx        RCX        arg4* (r10)     arg4          *syscall uses r10
regs->r8        R8         arg5            arg5
regs->r9        R9         arg6            arg6
regs->ax        RAX        return value    return value  Also: syscall num
regs->ip        RIP        —               —             Instruction ptr
regs->sp        RSP        —               —             Stack pointer
regs->bp        RBP        —               —             Base pointer
regs->flags     EFLAGS     —               —             CPU flags
regs->orig_ax   orig_RAX   syscall number  —             Original syscall#
```

### Macros for Argument Access

```c
/* Architecture-portable way to read function arguments */
#include <linux/ptrace.h>

/* regs_get_kernel_argument(regs, n) — get Nth function arg (0-indexed) */
unsigned long arg0 = regs_get_kernel_argument(regs, 0);  /* first arg */
unsigned long arg1 = regs_get_kernel_argument(regs, 1);  /* second arg */

/* Return value from kretprobe */
long retval = regs_return_value(regs);  /* RAX on x86 */
```

### Complete State Machine of a kprobe

```
         register_kprobe()
              │
              ▼
        ┌─────────────┐
        │  REGISTERED │  ← INT3/BRK installed at address
        │  (armed)    │     pre/post handlers active
        └──────┬──────┘
               │
        ┌──────┴──────────────────┐
        │                         │
        ▼                         ▼
  disable_kprobe()         try_to_optimize()
        │                         │
        ▼                         ▼
  ┌─────────────┐         ┌───────────────┐
  │  DISABLED   │         │  OPTIMIZED    │  ← JMP installed
  │             │         │               │     (no INT3)
  └──────┬──────┘         └───────┬───────┘
         │                        │
  enable_kprobe()        unoptimize / disable
         │                        │
         └──────────┬─────────────┘
                    │
                    ▼
             unregister_kprobe()
                    │
                    ▼
           ┌────────────────┐
           │    REMOVED     │  ← Original bytes restored
           │                │     Freed from hash table
           └────────────────┘
```

---

## Appendix A: Complete kprobe Module Template

```c
/* kprobe_template.c — Copy-paste starting point */

#include <linux/kernel.h>
#include <linux/module.h>
#include <linux/kprobes.h>
#include <linux/ptrace.h>
#include <linux/sched.h>
#include <linux/slab.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Your Name");
MODULE_DESCRIPTION("kprobe template");

/* ===== Configuration ===== */
#define PROBE_SYMBOL "vfs_read"   /* Function to probe */

/* ===== Per-CPU / atomic state ===== */
static DEFINE_PER_CPU(u64, probe_hit_count);

/* ===== Pre-handler ===== */
static int my_pre_handler(struct kprobe *p, struct pt_regs *regs)
{
    this_cpu_inc(probe_hit_count);

    /* Access arguments (x86-64 ABI) */
    /* arg1=regs->di, arg2=regs->si, arg3=regs->dx, arg4=regs->cx */

    return 0;  /* 0: continue; 1: skip post_handler */
}

/* ===== Post-handler ===== */
static void my_post_handler(struct kprobe *p, struct pt_regs *regs,
                             unsigned long flags)
{
    /* Called after the probed instruction executed */
    /* regs->ax = return value if at function end */
}

/* ===== Fault handler (optional) ===== */
static int my_fault_handler(struct kprobe *p, struct pt_regs *regs, int trapnr)
{
    pr_err("Fault %d in probe handler for %s\n", trapnr, PROBE_SYMBOL);
    return 0;  /* 0: not handled (kernel will deal with it) */
}

/* ===== kprobe structure ===== */
static struct kprobe kp = {
    .symbol_name   = PROBE_SYMBOL,
    .pre_handler   = my_pre_handler,
    .post_handler  = my_post_handler,
    /* .fault_handler = my_fault_handler, */  /* optional */
    /* .offset = 0, */                         /* byte offset into symbol */
};

/* ===== Module init ===== */
static int __init template_init(void)
{
    int ret;

    ret = register_kprobe(&kp);
    if (ret < 0) {
        pr_err("register_kprobe failed for '%s': %d\n", PROBE_SYMBOL, ret);
        if (ret == -ENOENT)
            pr_err("Symbol '%s' not found in kallsyms\n", PROBE_SYMBOL);
        if (ret == -EINVAL)
            pr_err("Address blacklisted or invalid\n");
        return ret;
    }

    pr_info("kprobe planted at %pS (addr=%p)\n", kp.addr, kp.addr);
    pr_info("Flags: optimized=%d ftrace=%d\n",
            !!(kp.flags & KPROBE_FLAG_OPTIMIZED),
            !!(kp.flags & KPROBE_FLAG_FTRACE));
    return 0;
}

/* ===== Module exit ===== */
static void __exit template_exit(void)
{
    int cpu;
    u64 total = 0;

    unregister_kprobe(&kp);

    for_each_possible_cpu(cpu)
        total += per_cpu(probe_hit_count, cpu);

    pr_info("kprobe at %pS unregistered. Total hits: %llu. Missed: %lu\n",
            kp.addr, total, kp.nmissed);
}

module_init(template_init);
module_exit(template_exit);
```

---

## Appendix B: kprobe Internal Call Flow (Complete)

```
register_kprobe(p)
  ├─ check_kprobe_rereg(p)            ← detect duplicate registration
  ├─ kprobe_addr(p)                   ← resolve symbol + offset → addr
  │    ├─ kallsyms_lookup_name()
  │    └─ kprobe_lookup_name()
  ├─ check_kprobe_address_safe(p)     ← blacklist + text check
  │    ├─ within_kprobe_blacklist()
  │    ├─ kernel_text_address()
  │    └─ arch_check_kprobe()
  ├─ prepare_kprobe(p)                ← arch: slot, copy instr
  │    └─ arch_prepare_kprobe()
  │         ├─ get_insn_slot()
  │         ├─ __copy_instruction()
  │         │    ├─ insn_decode()
  │         │    ├─ handle_riprel_insn()
  │         │    └─ synthesize_reljump()
  │         └─ p->opcode = *p->addr
  ├─ (if another probe at addr):
  │    register_aggr_kprobe()
  │         ├─ alloc_aggr_kprobe()
  │         └─ add_new_kprobe()
  ├─ (else):
  │    hlist_add_head_rcu()           ← add to hash table
  ├─ arm_kprobe(p)
  │    └─ arch_arm_kprobe(p)
  │         └─ text_poke_bp(p->addr, INT3, 1, NULL)
  │              ├─ poke_int3_handler  ← atomic: pause CPUs, write, resume
  │              └─ smp_wmb()
  └─ try_to_optimize_kprobe(p)        ← async: maybe convert to JMP
       └─ schedule_delayed_work(&optimizing_work, ...)
```

---

## Appendix C: Glossary

| Term | Meaning |
|------|---------|
| **kprobe** | A dynamic kernel probe at a specific instruction address |
| **kretprobe** | A probe that fires when a probed function returns |
| **fprobe** | Fast probe using ftrace hooks (function entry/exit only) |
| **pre_handler** | Callback invoked before the probed instruction executes |
| **post_handler** | Callback invoked after the probed instruction executes |
| **INT3** | x86 breakpoint instruction (0xCC), 1 byte |
| **BRK #4** | ARM64 breakpoint instruction, 4 bytes |
| **TF** | Trap Flag in EFLAGS; causes #DB after each instruction |
| **Single-step** | Execute exactly one instruction, then trap |
| **Instruction slot** | Memory buffer where original instruction is copied for execution |
| **Jump optimization** | Replace INT3 + single-step with a direct JMP to detour buffer |
| **Aggregated kprobe** | Internal proxy kprobe when multiple probes share an address |
| **nmissed** | Count of probe hits that were skipped (reentrancy / no instances) |
| **KASLR** | Kernel Address Space Layout Randomization; randomizes kernel load address |
| **kallsyms** | Kernel symbol table embedded in the kernel image |
| **kprobe_insn_cache** | Per-arch cache of instruction slot pages |
| **text_poke_bp** | Kernel function for safely patching live kernel text |
| **pt_regs** | Saved CPU register state at exception/interrupt entry |
| **tracefs** | Virtual filesystem for kernel tracing (usually at /sys/kernel/tracing) |
| **kprobe_events** | tracefs interface for creating kprobes from userspace |
| **BPF_PROG_TYPE_KPROBE** | eBPF program type that attaches to kprobes |
| **NOKPROBE_SYMBOL** | Macro to blacklist a symbol from being probed |
| **__kprobes** | GCC attribute placing function in .kprobes.text (non-probeable) |
| **RCU** | Read-Copy-Update; used to safely add/remove probes concurrently |

---

*This document covers kprobes as of Linux kernel 6.x. The implementation details are derived from kernel source files: `kernel/kprobes.c`, `arch/x86/kernel/kprobes/`, `arch/arm64/kernel/probes/`, `include/linux/kprobes.h`, `kernel/trace/trace_kprobe.c`, and `rust/kernel/kprobes.rs`.*
