# Stack Unwinding: A Comprehensive In-Depth Guide
## C, C++, Rust, Go, Python, Java — Architecture, Internals, and Production Engineering

---

## Table of Contents

1. [Foundations: The Call Stack and Stack Frames](#1-foundations-the-call-stack-and-stack-frames)
2. [Stack Frame Anatomy on x86-64](#2-stack-frame-anatomy-on-x86-64)
3. [What Stack Unwinding Is and Why It Exists](#3-what-stack-unwinding-is-and-why-it-exists)
4. [DWARF Call Frame Information: The Unwinding Infrastructure](#4-dwarf-call-frame-information-the-unwinding-infrastructure)
5. [libunwind and the _Unwind ABI](#5-libunwind-and-the-_unwind-abi)
6. [C: No Native Unwinding — setjmp/longjmp and Manual Patterns](#6-c-no-native-unwinding--setjmplongjmp-and-manual-patterns)
7. [C++: Exception-Based Unwinding — Itanium ABI Deep Dive](#7-c-exception-based-unwinding--itanium-abi-deep-dive)
8. [Rust: panic! and Structured Unwinding](#8-rust-panic-and-structured-unwinding)
9. [Go: defer/panic/recover and Goroutine Stack Design](#9-go-deferpanicsrecover-and-goroutine-stack-design)
10. [Python: Interpreter-Level Frame Unwinding (CPython)](#10-python-interpreter-level-frame-unwinding-cpython)
11. [Java: JVM Bytecode Exception Tables](#11-java-jvm-bytecode-exception-tables)
12. [Linux Kernel Unwinding: DWARF vs ORC](#12-linux-kernel-unwinding-dwarf-vs-orc)
13. [Cross-Language FFI and Unwinding Hazards](#13-cross-language-ffi-and-unwinding-hazards)
14. [Security: Unwinding as Attack Surface](#14-security-unwinding-as-attack-surface)
15. [Performance: Zero-Cost Exception Model Analysis](#15-performance-zero-cost-exception-model-analysis)
16. [Debugging Tools and Techniques](#16-debugging-tools-and-techniques)
17. [Mental Models and Summary Tables](#17-mental-models-and-summary-tables)

---

## 1. Foundations: The Call Stack and Stack Frames

### What Is the Call Stack?

The call stack is a region of memory used by a process to track function calls, local variables,
return addresses, and saved registers. It is a last-in, first-out (LIFO) data structure that grows
and shrinks as functions are called and return.

On virtually all modern architectures, the stack grows **downward** — toward lower addresses. This
is a hardware convention, not a software choice. When you push something onto the stack, the stack
pointer decreases.

The hardware maintains a dedicated register pointing to the top of the stack:
- x86-64:   `%rsp` (stack pointer), optionally `%rbp` (frame/base pointer)
- AArch64:  `sp`, `x29` (frame pointer)
- RISC-V:   `sp` (x2), `fp` (x8/s0)

### How a Function Call Works at the Machine Level

When the CPU executes `CALL target`:
1. It pushes the return address (address of next instruction after CALL) onto the stack.
2. It transfers control to `target`.

The called function (the callee) then typically:
1. Pushes the caller's `%rbp` to save it.
2. Sets `%rbp = %rsp` to establish a new frame base.
3. Subtracts from `%rsp` to allocate space for local variables.

This is the **function prologue**. The inverse is the **epilogue**, where the function:
1. Restores `%rsp = %rbp`.
2. Pops the saved `%rbp`.
3. Executes `RET`, which pops the return address and jumps to it.

When frame pointers are omitted (e.g., `gcc -fomit-frame-pointer`), the compiler tracks offsets
from `%rsp` directly, which means an external tool cannot walk the stack using `%rbp` links alone.
This is why DWARF CFI or ORC are essential — they encode how to find each frame's registers even
without frame pointers.

---

## 2. Stack Frame Anatomy on x86-64

### System V AMD64 ABI (Linux, macOS)

Calling convention for the first 6 integer/pointer arguments: `%rdi`, `%rsi`, `%rdx`, `%rcx`,
`%r8`, `%r9`. Return value: `%rax` (and `%rdx` for 128-bit). Additional arguments are passed on
the stack.

Callee-saved (non-volatile) registers: `%rbx`, `%rbp`, `%r12`–`%r15`. The callee must save and
restore these before use.

Caller-saved (volatile) registers: `%rax`, `%rcx`, `%rdx`, `%rsi`, `%rdi`, `%r8`–`%r11`. The
callee can clobber these freely.

The stack must be 16-byte aligned before a `CALL` instruction (so `%rsp` is 16-byte aligned right
after the `CALL` pushes the return address, meaning `%rsp % 16 == 8` at function entry).

### Stack Frame Layout

```
                      HIGHER ADDRESSES
                      
  +-------------------------------------------------+
  |  Caller's stack frame (higher frames ...)       |
  +-------------------------------------------------+
  |  Argument n (7th+, passed on stack by caller)  |  <-- [rbp + 16 + 8*(n-7)]
  +-------------------------------------------------+
  |  Argument 8                                     |
  +-------------------------------------------------+
  |  Argument 7                                     |  <-- [rbp + 16]
  +-------------------------------------------------+
  |  Return Address    (pushed by CALL)             |  <-- [rbp + 8]
  +-------------------------------------------------+  <-- %rbp points here (after prologue)
  |  Saved %rbp        (pushed in prologue)         |  <-- [rbp + 0]
  +-------------------------------------------------+
  |  Local variable 1                               |  <-- [rbp - 8]
  +-------------------------------------------------+
  |  Local variable 2                               |  <-- [rbp - 16]
  +-------------------------------------------------+
  |  Local variable 3  (e.g., a struct, padded)     |  <-- [rbp - 24]
  +-------------------------------------------------+
  |  Saved %rbx        (callee-saved, if used)      |
  +-------------------------------------------------+
  |  Saved %r12                                     |
  +-------------------------------------------------+
  |  Saved %r13                                     |
  +-------------------------------------------------+
  |  Alignment padding (to keep %rsp 16-byte aligned)|
  +-------------------------------------------------+  <-- %rsp (current stack top)
  
                      LOWER ADDRESSES (stack grows down)
```

### The Red Zone (x86-64 SysV ABI)

The 128 bytes *below* `%rsp` are the "red zone" — a region the ABI guarantees won't be clobbered
by signal handlers or other asynchronous events. Leaf functions (those that call no other function)
may use the red zone for locals without adjusting `%rsp`. The kernel and interrupt handlers ignore
the red zone for user-space threads (they switch to a separate stack), but this means you **cannot**
use the red zone in kernel code — that is why Linux kernel compilation uses `-mno-red-zone`.

### Multi-Frame Call Stack View

```
  CALLER CHAIN (main → foo → bar → baz — current execution in baz)

  +--------------------------------+  Higher addresses
  |     main() frame               |
  |  local vars of main            |
  |  [saved rbp = OS/libc frame]   |
  |  [return addr: libc start]     |
  +--------------------------------+
  |     foo() frame                |
  |  local vars of foo             |
  |  [saved rbp = main's rbp]      |  ← foo's rbp
  |  [return addr: main+0x3a]      |
  +--------------------------------+
  |     bar() frame                |
  |  local vars of bar             |
  |  [saved rbp = foo's rbp]       |  ← bar's rbp
  |  [return addr: foo+0x1f]       |
  +--------------------------------+
  |     baz() frame (current)      |
  |  local vars of baz             |  ← rbp
  |  [saved rbp = bar's rbp]       |
  |  [return addr: bar+0x08]       |
  +--------------------------------+  ← rsp (current top)
  
  Lower addresses
```

The frame pointer chain (`%rbp` → saved `%rbp` → ...) lets a debugger walk the stack when frame
pointers are present. With `-fomit-frame-pointer`, this chain is broken; DWARF CFI or ORC metadata
is the only way to reconstruct frames reliably.

---

## 3. What Stack Unwinding Is and Why It Exists

### The Core Problem

Consider: `main` calls `foo`, which calls `bar`, which calls `baz`. Inside `baz`, an error
condition occurs. We need to:

1. Propagate the error to a handler that may exist somewhere up the call chain.
2. **Along the way, cleanly release every resource** that was acquired by `baz`, `bar`, or `foo`.

A simple `goto error_handler` fails because it can't clean up stack-allocated objects across
frames. A simple return-code pattern forces every function to check and propagate errors manually
(which C does and which is error-prone).

Stack unwinding is the mechanism that:
- Traverses the call stack from the point of the error upward.
- At each frame, executes any registered **cleanup handlers** (destructors, defer statements, etc.).
- Transfers control to the first frame that has a **handler** (catch block, recover(), etc.).

### Normal Return vs. Exceptional Exit

Normal return: callee executes its epilogue, pops return address, jumps to caller. Orderly, fast.
Exceptional exit: the unwinding mechanism takes over — it reads metadata tables to find where each
frame's data lives, runs cleanup code, and eventually lands in a handler frame.

The asymmetry in cost is intentional. The "happy path" (no exception) should be fast. The
"unhappy path" (exception/panic) can be slower because it is rare. This is the **zero-cost
exception model** — more on this in Section 15.

### What "Cleanup" Means Per Language

| Language | Cleanup mechanism during unwinding        |
|----------|-------------------------------------------|
| C        | None (setjmp/longjmp skips cleanup)       |
| C++      | Destructors of local objects              |
| Rust     | `Drop::drop()` of all live owned values   |
| Go       | All registered `defer` functions          |
| Python   | `finally` blocks, `__exit__` methods      |
| Java     | `finally` blocks                          |

---

## 4. DWARF Call Frame Information: The Unwinding Infrastructure

### Why DWARF CFI Exists

For a stack walker to unwind a frame, it needs to know:
1. What is the address of the calling frame's instruction (i.e., where was I called from)?
2. Where are the callee-saved registers stored in this frame?
3. How large is this frame (so we can find the previous frame's stack pointer)?

This information changes instruction-by-instruction during the function prologue and epilogue. DWARF
CFI encodes this as a **state machine program** embedded in the binary.

### ELF Sections

```
ELF Binary layout (relevant sections):

  .text         — machine code
  .rodata       — read-only data
  .data         — writable data
  .bss          — zero-initialized data

  .eh_frame     — DWARF Call Frame Information (exception handling, always present)
  .eh_frame_hdr — sorted index of FDEs for binary search (optional but standard)
  .gcc_except_table — LSDA (Language-Specific Data Area): per-function exception tables
  .debug_frame  — DWARF debug info (may be stripped; .eh_frame is not stripped)
```

The `.eh_frame` section is **always loaded into the process image** because it is needed at
runtime. `.debug_frame` is the debug version and is usually stripped from production binaries.
The linker/dynamic loader registers `.eh_frame` with the C++ runtime via `__register_frame_info`.

### CIE: Common Information Entry

```
  .eh_frame section structure:
  +-----------------------------------------------------------+
  |  CIE (Common Information Entry)                           |
  |  +-----------------------------------------------------+  |
  |  | length          (4 bytes)                           |  |
  |  | CIE_id = 0      (4 bytes; distinguishes CIE/FDE)   |  |
  |  | version = 1     (1 byte)                            |  |
  |  | augmentation    (NUL-terminated string "zPLR" etc.) |  |
  |  | code_align_fac  (ULEB128; typically 1 for x86-64)  |  |
  |  | data_align_fac  (SLEB128; typically -8 for x86-64) |  |
  |  | return_addr_col (ULEB128; column for ret addr reg)  |  |
  |  | [augmentation data length + content if 'z']         |  |
  |  |   'P' → personality function pointer               |  |
  |  |   'L' → LSDA encoding                              |  |
  |  |   'R' → FDE encoding (how FDE addresses encoded)   |  |
  |  | initial_instructions (CFI bytecode program)         |  |
  |  +-----------------------------------------------------+  |
  +-----------------------------------------------------------+
```

The `augmentation` string is critical for C++ and Rust. A typical value is `"zPLR"`:
- `z`: augmentation data length field present.
- `P`: personality function pointer is present (e.g., `__gxx_personality_v0` for C++).
- `L`: LSDA (exception handling table) encoding is present.
- `R`: FDE pointer encoding is specified.

### FDE: Frame Description Entry

```
  FDE (one per function or code range):
  +-----------------------------------------------------------+
  |  length           (4 bytes; size of this FDE minus 4)    |
  |  CIE_pointer      (4 bytes; offset to parent CIE)        |
  |  pc_begin         (function start address, encoded)       |
  |  pc_range         (length of code range this FDE covers)  |
  |  [augmentation data length, if 'z']                       |
  |    lsda_pointer  → points into .gcc_except_table          |
  |  CFI instructions (bytecode program for this function)    |
  +-----------------------------------------------------------+
```

The `CFI instructions` describe, for each instruction address in the function, what the Canonical
Frame Address (CFA) is and where each register can be found.

### CFA: Canonical Frame Address

The CFA is a **virtual register**: an abstract value representing the stack pointer as it was at
the moment of the call to this function (i.e., the caller's `%rsp` just before CALL was executed).
It is not stored anywhere — it is computed by evaluating the CFA rule for the current PC.

At function entry (before prologue):
```
  CFA = %rsp + 8        (8 bytes for the return address pushed by CALL)
  %rip saved at CFA - 8 (return address)
```

After `PUSH %rbp`:
```
  CFA = %rsp + 16
  %rip saved at CFA - 8
  %rbp saved at CFA - 16
```

After `MOV %rsp, %rbp` and `SUB $32, %rsp`:
```
  CFA = %rbp + 16       (now based on %rbp so it doesn't change as %rsp changes)
  %rip saved at CFA - 8
  %rbp saved at CFA - 16
  ... locals at [rbp-8], [rbp-16], etc.
```

The CFI state machine emits instructions like:
- `DW_CFA_def_cfa(reg, offset)` — CFA = reg + offset
- `DW_CFA_def_cfa_offset(offset)` — change the offset part of CFA
- `DW_CFA_def_cfa_register(reg)` — change the register part of CFA
- `DW_CFA_offset(reg, offset)` — register is stored at CFA + offset
- `DW_CFA_register(reg1, reg2)` — reg1 is in reg2
- `DW_CFA_undefined(reg)` — register has no recoverable value
- `DW_CFA_advance_loc(delta)` — advance current PC by delta * code_align_factor

### LSDA: Language-Specific Data Area (.gcc_except_table)

The LSDA encodes per-function exception handling information. The C++ runtime reads this during
Phase 2 (cleanup) to determine what destructors to run and which `catch` types match the thrown
exception.

```
  .gcc_except_table structure (per function):
  
  +--------------------------------------------------+
  |  lpstart_enc (DW_EH_PE_omit if same as pc_begin) |
  |  ttype_enc   (encoding for type table entries)   |
  |  ttype_offset (ULEB128, offset to type table end)|
  |  call_site_enc (encoding for call-site entries)  |
  |  call_site_len (ULEB128, size of call site table)|
  |                                                  |
  |  CALL SITE TABLE:                                |
  |  [ cs_start | cs_len | cs_lp | cs_action ]  × N |
  |  cs_start:  offset from lpstart to call site     |
  |  cs_len:    length of the call site range        |
  |  cs_lp:     offset to landing pad (0 = no LP)    |
  |  cs_action: index into action table (0 = cleanup)|
  |                                                  |
  |  ACTION TABLE:                                   |
  |  [ type_filter | next_action ] × M               |
  |  type_filter > 0: catch(type_table[-type_filter])|
  |  type_filter < 0: exception spec filter          |
  |  type_filter = 0: cleanup (finally block)        |
  |                                                  |
  |  TYPE TABLE:                                     |
  |  [ pointer to std::type_info ] × K               |
  |  (type_table[-1], type_table[-2], ...)            |
  +--------------------------------------------------+
```

A **landing pad** is a label in the generated machine code where the unwinder transfers control
after finding a matching call site. The compiler generates one landing pad per try block (or per
object with a non-trivial destructor). After transferring to the landing pad, the landing pad runs
destructors or jumps to the actual catch block.

### Reading .eh_frame with Tools

```bash
# Show raw DWARF CFI info
readelf -wF binary

# Show .eh_frame in human-readable form
readelf --debug-dump=frames binary

# Show exception tables
readelf -wE binary   # or --debug-dump=exception
```

---

## 5. libunwind and the _Unwind ABI

### The Low-Level Unwinding Protocol

The `_Unwind_*` API is defined by the **Itanium C++ ABI** and implemented by `libgcc_s` (GCC) or
`libunwind` (LLVM). All C++ and Rust exception/panic code on Linux uses this API indirectly.

Key functions:
```c
/* Raise an exception — triggers two-phase unwinding */
_Unwind_Reason_Code _Unwind_RaiseException(_Unwind_Exception *exception_object);

/* Resume unwinding after cleanup (called at end of landing pad) */
void _Unwind_Resume(_Unwind_Exception *exception_object);

/* Force unwinding (used by runtime.Goexit in Go, pthread_exit, etc.) */
_Unwind_Reason_Code _Unwind_ForcedUnwind(
    _Unwind_Exception *exception_object,
    _Unwind_Stop_Fn stop_fn,
    void *stop_parameter);

/* Read an integer register from the unwinding context */
_Unwind_Word _Unwind_GetGR(_Unwind_Context *context, int index);

/* Set an integer register in the unwinding context */
void _Unwind_SetGR(_Unwind_Context *context, int index, _Unwind_Word new_value);

/* Get the IP (instruction pointer) of the current frame */
_Unwind_Ptr _Unwind_GetIP(_Unwind_Context *context);

/* Set the IP of the current frame (used to redirect to landing pad) */
void _Unwind_SetIP(_Unwind_Context *context, _Unwind_Ptr new_value);

/* Get the language-specific data pointer for the current frame */
_Unwind_Ptr _Unwind_GetLanguageSpecificData(_Unwind_Context *context);
```

The `_Unwind_Exception` struct is the core:
```c
struct _Unwind_Exception {
    uint64_t  exception_class;   /* 8-byte tag (e.g., "GNUCC++\0" for G++) */
    _Unwind_Exception_Cleanup_Fn exception_cleanup;  /* called when done */
    uintptr_t private_1;         /* used by the unwinder internally */
    uintptr_t private_2;         /* used by the unwinder internally */
} __attribute__((aligned(16)));
```

The `exception_class` field lets the personality function know which runtime threw the exception
(allowing foreign exception passthrough — e.g., C++ catching Objective-C exceptions on macOS).

### The Personality Function

The **personality function** is the bridge between the generic unwinder and the language runtime.
It is called by the unwinder once per frame during both phases. Its job:

```c
_Unwind_Reason_Code __gxx_personality_v0(
    int version,
    _Unwind_Action actions,       /* combination of phase flags */
    uint64_t exceptionClass,
    _Unwind_Exception *ue_header,
    _Unwind_Context *context);    /* opaque context for register access */
```

The `actions` parameter is a bitmask:
```
_UA_SEARCH_PHASE   (1) — Phase 1: just look for a handler, no side effects
_UA_CLEANUP_PHASE  (2) — Phase 2: run cleanups and route to handler
_UA_HANDLER_FRAME  (4) — this frame was identified as the handler in Phase 1
_UA_FORCE_UNWIND   (8) — forced unwind, should not be caught
```

### Two-Phase Unwinding: The Complete Protocol

```
  PHASE 1 — Search Phase (no side effects, pure scan):
  =====================================================

  _Unwind_RaiseException() called
          |
          v
  +---------------+
  |  Frame N      |  personality(SEARCH_PHASE)
  |  (thrower)    |  → reads LSDA
  |               |  → no catch for this type
  |               |  → return _URC_CONTINUE_UNWIND
  +---------------+
          | (unwinder reads CFI, steps to previous frame)
          v
  +---------------+
  |  Frame N-1    |  personality(SEARCH_PHASE)
  |               |  → reads LSDA
  |               |  → no catch for this type
  |               |  → return _URC_CONTINUE_UNWIND
  +---------------+
          |
          v
  +---------------+
  |  Frame N-2    |  personality(SEARCH_PHASE)
  |               |  → reads LSDA
  |               |  → catch(T) MATCHES thrown type!
  |               |  → return _URC_HANDLER_FOUND
  +---------------+
          |
          v
  (Frame N-2 saved as "handler frame" in exception object)
  (Phase 1 ends. No stack change yet. No destructors. No side effects.)


  PHASE 2 — Cleanup Phase (destructors and control transfer):
  ===========================================================

  _Unwind_RaiseException() starts Phase 2, walking from Frame N again
          |
          v
  +---------------+
  |  Frame N      |  personality(CLEANUP_PHASE)
  |  (thrower)    |  → reads LSDA: has landing pad for cleanup?
  |               |  → yes: SetIP(landing_pad), SetGR(reg, exception_ptr)
  |               |  → return _URC_INSTALL_CONTEXT
  +-----|  ^-------+
        |  |
        v  |  (machine jumps to landing pad in Frame N)
  [landing pad: runs destructors for objects in Frame N]
  [calls _Unwind_Resume() to continue unwinding]
        |
        v
  +---------------+
  |  Frame N-1    |  personality(CLEANUP_PHASE)
  |               |  → has cleanups → install landing pad
  +-----|  ^-------+
        |  |
        v  |
  [landing pad: runs destructors for Frame N-1]
  [calls _Unwind_Resume()]
        |
        v
  +-----|-----------+
  |  Frame N-2      |  personality(CLEANUP_PHASE | HANDLER_FRAME)
  |  (handler frame)|  → found catch block → install catch landing pad
  |                 |  → return _URC_INSTALL_CONTEXT
  +-----------------+
        |
        v
  [catch landing pad: extracts exception, runs catch body]
```

The key insight: **Phase 1 is a dry run**. It prevents running destructors in Frame N or N-1 if
no handler is found at all (which would be worse than useless since std::terminate() would be
called anyway). Only after confirming a handler exists does Phase 2 commit to cleanup.

---

## 6. C: No Native Unwinding — setjmp/longjmp and Manual Patterns

### The Absence Problem

C has no exception model, no destructors, and no automatic cleanup. If a function deep in a call
chain encounters a fatal error, the options are:
1. Return an error code and have every caller check and propagate it (the idiomatic C way).
2. Use `setjmp`/`longjmp` for non-local jumps.
3. Abort via `exit()`, `_exit()`, `abort()`.
4. Use POSIX signals for asynchronous error delivery.
5. Use `goto` within a function for cleanup (the famous "goto cleanup" pattern).

### setjmp / longjmp

```c
#include <setjmp.h>

jmp_buf env;   /* stores CPU register state */

int setjmp(jmp_buf env);   /* saves state, returns 0 on first call */
void longjmp(jmp_buf env, int val); /* restores state, setjmp appears to return val */
```

Internally on x86-64, `setjmp` saves: `%rbx`, `%rbp`, `%r12`–`%r15`, `%rsp`, `%rip` (the return
address of setjmp's caller, which is where execution resumes). The `jmp_buf` is essentially a
snapshot of the callee-saved registers and the return address.

`longjmp` restores all those registers and jumps to the saved `%rip`. The CPU is now executing at
the instruction after the original `setjmp` call, with the saved register state, as if `setjmp`
returned `val`.

**Critical limitation**: `longjmp` does NOT run any destructors, C++ exceptions, or any cleanup
code for frames it skips over. It also makes all non-`volatile` local variables in the `setjmp`
frame have indeterminate values after `longjmp` returns.

**Security concern**: `glibc` implements `__longjmp_chk` and pointer mangling on `jmp_buf`
to detect stack corruption and prevent `jmp_buf` hijacking. The saved `%rsp` and `%rip` in
`jmp_buf` are XOR'd with a secret value (`PTR_MANGLE`). This is not a full security guarantee
but raises the cost of exploitation.

### The Goto Cleanup Pattern (Idiomatic C)

```c
int function_with_cleanup(void) {
    char *buf = NULL;
    int fd = -1;
    int rc = 0;

    buf = malloc(4096);
    if (!buf) { rc = -ENOMEM; goto out; }

    fd = open("/etc/passwd", O_RDONLY);
    if (fd < 0) { rc = -errno; goto out_free; }

    /* ... work ... */

out_close:
    close(fd);
out_free:
    free(buf);
out:
    return rc;
}
```

This is **stack unwinding done manually**. Labels correspond to cleanup points. This is error-
prone (easy to miss a label or get order wrong) but has zero overhead and zero magic.

### POSIX Signals and Stack Unwinding

Signal handlers are delivered asynchronously. The kernel pushes a signal frame onto the user-space
stack. When the signal handler returns, it executes a `sigreturn` system call that pops the signal
frame and restores the original execution context.

Signal handlers break stack unwinding badly:
- DWARF CFI does not account for signal frames in standard object files.
- `libunwind` provides `_Unwind_Context` that can detect signal frames via
  `_Unwind_GetIPInfo()` returning a special flag.
- Tools like `perf` and `gdb` have signal frame awareness built in.

If a signal handler calls `longjmp` out of the signal context, the signal mask is **not**
restored. Use `sigsetjmp`/`siglongjmp` to save and restore the signal mask.

### libunwind C API for Explicit Stack Walking

```c
#define UNW_LOCAL_ONLY
#include <libunwind.h>

void print_backtrace(void) {
    unw_cursor_t cursor;
    unw_context_t uc;
    unw_word_t ip, sp;
    char sym[256];

    unw_getcontext(&uc);          /* capture current CPU state */
    unw_init_local(&cursor, &uc); /* initialize cursor at current frame */

    while (unw_step(&cursor) > 0) {  /* step to previous frame */
        unw_get_reg(&cursor, UNW_REG_IP, &ip);
        unw_get_reg(&cursor, UNW_REG_SP, &sp);
        unw_get_proc_name(&cursor, sym, sizeof(sym), NULL);
        fprintf(stderr, "  ip=0x%lx sp=0x%lx  %s\n", ip, sp, sym);
    }
}
```

`unw_step` evaluates the DWARF CFI for the current frame to find the previous frame's register
values. It is the core of all stack-walking tools on Linux.

---

## 7. C++: Exception-Based Unwinding — Itanium ABI Deep Dive

### The throw Mechanism

When you write `throw SomeException("msg")`, the compiler generates:

```cpp
// Conceptual expansion of: throw SomeException("msg");
void *exception_obj = __cxa_allocate_exception(sizeof(SomeException));
try {
    new (exception_obj) SomeException("msg");  // construct in place
} catch (...) {
    __cxa_free_exception(exception_obj);
    throw;
}
__cxa_throw(exception_obj, typeid(SomeException), SomeException::~SomeException);
```

`__cxa_throw` is the key function. It:
1. Stores the `type_info` pointer and destructor in the exception object header.
2. Sets up the `_Unwind_Exception` struct embedded in the CXA exception header.
3. Calls `_Unwind_RaiseException`.

The full CXA exception object layout:
```
  +-------------------------------------------+
  |  __cxa_exception header:                   |
  |  +-----------------------------------------|
  |  | type_info *exceptionType                |
  |  | void (*exceptionDestructor)(void *)     |
  |  | std::unexpected_handler unexpectedHandler|
  |  | std::terminate_handler terminateHandler |
  |  | __cxa_exception *nextException           |  (exception chain)
  |  | int handlerCount                        |
  |  | int handlerSwitchValue                  |
  |  | const char *actionRecord                |
  |  | const char *languageSpecificData        |
  |  | void *catchTemp                         |
  |  | void *adjustedPtr                       |
  |  | _Unwind_Exception unwindHeader          |  ← this is what libunwind sees
  +-------------------------------------------+
  |  exception object data (the thrown value)  |
  +-------------------------------------------+
```

`__cxa_allocate_exception` typically allocates from a thread-local emergency buffer (to avoid
failing to allocate during OOM conditions, which would make `std::bad_alloc` impossible to throw).

### The Personality Function: __gxx_personality_v0

This is GCC's C++ personality function. When called during Phase 1 with `_UA_SEARCH_PHASE`:

1. Extract `lsda_pointer` from the current frame's FDE augmentation data.
2. Determine the current `%rip` (instruction pointer).
3. Search the call-site table in the LSDA for a matching entry.
4. For the matching call site, walk the action table.
5. For each action, compare the thrown `type_info` against entries in the type table using
   `std::type_info::__do_catch()`.
6. If a match: return `_URC_HANDLER_FOUND`.
7. If cleanup only (destructor, not catch): continue.
8. If no entry: return `_URC_CONTINUE_UNWIND`.

During Phase 2 with `_UA_CLEANUP_PHASE`:

1. Find the landing pad address for the current call site.
2. Set the context IP to the landing pad address via `_Unwind_SetIP`.
3. Set a register (typically `%rax` on x86-64) to point to the exception object.
4. Set another register (typically `%rdx`) to the handler switch value.
5. Return `_URC_INSTALL_CONTEXT`.

The landing pad in machine code looks like:

```
; Generated landing pad for a scope with a std::string local:
landing_pad:
    ; %rax = exception object pointer (set by personality function)
    ; %rdx = handler switch value
    call __cxa_begin_catch   ; register with CXA runtime that we're handling
    ; ... destructor calls for objects in scope ...
    ; If this is cleanup (not catch), call _Unwind_Resume:
    call _Unwind_Resume
    ; If this is a catch block, the catch body follows:
catch_body:
    ; user catch code
    call __cxa_end_catch     ; decrement handler count, call destructor if needed
```

### RAII and Destructors: The Cleanup Contract

Every local variable with a destructor that goes out of scope during unwinding has its destructor
called. This is guaranteed by the standard. The compiler ensures this by:
1. For each scope that has objects with destructors, the compiler generates a call-site table
   entry in the LSDA pointing to a landing pad.
2. That landing pad calls destructors, then calls `_Unwind_Resume` to continue unwinding.

If a destructor throws during stack unwinding, `std::terminate` is called immediately (nested
exception with active exception = terminate). This is why destructors must be `noexcept`.

### noexcept: Optimization and Guarantee

```cpp
void guaranteed_nothrow() noexcept {
    // If an exception propagates out of here, std::terminate() is called
    // The compiler can optimize: no landing pads, no EH tables, faster code
}
```

`noexcept` functions get **no EH metadata** generated (no LSDA entries, no personality pointer
in FDE augmentation). This means the unwinder, upon reaching a `noexcept` frame, cannot find
cleanup code — it calls `std::terminate`. This is correct behavior but means you can never
accidentally propagate through `noexcept`.

Importantly, `noexcept` functions in `extern "C"` linkage have the same effect, which is relevant
for C callbacks called from C++ code.

### Exception Safety Levels (Production Contracts)

```
Nothrow guarantee (strongest):
  - The operation never throws. Period.
  - Use noexcept.
  - Example: destructors, swap(), move constructors.

Strong guarantee:
  - If the operation throws, the program state is unchanged (atomic).
  - Copy-then-swap idiom achieves this.

Basic guarantee:
  - If the operation throws, program is in a valid (but unspecified) state.
  - No resources are leaked.
  - Minimum acceptable level for production code.

No guarantee (worst):
  - Resources may leak, invariants may be broken.
  - Never acceptable in production.
```

### std::terminate vs std::unexpected

`std::terminate` is called when:
- An exception propagates out of `main()`.
- An exception is thrown during stack unwinding (double-exception).
- A `noexcept` function throws.
- A pure virtual function is called during construction/destruction.
- `std::rethrow_exception` is called with a null pointer.

`std::unexpected` (C++03 only, removed in C++17) was called when a function with an exception
specification threw something not in the spec. Replaced by `noexcept` in modern C++.

### Exception Re-throwing and Propagation

```cpp
std::exception_ptr ep;

try {
    // ... some code that might throw ...
} catch (...) {
    ep = std::current_exception();   // capture exception (ref-counted)
}

// Later, in another thread or context:
std::rethrow_exception(ep);          // re-throw; triggers full unwinding again
```

`std::exception_ptr` is reference-counted. The exception object lives until the last
`exception_ptr` to it is destroyed. This is how exception cross-thread transfer works in
`std::future`/`std::promise`.

---

## 8. Rust: panic! and Structured Unwinding

### Philosophy: Panics Are Not Exceptions

Rust draws a hard line between **expected errors** and **unexpected errors**:

- Expected errors → `Result<T, E>` — returned, checked, propagated explicitly.
- Unexpected errors → `panic!` — program logic violated; cannot continue.

`panic!` is not a general-purpose error handling mechanism. It is for: indexing out of bounds,
integer overflow in debug mode, `unwrap()` on `None`, assertion failures, and explicit
`panic!("invariant violated")` calls. In production network/kernel code: panics should be audited
away or turned into `Result` paths wherever possible.

### Two Panic Strategies: unwind vs abort

Configured in `Cargo.toml`:
```toml
[profile.release]
panic = "abort"    # calls core::intrinsics::abort() immediately
                   # smallest binary, no unwinding overhead
                   # no cleanup, no Drop, memory freed by OS on exit

[profile.dev]
panic = "unwind"   # default; enables catch_unwind and Drop during panics
```

For systems programming (kernel modules, embedded, safety-critical code), `panic = "abort"` is
often the right choice. It eliminates all EH overhead, and cleanup should be handled explicitly.

For user-space services where thread isolation matters (each request in its own thread), `panic =
"unwind"` + `catch_unwind` at thread boundaries allows a thread to die cleanly without killing
the process.

### How unwind Mode Works Internally

Rust with `panic = "unwind"` uses the **same Itanium ABI mechanism** as C++:

1. `panic!` macro → `std::panicking::begin_panic` or `core::panicking::panic`
2. → `__rust_start_panic` (extern "C" link)
3. → allocates a `Box<dyn Any + Send>` for the panic payload (using `__rust_alloc`)
4. → calls `_Unwind_RaiseException` (same libgcc/libunwind mechanism as C++)

Rust has its own personality function: `rust_eh_personality` (or `__rust_eh_personality` in some
versions). It behaves similarly to `__gxx_personality_v0` but reads Rust-generated LSDA data.

The compiler generates landing pads for all scopes containing values that implement `Drop`. During
Phase 2 cleanup, these landing pads call `Drop::drop()` for each live value.

### catch_unwind: Safe Boundary

```rust
use std::panic;

let result = panic::catch_unwind(|| {
    // code that might panic
    do_risky_thing()
});

match result {
    Ok(val)  => { /* normal return */ }
    Err(payload) => {
        // panic was caught; extract payload with downcast
        if let Some(s) = payload.downcast_ref::<&str>() {
            eprintln!("panicked with: {}", s);
        }
    }
}
```

Internally, `catch_unwind` is implemented using `__rust_try` — a compiler intrinsic that sets up
a C++ style try block (a landing pad that catches any exception and converts it to a `Result`).
The closure passed to `catch_unwind` must be `UnwindSafe`, meaning it does not hold references
to state that could be left in an inconsistent state after a panic.

### Drop Trait and Unwinding

Every type in Rust with a `Drop` impl will have it called when:
- The value's scope ends normally.
- The value's scope is exited via panic unwinding.

The compiler statically tracks which values are "live" at each point using the **borrow checker
and drop elaboration pass**. Drop glue is generated for every type. During unwinding, the same
drop glue runs as during normal exits.

```
  Scope with live values during panic:

  fn example() {
      let guard = MutexGuard::new(&MUTEX);   // ← Drop: releases mutex
      let buf = Vec::<u8>::with_capacity(1024); // ← Drop: frees heap
      
      do_something_that_panics();
      
      // If panic occurs here:
      // 1. buf.drop() called first (LIFO)
      // 2. guard.drop() called next (releases mutex)
      // 3. Unwinding continues up the stack
  }
```

### Mutex Poisoning

If a thread panics while holding a `std::sync::MutexGuard`, the mutex becomes **poisoned**. The
next `lock()` on that mutex returns `Err(PoisonError)`. This signals to other threads that the
guarded data may be in an inconsistent state.

```rust
let result = mutex.lock();
match result {
    Ok(guard) => { /* normal */ }
    Err(poisoned) => {
        // Can choose to recover the guard anyway:
        let guard = poisoned.into_inner();
        // But you must manually verify data consistency!
    }
}
```

This is a deliberate safety feature — if you were in the middle of modifying shared state and
panicked, you should not silently allow other threads to see corrupt data.

### Panics in no_std Environments

Without `std`, you must provide a `#[panic_handler]`:

```rust
#![no_std]

use core::panic::PanicInfo;

#[panic_handler]
fn panic_handler(info: &PanicInfo) -> ! {
    // info.location() → file and line
    // info.message()  → format string
    // For kernel code:
    // loop {} or system_halt() or serial_print_and_halt()
    loop {}
}
```

This replaces the entire panic runtime. There is no unwinding — `panic = "abort"` is the only
option in `no_std` unless you bring in a custom unwinder.

### async/await and Panics

A panic inside an `async` block panics the **task**, not the thread. The executor (e.g., tokio)
wraps task execution in `catch_unwind`. When a task panics, the `JoinHandle` returned from
`task::spawn` will contain `Err(JoinError::panic(...))` when awaited.

```rust
let handle = tokio::spawn(async {
    panic!("task exploded");
});
match handle.await {
    Ok(result) => { /* task completed */ }
    Err(e) if e.is_panic() => { /* task panicked */ }
    Err(e) => { /* task cancelled */ }
}
```

### FFI and Panic Safety

**This is critical for systems code**: unwinding a panic across an FFI boundary (`extern "C"`)
into C or other non-Rust code is **undefined behavior** in current Rust (RFC 2945 is working to
fix this with `extern "C-unwind"`).

```rust
extern "C" fn callback_from_c() {
    // DO NOT let a panic escape here!
    // Wrap with catch_unwind:
    let _ = std::panic::catch_unwind(|| {
        // risky code
    });
}
```

The `extern "C-unwind"` ABI (stabilized in Rust 1.73) allows C ABIs to unwind through Rust
and vice versa safely. But the called C code must be compiled with appropriate unwind tables.

---

## 9. Go: defer/panic/recover and Goroutine Stack Design

### Goroutine Stack Architecture

Go goroutines start with a **small stack** (currently 2KB or 8KB depending on configuration)
and grow as needed. This is fundamentally different from threads (which typically get 8MB stacks
at creation).

```
  Initial goroutine stack (not contiguous with other goroutines):
  
  +------------------------------------+  ← top of stack (higher addr)
  |  goroutine metadata (g struct)     |
  |  stackguard0                       |  ← trap threshold
  |  stackguard1                       |  ← for CGO
  +====================================+
  |  goroutine function frames...      |
  |  frame for main goroutine func     |
  |  frame for goroutine launch wrapper|
  +====================================+
  |  [stack grows downward here]       |
  |  ...                               |
  +------------------------------------+  ← lo: bottom of stack (lower addr)

  Each function's prologue checks:
  if SP < stackguard0 { call runtime.morestack }
```

**Stack Growth Mechanism (Contiguous Stacks since Go 1.4)**:

```
  Goroutine stack overflow detected (SP < stackguard0):
  
  1. runtime.morestack() is called
  2. New stack allocated (2x the current stack size)
  3. All frames COPIED to new stack
  4. ALL POINTERS into old stack updated (GC-assisted pointer scan)
  5. Old stack freed
  6. Goroutine resumes on new stack

  Shrinking: if stack is <1/4 used after GC, shrink to 1/2 size.
```

This copying approach replaced the old **segmented stacks** model (Go ≤ 1.3). Segmented stacks
had the **hot split problem**: a function called near a stack boundary would repeatedly
allocate/deallocate a new segment on every call (e.g., in a tight loop), causing O(n) overhead.
Contiguous stacks pay a higher cost for growth but eliminate the hot split problem entirely.

### defer: Implementation and Semantics

`defer f()` registers a deferred call that runs when the surrounding function **returns** (either
normally or via panic). Deferred calls run in **LIFO** order.

**Old implementation** (pre-Go 1.14): `defer` heap-allocates a `_defer` struct and links it into
the goroutine's `_defer` list:
```
  g._defer → [_defer: fn=f3, args...] → [_defer: fn=f2] → [_defer: fn=f1] → nil
```

**Open-coded defers** (Go 1.14+): For defer calls that can be analyzed statically (e.g., no
defer inside a loop), the compiler **inlines** the deferred calls directly at each return site.
A bitmask in the function records which defers have been "activated". This avoids the heap
allocation and pointer chasing, making typical defer nearly zero-cost on the happy path.

### panic/recover Protocol

```go
func safeDivide(a, b int) (result int, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered: %v", r)
        }
    }()
    return a / b, nil   // b == 0 → runtime panic: division by zero
}
```

When `panic(value)` is called:
1. The Go runtime creates a `_panic` struct and links it to the goroutine: `g._panic`.
2. The runtime begins executing deferred functions **from the current function outward**.
3. For each deferred function, before calling it, it links the `_panic` as `g._panic`.
4. If a deferred function calls `recover()`, and there is an active `g._panic`:
   - `recover()` returns the panic value.
   - The `_panic` is marked as recovered.
   - The deferred function continues normally.
   - After the deferred function returns, the panic is considered handled.
   - The frame that contained the `defer` returns normally (with whatever return values
     were set by the deferred function).

```
  panic() execution flow:

  goroutine executing...
       |
       v
  panic("oh no") called in frame F3
       |
       v
  runtime marks g._panic = &{val: "oh no", recovered: false}
       |
       v
  Run deferred functions for F3 (LIFO) → none
       |
       v
  Unwind to F2, run deferred functions for F2 (LIFO)
       |
       v
  Unwind to F1, run deferred functions for F1:
    → deferred func called
    → recover() returns "oh no", marks _panic.recovered = true
    → deferred func can set return values of F1
    → deferred func returns
       |
       v
  F1 returns normally (panic recovered)
  Goroutine continues executing at F0's code after the F1 call
```

If no `recover()` is reached, after all deferred functions run, the runtime:
1. Prints the panic value and stack trace.
2. Calls `runtime.throw` → terminates all goroutines and the process.

### runtime.Goexit()

`runtime.Goexit()` terminates the current goroutine cleanly — all deferred functions run, but
the goroutine exits without recovering. Unlike panic, `Goexit` **cannot be stopped by recover()**.

Internally, `Goexit` sets a flag `g._panic.isGoExit` (or equivalent). During the defer unwinding,
`recover()` detects this flag and returns `nil` instead of stopping the unwind.

```go
func worker() {
    defer cleanup()        // WILL run
    defer log("exiting")   // WILL run

    doSetup()
    if configInvalid {
        runtime.Goexit()   // returns to goroutine launcher with defers run
    }
    // NOT reached after Goexit
    doWork()
}
```

### Go vs C++/Rust: Key Differences in Unwinding

```
  C++/Rust (DWARF-based):               Go (runtime-driven):
  ─────────────────────                 ──────────────────────
  Two-phase: search then cleanup        Single-phase: run defers as you go
  Personality function per-frame        Runtime walks goroutine _defer list
  Metadata in .eh_frame/.gcc_except_*  Metadata in Go runtime structures
  Exception type matching               No exception type matching (one panic val)
  Can catch specific types              recover() catches any panic value
  Cross-frame destructor order: LIFO    defer order: LIFO, per-goroutine
  Stack frames unmodified in Phase 1    No Phase 1 equivalent
  Works across FFI (C-unwind)           Does NOT work across CGO boundary safely
```

---

## 10. Python: Interpreter-Level Frame Unwinding (CPython)

### CPython Frame Objects

Python's call stack is maintained by CPython's **eval loop** (`ceval.c`). Each function invocation
creates a `PyFrameObject` (a Python heap object):

```
  PyFrameObject:
  +-----------------------------------------------+
  |  f_back       → previous frame (linked list)  |
  |  f_code       → PyCodeObject (bytecode etc.)  |
  |  f_globals    → dict of global names          |
  |  f_locals     → dict or fastlocals array      |
  |  f_lasti      → last bytecode index executed  |
  |  f_lineno     → current line number           |
  |  f_stacktop   → eval stack pointer            |
  |  [exception state: f_exc_type, value, tb]     |
  +-----------------------------------------------+
```

The C stack (CPython's own stack) has a `_PyEval_EvalFrameDefault` activation for each Python
frame. Python frames are heap objects — they can be accessed, inspected, and even modified at
runtime via `sys._getframe()`.

### Exception Tables (CPython 3.11+)

Prior to Python 3.11, exception handling used "block stacks" in the frame object — per-bytecode-
block records of what handlers were active. From 3.11, CPython uses a compact **exception table**
similar to LSDA in C++:

```
  Exception table entry (Python 3.11+):
  +----------------------------------------------------------+
  | start_offset  | end_offset  | target  | depth  | lasti  |
  | (bytecode range this entry covers)   | (stack cleanup)  |
  +----------------------------------------------------------+
  
  start_offset, end_offset: range of bytecode instructions this covers
  target: where to jump if an exception occurs in [start, end)
  depth:  how many operand stack entries to pop on exception
  lasti:  whether to update f_lasti to the exception's instruction
```

The eval loop checks on each exception whether the current bytecode offset falls in any exception
table entry. If so, it jumps to `target`. This is essentially the same as LSDA call-site table
lookup, but operating on Python bytecode instead of machine code.

### Exception Object Hierarchy and Propagation

```
  BaseException
  ├── SystemExit
  ├── KeyboardInterrupt
  ├── GeneratorExit
  └── Exception
      ├── StopIteration
      ├── StopAsyncIteration
      ├── ArithmeticError
      │   ├── ZeroDivisionError
      │   └── OverflowError
      ├── LookupError
      │   ├── IndexError
      │   └── KeyError
      ├── ValueError
      ├── TypeError
      ├── RuntimeError
      └── ... (many more)
```

When an exception occurs:
1. CPython creates an exception object (a `PyObject *`).
2. Sets the current exception state in the thread state (`PyThreadState`).
3. Returns `NULL` from the C function that failed.
4. The eval loop checks for `NULL` return and starts unwinding bytecode frames.
5. At each frame, looks up the exception table; if a handler exists, jumps there.
6. If no handler, pops the frame and propagates to the calling frame.

### try/except/finally/with

```python
try:
    risky()
except ValueError as e:
    handle_value_error(e)
except (TypeError, KeyError):
    handle_type_or_key()
except Exception:
    handle_generic()
else:
    # Runs only if NO exception was raised
    post_success()
finally:
    # ALWAYS runs (even if except re-raises, or return is in try)
    cleanup()
```

The `finally` block is guaranteed to run. Internally, CPython compiles `finally` blocks into
handler targets that execute the finally code then either re-raise the exception or continue
normally. The exception state is saved/restored around finally execution.

The `with` statement:
```python
with ContextManager() as cm:
    body()
```
Compiles to roughly:
```python
cm = ContextManager()
val = cm.__enter__()
exc = True
try:
    body()
except:
    exc = False
    if not cm.__exit__(*sys.exc_info()):
        raise
    # If __exit__ returns True: exception suppressed
finally:
    if exc:
        cm.__exit__(None, None, None)
```

`__exit__(exc_type, exc_val, exc_tb)` returning a truthy value suppresses the exception — this is
Python's analog of catching an exception and not re-raising.

### Exception Chaining

```python
try:
    open("file")
except FileNotFoundError as e:
    raise RuntimeError("config missing") from e
    # e.__cause__ = FileNotFoundError
    # "The above exception was the direct cause..."
```

Implicit chaining (without `from`): if an exception occurs during exception handling,
`exc.__context__` is set to the currently active exception.

### Generator throw() and Async Coroutines

Generators can receive exceptions:
```python
def gen():
    try:
        value = yield
    except ValueError:
        yield "caught"

g = gen()
next(g)           # advance to yield
g.throw(ValueError("oops"))  # inject exception at yield point
```

`generator.throw(type, value, traceback)` raises the given exception at the point where the
generator is suspended. This is Python's mechanism for coroutine cancellation. `asyncio.Task`
cancellation works by calling `coro.throw(CancelledError())` on the coroutine object.

---

## 11. Java: JVM Bytecode Exception Tables

### JVM Exception Handling Architecture

Java exceptions are handled at the **bytecode** level. Each compiled method (`.class` file)
contains an exception table alongside the bytecode:

```
  Method bytecode:
  +------------------------------------------+
  |  0:  invokevirtual #2  (risky())          |
  |  3:  astore_1          (result = ...)     |
  |  4:  goto 22           (skip catch block) |
  |                                           |
  |  7:  astore_2          (catch block: e=)  |  ← exception handler
  |  8:  aload_2                              |
  |  9:  invokevirtual #3  (e.getMessage())   |
  |  ...                                      |
  | 22:  [finally body / normal continuation] |
  +------------------------------------------+

  Exception table:
  +-------------------------------------------------+
  | from | to  | target | type                      |
  |  0   |  3  |   7    | java/lang/IOException     |
  |  0   | 22  |  30    | any (finally handler)     |
  +-------------------------------------------------+
```

Fields:
- `from`, `to`: bytecode range (inclusive, exclusive) where this handler is active.
- `target`: bytecode offset of the handler (the catch block entry point).
- `type`: fully qualified exception class, or 0 for `finally` (catch-all).

When an exception is thrown (`athrow` bytecode or native method):
1. JVM searches the current method's exception table for an entry where `from ≤ PC < to` and
   `type` matches the exception class (or is a superclass).
2. If found: push exception reference on operand stack, jump to `target`.
3. If not found: pop the current frame from the call stack, repeat in the calling method.
4. If the bottom of the call stack is reached: call `Thread.getUncaughtExceptionHandler()`.

### Checked vs Unchecked Exceptions

```
  Exception hierarchy in Java:
  
  Throwable
  ├── Error (unchecked — program errors, not caught by applications)
  │   ├── OutOfMemoryError
  │   ├── StackOverflowError
  │   └── VirtualMachineError
  └── Exception (checked — must be declared in throws clause or caught)
      ├── IOException (checked)
      ├── SQLException (checked)
      └── RuntimeException (unchecked — not required to declare)
          ├── NullPointerException
          ├── IndexOutOfBoundsException
          └── ClassCastException
```

Checked exceptions are a **compile-time** contract enforced by `javac`. The JVM itself does not
distinguish checked from unchecked — the exception table handles both identically at runtime.

### finally Block Compilation

The Java compiler **duplicates** the `finally` block for every possible exit path:
```java
try {
    risky();
} catch (IOException e) {
    handle(e);
} finally {
    cleanup();    // appears 3 times in bytecode:
                  // 1. After normal try exit
                  // 2. After catch block exit
                  // 3. As exception handler for any uncaught exception
}
```

There is no `goto`-to-finally in JVM bytecode. The finally code is inlined at each exit point.
The exception handler version (`type=0` in exception table) catches any exception, runs the
finally code, then re-throws the exception.

### try-with-resources (Java 7+)

```java
try (InputStream is = new FileInputStream(path)) {
    process(is);
}
```

Compiles to (roughly):
```java
InputStream is = new FileInputStream(path);
Throwable primaryExc = null;
try {
    process(is);
} catch (Throwable t) {
    primaryExc = t;
    throw t;
} finally {
    if (is != null) {
        if (primaryExc != null) {
            try {
                is.close();
            } catch (Throwable suppressedExc) {
                primaryExc.addSuppressed(suppressedExc);  // Java 7 feature
            }
        } else {
            is.close();
        }
    }
}
```

`addSuppressed` / `getSuppressed()` keeps track of exceptions that occurred during cleanup,
so they are not silently lost.

### StackOverflowError

JVM maintains a stack depth limit per thread. When exceeded, it throws `StackOverflowError` (a
subclass of `Error`, unchecked). Notably:
- The JVM reserves extra stack space to ensure `StackOverflowError` can be thrown and handled.
- You can catch `StackOverflowError` (though it's rarely meaningful).
- After catching, stack space is available again (the stack unwound to the catch point).

### JIT Compilation and Exceptions

HotSpot JVM compiles hot methods to native code but still maintains exception tables in a
JIT-specific format. JIT-compiled frames can be **deoptimized** — converted back to interpreted
frames — if needed (e.g., when a debugger sets a breakpoint or when an unusual exception path
is taken). This is transparent to the programmer but means exception handling in JIT code
involves a potential deoptimization step.

---

## 12. Linux Kernel Unwinding: DWARF vs ORC

### Why the Kernel Needs Its Own Unwinder

The kernel has unique constraints that make userspace DWARF unwinding inadequate:
- No userspace libraries (no libunwind, no libgcc).
- Highly optimized code with aggressive frame pointer omission.
- Interrupt context: the interrupted code's frame is special (not called by CALL).
- NMI (non-maskable interrupt): may interrupt any point including other NMI handlers.
- Stack switching: kernel entry from userspace, IRQ stacks, exception stacks.
- Speed matters: stack traces in `perf`, `ftrace`, `kprobes` must be fast.

### DWARF Unwinding in the Kernel (Legacy/Old)

The kernel historically used frame-pointer-based unwinding (`CONFIG_FRAME_POINTER`) or DWARF.
DWARF was used by `perf` via `--call-graph dwarf`, but the kernel itself couldn't use libunwind
(userspace tool). Tools like `perf` injected DWARF parsing code or used `libdw` (elfutils).

Problems:
- DWARF parsing is slow and complex.
- DWARF generated by compilers for kernel code was sometimes incorrect for optimized builds.
- Interrupt frames have no DWARF description by default.

### ORC: Oops Rewind Capability (Linux ≥ 4.14)

ORC was designed by Josh Poimboeuf (Red Hat). It replaces DWARF for kernel stack unwinding.
`CONFIG_UNWINDER_ORC` is the default on x86 since kernel 4.14.

**ORC Format**: Two parallel ELF sections:

```
  .orc_unwind_ip   (instruction pointer array — sorted, relative offsets)
  .orc_unwind      (ORC entry array — parallel to .orc_unwind_ip)

  Each .orc_unwind entry (6 bytes):
  +----------------------------------------------+
  | sp_offset (s16)   | bp_offset (s16)          |
  | sp_reg    (4 bits) | bp_reg   (4 bits)        |
  | type      (2 bits) | end      (1 bit)  | pad  |
  +----------------------------------------------+

  sp_reg, sp_offset: describes where to find the caller's SP
  bp_reg, bp_offset: describes where to find the caller's BP (%rbp)
  type: CALL, REGS, REGS_PARTIAL (for signal/exception frames)
  end: marks end of function range
```

ORC is simpler than DWARF: it only needs to track SP and BP (to find the previous frame), not the
full register set. For a stack trace (not full register recovery), this is sufficient.

**Lookup**: Binary search on `.orc_unwind_ip` using the current instruction pointer. O(log n).
This is much faster than DWARF evaluation, which requires running a stack machine.

**objtool**: A build-time static analysis tool that generates ORC tables by analyzing compiled
kernel object files. It statically simulates the execution of each function to determine SP
behavior at every instruction. This is more accurate than relying on compiler-generated DWARF,
especially for hand-written assembly.

```
  ORC vs DWARF Comparison:

  Property              ORC                     DWARF
  ────────────────────  ──────────────────────  ─────────────────────
  Section size          Small (6 bytes/entry)   Large (DWARF opcodes)
  Lookup speed          O(log n) binary search  O(log n) + opcode eval
  Accuracy              High (objtool analysis) Variable (compiler)
  Interrupt frames      Supported (type=REGS)   Partial/separate
  Register recovery     SP + BP only            Full register set
  Dependency            objtool at build time   Compiler DWARF output
  Signal frame support  Yes (type=REGS_PARTIAL) Via special sigframe
  Runtime library       None                    libdwarf/libunwind
```

### CONFIG_UNWINDER Options

```
CONFIG_UNWINDER_ORC        — ORC unwinder (default, recommended for production)
CONFIG_UNWINDER_FRAME_POINTER — Frame pointer unwinder (simple, reliable, slow)
CONFIG_UNWINDER_GUESS      — Heuristic "guess" unwinder (unreliable, last resort)
```

Frame pointer unwinder requires `CONFIG_FRAME_POINTER`, which adds `PUSH %rbp` / `MOV %rsp, %rbp`
to every function. This has ~1–3% performance overhead but makes stack walking trivial.

### Kernel Oops and Panic Stack Traces

When the kernel encounters a BUG, NULL dereference, or other fault:
1. The fault handler saves all registers into a `pt_regs` struct.
2. `dump_stack()` is called, which invokes the configured unwinder.
3. The unwinder walks from the current SP upward, printing each frame.
4. Frames are marked "reliable" or "unreliable":

```
  [<ffffffff81234567>] buggy_function+0x23/0x80   (reliable)
  [<ffffffff81345678>] caller_function+0x45/0x100  (reliable)
  [<ffffffffffffffff>] 0xffffffffffffffff           (unreliable - heuristic guess)
```

Unreliable frames appear when the unwinder had to guess (e.g., scanning the stack for return
addresses that look like kernel text addresses). They may be spurious.

### perf with DWARF and ORC

```bash
# Capture stack traces using DWARF (userspace perf reads .eh_frame)
perf record -g dwarf -F 99 ./program

# Capture using frame pointers (requires -fno-omit-frame-pointer compilation)
perf record -g fp -F 99 ./program

# For kernel stack traces, perf uses ORC via /proc/kcore or kallsyms
perf record -g --call-graph=dwarf,8192 -e cycles:k sleep 1
```

---

## 13. Cross-Language FFI and Unwinding Hazards

### The Fundamental Problem

Different languages have different unwinding ABIs. When an exception or panic crosses an FFI
boundary (C ↔ C++, Rust ↔ C, Go ↔ C), the results can be:
- Silent corruption of the other language's unwinding state.
- Segfault or crash.
- Memory leaks (destructors never called).
- Undefined behavior.

### C++ Exceptions Across Shared Library Boundaries

Two C++ shared libraries must use **compatible** personality functions and EH ABIs. On Linux, all
GCC-compiled C++ code uses the same Itanium ABI — exceptions can cross boundaries freely if:
- Both use the same `libstdc++.so` or `libc++.so`.
- `exception_class` in `_Unwind_Exception` matches.

Problems arise when:
- One library uses `libstdc++` and another uses `libc++`.
- One library was compiled with `-fno-exceptions` (may not have EH tables, breaking the chain).
- Windows MSVC vs MinGW/Clang on Windows (different ABI entirely).

### Rust ↔ C FFI and Panics

As noted in Section 8: unwinding a Rust panic through `extern "C"` frames is UB. The safe
pattern:

```rust
// Rust function called from C:
#[no_mangle]
pub extern "C" fn rust_callback(data: *mut Data) -> c_int {
    match std::panic::catch_unwind(|| {
        let data = unsafe { &mut *data };
        process(data)  // might panic
    }) {
        Ok(result) => result,
        Err(_) => -1,   // map panic to error code
    }
}
```

With `extern "C-unwind"` (Rust 1.73+):
```rust
// Allows C++ exceptions to propagate through Rust frames:
pub extern "C-unwind" fn callback_allowing_unwind() {
    // Rust panic here: well-defined behavior, unwinds through C-unwind frames
    do_something();
}
```

### Go CGO and C Panics

When Go code calls C via CGO, and the C code triggers a signal or a C++ exception:
- C++ exceptions are NOT handled by Go — they crash the process.
- C signals that are not SIGSEGV/SIGBUS/SIGPROF are passed to the Go runtime signal handler.
- A panic in a CGO callback (C calling Go) must be caught before returning to C.

```go
//export GoCallback
func GoCallback() {
    defer func() {
        if r := recover(); r != nil {
            // Must not let panic escape to C!
            log.Printf("recovered in CGO callback: %v", r)
        }
    }()
    doGoWork()
}
```

### Mixed C/C++ Projects: extern "C" Boundary

`extern "C"` functions in C++ do NOT propagate C++ exceptions through their boundary in the
standard-conforming sense. While the runtime may allow it physically, it is undefined behavior.
The correct pattern:

```cpp
// C++ function called from C:
extern "C" int do_work_safe(void *data) {
    try {
        do_cpp_work(static_cast<MyClass*>(data));
        return 0;
    } catch (const std::exception &e) {
        set_last_error(e.what());
        return -1;
    } catch (...) {
        set_last_error("unknown error");
        return -2;
    }
}
```

---

## 14. Security: Unwinding as Attack Surface

### SEH Overwrites (Windows — Historical but Instructive)

On 32-bit Windows, Structured Exception Handling (SEH) used a **linked list of exception handler
records on the stack**:

```
  Thread Information Block (TIB) → [SEH frame on stack] → [SEH frame] → → 0xFFFFFFFF

  SEH frame layout (on stack):
  +----------------------------------+
  | prev_handler (next SEH record)   |
  | handler_fn   (pointer to handler)|  ← attacker overwrites this
  +----------------------------------+
```

A stack buffer overflow could overwrite an SEH frame's `handler_fn`. When an exception was
triggered (even intentionally), the overwritten handler was called — giving code execution.
This was a major attack vector exploited widely in the 2000s.

**Mitigations**:
- **SafeSEH**: linker records all valid handler addresses; kernel checks at exception time.
- **SEHOP** (SEH Overwrite Protection): adds a sentinel at the end of the chain; validates chain
  integrity before dispatching.
- **64-bit Windows**: moved to table-based EH (similar to Itanium ABI); no chain-on-stack.

### Exception Handler Enumeration (Reconnaissance)

An attacker who can read process memory can enumerate exception handlers to:
- Locate executable code addresses (defeating ASLR if handlers have known offsets from modules).
- Identify the EH infrastructure and craft bypasses.

Mitigation: **ASLR for all modules**, including EH infrastructure libraries.

### Stack Canaries and Unwinding Interaction

Stack canaries (inserted by `-fstack-protector` or `-fstack-protector-strong`) place a random
value between local variables and the saved return address. Before function return (epilogue),
the canary is checked:

```
  +--------------------+
  |  local buffers     |
  +--------------------+
  |  CANARY VALUE      |  ← random, checked before return
  +--------------------+
  |  saved %rbp        |
  +--------------------+
  |  return address    |  ← attacker target
  +--------------------+
```

If a buffer overflow reaches the return address, it must overwrite the canary, which is detected.
However: canaries protect the **normal return path** but not all unwinding paths. An attacker who
can control exception dispatch (e.g., by corrupting `_Unwind_Exception` fields) may bypass canary
detection if they can trigger exception-based control flow. Canaries should be combined with CFI.

### Control Flow Integrity (CFI) and Unwinding

CFI is a mitigation that restricts indirect branches to valid targets. It interacts with unwinding
because:
- Personality functions are called via function pointer stored in FDE augmentation.
- Landing pads are target addresses set by the personality function.
- `_Unwind_SetIP` can redirect execution to any address.

**LLVM CFI** with `-fsanitize=cfi-icall` validates indirect calls against a type-based
allowlist. This can catch malicious personality function pointers if `.eh_frame` is corrupted.

**Intel CET (Control-flow Enforcement Technology)**:
- **Shadow Stack**: Stores return addresses in a separate read-only stack. `RET` checks that the
  popped address matches the shadow stack. Prevents ROP (return-oriented programming).
- **Indirect Branch Tracking (IBT)**: Requires `ENDBR64` instruction at valid indirect branch
  targets. Personality functions and landing pads must have `ENDBR64` at their entry.

The interaction: CET shadow stack works well with normal stack unwinding because `_Unwind_Resume`
does proper RET instructions. However, `longjmp` bypasses the shadow stack — glibc's
`longjmp_chk` was updated to synchronize the shadow stack when `longjmp` is used.

### DWARF Metadata Tampering

If an attacker can write to `.eh_frame` or `.gcc_except_table` (e.g., via a write primitive
beyond the initial buffer):
- They could corrupt CFA rules to make the unwinder compute wrong values.
- They could point LSDA call-site entries at attacker-controlled landing pads.
- They could replace the personality function pointer with an attacker-controlled function.

Mitigations: read-only memory mapping of ELF sections (`.eh_frame` is typically in a read-only
segment), write-XOR-execute protections, and pointer mangling for stored addresses.

### Kernel Stack Unwinding and Information Leaks

Kernel stack traces printed during oops/panics may leak kernel text addresses, which defeats
KASLR. Mitigations:
- `dmesg` rate limiting and filtering (`/proc/sys/kernel/dmesg_restrict`).
- `kptr_restrict`: controls whether kernel pointers appear in dmesg and `/proc/kallsyms`.
- `kernel.perf_event_paranoid`: controls perf access.

```bash
# Restrict kernel pointer exposure:
sysctl kernel.kptr_restrict=2
sysctl kernel.dmesg_restrict=1
```

---

## 15. Performance: Zero-Cost Exception Model Analysis

### The Core Claim

"Zero-cost" (or "zero-overhead") exceptions means: if no exception is thrown, the exception
handling infrastructure costs nothing in the happy path. There is no instruction executed to
"maintain" exception state during normal execution. This is opposed to the **setjmp/longjmp
model** where you pay cost on every function entry that might throw.

### Why It's True for the Happy Path

With table-based EH (DWARF/LSDA):
1. No instructions inserted in the function body for EH.
2. No register used for exception state.
3. EH metadata is in separate sections (`.eh_frame`, `.gcc_except_table`), not in `.text`.
4. The personality function and LSDA tables are cold data — loaded only when an exception occurs.

Result: on the happy path (no exception), the only cost is:
- **Binary size increase**: EH tables add ~10–30% to binary size.
- **Instruction cache effects**: EH tables don't affect I-cache (they're separate sections).
- **Data cache**: EH tables are only loaded when unwinding — no D-cache cost on happy path.

### The Unhappy Path Is Expensive

When an exception/panic/longjmp occurs:
- **C++**: Phase 1 searches all frames (personality function called per frame, LSDA parsed).
  This is O(n) in stack depth × O(m) in exception types × LSDA parse cost.
  Typical range: microseconds to milliseconds.
- **Rust panic**: similar to C++, plus `Box<dyn Any>` allocation.
- **Go panic**: linear in number of deferred functions; no DWARF involved; faster than C++.
- **Python**: O(1) exception table lookup per frame, but frame objects are heap objects.
- **Java**: O(1) bytecode table lookup per frame; JIT maintains exception metadata.
- **C setjmp/longjmp**: O(1) — register restore only. BUT no cleanup.

### Quantitative Comparison

```
  (Approximate relative costs, normalized to function call overhead = 1)

  Operation                          C       C++     Rust    Go      Python  Java
  ─────────────────────────────────  ─────   ─────   ─────   ─────   ──────  ─────
  Normal function call               1×      1×      1×      1×      100×    10×
  Function with try block (no throw) 1×      1×      1×      1×      100×    10×
  Throwing/panicking (1 frame)       N/A     ~1000×  ~1000×  ~100×   ~50×    ~500×
  Stack depth 10, exception thrown   N/A     ~5000×  ~5000×  ~200×   ~200×   ~1000×
  longjmp (non-cleanup)              ~2×     N/A     N/A     N/A     N/A     N/A

  Note: Python/Java base call overhead is higher, so exception overhead 
  relative to Python base is lower than the absolute numbers suggest.
```

### Why noexcept Matters in Hot Paths

```cpp
// Without noexcept: compiler generates landing pads and EH tables
// even if the function never throws
void process_packet(Packet &p) {
    parser.parse(p.data());  // might throw?
    route.forward(p);
}

// With noexcept: compiler elides all EH infrastructure for this function
// and may inline more aggressively
void process_packet(Packet &p) noexcept {
    parser.parse(p.data());  // if it throws: std::terminate
    route.forward(p);
}
```

For network packet processing in a tight loop, `noexcept` can measurably reduce code size and
improve branch prediction by eliminating landing pad branches.

### Rust abort vs unwind in Systems Code

For a Linux kernel module (Rust in kernel context):
- `panic = "abort"` is mandatory (no heap allocator for exception objects, no libunwind).
- All panics call the panic handler which calls `BUG()` or halts the CPU.

For a user-space high-performance network service:
- `panic = "unwind"` + `catch_unwind` at thread boundaries.
- Critical inner loops use `Result<T, E>` instead of panic to avoid any EH cost.

---

## 16. Debugging Tools and Techniques

### GDB: Stack Unwinding Awareness

```bash
# Basic stack trace
(gdb) bt
# Full trace with arguments
(gdb) bt full

# Walk up/down frames
(gdb) frame 3
(gdb) up
(gdb) down

# Show raw frame info (useful for broken frames)
(gdb) info frame
(gdb) info registers

# Force DWARF-based unwinding even if frame pointers missing
(gdb) set backtrace limit 50
```

GDB uses DWARF CFI internally for frame unwinding. If GDB shows `??` for function names or
corrupt backtraces, it usually means:
1. Debug symbols stripped (install `-dbg` packages or compile with `-g`).
2. Frame pointers omitted and DWARF CFI missing/corrupt.
3. Stack corruption (return address overwritten).

To compile with full debug + frame pointers:
```bash
gcc -g -fno-omit-frame-pointer -O1 source.c -o binary
```

### addr2line: Translating Addresses to Source

```bash
# Translate instruction address to source file + line number
addr2line -e binary -f 0x4012a3
# Output: function_name
#         /path/to/source.c:42

# With eu-addr2line (elfutils version, supports more DWARF features)
eu-addr2line -e binary --pretty-print 0x4012a3
```

### readelf / objdump for EH Inspection

```bash
# Show all DWARF sections
readelf -W -wF binary         # show .eh_frame decoded
readelf -W -wE binary         # show .gcc_except_table decoded (exception tables)
readelf -l binary             # show ELF segments (check PT_GNU_EH_FRAME)

# Show raw section content
objdump -s -j .eh_frame binary
objdump -d binary             # disassembly with function names

# Inspect ORC tables (Linux kernel binaries)
objdump -j .orc_unwind binary
```

### perf: Sampling with Stack Traces

```bash
# Record with DWARF-based call graphs (captures stack memory snapshot)
perf record --call-graph=dwarf -F 99 -p PID

# Record with frame pointer-based call graphs (fast, requires -fno-omit-frame-pointer)
perf record --call-graph=fp -F 999 ./program

# Record with LBR (Last Branch Record) — hardware, Intel only, limited depth ~16-32
perf record --call-graph=lbr -F 9999 ./program

# Generate flamegraph
perf script | stackcollapse-perf.pl | flamegraph.pl > flamegraph.svg

# Per-event stack trace
perf record -e 'sched:sched_switch' --call-graph=fp sleep 5
perf report --stdio
```

### Valgrind and Sanitizers

```bash
# Valgrind: shows stack at allocation/free time (libunwind-based)
valgrind --leak-check=full --track-origins=yes ./program

# AddressSanitizer: stack traces at error detection
gcc -fsanitize=address -g ./program
# ASAN uses fast stack unwinding for performance

# UBSanitizer: undefined behavior detection
gcc -fsanitize=undefined -g ./program
```

### Rust Backtrace

```bash
# Enable backtrace on panic
RUST_BACKTRACE=1 ./program
RUST_BACKTRACE=full ./program   # includes all frames including std internals

# Programmatic capture
use std::backtrace::Backtrace;
let bt = Backtrace::capture();  // requires RUST_BACKTRACE=1 or force:
let bt = Backtrace::force_capture();
println!("{}", bt);
```

### Go Stack Trace Tools

```bash
# Trigger stack dump for all goroutines (SIGQUIT on Linux)
kill -SIGQUIT <pid>

# Or programmatically:
import "runtime/debug"
debug.PrintStack()          # current goroutine
# or
buf := make([]byte, 1<<20)
n := runtime.Stack(buf, true)  // true = all goroutines
fmt.Fprintf(os.Stderr, "%s", buf[:n])

# Control stack trace verbosity on panic
GOTRACEBACK=none  ./program  # suppress goroutine dumps
GOTRACEBACK=all   ./program  # include runtime goroutines
GOTRACEBACK=system ./program  # include runtime frames in all goroutines
GOTRACEBACK=crash  ./program  # dump core + full traces
```

### Python Traceback Module

```python
import traceback
import sys

try:
    risky()
except Exception:
    # Print to stderr
    traceback.print_exc()
    
    # Get as string
    s = traceback.format_exc()
    
    # Inspect programmatically
    exc_type, exc_value, exc_tb = sys.exc_info()
    frames = traceback.extract_tb(exc_tb)
    for frame in frames:
        print(f"  {frame.filename}:{frame.lineno} in {frame.name}")
        print(f"    {frame.line}")
```

### Java Stack Trace Utilities

```java
// Print stack trace
try {
    risky();
} catch (Exception e) {
    e.printStackTrace();                 // to stderr
    
    // To string
    StringWriter sw = new StringWriter();
    e.printStackTrace(new PrintWriter(sw));
    String trace = sw.toString();
    
    // Programmatic access
    StackTraceElement[] elements = e.getStackTrace();
    for (StackTraceElement elem : elements) {
        System.out.println(elem.getClassName() + "." +
                           elem.getMethodName() + "(" +
                           elem.getFileName() + ":" +
                           elem.getLineNumber() + ")");
    }
}

// Thread dump (all threads)
Map<Thread, StackTraceElement[]> allTraces = Thread.getAllStackTraces();
```

---

## 17. Mental Models and Summary Tables

### The Fundamental Invariant

Regardless of language, **stack unwinding enforces this invariant**: when control leaves a scope
(normally or exceptionally), every resource acquired within that scope is deterministically
released. The mechanism differs; the contract is the same.

### Decision Tree: Which Unwinding Model for Which Situation?

```
  Am I writing systems/kernel code with no OS support (bare metal, kernel module)?
      YES → panic = "abort" (Rust), custom panic handler (no_std), goto cleanup (C)
      NO  → continue...
  
  Is this performance-critical hot path (packet processing, tight loop)?
      YES → Prefer error codes (C, Rust Result<T,E>), not exceptions/panics
      NO  → continue...
  
  Is resource cleanup critical and I need automatic guarantees?
      YES → C++/Rust RAII + unwinding, Go defer, Java try-with-resources
      NO  → C-style error codes + manual cleanup acceptable
  
  Do I need to catch panics/exceptions for isolation (e.g., plugin system)?
      YES → Rust catch_unwind, Go recover, C++ try/catch, Java try/catch
      NO  → Let the error propagate to top-level handler
  
  Is this cross-language code (FFI boundary)?
      YES → Catch ALL panics/exceptions before crossing the boundary
            Convert to error codes or tagged unions
      NO  → Use language-native mechanism
```

### Language-by-Language Summary

```
  Language    Mechanism            EH Model        Cleanup         Catching
  ──────────  ───────────────────  ──────────────  ──────────────  ─────────────────
  C           setjmp/longjmp       None (manual)   None (manual)   setjmp return val
  C++         throw/try/catch      2-phase DWARF   Destructors     catch blocks
  Rust        panic!/catch_unwind  2-phase DWARF   Drop trait      catch_unwind()
  Go          panic/defer/recover  Runtime-driven  defer funcs     recover()
  Python      raise/try/except     Interpreter     finally/__exit__  except blocks
  Java        throw/try/catch      JVM bytecodes   finally/close() catch blocks
```

### Stack Unwinding Metadata Location

```
  Language    Metadata Location             Format          Generated by
  ──────────  ────────────────────────────  ──────────────  ────────────────────
  C (libunw)  .eh_frame (ELF section)       DWARF CFI       Compiler
  C++         .eh_frame + .gcc_except_table DWARF+LSDA      Compiler
  Rust        .eh_frame + .gcc_except_table DWARF+LSDA      rustc
  Go          runtime data structures       Go runtime fmt  gc (Go compiler)
  Python      Per-method exception table    CPython format  Python compiler
  Java        Per-method exception table    .class format   javac
  Linux Kern  .orc_unwind + .orc_unwind_ip  ORC format      objtool (build-time)
```

### The Two-Phase Protocol at a Glance

```
  Two-phase unwinding (C++, Rust with unwind):
  
  Phase 1 (Search)          Phase 2 (Cleanup)
  ─────────────────────     ─────────────────────────────────
  Pure scan                 Side effects begin
  No destructors run        Destructors run (LIFO)
  No control transfer       Control transferred to landing pads
  Tests: "is there a        Runs: all cleanup in frames below
          handler at all?"  handler, then enters handler
  Cost: O(stack depth)      Cost: O(objects with destructors)
  
  If Phase 1 fails to       If a destructor throws in Phase 2:
  find a handler:           → std::terminate() immediately
  → std::terminate()        → (destructor must be noexcept)
```

### Key Properties That Distinguish Go's Model

```
  Go panic/recover vs C++ exception:
  
  1. No type matching: recover() catches ANY panic; you get the value and
     decide what to do. C++ catch is type-discriminated.
  
  2. Single-phase: Go runs deferred functions as it unwinds, combining
     Phase 1 and Phase 2 into one pass. It doesn't pre-scan for handlers.
  
  3. Goroutine-local: panic/recover is strictly within one goroutine.
     A panic in goroutine A does not affect goroutine B. In C++, an
     uncaught exception terminates the whole process.
  
  4. Goexit is unstoppable: runtime.Goexit() runs defers but cannot be
     recovered. There is no C++ equivalent (std::terminate doesn't run
     destructors; Goexit does run defers).
  
  5. defer stack vs EH tables: Go's cleanup is a runtime linked list of
     _defer structs. C++/Rust cleanup is encoded in static binary tables
     (.gcc_except_table) — no runtime overhead on the happy path.
```

### The Security Engineer's Checklist

```
  For every C/C++/Rust/Go service or kernel component:

  [ ] All resources use RAII or defer — no manual cleanup that could be skipped
  [ ] No panic/exception across FFI boundaries — catch before crossing
  [ ] noexcept on all destructors (C++)
  [ ] panic = "abort" for kernel and safety-critical code (Rust)
  [ ] recover() at all goroutine top-levels if partial failure is acceptable (Go)
  [ ] Stack canaries enabled (-fstack-protector-strong or stronger)
  [ ] ASLR + PIE for all binaries
  [ ] Shadow stack (Intel CET) enabled where available
  [ ] kptr_restrict + dmesg_restrict on production kernels
  [ ] No longjmp across functions with non-trivial objects (C++)
  [ ] All callbacks from C into Rust/C++ wrapped with catch_unwind/try-catch
  [ ] Exception safety level documented for all library APIs (C++)
```

### Questions That Sharpen Your Mental Model

1. A function is marked `noexcept` but calls `malloc`. If `malloc` throws `std::bad_alloc`, what
   happens? (Answer: `std::terminate`. `noexcept` catches the escape and terminates.)

2. In Rust, if `Drop::drop` panics while another panic is already unwinding, what happens?
   (Answer: `abort`. Rust detects double-panic and calls abort, analogous to C++ terminate.)

3. A Go program has goroutine A that panics without recover, and goroutine B doing I/O. What
   happens to goroutine B? (Answer: the whole program terminates — unrecovered panics are fatal.)

4. You write a C library function that is called from C++ code. Inside it, you call `longjmp` to
   a `setjmp` that is also inside the C library. C++ objects exist on frames between the
   `setjmp` and `longjmp`. What happens to those objects? (Answer: their destructors are NOT
   called — undefined behavior and resource leaks. This is the classic C/C++ interop hazard.)

5. In the Linux kernel, why can't you use DWARF-based unwinding during NMI handling?
   (Answer: NMI can interrupt any point including other NMI handlers or even the ORC unwinder
   itself. Stack state may be inconsistent. Kernel has hardened NMI unwinding with separate
   stacks and restricted ORC traversal for NMI context.)

6. A Rust library compiled with `panic = "unwind"` is used by a binary compiled with
   `panic = "abort"`. What happens when the library code panics? (Answer: the `abort` runtime
   is linked in; `panic = "abort"` wins for the final binary. The `unwind` setting in library
   Cargo.toml is advisory — the final binary's profile controls the panic runtime.)

---

## References and Further Reading

```
Specifications and Standards:
  Itanium C++ ABI: https://itanium-cxx-abi.github.io/cxx-abi/abi-eh.html
  DWARF Standard:  https://dwarfstd.org (DWARF5 for CFI)
  System V AMD64 ABI: https://gitlab.com/x86-psABIs/x86-64-ABI

Linux Kernel:
  ORC unwinder: Documentation/arch/x86/orc-unwinder.rst
  objtool:      tools/objtool/Documentation/objtool.txt
  Frame pointer unwinding: arch/x86/kernel/unwind_frame.c
  ORC unwinding:           arch/x86/kernel/unwind_orc.c

Source Code to Read:
  libunwind:     https://github.com/libunwind/libunwind
  libgcc EH:     gcc/unwind-dw2.c, gcc/unwind-dw2-fde.c
  LLVM libunwind: libunwind/src/UnwindCursor.hpp
  Rust panic:    library/std/src/panicking.rs
  Go panic:      src/runtime/panic.go
  Go defer:      src/runtime/defer.go
  CPython EH:    Python/ceval.c, Python/compile.c (exception table generation)
  JVM EH:        hotspot/src/share/vm/interpreter/interpreterRuntime.cpp

Tools:
  readelf (binutils), objdump, addr2line, eu-addr2line (elfutils)
  perf, valgrind, gdb, lldb
  pahole (DWARF structure inspection)
  dwarfdump (LLVM), dwarf-explore
```

---

*Document written for systems engineers working on Linux kernel networking, cloud security, and
user-space protocol implementation. Emphasizes production engineering concerns: correctness,
performance, security, and cross-language interoperability.*
