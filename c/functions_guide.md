# CS Concepts for Functions in Rust, C, and Go
## A Complete, In-Depth Reference Guide

---

> This guide covers every foundational and advanced computer science concept you must understand when working with functions in Rust, C, and Go. Every concept is backed by real memory models, architecture diagrams, and code in all three languages. After this guide, you will have a deep mental model for reasoning about performance, safety, correctness, and design.

---

## Table of Contents

1. [Memory Architecture: Stack and Heap](#1-memory-architecture-stack-and-heap)
2. [Activation Records and Stack Frames](#2-activation-records-and-stack-frames)
3. [Calling Conventions and ABI](#3-calling-conventions-and-abi)
4. [Parameter Passing Semantics](#4-parameter-passing-semantics)
5. [Return Value Semantics](#5-return-value-semantics)
6. [Function Pointers and First-Class Functions](#6-function-pointers-and-first-class-functions)
7. [Closures and Environment Capture](#7-closures-and-environment-capture)
8. [Recursion, Call Depth, and Stack Overflow](#8-recursion-call-depth-and-stack-overflow)
9. [Tail Call Optimization and Trampolining](#9-tail-call-optimization-and-trampolining)
10. [Inlining and Compiler Optimizations](#10-inlining-and-compiler-optimizations)
11. [Ownership, Borrowing, and Lifetimes in Functions (Rust)](#11-ownership-borrowing-and-lifetimes-in-functions-rust)
12. [Generic Functions and Monomorphization](#12-generic-functions-and-monomorphization)
13. [Static vs Dynamic Dispatch](#13-static-vs-dynamic-dispatch)
14. [Error Handling in Functions](#14-error-handling-in-functions)
15. [Higher-Order Functions](#15-higher-order-functions)
16. [Variadic Functions](#16-variadic-functions)
17. [Concurrency and Functions](#17-concurrency-and-functions)
18. [ABI, Linkage, and FFI](#18-abi-linkage-and-ffi)
19. [Functional Programming Patterns](#19-functional-programming-patterns)
20. [Compiler Optimizations on Functions](#20-compiler-optimizations-on-functions)

---

## 1. Memory Architecture: Stack and Heap

### 1.1 The Mental Model

Every running program has its memory divided into several regions. Functions live in two of them: **code (text segment)** and **stack**. Data they allocate lives on the **heap**.

```
Process Virtual Address Space (64-bit Linux)
+-------------------------------------------------+   High Address
|  Kernel Space (not accessible to user program) |   0xFFFF_FFFF_FFFF_FFFF
+-------------------------------------------------+
|  Stack (grows DOWN)                             |
|  [ stack frame for main() ]                     |
|  [ stack frame for foo()  ]                     |
|  [ stack frame for bar()  ]  <-- rsp (top)     |
|  ...                                            |
|                                                 |
|  (gap / unmapped pages)                         |
|                                                 |
|  Heap (grows UP)                                |
|  [ malloc'd / new'd data ]                      |
|  ...                                            |
+-------------------------------------------------+
|  BSS segment  (uninitialized global data)       |
+-------------------------------------------------+
|  Data segment (initialized global/static data) |
+-------------------------------------------------+
|  Text segment (compiled machine code)           |   Low Address
+-------------------------------------------------+   0x0000_0000_0040_0000
```

### 1.2 Stack — The Function's Home

The stack is a **LIFO (Last In, First Out)** contiguous region of memory.

- **Managed by CPU registers**: `rsp` (stack pointer) always points to the top of the stack.
- On x86-64, the stack **grows downward** — pushing means subtracting from `rsp`.
- Each function call **pushes** a new frame onto the stack.
- When a function returns, its frame is **popped** (rsp is restored).
- Typical default size: 8 MB on Linux, 1 MB on Windows, 8 MB on macOS.

```
STACK GROWTH DIRECTION

High Address
┌──────────────────────────────────┐
│          main() frame            │  ← oldest frame
├──────────────────────────────────┤
│          process() frame         │
├──────────────────────────────────┤
│          calculate() frame       │  ← rsp points here
└──────────────────────────────────┘
Low Address

         ↑  stack grows DOWNWARD  ↓
         (new frames go to lower addresses)
```

**Why stack allocation is fast:**
- It's just arithmetic: `sub rsp, N` allocates N bytes instantly.
- No bookkeeping, no fragmentation — just a pointer.
- Data is already "in cache" because functions access it immediately.

### 1.3 Heap — Dynamically Allocated Data

The heap is used when:
- Size is not known at compile time.
- Data must outlive the function that created it.
- Data is very large (stack is limited).

```
HEAP LAYOUT

┌────────────────────────────────────────────┐
│  Allocator Metadata (e.g., jemalloc/glibc) │
├────────────────────────────────────────────┤
│  [ Allocated Block: 64 bytes  ]            │  ← returned from malloc/Box::new
│  [ Free Block: 128 bytes      ]            │
│  [ Allocated Block: 256 bytes ]            │
│  [ Free Block: 32 bytes       ]            │
│  ...                                        │
└────────────────────────────────────────────┘
```

**C — heap allocation:**
```c
#include <stdlib.h>
#include <string.h>

char* create_greeting(const char* name) {
    // heap-allocate a buffer; outlives this function
    size_t len = strlen(name) + 8;
    char* buf = malloc(len);   // raw heap alloc
    if (!buf) return NULL;
    snprintf(buf, len, "Hello, %s", name);
    return buf;                // caller must free()
}

int main(void) {
    char* msg = create_greeting("Alice");
    printf("%s\n", msg);
    free(msg);                 // manual deallocation
    return 0;
}
```

**Rust — heap allocation via `Box` and `String`:**
```rust
fn create_greeting(name: &str) -> String {
    // String is heap-allocated; ownership is returned to caller
    format!("Hello, {}", name)
    // When the String is dropped, memory is freed automatically
}

fn main() {
    let msg = create_greeting("Alice");
    println!("{}", msg);
    // msg is dropped here → heap freed automatically
}
```

**Go — heap allocation via GC:**
```go
package main

import "fmt"

func createGreeting(name string) string {
    // Go's GC manages heap memory — no explicit free
    return fmt.Sprintf("Hello, %s", name)
}

func main() {
    msg := createGreeting("Alice")
    fmt.Println(msg)
    // GC will collect msg when no longer referenced
}
```

### 1.4 Stack vs Heap: Decision Rules

```
ALLOCATION DECISION TREE

Function needs to store data
         │
         ▼
  Is the size known at compile time?
  ├── NO  ──────────────────────────────► HEAP (malloc, Box, new)
  └── YES
         │
         ▼
  Must data outlive the function?
  ├── YES ─────────────────────────────► HEAP
  └── NO
         │
         ▼
  Is the data very large (> ~100KB)?
  ├── YES ─────────────────────────────► HEAP (avoid stack overflow)
  └── NO
         │
         ▼
  STACK  (fast, automatic, cache-friendly)
```

---

## 2. Activation Records and Stack Frames

### 2.1 What Is a Stack Frame?

When a function is called, the CPU sets up a **stack frame** (also called an **activation record**). This contains everything the function needs to execute:

```
STACK FRAME ANATOMY (x86-64 System V ABI)

High Address (caller's frame above)
┌──────────────────────────────────────────┐
│  Arg 7, Arg 8... (extra args, if any)   │  ← passed on stack if > 6 args
├──────────────────────────────────────────┤
│  Return Address (saved %rip)             │  ← where to jump when done
├──────────────────────────────────────────┤  ← %rbp points HERE (base ptr)
│  Saved %rbp (caller's frame pointer)    │
├──────────────────────────────────────────┤
│  Local Variable 1                        │
│  Local Variable 2                        │
│  Local Variable 3                        │
│  ...                                     │
├──────────────────────────────────────────┤
│  Saved Callee-Saved Registers            │
│  (rbx, r12, r13, r14, r15 if used)      │
├──────────────────────────────────────────┤
│  Alignment Padding (to 16-byte boundary)│
└──────────────────────────────────────────┘  ← %rsp points HERE (stack ptr)
Low Address (next frame will go below)
```

**Every function call does this:**
```
CALL SEQUENCE (machine level)
                                         
 1. Caller evaluates arguments
 2. Arguments 1-6 go into registers (rdi, rsi, rdx, rcx, r8, r9)
 3. Extra arguments pushed onto stack
 4. CALL instruction:
       a. Pushes return address (next instruction's %rip) onto stack
       b. Jumps to function entry
 5. Callee prologue:
       a. PUSH %rbp           → saves caller's frame pointer
       b. MOV %rsp, %rbp      → new frame pointer
       c. SUB $N, %rsp        → allocates N bytes for locals
 6. Function executes
 7. Callee epilogue:
       a. MOV %rbp, %rsp      → restore stack pointer
       b. POP %rbp            → restore caller's frame pointer
       c. RET                 → pops return address, jumps to caller
```

### 2.2 Visualizing Nested Calls

```c
int add(int a, int b) { return a + b; }
int mul(int x, int y) { return x * y; }
int compute(int n)    { return add(n, mul(n, 2)); }
int main()            { return compute(5); }
```

```
STACK DURING compute(5) calling add(5, mul(5,2)):

Low Addr
┌─────────────────────────────┐
│         add() frame         │  ← rsp (top of stack)
│  local: [none]              │
│  saved rbp: → mul frame     │
│  return addr: → mul+offset  │
├─────────────────────────────┤
│         mul() frame         │
│  local: x=5, y=2            │
│  saved rbp: → compute frame │
│  return addr: → compute+off │
├─────────────────────────────┤
│       compute() frame       │
│  local: n=5                 │
│  saved rbp: → main frame    │
│  return addr: → main+offset │
├─────────────────────────────┤
│         main() frame        │
│  local: [none]              │
│  saved rbp: → OS frame      │
│  return addr: → libc start  │
└─────────────────────────────┘
High Addr
```

### 2.3 Inspecting Stack Frames in Practice

**C — using `__builtin_frame_address`:**
```c
#include <stdio.h>

void inner(void) {
    void* frame = __builtin_frame_address(0);
    printf("inner frame at:  %p\n", frame);
}

void outer(void) {
    void* frame = __builtin_frame_address(0);
    printf("outer frame at:  %p\n", frame);
    inner();
}

int main(void) {
    void* frame = __builtin_frame_address(0);
    printf("main  frame at:  %p\n", frame);
    outer();
    // Frames will be at decreasing addresses — stack grows down
    return 0;
}
```

**Rust — stack frame sizes via size_of for local variables:**
```rust
use std::mem;

fn inner() {
    let x: [u8; 64] = [0; 64];  // 64 bytes of local data
    let addr = x.as_ptr();
    println!("inner local at: {:p}", addr);
}

fn outer() {
    let y: [u8; 128] = [0; 128]; // 128 bytes of local data
    let addr = y.as_ptr();
    println!("outer local at: {:p}", addr);
    inner(); // inner's locals will be at even lower addresses
}

fn main() {
    outer();
}
```

**Go — goroutine stacks (growable):**
```go
package main

import (
    "fmt"
    "runtime"
)

func showStack(depth int) {
    buf := make([]byte, 1024)
    n := runtime.Stack(buf, false)
    if depth == 0 {
        fmt.Printf("Goroutine stack:\n%s\n", buf[:n])
    } else {
        showStack(depth - 1)
    }
}

func main() {
    showStack(3)
}
```

> **Key Go insight:** Go uses **segmented/growable stacks**. A goroutine starts with a tiny stack (2–8 KB) and the runtime automatically grows it when needed. This is fundamentally different from C and Rust where stack size is fixed per thread (typically 8 MB). This is why Go can have millions of goroutines.

### 2.4 Stack Frame Size and Performance

```
STACK FRAME SIZE IMPACT

Small frame (< cache line, 64 bytes):
  [local a][local b][return addr][rbp]
   └──────────── all in one cache line ──────────────┘
   → FAST: single cache hit for all locals

Large frame (many locals, arrays):
  [local a][local b][array[1024]][...][return addr][rbp]
   └──cache line─┘└──cache line─┘└──cache line─┘
   → SLOWER: multiple cache misses for large locals
```

---

## 3. Calling Conventions and ABI

### 3.1 What Is a Calling Convention?

A **calling convention** is a contract between caller and callee specifying:
- Which registers hold arguments
- Which registers hold return values
- Which registers must be preserved (callee-saved vs caller-saved)
- How the stack is aligned
- Who cleans up arguments from the stack

Without a calling convention, two functions compiled separately could not interoperate.

### 3.2 x86-64 System V ABI (Linux, macOS)

```
ARGUMENT PASSING REGISTERS (integers/pointers)

Argument #   Register   Alias
─────────────────────────────
   1st        %rdi      destination index
   2nd        %rsi      source index
   3rd        %rdx      data register
   4th        %rcx      counter
   5th        %r8       general
   6th        %r9       general
   7th+       Stack     pushed right-to-left

FLOATING POINT ARGUMENTS
   1st-8th    %xmm0-%xmm7

RETURN VALUES
  Integer     %rax (and %rdx for 128-bit)
  Float       %xmm0

CALLER-SAVED (caller must save if it needs them after the call)
  %rax, %rcx, %rdx, %rsi, %rdi, %r8, %r9, %r10, %r11

CALLEE-SAVED (callee must preserve these)
  %rbx, %rbp, %r12, %r13, %r14, %r15, %rsp
```

```
FUNCTION CALL: foo(a, b, c, d, e, f, g, h)
  (8 arguments)

  Caller code:
  ┌────────────────────────────────────────┐
  │  mov %rdi, a     ; arg 1              │
  │  mov %rsi, b     ; arg 2              │
  │  mov %rdx, c     ; arg 3              │
  │  mov %rcx, d     ; arg 4              │
  │  mov %r8,  e     ; arg 5              │
  │  mov %r9,  f     ; arg 6              │
  │  push h          ; arg 8 (last first) │
  │  push g          ; arg 7              │
  │  call foo                             │
  │  add $16, %rsp   ; clean up 2 args   │
  └────────────────────────────────────────┘
```

### 3.3 Windows x64 ABI (Different!)

```
WINDOWS x64 ARGUMENT REGISTERS
  %rcx, %rdx, %r8, %r9  (only 4 register args)
  Stack for the rest
  
SHADOW SPACE: 32-byte "home space" always reserved on stack
  even if the function has fewer than 4 parameters.
```

### 3.4 Demonstrating ABI in C

```c
// This compiles to exact register usage described above
// Compile with: gcc -O0 -S to see the assembly

long add_six(long a, long b, long c, long d, long e, long f) {
    // a → rdi, b → rsi, c → rdx, d → rcx, e → r8, f → r9
    return a + b + c + d + e + f;
}

long add_eight(long a, long b, long c, long d,
               long e, long f, long g, long h) {
    // g and h are on the stack (rbp+16 and rbp+24 in callee)
    return a + b + c + d + e + f + g + h;
}

int main(void) {
    long r1 = add_six(1, 2, 3, 4, 5, 6);
    long r2 = add_eight(1, 2, 3, 4, 5, 6, 7, 8);
    return (int)(r1 + r2);
}
```

**Rust — ABI and extern:**
```rust
// Rust uses the same System V ABI for plain functions
// but guarantees nothing about struct layout by default

// Explicit C ABI (matches System V exactly):
extern "C" fn add_six(a: i64, b: i64, c: i64,
                       d: i64, e: i64, f: i64) -> i64 {
    a + b + c + d + e + f
}

// Rust's default "Rust" ABI — undefined, may reorder params
fn add_six_rust(a: i64, b: i64, c: i64,
                d: i64, e: i64, f: i64) -> i64 {
    a + b + c + d + e + f
}

fn main() {
    println!("{}", add_six(1, 2, 3, 4, 5, 6));
}
```

**Go — ABI evolution:**
```go
package main

// Go originally used stack-based argument passing for all args.
// Since Go 1.17, the register-based calling convention is used
// (similar to System V ABI) for better performance.

// You cannot specify the ABI in user code directly;
// the Go compiler manages this automatically.

//go:noinline  // prevent inlining so we can observe stack frames
func addSix(a, b, c, d, e, f int) int {
    return a + b + c + d + e + f
}

func main() {
    result := addSix(1, 2, 3, 4, 5, 6)
    println(result)
}
```

### 3.5 Struct Arguments and the ABI

Small structs (≤ 2 words) are passed in registers. Large structs are passed on the stack (by copying).

```c
// C - struct passing

#include <stdio.h>

typedef struct { int x; int y; } Point;          // 8 bytes → registers
typedef struct { int data[16]; } BigData;         // 64 bytes → on stack

Point make_point(int x, int y) {
    return (Point){x, y};   // returned in rax:rdx (two registers)
}

// BigData is secretly passed as a pointer by the ABI:
// void process_big(BigData *__hidden_return, BigData arg)
BigData double_data(BigData d) {
    BigData result;
    for (int i = 0; i < 16; i++) result.data[i] = d.data[i] * 2;
    return result;
}

int main(void) {
    Point p = make_point(3, 4);
    BigData bd = {{1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16}};
    BigData result = double_data(bd);
    printf("(%d, %d)\n", p.x, p.y);
    return 0;
}
```

```rust
// Rust - struct passing and repr

#[derive(Debug)]
struct Point { x: i32, y: i32 }      // 8 bytes, passed in registers

#[repr(C)]                            // C-compatible layout
#[derive(Debug, Clone)]
struct BigData { data: [i32; 16] }   // 64 bytes, passed on stack

fn make_point(x: i32, y: i32) -> Point {
    Point { x, y }   // returned in registers
}

fn double_data(d: BigData) -> BigData {
    // d is moved (copied) into this function's stack frame
    let mut result = d;
    for v in &mut result.data { *v *= 2; }
    result
}

fn main() {
    let p = make_point(3, 4);
    let bd = BigData { data: [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16] };
    let result = double_data(bd);
    println!("{:?}", p);
    println!("{:?}", result.data[0]);
}
```

---

## 4. Parameter Passing Semantics

### 4.1 Pass by Value

A **copy** of the argument is made and given to the callee. Modifications inside the function do not affect the caller's data.

```
PASS BY VALUE

Caller memory:    [  x = 42  ]
                        │
                        │  copy
                        ▼
Callee frame:    [ param = 42 ]
                        │
                     modify
                        │
                        ▼
                 [ param = 99 ]   (caller's x is still 42)
```

**C:**
```c
#include <stdio.h>

void triple(int n) {
    n *= 3;          // modifies the copy, not the original
    printf("inside: %d\n", n);
}

int main(void) {
    int x = 10;
    triple(x);
    printf("outside: %d\n", x);  // still 10
    return 0;
}
```

**Rust:**
```rust
fn triple(mut n: i32) -> i32 {
    n *= 3;    // n is a copy of the argument
    n
}

fn main() {
    let x = 10i32;
    let result = triple(x);
    println!("x = {}, result = {}", x, result);  // x = 10, result = 30
}
```

**Go:**
```go
package main

import "fmt"

func triple(n int) int {
    n *= 3    // n is a copy
    return n
}

func main() {
    x := 10
    result := triple(x)
    fmt.Printf("x = %d, result = %d\n", x, result) // x = 10, result = 30
}
```

### 4.2 Pass by Pointer/Reference

The address of the variable is passed. The callee can read and modify the original data.

```
PASS BY POINTER/REFERENCE

Caller memory:    [  x = 42  ] at address 0x7fff0010
                        │
                        │  pass address 0x7fff0010
                        ▼
Callee frame:    [ ptr = 0x7fff0010 ]
                        │
               dereference and modify
                *ptr = 99
                        │
                        ▼
Caller memory:    [  x = 99  ]  ← MODIFIED
```

**C:**
```c
#include <stdio.h>

void triple_inplace(int *n) {
    *n *= 3;    // modifies through the pointer
}

void swap(int *a, int *b) {
    int temp = *a;
    *a = *b;
    *b = temp;
}

int main(void) {
    int x = 10;
    triple_inplace(&x);
    printf("x = %d\n", x);   // x = 30

    int p = 1, q = 2;
    swap(&p, &q);
    printf("p=%d, q=%d\n", p, q);  // p=2, q=1
    return 0;
}
```

**Rust — borrowing (safe references):**
```rust
fn triple_inplace(n: &mut i32) {
    *n *= 3;    // modify through mutable reference
}

fn swap(a: &mut i32, b: &mut i32) {
    // Rust ensures a and b do not alias — no UB
    std::mem::swap(a, b);
}

fn main() {
    let mut x = 10i32;
    triple_inplace(&mut x);
    println!("x = {}", x);   // x = 30

    let mut p = 1i32;
    let mut q = 2i32;
    swap(&mut p, &mut q);
    println!("p={}, q={}", p, q);  // p=2, q=1
}
```

**Go — pointers:**
```go
package main

import "fmt"

func tripleInplace(n *int) {
    *n *= 3    // modify through pointer
}

func swap(a, b *int) {
    *a, *b = *b, *a
}

func main() {
    x := 10
    tripleInplace(&x)
    fmt.Printf("x = %d\n", x)  // x = 30

    p, q := 1, 2
    swap(&p, &q)
    fmt.Printf("p=%d, q=%d\n", p, q)  // p=2, q=1
}
```

### 4.3 Pass by Move (Rust Ownership Transfer)

Rust's unique feature: passing a value **transfers ownership**. The caller loses access.

```
RUST MOVE SEMANTICS

Caller owns:   [  String "hello"  ]  at heap addr 0xABC
               [ ptr=0xABC, len=5, cap=8 ] on stack
                        │
                        │  move (stack metadata copied, ownership transferred)
                        ▼
Callee owns:   [ ptr=0xABC, len=5, cap=8 ] on callee's stack
Caller's var:  INVALID — compiler prevents use after move
```

```rust
fn consume(s: String) {
    println!("Consumed: {}", s);
    // s is dropped here; heap memory freed
}

fn borrow(s: &String) {
    println!("Borrowed: {}", s);
    // s is NOT dropped; just a reference
}

fn main() {
    let s1 = String::from("hello");

    borrow(&s1);        // lend s1 — s1 still valid
    println!("{}", s1); // OK

    consume(s1);        // MOVE s1 — s1 no longer valid
    // println!("{}", s1); // ← COMPILE ERROR: value used after move
}
```

### 4.4 Large Struct Passing: Value vs Reference Performance

```
PERFORMANCE COMPARISON: passing 1KB struct

Pass by value:
  Caller stack → copy 1024 bytes → Callee stack
  Cost: 1024 bytes memcpy every call

Pass by reference (pointer/borrow):
  Caller stack → pass 8-byte pointer → Callee stack
  Cost: 8 bytes; no copy

Pass by move (Rust):
  Compiler may optimize to reference in many cases.
  Stack-to-stack copy if not optimized.
```

**C — efficient large struct passing:**
```c
#include <string.h>
#include <stdio.h>

typedef struct { int pixels[1024]; } Image;

// SLOW: copies 4096 bytes on every call
int sum_pixels_slow(Image img) {
    int sum = 0;
    for (int i = 0; i < 1024; i++) sum += img.pixels[i];
    return sum;
}

// FAST: passes 8-byte pointer
int sum_pixels_fast(const Image *img) {
    int sum = 0;
    for (int i = 0; i < 1024; i++) sum += img->pixels[i];
    return sum;
}

int main(void) {
    Image img;
    memset(&img, 1, sizeof(img));
    printf("%d\n", sum_pixels_fast(&img));
    return 0;
}
```

**Rust — zero-cost with references:**
```rust
struct Image { pixels: [i32; 1024] }

// Borrow: zero copy, compile-time aliasing safety
fn sum_pixels(img: &Image) -> i32 {
    img.pixels.iter().sum()
}

// Move: compiler will likely pass as hidden pointer (optimization)
fn consume_image(img: Image) -> i32 {
    img.pixels.iter().sum()
}

fn main() {
    let img = Image { pixels: [1; 1024] };
    println!("{}", sum_pixels(&img));  // img still owned by main
    println!("{}", consume_image(img)); // img moved in
}
```

**Go — value copy is explicit:**
```go
package main

import "fmt"

type Image struct{ pixels [1024]int32 }

// Go passes structs by value — this copies 4096 bytes!
func sumPixelsSlow(img Image) int32 {
    var sum int32
    for _, p := range img.pixels { sum += p }
    return sum
}

// Efficient: pointer to struct (8 bytes)
func sumPixelsFast(img *Image) int32 {
    var sum int32
    for _, p := range img.pixels { sum += p }
    return sum
}

func main() {
    img := Image{}
    for i := range img.pixels { img.pixels[i] = 1 }
    fmt.Println(sumPixelsFast(&img))
}
```

---

## 5. Return Value Semantics

### 5.1 Register Returns

Small values (≤ 2 registers = 16 bytes) are returned in registers (`rax`, `rdx`). This is extremely fast — no memory involved.

```
RETURN VALUE IN REGISTERS

Function returns int (4 bytes):
  mov eax, <result>    ; put result in rax (lower 32 bits)
  ret

Function returns two ints (8 bytes):
  mov eax, <first>     ; lower 32 bits of rax
  mov edx, <second>    ; lower 32 bits of rdx
  ret

Function returns pointer (8 bytes):
  mov rax, <ptr>       ; full 64-bit pointer in rax
  ret
```

### 5.2 Large Return Values (Stack Return)

For structs larger than 2 registers, the caller pre-allocates space on its stack and passes a hidden pointer as the first argument.

```
LARGE STRUCT RETURN (RVO — Return Value Optimization)

Caller code (conceptually):
  BigStruct result;           // space allocated in caller's frame
  some_func(&result, args...); // hidden first arg = &result
  // result is now filled

Machine code reality:
  lea rdi, [rbp - sizeof(BigStruct)]   ; pass hidden return ptr
  call some_func
  ; struct now lives at [rbp - sizeof(BigStruct)]
```

### 5.3 Multiple Return Values

**C — only via output parameters or structs:**
```c
#include <stdio.h>
#include <stdbool.h>

// Method 1: out parameters via pointers
bool divide(int a, int b, int *quotient, int *remainder) {
    if (b == 0) return false;
    *quotient  = a / b;
    *remainder = a % b;
    return true;
}

// Method 2: return a struct
typedef struct { int quot; int rem; bool ok; } DivResult;
DivResult divide2(int a, int b) {
    if (b == 0) return (DivResult){0, 0, false};
    return (DivResult){a / b, a % b, true};
}

int main(void) {
    int q, r;
    if (divide(17, 5, &q, &r)) {
        printf("17/5 = %d rem %d\n", q, r);
    }

    DivResult dr = divide2(17, 5);
    if (dr.ok) {
        printf("17/5 = %d rem %d\n", dr.quot, dr.rem);
    }
    return 0;
}
```

**Rust — tuples and Result:**
```rust
fn divide(a: i32, b: i32) -> Option<(i32, i32)> {
    if b == 0 { return None; }
    Some((a / b, a % b))
}

// Idiomatic: Result for error conditions
fn divide_safe(a: i32, b: i32) -> Result<(i32, i32), &'static str> {
    if b == 0 { return Err("division by zero"); }
    Ok((a / b, a % b))
}

fn main() {
    if let Some((q, r)) = divide(17, 5) {
        println!("17/5 = {} rem {}", q, r);
    }

    match divide_safe(17, 0) {
        Ok((q, r)) => println!("{} rem {}", q, r),
        Err(e)     => println!("Error: {}", e),
    }
}
```

**Go — native multiple return values:**
```go
package main

import (
    "errors"
    "fmt"
)

// Go natively supports multiple returns
func divide(a, b int) (int, int, error) {
    if b == 0 {
        return 0, 0, errors.New("division by zero")
    }
    return a / b, a % b, nil
}

// Named return values (useful for documentation and defer)
func divideNamed(a, b int) (quotient int, remainder int, err error) {
    if b == 0 {
        err = errors.New("division by zero")
        return  // bare return — returns named values
    }
    quotient = a / b
    remainder = a % b
    return
}

func main() {
    q, r, err := divide(17, 5)
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("17/5 = %d rem %d\n", q, r)
}
```

### 5.4 Named Return Values in Go — Internals

```
GO NAMED RETURN VALUES LAYOUT

func foo() (result int, err error) {
    // result and err are pre-allocated in the stack frame
    // as if they were local variables declared at the top
}

Stack frame:
  [ result (int, 8 bytes) ]
  [ err    (interface, 16 bytes) ]
  [ ...other locals... ]
  [ return address ]
  [ saved rbp ]

bare `return` just reads the current values of result and err
from the stack and returns them.
```

---

## 6. Function Pointers and First-Class Functions

### 6.1 What Is a Function Pointer?

A **function pointer** stores the memory address of a function's compiled code in the text segment. Calling through a function pointer is an **indirect call** — the CPU must look up the address at runtime.

```
FUNCTION POINTER MECHANISM

Text Segment:
  0x401000: <code for add>
  0x401020: <code for mul>
  0x401040: <code for sub>

Function pointer variable:
  fp = 0x401020    (points to mul)

Calling fp(3, 4):
  1. Load address from fp → 0x401020
  2. Indirect jump to 0x401020
  3. Execute mul(3, 4) → 12

Direct call (compile-time known):
  call 0x401020    (hardcoded in machine code)

Indirect call (runtime):
  call *fp         (loads address, then calls)
  → prevents branch prediction, can miss instruction cache
```

### 6.2 Function Pointers in C

```c
#include <stdio.h>
#include <stdlib.h>

// Type alias for function pointer: takes two ints, returns int
typedef int (*BinaryOp)(int, int);

int add(int a, int b) { return a + b; }
int sub(int a, int b) { return a - b; }
int mul(int a, int b) { return a * b; }
int divide(int a, int b) { return b ? a / b : 0; }

// Higher-order function: takes a function pointer
int apply(BinaryOp op, int x, int y) {
    return op(x, y);
}

// Array of function pointers (jump table / dispatch table)
BinaryOp dispatch[] = { add, sub, mul, divide };
const char *names[]  = { "add", "sub", "mul", "div" };

// Callback pattern: sort with custom comparator
int compare_desc(const void *a, const void *b) {
    return (*(int*)b) - (*(int*)a);
}

int main(void) {
    // Direct usage
    BinaryOp op = add;
    printf("add(3,4) = %d\n", op(3, 4));

    // Passing as argument
    printf("apply(mul, 5, 6) = %d\n", apply(mul, 5, 6));

    // Jump table dispatch
    int x = 10, y = 3;
    for (int i = 0; i < 4; i++) {
        printf("%s(%d,%d) = %d\n", names[i], x, y, dispatch[i](x, y));
    }

    // qsort callback
    int arr[] = {5, 2, 8, 1, 9, 3};
    qsort(arr, 6, sizeof(int), compare_desc);
    for (int i = 0; i < 6; i++) printf("%d ", arr[i]);
    printf("\n");

    return 0;
}
```

### 6.3 Function Pointers in Rust

```rust
// Rust distinguishes between:
// fn(T) -> U       — function pointer (zero-size, direct)
// impl Fn(T) -> U  — closure trait (may carry captured state)

fn add(a: i32, b: i32) -> i32 { a + b }
fn mul(a: i32, b: i32) -> i32 { a * b }

fn apply(f: fn(i32, i32) -> i32, x: i32, y: i32) -> i32 {
    f(x, y)
}

fn make_adder(n: i32) -> fn(i32) -> i32 {
    // Only works if the function doesn't capture n
    // (fn pointers cannot capture environment)
    // This won't compile if n is captured:
    // fn inner(x: i32) -> i32 { x + n }  // ERROR: can't capture

    // Use a closure instead (next section)
    fn add_one(x: i32) -> i32 { x + 1 }  // n is baked-in
    add_one
}

fn main() {
    // Function pointer variable
    let f: fn(i32, i32) -> i32 = add;
    println!("{}", f(3, 4));

    // Array of function pointers
    let ops: [fn(i32, i32) -> i32; 2] = [add, mul];
    for op in &ops {
        println!("{}", op(5, 6));
    }

    // Passing as argument
    println!("{}", apply(mul, 5, 6));
}
```

### 6.4 Function Values in Go

```go
package main

import (
    "fmt"
    "sort"
)

// In Go, functions are first-class values.
// Function type: func(int, int) int

func add(a, b int) int { return a + b }
func mul(a, b int) int { return a * b }

func apply(f func(int, int) int, x, y int) int {
    return f(x, y)
}

// Go functions can return function values
func makeMultiplier(factor int) func(int) int {
    // This creates a closure that captures `factor`
    return func(x int) int {
        return x * factor
    }
}

func main() {
    // Function variable
    var f func(int, int) int = add
    fmt.Println(f(3, 4))  // 7

    // Pass as argument
    fmt.Println(apply(mul, 5, 6))  // 30

    // Function values in slices (dispatch table)
    ops := []func(int, int) int{add, mul}
    for _, op := range ops {
        fmt.Println(op(10, 3))
    }

    // Closure returning function
    double := makeMultiplier(2)
    triple := makeMultiplier(3)
    fmt.Println(double(5))  // 10
    fmt.Println(triple(5))  // 15

    // Sort with function value
    nums := []int{5, 2, 8, 1, 9}
    sort.Slice(nums, func(i, j int) bool {
        return nums[i] < nums[j]
    })
    fmt.Println(nums)
}
```

---

## 7. Closures and Environment Capture

### 7.1 What Is a Closure?

A **closure** is a function + captured environment. When a function references variables from its enclosing scope, the runtime (or compiler) must store those variables somewhere accessible to the function — this is the **closure record** or **upvalue**.

```
CLOSURE INTERNALS

Source code:
  counter_start = 10
  increment = λ(n) { counter_start + n }

Without closure (simple function pointer):
  [fn ptr] → code only, no state

With closure:
  [fn ptr | env ptr] → [fn ptr → code]
                          [env ptr → { counter_start: 10 }]

Memory layout of a closure object:
  ┌─────────────────────────────────────────┐
  │  fn_ptr: address of compiled inner fn   │  8 bytes
  │  env: pointer to captured variables     │  8 bytes
  └─────────────────────────────────────────┘

Captured variables (heap-allocated if they outlive scope):
  ┌─────────────────────────────────────────┐
  │  counter_start: 10  (i32, 4 bytes)     │
  └─────────────────────────────────────────┘
```

### 7.2 Capture Modes

```
CAPTURE MODES

By Reference (borrow):
  Captured variable lives in outer scope.
  Closure holds a reference/pointer.
  Outer scope must outlive closure.

  outer: [ x = 42 ]
              ↑ reference
  closure: [ fn_ptr | ref_to_x ]

By Value (copy):
  A copy of the variable is stored in the closure.
  Outer variable and closure's copy are independent.

  outer: [ x = 42 ]   (not captured, still alive)
  closure: [ fn_ptr | copy_x = 42 ]

By Move:
  Ownership transferred into the closure.
  Outer variable is invalidated.
  Used when closure must outlive the outer scope.

  outer: [ x = INVALID ]
  closure: [ fn_ptr | owned_x = 42 ]
```

### 7.3 Closures in C (Manual — No Native Closures)

C does not support closures natively. You simulate them with structs and function pointers.

```c
#include <stdio.h>
#include <stdlib.h>

// Manual closure: struct holds state + function pointer
typedef struct {
    int (*fn)(void *env, int x);   // function pointer
    void *env;                     // captured environment
} Closure;

// The actual logic
static int adder_fn(void *env, int x) {
    int *n = (int *)env;
    return *n + x;
}

// "Constructor" for the adder closure
Closure make_adder(int n) {
    int *captured = malloc(sizeof(int));
    *captured = n;
    return (Closure){ adder_fn, captured };
}

// Call a closure
int call(Closure c, int x) {
    return c.fn(c.env, x);
}

int main(void) {
    Closure add5  = make_adder(5);
    Closure add10 = make_adder(10);

    printf("%d\n", call(add5,  3));   // 8
    printf("%d\n", call(add10, 3));   // 13

    // Caller must free the environment manually
    free(add5.env);
    free(add10.env);
    return 0;
}
```

### 7.4 Closures in Rust: Fn, FnMut, FnOnce

Rust closures are typed by how they use captured variables. This is the **most nuanced** aspect of closures in any systems language.

```
RUST CLOSURE TRAIT HIERARCHY

FnOnce  ←  can be called only ONCE (consumes captured vars)
  ↑
FnMut   ←  can be called multiple times, may mutate captured vars
  ↑
Fn      ←  can be called multiple times, only borrows captured vars
```

```rust
fn main() {
    // ─── Fn: shared borrow, callable many times ───
    let name = String::from("Alice");
    let greet = || println!("Hello, {}!", name);   // borrows name
    greet();   // OK
    greet();   // OK again
    println!("{}", name);  // name still usable (only borrowed)

    // ─── FnMut: exclusive borrow, callable many times ───
    let mut count = 0;
    let mut increment = || { count += 1; count };   // mutably borrows count
    println!("{}", increment());  // 1
    println!("{}", increment());  // 2
    // println!("{}", count);   // ERROR: count mutably borrowed by increment

    // ─── FnOnce: moves captured value, callable only once ───
    let greeting = String::from("Goodbye!");
    let say_goodbye = move || {
        // `move` forces ownership transfer into the closure
        println!("{}", greeting);
        drop(greeting);  // greeting is consumed here
    };
    say_goodbye();   // OK — greeting moved into closure, then dropped
    // say_goodbye();   // ERROR: cannot call FnOnce more than once
    // println!("{}", greeting);  // ERROR: greeting was moved

    // ─── move closure for threads ───
    let data = vec![1, 2, 3];
    let handle = std::thread::spawn(move || {
        // data MUST be moved: thread may outlive the current scope
        println!("{:?}", data);
    });
    // println!("{:?}", data);  // ERROR: data moved into thread
    handle.join().unwrap();
}

// ─── Functions accepting closures via traits ───
fn call_twice<F: Fn()>(f: F) {
    f();
    f();
}

fn call_once<F: FnOnce()>(f: F) {
    f();
}

fn call_mutating<F: FnMut() -> i32>(mut f: F) -> i32 {
    f() + f()
}
```

### 7.5 Closure Memory Layout in Rust

```rust
// Rust closures are anonymous structs generated by the compiler

let x = 10i32;
let y = 20i32;

// This closure:
let add_xy = |z: i32| x + y + z;

// Is approximately compiled as:
struct AddXyClosure {
    x: i32,   // captured by copy
    y: i32,   // captured by copy
}
impl Fn(i32) -> i32 for AddXyClosure {
    fn call(&self, z: i32) -> i32 {
        self.x + self.y + z
    }
}

// sizeof(add_xy) = sizeof(i32) + sizeof(i32) = 8 bytes
// This is a zero-overhead abstraction — no heap allocation for the closure
println!("{}", std::mem::size_of_val(&add_xy)); // 8
```

### 7.6 Closures in Go

```go
package main

import "fmt"

// Go closures capture variables by reference (by default)
// The captured variable is heap-allocated if it escapes

func makeCounter() func() int {
    count := 0       // count escapes to heap (captured by closure)
    return func() int {
        count++
        return count
    }
}

func makeAdder(n int) func(int) int {
    return func(x int) int {
        return x + n  // n is captured by value (copied at creation)
    }
}

// Classic Go closure pitfall: loop variable capture
func badClosures() []func() int {
    funcs := make([]func() int, 3)
    for i := 0; i < 3; i++ {
        // BUG: all closures capture the same `i` variable
        funcs[i] = func() int { return i }
    }
    return funcs
    // All 3 will return 3 (the final value of i)
}

func goodClosures() []func() int {
    funcs := make([]func() int, 3)
    for i := 0; i < 3; i++ {
        i := i  // shadow: create a new variable each iteration
        funcs[i] = func() int { return i }
    }
    return funcs
    // Returns 0, 1, 2 as expected
}

func main() {
    counter := makeCounter()
    fmt.Println(counter())  // 1
    fmt.Println(counter())  // 2
    fmt.Println(counter())  // 3

    add5 := makeAdder(5)
    fmt.Println(add5(3))   // 8

    bad := badClosures()
    fmt.Println(bad[0](), bad[1](), bad[2]())  // 3, 3, 3

    good := goodClosures()
    fmt.Println(good[0](), good[1](), good[2]())  // 0, 1, 2
}
```

### 7.7 Go Closure Memory: Escape Analysis

```
GO ESCAPE ANALYSIS

func makeCounter() func() int {
    count := 0      // ← escapes to heap because returned closure captures it

    // The compiler detects that `count` outlives this function:
    // 1. makeCounter returns
    // 2. caller holds a func() int
    // 3. that func() still references count
    // → count must be heap-allocated

    return func() int {
        count++
        return count
    }
}

Stack frame of makeCounter() while executing:
  [ count (pointer to heap) ]  ← not the value itself!

Heap:
  [ int value: 0 → 1 → 2 → ... ]

The closure object:
  [ fn_ptr | count_ptr ]
```

---

## 8. Recursion, Call Depth, and Stack Overflow

### 8.1 How Recursion Works on the Stack

Each recursive call pushes a new stack frame. Deep recursion can exhaust the stack.

```
RECURSIVE FACTORIAL: fact(5)

Call Tree:           Stack at deepest point:
fact(5)              ┌────────────────────┐  ← rsp
  fact(4)            │  fact(1) frame     │
    fact(3)          │  fact(2) frame     │
      fact(2)        │  fact(3) frame     │
        fact(1)      │  fact(4) frame     │
        → returns 1  │  fact(5) frame     │
      → returns 2    │  main() frame      │
    → returns 6      └────────────────────┘
  → returns 24
→ returns 120

Each frame contains:
  - parameter n (8 bytes)
  - return address (8 bytes)
  - saved rbp (8 bytes)
  = 24+ bytes per frame

For fact(10000): 240,000+ bytes of stack used
For fact(100000): 2.4+ MB → stack overflow (default 8 MB)
```

### 8.2 Recursion in All Three Languages

**C:**
```c
#include <stdio.h>

// Direct recursion
long factorial(int n) {
    if (n <= 1) return 1;
    return n * factorial(n - 1);
}

// Mutual recursion
int is_even(int n);
int is_odd(int n);

int is_even(int n) { return n == 0 ? 1 : is_odd(n - 1); }
int is_odd(int n)  { return n == 0 ? 0 : is_even(n - 1); }

int main(void) {
    printf("%ld\n", factorial(10));   // 3628800
    printf("%d\n", is_even(100));     // 1
    return 0;
}
```

**Rust:**
```rust
fn factorial(n: u64) -> u64 {
    if n <= 1 { 1 } else { n * factorial(n - 1) }
}

// Rust: recursive closures require special handling
fn fibonacci(n: u64) -> u64 {
    match n {
        0 => 0,
        1 => 1,
        _ => fibonacci(n - 1) + fibonacci(n - 2),
    }
}

fn main() {
    println!("{}", factorial(10));    // 3628800
    println!("{}", fibonacci(30));   // 832040
}
```

**Go:**
```go
package main

import "fmt"

func factorial(n int) int {
    if n <= 1 { return 1 }
    return n * factorial(n-1)
}

// Go: goroutines start with 2KB-8KB stacks and grow
// So deep recursion is handled differently than in C/Rust!
func deepRecurse(n int) int {
    if n == 0 { return 0 }
    return 1 + deepRecurse(n-1)
}

func main() {
    fmt.Println(factorial(10))      // 3628800
    // This would crash in C with n=1000000, but Go grows the stack:
    fmt.Println(deepRecurse(100000))  // 100000 — works!
}
```

### 8.3 Stack Overflow Detection

**C:**
```c
#include <stdio.h>
#include <signal.h>
#include <stdlib.h>

void handler(int sig) {
    // Cannot reliably recover from SIGSEGV caused by stack overflow
    fprintf(stderr, "Stack overflow detected!\n");
    exit(1);
}

int infinite_recurse(int n) {
    return infinite_recurse(n + 1);  // no base case
}

int main(void) {
    signal(SIGSEGV, handler);  // segfault = usually stack overflow
    return infinite_recurse(0);  // will segfault
}
```

**Rust:**
```rust
// Rust does NOT protect against stack overflows by default
// (it would be too expensive to check on every call)
// You will get a SIGSEGV / SIGBUS just like C

fn infinite() -> i32 { 1 + infinite() }

fn main() {
    // stacker crate provides explicit stack size management:
    stacker::maybe_grow(32 * 1024, 1024 * 1024, || {
        // Run potentially deep recursive code with guaranteed stack space
        // println!("{}", infinite()); // still dangerous
    });
}
```

**Go:**
```go
package main

import (
    "fmt"
    "runtime/debug"
)

// Go runtime detects stack overflows and panics (does not SIGSEGV)
func infinite() int { return 1 + infinite() }

func safeCall() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("Caught panic:", r)
            debug.PrintStack()
        }
    }()
    infinite()
}

func main() {
    safeCall()  // prints "runtime: goroutine stack exceeds ..."
}
```

---

## 9. Tail Call Optimization and Trampolining

### 9.1 What Is Tail Call Optimization (TCO)?

A **tail call** is a function call that is the **last action** a function performs — its result is directly returned without any further computation.

```
TAIL CALL vs NON-TAIL CALL

Non-tail call (recursive factorial):
  factorial(5)
    = 5 * factorial(4)       ← must remember "5 *"
         = 4 * factorial(3)  ← must remember "4 *"
              ...
  All frames must be kept alive on stack to compute the product.

Tail call (tail-recursive factorial):
  factorial(5, 1)
  = factorial(4, 5)    ← nothing to do after this call
  = factorial(3, 20)   ← nothing to do after this call
  = factorial(2, 60)
  = factorial(1, 120)
  = 120

  With TCO: Each frame can be REUSED (the compiler replaces the
  call with a JUMP). Stack depth stays constant = O(1) space.

WITHOUT TCO:
┌─────────────┐
│ fact(5,1)   │─→ saves state, calls fact(4,5)
├─────────────┤
│ fact(4,5)   │─→ saves state, calls fact(3,20)
├─────────────┤
│ fact(3,20)  │
├─────────────┤
│ fact(2,60)  │
├─────────────┤
│ fact(1,120) │─→ returns 120
└─────────────┘

WITH TCO (tail call → jump):
┌─────────────┐
│ fact(n, acc)│  ← only ONE frame, reused each iteration
└─────────────┘
```

### 9.2 TCO in C (Compiler-Dependent)

```c
#include <stdio.h>

// Tail-recursive factorial (the call to fact_tail is in tail position)
long fact_tail(int n, long acc) {
    if (n <= 1) return acc;
    return fact_tail(n - 1, n * acc);   // tail call — compiler may optimize
}

long factorial(int n) {
    return fact_tail(n, 1);
}

// GCC/Clang WILL optimize this to a loop with -O2 or higher
// Verify: gcc -O2 -S factorial.c → check for `jmp` instead of `call`

// Equivalent iterative version (what TCO produces):
long factorial_iter(int n) {
    long acc = 1;
    while (n > 1) {
        acc *= n;
        n--;
    }
    return acc;
}

int main(void) {
    printf("%ld\n", factorial(20));  // 2432902008176640000
    return 0;
}
```

### 9.3 TCO in Rust (No Guarantee, but Likely with Optimization)

```rust
// Rust does NOT guarantee TCO, but LLVM often applies it at -O2

fn factorial_tail(n: u64, acc: u64) -> u64 {
    if n <= 1 { return acc; }
    factorial_tail(n - 1, n * acc)  // likely TCO'd by LLVM
}

pub fn factorial(n: u64) -> u64 {
    factorial_tail(n, 1)
}

// When you NEED guaranteed stack safety, use loops or trampolining:
fn factorial_loop(n: u64) -> u64 {
    let mut acc = 1u64;
    let mut i = n;
    while i > 1 {
        acc *= i;
        i -= 1;
    }
    acc
}

fn main() {
    println!("{}", factorial(20));
    println!("{}", factorial_loop(20));
}
```

### 9.4 TCO in Go (NOT Supported)

```go
// Go does NOT perform TCO. Tail calls still push new frames.
// Deep tail recursion in Go will grow the stack, but Go handles
// this with its segmented/growable stack — so it won't crash,
// but it wastes memory.

package main

import "fmt"

func factTail(n, acc int) int {
    if n <= 1 { return acc }
    return factTail(n-1, n*acc)  // NOT optimized — new frame each time
}

// Preferred in Go: use loops
func factLoop(n int) int {
    acc := 1
    for n > 1 {
        acc *= n
        n--
    }
    return acc
}

func main() {
    fmt.Println(factTail(10, 1))  // 3628800 (but inefficient)
    fmt.Println(factLoop(10))     // 3628800 (efficient)
}
```

### 9.5 Trampolining — Manual TCO for Any Language

**Trampolining** replaces recursive calls with a loop that processes **thunks** (deferred computations), achieving constant stack depth without compiler TCO support.

```
TRAMPOLINE MECHANISM

Normal recursion:
  f(1) → f(2) → f(3) → f(4) → 4  (4 stack frames)

Trampoline:
  f(1) → Return Thunk("call f(2)")
  Loop: call thunk → Return Thunk("call f(3)")
  Loop: call thunk → Return Thunk("call f(4)")
  Loop: call thunk → Return Value(4)
  Result: 4  (only 1 stack frame at any time)
```

**Rust trampoline:**
```rust
enum Trampoline<T> {
    Done(T),
    More(Box<dyn FnOnce() -> Trampoline<T>>),
}

fn run<T>(mut t: Trampoline<T>) -> T {
    loop {
        match t {
            Trampoline::Done(v) => return v,
            Trampoline::More(f) => t = f(),
        }
    }
}

fn factorial_trampoline(n: u64, acc: u64) -> Trampoline<u64> {
    if n <= 1 {
        Trampoline::Done(acc)
    } else {
        Trampoline::More(Box::new(move || factorial_trampoline(n - 1, n * acc)))
    }
}

fn main() {
    let result = run(factorial_trampoline(20, 1));
    println!("{}", result);  // 2432902008176640000
    // Stack depth is O(1) regardless of n
}
```

**Go trampoline:**
```go
package main

import "fmt"

type Result struct {
    done  bool
    value int
    next  func() Result
}

func trampoline(f func() Result) int {
    result := f()
    for !result.done {
        result = result.next()
    }
    return result.value
}

func factTramp(n, acc int) func() Result {
    return func() Result {
        if n <= 1 {
            return Result{done: true, value: acc}
        }
        return Result{next: factTramp(n-1, n*acc)}
    }
}

func main() {
    result := trampoline(factTramp(10, 1))
    fmt.Println(result)  // 3628800
}
```

---

## 10. Inlining and Compiler Optimizations

### 10.1 What Is Function Inlining?

**Inlining** replaces a function call with the function's body at the call site. This eliminates:
- Function call overhead (push/pop registers, jump)
- Stack frame setup/teardown
- Parameter passing overhead

And enables:
- Further optimizations across call boundaries (constant propagation, dead code elimination)

```
BEFORE INLINING:

int square(int x) { return x * x; }
int main() {
    int r = square(5);
    return r;
}

Assembly:
  main:
    mov $5, %rdi          ; pass arg
    call square           ; push return addr, jump
    ; square:
    ;   imul %edi, %edi
    ;   ret               ; pop return addr, jump back
    mov %eax, %ebx        ; save result
    ...

AFTER INLINING:

  main:
    mov $25, %eax         ; compiler computed 5*5=25 at compile time!
    ; NO function call at all — dead code eliminated too
```

### 10.2 Inlining in C

```c
#include <stdio.h>

// Hint to compiler (not a guarantee)
static inline int square(int x) {
    return x * x;
}

// Force inline (GCC/Clang extension)
static __attribute__((always_inline)) inline int cube(int x) {
    return x * x * x;
}

// Prevent inlining
static __attribute__((noinline)) int heavy_computation(int x) {
    // Long function body discourages automatic inlining
    volatile int r = x;
    for (int i = 0; i < 1000; i++) r ^= i;
    return r;
}

int main(void) {
    int a = square(5);          // likely inlined → just `imul 5, 5`
    int b = cube(3);            // always inlined
    int c = heavy_computation(7); // never inlined
    printf("%d %d %d\n", a, b, c);
    return 0;
}
```

### 10.3 Inlining in Rust

```rust
// Rust's #[inline] attributes:
// #[inline]         — hint to LLVM that this should be inlined
// #[inline(always)] — force inline (LLVM will usually comply)
// #[inline(never)]  — never inline

#[inline]
fn square(x: i32) -> i32 {
    x * x
}

#[inline(always)]
fn cube(x: i32) -> i32 {
    x * x * x
}

#[inline(never)]
fn heavy(x: i32) -> i32 {
    (0..1000).fold(x, |acc, i| acc ^ i)
}

// Generic functions are ALWAYS inlined into their instantiations
fn generic_square<T: std::ops::Mul<Output = T> + Copy>(x: T) -> T {
    x * x   // monomorphized + inlined at each call site
}

fn main() {
    println!("{}", square(5));        // 25, likely constant-folded to 25
    println!("{}", cube(3));          // 27
    println!("{}", heavy(7));         // computed at runtime
    println!("{}", generic_square(4i32));   // 16
    println!("{}", generic_square(4.0f64)); // 16.0
}
```

### 10.4 Inlining in Go

```go
package main

import "fmt"

// Go's compiler inlines small functions automatically
// Use //go:noinline to prevent inlining (for testing/benchmarking)

//go:noinline
func square(x int) int {
    return x * x
}

// Go will inline this (small, no loops)
func cube(x int) int {
    return x * x * x
}

// Too complex to inline (loop)
func sumN(n int) int {
    sum := 0
    for i := 0; i <= n; i++ {
        sum += i
    }
    return sum
}

func main() {
    fmt.Println(square(5))  // won't be inlined (noinline)
    fmt.Println(cube(3))    // will be inlined
    fmt.Println(sumN(100))  // not inlined (too complex)
}
```

```
INLINING DECISION TREE (Compilers)

Function is small (< ~30-80 bytecodes/instructions)?
├── YES → Strong candidate for inlining
└── NO  → Rarely inlined unless always_inline

Function is called in a hot loop?
├── YES → Profiler-guided inlining will inline it
└── NO  → Less priority

Function is generic/template instantiation?
├── YES (Rust/C++) → Almost always inlined per instantiation
└── NO  → Normal rules apply

Function has loops inside?
├── YES → Compiler may decline inlining (code size concern)
└── NO  → More likely to inline

Is always_inline set?
├── YES → Force inline regardless
└── NO  → Compiler decides

Result: INLINE → copy body to call site, remove call overhead
         NO INLINE → emit as separate function, use call instruction
```

---

## 11. Ownership, Borrowing, and Lifetimes in Functions (Rust)

### 11.1 Ownership Rules

Rust enforces three ownership rules at compile time:
1. Each value has exactly one owner.
2. When the owner goes out of scope, the value is dropped.
3. Ownership can be transferred (moved) or temporarily lent (borrowed).

```
OWNERSHIP AND FUNCTION CALLS

fn consume(s: String) { ... }    // takes ownership
fn borrow(s: &String) { ... }    // borrows (shared ref)
fn borrow_mut(s: &mut String) {} // borrows mutably

main() owns: [ String "hello" ]
                    │
             ┌──────┴───────┐
             │ borrow(&s)   │  → lends ref, main retains ownership
             │ still own s  │
             └──────────────┘
                    │
             ┌──────┴───────┐
             │ consume(s)   │  → ownership transferred to consume
             │ s is gone    │  → consume's scope end → drop
             └──────────────┘
```

### 11.2 The Borrow Checker: Core Rules

```
BORROW CHECKER RULES

At any point in time, for a value T, you may have EITHER:
  Option A: Any number of shared references (&T)
  Option B: Exactly ONE mutable reference (&mut T)
  But NEVER both simultaneously.

Valid:
  let a = &x;    // shared
  let b = &x;    // another shared — OK
  println!("{} {}", a, b);  // both used — OK

Invalid:
  let a = &mut x;   // mutable
  let b = &x;       // shared while mutable exists — ERROR
  println!("{} {}", a, b);
```

```rust
fn main() {
    let mut v = vec![1, 2, 3];

    // ── SHARED BORROWS ──
    let r1 = &v;
    let r2 = &v;
    println!("{:?} {:?}", r1, r2);  // OK: two shared refs

    // ── MUTABLE BORROW ──
    // r1 and r2 are no longer used after here (NLL: non-lexical lifetimes)
    let r3 = &mut v;
    r3.push(4);  // OK: exclusive mutable borrow
    println!("{:?}", r3);

    // ── FUNCTION-SCOPED BORROWS ──
    fn sum(v: &Vec<i32>) -> i32 {
        v.iter().sum()    // borrows v
    }
    fn add_elem(v: &mut Vec<i32>, x: i32) {
        v.push(x);        // mutably borrows v
    }

    println!("{}", sum(&v));   // shared borrow
    add_elem(&mut v, 5);       // mutable borrow
    println!("{}", sum(&v));   // shared borrow again
}
```

### 11.3 Lifetimes in Functions

Lifetimes describe **how long references are valid**. The compiler needs lifetime annotations when it can't infer which input reference a returned reference points to.

```
LIFETIME PROBLEM

fn longest(s1: &str, s2: &str) -> &str {
    if s1.len() > s2.len() { s1 } else { s2 }
}

// Compiler asks: does the return value reference s1 or s2?
// It might be either! The return reference's lifetime depends on
// whichever input is shorter.

// Fix: lifetime annotation says "return lives as long as the shorter of s1, s2"
fn longest<'a>(s1: &'a str, s2: &'a str) -> &'a str {
    if s1.len() > s2.len() { s1 } else { s2 }
}

LIFETIME DIAGRAM:

fn main() {
    let s1 = String::from("long string");    // 'long starts here
    let result;
    {
        let s2 = String::from("xy");          // 'short starts here
        result = longest(s1.as_str(), s2.as_str());
        //       'a = min('long, 'short) = 'short
        println!("{}", result);               // OK: 'short still alive
    }                                          // 'short ends here
    // println!("{}", result);  // ERROR: result may point to s2 which is gone
}
```

```rust
// Lifetime annotations in practice

// 'a means: output lives at least as long as input
fn first_word(s: &str) -> &str {
    let bytes = s.as_bytes();
    for (i, &b) in bytes.iter().enumerate() {
        if b == b' ' { return &s[..i]; }
    }
    &s[..]
}

// Struct with reference field: must annotate
struct Important<'a> {
    content: &'a str,   // must not outlive the source string
}

impl<'a> Important<'a> {
    fn announce(&self) -> &str {
        self.content    // return lifetime elided = tied to &self
    }
}

// 'static lifetime: lives for the entire program
fn get_greeting() -> &'static str {
    "Hello, World!"   // string literals are 'static
}

fn main() {
    let s = String::from("hello world foo");
    let word = first_word(&s);
    println!("{}", word);  // "hello"
    // drop(s);  // ERROR: s is borrowed by word

    let novel = String::from("Call me Ishmael...");
    let i = Important { content: novel.split('.').next().unwrap() };
    println!("{}", i.announce());
}
```

### 11.4 Lifetime Elision Rules

Rust applies three lifetime elision rules automatically so you don't need to annotate most functions:

```
LIFETIME ELISION RULES

Rule 1: Each reference parameter gets its own lifetime.
  fn foo(x: &T, y: &U)
  → fn foo<'a, 'b>(x: &'a T, y: &'b U)

Rule 2: If there is exactly one input lifetime, it applies to all output lifetimes.
  fn foo(x: &T) -> &U
  → fn foo<'a>(x: &'a T) -> &'a U

Rule 3: If one of the inputs is &self or &mut self, its lifetime applies to all outputs.
  fn foo(&self, x: &T) -> &U
  → fn foo<'a, 'b>(&'a self, x: &'b T) -> &'a U
```

---

## 12. Generic Functions and Monomorphization

### 12.1 Generic Functions: The Concept

Generic functions operate on multiple types without code duplication. The compiler **specializes** them per type.

```
GENERIC FUNCTION INSTANTIATION

Source:
  fn max<T: PartialOrd>(a: T, b: T) -> T { if a > b { a } else { b } }

Usage:
  max(3i32, 5i32)     → compiler generates: max_i32(a: i32, b: i32) -> i32
  max(3.0f64, 5.0f64) → compiler generates: max_f64(a: f64, b: f64) -> f64
  max("a", "z")       → compiler generates: max_str(a: &str, b: &str) -> &str

This is MONOMORPHIZATION: one generic → many concrete implementations
```

### 12.2 C — Generics via Macros and _Generic

```c
#include <stdio.h>

// C11 _Generic: compile-time type dispatch
#define max(a, b) _Generic((a), \
    int:    max_int,            \
    double: max_double,         \
    float:  max_float           \
)(a, b)

int    max_int(int a, int b)       { return a > b ? a : b; }
double max_double(double a, double b) { return a > b ? a : b; }
float  max_float(float a, float b) { return a > b ? a : b; }

// Type-safe generic container via macros (common pattern in C)
#define DEFINE_STACK(T, NAME)                        \
    typedef struct { T *data; int top; int cap; } NAME; \
    void NAME##_push(NAME *s, T val) {               \
        s->data[s->top++] = val;                     \
    }                                                \
    T NAME##_pop(NAME *s) {                          \
        return s->data[--s->top];                    \
    }

DEFINE_STACK(int, IntStack)
DEFINE_STACK(double, DoubleStack)

int main(void) {
    printf("%d\n", max(3, 5));         // calls max_int
    printf("%.1f\n", max(3.0, 5.0));  // calls max_double

    IntStack s = { .data = malloc(10 * sizeof(int)), .top = 0, .cap = 10 };
    IntStack_push(&s, 42);
    printf("%d\n", IntStack_pop(&s));
    free(s.data);
    return 0;
}
```

### 12.3 Rust Generics with Trait Bounds

```rust
use std::fmt::Display;

// Trait bounds: T must implement PartialOrd
fn max<T: PartialOrd>(a: T, b: T) -> T {
    if a > b { a } else { b }
}

// Multiple bounds with `+`
fn print_max<T: PartialOrd + Display>(a: T, b: T) {
    println!("max = {}", max(a, b));
}

// Where clause (cleaner for complex bounds)
fn complex_fn<T, U>(a: T, b: U) -> String
where
    T: Display + Clone,
    U: Display + PartialOrd,
{
    format!("{} {}", a, b)
}

// Generic structs with methods
struct Pair<T> { first: T, second: T }

impl<T: Display + PartialOrd> Pair<T> {
    fn larger(&self) -> &T {
        if self.first > self.second { &self.first } else { &self.second }
    }
}

fn main() {
    println!("{}", max(3i32, 5));     // max::<i32>
    println!("{}", max(3.0f64, 5.0)); // max::<f64>
    println!("{}", max("a", "z"));    // max::<&str>

    print_max(1, 2);
    print_max("hello", "world");

    let pair = Pair { first: 5, second: 10 };
    println!("{}", pair.larger());
}
```

### 12.4 Go Generics (1.18+)

```go
package main

import (
    "fmt"
    "golang.org/x/exp/constraints"
)

// Generic function with type constraint
func Max[T constraints.Ordered](a, b T) T {
    if a > b { return a }
    return b
}

// Generic with custom constraint interface
type Number interface {
    int | int32 | int64 | float32 | float64
}

func Sum[T Number](nums []T) T {
    var total T
    for _, n := range nums { total += n }
    return total
}

// Generic struct
type Stack[T any] struct {
    items []T
}

func (s *Stack[T]) Push(item T) {
    s.items = append(s.items, item)
}

func (s *Stack[T]) Pop() (T, bool) {
    if len(s.items) == 0 {
        var zero T
        return zero, false
    }
    top := s.items[len(s.items)-1]
    s.items = s.items[:len(s.items)-1]
    return top, true
}

func main() {
    fmt.Println(Max(3, 5))          // int
    fmt.Println(Max(3.0, 5.0))      // float64
    fmt.Println(Max("a", "z"))      // string

    ints := []int{1, 2, 3, 4, 5}
    floats := []float64{1.1, 2.2, 3.3}
    fmt.Println(Sum(ints))          // 15
    fmt.Println(Sum(floats))        // 6.6

    s := Stack[int]{}
    s.Push(1)
    s.Push(2)
    val, ok := s.Pop()
    fmt.Println(val, ok)            // 2 true
}
```

### 12.5 Monomorphization vs Dynamic Dispatch: Trade-offs

```
MONOMORPHIZATION (Static Dispatch — Rust, C templates)

  Source: fn process<T: Trait>(t: T)

  Generated:
    fn process_TypeA(t: TypeA) { ... }
    fn process_TypeB(t: TypeB) { ... }
    fn process_TypeC(t: TypeC) { ... }

  Pro: Maximum performance (direct call, inlineable, no vtable)
  Con: Code bloat (binary size grows with each type instantiation)

DYNAMIC DISPATCH (Runtime — Go interfaces, Rust dyn Trait)

  Source: fn process(t: &dyn Trait)

  Generated:
    fn process(t: *VTable, data: *void) {
        // call through vtable
    }

  Pro: Smaller binary, works with heterogeneous collections
  Con: Indirect call overhead, prevents inlining, cache miss risk
```

---

## 13. Static vs Dynamic Dispatch

### 13.1 Static Dispatch — Compile-Time Resolution

The compiler knows the exact function to call at compile time.

```
STATIC DISPATCH

Source:            Compiled:
  fn foo() { ... }    → 0x401000: code for foo
  fn bar() { ... }    → 0x401020: code for bar

  fn main() {
    foo();   → call 0x401000   (hardcoded address)
    bar();   → call 0x401020   (hardcoded address)
  }

Branch predictor knows exactly where to go → fast
Inlining possible → even faster
```

### 13.2 Dynamic Dispatch — Runtime Resolution

The actual function is determined at runtime through a **vtable** (virtual method table).

```
VTABLE STRUCTURE (for interface/trait dispatch)

Trait: Draw { fn draw(&self); fn bounds(&self) -> Rect; }

VTable for Circle:
  ┌──────────────────────────────────┐
  │ size:    = sizeof(Circle)        │
  │ align:   = alignof(Circle)       │
  │ drop_fn: = Circle::drop          │
  │ draw:    = Circle::draw          │  ← function pointer
  │ bounds:  = Circle::bounds        │  ← function pointer
  └──────────────────────────────────┘

VTable for Rectangle:
  ┌──────────────────────────────────┐
  │ size:    = sizeof(Rectangle)     │
  │ draw:    = Rectangle::draw       │
  │ bounds:  = Rectangle::bounds     │
  └──────────────────────────────────┘

Fat pointer (trait object): [data_ptr | vtable_ptr]
                              ↓            ↓
                          Circle data   Circle VTable

Calling draw():
  1. Load vtable_ptr from fat pointer
  2. Load draw function pointer from vtable
  3. Call through function pointer  ← indirect, can miss cache
```

### 13.3 Dynamic Dispatch in C (Manual vtable)

```c
#include <stdio.h>

// Manual vtable (C's way of doing OOP/dynamic dispatch)
typedef struct Shape Shape;

typedef struct {
    void (*draw)(const Shape *self);
    double (*area)(const Shape *self);
    void (*destroy)(Shape *self);
} ShapeVTable;

struct Shape {
    const ShapeVTable *vtable;   // pointer to vtable
    // derived structs extend this
};

// --- Circle ---
typedef struct {
    Shape base;    // MUST be first field
    double radius;
} Circle;

static void circle_draw(const Shape *self) {
    const Circle *c = (const Circle *)self;
    printf("Drawing circle r=%.1f\n", c->radius);
}
static double circle_area(const Shape *self) {
    const Circle *c = (const Circle *)self;
    return 3.14159 * c->radius * c->radius;
}
static void circle_destroy(Shape *s) { free(s); }

static const ShapeVTable circle_vtable = { circle_draw, circle_area, circle_destroy };

Circle* circle_new(double r) {
    Circle *c = malloc(sizeof(Circle));
    c->base.vtable = &circle_vtable;
    c->radius = r;
    return c;
}

// --- Rectangle ---
typedef struct {
    Shape base;
    double w, h;
} Rect;

static void rect_draw(const Shape *s) {
    const Rect *r = (const Rect *)s;
    printf("Drawing rect %.1fx%.1f\n", r->w, r->h);
}
static double rect_area(const Shape *s) {
    const Rect *r = (const Rect *)s;
    return r->w * r->h;
}
static void rect_destroy(Shape *s) { free(s); }

static const ShapeVTable rect_vtable = { rect_draw, rect_area, rect_destroy };

Rect* rect_new(double w, double h) {
    Rect *r = malloc(sizeof(Rect));
    r->base.vtable = &rect_vtable;
    r->w = w; r->h = h;
    return r;
}

// Polymorphic function: works on any Shape
void print_shape(const Shape *s) {
    s->vtable->draw(s);
    printf("  area = %.2f\n", s->vtable->area(s));
}

int main(void) {
    Shape *shapes[] = {
        (Shape*)circle_new(5.0),
        (Shape*)rect_new(3.0, 4.0),
        (Shape*)circle_new(2.5),
    };
    for (int i = 0; i < 3; i++) {
        print_shape(shapes[i]);
        shapes[i]->vtable->destroy(shapes[i]);
    }
    return 0;
}
```

### 13.4 Dynamic Dispatch in Rust — `dyn Trait`

```rust
trait Draw {
    fn draw(&self);
    fn area(&self) -> f64;
}

struct Circle { radius: f64 }
struct Rect   { w: f64, h: f64 }

impl Draw for Circle {
    fn draw(&self) { println!("Drawing circle r={:.1}", self.radius); }
    fn area(&self) -> f64 { std::f64::consts::PI * self.radius * self.radius }
}

impl Draw for Rect {
    fn draw(&self) { println!("Drawing rect {:.1}x{:.1}", self.w, self.h); }
    fn area(&self) -> f64 { self.w * self.h }
}

// Static dispatch: monomorphized, fastest
fn print_shape_static<S: Draw>(s: &S) {
    s.draw();
    println!("  area = {:.2}", s.area());
}

// Dynamic dispatch: trait object, works with heterogeneous collections
fn print_shape_dynamic(s: &dyn Draw) {
    s.draw();
    println!("  area = {:.2}", s.area());
}

fn main() {
    // Static dispatch — concrete types
    let c = Circle { radius: 5.0 };
    let r = Rect { w: 3.0, h: 4.0 };
    print_shape_static(&c);  // monomorphized to Circle version
    print_shape_static(&r);  // monomorphized to Rect version

    // Dynamic dispatch — heterogeneous Vec
    let shapes: Vec<Box<dyn Draw>> = vec![
        Box::new(Circle { radius: 5.0 }),
        Box::new(Rect { w: 3.0, h: 4.0 }),
        Box::new(Circle { radius: 2.5 }),
    ];
    for shape in &shapes {
        print_shape_dynamic(shape.as_ref());  // vtable lookup
    }
}
```

### 13.5 Dynamic Dispatch in Go — Interfaces

```go
package main

import (
    "fmt"
    "math"
)

// Go interfaces are structurally typed (implicit implementation)
type Shape interface {
    Draw()
    Area() float64
}

type Circle struct{ Radius float64 }
type Rect   struct{ W, H float64 }

func (c Circle) Draw() { fmt.Printf("Drawing circle r=%.1f\n", c.Radius) }
func (c Circle) Area() float64 { return math.Pi * c.Radius * c.Radius }

func (r Rect) Draw() { fmt.Printf("Drawing rect %.1fx%.1f\n", r.W, r.H) }
func (r Rect) Area() float64 { return r.W * r.H }

// Go interface value = [type_descriptor | data_ptr]
// The type_descriptor is Go's equivalent of a vtable pointer

func printShape(s Shape) {
    s.Draw()                               // dynamic dispatch
    fmt.Printf("  area = %.2f\n", s.Area())
}

func main() {
    shapes := []Shape{
        Circle{Radius: 5.0},
        Rect{W: 3.0, H: 4.0},
        Circle{Radius: 2.5},
    }
    for _, s := range shapes {
        printShape(s)  // each call goes through interface dispatch
    }
}
```

```
GO INTERFACE INTERNAL LAYOUT

type Shape interface { Draw(); Area() float64 }
var s Shape = Circle{Radius: 5.0}

Interface value in memory:
  ┌─────────────────────────────────────────────────┐
  │ type_ptr  → Circle type descriptor (itab)       │
  │ data_ptr  → Circle{Radius: 5.0} (on heap or    │
  │              stack, depending on size)           │
  └─────────────────────────────────────────────────┘

itab (interface table for Circle implementing Shape):
  ┌─────────────────────────────────────────────────┐
  │ inter: *interfacetype  (Shape's type info)      │
  │ type:  *_type          (Circle's type info)     │
  │ hash:  uint32                                   │
  │ fun[0]: Circle.Draw                             │
  │ fun[1]: Circle.Area                             │
  └─────────────────────────────────────────────────┘
```

---

## 14. Error Handling in Functions

### 14.1 C — Error Codes and errno

```c
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
#include <string.h>

// Convention: return negative value / 0 / NULL for error
// Global errno is set by many library functions

int open_and_read(const char *path, char *buf, size_t size) {
    FILE *f = fopen(path, "r");
    if (!f) {
        // errno is set by fopen (e.g., ENOENT, EACCES)
        fprintf(stderr, "fopen: %s\n", strerror(errno));
        return -1;
    }

    size_t n = fread(buf, 1, size - 1, f);
    buf[n] = '\0';

    if (ferror(f)) {
        fclose(f);
        return -2;
    }

    fclose(f);
    return (int)n;
}

// Chained error propagation (manual, verbose)
int process_file(const char *path) {
    char buf[4096];
    int n = open_and_read(path, buf, sizeof(buf));
    if (n < 0) return n;  // propagate error

    printf("Read %d bytes: %.20s...\n", n, buf);
    return 0;
}

int main(void) {
    if (process_file("/etc/hostname") < 0) {
        fprintf(stderr, "Failed\n");
        return 1;
    }
    return 0;
}
```

### 14.2 Rust — Result<T, E> and the ? Operator

```rust
use std::fs;
use std::io;
use std::num::ParseIntError;
use std::fmt;

// Custom error type
#[derive(Debug)]
enum AppError {
    Io(io::Error),
    Parse(ParseIntError),
    Custom(String),
}

impl fmt::Display for AppError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            AppError::Io(e)      => write!(f, "IO error: {}", e),
            AppError::Parse(e)   => write!(f, "Parse error: {}", e),
            AppError::Custom(s)  => write!(f, "Error: {}", s),
        }
    }
}

// From conversions allow ? to auto-convert error types
impl From<io::Error> for AppError {
    fn from(e: io::Error) -> Self { AppError::Io(e) }
}
impl From<ParseIntError> for AppError {
    fn from(e: ParseIntError) -> Self { AppError::Parse(e) }
}

fn read_number_from_file(path: &str) -> Result<i32, AppError> {
    let content = fs::read_to_string(path)?;   // ? converts io::Error → AppError
    let n: i32 = content.trim().parse()?;      // ? converts ParseIntError → AppError
    if n < 0 {
        return Err(AppError::Custom(format!("negative number: {}", n)));
    }
    Ok(n)
}

// ? operator is syntactic sugar for:
// match expr { Ok(v) => v, Err(e) => return Err(e.into()) }

fn double_from_file(path: &str) -> Result<i32, AppError> {
    let n = read_number_from_file(path)?;
    Ok(n * 2)
}

fn main() {
    match double_from_file("number.txt") {
        Ok(n)  => println!("Result: {}", n),
        Err(e) => eprintln!("Error: {}", e),
    }

    // Using .unwrap_or, .map, .and_then for chaining
    let result = read_number_from_file("number.txt")
        .map(|n| n * 2)
        .unwrap_or(0);
    println!("{}", result);
}
```

### 14.3 Go — (T, error) Multiple Return

```go
package main

import (
    "errors"
    "fmt"
    "os"
    "strconv"
    "strings"
)

// Custom error type
type AppError struct {
    Code    int
    Message string
    Wrapped error
}

func (e *AppError) Error() string {
    return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Wrapped }

// Sentinel errors
var ErrNotFound = errors.New("not found")
var ErrInvalid  = errors.New("invalid input")

func readNumberFromFile(path string) (int, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return 0, &AppError{Code: 100, Message: "file read failed", Wrapped: err}
    }

    n, err := strconv.Atoi(strings.TrimSpace(string(data)))
    if err != nil {
        return 0, &AppError{Code: 200, Message: "parse failed", Wrapped: err}
    }

    if n < 0 {
        return 0, ErrInvalid
    }
    return n, nil
}

func doubleFromFile(path string) (int, error) {
    n, err := readNumberFromFile(path)
    if err != nil {
        return 0, fmt.Errorf("doubleFromFile: %w", err)  // %w wraps error
    }
    return n * 2, nil
}

func main() {
    result, err := doubleFromFile("number.txt")
    if err != nil {
        // errors.Is: checks error chain for sentinel
        if errors.Is(err, ErrInvalid) {
            fmt.Println("invalid input")
        }
        // errors.As: unwraps to specific type
        var appErr *AppError
        if errors.As(err, &appErr) {
            fmt.Printf("AppError code: %d\n", appErr.Code)
        }
        fmt.Println("Error:", err)
        return
    }
    fmt.Println("Result:", result)
}
```

```
ERROR HANDLING COMPARISON

          │  C              │  Rust           │  Go
──────────┼─────────────────┼─────────────────┼──────────────────
Mechanism │  return codes   │  Result<T,E>    │  (T, error)
          │  + errno        │                 │
Error type│  int/NULL/errno │  Generic E      │  error interface
Propagate │  manual if/     │  ? operator     │  if err != nil {
          │  return         │  (auto)         │    return 0, err }
Ignore    │  easy (just     │  must_use warn  │  easy (just _)
error     │  ignore)        │  on Result      │
Composing │  messy/manual   │  .map/.and_then │  errors.Is/As/%w
errors    │                 │                 │
```

---

## 15. Higher-Order Functions

### 15.1 Map, Filter, Reduce

**C — manual implementation (no native HOF):**
```c
#include <stdio.h>
#include <stdlib.h>

typedef int (*MapFn)(int);
typedef int (*FilterFn)(int);
typedef int (*ReduceFn)(int, int);

// map: applies fn to each element, returns new array
int* map(const int *arr, int n, MapFn fn, int *out_len) {
    *out_len = n;
    int *result = malloc(n * sizeof(int));
    for (int i = 0; i < n; i++) result[i] = fn(arr[i]);
    return result;
}

// filter: keeps elements for which fn returns nonzero
int* filter(const int *arr, int n, FilterFn fn, int *out_len) {
    int *result = malloc(n * sizeof(int));
    int j = 0;
    for (int i = 0; i < n; i++) if (fn(arr[i])) result[j++] = arr[i];
    *out_len = j;
    return result;
}

// reduce: fold array to single value
int reduce(const int *arr, int n, ReduceFn fn, int init) {
    int acc = init;
    for (int i = 0; i < n; i++) acc = fn(acc, arr[i]);
    return acc;
}

// Concrete functions
int double_it(int x)    { return x * 2; }
int is_even(int x)      { return x % 2 == 0; }
int add(int acc, int x) { return acc + x; }

int main(void) {
    int nums[] = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10};
    int n = 10;

    int out_n;
    int *doubled  = map(nums, n, double_it, &out_n);
    int *evens    = filter(nums, n, is_even, &out_n);
    int total     = reduce(nums, n, add, 0);

    printf("sum = %d\n", total);  // 55

    free(doubled);
    free(evens);
    return 0;
}
```

**Rust — idiomatic iterators:**
```rust
fn main() {
    let nums = vec![1i32, 2, 3, 4, 5, 6, 7, 8, 9, 10];

    // map: transform each element
    let doubled: Vec<i32> = nums.iter().map(|&x| x * 2).collect();
    println!("{:?}", doubled);  // [2, 4, 6, 8, 10, ...]

    // filter: keep matching elements
    let evens: Vec<&i32> = nums.iter().filter(|&&x| x % 2 == 0).collect();
    println!("{:?}", evens);    // [2, 4, 6, 8, 10]

    // fold (reduce): accumulate
    let sum: i32 = nums.iter().fold(0, |acc, &x| acc + x);
    println!("{}", sum);        // 55

    // Chained: filter even, double, sum
    let result: i32 = nums.iter()
        .filter(|&&x| x % 2 == 0)
        .map(|&x| x * 2)
        .sum();
    println!("{}", result);     // 60 (2+4+6+8+10)*2

    // flatten: nested iterators
    let nested = vec![vec![1,2], vec![3,4], vec![5,6]];
    let flat: Vec<i32> = nested.into_iter().flatten().collect();
    println!("{:?}", flat);     // [1, 2, 3, 4, 5, 6]

    // zip: pair two iterators
    let a = vec![1, 2, 3];
    let b = vec!["one", "two", "three"];
    let zipped: Vec<(i32, &&str)> = a.iter().zip(b.iter()).map(|(&n, s)| (n, s)).collect();

    // Rust iterators are LAZY: no computation until collect()/sum()/etc.
    // The entire chain is compiled to a single loop with NO allocations
    // until the final .collect()
}
```

**Go — manual (pre-1.18) and generic (1.18+):**
```go
package main

import "fmt"

// Pre-generics: type-specific or use any/interface{}
func mapInts(nums []int, f func(int) int) []int {
    result := make([]int, len(nums))
    for i, n := range nums { result[i] = f(n) }
    return result
}

func filterInts(nums []int, f func(int) bool) []int {
    var result []int
    for _, n := range nums { if f(n) { result = append(result, n) } }
    return result
}

func reduceInts(nums []int, init int, f func(int, int) int) int {
    acc := init
    for _, n := range nums { acc = f(acc, n) }
    return acc
}

// Generic versions (Go 1.18+)
func Map[T, U any](slice []T, f func(T) U) []U {
    result := make([]U, len(slice))
    for i, v := range slice { result[i] = f(v) }
    return result
}

func Filter[T any](slice []T, f func(T) bool) []T {
    var result []T
    for _, v := range slice { if f(v) { result = append(result, v) } }
    return result
}

func Reduce[T, U any](slice []T, init U, f func(U, T) U) U {
    acc := init
    for _, v := range slice { acc = f(acc, v) }
    return acc
}

func main() {
    nums := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

    doubled := mapInts(nums, func(n int) int { return n * 2 })
    evens   := filterInts(nums, func(n int) bool { return n%2 == 0 })
    sum     := reduceInts(nums, 0, func(acc, n int) int { return acc + n })

    fmt.Println(doubled)
    fmt.Println(evens)
    fmt.Println(sum)  // 55

    // Generic versions
    strs := Map(nums, func(n int) string { return fmt.Sprintf("%d", n) })
    fmt.Println(strs)
}
```

### 15.2 Function Composition

```rust
// Rust: compose two functions
fn compose<A, B, C>(f: impl Fn(A) -> B, g: impl Fn(B) -> C) -> impl Fn(A) -> C {
    move |x| g(f(x))
}

fn main() {
    let double = |x: i32| x * 2;
    let add_one = |x: i32| x + 1;
    let increment_and_double = compose(add_one, double);  // double(add_one(x))
    println!("{}", increment_and_double(5));  // (5+1)*2 = 12
}
```

---

## 16. Variadic Functions

### 16.1 C — `va_list` and `<stdarg.h>`

```c
#include <stdio.h>
#include <stdarg.h>
#include <string.h>

// Fixed arg `count` tells us how many variadic args follow
int sum_ints(int count, ...) {
    va_list args;
    va_start(args, count);    // initialize va_list after last fixed arg

    int total = 0;
    for (int i = 0; i < count; i++) {
        total += va_arg(args, int);  // retrieve next argument as int
    }

    va_end(args);             // clean up
    return total;
}

// Variadic with format string (like printf)
void log_message(const char *fmt, ...) {
    va_list args;
    va_start(args, fmt);
    vprintf(fmt, args);       // vprintf accepts va_list
    va_end(args);
    printf("\n");
}

int main(void) {
    printf("%d\n", sum_ints(3, 10, 20, 30));   // 60
    printf("%d\n", sum_ints(5, 1, 2, 3, 4, 5)); // 15
    log_message("Hello %s, you are %d years old", "Alice", 30);
    return 0;
}
```

### 16.2 Go — Native Variadic Functions

```go
package main

import "fmt"

// ... means zero or more arguments of that type
func sum(nums ...int) int {
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}

// Variadic with different types via interface{}
func logAll(format string, args ...interface{}) {
    fmt.Printf("[LOG] "+format+"\n", args...)
}

// Spreading a slice into variadic function
func main() {
    fmt.Println(sum(1, 2, 3))           // 6
    fmt.Println(sum(1, 2, 3, 4, 5))     // 15
    fmt.Println(sum())                   // 0

    // Spread a slice with `...`
    nums := []int{1, 2, 3, 4, 5}
    fmt.Println(sum(nums...))            // 15

    logAll("User %s logged in at port %d", "alice", 8080)
}
```

### 16.3 Rust — Macros for Variadic-Like Behavior

```rust
// Rust functions cannot be variadic (no va_list equivalent).
// Variadic behavior is achieved via macros (println!, vec!, etc.)

// Custom macro mimicking variadic:
macro_rules! sum {
    // Base case: single value
    ($x:expr) => { $x };
    // Recursive case: add head + tail
    ($x:expr, $($rest:expr),+) => {
        $x + sum!($($rest),+)
    };
}

macro_rules! max_of {
    ($x:expr) => { $x };
    ($x:expr, $($rest:expr),+) => {
        {
            let rest = max_of!($($rest),+);
            if $x > rest { $x } else { rest }
        }
    };
}

// For actual runtime variadic, use slices:
fn sum_slice(nums: &[i32]) -> i32 {
    nums.iter().sum()
}

fn main() {
    println!("{}", sum!(1, 2, 3, 4, 5));    // 15 — macro expansion
    println!("{}", max_of!(3, 1, 4, 1, 5)); // 5
    println!("{}", sum_slice(&[1, 2, 3]));   // 6
}
```

---

## 17. Concurrency and Functions

### 17.1 Thread Safety and Function Calls

```
THREAD SAFETY REQUIREMENTS FOR FUNCTIONS

Pure function (no side effects, no global state):
  → Always thread-safe, no synchronization needed

Function with immutable shared data:
  → Thread-safe with read locks or atomic reads

Function with mutable shared state:
  → Requires synchronization:
    - Mutex / RwLock
    - Atomic operations
    - Message passing (channels)

Function on the stack only:
  → Always thread-safe (stack is per-thread)
```

**C — threads and function safety:**
```c
#include <stdio.h>
#include <stdlib.h>
#include <pthread.h>

// Thread-safe function (uses only local variables)
int pure_add(int a, int b) { return a + b; }

// NOT thread-safe (shared mutable static)
static int counter = 0;
void unsafe_increment(void) {
    counter++;  // data race!
}

// Thread-safe with mutex
static pthread_mutex_t mtx = PTHREAD_MUTEX_INITIALIZER;
void safe_increment(void) {
    pthread_mutex_lock(&mtx);
    counter++;
    pthread_mutex_unlock(&mtx);
}

void* thread_fn(void *arg) {
    for (int i = 0; i < 10000; i++) safe_increment();
    return NULL;
}

int main(void) {
    pthread_t t1, t2;
    pthread_create(&t1, NULL, thread_fn, NULL);
    pthread_create(&t2, NULL, thread_fn, NULL);
    pthread_join(t1, NULL);
    pthread_join(t2, NULL);
    printf("counter = %d\n", counter);  // should be 20000
    return 0;
}
```

**Rust — compile-time thread safety:**
```rust
use std::sync::{Arc, Mutex};
use std::thread;

// Pure function: Send + Sync implicitly, zero overhead
fn pure_add(a: i32, b: i32) -> i32 { a + b }

// Shared mutable state: Rust enforces Mutex at compile time
fn main() {
    let counter = Arc::new(Mutex::new(0i32));

    let handles: Vec<_> = (0..4).map(|_| {
        let c = Arc::clone(&counter);
        thread::spawn(move || {
            for _ in 0..10_000 {
                let mut guard = c.lock().unwrap();
                *guard += 1;
            }
        })
    }).collect();

    for h in handles { h.join().unwrap(); }
    println!("counter = {}", *counter.lock().unwrap());  // 40000

    // Rust's Send/Sync traits enforce:
    // Send: type can be transferred across thread boundary
    // Sync: type can be shared (& reference) across threads
    // Raw pointers are !Send and !Sync — caught at compile time
}
```

### 17.2 Go Goroutines and Functions

```go
package main

import (
    "fmt"
    "sync"
    "sync/atomic"
)

// Any function can be run as a goroutine with `go`
func heavyWork(id int, wg *sync.WaitGroup) {
    defer wg.Done()
    fmt.Printf("goroutine %d done\n", id)
}

// Channels for communication between goroutines
func producer(ch chan<- int, n int) {
    for i := 0; i < n; i++ {
        ch <- i * i
    }
    close(ch)
}

func consumer(ch <-chan int, results *[]int) {
    for v := range ch {
        *results = append(*results, v)
    }
}

// Atomic counter (lock-free)
var atomicCounter int64

func atomicIncrement(wg *sync.WaitGroup) {
    defer wg.Done()
    atomic.AddInt64(&atomicCounter, 1)
}

func main() {
    var wg sync.WaitGroup

    // Launch 10 goroutines
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go heavyWork(i, &wg)
    }
    wg.Wait()

    // Channel pipeline
    ch := make(chan int, 10)  // buffered channel
    go producer(ch, 5)
    var results []int
    consumer(ch, &results)
    fmt.Println(results)  // [0, 1, 4, 9, 16]

    // Atomic counter
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go atomicIncrement(&wg)
    }
    wg.Wait()
    fmt.Println("atomic counter:", atomicCounter)  // 1000
}
```

### 17.3 Rust Async Functions

```rust
use tokio;

// async fn returns a Future<Output = T>
async fn fetch_data(url: &str) -> Result<String, String> {
    // Simulate async I/O
    tokio::time::sleep(std::time::Duration::from_millis(10)).await;
    Ok(format!("data from {}", url))
}

async fn process(url: &str) -> String {
    match fetch_data(url).await {
        Ok(data)  => format!("processed: {}", data),
        Err(e)    => format!("error: {}", e),
    }
}

// Run multiple async tasks concurrently
async fn run_concurrent() {
    let (a, b, c) = tokio::join!(
        fetch_data("url1"),
        fetch_data("url2"),
        fetch_data("url3"),
    );
    println!("{:?} {:?} {:?}", a, b, c);
}

#[tokio::main]
async fn main() {
    let result = process("https://example.com").await;
    println!("{}", result);
    run_concurrent().await;
}
```

```
ASYNC FUNCTION STATE MACHINE

async fn example() {
    let x = step1().await;  // suspension point 1
    let y = step2(x).await; // suspension point 2
    return y;
}

Compiled state machine:
  enum ExampleState {
    State0,                  // initial
    State1 { x: i32 },      // after first await
    Done(i32),              // final
  }

  impl Future for Example {
    fn poll(&mut self) -> Poll<i32> {
      match self.state {
        State0 => {
          match step1.poll() {
            Ready(x) → self.state = State1 { x }; // transition
            Pending  → return Pending;             // yield control
          }
        }
        State1 { x } => {
          match step2(x).poll() {
            Ready(y) → return Ready(y);
            Pending  → return Pending;
          }
        }
      }
    }
  }
```

---

## 18. ABI, Linkage, and FFI

### 18.1 Symbol Visibility and Linkage

```
SYMBOL VISIBILITY

Internal linkage (static in C, private in modules):
  Symbol only visible within its translation unit.
  Can have same name in different .c files without conflict.

External linkage (default in C, pub in Rust):
  Symbol visible to the linker, accessible from other files.

Linker sees:
  object1.o: [_foo (internal)] [bar (external)]
  object2.o: [_foo (internal)] [bar (external)] → LINKER ERROR: duplicate bar

Name mangling (C++ and Rust mangle names for overloading/generics):
  Rust: _ZN5hello4mainE → "main in module hello"
  C:    no mangling (what you write is what the linker sees)
```

**C — static and extern:**
```c
// file1.c
static int internal_counter = 0;  // internal linkage
int public_counter = 0;            // external linkage

static void internal_helper(void) { internal_counter++; }

void increment(void) {
    internal_helper();
    public_counter++;
}

// file2.c
extern int public_counter;  // declare (not define) external symbol

void show_counter(void) {
    printf("%d\n", public_counter);
}
```

### 18.2 C FFI in Rust

```rust
// Calling C functions from Rust

// Link to the C standard library's math functions
#[link(name = "m")]
extern "C" {
    fn sqrt(x: f64) -> f64;
    fn pow(base: f64, exp: f64) -> f64;
    fn abs(x: i32) -> i32;
}

fn main() {
    // All C function calls through FFI are unsafe
    // (Rust cannot verify C's safety guarantees)
    let result = unsafe { sqrt(2.0) };
    println!("{}", result);  // 1.4142135623730951

    let p = unsafe { pow(2.0, 10.0) };
    println!("{}", p);  // 1024.0
}
```

```rust
// Exposing Rust functions to C

// repr(C) ensures C-compatible memory layout
#[repr(C)]
pub struct Point { pub x: f64, pub y: f64 }

// no_mangle: use exact name "distance" (not mangled)
// extern "C": use C calling convention
#[no_mangle]
pub extern "C" fn distance(a: Point, b: Point) -> f64 {
    let dx = a.x - b.x;
    let dy = a.y - b.y;
    (dx * dx + dy * dy).sqrt()
}

// C header (generated or hand-written):
// typedef struct { double x; double y; } Point;
// double distance(Point a, Point b);
```

### 18.3 CGo — Calling C from Go

```go
package main

/*
#include <string.h>
#include <math.h>

double c_hypotenuse(double a, double b) {
    return sqrt(a*a + b*b);
}
*/
import "C"   // CGo magic import

import "fmt"

func main() {
    result := C.c_hypotenuse(3.0, 4.0)  // calls C code
    fmt.Println(float64(result))          // 5.0

    // C string interop
    s := C.CString("hello")    // allocates C string
    defer C.free(unsafe.Pointer(s))
    n := C.strlen(s)
    fmt.Println(int(n))        // 5
}
```

---

## 19. Functional Programming Patterns

### 19.1 Pure Functions

A **pure function** has no side effects and always returns the same output for the same input.

```
PURE vs IMPURE

Pure:
  int add(int a, int b) { return a + b; }
  // Same inputs → same output, always
  // No global state, no I/O, no mutations

Impure:
  int add_and_log(int a, int b) {
      printf("adding %d + %d\n", a, b);  // side effect: I/O
      return a + b;
  }

  int running_total = 0;
  int add_to_total(int x) {
      running_total += x;   // side effect: mutation of global
      return running_total; // output depends on history, not just x
  }
```

### 19.2 Currying and Partial Application

**Currying** transforms a function of N args into N functions of 1 arg each.
**Partial application** fixes some arguments to produce a function of fewer args.

```
CURRYING

add(a, b) = a + b            // 2-arg function

curried_add(a) = fn(b) { a + b }  // 1-arg function returning 1-arg function

add5 = curried_add(5)        // partially applied
add5(3) = 8
add5(10) = 15
```

**Rust:**
```rust
// Partial application via closures
fn make_adder(n: i32) -> impl Fn(i32) -> i32 {
    move |x| x + n    // captures n by move
}

fn make_multiplier(n: i32) -> impl Fn(i32) -> i32 {
    move |x| x * n
}

// Curried function
fn curried_add(a: i32) -> impl Fn(i32) -> i32 {
    move |b| a + b
}

fn main() {
    let add5 = make_adder(5);
    let double = make_multiplier(2);

    println!("{}", add5(3));        // 8
    println!("{}", double(7));      // 14

    // Composing partial applications
    let add5_then_double: Vec<i32> = (1..=5).map(|x| double(add5(x))).collect();
    println!("{:?}", add5_then_double);  // [12, 14, 16, 18, 20]

    // Curried
    let add = curried_add(10);
    println!("{}", add(5));  // 15
}
```

**Go:**
```go
package main

import "fmt"

func makeAdder(n int) func(int) int {
    return func(x int) int { return x + n }
}

func compose(f, g func(int) int) func(int) int {
    return func(x int) int { return g(f(x)) }
}

func main() {
    add5 := makeAdder(5)
    add10 := makeAdder(10)
    add15 := compose(add5, add10)  // add5 then add10

    fmt.Println(add5(3))    // 8
    fmt.Println(add15(0))   // 15
}
```

### 19.3 Memoization

```rust
use std::collections::HashMap;

// Memoized fibonacci
struct Memo {
    cache: HashMap<u64, u64>,
}

impl Memo {
    fn new() -> Self { Memo { cache: HashMap::new() } }

    fn fib(&mut self, n: u64) -> u64 {
        if n <= 1 { return n; }
        if let Some(&v) = self.cache.get(&n) { return v; }
        let result = self.fib(n - 1) + self.fib(n - 2);
        self.cache.insert(n, result);
        result
    }
}

fn main() {
    let mut m = Memo::new();
    println!("{}", m.fib(50));  // computed efficiently via memoization
}
```

**Go:**
```go
package main

import (
    "fmt"
    "sync"
)

type MemoFunc struct {
    fn    func(int) int
    cache map[int]int
    mu    sync.Mutex
}

func NewMemo(fn func(int) int) *MemoFunc {
    return &MemoFunc{fn: fn, cache: make(map[int]int)}
}

func (m *MemoFunc) Call(arg int) int {
    m.mu.Lock()
    if v, ok := m.cache[arg]; ok {
        m.mu.Unlock()
        return v
    }
    m.mu.Unlock()

    result := m.fn(arg)

    m.mu.Lock()
    m.cache[arg] = result
    m.mu.Unlock()
    return result
}

func main() {
    slow := func(n int) int {
        // imagine expensive computation
        sum := 0
        for i := 0; i <= n; i++ { sum += i }
        return sum
    }
    memoized := NewMemo(slow)
    fmt.Println(memoized.Call(100))  // computed
    fmt.Println(memoized.Call(100))  // from cache
}
```

---

## 20. Compiler Optimizations on Functions

### 20.1 Dead Code Elimination

```
DEAD CODE ELIMINATION

Source:
  fn foo() -> i32 {
      let x = compute_expensive();   // expensive!
      let y = 5;
      return y;  // x is never used
  }

After optimization:
  fn foo() -> i32 {
      return 5;  // compiler eliminated compute_expensive()
  }

For this to work:
  - compute_expensive must have no observable side effects
  - Compiler must prove x is never used
```

### 20.2 Constant Folding and Propagation

```c
// C: before constant folding
int result = 2 * 3 + 4 / 2;

// After constant folding:
int result = 8;   // computed at compile time

// More complex:
const int SIZE = 10;
int arr[SIZE];   // SIZE is folded to 10

// Constant propagation across function calls:
static inline int double_it(int x) { return x * 2; }
int y = double_it(21);  // folded to 42 after inlining
```

### 20.3 Devirtualization

When the compiler can prove the concrete type at a call site, it converts a virtual/dynamic call to a direct call.

```
DEVIRTUALIZATION

Before:
  dyn Draw shape = Circle { r: 5.0 };
  shape.draw();  // virtual call through vtable

Compiler proves shape is always a Circle in this code path:
  Circle::draw(5.0)  // direct call, inlineable!

Performance impact:
  Virtual call:  ~10-20 ns (vtable lookup + indirect jump + cache miss)
  Direct call:   ~0.5 ns (or 0 ns if inlined)
```

### 20.4 Loop Unrolling

```c
// Original loop
for (int i = 0; i < 4; i++) arr[i] *= 2;

// After unrolling (compiler does this automatically):
arr[0] *= 2;
arr[1] *= 2;
arr[2] *= 2;
arr[3] *= 2;
// Eliminates loop overhead: counter increment, branch check
// Enables SIMD vectorization
```

### 20.5 Profile-Guided Optimization (PGO)

```
PGO WORKFLOW

Step 1: Compile with instrumentation
  clang -fprofile-instr-generate program.c -o program_instrumented

Step 2: Run with representative workload (generates profiling data)
  ./program_instrumented < typical_input.txt
  → creates default.profraw

Step 3: Compile optimized with profile data
  llvm-profdata merge -o profile.profdata default.profraw
  clang -fprofile-instr-use=profile.profdata -O2 program.c -o program_pgo

Benefits:
  - Hot functions are inlined aggressively
  - Cold functions are not inlined (saves binary size)
  - Branch predictions tuned to real data
  - Frequently executed loops unrolled

Rust PGO:
  RUSTFLAGS="-Cprofile-generate=/tmp/pgo-data" cargo build --release
  ./target/release/myapp < workload.txt
  llvm-profdata merge -o /tmp/merged.profdata /tmp/pgo-data
  RUSTFLAGS="-Cprofile-use=/tmp/merged.profdata" cargo build --release
```

### 20.6 Summary of Key Concepts: The Full Mental Model

```
FUNCTIONS: COMPLETE MENTAL MODEL

┌──────────────────────────────────────────────────────────────────────┐
│                        WHEN YOU WRITE A FUNCTION                     │
└──────────────────────────────────────────────────────────────────────┘

1. DECLARATION
   → Name, parameters (type + passing mode), return type
   → Visibility (public/private/export)
   → Calling convention (C ABI, Rust ABI, Go ABI)

2. COMPILATION
   → Compiler checks types, lifetimes (Rust), escape analysis (Go)
   → Decides: inline or emit as separate symbol
   → Applies: constant folding, dead code elimination
   → For generics: monomorphize (Rust) or use interface (Go)

3. LINKING
   → Symbol exported or kept internal
   → Resolved at link time (static) or runtime (dynamic)

4. RUNTIME CALL
   → Arguments marshaled to registers/stack per ABI
   → Stack frame pushed: local vars, saved regs, return addr
   → Control transferred to function body
   → Result placed in return register(s)
   → Stack frame popped, caller resumes

5. MEMORY SAFETY
   C:    programmer's responsibility (UB possible)
   Rust: borrow checker enforces at compile time (no UB in safe code)
   Go:   GC + bounds checks at runtime (panics instead of UB)

6. PERFORMANCE LEVERS
   → Inline small hot functions
   → Pass large structs by reference/pointer
   → Use static dispatch where possible
   → Leverage iterator/functional chains (lazy, zero-alloc in Rust)
   → Minimize allocations in hot paths
   → Use tail recursion → TCO or loops
```

---

## Quick Reference: Performance and Safety Cheat Sheet

```
PARAMETER PASSING

                  C              Rust              Go
──────────────────────────────────────────────────────
Small value     int x          x: i32           n int
(pass by value) Copy on call   Copy on call     Copy on call

Large value     pass *T        pass &T          pass *T
(by reference)  manual safety  borrow checked   pointer

Ownership       N/A            move semantics   N/A (GC manages)
Transfer                       T (not &T)

Mutable ref     int *x         x: &mut T        x *int


RETURN VALUES

                  C              Rust              Go
──────────────────────────────────────────────────────
Single          return val     return val        return val
Multiple        out params     (a, b) tuple      a, b, err
Error           int/NULL       Result<T,E>       (T, error)
Infallible      value          Ok(value)         value, nil


DISPATCH

                  C              Rust              Go
──────────────────────────────────────────────────────
Static          direct call    fn(T)             concrete type
Dynamic         vtable (manual) dyn Trait        interface
Cost            free/zero      zero/vtable       interface dispatch


CLOSURES

                  C              Rust              Go
──────────────────────────────────────────────────────
Native          No (manual)    Yes (Fn/FnMut/    Yes (func literal)
                               FnOnce)
Capture modes   Manual struct  auto/move         reference (by default)
Heap alloc?     Manual         Only if boxed     If variable escapes
Thread-safe?    Manual         Send+Sync types   WaitGroup/channels
```

---

*End of Guide*

> **Mental model summary:** A function is a named piece of code in the text segment, accessed through an ABI-defined contract. Every call pushes an activation record onto the stack, passes arguments through registers and/or the stack, executes, places a return value in a register, and pops the frame. Closures extend this with captured state. Generics extend this with type polymorphism (static) or interface dispatch (dynamic). Rust adds compile-time memory safety via ownership and borrows. Go adds goroutine concurrency with growable stacks. C gives you full control — and full responsibility.
