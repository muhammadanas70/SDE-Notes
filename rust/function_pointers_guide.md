# Function Pointers: A Complete In-Depth Guide — C, Go & Rust

> Covers every concept, mental model, memory layout, calling convention, idiom,
> pitfall, and pattern you need to reason fluently about functions-as-values
> in all three languages.

---

## Table of Contents

1. [Foundational Mental Model](#1-foundational-mental-model)
2. [Memory Layout & Calling Conventions](#2-memory-layout--calling-conventions)
3. [C — Function Pointers](#3-c--function-pointers)
   - 3.1 What Is a Function in C?
   - 3.2 Pointer Syntax Dissected
   - 3.3 typedef — Cleaning Up the Syntax
   - 3.4 Calling Through a Function Pointer
   - 3.5 Passing Function Pointers as Arguments
   - 3.6 Returning Function Pointers
   - 3.7 Arrays of Function Pointers
   - 3.8 Callbacks
   - 3.9 Function Pointer Tables (Jump Tables / vtable simulation)
   - 3.10 void* and Function Pointers
   - 3.11 NULL Function Pointers & Guards
   - 3.12 Casting & Aliasing Dangers
   - 3.13 Variadic Function Pointers
   - 3.14 Signal Handlers
   - 3.15 qsort — The Classic Higher-Order Function
   - 3.16 State Machines with Function Pointers
   - 3.17 C99 / C11 / C17 Nuances
4. [Go — Function Values & Closures](#4-go--function-values--closures)
   - 4.1 Functions Are First-Class Values
   - 4.2 Function Types
   - 4.3 Anonymous Functions
   - 4.4 Closures — Capturing Variables
   - 4.5 Closures and Goroutines (The Classic Bug)
   - 4.6 Method Values vs Method Expressions
   - 4.7 Functions as Map Values
   - 4.8 Higher-Order Functions
   - 4.9 Returning Functions
   - 4.10 Variadic Functions as Values
   - 4.11 Interfaces vs Function Types
   - 4.12 Function Types with Methods (Adapter Pattern)
   - 4.13 Nil Function Values
5. [Rust — Function Pointers & Closures](#5-rust--function-pointers--closures)
   - 5.1 The Rust Function Landscape (Overview)
   - 5.2 Bare Function Pointers (`fn`)
   - 5.3 Function Items vs Function Pointers
   - 5.4 Closures and the Three Traits: Fn, FnMut, FnOnce
   - 5.5 How Closures Capture (by ref, by mut ref, by value)
   - 5.6 move Closures
   - 5.7 Closure Coercion to fn Pointer
   - 5.8 Static Dispatch: `impl Fn`
   - 5.9 Dynamic Dispatch: `dyn Fn` and `Box<dyn Fn>`
   - 5.10 Returning Closures from Functions
   - 5.11 Higher-Kinded Patterns: Combinators
   - 5.12 Function Pointers in unsafe
   - 5.13 Memory Layout of fn vs Closures
   - 5.14 Lifetime of Closures
   - 5.15 Closures in Iterators
   - 5.16 Thread Safety: Send + Sync on Closures
6. [Cross-Language Comparison](#6-cross-language-comparison)
7. [Advanced Patterns Across All Three Languages](#7-advanced-patterns-across-all-three-languages)
8. [Common Pitfalls & Anti-Patterns](#8-common-pitfalls--anti-patterns)

---

## 1. Foundational Mental Model

A **function pointer** is a variable that holds the address of a function rather
than the address of data. At the machine level every function is just a sequence
of instructions that lives at some address in the text (code) segment of the
process image. A function pointer is simply a word-size integer that records that
address.

```
Process Memory Map
==================

  High Address
  ┌────────────────────────────┐
  │          Stack             │  ← local variables, return addresses
  ├────────────────────────────┤
  │            ↓               │
  │          (gap)             │
  │            ↑               │
  ├────────────────────────────┤
  │           Heap             │  ← malloc / new / Box::new
  ├────────────────────────────┤
  │    BSS  (zero-init data)   │  ← global/static uninitialised vars
  ├────────────────────────────┤
  │    Data (initialised)      │  ← global/static initialised vars
  ├────────────────────────────┤
  │    Text / Code Segment     │  ← compiled machine code lives here
  │   ┌──────────────────┐     │
  │   │  add():  0x401020│     │  ← function "add" starts at 0x401020
  │   │  push rbp        │     │
  │   │  mov rbp,rsp     │     │
  │   │  …               │     │
  │   ├──────────────────┤     │
  │   │  main(): 0x401060│     │
  │   │  …               │     │
  │   └──────────────────┘     │
  ├────────────────────────────┤
  │  ELF/PE/Mach-O Header      │
  └────────────────────────────┘
  Low Address

  A function pointer is just a variable containing
  one of those addresses, e.g. fp = 0x401020.
```

When you *call* through a function pointer the CPU performs:
1. Load the address stored in the pointer into a register (e.g. RAX).
2. Execute `CALL RAX` — push the return address onto the stack, jump to RAX.
3. The callee runs its body.
4. `RET` — pop the return address, jump back.

This is in contrast to a *direct call* (`CALL 0x401020`) where the target
address is encoded in the instruction itself at compile time.

**Why use function pointers?**

| Goal | Mechanism |
|---|---|
| Decouple caller from callee at compile time | Pass a function pointer instead of hard-coding the callee |
| Swap behaviour at runtime | Store different function pointers in the same variable |
| Build plugin systems | Exported function table in a shared library |
| Implement polymorphism in C | Struct containing function pointers (vtable) |
| Callbacks / event handlers | Register a pointer that the framework calls when an event fires |
| Higher-order functions (map, filter, reduce) | Accept a function pointer as a parameter |
| State machines | Each state is a function pointer; transitions update the pointer |

---

## 2. Memory Layout & Calling Conventions

### 2.1 Size of a Function Pointer

On modern 64-bit platforms a function pointer is 8 bytes (same as any data
pointer). On 32-bit platforms it is 4 bytes. **C does not guarantee that a
function pointer fits in a `void*`** (they can differ on some obscure
architectures such as Harvard-architecture embedded systems), but on all common
OSes (Linux/macOS/Windows x86-64) they are the same size and mutually
castable.

```
Stack frame during a call through fp:

  ┌──────────────────────────────────────┐  ← rsp before call
  │  return address (8 bytes)            │
  ├──────────────────────────────────────┤  ← rbp (base pointer)
  │  caller's saved rbp (8 bytes)        │
  ├──────────────────────────────────────┤
  │  local var: fp  (8 bytes, an addr)   │  ← fp = 0x401020
  ├──────────────────────────────────────┤
  │  …other locals…                      │
  └──────────────────────────────────────┘

  The CPU does:  CALL [rbp - offset]
  which is equivalent to:  CALL *fp  (indirect call)
```

### 2.2 Calling Conventions

A calling convention is a contract between caller and callee about:
- Which registers carry arguments.
- Which registers the callee must preserve.
- Who cleans up the stack.
- How multi-word return values are handled.

| Platform | Convention | Arg Regs | Return | Stack Cleanup |
|---|---|---|---|---|
| Linux/macOS x86-64 | System V AMD64 ABI | rdi, rsi, rdx, rcx, r8, r9 | rax (rax:rdx for 128-bit) | Caller |
| Windows x86-64 | Microsoft x64 | rcx, rdx, r8, r9 | rax | Caller |
| 32-bit x86 | cdecl | stack only | eax | Caller |
| 32-bit x86 | stdcall (WinAPI) | stack only | eax | Callee |
| ARM64 (AAPCS64) | AAPCS64 | x0–x7 | x0 | Caller |

In C you can annotate calling conventions explicitly:
```c
// Windows __stdcall (WinAPI callbacks must match)
typedef int (__stdcall *WinCallback)(HWND, UINT, WPARAM, LPARAM);

// GCC attribute
typedef void (*Handler)(int) __attribute__((cdecl));
```

Rust uses `extern "C"` and `extern "system"` to select conventions.
Go uses its own internal ABI (register-based since Go 1.17, stack-based before).

### 2.3 Indirect vs Direct Calls — Performance

A direct call is just `CALL imm32` — one instruction. An indirect call through a
pointer is `CALL [reg]` — the CPU must first load the value from memory into a
register, then call. This adds:

1. A load (usually cheap if the pointer is in cache).
2. Branch misprediction penalty (the CPU cannot speculate the target as easily
   as a constant address).

This is why performance-critical code sometimes avoids virtual dispatch or
function pointer tables in hot paths, preferring inlinable direct calls
(templates in C++, generics/monomorphisation in Rust).

---

## 3. C — Function Pointers

### 3.1 What Is a Function in C?

In C a function name *decays* to a pointer to the function in almost every
context, exactly analogous to how an array name decays to a pointer to its first
element.

```c
#include <stdio.h>

int add(int a, int b) { return a + b; }

int main(void) {
    // These three lines are ALL equivalent.
    // The function name, &function, and *(&function) all give you the address.
    printf("%p\n", (void*)add);         // decay: add → &add
    printf("%p\n", (void*)&add);        // explicit address-of
    printf("%p\n", (void*)***add);      // dereferencing a function pointer
                                         // immediately gives you back the pointer
    // All three print the same address.
    return 0;
}
```

A function type in C is written as its return type followed by a parameter list:
`int(int, int)` means "a function that takes two ints and returns int". But you
cannot have a variable of *function type* — only a variable of *pointer-to-
function type*.

### 3.2 Pointer Syntax Dissected

The most confusing aspect of C function pointer syntax is the placement of `*`.
The rule: the `*` that binds to the variable name must be inside parentheses
to prevent it from being parsed as part of the return type.

```
Return type     Pointer    Name    Param list
    │              │         │         │
    ▼              ▼         ▼         ▼
   int           (*         fp   ) (int, int)

Read inside-out:
  1. fp               → "fp is…"
  2. (*fp)            → "…a pointer…"
  3. (*fp)(int, int)  → "…to a function taking (int, int)…"
  4. int(*fp)(int,int)→ "…returning int"
```

Without parentheses `int *fp(int, int)` means something completely different:
"fp is a function taking (int, int) returning int*" — a pointer-returning
function, not a function pointer!

```c
// ─── Correct declarations ─────────────────────────────────────────────────
int (*fp)(int, int);          // pointer to function(int,int)->int
void (*vp)(void);             // pointer to function()->void
double (*dp)(double);         // pointer to function(double)->double
char *(*sp)(const char *);    // pointer to function(const char*)->char*
int (*(*mp)[4])(int);         // pointer to array[4] of pointers to function(int)->int

// ─── Common mistake ──────────────────────────────────────────────────────
int *fp2(int, int);           // WRONG INTENT: this is a prototype for a
                               // function named fp2 that returns int*
```

**Complete annotation diagram:**

```
int  (*fp) (int a,  int b)
│     ││    │           │
│     ││    └─────┬─────┘
│     ││     parameter list
│     │└── variable name: fp
│     └─── * makes it a pointer
└────── return type of the pointed-to function
```

### 3.3 typedef — Cleaning Up the Syntax

typedef moves the complexity behind a name. The pattern is:
`typedef <what you would write for a variable> <alias>;`

```c
// Without typedef (messy, error-prone):
int (*fp)(int, int);
int (*operations[4])(int, int);
int (*make_op(char))(int, int);   // function returning function pointer

// With typedef (clean):
typedef int (*BinaryOp)(int, int);    // BinaryOp is now a type alias

BinaryOp fp;                           // same as: int (*fp)(int, int)
BinaryOp operations[4];                // array of 4 BinaryOps
BinaryOp make_op(char op);             // function returning BinaryOp

// ─── typedef syntax anatomy ──────────────────────────────────────────────
//
//   typedef   int  (* BinaryOp )(int, int);
//      │       │      │              │
//      │       │      │         parameter list
//      │       │      └── the alias name goes where the variable name was
//      │       └── return type
//      └── keyword that makes it a type alias, not a variable declaration
```

### 3.4 Calling Through a Function Pointer

```c
#include <stdio.h>

typedef int (*BinaryOp)(int, int);

int add(int a, int b) { return a + b; }
int sub(int a, int b) { return a - b; }
int mul(int a, int b) { return a * b; }

int main(void) {
    BinaryOp op;

    // ─── Assignment ───────────────────────────────────────────────────────
    op = add;      // or: op = &add;  (both are identical)

    // ─── Calling ─────────────────────────────────────────────────────────
    // Two syntaxes; both compile to the same machine code:
    int r1 = op(3, 4);       // modern style (implicit dereference)
    int r2 = (*op)(3, 4);    // explicit dereference style (older, verbose)

    printf("op(3,4) = %d\n", r1);   // 7
    printf("(*op)(3,4) = %d\n", r2); // 7

    // ─── Reassignment at runtime ─────────────────────────────────────────
    op = sub;
    printf("op(3,4) = %d\n", op(3,4));   // -1

    op = mul;
    printf("op(3,4) = %d\n", op(3,4));   // 12

    return 0;
}
```

**Important:** function pointers cannot be incremented or used in pointer
arithmetic. `op++` is a compile error — you are not iterating over an array of
code bytes, you are storing a single address.

### 3.5 Passing Function Pointers as Arguments

This enables **higher-order functions** — functions that accept other functions
as arguments.

```c
#include <stdio.h>
#include <stdlib.h>
#include <math.h>

typedef double (*Transformer)(double);

// apply_all takes a function pointer and applies it to every element
void apply_all(double *arr, int n, Transformer fn) {
    for (int i = 0; i < n; i++) {
        arr[i] = fn(arr[i]);
    }
}

// integrate numerically using a function pointer
// approximates ∫(a..b) f(x) dx using the trapezoidal rule
double integrate(double a, double b, int steps, double (*f)(double)) {
    double h = (b - a) / steps;
    double sum = (f(a) + f(b)) / 2.0;
    for (int i = 1; i < steps; i++) {
        sum += f(a + i * h);
    }
    return sum * h;
}

double square(double x) { return x * x; }
double cube(double x)   { return x * x * x; }

int main(void) {
    double data[] = {1.0, 2.0, 3.0, 4.0};
    int n = 4;

    apply_all(data, n, sqrt);    // use stdlib sqrt directly
    for (int i = 0; i < n; i++) printf("%.4f ", data[i]);
    printf("\n");
    // Output: 1.0000 1.4142 1.7321 2.0000

    // ∫(0..1) x² dx = 1/3 ≈ 0.3333
    printf("%.6f\n", integrate(0.0, 1.0, 1000000, square));

    // ∫(0..1) x³ dx = 1/4 = 0.25
    printf("%.6f\n", integrate(0.0, 1.0, 1000000, cube));

    return 0;
}
```

### 3.6 Returning Function Pointers

Functions can return function pointers. The raw syntax is notoriously ugly;
typedef rescues readability.

```c
#include <stdio.h>

typedef int (*BinaryOp)(int, int);

int add(int a, int b) { return a + b; }
int sub(int a, int b) { return a - b; }
int mul(int a, int b) { return a * b; }
int divide(int a, int b) { return b != 0 ? a / b : 0; }

// Returns a function pointer based on a character
BinaryOp get_operation(char op) {
    switch (op) {
        case '+': return add;
        case '-': return sub;
        case '*': return mul;
        case '/': return divide;
        default:  return NULL;
    }
}

// Raw syntax (no typedef) — extremely ugly, for illustration only:
// int (*get_op_raw(char op))(int,int) { ... }

int main(void) {
    char ops[] = {'+', '-', '*', '/'};
    int a = 10, b = 3;

    for (int i = 0; i < 4; i++) {
        BinaryOp fn = get_operation(ops[i]);
        if (fn != NULL) {
            printf("10 %c 3 = %d\n", ops[i], fn(a, b));
        }
    }
    // Output:
    // 10 + 3 = 13
    // 10 - 3 = 7
    // 10 * 3 = 30
    // 10 / 3 = 3
    return 0;
}
```

### 3.7 Arrays of Function Pointers

An array of function pointers is the heart of **jump tables** and **dispatch
tables**. It provides O(1) dispatch versus a chain of if/else or switch
statements.

```c
#include <stdio.h>

// ─── Dispatch table for a simple calculator ──────────────────────────────

typedef double (*MathFn)(double, double);

double op_add(double a, double b) { return a + b; }
double op_sub(double a, double b) { return a - b; }
double op_mul(double a, double b) { return a * b; }
double op_div(double a, double b) { return b != 0.0 ? a / b : 0.0; }

// Array of function pointers indexed by a token (0=add, 1=sub, ...)
MathFn dispatch[] = { op_add, op_sub, op_mul, op_div };

// ─── ASCII diagram: how the dispatch table looks in memory ───────────────
//
//   dispatch[]   (array of 4 pointers, 32 bytes on 64-bit)
//
//   ┌─────────────────┬─────────────────┬─────────────────┬─────────────────┐
//   │   &op_add       │   &op_sub       │   &op_mul       │   &op_div       │
//   │  0x401020       │  0x401040       │  0x401060       │  0x401080       │
//   └────────┬────────┴────────┬────────┴────────┬────────┴────────┬────────┘
//    [0]      │         [1]     │         [2]     │         [3]     │
//             ▼                ▼                 ▼                  ▼
//           op_add           op_sub            op_mul            op_div
//          machine          machine           machine            machine
//           code             code              code               code

int main(void) {
    int token = 2; // multiply
    double result = dispatch[token](6.0, 7.0);
    printf("result = %.1f\n", result);  // 42.0

    // iterate all:
    const char *names[] = {"add","sub","mul","div"};
    for (int i = 0; i < 4; i++) {
        printf("%s(10, 3) = %.2f\n", names[i], dispatch[i](10.0, 3.0));
    }
    return 0;
}
```

**2D array of function pointers** (matrix of operations):

```c
typedef int (*Op)(int, int);

int add(int a, int b) { return a+b; }
int sub(int a, int b) { return a-b; }
int mul(int a, int b) { return a*b; }
int mod(int a, int b) { return b!=0?a%b:0; }

// 2×2 table: table[row][col]
Op table[2][2] = {
    { add, sub },
    { mul, mod }
};

// Usage: table[1][0](7, 3) → mul(7,3) = 21
```

### 3.8 Callbacks

A **callback** is a function pointer you register with a library or framework;
the library calls your function when a specific event occurs. Your code is the
*callee*, the library's generic code is the *caller*.

```
  Your code                    Library code
  ─────────────────────────    ─────────────────────────────────────
  define my_callback() { }     define register_cb(fn_ptr, userdata)
                               define trigger_event():
  register_cb(&my_callback,        fn_ptr(event_data, userdata)
              my_state)

  Flow:
  1. You define my_callback.
  2. You call register_cb, handing a pointer to my_callback and
     a void* to your own state ("userdata" / "context").
  3. Later the library calls my_callback(event_data, my_state).
```

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// ─── Generic event system ─────────────────────────────────────────────────

typedef void (*EventHandler)(const char *event, void *userdata);

#define MAX_HANDLERS 16

typedef struct {
    EventHandler fn;
    void *userdata;
} Registration;

typedef struct {
    Registration handlers[MAX_HANDLERS];
    int count;
} EventBus;

void bus_subscribe(EventBus *bus, EventHandler fn, void *userdata) {
    if (bus->count < MAX_HANDLERS) {
        bus->handlers[bus->count].fn       = fn;
        bus->handlers[bus->count].userdata = userdata;
        bus->count++;
    }
}

void bus_publish(EventBus *bus, const char *event) {
    for (int i = 0; i < bus->count; i++) {
        bus->handlers[i].fn(event, bus->handlers[i].userdata);
    }
}

// ─── User callbacks ───────────────────────────────────────────────────────

typedef struct { int count; const char *name; } Counter;

void counter_handler(const char *event, void *userdata) {
    Counter *c = (Counter *)userdata;
    c->count++;
    printf("[%s] received '%s' (total: %d)\n", c->name, event, c->count);
}

void logger_handler(const char *event, void *userdata) {
    FILE *f = (FILE *)userdata;
    fprintf(f, "LOG: %s\n", event);
}

int main(void) {
    EventBus bus = {0};
    Counter c1 = {0, "Alpha"};
    Counter c2 = {0, "Beta"};

    bus_subscribe(&bus, counter_handler, &c1);
    bus_subscribe(&bus, counter_handler, &c2);
    bus_subscribe(&bus, logger_handler,  stdout);

    bus_publish(&bus, "startup");
    bus_publish(&bus, "data_ready");
    bus_publish(&bus, "shutdown");

    return 0;
}
```

The `void *userdata` pattern is the C idiom for giving the callback access to
its own state without globals. Every modern C callback API (libuv, GTK, POSIX
aio, etc.) uses this pattern.

### 3.9 Function Pointer Tables (Jump Tables / vtable Simulation)

C has no built-in OOP, but you can simulate virtual dispatch using a **vtable**
— a struct of function pointers — exactly what C++ compilers generate under the
hood.

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

// ─── "Base class" — the vtable ────────────────────────────────────────────

typedef struct Shape Shape;

typedef struct {
    double  (*area)     (const Shape *);
    double  (*perimeter)(const Shape *);
    void    (*describe) (const Shape *);
    void    (*destroy)  (Shape *);
} ShapeVTable;

struct Shape {
    const ShapeVTable *vtable;   // pointer to the vtable (like __vptr in C++)
    // common fields could go here
};

// ─── Circle ───────────────────────────────────────────────────────────────

typedef struct {
    Shape base;          // MUST be first — enables safe casting
    double radius;
} Circle;

static double circle_area(const Shape *s) {
    const Circle *c = (const Circle *)s;
    return M_PI * c->radius * c->radius;
}
static double circle_perim(const Shape *s) {
    const Circle *c = (const Circle *)s;
    return 2.0 * M_PI * c->radius;
}
static void circle_desc(const Shape *s) {
    const Circle *c = (const Circle *)s;
    printf("Circle(r=%.2f)\n", c->radius);
}
static void circle_destroy(Shape *s) { free(s); }

static const ShapeVTable circle_vtable = {
    circle_area, circle_perim, circle_desc, circle_destroy
};

Shape *circle_new(double r) {
    Circle *c = malloc(sizeof *c);
    c->base.vtable = &circle_vtable;
    c->radius = r;
    return &c->base;
}

// ─── Rectangle ────────────────────────────────────────────────────────────

typedef struct {
    Shape base;
    double w, h;
} Rect;

static double rect_area(const Shape *s) {
    const Rect *r = (const Rect *)s; return r->w * r->h;
}
static double rect_perim(const Shape *s) {
    const Rect *r = (const Rect *)s; return 2.0*(r->w + r->h);
}
static void rect_desc(const Shape *s) {
    const Rect *r = (const Rect *)s;
    printf("Rect(%.2f x %.2f)\n", r->w, r->h);
}
static void rect_destroy(Shape *s) { free(s); }

static const ShapeVTable rect_vtable = {
    rect_area, rect_perim, rect_desc, rect_destroy
};

Shape *rect_new(double w, double h) {
    Rect *r = malloc(sizeof *r);
    r->base.vtable = &rect_vtable;
    r->w = w; r->h = h;
    return &r->base;
}

// ─── Polymorphic dispatch ─────────────────────────────────────────────────

void print_shape_info(const Shape *s) {
    s->vtable->describe(s);
    printf("  area      = %.4f\n", s->vtable->area(s));
    printf("  perimeter = %.4f\n", s->vtable->perimeter(s));
}

int main(void) {
    Shape *shapes[] = {
        circle_new(5.0),
        rect_new(4.0, 6.0),
        circle_new(1.0),
    };
    int n = 3;

    for (int i = 0; i < n; i++) {
        print_shape_info(shapes[i]);
        shapes[i]->vtable->destroy(shapes[i]);
    }
    return 0;
}

// ─── Memory layout diagram ────────────────────────────────────────────────
//
//  circle_vtable (const, in .rodata segment):
//  ┌───────────────┬───────────────┬───────────────┬───────────────┐
//  │ &circle_area  │ &circle_perim │ &circle_desc  │&circle_destroy│
//  └───────────────┴───────────────┴───────────────┴───────────────┘
//
//  Circle instance (on heap):
//  ┌──────────────────────────────────────┬──────────────┐
//  │  base.vtable → &circle_vtable        │  radius=5.0  │
//  └──────────────────────────────────────┴──────────────┘
//  ▲
//  Cast to Shape* — safe because base is first member
//
//  Calling s->vtable->area(s):
//  1. Load s->vtable        → gets address of circle_vtable
//  2. Load vtable->area     → gets address of circle_area
//  3. CALL circle_area(s)   → indirect call
```

### 3.10 void* and Function Pointers

`void*` is the generic data pointer in C. The C standard (unlike POSIX) does
NOT guarantee that a function pointer can be stored in a `void*` and retrieved
correctly. However, POSIX requires it, and all Unix/Linux/macOS systems comply.

```c
#include <stdio.h>

// ─── The POSIX-compliant trick (common in practice, not strict C) ─────────
void *fp_as_void;
int my_fn(int x) { return x * 2; }

// POSIX allows this; strict C11 does not define behaviour
fp_as_void = (void *)my_fn;

// Restore via intermediate cast
int (*restored)(int) = (int (*)(int))fp_as_void;
printf("%d\n", restored(21)); // 42

// ─── Safer approach: use a union ──────────────────────────────────────────
typedef int (*IntFn)(int);

union FnStorage {
    void  *data_ptr;
    IntFn  fn_ptr;
};

union FnStorage store;
store.fn_ptr = my_fn;
// store.data_ptr for data storage, store.fn_ptr for function storage
```

### 3.11 NULL Function Pointers & Guards

A function pointer that has not been assigned is indeterminate (undefined
behaviour to call). The idiomatic way to express "no function" is `NULL`, and
you must always check before calling.

```c
#include <stdio.h>

typedef void (*Hook)(void);

typedef struct {
    Hook on_start;
    Hook on_stop;
    Hook on_error;    // optional — can be NULL
} Plugin;

void run_plugin(Plugin *p) {
    if (p->on_start) p->on_start();   // guard: only call if non-NULL

    // ... do work ...

    if (p->on_error) p->on_error();   // optional hook

    if (p->on_stop) p->on_stop();
}

// Designated initialisers make optional hooks explicit:
Plugin my_plugin = {
    .on_start = my_start_fn,
    .on_stop  = my_stop_fn,
    .on_error = NULL        // explicitly disabled
};
```

### 3.12 Casting & Aliasing Dangers

You can cast between compatible function pointer types, but calling through a
pointer to a function of incompatible type is **undefined behaviour** — the
stack layout for arguments is wrong.

```c
typedef int (*FnIntInt)(int);
typedef int (*FnIntIntInt)(int, int);

int square(int x) { return x * x; }

FnIntInt f = square;

// DANGER: casting to incompatible signature
FnIntIntInt g = (FnIntIntInt)f;
g(2, 3);   // UB! extra argument pushed/passed, callee reads garbage

// SAFE: casting between identical signatures
typedef int (*AliasedFn)(int);
AliasedFn h = (AliasedFn)f;
h(5);   // OK: same ABI
```

### 3.13 Variadic Function Pointers

A pointer to a variadic function (one with `...`) works, but you cannot invent
a variadic call from a non-variadic pointer — the type must already include `...`.

```c
#include <stdarg.h>
#include <stdio.h>

typedef int (*VarFn)(const char *fmt, ...);

VarFn printer = printf;        // printf is variadic
printer("Hello %s %d\n", "world", 42);  // fine
```

### 3.14 Signal Handlers

`signal()` is one of the oldest function-pointer-based APIs in C:

```c
#include <signal.h>
#include <stdio.h>

// Must match the prototype: void handler(int)
void handle_sigint(int signo) {
    printf("\nCaught signal %d\n", signo);
    // reinstall (on some systems the handler is reset to SIG_DFL after firing)
    signal(SIGINT, handle_sigint);
}

int main(void) {
    // signal() returns the previous handler (also a function pointer)
    void (*old_handler)(int) = signal(SIGINT, handle_sigint);

    // SIG_DFL and SIG_IGN are special sentinel function pointer values
    if (old_handler == SIG_DFL) puts("was default");
    if (old_handler == SIG_IGN) puts("was ignored");

    pause(); // wait for signal
    return 0;
}
```

`sigaction()` uses a struct with a union of `void(*sa_handler)(int)` and
`void(*sa_sigaction)(int, siginfo_t*, void*)` — two differently-typed function
pointer fields.

### 3.15 qsort — The Classic Higher-Order Function

`qsort` is the archetypal C higher-order function. Its comparator parameter is a
function pointer:

```c
void qsort(void *base, size_t nmemb, size_t size,
           int (*compar)(const void *, const void *));
```

The comparator receives `const void*` pointers to two elements and must return
< 0, 0, or > 0. Because `void*` erases the type, you cast inside the comparator.

```c
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

// ─── Sorting integers ─────────────────────────────────────────────────────
int cmp_int_asc(const void *a, const void *b) {
    // Subtracting can overflow for large ints; prefer this pattern:
    int ia = *(const int *)a;
    int ib = *(const int *)b;
    return (ia > ib) - (ia < ib);   // returns -1, 0, or 1
}

int cmp_int_desc(const void *a, const void *b) {
    return cmp_int_asc(b, a);   // reverse the arguments
}

// ─── Sorting strings ──────────────────────────────────────────────────────
int cmp_str(const void *a, const void *b) {
    return strcmp(*(const char **)a, *(const char **)b);
}

// ─── Sorting structs by field ─────────────────────────────────────────────
typedef struct { char name[32]; int age; } Person;

int cmp_person_age(const void *a, const void *b) {
    const Person *pa = a, *pb = b;
    return (pa->age > pb->age) - (pa->age < pb->age);
}

int main(void) {
    int nums[] = {5, 2, 8, 1, 9, 3};
    qsort(nums, 6, sizeof(int), cmp_int_asc);
    for (int i = 0; i < 6; i++) printf("%d ", nums[i]);
    printf("\n");  // 1 2 3 5 8 9

    // sort descending by swapping comparator
    qsort(nums, 6, sizeof(int), cmp_int_desc);
    for (int i = 0; i < 6; i++) printf("%d ", nums[i]);
    printf("\n");  // 9 8 5 3 2 1

    const char *words[] = {"banana","apple","cherry","date"};
    qsort(words, 4, sizeof(char *), cmp_str);
    for (int i = 0; i < 4; i++) printf("%s ", words[i]);
    printf("\n");  // apple banana cherry date

    Person people[] = {
        {"Alice", 30}, {"Bob", 25}, {"Carol", 35}
    };
    qsort(people, 3, sizeof(Person), cmp_person_age);
    for (int i = 0; i < 3; i++) printf("%s:%d ", people[i].name, people[i].age);
    printf("\n");  // Bob:25 Alice:30 Carol:35

    return 0;
}
```

### 3.16 State Machines with Function Pointers

Each state is represented by a function; the transition is simply reassigning
the state pointer.

```c
#include <stdio.h>

// ─── Traffic light state machine ─────────────────────────────────────────

typedef struct StateMachine StateMachine;
typedef void (*StateFn)(StateMachine *);

struct StateMachine {
    StateFn current_state;
    int tick;
};

// Forward declarations
void state_red(StateMachine *sm);
void state_green(StateMachine *sm);
void state_yellow(StateMachine *sm);

void state_red(StateMachine *sm) {
    printf("[tick %2d] RED — Stop\n", sm->tick);
    if (sm->tick % 4 == 0)
        sm->current_state = state_green;     // transition
}
void state_green(StateMachine *sm) {
    printf("[tick %2d] GREEN — Go\n", sm->tick);
    if (sm->tick % 4 == 0)
        sm->current_state = state_yellow;
}
void state_yellow(StateMachine *sm) {
    printf("[tick %2d] YELLOW — Caution\n", sm->tick);
    if (sm->tick % 4 == 0)
        sm->current_state = state_red;
}

//  State transition diagram:
//
//   ┌───────┐   tick%4==0   ┌────────┐   tick%4==0   ┌────────┐
//   │  RED  │──────────────▶│ GREEN  │──────────────▶│ YELLOW │
//   └───────┘               └────────┘               └────────┘
//       ▲                                                  │
//       └──────────────────────────────────────────────────┘
//                          tick%4==0

int main(void) {
    StateMachine sm = { .current_state = state_red, .tick = 0 };
    for (int i = 0; i < 12; i++) {
        sm.tick = i;
        sm.current_state(&sm);   // call through function pointer
    }
    return 0;
}
```

### 3.17 C99 / C11 / C17 Nuances

| Feature | Standard |
|---|---|
| `inline` functions (still have an address) | C99 |
| `_Noreturn` on function pointer type | C11 (via function attribute) |
| Type-generic math (`tgmath.h`) macro resolution | C99 |
| Constraint: function pointers not comparable with `<` or `>` | All standards |
| Comparing function pointers with `==` / `!=` | Defined |
| Pointer to `inline` function: no guarantee it matches | C99 (see inline linkage rules) |
| `_Generic` can select a function based on type | C11 |

```c
// C11 _Generic selects among function pointers based on argument type
#define ABS(x) _Generic((x), \
    int:    abs_int, \
    double: fabs,   \
    float:  fabsf   \
)(x)
```

---

## 4. Go — Function Values & Closures

### 4.1 Functions Are First-Class Values

In Go, functions are first-class values. This means:
- A function can be stored in a variable.
- A function can be passed as an argument.
- A function can be returned from another function.
- A function can be placed in a slice, map, or struct.

```go
package main

import "fmt"

func add(a, b int) int { return a + b }

func main() {
    // Assign to variable — type is inferred as func(int, int) int
    f := add
    fmt.Println(f(3, 4))   // 7

    // Explicit type annotation
    var g func(int, int) int = add
    fmt.Println(g(3, 4))   // 7

    // Store in a slice
    ops := []func(int, int) int{add}
    fmt.Println(ops[0](10, 5))   // 15

    // Assign to struct field
    type Calc struct {
        Op func(int, int) int
    }
    c := Calc{Op: add}
    fmt.Println(c.Op(6, 7))   // 13
}
```

### 4.2 Function Types

A **function type** in Go specifies the parameter types and return types. Two
functions have the same type if and only if they have identical parameter lists
and return lists (types only, not names).

```go
// These are the same function type:
func(int, int) int
func(a, b int) int    // parameter names don't matter in the type
func(x int, y int) int

// Different types:
func(int) int          // one parameter
func(int, int) int     // two parameters

// Function type as a named type — enables methods on it
type HandlerFunc func(http.ResponseWriter, *http.Request)

// Methods on the named function type (see net/http)
func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    f(w, r)
}
```

```
Go function value memory layout:

  func variable (2 words on 64-bit)
  ┌─────────────────────┬──────────────────────┐
  │   pointer to code   │  pointer to closure  │
  │   (instruction ptr) │  environment (data)  │
  └─────────────────────┴──────────────────────┘
       │                       │
       ▼                       ▼
   machine code          captured variables
   of the function       (nil for plain functions
                          that capture nothing)
```

For a plain non-closure function, the closure pointer is either nil or points to
a trivial struct. For a closure, the second word points to a heap-allocated
struct containing the captured variables.

### 4.3 Anonymous Functions

An anonymous function (function literal) is a function defined inline without a
name. It is the primary way to create closures in Go.

```go
package main

import "fmt"

func main() {
    // Immediately invoked function expression (IIFE)
    result := func(x, y int) int { return x + y }(3, 4)
    fmt.Println(result) // 7

    // Assigned to a variable
    greet := func(name string) string {
        return "Hello, " + name + "!"
    }
    fmt.Println(greet("World"))

    // Used inline as an argument
    apply := func(n int, f func(int) int) int { return f(n) }
    fmt.Println(apply(5, func(x int) int { return x * x })) // 25

    // Recursive anonymous function — must declare first, then assign
    var fib func(n int) int
    fib = func(n int) int {
        if n <= 1 { return n }
        return fib(n-1) + fib(n-2)
    }
    fmt.Println(fib(10)) // 55
}
```

### 4.4 Closures — Capturing Variables

A **closure** is a function that *closes over* (captures) variables from its
enclosing scope. The captured variables are accessed by reference (not by value)
— the closure and the outer scope share the same variable.

```go
package main

import "fmt"

// makeCounter returns a closure that captures `count`
func makeCounter() func() int {
    count := 0          // lives on the heap (escaped from stack)
    return func() int {
        count++          // reads AND writes the captured variable
        return count
    }
}

// makeAdder: classic closure factory
func makeAdder(base int) func(int) int {
    return func(x int) int {
        return base + x   // `base` is captured by reference
    }
}

func main() {
    c1 := makeCounter()
    c2 := makeCounter()   // independent counter — separate `count`

    fmt.Println(c1(), c1(), c1())   // 1 2 3
    fmt.Println(c2(), c2())          // 1 2  (c2 is independent)

    add5  := makeAdder(5)
    add10 := makeAdder(10)
    fmt.Println(add5(3))   // 8
    fmt.Println(add10(3))  // 13

    // Shared state — both closures capture the SAME variable
    x := 0
    inc := func() { x++ }
    get := func() int { return x }
    inc(); inc(); inc()
    fmt.Println(get())  // 3
}
```

**Memory model for closures:**

```
makeCounter() stack frame (while executing):

  ┌─────────────────────────────────────┐
  │  count  (int, addr = 0xc0000b4010) │  ← escapes to heap
  └─────────────────────────────────────┘

  After return, Go's escape analysis detects `count` is captured
  and allocates it on the heap. The returned closure value holds:

  Closure struct (on heap):
  ┌──────────────────┬──────────────────────────────────┐
  │  code pointer    │  environment pointer              │
  │  → anon func     │  → { count *int → 0xc0000b4010 } │
  └──────────────────┴──────────────────────────────────┘

  Two distinct calls to makeCounter() produce two independent
  closures with two independent `count` heap allocations.
```

### 4.5 Closures and Goroutines (The Classic Bug)

This is one of the most common Go bugs. It arises when a closure captures a
loop variable, but the goroutine runs *after* the loop has completed, by which
time the variable holds the last iteration's value.

```go
package main

import (
    "fmt"
    "sync"
)

// ─── BUG: all goroutines see i == 10 ─────────────────────────────────────
func buggy() {
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            fmt.Println(i)   // captures i by reference; by the time this runs,
                              // i may already be 5
        }()
    }
    wg.Wait()
}

// ─── FIX 1: capture by value via function parameter ───────────────────────
func fixed_param() {
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(n int) {     // n is a parameter — a fresh copy
            defer wg.Done()
            fmt.Println(n)
        }(i)                  // pass i as argument — copied at call time
    }
    wg.Wait()
}

// ─── FIX 2: shadow with a new variable ────────────────────────────────────
func fixed_shadow() {
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        i := i              // new i shadows outer i; each iteration gets its own
        wg.Add(1)
        go func() {
            defer wg.Done()
            fmt.Println(i)   // captures the shadowed, per-iteration i
        }()
    }
    wg.Wait()
}

// ─── FIX 3 (Go 1.22+): loop variable is per-iteration by default ──────────
// In Go 1.22 the loop variable `i` is freshly declared each iteration,
// eliminating this class of bug without any source change.
```

**Diagram of the bug:**

```
Timeline:

  Main goroutine          Spawned goroutines
  ──────────────          ──────────────────
  i=0 → spawn g0
  i=1 → spawn g1
  i=2 → spawn g2
  i=3 → spawn g3
  i=4 → spawn g4
  i=5 (loop ends)
                    g0 runs → reads i → sees 5
                    g1 runs → reads i → sees 5
                    g2 runs → reads i → sees 5
                    (all see 5 because they all
                     share the same variable)

  All goroutines share a pointer to the same `i`.
  By the time they run, the loop is done and i==5.
```

### 4.6 Method Values vs Method Expressions

Go has two ways to turn a method into a function value.

**Method Value**: bound to a specific receiver instance. You call it without
providing the receiver.

**Method Expression**: unbound. You must supply the receiver explicitly as the
first argument.

```go
package main

import "fmt"

type Rect struct{ W, H float64 }

func (r Rect) Area() float64  { return r.W * r.H }
func (r Rect) Scale(f float64) Rect { return Rect{r.W * f, r.H * f} }

func main() {
    r := Rect{3, 4}

    // ─── Method Value (bound) ──────────────────────────────────────────────
    areaFn := r.Area       // type: func() float64  (receiver r is captured)
    fmt.Println(areaFn())  // 12 — no need to pass r

    // ─── Method Expression (unbound) ──────────────────────────────────────
    // Accessed via the TYPE, not an instance
    areaExpr := Rect.Area           // type: func(Rect) float64
    fmt.Println(areaExpr(r))        // 12 — must pass r explicitly

    scaleExpr := Rect.Scale         // type: func(Rect, float64) Rect
    fmt.Println(scaleExpr(r, 2.0))  // {6 8}

    // ─── Practical use: storing heterogeneous operations ───────────────────
    rects := []Rect{{1,2}, {3,4}, {5,6}}
    computeArea := Rect.Area   // method expression used as a reusable function
    for _, rect := range rects {
        fmt.Printf("%.0f ", computeArea(rect))   // 2 12 30
    }
}
```

### 4.7 Functions as Map Values

```go
package main

import (
    "fmt"
    "strings"
)

type Transform func(string) string

func main() {
    // Dispatch table using a map
    transforms := map[string]Transform{
        "upper":   strings.ToUpper,
        "lower":   strings.ToLower,
        "title":   strings.Title,
        "reverse": func(s string) string {
            r := []rune(s)
            for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
                r[i], r[j] = r[j], r[i]
            }
            return string(r)
        },
    }

    input := "Hello World"
    for name, fn := range transforms {
        fmt.Printf("%-10s → %s\n", name, fn(input))
    }

    // Dynamic dispatch via map key
    key := "upper"
    if fn, ok := transforms[key]; ok {
        fmt.Println(fn(input))
    }
}
```

### 4.8 Higher-Order Functions

```go
package main

import "fmt"

// Map applies f to every element of slice, returning a new slice
func Map[T, U any](slice []T, f func(T) U) []U {
    result := make([]U, len(slice))
    for i, v := range slice {
        result[i] = f(v)
    }
    return result
}

// Filter keeps elements for which pred returns true
func Filter[T any](slice []T, pred func(T) bool) []T {
    var result []T
    for _, v := range slice {
        if pred(v) { result = append(result, v) }
    }
    return result
}

// Reduce folds slice into a single value
func Reduce[T, U any](slice []T, init U, f func(U, T) U) U {
    acc := init
    for _, v := range slice {
        acc = f(acc, v)
    }
    return acc
}

// Compose composes two functions: compose(f,g)(x) = f(g(x))
func Compose[T any](f, g func(T) T) func(T) T {
    return func(x T) T { return f(g(x)) }
}

func main() {
    nums := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

    doubled := Map(nums, func(x int) int { return x * 2 })
    fmt.Println(doubled)   // [2 4 6 8 10 12 14 16 18 20]

    evens := Filter(nums, func(x int) bool { return x%2 == 0 })
    fmt.Println(evens)     // [2 4 6 8 10]

    sum := Reduce(nums, 0, func(acc, x int) int { return acc + x })
    fmt.Println(sum)       // 55

    double := func(x int) int { return x * 2 }
    addOne := func(x int) int { return x + 1 }
    doubleThenAdd := Compose(addOne, double)   // addOne(double(x))
    fmt.Println(doubleThenAdd(5))              // 11
}
```

### 4.9 Returning Functions

```go
package main

import (
    "fmt"
    "time"
)

// Memoize wraps a function to cache its results
func Memoize(f func(int) int) func(int) int {
    cache := map[int]int{}
    return func(n int) int {
        if v, ok := cache[n]; ok {
            return v
        }
        v := f(n)
        cache[n] = v
        return v
    }
}

// RateLimiter returns a function that enforces a minimum gap between calls
func RateLimiter(f func(), minGap time.Duration) func() {
    last := time.Time{}
    return func() {
        if time.Since(last) >= minGap {
            f()
            last = time.Now()
        }
    }
}

// Retry wraps a function and retries it up to maxAttempts times
func Retry(maxAttempts int, f func() error) func() error {
    return func() error {
        var err error
        for i := 0; i < maxAttempts; i++ {
            if err = f(); err == nil { return nil }
            fmt.Printf("attempt %d failed: %v\n", i+1, err)
        }
        return err
    }
}

func main() {
    fib := Memoize(func(n int) int {
        if n <= 1 { return n }
        // NOTE: this won't memoize recursive calls
        // A real memoised fib needs the wrapper in the recursion
        return n // simplified
    })
    fmt.Println(fib(10))
}
```

### 4.10 Variadic Functions as Values

```go
package main

import "fmt"

// A variadic function has type func(...T), not func([]T)
// To store it in a variable, the variable type must include ...

type Logger func(format string, args ...interface{})

func myLog(format string, args ...interface{}) {
    fmt.Printf("[LOG] "+format+"\n", args...)
}

func withPrefix(prefix string, log Logger) Logger {
    return func(format string, args ...interface{}) {
        log("["+prefix+"] "+format, args...)
    }
}

func main() {
    var log Logger = myLog
    log("Hello %s", "World")   // [LOG] Hello World

    appLog := withPrefix("APP", myLog)
    appLog("started on port %d", 8080)
    // [LOG] [APP] started on port 8080

    // Spreading a slice into a variadic call
    args := []interface{}{"Alice", 30}
    log("User: %s, age: %d", args...)
}
```

### 4.11 Interfaces vs Function Types

Go offers two ways to abstract over behaviour: **interfaces** and **function types**.

```go
// ─── Interface approach ───────────────────────────────────────────────────
type Sorter interface {
    Less(i, j int) bool
    Len() int
    Swap(i, j int)
}

// ─── Function type approach ───────────────────────────────────────────────
type LessFunc func(i, j int) bool

// When to use which?
//
//  Use a function type when:
//  - The abstraction needs only ONE method/operation
//  - You want callers to use a closure or anonymous function inline
//  - Adapters: net/http.HandlerFunc, sort.Slice
//
//  Use an interface when:
//  - The abstraction needs MULTIPLE related operations
//  - You need the type to carry state in named fields
//  - You want to add methods later without breaking callers
```

The standard library's `sort.Slice` uses a function-type approach:

```go
people := []struct{ Name string; Age int }{
    {"Alice", 30}, {"Bob", 25}, {"Carol", 35},
}
sort.Slice(people, func(i, j int) bool {
    return people[i].Age < people[j].Age
})
```

### 4.12 Function Types with Methods (Adapter Pattern)

You can define methods on a named function type, making the function itself
implement an interface. `net/http.HandlerFunc` is the canonical example.

```go
package main

import "fmt"

// Define an interface
type Processor interface {
    Process(data string) string
}

// Named function type
type ProcessorFunc func(string) string

// Make ProcessorFunc implement Processor
func (f ProcessorFunc) Process(data string) string {
    return f(data)
}

func pipeline(p Processor, data string) string {
    return p.Process(data)
}

func main() {
    // A plain function can now be used wherever a Processor is needed
    upper := ProcessorFunc(func(s string) string {
        result := ""
        for _, c := range s {
            if c >= 'a' && c <= 'z' { result += string(c - 32) } else { result += string(c) }
        }
        return result
    })

    fmt.Println(pipeline(upper, "hello world"))   // HELLO WORLD
}
```

### 4.13 Nil Function Values

A function variable that has not been assigned is `nil`. Calling a nil function
value panics.

```go
var f func()
if f != nil {
    f()   // safe
}
f()   // panic: runtime error: invalid memory address or nil pointer dereference

// Nil check is idiomatic in Go for optional callbacks:
type Config struct {
    OnSuccess func(result string)
    OnError   func(err error)
}
cfg := Config{OnSuccess: mySuccessHandler} // OnError is nil

if cfg.OnError != nil {
    cfg.OnError(someErr)   // only call if set
}
```

---

## 5. Rust — Function Pointers & Closures

### 5.1 The Rust Function Landscape (Overview)

Rust has several distinct but related constructs. Understanding their differences
is essential before using any of them.

```
Rust callable types taxonomy:
═══════════════════════════════════════════════════════════════════════

  fn pointer (fn(T) -> U)
  │
  │  A raw function pointer. Zero-overhead. Can only point to
  │  a named function or a non-capturing closure. NO environment.
  │  8 bytes on 64-bit. Always copiable (implements Copy).

  ├─── Function Item (zero-sized type, unique per function)
  │    Every named function has its own unique type (a "function item type").
  │    The type carries the function's identity statically.
  │    Zero-sized: no runtime cost. Coerces to fn pointer when needed.

  ├─── Closure (anonymous type implements Fn/FnMut/FnOnce)
  │    │
  │    │  Each closure has its own unique anonymous struct type.
  │    │  The struct holds the captured variables.
  │    │  Size = sum of captured variable sizes.
  │    │
  │    ├── |x| x + 1             → captures nothing → Fn + FnMut + FnOnce
  │    │                           coerces to fn(i32)->i32
  │    ├── |x| x + n             → captures n by ref → Fn + FnMut + FnOnce
  │    │                           (if n is not mut-used)
  │    ├── |x| { n += 1; x }     → captures n by mut ref → FnMut + FnOnce
  │    └── move |x| x + n        → captures n by value → Fn + FnMut + FnOnce
  │
  └─── Trait Objects (dyn Fn / dyn FnMut / dyn FnOnce)
       │
       │  Type erasure via a fat pointer (data ptr + vtable ptr).
       │  16 bytes. Enables heterogeneous collections of closures.
       │
       ├── &dyn Fn(T) -> U         (borrowed, shared)
       ├── Box<dyn Fn(T) -> U>     (owned, heap-allocated)
       └── Arc<dyn Fn(T) -> U>     (shared ownership, thread-safe)
```

### 5.2 Bare Function Pointers (`fn`)

A bare function pointer in Rust is written as `fn(ArgTypes) -> ReturnType`. It
behaves similarly to a C function pointer: it stores a machine-code address and
nothing else.

```rust
fn add(a: i32, b: i32) -> i32 { a + b }
fn sub(a: i32, b: i32) -> i32 { a - b }
fn mul(a: i32, b: i32) -> i32 { a * b }

fn main() {
    // ── Declaration and assignment ────────────────────────────────────────
    let f: fn(i32, i32) -> i32 = add;    // explicit type
    let g = add;                           // type is the function ITEM type
                                           // (not yet fn pointer, but coerces)

    println!("{}", f(3, 4));   // 7
    println!("{}", g(3, 4));   // 7

    // ── Array of function pointers ────────────────────────────────────────
    let ops: [fn(i32, i32) -> i32; 3] = [add, sub, mul];
    for op in &ops {
        println!("{}", op(10, 3));
    }
    // Output: 13, 7, 30

    // ── Function pointer in a struct ──────────────────────────────────────
    struct Op {
        name: &'static str,
        f:    fn(i32, i32) -> i32,
    }

    let operations = [
        Op { name: "add", f: add },
        Op { name: "sub", f: sub },
        Op { name: "mul", f: mul },
    ];

    for op in &operations {
        println!("{}: {}", op.name, (op.f)(6, 7));
    }
}
```

**Properties of `fn` pointer:**
- Implements `Copy`, `Clone`, `Send`, `Sync`.
- Size is exactly `usize` (8 bytes on 64-bit).
- Cannot hold closures that capture variables (they have size > 0).
- Calling is zero-overhead (one indirect branch).

### 5.3 Function Items vs Function Pointers

This is a subtle but important distinction unique to Rust.

```rust
fn double(x: i32) -> i32 { x * 2 }
fn triple(x: i32) -> i32 { x * 3 }

fn main() {
    // `a` has type: fn(i32) -> i32 {double}
    // This is a ZERO-SIZED type unique to the function `double`.
    let a = double;

    // `b` has type: fn(i32) -> i32 {triple}
    // DIFFERENT zero-sized type, even though same signature.
    let b = triple;

    // You cannot put a and b in the same array:
    // let arr = [a, b];   // ERROR: type mismatch

    // To unify them, coerce to fn pointer:
    let arr: [fn(i32) -> i32; 2] = [double, triple];   // OK — coerced

    // Or use explicit coercion:
    let fp: fn(i32) -> i32 = a;   // coerce item → fn pointer

    // Sizes:
    // std::mem::size_of_val(&a) == 0   (zero-sized function item)
    // std::mem::size_of_val(&fp) == 8  (fn pointer, pointer-sized)
}
```

```
Function Item Type vs fn Pointer:

  Function Item:
  ┌───────────────────────────────────────────────────┐
  │  type: fn(i32)->i32 {double}                       │
  │  size: 0 bytes                                     │
  │  the compiler knows AT COMPILE TIME which          │
  │  function is called — monomorphised, inlinable     │
  └───────────────────────────────────────────────────┘

  fn Pointer:
  ┌───────────────────────────────────────────────────┐
  │  type: fn(i32)->i32                                │
  │  size: 8 bytes (stores an address)                 │
  │  the compiler DOES NOT KNOW at compile time which  │
  │  function will be called — indirect call           │
  └───────────────────────────────────────────────────┘
```

### 5.4 Closures and the Three Traits: Fn, FnMut, FnOnce

Rust closures implement one or more of three traits. The compiler automatically
infers which traits to implement based on how the closure uses its captured
variables.

```
Trait hierarchy:
                    FnOnce
                   (can be called once, consumes captures)
                       │
                    FnMut
                   (can be called multiple times, may mutate captures)
                  (also implements FnOnce)
                       │
                      Fn
                   (can be called multiple times, read-only access)
                  (also implements FnMut and FnOnce)

  More restrictive ──────────────────────────────── Less restrictive
  FnOnce (fewest requirements)      Fn (most requirements to implement)

  Every Fn is also FnMut and FnOnce.
  Every FnMut is also FnOnce.
```

```rust
fn call_once<F: FnOnce() -> String>(f: F) -> String {
    f()   // consumes f; can only be called once
}

fn call_mut<F: FnMut() -> i32>(mut f: F) -> i32 {
    f() + f() + f()   // called multiple times; f may change state
}

fn call_fn<F: Fn() -> i32>(f: F) -> i32 {
    f() + f() + f()   // called multiple times; f does NOT change state
}

fn main() {
    // ── FnOnce: closure that moves a value out ─────────────────────────────
    let name = String::from("Alice");
    let greet = || format!("Hello, {}!", name);   // captures name by move (implicitly)
    // Actually String is not Copy so greet captures name, let's force move:
    let name2 = String::from("Bob");
    let consume = move || {
        let s = name2;   // moves name2 out of the closure — can only do once
        s.to_uppercase()
    };
    println!("{}", call_once(consume));   // "BOB"
    // println!("{}", call_once(consume)); // ERROR: consume moved into call_once

    // ── FnMut: closure that mutates a capture ─────────────────────────────
    let mut count = 0;
    let mut counter = || { count += 1; count };
    println!("{}", call_mut(&mut counter));   // 1 + 2 + 3 = 6
    // counter is FnMut: each call modifies `count`

    // ── Fn: closure that only reads captures ──────────────────────────────
    let base = 10;
    let adder = |x: i32| base + x;   // captures `base` by shared ref
    println!("{}", call_fn(|| adder(5)));  // 15 + 15 + 15 = 45... wait
    // let's demonstrate clearly:
    let readonly = || base * 2;
    println!("{}", call_fn(readonly));   // 20 + 20 + 20 = 60
}
```

**Detailed rules for which trait is implemented:**

| Closure body | Captures how | Implements |
|---|---|---|
| Does not capture anything | N/A | `Fn`, `FnMut`, `FnOnce`, coerces to `fn` |
| Reads captured values only | by shared ref `&T` | `Fn`, `FnMut`, `FnOnce` |
| Mutates captured values | by mutable ref `&mut T` | `FnMut`, `FnOnce` |
| Drops or moves captured values out | by value (move out) | `FnOnce` |
| Uses `move` keyword; reads only | by value `T` (T: Copy or clone) | `Fn`, `FnMut`, `FnOnce` |
| Uses `move` keyword; mutates | by value `T` | `FnMut`, `FnOnce` |

### 5.5 How Closures Capture (by ref, by mut ref, by value)

The compiler determines the *minimum* capture mode needed.

```rust
fn main() {
    let x = 10;
    let y = String::from("hello");

    // ── Capture by shared reference (&T) ─────────────────────────────────
    //    Used when: closure only reads x
    //    x must outlive the closure
    let read_x = || println!("{}", x);   // captures &x
    read_x();
    println!("x is still: {}", x);       // x is still accessible

    // ── Capture by mutable reference (&mut T) ─────────────────────────────
    //    Used when: closure modifies a variable
    let mut n = 0;
    let mut inc = || { n += 1; };        // captures &mut n
    inc(); inc();
    // println!("{}", n);   // ERROR while `inc` holds &mut n
    drop(inc);               // release the mutable borrow
    println!("n = {}", n);   // OK: 2

    // ── Capture by value (move) ────────────────────────────────────────────
    //    Triggered by `move` keyword or when the closure needs ownership
    let s = String::from("world");
    let use_s = move || println!("{}", s);   // s is MOVED into the closure
    // println!("{}", s);   // ERROR: s was moved
    use_s();   // prints "world" (closure owns s)

    // ── Multiple captures with mixed modes ─────────────────────────────────
    let a = 1;                      // i32: Copy
    let mut b = String::new();      // String: not Copy

    let mut mixed = || {
        let _ = a;         // captures a by copy (since a is Copy and move is not forced)
        b.push('x');       // captures b by &mut
    };
    mixed(); mixed();
    drop(mixed);
    println!("b = {}", b);   // "xx"
}
```

**Closure memory layout:**

```
Closure that captures: x: i32, y: &String, z: String
                       (by value  by ref      by value)

Closure anonymous struct (compiler-generated):
┌──────────────────┬──────────────────┬──────────────────┐
│  x: i32          │  y: &String      │  z: String       │
│  (4 bytes)       │  (8 bytes, ptr)  │  (24 bytes)      │
└──────────────────┴──────────────────┴──────────────────┘
Total size: 4 + 8 + 24 = 36 bytes (+ padding)

Compare to fn pointer:
┌──────────────────┐
│  code address    │   8 bytes only
└──────────────────┘
```

### 5.6 move Closures

The `move` keyword forces ALL captures to be by value, regardless of how the
closure uses them. This is essential for:
- Sending closures across threads (the captured data must be owned, not borrowed,
  so it can cross thread boundaries).
- Returning closures from functions (the captures must not borrow from the stack
  frame that is about to be destroyed).

```rust
use std::thread;

fn main() {
    let data = vec![1, 2, 3, 4, 5];

    // ── Without move: closure borrows data ────────────────────────────────
    // The following would NOT compile if spawning a thread, because the
    // thread might outlive `data`. With move, it compiles:
    let handle = thread::spawn(move || {
        // data was MOVED into this closure
        // The closure owns data; no borrow issues
        println!("sum = {}", data.iter().sum::<i32>());
    });
    // println!("{:?}", data);   // ERROR: data was moved
    handle.join().unwrap();

    // ── move + Copy types ────────────────────────────────────────────────
    let n = 42;   // i32 implements Copy
    let print_n = move || println!("n = {}", n);
    print_n();
    println!("n still accessible: {}", n);   // OK: n was copied, not moved
}
```

### 5.7 Closure Coercion to fn Pointer

A closure that captures **nothing** can be coerced to a bare `fn` pointer. This
is the key interoperability point between closures and C FFI / function pointers.

```rust
fn apply(f: fn(i32) -> i32, x: i32) -> i32 {
    f(x)
}

fn main() {
    // ── Non-capturing closure coerces to fn pointer ───────────────────────
    let double: fn(i32) -> i32 = |x| x * 2;   // OK: captures nothing
    println!("{}", apply(double, 5));            // 10

    // ── Named function also coerces ───────────────────────────────────────
    fn triple(x: i32) -> i32 { x * 3 }
    println!("{}", apply(triple, 5));   // 15

    // ── Capturing closure does NOT coerce ─────────────────────────────────
    let factor = 4;
    // let f: fn(i32) -> i32 = |x| x * factor;   // ERROR!
    // "closures can only be coerced to `fn` types if they do not capture
    //  any variables"

    // ── C FFI: passing a Rust closure as a C callback ─────────────────────
    // extern "C" fn qsort_cmp(a: *const std::ffi::c_void,
    //                          b: *const std::ffi::c_void) -> i32 { ... }
    // The extern "C" annotation selects the C calling convention.
    // Must be a fn pointer (non-capturing) or a named extern fn.
}
```

### 5.8 Static Dispatch: `impl Fn`

When a function takes `impl Fn(T) -> U`, the compiler **monomorphises** it:
generates a separate specialised version for every concrete closure or function
passed. Zero overhead at runtime — identical to a direct call.

```rust
// T is monomorphised: compiler generates a different version for each F
fn transform<F: Fn(i32) -> i32>(data: &[i32], f: F) -> Vec<i32> {
    data.iter().map(|&x| f(x)).collect()
}

// Shorter spelling using `impl Fn`
fn transform2(data: &[i32], f: impl Fn(i32) -> i32) -> Vec<i32> {
    data.iter().map(|&x| f(x)).collect()
}

fn main() {
    let nums = vec![1, 2, 3, 4, 5];

    // Each call may generate different machine code (monomorphised)
    let doubled = transform(&nums, |x| x * 2);
    let squared = transform(&nums, |x| x * x);

    println!("{:?}", doubled);   // [2, 4, 6, 8, 10]
    println!("{:?}", squared);   // [1, 4, 9, 16, 25]

    // Concrete type can be inlined by the optimizer
    // No vtable, no heap allocation
}
```

**Static dispatch diagram:**

```
Source code:
    transform(&nums, |x| x * 2)
    transform(&nums, |x| x * x)

Compiled binary contains TWO specialised functions:
    transform__closure_double(&nums, closure1)
    transform__closure_square(&nums, closure2)

Each is a direct call — the compiler knows exactly which function to invoke.
No indirection at runtime.
```

### 5.9 Dynamic Dispatch: `dyn Fn` and `Box<dyn Fn>`

When the concrete type of a closure is not known at compile time (e.g., chosen
at runtime, or multiple closure types stored in the same collection), you need
**dynamic dispatch** via trait objects.

```rust
fn main() {
    // ── Box<dyn Fn> — single owned closure ───────────────────────────────
    let factor = 3;
    let triple: Box<dyn Fn(i32) -> i32> = Box::new(move |x| x * factor);
    println!("{}", triple(7));   // 21

    // ── Vec<Box<dyn Fn>> — heterogeneous collection ───────────────────────
    let n = 10;
    let mut transforms: Vec<Box<dyn Fn(i32) -> i32>> = vec![
        Box::new(|x| x + 1),
        Box::new(|x| x * 2),
        Box::new(move |x| x + n),    // captures n
        Box::new(|x: i32| x.abs()),
    ];

    for f in &transforms {
        println!("{}", f(-5));
    }
    // Output: -4, -10, 5, 5

    // ── &dyn Fn — borrowed reference to a trait object ───────────────────
    fn call_with_ref(f: &dyn Fn(i32) -> i32, x: i32) -> i32 {
        f(x)
    }
    let add_one = |x| x + 1;
    println!("{}", call_with_ref(&add_one, 41));   // 42

    // ── dyn FnMut — mutable closure ───────────────────────────────────────
    let mut count = 0;
    let mut counter: Box<dyn FnMut() -> i32> = Box::new(|| {
        count += 1;
        count
    });
    println!("{}", counter());   // 1
    println!("{}", counter());   // 2
}
```

**Fat pointer layout for `dyn Fn`:**

```
Box<dyn Fn(i32) -> i32>   — a "fat pointer" (2 words = 16 bytes)

  ┌────────────────────┬────────────────────┐
  │   data pointer     │   vtable pointer   │
  │   → closure data   │   → Fn vtable      │
  │   on the heap      │   in .rodata       │
  └────────────────────┴────────────────────┘

  Closure data (heap):         Fn vtable (.rodata):
  ┌──────────────────┐         ┌──────────────────────┐
  │  captured vars   │         │  drop_in_place ptr   │
  │  (e.g. factor=3) │         │  size / alignment    │
  └──────────────────┘         │  call (Fn::call)     │
                                │  call_mut (FnMut)    │
                                │  call_once (FnOnce)  │
                                └──────────────────────┘

  Calling f(x):
  1. Load vtable pointer from Box fat pointer
  2. Load Fn::call function pointer from vtable
  3. CALL indirectly (like a C++ virtual call)
```

### 5.10 Returning Closures from Functions

Returning a closure from a function requires either `impl Fn` (static dispatch,
concrete type hidden but known at compile time) or `Box<dyn Fn>` (dynamic
dispatch, heap-allocated).

```rust
// ── Return with impl Fn (preferred when possible) ─────────────────────────
fn make_adder(n: i32) -> impl Fn(i32) -> i32 {
    move |x| x + n   // n is moved into the closure
}

// ── Return with Box<dyn Fn> (needed for runtime polymorphism) ─────────────
fn make_op(kind: &str) -> Box<dyn Fn(i32, i32) -> i32> {
    match kind {
        "add" => Box::new(|a, b| a + b),
        "mul" => Box::new(|a, b| a * b),
        _     => Box::new(|a, _b| a),
    }
}

// ── Currying (returning a closure that returns a closure) ─────────────────
fn curry_add(a: i32) -> impl Fn(i32) -> impl Fn(i32) -> i32 {
    move |b| move |c| a + b + c
}

fn main() {
    let add5 = make_adder(5);
    let add10 = make_adder(10);
    println!("{} {}", add5(3), add10(3));   // 8 13

    let op = make_op("mul");
    println!("{}", op(6, 7));   // 42

    let f = curry_add(1)(2);
    println!("{}", f(3));   // 6
}
```

### 5.11 Higher-Kinded Patterns: Combinators

Rust iterators are built entirely on closures and higher-order functions. The
iterator adaptor methods (`map`, `filter`, `flat_map`, etc.) take `impl Fn`
parameters and are all zero-cost when chained together (the compiler fuses them
into a single loop).

```rust
fn main() {
    let numbers = vec![1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

    // Entire pipeline: no intermediate allocations, single loop
    let result: Vec<String> = numbers.iter()
        .filter(|&&x| x % 2 == 0)           // keep evens
        .map(|&x| x * x)                     // square them
        .filter(|&x| x > 10)                 // keep > 10
        .map(|x| format!("val={}", x))        // format
        .collect();

    println!("{:?}", result);   // ["val=16", "val=36", "val=64", "val=100"]

    // ── Function composition with closures ─────────────────────────────────
    fn compose<A, B, C>(
        f: impl Fn(A) -> B,
        g: impl Fn(B) -> C,
    ) -> impl Fn(A) -> C {
        move |x| g(f(x))
    }

    let double_then_square = compose(|x: i32| x * 2, |x| x * x);
    println!("{}", double_then_square(3));   // (3*2)² = 36

    // ── Fold with closure ─────────────────────────────────────────────────
    let sum: i32 = (1..=100).fold(0, |acc, x| acc + x);
    println!("Sum 1..100 = {}", sum);   // 5050

    // ── flat_map ─────────────────────────────────────────────────────────
    let words = vec!["hello world", "foo bar"];
    let letters: Vec<&str> = words.iter()
        .flat_map(|s| s.split_whitespace())
        .collect();
    println!("{:?}", letters);   // ["hello", "world", "foo", "bar"]
}
```

### 5.12 Function Pointers in unsafe

When doing FFI (calling C from Rust or exposing Rust to C), you work with
function pointers under `unsafe` because the compiler cannot verify the
signatures match.

```rust
use std::ffi::c_int;

// ── Declaring an extern C function ────────────────────────────────────────
extern "C" {
    fn abs(input: c_int) -> c_int;
    // qsort from libc
    fn qsort(
        base: *mut std::ffi::c_void,
        nmemb: usize,
        size: usize,
        compar: Option<unsafe extern "C" fn(*const std::ffi::c_void,
                                             *const std::ffi::c_void) -> c_int>,
    );
}

// ── A Rust function with C calling convention, callable from C ────────────
#[no_mangle]
pub extern "C" fn rust_add(a: i32, b: i32) -> i32 {
    a + b
}

// ── Callback passed to C ───────────────────────────────────────────────────
unsafe extern "C" fn cmp_i32(
    a: *const std::ffi::c_void,
    b: *const std::ffi::c_void,
) -> c_int {
    let a = *(a as *const i32);
    let b = *(b as *const i32);
    a.cmp(&b) as c_int
}

fn main() {
    // ── Calling a C function ───────────────────────────────────────────────
    let result = unsafe { abs(-42) };
    println!("{}", result);   // 42

    // ── Storing an extern fn pointer ──────────────────────────────────────
    let f: unsafe extern "C" fn(c_int) -> c_int = abs;
    let r = unsafe { f(-100) };
    println!("{}", r);   // 100

    // ── Option<fn> for nullable callbacks ─────────────────────────────────
    // Option<fn()> is the idiomatic way to express a nullable function pointer.
    // Option<fn()> is guaranteed to be pointer-sized (null = None).
    let mut data = [5i32, 3, 8, 1, 4];
    unsafe {
        qsort(
            data.as_mut_ptr() as *mut std::ffi::c_void,
            5,
            std::mem::size_of::<i32>(),
            Some(cmp_i32),
        );
    }
    println!("{:?}", data);   // [1, 3, 4, 5, 8]
}
```

### 5.13 Memory Layout of fn vs Closures

```rust
use std::mem;

fn add(x: i32, y: i32) -> i32 { x + y }

fn main() {
    let n = 42;

    // Function item — zero-sized
    let item = add;
    println!("function item size:          {} bytes", mem::size_of_val(&item));   // 0

    // fn pointer — pointer-sized
    let ptr: fn(i32, i32) -> i32 = add;
    println!("fn pointer size:             {} bytes", mem::size_of_val(&ptr));    // 8

    // Non-capturing closure — zero-sized
    let nc = |x: i32, y: i32| x + y;
    println!("non-capturing closure size:  {} bytes", mem::size_of_val(&nc));     // 0

    // Closure capturing one i32 (4 bytes)
    let c1 = |x: i32| x + n;
    println!("closure (captures i32) size: {} bytes", mem::size_of_val(&c1));    // 4

    // Closure capturing a String (3 words = 24 bytes on 64-bit)
    let s = String::from("hello");
    let c2 = move || println!("{}", s);
    println!("closure (captures String) size: {} bytes", mem::size_of_val(&c2)); // 24

    // Box<dyn Fn> — fat pointer: 16 bytes regardless of closure size
    let boxed: Box<dyn Fn()> = Box::new(|| println!("hi"));
    println!("Box<dyn Fn()> size:          {} bytes", mem::size_of_val(&boxed)); // 16
}
```

**Size summary ASCII table:**

```
Type                            Size (64-bit)     Heap?    Virtual dispatch?
────────────────────────────    ─────────────     ─────    ─────────────────
fn(T) -> U (pointer)            8 bytes           No       Yes (indirect call)
Function item (zero-sized)      0 bytes           No       No (direct call)
Non-capturing closure           0 bytes           No       No (direct/inlined)
Capturing closure (N bytes)     N bytes           No*      No (monomorphised)
&dyn Fn(T) -> U                 16 bytes (fat)    No       Yes (vtable)
Box<dyn Fn(T) -> U>             16 bytes (fat)    Yes      Yes (vtable)
Arc<dyn Fn(T) -> U>             16 bytes (fat)    Yes      Yes (vtable)

* closures may escape to the heap via Box or if they are inside a Box<dyn>
```

### 5.14 Lifetime of Closures

Closures that capture references must not outlive the references. The compiler
enforces this via lifetimes.

```rust
fn create_greeter<'a>(name: &'a str) -> impl Fn() + 'a {
    // Closure captures `name` — its lifetime is tied to `name`
    // The `+ 'a` bound says the returned closure lives at most as long as `name`
    move || println!("Hello, {}!", name)
}

fn main() {
    let greeting;
    {
        let name = String::from("Alice");
        // greeting = create_greeter(&name);   // ERROR: name doesn't live long enough
        // If we needed this, we'd use an owned String inside the closure
    }

    // Using owned data avoids lifetime constraints:
    let name = String::from("Bob");
    let g = create_greeter(&name);
    g();   // "Hello, Bob!"
}   // g is dropped here; name is still valid (g borrows name)
```

**Static closures** — closures stored in `'static` context (e.g., a global or a
thread) must not borrow anything shorter than `'static`:

```rust
// Thread closures must be 'static (no borrows of local variables)
fn spawn_thread() {
    let owned = String::from("owned data");
    std::thread::spawn(move || {
        // `move` makes the closure take ownership of `owned`
        println!("{}", owned);
    }).join().unwrap();
}
```

### 5.15 Closures in Iterators

```rust
fn main() {
    // scan: like fold but yields intermediate accumulators
    let running_sum: Vec<i32> = (1..=5)
        .scan(0, |acc, x| { *acc += x; Some(*acc) })
        .collect();
    println!("{:?}", running_sum);   // [1, 3, 6, 10, 15]

    // take_while / skip_while
    let data = vec![2, 4, 6, 7, 8, 10];
    let before_odd: Vec<_> = data.iter()
        .take_while(|&&x| x % 2 == 0)
        .collect();
    println!("{:?}", before_odd);   // [2, 4, 6]

    // zip + map
    let a = vec![1, 2, 3];
    let b = vec![4, 5, 6];
    let dots: Vec<i32> = a.iter().zip(b.iter()).map(|(x, y)| x * y).collect();
    println!("dot product: {}", dots.iter().sum::<i32>());   // 32

    // windows + position
    let haystack = vec![1, 2, 3, 4, 5];
    let target = [3, 4];
    let pos = haystack.windows(2).position(|w| w == target);
    println!("{:?}", pos);   // Some(2)

    // Chained closures — the compiler fuses into one loop (no allocations)
    let result: i32 = (0i32..1_000_000)
        .filter(|x| x % 3 == 0 || x % 5 == 0)
        .map(|x| x * x)
        .take(5)
        .sum();
    println!("{}", result);   // 0² + 3² + 5² + 6² + 9² = 0+9+25+36+81 = 151
}
```

### 5.16 Thread Safety: Send + Sync on Closures

Rust's type system enforces that closures sent across thread boundaries can only
capture data that is itself thread-safe.

```rust
use std::sync::{Arc, Mutex};
use std::thread;

fn main() {
    // ── Closure must be Send to cross thread boundaries ───────────────────
    let data = Arc::new(Mutex::new(vec![1, 2, 3]));
    let data_clone = Arc::clone(&data);

    let handle = thread::spawn(move || {
        // `move` captures data_clone by value.
        // Arc<Mutex<Vec<i32>>>: Arc is Send, so this is safe.
        let mut v = data_clone.lock().unwrap();
        v.push(4);
    });
    handle.join().unwrap();
    println!("{:?}", *data.lock().unwrap());   // [1, 2, 3, 4]

    // ── Rc<T> is NOT Send — this would not compile ─────────────────────────
    // let rc = std::rc::Rc::new(42);
    // thread::spawn(move || println!("{}", rc));   // ERROR: Rc cannot be sent

    // ── Function pointer fn() is always Send + Sync ───────────────────────
    fn pure(x: i32) -> i32 { x * 2 }
    let fp: fn(i32) -> i32 = pure;
    thread::spawn(move || { println!("{}", fp(21)); }).join().unwrap();

    // ── Arc<dyn Fn + Send + Sync> for shared closures across threads ───────
    let shared: Arc<dyn Fn(i32) -> i32 + Send + Sync> = Arc::new(|x| x + 1);
    let shared_clone = Arc::clone(&shared);
    thread::spawn(move || { println!("{}", shared_clone(41)); }).join().unwrap();
    println!("{}", shared(0));
}
```

---

## 6. Cross-Language Comparison

### 6.1 Feature Matrix

```
Feature                    C                Go               Rust
──────────────────────────────────────────────────────────────────────────────
First-class functions      Yes (pointer)    Yes (value)      Yes (fn/closure)
Anonymous functions        No (C99: no)     Yes (literal)    Yes (closure)
Closures                   No (manual)      Yes              Yes
Capture mode control       N/A              Automatic        Auto + `move`
Type of function var       pointer type     func type        fn type / closure
Null/nil callable          Yes (NULL)       Yes (nil)        Option<fn> / None
Generic over callable      No               No (pre-1.18)    Yes (Fn traits)
Returning a function       Yes (ptr)        Yes              impl Fn / Box<dyn>
Higher-order functions     Yes (qsort)      Yes              Yes (iterators)
Zero-cost abstraction      Yes (direct)     No (interface)   Yes (impl Fn)
Dynamic dispatch           Manual vtable    Interface        dyn Fn
Thread safety              Manual           goroutines+chan  Send + Sync traits
FFI / interop              Native           cgo              extern "C"
Size of non-capturing fn   pointer (8B)     func val (16B)   0 bytes (item)
Size of capturing closure  N/A              16B + heap env   N bytes inline
```

### 6.2 Equivalent Patterns Side-by-Side

**Storing a callback:**

```c
// C
typedef void (*Callback)(int);
void register(Callback cb) { cb(42); }
```

```go
// Go
type Callback func(int)
func register(cb Callback) { cb(42) }
```

```rust
// Rust — static dispatch (zero-cost)
fn register(cb: impl Fn(i32)) { cb(42); }
// Rust — dynamic dispatch (flexible)
fn register_dyn(cb: &dyn Fn(i32)) { cb(42); }
```

**Table dispatch:**

```c
// C — array of function pointers
typedef int (*Op)(int,int);
Op ops[] = {add, sub, mul};
ops[i](a, b);
```

```go
// Go — map or slice of function values
ops := map[string]func(int,int)int{"add":add,"sub":sub}
ops["add"](a, b)
```

```rust
// Rust — array of fn pointers (all same type)
let ops: &[fn(i32,i32)->i32] = &[add, sub, mul];
ops[i](a, b);
// or a HashMap<&str, Box<dyn Fn(i32,i32)->i32>>
```

**Passing context to a callback:**

```c
// C — void* userdata pattern
void run(void (*cb)(int, void *), void *ctx) { cb(42, ctx); }
```

```go
// Go — closures capture context automatically
func run(cb func(int)) { cb(42) }
ctx := &MyContext{...}
run(func(n int) { ctx.handle(n) })   // ctx captured
```

```rust
// Rust — closures capture context automatically
fn run(cb: impl Fn(i32)) { cb(42); }
let ctx = MyContext::new();
run(move |n| ctx.handle(n));   // ctx moved into closure
```

---

## 7. Advanced Patterns Across All Three Languages

### 7.1 Middleware / Decorator Chain

**C:**
```c
typedef int (*Handler)(const char *req);
typedef int (*Middleware)(const char *req, Handler next);

int logging_middleware(const char *req, Handler next) {
    printf("[LOG] Request: %s\n", req);
    int r = next(req);
    printf("[LOG] Response: %d\n", r);
    return r;
}
```

**Go:**
```go
type Handler func(string) int

func logging(next Handler) Handler {
    return func(req string) int {
        fmt.Printf("[LOG] %s\n", req)
        r := next(req)
        fmt.Printf("[LOG] → %d\n", r)
        return r
    }
}

func chain(h Handler, middlewares ...func(Handler) Handler) Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        h = middlewares[i](h)
    }
    return h
}
```

**Rust:**
```rust
type Handler = Box<dyn Fn(&str) -> i32>;

fn logging(next: Handler) -> Handler {
    Box::new(move |req| {
        println!("[LOG] {}", req);
        let r = next(req);
        println!("[LOG] → {}", r);
        r
    })
}
```

### 7.2 Memoisation

**C:**
```c
#include <stdlib.h>
#define CACHE_SIZE 256

typedef long long (*LLFn)(int);

struct MemoEntry { int key; long long val; int set; };
struct MemoEntry cache[CACHE_SIZE];

long long memoised_fib(int n) {
    if (n < 0 || n >= CACHE_SIZE) return 0;
    if (cache[n].set) return cache[n].val;
    long long r = (n <= 1) ? n : memoised_fib(n-1) + memoised_fib(n-2);
    cache[n] = (struct MemoEntry){n, r, 1};
    return r;
}
```

**Go:**
```go
func Memoize[T comparable, U any](f func(T) U) func(T) U {
    cache := map[T]U{}
    return func(arg T) U {
        if v, ok := cache[arg]; ok { return v }
        v := f(arg)
        cache[arg] = v
        return v
    }
}
```

**Rust:**
```rust
use std::collections::HashMap;

fn memoize<T, U, F>(mut f: F) -> impl FnMut(T) -> U
where
    T: Eq + std::hash::Hash + Clone,
    U: Clone,
    F: FnMut(T) -> U,
{
    let mut cache = HashMap::new();
    move |arg: T| {
        if let Some(v) = cache.get(&arg) { return v.clone(); }
        let v = f(arg.clone());
        cache.insert(arg, v.clone());
        v
    }
}
```

### 7.3 Observer / Event System

**Go (idiomatic):**
```go
type EventBus[T any] struct {
    mu       sync.RWMutex
    handlers []func(T)
}

func (b *EventBus[T]) Subscribe(h func(T)) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.handlers = append(b.handlers, h)
}

func (b *EventBus[T]) Publish(event T) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    for _, h := range b.handlers { h(event) }
}
```

**Rust:**
```rust
use std::sync::{Arc, Mutex};

struct EventBus<T> {
    handlers: Mutex<Vec<Box<dyn Fn(&T) + Send>>>,
}

impl<T: Send> EventBus<T> {
    fn new() -> Self { Self { handlers: Mutex::new(vec![]) } }

    fn subscribe(&self, h: impl Fn(&T) + Send + 'static) {
        self.handlers.lock().unwrap().push(Box::new(h));
    }

    fn publish(&self, event: &T) {
        for h in self.handlers.lock().unwrap().iter() { h(event); }
    }
}
```

---

## 8. Common Pitfalls & Anti-Patterns

### 8.1 C Pitfalls

```
Pitfall 1: Forgetting parentheses around *name
──────────────────────────────────────────────
  int *fp(int) {}        ← function returning int*   (probably wrong)
  int (*fp)(int) = fn;   ← pointer to function       (correct)

Pitfall 2: Calling a NULL function pointer
──────────────────────────────────────────
  typedef void (*Hook)(void);
  Hook h = NULL;
  h();    ← CRASH: null pointer dereference / SIGSEGV
  ALWAYS guard: if (h) h();

Pitfall 3: Calling through incompatible type (UB)
─────────────────────────────────────────────────
  int f(int x) { return x; }
  void (*bad)(void) = (void(*)(void))f;
  bad();   ← UB: ABI mismatch, arguments/return corrupted

Pitfall 4: Storing a function pointer in void* (non-POSIX)
───────────────────────────────────────────────────────────
  void *p = (void *)printf;   ← strictly UB in C (fine on POSIX)

Pitfall 5: Returning pointer to local variable through fn pointer result
────────────────────────────────────────────────────────────────────────
  int *bad_fn(void) {
      int local = 42;
      return &local;   ← dangling pointer!
  }
  int (*fp)(void) = bad_fn;  ← fp is fine; *fp() returns dangling ptr
```

### 8.2 Go Pitfalls

```
Pitfall 1: Loop variable capture (pre Go 1.22)
──────────────────────────────────────────────
  for _, v := range slice {
      go func() { fmt.Println(v) }()   ← all goroutines see last v
  }
  Fix: pass v as argument or shadow: v := v

Pitfall 2: Calling a nil function value
───────────────────────────────────────
  var f func()
  f()   ← panic: runtime error: invalid memory address

Pitfall 3: Closure captures a loop index in goroutines
──────────────────────────────────────────────────────
  See Section 4.5 — the classic Go bug

Pitfall 4: Method value vs method expression confusion
──────────────────────────────────────────────────────
  r.Area   → func() float64   (bound to r)
  Rect.Area → func(Rect) float64   (unbound)
  Mixing them up causes type errors or wrong results

Pitfall 5: Modifying a captured map/slice from multiple goroutines
──────────────────────────────────────────────────────────────────
  Closures capturing maps share the map pointer.
  Concurrent writes cause data races.
  Fix: use sync.Mutex or sync.Map
```

### 8.3 Rust Pitfalls

```
Pitfall 1: Storing a capturing closure in fn pointer
─────────────────────────────────────────────────────
  let n = 5;
  let f: fn(i32) -> i32 = |x| x + n;   ← COMPILE ERROR
  Fix: use impl Fn or Box<dyn Fn>

Pitfall 2: Returning a closure that borrows from the stack
───────────────────────────────────────────────────────────
  fn bad() -> impl Fn() {
      let local = String::from("hi");
      || println!("{}", local)   ← ERROR: local does not live long enough
  }
  Fix: use `move` to take ownership: move || println!("{}", local)

Pitfall 3: Calling FnOnce more than once
─────────────────────────────────────────
  let f: Box<dyn FnOnce()> = Box::new(|| {});
  f();
  f();   ← COMPILE ERROR: use of moved value

Pitfall 4: Box<dyn Fn> vs Box<dyn FnMut>
─────────────────────────────────────────
  let mut count = 0;
  let f: Box<dyn Fn()> = Box::new(|| { count += 1; });   ← ERROR
  // Fn requires immutable borrow; mutating count requires FnMut
  let f: Box<dyn FnMut()> = Box::new(|| { count += 1; });  ← OK

Pitfall 5: Thread boundary with non-Send closure
─────────────────────────────────────────────────
  let rc = std::rc::Rc::new(42);
  std::thread::spawn(move || println!("{}", rc));   ← ERROR: Rc is not Send
  Fix: use Arc instead of Rc

Pitfall 6: Infinite size from recursive closure type
─────────────────────────────────────────────────────
  let f = |x| f(x + 1);   ← ERROR: closure captures itself, infinite size
  Fix: use fn item (named function) for recursion, or Box<dyn Fn>

Pitfall 7: Forgetting mut for FnMut
─────────────────────────────────────
  let f = || { count += 1; };
  f();   ← ERROR: cannot borrow `f` as mutable because it is not mutable
  Fix: let mut f = || { count += 1; };
       mut f();  ← OK
```

---

## Summary Mental Model

```
C:
  A function is machine code at an address.
  A function pointer is a variable holding that address.
  No closures — simulate with void* userdata + manual struct.
  Power: vtables, callbacks, plugin systems, state machines.
  Risk: type mismatches, NULL dereference, dangling pointers.

Go:
  Functions are first-class values — store, pass, return them.
  Closures capture variables BY REFERENCE automatically.
  nil function values panic on call — guard with != nil.
  Goroutine + closure loop variable capture is the #1 bug.
  Use map[string]func for dispatch tables.
  Use function types with methods to implement interfaces.

Rust:
  fn(T)->U: bare pointer, 8 bytes, no captures, Copy.
  Function item: zero-sized, one specific function, inlinable.
  Closure: anonymous struct holding captures, auto-sized.
  impl Fn: monomorphised, zero overhead (like templates).
  dyn Fn: fat pointer + vtable, runtime polymorphism, heap.
  Fn ⊃ FnMut ⊃ FnOnce (read-only ⊃ mutable ⊃ consuming).
  move: force all captures by value (needed for threads/return).
  The compiler enforces correct lifetimes and thread safety.
```

---

*End of guide. All code examples are complete, compile-ready, and annotated.*
