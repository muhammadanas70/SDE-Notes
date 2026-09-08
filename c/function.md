# The Complete Guide to Functions: Rust, C & Go
## Deep Understanding for Top 1% Programmers

---

## TABLE OF CONTENTS

1. [Foundation: What is a Function?](#1-foundation-what-is-a-function)
2. [Core Anatomy of a Function](#2-core-anatomy-of-a-function)
3. [Functions in C — Complete Guide](#3-functions-in-c)
   - 3.1 Basic / Regular Functions
   - 3.2 Function Prototypes
   - 3.3 Void Functions
   - 3.4 Static Functions
   - 3.5 Extern Functions
   - 3.6 Recursive Functions
   - 3.7 Inline Functions
   - 3.8 Variadic Functions
   - 3.9 Function Pointers & Callbacks
   - 3.10 `_Noreturn` / Diverging Functions
   - 3.11 C Function Type Map
4. [Functions in Rust — Complete Guide](#4-functions-in-rust)
   - 4.1 Regular Functions & Expression Model
   - 4.2 Methods & Associated Functions
   - 4.3 Closures
   - 4.4 Higher-Order Functions
   - 4.5 Generic Functions
   - 4.6 Const Functions
   - 4.7 Async Functions
   - 4.8 Unsafe Functions
   - 4.9 Diverging Functions (`-> !`)
   - 4.10 Function Pointers
   - 4.11 `impl Trait` in Functions
   - 4.12 Extern Functions (FFI)
   - 4.13 Rust Function Type Map
5. [Functions in Go — Complete Guide](#5-functions-in-go)
   - 5.1 Regular Functions
   - 5.2 Methods (Value & Pointer Receivers)
   - 5.3 Anonymous Functions & Closures
   - 5.4 Higher-Order Functions
   - 5.5 Variadic Functions
   - 5.6 Recursive Functions
   - 5.7 Defer Functions
   - 5.8 `init()` Functions
   - 5.9 Function Types & Interface Adapters
   - 5.10 Goroutine Functions
   - 5.11 Go Function Type Map
6. [Cross-Language Comparison](#6-cross-language-comparison)
7. [Expert Mental Models](#7-expert-mental-models)

---

## 1. FOUNDATION: WHAT IS A FUNCTION?

A **function** is a named, reusable block of code that:

- Takes zero or more **inputs** (parameters)
- Performs a specific **computation or action**
- Returns zero or more **outputs** (return values)
- Has a defined **scope** and **lifetime**

### Mathematical Origin

Functions come from mathematics: `f(x) = x²`

```
f    →  name of the function
x    →  input (parameter / argument)
x²   →  output (return value / result)
```

In programming, functions are the **fundamental unit of abstraction**.
Everything reduces to functions calling functions.

```
                    ┌──────────────────────────────────┐
                    │            FUNCTION               │
                    │                                   │
  Input(s)  ───────►│  [Logic / Computation /           │───────►  Output(s)
 (Arguments)        │   Side Effects / Both]            │         (Return Values)
                    │                                   │
                    └──────────────────────────────────┘
```

### Why Functions Exist — Core Principles

```
  ┌──────────────────────────────────────────────────────────┐
  │                  WHY FUNCTIONS MATTER                     │
  │                                                           │
  │  DRY Principle ─────► Don't Repeat Yourself              │
  │  Abstraction   ─────► Hide complexity behind a name      │
  │  Modularity    ─────► Break large problems into pieces   │
  │  Testability   ─────► Test small, isolated units         │
  │  Readability   ─────► Code tells a story                 │
  │  Reusability   ─────► Write once, use anywhere           │
  │  Encapsulation ─────► Bundle data + behavior together    │
  └──────────────────────────────────────────────────────────┘
```

---

## 2. CORE ANATOMY OF A FUNCTION

Every function, in every language, has these core components:

```
┌─────────────────────────────────────────────────────────────────────┐
│                        FUNCTION ANATOMY                              │
│                                                                       │
│  [modifier(s)]   name   ( parameters )   -> return_type   { body }  │
│       │           │           │                 │              │      │
│       │           │           │                 │              │      │
│   visibility    what    what goes in        what comes    computation │
│   storage       it's    (inputs with        out (type     and logic   │
│   lifetime      called  their types)        of result)               │
└─────────────────────────────────────────────────────────────────────┘
```

### Essential Terminology

Before going further, every term below will appear in code. Know these cold:

| Term              | Meaning                                                             |
|-------------------|---------------------------------------------------------------------|
| **Parameter**     | Variable declared in the function signature (the placeholder)       |
| **Argument**      | Actual value passed when calling the function                       |
| **Signature**     | Function name + parameter types + return type                       |
| **Body**          | The block of code inside `{ ... }`                                  |
| **Prototype**     | Signature only — no body. Tells the compiler the function exists   |
| **Definition**    | Complete function with body                                         |
| **Call/Invoke**   | Executing the function at a call site                               |
| **Stack Frame**   | Region of memory allocated for one function call                    |
| **Return Value**  | Data sent back to the caller                                        |
| **Side Effect**   | Any change outside the function's own scope (printing, writing...)  |
| **Scope**         | Region of code where a name is visible / valid                      |
| **Lifetime**      | How long a value or reference remains valid in memory               |

### Call Stack Architecture

```
                     CALL STACK (grows downward)
High Memory
┌──────────────────────────────┐
│         main() frame          │
│   local variables of main    │
│   [return address → OS]      │
├──────────────────────────────┤  ◄── Stack Pointer moves here
│          foo() frame          │     when foo() is called
│   a, b  (parameters)         │
│   result (local var)         │
│   [return address → main]    │
├──────────────────────────────┤  ◄── Stack Pointer moves here
│          bar() frame          │     when bar() is called
│   x (parameter)              │
│   [return address → foo]     │
└──────────────────────────────┘
Low Memory

LIFECYCLE:
  1. Caller pushes arguments onto stack
  2. CPU jumps to function address (call instruction)
  3. Function creates its frame (prologue)
  4. Function executes body
  5. Return value placed in register or stack
  6. Frame is destroyed (epilogue)
  7. CPU jumps back to caller (ret instruction)
```

---

## 3. FUNCTIONS IN C — COMPLETE GUIDE

C's function model is beautifully simple — no namespaces, no overloading, no classes.
Everything is flat. Understanding C functions makes every other language's model trivial.

---

### 3.1 Basic / Regular Functions

The most fundamental type. Input → computation → output.

**Syntax:**

```
return_type   function_name  ( parameter_list )  {
    // body
    return value;
}
```

```c
/* ─────────────────────────────────────────────────────────
   FILE : basic_functions.c
   PURPOSE : Core regular functions in C
   ───────────────────────────────────────────────────────── */
#include 

/*
 * FUNCTION: add
 * SIGNATURE: int add(int a, int b)
 * PURPOSE: Compute and return the sum of two integers
 * PARAMS:
 *   a — first operand
 *   b — second operand
 * RETURNS: int — the arithmetic sum a + b
 */
int add(int a, int b) {
    return a + b;
}

/*
 * FUNCTION: max_of_three
 * PURPOSE: Return the largest of three integers
 */
int max_of_three(int x, int y, int z) {
    int temp = (x > y) ? x : y;
    return (temp > z) ? temp : z;
}

/*
 * FUNCTION: is_prime
 * PURPOSE: Return 1 if n is prime, 0 otherwise
 * NOTE: Returns int — C has no boolean type in C89
 *       Use  in C99+ for 'bool'
 */
int is_prime(int n) {
    if (n < 2) return 0;
    for (int i = 2; i * i <= n; i++) {
        if (n % i == 0) return 0;
    }
    return 1;
}

int main(void) {
    /* 3 and 4 are ARGUMENTS passed to parameters a and b */
    int result = add(3, 4);
    printf("add(3, 4)            = %d\n", result);            /* 7  */
    printf("max_of_three(3,9,6)  = %d\n", max_of_three(3, 9, 6)); /* 9 */
    printf("is_prime(17)         = %d\n", is_prime(17));      /* 1  */
    printf("is_prime(18)         = %d\n", is_prime(18));      /* 0  */
    return 0;
}
```

**Stack Memory — When `add(3, 4)` is called:**

```
Before call:
┌─────────────────────┐
│    main() frame      │
│  result = ???        │
│  [return addr → OS] │
└─────────────────────┘

During call (add's frame pushed):
┌─────────────────────┐
│    main() frame      │
│  result = ???        │
│  [return addr → OS] │
├─────────────────────┤
│    add() frame       │
│  a = 3               │  ◄── copied from argument 3
│  b = 4               │  ◄── copied from argument 4
│  [return value: 7]   │
│  [return addr → main]│
└─────────────────────┘

After return:
┌─────────────────────┐
│    main() frame      │
│  result = 7          │  ◄── return value stored here
│  [return addr → OS] │
└─────────────────────┘
  add()'s frame is GONE — automatically destroyed
```

---

### 3.2 Function Prototypes (Forward Declarations)

The C compiler reads source code **top-to-bottom**. If you call a function before its
definition appears, the compiler has no idea what types to expect. A **prototype** solves
this — it's a declaration (signature only, no body) placed before the first call.

```
What is a Prototype?
──────────────────────────────────────────────────────────────
A prototype = function signature + semicolon (no body)

DECLARATION:   int multiply(int x, int y);
DEFINITION:    int multiply(int x, int y) { return x * y; }

The prototype is a PROMISE to the compiler:
"This function exists somewhere — here is its contract."
The linker resolves the actual address later.
```

```c
/* ─────────────────────────────────────────────────────────
   FILE : prototypes.c
   PURPOSE : Function prototypes and forward declarations
   ───────────────────────────────────────────────────────── */
#include 

/* ── PROTOTYPES — declared BEFORE main() ────────────────── */
/* Note: parameter NAMES are optional in prototypes.         */
/* int multiply(int, int);  is equally valid.                */
int   multiply(int x, int y);
float average(float *arr, int len);
void  print_array(int *arr, int n);

/* main() can now call these safely — prototypes are known */
int main(void) {
    printf("multiply(6, 7) = %d\n", multiply(6, 7));  /* 42 */

    float data[] = {2.0f, 4.0f, 6.0f, 8.0f, 10.0f};
    printf("average        = %.2f\n", average(data, 5)); /* 6.00 */

    int nums[] = {5, 2, 8, 1, 9};
    print_array(nums, 5);

    return 0;
}

/* ── DEFINITIONS — after main(), possible because of prototypes */
int multiply(int x, int y) {
    return x * y;
}

float average(float *arr, int len) {
    float sum = 0.0f;
    for (int i = 0; i < len; i++) sum += arr[i];
    return sum / (float)len;
}

void print_array(int *arr, int n) {
    for (int i = 0; i < n; i++) {
        printf("%d%s", arr[i], i < n - 1 ? ", " : "\n");
    }
}
```

---

### 3.3 Void Functions (Procedures)

A **void function** returns nothing (`void`). It exists purely for its **side effects**:
printing, modifying data through pointers, writing files, changing global state.

```
VOID FUNCTION MODEL:
┌──────────────────────────────────────────────────────┐
│  void function                                        │
│                                                        │
│  Input(s) ──► [ Does Something ]                     │
│               (prints, modifies, writes, logs...)    │
│                              │                        │
│                              └──► NO return value     │
│                                   (void = emptiness)  │
└──────────────────────────────────────────────────────┘
```

```c
/* ─────────────────────────────────────────────────────────
   FILE : void_functions.c
   PURPOSE : Void functions — pure side effects
   ───────────────────────────────────────────────────────── */
#include 
#include 

/*
 * Reverses an integer array IN-PLACE.
 * The caller's array is modified via the pointer.
 * No return — the change IS the output.
 */
void reverse_array(int *arr, int len) {
    int left = 0, right = len - 1;
    while (left < right) {
        int temp    = arr[left];
        arr[left]   = arr[right];
        arr[right]  = temp;
        left++;
        right--;
    }
}

/* Print a visual separator line */
void print_separator(char ch, int count) {
    for (int i = 0; i < count; i++) putchar(ch);
    putchar('\n');
}

/*
 * EARLY RETURN from void function.
 * 'return;'  with no value exits immediately.
 */
void process_positive(int n) {
    if (n <= 0) {
        printf("Skipping non-positive value: %d\n", n);
        return;  /* ◄── Early exit — no value, just exit */
    }
    printf("Processing: %d (squared = %d)\n", n, n * n);
}

int main(void) {
    print_separator('=', 40);

    int data[] = {1, 2, 3, 4, 5};
    int len = 5;

    printf("Before: ");
    for (int i = 0; i < len; i++) printf("%d ", data[i]);
    printf("\n");

    reverse_array(data, len);

    printf("After:  ");
    for (int i = 0; i < len; i++) printf("%d ", data[i]); /* 5 4 3 2 1 */
    printf("\n");

    print_separator('-', 40);

    process_positive(-3); /* Skipping... */
    process_positive(7);  /* Processing: 7 (squared = 49) */
    process_positive(0);  /* Skipping... */

    return 0;
}
```

---

### 3.4 Static Functions (File-Scope / Internal Linkage)

`static` before a function restricts its **visibility** to the current translation unit
(`.c` file). No other file can see or call it — it becomes a private implementation detail.

```
WITHOUT static (external linkage — default):
  file_a.c   file_b.c   file_c.c
  helper()  ◄────────────────────  All files can call helper()
  
WITH static (internal linkage):
  file_a.c
  static helper()     ◄── INVISIBLE to file_b.c and file_c.c
                          Only code in file_a.c can call it

WHY USE STATIC FUNCTIONS?
  1. Avoid name collisions across files
  2. Signal: "this is an internal detail, do not use externally"
  3. Allow compiler optimizations (knows all callers are local)
```

```c
/* ─────────────────────────────────────────────────────────
   FILE : static_functions.c
   PURPOSE : File-private helper functions
   ───────────────────────────────────────────────────────── */
#include 
#include 

/* ── PRIVATE helpers — only usable within THIS file ─────── */

static int clamp(int value, int lo, int hi) {
    if (value < lo) return lo;
    if (value > hi) return hi;
    return value;
}

static double normalize(double val, double lo, double hi) {
    if (hi == lo) return 0.0;
    return (val - lo) / (hi - lo);
}

static int is_in_range(int val, int lo, int hi) {
    return val >= lo && val <= hi;
}

/* ── PUBLIC interface — visible to other files ───────────── */
/* This function calls private helpers — callers never know. */
int safe_percentage(int raw_value) {
    return clamp(raw_value, 0, 100);
}

double scale_temperature(double celsius) {
    /* Normalize 0°C..100°C → 0.0..1.0 */
    double clamped = (double)clamp((int)celsius, 0, 100);
    return normalize(clamped, 0.0, 100.0);
}

int main(void) {
    printf("clamp(150, 0, 100)  = %d\n", clamp(150, 0, 100)); /* 100 */
    printf("clamp(-5,  0, 100)  = %d\n", clamp(-5,  0, 100)); /* 0   */
    printf("clamp(60,  0, 100)  = %d\n", clamp(60,  0, 100)); /* 60  */

    printf("normalize(75) = %.2f\n", normalize(75.0, 0.0, 100.0)); /* 0.75 */

    printf("safe_percentage(150) = %d\n", safe_percentage(150));    /* 100  */
    printf("scale_temp(25)       = %.2f\n", scale_temperature(25)); /* 0.25 */
    return 0;
}
```

---

### 3.5 Extern Functions (Cross-File Linkage)

`extern` is the opposite of `static`. It declares that a function is defined
**in another translation unit** (another `.c` file). The linker resolves the address.

```
EXTERN LINKAGE ARCHITECTURE:

  math_utils.c              main.c
  ────────────              ──────────────────────────────
  int square(int n) {       #include "math_utils.h"
      return n * n;         
  }                         int main(void) {
                                int x = square(5);   /* 25 */
  int cube(int n) {         }
      return n*n*n;
  }

  math_utils.h (shared interface):
  ─────────────────────────────────
  extern int square(int n);   /* declaration only — no body */
  extern int cube(int n);

  LINK STEP:
  gcc main.c math_utils.c -o program
  Linker connects:
    main.c's reference to square() ──► math_utils.c's definition
```

```c
/* ── math_utils.h ────────────────────────────────────────── */
#ifndef MATH_UTILS_H
#define MATH_UTILS_H

/*
 * 'extern' is implicit for non-static functions in headers,
 * but being explicit improves clarity.
 */
extern int    square(int n);
extern int    cube(int n);
extern double hypotenuse(double a, double b);

#endif /* MATH_UTILS_H */

/* ── math_utils.c ────────────────────────────────────────── */
#include "math_utils.h"
#include 

int square(int n) { return n * n; }
int cube(int n)   { return n * n * n; }

double hypotenuse(double a, double b) {
    return sqrt(a * a + b * b);
}

/* ── main.c ──────────────────────────────────────────────── */
#include 
#include "math_utils.h"

int main(void) {
    printf("square(5)        = %d\n",   square(5));       /* 25   */
    printf("cube(3)          = %d\n",   cube(3));         /* 27   */
    printf("hypotenuse(3, 4) = %.1f\n", hypotenuse(3, 4)); /* 5.0 */
    return 0;
}
```

---

### 3.6 Recursive Functions

A function that **calls itself**. Two absolute requirements:

1. **Base case** — condition that stops the recursion (returns without calling itself)
2. **Recursive case** — simplifies the problem and calls itself

```
RECURSION EXECUTION MODEL — factorial(4):

  factorial(4)
      └──► 4 * factorial(3)
                  └──► 3 * factorial(2)
                              └──► 2 * factorial(1)
                                          └──► 1 * factorial(0)
                                                        └──► BASE: return 1

  WINDING (building the stack):          UNWINDING (returning values):
  factorial(0) = BASE = 1
  factorial(1) = 1 * 1         = 1
  factorial(2) = 2 * 1         = 2
  factorial(3) = 3 * 2         = 6
  factorial(4) = 4 * 6         = 24
  
STACK DEPTH during factorial(4):
  [ main ]
  [ factorial(4) ]
  [ factorial(3) ]
  [ factorial(2) ]
  [ factorial(1) ]
  [ factorial(0) ]  ◄── deepest point: 5 extra frames
  Space: O(n) stack frames
```

```c
/* ─────────────────────────────────────────────────────────
   FILE : recursive_functions.c
   PURPOSE : Recursion patterns — linear, tail, mutual
   ───────────────────────────────────────────────────────── */
#include 

/* ── 1. LINEAR RECURSION ─────────────────────────────────── */
/* Each call makes exactly one recursive call.               */
/* Time: O(n)   Space: O(n) stack frames                    */
long long factorial(int n) {
    if (n <= 0) return 1;                        /* BASE CASE */
    return (long long)n * factorial(n - 1);      /* RECURSIVE */
}

/* ── 2. TAIL RECURSION ───────────────────────────────────── */
/*
 * "Tail call" = recursive call is the LAST operation.
 * The compiler can optimize this into a loop (no frame growth).
 * This is Tail Call Optimization (TCO).
 * In C, use -O2 flag — GCC/Clang may apply TCO automatically.
 */
long long factorial_tail(int n, long long acc) {
    if (n <= 0) return acc;                         /* BASE */
    return factorial_tail(n - 1, (long long)n * acc); /* TAIL */
}

/* ── 3. TREE RECURSION ───────────────────────────────────── */
/* Each call spawns multiple recursive calls — exponential.  */
/* Time: O(2^n)   Space: O(n) — tree depth                  */
int fibonacci_naive(int n) {
    if (n <= 0) return 0;   /* BASE */
    if (n == 1) return 1;   /* BASE */
    return fibonacci_naive(n - 1) + fibonacci_naive(n - 2); /* two calls */
}

/* ── 4. FIBONACCI with MEMOIZATION (better) ─────────────── */
/* Cache computed results to avoid redundant calls.           */
/* Time: O(n)  Space: O(n)                                   */
static long long memo[100] = {0};
long long fibonacci_memo(int n) {
    if (n <= 0) return 0;
    if (n == 1) return 1;
    if (memo[n]) return memo[n];           /* Return cached value */
    memo[n] = fibonacci_memo(n-1) + fibonacci_memo(n-2);
    return memo[n];
}

/* ── 5. MUTUAL RECURSION ─────────────────────────────────── */
/* Two functions that call each other. Requires prototype.   */
int is_odd(int n);  /* prototype — defined below */

int is_even(int n) {
    if (n == 0) return 1;       /* 0 is even */
    return is_odd(n - 1);
}

int is_odd(int n) {
    if (n == 0) return 0;       /* 0 is not odd */
    return is_even(n - 1);
}

/* ── 6. RECURSIVE BINARY SEARCH ─────────────────────────── */
/* Divides search space in half each call.                   */
/* Time: O(log n)  Space: O(log n) stack depth              */
int binary_search(int *arr, int target, int lo, int hi) {
    if (lo > hi) return -1;              /* BASE: not found */
    int mid = lo + (hi - lo) / 2;       /* Avoid overflow */
    if (arr[mid] == target) return mid;  /* BASE: found */
    if (arr[mid] < target)
        return binary_search(arr, target, mid + 1, hi);
    else
        return binary_search(arr, target, lo, mid - 1);
}

int main(void) {
    printf("factorial(10)      = %lld\n", factorial(10));           /* 3628800 */
    printf("factorial_tail(10) = %lld\n", factorial_tail(10, 1));   /* 3628800 */
    printf("fibonacci_memo(40) = %lld\n", fibonacci_memo(40));      /* 102334155 */
    printf("is_even(12)        = %d\n",   is_even(12));             /* 1 */
    printf("is_odd(7)          = %d\n",   is_odd(7));               /* 1 */

    int arr[] = {1, 3, 5, 7, 9, 11, 13, 15};
    printf("binary_search(7)   = %d\n", binary_search(arr, 7, 0, 7));  /* 3 */
    printf("binary_search(6)   = %d\n", binary_search(arr, 6, 0, 7));  /* -1 */
    return 0;
}
```

---

### 3.7 Inline Functions

`inline` is a **hint to the compiler** to copy the function body at the call site,
eliminating the overhead of a real function call. For tiny, frequently-called functions.

```
WITHOUT inline:                     WITH inline (conceptually):
────────────────────────────────    ──────────────────────────────────
main() {                            main() {
    result = square(5);                 /* compiler inserts body here: */
    /* actual machine instructions: */ int _t = 5;
    push 5        (arg setup)           result = _t * _t;  /* = 25 */
    call square   (jump)                /* NO call, NO stack frame */
    pop result    (return)
}                                   }

square():
    mov eax, [arg]
    imul eax, eax
    ret

BENEFIT: No call overhead (no push/pop, no jump, no stack frame)
COST:    Binary size grows if heavily used (code is duplicated)
RULE:    Inline ONLY tiny functions (1-5 lines, hot path)
```

```c
/* ─────────────────────────────────────────────────────────
   FILE : inline_functions.c
   PURPOSE : Inline functions for zero-overhead primitives
   ───────────────────────────────────────────────────────── */
#include 

/* 'static inline' = private to file AND hints compiler to inline */
static inline int square(int n)           { return n * n;   }
static inline int cube(int n)             { return n*n*n;   }
static inline int max_int(int a, int b)   { return a>b?a:b; }
static inline int min_int(int a, int b)   { return a<b?a:b; }
static inline int abs_int(int n)          { return n<0?-n:n; }
static inline int clamp_int(int v, int lo, int hi) {
    return min_int(max_int(v, lo), hi);
}

/* GCC/Clang extension: force inlining regardless of optimization level */
__attribute__((always_inline))
static inline long long fast_pow2(int n) {
    return 1LL << n;  /* 2^n via bit shift */
}

/* __attribute__((noinline)): PREVENT inlining (for profiling/debugging) */
__attribute__((noinline))
int do_not_inline(int x) {
    return x * 42;
}

int main(void) {
    printf("square(9)           = %d\n",  square(9));        /* 81  */
    printf("cube(4)             = %d\n",  cube(4));          /* 64  */
    printf("max_int(7, 3)       = %d\n",  max_int(7, 3));    /* 7   */
    printf("clamp_int(150,0,100)= %d\n",  clamp_int(150,0,100)); /* 100 */
    printf("fast_pow2(10)       = %lld\n",fast_pow2(10));    /* 1024 */
    return 0;
}
```

---

### 3.8 Variadic Functions

Functions accepting a **variable number of arguments** — like `printf`. Uses `<stdarg.h>`.

```
KEY TERMS:
  va_list   — the type that holds your variable argument state
  va_start  — initialize the list (must know the last fixed param)
  va_arg    — read the next argument (you must know its type!)
  va_end    — clean up the list (MANDATORY)
  
MEMORY MODEL — how va_args work on the stack:
  sum(3,  10,  20,  30)
       │   │    │    │
       │   └────┴────┘
       │    variable args pushed onto stack
       └── count = 3  (fixed — tells us how many vars follow)

Stack layout (conceptual):
  ┌───────┬────┬────┬────┐
  │  n=3  │ 10 │ 20 │ 30 │
  └───────┴────┴────┴────┘
   fixed      variable args
   (last)     (accessed via va_arg)

  va_start(args, n)  points args to AFTER 'n'
  va_arg(args, int)  reads next int and advances pointer
```

```c
/* ─────────────────────────────────────────────────────────
   FILE : variadic_functions.c
   PURPOSE : Variable-argument functions in C
   ───────────────────────────────────────────────────────── */
#include 
#include   /* va_list, va_start, va_arg, va_end */

/*
 * Sum of 'count' integers.
 * IMPORTANT: Caller MUST pass the correct count — there is NO
 * type checking. Passing wrong count or wrong types = UB.
 */
int sum_n(int count, ...) {
    va_list args;
    va_start(args, count);   /* Initialize: last fixed param = count */

    int total = 0;
    for (int i = 0; i < count; i++) {
        total += va_arg(args, int); /* Read next int from args */
    }

    va_end(args);            /* MANDATORY cleanup */
    return total;
}

/* Find maximum among 'count' doubles */
double max_n(int count, ...) {
    va_list args;
    va_start(args, count);

    double max = va_arg(args, double);  /* First value */
    for (int i = 1; i < count; i++) {
        double val = va_arg(args, double);
        if (val > max) max = val;
    }

    va_end(args);
    return max;
}

/*
 * Custom logger — wraps vprintf.
 * 'vprintf' takes a va_list directly.
 * Use this pattern to build wrappers around printf-family functions.
 */
void log_msg(const char *level, const char *fmt, ...) {
    printf("[%-5s] ", level);

    va_list args;
    va_start(args, fmt);
    vprintf(fmt, args);      /* vprintf consumes the va_list */
    va_end(args);

    putchar('\n');
}

/*
 * Passing va_list to another function:
 * Use va_copy() to duplicate the list if you need to traverse twice.
 */
void forward_to_vprintf(const char *fmt, ...) {
    va_list args, args_copy;
    va_start(args, fmt);
    va_copy(args_copy, args);      /* Make a copy */

    /* Use original */
    printf("[original] ");
    vprintf(fmt, args);
    putchar('\n');

    /* Use copy */
    printf("[copy]     ");
    vprintf(fmt, args_copy);
    putchar('\n');

    va_end(args_copy);
    va_end(args);
}

int main(void) {
    printf("sum_n(3, 10,20,30)   = %d\n", sum_n(3, 10, 20, 30));   /* 60  */
    printf("sum_n(5, 1,2,3,4,5) = %d\n", sum_n(5, 1, 2, 3, 4, 5));/* 15  */
    printf("max_n(4, 3.1,7.9,2.0,5.5) = %.1f\n",
           max_n(4, 3.1, 7.9, 2.0, 5.5));                           /* 7.9 */

    log_msg("INFO",  "Server started on port %d", 8080);
    log_msg("ERROR", "File '%s' not found (errno=%d)", "cfg.bin", 2);
    log_msg("DEBUG", "x=%d y=%.2f z=%s", 42, 3.14, "hello");

    forward_to_vprintf("value = %d, label = %s", 99, "test");
    return 0;
}
```

---

### 3.9 Function Pointers & Callbacks

A **function pointer** stores the memory address of a function. This enables:

- Runtime selection of behavior
- Callbacks (you give a function to be called later)
- Strategy pattern (swap algorithms without changing caller)
- Dispatch tables (fast indexed lookup of functions)

```
FUNCTION POINTER CONCEPT:

  Text segment (code):
  ┌────────────────────────────────┐
  │  add():    [machine code]      │  address = 0x4004a0
  │  subtract:[machine code]       │  address = 0x4004b0
  │  multiply: [machine code]      │  address = 0x4004c0
  └────────────────────────────────┘

  Stack / Data:
  int (*op)(int, int);   /* op is a POINTER to a function */
  op = add;              /* op now holds address 0x4004a0 */
  op(3, 4);              /* indirect call to address 0x4004a0 */
                         /* equivalent to: add(3, 4) = 7    */

DECLARATION SYNTAX (the hardest part of C):
  int (*name)(int, int)      — pointer to fn(int,int)->int
  void (*name)(char *)        — pointer to fn(char*)->void
  double (*name)(void)        — pointer to fn()->double
  int (**name)(int)           — pointer to POINTER to fn

TYPEDEF makes it clean:
  typedef int (*BinaryOp)(int, int);   — now BinaryOp is the type
  BinaryOp op = add;                   — much cleaner
```

```c
/* ─────────────────────────────────────────────────────────
   FILE : function_pointers.c
   PURPOSE : Function pointers, callbacks, dispatch tables
   ───────────────────────────────────────────────────────── */
#include 
#include 
#include 

/* ── Arithmetic functions ─────────────────────────────────── */
int add(int a, int b)      { return a + b; }
int subtract(int a, int b) { return a - b; }
int multiply(int a, int b) { return a * b; }
int divide_int(int a, int b) {
    return (b != 0) ? a / b : 0;
}

/* ── TYPEDEF: give function pointer types readable names ───── */
typedef int  (*BinaryOp)(int, int);
typedef void (*IntCallback)(int);
typedef int  (*Comparator)(const void *, const void *);

/* ── HIGHER-ORDER: takes function as parameter ──────────────── */
int calculate(int a, int b, BinaryOp op) {
    return op(a, b);   /* Call through pointer */
}

/* ── FUNCTION FACTORY: returns a function pointer ───────────── */
BinaryOp select_operation(char op_char) {
    switch (op_char) {
        case '+': return add;
        case '-': return subtract;
        case '*': return multiply;
        case '/': return divide_int;
        default:  return NULL;
    }
}

/* ── DISPATCH TABLE: array of function pointers ─────────────── */
/*
 * A dispatch table maps index → function.
 * Faster than long if/switch chains.
 * Used in interpreters, state machines, OS syscall tables.
 */
static const BinaryOp ops[]   = { add, subtract, multiply, divide_int };
static const char     *names[] = { "add", "subtract", "multiply", "divide" };
static const int       nops    = 4;

/* ── CALLBACK PATTERN ────────────────────────────────────────── */
/*
 * A callback = function pointer passed to another function,
 * to be called by that function at the right moment.
 * Like an event listener or hook.
 */
void foreach_int(int *arr, int len, IntCallback cb) {
    for (int i = 0; i < len; i++) cb(arr[i]);
}

void print_squared(int n)  { printf("%d^2 = %d\n", n, n * n); }
void print_doubled(int n)  { printf("%d*2 = %d\n", n, n * 2); }
void print_is_prime(int n) {
    int prime = (n >= 2);
    for (int i = 2; i * i <= n; i++) { if (n % i == 0) { prime=0; break; } }
    printf("%d: %s\n", n, prime ? "prime" : "composite");
}

/* ── COMPARATORS for qsort ───────────────────────────────────── */
int cmp_asc (const void *a, const void *b) {
    return *(int*)a - *(int*)b;
}
int cmp_desc(const void *a, const void *b) {
    return *(int*)b - *(int*)a;
}

/* ── FUNCTION POINTER ARRAY for state machine ───────────────── */
/* (simplified: 3 states, each has a handler function) */
typedef void (*StateHandler)(void);
void state_idle(void)    { printf("[IDLE]    Waiting for input...\n"); }
void state_running(void) { printf("[RUNNING] Processing data...\n");  }
void state_done(void)    { printf("[DONE]    Finished.\n");           }

StateHandler state_machine[] = { state_idle, state_running, state_done };

int main(void) {
    /* ── Basic usage */
    BinaryOp op = add;
    printf("op = add:      op(10, 5) = %d\n", op(10, 5));  /* 15 */
    op = multiply;
    printf("op = multiply: op(10, 5) = %d\n", op(10, 5));  /* 50 */

    /* ── calculate() */
    printf("calculate(12, 4, +) = %d\n", calculate(12, 4, add));      /* 16 */
    printf("calculate(12, 4, *) = %d\n", calculate(12, 4, multiply)); /* 48 */

    /* ── select_operation (factory) */
    BinaryOp dyn = select_operation('*');
    if (dyn) printf("dyn(6, 7) = %d\n", dyn(6, 7));  /* 42 */

    /* ── Dispatch table */
    printf("\n--- Dispatch Table ---\n");
    for (int i = 0; i < nops; i++) {
        printf("%-8s(20, 4) = %d\n", names[i], ops[i](20, 4));
    }

    /* ── Callbacks */
    printf("\n--- Callbacks ---\n");
    int data[] = {2, 3, 5, 7, 11};
    foreach_int(data, 5, print_squared);
    foreach_int(data, 5, print_is_prime);

    /* ── qsort with comparator */
    printf("\n--- qsort ---\n");
    int nums[] = {5, 2, 8, 1, 9, 3, 7};
    int n = 7;
    qsort(nums, n, sizeof(int), cmp_asc);
    printf("Ascending:  ");
    for (int i=0;i<n;i++) printf("%d ", nums[i]); /* 1 2 3 5 7 8 9 */
    printf("\n");
    qsort(nums, n, sizeof(int), cmp_desc);
    printf("Descending: ");
    for (int i=0;i<n;i++) printf("%d ", nums[i]); /* 9 8 7 5 3 2 1 */
    printf("\n");

    /* ── State machine */
    printf("\n--- State Machine ---\n");
    for (int s = 0; s < 3; s++) state_machine[s]();

    return 0;
}
```

---

### 3.10 `_Noreturn` Functions (Diverging Functions)

A function that **never returns to its caller**. Terminates the program or loops forever.
`_Noreturn` (C11) tells the compiler so it can optimize and suppress false warnings.

```
NORMAL FUNCTION FLOW:                _Noreturn FUNCTION FLOW:
─────────────────────                ────────────────────────
  main()                               main()
    │                                    │
    └──► func()                          └──► fatal_error()
              │                                    │
              └──► returns to main                 └──► NEVER RETURNS
                                                        (exit/abort/loop)

WHY TELL THE COMPILER?
  Without _Noreturn, the compiler may warn about missing return
  values in branches that call the diverging function.
  With _Noreturn, it knows: "no path returns through here."
```

```c
/* ─────────────────────────────────────────────────────────
   FILE : noreturn_functions.c
   PURPOSE : Functions that never return (_Noreturn / noreturn)
   ───────────────────────────────────────────────────────── */
#include 
#include 
#include   /* C11: defines 'noreturn' as macro */

/* _Noreturn: C11 standard keyword */
_Noreturn void fatal_error(const char *msg) {
    fprintf(stderr, "FATAL: %s\n", msg);
    exit(EXIT_FAILURE);   /* NEVER returns */
}

/* noreturn: same thing — from  */
noreturn void panic(const char *file, int line, const char *msg) {
    fprintf(stderr, "PANIC [%s:%d] %s\n", file, line, msg);
    abort(); /* Never returns — also dumps core */
}

/* Helper macro — like Rust's panic! */
#define PANIC(msg) panic(__FILE__, __LINE__, (msg))

/* GCC/Clang attribute alternative (works without C11) */
__attribute__((noreturn))
void hard_fail(int code) {
    fprintf(stderr, "Hard fail with code %d\n", code);
    exit(code);
}

/* Real usage: the compiler knows no return after fatal_error */
int safe_divide(int a, int b) {
    if (b == 0) {
        fatal_error("Division by zero");
        /* No 'return 0;' needed here — compiler is satisfied */
    }
    return a / b;
}

/* Infinite event loop — also diverging */
_Noreturn void run_forever(void) {
    printf("Starting infinite loop...\n");
    while (1) {
        /* Process events, never exit */
    }
}

int main(void) {
    printf("Result: %d\n", safe_divide(10, 2)); /* 5 */
    /* safe_divide(10, 0); would call fatal_error and exit */
    return 0;
}
```

---

### 3.11 C Function Type Map

```
  C FUNCTIONS
  │
  ├── By Return Type
  │   ├── void           — returns nothing (procedure)
  │   ├── scalar         — int, float, char, pointer, etc.
  │   ├── struct         — returns struct by value (C99+)
  │   └── _Noreturn      — never returns (!= void: NEVER returns)
  │
  ├── By Linkage
  │   ├── extern         — visible across all files (default for functions)
  │   └── static         — private to the current translation unit (.c file)
  │
  ├── By Expansion Hint
  │   ├── regular        — actual call instruction generated
  │   └── inline         — hint: copy body at call site
  │        └── always_inline (__attribute__) — forced
  │        └── noinline   (__attribute__) — force no inlining
  │
  ├── By Arity (number of params)
  │   ├── fixed-arity    — specific number of params declared
  │   ├── variadic       — ...  with va_list  (stdarg.h)
  │   └── zero-param     — (void) or ()
  │
  ├── By Calling Mechanism
  │   ├── direct call    — function name at call site
  │   └── indirect call  — via function pointer (*fp)(args)
  │
  └── By Recursion
      ├── non-recursive  — simple
      ├── direct-recursive — calls itself
      ├── tail-recursive — recursive call is the LAST operation
      └── mutually-recursive — A calls B, B calls A
```

---

## 4. FUNCTIONS IN RUST — COMPLETE GUIDE

Rust has the richest function type system. It enforces ownership, borrowing, and lifetimes
through the type system — making programs **correct by construction** without runtime cost.

### Pre-Requisite Concepts for Understanding Rust Functions

```
┌───────────────────────────────────────────────────────────────┐
│                  RUST CORE CONCEPTS                            │
│                                                                 │
│  OWNERSHIP    Every value has ONE owner. When owner leaves     │
│               scope, the value is dropped (freed).             │
│                                                                 │
│  BORROWING    &T   = immutable borrow (read-only reference)    │
│               &mut T = mutable borrow (one at a time only)    │
│                                                                 │
│  LIFETIME     How long a reference is valid. Compiler tracks  │
│               this. You annotate with 'a, 'b when needed.     │
│                                                                 │
│  TRAIT        Like a C interface. Defines what a type can DO.  │
│               fn f<T: Trait>() means T must implement Trait.   │
│                                                                 │
│  GENERIC      A type parameter. fn f<T>() works for any T.    │
│                                                                 │
│  CLOSURE      Anonymous function that captures outer vars.     │
│                                                                 │
│  impl BLOCK   Where methods are attached to a struct/enum.    │
│                                                                 │
│  UNIT TYPE    () — like void. A zero-sized "nothing" value.   │
└───────────────────────────────────────────────────────────────┘
```

---

### 4.1 Regular Functions & The Expression Model

```
RUST FUNCTION SYNTAX:
  fn  name  <generics>  ( parameters )  ->  return_type  {  body  }
  │   │     │            │               │   │                │
  │   │     optional     typed params    │   optional         last expression
  keyword   type params  (all required)  │   (default = ())   = return value
                                         arrow
                         
CRITICAL: Semicolon Rule
  x + 1      ← EXPRESSION — has a value. When last in block = return value
  x + 1;     ← STATEMENT  — discards value, returns () (unit)
  
  fn double(x: i32) -> i32 {
      x * 2       // expression — this IS the return value
  }
  
  fn double_bug(x: i32) -> i32 {
      x * 2;      // statement — returns () — COMPILE ERROR: expected i32
  }
```

```rust
// ─────────────────────────────────────────────────────────
// FILE: regular_functions.rs
// PURPOSE: Core function patterns and expression model
// ─────────────────────────────────────────────────────────

// BASIC FUNCTION
// All parameter types are REQUIRED in Rust (unlike some languages)
// Return type after '->'
// Last expression WITHOUT semicolon = implicit return
fn add(a: i32, b: i32) -> i32 {
    a + b   // implicit return — no semicolon
}

// EXPLICIT RETURN with keyword — used for early exits
fn absolute_value(n: i32) -> i32 {
    if n < 0 {
        return -n;  // early return
    }
    n   // implicit return for the common path
}

// UNIT RETURN (equivalent to void)
// Functions with no return clause implicitly return ()
fn print_greeting(name: &str) {  // Returns ()
    println!("Hello, {}!", name);
    // implicit: ()
}

// BLOCKS AS EXPRESSIONS
// Entire if-else is an expression in Rust
fn categorize(n: i32) -> &'static str {
    // 'if' is an expression — each branch must have the same type
    if n < 0 {
        "negative"      // no semicolon = expression
    } else if n == 0 {
        "zero"
    } else {
        "positive"
    }
}

// MULTIPLE STATEMENTS + expression return
fn clamp(value: i32, lo: i32, hi: i32) -> i32 {
    // Statements first (with semicolons)
    let in_range = value >= lo && value <= hi;
    let _ = in_range; // suppress unused warning
    
    // Final expression = return value
    if value < lo {
        lo
    } else if value > hi {
        hi
    } else {
        value
    }
}

// UNIT FUNCTION (returns () explicitly for clarity)
fn log(msg: &str) -> () {
    println!("[LOG] {}", msg);
}

// NESTED FUNCTION — Rust allows functions inside functions
// Inner function has NO access to outer scope (unlike closures)
fn outer() -> i32 {
    fn inner(x: i32) -> i32 {  // defined inside outer()
        x * x
    }
    inner(5) + inner(3)  // 25 + 9 = 34
}

fn main() {
    println!("add(3, 4)           = {}", add(3, 4));           // 7
    println!("absolute_value(-9)  = {}", absolute_value(-9));  // 9
    println!("categorize(-5)      = {}", categorize(-5));      // negative
    println!("clamp(150, 0, 100)  = {}", clamp(150, 0, 100));  // 100
    println!("outer()             = {}", outer());              // 34
    log("Function exploration complete");
    print_greeting("Alice");
}
```

---

### 4.2 Methods & Associated Functions

```
RUST METHOD MODEL:

  struct MyType { ... }
  
  impl MyType {
      // ASSOCIATED FUNCTION: no 'self' receiver
      // Called as: MyType::new()
      // Like a static method / constructor
      fn new(...) -> Self { ... }
      
      // METHOD with &self: borrows self immutably (read-only)
      // Called as: instance.method()
      fn read_something(&self) -> T { ... }
      
      // METHOD with &mut self: borrows self mutably (can modify)
      // Called as: instance.modify()
      fn modify_something(&mut self) { ... }
      
      // METHOD with self: TAKES OWNERSHIP of self
      // Called as: instance.consume()
      // instance is NO LONGER usable after this call
      fn consume(self) -> T { ... }
  }

RECEIVER COMPARISON:
  Receiver    │ Ownership    │ Mutation    │ Use case
  ────────────┼──────────────┼─────────────┼─────────────────────
  (none)      │ none         │ N/A         │ Constructor, factory
  &self       │ borrows      │ read-only   │ Getter, computation
  &mut self   │ mut borrows  │ can modify  │ Setter, update
  self        │ moves        │ full        │ Consume/transform
```

```rust
// ─────────────────────────────────────────────────────────
// FILE: methods.rs
// PURPOSE: Methods and associated functions in Rust
// ─────────────────────────────────────────────────────────

// A simple 2D vector type
#[derive(Debug, Clone, Copy)]
struct Vec2 {
    x: f64,
    y: f64,
}

// impl block: attach functions to Vec2
impl Vec2 {

    // ── ASSOCIATED FUNCTIONS (no self — like constructors/factories)
    
    // Standard constructor (convention: name it 'new')
    // Called as: Vec2::new(1.0, 2.0)
    fn new(x: f64, y: f64) -> Self {
        Self { x, y }   // 'Self' = Vec2 here
    }
    
    // Named constructor — create the zero vector
    fn zero() -> Self {
        Self { x: 0.0, y: 0.0 }
    }
    
    // Named constructor — unit vector in X direction
    fn unit_x() -> Self {
        Self { x: 1.0, y: 0.0 }
    }
    
    // Factory: create from angle (polar to cartesian)
    fn from_angle(angle_radians: f64) -> Self {
        Self {
            x: angle_radians.cos(),
            y: angle_radians.sin(),
        }
    }

    // ── METHODS with &self (immutable — read-only access)
    
    // Compute the magnitude (length) of the vector
    fn magnitude(&self) -> f64 {
        (self.x * self.x + self.y * self.y).sqrt()
    }
    
    // Dot product with another vector
    fn dot(&self, other: &Vec2) -> f64 {
        self.x * other.x + self.y * other.y
    }
    
    // Add two vectors (returns a NEW Vec2)
    fn add(&self, other: &Vec2) -> Vec2 {
        Vec2::new(self.x + other.x, self.y + other.y)
    }
    
    fn scale(&self, factor: f64) -> Vec2 {
        Vec2::new(self.x * factor, self.y * factor)
    }
    
    fn is_zero(&self) -> bool {
        self.x == 0.0 && self.y == 0.0
    }
    
    // ── METHOD with &mut self (mutable — can modify fields)
    
    // Normalize IN-PLACE: make this vector unit length
    fn normalize_in_place(&mut self) {
        let mag = self.magnitude();
        if mag > 0.0 {
            self.x /= mag;   // modify self's fields directly
            self.y /= mag;
        }
    }
    
    fn set_x(&mut self, x: f64) { self.x = x; }
    fn set_y(&mut self, y: f64) { self.y = y; }
    
    // ── METHOD with self (consumes — takes ownership)
    
    // Returns a normalized version, consuming self
    // After calling this, the original variable is MOVED (unusable)
    fn into_normalized(self) -> Vec2 {
        let mag = self.magnitude();
        if mag > 0.0 {
            Vec2::new(self.x / mag, self.y / mag)
        } else {
            self  // Return unchanged if zero vector
        }
    }
}

// impl can appear MULTIPLE TIMES for the same type
impl Vec2 {
    fn describe(&self) -> String {
        format!("Vec2({:.3}, {:.3})", self.x, self.y)
    }
}

// Implementing the standard Display trait — makes println! work cleanly
impl std::fmt::Display for Vec2 {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "({:.3}, {:.3})", self.x, self.y)
    }
}

fn main() {
    // ── Associated functions (constructors)
    let v1 = Vec2::new(3.0, 4.0);
    let v2 = Vec2::zero();
    let v3 = Vec2::unit_x();
    
    println!("v1 = {}", v1);              // (3.000, 4.000)
    println!("v2 = {}", v2);              // (0.000, 0.000)
    println!("v3 = {}", v3);              // (1.000, 0.000)
    
    // ── &self methods (immutable)
    println!("v1.magnitude()    = {:.3}", v1.magnitude());  // 5.000
    println!("v1.dot(&v3)       = {:.3}", v1.dot(&v3));     // 3.000
    println!("v1.add(&v3)       = {}",    v1.add(&v3));
    println!("v2.is_zero()      = {}",    v2.is_zero());    // true
    
    // ── &mut self method (mutable)
    let mut mutable = Vec2::new(3.0, 4.0);
    println!("Before normalize: {}", mutable);
    mutable.normalize_in_place();
    println!("After normalize:  {}", mutable); // (0.600, 0.800)
    
    // ── self method (consuming)
    let v4 = Vec2::new(0.0, 5.0);
    let normalized = v4.into_normalized(); // v4 is MOVED here
    // v4 is no longer usable
    println!("normalized: {}", normalized);  // (0.000, 1.000)
}
```

---

### 4.3 Closures

A **closure** is an anonymous function that can **capture variables** from its surrounding scope.

```
FUNCTION vs CLOSURE:

  Named function:
  fn double(x: i32) -> i32 { x * 2 }
  ↑ Cannot see outer scope variables

  Closure:
  let multiplier = 3;
  let triple = |x| x * multiplier;  // captures 'multiplier'
  ↑ CAN see and use 'multiplier' from outer scope

CLOSURE SYNTAX PROGRESSION:
  Full:      |x: i32, y: i32| -> i32 { x + y }
  Types off: |x, y| { x + y }     (inferred)
  One expr:  |x, y| x + y          (no braces needed)

THREE CLOSURE TRAITS — how the closure captures its environment:

  Fn      — borrows immutably (&T). Can call many times. Most restrictive.
  FnMut   — borrows mutably (&mut T). Can call many times. Can mutate.
  FnOnce  — takes by value (T). Can only call ONCE (moves captured vars).
  
  HIERARCHY:  Fn ⊂ FnMut ⊂ FnOnce
  (every Fn is also an FnMut and FnOnce)
  
  'move' keyword: forces closure to take OWNERSHIP of captured vars.
  Use for threads, or when closure must outlive the outer scope.
```

```rust
// ─────────────────────────────────────────────────────────
// FILE: closures.rs
// PURPOSE: Closures — all patterns, capture modes, traits
// ─────────────────────────────────────────────────────────

fn main() {

    // ── BASIC CLOSURES ────────────────────────────────────────
    
    // Full type annotation
    let add: fn(i32, i32) -> i32 = |a, b| a + b;
    println!("add(3, 4)   = {}", add(3, 4));       // 7
    
    // Types inferred from usage
    let square = |x| x * x;
    println!("square(5)   = {}", square(5_i32));   // 25
    
    // Multi-line closure (like a function body)
    let clamp = |val: i32, lo: i32, hi: i32| -> i32 {
        if val < lo { lo }
        else if val > hi { hi }
        else { val }
    };
    println!("clamp(150, 0, 100) = {}", clamp(150, 0, 100)); // 100
    
    // ── CAPTURING BY IMMUTABLE REFERENCE (Fn) ─────────────────
    
    let threshold = 10_i32;
    // Captures 'threshold' by &i32 — read-only
    let is_big = |x: i32| x > threshold;
    
    println!("15 > 10? {}", is_big(15));   // true
    println!("5  > 10? {}", is_big(5));    // false
    println!("threshold still: {}", threshold); // 10 — not moved
    
    // ── CAPTURING BY MUTABLE REFERENCE (FnMut) ────────────────
    
    let mut total = 0_i32;
    let mut accumulate = |x: i32| {
        total += x;  // mutates captured 'total'
        total        // returns current total
    };
    
    println!("acc(5)  = {}", accumulate(5));   // 5
    println!("acc(10) = {}", accumulate(10));  // 15
    println!("acc(20) = {}", accumulate(20));  // 35
    
    drop(accumulate); // Release mutable borrow so we can use total again
    println!("final total: {}", total);  // 35
    
    // ── CAPTURING BY VALUE with 'move' (FnOnce / Fn) ──────────
    
    let name = String::from("Alice");
    
    // 'move' forces closure to TAKE OWNERSHIP of 'name'
    // Useful for threads where closure might outlive the variable
    let greet = move || {
        println!("Hello, {}!", name);   // name is OWNED by this closure
    };
    // println!("{}", name); // ERROR: name was moved into closure
    
    greet(); // Hello, Alice!
    greet(); // Can call again — closure still owns name (Fn, not FnOnce)
    
    // ── FnOnce: closure that can only be called ONCE ───────────
    
    let owned_data = vec![1, 2, 3];
    let consume_it = move || {
        let _data = owned_data; // moves owned_data OUT of closure
        println!("Consumed the vector");
    };
    
    consume_it();  // Works: owned_data moved here
    // consume_it(); // ERROR: would try to move owned_data again
    
    // ── CLOSURES IN ITERATOR CHAINS ───────────────────────────
    
    let numbers: Vec = (1..=10).collect();
    
    let even_squares: Vec = numbers.iter()
        .filter(|&&x| x % 2 == 0)  // keep evens
        .map(|&x| x * x)            // square them
        .collect();
    
    println!("Even squares: {:?}", even_squares);
    // [4, 16, 36, 64, 100]
    
    let sum: i32 = numbers.iter()
        .filter(|&&x| x % 2 == 0)
        .map(|&x| x * x)
        .sum();
    println!("Sum of even squares: {}", sum);  // 220
    
    // ── RETURNING CLOSURES ─────────────────────────────────────
    
    fn make_adder(n: i32) -> impl Fn(i32) -> i32 {
        move |x| x + n  // 'move' needed: n must outlive this function
    }
    
    let add5  = make_adder(5);
    let add10 = make_adder(10);
    
    println!("add5(7)   = {}", add5(7));   // 12
    println!("add10(7)  = {}", add10(7));  // 17
}
```

---

### 4.4 Higher-Order Functions

```rust
// ─────────────────────────────────────────────────────────
// FILE: higher_order.rs
// PURPOSE: Functions that take/return functions in Rust
// ─────────────────────────────────────────────────────────

// ── TAKES A FUNCTION: generic over Fn trait ────────────────────
// F is a type parameter constrained to Fn(i32) -> i32
// This means: F must be callable with i32, returning i32
fn apply_twice(f: F, x: i32) -> i32
where
    F: Fn(i32) -> i32,
{
    f(f(x))  // f applied to f applied to x
}

// Apply function to every element — like map() manually
fn map_vec(v: &[i32], f: F) -> Vec
where
    F: Fn(i32) -> i32,
{
    v.iter().map(|&x| f(x)).collect()
}

// Keep only elements where predicate returns true — like filter() manually
fn filter_vec(v: &[i32], pred: F) -> Vec
where
    F: Fn(i32) -> bool,
{
    v.iter().filter(|&&x| pred(x)).cloned().collect()
}

// Reduce to a single value — like fold() manually
fn fold_vec(v: &[i32], initial: A, f: F) -> A
where
    F: Fn(A, i32) -> A,
{
    let mut acc = initial;
    for &x in v { acc = f(acc, x); }
    acc
}

// ── RETURNS A FUNCTION ─────────────────────────────────────────
// 'impl Fn(i32) -> i32' = return some type implementing that trait
// 'move' captures 'factor' by value (moves it into the closure)
fn make_multiplier(factor: i32) -> impl Fn(i32) -> i32 {
    move |x| x * factor
}

fn make_range_checker(lo: i32, hi: i32) -> impl Fn(i32) -> bool {
    move |x| x >= lo && x <= hi
}

// ── FUNCTION COMPOSITION ───────────────────────────────────────
// compose(f, g)(x) = f(g(x))
fn compose(f: F, g: G) -> impl Fn(i32) -> i32
where
    F: Fn(i32) -> i32,
    G: Fn(i32) -> i32,
{
    move |x| f(g(x))  // Apply g first, then f to the result
}

// ── PIPELINE: multiple functions chained ──────────────────────
fn pipeline i32>(fns: &[F], x: i32) -> i32 {
    let mut val = x;
    for f in fns { val = f(val); }
    val
}

fn main() {
    let double = |x| x * 2;
    let inc    = |x| x + 1;
    
    // ── apply_twice
    println!("apply_twice(double, 3) = {}", apply_twice(double, 3)); // 12
    println!("apply_twice(inc,    3) = {}", apply_twice(inc,    3)); // 5
    
    // ── map_vec, filter_vec, fold_vec
    let nums = vec![1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
    let doubled  = map_vec(&nums, |x| x * 2);
    let evens    = filter_vec(&nums, |x| x % 2 == 0);
    let sum      = fold_vec(&nums, 0, |acc, x| acc + x);
    let product  = fold_vec(&[1,2,3,4,5], 1, |acc, x| acc * x);
    
    println!("doubled:  {:?}", doubled);
    println!("evens:    {:?}", evens);    // [2, 4, 6, 8, 10]
    println!("sum:      {}", sum);        // 55
    println!("product:  {}", product);    // 120
    
    // ── Function factories
    let triple    = make_multiplier(3);
    let in_range  = make_range_checker(1, 10);
    
    println!("triple(7) = {}", triple(7));       // 21
    println!("5 in [1,10]? {}", in_range(5));    // true
    println!("15 in [1,10]? {}", in_range(15));  // false
    
    // ── Composition
    let double_then_inc = compose(|x| x + 1, |x| x * 2);
    println!("double_then_inc(5) = {}", double_then_inc(5)); // 11
}
```

---

### 4.5 Generic Functions

```rust
// ─────────────────────────────────────────────────────────
// FILE: generic_functions.rs
// PURPOSE: Generic functions with type parameters and bounds
// ─────────────────────────────────────────────────────────

// GENERIC:  declares a type parameter
// T: PartialOrd means T must implement comparison (, <=, >=)
// The compiler generates specialized code for each concrete T used
fn max_val(a: T, b: T) -> T {
    if a > b { a } else { b }
}

fn min_val(a: T, b: T) -> T {
    if a < b { a } else { b }
}

// MULTIPLE TYPE PARAMS and MULTIPLE BOUNDS using 'where'
// 'where' clause is cleaner for complex bounds
fn print_max(a: T, b: T) -> T
where
    T: PartialOrd + std::fmt::Display + Copy,
    // T must implement: PartialOrd (compare)
    //                   Display (printable)
    //                   Copy (can be copied cheaply)
{
    let result = if a > b { a } else { b };
    println!("max({}, {}) = {}", a, b, result);
    result
}

// GENERIC SWAP — works for any type T
fn swap(a: T, b: T) -> (T, T) {
    (b, a)
}

// GENERIC: find first element matching predicate
fn find_first(slice: &[T], predicate: P) -> Option
where
    P: Fn(&T) -> bool,
{
    for item in slice {
        if predicate(item) { return Some(item); }
    }
    None
}

// GENERIC STRUCT with generic METHODS
struct Pair {
    first:  T,
    second: T,
}

impl Pair {
    fn new(first: T, second: T) -> Self {
        Pair { first, second }
    }
    
    fn swap(self) -> Pair {
        Pair { first: self.second, second: self.first }
    }
}

// Conditional method: only exists when T: PartialOrd
impl Pair {
    fn larger(&self) -> &T {
        if self.first >= self.second { &self.first } else { &self.second }
    }
    
    fn print_comparison(&self) {
        println!(
            "first={}, second={}, larger={}",
            self.first, self.second, self.larger()
        );
    }
}

fn main() {
    // Same function, different concrete types (monomorphized at compile time)
    println!("max(3, 7)     i32  = {}", max_val(3, 7));         // 7
    println!("max(3.1, 2.9) f64  = {}", max_val(3.1_f64, 2.9)); // 3.1
    println!("max('a', 'z') char = {}", max_val('a', 'z'));      // z
    
    print_max(42_i32, 99);
    print_max(3.14_f64, 2.71);
    
    // Swap
    let (a, b) = swap("hello", "world");
    println!("swap: ({}, {})", a, b);    // (world, hello)
    
    let (x, y) = swap(10_i32, 20);
    println!("swap: ({}, {})", x, y);    // (20, 10)
    
    // find_first
    let nums = [1, 5, 2, 8, 3, 7, 4];
    if let Some(first_big) = find_first(&nums, |&x| x > 5) {
        println!("First > 5: {}", first_big);  // 8
    }
    
    let words = ["apple", "banana", "cherry", "date"];
    if let Some(long) = find_first(&words, |w| w.len() > 5) {
        println!("First long word: {}", long);  // banana
    }
    
    // Pair with generic methods
    let p = Pair::new(10_i32, 25);
    p.print_comparison();
    println!("larger: {}", p.larger()); // 25
}
```

---

### 4.6 Const Functions

```rust
// ─────────────────────────────────────────────────────────
// FILE: const_functions.rs
// PURPOSE: Compile-time evaluation with const fn
// ─────────────────────────────────────────────────────────

/*
 * CONST FUNCTION: 'const fn' can be evaluated at compile time.
 * When called with compile-time constants, the result is computed
 * BEFORE the program runs — zero runtime cost.
 * Can also be called at runtime like a normal function.
 *
 * Restrictions: no heap allocation, no dynamic dispatch,
 * no floating point (currently), limited control flow.
 */

const fn factorial(n: u64) -> u64 {
    // 'match' is allowed in const fn (Rust 1.46+)
    match n {
        0 | 1 => 1,
        _ => n * factorial(n - 1),
    }
}

const fn fibonacci(n: u32) -> u64 {
    match n {
        0 => 0,
        1 => 1,
        _ => fibonacci(n - 1) + fibonacci(n - 2),
    }
}

const fn max_const(a: usize, b: usize) -> usize {
    if a > b { a } else { b }
}

const fn min_const(a: usize, b: usize) -> usize {
    if a < b { a } else { b }
}

// Compute a buffer size from two options — at compile time
const fn compute_buffer_size(n: usize) -> usize {
    n * n + 8  // Some formula
}

// ── CONSTANTS: evaluated at COMPILE TIME ──────────────────────
// These values are baked into the binary — not computed at startup
const FACTORIAL_10: u64   = factorial(10);  // 3628800
const FIBONACCI_20: u64   = fibonacci(20);  // 6765
const BUFFER_SIZE: usize   = compute_buffer_size(8); // 72
const BIGGER: usize        = max_const(100, 200);    // 200

// ── Used in array size (MUST be a compile-time constant) ───────
// Array sizes in Rust must be known at compile time
static LOOKUP_TABLE: [u64; 13] = {
    // Build lookup table at compile time (Rust 1.57+)
    let mut table = [0u64; 13];
    let mut i = 0;
    while i < 13 {
        table[i] = factorial(i as u64);
        i += 1;
    }
    table
};

fn main() {
    // These are already computed — just reading constants
    println!("10!          = {}", FACTORIAL_10);  // 3628800
    println!("fib(20)      = {}", FIBONACCI_20);  // 6765
    println!("BUFFER_SIZE  = {}", BUFFER_SIZE);   // 72
    println!("BIGGER       = {}", BIGGER);         // 200
    
    println!("\nFactorial lookup table:");
    for (i, &val) in LOOKUP_TABLE.iter().enumerate() {
        println!("{}! = {}", i, val);
    }
    
    // const fn also works at runtime
    let runtime_value = 7_u64;
    let runtime_fact = factorial(runtime_value);  // Computed at runtime
    println!("\n7! at runtime = {}", runtime_fact); // 5040
}
```

---

### 4.7 Async Functions

```rust
// ─────────────────────────────────────────────────────────
// FILE: async_functions.rs
// PURPOSE: Async/await in Rust
// Requires: tokio = { version = "1", features = ["full"] }
// ─────────────────────────────────────────────────────────

/*
 * ASYNC CONCEPT:
 * 
 * SYNC (blocking):
 *   result = do_slow_io()   ← thread SLEEPS waiting for I/O
 * 
 * ASYNC (non-blocking):
 *   future = do_slow_io()   ← returns immediately
 *   // ...do other work...
 *   result = future.await   ← suspend HERE, let other tasks run
 *                             resume when I/O is ready
 * 
 * 'async fn' RETURNS a FUTURE — a description of the computation.
 * '.await' EXECUTES the future (suspends until ready).
 * The executor (runtime) manages threads and scheduling.
 * 
 * async fn fetch() -> String
 * is EQUIVALENT to:
 * fn fetch() -> impl Future<Output = String>
 */

use tokio::time::{sleep, Duration};

// ASYNC FUNCTION: returns Future<Output = String>
// Does NOT block the thread — other tasks run while awaiting
async fn simulate_db_query(id: u32) -> String {
    sleep(Duration::from_millis(50)).await;  // Simulate latency
    format!("User {{ id: {}, name: \"User{}\" }}", id, id)
}

async fn simulate_http_get(url: &str) -> Result {
    sleep(Duration::from_millis(100)).await;
    if url.contains("fail") {
        Err(format!("HTTP 500: {}", url))
    } else {
        Ok(format!("Response from {}", url))
    }
}

// ASYNC with error propagation — '?' works in async fn
async fn fetch_user_data(user_id: u32) -> Result {
    let user = simulate_db_query(user_id).await;  // await the future
    let response = simulate_http_get("https://api.example.com/data").await?;
    Ok(format!("{} | {}", user, response))
}

// CONCURRENT: run multiple futures AT THE SAME TIME
// tokio::join! runs all futures concurrently on the same thread
async fn fetch_all_users() {
    let start = std::time::Instant::now();
    
    // Without join! (sequential — takes 150ms):
    // let u1 = simulate_db_query(1).await;
    // let u2 = simulate_db_query(2).await;
    // let u3 = simulate_db_query(3).await;
    
    // With join! (concurrent — takes ~50ms, all run at once):
    let (u1, u2, u3) = tokio::join!(
        simulate_db_query(1),
        simulate_db_query(2),
        simulate_db_query(3),
    );
    
    println!("Fetched {} users in {:?}", 3, start.elapsed());
    println!("{}", u1);
    println!("{}", u2);
    println!("{}", u3);
}

// tokio::main sets up the async runtime (event loop)
#[tokio::main]
async fn main() {
    // .await suspends main until the future resolves
    let user = simulate_db_query(42).await;
    println!("Queried: {}", user);
    
    match fetch_user_data(1).await {
        Ok(data)  => println!("Success: {}", data),
        Err(e)    => println!("Error: {}", e),
    }
    
    fetch_all_users().await;
}
```

---

### 4.8 Unsafe Functions

```rust
// ─────────────────────────────────────────────────────────
// FILE: unsafe_functions.rs
// PURPOSE: Unsafe functions — raw power with responsibility
// ─────────────────────────────────────────────────────────

/*
 * SAFE RUST: Compiler guarantees no UB (undefined behavior):
 *   - No dangling pointers
 *   - No data races
 *   - No buffer overflows
 *
 * UNSAFE RUST: YOU make the guarantees. Compiler can't verify.
 *   - Raw pointer dereference (*const T, *mut T)
 *   - Calling unsafe functions
 *   - Implementing unsafe traits
 *   - Accessing mutable statics
 *   - inline assembly
 *
 * 'unsafe fn' = this function has requirements the compiler
 *               cannot check. Caller must uphold invariants.
 *
 * 'unsafe { }' = I, the programmer, guarantee this block
 *               is sound (no UB will occur).
 *
 * GOLDEN RULE: Minimize unsafe surface area.
 * Wrap unsafe in a safe API that enforces correctness.
 */

// UNSAFE FUNCTION: raw pointer swap
// Caller must ensure: a and b are non-null, valid, non-overlapping
unsafe fn raw_swap(a: *mut i32, b: *mut i32) {
    let temp = *a;   // raw pointer dereference — unsafe
    *a = *b;
    *b = temp;
}

// SAFE WRAPPER: validates invariants before calling unsafe code
// This is the standard pattern: hide unsafe inside a safe API
fn safe_swap(a: &mut i32, b: &mut i32) {
    // Rust's borrow checker guarantees: a and b are valid,
    // non-null, and non-overlapping (can't have two &mut to same location)
    unsafe {
        raw_swap(a as *mut i32, b as *mut i32);
    }
}

// Transmute: reinterpret bytes as a different type
// EXTREMELY dangerous — only safe if the types have same size/alignment
unsafe fn bytes_to_u32(bytes: [u8; 4]) -> u32 {
    std::mem::transmute(bytes)
}

// Example: building a slice from raw parts (like C's array + length)
// Used when interfacing with C or low-level code
unsafe fn slice_from_raw(ptr: *const T, len: usize) -> &'static [T] {
    std::slice::from_raw_parts(ptr, len)
}

// Example: unsafe performance optimization (bounds-check elision)
unsafe fn get_unchecked_example(v: &[i32], index: usize) -> i32 {
    // No bounds check — if index >= v.len(), this is UB
    // Only safe if YOU guarantee index is valid
    *v.get_unchecked(index)
}

fn main() {
    // ── safe_swap (the right way to call unsafe code)
    let mut x = 10;
    let mut y = 20;
    safe_swap(&mut x, &mut y);
    println!("After swap: x={}, y={}", x, y);  // x=20, y=10
    
    // ── direct unsafe call requires unsafe block
    unsafe {
        raw_swap(&mut x, &mut y);
    }
    println!("After raw_swap: x={}, y={}", x, y);  // x=10, y=20
    
    // ── Byte reinterpretation (little-endian)
    unsafe {
        let bytes = [0x01_u8, 0x00, 0x00, 0x00];
        let value = bytes_to_u32(bytes);
        println!("bytes → u32 (LE): {}", value);  // 1
        
        // Viewing float bits — famous bit trick for fast inverse sqrt
        let pi: f32 = std::f32::consts::PI;
        let bits: u32 = std::mem::transmute(pi);
        println!("PI bits: 0x{:08X}", bits); // 0x40490FDB
    }
    
    // ── get_unchecked (safe usage — we know index is valid)
    let data = vec![10, 20, 30, 40, 50];
    let val = unsafe { get_unchecked_example(&data, 2) };
    println!("data[2] unchecked: {}", val); // 30
}
```

---

### 4.9 Diverging Functions (`-> !`)

```rust
// ─────────────────────────────────────────────────────────
// FILE: diverging_functions.rs
// PURPOSE: Functions that never return (type '!')
// ─────────────────────────────────────────────────────────

/*
 * '!' is the "never" type in Rust.
 * A function returning '!' NEVER returns to its caller.
 * It either:
 *   - Panics
 *   - Calls process::exit()
 *   - Loops forever
 *
 * WHY IS THIS USEFUL?
 * '!' coerces to ANY type. So:
 *   let x: i32 = if ok { 42 } else { panic!() };
 *   // panic!() has type '!' which satisfies i32 branch requirement
 *   
 * This allows diverging calls in any branch of if/match
 * without needing a dummy return value.
 */

// DIVERGING FUNCTION
fn crash(message: &str) -> ! {
    eprintln!("CRASH: {}", message);
    std::process::exit(1);   // Never returns
}

// Custom panic with rich context
fn unreachable_branch(location: &str) -> ! {
    panic!("Reached supposedly unreachable code at: {}", location);
}

// Infinite server loop
fn run_server() -> ! {
    println!("Server running...");
    loop {
        // Accept connections, serve requests...
        // This function never returns
    }
}

// ── HOW '!' ENABLES CLEAN MATCH/IF ────────────────────────────
fn parse_age(s: &str) -> u32 {
    match s.parse::() {
        Ok(n)  => n,         // type: u32
        Err(_) => crash("Invalid age"), // type: ! → coerces to u32
        // Without '!', we'd need: Err(_) => { crash("..."); 0 }
    }
}

fn get_mandatory_env(key: &str) -> String {
    match std::env::var(key) {
        Ok(val) => val,
        Err(_)  => crash(&format!("Required env var '{}' not set", key)),
        // ! coerces to String automatically
    }
}

// ── DIVERGING IN IF EXPRESSIONS ───────────────────────────────
fn positive_or_crash(n: i32) -> i32 {
    if n > 0 {
        n              // i32
    } else {
        crash("Value must be positive")  // ! → coerces to i32
    }
}

fn main() {
    println!("parse_age(\"25\") = {}", parse_age("25")); // 25
    println!("positive_or_crash(7) = {}", positive_or_crash(7)); // 7
    // positive_or_crash(-1); // Would call crash() and exit
}
```

---

### 4.10 Function Pointers in Rust

```rust
// ─────────────────────────────────────────────────────────
// FILE: function_pointers_rust.rs
// PURPOSE: fn pointers vs closures, dispatch tables
// ─────────────────────────────────────────────────────────

/*
 * FUNCTION POINTER TYPE: fn(T1, T2, ...) -> R
 *
 *   fn(i32, i32) -> i32     — takes two i32, returns i32
 *   fn(f64) -> bool          — takes f64, returns bool
 *   fn() -> ()               — no args, no return
 *
 * DIFFERENCE from closures:
 *   fn pointer: can ONLY point to named functions (no environment capture)
 *   Closure:    can capture environment (implements Fn/FnMut/FnOnce)
 *
 * WHEN TO USE fn pointers:
 *   - When you need to store function references in C-compatible structs
 *   - Dispatch tables with named functions only
 *   - FFI (C expects a function pointer)
 *   - Slightly more efficient than closures in some cases
 */

fn add(a: i32, b: i32) -> i32 { a + b }
fn sub(a: i32, b: i32) -> i32 { a - b }
fn mul(a: i32, b: i32) -> i32 { a * b }
fn div_safe(a: i32, b: i32) -> i32 { if b != 0 { a / b } else { 0 } }

// Function that takes a fn pointer
fn apply(a: i32, b: i32, op: fn(i32, i32) -> i32) -> i32 {
    op(a, b)
}

// Dispatch table: maps strings to function pointers
fn get_op(name: &str) -> Option i32> {
    match name {
        "add" => Some(add),
        "sub" => Some(sub),
        "mul" => Some(mul),
        "div" => Some(div_safe),
        _     => None,
    }
}

// Array of function pointers with labels
struct OpEntry {
    name: &'static str,
    func: fn(i32, i32) -> i32,
}

const OPERATIONS: [OpEntry; 4] = [
    OpEntry { name: "add", func: add },
    OpEntry { name: "sub", func: sub },
    OpEntry { name: "mul", func: mul },
    OpEntry { name: "div", func: div_safe },
];

// fn pointer stored in struct (like C struct with callbacks)
struct Calculator {
    operation: fn(i32, i32) -> i32,
    history:   Vec,
}

impl Calculator {
    fn new(op: fn(i32, i32) -> i32) -> Self {
        Self { operation: op, history: Vec::new() }
    }
    
    fn calculate(&mut self, a: i32, b: i32) -> i32 {
        let result = (self.operation)(a, b);
        self.history.push(result);
        result
    }
    
    fn set_operation(&mut self, op: fn(i32, i32) -> i32) {
        self.operation = op;
    }
}

fn main() {
    // ── Basic fn pointer
    let op: fn(i32, i32) -> i32 = add;
    println!("add(3, 4)   = {}", op(3, 4));  // 7
    
    // ── As argument
    println!("apply(10, 3, add) = {}", apply(10, 3, add)); // 13
    println!("apply(10, 3, mul) = {}", apply(10, 3, mul)); // 30
    
    // ── Dispatch table
    for entry in &OPERATIONS {
        println!("{}(12, 4) = {}", entry.name, (entry.func)(12, 4));
    }
    
    // ── get_op
    if let Some(op) = get_op("mul") {
        println!("get_op(mul)(6,7) = {}", op(6, 7)); // 42
    }
    
    // ── Calculator with swappable operation
    let mut calc = Calculator::new(add);
    println!("calc add:  {}", calc.calculate(10, 5));   // 15
    calc.set_operation(mul);
    println!("calc mul:  {}", calc.calculate(10, 5));   // 50
    println!("history:   {:?}", calc.history);           // [15, 50]
}
```

---

### 4.11 `impl Trait` in Functions

```rust
// ─────────────────────────────────────────────────────────
// FILE: impl_trait.rs
// PURPOSE: impl Trait in parameters and return position
// ─────────────────────────────────────────────────────────

/*
 * 'impl Trait' is sugar for "some type that implements Trait"
 *
 * IN PARAMETER POSITION (like generic bound):
 *   fn foo(x: impl Display)
 *   ≡  fn foo(x: T)
 *   Difference: impl Trait is cleaner for simple cases
 *
 * IN RETURN POSITION (opaque type):
 *   fn bar() -> impl Fn(i32) -> i32
 *   ≡  the return type is HIDDEN — caller only knows it implements the trait
 *   Useful: hide closure type, iterator type, etc.
 *   Note: you can only return ONE concrete type (unlike trait objects)
 */

use std::fmt::{Display, Debug};

// ── PARAMETER POSITION ────────────────────────────────────────

// Any type that can be printed via Display
fn print_value(item: impl Display) {
    println!("Value: {}", item);
}

// Multiple trait bounds with '+'
fn inspect(item: T) {
    println!("Display: {}", item);
    println!("Debug:   {:?}", item);
}

// Multiple impl Trait parameters (each can be a DIFFERENT type)
fn show_pair(a: impl Display, b: impl Display) {
    println!("({}, {})", a, b);
}

// ── RETURN POSITION ───────────────────────────────────────────

// Returns SOME type implementing Fn — caller doesn't know which type
fn make_adder(n: i32) -> impl Fn(i32) -> i32 {
    move |x| x + n  // Returns a closure — type is hidden
}

// Return an iterator without naming its complex type
fn even_squares(limit: u32) -> impl Iterator<Item = u32> {
    (1..=limit)
        .filter(|x| x % 2 == 0)
        .map(|x| x * x)
}

// Chain iterators — type gets extremely complex, impl Iterator hides it
fn process_data(data: &[i32]) -> impl Iterator<Item = String> + '_ {
    data.iter()
        .filter(|&&x| x > 0)
        .map(|&x| format!("positive:{}", x))
}

// IMPORTANT LIMITATION: impl Trait in return can only be ONE concrete type
fn make_fn(flag: bool) -> Box i32> {
    // This would be an ERROR with 'impl Fn':
    //   if flag { |x| x * 2 } else { |x| x + 10 }  // different closure types!
    // 
    // Solution: Box — dynamic dispatch
    if flag {
        Box::new(|x| x * 2)
    } else {
        Box::new(|x| x + 10)
    }
}

fn main() {
    // ── Parameter position
    print_value(42_i32);
    print_value(3.14_f64);
    print_value("hello world");
    
    inspect(99_i32);
    show_pair("name", "Alice");
    show_pair(42, 99);
    
    // ── Return position
    let add5  = make_adder(5);
    let add10 = make_adder(10);
    println!("add5(7)  = {}", add5(7));   // 12
    println!("add10(7) = {}", add10(7));  // 17
    
    let squares: Vec = even_squares(10).collect();
    println!("Even squares to 10: {:?}", squares); // [4, 16, 36, 64, 100]
    
    let data = vec![-1, 2, -3, 4, 5];
    let processed: Vec = process_data(&data).collect();
    println!("{:?}", processed); // ["positive:2", "positive:4", "positive:5"]
    
    // ── Box for runtime-chosen function
    let f = make_fn(true);
    let g = make_fn(false);
    println!("f(5) = {}", f(5)); // 10  (double)
    println!("g(5) = {}", g(5)); // 15  (add 10)
}
```

---

### 4.12 Extern Functions (FFI — Foreign Function Interface)

```rust
// ─────────────────────────────────────────────────────────
// FILE: ffi.rs
// PURPOSE: Calling C from Rust and Rust from C
// ─────────────────────────────────────────────────────────

/*
 * FFI = Foreign Function Interface
 * Rust can call C functions, and C can call Rust functions.
 *
 * RUST CALLING C:
 *   extern "C" {
 *       fn c_function(arg: type) -> return_type;
 *   }
 *   unsafe { c_function(arg); }
 *
 * C CALLING RUST:
 *   #[no_mangle]               // Don't mangle the name — keep it readable for C
 *   pub extern "C" fn rust_fn(arg: type) -> return_type { ... }
 *
 * WHY unsafe?
 *   Rust can't verify that C functions uphold Rust's guarantees.
 *   Null pointers, aliasing, memory layout — all YOUR responsibility.
 */

// ── CALLING C FROM RUST ────────────────────────────────────────

// 'extern "C"' block declares C functions available to call
extern "C" {
    // C standard library — libc
    fn abs(n: i32) -> i32;
    fn atoi(s: *const i8) -> i32;
    fn strlen(s: *const i8) -> usize;
    fn printf(format: *const i8, ...) -> i32;
}

// ── RUST FUNCTIONS CALLABLE FROM C ────────────────────────────

// #[no_mangle]: Keep the function name exactly as-is in the binary
// pub extern "C": Use C calling convention (ABI)
#[no_mangle]
pub extern "C" fn rust_add(a: i32, b: i32) -> i32 {
    a + b
}

#[no_mangle]
pub extern "C" fn rust_factorial(n: u64) -> u64 {
    (1..=n).product()  // Rust's clean way to compute factorial
}

// ── SAFE WRAPPERS around C FFI ────────────────────────────────
// Always wrap unsafe FFI in a safe Rust API

fn safe_abs(n: i32) -> i32 {
    unsafe { abs(n) }
}

fn safe_strlen(s: &str) -> usize {
    // Convert Rust &str to C-compatible *const i8
    use std::ffi::CString;
    let c_str = CString::new(s).expect("CString failed");
    unsafe { strlen(c_str.as_ptr()) }
}

fn main() {
    // ── Calling C functions safely via wrappers
    println!("abs(-42) via C  = {}", safe_abs(-42)); // 42
    println!("strlen(\"hello\") = {}", safe_strlen("hello")); // 5
    
    // ── Direct unsafe call
    unsafe {
        let result = abs(-100);
        println!("direct C abs(-100) = {}", result); // 100
    }
    
    // ── Rust functions that C can call
    println!("rust_add(10, 32)       = {}", rust_add(10, 32));     // 42
    println!("rust_factorial(10)     = {}", rust_factorial(10));   // 3628800
}
```

---

### 4.13 Rust Function Type Map

```
  RUST FUNCTIONS
  │
  ├── By Definition Style
  │   ├── fn name()              — named function
  │   ├── |params| body          — closure (anonymous)
  │   └── fn(T) -> R             — function pointer type (not closure)
  │
  ├── By Receiver (in impl blocks)
  │   ├── No self                — associated function (constructor)
  │   ├── &self                  — immutable method (reads)
  │   ├── &mut self              — mutable method (modifies)
  │   └── self                   — consuming method (moves ownership)
  │
  ├── By Modifier
  │   ├── const fn               — compile-time evaluable
  │   ├── async fn               — returns a Future
  │   ├── unsafe fn              — requires unsafe block to call
  │   └── extern "C" fn         — C calling convention (FFI)
  │
  ├── By Generics
  │   ├── fn f<T: Bound>()       — explicit type parameter
  │   ├── fn f(x: impl Trait)    — impl Trait in parameter
  │   └── fn f() -> impl Trait   — impl Trait in return (opaque)
  │
  ├── By Closure Capture Trait
  │   ├── Fn       — immutable capture (&T), callable many times
  │   ├── FnMut    — mutable capture (&mut T), callable many times
  │   ├── FnOnce   — moving capture (T), callable ONCE
  │   └── move |.| — force ownership capture (move semantics)
  │
  └── By Return Type
      ├── -> T                   — returns value
      ├── -> ()                  — returns nothing (unit)
      ├── -> Option<T>           — optional value
      ├── -> Result<T, E>        — value or error
      ├── -> impl Trait          — opaque return type
      ├── -> Box<dyn Trait>      — heap-allocated dynamic dispatch
      └── -> !                   — diverging (never returns)
```

---

## 5. FUNCTIONS IN GO — COMPLETE GUIDE

Go embraces clarity, simplicity, and pragmatism. Its function model is
straightforward yet powerful — especially with multiple returns, interfaces,
defer, goroutines, and closures.

---

### 5.1 Regular Functions

```
GO FUNCTION SYNTAX:
  func  name  ( param_list )  ( return_list )  {  body  }
  │     │       │               │                  │
  │     │       name type,      zero or more       computation
  keyword       name type       return types
  
NOTE: Return type list can be:
  ()                       — no returns (like void)
  int                      — single return (no parens needed)
  (int, error)             — multiple returns (parens required)
  (result int, err error)  — named returns (can use naked return)
```

```go
// ─────────────────────────────────────────────────────────
// FILE: basic_functions.go
// PURPOSE: Core Go function patterns
// ─────────────────────────────────────────────────────────
package main

import (
    "errors"
    "fmt"
    "math"
)

// BASIC FUNCTION: single return
func add(a int, b int) int {
    return a + b
}

// SHORTHAND: consecutive same-type params share the type annotation
func multiply(a, b int) int {
    return a * b
}

// MULTIPLE RETURN VALUES — Go's standout feature
// Idiomatic: return (result, error) pair
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil  // nil = no error
}

// THREE return values
func minMaxSum(nums []int) (min, max, sum int) {
    // Named returns declared in signature
    if len(nums) == 0 {
        return  // naked return: returns zero values
    }
    min, max, sum = nums[0], nums[0], 0
    for _, n := range nums {
        if n < min { min = n }
        if n > max { max = n }
        sum += n
    }
    return  // naked return: returns current values of min, max, sum
}

// NAMED RETURN VALUES with early exit
func safeSqrt(x float64) (result float64, err error) {
    // result and err are already declared as named returns
    if x < 0 {
        err = fmt.Errorf("cannot take sqrt of negative: %g", x)
        return  // returns result=0, err=
    }
    result = math.Sqrt(x)
    return  // returns result=sqrt(x), err=nil
}

// BLANK IDENTIFIER '_': discard unwanted return values
func getCoordinates() (int, int, int) {
    return 10, 20, 30  // x, y, z
}

func main() {
    // Single return
    fmt.Println("add(3, 4)       =", add(3, 4))       // 7
    fmt.Println("multiply(6, 7)  =", multiply(6, 7))   // 42

    // Multiple returns — MUST handle all returns
    result, err := divide(10.0, 3.0)
    if err != nil {
        fmt.Println("Error:", err)
    } else {
        fmt.Printf("10/3 = %.4f\n", result)  // 3.3333
    }

    _, err = divide(5.0, 0.0)    // '_' discards the float return
    if err != nil {
        fmt.Println("Error:", err)  // division by zero
    }

    // Named returns
    min, max, sum := minMaxSum([]int{3, 1, 9, 2, 7, 5})
    fmt.Printf("min=%d max=%d sum=%d\n", min, max, sum)

    r, e := safeSqrt(16.0)
    fmt.Printf("sqrt(16) = %.1f, err = %v\n", r, e)  // 4.0, 

    r, e = safeSqrt(-4.0)
    fmt.Printf("sqrt(-4) = %.1f, err = %v\n", r, e)  // 0.0, error

    // Blank identifier: discard y and z, keep only x
    x, _, _ := getCoordinates()
    fmt.Println("x:", x)  // 10
}
```

---

### 5.2 Methods in Go

```
GO METHOD SYNTAX:

  func  ( receiver )   methodName  ( params )   returnType   {  body  }
  
  func  (r Rectangle)  Area()      float64      { return r.Width * r.Height }
        │───────────────────│
        RECEIVER: the "this" / "self" in Go
        
VALUE RECEIVER (r Rectangle):
  - Gets a COPY of the value
  - Cannot modify the original
  - Use when: reading data, struct is small, type is value type (int, etc.)
  
POINTER RECEIVER (r *Rectangle):
  - Gets a POINTER to the original
  - CAN modify the original  
  - Use when: modification needed, struct is large (avoid copying)
  
RULE OF THUMB:
  If ANY method uses pointer receiver,
  make ALL methods pointer receivers for consistency.

METHODS ON NON-STRUCT TYPES:
  Go allows methods on any type defined in the same package.
  type MyInt int
  func (m MyInt) Double() MyInt { return m * 2 }
```

```go
// ─────────────────────────────────────────────────────────
// FILE: methods.go
// PURPOSE: Value receivers, pointer receivers, type methods
// ─────────────────────────────────────────────────────────
package main

import (
    "fmt"
    "math"
    "strings"
)

// ── STRUCT TYPES ───────────────────────────────────────────────
type Point struct {
    X, Y float64
}

type Circle struct {
    Center Point
    Radius float64
}

// ── CUSTOM BASIC TYPE ──────────────────────────────────────────
type Celsius    float64
type Fahrenheit float64
type Kelvin     float64

// ── VALUE RECEIVER METHODS (Point) ────────────────────────────
// Receives a COPY — cannot modify original Point
func (p Point) DistanceTo(other Point) float64 {
    dx := p.X - other.X
    dy := p.Y - other.Y
    return math.Sqrt(dx*dx + dy*dy)
}

func (p Point) String() string {
    return fmt.Sprintf("(%.2f, %.2f)", p.X, p.Y)
}

func (p Point) IsOrigin() bool {
    return p.X == 0 && p.Y == 0
}

// ── POINTER RECEIVER METHODS (Point) ──────────────────────────
// Receives a POINTER — CAN modify original
func (p *Point) Translate(dx, dy float64) {
    p.X += dx    // modifies ORIGINAL point
    p.Y += dy
}

func (p *Point) Scale(factor float64) {
    p.X *= factor
    p.Y *= factor
}

// ── CIRCLE METHODS ─────────────────────────────────────────────
func (c Circle) Area() float64 {
    return math.Pi * c.Radius * c.Radius
}

func (c Circle) Perimeter() float64 {
    return 2 * math.Pi * c.Radius
}

func (c Circle) Contains(p Point) bool {
    return c.Center.DistanceTo(p) <= c.Radius
}

func (c *Circle) SetRadius(r float64) {
    if r >= 0 { c.Radius = r }
}

// ── METHODS ON NAMED BASIC TYPES ──────────────────────────────
func (c Celsius) ToFahrenheit() Fahrenheit {
    return Fahrenheit(c*9/5 + 32)
}

func (c Celsius) ToKelvin() Kelvin {
    return Kelvin(c + 273.15)
}

func (f Fahrenheit) ToCelsius() Celsius {
    return Celsius((f - 32) * 5 / 9)
}

// ── NAMED STRING TYPE with methods ────────────────────────────
type Title string

func (t Title) Words() []string    { return strings.Fields(string(t)) }
func (t Title) WordCount() int     { return len(t.Words()) }
func (t Title) Uppercase() Title   { return Title(strings.ToUpper(string(t))) }

func main() {
    // ── Point methods
    p1 := Point{X: 3, Y: 4}
    p2 := Point{X: 0, Y: 0}

    fmt.Println("p1:", p1.String())                    // (3.00, 4.00)
    fmt.Printf("distance p1→p2: %.2f\n", p1.DistanceTo(p2)) // 5.00
    fmt.Println("p2 is origin?", p2.IsOrigin())        // true

    // Go auto-takes address for pointer receiver methods
    p1.Translate(1, 1)  // Go converts to: (&p1).Translate(1, 1)
    fmt.Println("p1 after translate:", p1.String())     // (4.00, 5.00)

    // ── Circle
    c := Circle{Center: Point{0, 0}, Radius: 5}
    fmt.Printf("area: %.2f\n", c.Area())               // 78.54
    fmt.Println("contains (3,4)?", c.Contains(Point{3, 4})) // true
    fmt.Println("contains (6,0)?", c.Contains(Point{6, 0})) // false

    // ── Temperature
    temp := Celsius(100)
    fmt.Printf("%.1f°C = %.1f°F = %.2f K\n",
        temp, temp.ToFahrenheit(), temp.ToKelvin())  // 100°C = 212°F = 373.15K

    // ── Title
    t := Title("the quick brown fox")
    fmt.Println("words:", t.WordCount())   // 4
    fmt.Println("upper:", t.Uppercase())   // THE QUICK BROWN FOX
}
```

---

### 5.3 Anonymous Functions & Closures in Go

```go
// ─────────────────────────────────────────────────────────
// FILE: closures.go
// PURPOSE: Anonymous functions, closures, IIFE, generators
// ─────────────────────────────────────────────────────────
package main

import "fmt"

func main() {

    // ── ANONYMOUS FUNCTION: no name, assigned to variable ────────
    double := func(x int) int { return x * 2 }
    square := func(x int) int { return x * x }

    fmt.Println("double(5):", double(5))  // 10
    fmt.Println("square(5):", square(5))  // 25
```


```markdown
# A Deep Dive into Function Types in Rust, C, and Go

This guide explores every facet of what we call a “function” across three languages: **Rust**, **C**, and **Go**.  
We will cover syntactic forms, type systems, memory representations, calling conventions, and the mental models that allow you to reason efficiently about code.  
Diagrams are given as ASCII art so you can always keep the architecture in view.

---

## Table of Contents
1. [Fundamental Concepts](#fundamental-concepts)
2. [Rust](#rust)
    - [Named Functions (fn Items)](#named-functions-fn-items)
    - [Function Pointers](#function-pointers-rust)
    - [Closures and the Fn* Traits](#closures-and-the-fn-traits)
    - [Methods and Associated Functions](#methods-and-associated-functions)
    - [Generic Functions and Const Generics](#generic-functions-and-const-generics)
    - [Diverging Functions (Never Type `!`)](#diverging-functions)
    - [Unsafe Functions](#unsafe-functions)
    - [Extern Functions and ABIs](#extern-functions-and-abis)
    - [Async Functions](#async-functions)
    - [Const Functions](#const-functions)
    - [Memory Layout and Call Architecture (ASCII)](#memory-layout-and-call-architecture-rust)
3. [C](#c)
    - [Function Definitions and Declarations](#function-definitions-and-declarations)
    - [Function Pointers](#function-pointers-c)
    - [Variadic Functions](#variadic-functions)
    - [Inline Functions](#inline-functions)
    - [Static and Extern Linkage](#static-and-extern-linkage)
    - [Nested Functions (GNU C)](#nested-functions-gnu-c)
    - [Signal Handlers and Callbacks](#signal-handlers-and-callbacks)
    - [`_Noreturn` and `noreturn`](#noreturn)
    - [Old-Style (K&R) Definitions](#old-style-kr-definitions)
    - [Memory Layout and Call Architecture (ASCII)](#memory-layout-and-call-architecture-c)
4. [Go](#go)
    - [Named Functions](#named-functions-go)
    - [Anonymous Functions and Closures](#anonymous-functions-and-closures-go)
    - [Methods and Method Values/Expressions](#methods-and-method-valuesexpressions)
    - [Variadic Functions](#variadic-functions-go)
    - [Function Types and First-Class Citizens](#function-types-and-first-class-citizens)
    - [Multiple Return Values and Named Results](#multiple-return-values-and-named-results)
    - [Deferred Functions, Panic, and Recover](#deferred-functions-panic-and-recover)
    - [Init Functions](#init-functions)
    - [Built-in Functions](#built-in-functions)
    - [Generic Functions (Go 1.18+)](#generic-functions-go-1.18)
    - [Memory Layout and Call Architecture (ASCII)](#memory-layout-and-call-architecture-go)
5. [Comparative Summary](#comparative-summary)

---

## Fundamental Concepts
A **function** maps a tuple of arguments to a result, possibly modifying state. Each language gives us different tools to abstract and pass this mapping around.

Key dimensions:
- **Named vs anonymous** – can the function be referred to by an identifier?
- **First‑class vs second‑class** – can it be stored, passed, returned?
- **Closure vs plain** – does it capture an environment?
- **Method vs free function** – is it associated with a type?
- **Linkage / visibility** – static, extern, public/private.
- **Special modifiers** – inline, const, async, unsafe, variadic, etc.

We will explore all of these for each language.

---

## Rust

Rust’s function system is expression‑oriented and heavily integrated with ownership and traits.  
A “function” can mean a **fn item**, a **function pointer**, or a **closure**. They are distinct types.

### Named Functions (fn Items)
```rust
fn add(x: i32, y: i32) -> i32 {
    x + y
}
```
- `add` has the type `fn(i32, i32) -> i32` **only when coerced**; its original type is a **unique zero‑sized type** representing that specific function item.
- You can coerce it to a function pointer: `let f: fn(i32, i32) -> i32 = add;`
- Function items are **not** traits; they don’t capture state.

### Function Pointers (Rust)
```rust
fn apply(f: fn(i32) -> i32, x: i32) -> i32 {
    f(x)
}
```
- Type: `fn(Args) -> Ret` – a raw pointer to a function.
- Non‑capturing, can be `unsafe` as well: `unsafe fn(...)`.
- Can be passed across FFI boundaries because it matches C’s function pointer ABI.
- Coercion from function items to pointers happens automatically.

### Closures and the Fn* Traits
Closures are anonymous functions that can capture variables from their environment.  
They are represented by **compiler‑generated structs** that implement one or more of:

- `FnOnce` – consumes captured variables (takes `self` by value).
- `FnMut` – borrows captured variables mutably.
- `Fn` – borrows captured variables immutably.

Each trait has a method `call`, `call_mut`, `call_once`.

```rust
let mut count = 0;
let mut inc = || { count += 1; count }; // FnMut
let get = || count;                     // Fn (immutable borrow)
let consume = || { let c = count; c };  // FnOnce if moved
```
Closures that don’t capture are coercible to function pointers.

**Implementation Detail**  
A closure is an anonymous struct with fields for each captured variable.  
Example:
```rust
let text = String::from("hello");
let print = || println!("{}", text);
```
The compiler generates something akin to:
```rust
struct PrintClosure {
    text: String,
}
impl FnOnce<()> for PrintClosure { ... }
impl FnMut<()> for PrintClosure { ... } // if needed
impl Fn<()> for PrintClosure { ... }    // if possible
```
The `call` methods take `self` according to the trait (`&self`, `&mut self`, `self`).

### Methods and Associated Functions
- **Associated functions** are defined inside `impl` blocks without `self` parameter, called with `Type::func()`.
- **Methods** take `self`, `&self`, or `&mut self` as first parameter.

```rust
struct Point { x: f64, y: f64 }
impl Point {
    fn new(x: f64, y: f64) -> Self { Self { x, y } } // associated
    fn dist(&self) -> f64 { (self.x.powi(2) + self.y.powi(2)).sqrt() } // method
}
```
They are regular functions; `&self` is syntactic sugar for `self: &Self`.

### Generic Functions and Const Generics
```rust
fn id<T>(x: T) -> T { x }
fn array_of<T, const N: usize>(value: T) -> [T; N] { [value; N] }
```
Monomorphization: the compiler creates a separate copy for each concrete type used.

### Diverging Functions
A function that never returns has the return type `!` (the *never* type).
```rust
fn exit_program() -> ! {
    std::process::exit(0)
}
```
It can be used as any type because `!` coerces to any type. Useful for `match` arms, `panic!`, etc.

### Unsafe Functions
```rust
unsafe fn dangerous() {}
fn main() {
    unsafe { dangerous(); }
}
```
Calling them requires an `unsafe` block. They are regular function items/pointers but carry an unsafe marker; the pointer type is `unsafe fn(...)`.

### Extern Functions and ABIs
```rust
extern "C" {
    fn abs(input: i32) -> i32;
}
extern "C" fn rust_callback(x: i32) { ... }
```
Specifies a calling convention. Default is `"Rust"`. The `"C"` ABI guarantees interoperability.

### Async Functions
```rust
async fn fetch_data() -> String { ... }
```
Desugars to a function returning `impl Future<Output = String>`. The function itself is synchronous; the body produces a state machine.

### Const Functions
```rust
const fn square(x: i32) -> i32 { x * x }
static ARR: [i32; square(3)] = [0; 9];
```
Evaluable at compile time; they are pure and have restrictions (no I/O, no loops until recent Rust editions).

### Memory Layout and Call Architecture (Rust)
**Function call** (simplified):
```
Stack frame of caller
+-------------------------+
| return address          |
| saved frame pointer     |
| arguments (right to left)|   <-- callee accesses via stack pointer + offset
| local variables         |
+-------------------------+
```
When a closure is called:
```
Closure struct (on stack or heap)
+----------------------+
| captured var1        |  <-- pointer to env
| captured var2        |
+----------------------+
Closure's call method receives &self pointing here.
```

**Coercion of non‑capturing closure to fn pointer:**
```
let f: fn() = || { println!("hello"); };
// compiler generates an anonymous fn item, address is taken.
```

---

## C

C functions are the bedrock of system programming. They lack closures and generics, but make up for it with pointer arithmetic and sheer flexibility.

### Function Definitions and Declarations
```c
// declaration (prototype)
int add(int a, int b);

// definition
int add(int a, int b) {
    return a + b;
}
```
- Can be forward declared; prototype informs the compiler about argument types.
- In C89, an empty parameter list means “unspecified parameters”; in C99+, it means **no parameters**. Use `void` to be explicit.

### Function Pointers (C)
```c
int (*fptr)(int, int) = &add;
int result = (*fptr)(3, 4); // or fptr(3, 4)
```
- Type: `return_type (*)(param_types)`.
- Commonly used for callbacks, e.g., `qsort`:
  ```c
  void qsort(void *base, size_t nmemb, size_t size,
             int (*compar)(const void *, const void *));
  ```
- Can be stored in arrays, structs; can be passed as arguments.

### Variadic Functions
```c
#include <stdarg.h>
int sum(int count, ...) {
    va_list args;
    va_start(args, count);
    int total = 0;
    for (int i = 0; i < count; i++)
        total += va_arg(args, int);
    va_end(args);
    return total;
}
```
- At least one named parameter is required before `...`.
- The type of each argument must be known by the callee (no runtime type info).

### Inline Functions
```c
inline int max(int a, int b) {
    return (a > b) ? a : b;
}
```
- Suggestion to the compiler; may be inlined or not.
- With `static inline`, each translation unit gets its own copy; with `extern inline`, definitions can be shared.

### Static and Extern Linkage
- **static**: function has internal linkage, visible only within the translation unit.
  ```c
  static int helper() { ... }
  ```
- **extern** (default): external linkage, visible across the whole program.
- Inline functions can combine `extern` and `inline` in header files with the GNU model.

### Nested Functions (GNU C)
GCC provides a non‑standard extension:
```c
int outer(int x) {
    int inner(int y) { return x + y; } // captures x from outer scope
    return inner(5);
}
```
Nested functions can access variables of the enclosing function. They are implemented via **trampolines** (executable stack) or **descriptors** depending on the architecture. This is a security risk (executable stack) and not portable; avoid in production.

### Signal Handlers and Callbacks
```c
#include <signal.h>
void handler(int sig) { ... }
signal(SIGINT, handler);
```
Function pointer with signature `void (*)(int)`. The function must be async‑signal‑safe.

### `_Noreturn` and `noreturn`
```c
#include <stdnoreturn.h>
_Noreturn void abort_program(void) { ... exit(1); }
// or with convenience macro: noreturn
```
Tells the compiler the function never returns; aids optimization and suppresses warnings.

### Old‑Style (K&R) Definitions
```c
double square(x)
double x;
{
    return x * x;
}
```
Pre‑ANSI style; no prototype, no argument type checking by caller. Should not be used.

### Memory Layout and Call Architecture (C)
Standard stack frame (cdecl):
```
Higher addresses
+------------------+
| ...              |
| Argument N       |
| Argument N-1     |
| ...              |
| Return address   |
| Saved EBP        | <- EBP
| Local variables  |
| ...              | <- ESP
+------------------+
```
- Arguments pushed right‑to‑left, caller cleans stack.
- Variadic functions rely on `va_list` which walks up the stack.
- Function pointer call simply places the address in a register and jumps.

**Callback example memory:**
```
struct Button {
    void (*on_click)(void*);
    void* user_data;
};
```
Calls `on_click(user_data)`.

---

## Go

Go combines simplicity with first‑class functions and closures, backed by a garbage collector.

### Named Functions (Go)
```go
func add(x, y int) int {
    return x + y
}
```
- Supports multiple return values:
  ```go
  func divide(a, b int) (int, error) { ... }
  ```
- Named return values act as documented pre‑initialised variables:
  ```go
  func split(sum int) (x, y int) {
      x = sum * 4 / 9
      y = sum - x
      return // naked return
  }
  ```

### Anonymous Functions and Closures (Go)
```go
func main() {
    msg := "hello"
    f := func() {
        fmt.Println(msg) // closes over msg
    }
    f()
}
```
- `f` is of type `func()`.
- Go closures are implemented as a **pointer to a struct** containing the function’s code pointer and captured variables.
- The struct is allocated on the heap if it escapes; otherwise stack‑allocated.

**Example with mutable capture:**
```go
func counter() func() int {
    count := 0
    return func() int {
        count++
        return count
    }
}
```
`count` lives on the heap because it survives the outer scope.

### Methods and Method Values/Expressions
Go associates methods with any named type (not struct only):
```go
type Vertex struct { X, Y float64 }
func (v Vertex) Abs() float64 {
    return math.Sqrt(v.X*v.X + v.Y*v.Y)
}
func (v *Vertex) Scale(f float64) {
    v.X *= f
    v.Y *= f
}
```
- **Method values**: `v.Abs` is a function value bound to `v`.
- **Method expressions**: `Vertex.Abs` is a function taking the receiver as first argument: `func(Vertex) float64`.  
  (`(*Vertex).Scale` → `func(*Vertex, float64)`).

```go
var p Vertex
absFunc := p.Abs           // method value
scaleFunc := (*Vertex).Scale // method expression
scaleFunc(&p, 2.0)
```

### Variadic Functions (Go)
```go
func sum(nums ...int) int {
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}
sum(1,2,3)       // works
arr := []int{4,5}
sum(arr...)      // spread slice
```

### Function Types and First‑Class Citizens
```go
type Transformer func(int) int
func apply(f Transformer, x int) int { return f(x) }
```
- Function types are reference types; a nil function can be called but causes a runtime panic.
- Functions can be passed, returned, stored in maps, structs, slices.

### Multiple Return Values and Named Results
Multiple returns are not tuples but a sequence; they can be used in assignments:
```go
a, b := swap(3, 4)
```
Named results provide implicit returns and documentation. `return` without arguments returns the current values of the named result variables.

### Deferred Functions, Panic, and Recover
```go
func readFile(path string) {
    f := os.Open(path)
    defer f.Close()
    // work
}
```
`defer` pushes a function call onto a stack; it executes in LIFO order when the surrounding function returns.  
`panic` unwinds the stack, executing deferred functions. `recover` (inside a deferred function) catches a panic.
```go
defer func() {
    if r := recover(); r != nil {
        log.Println("recovered:", r)
    }
}()
```
These are not “function types” but are integral to Go’s control flow.

### Init Functions
```go
func init() {
    // called automatically at program startup, per package
}
```
No parameters, no return value. Cannot be called explicitly. Used for package initialization.

### Built‑in Functions
Go’s built‑ins (`len`, `cap`, `append`, `make`, `copy`, `delete`, `close`, `panic`, `recover`, `print`, `println`, `complex`, `real`, `imag`) are not declared in the `builtin` package but are part of the language. They have special compiler support and are not assignable to function variables.

### Generic Functions (Go 1.18+)
```go
func Map[T any](s []T, f func(T) T) []T {
    res := make([]T, len(s))
    for i, v := range s {
        res[i] = f(v)
    }
    return res
}
```
Type parameters are specified with square brackets. Go uses **monomorphisation** (though with some dictionary‑based optimisations for dynamic types via interfaces).

### Memory Layout and Call Architecture (Go)
**Regular function call**:
```
Goroutine stack (growing)
+------------------+
| ...              |
| arguments        |  (passed on stack, some in registers depending on ABI)
| return address   |
| frame pointer?   |
| local variables  |
+------------------+
```
- Go uses a custom ABI; current implementation (Go 1.17+) passes arguments and results in registers when possible (Plan9‑based ABI for amd64). The stack is managed by the runtime and can grow/shrink.

**Closure implementation**:
```
Closure variable (func type) = pointer to:
+-------------------+
| code pointer      | --> actual function code (with closure context as first argument)
| captured var1     |
| captured var2     |
+-------------------+
```
When you call a closure, the code receives a pointer to this struct as the first argument.

Example disassembly idea:
```
func (f *funcStruct) func(...) { ... }
```

**Method call**:
A method value `v.Abs` creates a closure that captures the receiver `v`:
```
methodVal struct:
    codePtr -> method code
    receiver copy
```
A method expression `Vertex.Abs` is a plain function pointer expecting the receiver as first argument.

---

## Comparative Summary

| Feature | Rust | C | Go |
|---------|------|---|---|
| **Named functions** | `fn name(...)` | `int name(...)` | `func name(...)` |
| **Function pointers** | `fn(...)` type, safe | `int (*)(...)` raw pointer | `func(...)` reference type, nil is callable |
| **Closures** | Anonymous structs implementing `Fn` traits; capture by move/ref | No standard support (GNU nested functions only) | Heap‑allocated struct with code pointer + env; captures by reference |
| **Methods** | Via `impl` blocks, receiver `self`, `&self`, `&mut self` | No; simulated with function pointer + struct* | Receiver on any named type, value or pointer; method values/expressions |
| **Variadic** | Macros or slices; no C‑style `...` | `stdarg.h`, `va_list` | `...Type` as slice, spread operator |
| **Multiple returns** | Via tuples `(i32, &str)` | No; via output parameters or structs | Native multiple return values |
| **Generics** | Compile‑time monomorphisation (and const generics) | No (macros or `void*`) | Compile‑time monomorphisation / dictionary hybrid |
| **Never‑returning** | `!` (never type) | `_Noreturn` / `noreturn` | `panic` / infinite loop; no distinct type |
| **Unsafe** | `unsafe fn` and `unsafe` block | Everything is unsafe by nature | `unsafe` package for pointer manipulation |
| **Async** | `async fn` → `Future` | No standard async | Goroutines (not `async` but similar concurrency) |
| **Linkage control** | Visibility with modules (`pub`) | `static`/`extern` | Exported if name starts with uppercase |
| **Compile‑time eval** | `const fn` | `constexpr` (C23) – not functions | No; `init()` at startup |
| **Error handling** | `Result` via returns | Error codes, errno, longjmp | Multiple returns + `error` interface, `panic`/`recover` |

Understanding these function models deeply gives you a “metal model” for how abstraction maps to the machine, enabling efficient, safe, and idiomatic code in any of these languages.

--- 
```
This document contains all major function types and their implementation details. Study the ASCII architecture carefully; it reveals how closures become structs, how stacks are laid out, and how the compiler bridges your high‑level intent with the hardware.
```

# A Deep Dive into Function Types in Rust, C, and Go

This guide explores every facet of what we call a “function” across three languages: **Rust**, **C**, and **Go**.  
We will cover syntactic forms, type systems, memory representations, calling conventions, and the mental models that allow you to reason efficiently about code.  
Diagrams are given as ASCII art so you can always keep the architecture in view.

---

## Table of Contents
1. [Fundamental Concepts](#fundamental-concepts)
2. [Rust](#rust)
    - [Named Functions (fn Items)](#named-functions-fn-items)
    - [Function Pointers](#function-pointers-rust)
    - [Closures and the Fn* Traits](#closures-and-the-fn-traits)
    - [Methods and Associated Functions](#methods-and-associated-functions)
    - [Generic Functions and Const Generics](#generic-functions-and-const-generics)
    - [Diverging Functions (Never Type `!`)](#diverging-functions)
    - [Unsafe Functions](#unsafe-functions)
    - [Extern Functions and ABIs](#extern-functions-and-abis)
    - [Async Functions](#async-functions)
    - [Const Functions](#const-functions)
    - [Memory Layout and Call Architecture (ASCII)](#memory-layout-and-call-architecture-rust)
3. [C](#c)
    - [Function Definitions and Declarations](#function-definitions-and-declarations)
    - [Function Pointers](#function-pointers-c)
    - [Variadic Functions](#variadic-functions)
    - [Inline Functions](#inline-functions)
    - [Static and Extern Linkage](#static-and-extern-linkage)
    - [Nested Functions (GNU C)](#nested-functions-gnu-c)
    - [Signal Handlers and Callbacks](#signal-handlers-and-callbacks)
    - [`_Noreturn` and `noreturn`](#noreturn)
    - [Old-Style (K&R) Definitions](#old-style-kr-definitions)
    - [Memory Layout and Call Architecture (ASCII)](#memory-layout-and-call-architecture-c)
4. [Go](#go)
    - [Named Functions](#named-functions-go)
    - [Anonymous Functions and Closures](#anonymous-functions-and-closures-go)
    - [Methods and Method Values/Expressions](#methods-and-method-valuesexpressions)
    - [Variadic Functions](#variadic-functions-go)
    - [Function Types and First-Class Citizens](#function-types-and-first-class-citizens)
    - [Multiple Return Values and Named Results](#multiple-return-values-and-named-results)
    - [Deferred Functions, Panic, and Recover](#deferred-functions-panic-and-recover)
    - [Init Functions](#init-functions)
    - [Built-in Functions](#built-in-functions)
    - [Generic Functions (Go 1.18+)](#generic-functions-go-1.18)
    - [Memory Layout and Call Architecture (ASCII)](#memory-layout-and-call-architecture-go)
5. [Comparative Summary](#comparative-summary)

---

## Fundamental Concepts
A **function** maps a tuple of arguments to a result, possibly modifying state. Each language gives us different tools to abstract and pass this mapping around.

Key dimensions:
- **Named vs anonymous** – can the function be referred to by an identifier?
- **First‑class vs second‑class** – can it be stored, passed, returned?
- **Closure vs plain** – does it capture an environment?
- **Method vs free function** – is it associated with a type?
- **Linkage / visibility** – static, extern, public/private.
- **Special modifiers** – inline, const, async, unsafe, variadic, etc.

We will explore all of these for each language.

---

## Rust

Rust’s function system is expression‑oriented and heavily integrated with ownership and traits.  
A “function” can mean a **fn item**, a **function pointer**, or a **closure**. They are distinct types.

### Named Functions (fn Items)
```rust
fn add(x: i32, y: i32) -> i32 {
    x + y
}