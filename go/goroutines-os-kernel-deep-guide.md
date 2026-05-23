# Deep Dive: Goroutines, OS Threads, Kernel Scheduling, Virtual Memory, and CPU Execution

> A complete mental model for understanding how code actually runs — from your Go program
> down to physical transistors switching in RAM.

---

## Table of Contents

1. [The CPU — The Foundation of Everything](#1-the-cpu--the-foundation-of-everything)
2. [Privilege Rings — Kernel Mode vs User Mode](#2-privilege-rings--kernel-mode-vs-user-mode)
3. [Processes and Threads — OS Abstractions](#3-processes-and-threads--os-abstractions)
4. [Context Switching — How Multitasking Actually Works](#4-context-switching--how-multitasking-actually-works)
5. [The Kernel Scheduler — Deciding Who Runs](#5-the-kernel-scheduler--deciding-who-runs)
6. [Virtual Memory — The Grand Illusion](#6-virtual-memory--the-grand-illusion)
7. [Page Tables — Translating Virtual to Physical](#7-page-tables--translating-virtual-to-physical)
8. [The MMU — Hardware Translation Engine](#8-the-mmu--hardware-translation-engine)
9. [TLB — Caching Address Translations](#9-tlb--caching-address-translations)
10. [Goroutines — User-Space Scheduling](#10-goroutines--user-space-scheduling)
11. [The Go Runtime Scheduler — M:N Threading](#11-the-go-runtime-scheduler--mn-threading)
12. [Why Goroutine Switching is Cheaper Than Thread Switching](#12-why-goroutine-switching-is-cheaper-than-thread-switching)
13. [Syscalls — Crossing the Privilege Boundary](#13-syscalls--crossing-the-privilege-boundary)
14. [The Complete Stack — Everything Together](#14-the-complete-stack--everything-together)
15. [Green Threads and User-Space Runtimes in Other Languages](#15-green-threads-and-user-space-runtimes-in-other-languages)
16. [Mental Models and Key Insights](#16-mental-models-and-key-insights)

---

## 1. The CPU — The Foundation of Everything

The CPU is the only thing that actually *executes* anything. Every abstraction — goroutines,
threads, processes, virtual memory — is built on top of the CPU.

At its most fundamental level, the CPU does exactly one thing:

```
fetch instruction from memory
         |
         v
decode instruction (what operation is this?)
         |
         v
execute instruction (perform the operation)
         |
         v
         [repeat forever]
```

That's it. The CPU is a very fast loop.

### What a CPU Actually Knows About

The CPU natively understands only:

- **Registers** — tiny, ultra-fast memory cells inside the CPU itself (RAX, RBX, RSP, RIP, etc.)
- **Memory addresses** — locations in RAM
- **Instructions** — encoded operations like MOV, ADD, JMP, CALL, SYSCALL
- **Privilege levels** — Ring 0 (kernel) vs Ring 3 (user), enforced in hardware
- **Interrupts** — signals that force the CPU to jump to a kernel handler

The CPU does NOT know about:

- Go programs
- Goroutines
- Processes
- Files
- Networks
- Threads

All of those are abstractions invented by operating systems and runtimes, layered on top of
the CPU's raw capabilities.

### The CPU's Most Important Registers

```
+----------+----------------------------------------------------------+
| Register | Purpose                                                  |
+----------+----------------------------------------------------------+
| RIP      | Instruction Pointer — address of the NEXT instruction    |
| RSP      | Stack Pointer — top of the current stack                 |
| RBP      | Base Pointer — base of the current stack frame           |
| RAX      | General purpose / return value register                  |
| RBX/RCX  | General purpose                                          |
| RFLAGS   | CPU flags (zero flag, carry flag, overflow flag, etc.)   |
| CR3      | Points to root page table (current process's memory map) |
| CS       | Code Segment — encodes current privilege level           |
+----------+----------------------------------------------------------+
```

**RIP** is the most important register to understand. It points to the next instruction.
When the OS switches between threads, it saves RIP and restores another thread's RIP.
From the CPU's perspective, execution simply "teleports" to a different location.

---

## 2. Privilege Rings — Kernel Mode vs User Mode

Modern CPUs implement hardware-enforced privilege levels, called **protection rings**.
x86-64 defines 4 rings (Ring 0 through Ring 3), but most operating systems only use two:

```
+------------------------------------------------------+
|                                                      |
|   Ring 3   USER MODE                                 |
|   - Applications (your Go program, Chrome, etc.)     |
|   - Go runtime                                       |
|   - C standard library                               |
|   - RESTRICTED: cannot execute privileged CPU ops    |
|                                                      |
+------------------------------------------------------+
|                                                      |
|   Ring 0   KERNEL MODE                               |
|   - Linux kernel                                     |
|   - Device drivers                                   |
|   - PRIVILEGED: full hardware access                 |
|                                                      |
+------------------------------------------------------+
              |
              v
         CPU Hardware
```

### What "Restricted" Actually Means in User Mode

User mode code is blocked from executing privileged CPU instructions. If it tries, the CPU
raises a fault and jumps to the kernel.

Examples of **privileged instructions** (forbidden in user mode):

```
CLI      — disable CPU interrupts (would break multitasking)
STI      — enable CPU interrupts
HLT      — halt the CPU
MOV CR3  — change page table (would let you access any process's memory)
IN/OUT   — direct I/O port access (hardware communication)
LGDT     — load the Global Descriptor Table
```

Examples of **allowed instructions** in user mode:

```
MOV RAX, 5        — load a value into a register
ADD RAX, RBX      — arithmetic
CALL function     — call a function
JMP label         — unconditional jump
PUSH/POP          — stack operations
CMP RAX, 0        — compare
```

This means your Go program, and even the Go runtime itself, runs directly on the CPU
as machine code — just in restricted mode. It does NOT "go through" the kernel for every
instruction.

### Key Insight: User Mode Is Not Slow

There is a widespread misconception that "kernel mode is faster because it's lower-level."
This is **wrong**. User mode instructions execute directly on the CPU at full speed.

The overhead comes from **transitioning between modes**, not from being in user mode.
The transition itself (user → kernel → user) is expensive.

---

## 3. Processes and Threads — OS Abstractions

### What Is a Process?

A process is an OS-managed container for running a program. It provides:

```
+-----------------------------------------------+
| PROCESS                                       |
|                                               |
|  Virtual Address Space (isolated memory)      |
|  +------------------------------------------+ |
|  | code section (.text)                     | |
|  | read-only data (.rodata)                 | |
|  | global data (.data, .bss)               | |
|  | heap (dynamic allocation)                | |
|  | stack(s) (one per thread)                | |
|  | memory-mapped regions                    | |
|  +------------------------------------------+ |
|                                               |
|  Open file descriptors                        |
|  Network sockets                              |
|  Signal handlers                              |
|  One or more threads                          |
|  Process ID (PID)                             |
|  Credentials (user ID, group ID)              |
+-----------------------------------------------+
```

Processes are isolated from each other. One process cannot read another's memory
(unless explicitly shared). This is enforced by the virtual memory system.

### What Is an OS Thread?

A thread is the actual unit of execution within a process. The CPU runs threads, not processes.

A thread consists of:

```
+-----------------------------------------------+
| OS THREAD                                     |
|                                               |
|  Thread ID (TID)                              |
|  CPU register state:                          |
|    RIP (instruction pointer)                  |
|    RSP (stack pointer)                        |
|    RBP (base pointer)                         |
|    RAX, RBX, RCX, RDX, RSI, RDI              |
|    R8–R15                                     |
|    RFLAGS                                     |
|    XMM0–XMM15 (floating point / SIMD)         |
|  Stack (usually 1–8 MB reserved)              |
|  Scheduling metadata:                         |
|    priority                                   |
|    CPU affinity                               |
|    run queue position                         |
|    sleep/wake state                           |
+-----------------------------------------------+
```

Multiple threads share the same process's virtual address space (heap, globals, code).
Each thread has its own stack and register state.

### Relationship Between Processes and Threads

```
+---------------------------------------------------+
| PROCESS (PID 1234)                                |
|                                                   |
|   Shared:                         Thread 1        |
|   - Virtual memory                  Stack A       |
|   - Heap                            Registers A   |
|   - File descriptors                RIP_A         |
|   - Code (.text)                                  |
|                                   Thread 2        |
|                                     Stack B       |
|                                     Registers B   |
|                                     RIP_B         |
+---------------------------------------------------+
```

---

## 4. Context Switching — How Multitasking Actually Works

If you have 4 CPU cores and 200 threads running (Chrome, VS Code, your Go server, etc.),
most threads cannot run simultaneously. The OS creates the *illusion* of parallelism by
rapidly switching which thread runs on each core.

This rapid switching is called a **context switch**.

### What Happens During a Context Switch

Step by step, when the kernel switches from Thread A to Thread B on the same CPU core:

```
[THREAD A IS RUNNING ON CPU CORE 2]
          |
          | <- timer interrupt fires (e.g. every 4ms)
          v
[CPU automatically jumps to kernel interrupt handler]
[CPU switches to Ring 0 / kernel mode]

  KERNEL SAVES THREAD A's STATE:
  +---------------------------------+
  | Save RIP  (where A was)        |
  | Save RSP  (A's stack)          |
  | Save RBP                       |
  | Save RAX, RBX, RCX, RDX       |
  | Save RSI, RDI, R8-R15         |
  | Save RFLAGS                    |
  | Save XMM0-XMM15               |
  +---------------------------------+
  (stored in kernel memory for Thread A)

  KERNEL SCHEDULER RUNS:
  - Decides which thread runs next (Thread B)
  - May update Thread A's priority
  - Updates run queues

  IF switching processes too:
  - Load new CR3 (new page table = new address space)
  - TLB flush (old address translations are now invalid)

  KERNEL LOADS THREAD B's STATE:
  +---------------------------------+
  | Restore RIP  (where B was)     |
  | Restore RSP  (B's stack)       |
  | Restore RBP                    |
  | Restore RAX, RBX, ...         |
  | Restore RFLAGS                 |
  | Restore XMM0-XMM15            |
  +---------------------------------+

[CPU switches back to Ring 3 / user mode]
[RIP now points to where Thread B was]
[Thread B continues as if nothing happened]
```

### What Triggers a Context Switch

The kernel regains control of the CPU through:

```
+---------------------------+--------------------------------------+
| Trigger                   | Example                              |
+---------------------------+--------------------------------------+
| Timer interrupt           | 4ms scheduler tick fires             |
| Syscall                   | Thread calls read(), write(), etc.   |
| Page fault                | Thread accesses unmapped memory      |
| Hardware interrupt        | Disk I/O completes, network packet   |
| Thread blocks             | Waiting on mutex, I/O, sleep()       |
| Thread terminates         | Thread exits or crashes              |
| Explicit yield            | Thread calls sched_yield()           |
+---------------------------+--------------------------------------+
```

### The Cost of a Context Switch

A context switch is expensive because it involves:

- Entering kernel mode (privilege transition overhead)
- Saving ~20+ registers to memory
- Running the kernel scheduler (decision-making code)
- Possibly changing CR3 (if switching processes) — invalidates TLB
- Restoring ~20+ registers from memory
- Returning to user mode
- Cache disruption — Thread B likely has a "cold" cache

Typical cost: **1–10 microseconds** per context switch on modern hardware.
At 1000 context switches/second, that's 1–10ms of overhead per core per second.

---

## 5. The Kernel Scheduler — Deciding Who Runs

The kernel scheduler is the component responsible for deciding which OS thread runs on
which CPU core at any moment.

Linux uses the **CFS (Completely Fair Scheduler)** as its default scheduler for normal
processes, plus real-time schedulers for high-priority work.

### CFS — Completely Fair Scheduler

CFS tracks how much CPU time each thread has received, measured in **virtual runtime** (vruntime).

The core idea: always run the thread with the *smallest vruntime* — the one that has
received the least CPU time relative to others.

```
Threads in the run queue (sorted by vruntime):

  vruntime:   10ms    15ms    22ms    35ms    41ms
              [A]     [B]     [C]     [D]     [E]
               ^
               |--- Scheduler picks A (smallest vruntime)

  A runs for 4ms...

  vruntime:   14ms    15ms    22ms    35ms    41ms
              [A]     [B]     [C]     [D]     [E]
                       ^
                       |--- Now B has the smallest vruntime
```

CFS uses a **red-black tree** (self-balancing BST) internally so it can always find the
minimum-vruntime thread in O(log n) time.

### Scheduler Data Structures

```
CPU Core 0 Run Queue:
+------------------------------------------+
| Red-Black Tree (sorted by vruntime)      |
|                                          |
|              [Thread C: 22ms]            |
|             /                \           |
|    [Thread A: 10ms]   [Thread D: 35ms]  |
|             \                            |
|        [Thread B: 15ms]                 |
|                                          |
| Current: Thread A (leftmost = smallest)  |
+------------------------------------------+
```

### Load Balancing Across Cores

If one CPU core has 8 threads and another has 0, the scheduler periodically **migrates**
threads between cores to balance load.

```
Core 0 run queue: [A][B][C][D][E][F][G][H]    <- 8 threads
Core 1 run queue: []                            <- 0 threads
         |
         v scheduler detects imbalance
         |
Core 0 run queue: [A][B][C][D]                 <- 4 threads
Core 1 run queue: [E][F][G][H]                 <- 4 threads migrated
```

Migration also has costs: thread state may need copying, and cache locality is disrupted.

---

## 6. Virtual Memory — The Grand Illusion

Virtual memory is one of the most important ideas in operating systems. It gives every
process the *illusion* that it has its own private, contiguous memory space starting
near address 0.

### The Problem Virtual Memory Solves

Without virtual memory:

- Programs would have to coordinate to avoid using the same RAM addresses
- A buggy program could overwrite another program's memory
- Running 100 programs simultaneously would require 100x the RAM
- Programs couldn't be loaded at arbitrary addresses

Virtual memory solves all of this.

### What a Process Sees vs What Actually Exists

```
PROCESS A's VIEW (virtual address space):
+-----------------------+  address 0xFFFFFFFFFFFF  (top)
| kernel space          |  (unmapped for user; kernel lives here)
+-----------------------+
| stack                 |  grows downward
|         |             |
|         v             |
|                       |
|         ^             |
|         |             |
| heap                  |  grows upward (malloc, new, make)
+-----------------------+
| .bss   (uninit data)  |
| .data  (global vars)  |
| .rodata (constants)   |
| .text  (code)         |
+-----------------------+  address 0x000000000000  (bottom)

PHYSICAL RAM (what actually exists):
+----+----+----+----+----+----+----+----+
| A2 | B1 | A1 | B3 | C1 | A3 | B2 | C2 |  <- pages from many processes mixed together
+----+----+----+----+----+----+----+----+
 0    1    2    3    4    5    6    7       <- physical page numbers
```

Process A thinks it has a nice contiguous address space. In reality, its pages are
scattered throughout physical RAM, interleaved with pages from other processes.

### Virtual Address Space Layout on x86-64 Linux

```
+------------------------+  0xFFFFFFFFFFFFFFFF
|  Kernel space          |  Kernel mapped here in every process
|  (not accessible from  |  (protected by page table permissions)
|   user mode)           |
+------------------------+  0xFFFF800000000000
| [canonical hole]       |  Invalid addresses (hardware limitation)
+------------------------+  0x00007FFFFFFFFFFF
|  Stack                 |  Thread stacks (grows down)
|                        |
|  mmap region           |  Shared libraries, file mappings
|                        |
|  Heap                  |  Dynamic memory (grows up)
|                        |
|  .bss / .data          |  Global variables
|  .rodata               |  String literals, constants
|  .text                 |  Executable code
+------------------------+  0x0000000000400000
|  (reserved)            |
+------------------------+  0x0000000000000000
```

### Memory Isolation Between Processes

Two processes can use the same virtual address (e.g., 0x7fff0000) and they point to
completely different physical RAM pages:

```
Process A:  virtual 0x7fff0000 ---> physical page 0x2C1A000  (Process A's data)
Process B:  virtual 0x7fff0000 ---> physical page 0x9F32000  (Process B's data)
```

They never conflict because the page tables for each process have different mappings.

---

## 7. Page Tables — Translating Virtual to Physical

The kernel maintains a **page table** for every process. A page table is a data structure
stored in RAM that records the mapping from virtual pages to physical pages.

### What Is a Page?

Memory is divided into fixed-size chunks called **pages**. On x86-64, the default page size
is **4096 bytes (4 KB)**.

```
Virtual address space divided into pages:

page 0: addresses 0x0000 to 0x0FFF
page 1: addresses 0x1000 to 0x1FFF
page 2: addresses 0x2000 to 0x2FFF
...

Physical RAM also divided into frames (same size):

frame 0: physical 0x0000 to 0x0FFF
frame 1: physical 0x1000 to 0x1FFF
...
```

The page table maps each virtual page number to a physical frame number.

### Why Not a Single Flat Table?

On x86-64, the virtual address space is 128 TB per process. A flat array with one entry
per 4KB page would require:

```
128 TB / 4 KB = 33,554,432 entries
33,554,432 x 8 bytes = 256 MB per process

With 100 processes: 25 GB just for page tables!
```

This is unworkable. So modern CPUs use **multi-level page tables**.

### x86-64 Four-Level Page Table Structure

```
Virtual Address (48 bits used):
+-------+-------+-------+-------+-------------+
| PML4  | PDPT  |  PD   |  PT   | Page Offset |
| 9bits | 9bits | 9bits | 9bits |   12 bits   |
+-------+-------+-------+-------+-------------+

Levels:
PML4  = Page Map Level 4    (root, pointed to by CR3)
PDPT  = Page Directory Pointer Table
PD    = Page Directory
PT    = Page Table

Each level has 512 entries (2^9).
Each entry is 8 bytes.
Each table is exactly one 4KB page.
```

### Walking the Page Table

To translate virtual address `0x00007FFF1234ABCD`:

```
Virtual:   0x00007FFF1234ABCD

Break it down:
  PML4 index = bits[47:39] = 0xFF  = 255
  PDPT index = bits[38:30] = 0x1FC = 508
  PD   index = bits[29:21] = 0x091 = 145
  PT   index = bits[20:12] = 0x034 = 52
  Offset     = bits[11:0]  = 0xBCD = 3021

Step 1: CPU reads CR3 -> physical address of PML4 table (e.g. 0x1A000)
Step 2: CPU reads PML4[255] -> physical address of PDPT table (e.g. 0x2B000)
Step 3: CPU reads PDPT[508] -> physical address of PD table (e.g. 0x3C000)
Step 4: CPU reads PD[145]   -> physical address of PT table (e.g. 0x4D000)
Step 5: CPU reads PT[52]    -> physical frame address (e.g. 0x5E000)
Step 6: Physical address = 0x5E000 + 0xBCD = 0x5EBCD
```

```
CR3 --------> [PML4 table]
                   |
              entry[255]
                   |
                   v
              [PDPT table]
                   |
              entry[508]
                   |
                   v
              [PD table]
                   |
              entry[145]
                   |
                   v
              [PT table]
                   |
              entry[52]
                   |
                   v
              Physical frame 0x5E000
                   +
              Offset 0xBCD
                   |
                   v
              Physical address 0x5EBCD
```

### Page Table Entry Bits

Each page table entry is 64 bits. The upper bits store the physical frame address.
The lower 12 bits are **flags**:

```
Bit 0: Present        — is this page actually in RAM? (0 = page fault)
Bit 1: Writable       — can this page be written to?
Bit 2: User-accessible — can Ring 3 code access this? (0 = kernel only)
Bit 3: Write-through  — cache write behavior
Bit 4: Cache disable  — bypass CPU cache for this page
Bit 5: Accessed       — CPU sets this when page is read
Bit 6: Dirty          — CPU sets this when page is written
Bit 7: Huge page      — this entry covers 2MB instead of 4KB
Bit 63: NX (No-Execute) — forbid executing code from this page (data pages)
```

These flags enable powerful features:

- **NX bit**: prevents buffer overflow exploits from executing shellcode on the stack
- **Present bit**: enables demand paging — pages can be on disk, loaded only when accessed
- **User bit**: kernel memory pages have User=0, so user code cannot access them

---

## 8. The MMU — Hardware Translation Engine

The Memory Management Unit (MMU) is a piece of hardware built into the CPU that performs
virtual-to-physical address translation automatically for every memory access.

```
          Your code accesses virtual address 0x7FFF1234
                         |
                         v
                   +----------+
                   |   MMU    |  (inside the CPU)
                   +----------+
                         |
              checks TLB (cache) first
                    /         \
              TLB hit         TLB miss
                /               \
               /                 v
    Use cached           Walk page tables in RAM
    translation          (CR3 -> PML4 -> PDPT -> PD -> PT)
               \                 /
                v               v
          Physical address (e.g. 0x2C1ABCD)
                         |
                         v
                   Access RAM
```

### The MMU Is In Hardware

This is crucial. The MMU is not software. It is transistor circuits inside the CPU die.
Address translation happens in hardware, in parallel with instruction execution in modern
CPUs. It does not add a meaningful delay for TLB-hit accesses.

### What Happens When the Present Bit Is 0?

If the MMU finds a page table entry with Present=0, it triggers a **page fault** — a CPU
exception that jumps to the kernel's page fault handler.

The kernel then:

1. Checks if the access is valid (is this address supposed to exist?)
2. If invalid → segmentation fault (SIGSEGV), program crashes
3. If valid but page is on disk → loads the page from swap into RAM
4. Updates the page table entry (Present=1, sets physical frame)
5. Returns execution to the instruction that faulted (which now succeeds)

This is how **demand paging** and **swap space** work. Programs can use more virtual memory
than physical RAM exists.

---

## 9. TLB — Caching Address Translations

Walking a 4-level page table requires up to 4 RAM accesses for *every* memory access.
That would be catastrophically slow. The TLB (Translation Lookaside Buffer) solves this.

### What the TLB Is

The TLB is a small, extremely fast cache built into the CPU that stores recent
virtual-to-physical translations.

```
TLB Structure (simplified):

+----------------+---------------------+-------+
| Virtual Page # | Physical Frame #    | Flags |
+----------------+---------------------+-------+
| 0x7FFF1000     | 0x2C1A000           | R/W   |
| 0x400000       | 0x5E3B000           | R/X   |
| 0x7FFFFFFF1000 | 0x99AC000           | R/W   |
| ...            | ...                 | ...   |
+----------------+---------------------+-------+
   (32 to 1024 entries depending on CPU model)
```

Modern CPUs have multiple levels of TLB:

```
L1 ITLB (Instruction TLB): ~128 entries, 1-2 cycle latency
L1 DTLB (Data TLB):        ~64 entries, 1-2 cycle latency
L2 STLB (Shared TLB):      ~2048 entries, ~8 cycle latency
```

On a TLB miss, the CPU performs a page table walk (hardware page table walker) which
takes ~10-100 cycles. The result is loaded into the TLB for future accesses.

### TLB and Context Switching

This is where TLB has a critical interaction with thread/process scheduling.

**Same process, same thread** — TLB entries remain valid. Fast.

**Different thread, same process** — TLB entries might still be valid (same page tables).

**Different process** — when CR3 is loaded with a new value (different page tables),
ALL TLB entries for the old process become invalid. The CPU must flush them.

This **TLB flush** on a process context switch is one reason why context switches are
expensive. After the switch, the new process starts with a cold TLB and suffers many
cache misses until the TLB warms up.

```
Before context switch (Process A running):
  TLB: [A:0x1000->0x2A] [A:0x2000->0x3B] [A:0x3000->0x4C] [A:0x4000->0x5D]
                      All valid for Process A

Switch to Process B (CR3 changes):
  TLB: [invalid] [invalid] [invalid] [invalid]
                      All flushed!

After switching:
  TLB: [B:0x1000->0x9F] <- slowly refilled as Process B accesses memory
```

Linux uses **PCID (Process Context Identifiers)** on modern CPUs to avoid full TLB flushes.
Each process is assigned a PCID tag, and TLB entries are tagged with the PCID. On switching,
only entries with the wrong PCID are ignored — no full flush required.

---

## 10. Goroutines — User-Space Scheduling

A goroutine is NOT an OS thread. It is a lightweight unit of concurrency managed
entirely by the Go runtime in user space.

### The Core Difference

```
OS Thread:
- Created by kernel
- Managed by kernel scheduler
- Kernel knows about it
- ~1–8 MB stack (reserved at creation)
- Switching requires kernel involvement
- Expensive to create (~50–100 microseconds)
- Can run on a CPU core

Goroutine:
- Created by Go runtime
- Managed by Go runtime scheduler
- Kernel has NO idea it exists
- ~2–8 KB stack (grows dynamically up to 1GB default)
- Switching is pure user-space code
- Cheap to create (~1–2 microseconds)
- Cannot run directly on CPU — must be assigned to an OS thread
```

### Goroutine Stack

When you write:

```go
go func() {
    // some code
}()
```

The Go runtime allocates a small goroutine struct and a tiny stack (~2 KB):

```
Goroutine struct:
+---------------------------------+
| goroutine ID                    |
| state (running/runnable/waiting)|
| stack pointer (SP)              |
| program counter (PC)            |
| stack lo (bottom of stack)      |
| stack hi (top of stack)         |
| wait reason                     |
| function to run                 |
+---------------------------------+

Stack: [  2 KB initially  ]  grows as needed, up to 1 GB
```

The stack grows dynamically. When a goroutine's stack is about to overflow, the Go runtime
allocates a larger stack and copies everything over. This is called **stack copying** (older
versions used **segmented stacks**).

### Why Goroutines Are So Cheap

You can create 1,000,000 goroutines comfortably:

```
1,000,000 goroutines x 2 KB stack = 2 GB RAM
1,000,000 OS threads x 8 MB stack = 8,000 GB RAM  <- impossible
```

The small stack size is the primary reason goroutines scale so well.

---

## 11. The Go Runtime Scheduler — M:N Threading

The Go runtime implements **M:N threading**: M goroutines are multiplexed onto N OS threads.

### The G, M, P Model

The Go scheduler uses three types of entities:

```
G — Goroutine
    A unit of work. Contains the goroutine stack, program counter, state.

M — Machine (OS Thread)
    An actual OS thread. Runs on a CPU core. Executes Go code.

P — Processor (logical processor)
    A "context" that holds a local run queue of goroutines.
    One P is assigned to one M at a time.
    GOMAXPROCS controls how many P's exist (default = number of CPU cores).
```

```
                    Go Runtime Scheduler
+----------------------------------------------------------+
|                                                          |
|  P0                    P1                    P2          |
|  +---------------+     +---------------+                 |
|  | Local Run Q   |     | Local Run Q   |                 |
|  | [G3][G7][G12] |     | [G1][G5]      |                 |
|  +---------------+     +---------------+                 |
|       |                      |                           |
|       M0                     M1                          |
|  (OS Thread)            (OS Thread)                      |
|       |                      |                           |
+----------------------------------------------------------+
        |                      |
   CPU Core 0             CPU Core 1

Global Run Queue: [G8][G9][G10][G11]  (goroutines not yet assigned to a P)
```

### How a Goroutine Gets Scheduled

```
1. You call: go someFunc()

2. Go runtime creates a new G struct
   Initializes stack (~2 KB)
   Sets program counter to someFunc

3. Runtime puts G into the current P's local run queue
   (or global queue if local is full)

4. An M running with a P picks up the G from the queue
   Saves current goroutine state
   Loads G's stack pointer and program counter
   Starts executing someFunc

5. If G blocks (I/O, channel, mutex):
   M hands off P to another M (or wakes a parked M)
   G is moved to a wait queue
   M continues with another G from P's queue

6. When G is unblocked:
   G is put back into a run queue
   Some M will pick it up
```

### Work Stealing

If P0's local run queue is empty, P0's M "steals" goroutines from another P's queue:

```
P0 local queue: []          (empty! M0 is idle)
P1 local queue: [G3][G4][G5][G6]

P0 steals half of P1's queue:
P0 local queue: [G5][G6]   <- stolen
P1 local queue: [G3][G4]   <- remaining
```

This ensures all CPUs stay busy and no goroutines starve.

### The Scheduler Loop (Simplified)

Every M runs this loop:

```
schedule():
  g = findRunnable()    // look in local queue, global queue, steal from others
  execute(g)            // actually run the goroutine

findRunnable():
  1. Check local P's run queue (fast, no lock needed)
  2. Check global run queue (needs lock)
  3. Check network poller (goroutines waiting on I/O)
  4. Steal from another P's run queue
  5. If all else fails, park M (put thread to sleep)
```

### Goroutine States

```
+------------+     go statement      +----------+
|  Dead/New  | -------------------> | Runnable |
+------------+                      +----------+
                                         |
                                    M picks up goroutine
                                         |
                                         v
                                    +----------+
                  goroutine blocks  | Running  |
              +--------------------  +----------+
              |                           |
              v                    goroutine yields or
         +----------+              preempted
         | Waiting  |                    |
         +----------+                    v
              |                    +----------+
      event occurs                 | Runnable |  (back to queue)
              |                    +----------+
              v
         +----------+
         | Runnable |  (back to queue)
         +----------+
```

### Goroutine Preemption

Early Go versions (< 1.14) used **cooperative preemption**: goroutines had to voluntarily yield.
A tight CPU loop could starve other goroutines.

Go 1.14+ uses **asynchronous preemption**: the runtime sends a signal (SIGURG on Unix) to
the OS thread, which causes the goroutine to stop at a safe point.

This makes Go goroutines behave more fairly, similar to OS thread preemption.

---

## 12. Why Goroutine Switching is Cheaper Than Thread Switching

This is the key insight. Let's compare them precisely.

### OS Thread Context Switch

```
1. Timer interrupt -> CPU enters kernel mode
   [~10-20 cycles for mode transition]

2. Kernel ISR (Interrupt Service Routine) runs
   [kernel code executing]

3. Save ALL registers to thread's kernel stack:
   RIP, RSP, RBP, RFLAGS
   RAX, RBX, RCX, RDX, RSI, RDI
   R8-R15
   XMM0-XMM15 (16 x 128-bit SSE registers)
   [~100-200 cycles for register saves]

4. Kernel scheduler runs (CFS, picks next thread)
   [may involve lock acquisition on run queues]
   [~hundreds of cycles]

5. If different process: load new CR3 -> TLB flush
   [potentially thousands of cycles for TLB warmup]

6. Restore next thread's registers from memory
   [~100-200 cycles]

7. Return to user mode
   [~10-20 cycles for mode transition]

Total: 1,000 to 10,000+ cycles
       (1–10 microseconds on a 1 GHz CPU equivalent)
```

### Goroutine Context Switch

```
1. Go runtime scheduler function runs (in user space, no mode switch)

2. Save goroutine-relevant registers:
   SP (stack pointer)
   PC (program counter)
   Only caller-saved registers (Go calling convention saves fewer regs)
   [~10-20 cycles]

3. Update goroutine struct state

4. Find next goroutine (from P's local queue, no lock needed)
   [~10-50 cycles]

5. Load next goroutine's SP and PC
   [~10-20 cycles]

6. Jump to next goroutine's code
   [1 cycle]

Total: ~100-300 cycles
       (~0.1 microseconds on modern hardware)
```

### Side-by-Side Comparison

```
+----------------------------------------+-------------+------------------+
| Operation                              | OS Thread   | Goroutine        |
+----------------------------------------+-------------+------------------+
| Mode transition (user <-> kernel)      | Yes (~20ns) | No               |
| Kernel scheduler execution             | Yes         | No               |
| Registers saved                        | ~30+        | ~6-10            |
| TLB flush (same process switch)        | No          | No               |
| TLB flush (different process switch)   | Yes         | N/A              |
| Lock acquisition                       | Yes (kernel)| No (per-P queue) |
| Cache disruption                       | Significant | Minimal          |
| Typical cost                           | 1–10 µs     | 0.1–0.3 µs      |
+----------------------------------------+-------------+------------------+
```

Goroutine switching is approximately **10-100x cheaper** than OS thread switching.

### The Stack Size Advantage

```
OS Thread:
  - Stack: 1–8 MB reserved at thread creation
  - 10,000 threads: 10–80 GB RAM just for stacks
  - Stack overflow = crash (fixed size)

Goroutine:
  - Stack: 2 KB initially
  - 1,000,000 goroutines: 2 GB RAM for stacks
  - Stack grows automatically up to 1 GB
```

---

## 13. Syscalls — Crossing the Privilege Boundary

A syscall (system call) is the mechanism by which user-space code asks the kernel to
perform a privileged operation.

### How a Syscall Works

```
Your Go code:
  data, err := os.ReadFile("data.txt")
         |
         v
Go standard library (os package)
         |
         v
Go runtime syscall wrapper
         |
         v
  SYSCALL instruction executed by CPU
         |
         | [CPU switches to Ring 0 / kernel mode]
         | [CPU loads kernel stack]
         | [CPU jumps to syscall entry point in kernel]
         |
         v
  Linux kernel sys_read() handler:
  - validates arguments
  - checks file descriptor
  - copies data from kernel buffer to user-space buffer
  - updates file position
         |
         | [kernel executes SYSRET instruction]
         | [CPU switches back to Ring 3 / user mode]
         | [CPU restores user-space stack]
         |
         v
  Back in Go runtime
         |
         v
  Returns to os.ReadFile caller
```

The **SYSCALL** instruction is the only sanctioned way for user code to enter kernel mode.
It transfers control to a fixed kernel entry point (stored in a special MSR register).

### Syscall Numbers on Linux x86-64 (Examples)

```
+------+-------------------+
| #    | Syscall           |
+------+-------------------+
|   0  | read              |
|   1  | write             |
|   2  | open              |
|   3  | close             |
|   9  | mmap              |
|  11  | munmap            |
|  39  | getpid            |
|  56  | clone (threads)   |
|  60  | exit              |
|  61  | wait4             |
| 202  | futex             |
| 228  | clock_gettime     |
| 231  | exit_group        |
+------+-------------------+
```

### Goroutines and Syscalls — A Special Case

When a goroutine makes a blocking syscall (like reading from disk), the Go runtime
handles it specially to avoid blocking the OS thread:

```
Goroutine G1 calls os.ReadFile():
                |
                v
        Go runtime detects blocking syscall
                |
                v
  P is detached from M (OS thread)
  P is handed to another M (or new M is created)
                |
                |---------> P continues running other goroutines
                |
  M makes the blocking syscall (M is now "stuck" in kernel)
                |
                | (some time passes, kernel completes the I/O)
                |
  M (OS thread) wakes up with data
  G1 is marked Runnable, put back in run queue
  M tries to acquire a P
                |
                v
  Eventually G1 runs on some M with a P
  Returns data to os.ReadFile caller
```

For **non-blocking I/O** (network, etc.), Go uses the OS's async I/O facilities
(epoll on Linux, kqueue on macOS) so the OS thread never actually blocks.

---

## 14. The Complete Stack — Everything Together

Let's now put every concept together into one coherent diagram.

### The Full Execution Stack

```
+========================================================+
|                                                        |
|   YOUR GO CODE                                         |
|   x := a + b                                           |
|   go someFunc()                                        |
|   data, _ := os.ReadFile("f.txt")                     |
|                                                        |
+========================================================+
             |             |              |
        no syscall      goroutine       syscall
             |             |              |
             v             v              v
+========================================================+
|                                                        |
|   GO RUNTIME                                           |
|                                                        |
|   Goroutine Scheduler (G/M/P model)                    |
|   Stack manager (grows goroutine stacks)               |
|   Garbage Collector                                    |
|   Memory Allocator (span/mcache/mcentral/mheap)        |
|   Channel operations                                   |
|   Select implementation                                |
|   Timer management                                     |
|   Network poller (epoll/kqueue)                        |
|                                                        |
+========================================================+
             |                            |
       user-space ops                 syscall
      (no kernel needed)                 |
             |                           v
             |          +========================================================+
             |          |                                                        |
             |          |   LINUX KERNEL                                         |
             |          |                                                        |
             |          |   Kernel Scheduler (CFS) — schedules OS threads       |
             |          |   Virtual Memory Manager — manages page tables         |
             |          |   File System (VFS, ext4, etc.)                        |
             |          |   Network Stack (TCP/IP, sockets)                      |
             |          |   Device Drivers                                       |
             |          |   Interrupt Handlers                                   |
             |          |   System Call Interface                                |
             |          |                                                        |
             |          +========================================================+
             |                            |
             +--------------------------> |
                                          v
             +========================================================+
             |                                                        |
             |   CPU HARDWARE                                         |
             |                                                        |
             |   Execution Units (ALU, FPU, SIMD)                    |
             |   Registers (RAX, RIP, RSP, CR3, ...)                 |
             |   MMU + TLB (virtual->physical translation)            |
             |   L1/L2/L3 Cache                                       |
             |   Privilege Rings (Ring 0 / Ring 3)                   |
             |   Timer/Interrupt Controller (APIC)                   |
             |                                                        |
             +========================================================+
                                          |
                                          v
             +========================================================+
             |                                                        |
             |   PHYSICAL RAM                                         |
             |                                                        |
             |   Code pages (.text)                                   |
             |   Data pages (.data, .bss)                             |
             |   Heap pages                                           |
             |   Stack pages                                          |
             |   Kernel memory                                        |
             |   Page tables                                          |
             |   DMA buffers                                          |
             |                                                        |
             +========================================================+
```

### A Complete Execution Trace: `x := a + b`

This is the simplest case — no goroutine switching, no syscalls.

```
Your code:  x := a + b

Compiled to:
  MOV RAX, [RSP+8]    ; load 'a' from stack
  MOV RBX, [RSP+16]   ; load 'b' from stack
  ADD RAX, RBX        ; add them
  MOV [RSP+24], RAX   ; store result in 'x'

Execution:
  1. CPU fetches MOV instruction from L1 instruction cache
  2. MOV RAX, [RSP+8] -> MMU translates RSP+8 -> physical address
     TLB hit -> physical address found in ~1-2 cycles
     CPU reads from L1 data cache (or L2/L3/RAM if cache miss)
  3. Same for RBX
  4. ADD executes in ALU in 1 cycle
  5. MOV stores result, MMU translates destination address

Kernel involvement: ZERO
Mode transitions: ZERO
The entire sequence runs in Ring 3 user mode, directly on CPU.
```

### A Complete Execution Trace: `os.ReadFile("data.txt")`

```
os.ReadFile("data.txt")
  |
  v
syscall.Read(fd, buf, len)  [Go runtime]
  |
  v
SYSCALL instruction executes
  |  CPU switches to Ring 0
  |  CPU loads kernel stack pointer (from MSR_LSTAR)
  |  CPU saves RCX (return address), R11 (RFLAGS)
  v
kernel sys_read() entry point
  |
  v
  kernel validates fd (is it open? does process own it?)
  kernel checks buffer address (is it valid user memory?)
  |
  v
  kernel checks buffer cache (is file data already in RAM?)
  if YES -> copy from kernel buffer cache to user buffer
  if NO  -> wait for disk I/O (goroutine blocks, M detaches from P)
  |
  v
  data copied to user buffer
  kernel sets RAX = number of bytes read
  |
  v
SYSRET instruction executes
  |  CPU switches back to Ring 3
  |  CPU restores user stack (RSP)
  |  CPU jumps to return address (was in RCX)
  v
back in Go runtime, returns to os.ReadFile
  |
  v
data available in caller
```

---

## 15. Green Threads and User-Space Runtimes in Other Languages

Go is not unique. Many languages implement their own user-space concurrency, all for
the same reason: OS thread switching is expensive.

### Comparison Table

```
+------------------+-------------------+---------+------------------+------------------+
| Language         | Abstraction       | Default | Stack Size       | Scheduler        |
+------------------+-------------------+---------+------------------+------------------+
| Go               | Goroutines        | M:N     | 2KB - 1GB        | Work-stealing    |
| Erlang/Elixir    | Processes (BEAM)  | M:N     | 233 words        | Per-scheduler    |
| Java (21+)       | Virtual Threads   | M:N     | ~KB, grows       | ForkJoinPool     |
| Kotlin           | Coroutines        | M:N     | Heap only        | Dispatcher       |
| Python           | asyncio           | 1:1*    | OS thread stack  | Event loop       |
| JavaScript       | Promises/async    | 1:1**   | One call stack   | Event loop       |
| Rust (Tokio)     | async/await tasks | M:N     | Heap-allocated   | Work-stealing    |
| C# (.NET)        | Tasks (TPL)       | M:N     | Thread pool      | ThreadPool       |
| Haskell (GHC)    | Threads           | M:N     | ~1KB, grows      | Round-robin      |
+------------------+-------------------+---------+------------------+------------------+

* Python has a GIL; only one OS thread runs Python bytecode at a time
** JavaScript is single-threaded with a non-blocking event loop
```

### Why They All Converged on M:N Threading

The fundamental math is the same for every language:

```
OS threads are expensive:
  - Creation: ~50-100 µs
  - Stack: 1-8 MB each
  - Switching: 1-10 µs

User-space concurrency is cheap:
  - Creation: ~1 µs
  - Stack: 2KB - few KB
  - Switching: 0.1-0.3 µs

For I/O-bound workloads with thousands of concurrent tasks,
user-space scheduling wins by 10-100x.
```

### Go vs Java Virtual Threads (JVM 21+)

Java's Project Loom (released as Virtual Threads in Java 21) is the closest analogue to Go goroutines.

```
+------------------------+---------------------+---------------------+
| Feature                | Go Goroutines       | Java Virtual Threads|
+------------------------+---------------------+---------------------+
| Stack size             | 2 KB initial        | ~hundreds bytes     |
| Stack growth           | Copy-and-grow       | Continuation-based  |
| Scheduler              | Go runtime          | ForkJoinPool        |
| Blocking syscall       | M detaches, async   | Thread pinning*     |
| GC integration         | Yes                 | Yes                 |
| Preemption             | Async (Go 1.14+)    | Cooperative         |
+------------------------+---------------------+---------------------+

* Java virtual threads can "pin" a carrier thread during synchronized blocks
  with native methods, reducing scalability in some cases.
```

---

## 16. Mental Models and Key Insights

### Model 1: Two-Level Scheduling

```
LEVEL 1: Kernel Scheduler
  Asks: "Which OS thread gets CPU time?"
  Operates on: OS threads (M's in Go)
  Scheduler: CFS (Linux), other schedulers possible
  Trigger: Timer interrupts, I/O completion, thread blocking

LEVEL 2: Go Runtime Scheduler
  Asks: "Which goroutine gets to run on this OS thread?"
  Operates on: Goroutines (G's)
  Scheduler: Work-stealing M:N scheduler
  Trigger: goroutine blocks, GOMAXPROCS, preemption signals
```

These two schedulers operate independently. The kernel has no idea goroutines exist.
The Go runtime has no control over which OS thread the kernel runs next.

### Model 2: The Landlord Analogy

```
KERNEL is the building owner:
  - Owns the building (CPU cores)
  - Rents rooms (OS threads) to tenants (processes)
  - Decides when to kick a tenant out (preemption)
  - Provides utilities (I/O, networking, memory)

YOUR GO PROCESS is the tenant:
  - Rented a set of rooms (OS threads = GOMAXPROCS)
  - Can arrange furniture freely (goroutines) inside their rooms
  - Cannot access other tenants' rooms (process isolation)
  - Cannot rewire electricity (no direct hardware access)

GOROUTINES are the people inside the rooms:
  - Move between rooms (OS threads) as arranged by the Go runtime
  - The landlord doesn't know they exist
  - Very cheap to create, cheap to move around
```

### Model 3: The Kernel Does Not Babysit Every Instruction

```
WRONG mental model:
  instruction -> kernel checks it -> CPU executes

CORRECT mental model:
  kernel sets up execution environment (page tables, CR3, thread state)
           |
           v
  CPU executes user-space instructions DIRECTLY at full speed
           |
           until one of these happens:
           |-- timer interrupt (scheduler tick)
           |-- syscall (read, write, etc.)
           |-- page fault (access unmapped memory)
           |-- hardware interrupt (keyboard, network)
           |
           then kernel gets control briefly
           then returns to user code
```

### Model 4: User Space Is Not Inside Kernel Space

```
WRONG:
  [Kernel Space [User Space [Go Runtime [Your Code]]]]

CORRECT:
  +-------------------+     +-------------------+
  |   User Space      |     |   Kernel Space    |
  |                   |     |                   |
  |  Go Runtime       |     |  Linux Kernel     |
  |  Your Code        |<--->|  Drivers          |
  |                   | syscall/return          |
  |  All in Ring 3    |     |  All in Ring 0    |
  +-------------------+     +-------------------+
            |                        |
            +----------+-------------+
                       |
                  CPU Hardware
                  (executes both)
```

### Model 5: Physical RAM Is the Ultimate Reality

```
Everything eventually lives in physical RAM:
  - Your Go code (.text section)
  - Goroutine stacks
  - Heap allocations
  - Page tables
  - Kernel code and data
  - TLB is inside CPU, but backed by RAM-stored page tables

All accesses go through:
  virtual address
       |
  MMU (hardware in CPU)
       |
  TLB lookup (cache in CPU)
       |   miss?
       |---------> page table walk in RAM
       |
  physical address
       |
  L1/L2/L3 cache
       |   miss?
       |---------> DRAM (actual RAM chips)
```

### Key Numbers to Remember

```
+-------------------------------------+------------------+
| Operation                           | Approximate Time |
+-------------------------------------+------------------+
| CPU cycle (3 GHz)                   | 0.3 ns           |
| L1 cache hit                        | 1-4 ns           |
| L2 cache hit                        | 4-12 ns          |
| L3 cache hit                        | 30-70 ns         |
| TLB hit                             | 1-2 ns           |
| TLB miss + page table walk          | 10-100 ns        |
| RAM access (DRAM)                   | 60-100 ns        |
| Goroutine context switch            | 100-300 ns       |
| OS thread context switch            | 1,000-10,000 ns  |
| Syscall round trip                  | 200-1,000 ns     |
| SSD random read                     | 100,000 ns       |
| HDD random read                     | 10,000,000 ns    |
+-------------------------------------+------------------+
```

### The Hierarchy of Costs

Understanding relative costs is how you think efficiently about system performance:

```
Cheapest                                               Most Expensive
|                                                               |
v                                                               v
CPU register   L1$   L2$   L3$   RAM   goroutine  OS thread   disk
  access       hit   hit   hit   read   switch      switch      I/O
  (0.3ns)    (1ns)(5ns)(40ns)(80ns) (200ns)  (5000ns)  (10ms)
```

When you see 1,000,000 goroutines all waiting on I/O (blocked, not burning CPU), that's
efficient — they're sleeping, occupying only ~2 KB each in RAM, and imposing zero scheduling
overhead because the Go runtime only runs runnable goroutines.

### Why This Matters for Writing Better Go

```go
// BAD: launching OS threads directly for each request (if you could)
// Would run out of memory/threads at ~10,000 concurrent requests

// GOOD: goroutines
for req := range requests {
    go handleRequest(req)   // 2KB each, scheduler handles thousands
}
```

```go
// BAD: spinning in a tight loop (burns CPU, prevents other goroutines)
for !done {
    // busy wait
}

// GOOD: use channels or sync primitives
// goroutine will block (state = Waiting), OS thread freed for other goroutines
<-doneCh
```

```go
// BAD: using more OS threads than CPU cores for CPU-bound work
// Creates context switch overhead
runtime.GOMAXPROCS(1000)  // 1000 OS threads, most just context switching

// GOOD: GOMAXPROCS = number of CPU cores (the default)
// Goroutines handle I/O concurrency, OS threads handle CPU parallelism
```

---

## Summary

```
+--------------------------------------------------------+
| CONCEPT           | WHAT IT IS                        |
+-------------------+-----------------------------------+
| CPU               | Only thing that truly executes;   |
|                   | knows only: registers, addresses,  |
|                   | instructions, privilege levels     |
+-------------------+-----------------------------------+
| Privilege rings   | Hardware-enforced CPU modes;       |
|                   | Ring 0 = kernel (full access);     |
|                   | Ring 3 = user (restricted)         |
+-------------------+-----------------------------------+
| Process           | OS container: virtual address      |
|                   | space + open resources + threads   |
+-------------------+-----------------------------------+
| OS Thread         | Actual CPU execution unit;         |
|                   | managed by kernel scheduler        |
+-------------------+-----------------------------------+
| Context switch    | Kernel saves/restores thread       |
|                   | register state; ~1-10 µs           |
+-------------------+-----------------------------------+
| Virtual memory    | Per-process illusion of private    |
|                   | address space; isolation + safety  |
+-------------------+-----------------------------------+
| Page tables       | Kernel data structure in RAM;      |
|                   | maps virtual pages -> physical     |
+-------------------+-----------------------------------+
| MMU               | CPU hardware; translates addresses |
|                   | using page tables                  |
+-------------------+-----------------------------------+
| TLB               | CPU cache for address translations;|
|                   | avoids page-table walks on hits    |
+-------------------+-----------------------------------+
| Goroutine         | User-space concurrency unit;       |
|                   | managed by Go runtime, ~2KB stack  |
+-------------------+-----------------------------------+
| Go scheduler      | M:N; maps goroutines to OS threads |
|                   | using G/M/P model, work stealing   |
+-------------------+-----------------------------------+
| Syscall           | Only way to cross Ring 3->Ring 0;  |
|                   | kernel performs privileged action  |
+-------------------+-----------------------------------+
```

The key mental model that ties everything together:

> The kernel sets up the execution environment (page tables, OS threads, memory regions).
> The CPU executes user-space instructions directly at full speed.
> The kernel only gets involved on syscalls, interrupts, faults, and scheduling events.
> The Go runtime operates entirely in user space, managing goroutines without kernel awareness.
> Physical RAM is the ground truth — everything (code, stacks, page tables, caches) ultimately
> lives there, accessed through layers of hardware abstraction (MMU, TLB, CPU caches).
