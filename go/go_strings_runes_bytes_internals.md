# Go Strings, Runes & Bytes: A Complete In-Depth Guide
> Everything happening under the hood — memory layout, Unicode, UTF-8, conversions, allocations, and the mental models you need to think like the Go runtime.

---

## Table of Contents

1. [What Is a String in Go?](#1-what-is-a-string-in-go)
2. [String Immutability: The Real Reason Why](#2-string-immutability-the-real-reason-why)
3. [Memory Layout of a Go String](#3-memory-layout-of-a-go-string)
4. [The Go Runtime String Header](#4-the-go-runtime-string-header)
5. [ASCII vs Unicode vs UTF-8: The Foundation](#5-ascii-vs-unicode-vs-utf-8-the-foundation)
6. [What Is a Rune?](#6-what-is-a-rune)
7. [UTF-8 Encoding in Detail](#7-utf-8-encoding-in-detail)
8. [String Indexing: Bytes, Not Characters](#8-string-indexing-bytes-not-characters)
9. [String to []byte Conversion: Under the Hood](#9-string-to-byte-conversion-under-the-hood)
10. [String to []rune Conversion: Under the Hood](#10-string-to-rune-conversion-under-the-hood)
11. [[]byte to String Conversion](#11-byte-to-string-conversion)
12. [[]rune to String Conversion](#12-rune-to-string-conversion)
13. [Compiler Optimizations: When Copies Are Avoided](#13-compiler-optimizations-when-copies-are-avoided)
14. [String Interning and the String Table](#14-string-interning-and-the-string-table)
15. [Ranging Over Strings](#15-ranging-over-strings)
16. [String Concatenation Internals](#16-string-concatenation-internals)
17. [strings.Builder and bytes.Buffer Internals](#17-stringsbuilder-and-bytesbuffer-internals)
18. [unsafe.String and unsafe.SliceData: Zero-Copy Conversions](#18-unsafestring-and-unsafeslicedata-zero-copy-conversions)
19. [The reflect.StringHeader and reflect.SliceHeader](#19-the-reflectstringheader-and-reflectsliceheader)
20. [Garbage Collector Interaction](#20-garbage-collector-interaction)
21. [Common Pitfalls and Their Explanations](#21-common-pitfalls-and-their-explanations)
22. [Mental Models Summary](#22-mental-models-summary)

---

## 1. What Is a String in Go?

In Go, a **string is a read-only slice of bytes**. That is the most precise and complete definition. It is NOT:

- A null-terminated C string
- A Java String object on the heap with metadata
- A Python object with reference count and encoding field
- An array of characters

A Go string is defined in the spec as:

> *"A string value is a (possibly empty) sequence of bytes. Strings are immutable: once created, it is impossible to change the contents of a string."*

The key insight is: **a string is just a pointer + a length**. Nothing more.

```
String = (pointer to byte array) + (integer length)
```

This two-word descriptor is stored on the stack (or inline in a struct). The actual byte data lives somewhere in memory — either in the read-only data segment (for string literals) or on the heap (for dynamically created strings).

---

## 2. String Immutability: The Real Reason Why

Go strings are immutable **by design and by enforcement**. Let's understand what "immutable" actually means at the machine level.

### Why Immutability?

**Reason 1: Safe sharing without copying**

Because strings cannot be mutated, Go can safely share the underlying byte array between multiple string values. When you assign one string to another:

```go
a := "hello"
b := a   // b points to SAME memory as a
```

Both `a` and `b` point to the exact same bytes in memory. No copy is made. This is safe ONLY because neither can mutate the data.

**Reason 2: String literals live in read-only memory**

String literals in Go source code are embedded in the binary's **read-only data segment** (`.rodata` on ELF systems, `__TEXT,__cstring` on Mach-O). The OS marks these pages as non-writable. If you could mutate a string, you'd get a segmentation fault when trying to write to a literal.

**Reason 3: Concurrency safety**

Immutable strings can be freely shared across goroutines without synchronization. No race conditions possible because no writes can happen.

**Reason 4: Hash map key stability**

Go map keys must be comparable and stable. Because strings can't change, using them as map keys is always safe. The hash of a string key won't change after insertion.

### What "Immutable" Enforces at the Type Level

```go
s := "hello"
s[0] = 'H'  // COMPILE ERROR: cannot assign to s[0] (strings are immutable)
```

The compiler rejects index assignment on strings. But the *pointer* inside the string header can be shared, repointed, or passed around freely — just the bytes it points to cannot be modified through the string interface.

---

## 3. Memory Layout of a Go String

Let's look at the actual memory layout, byte by byte.

### String Literal: "hello"

```
BINARY (.rodata segment) - READ ONLY, mapped into process memory:
┌─────────────────────────────────────────────────────────────────┐
│  Address: 0x004A3210                                            │
│  ┌────┬────┬────┬────┬────┐                                     │
│  │ h  │ e  │ l  │ l  │ o  │                                     │
│  │0x68│0x65│0x6C│0x6C│0x6F│                                     │
│  └────┴────┴────┴────┴────┘                                     │
│   [0]  [1]  [2]  [3]  [4]   ← byte offsets                     │
└─────────────────────────────────────────────────────────────────┘

STACK (goroutine stack frame):
┌─────────────────────────────────────────────────────────────────┐
│  Variable: s (type string)                                      │
│  ┌──────────────────────┬──────────────────────┐               │
│  │  ptr: 0x004A3210     │  len: 5              │               │
│  │  (8 bytes on 64-bit) │  (8 bytes on 64-bit) │               │
│  └──────────────────────┴──────────────────────┘               │
│   ^─────────────────────────┘                                   │
│   Points to .rodata                                             │
└─────────────────────────────────────────────────────────────────┘

Total size of string header: 16 bytes (on 64-bit systems)
Total size of string header:  8 bytes (on 32-bit systems)
```

### Two Strings Sharing the Same Backing Array

```go
a := "hello world"
b := a[6:]   // "world"
```

```
.rodata:
┌────┬────┬────┬────┬────┬────┬────┬────┬────┬────┬────┐
│ h  │ e  │ l  │ l  │ o  │    │ w  │ o  │ r  │ l  │ d  │
└────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┘
 0    1    2    3    4    5    6    7    8    9    10
 ▲                                 ▲
 │                                 │
 │  String "a":                    │  String "b":
 │  ┌──────────┬──────┐            │  ┌──────────┬──────┐
 └──│ ptr      │len=11│            └──│ ptr      │len=5 │
    └──────────┴──────┘               └──────────┴──────┘

No allocation. b.ptr = a.ptr + 6. Both share the same bytes.
```

---

## 4. The Go Runtime String Header

The Go runtime defines a string as the `StringHeader` type (now superseded by `unsafe.String` / `unsafe.StringData` in Go 1.20+, but the concept is the same).

### Internal Runtime Definition (runtime/string.go)

```go
// This is what the Go runtime uses internally:
type stringStruct struct {
    str unsafe.Pointer  // pointer to the first byte
    len int             // number of bytes (NOT characters)
}
```

This maps 1:1 with `reflect.StringHeader`:

```go
// reflect package:
type StringHeader struct {
    Data uintptr  // same as str — address of first byte
    Len  int      // same as len
}
```

### Inspecting It with reflect

```go
package main

import (
    "fmt"
    "reflect"
    "unsafe"
)

func main() {
    s := "hello, 世界"
    
    // Get the string header
    hdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
    
    fmt.Printf("String value  : %q\n", s)
    fmt.Printf("Header.Data   : 0x%X\n", hdr.Data)
    fmt.Printf("Header.Len    : %d bytes\n", hdr.Len)
    fmt.Printf("len(s)        : %d bytes\n", len(s))
    fmt.Printf("RuneCount     : %d runes\n", len([]rune(s)))
    fmt.Printf("unsafe.Sizeof : %d bytes (header size)\n", unsafe.Sizeof(s))
    
    // Manually read the bytes through the pointer
    for i := 0; i < hdr.Len; i++ {
        b := *(*byte)(unsafe.Pointer(hdr.Data + uintptr(i)))
        fmt.Printf("  byte[%d] = 0x%02X (%d)\n", i, b, b)
    }
}
```

Output:
```
String value  : "hello, 世界"
Header.Data   : 0x4A3210
Header.Len    : 13 bytes    ← "hello, " = 7 bytes, "世" = 3 bytes, "界" = 3 bytes
len(s)        : 13 bytes
RuneCount     : 9 runes
unsafe.Sizeof : 16 bytes (header size)
  byte[0]  = 0x68 (104)   h
  byte[1]  = 0x65 (101)   e
  ...
  byte[7]  = 0xE4 (228)   ← first byte of 世 (UTF-8: E4 B8 96)
  byte[8]  = 0xB8 (184)
  byte[9]  = 0x96 (150)
  byte[10] = 0xE7 (231)   ← first byte of 界 (UTF-8: E7 95 8C)
  byte[11] = 0x95 (149)
  byte[12] = 0x8C (140)
```

---

## 5. ASCII vs Unicode vs UTF-8: The Foundation

To understand rune and byte conversions, you must have a crystal-clear mental model of these three things.

### ASCII (American Standard Code for Information Interchange)

- 128 characters (0–127), fits in 7 bits
- Extended ASCII: 256 characters (0–255), fits in 1 byte
- Only covers English characters + control codes

```
ASCII Table (subset):
Dec  Hex   Char
-----------------
 65  0x41   A
 66  0x42   B
 97  0x61   a
 48  0x30   0
 32  0x20  (space)
```

### Unicode

Unicode is a **standard** that assigns a unique integer (called a **code point**) to every character in every human writing system.

- Code points range: U+0000 to U+10FFFF (1,114,112 possible values)
- Written as: U+XXXX (4 hex digits for BMP) or U+XXXXX / U+XXXXXX (5-6 digits for supplementary)
- Currently ~150,000 characters assigned (as of Unicode 15.1)

```
Unicode Code Points (examples):
Character  Code Point   Decimal   Description
─────────────────────────────────────────────────────────────
'A'        U+0041       65        Latin Capital Letter A
'a'        U+0061       97        Latin Small Letter A
'€'        U+20AC       8364      Euro Sign
'世'        U+4E16       19990     CJK Unified Ideograph
'😀'        U+1F600      128512    Grinning Face Emoji
```

Unicode defines WHAT number a character maps to, but NOT how to store it in memory.

### UTF-8 (Unicode Transformation Format — 8-bit)

UTF-8 is an **encoding** — it specifies how to represent Unicode code points as sequences of bytes.

Key properties:
- Variable-width: 1 to 4 bytes per code point
- ASCII-compatible: code points 0–127 are encoded as a single byte (same as ASCII)
- Self-synchronizing: you can find the start of a character from any position
- No null bytes in multi-byte sequences (safe for C strings)

```
UTF-8 Encoding Table:
──────────────────────────────────────────────────────────────────────────────
Code Point Range   Bytes  Byte 1    Byte 2    Byte 3    Byte 4
──────────────────────────────────────────────────────────────────────────────
U+0000–U+007F        1   0xxxxxxx
U+0080–U+07FF        2   110xxxxx  10xxxxxx
U+0800–U+FFFF        3   1110xxxx  10xxxxxx  10xxxxxx
U+10000–U+10FFFF     4   11110xxx  10xxxxxx  10xxxxxx  10xxxxxx
──────────────────────────────────────────────────────────────────────────────

Legend:
  0xxxxxxx   → leading byte of 1-byte sequence (bit 7 = 0)
  110xxxxx   → leading byte of 2-byte sequence
  1110xxxx   → leading byte of 3-byte sequence
  11110xxx   → leading byte of 4-byte sequence
  10xxxxxx   → continuation byte (always starts with 10)
  x          → payload bits (the actual code point value)
```

### Encoding Example: '世' (U+4E16)

```
Step 1: Code point in binary
  U+4E16 = 0100 1110 0001 0110  (16 bits)

Step 2: It falls in U+0800–U+FFFF range → 3-byte encoding
  Template: 1110xxxx 10xxxxxx 10xxxxxx
            (4 bits)  (6 bits)  (6 bits) → 16 bits total payload

Step 3: Split bits into groups (4, 6, 6):
  0100 1110 0001 0110
  ↓
  0100   111000   010110
  ↓       ↓         ↓
  4-bits  6-bits    6-bits

Step 4: Fill into template:
  1110|0100  10|111000  10|010110
  = 0xE4     = 0xB8     = 0x96

Result: E4 B8 96 (3 bytes)

Verification:
  E4 = 1110 0100
  B8 = 1011 1000
  96 = 1001 0110

  Extract payload: 0100 | 111000 | 010110
  Reassemble:      0100 1110 0001 0110  = 0x4E16 ✓
```

### Encoding Example: '😀' (U+1F600)

```
Step 1: Code point in binary
  U+1F600 = 0001 1111 0110 0000 0000  (21 bits)

Step 2: Falls in U+10000–U+10FFFF range → 4-byte encoding
  Template: 11110xxx 10xxxxxx 10xxxxxx 10xxxxxx
            (3 bits)  (6 bits)  (6 bits)  (6 bits) → 21 bits total

Step 3: Split bits (3, 6, 6, 6):
  000 | 011111 | 011000 | 000000

Step 4: Fill template:
  11110|000  10|011111  10|011000  10|000000
  = 0xF0     = 0x9F     = 0x98     = 0x80

Result: F0 9F 98 80 (4 bytes)
```

---

## 6. What Is a Rune?

In Go, `rune` is simply a **type alias for `int32`**:

```go
// From builtin/builtin.go:
type rune = int32
```

A rune represents a **Unicode code point**. The value of a rune IS the Unicode code point number.

```go
var r rune = '世'
fmt.Println(r)           // 19990    (decimal code point)
fmt.Printf("%U\n", r)   // U+4E16   (Unicode notation)
fmt.Printf("%c\n", r)   // 世        (the character)
fmt.Printf("%b\n", r)   // 100111000010110 (binary)
fmt.Printf("%x\n", r)   // 4e16     (hex)
```

### Why int32?

- Unicode code points go up to U+10FFFF = 1,114,111
- That fits in 21 bits
- `int32` = 32 bits — enough room with space to spare
- `int32` is signed; negative values are invalid runes (e.g., `RuneError` handling)

### Special Rune Constants

```go
// From unicode/utf8 package:
const (
    RuneError = '\uFFFD'  // = 65533 — the replacement character (�)
    RuneSelf  = 0x80      // = 128  — code points below this are single-byte in UTF-8
    MaxRune   = '\U0010FFFF' // = 1,114,111 — maximum valid Unicode code point
    UTFMax    = 4            // max bytes per UTF-8 encoded rune
)
```

### Memory Size

```
rune  = int32 = 4 bytes (always, regardless of code point value)
byte  = uint8 = 1 byte

A rune always takes 4 bytes of memory when stored as a rune variable.
A rune in a UTF-8 string takes 1–4 bytes (variable).

┌─────────────────────────────────────────────────────────┐
│  rune('A') = 65                                         │
│  ┌────────┬────────┬────────┬────────┐                 │
│  │  0x41  │  0x00  │  0x00  │  0x00  │  (little-endian)│
│  └────────┴────────┴────────┴────────┘                 │
│   byte 0   byte 1   byte 2   byte 3                     │
│   = 4 bytes total for this rune                         │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│  rune('世') = 19990 = 0x00004E16                        │
│  ┌────────┬────────┬────────┬────────┐                 │
│  │  0x16  │  0x4E  │  0x00  │  0x00  │  (little-endian)│
│  └────────┴────────┴────────┴────────┘                 │
│   = 4 bytes total for this rune                         │
└─────────────────────────────────────────────────────────┘
```

---

## 7. UTF-8 Encoding in Detail

Go source files are always UTF-8. Go strings store bytes. Go's `unicode/utf8` package provides the tools to interpret those bytes as UTF-8.

### The utf8 Package Key Functions

```go
package main

import (
    "fmt"
    "unicode/utf8"
)

func main() {
    s := "hello, 世界!"
    
    // Number of bytes vs number of runes
    fmt.Println(len(s))                    // 14 bytes
    fmt.Println(utf8.RuneCountInString(s)) // 10 runes
    
    // Decode one rune at a time
    for i := 0; i < len(s); {
        r, size := utf8.DecodeRuneInString(s[i:])
        fmt.Printf("index=%d rune=%c (%U) size=%d bytes\n", i, r, r, size)
        i += size
    }
    
    // Check validity
    fmt.Println(utf8.ValidString(s)) // true
    
    // Encode a rune to bytes
    buf := make([]byte, utf8.UTFMax)
    n := utf8.EncodeRune(buf, '世')
    fmt.Printf("Encoded: %X (%d bytes)\n", buf[:n], n) // E4 B8 96, 3 bytes
    
    // Rune length in bytes
    fmt.Println(utf8.RuneLen('A'))  // 1
    fmt.Println(utf8.RuneLen('世')) // 3
    fmt.Println(utf8.RuneLen('😀')) // 4
}
```

### UTF-8 Byte Pattern Detection

```
How to identify a byte's role in UTF-8:

Byte value range   Binary prefix    Role
───────────────────────────────────────────────────────
0x00–0x7F (0–127)    0xxxxxxx      Single-byte char (ASCII)
0x80–0xBF            10xxxxxx      Continuation byte
0xC0–0xDF            110xxxxx      Start of 2-byte sequence
0xE0–0xEF            1110xxxx      Start of 3-byte sequence
0xF0–0xF7            11110xxx      Start of 4-byte sequence
0xF8–0xFF                          Invalid in UTF-8

Test in Go:
  b >= 0x80 && b <= 0xBF  → continuation byte
  b >= 0xC0               → start of multi-byte sequence
  utf8.RuneStart(b)       → true if b is the start of a rune
```

### Internals of utf8.DecodeRuneInString

The decoder does this (simplified):

```go
func DecodeRuneInString(s string) (r rune, size int) {
    if len(s) == 0 {
        return RuneError, 0
    }
    b0 := s[0]
    
    // Single byte (ASCII): 0xxxxxxx
    if b0 < 0x80 {
        return rune(b0), 1
    }
    
    // Determine sequence length from leading byte
    var sz int
    switch {
    case b0 < 0xE0: sz = 2   // 110xxxxx
    case b0 < 0xF0: sz = 3   // 1110xxxx
    default:        sz = 4   // 11110xxx
    }
    
    if len(s) < sz {
        return RuneError, 1  // truncated
    }
    
    // Validate continuation bytes and extract payload
    // (simplified — real code has more edge case checks)
    var r32 rune
    switch sz {
    case 2:
        r32 = rune(b0&0x1F)<<6 | rune(s[1]&0x3F)
    case 3:
        r32 = rune(b0&0x0F)<<12 | rune(s[1]&0x3F)<<6 | rune(s[2]&0x3F)
    case 4:
        r32 = rune(b0&0x07)<<18 | rune(s[1]&0x3F)<<12 |
              rune(s[2]&0x3F)<<6 | rune(s[3]&0x3F)
    }
    
    return r32, sz
}
```

---

## 8. String Indexing: Bytes, Not Characters

This is one of the most important things to understand:

**`s[i]` gives you the i-th BYTE, not the i-th character.**

```go
s := "hello, 世界"

fmt.Println(s[0])  // 104  ('h' as a byte — works, it's ASCII)
fmt.Println(s[7])  // 228  (0xE4 — first byte of '世', NOT '世' itself)
fmt.Println(s[8])  // 184  (0xB8 — second byte of '世')
fmt.Println(s[9])  // 150  (0x96 — third byte of '世')
```

### Byte Layout of "hello, 世界"

```
Index:  0    1    2    3    4    5    6    7    8    9   10   11   12
Byte:  0x68 0x65 0x6C 0x6C 0x6F 0x2C 0x20 0xE4 0xB8 0x96 0xE7 0x95 0x8C
Char:  'h'  'e'  'l'  'l'  'o'  ','  ' '  ←────'世'────→  ←────'界'────→

Rune boundaries:
  Rune 0: byte[0]        → 'h'  (1 byte)
  Rune 1: byte[1]        → 'e'  (1 byte)
  Rune 2: byte[2]        → 'l'  (1 byte)
  Rune 3: byte[3]        → 'l'  (1 byte)
  Rune 4: byte[4]        → 'o'  (1 byte)
  Rune 5: byte[5]        → ','  (1 byte)
  Rune 6: byte[6]        → ' '  (1 byte)
  Rune 7: byte[7..9]     → '世' (3 bytes) ← byte index ≠ rune index!
  Rune 8: byte[10..12]   → '界' (3 bytes)
```

### The Dangerous Substring Bug

```go
s := "hello, 世界"

// WRONG: splits in middle of a rune!
fmt.Println(s[:8])  // "hello, " + one garbage byte → broken UTF-8

// RIGHT: use utf8-aware slicing
r := []rune(s)
fmt.Println(string(r[:8]))  // "hello, 世" — correct 8 runes
```

---

## 9. String to []byte Conversion: Under the Hood

```go
s := "hello"
b := []byte(s)  // What actually happens?
```

### The Conversion Steps

```
Step 1: Read string header
        s.ptr → points to original bytes (possibly .rodata)
        s.len → 5

Step 2: Allocate new backing array on the heap
        runtime.mallocgc(5, nil, false)
        → returns pointer to freshly allocated 5-byte region

Step 3: Copy bytes from string to new allocation
        runtime.memmove(newptr, s.ptr, 5)

Step 4: Construct slice header
        b.ptr → newptr    (newly allocated memory)
        b.len → 5
        b.cap → 5         (exactly the length)

Memory diagram:

BEFORE (string):
  .rodata:
  ┌────┬────┬────┬────┬────┐
  │ h  │ e  │ l  │ l  │ o  │
  └────┴────┴────┴────┴────┘
     ▲
     │  s: [ptr=▲, len=5]

AFTER ([]byte):
  .rodata (unchanged):
  ┌────┬────┬────┬────┬────┐
  │ h  │ e  │ l  │ l  │ o  │  ← original, untouched
  └────┴────┴────┴────┴────┘

  Heap (new allocation):
  ┌────┬────┬────┬────┬────┐
  │ h  │ e  │ l  │ l  │ o  │  ← copied bytes
  └────┴────┴────┴────┴────┘
     ▲
     │  b: [ptr=▲, len=5, cap=5]
```

### The []byte Slice Header

```go
// runtime/slice.go equivalent:
type sliceHeader struct {
    ptr unsafe.Pointer  // pointer to first element
    len int             // number of elements
    cap int             // capacity (max without reallocation)
}
```

```
SliceHeader layout (64-bit):
┌──────────────────────────────────────────────────────────┐
│ Offset 0:  ptr  (8 bytes) — pointer to backing array     │
│ Offset 8:  len  (8 bytes) — current number of elements   │
│ Offset 16: cap  (8 bytes) — total capacity               │
│                                                           │
│ Total: 24 bytes                                           │
└──────────────────────────────────────────────────────────┘
```

### Why a Copy Is Made

The string backing array might be in `.rodata` (read-only memory). A `[]byte` slice needs to be writable (you can do `b[0] = 'H'`). Therefore, Go MUST copy the bytes to writable heap memory. This is not a performance bug — it's a correctness requirement.

### The Runtime Function

The actual Go runtime function called for `string → []byte`:

```go
// runtime/string.go
func stringtoslicebyte(buf *tmpBuf, s string) []byte {
    var b []byte
    if buf != nil && len(s) <= len(buf) {
        // Small strings: use stack buffer to avoid heap allocation
        *buf = tmpBuf{}
        b = buf[:len(s)]
    } else {
        // Large strings: allocate on heap
        b = rawbyteslice(len(s))
    }
    copy(b, s)  // copies the bytes
    return b
}
```

Note the `tmpBuf` optimization: for small strings (≤32 bytes), the Go compiler can stack-allocate the buffer, avoiding heap allocation entirely!

```go
// The tmpBuf type:
type tmpBuf [32]byte  // 32-byte stack buffer
```

### Full Example with Memory Analysis

```go
package main

import (
    "fmt"
    "unsafe"
    "reflect"
)

func main() {
    s := "hello, 世界"
    
    // Get string's data address
    sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
    fmt.Printf("String ptr: 0x%X\n", sh.Data)
    fmt.Printf("String len: %d\n", sh.Len)
    
    // Convert to []byte
    b := []byte(s)
    
    // Get slice's data address
    bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
    fmt.Printf("Slice ptr:  0x%X\n", bh.Data)  // DIFFERENT address!
    fmt.Printf("Slice len:  %d\n", bh.Len)
    fmt.Printf("Slice cap:  %d\n", bh.Cap)
    
    // Prove it's a copy: modify the byte slice
    b[0] = 'H'
    fmt.Println(string(b))  // "Hello, 世界"
    fmt.Println(s)          // "hello, 世界" — unchanged!
    
    // Prove addresses differ
    fmt.Println(sh.Data == bh.Data)  // false — different memory
}
```

---

## 10. String to []rune Conversion: Under the Hood

```go
s := "hello, 世界"
r := []rune(s)  // What actually happens?
```

This is MORE expensive than `[]byte` because:
1. The number of runes is not known without scanning the string
2. Each rune is 4 bytes, but UTF-8 chars are 1–4 bytes
3. Full UTF-8 decoding must happen

### The Conversion Steps

```
Step 1: Count the runes (first pass over the string)
        OR over-allocate by len(s) runes (worst case: all ASCII)
        Go's runtime typically allocates len(s) capacity first

Step 2: Allocate a []int32 ([]rune) on the heap
        size = numRunes * 4 bytes each

Step 3: Decode UTF-8 bytes → rune values (second pass)
        For each valid UTF-8 sequence, compute the code point
        and store it as int32 in the slice

Step 4: Return the slice

Memory diagram for "hello, 世界" (13 bytes → 9 runes):

String bytes in .rodata:
┌────┬────┬────┬────┬────┬────┬────┬────┬────┬────┬────┬────┬────┐
│0x68│0x65│0x6C│0x6C│0x6F│0x2C│0x20│0xE4│0xB8│0x96│0xE7│0x95│0x8C│
│ h  │ e  │ l  │ l  │ o  │ ,  │    │ ←──────世──────→ │ ←──────界──────→ │
└────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┘
  1B   1B   1B   1B   1B   1B   1B          3B                3B

[]rune on Heap (9 * 4 = 36 bytes):
┌────────────┬────────────┬────────────┬────────────┬────────────┬────────────┬────────────┬────────────┬────────────┐
│  0x00000068│  0x00000065│  0x0000006C│  0x0000006C│  0x0000006F│  0x0000002C│  0x00000020│  0x00004E16│  0x00754E  │
│    'h'=104 │    'e'=101 │    'l'=108 │    'l'=108 │    'o'=111 │    ','=44  │    ' '=32  │  '世'=19990│  '界'=30028│
│  [4 bytes] │  [4 bytes] │  [4 bytes] │  [4 bytes] │  [4 bytes] │  [4 bytes] │  [4 bytes] │  [4 bytes] │  [4 bytes] │
└────────────┴────────────┴────────────┴────────────┴────────────┴────────────┴────────────┴────────────┴────────────┘
   rune[0]      rune[1]      rune[2]      rune[3]      rune[4]      rune[5]      rune[6]      rune[7]      rune[8]
```

### The Runtime Function

```go
// runtime/string.go
func stringtoslicerune(buf *[tmpStringBufSize]rune, s string) []rune {
    // Quick check: all ASCII?
    // (optimization: if all bytes < 0x80, just cast each byte to rune)
    
    // Count runes first
    n := 0
    for _, _ = range s {
        n++
    }
    
    var a []rune
    if buf != nil && n <= len(buf) {
        *buf = [tmpStringBufSize]rune{}
        a = buf[:n]
    } else {
        a = rawruneslice(n)
    }
    
    // Decode UTF-8 into runes
    n = 0
    for _, r := range s {
        a[n] = r
        n++
    }
    return a
}
```

### Comparing []byte vs []rune Conversion Cost

```
String "hello" (5 ASCII chars):

  → []byte:  allocate 5 bytes, memcopy 5 bytes. O(n) bytes.
  → []rune:  allocate 20 bytes (5*4), decode 5 chars, store 5 int32s. O(n) bytes + O(n) decode.

String "世界" (6 bytes, 2 CJK chars):

  → []byte:  allocate 6 bytes, memcopy 6 bytes. Simple.
  → []rune:  allocate 8 bytes (2*4), decode UTF-8 (6 bytes in → 2 rune values out). Compact output.

String "😀😀😀" (12 bytes, 3 emoji, each 4 bytes in UTF-8):

  → []byte:  allocate 12 bytes, copy 12 bytes.
  → []rune:  allocate 12 bytes (3*4), decode UTF-8 (12 bytes → 3 runes).
              Same byte count coincidentally, but different values stored!
```

### Full Rune Conversion Example

```go
package main

import (
    "fmt"
    "unicode/utf8"
    "unsafe"
    "reflect"
)

func main() {
    s := "hello, 世界"
    
    r := []rune(s)
    
    rh := (*reflect.SliceHeader)(unsafe.Pointer(&r))
    fmt.Printf("Rune slice ptr:  0x%X\n", rh.Data)
    fmt.Printf("Rune slice len:  %d\n", rh.Len)   // 9 runes
    fmt.Printf("Rune slice cap:  %d\n", rh.Cap)   // 9
    fmt.Printf("Memory used:     %d bytes\n", rh.Len * int(unsafe.Sizeof(r[0]))) // 36 bytes
    
    // Each rune is 4 bytes regardless of UTF-8 size
    for i, ru := range r {
        byteSize := utf8.RuneLen(ru)
        fmt.Printf("r[%d] = %c  code_point=%-6d  utf8_bytes=%d  stored_as=4bytes\n",
            i, ru, ru, byteSize)
    }
}
```

---

## 11. []byte to String Conversion

```go
b := []byte{'h', 'e', 'l', 'l', 'o'}
s := string(b)  // What happens?
```

### The Conversion Steps

```
Step 1: Allocate a new backing array in read-friendly memory (heap)
        size = len(b) bytes

Step 2: Copy bytes from slice to new allocation
        runtime.memmove(newptr, b.ptr, len(b))

Step 3: Construct string header
        s.ptr → newptr
        s.len → len(b)

Memory diagram:
  Heap (slice backing array — WRITABLE):
  ┌────┬────┬────┬────┬────┐
  │ h  │ e  │ l  │ l  │ o  │
  └────┴────┴────┴────┴────┘
     ▲  b: [ptr=▲, len=5, cap=5]

  Heap (new string backing — also on heap, could be freed):
  ┌────┬────┬────┬────┬────┐
  │ h  │ e  │ l  │ l  │ o  │  ← copy
  └────┴────┴────┴────┴────┘
     ▲  s: [ptr=▲, len=5]
```

### Why Copy Again?

The original `[]byte` slice is mutable — someone could modify `b[0]` after calling `string(b)`. If the string shared the same memory, this would violate string immutability. So Go copies.

However, there's an important compiler optimization here (see Section 13).

### The Runtime Function

```go
// runtime/string.go
func slicebytetostring(buf *tmpBuf, b []byte) string {
    l := len(b)
    if l == 0 {
        return ""
    }
    if l == 1 {
        // Single-byte optimization: return pre-allocated string
        stringOfByte := staticuint64s[b[0]]  // pre-built table
        return unsafe.String(&stringOfByte, 1)
    }
    
    var p unsafe.Pointer
    if buf != nil && l <= len(buf) {
        p = unsafe.Pointer(buf)  // use stack buffer
    } else {
        p = mallocgc(uintptr(l), nil, false)  // heap allocation
    }
    memmove(p, unsafe.Pointer(&b[0]), uintptr(l))
    return unsafe.String((*byte)(p), l)
}
```

Note the `staticuint64s` optimization: for single-byte strings (all 256 ASCII values), Go has a pre-allocated lookup table. No allocation needed!

---

## 12. []rune to String Conversion

```go
r := []rune{'h', 'e', 'l', 'l', 'o', ',', ' ', '世', '界'}
s := string(r)  // What happens?
```

This is the reverse of section 10: encode each rune (int32 code point) as UTF-8 bytes.

### The Conversion Steps

```
Step 1: Calculate required byte size
        For each rune, determine its UTF-8 byte count (1–4)
        Sum them all up

Step 2: Allocate that many bytes on the heap

Step 3: Encode each rune into UTF-8 bytes in the allocated buffer

Step 4: Construct string header

[]rune (36 bytes in memory):
┌────────────┬─ ... ─┬────────────┬────────────┐
│  0x00000068│  ...  │  0x00004E16│  0x0000754E│
│     'h'    │       │    '世'    │    '界'    │
└────────────┴─ ... ─┴────────────┴────────────┘

Encoded UTF-8 string (13 bytes):
┌────┬────┬────┬────┬────┬────┬────┬────┬────┬────┬────┬────┬────┐
│0x68│0x65│0x6C│0x6C│0x6F│0x2C│0x20│0xE4│0xB8│0x96│0xE7│0x95│0x8C│
└────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┘
 'h'  'e'  'l'  'l'  'o'  ','  ' '  ←────'世'────→  ←────'界'────→
```

### The Runtime Function

```go
// runtime/string.go
func slicerunetostring(buf *tmpBuf, a []rune) string {
    if len(a) == 0 {
        return ""
    }
    
    // Calculate total byte size needed
    size1 := 0
    for _, r := range a {
        size1 += utf8.RuneLen(r)  // 1, 2, 3, or 4
    }
    
    // Allocate
    s, b := rawstringtmp(buf, size1)
    
    // Encode each rune as UTF-8
    size2 := 0
    for _, r := range a {
        // This is equivalent to utf8.EncodeRune:
        if uint32(r) < utf8.RuneSelf {
            b[size2] = byte(r)
            size2++
        } else {
            size2 += utf8.EncodeRune(b[size2:], r)
        }
    }
    return s
}
```

---

## 13. Compiler Optimizations: When Copies Are Avoided

The Go compiler is smart. It avoids unnecessary copies in several key scenarios.

### Optimization 1: `[]byte` to `string` in Map Lookup

```go
m := map[string]int{"hello": 1}
b := []byte("hello")

// This does NOT allocate a new string:
v := m[string(b)]  // compiler converts []byte→string without copy for map lookup
```

The compiler knows that the temporary string will not escape, so it can use the slice's backing array directly.

### Optimization 2: `string` + Concatenation

```go
b := []byte("hello")
// This also avoids copy in some cases:
if string(b) == "hello" {  // no allocation!
    // ...
}
```

When a `[]byte→string` conversion is used only for comparison, the compiler can skip allocation.

### Optimization 3: `for range` on String

```go
s := "hello"
// This does NOT convert to []rune first:
for i, r := range s {
    // i = byte index, r = rune value
    // Decoded on the fly by the range loop itself
}
```

The `for range` over a string is a built-in operation that decodes UTF-8 incrementally without allocating a `[]rune` slice.

### Optimization 4: Small String Stack Allocation

When strings are small and provably non-escaping, the Go compiler allocates them on the goroutine stack instead of the heap:

```go
func process(data []byte) {
    s := string(data)  // if s doesn't escape this function → stack allocated
    _ = strings.Contains(s, "foo")  // used locally only
}
```

### Optimization 5: `[]byte(s)` in `for range`

```go
s := "hello"
// This also avoids a copy in for-range context:
for i, b := range []byte(s) {
    // The []byte is not actually allocated in recent Go versions
}
```

### Seeing Optimizations with go build -gcflags

```bash
# Show escape analysis decisions:
go build -gcflags="-m=2" ./...

# Show all optimizations:
go build -gcflags="-m" ./...

# Disassemble to see actual machine code:
go tool compile -S main.go | grep -A5 "stringtoslicebyte"
```

### Optimization Summary Table

```
Conversion            Context                    Allocation?
──────────────────────────────────────────────────────────────────
string → []byte       map lookup                 NO (compiler opt)
string → []byte       comparison (==)            NO (compiler opt)
string → []byte       general use                YES (always copies)
[]byte → string       map lookup                 NO (compiler opt)
[]byte → string       comparison                 NO (compiler opt)
[]byte → string       general use                YES (always copies)
string → []rune       for range                  NO (decoded in-place)
string → []rune       general use                YES (always allocates)
[]rune → string       any                        YES (must encode UTF-8)
```

---

## 14. String Interning and the String Table

### String Literals in the Binary

All string literals in your Go program are stored in the **read-only data section** of the binary:

```bash
# Inspect a compiled Go binary:
go build -o myapp main.go
strings myapp | grep "hello"  # See literal strings
objdump -s -j .rodata myapp   # See raw rodata bytes
```

### The String Constant Table

The Go linker deduplicates identical string literals. If you write `"hello"` ten times in your program, there's only ONE copy of those bytes in the binary's `.rodata`.

```go
// All three s1, s2, s3 point to the SAME bytes in .rodata:
const s1 = "hello"
var s2 = "hello"
var s3 = "hello"

// Verify (using unsafe):
// s1.ptr == s2.ptr == s3.ptr   → true (same .rodata address)
```

### Runtime String Interning

Go does NOT automatically intern dynamically-created strings (unlike Java). If you build two identical strings at runtime, they get two separate heap allocations:

```go
a := strings.Repeat("x", 5)  // heap alloc 1: "xxxxx"
b := strings.Repeat("x", 5)  // heap alloc 2: "xxxxx"
// a and b are equal in value but different in memory
```

For manual interning, you can use a `sync.Map` or a map:

```go
package main

import (
    "sync"
)

// Simple string interner
type Interner struct {
    mu sync.Mutex
    m  map[string]string
}

func (in *Interner) Intern(s string) string {
    in.mu.Lock()
    defer in.mu.Unlock()
    if existing, ok := in.m[s]; ok {
        return existing  // return the canonical copy
    }
    in.m[s] = s
    return s
}
```

---

## 15. Ranging Over Strings

`for i, r := range s` is the correct way to iterate over characters. It decodes UTF-8 on the fly.

### What range Does Step by Step

```go
s := "A世B"  // bytes: 0x41 0xE4 0xB8 0x96 0x42

for i, r := range s {
    fmt.Printf("i=%d r=%c\n", i, r)
}
// Output:
// i=0 r=A      ← byte index 0, rune 'A' (1 byte)
// i=1 r=世     ← byte index 1, rune '世' (3 bytes, occupies bytes 1,2,3)
// i=4 r=B      ← byte index 4, rune 'B' (1 byte) — NOTE: i jumps from 1 to 4!
```

The key point: `i` is the **byte index** of the START of the rune, NOT the rune's ordinal position.

### Internal Algorithm of range on string

```
state = {byte_index: 0}

loop:
  if byte_index >= len(s): STOP

  b0 = s[byte_index]

  if b0 < 0x80:                    // ASCII single byte
    r = rune(b0)
    size = 1
  else:
    r, size = utf8.DecodeRuneInString(s[byte_index:])

  yield (i=byte_index, r=r)
  byte_index += size
  goto loop

ASCII String "hello" traversal:
  iter 1: i=0, r='h', size=1, next_i=1
  iter 2: i=1, r='e', size=1, next_i=2
  iter 3: i=2, r='l', size=1, next_i=3
  iter 4: i=3, r='l', size=1, next_i=4
  iter 5: i=4, r='o', size=1, next_i=5
  done (next_i >= len=5)

UTF-8 String "A世B" traversal:
  iter 1: i=0, b0=0x41 (<0x80), r='A', size=1, next_i=1
  iter 2: i=1, b0=0xE4 (≥0x80, 3-byte seq), r='世', size=3, next_i=4
  iter 3: i=4, b0=0x42 (<0x80), r='B', size=1, next_i=5
  done (next_i >= len=5)
```

### Invalid UTF-8 Handling in range

```go
s := string([]byte{0x41, 0xFF, 0x42})  // 'A', invalid byte, 'B'

for i, r := range s {
    fmt.Printf("i=%d r=%c (%d)\n", i, r, r)
}
// Output:
// i=0 r=A (65)
// i=1 r=� (65533)   ← RuneError (U+FFFD) for the invalid byte 0xFF
// i=2 r=B (66)
```

`range` never panics on invalid UTF-8; it yields `utf8.RuneError` (U+FFFD, value 65533) for each invalid byte and advances by exactly 1 byte.

---

## 16. String Concatenation Internals

### Single `+` Concatenation

```go
a := "hello"
b := " world"
c := a + b  // What happens?
```

```
Step 1: Compute total length = len(a) + len(b) = 11
Step 2: Allocate 11 bytes on the heap
Step 3: Copy a's bytes into new buffer (bytes 0–4)
Step 4: Copy b's bytes into new buffer (bytes 5–10)
Step 5: Construct string header pointing to new buffer

Result: one new heap allocation of 11 bytes.
        a and b are unchanged.

Memory:
  a.ptr → [h e l l o]           (original, untouched)
  b.ptr → [ w o r l d]          (original, untouched)
  c.ptr → [h e l l o   w o r l d]  (new allocation)
```

### Multiple `+` in One Expression

```go
c := "hello" + " " + "world"
```

The Go compiler is smart: when multiple string literals are concatenated, it often computes the result at **compile time**! The resulting string is directly embedded in `.rodata` with no runtime allocation.

### Loop Concatenation: The Classic Trap

```go
// BAD: O(n²) allocations
s := ""
for i := 0; i < n; i++ {
    s += words[i]  // Each += allocates a new string!
}
```

```
Iteration 1: alloc  5 bytes → "hello"
Iteration 2: alloc  8 bytes → "hello my"        (copies 5 + 3)
Iteration 3: alloc 14 bytes → "hello my friend"  (copies 8 + 6)
...

Total bytes copied = 5 + 8 + 14 + ... = O(n²)
Total allocations = n
```

### strings.Builder: The Right Way

```go
// GOOD: Amortized O(n) with strings.Builder
var sb strings.Builder
for i := 0; i < n; i++ {
    sb.WriteString(words[i])
}
s := sb.String()
```

---

## 17. strings.Builder and bytes.Buffer Internals

### strings.Builder Layout

```go
// strings/builder.go
type Builder struct {
    addr *Builder  // of receiver, to detect copies by value
    buf  []byte    // internal byte buffer (mutable!)
}
```

```
strings.Builder state diagram:

Initial:
  ┌──────────────────────────────────────┐
  │ addr: nil                            │
  │ buf:  [ptr=nil, len=0, cap=0]        │
  └──────────────────────────────────────┘

After WriteString("hello"):
  Heap:
  ┌────┬────┬────┬────┬────┬────┬────┬────┐
  │ h  │ e  │ l  │ l  │ o  │    │    │    │
  └────┴────┴────┴────┴────┴────┴────┴────┘
   [0]  [1]  [2]  [3]  [4]   capacity=8 (grown by Go's append rules)
  
  Builder:
  ┌──────────────────────────────────────┐
  │ addr: &builder                       │
  │ buf:  [ptr=▲, len=5, cap=8]          │
  └──────────────────────────────────────┘

After WriteString(" world"):
  Heap (same allocation, enough cap):
  ┌────┬────┬────┬────┬────┬────┬────┬────┬────┬────┬────┐
  │ h  │ e  │ l  │ l  │ o  │    │ w  │ o  │ r  │ l  │ d  │ ...
  └────┴────┴────┴────┴────┴────┴────┴────┴────┴────┴────┘
  
  Builder:
  ┌──────────────────────────────────────┐
  │ buf:  [ptr=▲, len=11, cap=?]         │
  └──────────────────────────────────────┘

sb.String():
  Returns string{ptr: buf.ptr, len: buf.len}
  ← ZERO COPY! The string points directly into buf's memory.
  (Safe because Builder gives you the string and you stop using Builder)
```

### The Magic of sb.String()

```go
// strings/builder.go
func (b *Builder) String() string {
    return unsafe.String(unsafe.SliceData(b.buf), len(b.buf))
}
```

This creates a string that shares memory with the Builder's internal buffer — **no copy**! This is safe because:
1. You're expected to stop using the Builder after calling `String()`
2. The `addr` field detects if you copy the Builder by value (which could be unsafe)

### Buffer Growth Algorithm

When a write exceeds capacity, the buffer grows. Go's `append` uses a growth formula:

```
Old cap < 256:     new cap = old cap * 2
Old cap >= 256:    new cap = old cap + (old cap + 3*256) / 4
                   (roughly 1.25x growth for large buffers)

Growth trace for WriteString calls to a Builder:
  After "hello"  (5):  cap=8   (Go allocates at least 8)
  After " world" (6):  cap=8   (still fits: 5+6=11 > 8? → realloc)
  
  Actually:
  Initial append: Go allocates capacity for 5 bytes → probably cap=8
  Next append needs 11 total: 11 > 8 → grow: new_cap = max(8*2, 11) = 16
  ...
```

### bytes.Buffer vs strings.Builder

```
bytes.Buffer:
  - Can read FROM the buffer as well as write TO it
  - Tracks a "read position" (separate from write position)
  - More overhead (tracking unread bytes)
  - Use when you need an io.Reader AND io.Writer

strings.Builder:
  - Write-only (append only)
  - Lighter: just a []byte with WriteString/WriteByte/etc.
  - Use when building strings efficiently
  - String() returns without copying (unsafe trick)

bytes.Buffer layout:
  type Buffer struct {
      buf      []byte  // storage
      off      int     // read offset (buf[off:] = unread portion)
      lastRead readOp  // last read operation (for UnreadByte/UnreadRune)
  }
```

---

## 18. unsafe.String and unsafe.SliceData: Zero-Copy Conversions

Since Go 1.20, the official way to do zero-copy string↔[]byte conversions uses new `unsafe` builtins:

```go
import "unsafe"

// []byte → string (ZERO COPY — no allocation, no memcopy)
func bytesToString(b []byte) string {
    return unsafe.String(unsafe.SliceData(b), len(b))
}

// string → []byte (ZERO COPY — dangerous! string must not be modified)
func stringToBytes(s string) []byte {
    return unsafe.Slice(unsafe.StringData(s), len(s))
}
```

### How These Work

```go
// unsafe.SliceData(b []byte) → *byte
// Returns a pointer to the first element of the slice's backing array
// For a nil slice, returns nil
// For a zero-length slice, may return an invalid (but non-nil) pointer

// unsafe.StringData(s string) → *byte  
// Returns a pointer to the first byte of the string's data
// For an empty string, may return nil or any non-nil pointer

// unsafe.String(ptr *byte, len IntegerType) → string
// Creates a string with:
//   .ptr = ptr
//   .len = len
// NO allocation, NO copy. Just constructs the 16-byte header.

// unsafe.Slice(ptr *T, len IntegerType) → []T
// Creates a slice with:
//   .ptr = ptr
//   .len = len
//   .cap = len
// NO allocation, NO copy.
```

### The Danger of Zero-Copy string→[]byte

```go
s := "hello"
b := unsafe.Slice(unsafe.StringData(s), len(s))

// s points to .rodata (read-only OS page)
// b points to the SAME memory

b[0] = 'H'  // SEGFAULT! Writing to read-only memory!
```

This will crash your program. Only use `stringToBytes` when you KNOW you will never modify the slice (e.g., passing it to a function that only reads it).

### The Pre-1.20 Way (Still Works)

```go
// Using reflect headers (works but reflect.StringHeader is deprecated):
func bytesToStringUnsafe(b []byte) string {
    bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
    sh := reflect.StringHeader{
        Data: bh.Data,
        Len:  bh.Len,
    }
    return *(*string)(unsafe.Pointer(&sh))
}

func stringToBytesUnsafe(s string) []byte {
    sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
    bh := reflect.SliceHeader{
        Data: sh.Data,
        Len:  sh.Len,
        Cap:  sh.Len,
    }
    return *(*[]byte)(unsafe.Pointer(&bh))
}
```

### When Is Zero-Copy Legitimate?

```go
// LEGITIMATE: Converting to []byte just to pass to a function that reads it
func hashString(s string) uint64 {
    b := unsafe.Slice(unsafe.StringData(s), len(s))
    return xxhash.Sum64(b)  // xxhash only reads, doesn't write
}

// LEGITIMATE: Converting []byte to string for printing/comparison
// (the []byte was just built and won't be modified)
func processBuffer(buf []byte) {
    s := unsafe.String(unsafe.SliceData(buf), len(buf))
    if s == "expected" {  // safe: just reading
        // ...
    }
}

// NOT LEGITIMATE: Modifying the bytes of a string
func bad(s string) {
    b := unsafe.Slice(unsafe.StringData(s), len(s))
    b[0] = 'X'  // UNDEFINED BEHAVIOR / SEGFAULT
}
```

---

## 19. The reflect.StringHeader and reflect.SliceHeader

These types let you inspect and manipulate string/slice internals. As of Go 1.20, they're deprecated in favor of `unsafe.String`/`unsafe.Slice`, but understanding them builds a clear mental model.

### StringHeader

```go
// reflect/value.go
type StringHeader struct {
    Data uintptr  // pointer to byte data (as integer, NOT *byte)
    Len  int      // number of bytes
}

// Size: 16 bytes on 64-bit, 8 bytes on 32-bit
```

### SliceHeader

```go
// reflect/value.go
type SliceHeader struct {
    Data uintptr  // pointer to element data
    Len  int      // number of elements
    Cap  int      // capacity in elements
}

// Size: 24 bytes on 64-bit, 12 bytes on 32-bit
```

### Layout Comparison

```
64-bit system memory layout:

StringHeader:
┌──────────────────────┬──────────────────────┐
│  Data (uintptr)      │  Len (int)           │
│  8 bytes             │  8 bytes             │
│  offset: 0           │  offset: 8           │
└──────────────────────┴──────────────────────┘
Total: 16 bytes

SliceHeader:
┌──────────────────────┬──────────────────────┬──────────────────────┐
│  Data (uintptr)      │  Len (int)           │  Cap (int)           │
│  8 bytes             │  8 bytes             │  8 bytes             │
│  offset: 0           │  offset: 8           │  offset: 16          │
└──────────────────────┴──────────────────────┴──────────────────────┘
Total: 24 bytes

A string IS the first 16 bytes of a slice header (ptr + len)!
This is why the unsafe conversion between string and []byte works by reinterpreting memory.
```

### Complete Introspection Example

```go
package main

import (
    "fmt"
    "reflect"
    "unsafe"
)

func inspectString(s string) {
    h := (*reflect.StringHeader)(unsafe.Pointer(&s))
    fmt.Printf("=== String: %q ===\n", s)
    fmt.Printf("  Header address:  %p\n", &s)
    fmt.Printf("  Data pointer:    0x%016X\n", h.Data)
    fmt.Printf("  Length (bytes):  %d\n", h.Len)
    fmt.Printf("  Header size:     %d bytes\n", unsafe.Sizeof(s))
    
    // Print raw bytes
    fmt.Printf("  Raw bytes: ")
    for i := 0; i < h.Len; i++ {
        b := *(*byte)(unsafe.Pointer(h.Data + uintptr(i)))
        fmt.Printf("%02X ", b)
    }
    fmt.Println()
}

func inspectSlice(b []byte) {
    h := (*reflect.SliceHeader)(unsafe.Pointer(&b))
    fmt.Printf("=== []byte: %v ===\n", b)
    fmt.Printf("  Header address:  %p\n", &b)
    fmt.Printf("  Data pointer:    0x%016X\n", h.Data)
    fmt.Printf("  Length:          %d\n", h.Len)
    fmt.Printf("  Capacity:        %d\n", h.Cap)
    fmt.Printf("  Header size:     %d bytes\n", unsafe.Sizeof(b))
}

func main() {
    s := "hello, 世界"
    inspectString(s)
    
    b := []byte(s)
    inspectSlice(b)
    
    fmt.Printf("\nSame backing data? %v\n",
        (*reflect.StringHeader)(unsafe.Pointer(&s)).Data ==
        (*reflect.SliceHeader)(unsafe.Pointer(&b)).Data)
    // false — []byte(s) made a copy
}
```

---

## 20. Garbage Collector Interaction

### String Lifetime and GC

The GC treats strings like any other pointer-bearing value. The backing byte array is kept alive as long as any string header points to it.

```go
// IMPORTANT: Substring keeps entire original backing array alive!
func getPrefix(s string) string {
    return s[:3]  // returns a 3-byte string, but holds reference to ALL of s's bytes
}

// If s was a 1MB string, the original 1MB stays in memory even though
// you only kept 3 bytes!
```

### The Substring Memory Leak

```
Original string (1MB):
┌──────────────────────────────────────────────────────────────┐
│  [1,000,000 bytes of data]                                   │
└──────────────────────────────────────────────────────────────┘
     ▲
     │  sub = original[:3]
     │  ┌──────────┬─────┐
     └──│ ptr      │len=3│   ← only 3 bytes "visible"
        └──────────┴─────┘   but the entire 1MB backing array is PINNED

GC cannot free the 1MB array because sub.ptr points into it!
```

### Fix: Force a Copy

```go
func getPrefix(s string) string {
    // Make an independent copy — breaks the reference to the large string
    return string([]byte(s[:3]))
    // Or equivalently:
    // return strings.Clone(s[:3])  // Go 1.20+
}
```

`strings.Clone` (Go 1.20+) explicitly creates a fresh copy:

```go
// strings/clone.go
func Clone(s string) string {
    if len(s) == 0 {
        return ""
    }
    b := make([]byte, len(s))
    copy(b, s)
    return unsafe.String(&b[0], len(b))
}
```

### Write Barrier and Strings

When the GC runs, it needs to track pointers. String headers contain a pointer (`Data`). The GC's **write barrier** ensures that when a string is stored into a heap location, the GC is notified so it can update its pointer tracking.

```
Stack string → heap struct field:

  s := "hello"             // s on stack, data in .rodata
  p := &MyStruct{}         // MyStruct on heap
  p.Name = s               // Write barrier fires!
                           // GC now knows p.Name.Data → .rodata
                           // Keeps .rodata pinned (but .rodata never moves anyway)

  When the struct is assigned a dynamically created string:
  p.Name = buildString()   // Write barrier fires!
                           // GC records: p.Name.Data → heap address
                           // GC won't collect that heap allocation
```

---

## 21. Common Pitfalls and Their Explanations

### Pitfall 1: Expecting `len(s)` to Return Character Count

```go
s := "hello, 世界"
fmt.Println(len(s))         // 13 ← BYTES, not characters!
fmt.Println(len([]rune(s))) // 9  ← RUNES (characters)
fmt.Println(utf8.RuneCountInString(s)) // 9 ← more efficient (no allocation)
```

**Why**: `len()` on a string is O(1) — it just reads the `len` field from the string header. There's no O(n) scan. But this length is in bytes.

### Pitfall 2: Byte Indexing into Multi-Byte Strings

```go
s := "世界"
fmt.Println(s[0])   // 228 (0xE4) — first BYTE of '世', not '世'!
fmt.Println(s[1])   // 184 (0xB8) — second BYTE of '世'
```

**Fix**:
```go
r := []rune(s)
fmt.Println(r[0])   // 19990 — rune value of '世'
fmt.Println(string(r[0]))  // "世"
```

### Pitfall 3: Modifying String via unsafe

```go
s := "hello"
// This is UNDEFINED BEHAVIOR:
b := (*[5]byte)(unsafe.Pointer(unsafe.StringData(s)))
b[0] = 'H'  // Potential SEGFAULT if s is in .rodata!
```

String literals are in read-only memory. Always copy first.

### Pitfall 4: Confusing Rune Index with Byte Index in range

```go
s := "A世B"
for i, r := range s {
    fmt.Println(i, r)
    // i is byte offset: 0, 1, 4 — NOT 0, 1, 2!
}
```

### Pitfall 5: String Comparison vs Byte Comparison

```go
// These are equivalent — string comparison is byte-by-byte
s1 := "hello"
s2 := "hello"
fmt.Println(s1 == s2)  // true — O(n) comparison of bytes
// But: if s1 and s2 are the same literal, compiler may short-circuit with pointer equality!
```

### Pitfall 6: Concatenation in Hot Loops

```go
// SLOW: O(n²) due to repeated allocation + copy
result := ""
for _, w := range words {
    result += w
}

// FAST: O(n) with pre-allocated Builder
var sb strings.Builder
sb.Grow(totalLength)  // pre-allocate if you know the size
for _, w := range words {
    sb.WriteString(w)
}
result := sb.String()
```

### Pitfall 7: fmt.Sprintf for String Building

```go
// SLOW: Uses reflection, interface boxing, format parsing
s := fmt.Sprintf("%s%s", a, b)

// FAST: Direct concatenation or Builder
s := a + b  // fine for just 2 strings
```

### Pitfall 8: The Substring Memory Retention Problem (Revisited)

```go
func extractFirstLine(bigLog string) string {
    idx := strings.Index(bigLog, "\n")
    if idx < 0 {
        return bigLog
    }
    return bigLog[:idx]  // LEAK: retains entire bigLog in memory!
}

// Fix:
func extractFirstLine(bigLog string) string {
    idx := strings.Index(bigLog, "\n")
    if idx < 0 {
        return strings.Clone(bigLog)
    }
    return strings.Clone(bigLog[:idx])  // independent copy
}
```

---

## 22. Mental Models Summary

Here are the precise mental models to carry in your head:

### Model 1: A String Is a (Pointer, Length) Pair

```
string = struct { ptr *byte; len int }

This is IT. Nothing else. 16 bytes on 64-bit.
The ptr can point anywhere: .rodata, heap, stack.
The len is in BYTES, not characters.
```

### Model 2: Bytes Are the Ground Truth

```
A string stores BYTES.
"Length" means BYTE count.
Indexing gives BYTES.
Slicing operates on BYTE boundaries.
You must opt-in to rune/character awareness.
```

### Model 3: UTF-8 Is Variable-Width

```
1 character ≠ 1 byte (in general)
1 character = 1, 2, 3, or 4 bytes (in UTF-8)

ASCII (U+0000–U+007F):  always 1 byte
Latin extended:          1–2 bytes
CJK, Arabic, Hebrew:    3 bytes
Emoji, rare scripts:    4 bytes
```

### Model 4: Rune Is Just int32

```
rune = int32 = Unicode code point as an integer
'世' = rune(19990) = int32(0x4E16)

A []rune is a []int32 where each element holds a code point.
Always 4 bytes per element, regardless of the character.
UTF-8 is more space-efficient for ASCII-heavy text.
```

### Model 5: Conversions Always Copy (Unless Optimized Away)

```
string → []byte:  allocate heap, memmove, new slice header
string → []rune:  allocate heap, UTF-8 decode, new slice header  
[]byte → string:  allocate heap, memmove, new string header
[]rune → string:  allocate heap, UTF-8 encode, new string header

Exceptions (compiler-optimized, no copy):
  - string(b) used only for comparison
  - string(b) used only as map key
  - []byte(s) in for range
```

### Model 6: The Cost Hierarchy

```
Operation                     Relative Cost
──────────────────────────────────────────────────────
len(s)                        O(1)  — reads length field
s[i]                          O(1)  — pointer arithmetic
s[i:j]                        O(1)  — new header, no copy
s1 == s2                      O(n)  — byte-by-byte compare
string(b []byte)              O(n)  — copy n bytes
[]byte(s)                     O(n)  — copy n bytes
[]rune(s)                     O(n)  — decode n bytes → m runes (m ≤ n)
string(r []rune)              O(m)  — encode m runes → UTF-8 bytes
strings.Builder.String()      O(1)  — zero copy (unsafe trick)
for range s                   O(n)  — UTF-8 decode, no allocation
utf8.RuneCountInString(s)     O(n)  — scan bytes, count rune starts
strings.Clone(s)              O(n)  — copy n bytes
```

### Model 7: The Immutability Guarantee Comes From the Type System

```
string type → compiler enforces read-only access
[]byte type → read/write access to bytes

These have the SAME memory layout for their data portion,
but the type system exposes different operations.

string:  s[i] → byte (read only, compiler rejects s[i] = x)
[]byte:  b[i] → byte (read/write, b[i] = x is legal)

The immutability is NOT enforced by the hardware/OS for heap strings.
(OS protects .rodata pages, but heap strings are mutable via unsafe)
Immutability is a LANGUAGE-LEVEL CONTRACT.
```

---

## Complete Reference Implementation

```go
package main

import (
    "fmt"
    "strings"
    "unicode/utf8"
    "unsafe"
)

// ─────────────────────────────────────────────
// 1. String header inspection
// ─────────────────────────────────────────────

type stringHeader struct {
    Data uintptr
    Len  int
}

func getStringHeader(s string) stringHeader {
    return *(*stringHeader)(unsafe.Pointer(&s))
}

// ─────────────────────────────────────────────
// 2. Manual UTF-8 encoder
// ─────────────────────────────────────────────

func encodeRuneManual(r rune) []byte {
    switch {
    case r < 0x80:
        return []byte{byte(r)}
    case r < 0x800:
        return []byte{
            byte(0xC0 | (r >> 6)),
            byte(0x80 | (r & 0x3F)),
        }
    case r < 0x10000:
        return []byte{
            byte(0xE0 | (r >> 12)),
            byte(0x80 | ((r >> 6) & 0x3F)),
            byte(0x80 | (r & 0x3F)),
        }
    default:
        return []byte{
            byte(0xF0 | (r >> 18)),
            byte(0x80 | ((r >> 12) & 0x3F)),
            byte(0x80 | ((r >> 6) & 0x3F)),
            byte(0x80 | (r & 0x3F)),
        }
    }
}

// ─────────────────────────────────────────────
// 3. Manual UTF-8 decoder
// ─────────────────────────────────────────────

func decodeRuneManual(b []byte) (rune, int) {
    if len(b) == 0 {
        return utf8.RuneError, 0
    }
    b0 := b[0]
    switch {
    case b0 < 0x80: // 1-byte (ASCII)
        return rune(b0), 1
    case b0 < 0xE0: // 2-byte
        if len(b) < 2 {
            return utf8.RuneError, 1
        }
        return rune(b0&0x1F)<<6 | rune(b[1]&0x3F), 2
    case b0 < 0xF0: // 3-byte
        if len(b) < 3 {
            return utf8.RuneError, 1
        }
        return rune(b0&0x0F)<<12 | rune(b[1]&0x3F)<<6 | rune(b[2]&0x3F), 3
    default: // 4-byte
        if len(b) < 4 {
            return utf8.RuneError, 1
        }
        return rune(b0&0x07)<<18 | rune(b[1]&0x3F)<<12 |
            rune(b[2]&0x3F)<<6 | rune(b[3]&0x3F), 4
    }
}

// ─────────────────────────────────────────────
// 4. Efficient string builder (like strings.Builder)
// ─────────────────────────────────────────────

type EfficientBuilder struct {
    buf []byte
}

func (eb *EfficientBuilder) WriteString(s string) {
    eb.buf = append(eb.buf, s...)
}

func (eb *EfficientBuilder) WriteRune(r rune) {
    var tmp [4]byte
    n := utf8.EncodeRune(tmp[:], r)
    eb.buf = append(eb.buf, tmp[:n]...)
}

func (eb *EfficientBuilder) String() string {
    // Zero-copy: create string header pointing to buf's backing array
    return unsafe.String(unsafe.SliceData(eb.buf), len(eb.buf))
}

func (eb *EfficientBuilder) Len() int {
    return len(eb.buf)
}

// ─────────────────────────────────────────────
// 5. Character-aware string operations
// ─────────────────────────────────────────────

// RuneAt returns the rune at rune-index i (O(n) — must scan from start)
func RuneAt(s string, runeIdx int) (rune, bool) {
    i := 0
    for _, r := range s {
        if i == runeIdx {
            return r, true
        }
        i++
    }
    return 0, false
}

// SubstringByRunes returns s[start:end] in rune terms (O(n))
func SubstringByRunes(s string, start, end int) string {
    runes := []rune(s)
    if start < 0 || end > len(runes) || start > end {
        return ""
    }
    return string(runes[start:end])
}

// ReverseString reverses a string by rune (handles multi-byte correctly)
func ReverseString(s string) string {
    runes := []rune(s)
    for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
        runes[i], runes[j] = runes[j], runes[i]
    }
    return string(runes)
}

// ─────────────────────────────────────────────
// 6. Zero-copy conversions (unsafe!)
// ─────────────────────────────────────────────

// BytesToStringNoCopy converts []byte to string without allocation.
// SAFETY: The returned string must not outlive b, and b must not be modified.
func BytesToStringNoCopy(b []byte) string {
    if len(b) == 0 {
        return ""
    }
    return unsafe.String(unsafe.SliceData(b), len(b))
}

// StringToBytesNoCopy converts string to []byte without allocation.
// SAFETY: The returned []byte must not be modified!
func StringToBytesNoCopy(s string) []byte {
    if s == "" {
        return nil
    }
    return unsafe.Slice(unsafe.StringData(s), len(s))
}

// ─────────────────────────────────────────────
// 7. String analysis tool
// ─────────────────────────────────────────────

type StringAnalysis struct {
    Value      string
    ByteLen    int
    RuneCount  int
    IsASCII    bool
    IsValidUTF8 bool
    Bytes      []byte
    Runes      []rune
    RuneInfo   []RuneInfo
}

type RuneInfo struct {
    ByteIndex  int
    RuneIndex  int
    Rune       rune
    UTF8Bytes  []byte
    ByteSize   int
}

func AnalyzeString(s string) StringAnalysis {
    a := StringAnalysis{
        Value:       s,
        ByteLen:     len(s),
        IsValidUTF8: utf8.ValidString(s),
        Bytes:       []byte(s),
        Runes:       []rune(s),
    }
    a.RuneCount = len(a.Runes)
    a.IsASCII = a.ByteLen == a.RuneCount

    runeIdx := 0
    for byteIdx := 0; byteIdx < len(s); {
        r, size := utf8.DecodeRuneInString(s[byteIdx:])
        info := RuneInfo{
            ByteIndex: byteIdx,
            RuneIndex: runeIdx,
            Rune:      r,
            UTF8Bytes: []byte(s[byteIdx : byteIdx+size]),
            ByteSize:  size,
        }
        a.RuneInfo = append(a.RuneInfo, info)
        byteIdx += size
        runeIdx++
    }
    return a
}

func (a StringAnalysis) Print() {
    fmt.Printf("String: %q\n", a.Value)
    fmt.Printf("  Byte length:   %d\n", a.ByteLen)
    fmt.Printf("  Rune count:    %d\n", a.RuneCount)
    fmt.Printf("  Is ASCII:      %v\n", a.IsASCII)
    fmt.Printf("  Valid UTF-8:   %v\n", a.IsValidUTF8)
    fmt.Printf("  Bytes (hex):   ")
    for _, b := range a.Bytes {
        fmt.Printf("%02X ", b)
    }
    fmt.Println()
    fmt.Printf("  Rune details:\n")
    for _, ri := range a.RuneInfo {
        fmt.Printf("    rune[%d] byte[%d]: %c (U+%04X) → UTF-8: ",
            ri.RuneIndex, ri.ByteIndex, ri.Rune, ri.Rune)
        for _, b := range ri.UTF8Bytes {
            fmt.Printf("%02X ", b)
        }
        fmt.Printf("(%d byte(s))\n", ri.ByteSize)
    }
}

// ─────────────────────────────────────────────
// Main: demonstrate everything
// ─────────────────────────────────────────────

func main() {
    fmt.Println("════════════════════════════════════════")
    fmt.Println("  Go String / Rune / Byte Deep Dive")
    fmt.Println("════════════════════════════════════════")
    
    // 1. String header
    s := "hello, 世界"
    h := getStringHeader(s)
    fmt.Printf("\n[1] String Header\n")
    fmt.Printf("    ptr=0x%X len=%d\n", h.Data, h.Len)
    
    // 2. Byte vs Rune
    fmt.Printf("\n[2] Byte vs Rune counts\n")
    fmt.Printf("    len(s)=%d bytes, %d runes\n", len(s), utf8.RuneCountInString(s))
    
    // 3. Manual encode/decode
    fmt.Printf("\n[3] Manual UTF-8 encode/decode\n")
    for _, r := range []rune{'A', '€', '世', '😀'} {
        encoded := encodeRuneManual(r)
        decoded, size := decodeRuneManual(encoded)
        fmt.Printf("    %c (U+%04X) → %X → decoded=%c size=%d\n",
            r, r, encoded, decoded, size)
    }
    
    // 4. Builder
    fmt.Printf("\n[4] EfficientBuilder\n")
    var eb EfficientBuilder
    eb.WriteString("hello")
    eb.WriteString(", ")
    eb.WriteRune('世')
    eb.WriteRune('界')
    fmt.Printf("    Built: %q (len=%d)\n", eb.String(), eb.Len())
    
    // 5. String analysis
    fmt.Printf("\n[5] String Analysis\n")
    AnalyzeString("A€世😀").Print()
    
    // 6. Zero-copy
    fmt.Printf("\n[6] Zero-Copy Conversions\n")
    b := []byte("hello world")
    zs := BytesToStringNoCopy(b)
    fmt.Printf("    []byte→string (no copy): %q\n", zs)
    // DO NOT modify b after this point if zs is still in use!
    
    // 7. Substring memory model
    fmt.Printf("\n[7] Substring sharing\n")
    big := strings.Repeat("x", 100) + "hello" + strings.Repeat("y", 100)
    sub := big[100:105]  // "hello"
    subClone := strings.Clone(sub)  // independent copy
    hBig := getStringHeader(big)
    hSub := getStringHeader(sub)
    hClone := getStringHeader(subClone)
    fmt.Printf("    big ptr:   0x%X\n", hBig.Data)
    fmt.Printf("    sub ptr:   0x%X (offset %d from big)\n",
        hSub.Data, hSub.Data-hBig.Data)
    fmt.Printf("    clone ptr: 0x%X (independent!)\n", hClone.Data)
    
    // 8. Rune operations
    fmt.Printf("\n[8] Character-Aware Operations\n")
    s2 := "hello, 世界"
    r0, _ := RuneAt(s2, 7)   // '世'
    fmt.Printf("    RuneAt(7): %c\n", r0)
    fmt.Printf("    SubstringByRunes(7,9): %q\n", SubstringByRunes(s2, 7, 9))
    fmt.Printf("    Reverse: %q\n", ReverseString(s2))
}
```

---

## Quick Reference Cheat Sheet

```
┌──────────────────────────────────────────────────────────────────────────┐
│               GO STRING / RUNE / BYTE CHEAT SHEET                        │
├──────────────────────┬───────────────────────────────────────────────────┤
│ Type                 │ Internals                                          │
├──────────────────────┼───────────────────────────────────────────────────┤
│ string               │ {ptr *byte, len int} — 16 bytes on 64-bit         │
│ []byte               │ {ptr *byte, len int, cap int} — 24 bytes          │
│ []rune               │ {ptr *int32, len int, cap int} — 24 bytes         │
│ rune                 │ int32 — 4 bytes — Unicode code point              │
│ byte                 │ uint8 — 1 byte — raw byte value                   │
├──────────────────────┼───────────────────────────────────────────────────┤
│ Operation            │ Notes                                              │
├──────────────────────┼───────────────────────────────────────────────────┤
│ len(s)               │ byte count — O(1)                                 │
│ s[i]                 │ i-th BYTE — O(1)                                  │
│ s[i:j]               │ substring — O(1) — no copy                        │
│ s1 + s2              │ new allocation + copy — O(n)                      │
│ []byte(s)            │ new alloc + memcopy — O(n)                        │
│ []rune(s)            │ new alloc + UTF-8 decode — O(n)                   │
│ string(b)            │ new alloc + memcopy — O(n)                        │
│ string(r)            │ new alloc + UTF-8 encode — O(n)                   │
│ for i,r := range s   │ UTF-8 decode in-place — O(n) — no alloc           │
├──────────────────────┼───────────────────────────────────────────────────┤
│ UTF-8 Sizes          │                                                    │
├──────────────────────┼───────────────────────────────────────────────────┤
│ U+0000–U+007F        │ 1 byte  (ASCII)                                   │
│ U+0080–U+07FF        │ 2 bytes (Latin ext, Greek, Arabic, Hebrew...)     │
│ U+0800–U+FFFF        │ 3 bytes (CJK, most other scripts)                 │
│ U+10000–U+10FFFF     │ 4 bytes (Emoji, rare scripts, ancient chars)      │
├──────────────────────┼───────────────────────────────────────────────────┤
│ When to Use          │ Why                                                │
├──────────────────────┼───────────────────────────────────────────────────┤
│ string               │ Store/pass text — immutable, shareable            │
│ []byte               │ Modify bytes, I/O, network, file operations       │
│ []rune               │ Character-level operations (index, reverse, etc.) │
│ strings.Builder      │ Build strings in loops — amortized O(n)           │
│ for range s          │ Iterate characters (runes) — no allocation        │
│ utf8.RuneCount...    │ Count chars without []rune allocation              │
│ strings.Clone        │ Break substring reference to backing array        │
└──────────────────────┴───────────────────────────────────────────────────┘
```

---

*This guide covers the complete lifecycle of Go strings from binary representation through runtime conversions, compiler optimizations, and GC interactions. The mental models in Section 22 are the foundation — once these are solid, all the specific behaviors follow naturally.*
