# Rust Format Specifiers — Complete & Comprehensive Guide

> **Summary:** Rust's formatting system is a zero-cost, compile-time–verified DSL embedded in string literals. Every `{}` placeholder is resolved at compile time into a monomorphized call chain through `std::fmt` traits, `Formatter`, and `Arguments`. Unlike C's `printf`, there are no runtime format-string vulnerabilities — the compiler rejects ill-formed specifiers and type mismatches. Understanding the full machinery — from the raw syntax through the trait dispatch, the `Formatter` state machine, the `format_args!` ABI, and custom implementations — gives you precise mental models for performance, correctness, and extensibility.

---

## Table of Contents

1. [Architecture: How Formatting Works Under the Hood](#1-architecture)
2. [The Format String Syntax Grammar](#2-grammar)
3. [All Format Traits and Their Specifiers](#3-traits)
4. [Fill, Alignment, Width, Precision](#4-fill-align-width-precision)
5. [Flags: `+`, `-`, `0`, `#`, space](#5-flags)
6. [Positional and Named Arguments](#6-positional-named)
7. [Dynamic Width and Precision](#7-dynamic)
8. [Escaping Braces](#8-escaping)
9. [The `Formatter` Struct — Internals](#9-formatter)
10. [All Formatting Macros](#10-macros)
11. [The `format_args!` ABI and `Arguments<'_>`](#11-format-args)
12. [Implementing Custom `Display` and `Debug`](#12-custom-impl)
13. [Implementing All Other `fmt` Traits](#13-other-traits)
14. [The `Write` Trait](#14-write-trait)
15. [Padding, Sign, and Alternate Forms — Deep Dive](#15-deep-dive)
16. [Debug vs Display — Semantic Contract](#16-debug-display)
17. [Pretty-Print `{:#?}` and Derive Internals](#17-pretty-print)
18. [Performance: Zero-Cost Abstractions and Pitfalls](#18-performance)
19. [Advanced: `impl fmt::Display` for Wrappers (Newtype Pattern)](#19-newtype)
20. [Advanced: Conditional Formatting](#20-conditional)
21. [Advanced: Recursive and Nested Formatting](#21-recursive)
22. [Advanced: `format_args!` for Allocation-Free Logging](#22-alloc-free)
23. [Common Mistakes and Edge Cases](#23-mistakes)
24. [Threat Model and Security Properties](#24-security)
25. [Complete Reference Table](#25-reference-table)
26. [Next 3 Steps](#26-next-steps)

---

## 1. Architecture

### 1.1 Compile-Time Machinery

```
SOURCE CODE
  format!("Hello {name:>10.5} = {val:#010x}", name=s, val=v)
         │
         ▼
  rustc: parse format string at compile time
         │
         ├─ Validate specifiers, argument count, types
         ├─ Resolve each {} to a fmt::Trait call
         └─ Emit: format_args!(...) → core::fmt::Arguments<'_>
                                           │
                                           ▼
                              monomorphized dispatch table
                              [fn(&dyn Any, &mut Formatter) -> Result]
                                           │
                                           ▼
                              std::fmt::write(output, args)
                                           │
                              ┌────────────┴────────────┐
                              │                         │
                        Formatter                  Write impl
                        (state machine)            (String / File /
                         fill/align/width/          BufWriter / etc.)
                         precision/flags
```

### 1.2 Runtime Call Graph

```
format!("x={:08b}", val)
    │
    └─► format_args!("x={:08b}", val)
              │
              └─► core::fmt::Arguments {
                      pieces:    &["x="],          ← static string slices
                      fmt:       &[rt::Argument],  ← format specs
                      args:      &[rt::Argument],  ← actual values
                  }
                      │
                      └─► std::fmt::write(&mut output, args)
                                │
                                ├─ write static piece "x="
                                └─ dispatch: fmt::Binary::fmt(&val, &mut formatter)
                                                │
                                                └─ formatter.pad_integral(...)
                                                        │
                                                        └─ output.write_str("00001010")
```

### 1.3 Trait Object Dispatch Table

```
Arguments<'_>
┌────────────────────────────────────────────────────────┐
│  pieces: &'static [&'static str]                       │
│  fmt:    Option<&'static [rt::v1::Argument]>           │
│  args:   &'a [rt::v1::ArgumentV1<'a>]                  │
│              │                                         │
│              └─ ArgumentV1 {                           │
│                   value: *const (),   ← type-erased   │
│                   formatter: fn(*const (), &mut Fmt)   │← fn ptr
│                 }                                      │
└────────────────────────────────────────────────────────┘

Each formatter fn pointer is:
  <T as fmt::Display>::fmt as fn(*const (), &mut Formatter)
  <T as fmt::Debug>::fmt   as fn(*const (), &mut Formatter)
  <T as fmt::Binary>::fmt  as fn(*const (), &mut Formatter)
  ...etc
```

### 1.4 Formatter State Machine

```
Formatter<'_>
┌─────────────────────────────────────────────────────────┐
│  buf:       &'a mut (dyn Write + 'a)   ← output sink   │
│  flags:     u32                        ← sign/alt/zero  │
│  fill:      char                       ← pad char       │
│  align:     Alignment (Left/Right/Center/Unknown)       │
│  width:     Option<usize>                               │
│  precision: Option<usize>                               │
└─────────────────────────────────────────────────────────┘

Formatter methods:
  .pad(str)                ← apply fill+align+width to a str
  .pad_integral(bool, prefix, buf) ← for numeric types
  .write_str(str)          ← raw write, bypasses padding
  .write_fmt(args)         ← recursive format_args
  .fill() -> char
  .align() -> Option<Alignment>
  .width() -> Option<usize>
  .precision() -> Option<usize>
  .sign_plus() -> bool     ← `+` flag
  .sign_minus() -> bool    ← `-` flag (left-align)
  .alternate() -> bool     ← `#` flag
  .sign_aware_zero_pad() -> bool ← `0` flag
```

---

## 2. Grammar

### 2.1 Full BNF Grammar of a Format Specifier

```
format_string   := text [ '{' format_spec '}' text ]*
format_spec     := [ argument ] [ ':' format_ops ]
argument        := integer          (positional: {0}, {1})
                 | identifier       (named: {name})
                 | ''               (implicit next: {})

format_ops      := [ [ fill ] align ] [ sign ] [ '#' ] [ '0' ] [ width ] [ '.' precision ] type

fill            := any character (default ' ')
align           := '<'   left
                 | '^'   center
                 | '>'   right
sign            := '+'   always show sign
                 | '-'   omitted (default; for numbers, show '-' only for negatives)
'#'             := alternate form
'0'             := zero-pad (implies right-align with '0' as fill)
width           := integer | variable   (e.g., 10 or name$ or 0$)
precision       := '.' ( integer | variable | '*' )
type            := ''   Display
                 | '?'  Debug
                 | 'x?' LowerHex Debug
                 | 'X?' UpperHex Debug
                 | 'o'  Octal
                 | 'x'  LowerHex
                 | 'X'  UpperHex
                 | 'b'  Binary
                 | 'e'  LowerExp
                 | 'E'  UpperExp
                 | 'p'  Pointer
                 | 'a'  (not stable as of 1.78; reserved)
```

### 2.2 Visual Anatomy of a Specifier

```
  { argument : fill align sign # 0 width . precision type }
  │           │    │     │    │ │  │      │ │          │
  │           │    │     │    │ │  │      │ │          └─ format type (x, b, ?, …)
  │           │    │     │    │ │  │      │ └─ precision digits
  │           │    │     │    │ │  │      └─ '.' separator
  │           │    │     │    │ │  └─ minimum field width
  │           │    │     │    │ └─ zero-pad flag
  │           │    │     │    └─ alternate flag
  │           │    │     └─ sign flag ('+' or '-')
  │           │    └─ alignment ('<', '^', '>')
  │           └─ fill character (any unicode scalar)
  └─ argument index or name (optional)
```

---

## 3. Format Traits and Their Specifiers

### 3.1 Complete Trait Map

| Specifier | Trait             | Example Output          | Rust Path            |
|-----------|-------------------|-------------------------|----------------------|
| `{}`      | `fmt::Display`    | `42`, `hello`           | `std::fmt::Display`  |
| `{:?}`    | `fmt::Debug`      | `42`, `"hello"`, `[1,2]`| `std::fmt::Debug`    |
| `{:#?}`   | `fmt::Debug`      | pretty-printed          | `std::fmt::Debug`    |
| `{:b}`    | `fmt::Binary`     | `101010`                | `std::fmt::Binary`   |
| `{:o}`    | `fmt::Octal`      | `52`                    | `std::fmt::Octal`    |
| `{:x}`    | `fmt::LowerHex`   | `2a`                    | `std::fmt::LowerHex` |
| `{:X}`    | `fmt::UpperHex`   | `2A`                    | `std::fmt::UpperHex` |
| `{:e}`    | `fmt::LowerExp`   | `4.2e1`                 | `std::fmt::LowerExp` |
| `{:E}`    | `fmt::UpperExp`   | `4.2E1`                 | `std::fmt::UpperExp` |
| `{:p}`    | `fmt::Pointer`    | `0x7fff5fbff8a0`        | `std::fmt::Pointer`  |
| `{:x?}`   | `fmt::Debug`+hex  | `'\x2a'`                | `std::fmt::Debug`    |
| `{:X?}`   | `fmt::Debug`+HEX  | `'\x2A'`                | `std::fmt::Debug`    |

### 3.2 All Primitive Types and Which Traits They Implement

```
Type          Display  Debug  Binary  Octal  LowerHex  UpperHex  LowerExp  UpperExp  Pointer
─────────────────────────────────────────────────────────────────────────────────────────────
i8..i128        ✓       ✓      ✓       ✓       ✓         ✓         ✗         ✗         ✗
u8..u128        ✓       ✓      ✓       ✓       ✓         ✓         ✗         ✗         ✗
isize/usize     ✓       ✓      ✓       ✓       ✓         ✓         ✗         ✗         ✗
i128/u128       ✓       ✓      ✓       ✓       ✓         ✓         ✗         ✗         ✗
f32/f64         ✓       ✓      ✗       ✗       ✗         ✗         ✓         ✓         ✗
bool            ✓       ✓      ✗       ✗       ✗         ✗         ✗         ✗         ✗
char            ✓       ✓      ✗       ✗       ✗         ✗         ✗         ✗         ✗
str / &str      ✓       ✓      ✗       ✗       ✗         ✗         ✗         ✗         ✗
String          ✓       ✓      ✗       ✗       ✗         ✗         ✗         ✗         ✗
*const T        ✗       ✓      ✗       ✗       ✗         ✗         ✗         ✗         ✓
*mut T          ✗       ✓      ✗       ✗       ✗         ✗         ✗         ✗         ✓
&T (any)        (delegates to T's impl)
Option<T>       ✗       ✓(T:Debug) ✗  ✗       ✗         ✗         ✗         ✗         ✗
Result<T,E>     ✗       ✓      ✗       ✗       ✗         ✗         ✗         ✗         ✗
Vec<T>          ✗       ✓      ✗       ✗       ✗         ✗         ✗         ✗         ✗
[T]/[T;N]       ✗       ✓      ✗       ✗       ✗         ✗         ✗         ✗         ✗
tuples (0-12)   ✗       ✓      ✗       ✗       ✗         ✗         ✗         ✗         ✗
```

---

## 4. Fill, Alignment, Width, Precision

### 4.1 Fill and Alignment

The fill character is **any Unicode scalar value** placed before the alignment character. If no fill is specified, space (`' '`) is used.

```rust
// Alignment
format!("{:<10}", "left")    // "left      "   — left-aligned, padded right
format!("{:>10}", "right")   // "     right"   — right-aligned, padded left
format!("{:^10}", "center")  // "  center  "   — centered
format!("{:^11}", "center")  // "  center   "  — odd: extra pad on right

// Fill character
format!("{:*<10}", "left")   // "left******"
format!("{:*>10}", "right")  // "*****right"
format!("{:*^10}", "center") // "**center**"
format!("{:0>8}", 42)        // "00000042"  — fill with '0', right-align
                             //   NOTE: differs from {:08} (see zero-pad flag)

// Unicode fill
format!("{:🦀^9}", "hi")     // "🦀🦀🦀hi🦀🦀🦀" (width is char count, not bytes)
```

### 4.2 Width

Width specifies the **minimum** number of **Unicode scalar values** (characters, not bytes) in the output. If the formatted value is wider, no truncation occurs (truncation is precision's job, for strings).

```rust
format!("{:5}",  42)      // "   42"   — 5 chars wide, right-align (numeric default)
format!("{:5}",  "hi")    // "hi   "   — 5 chars wide, left-align (string default)
format!("{:05}", 42)      // "00042"   — zero-pad
format!("{:5}",  999999)  // "999999"  — no truncation, output is 6 chars
```

**Default alignment:**
- Numeric types (`i*`, `u*`, `f*`): **right-aligned**
- Everything else (`str`, `String`, custom Display): **left-aligned**

### 4.3 Precision

Precision has **two distinct meanings** depending on context:

#### For floating-point types

Precision = number of digits **after** the decimal point.

```rust
format!("{:.2}",  3.14159)   // "3.14"
format!("{:.0}",  3.7)       // "4"     — rounds
format!("{:.5}",  1.0)       // "1.00000"
format!("{:8.2}", 3.14)      // "    3.14"  — width=8, precision=2
format!("{:08.2}",3.14)      // "00003.14"
```

#### For string types

Precision = **maximum** number of Unicode scalar values to output (truncates if longer).

```rust
format!("{:.5}", "Hello, World!")   // "Hello"
format!("{:.5}", "Hi")             // "Hi"    — shorter than 5, no padding
format!("{:10.5}", "Hello, World!")// "Hello     " — width=10, truncate to 5, pad to 10
```

#### For integer types

Precision has **no defined standard meaning** for integers when using `{}` or `{:x}` etc. It does nothing for integers under Display, Debug, or integer hex/bin/oct. Avoid relying on it for integers — it's a no-op in current Rust.

---

## 5. Flags

### 5.1 The `+` Sign Flag

Forces a sign character (`+` or `-`) to always be printed for numeric types.

```rust
format!("{:+}", 42)     // "+42"
format!("{:+}", -42)    // "-42"
format!("{:+}", 0)      // "+0"
format!("{:+.2}", 3.14) // "+3.14"
```

### 5.2 The `-` Flag

The `-` alignment shorthand means **left-align**. It is equivalent to `{:<}`.

```rust
format!("{:-<10}", 42)  // compile error — '-' as fill only, not valid alone
format!("{:<10}", 42)   // "42        "
// In practice, '-' in the sign position just means "no forced sign" (default)
// Don't confuse: '-' as sign position vs '<' as alignment
```

> **Critical distinction:** In Rust's grammar, `-` is a **sign modifier** (show minus for negatives — the default). It is NOT an alignment specifier. Left-alignment is done with `<`. This differs from C's `%-10d`.

### 5.3 The `#` Alternate Flag

Adds a **type-specific prefix**:

```rust
format!("{:#b}", 42)    // "0b101010"    Binary with 0b prefix
format!("{:#o}", 42)    // "0o52"        Octal with 0o prefix
format!("{:#x}", 42)    // "0x2a"        LowerHex with 0x prefix
format!("{:#X}", 42)    // "0x2A"        UpperHex with 0x prefix (note: always lowercase 0x)
format!("{:#?}", vec![1,2,3]) // pretty-printed Debug
format!("{:#e}", 1234.5)      // "1.2345e3" — alternate float form (no effect in std)
```

**Width with alternate form (prefix counts toward width):**
```rust
format!("{:#010x}", 42) // "0x0000002a"  — width=10 includes the "0x"
format!("{:#010b}", 5)  // "0b00000101"  — width=10 includes the "0b"
```

### 5.4 The `0` Zero-Pad Flag

Zero-pads a numeric type to the specified width. Implies right-alignment. The zero padding is inserted **between the sign/prefix and the digits**.

```rust
format!("{:08}", 42)     // "00000042"
format!("{:08}", -42)    // "-0000042"   — sign before zeros
format!("{:#010x}", 42)  // "0x0000002a" — prefix before zeros
format!("{:08.2}", 3.14) // "00003.14"   — zeros before decimal
```

**Zero-pad vs fill with `'0'`:**
```rust
format!("{:0>8}", 42)    // "00000042"   — same visual result for positive
format!("{:0>8}", -42)   // "0000-042"   — DIFFERENT: sign is inside padding (WRONG for numbers)
format!("{:08}",  -42)   // "-0000042"   — CORRECT: sign before zero-padding
```

This is the critical difference: `{:08}` is sign-aware; `{:0>8}` is not.

### 5.5 The Space Flag (` `)

A space before a number when no sign is shown, so positive and negative numbers align in columns.

```rust
format!("{: }", 42)    // " 42"
format!("{: }", -42)   // "-42"
// Use case: align columns
for n in [-100, 0, 100, -1, 1] {
    println!("{: >6}", n);
}
// " -100"
// "    0"
// "  100"
// "   -1"
// "    1"
```

---

## 6. Positional and Named Arguments

### 6.1 Implicit (Sequential) Arguments

```rust
format!("{} {} {}", 1, 2, 3)  // "1 2 3"
// Each {} consumes the next argument in order
```

### 6.2 Positional Arguments

```rust
format!("{0} {1} {0}", "a", "b")   // "a b a"
format!("{1} {0}",     "first", "second") // "second first"
format!("{0:>10} {0:b}", 42)        // "        42 101010"  — same arg, different format
```

**Mixing implicit and positional is a compile error:**
```rust
// format!("{} {0}", "a") — ERROR: cannot mix implicit and positional
```

### 6.3 Named Arguments

```rust
let name = "Alice";
let age  = 30_u32;
format!("{name} is {age}")             // "Alice is 30" (captured from env)
format!("{name:>10} is {age:03}")      // "     Alice is 030"

// Explicit named args
format!("{n:>10}", n = "Bob")          // "       Bob"

// Named + format ops
format!("{value:#010x}", value = 255)  // "0x000000ff"
```

### 6.4 Named Arguments and Capture (Rust 1.58+)

Since Rust 1.58, identifiers in scope can be captured directly:

```rust
let width  = 10_usize;
let fill   = '*';
let value  = 42_u64;
// format!("{value:fill^width}") — NOT valid, fill/width are not specifier syntax

// But names for width/precision via $ syntax:
format!("{:>width$}", "hi", width = 10)  // "        hi"
format!("{:.prec$}",  3.14, prec  = 4)  // "3.1400"
```

---

## 7. Dynamic Width and Precision

### 7.1 The `$` Variable Specifier

Width and precision can be taken from **other arguments** using the `$` suffix:

```rust
// width from argument by position
format!("{:>0$}", 42, 8)         // "      42"  — width=8 from arg[0]=8... wait:
// Actually: {:<width>$} where width$ refers to the argument
format!("{:>1$}", 42, 8)         // "      42"  — arg[1]=8 is the width
format!("{:.1$}", 3.14159, 3)    // "3.142"     — arg[1]=3 is the precision

// named dynamic width
format!("{:>width$}", 42, width=8)      // "      42"
format!("{:.prec$}",  3.14, prec=2)     // "3.14"

// both dynamic
format!("{:>width$.prec$}", 3.14, width=10, prec=2)  // "      3.14"
```

### 7.2 The `*` Precision Specifier

When using positional format, `.*` takes the **next positional argument** as precision:

```rust
format!("{:.1$}", 3.14159, 3)    // "3.142" — 1$ = arg at index 1 = 3
// With .*: this is the "next argument" form (less common)
format!("{:.prec$}", 3.14159, prec=3) // preferred named form
```

### 7.3 Runtime Width/Precision Pattern

```rust
fn fmt_dynamic(value: f64, width: usize, prec: usize) -> String {
    format!("{:>width$.prec$}", value, width = width, prec = prec)
}

fn main() {
    println!("{}", fmt_dynamic(3.14159, 12, 4)); // "      3.1416"
}
```

---

## 8. Escaping Braces

To output a literal `{` or `}`, double them:

```rust
format!("{{")            // "{"
format!("}}")            // "}"
format!("{{{}}}", 42)    // "{42}"   — outer {{ and }} escape, inner {} formats
format!("{{{}:b}}", 42)  // "{42:b}" — the :b is literal text
format!("{:b}", 42)      // "101010" — this is the actual binary format
```

**Literal curly brace with value:**
```rust
let n = 42;
format!("value = {{{n}}}")   // "value = {42}"
format!("{{value = {n}}}")   // "{value = 42}"
```

---

## 9. The `Formatter` Struct — Internals

### 9.1 Structure and Fields (conceptually)

```rust
// std::fmt::Formatter<'_> — simplified internal representation
pub struct Formatter<'a> {
    flags:     u32,           // bit field: sign_plus, sign_minus, alternate, sign_zero
    fill:      char,
    align:     rt::Alignment, // Left, Right, Center, Unknown
    width:     Option<usize>,
    precision: Option<usize>,
    buf:       &'a mut (dyn Write + 'a),
}
```

### 9.2 Public API

```rust
impl<'a> Formatter<'a> {
    // Output control
    pub fn write_str(&mut self, data: &str) -> Result;
    pub fn write_char(&mut self, c: char) -> Result;
    pub fn write_fmt(&mut self, fmt: Arguments<'_>) -> Result;

    // Padding helpers (for custom impls)
    pub fn pad(&mut self, s: &str) -> Result;
    pub fn pad_integral(&mut self, is_nonnegative: bool,
                        prefix: &str, buf: &str) -> Result;

    // Spec accessors
    pub fn fill(&self) -> char;
    pub fn align(&self) -> Option<Alignment>;
    pub fn width(&self) -> Option<usize>;
    pub fn precision(&self) -> Option<usize>;

    // Flag accessors
    pub fn sign_plus(&self) -> bool;       // '+' flag
    pub fn sign_minus(&self) -> bool;      // '-' flag (left align)
    pub fn alternate(&self) -> bool;       // '#' flag
    pub fn sign_aware_zero_pad(&self) -> bool; // '0' flag
}
```

### 9.3 `pad` vs `pad_integral`

```
pad(s):
  ┌────────────────────────────────────────────────┐
  │ 1. compute display_width = s.chars().count()   │
  │ 2. if display_width >= width: write s as-is    │
  │ 3. else: compute pad_count = width - display_w │
  │ 4. distribute pad_count per alignment:          │
  │      Left:   write s, then fill×pad_count       │
  │      Right:  write fill×pad_count, then s       │
  │      Center: write fill×left_pad, s, fill×right │
  │      Unknown (default for Display): Left         │
  └────────────────────────────────────────────────┘

pad_integral(is_nonneg, prefix, buf):
  ┌──────────────────────────────────────────────────┐
  │ 1. Determine sign: '-' if neg, '+' if flag set    │
  │ 2. Determine alternate prefix: "0x", "0b", "0o"   │
  │ 3. total_len = sign.len + prefix.len + buf.len     │
  │ 4. if zero_pad: fill with '0' between prefix+sign  │
  │    and digits until width is met                   │
  │ 5. else: pad as per normal alignment               │
  │ 6. write sign + prefix + [zero-pad] + buf          │
  └──────────────────────────────────────────────────┘
```

### 9.4 `DebugStruct`, `DebugTuple`, `DebugList`, `DebugSet`, `DebugMap`

These helpers are returned by `Formatter` methods and simplify implementing `Debug`:

```rust
// DebugStruct helper
pub fn debug_struct<'b>(&'b mut self, name: &str) -> DebugStruct<'b, 'a>;
pub fn debug_tuple<'b>(&'b mut self, name: &str) -> DebugTuple<'b, 'a>;
pub fn debug_list<'b>(&'b mut self) -> DebugList<'b, 'a>;
pub fn debug_set<'b>(&'b mut self) -> DebugSet<'b, 'a>;
pub fn debug_map<'b>(&'b mut self) -> DebugMap<'b, 'a>;
```

These helpers automatically handle:
- Struct syntax `Name { field: value }`
- Pretty-print indentation when `{:#?}` is used
- Commas and braces

---

## 10. All Formatting Macros

### 10.1 Complete Macro Inventory

| Macro          | Output to        | Returns          | Allocates? | Notes                                     |
|----------------|------------------|------------------|------------|-------------------------------------------|
| `format!`      | `String`         | `String`         | Yes        | Most common; heap allocation              |
| `format_args!` | `Arguments<'_>`  | `Arguments<'_>`  | No         | Zero-alloc; core primitive                |
| `write!`       | `&mut impl Write`| `fmt::Result`    | No         | Writes to any `Write` implementor         |
| `writeln!`     | `&mut impl Write`| `fmt::Result`    | No         | Like `write!` but appends `\n`            |
| `print!`       | stdout           | `()`             | Yes*       | *locks stdout, may alloc for intermediate |
| `println!`     | stdout           | `()`             | Yes*       | Like `print!` but appends `\n`            |
| `eprint!`      | stderr           | `()`             | Yes*       | Writes to stderr                          |
| `eprintln!`    | stderr           | `()`             | Yes*       | Like `eprint!` but appends `\n`           |
| `panic!`       | panic message    | `!` (never)      | Yes        | Formats message and panics                |
| `assert!`      | panic on false   | `()`             | Cond.      | Accepts format args                       |
| `assert_eq!`   | panic on !=      | `()`             | Cond.      | Displays both values on failure           |
| `assert_ne!`   | panic on ==      | `()`             | Cond.      | Displays both values on failure           |
| `dbg!`         | stderr           | value            | Yes        | Prints file, line, expr, value; returns   |
| `todo!`        | panic            | `!`              | Yes        | `panic!("not yet implemented")`           |
| `unimplemented!`| panic           | `!`              | Yes        | Semantic alias for `todo!`                |
| `unreachable!` | panic            | `!`              | Yes        | Marks logically unreachable code          |

### 10.2 `write!` with Custom Buffers

```rust
use std::fmt::Write;   // NOTE: std::fmt::Write, not std::io::Write

let mut buf = String::new();
write!(buf, "x = {}", 42).unwrap();      // buf = "x = 42"
writeln!(buf, " y = {:.2}", 3.14).unwrap(); // buf = "x = 42 y = 3.14\n"
```

```rust
use std::io::Write;  // std::io::Write for byte-based output

let mut buf: Vec<u8> = Vec::new();
write!(buf, "x = {}", 42).unwrap();
// buf = b"x = 42"
```

**`std::fmt::Write` vs `std::io::Write`:**
```
std::fmt::Write:
  fn write_str(&mut self, s: &str) -> fmt::Result
  fn write_char(&mut self, c: char) -> fmt::Result
  fn write_fmt(&mut self, args: Arguments<'_>) -> fmt::Result
  Implemented by: String, &mut String, Formatter
  Works with: write!/writeln! from std::fmt

std::io::Write:
  fn write(&mut self, buf: &[u8]) -> io::Result<usize>
  fn write_all(&mut self, buf: &[u8]) -> io::Result<()>
  fn write_fmt(&mut self, fmt: Arguments<'_>) -> io::Result<()>
  Implemented by: File, Vec<u8>, BufWriter, TcpStream, etc.
  Works with: write!/writeln! from std::io
```

### 10.3 `dbg!` Macro Details

```rust
let x = 5;
let y = dbg!(x * 2) + 1;
// stderr: [src/main.rs:2] x * 2 = 10
// y = 11  (dbg! returns the value)

// Works with multi-expressions
dbg!(1 + 1, 2 + 2);
// [src/main.rs:X] 1 + 1 = 2
// [src/main.rs:X] 2 + 2 = 4
```

`dbg!` uses `{:#?}` (pretty-print Debug) internally.

---

## 11. The `format_args!` ABI and `Arguments<'_>`

### 11.1 What `format_args!` Actually Is

`format_args!` is a compiler built-in (not a regular macro). It returns a `core::fmt::Arguments<'_>` — a **stack-allocated, borrow-based, zero-allocation** description of a formatting operation.

```rust
// Conceptually (actual implementation is in the compiler):
let args: fmt::Arguments<'_> = format_args!("x = {}, y = {:.2}", x, y);
// args contains:
//   - Static string pieces: ["x = ", ", y = ", ""]
//   - Format specs: [Display{}, {precision=2}]
//   - References to x and y (erased to *const ())
// NO heap allocation at this point.
```

### 11.2 The `Arguments<'_>` Lifetime

The lifetime `'_` ties `Arguments` to the **borrowed values** it references:

```rust
fn log(args: fmt::Arguments<'_>) {
    // Forward to any Write implementor
    println!("{}", args);
}

log(format_args!("error: {} at line {}", msg, line));
// The values msg and line must outlive the call
```

### 11.3 Allocation-Free Logging Pattern

```rust
use std::fmt;
use std::io::{self, Write};

fn write_log(w: &mut impl io::Write, args: fmt::Arguments<'_>) -> io::Result<()> {
    w.write_fmt(args)
}

// Zero allocation:
write_log(&mut stderr, format_args!("error: {} code={}", msg, code)).unwrap();

// Macro wrapper:
macro_rules! log_err {
    ($w:expr, $($arg:tt)*) => {
        write_log($w, format_args!($($arg)*))
    };
}
```

### 11.4 Internals of `rt::v1::Argument`

```rust
// core/src/fmt/rt/v1.rs (simplified)
pub struct Argument {
    pub position:  usize,       // which arg index
    pub format:    FormatSpec,
}

pub struct FormatSpec {
    pub fill:       char,
    pub align:      Alignment,
    pub flags:      u32,        // bits for sign_plus, sign_minus, alternate, zero
    pub precision:  Count,      // Implied | Param(usize) | NextParam | Is(usize)
    pub width:      Count,
}

pub enum Count {
    Implied,        // not specified
    Is(usize),      // literal number
    Param(usize),   // from positional argument
}
```

---

## 12. Implementing Custom `Display` and `Debug`

### 12.1 Implementing `Display`

`Display` is the human-readable representation. It should **not** include type names, quotes, or debug noise.

```rust
use std::fmt;

#[derive(Clone)]
pub struct IpAddr {
    octets: [u8; 4],
}

impl fmt::Display for IpAddr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}.{}.{}.{}",
            self.octets[0],
            self.octets[1],
            self.octets[2],
            self.octets[3],
        )
    }
}

// Usage:
let ip = IpAddr { octets: [192, 168, 1, 1] };
println!("{}", ip);          // "192.168.1.1"
println!("{:>20}", ip);      // "         192.168.1.1" — padding works automatically
let s = ip.to_string();      // Display is used by ToString automatically
```

### 12.2 Implementing `Debug`

`Debug` is the machine-readable/developer representation. Use `Formatter`'s helpers:

```rust
use std::fmt;

pub struct Packet {
    src:  IpAddr,
    dst:  IpAddr,
    ttl:  u8,
    data: Vec<u8>,
}

impl fmt::Debug for Packet {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Packet")
            .field("src",  &self.src)   // src implements Display, but Debug needs Debug
            .field("dst",  &self.dst)
            .field("ttl",  &self.ttl)
            .field("data", &self.data)
            .finish()
    }
}
```

**Implementing both Display AND Debug with shared logic:**

```rust
impl fmt::Display for IpAddr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}.{}.{}.{}",
            self.octets[0], self.octets[1],
            self.octets[2], self.octets[3])
    }
}

impl fmt::Debug for IpAddr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // Re-use Display logic for the value part
        if f.alternate() {
            f.debug_struct("IpAddr")
                .field("octets", &self.octets)
                .finish()
        } else {
            write!(f, "IpAddr({})", self)   // calls Display
        }
    }
}
```

### 12.3 Honoring `Formatter` Options in Custom Impls

When you implement `Display`, you should honor width, precision, alignment, and fill if meaningful. The `pad` method handles this for strings:

```rust
impl fmt::Display for IpAddr {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // Format into a temporary buffer first
        let s = format!("{}.{}.{}.{}",
            self.octets[0], self.octets[1],
            self.octets[2], self.octets[3]);

        // Then let pad() handle width/alignment/fill
        f.pad(&s)
    }
}
// Now {:>20} and {:*^25} work correctly for IpAddr
```

---

## 13. Implementing All Other `fmt` Traits

### 13.1 `fmt::Binary`

```rust
use std::fmt;

pub struct Flags(u32);

impl fmt::Binary for Flags {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // Delegate to the inner u32's Binary impl
        fmt::Binary::fmt(&self.0, f)
    }
}

// Or manually, honoring alternate (#) flag:
impl fmt::Binary for Flags {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let val = self.0;
        if f.alternate() {
            write!(f, "Flags(0b{:b})", val)
        } else {
            fmt::Binary::fmt(&val, f)
        }
    }
}

let flags = Flags(0b1010_1100);
println!("{:b}",   flags);  // "10101100"
println!("{:#b}",  flags);  // "Flags(0b10101100)"  — alternate form
println!("{:016b}",flags);  // "0000000010101100"
```

### 13.2 `fmt::LowerHex` and `fmt::UpperHex`

```rust
use std::fmt;

pub struct Address(usize);

impl fmt::LowerHex for Address {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // Use pad_integral for correct sign/prefix/zero-pad handling
        let val = self.0;
        if f.alternate() {
            write!(f, "0x{:x}", val)
        } else {
            write!(f, "{:x}", val)
        }
    }
}

impl fmt::UpperHex for Address {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let val = self.0;
        if f.alternate() {
            write!(f, "0x{:X}", val)
        } else {
            write!(f, "{:X}", val)
        }
    }
}

// Correct way using pad_integral for full spec support:
impl fmt::LowerHex for Address {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let val = self.0;
        // format digits
        let s = format!("{:x}", val);
        let prefix = if f.alternate() { "0x" } else { "" };
        f.pad_integral(true, prefix, &s)
    }
}
```

### 13.3 `fmt::Octal`

```rust
impl fmt::Octal for Flags {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        fmt::Octal::fmt(&self.0, f)
    }
}
println!("{:o}",   Flags(0o755)); // "755"
println!("{:#o}",  Flags(0o755)); // "0o755"
println!("{:08o}", Flags(0o755)); // "00000755"
```

### 13.4 `fmt::LowerExp` and `fmt::UpperExp`

```rust
pub struct Nanoseconds(f64);

impl fmt::LowerExp for Nanoseconds {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:e}ns", self.0)
    }
}

impl fmt::UpperExp for Nanoseconds {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:E}ns", self.0)
    }
}

let t = Nanoseconds(1234.5e6);
println!("{:e}", t);    // "1.2345e9ns"
println!("{:E}", t);    // "1.2345E9ns"
println!("{:.2e}", t);  // "1.23e9ns"
```

### 13.5 `fmt::Pointer`

```rust
pub struct SafePtr<T>(*const T);

impl<T> fmt::Pointer for SafePtr<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        fmt::Pointer::fmt(&self.0, f)
    }
}

let x = 42_u64;
let p = SafePtr(&x as *const u64);
println!("{:p}", p);    // "0x7ffd5b2e0428"  (platform address)
println!("{:#p}", p);   // same (alternate has no extra effect for pointers)
```

---

## 14. The `Write` Trait

### 14.1 `std::fmt::Write` — For Formatting Targets

```rust
pub trait Write {
    // Required:
    fn write_str(&mut self, s: &str) -> fmt::Result;

    // Provided (can override for efficiency):
    fn write_char(&mut self, c: char) -> fmt::Result {
        self.write_str(c.encode_utf8(&mut [0; 4]))
    }
    fn write_fmt(&mut self, args: fmt::Arguments<'_>) -> fmt::Result {
        fmt::write(self, args)
    }
}
```

**Implementing a custom write buffer:**

```rust
use std::fmt::{self, Write};

pub struct CapacityWriter {
    buf:      String,
    capacity: usize,
    overflow: bool,
}

impl CapacityWriter {
    pub fn new(capacity: usize) -> Self {
        Self { buf: String::with_capacity(capacity), capacity, overflow: false }
    }
    pub fn as_str(&self) -> &str { &self.buf }
    pub fn overflowed(&self) -> bool { self.overflow }
}

impl fmt::Write for CapacityWriter {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        let remaining = self.capacity.saturating_sub(self.buf.len());
        if s.len() <= remaining {
            self.buf.push_str(s);
        } else {
            // write what fits, mark overflow
            let fits = &s[..find_char_boundary(s, remaining)];
            self.buf.push_str(fits);
            self.overflow = true;
        }
        Ok(())
    }
}

fn find_char_boundary(s: &str, pos: usize) -> usize {
    // Walk back to valid UTF-8 boundary
    let mut p = pos.min(s.len());
    while p > 0 && !s.is_char_boundary(p) { p -= 1; }
    p
}

// Usage:
let mut w = CapacityWriter::new(16);
write!(w, "Hello, {}", "World! This is long").unwrap();
println!("{:?}", w.as_str());     // "Hello, World! T" (truncated)
println!("overflow: {}", w.overflowed()); // true
```

### 14.2 Implementing a Hex-Escape Writer

```rust
use std::fmt::{self, Write};

/// Wraps a Write and escapes non-ASCII bytes as \xNN
pub struct AsciiSafeWriter<W: Write> {
    inner: W,
}

impl<W: Write> AsciiSafeWriter<W> {
    pub fn new(inner: W) -> Self { Self { inner } }
}

impl<W: Write> Write for AsciiSafeWriter<W> {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        for c in s.chars() {
            if c.is_ascii_graphic() || c == ' ' {
                self.inner.write_char(c)?;
            } else {
                let mut buf = [0u8; 4];
                let encoded = c.encode_utf8(&mut buf);
                for byte in encoded.bytes() {
                    write!(self.inner, "\\x{:02x}", byte)?;
                }
            }
        }
        Ok(())
    }
}

let mut out = String::new();
let mut safe = AsciiSafeWriter::new(&mut out);
write!(safe, "Hello\x00\nWorld\u{1F600}").unwrap();
println!("{}", out); // "Hello\x00\x0aWorld\xf0\x9f\x98\x80"
```

---

## 15. Padding, Sign, and Alternate Forms — Deep Dive

### 15.1 Padding Algorithm for Strings

```
Input: s = "Hi", width = 8, fill = '*', align = Center

Step 1: display_len = s.chars().count() = 2
Step 2: pad_total   = width - display_len = 8 - 2 = 6
Step 3: left_pad    = pad_total / 2       = 3
        right_pad   = pad_total - left_pad = 3

Step 4: output = "***Hi***"

For odd padding (width=9):
  left_pad  = 3
  right_pad = 4
  output    = "***Hi****"
```

### 15.2 Padding Algorithm for Integers with Zero-Pad

```
Input: v = -42, width = 10, zero_pad = true

Step 1: sign    = "-"         (1 char)
Step 2: prefix  = ""          (no alternate)
Step 3: digits  = "42"        (2 chars)
Step 4: current_len = 1+0+2  = 3
Step 5: zeros_needed = 10-3  = 7
Step 6: output  = "-" + "0000000" + "42"
                = "-000000042"
```

### 15.3 Width Counting is Unicode Scalar Values, Not Display Width

```rust
// Rust counts Unicode scalar values (char), NOT grapheme clusters, NOT display columns
format!("{:5}", "a\u{0301}") // "á " — 'a' + combining accent = 2 scalars, width=5 pads to 3
format!("{:5}", "🦀")        // "🦀    " — crab emoji = 1 scalar, pads to 4 spaces
// BUT in a terminal, emoji takes 2 columns — Rust doesn't account for this
// For terminal-correct padding, use unicode-width crate
```

---

## 16. Debug vs Display — Semantic Contract

```
                ┌─────────────────────────────────────────────┐
                │           SEMANTIC CONTRACT                  │
                ├──────────────────┬──────────────────────────┤
                │ Display          │ Debug                     │
                ├──────────────────┼──────────────────────────┤
                │ For end users    │ For developers            │
                │ No type names    │ Includes type names       │
                │ No quotes        │ Strings quoted            │
                │ No braces        │ Structs use {} syntax     │
                │ Lossy OK         │ Should be parseable       │
                │ May be localized │ Should be unambiguous     │
                │ Custom to domain │ Derive-able always        │
                ├──────────────────┼──────────────────────────┤
                │ {} specifier     │ {:?} specifier            │
                │ format_args!     │ assert_eq! output         │
                │ user messages    │ dbg! output               │
                │ log output       │ error! macro output       │
                └──────────────────┴──────────────────────────┘
```

```rust
let s = String::from("hello\nworld");

println!("{}", s);   // hello
                     // world

println!("{:?}", s); // "hello\nworld"   — escaped, quoted

let v: Vec<i32> = vec![1, 2, 3];
println!("{:?}", v);  // [1, 2, 3]
// println!("{}", v); // ERROR: Vec does not implement Display
```

---

## 17. Pretty-Print `{:#?}` and Derive Internals

### 17.1 The `#` Flag with Debug

The alternate (`#`) flag tells `Formatter` to enter **pretty-print mode**, which:
- Adds newlines after opening braces
- Indents nested structures by 4 spaces per level
- Adds trailing commas
- Adds newlines before closing braces

```rust
#[derive(Debug)]
struct Server {
    addr:    std::net::SocketAddr,
    workers: usize,
    tls:     TlsConfig,
}

#[derive(Debug)]
struct TlsConfig {
    cert:    String,
    key:     String,
    ciphers: Vec<String>,
}

let s = Server {
    addr:    "0.0.0.0:8443".parse().unwrap(),
    workers: 8,
    tls: TlsConfig {
        cert:    "/etc/ssl/cert.pem".into(),
        key:     "/etc/ssl/key.pem".into(),
        ciphers: vec!["TLS_AES_256_GCM_SHA384".into()],
    },
};

println!("{:#?}", s);
// Server {
//     addr: 0.0.0.0:8443,
//     workers: 8,
//     tls: TlsConfig {
//         cert: "/etc/ssl/cert.pem",
//         key: "/etc/ssl/key.pem",
//         ciphers: [
//             "TLS_AES_256_GCM_SHA384",
//         ],
//     },
// }
```

### 17.2 What `#[derive(Debug)]` Generates

For a struct:
```rust
#[derive(Debug)]
struct Point { x: f64, y: f64 }

// Roughly expands to:
impl std::fmt::Debug for Point {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Point")
            .field("x", &self.x)
            .field("y", &self.y)
            .finish()
    }
}
```

For an enum:
```rust
#[derive(Debug)]
enum Status { Ok, Err(String), Code(u32, u32) }

// Roughly expands to:
impl std::fmt::Debug for Status {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Status::Ok           => f.write_str("Ok"),
            Status::Err(s)       => f.debug_tuple("Err").field(s).finish(),
            Status::Code(a, b)   => f.debug_tuple("Code").field(a).field(b).finish(),
        }
    }
}
```

### 17.3 `DebugStruct` / `DebugList` / `DebugMap` API

```rust
// DebugStruct
impl fmt::Debug for MyStruct {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let mut ds = f.debug_struct("MyStruct");
        ds.field("name",  &self.name);
        ds.field("value", &self.value);
        // finish_non_exhaustive() adds ".." to indicate hidden fields
        if self.secret.is_empty() {
            ds.finish()
        } else {
            ds.finish_non_exhaustive()  // "MyStruct { name: .., value: .., .. }"
        }
    }
}

// DebugList
impl fmt::Debug for MyList {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_list().entries(self.items.iter()).finish()
    }
}

// DebugMap
impl fmt::Debug for MyMap {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_map()
            .entries(self.map.iter().map(|(k, v)| (k, v)))
            .finish()
    }
}

// DebugSet
impl fmt::Debug for MySet {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_set().entries(self.set.iter()).finish()
    }
}
```

---

## 18. Performance: Zero-Cost Abstractions and Pitfalls

### 18.1 The Zero-Cost Model

```
format!("x = {}", val)
    │
    ▼
format_args!("x = {}", val)  ← stack-only, no allocation
    │
    ▼
std::fmt::write(&mut String::new(), args)
    │                ▲
    └─ allocates     └─ the String::new() allocates
                        the format_args! itself does NOT
```

**Key insight:** `format_args!` is free. `format!` pays for the `String` allocation. Use `format_args!` when you can forward directly to a `Write` without intermediate `String`.

### 18.2 Performance Pitfalls

```rust
// SLOW: multiple intermediate allocations
let result = format!("prefix: ") + &format!("{}", val) + &format!(" suffix");

// FAST: single write
let result = format!("prefix: {} suffix", val);

// FAST in hot path: write to pre-allocated buffer
let mut buf = String::with_capacity(64);
write!(buf, "prefix: {} suffix", val).unwrap();
buf.clear();  // reuse in loop

// BEST in hot path: avoid String entirely
use std::io::Write;
let stdout = std::io::stdout();
let mut out = stdout.lock();
write!(out, "prefix: {} suffix\n", val).unwrap();
```

### 18.3 `Display` for Hot Paths

```rust
// Avoid in hot path — calls Display, may allocate:
log::info!("value = {}", obj);

// Better — use format_args! to avoid String alloc:
log::info!("{}", format_args!("value = {}", obj));

// Or implement Display directly on the type instead of formatting a String
```

### 18.4 `to_string()` Internals

`ToString` is automatically implemented for any type that implements `Display`:
```rust
// This:
let s: String = value.to_string();
// Is exactly equivalent to:
let s: String = format!("{}", value);
// Which allocates a new String.
```

There is no way to avoid the allocation through this path.

### 18.5 `format_args!` Lifetime Constraint

```rust
// COMPILE ERROR: format_args! result can't outlive the arguments
let args = {
    let s = String::from("hello");
    format_args!("{}", s)  // ERROR: s dropped here
};
println!("{}", args);
```

This is intentional and safe — the lifetime is strictly bounded.

---

## 19. Advanced: Newtype Pattern for Custom Formatting

```rust
use std::fmt;

/// Wraps &[u8] and formats as hex dump
pub struct HexDump<'a>(pub &'a [u8]);

impl<'a> fmt::Display for HexDump<'a> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for (i, chunk) in self.0.chunks(16).enumerate() {
            // Offset
            write!(f, "{:08x}  ", i * 16)?;
            // Hex bytes
            for (j, byte) in chunk.iter().enumerate() {
                if j == 8 { write!(f, " ")?; }
                write!(f, "{:02x} ", byte)?;
            }
            // Padding if last chunk < 16
            let pad = 16 - chunk.len();
            for j in 0..pad {
                if chunk.len() + j == 8 { write!(f, " ")?; }
                write!(f, "   ")?;
            }
            write!(f, " |")?;
            // ASCII
            for &byte in chunk {
                let c = if (0x20..0x7f).contains(&byte) { byte as char } else { '.' };
                write!(f, "{}", c)?;
            }
            writeln!(f, "|")?;
        }
        Ok(())
    }
}

// Usage:
let data = b"Hello, World!\x00\x01\x02\x03";
println!("{}", HexDump(data));
// 00000000  48 65 6c 6c 6f 2c 20 57  6f 72 6c 64 21 00 01 02  |Hello, World!...|
// 00000010  03                                                 |...|
```

---

## 20. Advanced: Conditional Formatting

```rust
use std::fmt;

/// Formats a byte count with adaptive units
pub struct ByteSize(pub u64);

impl fmt::Display for ByteSize {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        const KB: u64 = 1024;
        const MB: u64 = KB * 1024;
        const GB: u64 = MB * 1024;
        const TB: u64 = GB * 1024;

        let (value, unit) = if self.0 >= TB {
            (self.0 as f64 / TB as f64, "TiB")
        } else if self.0 >= GB {
            (self.0 as f64 / GB as f64, "GiB")
        } else if self.0 >= MB {
            (self.0 as f64 / MB as f64, "MiB")
        } else if self.0 >= KB {
            (self.0 as f64 / KB as f64, "KiB")
        } else {
            return write!(f, "{} B", self.0);
        };

        // Honor precision from formatter, default to 2
        let prec = f.precision().unwrap_or(2);
        write!(f, "{:.prec$} {}", value, unit, prec = prec)
    }
}

println!("{}", ByteSize(1_500_000_000));    // "1.40 GiB"
println!("{:.1}", ByteSize(1_500_000_000)); // "1.4 GiB"
println!("{:.0}", ByteSize(1_500_000_000)); // "1 GiB"
```

---

## 21. Advanced: Recursive and Nested Formatting

```rust
use std::fmt;

/// A tree structure with proper indented Display
pub enum Tree {
    Leaf(i64),
    Node { label: String, children: Vec<Tree> },
}

struct IndentedTree<'a> {
    tree:   &'a Tree,
    indent: usize,
}

impl<'a> fmt::Display for IndentedTree<'a> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let pad = "  ".repeat(self.indent);
        match self.tree {
            Tree::Leaf(v) => writeln!(f, "{}Leaf({})", pad, v),
            Tree::Node { label, children } => {
                writeln!(f, "{}Node({})", pad, label)?;
                for child in children {
                    fmt::Display::fmt(
                        &IndentedTree { tree: child, indent: self.indent + 1 },
                        f,
                    )?;
                }
                Ok(())
            }
        }
    }
}

impl fmt::Display for Tree {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        fmt::Display::fmt(&IndentedTree { tree: self, indent: 0 }, f)
    }
}
```

---

## 22. Advanced: `format_args!` for Allocation-Free Logging

```rust
use std::fmt;
use std::io::{self, Write, BufWriter};
use std::time::SystemTime;

pub struct Logger<W: io::Write> {
    out: BufWriter<W>,
}

impl<W: io::Write> Logger<W> {
    pub fn new(w: W) -> Self {
        Self { out: BufWriter::with_capacity(8192, w) }
    }

    // Key: takes fmt::Arguments<'_> — zero-copy, no intermediate String
    pub fn log(&mut self, level: &str, args: fmt::Arguments<'_>) {
        let ts = SystemTime::now()
            .duration_since(SystemTime::UNIX_EPOCH)
            .unwrap_or_default();
        // Only one write call (via BufWriter)
        let _ = write!(self.out, "[{}.{:09}] [{}] {}\n",
            ts.as_secs(), ts.subsec_nanos(), level, args);
    }
}

// Convenience macro — passes format_args! directly, no String allocation
macro_rules! log_info {
    ($logger:expr, $($arg:tt)*) => {
        $logger.log("INFO", format_args!($($arg)*))
    };
}

// Usage:
let mut log = Logger::new(io::stderr());
let pkt_size = 1500_usize;
let src = "10.0.0.1";
log_info!(log, "packet from {} size={}", src, pkt_size);
// Zero heap allocation for the format operation
```

---

## 23. Common Mistakes and Edge Cases

### 23.1 Type Must Implement the Requested Trait

```rust
let v: Vec<i32> = vec![1, 2];
println!("{}", v);   // ERROR: Vec<i32> doesn't implement Display
println!("{:?}", v); // OK: Vec<i32> implements Debug
println!("{:x}", v); // ERROR: Vec<i32> doesn't implement LowerHex
```

### 23.2 Integer Precision Is a No-Op

```rust
format!("{:.5}", 42)   // "42"  — precision ignored for integers with {}
format!("{:.5}", 42_u8) // "42" — same
// For floats it works:
format!("{:.5}", 42.0) // "42.00000"
```

### 23.3 Width for `{:?}` Does Not Pad the Inner Debug Output

```rust
// This works:
format!("{:>20?}", "hello") // ERROR or not what you expect
// The format is {:>20?} — but this means Debug with right-align+width=20
format!("{:>20?}", "hello")   // '     "hello"' — width=20, right-align Debug output
// The width/align DOES apply to Debug output
```

### 23.4 Mixing `{}` and `{0}` is Forbidden

```rust
format!("{} {0}", "a")  // COMPILE ERROR
// Either all implicit or all explicit
```

### 23.5 Lifetime of `format_args!`

```rust
fn bad() -> fmt::Arguments<'static> {
    let s = String::from("hi");
    format_args!("{}", s)  // ERROR: s doesn't live long enough
}
// format_args! borrows its arguments; can't return it
```

### 23.6 Float NaN and Infinity

```rust
format!("{}", f64::NAN)      // "NaN"
format!("{}", f64::INFINITY) // "inf"
format!("{}", f64::NEG_INFINITY) // "-inf"
format!("{:+}", f64::NAN)    // "NaN"   — sign flag has no effect on NaN
format!("{:08.2}", f64::NAN) // "     NaN" — padding applies
```

### 23.7 `{:p}` on References vs Raw Pointers

```rust
let x = 42_u64;
let r = &x;
println!("{:p}", r);          // prints address of x
println!("{:p}", &r);         // prints address of r (the reference itself)
println!("{:p}", r as *const u64);  // same as first
```

### 23.8 Debug for `char` Includes Quotes

```rust
format!("{}", 'a')    // "a"
format!("{:?}", 'a')  // "'a'"   — includes single quotes
format!("{:?}", '\n') // "'\\n'" — escaped
```

### 23.9 Width and Unicode

```rust
// "🦀" is U+1F980, 4 UTF-8 bytes, but ONE char (scalar value), width=1 in Rust's eyes
format!("{:5}", "🦀")  // "🦀    " — 4 spaces appended (5 - 1 = 4)
// In a terminal this looks wrong because emoji takes 2 columns
// Use unicode-width crate: UnicodeWidthStr::width("🦀") = 2
```

### 23.10 Zero-Padding Floats

```rust
format!("{:010.3}", 3.14)    // "000003.140"  — 10 chars total
format!("{:010.3}", -3.14)   // "-00003.140"  — sign is before zeros
format!("{:+010.3}", 3.14)   // "+00003.140"  — plus sign before zeros
```

---

## 24. Threat Model and Security Properties

### 24.1 Compile-Time Format String Validation

```
THREAT: Format string injection (as in C's printf(user_input))
MITIGATION: Rust format strings MUST be string literals (with minor exceptions).
            The compiler validates:
            - All {} have corresponding arguments
            - All arguments are used (warning if not)
            - Argument types implement the required trait
            - No type confusion between argument indices

// This is IMPOSSIBLE in Rust:
let user_fmt = "%s %s %n";  // C-style injection
printf(user_fmt, arg1);     // UAF/stack corruption

// In Rust, the format string must be a literal:
format!(user_input, arg);   // COMPILE ERROR: format argument must be a string literal
```

### 24.2 Information Disclosure via Debug

```
THREAT: Accidentally printing sensitive data via {:?} on structs
MITIGATION: 
  - Do NOT #[derive(Debug)] on structs containing secrets (passwords, keys, tokens)
  - Implement Debug manually, redacting secret fields:
```

```rust
use std::fmt;

pub struct ApiKey {
    pub id:     String,
    secret: Vec<u8>,   // private field
}

impl fmt::Debug for ApiKey {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ApiKey")
            .field("id",     &self.id)
            .field("secret", &"[REDACTED]")  // never print the actual secret
            .finish()
    }
}

impl fmt::Display for ApiKey {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // Display only shows the ID
        write!(f, "ApiKey({})", self.id)
    }
}
```

### 24.3 Log Injection

```
THREAT: Newlines in user-supplied values corrupting structured logs
MITIGATION: Implement Display for user-supplied strings that escapes control chars
```

```rust
pub struct SafeLogStr<'a>(pub &'a str);

impl<'a> fmt::Display for SafeLogStr<'a> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for c in self.0.chars() {
            match c {
                '\n' => f.write_str("\\n")?,
                '\r' => f.write_str("\\r")?,
                '\t' => f.write_str("\\t")?,
                c if c.is_control() => write!(f, "\\u{{{:04x}}}", c as u32)?,
                c => f.write_char(c)?,
            }
        }
        Ok(())
    }
}

let user_input = "alice\nadmin";
println!("user={}", SafeLogStr(user_input));
// user=alice\nadmin   — injection neutralized
```

### 24.4 Allocation DoS via Width/Precision

```
THREAT: Attacker-controlled width=usize::MAX causes OOM
MITIGATION: Never use attacker-controlled values as dynamic width/precision
            without bounds-checking
```

```rust
// UNSAFE PATTERN — don't do this with attacker-controlled width:
let width: usize = attacker_input.parse().unwrap(); // could be usize::MAX
format!("{:>width$}", "data", width = width);        // OOM

// SAFE PATTERN:
let width: usize = attacker_input.parse().unwrap_or(0).min(256);
format!("{:>width$}", "data", width = width);
```

---

## 25. Complete Reference Table

### 25.1 Specifier Quick Reference

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│  SPECIFIER         │ MEANING                          │ EXAMPLE          │ OUTPUT │
├────────────────────┼──────────────────────────────────┼──────────────────┼────────┤
│ {}                 │ Display                          │ format!("{}", 42)│ "42"   │
│ {:.2}              │ Display, precision=2             │ format!("{:.2}",3.14159)│"3.14"│
│ {:10}              │ Display, width=10                │ format!("{:10}","hi")│"hi        "│
│ {:>10}             │ right-align, width=10            │                  │        │
│ {:<10}             │ left-align, width=10             │                  │        │
│ {:^10}             │ center, width=10                 │                  │        │
│ {:*^10}            │ fill=*, center, width=10         │                  │        │
│ {:+}               │ force sign                       │ format!("{:+}",42)│ "+42" │
│ {:#}               │ alternate form                   │                  │        │
│ {:0}               │ zero-pad                         │                  │        │
│ {:08}              │ zero-pad to width=8              │ format!("{:08}",42)│"00000042"│
│ {:?}               │ Debug                            │ format!("{:?}","hi")│ "\"hi\""│
│ {:#?}              │ Debug, pretty-print              │                  │        │
│ {:b}               │ Binary                           │ format!("{:b}",10)│ "1010"│
│ {:#b}              │ Binary, alternate (0b prefix)    │ format!("{:#b}",10)│"0b1010"│
│ {:o}               │ Octal                            │ format!("{:o}",8) │ "10"  │
│ {:#o}              │ Octal, alternate (0o prefix)     │ format!("{:#o}",8)│"0o10"│
│ {:x}               │ LowerHex                         │ format!("{:x}",255)│ "ff" │
│ {:X}               │ UpperHex                         │ format!("{:X}",255)│ "FF" │
│ {:#x}              │ LowerHex, alternate (0x prefix)  │ format!("{:#x}",255)│"0xff"│
│ {:#010x}           │ LowerHex, alt, zero-pad, width=10│ format!("{:#010x}",255)│"0x000000ff"│
│ {:e}               │ LowerExp (scientific)            │ format!("{:e}",1000.0)│"1e3"│
│ {:E}               │ UpperExp (scientific)            │ format!("{:E}",1000.0)│"1E3"│
│ {:p}               │ Pointer (address)                │ format!("{:p}",&x)│"0x..."│
│ {:x?}              │ Debug with hex escaping          │ format!("{:x?}",'*')│"'*'"│
│ {0}                │ positional arg[0]                │ format!("{0}{0}","a")│"aa"│
│ {name}             │ named argument                   │                  │        │
│ {:width$}          │ dynamic width from arg           │                  │        │
│ {:.prec$}          │ dynamic precision from arg       │                  │        │
│ {{                 │ literal '{'                      │ format!("{{")    │ "{"   │
│ }}                 │ literal '}'                      │ format!("}}")    │ "}"   │
└────────────────────┴──────────────────────────────────┴──────────────────┴────────┘
```

### 25.2 Trait Implementation Requirements

```
To use {:?}   → implement or derive fmt::Debug
To use {}     → implement fmt::Display (does NOT auto-derive)
To use {:b}   → implement fmt::Binary
To use {:o}   → implement fmt::Octal
To use {:x}   → implement fmt::LowerHex
To use {:X}   → implement fmt::UpperHex
To use {:e}   → implement fmt::LowerExp
To use {:E}   → implement fmt::UpperExp
To use {:p}   → implement fmt::Pointer
                (raw pointers have built-in Pointer impl)

Note: implementing Display automatically enables .to_string() via ToString blanket impl
Note: all numeric primitives implement all numeric traits
Note: &T implements all traits that T implements (by delegation)
Note: Box<T> implements all traits that T implements (by delegation)
```

### 25.3 Macro Output Targets

```
format!()         → String (heap)
format_args!()    → Arguments<'_> (stack, zero-alloc)
write!()          → &mut dyn fmt::Write or &mut dyn io::Write
writeln!()        → same + appends '\n'
print!()          → stdout (locked)
println!()        → stdout (locked) + '\n'
eprint!()         → stderr (locked)
eprintln!()       → stderr (locked) + '\n'
dbg!()            → stderr + returns the value
```

---

## 26. Next 3 Steps

### Step 1 — Verify Compile-Time Behavior

Create a test file to see exactly what the compiler accepts and rejects, and examine the generated code:

```bash
cat > /tmp/fmt_test.rs << 'EOF'
use std::fmt;

fn main() {
    // Examine format_args! output at compile time
    let args = format_args!("x={:08x} y={:.3}", 255_u32, 3.14159_f64);
    println!("{}", args);

    // Examine what pad_integral does
    let mut s = String::new();
    use std::fmt::Write;
    write!(s, "{:#010x}", 255_u32).unwrap();
    println!("hex: {}", s);

    // Dynamic width
    let w = 15_usize;
    let p = 4_usize;
    println!("{:>width$.prec$}", 3.14159, width=w, prec=p);
}
EOF
rustc /tmp/fmt_test.rs -o /tmp/fmt_test && /tmp/fmt_test
# Also examine MIR/expanded macros:
rustc -Z unstable-options --edition 2021 --print=expanded /tmp/fmt_test.rs 2>/dev/null | head -100
```

### Step 2 — Implement a Production-Grade Custom Formatter

Build a structured log formatter that:
- Takes `format_args!` (no allocation)
- Outputs JSON or logfmt
- Handles all format traits
- Redacts secrets
- Benchmarks with `criterion`

```bash
cargo new --lib fmt_logger
cd fmt_logger
cargo add criterion --dev
# Implement fmt::Write on a logfmt/JSON emitter
# Benchmark: format_args! path vs format!() + write! path
```

### Step 3 — Deep Dive into `core::fmt` Source

Read the actual compiler source to understand the full implementation:

```bash
# Clone rust source
git clone --depth=1 https://github.com/rust-lang/rust.git /tmp/rust-src
# Key files to study:
# 1. Format trait definitions and macros:
cat /tmp/rust-src/library/core/src/fmt/mod.rs        | less
# 2. Runtime format spec types:
cat /tmp/rust-src/library/core/src/fmt/rt.rs          | less
# 3. Numeric formatting (pad_integral, etc.):
cat /tmp/rust-src/library/core/src/fmt/num.rs         | less
# 4. Float formatting:
cat /tmp/rust-src/library/core/src/fmt/float.rs       | less
# 5. Compiler macro expansion (format_args!):
cat /tmp/rust-src/compiler/rustc_builtin_macros/src/format.rs | less
```

---

## References

1. [Rust std::fmt documentation](https://doc.rust-lang.org/std/fmt/) — official reference
2. [Rust Reference: Tokens — String literals](https://doc.rust-lang.org/reference/tokens.html#string-literals)
3. [RFC 0640: Debug Improvements](https://rust-lang.github.io/rfcs/0640-debug-improvements.html)
4. [RFC 2795: format_args Implicit Captures](https://rust-lang.github.io/rfcs/2795-format-args-implicit-captures.html) (Rust 1.58)
5. [core/src/fmt/mod.rs](https://github.com/rust-lang/rust/blob/master/library/core/src/fmt/mod.rs)
6. [core/src/fmt/rt.rs](https://github.com/rust-lang/rust/blob/master/library/core/src/fmt/rt.rs)
7. [compiler/rustc_builtin_macros/src/format.rs](https://github.com/rust-lang/rust/blob/master/compiler/rustc_builtin_macros/src/format.rs)
8. [unicode-width crate](https://crates.io/crates/unicode-width) — correct terminal column widths
9. [The Rustonomicon: Unsafe Rust and fmt](https://doc.rust-lang.org/nomicon/)
