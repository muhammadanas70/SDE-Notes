# C Format Specifiers — The Complete In-Depth Guide

> Covers every specifier, flag, modifier, internal mechanism, memory layout, va_list internals,
> custom printf implementation, scanf semantics, undefined behavior, and security implications.

---

## Table of Contents

1. [What Are Format Specifiers?](#1-what-are-format-specifiers)
2. [The Anatomy of a Format Specifier](#2-the-anatomy-of-a-format-specifier)
3. [How printf Works Under the Hood](#3-how-printf-works-under-the-hood)
4. [va_list and the Calling Convention](#4-va_list-and-the-calling-convention)
5. [Integer Specifiers](#5-integer-specifiers)
6. [Floating-Point Specifiers](#6-floating-point-specifiers)
7. [Character and String Specifiers](#7-character-and-string-specifiers)
8. [Pointer Specifier](#8-pointer-specifier)
9. [Special Specifiers — %% and %n](#9-special-specifiers----and-n)
10. [Width and Precision](#10-width-and-precision)
11. [Flags](#11-flags)
12. [Length Modifiers](#12-length-modifiers)
13. [scanf — Input Parsing Deep Dive](#13-scanf--input-parsing-deep-dive)
14. [Type Promotion and Default Argument Promotions](#14-type-promotion-and-default-argument-promotions)
15. [Undefined Behavior and Common Pitfalls](#15-undefined-behavior-and-common-pitfalls)
16. [Building a Minimal printf from Scratch](#16-building-a-minimal-printf-from-scratch)
17. [Format String Vulnerabilities](#17-format-string-vulnerabilities)
18. [Quick Reference Card](#18-quick-reference-card)

---

## 1. What Are Format Specifiers?

A **format specifier** is a placeholder embedded inside a format string. It tells the standard
library functions (`printf`, `scanf`, `sprintf`, `fprintf`, `sscanf`, etc.) two things:

1. **What type** of data to consume from the variable argument list (or write into, for `scanf`).
2. **How to render / parse** that data — base, width, precision, padding, sign, etc.

The format string is a regular C string (a `const char *`). Everything in it that is *not* a
format specifier is copied literally to the output (for `printf` family) or matched literally
against the input (for `scanf` family).

```
"Result: %+10.4f meters\n"
         ^-----------^
         This is one format specifier
```

### Functions that use format strings

| Family        | Functions                                              |
|---------------|--------------------------------------------------------|
| Output        | `printf`, `fprintf`, `sprintf`, `snprintf`, `vprintf`, `vfprintf`, `vsprintf`, `vsnprintf` |
| Input         | `scanf`, `fscanf`, `sscanf`, `vscanf`, `vfscanf`, `vsscanf` |
| Wide output   | `wprintf`, `fwprintf`, `swprintf`                      |
| Misc          | `syslog` (POSIX), `err` / `warn` (BSD)                 |

---

## 2. The Anatomy of a Format Specifier

Every format specifier follows this grammar, specified in C11 §7.21.6.1:

```
%[flags][width][.precision][length-modifier]conversion-specifier
  ^       ^        ^            ^                  ^
  |       |        |            |                  |
  |       |        |            |                  Must be present
  |       |        |            Optional: h hh l ll L z t j
  |       |        Optional: .N or .* (for scanf: .*  is ignored)
  |       Optional: N or * (read from next argument)
  Always % sign
```

### Full breakdown with example

```
   %   -   10   .   5    l     f
   |   |    |   |   |    |     |
   |   |    |   |   |    |     conversion: floating point
   |   |    |   |   |    length modifier: long double? No — 'l' on 'f' = double
   |   |    |   |   precision: 5 digits after decimal
   |   |    |   precision separator
   |   |    minimum field width: 10 characters
   |   flag: left-justify
   % introducer
```

### All conversion specifiers at a glance

```
INTEGERS          FLOATS           CHARS / STRINGS    SPECIAL
  %d  — signed      %f  — decimal    %c  — char          %%  — literal %
  %i  — signed      %e  — sci lower  %s  — string        %n  — write count
  %u  — unsigned    %E  — sci upper  %lc — wide char
  %o  — octal       %g  — shorter    %ls — wide string
  %x  — hex lower   %G  — shorter    %p  — pointer
  %X  — hex upper   %a  — hex float
                    %A  — hex float upper
```

---

## 3. How printf Works Under the Hood

Understanding `printf` at a mechanical level prevents entire classes of bugs.

### 3.1 High-level flow

```
printf("x=%d, y=%f\n", 42, 3.14);
  |
  v
  1. Receive format string + variadic arguments
  2. Walk the format string byte-by-byte
  3. When '%' is encountered:
       a. Parse flags, width, precision, length modifier, specifier
       b. Pull the next argument from the va_list
       c. Convert the value to text
       d. Apply field width / precision / flags
       e. Write the resulting text to the output stream
  4. Non-'%' characters are written directly to the output stream
  5. Return total number of characters written
```

### 3.2 ASCII state machine for format string parsing

```
State machine inside printf's format string scanner:

        +----------+
        |  START   |<-----------------------------------------+
        +----------+                                          |
             |                                                |
    read char c                                               |
             |                                                |
    c == '%'?                                                 |
      /      \                                                |
    YES       NO                                              |
     |         \                                              |
     |    [LITERAL]                                           |
     |    emit c to output                                    |
     |    goto START                                          |
     |                                                        |
  [SPEC_START]                                                |
  initialize:                                                 |
    flags=0, width=0,                                         |
    prec=-1, lenmod=NONE                                      |
     |                                                        |
     v                                                        |
  [PARSE_FLAGS] <---+                                         |
  read char c       |                                         |
  c in {-,+,' ',#,0}?                                        |
      YES: record flag, loop back --+                         |
      NO:  fall through                                       |
     |                                                        |
     v                                                        |
  [PARSE_WIDTH]                                               |
  c == '*'?  -> pop int from va_list as width                 |
  c in '0'..'9'? -> accumulate digits as width                |
     |                                                        |
     v                                                        |
  [PARSE_PREC]                                                |
  c == '.'?  -> read precision                                |
    next == '*'? -> pop int from va_list                      |
    else accumulate digits                                    |
     |                                                        |
     v                                                        |
  [PARSE_LENMOD]                                              |
  c in {h,l,L,z,t,j}? -> record, advance                     |
     |                                                        |
     v                                                        |
  [CONVERSION]                                                |
  dispatch on specifier character                             |
  (d,i,u,o,x,X,f,e,g,a,c,s,p,n,%)                           |
  pull argument, convert to text, pad, emit                   |
     |                                                        |
     +--------------------------------------------------------+
                    loop back to START
```

### 3.3 The conversion pipeline for a single specifier

```
Raw argument from va_list
         |
         v
  [TYPE INTERPRETATION]         ← length modifier decides this
  e.g. int, long, long long, double, long double, char*, void*
         |
         v
  [VALUE → TEXT CONVERSION]     ← specifier decides the algorithm
  e.g. itoa base 10, itoa base 16, dtoa, etc.
         |
         v
  [PRECISION APPLICATION]
  Strings: truncate at .N chars
  Integers: pad with leading zeros to at least .N digits
  Floats: digits after decimal point (default 6)
         |
         v
  [SIGN / PREFIX INSERTION]     ← flags decide this
  '+' flag: prepend +
  ' ' flag: prepend space
  '#' flag: prepend 0, 0x, 0X for o, x, X
         |
         v
  [FIELD WIDTH PADDING]         ← width and '-' flag
  Left-justify '-': pad spaces on the right
  Right-justify  : pad spaces (or '0') on the left
         |
         v
  FINAL TEXT STRING → written to output
```

---

## 4. va_list and the Calling Convention

Format specifiers work because of **variadic functions** and the `va_list` mechanism. This is the
engine that drives all format functions. Misunderstanding it is the root cause of most
format-string undefined behavior.

### 4.1 How variadic arguments are passed (x86-64 System V ABI)

On 64-bit Linux / macOS, the System V AMD64 ABI defines:

- Integer/pointer arguments: pass in registers RDI, RSI, RDX, RCX, R8, R9 (first 6)
- Floating-point arguments: pass in XMM0–XMM7 (first 8)
- Remaining arguments: pushed onto the stack right-to-left

```
printf("x=%d y=%d z=%d\n", 10, 20, 30);

Register state at the call site:
  RDI = pointer to "x=%d y=%d z=%d\n"   (arg 0, the format string)
  RSI = 10                                (arg 1)
  RDX = 20                                (arg 2)
  RCX = 30                                (arg 3)
  AL  = 0                                 (number of XMM regs used, for variadics)

Stack layout (top = lower address):
  [return address]
  [caller's saved frame pointer]
  ...
```

```
Memory layout of va_list on x86-64 (gp_offset / fp_offset / reg_save_area):

  typedef struct {
      unsigned int  gp_offset;     // bytes used in GP register area
      unsigned int  fp_offset;     // bytes used in FP register area
      void         *overflow_arg;  // next stack argument
      void         *reg_save_area; // points to saved register area
  } va_list[1];

  reg_save_area (304 bytes):
  +--------+--------+--------+--------+--------+--------+
  | RDI(0) | RSI(8) |RDX(16) |RCX(24) | R8(32) | R9(40) | <- GP regs (48 bytes)
  +--------+--------+--------+--------+--------+--------+
  |XMM0(48)|XMM1(64)| ... |XMM7(160)|           <- FP regs (128 bytes)
  +--------+--------+-----+--------+

  va_arg(ap, int) on x86-64:
    1. if (ap.gp_offset < 48)             // still have GP register args
         ptr = ap.reg_save_area + ap.gp_offset
         ap.gp_offset += 8
       else                               // spilled to stack
         ptr = ap.overflow_arg
         ap.overflow_arg += 8
    2. return *(int*)ptr
```

### 4.2 va_list in practice — C code

```c
#include <stdarg.h>
#include <stdio.h>

/* Manually walk a va_list to sum integers */
int sum_n(int count, ...) {
    va_list ap;
    va_start(ap, count);   /* initialize ap; count is last named param  */

    int total = 0;
    for (int i = 0; i < count; i++) {
        int v = va_arg(ap, int);   /* fetch next int-sized argument      */
        total += v;
    }

    va_end(ap);            /* mandatory: may zero out ap on some ABI     */
    return total;
}

int main(void) {
    printf("%d\n", sum_n(3, 10, 20, 30));  /* prints 60 */
    return 0;
}
```

### 4.3 Why mismatched types cause undefined behavior

```c
printf("%d\n", 3.14);  // UB: 3.14 is a double (8 bytes); %d reads an int (4 bytes)
                        // It reads the LOW 4 BYTES of the double's IEEE-754 repr
                        // On x86-64 the double was in XMM0, but %d reads GP register
                        // → reads garbage / uninitialized data from RSI
```

The format string and the actual arguments are processed independently. The compiler cannot
(portably) check them at runtime. This is why type mismatches are undefined behavior, not
just a wrong result.

---

## 5. Integer Specifiers

### 5.1 Complete table

| Spec | Argument type  | Interpretation       | Notes                         |
|------|----------------|----------------------|-------------------------------|
| `%d` | `int`          | Signed decimal       | Most common                   |
| `%i` | `int`          | Signed decimal       | Same as `%d` for printf; different for scanf |
| `%u` | `unsigned int` | Unsigned decimal     |                               |
| `%o` | `unsigned int` | Unsigned octal       | `#` flag adds `0` prefix      |
| `%x` | `unsigned int` | Unsigned hex lower   | `#` flag adds `0x` prefix     |
| `%X` | `unsigned int` | Unsigned hex upper   | `#` flag adds `0X` prefix     |

### 5.2 How signed integers are converted — the algorithm

```c
/*
 * Conceptual implementation of %d conversion.
 * Real libc uses optimized algorithms; this shows the logic.
 */
#include <stdint.h>
#include <string.h>

static void int_to_decimal(char *buf, size_t bufsz, intmax_t value,
                            int width, int prec, int flags)
{
    char tmp[64];
    int  negative = (value < 0);
    uintmax_t uval = negative ? -(uintmax_t)value : (uintmax_t)value;
    int  pos = 0;

    /* 1. Convert absolute value to digits (least-significant first) */
    if (uval == 0) {
        tmp[pos++] = '0';
    } else {
        while (uval > 0) {
            tmp[pos++] = '0' + (uval % 10);
            uval /= 10;
        }
    }

    /* 2. Apply precision: pad with leading zeros to 'prec' digits */
    while (prec > 0 && pos < prec)
        tmp[pos++] = '0';

    /* 3. Reverse (we built digits in reverse order) */
    for (int l = 0, r = pos - 1; l < r; l++, r--) {
        char t = tmp[l]; tmp[l] = tmp[r]; tmp[r] = t;
    }
    tmp[pos] = '\0';

    /* 4. Determine sign character */
    char sign = 0;
    if (negative)              sign = '-';
    else if (flags & FLAG_PLUS) sign = '+';
    else if (flags & FLAG_SPACE) sign = ' ';

    /* 5. Compute total width, apply padding (see §10 Width and Precision) */
    int total = pos + (sign ? 1 : 0);
    /* ... padding logic here ... */
}
```

### 5.3 Signed vs unsigned — bit-level view

```
Value: -1 stored as int (32-bit two's complement)

Binary:  11111111 11111111 11111111 11111111

%d  reads it as signed   → -1
%u  reads it as unsigned → 4294967295
%o  reads it as unsigned → 37777777777
%x  reads it as unsigned → ffffffff
%X  reads it as unsigned → FFFFFFFF

 +--+--+--+--+--+--+--+--+  +--+--+--+--+--+--+--+--+
 |1 |1 |1 |1 |1 |1 |1 |1 |  |1 |1 |1 |1 |1 |1 |1 |1 |  ... (32 bits total)
 +--+--+--+--+--+--+--+--+  +--+--+--+--+--+--+--+--+
  ^
  Sign bit (bit 31): 1 = negative for %d; just a value bit for %u/%x/%o
```

### 5.4 Difference between %d and %i in scanf

For `printf`, `%d` and `%i` are identical. For `scanf`, they differ:

```c
int a, b;
sscanf("010 010", "%d %i", &a, &b);
//  %d: 010 is always parsed as decimal 10
//  %i: 010 is parsed as octal 8 (leading 0 = octal prefix)
printf("a=%d b=%d\n", a, b);  // a=10  b=8

// %i auto-detects base:
//   0x or 0X prefix → hexadecimal
//   0 prefix        → octal
//   otherwise       → decimal
```

### 5.5 Integer output examples with all specifiers

```c
#include <stdio.h>

int main(void) {
    int   n  = 255;
    int   neg = -42;
    unsigned u = 4294967295U;  /* UINT_MAX */

    printf("Dec:       %d\n",    n);    /* 255          */
    printf("Signed i:  %i\n",    n);    /* 255          */
    printf("Unsigned:  %u\n",    u);    /* 4294967295   */
    printf("Octal:     %o\n",    n);    /* 377          */
    printf("Octal #:   %#o\n",   n);    /* 0377         */
    printf("Hex lower: %x\n",    n);    /* ff           */
    printf("Hex upper: %X\n",    n);    /* FF           */
    printf("Hex #:     %#x\n",   n);    /* 0xff         */
    printf("Hex #:     %#X\n",   n);    /* 0XFF         */
    printf("Negative:  %d\n",    neg);  /* -42          */
    printf("Neg +flag: %+d\n",   neg);  /* -42 (sign already present) */
    printf("Pos +flag: %+d\n",   n);    /* +255         */
    return 0;
}
```

---

## 6. Floating-Point Specifiers

### 6.1 Complete table

| Spec | Form               | Default precision | Example (value=3.14159)   |
|------|--------------------|-------------------|---------------------------|
| `%f` | Decimal notation   | 6 digits          | `3.141590`                |
| `%e` | Scientific (lower) | 6 digits mantissa | `3.141590e+00`            |
| `%E` | Scientific (upper) | 6 digits mantissa | `3.141590E+00`            |
| `%g` | Shorter of f/e     | 6 sig digits      | `3.14159`                 |
| `%G` | Shorter of f/E     | 6 sig digits      | `3.14159`                 |
| `%a` | Hex float (lower)  | min digits needed | `0x1.921f9f01b866ep+1`    |
| `%A` | Hex float (upper)  | min digits needed | `0X1.921F9F01B866EP+1`    |

### 6.2 IEEE 754 double — memory layout

```
64-bit double (IEEE 754-2008):

Bit 63     Bits 62-52        Bits 51-0
  |            |                   |
  v            v                   v
 [S] [EEEEEEEEEEE] [MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM]
  1       11                              52

 S  = sign        (1 bit):   0=positive, 1=negative
 E  = exponent    (11 bits): biased exponent = actual_exp + 1023
 M  = mantissa    (52 bits): fractional part; implied leading 1

value = (-1)^S × 1.M × 2^(E-1023)    (normalized numbers)

3.14 in memory:
  sign = 0  (positive)
  actual exponent = 1 (3.14 ≈ 1.57 × 2^1)
  biased exponent = 1 + 1023 = 1024 = 10000000000b

  hex: 400 91EB8 51EB851F  (approx)

Special values:
  E = 0x7FF, M = 0  → ±Infinity
  E = 0x7FF, M ≠ 0  → NaN (quiet or signaling)
  E = 0,     M = 0  → ±0.0
  E = 0,     M ≠ 0  → Denormalized (subnormal) number
```

### 6.3 %f — decimal notation

`%f` converts the `double` to a string in fixed-point notation.
Precision specifies **digits after the decimal point** (default 6).

```c
printf("%.2f\n",    3.14159);   /* 3.14          */
printf("%.0f\n",    3.14159);   /* 3             */
printf("%f\n",      3.14159);   /* 3.141590      */
printf("%.10f\n",   3.14159);   /* 3.1415900000  */
printf("%f\n",      0.0);       /* 0.000000      */
printf("%f\n",     -0.0);       /* -0.000000     */
printf("%f\n",  1.0/0.0);       /* inf           */
printf("%f\n", -1.0/0.0);       /* -inf          */
printf("%f\n",  0.0/0.0);       /* -nan or nan   (implementation-defined) */
```

### 6.4 %e — scientific notation

Precision specifies digits after decimal in the mantissa. Exponent always has at least 2 digits.

```c
printf("%e\n",    12345.6789);  /* 1.234568e+04   */
printf("%.2e\n",  12345.6789);  /* 1.23e+04       */
printf("%E\n",    12345.6789);  /* 1.234568E+04   */
printf("%e\n",    0.00042);     /* 4.200000e-04   */
printf("%e\n",    0.0);         /* 0.000000e+00   */
```

### 6.5 %g — shortest representation

`%g` chooses between `%f` and `%e` based on the exponent:

```
Let P = precision (default 6, but at least 1)
Let E = exponent of the value when written as  X.XXXe+EE

If E < -4 OR E >= P  →  use %e format (with precision P-1 after decimal)
Otherwise            →  use %f format (with precision P - (E+1) after decimal)

Trailing zeros after decimal are REMOVED by %g (unless # flag is set).
Decimal point removed if no digits follow it.

Examples with default precision (6 significant digits):
  0.00001      E=-5 → E < -4 → %e:  1e-05
  0.0001       E=-4 → E >= -4 AND E < 6 → %f: 0.0001
  123456       E= 5 → E < 6 → %f:  123456
  1234567      E= 6 → E >= 6 → %e: 1.23457e+06
  3.14159      E= 0 → %f: 3.14159
  3.14159e10   E=10 → E >= 6 → %e: 3.14159e+10
```

```c
printf("%g\n",  0.00001);    /* 1e-05        */
printf("%g\n",  0.0001);     /* 0.0001       */
printf("%g\n",  100000.0);   /* 100000       */
printf("%g\n",  1000000.0);  /* 1e+06        */
printf("%g\n",  3.14159);    /* 3.14159      */
printf("%g\n",  3.10);       /* 3.1          (trailing zero removed) */
printf("%#g\n", 3.10);       /* 3.10000      (# flag keeps trailing zeros) */
```

### 6.6 %a — hexadecimal floating point

`%a` represents a `double` in hexadecimal scientific notation. It is exact — no rounding occurs
for values representable as IEEE 754 doubles.

```
Format:  [-]0xH.HHHHp±dd
           |  |  |    | |
           |  |  |    | decimal exponent of 2
           |  |  |    'p' separates mantissa from exponent
           |  |  fractional part of mantissa in hex
           |  leading hex digit (1 for normalized, 0 for subnormal)
           optional sign
```

```c
printf("%a\n",  1.0);       /* 0x1p+0          */
printf("%a\n",  2.0);       /* 0x1p+1          */
printf("%a\n",  0.5);       /* 0x1p-1          */
printf("%a\n",  3.14);      /* 0x1.91eb851eb852p+1  (exact!) */
printf("%a\n",  0.1);       /* 0x1.999999999999ap-4 */
                             /* 0.1 cannot be represented exactly;
                                the hex form shows the actual stored value */
```

### 6.7 Floating-point precision internals

```
How "%.2f" of 3.14159 is computed conceptually:

1. Parse the double to its decimal representation.
   3.14159 → integer part = 3, fractional part = 0.14159...

2. Round the fractional part to 'precision' decimal places.
   0.14159... rounded to 2 places:
     0.14159 × 100 = 14.159
     floor(14.159 + 0.5) = floor(14.659) = 14
   → 0.14

3. Recombine: "3.14"

4. Apply field width / flags.

Rounding mode: "round half to even" (banker's rounding) in C99+,
but many implementations use "round half away from zero".

printf("%.1f\n", 2.25);   /* Could be "2.2" or "2.3" depending on impl! */
printf("%.1f\n", 2.35);   /* Could be "2.3" or "2.4" depending on impl! */
```

---

## 7. Character and String Specifiers

### 7.1 %c — single character

`%c` takes an `int` argument, converts it to `unsigned char`, and writes that one character.

```c
printf("%c\n", 'A');      /* A        */
printf("%c\n", 65);       /* A        (65 is ASCII 'A') */
printf("%c\n", 0x1B);     /* ESC character */

/* Width works: */
printf("%5c\n",  'A');    /*     A    (right-justified, 4 spaces + A) */
printf("%-5c|\n",'A');    /* A    |   (left-justified) */
```

```
Memory model for %c:

int argument (32 bits):
  00000000 00000000 00000000 01000001   (value 65 = 'A')

%c takes only the LOW 8 BITS:
  01000001 → unsigned char → written as 'A'

This is why %c takes int, not char: default argument promotion
promotes char to int before passing to a variadic function.
```

### 7.2 %s — string

`%s` takes a `char *` (pointer to a null-terminated string) and writes characters until `'\0'`.

```
Width and precision with %s:
  width    → minimum field width; pad with spaces
  precision → MAXIMUM number of characters to print (does NOT require null-terminator
               within that range, unlike the rest of the string)

printf("%.3s\n", "Hello");    /* Hel     (precision truncates) */
printf("%10s\n", "Hello");    /* '     Hello'  (right-justified) */
printf("%-10s|\n","Hello");   /* 'Hello     |' (left-justified) */
printf("%10.3s\n","Hello");   /* '       Hel'  (both) */
```

```c
/* DANGER: passing NULL to %s is undefined behavior */
char *p = NULL;
printf("%s\n", p);    /* UB! Many implementations print "(null)" but not guaranteed */

/* DANGER: non-null-terminated string with no precision */
char buf[3] = {'H','i'};  /* no null terminator */
printf("%s\n", buf);       /* UB: reads beyond buf until random '\0' found */
printf("%.2s\n", buf);     /* OK: precision limits to 2 chars */
```

### 7.3 %lc and %ls — wide characters and strings

`%lc` takes a `wint_t` (wide character int). `%ls` takes a `wchar_t *`.
These convert through the current locale's wide-to-multibyte converter.

```c
#include <wchar.h>
#include <locale.h>

setlocale(LC_ALL, "en_US.UTF-8");
wprintf(L"%lc\n", L'α');    /* Prints α if terminal supports UTF-8 */
printf("%lc\n",   (wint_t)L'α');
```

---

## 8. Pointer Specifier

### 8.1 %p — print a pointer

`%p` takes a `void *` and prints the pointer value in an implementation-defined format.
On most systems, it prints a hexadecimal address with a `0x` prefix.

```c
int   x  = 42;
int  *px = &x;
void *vp = px;

printf("%p\n",  (void*)px);   /* e.g.  0x7ffd3a4c2e04  */
printf("%p\n",  (void*)0);    /* (nil) or 0x0 depending on impl */
```

```
Why cast to void*?
  %p is defined to take void*. Passing int* without the cast is
  technically implementation-defined (pointer types may differ in
  representation on exotic architectures like Harvard or 20-bit segmented).
  On modern flat-memory architectures it works, but the cast is correct C.
```

### 8.2 Pointer address space on 64-bit Linux

```
Virtual address space (x86-64 Linux, canonical addresses):

0x0000000000000000 ─────────────── NULL / unmapped
0x0000000000400000 ─────────────── .text (code) typically starts here
0x0000000000600000 ─────────────── .data, .bss (globals)
         ...
0x00007fff00000000 ─────────────── stack region top ≈ 0x00007fffffffffff
0xffff800000000000 ─────────────── kernel space (not accessible to user)

printf("%p\n", &main)    →  something like  0x400620
printf("%p\n", &x)       →  something like  0x7ffd3a4c2e04  (stack)
printf("%p\n", malloc(8))→  something like  0x55a3b4e022a0  (heap)
```

---

## 9. Special Specifiers — %% and %n

### 9.1 %% — literal percent sign

`%%` outputs a single `%` character. It takes no argument.

```c
printf("100%%\n");          /* 100%   */
printf("%d%%\n", 50);       /* 50%    */
```

### 9.2 %n — write character count

`%n` is the most unusual specifier. It does not print anything. Instead, it writes the number
of characters printed so far into the `int *` argument.

```c
int count;
printf("Hello%n, World!\n", &count);
/* After the call, count == 5 (characters printed before %n) */
printf("count=%d\n", count);    /* count=5 */
```

```
Timeline of printf("Hello%n, World!\n", &count):

  char by char:
    H  ← written, total=1
    e  ← written, total=2
    l  ← written, total=3
    l  ← written, total=4
    o  ← written, total=5
    %n ← *count = 5 (write 5 into count; nothing printed)
    ,  ← written, total=6
    ...
    \n ← written, total=14
  return 14
```

**Length variants of %n:**

| Specifier | Argument type | Description               |
|-----------|---------------|---------------------------|
| `%n`      | `int *`       | Stores character count     |
| `%hn`     | `short *`     | Stores into short          |
| `%hhn`    | `signed char *` | Stores into signed char  |
| `%ln`     | `long *`      | Stores into long           |
| `%lln`    | `long long *` | Stores into long long      |
| `%zn`     | `size_t *`    | Stores into size_t         |

> **Security warning**: `%n` is disabled by default in Microsoft's CRT and glibc's
> `printf_chk`. It is a vector for format string exploits (see §17).

---

## 10. Width and Precision

### 10.1 Width — minimum field width

Width specifies the **minimum** number of characters to output. If the converted value is
shorter, it is padded (default: with spaces on the left). If longer, the width is ignored —
output is never truncated by width.

```
printf("%5d\n",  42);    /*    42  ← 3 spaces + 2 digits = 5 chars */
printf("%5d\n", 123456); /* 123456 ← width exceeded; 6 chars output */
printf("%-5d|\n", 42);   /* 42   | ← left-justified, spaces on right */
printf("%05d\n", 42);    /* 00042  ← zero-padding flag */
```

```
Right-justify (default):          Left-justify (-flag):
  [  ][  ][  ][ 4][ 2]              [ 4][ 2][  ][  ][  ]
  +--+--+--+--+--+                  +--+--+--+--+--+
  |  |  |  | 4| 2|                  | 4| 2|  |  |  |
  +--+--+--+--+--+                  +--+--+--+--+--+
   1   2   3   4   5                  1   2   3   4   5
```

### 10.2 Dynamic width via *

If width is `*`, the width value is read from the next `int` argument (before the value to print).
Negative dynamic width is equivalent to the `-` flag + absolute width.

```c
printf("%*d\n",  10, 42);    /*          42  */
printf("%*d\n",  -10, 42);   /* 42           */  /* negative = left-justify */
printf("%*d\n",   0, 42);    /* 42  */            /* width 0 = no padding */

/* Useful for dynamic column widths: */
int col_width = 20;
printf("%-*s %s\n", col_width, "Name", "Age");
printf("%-*s %d\n", col_width, "Alice", 30);
```

### 10.3 Precision — meaning differs by specifier

```
+-----------+-----------------------------------------+-----------------------------+
| Specifier | Meaning of precision .N                 | Default if omitted          |
+-----------+-----------------------------------------+-----------------------------+
| %d %i     | Minimum number of digits (pad w/ zeros) | 1                           |
| %u %o     | Minimum number of digits                | 1                           |
| %x %X     | Minimum number of digits                | 1                           |
| %f %e %E  | Digits after decimal point              | 6                           |
| %g %G     | Significant digits total                | 6                           |
| %a %A     | Digits after hex point                  | Min needed for exact repr   |
| %s        | Maximum number of chars printed          | All (until null terminator) |
| %c        | Precision has no effect                 | —                           |
| %p        | Implementation-defined                  | —                           |
+-----------+-----------------------------------------+-----------------------------+
```

```c
/* Integer precision — adds LEADING zeros */
printf("%.8d\n", 255);     /* 00000255  — at least 8 digits */
printf("%.1d\n", 0);       /* 0         */
printf("%.0d\n", 0);       /* (empty!)  — "%.0d" of 0 = empty string! */

/* String precision — TRUNCATES */
printf("%.5s\n", "Hello, World!");   /* Hello */

/* Float precision */
printf("%.0f\n", 3.7);    /* 4         — 0 places, rounded */
printf("%.0f\n", 3.5);    /* 4         — round half to even (or away) */
printf("%.0f\n", 4.5);    /* 4 or 5    — implementation-defined rounding */
```

### 10.4 Interaction between width, precision, and length

```
For printf("%-10.5s|\n", "Hello, World!"):

  "Hello, World!" truncated to 5 chars → "Hello"
  Field width is 10, left-justified:
  "Hello     |"
   ^^^^^      ^
   5 chars    |
         ^^^^^
         5 padding spaces

For printf("%10.5s|\n", "Hello, World!"):
  "     Hello|"
   ^^^^^
   5 spaces padding
```

---

## 11. Flags

Flags modify the output format. Multiple flags can appear in any order.

| Flag    | Meaning                                                              |
|---------|----------------------------------------------------------------------|
| `-`     | Left-justify within field (default is right-justify)                 |
| `+`     | Always print sign (+ or -) for numeric conversions                   |
| ` `     | (space) Prefix positive numbers with a space (if no + flag)          |
| `#`     | Alternate form: `0` for octal, `0x`/`0X` for hex, always decimal point for floats |
| `0`     | Pad with zeros instead of spaces (for numeric conversions)           |

### 11.1 The `-` flag — left-justify

```c
printf("|%10d|\n",  42);   /* |        42| */
printf("|%-10d|\n", 42);   /* |42        | */
printf("|%-10s|\n", "hi"); /* |hi        | */

/* - overrides 0 when both given */
printf("|%-010d|\n", 42);  /* |42        | — - wins, no zero-pad */
```

### 11.2 The `+` flag — always show sign

```c
printf("%+d\n",  42);    /* +42  */
printf("%+d\n", -42);    /* -42  */
printf("%+f\n", 3.14);   /* +3.140000 */

/* + overrides space flag */
printf("%+ d\n", 42);    /* +42  (+ wins) */
```

### 11.3 The space flag — positive sign placeholder

```c
printf("% d\n",  42);    /*  42  (leading space) */
printf("% d\n", -42);    /* -42  (minus, not space) */

/* Useful for aligning columns that may contain positive and negative: */
printf("% d\n",  100);   /*  100 */
printf("% d\n", -200);   /* -200 */
printf("% d\n",   50);   /*   50 */
```

### 11.4 The `#` flag — alternate form

```c
printf("%#o\n",  255);    /* 0377     — octal prefix */
printf("%#x\n",  255);    /* 0xff     — hex prefix   */
printf("%#X\n",  255);    /* 0XFF     — uppercase hex prefix */
printf("%#f\n",  3.0);    /* 3.000000 — always decimal point */
printf("%#g\n",  3.0);    /* 3.00000  — trailing zeros kept */
printf("%#.0f\n", 3.0);   /* 3.       — decimal point even with 0 precision */

/* Note: #d and #i are undefined behavior / ignored */
printf("%#d\n",  255);    /* 255 (# has no defined meaning for %d) */
```

### 11.5 The `0` flag — zero padding

```c
printf("%05d\n",   42);   /* 00042    */
printf("%05d\n",  -42);   /* -0042    — sign before the zeros */
printf("%+05d\n",  42);   /* +0042    — sign before the zeros */
printf("%010.2f\n", 3.14);/* 0000003.14 */
printf("%010s\n", "hi");  /* 0 flag ignored for strings (spaces used) */

/* Zero padding with precision: precision overrides zero-flag for integers */
printf("%05.8d\n", 42);   /* 00000042 — precision (8) sets min digits;
                               width (5) already satisfied by 8 zeros */
```

### 11.6 Flags interaction diagram

```
Input: value = 42, format = "%+08.5d"

Step 1: Apply precision .5 → "00042"  (5 digits minimum)
Step 2: Apply + flag       → "+00042"
Step 3: Apply width 8, 0-flag: already 6 chars, need 8 → "+0000042"
  Wait! The sign came before the zero-padding...

Correct sequence:
  digits with precision: "00042"
  sign character: "+"
  zero-pad to width 8: total now is 6 (+00042), need 2 more zeros
  → "+0000042"

printf("%+08.5d\n", 42);   /* +0000042 */
printf("%+08.5d\n", -42);  /* -0000042 */
```

---

## 12. Length Modifiers

Length modifiers adjust the **size** of the argument that the conversion specifier reads.
Without the correct modifier, `va_arg` fetches the wrong number of bytes.

### 12.1 Complete length modifier table

```
+----------+-------------+--------------------------------------------------+
| Modifier | On integer  | On float  | On pointer/size                      |
+----------+-------------+-----------+--------------------------------------+
| (none)   | int/uint    | double    | —                                    |
| hh       | signed/unsigned char | — | —                                  |
| h        | short/ushort| —         | —                                    |
| l        | long/ulong  | double*   | wint_t (%lc) / wchar_t* (%ls)       |
| ll       | long long   | —         | —                                    |
| L        | —           | long double| —                                   |
| z        | size_t      | —         | —                                    |
| t        | ptrdiff_t   | —         | —                                    |
| j        | intmax_t /  | —         | —                                    |
|          | uintmax_t   |           |                                      |
+----------+-------------+-----------+--------------------------------------+

* 'l' on %f/%e/%g/%a has no effect (double is default for floats;
  long double needs 'L')
```

### 12.2 Why length modifiers exist — size matters

```
Default argument promotion rules (C11 §6.5.2.2):
  - char, short → promoted to int
  - float → promoted to double

So without a length modifier, ALL integer variadic args are treated as int/uint,
and ALL float variadic args are treated as double.

When you use a longer type, you MUST tell printf how big it is:

int       x = 10;          printf("%d",   x);    /* OK: int    */
long      l = 10L;         printf("%ld",  l);    /* OK: long   */
long long ll = 10LL;       printf("%lld", ll);   /* OK: long long */
size_t    s = 10;          printf("%zu",  s);    /* OK: size_t */
ptrdiff_t p = 10;          printf("%td",  p);    /* OK: ptrdiff_t */
intmax_t  m = 10;          printf("%jd",  m);    /* OK: intmax_t */

float     f = 1.5f;        printf("%f",   f);    /* OK: promoted to double */
double    d = 1.5;         printf("%f",   d);    /* OK: double */
long double ld = 1.5L;     printf("%Lf",  ld);   /* OK: long double */
```

### 12.3 What happens on mismatch — byte-level view

```c
long long bigval = 0x0000000DEADBEEFLL;  /* 64-bit value */
printf("%d\n", bigval);  /* WRONG: %d reads 32 bits, gets only DEADBEEF
                             or 0x0000000D depending on endianness + ABI */

/* x86-64, little-endian, System V ABI:
   bigval passed in RSI (64-bit register)
   %d reads 32-bit int from lower 32 bits of RSI

   RSI = 0x00000000DEADBEEF
         [  high 32  ] [  low 32   ]
          0x00000000    0xDEADBEEF

   %d reads 0xDEADBEEF as signed int → -559038737
   printf("%d\n", bigval);  might print: -559038737
*/
```

### 12.4 hh and h — sub-int types

Because `char` and `short` are promoted to `int` before passing to variadic functions,
`%hhd` and `%hd` don't change what is passed — they change how the received `int` is
interpreted/truncated before conversion.

```c
int   i   = 300;
short s   = 300;
char  c   = (char)300;  /* truncated to 44 (300 % 256) on most platforms */

printf("%d\n",   i);    /* 300  */
printf("%hd\n",  i);    /* 300  (short range; but value fits, so 300) */
printf("%hd\n",  400);  /* 400  — or possibly truncated to -112 if we view as short */
printf("%hhd\n", 200);  /* -56  (200 as signed char: 200 - 256 = -56) */
printf("%hhu\n", 200);  /* 200  (200 as unsigned char) */
printf("%hhx\n", 255);  /* ff   */

/* %hhd is most useful in scanf: */
signed char byte;
sscanf("200", "%hhd", &byte);   /* byte = -56 */
sscanf("200", "%hhu", (unsigned char *)&byte);  /* byte = 200 */
```

### 12.5 z, t, j — portable specifiers for standard types

These were added in C99 to handle types whose sizes vary by platform.

```c
#include <stdint.h>
#include <stddef.h>
#include <inttypes.h>

size_t    n = sizeof(long);
ptrdiff_t d = ptr2 - ptr1;
intmax_t  m = INTMAX_MAX;

/* CORRECT */
printf("sizeof(long) = %zu\n",   n);   /* %z for size_t       */
printf("ptr diff     = %td\n",   d);   /* %t for ptrdiff_t    */
printf("intmax       = %jd\n",   m);   /* %j for intmax_t     */
printf("uintmax      = %ju\n",  (uintmax_t)UINTMAX_MAX);

/* WRONG on 64-bit (size_t = unsigned long = 8 bytes; %u = 4 bytes) */
printf("sizeof(long) = %u\n",   (unsigned)n);  /* truncates on 64-bit */
```

### 12.6 The <inttypes.h> macros for fixed-width types

C99 provides macros for `int32_t`, `int64_t`, etc. because the correct specifier depends
on the platform.

```c
#include <stdint.h>
#include <inttypes.h>

int32_t  a = 100;
int64_t  b = 1LL << 40;
uint64_t c = UINT64_MAX;

/* PORTABLE: */
printf("%" PRId32 "\n", a);   /* %d on 32-bit platforms where int32_t=int */
printf("%" PRId64 "\n", b);   /* %ld on Linux 64-bit, %lld on Windows     */
printf("%" PRIu64 "\n", c);   /* %lu or %llu depending on platform        */

/* Common macros: */
/* PRId8,  PRId16,  PRId32,  PRId64  — signed decimal   */
/* PRIu8,  PRIu16,  PRIu32,  PRIu64  — unsigned decimal */
/* PRIx8,  PRIx16,  PRIx32,  PRIx64  — unsigned hex     */
/* PRIo8,  PRIo16,  PRIo32,  PRIo64  — unsigned octal   */
/* SCNd32, SCNd64, SCNu32, SCNu64    — for scanf         */
```

---

## 13. scanf — Input Parsing Deep Dive

`scanf` shares the format specifier syntax with `printf` but has important differences.
Misunderstanding `scanf` is one of the most common sources of bugs in C programs.

### 13.1 Key differences from printf

| Aspect            | printf                     | scanf                              |
|-------------------|----------------------------|------------------------------------|
| Arguments         | Values to format           | **Pointers** to store results      |
| Width             | Minimum output width       | **Maximum** characters to consume  |
| Precision         | Output precision           | **Not supported** (ignored/UB)     |
| `%s`              | Prints a string            | Reads non-whitespace into a buffer |
| `%c`              | Prints one char            | Reads one char (including space)   |
| `%i`              | Same as `%d`               | Detects base: 0x=hex, 0=octal     |
| Return value      | Characters written         | Items successfully assigned        |
| `*` modifier      | Dynamic width from args    | **Assignment suppression**         |

### 13.2 How scanf processes input

```
scanf("%d %f %s", &i, &f, str):

INPUT STREAM: "  42  3.14  hello  \n"
               ^^
               leading whitespace
               |
               v
[WHITESPACE SKIP]  ← most specifiers skip leading whitespace automatically
                     exceptions: %c, %[, %n (these do NOT skip whitespace)

[MATCH %d]:
  skip whitespace
  read chars '4','2'
  stop at ' ' (not a digit)
  convert "42" to int, store at &i
  i = 42

[MATCH literal ' ']:
  one or more whitespace characters in format string
  matches zero or more whitespace in input
  → input "  3.14..." matches fine

[MATCH %f]:
  skip whitespace
  read chars '3','.','1','4'
  stop at ' '
  convert "3.14" to float, store at &f
  f = 3.140000

[MATCH literal ' ']:
  skip whitespace

[MATCH %s]:
  skip whitespace
  read chars 'h','e','l','l','o'
  stop at ' ' (whitespace terminates %s)
  null-terminate and store at str
  str = "hello"

return 3   (three items assigned)
```

### 13.3 The return value of scanf

```c
int   n;
float f;
char  s[64];

int ret = scanf("%d %f %s", &n, &f, s);

/* ret == 3: all three items assigned successfully
   ret == 1: only %d matched; %f failed
   ret == 0: nothing matched
   ret == EOF: end-of-file before any item matched */

/* ALWAYS check the return value: */
if (ret != 3) {
    fprintf(stderr, "Parsing failed at item %d\n", ret + 1);
}
```

### 13.4 Width in scanf — limits input length

```c
char buf[8];
scanf("%7s", buf);    /* reads at most 7 chars + writes null terminator
                         total buffer size needed: 8 */

/* DANGEROUS — buffer overflow: */
scanf("%s", buf);     /* reads unlimited chars! If user types >7, overflow */

/* %s WITH width is the safe form: always use width with %s in scanf */
scanf("%7s", buf);    /* safe */
```

### 13.5 %c in scanf — does NOT skip whitespace

```c
char ch;

/* After reading a number, a newline remains in the buffer: */
int n;
scanf("%d", &n);
scanf("%c", &ch);    /* reads '\n', not the next meaningful character */

/* Fix: add a space before %c to skip whitespace: */
scanf("%d", &n);
scanf(" %c", &ch);   /* space in format string skips whitespace including \n */
```

### 13.6 %[ — scanset / character class

`%[...]` reads characters that are members of the specified set. It is unique to `scanf`.

```c
char buf[64];

/* Read only alphabetic characters: */
scanf("%63[a-zA-Z]", buf);

/* Read until newline (like fgets but with scanf): */
scanf("%63[^\n]", buf);

/* Read only digits: */
scanf("%63[0-9]", buf);

/* Read only non-space characters (similar to %s but keeps internal spaces...
   actually %s stops at any whitespace): */
scanf("%63[^ \t\n]", buf);   /* stops only at space, tab, newline */

/* Negate with ^ at start: read everything EXCEPT the listed chars */
scanf("%63[^,]", buf);       /* read until comma */
```

```
%[ rules:
  %[abc]   → matches 'a', 'b', or 'c'
  %[a-z]   → matches lowercase letters (range)
  %[^abc]  → matches anything except 'a', 'b', 'c'
  %[]]     → ] as first char = literal ']' in set
  %[^]]    → matches anything except ']'
  %[]a]    → matches ']' and 'a'
  Always null-terminate the result.
```

### 13.7 Assignment suppression with *

```c
int year, day;
char month[16];

/* "15 January 2024" → we want day and year but skip month */
sscanf("15 January 2024", "%d %*s %d", &day, &year);
/*                             ^^^
                               * suppresses assignment; January is consumed
                               but not stored anywhere */
printf("%d %d\n", day, year);  /* 15 2024 */
```

### 13.8 %n in scanf

Just like in `printf`, `%n` in `scanf` writes the number of characters consumed so far.
Useful for error recovery:

```c
int n, consumed;
sscanf("   42rest", " %d%n", &n, &consumed);
printf("n=%d, consumed=%d\n", n, consumed);
/* n=42, consumed=5  (3 spaces + "42") */
/* "rest" remains in string for next parsing */
```

---

## 14. Type Promotion and Default Argument Promotions

This section explains the automatic type widening that happens with variadic functions, which
directly affects which format specifiers are correct.

### 14.1 Integer promotions

C performs **integer promotion** on `char`, `short`, `_Bool`, and bit-fields before any
expression. For variadic functions, this means:

```
Type passed      →   Type actually received by variadic function
──────────────────────────────────────────────────────────────
char             →   int   (sign-extended if signed char)
unsigned char    →   int   (zero-extended)
short            →   int   (sign-extended)
unsigned short   →   int or unsigned int (platform-dependent)
int              →   int
unsigned int     →   unsigned int
long             →   long
unsigned long    →   unsigned long
long long        →   long long
unsigned long long → unsigned long long
float            →   double   ← important!
double           →   double
long double      →   long double
```

### 14.2 Float promotion — why there is no %f for float

```c
float fval = 1.5f;
printf("%f\n", fval);   /* CORRECT! fval is promoted to double before call.
                            %f reads a double. This works. */

/* There is NO "%hf" for float. float is always promoted to double.
   If you truly need to distinguish, use %a which shows exact bits. */

double dval = 1.5;
printf("%f\n",  dval);  /* %f: double — correct */
printf("%Lf\n", dval);  /* WRONG: %Lf reads long double; dval is double
                            → reads 80-bit or 128-bit = garbage */

long double ldval = 1.5L;
printf("%Lf\n", ldval); /* CORRECT: %Lf for long double */
```

### 14.3 Memory size chart on common platforms

```
                     ILP32        LP64        LLP64
                    (32-bit)   (Linux/Mac)  (Windows 64)
Type                 bytes       bytes        bytes
──────────────────  ──────────  ──────────  ──────────
char                   1           1            1
short                  2           2            2
int                    4           4            4
long                   4           8            4     ← differs!
long long              8           8            8
void*                  4           8            8
size_t                 4           8            8
ptrdiff_t              4           8            8
float                  4           4            4
double                 8           8            8
long double           12/16        16           8     ← often 80-bit on x87
```

---

## 15. Undefined Behavior and Common Pitfalls

### 15.1 Mismatched type

```c
/* UB: Passing double, reading int */
printf("%d\n", 3.14);    /* reads the low bytes of xmm0 as int */

/* UB: Passing int, reading double */
double d;
sscanf("42", "%lf", &d);   /* OK: %lf reads double */
scanf("%f",  &d);           /* WRONG: %f reads float, but &d is double* */

/* UB: Missing length modifier */
long n = 100000L;
printf("%d\n", n);          /* OK on LP64 if long==int size, UB on LLP64 */
printf("%ld\n", n);         /* ALWAYS correct */
```

### 15.2 Missing arguments

```c
printf("%d %d\n", 42);   /* UB: second %d has no argument
                             reads whatever is in RSI/stack beyond 42
                             — could print garbage, crash, or anything */
```

### 15.3 Writing through NULL or bad pointer with scanf

```c
int *p = NULL;
scanf("%d", p);      /* UB: writes through NULL pointer → segfault */

/* Most common beginners' bug: forgetting & */
int n;
scanf("%d", n);      /* UB: passes value of n (uninitialized) as pointer */
scanf("%d", &n);     /* CORRECT */
```

### 15.4 Buffer overflow with %s

```c
char buf[10];
scanf("%s", buf);           /* DANGER: unlimited input */
scanf("%9s", buf);          /* SAFE: max 9 chars + null terminator */

gets(buf);                  /* NEVER USE gets() — it's been removed in C11 */
fgets(buf, sizeof(buf), stdin); /* SAFE alternative */
```

### 15.5 Signed/unsigned mismatch

```c
unsigned int u = UINT_MAX;   /* 4294967295 */
printf("%d\n", u);           /* Implementation-defined: likely prints -1
                                 on 2's complement, but UB per the standard */
printf("%u\n", u);           /* CORRECT: 4294967295 */

int i = -1;
printf("%u\n", i);           /* Implementation-defined: likely 4294967295 */
printf("%d\n", i);           /* CORRECT: -1 */
```

### 15.6 Precision trap with integers

```c
/* %.0d of 0 produces EMPTY STRING — common surprise */
printf("[%.0d]\n", 0);    /* [] — empty brackets */
printf("[%.0d]\n", 1);    /* [1] */
printf("[%d]\n",   0);    /* [0] */
```

### 15.7 The `%n` suppressor

Some security-hardened environments (`_FORTIFY_SOURCE`, MSVC) disable `%n` at runtime:

```c
int count;
printf("hello%n", &count);  /* May abort with: "*** %n in writable segment detected ***" */
```

### 15.8 Compiler warnings

Always compile with:
```
gcc -Wall -Wextra -Wformat=2 -Wformat-overflow -Wformat-truncation
```

`-Wformat=2` enables format string checking:
```
printf("%d\n", 3.14);     /* warning: format '%d' expects type 'int' */
printf("%s\n", 42);       /* warning: format '%s' expects type 'char *' */
```

---

## 16. Building a Minimal printf from Scratch

Implementing a minimal `printf` reveals every internal mechanism. This implementation handles
`%d`, `%s`, `%c`, `%x`, `%%`, width, and left-justify flag.

```c
#include <stdarg.h>
#include <unistd.h>  /* write() */
#include <string.h>  /* strlen() */
#include <stdint.h>

/* ─────────────────────── low-level output ─────────────────────── */

static void emit_char(char c) {
    write(STDOUT_FILENO, &c, 1);
}

static void emit_str(const char *s, int len) {
    write(STDOUT_FILENO, s, len);
}

/* ─────────────────────── integer → string ─────────────────────── */

/* Converts unsigned value 'v' in given 'base' into 'buf' (reversed).
   Returns number of characters. */
static int uint_to_str(uintmax_t v, int base, int uppercase,
                        char *buf, int bufsz)
{
    const char *digits_lower = "0123456789abcdef";
    const char *digits_upper = "0123456789ABCDEF";
    const char *digits = uppercase ? digits_upper : digits_lower;

    if (v == 0) { buf[0] = '0'; return 1; }

    int len = 0;
    while (v > 0 && len < bufsz - 1) {
        buf[len++] = digits[v % base];
        v /= base;
    }
    /* reverse */
    for (int l = 0, r = len - 1; l < r; l++, r--) {
        char t = buf[l]; buf[l] = buf[r]; buf[r] = t;
    }
    buf[len] = '\0';
    return len;
}

/* ─────────────────────── padding helper ─────────────────────── */

static void pad(char c, int n) {
    for (int i = 0; i < n; i++) emit_char(c);
}

/* ─────────────────────── core dispatcher ─────────────────────── */

/*
 * Parsed specifier state.
 */
typedef struct {
    int  flag_minus;   /* '-' left-justify       */
    int  flag_zero;    /* '0' zero-pad            */
    int  flag_plus;    /* '+' always show sign    */
    int  width;        /* minimum field width     */
    int  prec;         /* precision (-1 = default)*/
} Fmt;

static void do_string(const char *s, Fmt *f) {
    if (!s) s = "(null)";
    int slen = (int)strlen(s);
    if (f->prec >= 0 && slen > f->prec) slen = f->prec;

    int pad_count = (f->width > slen) ? f->width - slen : 0;

    if (!f->flag_minus) pad(' ', pad_count);
    emit_str(s, slen);
    if ( f->flag_minus) pad(' ', pad_count);
}

static void do_int(intmax_t value, int base, int upper, int is_signed, Fmt *f) {
    char       buf[64];
    int        negative = is_signed && (value < 0);
    uintmax_t  uval     = negative ? -(uintmax_t)value : (uintmax_t)value;
    char       sign     = negative ? '-' : (f->flag_plus ? '+' : 0);

    int dlen = uint_to_str(uval, base, upper, buf, sizeof(buf));

    /* precision: min digits */
    int prec_pad = (f->prec > dlen) ? f->prec - dlen : 0;

    /* total content width */
    int total = (sign ? 1 : 0) + prec_pad + dlen;
    int field_pad = (f->width > total) ? f->width - total : 0;

    char pad_char = (f->flag_zero && f->prec < 0 && !f->flag_minus) ? '0' : ' ';

    if (!f->flag_minus) {
        if (pad_char == '0') {
            if (sign) emit_char(sign);
            pad('0', field_pad);
        } else {
            pad(' ', field_pad);
            if (sign) emit_char(sign);
        }
    } else {
        if (sign) emit_char(sign);
    }

    pad('0', prec_pad);     /* precision zeros */
    emit_str(buf, dlen);    /* digits */

    if (f->flag_minus) pad(' ', field_pad);
}

/* ─────────────────────── main mini_printf ─────────────────────── */

int mini_printf(const char *fmt, ...) {
    va_list ap;
    va_start(ap, fmt);

    int written = 0;

    while (*fmt) {
        if (*fmt != '%') {
            emit_char(*fmt++);
            written++;
            continue;
        }
        fmt++;  /* skip '%' */

        /* ── parse flags ── */
        Fmt f = {0, 0, 0, 0, -1};
        while (1) {
            if      (*fmt == '-') { f.flag_minus = 1; fmt++; }
            else if (*fmt == '0') { f.flag_zero  = 1; fmt++; }
            else if (*fmt == '+') { f.flag_plus  = 1; fmt++; }
            else break;
        }

        /* ── parse width ── */
        while (*fmt >= '0' && *fmt <= '9')
            f.width = f.width * 10 + (*fmt++ - '0');

        /* ── parse precision ── */
        if (*fmt == '.') {
            fmt++;
            f.prec = 0;
            while (*fmt >= '0' && *fmt <= '9')
                f.prec = f.prec * 10 + (*fmt++ - '0');
        }

        /* ── dispatch on specifier ── */
        switch (*fmt++) {
            case 'd': case 'i': {
                intmax_t v = va_arg(ap, int);
                do_int(v, 10, 0, 1, &f);
                break;
            }
            case 'u': {
                uintmax_t v = (uintmax_t)va_arg(ap, unsigned int);
                do_int((intmax_t)v, 10, 0, 0, &f);
                break;
            }
            case 'x': {
                uintmax_t v = (uintmax_t)va_arg(ap, unsigned int);
                do_int((intmax_t)v, 16, 0, 0, &f);
                break;
            }
            case 'X': {
                uintmax_t v = (uintmax_t)va_arg(ap, unsigned int);
                do_int((intmax_t)v, 16, 1, 0, &f);
                break;
            }
            case 'o': {
                uintmax_t v = (uintmax_t)va_arg(ap, unsigned int);
                do_int((intmax_t)v, 8, 0, 0, &f);
                break;
            }
            case 's': {
                const char *s = va_arg(ap, const char *);
                do_string(s, &f);
                break;
            }
            case 'c': {
                char c = (char)va_arg(ap, int);
                char tmp[2] = {c, 0};
                do_string(tmp, &f);
                break;
            }
            case 'p': {
                uintmax_t v = (uintmax_t)(uintptr_t)va_arg(ap, void *);
                emit_str("0x", 2);
                do_int((intmax_t)v, 16, 0, 0, &f);
                break;
            }
            case '%':
                emit_char('%');
                break;
            case 'n': {
                int *p = va_arg(ap, int *);
                *p = written;
                break;
            }
            default:
                emit_char('?');
                break;
        }
    }

    va_end(ap);
    return written;
}

/* ─────────────────────── test driver ─────────────────────── */

int main(void) {
    mini_printf("Hello, World!\n");
    mini_printf("%d\n", 42);
    mini_printf("%d\n", -42);
    mini_printf("%05d\n", 42);
    mini_printf("%-10s|\n", "left");
    mini_printf("%10s|\n", "right");
    mini_printf("%x\n", 255);
    mini_printf("%X\n", 255);
    mini_printf("%+d %+d\n", 42, -42);
    mini_printf("%.5d\n", 42);
    mini_printf("%10.5d\n", 42);
    return 0;
}
```

### 16.1 Execution trace for `mini_printf("%05d\n", 42)`

```
mini_printf("%05d\n", 42)

fmt points to: '%','0','5','d','\n','\0'

Iteration 1:
  *fmt == '%' → enter format specifier parsing
  fmt++ → points to '0','5','d','\n','\0'

  parse flags:
    *fmt == '0' → flag_zero = 1, fmt++ → points to '5','d','\n','\0'
    no more flags

  parse width:
    *fmt == '5' → width = 5, fmt++ → points to 'd','\n','\0'

  parse precision:
    *fmt != '.' → prec = -1 (default)

  dispatch:
    *fmt == 'd' → fmt++ → points to '\n','\0'
    v = va_arg(ap, int) = 42

  do_int(42, 10, 0, 1, &f):
    negative = 0
    uval = 42
    sign = 0  (no flag_plus, not negative)
    dlen = uint_to_str(42, 10, ...) = 2, buf = "42"
    prec_pad = 0  (prec=-1)
    total = 0 + 0 + 2 = 2
    field_pad = 5 - 2 = 3
    pad_char = '0'  (flag_zero=1, prec<0, not flag_minus)
    Output: pad '0' three times → "000"
    Output: "42"
    Result: "00042"

Iteration 2:
  *fmt == '\n' → not '%' → emit '\n'

Output: "00042\n"
```

---

## 17. Format String Vulnerabilities

Format string attacks are a class of memory-safety exploits that arise when untrusted
user data is passed directly as the format string argument.

### 17.1 The root cause

```c
/* VULNERABLE: user-controlled format string */
char user_input[256];
fgets(user_input, sizeof(user_input), stdin);
printf(user_input);           /* ← DO NOT DO THIS */

/* SAFE: */
printf("%s", user_input);     /* user_input is treated as data, not format */
```

### 17.2 Information disclosure via %x

```c
/* If the attacker passes: "%x %x %x %x %x %x %x %x"
   printf pops arguments from the stack/registers that were NEVER provided.
   This leaks stack memory: return addresses, local variables, canaries. */

printf("%x %x %x %x");
/* Output might be: bffff23c 41414141 bffff600 400520
                    ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                    raw memory contents leaked */
```

### 17.3 Arbitrary write via %n

```c
/* Attacker-crafted format: "AAAA%134513824x%n"
   This:
   1. Pads output to exactly 134513828 characters total
   2. %n writes that count (134513828 = 0x0804A044) into the address
      that happens to be on the stack

   This can overwrite any memory address the process can write to:
   - Return addresses → control flow hijack
   - GOT entries      → library function redirection
   - Function pointers → direct code execution

   Protection mitigations:
   - FORTIFY_SOURCE: enables __printf_chk() which prohibits %n from
     a format string in writable memory
   - Stack canaries: detect stack corruption
   - ASLR: randomizes addresses (makes targeting harder)
   - Full RELRO: makes GOT read-only
*/
```

### 17.4 Safe coding rules

```c
/* Rule 1: Never use printf(user_string) */
printf("%s", user_string);       /* ALWAYS add %s */

/* Rule 2: Use snprintf with size limits */
char buf[128];
snprintf(buf, sizeof(buf), "User: %s, Age: %d", name, age);

/* Rule 3: Check snprintf truncation */
int ret = snprintf(buf, sizeof(buf), fmt, ...);
if (ret < 0 || ret >= (int)sizeof(buf)) {
    /* truncation occurred or error */
}

/* Rule 4: Use compile-time format checking */
/* GCC/Clang attribute: */
__attribute__((format(printf, 1, 2)))
void my_log(const char *fmt, ...);

/* Rule 5: On MSVC, use safe variants */
printf_s("%s\n", s);    /* %n not allowed, NULL checked */
scanf_s("%s", buf, (unsigned)sizeof(buf));
```

---

## 18. Quick Reference Card

### Output format specifiers (printf family)

```
╔══════╦══════════════════╦═══════════════════════════════════════════════╗
║ Spec ║ Argument type    ║ Output format                                 ║
╠══════╬══════════════════╬═══════════════════════════════════════════════╣
║  %d  ║ int              ║ Signed decimal integer                        ║
║  %i  ║ int              ║ Signed decimal integer (same as %d)           ║
║  %u  ║ unsigned int     ║ Unsigned decimal integer                      ║
║  %o  ║ unsigned int     ║ Unsigned octal integer                        ║
║  %x  ║ unsigned int     ║ Unsigned lowercase hexadecimal                ║
║  %X  ║ unsigned int     ║ Unsigned uppercase hexadecimal                ║
║  %f  ║ double           ║ Decimal floating point (default .6)           ║
║  %e  ║ double           ║ Scientific notation lowercase (1.2e+03)       ║
║  %E  ║ double           ║ Scientific notation uppercase (1.2E+03)       ║
║  %g  ║ double           ║ Shorter of %f or %e; strips trailing zeros    ║
║  %G  ║ double           ║ Shorter of %f or %E; strips trailing zeros    ║
║  %a  ║ double           ║ Hexadecimal floating point lowercase           ║
║  %A  ║ double           ║ Hexadecimal floating point uppercase           ║
║  %c  ║ int (char)       ║ Single character                              ║
║  %s  ║ char *           ║ Null-terminated string                        ║
║ %lc  ║ wint_t           ║ Wide character                                ║
║ %ls  ║ wchar_t *        ║ Wide string                                   ║
║  %p  ║ void *           ║ Pointer address (0x...)                       ║
║  %%  ║ (none)           ║ Literal % character                           ║
║  %n  ║ int *            ║ Store chars-written count (nothing output)    ║
╚══════╩══════════════════╩═══════════════════════════════════════════════╝
```

### Flags

```
╔══════╦════════════════════════════════════════════════════════════════╗
║ Flag ║ Effect                                                         ║
╠══════╬════════════════════════════════════════════════════════════════╣
║  -   ║ Left-justify (pad on right instead of left)                   ║
║  +   ║ Always show sign (+ or -)                                     ║
║  ' ' ║ Prefix positive numbers with space (if no + flag)             ║
║  #   ║ Alternate: 0 for %o, 0x for %x, 0X for %X, always dot for %f ║
║  0   ║ Zero-pad numbers on the left (overridden by - flag)           ║
╚══════╩════════════════════════════════════════════════════════════════╝
```

### Length modifiers

```
╔════════╦═══════════════╦════════════════════════════════════════════╗
║ Mod    ║ Integer type  ║ Notes                                      ║
╠════════╬═══════════════╬════════════════════════════════════════════╣
║ (none) ║ int / uint    ║ double for floats                          ║
║  hh    ║ (u)char       ║ C99+                                       ║
║  h     ║ (u)short      ║                                            ║
║  l     ║ (u)long       ║ wint_t/%lc, wchar_t*/%ls                  ║
║  ll    ║ (u)long long  ║ C99+                                       ║
║  L     ║ (float only)  ║ long double                                ║
║  z     ║ size_t        ║ C99+                                       ║
║  t     ║ ptrdiff_t     ║ C99+                                       ║
║  j     ║ intmax_t      ║ C99+                                       ║
╚════════╩═══════════════╩════════════════════════════════════════════╝
```

### Common scanf-specific behaviors

```
╔═══════╦══════════════════════════════════════════════════════════════╗
║ Spec  ║ scanf behavior                                               ║
╠═══════╬══════════════════════════════════════════════════════════════╣
║  %d   ║ Decimal only; ignores base prefix                            ║
║  %i   ║ Auto-detects base: 0x=hex, 0=octal, else decimal            ║
║  %s   ║ Reads non-whitespace; ALWAYS use width: %Ns                 ║
║  %c   ║ Reads one char INCLUDING whitespace; use " %c" to skip WS   ║
║  %[…] ║ Reads characters in set; %[^…] reads chars NOT in set       ║
║  %*d  ║ * suppresses assignment: reads and discards                  ║
║  %n   ║ Writes chars consumed so far; no input consumed             ║
╚═══════╩══════════════════════════════════════════════════════════════╝
```

### Minimum buffer sizes for integer conversion (in decimal)

```
Type            Max decimal digits  Recommended buf
───────────────────────────────────────────────────
char            4   (-128)          8
short           6   (-32768)        8
int             11  (-2147483648)   16
long (64-bit)   20  (-9223372036854775808)  24
long long       20  (-9223372036854775808)  24
unsigned 64     20  (18446744073709551615)  24
```

### snprintf return value and safe usage pattern

```c
/* snprintf ALWAYS null-terminates (if buf is non-NULL and n > 0).
   Return value is the number of characters that WOULD have been written
   if buf were large enough (excluding null terminator).
   If ret >= n: truncation occurred.
   If ret < 0: encoding error. */

char buf[64];
int ret = snprintf(buf, sizeof(buf), "x=%d y=%d", x, y);
if (ret < 0) {
    /* encoding error */
} else if (ret >= (int)sizeof(buf)) {
    /* output was truncated; buf contains first 63 chars + '\0' */
} else {
    /* buf contains the complete string, ret chars long */
}
```

---

*Guide covers C11 (ISO/IEC 9899:2011) format specifiers. C99 introduced `hh`, `ll`, `z`, `t`, `j`,
`%a`/`%A`, and `%F`. C23 adds `%b` for binary integers. Always compile with `-std=c11` or later
and enable `-Wformat=2` to catch mismatches at compile time.*
