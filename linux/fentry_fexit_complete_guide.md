# fentry / fexit — Complete In-Depth Guide

> A complete reference covering kernel internals, architecture, BTF, attachment
> mechanics, performance, C and Rust implementations, and mental models for
> thinking about Linux tracing with fentry/fexit.

---

## Table of Contents

1. [Conceptual Foundation — Why fentry/fexit Exists](#1-conceptual-foundation)
2. [Historical Context — The Road from kprobes](#2-historical-context)
3. [Kernel Architecture — How It Works Under the Hood](#3-kernel-architecture)
4. [BTF — The Backbone of fentry/fexit](#4-btf-the-backbone)
5. [Trampoline Mechanism — The Performance Heart](#5-trampoline-mechanism)
6. [Attachment — Hooking a Function](#6-attachment)
7. [Argument Access — Reading Kernel State](#7-argument-access)
8. [Return Value Access (fexit)](#8-return-value-access)
9. [fentry vs fexit — Differences and Use Cases](#9-fentry-vs-fexit)
10. [fentry vs kprobes/kretprobes — Detailed Comparison](#10-fentry-vs-kprobes)
11. [fentry vs tracepoints — Detailed Comparison](#11-fentry-vs-tracepoints)
12. [Verifier Rules for fentry/fexit Programs](#12-verifier-rules)
13. [C Implementation — Complete Examples](#13-c-implementation)
14. [Rust Implementation — Complete Examples](#14-rust-implementation)
15. [Multi-attach and Sharing Trampolines](#15-multi-attach)
16. [sleepable fentry/fexit (BPF_F_SLEEPABLE)](#16-sleepable)
17. [Struct-ops and fentry Interaction](#17-struct-ops-interaction)
18. [Tail Calls and fentry](#18-tail-calls)
19. [Real-World Use Cases](#19-real-world-use-cases)
20. [Debugging and Introspection](#20-debugging)
21. [Kernel Version Compatibility Matrix](#21-compatibility)
22. [Gotchas, Pitfalls, Limitations](#22-gotchas)
23. [Mental Models — Thinking in fentry/fexit](#23-mental-models)
24. [Glossary](#24-glossary)

---

## 1. Conceptual Foundation

### What is fentry/fexit?

`fentry` and `fexit` are **eBPF program types** that attach to the **entry** and
**exit points** of any kernel function that has BTF (BPF Type Format) type
information available. They allow you to:

- Observe every call to a kernel function (fentry)
- Observe every return from a kernel function, plus what it returned (fexit)
- Read all arguments with their **true types** (not void pointers)
- Do all of this with extremely low overhead

The key mental model is:

```
                  kernel function call path
                  ─────────────────────────
caller ──► [fentry BPF program runs] ──► actual_kernel_function() ──► [fexit BPF program runs] ──► caller
```

They are **non-modifying observers**. They cannot alter arguments, return values,
or control flow (unlike `bpf_override_return` with kprobes).

### The Core Promise

| Property          | Value                                              |
|-------------------|----------------------------------------------------|
| Overhead          | ~10–20 ns per invocation (vs ~100+ ns for kprobes) |
| Type safety       | Full BTF-backed argument types                     |
| Stability         | Kernel symbol stability (not ABI, but symbol names)|
| Granularity       | Per-function, any kernel function with BTF         |
| Modifiability     | Read-only (cannot alter args or return values)     |

---

## 2. Historical Context

### The Evolution of Linux Kernel Tracing

```
Timeline of kernel tracing mechanisms:
─────────────────────────────────────────────────────────────────────────────

1991  ┌──────────────────────────────────────────────────────────────────┐
      │  Linux 0.01 — no tracing infrastructure                         │
      └──────────────────────────────────────────────────────────────────┘

2004  ┌──────────────────────────────────────────────────────────────────┐
      │  kprobes (Linux 2.6.9)                                           │
      │  • Dynamic instrumentation via INT3 / single-step               │
      │  • kretprobes for return value capture                           │
      │  • Works on any compiled symbol                                  │
      │  • High overhead: exception handling path                        │
      └──────────────────────────────────────────────────────────────────┘

2008  ┌──────────────────────────────────────────────────────────────────┐
      │  Ftrace (Linux 2.6.27)                                           │
      │  • Compiler-inserted mcount/fentry stubs                         │
      │  • GCC -pg / -mfentry flags add call at function start           │
      │  • Used by function_graph_tracer, function_tracer                │
      │  • The mcount/fentry NOP infrastructure is what BPF later reuses │
      └──────────────────────────────────────────────────────────────────┘

2009  ┌──────────────────────────────────────────────────────────────────┐
      │  Tracepoints (Linux 2.6.32)                                      │
      │  • Stable ABI markers in kernel source                           │
      │  • Explicit TRACE_EVENT macros by kernel developers              │
      │  • Static but stable; only where developers placed them          │
      └──────────────────────────────────────────────────────────────────┘

2014  ┌──────────────────────────────────────────────────────────────────┐
      │  eBPF arrives (Linux 3.18)                                       │
      │  • Extended BPF with maps, verifier, JIT                         │
      │  • kprobe-backed BPF programs (BPF_PROG_TYPE_KPROBE)             │
      └──────────────────────────────────────────────────────────────────┘

2019  ┌──────────────────────────────────────────────────────────────────┐
      │  BTF introduced (Linux 5.2)                                      │
      │  • Compact type information embedded in kernel                   │
      │  • Foundation for type-aware BPF programs                        │
      └──────────────────────────────────────────────────────────────────┘

2020  ┌──────────────────────────────────────────────────────────────────┐
      │  fentry/fexit (Linux 5.5)  ← We are here                        │
      │  • BPF_PROG_TYPE_TRACING with BPF_TRACE_FENTRY/FEXIT             │
      │  • Uses ftrace trampoline infrastructure                          │
      │  • BTF-backed argument types — no casting needed                 │
      │  • Direct call via trampoline, not exception-based               │
      └──────────────────────────────────────────────────────────────────┘

2021  ┌──────────────────────────────────────────────────────────────────┐
      │  Sleepable fentry/fexit (Linux 5.10)                             │
      │  • BPF_F_SLEEPABLE flag                                          │
      │  • Can use sleepable helpers (bpf_copy_from_user, etc.)          │
      └──────────────────────────────────────────────────────────────────┘

2022+ ┌──────────────────────────────────────────────────────────────────┐
      │  Multi-attach trampolines, struct-ops extensions, more           │
      └──────────────────────────────────────────────────────────────────┘
```

### Why kprobes Were Not Enough

kprobes work by:
1. Replacing the first byte of an instruction with `0xCC` (INT3 breakpoint)
2. When the CPU hits `0xCC`, it raises an exception
3. The exception handler checks if it's a kprobe, saves all registers, calls your handler
4. Single-steps the original instruction, re-patches, continues

This is **expensive** because:
- Exception handling crosses privilege boundaries in complex ways
- All registers must be saved/restored (full pt_regs)
- The single-step mechanism is slow
- No type information → arguments are raw `void *` or `unsigned long`

fentry/fexit sidestep all of this.

---

## 3. Kernel Architecture

### The Big Picture

```
 ┌─────────────────────────────────────────────────────────────────────┐
 │                        KERNEL ADDRESS SPACE                         │
 │                                                                      │
 │  ┌─────────────────────────────────────────────────────────────┐    │
 │  │              COMPILED KERNEL FUNCTION                        │    │
 │  │                                                              │    │
 │  │  do_sys_openat2:                                             │    │
 │  │    e8 xx xx xx xx   call __fentry__    ◄── NOP or live call  │    │
 │  │    55               push %rbp                               │    │
 │  │    48 89 e5         mov  %rsp, %rbp                         │    │
 │  │    ...              (function body)                          │    │
 │  │    c3               ret                                      │    │
 │  └──────────────────┬──────────────────────────────────────────┘    │
 │                     │                                                 │
 │           When BPF  │  attaches, ftrace infrastructure patches       │
 │           the call  │  site so __fentry__ leads to the trampoline    │
 │                     │                                                 │
 │                     ▼                                                 │
 │  ┌─────────────────────────────────────────────────────────────┐    │
 │  │                  BPF TRAMPOLINE (JIT-generated)              │    │
 │  │                                                              │    │
 │  │  Generated at attach time. Lives in BPF memory area.         │    │
 │  │                                                              │    │
 │  │  ┌─────────────────────────────────────────────────────┐    │    │
 │  │  │  ENTRY TRAMPOLINE                                    │    │    │
 │  │  │  1. Save args: rdi, rsi, rdx, rcx, r8, r9 (SysV)   │    │    │
 │  │  │  2. Call fentry BPF program (direct call, JITed)    │    │    │
 │  │  │  3. Restore args                                     │    │    │
 │  │  │  4. Return to actual kernel function body            │    │    │
 │  │  └─────────────────────────────────────────────────────┘    │    │
 │  │                                                              │    │
 │  │  ┌─────────────────────────────────────────────────────┐    │    │
 │  │  │  EXIT TRAMPOLINE                                     │    │    │
 │  │  │  1. Wraps return path of the original function       │    │    │
 │  │  │  2. Saves args + return value (rax/rax:rdx)          │    │    │
 │  │  │  3. Calls fexit BPF program                          │    │    │
 │  │  │  4. Restores return value, returns to caller         │    │    │
 │  │  └─────────────────────────────────────────────────────┘    │    │
 │  └─────────────────────────────────────────────────────────────┘    │
 │                                                                      │
 │  ┌─────────────────────────────────────────────────────────────┐    │
 │  │              BPF VERIFIER + JIT COMPILER                     │    │
 │  │  • Validates program safety at load time                     │    │
 │  │  • Checks argument types match BTF prototype                 │    │
 │  │  • JIT compiles to native x86-64 (or arm64, s390x, etc.)    │    │
 │  └─────────────────────────────────────────────────────────────┘    │
 │                                                                      │
 │  ┌─────────────────────────────────────────────────────────────┐    │
 │  │              BTF (BPF Type Format)                           │    │
 │  │  • Embedded in vmlinux or module .ko                         │    │
 │  │  • Describes every struct, enum, function prototype          │    │
 │  │  • Read by verifier to validate arg access                   │    │
 │  └─────────────────────────────────────────────────────────────┘    │
 └─────────────────────────────────────────────────────────────────────┘
```

### The Call Flow in Detail

```
User Process                   Kernel                        BPF
─────────────                  ──────                        ───

syscall(open)
    │
    ▼
do_sys_openat2()  ◄──── first instruction: call __fentry__
    │                                              │
    │                   ftrace sees registered     │
    │                   ops for this symbol        │
    │                              │               │
    │                              ▼               │
    │                   [BPF Trampoline Entry]
    │                      save rdi,rsi,rdx...
    │                      call JITed_fentry_prog ──────────► fentry BPF prog runs
    │                      restore regs                            reads args
    │                   return                                     writes maps
    │                              │
    ▼                              │
[function body]  ◄─────────────────┘
    │
    │  (function executes normally)
    │
    ▼
return (rax = fd)
    │
    │          ┌── fexit trampoline intercepts return
    │          │
    ▼          ▼
[BPF Trampoline Exit]
    save rdi,rsi,...,rax
    call JITed_fexit_prog ──────────────────────────────► fexit BPF prog runs
    restore rax                                               reads args + retval
                                                              writes maps
    return to caller
```

### Kernel Source Paths (Linux kernel)

The implementation spans several files:

```
linux/
├── kernel/bpf/
│   ├── trampoline.c          ← Core trampoline allocation, arch_prepare_bpf_trampoline()
│   ├── btf.c                 ← BTF parsing, btf_check_func_arg_match()
│   ├── verifier.c            ← check_attach_btf_id(), argument validation
│   └── syscall.c             ← bpf_prog_load(), link creation
├── arch/x86/net/
│   └── bpf_jit_comp.c        ← arch_prepare_bpf_trampoline() for x86_64
├── arch/arm64/net/
│   └── bpf_jit_comp.c        ← ARM64 trampoline
├── include/linux/
│   ├── bpf.h                 ← BPF_PROG_TYPE_TRACING, bpf_tramp_*
│   └── btf.h                 ← BTF structures
└── kernel/trace/
    └── bpf_trace.c           ← fentry/fexit prog ops, context definitions
```

---

## 4. BTF — The Backbone

### What BTF Is

BTF (BPF Type Format) is a compact, deduplicated type system embedded in the
kernel binary (vmlinux) and loaded loadable kernel modules (.ko). It was designed
specifically for BPF but has become a general kernel introspection mechanism.

Think of BTF as the kernel's **debug information** with two key differences:
- It is **extremely compact** (vmlinux BTF is ~5MB vs ~500MB of DWARF)
- It is **always present** in production kernels when `CONFIG_DEBUG_INFO_BTF=y`

```
 DWARF (traditional debug info)            BTF (BPF Type Format)
 ─────────────────────────────             ──────────────────────
 Size: 300-600 MB                          Size: 3-6 MB
 Format: complex, hierarchical             Format: flat array of type descriptors
 Embedded in: separate .debug section      Embedded in: .BTF section of vmlinux
 Loaded: by debuggers on demand            Loaded: by kernel at boot
 Deduplicated: no                          Deduplicated: yes (pahole --btf_encode)
 Runtime accessible: no                    Runtime accessible: yes (/sys/kernel/btf/)
```

### BTF Structure (Simplified)

```
 vmlinux BTF blob:
 ┌────────────────────────────────────────────────────────────┐
 │  btf_header                                                │
 │  ┌──────────┬───────────────────────────────────────────┐ │
 │  │ type_len │ type_data[] ── array of btf_type structs  │ │
 │  └──────────┴───────────────────────────────────────────┘ │
 │  ┌──────────┬───────────────────────────────────────────┐ │
 │  │ str_len  │ string_section ── null-terminated strings  │ │
 │  └──────────┴───────────────────────────────────────────┘ │
 └────────────────────────────────────────────────────────────┘

 Each btf_type encodes one of:
   BTF_KIND_INT         — integer (u8, u16, u32, u64, etc.)
   BTF_KIND_PTR         — pointer to another type
   BTF_KIND_STRUCT      — struct with named fields and offsets
   BTF_KIND_UNION       — union
   BTF_KIND_ENUM        — enum values
   BTF_KIND_FUNC        — function (name, proto type ID)
   BTF_KIND_FUNC_PROTO  — (return type, param types, param names)
   BTF_KIND_ARRAY       — array
   BTF_KIND_TYPEDEF     — typedef alias
   BTF_KIND_CONST/VOLATILE/RESTRICT — qualifiers
```

### How BTF Enables fentry/fexit

When you write `SEC("fentry/do_sys_openat2")` in your BPF program:

1. The loader reads the function name `do_sys_openat2`
2. It looks up this name in the kernel's BTF
3. BTF says: `do_sys_openat2(int dfd, struct filename *filename, struct open_how *how)`
4. The verifier enforces that your BPF program's first three arguments match those types
5. The trampoline is generated to pass those exact arguments

Without BTF, this type-safe argument passing is impossible.

### Accessing Kernel BTF

```c
// From userspace, kernel BTF is exposed at:
// /sys/kernel/btf/vmlinux

// You can inspect it with bpftool:
// $ bpftool btf dump file /sys/kernel/btf/vmlinux format raw
// $ bpftool btf dump id 0

// Or programmatically:
int fd = open("/sys/kernel/btf/vmlinux", O_RDONLY);
struct btf *btf = btf__load_from_kernel_by_id(0); // libbpf API
```

### BTF for Modules

Kernel modules have their own BTF:

```
/sys/kernel/btf/
├── vmlinux        ← core kernel BTF
├── nf_conntrack   ← conntrack module BTF
├── kvm            ← KVM module BTF
└── ...
```

If you want to fentry-attach to a function inside a module, the loader must find
the BTF ID in the module's BTF file, not vmlinux.

---

## 5. Trampoline Mechanism

This is the most important architectural concept. Understanding the trampoline
is what makes fentry/fexit's performance advantage clear.

### The mcount/fentry NOP — Compiler Infrastructure

Every compiled kernel function (when `CONFIG_DYNAMIC_FTRACE=y`) starts with a
special instruction:

```asm
; x86_64 — function compiled with -mfentry (GCC) or -fpatchable-function-entry (Clang)

do_sys_openat2:
    e8 00 00 00 00     call __fentry__   ; 5-byte CALL instruction
    ; OR, when no tracer is active, this is replaced with:
    66 66 66 66 90     nop nop nop nop nop  ; 5-byte NOP (same size!)
    ; The kernel patches this at boot via apply_paravirt_fixups()
```

The crucial insight: **a 5-byte NOP and a 5-byte CALL are the same byte width**,
so the kernel can atomically patch one to the other using text_poke_bp().

```
Function start — NO tracer active:
┌─────────────────────────────────────────────┐
│  NOP NOP NOP NOP NOP  (5 bytes)             │
│  PUSH RBP                                   │
│  MOV  RSP, RBP                              │
│  ...function body...                        │
└─────────────────────────────────────────────┘

Function start — fentry/ftrace tracer active:
┌─────────────────────────────────────────────┐
│  CALL <trampoline_addr>  (5 bytes)          │  ← patched by register_ftrace_function()
│  PUSH RBP                                   │
│  MOV  RSP, RBP                              │
│  ...function body...                        │
└─────────────────────────────────────────────┘
```

### Trampoline Generation (arch_prepare_bpf_trampoline)

The trampoline is a small piece of JIT-compiled machine code generated at attach
time. Here is its structure for x86_64 (pseudo-assembly):

```
BPF Trampoline for fentry+fexit on func(arg0, arg1, arg2) -> retval:

─────────────────────────────────────────
ENTRY SECTION:
─────────────────────────────────────────
trampoline_entry:
    ; Called like a function: trampoline is inserted via the call __fentry__ slot
    ; At entry: rdi=arg0, rsi=arg1, rdx=arg2, stack has return address

    sub  rsp, STACK_SIZE          ; allocate stack frame
    mov  [rsp+0],  rdi            ; save arg0
    mov  [rsp+8],  rsi            ; save arg1
    mov  [rsp+16], rdx            ; save arg2
    ; (save more regs if more args)

    ; --- call fentry BPF programs ---
    lea  rdi, [rsp+0]             ; rdi → pointer to saved args array
    call bpf_prog_fentry_1        ; direct call to JITed BPF prog 1
    ; (repeat for each attached fentry program)

    ; restore args for the actual function
    mov  rdi, [rsp+0]
    mov  rsi, [rsp+8]
    mov  rdx, [rsp+16]

─────────────────────────────────────────
FEXIT WRAPPING:
─────────────────────────────────────────
    ; For fexit, the trampoline does not just call and return.
    ; Instead it:
    ;   1. Calls the real function by returning to it normally (fentry part ends)
    ;   2. Patches the return address on the stack to point to fexit_return stub

    ; Replace return address to intercept the return:
    mov  rax, [rsp+STACK_SIZE]    ; original caller return addr
    mov  [saved_ret_addr], rax
    lea  rax, fexit_return_stub
    mov  [rsp+STACK_SIZE], rax    ; now function will "return" to our stub

    add  rsp, STACK_SIZE
    ret                           ; "return" to actual function (which runs normally)

─────────────────────────────────────────
fexit_return_stub:
─────────────────────────────────────────
    ; Called when the instrumented function executes its RET
    ; rax = return value (or rax:rdx for 128-bit)

    sub  rsp, STACK_SIZE2
    mov  [rsp+0],  rdi            ; save args again (restored from earlier save)
    ; ...
    mov  [rsp+RET_OFF], rax       ; save return value

    ; --- call fexit BPF programs ---
    lea  rdi, [rsp+0]             ; pointer to args+retval array
    call bpf_prog_fexit_1
    ; (repeat for each attached fexit program)

    mov  rax, [rsp+RET_OFF]       ; restore return value
    add  rsp, STACK_SIZE2
    jmp  [saved_ret_addr]         ; jump to original caller
```

> **Key insight**: The trampoline uses **direct calls**, not exception-based
> dispatch. This is why fentry/fexit is ~10x faster than kprobes.

### Text Patching Safety

The kernel uses `text_poke_bp()` to atomically replace the NOP with a CALL.
The "bp" stands for breakpoint — it temporarily inserts an INT3, lets all CPUs
quiesce past it, then writes the new CALL instruction and removes the INT3.
This is safe even on SMP systems.

```
 CPU 0 thread          CPU 1 thread          CPU 2 thread
 ─────────────         ─────────────         ─────────────
 text_poke_bp()
   write INT3 at target
   IPI all other CPUs ──────────────────────► stop_machine or sync
                                              ◄── ack
   write 4 bytes of CALL
   remove INT3
   IPI done ────────────────────────────────► resume
```

---

## 6. Attachment

### BPF Program Type and Expected Attach Type

fentry/fexit programs use:
- `prog_type = BPF_PROG_TYPE_TRACING`
- `expected_attach_type = BPF_TRACE_FENTRY` or `BPF_TRACE_FEXIT`
- `attach_btf_id` = the BTF ID of the target function

### Section Name Convention (libbpf)

```c
// libbpf uses SEC() to derive prog type and target from the section name:
SEC("fentry/do_sys_openat2")    // attaches to kernel function do_sys_openat2
SEC("fexit/do_sys_openat2")     // attaches to exit of do_sys_openat2

// For module functions:
SEC("fentry/bpf_testmod/bpf_testmod_test_read")
```

### Kernel-Side Attachment Path

```
bpf_prog_load()
    │
    ├── check prog_type == BPF_PROG_TYPE_TRACING
    ├── check expected_attach_type ∈ {FENTRY, FEXIT, ...}
    ├── check_attach_btf_id()
    │       ├── look up function name in BTF
    │       ├── resolve BTF_FUNC → BTF_FUNC_PROTO
    │       ├── validate prototype against BPF prog signature
    │       └── store btf_id in prog->aux->attach_btf_id
    └── bpf_check() (verifier)
            └── check_func_arg_reg_off() for each arg

bpf_link_create(BPF_TRACE_FENTRY/FEXIT)
    │
    ├── bpf_tracing_link_attach()
    ├── bpf_trampoline_get() — find or create trampoline for this BTF ID
    │       └── hlist keyed by (btf_id, is_module)
    ├── bpf_trampoline_link_prog()
    │       ├── add prog to trampoline->progs_hlist
    │       └── if first prog: arch_prepare_bpf_trampoline() + register_ftrace_function()
    │               ├── allocate RWX memory for trampoline code
    │               ├── JIT-compile the entry/exit stubs
    │               └── text_poke_bp() to activate the CALL
    └── return link_fd
```

### Link vs Direct Attachment

Since Linux 5.7, `BPF_LINK_CREATE` is the preferred attachment method. Links:
- Are reference-counted
- Can be pinned in bpffs
- Are automatically cleaned up when the fd is closed or the process exits

Old-style `bpf_raw_tracepoint_open` is not used for fentry/fexit.

```c
// Attaching via bpf_link_create (C, raw syscall):
union bpf_attr attr = {
    .link_create = {
        .prog_fd       = prog_fd,
        .target_btf_id = btf_id,   // found by name lookup
        .attach_type   = BPF_TRACE_FENTRY,
    },
};
int link_fd = bpf(BPF_LINK_CREATE, &attr, sizeof(attr));
```

---

## 7. Argument Access

### How Arguments Are Presented to BPF Programs

When a fentry program runs, the BPF context (`ctx`) is actually a pointer to an
array of arguments laid out according to the target function's BTF prototype.

The verifier rewrites argument accesses using knowledge of the types. From the
BPF program's perspective:

```c
// Kernel function being traced:
// long do_sys_openat2(int dfd, struct filename *filename, struct open_how *how);

// fentry BPF program:
SEC("fentry/do_sys_openat2")
int BPF_PROG(fentry_openat2,
             int dfd,                    // arg 0 — actual type: int
             struct filename *filename,  // arg 1 — actual type: struct filename *
             struct open_how *how)       // arg 2 — actual type: struct open_how *
{
    // args have real types — no casting!
    bpf_printk("openat2: dfd=%d\n", dfd);
    // can dereference pointers safely (verifier tracks nullability)
    return 0;
}
```

### The BPF_PROG Macro

The `BPF_PROG` macro (from `bpf/bpf_tracing.h`) expands the typed argument list
into the correct context access pattern:

```c
// What BPF_PROG expands to (simplified):
#define BPF_PROG(name, args...)                         \
int name(unsigned long long *ctx)                       \
{                                                       \
    /* verifier knows ctx[0] = arg0, ctx[1] = arg1 ... */ \
    _Pragma("GCC diagnostic push")                      \
    /* individual typed args extracted from ctx[] */    \
    return ____##name(ctx, ##args);                     \
}                                                       \
static int ____##name(unsigned long long *ctx, args)
```

Under the hood, each `argN` is read as `ctx[N]` and cast to the correct type as
known by the verifier via BTF.

### Register-to-Memory Mapping (x86_64 SysV ABI)

```
 Argument #   Register   ctx[] index   Stack offset in trampoline
 ──────────   ────────   ───────────   ─────────────────────────
 0            rdi        ctx[0]        [rsp + 0]
 1            rsi        ctx[1]        [rsp + 8]
 2            rdx        ctx[2]        [rsp + 16]
 3            rcx        ctx[3]        [rsp + 24]
 4            r8         ctx[4]        [rsp + 32]
 5            r9         ctx[5]        [rsp + 40]
 6+           stack      ctx[6+]       (from original caller's stack)
```

### Pointer Dereferencing Rules

```c
SEC("fentry/tcp_sendmsg")
int BPF_PROG(trace_tcp_sendmsg,
             struct sock *sk,       // pointer to kernel struct
             struct msghdr *msg,
             size_t size)
{
    // Dereferencing kernel pointers:
    // Method 1: BPF_CORE_READ (preferred — uses CO-RE relocations)
    __u32 src_port = BPF_CORE_READ(sk, __sk_common.skc_num);

    // Method 2: bpf_probe_read_kernel (always works, older)
    __u32 dst_port;
    bpf_probe_read_kernel(&dst_port, sizeof(dst_port),
                          &sk->__sk_common.skc_dport);

    // Direct dereference (works if verifier can prove non-NULL):
    // __u32 state = sk->sk_state;  // this may or may not be allowed

    return 0;
}
```

### CO-RE (Compile Once, Run Everywhere)

CO-RE is what makes fentry programs portable across kernel versions:

```
 Source Code:              vmlinux.h:               Kernel BTF:
 ──────────────            ─────────                ────────────
 BPF_CORE_READ(sk,  ──► struct sock {          ──► offset of skc_num
   __sk_common.skc_num)    struct sock_common          in this kernel's
                             __sk_common;              actual struct sock
                           ...
                         };

 At load time, libbpf resolves the field offset from the running kernel's
 BTF, adjusting for struct layout differences between kernel versions.
```

---

## 8. Return Value Access

### fexit — Arguments AND Return Value

fexit programs receive all arguments the original function received (from the
trampoline's saved copies) **plus** the return value appended as the last
parameter.

```c
// Kernel function:
// long do_sys_openat2(int dfd, struct filename *filename, struct open_how *how);
// Returns: file descriptor number, or negative errno

SEC("fexit/do_sys_openat2")
int BPF_PROG(fexit_openat2,
             int dfd,
             struct filename *filename,
             struct open_how *how,
             long ret)              // ← return value, ALWAYS last
{
    if (ret < 0) {
        bpf_printk("openat2 FAILED: err=%ld\n", ret);
    } else {
        bpf_printk("openat2 fd=%ld dfd=%d\n", ret, dfd);
    }
    return 0;
}
```

### Return Value Types

The return value's type comes from BTF — it's the declared return type of the
function:

```
Function returns:     BPF prog last arg type:
─────────────────     ───────────────────────
int                   int ret
long                  long ret
struct file *         struct file *ret   (pointer to returned struct)
void                  (no retval param)
__u64                 __u64 ret
```

### Struct Return Values

For functions returning pointers to structs:

```c
// Kernel: struct file *filp_open(const char *filename, int flags, umode_t mode)

SEC("fexit/filp_open")
int BPF_PROG(trace_filp_open,
             const char *filename,
             int flags,
             umode_t mode,
             struct file *ret)   // ret is the returned file pointer (may be ERR_PTR)
{
    if (IS_ERR_OR_NULL(ret)) {
        long err = PTR_ERR(ret);
        bpf_printk("filp_open failed: %ld\n", err);
        return 0;
    }
    // Can read fields from returned struct:
    unsigned int f_mode = BPF_CORE_READ(ret, f_mode);
    bpf_printk("file opened, f_mode=0x%x\n", f_mode);
    return 0;
}
```

---

## 9. fentry vs fexit — Differences and Use Cases

```
 Feature                    fentry                  fexit
 ──────────────────────     ──────────────────       ──────────────────────
 When it runs               Before function body     After function returns
 Arguments available        Yes                      Yes (saved by trampoline)
 Return value available     No                       Yes (last param)
 Can observe call latency   No (no end time)         Yes (pair with fentry)
 Can observe errors         Partially (pre-call)     Yes (check return value)
 Can observe I/O size       Partially                Yes (bytes read/written)
 Overhead                   Lower (no ret wrapping)  Slightly higher
 Use case                   "What was called?"       "What happened?"
```

### Latency Measurement Pattern

A common pattern pairs fentry + fexit through a map:

```c
// fentry: record start time
SEC("fentry/tcp_sendmsg")
int BPF_PROG(fentry_tcp_sendmsg, struct sock *sk, struct msghdr *msg, size_t size)
{
    __u64 ts = bpf_ktime_get_ns();
    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    bpf_map_update_elem(&start_times, &pid, &ts, BPF_ANY);
    return 0;
}

// fexit: compute and record latency
SEC("fexit/tcp_sendmsg")
int BPF_PROG(fexit_tcp_sendmsg,
             struct sock *sk, struct msghdr *msg, size_t size, int ret)
{
    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    __u64 *ts = bpf_map_lookup_elem(&start_times, &pid);
    if (!ts) return 0;
    __u64 latency_ns = bpf_ktime_get_ns() - *ts;
    bpf_map_delete_elem(&start_times, &pid);
    // record latency in histogram map
    ...
    return 0;
}
```

---

## 10. fentry vs kprobes

### Architectural Comparison

```
 kprobe mechanism:
 ─────────────────────────────────────────────────────────────────────────
                                              do_sys_openat2:
                                              0xCC (INT3)  ← kprobe installed
                                              ...

 Call path:
 CPU executes INT3
    → hardware exception
    → exception handler (do_int3 / notifier chain)
    → kprobe_int3_handler()
    → save ALL registers (pt_regs — 168 bytes on x86_64)
    → call pre_handler (your BPF program)
    → single-step original instruction
    → call post_handler
    → restore registers
    → resume

 Total: hardware exception + notifier chain traversal + full reg save/restore
 Latency: ~100-300 ns per hit

 ─────────────────────────────────────────────────────────────────────────

 fentry mechanism:
 ─────────────────────────────────────────────────────────────────────────
                                              do_sys_openat2:
                                              CALL <trampoline>  ← 5-byte CALL

 Call path:
 CPU executes CALL (normal function call — no exception)
    → trampoline runs
    → save 6 argument registers (48 bytes)
    → call JITed BPF program (direct call)
    → restore registers
    → return to function body

 Total: normal function call + partial reg save + direct prog call
 Latency: ~10-30 ns per hit

 ─────────────────────────────────────────────────────────────────────────
```

### Feature Comparison Table

```
 Feature                        kprobe/kretprobe    fentry/fexit
 ──────────────────────────     ────────────────    ────────────
 Overhead                       ~100-300 ns          ~10-30 ns
 Type information               None (raw regs)      Full BTF types
 Argument access                pt_regs cast         Typed direct access
 Inline functions               Yes (with care)      No (no BTF entry)
 Non-exported symbols           Yes (if no kpatch)   Yes (if in BTF)
 Interrupt context              Yes                  Yes (same rules)
 CONFIG_DYNAMIC_FTRACE needed   No                   Yes
 CONFIG_DEBUG_INFO_BTF needed   No                   Yes
 Kernel version                 2.6.9+               5.5+
 Modify return value            Yes (override)       No (read-only)
 SMP safe                       Yes                  Yes
 Recursion protection           Manual               Automatic (per-CPU)
 Struct access                  Manual cast+probe    CO-RE / direct
```

### When to Use kprobes Instead of fentry

Use kprobes when:
1. You need to trace functions that are **inlined** (inlined functions have no
   call site and therefore no mcount/fentry slot)
2. You need to trace on kernels < 5.5
3. You need to **modify return values** (`bpf_override_return`)
4. The target function does **not appear in BTF** (some arch-specific code)
5. You need to probe at an arbitrary instruction offset, not just function entry

---

## 11. fentry vs Tracepoints

### Philosophical Difference

```
 Tracepoints:                              fentry/fexit:
 ─────────────────────────────────         ─────────────────────────────────
 Designed by kernel developers            Any function with BTF
 Stable ABI — won't change                Follow function signature (can change)
 Placed at "interesting" moments          Function entry/exit only
 TRACE_EVENT macros in source             Automatic from BTF
 Available since 2.6.32                   Available since 5.5
 Some overhead even when disabled         Zero overhead when detached (NOP)
 Rich semantic context                    Raw kernel data

 Example:
 tracepoint/syscalls/sys_enter_openat     fentry/do_sys_openat2
   → struct { int dfd; char *filename;      → (int dfd, struct filename *filename,
             int flags; mode_t mode; }           struct open_how *how)
   → user-visible, sanitized args           → internal kernel representation
```

### Which to Use

| Scenario                               | Preferred Mechanism          |
|----------------------------------------|------------------------------|
| Production observability tool          | Tracepoints (stable ABI)     |
| Kernel debugging / development         | fentry/fexit                 |
| Network stack deep dive                | fentry/fexit (no tracepoints)|
| Cross-kernel-version tool              | Tracepoints + CO-RE fallback |
| Lowest possible overhead               | fentry/fexit                 |
| Standardized tooling (BCC, bpftrace)   | Tracepoints                  |

---

## 12. Verifier Rules for fentry/fexit Programs

The BPF verifier applies special rules to fentry/fexit programs.

### Argument Validation

```
check_attach_btf_id():
  1. Resolve function name → BTF_KIND_FUNC type
  2. Follow to BTF_KIND_FUNC_PROTO
  3. Extract (return_type, [param_types...])
  4. Compare with BPF prog's declared argument types
  5. REJECT if types mismatch

For fexit:
  6. The last argument in the BPF prog must match the return type of target func
```

### Context Access Restrictions

```
fentry/fexit ctx is special:
  - ctx[0]..ctx[N-1] = arguments (for N-arg function)
  - ctx[N]           = return value (fexit only)
  - ctx is read-only from BPF program's perspective
  - each ctx[i] access is rewritten to load from trampoline stack frame
```

### Allowed BPF Helpers

fentry/fexit programs can use **most** BPF helpers, including:
- All map operations (lookup, update, delete, push/pop)
- `bpf_get_current_pid_tgid()`, `bpf_get_current_comm()`
- `bpf_probe_read_kernel()` and family
- `bpf_ktime_get_ns()`
- `bpf_perf_event_output()`, `bpf_ringbuf_output()`
- `bpf_send_signal()` (for process signaling)
- `bpf_get_stackid()` / `bpf_get_stack()` (stack traces)

Non-sleepable fentry/fexit programs CANNOT:
- `bpf_copy_from_user()` (sleepable helper)
- `bpf_task_storage_get()` with GFP flags that may sleep

### Recursion Protection

The kernel prevents recursive fentry invocation. If a fentry BPF program calls
a helper that itself triggers the same fentry (e.g., map update calls a function
being traced), the recursion is detected and the nested invocation is skipped.
This is done with per-CPU counters in the trampoline.

---

## 13. C Implementation

### Complete Example: TCP Connection Tracer

This example traces all TCP connections and their durations.

**Kernel-side BPF program (tcp_trace.bpf.c):**

```c
// SPDX-License-Identifier: GPL-2.0
// Requires: kernel >= 5.5, CONFIG_DEBUG_INFO_BTF=y

#include "vmlinux.h"             // generated from kernel BTF by bpftool
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

// ─────────────────────────────────────────────────────────────────────────────
// Data structures shared between kernel BPF and user space
// ─────────────────────────────────────────────────────────────────────────────

struct conn_event {
    __u32 pid;
    __u32 tid;
    __u32 saddr;      // source IPv4
    __u32 daddr;      // dest IPv4
    __u16 sport;
    __u16 dport;
    __u64 ts_ns;      // timestamp nanoseconds
    __u8  event_type; // 0=connect, 1=accept, 2=close
    char  comm[16];
};

// ─────────────────────────────────────────────────────────────────────────────
// BPF Maps
// ─────────────────────────────────────────────────────────────────────────────

// Per-PID in-flight connection tracking
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, __u32);           // PID
    __type(value, __u64);         // start timestamp
} start_ts SEC(".maps");

// Ring buffer for events to userspace
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24); // 16 MB
} events SEC(".maps");

// ─────────────────────────────────────────────────────────────────────────────
// Helper: extract IPv4 from sock
// ─────────────────────────────────────────────────────────────────────────────

static __always_inline void fill_conn_event(struct conn_event *e,
                                             struct sock *sk,
                                             __u8 type)
{
    __u64 id = bpf_get_current_pid_tgid();
    e->pid   = id >> 32;
    e->tid   = (__u32)id;
    e->ts_ns = bpf_ktime_get_ns();
    e->event_type = type;
    bpf_get_current_comm(e->comm, sizeof(e->comm));

    // CO-RE: works across kernel versions even if struct layout changed
    e->saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    e->daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    e->sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    e->dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
}

// ─────────────────────────────────────────────────────────────────────────────
// fentry: tcp_connect — when a TCP connect() is initiated
// ─────────────────────────────────────────────────────────────────────────────

SEC("fentry/tcp_connect")
int BPF_PROG(trace_tcp_connect, struct sock *sk)
{
    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    __u64 ts  = bpf_ktime_get_ns();

    // Record start time for latency measurement
    bpf_map_update_elem(&start_ts, &pid, &ts, BPF_ANY);

    // Emit connect event
    struct conn_event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;

    fill_conn_event(e, sk, 0 /* connect */);
    bpf_ringbuf_submit(e, 0);
    return 0;
}

// ─────────────────────────────────────────────────────────────────────────────
// fexit: tcp_connect — capture result of the connect attempt
// ─────────────────────────────────────────────────────────────────────────────

SEC("fexit/tcp_connect")
int BPF_PROG(trace_tcp_connect_ret, struct sock *sk, int ret)
{
    // 'ret' is the return value of tcp_connect()
    // 0 = success (connection in progress), negative = error
    if (ret != 0) {
        __u32 pid = bpf_get_current_pid_tgid() >> 32;
        bpf_map_delete_elem(&start_ts, &pid);
        bpf_printk("tcp_connect failed for pid=%u err=%d\n",
                   bpf_get_current_pid_tgid() >> 32, ret);
    }
    return 0;
}

// ─────────────────────────────────────────────────────────────────────────────
// fentry: inet_csk_accept — new connection accepted (server side)
// ─────────────────────────────────────────────────────────────────────────────

SEC("fentry/inet_csk_accept")
int BPF_PROG(trace_inet_csk_accept,
             struct sock *sk,
             int flags,
             int *err,
             bool kern)
{
    // At fentry, we just record the attempt
    return 0;
}

// ─────────────────────────────────────────────────────────────────────────────
// fexit: inet_csk_accept — capture the newly accepted socket
// ─────────────────────────────────────────────────────────────────────────────

SEC("fexit/inet_csk_accept")
int BPF_PROG(trace_inet_csk_accept_ret,
             struct sock *sk,   // listening socket
             int flags,
             int *err,
             bool kern,
             struct sock *ret)  // returned: new connected socket (or NULL)
{
    if (!ret) return 0;  // accept failed

    // ret is the newly connected socket
    struct conn_event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;

    fill_conn_event(e, ret, 1 /* accept */);
    bpf_ringbuf_submit(e, 0);
    return 0;
}

// ─────────────────────────────────────────────────────────────────────────────
// fentry: tcp_close — connection being closed
// ─────────────────────────────────────────────────────────────────────────────

SEC("fentry/tcp_close")
int BPF_PROG(trace_tcp_close, struct sock *sk, long timeout)
{
    struct conn_event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;

    fill_conn_event(e, sk, 2 /* close */);
    bpf_ringbuf_submit(e, 0);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
```

**User-space loader (tcp_trace.c):**

```c
#include <stdio.h>
#include <stdlib.h>
#include <signal.h>
#include <arpa/inet.h>
#include <bpf/libbpf.h>
#include "tcp_trace.skel.h"    // generated by bpftool gen skeleton

static volatile bool running = true;
static void sig_handler(int sig) { running = false; }

// Event handler called by ring buffer polling
static int handle_event(void *ctx, void *data, size_t data_sz)
{
    const struct conn_event *e = data;
    char saddr[INET_ADDRSTRLEN], daddr[INET_ADDRSTRLEN];

    inet_ntop(AF_INET, &e->saddr, saddr, sizeof(saddr));
    inet_ntop(AF_INET, &e->daddr, daddr, sizeof(daddr));

    const char *types[] = { "CONNECT", "ACCEPT", "CLOSE" };
    printf("%-8s  pid=%-6u  comm=%-16s  %s:%-5u → %s:%-5u\n",
           types[e->event_type], e->pid, e->comm,
           saddr, e->sport, daddr, e->dport);
    return 0;
}

int main(void)
{
    struct tcp_trace_bpf *skel;
    struct ring_buffer    *rb;
    int err;

    // Open, load, verify the BPF skeleton
    skel = tcp_trace_bpf__open_and_load();
    if (!skel) {
        fprintf(stderr, "Failed to open/load skeleton\n");
        return 1;
    }

    // Attach all fentry/fexit programs (skeleton attach handles links)
    err = tcp_trace_bpf__attach(skel);
    if (err) {
        fprintf(stderr, "Failed to attach BPF programs: %d\n", err);
        goto cleanup;
    }

    // Set up ring buffer polling
    rb = ring_buffer__new(bpf_map__fd(skel->maps.events),
                          handle_event, NULL, NULL);
    if (!rb) {
        fprintf(stderr, "Failed to create ring buffer\n");
        goto cleanup;
    }

    signal(SIGINT, sig_handler);
    printf("Tracing TCP connections... Ctrl-C to stop.\n");
    printf("%-8s  %-8s  %-16s  %s\n",
           "TYPE", "PID", "COMM", "CONNECTION");

    while (running) {
        err = ring_buffer__poll(rb, 100 /* ms timeout */);
        if (err < 0 && err != -EINTR) {
            fprintf(stderr, "ring_buffer poll error: %d\n", err);
            break;
        }
    }

    ring_buffer__free(rb);
cleanup:
    tcp_trace_bpf__destroy(skel);
    return err < 0 ? 1 : 0;
}
```

**Build system (Makefile):**

```makefile
CLANG    ?= clang
BPFTOOL  ?= bpftool
ARCH     ?= $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')

# Generate vmlinux.h from running kernel's BTF
vmlinux.h:
	$(BPFTOOL) btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h

# Compile BPF program to BPF bytecode
tcp_trace.bpf.o: tcp_trace.bpf.c vmlinux.h
	$(CLANG) -g -O2 -target bpf -D__TARGET_ARCH_$(ARCH) \
	    -I. -c tcp_trace.bpf.c -o tcp_trace.bpf.o

# Generate skeleton header from BPF object
tcp_trace.skel.h: tcp_trace.bpf.o
	$(BPFTOOL) gen skeleton tcp_trace.bpf.o name tcp_trace_bpf \
	    > tcp_trace.skel.h

# Compile user-space loader
tcp_trace: tcp_trace.c tcp_trace.skel.h
	$(CC) -g -O2 tcp_trace.c -o tcp_trace -lbpf -lelf -lz
```

---

### Complete Example 2: System Call Error Rate Monitor

```c
// syscall_errors.bpf.c
// Tracks error rates per syscall using fexit on the syscall table dispatch

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

// Per-syscall error count
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 512);
    __type(key, __u32);    // syscall NR
    __type(value, __u64);  // error count
} error_counts SEC(".maps");

// Per-syscall total count
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 512);
    __type(key, __u32);
    __type(value, __u64);
} total_counts SEC(".maps");

// We attach to do_syscall_64 — the main syscall dispatch
// Signature: void do_syscall_64(struct pt_regs *regs, int nr)
// But note: fexit sees the regs state AFTER the syscall ran

SEC("fexit/do_syscall_64")
int BPF_PROG(trace_syscall_exit,
             struct pt_regs *regs,
             int nr,
             void *ret)  // do_syscall_64 is void, so we omit retval
{
    // Read the return value from regs->ax (set by syscall)
    long rax = BPF_CORE_READ(regs, ax);
    __u32 syscall_nr = (__u32)nr;
    __u64 one = 1;

    // Increment total count
    __u64 *total = bpf_map_lookup_elem(&total_counts, &syscall_nr);
    if (total) {
        __sync_fetch_and_add(total, 1);
    } else {
        bpf_map_update_elem(&total_counts, &syscall_nr, &one, BPF_NOEXIST);
    }

    // If return value is a negative errno, count as error
    if (rax < 0 && rax > -4096) {  // Linux errno range
        __u64 *errs = bpf_map_lookup_elem(&error_counts, &syscall_nr);
        if (errs) {
            __sync_fetch_and_add(errs, 1);
        } else {
            bpf_map_update_elem(&error_counts, &syscall_nr, &one, BPF_NOEXIST);
        }
    }

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
```

---

### Complete Example 3: File I/O Latency Histogram

```c
// io_latency.bpf.c
// Measures vfs_read and vfs_write latency with a BPF histogram map

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

// Histogram: latency slots in powers of 2 (nanoseconds)
#define HIST_SLOTS 64

struct hist {
    __u64 slots[HIST_SLOTS];
};

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 2);    // [0] = read hist, [1] = write hist
    __type(key, __u32);
    __type(value, struct hist);
} latency_hist SEC(".maps");

// Per-task in-flight map (keyed by task_struct pointer for correctness)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 8192);
    __type(key, __u64);       // (pid << 1) | op_type
    __type(value, __u64);     // start timestamp
} in_flight SEC(".maps");

static __always_inline void record_start(__u64 key)
{
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&in_flight, &key, &ts, BPF_ANY);
}

static __always_inline void record_end(__u64 key, __u32 hist_idx)
{
    __u64 *start = bpf_map_lookup_elem(&in_flight, &key);
    if (!start) return;

    __u64 delta = bpf_ktime_get_ns() - *start;
    bpf_map_delete_elem(&in_flight, &key);

    // Compute log2 slot
    int slot = 0;
    __u64 tmp = delta;
    while (tmp > 1 && slot < HIST_SLOTS - 1) {
        tmp >>= 1;
        slot++;
    }

    struct hist *h = bpf_map_lookup_elem(&latency_hist, &hist_idx);
    if (h) {
        __sync_fetch_and_add(&h->slots[slot], 1);
    }
}

// vfs_read(struct file *file, char __user *buf, size_t count, loff_t *pos)
SEC("fentry/vfs_read")
int BPF_PROG(fentry_vfs_read,
             struct file *file, char *buf, size_t count, loff_t *pos)
{
    __u64 key = ((__u64)(bpf_get_current_pid_tgid() >> 32) << 1) | 0;
    record_start(key);
    return 0;
}

SEC("fexit/vfs_read")
int BPF_PROG(fexit_vfs_read,
             struct file *file, char *buf, size_t count, loff_t *pos,
             ssize_t ret)
{
    if (ret < 0) return 0;  // skip errors
    __u64 key = ((__u64)(bpf_get_current_pid_tgid() >> 32) << 1) | 0;
    __u32 hist_idx = 0;
    record_end(key, hist_idx);
    return 0;
}

// vfs_write(struct file *file, const char __user *buf, size_t count, loff_t *pos)
SEC("fentry/vfs_write")
int BPF_PROG(fentry_vfs_write,
             struct file *file, const char *buf, size_t count, loff_t *pos)
{
    __u64 key = ((__u64)(bpf_get_current_pid_tgid() >> 32) << 1) | 1;
    record_start(key);
    return 0;
}

SEC("fexit/vfs_write")
int BPF_PROG(fexit_vfs_write,
             struct file *file, const char *buf, size_t count, loff_t *pos,
             ssize_t ret)
{
    if (ret < 0) return 0;
    __u64 key = ((__u64)(bpf_get_current_pid_tgid() >> 32) << 1) | 1;
    __u32 hist_idx = 1;
    record_end(key, hist_idx);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
```

---

## 14. Rust Implementation

### Using Aya (Pure Rust BPF Framework)

[Aya](https://aya-rs.dev) is a pure Rust eBPF library that supports fentry/fexit.
It does not depend on libbpf or the C toolchain for the BPF program compilation
(it uses `rustc` targeting `bpf` directly).

**Project structure:**

```
tcp-tracer/
├── Cargo.toml
├── tcp-tracer/              ← user-space Rust binary
│   ├── Cargo.toml
│   └── src/
│       └── main.rs
└── tcp-tracer-ebpf/         ← BPF program (compiled to BPF bytecode by rustc)
    ├── Cargo.toml
    └── src/
        └── main.rs
```

**Workspace Cargo.toml:**

```toml
[workspace]
members = ["tcp-tracer", "tcp-tracer-ebpf"]
resolver = "2"

[profile.dev]
opt-level = 3

[profile.release]
lto = true
```

**BPF program (tcp-tracer-ebpf/src/main.rs):**

```rust
#![no_std]
#![no_main]

// This compiles to BPF bytecode, not native.
// Target: bpf-unknown-none (or bpfel-unknown-none for little-endian)

use aya_bpf::{
    macros::{fentry, fexit, map},
    maps::{HashMap, RingBuf},
    programs::{FEntryContext, FExitContext},
    BpfContext,
};
use aya_log_ebpf::info;

// Shared types (also defined in common crate)
#[repr(C)]
pub struct ConnEvent {
    pub pid:        u32,
    pub saddr:      u32,
    pub daddr:      u32,
    pub sport:      u16,
    pub dport:      u16,
    pub event_type: u8,
    pub retval:     i32,
}

// BPF Maps
#[map(name = "start_times")]
static mut START_TIMES: HashMap<u32, u64> = HashMap::with_max_entries(4096, 0);

#[map(name = "events")]
static mut EVENTS: RingBuf = RingBuf::with_byte_size(1 << 24, 0);

// ─────────────────────────────────────────────────────────────────────────────
// fentry/tcp_connect
// ─────────────────────────────────────────────────────────────────────────────

#[fentry(function = "tcp_connect")]
pub fn tcp_connect_entry(ctx: FEntryContext) -> u32 {
    match unsafe { try_tcp_connect_entry(&ctx) } {
        Ok(ret)  => ret,
        Err(_)   => 0,
    }
}

unsafe fn try_tcp_connect_entry(ctx: &FEntryContext) -> Result<u32, i64> {
    // Read arg0: struct sock *sk
    // In Aya, FEntryContext::arg(n) reads the nth argument
    // The type must match the kernel function's BTF signature
    let _sk: *const core::ffi::c_void = ctx.arg(0);

    let pid = ctx.pid();
    let ts  = aya_bpf::helpers::bpf_ktime_get_ns();

    START_TIMES.insert(&pid, &ts, 0)?;

    info!(ctx, "tcp_connect: pid={}", pid);
    Ok(0)
}

// ─────────────────────────────────────────────────────────────────────────────
// fexit/tcp_connect — captures return value
// ─────────────────────────────────────────────────────────────────────────────

#[fexit(function = "tcp_connect")]
pub fn tcp_connect_exit(ctx: FExitContext) -> u32 {
    match unsafe { try_tcp_connect_exit(&ctx) } {
        Ok(ret)  => ret,
        Err(_)   => 0,
    }
}

unsafe fn try_tcp_connect_exit(ctx: &FExitContext) -> Result<u32, i64> {
    // For fexit, the return value is the LAST argument in FExitContext
    let ret: i32 = ctx.arg(1);  // arg(0)=sk, arg(1)=retval for tcp_connect
    let pid = ctx.pid();

    if ret != 0 {
        // Connect failed
        if let Some(start_ts) = START_TIMES.get(&pid) {
            let latency = aya_bpf::helpers::bpf_ktime_get_ns() - *start_ts;
            info!(ctx, "tcp_connect FAILED pid={} err={} lat={}ns",
                  pid, ret, latency);
        }
        START_TIMES.remove(&pid).ok();
    }

    Ok(0)
}

// ─────────────────────────────────────────────────────────────────────────────
// fexit/inet_csk_accept — new incoming connection
// ─────────────────────────────────────────────────────────────────────────────

#[fexit(function = "inet_csk_accept")]
pub fn inet_csk_accept_exit(ctx: FExitContext) -> u32 {
    match unsafe { try_accept_exit(&ctx) } {
        Ok(ret)  => ret,
        Err(_)   => 0,
    }
}

unsafe fn try_accept_exit(ctx: &FExitContext) -> Result<u32, i64> {
    // inet_csk_accept(struct sock *sk, int flags, int *err, bool kern) -> struct sock *
    // retval = arg(4) in fexit context
    let new_sk: *const core::ffi::c_void = ctx.arg(4);
    if new_sk.is_null() {
        return Ok(0);
    }

    let pid = ctx.pid();
    info!(ctx, "inet_csk_accept: new connection, pid={}", pid);
    Ok(0)
}

// Required: panic handler for no_std
#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    loop {}
}
```

**User-space loader (tcp-tracer/src/main.rs):**

```rust
use aya::{
    include_bytes_aligned,
    programs::{FEntry, FExit},
    Bpf,
};
use aya_log::BpfLogger;
use log::{info, warn};
use tokio::signal;

#[tokio::main]
async fn main() -> Result<(), anyhow::Error> {
    env_logger::init();

    // Load the compiled BPF object (embedded at compile time)
    // aya-build embeds the BPF ELF during build.rs
    #[cfg(debug_assertions)]
    let mut bpf = Bpf::load(include_bytes_aligned!(
        "../../target/bpf/programs/tcp-tracer/tcp_tracer.bpf.o"
    ))?;
    #[cfg(not(debug_assertions))]
    let mut bpf = Bpf::load(include_bytes_aligned!(
        "../../target/bpf/programs/tcp-tracer-release/tcp_tracer.bpf.o"
    ))?;

    // Set up BPF logger (reads from aya_log ring buffer)
    if let Err(e) = BpfLogger::init(&mut bpf) {
        warn!("Failed to init BPF logger: {}", e);
    }

    // Attach fentry/tcp_connect
    let program: &mut FEntry = bpf
        .program_mut("tcp_connect_entry")
        .unwrap()
        .try_into()?;
    program.load("tcp_connect", &btf)?;  // "tcp_connect" = kernel func name
    program.attach()?;

    // Attach fexit/tcp_connect
    let program: &mut FExit = bpf
        .program_mut("tcp_connect_exit")
        .unwrap()
        .try_into()?;
    program.load("tcp_connect", &btf)?;
    program.attach()?;

    // Attach fexit/inet_csk_accept
    let program: &mut FExit = bpf
        .program_mut("inet_csk_accept_exit")
        .unwrap()
        .try_into()?;
    program.load("inet_csk_accept", &btf)?;
    program.attach()?;

    info!("TCP tracer running. Press Ctrl-C to stop.");

    // Wait for Ctrl-C
    signal::ctrl_c().await?;
    info!("Exiting.");
    Ok(())
}
```

**Build support (tcp-tracer-ebpf/Cargo.toml):**

```toml
[package]
name    = "tcp-tracer-ebpf"
version = "0.1.0"
edition = "2021"

[[bin]]
name              = "tcp-tracer-ebpf"
path              = "src/main.rs"
# BPF target — compiled by rustc to BPF bytecode
# Must be in .cargo/config.toml: [build] target = "bpfel-unknown-none"

[dependencies]
aya-bpf = { version = "0.1", features = ["fentry", "fexit"] }
aya-log-ebpf = "0.1"

[profile.release]
opt-level = 3
lto       = true
```

**Build process (.cargo/config.toml for the ebpf crate):**

```toml
[build]
target = "bpfel-unknown-none"  # BPF little-endian (most architectures)

[unstable]
build-std = ["core"]  # only core, no std for no_std BPF programs
```

### Using libbpf-rs (Rust bindings to libbpf)

An alternative to Aya is `libbpf-rs`, which wraps the C libbpf library. It is
useful when you want Rust in user space but still use C for BPF programs:

```rust
// user-space only, BPF program is written in C
use libbpf_rs::{ObjectBuilder, RingBufferBuilder};
use std::time::Duration;

fn main() -> anyhow::Result<()> {
    // Load the BPF object compiled from C
    let obj_builder = ObjectBuilder::default();
    let open_obj = obj_builder.open_file("tcp_trace.bpf.o")?;
    let mut obj = open_obj.load()?;

    // fentry/fexit programs are attached by BPF_LINK_CREATE internally
    // libbpf-rs exposes this via prog.attach_trace()
    let fentry_prog = obj
        .prog_mut("trace_tcp_connect")
        .ok_or_else(|| anyhow::anyhow!("prog not found"))?;
    let _link = fentry_prog.attach_trace()?;

    let fexit_prog = obj
        .prog_mut("trace_tcp_connect_ret")
        .ok_or_else(|| anyhow::anyhow!("prog not found"))?;
    let _link2 = fexit_prog.attach_trace()?;

    // Set up ring buffer
    let mut rb_builder = RingBufferBuilder::new();
    rb_builder.add(obj.map("events").unwrap(), handle_event)?;
    let rb = rb_builder.build()?;

    loop {
        rb.poll(Duration::from_millis(100))?;
    }
}

fn handle_event(_data: &[u8]) -> i32 {
    // parse and print event
    0
}
```

---

## 15. Multi-attach and Sharing Trampolines

### Multiple Programs on One Function

You can attach multiple fentry **and** multiple fexit programs to the same
kernel function. The kernel uses one trampoline per target function and chains
all attached programs through it.

```
 Kernel function: tcp_sendmsg
         │
         ▼ (call __fentry__ → trampoline)
 ┌─────────────────────────────────────────────────────┐
 │  BPF TRAMPOLINE for tcp_sendmsg                     │
 │                                                      │
 │  FENTRY section:                                     │
 │    call bpf_prog_A_fentry   ← latency monitor       │
 │    call bpf_prog_B_fentry   ← security audit         │
 │    call bpf_prog_C_fentry   ← debug logger           │
 │    ── continue to actual tcp_sendmsg body ──         │
 │                                                      │
 │  FEXIT section (on return path):                    │
 │    call bpf_prog_X_fexit    ← error counter          │
 │    call bpf_prog_Y_fexit    ← latency recorder       │
 │                                                      │
 │  Programs are called in attachment order.            │
 │  If one returns non-zero, subsequent progs still run.│
 └─────────────────────────────────────────────────────┘
```

### Trampoline Regeneration

When you add or remove a program from a trampoline, the kernel:
1. Allocates a new trampoline code buffer
2. Compiles the new sequence into it
3. Atomically swaps the function's call target to the new trampoline
4. Frees the old trampoline (after RCU grace period)

This means adding/removing observers is atomic from the kernel's perspective.

### Maximum Programs per Trampoline

The current kernel limit is 40 BPF programs total per trampoline
(`BPF_MAX_TRAMP_PROGS`). This covers the sum of fentry + fexit programs on
the same function. This limit may change in future kernels.

---

## 16. Sleepable fentry/fexit

### What "Sleepable" Means

Normal BPF programs run with preemption disabled. Sleepable programs (marked
with `BPF_F_SLEEPABLE`) may block, sleep, or use helpers that sleep (like
`bpf_copy_from_user` which may fault in user pages).

### When to Use Sleepable fentry

- Copying data from user space pointers passed to syscalls
- Using `bpf_task_storage_get()` with allocation flags
- Using `bpf_inode_storage_get()` or `bpf_sk_storage_get()` with alloc flags

### Constraints on Sleepable Programs

Sleepable fentry/fexit programs:
1. Cannot be attached to functions that run in interrupt context
2. Cannot be attached to functions that hold spinlocks
3. The kernel verifier checks that the target function is "sleepable" (i.e.,
   the function is never called from non-sleepable context)
4. Must use only sleepable-safe helpers

### C Example: Sleepable fentry Copying User Data

```c
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

struct event {
    __u32 pid;
    char  filename[256];
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 20);
} events SEC(".maps");

// Mark the program as sleepable
// This is required for bpf_copy_from_user()
SEC("fentry.s/do_sys_openat2")
//          ^ the .s suffix marks it as sleepable in libbpf >= 0.8
int BPF_PROG(sleepable_openat2,
             int dfd,
             struct filename *filename,
             struct open_how *how)
{
    struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;

    e->pid = bpf_get_current_pid_tgid() >> 32;

    // filename->name is a kernel pointer to a kernel-side string
    // We can read it with bpf_probe_read_kernel_str
    const char *kname = BPF_CORE_READ(filename, name);
    bpf_probe_read_kernel_str(e->filename, sizeof(e->filename), kname);

    bpf_ringbuf_submit(e, 0);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
```

The kernel enforces sleepability via `check_sleepable_lsm_hooks()` and
`bpf_prog_sleepable()`. Not all functions are valid attachment targets for
sleepable programs.

---

## 17. Struct-ops and fentry Interaction

### What struct-ops Is

`BPF_PROG_TYPE_STRUCT_OPS` lets you replace kernel function pointers (like
the functions in `struct tcp_congestion_ops`) with BPF programs. This is
different from fentry/fexit but relates to the same trampoline infrastructure.

### Interaction

fentry/fexit programs can attach to **BPF struct-ops programs** — i.e., you can
observe calls to BPF-implemented kernel operations:

```
  TCP stack calls tcp_cong_ops->ssthresh()
         │
         ▼
  BPF struct-ops program (your custom TCP CC)
         │
         ├──► fentry BPF program can observe this call
         │
         ▼
  returns value back to TCP stack
         │
         └──► fexit BPF program can observe the return
```

This allows layering: you can instrument your own BPF programs with other BPF
programs, enabling meta-observability.

---

## 18. Tail Calls and fentry

### Tail Calls Within fentry/fexit Programs

fentry/fexit programs can perform tail calls to other BPF programs in a
`BPF_MAP_TYPE_PROG_ARRAY`. The tail call replaces the current program's stack
frame (in BPF terms), allowing longer processing chains without hitting the
4096 instruction limit of a single program.

```c
struct {
    __uint(type, BPF_MAP_TYPE_PROG_ARRAY);
    __uint(max_entries, 8);
    __type(key, __u32);
    __type(value, __u32);
} tail_call_map SEC(".maps");

SEC("fentry/tcp_sendmsg")
int BPF_PROG(fentry_tcp_sendmsg, struct sock *sk, struct msghdr *msg, size_t size)
{
    // do quick filtering first
    __u16 family = BPF_CORE_READ(sk, __sk_common.skc_family);
    if (family == AF_INET6) {
        // tail call to IPv6-specific handler
        bpf_tail_call(ctx, &tail_call_map, 1);
        // falls through here if tail call fails (map slot empty, limit reached)
    }
    // IPv4 path
    return 0;
}
```

### Tail Call Limit

BPF limits tail call chains to 33 hops (MAX_TAIL_CALL_CNT = 33). This prevents
infinite loops that the verifier cannot statically detect.

---

## 19. Real-World Use Cases

### 1. Security Enforcement (LSM-like behavior)

```
Goal: detect and alert on processes opening /etc/shadow

SEC("fentry/do_sys_openat2")
  → read filename, match against "/etc/shadow"
  → send alert via ringbuf or bpf_send_signal()
```

### 2. Network Performance Observability

```
Goal: measure TCP RTT distribution per destination IP

SEC("fentry/tcp_rcv_established")   ← packet received (ACK for sent data)
  → record per-connection timestamp in hash map keyed by skb

SEC("fexit/tcp_transmit_skb")       ← packet sent
  → calculate RTT from timestamp
  → update histogram map
```

### 3. Memory Leak Detection

```
Goal: track kmalloc/kfree pairing to detect leaks

SEC("fexit/kmalloc")
  → record {ptr=retval, size, stack_id} in hash map

SEC("fentry/kfree")
  → remove {ptr=arg0} from hash map
  → on unload, remaining entries = leaks
```

### 4. Scheduler Latency Analysis

```
Goal: measure wakeup-to-run latency for processes

SEC("fentry/wake_up_new_task") or SEC("fentry/try_to_wake_up")
  → record wakeup timestamp per PID

SEC("fentry/finish_task_switch")
  → calculate latency from wakeup to actually running
  → histogram per priority class
```

### 5. Storage I/O Amplification

```
Goal: detect when 1 userspace write causes many block I/O ops

SEC("fentry/vfs_write")        → count user writes per process
SEC("fentry/submit_bio")       → count block I/O submissions

Compare ratio: block_ios / user_writes to detect amplification
```

### 6. Cryptography Audit

```
Goal: log all uses of kernel crypto primitives

SEC("fentry/crypto_shash_update")
  → log {pid, comm, algo_name, data_len}

SEC("fentry/crypto_aead_encrypt")
  → log {pid, comm, keylen, datalen}
```

---

## 20. Debugging and Introspection

### Listing Attached fentry/fexit Programs

```bash
# List all BPF programs
bpftool prog list

# Filter for TRACING type (fentry/fexit/tracing)
bpftool prog list | grep TRACING

# Detailed info for a specific prog
bpftool prog show id <ID> -j | jq .

# Show the JIT-compiled instructions
bpftool prog dump jited id <ID>

# Show the BPF bytecode
bpftool prog dump xlated id <ID>
```

### Listing Active Links

```bash
# Links are the attachment objects
bpftool link list

# Detailed link info including target BTF ID
bpftool link show id <LINK_ID> -j
```

### Finding BTF IDs

```bash
# Find BTF ID for a function by name
bpftool btf dump id 0 | grep -A2 "tcp_connect"

# Or use bpftool prog to find by attach_btf_id
bpftool prog show -j | jq '.[].attach_btf_id'

# List all functions available for fentry/fexit
bpftool btf dump file /sys/kernel/btf/vmlinux format raw | \
    grep BTF_KIND_FUNC | awk '{print $3}'
```

### Verifying Trampoline Activation

```bash
# Check ftrace function list (shows which functions have active tracers)
cat /sys/kernel/debug/tracing/enabled_functions | grep tcp_connect

# Check trampoline usage via BPF stats
cat /proc/sys/kernel/bpf_stats_enabled   # enable stats
bpftool prog show --stats               # show run counts, durations
```

### Checking with /proc/kallsyms

```bash
# Verify your target function is exported
grep "do_sys_openat2\|tcp_connect" /proc/kallsyms

# Check if the function has the ftrace NOP slot
objdump -d /proc/kcore 2>/dev/null | grep -A5 "do_sys_openat2"
# Or use perf probe:
perf probe --list 2>/dev/null | grep openat
```

### bpf_printk for Debugging

```c
// In BPF programs, bpf_printk() writes to the trace pipe
bpf_printk("fentry hit: pid=%u arg0=%lu\n",
            bpf_get_current_pid_tgid() >> 32, (unsigned long)arg0);

// Read output:
// cat /sys/kernel/debug/tracing/trace_pipe
// or
// cat /sys/kernel/debug/tracing/trace
```

---

## 21. Kernel Version Compatibility Matrix

```
 Feature                                  Min kernel   Config required
 ─────────────────────────────────────    ──────────   ───────────────────────────────
 fentry/fexit basic                       5.5          CONFIG_DEBUG_INFO_BTF=y
                                                       CONFIG_DYNAMIC_FTRACE=y
                                                       CONFIG_FUNCTION_TRACER=y
 fexit with return value                  5.5          same as above
 BPF_LINK_CREATE for fentry              5.7          same as above
 Sleepable fentry/fexit (.s suffix)       5.10         + CONFIG_BPF_LSM=y (sometimes)
 Module fentry (kmod BTF)                5.11         + CONFIG_MODULE_ALLOW_BTF_MISMATCH
 Multi-prog per trampoline (40 limit)     5.12         same
 CO-RE in fentry programs                5.5+         + pahole >= 1.16 for gen BTF
 fentry on arm64                         5.5          CONFIG_ARM64_MODULE_PLTS=y
                                                       (arm64 trampoline support)
 fentry on s390x                         5.7          CONFIG_DYNAMIC_FTRACE=y
 fentry on riscv                         5.13         CONFIG_DYNAMIC_FTRACE=y
 bpf_get_func_ip() in fentry             5.15         same
 fentry on arm (32-bit)                  not supported —
 fentry on mips                          not supported —
```

### Checking Kernel Support at Runtime

```c
// libbpf provides a feature probe:
bool fentry_supported = libbpf_probe_bpf_prog_type(
    BPF_PROG_TYPE_TRACING, NULL);

// Or check from shell:
// bpftool feature probe prog_type BPF_PROG_TYPE_TRACING
```

---

## 22. Gotchas, Pitfalls, Limitations

### 1. Inlined Functions Are Invisible

If the kernel compiles a function as `inline` or the compiler decides to inline
it (even without the `inline` keyword), there is no call site and no mcount/fentry
slot. The function will not appear in BTF as a FUNC kind (it may appear as a
FUNC_PROTO if it's used as a function pointer, but not as an attachable function).

```c
// This CANNOT be traced with fentry:
static __always_inline int some_helper(int x) { return x + 1; }

// This CAN be traced (has its own symbol + BTF FUNC):
noinline int some_helper(int x) { return x + 1; }
```

**Mitigation**: Use kprobes for inlined functions, or add `noinline` in kernel
source if you're doing kernel development.

### 2. Arguments Are Saved Before the Function Body

If a function modifies its arguments (e.g., pointer dereferences into arguments),
the saved argument values in fexit reflect the **original** call-time values,
not what the function may have written through pointer arguments.

```c
// Kernel function:
// ssize_t vfs_read(struct file *file, char __user *buf,
//                   size_t count, loff_t *pos)
// The function MODIFIES *pos (updates file position)

// In fexit BPF:
SEC("fexit/vfs_read")
int BPF_PROG(fexit_vfs_read,
             struct file *file, char __user *buf, size_t count,
             loff_t *pos,   // ← this is the pointer, NOT the value
             ssize_t ret)
{
    // To get the updated position, you must dereference:
    loff_t new_pos;
    bpf_probe_read_kernel(&new_pos, sizeof(new_pos), pos);
    // new_pos now has the updated position
}
```

### 3. Functions Called in NMI/IRQ Context

If the target function can be called in NMI or IRQ context, your BPF program
must be safe in that context. The verifier does not always enforce this.

**Rules**:
- No spinning on locks
- No sleeping (obviously)
- Be careful with per-CPU maps (they are safe)
- Be careful with hash maps (they use locks internally, but use trylock in IRQ)

### 4. Variable Argument Functions (Variadics)

fentry/fexit does **not** support variadic functions (functions declared with
`...`). These cannot be described completely in BTF, so the verifier rejects
attachment attempts.

### 5. Static / Non-exported Symbols

A function does not need to be exported (in `/proc/kallsyms` with a non-static
address) to be traced — it only needs to appear in BTF. However, functions
declared `static` in C may be inlined and thus not traceable.

### 6. Trampoline Memory

Trampolines are allocated from a special memory area. If you attach many fentry
programs to many functions simultaneously, you may consume significant kernel
memory. Each trampoline is typically 256–512 bytes of executable memory.

### 7. Argument Count Limit

BPF programs can access up to 6 arguments (matching x86-64 SysV ABI register
count). Functions with more than 6 arguments pass the remainder on the stack.
The kernel supports tracing stack arguments, but this is architecture-specific
and limited to what the trampoline saves.

### 8. BPF Program Return Value

fentry/fexit programs must return 0. Non-zero returns are ignored (unlike XDP
or TC programs where the return value has meaning). The program is an observer,
not a controller.

### 9. PREEMPT_RT (Real-Time) Kernels

On `PREEMPT_RT` kernels, some internal locking behaviors change. The trampoline
itself is safe, but your BPF program's interaction with maps may behave
differently. Sleepable programs are generally safer on RT kernels.

### 10. Missing BTF in Stripped Kernels

Some distributions ship kernels without BTF (`CONFIG_DEBUG_INFO_BTF=n`). In this
case, fentry/fexit cannot work at all. You can check:

```bash
ls /sys/kernel/btf/vmlinux   # must exist
# OR
zcat /proc/config.gz | grep CONFIG_DEBUG_INFO_BTF
```

---

## 23. Mental Models

### Mental Model 1: The Observer in the Call Graph

Think of your kernel's function call graph as a massive tree of function
invocations. fentry/fexit lets you place "watchers" at any node in that tree,
for both the "entering" moment and the "exiting" moment. The watchers can read
the state passing through but cannot affect it.

```
                            kernel call graph
                            ─────────────────
syscall_entry
    │
    ├── security_check()              [fentry watcher A]
    │       └── lsm_hook()
    │
    ├── vfs_open()                    [fentry watcher B] [fexit watcher B']
    │       ├── path_lookup()
    │       │       └── dentry_open()
    │       └── alloc_file()
    │
    └── fd_install()
```

### Mental Model 2: The Decorator Pattern

fentry/fexit is the kernel equivalent of the decorator/wrapper pattern in
software engineering:

```python
# Software engineering:
@my_decorator     # = fentry (before) + fexit (after)
def original_function(a, b, c):
    return result
```

```
# Kernel equivalent:
[fentry BPF program]   # runs before
original_kernel_function(a, b, c)
[fexit BPF program]    # runs after, sees a,b,c AND result
```

### Mental Model 3: The Toll Booth

The trampoline is like a toll booth on a highway (the function call path):
- Every vehicle (function call) must pass through
- The toll booth (trampoline) checks and records information
- It does not stop or redirect vehicles, just observes
- The overhead is the time spent in the booth (nanoseconds)
- If the booth is removed (program detached), the highway is unobstructed (NOP)

### Mental Model 4: Function as a Black Box with Probes

```
                    ┌─────────────────────────────────┐
                    │                                 │
fentry probe ──►    │   do_sys_openat2(dfd, name, how)│    ──► fexit probe
sees: dfd,name,how  │                                 │         sees: dfd,name,how,ret
                    │   (internal implementation)     │
                    │   - path resolution              │
                    │   - permission checks            │
                    │   - file object allocation       │
                    │                                 │
                    └─────────────────────────────────┘

You see: what goes in (fentry) and what comes out (fexit).
You do NOT see: what happens inside (for that, attach to inner functions).
```

### Mental Model 5: The Cost Model

```
Operation                         CPU Cycles    Notes
──────────────────────────────    ──────────    ──────────────────────────────
NOP (no program attached)         ~1            5-byte NOP, nearly free
fentry attachment overhead        ~30-80        call + save regs + BPF prog
fexit attachment overhead         ~60-120       fentry + ret addr hijack + restore
kprobe equivalent                 ~300-800      exception + full pt_regs save
tracepoint equivalent             ~50-150       static call + arg packaging
uprobe                            ~1000+        user/kernel boundary crossing

The decision: if you need <50ns overhead, fentry is the only BPF option.
```

### Mental Model 6: Lifetime and Ownership

```
fentry/fexit program lifetime:

bpf_prog_load()      → program object created, refcount=1
bpf_link_create()    → link object created, trampoline activated
                       program refcount=2, link holds a reference

Program runs         → may be called millions of times per second

close(link_fd)       → link object destroyed
OR process dies      → link cleaned up by kernel
                     → trampoline deactivated (NOP restored)
                     → program refcount goes to 1

close(prog_fd)       → program refcount goes to 0 → freed

If link is pinned to bpffs:
  /sys/fs/bpf/my_link  → link persists even if creating process dies
  unlink()             → link destroyed, trampoline deactivated
```

---

## 24. Glossary

| Term | Definition |
|------|-----------|
| **fentry** | BPF program that runs at the entry point of a kernel function, before the function body executes |
| **fexit** | BPF program that runs at the exit point of a kernel function, after the function body returns, with access to the return value |
| **BTF** | BPF Type Format — compact type system embedded in the kernel, enables type-safe argument access |
| **Trampoline** | JIT-generated machine code stub that calls BPF programs and saves/restores function arguments |
| **mcount/fentry NOP** | Compiler-inserted 5-byte NOP at the start of every kernel function, patchable to a CALL for ftrace/BPF use |
| **text_poke_bp** | Kernel primitive for safely patching kernel text on a running SMP system using INT3-based synchronization |
| **CO-RE** | Compile Once, Run Everywhere — mechanism by which BPF programs adapt struct field offsets to the running kernel at load time |
| **vmlinux.h** | Auto-generated header file containing all kernel types from BTF, used in BPF programs |
| **BPF_PROG macro** | C macro that expands a typed argument list for fentry/fexit programs into the correct context access pattern |
| **BPF_CORE_READ** | Macro for type-aware CO-RE field access that handles struct layout differences between kernel versions |
| **BPF Link** | Reference-counted kernel object representing an attachment; cleaned up on fd close or process exit |
| **Skeleton** | Auto-generated C/Rust glue code (by bpftool gen skeleton) that wraps a BPF object for easy loading and attaching |
| **Sleepable** | BPF program flag (BPF_F_SLEEPABLE) indicating the program may block; enables helpers like bpf_copy_from_user |
| **DYNAMIC_FTRACE** | Kernel config option that enables runtime patching of mcount NOPs — required for fentry/fexit |
| **BPF_PROG_TYPE_TRACING** | The BPF program type used for fentry, fexit, raw_tracepoint, and iter programs |
| **attach_btf_id** | The BTF type ID of the target function, used by the verifier and linker to locate the function |
| **SysV ABI** | The calling convention used on x86-64 Linux: args in rdi,rsi,rdx,rcx,r8,r9, return in rax |
| **Ring buffer** | BPF_MAP_TYPE_RINGBUF — efficient one-producer, one-consumer buffer for sending events to user space |
| **Aya** | Pure Rust eBPF library supporting fentry/fexit, compiles BPF programs with rustc instead of clang |
| **libbpf** | The canonical C library for loading and attaching BPF programs; handles BTF lookup, CO-RE relocation |
| **BPF verifier** | Kernel component that statically analyzes BPF programs before loading to prove safety |
| **JIT compiler** | Kernel component that translates BPF bytecode to native machine instructions (x86-64, arm64, etc.) |
| **pahole** | Tool that generates BTF from DWARF; used by the kernel build system to embed BTF into vmlinux |

---

## Summary: The Complete Mental Stack

```
 LAYER 7: Your Use Case
   "I want to measure TCP connect latency"
         │
 LAYER 6: Program Pattern
   fentry/tcp_connect + fexit/tcp_connect + hash map for timing
         │
 LAYER 5: BPF Program (C or Rust)
   BPF_PROG(fentry_fn, struct sock *sk) { ... }
   BPF_PROG(fexit_fn, struct sock *sk, int ret) { ... }
         │
 LAYER 4: BTF Verification
   Verifier confirms: "yes, tcp_connect takes (struct sock *) and returns int"
   CO-RE records relocations for any struct field accesses
         │
 LAYER 3: JIT Compilation
   BPF bytecode → native x86-64 instructions
   Stored in kernel memory as executable code
         │
 LAYER 2: Trampoline Infrastructure
   arch_prepare_bpf_trampoline() generates entry/exit stubs
   register_ftrace_function() patches the function's NOP → CALL trampoline
         │
 LAYER 1: Hardware / CPU
   5-byte CALL instruction at start of tcp_connect
   CPU executes CALL → trampoline → BPF prog → returns to function
   ~30ns total overhead
         │
 LAYER 0: Silicon
   No exceptions, no interrupts — just a function call
```

---

*This document covers the fentry/fexit BPF subsystem as of Linux 6.x. The
kernel is always evolving; check kernel/bpf/trampoline.c and
arch/x86/net/bpf_jit_comp.c for the latest implementation details.*
