# Memory Management in Rust, Go, and C
## A Complete, In-Depth Guide

---

## Table of Contents

1. [Foundations: How Memory Works](#1-foundations-how-memory-works)
   - 1.1 Physical vs Virtual Memory
   - 1.2 Process Address Space Layout
   - 1.3 Memory Segments
   - 1.4 The Stack vs The Heap
   - 1.5 Memory Alignment and Padding
   - 1.6 Cache Hierarchy and Locality

2. [C Memory Management](#2-c-memory-management)
   - 2.1 Storage Duration
   - 2.2 The Stack in C
   - 2.3 Stack Frame Mechanics
   - 2.4 Dynamic Memory — The Heap
   - 2.5 malloc / calloc / realloc / free
   - 2.6 How malloc Works Internally
   - 2.7 Memory Alignment in C
   - 2.8 Pointers Deep Dive
   - 2.9 Arena/Pool Allocators
   - 2.10 All Memory Bugs
   - 2.11 Defensive Patterns
   - 2.12 Tools: Valgrind, ASan, MSan

3. [Rust Memory Management](#3-rust-memory-management)
   - 3.1 The Ownership Model
   - 3.2 Move Semantics
   - 3.3 Copy vs Move Types
   - 3.4 Borrowing and References
   - 3.5 The Borrow Checker
   - 3.6 Lifetimes
   - 3.7 Lifetime Elision Rules
   - 3.8 RAII and the Drop Trait
   - 3.9 Smart Pointers
   - 3.10 Interior Mutability
   - 3.11 Memory Layout in Rust
   - 3.12 Pin<T> and Unpin
   - 3.13 Unsafe Rust and Raw Pointers
   - 3.14 Custom Allocators

4. [Go Memory Management](#4-go-memory-management)
   - 4.1 Go's Memory Philosophy
   - 4.2 Goroutine Stacks
   - 4.3 Escape Analysis
   - 4.4 Go's Memory Allocator Internals
   - 4.5 The Garbage Collector
   - 4.6 Tri-Color Mark-and-Sweep
   - 4.7 Write Barriers
   - 4.8 GC Phases and Pacing
   - 4.9 sync.Pool
   - 4.10 GC Tuning: GOGC and GOMEMLIMIT
   - 4.11 Memory Leaks in Go
   - 4.12 Memory Profiling

5. [Comparison and Mental Models](#5-comparison-and-mental-models)

---

## 1. Foundations: How Memory Works

Before understanding how C, Rust, and Go manage memory, you must have a precise mental model of the hardware and OS layers they sit on top of.

---

### 1.1 Physical vs Virtual Memory

A modern OS gives each process an **illusion** of having its own contiguous address space. This is **virtual memory**. The CPU's Memory Management Unit (MMU) translates virtual addresses to physical addresses using a **page table** maintained by the OS.

```
PROCESS A VIEW                  PHYSICAL RAM
+------------------+            +------------------+
| Virtual Address  |   MMU      | Physical Frame   |
| 0x0000           | -------->  | 0x3A0000         |
| 0x1000           | -------->  | 0x7F2000         |
| 0x2000           | -------->  | (on disk/swap)   |  <-- Page fault triggers OS
| ...              |            | ...              |
+------------------+            +------------------+

PROCESS B VIEW
+------------------+
| Virtual Address  |            Both processes see "same" virtual addresses
| 0x0000           | -------->  | 0xB10000         |  (different physical)
| 0x1000           | -------->  | 0xC22000         |
+------------------+
```

**Key properties of virtual memory:**
- Each page is typically **4 KB** (x86-64 default). Huge pages are 2MB or 1GB.
- Pages not in RAM are stored on **swap** (disk). Accessing them triggers a **page fault**, a hardware interrupt that lets the OS load the page back in.
- The OS uses **demand paging**: pages are only loaded into physical memory when accessed, not at process start.
- Two processes mapping the same physical frame = **shared memory** (used by shared libraries, IPC).

---

### 1.2 Process Address Space Layout

On a 64-bit Linux system, a process address space looks like:

```
High Address  0xFFFF FFFF FFFF FFFF
              +-----------------------------+
              |  Kernel Space (not mapped)  |  <-- Only accessible in kernel mode
              +-----------------------------+
              |  Stack (grows downward)     |  <-- Each thread has its own stack
              |          |                  |
              |          v                  |
              |      (unmapped gap)         |  <-- Stack and heap grow toward each other
              |                             |
              |          ^                  |
              |          |                  |
              |  Heap (grows upward)        |  <-- malloc, new, Box::new etc.
              +-----------------------------+
              |  BSS Segment                |  <-- Uninitialized global/static data
              +-----------------------------+
              |  Data Segment (.data)       |  <-- Initialized global/static data
              +-----------------------------+
              |  Text Segment (.text)       |  <-- Executable code (read-only)
              +-----------------------------+
Low Address   0x0000 0000 0000 0000
```

On Linux x86-64 with ASLR (Address Space Layout Randomization), base addresses are randomized at each execution to make exploitation harder.

---

### 1.3 Memory Segments in Detail

#### Text Segment (.text)
- Contains compiled machine code (read-only, executable).
- Shared across processes running the same binary (OS deduplication).
- Attempting to write to it causes a **segmentation fault**.
- String literals (`"hello"`) often live here.

#### Data Segment (.data)
- Contains **initialized** global and static variables.
- Readable and writable.
- Example: `int x = 5;` at file scope → stored in `.data`.

#### BSS Segment (.bss)
- Contains **uninitialized** (or zero-initialized) global and static variables.
- Does not actually occupy space in the binary file (just a size annotation).
- OS zero-fills it at process start.
- Example: `int y;` at file scope → stored in `.bss`.

#### Heap
- Dynamically allocated memory, managed by the runtime/allocator.
- Grows upward (toward higher addresses) via `sbrk()` or `mmap()` system calls.
- Persists until explicitly freed (C) or collected (Go) or dropped (Rust).

#### Stack
- Automatically managed, LIFO structure.
- Grows downward (toward lower addresses).
- Holds function call frames: local variables, return addresses, saved registers, arguments.
- Each thread has its own stack (default ~8MB on Linux).
- Overflow = **stack overflow** → OS sends SIGSEGV.

---

### 1.4 The Stack vs The Heap

This is the most critical mental model to internalize.

```
STACK                              HEAP
+------------------------+         +------------------------+
|  main() frame          |         | [Header][Data         ]|
|    int a = 5           |         |                        |
|    char buf[64]        |         | [Header][Data   ]      |
|  +--------------------+|         |                        |
|  | foo() frame        ||         | [Free ][           ]   |
|  |   int b = 10       ||         |                        |
|  |   int* p -------+--||-------> | [Header][Data        ] |
|  +--------------------+|         |                        |
+------------------------+         +------------------------+
      ^                                       ^
      |                                       |
  Managed by CPU/compiler             Managed by allocator
  (push/pop on call/return)           (malloc/free or GC or Drop)
  O(1) allocation                     O(1) amortized, fragmentation possible
  Size known at compile time          Size known at runtime
  Spatial locality (cache-friendly)   Pointer chasing (cache-unfriendly)
  Limited size (~8MB per thread)      Limited by virtual address space
  Automatic cleanup                   Explicit/GC/RAII cleanup
```

**Rule of thumb:**
- Stack: small, short-lived, size-known data.
- Heap: large, long-lived, dynamically-sized data.

---

### 1.5 Memory Alignment and Padding

CPUs access memory most efficiently when data is aligned to its natural boundary. A 4-byte int should start at an address divisible by 4. An 8-byte double at an address divisible by 8.

```
struct Example {     // How it looks in memory on a 64-bit system:
    char a;          // 1 byte at offset 0
                     // 3 bytes PADDING (to align b to 4-byte boundary)
    int  b;          // 4 bytes at offset 4
    char c;          // 1 byte at offset 8
                     // 7 bytes PADDING (to align next struct to 8-byte boundary)
    double d;        // 8 bytes at offset 16
};                   // Total: 24 bytes (not 14!)

Memory layout:
Offset: 0    1    2    3    4    5    6    7    8    9...15  16...23
        [a] [PAD  PAD  PAD] [   b         ] [c] [PAD PAD... ] [ d       ]
```

**Why padding?**
- Misaligned access on x86 works but is slower (2 cache line reads instead of 1).
- On some architectures (ARM strict mode) misaligned access causes a hardware fault.

**Struct field ordering matters:**

```
// BAD: 24 bytes
struct Bad  { char a; int b; char c; double d; };

// GOOD: 16 bytes (reorder fields from largest to smallest)
struct Good { double d; int b; char a; char c; };

Memory layout of Good:
Offset: 0...7   8...11  12  13  14  15
        [ d   ] [  b  ] [a] [c] [padding x2]
```

This concept is identical in C, Rust, and Go (though Rust reorders by default for optimization).

---

### 1.6 Cache Hierarchy and Memory Locality

```
CPU Core
  +----------+
  |    ALU   |    Registers: ~16 x 8 bytes, <1 cycle access
  +----------+
       |
  +----------+
  |  L1 Cache|    ~32KB,   3-4 cycles,   per-core
  +----------+
       |
  +----------+
  |  L2 Cache|    ~256KB,  10-12 cycles, per-core
  +----------+
       |
  +--------------------+
  |     L3 Cache       |  ~8-32MB,  30-50 cycles,  shared across cores
  +--------------------+
       |
  +--------------------+
  |       RAM          |  GBs,      100-200 cycles
  +--------------------+
       |
  +--------------------+
  |       Disk (SSD)   |  ~100,000 cycles for random access
  +--------------------+
```

Cache lines are typically **64 bytes**. When you access one byte, the CPU fetches the entire 64-byte cache line. This is why **spatial locality** (accessing nearby memory sequentially) is critical.

---

## 2. C Memory Management

C gives you direct, manual control over memory with no safety net. This is both its power and its peril. You are responsible for every allocation and every deallocation.

---

### 2.1 Storage Duration

C has four storage durations:

| Duration    | Keyword        | Where        | Lifetime                  |
|-------------|---------------|--------------|---------------------------|
| Automatic   | (default)     | Stack        | Until enclosing scope ends|
| Static      | `static`/global| .data/.bss  | Entire program lifetime   |
| Thread-local| `_Thread_local`| TLS segment  | Thread lifetime           |
| Dynamic     | malloc/free    | Heap         | Between malloc and free   |

```c
#include <stdio.h>
#include <stdlib.h>

int global_var = 42;          // Static (initialized) — .data segment
int uninit_var;               // Static (uninitialized) — .bss segment, zeroed
static int file_scoped = 10;  // Static, file-scope only

void demo(void) {
    int local = 5;            // Automatic — stack, gone after function returns
    static int persist = 0;   // Static inside function — .data, persists across calls
    persist++;

    int *heap = malloc(sizeof(int)); // Dynamic — heap, must free
    *heap = 100;
    free(heap);               // Must free or leak
}
```

---

### 2.2 The Stack in C

The stack is managed automatically by the compiler through function call/return sequences using CPU instructions.

```
Before foo() call:             After foo() called:
+------------------+           +------------------+
| main's frame     |           | main's frame     |
|  int x = 5      |           |  int x = 5      |
|  ...             |           |  ...             |
|                  |           +------------------+
|  [stack ptr] -> |           | foo's frame      |
|                  |           |  return address  | <-- pushed by CALL instruction
+------------------+           |  saved RBP       | <-- caller's frame pointer
                                |  int a = 1      |
                                |  char buf[32]   |
                                |  [stack ptr] -> |
                                +------------------+
```

**What goes on a stack frame:**
1. **Return address** – where to jump when the function returns.
2. **Saved caller registers** – to restore after the call.
3. **Local variables** – all variables declared in the function.
4. **Function arguments** (some may be in registers per ABI).
5. **Alignment padding** – to keep the stack 16-byte aligned (System V AMD64 ABI).

```c
void foo(int a, int b) {
    char buffer[16];       // 16 bytes on stack
    int result = a + b;    // 4 bytes on stack
    // total frame: ~48 bytes (includes return addr, saved rbp, etc.)
}
```

Stack frame diagram for `foo(3, 4)` on x86-64 Linux:

```
High address
+-----------------------+
|   Argument 'b' = 4    |  (passed in register RSI, may spill here)
|   Argument 'a' = 3    |  (passed in register RDI, may spill here)
+-----------------------+
|   Return Address      |  <-- pushed by CALL instruction (8 bytes)
+-----------------------+
|   Saved RBP           |  <-- old base pointer (8 bytes)
+-----------------------+  <-- RBP points here (frame pointer)
|   result (4 bytes)    |
|   [4 bytes padding]   |  <-- alignment
|   buffer[16]          |  <-- 16 bytes
+-----------------------+  <-- RSP points here (stack pointer)
Low address
```

---

### 2.3 Stack Frame Mechanics

```c
#include <stdio.h>

int add(int x, int y) {
    int sum = x + y;  // sum lives on the stack
    return sum;       // return value in RAX register; frame is popped
}

int main(void) {
    int a = 3;
    int b = 4;
    int result = add(a, b);  // stack frame for add() pushed, then popped
    printf("%d\n", result);
    return 0;
}
```

**Danger: Returning pointer to stack variable**

```c
// BUG: stack memory returned to caller — UNDEFINED BEHAVIOR
int* bad_function(void) {
    int local = 42;
    return &local;  // WRONG: local is destroyed when function returns
}

// This is a dangling pointer — classic C bug
int* p = bad_function();
printf("%d\n", *p);  // Reading freed stack memory — undefined behavior
```

---

### 2.4 Dynamic Memory — The Heap

When you need memory whose size is unknown at compile time, or that must outlive the function that created it, you use the heap.

```c
#include <stdlib.h>
#include <string.h>

// Pattern: allocate, use, free
int main(void) {
    int n = 100;

    // Allocate array of 100 ints
    int *arr = malloc(n * sizeof(int));
    if (arr == NULL) {   // ALWAYS check for allocation failure
        perror("malloc");
        return 1;
    }

    // Use the memory
    for (int i = 0; i < n; i++) arr[i] = i * 2;

    // Free when done
    free(arr);
    arr = NULL;  // Good practice: null out to prevent use-after-free

    return 0;
}
```

---

### 2.5 malloc / calloc / realloc / free

#### `malloc(size_t size)`
Allocates `size` bytes of uninitialized memory. Returns `void*` or `NULL` on failure.

```c
int *p = malloc(sizeof(int) * 10);  // 40 bytes, contents UNDEFINED
if (!p) { /* handle */ }
```

#### `calloc(size_t nmemb, size_t size)`
Allocates `nmemb * size` bytes, **zero-initialized**. Safer for arrays.

```c
int *p = calloc(10, sizeof(int));  // 40 bytes, all zeros
```

Internally, calloc can skip zeroing if the pages are freshly mapped from the OS (OS already provides zero pages). This is an optimization unavailable to malloc + memset.

#### `realloc(void *ptr, size_t new_size)`
Resizes a previously malloc'd block.

```c
int *arr = malloc(10 * sizeof(int));

// Grow the array
int *tmp = realloc(arr, 20 * sizeof(int));
if (tmp == NULL) {
    // realloc FAILED — arr is still valid, don't lose it!
    free(arr);
    return 1;
}
arr = tmp;  // ONLY assign if realloc succeeded
```

**Important:** `realloc` may move the block to a new address. All old pointers into it are **invalidated**.

#### `free(void *ptr)`
Returns memory to the allocator. Does NOT zero memory. Does NOT set the pointer to NULL.

```c
free(ptr);
// ptr now points to freed memory — it's a dangling pointer
// Accessing it is undefined behavior
ptr = NULL;  // best practice: prevent accidental use
```

**Rules for free:**
- Only free pointers returned by `malloc/calloc/realloc`.
- Free exactly once (double-free is UB and security vulnerability).
- Never free a stack pointer.
- Never free a pointer into the middle of an allocation.

---

### 2.6 How malloc Works Internally

Understanding this is critical for performance and debugging.

The C library allocator (e.g., glibc's **ptmalloc2**) manages a pool of memory obtained from the OS.

#### Obtaining memory from the OS

```
+-------------------+        sbrk()/brk()     +-------------------+
|  malloc internals |  ------------------>    |  OS Kernel        |
|  (user space)     |  <------------------    |  (grows heap      |
+-------------------+        virtual pages    |   in page-sized   |
                                              |   chunks)         |
                                              +-------------------+
      OR

+-------------------+        mmap(MAP_ANON)   +-------------------+
|  malloc internals |  ------------------>    |  OS Kernel        |
+-------------------+  <------------------    |  (new mapping     |
                        anonymous pages       |   anywhere)       |
                                              +-------------------+
```

Large allocations (>128KB by default) use `mmap` directly.
Small/medium allocations use a pool managed by the allocator.

#### Free list structure (simplified)

```
Heap region (obtained from OS):

+--------+----------+--------+----------+--------+----------+
|  HDR   |   USED   |  HDR   |   FREE   |  HDR   |   USED   |
| size=32|  32 bytes | size=64| 64 bytes | size=16|  16 bytes|
| in_use |  (data)  | free   |  (next---+->free) | in_use   |
+--------+----------+--------+----------+--------+----------+
                                 |
                           [free list pointer]
                                 |
                                 v
                    +--------+----------+
                    |  HDR   |   FREE   |
                    | size=96| 96 bytes |
                    | free   | (next=0) |
                    +--------+----------+
```

**ptmalloc chunk header (simplified):**
```
+-----------------------------------+
|  prev_size  (8 bytes)             |  <- size of previous chunk if it's free
|  size       (8 bytes)             |  <- size of this chunk + flags in low bits
|  fd         (8 bytes, if free)    |  <- forward pointer to next free chunk
|  bk         (8 bytes, if free)    |  <- backward pointer to previous free chunk
+-----------------------------------+
|  user data begins here            |
```

#### Bins (free lists organized by size)

ptmalloc organizes free chunks into bins:
- **Fast bins**: chunks 16–80 bytes, LIFO singly-linked, no coalescing — fastest
- **Small bins**: chunks <512 bytes, doubly-linked, exact sizes
- **Large bins**: chunks ≥512 bytes, sorted by size
- **Unsorted bin**: recently freed chunks, sorted lazily

#### Thread caches (tcmalloc / jemalloc)

Modern allocators like **tcmalloc** (used by Go and Chrome) and **jemalloc** (used by Firefox, Rust) add per-thread caches to avoid locking:

```
Thread 1                         Thread 2
+------------------+             +------------------+
| Thread Cache     |             | Thread Cache     |
|  size=8:  [list] |             |  size=8:  [list] |
|  size=16: [list] |             |  size=16: [list] |
|  size=32: [list] |             |  size=32: [list] |
+-------+----------+             +--------+---------+
        |                                 |
        +---------------+-----------------+
                        |
               +--------+--------+
               | Central Cache   |   (less frequent access, locked)
               +-----------------+
                        |
               +--------+--------+
               |      Heap       |   (OS pages)
               +-----------------+
```

---

### 2.7 Memory Alignment in C

```c
#include <stdlib.h>
#include <stdio.h>
#include <stdint.h>

// Standard malloc guarantees alignment suitable for any standard type
// (usually 8 or 16 bytes on 64-bit systems)
void *p = malloc(1024);   // guaranteed aligned to max_align_t

// For SIMD or custom alignment requirements:
// C11: aligned_alloc(alignment, size)
void *simd_buf = aligned_alloc(64, 1024);  // 64-byte aligned for AVX-512
free(simd_buf);

// POSIX: posix_memalign
void *buf;
posix_memalign(&buf, 32, 1024);  // 32-byte aligned
free(buf);

// Check struct alignment and size:
printf("sizeof(int): %zu\n", sizeof(int));
printf("alignof(int): %zu\n", _Alignof(int));

// Force alignment of a struct:
typedef struct {
    int x;
    int y;
} __attribute__((aligned(16))) AlignedPoint;  // GCC/Clang
```

---

### 2.8 Pointers Deep Dive

A pointer is a variable holding a memory address. On a 64-bit system, all pointers are 8 bytes.

```
int x = 42;
int *p = &x;

Memory layout:
                 +--------+
Address 0x7fff1000:| x = 42 |   (4 bytes)
                 +--------+
                 
                 +------------------+
Address 0x7fff1010:| p = 0x7fff1000  |  (8 bytes — holds address of x)
                 +------------------+
                       |
                       +-----> points to x at 0x7fff1000
```

#### Pointer arithmetic

```c
int arr[5] = {10, 20, 30, 40, 50};
int *p = arr;   // p points to arr[0]

/*
Memory:
Addr: 0x1000  0x1004  0x1008  0x100C  0x1010
      [  10  ][  20  ][  30  ][  40  ][  50  ]
       ^
       p
*/

p + 1;  // advances by sizeof(int) = 4 bytes → points to arr[1]
p + 2;  // advances by 8 bytes → points to arr[2]

*(p + 3);  // == arr[3] == 40
p[3];      // same thing — subscript is syntactic sugar for *(p + 3)

// Pointer difference: number of elements between two pointers
ptrdiff_t diff = (arr + 4) - arr;  // == 4, not 16
```

#### Pointer types

```c
void *vp;          // Generic pointer, no type info, cannot dereference directly
int *ip;           // Pointer to int
const int *cip;    // Pointer to const int (can't modify through pointer)
int * const pci;   // Const pointer to int (can't change pointer itself)
int **ipp;         // Pointer to pointer to int

// Function pointer
int (*add_func)(int, int);  // Pointer to function taking two ints, returning int
add_func = &some_add_function;
int result = add_func(3, 4);
```

#### void* and casting

```c
// C allows implicit conversion between void* and any pointer type
int x = 5;
void *vp = &x;         // implicit conversion OK in C
int *ip = vp;          // implicit conversion back OK in C (NOT in C++)

// Manual cast (C++ requires this)
int *ip2 = (int *)vp;
```

---

### 2.9 Arena/Pool Allocators

For performance-critical code, custom allocators avoid `malloc`'s overhead.

#### Arena Allocator

An arena (also called a region or bump allocator) allocates by bumping a pointer. All memory freed at once.

```c
#include <stdlib.h>
#include <stdint.h>
#include <string.h>

typedef struct {
    uint8_t *buf;    // Backing buffer
    size_t   size;   // Total size
    size_t   offset; // Current allocation position
} Arena;

Arena arena_init(size_t size) {
    Arena a;
    a.buf = malloc(size);
    a.size = size;
    a.offset = 0;
    return a;
}

void* arena_alloc(Arena *a, size_t size, size_t align) {
    // Align offset up to 'align'
    size_t padding = (align - (a->offset % align)) % align;
    size_t new_offset = a->offset + padding + size;

    if (new_offset > a->size) return NULL;  // out of space

    void *ptr = a->buf + a->offset + padding;
    a->offset = new_offset;
    return ptr;
}

void arena_reset(Arena *a) {
    a->offset = 0;   // "Free" all: just reset pointer, no per-allocation overhead
}

void arena_destroy(Arena *a) {
    free(a->buf);
    a->buf = NULL;
}

/*
Arena memory layout:

+-------+-------+-------+---+-------+----------+
| alloc1| alloc2| alloc3|PAD| alloc4| FREE SPACE|
+-------+-------+-------+---+-------+----------+
0                                   ^           ^
                                 offset        size

All freed at once by resetting offset to 0. O(1) alloc, O(1) free-all.
*/
```

#### Object Pool Allocator

Pre-allocates fixed-size objects to avoid fragmentation:

```c
#define POOL_SIZE 1024
#define OBJECT_SIZE sizeof(MyStruct)

typedef struct FreeNode { struct FreeNode *next; } FreeNode;

typedef struct {
    uint8_t  memory[POOL_SIZE * OBJECT_SIZE];
    FreeNode *free_list;
    size_t    count;
} Pool;

void pool_init(Pool *p) {
    p->free_list = NULL;
    // Chain all objects into a free list
    for (int i = POOL_SIZE - 1; i >= 0; i--) {
        FreeNode *node = (FreeNode *)(p->memory + i * OBJECT_SIZE);
        node->next = p->free_list;
        p->free_list = node;
    }
    p->count = POOL_SIZE;
}

void* pool_alloc(Pool *p) {
    if (!p->free_list) return NULL;
    FreeNode *node = p->free_list;
    p->free_list = node->next;
    p->count--;
    return node;
}

void pool_free(Pool *p, void *ptr) {
    FreeNode *node = ptr;
    node->next = p->free_list;
    p->free_list = node;
    p->count++;
}

/*
Pool layout:

Pre-allocated slab:
+-----+-----+-----+-----+-----+-----+
| obj0| obj1| obj2| obj3| obj4| obj5|
+--+--+--+--+-----+--+--+--+--+-----+
   |     |            |     |
   |     |    FREE    |     |
   +---->+----------->+---->+ (free list chain through objects)
*/
```

---

### 2.10 All Memory Bugs

This is where C programmers spend most of their debugging time.

#### Memory Leak

Allocated memory never freed. The program's memory usage grows forever.

```c
void process_request(void) {
    char *buf = malloc(4096);
    if (some_early_return_condition) {
        return;       // BUG: forgot to free buf before return
    }
    // ... use buf ...
    free(buf);        // Only reached if early return doesn't trigger
}
```

#### Dangling Pointer (Use-After-Free)

Using a pointer after the memory it points to has been freed.

```c
int *p = malloc(sizeof(int));
*p = 42;
free(p);
// p is now a dangling pointer
printf("%d\n", *p);  // UB: reads freed memory — may crash, may print garbage,
                     // may print old value, may trigger security exploit
```

The allocator may have given that memory to another allocation, so you might be silently corrupting another object's data.

#### Double Free

Freeing the same pointer twice. Often causes heap corruption.

```c
int *p = malloc(sizeof(int));
free(p);
free(p);  // DOUBLE FREE: corrupts allocator metadata
          // May crash immediately, or cause silent heap corruption
          // that manifests as a crash much later
          // Classic security vulnerability (heap exploitation)
```

#### Buffer Overflow

Writing past the end of an allocated buffer.

```c
char *buf = malloc(10);
strcpy(buf, "Hello, World!");  // BUG: "Hello, World!" is 14 chars + NUL = 15 bytes
                               // Overwrites memory past buf[9]
                               // Can corrupt adjacent allocations, free list, or return addresses
```

#### Stack Buffer Overflow

```c
void vulnerable(const char *input) {
    char local[32];
    strcpy(local, input);  // BUG: if input > 31 chars, corrupts stack
                           // Can overwrite saved return address → arbitrary code execution
}
```

This is the basis of classic stack-smashing attacks.

#### Heap Buffer Underflow

Writing before the start of a buffer:

```c
int *arr = malloc(10 * sizeof(int));
arr[-1] = 99;   // BUG: writes into malloc chunk header or previous chunk
```

#### Wild Pointer

Using an uninitialized pointer:

```c
int *p;           // Uninitialized — could point anywhere
*p = 42;          // BUG: writes to random address
```

#### Off-by-One Error

```c
int *arr = malloc(10 * sizeof(int));
for (int i = 0; i <= 10; i++) {   // BUG: should be i < 10
    arr[i] = i;                    // arr[10] is out of bounds
}
```

#### Memory Overlap

```c
char src[20] = "Hello World";
char dst[20];
// memcpy assumes no overlap — for overlapping regions, use memmove
memcpy(dst, src, 11);   // OK, no overlap here
// memcpy(src, src+3, 8); // BUG: overlapping regions with memcpy
memmove(src, src+3, 8); // CORRECT: handles overlap
```

#### Integer Overflow in Size Calculation

```c
// Attacker controls 'n'
size_t n = UINT_MAX;   // e.g., 0xFFFFFFFF
int *arr = malloc(n * sizeof(int));  // integer overflow: n * 4 wraps to small value
// malloc succeeds with tiny buffer, but code writes 'n' elements → overflow
for (size_t i = 0; i < n; i++) arr[i] = 0;
```

**Safe pattern:**
```c
// Check for overflow before multiplying
if (n > SIZE_MAX / sizeof(int)) { /* handle error */ }
int *arr = malloc(n * sizeof(int));
// OR use calloc which handles this internally:
int *arr2 = calloc(n, sizeof(int));
```

#### Freeing Stack Memory

```c
void bad(void) {
    int local = 5;
    free(&local);   // CATASTROPHIC: tries to free stack memory
}
```

#### Freeing Static Memory

```c
char *s = "hello";   // String literal in .text segment (read-only)
free(s);             // CRASH: not heap-allocated
```

---

### 2.11 Defensive Patterns in C

```c
// 1. Always check malloc return
void *p = malloc(size);
if (!p) { perror("malloc"); exit(1); }

// 2. Null pointer after free
free(p);
p = NULL;

// 3. Wrapper that aborts on OOM
void *xmalloc(size_t size) {
    void *p = malloc(size);
    if (!p) { perror("malloc"); abort(); }
    return p;
}

// 4. Use sizeof with the variable, not the type
int *arr = malloc(n * sizeof *arr);   // *arr has type int, automatically correct

// 5. Prefer calloc for arrays (zero-initializes, handles overflow internally)
int *arr2 = calloc(n, sizeof *arr2);

// 6. Safe string functions
char dst[64];
// WRONG: strncpy does not guarantee NUL termination
strncpy(dst, src, sizeof(dst));

// RIGHT (POSIX):
strlcpy(dst, src, sizeof(dst));   // always NUL-terminates
// Or manually:
snprintf(dst, sizeof(dst), "%s", src);

// 7. const correctness — document non-mutation intent
void print_string(const char *s);

// 8. Use compiler flags
// -Wall -Wextra -Wstrict-aliasing -fsanitize=address,undefined
```

---

### 2.12 Tools: Valgrind, ASan, MSan

#### Valgrind (memcheck)

Runs the program in a virtual machine, tracking every allocation and access. Extremely thorough but 10-50x slower.

```bash
# Compile with debug info
gcc -g -O0 program.c -o program

# Run under Valgrind
valgrind --leak-check=full --show-leak-kinds=all \
         --track-origins=yes --verbose ./program

# Output for a leak:
# ==12345== 40 bytes in 1 blocks are definitely lost in loss record 1 of 2
# ==12345==    at 0x4C2FB0F: malloc (in /usr/lib/valgrind/...)
# ==12345==    by 0x10865B: main (program.c:15)
```

#### AddressSanitizer (ASan)

Compiler instrumentation. ~2x overhead. Detects:
- Heap/stack/global buffer overflow
- Use-after-free
- Use-after-return
- Double free
- Memory leaks

```bash
gcc -fsanitize=address -fno-omit-frame-pointer -g program.c -o program
./program

# Output for use-after-free:
# ==12345==ERROR: AddressSanitizer: heap-use-after-free on address 0x602000000010
# READ of size 4 at 0x602000000010 thread T0
#     #0 0x401234 in main program.c:20
```

#### MemorySanitizer (MSan)

Detects reads from uninitialized memory. Clang only.

```bash
clang -fsanitize=memory -fno-omit-frame-pointer -g program.c -o program
```

#### UndefinedBehaviorSanitizer (UBSan)

Detects integer overflow, misaligned access, null pointer dereference, etc.

```bash
gcc -fsanitize=undefined -g program.c -o program
```

---

## 3. Rust Memory Management

Rust achieves memory safety without a garbage collector through a **compile-time** ownership and borrowing system. The core insight: at compile time, Rust tracks who "owns" each piece of memory and statically proves there are no invalid accesses.

---

### 3.1 The Ownership Model

Every piece of memory in Rust has exactly one **owner** at any point in time. When the owner goes out of scope, the memory is freed. This is enforced by the compiler — there are no runtime checks.

**The Three Rules of Ownership:**
1. Each value in Rust has exactly one owner.
2. There can only be one owner at a time.
3. When the owner goes out of scope, the value is dropped (memory freed).

```rust
fn main() {
    let s = String::from("hello");  // s owns the heap-allocated string
    //                              // "hello" is on the heap, metadata on stack

    {
        let t = s;    // Ownership of "hello" MOVES to t
                      // s is no longer valid — compile error if used!
    }                 // t goes out of scope here — "hello" is freed (Drop called)

    // println!("{}", s);  // ERROR: borrow of moved value `s`
}
```

**Why one owner? Why not just reference counting?**
- Reference counting has runtime overhead (incrementing/decrementing counters).
- Single ownership enables compile-time proof of validity.
- No cycles possible (unlike RC without Weak), so no GC needed.

---

### 3.2 Move Semantics

Assigning a non-Copy type transfers ownership. The original binding is invalidated.

```rust
// Stack-allocated types (Copy): bit-for-bit copy, both remain valid
let x: i32 = 5;
let y = x;         // x is COPIED — both x and y are valid
println!("{}", x); // OK

// Heap-allocated types (non-Copy): ownership moves
let s1 = String::from("hello");
//
// Stack:           Heap:
// +-------+        +---+---+---+---+---+
// | s1    |        | h | e | l | l | o |
// | ptr --|------> +---+---+---+---+---+
// | len=5 |
// | cap=5 |
// +-------+

let s2 = s1;   // s1's stack data (ptr, len, cap) is COPIED to s2
               // But the heap data is NOT copied — s2 now owns it
               // s1 is invalidated immediately

// Stack:           Heap:
// +-------+
// | s1    |  (INVALIDATED — compiler will reject any use of s1)
// +-------+
// +-------+        +---+---+---+---+---+
// | s2    |        | h | e | l | l | o |
// | ptr --|------> +---+---+---+---+---+
// | len=5 |
// | cap=5 |
// +-------+

// println!("{}", s1);  // ERROR: use of moved value: `s1`
println!("{}", s2);     // OK
// s2 goes out of scope → heap memory freed exactly once
```

**Why this is not shallow copying (pointer aliasing):** In C, `char *s2 = s1` would give you two pointers to the same heap data. Both could free it (double free) or write to it (races). Rust's move semantics prevent both.

---

### 3.3 Copy vs Move Types

Types that implement the `Copy` trait are copied on assignment instead of moved.

```rust
// Copy types (all stack-allocated, trivially copyable)
let a: i32   = 5;
let b: f64   = 3.14;
let c: bool  = true;
let d: char  = 'x';
let e: (i32, i32) = (1, 2);   // tuple of Copy types is Copy
let f: [i32; 4]   = [1,2,3,4]; // fixed-size array of Copy types is Copy

let a2 = a;  // a is COPIED, both a and a2 are valid

// Move types (own heap data or are non-trivially droppable)
// String, Vec<T>, Box<T>, HashMap, File, Mutex, etc.
```

**Rules for Copy:**
- A type is `Copy` only if all its fields are `Copy`.
- A type cannot be `Copy` if it implements `Drop` (they are mutually exclusive).
- Primitives (i32, u64, f32, bool, char, &T, *const T, *mut T) are `Copy`.

---

### 3.4 Borrowing and References

Instead of transferring ownership, you can **borrow** a value: create a temporary reference to it.

There are two kinds of references:
- `&T` — shared reference (immutable borrow)
- `&mut T` — exclusive reference (mutable borrow)

**Borrow rules (enforced by the borrow checker):**
1. You can have any number of `&T` references, OR
2. Exactly one `&mut T` reference.
3. Never both at the same time.
4. References must always be valid (no dangling references).

```rust
fn main() {
    let s = String::from("hello");

    // Shared reference — does NOT take ownership
    let len = calculate_length(&s);   // pass reference
    println!("'{}' has length {}", s, len);  // s still valid

    // Mutable reference
    let mut s2 = String::from("hello");
    change(&mut s2);
    println!("{}", s2);  // "hello, world"
}

fn calculate_length(s: &String) -> usize {
    s.len()
    // s goes out of scope here, but we don't own it, so nothing is dropped
}

fn change(s: &mut String) {
    s.push_str(", world");
}
```

**Reference memory layout:**

```
                    Stack                      Heap
fn main:
    s               +----------+
    (String)        | ptr -----+---------> [h][e][l][l][o]
                    | len=5    |
                    | cap=5    |
                    +----------+

fn calculate_length:
    s               +----------+
    (&String)       | ptr -----+----> points to s in main's stack frame
                    +----------+      (a reference to a String, not the String itself)
```

#### Preventing data races at compile time

```rust
let mut v = vec![1, 2, 3];

let r1 = &v;      // shared ref 1
let r2 = &v;      // shared ref 2 — OK, multiple shared refs allowed
println!("{:?} {:?}", r1, r2);  // r1 and r2 used here, their scope ends here

// Now mutable ref is allowed (r1 and r2 no longer in use):
let r3 = &mut v;  // exclusive mutable ref — OK
r3.push(4);

// But this would fail:
// let r4 = &v;    // ERROR: cannot borrow as immutable while mutable borrow exists
```

---

### 3.5 The Borrow Checker

The borrow checker is a compile-time static analysis pass in the Rust compiler (rustc) that enforces the ownership and borrowing rules. It operates on a control-flow graph of the function and tracks lifetimes.

**How it works conceptually:**

```rust
fn main() {
    let r;                   // ---- 'r lifetime starts

    {
        let x = 5;           // -------- 'x lifetime starts
        r = &x;              // r borrows x — but x's lifetime is shorter than r's!
    }                        // -------- 'x lifetime ends — x is dropped
                             //           r would now be a dangling reference!

    println!("{}", r);       // ERROR caught here by borrow checker
}

// Compiler error:
// error[E0597]: `x` does not live long enough
//   --> src/main.rs:6:13
//    |
// 5  |         let x = 5;
//    |             - binding `x` declared here
// 6  |         r = &x;
//    |             ^^ borrowed value does not live long enough
// 7  |     }
//    |     - `x` dropped here while still borrowed
// 8  |
// 9  |     println!("{}", r);
//    |                    - borrow later used here
```

---

### 3.6 Lifetimes

Lifetimes are the borrow checker's way of tracking how long a reference is valid. Most of the time they are inferred (elided). When the compiler can't figure them out, you annotate them explicitly.

```rust
// Without lifetime annotation, this won't compile:
// fn longest(x: &str, y: &str) -> &str { ... }
// The compiler doesn't know if the return borrows from x or y.

// With explicit lifetime annotation:
fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {
    //      ^^^  ^^^           ^^^           ^^^
    // 'a is a lifetime parameter.
    // "x and y both live at least as long as 'a,
    //  and the return value lives at most as long as 'a"

    if x.len() > y.len() { x } else { y }
}

fn main() {
    let string1 = String::from("long string");
    let result;
    {
        let string2 = String::from("xyz");
        result = longest(string1.as_str(), string2.as_str());
        println!("Longest: {}", result);  // OK — result used while both strings live
    }
    // println!("{}", result);  // ERROR if here — string2 dropped
}
```

**Lifetime in structs:**

```rust
// This struct holds a reference, so it needs a lifetime annotation
struct Excerpt<'a> {
    part: &'a str,   // "the reference 'part' lives at least as long as 'a"
}

fn main() {
    let novel = String::from("Call me Ishmael. Some years ago...");
    let first_sentence;
    {
        let i = novel.find('.').unwrap_or(novel.len());
        first_sentence = &novel[..i];
    }
    let excerpt = Excerpt { part: first_sentence };
    println!("{}", excerpt.part);  // OK — novel still lives
}
```

---

### 3.7 Lifetime Elision Rules

You don't always write lifetimes explicitly. The compiler applies three elision rules:

1. Each reference parameter gets its own lifetime parameter.
2. If there is exactly one input lifetime, it is assigned to all output lifetimes.
3. If there is a `&self` or `&mut self` parameter, its lifetime is assigned to all output lifetimes.

```rust
// These are equivalent:
fn first_word(s: &str) -> &str { ... }
fn first_word<'a>(s: &'a str) -> &'a str { ... }  // rule 2 applied

// Rule 3:
impl MyStruct {
    fn get_name(&self) -> &str { &self.name }
    // Equivalent to:
    fn get_name<'a>(&'a self) -> &'a str { &self.name }
}
```

---

### 3.8 RAII and the Drop Trait

RAII (Resource Acquisition Is Initialization): resources are tied to object lifetimes. When the object is destroyed, the resource is released.

The `Drop` trait's `drop` method is called automatically when a value goes out of scope.

```rust
struct MyFile {
    name: String,
    data: Vec<u8>,
}

impl Drop for MyFile {
    fn drop(&mut self) {
        println!("Dropping file: {}", self.name);
        // Flush, close, cleanup — automatically called on scope exit
    }
}

fn main() {
    let f = MyFile {
        name: String::from("log.txt"),
        data: vec![1, 2, 3],
    };
    println!("File created");
    // f goes out of scope here → Drop::drop called → "Dropping file: log.txt"
    // f.data (Vec) is also dropped → heap memory freed
    // f.name (String) is also dropped → heap memory freed
}
```

**Drop order:** Fields are dropped in declaration order. Struct dropped before its fields. Stack variables dropped in reverse order of declaration (LIFO).

```rust
fn main() {
    let a = String::from("A");  // dropped 3rd
    let b = String::from("B");  // dropped 2nd
    let c = String::from("C");  // dropped 1st  (LIFO)
}
// Drop order: C, B, A

struct Outer {
    first:  String,  // dropped 1st (declaration order)
    second: String,  // dropped 2nd
}
// Outer itself dropped, THEN its fields in order
```

**Manual early drop:**
```rust
let s = String::from("hello");
drop(s);   // explicit early drop — s is freed here
// println!("{}", s);  // ERROR: use of moved value
```

---

### 3.9 Smart Pointers

Smart pointers add heap allocation, reference counting, or other behaviors on top of raw ownership.

#### `Box<T>` — Simple Heap Allocation

```rust
// Puts T on the heap. Single owner. Zero overhead at runtime vs raw pointer.
let b = Box::new(5);  // i32 allocated on heap
println!("b = {}", *b);  // dereference with *
// b goes out of scope → heap memory freed

// Use cases:
// 1. Recursive types (can't have infinite size on stack)
enum List {
    Cons(i32, Box<List>),  // Box breaks the infinite recursion
    Nil,
}
// 2. Large data you don't want to copy
// 3. Trait objects (dynamic dispatch)
let trait_obj: Box<dyn std::fmt::Display> = Box::new(42);
```

**Memory layout of Box<T>:**
```
Stack:                    Heap:
+-----------+             +-----------+
| b         |             |     5     |
| ptr ------+-----------> +-----------+
+-----------+
(Box<i32> = 8 bytes)      (i32 = 4 bytes)
```

#### `Rc<T>` — Reference Counted (Single-Threaded)

Allows multiple owners. Freed when last owner drops.

```rust
use std::rc::Rc;

let a = Rc::new(String::from("hello"));
// Rc layout:
//
// Stack:          Heap (RcBox):
// +--------+      +------------+-------+
// | a      |      | strong = 1 | weak=0|  <-- reference count
// | ptr ---+----> +------------+-------+
// +--------+      |   "hello"          |  <-- actual data
//                 +--------------------+

let b = Rc::clone(&a);   // Increments strong count to 2, does NOT clone data
let c = Rc::clone(&a);   // Strong count = 3

println!("strong count: {}", Rc::strong_count(&a));  // 3

drop(b);   // Strong count = 2
drop(c);   // Strong count = 1
// drop(a) → Strong count = 0 → data freed
```

`Rc` is NOT thread-safe. Use `Arc` for multi-threaded code.

#### `Arc<T>` — Atomic Reference Counted (Thread-Safe)

```rust
use std::sync::Arc;
use std::thread;

let data = Arc::new(vec![1, 2, 3]);

let data2 = Arc::clone(&data);
let handle = thread::spawn(move || {
    println!("Thread sees: {:?}", data2);
});

println!("Main sees: {:?}", data);
handle.join().unwrap();
// Both threads safely reference the same Vec
// Atomic operations ensure correct ref count despite concurrent updates
```

`Arc` uses atomic operations (like CAS) for the reference count, which has slightly more overhead than `Rc`.

#### `Weak<T>` — Weak References

Breaks reference cycles in `Rc/Arc`. Does not prevent deallocation.

```rust
use std::rc::{Rc, Weak};
use std::cell::RefCell;

// Classic reference cycle problem:
struct Node {
    value: i32,
    children: Vec<Rc<RefCell<Node>>>,
    parent: Option<Weak<RefCell<Node>>>,  // Weak to prevent cycle
}
```

---

### 3.10 Interior Mutability

Normally Rust enforces: if you have a `&T` reference, you cannot mutate through it. Interior mutability is a design pattern that allows mutation through shared references, using runtime checks instead of compile-time checks.

#### `Cell<T>` — For Copy types, no runtime overhead

```rust
use std::cell::Cell;

let x = Cell::new(5);
let y = &x;  // shared reference

// You CAN mutate through a shared reference via Cell
y.set(10);
println!("{}", x.get());  // 10
```

#### `RefCell<T>` — For non-Copy types, runtime borrow checking

```rust
use std::cell::RefCell;

let v = RefCell::new(vec![1, 2, 3]);

// borrow() returns Ref<T> — shared reference at runtime
let r1 = v.borrow();
let r2 = v.borrow();   // OK — multiple shared borrows allowed
println!("{:?}", *r1);

drop(r1); drop(r2);   // Must release before taking mutable borrow

// borrow_mut() returns RefMut<T> — exclusive mutable reference at runtime
v.borrow_mut().push(4);  // OK — no other active borrows

// This would PANIC at runtime (not compile time):
// let _r = v.borrow();
// let _rw = v.borrow_mut();  // PANIC: already borrowed
```

**RefCell enforces the same rules as the borrow checker, but at runtime (panics instead of compile errors).** Use it when you know you're right but the compiler can't verify it statically.

#### Common pattern: `Rc<RefCell<T>>` for shared mutable ownership

```rust
use std::rc::Rc;
use std::cell::RefCell;

let shared = Rc::new(RefCell::new(vec![1, 2, 3]));
let clone1 = Rc::clone(&shared);
let clone2 = Rc::clone(&shared);

clone1.borrow_mut().push(4);
clone2.borrow_mut().push(5);

println!("{:?}", shared.borrow());  // [1, 2, 3, 4, 5]
```

---

### 3.11 Memory Layout in Rust

#### `std::mem::size_of` and `std::mem::align_of`

```rust
use std::mem;

println!("{}", mem::size_of::<i32>());       // 4
println!("{}", mem::size_of::<i64>());       // 8
println!("{}", mem::size_of::<bool>());      // 1
println!("{}", mem::size_of::<String>());    // 24 (ptr + len + cap on 64-bit)
println!("{}", mem::size_of::<Box<i32>>()); // 8 (just a pointer)
println!("{}", mem::size_of::<Option<Box<i32>>>());  // 8! (null pointer optimization)
```

**Null Pointer Optimization (NPO):** `Option<Box<T>>` has the same size as `Box<T>` because `None` is represented as a null pointer. The compiler uses invalid states as discriminants.

#### Struct layout

By default, Rust may reorder struct fields for optimal packing (unlike C which preserves declaration order):

```rust
struct DefaultLayout {
    a: bool,  // 1 byte
    b: i32,   // 4 bytes
    c: u8,    // 1 byte
}
// Rust may reorder to: b (4), a (1), c (1), [2 padding] = 8 bytes

#[repr(C)]  // Force C-compatible layout (declaration order)
struct CLayout {
    a: bool,  // 1 + 3 padding
    b: i32,   // 4
    c: u8,    // 1 + 7 padding
}
// Total: 16 bytes (C layout)

#[repr(packed)]  // Remove padding (may cause misaligned access)
struct Packed {
    a: bool,
    b: i32,
    c: u8,
}
// Total: 6 bytes (but misaligned — slow or UB on some platforms)
```

#### Fat pointers

References to dynamically-sized types (DSTs) are "fat pointers" — two-word values containing a data pointer and metadata.

```rust
// &str: pointer to UTF-8 bytes + length
// &[T]: pointer to elements + length
// &dyn Trait: pointer to data + pointer to vtable

// Thin pointer (one word = 8 bytes):
let p: &i32 = &5;

// Fat pointer (two words = 16 bytes):
let s: &str = "hello";    // ptr to 'h' + length 5
let sl: &[i32] = &[1,2,3]; // ptr to array + length 3
// &dyn Trait: (ptr to data, ptr to vtable with method pointers)
```

#### `PhantomData<T>`

A zero-size marker type that tells the compiler a type "logically" contains a T, affecting variance, Send/Sync, and Drop checks.

```rust
use std::marker::PhantomData;

struct MyVec<T> {
    ptr: *mut T,
    len: usize,
    cap: usize,
    _marker: PhantomData<T>,  // "This struct logically owns T values"
}
// Without PhantomData, rustc won't know to apply Drop checking for T
```

---

### 3.12 `Pin<T>` and `Unpin`

`Pin<P>` prevents the data pointed to by pointer `P` from being moved in memory. This is critical for async/await and self-referential structs.

```rust
use std::pin::Pin;

// Problem: async state machines can have self-referential pointers
// (a future holding a reference to a local variable in itself).
// Moving such a struct would invalidate the pointer.

// Pin guarantees: once pinned, the value at the pointer won't be moved.

use std::future::Future;

async fn async_func() {
    let local = 5;
    // The compiler generates a state machine struct.
    // Some states may hold &local.
    // Pin ensures the struct isn't moved while awaited.
    some_async_operation().await;
    println!("{}", local);
}

// Most types are Unpin (can be freely moved even when pinned):
// i32, String, Vec<T> — all Unpin
// !Unpin types: async fn return types, self-referential structs
```

---

### 3.13 Unsafe Rust and Raw Pointers

Unsafe Rust allows you to opt out of memory safety guarantees when you need to interface with C, build low-level abstractions, or do things the borrow checker can't verify.

```rust
fn main() {
    let x = 5;

    // Raw pointers: *const T (read-only) and *mut T (read-write)
    let p: *const i32 = &x;          // Create from reference
    let mp: *mut i32 = &mut 5 as *mut i32; // Mutable raw pointer

    // Can create raw pointers anywhere — safe
    // But dereferencing requires unsafe block:
    unsafe {
        println!("{}", *p);  // Dereference — possibly unsafe
    }

    // Calling C functions
    extern "C" {
        fn abs(input: i32) -> i32;  // Declaration of C function
    }
    unsafe {
        println!("{}", abs(-5));  // Calling foreign function
    }

    // Unsafe allows:
    // 1. Dereference raw pointers
    // 2. Call unsafe functions/methods
    // 3. Access or modify mutable static variables
    // 4. Implement unsafe traits
    // 5. Access fields of unions
}
```

**Raw pointer arithmetic:**

```rust
unsafe {
    let arr: [i32; 5] = [1, 2, 3, 4, 5];
    let ptr: *const i32 = arr.as_ptr();

    // Advance pointer: pointer.add(n) == pointer + n * sizeof(T)
    let third = ptr.add(2);
    println!("{}", *third);  // 3

    // Pointer difference: ptr.offset_from()
    let p1 = arr.as_ptr();
    let p5 = arr.as_ptr().add(4);
    let diff = p5.offset_from(p1);  // 4
}
```

**`std::alloc` — Manual allocation in Rust:**

```rust
use std::alloc::{alloc, dealloc, Layout};

unsafe {
    let layout = Layout::array::<i32>(10).unwrap();  // Layout for [i32; 10]

    let ptr = alloc(layout) as *mut i32;
    if ptr.is_null() { panic!("allocation failed"); }

    // Write to allocated memory
    for i in 0..10 {
        ptr.add(i).write(i as i32);
    }

    // Read back
    println!("{}", *ptr.add(5));  // 5

    // Deallocate
    dealloc(ptr as *mut u8, layout);
}
```

---

### 3.14 Custom Allocators

Rust allows replacing the global allocator via the `GlobalAlloc` trait.

```rust
use std::alloc::{GlobalAlloc, Layout, System};
use std::sync::atomic::{AtomicUsize, Ordering};

struct CountingAllocator {
    allocated: AtomicUsize,
}

unsafe impl GlobalAlloc for CountingAllocator {
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        let ptr = System.alloc(layout);
        if !ptr.is_null() {
            self.allocated.fetch_add(layout.size(), Ordering::Relaxed);
        }
        ptr
    }

    unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
        System.dealloc(ptr, layout);
        self.allocated.fetch_sub(layout.size(), Ordering::Relaxed);
    }
}

#[global_allocator]
static ALLOCATOR: CountingAllocator = CountingAllocator {
    allocated: AtomicUsize::new(0),
};

fn main() {
    let v: Vec<i32> = vec![1, 2, 3, 4, 5];
    println!("Allocated: {} bytes", ALLOCATOR.allocated.load(Ordering::Relaxed));
    drop(v);
    println!("Allocated after drop: {} bytes", ALLOCATOR.allocated.load(Ordering::Relaxed));
}
```

---

## 4. Go Memory Management

Go takes a different approach: **automatic garbage collection** with **escape analysis** to minimize GC pressure. The programmer doesn't manually manage memory, but understanding Go's GC, escape analysis, and allocator is crucial for writing high-performance Go code.

---

### 4.1 Go's Memory Philosophy

Go aims for simplicity: you allocate with `new` or composite literals, and the runtime reclaims memory. But it's not naive — Go's runtime includes:

1. A **per-thread (P) cache** allocator inspired by tcmalloc for fast small allocations.
2. **Escape analysis** at compile time to keep as much on the stack as possible.
3. A **concurrent tri-color mark-and-sweep GC** for heap memory.
4. **Dynamically growing goroutine stacks** (unlike C/Rust fixed thread stacks).

```go
package main

import "fmt"

func main() {
    // Stack allocated (escape analysis determines this):
    x := 5
    arr := [3]int{1, 2, 3}

    // Heap allocated (slice header on stack, backing array on heap):
    s := make([]int, 10, 20)

    // Heap allocated (new returns a pointer):
    p := new(int)
    *p = 42

    fmt.Println(x, arr, s, *p)
    // When main returns, GC will eventually collect p and s's backing array
}
```

---

### 4.2 Goroutine Stacks

One of Go's most important innovations: goroutines start with tiny stacks (currently **2KB** to **8KB**) that grow dynamically.

```
Goroutine G1                    Goroutine G2
+------------------+            +------------------+
| 2KB initial stack|            | 2KB initial stack|
|   main()         |            |   worker()       |
|   foo()          |            |   ...            |
+------------------+            +------------------+
        |
        | Stack needs to grow
        v
+------------------+  <-- runtime allocates new contiguous stack (2x size)
| 4KB new stack    |      copies all frames from old stack to new
|   main()  (copy) |      updates all pointers into the stack
|   foo()   (copy) |      frees old stack
|   bar()          |
+------------------+

(This is "copyable/contiguous stacks", introduced in Go 1.4
replacing the older "segmented stacks" / "split stacks" approach)
```

**Why this matters:**
- With OS threads, each thread needs a large stack pre-allocated (~1-8MB).
- With goroutines' tiny starting stack, you can have **millions of goroutines**.
- The stack grows on demand, meaning deep recursion is safe (up to memory limits).
- Stack shrinking also occurs: if a goroutine's stack is mostly idle, the GC can shrink it.

**Stack growth trigger (stack overflow check):**
The compiler inserts a check at each function entry point. If the remaining stack space is too small for the function's requirements, `runtime.morestack` is called to grow the stack.

---

### 4.3 Escape Analysis

Escape analysis is Go's compile-time analysis that determines whether a variable can live on the stack or must "escape" to the heap.

**A variable escapes to the heap when:**
1. Its address is returned from a function.
2. It is stored in an interface value.
3. It is assigned to a heap variable.
4. It is too large for the stack.
5. It is captured by a closure that outlives the function.
6. It is sent to a channel (sometimes).

```go
package main

// NOT escaped — lives on stack
func noEscape() int {
    x := 42     // x stays on stack
    return x    // returned by VALUE, copy made
}

// ESCAPED — lives on heap
func escaped() *int {
    x := 42     // x must escape to heap — its address is returned
    return &x   // caller will hold this pointer after function returns
}

// ESCAPED via interface
func escapeViaInterface(v interface{}) {
    // v's concrete value is on the heap because
    // interfaces store a pointer to data + type pointer
}

// ESCAPED — too large for stack (approximate threshold ~32KB)
func largeAlloc() {
    var buf [1 << 20]byte  // 1MB — definitely on heap
    _ = buf
}
```

**Inspecting escape analysis:**
```bash
go build -gcflags="-m -m" main.go

# Output examples:
# ./main.go:10:2: x escapes to heap
# ./main.go:15:6: &x escapes to heap
# ./main.go:20:2: buf does not escape
```

**Performance implications:**
- Stack allocation: ~1 ns (just adjust stack pointer).
- Heap allocation: ~100 ns (allocator bookkeeping, possible GC pressure).

**Reducing escapes for performance:**
```go
// BAD: causes escape because []byte is heap-allocated
func processData(data string) []byte {
    result := make([]byte, len(data))
    copy(result, data)
    return result
}

// BETTER: caller provides buffer (no escape)
func processDataInto(data string, buf []byte) []byte {
    n := copy(buf, data)
    return buf[:n]
}

// Also: passing arrays by pointer to avoid copy
// without causing escape when the pointer doesn't escape:
func sum(arr *[100]int) int {
    total := 0
    for _, v := range arr { total += v }
    return total
}
```

---

### 4.4 Go's Memory Allocator Internals

Go uses an allocator based on **tcmalloc** (Thread-Caching Malloc), adapted for GC integration.

#### Key structures

```
+-------------------------------------------+
|                  mheap                     |
|  (central heap, manages memory from OS)    |
|                                            |
|  +----------+  +----------+  +----------+ |
|  | mcentral |  | mcentral |  | mcentral | |  <- one per size class
|  | size= 8  |  | size=16  |  | size=...  | |
|  +----------+  +----------+  +----------+ |
+-------------------------------------------+
         |                |
         |                |
+--------+-+         +----+----+
| P (processor)|     | P (processor)|
| +----------+ |     | +----------+ |
| |  mcache  | |     |  |  mcache  | |
| | size=8:  | |     |  | size=8:  | |
| | [spans]  | |     |  | [spans]  | |
| | size=16: | |     |  | size=16: | |
| | [spans]  | |     |  | [spans]  | |
| +----------+ |     |  +----------+ |
+--------------+     +--------------+
```

#### Size classes

Go has 67 size classes (as of Go 1.21), from 8 bytes to 32KB. Allocations are rounded up to the nearest size class.

```
Size class examples:
  1:    8 bytes
  2:   16 bytes
  3:   24 bytes
  4:   32 bytes
  ...
 32:  512 bytes
 ...
 67: 32768 bytes (32KB)

Objects > 32KB: allocated directly from mheap as "large" spans
```

#### mspan

An mspan is a contiguous run of memory pages (1 page = 8KB in Go) holding objects of one size class.

```
mspan (size class 32 = 512 bytes, 2 pages = 16384 bytes):

+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+------+
| obj  | obj  | obj  | obj  | obj  | ...  | obj  |
| 0    | 1    | 2    | 3    | 4    |      | 31   |
+------+------+------+------+------+------+------+

freeindex: tracks the next free object
allocBits: bitmap of which objects are allocated
gcmarkBits: bitmap used by GC to track live objects
```

#### Allocation flow

```
Goroutine calls make([]byte, 64)
         |
         v
1. Determine size class: 64 bytes -> size class 6 (64 bytes exactly)
         |
         v
2. Check P's mcache for size class 6
   - If available: take from mcache (no lock needed)
         |
         v
3. If mcache is empty: fetch from mcentral (needs lock)
         |
         v
4. If mcentral is empty: allocate new mspan from mheap (system lock)
         |
         v
5. If mheap has no memory: call OS (mmap/sbrk) to get more pages
```

---

### 4.5 The Garbage Collector

Go's GC is a **concurrent, tri-color mark-and-sweep** GC with a write barrier. "Concurrent" means most GC work runs alongside your application goroutines, not stopping the world.

**Goals:**
- Sub-millisecond STW (stop-the-world) pauses.
- Throughput: minimize CPU time spent on GC.
- Memory: minimize overhead above live heap.

---

### 4.6 Tri-Color Mark-and-Sweep

Objects are conceptually colored: white, gray, or black.

```
Initial state:     All objects are WHITE (potentially garbage)

 [A] --> [B] --> [C]
 [D]     [E]

Roots (globals, goroutine stacks) are initially GRAY:
 
 [A*]                  * = gray
 
GC invariant: A BLACK object may NOT point to a WHITE object.
              (if it does, the write barrier turns the white object gray)

Mark phase:
 Step 1: Pop gray object A, make it BLACK, color its children GRAY:
         [A] (black) --> [B*] --> [C]
         [D] [E]
 
 Step 2: Pop gray B, make it BLACK, color C GRAY:
         [A]--> [B]--> [C*]
 
 Step 3: C has no children, make it BLACK:
         [A]--> [B]--> [C]   All reachable objects are BLACK
         [D] [E]              D, E are still WHITE (garbage)

Sweep phase:
 Free all WHITE objects (D and E).
 Reset BLACK objects to WHITE for next cycle.
```

**Invariant enforced by write barrier:** If a black object gets a pointer to a white object (via assignment), the write barrier turns the white object gray. This prevents the GC from mistakenly collecting a live object.

---

### 4.7 Write Barriers

A write barrier is code injected by the compiler around pointer writes to maintain the GC invariant during concurrent marking.

```go
// When you write:
a.field = b

// The compiler generates approximately:
//   writeBarrier(a.field, b)
//   a.field = b

// The write barrier (Dijkstra insertion barrier, simplified):
func writeBarrier(slot *interface{}, new interface{}) {
    if gcphase == _GCmark {
        // If new value is white, shade it gray
        shade(new)
    }
}
```

**Performance cost:** Write barriers add overhead to pointer writes. This is why GC languages have higher constant factors for pointer-heavy code. Go's write barrier is designed to be very cheap (a few instructions).

---

### 4.8 GC Phases and Pacing

```
Timeline:
                    GC cycle 1                           GC cycle 2
+---------------+----------------+-----+-------+--------+----------------+
|   Mutator     | Mark Setup STW | Concurrent  | Finish |   Mutator      |
|   (your code) |   (stop world) |    Mark     | STW    |   (your code)  |
+---------------+----------------+-----+-------+--------+----------------+
                |<---STW ~50µs-->|<-- concurrent -->|STW|
```

1. **GC Off**: Normal execution, allocating objects.
2. **Mark Setup (STW)**: Briefly pause ALL goroutines. Enable write barrier. Scan goroutine stacks for root pointers. Resume goroutines.
3. **Concurrent Mark**: GC goroutines run concurrently with application, marking reachable objects using tri-color algorithm.
4. **Mark Termination (STW)**: Briefly pause ALL goroutines. Finalize marking (handle anything that changed during concurrent mark). Disable write barrier.
5. **Concurrent Sweep**: Freed pages swept back into allocator concurrently.

**GC pacer:** The GC is triggered when the heap reaches `target_size = live_heap * (1 + GOGC/100)`. With `GOGC=100` (default), GC triggers when heap doubles from the previous live set.

```
Live heap after GC: 50 MB
GOGC: 100
Next GC trigger: 50 * (1 + 100/100) = 100 MB

If GOGC=200:
Next GC trigger: 50 * (1 + 200/100) = 150 MB  (less frequent GC, more memory)

If GOGC=25:
Next GC trigger: 50 * (1 + 25/100) = 62.5 MB  (more frequent GC, less memory)
```

---

### 4.9 sync.Pool

`sync.Pool` is a pool of temporary objects that can be reused to reduce GC pressure. The pool is cleared during GC cycles.

```go
package main

import (
    "bytes"
    "sync"
)

var bufPool = sync.Pool{
    New: func() interface{} {
        return &bytes.Buffer{}
    },
}

func processRequest(data []byte) []byte {
    // Get from pool (or allocate new if pool is empty)
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()  // IMPORTANT: always reset before use

    defer func() {
        // Return to pool when done — prevents GC pressure
        bufPool.Put(buf)
    }()

    buf.Write(data)
    // ... process ...
    result := make([]byte, buf.Len())
    copy(result, buf.Bytes())
    return result
}

// Without sync.Pool: every request allocates a new Buffer → GC pressure
// With sync.Pool: buffers are reused → much less GC work
```

**Important:** `sync.Pool` objects are cleared on every GC cycle. Don't use it for persistent state.

---

### 4.10 GC Tuning: GOGC and GOMEMLIMIT

#### GOGC (Go GC percent)

```go
import "runtime/debug"

// Via environment variable:
// GOGC=100 ./myapp     (default: GC when heap doubles)
// GOGC=off ./myapp     (disable GC — for benchmarking only)

// Programmatically:
debug.SetGCPercent(200)  // More memory, less GC
debug.SetGCPercent(50)   // Less memory, more GC
debug.SetGCPercent(-1)   // Disable GC
```

#### GOMEMLIMIT (Go 1.19+)

Caps the total memory the Go runtime will use. Prevents OOM kills in containers.

```go
import "runtime/debug"

// Via environment variable:
// GOMEMLIMIT=512MiB ./myapp

// Programmatically:
debug.SetMemoryLimit(512 * 1024 * 1024)  // 512 MB

// When GOMEMLIMIT is set, the GC becomes more aggressive
// as the heap approaches the limit, even ignoring GOGC.
// This is the "ballast" approach alternative.
```

#### runtime.GC()

Force an immediate GC cycle:
```go
import "runtime"

runtime.GC()  // Useful in tests or after loading large data you know is now dead
```

---

### 4.11 Memory Leaks in Go

Go can still have "logical" memory leaks — objects that are technically reachable by the GC (so not collected) but no longer needed by the application.

#### Goroutine leaks

```go
// LEAK: goroutine blocked forever because nobody reads the channel
func leakyFunction() {
    ch := make(chan int)  // unbuffered channel
    go func() {
        result := expensiveComputation()
        ch <- result  // blocked forever if nobody reads ch
    }()
    // return without reading ch — goroutine is leaked!
    // The goroutine (and its stack) stays in memory forever
}

// FIX: use context for cancellation
func fixedFunction(ctx context.Context) {
    ch := make(chan int, 1)  // buffered: goroutine can always send
    go func() {
        select {
        case <-ctx.Done():
            return  // cancelled — goroutine exits
        case ch <- expensiveComputation():
        }
    }()
    // ... use ch or cancel context
}
```

#### Map accumulation

```go
// LEAK: map keeps growing, old entries never deleted
type Cache struct {
    data map[string][]byte
    mu   sync.Mutex
}

func (c *Cache) Store(key string, value []byte) {
    c.mu.Lock()
    c.data[key] = value  // keeps growing — never evicts old entries
    c.mu.Unlock()
}

// FIX: use TTL, LRU eviction, or a library like github.com/hashicorp/golang-lru
```

#### Slice backing array retention

```go
// LEAK: keeping reference to large slice via small subslice
func loadFileHeaders(filename string) []byte {
    data, _ := os.ReadFile(filename)  // loads entire file (e.g., 1GB)
    header := data[:100]              // subslice shares backing array!
    return header                     // returns 100 bytes, but 1GB stays in memory!
}

// FIX: copy the data you need
func loadFileHeadersFix(filename string) []byte {
    data, _ := os.ReadFile(filename)
    header := make([]byte, 100)
    copy(header, data[:100])
    // data is no longer referenced — GC can collect it
    return header
}
```

#### Interface boxing of large structs

```go
// Every time you assign a large struct to an interface,
// Go may heap-allocate a copy.
type BigStruct struct { data [1000]int }

func process(v interface{}) { /* ... */ }

s := BigStruct{}
process(s)  // s's data is copied to heap for boxing

// FIX: use pointer to large struct with interface
process(&s)  // only pointer is boxed — no copy of BigStruct
```

---

### 4.12 Memory Profiling with pprof

```go
package main

import (
    "net/http"
    _ "net/http/pprof"  // registers /debug/pprof/ handlers
    "runtime"
    "runtime/pprof"
    "os"
)

func main() {
    // Option 1: HTTP endpoint (live profiling)
    go func() {
        http.ListenAndServe(":6060", nil)
    }()
    // Then: go tool pprof http://localhost:6060/debug/pprof/heap

    // Option 2: File-based profiling
    f, _ := os.Create("mem.prof")
    defer func() {
        runtime.GC()  // get up-to-date statistics
        pprof.WriteHeapProfile(f)
        f.Close()
    }()

    // ... run your program ...
}

// Analyze with:
// go tool pprof mem.prof
// (pprof) top10
// (pprof) web   (shows graph in browser)
// (pprof) list functionName

// For allocation profiling (who allocates most):
// go test -memprofile mem.out -bench BenchmarkMyFunc
// go tool pprof mem.out
```

**Runtime memory statistics:**

```go
var m runtime.MemStats
runtime.ReadMemStats(&m)

fmt.Printf("Heap alloc:     %d KB\n", m.HeapAlloc/1024)
fmt.Printf("Heap total:     %d KB\n", m.HeapSys/1024)
fmt.Printf("Heap in use:    %d KB\n", m.HeapInuse/1024)
fmt.Printf("Heap released:  %d KB\n", m.HeapReleased/1024)
fmt.Printf("Stack in use:   %d KB\n", m.StackInuse/1024)
fmt.Printf("GC cycles:      %d\n", m.NumGC)
fmt.Printf("Total alloc:    %d KB\n", m.TotalAlloc/1024)
fmt.Printf("Mallocs:        %d\n", m.Mallocs)
fmt.Printf("Frees:          %d\n", m.Frees)
fmt.Printf("Last GC:        %v\n", time.Unix(0, int64(m.LastGC)))
fmt.Printf("GC pause total: %v\n", time.Duration(m.PauseTotalNs))
```

---

## 5. Comparison and Mental Models

### 5.1 Memory Safety Guarantees

```
                  Memory Safety Property:
                  +---------+------------+------------+
                  |         |  Prevents  |  Prevents  |
Language          | Errors  |  at Compile| at Runtime |
+------------------+---------+------------+------------+
| C                | None    |    No      |    No      |
|                  | (manual)| (optional  |            |
|                  |         |  tools)    |            |
+------------------+---------+------------+------------+
| Rust             | Many    |    Yes     |  for safe  |
|                  | (owner- |  (borrow   |  Rust only |
|                  |  ship)  |   checker) |            |
+------------------+---------+------------+------------+
| Go               | Limited |    No      |    Yes     |
|                  | (GC)    |  (escape   |  (GC, nil  |
|                  |         |   analysis)|  checks)  |
+------------------+---------+------------+------------+
```

### 5.2 What Each Language Catches

```
Bug Type               C        Rust      Go
--------------------- -------  --------  --------
Use-after-free         ❌ UB    ✅ Compile  ✅ GC
Double-free            ❌ UB    ✅ Compile  N/A (GC)
Memory leak            ❌ None  ✅ (RAII)  ✅ GC (logical leaks possible)
Buffer overflow        ❌ UB    ✅ Compile  ✅ Runtime panic
Null pointer deref     ❌ Crash ✅ No null ✅ Runtime panic
Data race              ❌ UB    ✅ Compile  ✅ Race detector (optional)
Dangling pointer       ❌ UB    ✅ Compile  N/A (GC)
Stack overflow         ❌ SEGV  ❌ SEGV    ✅ Grows stack
Integer overflow       ❌ UB/wrap❌ wrap*  ❌ wrap
Uninitialized read     ❌ UB    ✅ Compile  ✅ Zero-initialized
```
*Rust: debug mode panics on overflow, release mode wraps

### 5.3 Allocation Model Comparison

```
Allocation:
     C              Rust               Go
+----------+    +----------+      +----------+
| malloc() |    | Box::new |      | make()   |
|          |    | Vec::new |      | new()    |
|          |    | String:: |      | &T{}     |
|          |    | from()   |      |          |
+----------+    +----------+      +----------+
     |                |               |
     v                v               v
  Heap           Heap (Box)        Heap OR Stack
  always         Stack (local)     (escape analysis
                 sometimes         decides)

Deallocation:
     C              Rust               Go
+----------+    +----------+      +----------+
| free()   |    | Drop     |      | GC       |
| manual   |    | automatic|      | automatic|
|          |    | (scope)  |      | (cycles) |
+----------+    +----------+      +----------+
```

### 5.4 The Three Mental Models

#### C Mental Model: "You own everything, the machine trusts you"
```
You are the allocator manager.
Every malloc has exactly one matching free.
You must draw the ownership graph yourself (in your head or comments).
No guardrails — bugs are silent and dangerous.
Maximum control, maximum responsibility.

Good for: OS kernels, embedded systems, performance-critical
          inner loops, interfacing with hardware.
```

#### Rust Mental Model: "Ownership graph enforced at compile time"
```
Every value has exactly one owner (a variable).
Ownership can be transferred (moved) or temporarily lent (borrowed).
References (&T, &mut T) are leases — the compiler tracks when they expire.
When the owner drops, the resource is freed. Automatically. Exactly once.

The compiler IS your memory safety proof.
If it compiles, there are no use-after-free, double-free, or data races.

Good for: systems programming where you need C-level performance
          but cannot afford C-level bugs: browsers, game engines,
          databases, network services.
```

#### Go Mental Model: "Just allocate; trust the GC"
```
Create values freely. The GC will clean up.
The runtime decides stack vs heap (escape analysis).
Your job: avoid logical leaks (goroutine leaks, map accumulation).
Tune GC if needed (GOGC, GOMEMLIMIT, sync.Pool).

Good for: server-side applications, CLIs, microservices,
          DevOps tools. Prioritizes developer velocity.
```

---

### 5.5 Performance Characteristics

```
Operation             C            Rust          Go
--------------------  -----------  -----------   ---------
Stack allocation      ~0 ns        ~0 ns         ~0 ns
Small heap alloc      ~50-100 ns   ~50-100 ns    ~100-200 ns
Large heap alloc      ~500 ns      ~500 ns       ~1 µs
Free/Drop             ~20-50 ns    ~20-50 ns     (GC, periodic)
GC pause              N/A          N/A           <1ms (STW portions)
GC overhead           N/A          N/A           ~5-10% CPU
Memory overhead       ~8-16 bytes  ~8-16 bytes   +25-100% (GC headroom)
Ref counting (Rc/Arc) N/A          ~5 ns (Rc)    N/A
                                   ~20 ns (Arc)
```

---

### 5.6 Complete Implementation Example: A Memory-Tracked Buffer

All three languages implementing the same thing: a growable byte buffer with tracked usage.

#### C Implementation

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <assert.h>

/* ===== C: Manual Memory Management ===== */

typedef struct {
    uint8_t *data;
    size_t   len;
    size_t   cap;
} Buffer;

/* Initialize: no allocation yet */
Buffer buffer_new(void) {
    return (Buffer){ .data = NULL, .len = 0, .cap = 0 };
}

/* Append data, growing if needed */
int buffer_push(Buffer *b, const uint8_t *src, size_t n) {
    /* Check if we need to grow */
    if (b->len + n > b->cap) {
        size_t new_cap = b->cap == 0 ? 16 : b->cap * 2;
        while (new_cap < b->len + n) new_cap *= 2;

        /* Use realloc — must handle failure without losing original ptr */
        uint8_t *tmp = realloc(b->data, new_cap);
        if (!tmp) return -1;   /* allocation failure */
        b->data = tmp;
        b->cap = new_cap;
    }
    memcpy(b->data + b->len, src, n);
    b->len += n;
    return 0;
}

/* Get a read-only view */
const uint8_t *buffer_as_slice(const Buffer *b, size_t *len_out) {
    *len_out = b->len;
    return b->data;
}

/* Free: caller is responsible for calling this exactly once */
void buffer_free(Buffer *b) {
    free(b->data);    /* free NULL is a no-op, safe */
    b->data = NULL;   /* zero out to catch double-free bugs early */
    b->len = 0;
    b->cap = 0;
}

int main(void) {
    Buffer buf = buffer_new();

    const uint8_t data1[] = "Hello, ";
    const uint8_t data2[] = "World!";

    if (buffer_push(&buf, data1, sizeof(data1) - 1) < 0) {
        fprintf(stderr, "allocation failed\n");
        return 1;
    }
    if (buffer_push(&buf, data2, sizeof(data2) - 1) < 0) {
        buffer_free(&buf);
        fprintf(stderr, "allocation failed\n");
        return 1;
    }

    size_t len;
    const uint8_t *view = buffer_as_slice(&buf, &len);
    printf("Buffer (%zu bytes): %.*s\n", len, (int)len, view);
    printf("Capacity: %zu\n", buf.cap);

    buffer_free(&buf);   /* MUST remember to call this */
    /* buf.data is now NULL — buffer_free(&buf) again would be a no-op */

    return 0;
}
```

#### Rust Implementation

```rust
/* ===== Rust: Ownership-Based Memory Management ===== */

pub struct Buffer {
    data: Vec<u8>,  // Vec manages heap allocation internally
}

impl Buffer {
    pub fn new() -> Self {
        Buffer { data: Vec::new() }
    }

    pub fn with_capacity(cap: usize) -> Self {
        Buffer { data: Vec::with_capacity(cap) }
    }

    pub fn push(&mut self, src: &[u8]) {
        // Vec::extend_from_slice handles reallocation automatically
        // If capacity is exceeded, it doubles (amortized O(1))
        self.data.extend_from_slice(src);
    }

    // Returns a borrowed slice — lifetime tied to &self
    // The borrow checker ensures the buffer isn't modified while slice is alive
    pub fn as_slice(&self) -> &[u8] {
        &self.data
    }

    pub fn len(&self) -> usize {
        self.data.len()
    }

    pub fn capacity(&self) -> usize {
        self.data.capacity()
    }
}

// Drop is implemented automatically for Vec<u8>
// When Buffer goes out of scope, Vec<u8> is dropped,
// which frees the heap memory. No explicit free needed.
// This is RAII.

fn main() {
    let mut buf = Buffer::new();

    buf.push(b"Hello, ");
    buf.push(b"World!");

    let view: &[u8] = buf.as_slice();
    println!("Buffer ({} bytes): {}", view.len(),
             std::str::from_utf8(view).unwrap());
    println!("Capacity: {}", buf.capacity());

    // buf goes out of scope here → Drop called on Vec<u8> → heap freed
    // No need to manually free anything
    // Cannot double-free: there is only one owner (buf)
    // Cannot use-after-free: buf is gone after this scope
}

// BONUS: Thread-safe shared buffer using Arc<Mutex<Buffer>>
use std::sync::{Arc, Mutex};
use std::thread;

fn shared_buffer_demo() {
    let shared_buf = Arc::new(Mutex::new(Buffer::new()));

    let buf1 = Arc::clone(&shared_buf);
    let t1 = thread::spawn(move || {
        buf1.lock().unwrap().push(b"from thread 1 ");
    });

    let buf2 = Arc::clone(&shared_buf);
    let t2 = thread::spawn(move || {
        buf2.lock().unwrap().push(b"from thread 2");
    });

    t1.join().unwrap();
    t2.join().unwrap();

    let guard = shared_buf.lock().unwrap();
    println!("Shared: {} bytes", guard.len());
    // guard dropped (Mutex unlocked), shared_buf dropped (Arc count 0),
    // Mutex<Buffer> dropped, Buffer dropped, Vec<u8> dropped, heap freed.
}
```

#### Go Implementation

```go
/* ===== Go: GC-Based Memory Management ===== */

package main

import (
    "fmt"
    "sync"
)

// Buffer: managed by Go's GC — no explicit free
type Buffer struct {
    data []byte   // slice: header on stack (if local), backing array on heap
}

func NewBuffer() *Buffer {
    return &Buffer{}  // escapes to heap (address returned from function)
}

func NewBufferWithCapacity(cap int) *Buffer {
    return &Buffer{
        data: make([]byte, 0, cap),  // pre-allocate backing array on heap
    }
}

func (b *Buffer) Push(src []byte) {
    // append handles reallocation automatically
    // if cap exceeded, Go allocates new backing array (1.25x or 2x growth)
    // and copies data — like Rust's Vec
    b.data = append(b.data, src...)
}

func (b *Buffer) AsSlice() []byte {
    return b.data   // returns slice (ptr + len + cap), sharing backing array
                    // No lifetime annotation needed — GC handles validity
}

func (b *Buffer) Len() int      { return len(b.data) }
func (b *Buffer) Cap() int      { return cap(b.data) }

// No Free() method needed — GC handles deallocation
// When no goroutine holds a reference to *Buffer, it's eligible for collection

// ===== Thread-safe version =====
type SafeBuffer struct {
    mu   sync.Mutex
    data []byte
}

func (sb *SafeBuffer) Push(src []byte) {
    sb.mu.Lock()
    defer sb.mu.Unlock()
    sb.data = append(sb.data, src...)
}

func (sb *SafeBuffer) AsSlice() []byte {
    sb.mu.Lock()
    defer sb.mu.Unlock()
    // Return a copy to avoid race after unlock
    cp := make([]byte, len(sb.data))
    copy(cp, sb.data)
    return cp
}

func main() {
    buf := NewBuffer()

    buf.Push([]byte("Hello, "))
    buf.Push([]byte("World!"))

    view := buf.AsSlice()
    fmt.Printf("Buffer (%d bytes): %s\n", len(view), view)
    fmt.Printf("Capacity: %d\n", buf.Cap())

    // buf goes out of scope — GC will eventually free it
    // No explicit free, no risk of double-free or use-after-free
    // (as long as no goroutine leak holds a reference)

    // ===== Reduce allocations with sync.Pool =====
    pool := &sync.Pool{
        New: func() interface{} { return &Buffer{} },
    }

    for i := 0; i < 1000; i++ {
        b := pool.Get().(*Buffer)
        b.data = b.data[:0]             // reset without freeing backing array
        b.Push([]byte("request data"))
        // ... process ...
        pool.Put(b)                     // return to pool — reuse backing array
    }
    // Without pool: 1000 allocations. With pool: ~1 allocation reused 1000 times.
}
```

---

### 5.7 When Stack Grows vs Heap: Side-by-Side

```
C:
   int x = 5;            -> stack (automatic)
   malloc(n)             -> heap (always)
   static int s = 5;     -> .data (static)
   "literal"             -> .text (static)

Rust:
   let x: i32 = 5;       -> stack
   Box::new(5)           -> heap
   let v = vec![1,2,3];  -> stack (header) + heap (data)
   "literal"             -> .text (static &str)
   &T borrowed ref       -> stack (pointer only)
   Rc::new(5)            -> heap (with refcount header)

Go:
   x := 5                -> stack (unless it escapes)
   &x                    -> heap (address taken, likely escapes)
   make([]int, 10)       -> heap (backing array)
   new(int)              -> heap (almost always)
   "literal"             -> .text (string constant)
   go func() { }        -> heap (goroutine stack + closure)
```

---

### 5.8 The Full Ownership/Lifetime/GC Decision Tree

```
Do you need to manage memory?
         |
    +---------+-------+
    |                 |
  Yes                No (scripting, prototyping)
    |                 |
    +------ Choose a language ------+
    |                               |
  Need C ABI/maximum control?    No
    |                               |
   Yes → C                      Need fearless concurrency
    |    (manual, unsafe,        and zero-cost abstractions?
    |     powerful, dangerous)       |
    |                            Yes → Rust
    |                                 (ownership, borrow checker,
    |                                  RAII, no GC, safe by default)
    |                            No → Go
    |                                 (GC, escape analysis,
    |                                  simple, fast enough, goroutines)
```

---

### 5.9 Memory Overhead Summary

```
Object overhead:
+-------------------+--------+--------+--------+
| Overhead per obj  |   C    |  Rust  |   Go   |
+-------------------+--------+--------+--------+
| malloc header     |  8-16B |  8-16B |  0*    |
| Box<T>            |  N/A   |  0B†   |  N/A   |
| Rc<T>             |  N/A   |  16B‡  |  N/A   |
| Arc<T>            |  N/A   |  16B‡  |  N/A   |
| GC metadata       |  N/A   |  N/A   |  per   |
|                   |        |        |  span  |
+-------------------+--------+--------+--------+
* Go uses slab allocator: metadata is per span, not per object
† Box<T> is just a pointer; no extra header in Rust's default allocator
‡ Rc/Arc: strong count (8B) + weak count (8B) ahead of data

GC headroom needed:
  C:    0% (you control all memory)
  Rust: 0% (no GC)
  Go:   ~33-100% (GC needs free space to work efficiently;
        with GOGC=100, heap can be 2x live set)
```

---

### 5.10 Cheat Sheet

```
===== QUICK REFERENCE =====

C:
  malloc(n * sizeof(T))     — allocate n items of type T
  calloc(n, sizeof(T))      — same, zero-initialized
  realloc(ptr, new_n*sizeof)— resize
  free(ptr)                 — deallocate
  ptr = NULL after free     — prevent dangling
  ALWAYS check NULL return  — malloc can fail

Rust:
  let x = T { .. }          — stack allocated
  Box::new(x)               — heap, single owner
  Rc::new(x)                — heap, multiple owners (single-threaded)
  Arc::new(x)               — heap, multiple owners (multi-threaded)
  Rc::clone(&r)             — clone handle, not data (inc refcount)
  &x                        — immutable borrow
  &mut x                    — mutable borrow (exclusive)
  drop(x)                   — explicit early drop
  unsafe { *raw_ptr }       — raw pointer dereference

Go:
  make([]T, len, cap)       — slice (heap backing array)
  make(map[K]V)             — map
  make(chan T, buf)          — channel
  new(T)                    — *T, zero-initialized on heap
  &T{field: val}            — pointer to heap-allocated T
  runtime.GC()              — force GC
  GOGC=100 (env)            — GC target (higher = less frequent)
  GOMEMLIMIT=512MiB (env)   — hard memory cap
  go build -gcflags="-m"    — show escape analysis
  sync.Pool                 — reuse objects, reduce GC pressure
```

---

*End of guide. This document covers the full depth of memory management concepts in C, Rust, and Go, providing the mental models, technical mechanisms, and practical implementation knowledge needed to reason about memory efficiently and correctly in all three languages.*
