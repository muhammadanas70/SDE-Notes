# The Complete C Keywords Reference
## A Deep, Uncompromising Guide for Systems Programmers

> *"Knowing the name of something is not the same as knowing that thing."*
> — Richard Feynman

---

## Table of Contents

1. [What Is a Keyword in C?](#what-is-a-keyword)
2. [The C Standards Timeline — Why It Matters](#c-standards-timeline)
3. [CATEGORY 1 — Data Type Keywords](#category-1-data-type-keywords)
   - [int](#int)
   - [char](#char)
   - [float](#float)
   - [double](#double)
   - [short](#short)
   - [long](#long)
   - [signed / unsigned](#signed-unsigned)
   - [void](#void)
4. [CATEGORY 2 — Storage Class Keywords](#category-2-storage-class-keywords)
   - [auto](#auto)
   - [register](#register)
   - [static](#static)
   - [extern](#extern)
5. [CATEGORY 3 — Type Qualifier Keywords](#category-3-type-qualifier-keywords)
   - [const](#const)
   - [volatile](#volatile)
   - [restrict (C99)](#restrict)
6. [CATEGORY 4 — Control Flow Keywords](#category-4-control-flow-keywords)
   - [if / else](#if-else)
   - [switch / case / default](#switch-case-default)
   - [break](#break)
   - [continue](#continue)
   - [return](#return)
   - [goto](#goto)
7. [CATEGORY 5 — Loop Keywords](#category-5-loop-keywords)
   - [for](#for)
   - [while](#while)
   - [do](#do)
8. [CATEGORY 6 — Aggregate Type Keywords](#category-6-aggregate-type-keywords)
   - [struct](#struct)
   - [union](#union)
   - [enum](#enum)
   - [typedef](#typedef)
9. [CATEGORY 7 — Size and Alignment Keywords](#category-7-size-and-alignment-keywords)
   - [sizeof](#sizeof)
   - [_Alignas / _Alignof (C11)](#alignas-alignof)
10. [CATEGORY 8 — Function Keywords](#category-8-function-keywords)
    - [inline (C99)](#inline)
    - [_Noreturn (C11)](#noreturn)
11. [CATEGORY 9 — C99 Special Type Keywords](#category-9-c99-special-type-keywords)
    - [_Bool](#bool)
    - [_Complex / _Imaginary](#complex-imaginary)
12. [CATEGORY 10 — C11 Concurrency and Atomics](#category-10-c11-concurrency)
    - [_Atomic](#atomic)
    - [_Thread_local](#thread-local)
13. [CATEGORY 11 — C11 Generic and Static Assert](#category-11-generic-static-assert)
    - [_Generic](#generic)
    - [_Static_assert](#static-assert)
14. [CATEGORY 12 — C23 New Keywords](#category-12-c23-keywords)
15. [The Linux Kernel — How It Uses These Keywords](#linux-kernel-patterns)
16. [Fatal Mistakes Master Reference](#fatal-mistakes-master-reference)
17. [Misinformation Hall of Fame](#misinformation-hall-of-fame)
18. [Mental Models for Expert C Thinking](#mental-models)

---

## What Is a Keyword? {#what-is-a-keyword}

A **keyword** (also called a *reserved word*) is an identifier that the C language standard reserves for its own use. You **cannot** use keywords as variable names, function names, struct names, or any other user-defined identifier.

```
C SOURCE CODE
     │
     ▼
┌──────────────────────────────────────────┐
│              C PREPROCESSOR             │
│   Handles #include, #define, #ifdef     │
│   Macros are NOT keywords — they're     │
│   text substitution before compilation  │
└──────────────────────┬───────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────┐
│              C COMPILER                 │
│                                         │
│  LEXER: Breaks source into tokens       │
│         KEYWORDS are recognized here    │
│         as special tokens — they drive  │
│         the grammar of the language     │
│                                         │
│  PARSER: Builds AST using keyword rules │
│                                         │
│  SEMANTIC ANALYSIS: Type checking       │
│                                         │
│  CODE GENERATION: Machine instructions  │
└──────────────────────────────────────────┘
```

**Critical distinction** that most beginners miss:
- `NULL` is **NOT** a keyword — it is a macro defined in `<stddef.h>` (expands to `(void*)0` or `0`)
- `true` / `false` in C99 are **NOT** keywords — they are macros in `<stdbool.h>`
- `true` / `false` in C23 **ARE** keywords
- `size_t` is **NOT** a keyword — it is a typedef
- `printf` is **NOT** a keyword — it is a library function

---

## The C Standards Timeline — Why It Matters {#c-standards-timeline}

```
YEAR    STANDARD    KEYWORDS ADDED
─────────────────────────────────────────────────────────────────
1972    K&R C       Original Dennis Ritchie C (no formal standard)
                    No prototypes. int was default everywhere.

1978    K&R (book)  The C Programming Language published.
                    Still no formal standard.

1989    C89/ANSI C  32 keywords formalized:
                    auto break case char const continue default
                    do double else enum extern float for goto
                    if int long register return short signed
                    sizeof static struct switch typedef union
                    unsigned void volatile while

1990    C90/ISO C   Identical to C89 (ISO ratified ANSI standard)

1999    C99         5 new keywords:
                    inline restrict _Bool _Complex _Imaginary

2011    C11         7 new keywords:
                    _Alignas _Alignof _Atomic _Generic
                    _Noreturn _Static_assert _Thread_local

2018    C17/C18     Bug fix release. NO new keywords.

2023    C23         New keywords:
                    alignas alignof bool false nullptr
                    static_assert thread_local true typeof
                    typeof_unqual _BitInt constexpr
─────────────────────────────────────────────────────────────────
```

**Why this matters for you**: When you compile with `-std=c89`, keywords like `inline` are not recognized. When you read Linux kernel code, you will see `__inline__`, `__volatile__`, `__asm__` — these are GCC-specific spellings that work in strict C89 mode. Understanding standards prevents mysterious compilation errors.

---

# CATEGORY 1 — Data Type Keywords {#category-1-data-type-keywords}

These keywords describe **what kind of data** a variable holds and how many bytes the machine uses to store it.

---

## `int` {#int}

### What It Actually Means

`int` means **integer** — a whole number (no decimal point). But the crucial fact that most beginners get wrong:

> **`int` does NOT have a fixed size. It is defined to be the "natural word size" of the target platform.**

The C standard says `int` must be **at least 16 bits**. On modern 32-bit and 64-bit systems it is almost always 32 bits, but this is a **convention**, not a guarantee.

```
ARCHITECTURE REALITY:

Platform              sizeof(int)   Range
──────────────────────────────────────────────────────
16-bit (old 8051)        2 bytes    -32768 to 32767
32-bit Linux/Windows     4 bytes    -2147483648 to 2147483647
64-bit Linux             4 bytes    -2147483648 to 2147483647
64-bit Windows           4 bytes    Same
AVR microcontroller      2 bytes    -32768 to 32767
MSVC /Wp64 mode          4 bytes    Same
──────────────────────────────────────────────────────
Note: On 64-bit Linux, int is still 4 bytes.
      long is 8 bytes on Linux but 4 bytes on Windows 64-bit.
      This is the LP64 vs LLP64 ABI difference.
```

**The Memory Layout of int (32-bit, little-endian)**:

```
int x = 305419896;   /* 0x12345678 */

Memory Address    Low ◄─────────────────────► High
                  ┌────────┬────────┬────────┬────────┐
Address:          │  0x00  │  0x01  │  0x02  │  0x03  │
                  ├────────┼────────┼────────┼────────┤
Byte value:       │  0x78  │  0x56  │  0x34  │  0x12  │
                  └────────┴────────┴────────┴────────┘
                  Least                         Most
                  Significant                Significant
                  Byte                          Byte

This is LITTLE-ENDIAN (x86, ARM in LE mode, RISC-V).
BIG-ENDIAN (SPARC, old MIPS, network byte order) stores 0x12 first.

Bit layout of value 305419896:
Bit 31 (MSB, sign bit)                         Bit 0 (LSB)
   │                                               │
   ▼                                               ▼
   0 001 0010  0011 0100  0101 0110  0111 1000
   ^sign=0 means positive
```

### Exact Usage Patterns

```c
/* CORRECT USAGE */

/* 1. Simple counter — acceptable when range is known small */
int i;
for (i = 0; i < 100; i++) { }

/* 2. Function return values — very idiomatic in C */
int result = some_function();
if (result < 0) {
    /* error */
}

/* 3. File descriptor (Linux) — always int, never size_t */
int fd = open("/etc/passwd", O_RDONLY);

/* 4. Boolean-like conditions (C89 style) */
int found = 0;
/* ... */
if (found) { }
```

### Common Mistakes People Make

**Mistake 1: Assuming int is 64-bit on a 64-bit system**

```c
/* WRONG — on 64-bit Linux, this may silently truncate */
long ptr_val = (long)some_pointer;  /* long is 8 bytes here, so OK on Linux */
int ptr_val2 = (int)some_pointer;   /* WRONG — int is 4 bytes, pointer is 8 bytes */

/* CORRECT — use intptr_t from <stdint.h> */
#include <stdint.h>
intptr_t ptr_val = (intptr_t)some_pointer;  /* Guaranteed pointer-sized integer */
```

**Mistake 2: Integer overflow — undefined behavior, NOT wrapping**

```c
/* This is UNDEFINED BEHAVIOR in C, not guaranteed wrap-around */
int x = INT_MAX;  /* 2147483647 */
int y = x + 1;    /* UB! Compiler may optimize this out entirely */

/*
   GCC with -O2 may see this pattern and eliminate the overflow check:
   if (x + 1 < x) { handle_overflow(); }
   becomes: (nothing) — because "signed overflow never happens" per UB rules
*/

/* CORRECT: Use unsigned for defined wrap, or check before adding */
#include <limits.h>
if (x > INT_MAX - 1) {
    /* overflow would occur */
}

/* Or use compiler builtins (GCC/Clang) */
int result;
if (__builtin_sadd_overflow(x, 1, &result)) {
    /* overflow detected */
}
```

**Mistake 3: int vs. size_t for array sizes and indices**

```c
/* WRONG — size_t is unsigned. If array size > INT_MAX this wraps */
int len = strlen(str);  /* strlen returns size_t, which can be 8 bytes */
for (int i = 0; i < len; i++) { }

/* Also WRONG — comparing signed and unsigned causes compiler warning and logic bugs */
int n = -1;
size_t sz = 10;
if (n < sz) { }  /* n is converted to size_t: -1 becomes huge number! Condition is FALSE */

/* CORRECT */
size_t len = strlen(str);
for (size_t i = 0; i < len; i++) { }
```

### Linux Kernel Usage

```c
/*
 * In Linux kernel source (fs/read_write.c style):
 * File descriptors are always int — this is a POSIX contract.
 * Return values from syscalls are int (negative = error via errno).
 */

/* From linux/include/linux/types.h philosophy: */
typedef __s32  s32;   /* Kernel uses explicit-width types */
typedef __u32  u32;
typedef __s64  s64;
typedef __u64  u64;

/*
 * The kernel AVOIDS plain 'int' for sizes and counts.
 * It uses:
 *   size_t    for memory sizes
 *   ssize_t   for sizes that can be negative (error)
 *   loff_t    for file offsets (64-bit even on 32-bit systems)
 *   pid_t     for process IDs
 *   uid_t     for user IDs
 */

/* Correct kernel-style function signature */
ssize_t my_read(struct file *file, char __user *buf, size_t count, loff_t *pos);
/*              ──────────────────  ────────────────  ──────────    ──────────
                File object         User buffer ptr   Byte count    File offset
                                    __user = in       size_t NOT    64-bit
                                    user space        int           always     */
```

---

## `char` {#char}

### What It Actually Means

`char` means **character** and is guaranteed to be exactly **1 byte** (8 bits on virtually all modern systems). BUT — and this is a critical subtlety that causes real bugs:

> **Whether `char` is signed or unsigned is IMPLEMENTATION-DEFINED. The compiler decides.**

```
THREE DISTINCT char TYPES IN C:
┌─────────────────────────────────────────────────────────────────┐
│  char         — implementation-defined signedness               │
│                 (could be signed or unsigned depending on       │
│                  compiler/platform/flags)                       │
│                                                                 │
│  signed char  — guaranteed -128 to 127                         │
│                                                                 │
│  unsigned char — guaranteed 0 to 255                           │
│                                                                 │
│  These are THREE DISTINCT TYPES. char ≠ signed char            │
│  even if char happens to be signed on your platform.           │
└─────────────────────────────────────────────────────────────────┘

Platform defaults:
  x86/x86-64 Linux (GCC):   char is SIGNED   (-128 to 127)
  ARM Linux (GCC):           char is UNSIGNED (0 to 255) by default
  ARM with -fsigned-char:    char is SIGNED
  MSVC (Windows):            char is SIGNED
```

**Memory Layout**:

```
char c = 'A';   /* ASCII 65 = 0x41 */

Address  ┌────────┐
  0x00   │  0x41  │  (1 byte, 8 bits: 0100 0001)
         └────────┘

signed char interpretation:   bit 7 is sign bit
                               0100 0001 = +65 ✓

unsigned char interpretation:  all bits are value
                               0100 0001 = 65 ✓

char c2 = 0xFF;
signed char:    1111 1111 = -1
unsigned char:  1111 1111 = 255

THIS DIFFERENCE CAUSES REAL BUGS.
```

### The Promoted-to-int Trap

```c
/*
 * CRITICAL: In C, char is ALWAYS promoted to int when used
 * in expressions. This is called "integer promotion."
 */

char c = 0xFF;           /* On ARM (unsigned char): value is 255 */
                         /* On x86 (signed char):   value is -1  */

/* When passed to a function expecting int: */
int val = c;
/* ARM: val = 255 */
/* x86: val = -1  */
/* DIFFERENT BEHAVIOR ON DIFFERENT MACHINES */

/* The classic bug: */
char ch;
while ((ch = getchar()) != EOF) {  /* WRONG! */
    /* EOF is typically -1.
       If char is unsigned: ch can never be -1,
       so this loop runs forever on unsigned char platforms.
       getchar() returns int specifically to hold EOF.    */
}

/* CORRECT: */
int ch;
while ((ch = getchar()) != EOF) {  /* Returns int, can hold EOF */
    process((char)ch);              /* Cast back only after EOF check */
}
```

### Exact Usage Rules

```c
/* USE char for: */
char name[64];              /* String storage */
char *str = "hello";        /* String pointer (but prefer const char*) */
const char *msg = "error";  /* String literal pointer */

/* USE unsigned char for: */
unsigned char pixel;        /* Image/binary data, 0-255 range */
unsigned char *bytes;       /* Raw byte manipulation */
unsigned char checksum;     /* Bit manipulation where sign causes bugs */

/* USE signed char for: */
signed char delta;          /* Small signed values, -128 to 127 */

/* USE int for: */
int ch = getchar();         /* Reading characters from I/O — always int */
int c = fgetc(file);        /* Same rule */
```

### Linux Kernel Usage

```c
/*
 * The Linux kernel defines __kernel_size_t, u8, s8, etc.
 * For single bytes the kernel almost always uses u8 or __u8
 * (which is guaranteed unsigned char underneath).
 *
 * From include/linux/types.h:
 *   typedef unsigned char  __u8;
 *   typedef signed char    __s8;
 *
 * The kernel uses char primarily for strings (null-terminated).
 * For binary data: u8 (unsigned char).
 * This eliminates the signed/unsigned ambiguity entirely.
 */

/* Kernel-style: explicit about signedness */
u8 byte_value;       /* Always 0-255, no sign confusion */
s8 signed_byte;      /* Always -128 to 127 */
char *name;          /* Only for actual text strings */

/* __user annotation: tells sparse checker this is user-space pointer */
int copy_to_user_example(char __user *ubuf, const char *kbuf, size_t n)
{
    return copy_to_user(ubuf, kbuf, n);
}
```

---

## `float` {#float}

### What It Actually Means

`float` is a **single-precision** IEEE 754 floating-point number. It is exactly **32 bits** on all modern systems.

```
IEEE 754 Single Precision (float) — 32 bits:
┌─────────┬──────────────┬──────────────────────────────────────┐
│  Sign   │   Exponent   │              Mantissa                │
│  1 bit  │    8 bits    │              23 bits                 │
└─────────┴──────────────┴──────────────────────────────────────┘
 Bit 31   Bits 30-23                  Bits 22-0

Value = (-1)^sign × 2^(exponent-127) × (1 + mantissa/2^23)

Precision: ~7 decimal digits
Range:     ~1.18×10^-38 to ~3.4×10^38
Special:   0, -0, +Inf, -Inf, NaN

Example: float f = 0.1f;
         /* 0.1 CANNOT be represented exactly in binary! */
         /* Stored as: 0.100000001490116119384765625  */
```

### The Deadly Precision Trap

```c
#include <stdio.h>
#include <math.h>

int main(void) {
    float a = 0.1f;
    float b = 0.2f;
    float c = a + b;
    
    /* WRONG: floating-point comparison with == */
    if (c == 0.3f) {
        printf("equal\n");     /* This may NOT print! */
    }
    
    /* WHY:
       0.1f ≈ 0.100000001490116119...
       0.2f ≈ 0.200000002980232238...
       sum  ≈ 0.300000011920928955...
       0.3f ≈ 0.300000011920928955...  (may match by luck)
       or
       0.3f ≈ 0.300000004470348358...  (may NOT match)
    */
    
    /* CORRECT: Use epsilon comparison */
    float epsilon = 1e-6f;
    if (fabsf(c - 0.3f) < epsilon) {
        printf("approximately equal\n");  /* Correct */
    }
    
    return 0;
}
```

### When to Use float vs double

```c
/*
 * USE float ONLY WHEN:
 *   1. You have GPU/SIMD code that processes huge arrays (4 floats per register vs 2 doubles)
 *   2. Memory is severely constrained (embedded systems)
 *   3. You are interoperating with OpenGL/graphics APIs
 *   4. You are storing millions of values and the 4-byte size matters
 *
 * USE double BY DEFAULT in C.
 * The C standard says floating-point constants ARE double by default:
 */

float f = 3.14;   /* 3.14 is double! It gets converted to float — precision LOST */
float g = 3.14f;  /* 3.14f is a float literal — correct */

double d = 3.14;  /* Correct — 3.14 IS a double literal */

/* Math functions: */
float result1 = sqrt(2.0f);   /* WRONG: sqrt takes double, returns double */
float result2 = sqrtf(2.0f);  /* CORRECT: sqrtf is the float version */
/* The 'f' suffix on math functions: sqrtf, sinf, cosf, fabsf, etc. */
```

---

## `double` {#double}

### What It Actually Means

`double` means **double-precision** floating-point. It is exactly **64 bits** on all modern systems. `double` is the **default floating-point type in C**.

```
IEEE 754 Double Precision (double) — 64 bits:
┌─────────┬──────────────────┬──────────────────────────────────┐
│  Sign   │    Exponent      │           Mantissa               │
│  1 bit  │    11 bits       │           52 bits                │
└─────────┴──────────────────┴──────────────────────────────────┘
 Bit 63   Bits 62-52                  Bits 51-0

Precision: ~15-16 decimal digits (vs ~7 for float)
Range:     ~2.23×10^-308 to ~1.80×10^308
```

`long double` is platform-specific:
- x86 Linux: 80-bit extended precision (10 bytes, but padded to 12 or 16)
- MSVC Windows: just 64-bit (same as double — Microsoft chose this)
- ARM64: 128-bit quad precision (some implementations)

---

## `short` {#short}

### What It Actually Means

`short` (full form: `short int`) is an integer type guaranteed to be **at least 16 bits**. On virtually all modern platforms it is exactly **16 bits**.

```c
short s;              /* At least 16 bits */
short int s2;         /* Same thing */
signed short s3;      /* Signed (default) */
unsigned short us;    /* Unsigned: 0 to 65535 */

/*
 * Range:
 *   signed short:   -32768 to 32767
 *   unsigned short: 0 to 65535
 */
```

**The Integer Promotion Trap with short**:

```c
short a = 30000;
short b = 30000;
short c = a + b;   /* WRONG! a and b are promoted to int before addition.
                              Result 60000 fits in int.
                              But assignment back to short OVERFLOWS.
                              60000 > 32767 — undefined behavior for signed. */

/* Why promotion happens:
   The CPU does not have native 16-bit arithmetic instructions on x86.
   The CPU works with 32-bit registers. The compiler promotes to int,
   does the math, then truncates back. The truncation is the danger. */

/* CORRECT: Check for overflow or use int */
int a_wide = 30000;
int b_wide = 30000;
int result = a_wide + b_wide;  /* 60000, fits in int */
```

**When to use short**:

```c
/* 1. Network protocol fields (exactly 16-bit by protocol spec) */
struct tcp_header {
    uint16_t source_port;   /* Better: use fixed-width types */
    uint16_t dest_port;
    /* ... */
};

/* 2. Hardware register access */
volatile uint16_t *uart_reg = (volatile uint16_t *)0x40001000;

/* 3. Compact arrays where memory matters */
short heights[1000000];  /* 2MB instead of 4MB with int */
```

---

## `long` {#long}

### What It Actually Means

`long` is an integer type guaranteed to be **at least 32 bits**. The actual size varies significantly between platforms — this is where many cross-platform bugs hide.

```
THE LP64 / LLP64 / ILP64 DISASTER:

ABI Name   Platform              int    long   long long  pointer
──────────────────────────────────────────────────────────────────
ILP32      32-bit (all)          32     32     64         32
LP64       64-bit Linux/macOS    32     64     64         64
LLP64      64-bit Windows        32     32     64         64
ILP64      SPARC (rare)          64     64     64         64
──────────────────────────────────────────────────────────────────

KEY FACT:
  On 64-bit Linux:   sizeof(long) = 8  (LP64)
  On 64-bit Windows: sizeof(long) = 4  (LLP64)

  Code that assumes long is 64-bit will SILENTLY BREAK on Windows.
```

```c
/* The cross-platform mistake */
long big_value = 0x100000000L;  /* On Windows: truncated! long is 4 bytes */

/* CORRECT: Use stdint.h types for guaranteed widths */
#include <stdint.h>
int64_t big_value = 0x100000000LL;   /* Always 64-bit */
int32_t medium   = 0x7FFFFFFF;        /* Always 32-bit */

/* For sizes and pointer arithmetic: */
size_t   sz;     /* Unsigned, size of objects in memory */
ptrdiff_t diff;  /* Signed difference between two pointers */
intptr_t iptr;   /* Integer that can hold a pointer */
```

### Linux Kernel and long

```c
/*
 * Linux kernel heavily uses 'long' but with awareness of the LP64 ABI.
 * Kernel syscall numbers, return values, and flags are often 'long'
 * because the kernel ABI was designed around the LP64 model.
 *
 * Example: sys_read() prototype in kernel:
 */
asmlinkage long sys_read(unsigned int fd, char __user *buf, size_t count);
/*           ────
             'long' here: on 64-bit Linux this is 64-bit.
             Return can hold very large ssize_t values or negative errors.
*/
```

---

## `signed` / `unsigned` {#signed-unsigned}

### What They Actually Mean

These are **type specifiers** that modify integer types to indicate whether they can represent negative values.

```
BINARY REPRESENTATION COMPARISON:

8-bit example:
Binary     unsigned char    signed char
──────────────────────────────────────
0000 0000       0               0
0000 0001       1               1
...
0111 1111     127             127
1000 0000     128            -128   ← Two's complement wraps
1000 0001     129            -127
...
1111 1110     254              -2
1111 1111     255              -1

TWO'S COMPLEMENT (how signed integers work):
To negate a number:
  1. Flip all bits (bitwise NOT)
  2. Add 1

Example: -128 in 8-bit:
  128 in binary: 1000 0000
  But wait — 128 doesn't fit in 7 bits of magnitude.
  -128 is a special case: 1000 0000 (minimum value)
  -128 has no positive counterpart in 8-bit signed!
  (INT_MIN has no positive counterpart — abs(INT_MIN) is UB)
```

**The most dangerous unsigned mistake**:

```c
/* CLASSIC BUG: unsigned wrap-around in loop condition */
unsigned int n = 10;
unsigned int i;

for (i = n - 1; i >= 0; i--) {  /* INFINITE LOOP! */
    /* When i reaches 0 and we subtract 1:
       0 - 1 in unsigned arithmetic = UINT_MAX (wraps to max value)
       UINT_MAX >= 0 is ALWAYS true (unsigned >= 0 is always true)
       Loop never terminates! */
    process(i);
}

/* CORRECT approach 1: count up instead */
for (i = 0; i < n; i++) { process(n - 1 - i); }

/* CORRECT approach 2: use signed */
int si;
for (si = (int)n - 1; si >= 0; si--) { process(si); }

/* CORRECT approach 3: careful unsigned */
for (i = n; i-- > 0; ) {  /* Post-decrement: check then decrement */
    process(i);             /* i goes: 9, 8, 7, ..., 0 */
}
```

**Signed/unsigned comparison — silent logic error**:

```c
#include <stdio.h>

int main(void) {
    int    a = -1;
    unsigned int b = 1;
    
    /* When C compares signed and unsigned, it converts signed to unsigned */
    /* -1 as unsigned int = UINT_MAX = 4294967295 */
    /* 4294967295 > 1 is TRUE */
    
    if (a < b) {
        printf("a < b\n");   /* You'd expect this */
    } else {
        printf("a >= b\n");  /* But THIS prints because -1 becomes huge unsigned */
    }
    
    return 0;
}
/* Compiler warning: -Wsign-compare catches this. ALWAYS compile with this flag. */
```

---

## `void` {#void}

### What It Actually Means

`void` means **nothing** or **absence of type**. It is used in three distinct and unrelated contexts. Most beginners only understand one.

```
THREE USES OF void:

┌──────────────────────────────────────────────────────────┐
│ USE 1: Return type — function returns nothing            │
│   void print_msg(const char *s) { puts(s); }            │
│                                                          │
│ USE 2: Parameter list — function takes no arguments      │
│   int get_random(void);  /* C style: no args */          │
│   int get_random();      /* K&R style: unspecified! */   │
│                          /* These are DIFFERENT in C89 */ │
│                                                          │
│ USE 3: void* — pointer to unspecified type               │
│   void *ptr = malloc(100);  /* Points to "something" */  │
│   /* void* can be assigned to/from any pointer type      │
│      WITHOUT a cast in C (but NOT in C++) */             │
└──────────────────────────────────────────────────────────┘
```

**The `void` vs. empty parameter list trap**:

```c
/* In C (not C++): */

/* This declares a function taking UNKNOWN arguments (K&R style): */
int foo();

/* This declares a function taking NO arguments: */
int bar(void);

/* The difference: */
foo(1, 2, 3);   /* Legal — compiler accepts unknown args */
bar(1, 2, 3);   /* Error — prototype says no args */

/* In C99 and later, empty () is deprecated as "unspecified parameters."
   ALWAYS use (void) for zero-argument functions in C. */

/* In C++: int foo() and int foo(void) are identical — both mean no args.
   C and C++ differ here! */
```

**`void *` — the generic pointer**:

```c
#include <stdlib.h>
#include <string.h>

/*
 * void* is the foundation of generic programming in C.
 * malloc() returns void* — a pointer to allocated memory
 * of unspecified type. You tell the compiler what it is
 * by assigning it to a typed pointer.
 */

int *arr = malloc(10 * sizeof(int));   /* No cast needed in C */
int *arr2 = (int *)malloc(10 * sizeof(int));  /* Cast: C++ style, OK in C too */

/* 
 * WHY no cast in C:
 *   void* implicitly converts to any pointer type.
 *   Casting malloc() return in C can HIDE the bug of
 *   forgetting to #include <stdlib.h>.
 *   In C89 without the include, malloc returns int (implicit function rule),
 *   the cast silences the type mismatch warning, and you get
 *   a 32-bit int masquerading as a 64-bit pointer on 64-bit systems.
 *   CRASH. Always include <stdlib.h>.
 */

/* void* arithmetic: ILLEGAL in standard C */
void *p = malloc(100);
p++;   /* ERROR: cannot increment void* — sizeof(void) is undefined */
/* Some compilers (GCC) allow this as an extension where sizeof(void)=1 */

/* CORRECT: cast to char* for byte arithmetic */
char *bytes = (char *)p;
bytes++;   /* Legal: char is 1 byte */
```

**Linux Kernel void usage**:

```c
/*
 * Linux kernel uses void* extensively for:
 *   1. Generic data pointers in data structures
 *   2. The 'private' field in many structures
 *   3. RCU (Read-Copy-Update) protected pointers
 */

/* From linux/include/linux/netdevice.h (simplified): */
struct net_device {
    /* ... */
    void *priv;     /* Driver private data */
    /* ... */
};

/* Kernel memory allocation returns void*: */
void *kmalloc(size_t size, gfp_t flags);
void kfree(const void *objp);

/* Usage: */
struct my_data *data = kmalloc(sizeof(*data), GFP_KERNEL);
if (!data)
    return -ENOMEM;
```

---

# CATEGORY 2 — Storage Class Keywords {#category-2-storage-class-keywords}

Storage class keywords tell the compiler **where** a variable lives in memory and **how long** it lives.

```
MEMORY LAYOUT OF A RUNNING PROCESS:

High Address
┌─────────────────────────────────────────┐
│              STACK                      │  ← auto variables live here
│         (grows downward ↓)             │     Function-local variables
│                                         │     Freed when function returns
├─────────────────────────────────────────┤
│                                         │
│         (unmapped / guard page)         │
│                                         │
├─────────────────────────────────────────┤
│              HEAP                       │  ← malloc/calloc/realloc
│         (grows upward ↑)               │     Lives until free()
│                                         │
├─────────────────────────────────────────┤
│              BSS SEGMENT                │  ← Zero-initialized globals/statics
│         (uninitialized data)            │     static int x; (starts at 0)
├─────────────────────────────────────────┤
│              DATA SEGMENT               │  ← Initialized globals/statics
│         (initialized data)             │     static int y = 5;
├─────────────────────────────────────────┤
│              TEXT SEGMENT               │  ← Program code (read-only)
│         (code / instructions)          │     inline functions may be here
├─────────────────────────────────────────┤
│         (OS kernel space)               │
└─────────────────────────────────────────┘
Low Address

LIFETIME MAP:
  auto variables:    Exist from declaration to end of their block { }
  static variables:  Exist for entire program lifetime
  extern variables:  Exist in another translation unit
  register variables: Same as auto but compiler may put in CPU register
```

---

## `auto` {#auto}

### What It Actually Means

`auto` means **automatic storage duration** — the variable is automatically created when its scope is entered and destroyed when it exits. This is the **default** for all local variables.

> **In C89/C99/C11: `auto` is almost never written explicitly because it's the default.**

```c
/* These two declarations are 100% identical in C: */
auto int x = 5;   /* Explicit auto — valid but never used */
int x = 5;        /* Implicit auto — always written this way */

/* 'auto' serves no purpose in C code because local variables
   are auto by default. You will NEVER see 'auto' in real C code. */
```

**CRITICAL MISINFORMATION**: Some beginners (coming from C++) think `auto` in C means type deduction like C++11's `auto`. **IT DOES NOT.**

```c
/* C++ (C++11 and later): */
auto x = 5;      /* x deduced to be int */
auto y = 3.14;   /* y deduced to be double */
auto z = "hi";   /* z deduced to be const char* */

/* C (all standards): */
auto x = 5;      /* MEANS: auto int x = 5; Same as int x = 5; */
                 /* NOT type deduction! */
                 /* This is valid C but pointless. */
```

**Where auto matters conceptually**:

```c
void function(void) {
    int a = 10;        /* auto — on the stack, created at this line */
    {
        int b = 20;    /* auto — inner scope, separate from outer */
        /* a is visible here */
        /* b is visible here */
    }
    /* b is DESTROYED here — stack pointer moves back */
    /* b is no longer accessible */
    /* a is still alive */
}
/* a is DESTROYED here — function returns */
```

---

## `register` {#register}

### What It Actually Means

`register` is a **hint** to the compiler that this variable should be stored in a CPU register rather than memory, for faster access.

```
WITHOUT register:
  Variable in memory (stack)
  
  CPU Register ──── Load ────► Stack Memory  (read: ~1-3 cycles)
  CPU Register ◄─── Store ─── Stack Memory  (write: ~1-3 cycles)
  
WITH register hint (if compiler honors it):
  Variable in CPU register directly
  
  Arithmetic operation uses register directly (~1 cycle)
  No load/store overhead

CPU REGISTERS (x86-64):
  General purpose: rax, rbx, rcx, rdx, rsi, rdi, rsp, rbp
                   r8, r9, r10, r11, r12, r13, r14, r15
  Total: 16 registers (but rsp=stack ptr, rbp=frame ptr are reserved)
  Practical: ~14 registers available for variables
```

### The Reality of register in Modern C

```c
/* C89 usage (historical): */
register int i;
for (register int j = 0; j < 1000; j++) {
    /* hint: put i and j in registers */
}

/*
 * MODERN REALITY:
 * 1. Modern compilers (GCC, Clang) IGNORE the register keyword.
 *    Their optimizers are smarter than explicit hints.
 *    With -O2, the compiler automatically puts hot variables in registers.
 *
 * 2. In C99/C11, register has one remaining effect:
 *    You CANNOT take the address of a register variable.
 */

register int x = 5;
int *p = &x;   /* ERROR — cannot take address of register variable */
               /* (Even if compiler ignores the hint, the restriction stands) */

/*
 * WHY the address restriction exists:
 *   CPU registers don't have memory addresses.
 *   If a variable is truly in a register, it has no address.
 *   The restriction enforces this semantic even if the
 *   compiler puts it in memory.
 */

/* In C11: 'register' is retained but deprecated in practice.
   In C23: 'register' is not removed but considered obsolescent. */
```

**When register actually mattered (historical)**:

```c
/* Old C89 critical loop — registers made real difference before optimizers */
void old_style_loop(int *arr, int n) {
    register int i;        /* Force i into register */
    register int *p = arr; /* Force pointer into register */
    register int sum = 0;  /* Force accumulator into register */
    
    for (i = 0; i < n; i++) {
        sum += p[i];
    }
}

/* Modern equivalent — let the optimizer decide: */
void modern_loop(const int *restrict arr, int n) {
    int sum = 0;
    for (int i = 0; i < n; i++) {
        sum += arr[i];
    }
}
/* The modern version with 'restrict' and -O2 will auto-vectorize
   using SIMD (SSE/AVX) — far faster than any register hint. */
```

---

## `static` {#static}

### What It Actually Means

`static` is the most **overloaded** keyword in C. It has **two completely different meanings** depending on where it is used:

```
┌────────────────────────────────────────────────────────────────┐
│ MEANING 1: Applied to LOCAL variables inside a function        │
│   → Changes LIFETIME: variable persists across function calls  │
│   → Stored in DATA/BSS segment, not on stack                   │
│   → Initialized ONCE (at program start or first use)          │
│                                                                 │
│ MEANING 2: Applied to GLOBAL variables or functions            │
│   → Changes LINKAGE: variable/function is file-private         │
│   → Not visible to other translation units (.c files)          │
│   → Prevents naming collisions between files                   │
└────────────────────────────────────────────────────────────────┘
```

### Meaning 1 — Persistent Local Variable

```c
#include <stdio.h>

int counter(void) {
    static int count = 0;  /* Initialized ONCE. Lives in BSS/data segment. */
    count++;               /* Retains value between calls */
    return count;
}

int main(void) {
    printf("%d\n", counter());  /* 1 */
    printf("%d\n", counter());  /* 2 */
    printf("%d\n", counter());  /* 3 */
    return 0;
}

/*
 * MEMORY DIAGRAM:
 *
 * Stack (during counter() call):
 *   ┌──────────────┐
 *   │ return addr  │
 *   └──────────────┘
 *   (count is NOT on stack — it's in BSS/data)
 *
 * BSS/Data segment (entire program lifetime):
 *   ┌──────────────┐
 *   │   count = 3  │  ← persists between calls
 *   └──────────────┘
 *
 * THREAD SAFETY WARNING:
 *   Static locals are NOT thread-safe!
 *   Multiple threads calling counter() simultaneously will have
 *   a race condition on 'count'.
 *   Solution: use atomic operations or mutex protection.
 */
```

**Static local — real-world use cases**:

```c
/* 1. Singleton pattern — initialize once */
struct Config *get_config(void) {
    static struct Config *instance = NULL;
    if (instance == NULL) {
        instance = malloc(sizeof(struct Config));
        load_config(instance);
    }
    return instance;
    /* WARNING: Not thread-safe without mutex! */
}

/* 2. Lazy initialization of lookup tables */
const int *get_fibonacci_table(void) {
    static int fib[50] = {0};
    static int initialized = 0;
    
    if (!initialized) {
        fib[0] = 0; fib[1] = 1;
        for (int i = 2; i < 50; i++) {
            fib[i] = fib[i-1] + fib[i-2];
        }
        initialized = 1;
    }
    return fib;
}

/* 3. Function generating unique IDs */
unsigned int next_id(void) {
    static unsigned int id = 0;
    return ++id;
}
```

### Meaning 2 — File-Private Linkage

```c
/*
 * FILE: module_a.c
 */

/* This function is PRIVATE to module_a.c — not visible to other .c files */
static int helper_function(int x) {
    return x * 2;
}

/* This global is PRIVATE to module_a.c */
static int module_state = 0;

/* This function IS visible to other .c files (no static) */
int public_function(int x) {
    module_state++;
    return helper_function(x);
}
```

```c
/*
 * FILE: module_b.c
 */

/* These will cause LINKER ERRORS because helper_function is static in module_a.c */
/* extern int helper_function(int);  ← linker: undefined symbol */
/* extern int module_state;          ← linker: undefined symbol */

/* Only public_function is accessible */
extern int public_function(int x);
```

```
LINKING DIAGRAM:

module_a.o              module_b.o
┌─────────────────┐    ┌─────────────────┐
│ static helper() │    │                 │
│ static state    │    │  uses:          │
│ PUBLIC public() │◄───│  public_function │
└─────────────────┘    └─────────────────┘

Linker combines:
  ✓ public_function: visible, linked
  ✗ helper_function: hidden, not exported
  ✗ module_state:    hidden, not exported
```

**Linux Kernel static usage**:

```c
/*
 * The Linux kernel uses 'static' extensively for:
 *   1. File-scoped functions (most kernel functions are static)
 *   2. Module-private data
 *   3. Per-CPU variables with DEFINE_PER_CPU
 *
 * Example from kernel network subsystem (conceptual):
 */

/* Private to this .c file: */
static int tcp_parse_options(struct sk_buff *skb) { /* ... */ return 0; }

/* Private state: */
static DEFINE_SPINLOCK(tcp_secret_lock);
static u32 tcp_secret_stamp;

/* Static inline — both static (file-private) AND inline (inlined): */
static inline u32 tcp_time_stamp(void) {
    return jiffies;  /* kernel timer ticks */
}

/*
 * KERNEL RULE: If a function is only used in one .c file,
 * it MUST be static. This:
 *   1. Prevents namespace pollution
 *   2. Allows compiler to optimize more aggressively
 *   3. Enables link-time dead code elimination
 *   4. Is enforced by kernel coding style
 */
```

### static in Array Parameters (C99 — Almost Nobody Knows This)

```c
/*
 * HIDDEN FEATURE: 'static' inside array parameter brackets
 * tells the compiler the array has AT LEAST that many elements.
 * This is a GUARANTEE to the optimizer — if you lie, UB.
 */

/* Tell compiler: arr has at LEAST 4 elements */
void process(int arr[static 4]) {
    /* Compiler knows arr is not NULL */
    /* Compiler knows it can safely access arr[0], arr[1], arr[2], arr[3] */
    int sum = arr[0] + arr[1] + arr[2] + arr[3];  /* No bounds check needed */
}

/* Usage: */
int data[10];
process(data);    /* OK: 10 >= 4 */
process(NULL);    /* UNDEFINED BEHAVIOR — violates the static 4 guarantee */

/* This enables better auto-vectorization and null-pointer elimination */
```

---

## `extern` {#extern}

### What It Actually Means

`extern` means **external linkage** — this variable or function is defined **somewhere else** (another translation unit / .c file). It is a **declaration**, not a definition.

```
DECLARATION vs DEFINITION — The Fundamental Distinction:

DECLARATION: "This thing EXISTS somewhere. Here is its type."
             No memory is allocated.
             
DEFINITION:  "THIS is where the thing lives."
             Memory IS allocated (for variables).

extern int x;       ← DECLARATION  (just a promise: x exists somewhere)
int x;              ← DEFINITION   (allocates memory for x)
int x = 5;          ← DEFINITION   (allocates and initializes)

FUNCTION RULES:
int foo(void);      ← DECLARATION (prototype) — no memory, just type info
int foo(void) { }   ← DEFINITION  — actual code, allocated in text segment

A variable can be DECLARED many times but DEFINED exactly ONCE (ODR).
```

**The One Definition Rule (ODR)**:

```c
/*
 * FILE: config.h
 *
 * WRONG pattern — causes "multiple definition" linker error when included
 * in multiple .c files:
 */
int global_count = 0;  /* DEFINITION in header — included by every .c file = multiple definitions */

/*
 * CORRECT pattern:
 */

/* config.h */
extern int global_count;  /* DECLARATION — just a promise */

/* config.c */
int global_count = 0;     /* DEFINITION — exactly once */
```

```c
/*
 * EXTERN LINKAGE DIAGRAM:
 *
 * main.c          math_utils.c        string_utils.c
 * ──────          ────────────        ──────────────
 * extern int      int shared = 0;     extern int
 *   shared;  ────►  (definition)  ◄───  shared;
 *                    (in BSS/data)
 *
 * All three files share ONE instance of 'shared'.
 * main.c and string_utils.c DECLARE it.
 * math_utils.c DEFINES it.
 * Linker resolves the references.
 */
```

**extern "C" — The Bridge Between C and C++**:

```c
/*
 * C++ uses "name mangling" — it encodes the full function signature
 * into the symbol name to support overloading.
 *
 * C function: void foo(int)     → symbol: _foo   (simple)
 * C++ function: void foo(int)   → symbol: _Z3fooi (mangled)
 *
 * When C++ code wants to call C functions (or vice versa):
 */

/* In a header file (mylib.h): */
#ifdef __cplusplus
extern "C" {
#endif

/* These functions have C linkage — no name mangling */
void c_function(int x);
int  another_c_function(const char *str);

#ifdef __cplusplus
}
#endif

/*
 * Without extern "C", a C++ file trying to call c_function would
 * look for the mangled symbol _Z10c_functioni, not find it,
 * and get a linker error.
 */
```

**Linux Kernel extern usage**:

```c
/*
 * The kernel declares many global variables with extern in headers.
 * Key examples:
 */

/* From include/linux/sched.h (simplified): */
extern struct task_struct init_task;   /* The initial process */
extern struct mm_struct   init_mm;     /* The initial memory map */

/* From include/linux/jiffies.h: */
extern unsigned long volatile jiffies; /* System timer ticks */
/*                   ^^^^^^^^
                     Note: also volatile! It changes asynchronously (from timer IRQ).
                     This is a perfect example of combining extern + volatile. */

/* From include/linux/kernel.h: */
extern int printk(const char *fmt, ...);  /* Kernel's printf */
```

---

# CATEGORY 3 — Type Qualifier Keywords {#category-3-type-qualifier-keywords}

Type qualifiers modify **how a variable can be accessed**, not what type it is.

---

## `const` {#const}

### What It Actually Means

`const` means the variable's value should **not be modified** after initialization. It is a **promise to the compiler**, not a runtime enforcement.

**`const` is about the variable, not the value. Objects themselves are not constant — variables can be declared constant.**

```
THE CONST CLOCKWISE-SPIRAL RULE (C Declaration Reading):

Read declarations from the variable name, spiraling clockwise/outward.

int *p;
      ↑
      p is a pointer to int

const int *p;
            ↑
            p is a pointer to const int
            (pointer is mutable, pointed-to int is const)

int * const p;
      ↑
      p is a const pointer to int
      (pointer is const, pointed-to int is mutable)

const int * const p;
            ↑
            p is a const pointer to const int
            (both pointer and int are const)
```

**The Pointer and const — The Most Confused Topic in C**:

```c
/* 
 * Four combinations — memorize these:
 */

int value = 42;
int other = 99;

/* 1. Pointer to non-const int — can change both pointer and value */
int *p1 = &value;
*p1 = 100;   /* OK: change the int */
p1 = &other; /* OK: change where pointer points */

/* 2. Pointer to const int — can only change the pointer, not the value */
const int *p2 = &value;
*p2 = 100;   /* ERROR: cannot modify const int through this pointer */
p2 = &other; /* OK: can change where pointer points */

/* 3. Const pointer to non-const int — can only change the value, not the pointer */
int * const p3 = &value;
*p3 = 100;   /* OK: can modify the int */
p3 = &other; /* ERROR: cannot change a const pointer */

/* 4. Const pointer to const int — cannot change anything */
const int * const p4 = &value;
*p4 = 100;   /* ERROR */
p4 = &other; /* ERROR */

/*
 * MEMORY DIAGRAM for p1, p2, p3, p4 (all pointing to 'value'):
 *
 * Stack:
 * ┌──────────────────┐
 * │ p1: 0x7fff1000   │──► [value: 42]   p1: mutable ptr, mutable value
 * │ p2: 0x7fff1000   │──► [value: 42]   p2: mutable ptr, const value
 * │ p3: 0x7fff1000   │──► [value: 42]   p3: const ptr, mutable value
 * │ p4: 0x7fff1000   │──► [value: 42]   p4: const ptr, const value
 * └──────────────────┘
 *
 * const is a compiler-enforced CONTRACT, not a hardware lock.
 * The memory itself is the same in all four cases.
 */
```

**`const` does NOT mean "compile-time constant"** (a crucial misunderstanding):

```c
/* C does NOT allow variable-length arrays initialized with const: */
const int N = 10;
int arr[N];   /* In C99: this is a VLA (Variable Length Array), not a compile-time array! */
              /* In C89: this is ILLEGAL */
              /* In C++: this IS a compile-time constant — C and C++ differ! */

/* For true compile-time constants in C: use #define or enum */
#define MAX_SIZE 10
int arr2[MAX_SIZE];  /* TRUE compile-time constant — always valid */

enum { BUFFER_SIZE = 256 };
char buffer[BUFFER_SIZE];  /* Also true compile-time constant */
```

**Casting away const — defined but dangerous**:

```c
const int x = 42;
int *p = (int *)&x;   /* Cast away const — legal but dangerous */
*p = 100;             /* UNDEFINED BEHAVIOR if x was truly const */
                      /* On x86: often "works" (modifies stack) */
                      /* If x is in read-only memory (.rodata): SIGSEGV */

/* String literals — the classic const violation: */
char *s = "hello";         /* WRONG in C++, warning in C — string is in .rodata */
const char *s2 = "hello";  /* CORRECT — read-only memory correctly typed */
*s = 'H';   /* UNDEFINED BEHAVIOR — likely SIGSEGV, .rodata is read-only */
```

**Linux Kernel const usage**:

```c
/*
 * The kernel uses const heavily for:
 *   1. Read-only data structures (dispatch tables, file operations)
 *   2. Immutable string literals
 *   3. Preventing accidental modification of critical structures
 */

/* File operations table — read-only: */
static const struct file_operations my_fops = {
    .owner   = THIS_MODULE,
    .read    = my_read,
    .write   = my_write,
    .open    = my_open,
    .release = my_release,
};

/* Function parameter: promise not to modify the buffer: */
ssize_t my_write(struct file *f, const char __user *buf, size_t len, loff_t *off) {
    /* buf is const: we promise only to read from user-space, not write */
    char kbuf[256];
    if (copy_from_user(kbuf, buf, min(len, sizeof(kbuf))))
        return -EFAULT;
    return len;
}

/* Kernel constant data in .rodata section: */
static const char * const error_strings[] = {
    "success",
    "invalid argument",
    "out of memory",
    "permission denied",
};
```

---

## `volatile` {#volatile}

### What It Actually Means

`volatile` tells the compiler: **"This variable can change at any time, without any action by the code you can see. Do not optimize accesses to it."**

Without `volatile`, the compiler assumes a variable only changes when the visible code changes it. This assumption enables powerful optimizations. `volatile` breaks this assumption.

```
WHY VOLATILE EXISTS — THE OPTIMIZATION PROBLEM:

Without volatile:
  int status = 0;
  while (status == 0) { }   /* Spin-wait */
  
  Compiler sees: "status is always 0 in this loop. Optimize to infinite loop."
  
  Assembly (WRONG without volatile):
    LOAD status → register
    COMPARE register, 0
    JUMP-IF-EQUAL (infinite loop — never re-reads status!)
  
  ────────────────────────────────────────────────────────

With volatile:
  volatile int status = 0;
  while (status == 0) { }   /* Spin-wait */
  
  Compiler: "Must re-read status from memory every iteration."
  
  Assembly (CORRECT with volatile):
    LOAD status → register  ← re-executed every iteration
    COMPARE register, 0
    JUMP-IF-EQUAL (loops back to LOAD)
```

**When to Use volatile**:

```c
/*
 * USE volatile WHEN a variable can change due to:
 *   1. Hardware registers (memory-mapped I/O)
 *   2. Interrupt service routines (ISR modifies a variable)
 *   3. Signal handlers
 *   4. Shared memory between processes (not threads — different rules)
 *   5. setjmp/longjmp scenarios
 */

/* ─── USE CASE 1: Memory-Mapped I/O (Embedded Systems) ─── */
#define UART_STATUS_REG  ((volatile uint32_t *)0x40001000)
#define UART_DATA_REG    ((volatile uint32_t *)0x40001004)

void uart_send(char c) {
    /* Wait until UART is ready — hardware sets bit 0 when ready */
    while ((*UART_STATUS_REG & 0x01) == 0) {
        /* Without volatile: compiler may optimize this away!
           With volatile:    reads the register every iteration. */
    }
    *UART_DATA_REG = (uint32_t)c;
}

/* ─── USE CASE 2: Interrupt Handler Communication ─── */
#include <signal.h>
#include <stdint.h>

/* Flag set by interrupt/signal handler: */
volatile sig_atomic_t g_interrupted = 0;  /* sig_atomic_t is volatile-safe integer */

void signal_handler(int sig) {
    g_interrupted = 1;  /* Set by interrupt/signal */
}

void main_loop(void) {
    while (!g_interrupted) {
        /* Without volatile: compiler may keep g_interrupted in register,
           never re-reading from memory, never seeing the handler's write.
           Loop runs forever. */
        do_work();
    }
}

/* ─── USE CASE 3: setjmp/longjmp ─── */
#include <setjmp.h>

void setjmp_example(void) {
    jmp_buf env;
    volatile int x = 0;  /* Must be volatile to be restored after longjmp */
    
    if (setjmp(env) == 0) {
        x = 42;
        longjmp(env, 1);  /* Jump back to setjmp */
    }
    /* If x were not volatile, its value after longjmp is undefined */
    /* With volatile, x is guaranteed to be 42 here */
}
```

**What volatile does NOT do** (the deadly misinformation):

```c
/*
 * MYTH: "volatile makes variables thread-safe"
 * REALITY: volatile provides NO synchronization, NO atomicity,
 *          NO memory ordering guarantees for threads.
 *
 * volatile prevents the COMPILER from caching the variable in
 * a register. It does NOT prevent the CPU from:
 *   - Reordering loads and stores
 *   - Using stale cache lines
 *   - Partial reads/writes on some architectures
 *
 * For thread safety you need:
 *   - Mutexes (pthread_mutex_t)
 *   - Atomic operations (_Atomic in C11 or GCC __atomic builtins)
 *   - Memory barriers (__sync_synchronize() or atomic_thread_fence())
 */

/* WRONG: Using volatile for thread synchronization */
volatile int shared_data = 0;

void thread_1(void) { shared_data = 42; }  /* No guarantee thread_2 sees this */
void thread_2(void) { 
    while (shared_data == 0);  /* Might never see the write from thread_1 */
}

/* CORRECT: Use C11 atomics */
#include <stdatomic.h>
_Atomic int shared_data = 0;
void thread_1(void) { atomic_store(&shared_data, 42); }
void thread_2(void) { while (atomic_load(&shared_data) == 0); }
```

**Linux Kernel volatile usage**:

```c
/*
 * The Linux kernel is VERY conservative about volatile usage.
 * The kernel has its own memory barriers and locking primitives.
 * Most volatile use in the kernel is for:
 *   1. jiffies (the global timer counter, set by timer interrupt)
 *   2. Memory-mapped device registers
 *   3. Flags modified in interrupt context
 *
 * From include/linux/jiffies.h:
 */
extern unsigned long volatile jiffies;

/*
 * For hardware registers, the kernel uses helper macros that
 * incorporate volatile internally:
 */
#define readl(addr)       (*(volatile unsigned int *)(addr))
#define writel(val, addr) (*(volatile unsigned int *)(addr) = (val))

/* Usage: */
unsigned int status = readl(device_base + STATUS_OFFSET);
writel(0x01, device_base + CONTROL_OFFSET);  /* Write 1 to control register */

/*
 * KERNEL PHILOSOPHY:
 *   Use locking primitives (spinlock, mutex, rcu) for synchronization.
 *   Use volatile ONLY for hardware and interrupt-context flags.
 *   Never use volatile as a substitute for proper locking.
 */
```

---

## `restrict` (C99) {#restrict}

### What It Actually Means

`restrict` is a **promise** to the compiler that for the lifetime of a pointer, **only that pointer** (or pointers derived from it) will be used to access the object it points to. In other words: no pointer aliasing.

```
THE ALIASING PROBLEM:

void add(int *a, int *b, int *c) {
    *a = *b + *c;  
}

/*
 * The compiler CANNOT assume a, b, c point to different memory.
 * What if called as: add(x, x, x)?
 * What if a == c? Then reading *c after writing *a gives different result.
 *
 * So the compiler must:
 *   1. Load *b into register
 *   2. Load *c into register
 *   3. ADD registers
 *   4. Store to *a
 *   5. (Cannot optimize further — aliasing may exist)
 *
 * DIAGRAM (possible aliasing):
 *   a ──────►[memory cell X]◄────── c  ← a and c point to SAME location!
 *   b ──────►[memory cell Y]
 *
 * With restrict:
 */

void add_restrict(int * restrict a, int * restrict b, int * restrict c) {
    *a = *b + *c;
}

/*
 * Now the compiler KNOWS a, b, c point to different objects.
 * It can:
 *   1. Load *b and *c in any order
 *   2. Keep values in registers
 *   3. Reorder, vectorize, optimize freely
 *   4. Result is guaranteed to be the same (no aliasing)
 *
 * DIAGRAM (with restrict — guaranteed no aliasing):
 *   a ──────►[memory cell X]
 *   b ──────►[memory cell Y]
 *   c ──────►[memory cell Z]
 *   All DISTINCT. Compiler may vectorize the loop.
 */
```

**Real-world restrict usage**:

```c
#include <string.h>  /* memcpy uses restrict */

/*
 * The standard library signature of memcpy:
 *   void *memcpy(void * restrict dest, const void * restrict src, size_t n);
 *
 * The restrict tells the compiler dest and src don't overlap.
 * This is why memcpy is UNDEFINED BEHAVIOR for overlapping regions
 * (use memmove for overlapping).
 *
 * memmove's signature:
 *   void *memmove(void *dest, const void *src, size_t n);
 * No restrict — memmove handles overlap correctly (slower but safe).
 */

/* restrict in your own code: */
void vector_add(float * restrict result,
                const float * restrict a,
                const float * restrict b,
                int n) {
    for (int i = 0; i < n; i++) {
        result[i] = a[i] + b[i];
    }
    /*
     * With restrict + -O2:
     *   Compiler generates SIMD (AVX/SSE) code — processes 4-8 floats per cycle
     *
     * Without restrict:
     *   Compiler generates scalar code — 1 float per cycle
     *   (must assume result may alias a or b)
     */
}

/* BREAKING THE restrict PROMISE — UNDEFINED BEHAVIOR: */
float arr[100];
vector_add(arr, arr, arr, 100);  /* WRONG: result aliases a and b */
```

---

# CATEGORY 4 — Control Flow Keywords {#category-4-control-flow-keywords}

---

## `if` / `else` {#if-else}

### The Hidden Traps

```c
/* ─── TRAP 1: The dangling else problem ─── */

int x = 1, y = 0;
if (x == 1)
    if (y == 1)
        printf("A\n");
else              /* Which 'if' does this else belong to? */
    printf("B\n");

/*
 * In C: else always matches the NEAREST preceding if.
 * This code is equivalent to:
 *
 * if (x == 1) {
 *     if (y == 1)
 *         printf("A\n");
 *     else            ← matches inner if (y == 1)
 *         printf("B\n");
 * }
 *
 * Result: prints "B" (x==1, y==0, inner else taken)
 *
 * NOT:
 * if (x == 1) {
 *     if (y == 1)
 *         printf("A\n");
 * } else {
 *     printf("B\n");  ← You might THINK this is the else
 * }
 *
 * SOLUTION: Always use braces. ALWAYS.
 */

/* CORRECT */
if (x == 1) {
    if (y == 1) {
        printf("A\n");
    }
} else {
    printf("B\n");
}
```

```c
/* ─── TRAP 2: Assignment in condition ─── */

int n;
if (n = get_value()) {  /* Is this assignment or comparison? */
    /* n = get_value() assigns return value to n, then tests it */
}

/* This is LEGAL and sometimes intentional: */
char *p;
while ((p = strstr(haystack, needle)) != NULL) {
    /* Do something with p */
    haystack = p + 1;
}
/* The extra parentheses signal: "I meant assignment, not == comparison" */

/* Accidental assignment (bug): */
if (n = 0) {   /* Should be n == 0 but assigns 0 to n */
    /* Never executes: 0 is false */
}
/* Compiler warning -Wparentheses catches this */

/* Yoda conditions to prevent this mistake: */
if (0 == n) {  /* "Yoda style": if you accidentally write = instead of ==,
                  compiler errors because 0 = n is invalid */
    /* ... */
}
```

---

## `switch` / `case` / `default` {#switch-case-default}

### What They Actually Mean

`switch` evaluates an integer expression and jumps to the matching `case` label. If no case matches, it jumps to `default`.

```
SWITCH FLOW DIAGRAM:

switch (expr) {
    case A: ──► [code A] ──► FALLTHROUGH (no break!) ──►
    case B: ──► [code B] ──► break ──────────────────────► after switch
    case C: ──► [code C] ──► break ──────────────────────► after switch  
    default:──► [code D] ──► break ──────────────────────► after switch
}

                    switch(expr)
                         │
              ┌──────────┼──────────┬──────────┐
              │          │          │          │
           expr==A    expr==B    expr==C    no match
              │          │          │          │
              ▼          ▼          ▼          ▼
           [code A]   [code B]  [code C]   [default]
              │          │          │          │
              │ (fall)   │ (break)  │ (break)  │ (break)
              ▼          │          │          │
           [code B]      │          │          │
              │          │          │          │
              │ (break)  │          │          │
              ▼          ▼          ▼          ▼
         ──────────────────────────────────────────►
                     after switch
```

**The Critical Fallthrough Behavior**:

```c
/* INTENTIONAL FALLTHROUGH (explicit, documented): */
switch (status) {
    case STATUS_CRITICAL:
        alert_operator();
        /* FALLTHROUGH — also do normal error handling */
    case STATUS_ERROR:
        log_error();
        /* FALLTHROUGH — also do cleanup */
    case STATUS_WARNING:
        cleanup();
        break;
    default:
        break;
}

/* In C17 and later, use [[fallthrough]] attribute (from C23 or GCC extension): */
/* [[__fallthrough__]]; or __attribute__((fallthrough)); */

/* ACCIDENTAL FALLTHROUGH (common bug): */
switch (c) {
    case 'a':
        printf("found a\n");
        /* BUG: forgot break! Falls through to case 'b' */
    case 'b':
        printf("found b\n");
        break;
}
/* Input 'a': prints "found a" then "found b" — probably not intended */
```

**Switch — What Can Go In The Expression**:

```c
/* switch ONLY works with integer types: */
switch (some_int)    { }  /* OK */
switch (some_char)   { }  /* OK — char promoted to int */
switch (some_enum)   { }  /* OK — enum is int */
switch (some_float)  { }  /* ERROR: float not allowed */
switch (some_string) { }  /* ERROR: can't switch on strings */
switch (some_struct) { }  /* ERROR: can't switch on structs */

/* Case labels MUST be integer constant expressions: */
int x = 5;
switch (val) {
    case 5:    { }  /* OK: integer literal */
    case x:    { }  /* ERROR: x is not a constant */
    case 2+3:  { }  /* OK: constant expression */
    case 'A':  { }  /* OK: character constant (is an int) */
}
```

**Duff's Device — The Most Famous switch Trick**:

```c
/*
 * Duff's Device — Tom Duff, 1983
 * Combines switch fallthrough with a do-while loop.
 * Historically used to unroll copy loops.
 * Now mostly a curiosity (compilers do this automatically).
 * DEMONSTRATES: switch and loop can be interleaved!
 */

void duffs_device(char *to, const char *from, int count) {
    int n = (count + 7) / 8;
    switch (count % 8) {
        case 0: do { *to++ = *from++;  /* FALLS THROUGH */
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
/*
 * The switch jumps into the middle of the loop on first iteration.
 * Then the loop runs full 8-copy iterations.
 * This unrolls the loop 8x with minimal code.
 *
 * Trivial for modern compilers to do better — but it's a fascinating
 * demonstration that switch labels are just jump targets.
 */
```

---

## `break` {#break}

### What It Actually Means

`break` exits the **immediately enclosing** `switch`, `for`, `while`, or `do-while`. It does **not** exit `if` statements, and it only exits **one** level.

```c
/* COMMON MISCONCEPTION: break exits nested loops */

/* WRONG expectation: */
for (int i = 0; i < 10; i++) {
    for (int j = 0; j < 10; j++) {
        if (i == 5 && j == 5) {
            break;  /* Only exits INNER loop! Not outer! */
        }
    }
    /* Execution continues here after break */
}

/* SOLUTIONS for breaking nested loops: */

/* Solution 1: goto (legitimate use case for goto) */
for (int i = 0; i < 10; i++) {
    for (int j = 0; j < 10; j++) {
        if (i == 5 && j == 5) {
            goto done;
        }
    }
}
done:
    /* Execution jumps here */

/* Solution 2: Flag variable */
int found = 0;
for (int i = 0; i < 10 && !found; i++) {
    for (int j = 0; j < 10 && !found; j++) {
        if (i == 5 && j == 5) {
            found = 1;
        }
    }
}

/* Solution 3: Refactor into function with return */
int search(int arr[][10], int rows, int target) {
    for (int i = 0; i < rows; i++) {
        for (int j = 0; j < 10; j++) {
            if (arr[i][j] == target) {
                return 1;  /* return exits the function entirely */
            }
        }
    }
    return 0;
}
```

---

## `continue` {#continue}

### What It Actually Means

`continue` skips the **rest of the current loop iteration** and proceeds to the next iteration.

```c
/* continue with different loop types: */

/* ── for loop ── */
for (int i = 0; i < 10; i++) {
    if (i % 2 == 0) continue;   /* Skip even numbers */
    /* Goes to: i++ then i < 10 check */
    printf("%d ", i);            /* Prints: 1 3 5 7 9 */
}

/* ── while loop ── */
int i = 0;
while (i < 10) {
    i++;
    if (i % 2 == 0) continue;   /* Skip even */
    /* Goes to: while (i < 10) check */
    printf("%d ", i);            /* Prints: 1 3 5 7 9 */
}

/* ── do-while loop ── */
int j = 0;
do {
    j++;
    if (j % 2 == 0) continue;   /* Skip even */
    /* Goes to: while (j < 10) check */
    printf("%d ", j);
} while (j < 10);

/* ── continue does NOT work with switch ── */
switch (x) {
    case 1:
        continue;  /* ERROR if not inside a loop. If inside a loop,
                      this continues the LOOP, not the switch! */
}
```

---

## `return` {#return}

### Exact Usage and Traps

```c
/* ── Return type must match function declaration ── */

int add(int a, int b) {
    return a + b;         /* Returns int — correct */
    return 3.14;          /* Returns double, truncated to int — warning */
    return;               /* ERROR: must return a value for non-void function */
}

void print_msg(void) {
    printf("hello\n");
    return;               /* Optional for void functions */
    return 5;             /* ERROR: can't return value from void function */
}

/* ── Returning local variables — the DEADLY mistake ── */
int *get_local(void) {
    int x = 42;
    return &x;   /* UNDEFINED BEHAVIOR: x is on the stack.
                    After function returns, stack frame is gone.
                    The returned pointer is a dangling pointer.
                    Accessing it is UB — may crash, may seem to work,
                    may corrupt other data. */
}

/* CORRECT: Return a copy, not a pointer to local */
int get_value(void) {
    int x = 42;
    return x;    /* Copy of x returned — safe */
}

/* CORRECT: Return pointer to heap memory (caller must free!) */
int *get_allocated(void) {
    int *p = malloc(sizeof(int));
    *p = 42;
    return p;    /* Heap memory — safe, but caller must free(p) */
}

/* CORRECT: Return pointer to static memory (thread-unsafe but technically valid) */
int *get_static(void) {
    static int x = 42;
    return &x;  /* Static storage — always valid, but not thread-safe */
}
```

**Return in main — special rules**:

```c
int main(void) {
    return 0;    /* 0 = success (EXIT_SUCCESS) */
    return 1;    /* Non-zero = failure (EXIT_FAILURE) */
    /* In C99 and later: falling off main() without return is same as return 0 */
    /* In C89: falling off main without return is UB */
}
```

---

## `goto` {#goto}

### What It Actually Means

`goto` is an **unconditional jump** to a labeled statement within the same function. It is perhaps the most **feared but misunderstood** keyword in C.

```
GOTO FLOW:

    Statement 1
    goto label;        ──────────────────────────────┐
    Statement 2        ← SKIPPED                      │
    Statement 3        ← SKIPPED                      │
    ...                                               │
label:                 ◄──────────────────────────────┘
    Statement N        ← Execution resumes here
```

**goto is not always evil — the Linux kernel uses it extensively**:

```c
/*
 * THE LEGITIMATE USE CASE: Error handling with cleanup.
 *
 * This pattern avoids deeply nested if statements and
 * ensures proper cleanup in all error paths.
 * The Linux kernel uses this CONSTANTLY.
 */

int initialize_system(void) {
    int err;
    void *resource_a = NULL;
    void *resource_b = NULL;
    void *resource_c = NULL;
    
    resource_a = allocate_a();
    if (!resource_a) {
        err = -ENOMEM;
        goto fail_a;
    }
    
    resource_b = allocate_b();
    if (!resource_b) {
        err = -ENOMEM;
        goto fail_b;
    }
    
    resource_c = allocate_c();
    if (!resource_c) {
        err = -ENOMEM;
        goto fail_c;
    }
    
    /* All succeeded */
    return 0;

fail_c:
    free_b(resource_b);   /* Clean up in reverse order */
fail_b:
    free_a(resource_a);
fail_a:
    return err;
}

/*
 * FLOW DIAGRAM:
 *
 * allocate_a() ──fail──► goto fail_a ──► return err
 *      │
 *   success
 *      │
 * allocate_b() ──fail──► goto fail_b ──► free_a ──► return err
 *      │
 *   success
 *      │
 * allocate_c() ──fail──► goto fail_c ──► free_b ──► free_a ──► return err
 *      │
 *   success
 *      │
 *   return 0
 *
 * This is CLEAN, LINEAR, and CORRECT.
 * Without goto, this requires deeply nested if-else or repeated cleanup code.
 */
```

**Restrictions on goto**:

```c
/* goto CANNOT jump over variable initialization in C99/C11 */
goto skip;
int x = 10;   /* Jumps over this — undefined if x is used later */
skip:
printf("%d\n", x);   /* UB: x may be uninitialized */

/* goto CANNOT jump into a different function */
void foo(void);
void bar(void) {
    goto label;  /* ERROR: label is in foo(), can't jump there */
}
void foo(void) {
    label:
    printf("foo\n");
}

/* goto CAN jump backward (loops) */
int i = 0;
loop:
    if (i < 10) {
        printf("%d\n", i++);
        goto loop;
    }
/* This is a valid (but unusual) loop using goto */
```

---

# CATEGORY 5 — Loop Keywords {#category-5-loop-keywords}

---

## `for` {#for}

### Anatomy of the for Loop

```
for ( INIT ; CONDITION ; UPDATE ) {
      ────   ─────────   ──────
       (1)      (2)        (3)
        │        │          │
        │        │          └── Executed after each iteration
        │        └──────────── Tested before each iteration
        └───────────────────── Executed once before loop starts

EXECUTION ORDER:
(1) INIT
    │
    ▼
(2) CONDITION ──false──► [after loop]
    │ true
    ▼
[loop body]
    │
    ▼
(3) UPDATE
    │
    └──────────────────► (2) CONDITION
```

**All parts are optional**:

```c
/* All three parts can be omitted: */
for (;;) {
    /* Infinite loop — common in embedded systems and servers */
    /* Must break out with: break, return, goto, exit() */
}

/* Init can declare variable (C99 and later): */
for (int i = 0; i < 10; i++) { }
/* i is scoped to the loop — doesn't exist after the loop */

/* Multiple expressions in init and update: */
for (int i = 0, j = 10; i < j; i++, j--) {
    printf("i=%d j=%d\n", i, j);
}
/* Output: 0 10, 1 9, 2 8, 3 7, 4 6 */
/* Stops when i == j (both at 5) */
```

**The off-by-one error (OBOE) — most common loop bug**:

```c
int arr[10];   /* Valid indices: 0 to 9 */

/* WRONG: reads arr[10] — one past the end — UB */
for (int i = 0; i <= 10; i++) {
    printf("%d\n", arr[i]);  /* arr[10] is out of bounds */
}

/* CORRECT: */
for (int i = 0; i < 10; i++) {   /* i goes 0,1,2,...,9 */
    printf("%d\n", arr[i]);
}

/* MENTAL MODEL: 
   For arrays of size N:
     - Start at 0 (first element)
     - Condition: i < N (not <=)
     - Last valid index: N-1
*/
```

---

## `while` {#while}

### Exact Usage

```c
/* while tests condition BEFORE the body executes. */
/* If condition is false initially, body NEVER executes. */

int n = 0;
while (n < 0) {        /* False immediately — body never runs */
    printf("never\n");
}

/* Common while patterns: */

/* 1. Reading until EOF */
int ch;
while ((ch = getchar()) != EOF) {
    putchar(ch);
}

/* 2. Pointer traversal */
char *p = str;
while (*p != '\0') {
    process(*p);
    p++;
}
/* Equivalent but idiomatic: while (*p) process(*p++); */

/* 3. Waiting for condition */
while (!is_ready()) {
    sleep(1);
}
```

---

## `do` {#do}

### What Makes do-while Different

```c
/* do-while tests condition AFTER the body executes. */
/* Body ALWAYS executes at least once. */

int n;
do {
    printf("Enter a positive number: ");
    scanf("%d", &n);
} while (n <= 0);   /* Repeats until user enters positive number */
/* Without do-while, you'd need to duplicate the prompt or use a flag */

/* do-while in macros (advanced but critical): */
#define SAFE_FREE(p)  do { free(p); (p) = NULL; } while (0)

/*
 * WHY do { } while(0) in macros?
 *
 * Without it:
 *   #define SAFE_FREE(p)  free(p); (p) = NULL;
 *
 *   if (condition)
 *       SAFE_FREE(ptr);   expands to:
 *   if (condition)
 *       free(ptr);
 *   (ptr) = NULL;          ← Always executes, not part of if!
 *
 * With do { } while(0):
 *   if (condition)
 *       SAFE_FREE(ptr);   expands to:
 *   if (condition)
 *       do { free(ptr); (ptr) = NULL; } while(0);
 *   The entire multi-statement block is properly grouped.
 *   The while(0) makes the loop execute exactly once.
 *   The trailing semicolon after SAFE_FREE(ptr); "consumes" 
 *   the semicolon from the macro expansion correctly.
 */
```

---

# CATEGORY 6 — Aggregate Type Keywords {#category-6-aggregate-type-keywords}

---

## `struct` {#struct}

### What It Actually Means

`struct` creates a **compound type** — a collection of variables (called **members** or **fields**) grouped together under one name, stored **contiguously in memory**.

```
STRUCT MEMORY LAYOUT:

struct Point {
    int x;    /* 4 bytes */
    int y;    /* 4 bytes */
};

Memory:
┌────────────────────────────────────┐
│  x (4 bytes)  │  y (4 bytes)       │
│  offset: 0    │  offset: 4         │
└────────────────────────────────────┘
  0x00           0x04           0x08
Total: 8 bytes

struct Mixed {
    char   a;    /* 1 byte  */
    int    b;    /* 4 bytes */
    char   c;    /* 1 byte  */
    double d;    /* 8 bytes */
};

Memory (WITH padding for alignment):
┌───┬────────────────┬───┬──────────────┬────────────────────────┐
│ a │  padding(3)    │ b │   padding(3) │ c │padding(7)│    d    │
│ 1 │  3 bytes       │ 4 │              │ 1 │          │ 8 bytes │
└───┴────────────────┴───┴──────────────┴───┴──────────┴─────────┘
 0x00  0x01-0x03    0x04  0x08         0x09  0x0A-0x0F  0x10

Wait — let me draw this more carefully:

Offset 0:  char a       (1 byte)
Offset 1:  [3 bytes padding — int b must be at 4-byte aligned address]
Offset 4:  int b        (4 bytes)
Offset 8:  char c       (1 byte)
Offset 9:  [7 bytes padding — double d must be at 8-byte aligned address]
Offset 16: double d     (8 bytes)
Total:     24 bytes (NOT 14!)

ALIGNMENT RULE: Each member is stored at an address that is a
multiple of its own size (for natural alignment).
  char:   1-byte aligned (any address)
  short:  2-byte aligned (even addresses)
  int:    4-byte aligned (divisible by 4)
  double: 8-byte aligned (divisible by 8)
```

**Struct padding and packing**:

```c
#include <stdio.h>
#include <stddef.h>

struct Wasteful {
    char   a;      /* 1 byte at offset 0 */
    double b;      /* 8 bytes at offset 8 (7 bytes padding!) */
    char   c;      /* 1 byte at offset 16 */
};
/* sizeof: 1 + 7(pad) + 8 + 1 + 7(pad) = 24 bytes */

struct Efficient {
    double b;      /* 8 bytes at offset 0 */
    char   a;      /* 1 byte at offset 8 */
    char   c;      /* 1 byte at offset 9 */
};
/* sizeof: 8 + 1 + 1 + 6(pad to next 8 boundary) = 16 bytes */
/* RULE: Order struct fields largest to smallest to minimize padding */

/* PACKED struct — no padding (may cause unaligned access): */
struct __attribute__((packed)) NetworkPacket {
    uint8_t  type;       /* 1 byte */
    uint32_t length;     /* 4 bytes — now at offset 1! Unaligned! */
    uint16_t checksum;   /* 2 bytes */
};
/* Total: 7 bytes (no padding) */
/* WARNING: Accessing .length on x86 works but is slow.
            On ARM (strict alignment), accessing unaligned int = SIGBUS! */

/* Check offsets with offsetof: */
printf("offset of b: %zu\n", offsetof(struct Wasteful, b));  /* 8 */
printf("sizeof struct: %zu\n", sizeof(struct Wasteful));      /* 24 */
```

**Flexible array members (C99)**:

```c
/*
 * A struct can end with an array of unspecified size.
 * This allows variable-length structs.
 */

struct Message {
    int  type;
    int  length;
    char data[];   /* Flexible array member — no size specified */
};

/* Allocate space for struct + data: */
int data_len = 100;
struct Message *msg = malloc(sizeof(struct Message) + data_len);
msg->type   = 1;
msg->length = data_len;
memcpy(msg->data, source_data, data_len);

/*
 * sizeof(struct Message) does NOT include the flexible array.
 * sizeof(struct Message) = sizeof(int) + sizeof(int) = 8 bytes.
 * The flexible array occupies the memory allocated after the struct.
 *
 * Linux kernel uses this everywhere:
 *   struct sk_buff (socket buffer) has flexible data at the end
 *   struct nlmsghdr (netlink message) uses this pattern
 */
```

**Linux Kernel struct patterns**:

```c
/*
 * The Linux kernel has thousands of structs.
 * Key patterns:
 */

/* 1. list_head — embedded doubly-linked list */
struct list_head {
    struct list_head *next, *prev;
};

struct my_item {
    int data;
    struct list_head list;  /* Embed list_head in your struct */
};

/* Access the containing struct from a list_head pointer: */
/* container_of(ptr, type, member) macro */
struct my_item *item = container_of(list_ptr, struct my_item, list);

/* 2. kobject — base object for reference counting */
struct kobject {
    const char         *name;
    struct list_head    entry;
    struct kobject     *parent;
    struct kset        *kset;
    const struct kobj_type *ktype;
    struct kernfs_node *sd;
    struct kref         kref;   /* reference count */
    /* ... */
};

/* 3. Opaque pointers (hide struct internals): */
/* In header: */
struct MyPrivate;   /* Forward declaration — incomplete type */
struct MyPrivate *create(void);
void destroy(struct MyPrivate *p);

/* In implementation (.c file only): */
struct MyPrivate {
    int secret_data;
    /* ... */
};
```

---

## `union` {#union}

### What It Actually Means

`union` creates a type where **all members share the same memory location**. The size of a union is the size of its **largest member**. Only one member should be "active" at any time.

```
UNION MEMORY LAYOUT:

union Data {
    int    i;      /* 4 bytes */
    float  f;      /* 4 bytes */
    char   c[4];   /* 4 bytes */
};

ALL MEMBERS OCCUPY THE SAME MEMORY:

Memory: ┌────────────────────────────────────┐
        │           4 bytes                  │
        └────────────────────────────────────┘
              ↑              ↑            ↑
          int i           float f      char c[4]
        (interprets    (interprets    (interprets
        4 bytes as      4 bytes as    4 bytes as
        32-bit int)     float)        char array)

sizeof(union Data) = 4 (the largest member)
sizeof(struct Data) = 12 (sum of all members + padding)
```

**Legitimate Uses of union**:

```c
/* ── USE 1: Type punning (inspecting bit patterns) ── */
union FloatBits {
    float    f;
    uint32_t bits;
};

union FloatBits fb;
fb.f = 3.14f;
printf("3.14f in hex: 0x%08X\n", fb.bits);
/* Output: 0x4048F5C3 — the IEEE 754 bit pattern of 3.14 */

/*
 * WARNING: Reading from a union member other than the last-written
 * member is TECHNICALLY undefined behavior in C (but defined in C11
 * for "type punning" — reading char arrays to inspect bytes is always safe).
 * In practice, GCC/Clang treat union type punning as defined behavior.
 */

/* SAFE type punning with memcpy (always well-defined): */
float value = 3.14f;
uint32_t bits;
memcpy(&bits, &value, sizeof(bits));
printf("bits: 0x%08X\n", bits);

/* ── USE 2: Tagged union (discriminated union) — simulating variant types ── */
typedef enum { TYPE_INT, TYPE_FLOAT, TYPE_STRING } ValueType;

typedef struct {
    ValueType type;    /* Discriminator — tells which union member is active */
    union {
        int    i;
        float  f;
        char  *s;
    } data;
} Value;

Value v1 = { .type = TYPE_INT,   .data.i = 42 };
Value v2 = { .type = TYPE_FLOAT, .data.f = 3.14f };
Value v3 = { .type = TYPE_STRING,.data.s = "hello" };

void print_value(const Value *v) {
    switch (v->type) {
        case TYPE_INT:    printf("int: %d\n",    v->data.i); break;
        case TYPE_FLOAT:  printf("float: %.2f\n",v->data.f); break;
        case TYPE_STRING: printf("str: %s\n",    v->data.s); break;
    }
}

/* ── USE 3: Network protocol parsing ── */
union IPv4Address {
    uint32_t whole;      /* Full 32-bit address */
    uint8_t  octets[4]; /* Individual octets */
};

union IPv4Address ip;
ip.whole = 0xC0A80001;  /* 192.168.0.1 in network byte order */
printf("%d.%d.%d.%d\n", ip.octets[3], ip.octets[2], 
                          ip.octets[1], ip.octets[0]);
/* On little-endian: prints 192.168.0.1 */
```

---

## `enum` {#enum}

### What It Actually Means

`enum` creates a named set of **integer constants**. Values default to 0, 1, 2... but can be explicitly set.

```c
/* Basic enum: */
enum Direction {
    NORTH,   /* = 0 (default) */
    EAST,    /* = 1 */
    SOUTH,   /* = 2 */
    WEST     /* = 3 */
};

/* Explicit values: */
enum Color {
    RED   = 1,
    GREEN = 2,
    BLUE  = 4,
    WHITE = RED | GREEN | BLUE   /* = 7 — bit flags! */
};

/* Mixed: */
enum StatusCode {
    OK          =  0,
    NOT_FOUND   = 404,
    SERVER_ERR  = 500,
    NEXT_CODE           /* = 501 — continues from previous */
};
```

**Critical enum facts most people don't know**:

```c
/*
 * FACT 1: enum constants ARE int.
 *         The enum type itself may or may not be int — compiler decides.
 *         In C (not C++), enum and int are freely convertible.
 */

enum Direction d = 42;   /* Legal in C — enum is just int */
int x = NORTH;           /* Legal — enum constant is int */

/* FACT 2: enum doesn't prevent using values outside the enum */
enum Color c = 999;  /* Legal in C — no range check */

/* FACT 3: Two enum types are distinct but compare as int */
enum A { X = 1 };
enum B { Y = 1 };
enum A a = X;
enum B b = Y;
if (a == b) { }   /* Compiles — they're both int value 1 */

/* FACT 4: Use enum for true compile-time constants */
enum { BUFFER_SIZE = 4096 };    /* Better than #define for debugging */
char buf[BUFFER_SIZE];           /* Works — this IS a constant expression */
```

**The enum-as-bit-flags pattern** (Linux kernel style):

```c
/* File permission bits — powers of 2 for bitmasking: */
enum FileMode {
    MODE_READ    = (1 << 0),  /* 0001 = 1  */
    MODE_WRITE   = (1 << 1),  /* 0010 = 2  */
    MODE_EXEC    = (1 << 2),  /* 0100 = 4  */
    MODE_SETUID  = (1 << 3),  /* 1000 = 8  */
};

/* Set multiple flags: */
int perms = MODE_READ | MODE_WRITE;   /* 0011 = 3 */

/* Test a flag: */
if (perms & MODE_READ) { /* can read */ }

/* Clear a flag: */
perms &= ~MODE_WRITE;   /* Clear write bit */

/* Toggle a flag: */
perms ^= MODE_EXEC;     /* Toggle execute bit */
```

---

## `typedef` {#typedef}

### What It Actually Means

`typedef` creates an **alias** for an existing type. It does NOT create a new type — it creates a new **name** for an existing type.

```c
/* Basic syntax: typedef existing_type new_name; */

typedef unsigned long int uint64_t_example;   /* New name for unsigned long int */
typedef char *StringPtr;                       /* New name for char* */
typedef int (*FuncPtr)(int, int);              /* New name for function pointer type */

/* With struct: */
typedef struct {
    int x;
    int y;
} Point;

Point p;       /* No 'struct' keyword needed */
p.x = 1;
p.y = 2;

/* Versus without typedef: */
struct PointRaw {
    int x;
    int y;
};
struct PointRaw p2;  /* Must say 'struct' */
```

**The Self-Referential Struct with typedef**:

```c
/*
 * For self-referential structs (linked lists, trees), you need
 * the tag name because the typedef isn't complete yet:
 */

/* WRONG: */
typedef struct {
    int data;
    Node *next;  /* ERROR: 'Node' not yet defined at this point */
} Node;

/* CORRECT: Use the struct tag: */
typedef struct Node {
    int data;
    struct Node *next;   /* Use 'struct Node' — the tag is defined */
} Node;
/* Now 'Node' and 'struct Node' both refer to the same type */

/* Alternative: forward declaration */
typedef struct Node Node;   /* Declare the typedef first */
struct Node {
    int data;
    Node *next;             /* Now Node is defined, can use it */
};
```

**Common typedef patterns in systems code**:

```c
/* Fixed-width integer types (<stdint.h>): */
typedef unsigned char  uint8_t;
typedef unsigned short uint16_t;
typedef unsigned int   uint32_t;
typedef unsigned long  uint64_t;   /* Platform-dependent! May need long long */

/* Function pointer typedefs — dramatically improve readability: */
/* Without typedef: */
int (*compare_fn)(const void *, const void *);
void qsort(void *base, size_t nmemb, size_t size,
           int (*compar)(const void *, const void *));

/* With typedef: */
typedef int (*CompareFunc)(const void *, const void *);
void qsort(void *base, size_t nmemb, size_t size, CompareFunc compar);
/* Much cleaner! */

/* Opaque handle pattern: */
typedef void *FileHandle;   /* Hides implementation, callers just use FileHandle */
FileHandle open_file(const char *path);
void close_file(FileHandle h);
```

---

# CATEGORY 7 — Size and Alignment Keywords {#category-7-size-and-alignment-keywords}

---

## `sizeof` {#sizeof}

### What It Actually Means

`sizeof` is a **compile-time operator** (not a function) that returns the size in bytes of a type or object. The return type is `size_t` (an unsigned integer type).

```c
/*
 * sizeof is evaluated at COMPILE TIME (with one exception: VLAs in C99).
 * It does NOT evaluate its operand at runtime:
 */

int x = 5;
size_t s = sizeof(x);    /* Result: 4 (on most systems). x is NOT accessed. */
size_t s2 = sizeof(x++); /* x is NOT incremented! sizeof doesn't evaluate. */
/* After this: x is still 5 */

/* sizeof with parentheses: */
sizeof(int)     /* Type: requires parentheses */
sizeof x        /* Variable: parentheses optional but conventional */
sizeof(x)       /* Also fine */

/* sizeof arrays: */
int arr[10];
sizeof(arr)           /* = 40 (10 * sizeof(int) = 10 * 4) */
sizeof(arr)/sizeof(arr[0])   /* = 10 — number of elements */

/* THE CLASSIC POINTER vs ARRAY MISTAKE: */
void process(int arr[]) {   /* arr is actually int* — array decays to pointer! */
    size_t n = sizeof(arr) / sizeof(arr[0]);  /* WRONG: sizeof(arr) = sizeof(int*) = 8 */
}

/* CORRECT: Pass the size explicitly */
void process(int *arr, size_t n) { }

/* sizeof on structs — includes padding: */
struct {
    char c;    /* 1 byte + 3 padding */
    int  i;    /* 4 bytes */
} s;
sizeof(s) == 8  /* NOT 5 — padding is included */
```

**The `sizeof(*ptr)` idiom** — critical for malloc:

```c
/* FRAGILE: repeating the type */
int *p = malloc(sizeof(int) * 100);

/* ROBUST: sizeof applied to dereferenced pointer */
int *p = malloc(sizeof(*p) * 100);
/*
 * If you later change 'int *p' to 'double *p',
 * sizeof(*p) automatically updates.
 * The first version would still allocate sizeof(int) * 100 — a bug.
 */

/* Linux kernel style: */
struct my_struct *obj = kmalloc(sizeof(*obj), GFP_KERNEL);
/* If obj's type changes, the allocation automatically stays correct. */
```

---

## `_Alignas` / `_Alignof` (C11) {#alignas-alignof}

### What They Actually Mean

**`_Alignof(type)`**: Returns the alignment requirement (in bytes) of a type — the address must be a multiple of this value.

**`_Alignas(N)` or `_Alignas(type)`**: Specifies that a variable must be aligned to at least N bytes (or the alignment of the given type).

```c
#include <stdalign.h>   /* Provides alignas and alignof macros */

/* Alignment requirements on x86-64: */
_Alignof(char)    /* = 1 */
_Alignof(short)   /* = 2 */
_Alignof(int)     /* = 4 */
_Alignof(double)  /* = 8 */

/* Forcing alignment: */
alignas(64) int cache_line_data[16];
/*
 * 64-byte alignment = one cache line on most modern CPUs.
 * Ensures this array starts at a cache line boundary.
 * Prevents false sharing between threads on different data.
 */

/* SIMD alignment (SSE requires 16-byte, AVX requires 32-byte): */
alignas(32) float simd_buffer[8];   /* 8 floats = 256 bits = AVX register */

/* Alignment in structs: */
struct CacheAligned {
    alignas(64) int data[16];    /* First member: cache-line aligned */
    int other;                    /* Will follow immediately after */
};
```

**Linux kernel alignment usage**:

```c
/*
 * Kernel defines __aligned() for GCC attribute:
 *   #define __aligned(x) __attribute__((aligned(x)))
 *
 * Per-CPU variables must be cache-line aligned to prevent false sharing:
 */

DEFINE_PER_CPU_ALIGNED(struct cpu_stats, cpu_stats);
/* Expands to a per-CPU variable with 64-byte (cache line) alignment */

/* Interrupt descriptor table: */
struct gate_struct {
    u16    offset_low;
    u16    segment;
    struct idt_bits bits;
    u16    offset_middle;
    u32    offset_high;
    u32    reserved;
} __attribute__((packed));
```

---

# CATEGORY 8 — Function Keywords {#category-8-function-keywords}

---

## `inline` (C99) {#inline}

### What It Actually Means

`inline` is a **hint** to the compiler that calls to this function should be replaced by the function's code directly at the call site, eliminating function call overhead.

```
WITHOUT inline:
  Caller code:              Called function:
  ┌──────────────┐         ┌──────────────────┐
  │  ...         │         │ int add(int a,    │
  │  CALL add    │────────►│        int b) {  │
  │  (push args  │         │   return a + b;  │
  │   jump to    │         │ }                │
  │   add's addr)│◄────────│ (RET instruction) │
  │  ...         │         └──────────────────┘
  └──────────────┘

  Overhead: push args to stack, jump, setup frame, return, cleanup.
  ~5-10 instructions of overhead for a 1-instruction function.

WITH inline:
  ┌──────────────┐
  │  ...         │
  │  temp = a+b  │  ← Function body substituted HERE
  │  (no call!)  │
  │  ...         │
  └──────────────┘
  Zero overhead.
```

**The Three Faces of inline in C99**:

```c
/*
 * C99 inline has CONFUSING rules that most programmers get wrong.
 *
 * 1. inline (without extern or static):
 *    The function has inline linkage.
 *    The compiler may or may not inline it.
 *    An EXTERNAL definition must exist somewhere (one translation unit
 *    must define it WITHOUT the inline keyword or with 'extern inline').
 *    If multiple .c files include the same inline function definition,
 *    the linker must pick exactly one external definition.
 *
 * 2. static inline:
 *    The function is file-private AND the compiler may inline it.
 *    THIS IS WHAT YOU ALMOST ALWAYS WANT.
 *    No external definition needed.
 *    Can be defined in a header — each .c file gets its own copy.
 *    The most common and safe usage.
 *
 * 3. extern inline:
 *    Forces an external (non-inline) definition in this translation unit.
 *    Other translation units that see the inline definition will use THIS
 *    as the external fallback.
 *    Used in pairs: one file has inline, one has extern inline.
 *    Rarely needed; complex.
 */

/* ── CORRECT common usage: static inline ── */
/* In a header file (mymath.h): */
static inline int max(int a, int b) {
    return (a > b) ? a : b;
}
/* Every .c file that includes this header gets its own copy.
   The compiler will likely inline the call.
   No linker issues. */

/* ── WRONG: non-static inline in header ── */
/* In header: */
inline int min(int a, int b) {   /* WRONG for header! */
    return (a < b) ? a : b;
}
/* Multiple .c files include this → multiple external definitions!
   Linker error or undefined behavior. */
```

**`inline` is just a hint — the compiler may ignore it**:

```c
/*
 * Modern compilers inline functions based on heuristics,
 * regardless of the 'inline' keyword.
 * A large function marked 'inline' will NOT be inlined.
 * A small function NOT marked 'inline' WILL be inlined at -O2.
 *
 * To FORCE inlining (GCC/Clang):
 */
__attribute__((always_inline)) static inline int my_func(int x) {
    return x * 2;
}

/* To PREVENT inlining: */
__attribute__((noinline)) int no_inline_func(int x) {
    return x * 3;
}
/* Used when you want to measure a function's performance accurately
   or when inlining would cause code size explosion. */
```

---

## `_Noreturn` (C11) {#noreturn}

### What It Actually Means

`_Noreturn` (or `noreturn` with `<stdnoreturn.h>`) tells the compiler that a function **never returns** to its caller. It either loops forever, calls `exit()`, `abort()`, `longjmp()`, or throws an exception.

```c
#include <stdlib.h>
#include <stdnoreturn.h>

/* Using the macro: */
noreturn void fatal_error(const char *msg) {
    fprintf(stderr, "FATAL: %s\n", msg);
    abort();   /* Never returns */
}

/* Using the keyword directly: */
_Noreturn void exit_with_code(int code) {
    exit(code);  /* Never returns */
}

/*
 * WHY THIS MATTERS:
 *
 * 1. Compiler optimization: Code after a noreturn call is dead code.
 *    The compiler won't generate it.
 *
 * 2. Warning suppression: Without _Noreturn, the compiler warns about
 *    missing return statements after calls to functions that don't return:
 *
 *    int get_value(int x) {
 *        if (x < 0) {
 *            fatal_error("negative");  // Compiler doesn't know this never returns
 *        }
 *        return x * 2;
 *        // Compiler may warn: "control may reach end of non-void function"
 *        // With _Noreturn on fatal_error, no warning.
 *    }
 *
 * 3. Helps static analysis tools understand control flow.
 */

/* Linux kernel equivalent: */
/* __noreturn is defined as __attribute__((noreturn)) */
__noreturn void panic(const char *fmt, ...);
```

---

# CATEGORY 9 — C99 Special Type Keywords {#category-9-c99-special-type-keywords}

---

## `_Bool` {#bool}

### What It Actually Means

`_Bool` is C99's built-in boolean type. It can hold only **0** or **1**.

```c
#include <stdbool.h>   /* Provides 'bool', 'true', 'false' macros */

/* Without stdbool.h: */
_Bool flag = 0;        /* false */
_Bool flag2 = 1;       /* true */
_Bool flag3 = 42;      /* Stored as 1 — any non-zero value becomes 1 */
_Bool flag4 = -1;      /* Stored as 1 — same rule */

/* With stdbool.h (preferred): */
bool found = false;
bool valid = true;

/*
 * _Bool is an unsigned integer type.
 * sizeof(_Bool) is typically 1 (1 byte), not 1 bit.
 * The value is always normalized to 0 or 1.
 *
 * This normalization is the KEY difference from int:
 */

int x = 42;
_Bool b = x;    /* b = 1 (normalized!) */
int y = b;      /* y = 1 */

/* C89 boolean simulation: */
#define BOOL int
#define TRUE  1
#define FALSE 0
/* Problem: no normalization. TRUE could be any non-zero value. */
/* int b = 42; is "true" but b != TRUE (42 != 1) */
/* _Bool solves this: _Bool b = 42; then b == true is guaranteed. */
```

---

## `_Complex` / `_Imaginary` {#complex-imaginary}

```c
#include <complex.h>

/* Complex number types: */
float _Complex        fc;   /* single precision complex */
double _Complex       dc;   /* double precision complex */
long double _Complex  ldc;  /* extended precision complex */

/* With <complex.h> macros: */
double complex z1 = 3.0 + 4.0 * I;   /* 3 + 4i */
double complex z2 = 1.0 - 2.0 * I;   /* 1 - 2i */

double complex sum = z1 + z2;          /* 4 + 2i */
double complex product = z1 * z2;      /* (3+4i)(1-2i) = 3-6i+4i-8i² = 11-2i */

double magnitude = cabs(z1);           /* |z1| = sqrt(3²+4²) = 5.0 */
double angle     = carg(z1);           /* arg(z1) = atan2(4,3) ≈ 0.927 rad */

/*
 * _Imaginary is rarely supported — most compilers only support _Complex.
 * Practical code uses double complex from <complex.h>.
 * Used in signal processing, physics simulation, electrical engineering.
 */
```

---

# CATEGORY 10 — C11 Concurrency and Atomics {#category-10-c11-concurrency}

---

## `_Atomic` {#atomic}

### What It Actually Means

`_Atomic` declares a variable whose operations are **guaranteed to be atomic** — they appear to complete as a single, indivisible operation from the perspective of all threads.

```
WITHOUT atomics (data race — undefined behavior):

Thread 1:           Thread 2:
LOAD counter        LOAD counter
ADD 1               ADD 1
STORE counter       STORE counter

Problem:
  Thread 1 loads: 5
  Thread 2 loads: 5
  Both add 1: both have 6
  Thread 1 stores: 6
  Thread 2 stores: 6
  Final value: 6 — WRONG! Should be 7.
  This is a "lost update" — a race condition.

WITH _Atomic:

Thread 1:           Thread 2:
atomic_add(1)       atomic_add(1)  ← These are SERIALIZED
                                      One completes before the other
                                      starts. Result: 7. Correct.
```

```c
#include <stdatomic.h>

/* Declaring atomic variables: */
_Atomic int counter = 0;          /* C11 keyword syntax */
atomic_int counter2 = 0;          /* typedef from <stdatomic.h> — same thing */

/* Operations: */
atomic_store(&counter, 42);           /* Atomic write */
int val = atomic_load(&counter);      /* Atomic read */
int old = atomic_exchange(&counter, 100);  /* Atomic swap: returns old, sets new */

/* Atomic arithmetic: */
atomic_fetch_add(&counter, 1);   /* counter++ atomically — returns old value */
atomic_fetch_sub(&counter, 1);   /* counter-- atomically */
atomic_fetch_or(&counter, 0x01); /* counter |= 1 atomically */

/* Compare-and-swap (CAS) — the foundation of lock-free data structures: */
int expected = 5;
int desired  = 10;
bool success = atomic_compare_exchange_strong(&counter, &expected, desired);
/*
 * If counter == expected (5): set counter = desired (10), return true
 * If counter != expected:     set expected = current counter value, return false
 *
 * This is used to implement lock-free algorithms:
 *   "Only update if the value hasn't changed since I last read it."
 */

/* Memory ordering — fine-grained control: */
atomic_store_explicit(&counter, 42, memory_order_release);
int v = atomic_load_explicit(&counter, memory_order_acquire);
/*
 * memory_order_seq_cst:  Default. Sequential consistency. Strongest. Slowest.
 * memory_order_release:  All previous writes visible before this store.
 * memory_order_acquire:  All subsequent reads see writes before the paired release.
 * memory_order_relaxed:  No ordering. Just atomicity. Fastest.
 * memory_order_acq_rel:  Both acquire and release (for read-modify-write ops).
 * memory_order_consume:  Dependency ordering (rarely used, complex).
 */
```

**Lock-free counter example**:

```c
#include <stdatomic.h>
#include <stdio.h>
#include <pthread.h>

atomic_int global_counter = 0;

void *increment_thread(void *arg) {
    for (int i = 0; i < 1000000; i++) {
        atomic_fetch_add_explicit(&global_counter, 1, memory_order_relaxed);
        /* relaxed: we only need atomicity, not ordering */
    }
    return NULL;
}

int main(void) {
    pthread_t t1, t2;
    pthread_create(&t1, NULL, increment_thread, NULL);
    pthread_create(&t2, NULL, increment_thread, NULL);
    pthread_join(t1, NULL);
    pthread_join(t2, NULL);
    printf("Counter: %d\n", atomic_load(&global_counter));
    /* Always prints 2000000 — no race condition */
    return 0;
}
```

---

## `_Thread_local` {#thread-local}

### What It Actually Means

`_Thread_local` (or `thread_local` with C23, or `__thread` as GCC extension) declares a variable with **thread-local storage** — each thread gets its own independent copy of the variable.

```
WITHOUT thread_local:

Thread 1         Thread 2
   │                │
   ├── reads errno  ├── writes errno (async I/O error!)
   │                │
   └── errno is 0?  └── errno = EAGAIN
   
SHARED errno = BOTH THREADS SEE SAME VALUE
Race condition on errno!

WITH thread_local:

Thread 1's stack                Thread 2's stack
┌──────────────────────┐        ┌──────────────────────┐
│  Thread 1 errno = 0  │        │  Thread 2 errno = 11 │
│  (EAGAIN = 11)       │        │  (EAGAIN)            │
└──────────────────────┘        └──────────────────────┘
Completely independent copies — no sharing, no race.
```

```c
#include <threads.h>   /* For thread_local macro in C11 */

/* errno is the canonical example — it IS thread-local in POSIX: */
/* From <errno.h> on Linux: */
/* #define errno (*__errno_location()) */
/* __errno_location() returns a pointer to this thread's errno */

/* Your own thread-local variables: */
_Thread_local int per_thread_counter = 0;
_Thread_local char per_thread_buffer[256];

/* Each thread increments its own counter: */
void thread_function(void) {
    per_thread_counter++;
    /* No race condition — each thread has its own copy */
}

/* Combining with static: */
static _Thread_local int static_thread_local = 0;
/* Per-thread, persists for thread's lifetime, file-scoped */

/*
 * RESTRICTIONS:
 * - _Thread_local can only be used with static storage duration variables
 *   (file-scope variables, static local variables)
 * - Cannot be used with function parameters or auto (stack) variables
 * - Initialization: must be a constant expression (like static)
 */
```

---

# CATEGORY 11 — C11 Generic and Static Assert {#category-11-generic-static-assert}

---

## `_Generic` {#generic}

### What It Actually Means

`_Generic` is a **compile-time type selection** expression — it selects one of several expressions based on the type of a controlling expression.

```c
/*
 * Syntax:
 *   _Generic(controlling_expression,
 *            type1: expression1,
 *            type2: expression2,
 *            default: default_expression)
 *
 * At compile time: the type of controlling_expression is determined.
 * The corresponding expression is substituted.
 * Other branches are NOT evaluated.
 * This is how type-generic macros work in <tgmath.h>.
 */

/* Type-generic absolute value: */
#define ABS(x)  _Generic((x),           \
    int:         abs(x),                 \
    long:        labs(x),                \
    long long:   llabs(x),               \
    float:       fabsf(x),               \
    double:      fabs(x),                \
    long double: fabsl(x),               \
    default:     abs(x)                  \
)

/* Usage: */
int    i = ABS(-5);     /* Calls abs(-5) */
double d = ABS(-3.14);  /* Calls fabs(-3.14) */
float  f = ABS(-1.5f);  /* Calls fabsf(-1.5f) */

/* Type name macro: */
#define TYPENAME(x)  _Generic((x),       \
    int:          "int",                  \
    float:        "float",                \
    double:       "double",               \
    char*:        "char*",                \
    default:      "unknown"               \
)

printf("%s\n", TYPENAME(42));      /* "int" */
printf("%s\n", TYPENAME(3.14));    /* "double" */
printf("%s\n", TYPENAME("hi"));    /* "char*" */
```

---

## `_Static_assert` {#static-assert}

### What It Actually Means

`_Static_assert` performs a **compile-time assertion**. If the condition is false, compilation fails with an error message. No runtime overhead.

```c
#include <assert.h>   /* Provides static_assert macro in C11 */

/* Ensure platform assumptions are correct: */
_Static_assert(sizeof(int) == 4, "int must be 4 bytes on this platform");
_Static_assert(sizeof(void *) == 8, "Expected 64-bit pointers");
_Static_assert(sizeof(long) >= 8, "long must be at least 8 bytes");

/* Ensure struct layout assumptions (for network protocol code): */
_Static_assert(sizeof(struct TCPHeader) == 20, "TCP header must be exactly 20 bytes");
_Static_assert(offsetof(struct TCPHeader, seq_num) == 4, "seq_num at wrong offset");

/* Ensure enum fits in uint8_t: */
typedef enum { A=0, B=1, C=2, MAX_VALUE=255 } MyEnum;
_Static_assert(MAX_VALUE <= 255, "Enum doesn't fit in uint8_t");

/* In C11 with <assert.h>: */
static_assert(sizeof(int) == 4, "int must be 4 bytes");

/*
 * Difference from runtime assert():
 *   static_assert:   Checked at COMPILE TIME. If fails: compilation error.
 *   assert():        Checked at RUNTIME. If fails: aborts program.
 *                    Can be disabled with -DNDEBUG.
 *
 * static_assert has ZERO runtime cost.
 */
```

---

# CATEGORY 12 — C23 New Keywords {#category-12-c23-keywords}

```c
/*
 * C23 (published 2023) adds keywords that were previously macros or extensions.
 * Most require a C23-capable compiler (GCC 13+, Clang 17+).
 */

/* ── bool, true, false — now actual keywords (were macros in C99/C11) ── */
bool flag = true;     /* Now a keyword, no #include <stdbool.h> needed */

/* ── nullptr — proper null pointer constant ── */
int *p = nullptr;     /* Cleaner than NULL (which is (void*)0 or 0) */
                      /* nullptr has type nullptr_t, not void* or int */
                      /* Prevents "NULL == 0" integer comparison confusion */

/* ── alignas, alignof — keywords replacing _Alignas, _Alignof ── */
alignas(64) int cache_data[16];
size_t align = alignof(double);

/* ── static_assert — keyword replacing _Static_assert ── */
static_assert(sizeof(int) == 4, "need 32-bit int");

/* ── thread_local — keyword replacing _Thread_local ── */
thread_local int per_thread = 0;

/* ── typeof / typeof_unqual — get type of an expression ── */
int x = 5;
typeof(x) y = 10;         /* y is int */
typeof(x + 3.14) z = 0;  /* z is double (int + double = double) */

/* typeof_unqual: removes const/volatile/restrict qualifiers */
const int ci = 5;
typeof(ci) a = 10;          /* a is const int */
typeof_unqual(ci) b = 10;   /* b is int (const removed) */

/* ── constexpr — guaranteed compile-time constant ── */
constexpr int BUFFER_SIZE = 4096;    /* TRUE compile-time constant, unlike C89 const */
int arr[BUFFER_SIZE];                 /* Always valid — not a VLA */

/* ── _BitInt(N) — precise-width integers ── */
_BitInt(7) x7;    /* 7-bit signed integer: -64 to 63 */
_BitInt(128) big; /* 128-bit integer (larger than long long!) */
unsigned _BitInt(3) u3;  /* 3-bit unsigned: 0 to 7 */
```

---

# LINUX KERNEL PATTERNS {#linux-kernel-patterns}

```
LINUX KERNEL KEYWORD USAGE MAP:

static     ── Used everywhere for file-private functions and variables
const      ── File operation tables, lookup tables, strings
volatile   ── jiffies, memory-mapped device registers, flags
extern     ── Cross-subsystem declarations in headers
inline     ── static inline for small, hot functions
_Atomic    ── Reference counts, lock-free data, CPU-local counters
restrict   ── Memory copy helpers (memcpy_user, etc.)
typeof     ── container_of macro, type-safe macros (GCC extension)
__packed   ── Protocol headers (not a C keyword — GCC attribute)
__aligned  ── Cache line alignment (GCC attribute equivalent of alignas)
_Noreturn  ── panic(), BUG(), die() functions
```

**The `container_of` macro — kernel's most used macro**:

```c
/*
 * container_of: Given a pointer to a MEMBER of a struct,
 * get the pointer to the CONTAINING struct.
 *
 * Used with embedded list_head, hlist_node, etc.
 */

#define container_of(ptr, type, member) ({                      \
    const typeof( ((type *)0)->member ) *__mptr = (ptr);        \
    (type *)( (char *)__mptr - offsetof(type, member) );})

/*
 * HOW IT WORKS:
 *
 * struct my_device {
 *     int id;
 *     struct list_head list;   ← We have a pointer to THIS
 *     char name[32];
 * };
 *
 * Given: struct list_head *lp
 * Want:  struct my_device *dev
 *
 * MEMORY LAYOUT:
 * ┌──────────────────────────────────────────────────┐
 * │  id (4)  │ padding │  list (16)  │  name (32)   │
 * └──────────────────────────────────────────────────┘
 * ^                    ^
 * │                    │
 * dev (start)          lp (pointer we have)
 *                      │
 *                      └── offset = offsetof(struct my_device, list)
 *
 * dev = (char*)lp - offsetof(struct my_device, list)
 *     = subtract the offset → arrive at struct start
 *
 * container_of(lp, struct my_device, list) gives us 'dev'
 */

/* Usage in kernel list iteration: */
struct list_head *pos;
list_for_each(pos, &device_list) {
    struct my_device *dev = container_of(pos, struct my_device, list);
    printk(KERN_INFO "device: %s\n", dev->name);
}
```

**Linux kernel GFP flags — a real example of proper keyword usage**:

```c
/*
 * Memory allocation in kernel context requires flags
 * because you can't always sleep (block for memory):
 *
 * GFP = Get Free Pages
 */

/* Interrupt context — CANNOT sleep: */
void *p = kmalloc(size, GFP_ATOMIC);
/*              GFP_ATOMIC: Never sleeps. May fail. Use in interrupt handlers. */

/* Process context — CAN sleep: */
void *q = kmalloc(size, GFP_KERNEL);
/*              GFP_KERNEL: May sleep. Higher chance of success. */

/* User-space memory: */
unsigned long page = get_zeroed_page(GFP_USER);

/*
 * The keyword usage here:
 *   - size_t size: always for memory sizes
 *   - gfp_t flags: typedef'd unsigned int — explicit typing
 *   - void *return: generic pointer, caller casts appropriately
 *   - Static functions in kmalloc path: optimized per-context
 *   - __must_check attribute: caller must check return value
 */
```

---

# FATAL MISTAKES MASTER REFERENCE {#fatal-mistakes-master-reference}

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    FATAL MISTAKES — CAUSES AND EFFECTS                      │
├─────────────────────────┬────────────────────────┬─────────────────────────┤
│ MISTAKE                 │ ROOT CAUSE             │ RESULT                  │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ Returning local var ptr │ Stack frame freed on   │ Dangling pointer,       │
│                         │ function return        │ SIGSEGV or corruption   │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ Signed overflow         │ UB — compiler may      │ Miscompiled code,       │
│ (INT_MAX + 1)           │ optimize assuming it   │ security vulnerabilities │
│                         │ never happens          │                         │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ Unsigned loop countdown │ i-- at 0 wraps to      │ Infinite loop,          │
│ while (i-- >= 0)        │ UINT_MAX               │ buffer overflow         │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ Comparing signed and    │ Signed promoted to     │ Logic errors: -1 > 1    │
│ unsigned (int vs size_t)│ unsigned silently      │ appears to be TRUE      │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ switch fallthrough      │ No break; compiler     │ Executes wrong cases,   │
│ without break           │ does not warn by default│ corrupt state          │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ sizeof on decayed array │ Array parameter becomes│ Wrong size calc,        │
│ parameter               │ pointer in function    │ buffer overflow         │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ Using volatile for      │ volatile has no        │ Race condition,         │
│ thread synchronization  │ memory ordering        │ data corruption         │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ Modifying string literal│ Literals in .rodata    │ SIGSEGV                 │
│ through char*           │ (read-only memory)     │                         │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ int for getchar return  │ EOF is -1, char may be │ Infinite loop on some   │
│                         │ unsigned — can't hold  │ platforms               │
│                         │ -1 value               │                         │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ Casting malloc result   │ In C89, hides missing  │ 64-bit pointer truncated│
│ in C (C89 specifically) │ #include <stdlib.h>    │ silently — crash        │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ Assuming int is 64-bit  │ int is 32-bit on all   │ Data loss on 64-bit     │
│ on 64-bit platform      │ modern platforms       │ values                  │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ assuming long is same   │ LP64 vs LLP64 ABI      │ Code breaks on Windows  │
│ size on all 64-bit OS   │ difference             │                         │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ VLA in C89 mode         │ VLAs are C99 feature   │ Compilation failure     │
│                         │                        │                         │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ Struct layout assumption│ Padding is             │ Buffer overflows,       │
│ without checking        │ compiler/arch dependent│ protocol errors         │
├─────────────────────────┼────────────────────────┼─────────────────────────┤
│ restrict with           │ Violates the no-alias  │ Undefined behavior,     │
│ overlapping pointers    │ contract               │ wrong results           │
└─────────────────────────┴────────────────────────┴─────────────────────────┘
```

---

# MISINFORMATION HALL OF FAME {#misinformation-hall-of-fame}

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    MYTHS vs REALITY                                         │
├───────────────────────────────────┬─────────────────────────────────────────┤
│ MYTH                              │ REALITY                                 │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "int is 4 bytes always"           │ int is at least 16 bits. It's 32-bit   │
│                                   │ by convention on modern CPUs but NOT   │
│                                   │ guaranteed by the standard.             │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "auto in C is like auto in C++"   │ C auto = automatic storage (default).  │
│                                   │ C++ auto = type deduction. DIFFERENT.  │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "register makes code faster"      │ Modern compilers ignore register.       │
│                                   │ Optimizers are smarter. May even hurt  │
│                                   │ optimization by preventing aliasing     │
│                                   │ analysis.                               │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "volatile makes code thread-safe" │ volatile has NO thread-safety           │
│                                   │ guarantees. Use _Atomic or mutexes.    │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "const means compile-time         │ const int n = 10; is NOT a compile-    │
│  constant in C"                   │ time constant in C. It's a read-only   │
│                                   │ variable. Use #define or enum.         │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "goto is always evil"             │ goto is the correct tool for error     │
│                                   │ handling and cleanup in C. Linux       │
│                                   │ kernel uses it thousands of times.     │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "signed integer overflow wraps    │ Signed overflow is UNDEFINED BEHAVIOR. │
│  around on overflow"              │ The compiler may eliminate overflow     │
│                                   │ checks! Only unsigned wraps (defined). │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "NULL is a keyword"               │ NULL is a MACRO (#define NULL ((void*)0│
│                                   │ or 0 or 0L). Not a keyword.           │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "inline guarantees inlining"      │ inline is a HINT. Compiler can ignore  │
│                                   │ it. Use __attribute__((always_inline)) │
│                                   │ to force it on GCC/Clang.             │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "sizeof is a function"            │ sizeof is an OPERATOR evaluated at     │
│                                   │ COMPILE TIME (except VLAs). It never  │
│                                   │ evaluates its operand.                 │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "char is always signed"           │ char signedness is IMPLEMENTATION-     │
│                                   │ DEFINED. ARM defaults to unsigned,     │
│                                   │ x86 defaults to signed.               │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "union type-punning is always UB" │ Reading a union member other than the  │
│                                   │ last-written IS UB in strict C99, but │
│                                   │ C11 and GCC/Clang explicitly allow it. │
│                                   │ Use memcpy for guaranteed behavior.    │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "extern is for global variables   │ extern declares external linkage for   │
│  only"                            │ both variables AND functions. Function │
│                                   │ declarations are implicitly extern.    │
├───────────────────────────────────┼─────────────────────────────────────────┤
│ "static local variables are       │ Static locals are NOT thread-safe.     │
│  thread-safe"                     │ Multiple threads can race on them.     │
│                                   │ Use _Thread_local or mutex protection. │
└───────────────────────────────────┴─────────────────────────────────────────┘
```

---

# MENTAL MODELS FOR EXPERT C THINKING {#mental-models}

## 1. The Lifetime Mental Model

> Every object has a lifetime. Think of it as a heartbeat — it starts at creation and stops at destruction. Any access outside the lifetime is undefined behavior.

```
Object Type     Starts             Ends
──────────────────────────────────────────────────────
auto (local)    Declaration        End of enclosing { }
static          Program start      Program end
heap (malloc)   malloc() call      free() call
thread_local    Thread start       Thread end
```

## 2. The Undefined Behavior Optimizer Model

> The compiler assumes UB never happens. If it can prove UB would occur on a code path, it may ELIMINATE that code path entirely.

```
if (ptr != NULL) {
    *ptr = 5;          ← If this executes, ptr != NULL (no UB)
    if (ptr == NULL) { ← DEAD CODE: compiler knows ptr != NULL here
        abort();       ← ELIMINATED by optimizer
    }
}
/* A real CVE (security vulnerability) pattern:
   Null check eliminated after use, leading to null dereference. */
```

## 3. The Type System Contract Model

> `const`, `volatile`, `restrict` are CONTRACTS between you and the compiler. Breaking a contract (casting away const, aliasing despite restrict) results in UB — the compiler will optimize based on the contract, not reality.

## 4. The Platform Portability Model

> When you write C, you're writing for an abstract machine. The concrete machine varies. Assume:
> - Integer sizes: use `<stdint.h>` types
> - Endianness: never assume; use `htonl/ntohl` for network data
> - Alignment: never assume packed layout; use `offsetof` and `sizeof`
> - Pointer size: never assume; use `intptr_t`/`uintptr_t`

## 5. The Translation Unit Mental Model

```
COMPILATION UNIT (one .c file + its includes):

file1.c ──► [compile] ──► file1.o (object file)
file2.c ──► [compile] ──► file2.o
file3.c ──► [compile] ──► file3.o

         [LINKER]
file1.o ─┐
file2.o ─┼──► executable
file3.o ─┘
libc.a  ─┘

Visibility rules:
  static: visible only within one .c file's object
  extern: the linker resolves cross-object references
  inline (non-static): must have one external definition for the linker
```

---

## Recommended Compilation Flags for Maximum Safety

```bash
# Development and learning — catch all problems:
gcc -std=c11 -Wall -Wextra -Wpedantic -Wshadow -Wconversion \
    -Wsign-compare -Wstrict-prototypes -Wmissing-prototypes \
    -fsanitize=address,undefined -fstack-protector-strong \
    -g -O0 your_file.c

# Production — optimized with safety:
gcc -std=c11 -Wall -Wextra -O2 -DNDEBUG \
    -fstack-protector-strong -D_FORTIFY_SOURCE=2 \
    your_file.c

# Flags explained:
# -std=c11          Force C11 standard
# -Wall             All common warnings
# -Wextra           Extra warnings
# -Wpedantic        Strict standard compliance
# -Wsign-compare    Warn on signed/unsigned comparison
# -fsanitize=...    Runtime UB and memory error detection
# -fstack-protector Stack overflow detection
# -D_FORTIFY_SOURCE Buffer overflow detection in library calls
```

---

*End of Guide — Total C Keywords Covered: All 44 (C89: 32, C99: +5, C11: +7) + C23 additions*

*This guide reflects behavior as of GCC 13, Clang 17, on Linux/x86-64 with glibc.*
