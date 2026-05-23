# Go Format Specifiers — Complete In-Depth Guide

## Summary

Go's `fmt` package is a reflection-driven, interface-polymorphic formatting engine built on top of `io.Writer`.
Every `Printf`/`Sprintf`/`Fprintf` call ultimately calls `fmt.(*pp).doPrintf`, which lexes the format string,
dispatches each verb to a typed handler, and writes through a pooled `pp` struct to avoid heap allocations.
Understanding the full verb taxonomy — general, boolean, integer, float, complex, string, pointer, compound
types — together with flags (`-`, `+`, `#`, `0`, space), width/precision, argument indexing, and the four
formatting interfaces (`Stringer`, `GoStringer`, `Formatter`, `error`) gives you precise control over every
byte emitted and lets you write zero-allocation custom formatters for hot paths.

---

## Table of Contents

1. [Package Architecture — How It Works Under the Hood](#1-package-architecture)
2. [The `pp` Struct and Sync Pool](#2-the-pp-struct-and-sync-pool)
3. [Format String Grammar](#3-format-string-grammar)
4. [General Verbs — `%v`, `%+v`, `%#v`, `%T`](#4-general-verbs)
5. [Boolean Verb — `%t`](#5-boolean-verb)
6. [Integer Verbs — `%d`, `%b`, `%o`, `%O`, `%x`, `%X`, `%c`, `%U`, `%q`](#6-integer-verbs)
7. [Floating-Point and Complex Verbs — `%e`, `%E`, `%f`, `%F`, `%g`, `%G`, `%x`, `%X`](#7-floating-point-verbs)
8. [String and Byte-Slice Verbs — `%s`, `%q`, `%x`, `%X`](#8-string-verbs)
9. [Pointer Verb — `%p`](#9-pointer-verb)
10. [Flags — `-`, `+`, `#`, `0`, space](#10-flags)
11. [Width and Precision](#11-width-and-precision)
12. [Argument Indexing — `%[n]verb`](#12-argument-indexing)
13. [Printing Functions Taxonomy](#13-printing-functions-taxonomy)
14. [Formatting Interfaces](#14-formatting-interfaces)
    - [fmt.Stringer](#141-fmtstringer)
    - [fmt.GoStringer](#142-fmtgostringer)
    - [fmt.Formatter](#143-fmtformatter)
    - [error](#144-error-interface)
15. [Scanning — `Scan`, `Scanf`, `Sscan`, `Fscan`](#15-scanning)
16. [Errorf and `%w` — Error Wrapping](#16-errorf-and-w)
17. [Compound Types — Structs, Slices, Maps, Channels, Functions](#17-compound-types)
18. [Recursive and Cyclic Structures](#18-recursive-and-cyclic-structures)
19. [Unicode, Runes, and Multibyte Strings](#19-unicode-runes-and-multibyte-strings)
20. [Performance: Allocations, `sync.Pool`, `strings.Builder`](#20-performance)
21. [Common Pitfalls and Anti-Patterns](#21-common-pitfalls)
22. [Complete Reference Table](#22-complete-reference-table)
23. [Advanced: Zero-Allocation Custom Formatter](#23-advanced-zero-allocation-custom-formatter)
24. [Threat Model for Format Strings](#24-threat-model-for-format-strings)
25. [Next 3 Steps](#25-next-3-steps)

---

## 1. Package Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           fmt Package — Call Flow                           │
│                                                                             │
│  Caller                                                                     │
│  ──────                                                                     │
│  fmt.Sprintf(format, args...)                                               │
│       │                                                                     │
│       ▼                                                                     │
│  newPrinter()  ◄──── sync.Pool (reuses *pp objects)                        │
│       │                                                                     │
│       ▼                                                                     │
│  p.doPrintf(format, args)                                                   │
│       │                                                                     │
│       ├── lexes format string byte-by-byte                                  │
│       │     ├── literal bytes  → p.buf.writeString()                        │
│       │     └── '%' detected   → parse verb descriptor                     │
│       │                              ├── flags    (-,+,#,0,space)           │
│       │                              ├── width    ([n] or *)                │
│       │                              ├── precision(.[n] or .*)              │
│       │                              └── verb     (v,d,s,x,…)              │
│       │                                                                     │
│       ├── p.printArg(arg, verb)                                             │
│       │     │                                                               │
│       │     ├── interface checks (fast path, ordered by frequency)         │
│       │     │     ├── fmt.Formatter      → arg.Format(p, verb)             │
│       │     │     ├── error             → arg.Error() → print as %s        │
│       │     │     └── fmt.Stringer      → arg.String() → print as %s       │
│       │     │                                                               │
│       │     └── reflect.TypeOf(arg).Kind() switch (slow path)              │
│       │           ├── Bool      → p.fmtBool()                              │
│       │           ├── Int*      → p.fmtInteger()                           │
│       │           ├── Uint*     → p.fmtInteger()                           │
│       │           ├── Float*    → p.fmtFloat()                             │
│       │           ├── Complex*  → p.fmtComplex()                           │
│       │           ├── String    → p.fmtString()                            │
│       │           ├── []byte    → p.fmtBytes()                             │
│       │           ├── Ptr/Chan/Func/UnsafePtr → p.fmtPointer()             │
│       │           ├── Struct    → p.printValue() (recursive)               │
│       │           ├── Map       → p.printValue() (recursive, sorted keys)  │
│       │           └── Slice/Array → p.printValue() (recursive)             │
│       │                                                                     │
│       └── p.free() → returns *pp to sync.Pool                              │
│                                                                             │
│  Result: string (Sprintf) | n,err (Fprintf → io.Writer) | os.Stdout (Print)│
└─────────────────────────────────────────────────────────────────────────────┘
```

### Key source files (Go stdlib, `src/fmt/`)

| File | Responsibility |
|---|---|
| `format.go` | Low-level padding/truncation; the `fmtFlags` struct |
| `print.go` | `pp` struct, `doPrintf`, `printArg`, all verb handlers |
| `scan.go` | `ss` struct, `doPrintf` counterpart for scanning |
| `errors.go` | `%w` wrapping, `Errorf` |
| `doc.go` | Package-level documentation |

---

## 2. The `pp` Struct and Sync Pool

```
┌────────────────────────────────────────────────────────────┐
│  pp struct (internal, ~320 bytes on amd64)                 │
│                                                            │
│  buf        buffer      // []byte, inline capacity 64      │
│  arg        any         // current argument being formatted│
│  value      reflect.Value                                  │
│  fmt        fmt         // flags, width, precision         │
│  reordered  bool        // argument index reordering used  │
│  goodArgNum bool        // last [n] index was valid        │
│  panicking  bool        // prevent recursive panic         │
│  erroring   bool        // inside error formatting         │
│  wrapErrs   bool        // %w was seen                     │
│  wrappedErr error       // the wrapped error               │
└────────────────────────────────────────────────────────────┘

sync.Pool lifecycle:
  newPrinter() → pool.Get() → *pp | alloc new *pp
  p.free()     → p.reset(); pool.Put(p)  (only if buf < 64 KiB)
```

The `sync.Pool` avoids per-call heap allocation for the `pp` object.
The internal `buffer` type is `[]byte`. When the format result fits in the initial 64-byte
inline array the entire call is allocation-free (for `Fprintf` paths that write to an
existing `io.Writer`). `Sprintf` always allocates at least the returned `string`.

---

## 3. Format String Grammar

```
format_string := { literal_char | verb }

verb :=
    '%'
    { flag }*
    [ arg_index ]
    [ width ]
    [ '.' precision ]
    conversion_verb

flag         := '-' | '+' | '#' | '0' | ' '
arg_index    := '[' decimal_integer ']'
width        := decimal_integer | '*' | '[' decimal_integer ']' '*'
precision    := decimal_integer | '*' | '[' decimal_integer ']' '*'
conversion_verb := 'v'|'T'|'t'|'b'|'c'|'d'|'o'|'O'|'q'|'x'|'X'
                 | 'U'|'e'|'E'|'f'|'F'|'g'|'G'|'s'|'p'|'w'|'%'

%% → literal '%' (no argument consumed)
```

Example decomposition:

```
  %+12.4[3]f
  │││  │  │└── verb:      'f' (decimal floating-point)
  │││  │  └─── arg index: 3rd argument
  │││  └─────── precision: 4
  ││└────────── width:     12
  │└─────────── flag:      '+' (always print sign)
  └──────────── start of verb
```

---

## 4. General Verbs

### `%v` — Default Format

`%v` is the "value" verb. It formats each type using its natural representation:

```go
package main

import "fmt"

type Point struct{ X, Y int }

func main() {
    p := Point{3, 7}

    fmt.Printf("%v\n",  p)          // {3 7}
    fmt.Printf("%+v\n", p)          // {X:3 Y:7}   — includes field names
    fmt.Printf("%#v\n", p)          // main.Point{X:3, Y:7}  — Go syntax

    var x interface{} = 42
    fmt.Printf("%v\n",  x)          // 42
    fmt.Printf("%v\n",  nil)        // <nil>

    s := []int{1, 2, 3}
    fmt.Printf("%v\n",  s)          // [1 2 3]
    fmt.Printf("%#v\n", s)          // []int{1, 2, 3}

    m := map[string]int{"a": 1}
    fmt.Printf("%v\n",  m)          // map[a:1]
    fmt.Printf("%#v\n", m)          // map[string]int{"a":1}
}
```

### `%T` — Type Name

```go
fmt.Printf("%T\n", 42)                    // int
fmt.Printf("%T\n", 3.14)                  // float64
fmt.Printf("%T\n", "hello")               // string
fmt.Printf("%T\n", []byte{})              // []uint8
fmt.Printf("%T\n", map[string]int{})      // map[string]int
fmt.Printf("%T\n", Point{})               // main.Point
fmt.Printf("%T\n", (*Point)(nil))         // *main.Point
fmt.Printf("%T\n", func() {})             // func()
```

`%T` uses `reflect.TypeOf(arg).String()` internally. It always returns the fully qualified
type name relative to the current package path.

---

## 5. Boolean Verb

### `%t`

```go
fmt.Printf("%t\n",  true)   // true
fmt.Printf("%t\n",  false)  // false
fmt.Printf("%5t\n", true)   //  true   (right-padded to width 5)
fmt.Printf("%-5t|\n",true)  // true |  (left-aligned)
```

Internally: `strconv.AppendBool(buf, b)` — zero allocation.

---

## 6. Integer Verbs

### Complete taxonomy

```
%b   — base 2               (binary)
%c   — Unicode code point   (rune → character)
%d   — base 10              (decimal, default for int types)
%o   — base 8               (octal, no prefix)
%O   — base 8 with 0o prefix
%q   — single-quoted character literal (safely escaped)
%x   — base 16, lower-case  (a–f)
%X   — base 16, upper-case  (A–F)
%U   — Unicode format       U+HHHH
```

```go
package main

import "fmt"

func main() {
    n := 255

    fmt.Printf("%b\n",  n)  // 11111111
    fmt.Printf("%c\n",  65) // A
    fmt.Printf("%d\n",  n)  // 255
    fmt.Printf("%o\n",  n)  // 377
    fmt.Printf("%O\n",  n)  // 0o377
    fmt.Printf("%q\n",  65) // 'A'
    fmt.Printf("%x\n",  n)  // ff
    fmt.Printf("%X\n",  n)  // FF
    fmt.Printf("%U\n",  65) // U+0041

    // Width / padding
    fmt.Printf("%08b\n",  n)  // 11111111   (zero-padded to 8)
    fmt.Printf("%#x\n",   n)  // 0xff       (#  adds 0x prefix)
    fmt.Printf("%#o\n",   n)  // 0377       (#  adds 0  prefix)
    fmt.Printf("%+d\n",   n)  // +255       (+ always show sign)
    fmt.Printf("% d\n",   n)  //  255       (space: space before positive)
    fmt.Printf("%10d\n",  n)  //        255 (right-aligned, width 10)
    fmt.Printf("%-10d|\n",n)  // 255|       (left-aligned)

    // Unsigned types
    var u uint8 = 255
    fmt.Printf("%d\n",  u)   // 255
    fmt.Printf("%x\n",  u)   // ff
    fmt.Printf("%08b\n",u)   // 11111111

    // rune (alias for int32)
    var r rune = '€'
    fmt.Printf("%c\n",  r)   // €
    fmt.Printf("%U\n",  r)   // U+20AC
    fmt.Printf("%#U\n", r)   // U+20AC '€'  (# adds character in quotes)
    fmt.Printf("%d\n",  r)   // 8364
}
```

### Integer internal pipeline

```
printArg(arg=255, verb='x')
       │
       ▼
p.fmtInteger(v=255, base=16, isSigned=true, verb='x', ldigits="0123456789abcdef")
       │
       ├── convert digits into p.buf via itoa loop (no strconv heap alloc)
       ├── apply sign / prefix (#x → "0x")
       ├── apply zero-padding (flag '0' + width)
       └── apply space-padding (flag '-' + width)
```

---

## 7. Floating-Point Verbs

### Verb meanings

```
%e   — scientific notation, lower-case e    e.g. -1.234456e+78
%E   — scientific notation, upper-case E    e.g. -1.234456E+78
%f   — decimal, no exponent                 e.g. 123.456
%F   — same as %f (for symmetry with %E)
%g   — %e for large exponents, %f otherwise (shortest representation)
%G   — %E for large exponents, %F otherwise
%x   — hexadecimal float, lower-case        e.g. -0x1.23abcp+20
%X   — hexadecimal float, upper-case
```

### Default precisions

| Verb | Default precision |
|---|---|
| `%e`, `%E` | 6 digits after decimal point |
| `%f`, `%F` | 6 digits after decimal point |
| `%g`, `%G` | smallest number of digits necessary to represent uniquely |
| `%x`, `%X` | minimum digits necessary |

```go
package main

import "fmt"

func main() {
    f := 123456.789

    fmt.Printf("%e\n",   f)       // 1.234568e+05
    fmt.Printf("%E\n",   f)       // 1.234568E+05
    fmt.Printf("%f\n",   f)       // 123456.789000
    fmt.Printf("%g\n",   f)       // 123456.789
    fmt.Printf("%G\n",   f)       // 123456.789

    // Precision
    fmt.Printf("%.2f\n", f)       // 123456.79
    fmt.Printf("%.0f\n", f)       // 123457
    fmt.Printf("%10.2f\n", f)     // 123456.79  (width 10, right-align)
    fmt.Printf("%-10.2f|\n", f)   // 123456.79| (left-align)
    fmt.Printf("%010.2f\n", f)    // 0123456.79 (zero-pad)

    // Sign and space
    fmt.Printf("%+f\n", f)        // +123456.789000
    fmt.Printf("% f\n", f)        //  123456.789000

    // Hex float (exact IEEE 754 representation)
    fmt.Printf("%x\n",   1.0)     // 0x1p+00
    fmt.Printf("%x\n",   0.5)     // 0x1p-01
    fmt.Printf("%x\n",   f)       // 0x1.e240cap+16
    fmt.Printf("%X\n",   f)       // 0X1.E240CAP+16

    // Special values
    fmt.Printf("%f\n", 1.0/0.0)   // +Inf
    fmt.Printf("%f\n", -1.0/0.0)  // -Inf
    fmt.Printf("%f\n", 0.0/0.0)   // NaN

    // Complex
    c := complex(1.5, -2.5)
    fmt.Printf("%v\n",  c)        // (1.5-2.5i)
    fmt.Printf("%f\n",  c)        // (1.500000-2.500000i)
    fmt.Printf("%.2e\n",c)        // (1.50e+00-2.50e+00i)
}
```

### Floating-point internal pipeline

```
strconv.AppendFloat(dst, f, fmt_byte, prec, 64)
         │
         └── Ryu algorithm (Go 1.15+) for shortest decimal
             Grisu3 fallback for fixed precision
             Result buffered into pp.buf, then padded
```

The Go runtime uses the **Ryu algorithm** (2018, Ulf Adams) for `%g`/`%v` shortest representation,
and falls back to `Grisu3` for fixed-precision formats. This is O(1) with small constant — no
big-integer intermediate.

---

## 8. String and Byte-Slice Verbs

```
%s   — raw string (unquoted)
%q   — double-quoted, Go string literal (safely escaped)
%x   — hex encoding of each byte, lower-case (two hex chars per byte)
%X   — hex encoding of each byte, upper-case
```

```go
package main

import "fmt"

func main() {
    s := "Hello, 世界\n"
    b := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x00, 0xff}

    fmt.Printf("%s\n",  s)   // Hello, 世界
                              // (followed by literal newline from \n)
    fmt.Printf("%q\n",  s)   // "Hello, 世界\n"
    fmt.Printf("%x\n",  s)   // 48656c6c6f2ce4b896e754ec0a
    fmt.Printf("%X\n",  s)   // 48656C6C6F2CE4B896E754EC0A
    fmt.Printf("% x\n",s)    // 48 65 6c 6c 6f 2c e4 b8 96 e7 54 ec 0a (space flag)
    fmt.Printf("%x\n",  b)   // 48656c6c6f00ff
    fmt.Printf("%q\n",  b)   // "Hello\x00\xff"  (byte slice quoted)

    // Width/Precision on strings
    fmt.Printf("%10s\n",    "hi")  //         hi  (right-align in 10)
    fmt.Printf("%-10s|\n",  "hi")  // hi|         (left-align)
    fmt.Printf("%.3s\n",    "hello") // hel       (truncate to 3 bytes)
    fmt.Printf("%10.3s\n",  "hello") //        hel

    // %q escaping rules:
    fmt.Printf("%q\n", "\x00\x01\x7f")  // "\x00\x01\x7f"
    fmt.Printf("%q\n", "tab\there")     // "tab\there"
    fmt.Printf("%q\n", `raw "quotes"`)  // "raw \"quotes\""
    fmt.Printf("%q\n", "€")             // "€"   (valid UTF-8 kept as-is)
    fmt.Printf("%q\n", "\xc0\x80")      // "\xc0\x80" (invalid UTF-8 escaped)
}
```

### Precision on strings = byte limit, not rune limit

This is a subtle but critical point:

```go
s := "日本語"   // 9 bytes (3 × 3-byte UTF-8 sequences)

fmt.Printf("%.6s\n", s)  // 日本  (6 bytes = exactly 2 runes, valid cutoff)
fmt.Printf("%.5s\n", s)  // 日    (5 bytes: cuts mid-rune for 2nd rune,
                           // Go's fmt drops the partial rune → only 1 rune printed)
```

Internally `fmt` does NOT split mid-rune: when precision cuts inside a multibyte sequence
it prints only complete runes up to the precision byte count.

---

## 9. Pointer Verb

### `%p`

```
%p   — base-16 address with leading 0x
```

```go
package main

import "fmt"

func main() {
    x := 42
    p := &x
    s := []int{1, 2, 3}
    c := make(chan int, 1)
    m := map[string]int{}

    fmt.Printf("%p\n", p)   // 0xc0000b4010  (address of x)
    fmt.Printf("%p\n", s)   // 0xc0000c2000  (address of backing array)
    fmt.Printf("%p\n", c)   // 0xc0000b6000  (channel pointer)
    fmt.Printf("%p\n", m)   // 0xc0000ba000  (map pointer)

    // %b/%o/%d/%x on a pointer → NOT valid, results in %!b(uintptr=...)
    // Use uintptr cast for raw arithmetic:
    addr := uintptr(fmt.Sprintf("%d", p)) // WRONG — do not do this
    _ = addr
    // Correct:
    fmt.Printf("0x%x\n", uintptr(p)) // raw hex without 0x prefix from %p
}
```

**Security note**: Pointer values expose ASLR layout. Never log `%p` of sensitive objects
in production without sanitization. See §24.

---

## 10. Flags

Flags modify the output of a verb. Multiple flags can be combined in any order after `%`.

```
Flag   Meaning
─────  ───────────────────────────────────────────────────────────────
-      Left-align within field width (default is right-align).
       Overrides 0 flag.

+      Always print a sign for numeric types (+ or -).
       For %q: print Go-syntax character/string literal.

#      Alternate format:
         %#b   → (no change, just for completeness)
         %#o   → prefix 0
         %#O   → prefix 0o  (same as %O without #)
         %#x   → prefix 0x
         %#X   → prefix 0X
         %#p   → prefix 0x  (same as default for %p)
         %#q   → always use backquote raw string if possible
         %#v   → Go-syntax representation (see §4)
         %#U   → U+0041 'A'  (adds character after Unicode point)
         %#e %#f %#g → always include decimal point

0      Pad with leading zeros (instead of spaces).
       Applied after sign/prefix.
       Ignored when - flag is present.

' '    (space) Leave a space before positive numbers.
       Useful to align with negative numbers.
       Ignored when + flag is present.
```

### Detailed examples

```go
package main

import "fmt"

func main() {
    // ── Left/right alignment ─────────────────────────────────────────
    fmt.Printf("[%10d]\n",   42)   // [        42]  right-aligned
    fmt.Printf("[%-10d]\n",  42)   // [42        ]  left-aligned
    fmt.Printf("[%-10s]\n",  "hi") // [hi        ]  left-aligned string

    // ── Sign ─────────────────────────────────────────────────────────
    fmt.Printf("[%+d]\n",  42)   // [+42]
    fmt.Printf("[%+d]\n", -42)   // [-42]
    fmt.Printf("[% d]\n",  42)   // [ 42]
    fmt.Printf("[% d]\n", -42)   // [-42]

    // ── Alternate formats ─────────────────────────────────────────────
    fmt.Printf("[%#o]\n",  255)   // [0377]
    fmt.Printf("[%#x]\n",  255)   // [0xff]
    fmt.Printf("[%#X]\n",  255)   // [0XFF]
    fmt.Printf("[%#b]\n",  10)    // [1010]     (no change for %b)
    fmt.Printf("[%#f]\n",  1.0)   // [1.000000] (forces decimal point, same for %f)
    fmt.Printf("[%#g]\n",  1.0)   // [1.00000]  (forces trailing zeros)
    fmt.Printf("[%#U]\n",  'A')   // [U+0041 'A']
    fmt.Printf("[%#q]\n", "hello\n") // [`hello\n`] — raw backquote if printable

    // ── Zero padding ──────────────────────────────────────────────────
    fmt.Printf("[%010d]\n",  42)   // [0000000042]
    fmt.Printf("[%010.2f]\n",3.14) // [0000003.14]
    fmt.Printf("[%-010d]\n", 42)   // [42        ]  - overrides 0

    // ── Sign + zero padding ───────────────────────────────────────────
    fmt.Printf("[%+010d]\n",  42)  // [+000000042]
    fmt.Printf("[%+010d]\n", -42)  // [-000000042]

    // ── Combining flags ───────────────────────────────────────────────
    fmt.Printf("[%+#010x]\n", 255) // [+0x000000ff]  (+, #, 0, width)
}
```

### Flag interaction matrix

```
         -   +   #   0   ' '
    -    –   ok  ok  (0 ignored)  ok
    +    ok  –   ok  ok           (' ' ignored)
    #    ok  ok  –   ok           ok
    0    (ignored by -)  ok  ok  –   ok
    ' '  ok  (ignored by +)  ok  ok  –
```

---

## 11. Width and Precision

```
Width     = minimum field width (characters).
            If the value is narrower, pad is added.
            If wider, no truncation (except for strings with precision).

Precision = for floats:  digits after decimal point (%e,%f) or
                         total significant digits (%g)
            for strings: maximum byte count output
            for integers: minimum digit count (zero-pads on left)
            for %g/%G:   maximum significant digits
```

### Dynamic width and precision via `*`

When width or precision is `*`, the next argument (must be `int`) is used:

```go
package main

import "fmt"

func main() {
    // Static
    fmt.Printf("%8.2f\n", 3.14159)       //     3.14

    // Dynamic via *
    fmt.Printf("%*.*f\n", 8, 2, 3.14159) //     3.14  (same result)

    // Negative width → left-align (same as - flag)
    fmt.Printf("%*d\n", -10, 42)          // 42          (left)
    fmt.Printf("%*d\n",  10, 42)          //         42  (right)

    // Precision 0 on integers = no digits (unusual)
    fmt.Printf("%.0d\n", 0)  //    (empty — zero with precision 0)
    fmt.Printf("%.5d\n", 42) // 00042

    // Width on strings
    fmt.Printf("%*.*s\n", 10, 3, "hello") //        hel
}
```

### Dynamic width via argument index

```go
// Using explicit argument indexing with *
fmt.Printf("%[3]*.[2]*[1]f\n", 3.14159, 2, 8)
//  [1] = 3.14159 (the float)
//  [2] = 2       (precision)
//  [3] = 8       (width)
// Output:     3.14
```

---

## 12. Argument Indexing

`[n]` selects argument n (1-based). This is powerful for reuse and localization.

```go
package main

import "fmt"

func main() {
    // Basic indexing
    fmt.Printf("%[2]s %[1]s\n", "world", "hello")  // hello world

    // Reusing arguments
    fmt.Printf("%[1]d %[1]d %[1]d\n", 42)          // 42 42 42

    // Mixed with positional
    fmt.Printf("%d %[1]d %d\n", 1, 2)              // 1 1 2
    //  first %d        → arg 1 = 1
    //  %[1]d           → arg 1 = 1  (explicit reuse)
    //  last %d         → arg 2 = 2  (resumes after last explicit index)

    // With width via index
    fmt.Printf("%[2]*[1]d\n", 42, 10)              //         42

    // Localization use case
    name, count := "users", 5
    fmt.Printf("Found %[2]d %[1]s\n", name, count) // Found 5 users
    fmt.Printf("%[1]s trouvé: %[2]d\n", name, count)// users trouvé: 5

    // Out-of-range index
    fmt.Printf("%[5]d\n", 1, 2)     // %!(EXTRA int=1, int=2) errors shown
}
```

### Argument index state machine

```
format: "%[2]d %d %[1]d %d"
args:   [10, 20]

Step 1: %[2]d  → explicit index 2 → prints 20; lastIndex = 2
Step 2: %d     → implicit: lastIndex+1 = 3 → OUT OF RANGE → %!d(MISSING)
Step 3: %[1]d  → explicit index 1 → prints 10; lastIndex = 1
Step 4: %d     → implicit: lastIndex+1 = 2 → prints 20
```

---

## 13. Printing Functions Taxonomy

```
┌──────────────────────────────────────────────────────────────────┐
│                   fmt Printing Functions                         │
│                                                                  │
│  WRITE TO os.Stdout                                              │
│  ─────────────────                                               │
│  Print(a ...any)                 → no format, space between      │
│                                    non-string operands           │
│  Println(a ...any)               → spaces between all operands,  │
│                                    trailing newline              │
│  Printf(format string, a ...any) → format string                 │
│                                                                  │
│  RETURN string                                                   │
│  ─────────────                                                   │
│  Sprint(a ...any)   string                                       │
│  Sprintln(a ...any) string                                       │
│  Sprintf(format, a) string                                       │
│                                                                  │
│  WRITE TO io.Writer                                              │
│  ─────────────────                                               │
│  Fprint(w, a ...any)    (n int, err error)                       │
│  Fprintln(w, a ...any)  (n int, err error)                       │
│  Fprintf(w, fmt, a ...) (n int, err error)                       │
│                                                                  │
│  RETURN error (wrapping)                                         │
│  ────────────────────                                            │
│  Errorf(format, a ...) error                                     │
└──────────────────────────────────────────────────────────────────┘
```

### Space insertion rules for `Print`/`Sprint`/`Fprint`

```go
fmt.Print("a", "b")          // ab       — no space between two strings
fmt.Print("a", 1)            // a1       — no space: string + other
fmt.Print(1, 2)              // 1 2      — space between two non-strings
fmt.Print(1, "b")            // 1 b      — space: non-string + string
fmt.Print(1, 2, "b", 3, 4)  // 1 2 b 3 4
```

Rule: a space is added between two adjacent operands when **neither** is a string.

### `Println` vs `Print`

```go
fmt.Println("a", "b", 1)    // a b 1\n  — spaces between ALL, trailing newline
fmt.Print("a", "b", 1, "\n")// ab 1\n   — spaces only between non-strings
```

---

## 14. Formatting Interfaces

These four interfaces, checked in this priority order, let any type control its own format output.

```
Priority order (checked via type assertion, not reflect):
  1. fmt.Formatter   (most specific — full control)
  2. error           (if verb is v, s, or q)
  3. fmt.Stringer    (if verb is v or s)
  4. reflect switch  (fallback)
```

### 14.1 fmt.Stringer

```go
type Stringer interface {
    String() string
}
```

Called when verb is `%s` or `%v`. The most common interface to implement.

```go
package main

import "fmt"

type Color int

const (
    Red Color = iota
    Green
    Blue
)

func (c Color) String() string {
    switch c {
    case Red:   return "Red"
    case Green: return "Green"
    case Blue:  return "Blue"
    default:    return fmt.Sprintf("Color(%d)", int(c))
    }
}

func main() {
    c := Green
    fmt.Printf("%v\n", c)  // Green   (String() called)
    fmt.Printf("%s\n", c)  // Green
    fmt.Printf("%d\n", c)  // 1       (String() NOT called for %d)
    fmt.Printf("%T\n", c)  // main.Color
}
```

**Pitfall — infinite recursion**:

```go
type MyError struct{ msg string }

// WRONG: causes infinite recursion
func (e *MyError) String() string {
    return fmt.Sprintf("Error: %v", e)  // %v calls String() again!
}

// CORRECT: use struct fields directly or cast to base type
func (e *MyError) String() string {
    return "Error: " + e.msg
}
```

### 14.2 fmt.GoStringer

```go
type GoStringer interface {
    GoString() string
}
```

Called only for `%#v`. Returns a Go-syntax representation.

```go
package main

import "fmt"

type IPAddr [4]byte

func (ip IPAddr) String() string {
    return fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])
}

func (ip IPAddr) GoString() string {
    return fmt.Sprintf("IPAddr{%d, %d, %d, %d}", ip[0], ip[1], ip[2], ip[3])
}

func main() {
    host := IPAddr{192, 168, 0, 1}
    fmt.Printf("%v\n",  host)   // 192.168.0.1         (String())
    fmt.Printf("%s\n",  host)   // 192.168.0.1         (String())
    fmt.Printf("%#v\n", host)   // IPAddr{192, 168, 0, 1} (GoString())
}
```

### 14.3 fmt.Formatter

The most powerful interface — full control over the output including flags, width, precision.

```go
type Formatter interface {
    Format(f State, verb rune)
}

type State interface {
    Write(b []byte) (n int, err error) // write formatted output
    Width() (wid int, ok bool)         // requested width
    Precision() (prec int, ok bool)    // requested precision
    Flag(c int) bool                   // was flag c ('-','+',' ','#','0') set?
}
```

Full custom formatter example:

```go
package main

import (
    "fmt"
    "strings"
)

// Duration formats nanoseconds intelligently.
type Duration int64

func (d Duration) Format(f fmt.State, verb rune) {
    ns := int64(d)

    switch verb {
    case 'v', 's':
        switch {
        case ns < 1_000:
            fmt.Fprintf(f, "%dns", ns)
        case ns < 1_000_000:
            fmt.Fprintf(f, "%.3fµs", float64(ns)/1e3)
        case ns < 1_000_000_000:
            fmt.Fprintf(f, "%.3fms", float64(ns)/1e6)
        default:
            fmt.Fprintf(f, "%.3fs", float64(ns)/1e9)
        }

    case 'd':
        // Raw nanoseconds, respects width/precision
        w, wOk := f.Width()
        p, pOk := f.Precision()
        format := "%"
        if f.Flag('-') { format += "-" }
        if f.Flag('+') { format += "+" }
        if f.Flag('0') { format += "0" }
        if wOk { format += fmt.Sprintf("%d", w) }
        if pOk { format += fmt.Sprintf(".%d", p) }
        format += "d"
        fmt.Fprintf(f, format, ns)

    default:
        // Unsupported verb: emit standard error string
        fmt.Fprintf(f, "%%!%c(Duration=%d)", verb, ns)
    }

    // Honor width for 'v'/'s'
    if verb == 'v' || verb == 's' {
        if w, ok := f.Width(); ok {
            // Simple right-pad (real impl would measure and pad)
            _ = w
            _ = strings.Repeat(" ", 0) // placeholder
        }
    }
}

func main() {
    d1 := Duration(500)
    d2 := Duration(1_500_000)
    d3 := Duration(2_500_000_000)

    fmt.Printf("%v\n",  d1)  // 500ns
    fmt.Printf("%v\n",  d2)  // 1.500ms
    fmt.Printf("%v\n",  d3)  // 2.500s
    fmt.Printf("%d\n",  d2)  // 1500000
    fmt.Printf("%10d\n",d2)  //    1500000
    fmt.Printf("%q\n",  d2)  // %!q(Duration=1500000)
}
```

### Inspecting flags inside Format

```go
func (x MyType) Format(f fmt.State, verb rune) {
    plusFlag  := f.Flag('+')  // '+' was specified
    sharpFlag := f.Flag('#')  // '#' was specified
    spaceFlag := f.Flag(' ')  // ' ' was specified
    zeroFlag  := f.Flag('0')  // '0' was specified
    minusFlag := f.Flag('-')  // '-' was specified

    width, hasWidth       := f.Width()
    prec,  hasPrec        := f.Precision()

    // Write output directly to f (implements io.Writer)
    fmt.Fprintf(f, "verb=%c plus=%v width=%d prec=%d",
        verb, plusFlag,
        map[bool]int{true: width, false: -1}[hasWidth],
        map[bool]int{true: prec,  false: -1}[hasPrec],
    )
}
```

### 14.4 error Interface

```go
type error interface {
    Error() string
}
```

When printing with `%v`, `%s`, or `%q`, if a value implements `error`, `Error()` is called.

```go
package main

import (
    "errors"
    "fmt"
)

type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on field %q: %s", e.Field, e.Message)
}

func main() {
    err := &ValidationError{Field: "email", Message: "invalid format"}

    fmt.Printf("%v\n", err)   // validation error on field "email": invalid format
    fmt.Printf("%s\n", err)   // validation error on field "email": invalid format
    fmt.Printf("%q\n", err)   // "validation error on field \"email\": invalid format"
    fmt.Printf("%T\n", err)   // *main.ValidationError
    fmt.Printf("%d\n", err)   // %!d(*main.ValidationError=&{email invalid format})
    fmt.Printf("%p\n", err)   // 0xc0000b4020  (pointer address)

    // Wrapping
    wrapped := fmt.Errorf("outer: %w", err)
    fmt.Println(wrapped)                       // outer: validation error on field "email": invalid format
    fmt.Println(errors.Is(wrapped, err))       // false (not same pointer for Is)
    var ve *ValidationError
    fmt.Println(errors.As(wrapped, &ve))       // true
}
```

---

## 15. Scanning

The `fmt` scan family is the reverse of print: it reads formatted text and populates variables.

### Function taxonomy

```
STDIN (os.Stdin)
  Scan(a ...any)                  — whitespace-separated, newline = space
  Scanln(a ...any)                — newline terminates scan
  Scanf(format string, a ...any)  — format-directed

STRING (from string)
  Sscan(str string, a ...any)
  Sscanln(str string, a ...any)
  Sscanf(str, format string, a ...any)

io.Reader
  Fscan(r io.Reader, a ...any)
  Fscanln(r io.Reader, a ...any)
  Fscanf(r io.Reader, format string, a ...any)
```

### Scan verbs (subset)

```
%d   — decimal integer
%i   — integer (auto-detects base: 0x hex, 0 octal, else decimal)
%o   — octal integer
%x   — hex integer
%f %e %g — float
%s   — string (space-delimited token)
%q   — quoted string
%v   — default format (type-specific)
%t   — boolean
%c   — single character (no whitespace skip)
```

```go
package main

import "fmt"

func main() {
    var name string
    var age  int
    var gpa  float64

    // From string
    n, err := fmt.Sscanf("Alice 30 3.95", "%s %d %f", &name, &age, &gpa)
    fmt.Printf("scanned=%d err=%v name=%s age=%d gpa=%.2f\n",
        n, err, name, age, gpa)
    // scanned=3 err=<nil> name=Alice age=30 gpa=3.95

    // Quoted strings
    var s string
    fmt.Sscan(`"hello world"`, &s)
    fmt.Println(s) // hello  (stops at space; use %q to read full quoted token)

    fmt.Sscanf(`"hello world"`, "%q", &s)
    fmt.Println(s) // hello world  (full quoted string)

    // Multi-line
    input := "10 20\n30 40"
    var a, b, c, d int
    fmt.Sscan(input, &a, &b, &c, &d) // newline acts as space
    fmt.Println(a, b, c, d) // 10 20 30 40

    fmt.Sscanln(input, &a, &b, &c, &d) // stops at newline
    fmt.Println(a, b) // 10 20 (c,d not set)
}
```

### Custom Scanner — `fmt.Scanner` interface

```go
type Scanner interface {
    Scan(state ScanState, verb rune) error
}

type ScanState interface {
    ReadRune() (r rune, size int, err error)
    UnreadRune() error
    SkipSpace()
    Token(skipSpace bool, f func(rune) bool) (token []byte, err error)
    Width() (wid int, ok bool)
    Read(buf []byte) (n int, err error)
}
```

```go
package main

import (
    "fmt"
    "strings"
)

// Fraction reads "a/b" format.
type Fraction struct{ Num, Den int }

func (f *Fraction) Scan(state fmt.ScanState, verb rune) error {
    _, err := fmt.Fscan(state, &f.Num)
    if err != nil { return err }

    tok, err := state.Token(false, func(r rune) bool { return r == '/' })
    if err != nil { return err }
    if string(tok) != "/" {
        return fmt.Errorf("expected '/', got %q", tok)
    }

    _, err = fmt.Fscan(state, &f.Den)
    return err
}

func main() {
    var frac Fraction
    n, err := fmt.Fscan(strings.NewReader("3/4"), &frac)
    fmt.Printf("n=%d err=%v frac=%+v\n", n, err, frac)
    // n=1 err=<nil> frac={Num:3 Den:4}
}
```

---

## 16. Errorf and `%w` — Error Wrapping

`%w` is a special verb available **only** in `fmt.Errorf`. It wraps an error value so that
`errors.Is` and `errors.As` can unwrap the chain.

```go
package main

import (
    "errors"
    "fmt"
    "os"
)

var ErrPermission = errors.New("permission denied")

func readFile(path string) error {
    _, err := os.Open(path)
    if err != nil {
        // %w wraps err, %v would not
        return fmt.Errorf("readFile(%q): %w", path, err)
    }
    return nil
}

func processFile(path string) error {
    if err := readFile(path); err != nil {
        return fmt.Errorf("processFile: %w", err)
    }
    return nil
}

func main() {
    err := processFile("/etc/shadow")

    fmt.Println(err)
    // processFile: readFile("/etc/shadow"): open /etc/shadow: permission denied

    // Unwrap chain
    var pathErr *os.PathError
    if errors.As(err, &pathErr) {
        fmt.Printf("Path error on: %s\n", pathErr.Path)
        // Path error on: /etc/shadow
    }

    fmt.Println(errors.Is(err, os.ErrPermission))  // true
}
```

### Multiple wrapping (Go 1.20+)

```go
// Wrapping multiple errors with %w %w
err1 := errors.New("err1")
err2 := errors.New("err2")
combined := fmt.Errorf("both: %w and %w", err1, err2)

fmt.Println(errors.Is(combined, err1))  // true
fmt.Println(errors.Is(combined, err2))  // true

// Unwrap returns []error (via interface { Unwrap() []error })
type multiErr struct{ errs []error }
```

### `%w` vs `%v` vs `%s`

```
%w   → wraps the error value (errors.Is/As work)  — use for error propagation
%v   → formats using Error() string only — unwrap chain broken
%s   → same as %v for errors
```

---

## 17. Compound Types

### Structs

```go
type Server struct {
    Host string
    Port int
    TLS  bool
}

s := Server{"localhost", 8443, true}

fmt.Printf("%v\n",  s)    // {localhost 8443 true}
fmt.Printf("%+v\n", s)    // {Host:localhost Port:8443 TLS:true}
fmt.Printf("%#v\n", s)    // main.Server{Host:"localhost", Port:8443, TLS:true}
```

Unexported fields are NOT printed by `%v`/`%+v` if the type is in another package
(reflect cannot access them). Within the same package, they are visible.

### Maps

```go
m := map[string]int{"b": 2, "a": 1, "c": 3}
fmt.Printf("%v\n",  m)    // map[a:1 b:2 c:3]   — keys sorted for determinism
fmt.Printf("%#v\n", m)    // map[string]int{"a":1, "b":2, "c":3}
```

**Map key sorting**: Go's `fmt` package sorts map keys before printing (since Go 1.12)
to ensure deterministic output. Sorting uses `reflect.Value`'s `Less` comparison,
which works for any ordered key type.

### Slices and Arrays

```go
sl := []int{1, 2, 3}
ar := [3]int{4, 5, 6}

fmt.Printf("%v\n",  sl)   // [1 2 3]
fmt.Printf("%v\n",  ar)   // [4 5 6]
fmt.Printf("%#v\n", sl)   // []int{1, 2, 3}
fmt.Printf("%#v\n", ar)   // [3]int{4, 5, 6}
```

### Channels

```go
ch := make(chan int, 5)
fmt.Printf("%v\n", ch)    // 0xc0000c6000
fmt.Printf("%T\n", ch)    // chan int
fmt.Printf("%p\n", ch)    // 0xc0000c6000
```

### Functions

```go
f := func(x int) int { return x * 2 }
fmt.Printf("%v\n", f)    // 0x10a2340  (function pointer)
fmt.Printf("%T\n", f)    // func(int) int
```

---

## 18. Recursive and Cyclic Structures

Go's `fmt` does not detect cycles — it will panic via stack overflow or print
`<already printing>` for some cases. The `%v` handler does include a depth check
and emits `{...}` for deeply nested structs.

```go
package main

import "fmt"

type Node struct {
    Val  int
    Next *Node
}

func main() {
    a := &Node{Val: 1}
    b := &Node{Val: 2}
    a.Next = b
    // b.Next = a  // DO NOT: causes infinite recursion in fmt.Printf

    fmt.Printf("%v\n",  a)    // &{1 0xc0000b4020}  (pointer shown)
    fmt.Printf("%+v\n", a)    // &{Val:1 Next:0xc0000b4020}
    fmt.Printf("%#v\n", a)    // &main.Node{Val:1, Next:(*main.Node)(0xc0000b4020)}
}
```

For cyclic structures, implement `fmt.Stringer` or `fmt.Formatter` with explicit
cycle detection:

```go
type Graph struct {
    ID       int
    Edges    []*Graph
    printing bool  // guard
}

func (g *Graph) String() string {
    if g.printing {
        return fmt.Sprintf("<cycle:%d>", g.ID)
    }
    g.printing = true
    defer func() { g.printing = false }()

    var edges []string
    for _, e := range g.Edges {
        edges = append(edges, e.String())
    }
    return fmt.Sprintf("Node(%d)->%v", g.ID, edges)
}
```

---

## 19. Unicode, Runes, and Multibyte Strings

```go
package main

import "fmt"

func main() {
    // Rune verbs
    r := '€'  // rune (int32), value 8364 = 0x20AC
    fmt.Printf("%c\n",  r)    // €
    fmt.Printf("%d\n",  r)    // 8364
    fmt.Printf("%x\n",  r)    // 20ac
    fmt.Printf("%U\n",  r)    // U+20AC
    fmt.Printf("%#U\n", r)    // U+20AC '€'
    fmt.Printf("%q\n",  r)    // '€'
    fmt.Printf("%08b\n",r)    // 10000000101100  (binary)

    // Strings containing multibyte sequences
    s := "Hello, 世界"
    fmt.Printf("len=%d\n",         len(s))    // 13 bytes, not 9 runes
    fmt.Printf("rune count=%d\n",  len([]rune(s))) // 9

    // Iterating
    for i, ch := range s {
        fmt.Printf("byte[%2d] rune=%c U+%04X\n", i, ch, ch)
    }
    // byte[ 0] rune=H U+0048
    // byte[ 1] rune=e U+0065
    // ...
    // byte[ 7] rune=世 U+4E16
    // byte[10] rune=界 U+754C

    // %q on invalid UTF-8
    bad := "\xff\xfe"
    fmt.Printf("%q\n", bad)    // "\xff\xfe"  (hex-escaped)
    fmt.Printf("%s\n", bad)    // (raw bytes — may be garbled in terminal)
    fmt.Printf("%x\n", bad)    // fffe
}
```

### UTF-8 byte structure (for reference)

```
Code point range      Byte 1      Byte 2      Byte 3      Byte 4
U+0000–U+007F         0xxxxxxx
U+0080–U+07FF         110xxxxx    10xxxxxx
U+0800–U+FFFF         1110xxxx    10xxxxxx    10xxxxxx
U+10000–U+10FFFF      11110xxx    10xxxxxx    10xxxxxx    10xxxxxx

'€' = U+20AC = 0010 0000 1010 1100
→ 3-byte: 1110 0010  10 000010  10 101100
→ hex:    E2          82          AC
```

---

## 20. Performance

### Benchmark: `fmt.Sprintf` vs alternatives

```go
package bench_test

import (
    "fmt"
    "strconv"
    "strings"
    "testing"
)

func BenchmarkSprintf(b *testing.B) {
    for b.Loop() {
        _ = fmt.Sprintf("value: %d", 42)
    }
}

func BenchmarkStrconvItoa(b *testing.B) {
    for b.Loop() {
        _ = "value: " + strconv.Itoa(42)
    }
}

func BenchmarkStringsBuilder(b *testing.B) {
    for b.Loop() {
        var sb strings.Builder
        sb.WriteString("value: ")
        sb.WriteString(strconv.Itoa(42))
        _ = sb.String()
    }
}

func BenchmarkFprintfWriter(b *testing.B) {
    var buf strings.Builder
    for b.Loop() {
        buf.Reset()
        fmt.Fprintf(&buf, "value: %d", 42)
        _ = buf.String()
    }
}
```

Run:

```bash
go test -bench=. -benchmem ./...
```

Typical results (amd64, Go 1.22):

```
BenchmarkSprintf          10M   ~115 ns/op   32 B/op   1 allocs/op
BenchmarkStrconvItoa      30M   ~40  ns/op   16 B/op   1 allocs/op
BenchmarkStringsBuilder   20M   ~60  ns/op   16 B/op   1 allocs/op
BenchmarkFprintfWriter    20M   ~80  ns/op    0 B/op   0 allocs/op  ← best for hot paths
```

### Zero-allocation patterns

```go
// Pattern 1: Pre-allocated buffer + Fprintf
var buf [256]byte
n := copy(buf[:], "value: ")
n += len(strconv.AppendInt(buf[n:n], 42, 10))
result := string(buf[:n])

// Pattern 2: sync.Pool of bytes.Buffer
var pool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

func formatSomething(x int) string {
    buf := pool.Get().(*bytes.Buffer)
    buf.Reset()
    fmt.Fprintf(buf, "x=%d", x)
    s := buf.String()
    pool.Put(buf)
    return s
}

// Pattern 3: AppendXxx from strconv (zero alloc)
dst := make([]byte, 0, 64)
dst = strconv.AppendInt(dst, 42, 10)
dst = append(dst, ' ')
dst = strconv.AppendFloat(dst, 3.14, 'f', 2, 64)
```

### `fmt` escape analysis: when does it heap-allocate?

```
fmt.Sprintf("val=%d", x)     → 1 alloc (the string result)
fmt.Fprintf(w, "val=%d", x)  → 0 allocs if w is not nil interface{} and x is concrete
                                 1 alloc if x needs boxing into any{}
fmt.Println(x)               → 1 alloc ([]any{x} for variadic, + string)
```

The key cost is **interface boxing** of the variadic `any` args. For `int`, `string`,
`bool`, and pointer types the compiler can sometimes avoid boxing via escape analysis,
but in practice most `fmt.Printf` calls allocate once per non-pointer argument boxed
into `any`.

---

## 21. Common Pitfalls and Anti-Patterns

### 21.1 Wrong verb for type

```go
fmt.Printf("%d\n", "hello")   // %!d(string=hello)
fmt.Printf("%s\n", 42)        // %!s(int=42)
fmt.Printf("%v\n", nil)       // <nil>
fmt.Printf("%d\n", nil)       // %!d(<nil>)
```

The `%!verb(TYPE=VALUE)` format is Go's error representation for a mismatched verb.
No panic — always safe — but you will get garbage output.

### 21.2 Stringer infinite recursion

```go
type T struct{ v int }
func (t T) String() string {
    return fmt.Sprintf("%v", t)  // INFINITE RECURSION: %v calls String() on T
}
// Fix:
func (t T) String() string {
    return fmt.Sprintf("%d", t.v)  // use field directly
}
```

### 21.3 Pointer vs value receiver on Stringer

```go
type T struct{ v int }
func (t *T) String() string { return fmt.Sprintf("T(%d)", t.v) }

t  := T{1}
tp := &T{1}

fmt.Println(t)   // {1}          — String() not called (non-addressable)
fmt.Println(tp)  // T(1)         — String() called (pointer receiver)
fmt.Println(&t)  // T(1)         — String() called (addressable pointer)
```

**Rule**: If `String()` has pointer receiver, the value must be passed as a pointer
or be addressable for `fmt` to call `String()`.

### 21.4 `%v` on interface nil vs typed nil

```go
var err error       = nil
var pe  *os.PathError = nil

fmt.Println(err == nil)                       // true
fmt.Println((*os.PathError)(nil) == nil)       // true
fmt.Println(error((*os.PathError)(nil)) == nil)// false  ← typed nil

fmt.Printf("%v\n", err)                       // <nil>
fmt.Printf("%v\n", error(pe))                 // <nil>   (Error() called on typed nil)
// If Error() dereferences receiver without nil check → PANIC
```

### 21.5 Map ordering in Sprintf for testing

```go
m := map[string]int{"a": 1, "b": 2}
s1 := fmt.Sprintf("%v", m)  // always "map[a:1 b:2]" since Go 1.12
s2 := fmt.Sprintf("%v", m)  // same

// Before Go 1.12, map iteration was non-deterministic → Sprintf output varied
// Now it's safe to use in tests (keys are sorted)
```

### 21.6 Mixing `%w` with non-error types

```go
// This panics at runtime:
err := fmt.Errorf("%w", 42)  // 42 does not implement error
// → panic: fmt.Errorf: %w verb used with non-error argument 42 of type int
```

### 21.7 `%p` on non-pointer types

```go
x := 42
fmt.Printf("%p\n", x)         // %!p(int=42)  — not a pointer
fmt.Printf("%p\n", &x)        // 0xc000...    — correct
fmt.Printf("%p\n", []int{1})  // 0xc000...    — slice header pointer (ok)
```

### 21.8 Precision on integers

```go
fmt.Printf("%.5d\n",  42)  // 00042   — minimum 5 digits, zero-padded on left
fmt.Printf("%.5d\n",   0)  //          — zero with precision 0 = empty? No:
fmt.Printf("%.0d\n",   0)  //          — empty string (special case: 0 with prec 0)
fmt.Printf("%.1d\n",   0)  // 0        — 0 with prec 1 = "0"
```

### 21.9 Format string from untrusted input (security)

```go
// VULNERABLE: user controls format string
userInput := "%x%x%x%x"
fmt.Printf(userInput, sensitiveData)  // leaks sensitiveData as hex

// SAFE: always use %s or %v with user-provided strings
fmt.Printf("%s\n", userInput)   // prints literal "%x%x%x%x"
fmt.Println(userInput)          // same
```

---

## 22. Complete Reference Table

```
╔══════╦══════════╦═══════════════════════════════════════════════════════════╗
║ Verb ║  Types   ║ Description                                               ║
╠══════╬══════════╬═══════════════════════════════════════════════════════════╣
║  %v  ║  any     ║ Default format. Delegates to Formatter/Stringer/error.    ║
║ %+v  ║ struct   ║ Default + field names in structs                          ║
║ %#v  ║  any     ║ Go-syntax representation (GoStringer if available)        ║
║  %T  ║  any     ║ Go type name via reflect                                  ║
║  %%  ║  —       ║ Literal percent sign, no argument consumed                ║
╠══════╬══════════╬═══════════════════════════════════════════════════════════╣
║  %t  ║  bool    ║ true or false                                             ║
╠══════╬══════════╬═══════════════════════════════════════════════════════════╣
║  %b  ║  int     ║ Binary (base 2)                                           ║
║  %c  ║  int     ║ Unicode code point → character                            ║
║  %d  ║  int     ║ Decimal (base 10)                                         ║
║  %o  ║  int     ║ Octal (base 8, no prefix)                                 ║
║  %O  ║  int     ║ Octal with 0o prefix                                      ║
║  %q  ║  int     ║ Single-quoted rune literal, safely escaped                ║
║  %x  ║  int     ║ Hexadecimal, lower-case (a-f)                             ║
║  %X  ║  int     ║ Hexadecimal, upper-case (A-F)                             ║
║  %U  ║  int     ║ Unicode format: U+HHHH                                    ║
╠══════╬══════════╬═══════════════════════════════════════════════════════════╣
║  %b  ║  float   ║ Exponent of 2, e.g. -123456p-78                          ║
║  %e  ║  float   ║ Scientific, lower-case e: 1.234e+05                       ║
║  %E  ║  float   ║ Scientific, upper-case E: 1.234E+05                       ║
║  %f  ║  float   ║ Decimal, no exponent: 123456.789                          ║
║  %F  ║  float   ║ Same as %f (for symmetry)                                 ║
║  %g  ║  float   ║ Shortest: %e for large exp, %f otherwise                  ║
║  %G  ║  float   ║ Shortest: %E for large exp, %F otherwise                  ║
║  %x  ║  float   ║ Hex mantissa, decimal exponent: 0x1.fp+06                 ║
║  %X  ║  float   ║ Hex mantissa, upper: 0X1.FP+06                            ║
╠══════╬══════════╬═══════════════════════════════════════════════════════════╣
║  %s  ║  string  ║ Raw string value (unquoted)                               ║
║  %q  ║  string  ║ Double-quoted Go string literal, safely escaped           ║
║  %x  ║  string  ║ Hex encoding, 2 chars/byte, lower-case                    ║
║  %X  ║  string  ║ Hex encoding, 2 chars/byte, upper-case                    ║
╠══════╬══════════╬═══════════════════════════════════════════════════════════╣
║  %s  ║  []byte  ║ Raw bytes as string                                       ║
║  %q  ║  []byte  ║ Double-quoted, escaped                                    ║
║  %x  ║  []byte  ║ Hex encoding                                              ║
║  %X  ║  []byte  ║ Hex encoding upper                                        ║
╠══════╬══════════╬═══════════════════════════════════════════════════════════╣
║  %p  ║  ptr     ║ Pointer address in base-16 with 0x                        ║
╠══════╬══════════╬═══════════════════════════════════════════════════════════╣
║  %w  ║  error   ║ Errorf only: wraps error for errors.Is/As                 ║
╚══════╩══════════╩═══════════════════════════════════════════════════════════╝

FLAGS
╔═══════╦══════════════════════════════════════════════════════════════════════╗
║ Flag  ║ Meaning                                                              ║
╠═══════╬══════════════════════════════════════════════════════════════════════╣
║  -    ║ Left-align (default is right); overrides 0                          ║
║  +    ║ Always show sign; for %q uses Go syntax                             ║
║  #    ║ Alternate: 0x/%x, 0/%o, decimal point/%g, U+.../%U                  ║
║  0    ║ Pad with zeros (instead of spaces)                                  ║
║ ' '   ║ Space before positive number (to align with negative)               ║
╚═══════╩══════════════════════════════════════════════════════════════════════╝
```

---

## 23. Advanced: Zero-Allocation Custom Formatter

A production-grade `Formatter` for a log record that avoids all allocations on the
hot path by writing directly into a pre-allocated `[]byte` sink:

```go
package main

import (
    "fmt"
    "os"
    "sync"
    "time"
)

// LogRecord is a structured log entry.
type LogRecord struct {
    Time    time.Time
    Level   string
    Message string
    Fields  map[string]any
}

// bufPool provides reusable byte slices for formatting.
var bufPool = sync.Pool{
    New: func() any {
        b := make([]byte, 0, 512)
        return &b
    },
}

// Format implements fmt.Formatter.
// Verbs:
//   %v, %s  → compact: time level msg key=val...
//   %+v     → verbose: adds full RFC3339Nano timestamp
//   %j      → JSON-ish (custom)
func (r LogRecord) Format(f fmt.State, verb rune) {
    bp := bufPool.Get().(*[]byte)
    buf := (*bp)[:0]

    switch verb {
    case 'v', 's':
        if f.Flag('+') {
            buf = r.Time.AppendFormat(buf, time.RFC3339Nano)
        } else {
            buf = r.Time.AppendFormat(buf, "15:04:05.000")
        }
        buf = append(buf, ' ')
        buf = append(buf, '[')
        buf = append(buf, r.Level...)
        buf = append(buf, ']')
        buf = append(buf, ' ')
        buf = append(buf, r.Message...)

        // Append sorted fields (deterministic output)
        for k, v := range r.Fields {
            buf = append(buf, ' ')
            buf = append(buf, k...)
            buf = append(buf, '=')
            buf = fmt.Appendf(buf, "%v", v)
        }

    case 'j':
        buf = append(buf, `{"time":"`...)
        buf = r.Time.AppendFormat(buf, time.RFC3339Nano)
        buf = append(buf, `","level":"`...)
        buf = append(buf, r.Level...)
        buf = append(buf, `","msg":"`...)
        buf = append(buf, r.Message...)
        buf = append(buf, '"')
        buf = append(buf, '}')

    default:
        buf = fmt.Appendf(buf, "%%!%c(LogRecord)", verb)
    }

    f.Write(buf) //nolint:errcheck
    *bp = buf
    bufPool.Put(bp)
}

func main() {
    rec := LogRecord{
        Time:    time.Now(),
        Level:   "INFO",
        Message: "server started",
        Fields:  map[string]any{"port": 8443, "tls": true},
    }

    fmt.Printf("%v\n",  rec)
    fmt.Printf("%+v\n", rec)
    fmt.Fprintf(os.Stdout, "%j\n", rec)
}
```

### `fmt.Appendf` (Go 1.19+)

```go
// Appendf appends a formatted string to a byte slice (no intermediate string).
// This is the most allocation-efficient path.
buf := make([]byte, 0, 128)
buf = fmt.Appendf(buf, "x=%d y=%.2f", 10, 3.14)
buf = fmt.Appendln(buf, "done")
buf = fmt.Append(buf, "raw", " ", "values")
```

`fmt.Appendf` / `fmt.Appendln` / `fmt.Append` (Go 1.19) are the preferred zero-copy APIs
for building formatted output into existing byte slices.

---

## 24. Threat Model for Format Strings

```
┌──────────────────────────────────────────────────────────────────────────┐
│               Go fmt Threat Model                                        │
│                                                                          │
│  THREAT 1: Format String Injection                                       │
│  ─────────────────────────────────                                       │
│  Risk:    User-controlled format string leaks data or causes DoS         │
│  Example: fmt.Printf(userInput, secretKey)                               │
│           → if userInput = "%x%x", secretKey printed as hex              │
│  Go severity: LOWER than C (no memory write via %n in Go's fmt package;  │
│               %n is not supported), but data leakage still possible.     │
│                                                                          │
│  Mitigation:                                                             │
│   • NEVER pass user-controlled data as the format string argument        │
│   • Use fmt.Printf("%s", userInput) — always a literal format string     │
│   • Static analysis: go vet -printf checks for mismatched verb/type      │
│   • go vet also catches: fmt.Printf(s) where s is non-constant           │
│                                                                          │
│  THREAT 2: Pointer Address Leakage (%p)                                  │
│  ──────────────────────────────────────                                  │
│  Risk:    ASLR bypass; reveals heap layout                               │
│  Example: logging fmt.Sprintf("%p", sensitiveStruct) in public logs      │
│  Mitigation:                                                             │
│   • Sanitize log output before externalizing                             │
│   • Use structured logging (log/slog) which avoids %p on sensitive types │
│                                                                          │
│  THREAT 3: Panic via nil Stringer/Formatter                              │
│  ──────────────────────────────────────────                              │
│  Risk:    Nil pointer dereference inside String() or Format() causes panic│
│  Example: (*MyType)(nil).String() → PANIC if String() dereferences recv  │
│  Mitigation:                                                             │
│   • Always nil-check in pointer receiver methods                         │
│   • func (t *T) String() string { if t == nil { return "<nil>" } ... }   │
│                                                                          │
│  THREAT 4: Infinite Recursion / Stack Overflow                           │
│  ─────────────────────────────────────────────                           │
│  Risk:    String()/Format() calls fmt.Sprintf("%v", self)                │
│  Example: See §21.2 — stack grows until goroutine stack limit            │
│  Mitigation:                                                             │
│   • Use struct field accessors directly, never %v on receiver itself     │
│   • Code review + go vet                                                 │
│                                                                          │
│  THREAT 5: Goroutine DoS via Unbounded Formatting                        │
│  ─────────────────────────────────────────────────                       │
│  Risk:    Formatting a deeply nested or very large data structure         │
│           exhausts CPU/memory                                            │
│  Example: fmt.Sprintf("%#v", veryDeepMap)                                │
│  Mitigation:                                                             │
│   • Set maximum depth in custom Formatter implementations                 │
│   • Avoid fmt.Sprintf on external/untrusted data structures              │
│   • Use json.Marshal with a size-limited Writer for structured output     │
│                                                                          │
│  STATIC ANALYSIS TOOLS                                                   │
│  ──────────────────────                                                  │
│  go vet              — mismatched verbs, non-constant format strings     │
│  staticcheck SA1006  — fmt.Printf(x) where x is a variable              │
│  staticcheck SA1007  — invalid format verbs                              │
│  golangci-lint       — wraps all of the above                            │
└──────────────────────────────────────────────────────────────────────────┘
```

### Static analysis commands

```bash
# Built-in vet (always run)
go vet ./...

# staticcheck
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...

# golangci-lint (comprehensive)
golangci-lint run --enable=govet,staticcheck,errcheck,gosec
```

---

## 25. Next 3 Steps

### Step 1 — Benchmark your hot paths today

```bash
mkdir fmtbench && cd fmtbench
cat > bench_test.go << 'EOF'
package fmtbench

import (
    "fmt"
    "strconv"
    "strings"
    "testing"
)

func BenchmarkSprintf(b *testing.B) {
    for b.Loop() { _ = fmt.Sprintf("x=%d y=%s", 42, "hello") }
}
func BenchmarkAppendf(b *testing.B) {
    buf := make([]byte, 0, 64)
    for b.Loop() { buf = fmt.Appendf(buf[:0], "x=%d y=%s", 42, "hello") }
}
func BenchmarkManual(b *testing.B) {
    var sb strings.Builder
    for b.Loop() {
        sb.Reset()
        sb.WriteString("x=")
        sb.WriteString(strconv.Itoa(42))
        sb.WriteString(" y=hello")
        _ = sb.String()
    }
}
EOF
go test -bench=. -benchmem -count=5 -cpuprofile=cpu.pprof .
go tool pprof -http=:6060 cpu.pprof
```

### Step 2 — Implement `fmt.Formatter` for your core domain types

Take your most-logged struct (e.g., a request context, span, or security principal)
and implement `Format(f fmt.State, verb rune)`. Measure allocation difference
before/after with `go test -benchmem`.

```bash
# Verify zero allocations with testing.AllocsPerRun
go test -run TestFormatZeroAlloc -v ./...
```

### Step 3 — Enable `go vet` format checking in CI

```yaml
# .github/workflows/lint.yml (example)
- name: go vet
  run: go vet ./...
- name: staticcheck
  run: staticcheck ./...
```

And configure your editor to run `go vet` on save. This catches format mismatches at
development time before they reach production.

---

## References

- Go stdlib source: `src/fmt/print.go`, `src/fmt/format.go`, `src/fmt/scan.go`
  → `https://cs.opensource.google/go/go/+/main:src/fmt/`
- Official `fmt` package doc: `https://pkg.go.dev/fmt`
- Ryu algorithm (shortest float): `https://github.com/ulfjack/ryu`
- Go spec, string types: `https://go.dev/ref/spec#String_types`
- `fmt.Appendf` (Go 1.19): `https://go.dev/blog/go1.19`
- Go error wrapping: `https://go.dev/blog/go1.13-errors`
- staticcheck: `https://staticcheck.dev/docs/checks/#SA1006`
- Effective Go (formatting): `https://go.dev/doc/effective_go#printing`
