# Type Casting: A Complete In-Depth Guide
## C · Rust · Go

---

## Table of Contents

1. [What Is a Type and Why Casting Exists](#1-what-is-a-type-and-why-casting-exists)
2. [Memory Layout Fundamentals](#2-memory-layout-fundamentals)
3. [Type Casting in C](#3-type-casting-in-c)
   - 3.1 Implicit Conversion (Coercion)
   - 3.2 Explicit Cast Syntax
   - 3.3 Integer Promotions and Usual Arithmetic Conversions
   - 3.4 Signed ↔ Unsigned Conversions
   - 3.5 Integer ↔ Floating-Point Conversions
   - 3.6 Pointer Casts
   - 3.7 void* and Generic Pointers
   - 3.8 Function Pointer Casts
   - 3.9 Casting with unions (Type Punning)
   - 3.10 memcpy-based Type Punning
   - 3.11 Undefined Behaviour Landmines
   - 3.12 Practical Patterns and Anti-Patterns
4. [Type Casting in Rust](#4-type-casting-in-rust)
   - 4.1 The `as` Keyword
   - 4.2 From / Into Traits
   - 4.3 TryFrom / TryInto Traits
   - 4.4 Transmute
   - 4.5 Raw Pointer Casts
   - 4.6 Fat Pointer Casts
   - 4.7 Coercions
   - 4.8 Deref Coercions
   - 4.9 Unsizing Coercions
   - 4.10 Type Punning the Safe Way
   - 4.11 Casting in `unsafe` code
5. [Type Casting in Go](#5-type-casting-in-go)
   - 5.1 Explicit Type Conversion
   - 5.2 Numeric Conversions
   - 5.3 String ↔ Byte / Rune Slices
   - 5.4 Interface Conversions and Type Assertions
   - 5.5 Type Switches
   - 5.6 The `unsafe.Pointer` and `uintptr` Pipeline
   - 5.7 reflect-based Conversion
   - 5.8 Aliased Types vs Defined Types
6. [Side-by-Side Comparison Table](#6-side-by-side-comparison-table)
7. [Deep Dives: Internals and Edge Cases](#7-deep-dives-internals-and-edge-cases)
   - 7.1 IEEE 754 and float casts
   - 7.2 Two's Complement and Wrap-around
   - 7.3 Pointer Provenance
   - 7.4 Strict Aliasing
   - 7.5 Endianness
8. [Complete Code Examples](#8-complete-code-examples)
9. [Summary Mental Models](#9-summary-mental-models)

---

## 1. What Is a Type and Why Casting Exists

A **type** is a promise to the compiler (and CPU) about:

1. **How many bytes** a value occupies in memory.
2. **How those bytes are interpreted** (two's complement integer, IEEE 754 float, pointer, etc.).
3. **What operations are valid** on those bytes.

```
   Memory cell (8 bytes / 64-bit)
   ┌────────────────────────────────────────────────────────────────┐
   │  0100 0000  0101 1001  0000 0000  0000 0000  ...              │
   └────────────────────────────────────────────────────────────────┘
         │                         │                     │
    As uint64_t               As double            As pointer
    = 4625759167262932992     = 100.0             = (some address)
```

The **same bit pattern** means completely different things depending on the type lens applied. Casting is the act of **changing the lens**, sometimes also **changing the bit pattern itself** (value conversion), sometimes not (reinterpretation).

### Two Fundamentally Different Kinds of Cast

```
   ┌──────────────────────────────────────────────────────────────┐
   │  KIND 1: VALUE CONVERSION                                    │
   │                                                              │
   │   int  i = 300;                                              │
   │   float f = (float)i;   // bits change: 300 → 0x43960000    │
   │                                                              │
   │   Before: 0x0000012C  (integer 300)                          │
   │   After:  0x43960000  (IEEE 754 for 300.0f)                  │
   │                                                              │
   │   The VALUE is preserved (or approximated); bits differ.     │
   └──────────────────────────────────────────────────────────────┘

   ┌──────────────────────────────────────────────────────────────┐
   │  KIND 2: REINTERPRETATION (Type Punning)                     │
   │                                                              │
   │   float  f = 1.0f;   // bits: 0x3F800000                    │
   │   int*   p = (int*)&f;                                       │
   │   int    i = *p;     // i = 0x3F800000 = 1065353216         │
   │                                                              │
   │   The BIT PATTERN is unchanged; meaning flips.              │
   └──────────────────────────────────────────────────────────────┘
```

Understanding which kind every cast performs is the entire skill of safe casting.

---

## 2. Memory Layout Fundamentals

Before going per-language, internalize memory layout because every cast operates on it.

### Numeric Type Sizes (typical 64-bit LP64 / LLP64 systems)

```
   C Type          Rust Type       Go Type     Bytes   Bit Pattern Interpretation
   ─────────────   ─────────────   ─────────   ─────   ──────────────────────────
   char            i8              int8         1       Two's complement
   unsigned char   u8              uint8        1       Unsigned binary
   short           i16             int16        2       Two's complement
   unsigned short  u16             uint16       2       Unsigned binary
   int             i32             int32        4       Two's complement
   unsigned int    u32             uint32       4       Unsigned binary
   long (Linux64)  i64             int64        8       Two's complement
   long long       i64             int64        8       Two's complement
   unsigned long   u64             uint64       8       Unsigned binary
   float           f32             float32      4       IEEE 754 single
   double          f64             float64      8       IEEE 754 double
   pointer         *T / usize      uintptr      8*      Address (platform)
   _Bool           bool            bool         1       0 or 1
   size_t          usize           uint         8*      Platform-native unsigned
   ptrdiff_t       isize           int          8*      Platform-native signed
                                                * on 64-bit platforms
```

### Memory Diagrams for Numeric Types

```
   u8  / uint8  / unsigned char  (1 byte)
   ┌──────────┐
   │  7 ... 0 │   Range: 0..255
   └──────────┘

   i8  / int8  / signed char  (1 byte)
   ┌──────────┐
   │ S 6 ... 0│   S = sign bit.  Range: -128..127
   └──────────┘

   i32 / int32 / int  (4 bytes, little-endian in memory)
   Memory address →
   ┌──────────┬──────────┬──────────┬──────────┐
   │  byte 0  │  byte 1  │  byte 2  │  byte 3  │
   │ (LSB)    │          │          │  (MSB)   │
   └──────────┴──────────┴──────────┴──────────┘
   Value 0x01020304 stored little-endian:
   ┌──────────┬──────────┬──────────┬──────────┐
   │   0x04   │   0x03   │   0x02   │   0x01   │
   └──────────┴──────────┴──────────┴──────────┘

   f32 / float32 / float  (4 bytes, IEEE 754 single precision)
   ┌───┬────────┬───────────────────────┐
   │ S │  Exp  │       Mantissa        │
   │ 1 │  8bit │         23bit         │
   └───┴────────┴───────────────────────┘
   Value = (-1)^S * 2^(Exp-127) * 1.Mantissa

   f64 / float64 / double  (8 bytes, IEEE 754 double precision)
   ┌───┬───────────┬────────────────────────────────────────────┐
   │ S │    Exp    │                  Mantissa                  │
   │ 1 │   11bit   │                    52bit                   │
   └───┴───────────┴────────────────────────────────────────────┘
   Value = (-1)^S * 2^(Exp-1023) * 1.Mantissa
```

### Alignment and Padding

```
   struct Example {
       char  a;     // 1 byte  @ offset 0
       // 3 bytes padding
       int   b;     // 4 bytes @ offset 4
       char  c;     // 1 byte  @ offset 8
       // 7 bytes padding
       long  d;     // 8 bytes @ offset 16
   };               // total: 24 bytes

   Memory layout:
   Offset:  0    1    2    3    4    5    6    7    8    9   ...  16   ...
           ┌────┬────┬────┬────┬────┴────┴────┴────┬────┬───     ┌────────┐
           │ a  │PAD │PAD │PAD │         b          │ c  │...PAD  │   d    │
           └────┴────┴────┴────┴────────────────────┴────┴───     └────────┘

   Casting a pointer to this struct means understanding these offsets.
```

---

## 3. Type Casting in C

C has the most permissive and dangerous casting system. It trusts you completely.

### 3.1 Implicit Conversion (Coercion)

C silently converts types in many contexts without any cast syntax. These are **implicit conversions**.

```c
#include <stdio.h>

void implicit_demo(void) {
    // Integer promotion: smaller types → int before arithmetic
    char  a = 200;       // 0xC8 (unsigned-like pattern)
    char  b = 100;
    int   result = a + b; // a and b are PROMOTED to int before adding
                          // result = 300, NOT 44 (no truncation yet)

    // Assignment conversion: RHS converted to LHS type
    double d = 3;         // int 3 → double 3.0  (value-preserving)
    int    i = 3.9;       // double 3.9 → int 3  (truncation, not rounding!)

    // Function argument conversion (if prototype is visible)
    // printf("%d", 3.14);  // UB: wrong format spec, but no compiler cast
}
```

```
   Integer Promotion Rule (C11 §6.3.1.1):
   
   Every integer expression of rank < int is promoted to int
   (or unsigned int if int cannot hold all values of that type)
   
   Rank hierarchy (low → high):
   _Bool < char < short < int < long < long long
   
   Example:
   char c1 = 0xFF;   // -1 if signed, 255 if unsigned
   char c2 = 0x01;
   int  r  = c1 + c2;
   
   Step-by-step:
   ┌────────────────────────────────────────────────┐
   │  c1 (char, 1 byte)   →  (int, 4 bytes)         │
   │  0xFF                →  0xFFFFFFFF (-1 signed) │
   │                      OR 0x000000FF (255 uint)  │
   │                                                │
   │  c2 (char, 1 byte)   →  (int, 4 bytes)         │
   │  0x01                →  0x00000001             │
   │                                                │
   │  Add in int domain:  -1 + 1 = 0                │
   │               OR:   255 + 1 = 256              │
   └────────────────────────────────────────────────┘
   DANGEROUS if you assume char is unsigned everywhere!
```

### 3.2 Explicit Cast Syntax

```c
// Syntax: (target_type) expression

int   i  = (int)   3.9;      // double → int   : value becomes 3
float f  = (float) 1000000;  // int → float    : may lose precision
char  c  = (char)  300;      // int → char     : truncation to 8 bits
void* vp = (void*) &i;       // int* → void*   : pointer widening
int*  ip = (int*)  vp;       // void* → int*   : pointer narrowing
```

### 3.3 Integer Promotions and Usual Arithmetic Conversions (UAC)

The UAC rules govern what happens when two different numeric types appear in a binary expression.

```
   Usual Arithmetic Conversions (C11 §6.3.1.8)
   
   In any binary arithmetic/comparison expression (a OP b):
   
   Step 1: Apply integer promotions to each operand independently
   Step 2: If either is long double  → both become long double
   Step 3: If either is double       → both become double
   Step 4: If either is float        → both become float
   Step 5: Both are integer. If same type → done.
   Step 6: If same signedness        → use higher rank
   Step 7: If unsigned has >= rank   → both become unsigned type
   Step 8: If signed can hold all values of unsigned → use signed type
   Step 9: Otherwise → both become unsigned version of signed type

   Gotcha example:
   int     i  = -1;
   unsigned u  = 1;
   if (i < u)   // You'd expect true, but...
       printf("negative");
   else
       printf("positive");   // ← This prints! Because:
   // i is converted to unsigned: (unsigned)(-1) = 4294967295 > 1
```

```
   Conversion chain diagram:

   Expression: (short)a + (unsigned char)b + (long)c

   ┌──────────┐   promote   ┌──────────┐
   │  short a │ ──────────► │   int    │
   └──────────┘             └────┬─────┘
                                 │  UAC with...
   ┌──────────────┐  promote  ┌──┴───────┐
   │ unsigned char│ ─────────►│   int    │──► int + int = int  (temp1)
   └──────────────┘           └──────────┘
                                                │
   ┌──────────┐                                 │  UAC: int vs long
   │  long c  │                                 │  → long (higher rank, same sign)
   └──────────┘                                 ▼
                                            temp1 → long
                                            long + long = long   (result)
```

### 3.4 Signed ↔ Unsigned Conversions

This is a rich source of bugs. You must know the exact rules.

```c
#include <stdio.h>
#include <stdint.h>

void signed_unsigned(void) {
    // CASE 1: Signed positive → unsigned (always safe in range)
    int      i  = 42;
    unsigned u  = (unsigned)i;    // u = 42, identical bit pattern
    printf("%u\n", u);            // 42

    // CASE 2: Signed negative → unsigned
    int      neg = -1;
    unsigned un  = (unsigned)neg; // modular reduction: -1 + 2^32 = 4294967295
    printf("%u\n", un);           // 4294967295 (0xFFFFFFFF)

    // CASE 3: Unsigned → signed (value fits)
    uint32_t big = 100;
    int32_t  s   = (int32_t)big;  // s = 100, safe
    printf("%d\n", s);

    // CASE 4: Unsigned → signed (value DOESN'T fit)
    uint32_t huge = 3000000000U;  // > INT32_MAX (2147483647)
    int32_t  bad  = (int32_t)huge; // IMPLEMENTATION-DEFINED behavior in C
    // On typical 2's complement: huge - 2^32 = 3000000000 - 4294967296 = -1294967296
    printf("%d\n", bad);          // -1294967296 (2's complement machines)
}
```

```
   Two's Complement Wheel — 4-bit example:
   
        0000 (0)
      1111   0001
    (−1/15)  (1)
    1110       0010
   (−2/14)    (2)
    1101       0011
   (−3/13)    (3)
     1100   0100
    (−4/12)  (4)
      1011   0101
    (−5/11)  (5)
        1010   0110
       (−6/10) (6)
         1001 0111
         (−7)  (7)
           1000
          (−8)

   Signed interpretation: top half of circle is negative
   Unsigned interpretation: all values are positive (0..15)
   
   Cast signed → unsigned: walk the circle counter-clockwise mod 16
   Cast unsigned → signed: if value > 7, it becomes negative in signed view
```

```
   Bit pattern of -1 in various widths (two's complement):

   int8_t:    1111 1111              = 0xFF
   int16_t:   1111 1111  1111 1111  = 0xFFFF
   int32_t:   0xFFFF FFFF
   int64_t:   0xFFFF FFFF FFFF FFFF

   uint8_t  u = (uint8_t)(-1);  → 0xFF = 255
   uint16_t u = (uint16_t)(-1); → 0xFFFF = 65535
   uint32_t u = (uint32_t)(-1); → 0xFFFFFFFF = 4294967295
   uint64_t u = (uint64_t)(-1); → 0xFFFFFFFFFFFFFFFF = 18446744073709551615
```

### 3.5 Integer ↔ Floating-Point Conversions

```c
#include <stdio.h>
#include <math.h>

void float_int_conversions(void) {
    // int → float (may lose precision for large integers)
    int   big = 16777217;       // 2^24 + 1
    float f   = (float)big;     // float has 23-bit mantissa = 24 bits of precision
    printf("%.1f\n", f);        // 16777216.0 — the +1 is LOST!
    
    // int → double (safe for all 32-bit integers; double has 52-bit mantissa)
    double d = (double)big;
    printf("%.1f\n", d);        // 16777217.0 — exact

    // float → int: truncation towards zero (not rounding)
    printf("%d\n", (int)3.9f);  // 3
    printf("%d\n", (int)(-3.9f)); // -3  (truncated towards zero)

    // Overflow: float value doesn't fit in int → UNDEFINED BEHAVIOUR
    float huge = 1e30f;
    // int bad = (int)huge;  // UB! Do NOT do this.

    // Safe: check before casting
    if (huge < (float)INT_MAX && huge > (float)INT_MIN)
        printf("safe to cast\n");
    else
        printf("would overflow\n");
        
    // NaN / Inf → int is also UB in C
    float nan_val = 0.0f / 0.0f;
    // int i = (int)nan_val;  // UB!
}
```

```
   float (IEEE 754 single) precision diagram:

   Integer range safely representable in float:
   ─────────────────────────────────────────────
   float mantissa = 23 explicit bits + 1 implicit = 24 bits
   Therefore integers -2^24 to 2^24 (±16,777,216) are exact
   Integers outside this range may be rounded to nearest representable value

   ◄──────────────── All floats ──────────────────►
   ◄── Exact ints ──►                              
   -16M             0              +16M     +1e38

   double mantissa = 52 explicit bits + 1 implicit = 53 bits
   Integers -2^53 to 2^53 (±9,007,199,254,740,992) are exact in double
```

### 3.6 Pointer Casts

Pointer casts are where C becomes very powerful and very dangerous.

```c
#include <stdio.h>
#include <stdint.h>

void pointer_cast_demo(void) {
    int   val  = 0x41424344;  // bytes: 0x44, 0x43, 0x42, 0x41 (little-endian)
    int*  ip   = &val;

    // Cast to char* to inspect bytes
    char* cp = (char*)ip;
    printf("Bytes: %02X %02X %02X %02X\n",
           (unsigned char)cp[0],
           (unsigned char)cp[1],
           (unsigned char)cp[2],
           (unsigned char)cp[3]);
    // On little-endian: 44 43 42 41

    // Cast to unsigned char* is the CANONICAL way to inspect bytes
    unsigned char* bp = (unsigned char*)ip;

    // Cast pointer to integer (for arithmetic, logging, etc.)
    uintptr_t addr = (uintptr_t)ip;
    printf("Address: 0x%lX\n", (unsigned long)addr);
    
    // Cast integer back to pointer — ONLY SAFE if addr came from a pointer
    int* ip2 = (int*)addr;
    printf("Round-trip value: %d\n", *ip2);  // 0x41424344 = 1094861636
}
```

```
   Pointer size and representation:

   32-bit platform:
   ┌──────────────────────────────────┐
   │          4-byte pointer          │
   │  0x0804A020  (example address)   │
   └──────────────────────────────────┘

   64-bit platform:
   ┌─────────────────────────────────────────────────────────────────┐
   │                       8-byte pointer                            │
   │  0x00007FFE A1B2C3D4  (example stack address)                   │
   └─────────────────────────────────────────────────────────────────┘
   Note: upper 16 bits are often 0 on current x86_64 (48-bit VA)
   ARM64 uses pointer authentication codes in upper bits (PAC)

   Pointer Cast Width Compatibility:
   ┌──────────────────────────────────────────────────────────────┐
   │ int*    → char*    : Legal (byte access, useful)             │
   │ int*    → short*   : Legal syntax, but alignment UB risk     │
   │ int*    → float*   : Legal syntax, strict-aliasing UB        │
   │ int*    → void*    : Legal, always (information loss: size)  │
   │ void*   → int*     : Legal if original was int*              │
   │ int*    → int**    : Legal syntax, meaningless semantically  │
   │ int(*)()→ void*    : IMPLEMENTATION-DEFINED (not guaranteed) │
   └──────────────────────────────────────────────────────────────┘
```

### 3.7 `void*` and Generic Pointers

```c
#include <stdlib.h>
#include <string.h>

// void* is the C generic pointer — no type information, no size information
void* my_memdup(const void* src, size_t n) {
    void* dst = malloc(n);
    if (!dst) return NULL;
    memcpy(dst, src, n);
    return dst;  // caller must cast to the right type
}

void void_ptr_demo(void) {
    int arr[3] = {1, 2, 3};
    
    // void* assignment from any pointer: implicit (no cast needed in C, but needed in C++)
    void* vp = arr;            // int* → void*: implicit in C
    int*  ip = (int*)vp;       // void* → int*: explicit cast required
    
    // Dereferencing void* is NOT allowed (no size info)
    // *vp = 5;  // ERROR: incomplete type

    // malloc returns void*; in C you cast it explicitly for clarity
    int* heap_arr = (int*)malloc(3 * sizeof(int));
    // ...
    free(heap_arr);  // free() also takes void*
}
```

### 3.8 Function Pointer Casts

```c
#include <stdio.h>

void hello(void) { printf("hello\n"); }
int  add(int a, int b) { return a + b; }

void fn_ptr_cast_demo(void) {
    // A function pointer's type encodes its signature
    void (*fp1)(void)     = hello;
    int  (*fp2)(int, int) = add;

    // Casting between incompatible function pointer types and calling through them is UB
    // This is for illustration only:
    void (*fp3)(void) = (void(*)(void))fp2;  // DANGEROUS: do NOT call fp3!

    // Only safe: cast function ptr → void* (implementation-defined, not guaranteed portable)
    // POSIX guarantees this; C standard does not
    // void* vfp = (void*)fp1;  // POSIX only

    // Safe round-trip: void(*)(void) → void(*)(void)
    void (*fp4)(void) = hello;
    void (*fp5)(void) = (void(*)(void))fp4; // safe: same type
    fp5();  // "hello"
}
```

### 3.9 Casting with `union` (Type Punning)

```c
#include <stdio.h>
#include <stdint.h>

// Inspecting a float's bit representation using union (defined behavior in C99/C11)
union float_int {
    float    f;
    uint32_t u;
};

void union_pun_demo(void) {
    union float_int x;
    x.f = 1.0f;
    printf("1.0f bit pattern: 0x%08X\n", x.u); // 0x3F800000

    x.f = -0.0f;
    printf("-0.0f bit pattern: 0x%08X\n", x.u); // 0x80000000

    x.f = 1.0f / 0.0f; // +Inf
    printf("+Inf bit pattern: 0x%08X\n", x.u); // 0x7F800000

    x.u = 0x7FC00000; // quiet NaN
    printf("NaN value: %f\n", x.f);
}
```

```
   Union memory layout:

   union float_int {
       float    f;  // 4 bytes
       uint32_t u;  // 4 bytes
   };

   ┌──────────────────────────────────────────────────────┐
   │  Shared storage — same 4 bytes                       │
   │  ┌──────────┬──────────┬──────────┬──────────┐       │
   │  │  byte 0  │  byte 1  │  byte 2  │  byte 3  │       │
   │  └──────────┴──────────┴──────────┴──────────┘       │
   │       ▲                                               │
   │  read as f → IEEE 754 float interpretation           │
   │  read as u → unsigned 32-bit integer interpretation  │
   └──────────────────────────────────────────────────────┘

   C11 standard: writing one member and reading another is defined behavior
   (unlike C++, where it's implementation-defined but widely supported)
```

### 3.10 `memcpy`-based Type Punning

The strictly portable and always-defined-behavior approach:

```c
#include <string.h>
#include <stdint.h>
#include <stdio.h>

float bits_to_float(uint32_t bits) {
    float result;
    memcpy(&result, &bits, sizeof(result)); // Defined behavior: copy bytes
    return result;
}

uint32_t float_to_bits(float f) {
    uint32_t bits;
    memcpy(&bits, &f, sizeof(bits));
    return bits;
}

void memcpy_pun_demo(void) {
    uint32_t bits  = 0x3F800000;
    float    f     = bits_to_float(bits);
    printf("0x3F800000 as float: %f\n", f);  // 1.000000

    float    pi    = 3.14159265f;
    uint32_t pbits = float_to_bits(pi);
    printf("pi bits: 0x%08X\n", pbits);       // 0x40490FDB
}
```

### 3.11 Undefined Behaviour Landmines

```c
// ─── UB #1: Strict Aliasing Violation ────────────────────────────
float f = 3.14f;
int*  p = (int*)&f;
int   i = *p;   // UB! compiler may assume int* and float* don't alias
                 // Use union or memcpy instead

// ─── UB #2: Misaligned Access ─────────────────────────────────────
char buf[8] = {0};
int* ip = (int*)(buf + 1); // int requires 4-byte alignment; buf+1 is misaligned
int  v  = *ip;             // UB! (and bus error on strict-alignment architectures)

// ─── UB #3: Float → Int Overflow ──────────────────────────────────
float huge = 1e30f;
int   bad  = (int)huge;    // UB if value doesn't fit in int

// ─── UB #4: Integer Overflow (Signed) ─────────────────────────────
int  max  = INT_MAX;
int  over = max + 1;       // UB! signed overflow. Use unsigned arithmetic or __builtin_add_overflow
```

```
   Strict Aliasing Rule (C11 §6.5):
   
   An object shall be accessed only through an lvalue expression
   that is compatible with:
     - the effective type of the object
     - a qualified version of the effective type
     - a signed/unsigned version of the effective type
     - an aggregate containing the above
     - char or unsigned char (ALWAYS allowed for byte access)
   
   The compiler USES this rule for optimization:
   
   void process(int* a, float* b) {
       *b = 1.0f;
       *a = 2;
       return *b;  // compiler may return 1.0f WITHOUT re-reading memory
   }               // because int* and float* cannot alias (strict aliasing)
   
   If you cast (int*)&some_float, the compiler may "optimize away" your read.
   Use -fno-strict-aliasing (GCC/Clang) to disable, or use union/memcpy.
```

### 3.12 Practical Patterns and Anti-Patterns

```c
// ✅ GOOD: Inspect bytes of any type portably
void print_bytes(const void* ptr, size_t len) {
    const unsigned char* p = (const unsigned char*)ptr;
    for (size_t i = 0; i < len; i++)
        printf("%02X ", p[i]);
    printf("\n");
}

// ✅ GOOD: Generic swap
void swap(void* a, void* b, size_t size) {
    unsigned char tmp[256]; // or use VLA / alloca
    memcpy(tmp, a, size);
    memcpy(a,   b, size);
    memcpy(b, tmp, size);
}

// ✅ GOOD: Type-safe container using macros
#define CAST_PTR(type, ptr) ((type*)(ptr))

// ❌ BAD: Classic sign-extension bug
char  arr[4] = {0x80, 0x00, 0x00, 0x00};
int   val    = arr[0]; // if char is signed: val = -128, not 128!
// FIX:
int   val2   = (unsigned char)arr[0]; // val2 = 128

// ❌ BAD: Comparing signed and unsigned
int      si = -1;
unsigned ui = 1;
if (si < ui) { /* never executes due to UAC */ }
// FIX: cast explicitly
if (si < (int)ui)  { /* now correct */ }
if ((long long)si < (long long)ui) { /* also correct */ }
```

---

## 4. Type Casting in Rust

Rust has a layered system of explicit casts, fallible conversions, and strictly-scoped unsafe operations. There is no implicit numeric conversion between distinct types.

### 4.1 The `as` Keyword

`as` is Rust's primitive cast. It works between numeric primitives and pointers. It is always explicit, always compiles without `unsafe`, but may silently truncate or change sign.

```rust
fn as_keyword_demo() {
    // ── Widening: always safe, value preserved ──────────────────
    let i: i32 = 42;
    let l: i64 = i as i64;   // zero-extension for positive, sign-extension for negative

    let u: u8  = 200;
    let w: u32 = u as u32;   // zero-extension: 200u32

    // ── Narrowing: truncates high bits ──────────────────────────
    let big: u32 = 0x12345678;
    let small: u8 = big as u8;   // takes lowest 8 bits: 0x78 = 120
    println!("{}", small);        // 120

    let n: i32 = 300;
    let c: u8  = n as u8;    // 300 mod 256 = 44
    println!("{}", c);        // 44

    // ── Signed ↔ Unsigned ───────────────────────────────────────
    let neg: i32 = -1;
    let u32_val: u32 = neg as u32;  // reinterpret bits: 0xFFFFFFFF = 4294967295
    println!("{}", u32_val);         // 4294967295

    let big_u: u32 = 4294967295;
    let signed: i32 = big_u as i32; // reinterpret bits: 0xFFFFFFFF = -1
    println!("{}", signed);          // -1

    // ── Float → Int ─────────────────────────────────────────────
    let f: f64 = 3.99;
    let i: i32 = f as i32;   // truncation towards zero: 3 (not 4)
    println!("{}", i);         // 3

    let neg_f: f64 = -3.99;
    let neg_i: i32 = neg_f as i32; // -3
    println!("{}", neg_i);

    // ── Float → Int saturation (Rust 1.45+) ─────────────────────
    // Previously: huge float → int was UB (like C). Now saturates!
    let huge: f64 = 1e18_f64;
    let sat: i32  = huge as i32;   // i32::MAX = 2147483647 (saturated)
    println!("{}", sat);

    let nan: f64 = f64::NAN;
    let n_val: i32 = nan as i32;   // 0  (NaN → 0 since Rust 1.45)
    println!("{}", n_val);

    // ── Int → Float ─────────────────────────────────────────────
    let precise: i64 = 16_777_217; // 2^24 + 1
    let f32_val: f32 = precise as f32; // precision loss: rounds to 16777216.0
    println!("{}", f32_val);

    let f64_val: f64 = precise as f64; // exact (f64 has 53-bit mantissa)
    println!("{}", f64_val);

    // ── Pointer ↔ Integer ───────────────────────────────────────
    let x = 42i32;
    let ptr: *const i32 = &x as *const i32;
    let addr: usize = ptr as usize;
    let ptr2: *const i32 = addr as *const i32;
    // Reading through ptr2 requires unsafe
}
```

```
   Rust `as` truncation diagram:

   u32 value: 0x12345678
   ┌──────┬──────┬──────┬──────┐
   │  12  │  34  │  56  │  78  │    (hex bytes, big-endian display)
   └──────┴──────┴──────┴──────┘
                             ▲
   as u8 → keep only this byte: 0x78 = 120

   as u16 → keep bottom two bytes: 0x5678 = 22136

   as i8  → same bits: 0x78 = 120 if < 128, else reinterpret as negative
```

### 4.2 `From` / `Into` Traits

These are **infallible** conversions that are guaranteed to preserve the value exactly. They are the idiomatic way to convert types in safe Rust when the conversion cannot fail.

```rust
fn from_into_demo() {
    // From is the primary trait; Into is derived automatically
    let i: i32 = i32::from(10i16);     // i16 → i32: always fits
    let j: i64 = i64::from(i);          // i32 → i64: always fits
    let f: f64 = f64::from(42i32);      // i32 → f64: exact (all i32 fit in f64)

    // Into (ergonomic: let the compiler figure out target type)
    let x: i64 = 10i32.into();          // same as i64::from(10i32)
    let y: f64 = 10i32.into();

    // String conversions
    let s: String = String::from("hello");
    let s2: String = "world".to_string(); // &str → String

    // From in function signatures — very common pattern:
    fn takes_string(s: impl Into<String>) {
        let owned: String = s.into();
        println!("{}", owned);
    }
    takes_string("literal");    // &str works
    takes_string(String::from("owned"));  // String works

    // Why From/Into and NOT as?
    // From/Into are ONLY implemented for lossless conversions.
    // i8::from(300i32) would be a compile error (no such impl).
    // (300i32 as i8) silently truncates to 44.
}
```

```
   From/Into Conversion Table (numeric, all lossless):

   Source ──────────────────────────────────────────► Target
   
   i8   → i16, i32, i64, i128, isize, f32, f64
   i16  → i32, i64, i128, isize, f32, f64
   i32  → i64, i128, f64
   i64  → i128
   u8   → i16, i32, i64, i128, isize, u16, u32, u64, u128, usize, f32, f64
   u16  → i32, i64, i128, u32, u64, u128, usize, f32, f64
   u32  → i64, i128, u64, u128, f64
   u64  → i128, u128
   f32  → f64

   NOT in From: i32 → i8  (can lose data)
                u16 → i16 (same, can overflow)
                i32 → f32 (precision loss possible)
   Use TryFrom or `as` for those.
```

### 4.3 `TryFrom` / `TryInto` Traits

For conversions that might fail at runtime:

```rust
use std::convert::TryFrom;
use std::convert::TryInto;

fn try_from_demo() {
    // i32 → i8: might not fit
    let big: i32 = 300;
    let result: Result<i8, _> = i8::try_from(big);
    match result {
        Ok(small) => println!("Fits: {}", small),
        Err(e)    => println!("Doesn't fit: {}", e), // ← this branch taken
    }

    // Successful case
    let small: i32 = 42;
    let r: Result<i8, _> = i8::try_from(small);
    println!("{}", r.unwrap()); // 42

    // TryInto ergonomics
    let n: i32 = 127;
    let m: Result<i8, _> = n.try_into();
    println!("{:?}", m); // Ok(127)

    // u8 → char (only Some if valid ASCII)
    let byte: u8 = 65;
    let ch = char::from(byte); // infallible: any u8 is a valid char (Latin-1 extended)
    println!("{}", ch); // 'A'

    // u32 → char: fallible (not all u32 are valid Unicode codepoints)
    let code: u32 = 0x1F600;
    let emoji: Result<char, _> = char::try_from(code);
    println!("{:?}", emoji); // Ok('😀')

    let invalid: u32 = 0xD800; // surrogate half — not valid Unicode
    let bad: Result<char, _> = char::try_from(invalid);
    println!("{:?}", bad); // Err(...)
}
```

### 4.4 `transmute`

`transmute` is the nuclear option — it reinterprets the raw bits of a value as a different type. It requires `unsafe` and imposes zero runtime cost.

```rust
use std::mem;

unsafe fn transmute_demo() {
    // Inspect float bits (like C union trick)
    let f: f32 = 1.0f32;
    let u: u32 = mem::transmute::<f32, u32>(f);
    println!("1.0f32 bits: 0x{:08X}", u); // 0x3F800000

    // Reverse
    let bits: u32 = 0x3F800000u32;
    let back: f32 = mem::transmute::<u32, f32>(bits);
    println!("0x3F800000 as f32: {}", back); // 1

    // transmute a slice pointer to inspect the internals of a fat pointer
    let v: Vec<i32> = vec![1, 2, 3];
    let slice: &[i32] = &v;

    // Fat pointer: (data_ptr, length)
    let (ptr, len): (*const i32, usize) = mem::transmute(slice);
    println!("ptr={:p}, len={}", ptr, len); // ptr=0x..., len=3

    // DANGER: sizes must match exactly!
    // mem::transmute::<u8, u32>(0u8); // compile error: size mismatch
    // mem::transmute::<f32, u64>(0f32); // compile error: size mismatch (4 vs 8)

    // transmute lifetime (extremely dangerous — can create dangling references)
    // let r: &'static str = mem::transmute::<&str, &'static str>("hello");
    // Only safe if you know the data truly lives that long!
}
```

```
   transmute memory model:

   Before transmute:
   ┌──────────────────────────────────┐
   │  f32 value: 1.0f                 │
   │  Bit pattern: 0x3F800000         │
   │  ┌──────┬──────┬──────┬──────┐  │
   │  │  3F  │  80  │  00  │  00  │  │
   │  └──────┴──────┴──────┴──────┘  │
   └──────────────────────────────────┘
           │  (no bytes change, no CPU instruction)
           ▼
   After transmute:
   ┌──────────────────────────────────┐
   │  u32 value: 0x3F800000           │
   │  = 1065353216 decimal            │
   │  ┌──────┬──────┬──────┬──────┐  │
   │  │  3F  │  80  │  00  │  00  │  │
   │  └──────┴──────┴──────┴──────┘  │
   └──────────────────────────────────┘

   transmute is compile-time type relabeling.
   Zero CPU instructions in release mode.
   
   Preconditions (YOU must guarantee):
   1. sizeof(From) == sizeof(To)  ← compiler checks this
   2. The bit pattern must be valid for To  ← YOU must guarantee
   3. No lifetime confusion  ← YOU must guarantee
   4. No alignment violation  ← YOU must guarantee
```

### 4.5 Raw Pointer Casts

```rust
fn raw_pointer_casts() {
    let mut x: i32 = 42;

    // Reference → raw pointer (safe context)
    let raw_const: *const i32 = &x as *const i32;
    let raw_mut:   *mut   i32 = &mut x as *mut i32;

    // *const → *mut cast (safe syntactically, dangerous semantically)
    let raw_mut2: *mut i32 = raw_const as *mut i32;
    // Writing through raw_mut2 while raw_const aliases it would be UB

    // Pointer to different type (like C's void* dance)
    let raw_i32: *const i32 = &x;
    let raw_u8:  *const u8  = raw_i32 as *const u8; // inspect bytes

    unsafe {
        println!("First byte: 0x{:02X}", *raw_u8); // little-endian LSB of 42
    }

    // *const T → *const U (reinterpret pointer)
    let f: f32 = 1.0f32;
    let fp: *const f32 = &f;
    let ip: *const u32 = fp as *const u32;
    unsafe {
        println!("f32 1.0 bits: 0x{:08X}", *ip); // 0x3F800000
    }

    // Pointer ↔ usize (for arithmetic)
    let addr: usize = raw_const as usize;
    let back: *const i32 = addr as *const i32;
    unsafe { println!("{}", *back); } // 42
}
```

```
   Raw pointer fat vs thin:

   Thin pointer (*const i32):
   ┌───────────────────────────────────┐
   │          address (8 bytes)        │
   └───────────────────────────────────┘

   Fat pointer (*const [i32]) — slice pointer:
   ┌───────────────────────────────────┬───────────────────┐
   │          address (8 bytes)        │   length (usize)  │
   └───────────────────────────────────┴───────────────────┘

   Fat pointer (*const dyn Trait) — trait object:
   ┌───────────────────────────────────┬───────────────────┐
   │          data address (8 bytes)   │  vtable ptr (8B)  │
   └───────────────────────────────────┴───────────────────┘
   
   vtable layout:
   ┌──────────────┬──────────────┬──────────────┬──────────────┐
   │  drop fn ptr │  size        │  alignment   │  method_1    │
   ├──────────────┼──────────────┼──────────────┼──────────────┤
   │  method_2    │  method_3    │   ...        │              │
   └──────────────┴──────────────┴──────────────┴──────────────┘
```

### 4.6 Fat Pointer Casts

```rust
fn fat_pointer_casts() {
    let v: Vec<i32> = vec![1, 2, 3, 4, 5];
    let slice: &[i32] = &v;

    // &[i32] is a fat pointer: (ptr, len)
    // Cannot directly cast fat pointer to *const i32 (would lose length)
    let thin: *const i32 = slice.as_ptr(); // explicit: take just the data ptr

    // Trait object — another fat pointer
    trait Greet { fn greet(&self); }
    struct Dog;
    impl Greet for Dog {
        fn greet(&self) { println!("Woof!"); }
    }

    let dog = Dog;
    let dyn_ref: &dyn Greet = &dog; // fat pointer: (ptr to Dog, ptr to vtable)

    // Transmute a &dyn Trait to inspect the fat pointer components (unsafe)
    let (data_ptr, vtable_ptr): (*const (), *const ()) =
        unsafe { std::mem::transmute(dyn_ref) };
    println!("data={:p}, vtable={:p}", data_ptr, vtable_ptr);

    // Casting &[i32] to *const [i32] preserves fat pointer
    let fat_raw: *const [i32] = slice as *const [i32];
    println!("len through fat raw: {}", unsafe { (*fat_raw).len() });
}
```

### 4.7 Coercions

Rust performs several **implicit coercions** without any syntax — these are not casts but type system-level automatic conversions.

```rust
fn coercion_demo() {
    // 1. &mut T → &T (deref coercion: mutable ref to shared ref)
    let mut x = 5;
    let r: &i32 = &mut x;  // automatic coercion

    // 2. *mut T → *const T
    let mut y = 10;
    let mp: *mut i32 = &mut y;
    let cp: *const i32 = mp;  // automatic coercion

    // 3. &T → *const T
    let z = 42i32;
    let rp: *const i32 = &z; // automatic coercion

    // 4. Array → slice (&[T; N] → &[T])
    let arr: [i32; 5] = [1, 2, 3, 4, 5];
    let sl: &[i32] = &arr;  // &[i32; 5] automatically coerces to &[i32]

    // 5. &String → &str
    let owned = String::from("hello");
    let borrow: &str = &owned;  // coercion via Deref<Target=str>

    // 6. &Vec<T> → &[T]
    let v: Vec<i32> = vec![1, 2];
    let s: &[i32] = &v; // coercion via Deref<Target=[T]>

    // 7. Closure → function pointer (if no captures)
    let add = |a: i32, b: i32| a + b;
    let fp: fn(i32, i32) -> i32 = add; // coercion
    println!("{}", fp(3, 4)); // 7
}
```

```
   Coercion Site Types:

   Coercions happen at:
   ┌─────────────────────────────────────────────────────────┐
   │ 1. let statements:  let x: &[i32] = &arr;              │
   │ 2. Function args:   fn foo(s: &str) { ... } foo(&owned) │
   │ 3. Return values:   fn bar() -> &[i32] { &arr }         │
   │ 4. Struct fields:   Struct { field: &str }              │
   │ 5. Array elements:  let arr: [&str; 2] = [&owned, "s"] │
   └─────────────────────────────────────────────────────────┘
```

### 4.8 Deref Coercions

```rust
use std::ops::Deref;

// Deref coercion chain:
// String → str  (String implements Deref<Target=str>)
// Vec<T> → [T]  (Vec<T> implements Deref<Target=[T]>)
// Box<T> → T    (Box<T> implements Deref<Target=T>)
// Rc<T>  → T
// Arc<T> → T

fn deref_coerce_demo() {
    // String → &str coercion depth
    let s: String = String::from("hello world");
    
    // Direct coercion (one level)
    let str_ref: &str = &s;
    
    // Works in function calls automatically
    fn print_str(s: &str) { println!("{}", s); }
    print_str(&s); // coercion: &String → &str

    // Box<String> → &str (two deref hops: Box→String→str)
    let boxed: Box<String> = Box::new(String::from("boxed"));
    print_str(&boxed); // &Box<String> → &String → &str (auto multi-level)

    // Manual deref for clarity
    let manual: &str = &*s;  // *s gives String backing, then & gives &str
}
```

```
   Deref coercion chain visualization:

   &Box<String>
        │
        │  *Box<String> → String   (Box::deref)
        ▼
   &String
        │
        │  *String → str           (String::deref)
        ▼
   &str  ← compiler uses this for fn(s: &str)

   The compiler inserts `*` operations automatically at coercion sites.
```

### 4.9 Unsizing Coercions

```rust
fn unsizing_demo() {
    // Sized → Unsized coercions
    
    // [T; N] → [T]
    let arr: [i32; 3] = [1, 2, 3];
    let slice: &[i32] = &arr;    // &[i32;3] unsizes to &[i32]

    // T → dyn Trait
    struct Cat;
    trait Sound { fn sound(&self) -> &str; }
    impl Sound for Cat { fn sound(&self) -> &str { "meow" } }

    let cat = Cat;
    let dyn_sound: &dyn Sound = &cat;  // &Cat unsizes to &dyn Sound
    println!("{}", dyn_sound.sound()); // "meow"

    // Box unsizing
    let boxed_cat: Box<Cat>       = Box::new(Cat);
    let boxed_dyn: Box<dyn Sound> = boxed_cat; // Box<Cat> unsizes to Box<dyn Sound>
}
```

### 4.10 Type Punning the Safe Way in Rust

```rust
use std::mem;

fn safe_punning() {
    // Option 1: bytemuck crate (best for production code)
    // bytemuck::cast::<f32, u32>(1.0f32)  — safe, checks alignment and size

    // Option 2: transmute (unsafe, but always zero-cost)
    let f: f32 = std::f32::consts::PI;
    let bits: u32 = unsafe { mem::transmute::<f32, u32>(f) };
    println!("pi bits: 0x{:08X}", bits); // 0x40490FDB

    // Option 3: to_bits() / from_bits() — safe, defined, idiomatic
    let f2: f32 = std::f32::consts::PI;
    let bits2: u32 = f2.to_bits();         // safe!
    let back: f32  = f32::from_bits(bits2); // safe!
    println!("{}", back); // 3.1415927

    // This is the PREFERRED approach. Compiler optimizes to same code as transmute.

    // f64 equivalents
    let d: f64 = std::f64::consts::E;
    let dbits: u64 = d.to_bits();
    let dback: f64 = f64::from_bits(dbits);
    println!("{}", dback); // 2.718281828459045
}
```

### 4.11 Casting in `unsafe` Code

```rust
unsafe fn unsafe_cast_patterns() {
    // Pattern: raw allocation and typed access
    let layout = std::alloc::Layout::from_size_align(16, 8).unwrap();
    let raw: *mut u8 = std::alloc::alloc(layout);

    // Treat raw memory as a specific struct
    #[repr(C)]
    struct Header { version: u32, length: u32 }
    let hdr: *mut Header = raw as *mut Header;
    (*hdr).version = 1;
    (*hdr).length  = 16;

    // Access bytes that come after the header
    let data: *mut u8 = (hdr as *mut u8).add(8); // pointer arithmetic
    *data = 0xFF;

    std::alloc::dealloc(raw, layout);

    // Pattern: slice from raw parts
    let v: Vec<i32> = vec![10, 20, 30];
    let ptr:  *const i32 = v.as_ptr();
    let len:  usize      = v.len();
    let slice: &[i32] = std::slice::from_raw_parts(ptr, len);
    println!("{:?}", slice); // [10, 20, 30]

    // Pattern: c-string to Rust str
    let cstr: *const i8 = b"hello\0".as_ptr() as *const i8;
    let cstring = std::ffi::CStr::from_ptr(cstr);
    let rstr    = cstring.to_str().unwrap();
    println!("{}", rstr); // hello
}
```

---

## 5. Type Casting in Go

Go has a clean and consistent type system. **All numeric conversions are explicit.** There is no implicit numeric coercion, no implicit narrowing, and no undefined behavior for numeric casts. Unsafe operations exist but are isolated.

### 5.1 Explicit Type Conversion

```go
package main

import "fmt"

func explicitConversions() {
    // Basic numeric conversion syntax: TargetType(value)
    var i int = 42
    var f float64 = float64(i) // int → float64
    var u uint    = uint(f)    // float64 → uint (truncation)

    fmt.Println(i, f, u) // 42 42 42

    // int widths
    var i8  int8  = 100
    var i16 int16 = int16(i8) // widening: safe
    var i32 int32 = int32(i16)
    var i64 int64 = int64(i32)

    _ = i8; _ = i16; _ = i32; _ = i64

    // Narrowing: Go truncates, no error
    var big int32 = 300
    var small int8 = int8(big) // 300 mod 256 = 44
    fmt.Println(small) // 44

    // Negative wrapping
    var neg int32 = -1
    var uval uint32 = uint32(neg) // 0xFFFFFFFF = 4294967295
    fmt.Println(uval) // 4294967295

    // Float → int truncation (towards zero)
    var pf float64 = 3.9
    var pi int = int(pf)  // 3
    var nf float64 = -3.9
    var ni int = int(nf)  // -3
    fmt.Println(pi, ni) // 3 -3

    // Int → float precision
    var large int64 = 16_777_217 // 2^24 + 1
    var f32 float32 = float32(large) // precision loss!
    fmt.Println(f32) // 1.6777216e+07 (rounds to 16777216)
}
```

```
   Go conversion rules summary:

   Numeric type conversion T(x):
   ┌────────────────────────────────────────────────────────┐
   │  int → int (same size):   identity                    │
   │  int → int (wider):       sign-extend if signed,      │
   │                           zero-extend if unsigned      │
   │  int → int (narrower):    truncate (keep low bits)     │
   │  signed → unsigned:       reinterpret bits (mod 2^n)  │
   │  unsigned → signed:       reinterpret bits             │
   │  int → float:             value conversion,            │
   │                           may round (float precision)  │
   │  float → int:             truncate towards zero        │
   │  float → float (wider):   always exact                 │
   │  float → float (narrower):rounds to nearest            │
   └────────────────────────────────────────────────────────┘
```

### 5.2 Numeric Conversions in Detail

```go
package main

import (
    "fmt"
    "math"
)

func numericEdgeCases() {
    // Float to int: no UB in Go (unlike C)!
    // If value is outside int range, result is implementation-specific
    // but NOT undefined behavior
    var huge float64 = 1e100
    var i int64 = int64(huge) // On most platforms: math.MinInt64 (implementation-defined)
    fmt.Println(i)

    // NaN → int
    nan := math.NaN()
    n := int64(nan) // 0 or some value; not UB, not panics
    fmt.Println(n)  // 0 on x86

    // Inf → int
    inf := math.Inf(1)
    inf_i := int64(inf) // math.MinInt64 on x86
    fmt.Println(inf_i)

    // Integer overflow is NOT undefined behavior in Go
    var max int8 = 127
    var over int8 = max + 1 // wraps to -128 (defined: two's complement)
    fmt.Println(over) // -128

    // uintptr ↔ integer
    var addr uintptr = 0xDEADBEEF
    var u64 uint64 = uint64(addr)
    _ = u64
}
```

### 5.3 String ↔ Byte / Rune Slices

This is one of the most common conversions in Go.

```go
package main

import "fmt"

func stringConversions() {
    // string → []byte: copies the bytes
    s := "Hello, 世界"
    b := []byte(s) // UTF-8 encoded bytes
    fmt.Printf("%v\n", b) // [72 101 108 108 111 44 32 228 184 150 231 149 140]
    // "世" is 3 bytes in UTF-8: 228 184 150
    // "界" is 3 bytes in UTF-8: 231 149 140

    // []byte → string: copies the bytes back
    s2 := string(b)
    fmt.Println(s2) // "Hello, 世界"

    // string → []rune: each element is a Unicode code point
    r := []rune(s)
    fmt.Printf("len(s)=%d, len(r)=%d\n", len(s), len(r))
    // len(s)=13 (bytes), len(r)=9 (code points)

    // []rune → string
    s3 := string(r)
    fmt.Println(s3) // "Hello, 世界"

    // int → string: NOT conversion of number to decimal string!
    // It converts the int as a Unicode code point
    ch := string(65)   // string(rune(65)): 'A'
    fmt.Println(ch)    // "A"
    ch2 := string(0x4e16) // 世
    fmt.Println(ch2)   // "世"

    // To convert number to decimal string, use fmt.Sprintf or strconv
    numStr := fmt.Sprintf("%d", 65) // "65"
    fmt.Println(numStr)

    // Byte access doesn't need conversion
    for i, c := range s {
        fmt.Printf("s[%d] = %c (U+%04X)\n", i, c, c)
    }
    // range iterates by rune; s[i] is a byte
}
```

```
   String memory layout in Go:

   string header (16 bytes on 64-bit):
   ┌───────────────────────────────────┬───────────────────┐
   │           data pointer            │       length      │
   │           (8 bytes)               │      (8 bytes)    │
   └───────────────────────────────────┴───────────────────┘
                     │                         │
                     ▼                         ▼
            ┌────────────────────────────┐   len = number of BYTES
            │  H e l l o ,   世 界       │
            │  72 65 6C 6C 6F 2C 20      │
            │  E4 B8 96 E7 95 8C         │
            └────────────────────────────┘
            immutable UTF-8 byte sequence

   []byte slice header (24 bytes on 64-bit):
   ┌───────────────────┬───────────────────┬───────────────────┐
   │    data pointer   │      length       │     capacity      │
   │     (8 bytes)     │     (8 bytes)     │     (8 bytes)     │
   └───────────────────┴───────────────────┴───────────────────┘
              │
              ▼
   ┌──────────────────────────────────────┐
   │  mutable byte array (heap-allocated) │
   └──────────────────────────────────────┘

   string(b)  → allocates new immutable string, copies bytes
   []byte(s)  → allocates new mutable byte slice, copies bytes
   Compiler may optimize away copies in certain contexts (e.g., map lookups).
```

### 5.4 Interface Conversions and Type Assertions

Interfaces are the primary polymorphism mechanism in Go. Conversions to/from interfaces happen via **type assertions**.

```go
package main

import "fmt"

type Animal interface {
    Sound() string
}

type Dog struct{ Name string }
type Cat struct{ Name string }

func (d Dog) Sound() string { return "Woof" }
func (c Cat) Sound() string { return "Meow" }

func interfaceDemo() {
    // Implicit assignment to interface — any type implementing Animal
    var a Animal
    a = Dog{Name: "Rex"}   // Dog → Animal: automatic (no cast syntax)
    fmt.Println(a.Sound()) // "Woof"

    // Type assertion: extract concrete type
    // Syntax: value.(ConcreteType)
    dog, ok := a.(Dog)    // "comma ok" idiom — safe
    if ok {
        fmt.Println("Is a Dog:", dog.Name)
    }

    cat, ok := a.(Cat)    // fails gracefully
    if !ok {
        fmt.Println("Not a Cat")
    }
    _ = cat

    // Panic version (use only when certain)
    // dog2 := a.(Dog)  // panics if a is not Dog
    // _ = dog2

    // interface → interface conversion
    type Namer interface { GetName() string }
    // Dog doesn't implement Namer, so this would fail
    // But if it did: var n Namer = a.(Namer)

    // any (interface{} / empty interface)
    var any_val interface{} = 42
    n, ok := any_val.(int)
    fmt.Println(n, ok) // 42 true

    f, ok := any_val.(float64)
    fmt.Println(f, ok) // 0 false (zero value, not ok)
}
```

```
   Interface memory layout in Go (16 bytes on 64-bit):

   interface value:
   ┌───────────────────────────────────┬───────────────────────────────────┐
   │          type pointer             │          data pointer             │
   │       (pointer to itab)           │    (pointer to value / value)     │
   │           (8 bytes)               │           (8 bytes)               │
   └───────────────────────────────────┴───────────────────────────────────┘
              │                                     │
              ▼                                     ▼
   ┌──────────────────────┐              ┌─────────────────────┐
   │       itab           │              │   Concrete value    │
   │ ┌──────────────────┐ │              │   (or ptr to value  │
   │ │  interface type  │ │              │    on heap)         │
   │ │  concrete type   │ │              └─────────────────────┘
   │ │  method table:   │ │
   │ │   Sound() → ptr  │ │
   │ └──────────────────┘ │
   └──────────────────────┘

   nil interface: both pointers are nil
   non-nil interface with nil data ptr: type ptr non-nil, data ptr nil
   (common "nil pointer in interface" gotcha)

   Type assertion:
   - Check: itab.concrete_type == requested type?
   - If yes: return data pointer, ok=true
   - If no:  return zero value, ok=false (or panic if not comma-ok form)
```

### 5.5 Type Switches

```go
package main

import "fmt"

func typeSwitch(i interface{}) string {
    switch v := i.(type) {
    case int:
        return fmt.Sprintf("int: %d", v)
    case int64:
        return fmt.Sprintf("int64: %d", v)
    case float64:
        return fmt.Sprintf("float64: %f", v)
    case string:
        return fmt.Sprintf("string: %q", v)
    case bool:
        return fmt.Sprintf("bool: %v", v)
    case []int:
        return fmt.Sprintf("[]int of len %d", len(v))
    case nil:
        return "nil"
    default:
        return fmt.Sprintf("unknown type %T", v)
    }
}

func main() {
    fmt.Println(typeSwitch(42))           // int: 42
    fmt.Println(typeSwitch(3.14))         // float64: 3.140000
    fmt.Println(typeSwitch("hello"))      // string: "hello"
    fmt.Println(typeSwitch(nil))          // nil
    fmt.Println(typeSwitch([]int{1,2,3})) // []int of len 3
}
```

### 5.6 The `unsafe.Pointer` and `uintptr` Pipeline

For low-level bit manipulation and interop:

```go
package main

import (
    "fmt"
    "unsafe"
)

func unsafePointerDemo() {
    // Rule: unsafe.Pointer is Go's void*
    // It can hold any pointer type.
    // uintptr is an integer large enough to hold any pointer value.

    x := int32(42)

    // *int32 → unsafe.Pointer → *float32 (type punning)
    up := unsafe.Pointer(&x)
    fp := (*float32)(up)
    fmt.Printf("int32(42) as float32 bits: %v\n", *fp)
    // int 42 = 0x0000002A; as float32 that's a very small denormal number

    // Float bit pattern inspection
    f := float32(1.0)
    ubits := (*uint32)(unsafe.Pointer(&f))
    fmt.Printf("float32(1.0) bits: 0x%08X\n", *ubits) // 0x3F800000

    // Pointer arithmetic using uintptr
    arr := [5]int64{10, 20, 30, 40, 50}
    base := unsafe.Pointer(&arr[0])

    // Access arr[2] via arithmetic
    // WARNING: must complete in one expression; GC may move objects!
    elem2 := (*int64)(unsafe.Pointer(uintptr(base) + 2*unsafe.Sizeof(arr[0])))
    fmt.Println(*elem2) // 30

    // Struct field access via offset
    type Point struct {
        X float64
        Y float64
    }
    p := Point{X: 3.0, Y: 4.0}
    yPtr := (*float64)(unsafe.Pointer(uintptr(unsafe.Pointer(&p)) + unsafe.Offsetof(p.Y)))
    fmt.Println(*yPtr) // 4.0
}
```

```
   unsafe.Pointer conversion rules (Go spec):

   Valid conversions:
   ┌──────────────────────────────────────────────────────────┐
   │  *T              →  unsafe.Pointer                       │
   │  unsafe.Pointer  →  *T                                   │
   │  unsafe.Pointer  →  uintptr   (for arithmetic)           │
   │  uintptr         →  unsafe.Pointer   (MUST be immediate) │
   └──────────────────────────────────────────────────────────┘

   DANGER: uintptr is just an integer. The GC does NOT treat it as
   a pointer. If a GC cycle runs between computing the uintptr and
   converting back to unsafe.Pointer, the object may have moved
   (on moving GC implementations) or been collected (if no other
   live reference exists).

   Safe pattern (all in one expression):
   (*T)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + offset))

   Unsafe pattern (GC may invalidate intermediate uintptr):
   u := uintptr(unsafe.Pointer(p))
   // GC runs here...
   unsafe.Pointer(u)  // u may point to freed memory

   uintptr lifetime:
   ┌────────────────────────────────────────────────┐
   │  uintptr  ←  just a number                     │
   │  GC sees: "no pointer here"                    │
   │  Object can be moved/collected while you hold  │
   │  the uintptr value.                            │
   │                                                │
   │  unsafe.Pointer ← GC sees: "pointer here"      │
   │  Object is pinned in memory while you hold it  │
   └────────────────────────────────────────────────┘
```

### 5.7 `reflect`-Based Conversion

```go
package main

import (
    "fmt"
    "reflect"
)

func reflectCast() {
    // reflect.Value can convert between numeric types
    x := 42
    v := reflect.ValueOf(x)
    
    // Convert to float64
    fv := v.Convert(reflect.TypeOf(float64(0)))
    fmt.Println(fv.Float()) // 42.0

    // Dynamic type check and conversion
    values := []interface{}{42, 3.14, "hello", true}
    for _, val := range values {
        rv := reflect.ValueOf(val)
        switch rv.Kind() {
        case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
            fmt.Printf("int: %d\n", rv.Int())
        case reflect.Float32, reflect.Float64:
            fmt.Printf("float: %f\n", rv.Float())
        case reflect.String:
            fmt.Printf("string: %s\n", rv.String())
        case reflect.Bool:
            fmt.Printf("bool: %v\n", rv.Bool())
        }
    }
}
```

### 5.8 Aliased Types vs Defined Types

This is a Go-specific concept that affects what conversions are implicit vs explicit.

```go
package main

import "fmt"

// TYPE DEFINITION: creates a new, distinct named type
type Celsius    float64
type Fahrenheit float64
type MyInt      int

// TYPE ALIAS: just another name for the same type
type Alias = int  // Alias IS int; no conversion needed

func typeDefinitionDemo() {
    var c Celsius    = 100.0
    var f Fahrenheit = Fahrenheit(c*9/5 + 32) // explicit conversion required
    // f = c  // compile error: cannot use Celsius as Fahrenheit
    // f = 100.0 + 32  // valid: untyped constant fits Fahrenheit

    fmt.Println(f) // 212

    var i int   = 42
    var m MyInt = MyInt(i)  // explicit conversion required
    // m = i // compile error: cannot use int as MyInt

    fmt.Println(m)

    // Alias works transparently
    var a Alias = 10
    var b int   = a  // no conversion needed: same type
    fmt.Println(a, b)

    // Underlying type compatibility
    // Two defined types with same underlying type CAN be converted:
    type Meters float64
    type Seconds float64
    var d Meters = 100.0
    // var t Seconds = d  // compile error even though both are float64 underneath
    var t Seconds = Seconds(d)  // explicit conversion: compiles, but semantically wrong!
    // The type system is trying to PREVENT this! Don't do it.
    _ = t
}
```

```
   Defined Type vs Alias:

   type MyInt int   ← NEW TYPE (distinct from int)
   ┌──────────────────────────────────────────────┐
   │  MyInt is its own type.                      │
   │  Underlying type: int                        │
   │  int → MyInt: requires explicit MyInt(value) │
   │  MyInt → int: requires explicit int(value)   │
   │  MyInt and int share operators (+, -, etc.)  │
   │  because they share the same underlying type │
   └──────────────────────────────────────────────┘

   type Alias = int   ← ALIAS (just another name)
   ┌──────────────────────────────────────────────┐
   │  Alias IS int. Same type.                    │
   │  No conversion ever needed.                  │
   │  Alias and int are interchangeable.           │
   └──────────────────────────────────────────────┘
```

---

## 6. Side-by-Side Comparison Table

```
   Operation          │ C                    │ Rust                    │ Go
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Syntax             │ (Type)expr           │ expr as Type            │ Type(expr)
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Implicit numeric   │ Yes (widening +      │ No (all explicit)       │ No (all explicit)
   conversions        │ integer promotion)   │                         │
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Narrowing behavior │ Truncate (defined    │ Truncate (defined by    │ Truncate (defined;
                      │ for unsigned; impl-  │ spec: keep low bits)    │ never UB)
                      │ defined for signed)  │                         │
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Float → Int        │ Truncates; overflow  │ Saturates (since 1.45); │ Truncates; impl-
   overflow           │ is UB                │ NaN → 0                 │ defined, not UB
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Integer overflow   │ Signed: UB           │ Debug: panic            │ Wraps (always
   (arithmetic)       │ Unsigned: wraps      │ Release: wraps          │ defined)
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Pointer to int     │ (uintptr_t)ptr       │ ptr as usize            │ uintptr(ptr) via
                      │                      │                         │ unsafe.Pointer
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Int to pointer     │ (T*)int              │ int as *T (unsafe       │ unsafe.Pointer(
                      │                      │ to deref)               │ uintptr(n)) → *T
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Type punning       │ union (C11: ok)      │ transmute (unsafe)      │ unsafe.Pointer
   (reinterpret bits) │ memcpy (always ok)   │ .to_bits() (safe)       │ reinterpret
                      │ *(T*)&x (UB!)        │ .from_bits() (safe)     │
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Generic pointer    │ void*                │ *const ()               │ unsafe.Pointer
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Polymorphic        │ void* + fn pointers  │ &dyn Trait (fat ptr)    │ interface{}
   dispatch           │                      │ generics (monomorphic)  │
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Runtime type check │ No (unsafe cast only)│ downcast Any/dyn        │ type assertion .(T)
                      │                      │ + trait objects         │ type switch
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Infallible convert │ N/A (use cast)       │ From/Into traits        │ N/A (use explicit)
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Fallible convert   │ N/A (check manually) │ TryFrom/TryInto        │ N/A (check manually)
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   String → bytes     │ (char*)str           │ str.as_bytes()          │ []byte(str)
   ───────────────────┼──────────────────────┼─────────────────────────┼──────────────────────
   Unsafe scope       │ entire file / -Wall  │ unsafe { } block        │ import "unsafe"
```

---

## 7. Deep Dives: Internals and Edge Cases

### 7.1 IEEE 754 and Float Casts

```
   IEEE 754 Single Precision (f32 / float):

   Bit 31  │  Bits 30-23  │  Bits 22-0
   ─────────┼──────────────┼────────────────────────
   Sign (S) │  Exponent(E) │  Mantissa (M)
   1 bit    │  8 bits      │  23 bits
   
   Value = (-1)^S × 2^(E-127) × 1.M  (normalized)
   Special cases:
     E = 0,   M = 0 → ±0
     E = 0,   M ≠ 0 → denormalized: (-1)^S × 2^(-126) × 0.M
     E = 255, M = 0 → ±Infinity
     E = 255, M ≠ 0 → NaN (quiet if M[22]=1, signaling if M[22]=0)

   Key values:
   ┌──────────────────────┬────────────────────┬──────────────────┐
   │  Float value         │  Hex bits          │  Binary          │
   ├──────────────────────┼────────────────────┼──────────────────┤
   │  0.0                 │  0x00000000        │  0 00000000 ...  │
   │  -0.0                │  0x80000000        │  1 00000000 ...  │
   │  1.0                 │  0x3F800000        │  0 01111111 ...  │
   │  -1.0                │  0xBF800000        │  1 01111111 ...  │
   │  2.0                 │  0x40000000        │  0 10000000 ...  │
   │  0.5                 │  0x3F000000        │  0 01111110 ...  │
   │  +Infinity           │  0x7F800000        │  0 11111111 0..0 │
   │  -Infinity           │  0xFF800000        │  1 11111111 0..0 │
   │  qNaN                │  0x7FC00000        │  0 11111111 1..0 │
   │  Max f32 (~3.4e38)   │  0x7F7FFFFF        │                  │
   │  Min normal(~1.2e-38)│  0x00800000        │                  │
   └──────────────────────┴────────────────────┴──────────────────┘

   int32 → float32 precision loss example:
   
   16777216 = 0x01000000 = 2^24
   Exact representable (mantissa fits)

   16777217 = 0x01000001 = 2^24 + 1
   ┌──────────────────────────────────────────────────────────────┐
   │  In binary: 1 0000 0000 0000 0000 0000 0001                 │
   │  Has 25 significant bits; f32 has only 24.                  │
   │  Rounded to nearest: 1 0000 0000 0000 0000 0000 000[0]      │
   │  = 16777216 (same as 2^24!)                                  │
   └──────────────────────────────────────────────────────────────┘
```

### 7.2 Two's Complement and Wrap-Around

```
   Two's complement for N-bit integers:
   
   Value of bit pattern b[N-1]...b[0]:
   = -b[N-1] * 2^(N-1) + sum(b[i] * 2^i for i in 0..N-2)
   
   Negation: flip all bits, add 1
   Example (8-bit):
     42  = 0010 1010
    -42  = 1101 0101 + 1 = 1101 0110 = 0xD6
    
   Verify: 0xD6 = -128 + 64 + 0 + 16 + 0 + 4 + 2 + 0 = -42 ✓

   Wrapping arithmetic (add, sub, mul):
   All operations are modular 2^N.
   
   INT8_MAX + 1:
     0111 1111  (127)
   + 0000 0001  (1)
   ─────────────
     1000 0000  (-128)   ← overflow wraps around

   In C: signed overflow is UB (compiler may assume it never happens)
   In Rust: debug mode panics; release mode wraps (use .wrapping_add())
   In Go: always wraps (defined behavior)

   Useful identity: -1 as unsigned = all-ones = maximum unsigned value
   
   u8:   (u8)(-1) = 0xFF = 255
   u16:  (u16)(-1) = 0xFFFF = 65535
   u32:  (u32)(-1) = 0xFFFFFFFF = 4294967295
   u64:  (u64)(-1) = 0xFFFFFFFFFFFFFFFF = 18446744073709551615
```

### 7.3 Pointer Provenance

Modern compilers track **pointer provenance** — which allocation a pointer "came from". This is crucial for understanding casting rules.

```
   Pointer Provenance Model:

   int x = 42;
   int y = 99;
   
   int* px = &x;   // px has PROVENANCE of x's allocation
   int* py = &y;   // py has PROVENANCE of y's allocation
   
   uintptr_t ax = (uintptr_t)px;  // ax is an INTEGER, no provenance
   uintptr_t ay = (uintptr_t)py;
   
   if (ax == ay - 4) {  // Hypothetically adjacent on stack
       int* p = (int*)ax;   // This pointer has UNDEFINED provenance
       *p = 5;              // UB even if numerically correct address!
   }
   
   Why does this matter?
   Compilers track provenance for alias analysis.
   Two pointers with different provenance cannot alias → optimization.

   RUST: Pointer provenance is explicitly modeled.
   std::ptr::from_exposed_addr() and .expose_addr() are the APIs
   for when you MUST go through integers and back.

   C: The new provenance model (ongoing standardization) uses
   __builtin_provenance() or similar to maintain it through integers.

   GO: unsafe.Pointer is the only sanctioned way; uintptr loses provenance.
```

### 7.4 Strict Aliasing

```
   Strict Aliasing Rule Summary:

   C (with -O2 default optimization):
   
   void broken(float* fp, int* ip) {
       *fp = 1.0f;
       *ip = 0;
       return *fp;  // Compiler KNOWS float* and int* can't alias.
                    // Returns 1.0f WITHOUT re-reading memory.
   }
   
   At the assembly level (x86):
   mov  dword [rdi], 0x3F800000   ; *fp = 1.0f
   mov  dword [rsi], 0            ; *ip = 0
   movss xmm0, [???]              ; May be OPTIMIZED TO: return 1.0f constant
   ret
   
   The compiler legally assumes that if you gave it different pointer types,
   they don't point to the same memory (alias). It's YOUR job to ensure this.
   
   Exceptions in C:
   - char* / unsigned char* / signed char*: can alias ANYTHING
   - A struct type and its first member type
   - Compatible aggregate types
   
   Rust's aliasing model (much stricter):
   - &T references: shared, never alias &mut T to same data
   - &mut T references: exclusive; nothing else can access that data
   - Raw pointers *const T, *mut T: no aliasing guarantees at all (unsafe only)
   - The borrow checker enforces this at compile time
   
   Go:
   - Go is specified to NOT use strict aliasing
   - The Go spec guarantees that unsafe.Pointer access is well-defined
     as long as you follow the stated patterns
   - Go programs are not subject to GCC/Clang strict-aliasing UB
```

### 7.5 Endianness

```
   Endianness affects how multi-byte values are laid out in memory.

   Value: 0x01020304 stored as int32

   Big-Endian (network byte order, some embedded):
   addr+0  addr+1  addr+2  addr+3
   ┌──────┬──────┬──────┬──────┐
   │  01  │  02  │  03  │  04  │
   └──────┴──────┴──────┴──────┘
   Most significant byte at lowest address

   Little-Endian (x86, x86_64, ARM in LE mode):
   addr+0  addr+1  addr+2  addr+3
   ┌──────┬──────┬──────┬──────┐
   │  04  │  03  │  02  │  01  │
   └──────┴──────┴──────┴──────┘
   Least significant byte at lowest address

   Casting pointer to char* on little-endian:
   int x = 0x01020304;
   char* p = (char*)&x;
   p[0] == 0x04  // LSB
   p[1] == 0x03
   p[2] == 0x02
   p[3] == 0x01  // MSB

   Endianness matters for:
   1. Network protocols (always use htonl/ntohl in C)
   2. File formats (JPEG, PNG, etc. specify byte order)
   3. Cross-platform serialization
   4. Any time you cast a multi-byte type to char* or uint8_t*

   In Rust:
   u32::from_be_bytes([0x01, 0x02, 0x03, 0x04]) // = 0x01020304 (big-endian)
   u32::from_le_bytes([0x04, 0x03, 0x02, 0x01]) // = 0x01020304 (little-endian)
   u32::from_ne_bytes(bytes)                     // native endian

   In Go:
   binary.BigEndian.Uint32(buf)    // big-endian bytes → uint32
   binary.LittleEndian.Uint32(buf) // little-endian bytes → uint32
```

---

## 8. Complete Code Examples

### 8.1 Fast Inverse Square Root (Quake III Classic)

This classic algorithm uses type punning to manipulate float bits:

```c
// ─── C version ────────────────────────────────────────────────────────────────
#include <stdint.h>

float fast_inv_sqrt_c(float number) {
    float x2 = number * 0.5f;
    float y   = number;

    // Evil floating point bit level hacking
    uint32_t bits;
    __builtin_memcpy(&bits, &y, sizeof(bits)); // safe: memcpy punning
    bits = 0x5F3759DF - (bits >> 1);            // magic constant + bit manipulation
    __builtin_memcpy(&y, &bits, sizeof(y));

    // One Newton-Raphson iteration
    y = y * (1.5f - (x2 * y * y));
    return y;
}
```

```rust
// ─── Rust version ─────────────────────────────────────────────────────────────
fn fast_inv_sqrt_rust(number: f32) -> f32 {
    let x2 = number * 0.5;
    let bits = number.to_bits();           // safe: defined by std
    let bits = 0x5F3759DF_u32 - (bits >> 1);
    let y = f32::from_bits(bits);          // safe: defined by std
    y * (1.5 - (x2 * y * y))              // one Newton-Raphson iteration
}
```

```go
// ─── Go version ───────────────────────────────────────────────────────────────
package main

import (
    "math"
    "unsafe"
)

func fastInvSqrtGo(number float32) float32 {
    x2 := number * 0.5
    // Type punning via unsafe.Pointer
    bits := *(*uint32)(unsafe.Pointer(&number))
    bits = 0x5F3759DF - (bits >> 1)
    y := *(*float32)(unsafe.Pointer(&bits))
    return y * (1.5 - float32(x2)*y*y)
}

// Safer Go version using math.Float32bits
func fastInvSqrtGoSafe(number float32) float32 {
    x2 := number * 0.5
    bits := math.Float32bits(number)       // defined: float32 → uint32 bits
    bits = 0x5F3759DF - (bits >> 1)
    y := math.Float32frombits(bits)        // defined: uint32 bits → float32
    return y * (1.5 - float32(x2)*y*y)
}
```

### 8.2 Serialization / Deserialization (Binary Protocol)

```c
// ─── C: Network packet parsing ────────────────────────────────────────────────
#include <stdint.h>
#include <string.h>
#include <arpa/inet.h>

typedef struct {
    uint16_t msg_type;
    uint16_t length;
    uint32_t sequence;
} PacketHeader;

void parse_packet(const uint8_t* buf, size_t buf_len) {
    if (buf_len < sizeof(PacketHeader)) return;

    PacketHeader hdr;
    // Safe: memcpy handles alignment for us
    memcpy(&hdr, buf, sizeof(hdr));

    // Convert network byte order → host byte order
    uint16_t type = ntohs(hdr.msg_type);
    uint16_t len  = ntohs(hdr.length);
    uint32_t seq  = ntohl(hdr.sequence);

    // Access payload
    const uint8_t* payload = buf + sizeof(hdr);
    // process(type, len, seq, payload);
}
```

```rust
// ─── Rust: Network packet parsing ────────────────────────────────────────────
use std::convert::TryInto;

#[repr(C, packed)]
struct PacketHeader {
    msg_type: u16,
    length:   u16,
    sequence: u32,
}

fn parse_packet(buf: &[u8]) -> Option<(u16, u16, u32, &[u8])> {
    if buf.len() < 8 { return None; }

    // Safe extraction using from_be_bytes (network = big-endian)
    let msg_type = u16::from_be_bytes(buf[0..2].try_into().ok()?);
    let length   = u16::from_be_bytes(buf[2..4].try_into().ok()?);
    let sequence = u32::from_be_bytes(buf[4..8].try_into().ok()?);
    let payload  = &buf[8..];

    Some((msg_type, length, sequence, payload))
}

// Usage:
fn main() {
    let packet: &[u8] = &[0x00, 0x01, 0x00, 0x0C, 0x00, 0x00, 0x00, 0x01, b'H', b'i'];
    if let Some((t, l, s, p)) = parse_packet(packet) {
        println!("type={}, len={}, seq={}, payload={:?}", t, l, s, p);
    }
}
```

```go
// ─── Go: Network packet parsing ───────────────────────────────────────────────
package main

import (
    "encoding/binary"
    "fmt"
)

type PacketHeader struct {
    MsgType  uint16
    Length   uint16
    Sequence uint32
}

func parsePacket(buf []byte) (*PacketHeader, []byte, error) {
    if len(buf) < 8 {
        return nil, nil, fmt.Errorf("buffer too short")
    }
    hdr := &PacketHeader{
        MsgType:  binary.BigEndian.Uint16(buf[0:2]),
        Length:   binary.BigEndian.Uint16(buf[2:4]),
        Sequence: binary.BigEndian.Uint32(buf[4:8]),
    }
    return hdr, buf[8:], nil
}
```

### 8.3 Safe Numeric Range Conversion

```c
// ─── C: Manual range check before cast ───────────────────────────────────────
#include <limits.h>
#include <stdbool.h>

bool safe_i32_to_i8(int32_t in, int8_t* out) {
    if (in < INT8_MIN || in > INT8_MAX) return false;
    *out = (int8_t)in;
    return true;
}

bool safe_f64_to_i32(double in, int32_t* out) {
    if (in < (double)INT32_MIN || in > (double)INT32_MAX || in != in) // NaN check
        return false;
    *out = (int32_t)in;
    return true;
}
```

```rust
// ─── Rust: TryFrom does this automatically ────────────────────────────────────
use std::convert::TryFrom;

fn safe_conversions() {
    // i32 → i8: TryFrom returns Err if out of range
    assert_eq!(i8::try_from(42i32),  Ok(42i8));
    assert!(i8::try_from(300i32).is_err()); // 300 > i8::MAX

    // Manual f64 → i32 (no TryFrom for float→int in std)
    fn f64_to_i32(v: f64) -> Option<i32> {
        if v.is_nan() || v.is_infinite() { return None; }
        if v < i32::MIN as f64 || v > i32::MAX as f64 { return None; }
        Some(v as i32)  // now safe to `as` cast
    }
    println!("{:?}", f64_to_i32(3.9));       // Some(3)
    println!("{:?}", f64_to_i32(1e18_f64));   // None
    println!("{:?}", f64_to_i32(f64::NAN));   // None
}
```

```go
// ─── Go: Manual check required (no TryFrom equivalent) ───────────────────────
package main

import (
    "fmt"
    "math"
)

func safeF64ToI32(v float64) (int32, bool) {
    if math.IsNaN(v) || math.IsInf(v, 0) {
        return 0, false
    }
    if v < math.MinInt32 || v > math.MaxInt32 {
        return 0, false
    }
    return int32(v), true
}

func main() {
    n, ok := safeF64ToI32(3.9)
    fmt.Println(n, ok) // 3 true
    n, ok = safeF64ToI32(1e18)
    fmt.Println(n, ok) // 0 false
}
```

### 8.4 Type-Erased Container

```c
// ─── C: void* type-erased array ──────────────────────────────────────────────
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

typedef struct {
    void*  data;
    size_t elem_size;
    size_t count;
    size_t capacity;
} TypedArray;

TypedArray ta_new(size_t elem_size) {
    return (TypedArray){ .data = NULL, .elem_size = elem_size, .count = 0, .capacity = 0 };
}

void ta_push(TypedArray* a, const void* elem) {
    if (a->count == a->capacity) {
        a->capacity = a->capacity ? a->capacity * 2 : 4;
        a->data = realloc(a->data, a->capacity * a->elem_size);
    }
    // Pointer arithmetic via char* (char* arithmetic is always 1 byte)
    char* dest = (char*)a->data + a->count * a->elem_size;
    memcpy(dest, elem, a->elem_size);
    a->count++;
}

void* ta_get(TypedArray* a, size_t i) {
    return (char*)a->data + i * a->elem_size;
}

int main(void) {
    TypedArray arr = ta_new(sizeof(double));
    double vals[] = {1.1, 2.2, 3.3};
    for (int i = 0; i < 3; i++) ta_push(&arr, &vals[i]);

    for (size_t i = 0; i < arr.count; i++) {
        double* p = (double*)ta_get(&arr, i);
        printf("%.1f\n", *p);
    }
    free(arr.data);
    return 0;
}
```

```rust
// ─── Rust: Generic Vec (no unsafe needed) ────────────────────────────────────
fn generic_demo() {
    // Rust generics are monomorphized — same safety, no void*
    let mut v: Vec<f64> = Vec::new();
    v.push(1.1);
    v.push(2.2);
    v.push(3.3);
    for x in &v { println!("{:.1}", x); }

    // Type-erased with Any (like void* but safe)
    use std::any::Any;
    let items: Vec<Box<dyn Any>> = vec![
        Box::new(42i32),
        Box::new(3.14f64),
        Box::new(String::from("hello")),
    ];
    for item in &items {
        if let Some(n) = item.downcast_ref::<i32>()  { println!("i32: {}", n); }
        if let Some(f) = item.downcast_ref::<f64>()  { println!("f64: {}", f); }
        if let Some(s) = item.downcast_ref::<String>(){ println!("String: {}", s); }
    }
}
```

```go
// ─── Go: interface{} type-erased container ───────────────────────────────────
package main

import "fmt"

func typeErasedDemo() {
    items := []interface{}{42, 3.14, "hello", true}
    for _, item := range items {
        switch v := item.(type) {
        case int:
            fmt.Printf("int: %d\n", v)
        case float64:
            fmt.Printf("float64: %f\n", v)
        case string:
            fmt.Printf("string: %s\n", v)
        case bool:
            fmt.Printf("bool: %v\n", v)
        }
    }
}
```

---

## 9. Summary Mental Models

### Mental Model 1: The Type Lens

```
   A value in memory is just bytes.
   A type is the lens through which those bytes are interpreted.
   Casting changes the lens.

   ┌─────────────────────────────────────────────────────────────────┐
   │                  Memory: 4 bytes                                │
   │    ┌──────┬──────┬──────┬──────┐                               │
   │    │  3F  │  80  │  00  │  00  │                               │
   │    └──────┴──────┴──────┴──────┘                               │
   │         │             │               │                         │
   │    uint32 lens    float32 lens    int32 lens                    │
   │    = 1065353216   = 1.0            = 1065353216                 │
   └─────────────────────────────────────────────────────────────────┘

   Value-preserving cast: change bits to preserve mathematical value
   Reinterpretation cast: keep bits, change the lens
```

### Mental Model 2: Safety Spectrum

```
   Safety ────────────────────────────────────────────────► Danger

   C         (Type)expr
   ├── Widening int        ✅ Always safe
   ├── int → double        ✅ Always safe (all i32 fit)
   ├── Narrowing int       ⚠️  Truncates silently
   ├── signed ↔ unsigned   ⚠️  Reinterprets bits
   ├── float → int         ⚠️  Truncates; overflow is UB
   ├── float ↔ int bits   ⚠️  UB via pointer cast; ok via union/memcpy
   └── pointer casts       ❌  Aliasing / alignment UB risk

   Rust     expr as Type
   ├── Widening (i16→i64)  ✅ But use From for clearest intent
   ├── From/Into           ✅ Infallible, lossless by definition
   ├── TryFrom/TryInto     ✅ Checked, returns Result
   ├── as narrowing        ⚠️  Truncates silently (allowed, documented)
   ├── as float→int        ✅ Saturates since 1.45 (no more UB)
   ├── transmute           ❌  Unsafe, zero checks, full power
   └── raw pointer casts   ❌  Unsafe to dereference

   Go       Type(expr)
   ├── Numeric conversions ✅ All well-defined, no UB
   ├── Type assertion .(T) ✅ Comma-ok form never panics
   ├── Type switch         ✅ Exhaustive, safe
   ├── unsafe.Pointer      ⚠️  Follows strict conversion rules
   └── uintptr ↔ pointer   ❌  GC hazard; must be atomic expression
```

### Mental Model 3: Choose the Right Tool

```
   You need to...                           Use in:
   ─────────────────────────────────────────────────────────────────
   Convert i32 → i64 (lossless)       C: (long)i  Rust: i64::from(i)  Go: int64(i)
   Convert i32 → i8 (may fail)        C: check+cast  Rust: i8::try_from(i)  Go: check+int8(i)
   Convert i32 → i8 (just truncate)   C: (char)i  Rust: i as i8  Go: int8(i)
   Inspect float bits                  C: memcpy  Rust: f.to_bits()  Go: math.Float32bits(f)
   Pointer → integer                  C: (uintptr_t)p  Rust: p as usize  Go: uintptr(unsafe.Ptr(p))
   Dynamic type dispatch               C: void*+fn  Rust: dyn Trait  Go: interface{}
   Runtime type check                  C: external tag  Rust: downcast/Any  Go: type assertion
   Reinterpret raw bits                C: union/memcpy  Rust: transmute (unsafe)  Go: unsafe.Pointer
```

### Mental Model 4: Where Bugs Hide

```
   C BUG HOTSPOTS:
   ┌──────────────────────────────────────────────────────────┐
   │ 1. signed/unsigned comparison (always check with -Wsign-compare)    │
   │ 2. char signed-ness varies by platform (use uint8_t/int8_t)         │
   │ 3. float→int overflow UB (check range first)                        │
   │ 4. Pointer cast + deref without alignment check                     │
   │ 5. Forgetting integer promotion in char arithmetic                  │
   │ 6. Strict aliasing violations (use union/memcpy)                    │
   └──────────────────────────────────────────────────────────┘

   RUST BUG HOTSPOTS:
   ┌──────────────────────────────────────────────────────────┐
   │ 1. `as` narrowing silently truncates (use TryFrom if value matters) │
   │ 2. transmute lifetime extension (dangling reference)                │
   │ 3. transmute bool/enum with invalid bit patterns                    │
   │ 4. uintptr_t intermediate in raw pointer arithmetic                 │
   └──────────────────────────────────────────────────────────┘

   GO BUG HOTSPOTS:
   ┌──────────────────────────────────────────────────────────┐
   │ 1. Unchecked type assertion panic (use comma-ok form)               │
   │ 2. Non-nil interface with nil concrete pointer                      │
   │ 3. uintptr stored across GC-safe points                             │
   │ 4. int(float) silently truncates (no rounding)                      │
   │ 5. string(int) produces Unicode char, not digit string              │
   └──────────────────────────────────────────────────────────┘
```

### Mental Model 5: The Casting Decision Tree

```
   Need to convert value A of type T to type U?
   
   ┌─ Are T and U the same type? ─── Yes ──► No conversion needed
   │
   ├─ Is the conversion always lossless (U can represent all T values)?
   │     ├── Yes, C:    (U)a    (implicit if widening)
   │     ├── Yes, Rust: U::from(a) or a.into()
   │     └── Yes, Go:   U(a)
   │
   ├─ Might the conversion lose data (narrowing / float precision)?
   │     ├── You want a runtime error if data lost:
   │     │     C:    check manually + (U)a
   │     │     Rust: U::try_from(a)?  or .try_into()?
   │     │     Go:   check manually + U(a)
   │     │
   │     └── You accept truncation (e.g., low N bits):
   │           C:    (U)a       (truncates)
   │           Rust: a as U     (truncates, silently)
   │           Go:   U(a)       (truncates, silently)
   │
   ├─ Do you want to REINTERPRET the bit pattern (no value conversion)?
   │     C:    union or memcpy (safe); *(U*)&a (UB via strict aliasing!)
   │     Rust: a.to_bits() / U::from_bits(a) (for floats);
   │           std::mem::transmute::<T, U>(a) (unsafe, general)
   │     Go:   *(*U)(unsafe.Pointer(&a))
   │
   └─ Do you need runtime type identity check?
         C:    external type tag / tagged union
         Rust: Any::downcast_ref::<U>()  or  match on dyn Trait
         Go:   value.(U) or type switch
```

---

*End of guide. This document covers all standard type casting mechanisms in C, Rust, and Go, including their internal memory representations, safety properties, undefined behavior hazards, and idiomatic usage patterns.*
