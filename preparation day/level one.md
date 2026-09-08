
# Complete Systems Programming Deep Guide
## C · Go · Rust — Memory, Internals, Algorithms, Data Structures

> **How to use this guide:** Every concept is explained from first principles — what it is, *why* it works that way, what the hardware/OS does, ASCII diagrams of memory layouts, and real implementations in C, Go, and Rust. This is a mental model builder, not a cheat sheet.

---

# Table of Contents

1. [The C Memory Model — Process Address Space](#1-the-c-memory-model)
2. [The Stack — Deep Internals](#2-the-stack)
3. [The Heap — Dynamic Memory](#3-the-heap)
4. [Dangling Pointers & Undefined Behavior](#4-dangling-pointers--undefined-behavior)
5. [Pointers — Full Mechanics](#5-pointers)
6. [Arrays — Real Memory Layout](#6-arrays)
7. [Arrays vs Pointers — The Crucial Difference](#7-arrays-vs-pointers)
8. [Strings in C](#8-strings-in-c)
9. [Functions & Parameters](#9-functions--parameters)
10. [Dynamic Memory — malloc/calloc/realloc/free](#10-dynamic-memory)
11. [Structs, Unions, Enums, Typedef](#11-structs-unions-enums-typedef)
12. [Memory Padding & Alignment](#12-memory-padding--alignment)
13. [Type System — Qualifiers & Casts](#13-type-system)
14. [Bits & Bytes — Bitwise Operations](#14-bits--bytes)
15. [Preprocessor & Compilation Pipeline](#15-preprocessor--compilation-pipeline)
16. [Undefined Behavior — Complete Taxonomy](#16-undefined-behavior)
17. [Go Runtime & Goroutine Scheduler](#17-go-runtime--goroutine-scheduler)
18. [Go: yield, Gosched, Channels as Generators](#18-go-yield-and-cooperative-scheduling)
19. [Go: Large File I/O — Streaming, Buffering, mmap](#19-go-large-file-io)
20. [Rust Ownership & Borrowing System](#20-rust-ownership--borrowing)
21. [Rust Lifetimes — Deep Explanation](#21-rust-lifetimes)
22. [Data Structures — Internals & ASCII Diagrams](#22-data-structures)
23. [Algorithm Internals — Sorting, Searching, Trees](#23-algorithm-internals)
24. [OS Concepts — syscalls, virtual memory, page faults](#24-os-concepts)

---

# 1. The C Memory Model

## The Big Picture: Process Address Space

When your OS launches a program, it gives it a virtual address space. On a 64-bit Linux system this is 128TB of virtual memory. Most of it is empty — just mappings. The memory is divided into specific segments with different rules.

```
Virtual Address Space (64-bit Linux, simplified)
Higher addresses
+----------------------------------+ 0xFFFFFFFFFFFFFFFF
|                                  |
|    Kernel Space (not accessible) |
|                                  |
+----------------------------------+ 0x7FFFFFFFFFFFFFFF
|                                  |
|    Stack                         | grows DOWN ↓
|    (local vars, return addrs,    |
|     saved registers)             |
|         ↓                        |
|    ....... (unmapped space) .....|
|         ↑                        |
|    Heap                          | grows UP ↑
|    (malloc/new)                  |
|                                  |
+----------------------------------+
|    BSS Segment                   |
|    (uninitialized global/static  |
|     vars — zeroed by OS)         |
+----------------------------------+
|    Data Segment                  |
|    (initialized global/static    |
|     vars)                        |
+----------------------------------+
|    Text Segment (read-only)      |
|    (compiled machine code,       |
|     string literals)             |
+----------------------------------+ 0x0000000000400000
|    (unmapped — NULL protection)  |
+----------------------------------+ 0x0000000000000000
Lower addresses
```

## Each Segment Explained

### Text Segment (Code)

- Contains compiled machine instructions
- **Read-only** — writes cause segfault (SIGSEGV)
- String literals (`"hello"`) live here too
- Shared between processes running the same program

```c
char *s = "hello";  // "hello" is in TEXT segment — READ ONLY
s[0] = 'H';         // SEGFAULT — writing to read-only memory
```

### Data Segment

- Initialized global and static variables
- Exists for the entire program lifetime
- Read-write

```c
int global_x = 42;         // DATA segment — initialized
static int static_y = 10;  // DATA segment — initialized static
```

### BSS Segment

- **B**lock **S**tarted by **S**ymbol
- Uninitialized (or zero-initialized) global/static variables
- OS guarantees zero-fill before program starts
- The executable just stores the *size*, not the actual zeros (saves disk space)

```c
int global_z;              // BSS — zero initialized automatically
static int static_w;       // BSS — zero initialized automatically
```

### Heap

- Dynamic memory allocated at runtime via `malloc`, `calloc`, `new`
- Managed by the C runtime (libc's allocator, e.g., ptmalloc/jemalloc/tcmalloc)
- Grows upward (toward higher addresses)
- **Your responsibility** to free — OS reclaims everything at process exit but leaks within program lifetime cause problems

### Stack

- Function call frames — local variables, parameters, return addresses, saved registers
- LIFO — last in, first out
- Grows **downward** on x86/x86-64/ARM (toward lower addresses)
- Has a fixed size limit (typically 1–8 MB on Linux, `ulimit -s`)
- **Automatically managed** — when function returns, its frame is destroyed

## Concrete Example

```c
#include <stdio.h>
#include <stdlib.h>

int global_init = 100;        // DATA segment
int global_uninit;             // BSS segment
static int module_static = 5; // DATA segment

void foo(int param) {          // param: STACK
    int local = 42;            // STACK
    static int call_count = 0; // DATA segment (persists across calls!)
    call_count++;

    int *heap_val = malloc(sizeof(int)); // pointer: STACK, int: HEAP
    *heap_val = 999;

    printf("%d\n", *heap_val);
    free(heap_val);  // must free heap allocation
}

int main() {
    char *literal = "hello"; // literal in TEXT, pointer on STACK
    foo(10);
    return 0;
}
```

Memory layout at the moment `foo(10)` is executing:

```
Stack (grows down)
+-------------------------+
| main's frame            |
|   literal = 0x400500    | <-- points to TEXT segment
|   return address        |
+-------------------------+
| foo's frame             |
|   param = 10            |
|   local = 42            |
|   heap_val = 0x55a3c120 | <-- points to HEAP
|   call_count (NOT HERE) | <-- static: lives in DATA segment
+-------------------------+

Heap (grows up)
+-------------------------+
| [int: 999]              | 0x55a3c120
+-------------------------+

DATA Segment
+-------------------------+
| global_init = 100       |
| module_static = 5       |
| call_count = 1          |
+-------------------------+

BSS Segment
+-------------------------+
| global_uninit = 0       |
+-------------------------+

TEXT Segment
+-------------------------+
| machine code for foo()  |
| machine code for main() |
| "hello\0"               |
+-------------------------+
```

---

# 2. The Stack

## What is a Stack Frame?

Every function call creates a **stack frame** (also called **activation record**). It contains:

1. **Function arguments** (or passed in registers on x86-64 for first 6 args)
2. **Return address** — where execution continues after function returns
3. **Saved base pointer** (rbp/ebp) — to restore caller's frame
4. **Local variables**
5. **Saved registers** — those the callee must preserve

## x86-64 Stack Frame Layout

```
Higher addresses
+-------------------------------+ <-- previous frame's rbp (caller's rsp)
|  function argument 7+        |  (first 6 args go in registers: rdi,rsi,rdx,rcx,r8,r9)
|  function argument 8         |
+-------------------------------+
|  return address              |  pushed by CALL instruction
+-------------------------------+ <-- after CALL
|  saved rbp (base pointer)    |  pushed by function prologue: push rbp
+-------------------------------+ <-- new rbp = rsp after push rbp; mov rbp, rsp
|  local variable 1            |
|  local variable 2            |
|  local variable 3            |
|  ...                         |
|  (alignment padding)         |  stack must be 16-byte aligned before CALL
+-------------------------------+ <-- rsp (stack pointer — moves as push/pop happen)
Lower addresses (stack grows down)
```

## Stack Frame in C — Annotated Assembly Concept

```c
int add(int a, int b) {
    int result = a + b;  // local variable
    return result;
}

int main() {
    int x = add(3, 7);
    return 0;
}
```

What happens step by step:

```
1. main calls add(3, 7)
   - 3 goes into register edi (first int arg)
   - 7 goes into register esi (second int arg)
   - CALL instruction pushes return address (address of next instruction in main)
     and jumps to add()

2. Inside add():
   - PUSH rbp          ; save caller's base pointer
   - MOV rbp, rsp      ; establish new frame base
   - SUB rsp, 16       ; make room for locals (aligned to 16 bytes)
   - MOV [rbp-4], edi  ; store arg a as local
   - MOV [rbp-8], esi  ; store arg b as local
   - ADD edi, esi      ; compute sum
   - MOV [rbp-12], eax ; store result
   - MOV eax, [rbp-12] ; load return value into eax (return value register)
   - MOV rsp, rbp      ; epilogue: restore rsp
   - POP rbp           ; restore caller's rbp
   - RET               ; pop return address and jump to it

3. Back in main():
   - eax contains 10
   - MOV [rbp-4], eax  ; store in x
```

Stack state during add():

```
          HIGH
+-------------------+
| main's locals     |
|   x (unset yet)   |
+-------------------+
| return address    |  <-- pushed by CALL
+-------------------+
| saved rbp         |  <-- pushed by add's prologue
+-------------------+  <-- rbp points here
| a (copy of 3)     |  [rbp - 4]
| b (copy of 7)     |  [rbp - 8]
| result            |  [rbp - 12]
| (padding)         |
+-------------------+  <-- rsp
          LOW
```

## Why Stack Size is Limited

The OS allocates a fixed virtual memory region for the stack (default 8MB on Linux). When you exceed it:

```
Stack overflow → SIGSEGV (segmentation fault)
```

Common causes:
- Infinite recursion
- Huge local arrays (`int arr[10000000];`)
- Deep call chains

## The Critical Rule: Local Variables Die When Function Returns

```c
int *dangerous() {
    int x = 42;       // x lives on THIS function's stack frame
    return &x;        // return address of x
}                     // FRAME DESTROYED HERE — x no longer exists

int main() {
    int *p = dangerous();  // p points to DEAD stack memory
    printf("%d\n", *p);    // UNDEFINED BEHAVIOR — may print 42, garbage, or crash
}
```

Stack state:

```
Before dangerous() returns:
+------------------+
| main frame       |
|   p (unset)      |
+------------------+
| dangerous frame  |  <-- rbp points here
|   x = 42        |  address: 0x7fff5abc1234
+------------------+  <-- rsp

After dangerous() returns:
+------------------+
| main frame       |
|   p = 0x7fff5abc1234  | <-- points to this ↓
+------------------+
| DEAD MEMORY      |  0x7fff5abc1234 — still contains 42 for now
| (stack freed)    |  but will be overwritten by next function call
+------------------+

After printf() is called:
+------------------+
| main frame       |
|   p = 0x7fff5abc1234  |
+------------------+
| printf's frame   |  <-- OVERWRITES the memory p points to!
|   its locals...  |  p now points to printf's stack data!
+------------------+
```

This is why `printf("%s\n", msg)` after `printf("test\n")` may show garbage — the second printf's own stack frame overwrote the memory that msg pointed to.

---

# 3. The Heap

## What is the Heap?

The heap is a large region of memory managed by the runtime allocator. Unlike the stack, you control when memory is allocated and freed.

```
Heap structure (simplified):
+----------------------------------+
|  heap metadata / allocator state |
+----------------------------------+
|  [allocated block A]             |
|  [free block]                    |
|  [allocated block B]             |
|  [free block]                    |
|  [allocated block C]             |
|  ...                             |
+----------------------------------+
|  (unmapped — can grow with brk() |
|   or mmap())                     |
+----------------------------------+
```

## How malloc Works Internally

`malloc` is implemented in libc (ptmalloc2 on Linux glibc). Internally:

1. For small allocations: manages **free lists** of fixed-size bins
2. For large allocations: calls `mmap()` directly for anonymous memory
3. Uses `brk()`/`sbrk()` syscalls to grow the heap region

Every allocated block has a **header** with metadata:

```
malloc'd block layout (ptmalloc):
+-----------------------------------+
| prev_size (8 bytes)               |  size of previous chunk (if prev is free)
+-----------------------------------+
| size (8 bytes)                    |  size of this chunk | flags
|   bit 0: PREV_INUSE               |
|   bit 1: IS_MMAPPED               |
|   bit 2: NON_MAIN_ARENA           |
+-----------------------------------+  <-- malloc() returns pointer to HERE
| user data                         |
| (your allocated bytes)            |
+-----------------------------------+
| next chunk header...              |
```

This is why:
- `free()` needs no size argument — it reads the size from the header
- `free()` twice corrupts the heap metadata → crash or security bug
- Buffer overflow into heap can corrupt adjacent chunk headers → heap exploit

## malloc in C

```c
#include <stdlib.h>
#include <string.h>

int main() {
    // malloc: allocates uninitialized memory
    int *arr = malloc(10 * sizeof(int));
    if (arr == NULL) {
        // always check! malloc can fail (out of memory)
        return 1;
    }
    // arr points to 40 bytes of UNINITIALIZED memory
    // reading arr[0] before writing = undefined behavior

    // calloc: allocates AND zeroes memory
    int *zeroed = calloc(10, sizeof(int));
    // zeroed[0] == 0, zeroed[1] == 0, etc. — guaranteed

    // realloc: resize an existing allocation
    arr = realloc(arr, 20 * sizeof(int));
    // may move the allocation to a new address!
    // old arr pointer is now invalid if realloc moved it
    // if realloc fails it returns NULL but OLD pointer is still valid

    // Safe realloc pattern:
    int *tmp = realloc(arr, 20 * sizeof(int));
    if (tmp == NULL) {
        free(arr);  // original still valid, free it
        return 1;
    }
    arr = tmp;

    // free: return memory to allocator
    free(arr);
    free(zeroed);

    // WRONG patterns:
    // free(arr);     // double free — undefined behavior
    // arr[0] = 1;   // use-after-free — undefined behavior

    return 0;
}
```

## Heap Fragmentation

Over time, interleaved allocations and frees can create "Swiss cheese" patterns:

```
Initial heap:
[AAAAAAAA][BBBBBBBB][CCCCCCCC][DDDDDDDD]

After free(B) and free(D):
[AAAAAAAA][free    ][CCCCCCCC][free    ]

Trying to allocate 16 bytes:
Total free = 16 bytes, BUT not contiguous
malloc(16) may FAIL even though technically enough free bytes exist
(or allocator may coalesce adjacent free blocks)
```

---

# 4. Dangling Pointers & Undefined Behavior

## Three Sources of Dangling Pointers

### 1. Return pointer to stack variable

```c
int *get_local() {
    int x = 10;
    return &x;  // WARNING: function returns address of local variable
}               // x's stack frame is destroyed here
```

### 2. Use after free

```c
int *p = malloc(sizeof(int));
*p = 42;
free(p);       // p is now a dangling pointer
printf("%d\n", *p);  // use-after-free: undefined behavior
// Even worse:
free(p);       // double-free: undefined behavior, may crash or exploit
```

### 3. Pointer to expired scope

```c
int *p;
{
    int x = 5;
    p = &x;
}   // x's lifetime ends here
*p = 10;  // dangling — x is gone
```

## Why Undefined Behavior "Sometimes Works"

This is the most dangerous aspect. UB does not mean "crash immediately." It means **anything can happen**, including seemingly correct behavior.

Why reading a dangling stack pointer sometimes works:
1. The stack memory is still physically there — it hasn't been unmapped
2. No other function has been called to overwrite it yet
3. The values byte-for-byte still match what you expect

```c
int *dangerous() {
    int x = 42;
    return &x;
}

int main() {
    int *p = dangerous();
    printf("%d\n", *p);   // might print 42 — stack not yet overwritten
    
    // Now do something else that uses the stack:
    int arr[100];
    for (int i = 0; i < 100; i++) arr[i] = i;
    
    printf("%d\n", *p);   // now likely prints garbage — arr[] overwrote the frame
}
```

## The Compiler's Right to Do Anything with UB

Modern compilers (GCC, Clang) are allowed to **assume UB never occurs**. This leads to shocking optimizations:

```c
// Compiler sees this:
int x;
if (x > 0) doA();
if (x > 0) doB();

// Compiler ASSUMES x > 0 check cannot be UB
// So if the first check is taken, the second must also be taken
// Optimized to:
if (x > 0) { doA(); doB(); }
// This is valid transformation ONLY if x is initialized
// If x is uninitialized, the compiler's assumption breaks real behavior
```

## Undefined Behavior Types — Complete List

| Category | Example | Why UB |
|----------|---------|--------|
| Null pointer deref | `*NULL` | Address 0 is unmapped |
| Dangling pointer | use-after-free | Memory may be reused |
| Out-of-bounds access | `arr[10]` on `int arr[5]` | Past valid memory |
| Signed integer overflow | `INT_MAX + 1` | C doesn't define wrapping |
| Uninitialized variable | `int x; use(x);` | Could be anything |
| Invalid pointer arithmetic | `ptr + 1000000` past object | Violates C object model |
| Type punning via cast | `*(float*)&my_int` | Strict aliasing violation |
| Modify string literal | `"hello"[0] = 'H'` | TEXT segment is read-only |
| Data race | concurrent write/read | Memory model violation |
| Calling free() twice | `free(p); free(p);` | Heap corruption |

## Rust's Answer to Dangling Pointers

Rust's ownership system prevents this entire class of bugs at compile time:

```rust
fn get_local() -> &i32 {  // ERROR: cannot return reference to local variable
    let x = 10;
    &x  // compiler rejects this: x doesn't live long enough
}

fn main() {
    // This will not compile:
    // let p = get_local();
}
```

Rust error:
```
error[E0106]: missing lifetime specifier
  --> src/main.rs:1:20
   |
1  | fn get_local() -> &i32 {
   |                    ^ expected named lifetime parameter
```

The compiler enforces that references cannot outlive what they point to.

## Go's Answer: Escape Analysis

Go doesn't have dangling pointers because it uses garbage collection. When you return a pointer to a local variable, Go's escape analysis detects this and **automatically allocates it on the heap**:

```go
func getLocal() *int {
    x := 10
    return &x  // Go automatically allocates x on the heap
}              // x "escapes" to the heap — it does NOT get destroyed

func main() {
    p := getLocal()
    fmt.Println(*p)  // perfectly safe — x is on heap, managed by GC
}
```

The Go compiler output with `-gcflags="-m"` will show:
```
./main.go:3:2: moved to heap: x
```

---

# 5. Pointers

## What is a Pointer?

A pointer is a variable that stores a **memory address**. That's it. Every pointer is just an integer — the address of something in memory.

```
Variable x at address 0x7fff1000:
+--------+
|  42    |   <-- value of x
+--------+
0x7fff1000

Pointer p at address 0x7fff1008:
+------------------+
|  0x7fff1000      |   <-- value of p = address of x
+------------------+
0x7fff1008
```

## Pointer Mechanics in C

```c
int x = 42;
int *p = &x;    // & = "address of" operator
                // p stores the address of x

printf("%p\n", (void*)p);   // prints address, like 0x7fff1000
printf("%d\n", *p);         // * = dereference: "value at address p" → 42

*p = 100;       // write through pointer — modifies x
printf("%d\n", x);  // prints 100
```

## Pointer Arithmetic

This is one of the most powerful and dangerous features of C. When you add to a pointer, it moves by **sizeof(type)** bytes, not by 1 byte.

```c
int arr[5] = {10, 20, 30, 40, 50};
int *p = arr;   // p points to arr[0]

// Memory layout:
// Address:  1000  1004  1008  1012  1016
// Values:   [10]  [20]  [30]  [40]  [50]
//            ^
//            p = 1000

p + 1  // = 1004 (not 1001!) — moves by sizeof(int) = 4 bytes
p + 2  // = 1008
*(p + 3)  // = value at 1012 = 40

// Equivalent to array indexing:
arr[i] == *(arr + i)  // THIS IS THE DEFINITION OF []
// So arr[3] literally compiles to *(arr + 3)
// And pointer arithmetic *(p + 3) is the same

// Pointer difference:
int *a = &arr[1];  // 1004
int *b = &arr[4];  // 1016
b - a  // = 3 (not 12!) — result is in units of int, not bytes
```

## Pointer to Pointer (Double Pointer)

```c
int x = 5;
int *p = &x;
int **pp = &p;

// Memory:
// x  at 0x100: [5]
// p  at 0x200: [0x100]  ← stores address of x
// pp at 0x300: [0x200]  ← stores address of p

**pp = 99;    // deref pp → get p (0x100) → deref p → write 99 to x
printf("%d", x);  // 99
```

Double pointers are common for:
- Modifying a pointer from a function (pass-by-pointer-to-pointer)
- 2D dynamically allocated arrays
- Arrays of strings

```c
// Modify pointer from function
void allocate(int **out) {
    *out = malloc(sizeof(int));
    **out = 42;
}

int main() {
    int *p = NULL;
    allocate(&p);     // pass address of p so function can change p
    printf("%d", *p); // 42
    free(p);
}
```

## Function Pointers

A function pointer stores the address of a function (which is in the TEXT segment):

```c
int add(int a, int b) { return a + b; }
int sub(int a, int b) { return a - b; }

// Declaration: int (*name)(int, int)
//   ^ return type  ^ name  ^ param types

int (*op)(int, int);  // declare function pointer
op = add;             // point to add
printf("%d\n", op(3, 4));  // 7

op = sub;
printf("%d\n", op(3, 4));  // -1

// Array of function pointers (like a vtable):
int (*operations[2])(int, int) = {add, sub};
for (int i = 0; i < 2; i++) {
    printf("%d\n", operations[i](10, 3));
}
```

In C++, virtual dispatch uses a **vtable** which is literally an array of function pointers. Each object has a hidden vtable pointer.

## Const Pointer Variations

```c
int x = 10;
int y = 20;

// 1. Pointer to const int: cannot modify value through pointer
//    but can change what the pointer points to
const int *p1 = &x;
*p1 = 5;    // ERROR: read-only location
p1 = &y;    // OK: pointer itself is changeable

// 2. Const pointer to int: cannot change what pointer points to
//    but can modify value through it
int *const p2 = &x;
*p2 = 5;    // OK: value is changeable
p2 = &y;    // ERROR: pointer itself is const

// 3. Const pointer to const int: neither changeable
const int *const p3 = &x;
*p3 = 5;    // ERROR
p3 = &y;    // ERROR

// Memory trick to remember:
// Read right-to-left from the variable name:
// const int *p1  → p1 is a (pointer to (const int))   → pointer is mutable, value is const
// int *const p2  → p2 is a (const pointer to int)     → pointer is const, value is mutable
// const int *const p3 → p3 is a (const pointer to const int) → both const
```

## Pointer Implementations in Rust

Rust has several pointer types with different safety guarantees:

```rust
fn main() {
    let x = 42i32;
    
    // Shared reference (immutable borrow) — like const int*
    let r: &i32 = &x;
    println!("{}", *r);  // deref to read
    
    let mut y = 100i32;
    
    // Mutable reference — like int*
    // RULE: only ONE mutable reference at a time (no aliasing!)
    let rm: &mut i32 = &mut y;
    *rm = 200;
    println!("{}", y);  // 200
    
    // Raw pointers (unsafe) — like C pointers
    let raw: *const i32 = &x as *const i32;
    let raw_mut: *mut i32 = &mut y as *mut i32;
    
    unsafe {
        println!("{}", *raw);      // dereference requires unsafe block
        *raw_mut = 999;
    }
    
    // Box<T> — heap allocated, owned pointer
    let boxed: Box<i32> = Box::new(42);
    println!("{}", *boxed);  // auto-deref
    // Box freed automatically when it goes out of scope
}
```

Go pointers:

```go
package main

import "fmt"

func increment(p *int) {
    *p++  // dereference and increment
}

func main() {
    x := 42
    p := &x         // p is *int
    fmt.Println(*p) // 42
    
    increment(&x)
    fmt.Println(x)  // 43
    
    // Go does NOT have pointer arithmetic
    // This is intentional for safety
    // p + 1 is a compile error in Go
    
    // Go DOES have unsafe.Pointer for rare low-level cases:
    // unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 4)
    // But this is explicitly "unsafe" and rare
}
```

---

# 6. Arrays

## Arrays in Memory — Contiguous Layout

An array is a **contiguous block of identically-typed elements**. This is the fundamental property from which everything else follows.

```c
int arr[5] = {10, 20, 30, 40, 50};
```

Memory layout:

```
Address:   1000   1004   1008   1012   1016
           +----+  +----+  +----+  +----+  +----+
           | 10 |  | 20 |  | 30 |  | 40 |  | 50 |
           +----+  +----+  +----+  +----+  +----+
arr[0]    arr[1]  arr[2]  arr[3]  arr[4]
```

Each element is exactly `sizeof(int)` = 4 bytes apart. The elements are adjacent in memory — no gaps.

## Array Indexing is Pointer Arithmetic

`arr[i]` is **defined** as `*(arr + i)`. This is not a convenient equivalence — it is the actual definition.

```c
int arr[5] = {10, 20, 30, 40, 50};

arr[3]       // same as *(arr + 3)
*(arr + 3)   // same as arr[3]
3[arr]       // also valid! (commutative: *(3 + arr) = *(arr + 3))
             // don't do this in real code
```

## 2D Arrays in Memory

2D arrays are stored in **row-major order** in C:

```c
int matrix[3][4] = {
    {1, 2, 3, 4},
    {5, 6, 7, 8},
    {9,10,11,12}
};
```

Memory layout (row-major — all of row 0, then row 1, then row 2):

```
Address: 0  4  8 12 16 20 24 28 32 36 40 44
         +--+--+--+--+--+--+--+--+--+--+--+--+
         | 1| 2| 3| 4| 5| 6| 7| 8| 9|10|11|12|
         +--+--+--+--+--+--+--+--+--+--+--+--+
         [0][0]         [1][0]         [2][0]
              [0][3]         [1][3]         [2][3]

matrix[r][c] is at byte offset: r * (4 * sizeof(int)) + c * sizeof(int)
                               = r * 16 + c * 4
```

This is why row-major traversal is cache-friendly (sequential memory access) but column-major is not (skips every 16 bytes).

## sizeof on Arrays

```c
int arr[5];
sizeof(arr)     // 20 — size of the entire array (5 * 4 bytes)
sizeof(arr[0])  // 4  — size of one element

// Once you pass an array to a function, it decays to a pointer
// and you LOSE sizeof information:
void func(int arr[]) {
    sizeof(arr);  // 8 — size of a POINTER, not the array!
}
```

---

# 7. Arrays vs Pointers — The Crucial Difference

## They Are NOT the Same

This confusion causes real bugs. Let's be precise:

```c
char arr[] = "hello";  // arr IS the array — 6 bytes on STACK: h,e,l,l,o,\0
char *ptr = "hello";   // ptr is a POINTER to string literal in TEXT segment
```

Memory:

```
TEXT segment:
+---+---+---+---+---+----+
| h | e | l | l | o | \0 |   ← string literal (READ ONLY)
+---+---+---+---+---+----+
addr: 0x400500

STACK:
arr: +---+---+---+---+---+----+
     | h | e | l | l | o | \0 |   ← actual COPY on stack (WRITABLE)
     +---+---+---+---+---+----+
     addr: 0x7fff1000

ptr: +------------------+
     |    0x400500      |         ← pointer to TEXT (READ ONLY through ptr)
     +------------------+
     addr: 0x7fff1010
```

```c
arr[0] = 'H';   // OK — arr is a writable copy on the stack
ptr[0] = 'H';   // SEGFAULT — writing to read-only TEXT segment
```

## Array Decay

When you use an array name in most expressions, it automatically **decays** to a pointer to its first element. This happens in:
- Passing to a function
- Most arithmetic operations
- Assignment (except when initializing another array)

But does NOT happen in:
- `sizeof(arr)` — gives full array size
- `&arr` — address of the whole array
- String initialization: `char arr[] = "hello"`

```c
int arr[5] = {1,2,3,4,5};
int *p = arr;      // arr decays to &arr[0]
int *q = &arr[0];  // explicitly the same thing
// p == q

// BUT:
&arr       // pointer to the entire array: type is int(*)[5]
&arr[0]    // pointer to first element: type is int*

// Both have the SAME value (same address) but different TYPES
// &arr + 1 would move past the ENTIRE array (5 * 4 = 20 bytes)
// &arr[0] + 1 would move to the next int (4 bytes)
```

## Passing Arrays to Functions

When you pass an array to a function, you **always** pass a pointer:

```c
void process(int arr[], int n) {
    // arr is actually int* here
    // sizeof(arr) == sizeof(int*) == 8
    // The function has NO way to know the array length from arr alone
    // That's why we pass n separately!
}

// These four declarations are IDENTICAL:
void f(int arr[]);
void f(int arr[10]);  // the 10 is ignored!
void f(int *arr);
// All mean: f receives a pointer to int
```

This is why C functions that operate on arrays must take a length parameter.

---

# 8. Strings in C

## What is a C String?

A C string is a **null-terminated array of char**. The null terminator `'\0'` (byte value 0) marks the end. There is no string type — just a convention.

```
char str[] = "hello";

Memory:
+---+---+---+---+---+----+
| h | e | l | l | o | \0 |
+---+---+---+---+---+----+
  0   1   2   3   4   5

strlen("hello") = 5    (counts chars BEFORE null terminator)
sizeof("hello") = 6    (includes null terminator)
```

## String Literal vs Char Array

```c
// String literal — in TEXT segment, read-only
char *s1 = "hello";      // s1 points to TEXT
s1[0] = 'H';             // SEGFAULT

// Char array — copy on stack, writable
char s2[] = "hello";     // s2 is a 6-byte stack array
s2[0] = 'H';             // OK — s2 is "Hello" now

// Heap string — writable, must free
char *s3 = malloc(6);
strcpy(s3, "hello");     // OK
s3[0] = 'H';             // OK
free(s3);                // must free
```

## String Functions and Their Dangers

### strlen

```c
size_t strlen(const char *s);
// Walks from s until it finds '\0', counting bytes
// Does NOT include the null terminator
// O(n) — must scan every character
// DANGEROUS if string is not null-terminated (walks off into memory)
```

### strcpy

```c
char *strcpy(char *dst, const char *src);
// Copies src (including \0) to dst
// NO bounds checking — if src is longer than dst, buffer overflow!

char small[5];
strcpy(small, "hello world");  // BUFFER OVERFLOW
// Writes 12 bytes into 5-byte buffer — corrupts adjacent memory
```

Buffer overflow anatomy:

```
Before overflow:
Stack:
small:  [?][?][?][?][?]
next:   [important data]

After strcpy(small, "hello world"):
        [h][e][l][l][o][ ][w][o][r][l][d][\0]
        ↑ small(5) ↑  ↑ OVERFLOW into next memory ↑

Overwrites: local variables, saved registers, return address
→ crash, corruption, or remote code execution (classic exploit)
```

### strncpy

```c
char *strncpy(char *dst, const char *src, size_t n);
// Copies AT MOST n bytes
// BUT: if src is n or more chars, NO null terminator is written!
// Result is NOT a valid C string!

char buf[5];
strncpy(buf, "hello", 5);  // copies h,e,l,l,o — no \0!
printf("%s", buf);          // UNDEFINED BEHAVIOR — no null terminator
strlen(buf);                // walks off the end looking for \0

// Safe pattern:
strncpy(buf, src, sizeof(buf) - 1);
buf[sizeof(buf) - 1] = '\0';  // always null-terminate manually
```

### snprintf (the safe alternative)

```c
// Always writes at most n bytes INCLUDING the null terminator
// Always null-terminates (as long as n > 0)
snprintf(buf, sizeof(buf), "%s %s", first, last);
// Safe — guaranteed null-terminated, no overflow
```

### strtok — stateful and dangerous

```c
// Modifies the string (replaces delimiters with \0)
// Keeps internal static state — NOT thread-safe
// Use strtok_r (reentrant version) in multithreaded code
char str[] = "a,b,c";
char *tok = strtok(str, ",");  // modifies str!
while (tok != NULL) {
    printf("%s\n", tok);
    tok = strtok(NULL, ",");  // NULL means "continue from last position"
}
```

## Strings in Go — Immutable, UTF-8

Go strings are:
- Immutable byte sequences
- UTF-8 encoded by default
- A string header = (pointer to data, length) — NOT null-terminated
- Stored in read-only data section or heap

```go
s := "hello"
// String header on stack:
// +------------------+--------+
// |  ptr to "hello"  |  len=5 |
// +------------------+--------+
// The actual bytes are in read-only memory

// String is immutable:
// s[0] = 'H'  // compile error

// But you can create new strings easily:
s2 := "H" + s[1:]  // "Hello" — creates new string

// Rune = Unicode code point (like int32)
for i, r := range s {
    fmt.Printf("index=%d rune=%c byte=%d\n", i, r, r)
}

// Convert to []byte for mutation:
b := []byte(s)
b[0] = 'H'
s3 := string(b)  // "Hello"

// strings.Builder — efficient string concatenation
var sb strings.Builder
for i := 0; i < 100; i++ {
    sb.WriteString("x")
}
result := sb.String()
```

## Strings in Rust — Owned vs Borrowed

Rust has two main string types:

```rust
// String — heap-allocated, owned, mutable
let mut owned: String = String::from("hello");
owned.push_str(" world");  // can mutate

// &str — string slice — borrowed reference to string data
// Can point to: string literal in binary, or part of a String
let borrowed: &str = "hello";          // points to binary read-only
let slice: &str = &owned[0..5];        // points into owned's heap data

// String internals:
// String = { ptr: *mut u8, len: usize, capacity: usize }
// &str  = { ptr: *const u8, len: usize }  (no capacity — not owned)

// String layout:
//   Stack variable:  [ ptr | len | cap ]
//                      |
//                      v
//   Heap:            [ h | e | l | l | o |   |   ]
//                     0   1   2   3   4     (unused capacity)

fn greet(name: &str) {  // takes &str — works for both String and &str
    println!("Hello, {}!", name);
}

fn main() {
    let s: String = String::from("Alice");
    greet(&s);      // deref coercion: &String → &str automatically
    greet("Bob");   // &str literal directly
}
```

---

# 9. Functions & Parameters

## C is Pass-by-Value — Always

Every function argument in C is **copied**. You always pass a value. To modify the caller's variable, you pass a pointer (which is itself passed by value — but dereferencing it lets you reach the original).

```c
void increment_wrong(int x) {
    x++;  // modifies local COPY — caller unchanged
}

void increment_right(int *x) {
    (*x)++;  // dereference pointer to modify original
}

int main() {
    int n = 5;
    increment_wrong(n);  // n still 5
    increment_right(&n); // n is now 6
}
```

Stack state during `increment_right(&n)`:

```
main's frame:
+---------------+
| n = 5         |  address: 0x7fff1000
+---------------+

increment_right's frame:
+---------------+
| x = 0x7fff1000|  <-- copy of the pointer, not copy of n
+---------------+

*x dereferences to: 0x7fff1000 → 5
(*x)++ changes n at 0x7fff1000 to 6
```

## Passing Structs — Full Copy

```c
typedef struct {
    int x, y, z;
    char name[50];
} Point;

void move_wrong(Point p) {  // copies the ENTIRE struct (62 bytes!)
    p.x += 10;              // modifies local copy only
}

void move_right(Point *p) {  // passes 8-byte pointer
    p->x += 10;              // modifies original
}
```

For large structs, always pass by pointer in C. This is both a correctness and performance issue.

## Variadic Functions

```c
#include <stdarg.h>

int sum(int count, ...) {
    va_list args;
    va_start(args, count);  // initialize iterator after last named param
    
    int total = 0;
    for (int i = 0; i < count; i++) {
        total += va_arg(args, int);  // read next argument as int
    }
    
    va_end(args);  // cleanup
    return total;
}

int main() {
    printf("%d\n", sum(3, 10, 20, 30));  // 60
}
```

This is how `printf` works. The format string tells it what types and how many arguments to expect.

## Recursion and Stack Depth

Each recursive call creates a new stack frame:

```c
int factorial(int n) {
    if (n <= 1) return 1;
    return n * factorial(n - 1);
}
```

Stack during `factorial(4)`:

```
+------------------+
| factorial(4)     |  n=4, waiting for factorial(3)
+------------------+
| factorial(3)     |  n=3, waiting for factorial(2)
+------------------+
| factorial(2)     |  n=2, waiting for factorial(1)
+------------------+
| factorial(1)     |  n=1, returns 1
+------------------+
```

Each frame is ~16-32 bytes. `factorial(100000)` → 1.6MB+ on stack → stack overflow.

**Tail recursion** avoids this (compiler converts to loop when detected):

```c
int factorial_tail(int n, int acc) {
    if (n <= 1) return acc;
    return factorial_tail(n - 1, n * acc);  // tail call — nothing to do after return
}
// With -O2, GCC converts this to a loop — constant stack space
```

## Go: Functions, Goroutines, Multiple Returns

```go
// Multiple return values — idiomatic Go
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
}

result, err := divide(10, 3)
if err != nil {
    log.Fatal(err)
}

// Named return values
func minmax(arr []int) (min, max int) {
    min, max = arr[0], arr[0]
    for _, v := range arr[1:] {
        if v < min { min = v }
        if v > max { max = v }
    }
    return  // naked return — returns min, max
}

// Closures capture by reference
func counter() func() int {
    count := 0
    return func() int {
        count++       // captures count from outer scope
        return count
    }
}

c := counter()
fmt.Println(c()) // 1
fmt.Println(c()) // 2
fmt.Println(c()) // 3
```

## Rust: Functions, Ownership Transfer

```rust
// Rust passes by move for non-Copy types (transfers ownership)
fn consume(s: String) {  // s is moved in
    println!("{}", s);
}  // s is dropped here (memory freed)

fn main() {
    let s = String::from("hello");
    consume(s);         // s is MOVED into consume
    // consume(s);      // ERROR: s was moved — cannot use again
    
    // To keep using s, either:
    // 1. Clone it (expensive copy)
    let s2 = String::from("hello");
    consume(s2.clone()); // s2 still valid
    
    // 2. Pass a reference (borrow)
    fn borrow(s: &String) {
        println!("{}", s);
    }
    borrow(&s2);  // s2 still valid after call
    
    // Copy types (i32, bool, f64, etc.) are always copied:
    let x: i32 = 5;
    let y = x;  // x is copied, not moved
    println!("{} {}", x, y);  // both still valid
}
```

---

# 10. Dynamic Memory

## malloc — Memory Allocation Library

```c
#include <stdlib.h>

// malloc(n): allocate n bytes, return void* or NULL
// Memory is UNINITIALIZED — contains garbage
void *malloc(size_t size);

// calloc(count, size): allocate count*size bytes, ZERO-INITIALIZED
void *calloc(size_t nmemb, size_t size);

// realloc(ptr, size): resize existing allocation
// May move to new location — old pointer INVALID if moved
void *realloc(void *ptr, size_t size);

// free(ptr): return memory to allocator
// ptr must be from malloc/calloc/realloc
// ptr must be the ORIGINAL pointer (not offset into allocation)
void free(void *ptr);
```

## Memory Lifecycle

```c
// Good lifecycle:
int *p = malloc(sizeof(int));   // 1. Allocate
if (!p) handle_oom();           // 2. Check for failure
*p = 42;                        // 3. Use
free(p);                        // 4. Free
p = NULL;                       // 5. Nullify (prevents double-free bugs)

// Bad patterns:
int *p = malloc(100);
free(p);
free(p);        // double-free: UNDEFINED BEHAVIOR, may crash or exploit
*p = 5;         // use-after-free: UNDEFINED BEHAVIOR

int *q = malloc(100);
// program ends without free(q) → memory leak
// OS reclaims at process exit, but during program life:
// - wastes memory
// - can cause OOM in long-running servers
```

## Dynamic Array — Resizable Array

```c
typedef struct {
    int *data;
    size_t len;
    size_t cap;
} Vec;

Vec vec_new() {
    return (Vec){ .data = NULL, .len = 0, .cap = 0 };
}

void vec_push(Vec *v, int val) {
    if (v->len == v->cap) {
        // Double capacity (amortized O(1) push)
        size_t new_cap = v->cap == 0 ? 4 : v->cap * 2;
        int *new_data = realloc(v->data, new_cap * sizeof(int));
        if (!new_data) {
            perror("realloc");
            exit(1);
        }
        v->data = new_data;
        v->cap = new_cap;
    }
    v->data[v->len++] = val;
}

void vec_free(Vec *v) {
    free(v->data);
    v->data = NULL;
    v->len = v->cap = 0;
}

int main() {
    Vec v = vec_new();
    for (int i = 0; i < 10; i++) vec_push(&v, i * i);
    for (size_t i = 0; i < v.len; i++) printf("%d ", v.data[i]);
    vec_free(&v);
}
```

Growth diagram:

```
cap=0:  (null)
cap=4:  [0][1][4][9]
cap=8:  [0][1][4][9][16][25][36][49]
cap=16: [0][1][4][9][16][25][36][49][64][81][ ][ ][ ][ ][ ][ ]
```

## Memory Errors — Valgrind Output

```
==12345== Invalid read of size 4
==12345==    at 0x400621: main (test.c:10)
==12345==  Address 0x5204040 is 0 bytes after a block of size 4 alloc'd
==12345==    at 0x4C2FB0F: malloc (in /usr/lib/valgrind/vgpreload_memcheck.so)
```

## Rust's Approach: No Manual Memory Management

Rust achieves memory safety without GC by using **RAII** (Resource Acquisition Is Initialization) and ownership rules:

```rust
fn main() {
    // Box<T> is a heap allocation
    let b: Box<i32> = Box::new(42);
    println!("{}", *b);
    // b is automatically freed when it goes out of scope — no free() needed
    
    // Vec<T> is the dynamic array — equivalent to C's Vec above
    let mut v: Vec<i32> = Vec::new();
    v.push(1);
    v.push(2);
    v.push(3);
    // Vec internals: { ptr: *mut T, len: usize, cap: usize }
    // Automatically resizes, automatically freed
    
    println!("len={} cap={}", v.len(), v.capacity());
    
    // String is heap-allocated Vec<u8> essentially
    let s = String::from("hello");
    // freed when s goes out of scope
}
// b, v, s all freed here — deterministic, no GC pause
```

```rust
// Double-free is IMPOSSIBLE in Rust — ownership prevents it:
fn main() {
    let v = vec![1, 2, 3];
    let v2 = v;         // v is MOVED to v2
    // drop(v);         // ERROR: v was moved
    drop(v2);           // OK: v2 owns the data, freed here
}

// Use-after-free is IMPOSSIBLE:
fn main() {
    let s = String::from("hello");
    let r = &s;         // borrow s
    drop(s);            // ERROR: cannot drop s while r is borrowed
    println!("{}", r);  // would be use-after-free, but compiler prevents it
}
```

Go's dynamic memory:

```go
package main

import "fmt"

func main() {
    // Slice = Go's dynamic array
    // Internally: { ptr *T, len int, cap int }
    s := make([]int, 0, 4)  // len=0, cap=4
    
    s = append(s, 1)  // len=1
    s = append(s, 2)  // len=2
    s = append(s, 3)  // len=3
    s = append(s, 4)  // len=4
    s = append(s, 5)  // len=5, cap doubles to 8 (new allocation)
    
    // Slice is managed by GC — no manual free
    
    // Map = hash table
    m := make(map[string]int)
    m["a"] = 1
    m["b"] = 2
    
    // All Go heap objects are managed by garbage collector
    // GC runs concurrently, pauses are very short (<1ms in modern Go)
}
```

---

# 11. Structs, Unions, Enums, Typedef

## Structs — Grouping Related Data

```c
struct Point {
    int x;
    int y;
};

// Usage:
struct Point p1;
p1.x = 10;
p1.y = 20;

struct Point p2 = {.x = 5, .y = 7};  // designated initializers

// Nested struct:
struct Rectangle {
    struct Point top_left;
    struct Point bottom_right;
};

// Pointer to struct — use -> for member access:
struct Point *pp = &p1;
pp->x = 99;    // equivalent to (*pp).x = 99
```

## Self-Referential Structures (Linked List Node)

```c
// Forward reference — struct Node refers to itself
struct Node {
    int data;
    struct Node *next;  // pointer to same type — OK
};

// Build a linked list:
struct Node *head = NULL;
struct Node *n1 = malloc(sizeof(struct Node));
struct Node *n2 = malloc(sizeof(struct Node));

n1->data = 10;
n1->next = n2;

n2->data = 20;
n2->next = NULL;

head = n1;
```

Memory layout:

```
head → +--------+--------+       +--------+--------+
       | data=10| next ──┼──────▶| data=20| next=0 |
       +--------+--------+       +--------+--------+
       0x55a3c0           0x55a3d0
```

## Unions — Overlapping Memory

A union stores multiple fields in the **same memory**, all overlapping from address 0. Only one field is valid at a time. Size of union = size of largest field.

```c
union Value {
    int   i;     // 4 bytes
    float f;     // 4 bytes
    char  c[4];  // 4 bytes
};                // sizeof(union Value) = 4

union Value v;
v.i = 0x41424344;  // write as int

// Now read as char array:
printf("%c %c %c %c\n", v.c[0], v.c[1], v.c[2], v.c[3]);
// On little-endian: D C B A (bytes reversed)

// This is a common technique to inspect byte representation of a value
```

Union memory layout:

```
+--+--+--+--+  ← all fields start at the same address
|  |  |  |  |  ← union Value occupies 4 bytes
+--+--+--+--+
 c[0]  c[2]
  c[1]  c[3]
 i (all 4 bytes)
 f (all 4 bytes)
```

## Tagged Union (Discriminated Union)

A common C pattern for type-safe variants:

```c
typedef enum { INT_VAL, FLOAT_VAL, STR_VAL } ValueType;

typedef struct {
    ValueType type;  // tag — tells you which union field is valid
    union {
        int   i;
        float f;
        char *s;
    } data;
} Value;

void print_value(Value *v) {
    switch (v->type) {
        case INT_VAL:   printf("int: %d\n",   v->data.i); break;
        case FLOAT_VAL: printf("float: %f\n", v->data.f); break;
        case STR_VAL:   printf("str: %s\n",   v->data.s); break;
    }
}
```

This is basically what Rust's `enum` is, but with compile-time safety.

## Enums

```c
enum Color { RED, GREEN, BLUE };
// RED=0, GREEN=1, BLUE=2 by default

enum HttpStatus {
    OK = 200,
    NOT_FOUND = 404,
    INTERNAL_ERROR = 500
};

// enum values are just int constants
int x = RED + GREEN;  // 0 + 1 = 1
```

C enums have no type safety — you can assign any int to an enum variable.

## typedef

```c
// Without typedef:
struct Point p;      // must say "struct"

// With typedef:
typedef struct {
    int x, y;
} Point;

Point p;  // cleaner

// typedef for function pointers:
typedef int (*BinaryOp)(int, int);

int add(int a, int b) { return a + b; }
BinaryOp op = add;
printf("%d\n", op(3, 4));  // 7
```

## Rust Enums — Much More Powerful

Rust enums are algebraic data types (tagged unions with compiler guarantees):

```rust
// Enum with data attached to variants
enum Shape {
    Circle { radius: f64 },
    Rectangle { width: f64, height: f64 },
    Triangle { base: f64, height: f64 },
}

fn area(shape: &Shape) -> f64 {
    match shape {
        Shape::Circle { radius }            => std::f64::consts::PI * radius * radius,
        Shape::Rectangle { width, height } => width * height,
        Shape::Triangle { base, height }   => 0.5 * base * height,
    }
}

// Option<T> = either Some(value) or None
// This replaces NULL with type safety
fn find(arr: &[i32], target: i32) -> Option<usize> {
    for (i, &v) in arr.iter().enumerate() {
        if v == target { return Some(i); }
    }
    None
}

let arr = [1, 2, 3, 4, 5];
match find(&arr, 3) {
    Some(idx) => println!("found at {}", idx),
    None      => println!("not found"),
}

// Result<T, E> = either Ok(value) or Err(error)
// Replaces error codes and exceptions
fn parse_int(s: &str) -> Result<i32, std::num::ParseIntError> {
    s.parse()
}
```

## Flexible Array Member (C99)

```c
// Useful for variable-length data at end of struct
struct Message {
    int type;
    size_t len;
    char data[];  // flexible array member — zero size in struct
};

// Allocate with extra space for data:
size_t n = 100;
struct Message *msg = malloc(sizeof(struct Message) + n);
msg->type = 1;
msg->len = n;
memcpy(msg->data, "hello...", n);
```

---

# 12. Memory Padding & Alignment

## What is Alignment?

CPUs access memory most efficiently when data is **aligned** — an N-byte type should start at an address divisible by N.

- `char` (1 byte): any address
- `short` (2 bytes): address % 2 == 0
- `int` (4 bytes): address % 4 == 0
- `double` (8 bytes): address % 8 == 0
- `pointer` (8 bytes): address % 8 == 0

If misaligned:
- x86: works but slower (two cache-line accesses)
- ARM/RISC-V: crash (alignment fault / bus error)

## Struct Padding

The compiler inserts **padding bytes** to ensure each field is properly aligned.

```c
struct Bad {
    char  a;     // 1 byte, offset 0
    // 3 bytes padding (to align b to offset 4)
    int   b;     // 4 bytes, offset 4
    char  c;     // 1 byte, offset 8
    // 7 bytes padding (to align d to offset 16)
    double d;    // 8 bytes, offset 16
    // total: 24 bytes
};
```

Memory layout:

```
Offset: 0   1   2   3   4   5   6   7   8   9  10  11  12  13  14  15  16  17...23
        +---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+------+
        | a |PAD PAD PAD|    b (int)    | c |PAD PAD PAD PAD PAD PAD PAD|  d (dbl) |
        +---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+------+
```

```c
struct Good {
    double d;    // 8 bytes, offset 0
    int    b;    // 4 bytes, offset 8
    char   a;    // 1 byte, offset 12
    char   c;    // 1 byte, offset 13
    // 2 bytes padding (for struct array alignment)
    // total: 16 bytes
};
```

**Rule: Order struct fields from largest to smallest alignment to minimize padding.**

```c
// Verify:
printf("Bad:  %zu\n", sizeof(struct Bad));   // 24
printf("Good: %zu\n", sizeof(struct Good));  // 16
```

## Struct Alignment is Whole-Struct Too

The struct itself must be aligned to its largest member's alignment. This affects:
1. Array of structs: each element must be aligned
2. sizeof always includes trailing padding

```c
struct Example {
    int   x;   // 4 bytes, offset 0
    char  c;   // 1 byte, offset 4
    // 3 bytes trailing padding — so next element in array is aligned
};
// sizeof = 8, NOT 5!
```

## Offsetof Macro

```c
#include <stddef.h>

struct S { int a; char b; double c; };

printf("a: %zu\n", offsetof(struct S, a));  // 0
printf("b: %zu\n", offsetof(struct S, b));  // 4
printf("c: %zu\n", offsetof(struct S, c));  // 8 (after 3 bytes padding from b)
```

## Packed Structs (No Padding)

```c
struct __attribute__((packed)) Packed {
    char  a;   // offset 0
    int   b;   // offset 1 — MISALIGNED!
    char  c;   // offset 5
};  // sizeof = 6

// Accessing Packed.b causes:
// - x86: extra work (unaligned access penalty)
// - ARM: possible bus error (SIGBUS)
// Use only for wire formats, binary file I/O, memory-mapped hardware registers
```

---

# 13. Type System

## Integer Types and Sizes

```c
// Guaranteed sizes (C99 <stdint.h>):
int8_t   // exactly 8 bits  = 1 byte,  range: -128 to 127
int16_t  // exactly 16 bits = 2 bytes
int32_t  // exactly 32 bits = 4 bytes
int64_t  // exactly 64 bits = 8 bytes

uint8_t  // unsigned 8-bit,  range: 0 to 255
uint16_t // unsigned 16-bit, range: 0 to 65535
uint32_t // unsigned 32-bit, range: 0 to 4,294,967,295
uint64_t // unsigned 64-bit, range: 0 to 18,446,744,073,709,551,615

// Platform-dependent (use stdint.h types for portability):
char     // 1 byte (signed or unsigned — implementation defined!)
short    // >= 16 bits (usually 2 bytes)
int      // >= 16 bits (usually 4 bytes on modern systems)
long     // >= 32 bits (4 bytes on Windows 64-bit, 8 bytes on Linux 64-bit!)
long long // >= 64 bits (usually 8 bytes)

size_t   // unsigned, platform-sized (size of addressable memory)
ptrdiff_t // signed, result of pointer subtraction
```

## Signed Integer Overflow — Undefined Behavior in C!

```c
int x = INT_MAX;  // 2147483647
int y = x + 1;    // UNDEFINED BEHAVIOR — signed overflow is UB in C
// Compiler is allowed to ASSUME this never happens
// May produce: -2147483648 (wrapping), INT_MAX, crash, or anything

// Unsigned overflow is NOT undefined behavior — it wraps:
unsigned int u = UINT_MAX;  // 4294967295
unsigned int v = u + 1;     // well-defined: wraps to 0
```

Why does this matter for the compiler?

```c
// Code:
for (int i = 0; i <= INT_MAX; i++) { ... }

// Compiler reasoning:
// "i <= INT_MAX is always true (since signed overflow is UB,
//  the loop condition can never be false)"
// Compiler may turn this into an infinite loop!
```

## Type Conversion Rules

```c
// Integer promotions: char/short promoted to int in expressions
char c = 200;  // 200 as char (may be -56 if signed!)
int x = c + 1; // c is promoted to int before addition

// Usual arithmetic conversions:
// int + long = long
// int + float = float
// int + double = double

// Signed/unsigned mixing:
int s = -1;
unsigned int u = 1;
if (s < u) {  // SURPRISE: s is converted to unsigned!
              // -1 becomes 4294967295 (wraps around)
              // 4294967295 > 1, so condition is FALSE!
    printf("negative is smaller\n");  // NOT printed!
}
```

## Volatile

```c
// volatile tells compiler: "do NOT cache this in registers, 
// read/write it from actual memory every time"

// Use cases:
// 1. Memory-mapped hardware registers
volatile uint32_t *UART_STATUS = (volatile uint32_t *)0x40001000;
while (!(*UART_STATUS & 0x01));  // poll until TX ready
// Without volatile, compiler might read register once into register
// and loop forever on the cached value

// 2. Signal handlers
volatile sig_atomic_t shutdown = 0;
void sighandler(int sig) { shutdown = 1; }
while (!shutdown) { /* work */ }  // correctly re-reads shutdown each iteration

// 3. Thread communication (use proper atomics instead in real code)
```

## restrict

```c
// restrict tells compiler: no other pointer aliases this memory
// Enables vectorization and other optimizations

void add_arrays(int *restrict a, int *restrict b, int *restrict c, int n) {
    for (int i = 0; i < n; i++) {
        c[i] = a[i] + b[i];
    }
    // Without restrict: compiler must assume a, b, c may overlap
    // → cannot use SIMD (must be sequential)
    // With restrict: compiler can safely vectorize
}
```

## Strict Aliasing

```c
// C standard says: you cannot access an object through a pointer
// of a different type (except char*)
// Violating this is undefined behavior

int x = 42;
float *fp = (float*)&x;  // STRICT ALIASING VIOLATION
*fp = 1.0f;              // UB — accessing int through float*

// Exception: char* can alias anything
char *cp = (char*)&x;    // OK — char* is always allowed
cp[0] = 0;               // OK

// Safe type punning via union or memcpy:
float safe_pun(int x) {
    float result;
    memcpy(&result, &x, sizeof(float));  // safe — memcpy uses char*
    return result;
}
```

---

# 14. Bits & Bytes

## Bitwise Operators

```c
// AND (&): both bits must be 1
0b1010 & 0b1100 = 0b1000  (8)

// OR (|): at least one bit must be 1
0b1010 | 0b1100 = 0b1110  (14)

// XOR (^): exactly one bit must be 1
0b1010 ^ 0b1100 = 0b0110  (6)

// NOT (~): flip all bits
~0b00001010 = 0b11110101  (for 8-bit)

// Left shift (<<): multiply by 2^n
5 << 3 = 5 * 8 = 40

// Right shift (>>):
// For unsigned/positive: divide by 2^n (logical shift — fills with 0)
// For negative signed: implementation-defined (arithmetic or logical)
40 >> 3 = 5
```

## Common Bit Manipulation Patterns

```c
uint32_t flags = 0;

// Set bit n:
flags |= (1u << n);

// Clear bit n:
flags &= ~(1u << n);

// Toggle bit n:
flags ^= (1u << n);

// Check bit n:
bool is_set = (flags >> n) & 1;
// or:
bool is_set = (flags & (1u << n)) != 0;

// Extract bits [high:low] (inclusive):
uint32_t extract(uint32_t val, int low, int high) {
    int width = high - low + 1;
    uint32_t mask = (1u << width) - 1;
    return (val >> low) & mask;
}

// Clear lowest set bit:
n &= (n - 1);  // classic trick

// Check if power of 2:
bool is_pow2 = (n > 0) && (n & (n - 1)) == 0;

// Lowest set bit position:
int lsb = __builtin_ctz(n);  // GCC/Clang built-in
```

## Endianness

Endianness is the **byte order** used to store multi-byte values.

```
Value: 0x12345678 stored at address 0x1000

Little-endian (x86, x86-64, ARM in LE mode):
Address: 0x1000 0x1001 0x1002 0x1003
         +------+------+------+------+
         | 0x78 | 0x56 | 0x34 | 0x12 |
         +------+------+------+------+
         LSB                    MSB
         (Least Significant Byte first)

Big-endian (network byte order, MIPS, SPARC):
Address: 0x1000 0x1001 0x1002 0x1003
         +------+------+------+------+
         | 0x12 | 0x34 | 0x56 | 0x78 |
         +------+------+------+------+
         MSB                    LSB
         (Most Significant Byte first)
```

Detecting endianness:

```c
#include <stdint.h>
#include <stdbool.h>

bool is_little_endian() {
    uint32_t x = 1;
    return *(uint8_t*)&x == 1;
    // On LE: bytes are [01 00 00 00], first byte is 1 → true
    // On BE: bytes are [00 00 00 01], first byte is 0 → false
}

// Or using union:
bool is_little_endian_v2() {
    union { uint32_t i; uint8_t c[4]; } test = { .i = 1 };
    return test.c[0] == 1;
}
```

Byte swapping (for network/protocol code):

```c
#include <arpa/inet.h>

uint32_t host_val = 0x12345678;
uint32_t network_val = htonl(host_val);  // host-to-network long (BE)
uint32_t back = ntohl(network_val);      // network-to-host long

// Manual swap:
uint32_t bswap32(uint32_t x) {
    return ((x >> 24) & 0xFF)        |
           ((x >> 8)  & 0xFF00)      |
           ((x << 8)  & 0xFF0000)    |
           ((x << 24) & 0xFF000000);
}
// GCC built-in: __builtin_bswap32(x)
```

## Bit Fields in Structs

```c
struct Flags {
    unsigned int is_active  : 1;   // 1 bit
    unsigned int priority   : 3;   // 3 bits (values 0-7)
    unsigned int type       : 4;   // 4 bits (values 0-15)
    unsigned int            : 0;   // force next field to new word boundary
    unsigned int data       : 16;  // 16 bits
};

struct Flags f = {0};
f.is_active = 1;
f.priority  = 5;
f.type      = 3;
```

Layout (implementation-dependent — not portable!):

```
Bit: 31...16 15...12 11...9 8...5 4  3  2  1  0
     [  data  ][type  ][reserved][priority][active]
```

---

# 15. Preprocessor & Compilation Pipeline

## The Four Stages of C Compilation

```
source.c
    |
    ▼  Stage 1: Preprocessing (cpp)
    |   - Expand #include files
    |   - Expand #define macros
    |   - Process #if/#ifdef/#endif
    |   - Strip comments
    |   Output: source.i (preprocessed C)
    |
    ▼  Stage 2: Compilation (cc1)
    |   - Parse C syntax into AST
    |   - Semantic analysis
    |   - Optimization passes
    |   - Generate assembly
    |   Output: source.s (assembly)
    |
    ▼  Stage 3: Assembly (as)
    |   - Convert assembly to machine code
    |   - Generate object file with symbol table
    |   Output: source.o (object file)
    |
    ▼  Stage 4: Linking (ld)
    |   - Combine multiple .o files
    |   - Resolve symbol references
    |   - Link with libraries (libc, etc.)
    |   Output: a.out (executable)
    ▼

# Run each stage manually:
gcc -E source.c -o source.i    # preprocessor only
gcc -S source.i -o source.s    # compile to assembly
gcc -c source.s -o source.o    # assemble to object
gcc source.o -o program        # link
```

## Header Files and Include Guards

```c
// mylib.h — without guard (dangerous):
struct Point { int x, y; };  // may be defined multiple times if included twice!

// mylib.h — with include guard (correct):
#ifndef MYLIB_H
#define MYLIB_H

struct Point { int x, y; };

void point_print(struct Point *p);

#endif  // MYLIB_H

// Alternative: pragma once (not C standard, but widely supported)
#pragma once
struct Point { int x, y; };
```

## Macros — Textual Substitution

```c
// Object-like macros:
#define PI 3.14159265358979
#define MAX_SIZE 1024
#define DEBUG_MODE  // just defined, no value (for #ifdef)

// Function-like macros:
#define MAX(a, b) ((a) > (b) ? (a) : (b))
// NOTE: extra parentheses are critical!

// Without parens — broken:
#define BAD_DOUBLE(x) x * 2
BAD_DOUBLE(3 + 4)  // expands to 3 + 4 * 2 = 11, NOT 14!

// With parens — correct:
#define DOUBLE(x) ((x) * 2)
DOUBLE(3 + 4)      // ((3 + 4) * 2) = 14

// Double-evaluation problem:
#define MAX(a, b) ((a) > (b) ? (a) : (b))
int x = 5, y = 3;
MAX(x++, y++)  // expands to: ((x++) > (y++) ? (x++) : (y++))
               // x is incremented TWICE if x > y!
               // This is why macros are dangerous for side effects

// Inline functions are safer (and faster in optimized builds):
static inline int max(int a, int b) { return a > b ? a : b; }
```

## Stringification and Concatenation

```c
// # turns argument into string literal:
#define STRINGIFY(x) #x
STRINGIFY(hello)  // → "hello"
STRINGIFY(42)     // → "42"

// Useful for debug printing:
#define DEBUG_INT(x) printf(#x " = %d\n", x)
int count = 42;
DEBUG_INT(count);  // prints: count = 42

// ## concatenates tokens:
#define MAKE_FUNC(type, name) type get_##name() { return name##_data; }
MAKE_FUNC(int, count)  // → int get_count() { return count_data; }
```

## Conditional Compilation

```c
#define PLATFORM_LINUX

#ifdef PLATFORM_LINUX
    #include <unistd.h>
    #define NEWLINE "\n"
#elif defined(PLATFORM_WINDOWS)
    #include <windows.h>
    #define NEWLINE "\r\n"
#else
    #error "Unsupported platform"
#endif

// Version checking:
#if __GNUC__ >= 9
    // Use GCC 9+ features
#endif

// Debug assertions:
#ifndef NDEBUG
    #define ASSERT(cond) do { \
        if (!(cond)) { \
            fprintf(stderr, "Assertion failed: %s at %s:%d\n", \
                    #cond, __FILE__, __LINE__); \
            abort(); \
        } \
    } while(0)
#else
    #define ASSERT(cond) ((void)0)
#endif
```

## The Linker and Symbol Resolution

```
file1.c:
    extern int shared_var;    // declaration (no memory allocated)
    void use_it() { printf("%d", shared_var); }

file2.c:
    int shared_var = 42;      // definition (memory allocated in DATA)
    void use_it();
    int main() { use_it(); }

Linker resolves:
file1.o: UNDEFINED symbol: shared_var
file2.o: DEFINED symbol:   shared_var at data+0x0

Linker: file1.o's reference to shared_var → file2.o's definition
Result: a.out where use_it() accesses the right memory location
```

Static vs dynamic linking:

```
Static linking (gcc -static):
    All library code COPIED into executable
    Larger binary, no runtime dependencies
    Portable

Dynamic linking (default):
    Library code NOT in executable
    At runtime, ld.so loads .so files
    Smaller binary, shared between processes
    Requires library installed on target system
```

---

# 16. Undefined Behavior

## Why UB Exists in C

The C standard deliberately leaves many behaviors undefined to give compilers maximum freedom to optimize. The standard reasons:
- Different platforms have different natural behaviors (overflow, shifts, etc.)
- Compilers can assume UB never occurs → better optimizations
- Systems programmers should know their target platform

## The Optimizer Uses UB as License

```c
// The C compiler is allowed to assume NO signed overflow occurs.
// This means if you write:
int *search(int *arr, int n, int target) {
    for (int i = 0; i < n; i++) {
        if (arr[i] == target) return &arr[i];
    }
    return NULL;
}

// And you call search(arr, INT_MAX, x):
// The compiler KNOWS i < INT_MAX is always true (signed overflow UB)
// So it may turn the loop into an INFINITE loop
```

## Sequence Points

A sequence point is a point in execution where all side effects of previous expressions are complete. Between sequence points, order of evaluation is undefined.

```c
int i = 5;
int x = i++ + i++;  // UNDEFINED BEHAVIOR
// Which i++ happens first? Standard doesn't say.
// Could be: (5 + 6) = 11 with i=7
// Or:       (5 + 5) = 10 with i=7
// Or anything else

// DEFINED behaviors:
int a = i++;   // a = 5, i = 6 — sequence point between statements
int b = ++i;   // b = 7, i = 7

// Function arguments are also unsequenced:
printf("%d %d\n", i++, i++);  // UB — which is evaluated first?
```

## Format String Vulnerability

```c
char msg[100];
fgets(msg, sizeof(msg), stdin);  // user inputs format string!

printf(msg);           // DANGEROUS — if msg contains %s, %n, etc.
printf("%s", msg);     // SAFE — msg is always treated as string, not format
```

What an attacker can do with a format string vulnerability:
- `%x` — read stack values (info leak)
- `%s` — read arbitrary memory addresses
- `%n` — WRITE to memory (the count of bytes printed so far)

---

# 17. Go Runtime & Goroutine Scheduler

## Go's M:N Threading Model

Go uses M:N threading (many goroutines on fewer OS threads):

```
Goroutines (G):     G1  G2  G3  G4  G5  G6  G7  G8 ... G10000
                    (lightweight, ~2KB stack each)
                      \   /   \   /   \   /   \   /
Processors (P):        P1      P2      P3      P4
                    (each has a run queue of goroutines)
                         \         /
                       M1  M2  M3  M4
                    (OS threads — typically GOMAXPROCS of them)
                         |   |   |   |
                    Linux threads (POSIX pthreads)
                         |   |   |   |
                         CPU cores
```

Key: `GOMAXPROCS` goroutines run truly in parallel (default = number of CPU cores). Other goroutines wait in run queues.

## Goroutine vs OS Thread

| Property | OS Thread | Goroutine |
|----------|-----------|-----------|
| Stack size | 1-8 MB (fixed) | 2KB (grows dynamically to GB) |
| Creation cost | ~10ms | ~microseconds |
| Context switch | OS managed, expensive | Go runtime, cheap |
| Blocking | Blocks OS thread | Can yield to other goroutines |
| Quantity | Thousands max | Millions possible |
| Scheduling | Preemptive (OS) | Cooperative + preemptive (Go) |

## Goroutine Stack Growth

```
Initial goroutine stack: 2KB

+-------------------+
|  function frame   |
|  function frame   |
|  function frame   |
+-------------------+

Stack overflow check at function entry (not hardware):
if (sp < stackguard) {
    // grow the stack
    runtime.morestack()
}

Goroutine stack after growth: 4KB (doubling)
+-----------------------------------+
|  function frame (copied from old) |
|  function frame (copied from old) |
|  function frame (copied from old) |
|  new frame                        |
+-----------------------------------+
```

Stack growth uses **stack copying** (not segmented stacks anymore as of Go 1.4):
- New larger stack allocated
- All frames copied
- All pointers updated
- Old stack freed

This is why you cannot store raw addresses of stack-allocated Go variables across function boundaries — they may move.

## Go Scheduler Internals

The scheduler is work-stealing, with per-P run queues:

```
P1's run queue:           P2's run queue:
[G3, G7, G12, G15]        [G2, G9]

P1 is busy executing G5.
P2 finishes G2, run queue now [G9].

P2 steals half of P1's queue:
P1: [G3, G7]  P2: [G12, G15, G9]
                   (stolen)  (own)
```

Scheduler events (when goroutines are scheduled/descheduled):
1. **Channel operations** (block waiting for channel)
2. **System calls** (network I/O, file I/O — Go uses `epoll/kqueue` internally)
3. **runtime.Gosched()** (voluntary yield)
4. **Preemption at function calls** (>= Go 1.14: async preemption via signals)
5. **Garbage collector** (stop-the-world pauses, very short)

## Channel Internals

```
ch := make(chan int, 3)  // buffered channel, capacity 3

Internal structure:
+----------------------------------------------+
|  buf:    [G1_val | G2_val |        ]          |  ring buffer
|  sendx:  2  (next write position)             |
|  recvx:  0  (next read position)              |
|  qcount: 2  (current count)                   |
|  dataqsiz: 3 (capacity)                       |
|  sendq:  [] (goroutines blocked on send)      |
|  recvq:  [] (goroutines blocked on receive)   |
|  lock:   mutex                                |
+----------------------------------------------+

When ch <- val:
  if buffer not full:  write to buf[sendx], advance sendx, done
  if buffer full:      park goroutine on sendq, yield

When val := <-ch:
  if buffer not empty: read from buf[recvx], advance recvx, wake sendq
  if buffer empty:     park goroutine on recvq, yield
```

---

# 18. Go: yield and Cooperative Scheduling

## runtime.Gosched()

```go
package main

import (
    "fmt"
    "runtime"
    "sync"
)

func worker(id int, wg *sync.WaitGroup) {
    defer wg.Done()
    for i := 0; i < 5; i++ {
        fmt.Printf("worker %d: step %d\n", id, i)
        runtime.Gosched()  // yield — let other goroutines run
    }
}

func main() {
    var wg sync.WaitGroup
    for i := 1; i <= 3; i++ {
        wg.Add(1)
        go worker(i, &wg)
    }
    wg.Wait()
}
```

What `runtime.Gosched()` does internally:
1. Save current goroutine state (registers, PC, SP)
2. Put goroutine back in P's run queue (state: Runnable)
3. Call `schedule()` — pick next goroutine from run queue
4. Restore new goroutine's state and run it

The old goroutine will run again, but not necessarily next.

## Channel as Generator (Python yield equivalent)

```go
// Python:
// def fibonacci():
//     a, b = 0, 1
//     while True:
//         yield a
//         a, b = b, a+b

// Go equivalent:
func fibonacci() <-chan int {
    ch := make(chan int)
    go func() {
        a, b := 0, 1
        for {
            ch <- a  // "yield" — blocks until receiver reads
            a, b = b, a+b
        }
    }()
    return ch
}

func main() {
    gen := fibonacci()
    for i := 0; i < 10; i++ {
        fmt.Println(<-gen)  // 0, 1, 1, 2, 3, 5, 8, 13, 21, 34
    }
    // Note: generator goroutine is still alive, blocked on ch <- a
    // It will be garbage collected when ch is no longer reachable
}
```

## Go 1.23+ Iterators (range-over-func)

Go 1.22 introduced `iter` package patterns. The idiomatic yield function:

```go
// Iterator function signature for ranging:
// func(yield func(V) bool)
// yield returns false when caller wants to stop (e.g., break in for-range)

func integers(n int) func(yield func(int) bool) {
    return func(yield func(int) bool) {
        for i := 0; i < n; i++ {
            if !yield(i) {
                return  // caller broke out of range loop
            }
        }
    }
}

// Range over the iterator:
for i := range integers(5) {
    fmt.Println(i)  // 0, 1, 2, 3, 4
}
```

## Preventing CPU Spin

```go
// BAD: 100% CPU usage
for !done.Load() {
}

// BETTER: yield scheduler
for !done.Load() {
    runtime.Gosched()
}

// BEST: use synchronization primitives
var mu sync.Mutex
var cond = sync.NewCond(&mu)

// Waiter:
mu.Lock()
for !condition {
    cond.Wait()  // releases lock, parks goroutine
}
mu.Unlock()

// Notifier:
mu.Lock()
condition = true
cond.Signal()  // wake one waiter
mu.Unlock()
```

---

# 19. Go: Large File I/O

## The Problem with os.ReadFile

```go
// This reads the ENTIRE file into memory:
data, err := os.ReadFile("/var/log/huge.log")  // 10GB → 10GB RAM
// Causes: OOM, swap thrashing, crash
```

## Streaming with bufio.Scanner

```go
package main

import (
    "bufio"
    "fmt"
    "os"
)

func processLargeFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)

    // Default max token size is 64KB
    // For long lines, increase the buffer:
    const maxLineSize = 10 * 1024 * 1024  // 10MB
    buf := make([]byte, 64*1024)
    scanner.Buffer(buf, maxLineSize)

    lineNum := 0
    for scanner.Scan() {
        lineNum++
        line := scanner.Text()     // string (allocated)
        // OR: scanner.Bytes()     // []byte (reused — zero allocation!)
        
        process(line)
    }

    return scanner.Err()
}
```

How `bufio.Scanner` works internally:

```
Disk ──→ os.File.Read() ──→ bufio internal buffer (4KB default) ──→ your code
                              ^
                              |
                      Scanner reads chunks here
                      finds delimiters (\n by default)
                      returns tokens one at a time
                      refills buffer as needed
```

## bufio.Reader for Maximum Control

```go
func streamFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    // Large buffer = fewer syscalls = better throughput
    reader := bufio.NewReaderSize(f, 256*1024)  // 256KB buffer

    for {
        // ReadString reads until delimiter (inclusive)
        line, err := reader.ReadString('\n')
        
        if len(line) > 0 {
            // Process line even if err != nil
            // (last line may not have \n)
            processLine(line)
        }
        
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
    }
    return nil
}
```

## Fixed-Size Chunk Reading (Binary Data)

```go
func readBinaryFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    // Allocate buffer ONCE outside loop — critical for performance
    buf := make([]byte, 32*1024)  // 32KB chunks

    for {
        n, err := f.Read(buf)
        if n > 0 {
            chunk := buf[:n]
            processChunk(chunk)
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
    }
    return nil
}
```

## Seeking in Large Files

```go
func readTail(path string, nBytes int64) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    // io.SeekEnd = 2 (from end of file)
    _, err = f.Seek(-nBytes, io.SeekEnd)
    if err != nil {
        return nil, err
    }

    return io.ReadAll(f)  // ReadAll only the tail portion
}

// Seek constants:
// io.SeekStart   = 0 (from beginning)
// io.SeekCurrent = 1 (from current position)
// io.SeekEnd     = 2 (from end)
```

## Memory-Mapped Files

For maximum performance (databases, search engines):

```go
import (
    "golang.org/x/exp/mmap"
    "os"
    "syscall"
)

// Method 1: golang.org/x/exp/mmap
func readWithMmap(path string) error {
    r, err := mmap.Open(path)
    if err != nil {
        return err
    }
    defer r.Close()

    // Access file like a byte slice — OS handles paging
    buf := make([]byte, 1024)
    n, err := r.ReadAt(buf, 0)  // read 1024 bytes at offset 0
    _ = buf[:n]
    return err
}

// Method 2: syscall.Mmap directly
func mmapDirect(path string) ([]byte, func(), error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, nil, err
    }
    
    fi, err := f.Stat()
    if err != nil {
        f.Close()
        return nil, nil, err
    }
    
    data, err := syscall.Mmap(
        int(f.Fd()),
        0,
        int(fi.Size()),
        syscall.PROT_READ,
        syscall.MAP_SHARED,
    )
    if err != nil {
        f.Close()
        return nil, nil, err
    }
    
    cleanup := func() {
        syscall.Munmap(data)
        f.Close()
    }
    
    return data, cleanup, nil
}
```

How mmap works:

```
Without mmap:
Process ──read()syscall──▶ kernel copies data ──▶ your buffer ──▶ process uses data
                           (two copies: disk→page cache, page cache→userspace)

With mmap:
Process ──mmap()──▶ maps file pages into virtual address space
Process reads data ──▶ PAGE FAULT ──▶ kernel loads page from disk to page cache
                       page cache is DIRECTLY mapped into process virtual memory
                       (one copy: disk→page cache, no extra userspace copy)
Process writes ──▶ write directly to page cache ──▶ OS flushes to disk lazily
```

## Buffered Writer for Output

```go
func writeEfficiently(path string, data []string) error {
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    defer f.Close()

    // Without bufio: each Write() is a syscall → slow
    // With bufio: accumulates writes in 4KB buffer, syscall only when full
    w := bufio.NewWriterSize(f, 64*1024)  // 64KB buffer
    defer w.Flush()  // flush remaining buffer to file on exit

    for _, line := range data {
        fmt.Fprintln(w, line)
    }
    return nil
}
```

---

# 20. Rust Ownership & Borrowing

## The Core Invariant

Rust's entire memory safety system is built on one rule enforced at compile time:

**At any point, for any piece of data, you can have EITHER:**
- **One mutable reference** (`&mut T`)
- **Any number of immutable references** (`&T`)
- **But NEVER both at the same time**

This is called the **XOR mutable aliasing rule**. It eliminates:
- Data races (one writer + multiple readers = race)
- Use-after-free (owner goes away, references become dangling)
- Double-free (only one owner can drop)
- Iterator invalidation (mutation while iterating)

## Ownership Rules

```
1. Every value has exactly ONE owner (variable).
2. When the owner goes out of scope, the value is dropped (freed).
3. Ownership can be transferred (moved) but not copied (unless type is Copy).
```

```rust
fn main() {
    // s1 owns the String
    let s1 = String::from("hello");
    
    // MOVE: ownership transferred from s1 to s2
    let s2 = s1;
    
    // s1 is no longer valid:
    // println!("{}", s1);  // ERROR: value borrowed after move
    
    println!("{}", s2);  // OK: s2 is the owner
    
    // s2 goes out of scope here → String is freed (drop runs)
}
```

Memory diagram:

```
After let s1 = String::from("hello"):
Stack:        s1
              +----------+-----+-----+
              | ptr      | len | cap |
              +--|-------+--5--+--5--+
                 |
                 ▼
Heap:          +---+---+---+---+---+
               | h | e | l | l | o |
               +---+---+---+---+---+

After let s2 = s1 (MOVE):
Stack:        s1          s2
              [INVALID]   +----------+-----+-----+
                          | ptr      | len | cap |
                          +--|-------+--5--+--5--+
                             |
                             ▼ (same heap data)
Heap:                      +---+---+---+---+---+
                           | h | e | l | l | o |
                           +---+---+---+---+---+
```

The move makes s1 invalid so the compiler prevents any access to s1. The heap data is owned exactly once (by s2), so it's freed exactly once (when s2 goes out of scope).

## Borrowing

```rust
fn main() {
    let s = String::from("hello");
    
    // Immutable borrow — can have many
    let r1 = &s;
    let r2 = &s;
    println!("{} {}", r1, r2);  // OK: multiple & borrows
    
    // After r1 and r2 are last used, they're dropped (non-lexical lifetimes)
    
    // Now we can mutably borrow:
    let mut s2 = String::from("world");
    let rm = &mut s2;
    rm.push_str("!!!");
    println!("{}", rm);   // OK
    // Can't use s2 directly while rm is live:
    // println!("{}", s2);  // ERROR: s2 is mutably borrowed
    
    println!("{}", s2);   // OK: rm is no longer used above
}
```

## The Borrow Checker's Algorithm

The borrow checker tracks:
1. The **lifetime** (scope) of every reference
2. Whether references **overlap** in conflicting ways

```rust
fn main() {
    let mut v = vec![1, 2, 3];
    
    let first = &v[0];      // immutable borrow of v
    v.push(4);              // ERROR: mutable operation on v while borrowed!
    // Why? v.push() may reallocate the buffer.
    // If it does, first points to the old (freed) buffer.
    // This is the "iterator invalidation" bug that C++ silently allows.
    
    println!("{}", first);  // this is why the borrow must still be live
}
```

Rust error:
```
error[E0502]: cannot borrow `v` as mutable because it is also borrowed as immutable
```

In C++, this compiles and runs — but is undefined behavior (dangling reference into vector after push_back causes reallocation).

## Lifetimes

Lifetime annotations describe how long references live. They're mostly inferred by the compiler.

```rust
// This function signature is incomplete — the compiler can't infer lifetimes:
fn longest(x: &str, y: &str) -> &str {  // ERROR: missing lifetime specifier
    if x.len() > y.len() { x } else { y }
}

// With lifetime annotation:
fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {
    // 'a means: the returned reference lives as long as
    // the SHORTER of x's and y's lifetimes
    if x.len() > y.len() { x } else { y }
}

fn main() {
    let s1 = String::from("long string");
    let result;
    {
        let s2 = String::from("xy");
        result = longest(s1.as_str(), s2.as_str());
        println!("{}", result);  // OK: s2 still alive here
    }
    // println!("{}", result);  // ERROR: s2 is dropped, result may point to it
}
```

## Smart Pointers

```rust
use std::rc::Rc;
use std::cell::RefCell;
use std::sync::{Arc, Mutex};

fn main() {
    // Box<T>: heap allocation, unique ownership
    let b = Box::new(5);
    println!("{}", *b);
    
    // Rc<T>: reference counted, multiple owners (single-threaded)
    let rc1 = Rc::new(String::from("shared"));
    let rc2 = Rc::clone(&rc1);  // increment reference count
    println!("{} refs", Rc::strong_count(&rc1));  // 2
    // Freed when count reaches 0
    
    // RefCell<T>: runtime borrow checking (interior mutability)
    let val = RefCell::new(5);
    *val.borrow_mut() += 1;  // runtime check, panics on violation
    
    // Arc<T> + Mutex<T>: thread-safe shared state
    let shared = Arc::new(Mutex::new(0));
    let clone = Arc::clone(&shared);
    
    let handle = std::thread::spawn(move || {
        let mut guard = clone.lock().unwrap();
        *guard += 1;
    });
    
    handle.join().unwrap();
    println!("{}", *shared.lock().unwrap());  // 1
}
```

---

# 21. Rust Lifetimes

## What Problem Lifetimes Solve

The borrow checker needs to verify that references don't outlive what they point to. For simple cases, it can infer this. For functions that return references, it needs guidance.

```rust
// The borrow checker sees:
fn first_word(s: &str) -> &str {
    let bytes = s.as_bytes();
    for (i, &byte) in bytes.iter().enumerate() {
        if byte == b' ' {
            return &s[..i];
        }
    }
    s
}
// Implicit lifetimes (elision rules apply — one input reference):
// fn first_word<'a>(s: &'a str) -> &'a str
// Return value lives as long as s
```

## Lifetime in Structs

```rust
// Struct that holds a reference must have a lifetime annotation:
struct Important<'a> {
    excerpt: &'a str,   // cannot outlive the string it references
}

fn main() {
    let novel = String::from("Call me Ishmael. Some years ago...");
    let first_sentence;
    {
        let i = novel.find('.').unwrap_or(novel.len());
        first_sentence = &novel[..i];
    }
    
    let imp = Important { excerpt: first_sentence };
    println!("{}", imp.excerpt);
    // novel is alive here, so first_sentence (which is &novel[..]) is valid
}
```

## 'static Lifetime

`'static` means the reference lives for the entire program duration.

```rust
// String literals are 'static — they're in the binary
let s: &'static str = "hello world";

// Can be returned from functions — never dangling:
fn get_greeting() -> &'static str {
    "Hello!"  // lives in TEXT segment for entire program
}
```

---

# 22. Data Structures

## 22.1 Array / Slice

The most fundamental data structure — contiguous memory.

```
Array of 6 ints:
Index:   [0]  [1]  [2]  [3]  [4]  [5]
         +----+----+----+----+----+----+
         | 10 | 20 | 30 | 40 | 50 | 60 |
         +----+----+----+----+----+----+
Addr:    1000 1004 1008 1012 1016 1020

Access by index: O(1) — multiply index by element size, add to base address
addr(arr[i]) = base + i * sizeof(T)

Search (unsorted): O(n) — linear scan
Search (sorted):   O(log n) — binary search
Insert/delete:     O(n) — must shift elements
```

C implementation:
```c
// Static array:
int arr[6] = {10, 20, 30, 40, 50, 60};

// Dynamic array:
int *arr = malloc(6 * sizeof(int));
arr[0] = 10;  // arr[i] = *(arr + i)

// Binary search:
int bsearch(int *arr, int n, int target) {
    int lo = 0, hi = n - 1;
    while (lo <= hi) {
        int mid = lo + (hi - lo) / 2;  // avoids overflow vs (lo+hi)/2
        if (arr[mid] == target) return mid;
        if (arr[mid] < target)  lo = mid + 1;
        else                    hi = mid - 1;
    }
    return -1;
}
```

## 22.2 Linked List

```
Singly linked list:

head → [10|→] → [20|→] → [30|→] → [40|NULL]
        node0    node1    node2     node3

Each node:
+--------+--------+
| data   | next * |
+--------+--------+

Doubly linked list:

NULL ← [10|→] ⇄ [20|→] ⇄ [30|→] ⇄ [40|→] → NULL
       ←[prev]  ←[prev]  ←[prev]  ←[prev]

Each node:
+--------+--------+--------+
| prev * | data   | next * |
+--------+--------+--------+
```

```c
typedef struct Node {
    int data;
    struct Node *next;
} Node;

// Prepend: O(1)
Node *prepend(Node *head, int val) {
    Node *n = malloc(sizeof(Node));
    n->data = val;
    n->next = head;
    return n;  // new head
}

// Append: O(n) (without tail pointer)
Node *append(Node *head, int val) {
    Node *n = malloc(sizeof(Node));
    n->data = val;
    n->next = NULL;
    
    if (head == NULL) return n;
    
    Node *cur = head;
    while (cur->next != NULL) cur = cur->next;
    cur->next = n;
    return head;
}

// Delete by value: O(n)
Node *delete_val(Node *head, int val) {
    // Handle head deletion:
    while (head && head->data == val) {
        Node *tmp = head;
        head = head->next;
        free(tmp);
    }
    
    Node *cur = head;
    while (cur && cur->next) {
        if (cur->next->data == val) {
            Node *tmp = cur->next;
            cur->next = tmp->next;
            free(tmp);
        } else {
            cur = cur->next;
        }
    }
    return head;
}

// Print all:
void print_list(Node *head) {
    while (head) {
        printf("%d → ", head->data);
        head = head->next;
    }
    printf("NULL\n");
}
```

Go linked list:

```go
type Node struct {
    data int
    next *Node
}

func prepend(head *Node, val int) *Node {
    return &Node{data: val, next: head}
}
```

Rust linked list (ownership makes this tricky — Box for ownership):

```rust
#[derive(Debug)]
enum List {
    Cons(i32, Box<List>),
    Nil,
}

impl List {
    fn prepend(self, val: i32) -> List {
        List::Cons(val, Box::new(self))
    }
    
    fn len(&self) -> usize {
        match self {
            List::Nil => 0,
            List::Cons(_, tail) => 1 + tail.len(),
        }
    }
}

fn main() {
    let list = List::Nil
        .prepend(3)
        .prepend(2)
        .prepend(1);
    println!("length: {}", list.len());
}
```

## 22.3 Stack (LIFO)

```
Push 10:  [10]
Push 20:  [10][20]
Push 30:  [10][20][30]
             top →

Pop:  returns 30, stack: [10][20]
Peek: returns 20, stack unchanged: [10][20]

Operations:
  push: O(1)
  pop:  O(1)
  peek: O(1)

Implementation options:
  1. Array with top index (preferred — cache friendly)
  2. Linked list with head as top
```

```c
#define STACK_MAX 1000

typedef struct {
    int data[STACK_MAX];
    int top;  // index of top element (-1 if empty)
} Stack;

void stack_init(Stack *s) { s->top = -1; }

bool stack_push(Stack *s, int val) {
    if (s->top >= STACK_MAX - 1) return false;  // overflow
    s->data[++s->top] = val;
    return true;
}

bool stack_pop(Stack *s, int *val) {
    if (s->top < 0) return false;  // underflow
    *val = s->data[s->top--];
    return true;
}

int stack_peek(Stack *s) {
    return s->data[s->top];
}
```

## 22.4 Queue (FIFO)

```
Circular array queue (capacity 5):

Initial: head=0, tail=0, size=0
[_][_][_][_][_]

Enqueue 10: head=0, tail=1, size=1
[10][_][_][_][_]
 ^head  ^tail

Enqueue 20, 30: head=0, tail=3, size=3
[10][20][30][_][_]
 ^head      ^tail

Dequeue: returns 10, head=1, size=2
[__][20][30][_][_]
     ^head   ^tail

Enqueue 40, 50: head=1, tail=0 (wraps!), size=4
[50][20][30][40][_]
^tail^head

This is why it's called a "circular" buffer.
```

```c
typedef struct {
    int *data;
    int  head, tail;
    int  size, cap;
} Queue;

Queue *queue_new(int cap) {
    Queue *q = malloc(sizeof(Queue));
    q->data = malloc(cap * sizeof(int));
    q->head = q->tail = q->size = 0;
    q->cap = cap;
    return q;
}

bool enqueue(Queue *q, int val) {
    if (q->size == q->cap) return false;
    q->data[q->tail] = val;
    q->tail = (q->tail + 1) % q->cap;  // wrap around
    q->size++;
    return true;
}

bool dequeue(Queue *q, int *val) {
    if (q->size == 0) return false;
    *val = q->data[q->head];
    q->head = (q->head + 1) % q->cap;  // wrap around
    q->size--;
    return true;
}
```

## 22.5 Hash Table

```
Hash table with chaining (load factor = size/buckets):

buckets array:
[0] → NULL
[1] → ["alice" | 25] → NULL
[2] → ["bob" | 30] → ["carol" | 22] → NULL  ← collision!
[3] → NULL
[4] → ["dave" | 40] → NULL
[5] → NULL
[6] → ["eve" | 28] → NULL
[7] → NULL

hash("alice") % 8 = 1  → bucket[1]
hash("bob")   % 8 = 2  → bucket[2]
hash("carol") % 8 = 2  → bucket[2] (collision → chain)
hash("dave")  % 8 = 4  → bucket[4]
hash("eve")   % 8 = 6  → bucket[6]

Operations (average, assuming good hash function):
  insert: O(1) amortized
  lookup: O(1) average, O(n) worst (all keys hash to same bucket)
  delete: O(1) average

Open addressing (probing) alternative:
  All entries stored IN the array (no chains)
  Collision → probe to next slot (linear, quadratic, or double hash)
  Better cache locality, but more complex deletion
```

```c
#define HASH_SIZE 64

typedef struct Entry {
    char *key;
    int   value;
    struct Entry *next;
} Entry;

typedef struct {
    Entry *buckets[HASH_SIZE];
} HashMap;

uint32_t hash_str(const char *key) {
    // djb2 hash function
    uint32_t h = 5381;
    int c;
    while ((c = *key++)) {
        h = ((h << 5) + h) + c;  // h * 33 + c
    }
    return h % HASH_SIZE;
}

void hmap_set(HashMap *m, const char *key, int val) {
    uint32_t idx = hash_str(key);
    Entry *e = m->buckets[idx];
    
    // Update existing:
    while (e) {
        if (strcmp(e->key, key) == 0) {
            e->value = val;
            return;
        }
        e = e->next;
    }
    
    // Insert new:
    Entry *new_entry = malloc(sizeof(Entry));
    new_entry->key   = strdup(key);
    new_entry->value = val;
    new_entry->next  = m->buckets[idx];
    m->buckets[idx]  = new_entry;
}

int hmap_get(HashMap *m, const char *key, bool *found) {
    uint32_t idx = hash_str(key);
    Entry *e = m->buckets[idx];
    while (e) {
        if (strcmp(e->key, key) == 0) {
            *found = true;
            return e->value;
        }
        e = e->next;
    }
    *found = false;
    return 0;
}
```

## 22.6 Binary Search Tree (BST)

```
BST Property: left subtree < node < right subtree

Insert: 50, 30, 70, 20, 40, 60, 80

           50
          /  \
        30    70
       / \   / \
      20 40 60  80

Search for 40:
  50 → 40 < 50 → go left
  30 → 40 > 30 → go right
  40 → found!

In-order traversal (left, root, right):
  20, 30, 40, 50, 60, 70, 80  ← always sorted!

Operations (balanced tree):
  search:  O(log n)
  insert:  O(log n)
  delete:  O(log n)
  min/max: O(log n)

Operations (degenerate/sorted input tree):
  All O(n) — degenerates to a linked list:
  Insert 1,2,3,4,5:
  1
   \
    2
     \
      3
       \
        4
         \
          5
```

```c
typedef struct BST {
    int val;
    struct BST *left, *right;
} BST;

BST *bst_insert(BST *root, int val) {
    if (root == NULL) {
        BST *n = malloc(sizeof(BST));
        n->val = val;
        n->left = n->right = NULL;
        return n;
    }
    if (val < root->val) root->left  = bst_insert(root->left,  val);
    else if (val > root->val) root->right = bst_insert(root->right, val);
    return root;
}

bool bst_search(BST *root, int val) {
    if (!root) return false;
    if (val == root->val) return true;
    if (val < root->val)  return bst_search(root->left, val);
    return bst_search(root->right, val);
}

void inorder(BST *root) {
    if (!root) return;
    inorder(root->left);
    printf("%d ", root->val);
    inorder(root->right);
}
```

## 22.7 Heap (Priority Queue)

```
Max-heap: parent >= children

Array representation of heap (index math):
parent(i) = (i-1)/2
left(i)   = 2*i + 1
right(i)  = 2*i + 2

Heap of [90, 80, 70, 60, 50, 30, 20]:
Array: [90, 80, 70, 60, 50, 30, 20]
Index:  [0] [1] [2] [3] [4] [5] [6]

Tree view:
           90 [0]
          /       \
       80 [1]    70 [2]
       /  \      /  \
    60[3] 50[4] 30[5] 20[6]

Insert 85:
1. Append to end:    [90,80,70,60,50,30,20,85]  index=7
2. Bubble up (sift up):
   parent(7) = 3 → arr[3]=60 < 85 → swap
   [90,80,70,85,50,30,20,60]
   parent(3) = 1 → arr[1]=80 < 85 → swap
   [90,85,70,80,50,30,20,60]
   parent(1) = 0 → arr[0]=90 >= 85 → stop
   Final: [90,85,70,80,50,30,20,60]

Extract max:
1. Take root (90)
2. Move last element to root: [60,85,70,80,50,30,20]
3. Sift down (bubble down):
   children of 0: left=85, right=70. Max=85 > 60 → swap
   [85,60,70,80,50,30,20]
   children of 1(60): left=80, right=50. Max=80 > 60 → swap
   [85,80,70,60,50,30,20]
   children of 3(60): none. Done.
```

```c
typedef struct {
    int *data;
    int  size, cap;
} MaxHeap;

void heap_push(MaxHeap *h, int val) {
    // Grow if needed
    if (h->size == h->cap) {
        h->cap = h->cap ? h->cap * 2 : 4;
        h->data = realloc(h->data, h->cap * sizeof(int));
    }
    h->data[h->size++] = val;
    // Sift up
    int i = h->size - 1;
    while (i > 0) {
        int parent = (i - 1) / 2;
        if (h->data[parent] < h->data[i]) {
            int tmp = h->data[parent];
            h->data[parent] = h->data[i];
            h->data[i] = tmp;
            i = parent;
        } else break;
    }
}

int heap_pop(MaxHeap *h) {
    int max = h->data[0];
    h->data[0] = h->data[--h->size];
    // Sift down
    int i = 0;
    while (true) {
        int left = 2*i+1, right = 2*i+2, largest = i;
        if (left  < h->size && h->data[left]  > h->data[largest]) largest = left;
        if (right < h->size && h->data[right] > h->data[largest]) largest = right;
        if (largest == i) break;
        int tmp = h->data[i];
        h->data[i] = h->data[largest];
        h->data[largest] = tmp;
        i = largest;
    }
    return max;
}
```

---

# 23. Algorithm Internals

## 23.1 Sorting Algorithms

### Quicksort

```
Average: O(n log n)    Worst: O(n²)  Space: O(log n) stack
In-place, cache-friendly, fast in practice

Partition around pivot (last element):

Array: [3, 6, 8, 10, 1, 2, 1]  pivot = 1
                               i = -1  j scans 0 to 5

j=0: 3 > 1, no swap. i=-1
j=1: 6 > 1, no swap. i=-1
j=2: 8 > 1, no swap. i=-1
j=3: 10> 1, no swap. i=-1
j=4: 1 ≤ 1, swap arr[++i]=arr[0] with arr[4]:
     [1, 6, 8, 10, 3, 2, 1]   i=0
j=5: 2 > 1, no swap. i=0

Place pivot at i+1=1:
     [1, 1, 8, 10, 3, 2, 6]

Pivot 1 is at index 1. All elements left < 1, all right > 1.
Recursively sort [1..0] (empty) and [8,10,3,2,6]

Recursion tree (balanced case):
          [1..n]
         /      \
     [1..n/2]  [n/2..n]
      /   \     /    \
    ...   ...  ...   ...

Depth = log n → total work = n * log n
```

```c
void swap(int *a, int *b) { int t = *a; *a = *b; *b = t; }

int partition(int *arr, int lo, int hi) {
    int pivot = arr[hi];
    int i = lo - 1;
    for (int j = lo; j < hi; j++) {
        if (arr[j] <= pivot) {
            swap(&arr[++i], &arr[j]);
        }
    }
    swap(&arr[i+1], &arr[hi]);
    return i + 1;
}

void quicksort(int *arr, int lo, int hi) {
    if (lo < hi) {
        int p = partition(arr, lo, hi);
        quicksort(arr, lo, p - 1);
        quicksort(arr, p + 1, hi);
    }
}
```

Go quicksort:

```go
func quicksort(arr []int) {
    if len(arr) <= 1 {
        return
    }
    pivot := arr[len(arr)-1]
    i := 0
    for j := 0; j < len(arr)-1; j++ {
        if arr[j] <= pivot {
            arr[i], arr[j] = arr[j], arr[i]
            i++
        }
    }
    arr[i], arr[len(arr)-1] = arr[len(arr)-1], arr[i]
    quicksort(arr[:i])
    quicksort(arr[i+1:])
}
```

### Merge Sort

```
Time: O(n log n) always    Space: O(n)    Stable sort

Divide and conquer:
[38, 27, 43, 3, 9, 82, 10]

Split:
[38, 27, 43, 3]    [9, 82, 10]

Split again:
[38, 27]  [43, 3]    [9, 82]  [10]

Split again:
[38] [27]  [43] [3]  [9] [82]  [10]

Merge pairs:
[27, 38]  [3, 43]    [9, 82]  [10]

Merge:
[3, 27, 38, 43]    [9, 10, 82]

Merge:
[3, 9, 10, 27, 38, 43, 82]

Merge step example — merging [3,27,38,43] and [9,10,82]:
Left ptr: L=0 (3)     Right ptr: R=0 (9)
Compare 3 vs 9:  3 < 9  → output 3,   L++
Compare 27 vs 9: 27 > 9 → output 9,   R++
Compare 27 vs 10:27 > 10→ output 10,  R++
Compare 27 vs 82:27 < 82→ output 27,  L++
Compare 38 vs 82:38 < 82→ output 38,  L++
Compare 43 vs 82:43 < 82→ output 43,  L++
Left exhausted → copy remaining [82]
Result: [3,9,10,27,38,43,82]
```

```c
void merge(int *arr, int lo, int mid, int hi) {
    int n1 = mid - lo + 1, n2 = hi - mid;
    int *L = malloc(n1 * sizeof(int));
    int *R = malloc(n2 * sizeof(int));
    
    for (int i = 0; i < n1; i++) L[i] = arr[lo + i];
    for (int j = 0; j < n2; j++) R[j] = arr[mid + 1 + j];
    
    int i = 0, j = 0, k = lo;
    while (i < n1 && j < n2) {
        arr[k++] = L[i] <= R[j] ? L[i++] : R[j++];
    }
    while (i < n1) arr[k++] = L[i++];
    while (j < n2) arr[k++] = R[j++];
    
    free(L);
    free(R);
}

void mergesort(int *arr, int lo, int hi) {
    if (lo < hi) {
        int mid = lo + (hi - lo) / 2;
        mergesort(arr, lo, mid);
        mergesort(arr, mid + 1, hi);
        merge(arr, lo, mid, hi);
    }
}
```

### Heapsort

```
Time: O(n log n) always    Space: O(1)    Not stable    In-place

Phase 1: Heapify — build max-heap in O(n)
  Start from last non-leaf and sift down up to root
  last non-leaf = n/2 - 1

Phase 2: Extract — repeatedly extract max into sorted position
  Swap root (max) with last element
  Reduce heap size by 1
  Sift down root
  Repeat

Heapify [3,1,4,1,5,9,2,6]:
Last non-leaf = 7/2-1 = 2 → sift down index 2 (value 4):
children: 9,2 → 9 > 4 → swap → [3,1,9,1,5,4,2,6]

Sift down index 1 (value 1):
children: 1,5 → 5 > 1 → swap → [3,5,9,1,1,4,2,6]
children of 4: 6 only → 6 > 1 → swap → [3,5,9,1,6,4,2,1]
wait, let me redo — children of old index 1 (now 5):
children are indices 3(1) and 4(6): 6 > 5 → swap
[3,6,9,1,5,4,2,1]... (continuing)

After full heapify: [9,6,4,1,5,3,2,1]

Extract phase:
Swap [0] and [7]: [1,6,4,1,5,3,2 | 9]   heap size=7
Sift down 1: [6,5,4,1,1,3,2 | 9]
Swap [0] and [6]: [2,5,4,1,1,3 | 6,9]   heap size=6
...continue until sorted
```

## 23.2 Binary Search

```
Precondition: sorted array

Array: [2, 5, 8, 12, 16, 23, 38, 56, 72, 91]
Index:  0  1  2   3   4   5   6   7   8   9

Search for 23:
lo=0, hi=9, mid=4: arr[4]=16 < 23 → lo=5
lo=5, hi=9, mid=7: arr[7]=56 > 23 → hi=6
lo=5, hi=6, mid=5: arr[5]=23 = 23 → found at 5!

Search for 50:
lo=0, hi=9, mid=4: arr[4]=16 < 50 → lo=5
lo=5, hi=9, mid=7: arr[7]=56 > 50 → hi=6
lo=5, hi=6, mid=5: arr[5]=23 < 50 → lo=6
lo=6, hi=6, mid=6: arr[6]=38 < 50 → lo=7
lo=7 > hi=6 → not found

CRITICAL: Always use mid = lo + (hi-lo)/2
NOT: mid = (lo+hi)/2
Because (lo+hi) can overflow when lo and hi are large ints!
```

```rust
fn binary_search(arr: &[i32], target: i32) -> Option<usize> {
    let (mut lo, mut hi) = (0usize, arr.len());
    while lo < hi {
        let mid = lo + (hi - lo) / 2;
        match arr[mid].cmp(&target) {
            std::cmp::Ordering::Equal   => return Some(mid),
            std::cmp::Ordering::Less    => lo = mid + 1,
            std::cmp::Ordering::Greater => hi = mid,
        }
    }
    None
}
```

## 23.3 Graph Traversal

```
BFS — Breadth-First Search (uses Queue, finds shortest path in unweighted graph):

Graph:
    1 - 2 - 5
    |   |
    3 - 4

Starting from 1:
Queue: [1]          Visited: {1}
Dequeue 1, enqueue neighbors 2,3:
Queue: [2,3]        Visited: {1,2,3}
Dequeue 2, enqueue neighbors 4,5:
Queue: [3,4,5]      Visited: {1,2,3,4,5}
Dequeue 3, enqueue neighbors (4 visited):
Queue: [4,5]        Visited: {1,2,3,4,5}
Dequeue 4: no unvisited neighbors
Queue: [5]
Dequeue 5: done.

BFS order: 1, 2, 3, 4, 5
Level 0: {1}
Level 1: {2, 3}   ← distance 1 from 1
Level 2: {4, 5}   ← distance 2 from 1

DFS — Depth-First Search (uses Stack or recursion, explores fully before backtracking):

Stack: [1]          Visited: {}
Pop 1, visit 1, push neighbors 2,3:
Stack: [2,3]        Visited: {1}
Pop 3, visit 3, push unvisited neighbors of 3: 4
Stack: [2,4]        Visited: {1,3}
Pop 4, visit 4, push unvisited: 2
Stack: [2,2]        Visited: {1,3,4}
Pop 2 (already seen if we check), visit 2, push 5
Stack: [2,5]        Visited: {1,2,3,4}
Pop 5, visit 5.
DFS order (one possible): 1, 3, 4, 2, 5
```

```go
package main

import "fmt"

type Graph map[int][]int

func bfs(g Graph, start int) []int {
    visited := make(map[int]bool)
    queue := []int{start}
    order := []int{}
    
    visited[start] = true
    
    for len(queue) > 0 {
        node := queue[0]
        queue = queue[1:]
        order = append(order, node)
        
        for _, neighbor := range g[node] {
            if !visited[neighbor] {
                visited[neighbor] = true
                queue = append(queue, neighbor)
            }
        }
    }
    return order
}

func dfs(g Graph, node int, visited map[int]bool, order *[]int) {
    if visited[node] {
        return
    }
    visited[node] = true
    *order = append(*order, node)
    for _, neighbor := range g[node] {
        dfs(g, neighbor, visited, order)
    }
}
```

---

# 24. OS Concepts

## Virtual Memory and Page Tables

Every process has its own **virtual address space**. Virtual addresses are translated to physical addresses by the MMU (Memory Management Unit) using page tables.

```
Process A's virtual memory:               Physical Memory:
+------------------+                     +------------------+
| Virtual page 0   | ──────────────────▶ | Physical frame 7 |
| Virtual page 1   | ──────────────────▶ | Physical frame 2 |
| Virtual page 2   | ──── page fault───▶ | (not in RAM yet) |
| ...              |                     | ...              |
+------------------+                     +------------------+

Process B's virtual memory:
+------------------+
| Virtual page 0   | ──────────────────▶ | Physical frame 9 |
| ...              |
+------------------+

Processes have ISOLATED address spaces:
  - Process A's address 0x1000 maps to different physical memory than
    Process B's address 0x1000
  - A cannot access B's physical memory (protected by MMU)
  - Only kernel can bypass this
```

Page size is typically 4KB on x86. Virtual address is split:

```
64-bit virtual address on Linux (4-level page table):
Bits: 63..48   47..39   38..30   29..21   20..12   11..0
       unused  PGD idx  PUD idx  PMD idx  PTE idx  page offset
       (sign extend)

Translation:
  CR3 register → PGD (Page Global Directory)
  PGD[47..39] → PUD entry → PUD physical address
  PUD[38..30] → PMD entry → PMD physical address
  PMD[29..21] → PTE entry → PTE physical address
  PTE[20..12] → page frame → physical address
  page frame + offset (11..0) = physical byte address

TLB (Translation Lookaside Buffer) = cache for recent translations
  TLB hit:  1 cycle
  TLB miss: 4 memory accesses (walk 4-level table)
```

## Page Fault

When you access a virtual address that's not currently mapped to physical memory:

```
1. CPU access: load from virtual address 0x7fff1234
2. MMU checks TLB → miss
3. MMU walks page table → page not present (bit 0 of PTE = 0)
4. CPU raises Page Fault exception
5. OS Page Fault Handler runs:
   a. Is this address valid? (check process VMAs — virtual memory areas)
   b. If yes: allocate physical frame, map it, populate page table
   c. Set PTE as present, update TLB
   d. Return to user — instruction re-executes (succeeds now)
   e. If no: send SIGSEGV to process → segfault

Demand paging: OS doesn't load all pages at program start
  Only loads pages when first accessed (lazy loading)
  Greatly reduces startup time and memory usage for large programs
```

## mmap Syscall

```c
#include <sys/mman.h>

// Map a file into memory:
void *mmap(void *addr,    // hint for placement (usually NULL)
           size_t length, // how many bytes to map
           int prot,      // PROT_READ | PROT_WRITE | PROT_EXEC
           int flags,     // MAP_SHARED | MAP_PRIVATE | MAP_ANONYMOUS
           int fd,        // file descriptor (-1 for anonymous)
           off_t offset); // offset in file

// Example — file-backed mapping:
int fd = open("data.bin", O_RDONLY);
void *data = mmap(NULL, file_size, PROT_READ, MAP_SHARED, fd, 0);
// Now access data[0..file_size-1] — OS loads pages on demand

// Example — anonymous mapping (like malloc for large allocs):
void *mem = mmap(NULL, 10*1024*1024, PROT_READ|PROT_WRITE,
                 MAP_PRIVATE|MAP_ANONYMOUS, -1, 0);

munmap(data, file_size);  // unmap when done
```

## Syscall Overhead

```
User program calling write():
  1. User code: call write(fd, buf, len)
  2. C library: set syscall number in rax (e.g., 1 for write)
  3. C library: syscall instruction (or int 0x80 on 32-bit)
  4. CPU: switch to kernel mode (ring 0), save registers
  5. Kernel: validate fd, buf, len
  6. Kernel: copy from user buffer to kernel buffer (or DMA)
  7. Kernel: schedule I/O
  8. Kernel: restore registers, switch to user mode (ring 3)
  9. Return to user code

Cost: ~100-1000 ns (much more than a function call ~1ns)

This is why buffering (bufio in Go, stdio in C) matters:
  Without buffering: 1 syscall per byte → catastrophically slow
  With 4KB buffer:   1 syscall per 4096 bytes → 4096x fewer syscalls
```

## Epoll — Non-Blocking I/O Multiplexing

This is how Go's network I/O works internally:

```
Traditional blocking I/O (thread per connection):
Thread 1: read(conn1) → blocks waiting for data
Thread 2: read(conn2) → blocks waiting for data
...
Thread N: read(connN) → blocks waiting for data
Problem: N connections → N threads → too much memory and context switching

epoll (Linux): event-driven, one or few threads:
1. epoll_create() → create epoll instance
2. epoll_ctl(ADD, conn1)  → register fd1 for events
   epoll_ctl(ADD, conn2)  → register fd2 for events
   ...register thousands of fds...
3. epoll_wait() → BLOCKS until any fd is ready
   Returns list of ready fds
4. Handle ready fds, then call epoll_wait() again

Go runtime uses epoll internally:
  netpoller goroutine monitors epoll
  When fd is ready, wakes up the goroutine that was blocked on it
  That goroutine runs its Read/Write — no blocking threads
  Thousands of goroutines, few OS threads
```

---

# Appendix: Output Prediction Questions

## Classic C Output Puzzles

```c
// 1. What does this print?
int a = 5, b = 3;
printf("%d %d\n", a++, ++b);
// Answer: 5 4
// a++ returns 5, then increments a to 6
// ++b increments b to 4, then returns 4

// 2. sizeof trap
char arr[] = "hello";
char *ptr  = "hello";
printf("%zu %zu\n", sizeof(arr), sizeof(ptr));
// Answer: 6 8
// arr is the array itself: 6 bytes (h,e,l,l,o,\0)
// ptr is a pointer: 8 bytes (on 64-bit)

// 3. Integer promotion
char c = 255;
printf("%d\n", c);
// Answer: -1 (if char is signed) or 255 (if char is unsigned)
// char signedness is implementation-defined!

// 4. printf format mismatch
int x = -1;
printf("%u\n", x);
// Answer: 4294967295
// -1 in signed int is 0xFFFFFFFF
// Interpreted as unsigned = 4294967295

// 5. Array out of bounds
int arr[5] = {0,1,2,3,4};
printf("%d\n", arr[5]);
// Undefined behavior — may print garbage, crash, or print 0
// No bounds checking in C

// 6. Pointer comparison trap
char *a = "hello";
char *b = "hello";
printf("%d\n", a == b);
// Either 0 or 1 — implementation defined!
// Compiler may share one string literal (same address) or not
// Use strcmp() to compare string content

// 7. Static variable
void count() {
    static int n = 0;
    printf("%d\n", ++n);
}
count(); count(); count();
// Prints: 1 2 3
// static local variables initialized once, persist across calls

// 8. Sequence point violation
int i = 1;
int x = i++ + i++;  // undefined behavior
// Could print 3, 2, or anything
// Evaluating both i++ in same expression without sequence point
```

---

cat > /home/claude/guide_part1.md << 'ENDOFPART1'
# The Complete Systems Programming & SDE Interview Guide
## C · Go · Rust — Memory, Data Structures, Algorithms, OS, Concurrency

> **How to use this guide:** Read linearly for mental model building. Every concept is explained from first principles, then shown with ASCII internals, then implemented in C, Go, and Rust. This makes concepts stick structurally — not just syntactically.

---

# PART 1: MEMORY ARCHITECTURE — THE ABSOLUTE FOUNDATION

Everything in systems programming sits on top of memory. If your mental model of memory is wrong, every other concept will be confusing.

---

## 1.1 Process Memory Layout

When the OS loads your program, it lays out memory in well-defined segments:

```
High addresses (0xFFFFFFFF on 32-bit, higher on 64-bit)
+---------------------------+
|        KERNEL SPACE       |  ← OS lives here, inaccessible to user code
+---------------------------+  0xC0000000 on 32-bit Linux
|          STACK            |  ← grows DOWNWARD ↓
|      [frame n]            |
|      [frame n-1]          |
|      [frame n-2]          |
|           ...             |
|     stack pointer (SP) →  |
|      (unmapped gap)       |  ← stack overflow happens here
|     heap pointer (BP) →   |
|           ...             |
|         HEAP              |  ← grows UPWARD ↑
|   [allocated chunks]      |
|   [free chunks]           |
+---------------------------+
|          BSS              |  ← uninitialized global/static vars (zeroed by OS)
+---------------------------+
|          DATA             |  ← initialized global/static vars
+---------------------------+
|          TEXT             |  ← program instructions (read-only, executable)
+---------------------------+
|         RODATA            |  ← read-only data: string literals, const arrays
+---------------------------+
Low addresses (0x00000000)
```

**Why does the stack grow downward?**

Historical reason: early systems had a single address space. Heap grew up from low memory, stack grew down from high memory. They grew toward each other, maximizing available space before collision. Modern 64-bit systems have enough virtual address space (128 TiB) that this matters less, but the convention remains.

---

## 1.2 The Stack — Deep Internals

The stack is a LIFO (Last In, First Out) region of memory managed automatically by the CPU using two registers:

- **RSP (Stack Pointer)**: points to the top (current position) of the stack
- **RBP (Base Pointer / Frame Pointer)**: points to the base of the current stack frame

### Stack Frame Anatomy

Every function call creates a "stack frame" (also called "activation record"):

```
          High addresses
          +------------------+
          | prev frame's RBP |  ← saved caller's frame pointer
          +------------------+  ← current RBP points here
          | local var a      |
          +------------------+
          | local var b      |
          +------------------+
          | local var c      |
          +------------------+  ← current RSP points here
          |   (free stack)   |
          Low addresses
```

When you call `void foo(int x, int y)`:

```
BEFORE CALL (caller's perspective):
+----------------------+
| caller's locals      |
+----------------------+
| ...                  |
+----------------------+  ← RSP

CALLING: push arguments (or pass in registers on x86-64)
+----------------------+
| argument y           |
+----------------------+
| argument x           |
+----------------------+  ← RSP

CALL instruction executes: pushes return address
+----------------------+
| return address       |  ← where to jump back after foo() returns
+----------------------+  ← RSP

INSIDE foo(): prologue saves RBP, sets new frame
+----------------------+
| saved caller's RBP   |
+----------------------+  ← RBP (new frame base)
| foo's local vars     |
+----------------------+
| foo's local vars     |
+----------------------+  ← RSP

AFTER foo() returns: epilogue restores RBP, pops return address
+----------------------+
| caller's locals      |
+----------------------+
| ...                  |
+----------------------+  ← RSP (back to before call)
```

### The Dangling Pointer Problem Explained With Memory

```c
char *createMessage() {
    char buff[500];           // buff lives at RSP - 500
    strcpy(buff, "hello");
    return buff;              // returns the ADDRESS of buff
}                             // ← stack frame destroyed here!

// buff was at, say, 0x7fff5abc1000
// after return, that memory is "free" for next function call
```

```
DURING createMessage():
Stack:
+------------------+
| [return addr]    |  0x7fff5abc11f8
+------------------+
| [saved RBP]      |  0x7fff5abc11f0
+------------------+
| buff[499]        |  0x7fff5abc11ef
| ...              |
| buff[0] = 'h'    |  0x7fff5abc1000  ← ptr = 0x7fff5abc1000
+------------------+
RSP → here

AFTER createMessage() returns:
Stack:
+------------------+
| [return addr]    |  ← RSP, next function call can use this area
+------------------+
| [saved RBP]      |  ← these bytes still EXIST but are LOGICALLY DEAD
+------------------+
| buff[499]        |  ← still contains 'h','e','l','l','o','\0' for now
| ...              |  ← BUT could be overwritten at any moment
| buff[0]          |
+------------------+

msg = 0x7fff5abc1000  ← DANGLING POINTER — points to dead memory
```

This is why `printf(msg)` *sometimes* works — the bytes haven't been overwritten yet. But after ANY other function call, those bytes belong to the new function's frame.

---

## 1.3 The Heap — Deep Internals

The heap is a region of memory managed explicitly by the programmer (in C) or by a garbage collector (in Go, Java) or by the ownership system (in Rust).

### How malloc() Works Internally

`malloc()` maintains a **free list** — a linked list of free memory chunks:

```
Heap memory region:
+--------+--------+--------+--------+--------+
| chunk1 | chunk2 | chunk3 | chunk4 | chunk5 |
+--------+--------+--------+--------+--------+

Each chunk has a header:
+-------+------+----------+----------+
| size  | used | prev_ptr | next_ptr |
+-------+------+----------+----------+
   8B      1B      8B          8B

free list (only free chunks):
NULL ← chunk1 ↔ chunk3 ↔ chunk5 → NULL
```

When you call `malloc(n)`:
1. Walk the free list looking for a chunk of size ≥ n
2. If found: mark it used, optionally split excess, return pointer to payload
3. If not found: ask the OS for more memory via `sbrk()` or `mmap()`

When you call `free(ptr)`:
1. Mark the chunk as free
2. Coalesce adjacent free chunks (merge to reduce fragmentation)
3. Add to free list

### malloc strategies

| Strategy | Description | Pros | Cons |
|----------|-------------|------|------|
| First Fit | Use first chunk that fits | Fast | Fragmentation |
| Best Fit | Use smallest chunk that fits | Less waste | Slow scan |
| Worst Fit | Use largest chunk | Large remainder usable | Destroys large chunks |
| Buddy System | Split blocks in powers of 2 | Fast coalescing | ~50% waste |
| Slab Allocator | Pools of same-size objects | Very fast for fixed sizes | Complex |

### Heap Fragmentation

```
Initial:
[  free 100B  ][  free 100B  ][  free 100B  ]

After many alloc/free cycles:
[used 10B][free 5B][used 10B][free 3B][used 10B][free 72B]

malloc(50) FAILS — no contiguous 50B block, even though 80B is free!
```

This is **external fragmentation**. It's why long-running servers eventually need restart or compaction.

---

## 1.4 Static Storage (.data and .bss)

```c
int global_initialized = 42;   // → .data segment
int global_uninitialized;      // → .bss segment (zeroed by OS loader)

void foo() {
    static int x = 10;         // → .data segment (persists across calls)
    static int y;              // → .bss segment
}
```

```
.data:
+---+---+---+---+
| 42| 10|   |   |
+---+---+---+---+
 ^   ^
 |   |
 |   static x
 global_initialized

.bss: (no actual storage in binary, just size info — OS zeros it)
+---+---+
| 0 | 0 |
+---+---+
 ^   ^
 |   static y
 global_uninitialized
```

---

## 1.5 .rodata — Read-Only Data

String literals live here:

```c
char *s = "hello";    // s is a pointer to .rodata
char arr[] = "hello"; // arr is a COPY on the stack
```

```
.rodata section:
Address 0x400600: h e l l o \0

Stack:
s: [0x400600] → points to .rodata
arr: [h][e][l][l][o][\0] → independent copy
```

**Attempting `s[0] = 'H'` is undefined behavior** — .rodata is mapped read-only. On Linux it will SIGSEGV. This is why `char *s = "hello"` and `char s[] = "hello"` behave differently.

---

## 1.6 Memory Access Costs — Cache Hierarchy

Understanding performance requires knowing this:

```
CPU Registers   ← 0 cycles (already in CPU)
L1 Cache        ← ~4 cycles   (32-64 KB, on-chip)
L2 Cache        ← ~12 cycles  (256 KB - 1 MB, on-chip)
L3 Cache        ← ~40 cycles  (4-32 MB, shared on-chip)
RAM             ← ~200 cycles (DRAM, ~100 ns)
SSD             ← ~100,000 cycles
HDD             ← ~10,000,000 cycles
```

Cache lines are 64 bytes. When you access `arr[0]`, the CPU loads bytes 0-63 into cache. Accessing `arr[1]` through `arr[15]` (ints) is nearly free — already in L1.

This is why **sequential memory access is faster than random access** — it exploits spatial locality. Linked lists are slower than arrays for iteration not because traversal is complex, but because each node may be in a different cache line (cache miss).

---

## 1.7 Memory Alignment

The CPU can read values most efficiently when they're aligned to their size:

```
struct BadLayout {   struct GoodLayout {
    char a;              int  x;       // offset 0, size 4
    int  x;              int  y;       // offset 4, size 4
    char b;              char a;       // offset 8, size 1
};                       char b;       // offset 9, size 1
                         char pad[2];  // offset 10, implicit padding
                     };

BadLayout in memory:   GoodLayout in memory:
[a][pad][pad][pad]     [x x x x]
[x  x   x   x  ]      [y y y y]
[b][pad][pad][pad]     [a b . .]
total: 12 bytes        total: 12 bytes (but predictable)
```

Rules for padding:
1. Each member is aligned to its own size (or smaller, platform-dependent)
2. `int` (4B) must be at address divisible by 4
3. `double` (8B) must be at address divisible by 8
4. Struct total size is padded to largest member's alignment

```c
// EXAMPLE: why sizeof(struct) != sum of member sizes
struct Example {
    char  a;    // offset 0, size 1
    // 3 bytes padding
    int   b;    // offset 4, size 4
    char  c;    // offset 8, size 1
    // 7 bytes padding
    double d;   // offset 16, size 8
};
// sizeof = 24, not 14!

// TRICK: pack members largest-first to minimize padding
struct Packed {
    double d;   // offset 0, size 8
    int    b;   // offset 8, size 4
    char   a;   // offset 12, size 1
    char   c;   // offset 13, size 1
    // 2 bytes padding
};
// sizeof = 16!
```

---

# PART 2: C LANGUAGE — COMPLETE DEEP DIVE

---

## 2.1 Pointers — Everything

A pointer is a variable that holds a memory address. That's all. The complexity comes from what you do with that address.

```
int x = 42;

Memory:
Address  Value
0x1000:  [42] [00] [00] [00]   ← x (4 bytes, little-endian)

int *p = &x;

Address  Value
0x2000:  [00] [10] [00] [00]   ← p (8 bytes on 64-bit, holds 0x1000)
         [00] [00] [00] [00]
```

### Pointer Taxonomy

| Type | Description | Example |
|------|-------------|---------|
| Valid pointer | Points to live memory | `int *p = &x;` |
| Null pointer | Holds address 0, never valid to dereference | `int *p = NULL;` |
| Wild pointer | Uninitialized, holds garbage address | `int *p;` |
| Dangling pointer | Was valid, memory since freed/ended | Return of local address |
| Void pointer | Generic pointer, needs cast to use | `void *p = malloc(n);` |
| Function pointer | Points to executable code | `int (*fp)(int, int)` |

### Pointer Arithmetic

When you do `p + 1`, C adds `sizeof(*p)` bytes, NOT 1 byte:

```c
int arr[] = {10, 20, 30, 40};
int *p = arr;  // p = address of arr[0]

Memory:
0x1000: [10][00][00][00]  ← arr[0], p points here
0x1004: [20][00][00][00]  ← arr[1], (p+1) points here
0x1008: [30][00][00][00]  ← arr[2], (p+2) points here
0x100C: [40][00][00][00]  ← arr[3], (p+3) points here

p + 1 = 0x1004 (not 0x1001!)
*(p + 2) = 30
p[3] == *(p + 3) == 40   // [] is SYNTAX SUGAR for pointer arithmetic
```

### Arrays vs Pointers — The Critical Difference

```c
int arr[5] = {1,2,3,4,5};
int *ptr = arr;

sizeof(arr) = 20  // 5 * 4 — compiler knows the full size
sizeof(ptr) = 8   // just a pointer on 64-bit

// arr decays to &arr[0] in most expressions, but NOT in:
// sizeof(arr) — gives full array size
// &arr        — gives pointer to array (type: int (*)[5])
// _Alignof(arr)
```

```
arr is like a LABEL for a fixed memory region:
+----+----+----+----+----+
| 1  | 2  | 3  | 4  | 5  |
+----+----+----+----+----+
arr (the label, not moveable)

ptr is a VARIABLE holding an address:
ptr: [0x1000] → same memory above
ptr can be reassigned: ptr = &arr[2]; // now points to middle
arr cannot: arr = &arr[2]; // COMPILE ERROR
```

### Const Pointer Combinations

```c
int x = 10;
int y = 20;

int *p = &x;           // pointer to int: can change both ptr and value
const int *p = &x;     // pointer to const int: can't change *p, can change p
int *const p = &x;     // const pointer to int: can't change p, can change *p
const int *const p = &x; // can't change anything

Memory model:
        +-----------+        +------+
p →     | address   |  →     | int  |
        +-----------+        +------+
"const int *p": the int is const (read-only through p)
"int *const p": the address itself is const (p is fixed)
```

### Function Pointers

```c
// Declare a function:
int add(int a, int b) { return a + b; }
int mul(int a, int b) { return a * b; }

// Declare a function pointer:
int (*operation)(int, int);

// Assign:
operation = add;
operation = &add;  // same thing — function name decays to pointer

// Call:
int result = operation(3, 4);  // 7
result = (*operation)(3, 4);   // also 7

// As parameter (callback):
void apply(int *arr, int n, int (*transform)(int)) {
    for (int i = 0; i < n; i++)
        arr[i] = transform(arr[i]);
}
```

Function pointer layout:

```
TEXT (code) segment:
0x4011a0: [add's machine code]
0x4011b0: [mul's machine code]

Stack:
operation: [0x4011a0]  → points into TEXT segment
operation = mul;
operation: [0x4011b0]  → points to mul's code
```

---

## 2.2 Strings in C — The Complete Picture

Strings in C are null-terminated arrays of `char`. There is no string type.

```c
// Three different things that look similar:

// 1. String literal — lives in .rodata, read-only
char *a = "hello";

// 2. Array initialized from literal — COPY on stack
char b[] = "hello";

// 3. Heap string
char *c = malloc(6);
strcpy(c, "hello");
```

Memory layout:

```
.rodata:
0x400600: [h][e][l][l][o][\0]

Stack:
a: [0x400600]              ← just a pointer to rodata

b: [h][e][l][l][o][\0]    ← full copy, 6 bytes on stack
   ^5 chars + 1 null terminator

Heap:
c: → [h][e][l][l][o][\0]  ← 6 bytes allocated
```

### Why strlen() != sizeof()

```c
char s[] = "hello";
// sizeof(s) = 6 (includes \0)
// strlen(s) = 5 (stops before \0)

// strlen walks memory byte by byte until \0:
size_t my_strlen(const char *s) {
    size_t n = 0;
    while (*s++ != '\0') n++;  // s++ advances, dereference checks for null
    return n;
}

Walking:
s[0]='h' != '\0' → n=1
s[1]='e' != '\0' → n=2
s[2]='l' != '\0' → n=3
s[3]='l' != '\0' → n=4
s[4]='o' != '\0' → n=5
s[5]='\0'        → STOP, return 5
```

### String Safety — The Correct Mental Model

```c
// DANGEROUS: buffer overflow
char dest[5];
strcpy(dest, "hello world");  // writes 12 bytes into 5-byte buffer!
// overwrites adjacent stack memory — classic exploit vector

// Memory (stack):
dest:  [h][e][l][l][o] ← 5 bytes we own
       [ ][w][o][r][l] ← overwriting adjacent memory! (saved RBP?)
       [d][\0]

// SAFE: strncpy (but has its own quirk)
strncpy(dest, "hello world", 4);
dest[4] = '\0';  // MUST manually null-terminate! strncpy doesn't guarantee it
                 // Also: strncpy zero-pads to n if src < n (wasteful)

// BETTER: snprintf
snprintf(dest, sizeof(dest), "%s", src);
// Always null-terminates, respects size, handles format strings
```

---

## 2.3 Dynamic Memory — Internals of malloc/free

```c
// malloc: allocate n bytes, uninitialized, returns NULL on failure
void *malloc(size_t n);

// calloc: allocate n*size bytes, zero-initialized
void *calloc(size_t n, size_t size);

// realloc: resize an allocation (may move it!)
void *realloc(void *ptr, size_t new_size);

// free: return memory to allocator
void free(void *ptr);
```

### What malloc actually allocates

```
You call: malloc(100)

glibc malloc allocates 100 + 8 bytes (or more for alignment):

+--------+---------------------------+
| header |       your 100 bytes      |
+--------+---------------------------+
   8B              ← pointer returned here

header contains: { size, flags }
This is why free(ptr) knows how much to release — the size is in the header!
```

### Memory Leak

```c
void leak() {
    char *p = malloc(100);
    // forgot free(p);
    return;  // p goes out of scope, address is LOST
             // 100 bytes forever unclaimed until process exits
}

Heap after many calls:
+------+------+------+------+
| USED | USED | USED | USED | ← all "in use" but no pointers to them
+------+------+------+------+
Total RAM consumed: N * 100 bytes (and growing)
```

### Double Free

```c
char *p = malloc(100);
free(p);
// p still holds the address, but memory is free
free(p);  // UNDEFINED BEHAVIOR — may corrupt allocator's free list

// Free list corruption:
// Normal free list: A → B → C
// After double free(A): A → A → ... (cycle!)
// Next malloc may return same address twice
// → two pointers think they own the same memory
// → data corruption / security vulnerability (heap exploit)
```

**Always set pointer to NULL after free:**

```c
free(p);
p = NULL;  // now free(p) is a no-op (free(NULL) is defined and safe)
```

### Use After Free

```c
int *arr = malloc(10 * sizeof(int));
arr[0] = 42;
free(arr);

// Memory returned to allocator, may be given to next malloc call
arr[0] = 99;  // WRITING TO FREED MEMORY
              // If another malloc got this address, you're corrupting their data

printf("%d\n", arr[0]); // READING FREED MEMORY
                        // Could print 99, or new owner's data, or crash
```

---

## 2.4 Structs, Unions, Enums

### Struct — Aggregate Type

```c
struct Point {
    float x;
    float y;
    float z;
};

Memory (12 bytes, no padding needed — all floats):
+------+------+------+
|  x   |  y   |  z   |
| 4B   | 4B   | 4B   |
+------+------+------+
0      4      8      12

struct Person {
    char name[20];
    int  age;
    char gender;  // 1B
    // 3B padding
    double salary; // 8B — must align to 8
};
// sizeof = 20 + 4 + 1 + 3(pad) + 8 = 36
```

### Self-Referential Struct — Linked List Node

```c
struct Node {
    int data;
    struct Node *next;  // pointer to same type — must use struct tag here
};

// A linked list:
Node A: [data=1][next=0x200]
Node B: [data=2][next=0x300]
Node C: [data=3][next=NULL]

A.next → B
B.next → C
C.next → NULL

ASCII visualization:
+------+-------+    +------+-------+    +------+-------+
|  1   |  *----+--->|  2   |  *----+--->|  3   | NULL  |
+------+-------+    +------+-------+    +------+-------+
```

### Union — Shared Memory

```c
union Value {
    int   i;
    float f;
    char  bytes[4];
};

// All members share the SAME memory location
// sizeof(union) = sizeof(largest member)

Memory (4 bytes total):
+----+----+----+----+
| b0 | b1 | b2 | b3 |
+----+----+----+----+
  ↑ i (int, 4 bytes)
  ↑ f (float, 4 bytes)
  ↑ bytes[0..3]

// Writing v.i = 0x41424344 then reading v.bytes:
v.bytes[0] = 0x44  (little-endian: LSB first)
v.bytes[1] = 0x43
v.bytes[2] = 0x42
v.bytes[3] = 0x41
```

**Use cases for union:**
- Type punning (inspect float's bit representation via int)
- Memory-efficient tagged variants (paired with an enum tag)
- Network protocol parsing (same bytes, different interpretations)

### Enum

```c
enum Color { RED, GREEN, BLUE };
// RED=0, GREEN=1, BLUE=2 (by default, sequential from 0)

enum Status { OK=200, NOT_FOUND=404, ERROR=500 };
// values are not sequential — that's allowed

// Enum values are compile-time int constants
// sizeof(enum Color) == sizeof(int) on most platforms
```

---

## 2.5 Type Qualifiers — const, static, extern, volatile

### `const`

```c
const int x = 5;        // x cannot be modified through this name
                         // but the memory might be modifiable via another pointer!
int *p = (int*)&x;
*p = 10;                 // undefined behavior, but might "work" if x is on stack
                         // crashes if x is in .rodata

// Function parameters:
void print(const char *s);  // "I promise not to modify what s points to"
// This is a contract, enforced by compiler, essential for API design
```

### `static`

```c
// In file scope: limits visibility to this translation unit
static int counter = 0;  // not visible to linker, won't conflict with other files

// In function scope: persists between calls
void count() {
    static int n = 0;  // initialized ONCE, lives in .data
    printf("%d\n", ++n);
}
// count() → 1, count() → 2, count() → 3 ...

// As function: same as file-scope static (C doesn't have "static method" like C++)
```

### `extern`

```c
// file1.c
int global_var = 42;  // definition + allocation

// file2.c
extern int global_var;  // declaration only — "this exists somewhere else"
// tells compiler: the linker will find this symbol
// extern is implicit for function declarations
```

### `volatile`

```c
// Tells compiler: this value can change at any time outside program control
// DO NOT cache it in a register, DO NOT reorder accesses

volatile int *io_register = (volatile int*)0xB8000; // hardware I/O port
*io_register = 1;  // must actually write to that address
*io_register = 2;  // cannot be optimized away

// Without volatile, compiler might optimize:
// *io_register = 1; *io_register = 2; → just *io_register = 2;
// (first write is "useless" from compiler's view — but HW might need it!)

// Use cases:
// - Memory-mapped hardware registers
// - Signal handlers
// - Setjmp/longjmp patterns
// - Shared memory between processes (but use proper atomics for threading)
```

---

## 2.6 Integer Types and Overflow

```
Signed types (two's complement):
  signed char:  -128 to 127         (8-bit)
  short:        -32768 to 32767     (16-bit)
  int:          -2^31 to 2^31-1     (32-bit on most platforms)
  long:         platform-dependent  (32-bit on Windows 64, 64-bit on Linux 64)
  long long:    -2^63 to 2^63-1     (64-bit, guaranteed by C99)

Unsigned types:
  unsigned char:  0 to 255
  unsigned short: 0 to 65535
  unsigned int:   0 to 2^32-1

Fixed-width (from stdint.h):
  int8_t, int16_t, int32_t, int64_t
  uint8_t, uint16_t, uint32_t, uint64_t
```

### Integer Overflow

```c
// SIGNED overflow is UNDEFINED BEHAVIOR in C:
int x = INT_MAX;  // 2147483647
x = x + 1;       // UNDEFINED — compiler can assume this never happens!
                  // With -O2, GCC may eliminate "impossible" branches

// UNSIGNED overflow is DEFINED — wraps modulo 2^n:
unsigned int u = UINT_MAX;  // 4294967295
u = u + 1;                  // DEFINED: u = 0 (wraps around)

// Two's complement visualization for 4-bit numbers:
//  0000=0, 0001=1, ..., 0111=7, 1000=-8, 1001=-7, ..., 1111=-1
//  Positive: MSB=0
//  Negative: MSB=1 (flip all bits + 1 to get magnitude)
//  -1 = 1111 (flip 0001→1110, +1→1111)
```

### Signed Integer Overflow — Why It's UB

The compiler makes optimizations ASSUMING UB never happens:

```c
// With signed overflow UB:
for (int i = 0; i <= INT_MAX; i++) { /* ... */ }
// GCC assumes i never overflows, so the loop condition is always true
// → compiler may turn this into an infinite loop!

// This is NOT a compiler bug — this is C standard compliance
```

---

## 2.7 The Preprocessor and Compilation Pipeline

### Stages of Compilation

```
Source code (foo.c)
        ↓
  [PREPROCESSOR]  cpp
        ↓
  Preprocessed source (foo.i)  — #include expanded, macros replaced
        ↓
  [COMPILER]  cc1
        ↓
  Assembly (foo.s)  — human-readable machine instructions
        ↓
  [ASSEMBLER]  as
        ↓
  Object file (foo.o)  — binary, but with unresolved symbols
        ↓
  [LINKER]  ld
        ↓
  Executable (a.out)  — fully resolved, loadable binary

gcc foo.c -E → stop after preprocessing (foo.i)
gcc foo.c -S → stop after compilation (foo.s)
gcc foo.c -c → stop after assembly (foo.o)
gcc foo.c    → full pipeline (a.out)
```

### Macros vs Functions

```c
// MACRO — text substitution, no type checking, no stack frame
#define SQUARE(x) ((x) * (x))

// Pitfall:
SQUARE(i++)   expands to ((i++) * (i++))  // i incremented TWICE — UB!
SQUARE(2+3)   → (2+3) * (2+3) = 25 ✓ (parentheses around x saved us)
SQUARE(2+3)   without parens: 2+3*2+3 = 11 ✗

// ALWAYS parenthesize macro arguments AND the whole macro:
#define MAX(a, b) ((a) > (b) ? (a) : (b))
// Still has double-evaluation: MAX(i++, j++) evaluates larger one twice!

// FUNCTION — type safe, single evaluation, but has call overhead (tiny)
static inline int square(int x) { return x * x; }  // inline hint = no overhead
```

### Header Guards

```c
// Without guards: if foo.h is included twice, struct is redefined → error
// foo.h:
#ifndef FOO_H         // if FOO_H is not defined
#define FOO_H         // define it (no value, just existence matters)

struct Foo { int x; };

#endif                // end of guarded block

// Second include: FOO_H is already defined → skips everything
```

---

## 2.8 Undefined Behavior — Complete Taxonomy

UB is a contract violation with the C standard. The compiler is allowed to assume UB never happens and make optimizations based on that. This can produce:
- Correct output (lucky)
- Garbage output
- Silently wrong output
- Security vulnerabilities
- Different behavior at different optimization levels

```
COMPLETE LIST OF COMMON UNDEFINED BEHAVIORS:

1. Dereferencing a NULL or wild pointer
   int *p = NULL; *p = 5;

2. Use-after-free
   free(p); *p = 5;

3. Double free
   free(p); free(p);

4. Buffer overflow/underflow
   int a[5]; a[10] = 1;

5. Signed integer overflow
   INT_MAX + 1

6. Left-shifting into/past sign bit
   1 << 31  (for int)

7. Returning pointer to local variable (use-after-return)
   return &local_var;

8. Uninitialized variable read
   int x; printf("%d", x);

9. Modifying string literal
   char *s = "hello"; s[0] = 'H';

10. Multiple modifications in one expression without sequence point
    i = i++

11. Mismatched printf format specifier
    printf("%d", 3.14);  // should be %f

12. Type punning via pointer cast (violates strict aliasing)
    float f = 1.0; int i = *(int*)&f;  // use union instead

13. Stack overflow (calling too deep / huge local array)
    void foo() { int arr[100000000]; }

14. Negative array index
    arr[-1] = 5;

15. Dereferencing pointer arithmetic that goes outside object
    int a[5]; int *p = a + 10; *p;
```

---

## 2.9 Bitwise Operations

Essential for systems programming — protocol parsing, bit flags, hardware control, cryptography.

```
Operation | Symbol | Effect
----------|--------|-------
AND       |   &    | bits set in BOTH operands
OR        |   |    | bits set in EITHER operand
XOR       |   ^    | bits set in EXACTLY ONE operand
NOT       |   ~    | flip all bits
Left shift|   <<   | shift bits left, fill 0 from right (* 2^n)
Right shift|  >>   | shift bits right (arithmetic or logical)
```

```
a = 0b10110100  (0xB4 = 180)
b = 0b01101101  (0x6D = 109)

a & b = 0b00100100  (AND)
a | b = 0b11111101  (OR)
a ^ b = 0b11011001  (XOR)
~a    = 0b01001011  (NOT)
a << 2 = 0b11010000  (left shift 2, = 180 * 4 = 720, but overflows 8-bit)
a >> 1 = 0b01011010  (right shift 1, = 180 / 2 = 90)
```

### Common Bit Tricks

```c
// Set bit n:
x |= (1 << n);

// Clear bit n:
x &= ~(1 << n);

// Toggle bit n:
x ^= (1 << n);

// Test if bit n is set:
if (x & (1 << n)) { ... }

// Extract lowest set bit:
int lsb = x & (-x);   // -x in two's complement flips bits and adds 1

// Round up to next power of 2:
n--;
n |= n >> 1;
n |= n >> 2;
n |= n >> 4;
n |= n >> 8;
n |= n >> 16;
n++;

// Check if power of 2:
bool isPow2 = n > 0 && (n & (n - 1)) == 0;

// Detect endianness:
uint32_t test = 0x01020304;
uint8_t *bytes = (uint8_t*)&test;
if (bytes[0] == 0x04) // little-endian (LSB first)
else if (bytes[0] == 0x01) // big-endian (MSB first)
```

### Endianness

```
Value: 0x12345678

Big-endian (network byte order):
Memory address: [0] [1] [2] [3]
Content:       [12][34][56][78]   ← MSB at lowest address

Little-endian (x86, ARM in default mode):
Memory address: [0] [1] [2] [3]
Content:       [78][56][34][12]   ← LSB at lowest address

Why it matters:
- Network protocols (TCP/IP) use big-endian
- x86/x64 CPUs use little-endian
- When reading binary files or network packets, must convert:
  uint32_t ntohl(uint32_t netlong);    // network to host long
  uint32_t htonl(uint32_t hostlong);   // host to network long
```

---

## 2.10 Output Prediction — Classic Traps

```c
// TRAP 1: printf of char as int
printf("%d", 'A');
// 'A' is an int constant with value 65
// Output: 65

// TRAP 2: uninitialized variable
int x;
printf("%d", x);
// x could be anything — garbage from stack
// Output: implementation-defined, could be 0, could be 4196416, could be anything

// TRAP 3: sequence point violation
int i = 0;
i = i++;
// UB: modifying i while also using its value without sequence point
// Different compilers produce different results

// TRAP 4: operator precedence
int x = 2 + 3 * 4;  // = 14 (not 20), * has higher precedence
int y = 1 << 2 + 1; // = 1 << 3 = 8 (not (1<<2)+1=5), + has higher than <<

// TRAP 5: printf format mismatch
printf("%d", 3.14);
// Passes double (8 bytes) but format reads int (4 bytes)
// Output: garbage (reads wrong bytes)

// TRAP 6: array out of bounds in printf puzzle
int arr[] = {10, 20, 30};
printf("%d\n", arr[5]);
// Out of bounds — reads whatever is at arr+5 in memory
// Likely garbage or crash
```

---

# PART 3: DATA STRUCTURES — INTERNALS, ASCII, AND IMPLEMENTATIONS

---

## 3.1 Arrays — Foundation of Everything

An array is a contiguous block of memory with elements of uniform size.

```
int arr[6] = {10, 20, 30, 40, 50, 60};

Memory layout:
Index:  [0]  [1]  [2]  [3]  [4]  [5]
Addr:  1000 1004 1008 1012 1016 1020
Data: [ 10][ 20][ 30][ 40][ 50][ 60]

Access formula: address(arr[i]) = base_address + i * sizeof(element)
arr[3] = 1000 + 3 * 4 = 1012 → value 40
O(1) access — this is why arrays are so fast
```

### Dynamic Array (Growable Array) — How Vec/vector Works

```
Initial: capacity=4, length=0
+----+----+----+----+
|    |    |    |    |
+----+----+----+----+

Push 4 elements: capacity=4, length=4
+----+----+----+----+
| 10 | 20 | 30 | 40 |
+----+----+----+----+

Push 5th element: FULL → GROW
1. Allocate new array with capacity*2 = 8
2. Copy all elements
3. Free old array
4. Set length=5, capacity=8

+----+----+----+----+----+----+----+----+
| 10 | 20 | 30 | 40 | 50 |    |    |    |
+----+----+----+----+----+----+----+----+
```

Amortized O(1) push: doubling strategy means N pushes require O(N) total copies.

```c
// C implementation
typedef struct {
    int *data;
    size_t len;
    size_t cap;
} DynArray;

DynArray da_new() {
    return (DynArray){ .data = NULL, .len = 0, .cap = 0 };
}

void da_push(DynArray *da, int val) {
    if (da->len == da->cap) {
        size_t new_cap = da->cap == 0 ? 4 : da->cap * 2;
        da->data = realloc(da->data, new_cap * sizeof(int));
        da->cap = new_cap;
    }
    da->data[da->len++] = val;
}

void da_free(DynArray *da) {
    free(da->data);
    da->data = NULL;
    da->len = da->cap = 0;
}
```

```go
// Go: built-in slice IS a dynamic array
// Internal representation:
// type slice struct {
//     array unsafe.Pointer
//     len   int
//     cap   int
// }

s := make([]int, 0, 4)
s = append(s, 10, 20, 30, 40)
s = append(s, 50) // triggers growth: new backing array, len=5, cap=8
```

```rust
// Rust: Vec<T> is a dynamic array
let mut v: Vec<i32> = Vec::with_capacity(4);
v.push(10); v.push(20); v.push(30); v.push(40);
v.push(50); // triggers growth internally
// Vec owns the heap allocation, freed when Vec is dropped
```

---

## 3.2 Linked Lists

### Singly Linked List

```
+------+------+    +------+------+    +------+------+
| data | next-+--->| data | next-+--->| data | NULL |
+------+------+    +------+------+    +------+------+
head                                  tail

Traversal: O(n)
Access by index: O(n)
Insert at head: O(1)
Insert at tail: O(n) without tail pointer, O(1) with tail pointer
Delete by value: O(n) to find, O(1) to remove once found
```

```c
// C implementation
typedef struct Node {
    int data;
    struct Node *next;
} Node;

Node *node_new(int data) {
    Node *n = malloc(sizeof(Node));
    n->data = data;
    n->next = NULL;
    return n;
}

// Insert at head: O(1)
Node *insert_front(Node *head, int data) {
    Node *n = node_new(data);
    n->next = head;
    return n;  // new head
}

// Insert at tail: O(n)
Node *insert_back(Node *head, int data) {
    Node *n = node_new(data);
    if (!head) return n;
    Node *cur = head;
    while (cur->next) cur = cur->next;
    cur->next = n;
    return head;
}

// Delete node with value: O(n)
Node *delete_val(Node *head, int val) {
    if (!head) return NULL;
    if (head->data == val) {
        Node *next = head->next;
        free(head);
        return next;
    }
    Node *cur = head;
    while (cur->next && cur->next->data != val)
        cur = cur->next;
    if (cur->next) {
        Node *to_del = cur->next;
        cur->next = to_del->next;
        free(to_del);
    }
    return head;
}

// Free entire list
void list_free(Node *head) {
    while (head) {
        Node *next = head->next;
        free(head);
        head = next;
    }
}
```

```go
// Go implementation
type Node struct {
    Data int
    Next *Node
}

type LinkedList struct {
    Head *Node
    Len  int
}

func (l *LinkedList) PushFront(data int) {
    l.Head = &Node{Data: data, Next: l.Head}
    l.Len++
}

func (l *LinkedList) PushBack(data int) {
    node := &Node{Data: data}
    if l.Head == nil {
        l.Head = node
    } else {
        cur := l.Head
        for cur.Next != nil { cur = cur.Next }
        cur.Next = node
    }
    l.Len++
}
```

```rust
// Rust implementation
// Linked lists in Rust are notorious for fighting the borrow checker
// The idiomatic way uses Box and Option

type Link<T> = Option<Box<Node<T>>>;

struct Node<T> {
    data: T,
    next: Link<T>,
}

struct LinkedList<T> {
    head: Link<T>,
    len: usize,
}

impl<T> LinkedList<T> {
    fn new() -> Self {
        LinkedList { head: None, len: 0 }
    }

    fn push_front(&mut self, data: T) {
        let old_head = self.head.take();
        self.head = Some(Box::new(Node { data, next: old_head }));
        self.len += 1;
    }

    fn pop_front(&mut self) -> Option<T> {
        self.head.take().map(|node| {
            self.head = node.next;
            self.len -= 1;
            node.data
        })
    }
}

impl<T> Drop for LinkedList<T> {
    fn drop(&mut self) {
        let mut cur = self.head.take();
        while let Some(mut node) = cur {
            cur = node.next.take(); // prevents recursive drop (stack overflow)
        }
    }
}
```

### Doubly Linked List

```
NULL ←+------+------+    +------+------+    +------+------+→ NULL
      | prev | data | ↔  | prev | data | ↔  | prev | data |
      | NULL |  10  |    | ←A   |  20  |    | ←B   |  30  |
      +------+------+    +------+------+    +------+------+
      head                                  tail

Benefits:
- O(1) traversal in both directions
- O(1) deletion if you have a pointer to the node (no need to find prev)
- Easier to implement reverse iteration

Trade-off:
- 2x pointer overhead per node
- More complex insert/delete (must update both prev and next)
```

---

## 3.3 Stack (ADT)

Stack is LIFO (Last In, First Out). Operations: push, pop, peek, isEmpty.

```
PUSH 1, 2, 3, 4:

After push(1):    After push(2):    After push(3):    After push(4):
+---+             +---+             +---+             +---+
|   | ← top      | 2 | ← top       | 3 | ← top       | 4 | ← top
+---+             +---+             +---+             +---+
| 1 |             | 1 |             | 2 |             | 3 |
+---+             +---+             +---+             +---+
                                    | 1 |             | 2 |
                                    +---+             +---+
                                                      | 1 |
                                                      +---+
pop() returns 4, then 3, then 2, then 1
```

### Array-based stack (O(1) push/pop, O(n) space)

```c
// C
#define MAX_STACK 1024
typedef struct {
    int data[MAX_STACK];
    int top;
} Stack;

void stack_init(Stack *s) { s->top = -1; }
bool stack_push(Stack *s, int val) {
    if (s->top == MAX_STACK - 1) return false; // overflow
    s->data[++s->top] = val;
    return true;
}
int stack_pop(Stack *s) {
    if (s->top < 0) return -1; // underflow
    return s->data[s->top--];
}
int stack_peek(Stack *s) {
    return s->data[s->top];
}
```

```go
// Go: use a slice as a stack
type Stack[T any] struct {
    data []T
}
func (s *Stack[T]) Push(v T)     { s.data = append(s.data, v) }
func (s *Stack[T]) Pop() (T, bool) {
    if len(s.data) == 0 { var zero T; return zero, false }
    v := s.data[len(s.data)-1]
    s.data = s.data[:len(s.data)-1]
    return v, true
}
```

```rust
// Rust: Vec<T> is a natural stack
struct Stack<T>(Vec<T>);
impl<T> Stack<T> {
    fn new() -> Self { Stack(Vec::new()) }
    fn push(&mut self, v: T) { self.0.push(v) }
    fn pop(&mut self) -> Option<T> { self.0.pop() }
    fn peek(&self) -> Option<&T> { self.0.last() }
}
```

---

## 3.4 Queue (ADT)

Queue is FIFO (First In, First Out). Operations: enqueue, dequeue, peek, isEmpty.

```
Enqueue 1, 2, 3:                   Dequeue:
front → [1][2][3] ← rear    →     [2][3] ← rear  (returns 1)
```

### Circular Queue (Ring Buffer)

Avoids O(n) shift on dequeue using modular arithmetic:

```
capacity = 5, initially empty:
Index:  [0][1][2][3][4]
Data:   [ ][ ][ ][ ][ ]
front=0, rear=0, size=0

Enqueue A,B,C:
        [A][B][C][ ][ ]
front=0, rear=3, size=3

Dequeue → A:
        [ ][B][C][ ][ ]
front=1, rear=3, size=2

Enqueue D,E,F (F wraps around!):
        [F][B][C][D][E]
front=1, rear=1, size=5 (FULL)

rear = (rear + 1) % capacity   // wraps: 4+1 % 5 = 0
```

```c
// C circular queue
typedef struct {
    int *data;
    int  front, rear, size, cap;
} CircularQueue;

CircularQueue cq_new(int cap) {
    CircularQueue q;
    q.data  = malloc(cap * sizeof(int));
    q.front = q.rear = q.size = 0;
    q.cap   = cap;
    return q;
}

bool cq_enqueue(CircularQueue *q, int val) {
    if (q->size == q->cap) return false;
    q->data[q->rear] = val;
    q->rear = (q->rear + 1) % q->cap;
    q->size++;
    return true;
}

int cq_dequeue(CircularQueue *q) {
    int val = q->data[q->front];
    q->front = (q->front + 1) % q->cap;
    q->size--;
    return val;
}
```

---

## 3.5 Hash Tables

A hash table maps keys to values using a hash function. Average O(1) insert/lookup/delete.

### Hash Function Concept

```
key → [hash function] → index → bucket

"alice" → hash("alice") = 5381*31^4+... = 912345678 % 8 = 2
"bob"   → hash("bob")   = ... % 8 = 5
"carol" → hash("carol") = ... % 8 = 2  ← COLLISION with "alice"!
```

### Collision Resolution

#### Separate Chaining

Each bucket is a linked list. Colliding keys form a chain:

```
Buckets:
[0]: → NULL
[1]: → NULL
[2]: → ["alice":A] → ["carol":C] → NULL   (collision chained)
[3]: → NULL
[4]: → NULL
[5]: → ["bob":B] → NULL
[6]: → NULL
[7]: → NULL

Load factor λ = n/m (items / buckets)
Average lookup: O(1 + λ)
Typically resize when λ > 0.75
```

#### Open Addressing (Linear Probing)

If bucket is occupied, try next:

```
Insert "alice" at 2:     Insert "carol" at 2 (occupied!) → try 3:
[0]:                     [0]:
[1]:                     [1]:
[2]: alice               [2]: alice
[3]:                     [3]: carol    ← placed at next empty slot
[4]:                     [4]:
[5]: bob                 [5]: bob

Lookup "carol": hash→2, check 2 (alice, not carol), check 3 (carol!) ✓
Delete: CANNOT simply clear! Must use tombstone marker or shift
```

```c
// C hash table with separate chaining
#define NBUCKETS 64

typedef struct Entry {
    char        *key;
    void        *val;
    struct Entry *next;
} Entry;

typedef struct {
    Entry *buckets[NBUCKETS];
    size_t size;
} HashMap;

static uint64_t fnv1a(const char *key) {
    uint64_t h = 14695981039346656037ULL;
    for (; *key; key++) {
        h ^= (uint8_t)*key;
        h *= 1099511628211ULL;
    }
    return h;
}

void hm_put(HashMap *m, const char *key, void *val) {
    uint64_t idx = fnv1a(key) % NBUCKETS;
    Entry *e = m->buckets[idx];
    while (e) {
        if (strcmp(e->key, key) == 0) { e->val = val; return; }
        e = e->next;
    }
    Entry *ne = malloc(sizeof(Entry));
    ne->key  = strdup(key);
    ne->val  = val;
    ne->next = m->buckets[idx];
    m->buckets[idx] = ne;
    m->size++;
}

void *hm_get(HashMap *m, const char *key) {
    uint64_t idx = fnv1a(key) % NBUCKETS;
    for (Entry *e = m->buckets[idx]; e; e = e->next)
        if (strcmp(e->key, key) == 0) return e->val;
    return NULL;
}
```

```go
// Go: map is a built-in hash table
// Internal: array of buckets, each bucket holds 8 key-value pairs
// Uses quadratic probing + overflow chaining
m := make(map[string]int)
m["alice"] = 100
v, ok := m["alice"]   // ok=true, v=100
v, ok = m["carol"]    // ok=false, v=0 (zero value)
delete(m, "alice")

// Iterating (order is RANDOMIZED by design to prevent reliance on order):
for k, v := range m {
    fmt.Println(k, v)
}
```

```rust
// Rust: HashMap<K, V> from std::collections
use std::collections::HashMap;

let mut m = HashMap::new();
m.insert("alice", 100);
m.insert("bob", 200);

if let Some(v) = m.get("alice") {
    println!("{}", v);  // 100
}

// Entry API — powerful for "insert if absent" pattern:
m.entry("carol").or_insert(300);       // insert if not present
*m.entry("alice").or_insert(0) += 50;  // alice = 150

// Iteration (also random order):
for (k, v) in &m {
    println!("{}: {}", k, v);
}
```

---

## 3.6 Binary Trees and BST

### Tree Terminology

```
           [8]            ← root
          /   \
        [3]   [10]        ← internal nodes
        / \      \
      [1] [6]   [14]      ← [1],[6],[14] are leaves
          / \   /
        [4] [7][13]

root: topmost node
leaf: node with no children
parent/child: edge relationship
depth of node: edges from root to node (root has depth 0)
height of tree: max depth of any leaf
subtree: any node and its descendants
```

### Binary Search Tree (BST) Property

```
For every node N:
- All values in N's LEFT subtree < N's value
- All values in N's RIGHT subtree > N's value

BST:            NOT a BST (5 is in wrong place):
     8               8
    / \             / \
   3   10          5   10
  / \              \
 1   6              3 ← wrong: 3 is to right of 5 but less than 5
```

BST operations:

```
SEARCH 6:
Start at root 8: 6 < 8, go left to 3
At 3: 6 > 3, go right to 6
At 6: found! O(h) where h = height

INSERT 5:
Search where 5 would be:
8 → go left to 3 → go right to 6 → go left (6's left is NULL)
Place 5 as 6's left child

DELETE 6 (has two children):
Find in-order successor (smallest in right subtree) = 7
Replace 6's value with 7
Delete the original 7 node

In-order traversal (left → root → right) gives SORTED order:
1, 3, 4, 6, 7, 8, 10, 13, 14
```

```c
// C BST implementation
typedef struct BST {
    int data;
    struct BST *left, *right;
} BST;

BST *bst_insert(BST *root, int val) {
    if (!root) {
        BST *n = malloc(sizeof(BST));
        n->data = val; n->left = n->right = NULL;
        return n;
    }
    if (val < root->data) root->left  = bst_insert(root->left,  val);
    else if (val > root->data) root->right = bst_insert(root->right, val);
    return root;
}

BST *bst_search(BST *root, int val) {
    if (!root || root->data == val) return root;
    if (val < root->data) return bst_search(root->left,  val);
    else                  return bst_search(root->right, val);
}

// In-order traversal: produces sorted output
void bst_inorder(BST *root) {
    if (!root) return;
    bst_inorder(root->left);
    printf("%d ", root->data);
    bst_inorder(root->right);
}
```

### BST Degeneracy Problem

```
Insert 1,2,3,4,5 in order into BST:
1
 \
  2
   \
    3
     \
      4
       \
        5

This is now a linked list! Height = n, ALL operations O(n).
Need self-balancing trees to guarantee O(log n).
```

---

## 3.7 AVL Trees — Self-Balancing BST

AVL trees maintain: |height(left) - height(right)| ≤ 1 for every node.

```
Balance factor = height(left) - height(right)
Valid: -1, 0, +1
If |BF| > 1 → ROTATE to restore balance

ROTATIONS:

Left Rotation (when right-heavy):
        3              5
       / \            / \
      2   5    →     3   6
         / \        / \
        4   6      2   4

Right Rotation (when left-heavy):
        5              3
       / \            / \
      3   6    →     2   5
     / \                / \
    2   4              4   6

Left-Right Rotation (zigzag left-right):
    5                5              4
   /                /              / \
  2       →        4      →       2   5
   \              /
    4            2

Right-Left Rotation (zigzag right-left):
  2            2              4
   \            \            / \
    5    →       4    →     2   5
   /              \
  4                5
```

AVL trees guarantee O(log n) for all operations because the height is bounded by 1.44 * log2(n).

---

## 3.8 Red-Black Tree

A Red-Black tree is a BST where each node has a color (red or black) and satisfies:
1. Root is black
2. Red nodes have only black children (no two consecutive red nodes)
3. Every path from root to null has the same number of black nodes

```
         (8,B)
        /      \
     (3,R)    (10,B)
     /   \         \
  (1,B) (6,B)    (14,R)
        /  \     /
      (4,R)(7,R)(13,R)

B=Black, R=Red

Black height (# black nodes on any root→null path) = 2 everywhere ✓
No consecutive reds ✓
Root is black ✓
```

Red-Black trees guarantee height ≤ 2 * log2(n+1), so O(log n) operations.

**Why Red-Black over AVL?**
- Faster insertions/deletions (fewer rotations: max 3 vs AVL's up to O(log n))
- AVL slightly faster lookups (stricter balance)
- Used in: Linux kernel (CFS scheduler), Java TreeMap, C++ std::map, Nginx

---

## 3.9 Heaps — Priority Queues

A binary heap is a complete binary tree stored as an array, satisfying the heap property.

```
Max-Heap property: parent >= children for all nodes

Heap as tree:        Same heap as array:
        90           Index: [0]  [1] [2]  [3] [4]  [5]  [6]
       /  \          Data: [ 90][ 70][80][ 50][60][ 75][ 40]
      70   80
     / \  / \
    50 60 75 40

Formulas for index i:
  parent(i)      = (i - 1) / 2
  left_child(i)  = 2 * i + 1
  right_child(i) = 2 * i + 2

parent(3) = (3-1)/2 = 1 → value 70 ✓ (50's parent is 70)
left_child(1) = 2*1+1 = 3 → value 50 ✓
```

### Heap Operations

```
INSERT 85 into max-heap:
1. Add at end (index 6):
   [90][70][80][50][60][75][40][85]
                                ↑ new

2. "Sift up" — compare with parent, swap if larger:
   parent(7) = 3, value 50 < 85 → SWAP
   [90][70][80][85][60][75][40][50]
   parent(3) = 1, value 70 < 85 → SWAP
   [90][85][80][70][60][75][40][50]
   parent(1) = 0, value 90 > 85 → STOP

EXTRACT MAX (remove root 90):
1. Replace root with last element:
   [50][85][80][70][60][75][40]
2. "Sift down" — compare with children, swap with larger child:
   children of 0: index 1 (85) and 2 (80), 85 > 50 → SWAP with 1
   [85][50][80][70][60][75][40]
   children of 1: index 3 (70) and 4 (60), 70 > 50 → SWAP with 3
   [85][70][80][50][60][75][40]
   children of 3: index 7 (none), 8 (none) → STOP
```

```c
// C max-heap
#define MAXHEAP 1024
typedef struct {
    int data[MAXHEAP];
    int size;
} Heap;

void heap_push(Heap *h, int val) {
    int i = h->size++;
    h->data[i] = val;
    // sift up
    while (i > 0) {
        int p = (i - 1) / 2;
        if (h->data[p] >= h->data[i]) break;
        int tmp = h->data[p]; h->data[p] = h->data[i]; h->data[i] = tmp;
        i = p;
    }
}

int heap_pop(Heap *h) {
    int max = h->data[0];
    h->data[0] = h->data[--h->size];
    // sift down
    int i = 0;
    for (;;) {
        int l = 2*i+1, r = 2*i+2, largest = i;
        if (l < h->size && h->data[l] > h->data[largest]) largest = l;
        if (r < h->size && h->data[r] > h->data[largest]) largest = r;
        if (largest == i) break;
        int tmp = h->data[i]; h->data[i] = h->data[largest]; h->data[largest] = tmp;
        i = largest;
    }
    return max;
}
```

```go
// Go: container/heap interface
import "container/heap"

type MaxHeap []int
func (h MaxHeap) Len() int           { return len(h) }
func (h MaxHeap) Less(i, j int) bool { return h[i] > h[j] } // > for max-heap
func (h MaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *MaxHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *MaxHeap) Pop() any {
    old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x
}

h := &MaxHeap{5, 3, 8, 1}
heap.Init(h)
heap.Push(h, 10)
fmt.Println(heap.Pop(h)) // 10
```

```rust
// Rust: BinaryHeap (max-heap by default)
use std::collections::BinaryHeap;
use std::cmp::Reverse;

let mut max_heap = BinaryHeap::new();
max_heap.push(5); max_heap.push(3); max_heap.push(8);
println!("{}", max_heap.peek().unwrap()); // 8
println!("{}", max_heap.pop().unwrap());  // 8

// Min-heap using Reverse wrapper:
let mut min_heap: BinaryHeap<Reverse<i32>> = BinaryHeap::new();
min_heap.push(Reverse(5)); min_heap.push(Reverse(3)); min_heap.push(Reverse(8));
println!("{}", min_heap.pop().unwrap().0); // 3
```

### Heapify — Building Heap from Array in O(n)

```
Naive: insert n elements one by one = O(n log n)

Better: Heapify — start from last non-leaf, sift down each
Last non-leaf index = n/2 - 1

For array [4, 10, 3, 5, 1]:
n=5, start at index 1 (10):
  children of 1: 3(5) and 4(1), 5>10? No → no swap
Start at index 0 (4):
  children of 0: 1(10) and 2(3), 10>4 → swap
  [10, 4, 3, 5, 1]
  children of 1: 3(5) and 4(1), 5>4 → swap
  [10, 5, 3, 4, 1] ← done!

Why O(n)? Half the nodes are leaves (0 sift-down work), quarter need 1 swap, etc.
Sum converges to O(n) by mathematical analysis.
```

---

## 3.10 Graphs

A graph G = (V, E) consists of vertices and edges.

```
Types:
- Directed (digraph): edges have direction A→B
- Undirected: edges are bidirectional A—B
- Weighted: edges have weights/costs
- DAG: Directed Acyclic Graph (no cycles)

Example undirected graph:
    0 --- 1
    |   / |
    |  /  |
    | /   |
    2 --- 3

Adjacency Matrix (O(V²) space):
    0  1  2  3
0 [ 0  1  1  0 ]
1 [ 1  0  1  1 ]
2 [ 1  1  0  1 ]
3 [ 0  1  1  0 ]

Adjacency List (O(V+E) space):
0: [1, 2]
1: [0, 2, 3]
2: [0, 1, 3]
3: [1, 2]
```

### BFS (Breadth-First Search)

Explores level by level. Uses a queue.

```
BFS from vertex 0:

Level 0: visit 0
Queue: [0]    Visited: {0}

Process 0, enqueue neighbors 1,2:
Queue: [1,2]  Visited: {0,1,2}

Process 1, enqueue unvisited neighbors (3):
Queue: [2,3]  Visited: {0,1,2,3}

Process 2, neighbors 0,1,3 all visited:
Queue: [3]

Process 3, all neighbors visited:
Queue: []

BFS Order: 0 → 1 → 2 → 3

BFS gives SHORTEST PATH in unweighted graph.
```

### DFS (Depth-First Search)

Explores as deep as possible before backtracking. Uses a stack (or recursion).

```
DFS from vertex 0 (recursive):

Visit 0 → go to neighbor 1
  Visit 1 → go to neighbor 2
    Visit 2 → go to neighbor 3
      Visit 3 → no unvisited neighbors → backtrack
    back at 2 → no more unvisited → backtrack
  back at 1 → no more unvisited → backtrack
back at 0 → no more unvisited → done

DFS Order: 0 → 1 → 2 → 3
```

```c
// C: BFS implementation
#include <stdio.h>
#include <stdbool.h>
#include <stdlib.h>

#define VMAX 100

typedef struct { int to, next; } Edge;
Edge edges[VMAX * VMAX];
int head[VMAX]; // head[v] = index of first edge from v, -1 if none
int edge_cnt = 0;

void add_edge(int from, int to) {
    edges[edge_cnt].to   = to;
    edges[edge_cnt].next = head[from];
    head[from]           = edge_cnt++;
}

void bfs(int start, int n) {
    bool visited[VMAX] = {false};
    int  queue[VMAX];
    int  front = 0, rear = 0;

    visited[start] = true;
    queue[rear++]  = start;

    while (front < rear) {
        int v = queue[front++];
        printf("%d ", v);
        for (int e = head[v]; e != -1; e = edges[e].next) {
            int u = edges[e].to;
            if (!visited[u]) {
                visited[u] = true;
                queue[rear++] = u;
            }
        }
    }
}
```

```go
// Go: DFS using recursion
func dfs(graph map[int][]int, node int, visited map[int]bool) {
    if visited[node] { return }
    visited[node] = true
    fmt.Print(node, " ")
    for _, neighbor := range graph[node] {
        dfs(graph, neighbor, visited)
    }
}

graph := map[int][]int{
    0: {1, 2},
    1: {0, 2, 3},
    2: {0, 1, 3},
    3: {1, 2},
}
dfs(graph, 0, make(map[int]bool))
```

```rust
// Rust: BFS
use std::collections::{HashMap, HashSet, VecDeque};

fn bfs(graph: &HashMap<i32, Vec<i32>>, start: i32) -> Vec<i32> {
    let mut visited = HashSet::new();
    let mut queue = VecDeque::new();
    let mut order = Vec::new();

    visited.insert(start);
    queue.push_back(start);

    while let Some(node) = queue.pop_front() {
        order.push(node);
        if let Some(neighbors) = graph.get(&node) {
            for &n in neighbors {
                if visited.insert(n) {
                    queue.push_back(n);
                }
            }
        }
    }
    order
}
```

### Dijkstra's Algorithm — Shortest Path in Weighted Graph

```
Weighted graph:
    0 --2-- 1
    |       |
    6       3
    |       |
    2 --1-- 3

Find shortest path from 0 to all vertices.

dist[] = [0, ∞, ∞, ∞]   // distance from source
priority queue: [(0, 0)]  // (distance, node)

Pop (0,0):
  neighbor 1: dist[1] = 0+2 = 2 → push (2,1)
  neighbor 2: dist[2] = 0+6 = 6 → push (6,2)
  dist[] = [0, 2, 6, ∞]

Pop (2,1):
  neighbor 3: dist[3] = 2+3 = 5 → push (5,3)
  dist[] = [0, 2, 6, 5]

Pop (5,3):
  neighbor 2: dist[2] = 5+1 = 6, not better (already 6)
  dist[] = [0, 2, 6, 5]

Pop (6,2): no better paths
Final: dist = [0, 2, 6, 5]
```

```go
// Go: Dijkstra with priority queue
import (
    "container/heap"
    "math"
)

type Item struct { node, dist int }
type PQ []Item
func (pq PQ) Len() int           { return len(pq) }
func (pq PQ) Less(i, j int) bool { return pq[i].dist < pq[j].dist }
func (pq PQ) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i] }
func (pq *PQ) Push(x any)        { *pq = append(*pq, x.(Item)) }
func (pq *PQ) Pop() any          { old:=*pq; n:=len(old); x:=old[n-1]; *pq=old[:n-1]; return x }

func dijkstra(graph map[int][][2]int, src, n int) []int {
    dist := make([]int, n)
    for i := range dist { dist[i] = math.MaxInt64 }
    dist[src] = 0

    pq := &PQ{{src, 0}}
    heap.Init(pq)

    for pq.Len() > 0 {
        cur := heap.Pop(pq).(Item)
        if cur.dist > dist[cur.node] { continue }
        for _, e := range graph[cur.node] {
            nb, w := e[0], e[1]
            if d := dist[cur.node] + w; d < dist[nb] {
                dist[nb] = d
                heap.Push(pq, Item{nb, d})
            }
        }
    }
    return dist
}
```

---

## 3.11 Sorting Algorithms

### Comparison of Sorting Algorithms

```
Algorithm    | Best     | Average  | Worst    | Space | Stable
-------------|----------|----------|----------|-------|-------
Bubble Sort  | O(n)     | O(n²)    | O(n²)    | O(1)  | Yes
Insertion    | O(n)     | O(n²)    | O(n²)    | O(1)  | Yes
Selection    | O(n²)    | O(n²)    | O(n²)    | O(1)  | No
Merge Sort   | O(n logn)| O(n logn)| O(n logn)| O(n)  | Yes
Quick Sort   | O(n logn)| O(n logn)| O(n²)    | O(logn)| No
Heap Sort    | O(n logn)| O(n logn)| O(n logn)| O(1)  | No
Tim Sort     | O(n)     | O(n logn)| O(n logn)| O(n)  | Yes
Counting Sort| O(n+k)   | O(n+k)   | O(n+k)   | O(k)  | Yes
Radix Sort   | O(nk)    | O(nk)    | O(nk)    | O(n+k)| Yes
```

### Merge Sort — Divide and Conquer

```
[38, 27, 43, 3, 9, 82, 10]

SPLIT:
[38, 27, 43, 3]    [9, 82, 10]

[38, 27] [43, 3]   [9, 82] [10]

[38] [27] [43] [3] [9] [82] [10]

MERGE (bottom up):
[27,38] [3,43]     [9,82] [10]

[3,27,38,43]       [9,10,82]

[3,9,10,27,38,43,82]

MERGE step:
Left: [27,38]   Right: [3,43]

i=0,j=0: 27 vs 3 → take 3, j=1
i=0,j=1: 27 vs 43 → take 27, i=1
i=1,j=1: 38 vs 43 → take 38, i=2 (left exhausted)
append remaining: 43
Result: [3,27,38,43]
```

```c
// C merge sort
void merge(int *arr, int l, int m, int r) {
    int n1 = m - l + 1, n2 = r - m;
    int L[n1], R[n2];
    for (int i = 0; i < n1; i++) L[i] = arr[l + i];
    for (int j = 0; j < n2; j++) R[j] = arr[m + 1 + j];
    int i = 0, j = 0, k = l;
    while (i < n1 && j < n2)
        arr[k++] = (L[i] <= R[j]) ? L[i++] : R[j++];
    while (i < n1) arr[k++] = L[i++];
    while (j < n2) arr[k++] = R[j++];
}

void mergesort(int *arr, int l, int r) {
    if (l >= r) return;
    int m = l + (r - l) / 2;
    mergesort(arr, l, m);
    mergesort(arr, m + 1, r);
    merge(arr, l, m, r);
}
```

```go
// Go merge sort
func mergeSort(arr []int) []int {
    if len(arr) <= 1 { return arr }
    mid := len(arr) / 2
    left  := mergeSort(arr[:mid])
    right := mergeSort(arr[mid:])
    return merge(left, right)
}
func merge(l, r []int) []int {
    result := make([]int, 0, len(l)+len(r))
    for len(l) > 0 && len(r) > 0 {
        if l[0] <= r[0] { result = append(result, l[0]); l = l[1:] } else
                        { result = append(result, r[0]); r = r[1:] }
    }
    return append(append(result, l...), r...)
}
```

```rust
// Rust merge sort
fn merge_sort(arr: &mut Vec<i32>) {
    let len = arr.len();
    if len <= 1 { return; }
    let mid = len / 2;
    let mut left  = arr[..mid].to_vec();
    let mut right = arr[mid..].to_vec();
    merge_sort(&mut left);
    merge_sort(&mut right);
    let (mut i, mut j, mut k) = (0, 0, 0);
    while i < left.len() && j < right.len() {
        if left[i] <= right[j] { arr[k] = left[i]; i += 1; }
        else                   { arr[k] = right[j]; j += 1; }
        k += 1;
    }
    while i < left.len()  { arr[k] = left[i];  i += 1; k += 1; }
    while j < right.len() { arr[k] = right[j]; j += 1; k += 1; }
}
```

### Quick Sort — Partition and Conquer

```
[3, 6, 8, 10, 1, 2, 1] pivot = 1 (last element)

PARTITION around pivot 1:
Maintain: elements ≤ pivot on left, > pivot on right

i starts at -1 (last position of ≤ region)
j scans left to right:

j=0: arr[0]=3 > pivot=1 → skip
j=1: arr[1]=6 > pivot → skip
j=2: arr[2]=8 > pivot → skip
j=3: arr[3]=10 > pivot → skip
j=4: arr[4]=1 ≤ pivot → i++, swap arr[0],arr[4]: [1,6,8,10,3,2,1]
j=5: arr[5]=2 > pivot → skip
Place pivot: swap arr[i+1],arr[last]: [1,1,8,10,3,2,6]
Pivot index = 1

Recurse on [1] and [8,10,3,2,6]

Worst case (already sorted + bad pivot): O(n²)
Average case (random pivot): O(n log n)
Fix: random pivot selection or median-of-three
```

### Binary Search

```
Search for 7 in sorted array [1,3,5,7,9,11,13,15]:

lo=0, hi=7

Iteration 1: mid = (0+7)/2 = 3, arr[3]=7
Found at index 3!

Search for 6:
Iteration 1: mid=3, arr[3]=7 > 6 → hi = mid-1 = 2
Iteration 2: mid=(0+2)/2=1, arr[1]=3 < 6 → lo = mid+1 = 2
Iteration 3: mid=(2+2)/2=2, arr[2]=5 < 6 → lo = 3
lo > hi → NOT FOUND

Key insight: use lo + (hi-lo)/2, NOT (lo+hi)/2 to avoid integer overflow!
```

```c
int binary_search(int *arr, int n, int target) {
    int lo = 0, hi = n - 1;
    while (lo <= hi) {
        int mid = lo + (hi - lo) / 2;
        if      (arr[mid] == target) return mid;
        else if (arr[mid] < target)  lo = mid + 1;
        else                         hi = mid - 1;
    }
    return -1;
}
```

```rust
fn binary_search(arr: &[i32], target: i32) -> Option<usize> {
    let (mut lo, mut hi) = (0usize, arr.len());
    while lo < hi {
        let mid = lo + (hi - lo) / 2;
        match arr[mid].cmp(&target) {
            std::cmp::Ordering::Equal   => return Some(mid),
            std::cmp::Ordering::Less    => lo = mid + 1,
            std::cmp::Ordering::Greater => hi = mid,
        }
    }
    None
}
```

---

## 3.12 Tries (Prefix Trees)

A trie is a tree where each path from root to a node represents a prefix of a stored string.

```
Stored words: "cat", "car", "card", "care", "bat"

         root
        /    \
       c      b
       |      |
       a      a
      / \     |
     t   r    t
         |\ 
         d  e

Search "car": root→c→a→r → found (if marked as word end)
Search "ca": root→c→a → NOT a complete word
Search "carp": root→c→a→r, no 'p' child → NOT FOUND

Applications:
- Autocomplete
- Spell checkers
- IP routing (longest prefix match)
- Dictionary lookups
```

```c
// C trie implementation
#define ALPHA 26
typedef struct Trie {
    struct Trie *children[ALPHA];
    bool is_end;
} Trie;

Trie *trie_new()  { return calloc(1, sizeof(Trie)); }

void trie_insert(Trie *root, const char *word) {
    Trie *cur = root;
    for (; *word; word++) {
        int idx = *word - 'a';
        if (!cur->children[idx])
            cur->children[idx] = trie_new();
        cur = cur->children[idx];
    }
    cur->is_end = true;
}

bool trie_search(Trie *root, const char *word) {
    Trie *cur = root;
    for (; *word; word++) {
        int idx = *word - 'a';
        if (!cur->children[idx]) return false;
        cur = cur->children[idx];
    }
    return cur->is_end;
}

bool trie_starts_with(Trie *root, const char *prefix) {
    Trie *cur = root;
    for (; *prefix; prefix++) {
        int idx = *prefix - 'a';
        if (!cur->children[idx]) return false;
        cur = cur->children[idx];
    }
    return true;
}
```

---

# PART 4: GO LANGUAGE — COMPLETE DEEP DIVE

---

## 4.1 The Go Runtime and GMP Scheduler

Go has its own runtime that sits between user code and the OS. Its most critical component is the **GMP scheduler** — the goroutine scheduler.

```
GMP MODEL:

G = Goroutine (user-space lightweight thread)
M = Machine (OS thread)
P = Processor (scheduling context, max = GOMAXPROCS)

                   +---+---+---+---+
                   |P0 |P1 |P2 |P3 |   ← GOMAXPROCS=4 Processors
                   +---+---+---+---+
                    |   |   |   |
                   [M] [M] [M] [M]      ← OS threads (can be > GOMAXPROCS)
                    |
                [G running]              ← 1 goroutine running per P
              
Each P has a local run queue:
P0: runq=[G1, G3, G7]
P1: runq=[G2, G8]
P2: runq=[]           ← idle P will STEAL work from P0
P3: runq=[G4, G5, G6]

Global run queue: [G9, G10]  ← fallback when local queues full

Work stealing:
If P2 is idle, it steals HALF of P0's queue → P2: [G3, G7]
This keeps all CPUs busy → better throughput
```

### Goroutine Internals

A goroutine starts with a **2 KB stack** (unlike OS threads which start at 8 MB). This allows running millions of goroutines.

```
Goroutine structure (simplified):
type g struct {
    stack       stack       // current stack bounds [lo, hi]
    stackguard0 uintptr     // for stack growth check
    m           *m          // current M (OS thread), nil if not running
    sched       gobuf       // saved register state (when preempted)
    status      uint32      // _Gidle, _Grunnable, _Grunning, _Gsyscall, _Gwaiting
    goid        int64       // goroutine ID
}

type gobuf struct {
    sp   uintptr  // stack pointer
    pc   uintptr  // program counter
    g    guintptr // owning goroutine
}
```

### Stack Growth

When a goroutine's stack is about to overflow, Go grows it (not overflow like C):

```
Initial goroutine stack: 2KB
+------------------+
|                  |
|   stack guard    | ← stackguard0 points here
|   [2KB usable]   |
|                  |
+------------------+

Function call checks: if SP < stackguard0 → stack must grow!

Go 1.3+: Copying stack (not segmented)
1. Allocate new stack (2x size): 4KB
2. Copy ALL stack frames to new location
3. Update all pointers to stack variables
4. Release old stack

+----------------------------------+
|                                  |
|   [4KB usable]                   |
|                                  |
+----------------------------------+

Max stack: 1GB by default (runtime.SetMaxStack)
```

### Goroutine States

```
                    ┌──────────┐
                    │  Gidle   │ ← newly allocated, not initialized
                    └────┬─────┘
                         │ go func()
                    ┌────▼─────┐
               ┌───►│Grunnable │◄──── woken up from wait
               │    └────┬─────┘
               │         │ scheduled
               │    ┌────▼─────┐
               │    │ Grunning │ ← actively running on M
               │    └────┬─────┘
               │         │ preempt / Gosched / sleep
               │    ┌────▼─────┐
               └────│ Gwaiting │ ← blocked on channel, timer, syscall
                    └──────────┘
```

### Preemption

Before Go 1.14: goroutines could only be preempted at function call sites (a compiler-inserted check). Tight loops without function calls could starve others.

After Go 1.14: **asynchronous preemption** via signals. The runtime sends `SIGURG` to an M running a goroutine that needs preempting. The signal handler saves goroutine state and yields.

```go
// This would block scheduler pre-1.14:
go func() {
    for i := 0; ; i++ {
        // no function calls → no preemption point pre-1.14
    }
}()

// Post-1.14: runtime sends SIGURG → goroutine preempted mid-loop
```

---

## 4.2 Channels — Deep Dive

Channels are Go's primary synchronization primitive: "Do not communicate by sharing memory; share memory by communicating."

### Channel Internals

```go
ch := make(chan int, 3)  // buffered channel, capacity=3

Internal hchan struct:
+------------------+
| buf (ring buffer)|  ← data: [_, _, _]
| sendx            |  ← next send index
| recvx            |  ← next receive index
| qcount           |  ← current # items
| dataqsiz         |  ← capacity (3)
| sendq            |  ← goroutines blocked on send (list)
| recvq            |  ← goroutines blocked on receive (list)
| lock mutex       |  ← protects all fields
+------------------+
```

### Buffered vs Unbuffered

```
Unbuffered channel (make(chan T)):
  Sender BLOCKS until receiver is ready
  Receiver BLOCKS until sender is ready
  Perfect rendezvous synchronization

  goroutine A: ch <- val  ← BLOCKS
  goroutine B: v := <-ch  ← UNBLOCKS A, receives val

Buffered channel (make(chan T, n)):
  Sender blocks ONLY when buffer full
  Receiver blocks ONLY when buffer empty
  Acts as a bounded queue

  ch = make(chan int, 3)
  ch <- 1  // no block
  ch <- 2  // no block
  ch <- 3  // no block
  ch <- 4  // BLOCKS: buffer full, wait for receiver
```

### Channel as Pipeline

```go
// Generator: produces values
func generate(nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        for _, n := range nums { out <- n }
        close(out)
    }()
    return out
}

// Stage: squares values
func square(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        for n := range in { out <- n * n }
        close(out)
    }()
    return out
}

// Pipeline:
// generate(2,3,4) → square → print
for v := range square(generate(2, 3, 4)) {
    fmt.Println(v)  // 4, 9, 16
}
```

### Select Statement

`select` is like a `switch` for channels — waits on multiple channel operations simultaneously:

```go
select {
case msg := <-ch1:
    // handle msg from ch1
case msg := <-ch2:
    // handle msg from ch2
case ch3 <- result:
    // sent to ch3
case <-time.After(5 * time.Second):
    // timeout
default:
    // non-blocking: executes if no channel is ready
}

// If multiple cases are ready, select picks ONE at random (fair selection)
```

### Channel Directions

```go
func send(ch chan<- int) {   // send-only channel
    ch <- 42
}

func recv(ch <-chan int) {   // receive-only channel
    fmt.Println(<-ch)
}

func both(ch chan int) {     // bidirectional
    ch <- 1
    <-ch
}

// Direction constraints are enforced at compile time
// Bidirectional chan int can be assigned to send-only or recv-only
// But not vice versa
```

### Channel Closing Rules

```go
// CLOSE means: no more values will be sent
close(ch)

// Reading from closed channel:
v, ok := <-ch
// ok=true:  received a real value
// ok=false: channel closed AND empty, v is zero value

// for range automatically stops when channel closes:
for v := range ch { ... }  // exits when ch closed and empty

// PANIC conditions:
close(nil_chan)      // panic
close(closed_chan)   // panic (double-close)
send_to_closed_chan  // panic (ch <- val on closed chan)
```

---

## 4.3 Go Memory Model

The Go memory model defines when reads of a variable in one goroutine can be guaranteed to see writes from another goroutine.

```
The "happens-before" relation:

If event A happens-before event B:
→ all memory writes visible before A are visible after B

Key happens-before rules:
1. Within a single goroutine: program order
2. go statement: go f() HB start of f
3. Channel send: ch <- v HB <-ch (receive)
4. Channel close: close(ch) HB receive of zero value
5. sync.Mutex: Unlock HB next Lock
6. sync.Once: o.Do(f) completes HB any subsequent o.Do(f)
```

### Races — What Goes Wrong Without Synchronization

```go
// DATA RACE: two goroutines access same variable, at least one writes
var counter int  // shared

go func() { counter++ }()  // read-modify-write, NOT atomic
go func() { counter++ }()

// counter++ compiles to: LOAD, ADD, STORE
// Goroutine A:  LOAD(0)         ADD(1)          STORE(1)
// Goroutine B:        LOAD(0)         ADD(1)          STORE(1)
// Result: counter = 1 (should be 2!)

// CORRECT: use sync/atomic or sync.Mutex
var counter int64
go func() { atomic.AddInt64(&counter, 1) }()
go func() { atomic.AddInt64(&counter, 1) }()
// or:
var mu sync.Mutex
go func() { mu.Lock(); counter++; mu.Unlock() }()
```

---

## 4.4 Go Interfaces and Duck Typing

An interface in Go is a set of method signatures. Any type that implements all methods *implicitly* satisfies the interface.

```go
// Interface definition
type Animal interface {
    Sound() string
    Move()  string
}

// Dog satisfies Animal (no "implements" keyword needed)
type Dog struct { Name string }
func (d Dog) Sound() string { return "Woof" }
func (d Dog) Move()  string { return "runs" }

// Cat satisfies Animal
type Cat struct { Name string }
func (c Cat) Sound() string { return "Meow" }
func (c Cat) Move()  string { return "slinks" }

func describe(a Animal) {
    fmt.Printf("%s, %s\n", a.Sound(), a.Move())
}

describe(Dog{"Rex"})  // "Woof, runs"
describe(Cat{"Luna"}) // "Meow, slinks"
```

### Interface Internals (iface)

```
An interface value has TWO words:
+----------+----------+
|  *itab   |  *data   |
+----------+----------+
    8B          8B

itab (interface table):
+------------------+
| *inter (type of  |
|   the interface) |
| *type (concrete  |
|   type)          |
| hash             |
| fun[0] → method1 | ← method pointer 1
| fun[1] → method2 | ← method pointer 2
+------------------+

*data: pointer to the concrete value (or the value itself if it fits in a pointer)

var a Animal = Dog{"Rex"}
a.Sound()
→ look up fun[0] in itab
→ call that function pointer with (*data) as receiver
→ one level of indirection → dynamic dispatch
```

### Empty Interface — interface{}

```go
var any interface{}
any = 42
any = "hello"
any = Dog{"Rex"}

// Used for:
// - Generic containers (pre-generics)
// - fmt.Println(args ...interface{})
// - JSON unmarshaling into unknown structure
// - reflect-based code

// Type assertion:
s, ok := any.(string)   // ok=false, s=""
n, ok := any.(int)      // ok=true, n=42 if any holds an int

// Type switch:
switch v := any.(type) {
case int:    fmt.Println("int:", v)
case string: fmt.Println("string:", v)
case Dog:    fmt.Println("Dog:", v.Name)
default:     fmt.Println("unknown")
}
```

---

## 4.5 Go Error Handling

```go
// Errors are values — the built-in error interface:
type error interface {
    Error() string
}

// Functions return errors as last value:
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
}

result, err := divide(10, 0)
if err != nil {
    log.Fatal(err)
}

// Wrapping errors (Go 1.13+):
import "errors"

var ErrNotFound = errors.New("not found")

func findUser(id int) (*User, error) {
    user, err := db.Query(id)
    if err != nil {
        return nil, fmt.Errorf("findUser %d: %w", id, err) // %w wraps
    }
    return user, nil
}

// Unwrapping:
if errors.Is(err, ErrNotFound) { ... }       // checks the chain
var nfe *NotFoundError
if errors.As(err, &nfe) { ... }              // extracts specific type
```

---

## 4.6 defer, panic, recover

```go
// defer: executes LIFO when surrounding function returns
// Used for cleanup, resource release, logging

func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil { return err }
    defer f.Close()  // guaranteed to run even if panic occurs

    // multiple defers:
    defer fmt.Println("3rd to execute")
    defer fmt.Println("2nd to execute")
    defer fmt.Println("1st to execute")
    // On return: "1st", "2nd", "3rd" (LIFO)

    return processData(f)
}

// panic: like throwing an exception
// recover: catch a panic (only in deferred function)

func safeDiv(a, b int) (result int, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered: %v", r)
        }
    }()
    return a / b, nil  // panic if b=0
}

result, err := safeDiv(10, 0)
// err = "recovered: runtime error: integer divide by zero"
// result = 0
```

---

## 4.7 Go File I/O — Complete Streaming Guide

```go
// The three levels of file reading:

// Level 1: os.ReadFile — loads entire file into memory
// ONLY for small files (< a few MB)
data, err := os.ReadFile("small.txt")

// Level 2: bufio.Scanner — line-by-line, easy API
file, _ := os.Open("medium.log")
defer file.Close()
scanner := bufio.NewScanner(file)
for scanner.Scan() {
    line := scanner.Text()  // one line, no trailing \n
    // process line
}
if err := scanner.Err(); err != nil { ... }

// Level 3: bufio.Reader — chunked, more control
reader := bufio.NewReaderSize(file, 64*1024) // 64KB buffer
for {
    line, err := reader.ReadString('\n')
    if err == io.EOF { break }
    if err != nil { panic(err) }
    // line includes '\n'
}

// Level 4: raw reads — fixed-size chunks, binary data
buf := make([]byte, 4096)
for {
    n, err := file.Read(buf)
    if err == io.EOF { break }
    process(buf[:n])
}
```

### Why Buffered I/O Matters

```
Without bufio:
  Read 1 byte → syscall read(fd, buf, 1) → kernel context switch
  Read 1 byte → syscall read(fd, buf, 1) → kernel context switch
  ... 10000 times for 10000 bytes = 10000 syscalls!

With bufio (64KB buffer):
  First read: syscall read(fd, buf, 65536) → kernel loads 65536 bytes
  Next 65535 reads from memory: NO syscall!
  Total syscalls for 10000 bytes: 1

Syscall cost: ~1-5 microseconds each
bufio.NewReaderSize(file, 64*1024) is almost always the right default.
```

### Memory-Mapped Files

```go
// For files too large even for streaming (TB-scale), use mmap
// mmap maps file pages directly into virtual address space
// OS page cache handles loading on demand (lazy)

import (
    "os"
    "syscall"
)

func mmapFile(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil { return nil, err }
    defer f.Close()

    fi, err := f.Stat()
    if err != nil { return nil, err }

    data, err := syscall.Mmap(
        int(f.Fd()),
        0,
        int(fi.Size()),
        syscall.PROT_READ,
        syscall.MAP_SHARED,
    )
    return data, err
}

// Access data[0] → OS loads the page containing byte 0 on demand
// Access data[1000000] → OS loads that page, only if needed
// Ideal for: search engines, databases, large config files
```

---

## 4.8 Go Generics (Go 1.18+)

```go
// Before generics: use interface{} (loses type safety) or code generation

// Generic function:
func Map[T, U any](slice []T, f func(T) U) []U {
    result := make([]U, len(slice))
    for i, v := range slice { result[i] = f(v) }
    return result
}

nums := []int{1, 2, 3, 4}
doubled := Map(nums, func(n int) int { return n * 2 })
// [2, 4, 6, 8]

strs := Map(nums, func(n int) string { return fmt.Sprint(n) })
// ["1", "2", "3", "4"]

// Type constraints:
type Number interface {
    int | int32 | int64 | float32 | float64
}

func Sum[T Number](nums []T) T {
    var total T
    for _, n := range nums { total += n }
    return total
}

Sum([]int{1, 2, 3})         // 6
Sum([]float64{1.1, 2.2})    // 3.3
```

---

## 4.9 Go Escape Analysis

Escape analysis determines whether a variable lives on the stack or heap. The compiler runs this at compile time.

```go
func stackOrHeap() *int {
    x := 42        // x might be on stack or heap
    return &x      // x's address is returned → x ESCAPES to heap
}

func noEscape() int {
    x := 42
    return x       // value returned, not address → x stays on STACK
}

// Check with: go build -gcflags='-m' ./...
// "x escapes to heap" vs "x does not escape"
```

Why it matters:
- Stack allocation: O(1) push/pop, cache-friendly, no GC pressure
- Heap allocation: requires GC, more expensive, but necessary for values that outlive their function

---

# PART 5: RUST LANGUAGE — COMPLETE DEEP DIVE

---

## 5.1 The Ownership System — The Core Mental Model

Rust's ownership system is the mechanism that enables memory safety without a garbage collector. Three rules govern everything:

```
OWNERSHIP RULES:
1. Every value has exactly ONE owner
2. When the owner goes out of scope, the value is dropped (freed)
3. There can be any number of immutable borrows OR exactly one mutable borrow,
   but never both at the same time

This is enforced at COMPILE TIME by the borrow checker.
No runtime checks. No garbage collection. Zero overhead.
```

### Move Semantics

```rust
let s1 = String::from("hello");
let s2 = s1;  // MOVE: s1's ownership transferred to s2

// s1 is now INVALID — the compiler knows this
// println!("{}", s1);  // COMPILE ERROR: value used after move

// Memory model:
// s1 on stack: { ptr: 0x1000, len: 5, cap: 5 }
// s2 on stack: { ptr: 0x1000, len: 5, cap: 5 }  ← same heap ptr!
// After move: s1 is logically erased by compiler
// When s2 goes out of scope: heap memory at 0x1000 is freed ONCE
// (no double-free!)

// Why move by default? For types that own heap memory.
// Integers, booleans, floats implement Copy → assignment clones, no move:
let x = 5;
let y = x;  // COPY, not move — both x and y are valid
```

### Clone vs Copy

```rust
// Copy: cheap, bitwise copy, both original and copy valid
// Implemented for: i32, f64, bool, char, &T, tuples of Copy types

// Clone: explicit deep copy, potentially expensive
let s1 = String::from("hello");
let s2 = s1.clone();  // heap data is COPIED
println!("{} {}", s1, s2);  // both valid

// String is NOT Copy because it owns heap memory
// i32 IS Copy because it's fixed-size, stack-only
```

---

## 5.2 Borrowing and References

Instead of moving, you can lend a reference:

```rust
// Immutable borrow: &T
fn calculate_length(s: &String) -> usize {
    s.len()  // s is a reference, doesn't own the data
}  // s goes out of scope, but the data is NOT freed (we don't own it)

let s = String::from("hello");
let len = calculate_length(&s);  // pass a reference
println!("{} has length {}", s, len);  // s still valid!

// Mutable borrow: &mut T
fn append(s: &mut String) {
    s.push_str(" world");
}
let mut s = String::from("hello");
append(&mut s);
println!("{}", s);  // "hello world"
```

### Borrow Rules — Why They Exist

```
RULE: At any time, you can have EITHER:
  - Any number of &T (immutable/shared references)
  - Exactly ONE &mut T (mutable/exclusive reference)

WHY? Prevents data races at compile time:
- Multiple readers: safe (reading doesn't cause conflicts)
- One writer: safe (exclusive access, no conflicts)
- Reader + writer: UNSAFE (writer might invalidate data reader sees)
                   → COMPILE ERROR in Rust!

// This is a compile-time equivalent of a read-write lock,
// but with ZERO runtime overhead.
```

```rust
let mut v = vec![1, 2, 3];

let r1 = &v;        // immutable borrow
let r2 = &v;        // another immutable borrow — fine!
println!("{:?} {:?}", r1, r2);  // r1 and r2 used here

// After r1, r2 last used, their borrows end (NLL: Non-Lexical Lifetimes)
// Now we can mutably borrow:
v.push(4);          // mutable borrow — fine, r1/r2 no longer active

// This would fail:
// let r = &v;
// v.push(4);       // ERROR: cannot mutably borrow while r is alive
// println!("{:?}", r); // r used here
```

---

## 5.3 Lifetimes

Lifetimes are labels that describe how long references are valid. They prevent dangling pointers at compile time.

```rust
// The borrow checker uses lifetimes to ensure references never outlive
// the data they point to.

// This function signature has implicit lifetime parameters:
fn longest(x: &str, y: &str) -> &str {
    if x.len() > y.len() { x } else { y }
}
// COMPILE ERROR: compiler doesn't know if return refers to x or y
//                and therefore doesn't know which lifetime to use

// EXPLICIT lifetime annotation:
fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {
    if x.len() > y.len() { x } else { y }
}
// 'a says: "the returned reference lives at least as long as
//          BOTH x and y" (the shorter of the two lifetimes)

// This prevents:
let result;
{
    let s2 = String::from("xyz");
    result = longest("long string", &s2);
    // s2 dropped here
}
println!("{}", result);  // COMPILE ERROR: s2 doesn't live long enough
```

### Lifetime Elision Rules

The compiler can infer lifetimes in many cases (you don't always need to annotate):

```rust
// Rule 1: each input reference gets its own lifetime
fn f(x: &i32) → &i32  →  fn f<'a>(x: &'a i32) -> &'a i32

// Rule 2: if there's exactly one input lifetime, output gets that lifetime
fn first_word(s: &str) -> &str  →  fn first_word<'a>(s: &'a str) -> &'a str

// Rule 3: if one of the inputs is &self, the output gets self's lifetime
// (applies to methods)

// The compiler applies these rules automatically.
// You only need explicit annotations when the rules don't uniquely determine
// the output lifetime.
```

### 'static Lifetime

```rust
// 'static means: valid for the entire program lifetime
// String literals are &'static str:
let s: &'static str = "hello, world"; // stored in program binary

// Heap data can be 'static if it's never freed:
let s: &'static str = Box::leak(Box::new(String::from("hello")));
// Box::leak intentionally leaks the memory, giving 'static reference
```

---

## 5.4 Rust Enums and Pattern Matching

Rust enums are algebraic data types (sum types) — each variant can hold different data.

```rust
// Standard library Option<T>:
enum Option<T> {
    Some(T),
    None,
}

// Standard library Result<T, E>:
enum Result<T, E> {
    Ok(T),
    Err(E),
}

// Custom enum:
enum Shape {
    Circle { radius: f64 },
    Rectangle { width: f64, height: f64 },
    Triangle(f64, f64, f64),  // three sides
}

// Memory layout:
// enum Shape is as large as its LARGEST variant + discriminant tag
// Circle    : [tag:1B][pad][radius:8B]   = 16B
// Rectangle : [tag:1B][pad][w:8B][h:8B] = 24B
// Triangle  : [tag:1B][pad][a:8B][b:8B][c:8B] = 32B
// sizeof(Shape) = 32B (max)
```

### Pattern Matching — Exhaustive

```rust
fn area(s: &Shape) -> f64 {
    match s {
        Shape::Circle { radius }            => std::f64::consts::PI * radius * radius,
        Shape::Rectangle { width, height }  => width * height,
        Shape::Triangle(a, b, c) => {
            let p = (a + b + c) / 2.0;
            (p * (p-a) * (p-b) * (p-c)).sqrt()
        }
    }
    // Compiler FORCES you to handle ALL variants — no forgotten cases!
}

// if let — handle one variant:
if let Some(val) = optional { println!("{}", val); }

// while let — loop while pattern matches:
while let Some(top) = stack.pop() { println!("{}", top); }

// Destructuring:
let (x, y, z) = (1, 2, 3);
let Point { x, y } = point;
let [first, .., last] = arr;
```

---

## 5.5 Smart Pointers

```rust
// Box<T> — heap allocation, single ownership
let b = Box::new(5);  // 5 lives on heap, b owns it
println!("{}", *b);   // dereference to get 5
// When b goes out of scope, heap memory freed automatically

// Use cases:
// - Recursive types (can't be stack-allocated, size unknown at compile time)
// - Large data you want on heap
// - Trait objects

// Recursive type REQUIRES Box:
enum List {
    Cons(i32, Box<List>),  // Box makes size known (pointer size)
    Nil,
}

// Rc<T> — Reference Counted, multiple ownership (single-threaded)
use std::rc::Rc;
let a = Rc::new(String::from("hello"));
let b = Rc::clone(&a);  // increment ref count (now = 2)
let c = Rc::clone(&a);  // ref count = 3
// When ALL of a,b,c dropped: ref count → 0 → memory freed
println!("{}", Rc::strong_count(&a)); // 3

// Arc<T> — Atomic Reference Counted (thread-safe version of Rc)
use std::sync::Arc;
let shared = Arc::new(Mutex::new(0));
let shared2 = Arc::clone(&shared);
thread::spawn(move || {
    let mut n = shared2.lock().unwrap();
    *n += 1;
});

// RefCell<T> — interior mutability (runtime borrow checking)
use std::cell::RefCell;
let x = RefCell::new(5);
*x.borrow_mut() += 1;  // runtime borrow check (panics if rules violated)
println!("{}", x.borrow()); // 6
```

### Smart Pointer Summary

```
Box<T>        - unique ownership, heap allocation
Rc<T>         - shared ownership, single-threaded, no mutation
Arc<T>        - shared ownership, multi-threaded, no mutation
RefCell<T>    - interior mutability, single-threaded, runtime check
Mutex<T>      - interior mutability, multi-threaded, blocking
RwLock<T>     - interior mutability, multi-threaded, multiple readers
Cell<T>       - interior mutability, Copy types only, no borrow overhead
Weak<T>       - non-owning reference (with Rc/Arc, breaks cycles)
```

---

## 5.6 Traits — Rust's Type System

Traits are Rust's interfaces. They define shared behavior.

```rust
// Trait definition:
trait Summary {
    fn summarize(&self) -> String;

    // Default implementation:
    fn preview(&self) -> String {
        format!("{}...", &self.summarize()[..50])
    }
}

// Implementation for a type:
struct Article { title: String, body: String }
impl Summary for Article {
    fn summarize(&self) -> String {
        format!("{}: {}", self.title, &self.body[..100])
    }
}

// Trait bounds — constrain generic types:
fn notify<T: Summary>(item: &T) {
    println!("{}", item.summarize());
}

// Multiple bounds:
fn print_and_notify<T: Summary + std::fmt::Display>(item: &T) { ... }

// Where clauses (for complex bounds):
fn complex<T, U>(t: &T, u: &U) where
    T: Summary + Clone,
    U: std::fmt::Display + std::fmt::Debug,
{ ... }

// impl Trait syntax (return position — opaque type):
fn make_summary() -> impl Summary {
    Article { title: "...".into(), body: "...".into() }
}
// Caller knows return type implements Summary, not the concrete type

// dyn Trait — trait objects (dynamic dispatch):
fn notify_dyn(item: &dyn Summary) {
    println!("{}", item.summarize());
}
// Box<dyn Summary> — heap-allocated trait object
let items: Vec<Box<dyn Summary>> = vec![
    Box::new(Article { ... }),
    Box::new(Tweet { ... }),
];
```

### Static vs Dynamic Dispatch

```
impl Trait / generics with bounds → STATIC DISPATCH (monomorphization)
  - Compiler generates separate code for each concrete type
  - Zero runtime overhead (like C++ templates)
  - Code size grows

dyn Trait → DYNAMIC DISPATCH (vtable)
  - Single function body, dispatches via vtable at runtime
  - ~1-2 ns overhead per call (pointer indirection)
  - Code size stays small
  - Required when you don't know the type at compile time

vtable (virtual function table):
+-------------------+
| pointer to type   |
| drop function ptr |
| size              |
| align             |
| method1 ptr       |
| method2 ptr       |
+-------------------+
&dyn Trait is a fat pointer: [data_ptr | vtable_ptr] (16 bytes)
```

---

## 5.7 Closures and Iterators

```rust
// Closure: anonymous function that captures its environment
let multiplier = 3;
let multiply = |x| x * multiplier;  // captures multiplier
println!("{}", multiply(5)); // 15

// Three closure traits (how they capture):
// Fn:     borrows immutably (can be called multiple times)
// FnMut:  borrows mutably  (can be called multiple times, mutates state)
// FnOnce: takes ownership  (can only be called ONCE)

let mut count = 0;
let mut increment = || { count += 1; count };  // FnMut — mutates count
println!("{}", increment());  // 1
println!("{}", increment());  // 2

// Iterators — lazy, zero-cost
let v = vec![1, 2, 3, 4, 5];

// These don't execute until consumed:
let doubled = v.iter().map(|x| x * 2);       // lazy
let evens   = v.iter().filter(|&&x| x % 2 == 0);

// Consuming adaptors (call next() internally):
let sum: i32 = v.iter().sum();                // 15
let doubled_vec: Vec<i32> = v.iter()
    .filter(|&&x| x > 2)
    .map(|x| x * 2)
    .collect();                               // [6, 8, 10]

// Chaining is zero-cost: compiler inlines everything into a single loop
// This is as fast as handwritten imperative code:
let result: i32 = (0..1_000_000)
    .filter(|x| x % 2 == 0)
    .map(|x| x * x)
    .sum();
```

---

## 5.8 Unsafe Rust

Unsafe Rust allows you to opt into behaviors the compiler cannot verify. It's an escape hatch, not a failure.

```rust
// unsafe block grants 5 superpowers:
// 1. Dereference raw pointers
// 2. Call unsafe functions
// 3. Access/modify mutable statics
// 4. Implement unsafe traits
// 5. Access fields of unions

let mut num = 5;

// Raw pointers:
let r1 = &num as *const i32;      // immutable raw pointer
let r2 = &mut num as *mut i32;    // mutable raw pointer

unsafe {
    println!("{}", *r1);  // dereference raw pointer
    *r2 = 10;
}

// Calling C from Rust:
extern "C" {
    fn abs(input: i32) -> i32;  // declare C function
}

unsafe {
    println!("{}", abs(-3));  // call C function
}

// Raw pointer arithmetic (like C):
let arr = [1i32, 2, 3, 4, 5];
let p = arr.as_ptr();
unsafe {
    println!("{}", *p.add(2));  // 3 — ptr + 2 * sizeof(i32)
}
```

### Safe Abstraction Over Unsafe

The key pattern: use unsafe internally, expose a safe public API:

```rust
// std::Vec::split_at_mut is implemented with unsafe but is safe to call:
// (You can't implement this with safe Rust because borrowing two mut slices
//  from the same vec would violate borrow rules — but they don't overlap)

fn split_at_mut(slice: &mut [i32], mid: usize) -> (&mut [i32], &mut [i32]) {
    let len = slice.len();
    let ptr = slice.as_mut_ptr();
    assert!(mid <= len);
    unsafe {
        (
            std::slice::from_raw_parts_mut(ptr, mid),
            std::slice::from_raw_parts_mut(ptr.add(mid), len - mid),
        )
    }
}
```

---

## 5.9 Rust Concurrency

Rust's ownership rules make data races a compile-time error:

```rust
use std::thread;
use std::sync::{Arc, Mutex};

// Shared state with Arc<Mutex<T>>:
let counter = Arc::new(Mutex::new(0));
let mut handles = vec![];

for _ in 0..10 {
    let c = Arc::clone(&counter);
    let handle = thread::spawn(move || {
        let mut n = c.lock().unwrap();  // acquire mutex
        *n += 1;
    });                                  // mutex released when n drops
    handles.push(handle);
}

for h in handles { h.join().unwrap(); }
println!("{}", *counter.lock().unwrap());  // 10

// Send + Sync traits:
// Send: type can be transferred to another thread
// Sync: type can be shared between threads (&T is Send)
// Arc<T> is Send + Sync when T is Send + Sync
// Rc<T> is NOT Send (not thread-safe reference count)
// Mutex<T> is Send + Sync when T is Send
// RefCell<T> is NOT Sync (runtime borrow check not thread-safe)
```

---

# PART 6: OPERATING SYSTEMS AND SYSTEMS CONCEPTS

---

## 6.1 Processes vs Threads

```
PROCESS:
- Independent program in execution
- Has its own address space (virtual memory)
- Has its own file descriptors, signal handlers, etc.
- Creating is expensive (fork + exec)
- Isolated — crash in one doesn't affect others

THREAD:
- Unit of execution WITHIN a process
- Shares address space, heap, file descriptors with sibling threads
- Has its own: stack, registers, program counter
- Creating is cheaper than a process
- Crash in one thread can crash the whole process

Process memory:
+------------------+
| CODE / TEXT      |  shared between threads
+------------------+
| DATA / BSS       |  shared between threads
+------------------+
| HEAP             |  shared between threads (→ needs locks)
+------------------+
| [thread 1 stack] |  private to thread 1
+------------------+
| [thread 2 stack] |  private to thread 2
+------------------+
| [thread 3 stack] |  private to thread 3
+------------------+
```

### Context Switch

```
When OS decides to switch from Thread A to Thread B:

1. SAVE Thread A's CPU state:
   - Save registers (rax, rbx, rsp, rbp, ...) into A's TCB (Thread Control Block)
   - Save program counter (rip) → where to resume
   - Switch from user mode to kernel mode (ring 3 → ring 0)

2. SWITCH page tables (if switching processes, not just threads):
   - Update CR3 register (x86) to point to new process's page table
   - Flush TLB (Translation Lookaside Buffer) — expensive!

3. RESTORE Thread B's CPU state:
   - Load registers from B's TCB
   - Load B's program counter

4. Resume execution in Thread B

Cost: ~1-5 microseconds (TLB flush dominates for process switch)
      ~0.1-1 microsecond for same-process thread switch (no TLB flush)
```

---

## 6.2 Synchronization Primitives

### Mutex (Mutual Exclusion Lock)

```
A mutex has two states: LOCKED / UNLOCKED

Thread A: lock(m) → enters critical section
Thread B: lock(m) → BLOCKS (spins or sleeps until m is unlocked)

Thread A: unlock(m) → m goes to UNLOCKED
OS wakes Thread B: lock(m) → enters critical section

SPINLOCK vs MUTEX:
Spinlock: Thread B busy-waits ("spin"):
  while (atomic_test_and_set(&lock)) { /* spin */ }
  - Fast for short waits (no context switch)
  - Wastes CPU for long waits

Mutex: Thread B sleeps (kernel puts it in wait queue):
  - No wasted CPU
  - Context switch cost if lock is acquired quickly
```

### Semaphore

```
Semaphore S has an integer value:
- wait(S) (P): S--, if S < 0 → block
- signal(S) (V): S++, if S ≤ 0 → wake one waiter

Binary semaphore: S ∈ {0,1} — equivalent to mutex
Counting semaphore: S ∈ {0..N} — limit concurrent access

Example: database connection pool of 5:
S = 5
Thread 1: wait(S) → S=4, gets connection
Thread 2: wait(S) → S=3, gets connection
...
Thread 5: wait(S) → S=0, gets connection
Thread 6: wait(S) → S=-1, BLOCKS
Thread 1 done: signal(S) → S=0, wakes Thread 6
Thread 6: gets connection
```

### Condition Variable

```
Used to wait for a condition, not just a lock:

Producer-Consumer with condition variable:
mutex m; condvar not_empty, not_full;

PRODUCER:
lock(m)
while (queue_full) wait(not_full, m)  // atomically unlock m and sleep
enqueue(item)
signal(not_empty)
unlock(m)

CONSUMER:
lock(m)
while (queue_empty) wait(not_empty, m) // atomically unlock m and sleep
item = dequeue()
signal(not_full)
unlock(m)
```

### Read-Write Lock

```
Problem: many readers, few writers
Mutex solution: only ONE thread at a time (readers unnecessarily serialized)

RWLock solution:
- Multiple concurrent readers allowed
- Writers get exclusive access

lock_read(rwl):  if no writer → allow, increment reader_count
unlock_read(rwl): decrement reader_count, if 0 → signal waiting writers
lock_write(rwl): wait until reader_count=0 AND no other writer
unlock_write(rwl): signal waiting readers/writers
```

---

## 6.3 Virtual Memory and Paging

```
VIRTUAL ADDRESS SPACE (per process):
Every process thinks it has the entire address space (e.g., 0 to 2^64-1)
Reality: mapped to PHYSICAL memory via page tables

PAGE: fixed-size block (typically 4KB)
      virtual address = [VPN | page offset]
      physical address = [PPN | page offset]

Page Table (per process):
VPN 0: PPN 42 (present)
VPN 1: PPN 17 (present)
VPN 2: (not present) → PAGE FAULT → OS loads from disk
VPN 3: PPN 99 (present, read-only)

Virtual Address Translation:
virtual address 0x3000 + offset 0x200 = 0x3200
VPN = 0x3200 / 4096 = 0 (if page size = 4096)
Look up VPN 0 in page table → PPN 42
Physical address = 42 * 4096 + 0x200 = 0x2A200

TLB (Translation Lookaside Buffer):
Hardware cache of recent VPN→PPN translations
Hit:  ~1 cycle (just look up)
Miss: ~100 cycles (walk page table in memory)
```

---

## 6.4 Syscalls — How User Code Talks to the Kernel

```
USER SPACE          KERNEL SPACE
                   |
Application Code   |   Kernel
                   |
   libc wrapper    |   sys_open
     open()    ─ syscall ─►  (actually opens file)
                   |
              kernel returns result
                   |
   returns fd  ◄──────────
                   |

SYSCALL MECHANISM on x86-64:
1. Application puts syscall number in RAX (e.g., 2 for open)
2. Arguments go in RDI, RSI, RDX, R10, R8, R9
3. Execute "syscall" instruction (or INT 0x80 on 32-bit)
4. CPU switches to ring 0 (kernel mode)
5. Kernel handler runs
6. Result placed in RAX
7. CPU returns to ring 3 (user mode)

Cost: ~100-300 ns per syscall (mode switch overhead)
That's why:
- read() with 1-byte buffer is slow (many syscalls)
- read() with 64KB buffer is fast (few syscalls)
- mmap avoids syscall per access entirely
```

---

# PART 7: COMPLETE INTERVIEW Q&A

---

## All 100 C Interview Questions — Answered In Depth

### Memory, Pointers, and Lifetime

**Q1. What is the difference between stack memory and heap memory?**

Stack is automatically managed memory allocated in LIFO order at function call time. It's faster (just decrement the stack pointer), bounded in size (typically 8 MB), and automatically freed when a function returns. Heap is manually managed (or GC-managed), can grow to system memory limits, requires explicit allocation/deallocation, and survives beyond function calls.

**Q2. What happens when a function returns the address of a local variable?**

Undefined behavior. The local variable's stack frame is destroyed on return. The pointer becomes dangling — it points to memory the program no longer owns. It may appear to work because the bytes haven't been overwritten yet, but any subsequent function call can clobber them.

**Q3. What is undefined behavior in C?**

Behavior that the C standard does not define. The compiler is allowed to assume UB never happens and optimize accordingly. Results can range from appearing correct, to producing garbage, to security vulnerabilities, to different results at different optimization levels.

**Q4. What is a dangling pointer?**

A pointer that was once valid but the memory it points to has been freed or has gone out of scope. Dereferencing it is undefined behavior.

**Q5. What is a null pointer?**

A pointer holding the address 0 (or the platform's null address). `NULL` in C is typically `(void*)0`. Dereferencing NULL is undefined behavior (usually segfault). `free(NULL)` is defined and safe.

**Q6. What is a wild pointer?**

An uninitialized pointer — it holds whatever garbage value was in its memory location. Dereferencing it reads/writes a random memory address — extremely dangerous.

**Q7. What is the difference between `NULL` and `0` in pointer context?**

Semantically: `NULL` means "no object pointed to," `0` is the integer zero. In practice on most platforms they're the same bit pattern. `NULL` communicates intent more clearly. C11's `nullptr_t` and C++'s `nullptr` are even more type-safe. Using `NULL` instead of `0` for pointers is a style requirement to express meaning.

**Q8. What is the difference between `malloc`, `calloc`, `realloc`, and `free`?**

- `malloc(n)`: allocate n bytes, uninitialized (contains garbage)
- `calloc(n, size)`: allocate n*size bytes, zero-initialized (safer for arrays)
- `realloc(ptr, new_size)`: resize allocation — may allocate new region and copy data, old ptr may become invalid, returns NULL on failure (you must use the returned pointer)
- `free(ptr)`: return memory to allocator. `free(NULL)` is safe (no-op).

**Q9. What happens if you call `free()` twice on the same pointer?**

Double-free: undefined behavior. It corrupts the allocator's free list metadata, possibly creating cycles or overwriting neighboring data. Can lead to arbitrary code execution (classic heap exploit). Always `ptr = NULL` after `free`.

**Q10. What happens if you forget to `free()` allocated memory?**

Memory leak. The program's memory usage grows over time. For short-lived programs it's often harmless (OS reclaims all memory on exit). For long-lived servers it eventually causes OOM (Out of Memory), crash, or forced restart.

**Q11–Q20 (Arrays and Pointers):**

**Q11. Array vs pointer?** Arrays are fixed-size, their name is a compile-time label for a memory region. Pointers are variables holding addresses, can be reassigned, no inherent size info.

**Q12. Array decay?** In most expressions (passed to functions, arithmetic, assignment), an array name evaluates to the address of its first element. Exceptions: `sizeof`, `&`, `_Alignof`.

**Q13. `sizeof(arr)` vs `sizeof(ptr)`?** `sizeof(arr)` gives total array size in bytes (n * element_size). `sizeof(ptr)` gives pointer size: 4 bytes on 32-bit, 8 bytes on 64-bit.

**Q14. Pointer arithmetic?** `p + n` moves the pointer forward by `n * sizeof(*p)` bytes. Allows treating pointers like array iterators.

**Q15. `arr[i]` == `*(arr+i)`?** Yes, by definition. The subscript operator is syntactic sugar for pointer arithmetic plus dereference.

**Q16. `char *s` vs `char s[]`?** `char *s` is a pointer (variable holding address, typically points to `.rodata`). `char s[]` is an array (contiguous storage, independent copy). `sizeof(*s)=1` vs `sizeof(s)=n`. Cannot assign to `s` (array), can assign to pointer.

**Q17. Can you assign arrays in C?** No. `arr2 = arr1` is a compile error. You must `memcpy`. This is why function parameters receiving arrays are always pointers.

**Q18. Pointer to an array?** `int (*p)[5] = &arr` — p points to the whole array, not just arr[0]. `p+1` advances by `5*sizeof(int)`. Used for 2D array parameters.

**Q19. Array of pointers?** `int *p[5]` — five separate pointers. `char *strs[] = {"one","two","three"}` is array of string pointers.

**Q20. Multi-dimensional arrays in memory?** Row-major order: elements are contiguous row by row. `arr[i][j]` = `*(arr + i*cols + j)`. The "inner" index is the "fast" dimension (incrementing j is sequential in memory).

**Q21–Q30 (Strings):**

**Q21–Q30** covered in depth in Section 2.2 above.

**Q31–Q40 (Functions):**

**Q31. Argument passing in C?** Pass-by-value: function receives a COPY of each argument. Modifying the parameter does not affect the caller's variable.

**Q32. Why is C pass-by-value?** Design choice for simplicity and local reasoning. The function cannot accidentally modify caller's data through regular parameters.

**Q33. Simulate pass-by-reference?** Pass a pointer. `void swap(int *a, int *b)` — caller passes `&x, &y`. Function modifies `*a` and `*b`.

**Q34. Declaration vs definition?** Declaration tells compiler the function's name and types (no body). Definition provides the body. You can declare in .h, define in .c.

**Q35. Function prototype?** A function declaration. `int foo(int, char*);` lets the compiler type-check calls to `foo` before its definition is seen.

**Q36. Recursion and the stack?** Each recursive call pushes a new stack frame. Deep recursion can cause stack overflow. Tail call optimization (when supported) can reuse the same frame.

**Q37–Q39. Function pointers and callbacks?** Covered in Section 2.1. A callback is passing a function pointer to another function so it can call your function at the appropriate time (event handlers, comparators for sorting, etc.).

**Q40. Return multiple values?** Not directly. Options: return a struct, use out-parameters (pointers), global state (don't). Returning a struct is cleanest.

**Q41–Q50 (Structs, Unions, Enums):** Covered in Section 2.4.

**Q51–Q60 (Dynamic Memory):** Covered in Sections 2.3 and memory leak patterns above.

**Q61–Q70 (Preprocessor):** Covered in Section 2.7.

**Q71–Q80 (Type System):** Covered in Sections 2.5 and 2.6.

**Q81–Q90 (Bits and Bytes):** Covered in Section 2.9.

**Q91–Q100 (Output Prediction):** Covered in Section 2.10.

---

## 100 Go Interview Questions — Key Concepts

**Q1. What are goroutines and how do they differ from OS threads?**

Goroutines are user-space threads managed by the Go runtime. They start with 2KB stacks (vs 8MB for OS threads), are multiplexed onto OS threads by the GMP scheduler, and can number in the millions. Context switching between goroutines is ~100ns vs ~1µs for OS threads.

**Q2. What is a channel and when would you use it?**

A channel is a typed conduit for goroutine communication. Use it to pass data between goroutines, signal events, or control concurrency. "Share memory by communicating, don't communicate by sharing memory."

**Q3. What is the difference between buffered and unbuffered channels?**

Unbuffered: sender blocks until receiver is ready — guaranteed synchronization. Buffered: sender blocks only when buffer is full; acts as a bounded queue.

**Q4. What is `select` in Go?**

A multi-way channel wait. Like a `switch` for channel operations. If multiple cases are ready, one is chosen at random (fair). Use `default` for non-blocking operations.

**Q5. What is `defer` and what is its execution order?**

`defer` schedules a function call to run when the surrounding function returns (including panic). Multiple defers execute in LIFO order.

**Q6. What is a Go interface?**

A set of method signatures. Any type that implements all methods implicitly satisfies the interface (structural typing / duck typing). An interface value is a fat pointer: (type info, data pointer).

**Q7. What is a nil interface vs a nil pointer inside an interface?**

A nil interface has both pointer fields as nil. A non-nil interface holding a nil pointer has type info set but data pointer nil. `interface{}((*int)(nil)) != nil` — this is a common trap!

```go
var p *int = nil
var i interface{} = p
fmt.Println(i == nil)  // FALSE! Interface has type info for *int
```

**Q8. How does garbage collection work in Go?**

Tri-color mark-and-sweep with concurrent marking. Go's GC aims for sub-millisecond STW (Stop-The-World) pauses. It runs concurrently with program execution. Uses write barriers during concurrent marking.

**Q9. What is a goroutine leak?**

A goroutine that is started but never terminates. Goroutines are not garbage collected if they're stuck on a channel operation or long-running loop. Common in server code with improperly managed goroutine lifetimes. Use `context.Context` for cancellation.

**Q10. What is the `sync.WaitGroup`?**

A counter for waiting for a set of goroutines to complete:
```go
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() { defer wg.Done(); doWork() }()
}
wg.Wait()  // blocks until all 10 goroutines call Done()
```

**Q11. What is `context.Context` used for?**

Carries deadlines, cancellation signals, and request-scoped values across goroutines and API boundaries. Used to cancel in-flight operations when a request times out or client disconnects.

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
result, err := slowOperation(ctx)
```

**Q12. What is a race condition and how do you detect it in Go?**

When two goroutines access shared memory with at least one write, without synchronization. Detected using the race detector: `go run -race main.go` or `go test -race ./...`. Uses dynamic analysis (TSan under the hood).

**Q13. What is `sync.Once`?**

Ensures a function runs exactly once, even if called from multiple goroutines concurrently. Common for lazy initialization of expensive resources.

**Q14. What is `sync/atomic`?**

Package providing lock-free atomic operations on integers and pointers. `atomic.AddInt64`, `atomic.CompareAndSwapInt32`, etc. Faster than mutex for simple counters.

**Q15. What does `go vet` do?**

Static analysis of Go code. Checks for common mistakes: unreachable code, wrong format verbs, copy of sync types (mutex passed by value), suspicious code constructs.

**Q16. What are Go generics and when would you use them?**

Type parameters for functions and types (Go 1.18+). Use when you need the same algorithm for different types without sacrificing type safety. `func Map[T, U any](s []T, f func(T)U) []U`.

**Q17. What is escape analysis in Go?**

Compiler analysis determining if a variable escapes its function's scope (goes to heap) or stays on the stack. Heap allocation triggers GC; stack allocation is cheaper. Use `go build -gcflags='-m'` to inspect.

**Q18. How do you read a large file efficiently in Go?**

Use `bufio.NewReader` or `bufio.Scanner` for text, raw `file.Read()` with a reusable buffer for binary, `syscall.Mmap` for random access on very large files. Never `os.ReadFile` for large files.

**Q19. What is a goroutine's initial stack size and how does it grow?**

2KB initial. Grows by copying the entire stack to a new, larger allocation (typically doubled). Max default 1GB. No stack overflow in Go — panics only if max exceeded.

**Q20. What is the GMP model?**

G=Goroutine, M=Machine(OS thread), P=Processor(scheduling context). `GOMAXPROCS` Ps run concurrently. Each P has a local run queue. Work stealing balances load. Goroutines parked during syscalls release their P so other goroutines can run.

---

## 100 Rust Interview Questions — Key Concepts

**Q1. What is ownership in Rust?**

Every value has exactly one owner. When the owner goes out of scope, the value is dropped. This is enforced at compile time — enables memory safety without a GC.

**Q2. What is borrowing?**

Lending a reference to a value without transferring ownership. Immutable borrows (`&T`) allow multiple readers. Mutable borrows (`&mut T`) allow one writer. Both enforced at compile time by the borrow checker.

**Q3. What is the difference between `&T` and `&mut T`?**

`&T` is a shared, immutable reference. Multiple can coexist. `&mut T` is an exclusive, mutable reference. No other references (shared or mutable) can exist while a `&mut T` is alive.

**Q4. What is a lifetime?**

A label describing how long a reference is valid. The borrow checker uses lifetimes to prove references never outlive the data they point to. Explicit annotation needed when the compiler can't infer them.

**Q5. What is the difference between `String` and `&str`?**

`String` is a heap-allocated, growable, owned string. `&str` is a borrowed reference to a string slice (can point to a `String`, a literal, or any byte slice). Functions should usually take `&str` (more general).

**Q6. What does `clone()` do?**

Creates a deep copy of a value. For `String`, it copies the heap allocation. Explicit because it's potentially expensive. Types implementing `Copy` don't need clone — they're copied automatically.

**Q7. What is `Option<T>`?**

Rust's null safety mechanism. `Some(T)` for a value, `None` for absence. Forces you to handle both cases. Eliminates null pointer exceptions.

**Q8. What is `Result<T, E>`?**

Rust's error handling type. `Ok(T)` for success, `Err(E)` for failure. The `?` operator propagates errors up the call stack.

**Q9. What does the `?` operator do?**

In a function returning `Result`, `?` returns the `Err` variant early if the result is an error. Syntactic sugar for `match result { Ok(v) => v, Err(e) => return Err(e.into()) }`.

**Q10. What is a trait?**

A collection of method signatures defining shared behavior. Similar to interfaces in other languages. Types opt into traits with `impl TraitName for TypeName`.

**Q11. What is monomorphization?**

Rust's process of generating separate code for each concrete type used with a generic function. `fn foo<T>` with `T=i32` and `T=String` generates two separate `foo_i32` and `foo_String` functions. Zero runtime overhead but potentially larger binary.

**Q12. What is `dyn Trait`?**

A trait object. Enables dynamic dispatch via vtable. The concrete type isn't known at compile time. Used when you need a collection of different types implementing the same trait, or when the type can only be known at runtime.

**Q13. What is `Box<T>`?**

A smart pointer for heap allocation with single ownership. The heap memory is freed when `Box` is dropped. Used for: recursive types, large data on heap, trait objects.

**Q14. What is `Rc<T>` and when would you use it?**

Reference-counted smart pointer for shared ownership within a single thread. Use when you need multiple owners and can't determine which owner will be the last. For thread-safe sharing, use `Arc<T>`.

**Q15. What is interior mutability?**

A pattern allowing mutation of data through immutable references. Implemented via `Cell<T>`, `RefCell<T>` (single-threaded), `Mutex<T>`, `RwLock<T>` (multi-threaded). Moves the borrow checking to runtime.

**Q16. What is the difference between `panic!` and returning `Result`?**

`panic!` aborts the current thread (by default) — for unrecoverable errors (bug in the code). Returning `Result` allows the caller to handle the error — for expected failure conditions. Library code should prefer `Result`.

**Q17. What are closures in Rust?**

Anonymous functions that can capture their environment. Implement `Fn`, `FnMut`, or `FnOnce` depending on how they capture. Used heavily with iterator adapters.

**Q18. What is the iterator pattern in Rust?**

`Iterator` trait with `next() -> Option<T>` method. Adapters like `map`, `filter`, `fold` are lazy — no computation until consumed. Zero-cost: compiler inlines iterator chains into tight loops.

**Q19. What does `unsafe` mean in Rust?**

An explicit opt-in to 5 unsafe operations that the compiler cannot verify. Doesn't turn off all safety checks — just those 5. Good practice: wrap unsafe in safe public APIs, minimize unsafe scope.

**Q20. What is the `Send` trait?**

A marker trait indicating a type can be transferred to another thread. Auto-implemented for most types. `Rc<T>` is NOT `Send` (reference count not atomic). `Arc<T>` IS `Send`.

**Q21. What is `Sync`?**

A marker trait indicating `&T` can be shared between threads. `i32` is `Sync`. `Cell<T>` is NOT `Sync` (interior mutability without thread safety). `Mutex<T>` IS `Sync`.

**Q22. What is `mem::replace` and when would you use it?**

Replaces a value at a mutable reference, returning the old value, without cloning. Used when you need to take ownership from behind a mutable reference.

**Q23. What is the difference between `iter()`, `iter_mut()`, and `into_iter()`?**

- `iter()`: iterates over `&T` (immutable references)
- `iter_mut()`: iterates over `&mut T` (mutable references)  
- `into_iter()`: consumes the collection, iterates over `T` (owned values)

**Q24. What is a lifetime elision?**

Rules the compiler uses to infer lifetimes without explicit annotation. The three rules cover most cases (function parameters, single input, method self). Reduces boilerplate.

**Q25. What is `std::mem::size_of`?**

Returns the size of a type in bytes at compile time. Useful for FFI, memory layout inspection, and understanding Rust's type system internals.

---

# PART 8: CONCURRENCY PATTERNS AND LOCK-FREE PROGRAMMING

---

## 8.1 Atomics and Memory Ordering

Modern CPUs and compilers reorder instructions for performance. Atomics provide:
1. Atomicity: read-modify-write as a single uninterruptible operation
2. Memory ordering: visibility guarantees across CPUs

```
MEMORY ORDERING OPTIONS (from weakest to strongest):

Relaxed:   Only guarantees atomicity. No ordering. Fastest.
           Use for: counters where exact order doesn't matter.

Acquire:   After this load, all subsequent reads see effects
           from a matching Release. (Like entering a lock)

Release:   Before this store, all prior writes are visible
           to a thread doing an Acquire on the same value. (Like leaving a lock)

AcqRel:    Combines Acquire and Release. For read-modify-write.

SeqCst:    Strongest: total sequential ordering across all threads.
           Slowest. Default for most atomics.
```

```c
// C11 atomics:
#include <stdatomic.h>
atomic_int counter = ATOMIC_VAR_INIT(0);
atomic_fetch_add_explicit(&counter, 1, memory_order_relaxed);

// Compare-and-swap (CAS): the foundation of lock-free algorithms
int expected = 0;
int desired  = 1;
bool success = atomic_compare_exchange_strong(&lock, &expected, desired);
// Atomically: if (*lock == expected) { *lock = desired; return true; }
//             else { expected = *lock; return false; }
```

```go
// Go atomic operations:
import "sync/atomic"

var counter int64
atomic.AddInt64(&counter, 1)
val := atomic.LoadInt64(&counter)
atomic.StoreInt64(&counter, 42)
atomic.CompareAndSwapInt64(&counter, 0, 1)  // CAS
```

```rust
// Rust atomics:
use std::sync::atomic::{AtomicI64, Ordering};

static COUNTER: AtomicI64 = AtomicI64::new(0);
COUNTER.fetch_add(1, Ordering::Relaxed);
let val = COUNTER.load(Ordering::SeqCst);
COUNTER.compare_exchange(0, 1, Ordering::AcqRel, Ordering::Relaxed);
```

---

## 8.2 Lock-Free Data Structures

### Lock-Free Stack (Treiber Stack)

```
Lock-free stack uses CAS to atomically update the head pointer.

Stack state: top → [3] → [2] → [1] → NULL

PUSH(4) by Thread A:
1. new_node.next = top  (top = &node3)
2. CAS(top, &node3, &new_node)  // if top is still &node3, set to &new_node
   If CAS fails (another thread changed top) → retry from step 1

After push: top → [4] → [3] → [2] → [1] → NULL

POP by Thread B:
1. head = top          (head = &node4)
2. new_head = head.next  (new_head = &node3)
3. CAS(top, head, new_head)  // if top is still head, set to new_head
   If CAS fails → retry

Correctness: CAS succeeds only if the world hasn't changed since we read it.
             If it fails, we retry with the new state. → wait-free progress
```

---

## 8.3 The ABA Problem

```
A CAS vulnerability:

State: top → A → B → C

Thread 1: wants to pop A
  reads top = &A
  [Thread 1 paused]

Thread 2: pops A, pops B, pushes A back (reused node)
  State: top → A → C  (same address for A, different next!)

Thread 1 resumes:
  CAS(top, &A, &B) → SUCCEEDS! top was &A, so CAS works
  But now top → B (dangling pointer!), C is lost

SOLUTIONS:
1. Tagged pointers: pair pointer with a version counter
   (top, version) — CAS both atomically
   Thread 2 increments version → Thread 1's CAS fails (version mismatch)

2. Hazard pointers: mark nodes as "in use" before accessing
3. Epoch-based reclamation: delay freeing until all threads pass an epoch
4. In Rust: ownership system makes ABA hard to trigger (no arbitrary reuse)
```

---

# PART 9: NETWORKING AND I/O MODELS

---

## 9.1 I/O Models

```
BLOCKING I/O (default):
  Application   System call   Kernel
      |    read(fd)   |           |
      |──────────────►|           |
      |               |  wait for |
      | (BLOCKED)     |    data   |
      |               |           |
      |               |◄──────────|
      |◄──────────────|  return   |
  Resume when data available
  Simple, but one thread can serve only one connection

NON-BLOCKING I/O:
  fd = socket(); fcntl(fd, F_SETFL, O_NONBLOCK);
  read(fd) → returns EAGAIN if no data (immediately)
  Application polls: keep calling read() until data arrives
  Wastes CPU (busy polling)

I/O MULTIPLEXING (select/poll/epoll):
  Tell kernel: "wake me when ANY of these fds are ready"

  select(fds, timeout)  ← O(n) scan of fds, max ~1024 fds
  poll(fds, timeout)    ← O(n) scan, no fd limit
  epoll(Linux)          ← O(1), event-driven, scales to millions of fds

ASYNC I/O (aio, io_uring):
  Submit request to kernel, continue executing
  Kernel notifies when complete (via callback, signal, or completion queue)
  io_uring (Linux 5.1+): shared ring buffers, batch submissions, lowest latency
```

### epoll Model (Foundation of Go's netpoller, Nginx, Node.js)

```
Setup:
  epoll_fd = epoll_create()

Register interest:
  epoll_ctl(epoll_fd, EPOLL_CTL_ADD, client_fd, EPOLLIN | EPOLLET)
  // Edge-triggered (ET): only notified on new data arrival
  // Level-triggered (LT): notified as long as data available

Wait for events:
  n = epoll_wait(epoll_fd, events, MAX_EVENTS, timeout_ms)

Handle events:
  for i in 0..n:
      fd = events[i].data.fd
      if events[i].events & EPOLLIN:
          read from fd

This pattern allows ONE thread to manage THOUSANDS of connections.
Go's runtime uses epoll (Linux) / kqueue (macOS) internally for its netpoller.
goroutine blocks on network I/O → Go runtime registers fd with epoll →
goroutine is parked (not blocking an OS thread) → when fd becomes readable,
runtime wakes the goroutine.
```

---

# PART 10: ALGORITHMS — COMPLEXITY AND DYNAMIC PROGRAMMING

---

## 10.1 Complexity Analysis

```
Big-O Notation:

O(1)       — constant: array access, hash table lookup (average)
O(log n)   — logarithmic: binary search, BST ops (balanced), heap ops
O(n)       — linear: array scan, single pass, linked list traversal
O(n log n) — linearithmic: efficient sorting (merge, heap, quick avg)
O(n²)      — quadratic: bubble/selection/insertion sort, nested loops
O(n³)      — cubic: naive matrix multiplication, some DP
O(2ⁿ)      — exponential: recursive Fibonacci (naive), subset enumeration
O(n!)      — factorial: permutations, brute-force TSP

GROWTH COMPARISON at n=1000:
O(1)       = 1 op
O(log n)   = 10 ops
O(n)       = 1,000 ops
O(n log n) = 10,000 ops
O(n²)      = 1,000,000 ops
O(2ⁿ)      = 10^301 ops (impossible!)
```

## 10.2 Dynamic Programming — The Mental Model

DP solves problems by breaking them into overlapping subproblems and storing results.

Two conditions for DP:
1. **Optimal substructure**: optimal solution contains optimal subsolutions
2. **Overlapping subproblems**: same subproblems solved multiple times in recursion

### Fibonacci — The Classic DP Example

```
Naive recursion: fib(5)
           fib(5)
          /       \
      fib(4)      fib(3)
      /    \      /    \
   fib(3) fib(2) fib(2) fib(1)
   /  \
fib(2) fib(1)

fib(2) computed 3 times! O(2^n) time.

MEMOIZATION (top-down DP): cache results
memo = {}
fib(n):
    if n in memo: return memo[n]
    if n <= 1:   return n
    memo[n] = fib(n-1) + fib(n-2)
    return memo[n]
O(n) time, O(n) space

TABULATION (bottom-up DP): fill table iteratively
dp[0] = 0, dp[1] = 1
for i in 2..n:
    dp[i] = dp[i-1] + dp[i-2]
O(n) time, O(n) space

SPACE OPTIMIZED: only need last two values
prev, curr = 0, 1
for _ in 2..n:
    prev, curr = curr, prev + curr
O(n) time, O(1) space
```

### Longest Common Subsequence (LCS)

```
LCS("ABCBDAB", "BDCAB") = ?

Build a table:
    ""  B  D  C  A  B
""   0  0  0  0  0  0
A    0  0  0  0  1  1
B    0  1  1  1  1  2
C    0  1  1  2  2  2
B    0  1  1  2  2  3
D    0  1  2  2  2  3
A    0  1  2  2  3  3
B    0  1  2  2  3  4

Answer: LCS length = 4 ("BCAB" or "BDAB")

Recurrence:
if s1[i] == s2[j]: dp[i][j] = dp[i-1][j-1] + 1
else:              dp[i][j] = max(dp[i-1][j], dp[i][j-1])
```

### 0/1 Knapsack

```
Items: weight=[2,3,4,5], value=[3,4,5,6], capacity=5

dp[i][w] = max value using first i items with capacity w

    w: 0  1  2  3  4  5
i=0:   0  0  0  0  0  0
i=1:   0  0  3  3  3  3   (item1: w=2,v=3)
i=2:   0  0  3  4  4  7   (item2: w=3,v=4)
i=3:   0  0  3  4  5  7   (item3: w=4,v=5)
i=4:   0  0  3  4  5  7   (item4: w=5,v=6)

Answer: dp[4][5] = 7 (items 1 and 2: total weight=5, value=7)

Recurrence:
if w < weight[i]: dp[i][w] = dp[i-1][w]
else:             dp[i][w] = max(dp[i-1][w], dp[i-1][w-weight[i]] + value[i])
```

---

# PART 11: THE COMPILATION PIPELINE IN DEPTH

Understanding what happens from source to binary builds a foundational mental model for debugging, optimization, and security.

```
SOURCE CODE (foo.c)
    ↓ [C Preprocessor: cpp]
    - Expand #include → inline header contents
    - Replace #define macros → text substitution
    - Process #ifdef/#ifndef → conditional compilation
    - Remove comments
    - Output: foo.i (preprocessed C, often 10-50x larger than source)

    ↓ [Compiler Frontend: parsing + semantic analysis]
    - Tokenize (lex): break into tokens (keywords, identifiers, literals)
    - Parse: build AST (Abstract Syntax Tree) from tokens
    - Type check: verify type safety, implicit conversions
    - Output: IR (Intermediate Representation) — GIMPLE for GCC, LLVM IR for Clang

    ↓ [Compiler Middle-end: optimization passes]
    - Constant folding: int x = 2 + 3; → int x = 5;
    - Dead code elimination: remove unreachable code
    - Inlining: replace function call with function body
    - Loop unrolling: replicate loop body to reduce branch overhead
    - Vectorization: use SIMD instructions (SSE/AVX)
    - Alias analysis, escape analysis, strength reduction...

    ↓ [Compiler Backend: code generation]
    - Lower IR to target ISA (x86-64, ARM, RISC-V...)
    - Register allocation: map variables to CPU registers
    - Instruction scheduling: reorder to hide latency
    - Output: foo.s (assembly language)

ASSEMBLY (foo.s)
    ↓ [Assembler: as]
    - Convert assembly mnemonics → binary machine code
    - Resolve local labels
    - Output: foo.o (object file, ELF on Linux)

OBJECT FILE (foo.o):
    - .text: machine code
    - .data: initialized globals
    - .bss: zero-initialized globals
    - .rodata: read-only data
    - Symbol table: exported/imported names
    - Relocation table: locations that need patching by linker

    ↓ [Linker: ld]
    - Combines multiple .o files
    - Resolves external symbol references
    - Performs relocations (patch addresses)
    - Links libraries (static .a or dynamic .so)
    - Output: ELF executable or .so

EXECUTABLE or SHARED LIBRARY
    ↓ [OS Loader: execve + dynamic linker ld.so]
    - Maps ELF segments into virtual memory
    - Loads shared libraries
    - Applies remaining relocations
    - Calls constructors (C++/Rust)
    - Jumps to _start → main()
```

---

# PART 12: PRACTICAL DEBUGGING AND TOOLS

---

## Tools for Systems Programming

### C/C++
```
gcc/clang -fsanitize=address     → AddressSanitizer: detects heap/stack/buffer overflow
gcc/clang -fsanitize=undefined   → UBSanitizer: detects UB at runtime
gcc/clang -fsanitize=thread      → ThreadSanitizer: detects data races
valgrind --leak-check=full       → memory leak detection and profiling
gdb / lldb                       → debugger (break, step, examine memory)
perf stat / perf record          → CPU performance profiling
strace                           → trace system calls
ltrace                           → trace library calls
objdump -d binary                → disassemble binary
nm binary                        → list symbols
readelf -a binary                → full ELF inspection
```

### Go
```
go run -race main.go             → race detector
go test -race ./...              → race detector for tests
go build -gcflags='-m' ./...     → escape analysis output
go tool pprof                    → CPU/memory profiling
go tool trace                    → execution trace (goroutine timeline)
go vet ./...                     → static analysis
staticcheck ./...                → advanced static analysis
dlv                              → Delve debugger
```

### Rust
```
cargo build --release            → optimized build
cargo test                       → run tests
cargo bench                      → run benchmarks
cargo clippy                     → linter (catches many bugs and style issues)
cargo miri                       → experimental interpreter, catches UB in unsafe
cargo-asm                        → view assembly output for functions
cargo-flamegraph                 → CPU flamegraph profiling
RUST_BACKTRACE=1 cargo run       → full panic backtrace
```

---

# PART 13: INTERVIEW MENTAL MODELS — HOW TO THINK LIKE A SYSTEMS PROGRAMMER

The single most important skill in a systems interview is not knowing answers — it's having the right **mental model** to derive answers on the spot.

## The Four Questions

For any C code problem, ask:
1. **Where is this data stored?** (stack / heap / static / rodata)
2. **Who owns it?** (which function / struct is responsible for freeing)
3. **How long is it valid?** (lifetime of the allocation/scope)
4. **Is this behavior guaranteed?** (or is it undefined behavior)

## The Four Levels of Understanding

For any system:
1. **What does it do?** (API / interface)
2. **How does it work?** (implementation / algorithm)
3. **Why does it work?** (invariants / correctness proofs)
4. **What can go wrong?** (failure modes / edge cases)

## Complexity Intuition

When you see a loop inside a loop: O(n²) until proven otherwise.
When you see a binary split: O(log n) usually.
When you see n independent O(log n) operations: O(n log n).
When you see recursion with branching: O(2^depth) until proven otherwise (DP fixes this).

## The Stack vs Heap Heuristic

Rule of thumb:
- Known size at compile time + short-lived → **stack**
- Unknown size or long-lived → **heap**
- Shared between goroutines/threads → **heap** (with synchronization)
- Single-threaded, single-owner → **stack** if possible

## Language Choice Heuristic

- **C**: when you need direct hardware control, minimal runtime, embeddable, or are writing OS code
- **Go**: when you need fast development velocity, built-in concurrency, garbage collection is acceptable, network services
- **Rust**: when you need C-level performance with memory safety guarantees, systems work where correctness is critical

# Complete Systems Programming Interview Guide
## C · Go · Rust — Deep Internals, Mental Models, ASCII Diagrams

> **How to use this guide**: Every section builds a mental model. Read the ASCII diagrams slowly — they represent the actual machine state. Run every code snippet. The goal is not memorization but understanding *why* the machine behaves as it does.

---

# Table of Contents

1. [Process Memory Layout](#1-process-memory-layout)
2. [Stack Deep Dive](#2-stack-deep-dive)
3. [Heap Deep Dive](#3-heap-deep-dive)
4. [Pointers and Pointer Arithmetic](#4-pointers-and-pointer-arithmetic)
5. [Arrays vs Pointers — The Classic Confusion](#5-arrays-vs-pointers)
6. [Strings in C](#6-strings-in-c)
7. [Undefined Behavior — The Silent Killer](#7-undefined-behavior)
8. [Dynamic Memory Management Internals](#8-dynamic-memory-management)
9. [Structs, Unions, Enums, Typedef](#9-structs-unions-enums-typedef)
10. [Type System, Qualifiers, and Casts](#10-type-system-qualifiers-and-casts)
11. [Preprocessor and Compilation Pipeline](#11-preprocessor-and-compilation-pipeline)
12. [Function Pointers and Callbacks](#12-function-pointers-and-callbacks)
13. [Bitwise Operations and Endianness](#13-bitwise-operations-and-endianness)
14. [Go: Goroutines and the GMP Scheduler](#14-go-goroutines-and-gmp-scheduler)
15. [Go: Channels — Internal Design](#15-go-channels-internal-design)
16. [Go: File I/O for Large Files](#16-go-file-io)
17. [Rust: Ownership, Borrowing, Lifetimes](#17-rust-ownership-borrowing-lifetimes)
18. [Rust: Memory Safety Without a GC](#18-rust-memory-safety)
19. [Data Structures — Internal Implementation](#19-data-structures-internals)
20. [Algorithms — Internal Mechanics](#20-algorithms-internals)
21. [Operating System Concepts](#21-operating-system-concepts)
22. [Concurrency Fundamentals](#22-concurrency-fundamentals)

---

# 1. Process Memory Layout

## The Mental Model

Every running program is given a virtual address space by the OS. This is not physical RAM — it is a *virtual* map that the kernel and MMU translate to physical memory. Understanding this layout is the single most important foundation for systems programming.

```
Virtual Address Space (64-bit Linux, typical layout)
Higher addresses
+---------------------------+  0xFFFFFFFFFFFFFFFF
|     Kernel space          |  (inaccessible to user programs)
+---------------------------+  0xC000000000000000
|          ...              |
+---------------------------+
|   Stack (grows down ↓)    |  argv, env, local vars, return addrs
|                           |
|       ↓ grows             |
+---------------------------+  (stack pointer, changes dynamically)
|           ...             |  (large gap — unmapped virtual pages)
|       ↑ grows             |
+---------------------------+
|  Heap (grows up ↑)        |  malloc/free region
+---------------------------+  (brk pointer)
|   BSS segment             |  uninitialized globals/statics (zeroed by OS)
+---------------------------+
|   Data segment            |  initialized globals/statics
+---------------------------+
|   Text segment (code)     |  read-only, executable
+---------------------------+  0x0000000000400000 (typical ELF base)
|   NULL guard page         |  catching NULL pointer dereferences
+---------------------------+  0x0000000000000000
Lower addresses
```

### Segments Explained

**Text Segment**
- Contains compiled machine instructions.
- Read-only (write attempt = segfault).
- String literals often live here.
- Shared among processes running the same binary.

**Data Segment (initialized)**
- Global and static variables with non-zero initial values.
- `int x = 42;` at file scope → data segment.
- Exists in the binary on disk.

**BSS Segment (uninitialized)**
- Global and static variables with zero/no initializer.
- `int arr[10000];` at file scope → BSS.
- Not stored on disk — the OS just zeros the pages on demand.
- "BSS" = Block Started by Symbol (historical name).

**Heap**
- Dynamically allocated memory.
- Managed by the allocator (`malloc`/`free`, `new`/`delete`, etc).
- Grows upward conceptually (though modern allocators are more complex).

**Stack**
- Grows *downward* (toward lower addresses) on x86.
- One per thread.
- Holds: local variables, function arguments, return addresses, saved registers.
- Automatically managed: push on call, pop on return.

### C Example — Where Each Variable Lives

```c
#include <stdio.h>
#include <stdlib.h>

int global_init = 100;          // DATA segment
int global_uninit;              // BSS segment
static int static_init = 200;  // DATA segment

int main(void) {
    int local = 42;             // STACK
    static int func_static = 5; // DATA segment (persists across calls)
    int *heap_ptr = malloc(64); // heap_ptr is on STACK, *heap_ptr is on HEAP

    const char *literal = "hi"; // literal points into TEXT/RODATA segment
    char arr[] = "hello";       // arr is a STACK copy of the string

    printf("global_init  addr: %p\n", (void*)&global_init);
    printf("local        addr: %p\n", (void*)&local);
    printf("heap_ptr val addr: %p\n", (void*)heap_ptr);
    printf("literal      addr: %p\n", (void*)literal);

    free(heap_ptr);
    return 0;
}
```

Sample output (addresses will differ on your machine):
```
global_init  addr: 0x404020    ← data segment (low address)
local        addr: 0x7ffee4b2  ← stack (high address)
heap_ptr val addr: 0x55f3a002  ← heap (middle)
literal      addr: 0x402010    ← rodata (near text segment)
```

### Go Equivalent — Escape Analysis Decides Placement

In Go, the compiler decides stack vs heap through **escape analysis**:

```go
package main

import "fmt"

func stackVar() int {
    x := 42      // stays on stack — doesn't escape
    return x
}

func heapVar() *int {
    x := 42      // escapes to heap — pointer returned
    return &x    // taking address and returning → heap allocation
}

func main() {
    a := stackVar() // a is a copy, on stack
    b := heapVar()  // b is a pointer to heap
    fmt.Println(a, *b)
}
```

To see escape analysis decisions:
```bash
go build -gcflags="-m" main.go
# Output shows: "x escapes to heap"
```

### Rust Equivalent — Ownership Makes This Explicit

```rust
fn stack_var() -> i32 {
    let x: i32 = 42;   // stack allocated
    x                   // value is copied out, original dropped
}

fn heap_var() -> Box<i32> {
    let x = Box::new(42);  // explicitly heap-allocated
    x                       // ownership moved out, not dropped
}

fn main() {
    let a = stack_var();  // copy on stack
    let b = heap_var();   // Box<i32> owns heap memory
    println!("{} {}", a, b);
    // b dropped here → heap memory freed automatically
}
```

---

# 2. Stack Deep Dive

## What is a Stack Frame?

Every function call creates a **stack frame** (also called an **activation record**). It contains everything the function needs during its execution.

```
Call stack during: main() → foo() → bar()

Higher addresses
+================================+
|  main() frame                  |
|  - local variables of main     |
|  - saved registers             |
|  - return address (OS)         |
+================================+
|  foo() frame                   |
|  - arguments passed to foo     |
|  - local variables of foo      |
|  - saved rbp (frame pointer)   |
|  - return address (→ main)     |
+================================+  ← rbp (frame pointer)
|  bar() frame                   |
|  - arguments passed to bar     |
|  - local variables of bar      |
|  - saved rbp (→ foo's frame)   |
|  - return address (→ foo)      |
+================================+  ← rsp (stack pointer, current top)

Stack grows downward ↓
Lower addresses
```

### Detailed Stack Frame Layout (x86-64, System V ABI)

```
bar(int a, int b) called from foo:

  foo's frame
  +-----------------------+
  |  local var of foo     |  rbp + offset
  |  ...                  |
  +-----------------------+
  |  arg 1 (a) in rdi     |  (first 6 args in registers, rest on stack)
  |  arg 2 (b) in rsi     |
  +-----------------------+
  |  return address       |  address of next instruction in foo
  +-----------------------+  ← bar's rbp points here after prologue
  bar's frame
  +-----------------------+
  |  saved rbp (foo's)    |  ← [rbp + 0]
  +-----------------------+  ← rbp (bar's frame pointer)
  |  bar's local vars     |  ← [rbp - 8], [rbp - 16], ...
  |  int result           |
  |  char buf[64]         |
  |  ...                  |
  +-----------------------+  ← rsp (stack pointer)
```

### Function Prologue and Epilogue (x86-64 Assembly)

Every non-leaf function runs this setup:

```asm
; Prologue (function entry)
push    rbp          ; save caller's frame pointer
mov     rbp, rsp     ; set our frame pointer
sub     rsp, 48      ; allocate space for locals (48 bytes)

; ... function body ...

; Epilogue (function return)
mov     rsp, rbp     ; restore stack pointer (deallocate locals)
pop     rbp          ; restore caller's frame pointer
ret                  ; pop return address into rip, jump there
```

This is *exactly* what the compiler generates. Knowing this lets you understand:
- Why local variable lifetime ends at return
- Why a dangling pointer is dangerous
- What buffer overflows actually corrupt

### The Dangling Pointer — Full Stack Story

```c
#include <stdio.h>
#include <string.h>

char *createMessage() {
    char buff[500];           // lives in createMessage's stack frame
    strcpy(buff, "hi there");
    char *ptr = buff;
    return ptr;               // returns address of a dead stack location
}

int main(void) {
    char *msg = createMessage();
    // createMessage's frame is now "gone" (stack pointer moved back)
    // BUT the bytes may still physically be there
    printf("%s\n", msg);      // "works" or crashes — undefined behavior
    return 0;
}
```

**Stack state timeline:**

```
DURING createMessage():

Stack:
+---------------------------+
|  main() frame             |
|  msg = ???                |
|  return addr (OS)         |
+---------------------------+
|  createMessage() frame    |
|  buff[500] = "hi there\0" |  ← ptr points here (e.g., 0x7fff1000)
|  ptr = 0x7fff1000         |
+---------------------------+  ← rsp

AFTER createMessage() returns:

Stack:
+---------------------------+
|  main() frame             |
|  msg = 0x7fff1000         |  ← still points to old location!
|  return addr (OS)         |
+---------------------------+  ← rsp (stack rewound here)
|  [dead frame bytes]       |  ← 0x7fff1000 is HERE (below stack ptr)
|  buff still contains      |  may still say "hi there\0"
|  "hi there\0" for now...  |  until next function call overwrites it
+---------------------------+
```

This is why it "works sometimes":
- The bytes at `0x7fff1000` are unchanged *until* something else uses that stack space.
- Calling `printf` itself uses the stack, potentially overwriting those bytes.
- Behavior is completely unpredictable → **undefined behavior**.

### Stack Overflow

```c
void infinite_recurse(void) {
    char big_buffer[1024 * 1024];  // 1MB on each frame
    big_buffer[0] = 1;             // touch it so it's not optimized away
    infinite_recurse();            // recurse forever
}

// Each call adds ~1MB to stack → stack overflow → SIGSEGV (segfault)
// Default stack size: Linux = 8MB, macOS = 8MB, Windows = 1MB
```

### Go Stack — Growable and Copying

Go stacks start small (2KB typically) and **grow dynamically**:

```
Initial goroutine stack: 2KB

goroutine G1:
+------------------+
| frame 1 (main)   | 2KB initial
+------------------+

After many nested calls:
+---------------------------+
| NEW 4KB stack (copied!)   |
| frame 1 (main)            | ← all frames copied to new larger stack
| frame 2 (foo)             |
| frame 3 (bar)             |
| frame 4 (baz)             |
+---------------------------+
```

This is why **Go pointers to stack variables are safe** — the GC tracks them, and when the stack is copied, the pointers are updated. This is also why taking a pointer in Go can cause a heap escape.

### Rust Stack — Same as C, but Safe

```rust
fn create_message() -> String {
    let buff = String::from("hi there");
    buff  // ownership MOVED out, not a pointer to a local
         // buff's heap data persists, the String header is moved
}

fn main() {
    let msg = create_message();  // owns the string now
    println!("{}", msg);         // safe
}                                // msg dropped here, heap freed
```

Rust prevents the dangling pointer at compile time:
```rust
fn bad_create() -> &str {
    let buff = String::from("hi");
    &buff  // ERROR: returns reference to local variable
           // compiler error: cannot return reference to local data
}
```
```
error[E0515]: cannot return reference to local variable `buff`
```

---

# 3. Heap Deep Dive

## What is the Heap?

The heap is a region of memory where you manually (in C) or automatically (Go/Rust GC/ownership) request and release memory at runtime. Unlike the stack, heap allocations have no fixed lifetime.

```
Heap layout (simplified):

+----------------------------------------------------+
| Heap start (brk pointer)                           |
+----------------------------------------------------+
| [chunk header][user data .........][padding]       |
+----------------------------------------------------+
| [chunk header][user data ...][padding]             |
+----------------------------------------------------+
| [free chunk header][prev_ptr][next_ptr][.........] |  ← in free list
+----------------------------------------------------+
| [chunk header][user data ....................]     |
+----------------------------------------------------+
| ... (grows upward toward brk)                      |
+----------------------------------------------------+
| brk (program break — current heap limit)           |
+----------------------------------------------------+
```

### malloc Internals (glibc ptmalloc)

When you call `malloc(n)`, the allocator:

1. Rounds `n` up to alignment (usually 16 bytes on 64-bit).
2. Looks in free lists for a suitable chunk.
3. If none found, calls `sbrk()` or `mmap()` to get more memory from kernel.
4. Returns pointer to user data area.

**Chunk structure (glibc):**

```
malloc chunk in memory:

  Previous chunk (if free)
  +---------------------------+
  |  prev_size (8 bytes)      |  size of previous chunk if it's free
  +---------------------------+
  |  size (8 bytes)           |  size of THIS chunk + flags in low bits
  |  [P=prev_in_use][M][A]    |  P=1 means previous chunk is in use
  +---------------------------+  ← malloc() returns pointer HERE
  |  user data                |
  |  (n bytes requested)      |
  |  + alignment padding      |
  +---------------------------+
  |  (next chunk's prev_size) |
  +---------------------------+
```

**Free list (bins):**

```
Small bins (chunks < 512 bytes):
bin[2]:  16-byte chunks → [chunk]→[chunk]→[chunk]→NULL
bin[3]:  24-byte chunks → [chunk]→NULL
...
bin[63]: 504-byte chunks → [chunk]→[chunk]→NULL

Large bins (chunks >= 512 bytes):
bin[64]: 512-1023 bytes, sorted by size
bin[65]: 1024-1535 bytes, sorted by size
...

Unsorted bin:
Recently freed chunks go here first (cache)

Fast bins (very small, for speed):
16, 24, 32, 40, 48, 56, 64 byte chunks
LIFO (stack-like) for speed — no coalescing
```

### The malloc/free Lifecycle

```c
int *p = malloc(sizeof(int) * 4);  // request 32 bytes

// allocator finds/creates a 48-byte chunk (32 + header overhead + alignment):
// [header: size=48, P=1][16 bytes ... user data (32 bytes) ...][padding]
//                        ^
//                        p points here

p[0] = 10; p[1] = 20; p[2] = 30; p[3] = 40;

free(p);
// chunk goes to appropriate free bin
// [header: size=48, P=0][prev_ptr][next_ptr][...garbage...]
//                        ^
//                        p still points here (but we must not use it!)
```

### Common Heap Bugs

```c
// 1. Memory leak
void leak(void) {
    int *p = malloc(100);
    // forgot to free(p)
    // heap memory unreachable but allocated
}

// 2. Double free
void double_free(void) {
    int *p = malloc(64);
    free(p);
    free(p);  // DISASTER: corrupts allocator's free list metadata
              // can lead to arbitrary code execution in exploits!
}

// 3. Use-after-free
void use_after_free(void) {
    int *p = malloc(sizeof(int));
    *p = 42;
    free(p);
    printf("%d\n", *p);  // undefined behavior
                         // chunk may have been reallocated and overwritten
}

// 4. Heap buffer overflow
void overflow(void) {
    int *p = malloc(4 * sizeof(int));
    p[4] = 99;  // one past the end — overwrites next chunk's header!
}

// 5. NULL dereference after failed malloc
void no_check(void) {
    int *p = malloc(1024 * 1024 * 1024);  // 1GB might fail
    *p = 1;  // crash if p == NULL
}
```

### Go Heap — Managed by GC

```go
package main

import "fmt"

func allocate() []int {
    // slice header on stack, backing array on heap
    s := make([]int, 1000)
    return s  // backing array survives because GC tracks it
}

func main() {
    s := allocate()
    fmt.Println(len(s))
    // GC frees s's backing array when no more references exist
}
```

Go's GC is a **tricolor mark-and-sweep** with write barriers:
```
White = not yet visited
Grey  = discovered but children not scanned
Black = fully scanned

GC phases:
1. STW (stop-the-world) — short — set up write barriers
2. Concurrent mark — scan roots → grey → black
3. STW mark termination — short
4. Concurrent sweep — reclaim white objects
```

### Rust Heap — Ownership as Compile-Time GC

```rust
fn allocate() -> Vec<i32> {
    let v = vec![0i32; 1000];  // heap-allocated
    v  // ownership moved to caller
}  // if NOT moved, v would be dropped here and heap freed

fn main() {
    let s = allocate();  // owns the Vec
    println!("{}", s.len());
}  // s dropped here → Vec::drop() → deallocates heap memory
   // NO runtime overhead — compiler inserted the free at compile time
```

---

# 4. Pointers and Pointer Arithmetic

## What is a Pointer?

A pointer is a variable whose value is a memory address. On 64-bit systems, every pointer is 8 bytes, regardless of what it points to.

```
int x = 42;
int *p = &x;

Memory layout:
Address:  0x7fff0000   0x7fff0008
          +----------+ +---------+
          |    42    | | 0x7fff0000 |
          +----------+ +---------+
              x              p

p contains the value 0x7fff0000 (the address of x)
*p dereferences: goes to address 0x7fff0000 and reads the int there
```

### Pointer Arithmetic

Pointer arithmetic automatically scales by the size of the pointed-to type:

```c
int arr[5] = {10, 20, 30, 40, 50};
int *p = arr;  // p points to arr[0]

// Memory layout:
// Address: 1000  1004  1008  1012  1016
//          +--+  +--+  +--+  +--+  +--+
//          |10|  |20|  |30|  |40|  |50|
//          +--+  +--+  +--+  +--+  +--+
//           ^
//           p = 1000

p + 1  // = 1004 (moves 4 bytes = sizeof(int))
p + 2  // = 1008
p + 3  // = 1012

*(p + 2)  // = 30 (read value at address 1008)
p[2]      // EXACTLY the same as *(p + 2)
```

**Key formula:**  `p[i]` ≡ `*(p + i)` ≡ `*(i + p)` ≡ `i[p]` (all identical in C!)

### Pointer Types and Sizes

```c
#include <stdio.h>
#include <stdint.h>

char   *cp;   // points to 1-byte char
int    *ip;   // points to 4-byte int  
long   *lp;   // points to 8-byte long
double *dp;   // points to 8-byte double
void   *vp;   // generic pointer, no arithmetic

// All POINTERS are 8 bytes on 64-bit:
printf("%zu\n", sizeof(cp));  // 8
printf("%zu\n", sizeof(ip));  // 8
printf("%zu\n", sizeof(lp));  // 8

// But pointer arithmetic is type-aware:
char  *c = (char*)0x1000;
int   *i = (int*)0x1000;
c + 1;  // = 0x1001 (moves 1 byte)
i + 1;  // = 0x1004 (moves 4 bytes)
```

### Pointer to Pointer

```c
int x = 42;
int *p = &x;    // p points to x
int **pp = &p;  // pp points to p

// Memory:
// x:   [42]         at addr 0x100
// p:   [0x100]      at addr 0x200
// pp:  [0x200]      at addr 0x300

*pp   // == p  == 0x100
**pp  // == x  == 42
```

**Common use — modifying a pointer from a function:**

```c
void allocate(int **out) {
    *out = malloc(sizeof(int));  // write through pp to modify caller's pointer
    **out = 99;
}

int main(void) {
    int *p = NULL;
    allocate(&p);     // pass address of pointer
    printf("%d\n", *p);  // 99
    free(p);
}
```

### Const Pointer Variants

```c
// Read the declaration right-to-left:

const int *p;       // pointer to const int
                    // can change p (where it points)
                    // cannot change *p (the value)

int *const p;       // const pointer to int
                    // cannot change p (where it points)
                    // can change *p (the value)

const int *const p; // const pointer to const int
                    // cannot change either

// Examples:
int x = 1, y = 2;
const int *p = &x;
p = &y;     // OK — p itself can change
*p = 3;     // ERROR — *p is const

int *const q = &x;
q = &y;     // ERROR — q itself is const
*q = 3;     // OK — *q can change
```

### Go Pointers — Safer but Similar

```go
package main

import "fmt"

func increment(p *int) {
    *p++  // dereference and modify
}

func main() {
    x := 10
    increment(&x)  // pass address
    fmt.Println(x) // 11

    // No pointer arithmetic in Go:
    arr := [5]int{1, 2, 3, 4, 5}
    p := &arr[0]
    // p + 1 is NOT allowed in Go (compile error)
    // Use indices instead: arr[1], arr[2], etc.
}
```

### Rust References — Pointers with Guarantees

```rust
fn increment(p: &mut i32) {
    *p += 1;  // dereference and modify
}

fn main() {
    let mut x = 10;
    increment(&mut x);  // exclusive mutable reference
    println!("{}", x);  // 11

    // Raw pointers exist but require unsafe:
    let arr = [1i32, 2, 3, 4, 5];
    let p: *const i32 = arr.as_ptr();
    unsafe {
        println!("{}", *p.add(2));  // = 3, raw pointer arithmetic
    }
}
```

**Rust reference rules (enforced at compile time):**
```
At any point in time, you can have EITHER:
  - ONE mutable reference (&mut T), OR
  - ANY NUMBER of immutable references (&T)
But NOT both simultaneously.

This prevents:
  - Data races (two threads writing concurrently)
  - Iterator invalidation (modifying a collection while iterating)
  - Use-after-free (references cannot outlive the data they point to)
```

---

# 5. Arrays vs Pointers

## The Crucial Difference

Arrays and pointers are related but fundamentally different. This distinction trips up even experienced C programmers.

```c
int arr[5] = {1, 2, 3, 4, 5};
int *p = arr;

// sizeof reveals the truth:
sizeof(arr)  // = 20 (5 * 4 bytes) — knows the full array size
sizeof(p)    // = 8 (just a pointer — lost size information)
```

### Array Decay

In most expressions, an array name "decays" to a pointer to its first element:

```c
int arr[5] = {1, 2, 3, 4, 5};

// These are IDENTICAL after decay:
int *p = arr;       // arr decays to &arr[0]
int *q = &arr[0];   // explicit

// Contexts where array does NOT decay:
sizeof(arr)         // returns full array size (no decay)
&arr                // returns pointer to the ARRAY, not pointer to element
                    // type is int(*)[5], not int*
```

**The confusing `&arr` vs `arr`:**

```c
int arr[5] = {1, 2, 3, 4, 5};
int *p  = arr;    // &arr[0]: points to element, int*
int (*q)[5] = &arr; // points to the whole array, int(*)[5]

// Both p and q have the SAME numeric address (arr[0] and arr have same address)
// But arithmetic is different:
p + 1  // moves 4 bytes (sizeof int) → points to arr[1]
q + 1  // moves 20 bytes (sizeof int[5]) → points past the whole array!
```

ASCII diagram:

```
int arr[5]:
Address:  1000   1004   1008   1012   1016
          +--+   +--+   +--+   +--+   +--+
          | 1|   | 2|   | 3|   | 4|   | 5|
          +--+   +--+   +--+   +--+   +--+
           ^                              ^
           p (arr, &arr[0])               p+4 or q+1 (past array)
           q (&arr) — same address, different type
```

### Multidimensional Arrays

```c
int matrix[3][4];  // 3 rows, 4 columns

// Memory layout is ROW-MAJOR (all of row 0, then row 1, then row 2):
// Address: 0    4    8    12   16   20   24   28   32   40   44   48
//          [0,0][0,1][0,2][0,3][1,0][1,1][1,2][1,3][2,0][2,1][2,2][2,3]

matrix[r][c] == *(*(matrix + r) + c)
             == *(&matrix[0][0] + r * 4 + c)

// sizeof:
sizeof(matrix)     // 48 bytes (3 * 4 * 4)
sizeof(matrix[0])  // 16 bytes (one row: 4 * 4)
sizeof(matrix[0][0]) // 4 bytes (one int)
```

### Array of Pointers vs Pointer to Array

```c
// Array of pointers (like argv):
char *words[5];  // 5 pointers to char
// Each pointer can point anywhere
// Memory layout:
// +------+------+------+------+------+
// | ptr0 | ptr1 | ptr2 | ptr3 | ptr4 |  (array of 8-byte pointers)
// +------+------+------+------+------+
//    |      |
//    v      v
// "hello"  "world"  (pointed-to strings can be anywhere)

// Pointer to array:
int (*p)[4];  // pointer to an array of 4 ints
int matrix[3][4];
p = matrix;   // p points to first row (an array of 4 ints)
p[0]          // first row: int[4]
p[1]          // second row: int[4] (advances 16 bytes)
```

### Go — No Array Decay, Slices are the Idiom

```go
// In Go, arrays are value types (copying semantics):
a := [5]int{1, 2, 3, 4, 5}
b := a  // b is a FULL COPY of a
b[0] = 99
fmt.Println(a[0])  // 1 — a is unchanged!

// len/cap always known:
fmt.Println(len(a))  // 5 — no decay, size is always available

// Slices are the idiomatic dynamic array:
s := []int{1, 2, 3, 4, 5}  // slice: header (ptr, len, cap) + heap array

// Slice header:
// +-------+-----+-----+
// |  ptr  | len | cap |    (24 bytes total on 64-bit)
// +-------+-----+-----+
//     |
//     v
// +--+--+--+--+--+  (heap-allocated backing array)
// | 1| 2| 3| 4| 5|
// +--+--+--+--+--+
```

### Rust — Arrays and Slices

```rust
// Arrays: fixed size, stack allocated, no decay
let arr: [i32; 5] = [1, 2, 3, 4, 5];
let b = arr;  // arr is COPIED (if T: Copy), or moved
println!("{}", arr.len());  // 5, always known

// Slices: fat pointer (ptr + len), can reference arrays or Vec
let slice: &[i32] = &arr;          // borrow of array
let slice2: &[i32] = &arr[1..4];   // sub-slice

// Fat pointer (reference to slice):
// +------+-----+
// |  ptr | len |   (16 bytes: pointer + usize)
// +------+-----+
//     |
//     v
//  arr[0] in memory

// Vec<T>: heap-allocated, growable
let mut v: Vec<i32> = vec![1, 2, 3];
v.push(4);  // may reallocate backing array
// Vec header: +------+-----+-----+
//             |  ptr | len | cap |
//             +------+-----+-----+
```

---

# 6. Strings in C

## What is a C String?

A C string is a sequence of `char` values terminated by a null byte (`'\0'`). There is no separate string type — it's just a convention for where the string ends.

```
"hello" in memory:
+---+---+---+---+---+---+
| h | e | l | l | o |\0 |
+---+---+---+---+---+---+
  0   1   2   3   4   5    (6 bytes total)

strlen("hello") = 5   (characters up to but NOT including '\0')
sizeof("hello") = 6   (total bytes including '\0')
```

### String Literals vs Character Arrays

```c
// String literal — stored in read-only memory (text/rodata segment):
const char *s1 = "hello";  // s1 points INTO read-only memory
// s1[0] = 'H';            // CRASH — modifying read-only memory

// Character array — stored on stack (or wherever declared):
char s2[] = "hello";  // compiler creates a 6-byte array and copies "hello\0"
s2[0] = 'H';          // OK — this is a writable copy

// Memory comparison:
// s1:  pointer on stack → points to "hello\0" in .rodata
// s2:  the actual bytes ['h','e','l','l','o','\0'] on stack

sizeof(s1)   // 8 (just the pointer)
sizeof(s2)   // 6 (the full array with null terminator)
strlen(s1)   // 5
strlen(s2)   // 5
```

ASCII diagram:

```
Stack (main):
+------------------+
| s1 = 0x402010    |  ← pointer to .rodata
| s2 = ['h','e',   |  ← actual bytes on stack
|       'l','l',   |
|       'o', '\0'] |
+------------------+

.rodata (read-only):
Address 0x402010:
+---+---+---+---+---+---+
| h | e | l | l | o |\0 |
+---+---+---+---+---+---+
```

### String Functions — Internal Behavior

**`strcpy` — unsafe, no bounds checking:**
```c
char dst[10];
char *src = "hello world!";  // 13 bytes including \0
strcpy(dst, src);  // writes 13 bytes into 10-byte buffer!
                   // overwrites adjacent memory → stack smashing

// What strcpy does internally:
char *my_strcpy(char *dst, const char *src) {
    char *d = dst;
    while ((*dst++ = *src++) != '\0');  // copy byte by byte until null
    return d;
}
```

**`strncpy` — safer but still has traps:**
```c
char dst[10];
strncpy(dst, "hello world!", 9);  // copies at most 9 bytes
// TRAP: if src is longer than n, '\0' is NOT appended!
// dst might not be null-terminated after strncpy!
dst[9] = '\0';  // must manually terminate if you're not sure

// strncpy also pads with zeros if src is shorter — unusual behavior
```

**`strlcpy` (BSD) or `snprintf` — proper safe copy:**
```c
#include <string.h>

// strlcpy: always null-terminates, returns length of src
strlcpy(dst, src, sizeof(dst));

// snprintf: versatile safe string builder
snprintf(dst, sizeof(dst), "%s", src);  // always null-terminates
```

**`printf(msg)` — format string vulnerability:**
```c
char msg[100];
fgets(msg, sizeof(msg), stdin);
printf(msg);     // DANGER: if msg = "%x%x%x", reads stack memory!
printf("%s", msg);  // CORRECT: msg is just data, not a format string
```

### strlen Implementation

```c
size_t my_strlen(const char *s) {
    const char *p = s;
    while (*p != '\0') {
        p++;
    }
    return p - s;  // pointer difference = number of chars
}

// Trace on "hello":
// p=s  → *p='h' != '\0' → p++
// p=s+1 → *p='e' → p++
// p=s+2 → *p='l' → p++
// p=s+3 → *p='l' → p++
// p=s+4 → *p='o' → p++
// p=s+5 → *p='\0' → stop
// return (s+5) - s = 5
```

### Returning Pointers to Strings — All Safe Options

```c
// WRONG: pointer to local stack array
char *bad(void) {
    char buf[50] = "hello";
    return buf;  // dangling pointer!
}

// OPTION 1: static storage (beware: not thread-safe, not reentrant)
char *static_version(void) {
    static char buf[50] = "hello";
    return buf;  // buf lives for entire program
}

// OPTION 2: heap allocation (caller must free)
char *heap_version(void) {
    char *buf = malloc(50);
    if (!buf) return NULL;
    strcpy(buf, "hello");
    return buf;  // caller must free(ptr)
}

// OPTION 3: caller provides buffer (best practice in C)
int safe_version(char *buf, size_t bufsize) {
    return snprintf(buf, bufsize, "hello");
}
// caller:
char buf[50];
safe_version(buf, sizeof(buf));
```

### Go Strings — Immutable, UTF-8, Value Semantics

```go
// Go string is: pointer to bytes + length (no null terminator needed)
// +------+-----+
// |  ptr | len |   (16 bytes: the string header)
// +------+-----+
//     |
//     v
// bytes in memory (not necessarily null-terminated)

s := "hello"
fmt.Println(len(s))    // 5 (byte count)
fmt.Println(s[0])      // 104 (byte value of 'h')
// s[0] = 'H'          // ERROR: strings are immutable in Go

// Strings are value types — copying is cheap (header copy, shared bytes):
s2 := s   // both point to same bytes
s2 = "world"  // now s2 points to different bytes; s unchanged

// Iterate over runes (Unicode code points):
for i, r := range "héllo" {
    fmt.Printf("%d: %c (%d)\n", i, r, r)
}
// 0: h (104)
// 1: é (233) — 2 bytes in UTF-8! i jumps from 1 to 3
// 3: l (108)
// 4: l (108)
// 5: o (111)
```

### Rust Strings — Two Types, Both Sound

```rust
// &str: immutable string slice (like const char* but with length)
// String: owned, heap-allocated, growable (like char* from malloc)

// &str — lives anywhere (literal in rodata, slice of String, etc.)
let s1: &str = "hello";  // points to read-only memory, length known
// s1.len() = 5, s1 is NOT null-terminated (rust doesn't need it)

// String — owns its bytes on heap
let mut s2: String = String::from("hello");
s2.push_str(", world");  // can modify

// Conversions:
let slice: &str = &s2;  // borrow String as &str (free operation)
let owned: String = s1.to_string();  // copies bytes onto heap

// Memory layout:
// &str fat pointer:  [ptr | len]
// String:            [ptr | len | cap]  (like Vec<u8>)

// Rust strings are guaranteed UTF-8.
// If you need arbitrary bytes, use Vec<u8> or &[u8].
```

---

# 7. Undefined Behavior — The Silent Killer

## What is Undefined Behavior?

Undefined behavior (UB) is a situation in which the C standard places *no constraint* on what the compiler may do. This is not the same as "crash" or "wrong result" — it means *anything* can happen, including:

- Correct-looking output today
- Crash tomorrow
- Security vulnerability
- Time travel (optimization eliminating "impossible" code)

**The key insight:** When UB occurs, the optimizer is free to assume it *never* happens, which can lead to security bugs caused by the optimizer *removing* safety checks.

### Classic UB Examples

```c
// 1. Signed integer overflow
int x = INT_MAX;
x++;  // UB — signed overflow is UB in C (unlike unsigned which wraps)
      // Compiler may optimize loops assuming this never happens!

// 2. Null pointer dereference
int *p = NULL;
*p = 42;  // UB — almost always segfault, but technically UB

// 3. Out-of-bounds array access
int arr[5];
arr[5] = 1;   // UB — could corrupt memory, could crash, could "work"
arr[-1] = 1;  // UB — negative index

// 4. Use-after-free
int *p = malloc(sizeof(int));
free(p);
*p = 5;  // UB

// 5. Uninitialized variable
int x;
printf("%d\n", x);  // UB — could be anything in the stack frame

// 6. Modifying string literal
char *s = "hello";
s[0] = 'H';  // UB — write to read-only memory

// 7. Data race (C11 threads)
// Two threads writing to same variable without synchronization = UB

// 8. Shift out of range
int x = 1;
int y = x << 32;  // UB — shift >= width of type
int z = x << -1;  // UB — negative shift
```

### The Optimizer Exploit Pattern

This is the most dangerous UB pattern — the optimizer removes your safety check:

```c
// Programmer's intent: check for overflow before adding
int add(int a, int b) {
    if (a + b < a) {  // trying to detect overflow
        return -1;    // "overflow detected"
    }
    return a + b;
}
```

The compiler sees: "If `a + b < a` were true, that would be signed overflow, which is UB. Since UB never happens (by definition), this branch is dead code." Result:

```c
// Compiler effectively generates:
int add(int a, int b) {
    return a + b;  // overflow check silently removed!
}
```

**Correct overflow check:**
```c
#include <limits.h>
int safe_add(int a, int b) {
    if (b > 0 && a > INT_MAX - b) return -1;  // would overflow
    if (b < 0 && a < INT_MIN - b) return -1;  // would underflow
    return a + b;
}
// Or use __builtin_add_overflow (GCC/Clang):
int result;
if (__builtin_add_overflow(a, b, &result)) { /* handle */ }
```

### Detecting UB — Sanitizers

```bash
# AddressSanitizer: heap/stack buffer overflows, use-after-free
gcc -fsanitize=address -g program.c
./a.out

# UndefinedBehaviorSanitizer: signed overflow, null deref, shifts, etc.
gcc -fsanitize=undefined -g program.c
./a.out

# Both:
gcc -fsanitize=address,undefined -g program.c
```

### Go's Approach — Most UB Eliminated

```go
// Go eliminates most C UB sources:

// Bounds checks are automatic:
arr := [5]int{1,2,3,4,5}
_ = arr[5]  // runtime panic: index out of range (not silent corruption)

// Integer overflow is defined (wraps):
var x int32 = math.MaxInt32
x++  // x = -2147483648, defined behavior (wraps)

// Nil dereference: always panics (not UB)
var p *int
_ = *p  // panic: nil pointer dereference

// Uninitialized memory: zero-initialized always
var x int  // guaranteed to be 0
var s string // guaranteed to be ""
```

### Rust's Approach — UB Impossible in Safe Code

```rust
// Safe Rust eliminates all the common UB sources:

// Bounds checks:
let arr = [1, 2, 3, 4, 5];
let _ = arr[5];  // panic at runtime (like Go)
// Or use get() for explicit Option:
let x = arr.get(5);  // returns None, no panic

// No uninitialized memory in safe code:
let x: i32;
println!("{}", x);  // COMPILE ERROR: use of possibly-uninitialized `x`

// No dangling pointers: borrow checker prevents them
// No null pointers: use Option<T> instead

// Undefined behavior still exists — only in unsafe blocks:
unsafe {
    let ptr: *const i32 = std::ptr::null();
    let _ = *ptr;  // UB — null dereference in unsafe
}
```

---

# 8. Dynamic Memory Management

## malloc, calloc, realloc, free

```c
#include <stdlib.h>

// malloc: allocate n bytes, UNINITIALIZED
void *malloc(size_t n);

// calloc: allocate n*size bytes, ZERO-INITIALIZED
void *calloc(size_t n, size_t size);  // n elements of size bytes each

// realloc: resize an allocation
void *realloc(void *ptr, size_t new_size);
// - if ptr is NULL, behaves like malloc
// - if new_size is 0, behaves like free
// - may return NEW pointer (if had to move data)
// - always use returned pointer, never old one!

// free: release memory
void free(void *ptr);
// - free(NULL) is safe (no-op)
// - free anything else twice is UB
```

### realloc — The Common Trap

```c
// WRONG — leaks if realloc returns NULL:
int *arr = malloc(10 * sizeof(int));
arr = realloc(arr, 20 * sizeof(int));  // if this fails, arr = NULL, old memory lost!

// CORRECT:
int *new_arr = realloc(arr, 20 * sizeof(int));
if (!new_arr) {
    // arr still valid, handle error
    free(arr);
    return;
}
arr = new_arr;
```

### Implementing a Dynamic Array (Vector)

```c
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

typedef struct {
    int    *data;
    size_t  len;
    size_t  cap;
} IntVec;

IntVec vec_new(void) {
    return (IntVec){ .data = NULL, .len = 0, .cap = 0 };
}

int vec_push(IntVec *v, int val) {
    if (v->len == v->cap) {
        // Double the capacity (or start at 4)
        size_t new_cap = v->cap == 0 ? 4 : v->cap * 2;
        int *new_data = realloc(v->data, new_cap * sizeof(int));
        if (!new_data) return -1;
        v->data = new_data;
        v->cap  = new_cap;
    }
    v->data[v->len++] = val;
    return 0;
}

void vec_free(IntVec *v) {
    free(v->data);
    v->data = NULL;
    v->len = v->cap = 0;
}
```

Growth diagram:
```
Initial: cap=0, len=0, data=NULL

After push(1):
cap=4, len=1
data: +--+--+--+--+
      | 1| ?| ?| ?|
      +--+--+--+--+

After push(2),push(3),push(4),push(5):
cap=8, len=5  (doubled because len==cap)
data: +--+--+--+--+--+--+--+--+
      | 1| 2| 3| 4| 5| ?| ?| ?|
      +--+--+--+--+--+--+--+--+
```

### Go Implementation

```go
// Go's built-in slice IS the dynamic array:
s := make([]int, 0, 4)    // len=0, cap=4
s = append(s, 1, 2, 3, 4) // len=4, cap=4
s = append(s, 5)           // len=5, cap=8 (doubled automatically)

// append may or may not reallocate:
a := []int{1, 2, 3}
b := append(a, 4)
// If a had cap >= 4, b shares memory with a
// If not, b has new backing array
// This is why you must always use the return value of append!
```

### Rust Implementation

```rust
// Vec<T> is Rust's dynamic array:
let mut v: Vec<i32> = Vec::with_capacity(4);
v.push(1); v.push(2); v.push(3); v.push(4);
v.push(5);  // reallocates: new backing array with doubled capacity

// Manual implementation to understand internals:
use std::alloc::{alloc, dealloc, realloc, Layout};

struct MyVec<T> {
    ptr: *mut T,
    len: usize,
    cap: usize,
}

impl<T> Drop for MyVec<T> {
    fn drop(&mut self) {
        if self.cap > 0 {
            let layout = Layout::array::<T>(self.cap).unwrap();
            unsafe { dealloc(self.ptr as *mut u8, layout); }
        }
    }
}
```

### Allocator Internals — How Free Lists Work

```
Free list (doubly linked list in free chunks):

Heap memory:
+----------+----------+----------+----------+----------+
| [in use] | [FREE  ] | [in use] | [FREE  ] | [in use] |
+----------+----------+----------+----------+----------+
               |                    |
    +----------+                    |
    v                               |
[header]                       [header]
[fd: ----+]   ← forward ptr       [fd: NULL]
[bk: NULL]   ← backward ptr       [bk: ----+]
                                            |
                              (points back to first free chunk)

malloc(small):
  1. Look in appropriate small bin
  2. Remove chunk from bin
  3. Return to user

free(ptr):
  1. Look at adjacent chunks
  2. If adjacent chunks are also free, coalesce them (merge)
  3. Put merged chunk in appropriate bin
```

---

# 9. Structs, Unions, Enums, Typedef

## Structs — Layout and Padding

```c
struct Point {
    int x;   // 4 bytes
    int y;   // 4 bytes
};  // total: 8 bytes, no padding needed (both 4-byte aligned)

struct Mixed {
    char  a;  // 1 byte  at offset 0
              // 3 bytes PADDING (to align next int to 4-byte boundary)
    int   b;  // 4 bytes at offset 4
    char  c;  // 1 byte  at offset 8
              // 7 bytes PADDING (to make size a multiple of largest member = 8)
    double d; // 8 bytes at offset 16
};
// sizeof(struct Mixed) = 24 (not 14!)
```

ASCII layout:

```
struct Mixed memory layout:

Offset: 0    1    2    3    4    5    6    7    8    9    10   11   ...16  ...24
        +----+----+----+----+----+----+----+----+----+----+----+----+----+----+
        | a  |PADD|PADD|PADD|    b         |    | c  |PADD|PADD|PADD... | d      |
        +----+----+----+----+----+----+----+----+----+----+----+----+----+----+
                                 ^                                  ^
                           4-byte aligned                    8-byte aligned
```

**Why padding?** CPU reads are most efficient when data is at a natural alignment (address multiple of the type's size). Misaligned access may:
- Take extra cycles (some CPUs)
- Cause a fault (some embedded/RISC CPUs)
- Work fine (x86 permits misaligned, with penalty)

**Minimizing padding — order fields largest to smallest:**
```c
// Bad (24 bytes):
struct Bad { char a; double b; char c; };
//  offset 0: a (1 byte)
//  offset 1: 7 bytes padding
//  offset 8: b (8 bytes)
//  offset 16: c (1 byte)
//  offset 17: 7 bytes padding
// total: 24

// Good (10 bytes with no interior padding):
struct Good { double b; char a; char c; };
//  offset 0: b (8 bytes)
//  offset 8: a (1 byte)
//  offset 9: c (1 byte)
//  offset 10: 6 bytes padding (to align whole struct to 8)
// total: 16  (compiler must align struct to largest member)

// Force no padding (packed — dangerous, use only in hardware protocols):
struct __attribute__((packed)) Packed { char a; double b; char c; };
// sizeof = 10, but accessing b may be slow or trap on some architectures
```

### Unions — Shared Memory

A union lets multiple members share the same memory. Only one is valid at a time.

```c
union Data {
    int    i;
    float  f;
    char   bytes[4];
};

// All members start at the same address!
// sizeof(union Data) = 4 (size of largest member)
//
// Memory layout:
// Byte 0   Byte 1   Byte 2   Byte 3
// +-------+-------+-------+-------+
// |       |       |       |       |
// +-------+-------+-------+-------+
//  i (4 bytes, all)
//  f (4 bytes, all)
//  bytes[0] bytes[1] bytes[2] bytes[3]

union Data d;
d.i = 0x41424344;
printf("%c%c%c%c\n", d.bytes[0], d.bytes[1], d.bytes[2], d.bytes[3]);
// On little-endian: D C B A
// On big-endian: A B C D
// (useful for inspecting byte representation of values)
```

**Tagged union (discriminated union) — the safe pattern:**
```c
typedef enum { INT_TAG, FLOAT_TAG, STR_TAG } Tag;

typedef struct {
    Tag tag;
    union {
        int    i;
        float  f;
        char  *s;
    } value;
} Variant;

void print_variant(const Variant *v) {
    switch (v->tag) {
        case INT_TAG:   printf("int: %d\n", v->value.i); break;
        case FLOAT_TAG: printf("float: %f\n", v->value.f); break;
        case STR_TAG:   printf("str: %s\n", v->value.s); break;
    }
}
```

### Enums

```c
// Enum values are ints (guaranteed):
enum Color { RED, GREEN, BLUE };
// RED=0, GREEN=1, BLUE=2 (implicit)

enum Status { OK=200, NOT_FOUND=404, ERROR=500 };
// Explicit values, can be any int

enum Flags { READABLE=1, WRITABLE=2, EXECUTABLE=4 };
// Good for bit flags: OR them together
int perms = READABLE | WRITABLE;  // = 3

// sizeof(enum) is implementation-defined but usually sizeof(int)
printf("%zu\n", sizeof(enum Color));  // usually 4
```

### Go Enums — Iota

```go
type Color int

const (
    Red Color = iota  // 0
    Green             // 1
    Blue              // 2
)

// Bit flags:
type Perm int
const (
    Read    Perm = 1 << iota  // 1
    Write                     // 2
    Execute                   // 4
)

perms := Read | Write  // = 3
fmt.Println(perms & Read != 0)  // true: is readable?
```

### Rust Enums — Sum Types (Much More Powerful)

```rust
// Rust enums are algebraic data types (sum types)
// Each variant can have different data

enum Shape {
    Circle(f64),              // radius
    Rectangle(f64, f64),      // width, height
    Triangle { base: f64, height: f64 },  // named fields
}

fn area(s: &Shape) -> f64 {
    match s {
        Shape::Circle(r) => std::f64::consts::PI * r * r,
        Shape::Rectangle(w, h) => w * h,
        Shape::Triangle { base, height } => 0.5 * base * height,
    }
}

// The most important built-in enums:
// Option<T> replaces null pointers:
let maybe: Option<i32> = Some(42);
let none: Option<i32> = None;

// Result<T, E> replaces error codes/exceptions:
let ok: Result<i32, String> = Ok(42);
let err: Result<i32, String> = Err("something failed".to_string());
```

---

# 10. Type System, Qualifiers, and Casts

## const

```c
const int x = 42;    // x cannot be modified
x = 43;              // compile error

// const in function params = I promise not to modify this:
size_t strlen(const char *s);  // s points to chars I won't modify

// const doesn't mean immutable at the hardware level:
// (const_cast in C++ or casting away const in C is possible but UB to write through)
const int x = 42;
int *p = (int*)&x;  // cast away const
*p = 99;            // UB! (though often "works")
```

## volatile

`volatile` tells the compiler: "this value may change due to hardware, another thread, or signal — do not cache it in a register, do not optimize away reads/writes."

```c
// Hardware register:
volatile uint32_t *hw_register = (volatile uint32_t*)0xABCD0000;
uint32_t val = *hw_register;  // must actually read from address each time

// Without volatile, compiler might:
// - Cache value in register after first read
// - Optimize away "redundant" reads
// - Reorder memory accesses

// Interrupt flag:
volatile int interrupt_flag = 0;

void isr(void) {  // interrupt service routine
    interrupt_flag = 1;
}

void wait_for_interrupt(void) {
    while (!interrupt_flag);  // without volatile, compiler may hoist this out of loop!
}

// volatile + sig_atomic_t for signal handlers:
#include <signal.h>
volatile sig_atomic_t got_signal = 0;
```

## Integer Types and Promotions

```c
// Integer promotion: smaller types promoted to int before arithmetic
char a = 250;
char b = 10;
int result = a + b;  // both promoted to int first (no overflow of char!)
// = 260 as int, even though char can only hold 0-255 (unsigned) or -128 to 127

// Signed vs unsigned gotcha:
int  i = -1;
unsigned int u = 1;
if (i < u) {  // FALSE! i is converted to unsigned: -1 → very large positive number
    puts("i is less");
}
// -1 as unsigned = 4294967295 > 1, so condition is false!
```

## Type Punning with Unions vs Casts

```c
// Type punning: reinterpreting bytes as different type

// WRONG: strict aliasing violation (UB in C99+)
float f = 3.14f;
int i = *(int*)&f;  // UB: accessing float through int* violates strict aliasing

// CORRECT: use union
union FloatInt {
    float f;
    int   i;
};
union FloatInt u;
u.f = 3.14f;
int i = u.i;  // well-defined in C (NOT in C++!)

// CORRECT in C99+: use memcpy
float f = 3.14f;
int i;
memcpy(&i, &f, sizeof(i));  // always safe, no aliasing issue
// Compiler optimizes memcpy of same-size types to a single register move
```

## Go Type System

```go
// Go has strong typing — no implicit conversions between numeric types:
var x int = 42
var y int64 = int64(x)  // must be explicit
var z float64 = float64(x)

// Named types are distinct even if underlying type is same:
type Celsius float64
type Fahrenheit float64
c := Celsius(100)
// f := c + Fahrenheit(32)  // compile error! different types
f := Fahrenheit(c*9/5 + 32)  // must convert explicitly
```

## Rust Type System — Zero-Cost Abstractions

```rust
// Rust enforces conversion explicitly:
let x: i32 = 42;
let y: i64 = x as i64;    // widening: always safe
let z: i8 = x as i8;      // narrowing: truncates (no panic)
// Or use From/Into for safe conversions:
let y: i64 = i64::from(x); // won't compile if conversion could lose data

// Newtype pattern (zero-cost wrapper):
struct Meters(f64);
struct Seconds(f64);
// Cannot accidentally add meters to seconds!
fn speed(distance: Meters, time: Seconds) -> f64 {
    distance.0 / time.0
}
```

---

# 11. Preprocessor and Compilation Pipeline

## The Four Stages of C Compilation

```
Source file: foo.c
     |
     v
[1. PREPROCESSOR]  (cpp or cc -E)
     |  - Expands #include, #define, #if, #ifdef
     |  - Removes comments
     |  - Produces: foo.i (preprocessed C)
     v
[2. COMPILER]  (cc1)
     |  - Parses C, does semantic analysis
     |  - Optimizes
     |  - Produces: foo.s (assembly)
     v
[3. ASSEMBLER]  (as)
     |  - Converts assembly to machine code
     |  - Produces: foo.o (object file, ELF)
     v
[4. LINKER]  (ld)
     |  - Combines .o files + libraries
     |  - Resolves symbol references
     |  - Produces: a.out or foo (executable ELF)
     v
Executable
```

### Preprocessor Deep Dive

```c
// #define — textual substitution (dangerous — no type checking!)
#define PI 3.14159
#define SQUARE(x) ((x)*(x))  // parentheses critical!

// Why parens matter in macros:
#define BAD_SQUARE(x) x*x
BAD_SQUARE(2+1)  // expands to: 2+1*2+1 = 5, not 9!
SQUARE(2+1)      // expands to: ((2+1)*(2+1)) = 9 ✓

// #include "..." vs #include <...>
#include <stdio.h>   // look in system include dirs first
#include "mylib.h"   // look in current dir first, then system dirs

// Header guards prevent double-inclusion:
// mylib.h:
#ifndef MYLIB_H
#define MYLIB_H
// ... header contents ...
#endif

// Modern equivalent: #pragma once (non-standard but universally supported)
#pragma once

// Conditional compilation:
#ifdef DEBUG
    printf("debug: x = %d\n", x);
#endif

#if PLATFORM == LINUX
    // Linux-specific code
#elif PLATFORM == MACOS
    // macOS-specific code
#else
    #error "Unsupported platform"
#endif
```

### Object Files and Linking

```
ELF object file (foo.o) structure:
+------------------+
| ELF Header       | magic number, arch, entry point, etc.
+------------------+
| .text section    | compiled machine code
+------------------+
| .data section    | initialized global/static vars
+------------------+
| .bss section     | uninitialized global/static vars (just size)
+------------------+
| .rodata section  | string literals, const data
+------------------+
| Symbol Table     | list of defined/referenced symbols
| - printf (UNDEF) | references printf — linker must resolve
| - foo (GLOBAL)   | defines function foo
| - bar (LOCAL)    | defines static function bar (internal)
+------------------+
| Relocation Table | "patch address of printf once known"
+------------------+
```

**Linker's job:** Find all `UNDEF` symbols in symbol tables, find their definitions in other .o files or libraries, patch all the addresses.

```bash
# See symbols in an object file:
nm foo.o
# U printf    ← undefined (external reference)
# T main      ← defined in text section

# See linking errors: "undefined reference to `foo`" means
# no .o file or library defines symbol `foo`
```

### static vs extern

```c
// static — file-internal linkage (not visible to linker outside this file):
static int counter = 0;       // global: only visible in this .c file
static void helper(void) {}   // function: only visible in this .c file

// extern — declare that symbol is defined ELSEWHERE:
extern int global_from_other_file;  // declaration, no storage allocated
// In other.c: int global_from_other_file = 42;  // definition

// static local variable — persists across calls:
int generate_id(void) {
    static int next_id = 0;  // initialized once, persists
    return ++next_id;
}
// First call: returns 1
// Second call: returns 2
// etc.
```

---

# 12. Function Pointers and Callbacks

## What is a Function Pointer?

A function pointer stores the address of a function. This enables:
- Callbacks
- Dispatch tables (virtual functions without OOP)
- Plugin architectures
- Strategy pattern

```c
// Function pointer declaration syntax (read with inside-out rule):
// int (*fp)(int, int)
// fp is a pointer to a function taking (int, int) returning int

int add(int a, int b) { return a + b; }
int mul(int a, int b) { return a * b; }

int (*operation)(int, int);  // declare function pointer
operation = add;              // assign (no & needed — function decays to pointer)
int result = operation(3, 4); // call: result = 7
operation = mul;
result = operation(3, 4);     // result = 12
```

### Dispatch Table (Virtual Method Table)

```c
typedef struct {
    void (*speak)(void);   // function pointer in struct
    void (*move)(void);
    const char *name;
} Animal;

void dog_speak(void) { puts("Woof!"); }
void cat_speak(void) { puts("Meow!"); }
void dog_move(void)  { puts("Dog runs"); }
void cat_move(void)  { puts("Cat slinks"); }

Animal dog = { .speak = dog_speak, .move = dog_move, .name = "Dog" };
Animal cat = { .speak = cat_speak, .move = cat_move, .name = "Cat" };

Animal *animals[] = { &dog, &cat, &dog };
for (int i = 0; i < 3; i++) {
    animals[i]->speak();  // polymorphic dispatch!
}
// This is exactly how C++ vtables work!
```

### qsort — The Classic Callback

```c
#include <stdlib.h>

int compare_ints(const void *a, const void *b) {
    return (*(int*)a - *(int*)b);  // negative, 0, or positive
}

int arr[] = {5, 2, 8, 1, 9, 3};
qsort(arr, 6, sizeof(int), compare_ints);
// arr is now: {1, 2, 3, 5, 8, 9}
```

### Go Function Values and Closures

```go
// Functions are first-class values in Go:
add := func(a, b int) int { return a + b }
result := add(3, 4)  // 7

// Closures capture surrounding variables:
func makeCounter() func() int {
    count := 0
    return func() int {
        count++   // captures 'count' from enclosing scope
        return count
    }
}

counter := makeCounter()
fmt.Println(counter())  // 1
fmt.Println(counter())  // 2
fmt.Println(counter())  // 3
// Each call to makeCounter() creates a NEW independent counter
```

### Rust Closures and Function Pointers

```rust
// Three closure traits:
// Fn:     captures by reference, can be called multiple times
// FnMut:  captures by mutable reference, can be called multiple times
// FnOnce: captures by value (moves), can only be called once

// Function pointer (fn): points to a specific function, no captured environment
let fp: fn(i32, i32) -> i32 = |a, b| a + b;

// Closure with capture:
let multiplier = 3;
let mul = |x| x * multiplier;  // captures 'multiplier' by reference (Fn)
println!("{}", mul(5));  // 15

// Higher-order functions:
fn apply<F: Fn(i32) -> i32>(f: F, x: i32) -> i32 {
    f(x)
}

let double = |x| x * 2;
println!("{}", apply(double, 5));  // 10

// Iterator combinators (common in idiomatic Rust):
let sum: i32 = (1..=10)
    .filter(|x| x % 2 == 0)   // keep evens
    .map(|x| x * x)            // square them
    .sum();                    // = 4+16+36+64+100 = 220
```

---

# 13. Bitwise Operations and Endianness

## Bitwise Operators

```c
// Operators:
// & AND   | OR   ^ XOR   ~ NOT   << left shift   >> right shift

unsigned int a = 0b10110110;  // = 182
unsigned int b = 0b01101101;  // = 109

a & b  // 0b00100100 = 36   (1 only where BOTH are 1)
a | b  // 0b11111111 = 255  (1 where EITHER is 1)
a ^ b  // 0b11011011 = 219  (1 where they DIFFER)
~a     // 0b01001001 = 73   (flip all bits, including sign bit!)
a << 2 // 0b11011000 = 216  (shift left 2, multiply by 4)
a >> 1 // 0b01011011 = 91   (shift right 1, divide by 2 for unsigned)
```

### Bit Manipulation Techniques

```c
// Set bit n:
value |= (1 << n);

// Clear bit n:
value &= ~(1 << n);

// Toggle bit n:
value ^= (1 << n);

// Test bit n:
if (value & (1 << n)) { /* bit n is set */ }

// Extract bits [high:low] (inclusive):
// E.g., bits 3..1 of value:
uint32_t mask = ((1 << (3-1+1)) - 1) << 1;  // = 0b0000...01110
uint32_t extracted = (value & mask) >> 1;

// Power of 2 check:
int is_power_of_two = (n > 0) && (n & (n-1)) == 0;
// Explanation: 8 = 1000, 7 = 0111, 8&7 = 0000

// Clear lowest set bit:
value &= value - 1;
// 12 = 1100 → 12 & 11 = 1100 & 1011 = 1000 (cleared bit 2)

// Isolate lowest set bit:
lowest = value & (-value);  // -value = ~value + 1

// Count set bits (Brian Kernighan's algorithm):
int count = 0;
while (value) {
    value &= value - 1;  // clear lowest set bit
    count++;
}
```

### Arithmetic vs Logical Shifts

```
Unsigned (logical shift right): fills vacated bits with 0
0b10110000 >> 2 = 0b00101100   (0s inserted at top)

Signed (arithmetic shift right): fills with sign bit (implementation-defined in C, but typical)
0b10110000 (negative as int8) >> 2 = 0b11101100  (1s inserted at top, preserves sign)

Left shift: always fills with 0
0b00110000 << 2 = 0b11000000  (may cause overflow for signed)
```

### Endianness

Endianness is the order in which bytes are stored for multi-byte values.

```
Value: 0x12345678 (hex) stored at address 0x1000

Little-endian (x86, x86-64, ARM default):
Address: 0x1000  0x1001  0x1002  0x1003
         +------+------+------+------+
         |  78  |  56  |  34  |  12  |   LSB (least significant byte) first
         +------+------+------+------+
         low addr ←                → high addr

Big-endian (network byte order, some RISC):
Address: 0x1000  0x1001  0x1002  0x1003
         +------+------+------+------+
         |  12  |  34  |  56  |  78  |   MSB (most significant byte) first
         +------+------+------+------+
         low addr ←                → high addr
```

**Detecting endianness:**
```c
#include <stdint.h>

int is_little_endian(void) {
    uint32_t x = 1;
    return *(uint8_t*)&x == 1;
    // On little-endian: byte at lowest address = 0x01 (LSB of 0x00000001)
    // On big-endian: byte at lowest address = 0x00 (MSB of 0x00000001)
}

// Using union:
union {
    uint32_t i;
    uint8_t bytes[4];
} u = { .i = 1 };
int little = u.bytes[0] == 1;
```

**Network byte order:** TCP/IP uses big-endian. Use `htons()`/`ntohs()` (host-to-network-short, etc.) for cross-platform network code.

```c
#include <arpa/inet.h>
uint16_t port = htons(8080);  // convert to network byte order
uint32_t addr = htonl(0xC0A80001);  // 192.168.0.1 in network order
```

### Go Bit Operations

```go
import "math/bits"

x := uint32(0b10110110)

// Set/clear/test bits:
x |= 1 << 3   // set bit 3
x &^= 1 << 3  // clear bit 3 (&^ is bit-clear = AND NOT)
x ^= 1 << 3   // toggle bit 3
set := x & (1 << 3) != 0  // test bit 3

// Standard library functions:
bits.OnesCount32(x)    // count set bits
bits.Len32(x)          // position of highest set bit + 1
bits.RotateLeft32(x, 5) // rotate left by 5
bits.Reverse32(x)      // reverse all bits
```

### Rust Bit Operations

```rust
let x: u32 = 0b10110110;

// All C-style operators work:
let a = x | 0xFF;
let b = x & 0x0F;
let c = x ^ 0b11111111;
let d = !x;          // bitwise NOT (not logical !)
let e = x << 2;
let f = x >> 1;      // for unsigned: logical shift; signed: arithmetic

// Built-in methods:
x.count_ones()       // popcount
x.count_zeros()
x.leading_zeros()
x.trailing_zeros()
x.rotate_left(3)
x.swap_bytes()       // change endianness
x.to_be()            // to big-endian bytes
x.to_le()            // to little-endian bytes
u32::from_be(x)      // from big-endian bytes
```

---

# 14. Go: Goroutines and the GMP Scheduler

## What is a Goroutine?

A goroutine is a lightweight, cooperatively-and-preemptively scheduled thread of execution managed by the Go runtime. Key properties:

- Start with ~2KB stack (grows dynamically, up to 1GB default max)
- Multiplexed onto OS threads by the Go scheduler
- Millions can exist simultaneously
- Not 1:1 with OS threads (like Python's threading) — many goroutines per OS thread

### The GMP Model

Go's scheduler uses three entities:

```
G = Goroutine (green thread, user-space)
M = Machine (OS thread, kernel-level thread)
P = Processor (logical CPU, controls parallelism)

Relationships:
- GOMAXPROCS sets number of P's (default: number of CPU cores)
- Each P has a local run queue of G's
- Each M is attached to one P at a time (while running Go code)
- When M blocks (syscall), P detaches and finds another M

Architecture:
+-----+   +-----+   +-----+     ← P's (logical processors)
| P0  |   | P1  |   | P2  |
|LRQ0 |   |LRQ1 |   |LRQ2 |    ← Local Run Queues
+--+--+   +--+--+   +--+--+
   |         |         |
+--+--+   +--+--+   +--+--+
| M0  |   | M1  |   | M2  |     ← OS threads
+-----+   +-----+   +-----+

Global Run Queue (GRQ): overflow of local run queues

         +--+--+--+--+--+
GRQ:     | G | G | G | G |...
         +--+--+--+--+--+
```

### Goroutine States

```
Goroutine state machine:

  [Created] ──────────────────→ [Runnable]
                                    │
                          P picks it up
                                    │
                                    ▼
                               [Running]
                                    │
          ┌─────────────────────────┼──────────────────────┐
          │                         │                      │
          ▼                         ▼                      ▼
     [Waiting]                [Runnable]            [Dead/Zombie]
  (chan recv,                 (preempted,
   syscall, sleep)            time slice end)
          │
          │ event occurs
          ▼
      [Runnable]
```

### Work Stealing

When a P's local queue is empty, it steals half the goroutines from another P's queue:

```
Before steal:
P0 LRQ: [G1] [G2] [G3] [G4]   (4 goroutines)
P1 LRQ: []                     (empty, idle)

After P1 steals from P0:
P0 LRQ: [G1] [G2]             (P0 keeps half)
P1 LRQ: [G3] [G4]             (P1 stole half)
```

### runtime.Gosched() — Explicit Yield

```go
package main

import (
    "fmt"
    "runtime"
    "sync"
)

func main() {
    var wg sync.WaitGroup
    
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for j := 0; j < 5; j++ {
                fmt.Printf("goroutine %d: iteration %d\n", id, j)
                runtime.Gosched()  // yield: give other goroutines a chance
            }
        }(i)
    }
    
    wg.Wait()
}
```

What `runtime.Gosched()` does internally:
1. Saves goroutine state (registers, stack pointer).
2. Changes goroutine status from `_Grunning` to `_Grunnable`.
3. Puts goroutine back in local run queue (front or back).
4. Calls `schedule()` to pick next runnable goroutine.

### Go Preemption (Go 1.14+)

Before Go 1.14, goroutines were only preempted at function call points. A tight loop could starve other goroutines:

```go
// Pre-1.14: could hog CPU forever
go func() {
    for {
        x++  // no function calls, no preemption points
    }
}()
```

Go 1.14 added **asynchronous preemption** via signals (SIGURG on Unix). The runtime sends a signal to the OS thread running a goroutine that hasn't yielded in a while, forcing a scheduling point.

### Channel-Based Generator (Python-style yield)

```go
func fibonacci() <-chan int {
    ch := make(chan int)
    go func() {
        a, b := 0, 1
        for {
            ch <- a       // "yield" a
            a, b = b, a+b // advance state
        }
    }()
    return ch
}

func main() {
    fib := fibonacci()
    for i := 0; i < 10; i++ {
        fmt.Println(<-fib)
    }
}

// Output: 0 1 1 2 3 5 8 13 21 34
```

How it works:
- `ch <- a` BLOCKS until someone receives.
- This is the yield point — goroutine is suspended.
- Receiving goroutine unblocks the producer.
- Natural, lightweight coroutine semantics via channels.

### Go 1.22+ Range-Over-Function (New Iterator Protocol)

```go
// iter.Seq is func(yield func(V) bool)
// yield returns false if caller wants to stop

func fibonacci(yield func(int) bool) {
    a, b := 0, 1
    for {
        if !yield(a) {
            return  // caller broke the loop
        }
        a, b = b, a+b
    }
}

// Usage with range (Go 1.22+):
for v := range fibonacci {
    if v > 100 { break }
    fmt.Println(v)
}
```

### Rust Async/Await — Stackless Coroutines

```rust
use tokio::time::{sleep, Duration};

async fn task(id: u32) {
    for i in 0..5 {
        println!("task {}: iteration {}", id, i);
        sleep(Duration::from_millis(1)).await;  // yield to runtime
    }
}

#[tokio::main]
async fn main() {
    let t1 = tokio::spawn(task(1));
    let t2 = tokio::spawn(task(2));
    let _ = tokio::join!(t1, t2);  // run both concurrently
}
```

Rust async works by compiling async functions into **state machines**:

```
async fn task() compiles to roughly:

enum TaskState {
    Start,
    WaitingSleep { fut: SleepFuture },
    Done,
}

impl Future for TaskState {
    fn poll(&mut self, cx: &mut Context) -> Poll<()> {
        match self {
            Start => {
                // do first bit of work
                *self = WaitingSleep { fut: sleep(...) };
                Poll::Pending  // yield
            }
            WaitingSleep { fut } => {
                if fut.poll(cx).is_ready() {
                    *self = Done;
                    Poll::Ready(())
                } else {
                    Poll::Pending  // still waiting
                }
            }
        }
    }
}
```

No stack allocation needed — all state lives in the enum (on heap or caller's stack).

---

# 15. Go: Channels — Internal Design

## What is a Channel?

A channel is a typed, concurrent-safe communication pipe between goroutines. It encapsulates:
- A ring buffer (for buffered channels)
- A mutex
- Wait queues for senders and receivers

```
Unbuffered channel: make(chan int)
  Sender BLOCKS until receiver is ready.
  Receiver BLOCKS until sender is ready.
  Rendezvous semantics.

  G1 (sender)    channel    G2 (receiver)
       |             |            |
  ch <- 42 ────────→|←───── <-ch |
  (blocks)      handoff      (unblocks)

Buffered channel: make(chan int, 4)
  +------+------+------+------+
  |  42  |  17  |      |      |   ring buffer, capacity=4
  +------+------+------+------+
      ^                  ^
    sendx (write here)  recvx (read from here)
  
  Sender only blocks when buffer is FULL.
  Receiver only blocks when buffer is EMPTY.
```

### hchan Internal Structure

```go
// Simplified from Go runtime source (runtime/chan.go):
type hchan struct {
    qcount   uint           // number of elements in buffer
    dataqsiz uint           // capacity of circular buffer
    buf      unsafe.Pointer // pointer to circular buffer
    elemsize uint16         // size of one element
    closed   uint32         // 1 if channel is closed
    sendx    uint           // send index in buffer
    recvx    uint           // receive index in buffer
    recvq    waitq          // list of goroutines waiting to receive
    sendq    waitq          // list of goroutines waiting to send
    lock     mutex          // protects all fields
}
```

### Channel Operations State Machine

```
ch := make(chan int, 2)  // buffered, capacity 2

State: empty (qcount=0)
  receive → block (add to recvq)
  send    → put in buffer (qcount=1)

State: partially full (0 < qcount < cap)
  receive → take from buffer (qcount--)
  send    → put in buffer (qcount++)

State: full (qcount == cap)
  receive → take from buffer (qcount--)
  send    → block (add to sendq)

State: closed
  receive → drain buffer, then return zero value + false
  send    → panic!
```

### Select — Non-blocking Multi-channel Operations

```go
select {
case v := <-ch1:
    fmt.Println("received from ch1:", v)
case ch2 <- 42:
    fmt.Println("sent to ch2")
case <-time.After(time.Second):
    fmt.Println("timeout")
default:
    fmt.Println("no channel ready")
}
```

`select` with multiple ready channels chooses uniformly at random — this is a deliberate design choice to prevent starvation.

### Channel Patterns

```go
// Done channel / cancellation:
func worker(done <-chan struct{}) {
    for {
        select {
        case <-done:
            return  // stop working
        default:
            doWork()
        }
    }
}

done := make(chan struct{})
go worker(done)
time.Sleep(time.Second)
close(done)  // signal all workers to stop

// Fan-out: distribute work to multiple goroutines
func fanOut(in <-chan int, n int) []<-chan int {
    outs := make([]<-chan int, n)
    for i := range outs {
        out := make(chan int)
        outs[i] = out
        go func() {
            for v := range in {
                out <- v
            }
            close(out)
        }()
    }
    return outs
}

// Pipeline:
func generate(nums ...int) <-chan int { ... }
func square(in <-chan int) <-chan int { ... }

c := generate(2, 3, 4)
out := square(square(c))
for v := range out {
    fmt.Println(v)  // 16, 81, 256
}
```

---

# 16. Go: File I/O

## Reading Large Files — The Right Way

```
Wrong approach (small files only):
data, _ := os.ReadFile("huge.log")  
→ entire file into RAM
→ 10GB file = 10GB RAM = OOM crash

Correct approach (streaming):
file → bufio.Reader → process one chunk at a time
→ constant memory usage regardless of file size
```

### bufio.Scanner — Line-by-Line

```go
package main

import (
    "bufio"
    "fmt"
    "io"
    "os"
)

func processLines(filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    
    // If lines may be > 64KB, set custom buffer:
    buf := make([]byte, 0, 64*1024)
    scanner.Buffer(buf, 10*1024*1024)  // up to 10MB per line
    
    lineNum := 0
    for scanner.Scan() {
        line := scanner.Text()  // returns string (allocates)
        // or: scanner.Bytes() returns []byte (zero-copy, but slice is reused)
        lineNum++
        fmt.Printf("%d: %s\n", lineNum, line)
    }
    
    return scanner.Err()  // always check this!
}
```

### bufio.Reader — Lower Level, More Control

```go
func processReader(filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    // 64KB buffer (fewer syscalls than default 4KB)
    reader := bufio.NewReaderSize(file, 64*1024)

    for {
        line, err := reader.ReadString('\n')
        
        if len(line) > 0 {
            // process line (may or may not end with \n)
            process(line)
        }
        
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
    }
    return nil
}
```

### Chunked Reading — Binary Files

```go
func processChunks(filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    // Allocate buffer ONCE, outside the loop:
    buf := make([]byte, 4*1024)  // 4KB chunks

    for {
        n, err := file.Read(buf)
        
        if n > 0 {
            chunk := buf[:n]
            processBytes(chunk)
        }
        
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
    }
    return nil
}
```

### io.Reader Interface — The Abstraction

```go
// io.Reader is the fundamental I/O interface:
type Reader interface {
    Read(p []byte) (n int, err error)
}

// Everything that can produce bytes implements io.Reader:
// - *os.File
// - *bufio.Reader
// - *bytes.Buffer
// - *strings.Reader
// - net.Conn (network connection)
// - http.Response.Body
// - gzip.Reader
// - tls.Conn

// This means you can write functions that work with ANY source:
func processData(r io.Reader) error {
    buf := make([]byte, 4096)
    for {
        n, err := r.Read(buf)
        if n > 0 { process(buf[:n]) }
        if err == io.EOF { return nil }
        if err != nil { return err }
    }
}

// Use with file:
f, _ := os.Open("data.bin")
processData(f)

// Use with network:
processData(conn)

// Use with in-memory bytes (testing):
processData(strings.NewReader("test data"))
```

### Random Access and Seek

```go
file, _ := os.Open("data.bin")
defer file.Close()

// Seek to position 1024 from start:
_, err := file.Seek(1024, io.SeekStart)   // = os.SEEK_SET

// Seek relative to current position:
file.Seek(100, io.SeekCurrent)   // = os.SEEK_CUR

// Seek from end (read last 100 bytes):
file.Seek(-100, io.SeekEnd)      // = os.SEEK_END

// Read current position:
pos, _ := file.Seek(0, io.SeekCurrent)
fmt.Println("at position:", pos)
```

### Memory Mapping (mmap) — Maximum Performance

```go
import "golang.org/x/exp/mmap"

func readMapped(filename string) error {
    r, err := mmap.Open(filename)
    if err != nil {
        return err
    }
    defer r.Close()

    // Access file as a byte slice (backed by OS page cache)
    // No explicit Read() needed — access like a slice:
    buf := make([]byte, 1024)
    n, _ := r.ReadAt(buf, 0)  // read 1024 bytes at offset 0
    fmt.Println(n, "bytes read")
    return nil
}
```

```
mmap internals:
OS page table maps file pages into virtual address space.
When you access a page not yet loaded:
  → page fault
  → kernel loads page from disk/page cache into RAM
  → page table updated
  → program continues

Benefit: no user/kernel buffer copies for read paths.
Database engines (SQLite, PostgreSQL, LMDB) use mmap heavily.
```

---

# 17. Rust: Ownership, Borrowing, Lifetimes

## Ownership — The Central Innovation

Rust's memory safety without GC is achieved through **ownership rules** enforced at compile time:

```
Rules:
1. Every value has exactly ONE owner.
2. When the owner goes out of scope, the value is dropped (freed).
3. Ownership can be transferred (moved) but not duplicated (unless type is Copy).
```

### Move Semantics

```rust
let s1 = String::from("hello");  // s1 owns the String
let s2 = s1;                     // s1 MOVED to s2 — s1 is now invalid!

// Memory layout:
// s1 (before move):  [ptr=0x100 | len=5 | cap=5]
//                          |
//                    heap: [h][e][l][l][o]
//
// After move to s2:  s1 = invalid (stack value zeroed/moved)
//                    s2 = [ptr=0x100 | len=5 | cap=5]
//                               |
//                         heap: [h][e][l][l][o]
// No heap copy! Just the 24-byte header was moved.

println!("{}", s1);  // COMPILE ERROR: value used after move
println!("{}", s2);  // OK
// When s2 goes out of scope, heap is freed once (no double-free possible!)
```

### Clone — Explicit Deep Copy

```rust
let s1 = String::from("hello");
let s2 = s1.clone();  // explicit deep copy — heap bytes duplicated

// Now:
// s1: [ptr=0x100 | 5 | 5] → heap: [h][e][l][l][o]
// s2: [ptr=0x200 | 5 | 5] → heap: [h][e][l][l][o]  (independent copy)

println!("{} {}", s1, s2);  // both valid, both dropped independently
```

### Copy Types — Implicit Copy on "Move"

Types that implement `Copy` are cheaply duplicated (stack-only, no heap):

```rust
let x: i32 = 42;
let y = x;  // x is COPIED (not moved) because i32: Copy
println!("{} {}", x, y);  // both valid!

// Copy types: all integer types, float types, bool, char, [T; N] if T: Copy
// Non-Copy types: String, Vec<T>, Box<T>, any type with heap allocation
```

### Borrowing — References Without Ownership Transfer

```rust
fn print_length(s: &String) {  // borrows s — no ownership transfer
    println!("{}", s.len());
}  // borrow ends here — nothing is freed

let s = String::from("hello");
print_length(&s);  // lend s to function
println!("{}", s); // s still valid! still owned here
```

### Mutable References — Exclusive Access

```rust
fn append_world(s: &mut String) {
    s.push_str(", world");
}

let mut s = String::from("hello");
append_world(&mut s);
println!("{}", s);  // "hello, world"

// THE RULE: only ONE mutable reference at a time
let r1 = &mut s;
let r2 = &mut s;  // COMPILE ERROR: cannot borrow `s` as mutable more than once
// This prevents data races at compile time!

// Also cannot mix mutable and immutable:
let r1 = &s;     // immutable borrow
let r2 = &s;     // OK — multiple immutable borrows allowed
let r3 = &mut s; // COMPILE ERROR while r1, r2 are still in scope
```

### Lifetimes — Preventing Dangling References

Lifetimes are the compiler's way of ensuring references never outlive the data they point to:

```rust
// This function's return reference borrows from input:
fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {
    if x.len() > y.len() { x } else { y }
}
// 'a says: returned reference lives at most as long as the shorter of x and y

let string1 = String::from("long string");
let result;
{
    let string2 = String::from("xyz");
    result = longest(string1.as_str(), string2.as_str());
    println!("{}", result);  // OK — string2 still alive
}
// println!("{}", result);  // ERROR: string2 dropped, result is dangling

// The compiler catches this without runtime checks!
```

Lifetime diagram:
```
string1: ───────────────────────────────────────→ (alive)
string2: ─────────────────→ (dropped at })
result:  borrows from min(string1, string2)
         valid only while BOTH alive: ──────────→ (but constrained by string2)

If we use result after }, it's after string2's lifetime → compile error.
```

### The Borrow Checker — Mental Model

```
At every point in the code, the borrow checker asks:
  "Is there any way a reference could outlive the data it points to?"
  "Are there conflicting mutable and immutable accesses?"

If the answer is yes → COMPILE ERROR.

This eliminates at compile time:
  - Use-after-free
  - Dangling pointers
  - Double frees
  - Data races (in safe code)
  - Iterator invalidation
  - Buffer overflows (combined with bounds checking)
```

---

# 18. Rust: Memory Safety Without a GC

## Rust Comparison vs C and Go

```
Feature              C          Go         Rust
───────────────────────────────────────────────────────
Memory management    Manual     GC         Ownership/RAII
Null pointers        Yes (UB)   Yes (panic) No (Option<T>)
Dangling pointers    Yes (UB)   No (GC)    No (borrow checker)
Data races           Yes (UB)   Possible   No (safe code)
Buffer overflow      Yes (UB)   Panic      Panic (bounds check)
Uninitialized mem    Yes (UB)   No (zeroed) No (init required)
Runtime cost         Zero       GC pauses  Zero (no GC)
Compile-time cost    Low        Low        High (borrow checker)
```

### RAII — Resource Acquisition Is Initialization

```rust
// Rust automatically inserts drop() calls at end of scope:
{
    let file = File::open("data.txt")?;    // acquires resource
    let v = Vec::<i32>::with_capacity(100); // acquires heap memory
    let lock = mutex.lock().unwrap();       // acquires mutex lock
    
    // ... use resources ...
    
}   // ← compiler inserts: drop(lock), drop(v), drop(file) in REVERSE order
    // lock released → heap freed → file closed
    // Guaranteed even if we return early or panic!
```

Equivalent C++ RAII (Go has no equivalent — uses defer):
```go
// Go uses defer (manual, called in LIFO order):
file, _ := os.Open("data.txt")
defer file.Close()  // called when function returns

mu.Lock()
defer mu.Unlock()  // called when function returns
```

```c
// C has no RAII — you must manually free:
FILE *f = fopen("data.txt", "r");
void *data = malloc(1000);
// ... if we return early, must not forget:
// fclose(f);
// free(data);
// Manual tracking is error-prone!
```

### Smart Pointers in Rust

```rust
// Box<T> — heap allocation, single owner
let b = Box::new(42);
println!("{}", *b);   // deref: prints 42
// b dropped here → heap freed

// Rc<T> — reference counted, multiple owners (single-threaded)
use std::rc::Rc;
let a = Rc::new(String::from("hello"));
let b = Rc::clone(&a);  // increments reference count
// a: count=2, b: count=2
// when both dropped → count=0 → heap freed
// CANNOT use across threads

// Arc<T> — atomic reference counted (thread-safe version of Rc)
use std::sync::Arc;
let a = Arc::new(String::from("hello"));
let b = Arc::clone(&a);
// can be sent across threads

// RefCell<T> — runtime borrow checking (escape hatch for Rc internals)
use std::cell::RefCell;
let x = RefCell::new(42);
*x.borrow_mut() += 1;  // mutable borrow at runtime
println!("{}", x.borrow());  // immutable borrow
```

### unsafe Rust — Opting Out of Guarantees

```rust
// unsafe blocks allow:
// 1. Dereferencing raw pointers
// 2. Calling unsafe functions
// 3. Accessing mutable statics
// 4. Implementing unsafe traits

let mut x = 42;
let raw: *mut i32 = &mut x as *mut i32;

unsafe {
    *raw = 99;  // raw pointer dereference
    println!("{}", *raw);
}

// unsafe does NOT turn off the borrow checker everywhere
// it ONLY allows the 5 unsafe operations listed above
// Other Rust guarantees (type system, etc.) still apply
```

---

# 19. Data Structures — Internal Implementation

## Arrays and Slices

### Static Array

```
C: int arr[5] = {1, 2, 3, 4, 5};

Memory (contiguous):
Address: 1000  1004  1008  1012  1016
         +----+----+----+----+----+
         |  1 |  2 |  3 |  4 |  5 |
         +----+----+----+----+----+

Access: O(1) — direct computation: address = base + index * element_size
Insert/delete at middle: O(n) — must shift elements
Append (if resizing): amortized O(1) — double capacity strategy
```

## Linked Lists

### Singly Linked List

```
Structure:
[head] → [1 | next] → [2 | next] → [3 | next] → NULL

Each node:
+-------+------+
| data  | next |   next is pointer to next node (or NULL)
+-------+------+

Operations:
- Access by index: O(n) — must traverse from head
- Insert at head: O(1)
- Insert at tail: O(n) without tail pointer, O(1) with
- Delete (with pointer to node): O(1)
- Delete by value: O(n) (must search first)
- Search: O(n)
```

C implementation:
```c
typedef struct Node {
    int data;
    struct Node *next;
} Node;

Node *insert_front(Node *head, int val) {
    Node *new = malloc(sizeof(Node));
    new->data = val;
    new->next = head;  // new node points to old head
    return new;        // new node is the new head
}

// Before: head → [1] → [2] → [3] → NULL
// After insert_front(head, 0): head → [0] → [1] → [2] → [3] → NULL
```

Go implementation:
```go
type Node struct {
    data int
    next *Node
}

func insertFront(head *Node, val int) *Node {
    return &Node{data: val, next: head}
}
```

Rust implementation:
```rust
// Linked lists in Rust are notoriously tricky due to ownership:
// Each node must own its next node.
use std::boxed::Box;

struct Node {
    data: i32,
    next: Option<Box<Node>>,  // Option: either Some(next_node) or None
}

fn insert_front(head: Option<Box<Node>>, val: i32) -> Box<Node> {
    Box::new(Node { data: val, next: head })
}
```

### Doubly Linked List

```
[NULL ← [1 | prev | next] ↔ [2 | prev | next] ↔ [3 | prev | next] → NULL]
          ^head                                          ^tail

Each node: +------+------+------+
           | prev | data | next |
           +------+------+------+
```

## Stack (LIFO)

```
Operations:
  push(x): add to top
  pop():   remove from top
  peek():  see top without removing

Array-based stack:
  top=2
  +---+---+---+---+---+
  | 1 | 2 | 3 | . | . |
  +---+---+---+---+---+
         ↑ top

  push(4): top=3, arr[3]=4
  pop(): returns arr[3]=4, top=2

  All operations O(1)

Call stack is a stack: push on function call, pop on return.
```

```c
#define MAX 100
int stack[MAX];
int top = -1;

void push(int x) { stack[++top] = x; }
int  pop(void)   { return stack[top--]; }
int  peek(void)  { return stack[top]; }
int  empty(void) { return top == -1; }
```

## Queue (FIFO)

```
Circular array queue:
  capacity=5, head=1, tail=3, size=2

  +---+---+---+---+---+
  | . | A | B | . | . |
  +---+---+---+---+---+
        ↑           ↑
       head        tail
  
  enqueue(C): arr[tail]=C, tail=(tail+1)%cap
  dequeue(): returns arr[head], head=(head+1)%cap

  Modular arithmetic makes it circular:
  When tail reaches end, it wraps to beginning.
```

```c
#define CAP 100
int q[CAP];
int head = 0, tail = 0, size = 0;

void enqueue(int x) {
    q[tail] = x;
    tail = (tail + 1) % CAP;
    size++;
}
int dequeue(void) {
    int x = q[head];
    head = (head + 1) % CAP;
    size--;
    return x;
}
```

## Hash Map (Hash Table)

```
Hash map with separate chaining (handles collisions):

hash(key) % capacity = bucket index

Buckets array:
[0]: → NULL
[1]: → ["apple" | 5] → ["cat" | 3] → NULL   (collision: same bucket)
[2]: → NULL
[3]: → ["banana" | 6] → NULL
[4]: → ["dog" | 3] → NULL
[5]: → NULL
[6]: → ["egg" | 3] → NULL

insert("apple", 5):
  hash("apple") % 7 = 1
  prepend to bucket[1]

lookup("cat"):
  hash("cat") % 7 = 1
  walk chain: "apple"? no. "cat"? yes! return 3.

Load factor = num_elements / num_buckets
  When load_factor > 0.75 → resize (double buckets, rehash all)
```

Open addressing (probing) — alternative collision strategy:
```
Linear probing: if bucket[h] occupied, try h+1, h+2, ...

hash("apple") = 3, hash("banana") = 3

Insert "apple":
  bucket[3] = "apple"

Insert "banana":
  bucket[3] occupied → try bucket[4]
  bucket[4] = "banana"

Lookup "banana":
  hash = 3, bucket[3] = "apple" (not "banana")
  probe: bucket[4] = "banana" ✓

Deletion is tricky: must mark as "deleted" (tombstone), not empty,
to preserve probe chains.
```

C implementation (open addressing):
```c
#define SIZE 64
typedef struct { char *key; int val; int used; } Entry;
Entry table[SIZE];

int hash(const char *key) {
    int h = 0;
    while (*key) h = h * 31 + *key++;
    return ((h % SIZE) + SIZE) % SIZE;
}

void set(const char *key, int val) {
    int h = hash(key);
    while (table[h].used && strcmp(table[h].key, key) != 0)
        h = (h + 1) % SIZE;
    table[h] = (Entry){ .key = strdup(key), .val = val, .used = 1 };
}

int get(const char *key) {
    int h = hash(key);
    while (table[h].used) {
        if (strcmp(table[h].key, key) == 0) return table[h].val;
        h = (h + 1) % SIZE;
    }
    return -1;  // not found
}
```

## Binary Tree and BST

```
Binary Search Tree invariant:
  - All nodes in left subtree have keys < node's key
  - All nodes in right subtree have keys > node's key

Example BST:
            5
           / \
          3   8
         / \ / \
        1  4 6  9
           
Insert 7: goes to right of 6 (6 < 7 < 8)
            5
           / \
          3   8
         / \ / \
        1  4 6  9
            \ 
             7

Traversals:
  In-order (L, root, R):   1 3 4 5 6 7 8 9  (sorted order!)
  Pre-order (root, L, R):  5 3 1 4 8 6 7 9
  Post-order (L, R, root): 1 4 3 7 6 9 8 5
```

C implementation:
```c
typedef struct Node {
    int key;
    struct Node *left, *right;
} Node;

Node *insert(Node *root, int key) {
    if (!root) {
        Node *n = malloc(sizeof(Node));
        n->key = key; n->left = n->right = NULL;
        return n;
    }
    if (key < root->key) root->left  = insert(root->left,  key);
    else if (key > root->key) root->right = insert(root->right, key);
    return root;
}

void inorder(Node *root) {
    if (!root) return;
    inorder(root->left);
    printf("%d ", root->key);
    inorder(root->right);
}
```

## Heap (Priority Queue)

```
Max-heap: parent always >= children
Stored as array (complete binary tree property):

           10
          /  \
         9    7
        / \  / \
       8   6 4  5
      / \
     3   2

Array: [10, 9, 7, 8, 6, 4, 5, 3, 2]
Index:  [0, 1, 2, 3, 4, 5, 6, 7, 8]

For node at index i:
  parent:       (i-1) / 2
  left child:   2*i + 1
  right child:  2*i + 2

insert(11):
  1. Append to end: [..., 2, 11]  index 9
  2. Sift up: parent[9] = index 4 (value 6) < 11 → swap
              parent[4] = index 1 (value 9) < 11 → swap
              parent[1] = index 0 (value 10) < 11 → swap
              at root → done
  
  Final:       11
              /  \
             10    7
            / \  / \
           8   9 4  5
          / \ /
         3  2 6

extract_max():
  1. Remove root (11)
  2. Move last element to root: [6, ...]  (6 was last)
  3. Sift down: compare with children, swap with larger child
  O(log n) — height of tree
```

## Graph Representations

```
Graph: 5 nodes (0-4), edges: 0-1, 0-2, 1-3, 2-3, 3-4

Adjacency Matrix (good for dense graphs):
    0  1  2  3  4
0 [ 0, 1, 1, 0, 0 ]
1 [ 1, 0, 0, 1, 0 ]
2 [ 1, 0, 0, 1, 0 ]
3 [ 0, 1, 1, 0, 1 ]
4 [ 0, 0, 0, 1, 0 ]
Space: O(V²), Edge check: O(1), Neighbor list: O(V)

Adjacency List (good for sparse graphs):
0: [1, 2]
1: [0, 3]
2: [0, 3]
3: [1, 2, 4]
4: [3]
Space: O(V+E), Edge check: O(degree), Neighbor list: O(degree)
```

---

# 20. Algorithms — Internal Mechanics

## Sorting Algorithms

### Quicksort — Partition-based

```
Quicksort([5, 3, 8, 1, 9, 2, 7, 4, 6])

Pick pivot = 6 (last element in Lomuto scheme)
Partition: elements < 6 to left, >= 6 to right

[5, 3, 1, 2, 4 | 6 | 8, 9, 7]
                 ^ pivot in final position

Recurse on [5, 3, 1, 2, 4] and [8, 9, 7]

[5, 3, 1, 2, 4] → pivot = 4
[3, 1, 2 | 4 | 5]
Recurse: [3, 1, 2] and [5]

Tree of recursion:
[5,3,8,1,9,2,7,4,6]
     /         \
[5,3,1,2,4]  [8,9,7]
   /    \      /   \
[3,1,2] [5] [8,9]  [7]
 / \         / \
[1,2][3]  [8] [9]

Complexity:
  Average: O(n log n)
  Worst: O(n²) — already sorted array with bad pivot choice
  Space: O(log n) — recursion stack
```

C implementation:
```c
void swap(int *a, int *b) { int t = *a; *a = *b; *b = t; }

int partition(int arr[], int lo, int hi) {
    int pivot = arr[hi];  // Lomuto: last element
    int i = lo - 1;
    for (int j = lo; j < hi; j++) {
        if (arr[j] <= pivot) {
            i++;
            swap(&arr[i], &arr[j]);
        }
    }
    swap(&arr[i+1], &arr[hi]);
    return i + 1;
}

void quicksort(int arr[], int lo, int hi) {
    if (lo < hi) {
        int p = partition(arr, lo, hi);
        quicksort(arr, lo, p - 1);
        quicksort(arr, p + 1, hi);
    }
}
```

### Merge Sort — Divide and Conquer

```
MergeSort([5, 3, 8, 1, 9, 2, 7, 4])

Split:
[5, 3, 8, 1] | [9, 2, 7, 4]
[5, 3] [8, 1] | [9, 2] [7, 4]
[5][3] [8][1] | [9][2] [7][4]

Merge up:
[3,5]  [1,8]   [2,9]  [4,7]
   [1,3,5,8]      [2,4,7,9]
         [1,2,3,4,5,7,8,9]

Merge step (two sorted halves):
[1,3,5,8] and [2,4,7,9]
Compare heads: 1 vs 2 → take 1
Compare heads: 3 vs 2 → take 2
Compare heads: 3 vs 4 → take 3
Compare heads: 5 vs 4 → take 4
...

Complexity: O(n log n) always (best, average, worst)
Space: O(n) — needs temporary array
Stable: yes (equal elements maintain relative order)
```

### Binary Search

```
Sorted array: [1, 3, 5, 7, 9, 11, 13, 15, 17]
Search for 11

lo=0, hi=8
mid = (0+8)/2 = 4, arr[4]=9 < 11 → search right half
lo=5, hi=8

mid = (5+8)/2 = 6, arr[6]=13 > 11 → search left half
lo=5, hi=5

mid = (5+5)/2 = 5, arr[5]=11 == 11 → found at index 5!

Each step halves the search space → O(log n)

Visualization:
[1, 3, 5, 7, 9, 11, 13, 15, 17]
                ^9              step 1: mid=4
                     ^13        step 2: mid=6
                  ^11           step 3: mid=5 → found!
```

```c
int binary_search(int arr[], int n, int target) {
    int lo = 0, hi = n - 1;
    while (lo <= hi) {
        int mid = lo + (hi - lo) / 2;  // avoid overflow vs (lo+hi)/2
        if (arr[mid] == target) return mid;
        if (arr[mid] < target)  lo = mid + 1;
        else                    hi = mid - 1;
    }
    return -1;  // not found
}
```

### BFS and DFS

```
Graph:
0 - 1 - 3
|       |
2 - - - 4

BFS from 0 (uses queue):
Queue: [0]
Visit 0, enqueue neighbors: [1, 2]
Visit 1, enqueue neighbors: [3]     Queue: [2, 3]
Visit 2, enqueue neighbors: [4]     Queue: [3, 4]
Visit 3, enqueue neighbors: [4]     Queue: [4]  (4 already seen)
Visit 4                             Queue: []
BFS order: 0 1 2 3 4
Explores level by level — shortest path!

DFS from 0 (uses stack / recursion):
Visit 0
  Visit 1
    Visit 3
      Visit 4 (backtrack: 4 done)
    (backtrack: 3 done)
  (backtrack: 1 done)
  Visit 2 (4 already visited)
DFS order: 0 1 3 4 2
Explores depth first — good for cycle detection, topological sort
```

```go
func bfs(graph [][]int, start int) []int {
    n := len(graph)
    visited := make([]bool, n)
    order := []int{}
    queue := []int{start}
    visited[start] = true

    for len(queue) > 0 {
        node := queue[0]
        queue = queue[1:]
        order = append(order, node)
        for _, neighbor := range graph[node] {
            if !visited[neighbor] {
                visited[neighbor] = true
                queue = append(queue, neighbor)
            }
        }
    }
    return order
}
```

### Dynamic Programming — Memoization

```
Fibonacci with memoization:

Without memo (exponential!):
          fib(5)
         /      \
      fib(4)    fib(3)
      /    \    /    \
  fib(3) fib(2) fib(2) fib(1)
  ...
Each fib(2) is recalculated multiple times!

With memo (linear):
First call fib(5):
  fib(4) not cached → compute
    fib(3) not cached → compute
      fib(2) not cached → compute → cache: 1
      fib(1) = 1
    cache: fib(3) = 2
    fib(2) = 1 (cached!)
  cache: fib(4) = 3
  fib(3) = 2 (cached!)
cache: fib(5) = 5

All subsequent calls: O(1) lookup.
Total time: O(n) instead of O(2^n).
```

```rust
use std::collections::HashMap;

fn fib(n: u64, memo: &mut HashMap<u64, u64>) -> u64 {
    if n <= 1 { return n; }
    if let Some(&v) = memo.get(&n) { return v; }
    let result = fib(n - 1, memo) + fib(n - 2, memo);
    memo.insert(n, result);
    result
}

fn main() {
    let mut memo = HashMap::new();
    for i in 0..10 {
        print!("{} ", fib(i, &mut memo));
    }  // 0 1 1 2 3 5 8 13 21 34
}
```

---

# 21. Operating System Concepts

## Processes vs Threads

```
Process: independent program with its own address space
Thread: a unit of execution WITHIN a process, shares address space

Process A:           Process B:
+---------------+    +---------------+
| Virtual Addr  |    | Virtual Addr  |   (separate address spaces)
| Space         |    | Space         |
| Text          |    | Text          |
| Data/BSS      |    | Data/BSS      |
| Heap          |    | Heap          |
| Stack (T1)    |    | Stack (T1)    |
+---------------+    +---------------+

Process A with threads:
+---------------------------------------+
| Virtual Address Space (shared)        |
| Text (shared)                         |
| Data/BSS (shared)                     |
| Heap (shared)                         |
+---------------------------------------+
| Stack T1  | Stack T2  | Stack T3  |   (each thread has own stack)
+-----------+-----------+-----------+

Threads share: heap, globals, file descriptors, code
Threads own:   stack, registers, thread-local storage (TLS)
```

## Virtual Memory and Page Tables

```
Virtual address → Physical address translation:

Virtual address (48-bit on x86-64):
+--------+--------+--------+--------+-----------+
| PML4   | PDPT   | PD     | PT     | Offset    |
| (9bit) | (9bit) | (9bit) | (9bit) | (12 bit)  |
+--------+--------+--------+--------+-----------+

PML4 = Page Map Level 4
PDPT = Page Directory Pointer Table
PD   = Page Directory
PT   = Page Table

Translation (4 levels of lookup):
CR3 register → PML4 table
  → PML4 entry (9 bits of VA)
  → PDPT entry
  → PD entry
  → PT entry
  → Physical page base address
  + Offset (12 bits = 4096 bytes per page)
  = Physical address

TLB (Translation Lookaside Buffer): cache of recent VA→PA translations.
Context switch → TLB flush (very expensive! ~1000 cycles)
```

## System Calls

```
Userspace to kernel transition (x86-64 Linux):

1. Program calls library function (e.g., write())
2. Library puts syscall number in RAX, args in RDI, RSI, RDX, R10, R8, R9
3. SYSCALL instruction → CPU switches to ring 0 (kernel mode)
4. Kernel finds handler via syscall table
5. Kernel executes (e.g., copies bytes from userspace to kernel buffer, then to disk)
6. SYSRET → CPU back to ring 3 (user mode)
7. Return value in RAX

Costs: typically 100-500ns per syscall (mode switch + TLB effects)
That's why buffered I/O (bufio, stdio) matters — fewer syscalls!
```

```c
// Direct syscall (write 5 bytes to stdout):
#include <unistd.h>
write(1, "hello", 5);  // fd=1 (stdout), buf, count

// Under the hood (Linux x86-64 assembly):
// mov rax, 1      ; SYS_write
// mov rdi, 1      ; fd = stdout
// lea rsi, [buf]  ; buffer address
// mov rdx, 5      ; count
// syscall         ; enter kernel
```

## Process Creation (fork/exec)

```c
#include <unistd.h>
#include <sys/wait.h>

pid_t pid = fork();  // duplicate current process

if (pid == 0) {
    // CHILD: pid == 0 in child
    execve("/bin/ls", args, env);  // replace with new program
    // exec never returns on success
} else if (pid > 0) {
    // PARENT: pid = child's PID
    int status;
    waitpid(pid, &status, 0);  // wait for child to finish
} else {
    // fork() failed (pid == -1)
    perror("fork");
}
```

```
fork() creates a copy of the process:

Parent process:          Child process:
+------------------+    +------------------+
| PID: 1000        |    | PID: 1001        |
| Text (shared)    |    | Text (shared)    |   (same physical pages, COW)
| Data (COW copy)  |    | Data (COW copy)  |   (copy-on-write)
| Heap (COW copy)  |    | Heap (COW copy)  |
| Stack (copy)     |    | Stack (copy)     |   (stack is copied)
+------------------+    +------------------+

Copy-on-Write: pages are shared until one process writes,
then the kernel creates a private copy for that process.
```

## Signals

```c
#include <signal.h>

void handler(int sig) {
    write(1, "caught SIGINT\n", 14);  // async-signal-safe only!
}

// Install signal handler:
signal(SIGINT, handler);   // simple but deprecated
// or:
struct sigaction sa = {
    .sa_handler = handler,
    .sa_flags = SA_RESTART  // restart interrupted syscalls
};
sigaction(SIGINT, &sa, NULL);

// Common signals:
// SIGINT (2):   Ctrl+C
// SIGTERM (15): kill command (graceful termination request)
// SIGKILL (9):  kill -9 (cannot be caught or ignored!)
// SIGSEGV (11): segmentation fault
// SIGFPE (8):   floating point exception
// SIGHUP (1):   terminal hangup (often used to reload config)
// SIGPIPE (13): write to broken pipe
```

---

# 22. Concurrency Fundamentals

## Race Conditions

```
Thread 1:               Thread 2:
read x (= 0)
                        read x (= 0)
x = x + 1              x = x + 1
write x (= 1)
                        write x (= 1)   ← lost update!

Final x = 1, expected x = 2.
This is a data race — undefined behavior in C.
```

## Mutex (Mutual Exclusion)

```c
#include <pthread.h>

int counter = 0;
pthread_mutex_t lock = PTHREAD_MUTEX_INITIALIZER;

void *increment(void *arg) {
    for (int i = 0; i < 100000; i++) {
        pthread_mutex_lock(&lock);    // acquire — blocks if already held
        counter++;                    // critical section: only one thread at a time
        pthread_mutex_unlock(&lock);  // release
    }
    return NULL;
}
```

```
Mutex state machine:
  UNLOCKED ──── lock() ────→ LOCKED
  LOCKED   ── unlock() ────→ UNLOCKED
  LOCKED   ──── lock() ────→ (blocked, wait in queue)

  When unlock: wake one waiting thread.

Deadlock example:
  Thread 1 holds A, waits for B
  Thread 2 holds B, waits for A
  → Both wait forever!

  Prevention: always acquire locks in the SAME ORDER.
```

### Go Mutex and sync Package

```go
import "sync"

var (
    counter int
    mu      sync.Mutex
)

func increment() {
    mu.Lock()
    defer mu.Unlock()  // always use defer to prevent lock leaks!
    counter++
}

// sync.RWMutex for readers-writer lock:
var rwmu sync.RWMutex

func readData() int {
    rwmu.RLock()         // multiple readers can hold simultaneously
    defer rwmu.RUnlock()
    return counter
}

func writeData(v int) {
    rwmu.Lock()          // exclusive write lock
    defer rwmu.Unlock()
    counter = v
}
```

### Rust Mutex — Safety Enforced by Type System

```rust
use std::sync::{Arc, Mutex};
use std::thread;

fn main() {
    let counter = Arc::new(Mutex::new(0));  // Arc: shared ownership; Mutex: safe mutation
    
    let handles: Vec<_> = (0..10).map(|_| {
        let counter = Arc::clone(&counter);
        thread::spawn(move || {
            let mut num = counter.lock().unwrap();  // lock() returns MutexGuard
            *num += 1;
            // MutexGuard drops here → unlock() called automatically!
        })
    }).collect();
    
    for h in handles { h.join().unwrap(); }
    println!("{}", *counter.lock().unwrap());  // 10
}

// Rust PREVENTS data races at compile time:
// - Data shared between threads MUST be Arc<Mutex<T>> or similar
// - Trying to share &mut T across threads → compile error
// - "Fearless concurrency"
```

## Atomic Operations

```c
#include <stdatomic.h>

_Atomic int counter = 0;
atomic_fetch_add(&counter, 1);  // atomic increment: no mutex needed
int val = atomic_load(&counter);

// Under the hood (x86):
// LOCK ADD [mem], 1    — atomic read-modify-write via bus lock or cache coherency
// Atomic: entire operation happens without interruption
// Much cheaper than mutex for simple counters
```

```go
import "sync/atomic"

var counter int64

// Atomic increment:
atomic.AddInt64(&counter, 1)
val := atomic.LoadInt64(&counter)
atomic.StoreInt64(&counter, 42)

// Compare-and-swap (CAS) — foundation of all lock-free algorithms:
old := atomic.LoadInt64(&counter)
swapped := atomic.CompareAndSwapInt64(&counter, old, old+1)
// swapped=true if counter was still 'old' and was updated to old+1
// swapped=false if counter changed between Load and CAS
```

## Condition Variables — Waiting for Conditions

```c
pthread_mutex_t mu = PTHREAD_MUTEX_INITIALIZER;
pthread_cond_t  cv = PTHREAD_COND_INITIALIZER;
int ready = 0;

// Producer:
pthread_mutex_lock(&mu);
ready = 1;
pthread_cond_signal(&cv);  // wake up one waiting thread
pthread_mutex_unlock(&mu);

// Consumer:
pthread_mutex_lock(&mu);
while (!ready) {             // MUST be a loop (spurious wakeups!)
    pthread_cond_wait(&cv, &mu);  // atomically: unlock mu + wait
                                  // on wakeup: reacquire mu
}
// ready == 1 now
pthread_mutex_unlock(&mu);
```

```go
// Go equivalent using channel as condition variable:
ready := make(chan struct{})

// Producer:
go func() {
    doWork()
    close(ready)  // signal all waiting goroutines
}()

// Consumer:
<-ready  // blocks until channel closed
```

## Memory Ordering — The Final Boss

Modern CPUs and compilers reorder memory operations for performance. This is invisible in single-threaded code but catastrophic in multithreaded code:

```
CPU 1:              CPU 2:
x = 1               while (y == 0) {}  // spin wait
y = 1               assert(x == 1)     // may FAIL!

CPU may reorder: y = 1 before x = 1 (from CPU 2's perspective).
Even though source code has x = 1 before y = 1.

Memory barriers / fences prevent reordering:
x = 1;
SFENCE;   // store fence: all stores before this appear before stores after
y = 1;
```

```c
// C11 atomics with memory ordering:
_Atomic int x = 0, y = 0;

// Thread 1:
atomic_store_explicit(&x, 1, memory_order_release);
// All stores before this line are visible before y is set

// Thread 2:
int y_val = atomic_load_explicit(&y, memory_order_acquire);
// All loads after this see all stores from the release
int x_val = atomic_load_explicit(&x, memory_order_relaxed);
// x_val is guaranteed to be 1 if y_val was 1
```

```rust
use std::sync::atomic::{AtomicI32, Ordering};

static X: AtomicI32 = AtomicI32::new(0);
static Y: AtomicI32 = AtomicI32::new(0);

// Thread 1:
X.store(1, Ordering::Relaxed);
Y.store(1, Ordering::Release);  // release: X=1 visible before Y=1

// Thread 2:
if Y.load(Ordering::Acquire) == 1 {  // acquire: sees X=1 once Y=1 observed
    assert_eq!(X.load(Ordering::Relaxed), 1);  // guaranteed!
}
```

---

# Quick Reference Cheat Sheet

## C: Where Is My Variable?

```
+----------------------------+------------------+------------------+
| Declaration                | Location         | Lifetime         |
+----------------------------+------------------+------------------+
| int x = 5;  (in function)  | Stack            | Function scope   |
| static int x = 5;          | Data segment     | Program lifetime |
| int x;  (file scope)       | BSS segment      | Program lifetime |
| int x = 5; (file scope)    | Data segment     | Program lifetime |
| malloc(n)                  | Heap             | Until free()     |
| "hello"                    | .rodata          | Program lifetime |
| char s[] = "hello";        | Stack            | Function scope   |
+----------------------------+------------------+------------------+
```

## Pointer Cheat Sheet

```
int x = 42;

int *p = &x;    // p is pointer to int, value = address of x
*p = 99;        // dereference: write 99 to x through p

int **pp = &p;  // pointer to pointer
**pp = 100;     // double dereference: write 100 to x

int arr[5];
int *ap = arr;  // arr decays to &arr[0]
ap[2] == *(ap+2) == *(2+ap) == 2[ap]  // all identical!

sizeof(arr)   // 20 (full array)
sizeof(ap)    // 8 (just pointer)
```

## Go vs C vs Rust Memory Safety

```
Risk              C          Go         Rust (safe)
────────────────────────────────────────────────────
Null dereference  UB         panic      impossible (Option)
Dangling pointer  UB         impossible impossible (borrow checker)
Buffer overflow   UB         panic      panic (bounds check)
Data race         UB         detectable impossible (Send/Sync)
Use-after-free    UB         impossible impossible (ownership)
Double free       UB         impossible impossible (single owner)
Memory leak       possible   GC handles possible (Rc cycles)
Uninitialized var UB         zero-init  compile error
```

## Complexity Cheat Sheet

```
Data Structure    Access    Search    Insert    Delete    Space
──────────────────────────────────────────────────────────────
Array             O(1)      O(n)      O(n)      O(n)      O(n)
Linked List       O(n)      O(n)      O(1)*     O(1)*     O(n)
Hash Map          O(1)**    O(1)**    O(1)**    O(1)**    O(n)
BST (balanced)    O(log n)  O(log n)  O(log n)  O(log n)  O(n)
Heap              O(1)***   O(n)      O(log n)  O(log n)  O(n)
Stack             O(n)      O(n)      O(1)      O(1)      O(n)
Queue             O(n)      O(n)      O(1)      O(1)      O(n)

* at front, with pointer to node
** amortized, assumes good hash function
*** only min/max
```

