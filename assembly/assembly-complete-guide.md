# The Complete Assembly Language Guide (x86-64 / Linux)

> A ground-up, in-depth guide to assembly language, built around x86-64 Linux (NASM syntax), aimed at building a correct mental model of how a CPU actually executes your code. Written for someone comfortable in Rust/C but new to assembly.

---

## SYLLABUS

**Part I — Foundations**
1. What Assembly Actually Is (and why bother)
2. Number Systems: Binary, Hex, and Two's Complement
3. Computer Architecture 101 (CPU, Memory, Buses)
4. The x86-64 Register File
5. Memory: Layout, Segments, Stack, Heap

**Part II — Core Language**
6. Toolchain: Assembler, Linker, Object Files
7. Syntax: NASM vs AT&T, Instruction Anatomy
8. Addressing Modes
9. Data Movement Instructions
10. Arithmetic Instructions
11. Logical / Bitwise Instructions
12. The FLAGS Register and Comparisons
13. Control Flow: Jumps, Loops, Conditionals

**Part III — Structuring Programs**
14. The Stack in Depth
15. Procedures, Calling Conventions, the System V AMD64 ABI
16. System Calls on Linux
17. Arrays, Strings, and Structs in Memory

**Part IV — Advanced**
18. Floating Point: x87, SSE, AVX
19. Bit Tricks and SIMD
20. Interfacing with C/Rust (inline asm, FFI)
21. Security Mechanisms (stack canaries, ASLR, NX, ROP)
22. Debugging and Reverse Engineering Tools

**Part V — Practice**
23. Full Worked Programs (hello world → syscalls → functions → recursion)
24. Reading Compiler-Generated Assembly
25. Where to Go Next

---

# PART I — FOUNDATIONS

## 1. What Assembly Actually Is

Every instruction a CPU executes is a fixed-width (or variable-width, on x86) binary pattern called **machine code**. A CPU core has a piece of hardware — the **instruction decoder** — that reads these bit patterns out of memory and configures the rest of the chip (ALU, registers, memory unit) to do one specific, tiny operation: add two numbers, move a value, jump to a different instruction, etc.

Assembly language is a **human-readable, nearly 1:1 textual representation of machine code**. Each assembly line (mostly) corresponds to exactly one machine instruction. This is the fundamental difference from a language like Rust or C: those are *compiled down* through many transformation passes; assembly *is* almost the transformation's final output, just spelled with mnemonics (`mov`, `add`, `jmp`) instead of raw bits.

```
Your Rust code
      |
      v
   [rustc]  --->  LLVM IR  --->  optimization passes
      |
      v
  x86-64 ASSEMBLY   <---   YOU ARE HERE
      |
      v
   [assembler: nasm/as]
      |
      v
  Machine code (object file .o)
      |
      v
   [linker: ld]
      |
      v
  Executable binary (ELF on Linux)
      |
      v
  CPU fetches bytes, decodes, executes
```

Why learn it, given that no one writes production systems entirely in assembly anymore?

- **It removes the last layer of abstraction.** Once you know what `mov`, `call`, and `ret` actually do to registers and stack memory, concepts like "undefined behavior," "stack overflow," "calling convention," and "ABI" stop being folklore and become mechanical facts.
- **It explains performance.** Cache lines, branch prediction, instruction-level parallelism, SIMD — all of this is invisible until you're looking at the instructions themselves.
- **It's required for your kind of work.** eBPF bytecode, XDP hooks, packet parsing at line rate, gdb/objdump-level debugging, understanding what `#[no_std]` Rust actually compiles to — all of this sits directly on top of what this guide covers.

**Key vocabulary, precisely defined:**

| Term | Precise meaning |
|---|---|
| **Instruction Set Architecture (ISA)** | The contract between hardware and software: which instructions exist, what they do, register names, calling conventions are *not* part of the ISA (ABI is separate) |
| **x86-64 / AMD64** | The 64-bit ISA extending Intel's original x86 (which was 16-bit, then 32-bit "IA-32") |
| **Mnemonic** | The human-readable instruction name, e.g. `mov`, `add` |
| **Opcode** | The actual binary encoding of an instruction |
| **Operand** | The values an instruction acts on (registers, memory, immediates) |
| **Assembler** | Program that converts assembly text → machine code object file (NASM, GAS/`as`) |
| **Linker** | Program that combines object files + resolves symbols into a final executable (`ld`) |
| **ABI** | Application Binary Interface — the *convention* on top of the ISA: how functions pass arguments, which registers must be preserved, stack alignment rules, etc. |

---

## 2. Number Systems: Binary, Hex, and Two's Complement

You cannot read assembly without being fluent — not just "able to convert with a calculator," but fluent — in binary and hexadecimal, because register widths, masks, and offsets are constantly expressed in these bases.

### 2.1 Binary

Base 2. Each bit is a power of 2, MSB (most significant bit) on the left.

```
Bit position:  7   6   5   4   3   2   1   0
Value:        128  64  32  16   8   4   2   1

Byte 0b01001101:
   0   1   0   0   1   1   0   1
   |   |   |   |   |   |   |   |
   0 + 64+ 0 + 0 + 8 + 4 + 0 + 1 = 77
```

### 2.2 Hexadecimal

Base 16. Each hex digit maps to exactly 4 bits (a "nibble"), which is why hex is the natural shorthand for binary — unlike decimal, conversion is purely positional, no carrying required.

```
Hex digit:  0 1 2 3 4 5 6 7 8 9 A  B  C  D  E  F
Decimal:    0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15
Binary:  0000 0001 0010 0011 0100 0101 0110 0111
         1000 1001 1010 1011 1100 1101 1110 1111
```

A byte is always exactly 2 hex digits (`0x00`–`0xFF`). A 64-bit register is exactly 16 hex digits. This is why hex dumps and register views always look "aligned" — that alignment *is* the byte/nibble structure of memory.

```
0x4D  =  0100 1101  =  77 decimal
   ^one nibble each^
```

In NASM syntax, hex literals are written `0x1A` or `1Ah`. Binary literals: `0b0101` or `0101b`.

### 2.3 Two's Complement (signed integers)

CPUs do not have separate "add" and "subtract-aware-of-sign" circuits for integers — they use **two's complement** representation so that signed and unsigned addition use identical hardware.

**Rule:** to negate a number, invert all bits, then add 1.

```
    5  = 0000 0101
   ~5  = 1111 1010   (invert / one's complement)
   +1  = 1111 1011   =  -5   (two's complement)
```

The top bit is the sign bit: `0` = non-negative, `1` = negative — but it's not "magnitude + sign," it's a full reinterpretation of the bit pattern's value.

For an 8-bit signed byte, range is `-128` to `127`. Why asymmetric? Because `0000 0000` is claimed by zero, leaving 255 remaining patterns split as 128 negative / 127 positive.

```
1000 0000 = -128
1111 1111 = -1
0000 0000 = 0
0111 1111 = 127
```

**Why this matters in assembly:** the *same* `add` instruction is used whether you're treating operands as signed or unsigned — the bit pattern arithmetic is identical. What differs is which **flags** you check afterward (e.g. `jg`/`jl` for signed comparisons vs `ja`/`jb` for unsigned) and which widening/shrinking instructions you use (`movsx` sign-extends, `movzx` zero-extends). Getting signed vs. unsigned confused is one of the most common real bugs in low-level code — it's the same class of bug as a C `int` vs `unsigned int` comparison mistake, except now *you* choose the instruction, so the compiler can't save you.

### 2.4 Endianness

You already know this from XDP/network byte order work, but to state it precisely for the CPU side: x86-64 is **little-endian** — the least significant byte is stored at the lowest memory address.

```
32-bit value 0x12345678 stored at address 0x1000 on x86-64:

Address:   0x1000  0x1001  0x1002  0x1003
Byte:        78      56      34      12
             ^LSB                    ^MSB
```

This is exactly why your XDP/Rust code calls `.to_be()` before inserting IPs into a map: network protocols specify **big-endian** ("network byte order") wire format, but the x86-64 CPU registers/memory are little-endian, so raw register reads of a 4-byte IP field come out byte-reversed relative to how the RFC defines the field.

---

## 3. Computer Architecture 101

### 3.1 The Von Neumann Model

Nearly every general-purpose CPU (x86-64 included) implements a variant of the **von Neumann architecture**: a single unified memory holds both instructions and data, and the CPU repeatedly fetches, decodes, and executes instructions from memory.

```
        +------------------------------------------+
        |                   CPU                     |
        |   +--------+   +-----+   +-------------+  |
        |   | Control|-->| ALU |-->|  Registers  |  |
        |   |  Unit  |   +-----+   +-------------+  |
        |   +--------+                               |
        |        ^                                   |
        |        | fetch/decode/execute loop          |
        +--------|-----------------------------------+
                 |
                 v  (address/data/control bus)
        +------------------------------------------+
        |              Main Memory (RAM)            |
        |   [instructions and data, unified space]   |
        +------------------------------------------+
                 ^
                 |
        +------------------------------------------+
        |         I/O devices, disk, NIC, etc.       |
        +------------------------------------------+
```

### 3.2 The Fetch-Decode-Execute Cycle

This loop is the literal, physical thing happening billions of times per second:

```
   +-----------+     +-----------+     +-----------+     +----------+
   |   FETCH   | --> |  DECODE   | --> |  EXECUTE  | --> | WRITEBACK|
   +-----------+     +-----------+     +-----------+     +----------+
   read instr.        figure out       ALU does the       result stored
   at address in       what it means,   operation,         in register
   RIP register,       fetch operand    memory              or memory
   RIP += len          registers        read/write
        ^                                                       |
        +-------------------------------------------------------+
                        (repeat forever)
```

- **RIP** (Instruction Pointer, 64-bit) holds the address of the *next* instruction to execute. Every instruction implicitly advances it (or, for jumps/calls, explicitly overwrites it — that *is* what a jump instruction is: an assignment to RIP).
- Real modern CPUs (your actual laptop/server chip) pipeline, reorder, speculate, and execute many instructions "in flight" simultaneously — but the *architectural model* (the one assembly programs against) is this simple sequential loop. The complexity underneath is invisible to the ISA; it's a hardware implementation detail whose entire purpose is to make the sequential model *appear* true while going faster.

### 3.3 Cache Hierarchy (why it matters for perf)

```
   CPU core
     |
   [L1 cache]   ~32KB,   ~4 cycles latency   (per-core, split I-cache/D-cache)
     |
   [L2 cache]   ~256KB-1MB, ~12 cycles       (per-core or per-cluster)
     |
   [L3 cache]   several MB, ~40 cycles       (shared across cores)
     |
   [Main RAM]   GBs, ~200+ cycles            (shared, off-chip)
```

Every memory instruction in assembly (`mov rax, [rbx]`) is, physically, a request that travels down this hierarchy. Sequential memory access patterns (arrays) are fast because the hardware prefetcher predicts them and cache lines (typically 64 bytes) are pulled in bulk. Random pointer-chasing (linked lists) is slow because each access is a fresh, unpredictable trip potentially all the way to RAM. This single fact explains a huge fraction of "why is my code slow" in systems programming.

---

## 4. The x86-64 Register File

Registers are storage locations *inside* the CPU itself — not memory. They're the fastest storage that exists (accessed in the same cycle as the instruction using them), and there are very few of them, which is why register allocation is such a central concern in both compilers and hand-written assembly.

### 4.1 General Purpose Registers (GPRs)

x86-64 has 16 general-purpose 64-bit registers. Due to backward compatibility going back to the 8086 (16-bit), each register can be addressed at 4 different widths:

```
   63                              31          15      7       0
   +-------------------------------------------------------------+
   |                             RAX (64-bit)                     |
   +-------------------------------------------------------------+
                                   |            |       |
                                 EAX (32-bit)  AX (16-bit)
                                                |       |
                                              AH (8-bit) AL (8-bit)
```

- `RAX` — full 64 bits
- `EAX` — low 32 bits of RAX
- `AX`  — low 16 bits of RAX
- `AH`  — bits 8–15 (high byte of AX) — *only exists for the original 8 registers*
- `AL`  — bits 0–7 (low byte of AX)

**Important gotcha:** writing to a 32-bit sub-register (e.g. `mov eax, 5`) automatically **zero-extends** and clears the upper 32 bits of the full 64-bit register. Writing to a 16-bit or 8-bit sub-register does **not** clear the upper bits — it leaves them untouched. This asymmetry is a real, frequently-cited x86-64 wart, and forgetting it causes real bugs (a stale upper 48 bits silently surviving a `mov al, ...`).

**The 16 GPRs:**

```
+--------+--------+--------+--------+---------------------------------+
|  64-bit|  32-bit|  16-bit|  8-bit | Conventional / ABI role          |
+--------+--------+--------+--------+---------------------------------+
|  RAX   |  EAX   |  AX    |  AL    | return value; "accumulator"      |
|  RBX   |  EBX   |  BX    |  BL    | callee-saved general purpose     |
|  RCX   |  ECX   |  CX    |  CL    | 4th integer arg; loop counter    |
|  RDX   |  EDX   |  DX    |  DL    | 3rd integer arg                  |
|  RSI   |  ESI   |  SI    |  SIL   | 2nd integer arg; "source index"  |
|  RDI   |  EDI   |  DI    |  DIL   | 1st integer arg; "dest index"    |
|  RBP   |  EBP   |  BP    |  BPL   | frame/base pointer (optional)    |
|  RSP   |  ESP   |  SP    |  SPL   | stack pointer (hardware-special) |
|  R8    |  R8D   |  R8W   |  R8B   | 5th integer arg                  |
|  R9    |  R9D   |  R9W   |  R9B   | 6th integer arg                  |
|  R10   |  R10D  |  R10W  |  R10B  | caller-saved scratch / syscall   |
|  R11   |  R11D  |  R11W  |  R11B  | caller-saved scratch             |
|  R12   |  R12D  |  R12W  |  R12B  | callee-saved                     |
|  R13   |  R13D  |  R13W  |  R13B  | callee-saved                     |
|  R14   |  R14D  |  R14W  |  R14B  | callee-saved                     |
|  R15   |  R15D  |  R15W  |  R15B  | callee-saved                     |
+--------+--------+--------+--------+---------------------------------+
```

None of these "roles" (return value, argument N, callee-saved) are enforced by hardware — a `mov` instruction doesn't care what you use `RCX` for. The roles come entirely from the **System V AMD64 ABI**, a *software convention* that GCC, Clang, rustc, and the Linux kernel all agree to follow so that separately-compiled code can call into each other correctly. We cover this fully in §15.

### 4.2 Special-purpose registers

- **RIP** — instruction pointer. Cannot be directly written with `mov`; only changed via control-flow instructions (`jmp`, `call`, `ret`, conditional jumps). x86-64 added **RIP-relative addressing**, letting instructions reference memory as an offset from RIP — critical for position-independent code (PIE/ASLR).
- **RFLAGS** — a 64-bit register where individual bits ("flags") record facts about the most recent arithmetic/logical operation. Covered in depth in §12.
- **RSP** — the stack pointer. Technically a GPR, but hardware treats it specially: `push`, `pop`, `call`, `ret` all implicitly read/modify it. Using RSP for general arithmetic is legal but almost never done — corrupt it and your next `ret` jumps to garbage.

### 4.3 SIMD / Floating-point registers

- **XMM0–XMM15** — 128-bit registers for SSE, used both for scalar floating point (the standard way floats/doubles are handled in x86-64, replacing the old x87 stack) and for 128-bit-wide parallel (SIMD) integer/float operations.
- **YMM0–YMM15** — 256-bit AVX registers; the lower 128 bits alias XMM.
- **ZMM0–ZMM31** — 512-bit AVX-512 registers (server/newer CPUs only).

```
   ZMM0  [512 bits ......................................................]
   YMM0  [256 bits ..........................]  (low half of ZMM0)
   XMM0  [128 bits ..........]                   (low half of YMM0)
```

Covered in depth in §18–19.

---

## 5. Memory: Layout, Segments, Stack, Heap

A running Linux process's virtual address space is divided into regions ("segments") by convention and by the ELF loader / dynamic linker.

```
High addresses
0x7fffffffffff  +---------------------------+
                 |  Kernel space (not mapped |
                 |  into user address space  |
                 |  view; separate range)    |
                 +---------------------------+
                 |         Stack             |  <- grows DOWN
                 |            |               |
                 |            v               |
                 +---------------------------+
                 |     (unmapped guard gap)   |
                 +---------------------------+
                 |    Memory-mapped region    |  shared libs, mmap()
                 |    (libc.so, ld.so, etc.)  |
                 +---------------------------+
                 |            ^               |
                 |            |               |
                 |          Heap             |  <- grows UP
                 +---------------------------+
                 |   .bss  (uninit. globals)  |
                 +---------------------------+
                 |   .data (init. globals)    |
                 +---------------------------+
                 |   .rodata (string consts)  |
                 +---------------------------+
                 |   .text  (your code)       |
0x000000400000   +---------------------------+
Low addresses     (or ASLR-randomized base with PIE)
```

- **`.text`** — the actual machine instructions. Marked read + execute, *not* writable (this is an important security property — self-modifying code / injected shellcode into `.text` is blocked).
- **`.rodata`** — read-only data: string literals, constant tables.
- **`.data`** — initialized global/static variables.
- **`.bss`** — uninitialized (zero-initialized) global/static variables; doesn't take up file space, only runtime memory (the loader just zero-fills it).
- **Heap** — dynamic memory (`malloc`/`Box::new`), managed by an allocator, grows toward higher addresses.
- **Stack** — per-thread, grows toward *lower* addresses, holds return addresses, saved registers, local variables. Covered fully in §14.

You can literally see this layout for a running process:

```bash
cat /proc/<pid>/maps
```

which will show you these exact regions with their real addresses — a direct, verifiable link between this diagram and a live system, which is worth doing once so the abstraction stops being abstract.

---

# PART II — CORE LANGUAGE

## 6. Toolchain: Assembler, Linker, Object Files

We'll use **NASM** (Netwide Assembler) because its syntax is more readable than GAS/AT&T and it's what most standalone-assembly tutorials and this guide use. (§7.4 shows AT&T too, since that's what `objdump`/GCC output by default and you'll need to read it.)

```
   hello.asm  --[nasm -f elf64 hello.asm]-->  hello.o (object file, ELF format)
        |
        v
   hello.o  --[ld hello.o -o hello]-->  hello (final ELF executable)
        |
        v
   ./hello  --[kernel execve()]-->  process running
```

- `nasm -f elf64 hello.asm -o hello.o` — assembles into a 64-bit ELF **object file**. This file contains machine code plus a *symbol table* (names like function labels) but addresses aren't finalized yet — external references are just placeholders.
- `ld hello.o -o hello` — the **linker** resolves all symbol references across object files/libraries, lays out final memory addresses, and produces a runnable ELF executable.
- Later, when linking against libc (`gcc hello.o -o hello` or `ld -lc ...`), the linker also stitches in dynamic-linking metadata so the loader can find `libc.so` at runtime.

Useful inspection tools (you'll use these constantly):

| Tool | Purpose |
|---|---|
| `objdump -d file` | Disassemble: show the assembly for a compiled binary/object |
| `readelf -h file` | Show ELF header (entry point, architecture, etc.) |
| `nm file` | List symbols |
| `strace ./file` | Trace every syscall the program makes |
| `gdb ./file` | Interactive debugger — step instruction by instruction |
| `xxd file` / `hexdump -C file` | Raw hex dump |

---

## 7. Syntax: NASM vs AT&T, Instruction Anatomy

### 7.1 Anatomy of an instruction

```
        label:      mnemonic   destination, source      ; comment
              |         |            |          |
        optional    operation    operand 1   operand 2
                      name       (usually     (usually
                                  written to)  read from)

Example:
        loop_start:  add        rax, rbx              ; rax = rax + rbx
```

NASM's convention: **`mnemonic dest, src`** — result goes in the *first* operand. This is "Intel syntax."

### 7.2 A minimal, complete, real program

This is a genuine, runnable Linux x86-64 program with **zero libc dependency** — it talks directly to the kernel via syscalls. This is deliberately chosen as your first program because it forces you to see exactly what a "hello world" costs with nothing hidden.

```nasm
; hello.asm — prints "Hello, assembly!\n" and exits, using raw Linux syscalls
section .rodata
    msg     db "Hello, assembly!", 10   ; 10 = '\n'; db = "define byte"
    msg_len equ $ - msg                 ; $ = current address; length = here - start

section .text
    global _start                       ; entry point symbol, referenced by the linker

_start:
    ; write(fd=1, buf=msg, count=msg_len)
    mov     rax, 1          ; syscall number for write (see syscall table, §16)
    mov     rdi, 1          ; arg1: fd = 1 (stdout)
    mov     rsi, msg        ; arg2: pointer to buffer
    mov     rdx, msg_len    ; arg3: number of bytes
    syscall                 ; trap into the kernel

    ; exit(status=0)
    mov     rax, 60         ; syscall number for exit
    mov     rdi, 0          ; arg1: exit status
    syscall
```

Build and run:

```bash
nasm -f elf64 hello.asm -o hello.o
ld hello.o -o hello
./hello
# Hello, assembly!
```

Walk through *exactly* what happened:
1. `section .rodata` / `section .text` are **directives** (assembler instructions, not CPU instructions) that tell NASM which ELF segment subsequent bytes belong to.
2. `db "Hello, assembly!", 10` emits those literal bytes into `.rodata` at assembly time.
3. `msg_len equ $ - msg` is computed *at assemble time* — `equ` defines a constant, `$` means "the current address," so this is literally "current position minus the position where `msg` started," i.e. the string's byte length. No runtime cost.
4. `global _start` exports the symbol so the linker knows where execution begins (equivalent to `main`, but lower-level — `_start` runs before libc's runtime init would normally run, which we've skipped entirely).
5. Each `mov` loads one syscall argument into the register the **Linux syscall ABI** (not the C ABI — they differ!) mandates: `rax`=syscall number, then `rdi, rsi, rdx, r10, r8, r9` for args 1–6 in order.
6. `syscall` is a single instruction that switches the CPU to kernel (ring 0) mode, the kernel's syscall handler runs using `rax` to dispatch, and control returns to your next instruction with `rax` overwritten with the return value.

### 7.3 Directives you'll see constantly

| Directive | Meaning |
|---|---|
| `db`/`dw`/`dd`/`dq` | Define byte/word(2B)/doubleword(4B)/quadword(8B) static data |
| `resb`/`resw`/`resd`/`resq` | Reserve (uninitialized) bytes — goes in `.bss` |
| `equ` | Compile-time constant |
| `section .text/.data/.bss/.rodata` | Which ELF segment |
| `global sym` | Export a symbol (visible to linker/other files) |
| `extern sym` | Declare a symbol defined elsewhere (e.g. libc's `printf`) |
| `%define`, `%macro` | Preprocessor macros (like C's `#define`) |

### 7.4 NASM (Intel) vs GAS (AT&T) — you must recognize both

`objdump`, GDB, and GCC's `-S` output default to AT&T syntax on Linux, so even though we write NASM, you need to *read* AT&T fluently.

```
Intel (NASM):      mov     rax, rbx          ; rax = rbx   (dest, src)
AT&T (GAS):        mov     %rbx, %rax        ; rax = rbx   (src, dest)  <- operand order REVERSED

Intel:              mov     rax, [rbx+8]
AT&T:                mov     8(%rbx), %rax

Intel:              mov     eax, 5
AT&T:                movl    $5, %eax          ; immediates prefixed with $, size suffix on mnemonic
```

Rules for AT&T: registers prefixed `%`, immediates prefixed `$`, operand order is **src, dest** (opposite of Intel), memory access uses `offset(base, index, scale)`, and mnemonics often carry a size suffix (`b`=byte, `w`=word, `l`=32-bit long, `q`=quadword).

You will hit this constantly running `objdump -d -M intel ./binary` (the `-M intel` flag forces Intel syntax output, which is worth memorizing so you don't have to mentally flip operand order every time).

---

## 8. Addressing Modes

"Addressing mode" = *how an operand's actual value is determined*. This is one of the most important conceptual buckets in assembly, because it's the same handful of patterns reused by nearly every instruction.

```
1. Register direct:     mov rax, rbx           ; value = contents of rbx
2. Immediate:            mov rax, 42            ; value = literal constant 42
3. Memory direct:        mov rax, [0x4000]      ; value = memory at fixed address 0x4000
4. Register indirect:    mov rax, [rbx]         ; value = memory at address held in rbx
5. Base + displacement:  mov rax, [rbx + 8]     ; value = memory at (rbx + 8)
6. Base + index:         mov rax, [rbx + rcx]   ; value = memory at (rbx + rcx)
7. Base+index+scale+disp:mov rax, [rbx + rcx*8 + 16]
                                                 ; value = memory at (rbx + rcx*8 + 16)
                                                 ; <- this is EXACTLY how array[i] compiles
```

Mode 7 deserves its own explanation because it is the single addressing mode that makes arrays cheap. If `rbx` holds the base address of an `i64` array and `rcx` holds an index `i`, then `[rbx + rcx*8 + 0]` computes the address of `array[i]` in one instruction, with the `*8` (scale factor: 1, 2, 4, or 8 only) accounting for `i64` being 8 bytes wide — no separate multiply instruction needed; the CPU's address-generation unit does the multiply-add as part of decoding the memory operand, for free.

**Square brackets `[...]` mean "dereference"** — "go to this address in memory and use what's there," exactly like `*ptr` in C. Without brackets, you're using the address/register/value itself.

```nasm
mov rax, rbx      ; rax = rbx                    (copy the register value)
mov rax, [rbx]     ; rax = *(u64*)rbx              (dereference: load 8 bytes from that address)
lea rax, [rbx+8]   ; rax = rbx + 8                 (compute an ADDRESS, don't dereference — see below)
```

### `lea` — Load Effective Address

`lea` is one of the most misunderstood instructions for beginners. It uses memory-operand *syntax* but performs **no memory access at all** — it just computes the address arithmetic and puts the resulting number in the destination register. Because x86's address-computation hardware can do `base + index*scale + disp` in one cycle, `lea` is very commonly (ab)used as a fast general-purpose "multiply-and-add" instruction even when no actual memory/pointer is involved:

```nasm
lea rax, [rbx + rcx*4]   ; rax = rbx + rcx*4  — pure arithmetic, nothing read from memory
lea rax, [rbx + 8]       ; rax = rbx + 8      — equivalent to "add rax, rbx, 8" if that existed
```

---

## 9. Data Movement Instructions

```nasm
mov   dst, src        ; dst = src   (the single most common instruction)
movzx dst, src        ; move with zero-extend: widen an unsigned smaller value, fill upper bits with 0
movsx dst, src        ; move with sign-extend: widen a signed smaller value, replicate the sign bit
lea   dst, [expr]      ; dst = computed address (see §8)
push  src              ; push onto the stack (see §14)
pop   dst              ; pop off the stack
xchg  a, b             ; swap two operands' contents
cmov<cc> dst, src      ; conditional move — move only if a flag condition holds (branchless!)
```

**`movzx`/`movsx` example**, showing exactly why the distinction matters:

```nasm
mov  al, 0xFF        ; al = 11111111  (this is -1 as signed i8, or 255 as unsigned u8)

movzx eax, al         ; eax = 0x000000FF = 255   (zero-extend: treats al as UNSIGNED)
movsx eax, al         ; eax = 0xFFFFFFFF = -1    (sign-extend: treats al as SIGNED, replicates bit 7)
```

If you use the wrong one, you get a silently wrong value — no crash, no warning, just a bug that surfaces later as an "impossible" off-by-huge-number result. This is precisely the hardware-level version of a C `(int)(int8_t)x` vs `(int)(uint8_t)x` cast mismatch.

**`cmov` — branchless conditional move**, a genuinely important tool for performance-sensitive code (this is the kind of thing hand-tuned parsers/packet processing use to avoid branch mispredictions):

```nasm
; C equivalent: max = (a > b) ? a : b;
mov   eax, [a]
mov   ebx, [b]
cmp   eax, ebx
cmovle eax, ebx        ; if eax <= ebx (from the cmp flags), eax = ebx
; eax now holds max(a,b), with ZERO branches taken
```

---

## 10. Arithmetic Instructions

```nasm
add   dst, src     ; dst = dst + src
sub   dst, src     ; dst = dst - src
inc   dst          ; dst = dst + 1
dec   dst          ; dst = dst - 1
neg   dst          ; dst = -dst  (two's complement negate)

; Multiplication
imul  dst, src           ; signed multiply, dst = dst * src (two-operand form)
imul  dst, src, imm       ; dst = src * immediate
mul   src                 ; UNSIGNED multiply: rdx:rax = rax * src (128-bit result!)

; Division
idiv  src            ; SIGNED divide: rax = rdx:rax / src, rdx = remainder
div   src             ; UNSIGNED divide: same, unsigned interpretation
```

**Critical gotcha with `mul`/`div`**: these are *not* free-form two-operand instructions like `add`. They implicitly use the `RDX:RAX` register pair as a combined 128-bit accumulator. `mul rbx` means "multiply RAX by RBX, put the low 64 bits of the 128-bit result in RAX and the high 64 bits in RDX" — this is why a 64×64 multiply can produce a full 128-bit result without overflow, unlike `add`, which just wraps. Similarly `idiv rbx` divides the 128-bit value in `RDX:RAX` by RBX, producing quotient in RAX and remainder in RDX. Before a division you almost always need `cqo` (sign-extend RAX into RDX:RAX) or `xor rdx, rdx` (zero it, for unsigned) to set up RDX correctly — forgetting this is a classic beginner division bug (division either traps with SIGFPE or silently gives a wrong result).

```nasm
; Compute (a / b) with a, b signed 64-bit values in rax, rbx respectively
cqo             ; sign-extend rax into rdx:rax  (rdx = 0 or -1 depending on rax's sign)
idiv rbx        ; rax = quotient, rdx = remainder
```

---

## 11. Logical / Bitwise Instructions

```nasm
and   dst, src    ; bitwise AND
or    dst, src    ; bitwise OR
xor   dst, src    ; bitwise XOR
not   dst         ; bitwise NOT (one's complement, flips every bit)
shl   dst, cnt     ; shift left  (logical) — dst <<= cnt, zero-fill from right
shr   dst, cnt     ; shift right (logical) — dst >>= cnt, zero-fill from left (UNSIGNED semantics)
sar   dst, cnt     ; shift right (arithmetic) — replicates the sign bit (SIGNED semantics)
rol/ror dst, cnt   ; rotate left/right
```

Common idioms worth memorizing because you'll see them constantly in real (including compiler-generated) code:

```nasm
xor rax, rax     ; rax = 0.  Faster & smaller-encoded than "mov rax, 0" — extremely common idiom.
test rax, rax     ; sets flags as if "and rax, rax" was done, but discards result — used purely to
                   ; check "is rax zero / what's its sign" before a conditional jump, without an
                   ; actual mov or destructive op.
and  rax, 1        ; isolate the lowest bit — classic "is this even/odd" check
shl  rax, 3        ; rax *= 8  — shifting left by N multiplies by 2^N (fast, no real multiply)
shr  rax, 1        ; rax /= 2 for unsigned values (careful: NOT equivalent to signed division!)
```

**`shr` vs `sar`, precisely:** for a negative signed number, `shr` fills the vacated high bits with `0`, which corrupts the sign — it treats the bit pattern as unsigned. `sar` fills with copies of the *original* sign bit, correctly preserving negativity (arithmetically equivalent to floor-division by a power of 2 for negative numbers). Using `shr` where you meant `sar` is a real, subtle bug class.

```
   -8 as i8:  1111 1000

   shr by 1:  0111 1100  =  124   <- WRONG if you meant "divide -8 by 2"
   sar by 1:  1111 1100  =  -4    <- correct signed division by 2
```

---

## 12. The FLAGS Register and Comparisons

**RFLAGS** is a 64-bit register where individual bits are set/cleared automatically as a *side effect* of most arithmetic/logical instructions. Conditional jumps and `cmov` read these bits — they don't recompute anything, they just inspect state that's already there from the last relevant instruction.

The flags that matter for everyday code:

```
+------+---------------------------------------------------------------+
| Flag | Meaning                                                        |
+------+---------------------------------------------------------------+
| ZF   | Zero Flag — set if the result was exactly 0                    |
| SF   | Sign Flag — set if the result's top bit is 1 (negative, signed)|
| CF   | Carry Flag — set if an unsigned add overflowed / unsigned      |
|      | subtract needed a borrow                                       |
| OF   | Overflow Flag — set if a SIGNED add/sub overflowed the         |
|      | representable range                                            |
+------+---------------------------------------------------------------+
```

`cmp a, b` internally does `a - b` and throws away the numeric result, keeping only the flags — this is exactly analogous to `test` doing a discarded `and`. `cmp` is how every comparison in every `if`/loop in every compiled language ultimately becomes assembly.

```nasm
cmp rax, rbx      ; computes rax - rbx internally, sets ZF/SF/CF/OF, discards the subtraction result
je  equal_case     ; jump if ZF set        (rax == rbx)
jne not_equal       ; jump if ZF clear      (rax != rbx)
jg  greater_signed   ; jump if SF==OF and ZF==0   (rax > rbx, SIGNED)
jl  less_signed       ; jump if SF!=OF             (rax < rbx, SIGNED)
ja  above_unsigned    ; jump if CF==0 and ZF==0    (rax > rbx, UNSIGNED)
jb  below_unsigned     ; jump if CF==1               (rax < rbx, UNSIGNED)
```

This split — `jg`/`jl` (signed) vs `ja`/`jb` (unsigned) — exists precisely because "greater than" means something different depending on how you interpret the same bit pattern (§2.3). `cmp` doesn't know or care whether your values are signed; it always sets the same flags from the same subtraction. **You** choose the signed or unsigned conditional jump depending on how your program intends to interpret the numbers. This is the single most common source of real security bugs in C (e.g. an unsigned length check that silently becomes "always true" because a negative value wrapped to a huge unsigned number) — and now you can see exactly, mechanically, why.

---

## 13. Control Flow: Jumps, Loops, Conditionals

There is no `if`, no `while`, no `for` in machine code — only unconditional jumps (`jmp`), conditional jumps (§12's `je`/`jg`/etc.), and `call`/`ret`. Every higher-level control structure you've ever used compiles down to some arrangement of these plus labels.

### `if` / `else`

```c
// C
if (a > b) { x = 1; } else { x = 2; }
```
```nasm
; assembly
    cmp   rax, rbx        ; compare a, b
    jg    .if_true
    mov   rcx, 2           ; else branch
    jmp   .done
.if_true:
    mov   rcx, 1
.done:
```

### `while` loop

```c
// C
while (i < n) { sum += i; i++; }
```
```nasm
    ; rax=i, rbx=n, rcx=sum, all pre-initialized
.while_cond:
    cmp   rax, rbx
    jge   .while_end       ; if i >= n, exit loop
    add   rcx, rax          ; sum += i
    inc   rax                ; i++
    jmp   .while_cond
.while_end:
```

### `for` loop with `loop` instruction

x86 has a dedicated `loop` instruction: it decrements `rcx` and jumps if `rcx != 0` — a compact but nowadays rarely-used idiom (modern compilers prefer explicit `dec`+`jnz` because `loop` is actually *slower* on most modern microarchitectures — a good example of "the ISA offers it, but the ABI/compiler convention has moved on"):

```nasm
    mov   rcx, 10        ; loop counter = 10
.loop_top:
    ; ... loop body ...
    loop  .loop_top        ; rcx--; if rcx != 0, jump to .loop_top
```

### Jump table (how `switch` compiles for dense cases)

```nasm
; C: switch(x) { case 0: ...; case 1: ...; case 2: ...; }
    cmp   rax, 2
    ja    .default_case          ; bounds check first!
    lea   rdx, [rel jump_table]
    jmp   qword [rdx + rax*8]     ; index into table of addresses, jump there

section .rodata
jump_table:
    dq .case0, .case1, .case2
```

This is exactly why a C `switch` with dense, small-range integer cases compiles to O(1) code instead of a chain of comparisons — it becomes a table lookup + indirect jump, using the exact same base+index*scale addressing mode from §8.

---

# PART III — STRUCTURING PROGRAMS

## 14. The Stack in Depth

The stack is a region of memory (§5) used for function call bookkeeping, growing **downward** (toward lower addresses) as things are pushed.

```
Higher addresses
      +------------------+
      |  ... caller's     |
      |  earlier frame    |
      +------------------+  <- RBP of caller
      |  return address   |  <- pushed automatically by `call`
      +------------------+
      |  saved old RBP    |  <- pushed by callee's prologue (push rbp)
      +------------------+  <- RBP of current function (frame base)
      |  local variable 1 |
      |  local variable 2 |
      |       ...         |
      +------------------+  <- RSP (top of stack, grows toward lower addrs)
Lower addresses
```

`push`/`pop` mechanics, spelled out fully (this is *exactly* what happens, no hand-waving):

```nasm
push rax      ; equivalent to:  sub rsp, 8  \n  mov [rsp], rax
pop  rax      ; equivalent to:  mov rax, [rsp]  \n  add rsp, 8
```

`push` decrements RSP by 8 (one quadword) *first*, then writes the value at the new RSP. `pop` reads from RSP, *then* increments RSP by 8. This is why the stack "grows down" — pushing moves RSP toward address 0, and why a **stack overflow** is, physically, RSP running into the bottom of the mapped stack region (or a guard page, which deliberately triggers a segfault rather than silently corrupting adjacent memory).

### Stack frame prologue/epilogue

Every non-leaf function typically does this dance:

```nasm
my_function:
    push  rbp          ; save caller's frame pointer
    mov   rbp, rsp      ; establish our own frame pointer (RBP now = "frame base")
    sub   rsp, 32        ; allocate 32 bytes of local variable space

    ; ... function body, locals accessed as [rbp-8], [rbp-16], etc ...

    mov   rsp, rbp        ; deallocate locals (equivalent to "add rsp, 32")
    pop   rbp              ; restore caller's frame pointer
    ret                     ; pop return address into RIP, jump there
```

`ret` is precisely `pop rip` (not literally legal syntax since you can't `pop` into RIP directly, but that's exactly its semantics) — it pops the top of the stack into the instruction pointer. This is *why* stack-smashing (overflowing a buffer to overwrite the saved return address) is a viable attack: the return address is just data sitting in writable memory, and `ret` blindly trusts whatever 8 bytes are currently on top of the stack.

---

## 15. Procedures, Calling Conventions, the System V AMD64 ABI

The **System V AMD64 ABI** is the calling convention used by Linux, macOS, and BSDs (Windows x64 uses a different one — different register assignments and stack rules; worth knowing if you ever touch cross-platform FFI). This is not part of the x86-64 *ISA* — it's a software agreement, but every C, Rust, and Go compiler on Linux obeys it because interoperability requires it.

### 15.1 Argument passing

```
Integer/pointer arguments 1-6, IN ORDER:   RDI, RSI, RDX, RCX, R8, R9
7th and beyond:                             pushed onto the stack
Floating point arguments:                   XMM0-XMM7 (separate counting from int args)
Return value (integer/pointer):             RAX  (RDX:RAX if the value needs 128 bits)
Return value (float/double):                XMM0
```

### 15.2 Caller-saved vs. callee-saved registers

```
+--------------------------+--------------------------------------------+
| Caller-saved ("volatile")| RAX, RCX, RDX, RSI, RDI, R8-R11             |
| — callee is FREE to      | If the caller needs these values preserved  |
|   clobber these          | across a call, IT must save them (push)     |
|   without asking          | before calling and restore after           |
+--------------------------+--------------------------------------------+
| Callee-saved             | RBX, RBP, RSP, R12-R15                     |
| ("non-volatile")          | If the callee wants to USE these, it must  |
| — callee MUST preserve   | save the original value and restore it     |
|   these across the call   | before returning                            |
+--------------------------+--------------------------------------------+
```

This split exists purely as a **performance/convenience optimization by convention**: it lets both sides avoid unnecessary saves. If a function doesn't touch RBX at all, it costs zero instructions to satisfy "callee-saved" — it just never had to save what it never modified. Meanwhile the caller doesn't have to defensively save *every* register before every call, only the ones it actually still needs afterward.

### 15.3 Stack alignment

The ABI requires **RSP to be a multiple of 16 bytes at the point of a `call` instruction**. This matters because SSE instructions (`movaps` etc.) that operate on 16-byte-aligned memory *fault* on misaligned addresses, and libc functions rely on this invariant. Misaligning the stack by even 8 bytes (e.g. from an odd number of unbalanced `push`es before a `call`) is a real, classically painful bug — it can "work" for scalar integer code and then segfault the instant it calls into something using SSE internally (which is most of libc).

### 15.4 A complete, real example: calling `printf` from assembly

```nasm
; call_printf.asm — demonstrates the ABI: calling libc's printf(fmt, arg)
extern printf
extern exit
section .rodata
    fmt db "The answer is %d", 10, 0     ; NUL-terminated, C string

section .text
global main

main:
    push  rbp
    mov   rbp, rsp
    sub   rsp, 16              ; keep 16-byte alignment (call pushed 8, push rbp pushed 8)

    mov   rdi, fmt              ; arg1: format string  (RDI = 1st integer arg)
    mov   esi, 42                ; arg2: the integer to print (RSI = 2nd integer arg)
    xor   eax, eax                ; printf is variadic: ABI requires AL = number of vector
                                    ; (XMM) register args used, here 0, for varargs functions
    call  printf

    mov   edi, 0                  ; exit(0)
    call  exit
```

```bash
nasm -f elf64 call_printf.asm -o call_printf.o
gcc call_printf.o -o call_printf -no-pie      # link against libc via gcc driver
./call_printf
# The answer is 42
```

Notice: this uses `main` (not `_start`) and lets `gcc` link in libc's C runtime startup code, which calls `main` for you with a properly set-up environment — a very different, much more common approach than the raw `_start` in §7.2. The `xor eax, eax` line is a real, ABI-mandated detail specific to **variadic** functions (functions like `printf` that take a variable number of arguments) — it is not optional, and forgetting it is a genuine, documented bug source when hand-writing calls to variadic C functions.

---

## 16. System Calls on Linux

A **system call** is how user-space code asks the kernel to do something it cannot do itself (open a file, allocate memory, send a packet) — this is the actual mechanism, not a metaphor, by which the CPU switches privilege levels.

```
   User space (ring 3)              Kernel space (ring 0)
   +----------------+               +---------------------+
   | your program    |   syscall     |  kernel syscall      |
   | sets rax=N,      |------------->|  dispatch table       |
   | rdi/rsi/rdx/... |               |  (sys_call_table)      |
   | executes         |               |  runs handler          |
   | `syscall`         |<--------------|  returns via `sysret`  |
   | rax = return val  |               |                         |
   +----------------+               +---------------------+
```

The **Linux x86-64 syscall calling convention** (note: deliberately different from the C ABI in §15 — this trips people up):

```
Syscall number:     RAX
Arg 1:               RDI
Arg 2:               RSI
Arg 3:               RDX
Arg 4:               R10   <-- NOT RCX! (RCX is clobbered by the `syscall` instruction itself,
                                          which uses it internally to save RIP)
Arg 5:               R8
Arg 6:               R9
Return value:        RAX  (negative value = -errno on error, by convention)
```

Common syscall numbers on x86-64 Linux (full list: `/usr/include/asm/unistd_64.h` or `ausyscall --dump`):

```
+-----+---------+------------------------------------------+
| RAX | Name    | Args (rdi, rsi, rdx, ...)                 |
+-----+---------+------------------------------------------+
|  0  | read    | fd, buf, count                            |
|  1  | write   | fd, buf, count                            |
|  2  | open    | pathname, flags, mode                     |
|  3  | close   | fd                                        |
|  9  | mmap    | addr, length, prot, flags, fd, offset      |
| 12  | brk     | addr                                       |
| 41  | socket  | domain, type, protocol                     |
| 42  | connect | sockfd, addr, addrlen                       |
| 60  | exit    | status                                      |
| 231 | exit_group | status                                   |
+-----+---------+------------------------------------------+
```

This is the exact mechanism that sits underneath every `write!()`, `TcpStream::connect()`, or `File::open()` you've called from Rust — those std library calls are, at the bottom, a `syscall` instruction with these exact register conventions. Given your XDP/eBPF background: XDP hooks execute *inside* the kernel already (that's the whole performance point — packets are filtered before this user/kernel boundary crossing ever happens for dropped packets), which is precisely why an XDP program can't do arbitrary syscalls the way a normal process can — it's running in a restricted execution context (the eBPF verifier-checked, in-kernel VM) rather than issuing `syscall` instructions from ring 3 at all.

---

## 17. Arrays, Strings, and Structs in Memory

### 17.1 Arrays

An array is nothing but a contiguous run of same-sized elements; indexing is address arithmetic, fully explained already in §8's addressing mode 7:

```nasm
section .data
    arr dq 10, 20, 30, 40, 50    ; array of 5 quadwords (i64), contiguous in memory

section .text
    mov   rbx, arr        ; rbx = base address
    mov   rcx, 2            ; index i = 2
    mov   rax, [rbx + rcx*8]  ; rax = arr[2] = 30   (scale 8 = sizeof(i64))
```

### 17.2 Strings

x86 has no native "string type" — a string is a convention layered on top of a byte array, either:
- **NUL-terminated** (C convention): read bytes until you hit a `0` byte. Length must be computed by scanning (`O(n)`), which is exactly why `strlen` is O(n) and why Rust's `String`/`&str` deliberately store an explicit length instead.
- **Length-prefixed**: store the length before/alongside the data (this is what a NASM `msg_len equ $ - msg`-style constant effectively hand-rolls, and what Rust `&str`/`String` do at runtime with a `(ptr, len)` fat structure).

x86 also has dedicated (now largely legacy/rarely used by compilers) string instructions that operate using RSI (source), RDI (dest), and RCX (count) implicitly, honoring the Direction Flag (DF) for forward/backward iteration:

```nasm
cld               ; clear direction flag: process forward (increasing addresses)
mov   rsi, src      ; source pointer
mov   rdi, dst        ; dest pointer
mov   rcx, 100          ; count
rep   movsb              ; repeat: copy byte [rsi]->[rdi], rsi++, rdi++, rcx--, until rcx==0
                          ; this is a hardware-microcoded memcpy — genuinely how some
                          ; libc memcpy implementations bottom out for certain sizes
```

### 17.3 Structs

A struct is just a fixed layout of fields at fixed byte offsets from a base address — there is no "struct" concept in the hardware at all, only address + offset, exactly like array indexing but with per-field, not uniform, offsets and no runtime-computed scale:

```c
// C / Rust-ish struct
struct Packet {
    uint32_t src_ip;    // offset 0
    uint32_t dst_ip;    // offset 4
    uint16_t src_port;  // offset 8
    uint16_t dst_port;  // offset 10
    uint8_t  protocol;  // offset 12
    // (3 bytes padding to keep the struct's total size a multiple of its alignment)
};                       // total size: 16 bytes
```
```nasm
; rbx = pointer to a Packet
mov   eax, [rbx]           ; eax = packet.src_ip     (offset 0)
mov   ecx, [rbx + 4]        ; ecx = packet.dst_ip     (offset 4)
movzx edx, word [rbx + 8]    ; edx = packet.src_port   (offset 8, zero-extended to fill edx)
movzx r8d, byte [rbx + 12]    ; r8d = packet.protocol    (offset 12)
```

This directly explains **struct padding**: the compiler inserts unused bytes so each field starts at an address matching its own alignment requirement (a 4-byte field at a 4-byte-aligned offset, etc.), because unaligned memory access, while *legal* on x86 (unlike some RISC ISAs which fault), is slower and, for certain instructions/data types, disallowed. This is exactly the mechanical reason `#[repr(C)]` structs in Rust have the sizes/offsets they do, and why `#[repr(packed)]` removing that padding can make field access slower or, on stricter architectures, incorrect.

---

# PART IV — ADVANCED

## 18. Floating Point: x87, SSE, AVX

x86-64 has *three generations* of floating point support, and understanding why is a good lesson in ISA evolution under backward-compatibility constraints:

1. **x87** — the original 1980s floating-point coprocessor. Uses a bizarre 8-register **stack** (not addressable like normal registers — `fld`, `fadd`, `fstp` push/pop an internal stack), 80-bit extended precision internally. Legacy; you'll see it in very old code or very specific extended-precision needs, essentially never in new code.
2. **SSE/SSE2** — introduced flat, directly-addressable XMM registers (§4.3) and made SSE2 the *default* mechanism for `float`/`double` scalar math on x86-64 (this happened specifically because x86-64 was a new enough ISA revision to mandate SSE2 support, unlike 32-bit x86 where it was optional).
3. **AVX/AVX2/AVX-512** — widened registers to 256/512 bits, added a proper non-destructive three-operand form (`vaddps dst, src1, src2` instead of the old two-operand `dst = dst op src`), and are the modern default for anything performance-sensitive/vectorized.

```nasm
; scalar double-precision add using SSE2: c = a + b
movsd xmm0, [a]        ; load a (64-bit double) into low 64 bits of xmm0
movsd xmm1, [b]         ; load b
addsd xmm0, xmm1          ; xmm0 = xmm0 + xmm1  (scalar double add)
movsd [c], xmm0             ; store result
```

Naming convention, worth memorizing since it's systematic: `ss` = scalar single-precision, `sd` = scalar double-precision, `ps` = packed single (multiple floats at once), `pd` = packed double. So `addps` adds 4 floats at once (128-bit XMM ÷ 32-bit float = 4 lanes) — this *is* SIMD, covered next.

## 19. Bit Tricks and SIMD

**SIMD** (Single Instruction, Multiple Data) means one instruction operates on several values packed into one wide register simultaneously — the entire reason AVX registers are wide.

```
   xmm0 (128 bits) treated as 4 lanes of 32-bit floats:

   +----------+----------+----------+----------+
   |  float0  |  float1  |  float2  |  float3  |
   +----------+----------+----------+----------+

   addps xmm0, xmm1   performs 4 independent additions in ONE instruction:

   xmm0:  [ a0 | a1 | a2 | a3 ]
   xmm1:  [ b0 | b1 | b2 | b3 ]
     +      +    +    +    +
   result:[a0+b0|a1+b1|a2+b2|a3+b3]
```

This is exactly the mechanism compiler auto-vectorization exploits, and it's why hand-tuned packet-processing/parsing code (relevant to your networking work) can process multiple bytes/fields per instruction instead of one at a time — e.g. `pcmpeqb` comparing 16 bytes against a pattern simultaneously is the classic building block behind fast byte-scanning (`strchr`-like) implementations and is directly analogous to techniques used in fast packet-header parsing.

Common general bit tricks worth knowing cold, because you'll recognize them instantly in real/compiler-generated code:

```nasm
; check if a number is a power of two:  (n != 0) && ((n & (n-1)) == 0)
mov   rax, rdi
test  rax, rax
jz    .not_pow2
lea   rcx, [rax - 1]
and   rcx, rax
jnz   .not_pow2
; ... rcx == 0 here means it WAS a power of two

; count trailing zeros / find lowest set bit — has a dedicated instruction:
tzcnt rax, rbx      ; rax = number of trailing zero bits in rbx (BMI1 extension)

; population count (count set bits) — also a dedicated instruction:
popcnt rax, rbx      ; rax = number of 1-bits in rbx (very common in bitmask-heavy code)
```

## 20. Interfacing with C/Rust (inline asm, FFI)

### 20.1 Rust `#[no_std]`/inline asm

Given your XDP/eBPF work already lives in `#![no_std]` Rust, you've been *adjacent* to this boundary already. Rust's `asm!` macro is the direct way to embed raw instructions:

```rust
use std::arch::asm;

fn syscall_write(fd: i64, buf: *const u8, count: usize) -> i64 {
    let ret: i64;
    unsafe {
        asm!(
            "syscall",
            in("rax") 1i64,       // write syscall number
            in("rdi") fd,
            in("rsi") buf,
            in("rdx") count,
            lateout("rax") ret,    // rax holds the return value after `syscall`
            out("rcx") _,           // syscall clobbers rcx (used to save RIP internally)
            out("r11") _,             // syscall clobbers r11 (used to save RFLAGS internally)
        );
    }
    ret
}
```

This is literally the same syscall convention from §16, expressed through Rust's inline-asm operand syntax. The `out("rcx") _` / `out("r11") _` clobber declarations exist precisely because the `syscall` instruction itself — as a hardware fact, not a convention — destroys RCX and R11 (it uses them internally to save the pre-syscall RIP and RFLAGS so `sysret` can restore them). If you don't declare these clobbers, the Rust compiler may keep a value live in RCX/R11 across your `asm!` block and get silently corrupted — exactly the kind of bug this guide is meant to make mechanically obvious rather than mysterious.

### 20.2 Calling a hand-written assembly function from C/Rust

```nasm
; add_two.asm — a function callable from C/Rust following the System V ABI
section .text
global add_two
; extern "C" fn add_two(a: i64, b: i64) -> i64
add_two:
    mov   rax, rdi      ; 1st arg (a) is in rdi per the ABI
    add   rax, rsi        ; 2nd arg (b) is in rsi; rax accumulates the return value
    ret
```
```rust
// Rust side
extern "C" {
    fn add_two(a: i64, b: i64) -> i64;
}
fn main() {
    let result = unsafe { add_two(3, 4) };
    println!("{result}"); // 7
}
```

This works with zero glue code precisely *because* both sides agree on the System V ABI from §15 — `rustc`'s `extern "C"` annotation means "use the C calling convention," and that's the same convention this hand-written function obeys.

## 21. Security Mechanisms (stack canaries, ASLR, NX, ROP)

Now that you understand the raw mechanics of the stack (§14) and control flow (§13), these mitigations stop being buzzwords and become mechanically obvious:

- **NX bit ("No-eXecute")** — a page-table permission bit marking a memory page non-executable. `.text` is executable-but-not-writable; stack/heap are writable-but-not-executable. This directly blocks the classic "inject shellcode bytes onto the stack, overwrite the return address to point at them" attack, because the CPU will refuse to *fetch* instructions from a page marked NX, faulting instead.
- **ASLR (Address Space Layout Randomization)** — the loader randomizes the base addresses of the stack, heap, shared libraries, and (with PIE) even `.text` itself, at every process start. This is exactly why RIP-relative addressing (§4.2) exists as a first-class x86-64 feature: PIE code needs to reference its own data/strings using offsets *from wherever it happened to load*, not fixed absolute addresses.
- **Stack canary** — the compiler inserts a random value between local buffers and the saved return address in the prologue, and checks it's unchanged in the epilogue before `ret` runs:

```nasm
func_prologue:
    push  rbp
    mov   rbp, rsp
    sub   rsp, 40
    mov   rax, [fs:0x28]       ; load the canary value (kept in thread-local storage)
    mov   [rbp-8], rax           ; store it just above the saved RBP/return address

    ; ... function body, local buffer at [rbp-40] etc ...

    mov   rax, [rbp-8]           ; reload the canary
    xor   rax, [fs:0x28]           ; compare against the known-good value
    jnz   .stack_smashing_detected  ; if they differ, a buffer overflow clobbered it
    leave
    ret
```
  A buffer overflow big enough to reach the saved return address has to first overwrite this canary, and the epilogue's `xor`+`jnz` check catches that *before* the corrupted return address ever reaches `ret`.
- **ROP (Return-Oriented Programming)** — the attack technique that emerged specifically *because* NX blocks injected shellcode: instead of injecting new code, an attacker chains together addresses of existing tiny instruction sequences already present in the binary (each ending in `ret`), stacking their addresses on the stack so each `ret` "returns" into the next fragment. This is a direct, adversarial exploitation of the exact `ret = pop rip` mechanic from §14 — nothing mysterious, just that mechanic used maliciously by fully controlling what's on the stack.

## 22. Debugging and Reverse Engineering Tools

**GDB**, the essential instruction-level debugger:

```bash
gdb ./hello
(gdb) break _start          # set breakpoint
(gdb) run                    # start execution
(gdb) stepi                  # execute ONE instruction (step into)
(gdb) nexti                  # execute one instruction (step over calls)
(gdb) info registers          # dump all GPRs + flags
(gdb) x/10xb $rsp              # examine 10 bytes at RSP, in hex
(gdb) x/5i $rip                  # disassemble 5 instructions starting at RIP
(gdb) disassemble               # disassemble the current function
(gdb) p $rax                     # print a register's value
```

`objdump` for static (non-running) disassembly:

```bash
objdump -d -M intel ./hello        # disassemble .text, Intel syntax
objdump -d -M intel --no-show-raw-insn ./hello   # cleaner output, no raw hex bytes
```

`strace` to see the exact syscalls (§16) a program makes, with arguments and return values — often the single fastest way to understand what an unfamiliar binary actually does at runtime:

```bash
strace ./hello
# execve("./hello", ["./hello"], 0x7ffd...) = 0
# write(1, "Hello, assembly!\n", 18) = 18
# exit(0) = ?
```

---

# PART V — PRACTICE

## 23. Full Worked Programs

### 23.1 Recursive factorial (demonstrates the stack + calling convention together)

```nasm
; factorial.asm
; extern "C" fn factorial(n: u64) -> u64   -- callable from C/Rust
section .text
global factorial

factorial:
    ; arg: rdi = n.   returns: rax
    cmp   rdi, 1
    jle   .base_case

    push  rdi              ; save n across the recursive call (rdi is caller-saved
                             ; and would otherwise be clobbered by the nested call)
    dec   rdi                ; n - 1
    call  factorial            ; rax = factorial(n-1)
    pop   rdi                  ; restore our n

    imul  rax, rdi                ; rax = factorial(n-1) * n
    ret

.base_case:
    mov   rax, 1
    ret
```

Trace `factorial(4)` by hand — this single trace is worth doing on paper, because it's the clearest possible illustration of how the stack accumulates and unwinds pending state across recursive calls:

```
call factorial(4)
   push 4          stack: [4]
   call factorial(3)
      push 3         stack: [4, 3]
      call factorial(2)
         push 2         stack: [4, 3, 2]
         call factorial(1)
            base case: rax = 1, ret
         pop 2 -> rdi=2;  rax = 1 * 2 = 2;  ret         stack: [4, 3]
      pop 3 -> rdi=3;  rax = 2 * 3 = 6;  ret               stack: [4]
   pop 4 -> rdi=4;  rax = 6 * 4 = 24;  ret                    stack: []
result: 24
```

### 23.2 Sum an array (demonstrates loops + addressing modes together)

```nasm
; sum_array.asm
; extern "C" fn sum_array(ptr: *const i64, len: usize) -> i64
section .text
global sum_array

sum_array:
    ; rdi = ptr, rsi = len
    xor   rax, rax          ; sum = 0
    xor   rcx, rcx           ; i = 0
.loop:
    cmp   rcx, rsi
    jge   .done
    add   rax, [rdi + rcx*8]  ; sum += ptr[i]
    inc   rcx
    jmp   .loop
.done:
    ret
```

### 23.3 String length (NUL-terminated, demonstrates byte-level memory scanning)

```nasm
; strlen_asm.asm
; extern "C" fn strlen_asm(s: *const u8) -> usize
section .text
global strlen_asm

strlen_asm:
    ; rdi = pointer to NUL-terminated string
    xor   rax, rax          ; length = 0
.loop:
    cmp   byte [rdi + rax], 0
    je    .done
    inc   rax
    jmp   .loop
.done:
    ret
```

## 24. Reading Compiler-Generated Assembly

The fastest way to consolidate everything above is to compile small Rust/C snippets and read what a real compiler emits — this closes the loop between "I understand the primitives" and "I can read real code."

```bash
rustc --emit asm -O -C "panic=abort" my_file.rs -o my_file.s
# or, more targeted, use godbolt.org (Compiler Explorer) for instant side-by-side view
```

A good progression to try yourself, each one exercising a different section of this guide:
1. A function that adds two `i64`s → §15 (ABI, argument registers)
2. A function with an `if`/`else` → §13
3. A function that indexes a slice → §8, §17
4. A function that loops summing a `Vec<i64>` → §13 + §17, and check whether `-O` produces SIMD (`addps`-style instructions from §19) — this is auto-vectorization in action
5. A recursive function → §14, §23.1 directly

## 25. Where to Go Next

- **Intel® 64 and IA-32 Architectures Software Developer's Manuals** — the actual, authoritative, exhaustive specification of every instruction. Not a tutorial; a reference you'll return to for the rest of your career with this material.
- **AMD64 Architecture Programmer's Manual** — AMD's equivalent; useful for cross-checking, and it documents some things (like the syscall mechanism itself) more clearly than Intel's manual.
- **System V AMD64 ABI specification** — the actual document behind §15.
- Read the **Linux kernel's `arch/x86/entry/`** syscall entry code, now that §16 gives you the vocabulary to follow it.
- Given your XDP/eBPF direction specifically: read **`bpf_jit_comp.c`** in the Linux kernel source — it's the x86-64 JIT compiler that translates eBPF bytecode into exactly the machine code this guide teaches you to read, which will directly connect your two learning tracks.
- Practice on **Compiler Explorer (godbolt.org)** relentlessly — it is, by far, the highest-leverage tool for internalizing the mapping between high-level code and the assembly this guide has taught you to read.
