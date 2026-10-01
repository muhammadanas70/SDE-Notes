# The Complete & Comprehensive C Programming Mastery Guide
### Rules, Regulations, Concepts, Mental Models, and Deep Internals

---

## Table of Contents

1. [History, Standards & Why C Matters](#1-history-standards--why-c-matters)
2. [The Compilation Pipeline — Every Stage Explained](#2-the-compilation-pipeline--every-stage-explained)
3. [Program Structure & Anatomy](#3-program-structure--anatomy)
4. [Data Types, Sizes & Representation](#4-data-types-sizes--representation)
5. [Variables: Storage Classes, Qualifiers & Scope](#5-variables-storage-classes-qualifiers--scope)
6. [Operators: Complete Reference with Precedence](#6-operators-complete-reference-with-precedence)
7. [Control Flow — Every Construct](#7-control-flow--every-construct)
8. [Functions — Deep Mechanics](#8-functions--deep-mechanics)

---

## 1. History, Standards & Why C Matters

### 1.1 The Origins

C was created by **Dennis Ritchie** at Bell Labs between **1969 and 1973**, evolving from **B** (itself evolved from BCPL). The Unix operating system was rewritten in C around 1973, establishing C as the language of systems programming.

```
Timeline of C Evolution:
-----------------------------------------------------------------
1969  B language (Ken Thompson, Bell Labs)
1972  C language born — Dennis Ritchie
1973  Unix kernel rewritten in C
1978  "The C Programming Language" — Kernighan & Ritchie (K&R C)
1989  ANSI C / C89 standardized (also called C90 by ISO)
1999  C99 — major additions (//comments, VLA, _Bool, stdint.h)
2011  C11 — threads, atomics, _Generic, _Noreturn, _Static_assert
2017  C17 (C18) — bug fixes, no new features
2023  C23 — constexpr, nullptr, typeof, bit-precise integers
-----------------------------------------------------------------
```

### 1.2 Why C Is the Foundation Language

C gives you **direct hardware access** with minimal abstraction. Understanding C means understanding:
- How memory actually works (stack, heap, segments)
- How the CPU executes instructions
- How the OS manages processes
- How all higher-level languages are implemented

**The C Mental Model**: You are writing instructions for the machine. Every byte matters. Every allocation must be freed. Every pointer must be valid. The language trusts you completely and punishes mistakes at runtime, not compile time.

### 1.3 C Standards — What Changed and Why It Matters

**K&R C (pre-1989)**: No function prototypes. `int` was the default return type. Implicit declarations.

**C89/C90**: First standardized version. Introduced function prototypes, `void`, standardized library. Still the most portable standard.

**C99** additions you must know:
- `//` single-line comments
- Variable declarations anywhere in a block (not just at top)
- Variable Length Arrays (VLAs) — stack arrays with runtime size
- `_Bool`, `<stdbool.h>` — bool type
- `<stdint.h>` — exact width integers (`int32_t`, `uint8_t`, etc.)
- `<inttypes.h>` — printf/scanf format macros
- `restrict` keyword for pointer aliasing hints
- Designated initializers for structs and arrays
- Compound literals
- `long long int`
- `snprintf` guaranteed behavior
- `inline` keyword

**C11** additions you must know:
- `<threads.h>` — portable threading
- `<stdatomic.h>` — atomic operations
- `_Generic` — type-generic expressions
- `_Static_assert` — compile-time assertions
- `_Noreturn` — functions that never return
- Anonymous structs and unions
- Improved unicode support

**C17**: Purely a defect-fixing revision — same features as C11, cleaner spec.

**C23** (current):
- `nullptr` literal (replaces NULL for pointers)
- `typeof()` operator
- `constexpr` for constants
- Bit-precise integer types: `_BitInt(N)`
- Attribute syntax `[[nodiscard]]`, `[[deprecated]]`

---

## 2. The Compilation Pipeline — Every Stage Explained

Understanding exactly what happens to your source code before it runs is essential for debugging and optimization.

```
+------------------+
|  source.c        |  ← Your C source file (text)
+------------------+
         |
         v  [STAGE 1: PREPROCESSOR — cpp]
+------------------+
|  source.i        |  ← Preprocessed source (text, macros expanded)
+------------------+
         |
         v  [STAGE 2: COMPILER — cc1]
+------------------+
|  source.s        |  ← Assembly source (text)
+------------------+
         |
         v  [STAGE 3: ASSEMBLER — as]
+------------------+
|  source.o        |  ← Object file (binary, relocatable ELF)
+------------------+
         |
         v  [STAGE 4: LINKER — ld]
+------------------+
|  executable      |  ← Final ELF binary (or .exe on Windows)
+------------------+
```

### 2.1 Stage 1: Preprocessing

The preprocessor handles all `#` directives **before** compilation.

What it does:
- Removes all comments
- Expands `#include` — literally copies the file contents
- Expands `#define` macros — textual substitution
- Evaluates `#if`, `#ifdef`, `#ifndef`, `#elif`, `#else`, `#endif`
- Produces the `.i` file

```c
// To see preprocessor output:
// gcc -E source.c -o source.i

#include <stdio.h>
#define MAX 100
#define SQUARE(x) ((x)*(x))

int main(void) {
    int n = SQUARE(5);  /* becomes: int n = ((5)*(5)); */
    return 0;
}
```

### 2.2 Stage 2: Compilation

The compiler translates preprocessed C into **assembly language** specific to the target architecture (x86-64, ARM, RISC-V, etc.).

This stage:
- Performs lexical analysis (tokenization)
- Performs syntactic analysis (parsing → AST)
- Performs semantic analysis (type checking)
- Performs optimization (dead code elimination, loop unrolling, inlining)
- Generates assembly

```bash
# See assembly output:
gcc -S source.c -o source.s
# With optimization:
gcc -S -O2 source.c -o source.s
```

### 2.3 Stage 3: Assembly

The assembler converts assembly text into **object code** — binary machine instructions in a relocatable format (ELF on Linux, Mach-O on macOS, COFF/PE on Windows).

Object files contain:
- `.text` section: machine code
- `.data` section: initialized global/static data
- `.bss` section: uninitialized global/static data (zero-initialized)
- `.rodata` section: read-only data (string literals, const globals)
- Symbol table: list of defined and referenced symbols
- Relocation table: addresses that need to be fixed up by the linker

### 2.4 Stage 4: Linking

The linker combines multiple object files and libraries into a single executable.

```
object_a.o  ──┐
object_b.o  ──┤──► linker (ld) ──► executable
libc.a      ──┘
```

The linker:
- Resolves symbol references (finds where each function/variable is defined)
- Performs relocation (assigns final memory addresses)
- Handles static vs dynamic linking

**Static linking**: Libraries are copied into the executable. Larger binary, no runtime dependencies.

**Dynamic linking**: Libraries stay separate (`.so` on Linux, `.dll` on Windows). Smaller binary, requires libraries at runtime.

```bash
# See linker behavior:
gcc -v source.c      # verbose: shows all steps
gcc -Wl,-Map=out.map source.c  # generate linker map
ldd executable       # show dynamic library dependencies
nm executable        # show symbol table
objdump -d executable  # disassemble
```

### 2.5 Compiler Flags You Must Know

```bash
# Warnings (use ALL of these in real projects):
gcc -Wall          # Enable most warnings
gcc -Wextra        # Enable extra warnings beyond Wall
gcc -Wpedantic     # Strict ISO C compliance warnings
gcc -Werror        # Treat warnings as errors
gcc -Wshadow       # Warn when variable shadows another
gcc -Wformat=2     # Strict printf format checking
gcc -Wconversion   # Warn on implicit type conversions
gcc -Wnull-dereference  # Warn on possible null dereference

# Optimization levels:
gcc -O0   # No optimization (default) — best for debugging
gcc -O1   # Basic optimizations
gcc -O2   # Standard optimizations (most production code)
gcc -O3   # Aggressive optimizations (may increase binary size)
gcc -Os   # Optimize for size
gcc -Og   # Optimize for debugging experience

# Debugging:
gcc -g    # Include debug symbols (DWARF)
gcc -g3   # Include macro expansion info too

# Standards:
gcc -std=c89   # Compile as C89
gcc -std=c99   # Compile as C99
gcc -std=c11   # Compile as C11
gcc -std=c17   # Compile as C17
gcc -std=c23   # Compile as C23

# Sanitizers (catch bugs at runtime):
gcc -fsanitize=address    # AddressSanitizer: buffer overflow, use-after-free
gcc -fsanitize=undefined  # UBSan: undefined behavior detection
gcc -fsanitize=thread     # ThreadSanitizer: data races
gcc -fsanitize=memory     # MemorySanitizer: uninitialized reads (Clang only)

# Architecture:
gcc -m32   # Generate 32-bit code
gcc -m64   # Generate 64-bit code
```

---

## 3. Program Structure & Anatomy

### 3.1 The Complete Anatomy of a C Source File

```
+------------------------------------------------------+
| C SOURCE FILE — Logical Sections (in order)          |
+------------------------------------------------------+
| 1. File header comment (purpose, author, license)    |
| 2. System #include directives                        |
| 3. Local #include directives                         |
| 4. Macro #define constants                           |
| 5. Type definitions (typedef, struct, enum)          |
| 6. Global variable declarations (extern or static)   |
| 7. Function prototypes (declarations)                |
| 8. Function definitions (implementations)            |
| 9. main() — entry point (in main .c file only)       |
+------------------------------------------------------+
```

### 3.2 The Minimal Valid C Program

```c
int main(void) {
    return 0;
}
```

**Why `int main(void)` and not `void main()`?**

The C standard mandates that `main()` returns `int`. `void main()` is undefined behavior in standard C. The return value is the **exit status** passed to the operating system:
- `0` or `EXIT_SUCCESS` → program succeeded
- `1` or `EXIT_FAILURE` → program failed
- Any other positive value → implementation-defined meaning

**`main(void)` vs `main()`**: In C (unlike C++), `main()` means "accepts any arguments." `main(void)` explicitly means "accepts no arguments." Always use `main(void)` when you don't process command-line arguments.

### 3.3 The Two Valid Signatures of main()

```c
/* Signature 1: No command-line arguments */
int main(void) {
    return 0;
}

/* Signature 2: With command-line arguments */
int main(int argc, char *argv[]) {
    /*
     * argc: argument count (always >= 1)
     * argv: argument vector (array of strings)
     * argv[0]: name of the program (or empty string)
     * argv[1] through argv[argc-1]: actual arguments
     * argv[argc]: always NULL (guaranteed by standard)
     */
    return 0;
}

/* Also valid (equivalent): */
int main(int argc, char **argv) { return 0; }

/* C99/C11 also allow: */
int main(int argc, char *argv[], char *envp[]) { return 0; }
/* envp: environment variables (POSIX extension, not strict ISO C) */
```

### 3.4 The #include Directive — How It Really Works

```c
#include <stdio.h>    /* System header: searches compiler's include path */
#include "myfile.h"   /* Local header: searches current directory first */
```

**The include search path** (in order for `<>` headers):
1. Compiler's built-in paths (`/usr/include`, `/usr/local/include`)
2. Paths added with `-I/path/to/dir` flag

**The include search path** for `""` headers:
1. Directory of the source file containing the `#include`
2. Then same as `<>` headers

**What headers contain** (declarations, NOT definitions):
- Function prototypes: `int printf(const char *fmt, ...);`
- Type definitions: `typedef unsigned long size_t;`
- Macro definitions: `#define NULL ((void*)0)`
- Extern variable declarations: `extern int errno;`

### 3.5 Header Files — Rules and Regulations

Every header file MUST have an **include guard** to prevent double inclusion:

```c
/* myheader.h */
#ifndef MYHEADER_H        /* If MYHEADER_H is not defined... */
#define MYHEADER_H        /* ...define it and include contents */

/* All declarations go here */
typedef struct Node {
    int data;
    struct Node *next;
} Node;

int process(Node *n);

#endif /* MYHEADER_H */   /* End of guard */
```

**Alternative: `#pragma once`** (widely supported but NOT standard):
```c
#pragma once
/* declarations */
```

**Rule**: Headers should NEVER contain:
- Function definitions (except `inline` functions)
- Non-`extern` variable definitions
- `using` anything (that's C++)

---

## 4. Data Types, Sizes & Representation

### 4.1 The Type System Overview

```
C TYPES
├── Basic Types
│   ├── Integer Types
│   │   ├── char
│   │   ├── short (int)
│   │   ├── int
│   │   ├── long (int)
│   │   └── long long (int)     [C99]
│   └── Floating Point Types
│       ├── float
│       ├── double
│       └── long double
├── Derived Types
│   ├── Arrays
│   ├── Pointers
│   ├── Structures (struct)
│   ├── Unions (union)
│   └── Functions
├── Void Type
│   └── void
└── Enumerated Types
    └── enum
```

### 4.2 Integer Types — Sizes, Ranges, and Guarantees

The C standard specifies **minimum** sizes, not exact sizes. Actual sizes are platform/implementation-dependent.

```
TYPE              | MINIMUM BITS | TYPICAL (x86-64 LP64) | SIGNED RANGE
------------------+--------------+-----------------------+----------------------------
char              | 8            | 8 bits (1 byte)       | -128 to 127 (or 0 to 255)
signed char       | 8            | 8 bits                | -128 to 127
unsigned char     | 8            | 8 bits                | 0 to 255
short             | 16           | 16 bits (2 bytes)     | -32,768 to 32,767
unsigned short    | 16           | 16 bits               | 0 to 65,535
int               | 16           | 32 bits (4 bytes)     | -2,147,483,648 to 2,147,483,647
unsigned int      | 16           | 32 bits               | 0 to 4,294,967,295
long              | 32           | 64 bits (8 bytes)     | -9.2e18 to 9.2e18 (LP64)
unsigned long     | 32           | 64 bits               | 0 to 1.8e19
long long         | 64           | 64 bits (8 bytes)     | -9.2e18 to 9.2e18
unsigned long long| 64           | 64 bits               | 0 to 1.8e19
```

**Critical Rule**: The C standard only guarantees:
- `char` is at least 8 bits
- `short` >= `char`
- `int` >= `short`
- `long` >= `int`
- `long long` >= `long`
- `sizeof(char)` == 1 **always**

**The `char` ambiguity**: Whether `char` is signed or unsigned is **implementation-defined**. Never assume. Use `signed char` or `unsigned char` explicitly when it matters.

### 4.3 Fixed-Width Integer Types (C99) — Use These for Portability

```c
#include <stdint.h>

/* Exact-width types (may not exist on all platforms) */
int8_t    /* exactly 8 bits, signed  */
int16_t   /* exactly 16 bits, signed */
int32_t   /* exactly 32 bits, signed */
int64_t   /* exactly 64 bits, signed */

uint8_t   /* exactly 8 bits, unsigned  */
uint16_t  /* exactly 16 bits, unsigned */
uint32_t  /* exactly 32 bits, unsigned */
uint64_t  /* exactly 64 bits, unsigned */

/* Minimum-width types (guaranteed to exist) */
int_least8_t    int_least16_t    int_least32_t    int_least64_t
uint_least8_t   uint_least16_t   uint_least32_t   uint_least64_t

/* Fastest types with at least N bits */
int_fast8_t     int_fast16_t     int_fast32_t     int_fast64_t
uint_fast8_t    uint_fast16_t    uint_fast32_t    uint_fast64_t

/* Integer type that can hold a pointer */
intptr_t    /* signed, can hold any void* */
uintptr_t   /* unsigned, can hold any void* */

/* Widest available integer type */
intmax_t    /* largest signed integer type */
uintmax_t   /* largest unsigned integer type */
```

**Printf/scanf format specifiers for fixed-width types** (from `<inttypes.h>`):

```c
#include <inttypes.h>

int32_t x = 42;
printf("%" PRId32 "\n", x);   /* PRId32 expands to "d" on 32-bit int systems */
printf("%" PRIu64 "\n", (uint64_t)100);
```

### 4.4 Floating-Point Types

C uses **IEEE 754** floating-point representation.

```
TYPE        | SIZE    | PRECISION         | RANGE (approx)
------------+---------+-------------------+---------------------------
float       | 32 bits | ~7 decimal digits | ±1.2e-38 to ±3.4e38
double      | 64 bits | ~15-17 digits     | ±2.2e-308 to ±1.8e308
long double | ≥64bits | ≥15 digits        | platform-dependent
            |         | (80-bit x87 on    |
            |         |  x86-64 Linux)    |
```

**IEEE 754 float layout (32-bit)**:
```
 31  30      23  22                    0
 +---+--------+-------------------------+
 | S | Exp(8) |    Mantissa (23 bits)  |
 +---+--------+-------------------------+
   |      |              |
   |      |              └── Fractional part (implicit leading 1)
   |      └── Biased exponent (bias=127): actual exp = stored - 127
   └── Sign bit: 0=positive, 1=negative

Special values:
  Exp=0,   Mantissa=0  → ±0.0
  Exp=255, Mantissa=0  → ±infinity
  Exp=255, Mantissa≠0  → NaN (Not a Number)
  Exp=0,   Mantissa≠0  → Denormalized (subnormal) number
```

**Critical Rules for Floating Point**:
1. NEVER test floats for exact equality: `if (a == b)` is almost always wrong
2. Use epsilon comparison: `fabs(a - b) < 1e-9`
3. Floating-point arithmetic is NOT associative: `(a + b) + c ≠ a + (b + c)` in general
4. Integer → float conversion can lose precision for large integers

```c
#include <float.h>

/* Important constants from <float.h> */
FLT_EPSILON   /* smallest x such that 1.0f + x != 1.0f */
DBL_EPSILON   /* same for double (~2.22e-16) */
FLT_MAX       /* largest finite float (~3.4e38) */
DBL_MAX       /* largest finite double (~1.8e308) */
FLT_MIN       /* smallest normalized positive float */
HUGE_VAL      /* represents infinity for double */
NAN           /* Not a Number constant */
INFINITY      /* Positive infinity */
```

### 4.5 How Integers Are Stored in Memory

**Two's complement representation** (universal in modern CPUs, mandated by C23, de facto in C11):

```
8-bit signed char examples:
+--------+----------+------------------+
| Binary | Unsigned | Signed (2's comp)|
+--------+----------+------------------+
| 00000000|    0    |        0         |
| 00000001|    1    |        1         |
| 01111111|   127   |      127         |
| 10000000|   128   |     -128         |
| 10000001|   129   |     -127         |
| 11111111|   255   |       -1         |
+--------+----------+------------------+

Converting to two's complement negative:
Step 1: Write the positive value in binary
Step 2: Invert all bits (one's complement)
Step 3: Add 1

Example: -5 in 8-bit:
  +5  = 00000101
  inv = 11111010  (one's complement)
  +1  = 11111011  (two's complement = -5)
```

**Bit layout in memory (little-endian x86)**:
```
int x = 0x12345678;

Memory address:  [LOW]                    [HIGH]
Address offset:   0     1     2     3
Content (hex):   78    56    34    12

Each byte in memory stores bits 7:0 of that chunk.
```

### 4.6 The `sizeof` Operator

`sizeof` returns the size in bytes of a type or expression. It is evaluated at **compile time** (except for VLAs in C99).

```c
#include <stdio.h>

int main(void) {
    printf("char:        %zu bytes\n", sizeof(char));       /* always 1 */
    printf("short:       %zu bytes\n", sizeof(short));
    printf("int:         %zu bytes\n", sizeof(int));
    printf("long:        %zu bytes\n", sizeof(long));
    printf("long long:   %zu bytes\n", sizeof(long long));
    printf("float:       %zu bytes\n", sizeof(float));
    printf("double:      %zu bytes\n", sizeof(double));
    printf("long double: %zu bytes\n", sizeof(long double));
    printf("pointer:     %zu bytes\n", sizeof(void *));

    /* sizeof on expressions does NOT evaluate the expression */
    int a = 5;
    sizeof(a++);  /* a is still 5 — no side effect! */
    printf("a = %d\n", a);  /* prints 5 */

    return 0;
}
```

**Format specifier**: `sizeof` returns `size_t`, an unsigned integer type. Use `%zu` in printf.

### 4.7 Type Limits — `<limits.h>`

```c
#include <limits.h>

CHAR_BIT    /* bits per char, always 8 on modern systems */
CHAR_MIN    /* -128 or 0 depending on signedness */
CHAR_MAX    /* 127 or 255 */
SCHAR_MIN   /* -128 */
SCHAR_MAX   /* 127 */
UCHAR_MAX   /* 255 */
SHRT_MIN    /* -32768 */
SHRT_MAX    /* 32767 */
USHRT_MAX   /* 65535 */
INT_MIN     /* -2147483648 */
INT_MAX     /* 2147483647 */
UINT_MAX    /* 4294967295 */
LONG_MIN    /* -2147483648 (32-bit) or -9223372036854775808 (64-bit) */
LONG_MAX    /* 2147483647  (32-bit) or  9223372036854775807 (64-bit) */
LLONG_MIN   /* -9223372036854775808 */
LLONG_MAX   /* 9223372036854775807 */
ULLONG_MAX  /* 18446744073709551615 */
```

---

## 5. Variables: Storage Classes, Qualifiers & Scope

### 5.1 Variable Declaration Syntax

```c
[storage-class] [type-qualifier] type name [= initializer];

/* Examples: */
int x;                        /* uninitialized — garbage value on stack */
int y = 10;                   /* initialized */
static int count = 0;         /* static storage, initialized once */
const int MAX = 100;          /* constant, cannot be modified */
extern int global_var;        /* declared elsewhere, not defined here */
volatile int hardware_reg;    /* may change outside program control */
register int fast;            /* hint: store in CPU register (advisory) */
```

### 5.2 Storage Classes — The 4 Specifiers

#### `auto` (default for local variables)

```c
void func(void) {
    auto int x = 5;  /* 'auto' is implicit for local vars, rarely written */
    int y = 5;       /* identical to above */
    /*
     * - Allocated on the STACK when block is entered
     * - Deallocated when block exits
     * - Uninitialized by default (contains garbage)
     * - Scope: block where declared
     * - Lifetime: duration of block execution
     */
}
```

#### `static` — Two different uses

```c
/* USE 1: Static local variable */
void counter(void) {
    static int count = 0;   /* initialized ONCE, at program start */
    count++;                /* retains value between calls */
    printf("Called %d times\n", count);
    /*
     * - Allocated in .data or .bss segment (NOT stack)
     * - Initialized to 0 if no initializer (guaranteed)
     * - Scope: only within this function
     * - Lifetime: entire program execution
     */
}

/* USE 2: Static at file scope — internal linkage */
static int file_local_var = 42;  /* only visible in this translation unit */
static void helper(void) { ... } /* not visible to other .c files */
/*
 * WITHOUT static: external linkage (visible everywhere)
 * WITH static: internal linkage (only in this .c file)
 * Use static to hide implementation details!
 */
```

#### `extern` — Declare without defining

```c
/* In globals.c: */
int global_counter = 0;     /* DEFINITION: creates storage */

/* In main.c: */
extern int global_counter;  /* DECLARATION: no new storage, refers to globals.c */
extern void some_function(void);  /* declaring a function prototype */

/*
 * Rule: Every variable must be DEFINED exactly once
 * It can be DECLARED (extern) in many places
 * extern is implicit for function prototypes
 */
```

#### `register` — CPU register hint

```c
void loop_example(void) {
    register int i;  /* hint to compiler: keep in CPU register */
    for (i = 0; i < 1000000; i++) {
        /* i is frequently accessed — register hint may speed this up */
    }
    /* 
     * CANNOT take address of register variable: &i is illegal
     * Modern compilers ignore this hint and do their own register allocation
     * Rarely used in modern C — compilers are smarter
     */
}
```

### 5.3 Storage Class Summary Table

```
STORAGE CLASS | STORAGE    | DEFAULT INIT | SCOPE         | LIFETIME
--------------+------------+--------------+---------------+------------------
auto          | Stack      | Undefined    | Block         | Block execution
static(local) | BSS/Data   | Zero         | Block         | Program duration
static(file)  | BSS/Data   | Zero         | File          | Program duration
extern        | BSS/Data   | Zero         | Program-wide  | Program duration
register      | CPU reg    | Undefined    | Block         | Block execution
```

### 5.4 Type Qualifiers

#### `const` — Read-only values

```c
/* const applies to what is to its LEFT (or right if leftmost) */

const int x = 5;        /* x cannot be modified */
int const y = 5;        /* same as above */

/* With pointers — 4 combinations: */

      int *       p1;  /* non-const ptr to non-const int */
const int *       p2;  /* non-const ptr to const int    — can't change *p2 */
      int * const p3;  /* const ptr to non-const int    — can't change p3 */
const int * const p4;  /* const ptr to const int        — can't change either */

/* Memory trick: read right-to-left from the variable name */
/* p2: "p2 is a pointer to const int" → data is read-only */
/* p3: "p3 is a const pointer to int" → pointer is read-only */

/* Function parameters — const promises you won't modify the data */
size_t strlen(const char *s);  /* strlen won't modify the string */

/* const does NOT mean compile-time constant in C (unlike C++) */
const int n = 10;
int arr[n];  /* ILLEGAL in C89/C90! Legal in C99+ only as VLA */
```

#### `volatile` — Prevents optimization

```c
/* volatile tells the compiler: "don't optimize reads/writes away"
 * because this value may change outside the program's control
 */

/* Use cases: */
volatile int *hardware_status_reg = (volatile int *)0x40001000;
/* Hardware register — value changes from outside */

volatile sig_atomic_t signal_received = 0;
/* Modified by signal handler — must be volatile */

/* Example: without volatile, compiler might cache the value in a register */
volatile int flag = 0;
while (flag == 0) {
    /* busy wait — without volatile, compiler might optimize to infinite loop */
}
```

#### `restrict` (C99) — Aliasing hint

```c
/*
 * restrict tells the compiler: "no other pointer in this scope
 * points to the same memory as this pointer"
 * This enables aggressive pointer-related optimizations
 */

/* Standard library uses it: */
void *memcpy(void *restrict dest, const void *restrict src, size_t n);
/* dest and src are guaranteed not to overlap */

/* Your own usage: */
void add_vectors(float *restrict result,
                 const float *restrict a,
                 const float *restrict b,
                 int n) {
    for (int i = 0; i < n; i++) {
        result[i] = a[i] + b[i];  /* compiler can auto-vectorize safely */
    }
}
```

### 5.5 Scope Rules — The 4 Types of Scope

```
SCOPE TYPES:
+-----------+------------------------------------------------+
| Block     | Variables inside { }                           |
| File      | Variables declared at file level (outside fns) |
| Function  | Labels (goto labels only)                      |
| Prototype | Parameter names in function declarations       |
+-----------+------------------------------------------------+
```

```c
int file_scope_var = 10;   /* File scope: visible in entire file */

void func(int proto_scope) {  /* param: function scope starts here */
    int block_scope = 1;       /* block scope: only within this { } */
    {
        int inner = 2;          /* inner block scope */
        /* block_scope is visible here */
        /* inner shadows any outer 'inner' */
    }
    /* inner is NOT visible here */

    if (1) {
        int if_scope = 3;       /* scope limited to if block */
    }
    /* if_scope NOT visible here */

again:                          /* label: function scope (visible anywhere in func) */
    ;
}
```

**Shadowing**: An inner scope variable with the same name as an outer scope variable **hides** the outer one. This is legal but dangerous.

```c
int x = 100;
void func(void) {
    int x = 200;     /* shadows global x inside this function */
    {
        int x = 300; /* shadows function x inside this block */
        printf("%d", x);  /* prints 300 */
    }
    printf("%d", x);  /* prints 200 */
}
/* Use -Wshadow to get warnings about this */
```

### 5.6 Linkage — How Names Are Shared Across Files

```
LINKAGE TABLE:
+------------------+------------------+----------------------------+
| Declaration      | Where            | Linkage                    |
+------------------+------------------+----------------------------+
| int x;           | File scope       | External (visible globally)|
| static int x;    | File scope       | Internal (this .c only)    |
| int x;           | Block scope      | None (local only)          |
| extern int x;    | Anywhere         | External (reference)       |
| void func() {}   | File scope       | External                   |
| static void f(){} | File scope      | Internal (this .c only)    |
+------------------+------------------+----------------------------+
```

---

## 6. Operators: Complete Reference with Precedence

### 6.1 Arithmetic Operators

```c
int a = 17, b = 5;

a + b   /* 22  — addition */
a - b   /* 12  — subtraction */
a * b   /* 85  — multiplication */
a / b   /* 3   — integer division (truncates toward zero in C99+) */
a % b   /* 2   — modulo (remainder). Sign matches dividend in C99+ */

/* Integer division truncation rules (C99): */
 7 / 2  =  3   (truncates toward 0, not -∞)
-7 / 2  = -3   (NOT -4)
 7 / -2 = -3   (NOT -4)
-7 / -2 =  3

/* Modulo sign rule (C99): (a/b)*b + a%b == a */
 7 %  2 =  1
-7 %  2 = -1
 7 % -2 =  1
-7 % -2 = -1

/* DANGER: Integer overflow */
int x = INT_MAX;
x + 1;  /* UNDEFINED BEHAVIOR for signed integers! */
/* Unsigned overflow wraps around (defined behavior): */
unsigned int u = UINT_MAX;
u + 1;  /* = 0, well-defined modulo 2^32 */
```

### 6.2 Relational Operators

```c
/* All return int: 0 (false) or 1 (true) */
a == b   /* equal */
a != b   /* not equal */
a <  b   /* less than */
a >  b   /* greater than */
a <= b   /* less than or equal */
a >= b   /* greater than or equal */

/* DANGER: confusing = with == */
if (x = 5) { ... }   /* assigns 5 to x, then evaluates as 5 (truthy) */
if (x == 5) { ... }  /* compares x to 5 */

/* Yoda conditions (prevent the bug): */
if (5 == x) { ... }  /* if you accidentally write 5 = x, compiler errors */
```

### 6.3 Logical Operators

```c
/* Short-circuit evaluation: right side NOT evaluated if result known */
a && b   /* logical AND: true if both nonzero */
         /* b NOT evaluated if a is 0 */
a || b   /* logical OR: true if either nonzero */
         /* b NOT evaluated if a is nonzero */
!a       /* logical NOT: 1 if a==0, 0 if a!=0 */

/* Short-circuit example: */
int *p = NULL;
if (p != NULL && *p > 0) {
    /* *p is only dereferenced if p != NULL */
    /* Safe because of short-circuit */
}

/* Difference from bitwise: */
0xFF && 0x01  =  1    (logical: both nonzero → true)
0xFF &  0x01  =  0x01 (bitwise: AND each bit)
```

### 6.4 Bitwise Operators

```c
unsigned int a = 0b10110100;  /* 0xB4 = 180 */
unsigned int b = 0b11001010;  /* 0xCA = 202 */

a & b    /* AND:  10000000  = 0x80 */
a | b    /* OR:   11111110  = 0xFE */
a ^ b    /* XOR:  01111110  = 0x7E */
~a       /* NOT:  01001011  = 0x4B (all bits flipped) */
a << 1   /* left shift by 1:  multiply by 2 */
a >> 1   /* right shift by 1: divide by 2 (arithmetic for signed, logical for unsigned) */

/* RULE: Always use UNSIGNED types for bitwise operations */
/* Shifting signed integers into/through the sign bit is undefined behavior */

/* Common bit manipulation patterns: */

/* Set bit N:   */ value |=  (1u << N);
/* Clear bit N: */ value &= ~(1u << N);
/* Toggle bit N:*/ value ^=  (1u << N);
/* Test bit N:  */ if (value & (1u << N)) { ... }

/* Extract bits [high:low]: */
uint32_t bits = (value >> low) & ((1u << (high - low + 1)) - 1);

/* Check if power of 2: */
int is_power_of_2 = (n > 0) && ((n & (n - 1)) == 0);

/* Round up to next power of 2: */
uint32_t v = x - 1;
v |= v >> 1; v |= v >> 2; v |= v >> 4; v |= v >> 8; v |= v >> 16;
v++;

/* Swap without temp: */
a ^= b;
b ^= a;
a ^= b;
```

### 6.5 Assignment Operators

```c
x = 5;    /* simple assignment */
x += 5;   /* x = x + 5  */
x -= 5;   /* x = x - 5  */
x *= 5;   /* x = x * 5  */
x /= 5;   /* x = x / 5  */
x %= 5;   /* x = x % 5  */
x &= 5;   /* x = x & 5  */
x |= 5;   /* x = x | 5  */
x ^= 5;   /* x = x ^ 5  */
x <<= 2;  /* x = x << 2 */
x >>= 2;  /* x = x >> 2 */

/* Assignment is an expression that produces the assigned value */
int a, b, c;
a = b = c = 0;  /* right-to-left: c=0, b=0, a=0 */
```

### 6.6 Increment and Decrement Operators

```c
int x = 5;

/* Prefix: increment FIRST, then return new value */
++x;  /* x becomes 6, expression value = 6 */

/* Postfix: return current value FIRST, then increment */
x++;  /* expression value = 5, then x becomes 6 */

/* This distinction matters in expressions: */
int a = 5;
int b = ++a;  /* a=6, b=6 */
int c = a++;  /* c=6, a=7 */

/* UNDEFINED BEHAVIOR — multiple modifications without sequence point: */
int i = 5;
i = i++;        /* undefined: modifying i twice */
a[i] = i++;     /* undefined: reading and modifying i */
f(i++, i++);    /* undefined: order of argument evaluation unspecified */

/* Safe usage: */
for (int i = 0; i < n; i++) { ... }  /* i++ here is fine */
```

### 6.7 The Ternary Operator

```c
/* condition ? value_if_true : value_if_false */
int max = (a > b) ? a : b;
const char *label = (score >= 60) ? "pass" : "fail";

/* Nested ternary (C evaluates right-to-left, but avoid for readability): */
int grade = (score >= 90) ? 4 :
            (score >= 80) ? 3 :
            (score >= 70) ? 2 :
            (score >= 60) ? 1 : 0;

/* The ternary is an EXPRESSION (has a value), if/else is a STATEMENT */
/* Use ternary when you need an expression; use if/else for complex logic */
```

### 6.8 Comma Operator

```c
/* Evaluates left to right, result is rightmost expression */
int x = (a = 1, b = 2, a + b);  /* x = 3, a=1, b=2 */

/* Most common legitimate use: for loop with multiple counters */
for (int i = 0, j = 10; i < j; i++, j--) {
    printf("%d %d\n", i, j);
}
/* Here: i++, j-- are two expressions joined by comma */
```

### 6.9 Other Operators

```c
sizeof(type)      /* size of type in bytes */
sizeof expression /* size of expression's type */
_Alignof(type)    /* alignment requirement of type (C11) */
(type)expr        /* cast expression to type */
&var              /* address-of: get pointer to var */
*ptr              /* dereference: get value at pointer */
->                /* member access through pointer */
.                 /* member access through struct value */
[i]               /* array subscript */
func()            /* function call */
```

### 6.10 Operator Precedence Table (Highest to Lowest)

```
PREC | OPERATOR(S)                      | ASSOCIATIVITY
-----+----------------------------------+----------------
 1   | () [] -> . ++ -- (postfix)       | Left to right
 2   | ++ -- (prefix) + - ! ~ (type)   | Right to left
     | * & sizeof _Alignof              |
 3   | * / %                            | Left to right
 4   | + -                              | Left to right
 5   | << >>                            | Left to right
 6   | < <= > >=                        | Left to right
 7   | == !=                            | Left to right
 8   | &                                | Left to right
 9   | ^                                | Left to right
10   | |                                | Left to right
11   | &&                               | Left to right
12   | ||                               | Left to right
13   | ?:                               | Right to left
14   | = += -= *= /= %= &= |= ^= <<= >>= | Right to left
15   | , (comma)                        | Left to right
```

**The Golden Rule**: When in doubt, use parentheses. Don't rely on memorizing all precedence rules — write `(a & b) == 0` not `a & b == 0` (which is `a & (b == 0)` — a common bug!).

---

## 7. Control Flow — Every Construct

### 7.1 The `if` Statement

```c
/* Basic form */
if (condition) {
    /* executed if condition != 0 */
}

/* if-else */
if (condition) {
    /* true branch */
} else {
    /* false branch */
}

/* if-else chain */
if (score >= 90) {
    grade = 'A';
} else if (score >= 80) {
    grade = 'B';
} else if (score >= 70) {
    grade = 'C';
} else {
    grade = 'F';
}

/* ALWAYS USE BRACES — even for single statements */
if (x) foo();           /* dangerous — easy to add another statement wrongly */
if (x) { foo(); }       /* always safe */

/* Dangling else — which if does this else belong to? */
if (a)
    if (b)
        foo();
    else        /* belongs to INNER if (b), not outer if (a) */
        bar();

/* To bind else to outer if, use braces: */
if (a) {
    if (b)
        foo();
} else {         /* now clearly belongs to if (a) */
    bar();
}
```

### 7.2 The `switch` Statement

```c
switch (expression) {
    case CONST1:
        /* executed if expression == CONST1 */
        break;   /* REQUIRED to prevent fallthrough */
    case CONST2:
    case CONST3:  /* multiple cases for same code */
        /* executed if expression == CONST2 or CONST3 */
        break;
    default:
        /* executed if no case matches */
        /* can be anywhere but convention is last */
        break;
}

/* Rules: */
/* 1. expression must be an integer type (char, int, long, enum) */
/* 2. case values must be integer CONSTANT expressions */
/* 3. No two cases can have the same value */
/* 4. break exits the switch; without break: FALLTHROUGH occurs */

/* Intentional fallthrough (document it): */
switch (cmd) {
    case CMD_SAVE:
    case CMD_SAVE_AS:
        /* fallthrough is intentional */
        /* In C17, GCC accepts: __attribute__((fallthrough)); */
        /* In C23: [[fallthrough]]; */
        save_file();
        break;
    default:
        break;
}

/* switch vs if-else: switch can be compiled to a jump table — O(1) */
/* if-else chain is O(n) — switch is faster for many cases */

/* DANGER: declarations in switch */
switch (x) {
    int y;          /* ILLEGAL: declaration not in a case */
    case 1:
        int z = 5;  /* ILLEGAL in C89; legal in C99 but jumps over init */
        break;
    case 2:
        break;
}
/* Use blocks to avoid: */
switch (x) {
    case 1: {
        int z = 5;  /* safe inside a block */
        break;
    }
    case 2:
        break;
}
```

### 7.3 The `for` Loop

```c
for (init; condition; update) {
    body;
}

/* Execution order:
   1. init (once, before loop)
   2. condition: if false, exit loop
   3. body
   4. update
   5. goto 2
*/

/* Classic counter loop */
for (int i = 0; i < n; i++) {
    printf("%d\n", i);
}

/* Counting down */
for (int i = n - 1; i >= 0; i--) {
    arr[i] = 0;
}

/* Multiple variables (using comma operator) */
for (int i = 0, j = n - 1; i < j; i++, j--) {
    int tmp = arr[i]; arr[i] = arr[j]; arr[j] = tmp;
}

/* Any part can be empty */
for (;;) {            /* infinite loop */
    if (done) break;
}

for (init;;) {        /* no condition — never terminates without break */
    if (done) break;
    update;
}

/* Scope of loop variable (C99): */
for (int i = 0; i < n; i++) { ... }
/* i is NOT accessible after the loop */
```

### 7.4 The `while` Loop

```c
while (condition) {
    body;
}

/* Execution order:
   1. condition: if false, skip body
   2. body
   3. goto 1
*/

/* Use when: number of iterations unknown before starting */
int c;
while ((c = getchar()) != EOF) {
    putchar(c);
}

/* Common pattern: process until sentinel */
while (scanf("%d", &n) == 1) {
    process(n);
}
```

### 7.5 The `do-while` Loop

```c
do {
    body;
} while (condition);

/* Execution order:
   1. body (always executes at least ONCE)
   2. condition: if false, exit; if true, goto 1
*/

/* Use when: body must execute at least once */
int choice;
do {
    printf("Enter 1-5: ");
    scanf("%d", &choice);
} while (choice < 1 || choice > 5);

/* Macro trick: wrap multi-statement macros in do-while */
#define SWAP(a, b, type) do { \
    type _tmp = (a); \
    (a) = (b); \
    (b) = _tmp; \
} while (0)

/* Without do-while wrapper, this breaks: */
if (x > y)
    SWAP(x, y, int);   /* expands to if (x>y) { stmt1; stmt2; }; — broken! */
/* With do-while: safe with any if/else usage */
```

### 7.6 `break` and `continue`

```c
/* break: exits the innermost loop or switch */
for (int i = 0; i < n; i++) {
    if (arr[i] == target) {
        found = i;
        break;  /* exits the for loop */
    }
}

/* continue: skips rest of current iteration */
for (int i = 0; i < n; i++) {
    if (arr[i] < 0) continue;  /* skip negative numbers */
    process(arr[i]);
}

/* Breaking out of nested loops — use a flag or goto: */
int done = 0;
for (int i = 0; i < m && !done; i++) {
    for (int j = 0; j < n; j++) {
        if (matrix[i][j] == target) {
            done = 1;
            break;  /* exits inner loop */
        }
    }
}
/* OR use goto (legitimate use case): */
for (int i = 0; i < m; i++) {
    for (int j = 0; j < n; j++) {
        if (matrix[i][j] == target)
            goto found;
    }
}
found:
printf("Found!\n");
```

### 7.7 The `goto` Statement

```c
/* goto: unconditional jump to a label */
goto label;
...
label:
    statement;

/* LEGITIMATE uses of goto in C: */

/* 1. Error cleanup (most common in systems code) */
int process_file(const char *path) {
    FILE *f = fopen(path, "r");
    if (!f) return -1;

    char *buf = malloc(BUF_SIZE);
    if (!buf) goto cleanup_file;

    int *data = malloc(DATA_SIZE);
    if (!data) goto cleanup_buf;

    /* ... actual processing ... */
    int result = do_work(f, buf, data);

    free(data);
cleanup_buf:
    free(buf);
cleanup_file:
    fclose(f);
    return result;  /* or -1 if error path taken */
}

/* 2. Breaking out of nested loops (shown above) */

/* Rules: */
/* - goto cannot jump FORWARD over a variable initialization */
/* - goto cannot jump INTO a block */
/* - goto CAN jump backward */
```

### 7.8 The `return` Statement

```c
/* return exits the current function and optionally returns a value */
int add(int a, int b) {
    return a + b;  /* returns the value of a+b */
}

void print_hello(void) {
    printf("Hello\n");
    return;  /* optional in void functions at the end */
}

/* RULE: Non-void functions MUST return a value on all paths */
int max(int a, int b) {
    if (a > b) return a;
    return b;  /* every path returns */
}

/* Falling off the end of a non-void function is undefined behavior */
int broken(int x) {
    if (x > 0) return x;
    /* NO return here — UB if x <= 0 */
}
```

---

## 8. Functions — Deep Mechanics

### 8.1 Function Declaration vs Definition

```c
/* DECLARATION (prototype): tells compiler the function exists */
int add(int a, int b);     /* full prototype */
int add(int, int);         /* parameter names optional in declaration */

/* DEFINITION: the actual implementation */
int add(int a, int b) {
    return a + b;
}

/* A function must be DECLARED before it is CALLED */
/* The definition also serves as a declaration */

/* RULE: Put all function prototypes in a header file */
/* RULE: Never call a function without a prior declaration */
```

### 8.2 How the Call Stack Works

```
MEMORY LAYOUT (grows downward on most architectures):

High Address
+---------------------------+  ← bottom of stack (initial)
|  main() stack frame       |
|  - return address         |
|  - saved registers        |
|  - local variables        |
|  - arguments to callees   |
+---------------------------+
|  foo() stack frame        |  ← called from main
|  - return address (in main)|
|  - saved rbp              |
|  - local variables        |
+---------------------------+
|  bar() stack frame        |  ← called from foo
|  - return address (in foo) |
|  - local variables        |
+---------------------------+  ← current stack top (RSP register)
|                           |
|  (grows downward →)       |
Low Address
```

```c
/* Stack frame example: */
int bar(int x) {
    int local = x * 2;    /* on bar's stack frame */
    return local;
}

int foo(int a, int b) {
    int result = bar(a + b);  /* a, b are on foo's frame */
    return result;            /* bar's frame is gone after return */
}

int main(void) {
    int val = foo(3, 4);  /* all automatic vars go on the stack */
    return 0;
}
```

### 8.3 Pass by Value — The Fundamental Rule

**C ONLY passes arguments by value.** A function receives a **copy** of the argument.

```c
void double_it(int x) {
    x *= 2;  /* modifies the LOCAL COPY only */
    printf("Inside: %d\n", x);  /* 10 */
}

int main(void) {
    int n = 5;
    double_it(n);
    printf("Outside: %d\n", n);  /* still 5 — not modified! */
    return 0;
}

/* To modify the caller's variable, pass a POINTER: */
void double_it_real(int *x) {
    *x *= 2;  /* modifies the value at the address */
}

int main(void) {
    int n = 5;
    double_it_real(&n);  /* pass address */
    printf("n = %d\n", n);  /* 10 — modified! */
    return 0;
}
```

### 8.4 Function Pointers

A function pointer stores the address of a function, allowing functions to be passed as arguments, returned from functions, or stored in arrays.

```c
/* Syntax: returntype (*name)(paramtypes) */
int (*fp)(int, int);    /* pointer to function taking 2 ints, returning int */

/* Assigning: */
int add(int a, int b) { return a + b; }
int sub(int a, int b) { return a - b; }

fp = add;     /* assign without & (function name decays to pointer) */
fp = &add;    /* also valid — & is optional for functions */

/* Calling: */
int result = fp(3, 4);    /* calls add(3, 4) */
int result2 = (*fp)(3, 4); /* also valid — dereference is optional */

/* Using typedef for readability: */
typedef int (*BinaryOp)(int, int);

BinaryOp ops[] = { add, sub };
int r = ops[0](10, 5);  /* calls add */

/* Function pointer as parameter (callbacks): */
void apply_to_array(int *arr, int n, int (*transform)(int)) {
    for (int i = 0; i < n; i++) {
        arr[i] = transform(arr[i]);
    }
}

int double_val(int x) { return x * 2; }
int square_val(int x) { return x * x; }

int main(void) {
    int arr[] = {1, 2, 3, 4, 5};
    apply_to_array(arr, 5, double_val);  /* [2, 4, 6, 8, 10] */
    apply_to_array(arr, 5, square_val); /* [4, 16, 36, 64, 100] */
    return 0;
}

/* qsort uses a function pointer comparator: */
#include <stdlib.h>

int compare_ints(const void *a, const void *b) {
    int ia = *(const int *)a;
    int ib = *(const int *)b;
    return (ia > ib) - (ia < ib);  /* returns -1, 0, or 1 */
}

int arr[] = {5, 1, 3, 2, 4};
qsort(arr, 5, sizeof(int), compare_ints);
```

### 8.5 Recursion

```c
/* A function calling itself. Must have: */
/* 1. Base case: condition to stop recursion */
/* 2. Recursive case: reduce problem and recurse */

/* Factorial: */
long long factorial(int n) {
    if (n <= 1) return 1;          /* base case */
    return n * factorial(n - 1);  /* recursive case */
}

/* Call stack for factorial(4): */
/*
   factorial(4)
     └─ factorial(3)
          └─ factorial(2)
               └─ factorial(1)
                    └─ returns 1
               ← returns 2*1 = 2
          ← returns 3*2 = 6
     ← returns 4*6 = 24
*/

/* Fibonacci (inefficient — O(2^n) calls): */
int fib(int n) {
    if (n <= 1) return n;
    return fib(n-1) + fib(n-2);
}

/* Better: tail recursion (compiler may optimize to loop) */
long long fib_tail(int n, long long a, long long b) {
    if (n == 0) return a;
    if (n == 1) return b;
    return fib_tail(n - 1, b, a + b);
}
long long fibonacci(int n) { return fib_tail(n, 0, 1); }

/* Stack depth limit: deep recursion causes stack overflow */
/* Default stack size: ~1-8MB on most systems */
/* Rule of thumb: recursion depth > 10,000 → consider iteration */
```

### 8.6 Variadic Functions

Functions that accept a variable number of arguments.

```c
#include <stdarg.h>

/* Syntax: at least one fixed argument, then ... */
double average(int count, ...) {
    va_list args;      /* declare argument list */
    va_start(args, count);  /* initialize; last named param = count */

    double sum = 0;
    for (int i = 0; i < count; i++) {
        sum += va_arg(args, double);  /* get next arg as double */
    }

    va_end(args);      /* cleanup (required!) */
    return sum / count;
}

int main(void) {
    printf("%.2f\n", average(3, 1.0, 2.0, 3.0));  /* 2.00 */
    printf("%.2f\n", average(5, 1.0, 2.0, 3.0, 4.0, 5.0));  /* 3.00*/
    return 0;
}

/* Rules: */
/* 1. Must have at least one fixed argument */
/* 2. va_arg must be called exactly count times */
/* 3. va_arg type must match the actual type passed */
/* 4. char and short are promoted to int; float promoted to double */
/* 5. NO type safety — caller and callee must agree on types */

/* printf is the canonical example: */
/* int printf(const char *fmt, ...); */
```

### 8.7 Inline Functions (C99)

```c
/* inline suggests compiler replace the call with the function body */
/* Avoids function call overhead for small, frequently-called functions */

static inline int max(int a, int b) {
    return a > b ? a : b;
}

/* Rules: */
/* 1. inline is a HINT — compiler may ignore it */
/* 2. Definition must be in same translation unit where it's called */
/* 3. Use 'static inline' in headers to avoid multiple-definition errors */
/* 4. Don't inline large functions — code bloat outweighs benefit */
```

### 8.8 The `_Noreturn` Specifier (C11)

```c
#include <stdnoreturn.h>

/* Functions that never return (exit, abort, infinite loops) */
_Noreturn void fatal_error(const char *msg) {
    fprintf(stderr, "Fatal: %s\n", msg);
    exit(EXIT_FAILURE);
    /* If function returns here, UB — compiler may optimize assuming it doesn't */
}

/* Or with the macro from stdnoreturn.h: */
noreturn void panic(const char *msg) {
    abort();
}
```
