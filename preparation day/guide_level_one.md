# Systems Programming: Complete Interview Mastery Guide
### C · Go · Rust — From Fundamentals to World-Class Understanding

---

> **Mental Model First:** Before writing a single line, ask four questions about any piece of data:
> 1. **Where** is it stored? (stack / heap / static / read-only segment)
> 2. **Who** owns it?
> 3. **How long** is it valid?
> 4. **Is this behavior guaranteed** by the language standard, or is it undefined?
>
> If you can answer these four instantly, 90% of C interview traps become trivial.

---

# TABLE OF CONTENTS

```
PART I — C LANGUAGE: DEEP DIVE
  Chapter  1 — Memory Architecture: Stack, Heap, Static, Code Segments
  Chapter  2 — Pointers: The Core of C
  Chapter  3 — Arrays and Pointer Arithmetic
  Chapter  4 — Strings in C
  Chapter  5 — Functions: Parameters, Return Values, Pointers
  Chapter  6 — Structs, Unions, Enums, and Typedef
  Chapter  7 — Dynamic Memory Management
  Chapter  8 — Preprocessor and Compilation Pipeline
  Chapter  9 — Type System, Qualifiers, and Casts
  Chapter 10 — Bits, Bytes, and Low-Level Operations
  Chapter 11 — Output Prediction and Undefined Behavior Traps

PART II — GO LANGUAGE: DEEP DIVE
  Chapter 12 — The Go Scheduler and Yielding
  Chapter 13 — Large File I/O in Go

PART III — RUST: THE MEMORY-SAFE LENS
  Chapter 14 — Ownership, Borrowing, and Lifetimes vs C
  Chapter 15 — Rust Equivalents to Every C Trap
```

---

# PART I — C LANGUAGE: DEEP DIVE

---

## CHAPTER 1: MEMORY ARCHITECTURE

### What is a Process Memory Layout?

When your C program runs, the operating system gives it a virtual address space.
This space is divided into distinct **segments**, each with a different purpose,
lifetime, and set of rules.

```
HIGH ADDRESS (e.g., 0xFFFFFFFF on 32-bit)
┌─────────────────────────────────────────┐
│           KERNEL SPACE                  │  ← OS kernel, never directly accessed
│         (not accessible)               │
├─────────────────────────────────────────┤
│                                         │
│              STACK                      │  ← grows DOWNWARD ↓
│         (local variables,              │
│          function frames,              │
│          return addresses)             │
│                                         │
│          ↓ grows down                  │
│                                         │
│   ┌──────────────────────────────┐      │
│   │  Stack Frame: main()         │      │
│   │  ┌────────────────────────┐  │      │
│   │  │  local int x = 5       │  │      │
│   │  │  saved return address  │  │      │
│   │  │  saved base pointer    │  │      │
│   │  └────────────────────────┘  │      │
│   └──────────────────────────────┘      │
│                                         │
│   ···· unmapped gap (guard page) ···   │
│                                         │
│          ↑ grows up                    │
│                                         │
│              HEAP                       │  ← grows UPWARD ↑
│         (malloc/calloc/realloc)        │
│                                         │
├─────────────────────────────────────────┤
│           BSS SEGMENT                   │  ← uninitialized globals (zeroed)
│   int global_count;                    │     e.g.: int g; (no = value)
├─────────────────────────────────────────┤
│           DATA SEGMENT                  │  ← initialized globals & statics
│   int magic = 42;                      │     e.g.: int g = 10;
│   static int counter = 0;             │
├─────────────────────────────────────────┤
│           TEXT SEGMENT (CODE)           │  ← machine instructions (read-only)
│   [compiled instructions of main()]    │
│   [compiled instructions of foo()]     │
├─────────────────────────────────────────┤
│           READ-ONLY DATA               │  ← string literals
│   "hello, world\0"                    │     char *s = "hello" → points HERE
│   "error: null\0"                     │
└─────────────────────────────────────────┘
LOW ADDRESS (e.g., 0x00000000)
```

**Key insight:** Each segment has different rules about who creates it, who frees it,
and how long it lives. This is the root cause of every memory bug in C.

---

### Q1: What is the difference between Stack and Heap Memory?

#### Stack Memory

```
STACK CHARACTERISTICS:
┌──────────────────────────────────────────────────────────┐
│  Allocation:  Automatic — done by the CPU (push/pop)     │
│  Deallocation: Automatic — when function returns         │
│  Speed:       Extremely fast — single instruction        │
│  Size:        Limited (typically 1–8 MB per thread)      │
│  Data:        Local variables, function parameters,      │
│               return addresses, saved registers          │
│  Lifetime:    Tied to the enclosing scope/function       │
│  Thread:      Each thread has its OWN stack              │
└──────────────────────────────────────────────────────────┘

Call stack when foo() calls bar():

High address
┌──────────────────────────┐
│     main() frame         │  ← first frame created
│  int a = 1;              │
│  return address          │
├──────────────────────────┤
│     foo() frame          │  ← second frame
│  int b = 2;              │
│  return address → main   │
├──────────────────────────┤  ← current stack pointer (SP)
│     bar() frame          │  ← active frame
│  int c = 3;              │
│  return address → foo    │
└──────────────────────────┘
Low address

When bar() returns:
- frame is popped
- stack pointer moves up
- memory is "freed" (logically)
- but bytes REMAIN there (just considered reusable)
```

#### Heap Memory

```
HEAP CHARACTERISTICS:
┌──────────────────────────────────────────────────────────┐
│  Allocation:  Manual — you call malloc()/calloc()        │
│  Deallocation: Manual — you call free()                  │
│  Speed:       Slower — involves allocator bookkeeping    │
│  Size:        Large (limited by RAM + swap)              │
│  Data:        Long-lived objects, dynamic structures     │
│  Lifetime:    From malloc() until free() (you decide)   │
│  Thread:      Shared between all threads (needs locks)   │
└──────────────────────────────────────────────────────────┘

Heap layout (conceptual):

┌──────────────────────────────────────────────────────────┐
│  [HEADER][..data 32 bytes..][HEADER][..data 64 bytes..]  │
│           ↑                         ↑                    │
│        malloc'd                  malloc'd                │
│           chunk 1                  chunk 2               │
│                                                          │
│  [HEADER][....free block 128 bytes....................] ] │
│           ↑                                              │
│         free'd chunk (available for reuse)               │
└──────────────────────────────────────────────────────────┘

Each malloc() call:
1. Allocator searches free list
2. Finds a free block ≥ requested size
3. Splits it, marks as used
4. Returns pointer to data portion
```

#### C: Stack vs Heap example

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* Stack allocation: automatic, fast, scoped */
void stack_example(void) {
    int x = 42;             /* stack — valid only inside this function */
    char buf[128];          /* stack — 128 bytes on stack frame */
    buf[0] = 'A';
    buf[1] = '\0';
    printf("stack x=%d buf=%s\n", x, buf);
    /* x and buf destroyed here automatically */
}

/* Heap allocation: manual, flexible, long-lived */
int *heap_example(size_t n) {
    int *arr = malloc(n * sizeof(int));  /* heap allocation */
    if (arr == NULL) {
        return NULL;  /* always check — malloc can fail */
    }
    for (size_t i = 0; i < n; i++) {
        arr[i] = (int)i * 2;
    }
    return arr;  /* safe: heap memory outlives this function */
}

int main(void) {
    stack_example();

    int *data = heap_example(10);
    if (data == NULL) {
        fprintf(stderr, "allocation failed\n");
        return 1;
    }

    for (int i = 0; i < 10; i++) {
        printf("%d ", data[i]);
    }
    printf("\n");

    free(data);     /* MUST free — otherwise memory leak */
    data = NULL;    /* defensive: set to NULL after free */

    return 0;
}
```

#### Go: Stack vs Heap (escape analysis)

```go
package main

import "fmt"

// Go automatically decides stack vs heap via ESCAPE ANALYSIS.
// The compiler checks if data "escapes" the function scope.

// This stays on stack — does not escape
func stackLocal() int {
    x := 42    // stack allocated
    return x   // value copied — x does not escape
}

// This escapes to heap — pointer returned
func heapEscape() *int {
    x := 42    // compiler detects: x's address escapes
    return &x  // pointer returned — x promoted to heap
}

// Check with: go build -gcflags='-m' ./...

func main() {
    a := stackLocal()
    b := heapEscape()
    fmt.Println(a, *b)
}
```

#### Rust: Stack vs Heap

```rust
fn main() {
    // Stack allocation — fixed size, known at compile time
    let x: i32 = 42;              // stack
    let arr: [i32; 128] = [0; 128]; // stack (512 bytes)

    // Heap allocation — Box<T> is Rust's malloc equivalent
    let y: Box<i32> = Box::new(42); // heap
    // Box<T> is automatically freed when it goes out of scope
    // No manual free() needed — RAII handles it

    // Vec<T> — heap-allocated growable array
    let mut v: Vec<i32> = Vec::with_capacity(10);
    for i in 0..10 {
        v.push(i * 2);
    }
    // v is freed here at end of scope — no free() call

    println!("stack x={}, heap y={}", x, y);
    println!("vec: {:?}", v);
}
```

---

### Q2: What happens when a function returns the address of a local variable?

This is one of the most important C traps. Study every detail.

```
BEFORE createMessage() returns:

Stack (high to low):
┌──────────────────────────────────────────┐
│  main() frame                            │
│  char *msg;   (uninitialized)            │
│  return address → OS                    │
├──────────────────────────────────────────┤  ← frame boundary
│  createMessage() frame                   │
│  buff[500]:                              │
│  ┌─┬─┬─┬─┬─┬─┬─┬─┬─┬─┬─┐              │
│  │h│o│w│ │a│r│e│ │y│o│u│ → \0 ...     │
│  └─┴─┴─┴─┴─┴─┴─┴─┴─┴─┴─┘              │
│  ptr = 0x7fff_a100  (points to buff)    │
│  return address → main                  │
└──────────────────────────────────────────┘ ← Stack Pointer (SP)

AFTER createMessage() returns:

Stack:
┌──────────────────────────────────────────┐
│  main() frame                            │
│  char *msg = 0x7fff_a100  ← DANGER      │  msg holds old address
├──────────────────────────────────────────┤  ← frame boundary
│  [DESTROYED FRAME — memory REUSABLE]     │  buff is logically gone
│  bytes may still read "how are you"      │  but this is NOT guaranteed
│  OR may be overwritten by next call      │
└──────────────────────────────────────────┘
```

#### C: The exact buggy code and all fixes

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* ================================================================
 * VERSION 1: DANGLING POINTER BUG — do not do this
 * ================================================================ */
char *createMessage_WRONG(void) {
    char buff[500];           /* LOCAL stack array */
    strcpy(buff, "how are you");
    char *ptr = buff;
    return ptr;               /* DANGER: returning address of stack variable */
    /* After return: buff is destroyed, ptr is dangling */
}

/* ================================================================
 * VERSION 2: FIX — static storage (persists for program lifetime)
 *
 * TRADE-OFF: Only one copy exists; not thread-safe;
 *            value overwritten on every call.
 * ================================================================ */
char *createMessage_static(void) {
    static char buff[500];    /* static: lives in DATA segment, not stack */
    strcpy(buff, "how are you");
    return buff;              /* Safe: buff exists for program lifetime */
}

/* ================================================================
 * VERSION 3: FIX — heap allocation (caller must free)
 *
 * This is the most flexible and correct approach.
 * Ownership contract: caller is responsible for free().
 * ================================================================ */
char *createMessage_heap(void) {
    char *buff = malloc(500); /* heap allocation */
    if (buff == NULL) {
        return NULL;          /* handle allocation failure */
    }
    strncpy(buff, "how are you", 499);
    buff[499] = '\0';         /* guarantee null termination */
    return buff;              /* Safe: heap memory outlives function */
}

/* ================================================================
 * VERSION 4: FIX — caller provides the buffer (best practice)
 *
 * Ownership is crystal clear. Caller owns the buffer.
 * This is the pattern used in the C standard library.
 * ================================================================ */
int createMessage_caller(char *out, size_t out_size) {
    if (out == NULL || out_size == 0) {
        return -1;
    }
    int written = snprintf(out, out_size, "how are you");
    return written; /* returns number of bytes written */
}

int main(void) {
    /* Wrong: undefined behavior (may "work" or may crash) */
    /* char *msg = createMessage_WRONG(); */
    /* printf("%s\n", msg); */

    /* Static version */
    char *s1 = createMessage_static();
    printf("static: %s\n", s1);

    /* Heap version */
    char *s2 = createMessage_heap();
    if (s2 == NULL) {
        fprintf(stderr, "allocation failed\n");
        return 1;
    }
    printf("heap: %s (len=%zu)\n", s2, strlen(s2));
    free(s2);
    s2 = NULL;

    /* Caller-buffer version */
    char s3[500];
    int n = createMessage_caller(s3, sizeof(s3));
    if (n < 0) {
        fprintf(stderr, "createMessage failed\n");
        return 1;
    }
    printf("caller-buf: %s (written=%d)\n", s3, n);

    return 0;
}
```

#### Go: Go does not have this problem

```go
package main

import "fmt"

// In Go, the compiler uses escape analysis.
// If a variable's address escapes the function, Go
// automatically allocates it on the HEAP — not the stack.
// So there is no dangling pointer problem.

func createMessage() *string {
    msg := "how are you"  // compiler detects: address escapes
    return &msg           // Go promotes msg to heap — SAFE
}

func main() {
    s := createMessage()
    fmt.Println(*s)      // always safe in Go
}
```

#### Rust: The borrow checker prevents this at compile time

```rust
// Rust's borrow checker makes this a COMPILE ERROR.
// You cannot return a reference to a local variable.

// This WILL NOT COMPILE:
// fn create_message_wrong() -> &str {
//     let buff = String::from("how are you");
//     &buff  // ERROR: buff dropped at end of scope
//            // compiler says: "returns reference to local data `buff`"
// }

// Correct: return owned String (heap-allocated)
fn create_message_owned() -> String {
    String::from("how are you")   // heap String, ownership transferred
}

// Correct: caller provides buffer (zero-allocation pattern)
fn create_message_into(out: &mut String) {
    out.clear();
    out.push_str("how are you");
}

// Correct: return &'static str — string literal in read-only segment
fn create_message_static() -> &'static str {
    "how are you"   // points to read-only binary segment — always valid
}

fn main() {
    let s1 = create_message_owned();
    println!("owned: {}", s1);

    let mut buf = String::new();
    create_message_into(&mut buf);
    println!("into buf: {}", buf);

    let s3 = create_message_static();
    println!("static: {}", s3);
}
```

---

### Q3: What is Undefined Behavior in C?

**Undefined Behavior (UB)** means the C standard makes NO guarantee about what
happens. The compiler is free to:

- Produce the "expected" output
- Produce garbage output
- Crash the program
- Delete your files (theoretically)
- Optimize the code as if the UB never happens
- Emit different code in debug vs release builds

```
UB is NOT a runtime concept — it is a COMPILE-TIME contract violation.
The standard says: "If you do X, anything can happen."
The compiler ASSUMES you never trigger UB and uses that to optimize.

Example: signed integer overflow

int x = INT_MAX;
int y = x + 1;   // UB: signed overflow

Compiler reasoning:
  "x + 1 will never overflow (programmer promised no UB)"
  Therefore: y > x must always be true
  → Compiler can eliminate any check `if (y > x)` as always-true
  → Entire security check DELETED from binary
  → Your program now has a security vulnerability

This is real. CVEs have been created this way.
```

#### Categories of Undefined Behavior

```
┌──────────────────────────────────────────────────────────────────┐
│                   UNDEFINED BEHAVIOR CATEGORIES                  │
├──────────────────────────────────────────────────────────────────┤
│ 1. DANGLING POINTER     — accessing freed or out-of-scope memory │
│ 2. NULL DEREFERENCE     — dereferencing a null pointer           │
│ 3. SIGNED OVERFLOW      — int arithmetic that overflows          │
│ 4. ARRAY OOB            — accessing arr[-1] or arr[size]         │
│ 5. UNINITIALIZED READ   — reading variable before assignment     │
│ 6. DATA RACE            — concurrent unsynchronized RW           │
│ 7. STRICT ALIASING      — accessing object via incompatible type │
│ 8. MISALIGNED ACCESS    — reading int from odd address           │
│ 9. DIVISION BY ZERO     — integer division by zero               │
│ 10. INVALID SHIFT       — shifting by ≥ bit width                │
│ 11. SEQUENCE POINT UB   — i++ + ++i; modifying twice in expr     │
│ 12. FORMAT MISMATCH     — printf("%d", ptr)                      │
└──────────────────────────────────────────────────────────────────┘
```

#### C: Demonstrating UB (never do in production)

```c
#include <stdio.h>
#include <limits.h>

int main(void) {
    /* UB 1: Signed overflow */
    int x = INT_MAX;
    /* int y = x + 1;  // UB: signed overflow */

    /* UB 2: Uninitialized read */
    int z;
    /* printf("%d\n", z);  // UB: z has indeterminate value */

    /* UB 3: Array out of bounds */
    int arr[5] = {1, 2, 3, 4, 5};
    /* int bad = arr[10];  // UB: out of bounds access */

    /* UB 4: Null dereference */
    int *p = NULL;
    /* *p = 42;  // UB: null dereference */

    /* UB 5: Division by zero */
    /* int d = 5 / 0;  // UB */

    /* SAFE: unsigned overflow is DEFINED (wraps around) */
    unsigned int u = UINT_MAX;
    unsigned int v = u + 1;  /* defined: wraps to 0 */
    printf("unsigned wrap: %u\n", v);  /* always prints 0 */

    (void)x;
    return 0;
}
```

#### Rust and Go: No UB by default

```rust
// Rust prevents UB through the type system and borrow checker.
// In safe Rust, these are IMPOSSIBLE:
//   - null dereferences (Option<T> forces explicit handling)
//   - dangling pointers (borrow checker prevents)
//   - data races (Send/Sync traits enforce safety)
//   - use-after-free (ownership system prevents)

fn main() {
    // Array bounds: PANIC at runtime (not UB, not silent corruption)
    let arr = [1, 2, 3, 4, 5];
    // let x = arr[10];  // panics with index out of bounds — not UB

    // Overflow in debug mode: PANIC
    // Overflow in release mode: wraps (well-defined, not UB)
    let x: i32 = i32::MAX;
    // let y = x + 1;  // debug: panics; release: wraps (use wrapping_add)
    let y = x.wrapping_add(1);  // explicit wrap — always defined
    println!("wrapping: {}", y);

    // No null pointers in safe Rust:
    let maybe: Option<i32> = None;
    match maybe {
        Some(v) => println!("got {}", v),
        None    => println!("no value"),  // forced to handle None
    }
}
```

```go
// Go also prevents most UB:
// - nil dereferences: runtime PANIC (not silent)
// - array OOB: runtime PANIC
// - integer overflow: wraps (defined)
// - data races: detected by -race flag
package main

import "fmt"

func main() {
    arr := [5]int{1, 2, 3, 4, 5}
    // arr[10]  // runtime panic: index out of range — not silent UB

    var p *int = nil
    // *p = 42  // runtime panic: nil pointer dereference

    // Integer overflow: wraps (defined in Go)
    var x int32 = 2147483647 // INT32_MAX
    x++                      // wraps to -2147483648 (defined)
    fmt.Println(x)
}
```

---

### Q4: What is a Dangling Pointer?

A **dangling pointer** is a pointer that points to memory that has been freed,
gone out of scope, or otherwise invalidated.

```
Timeline of a dangling pointer:

Time →  T1          T2            T3             T4
        malloc()    use ptr       free(ptr)      use ptr again
           ↓            ↓              ↓               ↓
        ┌──────┐    ┌──────┐     ┌──────────┐   ┌──────────┐
Heap:   │ data │    │ data │     │ [FREED]  │   │ CORRUPT  │
        └──────┘    └──────┘     └──────────┘   └──────────┘
           ↑            ↑              ↑               ↑
        ptr valid    ptr valid     ptr DANGLING    UB: crash,
                                                  silent corrupt,
                                                  security hole
```

#### C: Three sources of dangling pointers

```c
#include <stdio.h>
#include <stdlib.h>

/* Source 1: Returning address of local variable */
int *get_local(void) {
    int x = 42;
    return &x;     /* x is on the stack; after return, x is destroyed */
}

/* Source 2: Use-after-free */
void use_after_free_demo(void) {
    int *p = malloc(sizeof(int));
    if (p == NULL) return;
    *p = 100;
    free(p);       /* p is now dangling */
    /* *p = 200;  */ /* UB: use after free */
}

/* Source 3: Pointer to expired scope */
int *get_block_local(void) {
    int result;
    {
        int x = 42;
        result = x;    /* copy the VALUE — safe */
        /* return &x;  */ /* would be dangling — x destroyed at } */
    }
    return NULL;  /* no pointer to return */
}

/* DEFENSIVE PATTERN: Always null the pointer after free */
void safe_free(void **ptr) {
    if (ptr != NULL && *ptr != NULL) {
        free(*ptr);
        *ptr = NULL;   /* prevent dangling pointer */
    }
}

int main(void) {
    int *p = malloc(sizeof(int));
    if (p == NULL) return 1;
    *p = 77;
    printf("before free: %d\n", *p);

    safe_free((void **)&p);
    /* p is now NULL — any access will crash (SIGSEGV) rather than silently corrupt */
    /* if (p) *p = 5;  // safe: condition is false */

    return 0;
}
```

---

### Q5: What is a Null Pointer?

A **null pointer** is a pointer whose value is the null address.
In C: the address 0 (but conceptually: "points to nothing").
Dereferencing it is **undefined behavior** (on most systems: SIGSEGV crash).

```
Pointer values:

  Valid pointer:     ┌────────────────┐
   ptr = 0x7fff100  │   some data    │
                     └────────────────┘

  NULL pointer:
   ptr = 0x000_0000  → points at address 0 → OS protects this page
                        Any access → SIGSEGV (segmentation fault)
```

```c
#include <stdio.h>
#include <stdlib.h>

/* Null pointer constants in C */
/* NULL is defined as (void *)0 in <stddef.h> / <stdlib.h> */

int main(void) {
    int *p = NULL;

    /* Testing for null before use — always do this */
    if (p == NULL) {
        printf("p is null — safe branch\n");
    } else {
        printf("p = %d\n", *p);
    }

    /* malloc returns NULL on failure */
    int *big = malloc(1024UL * 1024 * 1024 * 1024); /* 1 TB — will fail */
    if (big == NULL) {
        fprintf(stderr, "malloc failed — NULL returned\n");
        return 1;
    }
    free(big);

    return 0;
}
```

---

### Q6: What is a Wild Pointer?

A **wild pointer** (also called *uninitialized pointer*) is a pointer variable
that has been declared but never assigned a value.
It contains whatever garbage bytes happen to be in that stack location.

```
Wild pointer:

Stack frame:
┌─────────────────────────┐
│  int *p;                │  ← p contains: 0xABCD_BEEF (random garbage)
│  ...                    │
└─────────────────────────┘

If you do:  *p = 42;
You write to address 0xABCD_BEEF — could be:
  - kernel memory → SIGSEGV
  - another variable → silent data corruption
  - your own struct → corrupt your data structure
  - code segment → corrupt instructions → weird crashes later
```

```c
#include <stdio.h>

int main(void) {
    int *wild;          /* wild pointer — uninitialized */
    /* *wild = 5;  */   /* UB: could write anywhere in memory */

    /* ALWAYS initialize pointers */
    int *safe = NULL;   /* null — safe, crashable if misused but detectable */
    int x = 42;
    int *valid = &x;    /* pointing to valid memory */

    printf("valid: %d\n", *valid);  /* safe */

    return 0;
}
```

**Rule**: Always initialize pointers to `NULL` or a valid address.
A null pointer crash is *better* than a wild pointer — it is detectable.

---

### Q7: What is the difference between NULL and 0 in pointer context?

```c
#include <stdio.h>
#include <stddef.h>  /* defines NULL */

int main(void) {
    /* NULL is a null pointer constant */
    int *p1 = NULL;     /* correct — pointer null */

    /* 0 is also a null pointer constant in C */
    int *p2 = 0;        /* technically valid in C, but avoid */

    /* '\0' is the null CHARACTER (integer value 0) */
    char c = '\0';      /* string terminator */

    /* They all have value 0, but different semantic meaning */
    /* Use NULL for pointers, '\0' for characters, 0 for integers */

    printf("p1 == NULL: %d\n", p1 == NULL);  /* 1 */
    printf("p2 == NULL: %d\n", p2 == NULL);  /* 1 */
    printf("p1 == p2:   %d\n", p1 == p2);    /* 1 */

    return 0;
}

/*
 * Summary:
 *   NULL   = null pointer constant (use for pointers)
 *   0      = integer zero (use for integers)
 *   '\0'   = null character (use for chars/strings)
 *   false  = boolean false (C99: <stdbool.h>)
 *
 * Same VALUE, different INTENT.
 * Modern C: prefer NULL for pointers explicitly.
 */
```

---

## CHAPTER 2: POINTERS — THE CORE OF C

### What is a Pointer?

A **pointer** is a variable that stores a **memory address**.
Every variable in C lives at some address. A pointer lets you store and
manipulate that address directly.

```
Variable x in memory:

Address:  0x7fff_1000
          ┌─────────────┐
          │   42        │  ← value of x (int, 4 bytes)
          └─────────────┘

Pointer p pointing to x:

Address:  0x7fff_2000
          ┌─────────────────┐
          │  0x7fff_1000    │  ← value of p (the address of x)
          └─────────────────┘

Dereferencing: *p reads the value AT the address stored in p
    *p → go to 0x7fff_1000 → read 42

Taking address: &x reads the address OF x
    &x → 0x7fff_1000
```

```c
#include <stdio.h>

int main(void) {
    int x = 42;

    int *p = &x;    /* p holds the address of x */

    printf("x         = %d\n", x);          /* 42 */
    printf("&x        = %p\n", (void *)&x); /* address of x */
    printf("p         = %p\n", (void *)p);  /* same address */
    printf("*p        = %d\n", *p);         /* 42 — dereference */

    *p = 100;       /* modify x through p */
    printf("x after   = %d\n", x);          /* 100 */

    return 0;
}
```

---

### Q8: What is the difference between malloc, calloc, realloc, and free?

```
MEMORY ALLOCATION FUNCTIONS:
┌──────────────────────────────────────────────────────────────────────┐
│  malloc(size)                                                         │
│    → allocates `size` bytes                                          │
│    → contents UNINITIALIZED (random garbage)                         │
│    → returns void* or NULL on failure                                │
│                                                                       │
│  calloc(count, size)                                                 │
│    → allocates count * size bytes                                    │
│    → ALL BYTES ZEROED (initialized to 0)                             │
│    → slower than malloc (must zero memory)                           │
│    → returns void* or NULL on failure                                │
│    → also checks for integer overflow in count*size                  │
│                                                                       │
│  realloc(ptr, new_size)                                              │
│    → resize a previously malloc'd block                              │
│    → may move the block to a new address                             │
│    → if new_size == 0: implementation-defined (avoid)               │
│    → if ptr == NULL: equivalent to malloc(new_size)                 │
│    → returns NEW pointer or NULL on failure                          │
│    → if NULL returned, ORIGINAL ptr still valid                      │
│                                                                       │
│  free(ptr)                                                           │
│    → releases memory back to heap allocator                          │
│    → ptr must be exactly what malloc/calloc/realloc returned         │
│    → double-free is UB                                               │
│    → free(NULL) is safe (no-op)                                      │
└──────────────────────────────────────────────────────────────────────┘
```

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main(void) {
    const size_t N = 10;

    /* ── malloc: uninitialized ─────────────────────────────── */
    int *a = malloc(N * sizeof(int));
    if (a == NULL) {
        fprintf(stderr, "malloc failed\n");
        return 1;
    }
    /* a[0..N-1] contain garbage — MUST initialize before reading */
    for (size_t i = 0; i < N; i++) {
        a[i] = (int)i;
    }

    /* ── calloc: zero-initialized ─────────────────────────── */
    int *b = calloc(N, sizeof(int));
    if (b == NULL) {
        free(a);
        fprintf(stderr, "calloc failed\n");
        return 1;
    }
    /* b[0..N-1] are guaranteed to be 0 */
    printf("b[5] = %d\n", b[5]);  /* 0 */

    /* ── realloc: resize ──────────────────────────────────── */
    const size_t NEW_N = 20;
    int *a2 = realloc(a, NEW_N * sizeof(int));
    if (a2 == NULL) {
        /* IMPORTANT: a is STILL VALID if realloc fails */
        free(a);
        free(b);
        fprintf(stderr, "realloc failed\n");
        return 1;
    }
    a = a2;  /* update pointer — a2 may be a different address */
    for (size_t i = N; i < NEW_N; i++) {
        a[i] = (int)i * 2;
    }

    /* ── free: release ────────────────────────────────────── */
    free(a);    /* safe */
    free(b);    /* safe */
    free(NULL); /* always safe — no-op */

    a = NULL;   /* defensive null */
    b = NULL;

    return 0;
}
```

```go
// Go: automatic memory management via garbage collector
// You allocate, GC frees. No manual free().
package main

import "fmt"

func main() {
    // make: for slices, maps, channels (returns initialized value)
    slice := make([]int, 10)          // length=10, cap=10, zeroed
    slice2 := make([]int, 0, 20)      // length=0, cap=20

    // new: allocates zeroed memory, returns pointer
    p := new(int)   // equivalent to: var x int; &x (on heap)
    *p = 42

    // append: realloc-like growth
    for i := 0; i < 30; i++ {
        slice2 = append(slice2, i)    // may realloc internally
    }

    fmt.Println(slice[5], *p, len(slice2))
}
```

```rust
use std::alloc::{alloc, dealloc, realloc, Layout};

fn main() {
    // Rust's allocator API (low-level, like C's malloc/free)
    // Normally you'd use Vec<T>, Box<T> etc.

    unsafe {
        let layout = Layout::array::<i32>(10).unwrap();

        // malloc equivalent
        let ptr = alloc(layout) as *mut i32;
        if ptr.is_null() {
            panic!("allocation failed");
        }

        // Initialize (like calloc — must do manually after alloc)
        for i in 0..10_isize {
            ptr.offset(i).write(i as i32 * 2);
        }

        // realloc equivalent
        let new_layout = Layout::array::<i32>(20).unwrap();
        let new_ptr = realloc(ptr as *mut u8, layout, new_layout.size()) as *mut i32;
        if new_ptr.is_null() {
            dealloc(ptr as *mut u8, layout);
            panic!("realloc failed");
        }

        // free equivalent
        dealloc(new_ptr as *mut u8, new_layout);
    }

    // HIGH-LEVEL (normal Rust): use Vec<T> instead
    let mut v: Vec<i32> = Vec::with_capacity(10);
    for i in 0..10 {
        v.push(i * 2);
    }
    v.resize(20, 0);   // realloc-like
    println!("{:?}", &v[..5]);
    // v is freed automatically at end of scope
}
```

---

### Q9 & Q10: Double Free and Memory Leaks

```
DOUBLE FREE:
  T1: free(ptr) → allocator marks block as free
  T2: free(ptr) → allocator sees already-free block
                 → corrupts heap metadata
                 → UB: may crash, may allow attacker to
                       control next malloc() return value

Heap before double free:
  [CHUNK HEADER | data area | CHUNK FOOTER]
  Header contains: size, flags, forward/back links

After first free():
  [FREE HEADER | fd=next_free | bk=prev_free | FOOTER]

After second free():
  [CORRUPTED HEADER | fd overwritten | bk overwritten]
  Next malloc() may return an attacker-controlled address.
  This is the basis of "heap exploitation" security attacks.

MEMORY LEAK:
  malloc() ← allocate block
  ...use block...
  [function returns, program continues, block never freed]
  → block is inaccessible (lost pointer)
  → block is still counted as "in use"
  → heap grows unboundedly
  → eventually: OOM, OS kills process
```

```c
#include <stdio.h>
#include <stdlib.h>

/* Memory leak example */
void leaky_function(void) {
    int *p = malloc(1024);
    if (p == NULL) return;
    p[0] = 1;
    /* forget to free — 1024 bytes leaked every call */
}

/* Double free example (commented out — DO NOT RUN) */
void double_free_demo(void) {
    int *p = malloc(sizeof(int));
    if (p == NULL) return;
    *p = 42;
    free(p);
    /* free(p);  */ /* UB: double free — heap corruption */
    p = NULL;       /* prevention: null after free */
}

/* Correct pattern using a cleanup label */
int function_with_cleanup(void) {
    char *a = NULL;
    int  *b = NULL;
    int   result = 0;

    a = malloc(256);
    if (a == NULL) { result = -1; goto cleanup; }

    b = malloc(sizeof(int) * 64);
    if (b == NULL) { result = -1; goto cleanup; }

    /* ... do work ... */
    a[0] = 'X';
    b[0] = 99;
    result = 0;

cleanup:
    free(a);   /* free(NULL) is always safe */
    free(b);
    return result;
}

int main(void) {
    for (int i = 0; i < 1000; i++) {
        leaky_function(); /* leaks 1024 bytes every iteration */
    }
    /* after this loop: 1 MB leaked */

    double_free_demo();  /* prevented by null-after-free */

    int rc = function_with_cleanup();
    printf("rc = %d\n", rc);

    return 0;
}
```

---

## CHAPTER 3: ARRAYS AND POINTER ARITHMETIC

### What is an Array in C?

An array is a **contiguous block of same-type elements in memory**.
There is NO array object at runtime — the array name decays to a pointer.

```
int arr[5] = {10, 20, 30, 40, 50};

Memory layout (assuming int = 4 bytes):

Address:   0x1000  0x1004  0x1008  0x100C  0x1010
           ┌──────┬──────┬──────┬──────┬──────┐
arr:       │  10  │  20  │  30  │  40  │  50  │
           └──────┴──────┴──────┴──────┴──────┘
             [0]    [1]    [2]    [3]    [4]

arr       → 0x1000  (the address of element [0])
arr + 1   → 0x1004  (address of element [1])
arr + 2   → 0x1008  (address of element [2])
*(arr+2)  → 30      (value at element [2])
arr[2]    → 30      (IDENTICAL to *(arr+2))
```

### Q11: What is the difference between an Array and a Pointer?

```
KEY DIFFERENCES:

Array:
  int arr[5];
  - arr IS the array — it IS the storage
  - sizeof(arr) = 5 * sizeof(int) = 20 bytes
  - arr's address cannot change — &arr == arr (in value)
  - you CANNOT do: arr = another_array;
  - arr decays to &arr[0] in most expressions

Pointer:
  int *ptr = arr;
  - ptr IS NOT the array — it POINTS TO the array
  - sizeof(ptr) = 8 bytes (64-bit) — size of address
  - ptr can be changed: ptr = another_array;
  - ptr arithmetic is valid: ptr++, ptr + 3, etc.
```

### Q12: Why does an Array Decay to a Pointer?

```
Array decay: in most expressions, the array name is automatically
converted (decays) to a pointer to its first element.

This does NOT happen in:
  1. sizeof(arr)          — gives full array size
  2. &arr                 — gives address of the array itself
  3. _Alignof(arr)        — gives alignment
  4. As initializer for another char array: char b[] = "hello";
```

### Q13: sizeof(arr) vs sizeof(ptr)

```c
#include <stdio.h>

void wrong_sizeof(int *p, size_t n) {
    /* Inside a function that received array as parameter: */
    printf("sizeof(p) = %zu\n", sizeof(p)); /* 8 — pointer size, NOT array! */
    /* You MUST pass n separately */
    for (size_t i = 0; i < n; i++) {
        printf("%d ", p[i]);
    }
    printf("\n");
}

int main(void) {
    int arr[5] = {1, 2, 3, 4, 5};
    int *ptr = arr;

    printf("sizeof(arr) = %zu\n", sizeof(arr));  /* 20 (5 * 4) */
    printf("sizeof(ptr) = %zu\n", sizeof(ptr));  /* 8 (pointer) */

    /* Number of elements */
    size_t count = sizeof(arr) / sizeof(arr[0]); /* 20 / 4 = 5 */
    printf("element count = %zu\n", count);

    wrong_sizeof(arr, count);  /* must pass count explicitly */

    return 0;
}
```

### Q14 & Q15: Pointer Arithmetic

```
Pointer arithmetic works in UNITS of the pointed-to type.

int arr[5] = {10, 20, 30, 40, 50};
int *p = arr;   // p = 0x1000

p + 0  →  0x1000 (arr[0])
p + 1  →  0x1004 (arr[1])   adds sizeof(int) = 4
p + 2  →  0x1008 (arr[2])
p + 3  →  0x100C (arr[3])
p + 4  →  0x1010 (arr[4])

Equivalence:
  p[i]   ≡  *(p + i)   ← IDENTITY in C
  arr[i] ≡  *(arr + i) ← also true

Subtraction:
  p + 4 - p  = 4  (number of elements between — ptrdiff_t)
  Note: (char*)(p+4) - (char*)p = 16 (bytes)
```

```c
#include <stdio.h>
#include <stddef.h>

int main(void) {
    int arr[5] = {10, 20, 30, 40, 50};
    int *p = arr;

    /* Equivalences */
    printf("arr[2]      = %d\n", arr[2]);       /* 30 */
    printf("*(arr + 2)  = %d\n", *(arr + 2));   /* 30 */
    printf("p[2]        = %d\n", p[2]);          /* 30 */
    printf("*(p + 2)    = %d\n", *(p + 2));      /* 30 */
    printf("2[arr]      = %d\n", 2[arr]);        /* 30 — legal but bizarre */

    /* Distance between pointers */
    int *end = arr + 5;       /* one past last element */
    ptrdiff_t dist = end - p; /* 5 — element count */
    printf("distance    = %td\n", dist);

    /* Iterate using pointer arithmetic */
    printf("forward:  ");
    for (int *q = arr; q < arr + 5; q++) {
        printf("%d ", *q);
    }
    printf("\n");

    printf("backward: ");
    for (int *q = arr + 4; q >= arr; q--) {
        printf("%d ", *q);
    }
    printf("\n");

    return 0;
}
```

### Q16: char *s vs char s[] — Critical Distinction

```
char *s = "hello";      vs      char s[] = "hello";

  char *s = "hello";
  ┌───────────────────┐        ┌──────────────────────┐
  │  s (8 bytes)      │───────▶│  "hello\0"           │
  │  = address        │        │  (read-only segment) │
  └───────────────────┘        └──────────────────────┘
  - s is a pointer variable
  - string lives in read-only data segment
  - s[0] = 'H'; → UNDEFINED BEHAVIOR (may SIGSEGV)
  - s can be reassigned: s = "world"; ← valid

  char s[] = "hello";
  ┌─────────────────────────────────────┐
  │  s: [h][e][l][l][o][\0]            │  stack (or wherever s lives)
  └─────────────────────────────────────┘
  - s IS the array (stack-allocated)
  - bytes copied from read-only to stack at initialization
  - s[0] = 'H'; → VALID (modifying local copy)
  - s cannot be reassigned: s = "world"; ← compile error
```

```c
#include <stdio.h>
#include <string.h>

int main(void) {
    /* Pointer to string literal — READ ONLY */
    char *p = "hello";
    printf("p[0] = %c\n", p[0]);
    /* p[0] = 'H';  */  /* UB: modifying read-only memory */

    /* Array — WRITABLE COPY */
    char a[] = "hello";
    printf("a[0] = %c\n", a[0]);
    a[0] = 'H';  /* valid */
    printf("a    = %s\n", a);  /* "Hello" */

    /* Sizes */
    printf("sizeof(p) = %zu\n", sizeof(p));  /* 8 — pointer */
    printf("sizeof(a) = %zu\n", sizeof(a));  /* 6 — array incl. \0 */

    /* strcmp works on both (reads only) */
    char *p2 = "hello";
    printf("p == p2: %d\n", strcmp(p, p2) == 0);  /* 1 */

    return 0;
}
```

### Q17–Q20: Multi-dimensional Arrays

```
int matrix[3][4];   // 3 rows, 4 columns

Memory layout (ROW-MAJOR — C stores row by row):

  [0][0] [0][1] [0][2] [0][3] | [1][0] [1][1] [1][2] [1][3] | [2][0]...
  ───────────────────────────────────────────────────────────────────────
  address: 0   4   8   12       16   20   24   28       32  ...

  matrix[r][c]  address = base + (r * 4 + c) * sizeof(int)
                        = base + (r * 4 + c) * 4

Type of matrix:         int (*)[4] — pointer to array of 4 ints
Type of matrix[0]:      int[4]     — array of 4 ints (decays to int*)
Type of matrix[0][0]:   int        — single int
```

```c
#include <stdio.h>

void print_matrix(int (*mat)[4], int rows) {
    /* mat is "pointer to array of 4 ints" */
    for (int r = 0; r < rows; r++) {
        for (int c = 0; c < 4; c++) {
            printf("%3d ", mat[r][c]);
        }
        printf("\n");
    }
}

int main(void) {
    int matrix[3][4] = {
        {1,  2,  3,  4},
        {5,  6,  7,  8},
        {9, 10, 11, 12}
    };

    print_matrix(matrix, 3);

    /* Pointer to first element */
    int *flat = &matrix[0][0];
    printf("element [1][2] via flat = %d\n", flat[1*4 + 2]); /* 7 */

    /* Array of pointers (different from 2D array!) */
    int row0[] = {1, 2, 3};
    int row1[] = {4, 5, 6};
    int *jagged[2] = {row0, row1};  /* rows can have different sizes */
    printf("jagged[1][2] = %d\n", jagged[1][2]); /* 6 */

    return 0;
}
```

---

## CHAPTER 4: STRINGS IN C

### What is a String in C?

A **string** in C is a null-terminated (`\0`) array of `char`.
There is NO string type — just a convention: array of chars ending with `'\0'`.

```
"hello" in memory:

Index:  [0] [1] [2] [3] [4] [5]
Value:  'h' 'e' 'l' 'l' 'o' '\0'
        104  101 108 108 111   0

strlen("hello") = 5   (counts chars, stops AT '\0', does not include it)
sizeof("hello") = 6   (includes the '\0')
```

### Q21–Q30: All String Questions

```c
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

/* ───────────────────────────────────────────────────────────
 * Q21: String literal vs character array
 * ─────────────────────────────────────────────────────────── */
void q21_demo(void) {
    /* String literal: pointer to read-only segment */
    const char *lit = "hello";   /* use const to be explicit */

    /* Character array: writable stack copy */
    char arr[] = "hello";
    arr[0] = 'H';   /* valid */
}

/* ───────────────────────────────────────────────────────────
 * Q23 & Q24: strlen vs sizeof
 * ─────────────────────────────────────────────────────────── */
void q23_q24_demo(void) {
    char arr[] = "hello";

    printf("strlen(arr)  = %zu\n", strlen(arr));   /* 5 — no \0 */
    printf("sizeof(arr)  = %zu\n", sizeof(arr));   /* 6 — includes \0 */

    const char *ptr = "hello";
    printf("strlen(ptr)  = %zu\n", strlen(ptr));   /* 5 */
    printf("sizeof(ptr)  = %zu\n", sizeof(ptr));   /* 8 — pointer size! */

    /* strlen implementation concept: */
    size_t my_strlen(const char *s) {
        const char *p = s;
        while (*p != '\0') p++;
        return (size_t)(p - s);
    }
}

/* ───────────────────────────────────────────────────────────
 * Q25–Q27: strcpy, strncpy, sprintf, snprintf
 * ─────────────────────────────────────────────────────────── */
void q25_q27_demo(void) {
    char dst[10];
    const char *src = "hello";

    /* strcpy: no bounds check — dangerous */
    strcpy(dst, src);  /* safe only if dst is large enough */
    /* strcpy(dst, "this is very long string"); */ /* buffer overflow! */

    /* strncpy: bounded copy, but has a TRAP */
    char dst2[5];
    strncpy(dst2, src, sizeof(dst2));
    /* TRAP: if src length >= n, strncpy does NOT add '\0' */
    /* dst2 might NOT be null-terminated! */
    dst2[sizeof(dst2) - 1] = '\0';   /* ALWAYS null-terminate manually */

    /* snprintf: the safe way to build strings */
    char buf[32];
    int n = snprintf(buf, sizeof(buf), "value = %d", 42);
    /* n = number of bytes that would have been written (excl. \0) */
    /* if n >= sizeof(buf), output was truncated — check for this */
    if (n >= (int)sizeof(buf)) {
        fprintf(stderr, "output truncated\n");
    }
    printf("snprintf result: %s\n", buf);

    /* sprintf: like printf to string — NO BOUNDS CHECK — dangerous */
    /* sprintf(buf, "%s", some_untrusted_string); */ /* buffer overflow risk */
    /* ALWAYS prefer snprintf */
}

/* ───────────────────────────────────────────────────────────
 * Q28: What happens if string is not null-terminated?
 * ─────────────────────────────────────────────────────────── */
void q28_demo(void) {
    char bad[5] = {'h', 'e', 'l', 'l', 'o'}; /* NO \0 */

    /* strlen(bad) → reads past end until it finds a \0 somewhere */
    /* UB: reading out of bounds memory */

    char good[6] = {'h', 'e', 'l', 'l', 'o', '\0'}; /* correct */
    printf("good: %s, len=%zu\n", good, strlen(good));
}

/* ───────────────────────────────────────────────────────────
 * Q29: Correct safe string copy
 * ─────────────────────────────────────────────────────────── */
void safe_string_copy(char *dst, size_t dst_size, const char *src) {
    if (dst == NULL || src == NULL || dst_size == 0) return;

    /* Option 1: snprintf (recommended) */
    snprintf(dst, dst_size, "%s", src);

    /* Option 2: manual — ensures null termination */
    /* dst[dst_size - 1] = '\0'; */
    /* strncpy(dst, src, dst_size - 1); */
    /* dst[dst_size - 1] = '\0'; */
}

int main(void) {
    q21_demo();
    q23_q24_demo();
    q25_q27_demo();
    q28_demo();

    char result[20];
    safe_string_copy(result, sizeof(result), "hello world");
    printf("result: %s\n", result);

    return 0;
}
```

---

## CHAPTER 5: FUNCTIONS

### Q31–Q33: Pass by Value vs Simulated Pass by Reference

```
C is STRICTLY PASS-BY-VALUE.
Every argument is COPIED when passed to a function.

Pass by value:
  foo(x)        ─→  function gets a COPY of x
                    modifying parameter does NOT change original

Simulated pass by reference:
  foo(&x)       ─→  function gets a COPY of the ADDRESS of x
                    through the pointer, function CAN modify original

                           caller stack      callee stack
    foo(int x):
    ┌──────────────┐       ┌──────────────┐
    │  int n = 5   │       │  int x = 5   │ ← COPY of n
    └──────────────┘       └──────────────┘

    foo(int *x):
    ┌──────────────────┐   ┌───────────────────────┐
    │  int n = 5       │   │  int *x = 0xADDR_of_n │ ← COPY of address
    │  @ 0xADDR_of_n   │   └───────────────────────┘
    └──────────────────┘         *x = 10 → modifies n directly
```

```c
#include <stdio.h>

/* Pass by value — CANNOT modify original */
void increment_value(int x) {
    x++;  /* modifies local copy only */
}

/* Simulated pass by reference — CAN modify original */
void increment_ref(int *x) {
    if (x == NULL) return;  /* always validate */
    (*x)++;  /* dereference and increment */
}

/* Swap: classic example */
void swap(int *a, int *b) {
    if (a == NULL || b == NULL || a == b) return;
    int tmp = *a;
    *a = *b;
    *b = tmp;
}

/* Returning multiple values via output parameters */
typedef struct {
    int quotient;
    int remainder;
} DivResult;

DivResult divide(int dividend, int divisor) {
    DivResult r;
    r.quotient  = dividend / divisor;
    r.remainder = dividend % divisor;
    return r;  /* struct copy — fine for small structs */
}

int main(void) {
    int n = 5;

    increment_value(n);
    printf("after increment_value: n = %d\n", n);  /* 5 — unchanged */

    increment_ref(&n);
    printf("after increment_ref:   n = %d\n", n);  /* 6 — changed */

    int a = 10, b = 20;
    swap(&a, &b);
    printf("after swap: a=%d b=%d\n", a, b);  /* 20, 10 */

    DivResult r = divide(17, 5);
    printf("17/5: quotient=%d remainder=%d\n", r.quotient, r.remainder);

    return 0;
}
```

### Q37–Q39: Function Pointers and Callbacks

```
Function pointers: a variable that stores the address of a function.
This allows:
  - runtime selection of algorithms (strategy pattern)
  - callbacks (pass a function to another function)
  - dispatch tables (jump tables)
  - plugin systems

Syntax (confusing but logical):
  int (*fp)(int, int);   ← fp is a pointer to a function
                            taking two ints, returning int

The parentheses around *fp are REQUIRED.
  int *fp(int, int);     ← this is a function DECLARATION
                            returning int* (different thing!)

Memory:
  The function pointer stores the address of the first instruction
  of the function in the text (code) segment.
```

```c
#include <stdio.h>
#include <stdlib.h>

/* Function pointer type */
typedef int (*CompareFn)(const void *, const void *);

/* Functions to use as callbacks */
int compare_asc(const void *a, const void *b) {
    int ia = *(const int *)a;
    int ib = *(const int *)b;
    return (ia > ib) - (ia < ib);  /* avoids subtraction overflow */
}

int compare_desc(const void *a, const void *b) {
    return compare_asc(b, a);  /* reverse arguments */
}

/* Higher-order function: takes a function pointer */
void sort_and_print(int *arr, size_t n, CompareFn cmp) {
    qsort(arr, n, sizeof(int), cmp);
    for (size_t i = 0; i < n; i++) {
        printf("%d ", arr[i]);
    }
    printf("\n");
}

/* Dispatch table — array of function pointers */
typedef int (*MathOp)(int, int);

int add(int a, int b) { return a + b; }
int sub(int a, int b) { return a - b; }
int mul(int a, int b) { return a * b; }

int main(void) {
    int data[] = {5, 2, 8, 1, 9, 3};
    size_t n = sizeof(data) / sizeof(data[0]);

    int copy[6];
    for (size_t i = 0; i < n; i++) copy[i] = data[i];

    printf("ascending:  "); sort_and_print(data, n, compare_asc);
    printf("descending: "); sort_and_print(copy, n, compare_desc);

    /* Dispatch table */
    MathOp ops[] = {add, sub, mul};
    const char *names[] = {"add", "sub", "mul"};
    int x = 10, y = 3;

    for (int i = 0; i < 3; i++) {
        printf("%s(%d, %d) = %d\n", names[i], x, y, ops[i](x, y));
    }

    return 0;
}
```

```go
// Go: first-class functions (function values)
package main

import (
    "fmt"
    "sort"
)

type CompareFn func(a, b int) bool

func sortAndPrint(data []int, less CompareFn) {
    // sort.Slice uses a function value as callback
    sort.Slice(data, func(i, j int) bool {
        return less(data[i], data[j])
    })
    fmt.Println(data)
}

func main() {
    data := []int{5, 2, 8, 1, 9, 3}

    sortAndPrint(data, func(a, b int) bool { return a < b }) // ascending
    sortAndPrint(data, func(a, b int) bool { return a > b }) // descending

    // Dispatch table with map
    ops := map[string]func(int, int) int{
        "add": func(a, b int) int { return a + b },
        "sub": func(a, b int) int { return a - b },
        "mul": func(a, b int) int { return a * b },
    }

    for name, op := range ops {
        fmt.Printf("%s(10, 3) = %d\n", name, op(10, 3))
    }
}
```

```rust
fn sort_and_print(data: &mut [i32], cmp: impl Fn(&i32, &i32) -> std::cmp::Ordering) {
    data.sort_by(cmp);
    println!("{:?}", data);
}

fn main() {
    let mut data = vec![5, 2, 8, 1, 9, 3];

    sort_and_print(&mut data, |a, b| a.cmp(b));    // ascending
    sort_and_print(&mut data, |a, b| b.cmp(a));    // descending

    // Function pointers (fn pointers, not closures)
    let ops: &[(&str, fn(i32, i32) -> i32)] = &[
        ("add", |a, b| a + b),
        ("sub", |a, b| a - b),
        ("mul", |a, b| a * b),
    ];

    for (name, op) in ops {
        println!("{}(10, 3) = {}", name, op(10, 3));
    }
}
```

---

## CHAPTER 6: STRUCTS, UNIONS, ENUMS, AND TYPEDEF

### Q41: What is a Struct?

A **struct** groups related variables of different types under one name.
Unlike an array (same type), a struct can hold ints, chars, pointers — anything.

```
struct Point {
    int x;
    int y;
};

Memory layout (assuming int = 4 bytes, no padding needed here):
┌──────────┬──────────┐
│  x (4B)  │  y (4B)  │
└──────────┴──────────┘
total: 8 bytes
```

### Q42 & Q43: Struct vs Union

```
STRUCT: all members occupy their OWN memory simultaneously.
  struct S { int a; float b; };
  size = sizeof(a) + sizeof(b) + padding = 8 bytes
  a and b are independent

UNION: all members SHARE the same memory.
  Only ONE member is valid at any given time.
  size = max(sizeof(a), sizeof(b)) = 4 bytes

  union U { int i; float f; char c; };
  Memory:
  ┌──────────────────────────────────┐
  │  [i as int] OR [f as float]     │  same 4 bytes, different interpretation
  │  OR [c as char (only 1 byte)]   │
  └──────────────────────────────────┘

When to use union:
  - Tagged unions (variant types, sum types)
  - Interpreting the same bytes differently
  - Memory-constrained embedded systems
  - Network protocol parsing
```

### Q44–Q45: Enum

```
Enum: named integer constants.
  enum Color { RED, GREEN, BLUE };
  RED=0, GREEN=1, BLUE=2 (default: sequential from 0)

  enum Color { RED=1, GREEN=5, BLUE=10 };
  Custom values — not necessarily sequential.

  enum Status { OK=0, ERR_NULL=-1, ERR_OOM=-2 };
  Can be negative. Values need not be unique.
```

### Q49–Q50: Struct Padding and Alignment

```
Alignment rule: each member must start at an address that is a
multiple of its size (or its alignment requirement).

struct BAD {
    char  a;    /* 1 byte, at offset 0 */
    /*          3 bytes of PADDING inserted by compiler */
    int   b;    /* 4 bytes, needs 4-byte alignment, at offset 4 */
    char  c;    /* 1 byte, at offset 8 */
    /*          3 bytes of PADDING at end */
};
/* sizeof(BAD) = 12 (not 6!) */

Padding visualization:
┌────┬────┬────┬────┬────┬────┬────┬────┬────┬────┬────┬────┐
│ a  │PAD │PAD │PAD │ b             │ c  │PAD │PAD │PAD │
│ 1B │    │    │    │ 4B            │ 1B │    │    │    │
└────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┘
 0    1    2    3    4    5    6    7    8    9    10   11

struct GOOD {    /* reorder members by size, largest first */
    int   b;    /* 4 bytes, at offset 0 */
    char  a;    /* 1 byte, at offset 4 */
    char  c;    /* 1 byte, at offset 5 */
    /*          2 bytes of PADDING at end */
};
/* sizeof(GOOD) = 8 (smaller than BAD) */
```

```c
#include <stdio.h>
#include <stdint.h>
#include <stddef.h>

/* Tagged union — type-safe variant */
typedef enum { INT_VAL, FLOAT_VAL, STR_VAL } ValueType;

typedef struct {
    ValueType type;
    union {
        int    i;
        float  f;
        char  *s;
    } data;
} Value;

void print_value(const Value *v) {
    if (v == NULL) return;
    switch (v->type) {
        case INT_VAL:   printf("int: %d\n",   v->data.i); break;
        case FLOAT_VAL: printf("float: %f\n", v->data.f); break;
        case STR_VAL:   printf("str: %s\n",   v->data.s); break;
    }
}

/* Packed struct (GCC extension) — no padding */
struct __attribute__((packed)) PackedHeader {
    uint8_t  version;    /* 1 byte */
    uint16_t length;     /* 2 bytes */
    uint32_t checksum;   /* 4 bytes */
};                       /* sizeof = 7, not 8 */

/* Self-referential struct (linked list node) */
typedef struct Node {
    int          value;
    struct Node *next;  /* pointer to same type */
} Node;

int main(void) {
    /* Tagged union usage */
    Value v1 = { .type = INT_VAL,   .data = {.i = 42} };
    Value v2 = { .type = FLOAT_VAL, .data = {.f = 3.14f} };
    Value v3 = { .type = STR_VAL,   .data = {.s = "hello"} };

    print_value(&v1);
    print_value(&v2);
    print_value(&v3);

    /* Alignment and padding */
    struct Padded   { char a; int b; char c; };
    struct Repacked { int b; char a; char c; };

    printf("sizeof(Padded)   = %zu\n", sizeof(struct Padded));    /* 12 */
    printf("sizeof(Repacked) = %zu\n", sizeof(struct Repacked));  /* 8 */

    /* offsetof: byte offset of member within struct */
    printf("offsetof(Padded, b) = %zu\n", offsetof(struct Padded, b));  /* 4 */

    return 0;
}
```

---

## CHAPTER 7: DYNAMIC MEMORY IN DEPTH

### Q53: What is Use-After-Free?

```
Timeline:
  T1: ptr = malloc(n)   → ptr is valid
  T2: use *ptr          → ok
  T3: free(ptr)         → ptr is dangling (but address unchanged)
  T4: other = malloc(n) → allocator MAY return same address
  T5: use *ptr          → reading/writing other's memory!
                          This is a SECURITY VULNERABILITY.

Use-after-free allows:
  - Silent data corruption (hard to debug)
  - Type confusion (ptr thinks it points to struct A, but now struct B is there)
  - Security exploits (attacker controls what malloc returns)
  - Crashes (if allocator has filled freed memory with 0xDEAD)
```

### Q54: Heap Fragmentation

```
After many malloc/free cycles:

Heap state:
┌─────┬─────────┬─────┬──────────────┬─────┬───────┐
│USED │  FREE   │USED │     FREE     │USED │ FREE  │
│ 32B │  16B    │ 64B │     128B     │ 32B │  64B  │
└─────┴─────────┴─────┴──────────────┴─────┴───────┘
Total free: 16 + 128 + 64 = 208 bytes

malloc(200):  FAILS even though 208 bytes are free!
              Because no CONTIGUOUS free block of 200 bytes exists.
              This is fragmentation.

Solutions used by allocators:
  - Coalescing: merge adjacent free blocks
  - Compaction: move allocated blocks (not possible in C without GC)
  - Slab allocators: pools of same-size objects (Linux kernel uses this)
  - Arena allocators: allocate from a large block, free all at once
```

```c
/* Arena allocator: fast, zero fragmentation, batch free */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

#define ARENA_SIZE (1024 * 1024)  /* 1 MB */
#define ALIGN_UP(x, align) (((x) + (align) - 1) & ~((align) - 1))

typedef struct Arena {
    uint8_t *base;     /* beginning of arena memory */
    size_t   used;     /* bytes used so far */
    size_t   capacity; /* total capacity */
} Arena;

Arena arena_create(void) {
    Arena a;
    a.base = malloc(ARENA_SIZE);
    a.used = 0;
    a.capacity = (a.base != NULL) ? ARENA_SIZE : 0;
    return a;
}

void *arena_alloc(Arena *a, size_t size) {
    /* Align to 8 bytes for safe access on all platforms */
    size = ALIGN_UP(size, 8);
    if (a->used + size > a->capacity) {
        return NULL;  /* out of arena space */
    }
    void *ptr = a->base + a->used;
    a->used += size;
    return ptr;
}

void arena_reset(Arena *a) {
    a->used = 0;  /* "free" everything in O(1) */
}

void arena_destroy(Arena *a) {
    free(a->base);
    a->base = NULL;
    a->used = 0;
    a->capacity = 0;
}

int main(void) {
    Arena arena = arena_create();
    if (arena.base == NULL) {
        fprintf(stderr, "arena creation failed\n");
        return 1;
    }

    /* Fast allocations — no free() needed per-object */
    int   *arr  = arena_alloc(&arena, 100 * sizeof(int));
    char  *str  = arena_alloc(&arena, 256);

    if (arr == NULL || str == NULL) {
        fprintf(stderr, "arena alloc failed\n");
        arena_destroy(&arena);
        return 1;
    }

    for (int i = 0; i < 100; i++) arr[i] = i;
    snprintf(str, 256, "hello from arena");

    printf("arr[50]=%d str=%s\n", arr[50], str);
    printf("arena used: %zu bytes\n", arena.used);

    /* Free everything at once */
    arena_reset(&arena);
    printf("after reset: %zu bytes used\n", arena.used);

    arena_destroy(&arena);
    return 0;
}
```

---

## CHAPTER 8: PREPROCESSOR AND COMPILATION PIPELINE

### Q61: What is the C Preprocessor?

The **preprocessor** runs BEFORE the compiler. It performs text substitution.
It does NOT understand C syntax — it works on raw text.

```
Source file (.c)
    │
    ▼
┌───────────────────────────────────────────────────────┐
│  PREPROCESSOR (cpp)                                   │
│  - Expands #include → paste header contents           │
│  - Expands #define → text substitution                │
│  - Processes #if/#ifdef/#endif → conditional include  │
│  - Removes comments                                   │
│  - Produces: translation unit (expanded .c)           │
└───────────────────────────────────────────────────────┘
    │
    ▼
┌───────────────────────────────────────────────────────┐
│  COMPILER (cc1)                                       │
│  - Lexing: text → tokens                              │
│  - Parsing: tokens → AST (Abstract Syntax Tree)       │
│  - Semantic analysis: type checking                   │
│  - IR generation: AST → intermediate representation   │
│  - Optimization: constant folding, inlining, etc.     │
│  - Code generation: IR → assembly (.s)                │
└───────────────────────────────────────────────────────┘
    │
    ▼
┌───────────────────────────────────────────────────────┐
│  ASSEMBLER (as)                                       │
│  - Assembly (.s) → object code (.o)                   │
│  - Translates mnemonics to machine bytes              │
│  - Produces: relocatable object file                  │
└───────────────────────────────────────────────────────┘
    │
    ▼
┌───────────────────────────────────────────────────────┐
│  LINKER (ld)                                          │
│  - Combines multiple .o files                         │
│  - Resolves external symbol references                │
│  - Links with standard library (.a / .so)             │
│  - Produces: executable binary                        │
└───────────────────────────────────────────────────────┘
    │
    ▼
Executable (a.out / program)
```

### Q63–Q66: Header Guards and Macros

```c
/* mylib.h — header with guard */
#ifndef MYLIB_H   /* if not already defined */
#define MYLIB_H   /* define it (so second include is skipped) */

/* Contents of header */
typedef struct {
    int x;
    int y;
} Point;

int add(int a, int b);

#endif /* MYLIB_H */
```

```c
/* Macro dangers: always parenthesize arguments */
#include <stdio.h>

/* DANGEROUS macro */
#define SQUARE_WRONG(x)   x * x

/* SAFE macro */
#define SQUARE(x)         ((x) * (x))

/* MAX macro — evaluates argument TWICE (side effect danger) */
#define MAX(a, b)         ((a) > (b) ? (a) : (b))

/* Safe MAX using GCC statement expression (extension) */
#define SAFE_MAX(a, b)    ({                     \
    __typeof__(a) _a = (a);                      \
    __typeof__(b) _b = (b);                      \
    _a > _b ? _a : _b;                           \
})

/* Stringification */
#define STRINGIFY(x)  #x
#define TO_STRING(x)  STRINGIFY(x)

/* Token pasting */
#define CONCAT(a, b)  a##b

int main(void) {
    int x = 3;

    /* Dangerous: SQUARE_WRONG(x+1) expands to x+1*x+1 = x + x + 1 = 7, not 16 */
    printf("SQUARE_WRONG(x+1) = %d\n", SQUARE_WRONG(x+1));  /* 7 */
    printf("SQUARE(x+1)       = %d\n", SQUARE(x+1));         /* 16 */

    /* Side effect danger: MAX(i++, j++) evaluates i++ or j++ TWICE */
    int a = 5, b = 3;
    int m = MAX(a++, b++);      /* a++ evaluated twice if a>b! */
    printf("MAX result: %d, a=%d, b=%d\n", m, a, b);

    /* Stringify */
    printf("PI constant name: %s\n", STRINGIFY(PI));
    printf("Line: %s\n", TO_STRING(__LINE__));

    /* Token paste */
    int xy = 42;        /* variable named "xy" */
    printf("CONCAT(x,y) = %d\n", CONCAT(x, y));  /* expands to xy = 42 */

    return 0;
}
```

### Q70: extern vs static linkage

```
LINKAGE determines the VISIBILITY of a symbol across translation units.

extern:
  - External linkage — visible to OTHER .c files after linking
  - Default for functions and global variables
  - Declare with extern keyword in headers to share

static (at file scope):
  - Internal linkage — visible ONLY within the current .c file
  - Used to hide implementation details
  - Prevents naming conflicts between .c files

static (at function scope):
  - Local static — variable persists across calls (in DATA segment)
  - Initialized only once (on first call)
  - Like a global, but only accessible within that function
```

```c
/* counter.h */
#ifndef COUNTER_H
#define COUNTER_H

/* extern declaration — defined in counter.c */
extern int global_shared;

int counter_get(void);
void counter_increment(void);

#endif

/* counter.c */
#include "counter.h"

int global_shared = 100;     /* external linkage — visible everywhere */
static int private_state = 0; /* internal linkage — only counter.c */

int counter_get(void) {
    return private_state;
}

void counter_increment(void) {
    private_state++;

    /* Local static: persists across calls */
    static int call_count = 0;
    call_count++;
    /* call_count is incremented every call, survives between calls */
}

/* main.c */
#include <stdio.h>
#include "counter.h"

int main(void) {
    counter_increment();
    counter_increment();
    counter_increment();
    printf("counter = %d\n", counter_get());       /* 3 */
    printf("shared  = %d\n", global_shared);       /* 100 */
    /* private_state not accessible here — static internal linkage */
    return 0;
}
```

---

## CHAPTER 9: TYPE SYSTEM, QUALIFIERS, AND CASTS

### Q74: const int *p vs int *const p

```
Four combinations of const with pointers:

  int       *p;          pointer to int — both mutable
  const int *p;          pointer to CONST int — data immutable, ptr mutable
  int *const p;          CONST pointer to int — data mutable, ptr immutable
  const int *const p;    CONST pointer to CONST int — both immutable

Memory analogy:
  const int *p:
    p ───▶ [42]   p can point elsewhere, but cannot write *p = 10

  int *const p:
    p ───▶ [42]   p always points here, but *p = 10 is allowed

  const int *const p:
    p ───▶ [42]   nothing changes — maximally restrictive

Rule of thumb: read right-to-left
  "const int *p" → p is a pointer (*) to a const int
  "int *const p" → p is a const pointer (*const) to an int
```

```c
#include <stdio.h>

void read_only(const int *data, size_t n) {
    /* data[i] = 0; */  /* compile error: assignment of read-only location */
    for (size_t i = 0; i < n; i++) {
        printf("%d ", data[i]);  /* reading is ok */
    }
    printf("\n");
}

int main(void) {
    int arr[] = {1, 2, 3, 4, 5};

    const int *p1 = arr;
    /* *p1 = 10; */     /* error: cannot modify through p1 */
    p1 = arr + 1;       /* ok: p1 can move */

    int *const p2 = arr;
    *p2 = 10;            /* ok: can modify data */
    /* p2 = arr + 1; */ /* error: p2 cannot be reassigned */

    read_only(arr, 5);  /* safe: const promises not to modify */

    return 0;
}
```

### Q75–Q76: volatile

```
volatile tells the compiler:
  "Do NOT optimize accesses to this variable.
   Read from memory EVERY TIME. Write to memory EVERY TIME."

When to use:
  - Hardware memory-mapped registers (embedded systems)
  - Variables modified by signal handlers
  - Variables shared with interrupt service routines
  - Variables in spin-locks (before proper atomics)

Without volatile:
  int flag = 0;
  while (flag == 0) {}  // compiler may optimize to: while (true) {}
  // because it sees: flag never changes in this function!
  // So it caches flag in a register and never re-reads memory.

With volatile:
  volatile int flag = 0;
  while (flag == 0) {}  // compiler MUST re-read flag each iteration
  // because volatile tells it: "someone else may change this"
```

```c
#include <stdio.h>
#include <signal.h>

/* Signal handler sets this flag */
volatile sig_atomic_t stop_flag = 0;

void signal_handler(int sig) {
    (void)sig;
    stop_flag = 1;  /* volatile ensures main() sees this change */
}

int main(void) {
    signal(SIGINT, signal_handler);

    printf("Running... press Ctrl+C to stop\n");

    long count = 0;
    while (!stop_flag) {  /* volatile: re-read stop_flag each iteration */
        count++;
    }

    printf("Stopped after %ld iterations\n", count);
    return 0;
}
```

### Q73: Integer overflow — signed vs unsigned

```
SIGNED INTEGER OVERFLOW:
  int x = INT_MAX;   /* 2147483647 */
  x + 1              /* UNDEFINED BEHAVIOR in C */
  Compiler assumes it never happens and may:
    - Eliminate overflow checks
    - Produce wrong code
    - Create security vulnerabilities

UNSIGNED INTEGER OVERFLOW:
  unsigned int x = UINT_MAX;  /* 4294967295 */
  x + 1                       /* DEFINED: wraps to 0 */
  This is guaranteed modular arithmetic.

Type promotion rules (implicit conversions):
  int + int             → int
  int + unsigned int    → unsigned int (int is converted!)
  int + long            → long
  short + int           → int (short promoted to int)

DANGER:
  int a = -1;
  unsigned int b = 1;
  if (a < b) → FALSE! because -1 is converted to UINT_MAX (huge number)
```

```c
#include <stdio.h>
#include <limits.h>
#include <stdint.h>

int main(void) {
    /* Signed overflow — UB, result unpredictable */
    int s = INT_MAX;
    /* Detecting potential overflow BEFORE it happens: */
    if (s > INT_MAX - 1) {
        printf("would overflow\n");
    } else {
        s += 1;
        printf("s = %d\n", s);
    }

    /* Unsigned overflow — defined, wraps */
    unsigned int u = UINT_MAX;
    printf("UINT_MAX + 1 = %u\n", u + 1);  /* 0 */

    /* Signed/unsigned comparison trap */
    int a = -1;
    unsigned int b = 1;
    if ((unsigned int)a < b) {
        /* NOT reached: -1 as unsigned is UINT_MAX */
        printf("a < b\n");
    } else {
        printf("unsigned -1 = %u > %u\n", (unsigned int)a, b);
    }

    /* Fixed-width types for portability */
    int32_t x = INT32_MAX;   /* always 32 bits */
    int64_t y = INT64_MAX;   /* always 64 bits */
    printf("int32 max = %d, int64 max = %lld\n", x, (long long)y);

    return 0;
}
```

---

## CHAPTER 10: BITS, BYTES, AND LOW-LEVEL OPERATIONS

### Q81–Q83: Bitwise Operations

```
BITWISE OPERATIONS work bit-by-bit on integer representations.

AND  (&):  1 if BOTH bits are 1
  0b1010 & 0b1100 = 0b1000  (8)
  Use: masking, checking specific bits

OR   (|):  1 if EITHER bit is 1
  0b1010 | 0b0101 = 0b1111  (15)
  Use: setting bits

XOR  (^):  1 if bits are DIFFERENT
  0b1010 ^ 0b1100 = 0b0110  (6)
  Use: toggling bits, swap without temp, detecting differences

NOT  (~):  flip all bits (bitwise complement)
  ~0b1010 = 0b0101  (on 4-bit)
  Actually: ~x = -(x+1) for signed integers

LEFT SHIFT  (<<):  shift bits left, fill with 0 on right
  0b0001 << 3 = 0b1000  (multiply by 2^3 = 8)
  Signed: UB if shifting into sign bit

RIGHT SHIFT (>>):  shift bits right
  LOGICAL: fill with 0 (for unsigned)
  ARITHMETIC: fill with sign bit (for signed, implementation-defined in C)
  0b1000 >> 3 = 0b0001  (divide by 2^3)
```

```c
#include <stdio.h>
#include <stdint.h>

/* Practical bit manipulation patterns */

/* Check if bit N is set */
static inline int bit_check(uint32_t val, int n) {
    return (val >> n) & 1;
}

/* Set bit N */
static inline uint32_t bit_set(uint32_t val, int n) {
    return val | (1u << n);
}

/* Clear bit N */
static inline uint32_t bit_clear(uint32_t val, int n) {
    return val & ~(1u << n);
}

/* Toggle bit N */
static inline uint32_t bit_toggle(uint32_t val, int n) {
    return val ^ (1u << n);
}

/* Count set bits (popcount) */
static inline int popcount(uint32_t val) {
    /* Brian Kernighan's algorithm */
    int count = 0;
    while (val) {
        val &= val - 1;  /* clear lowest set bit */
        count++;
    }
    return count;
}

/* Swap two integers without temporary variable */
static inline void xor_swap(int *a, int *b) {
    /* ONLY works when a != b (different addresses) */
    if (a == b) return;
    *a ^= *b;
    *b ^= *a;
    *a ^= *b;
}

/* Check if n is a power of 2 */
static inline int is_power_of_two(uint32_t n) {
    return n > 0 && (n & (n - 1)) == 0;
}

int main(void) {
    uint32_t flags = 0b00001010;  /* bits 1 and 3 set */

    printf("bit 1 set: %d\n", bit_check(flags, 1));   /* 1 */
    printf("bit 2 set: %d\n", bit_check(flags, 2));   /* 0 */

    flags = bit_set(flags, 2);
    printf("after set bit 2:    0x%02X\n", flags);    /* 0x0E */

    flags = bit_clear(flags, 1);
    printf("after clear bit 1:  0x%02X\n", flags);    /* 0x0C */

    flags = bit_toggle(flags, 0);
    printf("after toggle bit 0: 0x%02X\n", flags);    /* 0x0D */

    printf("popcount(0xFF) = %d\n", popcount(0xFF));   /* 8 */

    int x = 10, y = 20;
    xor_swap(&x, &y);
    printf("after swap: x=%d y=%d\n", x, y);

    for (uint32_t i = 0; i <= 16; i++) {
        if (is_power_of_two(i)) printf("%u is a power of 2\n", i);
    }

    return 0;
}
```

### Q84–Q85: Endianness

```
ENDIANNESS: the byte order in which multi-byte integers are stored.

Value: 0x12345678 (32-bit integer)

LITTLE-ENDIAN (x86, ARM, most modern CPUs):
  Low address → High address
  ┌────┬────┬────┬────┐
  │ 78 │ 56 │ 34 │ 12 │   ← least significant byte FIRST
  └────┴────┴────┴────┘
  addr: 0   1   2   3

BIG-ENDIAN (network byte order, MIPS, SPARC, TCP/IP):
  Low address → High address
  ┌────┬────┬────┬────┐
  │ 12 │ 34 │ 56 │ 78 │   ← most significant byte FIRST
  └────┴────┴────┴────┘
  addr: 0   1   2   3

Why it matters:
  - Network protocols use big-endian (htonl/ntohl)
  - Binary file formats may specify endianness
  - Cross-platform binary data exchange
  - Reading hardware registers in embedded systems
```

```c
#include <stdio.h>
#include <stdint.h>

/* Detect endianness at runtime */
int is_little_endian(void) {
    uint32_t val = 1;
    /* Cast to byte pointer and check first byte */
    return *(uint8_t *)&val == 1;
    /* On little-endian: first byte = 0x01 (LSB)
     * On big-endian:    first byte = 0x00 (MSB) */
}

/* Byte-swap a 32-bit integer (convert between endianness) */
uint32_t bswap32(uint32_t x) {
    return ((x & 0xFF000000u) >> 24) |
           ((x & 0x00FF0000u) >>  8) |
           ((x & 0x0000FF00u) <<  8) |
           ((x & 0x000000FFu) << 24);
}

int main(void) {
    printf("System is %s-endian\n",
           is_little_endian() ? "little" : "big");

    uint32_t val = 0x12345678;
    printf("Original:  0x%08X\n", val);
    printf("Swapped:   0x%08X\n", bswap32(val));

    /* View bytes directly */
    uint8_t *bytes = (uint8_t *)&val;
    printf("Bytes in memory: ");
    for (int i = 0; i < 4; i++) {
        printf("0x%02X ", bytes[i]);
    }
    printf("\n");

    return 0;
}
```

### Q87: Pointer size on 32-bit vs 64-bit

```
Pointer size = size of virtual address space

32-bit system:
  Virtual address: 32 bits
  sizeof(void *) = 4 bytes
  Max addressable memory: 2^32 = 4 GB

64-bit system:
  Virtual address: 64 bits (actually 48 bits used on x86-64)
  sizeof(void *) = 8 bytes
  Max addressable memory: 2^48 = 256 TB (theoretical)

  sizeof(int)    = 4  (same on both)
  sizeof(long)   = 4 (Windows 64) or 8 (Linux/macOS 64)
  sizeof(void *) = 8 (all modern 64-bit systems)

Use size_t for sizes, uintptr_t for pointer-as-integer:
  size_t:    unsigned, same size as pointer
  ptrdiff_t: signed, for pointer differences
  uintptr_t: unsigned integer, can hold any pointer
```

### Q88–Q89: Alignment and Misalignment

```
Alignment: data type must be stored at address that is a multiple
of its alignment requirement.

On x86-64:
  char:    alignment 1 (any address)
  short:   alignment 2 (even addresses)
  int:     alignment 4 (addresses divisible by 4)
  long:    alignment 8 (addresses divisible by 8)
  double:  alignment 8
  void *:  alignment 8

Misaligned access:
  int *p = (int *)(some_odd_address);
  *p = 42;   // may cause:
              // x86: works but slower (hardware fixes it)
              // ARM: BUS ERROR / SIGBUS crash
              // SPARC/MIPS: BUS ERROR always

Why alignment matters:
  - CPU fetches aligned data in ONE bus cycle
  - Misaligned data may require TWO bus cycles
  - Some CPUs prohibit misaligned access entirely
  - Vectorized SIMD instructions require 16/32-byte alignment
```

---

## CHAPTER 11: OUTPUT PREDICTION AND UNDEFINED BEHAVIOR TRAPS

### Q91–Q98: Classic C Output Puzzles

```c
#include <stdio.h>
#include <limits.h>

void q91_q92(void) {
    /* Q91: printf("%d", 'A') */
    /* 'A' is an int with value 65 in ASCII */
    printf("%d\n", 'A');   /* prints: 65 */

    /* Q92: printf("%c", 65) */
    /* 65 as ASCII is 'A' */
    printf("%c\n", 65);    /* prints: A */
}

void q93_format_mismatch(void) {
    /* Q93: Format specifier mismatch — UB */
    /* int with %f: UB — reads wrong bytes from argument stack */
    int x = 42;
    /* printf("%f\n", x);  */ /* UB — float format, int argument */

    /* Pointer with %d: UB — pointer is 8 bytes, %d reads 4 */
    int *p = &x;
    /* printf("%d\n", p);  */ /* UB on 64-bit */
    printf("%p\n", (void *)p);  /* correct: %p for pointers */
}

void q94_uninitialized(void) {
    /* Q94: Uninitialized local variable */
    int x;     /* x has indeterminate value */
    /* printf("%d\n", x);  */ /* UB: may print anything, or crash */

    /* Always initialize */
    int y = 0;
    printf("y = %d\n", y);  /* 0 */
}

void q95_oob(void) {
    /* Q95: Array out of bounds */
    int arr[5] = {1, 2, 3, 4, 5};
    /* printf("%d\n", arr[10]);  */ /* UB: out of bounds */
    /* printf("%d\n", arr[-1]);  */ /* UB: before array */

    /* On real hardware this may read adjacent stack memory */
    /* leading to wrong values or SIGSEGV */
}

void q96_q97_sequence(void) {
    /* Q96: i++ + ++i — UB in C (not in C++) */
    int i = 5;
    /* int x = i++ + ++i;  */ /* UB: modifies i twice without sequence point */

    /* SAFE equivalent: separate statements */
    int a = i;   /* a = 5 */
    i++;         /* i = 6 */
    int b = ++i; /* i = 7, b = 7 */
    printf("a + b = %d\n", a + b);  /* 12 */
}

void q98_sequence_points(void) {
    /* Sequence points in C:
     * - After a full expression statement (;)
     * - After left operand of &&, ||, ?:
     * - After comma operator
     * - At function call (all args evaluated before call)
     *
     * Between sequence points: only ONE modification per object is allowed.
     */

    int i = 0;
    i = i++;    /* UB: modifies i, reads i — not between sequence points */
    /* Result: UB (may be 0 or 1 or anything) */

    /* SAFE: separate statements */
    i = 0;
    i++;        /* i is now 1, well-defined */
    printf("i = %d\n", i);
}

void q100_format_injection(void) {
    /* Q100: printf(msg) is dangerous if msg is user-controlled */
    /* Format string injection (real security vulnerability) */

    const char *safe = "hello %s %d";  /* user input could contain %s, %d, %n */

    /* DANGEROUS: */
    /* printf(safe);  */ /* would interpret %s, %d — reads random stack memory */
                         /* %n WRITES to memory — information disclosure / RCE */

    /* SAFE: */
    printf("%s\n", safe);  /* always use a format string literal */
}

int main(void) {
    q91_q92();
    q93_format_mismatch();
    q94_uninitialized();
    q95_oob();
    q96_q97_sequence();
    q98_sequence_points();
    q100_format_injection();
    return 0;
}
```

---
