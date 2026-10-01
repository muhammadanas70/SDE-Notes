# C Syntax Mastery: The Complete Deep-Dive Guide
## From Confusing to Crystal Clear — with Linux Kernel Internals

---

> **Mental Model:** Every confusing C syntax is a *composition of small, consistent rules*.
> The key is to learn each rule in isolation, then understand how they *compose*.
> Once you internalize the grammar of C declarations, nothing will ever look alien again.

---

## TABLE OF CONTENTS

```
PART I   — THE DECLARATION SYSTEM
  01. How C Declarations Actually Work (The Grammar)
  02. The `cdecl` Reading Rule — Clockwise/Spiral Method
  03. Pointers — All Forms, All Contexts
  04. const — What It Actually Qualifies
  05. volatile — The Compiler Fence
  06. restrict — Aliasing Promises
  07. Type Qualifiers in Combination

PART II  — POINTER ARITHMETIC AND MEMORY
  08. Pointer Arithmetic — The Scaling Rule
  09. Array Decay — The Silent Conversion
  10. Multi-Dimensional Arrays and Pointer Equivalence
  11. void* — The Generic Pointer
  12. Pointer to Array vs Array of Pointers
  13. Function Pointers — All Forms
  14. Pointer to Function Returning Pointer

PART III — TYPES AND STORAGE
  15. Storage Classes — static, extern, register, auto
  16. The Five Meanings of static
  17. Linkage — Internal, External, None
  18. typedef — What It Does and Does Not Do
  19. Struct, Union, Enum — Deep Internals
  20. Bit Fields — Layout, Alignment, Traps
  21. Flexible Array Members
  22. Anonymous Structs and Unions (C11)
  23. Designated Initializers (C99)
  24. Compound Literals (C99)

PART IV  — OPERATORS AND EXPRESSIONS
  25. Operator Precedence — The Full Table
  26. Sequence Points and Undefined Behavior
  27. Integer Promotions and Usual Arithmetic Conversions
  28. The Comma Operator
  29. The Ternary Operator — All Edge Cases
  30. sizeof — Traps and Runtime Behavior
  31. _Alignof and _Alignas (C11)
  32. Pre/Post Increment in Complex Expressions

PART V   — PREPROCESSOR AND MACROS
  33. Macros — Token Pasting, Stringification
  34. Variadic Macros
  35. X-Macros — The Linux Kernel Pattern
  36. #pragma, __attribute__, __builtin
  37. include Guards vs #pragma once

PART VI  — ADVANCED TYPE SYSTEM
  38. Variable-Length Arrays (VLA)
  39. Variadic Functions (va_list, va_arg)
  40. Inline Functions and Inline Assembly
  41. Type Punning — Safe and Unsafe Ways
  42. Endianness — Detection and Conversion
  43. Alignment — Padding, Packing, __packed__

PART VII — CONTROL FLOW INTERNALS
  44. goto and Labels — When the Kernel Uses Them
  45. setjmp / longjmp — Non-local Jumps
  46. switch Statement — Duff's Device and Fall-through

PART VIII — LINUX KERNEL C IDIOMS
  47. container_of — The Most Important Macro
  48. likely / unlikely — Branch Prediction Hints
  49. __must_check, __user, __kernel Annotations
  50. RCU — Read-Copy-Update Syntax
  51. Linked List — Linux Kernel Style
  52. Bitwise Operations in Device Drivers
  53. Atomic Operations
  54. Memory Barriers — smp_mb, rmb, wmb
  55. BUILD_BUG_ON and Static Assertions
  56. ARRAY_SIZE and Zero-Length Arrays
```

---

# PART I — THE DECLARATION SYSTEM

---

## 01. How C Declarations Actually Work (The Grammar)

### The Mental Model Before Anything Else

C declarations have a **grammatical structure** that most books never explain clearly.
Every declaration is composed of two parts:

```
[declaration-specifiers]  [declarator]  ;
       |                        |
  "what type"           "what name, and how"
```

**Declaration specifiers** are the base type words:
`int`, `char`, `unsigned long`, `const`, `volatile`, `static`, etc.

**Declarator** is everything that modifies how you *access* that base type:
`*`, `[]`, `()`, and the variable name itself.

```
     DECLARATION SPECIFIERS       DECLARATOR
     ┌─────────────────────┐   ┌─────────────────┐
     │                     │   │                 │
     │  unsigned long int  │   │  * const  arr[] │
     │                     │   │                 │
     └─────────────────────┘   └─────────────────┘
           base type              modifiers + name
```

### The Formal Grammar (Simplified)

```
declaration:
    declaration-specifiers declarator ;

declarator:
    pointer(opt) direct-declarator

direct-declarator:
    identifier
    direct-declarator [ constant-expression(opt) ]    ← array
    direct-declarator ( parameter-type-list )         ← function
    ( declarator )                                    ← grouping

pointer:
    * type-qualifier-list(opt)
    * type-qualifier-list(opt) pointer
```

This grammar is *recursive*. That is why C declarations can nest arbitrarily deep.

---

## 02. The cdecl Reading Rule — Clockwise/Spiral Method

### The Algorithm

To read any C declaration:

```
STEP 1: Find the identifier (variable or function name).
STEP 2: Look RIGHT — read [] or () if present.
STEP 3: Look LEFT  — read * (pointer) or type qualifiers.
STEP 4: If you hit a closing paren ), jump to the matching (
         and repeat steps 2 and 3 inside.
STEP 5: Continue until the base type is consumed.
```

### ASCII Diagram — Reading `int * const (*fp)(void)`

```
     int   *  const  (  *  fp  )  (void)
      │              │  │   │  │    │
      │              │  │   │  │    └── 5: ...taking no args
      │              │  │   │  └─────── 4: jump out of ()
      │              │  │   └────────── 1: START HERE (identifier)
      │              │  └────────────── 3: look left → pointer to...
      │              └───────────────── 4: hit ), go to matching (
      │                                 2: look right → (void) → function
      └───────────────────────────────── 5: returning pointer... to const int

FINAL READING:
fp is a pointer to a function taking no arguments,
returning a const pointer to int.
```

### Practice Decoding Table

```
Declaration                          Reading
─────────────────────────────────────────────────────────────────
int *p                               p is a pointer to int
int *p[10]                           p is an array[10] of pointer to int
int (*p)[10]                         p is a pointer to array[10] of int
int *f()                             f is a function returning pointer to int
int (*f)()                           f is a pointer to function returning int
int (*f[10])()                       f is array[10] of pointer to function returning int
const int *p                         p is a pointer to const int
int * const p                        p is a const pointer to int
const int * const p                  p is a const pointer to const int
int **pp                             pp is a pointer to pointer to int
void (*signal(int, void(*)(int)))(int)   signal is a function... (explained below)
```

### The Most Complex Standard Library Signature: `signal()`

```c
void (*signal(int sig, void (*handler)(int)))(int);
```

Step-by-step decode:

```
         signal
            │
    signal( ... )           → signal is a FUNCTION taking:
                                - int sig
                                - void (*handler)(int)  → pointer to function(int)->void
            │
    (*signal(...))(int)     → returning a POINTER TO function(int)->void
            │
    void (*signal(...))(int) → that function returns void

FINAL: signal is a function taking (int, pointer-to-function(int)->void)
       returning a pointer-to-function(int)->void
```

---

## 03. Pointers — All Forms, All Contexts

### What a Pointer Is — Memory Address Model

```
MEMORY LAYOUT (64-bit system):

Address     Value
─────────────────────────────
0x7fff0000   42      ← int x = 42;
0x7fff0004   ...
0x7fff1000   0x7fff0000  ← int *p = &x;
             │
             └── p stores the ADDRESS of x (8 bytes on 64-bit)
```

```c
#include <stdio.h>

int main(void) {
    int x = 42;
    int *p = &x;        /* p holds the address of x */

    printf("x         = %d\n",  x);      /* 42 */
    printf("&x        = %p\n",  (void*)&x);  /* address of x */
    printf("p         = %p\n",  (void*)p);   /* same as &x */
    printf("*p        = %d\n",  *p);     /* 42  — dereference */
    printf("sizeof(p) = %zu\n", sizeof(p));  /* 8 on 64-bit */
    printf("sizeof(x) = %zu\n", sizeof(x));  /* 4 */

    *p = 100;           /* modifies x through p */
    printf("x is now = %d\n", x);  /* 100 */

    return 0;
}
```

### Pointer Declaration Forms — All Variants

```c
/* ── BASIC POINTER ── */
int *p;               /* p is pointer to int, uninitialized (DANGER) */
int *p = NULL;        /* safe initialization */

/* ── DOUBLE POINTER ── */
int **pp;             /* pointer to pointer to int */

/*
  MEMORY:
  pp ──→ [address1] ──→ [address2] ──→ [int value]
*/

/* ── POINTER TO CONSTANT ── */
const int *p;         /* can NOT modify *p, CAN reassign p */
int const *p;         /* SAME as above — const applies to int */

/* ── CONSTANT POINTER ── */
int * const p = &x;  /* CAN modify *p, can NOT reassign p */
                      /* MUST initialize, like a reference in C++ */

/* ── CONSTANT POINTER TO CONSTANT ── */
const int * const p = &x;  /* neither *p nor p can change */

/* ── POINTER TO FUNCTION ── */
int (*fp)(int, int);  /* fp is pointer to function(int,int)->int */
```

### The Pointer Initialization Triangle

```
Pointer types and what is mutable:

                     Can change       Can change
                     p itself?        *p (data)?
                     ─────────        ──────────
int *p               YES              YES
const int *p         YES              NO
int * const p        NO               YES
const int * const p  NO               NO
```

### Pointers and NULL — Linux Kernel Style

```c
/* Linux kernel never uses NULL directly in conditions this way for clarity */
/* It checks explicitly */

/* ── USERSPACE CONVENTION ── */
if (ptr == NULL) { ... }
if (!ptr)        { ... }   /* same but less explicit */

/* ── LINUX KERNEL CONVENTION ── */
if (ptr == NULL)           /* preferred in kernel for clarity */
    return -EINVAL;

/* IS_ERR / ERR_PTR — kernel error pointers */
/* The kernel uses the last page of address space for error codes */
#define MAX_ERRNO   4095
#define IS_ERR_VALUE(x) ((unsigned long)(x) >= (unsigned long)-MAX_ERRNO)

static inline void *ERR_PTR(long error) {
    return (void *)error;
}
static inline long PTR_ERR(const void *ptr) {
    return (long)ptr;
}
static inline bool IS_ERR(const void *ptr) {
    return IS_ERR_VALUE((unsigned long)ptr);
}

/* USAGE in kernel driver code: */
struct file *f = filp_open("/dev/null", O_RDONLY, 0);
if (IS_ERR(f)) {
    long err = PTR_ERR(f);
    pr_err("Failed to open: %ld\n", err);
    return err;
}
```

---

## 04. const — What It Actually Qualifies

### The Rule: `const` Qualifies What Is On Its Left

```
const int *p    → int is const, p is not
int const *p    → SAME: int is const (const is still left of int's star)
int * const p   → * (the pointer itself) is const
```

### Syntax Position Chart

```
  const  int  *        p
    │     │   │        │
    │     │   │        └── name
    │     │   └─────────── "points to" (indirection)
    │     └─────────────── base type: int
    └───────────────────── THIS qualifies the int

  int  *  const        p
   │   │    │          │
   │   │    │          └── name
   │   │    └──────────── THIS qualifies the pointer p
   │   └──────────────── "points to"
   └──────────────────── base type: int
```

### Real Code — All Four Cases

```c
#include <stdio.h>

int main(void) {
    int a = 10, b = 20;

    /* CASE 1: Non-const pointer to non-const int */
    int *p1 = &a;
    *p1 = 99;    /* OK — modify data */
    p1  = &b;    /* OK — reassign pointer */

    /* CASE 2: Non-const pointer to const int */
    const int *p2 = &a;
    /* *p2 = 99; */   /* ERROR: read-only location */
    p2  = &b;         /* OK — reassign pointer */

    /* CASE 3: Const pointer to non-const int */
    int * const p3 = &a;
    *p3 = 99;    /* OK — modify data */
    /* p3 = &b; */  /* ERROR: assignment of read-only variable */

    /* CASE 4: Const pointer to const int */
    const int * const p4 = &a;
    /* *p4 = 99; */  /* ERROR */
    /* p4  = &b; */  /* ERROR */

    return 0;
}
```

### Linux Kernel const Usage

```c
/* In linux/string.h — const means this function won't modify src */
extern void *memcpy(void *dest, const void *src, size_t count);
/*                                ^^^^^ guarantee: src data is not modified */

/* In driver code — read-only hardware register address */
static void __iomem * const regs = (void __iomem *)0xFE000000;
/*                    ^^^^^ the pointer itself never changes */

/* Function parameter: passing const guarantees caller's data is safe */
int parse_header(const struct ethhdr *hdr) {
    /* hdr->h_proto is readable but we cannot overwrite header data */
    return ntohs(hdr->h_proto);
}
```

---

## 05. volatile — The Compiler Fence

### What volatile Means

Without `volatile`, the compiler can:
- Cache a variable in a register and never re-read memory
- Reorder reads/writes for optimization
- Eliminate "useless" reads that have no visible effect

`volatile` tells the compiler:
> *"Every access to this variable MUST actually touch memory. Do NOT optimize away any read or write."*

```
WITHOUT volatile:                 WITH volatile:
──────────────────────            ─────────────────────────
int x = *hw_reg;                  volatile int *hw_reg = ...;
int y = *hw_reg;                  int x = *hw_reg;  ← actual LOAD
                                  int y = *hw_reg;  ← actual LOAD again
COMPILER might optimize to:       (both loads happen)
  int x = *hw_reg;
  int y = x;         ← BUG!
```

### ASCII Diagram — Hardware Register Access

```
CPU                       Memory Bus                  Peripheral
 ┌─────────────┐           ┌────────┐               ┌──────────────┐
 │   Register  │           │        │               │  Status Reg  │
 │  r0 = ...   │◄─────────►│  BUS   │◄─────────────►│  0xFE004000  │
 └─────────────┘           └────────┘               │  (changes!)  │
                                                     └──────────────┘

Without volatile: compiler reads once, uses cached value → WRONG
With volatile:    compiler emits a new LOAD instruction every time → CORRECT
```

### All Forms of volatile

```c
volatile int x;             /* x is volatile int */
volatile int *p;            /* p points to volatile int */
int * volatile p;           /* p itself is volatile (pointer changes) */
volatile int * volatile p;  /* both p and *p are volatile */
```

### Linux Kernel volatile Usage

```c
/* ── MEMORY-MAPPED I/O ── */
/* linux/io.h: readl/writel use volatile internally */
static inline u32 readl(const volatile void __iomem *addr) {
    u32 val;
    __asm__ __volatile__("" ::: "memory"); /* compiler barrier */
    val = *(const volatile u32 __force *)addr;
    __asm__ __volatile__("" ::: "memory");
    return val;
}

/* ── SPIN LOCK FLAG ── */
typedef struct {
    volatile int slock;  /* must be re-read every time */
} spinlock_t;

/* ── INTERRUPT FLAG ── */
/* This flag is set by an interrupt handler and checked in main code */
static volatile int irq_fired = 0;

void irq_handler(void) {
    irq_fired = 1;  /* interrupt context writes it */
}

void main_loop(void) {
    while (!irq_fired)  /* main code reads it — must not be cached! */
        cpu_relax();
}
```

### volatile vs Memory Barriers

```
volatile is NOT a memory barrier for multi-core ordering.
It only prevents the LOCAL compiler from optimizing.
For multi-core, you need smp_mb(), smp_rmb(), smp_wmb().

volatile:       "don't cache in register"  (single-CPU correctness)
smp_mb():       "order relative to other CPUs" (multi-core ordering)
```

---

## 06. restrict — Aliasing Promises

### The Aliasing Problem

```c
void add(int *a, int *b, int *c) {
    *a = *b + *c;   /* compiler must assume: a, b, c might alias! */
}

/* If called as: add(&x, &x, &x); — they ALL point to x */
/* So compiler cannot reorder or cache — must reload each time */
```

`restrict` is your promise to the compiler:

> *"For the lifetime of this pointer, no other pointer will access the same memory."*

```c
void add(int * restrict a, int * restrict b, int * restrict c) {
    *a = *b + *c;   /* NOW: compiler knows they don't alias */
                    /* CAN optimize: cache *b and *c in registers */
}
```

### Linux Kernel and restrict

```c
/* memcpy is declared with restrict — src and dest MUST NOT overlap */
void *memcpy(void * restrict dest, const void * restrict src, size_t n);

/* If they overlap: use memmove instead (no restrict) */
void *memmove(void *dest, const void *src, size_t n);
```

---

## 07. Type Qualifiers in Combination

### Linux Kernel's `__iomem`, `__user`, `__kernel`

These are *Sparse* annotations — not standard C but used in Linux:

```c
/* __user — pointer to user-space memory (cannot dereference directly in kernel) */
int copy_from_user(void *to, const void __user *from, unsigned long n);

/* __iomem — pointer to memory-mapped I/O (use readl/writel, not *) */
void __iomem *ioremap(phys_addr_t phys_addr, size_t size);

/* Without Sparse, these expand to nothing: */
#ifdef __CHECKER__
# define __user    __attribute__((noderef, address_space(__user)))
# define __iomem   __attribute__((noderef, address_space(__iomem)))
#else
# define __user
# define __iomem
#endif
```

---

# PART II — POINTER ARITHMETIC AND MEMORY

---

## 08. Pointer Arithmetic — The Scaling Rule

### The Golden Rule

When you add an integer `n` to a pointer of type `T*`, the address advances by `n * sizeof(T)` bytes.

```
int arr[5] = {10, 20, 30, 40, 50};
int *p = arr;

p + 0  →  address of arr[0]  (base + 0 * 4)
p + 1  →  address of arr[1]  (base + 1 * 4)
p + 2  →  address of arr[2]  (base + 2 * 4)
p + n  →  address of arr[n]  (base + n * sizeof(int))
```

```
MEMORY (int = 4 bytes):

Address    Value    Pointer
0x100      10       ← p+0  (p)
0x104      20       ← p+1
0x108      30       ← p+2
0x10C      40       ← p+3
0x110      50       ← p+4
```

### Full Demonstration

```c
#include <stdio.h>

int main(void) {
    int arr[5] = {10, 20, 30, 40, 50};
    int *p = arr;    /* p points to arr[0] */

    for (int i = 0; i < 5; i++) {
        printf("p+%d: address=%p, value=%d\n",
               i, (void*)(p+i), *(p+i));
    }

    /* Pointer subtraction: gives number of ELEMENTS between */
    int *start = &arr[0];
    int *end   = &arr[4];
    ptrdiff_t diff = end - start;  /* = 4, not 16 */
    printf("diff = %td elements\n", diff);  /* %td for ptrdiff_t */

    return 0;
}
```

### Pointer Arithmetic with Different Types

```c
char   *pc; pc++;  /* advances by 1 byte  (sizeof char = 1) */
short  *ps; ps++;  /* advances by 2 bytes */
int    *pi; pi++;  /* advances by 4 bytes */
double *pd; pd++;  /* advances by 8 bytes */

struct Big { char data[100]; } *pb;
pb++;              /* advances by 100 bytes! */
```

---

## 09. Array Decay — The Silent Conversion

### What Decay Means

In almost every context, an array name **decays** (converts implicitly) to a pointer to its first element.

```
int arr[5];

arr         →  decays to  →  int * (pointer to arr[0])
&arr[0]     →  same thing
&arr        →  int (*)[5]  (pointer to the WHOLE array — different type!)
```

```
DIAGRAM:

arr = { [0] [1] [2] [3] [4] }
        │
        └── arr (after decay) = &arr[0]

&arr    = pointer to the entire array (same numeric address, different type)
&arr[0] = pointer to first element
```

### The Three Places Where Arrays Do NOT Decay

```c
sizeof(arr)     /* gives total bytes, NOT sizeof(pointer) */
&arr            /* address of whole array, type is int(*)[N] */
_Alignof(arr)   /* alignment of array, not pointer */
```

### Code Proof

```c
#include <stdio.h>

void takes_pointer(int *p) {
    printf("sizeof inside function: %zu\n", sizeof(p)); /* 8 — ptr size */
}

int main(void) {
    int arr[5] = {1, 2, 3, 4, 5};

    printf("sizeof(arr)     = %zu\n", sizeof(arr));     /* 20 — total */
    printf("sizeof(&arr[0]) = %zu\n", sizeof(&arr[0])); /* 8  — pointer */

    takes_pointer(arr);   /* arr decays to int* here */

    /* &arr vs arr: same address, different types */
    printf("arr   = %p\n", (void*)arr);    /* e.g. 0x7fff1000 */
    printf("&arr  = %p\n", (void*)&arr);   /* SAME address */

    /* But arithmetic is DIFFERENT: */
    int (*whole)[5] = &arr;
    printf("arr+1   = %p\n", (void*)(arr+1));      /* +4 bytes  */
    printf("whole+1 = %p\n", (void*)(whole+1));    /* +20 bytes */

    return 0;
}
```

### Linux Kernel: ARRAY_SIZE Macro

```c
/* linux/kernel.h */
/* This ONLY works if arr is a true array, not a decayed pointer */
#define ARRAY_SIZE(arr) (sizeof(arr) / sizeof((arr)[0]))

/* Kernel adds a type-safety check using a BUILD_BUG_ON: */
#define ARRAY_SIZE(arr) \
    (sizeof(arr) / sizeof((arr)[0]) + \
     BUILD_BUG_ON_ZERO(!__builtin_types_compatible_p(typeof(arr), \
                                                      typeof(&(arr)[0]))))
/* __builtin_types_compatible_p returns 0 if types are the same */
/* For array: typeof(arr) = int[5], typeof(&arr[0]) = int* — different ✓ */
/* For pointer: typeof(p) = int*, typeof(&p[0]) = int* — same → BUG! */
```

---

## 10. Multi-Dimensional Arrays and Pointer Equivalence

### Memory Layout — Row-Major Order

C stores multi-dimensional arrays in **row-major** order: all elements of row 0, then all of row 1, etc.

```c
int mat[3][4] = {
    {1,  2,  3,  4},   /* row 0 */
    {5,  6,  7,  8},   /* row 1 */
    {9, 10, 11, 12}    /* row 2 */
};
```

```
MEMORY (contiguous):
┌───┬───┬───┬───┬───┬───┬───┬───┬───┬────┬────┬────┐
│ 1 │ 2 │ 3 │ 4 │ 5 │ 6 │ 7 │ 8 │ 9 │ 10 │ 11 │ 12 │
└───┴───┴───┴───┴───┴───┴───┴───┴───┴────┴────┴────┘
  ↑               ↑               ↑
  mat[0]          mat[1]          mat[2]
  (row 0)         (row 1)         (row 2)
```

### Type of `mat`, `mat[i]`, `mat[i][j]`

```
Expression      Type                  Description
────────────────────────────────────────────────────────────
mat             int (*)[4]            pointer to array[4] of int (after decay)
mat[i]          int *                 pointer to int (row i, after decay)
mat[i][j]       int                   actual integer value
&mat            int (*)[3][4]         pointer to whole 2D array
&mat[i]         int (*)[4]            pointer to row array
&mat[i][j]      int *                 pointer to one element
```

### Passing 2D Array to Function

```c
/* METHOD 1: Fixed-size inner dimension (most common) */
void print_matrix(int mat[][4], int rows) {
    for (int i = 0; i < rows; i++)
        for (int j = 0; j < 4; j++)
            printf("%d ", mat[i][j]);
}
/* mat[][4] is really int (*mat)[4] — pointer to row of 4 ints */

/* METHOD 2: Explicit pointer type */
void print_matrix2(int (*mat)[4], int rows) {
    /* same as method 1 — mat is pointer to array[4] of int */
}

/* METHOD 3: VLA (C99) — any dimensions */
void print_matrix3(int rows, int cols, int mat[rows][cols]) {
    for (int i = 0; i < rows; i++)
        for (int j = 0; j < cols; j++)
            printf("%d ", mat[i][j]);
}

/* METHOD 4: Flatten to 1D pointer (C-style hack) */
void print_matrix4(int *mat, int rows, int cols) {
    for (int i = 0; i < rows; i++)
        for (int j = 0; j < cols; j++)
            printf("%d ", mat[i * cols + j]);
}
```

---

## 11. void* — The Generic Pointer

### Rules of void*

```
void* is special:
1. Can receive ANY pointer type without a cast (in C, not C++)
2. Can be assigned back to any pointer type without a cast (in C)
3. Cannot be dereferenced directly — must cast first
4. Cannot perform pointer arithmetic on it
```

```c
#include <stdlib.h>

int main(void) {
    int x = 42;
    void *v = &x;        /* any pointer → void*, no cast needed in C */

    int *p = v;          /* void* → any pointer, no cast needed in C */
    /* double *d = v; */ /* legal in C, undefined behavior at runtime */

    /* *v = 10; */       /* ERROR: cannot dereference void* */

    /* malloc returns void* */
    int *arr = malloc(10 * sizeof(int));   /* no cast in C */

    /* Kernel uses void* for generic parameters */
    return 0;
}
```

### Linux Kernel void* Usage

```c
/* linux/workqueue.h */
struct work_struct {
    /* ... */
};

/* The work callback: data is void* so any struct can be embedded */
typedef void (*work_func_t)(struct work_struct *work);

/* The container_of pattern (explained later) lets you get */
/* your struct back from the embedded work_struct pointer  */
```

---

## 12. Pointer to Array vs Array of Pointers

### The Critical Distinction

```c
int *arr[10];      /* array of 10 pointers to int */
int (*arr)[10];    /* pointer to (one) array of 10 ints */
```

```
int *arr[10]  — Array of 10 pointers:

arr[0] ──→ int value
arr[1] ──→ int value
arr[2] ──→ int value
...
arr[9] ──→ int value

Each pointer independently points somewhere.
```

```
int (*arr)[10]  — One pointer, pointing to a 10-element array:

arr ──→ [ int, int, int, int, int, int, int, int, int, int ]
                 (10 ints laid out contiguously)

Incrementing arr moves past all 10 ints at once.
```

### Real Code

```c
#include <stdio.h>

int main(void) {
    /* ── ARRAY OF POINTERS ── */
    int a = 1, b = 2, c = 3;
    int *ptrs[3] = {&a, &b, &c};   /* 3 pointers */
    printf("%d %d %d\n", *ptrs[0], *ptrs[1], *ptrs[2]);  /* 1 2 3 */

    /* ── POINTER TO ARRAY ── */
    int row[5] = {10, 20, 30, 40, 50};
    int (*parr)[5] = &row;      /* parr points to the array */
    printf("%d\n", (*parr)[2]); /* 30 */

    /* Useful for 2D arrays: */
    int matrix[3][5] = {{1,2,3,4,5},{6,7,8,9,10},{11,12,13,14,15}};
    int (*rowptr)[5] = matrix;   /* points to first row */
    printf("%d\n", rowptr[1][2]);  /* 8 — second row, third col */

    return 0;
}
```

---

## 13. Function Pointers — All Forms

### Declaration Anatomy

```
int  (*fp)  (int a, int b)
│     │ │    └───────────── parameter types
│     │ └────────────────── name: fp
│     └──────────────────── * means "pointer to function"
└────────────────────────── return type: int

READ: fp is a pointer to a function taking (int, int) and returning int
```

### All Forms of Function Pointer Usage

```c
#include <stdio.h>

/* ── DEFINE A FUNCTION ── */
int add(int a, int b) { return a + b; }
int sub(int a, int b) { return a - b; }
int mul(int a, int b) { return a * b; }

/* ── TYPEDEF for cleaner syntax ── */
typedef int (*BinaryOp)(int, int);

int main(void) {
    /* Direct declaration */
    int (*fp)(int, int);

    /* Assign */
    fp = add;       /* no & needed — function name decays to pointer */
    fp = &add;      /* also valid, explicit */

    /* Call — both forms work */
    int r1 = fp(3, 4);    /* implicit dereference */
    int r2 = (*fp)(3, 4); /* explicit dereference — same result */
    printf("%d %d\n", r1, r2);  /* 7 7 */

    /* Array of function pointers */
    BinaryOp ops[3] = {add, sub, mul};
    for (int i = 0; i < 3; i++)
        printf("%d\n", ops[i](10, 3));  /* 13, 7, 30 */

    return 0;
}
```

### Function Pointer as Parameter — qsort

```c
#include <stdlib.h>

/* qsort's comparator: function pointer parameter */
/* void qsort(void *base, size_t n, size_t size, */
/*            int (*compar)(const void *, const void *)); */

int compare_ints(const void *a, const void *b) {
    /* Cast from void* to int* then dereference */
    int ia = *(const int *)a;
    int ib = *(const int *)b;
    return (ia > ib) - (ia < ib);  /* branchless comparison */
}

int main(void) {
    int arr[] = {5, 2, 8, 1, 9, 3};
    qsort(arr, 6, sizeof(int), compare_ints);
    /* arr is now: 1, 2, 3, 5, 8, 9 */
}
```

### Linux Kernel — Function Pointers in Structs (vtable pattern)

```c
/* linux/fs.h — the virtual filesystem operations table */
struct file_operations {
    struct module *owner;
    loff_t  (*llseek)  (struct file *, loff_t, int);
    ssize_t (*read)    (struct file *, char __user *, size_t, loff_t *);
    ssize_t (*write)   (struct file *, const char __user *, size_t, loff_t *);
    int     (*open)    (struct inode *, struct file *);
    int     (*release) (struct inode *, struct file *);
    /* ... 30+ more operations */
};

/* A driver implements only what it needs: */
static struct file_operations my_fops = {
    .owner   = THIS_MODULE,
    .open    = my_open,
    .read    = my_read,
    .release = my_release,
    /* unimplemented → NULL → kernel checks and uses defaults */
};

/*
STRUCTURE IN MEMORY:

my_fops:
┌────────────┬──────────┐
│ .owner     │ ptr → module  │
├────────────┼──────────┤
│ .llseek    │ NULL          │  (not implemented)
├────────────┼──────────┤
│ .read      │ ptr → my_read │
├────────────┼──────────┤
│ .write     │ NULL          │
├────────────┼──────────┤
│ .open      │ ptr → my_open │
└────────────┴──────────┘
*/
```

---

## 14. Pointer to Function Returning Pointer

### The Hardest Form — Decoded

```c
int *(*fp)(int, char *);
```

Read it:
```
fp          → name
(*fp)       → pointer to function
(*fp)(int, char *) → taking (int, char*)
int *(*fp)(int, char *)  → returning int*

FINAL: fp is a pointer to a function(int, char*) returning int*
```

### Returning Function Pointer From Function

```c
/* Function that returns a function pointer */
int (*get_op(char op))(int, int) {
    if (op == '+') return add;
    if (op == '-') return sub;
    return NULL;
}

/* With typedef — MUCH cleaner */
typedef int (*BinOp)(int, int);

BinOp get_op_clean(char op) {
    if (op == '+') return add;
    if (op == '-') return sub;
    return NULL;
}
```

### signal() — The Canonical Complex Declaration

```c
/* Standard library: */
void (*signal(int signum, void (*handler)(int)))(int);

/* Equivalent with typedef — how it's often written in practice: */
typedef void (*sighandler_t)(int);
sighandler_t signal(int signum, sighandler_t handler);

/* Usage: */
#include <signal.h>

void my_handler(int sig) {
    /* handle signal */
}

int main(void) {
    signal(SIGINT, my_handler);   /* register handler */
    signal(SIGINT, SIG_DFL);      /* restore default */
    signal(SIGINT, SIG_IGN);      /* ignore signal */
}
```

---

# PART III — TYPES AND STORAGE

---

## 15. Storage Classes — static, extern, register, auto

### The Four Storage Classes

```
Storage Class    Lifetime           Scope           Linkage
─────────────────────────────────────────────────────────────
auto             block              local           none
static           program            local/file      internal
extern           program            declaration     external
register         block              local           none
```

### auto — The Default (Rarely Written)

```c
int main(void) {
    auto int x = 5;  /* explicit — but identical to: int x = 5; */
    /* stored on stack, destroyed when block exits */
}
```

### register — Hint to Compiler

```c
/* HISTORICAL: suggest variable be kept in CPU register */
/* Modern: compilers ignore this, but it does two things: */
/* 1. You cannot take the address of a register variable (&x is error) */
/* 2. Prevents aliasing — optimizer may benefit */

for (register int i = 0; i < n; i++) { /* classic idiom */ }

/* Linux kernel: __registers__ in assembly code is different */
```

---

## 16. The Five Meanings of static

This is one of the most confusing keywords in C because it means different things depending on context.

```
Context                          Meaning
──────────────────────────────────────────────────────────────────
static local variable            Persists across function calls
static global variable           File-scope only (internal linkage)
static function                  File-scope only (internal linkage)
static struct member             Does NOT exist in C (only C++)
static in array parameter        Minimum element count hint
```

### Meaning 1: Static Local Variable — Persistent State

```c
#include <stdio.h>

int counter(void) {
    static int count = 0;  /* initialized ONCE, lives forever */
    count++;               /* preserved between calls */
    return count;
}

int main(void) {
    printf("%d\n", counter());  /* 1 */
    printf("%d\n", counter());  /* 2 */
    printf("%d\n", counter());  /* 3 */
    /* count is NOT on the stack — it's in BSS/data segment */
}
```

```
MEMORY LAYOUT:

Stack:               BSS/Data Segment:
┌──────────┐         ┌─────────────────┐
│  main()  │         │ count = 0 → 1   │ ← static local
│          │         │ (persists!)     │
└──────────┘         └─────────────────┘
```

### Meaning 2: Static Global Variable — Internal Linkage

```c
/* file: module_a.c */
static int hidden = 42;       /* ONLY visible in this file */
static void helper(void) { }  /* ONLY callable from this file */

/* file: module_b.c */
extern int hidden;  /* ERROR at link time — hidden is not exported */
```

```
LINKAGE DIAGRAM:

module_a.c            module_b.c
┌──────────────┐      ┌──────────────┐
│ static int   │      │ extern int   │
│ hidden = 42  │      │ hidden;  ✗   │   LINKER ERROR
│              │      │              │
│ (stays here) │      │              │
└──────────────┘      └──────────────┘
```

### Meaning 3: Static Function — Encapsulation

```c
/* linux kernel pattern: helper functions are always static */
static int parse_value(const char *str) {
    /* only callable from this .c file */
    /* prevents symbol table pollution */
    /* allows compiler to inline aggressively */
}
```

### Meaning 4: static in Array Parameter (C99)

```c
/* Tells compiler: caller guarantees at least 10 elements */
void process(int arr[static 10]) {
    /* compiler can assume arr != NULL and has ≥10 elements */
    /* allows prefetch optimization */
}
/* NOT related to storage duration! */
```

### Linux Kernel static Pattern

```c
/* Every driver function is static unless exported via EXPORT_SYMBOL */

static int __init my_driver_init(void) {
    /* __init: placed in .init section, freed after boot */
    return 0;
}

static void __exit my_driver_exit(void) {
    /* __exit: only compiled for modules, not built-in */
}

module_init(my_driver_init);
module_exit(my_driver_exit);
```

---

## 17. Linkage — Internal, External, None

### The Three Kinds of Linkage

```
LINKAGE         MEANING                          HOW TO GET IT
────────────────────────────────────────────────────────────────
External        Visible across all .c files       global variable/function
                (linked together)                (no static keyword)

Internal        Visible only within one .c file   static global/function

None            No linkage — local only            local variables, params
```

```
MULTI-FILE LINKAGE DIAGRAM:

main.c              util.c              math.c
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ int g = 5;   │    │ extern int g;│    │ extern int g;│
│ (external)   │◄───┤ (sees g)     │    │ (sees g)     │
│              │    │              │    │              │
│ static int h;│    │ extern int h;│    │              │
│ (internal)   │    │ LINKER ERROR │    │              │
│              │    │              │    │              │
│ void foo(){}  │    │ foo();       │    │ foo();       │
│ (external)   │◄───┤ (can call)   │◄───┤ (can call)   │
│              │    │              │    │              │
│ static void  │    │ bar();       │    │              │
│ bar(){}       │    │ LINKER ERROR │    │              │
└──────────────┘    └──────────────┘    └──────────────┘
        │                   │                   │
        └───────────────────┴───────────────────┘
                        LINKER
```

### extern — The Declaration vs Definition Rule

```c
/* DEFINITION — allocates storage (only once, in one .c file) */
int global_count = 0;

/* DECLARATION — just tells compiler "this exists somewhere" */
extern int global_count;   /* no allocation, just a reference */

/* RULE: one definition, many declarations */
/* header.h should have: extern int global_count; */
/* one .c file should have: int global_count = 0; */
```

---

## 18. typedef — What It Does and Does Not Do

### typedef Creates an ALIAS, Not a New Type

```c
typedef unsigned long int ulong;
/* ulong is now an alias for "unsigned long int" */
/* They are completely interchangeable */

ulong x = 42;
unsigned long int y = 42;
/* x and y have the SAME type */
```

### typedef With Pointers — The Trap

```c
typedef int *IntPtr;

IntPtr p, q;
/* THIS IS: int *p, *q; — BOTH are pointers */
/* NOT: int *p, q;  */

/* COMPARE to macro: */
#define PTR int *
PTR p, q;
/* THIS IS: int *p, q; — p is pointer, q is int! TRAP! */
/* typedef is SAFER than #define for types */
```

### typedef for Structs — Linux Kernel Style

```c
/* Without typedef (Linux kernel preferred style for structs): */
struct task_struct {
    pid_t pid;
    char comm[16];
    /* ... */
};
struct task_struct *current;  /* must write "struct" each time */

/* With typedef (used for opaque types): */
typedef struct {
    spinlock_t lock;
    int count;
} atomic_t;

atomic_t a;   /* no "struct" needed */

/* Linux kernel uses typedef for: */
/* - u8, u16, u32, u64, s8, s16, s32, s64 (linux/types.h) */
/* - atomic_t, spinlock_t, mutex (opaque — internals hidden) */
/* NOT for: regular structs (task_struct, inode, etc.) */
```

### typedef for Function Pointers — Critical Pattern

```c
/* Without typedef — verbose and error-prone */
int (*register_callback(void (*cb)(int, void*), void *data))(int);

/* With typedef — clean and maintainable */
typedef void (*callback_fn)(int event, void *data);
typedef int  (*register_fn)(callback_fn cb, void *data);

/* Kernel interrupt handler typedef */
typedef irqreturn_t (*irq_handler_t)(int irq, void *dev_id);
```

---

## 19. Struct, Union, Enum — Deep Internals

### Struct Memory Layout

```c
struct Example {
    char  a;      /* 1 byte */
    int   b;      /* 4 bytes */
    char  c;      /* 1 byte */
    short d;      /* 2 bytes */
};
```

```
WITHOUT any packing (default alignment):

Offset  Bytes  Field
──────────────────────────────────────────
0       1      a  (char)
1       3      padding (to align b to 4-byte boundary)
4       4      b  (int)
8       1      c  (char)
9       1      padding (to align d to 2-byte boundary)
10      2      d  (short)
12      4      trailing padding (struct size = 16, aligned to 4)

Total: 16 bytes  ← not 1+4+1+2 = 8!

┌──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┐
│a │P │P │P │  b (4B)   │c │P │ d(2B) │  PADDING  │
└──┴──┴──┴──┴──┴──┴──┴──┴──┴──┴──┴──┴──┴──┴──┴──┘
 0  1  2  3  4  5  6  7  8  9  10 11 12 13 14 15
```

### Reordering Fields — Optimization

```c
/* Optimized layout — largest first */
struct Optimized {
    int   b;      /* 4 bytes at offset 0 */
    short d;      /* 2 bytes at offset 4 */
    char  a;      /* 1 byte  at offset 6 */
    char  c;      /* 1 byte  at offset 7 */
};
/* Total: 8 bytes — same data, half the size! */

┌──┬──┬──┬──┬──┬──┬──┬──┐
│  b (4 bytes)  │d(2B)│a │c │
└──┴──┴──┴──┴──┴──┴──┴──┘
 0  1  2  3  4  5  6  7
```

### Union — All Members Share Same Memory

```c
union Value {
    int    i;       /* 4 bytes */
    float  f;       /* 4 bytes */
    char   bytes[4]; /* 4 bytes */
};
/* Total: 4 bytes — the MAX of all members */
/* All members START at offset 0 */

union Value v;
v.i = 0x41424344;
/* Now v.bytes = {'D','C','B','A'} on little-endian */
/* And v.f = some float with those bit patterns */

MEMORY:
┌──────────────────┐
│  v.i    (4 bytes)│
│  v.f    (4 bytes)│  ← ALL OVERLAID AT SAME ADDRESS
│  v.bytes(4 bytes)│
└──────────────────┘
  offset 0
```

### Union for Type Punning (Reading Bits)

```c
#include <stdint.h>

/* Read float bits as integer without undefined behavior (C99 allows this) */
union FloatBits {
    float    f;
    uint32_t bits;
};

float value = 3.14f;
union FloatBits fb;
fb.f = value;
printf("float bits: 0x%08X\n", fb.bits);
/* 0x4048F5C3 */
```

### Linux Kernel Union Usage — skb (Socket Buffer)

```c
/* linux/skbuff.h — the network packet structure */
struct sk_buff {
    /* ... */
    union {
        __be16  inner_protocol;    /* for tunnel encapsulation */
        __u8    inner_ipproto;
    };
    /* ... */
    union {
        struct {
            unsigned long _skb_refdst;
            void (*destructor)(struct sk_buff *skb);
        };
        struct list_head tcp_tsorted_anchor;
    };
};
/* Anonymous union: access members directly as skb->inner_protocol */
```

### Enum — Named Constants

```c
/* enum creates integer constants */
enum Color {
    RED   = 0,    /* explicit value */
    GREEN = 1,
    BLUE  = 2,
};
/* Underlying type is int */

/* Without explicit values: starts at 0, increments by 1 */
enum Direction {
    NORTH,  /* 0 */
    EAST,   /* 1 */
    SOUTH,  /* 2 */
    WEST,   /* 3 */
};

/* Linux kernel enum style — ALL_CAPS, descriptive prefix */
enum hrtimer_restart {
    HRTIMER_NORESTART,
    HRTIMER_RESTART,
};
```

---

## 20. Bit Fields — Layout, Alignment, Traps

### What Bit Fields Are

Bit fields let you pack multiple values into a single integer.

```c
struct Flags {
    unsigned int read    : 1;  /* 1 bit */
    unsigned int write   : 1;  /* 1 bit */
    unsigned int execute : 1;  /* 1 bit */
    unsigned int         : 5;  /* 5 unnamed padding bits */
};
/* Total: 8 bits = 1 byte (stored in unsigned int = 4 bytes with padding) */
```

```
BIT LAYOUT (little-endian, typical):
bit:  7  6  5  4  3  2  1  0
      ─────────────────────────
      unused(5 bits)  │x │w │r│
                      └──┴──┴─┘
                      exe wri read
```

### Bit Field Rules and Traps

```c
struct BitExample {
    unsigned int a : 3;   /* values 0..7 */
    unsigned int b : 5;   /* values 0..31 */
    unsigned int   : 0;   /* force next field to new storage unit */
    unsigned int c : 4;   /* starts fresh */
};

/* TRAPS:
   1. Cannot take address of a bit field (&s.a is ERROR)
   2. Layout is implementation-defined (endianness, padding)
   3. signed bit fields have sign-extension issues:
*/
struct Signed {
    signed int x : 3;   /* values -4..3, NOT 0..7 */
};
struct Signed s;
s.x = 7;  /* WRAPS to -1 on most implementations */
```

### Linux Kernel Bit Fields — TCP Header

```c
/* linux/tcp.h */
struct tcphdr {
    __be16  source;
    __be16  dest;
    __be32  seq;
    __be32  ack_seq;
#if defined(__LITTLE_ENDIAN_BITFIELD)
    __u16   res1:4, doff:4, fin:1, syn:1, rst:1,
            psh:1, ack:1, urg:1, ece:1, cwr:1;
#elif defined(__BIG_ENDIAN_BITFIELD)
    __u16   doff:4, res1:4, cwr:1, ece:1, urg:1,
            ack:1, psh:1, rst:1, syn:1, fin:1;
#else
#error "Adjust your <asm/byteorder.h> defines"
#endif
    __be16  window;
    __sum16 check;
    __be16  urg_ptr;
};
```

---

## 21. Flexible Array Members (C99)

### What They Are

A struct can end with an array of unspecified size. The array size is determined at allocation time.

```c
struct Message {
    int  length;       /* fixed header */
    char data[];       /* flexible array — NO size specified */
};
/* sizeof(struct Message) = sizeof(int) = 4 */
/* data[] contributes 0 to sizeof */
```

### Usage Pattern

```c
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

struct Message *create_message(const char *text) {
    size_t text_len = strlen(text) + 1;
    /* Allocate struct + data in one shot */
    struct Message *msg = malloc(sizeof(*msg) + text_len);
    if (!msg) return NULL;
    msg->length = text_len;
    memcpy(msg->data, text, text_len);
    return msg;
}

int main(void) {
    struct Message *m = create_message("Hello, World!");
    printf("len=%d, data=%s\n", m->length, m->data);
    free(m);
}
```

```
MEMORY LAYOUT:

malloc(sizeof(Message) + 14):
┌───────────────┬──────────────────────────────────┐
│  length (4B)  │  data[14]: "Hello, World!\0"     │
└───────────────┴──────────────────────────────────┘
   fixed part         flexible part
```

### Linux Kernel Flexible Arrays

```c
/* linux/netlink.h */
struct nlmsghdr {
    __u32 nlmsg_len;
    __u16 nlmsg_type;
    __u16 nlmsg_flags;
    __u32 nlmsg_seq;
    __u32 nlmsg_pid;
};

/* Message data follows the header — accessed via NLMSG_DATA macro */
#define NLMSG_DATA(nlh)  ((void *)(((char *)nlh) + NLMSG_HDRLEN))
```

---

## 22. Anonymous Structs and Unions (C11)

### What Anonymous Means

A struct or union with no tag and no member name — its members are accessed as if they belong to the enclosing struct.

```c
struct Point3D {
    union {              /* anonymous union — no tag, no name */
        struct {         /* anonymous struct inside */
            float x, y;  /* accessed as p.x, p.y */
        };
        float xy[2];     /* accessed as p.xy[0], p.xy[1] */
    };
    float z;             /* accessed as p.z */
};

struct Point3D p = {.x = 1.0f, .y = 2.0f, .z = 3.0f};
printf("%f %f %f\n", p.x, p.y, p.z);
printf("%f %f\n", p.xy[0], p.xy[1]);  /* same data, different access */
```

### Linux Kernel Anonymous Usage

```c
/* linux/atomic.h */
typedef struct {
    union {
        atomic_long_t a;
        struct {
#ifdef CONFIG_64BIT
            long counter;
#else
            u32 lo, hi;
#endif
        };
    };
} atomic64_t;
```

---

## 23. Designated Initializers (C99)

### The Problem They Solve

Before C99, you had to initialize struct members in order. Designated initializers let you specify by name.

```c
struct Config {
    int   width;
    int   height;
    float scale;
    int   enabled;
    char  name[32];
};

/* OLD WAY (C89) — fragile, order-dependent */
struct Config c1 = {640, 480, 1.0f, 1, "default"};

/* NEW WAY (C99) — explicit, order-independent, unset fields → 0 */
struct Config c2 = {
    .width   = 640,
    .height  = 480,
    .scale   = 1.0f,
    .enabled = 1,
    .name    = "default",
};

/* Partial initialization — unspecified fields are ZERO */
struct Config c3 = {
    .enabled = 1,
    /* width=0, height=0, scale=0.0f, name="" */
};
```

### Array Designated Initializers

```c
int arr[10] = {
    [0] = 100,
    [5] = 200,
    [9] = 300,
    /* all others = 0 */
};

/* Sparse arrays */
int sparse[1000] = {
    [0]   = 1,
    [999] = 1,
    /* 998 zeros in between */
};
```

### Linux Kernel Designated Initializers

```c
/* linux/fs.h — every driver uses designated initializers */
static const struct file_operations proc_file_fops = {
    .open    = proc_file_open,
    .read    = seq_read,
    .llseek  = seq_lseek,
    .release = single_release,
    /* .write, .poll, .ioctl, etc. → NULL (not implemented) */
};

/* Interrupt request descriptor */
static struct irqaction timer_irqaction = {
    .handler   = timer_interrupt,
    .flags     = IRQF_DISABLED | IRQF_NOBALANCING | IRQF_IRQPOLL | IRQF_TIMER,
    .name      = "timer",
};
```

---

## 24. Compound Literals (C99)

### What They Are

A compound literal creates an unnamed object of a given type, inline.

```c
/* Syntax: (type){ initializer } */

/* Pointer to a temporary struct */
struct Point *p = &(struct Point){.x = 1, .y = 2};

/* Pass a struct literal directly to a function */
draw_rect((struct Rect){.x=0, .y=0, .w=100, .h=50});

/* Array literal */
int *arr = (int[]){1, 2, 3, 4, 5};
```

### Lifetime of Compound Literals

```c
int *get_array(void) {
    /* DANGER: compound literal has block scope */
    return (int[]){1, 2, 3};  /* dangling pointer after function returns! */
}

/* SAFE: at file scope — static duration */
int *global_arr = (int[]){1, 2, 3};  /* lives for program duration */
```

### Linux Kernel Usage

```c
/* Passing configuration inline */
platform_device_register(
    &(struct platform_device){
        .name = "my_device",
        .id   = -1,
        .dev  = {
            .platform_data = &my_platform_data,
        },
    }
);
```

---

# PART IV — OPERATORS AND EXPRESSIONS

---

## 25. Operator Precedence — The Full Table

### The Complete Precedence Table (High to Low)

```
Prec  Operator(s)                      Associativity   Category
────────────────────────────────────────────────────────────────
 15   () [] -> .                        Left→Right      Postfix
 14   ++ -- (postfix)                   Left→Right      Postfix
 13   ++ -- (prefix) + - ! ~ * & sizeof (type) _Alignof
                                        Right→Left      Prefix/Unary
 12   * / %                             Left→Right      Multiplicative
 11   + -                               Left→Right      Additive
 10   << >>                             Left→Right      Shift
  9   < <= > >=                         Left→Right      Relational
  8   == !=                             Left→Right      Equality
  7   &                                 Left→Right      Bitwise AND
  6   ^                                 Left→Right      Bitwise XOR
  5   |                                 Left→Right      Bitwise OR
  4   &&                                Left→Right      Logical AND
  3   ||                                Left→Right      Logical OR
  2   ?:                                Right→Left      Ternary
  1   = += -= *= /= %= &= ^= |= <<= >>= Right→Left      Assignment
  0   ,                                 Left→Right      Comma
```

### Critical Precedence Traps

```c
/* TRAP 1: & and == */
if (x & MASK == 0)         /* PARSED AS: x & (MASK == 0) — WRONG! */
if ((x & MASK) == 0)       /* CORRECT */

/* TRAP 2: -> vs * (dereference) */
*ptr->field    /* parsed as: *(ptr->field)  — dereference ptr->field */
(*ptr).field   /* dereference ptr first, then .field */

/* TRAP 3: sizeof and cast */
sizeof(int) * p    /* parsed as: (sizeof(int)) * p — correct intent */
sizeof int * p     /* ERROR: sizeof needs parens for type argument */
sizeof *p          /* sizeof the thing p points to — correct! */

/* TRAP 4: shift and arithmetic */
x = 1 << 3 + 1;   /* parsed as: 1 << (3+1) = 1<<4 = 16 */
                   /* NOT: (1<<3)+1 = 9 */

/* TRAP 5: postfix ++ vs prefix ++ */
*p++   /* parsed as: *(p++) — advance pointer, dereference old */
(*p)++ /* dereference p, then increment the VALUE */
++*p   /* increment the VALUE at p */
```

---

## 26. Sequence Points and Undefined Behavior

### What Is a Sequence Point?

A sequence point is a point in execution where all side effects of previous expressions are complete and no side effects of subsequent expressions have begun.

```
Sequence points in C:
1. End of a full expression (;)
2. After the left operand of && and || (short-circuit points)
3. After the condition of ?: 
4. After each argument evaluation in a function call (not ordered relative to each other)
5. At the , operator
6. Entry to and exit from a function call
```

### Undefined Behavior — Modifying Twice Between Sequence Points

```c
/* ALL OF THESE ARE UNDEFINED BEHAVIOR */

int i = 5;
i = i++;           /* UB: i modified twice, both sides use i */
i = ++i + i++;     /* UB: multiple modifications */
int a = i++ + i++; /* UB: which i++ happens first? */

/* Function arguments — ORDER OF EVALUATION IS UNSPECIFIED */
int f(int a, int b);
f(i++, i++);       /* UNSPECIFIED: which i++ first? (but not UB in C11+) */

/* SAFE versions */
int tmp = i++;
int result = tmp + i;   /* safe: one modification per expression */
```

### Linux Kernel and Sequence Points

```c
/* Kernel is very careful about this — uses explicit temporaries */

/* BAD (potential UB): */
*dst++ = *src++;   /* actually DEFINED — ptr increments are distinct objects */

/* CLEAR kernel style: */
do {
    *dst = *src;
    dst++;
    src++;
} while (--count);
```

---

## 27. Integer Promotions and Usual Arithmetic Conversions

### Integer Promotion Rule

In any arithmetic expression, types smaller than `int` are promoted to `int` first.

```
char   → int
short  → int
        (if int can represent all values of the type)
        (otherwise: unsigned int)
```

### The Usual Arithmetic Conversions (UAC)

When two different arithmetic types meet in an operation:

```
RULE (simplified, in order):
1. If either is long double → other becomes long double
2. If either is double      → other becomes double
3. If either is float       → other becomes float
4. Apply integer promotions to both
5. If same type → done
6. If both signed or both unsigned → smaller type converts to larger
7. If unsigned rank ≥ signed rank → signed converts to unsigned
8. If signed can represent all values of unsigned → unsigned converts to signed
9. Otherwise → both convert to unsigned version of signed type
```

### The Classic Bug: unsigned Comparison

```c
#include <stdio.h>

int main(void) {
    int x = -1;
    unsigned int y = 1;

    /* UAC: int (-1) → unsigned int (0xFFFFFFFF on 32-bit = 4294967295) */
    if (x < y)                          /* FALSE! -1 becomes huge unsigned */
        printf("x < y\n");
    else
        printf("x >= y\n");  /* prints this — surprising! */

    /* SAFER: explicit cast */
    if (x < (int)y)
        printf("x < y (correct)\n");  /* prints this */

    return 0;
}
```

### Promotion in Practice

```c
char a = 200, b = 100;
char result = a + b;       /* a and b promoted to int first */
                           /* 200 + 100 = 300 → truncated to char = 44 */

unsigned char uc = 255;
int i = uc + 1;            /* uc promoted to int: 255 + 1 = 256 (no overflow) */

/* KERNEL TRAP: */
u8 reg_val = read_reg();   /* read 8-bit hardware register */
if (reg_val - 1 > 0) {    /* reg_val promoted to int for subtraction */
    /* This ALWAYS true even if reg_val == 0 */
    /* 0 - 1 = -1 (int), and -1 > 0 is false... actually okay here */
    /* but with unsigned comparison it would wrap */
}
```

---

## 28. The Comma Operator

### What It Does

The comma operator evaluates its left operand (for side effects), discards the result, then evaluates and returns the right operand.

```c
int x = (3 + 4, 10 * 2);   /* x = 20 — left side (7) discarded */
int y = (printf("hello\n"), 42);  /* prints, then y = 42 */
```

### The Key Use: for loop with multiple expressions

```c
for (int i = 0, j = 10; i < j; i++, j--) {
/*                                ↑      */
/*         comma operator: both i++ and j-- execute */
    printf("i=%d j=%d\n", i, j);
}
```

### Comma in Macros — The Trick

```c
/* Multi-statement macro using comma: */
#define SWAP(a, b, type) \
    ((type)__tmp = (a), (a) = (b), (b) = __tmp)
/* Evaluates all three, returns last result */
```

---

## 29. The Ternary Operator — All Edge Cases

### Syntax

```
condition ? expression_if_true : expression_if_false
```

### Type Rules — Both Branches Must Be Compatible

```c
int x = condition ? 1 : 2;           /* both int → int result */
double d = condition ? 1 : 2.0;      /* int 1 promoted to double */
void *p = condition ? malloc(n) : NULL; /* both convertible to void* */

/* TRAP: different types */
int *p1 = ...;
double *p2 = ...;
void *r = condition ? p1 : p2;   /* ERROR in C: incompatible pointers */
```

### Lvalue Ternary (GNU Extension)

```c
/* GNU C allows ternary as lvalue: */
(condition ? a : b) = 10;   /* assigns to a or b depending on condition */
```

### Nested Ternary — Readable with Indentation

```c
const char *grade =
    score >= 90 ? "A" :
    score >= 80 ? "B" :
    score >= 70 ? "C" :
    score >= 60 ? "D" :
                  "F";
/* Right-associative: score>=90 ? "A" : (score>=80 ? "B" : (...)) */
```

### Linux Kernel Ternary Usage

```c
/* Kernel uses ternary for concise error handling */
return IS_ERR(ptr) ? PTR_ERR(ptr) : 0;

/* Selecting between two values */
size_t chunk = min(remaining, (size_t)PAGE_SIZE);
/* min() is a macro: */
#define min(x, y) ((x) < (y) ? (x) : (y))
```

---

## 30. sizeof — Traps and Runtime Behavior

### sizeof Is a Compile-Time Operator (Usually)

```c
sizeof(int)           /* 4 on most platforms */
sizeof(double)        /* 8 */
sizeof(void *)        /* 4 on 32-bit, 8 on 64-bit */
sizeof(struct S)      /* depends on padding */
sizeof "hello"        /* 6: includes null terminator */
sizeof(char[6])       /* 6 */
```

### sizeof on Expressions vs Types

```c
int x = 42;
sizeof x        /* 4 — parentheses optional for expressions */
sizeof(x)       /* 4 — same */
sizeof(int)     /* 4 — parens required for types */

/* sizeof does NOT evaluate the expression! */
int i = 0;
sizeof(i++);    /* i is still 0! No increment happens */
sizeof(1/0);    /* no division — no crash */
```

### sizeof With VLAs — Runtime!

```c
void func(int n) {
    int arr[n];                  /* VLA */
    printf("%zu\n", sizeof(arr)); /* RUNTIME evaluation! */
}
```

### The Array Size Pattern

```c
/* CORRECT — use sizeof on array, divide by element size */
int arr[10];
size_t n = sizeof(arr) / sizeof(arr[0]);     /* 10 */
size_t n2 = sizeof(arr) / sizeof(int);       /* also 10, but fragile */
/* arr[0] is safer: if type changes, it still works */

/* WRONG — pointer decay: */
void func(int *arr) {
    size_t n = sizeof(arr) / sizeof(arr[0]);  /* 8/4 = 2 — WRONG! */
}
```

---

## 31. _Alignof and _Alignas (C11)

### Alignment — What It Is

Every type has an alignment requirement: its address must be a multiple of some value.

```
Type        Size    Alignment (typical x86-64)
─────────────────────────────────────────────
char        1       1
short       2       2
int         4       4
long        8       8
float       4       4
double      8       8
pointer     8       8
```

```
_Alignof(int) = 4    /* int must be at address divisible by 4 */
_Alignof(double) = 8
```

### _Alignas — Force Alignment

```c
#include <stdalign.h>

/* Force a variable to be 64-byte aligned (cache line) */
_Alignas(64) int cache_line_data[16];

/* Align to same as another type */
_Alignas(double) char buffer[sizeof(double)];
```

### Linux Kernel Alignment

```c
/* linux/cache.h */
#define ____cacheline_aligned __attribute__((__aligned__(CACHE_LINE_SIZE)))
#define __cacheline_aligned   ____cacheline_aligned

/* Per-CPU data — each CPU has its own cache line to avoid false sharing */
struct irq_cpustat_t {
    unsigned int __softirq_pending;
} ____cacheline_aligned;

/* DMA buffer must be page-aligned */
void *buf;
buf = kmalloc(BUF_SIZE, GFP_KERNEL | GFP_DMA);
/* or */
buf = (void *)__get_free_pages(GFP_DMA, get_order(BUF_SIZE));
```

---

## 32. Pre/Post Increment in Complex Expressions

### The Definitive Rules

```
++i   (prefix):  increment i FIRST, then use the new value
i++   (postfix): use the current value FIRST, then increment
```

```c
int i = 5;
int a = ++i;   /* i becomes 6, a = 6 */
int b = i++;   /* b = 6 (current), then i becomes 7 */
int c = i;     /* c = 7 */
```

### In Pointer Traversal — The Most Common Use

```c
char src[] = "hello";
char dst[10];
char *s = src;
char *d = dst;

/* Copy loop — kernel style */
while ((*d++ = *s++) != '\0')
    ;
/* Breakdown:
   1. *s++: dereference s (get char), then s advances
   2. *d++ = ...: assign char to *d, then d advances
   3. (!= '\0'): loop until null terminator copied
*/
```

### The i++ in for loops

```c
/* These are equivalent: */
for (int i = 0; i < n; i++)  { }
for (int i = 0; i < n; ++i) { }

/* In a for loop, the increment is a standalone statement — */
/* pre vs post makes NO difference here */
/* (the result of i++ is discarded) */
```

---

# PART V — PREPROCESSOR AND MACROS

---

## 33. Macros — Token Pasting, Stringification

### Basic Macro Types

```c
/* OBJECT-LIKE: simple substitution */
#define PI 3.14159265358979
#define MAX_SIZE 1024
#define TRUE 1

/* FUNCTION-LIKE: parameterized substitution */
#define SQUARE(x) ((x) * (x))
/* Always parenthesize the whole thing AND each argument! */
```

### Why Parentheses Matter in Macros

```c
#define BAD_SQUARE(x)  x * x
#define GOOD_SQUARE(x) ((x) * (x))

int a = BAD_SQUARE(2 + 3);   /* 2 + 3 * 2 + 3 = 11 — WRONG */
int b = GOOD_SQUARE(2 + 3);  /* (2+3) * (2+3) = 25 — CORRECT */

int c = BAD_SQUARE(5);  /* 5 * 5 = 25 — looks right */
/* But: */
#define BAD_ABS(x) x < 0 ? -x : x
int d = BAD_ABS(-3) + 1;  /* -3 < 0 ? --3 : -3 + 1 — disaster! */
```

### Stringification — `#` Operator

```c
#define STRINGIFY(x) #x

STRINGIFY(hello)    → "hello"
STRINGIFY(3 + 4)    → "3 + 4"
STRINGIFY(int)      → "int"

/* Used in Linux kernel for version strings: */
#define KERNEL_VERSION(a,b,c) (((a) << 16) + ((b) << 8) + ((c) > 255 ? 255 : (c)))
#define UTS_RELEASE STRINGIFY(LINUX_VERSION_MAJOR) "." \
                    STRINGIFY(LINUX_VERSION_PATCHLEVEL) "." \
                    STRINGIFY(LINUX_VERSION_SUBLEVEL)
```

### Token Pasting — `##` Operator

```c
#define CONCAT(a, b) a##b

CONCAT(hello, world)  → helloworld (single token)
CONCAT(var, 1)        → var1

/* Linux kernel: generating unique names */
#define DEFINE_MUTEX(mutexname) \
    struct mutex mutexname = __MUTEX_INITIALIZER(mutexname)

/* Generating accessor functions */
#define MAKE_GETTER(type, field) \
    static inline type get_##field(struct MyStruct *s) { \
        return s->field; \
    }

MAKE_GETTER(int, width)   /* generates: get_width() */
MAKE_GETTER(int, height)  /* generates: get_height() */
```

### Multi-Line Macros — The do { } while(0) Pattern

```c
/* WRONG: if/else breaks this */
#define WRONG_MACRO(x) \
    statement1(x); \
    statement2(x);

if (condition)
    WRONG_MACRO(val);    /* only statement1 is in if body! */
else
    something();

/* CORRECT: wrap in do-while(0) */
#define CORRECT_MACRO(x) \
    do {                 \
        statement1(x);   \
        statement2(x);   \
    } while (0)

/* Now: */
if (condition)
    CORRECT_MACRO(val);  /* both statements in if body */
else
    something();         /* correctly parsed */
```

---

## 34. Variadic Macros (C99)

### Syntax

```c
#define DEBUG_PRINT(fmt, ...) printf("[DEBUG] " fmt, ##__VA_ARGS__)
/*                                             ↑
   ## before __VA_ARGS__ removes trailing comma if no args passed */

DEBUG_PRINT("Hello\n");                /* printf("[DEBUG] Hello\n") */
DEBUG_PRINT("x = %d\n", x);           /* printf("[DEBUG] x = %d\n", x) */
```

### Linux Kernel pr_* Macros

```c
/* linux/printk.h */
#define pr_fmt(fmt) fmt

#define pr_emerg(fmt, ...) \
    printk(KERN_EMERG pr_fmt(fmt), ##__VA_ARGS__)
#define pr_alert(fmt, ...) \
    printk(KERN_ALERT pr_fmt(fmt), ##__VA_ARGS__)
#define pr_err(fmt, ...) \
    printk(KERN_ERR pr_fmt(fmt), ##__VA_ARGS__)
#define pr_warn(fmt, ...) \
    printk(KERN_WARNING pr_fmt(fmt), ##__VA_ARGS__)
#define pr_info(fmt, ...) \
    printk(KERN_INFO pr_fmt(fmt), ##__VA_ARGS__)
#define pr_debug(fmt, ...) \
    printk(KERN_DEBUG pr_fmt(fmt), ##__VA_ARGS__)

/* Usage in driver: */
pr_info("Device probed: %s, irq=%d\n", dev->name, irq);
pr_err("Failed to allocate DMA buffer\n");
```

---

## 35. X-Macros — The Linux Kernel Pattern

### What X-Macros Are

X-macros generate repetitive code from a single table definition, avoiding duplication.

```c
/* Define the table ONCE */
#define SYSCALL_TABLE \
    X(0,  read)   \
    X(1,  write)  \
    X(2,  open)   \
    X(3,  close)  \
    X(4,  stat)

/* Generate enum from table */
enum syscall_nr {
#define X(nr, name) __NR_##name = nr,
    SYSCALL_TABLE
#undef X
};

/* Generate string names from same table */
const char *syscall_names[] = {
#define X(nr, name) [nr] = #name,
    SYSCALL_TABLE
#undef X
};

/* Generate function prototypes from same table */
#define X(nr, name) long sys_##name(void);
SYSCALL_TABLE
#undef X
```

### Linux Kernel X-Macro — Error Codes

```c
/* linux/errno.h style: */
#define ERRNO_TABLE \
    E(EPERM,   1,  "Operation not permitted")  \
    E(ENOENT,  2,  "No such file or directory") \
    E(ESRCH,   3,  "No such process")           \
    E(EINTR,   4,  "Interrupted system call")

/* Generate enum */
enum errno_vals {
#define E(name, num, str) name = num,
    ERRNO_TABLE
#undef E
};

/* Generate error string table */
const char *error_strings[] = {
#define E(name, num, str) [num] = str,
    ERRNO_TABLE
#undef E
};
```

---

## 36. #pragma, __attribute__, __builtin

### __attribute__ — GCC/Clang Compiler Directives

```c
/* Placement: after the entity it applies to */

/* ── FUNCTION ATTRIBUTES ── */

/* No return — function never returns (like exit()) */
void die(const char *msg) __attribute__((noreturn));

/* Warn if return value unused */
int open_file(const char *path) __attribute__((warn_unused_result));
/* Linux: #define __must_check __attribute__((warn_unused_result)) */

/* Alias — this function is an alias for another */
void my_exit(int code) __attribute__((alias("exit")));

/* Weak symbol — can be overridden by strong symbol */
void __attribute__((weak)) default_handler(void) { }

/* Constructor/Destructor — run before/after main */
void __attribute__((constructor)) init_module(void) { }
void __attribute__((destructor))  fini_module(void) { }

/* Format checking — like printf */
void my_log(const char *fmt, ...) __attribute__((format(printf, 1, 2)));
/*                                                       ↑       ↑  ↑
                                                   format   fmt_idx arg_idx  */

/* ── VARIABLE/TYPE ATTRIBUTES ── */

/* Packed — no padding */
struct __attribute__((packed)) PackedHeader {
    uint8_t  type;
    uint16_t length;
    uint32_t checksum;
} __attribute__((packed));
/* size = 7, not 8 (no padding) */

/* Aligned */
int x __attribute__((aligned(64)));  /* 64-byte alignment */

/* Unused — suppress unused warnings */
static int helper(void) __attribute__((unused));

/* Section — place in specific linker section */
const char version_info[] __attribute__((section(".rodata.version"))) = "1.0";
```

### Linux Kernel __attribute__ Usage

```c
/* linux/compiler_attributes.h */
#define __packed        __attribute__((__packed__))
#define __aligned(x)    __attribute__((__aligned__(x)))
#define __printf(a,b)   __attribute__((format(printf,a,b)))
#define __scanf(a,b)    __attribute__((format(scanf,a,b)))
#define __noreturn      __attribute__((__noreturn__))
#define __must_check    __attribute__((__warn_unused_result__))
#define __cold          __attribute__((__cold__))
#define __hot           __attribute__((__hot__))

/* __init and __exit put code in special sections */
#define __init          __section(".init.text") __cold  notrace
#define __exit          __section(".exit.text") __exitused __cold  notrace
```

### __builtin — GCC Built-in Functions

```c
/* Branch prediction hints */
__builtin_expect(expr, expected_value)

/* Linux kernel wraps these: */
#define likely(x)   __builtin_expect(!!(x), 1)
#define unlikely(x) __builtin_expect(!!(x), 0)

/* Other builtins used in kernel: */
__builtin_popcount(x)     /* count set bits */
__builtin_clz(x)          /* count leading zeros */
__builtin_ctz(x)          /* count trailing zeros */
__builtin_bswap32(x)      /* byte swap (for endian conversion) */
__builtin_offsetof(T, m)  /* offset of member m in struct T */
__builtin_types_compatible_p(t1, t2)  /* check type compatibility */
```

---

## 37. Include Guards vs #pragma once

### Include Guards — Portable

```c
/* linux/kernel.h style */
#ifndef _LINUX_KERNEL_H
#define _LINUX_KERNEL_H
/* ... content ... */
#endif /* _LINUX_KERNEL_H */
```

### #pragma once — Non-Standard but Widely Supported

```c
#pragma once
/* compiler ensures this file is included only once */
/* Faster: no macro lookup needed */
/* But: non-standard (not in C standard, though universally supported) */
```

### Linux Kernel Uses Guards — Never pragma once

```c
/* Every kernel header: */
#ifndef __LINUX_FOO_H
#define __LINUX_FOO_H

/* content */

#endif /* __LINUX_FOO_H */
```

---

# PART VI — ADVANCED TYPE SYSTEM

---

## 38. Variable-Length Arrays (VLA)

### What VLAs Are

Arrays whose size is determined at **runtime**, not compile time.

```c
#include <stdio.h>

void process(int n) {
    int arr[n];       /* VLA: size determined at runtime */
    /* arr is on the stack — same as normal arrays */
    /* sizeof(arr) = n * sizeof(int) — evaluated at runtime */

    for (int i = 0; i < n; i++)
        arr[i] = i * i;
}

int main(void) {
    process(5);
    process(100);
}
```

### VLA Limitations and Dangers

```c
/* Cannot initialize VLA like fixed arrays */
int vla[n] = {0};    /* ERROR in some compilers */
/* Use memset instead: */
memset(vla, 0, n * sizeof(int));

/* No static/extern VLA */
static int arr[n];   /* ERROR: VLA can't be static */

/* DANGER: stack overflow for large n */
void dangerous(int n) {
    int huge[n];     /* if n = 1000000: stack overflow, no warning */
}

/* Linux kernel: VLAs are BANNED since kernel 4.20 */
/* Reason: unpredictable stack usage, security risk */
/* Use: kmalloc() instead */
```

---

## 39. Variadic Functions (va_list, va_arg)

### The Mechanism

```c
#include <stdarg.h>

/* ... means "any number of additional arguments" */
int my_printf(const char *fmt, ...) {
    va_list args;           /* declare argument list handle */
    va_start(args, fmt);    /* initialize: fmt is LAST named param */

    /* Process format string... */
    while (*fmt) {
        if (*fmt == '%') {
            fmt++;
            if (*fmt == 'd') {
                int val = va_arg(args, int);  /* extract next int arg */
                /* print val */
            } else if (*fmt == 's') {
                char *s = va_arg(args, char*); /* extract next char* */
                /* print s */
            } else if (*fmt == 'f') {
                double d = va_arg(args, double); /* float is promoted to double! */
                /* print d */
            }
        }
        fmt++;
    }

    va_end(args);           /* cleanup — REQUIRED */
    return 0;
}
```

### Type Promotion in Variadic Args

```c
/* All arguments passed to ... undergo default argument promotions: */
char   → int     (integer promotion)
short  → int
float  → double  (float promotion)
/* so va_arg(args, char) is WRONG — use va_arg(args, int) */
/* and va_arg(args, float) is WRONG — use va_arg(args, double) */
```

### va_copy — For Using Args Twice

```c
int vsnprintf(char *buf, size_t size, const char *fmt, va_list args) {
    va_list args_copy;
    va_copy(args_copy, args);  /* make a copy */

    /* first pass: calculate needed size */
    int needed = calculate_size(fmt, args);

    /* second pass: actually format */
    format_string(buf, size, fmt, args_copy);

    va_end(args_copy);  /* must end the copy too */
    return needed;
}
```

---

## 40. Inline Functions and Inline Assembly

### inline Keyword

```c
/* DECLARATION: suggests compiler inline the function body */
static inline int max_int(int a, int b) {
    return a > b ? a : b;
}
/* static: internal linkage (otherwise multiple definition error) */
/* inline: hint to inline the call site — compiler may ignore */
```

### Inline Assembly (GNU C)

```c
/* Basic form: */
asm("instruction");

/* Extended form (used in kernel): */
asm volatile (
    "assembly template"           /* instruction(s) */
    : output operands             /* what C variables receive values */
    : input operands              /* what C variables provide values */
    : clobbered registers         /* registers this asm modifies */
);
```

### Linux Kernel Inline Assembly Examples

```c
/* ── x86: Read CPU timestamp counter ── */
static inline uint64_t rdtsc(void) {
    uint32_t lo, hi;
    asm volatile (
        "rdtsc"
        : "=a"(lo), "=d"(hi)  /* output: eax→lo, edx→hi */
        :                      /* no input */
        :                      /* no clobber */
    );
    return ((uint64_t)hi << 32) | lo;
}

/* ── x86: Atomic increment ── */
static inline void atomic_inc(atomic_t *v) {
    asm volatile (
        LOCK_PREFIX "incl %0"   /* LOCK prefix for SMP safety */
        : "+m"(v->counter)      /* "+m" = read-write memory operand */
    );
}

/* ── ARM: Memory barrier ── */
static inline void smp_mb(void) {
    asm volatile ("dmb ish" : : : "memory");
    /*                              ↑
                        "memory" clobber: tells compiler
                        not to reorder memory accesses around this */
}

/* ── Operand constraints: ── */
/*  "a" = eax/rax register       */
/*  "b" = ebx/rbx register       */
/*  "c" = ecx/rcx register       */
/*  "d" = edx/rdx register       */
/*  "r" = any general register   */
/*  "m" = memory location        */
/*  "i" = immediate integer      */
/*  "=" = write-only (output)    */
/*  "+" = read-write             */
/*  "&" = early-clobber          */
```

---

## 41. Type Punning — Safe and Unsafe Ways

### What Type Punning Is

Reading memory as a different type than it was written.

```c
float f = 3.14f;
int i = *(int *)&f;   /* UNDEFINED BEHAVIOR in C — strict aliasing violation */
```

### The Strict Aliasing Rule

> The compiler is allowed to assume that pointers to different types do not point to the same memory.

```c
/* Strict aliasing VIOLATION — UB */
float f = 3.14f;
int *ip = (int *)&f;
int x = *ip;   /* UB: compiler can assume int* never aliases float* */
               /* (except char* which is special) */
```

### Safe Type Punning — Union (C99 explicitly allows this)

```c
#include <stdint.h>

union FloatInt {
    float    f;
    uint32_t i;
};

float f = 3.14f;
union FloatInt u;
u.f = f;
uint32_t bits = u.i;   /* DEFINED in C99 — writing one union member */
                       /* and reading another is allowed */
```

### Safe Type Punning — memcpy (Always Portable)

```c
#include <string.h>
#include <stdint.h>

float f = 3.14f;
uint32_t bits;
memcpy(&bits, &f, sizeof(bits));   /* ALWAYS defined behavior */
/* compiler optimizes memcpy away for small fixed sizes */
```

### Linux Kernel `-fno-strict-aliasing`

```c
/* The Linux kernel is compiled with -fno-strict-aliasing */
/* This disables the strict aliasing optimization */
/* So the kernel can do raw memory manipulation freely */
/* Example: network buffer manipulation, protocol header overlaying */

struct ethhdr *eth = (struct ethhdr *)skb->data;  /* would be UB normally */
/* Safe in kernel because of -fno-strict-aliasing */
```

---

## 42. Endianness — Detection and Conversion

### What Endianness Is

```
LITTLE-ENDIAN (x86, ARM in LE mode):
Value 0x12345678 stored at address 0x100:

Address:  0x100  0x101  0x102  0x103
Byte:     0x78   0x56   0x34   0x12
           LSB                  MSB
          (least significant first)

BIG-ENDIAN (network byte order, some MIPS, SPARC):
Value 0x12345678:

Address:  0x100  0x101  0x102  0x103
Byte:     0x12   0x34   0x56   0x78
           MSB                  LSB
          (most significant first)
```

### Detection and Conversion

```c
#include <stdint.h>
#include <string.h>

/* ── RUNTIME DETECTION ── */
static inline int is_little_endian(void) {
    uint32_t x = 1;
    return *(uint8_t *)&x == 1;
}

/* ── COMPILE-TIME DETECTION ── */
#if __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__
#define HOST_IS_LE 1
#else
#define HOST_IS_LE 0
#endif

/* ── BYTE SWAP FUNCTIONS ── */
static inline uint16_t bswap16(uint16_t x) {
    return (x >> 8) | (x << 8);
}

static inline uint32_t bswap32(uint32_t x) {
    return ((x & 0xFF000000) >> 24) |
           ((x & 0x00FF0000) >>  8) |
           ((x & 0x0000FF00) <<  8) |
           ((x & 0x000000FF) << 24);
}
/* Or use GCC builtin: __builtin_bswap32(x) */
```

### Linux Kernel Endianness

```c
/* linux/byteorder/generic.h */
/* hton = host to network (big-endian) */
/* ntoh = network to host */

__be32 htonl(uint32_t hostlong);      /* host to network 32-bit */
__be16 htons(uint16_t hostshort);     /* host to network 16-bit */
uint32_t ntohl(__be32 netlong);       /* network to host 32-bit */
uint16_t ntohs(__be16 netshort);      /* network to host 16-bit */

/* Kernel also has: */
cpu_to_be32(x)    /* CPU endian → big-endian */
be32_to_cpu(x)    /* big-endian → CPU endian */
cpu_to_le32(x)    /* CPU endian → little-endian */
le32_to_cpu(x)    /* little-endian → CPU endian */

/* __be32, __le32: annotated types (Sparse checks these) */
```

---

## 43. Alignment — Padding, Packing, __packed__

### Why Alignment Exists

CPUs access memory most efficiently when data is naturally aligned:
- A 4-byte int should be at an address divisible by 4
- An 8-byte double at an address divisible by 8

Misaligned access can cause:
- Performance penalty (multiple bus cycles)
- Fault / crash on strict architectures (ARM without unaligned access support)

### Controlling Padding

```c
/* DEFAULT — compiler adds padding */
struct Padded {
    char  a;      /* 1 byte + 3 pad */
    int   b;      /* 4 bytes */
    char  c;      /* 1 byte + 7 pad (for 8-byte total alignment) */
    double d;     /* 8 bytes */
};
/* sizeof = 24 */

/* PACKED — no padding (may cause misaligned access) */
struct __attribute__((packed)) Packed {
    char   a;     /* 1 byte */
    int    b;     /* 4 bytes — at offset 1 (MISALIGNED!) */
    char   c;     /* 1 byte */
    double d;     /* 8 bytes — at offset 6 (MISALIGNED!) */
};
/* sizeof = 14 */
```

```
PADDED layout (sizeof=24):
┌─┬─┬─┬─┬─────────┬─┬─┬─┬─┬─┬─┬─┬──────────────────┐
│a│P│P│P│  b(4B)  │c│P│P│P│P│P│P│P│     d (8B)      │
└─┴─┴─┴─┴─────────┴─┴─┴─┴─┴─┴─┴─┴──────────────────┘
 0  1  2  3  4  5  6  7  8  9 10 11 12 ... 23

PACKED layout (sizeof=14):
┌─┬──────────┬─┬──────────────────┐
│a│  b (4B)  │c│     d (8B)       │
└─┴──────────┴─┴──────────────────┘
 0  1  2  3  4  5  6  7  8  9 10 11 12 13
```

### Linux Kernel Packed Structs

```c
/* Network protocol headers must be packed */
struct iphdr {
#if defined(__LITTLE_ENDIAN_BITFIELD)
    __u8    ihl:4, version:4;
#elif defined (__BIG_ENDIAN_BITFIELD)
    __u8    version:4, ihl:4;
#endif
    __u8    tos;
    __be16  tot_len;
    __be16  id;
    __be16  frag_off;
    __u8    ttl;
    __u8    protocol;
    __sum16 check;
    __be32  saddr;
    __be32  daddr;
    /* options follow */
} __attribute__((packed));  /* match wire format exactly */
```

---

# PART VII — CONTROL FLOW INTERNALS

---

## 44. goto and Labels — When the Kernel Uses Them

### Why goto Is Used in the Kernel

The Linux kernel uses `goto` extensively for **cleanup/error handling**. This is NOT spaghetti code — it is a structured, consistent pattern.

```c
/* PATTERN: cleanup on error — goto is the RIGHT tool here */

int complex_operation(void) {
    struct resource *r1 = NULL, *r2 = NULL, *r3 = NULL;
    int ret;

    r1 = allocate_resource_1();
    if (!r1) {
        ret = -ENOMEM;
        goto err_r1;        /* nothing allocated yet, just return */
    }

    r2 = allocate_resource_2();
    if (!r2) {
        ret = -ENOMEM;
        goto err_r2;        /* free r1 and return */
    }

    r3 = allocate_resource_3();
    if (!r3) {
        ret = -ENOMEM;
        goto err_r3;        /* free r1, r2 and return */
    }

    /* Use resources — do actual work */
    ret = do_work(r1, r2, r3);

    /* SUCCESS PATH: fall through cleanup */
err_r3:
    free_resource(r3);
err_r2:
    free_resource(r2);
err_r1:
    free_resource(r1);
    return ret;
}
```

```
FLOW DIAGRAM:

allocate_r1 ──→ OK ──→ allocate_r2 ──→ OK ──→ allocate_r3 ──→ OK ──→ work
     │                      │                       │
     │ FAIL                 │ FAIL                  │ FAIL
     ↓                      ↓                       ↓
  err_r1 ←────────────── err_r2 ←──────────────  err_r3
     │                      │                       │
     │ (free nothing)        │ (free r1)             │ (free r1,r2)
     └──────────────────────┴───────────────────────┘
                             ↓
                          return ret
```

### Why goto Is Better Than Alternatives Here

```c
/* ALTERNATIVE 1: nested if — grows rightward (pyramid of doom) */
int func(void) {
    if ((r1 = alloc1()) != NULL) {
        if ((r2 = alloc2()) != NULL) {
            if ((r3 = alloc3()) != NULL) {
                ret = do_work(r1, r2, r3);
                free(r3);
            } else ret = -ENOMEM;
            free(r2);
        } else ret = -ENOMEM;
        free(r1);
    } else ret = -ENOMEM;
    return ret;
}
/* Deeply nested, hard to read, easy to make mistakes */

/* goto is CLEANER for this pattern */
```

---

## 45. setjmp / longjmp — Non-local Jumps

### What They Do

`setjmp` saves the execution context (stack pointer, registers, return address).
`longjmp` restores that context, jumping back to the `setjmp` call site.

```c
#include <setjmp.h>

jmp_buf env;   /* stores the execution context */

void deep_function(void) {
    /* ... many calls deep ... */
    longjmp(env, 42);   /* jump back to where setjmp was called */
                        /* returns 42 from setjmp */
}

int main(void) {
    int val = setjmp(env);   /* FIRST call: returns 0 */
                             /* SECOND call (via longjmp): returns 42 */
    if (val == 0) {
        /* Normal execution path */
        deep_function();    /* eventually calls longjmp */
    } else {
        /* Came back via longjmp — val = 42 */
        printf("Returned with value %d\n", val);
    }
}
```

```
CALL STACK DIAGRAM:

main()         ← setjmp saves this frame's context
  └── func_a()
        └── func_b()
              └── func_c()
                    └── deep_function()
                          longjmp(env, 42) ──────────────┐
                                                         │
main() ←──────────────────────────────────────── unwinds stack
  setjmp returns 42                              all intermediate
                                                 frames DISCARDED
```

### Dangers of longjmp

```c
/* DANGER 1: local variables may be wrong if not volatile */
int main(void) {
    int x = 0;              /* might be in register */
    volatile int y = 0;    /* SAFE: volatile forces memory */

    if (setjmp(env) == 0) {
        x = 100;            /* might be in register */
        y = 100;
        call_longjmp();
    }
    /* After longjmp: x might be 0 (register restored!) */
    /*                y is guaranteed 100 (memory not restored) */
}

/* DANGER 2: resources (malloc, fopen) not cleaned up */
/* longjmp skips destructors and cleanup code */
```

### Linux Kernel — No setjmp/longjmp

```c
/* The kernel does NOT use setjmp/longjmp */
/* Instead it uses: */
/* - goto for local error handling */
/* - struct completion for waiting */
/* - signals for async events */
/* - kernel exceptions (do_page_fault, etc.) for hardware faults */
```

---

## 46. switch Statement — Duff's Device and Fall-through

### Full switch Mechanics

```c
switch (expression) {
    /* expression must be integer type */
    case constant1:
        /* code */
        break;          /* without break: FALLS THROUGH to next case */
    case constant2:
    case constant3:     /* two cases, same code — INTENTIONAL fall-through */
        /* code for both */
        break;
    default:            /* optional: matches everything else */
        break;
}
```

### Fall-through — Intentional vs Accidental

```c
/* ACCIDENTAL fall-through (bug): */
switch (x) {
    case 1:
        do_one();
        /* missing break! falls into case 2 */
    case 2:
        do_two();
        break;
}

/* INTENTIONAL fall-through — C17 attribute: */
switch (x) {
    case 1:
        do_one();
        __attribute__((fallthrough));   /* GCC/Clang: suppress warning */
        /* [[fallthrough]]; */          /* C17 standard attribute */
    case 2:
        do_two();
        break;
}
```

### Duff's Device — The Most Famous C Trick

```c
/* Unrolled loop using switch fall-through */
/* Used for fast memory copy in early game programming (David Duff, 1983) */

void send(int *to, int *from, int count) {
    int n = (count + 7) / 8;    /* ceil(count / 8) */
    switch (count % 8) {         /* jump into the middle of the loop */
        case 0: do { *to++ = *from++;
        case 7:      *to++ = *from++;
        case 6:      *to++ = *from++;
        case 5:      *to++ = *from++;
        case 4:      *to++ = *from++;
        case 3:      *to++ = *from++;
        case 2:      *to++ = *from++;
        case 1:      *to++ = *from++;
                } while (--n > 0);
    }
}
```

```
HOW DUFF'S DEVICE WORKS (count = 13):

count % 8 = 5  → jump to case 5
Execute: copy 1 (case 5)
         copy 2 (case 4)
         copy 3 (case 3)
         copy 2 (case 2)... wait:
         case 4,3,2,1 → 4 copies for partial loop
n = ceil(13/8) = 2  → loop runs 2 times
First iteration: 8 copies (case 0..1)
Total: 5 + 8 = 13 ✓
```

---

# PART VIII — LINUX KERNEL C IDIOMS

---

## 47. container_of — The Most Important Macro

### The Problem It Solves

The kernel's linked list stores `list_head` structs embedded inside other structs. How do you get back to the containing struct?

```
struct task_struct {                 struct list_head {
    pid_t pid;                           struct list_head *next;
    char  comm[16];                      struct list_head *prev;
    struct list_head tasks;  ←───────── };
    struct mm_struct *mm;
    /* ... */
};

LIST:
task1.tasks ←──→ task2.tasks ←──→ task3.tasks
    │                 │                 │
    task1             task2             task3

You have a pointer to tasks. You need a pointer to task_struct.
```

### The Macro

```c
/* linux/kernel.h */
#define container_of(ptr, type, member) ({              \
    const typeof( ((type *)0)->member ) *__mptr = (ptr);\
    (type *)( (char *)__mptr - offsetof(type, member) );\
})
```

### Breaking Down container_of

```
container_of(ptr, type, member)
           │       │       │
           │       │       └── struct field name (e.g., tasks)
           │       └────────── struct type (e.g., struct task_struct)
           └────────────────── pointer to the embedded member

ARITHMETIC:
  struct_address = member_address - offsetof(struct, member)

  ptr (list_head*)
   │
   ▼
   [tasks offset into task_struct]
   │
   │ subtract offsetof(task_struct, tasks)
   ▼
   [start of task_struct]

MEMORY:
┌──────────────────────────────────────────┐
│  task_struct                             │
│  ┌──────┐                               │
│  │ pid  │  offset 0                     │
│  ├──────┤                               │
│  │ comm │  offset 4                     │
│  ├──────┤                               │
│  │tasks │  offset 20  ← ptr points here │
│  ├──────┤                               │
│  │  mm  │  offset 36                    │
│  └──────┘                               │
└──────────────────────────────────────────┘
 ↑
 Start of task_struct = ptr - 20
```

### Full Breakdown of the Macro

```c
/* Step 1: typeof( ((type *)0)->member ) */
/* Cast 0 to pointer-to-type, access member — gets member's type */
/* ((struct task_struct *)0)->tasks  →  type is struct list_head */
/* This is a compile-time trick: no actual NULL dereference happens */

/* Step 2: const typeof(...) *__mptr = (ptr) */
/* Store ptr with the correct member type for type safety */
/* If ptr has wrong type, compiler warns */

/* Step 3: (char *)__mptr - offsetof(type, member) */
/* Cast to char* (byte arithmetic), subtract offset */
/* Result is the address of the containing struct */

/* Step 4: (type *)( ... ) */
/* Cast result to pointer to the containing struct */
```

### Usage in Kernel Code

```c
/* linux/list.h */
#define list_entry(ptr, type, member) \
    container_of(ptr, type, member)

#define list_for_each_entry(pos, head, member)                  \
    for (pos = list_first_entry(head, typeof(*pos), member);    \
         !list_entry_is_head(pos, head, member);                \
         pos = list_next_entry(pos, member))

/* USAGE: iterate all tasks */
struct task_struct *task;
list_for_each_entry(task, &init_task.tasks, tasks) {
    printk("Task: %s (pid %d)\n", task->comm, task->pid);
}
```

---

## 48. likely / unlikely — Branch Prediction Hints

### CPU Branch Prediction

Modern CPUs predict which branch will be taken and execute speculatively. If wrong, they flush the pipeline (expensive: ~15-20 cycles).

`likely` / `unlikely` tell the compiler to layout code so the predicted path requires no jump.

```c
/* linux/compiler.h */
#define likely(x)   __builtin_expect(!!(x), 1)
#define unlikely(x) __builtin_expect(!!(x), 0)
/*                                   ↑↑
                     !! converts to 0 or 1 first
                     (handles pointers, floats, etc.) */
```

### Generated Assembly Difference

```c
/* WITHOUT likely/unlikely: */
if (ptr == NULL) {
    handle_error();
}
continue_work();

/* Assembly:
   test rdi, rdi
   jne .L_continue     ; predicted: NOT NULL (default)
   call handle_error
.L_continue:
   ...
*/

/* WITH unlikely: */
if (unlikely(ptr == NULL)) {
    handle_error();
}
continue_work();

/* Assembly (compiler puts error path far away):
   test rdi, rdi
   je .L_error         ; rarely taken
   ...continue_work...
   ret
.L_error:
   call handle_error
   ...
*/
```

### When to Use Which

```c
/* Error checks → unlikely (errors are rare) */
if (unlikely(kmalloc_failed))
    return -ENOMEM;

/* Input validation → unlikely (bad input is rare) */
if (unlikely(!valid_pointer(ptr)))
    return -EINVAL;

/* Fast path → likely (normal case) */
if (likely(cache_hit))
    return cached_value;

/* Completion check → likely (usually done) */
if (likely(remaining == 0))
    break;
```

---

## 49. __must_check, __user, __kernel Annotations

### Sparse Annotations

Sparse is a static analysis tool for the Linux kernel. These annotations help it catch bugs.

```c
/* __must_check: caller MUST check return value */
int __must_check copy_from_user(void *to, const void __user *from, unsigned long n);

/* If you ignore the return value, Sparse warns: */
copy_from_user(buf, user_ptr, len);    /* WARNING: ignored return value */
int ret = copy_from_user(buf, user_ptr, len);  /* OK */

/* __user: pointer to userspace memory */
/* Cannot be directly dereferenced in kernel space */
/* Must use copy_from_user/copy_to_user */
int write_to_user(char __user *ubuf, size_t len) {
    *ubuf = 'A';   /* SPARSE WARNING: dereferencing __user pointer */
    /* Must use: */
    put_user('A', ubuf);    /* safe, handles fault */
    return 0;
}

/* __iomem: pointer to memory-mapped I/O */
/* Must use readb/readl/writeb/writel */
void __iomem *reg = ioremap(phys_addr, size);
u32 val = readl(reg);   /* correct */
u32 bad = *reg;         /* SPARSE WARNING: dereferencing __iomem */
```

---

## 50. RCU — Read-Copy-Update Syntax

### What RCU Is

RCU is a synchronization mechanism for the kernel. Readers run lock-free. Writers make a copy, update the copy, then publish atomically.

```c
/* READER SIDE: */
rcu_read_lock();                          /* disable preemption */
struct my_data *p = rcu_dereference(shared_ptr);  /* safe atomic read */
if (p)
    use_data(p);
rcu_read_unlock();                        /* re-enable preemption */

/* WRITER SIDE: */
struct my_data *old_p;
struct my_data *new_p = kmalloc(sizeof(*new_p), GFP_KERNEL);
/* Initialize new_p */
new_p->value = new_value;

spin_lock(&update_lock);
old_p = rcu_dereference_protected(shared_ptr, lockdep_is_held(&update_lock));
rcu_assign_pointer(shared_ptr, new_p);    /* publish atomically */
spin_unlock(&update_lock);

synchronize_rcu();   /* wait for all existing readers to finish */
kfree(old_p);        /* SAFE: no reader can see old_p now */
```

---

## 51. Linked List — Linux Kernel Style

### The Intrusive List Design

The kernel's list is **intrusive**: instead of the list containing data, the data contains the list node.

```c
/* linux/list.h */
struct list_head {
    struct list_head *next, *prev;
};

/* Embed in your struct: */
struct my_device {
    int id;
    char name[32];
    struct list_head list;   /* embedded list node */
};
```

```
MEMORY LAYOUT:

Traditional list:          Kernel intrusive list:
┌─────────────────┐        ┌──────────────────────────┐
│ list_node       │        │  my_device               │
│   next ──────►  │        │  id = 1                  │
│   prev ◄──────  │        │  name = "dev1"            │
│   data ──────►  │        │  list.next ──────────────►│
└─────────────────┘        │  list.prev ◄──────────────│
        │                  └──────────────────────────┘
        ▼
   [data stored separately]   [list node IS inside struct]
```

### Core List Operations

```c
/* Initialize */
LIST_HEAD(device_list);   /* declare and initialize head */
/* OR: */
struct list_head head;
INIT_LIST_HEAD(&head);

/* Add to list */
struct my_device *dev = kmalloc(sizeof(*dev), GFP_KERNEL);
dev->id = 1;
list_add(&dev->list, &device_list);        /* add to front */
list_add_tail(&dev->list, &device_list);   /* add to back */

/* Iterate */
struct my_device *pos;
list_for_each_entry(pos, &device_list, list) {
    printk("Device: %s\n", pos->name);
}

/* Remove */
list_del(&dev->list);

/* Safe iteration (allows deletion during loop) */
struct my_device *pos, *tmp;
list_for_each_entry_safe(pos, tmp, &device_list, list) {
    if (should_remove(pos)) {
        list_del(&pos->list);
        kfree(pos);
    }
}
```

---

## 52. Bitwise Operations in Device Drivers

### The Complete Bitwise Toolkit

```c
#include <stdint.h>

uint32_t reg = 0;

/* ── SET BIT n ── */
reg |= (1U << n);
/* Example: set bit 3: */
reg |= (1U << 3);    /* reg |= 0x00000008 */

/* ── CLEAR BIT n ── */
reg &= ~(1U << n);
/* Example: clear bit 3: */
reg &= ~(1U << 3);   /* reg &= 0xFFFFFFF7 */

/* ── TOGGLE BIT n ── */
reg ^= (1U << n);

/* ── TEST BIT n ── */
if (reg & (1U << n)) { /* bit is set */ }

/* ── SET BITS in RANGE [high:low] ── */
#define BITS(high, low) (((1U << ((high)-(low)+1)) - 1) << (low))
/* BITS(7, 4) = 0x000000F0 */

/* ── EXTRACT FIELD ── */
#define FIELD_GET(mask, reg) (((reg) & (mask)) / ((mask) & ~((mask)-1)))
/* Or manually: */
uint32_t field = (reg >> low_bit) & ((1U << num_bits) - 1);

/* ── INSERT FIELD ── */
reg = (reg & ~mask) | ((value << low_bit) & mask);
```

### Linux Kernel Bit Macros

```c
/* linux/bits.h */
#define BIT(nr)         (1UL << (nr))
#define BIT_ULL(nr)     (1ULL << (nr))
#define BITS_PER_BYTE   8

/* linux/bitfield.h */
#define FIELD_GET(_mask, _reg)  \
    (typeof(_mask))((_reg & (_mask)) / ((_mask) & ~((_mask) - 1)))
#define FIELD_PREP(_mask, _val) \
    ((_val) * ((_mask) & ~((_mask) - 1)) & (_mask))

/* Example: USB descriptor field manipulation */
#define USB_DIR_IN      0x80
#define USB_TYPE_MASK   0x60
#define USB_RECIP_MASK  0x1f

uint8_t bmRequestType;
if (bmRequestType & USB_DIR_IN) {
    /* direction: device to host */
}
int type = (bmRequestType & USB_TYPE_MASK) >> 5;
```

---

## 53. Atomic Operations

### Why Atomics Are Needed

On multi-core systems, simple operations like `x++` are NOT atomic:

```
Core 1:                      Core 2:
LOAD x (gets 5)
                             LOAD x (gets 5)  ← race!
ADD 1 → 6
STORE x = 6
                             ADD 1 → 6
                             STORE x = 6      ← lost update!

Result: x = 6 instead of 7
```

### Linux Kernel Atomic API

```c
/* linux/atomic.h */

atomic_t counter = ATOMIC_INIT(0);

atomic_set(&counter, 5);          /* set value */
int v = atomic_read(&counter);    /* read value */
atomic_inc(&counter);             /* counter++ atomically */
atomic_dec(&counter);             /* counter-- atomically */

/* Returns true if decremented to zero: */
if (atomic_dec_and_test(&counter)) {
    /* last reference — clean up */
    kfree(obj);
}

/* Add and return new value: */
int new_val = atomic_add_return(5, &counter);

/* Conditional: only set if currently equals old */
atomic_cmpxchg(&counter, old_val, new_val);
```

### Atomic Bit Operations

```c
/* linux/bitops.h */
unsigned long flags = 0;

set_bit(3, &flags);         /* set bit 3 atomically */
clear_bit(3, &flags);       /* clear bit 3 atomically */
test_bit(3, &flags);        /* test bit 3 (not atomic on all archs) */
test_and_set_bit(3, &flags);     /* returns old value, sets new */
test_and_clear_bit(3, &flags);   /* returns old value, clears */

/* Non-atomic versions (use when holding lock): */
__set_bit(3, &flags);
__clear_bit(3, &flags);
```

---

## 54. Memory Barriers — smp_mb, rmb, wmb

### Why Memory Barriers Are Needed

```
CPU can reorder memory operations for performance.
Other CPUs may see writes in different order.

CPU 1:              CPU 2:
x = 1;              while (!flag);  ← waits for flag
flag = 1;           use(x);         ← might see x=0 if reordered!

WITH BARRIER:
x = 1;
smp_wmb();          /* write barrier: x visible BEFORE flag */
flag = 1;
```

### Linux Kernel Barrier API

```c
/* Full memory barrier — reads and writes ordered */
smp_mb();

/* Write barrier — all writes before are visible before writes after */
smp_wmb();

/* Read barrier — all reads before complete before reads after */
smp_rmb();

/* Compiler barrier only (no CPU ordering) */
barrier();

/* Acquire/release semantics (C11-style, paired) */
smp_load_acquire(&x);   /* load with acquire: no reads after can move before */
smp_store_release(&x, val); /* store with release: all writes before visible */
```

---

## 55. BUILD_BUG_ON and Static Assertions

### Compile-Time Assertions

```c
/* linux/bug.h */

/* Generates a compile error if condition is TRUE */
#define BUILD_BUG_ON(condition) \
    ((void)sizeof(char[1 - 2*!!(condition)]))
/*
   !!(condition): converts to 0 or 1
   2*!!: makes it 0 or 2
   1 - 0 = 1 → sizeof(char[1]) = OK
   1 - 2 = -1 → sizeof(char[-1]) = COMPILE ERROR!
*/

/* C11 standard: _Static_assert */
_Static_assert(sizeof(int) == 4, "int must be 4 bytes");
_Static_assert(sizeof(void*) >= 4, "pointer too small");

/* Linux kernel uses: */
BUILD_BUG_ON(sizeof(struct ethhdr) != 14);
BUILD_BUG_ON(NR_SYSCALLS > PAGE_SIZE/sizeof(void*));

/* Returns expression value AND checks: */
#define BUILD_BUG_ON_ZERO(e) (sizeof(struct { int:(-!!(e)); }))
/* Used inside other macros to assert and still return 0 */
```

---

## 56. ARRAY_SIZE and Zero-Length Arrays

### ARRAY_SIZE — Safe Version

```c
/* linux/kernel.h */
#define ARRAY_SIZE(arr) \
    (sizeof(arr) / sizeof((arr)[0]) + \
     BUILD_BUG_ON_ZERO(!__builtin_types_compatible_p(typeof(arr), \
                                                      typeof(&(arr)[0]))))

/* typeof(arr)       = int[5] for real array */
/* typeof(&(arr)[0]) = int*   for real array */
/* They are DIFFERENT → !compatible → 0 → BUILD_BUG_ON_ZERO(0) → 0 */

/* For pointer: */
/* typeof(ptr)       = int* */
/* typeof(&ptr[0])   = int* */
/* They are SAME → !compatible → 1 → BUILD_BUG_ON_ZERO(1) → COMPILE ERROR */
```

### Zero-Length and One-Past-End Arrays

```c
/* HISTORICAL: zero-length array (GNU extension) */
struct Header {
    int length;
    char data[0];   /* GNU extension — points just past the struct */
};

/* STANDARD: flexible array member (C99) */
struct Header {
    int length;
    char data[];    /* standard — equivalent to data[0] in practice */
};

/* IN KERNEL: both are used */
/* data[0] for old code, data[] for new code */

/* ACCESS: */
struct Header *h = kmalloc(sizeof(*h) + data_len, GFP_KERNEL);
h->length = data_len;
memcpy(h->data, source, data_len);  /* data points right after h */
```

---

# APPENDIX: MENTAL MODELS AND READING COMPLEX CODE

---

## How to Read Kernel Code Like an Expert

### The Four-Pass Approach

```
PASS 1: Orientation
  - What file is this? (drivers/net/ethernet/intel/e1000/*.c)
  - What subsystem? (networking, block, memory)
  - What is the main struct? (struct e1000_adapter)

PASS 2: Data Structures
  - Draw the struct layout
  - Identify embedded list_head members
  - Note which fields are protected by which locks

PASS 3: Control Flow
  - Find init/exit functions
  - Find the main operation (probe, open, read, write, interrupt handler)
  - Trace through with a specific use case

PASS 4: Concurrency
  - Which operations can run concurrently?
  - Where are the locks taken and released?
  - What is protected by RCU vs spinlock vs mutex?
```

### Quick Reference: Confusing Kernel Syntax

```c
/* ── THING YOU SEE ──────────────── WHAT IT MEANS ── */

__init                           /* placed in .init.text, freed after boot */
__exit                           /* only compiled for loadable modules */
__iomem                          /* pointer to memory-mapped I/O */
__user                           /* pointer to user-space memory */
__be32, __le32                   /* annotated big/little-endian u32 */
__packed                         /* struct with no padding */
__aligned(n)                     /* n-byte aligned */
__must_check                     /* caller must check return value */
__cold                           /* rarely called (error paths) */
__hot                            /* frequently called (fast paths) */
__read_mostly                    /* in read-mostly cache section */
__percpu                         /* per-CPU variable */
KERN_ERR "message"               /* printk priority prefix string */
THIS_MODULE                      /* pointer to current module struct */
GFP_KERNEL                       /* memory allocation flags */
NULL                             /* (void *)0 */
BUG()                            /* kernel panic with file/line info */
BUG_ON(cond)                     /* panic if condition true */
WARN_ON(cond)                    /* print warning + stack, don't panic */
likely(x) / unlikely(x)          /* branch prediction hints */
READ_ONCE(x) / WRITE_ONCE(x,v)  /* prevent compiler reordering */
smp_mb() etc                     /* memory barriers */
DEFINE_MUTEX(name)               /* declare + init a mutex */
DEFINE_SPINLOCK(name)            /* declare + init a spinlock */
container_of(ptr,type,member)    /* get enclosing struct from member ptr */
list_for_each_entry(pos,head,m)  /* iterate linked list */
rcu_read_lock() / rcu_read_unlock() /* RCU read-side critical section */
```

---

## The Clockwise/Spiral Rule — Quick Reference Card

```
READING ANY DECLARATION:

  1. Find the identifier (name)
  2. Go RIGHT: read [] (array) or () (function) if present
  3. Go LEFT: read * (pointer) or qualifiers
  4. Hit ) ? jump to matching (, repeat from step 2
  5. Base type is last

EXAMPLES:

int *p           → p: pointer to int
int *p[5]        → p: array[5] of pointer to int
int (*p)[5]      → p: pointer to array[5] of int
int *f()         → f: function() returning pointer to int
int (*f)()       → f: pointer to function() returning int
int (*f[5])()    → f: array[5] of pointer to function() returning int
int **pp         → pp: pointer to pointer to int
void *(*f)(int)  → f: pointer to function(int) returning void*
```

---

## Undefined Behavior Quick Reference

```
UNDEFINED BEHAVIOR — always avoid:

1. Signed integer overflow:       INT_MAX + 1
2. Out-of-bounds array access:    arr[n] where n >= size
3. Dereferencing NULL:            *NULL
4. Dereferencing freed memory:    use-after-free
5. Stack buffer overflow:         write past end of local array
6. Unsequenced modifications:     i = i++
7. Strict aliasing violation:     *(int*)float_ptr
8. Shift by negative/too-large:   x << -1  or  x << 64
9. Division by zero:              x / 0
10. Modifying string literal:     char *s = "hi"; s[0] = 'H';

IMPLEMENTATION-DEFINED (not UB, but varies):
- sizeof(int): 2 or 4
- char signedness: signed or unsigned
- Bit field layout
- Enum underlying type
```

---

*End of Guide — Total Coverage: 56 Deep Topics with Linux Kernel Context*

*The goal is not to memorize — but to internalize the underlying model.*
*Once you see that all C syntax is just composition of a few rules,*
*every declaration becomes immediately readable.*
