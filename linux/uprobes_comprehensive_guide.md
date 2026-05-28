# uprobes: A Complete & Comprehensive Guide

> **A deep technical reference for Linux user-space dynamic instrumentation**

---

## Table of Contents

1. [Introduction & Philosophy](#1-introduction--philosophy)
2. [Historical Context & Motivation](#2-historical-context--motivation)
3. [Architectural Overview](#3-architectural-overview)
4. [How uprobes Work: The Full Mechanics](#4-how-uprobes-work-the-full-mechanics)
5. [Kernel Data Structures & Internals](#5-kernel-data-structures--internals)
6. [Breakpoint Insertion & Removal](#6-breakpoint-insertion--removal)
7. [Single-Step Emulation & XOL (Execute Out-of-Line)](#7-single-step-emulation--xol-execute-out-of-line)
8. [uretprobes: Return Probes](#8-uretprobes-return-probes)
9. [uprobe vs kprobe: Detailed Comparison](#9-uprobe-vs-kprobe-detailed-comparison)
10. [Architecture-Specific Implementations](#10-architecture-specific-implementations)
11. [The uprobe Trampoline & Task Context](#11-the-uprobe-trampoline--task-context)
12. [inode-Based Addressing Model](#12-inode-based-addressing-model)
13. [Kernel Subsystem Interfaces](#13-kernel-subsystem-interfaces)
14. [perf_events Integration](#14-perf_events-integration)
15. [ftrace Integration](#15-ftrace-integration)
16. [eBPF & uprobes](#16-ebpf--uprobes)
17. [bpftrace & uprobes](#17-bpftrace--uprobes)
18. [SystemTap & uprobes](#18-systemtap--uprobes)
19. [USDT (User Statically-Defined Tracing)](#19-usdt-user-statically-defined-tracing)
20. [C Implementation Examples](#20-c-implementation-examples)
21. [Rust Implementation Examples](#21-rust-implementation-examples)
22. [Kernel Module: Custom uprobe Consumer](#22-kernel-module-custom-uprobe-consumer)
23. [Reading Registers & Memory](#23-reading-registers--memory)
24. [Signal Handling Interaction](#24-signal-handling-interaction)
25. [COW (Copy-On-Write) & Memory Mapping Considerations](#25-cow-copy-on-write--memory-mapping-considerations)
26. [Multi-threading & SMP Considerations](#26-multi-threading--smp-considerations)
27. [Performance Characteristics & Overhead](#27-performance-characteristics--overhead)
28. [Security Model & Capabilities](#28-security-model--capabilities)
29. [Limitations & Known Gotchas](#29-limitations--known-gotchas)
30. [Debugging uprobes Themselves](#30-debugging-uprobes-themselves)
31. [Real-World Use Cases](#31-real-world-use-cases)
32. [Mental Model Summary](#32-mental-model-summary)

---

## 1. Introduction & Philosophy

**uprobes** (user-space probes) are a Linux kernel feature that enables **dynamic instrumentation of user-space programs at arbitrary instruction addresses** — without recompiling, relinking, or even restarting the target process.

The fundamental idea: you can tell the kernel "when any process executing this binary hits address X, call my handler." The kernel then:

1. Patches the binary's memory-mapped page with a breakpoint instruction.
2. Catches the resulting trap in the kernel.
3. Calls registered handler(s).
4. Resumes the original instruction.
5. Continues user-space execution transparently.

The target process has **no idea** it is being probed. The instrumentation is fully transparent.

### Why This Matters

Traditional profiling and tracing approaches fall into two categories:

| Approach | Requires Restart? | Recompile? | Runtime Overhead Always On? |
|---|---|---|---|
| printf / logging | Yes (recompile) | Yes | Yes |
| gdb breakpoints | No | No | Only when running |
| perf stat/record | No | No | Yes (sampling always active) |
| Static tracepoints | No | Yes | Yes (USDT markers always compiled in) |
| **uprobes** | **No** | **No** | **No (zero cost when inactive)** |

uprobes give you **surgical, on-demand, zero-overhead-when-inactive** introspection into running processes. This is the philosophy: observability without prior planning.

---

## 2. Historical Context & Motivation

### The DTrace Influence (2004)

Sun Microsystems introduced **DTrace** in Solaris 10. It provided a unified tracing framework with both kernel and user-space probes. Linux lacked an equivalent. This created a years-long gap in Linux observability.

### kprobes First (2002–2004)

The Linux kernel gained **kprobes** — dynamic kernel instrumentation — before user-space probes. kprobes let you probe any kernel address. But user-space was a different beast: each process has its own virtual address space, and modifying memory in one process must not affect others.

### Jprobes & Early User-Space Attempts

Various hacks existed:
- `ptrace(PTRACE_POKETEXT)` to insert breakpoints manually (gdb does this)
- Preloaded shared libraries with trampolines
- SystemTap kprobes-based user-space probing (inefficient, used `access_process_vm`)

### uprobes Merged: Linux 3.5 (2012)

Srikar Dronamraju and others at IBM contributed the uprobe infrastructure merged in **Linux 3.5 (July 2012)**. The design was elegant:

- **inode-based** rather than address-based (works across exec/fork/COW)
- Reuses the hardware breakpoint / trap infrastructure
- Provides a generic consumer API

### uretprobes Added

uretprobes (return probes for user-space) were added shortly after, completing the picture.

### eBPF + uprobes (Linux 3.15, 2014)

The ability to attach eBPF programs to uprobe events was a game-changer. Now you could run kernel-verified bytecode at user-space probe points, enabling powerful tracing tools like **bpftrace**, **BCC**, and **libbpf**.

### Timeline Summary

```
2002 - kprobes merged (kernel-space dynamic probes)
2004 - DTrace released on Solaris (inspiration)
2009 - Early uprobe patches (IBM/Red Hat)
2012 - uprobes merged: Linux 3.5
2013 - uretprobes merged: Linux 3.10+
2014 - eBPF merged: Linux 3.15; uprobe+eBPF: Linux 3.15+
2015 - bcc tools matured
2018 - bpftrace released
2019 - libbpf CO-RE; uprobe support in libbpf
2020+ - uprobes become default observability primitive in cloud infra
```

---

## 3. Architectural Overview

This is the complete picture of how all the pieces fit together:

```
  USER SPACE                          KERNEL SPACE
  ══════════════════════════════════════════════════════════════════════
                                      
  ┌─────────────────────┐             ┌──────────────────────────────────┐
  │   Target Process    │             │        uprobe Subsystem          │
  │                     │             │                                  │
  │  .text segment      │             │  ┌──────────────────────────┐   │
  │  ┌───────────────┐  │    SIGTRAP  │  │    uprobe_dispatch()     │   │
  │  │  func():      │  │ ──────────► │  │  - find inode+offset     │   │
  │  │  push rbp     │  │             │  │  - walk consumer list    │   │
  │  │  mov ...      │  │             │  │  - call each handler     │   │
  │  │  [INT3/BRK] ◄─┼──┼── patched  │  └──────────┬───────────────┘   │
  │  │  add ...      │  │             │             │                    │
  │  └───────────────┘  │             │  ┌──────────▼───────────────┐   │
  │                     │             │  │   Consumer Callbacks     │   │
  │  XOL trampoline     │             │  │  ┌──────────────────────┐│   │
  │  ┌───────────────┐  │◄────────────┼──┼──┤ perf_events consumer ││   │
  │  │ saved insn    │  │  resume     │  │  ├──────────────────────┤│   │
  │  │ single-step   │  │  execution  │  │  │  eBPF consumer       ││   │
  │  └───────────────┘  │             │  │  ├──────────────────────┤│   │
  │                     │             │  │  │  ftrace consumer     ││   │
  └─────────────────────┘             │  │  ├──────────────────────┤│   │
                                      │  │  │  custom consumer     ││   │
  ┌─────────────────────┐             │  │  └──────────────────────┘│   │
  │  bpftrace / perf /  │             │  └──────────────────────────┘   │
  │  BCC / libbpf tools │             │                                  │
  │                     │             │  ┌──────────────────────────┐   │
  │  write probe spec:  │  syscalls   │  │  uprobe_register()       │   │
  │  binary + offset ───┼────────────►│  │  - find/create struct    │   │
  │                     │             │  │    uprobe for inode      │   │
  └─────────────────────┘             │  │  - add consumer          │   │
                                      │  │  - install_breakpoint()  │   │
                                      │  └──────────────────────────┘   │
                                      │                                  │
                                      │  ┌──────────────────────────┐   │
                                      │  │  inode → uprobe mapping  │   │
                                      │  │  (rb-tree per inode)     │   │
                                      │  │                          │   │
                                      │  │  inode:offset → uprobe   │   │
                                      │  │  uprobe → [consumer list]│   │
                                      │  └──────────────────────────┘   │
                                      │                                  │
                                      │  ┌──────────────────────────┐   │
                                      │  │  mm_struct integration   │   │
                                      │  │  - mmap notifier         │   │
                                      │  │  - COW handler           │   │
                                      │  │  - munmap handler        │   │
                                      │  └──────────────────────────┘   │
                                      └──────────────────────────────────┘
```

### Key Insight: inode + offset, Not Virtual Address

Unlike kprobes (which use kernel virtual addresses), uprobes identify probe locations by:

```
(inode, file_offset)
```

This is crucial because:
- Multiple processes may mmap the same file at **different** virtual addresses
- ASLR randomizes load addresses
- The inode never changes — it's the stable identity of the file

The kernel translates `(inode, offset)` → actual virtual address per-process when installing the breakpoint.

---

## 4. How uprobes Work: The Full Mechanics

Let's trace the complete lifecycle of a uprobe, step by step.

### Phase 1: Registration

```
uprobe_register(inode, offset, consumer)
         │
         ▼
  Allocate/find struct uprobe for (inode, offset)
         │
         ▼
  Add consumer to uprobe->consumers list
         │
         ▼
  Call prepare_uprobe(uprobe)
         │
         ├── read original instruction bytes at offset
         │   (using kernel_read on the file)
         │
         └── analyze instruction (how long? is it branch? is it syscall?)
```

### Phase 2: Breakpoint Installation

When a process maps the file (or already has it mapped):

```
  uprobe_mmap() called via mmap_event notifier
         │
         ▼
  For each uprobe on this inode:
    - calculate VA = vma->vm_start + (offset - vma->vm_pgoff*PAGE_SIZE)
    - install_breakpoint(uprobe, mm, vma, va)
         │
         ▼
  get_user_pages() → get writable reference to the page
         │
         ▼
  Write INT3 (0xCC on x86) / BRK on ARM64 over first byte
         │
         ▼
  flush_dcache_page() + flush instruction cache
```

### Phase 3: Trap Handling (The Hot Path)

When the target process executes the patched address:

```
  CPU executes INT3 at patched address
         │
         ▼
  CPU raises #BP (exception vector 3 on x86)
         │
         ▼
  do_int3() in kernel
         │
         ▼
  notify_die(DIE_INT3) → uprobe_int3_handler()
         │
         ▼
  uprobe_pre_sstep_notifier()
         │
         ├── find uprobe for (current->mm, instruction_pointer)
         │
         ├── save CPU registers into pt_regs
         │
         ├── call consumer->handler(consumer, regs) for each consumer
         │   (this is where eBPF programs run, perf records, etc.)
         │
         └── prepare_xol_vma(): set up single-step out-of-line
                  │
                  ▼
             copy original instruction to XOL slot
             set instruction pointer to XOL slot
             set TF (Trap Flag) for single-step
```

### Phase 4: Single-Step Out-of-Line (XOL)

```
  Return to user space — but IP points to XOL slot, not original code
         │
         ▼
  CPU executes original (saved) instruction at XOL slot
         │
         ▼
  TF causes #DB (Debug Exception) after 1 instruction
         │
         ▼
  uprobe_debug_handler() / uprobe_post_sstep_notifier()
         │
         ├── call consumer->ret_handler (post-execution handlers)
         │
         ├── fixup instruction pointer:
         │   IP = original_addr + instruction_length
         │   (adjust for any IP-relative addressing fixups)
         │
         └── clear TF, restore original execution context
```

### Phase 5: Resumption

```
  Return to user space at original_addr + instruction_length
  Process continues executing normally
  Zero indication anything happened
```

---

## 5. Kernel Data Structures & Internals

The core data structures live in `kernel/events/uprobes.c` and `include/linux/uprobes.h`.

### `struct uprobe`

```c
/* kernel/events/uprobes.c (simplified, representative of kernel ~6.x) */
struct uprobe {
    struct rb_node      rb_node;     /* node in per-inode rb-tree */
    refcount_t          ref;         /* reference count */
    struct rw_semaphore register_rwsem; /* protects consumer list */
    struct rw_semaphore consumer_rwsem;
    struct list_head    list;        /* list of all uprobes */
    struct inode        *inode;      /* target inode */
    loff_t              offset;      /* byte offset within file */
    loff_t              ref_ctr_offset; /* USDT ref counter offset */
    unsigned long       flags;       /* UPROBE_COPY_INSN etc. */
    
    struct arch_uprobe  arch;        /* arch-specific data (see below) */
};
```

### `struct arch_uprobe` (x86)

```c
/* arch/x86/include/asm/uprobes.h */
struct arch_uprobe {
    union {
        u8   insn[MAX_UINSN_BYTES];   /* saved original instruction bytes */
        u8   ixol[MAX_UINSN_BYTES];   /* instruction for XOL slot */
    };
    struct arch_uprobe_task    *autask; /* per-task state during sstep */
    uprobe_opcode_t            ainsn;   /* instruction analysis result */
    u32                        def_arg1;
    unsigned long              fixups;  /* IP fixup flags */
    unsigned long              saved_maxinsn_size;
};
```

### `struct uprobe_consumer`

This is the interface every consumer (perf, eBPF, ftrace) must implement:

```c
/* include/linux/uprobes.h */
struct uprobe_consumer {
    /* 
     * handler: called BEFORE the instruction executes
     * return 0 to single-step, UPROBE_HANDLER_REMOVE to unregister,
     * UPROBE_HANDLER_IGNORE to skip single-step
     */
    int  (*handler)(struct uprobe_consumer *self, struct pt_regs *regs);
    
    /*
     * ret_handler: called AFTER the instruction executes (post-sstep)
     * only called if handler was called and returned 0
     */
    int  (*ret_handler)(struct uprobe_consumer *self,
                        unsigned long func,
                        struct pt_regs *regs);
    
    /*
     * filter: called to decide whether to arm a uprobe for a given mm
     * return true if this consumer wants probing for this mm
     */
    bool (*filter)(struct uprobe_consumer *self,
                   enum uprobe_filter_ctx ctx,
                   struct mm_struct *mm);
    
    struct list_head cons_node; /* linked list node */
};
```

### `struct arch_uprobe_task` (per-task single-step state)

```c
/* arch/x86/include/asm/uprobes.h */
struct arch_uprobe_task {
    unsigned long           saved_trap_nr;
    unsigned long           saved_tf;       /* saved Trap Flag */
#ifdef CONFIG_X86_64
    unsigned long           saved_scratch_register;
#endif
    unsigned int            tp_len;         /* length of original insn */
    enum uprobe_task_state  state;
};
```

### `struct uprobe_task` (per-task uprobe state)

```c
/* kernel/events/uprobes.c */
struct uprobe_task {
    enum uprobe_task_state  state;
    
    union {
        struct {
            struct arch_uprobe_task autask;
            unsigned long           vaddr;   /* address being probed */
        };
        struct {
            struct callback_head    dup_xol_work;
            unsigned long           dup_xol_addr;
        };
    };
    
    struct uprobe            *active_uprobe;  /* uprobe being single-stepped */
    unsigned long             xol_vaddr;      /* XOL slot vaddr */
    
    struct return_instance   *return_instances; /* uretprobe return stack */
    unsigned int              depth;            /* recursion depth */
};
```

### Per-Inode uprobe Tree

```c
/*
 * uprobes are stored in a per-inode red-black tree
 * keyed by (offset).
 * The inode's i_uprobe field (or uprobes_tree in inode) is the root.
 */

/* Access pattern: */
struct rb_root *root = &inode->i_mapping->... /* simplified */
/* In practice: uprobes are stored in a global rb_tree 
   keyed by (inode, offset) */
```

The actual lookup in the kernel:

```c
/* kernel/events/uprobes.c */
static struct uprobe *find_uprobe(struct inode *inode, loff_t offset)
{
    struct rb_node *n = uprobes_tree.rb_node;
    
    while (n) {
        struct uprobe *u = rb_entry(n, struct uprobe, rb_node);
        int match = match_uprobe_inode_offset(u, inode, offset);
        
        if (match < 0)
            n = n->rb_left;
        else if (match > 0)
            n = n->rb_right;
        else
            return get_uprobe(u); /* found it */
    }
    return NULL;
}
```

---

## 6. Breakpoint Insertion & Removal

### The Patch Mechanics

Installing a breakpoint on x86-64:

```
Original instruction in file/memory:
  Address: 0x401234
  Bytes:   55 48 89 e5 48 83 ec 10   (push rbp; mov rbp,rsp; sub rsp,0x10)

After uprobe installation:
  Address: 0x401234
  Bytes:   CC 48 89 e5 48 83 ec 10   (INT3; mov rbp,rsp; sub rsp,0x10)
           ^^
           Only first byte changed!
```

The saved original instruction is stored in `arch_uprobe.insn[]`.

### Why Only One Byte?

INT3 is a **1-byte** instruction (0xCC). This is why x86 breakpoints only overwrite one byte. On architectures with fixed-width instructions (ARM64, RISC-V), the entire instruction word is replaced with a dedicated breakpoint instruction.

### ARM64 Specifics

```
Original:   A9BF7BFD   stp x29, x30, [sp, #-16]!
After:      D4200000   BRK #0  (or implementation-specific)
```

ARM64 uses `BRK #0` or a specific immediate value to identify uprobe traps vs. other debug traps.

### The Page Modification Problem

User-space pages are not always writable. The uprobe code must:

```
1. get_user_pages_remote(mm, addr, FOLL_WRITE|FOLL_FORCE)
   → Get a writable struct page* for the target address

2. kmap_atomic(page)  or  kmap_local_page(page)
   → Map the page into kernel address space temporarily

3. modify the byte(s)

4. kunmap_atomic / kunmap_local

5. flush_dcache_page(page)
   → Ensure d-cache coherency

6. put_page(page)
```

The full implementation:

```c
/* arch/x86/kernel/uprobes.c (representative) */
static int uprobe_write_opcode(struct arch_uprobe *auprobe,
                                struct mm_struct *mm,
                                unsigned long vaddr,
                                uprobe_opcode_t opcode)
{
    struct page *old_page, *new_page;
    void *vaddr_old, *vaddr_new;
    struct vm_area_struct *vma;
    int ret;

    /* Get the old page */
    ret = get_user_pages_remote(mm, vaddr, 1, FOLL_FORCE,
                                &old_page, &vma, NULL);
    if (ret <= 0)
        return ret ? ret : -EFAULT;

    /* Allocate a new page for COW */
    new_page = alloc_page_vma(GFP_HIGHUSER_MOVABLE, vma, vaddr);
    if (!new_page) {
        put_page(old_page);
        return -ENOMEM;
    }

    /* Copy old page → new page, then patch */
    __SetPageUptodate(new_page);
    vaddr_old = kmap_atomic(old_page);
    vaddr_new = kmap_atomic(new_page);
    memcpy(vaddr_new, vaddr_old, PAGE_SIZE);
    
    /* Write the opcode at the offset within the page */
    memcpy(vaddr_new + (vaddr & ~PAGE_MASK), &opcode, UPROBE_SWBP_INSN_SIZE);
    
    kunmap_atomic(vaddr_new);
    kunmap_atomic(vaddr_old);

    /* Replace the page in the page table */
    ret = __replace_page(vma, vaddr, old_page, new_page);
    
    put_page(old_page);
    put_page(new_page);
    return ret;
}
```

### Copy-On-Write at Installation Time

Notice `__replace_page()` — this is key. The uprobe infrastructure performs its **own COW** when patching. This ensures:

1. The file's page cache page is **never modified** (the original file stays clean)
2. The private anonymous copy in the process's page table gets the breakpoint
3. Other processes sharing the original file's pages are not affected

```
Before uprobe installation:
  Process A page table:  0x401000 → file_page (read-only, shared)
  Process B page table:  0x400000 → file_page (read-only, shared)  [different ASLR base]
  Page cache:            file_page: [55 48 89 e5 ...]  (original)

After uprobe installation for Process A:
  Process A page table:  0x401000 → patched_page_A (CC 48 89 e5 ...)  [private copy]
  Process B page table:  0x400000 → file_page (read-only, shared)     [unaffected]
  Page cache:            file_page: [55 48 89 e5 ...]  (STILL original, untouched)
```

This is why uprobes are **process-specific by default** at the MM level, even though the probe is registered at the inode level.

---

## 7. Single-Step Emulation & XOL (Execute Out-of-Line)

This is the most subtle and architecturally interesting part of uprobes.

### The Problem: You Can't Just Re-Execute In Place

After handling the breakpoint, you need to execute the **original instruction**. But you can't:
1. Just execute it where it is (the INT3 is there now)
2. Just "pretend" it ran (some instructions have side effects we must preserve)
3. Easily emulate every possible instruction in kernel code

### The XOL Solution

**XOL (Execute Out of Line)**: Copy the original instruction to a **scratch area in user space** and have the process execute it there.

```
XOL Area in Process Address Space:
  ┌────────────────────────────────────────────────────┐
  │  XOL vma (special anonymous mapping, 1 page)       │
  │  Slot 0: [original_insn bytes][NOP padding][INT3]  │
  │  Slot 1: [empty]                                   │
  │  Slot 2: [empty]                                   │
  │  ...                                               │
  │  Slot N: [empty]                                   │
  └────────────────────────────────────────────────────┘
  
  XOL slot size: typically 128 bytes (MAX_XOL_SLOT_SIZE)
  XOL vma size: typically PAGE_SIZE
  Max concurrent xol slots: PAGE_SIZE / MAX_XOL_SLOT_SIZE = 32
```

### XOL Execution Flow

```
uprobe trap at 0x401234 (INT3):
  1. Save original instruction bytes to XOL slot: 
     xol_slot[0..N] = arch_uprobe.insn[]
  
  2. Place INT3 after the instruction in XOL slot:
     xol_slot[insn_len] = INT3   (to catch when insn finishes)
  
  3. Set regs->ip = xol_vaddr    (point IP to XOL slot)
  
  4. Set TF (Trap Flag) in EFLAGS  (x86) or equivalent

  5. Return to user space

User space executes instruction at XOL slot:
  - Instruction runs normally, with full CPU hardware support
  - All side effects (flags, registers, memory writes) occur normally

After instruction:
  - TF fires DEBUG exception (#DB)
  OR
  - INT3 at end of XOL slot fires BREAKPOINT exception
  
  6. uprobe_post_sstep_notifier():
     a. Compute correct next IP:
        new_ip = original_probe_addr + original_insn_length
     b. Apply any IP-relative fixups (see below)
     c. Set regs->ip = new_ip
     d. Clear TF
     e. Call ret_handler consumers
  
  7. Return to user space at new_ip (correct next instruction)
```

### IP-Relative Instruction Fixups

On x86-64, many instructions use **RIP-relative addressing**:

```asm
; Original at 0x401234:
lea rax, [rip + 0x2000]   ; means: rax = 0x401234 + 5 + 0x2000 = 0x403239
```

If we execute this at the XOL slot (say, 0x7fff00100000):

```asm
; At XOL slot 0x7fff00100000:
lea rax, [rip + 0x2000]   ; means: rax = 0x7fff00100000 + 5 + 0x2000 = 0x7fff00102005
                           ; WRONG! Completely different address!
```

The uprobe infrastructure must **fixup** such instructions. For x86-64:

```c
/* arch/x86/kernel/uprobes.c */
static void riprel_analyze(struct arch_uprobe *auprobe,
                           struct insn *insn)
{
    /* Detect if instruction uses RIP-relative addressing */
    if (!insn_rip_relative(insn))
        return;
    
    auprobe->fixups |= UPROBE_FIX_RIP_AX; /* or RIP_CX */
    /* Will save/restore scratch register and compute correct address */
}

static void riprel_pre_xol(struct arch_uprobe *auprobe,
                           struct pt_regs *regs,
                           struct arch_uprobe_task *autask)
{
    /*
     * Replace RIP-relative encoding with AX-indirect encoding
     * so the instruction can run at any address.
     * Save the correct RIP-relative target in %rax (scratch).
     */
    unsigned long target;
    target = regs->ip + insn.length + insn.displacement.value;
    autask->saved_scratch_register = regs->ax;
    regs->ax = target;
    /* patch XOL copy of instruction to use [rax] instead of [rip+disp] */
}
```

### Instructions That Cannot Use XOL

Some instructions **cannot** be executed out-of-line and must be **emulated** in kernel:

```c
/* arch/x86/kernel/uprobes.c */
static const struct uprobe_xol_ops branch_xol_ops = {
    .emulate  = branch_emulate_op,   /* branches: jmp, call, ret */
    .post_xol = branch_post_xol_op,
};

static const struct uprobe_xol_ops push_xol_ops = {
    .emulate  = push_emulate_op,    /* PUSH with specific encodings */
};
```

Instructions requiring emulation (x86-64):
- `CALL` — modifies both IP and stack; needs careful fixup
- `RET` — pops IP from stack, complex with uretprobes
- `JMP near/far` — IP fixup required for relative jumps
- `Jcc` (conditional jumps) — IP fixup
- `LOOP/LOOPE/LOOPNE` — IP fixup
- `SYSCALL/SYSENTER` — must not be single-stepped via XOL
- `HLT`, `RDTSC` etc. — privileged/special

---

## 8. uretprobes: Return Probes

uretprobes let you fire a handler when a **function returns**, giving access to:
- Return values (in `rax`/`xmm0` etc.)
- Elapsed time (by correlating with entry probe)
- Arguments still accessible (if saved at entry)

### Implementation: Return Address Hijacking

uretprobes work by **replacing the return address on the stack** with a trampoline:

```
Normal call/return:
  caller:  CALL func     → push return_addr, jmp func
  func:    ...body...
  func:    RET           → pop return_addr, jmp return_addr

With uretprobe on func():
  caller:  CALL func     → push return_addr, jmp func
  
  At func() entry (uprobe fires):
    1. Read return_addr from stack top
    2. Save (return_addr, uprobe*) in return_instance stack
    3. Overwrite stack top with trampoline_addr
  
  func:    ...body...
  func:    RET           → pop trampoline_addr, jmp trampoline_addr
  
  Trampoline (in XOL area):
    - Contains INT3
    - Fires uprobe_handler
    - Calls ret_handler consumers (rax = return value here!)
    - Restores real return_addr to IP
    - Returns to real caller
```

### The Return Instance Stack

```c
/* kernel/events/uprobes.c */
struct return_instance {
    struct uprobe       *uprobe;
    unsigned long        func;          /* function entry vaddr */
    unsigned long        stack;         /* saved stack pointer at entry */
    unsigned long        orig_ret_vaddr; /* real return address */
    bool                 chained;       /* nested uretprobe? */
    struct return_instance *next;
};
```

The `return_instances` form a **linked list per task**, acting as a stack. This handles:
- Recursive functions (each call frame gets its own entry)
- Nested function calls with probes on multiple functions
- `longjmp` / C++ exceptions (partially — see limitations)

### uretprobe Caveats

**Stack unwinding**: Modifying the return address breaks tools that read raw stack frames (like `backtrace()`). uretprobes temporarily insert a fake address.

**Tail-call optimization**: If the compiler turns `return other_func()` into a `JMP`, there's no `RET` instruction, so uretprobe on the outer function won't fire at "return." This is a known limitation.

**setjmp/longjmp**: If `longjmp` skips over a function frame that had a uretprobe, the return address on the stack no longer gets restored → the real return address is lost → **crash or silent corruption**. The kernel tries to detect this but it's an inherent architectural limitation.

---

## 9. uprobe vs kprobe: Detailed Comparison

```
Feature                    kprobe                    uprobe
─────────────────────────────────────────────────────────────────────────
Target space               Kernel                    User space
Probe identification       Kernel vaddr / symbol     (inode, file offset)
Breakpoint instruction     INT3 / BRK                INT3 / BRK
Trap handler               do_int3 / do_brk          do_int3 / do_brk
Context of handler         Kernel context            Kernel context
                           (softirq-safe, non-sleep) (process context, can sleep)
Single-step mechanism      Kernel text XOL           User-space XOL vma
IP-relative fixup          Yes (if needed)           Yes (more complex)
Access to registers        Full pt_regs              Full pt_regs
Access to memory           kmem_cache / copy_from_user  copy_from_user
Works across processes     N/A (kernel is shared)    Yes (per-mm breakpoint install)
ASLR considerations        N/A                       Transparent (inode+offset based)
Fork behavior              N/A                       Auto-inherits (COW handles it)
Performance (inactive)     Zero                      Zero
Performance (active)       ~100ns per hit            ~1000ns per hit (user/kernel boundary crossing)
Can probe JIT'd code?      N/A                       Yes (if file-backed) / No (anonymous)
Blacklist protection       Yes (kprobe_blacklist)    N/A
Works on modules           Yes                       Yes (shared libraries)
```

### Why uprobe Is Slower Than kprobe

When a uprobe fires, the cost is:
1. User→Kernel ring switch (expensive: TLB flush, cr3 switch on KPTI systems)
2. Exception dispatch
3. uprobe lookup in rb-tree
4. Consumer handler(s) execution
5. XOL setup and another user→kernel→user round trip for single-step

On a KPTI-enabled system (Meltdown mitigation), each ring switch involves a full page-table switch. This is why uprobe overhead is **~1–10 µs per hit** vs. kprobe's **~100 ns**.

---

## 10. Architecture-Specific Implementations

### x86 / x86-64

```
Breakpoint instruction:   INT3  (opcode 0xCC, 1 byte)
Exception vector:         #BP (vector 3)
Handler:                  do_int3() → uprobe_int3_handler()
Single-step mechanism:    TF (Trap Flag) in EFLAGS, #DB exception
XOL area:                 Special anonymous vma in user space
Max instruction length:   15 bytes (x86 variable-length)

Special handling:
- REX prefixes
- VEX/EVEX (AVX) instructions
- LOCK prefix
- BOUND instruction
- CALL/RET/Jcc emulation
- RIP-relative addressing fixup
```

### ARM64 (AArch64)

```
Breakpoint instruction:   BRK #0  (4 bytes, fixed-width ISA)
Exception type:           Synchronous exception, ESR_ELx_EC_BREAKPT_CUR
Handler:                  arm64_break_handler() → uprobe_breakpoint_handler()
Single-step mechanism:    SS (Software Step) exception, MDSCR_EL1.SS
XOL area:                 Special anonymous vma in user space
Instruction length:       Always 4 bytes

Special handling:
- PC-relative instructions: ADR, ADRP, B, BL, B.cond, CBZ, CBNZ, TBZ, TBNZ
- LDR/STR with PC-relative encoding
- All 4 bytes replaced (no partial-byte issue unlike x86)
```

ARM64 uprobe handler:

```c
/* arch/arm64/kernel/probes/uprobes.c */
int arch_uprobe_exception_notify(struct notifier_block *self,
                                 unsigned long val, void *data)
{
    struct die_args *args = data;
    struct pt_regs *regs = args->regs;
    
    if (user_mode(regs) && is_uprobe_xol_address(regs->pc))
        return uprobe_post_sstep_notifier(regs) ? NOTIFY_STOP : NOTIFY_DONE;
    
    return NOTIFY_DONE;
}

bool arch_uprobe_skip_sstep(struct arch_uprobe *auprobe, struct pt_regs *regs)
{
    /* 
     * Try to emulate the instruction instead of XOL
     * for PC-relative instructions, or instructions that
     * must not execute at a different address.
     */
    u32 insn = le32_to_cpu(*(u32 *)auprobe->insn);
    
    if (aarch64_insn_is_branch_imm(insn)) {
        /* emulate directly */
        simulate_branch(auprobe, regs);
        return true;
    }
    return false; /* fall through to XOL */
}
```

### RISC-V

```
Breakpoint instruction:   EBREAK  (4 bytes, or 2 bytes C.EBREAK for compressed ISA)
Support status:           Added in Linux 5.15
Constraints:              Compressed instruction extension complicates things
```

### PowerPC / s390

Both have uprobe support. s390 (IBM mainframe) was actually one of the earliest platforms, given IBM's involvement in the original uprobe development.

---

## 11. The uprobe Trampoline & Task Context

### The XOL VMA

Each process that has active uretprobes gets a special VMA:

```c
/* kernel/events/uprobes.c */
static struct vm_area_struct *get_xol_area(struct mm_struct *mm)
{
    struct vm_area_struct *area;
    
    if (mm->uprobes_state.xol_area)
        return mm->uprobes_state.xol_area;
    
    /* Allocate special XOL VMA */
    area = __install_special_mapping(mm,
                                     TASK_SIZE - PAGE_SIZE,  /* near top of user AS */
                                     PAGE_SIZE,
                                     VM_EXEC | VM_MAYEXEC | VM_DONTCOPY |
                                     VM_IO | VM_DONTEXPAND,
                                     &xol_mapping);
    return area;
}
```

The XOL vma has `VM_DONTCOPY` set, meaning it is **not inherited** on `fork()`. The child process will get a fresh XOL area.

### XOL Slot Allocation

```c
/* Each concurrent single-step needs its own slot */
/* Slots are allocated as a bitmap within the XOL page */

struct xol_area {
    wait_queue_head_t  wq;       /* waitqueue for slot availability */
    atomic_t           slot_count; /* current in-use slots */
    unsigned long     *bitmap;    /* one bit per slot */
    struct vm_special_mapping xol_mapping;
    struct page        *pages[2]; /* the actual pages */
    unsigned long      vaddr;     /* base vaddr of XOL area */
};
```

On a heavily threaded process with many concurrent uprobe hits, slots can be exhausted. The kernel will make threads wait until a slot is free.

---

## 12. inode-Based Addressing Model

### Why Inodes, Not Addresses?

Consider a shared library (`libc.so.6`) loaded by 1000 processes:

```
Process 1: libc mapped at 0x7f8800000000
Process 2: libc mapped at 0x7f9900000000   (ASLR)
Process 3: libc mapped at 0x7faa00000000   (ASLR)
...
Process N: libc mapped at 0x?????????????
```

All 1000 processes share the same inode for `libc.so.6`. A uprobe registered as:
```
(inode_of_libc, offset_0x80000)   ← "probe malloc at offset 0x80000"
```

... automatically applies to **all** 1000 processes. The kernel installs the breakpoint at each process's own virtual address (derived from its VMA layout).

### The VMA Notification System

```c
/* kernel/events/uprobes.c */

/* Called when a new VMA is mmap'd */
void uprobe_mmap(struct vm_area_struct *vma)
{
    struct inode *inode;
    struct uprobe *uprobe, *u;
    loff_t min_offset, max_offset;
    
    if (!valid_vma(vma, false))
        return;
    
    inode = file_inode(vma->vm_file);
    if (!inode)
        return;
    
    /* Check if any uprobes exist for this inode */
    /* Walk all uprobes in range and install breakpoints */
    spin_lock(&uprobes_treelock);
    u = find_node_in_range(inode, min_offset, max_offset);
    list_for_each_entry(uprobe, &u->list, ...) {
        install_breakpoint(uprobe, current->mm, vma, ...);
    }
    spin_unlock(&uprobes_treelock);
}

/* Called when a VMA is munmap'd */
void uprobe_munmap(struct vm_area_struct *vma, unsigned long start,
                   unsigned long end)
{
    /* Remove breakpoints for this range */
    remove_breakpoints_in_range(vma, start, end);
}
```

### Fork Behavior

```c
/* When a process forks, do_fork() → copy_mm() → dup_mm() is called.
 * The new mm has a copy of all VMAs (private COW).
 * The uprobe pages are still patched in the parent's mm.
 * 
 * For the child, uprobe_dup_mmap() is called:
 */
void uprobe_dup_mmap(struct mm_struct *oldmm, struct mm_struct *newmm)
{
    /* 
     * The child inherits the parent's page tables (COW).
     * The breakpoint pages are already in the child's page tables.
     * We just need to register the child's mm with all uprobes
     * that apply to it.
     */
    dup_uprobes(oldmm, newmm);
}
```

After fork, **both parent and child** are probed. This is usually desirable (trace all processes of a multi-process application).

---

## 13. Kernel Subsystem Interfaces

### The Public uprobe API (for kernel consumers)

```c
/* include/linux/uprobes.h */

/* Register a uprobe at (inode, offset) with given consumer */
int uprobe_register(struct inode *inode, loff_t offset,
                    struct uprobe_consumer *uc);

/* Register with a USDT reference counter at ref_ctr_offset */
int uprobe_register_refctr(struct inode *inode, loff_t offset,
                            loff_t ref_ctr_offset,
                            struct uprobe_consumer *uc);

/* Unregister a previously registered consumer */
void uprobe_unregister(struct inode *inode, loff_t offset,
                       struct uprobe_consumer *uc);

/* Apply/remove uprobe for a specific mm (used by perf for filtering) */
int uprobe_apply(struct inode *inode, loff_t offset,
                 struct uprobe_consumer *uc, bool add);

/* Check if current task is executing in XOL area */
bool uprobe_deny_signal(void);

/* Called from exception handlers */
int uprobe_pre_sstep_notifier(struct pt_regs *regs);
int uprobe_post_sstep_notifier(struct pt_regs *regs);
```

### Sysfs/procfs Interface for Userland

uprobes don't have a direct sysfs interface. Instead, they are accessed through:

1. **perf_event_open(2)** — via `perf_event_attr.type = PERF_TYPE_TRACEPOINT`
2. **/sys/kernel/debug/tracing/uprobe_events** — the ftrace interface
3. **bpf(2)** with `BPF_PROG_TYPE_KPROBE` (used for uprobes too) and `perf_event_open`

---

## 14. perf_events Integration

### Setting Up a uprobe via perf_event_open

The kernel path from `perf_event_open` to a uprobe:

```
perf_event_open(attr, pid, cpu, group_fd, flags)
  attr.type = PERF_TYPE_TRACEPOINT
  attr.config = <tracepoint_id>
  │
  ▼
perf_event_alloc()
  │
  ▼
perf_uprobe_init()  (kernel/trace/trace_uprobe.c)
  │
  ├── parse probe specification
  ├── resolve binary path → inode
  ├── resolve symbol/offset → file offset
  ├── create trace_uprobe struct
  │
  ▼
uprobe_register(inode, offset, &trace_uprobe->consumer)
```

### trace_uprobe Consumer

```c
/* kernel/trace/trace_uprobe.c */
struct trace_uprobe {
    struct list_head         list;
    struct trace_uprobe_filter filter;
    
    struct uprobe_consumer   consumer;  /* embedded consumer */
    
    struct path              path;      /* file path */
    struct inode            *inode;     /* target inode */
    char                    *filename;  /* printable name */
    
    unsigned long            offset;    /* probe offset */
    unsigned long            ref_ctr_offset; /* USDT ref ctr */
    
    /* Fetch arguments specification */
    unsigned int             nr_args;
    struct probe_arg         args[];
};

/* The consumer handler */
static int uprobe_dispatcher(struct uprobe_consumer *con,
                              struct pt_regs *regs)
{
    struct trace_uprobe *tu;
    tu = container_of(con, struct trace_uprobe, consumer);
    
    /* Check filter (pid, comm, etc.) */
    if (!filter_match(tu, current))
        return 0;
    
    /* Record into trace ring buffer */
    uprobe_trace_func(tu, regs, &tu->tp);
    
    return 0;
}
```

### Using perf CLI

```bash
# List uprobe events
perf probe -l

# Add a uprobe on function "malloc" in libc
perf probe -x /lib/x86_64-linux-gnu/libc.so.6 malloc

# Add a uprobe at a specific offset
perf probe -x /usr/bin/myapp 0x1234

# Add with argument capture
perf probe -x /lib/x86_64-linux-gnu/libc.so.6 'malloc size=%di'

# Add return probe
perf probe -x /lib/x86_64-linux-gnu/libc.so.6 'malloc%return retval=$retval'

# Record events
perf record -e probe_libc:malloc -ag -- sleep 10

# View results
perf script
```

---

## 15. ftrace Integration

The ftrace uprobe interface lives in `/sys/kernel/debug/tracing/`:

### uprobe_events File

```bash
# Syntax:
# p[:<event_name>] <binary_path>:<offset_or_symbol>[+offset] [arg=fetchspec ...]
# r[:<event_name>] <binary_path>:<offset_or_symbol>[+offset] [arg=fetchspec ...]

# Add entry probe on malloc
echo 'p:my_malloc /lib/x86_64-linux-gnu/libc.so.6:malloc size=%di' \
    > /sys/kernel/debug/tracing/uprobe_events

# Add return probe on malloc (capture return value)
echo 'r:my_malloc_ret /lib/x86_64-linux-gnu/libc.so.6:malloc retval=$retval' \
    >> /sys/kernel/debug/tracing/uprobe_events

# Enable the events
echo 1 > /sys/kernel/debug/tracing/events/uprobes/my_malloc/enable
echo 1 > /sys/kernel/debug/tracing/events/uprobes/my_malloc_ret/enable

# Enable tracing
echo 1 > /sys/kernel/debug/tracing/tracing_on

# Read trace
cat /sys/kernel/debug/tracing/trace

# Disable
echo 0 > /sys/kernel/debug/tracing/events/uprobes/my_malloc/enable
echo > /sys/kernel/debug/tracing/uprobe_events   # clear all
```

### Fetch Arguments Specification

```
%register   - CPU register (e.g., %di, %si, %dx, %cx, %r8, %r9 for x86-64 args)
$retval     - Return value register (%ax on x86-64)
$stack      - Stack pointer
$stackN     - Nth argument on stack (stack0 = stack pointer itself)
@addr       - Memory at absolute address 'addr'
@symbol     - Memory at symbol address
+offset(reg)- Memory at register + offset (dereference pointer)
+offset(@sym)- Memory at symbol+offset
s8/s16/s32/s64 - signed cast
u8/u16/u32/u64 - unsigned cast
b<width>@<offset>/<container> - bitfield extraction
string      - NUL-terminated string (for char* args)
x8/x16/x32/x64 - hexadecimal output
```

Example: capturing a `struct` argument field:

```bash
# Assume: void process_req(struct request *req)
# struct request { int id; long size; char name[64]; }
# id is at offset 0, size at offset 4, name at offset 8

echo 'p:probe_req /usr/bin/server:process_req \
    req_id=+0(%di):s32 \
    req_size=+4(%di):s64 \
    req_name=+8(%di):string' \
    > /sys/kernel/debug/tracing/uprobe_events
```

---

## 16. eBPF & uprobes

eBPF programs attached to uprobes are the most powerful and flexible way to use them. The BPF program runs in the kernel as a verified, safe bytecode program every time the probe fires.

### BPF Program Type: BPF_PROG_TYPE_KPROBE

Despite the name, `BPF_PROG_TYPE_KPROBE` handles both kprobes **and** uprobes. The context struct is `struct pt_regs`.

### Complete libbpf Example: Tracing malloc

#### Kernel-Side BPF Program (malloc_trace.bpf.c)

```c
// SPDX-License-Identifier: GPL-2.0
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

/* Map to store per-PID allocation sizes */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, u32);
    __type(value, u64);
} alloc_count SEC(".maps");

/* Map for output ring buffer */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

/* Event structure sent to user space */
struct alloc_event {
    u32  pid;
    u32  tid;
    u64  size;
    u64  addr;        /* filled by return probe */
    char comm[16];
};

/*
 * Uprobe on malloc entry.
 * On x86-64, first argument (size) is in %rdi.
 * PT_REGS_PARM1(ctx) extracts %rdi portably.
 */
SEC("uprobe//lib/x86_64-linux-gnu/libc.so.6:malloc")
int BPF_UPROBE(trace_malloc_entry, size_t size)
{
    u64 id = bpf_get_current_pid_tgid();
    u32 pid = id >> 32;
    u32 tid = (u32)id;
    
    /* Store size for correlation with return probe */
    bpf_map_update_elem(&alloc_count, &tid, &size, BPF_ANY);
    
    return 0;
}

/*
 * Uretprobe on malloc return.
 * PT_REGS_RC(ctx) extracts the return value (%rax on x86-64).
 */
SEC("uretprobe//lib/x86_64-linux-gnu/libc.so.6:malloc")
int BPF_URETPROBE(trace_malloc_return, void *ret)
{
    u64 id = bpf_get_current_pid_tgid();
    u32 pid = id >> 32;
    u32 tid = (u32)id;
    u64 *size_ptr;
    struct alloc_event *e;
    
    size_ptr = bpf_map_lookup_elem(&alloc_count, &tid);
    if (!size_ptr)
        return 0;
    
    /* Reserve space in ring buffer */
    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) {
        bpf_map_delete_elem(&alloc_count, &tid);
        return 0;
    }
    
    e->pid  = pid;
    e->tid  = tid;
    e->size = *size_ptr;
    e->addr = (u64)(unsigned long)ret;
    bpf_get_current_comm(e->comm, sizeof(e->comm));
    
    bpf_ringbuf_submit(e, 0);
    bpf_map_delete_elem(&alloc_count, &tid);
    
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
```

#### User-Space Loader (malloc_trace.c)

```c
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <unistd.h>
#include <bpf/libbpf.h>
#include "malloc_trace.skel.h"  /* generated by bpftool gen skeleton */

struct alloc_event {
    __u32 pid;
    __u32 tid;
    __u64 size;
    __u64 addr;
    char  comm[16];
};

static volatile bool running = true;

static void sig_handler(int sig)
{
    running = false;
}

static int handle_event(void *ctx, void *data, size_t data_sz)
{
    struct alloc_event *e = data;
    printf("PID: %-6d COMM: %-16s SIZE: %-8llu ADDR: 0x%016llx\n",
           e->pid, e->comm, (unsigned long long)e->size,
           (unsigned long long)e->addr);
    return 0;
}

int main(int argc, char **argv)
{
    struct malloc_trace_bpf *skel;
    struct ring_buffer *rb = NULL;
    int err;

    /* Set up signal handler for clean exit */
    signal(SIGINT, sig_handler);
    signal(SIGTERM, sig_handler);

    /* Load and verify BPF application */
    skel = malloc_trace_bpf__open();
    if (!skel) {
        fprintf(stderr, "Failed to open BPF skeleton\n");
        return 1;
    }

    /* Load BPF programs and maps */
    err = malloc_trace_bpf__load(skel);
    if (err) {
        fprintf(stderr, "Failed to load BPF skeleton: %d\n", err);
        goto cleanup;
    }

    /* Attach BPF programs to uprobes */
    err = malloc_trace_bpf__attach(skel);
    if (err) {
        fprintf(stderr, "Failed to attach BPF skeleton: %d\n", err);
        goto cleanup;
    }

    /* Set up ring buffer polling */
    rb = ring_buffer__new(bpf_map__fd(skel->maps.events),
                          handle_event, NULL, NULL);
    if (!rb) {
        fprintf(stderr, "Failed to create ring buffer\n");
        goto cleanup;
    }

    printf("Tracing malloc... Ctrl-C to exit.\n");
    printf("%-6s %-16s %-8s %s\n", "PID", "COMM", "SIZE", "ADDR");

    while (running) {
        err = ring_buffer__poll(rb, 100 /* ms */);
        if (err < 0 && err != -EINTR) {
            fprintf(stderr, "Error polling ring buffer: %d\n", err);
            break;
        }
    }

cleanup:
    ring_buffer__free(rb);
    malloc_trace_bpf__destroy(skel);
    return err < 0 ? -err : 0;
}
```

#### Build System (Makefile)

```makefile
CC      := clang
BPFTOOL := bpftool
ARCH    := $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')

# Generate vmlinux.h (BTF type information)
vmlinux.h:
	$(BPFTOOL) btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h

# Compile BPF C code to BPF ELF
malloc_trace.bpf.o: malloc_trace.bpf.c vmlinux.h
	$(CC) -g -O2 -target bpf -D__TARGET_ARCH_$(ARCH) \
	    -I/usr/include/$(shell uname -m)-linux-gnu \
	    -c malloc_trace.bpf.c -o malloc_trace.bpf.o

# Generate skeleton header from BPF ELF
malloc_trace.skel.h: malloc_trace.bpf.o
	$(BPFTOOL) gen skeleton malloc_trace.bpf.o > malloc_trace.skel.h

# Compile user-space loader
malloc_trace: malloc_trace.c malloc_trace.skel.h
	$(CC) -g -O2 -o malloc_trace malloc_trace.c -lbpf -lelf -lz

all: malloc_trace

clean:
	rm -f *.o *.skel.h vmlinux.h malloc_trace
```

### BPF Helper Functions Available in uprobe Context

```c
/* Available in BPF_PROG_TYPE_KPROBE (uprobe) programs: */

bpf_get_current_pid_tgid()      /* pid<<32 | tid */
bpf_get_current_uid_gid()       /* uid<<32 | gid */
bpf_get_current_comm()          /* process name */
bpf_get_current_task()          /* struct task_struct* (for CO-RE) */
bpf_get_current_cgroup_id()     /* cgroup id */

bpf_probe_read_user()           /* safe read from user-space address */
bpf_probe_read_user_str()       /* safe read string from user space */
bpf_probe_read_kernel()         /* safe read from kernel address */

bpf_map_lookup_elem()           /* hash/array map lookup */
bpf_map_update_elem()           /* hash/array map update */
bpf_map_delete_elem()           /* hash map delete */

bpf_perf_event_output()         /* write to perf ring buffer */
bpf_ringbuf_reserve()           /* reserve slot in ring buffer */
bpf_ringbuf_submit()            /* submit reserved slot */

bpf_ktime_get_ns()              /* monotonic clock in nanoseconds */
bpf_get_stackid()               /* capture stack trace */

bpf_send_signal()               /* send signal to current process */
bpf_override_return()           /* change return value (only with CONFIG_BPF_KPROBE_OVERRIDE) */
```

### BPF CO-RE with uprobes

CO-RE (Compile Once – Run Everywhere) enables reading kernel structures portably across kernel versions:

```c
#include "vmlinux.h"
#include <bpf/bpf_core_read.h>

SEC("uprobe//usr/bin/nginx:ngx_http_process_request")
int trace_nginx_request(struct pt_regs *ctx)
{
    /* Read from user-space struct via BPF CO-RE helpers */
    /* Get first argument: ngx_http_request_t *r */
    void *r = (void *)PT_REGS_PARM1(ctx);
    
    /* Read r->uri.data (char*) and r->uri.len (size_t) */
    /* These are at known offsets in nginx's struct */
    char uri[256] = {};
    __u64 uri_len = 0;
    void *uri_data = NULL;
    
    /* Read the len field (at known offset in ngx_str_t) */
    bpf_probe_read_user(&uri_len, sizeof(uri_len), r + /* offset of uri.len */);
    bpf_probe_read_user(&uri_data, sizeof(uri_data), r + /* offset of uri.data */);
    
    if (uri_data && uri_len > 0) {
        __u32 len = uri_len < sizeof(uri) - 1 ? uri_len : sizeof(uri) - 1;
        bpf_probe_read_user_str(uri, len + 1, uri_data);
        bpf_printk("nginx request: %s\n", uri);
    }
    
    return 0;
}
```

---

## 17. bpftrace & uprobes

bpftrace is a high-level tracing language built on eBPF that makes uprobes very ergonomic.

### bpftrace uprobe Syntax

```
uprobe:binary_path:function_name { action }
uprobe:binary_path:offset { action }
uretprobe:binary_path:function_name { action }
```

### bpftrace Built-ins Available in uprobe Context

```
pid, tid, uid, gid    - process/thread/user/group IDs
comm                  - process name (comm)
cpu                   - CPU number
nsecs                 - current timestamp in nanoseconds
elapsed               - time since bpftrace started
curthread             - current thread's task_struct
arg0..argN            - function arguments (in registers per ABI)
retval                - return value (uretprobe only)
stack, ustack         - kernel / user stack traces
kstack, kustack       - formatted kernel / user stack traces
```

### bpftrace Examples

```bash
# Trace all malloc calls with size > 1MB
bpftrace -e '
uprobe:/lib/x86_64-linux-gnu/libc.so.6:malloc
/arg0 > 1048576/
{
    printf("PID %d (%s) malloc(%d)\n", pid, comm, arg0);
    print(ustack);
}'

# Measure malloc latency
bpftrace -e '
uprobe:/lib/x86_64-linux-gnu/libc.so.6:malloc { @start[tid] = nsecs; }

uretprobe:/lib/x86_64-linux-gnu/libc.so.6:malloc
/@start[tid]/
{
    @latency = hist(nsecs - @start[tid]);
    delete(@start[tid]);
}'

# Trace file opens in any process
bpftrace -e '
uprobe:/lib/x86_64-linux-gnu/libc.so.6:open,
uprobe:/lib/x86_64-linux-gnu/libc.so.6:openat
{
    printf("%s opened: %s\n", comm, str(arg0));
}'

# Trace a specific PID
bpftrace -e '
uprobe:/usr/bin/python3:PyEval_EvalFrameEx
/pid == $1/
{
    printf("Python frame executed, tid=%d\n", tid);
}' $TARGET_PID

# Count function calls per process
bpftrace -e '
uprobe:/lib/x86_64-linux-gnu/libssl.so:SSL_read
{
    @calls[comm, pid] = count();
}'

# Profile function latency as histogram
bpftrace -e '
uprobe:/usr/bin/myserver:handle_request { @ts[tid] = nsecs; }
uretprobe:/usr/bin/myserver:handle_request /@ts[tid]/ {
    @latency_us = hist((nsecs - @ts[tid]) / 1000);
    delete(@ts[tid]);
}'

# Trace by USDT probe (application-defined markers)
bpftrace -e '
usdt:/usr/bin/node:node:http__server__request {
    printf("HTTP request: method=%s url=%s\n",
           str(arg0), str(arg1));
}'
```

### Listing Available uprobes

```bash
# List all probes in a binary
bpftrace -l 'uprobe:/usr/bin/bash:*'

# List with regex
bpftrace -l 'uprobe:/lib/x86_64-linux-gnu/libc.so.6:malloc*'

# List USDT probes
bpftrace -l 'usdt:/usr/bin/python3:*'
```

---

## 18. SystemTap & uprobes

SystemTap is an older but still-used tracing framework. It can use uprobes under the hood.

```stap
# SystemTap script: trace malloc
probe process("/lib/x86_64-linux-gnu/libc.so.6").function("malloc")
{
    printf("malloc(%d) called by PID %d (%s)\n",
           $size, pid(), execname())
}

probe process("/lib/x86_64-linux-gnu/libc.so.6").function("malloc").return
{
    printf("malloc returned %p\n", $return)
}

# Filter to a specific process
probe process("/usr/bin/httpd").statement("*@/src/http.c:234")
{
    printf("Hit line 234, var=%d\n", $local_var)
}
```

---

## 19. USDT (User Statically-Defined Tracing)

USDT probes are **statically defined** in source code but **dynamically activated**. They are a collaboration between the application developer and the tracer.

### What USDT Provides Over Plain uprobes

- **Stable, named probe points** (not dependent on function boundaries or addresses)
- **Typed arguments** (documented in the probe definition)
- **Reference counting** (probe code becomes NOP when no consumer)
- **Semantic stability** across application versions

### How USDT Works: The NOP Trick

```c
/* Application source code with USDT probe */
#include <sys/sdt.h>  /* dtrace-compatible probes */

void process_request(struct request *req)
{
    /* This expands to a NOP instruction (or multi-byte NOP) */
    /* Plus an ELF note section entry describing the probe */
    DTRACE_PROBE2(myapp, request__start,
                  req->id, req->size);
    
    /* ... actual processing ... */
    
    DTRACE_PROBE1(myapp, request__end, req->id);
}
```

The `DTRACE_PROBE2` macro expands to something like:

```asm
; x86-64 assembly generated by DTRACE_PROBE2
990:    nop                  ; Placeholder (can be patched to INT3)
        .pushsection .note.stapsdt,"?","note"
        .4byte  ....         ; Note header (namesz, descsz, type)
        .asciz  "stapsdt"    ; Provider/name
        .8byte  990b         ; Address of the NOP (probe location)
        .8byte  semaphore    ; Address of reference counter (optional)
        .asciz  "myapp"      ; Provider name
        .asciz  "request__start" ; Probe name
        .asciz  "%8@%rdi %4@%rsi" ; Argument specification
        .popsection
```

### The ELF `.note.stapsdt` Section

```bash
# Inspect USDT probes in a binary
readelf -n /usr/bin/python3 | grep -A 5 "stapsdt"

# Or use bpftrace to list them
bpftrace -l 'usdt:/usr/bin/python3:*'

# Output example:
# usdt:/usr/bin/python3:python:function__entry
# usdt:/usr/bin/python3:python:function__return
# usdt:/usr/bin/python3:python:line
# usdt:/usr/bin/python3:python:import__find__load__start
```

### USDT Reference Counter (Semaphore)

```c
/* Optional: Only execute probe code if someone is listening */

/* In header: */
extern unsigned short myapp_request_start_semaphore
    __attribute__((weak)) __asm__("myapp_request__start_semaphore");

/* In code: */
if (STAP_PROBABLE(myapp_request__start_semaphore)) {
    /* Expensive argument preparation only when probed */
    DTRACE_PROBE2(myapp, request__start, req->id, get_size(req));
}
```

The tracer increments/decrements this semaphore via uprobe on the semaphore address. The `ref_ctr_offset` field in `struct uprobe` handles this.

### Adding USDT Probes in C

```c
/* Compile with: gcc -I/usr/include/sys myapp.c -o myapp */
#include <sys/sdt.h>

/* Provider: myapp, Probe: request_start, Args: id (int), size (long) */
DTRACE_PROBE2(myapp, request_start, req->id, req->size);

/* Provider: myapp, Probe: db_query, Args: query string, duration ns */
DTRACE_PROBE2(myapp, db_query, query_str, duration_ns);
```

### Adding USDT Probes in Rust

```rust
// Cargo.toml
// [dependencies]
// probe = "0.5"
// OR
// tracing = "0.1"

use probe::probe;

fn process_request(req: &Request) {
    // USDT probe: provider=myapp, probe=request_start
    // Arguments: req.id (i32), req.size (usize)
    probe!(myapp, request_start, req.id, req.size);
    
    // ... processing ...
    
    probe!(myapp, request_end, req.id);
}
```

### Enabling USDT via bpftrace

```bash
# Trace Python function calls
bpftrace -e '
usdt:/usr/bin/python3:python:function__entry {
    printf("%s:%s:%d\n",
           str(arg0),   /* filename */
           str(arg1),   /* function name */
           arg2);       /* line number */
}'

# Trace with latency measurement
bpftrace -e '
usdt:/usr/bin/python3:python:function__entry  { @start[arg1, tid] = nsecs; }
usdt:/usr/bin/python3:python:function__return
/@start[arg1, tid]/
{
    @us[str(arg1)] = hist((nsecs - @start[arg1, tid]) / 1000);
    delete(@start[arg1, tid]);
}'
```

---

## 20. C Implementation Examples

### Example 1: Low-Level uprobe via perf_event_open

```c
/*
 * uprobe_perf.c
 * Demonstrates setting up a uprobe via perf_event_open directly
 * without using BPF or higher-level libraries.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/ioctl.h>
#include <sys/mman.h>
#include <sys/syscall.h>
#include <linux/perf_event.h>
#include <linux/hw_breakpoint.h>
#include <elf.h>
#include <errno.h>
#include <stdint.h>

/* 
 * Find the file offset of a symbol in an ELF binary.
 * This is what we need for uprobe registration.
 */
static long find_symbol_offset(const char *binary, const char *symbol)
{
    FILE *f;
    Elf64_Ehdr ehdr;
    Elf64_Shdr *shdrs;
    char *shstrtab;
    int i, j;
    long offset = -1;
    
    f = fopen(binary, "rb");
    if (!f) {
        perror("fopen");
        return -1;
    }
    
    /* Read ELF header */
    if (fread(&ehdr, sizeof(ehdr), 1, f) != 1) goto out;
    if (memcmp(ehdr.e_ident, ELFMAG, SELFMAG) != 0) goto out;
    
    /* Read section headers */
    shdrs = malloc(ehdr.e_shnum * sizeof(Elf64_Shdr));
    fseek(f, ehdr.e_shoff, SEEK_SET);
    fread(shdrs, sizeof(Elf64_Shdr), ehdr.e_shnum, f);
    
    /* Read section name string table */
    shstrtab = malloc(shdrs[ehdr.e_shstrndx].sh_size);
    fseek(f, shdrs[ehdr.e_shstrndx].sh_offset, SEEK_SET);
    fread(shstrtab, shdrs[ehdr.e_shstrndx].sh_size, 1, f);
    
    /* Walk sections looking for symbol tables */
    for (i = 0; i < ehdr.e_shnum; i++) {
        if (shdrs[i].sh_type != SHT_SYMTAB &&
            shdrs[i].sh_type != SHT_DYNSYM)
            continue;
        
        /* Read symbol table */
        int nsyms = shdrs[i].sh_size / sizeof(Elf64_Sym);
        Elf64_Sym *syms = malloc(shdrs[i].sh_size);
        fseek(f, shdrs[i].sh_offset, SEEK_SET);
        fread(syms, sizeof(Elf64_Sym), nsyms, f);
        
        /* Read associated string table */
        Elf64_Shdr *strtab_shdr = &shdrs[shdrs[i].sh_link];
        char *strtab = malloc(strtab_shdr->sh_size);
        fseek(f, strtab_shdr->sh_offset, SEEK_SET);
        fread(strtab, strtab_shdr->sh_size, 1, f);
        
        for (j = 0; j < nsyms; j++) {
            if (strcmp(strtab + syms[j].st_name, symbol) == 0) {
                /* 
                 * For shared libraries: st_value is the offset from
                 * the load base (= file offset for LOAD segment).
                 * For position-dependent executables: it's VA.
                 */
                offset = (long)syms[j].st_value;
                
                /* 
                 * Adjust for PIE/shared-lib: find the LOAD segment
                 * that contains this symbol and compute file offset.
                 */
                Elf64_Phdr *phdrs = malloc(ehdr.e_phnum * sizeof(Elf64_Phdr));
                fseek(f, ehdr.e_phoff, SEEK_SET);
                fread(phdrs, sizeof(Elf64_Phdr), ehdr.e_phnum, f);
                
                for (int k = 0; k < ehdr.e_phnum; k++) {
                    if (phdrs[k].p_type != PT_LOAD) continue;
                    if (offset >= (long)phdrs[k].p_vaddr &&
                        offset < (long)(phdrs[k].p_vaddr + phdrs[k].p_memsz)) {
                        /* file_offset = va - p_vaddr + p_offset */
                        offset = offset - phdrs[k].p_vaddr + phdrs[k].p_offset;
                        break;
                    }
                }
                free(phdrs);
                free(strtab);
                free(syms);
                goto out_free;
            }
        }
        free(strtab);
        free(syms);
    }

out_free:
    free(shdrs);
    free(shstrtab);
out:
    fclose(f);
    return offset;
}

/*
 * Set up a uprobe via the tracefs uprobe_events interface.
 * Returns the tracepoint ID to use with perf_event_open.
 */
static int setup_uprobe_tracefs(const char *binary, unsigned long offset,
                                 const char *event_name, int is_return)
{
    char path[512];
    char cmd[1024];
    FILE *f;
    int tracepoint_id = -1;
    
    /* Write to uprobe_events */
    f = fopen("/sys/kernel/debug/tracing/uprobe_events", "a");
    if (!f) {
        perror("fopen uprobe_events");
        return -1;
    }
    
    /* Format: p:event_name binary:offset */
    snprintf(cmd, sizeof(cmd), "%s:%s %s:0x%lx",
             is_return ? "r" : "p",
             event_name, binary, offset);
    
    fprintf(f, "%s\n", cmd);
    fclose(f);
    
    /* Read back the tracepoint ID */
    snprintf(path, sizeof(path),
             "/sys/kernel/debug/tracing/events/uprobes/%s/id",
             event_name);
    
    f = fopen(path, "r");
    if (!f) {
        perror("fopen tracepoint id");
        return -1;
    }
    fscanf(f, "%d", &tracepoint_id);
    fclose(f);
    
    return tracepoint_id;
}

/*
 * Read perf events from a mmap'd ring buffer.
 */
static void read_perf_events(int fd, void *mmap_base, size_t mmap_size)
{
    struct perf_event_mmap_page *header = mmap_base;
    uint64_t data_head, data_tail;
    char *data_base = (char *)mmap_base + getpagesize();
    size_t data_size = mmap_size - getpagesize();
    
    data_head = __atomic_load_n(&header->data_head, __ATOMIC_ACQUIRE);
    data_tail = header->data_tail;
    
    while (data_tail < data_head) {
        struct perf_event_header *evt;
        uint64_t offset = data_tail % data_size;
        
        evt = (struct perf_event_header *)(data_base + offset);
        
        if (evt->type == PERF_RECORD_SAMPLE) {
            printf("uprobe fired! type=%d size=%d\n",
                   evt->type, evt->size);
        }
        
        data_tail += evt->size;
    }
    
    header->data_tail = data_tail;
}

int main(int argc, char **argv)
{
    const char *binary  = "/lib/x86_64-linux-gnu/libc.so.6";
    const char *symbol  = "malloc";
    const char *evname  = "my_malloc_probe";
    
    /* Step 1: Find symbol offset in ELF */
    long offset = find_symbol_offset(binary, symbol);
    if (offset < 0) {
        fprintf(stderr, "Symbol '%s' not found in %s\n", symbol, binary);
        return 1;
    }
    printf("Found %s at file offset: 0x%lx\n", symbol, offset);
    
    /* Step 2: Register via tracefs */
    int tp_id = setup_uprobe_tracefs(binary, (unsigned long)offset, evname, 0);
    if (tp_id < 0) {
        fprintf(stderr, "Failed to set up uprobe (need root?)\n");
        return 1;
    }
    printf("Tracepoint ID: %d\n", tp_id);
    
    /* Step 3: Open perf event */
    struct perf_event_attr attr = {
        .type           = PERF_TYPE_TRACEPOINT,
        .size           = sizeof(attr),
        .config         = tp_id,      /* tracepoint ID */
        .sample_type    = PERF_SAMPLE_IP | PERF_SAMPLE_TID |
                          PERF_SAMPLE_TIME | PERF_SAMPLE_CPU,
        .sample_period  = 1,          /* trigger on every event */
        .wakeup_events  = 1,
        .disabled       = 1,
    };
    
    int perf_fd = syscall(SYS_perf_event_open, &attr,
                          -1,   /* all PIDs */
                           0,   /* all CPUs */
                          -1, 0);
    if (perf_fd < 0) {
        perror("perf_event_open");
        goto cleanup;
    }
    
    /* Step 4: mmap ring buffer */
    int page_size = getpagesize();
    int mmap_pages = 8;   /* must be power of 2 */
    size_t mmap_size = (1 + mmap_pages) * page_size;
    
    void *mmap_base = mmap(NULL, mmap_size,
                           PROT_READ | PROT_WRITE,
                           MAP_SHARED, perf_fd, 0);
    if (mmap_base == MAP_FAILED) {
        perror("mmap");
        goto cleanup;
    }
    
    /* Step 5: Enable and read */
    ioctl(perf_fd, PERF_EVENT_IOC_RESET, 0);
    ioctl(perf_fd, PERF_EVENT_IOC_ENABLE, 0);
    
    printf("Tracing %s:%s ... (Ctrl-C to stop)\n", binary, symbol);
    
    for (int i = 0; i < 5; i++) {
        sleep(1);
        read_perf_events(perf_fd, mmap_base, mmap_size);
    }
    
    ioctl(perf_fd, PERF_EVENT_IOC_DISABLE, 0);
    munmap(mmap_base, mmap_size);

cleanup:
    if (perf_fd >= 0) close(perf_fd);
    
    /* Clean up uprobe */
    FILE *f = fopen("/sys/kernel/debug/tracing/uprobe_events", "w");
    if (f) { fprintf(f, "\n"); fclose(f); }
    
    return 0;
}
```

### Example 2: Measuring Function Latency in C via ftrace

```c
/*
 * uprobe_latency.c
 * Uses the tracefs interface to measure function latency
 * by correlating entry and return probes.
 * Compile: gcc -O2 -o uprobe_latency uprobe_latency.c
 * Run: sudo ./uprobe_latency /lib/x86_64-linux-gnu/libc.so.6 malloc
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/stat.h>
#include <errno.h>
#include <time.h>

#define TRACEFS "/sys/kernel/debug/tracing"

static int write_file(const char *path, const char *data)
{
    int fd = open(path, O_WRONLY | O_TRUNC);
    if (fd < 0) { perror(path); return -1; }
    ssize_t n = write(fd, data, strlen(data));
    close(fd);
    return n < 0 ? -1 : 0;
}

static int append_file(const char *path, const char *data)
{
    int fd = open(path, O_WRONLY | O_APPEND);
    if (fd < 0) { perror(path); return -1; }
    ssize_t n = write(fd, data, strlen(data));
    close(fd);
    return n < 0 ? -1 : 0;
}

static long find_elf_symbol_offset(const char *binary, const char *sym);
/* ... (same as above, omitted for brevity) ... */

int main(int argc, char **argv)
{
    if (argc < 3) {
        fprintf(stderr, "Usage: %s <binary> <symbol>\n", argv[0]);
        return 1;
    }
    
    const char *binary = argv[1];
    const char *symbol = argv[2];
    char cmd[1024];
    
    /* Find offset */
    long offset = find_elf_symbol_offset(binary, symbol);
    if (offset < 0) {
        fprintf(stderr, "Symbol not found\n");
        return 1;
    }
    
    /* Clear existing uprobe events */
    write_file(TRACEFS "/uprobe_events", "");
    
    /* Add entry probe */
    snprintf(cmd, sizeof(cmd), "p:entry_%s %s:0x%lx",
             symbol, binary, offset);
    if (append_file(TRACEFS "/uprobe_events", cmd) < 0) return 1;
    
    /* Add return probe */
    snprintf(cmd, sizeof(cmd), "r:exit_%s %s:0x%lx $retval",
             symbol, binary, offset);
    if (append_file(TRACEFS "/uprobe_events", cmd) < 0) return 1;
    
    /* Enable events */
    snprintf(cmd, sizeof(cmd),
             TRACEFS "/events/uprobes/entry_%s/enable", symbol);
    write_file(cmd, "1");
    
    snprintf(cmd, sizeof(cmd),
             TRACEFS "/events/uprobes/exit_%s/enable", symbol);
    write_file(cmd, "1");
    
    /* Enable tracing */
    write_file(TRACEFS "/tracing_on", "1");
    
    printf("Tracing %s:%s for 5 seconds...\n", binary, symbol);
    sleep(5);
    
    /* Disable */
    write_file(TRACEFS "/tracing_on", "0");
    
    /* Read trace output */
    int trace_fd = open(TRACEFS "/trace", O_RDONLY);
    if (trace_fd >= 0) {
        char buf[4096];
        ssize_t n;
        while ((n = read(trace_fd, buf, sizeof(buf)-1)) > 0) {
            buf[n] = '\0';
            fputs(buf, stdout);
        }
        close(trace_fd);
    }
    
    /* Cleanup */
    write_file(TRACEFS "/uprobe_events", "");
    
    return 0;
}
```

### Example 3: Target Application with USDT Probes

```c
/*
 * server_with_usdt.c
 * Application instrumented with USDT probes.
 * Compile: gcc -O2 -o server_with_usdt server_with_usdt.c
 * (requires libstapsdt or sys/sdt.h from systemtap-sdt-dev)
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/sdt.h>    /* DTRACE_PROBE* macros */
#include <time.h>

/* 
 * USDT probes in this application:
 *
 * Provider: myserver
 * ├── request_start(int id, const char *method, const char *path)
 * ├── request_end(int id, int status, long duration_us)
 * ├── db_query_start(int query_id, const char *sql)
 * ├── db_query_end(int query_id, int rows, long duration_us)
 * └── cache_hit(const char *key)
 */

struct request {
    int         id;
    char        method[8];
    char        path[256];
    struct timespec start_time;
};

static void handle_request(struct request *req)
{
    struct timespec end;
    long duration_us;
    int status;
    
    /* USDT probe: request_start */
    DTRACE_PROBE3(myserver, request__start,
                  req->id, req->method, req->path);
    
    /* Simulate some work */
    usleep(rand() % 1000 + 100);
    
    /* Simulate a DB query */
    char sql[128];
    snprintf(sql, sizeof(sql), "SELECT * FROM users WHERE path='%s'", req->path);
    
    DTRACE_PROBE2(myserver, db__query__start, req->id, sql);
    usleep(rand() % 500);
    int rows = rand() % 10;
    DTRACE_PROBE3(myserver, db__query__end, req->id, rows, 400L);
    
    /* Simulate cache */
    if (rand() % 2)
        DTRACE_PROBE1(myserver, cache__hit, req->path);
    
    status = 200;
    clock_gettime(CLOCK_MONOTONIC, &end);
    duration_us = (end.tv_sec  - req->start_time.tv_sec)  * 1000000 +
                  (end.tv_nsec - req->start_time.tv_nsec) / 1000;
    
    /* USDT probe: request_end */
    DTRACE_PROBE3(myserver, request__end,
                  req->id, status, duration_us);
}

int main(void)
{
    printf("Server starting (PID %d)\n", getpid());
    printf("USDT probes: myserver:request__start, request__end,\n"
           "             db__query__start, db__query__end, cache__hit\n");
    printf("Trace with: bpftrace -e '\n"
           "  usdt:%s:myserver:request__start { "
           "printf(\"req %%d: %%s %%s\\n\", arg0, str(arg1), str(arg2)); }'\n",
           "/proc/self/exe");
    
    const char *methods[] = {"GET", "POST", "PUT", "DELETE"};
    const char *paths[]   = {"/api/users", "/api/orders", "/health", "/metrics"};
    
    for (int i = 1; ; i++) {
        struct request req = {
            .id = i,
        };
        strncpy(req.method, methods[rand() % 4], sizeof(req.method)-1);
        strncpy(req.path,   paths[rand() % 4],   sizeof(req.path)-1);
        clock_gettime(CLOCK_MONOTONIC, &req.start_time);
        
        handle_request(&req);
        
        usleep(100000);  /* 100ms between requests */
    }
    
    return 0;
}
```

---

## 21. Rust Implementation Examples

### Example 1: uprobe Monitoring Tool in Rust using libbpf-rs

```toml
# Cargo.toml
[package]
name = "uprobe-tracer"
version = "0.1.0"
edition = "2021"

[dependencies]
libbpf-rs = "0.21"
libbpf-sys = "1.3"
plain = "0.2"
anyhow = "1.0"
clap = { version = "4", features = ["derive"] }

[build-dependencies]
libbpf-cargo = "0.21"
```

```rust
// src/main.rs
use anyhow::{bail, Result};
use clap::Parser;
use libbpf_rs::{skel::SkelBuilder, OpenSkel, Object, ObjectBuilder, MapFlags};
use plain::Plain;
use std::time::Duration;

/// Arguments for the uprobe tracer
#[derive(Parser, Debug)]
#[command(about = "uprobe function latency tracer")]
struct Args {
    /// Path to binary to trace
    #[arg(short, long)]
    binary: String,
    
    /// Symbol name to trace
    #[arg(short, long)]
    symbol: String,
    
    /// Duration in seconds
    #[arg(short, long, default_value = "10")]
    duration: u64,
}

/// Event structure matching the BPF-side definition
#[repr(C)]
#[derive(Default, Copy, Clone)]
struct Event {
    pid:      u32,
    tid:      u32,
    duration: u64,
    comm:     [u8; 16],
}

unsafe impl Plain for Event {}

fn main() -> Result<()> {
    let args = Args::parse();
    
    // Bump rlimit for BPF
    let rlim = libc::rlimit {
        rlim_cur: libc::RLIM_INFINITY,
        rlim_max: libc::RLIM_INFINITY,
    };
    unsafe { libc::setrlimit(libc::RLIMIT_MEMLOCK, &rlim) };
    
    // Find symbol offset in ELF
    let offset = find_elf_symbol_offset(&args.binary, &args.symbol)?;
    println!("Found {} at offset 0x{:x}", args.symbol, offset);
    
    // Build and load BPF object
    let mut obj_builder = ObjectBuilder::default();
    obj_builder.debug(false);
    
    // Load pre-compiled BPF object (built at compile time)
    let open_obj = obj_builder.open_memory(
        include_bytes!(concat!(env!("OUT_DIR"), "/uprobe_tracer.bpf.o"))
    )?;
    
    let mut obj = open_obj.load()?;
    
    // Attach uprobe to entry
    let entry_prog = obj.prog_mut("trace_entry")
        .ok_or_else(|| anyhow::anyhow!("prog not found"))?;
    
    let _entry_link = entry_prog.attach_uprobe(
        false,        /* is_return = false (entry probe) */
        -1,           /* all PIDs */
        &args.binary,
        offset,
    )?;
    
    // Attach uretprobe (return probe)
    let exit_prog = obj.prog_mut("trace_exit")
        .ok_or_else(|| anyhow::anyhow!("prog not found"))?;
    
    let _exit_link = exit_prog.attach_uprobe(
        true,         /* is_return = true */
        -1,
        &args.binary,
        offset,
    )?;
    
    println!("Tracing {}:{} for {} seconds...",
             args.binary, args.symbol, args.duration);
    println!("{:<6} {:<16} {:>10}", "PID", "COMM", "LATENCY(µs)");
    
    // Set up ring buffer for events
    let map = obj.map("events")
        .ok_or_else(|| anyhow::anyhow!("map not found"))?;
    
    let mut builder = libbpf_rs::RingBufferBuilder::new();
    builder.add(map, |data: &[u8]| {
        let mut event = Event::default();
        plain::copy_from_bytes(&mut event, data).expect("data buffer too short");
        
        let comm = std::str::from_utf8(&event.comm)
            .unwrap_or("?")
            .trim_end_matches('\0');
        
        println!("{:<6} {:<16} {:>10}",
                 event.pid, comm, event.duration / 1000);
        0
    })?;
    
    let mgr = builder.build()?;
    
    let start = std::time::Instant::now();
    while start.elapsed() < Duration::from_secs(args.duration) {
        mgr.poll(Duration::from_millis(100))?;
    }
    
    Ok(())
}

/// Find ELF symbol offset (file offset, not virtual address)
fn find_elf_symbol_offset(binary: &str, symbol: &str) -> Result<usize> {
    use std::fs;
    
    let data = fs::read(binary)?;
    
    // Parse ELF header
    if &data[0..4] != b"\x7fELF" {
        bail!("Not an ELF file");
    }
    
    // Determine 32-bit vs 64-bit
    let is_64bit = data[4] == 2;
    if !is_64bit {
        bail!("Only 64-bit ELF supported in this example");
    }
    
    // Parse ELF64 header
    let e_shoff   = u64::from_le_bytes(data[40..48].try_into()?);
    let e_shentsize = u16::from_le_bytes(data[58..60].try_into()?) as usize;
    let e_shnum   = u16::from_le_bytes(data[60..62].try_into()?) as usize;
    let e_shstrndx = u16::from_le_bytes(data[62..64].try_into()?) as usize;
    
    let shoff = e_shoff as usize;
    
    // Get section name string table
    let shstrndx_off = shoff + e_shstrndx * e_shentsize;
    let shstr_offset = u64::from_le_bytes(
        data[shstrndx_off+24..shstrndx_off+32].try_into()?
    ) as usize;
    let shstr_size = u64::from_le_bytes(
        data[shstrndx_off+32..shstrndx_off+40].try_into()?
    ) as usize;
    
    // Walk sections
    for i in 0..e_shnum {
        let sh_off = shoff + i * e_shentsize;
        let sh_type = u32::from_le_bytes(data[sh_off+4..sh_off+8].try_into()?);
        
        // SHT_SYMTAB = 2, SHT_DYNSYM = 11
        if sh_type != 2 && sh_type != 11 { continue; }
        
        let sym_offset = u64::from_le_bytes(
            data[sh_off+24..sh_off+32].try_into()?
        ) as usize;
        let sym_size = u64::from_le_bytes(
            data[sh_off+32..sh_off+40].try_into()?
        ) as usize;
        let sym_link = u32::from_le_bytes(
            data[sh_off+40..sh_off+44].try_into()?
        ) as usize;
        
        // Get associated string table
        let str_sh_off = shoff + sym_link * e_shentsize;
        let str_offset = u64::from_le_bytes(
            data[str_sh_off+24..str_sh_off+32].try_into()?
        ) as usize;
        
        // Walk symbols (Elf64_Sym is 24 bytes)
        let nsyms = sym_size / 24;
        for j in 0..nsyms {
            let sym = sym_offset + j * 24;
            let st_name  = u32::from_le_bytes(data[sym..sym+4].try_into()?) as usize;
            let st_value = u64::from_le_bytes(data[sym+8..sym+16].try_into()?) as usize;
            
            // Read symbol name
            let name_start = str_offset + st_name;
            let name_end = data[name_start..].iter()
                .position(|&b| b == 0)
                .map(|p| name_start + p)
                .unwrap_or(name_start);
            
            let name = std::str::from_utf8(&data[name_start..name_end])?;
            if name != symbol { continue; }
            
            // Found! Convert virtual address to file offset
            // Parse program headers to find LOAD segment
            let e_phoff = u64::from_le_bytes(data[32..40].try_into()?) as usize;
            let e_phentsize = u16::from_le_bytes(data[54..56].try_into()?) as usize;
            let e_phnum = u16::from_le_bytes(data[56..58].try_into()?) as usize;
            
            for k in 0..e_phnum {
                let ph_off = e_phoff + k * e_phentsize;
                let p_type   = u32::from_le_bytes(data[ph_off..ph_off+4].try_into()?);
                if p_type != 1 { continue; } // PT_LOAD
                
                let p_offset = u64::from_le_bytes(data[ph_off+8..ph_off+16].try_into()?) as usize;
                let p_vaddr  = u64::from_le_bytes(data[ph_off+16..ph_off+24].try_into()?) as usize;
                let p_memsz  = u64::from_le_bytes(data[ph_off+40..ph_off+48].try_into()?) as usize;
                
                if st_value >= p_vaddr && st_value < p_vaddr + p_memsz {
                    return Ok(st_value - p_vaddr + p_offset);
                }
            }
            
            // If not found in LOAD segment, return as-is
            return Ok(st_value);
        }
    }
    
    bail!("Symbol '{}' not found in '{}'", symbol, binary)
}
```

### Example 2: USDT Probes in Rust using the `probe` crate

```rust
// src/lib.rs or src/main.rs
// Cargo.toml: probe = "0.5"

use probe::probe;
use std::time::{Instant, Duration};

/// Database query executor with USDT instrumentation
pub struct Database {
    name: String,
    query_count: u64,
}

impl Database {
    pub fn new(name: &str) -> Self {
        Database {
            name: name.to_string(),
            query_count: 0,
        }
    }
    
    pub fn execute(&mut self, sql: &str) -> Result<Vec<Vec<String>>, String> {
        self.query_count += 1;
        let query_id = self.query_count;
        
        // USDT probe: provider=mydb, probe=query_start
        // args: query_id (u64), sql (&str as *const u8 + len)
        probe!(mydb, query_start, query_id, sql.as_ptr(), sql.len());
        
        let start = Instant::now();
        
        // Simulate query execution
        let result = self.do_execute(sql);
        
        let duration_ns = start.elapsed().as_nanos() as u64;
        let row_count = result.as_ref().map(|r| r.len()).unwrap_or(0);
        
        // USDT probe: provider=mydb, probe=query_end
        probe!(mydb, query_end, query_id, row_count, duration_ns);
        
        result
    }
    
    fn do_execute(&self, sql: &str) -> Result<Vec<Vec<String>>, String> {
        // Simulated execution
        std::thread::sleep(Duration::from_micros(100));
        Ok(vec![
            vec!["col1".to_string(), "col2".to_string()],
        ])
    }
}

/// HTTP request handler with USDT probes
pub struct HttpHandler {
    request_count: u64,
}

impl HttpHandler {
    pub fn new() -> Self {
        HttpHandler { request_count: 0 }
    }
    
    pub fn handle(&mut self, method: &str, path: &str) -> u16 {
        self.request_count += 1;
        let req_id = self.request_count;
        
        // Fire USDT probe at request start
        probe!(http, request_start,
               req_id,
               method.as_ptr(), method.len(),
               path.as_ptr(), path.len());
        
        let start = Instant::now();
        
        // Simulate handling
        std::thread::sleep(Duration::from_micros(500));
        let status: u16 = 200;
        
        let duration_us = start.elapsed().as_micros() as u64;
        
        // Fire USDT probe at request end
        probe!(http, request_end, req_id, status as u64, duration_us);
        
        status
    }
}

// Trace with bpftrace:
// bpftrace -e '
//   usdt:/path/to/binary:mydb:query_start {
//     printf("query %lld: %s\n", arg0, str(arg1, arg2));
//   }
//   usdt:/path/to/binary:mydb:query_end {
//     printf("query %lld done: %d rows in %lld ns\n", arg0, arg1, arg2);
//   }
// '
```

### Example 3: Rust uprobe Inspector (reading /proc and tracefs)

```rust
// uprobe_inspector.rs
// A tool to register and monitor uprobes via the tracefs interface
// Run with: sudo cargo run --bin uprobe_inspector -- --binary /usr/bin/myapp --symbol myfunc

use std::fs::{self, OpenOptions};
use std::io::{self, Write, BufRead, BufReader};
use std::path::Path;
use std::thread;
use std::time::Duration;
use anyhow::{Context, Result};

const TRACEFS: &str = "/sys/kernel/debug/tracing";

pub struct UprobeManager {
    events: Vec<String>,
}

impl UprobeManager {
    pub fn new() -> Self {
        UprobeManager { events: Vec::new() }
    }
    
    /// Write to a tracefs file
    fn write_tracefs(&self, subpath: &str, content: &str) -> Result<()> {
        let path = format!("{}/{}", TRACEFS, subpath);
        let mut f = OpenOptions::new()
            .write(true)
            .append(subpath == "uprobe_events")
            .truncate(subpath != "uprobe_events")
            .open(&path)
            .with_context(|| format!("Failed to open {}", path))?;
        
        f.write_all(content.as_bytes())
            .with_context(|| format!("Failed to write to {}", path))?;
        Ok(())
    }
    
    /// Register an entry uprobe
    pub fn add_entry_probe(&mut self,
                            binary: &str,
                            offset: u64,
                            name: &str,
                            args: &[(&str, &str)]) -> Result<()>
    {
        let mut cmd = format!("p:{} {}:0x{:x}", name, binary, offset);
        for (arg_name, fetch_spec) in args {
            cmd.push_str(&format!(" {}={}", arg_name, fetch_spec));
        }
        cmd.push('\n');
        
        self.write_tracefs("uprobe_events", &cmd)
            .context("Failed to add uprobe")?;
        
        // Enable the event
        let enable_path = format!("events/uprobes/{}/enable", name);
        self.write_tracefs(&enable_path, "1")?;
        
        self.events.push(name.to_string());
        println!("Registered entry probe '{}': {}", name, cmd.trim());
        Ok(())
    }
    
    /// Register a return (ret) probe
    pub fn add_return_probe(&mut self,
                             binary: &str,
                             offset: u64,
                             name: &str) -> Result<()>
    {
        let cmd = format!("r:{} {}:0x{:x} retval=$retval\n", name, binary, offset);
        self.write_tracefs("uprobe_events", &cmd)?;
        
        let enable_path = format!("events/uprobes/{}/enable", name);
        self.write_tracefs(&enable_path, "1")?;
        
        self.events.push(name.to_string());
        println!("Registered return probe '{}': {}", name, cmd.trim());
        Ok(())
    }
    
    /// Start tracing
    pub fn start(&self) -> Result<()> {
        self.write_tracefs("tracing_on", "1")?;
        Ok(())
    }
    
    /// Stop tracing
    pub fn stop(&self) -> Result<()> {
        self.write_tracefs("tracing_on", "0")?;
        Ok(())
    }
    
    /// Stream trace output
    pub fn stream_trace<F>(&self, duration_secs: u64, handler: F) -> Result<()>
    where F: Fn(&str)
    {
        let path = format!("{}/trace_pipe", TRACEFS);
        let f = std::fs::File::open(&path)?;
        let reader = BufReader::new(f);
        
        let deadline = std::time::Instant::now()
            + Duration::from_secs(duration_secs);
        
        // trace_pipe is a blocking read; use a timeout thread
        // In production, use select()/poll() or a separate thread
        
        for line in reader.lines() {
            if std::time::Instant::now() >= deadline {
                break;
            }
            match line {
                Ok(l) if !l.starts_with('#') => handler(&l),
                _ => {}
            }
        }
        Ok(())
    }
    
    /// Get snapshot of current trace buffer
    pub fn read_trace(&self) -> Result<String> {
        let path = format!("{}/trace", TRACEFS);
        fs::read_to_string(path).context("Failed to read trace")
    }
}

impl Drop for UprobeManager {
    fn drop(&mut self) {
        // Clean up: disable events and clear uprobe_events
        for event in &self.events {
            let path = format!("events/uprobes/{}/enable", event);
            let _ = self.write_tracefs(&path, "0");
        }
        let _ = self.write_tracefs("uprobe_events", "");
        let _ = self.write_tracefs("tracing_on", "0");
        println!("UprobeManager: cleaned up {} probes", self.events.len());
    }
}

fn main() -> Result<()> {
    let binary = "/lib/x86_64-linux-gnu/libc.so.6";
    let offset = 0x9d360u64;  /* malloc offset — find with: readelf -sW libc.so.6 | grep malloc */
    
    let mut mgr = UprobeManager::new();
    
    // Add entry probe capturing size argument
    // On x86-64: arg0 = %di (first integer argument)
    mgr.add_entry_probe(binary, offset, "malloc_entry",
                        &[("size", "%di")])?;
    
    // Add return probe
    mgr.add_return_probe(binary, offset, "malloc_ret")?;
    
    mgr.start()?;
    
    println!("Tracing malloc for 5 seconds...");
    thread::sleep(Duration::from_secs(5));
    
    mgr.stop()?;
    
    // Read and parse trace output
    let trace = mgr.read_trace()?;
    let mut call_count = 0u64;
    let mut large_allocs = Vec::new();
    
    for line in trace.lines() {
        if line.contains("malloc_entry") {
            call_count += 1;
            // Parse size from: "... malloc_entry: (0x...) size=12345"
            if let Some(size_pos) = line.find("size=") {
                let size_str = &line[size_pos+5..];
                if let Ok(size) = size_str.split_whitespace().next()
                    .unwrap_or("0").parse::<u64>()
                {
                    if size > 1_000_000 {
                        large_allocs.push(size);
                    }
                }
            }
        }
    }
    
    println!("\n=== Summary ===");
    println!("Total malloc calls: {}", call_count);
    println!("Large allocations (>1MB): {}", large_allocs.len());
    for size in &large_allocs {
        println!("  {} bytes ({:.1} MB)", size, *size as f64 / 1_048_576.0);
    }
    
    Ok(())
}
```

---

## 22. Kernel Module: Custom uprobe Consumer

This shows how to write a kernel module that registers its own uprobe consumer, bypassing all userland tools.

```c
/*
 * my_uprobe_module.c
 *
 * Kernel module that installs a uprobe on a user binary.
 * Compile with your kernel's build system.
 *
 * Makefile:
 *   obj-m := my_uprobe_module.o
 *   all:
 *     make -C /lib/modules/$(shell uname -r)/build M=$(PWD) modules
 */

#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>
#include <linux/uprobes.h>
#include <linux/path.h>
#include <linux/namei.h>
#include <linux/fs.h>
#include <linux/ptrace.h>
#include <linux/sched.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Example Author");
MODULE_DESCRIPTION("Custom uprobe consumer example");

/* Configuration: binary path and offset to probe */
static char *binary = "/lib/x86_64-linux-gnu/libc.so.6";
module_param(binary, charp, 0);
MODULE_PARM_DESC(binary, "Path to binary to probe");

static unsigned long probe_offset = 0x9d360;  /* malloc offset */
module_param(probe_offset, ulong, 0);
MODULE_PARM_DESC(probe_offset, "File offset of probe point");

/* Statistics */
static atomic64_t call_count = ATOMIC64_INIT(0);
static atomic64_t total_size = ATOMIC64_INIT(0);

/* Our uprobe consumer */
static int my_handler(struct uprobe_consumer *uc, struct pt_regs *regs)
{
    unsigned long size;
    
    /* On x86-64, first argument is in %rdi */
    size = regs->di;
    
    atomic64_inc(&call_count);
    atomic64_add(size, &total_size);
    
    /* Only log large allocations to avoid log spam */
    if (size > 1024 * 1024) {
        pr_info("my_uprobe: PID %d (%s) malloc(%lu)\n",
                current->pid, current->comm, size);
    }
    
    return 0;  /* 0 = proceed with single-step; UPROBE_HANDLER_REMOVE = remove self */
}

static int my_ret_handler(struct uprobe_consumer *uc, unsigned long func,
                           struct pt_regs *regs)
{
    /* On x86-64, return value is in %rax */
    unsigned long ret = regs->ax;
    
    if (ret == 0) {
        pr_warn("my_uprobe: PID %d malloc returned NULL!\n", current->pid);
    }
    
    return 0;
}

static bool my_filter(struct uprobe_consumer *uc, enum uprobe_filter_ctx ctx,
                       struct mm_struct *mm)
{
    /* Only probe processes named "targetapp" */
    /* 
     * Note: current task might not have comm set yet when filter is called
     * during mmap. This is a simplified example.
     */
    return true; /* probe all processes */
}

static struct uprobe_consumer my_consumer = {
    .handler     = my_handler,
    .ret_handler = my_ret_handler,
    .filter      = my_filter,
};

/* The inode being probed */
static struct inode *probed_inode;

static int __init my_uprobe_init(void)
{
    struct path path;
    int ret;
    
    pr_info("my_uprobe: loading, binary=%s offset=0x%lx\n",
            binary, probe_offset);
    
    /* Look up the inode */
    ret = kern_path(binary, LOOKUP_FOLLOW, &path);
    if (ret) {
        pr_err("my_uprobe: failed to resolve path '%s': %d\n", binary, ret);
        return ret;
    }
    
    probed_inode = igrab(d_inode(path.dentry));
    path_put(&path);
    
    if (!probed_inode) {
        pr_err("my_uprobe: failed to get inode\n");
        return -EINVAL;
    }
    
    /* Register the uprobe */
    ret = uprobe_register(probed_inode, probe_offset, &my_consumer);
    if (ret) {
        pr_err("my_uprobe: uprobe_register failed: %d\n", ret);
        iput(probed_inode);
        probed_inode = NULL;
        return ret;
    }
    
    pr_info("my_uprobe: registered successfully\n");
    return 0;
}

static void __exit my_uprobe_exit(void)
{
    if (probed_inode) {
        uprobe_unregister(probed_inode, probe_offset, &my_consumer);
        iput(probed_inode);
    }
    
    pr_info("my_uprobe: total malloc calls: %lld, total bytes: %lld\n",
            atomic64_read(&call_count),
            atomic64_read(&total_size));
    pr_info("my_uprobe: unloaded\n");
}

module_init(my_uprobe_init);
module_exit(my_uprobe_exit);
```

---

## 23. Reading Registers & Memory

### Register Access in uprobe Handlers

On x86-64 System V ABI:
```
Function arguments:     %rdi, %rsi, %rdx, %rcx, %r8, %r9
                        (arg1, arg2, arg3, arg4, arg5, arg6)
Remaining args:         on stack (rsp+8, rsp+16, ...)
Return value:           %rax (or %rax:%rdx for 128-bit)
Floating point args:    %xmm0..%xmm7
Stack pointer:          %rsp
Frame pointer (opt):    %rbp
```

In BPF programs:
```c
/* Portable register access macros */
PT_REGS_PARM1(ctx)   /* arg1: %rdi on x86-64, %r0 on ARM64 */
PT_REGS_PARM2(ctx)   /* arg2: %rsi on x86-64, %r1 on ARM64 */
PT_REGS_PARM3(ctx)   /* arg3: %rdx on x86-64, %r2 on ARM64 */
PT_REGS_PARM4(ctx)   /* arg4: %rcx on x86-64, %r3 on ARM64 */
PT_REGS_PARM5(ctx)   /* arg5: %r8  on x86-64, %r4 on ARM64 */
PT_REGS_RC(ctx)      /* return value: %rax on x86-64, %r0 on ARM64 */
PT_REGS_SP(ctx)      /* stack pointer */
PT_REGS_IP(ctx)      /* instruction pointer */
PT_REGS_FP(ctx)      /* frame pointer */
```

### Reading User-Space Memory Safely

In BPF:
```c
/* Read a single value */
int value;
bpf_probe_read_user(&value, sizeof(value), (void *)user_ptr);

/* Read a string */
char buf[256];
bpf_probe_read_user_str(buf, sizeof(buf), (void *)user_str_ptr);

/* Read through a pointer chain (dereference) */
/* First read the pointer */
void *inner_ptr;
bpf_probe_read_user(&inner_ptr, sizeof(inner_ptr), outer_ptr);
/* Then read the data */
bpf_probe_read_user(&value, sizeof(value), inner_ptr + offset);
```

In kernel module uprobe handler:
```c
static int my_handler(struct uprobe_consumer *uc, struct pt_regs *regs)
{
    /* regs->di = first argument (pointer to struct) */
    void __user *user_ptr = (void __user *)regs->di;
    
    /* Safe read from user space */
    struct my_struct s;
    if (copy_from_user(&s, user_ptr, sizeof(s))) {
        pr_warn("Failed to read user struct\n");
        return 0;
    }
    
    /* Read a string */
    char buf[256];
    if (strncpy_from_user(buf, (char __user *)regs->si, sizeof(buf)) < 0) {
        pr_warn("Failed to read user string\n");
        return 0;
    }
    
    pr_info("Called with id=%d name=%s\n", s.id, buf);
    return 0;
}
```

### Stack Argument Access

For functions with >6 arguments (x86-64), args 7+ are on the stack:
```c
/* In BPF: read stack arguments */
SEC("uprobe//usr/bin/app:func_many_args")
int trace_many_args(struct pt_regs *ctx)
{
    /* First 6 args via PT_REGS_PARM* */
    long arg1 = PT_REGS_PARM1(ctx);
    /* arg7 is at rsp+8 (skip the return address) */
    long arg7;
    void *sp = (void *)PT_REGS_SP(ctx);
    bpf_probe_read_user(&arg7, sizeof(arg7), sp + 8);
    return 0;
}
```

---

## 24. Signal Handling Interaction

### Signals During uprobe Single-Step

If a signal arrives **while the process is single-stepping in the XOL slot**, the kernel must handle it carefully:

```
Normal flow:
  [uprobe fires] → [XOL setup] → [user executes at XOL] → [#DB] → [resume]

Signal arrives during XOL execution:
  [uprobe fires] → [XOL setup] → [user executes at XOL] → [SIGNAL!]
                                                              │
                                                              ▼
  kernel delivers signal, signal handler runs
                │
  signal handler returns → sigreturn
                │
  kernel detects task was in XOL (via uprobe_task->state)
                │
  re-run the uprobe handler? No — single-step bookkeeping preserved
  resume at XOL slot continuation or abort
```

The `struct uprobe_task` tracks the state machine:

```c
enum uprobe_task_state {
    UTASK_RUNNING,          /* normal execution */
    UTASK_SSTEP_ACQ,        /* acquiring XOL slot */
    UTASK_SSTEP_TRAPPED,    /* in XOL, waiting for #DB */
    UTASK_SSTEP,            /* single-stepping in XOL */
    UTASK_SSTEP_NONE,       /* XOL done */
};
```

### `uprobe_deny_signal()`

The kernel function `uprobe_deny_signal()` is called from the signal delivery path:

```c
/* kernel/signal.c */
static int do_signal(struct pt_regs *regs)
{
    /* If we're in the middle of a uprobe single-step, don't deliver signal yet */
    if (uprobe_deny_signal())
        return 0;
    /* ... normal signal delivery ... */
}
```

This prevents race conditions between signal delivery and XOL execution.

---

## 25. COW (Copy-On-Write) & Memory Mapping Considerations

### The Full COW Picture

```
Scenario: 10 processes sharing the same /usr/lib/libc.so.6
          A uprobe is registered on malloc

BEFORE uprobe registration:
  All 10 processes → same physical pages (read-only, shared)
  Page cache: original bytes [55 48 89 e5 ...]

AFTER uprobe registration:
  Process 1 VMA → patched private page [CC 48 89 e5 ...]  (COW copy)
  Process 2 VMA → patched private page [CC 48 89 e5 ...]  (COW copy)
  ...
  Process 10 VMA → patched private page [CC 48 89 e5 ...] (COW copy)
  Page cache: still original [55 48 89 e5 ...]             (UNCHANGED)

NEW process exec's the binary or mmap's libc:
  uprobe_mmap() is called → installs patched page for new process
  New process immediately gets the patched version
```

### Handling Shared Libraries with Multiple LOAD Segments

ELF shared libraries often have multiple PT_LOAD segments (e.g., RX for code, RW for data). The uprobe code must correctly handle:

```
.text section (executable):  vaddr 0x1000, file offset 0x1000
.rodata section (read-only): vaddr 0x5000, file offset 0x5000
.data section (writable):    vaddr 0x8000, file offset 0x7000  ← different!
```

The file-offset to vaddr mapping is segment-specific, not a simple base+offset.

### Anonymous Mappings (JIT'd Code)

uprobes **cannot** attach to:
- Anonymous memory mappings (no inode)
- JIT-compiled code in JavaScript engines, Java JVM hot code, etc.
- Self-modifying code that isn't file-backed

For JIT'd code, tools like `perf inject --jit` or the Java JVMTI interface create synthetic ELF files in `/tmp/perf-<pid>.map` or similar, enabling some limited post-hoc analysis but not live uprobes.

---

## 26. Multi-threading & SMP Considerations

### The "Parallel Execution" Problem

Consider a function probed with a uprobe, running in 100 threads simultaneously:

```
Thread 1:   executing at probe address → INT3 → trap → handler → XOL slot 0
Thread 2:   executing at probe address → INT3 → trap → handler → XOL slot 1
Thread 3:   executing at probe address → INT3 → trap → handler → XOL slot 2
...
Thread 32:  executing at probe address → INT3 → trap → handler → XOL slot 31
Thread 33:  executing at probe address → INT3 → trap → handler → WAIT (all slots full)
```

The XOL area has a finite number of slots. If exceeded, threads block on:
```c
wait_event(xol_area->wq, slots_available(xol_area));
```

### Locking in the uprobe Core

The uprobe subsystem uses multiple levels of locking:

```
Global:
  uprobes_treelock (spinlock) — protects the global rb-tree of all uprobes
  
Per-uprobe:
  uprobe->register_rwsem — protects the consumer list during registration
  uprobe->consumer_rwsem — protects consumer list during dispatch
  
Per-mm:
  mm->mmap_lock — held during VMA operations (breakpoint install/remove)
  
Per-XOL-area:
  xol_area->wq — waitqueue for slot allocation
  xol_area->bitmap — atomic bitmap for slot management
```

### uprobe on fork() + exec()

```
fork():
  - Child inherits parent's mm (COW)
  - Breakpoint pages are already in child's page tables
  - uprobe_dup_mmap() registers child's mm with all applicable uprobes
  - Child IS probed immediately after fork

exec():
  - exec() replaces the entire mm_struct
  - old VMAs are unmapped → uprobe_munmap() removes breakpoints from old mappings
  - new binary is mapped → uprobe_mmap() installs breakpoints if uprobes exist for new binary's inodes
```

---

## 27. Performance Characteristics & Overhead

### Overhead Breakdown (x86-64, no KPTI)

```
Per uprobe hit:
  User→Kernel ring transition:          ~80 ns
  do_int3 / exception dispatch:         ~20 ns
  uprobe_int3_handler + rb-tree lookup: ~50 ns
  Consumer handler(s):                  varies (10ns–1µs)
  XOL setup + return to user:           ~50 ns
  User executes XOL instruction:        ~10 ns
  #DB / return to kernel:               ~80 ns
  IP fixup + return to user:            ~50 ns
  ─────────────────────────────────────────────
  Total (empty handler):                ~340 ns
  Total (eBPF handler, map lookups):    ~1-5 µs
  Total (KPTI enabled):                 ~2-10× more
```

### With KPTI (Kernel Page Table Isolation, post-Meltdown)

Each user↔kernel transition requires a page-table switch (`cr3` write), which flushes TLB partially. On a system with KPTI:

```
User→Kernel ring transition: ~300-500 ns (vs ~80 ns without KPTI)
```

This makes each uprobe hit cost **~2-5 µs** on KPTI systems for minimal handlers.

### High-Frequency Probe Avoidance

If a function is called **millions of times per second**, upprobing it is impractical. Strategies:
1. **Sample**: only arm the uprobe 1% of the time (probabilistic filtering in eBPF)
2. **Aggregate**: use BPF maps to accumulate stats, read periodically
3. **USDT with reference counter**: probe only fires when actively traced
4. **Uprobes + dynamic throttling**: in BPF, check a rate-limiter variable

Example BPF throttling:
```c
SEC("uprobe//usr/bin/myapp:hot_function")
int trace_hot(struct pt_regs *ctx)
{
    /* Only sample every 1000 calls */
    static __u64 count = 0;
    if ((__sync_fetch_and_add(&count, 1) % 1000) != 0)
        return 0;
    
    /* Record sampled event */
    ...
    return 0;
}
```

### Benchmark: malloc tracing impact

| Tool | Overhead per malloc call |
|---|---|
| No tracing | ~50 ns |
| strace | ~5 µs (ptrace-based) |
| uprobe + empty BPF | ~2 µs |
| uprobe + BPF map write | ~3 µs |
| uprobe + BPF ring buffer | ~4 µs |
| bpftrace one-liner | ~5 µs |

---

## 28. Security Model & Capabilities

### Who Can Use uprobes?

```
Capability required: CAP_PERFMON (Linux 5.8+) or CAP_SYS_ADMIN (older kernels)

perf_event_open with uprobe:  CAP_PERFMON
ftrace uprobe_events:         CAP_PERFMON or CAP_SYS_ADMIN  
BPF + uprobe:                 CAP_BPF + CAP_PERFMON (Linux 5.8+)
Kernel module consumer:       CAP_SYS_MODULE (loading module)
```

### The `perf_event_paranoid` Sysctl

```bash
# /proc/sys/kernel/perf_event_paranoid
# -1: Allow all, including kernel profiling
#  0: Allow perf measurement, disallow raw tracepoints
#  1: Default; disallow per-CPU measurements by unprivileged users
#  2: Allow only own process profiling
#  3: Completely disallow (some distros set this)

cat /proc/sys/kernel/perf_event_paranoid
# Typical: 1 or 2

# Temporarily allow for debugging (root):
echo 0 > /proc/sys/kernel/perf_event_paranoid
```

### Security Implications

uprobes present security considerations:
1. **Information disclosure**: A privileged process can read another process's memory via uprobe arguments
2. **Return value manipulation**: With `bpf_override_return()`, a privileged BPF program can alter the return value of any user-space function (requires `CONFIG_BPF_KPROBE_OVERRIDE`)
3. **Denial of service**: Excessive uprobes on hot paths can severely degrade performance
4. **No kernel corruption**: Unlike kernel modules, uprobe handlers run in process context with normal kernel protections

### Namespaces & Containers

uprobes in containers:
- A container with `CAP_PERFMON` can probe binaries within its own PID namespace
- The host can probe container processes (with appropriate capabilities)
- `/sys/kernel/debug/tracing` is typically hidden in containers (bind mount or seccomp)
- eBPF uprobe programs see the host's PID namespace unless using `bpf_get_ns_current_pid_tgid()`

---

## 29. Limitations & Known Gotchas

### 1. Cannot Probe Anonymous Memory

uprobes require a file-backed inode. You cannot probe:
- `malloc`'d regions used as JIT arenas
- `mmap(MAP_ANONYMOUS)` pages
- Stack-allocated code (uncommon but exists in some JIT scenarios)

### 2. Instruction at Probe Point Must Be Complete

The probe point must be at the **start of a valid instruction**. Probing in the middle of an instruction causes undefined behavior (misaligned INT3 insertion).

Tools like `perf probe` and bpftrace use DWARF debug info to ensure probes land at function boundaries or valid statement addresses.

### 3. Inlined Functions

If a function is inlined by the compiler, there is **no single address to probe**. The function's code exists at multiple call sites. Each call site must be probed individually.

```bash
# Find all inlined instances with perf probe
perf probe -x /usr/bin/myapp --line myfunc
# or
bpftrace --usdt-file-based /usr/bin/myapp -l 'uprobe:*myfunc*'
```

### 4. Stack Frames & Unwinding

When a uprobe fires, the stack unwind (backtrace) is interrupted:
- The XOL slot address appears in stack frames
- Tools that use frame pointer unwinding see the XOL trampoline
- DWARF unwinding (via eh_frame) handles this better

### 5. longjmp / C++ Exceptions with uretprobes

As mentioned earlier, `longjmp` or C++ exception unwinding that skips stack frames with uretprobes active will leave orphaned `return_instance` entries and corrupt the return address on the skipped frame.

The kernel attempts mitigation: if `rsp` moves past the saved frame, the `return_instance` is discarded, but the original function's return address on the stack may have already been overwritten.

### 6. Tail Call Optimization

```c
/* Source: */
int foo() { return bar(); }  /* tail call */

/* Compiled to: */
foo:
    jmp bar    /* NOT a CALL+RET; just a JMP */
```

A uretprobe on `foo` will never fire, because `foo` never executes a `RET`. The entry probe fires, replaces the non-existent return address (the one that was passed to `foo`), but `bar` just jumps back to foo's caller directly.

### 7. GOT/PLT Indirection

When probing shared library functions via the PLT (Procedure Linkage Table):

```
Application calls malloc:
  app:  call malloc@plt         ← probe here misses library code
  PLT:  jmp [malloc@GOT]        ← first call: goes to ld.so resolver
        → resolves to libc:malloc ← probe here gets all malloc calls
```

Always probe the actual function in the shared library, not the PLT stub.

### 8. Stripped Binaries

Stripped binaries have no symbol table. You must:
- Use absolute file offsets (from `objdump`, `readelf -l`, or disassembler)
- Use debug packages (e.g., `libc6-dbg` on Debian) or `debuginfod`
- Use USDT probes (which don't require symbols)

```bash
# Find offset without symbols using objdump
objdump -d /lib/x86_64-linux-gnu/libc.so.6 | grep "<malloc>"
# 0000000000097d60 <malloc>:

# Or use nm
nm -D /lib/x86_64-linux-gnu/libc.so.6 | grep malloc
# 0000000000097d60 T malloc
```

### 9. Kernel Version Dependencies

Some features require specific kernel versions:

```
Kernel 3.5:  Basic uprobes merged
Kernel 3.10: uretprobes stable
Kernel 3.14: uprobe filter support
Kernel 4.6:  uprobe + eBPF (BPF_PROG_TYPE_KPROBE for uprobes)
Kernel 4.17: USDT reference counter support
Kernel 5.5:  bpf_d_path() helper
Kernel 5.8:  CAP_BPF / CAP_PERFMON split from CAP_SYS_ADMIN
Kernel 5.15: RISC-V uprobe support
Kernel 6.0+: Various BPF uprobe improvements
```

---

## 30. Debugging uprobes Themselves

### Is a uprobe Actually Installed?

```bash
# Check registered uprobe events
cat /sys/kernel/debug/tracing/uprobe_events

# Check if enabled
cat /sys/kernel/debug/tracing/events/uprobes/my_probe/enable

# Check if breakpoint is in process memory
# (look for the breakpoint byte 0xCC)
sudo cat /proc/<PID>/mem | hexdump -C | grep -A 1 "probe address"
# Or more precisely:
python3 -c "
import sys
addr = 0x401234  # your probe address
with open('/proc/<PID>/mem', 'rb') as f:
    f.seek(addr)
    data = f.read(16)
print(data.hex())
"
```

### Kernel Debugging with `/sys/kernel/debug/tracing/trace`

```bash
# Enable all uprobe tracing + kernel function tracing
echo function > /sys/kernel/debug/tracing/current_tracer
echo 1 > /sys/kernel/debug/tracing/events/uprobes/enable
cat /sys/kernel/debug/tracing/trace

# Sample output:
#          TASK-PID     CPU#  ||||       TIMESTAMP  FUNCTION
#             | |         |   ||||          |         |
              bash-1234   [001] d...   123.456789: my_malloc_probe: (0x7f.../0x97d60)
```

### Checking uprobe Hit Count

```bash
# perf stat shows how many times a uprobe fired
perf stat -e 'probe_libc:malloc' -p <PID> -- sleep 5
```

### Verifying XOL Area in Process Maps

```bash
# After a uprobe is active on a process
cat /proc/<PID>/maps | grep -E 'uprobe|xol'
# Should show something like:
# 7fff00000000-7fff00001000 r-xp 00000000 00:00 0  [uprobes]
```

### ftrace Debugging

```bash
# Enable verbose uprobe debugging (careful: very noisy)
echo 1 > /sys/kernel/debug/tracing/options/verbose

# View the ring buffer stats
cat /sys/kernel/debug/tracing/per_cpu/cpu0/stats
```

---

## 31. Real-World Use Cases

### 1. Production Performance Profiling

**Problem**: A Node.js application has intermittent latency spikes. No access to source; cannot restart.

```bash
# Profile every function call in node with >1ms latency
bpftrace -e '
uprobe:/usr/bin/node:uv__io_poll { @start[tid] = nsecs; }
uretprobe:/usr/bin/node:uv__io_poll /@start[tid]/ {
    $lat = (nsecs - @start[tid]) / 1000000;
    if ($lat > 1) {
        printf("io_poll took %d ms\n", $lat);
        print(ustack(10));
    }
    delete(@start[tid]);
}'
```

### 2. Security Monitoring

**Problem**: Detect attempts to call `exec*` family functions.

```bash
bpftrace -e '
uprobe:/lib/x86_64-linux-gnu/libc.so.6:execve,
uprobe:/lib/x86_64-linux-gnu/libc.so.6:execvp,
uprobe:/lib/x86_64-linux-gnu/libc.so.6:execvpe
{
    printf("[EXEC ATTEMPT] PID=%d COMM=%s PATH=%s\n",
           pid, comm, str(arg0));
    print(ustack(5));
}'
```

### 3. Zero-Instrumentation Distributed Tracing

**Problem**: Add OpenTelemetry-like tracing to a compiled Go application without modifying it.

Tools like **Odigos**, **Parca**, and **Pixie** use uprobes to automatically instrument:
- HTTP client/server request start/end
- gRPC calls
- Database queries
- Redis/memcached commands

They do this by probing known symbols in `net/http`, `google.golang.org/grpc`, `database/sql`, etc.

### 4. Memory Leak Detection

```python
#!/usr/bin/env python3
# Using BCC (BPF Compiler Collection)

from bcc import BPF
import ctypes

program = """
#include <uapi/linux/ptrace.h>

BPF_HASH(sizes, u64, u64);
BPF_HASH(ptrs, u64, u64);
BPF_PERF_OUTPUT(events);

struct data_t {
    u32 pid;
    u64 size;
    u64 ptr;
    char comm[TASK_COMM_LEN];
    int is_free;
};

int trace_malloc(struct pt_regs *ctx) {
    u64 size = PT_REGS_PARM1(ctx);
    u64 tid = bpf_get_current_pid_tgid();
    sizes.update(&tid, &size);
    return 0;
}

int trace_malloc_ret(struct pt_regs *ctx) {
    u64 tid = bpf_get_current_pid_tgid();
    u64 *size_ptr = sizes.lookup(&tid);
    if (!size_ptr) return 0;
    
    u64 ptr = PT_REGS_RC(ctx);
    ptrs.update(&ptr, size_ptr);
    sizes.delete(&tid);
    return 0;
}

int trace_free(struct pt_regs *ctx) {
    u64 ptr = PT_REGS_PARM1(ctx);
    ptrs.delete(&ptr);
    return 0;
}
"""

b = BPF(text=program)
libc = "/lib/x86_64-linux-gnu/libc.so.6"

b.attach_uprobe(name=libc, sym="malloc",     fn_name="trace_malloc")
b.attach_uretprobe(name=libc, sym="malloc",  fn_name="trace_malloc_ret")
b.attach_uprobe(name=libc, sym="free",       fn_name="trace_free")

print("Tracking allocations... Ctrl-C to show leaks")
try:
    import time
    while True:
        time.sleep(1)
except KeyboardInterrupt:
    print("\nPotential leaks (allocations without corresponding free):")
    ptrs = b["ptrs"]
    total = 0
    for ptr, size in sorted(ptrs.items(), key=lambda x: x[1].value, reverse=True)[:20]:
        print(f"  0x{ptr.value:016x}: {size.value} bytes")
        total += size.value
    print(f"\nTotal: {total} bytes in {len(ptrs)} allocations")
```

### 5. SSL/TLS Traffic Inspection

```bash
# Inspect SSL data before encryption / after decryption
# Works even with HTTPS (no need to intercept certificates)

bpftrace -e '
uprobe:/lib/x86_64-linux-gnu/libssl.so.3:SSL_write
{
    printf("SSL_write: pid=%d size=%d\n", pid, arg2);
    printf("%s\n", buf(arg1, arg2 < 200 ? arg2 : 200));
}

uprobe:/lib/x86_64-linux-gnu/libssl.so.3:SSL_read_ex
{
    @ssl_ctx[tid] = arg1;  /* save buf pointer */
    @ssl_len[tid] = arg2;
}

uretprobe:/lib/x86_64-linux-gnu/libssl.so.3:SSL_read_ex
/@ssl_ctx[tid]/
{
    printf("SSL_read: pid=%d size=%d\n", pid, retval);
    printf("%s\n", buf(@ssl_ctx[tid], @ssl_len[tid] < 200 ? @ssl_len[tid] : 200));
    delete(@ssl_ctx[tid]);
    delete(@ssl_len[tid]);
}'
```

### 6. Flame Graph Generation

```bash
# Generate a CPU flame graph using uprobes + perf

# 1. Profile with ustack capture
bpftrace -e '
profile:hz:99 {
    @stacks[ustack()] = count();
}' -o /tmp/stacks.bt &

# 2. Or via perf record
perf record -F 99 -g -p <PID> -- sleep 30

# 3. Generate flame graph
perf script | stackcollapse-perf.pl | flamegraph.pl > flame.svg
```

---

## 32. Mental Model Summary

Understanding uprobes deeply requires holding several interconnected concepts simultaneously. Here is a synthesized mental model:

### The Core Abstraction

```
A uprobe is a persistent, inode-scoped observation point.
It says: "whenever any CPU executes instruction at (inode, offset),
          call these handlers — transparently, without the process knowing."
```

### The Three Layers

```
Layer 1: IDENTIFICATION (inode + offset)
  - Stable across ASLR, forks, exec
  - inode = file identity, offset = position within file
  - Translated per-process to virtual addresses

Layer 2: MECHANISM (breakpoint + XOL)
  - INT3/BRK replaces first byte/word of instruction
  - Kernel catches trap, calls handlers
  - Original instruction runs safely in XOL trampoline
  - Completely transparent to target process

Layer 3: CONSUMERS (what actually runs)
  - perf ring buffer writes
  - eBPF programs (most powerful: full language, maps, ring buffers)
  - ftrace records
  - Custom kernel module handlers
  Multiple consumers can share one physical probe point.
```

### The Data Flow

```
Registration time:
  (binary path + symbol/offset) → inode lookup → file offset
  → uprobe struct in rb-tree → consumer added to list
  → breakpoint written into process page tables (per-mm)

Execution time (hot path):
  INT3 trap → kernel exception handler → uprobe dispatch
  → find uprobe in rb-tree (by mm + vaddr → inode + offset)
  → call each consumer's handler(regs)
  → copy insn to XOL → single-step → IP fixup → resume

Teardown time:
  consumer removed → uprobe refcount → 0 → remove breakpoint
  → page table restored → original instruction bytes back
```

### The Key Insight About Cost

```
INACTIVE uprobe (registered but process not running probed code):
  Cost = ZERO. The probe is just bytes in a page table.

ACTIVE uprobe (being hit):
  Cost per hit ≈ 1-10 µs (hardware-mediated, unavoidable ring transitions)
  This is ~10-100× slower than a kprobe (same probe, kernel side)
  Reason: user↔kernel ring switches dominate
```

### When to Use What

```
Goal: Observe production system without restart → uprobe or USDT
Goal: Maximum performance sensitivity → USDT (NOP when inactive, check refctr)
Goal: Kernel code → kprobe (not uprobe)
Goal: Script-level one-liners → bpftrace + uprobe
Goal: Production monitoring daemon → libbpf + uprobe + ringbuf
Goal: Legacy toolchain → SystemTap or perf probe
Goal: Structured, application-defined tracepoints → USDT
Goal: Debug a crash in production now → perf probe + perf record
Goal: Long-term observability framework → USDT + eBPF
```

### Complete System View (ASCII)

```
 ╔════════════════════════════════════════════════════════════════╗
 ║                    UPROBE COMPLETE SYSTEM                      ║
 ╠════════════════════════════════════════════════════════════════╣
 ║                                                                ║
 ║  TOOLING LAYER                                                 ║
 ║  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐ ║
 ║  │bpftrace  │ │  BCC /   │ │  perf    │ │ Custom kernel    │ ║
 ║  │one-liner │ │ libbpf   │ │  probe   │ │ module consumer  │ ║
 ║  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────────┬─────────┘ ║
 ║       │             │            │                 │           ║
 ║       └──────┬───────┘            └────────┬───────┘           ║
 ║              │ (syscall layer)             │ (direct API)       ║
 ╠══════════════╪═════════════════════════════╪════════════════════╣
 ║  KERNEL      │                             │                   ║
 ║  INTERFACE   ▼                             ▼                   ║
 ║  ┌───────────────────────┐   ┌─────────────────────────────┐  ║
 ║  │  perf_event_open(2)   │   │ uprobe_register() API       │  ║
 ║  │  + bpf(2)             │   │ (kernel internal)           │  ║
 ║  │  + tracefs writes     │   └──────────────┬──────────────┘  ║
 ║  └──────────┬────────────┘                  │                  ║
 ║             │                               │                  ║
 ║             ▼                               ▼                  ║
 ║  ┌──────────────────────────────────────────────────────────┐  ║
 ║  │                 uprobe CORE                              │  ║
 ║  │                                                          │  ║
 ║  │  Global rb-tree: (inode, offset) → struct uprobe         │  ║
 ║  │  Each uprobe → [consumer list] → handlers                │  ║
 ║  │                                                          │  ║
 ║  │  mmap notifier: install breakpoints on new mappings      │  ║
 ║  │  munmap notifier: remove breakpoints on unmappings       │  ║
 ║  │  fork notifier: replicate probes to child mm             │  ║
 ║  └────────────────────────┬─────────────────────────────────┘  ║
 ║                           │                                     ║
 ║                           ▼                                     ║
 ║  ┌──────────────────────────────────────────────────────────┐  ║
 ║  │              ARCHITECTURE LAYER                          │  ║
 ║  │                                                          │  ║
 ║  │  x86-64: INT3 (0xCC), #BP exception, TF single-step      │  ║
 ║  │  ARM64:  BRK #0, sync exception, SS single-step          │  ║
 ║  │  Both:   XOL trampoline in user-space anonymous vma      │  ║
 ║  │  Both:   IP-relative instruction fixup                   │  ║
 ║  └────────────────────────┬─────────────────────────────────┘  ║
 ╠════════════════════════════╪═════════════════════════════════════╣
 ║  USER SPACE                │                                    ║
 ║            ┌───────────────▼────────────────┐                  ║
 ║            │      Target Process             │                  ║
 ║            │                                 │                  ║
 ║            │  .text:                         │                  ║
 ║            │  [CC] rest of instruction...    │← INT3 patched    ║
 ║            │                                 │                  ║
 ║            │  XOL vma:                       │                  ║
 ║            │  [original insn][INT3]          │← executes here   ║
 ║            │                                 │  during sstep    ║
 ║            └─────────────────────────────────┘                  ║
 ║                                                                  ║
 ╚══════════════════════════════════════════════════════════════════╝
```

---

## References & Further Reading

### Kernel Source Files

```
kernel/events/uprobes.c         — Core uprobe implementation
include/linux/uprobes.h         — Public API definitions
arch/x86/kernel/uprobes.c       — x86-specific uprobe code
arch/arm64/kernel/probes/uprobes.c — ARM64-specific code
kernel/trace/trace_uprobe.c     — ftrace/perf_events consumer
```

### Essential Reading

- **LWN: User-space probes (uprobes)** — https://lwn.net/Articles/499190/
- **LWN: Uprobes in 3.5** — https://lwn.net/Articles/499190/
- **Brendan Gregg's BPF Performance Tools** — the definitive practical reference
- **Linux Kernel Tracing** by Steven Rostedt — deep ftrace/uprobe integration
- **bpftrace Reference Guide** — https://github.com/bpftrace/bpftrace/blob/master/docs/reference_guide.md
- **libbpf Documentation** — https://libbpf.readthedocs.io/

### Kernel Commits to Study

```bash
# Original uprobe merge
git log --oneline v3.4..v3.5 -- kernel/events/uprobes.c

# Key commit: uprobes infrastructure
# Author: Srikar Dronamraju <srikar@linux.vnet.ibm.com>
git show 2b144498950ff0c817 2011-xx-xx

# uretprobes
git log --oneline -- kernel/events/uprobes.c | grep -i "ret\|return"
```

---

*This guide covers uprobes as of Linux kernel 6.x. The core mechanisms have been stable since 3.5, with incremental improvements. The BPF/eBPF integration continues to evolve and is the recommended interface for new tooling.*
