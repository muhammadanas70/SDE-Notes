# Expressions, Statements, Declarations & Literals
## A World-Class Deep-Dive in C, Go, and Rust

> *"To understand a language is to understand the grammar of thought it imposes on the programmer."*
> — Systems Programming Wisdom

---

## Table of Contents

1. [The Mental Model: Grammar of Programming Languages](#1-the-mental-model-grammar-of-programming-languages)
2. [Literals — The Atomic Values](#2-literals--the-atomic-values)
3. [Expressions — Things That Produce Values](#3-expressions--things-that-produce-values)
4. [Statements — Things That Do Work](#4-statements--things-that-do-work)
5. [Declarations — Things That Introduce Names](#5-declarations--things-that-introduce-names)
6. [The Expression-Statement Continuum](#6-the-expression-statement-continuum)
7. [Scope, Lifetime, and the Compilation Model](#7-scope-lifetime-and-the-compilation-model)
8. [Hardware Reality: What the CPU Actually Sees](#8-hardware-reality-what-the-cpu-actually-sees)
9. [Language-by-Language Deep Comparison](#9-language-by-language-deep-comparison)
10. [Production-Grade Examples](#10-production-grade-examples)
11. [Cognitive Framework: Mental Models for Expert Thinking](#11-cognitive-framework-mental-models-for-expert-thinking)

---

## 1. The Mental Model: Grammar of Programming Languages

### 1.1 Why These Four Concepts Are the Foundation

Every programming language, no matter how complex, is built from four grammatical atoms:

```
┌─────────────────────────────────────────────────────────────────────┐
│                  GRAMMAR OF A PROGRAMMING LANGUAGE                  │
│                                                                     │
│   LITERAL          EXPRESSION         STATEMENT       DECLARATION   │
│  ─────────        ────────────        ─────────       ───────────   │
│  Raw value        Computation         Action          Introduction  │
│  baked into       that yields         that causes     of a name     │
│  source code      a value             a side effect   into scope    │
│                                                                     │
│   42              2 + 2               x = 4;          int x;        │
│   "hello"         f(x)                return x;       fn foo() {}   │
│   3.14            x > 0               for {...}       type T struct  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

Think of them like this using English grammar as analogy:

```
┌───────────────┬──────────────────────────────────┬──────────────────────┐
│  PL Concept   │  English Grammar Equivalent      │  Core Property       │
├───────────────┼──────────────────────────────────┼──────────────────────┤
│  Literal      │  Noun (a concrete object: "cat") │  Has a fixed value   │
│  Expression   │  Noun phrase ("the big cat")     │  Evaluates to value  │
│  Statement    │  Complete sentence ("Run!")       │  Has an effect       │
│  Declaration  │  Introduction ("Let me tell you  │  Creates a binding   │
│               │   about someone named Alice...")  │  (name → thing)      │
└───────────────┴──────────────────────────────────┴──────────────────────┘
```

### 1.2 How Each Language Classifies These

The same surface syntax means different things in different languages:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                   CLASSIFICATION COMPARISON                                  │
│                                                                              │
│  Concept         C                 Go               Rust                    │
│  ────────────    ──────────────    ──────────────   ──────────────────────  │
│                                                                              │
│  if              Statement         Statement        EXPRESSION (has value!)  │
│  loop/for        Statement         Statement        EXPRESSION (has value!)  │
│  block { }       Statement (cmpd)  Statement        EXPRESSION (has value!)  │
│  assignment      Expression (!)    Statement        Statement                │
│  function call   Expression        Expression       Expression               │
│  let/var/int     Declaration       Statement(!)     Statement(!)             │
│                                                                              │
│  NOTE: Languages disagree on where the boundaries are.                      │
│  This is intentional design philosophy, not accident.                       │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 1.3 Parse Tree: What a Compiler Actually Sees

When the compiler reads your source code, it builds a Parse Tree (also called AST — Abstract Syntax Tree). Understanding this is the key to thinking like a compiler.

```
Source: int x = 2 + 3 * 4;
                                                                      
               Declaration
                    │
         ┌──────────┴──────────┐
      Specifier              Declarator
      (int)              ┌────┴────┐
                       Name     Initializer
                       (x)         │
                              Expression
                                   │
                            BinaryExpr (+)
                           ┌───────┴──────────┐
                        Literal           BinaryExpr (*)
                          (2)            ┌────┴────┐
                                      Literal   Literal
                                        (3)       (4)
                                                      
  Evaluation order (bottom-up):
  Step 1: 3 * 4 = 12  (leaf nodes evaluated first)
  Step 2: 2 + 12 = 14
  Step 3: int x = 14  (declaration binds name 'x' to value 14)
```

The compiler walks this tree to generate machine code. Every rule below corresponds to a node type in this tree.

---

## 2. Literals — The Atomic Values

### 2.1 What Is a Literal?

A **literal** is a fixed value written directly in source code. It has no name, no address (until the compiler decides to give it one), and no computation. It is the simplest possible expression — the base case of the expression grammar.

**Key Properties of Literals:**
- They are evaluated at **compile time** (the compiler knows their value before running the program)
- They have a **type** determined by their form (e.g., `42` is an integer, `42.0` is a float)
- They may be stored in **read-only memory** (rodata section), on the **stack**, or embedded **directly in instructions**

### 2.2 Memory Placement of Literals

This is where systems understanding begins. Where does `42` live?

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    LITERAL PLACEMENT IN MEMORY                           │
│                                                                          │
│  Source Code                  Compiled Output                            │
│  ─────────────                ──────────────                             │
│                                                                          │
│  int x = 42;         →    MOV [rbp-4], 42      ← 42 is IMMEDIATE        │
│                            (embedded in instruction stream)              │
│                                                                          │
│  char *s = "hello";  →    s points to .rodata  ← string in READ-ONLY    │
│                            section of ELF/Mach-O binary                  │
│                                                                          │
│  const double PI =   →    FLD [.rodata+offset] ← 8-byte float in rodata │
│    3.14159265358979;                                                     │
│                                                                          │
│  int arr[] =         →    .data section        ← global literal arrays   │
│    {1, 2, 3};              (initialized data)                            │
│                                                                          │
│  Memory Layout:                                                          │
│  ┌──────────────────────────────────────────────────────────────┐        │
│  │  .text   │ .rodata │  .data  │  .bss   │  heap  │  stack   │        │
│  │ (code)   │ (const) │(init'd) │(uninit) │        │          │        │
│  │          │         │         │         │        │          │        │
│  │ MOV 42,  │ "hello" │ {1,2,3} │         │        │ x=42 ←  │        │
│  │ [rbp-4]  │ \0      │         │         │        │ frame    │        │
│  └──────────────────────────────────────────────────────────────┘        │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.3 Integer Literals

#### C — Integer Literals

C integer literals can be expressed in four bases and have suffix modifiers:

```c
// ─────────────────────────────────────────────────
//  C INTEGER LITERALS — Complete Reference
// ─────────────────────────────────────────────────

// Decimal (base 10): no prefix
int a = 42;
int b = 1000000;

// Octal (base 8): prefix 0 (zero)
int c = 052;      // = 42 decimal  (WARNING: leading 0 means OCTAL!)
int d = 0755;     // = 493 decimal (common in file permissions)

// Hexadecimal (base 16): prefix 0x or 0X
int e = 0x2A;     // = 42 decimal
int f = 0xFF;     // = 255 decimal
int g = 0xDEADBEEF; // classic debug pattern

// Binary (base 2): prefix 0b — C23 standard, GCC extension pre-C23
int h = 0b101010; // = 42 decimal

// Type suffixes determine the type:
// No suffix     → int
// L or l        → long
// LL or ll      → long long
// U or u        → unsigned int
// UL or ul      → unsigned long
// ULL or ull    → unsigned long long

long           x1 = 42L;
long long      x2 = 42LL;
unsigned int   x3 = 42U;
unsigned long  x4 = 42UL;

// Digit separators (C23): apostrophe as separator (not yet universal)
// int big = 1'000'000;  // may not be supported in all C compilers yet
```

**Critical Trap:** The octal prefix is one of C's oldest gotchas.

```c
// BUG: programmer writes "decimal" but compiler reads octal
int permissions = 0644;  // NOT 644! It's 6*64 + 4*8 + 4 = 420 decimal
// This is intentional — Unix permissions ARE octal, but confusing elsewhere.

// SAFE: use explicit hex or decimal when you don't want octal
int not_octal = 644;    // actually 644 decimal
int explicit  = 0x284;  // 644 decimal in hex (clear intention)
```

#### Go — Integer Literals

```go
// ─────────────────────────────────────────────────
//  GO INTEGER LITERALS — Complete Reference
// ─────────────────────────────────────────────────

package main

import "fmt"

func main() {
    // Decimal
    a := 42
    b := 1_000_000  // underscore as digit separator (Go 1.13+)

    // Octal: prefix 0o (Go 1.13+) or legacy prefix 0
    c := 0o52     // = 42 (EXPLICIT octal, safer than C's)
    d := 052      // = 42 (legacy octal — same as C trap, but Go keeps for compat)

    // Hexadecimal
    e := 0x2A
    f := 0xFF
    g := 0xDEAD_BEEF  // underscores for readability

    // Binary
    h := 0b101010  // = 42

    // Go has NO type suffixes on literals.
    // Instead, the type is inferred from context:
    var x int64 = 42       // literal 42 becomes int64
    var y uint8 = 42       // literal 42 becomes uint8
    var z float64 = 42     // literal 42 becomes float64 (integer literal, float type)

    // Untyped constants (key Go concept):
    const Big = 1 << 62    // "untyped int" — no fixed size until used
    const Small = Big >> 61 // still untyped

    fmt.Println(a, b, c, d, e, f, g, h, x, y, z, Small)
}

// IMPORTANT: Go's "untyped" constants are a powerful concept.
// An untyped constant has a "default type" but can be used wherever
// its value fits. They have arbitrary precision at compile time.
```

#### Rust — Integer Literals

```rust
// ─────────────────────────────────────────────────
//  RUST INTEGER LITERALS — Complete Reference
// ─────────────────────────────────────────────────

fn main() {
    // Decimal
    let a = 42;
    let b = 1_000_000;   // underscore separator — idiomatic in Rust

    // Octal: prefix 0o (explicit, NO legacy 0 prefix)
    let c = 0o52;        // = 42 — Rust has NO octal-via-leading-zero trap

    // Hexadecimal
    let d = 0x2A;
    let e = 0xFF;
    let f = 0xDEAD_BEEF_u32;  // suffix inline with value

    // Binary
    let g = 0b101010;    // = 42

    // Type suffixes are PART OF the literal (not separate):
    let h: i32  = 42;         // type annotation on variable
    let i       = 42i32;      // type suffix on literal
    let j       = 42_u64;     // u64 with separator for readability
    let k       = 42_usize;   // usize — pointer-sized unsigned integer

    // Rust integer types: i8, i16, i32, i64, i128, isize
    //                     u8, u16, u32, u64, u128, usize
    // Default inferred type: i32

    // Byte literal (u8 value from ASCII character):
    let byte_val: u8 = b'A';  // = 65
    let byte_esc: u8 = b'\n'; // = 10

    println!("{a} {b} {c} {d} {e} {g} {h} {i} {j} {k} {byte_val} {byte_esc}");
}
```

### 2.4 Floating-Point Literals

```
┌─────────────────────────────────────────────────────────────────────────┐
│              IEEE 754 FLOAT LAYOUT (64-bit double)                      │
│                                                                         │
│  Bit 63    Bits 62-52        Bits 51-0                                  │
│  ┌───┬────────────────┬────────────────────────────────────────────┐    │
│  │ S │   Exponent     │              Mantissa (Fraction)           │    │
│  │ 1 │   11 bits      │              52 bits                       │    │
│  └───┴────────────────┴────────────────────────────────────────────┘    │
│                                                                         │
│  Value = (-1)^S × 2^(Exponent - 1023) × 1.Mantissa                     │
│                                                                         │
│  Special values encoded by Exponent = all-1s:                          │
│    Mantissa = 0  → ±Infinity                                            │
│    Mantissa ≠ 0  → NaN (Not a Number)                                  │
│                                                                         │
│  This is why: 0.1 + 0.2 ≠ 0.3  (cannot represent 0.1 exactly in base2) │
└─────────────────────────────────────────────────────────────────────────┘
```

```c
// ── C FLOAT LITERALS ──────────────────────────────
double a = 3.14;        // double (64-bit) — no suffix = double
float  b = 3.14f;       // float (32-bit) — 'f' or 'F' suffix
long double c = 3.14L;  // long double (80-bit on x86, 128-bit on some)

// Scientific notation
double d = 1.5e3;       // = 1500.0
double e = 1.5e-3;      // = 0.0015
double f = 1.5E+10;     // = 15000000000.0

// Hexadecimal float (C99+) — exact representation
// Form: 0x<hex>.<hex>p<decimal_exponent_of_2>
double g = 0x1.8p+1;    // = 1.5 * 2^1 = 3.0 (exact in IEEE 754)
double h = 0x1.0p-52;   // = machine epsilon (smallest distinguishable delta)

// Special: INFINITY and NAN (from <math.h>)
double inf = INFINITY;
double nan = NAN;
```

```go
// ── GO FLOAT LITERALS ──────────────────────────────
a := 3.14           // default: float64 (Go always defaults to 64-bit)
var b float32 = 3.14
c := 1.5e3          // = 1500.0
d := 1.5e-3         // = 0.0015

// Hex float (Go 1.13+)
e := 0x1.8p+1       // = 3.0 exactly

// Go: NO float32/float64 suffix on literals.
// Use variable type annotation to select precision.
```

```rust
// ── RUST FLOAT LITERALS ──────────────────────────────
let a = 3.14;           // default: f64
let b: f32 = 3.14;      // f32 via annotation
let c = 3.14_f32;       // f32 via suffix
let d = 3.14_f64;       // f64 via suffix
let e = 1.5e3_f64;      // scientific + suffix
let f = 1.0;            // MUST have decimal point — '1' alone is int
// let g = 1;           // ERROR if used where f64 expected

// Rust float types: f32 (single), f64 (double)
// Default: f64

// Special values
let inf = f64::INFINITY;
let neg_inf = f64::NEG_INFINITY;
let nan = f64::NAN;
let eps = f64::EPSILON;  // 2^-52 for f64
```

### 2.5 Character and String Literals

#### Understanding String Memory Layout

```
Source: "hello"
                                                                        
  Compile Time            Run Time Memory (.rodata section)
  ────────────            ──────────────────────────────────
  "hello"      ─────→    Address  │ Byte │ Meaning
                          0x2000  │ 0x68 │ 'h'
                          0x2001  │ 0x65 │ 'e'
                          0x2002  │ 0x6C │ 'l'
                          0x2003  │ 0x6C │ 'l'
                          0x2004  │ 0x6F │ 'o'
                          0x2005  │ 0x00 │ NUL terminator (C only!)
                                           
  In C:   char *p = "hello"; → p = 0x2000, *p = 'h'
  In Go:  "hello"  → string header {ptr: 0x2000, len: 5}  (NO null term)
  In Rust: "hello" → &str {ptr: 0x2000, len: 5}           (NO null term)
```

```c
// ── C CHARACTER AND STRING LITERALS ──────────────────

// Character literals: single quotes, type = int (NOT char!)
char ch1 = 'A';      // ASCII 65 — stored as int, assigned to char
int  ch2 = 'A';      // type is actually int in C
int  ch3 = '\n';     // newline: 10
int  ch4 = '\t';     // tab: 9
int  ch5 = '\\';     // backslash: 92
int  ch6 = '\'';     // single quote: 39
int  ch7 = '\0';     // null character: 0
int  ch8 = '\x41';   // hex escape: 'A' (65)
int  ch9 = '\101';   // octal escape: 'A' (65)

// Wide character literals (for Unicode/multibyte):
wchar_t wch = L'Ω';        // wide char
char16_t ch16 = u'Ω';     // UTF-16 (C11)
char32_t ch32 = U'Ω';     // UTF-32 (C11)

// String literals: double quotes, type = char[] (array), decays to char*
const char *s1 = "hello";           // pointer to read-only memory
char        s2[] = "hello";         // copy into mutable char array (6 bytes incl NUL)
const char *s3 = "hello" " world";  // adjacent string concatenation at compile time
const char *s4 = "line1\n"
                 "line2\n";         // same: compiler merges adjacent string literals

// Wide strings
const wchar_t  *ws = L"hello";    // wide string
const char16_t *u  = u"hello";    // UTF-16
const char32_t *U  = U"hello";    // UTF-32
const char     *u8 = u8"hello";   // UTF-8 guaranteed (C11+)

// Raw/multiline: C has no raw string literal syntax (use escapes)
const char *path = "C:\\Users\\Name\\file.txt";   // must escape backslashes

// CRITICAL DIFFERENCE:
char *mutable_attempt = "hello";   // Compiles but UNDEFINED BEHAVIOR to modify!
char  mutable_copy[]  = "hello";   // This is SAFE to modify — it's a copy on stack
```

```go
// ── GO CHARACTER AND STRING LITERALS ──────────────────

// Rune literals (Go's character type = int32 = Unicode code point)
var r1 rune = 'A'       // = 65
var r2 rune = '界'       // = 30028 (Chinese character, a single rune)
var r3 rune = '\n'      // = 10
var r4 rune = '\u0041'  // = 65 ('A') via Unicode escape
var r5 rune = '\U00004E16' // Unicode escape (8 hex digits)
var r6 rune = '\x41'    // hex escape = 'A'
var r7 rune = '\101'    // octal escape = 'A'

// String literals: TWO kinds in Go

// 1. Interpreted string: double quotes — escape sequences processed
s1 := "hello\nworld"  // contains actual newline
s2 := "tab:\there"    // actual tab
s3 := "unicode: \u4e16\u754c"  // "世界"

// 2. Raw string literal: backticks — NO escape processing
s4 := `hello\nworld`  // literally: hello\nworld (backslash-n, not newline)
s5 := `path: C:\Users\Name`  // no escaping needed
s6 := `
Line 1
Line 2
Line 3
`  // multiline raw string — preserves all whitespace

// Go strings are immutable sequences of bytes (UTF-8 by convention)
// A string is a struct: { pointer, length } — no null terminator
// Iterating by byte vs rune:
for i, b := range []byte(s1) { _ = i; _ = b }  // byte index, byte value
for i, r := range s1 { _ = i; _ = r }           // byte index, rune value (decodes UTF-8)
```

```rust
// ── RUST CHARACTER AND STRING LITERALS ──────────────────

// char literal: a Unicode scalar value (always 4 bytes = u32)
let c1: char = 'A';          // ASCII = U+0041
let c2: char = '界';          // Chinese = U+754C
let c3: char = '\n';         // newline
let c4: char = '\t';         // tab
let c5: char = '\\';         // backslash
let c6: char = '\'';         // single quote
let c7: char = '\0';         // null char
let c8: char = '\x41';       // hex escape (1 byte range: \x00 to \xFF)
let c9: char = '\u{4E16}';   // Unicode escape (1–6 hex digits)
let c10: char = '\u{1F600}'; // emoji: 😀 (U+1F600)

// &str (string slice) — compile-time string literal → 'static lifetime
let s1: &str = "hello";          // UTF-8 guaranteed, immutable, in .rodata
let s2: &'static str = "hello";  // explicit 'static lifetime
let s3 = "hello\nworld";         // escape sequences interpreted
let s4 = "unicode: \u{4E16}";    // Unicode escape in string

// Raw string literal: r#"..."# — no escapes processed
let s5 = r"hello\nworld";        // literally: hello\nworld
let s6 = r"C:\Users\Name\file";  // no escaping
let s7 = r#"He said "hello""#;   // contains double quotes (use #)
let s8 = r##"raw with "# inside"##; // more # to handle # inside

// Byte string literal: b"..." → type &[u8]
let b1: &[u8] = b"hello";       // array of bytes, NOT a string
let b2: &[u8] = b"\x41\x42";   // = [65, 66] = b"AB"

// Raw byte string: rb"..." or br"..."
let b3: &[u8] = br"raw bytes\n"; // no escapes

// String type vs &str — critical distinction:
let owned: String = String::from("hello");  // heap-allocated, growable
let slice: &str   = &owned;                 // borrow of the heap string
let literal: &str = "hello";               // points to .rodata
// Both &str and &String deref to a string slice internally.
```

### 2.6 Boolean Literals

```c
// C — NO native bool until C99's <stdbool.h>
#include <stdbool.h>
bool t = true;    // = 1 (any non-zero is truthy in C)
bool f = false;   // = 0
// Without stdbool: true and false are macros: #define true 1, #define false 0
// C23 adds keywords bool, true, false natively
```

```go
// Go — bool is a distinct type, NOT an integer
var t bool = true
var f bool = false
// You CANNOT do: if 1 { } — only boolean allowed in conditions
// You CANNOT do: x := true + false — no arithmetic on bool
```

```rust
// Rust — bool is a distinct type (1 byte, but bool arithmetic is restricted)
let t: bool = true;
let f: bool = false;
// true as u8 == 1, false as u8 == 0 — explicit cast required
let n = true as u8;  // = 1u8
// Rust also does NOT allow integers in boolean positions
// if 1 { }  ← COMPILE ERROR
```

### 2.7 Composite / Compound Literals

These are "literal" values for compound types — arrays, structs, tuples:

```c
// ── C COMPOUND LITERALS ────────────────────────────────────────────────

// Array initializer (NOT technically a "literal" in C standard, but behaves like one)
int arr[3] = {1, 2, 3};
int arr2[]  = {1, 2, 3};    // size inferred from initializer (3)
int arr3[5] = {1, 2};       // remaining elements = 0: {1, 2, 0, 0, 0}
int arr4[5] = {0};           // all zeros — common idiom
int arr5[5] = {};            // C99+: all zeros (same as above)

// Designated initializers (C99+) — named elements
struct Point { int x; int y; };
struct Point p1 = {1, 2};             // positional
struct Point p2 = {.x = 1, .y = 2};  // designated (order independent!)
struct Point p3 = {.y = 5};           // .x defaults to 0

// Compound literal (C99): (type){initializer} — creates anonymous temporary
struct Point *get_origin(void) {
    // Returns pointer to compound literal — lifetime is enclosing block
    return &(struct Point){.x = 0, .y = 0};
}
// WARNING: the compound literal above is stack-allocated and only lives
// as long as the enclosing scope. Returning a pointer to it may dangle!

// Safer: use it directly without taking address of temporary:
void use_point(struct Point p) { /* use p */ }
// use_point((struct Point){.x=1, .y=2}); // pass by value — safe
```

```go
// ── GO COMPOSITE LITERALS ──────────────────────────────────────────────

type Point struct {
    X, Y int
}

// Struct literal
p1 := Point{1, 2}             // positional (field order matters)
p2 := Point{X: 1, Y: 2}      // named fields (recommended — order independent)
p3 := Point{Y: 5}             // X defaults to zero value (0)
p4 := Point{}                 // zero value struct: {X:0, Y:0}

// Pointer to composite literal — Go allocates on heap automatically
p5 := &Point{X: 1, Y: 2}     // *Point, heap-allocated (Go's escape analysis decides)

// Slice literal (dynamic array)
s1 := []int{1, 2, 3}              // slice of 3 ints
s2 := []string{"a", "b", "c"}
s3 := []Point{{1, 2}, {3, 4}}     // slice of structs (nested composite literals)

// Array literal (fixed size)
a1 := [3]int{1, 2, 3}
a2 := [...]int{1, 2, 3}           // ... lets compiler count = [3]int
a3 := [5]int{0: 1, 2: 99}        // designated: [1, 0, 99, 0, 0]

// Map literal (hash map)
m1 := map[string]int{
    "one":   1,
    "two":   2,
    "three": 3,
}
m2 := map[string]int{}   // empty map (different from nil map!)
var m3 map[string]int    // nil map — reading is safe, writing PANICS
```

```rust
// ── RUST STRUCT, ARRAY, TUPLE LITERALS ─────────────────────────────────

struct Point {
    x: i32,
    y: i32,
}

struct Color(u8, u8, u8);  // tuple struct

// Struct literal (always named fields)
let p1 = Point { x: 1, y: 2 };
let p2 = Point { x: 0, ..p1 }; // struct update syntax: x=0, y=2 from p1

// Tuple literal
let t1: (i32, f64, bool) = (42, 3.14, true);
let t2 = (1, 2, 3);
let first = t2.0;            // access by index
let (a, b, c) = t2;         // destructuring

// Array literal (fixed size, same type)
let arr1: [i32; 3] = [1, 2, 3];
let arr2 = [0i32; 5];        // repeat syntax: [0, 0, 0, 0, 0]
let arr3 = [1, 2, 3];        // type inferred: [i32; 3]

// Slice literal (reference to array data — no heap allocation!)
let slice: &[i32] = &[1, 2, 3];  // points to stack-allocated data

// Vec literal (heap-allocated, growable)
let v1: Vec<i32> = vec![1, 2, 3]; // macro expands to Vec::from([1,2,3])
let v2 = vec![0i32; 5];           // [0,0,0,0,0]

// Tuple struct literal
let red = Color(255, 0, 0);
let r = red.0;  // access field 0
```

### 2.8 The `null` / `nil` / `None` — The Zero Pointer Literal

```
┌──────────────────────────────────────────────────────────────────────┐
│              NULL REPRESENTATION ACROSS LANGUAGES                    │
│                                                                      │
│  C:    NULL  → (void*)0 — just zero cast to pointer — NOT SAFE      │
│               Dereferencing NULL = undefined behavior (usually crash) │
│                                                                      │
│  Go:   nil   → zero value for: pointers, slices, maps, channels,    │
│               functions, interfaces — each type's "empty" sentinel   │
│               Dereferencing nil pointer = runtime panic              │
│                                                                      │
│  Rust: None  → the None variant of Option<T> — TYPE SAFE!           │
│               The type system PREVENTS you from dereferencing None   │
│               You MUST match or unwrap to get the value              │
│                                                                      │
│  Philosophy:                                                         │
│  C:    "Trust me, I know this pointer is valid"  → Danger           │
│  Go:   "Check for nil before using"  → Runtime safety               │
│  Rust: "Prove to the compiler the value exists"  → Compile-time      │
└──────────────────────────────────────────────────────────────────────┘
```

```c
// C null — the billion-dollar mistake
int *p = NULL;
// *p = 5;  // undefined behavior — likely segfault
if (p != NULL) {
    *p = 5;  // safe
}
```

```go
// Go nil
var s []int             // s == nil (uninitialized slice)
var m map[string]int    // m == nil (uninitialized map)
var ptr *int            // ptr == nil (nil pointer)
var fn_ func()          // fn_ == nil (nil function)

if m == nil {
    m = make(map[string]int) // initialize before use
}
```

```rust
// Rust None — compiler-enforced null safety
let maybe: Option<i32> = None;
let value: Option<i32> = Some(42);

// Must handle None explicitly — compiler refuses to compile otherwise
match maybe {
    Some(n) => println!("Got: {n}"),
    None    => println!("Nothing"),
}

// Idiomatic shortcuts:
let n = value.unwrap_or(0);            // returns 0 if None
let n = value.unwrap_or_default();     // returns i32::default() = 0
let n = value.map(|x| x * 2);         // transforms inner value, stays Option<i32>
let n = value.and_then(|x| Some(x*2)); // flatmap (for chaining Options)
```

---

## 3. Expressions — Things That Produce Values

### 3.1 The Core Definition

An **expression** is any syntactic unit that evaluates to a value. The defining property is: it can appear on the **right side of an assignment** (or wherever a value is expected).

```
WHAT IS AN EXPRESSION?
                                                                       
  Input              Process              Output
  ─────              ───────              ──────
  Operands    +      Operator(s)   =      Value
  (literals,         (arithmetic,          (a typed
   variables,         logical,              result)
   function           comparison,
   calls, etc.)       bitwise, etc.)

Test: "Does it produce a value?"
  YES → Expression
  NO  → Statement (probably)
```

### 3.2 Expression Tree and Evaluation Order

```
Expression: a + b * c - (d / e)

            Evaluation Tree (AST)
            ─────────────────────
                    SUB (-)
                   /       \
                ADD (+)     DIV (/)
               /    \      /   \
              a    MUL(*) d     e
                   /   \
                  b     c

Evaluation Order (respecting operator precedence):
  Step 1: b * c         → temp1
  Step 2: a + temp1     → temp2
  Step 3: d / e         → temp3
  Step 4: temp2 - temp3 → result

Operator Precedence (C, Go, Rust all similar):
  Highest: () function call, [] indexing, . member access
           ++ -- (C) / unary: - ! ~ & *
           * / % (multiplicative)
           + - (additive)
           << >> (shift)
           < <= > >= (relational)
           == != (equality)
           & (bitwise AND)
           ^ (bitwise XOR)
           | (bitwise OR)
           && (logical AND)
           || (logical OR)
  Lowest:  = += -= *= etc. (assignment — right-to-left)
```

### 3.3 Categories of Expressions

#### Primary Expressions (atoms — cannot be broken down further)

```c
// In C, Go, and Rust — primary expressions are:
42          // integer literal
3.14        // float literal
'A'         // char literal
"hello"     // string literal
true        // boolean literal
x           // variable name (identifier)
(expr)      // parenthesized expression (used for grouping)
```

#### Arithmetic Expressions

```c
// ── C ───────────────────────────────────────────────
int a = 10, b = 3;
int add = a + b;   // 13
int sub = a - b;   // 7
int mul = a * b;   // 30
int div = a / b;   // 3  — INTEGER DIVISION (truncates toward zero in C99+)
int mod = a % b;   // 1
int neg = -a;      // -10 (unary negation)

// C integer division truncates toward zero:
// (-7) / 2 = -3 (NOT -4)
// 7 / (-2) = -3

// CRITICAL: integer overflow in C is UNDEFINED BEHAVIOR for signed int
// This is NOT an error — the compiler can assume it never happens!
int x = INT_MAX + 1;  // UB! compiler may "optimize" this to anything
unsigned int y = UINT_MAX + 1;  // defined: wraps to 0 (modular arithmetic)
```

```go
// ── Go ──────────────────────────────────────────────
a, b := 10, 3
add := a + b  // 13
sub := a - b  // 7
mul := a * b  // 30
div := a / b  // 3 — integer division
mod := a % b  // 1

// Go integer overflow: wraps around (defined behavior, no UB)
// But the math/bits package can detect overflow explicitly.

// Go has NO -- and ++ expressions (only statements)
// x++ is valid as a STATEMENT, but NOT in an expression:
// y := x++  ← COMPILE ERROR in Go
```

```rust
// ── Rust ────────────────────────────────────────────
let a: i32 = 10;
let b: i32 = 3;
let add = a + b;
let sub = a - b;
let mul = a * b;
let div = a / b;
let rem = a % b;  // Note: Rust calls it "remainder", not "modulo" — different for negatives

// CRITICAL: In debug mode, Rust PANICS on integer overflow
// In release mode (--release), it wraps (two's complement)
// Use explicit wrapping/checked/saturating methods for production:
let wrapped   = a.wrapping_add(i32::MAX);      // always wraps
let checked   = a.checked_add(i32::MAX);       // returns Option<i32>
let saturated = a.saturating_add(i32::MAX);    // clamps to MAX
let (result, overflowed) = a.overflowing_add(i32::MAX); // returns (result, bool)
```

#### Comparison Expressions

```c
// ── C ───────────────────────────────────────────────
// Result type is int (0 = false, 1 = true) in C
int lt = (a < b);   // 0 (false)
int gt = (a > b);   // 1 (true)
int le = (a <= b);  // 0
int ge = (a >= b);  // 1
int eq = (a == b);  // 0
int ne = (a != b);  // 1

// TRAP: comparing pointers with == compares addresses, not values!
char *s1 = "hello";
char *s2 = "hello";
if (s1 == s2) { /* may or may not be true — compiler may merge strings */ }
// Use strcmp() to compare string CONTENTS.
```

```go
// ── Go ──────────────────────────────────────────────
// Result type is bool
lt := a < b   // false
gt := a > b   // true
eq := a == b  // false

// Go strings CAN be compared with == (compares contents, not pointers)
s1 := "hello"
s2 := "hello"
if s1 == s2 { /* TRUE — Go compares string values */ }
```

```rust
// ── Rust ────────────────────────────────────────────
// Result type is bool
let lt = a < b;
let gt = a > b;
let eq = a == b;

// Rust: == on String compares contents; == on &str compares contents
// Use std::ptr::eq() to compare pointer addresses explicitly
```

#### Logical Expressions

```c
// ── C ───────────────────────────────────────────────
int p = 1, q = 0;  // C uses int for bool-like values

// Short-circuit evaluation: && and || stop evaluating once result is determined
int r1 = p && q;   // 0 — evaluates p (true), then q (false), stops, returns 0
int r2 = p || q;   // 1 — evaluates p (true), stops immediately, returns 1
int r3 = !p;       // 0 — logical NOT

// TRAP: & and | are BITWISE, not logical:
int x = 3, y = 5;
int a1 = x & y;   // BITWISE AND: 011 & 101 = 001 = 1
int a2 = x && y;  // LOGICAL AND: (nonzero) && (nonzero) = 1

// Ternary (conditional) expression — C/Go/... (NOT Rust)
int max = (a > b) ? a : b;  // if a>b then a else b — this is an EXPRESSION
```

```go
// ── Go ──────────────────────────────────────────────
p, q := true, false
r1 := p && q   // false
r2 := p || q   // true
r3 := !p       // false

// Go has NO ternary operator. Use if expression... except it's a statement.
// Idiomatic Go way:
max := a
if b > a {
    max = b
}
// Or for functions: use math.Max(float64(a), float64(b))
```

```rust
// ── Rust ────────────────────────────────────────────
let p = true;
let q = false;
let r1 = p && q;  // false
let r2 = p || q;  // true
let r3 = !p;      // false

// Rust has NO ternary operator either, but if IS an expression!
let max = if a > b { a } else { b };  // if-else as expression

// Rust's ? operator: propagates errors (expression that returns early)
fn read_file() -> Result<String, std::io::Error> {
    let content = std::fs::read_to_string("file.txt")?; // ? = expression
    Ok(content)
}
```

#### Bitwise Expressions

```
BITWISE OPERATION REFERENCE
                                                                    
  Input A:  0101 1010  (decimal: 90)
  Input B:  0011 1100  (decimal: 60)

  AND  (&):  0001 1000  = 24   (1 only where BOTH are 1)
  OR   (|):  0111 1110  = 126  (1 where EITHER is 1)
  XOR  (^):  0110 0110  = 102  (1 where inputs DIFFER)
  NOT  (~):  1010 0101  = 165  (flips every bit — for A)

  Left Shift  (A << 2): 0110 1000  = 360  (multiply by 2^n)
  Right Shift (A >> 2): 0001 0110  = 22   (divide by 2^n for unsigned)

  Bit manipulation patterns (all languages):
  Set bit n:    x |=  (1 << n)    // force bit n to 1
  Clear bit n:  x &= ~(1 << n)    // force bit n to 0
  Toggle bit n: x ^=  (1 << n)    // flip bit n
  Test bit n:   x  &  (1 << n)    // non-zero if bit n is 1
```

#### Assignment Expressions (C only as expression — important difference!)

```c
// ── C: assignment is an EXPRESSION ─────────────────
// This is a major C design choice with deep consequences:
int x;
int y = (x = 5);  // x=5 is an expression with value 5; y=5

// Common idiom — read until EOF:
while ((c = getchar()) != EOF) {
    // c = getchar() is an expression, its value is compared to EOF
    process(c);
}

// Assignment chain:
int a, b, c;
a = b = c = 0;  // right-associative: c=0, then b=(c=0)=0, then a=(b=0)=0

// TRAP: assignment vs comparison — the most famous C bug:
if (x = 5) {   // ASSIGNS 5 to x, then tests if 5 is nonzero (TRUE)
    // This is NOT comparing x to 5! It's x=5 then if(5) which is always true
}
// Correct:
if (x == 5) { /* comparison */ }
// Some programmers write: if (5 == x) — "Yoda condition" — compiler error if = used
```

```go
// ── Go: assignment is a STATEMENT ─────────────────
// You CANNOT use assignment as an expression in Go
// x := y = 5    ← SYNTAX ERROR
// while((c = read()) != EOF) ← SYNTAX ERROR in Go

// Go idiom for the same pattern:
for {
    c, err := reader.ReadByte()
    if err == io.EOF { break }
    process(c)
}
```

```rust
// ── Rust: assignment is a STATEMENT (returns ()) ──
// Assignment evaluates to () (unit type — empty tuple)
let x: () = { let mut y = 0; y = 5; };  // y=5 produces ()
// let z = (x = 5);  ← z would be of type (), not 5

// No chained assignment:
// a = b = 0;  ← ERROR (b=0 is (), cannot assign () to a)
```

### 3.4 Function Call Expressions

```c
// ── C ───────────────────────────────────────────────
#include <stdio.h>
#include <math.h>

int add(int a, int b) { return a + b; }

int result = add(3, 4);     // function call expression — value is 7
double sq  = sqrt(9.0);     // standard library call
printf("hi");               // call for side effect — return value (int) discarded
(void)printf("hi");         // explicit discard — suppresses "unused return" warning

// Function pointer call
int (*fn_ptr)(int, int) = add;
int r = fn_ptr(3, 4);   // call through pointer — same syntax
```

```go
// ── Go ──────────────────────────────────────────────
func add(a, b int) int { return a + b }

result := add(3, 4)

// Multiple return values — Go's powerful feature:
func divmod(a, b int) (int, int) {
    return a / b, a % b
}
q, r := divmod(10, 3)   // both values assigned
q2, _ := divmod(10, 3)  // discard second with blank identifier

// Named return values:
func split(sum int) (x, y int) {
    x = sum * 4 / 9
    y = sum - x
    return  // "naked return" — returns x and y
}
```

```rust
// ── Rust ────────────────────────────────────────────
fn add(a: i32, b: i32) -> i32 { a + b }  // last expr without semicolon = return value

let result = add(3, 4);

// Rust: function call is an expression
// The return type is the "type" of the expression
let doubled = add(2, 3) * 2;  // nested expression: 5 * 2 = 10

// Closures (anonymous functions) are also expressions:
let multiply = |a: i32, b: i32| -> i32 { a * b };
let product = multiply(3, 4);

// Method call syntax:
let s = String::from("hello");
let upper = s.to_uppercase();  // method call expression

// Chaining method calls (common Rust idiom):
let result = "  hello world  "
    .trim()
    .split_whitespace()
    .map(|w| w.to_uppercase())
    .collect::<Vec<_>>()
    .join(", ");
```

### 3.5 Block Expressions (Rust-Specific Power)

This is one of Rust's most important and distinctive features — blocks are expressions:

```rust
// ── RUST BLOCK EXPRESSIONS ──────────────────────────

// A block { ... } evaluates to the value of its last expression
let x = {
    let a = 5;
    let b = 10;
    a + b          // NO semicolon — this is the block's value
};
// x == 15

// Compare: with semicolon, last statement discards value
let y = {
    let a = 5;
    a + 10;        // semicolon makes this a statement, discards value
};
// y is of type () — unit

// This enables if-else and loop as expressions:
let sign = if x > 0 { 1 } else if x < 0 { -1 } else { 0 };

// loop expression returns value via break:
let result = loop {
    // ... some computation
    break 42;  // the value returned from the loop expression
};

// Block expressions create new scopes (ownership/borrow scope control):
let expensive_clone = {
    let temp = fetch_data();    // temp only lives inside this block
    process(temp)               // result escapes the block
};
// temp is dropped here, expensive_clone is the result
```

### 3.6 Place Expressions vs Value Expressions (Lvalue vs Rvalue)

This concept appears in ALL three languages, though named differently:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    LVALUE vs RVALUE MENTAL MODEL                             │
│                                                                              │
│  Traditional names: lvalue (left of =), rvalue (right of =)                 │
│  Modern names: Place expression vs Value expression                          │
│                                                                              │
│                         Assignment                                           │
│                        ┌──────────┐                                          │
│                        │  x = 42  │                                          │
│                        └──────────┘                                          │
│                         /        \                                           │
│             lvalue (place)        rvalue (value)                             │
│             ─────────────         ─────────────                              │
│             x                     42                                         │
│             arr[i]                arr[i] + 1                                 │
│             *ptr                  f(x)                                       │
│             p.field               p.field + 1                                │
│                                   "hello"                                    │
│                                   42 + 5                                     │
│                                                                              │
│  An lvalue:                         An rvalue:                               │
│  • Has an address (can take &)      • May have NO address                    │
│  • Can appear left of =             • Cannot appear left of =                │
│  • Persists beyond expression        • Temporary — dies after expression     │
│                                                                              │
│  In C:    called lvalue/rvalue                                               │
│  In Rust: called "place expression" / "value expression"                     │
│  In Go:   called "addressable" / "non-addressable"                           │
└──────────────────────────────────────────────────────────────────────────────┘
```

```c
// ── C: lvalue vs rvalue ───────────────────────────
int x = 5;
int *p = &x;   // &x: taking address of lvalue x — OK

// &5;          // ERROR: 5 is rvalue, has no address
// &(x + 1);   // ERROR: x+1 is rvalue (temporary)
// 5 = x;      // ERROR: 5 is not an lvalue (left must be lvalue)

// Modifiable vs non-modifiable lvalue:
const int cx = 5;
// cx = 10;  // ERROR: cx is non-modifiable lvalue (const)

// Array name is NOT a modifiable lvalue:
int arr[3] = {1, 2, 3};
// arr = something;  // ERROR: array name cannot be assigned
arr[0] = 99;         // OK: arr[0] IS a modifiable lvalue

// Functions are NOT lvalues:
// int (*fp)() = ... OK, but f(x) is rvalue — you can't assign to a call result
```

```rust
// ── Rust: place vs value expressions ─────────────
let mut x = 5;
let r = &x;     // & of place expression x — OK

// &5;           // ERROR (though there's a special case: &5 creates a temp ref)
                 // Actually in Rust: let r = &5 creates a temporary with lifetime
                 // equal to the reference — Rust is more nuanced here

// Rust tracks mutability at the place level:
// *r = 10;     // ERROR: r is &x (immutable reference), not &mut x
let rm = &mut x;
*rm = 10;       // OK: *rm is a place expression via mutable reference
```

---

## 4. Statements — Things That Do Work

### 4.1 The Core Definition

A **statement** is a complete unit of execution that causes the program to DO something. Unlike an expression (which produces a value), a statement causes a **side effect**: it changes state, controls flow, declares names, or interacts with the outside world.

```
┌──────────────────────────────────────────────────────────────────────┐
│  Expression vs Statement — The Fundamental Distinction               │
│                                                                      │
│  EXPRESSION              STATEMENT                                   │
│  ─────────               ─────────                                   │
│  "What is the value?"    "What happens?"                             │
│  Evaluates to a result   Executes an action                          │
│  CAN be nested           Usually CANNOT be nested (except in blocks) │
│  2 + 3 → 5               x = 5; → changes x, result discarded       │
│  sin(x) → float          if (c) { ... } → branch taken or not       │
│  x > 0 → bool            return x; → exits function                 │
└──────────────────────────────────────────────────────────────────────┘
```

### 4.2 Types of Statements

#### 4.2.1 Expression Statements

An expression statement is formed by taking an expression and adding a terminating semicolon. The expression is evaluated for its side effects, and its value is **discarded**.

```c
// ── C: expression statement = expression + semicolon ─────────
// Every expression can become a statement by appending ;

// Common expression statements:
x = 5;           // assignment expression → statement (side effect: x changes)
x++;             // increment expression → statement
x += 3;          // compound assignment → statement
printf("hi\n");  // function call expression → statement (side effect: output)
f(a, b);         // discard return value of f

// Legal but useless — pure expressions as statements (no side effects):
5;               // integer literal as statement — valid C, zero effect
x + 3;           // computation with no side effect — compiler may warn
x > 0;           // comparison result discarded — compiler may warn
```

```go
// ── Go: STRICT about expression statements ────────────────────
// Go ONLY allows specific expressions as statements:
// - calls (function/method calls)
// - receive operations (<-channel)
// - send operations (channel <- value)

// These are NOT valid expression statements in Go:
// x + 3;   ← SYNTAX ERROR (no effect, and Go rejects it)
// x > 0;   ← SYNTAX ERROR

// These ARE valid:
fmt.Println("hello")  // function call (side effects OK)
x++                   // increment (NOTE: x++ is a STATEMENT in Go, not expression!)
x--                   // decrement (same)
ch <- value           // channel send
value = <-ch          // channel receive (assignment statement)
_ = someFunc()        // blank assign — explicitly discards return value
```

```rust
// ── Rust: expression statement = expression + semicolon ──────
// Any expression can be turned into a statement with ;
// The semicolon explicitly says "evaluate this, then DISCARD the result"

let mut x = 5;
x = 10;          // assignment as statement (returns ())
x += 3;          // compound assignment statement
println!("hi");  // macro call statement (macros expand to expressions here)

// CRITICAL Rust rule — the semicolon changes MEANING in blocks:
let y = {
    5 + 3          // NO semicolon → block evaluates to 8
};
// y == 8

let z = {
    5 + 3;         // WITH semicolon → expression statement, value discarded
    // block now has no final expression, so it evaluates to ()
};
// z is of type () — this would be a compile error if z: i32 was expected
```

#### 4.2.2 Control Flow Statements

##### If Statement / Expression

```c
// ── C: if is a STATEMENT ──────────────────────────────────────
// if does NOT produce a value

if (x > 0) {
    printf("positive\n");
} else if (x < 0) {
    printf("negative\n");
} else {
    printf("zero\n");
}

// C supports if without braces (dangerous but legal):
if (x > 0) printf("positive\n");  // single statement — valid but avoid

// Switch statement (multi-way branch):
switch (x) {
    case 1:
        printf("one\n");
        break;         // MUST break — no implicit break! (fallthrough by default)
    case 2:
    case 3:
        printf("two or three\n");  // intentional fallthrough (cases 2 and 3)
        break;
    default:
        printf("other\n");
}
// Note: switch in C only works on INTEGER types (and enum which is int)

// Ternary as expression alternative to if-else for simple values:
int abs_x = (x >= 0) ? x : -x;
```

```go
// ── Go: if is a STATEMENT (with optional init statement) ──────
if x > 0 {
    fmt.Println("positive")
} else if x < 0 {
    fmt.Println("negative")
} else {
    fmt.Println("zero")
}
// Go REQUIRES braces — no braceless if allowed

// Go's powerful init statement in if:
if err := riskyOp(); err != nil {
    // err is SCOPED to this if block (and its else)
    return fmt.Errorf("riskyOp failed: %w", err)
}
// err is NOT accessible here — scoped to the if block only

// Go's switch (no fallthrough by default — OPPOSITE of C):
switch x {
case 1:
    fmt.Println("one")
    // implicit break — no fallthrough
case 2, 3:
    fmt.Println("two or three")  // multiple values per case
case 4:
    fmt.Println("four")
    fallthrough  // explicit fallthrough needed
case 5:
    fmt.Println("four or five")
default:
    fmt.Println("other")
}

// Go type switch:
var i interface{} = "hello"
switch v := i.(type) {
case int:    fmt.Printf("int: %d\n", v)
case string: fmt.Printf("string: %s\n", v)
default:     fmt.Printf("other: %T\n", v)
}
```

```rust
// ── Rust: if is an EXPRESSION ────────────────────────────────
// if-else evaluates to a value — BOTH branches must have same type

let sign = if x > 0 { 1 } else if x < 0 { -1 } else { 0 };
// sign: i32

// When used as statement (value discarded):
if x > 0 {
    println!("positive");
} else {
    println!("non-positive");
}

// Pattern matching with match (Rust's switch):
match x {
    0         => println!("zero"),
    1..=9     => println!("single digit"),  // range pattern
    10 | 20   => println!("ten or twenty"), // or-pattern
    n if n < 0 => println!("negative: {n}"), // guard condition
    _         => println!("other"),           // wildcard (must-have for exhaustiveness)
}

// match is an EXPRESSION:
let description = match x {
    0         => "zero",
    1..=9     => "single digit",
    _         => "other",
};

// Destructuring in match:
let pair = (1, -1);
match pair {
    (x, y) if x == y    => println!("equal"),
    (x, y) if x + y == 0 => println!("sum to zero"),
    (x, _)              => println!("first is {x}"),
}
```

##### Loop Statements / Expressions

```c
// ── C: Three loop constructs ────────────────────────────────

// while: condition checked before each iteration
int i = 0;
while (i < 10) {
    printf("%d ", i);
    i++;
}

// do-while: condition checked AFTER each iteration (runs at least once)
int j = 10;
do {
    printf("%d ", j);
    j++;
} while (j < 10);  // condition false, but body ran once

// for: init; condition; post — all parts are optional
for (int k = 0; k < 10; k++) {
    if (k == 5) continue;  // skip iteration
    if (k == 8) break;     // exit loop
    printf("%d ", k);
}

// Infinite loop:
// while (1) { ... }
// for (;;) { ... }

// goto (discouraged but legal):
    int n = 0;
loop:
    if (n < 10) {
        n++;
        goto loop;
    }
```

```go
// ── Go: ONE loop keyword (for) for all patterns ──────────────

// C-style for:
for i := 0; i < 10; i++ {
    fmt.Print(i, " ")
}

// while-style (condition only):
j := 0
for j < 10 {
    j++
}

// Infinite loop:
for {
    // break to exit
    break
}

// Range-based for (iterates over slice, map, string, channel):
nums := []int{1, 2, 3, 4, 5}
for i, v := range nums {
    fmt.Printf("index=%d value=%d\n", i, v)
}
for _, v := range nums {  // discard index
    fmt.Println(v)
}

// Range over map (random order):
m := map[string]int{"a": 1, "b": 2}
for key, val := range m {
    fmt.Printf("%s=%d\n", key, val)
}

// Range over string (iterates runes, not bytes):
for i, r := range "hello" {
    fmt.Printf("byte_index=%d rune=%c\n", i, r)
}

// Labeled break/continue (for nested loops):
outer:
for i := 0; i < 5; i++ {
    for j := 0; j < 5; j++ {
        if i+j == 6 { break outer }  // exits OUTER loop
    }
}
```

```rust
// ── Rust: loop, while, for — all are EXPRESSIONS ─────────────

// loop: infinite loop, returns value via break
let result = loop {
    let val = compute();
    if val > 100 {
        break val;  // return val from loop
    }
};

// while: condition-based loop (returns ())
let mut i = 0;
while i < 10 {
    i += 1;
}

// while let: loop while pattern matches (very idiomatic)
let mut stack = vec![1, 2, 3];
while let Some(top) = stack.pop() {
    println!("{top}");  // prints 3, 2, 1
}

// for: range-based (consumes iterators)
for i in 0..10 {           // 0..10 is a Range, exclusive: 0,1,2,...,9
    print!("{i} ");
}
for i in 0..=10 {          // inclusive range: 0,1,2,...,10
    print!("{i} ");
}
for (i, v) in nums.iter().enumerate() {
    println!("index={i} value={v}");
}

// Labeled loops:
'outer: for i in 0..5 {
    'inner: for j in 0..5 {
        if i + j == 6 { break 'outer; }
    }
}

// Loop returning a value:
let mut counter = 0;
let x = loop {
    counter += 1;
    if counter == 10 {
        break counter * 2;  // x = 20
    }
};
```

#### 4.2.3 Return Statement

```c
// ── C ───────────────────────────────────────────────
int square(int n) {
    return n * n;  // evaluates n*n, returns the value, exits function
}

void no_return(int n) {
    if (n == 0) return;  // early return with no value (void function)
    printf("%d\n", n);
    // implicit return at end of void function
}

// return with expression: evaluates the expression, then exits
// return without expression: only valid in void functions
```

```go
// ── Go ──────────────────────────────────────────────
func square(n int) int {
    return n * n
}

// Multiple return:
func minmax(a, b int) (int, int) {
    if a < b {
        return a, b
    }
    return b, a
}

// Named return values with defer-friendly pattern:
func openFile(path string) (f *os.File, err error) {
    f, err = os.Open(path)
    if err != nil {
        return  // returns f (nil) and err (the error)
    }
    return  // returns f and err (nil)
}
```

```rust
// ── Rust ────────────────────────────────────────────
fn square(n: i32) -> i32 {
    n * n   // implicit return: last expression without semicolon
}

fn square_explicit(n: i32) -> i32 {
    return n * n;  // explicit return — useful for early exits
}

fn check(n: i32) -> Option<i32> {
    if n < 0 {
        return None;   // early return
    }
    Some(n * n)        // implicit return of Some variant
}

// Diverging functions (never return) — return type is !
fn panic_example() -> ! {
    panic!("This function never returns normally");
}
```

#### 4.2.4 Let/Variable Declaration Statements

These are covered in detail in Section 5, but note their placement as statements:

```c
// C: declaration can appear at start of block (C89) or anywhere (C99+)
{
    int x = 5;      // declaration statement
    x = x + 1;     // assignment expression statement
    int y = x * 2; // C99+ allows mixed declarations and statements
}
```

```go
// Go: short variable declaration is a statement
x := 5           // short decl — only valid inside functions
var y int = 10   // full declaration — valid anywhere
y = y + 1        // assignment statement
```

```rust
// Rust: let is ALWAYS a statement (not an expression)
let x = 5;         // let statement
let mut y = 10;    // mutable let statement
y = y + 1;         // assignment statement (returns ())
// let z = (let w = 5);  ← ERROR: let is not an expression
```

---

## 5. Declarations — Things That Introduce Names

### 5.1 The Core Definition

A **declaration** introduces a **name** (identifier) into a **scope** and associates it with a **thing** (type, variable, function, etc.). After a declaration, the name can be used to refer to that thing.

**Declaration vs Definition:**
- **Declaration:** "I promise something named X exists and has this type"
- **Definition:** "Here is the actual memory/code for X"

```
┌───────────────────────────────────────────────────────────────────────┐
│              DECLARATION vs DEFINITION                                │
│                                                                       │
│  C:                                                                   │
│    extern int x;         → Declaration only (no storage allocated)   │
│    int x = 5;            → Declaration + Definition (storage!)       │
│    void foo(int);        → Declaration only (function prototype)     │
│    void foo(int n) {...} → Declaration + Definition (function body)  │
│                                                                       │
│  Go:  Declaration always = Definition (no forward declarations)      │
│       var x int = 5     → Both declaration and definition            │
│       func foo(n int) {} → Both declaration and definition           │
│                                                                       │
│  Rust: No forward declarations (uses module system instead)          │
│        let x: i32 = 5;  → Declaration + binding (always together)   │
│        fn foo(n: i32) {} → Declaration + definition (always together)│
└───────────────────────────────────────────────────────────────────────┘
```

### 5.2 Variable Declarations

#### C Variable Declarations

```c
// ─────────────────────────────────────────────────────────────
//  C VARIABLE DECLARATIONS — Complete Reference
// ─────────────────────────────────────────────────────────────

// Basic form: storage-class type name [= initializer];

// TYPE SPECIFIERS:
int x;               // uninitialized — indeterminate value (UB to read)
int y = 5;           // initialized
int a, b, c;         // multiple declarators (same type)
int d = 1, e = 2;    // multiple with initialization

// STORAGE CLASSES control where/how the variable is stored:
auto int local = 5;       // auto: stack-allocated (DEFAULT for local vars, rarely written)
static int persistent = 0; // static: persists across function calls, in .data/.bss
extern int global;         // extern: declaration of variable defined elsewhere
register int fast = 0;     // register: HINT to compiler to use register (modern compilers ignore)

// TYPE QUALIFIERS:
const int LIMIT = 100;     // immutable (read-only after initialization)
volatile int hw_reg;       // prevents compiler from caching — used for hardware registers
const volatile int *cvp;   // read-only, but may change (memory-mapped hardware register)
restrict int *rp;          // pointer uniquely owns its target — enables optimization (C99)

// COMPLEX DECLARATIONS (read right-to-left from identifier):
int *p;              // p: pointer to int
int **pp;            // pp: pointer to pointer to int
int arr[5];          // arr: array of 5 ints
int (*fp)(int, int); // fp: pointer to function taking (int,int) returning int
int *arr2[5];        // arr2: array of 5 pointers to int
int (*arr3)[5];      // arr3: pointer to array of 5 ints

// cdecl rule: "go right when you can, go left when you must"
// const int *p;     → p is pointer to const int (can change p, not *p)
// int * const p;    → p is const pointer to int (can change *p, not p)
// const int * const p; → p is const pointer to const int (neither changeable)

// Multiple variables with same type specifier (each declarator is separate):
int x2 = 1, *ptr = NULL, arr4[3]; // x2:int, ptr:int*, arr4:int[3]
```

**C Declaration Anatomy:**

```
         storage   type       declarator      initializer
         class     specifier
         ┌──┐  ┌────────┐  ┌────────────┐  ┌───────────┐
  static const  unsigned  long  int   (*fp)(int, int) = default_fn;
  └──────────────────────────────────────────────────────────────────┘
  
  Reading fp:  "fp is a pointer to function taking (int,int) returning
                long unsigned int, statically stored, const qualified"
```

#### Go Variable Declarations

```go
// ─────────────────────────────────────────────────────────────
//  GO VARIABLE DECLARATIONS — Complete Reference
// ─────────────────────────────────────────────────────────────

package main

// Package-level declarations:
var globalX int = 10    // full declaration
var globalY = 20        // type inferred from initializer
var globalZ int         // zero-initialized: 0

// Multiple declarations:
var (
    name  string = "Alice"
    age   int    = 30
    score float64             // zero value: 0.0
)

func main() {
    // Short variable declaration (ONLY inside functions):
    x := 5               // infers int
    y := 3.14            // infers float64
    s := "hello"         // infers string
    b := true            // infers bool

    // Short decl with multiple variables:
    a, b2 := 1, 2          // a=1, b2=2
    min, max := minmax(a, b2)

    // Short decl REQUIRES at least one NEW variable on left:
    x, z := 10, 20     // x is REASSIGNED, z is new — this is OK
    // x, y := 10, 20  // ERROR if both x and y already exist

    // Full declaration inside function:
    var i int           // zero value: 0
    var pi float64 = 3.14159

    // Type conversion (Go has NO implicit type conversion):
    var f float64 = float64(i)   // explicit cast required
    var u uint = uint(f)

    // Zero values (every declared variable gets its type's zero value):
    var vi   int        // 0
    var vf   float64    // 0.0
    var vb   bool       // false
    var vs   string     // ""
    var vp   *int       // nil
    var vsl  []int      // nil
    var vm   map[string]int // nil
    var vfn  func()     // nil

    _ = x; _ = y; _ = s; _ = b; _ = a; _ = min; _ = max; _ = z
    _ = i; _ = pi; _ = f; _ = u
    _ = vi; _ = vf; _ = vb; _ = vs; _ = vp; _ = vsl; _ = vm; _ = vfn
}

func minmax(a, b int) (int, int) {
    if a < b { return a, b }
    return b, a
}
```

#### Rust Variable Declarations

```rust
// ─────────────────────────────────────────────────────────────
//  RUST VARIABLE DECLARATIONS — Complete Reference
// ─────────────────────────────────────────────────────────────

fn main() {
    // let [mut] pattern [: type] [= expression];

    // Immutable binding (DEFAULT in Rust):
    let x = 5;          // immutable, type inferred: i32
    let y: i32 = 5;     // explicit type annotation
    // x = 6;           // COMPILE ERROR: x is immutable

    // Mutable binding:
    let mut z = 5;
    z = 6;              // OK

    // Shadowing — rebind the SAME name (different type allowed!):
    let x = "hello";   // shadows the previous x: i32, now x: &str
    let x = x.len();   // shadows again: now x: usize
    // This is NOT mutation — these are three different bindings

    // Type annotation forms:
    let a: i32 = 42;
    let b: &str = "hello";
    let c: Vec<i32> = Vec::new();
    let d: Option<i32> = Some(5);
    let e: Result<i32, String> = Ok(42);

    // Destructuring in let:
    let (p, q) = (1, 2);       // tuple destructuring
    let [first, rest @ ..] = [1, 2, 3, 4]; // array destructuring (nightly: @ binding)
    let Point { x: px, y: py } = Point { x: 1, y: 2 }; // struct destructuring

    // let-else: destructure or early return (Rust 1.65+):
    let Some(value) = maybe_value else {
        return;  // or panic!, continue, break
    };
    // value is now in scope and guaranteed to be the inner value

    // Uninitialized variables:
    let maybe_init: i32;
    // println!("{maybe_init}");  // COMPILE ERROR: possibly uninitialized
    maybe_init = 5;              // now initialized
    println!("{maybe_init}");    // OK

    // const: compile-time constant (different from immutable let)
    const MAX_SIZE: usize = 1024;         // ALL_CAPS convention
    const PI: f64 = std::f64::consts::PI; // must have explicit type, must be compile-time

    // static: program-lifetime variable (like C's static, lives for entire program)
    static COUNTER: std::sync::atomic::AtomicI32 = std::sync::atomic::AtomicI32::new(0);

    println!("{x} {z} {a} {b} {p} {q} {px} {py} {value} {MAX_SIZE} {PI}");
    _ = c; _ = d; _ = e;
}

struct Point { x: i32, y: i32 }
let maybe_value: Option<i32> = Some(42);
```

### 5.3 Function Declarations / Definitions

#### C Functions

```c
// ─────────────────────────────────────────────────────────────
//  C FUNCTION DECLARATIONS AND DEFINITIONS
// ─────────────────────────────────────────────────────────────

// Forward declaration (prototype) — in header files:
int add(int a, int b);           // declares name and signature
int add(int, int);               // parameter names optional in prototype
void process(void);              // void parameter = no parameters (NOT int(*)(void) = void(*)(void))
int variadic(int count, ...);    // variadic function prototype

// Definition — provides the body:
int add(int a, int b) {
    return a + b;
}

// Return type qualifiers:
static int private_helper(int n) { return n; }  // static: file scope only
extern int public_function(int n) { return n; } // extern: visible to other translation units
inline int fast_add(int a, int b) { return a + b; } // hint: inline expansion

// const in parameters:
void read_only(const int *arr, int len) {
    // *arr = 5;  // ERROR: cannot modify const-pointed data
}

// Function pointers:
int (*operation)(int, int) = add;          // function pointer variable
typedef int (*BinaryOp)(int, int);        // typedef for readability
BinaryOp op = add;
int result = op(3, 4);                    // call through pointer

// Array parameters decay to pointers:
void takes_array(int arr[], int len) { }    // same as int *arr
void takes_sized_array(int (*arr)[10]) { }  // pointer to array of 10 — keeps size info

// Variadic function:
#include <stdarg.h>
int sum_all(int count, ...) {
    va_list args;
    va_start(args, count);
    int total = 0;
    for (int i = 0; i < count; i++) {
        total += va_arg(args, int);
    }
    va_end(args);
    return total;
}
```

#### Go Functions

```go
// ─────────────────────────────────────────────────────────────
//  GO FUNCTION DECLARATIONS AND DEFINITIONS
// ─────────────────────────────────────────────────────────────

// Basic form: func name(params) return_types { body }

// Simple function:
func add(a, b int) int {
    return a + b
}

// Multiple parameters of same type can share type:
func multiply(a, b, c int) int {
    return a * b * c
}

// Multiple return values:
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
}

// Named return values (naked return):
func stats(nums []float64) (mean, variance float64) {
    sum := 0.0
    for _, n := range nums { sum += n }
    mean = sum / float64(len(nums))
    for _, n := range nums { variance += (n - mean) * (n - mean) }
    variance /= float64(len(nums))
    return  // returns mean and variance
}

// Variadic functions:
func joinStrings(sep string, parts ...string) string {
    return strings.Join(parts, sep)
}
// Call: joinStrings(", ", "a", "b", "c")
// Or:   words := []string{"a","b","c"}; joinStrings(", ", words...)

// First-class functions and closures:
func adder(base int) func(int) int {
    return func(x int) int {
        return base + x  // captures 'base' from outer scope
    }
}
add5 := adder(5)
fmt.Println(add5(3))  // 8

// Methods (function with receiver):
type Rectangle struct { Width, Height float64 }

func (r Rectangle) Area() float64 {       // value receiver
    return r.Width * r.Height
}
func (r *Rectangle) Scale(factor float64) { // pointer receiver (can modify r)
    r.Width  *= factor
    r.Height *= factor
}

// Defer: schedules function to run at end of enclosing function
func withDefer() {
    defer fmt.Println("cleanup")  // runs last
    fmt.Println("work")           // runs first
    // defers stack: LIFO order if multiple defers
}

// init function: special — runs before main, automatically
func init() {
    // package-level initialization
}
```

#### Rust Functions

```rust
// ─────────────────────────────────────────────────────────────
//  RUST FUNCTION DECLARATIONS AND DEFINITIONS
// ─────────────────────────────────────────────────────────────

// Basic form: fn name(params: types) -> return_type { body }

// Simple function:
fn add(a: i32, b: i32) -> i32 {
    a + b  // implicit return (no semicolon)
}

// Multiple parameters and explicit return:
fn clamp(value: f64, min: f64, max: f64) -> f64 {
    if value < min { return min; }
    if value > max { return max; }
    value
}

// Generics:
fn largest<T: PartialOrd>(a: T, b: T) -> T {
    if a > b { a } else { b }
}

// References and lifetimes:
fn longest<'a>(s1: &'a str, s2: &'a str) -> &'a str {
    // 'a is a lifetime parameter: output lives as long as the shorter input
    if s1.len() >= s2.len() { s1 } else { s2 }
}

// Multiple return via tuple:
fn min_max(nums: &[i32]) -> Option<(i32, i32)> {
    if nums.is_empty() { return None; }
    let (mut min, mut max) = (nums[0], nums[0]);
    for &n in &nums[1..] {
        if n < min { min = n; }
        if n > max { max = n; }
    }
    Some((min, max))
}

// Closures (anonymous functions — capture environment):
fn apply<F: Fn(i32) -> i32>(f: F, x: i32) -> i32 {
    f(x)
}
let double = |x: i32| x * 2;
let result = apply(double, 5);  // = 10

// Closures capture by reference, mutable reference, or value:
let mut count = 0;
let mut increment = || { count += 1; count };  // captures &mut count
let result1 = increment();  // 1
let result2 = increment();  // 2

// Move closure (takes ownership):
let data = vec![1, 2, 3];
let print_data = move || println!("{data:?}");  // data MOVED into closure
// println!("{data:?}");  // ERROR: data was moved

// Method impl:
struct Circle { radius: f64 }
impl Circle {
    // Associated function (no self — like static method):
    fn new(radius: f64) -> Self {
        Circle { radius }
    }
    // Method with immutable self:
    fn area(&self) -> f64 {
        std::f64::consts::PI * self.radius * self.radius
    }
    // Method with mutable self:
    fn scale(&mut self, factor: f64) {
        self.radius *= factor;
    }
    // Method that consumes self (takes ownership):
    fn into_diameter(self) -> f64 {
        self.radius * 2.0
    }
}

// Diverging function:
fn never_returns() -> ! {
    loop {}
}

// const fn: evaluated at compile time
const fn factorial(n: u64) -> u64 {
    match n {
        0 | 1 => 1,
        n     => n * factorial(n - 1),
    }
}
const FACT_10: u64 = factorial(10);  // computed at compile time
```

### 5.4 Type Declarations

```c
// ── C TYPE DECLARATIONS ──────────────────────────────────────

// struct — creates a new composite type
struct Point {
    int x;
    int y;
};  // NOTE: semicolon required after struct definition in C

// Usage requires 'struct' keyword without typedef:
struct Point p1;         // C: must say 'struct Point'

// typedef — creates an alias for a type (any type)
typedef struct Point Point2D;  // alias: 'Point2D' = 'struct Point'
typedef int Integer;            // alias: 'Integer' = 'int'
typedef void (*Callback)(int);  // alias for function pointer type

// Common idiom: typedef + struct in one:
typedef struct {
    int x;
    int y;
} Point;                         // now just 'Point' works (no 'struct' needed)

// enum:
enum Direction { NORTH, SOUTH, EAST, WEST };
// Values: NORTH=0, SOUTH=1, EAST=2, WEST=3 (auto-increment from 0)
enum Status { OK = 200, NOT_FOUND = 404, ERROR = 500 };  // custom values

// union — all members share the SAME memory:
union Data {
    int   i;
    float f;
    char  str[4];
};
// sizeof(union Data) = max(sizeof members)
// Writing one member and reading another = type punning (often UB)
```

```go
// ── GO TYPE DECLARATIONS ──────────────────────────────────────

// Named type (distinct from underlying type):
type Celsius    float64
type Fahrenheit float64

func CtoF(c Celsius) Fahrenheit {
    return Fahrenheit(c*9/5 + 32)
}
// Celsius and Fahrenheit are DIFFERENT types even though both are float64
// Cannot mix them without explicit conversion — prevents unit bugs!

// Type alias (Go 1.9+): same type, different name
type MyInt = int   // alias: MyInt IS int (NOT distinct)

// Struct type:
type Person struct {
    Name    string
    Age     int
    address string  // unexported (lowercase) — package-private
}

// Embedded struct (composition, not inheritance):
type Employee struct {
    Person            // embedded (promoted fields/methods)
    CompanyID string
}
emp := Employee{Person: Person{Name: "Alice", Age: 30}, CompanyID: "E123"}
fmt.Println(emp.Name)  // promoted from Person

// Interface type:
type Writer interface {
    Write([]byte) (int, error)
}
type ReadWriter interface {
    Reader           // embedded interface
    Writer           // embedded interface (interface composition)
}

// Function type:
type Transform func(int) int
var double Transform = func(x int) int { return x * 2 }
```

```rust
// ── RUST TYPE DECLARATIONS ──────────────────────────────────────

// Struct (product type):
#[derive(Debug, Clone, PartialEq)]  // auto-implement common traits
struct Point {
    x: f64,
    y: f64,
}

// Tuple struct:
struct Color(u8, u8, u8);

// Unit struct:
struct Marker;  // zero-size, used for type-level programming

// Enum (sum type — more powerful than C's enum):
#[derive(Debug)]
enum Shape {
    Circle(f64),                    // variant with data
    Rectangle(f64, f64),            // variant with two fields
    Triangle { base: f64, height: f64 }, // variant with named fields
    Point,                          // unit variant (like C enum)
}

// Result and Option are enums:
// enum Option<T> { Some(T), None }
// enum Result<T, E> { Ok(T), Err(E) }

// Type alias:
type Meters = f64;
type Kilograms = f64;
type NodeId = u64;

// Trait (interface equivalent):
trait Area {
    fn area(&self) -> f64;
    fn perimeter(&self) -> f64;  // can have default implementations
}

impl Area for Point {
    fn area(&self) -> f64 { 0.0 }
    fn perimeter(&self) -> f64 { 0.0 }
}

impl Area for Shape {
    fn area(&self) -> f64 {
        match self {
            Shape::Circle(r)             => std::f64::consts::PI * r * r,
            Shape::Rectangle(w, h)       => w * h,
            Shape::Triangle { base, height } => 0.5 * base * height,
            Shape::Point                 => 0.0,
        }
    }
    fn perimeter(&self) -> f64 { todo!() }
}

// newtype pattern (single-field tuple struct for type safety):
struct Meters2(f64);
struct Seconds(f64);
// Meters2 and Seconds are incompatible types despite both wrapping f64
```

---

## 6. The Expression-Statement Continuum

### 6.1 How Languages Unify Them Differently

```
┌─────────────────────────────────────────────────────────────────────────────┐
│              THE EXPRESSION-STATEMENT SPECTRUM                              │
│                                                                             │
│  C:                                                                         │
│  ┌─────────────┬───────────────────────────────────────────────────────┐   │
│  │ Expression  │ Statement                                             │   │
│  │             │                                                       │   │
│  │  2 + 3      │   if() {}   for() {}   while() {}   return;          │   │
│  │  x = 5  ◄──┼── assignment is ALSO an expression (value = 5)       │   │
│  │  x++    ◄──┼── prefix/postfix are expressions (value = old/new x)  │   │
│  └─────────────┴───────────────────────────────────────────────────────┘   │
│                                                                             │
│  Go:                                                                        │
│  ┌─────────────┬───────────────────────────────────────────────────────┐   │
│  │ Expression  │ Statement                                             │   │
│  │             │                                                       │   │
│  │  2 + 3      │   if {}    for {}    switch {}    return              │   │
│  │  f(x)       │   x = 5   (assignment is STATEMENT, not expression)  │   │
│  │             │   x++     (increment is STATEMENT, not expression)   │   │
│  └─────────────┴───────────────────────────────────────────────────────┘   │
│                                                                             │
│  Rust:                                                                      │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                  EVERYTHING IS POTENTIALLY AN EXPRESSION            │  │
│  │                                                                      │  │
│  │  2 + 3              → expression (value: 5)                         │  │
│  │  { let x=5; x }     → block expression (value: 5)                   │  │
│  │  if c { a } else { b } → if expression (value: a or b)             │  │
│  │  loop { break 42; } → loop expression (value: 42)                  │  │
│  │  match x { ... }    → match expression (value: matched arm)        │  │
│  │  x = 5;             → assignment expression + semicolon = statement │  │
│  │  let x = 5;         → statement (let is not an expression)         │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.2 The Semicolon Question: What Does It Mean?

```
┌────────────────────────────────────────────────────────────────┐
│  WHAT DOES A SEMICOLON ACTUALLY DO?                            │
│                                                                │
│  Language   Semicolon Meaning                                  │
│  ─────────  ──────────────────────────────────────────────── │
│  C          Terminates a statement (required)                  │
│             Expression → Statement: "do this and discard"      │
│                                                                │
│  Go         Statement terminator (auto-inserted by lexer)      │
│             Semicolons at line ends are usually implied        │
│             No semicolon = expression continues to next line   │
│                                                                │
│  Rust       TWO meanings:                                      │
│             1. At end of block's last expr → makes it unit ()  │
│             2. Within block → expression statement             │
│             The presence/absence of ; changes block type!      │
│                                                                │
│  Rust example:                                                 │
│    fn f() -> i32 { 5 + 3 }   // returns 8                     │
│    fn g() -> ()  { 5 + 3; }  // returns ()  (unit)            │
│    fn h() -> i32 { 5 + 3; 0 } // returns 0 (semicolon discards│
│                                //  5+3, then 0 is returned)    │
└────────────────────────────────────────────────────────────────┘
```

### 6.3 Compiler View: The Operator Precedence Table

```
C / Go / Rust Operator Precedence (High to Low):

Level  Operators                     Associativity
─────  ──────────────────────────    ─────────────
  15   ()  []  .  ->  (postfix)++   Left to Right
  14   (prefix)++ --  +  -  !  ~    Right to Left
       *  &  sizeof  (cast)
  13   *  /  %                       Left to Right
  12   +  -                          Left to Right
  11   <<  >>                        Left to Right
  10   <  <=  >  >=                  Left to Right
   9   ==  !=                        Left to Right
   8   &  (bitwise AND)              Left to Right
   7   ^  (bitwise XOR)              Left to Right
   6   |  (bitwise OR)               Left to Right
   5   &&  (logical AND)             Left to Right
   4   ||  (logical OR)              Left to Right
   3   ?:  (ternary — C only)        Right to Left
   2   =  +=  -=  *=  /=  etc.      Right to Left
   1   ,  (comma — C only)           Left to Right

NOTE: When in doubt, use parentheses! 
      Explicit grouping communicates intent and prevents bugs.
```

---

## 7. Scope, Lifetime, and the Compilation Model

### 7.1 What Is Scope?

**Scope** is the region of source code where a declared name is **visible** and **accessible**. It is a compile-time concept — the compiler uses scope to resolve names.

```
SCOPE VISUALIZATION (C-style block scoping)

int x = 10;   ← x declared at file scope (global)
              │
              │  void func() {
              │      int y = 20;  ← y declared at function scope
              │                  │
              │      if (true) {  │
              │          int z = 30; ← z at block scope
              │          // x, y, z all visible here
              │      }            │
              │      // x, y visible; z is OUT OF SCOPE
              │  }
              │
// Only x visible here (file scope)

Scopes nest: inner scope can see outer scope.
             outer scope CANNOT see inner scope.
             If inner declares same name as outer → SHADOWING.
```

### 7.2 Scope Rules by Language

```c
// ── C SCOPE RULES ──────────────────────────────────────────────

// 4 scope categories in C:
// 1. File scope (global): outside all functions
// 2. Function scope: applies only to labels (goto)
// 3. Block scope: inside { }
// 4. Function prototype scope: parameter names in prototypes

int global = 10;                    // file scope

static int file_private = 20;       // file scope, but NOT visible outside this .c file

void example(int param) {           // param: function scope
    int local = 30;                 // block scope
    {
        int inner = 40;             // inner block scope
        int local = 50;             // SHADOWS outer local (compilers warn)
        printf("%d\n", local);      // prints 50 (inner shadows outer)
    }
    // inner is out of scope
    printf("%d\n", local);          // prints 30 (outer local again)
}

// Forward declaration to use before definition:
void helper(void);   // prototype — declares 'helper' in file scope
void main_func(void) { helper(); }
void helper(void) { printf("helper\n"); }
```

```go
// ── GO SCOPE RULES ──────────────────────────────────────────────

// Go scope levels:
// 1. Universe scope: built-in names (int, string, make, len, nil, etc.)
// 2. Package scope: declared at package level (outside functions)
// 3. File scope: import declarations
// 4. Block scope: inside { }

package main

var PackageVar = 10  // package scope: accessible from all files in package

// Exported (capitalized) vs unexported (lowercase):
var Exported = "visible outside package"   // accessible via package.Exported
var unexported = "package-private"         // only accessible within this package

func scopeExample() {
    x := 1               // block scope (function body is a block)
    if x > 0 {
        y := 2           // inner block scope
        x = y            // outer x visible here
    }
    // y not accessible here

    // Short variable declaration creates new scope:
    for i := 0; i < 3; i++ {
        // i is scoped to the for loop
        fmt.Println(i)
    }
    // i not accessible here

    // Shadowing (Go allows it but go vet warns):
    x2 := "shadow"
    {
        x2 := 42      // shadows outer x2 — different type!
        _ = x2
    }
    _ = x2  // outer x2 ("shadow") still accessible
}
```

```rust
// ── RUST SCOPE AND OWNERSHIP ────────────────────────────────────

// Rust scopes are tied to OWNERSHIP and LIFETIMES

fn scope_ownership() {
    let s1 = String::from("hello");  // s1 owns the String
    {
        let s2 = s1;  // s1 MOVED to s2 — s1 is no longer valid
        // println!("{s1}");  // COMPILE ERROR: s1 moved
        println!("{s2}");    // OK
    }  // s2 dropped here — String freed
    // s2 not accessible

    let s3 = String::from("world");
    let len = {
        let r = &s3;       // borrow s3 (shared reference)
        r.len()            // block evaluates to length
    };  // r dropped here, but s3 still valid (borrow ended)
    println!("{s3} {len}");  // OK

    // Shadowing in Rust is idiomatic and has clear semantics:
    let x = 5;
    let x = x + 1;       // new binding, shadows old
    let x = x.to_string(); // new binding, different type
    println!("{x}");     // "6"
}
```

### 7.3 Lifetime: Beyond Scope

Rust formalizes a concept that C/Go handle informally: **lifetime** — how long a reference is valid.

```
LIFETIME VISUALIZATION IN RUST

fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {
                ──          ──            ──
         Lifetime  Lifetime       Lifetime
         annotation annotation     annotation

The 'a says: "the returned reference lives at least as long as
             the SHORTER of the input references"

                 Timeline:
                 ─────────
 'a starts: ┬──────────────────────────────────────────────────
            │  x: &str ──────────────────────────────────────┤ x ends
            │  y: &str ─────────────────────────┤ y ends
            │                                    ▲
            │  return: &'a str ─────────────────┘ must end no later than y
            └──────────────────────────────────────────────────

 If return could outlive either input → compiler REJECTS it → no dangling!
```

---

## 8. Hardware Reality: What the CPU Actually Sees

### 8.1 How Expressions Become Machine Code

```
SOURCE → TOKENS → AST → IR → OPTIMIZED IR → MACHINE CODE

Source:  int result = (a + b) * (c - d);

AST:
         MUL
        /   \
      ADD   SUB
      / \   / \
     a   b c   d

x86-64 Assembly (unoptimized):
    ; Assume a=rdi, b=rsi, c=rdx, d=rcx
    mov  eax, edi        ; eax = a
    add  eax, esi        ; eax = a + b
    mov  ecx2, edx       ; ecx2 = c
    sub  ecx2, ecx       ; ecx2 = c - d
    imul eax, ecx2       ; eax = (a+b) * (c-d)
    mov  [result], eax   ; store result

With optimization (-O2): compiler may use LEA, fuse operations,
reorder for pipeline efficiency, use SIMD if in a loop, etc.
```

### 8.2 Variable Declaration → Memory Allocation

```
DECLARATION TO MEMORY MAPPING

Stack Frame Layout (x86-64, simplified):

  High address
  ┌────────────────────────────────────────┐
  │  Caller's stack frame                  │
  │  ──────────────────────────────────    │
  │  Return address                        │ ← caller pushed this
  │  Saved rbp                             │ ← callee saves old rbp
  │  ──────────────────────────────────    │ ← rbp points here
  │  int a; [rbp - 4]                      │ ← 4 bytes (int)
  │  long b; [rbp - 12]                    │ ← 8 bytes (long)
  │  char c; [rbp - 13]                    │ ← 1 byte (char)
  │  [3 bytes padding]  [rbp - 16]         │ ← alignment to 4-byte boundary
  │  int arr[3] [rbp - 28]                 │ ← 12 bytes (3 ints)
  │  [4 bytes padding]                     │ ← align rsp to 16 bytes (ABI req)
  └────────────────────────────────────────┘
  Low address  ← rsp points here

KEY INSIGHT: 'int x = 5;' does TWO things:
  1. Reserves [rbp - 4] in the stack frame (compiler does this at function entry)
  2. Generates: MOV DWORD PTR [rbp-4], 5  (at the declaration site)
```

### 8.3 Cache Behavior of Literals vs Variables

```
┌─────────────────────────────────────────────────────────────────────────┐
│  CACHE BEHAVIOR — Why Storage Class Matters                             │
│                                                                         │
│  Small integer literal (e.g., 42 in mov eax, 42):                      │
│    → IMMEDIATE operand — embedded IN the instruction stream             │
│    → NO cache miss — it's part of the instruction itself               │
│    → Fetched with the instruction from L1 instruction cache             │
│                                                                         │
│  Stack variable (int x = 42):                                           │
│    → In L1 data cache (hot) for duration of function                   │
│    → rsp is in L1 almost always — very fast                            │
│    → If function is called many times: stack frame reused               │
│                                                                         │
│  Global/static variable:                                                │
│    → Fixed address in .data or .bss                                    │
│    → First access: potential L2/L3 miss (cold)                         │
│    → Subsequent: in cache (hot) — but shared across all calls           │
│    → Risk: false sharing in multithreaded code                         │
│                                                                         │
│  String literal (const char *s = "hello"):                             │
│    → Lives in .rodata section                                          │
│    → Shared between all uses of the same string (compiler merges)      │
│    → Read-only → OS can map as read-only page → write = segfault       │
│                                                                         │
│  Cache hierarchy (typical modern CPU):                                  │
│  L1 instruction cache: 32 KB  — ~4 cycles                              │
│  L1 data cache:        32 KB  — ~4 cycles                              │
│  L2 cache:             256 KB — ~12 cycles                             │
│  L3 cache:             8+ MB  — ~40 cycles                             │
│  Main memory (DRAM):   8+ GB  — ~200 cycles                            │
└─────────────────────────────────────────────────────────────────────────┘
```

### 8.4 Short-Circuit Evaluation in Machine Code

```c
// Source:
if (ptr != NULL && *ptr > 0) { process(ptr); }

// Assembly (conceptual):
    cmp  rdi, 0          ; compare ptr to NULL
    je   .skip           ; if ptr == NULL, jump to skip (short-circuit)
    cmp  DWORD [rdi], 0  ; only reached if ptr != NULL: dereference safe
    jle  .skip           ; if *ptr <= 0, skip
    call process         ; safe to call
.skip:
    ; continue...

; The jump instruction IS the short-circuit:
; If ptr==NULL, we NEVER execute 'cmp DWORD [rdi], 0'
; This is why short-circuit prevents null pointer dereferences.
```

---

## 9. Language-by-Language Deep Comparison

### 9.1 Complete Comparison Table

```
┌────────────────────────────────────────────────────────────────────────────────┐
│          C vs Go vs Rust — Feature-by-Feature Comparison                       │
├───────────────────────────┬──────────────────┬──────────────┬──────────────────┤
│  Feature                  │       C          │     Go       │      Rust        │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Integer types             │ int, long, etc.  │ int, int8,   │ i8-i128, u8-u128 │
│                           │ (size varies!)   │ int16...     │ isize, usize     │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Default integer type      │ int (16+ bits)   │ int (word)   │ i32              │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Integer overflow           │ UNDEFINED (signed│ Wraps        │ Panic(dbg)/Wrap  │
│                           │ Wraps (unsigned) │              │ (release)        │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ String type               │ char* (NUL term) │ string (fat  │ &str / String    │
│                           │                  │  pointer)    │ (fat pointer)    │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ String mutability         │ char[] mutable   │ Immutable    │ &str immutable   │
│                           │ char* dangerous  │              │ String mutable   │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Assignment                │ Expression       │ Statement    │ Statement (→ ()) │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ if                        │ Statement        │ Statement    │ EXPRESSION       │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ loops                     │ Statement        │ Statement    │ EXPRESSION       │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ blocks { }                │ Compound stmt    │ Compound     │ EXPRESSION       │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Ternary ?: operator       │ YES              │ NO           │ NO (use if-else) │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ x++ as expression         │ YES              │ NO (stmt)    │ NO (use x+=1)    │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Implicit type conversions │ Many (dangerous) │ None         │ None             │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Null safety               │ NO (UB)          │ Runtime check│ Compile-time     │
│                           │                  │ (nil)        │ (Option<T>)      │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Forward declarations      │ YES (extern)     │ NO           │ NO               │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Variable shadowing        │ YES (w/ warning) │ YES (in scope│ YES (idiomatic)  │
│                           │                  │ nesting)     │                  │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Uninitialized variables   │ ALLOWED (UB read)│ NOT ALLOWED  │ NOT ALLOWED      │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Zero values               │ Only for global/ │ ALL vars     │ Must init or use │
│                           │ static vars      │ have zeros   │ MaybeUninit      │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Multiline strings         │ Adjacent concat  │ Raw strings  │ Raw strings      │
│                           │ "str1" "str2"    │ backtick     │ r#"..."#         │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Digit separators          │ ' (C23 only)     │ _            │ _                │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Named constants           │ #define or const │ const        │ const            │
├───────────────────────────┼──────────────────┼──────────────┼──────────────────┤
│ Compile-time computation  │ constexpr (C23)  │ const        │ const fn         │
└────────────────────────────────────────────────────────────────────────────────┘
```

---

## 10. Production-Grade Examples

### 10.1 C — Complete Example with All Concepts

```c
/**
 * @file expression_demo.c
 * @brief Production-grade demonstration of C expressions, statements,
 *        declarations, and literals.
 *
 * Compile: gcc -std=c17 -Wall -Wextra -Wpedantic -O2 -o demo expression_demo.c
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdbool.h>
#include <limits.h>
#include <errno.h>

/* ─── Named Constants (no magic numbers) ────────────────────────────────── */
#define BUFFER_SIZE    256U
#define MAX_ITEMS      64U
#define INVALID_INDEX  SIZE_MAX

/* ─── Type Declarations ──────────────────────────────────────────────────── */
typedef enum {
    STATUS_OK      =  0,
    STATUS_ERROR   = -1,
    STATUS_NOT_FOUND = -2,
} Status;

typedef struct {
    int32_t  x;
    int32_t  y;
} Point;

typedef struct {
    Point  *data;
    size_t  count;
    size_t  capacity;
} PointArray;

/* ─── Forward Declarations ───────────────────────────────────────────────── */
static PointArray *point_array_create(size_t initial_capacity);
static Status      point_array_push(PointArray *arr, Point p);
static void        point_array_destroy(PointArray *arr);
static int32_t     point_distance_sq(Point a, Point b);
static void        point_print(Point p);

/* ─── Function Definitions ───────────────────────────────────────────────── */

/**
 * Demonstrates literal kinds in C.
 * NOTE: All variables initialized — no UB from uninitialized reads.
 */
static void demonstrate_literals(void) {
    /* Integer literals */
    int32_t decimal     =  42;
    int32_t hex         =  0x2A;        /* same value, hex form */
    int32_t octal       =  052;         /* same value, octal form */
    int32_t binary      =  0b101010;    /* GCC extension / C23 */
    uint32_t unsigned_l =  42U;
    int64_t  long_l     =  42LL;
    uint64_t ulong_l    =  42ULL;

    /* Floating-point literals */
    double   dbl        =  3.14159265358979323846;
    float    flt        =  3.14159265f;
    double   sci        =  1.602e-19;   /* electron charge */
    double   hex_flt    =  0x1.0p-52;  /* machine epsilon, exact */

    /* Character literals */
    char     ch         = 'A';
    int      newline    = '\n';
    int      null_ch    = '\0';

    /* String literal */
    const char *greeting = "hello, world";  /* read-only, in .rodata */
    char  mutable_str[]  = "mutable copy";  /* on stack, modifiable */

    /* Boolean literals (C99+) */
    bool t = true;
    bool f = false;

    /* Compound literal (C99): anonymous struct on stack */
    Point origin = (Point){ .x = 0, .y = 0 };

    /* Print to "use" variables and suppress warnings */
    printf("decimal=%d hex=%d octal=%d binary=%d\n",
           decimal, hex, octal, binary);
    printf("u32=%u i64=%lld u64=%llu\n", unsigned_l, (long long)long_l, (unsigned long long)ulong_l);
    printf("double=%.15f float=%.7f sci=%e hex_float=%a\n", dbl, (double)flt, sci, hex_flt);
    printf("char='%c' newline=%d null=%d\n", ch, newline, null_ch);
    printf("string='%s' mutable='%s'\n", greeting, mutable_str);
    printf("bool: true=%d false=%d\n", t, f);
    printf("compound literal: (%d, %d)\n", origin.x, origin.y);
}

/**
 * Demonstrates expressions of various categories.
 */
static void demonstrate_expressions(void) {
    int32_t a = 17, b = 5;

    /* Arithmetic expressions */
    int32_t sum     = a + b;          /* 22  */
    int32_t diff    = a - b;          /* 12  */
    int32_t product = a * b;          /* 85  */
    int32_t quotient = a / b;         /* 3   — integer division truncates */
    int32_t remainder = a % b;        /* 2   */
    int32_t negated  = -a;            /* -17 */

    /* Bitwise expressions */
    uint32_t ua = (uint32_t)a;
    uint32_t mask  = ua & 0xFU;       /* lower 4 bits: 17 & 0xF = 1 */
    uint32_t flags = ua | 0x80U;      /* set bit 7 */
    uint32_t toggled = ua ^ 0xFFU;    /* XOR with 0xFF */
    uint32_t shifted_l = ua << 2U;    /* multiply by 4 */
    uint32_t shifted_r = ua >> 1U;    /* divide by 2 */

    /* Comparison expressions — result is int (0 or 1) */
    int gt = (a > b);
    int lt = (a < b);
    int eq = (a == b);

    /* Logical expressions — short-circuit */
    int32_t *ptr = &a;
    bool safe_deref = (ptr != NULL) && (*ptr > 0);  /* short-circuit protects */

    /* Conditional (ternary) expression */
    int32_t max_val = (a > b) ? a : b;   /* value-producing if-else */
    int32_t abs_a   = (a >= 0) ? a : -a;

    /* Compound assignment expressions */
    int32_t x = 10;
    x += 5;   /* x = 15 */
    x *= 2;   /* x = 30 */
    x >>= 1;  /* x = 15 */

    /* Assignment as expression (C-specific) */
    int32_t y;
    int32_t z = (y = 42);   /* y=42 is expression with value 42; z=42 */

    printf("sum=%d diff=%d product=%d quotient=%d remainder=%d negated=%d\n",
           sum, diff, product, quotient, remainder, negated);
    printf("mask=0x%X flags=0x%X toggled=0x%X lshift=%u rshift=%u\n",
           mask, flags, toggled, shifted_l, shifted_r);
    printf("gt=%d lt=%d eq=%d safe_deref=%d max=%d abs=%d\n",
           gt, lt, eq, safe_deref, max_val, abs_a);
    printf("x=%d y=%d z=%d\n", x, y, z);
}

/**
 * Demonstrates statements of various kinds.
 */
static void demonstrate_statements(void) {
    /* Expression statements */
    int32_t x = 0;
    x = 5;           /* assignment expression statement */
    x++;             /* increment expression statement */
    (void)printf("x=%d\n", x);  /* function call statement, cast suppresses warn */

    /* If-else statement chain */
    if (x > 10) {
        puts("large");
    } else if (x > 5) {
        puts("medium");
    } else {
        puts("small");
    }

    /* Switch statement */
    switch (x) {
        case 0:
            puts("zero");
            break;
        case 5:
        case 6:
            puts("five or six");
            break;
        default:
            puts("other");
            break;
    }

    /* For loop */
    int32_t sum = 0;
    for (int32_t i = 0; i < 10; i++) {
        sum += i;
    }

    /* While loop */
    int32_t n = 10;
    while (n > 0) {
        n--;
    }

    /* Do-while: always runs body at least once */
    int32_t tries = 0;
    do {
        tries++;
    } while (tries < 3);

    printf("sum=%d n=%d tries=%d\n", sum, n, tries);
}

/* ─── Dynamic Array Implementation ───────────────────────────────────────── */

static PointArray *point_array_create(size_t initial_capacity) {
    if (initial_capacity == 0) {
        initial_capacity = 8U;
    }

    PointArray *arr = malloc(sizeof(PointArray));
    if (arr == NULL) {
        return NULL;
    }

    arr->data = malloc(initial_capacity * sizeof(Point));
    if (arr->data == NULL) {
        free(arr);
        return NULL;
    }

    arr->count    = 0;
    arr->capacity = initial_capacity;
    return arr;
}

static Status point_array_push(PointArray *arr, Point p) {
    if (arr == NULL) { return STATUS_ERROR; }

    if (arr->count >= arr->capacity) {
        /* Growth strategy: double capacity */
        size_t new_cap = arr->capacity * 2U;
        if (new_cap < arr->capacity) { /* overflow check */
            return STATUS_ERROR;
        }
        Point *new_data = realloc(arr->data, new_cap * sizeof(Point));
        if (new_data == NULL) {
            return STATUS_ERROR;
        }
        arr->data     = new_data;
        arr->capacity = new_cap;
    }

    arr->data[arr->count++] = p;
    return STATUS_OK;
}

static void point_array_destroy(PointArray *arr) {
    if (arr == NULL) { return; }
    free(arr->data);
    arr->data     = NULL;
    arr->count    = 0;
    arr->capacity = 0;
    free(arr);
}

static int32_t point_distance_sq(Point a, Point b) {
    int32_t dx = a.x - b.x;
    int32_t dy = a.y - b.y;
    return dx * dx + dy * dy;
}

static void point_print(Point p) {
    printf("(%d, %d)", p.x, p.y);
}

int main(void) {
    puts("=== C Literals ===");
    demonstrate_literals();

    puts("\n=== C Expressions ===");
    demonstrate_expressions();

    puts("\n=== C Statements ===");
    demonstrate_statements();

    puts("\n=== C Dynamic Array ===");
    PointArray *arr = point_array_create(4U);
    if (arr == NULL) {
        fputs("allocation failed\n", stderr);
        return EXIT_FAILURE;
    }

    /* Designated initializer compound literals */
    Point points[] = {
        (Point){.x =  0, .y =  0},
        (Point){.x =  3, .y =  4},
        (Point){.x = -1, .y =  7},
        (Point){.x =  5, .y = -2},
    };

    for (size_t i = 0; i < sizeof(points) / sizeof(points[0]); i++) {
        if (point_array_push(arr, points[i]) != STATUS_OK) {
            fputs("push failed\n", stderr);
            point_array_destroy(arr);
            return EXIT_FAILURE;
        }
    }

    Point origin = {.x = 0, .y = 0};
    for (size_t i = 0; i < arr->count; i++) {
        point_print(arr->data[i]);
        printf(" → dist² from origin = %d\n",
               point_distance_sq(arr->data[i], origin));
    }

    point_array_destroy(arr);
    return EXIT_SUCCESS;
}
```

### 10.2 Go — Complete Example

```go
// Package exprsdemo demonstrates expressions, statements, declarations,
// and literals in idiomatic, production-quality Go.
//
// Build: go build -v ./...
// Run:   go run expression_demo.go
package main

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// ─── Named Constants ────────────────────────────────────────────────────────

const (
	MaxItems     = 64
	DefaultCap   = 8
	InvalidIndex = -1
)

// iota: auto-incrementing constant generator
type Direction int

const (
	North Direction = iota // 0
	South                  // 1
	East                   // 2
	West                   // 3
)

func (d Direction) String() string {
	return [...]string{"North", "South", "East", "West"}[d]
}

// ─── Type Declarations ───────────────────────────────────────────────────────

// Point is a 2D integer coordinate.
type Point struct {
	X, Y int
}

// DistanceSq returns the squared Euclidean distance between two points.
// Using squared distance avoids sqrt — sufficient for comparison.
func (p Point) DistanceSq(other Point) float64 {
	dx := float64(p.X - other.X)
	dy := float64(p.Y - other.Y)
	return dx*dx + dy*dy
}

// Distance returns the Euclidean distance between two points.
func (p Point) Distance(other Point) float64 {
	return math.Sqrt(p.DistanceSq(other))
}

func (p Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}

// PointSlice is a named type for []Point to add methods.
type PointSlice []Point

// Closest returns the index and point closest to origin.
// Returns InvalidIndex, zero-value Point, and an error if empty.
func (ps PointSlice) Closest(origin Point) (int, Point, error) {
	if len(ps) == 0 {
		return InvalidIndex, Point{}, errors.New("pointslice: empty slice")
	}
	bestIdx := 0
	bestDist := ps[0].DistanceSq(origin)
	for i := 1; i < len(ps); i++ {
		if d := ps[i].DistanceSq(origin); d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx, ps[bestIdx], nil
}

// ─── Literal Demonstrations ──────────────────────────────────────────────────

func demonstrateLiterals() {
	// Integer literals
	decimal := 42
	hex := 0x2A
	octal := 0o52     // explicit octal (preferred over legacy 052)
	binary := 0b101010
	bigNum := 1_000_000 // digit separator

	// Float literals
	dbl := 3.14159265358979
	sci := 1.602e-19
	hexFlt := 0x1.8p+1 // = 3.0 exactly (hex float)

	// String literals
	interpreted := "hello\nworld" // escape sequences processed
	raw := `hello\nworld`         // raw: no escape processing
	multiline := `
Line 1
Line 2
Line 3
`

	// Rune literal
	ch := 'A'
	emoji := '😀'

	// Boolean literals
	t := true
	f := false

	// Composite literals
	pt := Point{X: 3, Y: 4}
	pts := []Point{{1, 2}, {3, 4}, {5, 6}}
	m := map[string]int{"one": 1, "two": 2, "three": 3}
	arr := [3]int{10, 20, 30}

	// nil literal (zero value for pointer/slice/map/func/interface/channel)
	var nilSlice []int
	var nilMap map[string]int
	var nilPtr *int

	fmt.Printf("Integers: dec=%d hex=%d oct=%d bin=%d big=%d\n",
		decimal, hex, octal, binary, bigNum)
	fmt.Printf("Floats: double=%.15f sci=%e hexflt=%f\n", dbl, sci, hexFlt)
	fmt.Printf("Strings: interpreted=%q raw=%q\n", interpreted, raw)
	fmt.Printf("Multiline: %q\n", strings.TrimSpace(multiline))
	fmt.Printf("Rune: ch='%c'(%d) emoji='%c'(%d)\n", ch, ch, emoji, emoji)
	fmt.Printf("Bool: true=%v false=%v\n", t, f)
	fmt.Printf("Composite: point=%v points=%v map=%v array=%v\n", pt, pts, m, arr)
	fmt.Printf("Nil: slice=%v map=%v ptr=%v\n", nilSlice, nilMap, nilPtr)
}

// ─── Expression Demonstrations ───────────────────────────────────────────────

func demonstrateExpressions() {
	a, b := 17, 5

	// Arithmetic
	sum := a + b
	diff := a - b
	product := a * b
	quotient := a / b    // integer division
	remainder := a % b

	// Bitwise
	bitwiseAnd := a & b
	bitwiseOr := a | b
	bitwiseXor := a ^ b
	leftShift := a << 2
	rightShift := a >> 1
	complement := ^a // bitwise NOT (Go uses ^ for NOT, not ~)

	// Comparison (result: bool)
	gt := a > b
	lt := a < b
	eq := a == b
	ne := a != b

	// Logical (short-circuit)
	var ptr *int
	safeDereference := ptr != nil && *ptr > 0 // short-circuit: ptr check first

	// Go has NO ternary — use if expression:
	max := a
	if b > a {
		max = b
	}

	// Type conversion (explicit, no implicit)
	flt := float64(a)
	back := int(flt)

	// Function call expression
	dist := math.Sqrt(float64(a*a + b*b))

	fmt.Printf("a=%d b=%d\n", a, b)
	fmt.Printf("arith: sum=%d diff=%d prod=%d quot=%d rem=%d\n",
		sum, diff, product, quotient, remainder)
	fmt.Printf("bitwise: and=%d or=%d xor=%d <<=%d >>=%d comp=%d\n",
		bitwiseAnd, bitwiseOr, bitwiseXor, leftShift, rightShift, complement)
	fmt.Printf("cmp: gt=%v lt=%v eq=%v ne=%v\n", gt, lt, eq, ne)
	fmt.Printf("logical: safeDeref=%v\n", safeDereference)
	fmt.Printf("max=%d flt=%f back=%d dist=%.4f\n", max, flt, back, dist)
}

// ─── Statement Demonstrations ────────────────────────────────────────────────

func demonstrateStatements() {
	// Short variable declaration statement
	x := 10

	// Assignment statement (NOT expression in Go)
	x = 20

	// Increment/decrement statements (NOT expressions in Go)
	x++
	x--

	// If statement with init statement (x scoped to if block):
	if y := x * 2; y > 30 {
		fmt.Printf("if-init: y=%d is > 30\n", y)
	} else {
		fmt.Printf("if-init: y=%d is <= 30\n", y)
	}

	// Switch statement (no fallthrough by default)
	switch x {
	case 0:
		fmt.Println("zero")
	case 10, 20:
		fmt.Println("ten or twenty")
	default:
		fmt.Println("other")
	}

	// Expression switch (like if-else chain)
	switch {
	case x < 0:
		fmt.Println("negative")
	case x == 0:
		fmt.Println("zero")
	default:
		fmt.Println("positive")
	}

	// For loop variants
	sum := 0
	for i := 0; i < 10; i++ {
		sum += i
	}
	fmt.Printf("sum 0..9 = %d\n", sum)

	// While-style for
	n := 10
	for n > 0 {
		n /= 2
	}
	fmt.Printf("n after halving = %d\n", n)

	// Range for over slice
	nums := []int{1, 2, 3, 4, 5}
	product := 1
	for _, v := range nums {
		product *= v
	}
	fmt.Printf("product = %d\n", product)

	// Labeled break
	found := false
outer:
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			if i*j == 6 {
				found = true
				break outer
			}
		}
	}
	fmt.Printf("found product 6: %v\n", found)

	// Return statement is in functions — see examples below
}

// ─── Error Handling Pattern ──────────────────────────────────────────────────

// ErrInvalidInput is a sentinel error.
var ErrInvalidInput = errors.New("invalid input")

// safeDivide performs checked integer division.
func safeDivide(a, b int) (int, error) {
	if b == 0 {
		return 0, fmt.Errorf("safeDivide(%d, %d): %w", a, b, ErrInvalidInput)
	}
	return a / b, nil
}

// processPoints demonstrates a complete real workflow.
func processPoints(pts PointSlice) error {
	if len(pts) == 0 {
		return errors.New("processPoints: no points to process")
	}

	origin := Point{X: 0, Y: 0}
	idx, closest, err := pts.Closest(origin)
	if err != nil {
		return fmt.Errorf("processPoints: %w", err)
	}

	fmt.Printf("  Closest to origin: pts[%d] = %v (dist=%.4f)\n",
		idx, closest, closest.Distance(origin))

	for i, p := range pts {
		fmt.Printf("  pts[%d] = %v  dist=%.4f\n", i, p, p.Distance(origin))
	}
	return nil
}

func main() {
	fmt.Println("=== Go Literals ===")
	demonstrateLiterals()

	fmt.Println("\n=== Go Expressions ===")
	demonstrateExpressions()

	fmt.Println("\n=== Go Statements ===")
	demonstrateStatements()

	fmt.Println("\n=== Go Declarations & Types ===")
	fmt.Printf("Direction North=%v South=%v\n", North, South)

	pts := PointSlice{
		{X: 0, Y: 0},
		{X: 3, Y: 4},
		{X: -1, Y: 7},
		{X: 5, Y: -2},
	}

	fmt.Println("\n=== Go Functions & Error Handling ===")
	if err := processPoints(pts); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	q, err := safeDivide(10, 3)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("10 / 3 = %d\n", q)
	}

	_, err = safeDivide(10, 0)
	if errors.Is(err, ErrInvalidInput) {
		fmt.Printf("Caught expected error: %v\n", err)
	}
}
```

### 10.3 Rust — Complete Example

```rust
//! Production-grade demonstration of Rust expressions, statements,
//! declarations, and literals.
//!
//! Run: cargo run --release

use std::fmt;

// ─── Named Constants ─────────────────────────────────────────────────────────

const MAX_ITEMS: usize = 64;
const DEFAULT_CAP: usize = 8;
const INVALID_INDEX: isize = -1;

// ─── Type Declarations ────────────────────────────────────────────────────────

/// A 2D integer coordinate.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
struct Point {
    x: i32,
    y: i32,
}

impl Point {
    /// Construct a new Point.
    const fn new(x: i32, y: i32) -> Self {
        Self { x, y }
    }

    /// Compute squared distance (avoids sqrt, useful for comparisons).
    fn distance_sq(self, other: Self) -> f64 {
        let dx = (self.x - other.x) as f64;
        let dy = (self.y - other.y) as f64;
        dx * dx + dy * dy
    }

    /// Compute Euclidean distance.
    fn distance(self, other: Self) -> f64 {
        self.distance_sq(other).sqrt()
    }
}

impl fmt::Display for Point {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "({}, {})", self.x, self.y)
    }
}

/// Named direction type (enum as sum type).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Direction {
    North,
    South,
    East,
    West,
}

impl fmt::Display for Direction {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Direction::North => write!(f, "North"),
            Direction::South => write!(f, "South"),
            Direction::East  => write!(f, "East"),
            Direction::West  => write!(f, "West"),
        }
    }
}

/// Custom error type for point operations.
#[derive(Debug)]
enum PointError {
    EmptySlice,
    InvalidOperation(String),
}

impl fmt::Display for PointError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            PointError::EmptySlice            => write!(f, "point error: empty slice"),
            PointError::InvalidOperation(msg) => write!(f, "point error: {msg}"),
        }
    }
}

impl std::error::Error for PointError {}

// ─── Literal Demonstrations ───────────────────────────────────────────────────

fn demonstrate_literals() {
    // Integer literals — explicit types via suffix or annotation
    let decimal:  i32    =  42;
    let hex:      i32    =  0x2A;
    let octal:    i32    =  0o52;      // always explicit octal in Rust
    let binary:   i32    =  0b10_1010; // underscores for readability
    let big_num:  i64    =  1_000_000_i64;
    let unsigned: u64    =  42_u64;
    let byte_val: u8     =  b'A';      // byte literal from ASCII char = 65
    let pointer_sz: usize = 42_usize;  // platform-dependent size

    // Float literals
    let dbl:  f64 = 3.141_592_653_589_793;
    let flt:  f32 = 3.141_592_f32;
    let sci:  f64 = 1.602e-19_f64;
    let inf:  f64 = f64::INFINITY;
    let nan:  f64 = f64::NAN;
    let eps:  f64 = f64::EPSILON;      // 2^-52 ≈ 2.2e-16

    // Character literals (always 4-byte Unicode scalar value)
    let ch:     char = 'A';
    let cjk:    char = '界';           // Chinese character
    let emoji:  char = '😀';           // emoji (4 bytes)
    let escape: char = '\n';
    let unicode_escape: char = '\u{1F600}'; // same as emoji above

    // String literals
    let s1: &str = "hello, world";
    let s2: &str = "line1\nline2\ttab";     // with escapes
    let s3: &str = r"no\nescape\nhere";     // raw string
    let s4: &str = r#"can contain "quotes" inside"#; // raw with delimiter
    let multiline: &str = r#"
        Line 1
        Line 2
        Line 3
    "#;

    // Byte string literal (type: &[u8])
    let bytes: &[u8] = b"hello";

    // Boolean
    let t: bool = true;
    let f: bool = false;

    // Composite literals
    let pt  = Point::new(3, 4);
    let pts = [Point::new(0,0), Point::new(1,1), Point::new(2,2)];
    let tup: (i32, f64, &str) = (42, 3.14, "hello");
    let arr = [0_i32; 5];  // [0, 0, 0, 0, 0]
    let vec_lit: Vec<i32> = vec![1, 2, 3, 4, 5];

    // None — safe null
    let nothing: Option<i32> = None;
    let something: Option<i32> = Some(42);

    println!("Integers: dec={decimal} hex={hex} oct={octal} bin={binary}");
    println!("  big={big_num} u64={unsigned} byte=0x{byte_val:02X} usize={pointer_sz}");
    println!("Floats: f64={dbl:.15} f32={flt:.7} sci={sci:e}");
    println!("  inf={inf} nan={nan} eps={eps:e}");
    println!("Chars: ch='{ch}' cjk='{cjk}' emoji='{emoji}' escape={escape:?} uescape='{unicode_escape}'");
    println!("Strings: s1={s1:?} raw={s3:?}");
    println!("  s4={s4:?}");
    println!("  multiline={multiline:?}");
    println!("Bytes: {bytes:?}");
    println!("Bool: true={t} false={f}");
    println!("Composite: pt={pt} pts={pts:?} tuple={tup:?}");
    println!("  arr={arr:?} vec={vec_lit:?}");
    println!("Option: nothing={nothing:?} something={something:?}");
}

// ─── Expression Demonstrations ────────────────────────────────────────────────

fn demonstrate_expressions() {
    let a: i32 = 17;
    let b: i32 = 5;

    // Arithmetic — checked, wrapping, and saturating variants
    let sum       = a + b;
    let diff      = a - b;
    let product   = a * b;
    let quotient  = a / b;    // integer division: truncates toward zero
    let remainder = a % b;    // sign of dividend in Rust (not modulus!)
    let negated   = -a;

    // Overflow-safe arithmetic:
    let checked   = a.checked_add(i32::MAX);    // None on overflow
    let wrapping  = a.wrapping_add(i32::MAX);   // wraps (always defined)
    let saturated = a.saturating_add(i32::MAX); // clamps to MAX

    // Bitwise
    let and  = a & b;
    let or   = a | b;
    let xor  = a ^ b;
    let not  = !a;           // bitwise NOT in Rust (same as ~ in C)
    let shl  = a << 2_i32;
    let shr  = a >> 1_i32;

    // Comparison — result: bool
    let gt = a > b;
    let lt = a < b;
    let eq = a == b;

    // Logical — short-circuit
    let x: Option<i32> = Some(42);
    let safe = x.is_some() && x.unwrap() > 0; // safe: is_some checked first

    // if as expression — NO ternary in Rust
    let max  = if a > b { a } else { b };
    let sign = if a > 0 { 1 } else if a < 0 { -1 } else { 0 };

    // match as expression
    let category = match a {
        0         => "zero",
        1..=9     => "single digit",
        10..=99   => "double digit",
        _         => "large",
    };

    // Block as expression
    let computed = {
        let temp = a * a + b * b;
        (temp as f64).sqrt()          // block evaluates to this
    };

    // Loop as expression
    let mut counter = 0_i32;
    let loop_result = loop {
        counter += 1;
        if counter >= 5 { break counter * 10; }
    };

    // Closure as expression
    let square = |n: i32| n * n;
    let cube   = |n: i32| n * n * n;
    let sq_a = square(a);
    let cu_b = cube(b);

    println!("a={a} b={b}");
    println!("arith: sum={sum} diff={diff} prod={product} quot={quotient} rem={remainder} neg={negated}");
    println!("checked: {checked:?} wrapping={wrapping} saturated={saturated}");
    println!("bitwise: and={and} or={or} xor={xor} not={not} shl={shl} shr={shr}");
    println!("cmp: gt={gt} lt={lt} eq={eq}  logical safe={safe}");
    println!("if-expr: max={max} sign={sign}  match={category}");
    println!("block={computed:.4}  loop={loop_result}  sq={sq_a} cu={cu_b}");
}

// ─── Statement Demonstrations ─────────────────────────────────────────────────

fn demonstrate_statements() {
    // let statement — always a statement, never an expression
    let x: i32 = 10;
    let mut y: i32 = 20;

    // Assignment statement (returns () — unit)
    y = 30;

    // Compound assignment
    y += 10; // y = 40

    // Expression statement: expression + semicolon
    println!("x={x} y={y}"); // macro call, returns ()

    // if statement (when result is not used):
    if x > 5 {
        println!("x is greater than 5");
    }

    // while statement
    let mut count = 0_i32;
    while count < 3 {
        count += 1;
    }

    // while let statement
    let mut stack = vec![1, 2, 3];
    while let Some(top) = stack.pop() {
        println!("popped: {top}");
    }

    // for statement (iterates over Iterator)
    let mut sum = 0_i32;
    for i in 0..10_i32 {
        sum += i;
    }
    println!("sum 0..10 = {sum}");

    // Shadowing via let statements (different types!):
    let count = count.to_string(); // shadows i32 with String
    let count: usize = count.len();   // shadows String with usize
    println!("count len = {count}");

    // let-else statement (Rust 1.65+):
    let maybe: Option<i32> = Some(42);
    let Some(value) = maybe else {
        println!("was None, returning early");
        return;
    };
    println!("value = {value}");
}

// ─── Functions with Ownership Semantics ──────────────────────────────────────

/// Find the point closest to origin.
/// Returns error if slice is empty.
fn closest_to_origin(points: &[Point]) -> Result<(usize, Point), PointError> {
    if points.is_empty() {
        return Err(PointError::EmptySlice);
    }

    let origin = Point::new(0, 0);
    let (idx, _dist) = points
        .iter()
        .enumerate()
        .map(|(i, p)| (i, p.distance_sq(origin)))
        .min_by(|(_i, d1), (_j, d2)| d1.partial_cmp(d2).unwrap())
        .ok_or_else(|| PointError::InvalidOperation("min failed".into()))?;

    Ok((idx, points[idx]))
}

/// Process a slice of points, printing analysis.
fn process_points(points: &[Point]) -> Result<(), PointError> {
    if points.is_empty() {
        return Err(PointError::EmptySlice);
    }

    let origin = Point::new(0, 0);

    let (idx, closest) = closest_to_origin(points)?;
    println!("  Closest to origin: points[{idx}] = {closest} (dist={:.4})",
             closest.distance(origin));

    for (i, p) in points.iter().enumerate() {
        println!("  points[{i}] = {p}  dist={:.4}", p.distance(origin));
    }
    Ok(())
}

// ─── Main ─────────────────────────────────────────────────────────────────────

fn main() {
    println!("=== Rust Literals ===");
    demonstrate_literals();

    println!("\n=== Rust Expressions ===");
    demonstrate_expressions();

    println!("\n=== Rust Statements ===");
    demonstrate_statements();

    println!("\n=== Rust Enum and Match ===");
    let dir = Direction::North;
    println!("Direction: {dir}");

    let description = match dir {
        Direction::North => "heading up",
        Direction::South => "heading down",
        Direction::East  => "heading right",
        Direction::West  => "heading left",
    };
    println!("  → {description}");

    println!("\n=== Rust Points & Error Handling ===");
    // Compile-time point creation (const fn)
    const ORIGIN: Point = Point::new(0, 0);
    let points = [
        ORIGIN,
        Point::new(3, 4),
        Point::new(-1, 7),
        Point::new(5, -2),
    ];

    match process_points(&points) {
        Ok(()) => println!("Processing complete."),
        Err(e) => eprintln!("Error: {e}"),
    }

    // Test error case
    match process_points(&[]) {
        Ok(()) => println!("No error (unexpected)"),
        Err(PointError::EmptySlice) => println!("Caught expected EmptySlice error"),
        Err(e) => eprintln!("Unexpected error: {e}"),
    }

    // Demonstrate const fn evaluated at compile time
    const _COMPILE_TIME_PT: Point = Point::new(10, 20);
    println!("\nAll demonstrations complete.");
}
```

---

## 11. Cognitive Framework: Mental Models for Expert Thinking

### 11.1 The "Can I Put It on the Right of =" Test

When you see any syntax construct and need to classify it:

```
QUICK CLASSIFICATION TEST
                                                                     
  Step 1: Ask — "Does this produce a value I could use?"

    YES → EXPRESSION
       Could it be written as: let x = <this construct>; ?
       If YES → confirmed expression
       
    NO  → STATEMENT or DECLARATION
       Does it introduce a name? → DECLARATION
       Does it cause an action?  → STATEMENT

  Special case for Rust:
    if, loop, match, { } blocks → ALL expressions
    let x = ... → STATEMENT (even though it has =, it's a let statement)
    x = ... → STATEMENT (assignment is a statement returning ())
    
  Special case for C:
    x = 5 → EXPRESSION (value is 5, side effect is storing into x)
    int x = 5 → DECLARATION (introduces x)
```

### 11.2 The "What Does the Compiler Need to Know" Model

Think like the compiler when reading code:

```
COMPILER NEEDS TO KNOW              SATISFIED BY
────────────────────────────────    ─────────────────────────
"What name am I binding to?"    →   Declaration
"What type does this have?"     →   Type annotation / inference
"What value does this start at?"→   Initializer / literal
"Where is this name visible?"   →   Scope rules
"How long does this value live?"→   Lifetime (Rust) / scope (C, Go)
"What operation to perform?"    →   Operator / function call
"What is the result value?"     →   Expression evaluation
"What side effect occurs?"      →   Statement execution
```

### 11.3 The Cognitive Chunking Hierarchy

Master programmers do not read code token-by-token. They chunk patterns:

```
Level 1: TOKEN           int  x  =  5  ;
Level 2: PHRASE          [type] [name] [= initializer] [;]
Level 3: CONSTRUCT       [variable declaration with initializer]
Level 4: PATTERN         [named constant for magic number elimination]
Level 5: ARCHITECTURE    [module state management via static allocation]

Goal: Think at Level 4-5, verify at Level 1-2 only when debugging.
```

### 11.4 Language Philosophy Summary

```
┌──────────────────────────────────────────────────────────────────────┐
│  LANGUAGE PHILOSOPHY — Why They Made These Choices                   │
│                                                                      │
│  C:   "Trust the programmer. Let everything be an expression.       │
│        Power and danger in equal measure."                           │
│        → Assignment as expression: enables terse, powerful idioms   │
│          but also infamous bugs (= vs ==)                            │
│                                                                      │
│  Go:  "Clarity is better than cleverness. Restrict to prevent       │
│        confusion. One obvious way to do everything."                 │
│        → Assignment as statement: removes = vs == confusion          │
│        → No ternary: forces readable multi-line if-else              │
│        → No x++ expression: removes subtle precedence bugs           │
│                                                                      │
│  Rust:"Safety without sacrificing expressiveness. If something      │
│        can be an expression, make it one. Let the type system        │
│        catch everything at compile time."                            │
│        → if, match, loop, blocks as expressions: enables elegant    │
│          patterns without unsafe ternary tricks                      │
│        → Semicolons change block type: precise, learnable rule       │
│        → let is statement: clear separation of binding vs evaluation │
└──────────────────────────────────────────────────────────────────────┘
```

### 11.5 The Expert's Reading Order

When encountering unfamiliar code, experts scan in this order:

```
1. DECLARATIONS first — what names exist? what types?
2. FUNCTION SIGNATURES — what are inputs, outputs, ownership?
3. CONTROL FLOW — what paths can execution take?
4. EXPRESSIONS — what values are computed?
5. LITERALS — what constants are hardcoded?
6. SIDE EFFECTS — what external state is modified?

This is top-down, then bottom-up thinking:
  Top-down: understand structure (declarations, types)
  Bottom-up: verify values (expressions, literals)
```

---

## Quick Reference Summary

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          MASTER QUICK REFERENCE                             │
├────────────────┬────────────────────────────────────────────────────────────┤
│  LITERALS      │  Fixed values in source: 42  3.14  'A'  "hello"  true      │
│                │  No computation — value known at compile time              │
│                │  May live in: instruction (imm), stack, .rodata, .data     │
├────────────────┼────────────────────────────────────────────────────────────┤
│  EXPRESSIONS   │  Produce a value. Can always appear on right of =           │
│                │  Includes: literals, identifiers, operators, calls          │
│                │  Rust adds: if, match, loop, block { }                     │
│                │  C adds:    assignment =, ternary ?:                       │
├────────────────┼────────────────────────────────────────────────────────────┤
│  STATEMENTS    │  Cause effects. Control flow. Cannot be nested as values   │
│                │  if, for, while, return, break, continue                   │
│                │  Expression statement: expression + ;  (discards value)    │
│                │  Rust semicolon: converts expression to statement (→ ())   │
├────────────────┼────────────────────────────────────────────────────────────┤
│  DECLARATIONS  │  Introduce names into scope                                │
│                │  Variable, function, type, const, static                   │
│                │  C: can be separate from definition (extern)               │
│                │  Go/Rust: declaration = definition (always)                │
└────────────────┴────────────────────────────────────────────────────────────┘

The Semicolon Rule (Rust):
  fn f() -> i32 { 5 }    → returns 5   (expression, no semicolon)
  fn g() -> ()  { 5; }   → returns ()  (statement, semicolon consumed value)

The Assignment Rule:
  C:    x = 5    is an expression with value 5   (use in conditions, chain)
  Go:   x = 5    is a statement (cannot use as value)
  Rust: x = 5    is a statement returning ()     (cannot use as value)

The Null Safety Rule:
  C:    NULL is just 0 cast to pointer — dereference = undefined behavior
  Go:   nil is zero value — dereference at runtime = panic (recoverable)
  Rust: None is a type-safe variant — compiler FORCES you to handle it
```

---

*Built for the monk who codes like a world-class scientist.*
*Study deeply. Practice deliberately. Understand the machine.*
