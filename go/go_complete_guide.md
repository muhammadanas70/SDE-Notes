# The Complete Go Programming Mastery Guide
## Rules, Regulations, Mental Models, and Deep Concepts

> **Purpose**: Build an iron-clad mental model of Go — every rule, every regulation, every concept explained in depth with ASCII architecture diagrams and real Go code. Read this once correctly, and you will think in Go.

---

## Table of Contents

1. [Go Philosophy & Design Principles](#1-go-philosophy--design-principles)
2. [Toolchain, Workspace & Module System](#2-toolchain-workspace--module-system)
3. [Go Program Structure](#3-go-program-structure)
4. [Lexical Elements & Syntax Rules](#4-lexical-elements--syntax-rules)
5. [Types — The Complete System](#5-types--the-complete-system)
6. [Variables, Constants & Zero Values](#6-variables-constants--zero-values)
7. [Operators & Expressions](#7-operators--expressions)
8. [Control Flow — Complete Rules](#8-control-flow--complete-rules)
9. [Functions — Deep Dive](#9-functions--deep-dive)
10. [Arrays & Slices — Internal Architecture](#10-arrays--slices--internal-architecture)
11. [Maps — Internal Architecture](#11-maps--internal-architecture)
12. [Structs — Complete Rules](#12-structs--complete-rules)
13. [Pointers — Memory Mental Model](#13-pointers--memory-mental-model)
14. [Interfaces — Implicit Contract System](#14-interfaces--implicit-contract-system)
15. [Error Handling — The Go Way](#15-error-handling--the-go-way)
16. [Goroutines — Concurrency Architecture](#16-goroutines--concurrency-architecture)
17. [Channels — Communication Rules](#17-channels--communication-rules)
18. [Sync Package — Synchronization Primitives](#18-sync-package--synchronization-primitives)
19. [Packages & Modules — Organization Rules](#19-packages--modules--organization-rules)
20. [Standard Library — Essential Packages](#20-standard-library--essential-packages)
21. [Generics — Type Parameters](#21-generics--type-parameters)
22. [Memory Model, Stack & Heap](#22-memory-model-stack--heap)
23. [Reflection](#23-reflection)
24. [Context Package](#24-context-package)
25. [Testing — Complete Rules](#25-testing--complete-rules)
26. [Build System, Tags & Tools](#26-build-system-tags--tools)
27. [Idiomatic Go Patterns](#27-idiomatic-go-patterns)
28. [Performance Engineering](#28-performance-engineering)
29. [The Go Runtime Scheduler](#29-the-go-runtime-scheduler)
30. [Common Pitfalls & Rules to Never Break](#30-common-pitfalls--rules-to-never-break)

---

## 1. Go Philosophy & Design Principles

### 1.1 Why Go Was Created

Go (also called Golang) was created at Google in 2007 by Robert Griesemer, Rob Pike, and Ken Thompson. It was born from frustration with existing languages at Google-scale:

- **C++** compiled slowly, was excessively complex, had manual memory management
- **Java** had bloated runtimes, verbose boilerplate, and the JVM startup overhead
- **Python** was too slow for systems programming
- **JavaScript/Node** had no strong typing and poor concurrency primitives

The goal: a language with **C-like performance**, **Python-like readability**, and **first-class concurrency**.

### 1.2 The Core Design Tenets

```
┌─────────────────────────────────────────────────────────────────┐
│                     GO DESIGN PHILOSOPHY                        │
│                                                                 │
│  SIMPLICITY        COMPOSABILITY        ORTHOGONALITY           │
│  ──────────        ─────────────        ─────────────           │
│  Only 25            Interfaces not      Features combine        │
│  keywords           inheritance         independently           │
│                                                                 │
│  EXPLICITNESS      SAFETY              PERFORMANCE              │
│  ────────────      ──────              ───────────              │
│  No magic,          GC + bounds         Compiles to             │
│  no hidden          checking +          native binary,          │
│  control flow       type safety         goroutines cheap        │
│                                                                 │
│  READABILITY       PRAGMATISM                                   │
│  ────────────      ──────────                                   │
│  Code is read      "Good enough"                                │
│  more than         trumps theoretical                           │
│  written           purity                                       │
└─────────────────────────────────────────────────────────────────┘
```

### 1.3 The Fundamental Rules of Go Thinking

**Rule 1 — Composition over Inheritance**  
Go has NO class hierarchy. NO `extends`. NO `implements` keyword. Behavior is composed through embedding and interfaces.

**Rule 2 — Errors are Values**  
Errors are not exceptions thrown up the stack. They are plain return values you handle immediately at each call site.

**Rule 3 — Interfaces are Implicit**  
A type satisfies an interface simply by having the right methods. No declaration of intent needed.

**Rule 4 — Concurrency is Not Parallelism**  
Goroutines are about *structuring* computation. Whether they run in parallel depends on GOMAXPROCS and available CPUs.

**Rule 5 — The Compiler Enforces Clarity**  
Unused imports = compile error. Unused variables = compile error. Exported names = capitalized. These are laws, not suggestions.

**Rule 6 — gofmt is Canonical**  
There is one formatting style. Use `gofmt`. Debate over tabs vs spaces is over.

---

## 2. Toolchain, Workspace & Module System

### 2.1 Go Installation Layout

```
GOROOT (the Go installation)
├── bin/
│   ├── go          ← The go toolchain binary
│   └── gofmt       ← The formatter
├── src/            ← Standard library source
├── pkg/            ← Compiled standard packages
└── lib/

GOPATH (legacy workspace, still relevant)
├── src/            ← Your source code (legacy)
├── pkg/            ← Compiled packages cache
└── bin/            ← Installed binaries (go install)

MODULE WORKSPACE (modern, go 1.11+)
myproject/
├── go.mod          ← Module definition
├── go.sum          ← Cryptographic checksums
├── main.go
└── internal/
    └── pkg/
```

### 2.2 The go.mod File — Every Rule

```go
module github.com/yourname/yourproject  // Module path (canonical import path)

go 1.22  // Minimum Go version required

require (
    github.com/gin-gonic/gin v1.9.1    // Direct dependency
    golang.org/x/sync v0.6.0           // Direct dependency
)

require (
    github.com/bytedance/sonic v1.11.3 // Indirect dependency (indirect tag)
    // ...
) 

replace github.com/old/pkg => ./local/pkg  // Replace with local version

exclude github.com/bad/pkg v1.2.3           // Exclude specific version
```

**Rule**: The module path in `go.mod` is the canonical import prefix for all packages in this module. If your module is `github.com/foo/bar`, then a package in `./internal/util` is imported as `github.com/foo/bar/internal/util`.

### 2.3 Essential go Commands — Complete Reference

```bash
# Project initialization
go mod init github.com/yourname/project  # Create go.mod

# Building
go build ./...                # Build all packages
go build -o mybin ./cmd/app   # Build with output name
go build -v ./...             # Verbose: show packages being built
go build -race ./...          # Build with race detector

# Running
go run main.go                # Compile and run (no binary kept)
go run ./cmd/app/             # Run package directory

# Testing
go test ./...                 # Test all packages
go test -v ./...              # Verbose output
go test -run TestFoo ./...    # Run only matching tests
go test -bench=. ./...        # Run benchmarks
go test -cover ./...          # Show coverage
go test -race ./...           # Test with race detector
go test -count=1 ./...        # Disable test caching

# Code quality
go vet ./...                  # Static analysis (finds bugs)
gofmt -w .                    # Format all files in place
goimports -w .                # Format + fix imports

# Dependency management
go get github.com/pkg/foo     # Add/update dependency
go get github.com/pkg/foo@v1.2.3  # Specific version
go get github.com/pkg/foo@latest  # Latest version
go mod tidy                   # Remove unused, add missing deps
go mod download               # Download deps to module cache
go mod vendor                 # Copy deps into vendor/
go mod verify                 # Verify checksums
go mod graph                  # Print dependency graph
go mod why github.com/pkg     # Explain why dep is needed

# Inspection
go list -m all                # List all modules
go list ./...                 # List all packages
go doc fmt.Println            # Show documentation
go env                        # Show Go environment variables

# Installation
go install github.com/tool@latest  # Install binary tool
```

### 2.4 Critical Environment Variables

```bash
GOROOT=/usr/local/go           # Where Go is installed (auto-set)
GOPATH=$HOME/go                # Workspace root (go install destination)
GOMODCACHE=$GOPATH/pkg/mod     # Module cache location
GOPROXY=https://proxy.golang.org,direct  # Module proxy
GONOSUMCHECK=                  # Bypass checksum verification
GOPRIVATE=github.com/yourcompany/*  # Private modules (no proxy)
GOFLAGS=-mod=vendor            # Default flags for go commands
GOOS=linux                     # Target OS for cross-compilation
GOARCH=amd64                   # Target architecture
CGO_ENABLED=0                  # Disable C interop (static binary)
GOMAXPROCS=8                   # Max OS threads (defaults to CPU count)
GODEBUG=gctrace=1              # GC tracing
GOGC=100                       # GC target percentage (default 100)
```

### 2.5 The Module Cache Architecture

```
$GOPATH/pkg/mod/
├── cache/
│   └── download/
│       └── github.com/
│           └── gin-gonic/
│               └── gin/
│                   ├── @v/
│                   │   ├── v1.9.1.info    ← version metadata
│                   │   ├── v1.9.1.mod     ← go.mod for this version
│                   │   └── v1.9.1.zip     ← source archive
│                   └── list               ← known versions
└── github.com/
    └── gin-gonic/
        └── gin@v1.9.1/        ← extracted (read-only)
            └── ...
```

**Rule**: Module cache entries are **read-only**. You cannot modify them. If you need to patch a dependency, use `replace` in `go.mod`.

---

## 3. Go Program Structure

### 3.1 Anatomy of a Go File

```go
// 1. BUILD TAGS (optional, must be at top before package)
//go:build linux && amd64

// 2. PACKAGE DECLARATION (mandatory, first non-comment line)
package main

// 3. IMPORT BLOCK
import (
    "fmt"               // Standard library
    "os"

    "github.com/foo/bar"  // Third-party (separated by blank line, conventional)

    "mymodule/internal/util"  // Internal package
)

// 4. PACKAGE-LEVEL DECLARATIONS (any order)
const Version = "1.0.0"

var globalCounter int

type Config struct {
    Debug bool
    Port  int
}

// 5. INIT FUNCTION (optional, can have multiple, runs before main)
func init() {
    // Package initialization code
}

// 6. MAIN FUNCTION (required for package main)
func main() {
    fmt.Println("Hello, Go")
}

// 7. OTHER FUNCTIONS
func helper() {}
```

### 3.2 Package Naming Rules

```
RULE 1: Package name = last element of import path
        import "net/http" → package http
        import "encoding/json" → package json

RULE 2: Package name must be lowercase, single word (no underscores)
        GOOD: package myutil
        BAD:  package MyUtil
        BAD:  package my_util

RULE 3: Package main is special — it defines an executable entry point
        Any other package name = library package

RULE 4: File names don't matter to the compiler — all .go files in
        the same directory belong to the same package

RULE 5: test files end in _test.go and belong to either:
        - Same package: package myutil       (white-box testing)
        - External test: package myutil_test (black-box testing)

RULE 6: Internal packages (path contains /internal/) can only be
        imported by code rooted at the parent of internal/
```

### 3.3 Import Rules — Complete

```go
// BLANK IMPORT: import for side effects only (runs init())
import _ "database/sql/driver"

// DOT IMPORT: imports all exported names into current scope (avoid!)
import . "fmt"
// Now you can write: Println() instead of fmt.Println()

// ALIASED IMPORT: rename to avoid collision or for convenience
import (
    myfmt "fmt"
    "math/rand"
    crand "crypto/rand"  // avoid collision with math/rand
)

// RULE: Every imported package MUST be used
// This causes a compile error:
import "fmt"   // if fmt is never used in the file → compile error

// RULE: Import cycle is not allowed
// Package A cannot import Package B if Package B imports Package A
```

### 3.4 Program Execution Order

```
┌─────────────────────────────────────────────────────────────┐
│                  PROGRAM STARTUP SEQUENCE                   │
│                                                             │
│  1. All imported packages initialize in dependency order    │
│     ┌──────────────────────────────────────┐               │
│     │ For each imported package (deepest   │               │
│     │ dependency first):                   │               │
│     │   a. Package-level vars evaluated    │               │
│     │   b. init() functions run (in        │               │
│     │      source file order, then by      │               │
│     │      file name alphabetically)       │               │
│     └──────────────────────────────────────┘               │
│                                                             │
│  2. Package main's init() runs                              │
│                                                             │
│  3. main() function runs                                    │
│                                                             │
│  NOTE: init() cannot be called explicitly                   │
│  NOTE: A package can have multiple init() functions         │
│  NOTE: Same file can have multiple init() functions         │
└─────────────────────────────────────────────────────────────┘
```

```go
package main

import "fmt"

var x = compute()  // Step 1: package-level var

func compute() int {
    fmt.Println("1. compute() called")
    return 42
}

func init() {
    fmt.Println("2. init() called, x =", x)
}

func main() {
    fmt.Println("3. main() called")
}

// Output:
// 1. compute() called
// 2. init() called, x = 42
// 3. main() called
```

---

## 4. Lexical Elements & Syntax Rules

### 4.1 The 25 Keywords — Memorize All

```
break        default      func         interface    select
case         defer        go           map          struct
chan         else         goto         package      switch
const        fallthrough  if           range        type
continue     for          import       return       var
```

That's it. 25 keywords. No `class`, no `extends`, no `implements`, no `try`, no `catch`, no `finally`, no `while`, no `do`.

### 4.2 Predeclared Identifiers (Not Keywords — Can Be Shadowed)

```
Types:
    bool byte complex64 complex128 error float32 float64
    int int8 int16 int32 int64 rune string
    uint uint8 uint16 uint32 uint64 uintptr any comparable

Constants:
    true false iota

Zero value:
    nil

Functions:
    append cap clear close complex copy delete imag len
    make new panic print println real recover
```

**Critical Rule**: These are NOT keywords. They CAN be shadowed (though shadowing them is terrible practice):

```go
// Legal but horrifying — don't do this:
func main() {
    len := 5    // shadows builtin len
    _ = len     // now len() function is inaccessible in this scope
}
```

### 4.3 Automatic Semicolon Insertion Rules

Go's lexer automatically inserts a semicolon `;` after a line's last token if that token is:
- An identifier (including keywords like `break`, `continue`, `fallthrough`, `return`)
- An integer, float, imaginary, rune, or string literal
- One of: `++`, `--`, `)`, `]`, `}`

**This rule drives all formatting conventions:**

```go
// CORRECT: opening brace MUST be on same line as statement
func foo() {        // ✓ - no semicolon inserted after ()
}

func foo()          // ✗ - semicolon inserted after (), then { on next line
{                   //     this is a syntax error
}

// CORRECT: if/else must keep braces adjacent
if x > 0 {
    // ...
} else {            // ✓ - } and else on same line
    // ...
}

if x > 0 {
    // ...
}                   // ← semicolon inserted here
else {              // ✗ - SYNTAX ERROR: unexpected else
}
```

### 4.4 Identifiers — Naming Rules

```
RULE 1: Must start with letter (Unicode) or underscore
RULE 2: Can contain letters, digits, underscores
RULE 3: Case-sensitive: Foo ≠ foo ≠ FOO
RULE 4: EXPORTED (public): starts with uppercase letter
RULE 5: UNEXPORTED (package-private): starts with lowercase letter
RULE 6: Blank identifier: _ (special — discards values, avoids "unused" errors)

Conventions (enforced by gofmt/golint):
  - Use camelCase: myVariable, computeHash
  - No underscores in names: NOT my_variable (except test funcs: Test_xxx)
  - Short names for short-lived: i, j, n, ok, err, v
  - Acronyms in all-caps: HTTP, URL, ID (HTTPServer, not HttpServer)
  - Interface names: often end in -er: Reader, Writer, Stringer
  - Single-method interface named after method: Read → Reader
```

### 4.5 Literals

```go
// INTEGER literals
42          // decimal
0b1010      // binary (Go 1.13+)
0o755       // octal (Go 1.13+)
0644        // octal (old style: leading 0)
0xFF        // hexadecimal
1_000_000   // underscores for readability (Go 1.13+)
0xDEAD_BEEF

// FLOAT literals
3.14
.5          // 0.5
1e10        // 10,000,000,000.0
1.5e-3      // 0.0015
0x1p-2      // hex float: 1 * 2^-2 = 0.25

// IMAGINARY literals (complex numbers)
3i
1.5i
1e3i

// RUNE literals (int32, Unicode code point)
'a'         // 97
'\n'        // newline (10)
'\t'        // tab (9)
'\\'        // backslash
'\''        // single quote
'\x41'      // hex: 'A'
'\u0041'    // Unicode: 'A'
'\U00000041'// Unicode long form: 'A'

// STRING literals
"Hello, World\n"    // interpreted string: escape sequences processed
`Hello, World\n`    // raw string: \n is two characters, not newline
                    // raw strings can span multiple lines
```

---

## 5. Types — The Complete System

### 5.1 Type Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                    GO TYPE SYSTEM                             │
│                                                               │
│  BASIC TYPES                                                  │
│  ├── Boolean: bool                                            │
│  ├── Numeric                                                  │
│  │   ├── Integer: int int8 int16 int32 int64                  │
│  │   │            uint uint8 uint16 uint32 uint64 uintptr     │
│  │   ├── Float: float32 float64                               │
│  │   └── Complex: complex64 complex128                        │
│  └── String: string                                           │
│                                                               │
│  COMPOSITE TYPES                                              │
│  ├── Array: [N]T                                              │
│  ├── Slice: []T                                               │
│  ├── Map: map[K]V                                             │
│  └── Struct: struct { fields }                                │
│                                                               │
│  REFERENCE TYPES                                              │
│  ├── Pointer: *T                                              │
│  ├── Function: func(params)(returns)                          │
│  ├── Interface: interface{ methods }                          │
│  ├── Slice: []T   (contains pointer to backing array)         │
│  ├── Map: map[K]V (contains pointer to hash table)            │
│  └── Channel: chan T                                          │
│                                                               │
│  SPECIAL TYPES                                                │
│  ├── byte = uint8  (alias)                                    │
│  ├── rune = int32  (alias, represents Unicode code point)     │
│  ├── any = interface{}  (alias, Go 1.18+)                     │
│  └── error = interface{ Error() string }                      │
└───────────────────────────────────────────────────────────────┘
```

### 5.2 Integer Types — Sizes and Ranges

```
Type      Size        Range
──────────────────────────────────────────────────────
int8      8-bit       -128 to 127
int16     16-bit      -32,768 to 32,767
int32     32-bit      -2,147,483,648 to 2,147,483,647
int64     64-bit      -9,223,372,036,854,775,808 to 9,223,372,036,854,775,807
uint8     8-bit       0 to 255
uint16    16-bit      0 to 65,535
uint32    32-bit      0 to 4,294,967,295
uint64    64-bit      0 to 18,446,744,073,709,551,615
int       platform    32-bit on 32-bit systems, 64-bit on 64-bit systems
uint      platform    same as int
uintptr   platform    large enough to hold any pointer

RULE: int and uint are NOT the same type as int64/uint64 even on 64-bit.
      You MUST explicitly convert between them.
```

```go
var a int = 10
var b int64 = 20
// b = a  ← COMPILE ERROR: cannot use a (type int) as type int64
b = int64(a)  // ← Correct: explicit conversion required
```

### 5.3 Type Conversion Rules

```go
// Go has NO implicit type conversion. All conversions are EXPLICIT.

var i int = 42
var f float64 = float64(i)   // int → float64
var u uint = uint(f)          // float64 → uint (truncates decimal)

// String conversions
var r rune = 'A'
s1 := string(r)               // rune → string: "A" (the character)
s2 := string(65)              // int → string: "A" (the character with code 65)
s3 := fmt.Sprintf("%d", 65)   // int → string: "65" (the decimal representation)
n, _ := strconv.Atoi("42")    // string → int: 42

// RULE: Converting int → string gives the CHARACTER, not the number string!
// Use strconv.Itoa(n) or fmt.Sprintf("%d", n) for numeric string representation

// Numeric truncation on narrowing conversion (no error, silent!)
var big int64 = 300
small := int8(big)  // 300 mod 256 = 44 (wraps around!)
// RULE: Go does NOT panic on overflow/truncation in explicit conversions
```

### 5.4 Type Declarations vs Type Aliases

```go
// TYPE DEFINITION: Creates a brand new type
type Celsius float64       // Celsius is a DISTINCT type from float64
type Fahrenheit float64    // Fahrenheit is a DISTINCT type from float64 AND Celsius

var c Celsius = 100
var f Fahrenheit = 100
// c = f  ← COMPILE ERROR: different types, even though underlying type is same
// c = Celsius(f)  ← OK: explicit conversion

// New type inherits the underlying type's operations (+, -, *, /)
// but NOT methods of the original named type

// TYPE ALIAS (Go 1.9+): Just another name for the same type
type MyInt = int   // MyInt IS int, they are completely interchangeable
var x MyInt = 5
var y int = x      // no conversion needed
```

### 5.5 String Internals

```
A Go string is an IMMUTABLE sequence of bytes (not characters!).
It is represented as a struct internally:

string header:
┌──────────────────┬─────────┐
│   pointer        │  length │
│   (to byte data) │ (bytes) │
└──────────────────┴─────────┘
        │
        ▼
┌───┬───┬───┬───┬───┬───┐
│ H │ e │ l │ l │ o │   │  ← Immutable byte array (UTF-8 encoded)
└───┴───┴───┴───┴───┴───┘
```

```go
s := "Hello, 世界"

// len() returns BYTES, not characters
len(s)  // = 13 (not 9!) — "世" and "界" are 3 bytes each in UTF-8

// Indexing gives a BYTE (uint8)
s[0]  // = 72 ('H') — a byte

// Range over string gives RUNES (Unicode code points)
for i, r := range s {
    fmt.Printf("index: %d, rune: %c, bytes: %d\n", i, r, len(string(r)))
}
// Notice: indices are byte offsets, not character positions
// 'H' at byte 0, 'e' at byte 1, ..., '世' at byte 7 (3-byte UTF-8 char)

// Convert to []rune for character-level access
runes := []rune(s)
len(runes)    // = 9 (actual characters)
runes[7]      // = '世'

// Strings are IMMUTABLE — you cannot write s[0] = 'h'
// To modify, convert to []byte, modify, convert back
b := []byte(s)
b[0] = 'h'
s2 := string(b)  // "hello, 世界"
```

---

## 6. Variables, Constants & Zero Values

### 6.1 Variable Declaration — All Forms

```go
// FORM 1: var with explicit type
var x int
var name string
var ok bool

// FORM 2: var with initializer (type inferred)
var x = 42           // x is int
var name = "Alice"   // name is string

// FORM 3: var with explicit type and initializer
var x int = 42

// FORM 4: Short variable declaration (inside functions only)
x := 42              // same as var x int = 42
name := "Alice"
ok := true

// FORM 5: Multiple variables in one statement
var x, y, z int              // all int, all zero
var a, b = 1, "hello"        // different types
x, y := 1, 2                 // short form
x, y = y, x                  // swap (no temp variable needed!)

// FORM 6: Block var declaration
var (
    host    = "localhost"
    port    = 8080
    timeout = 30 * time.Second
)

// RULE: := requires at least ONE NEW variable on the left
x := 1
x, y := 2, 3  // OK: y is new (x is reassigned)
x, x := 2, 3  // COMPILE ERROR: no new variables

// RULE: := is ONLY valid inside function bodies
// Package-level variables MUST use var
```

### 6.2 Zero Values — Complete Table

```
RULE: Every variable in Go is initialized to its zero value.
      There is NO uninitialized memory (unlike C).

Type              Zero Value
──────────────────────────────────────────────────
bool              false
int, int8..int64  0
uint, uint8..     0
float32, float64  0.0
complex64/128     0+0i
string            ""  (empty string)
pointer           nil
slice             nil (len=0, cap=0, no backing array)
map               nil (cannot write to nil map!)
channel           nil
function          nil
interface         nil
struct            all fields set to their zero values
array             all elements set to their zero values
```

```go
// CRITICAL: nil map vs empty map
var m map[string]int  // m == nil
m["key"] = 1          // PANIC: assignment to entry in nil map

m2 := map[string]int{}  // m2 != nil (empty but initialized)
m2["key"] = 1           // OK

// CRITICAL: nil slice vs empty slice
var s []int         // s == nil, len=0, cap=0
s = append(s, 1)    // OK: append handles nil slices gracefully

s2 := []int{}       // s2 != nil, but len=0, cap=0

// Testing for zero/nil:
var p *int
p == nil   // true

var i interface{}
i == nil   // true

// Typed nil TRAP:
var err *MyError = nil
var i2 interface{} = err
i2 == nil  // FALSE! Interface holds (type=*MyError, value=nil)
            // The interface is NOT nil because the type is set
```

### 6.3 Constants — Complete Rules

```go
// UNTYPED constant: has a "kind" but no specific type
// Can be used with any compatible type
const Pi = 3.14159           // untyped floating-point constant
const Big = 1 << 62          // untyped integer constant (can be larger than int64!)
const Message = "hello"      // untyped string constant

var f32 float32 = Pi         // OK: Pi is untyped, assigned to float32
var f64 float64 = Pi         // OK: same constant, different type
var i int = Big / 2          // OK if result fits in int

// TYPED constant: fixed type
const TypedPi float64 = 3.14159
var x float32 = TypedPi      // COMPILE ERROR: float64 cannot be used as float32

// IOTA: integer constant auto-incrementer (starts at 0 per const block)
const (
    StatusUnknown = iota   // 0
    StatusActive           // 1
    StatusInactive         // 2
    StatusDeleted          // 3
)

// IOTA with expressions
const (
    _  = iota              // 0 (skip)
    KB = 1 << (10 * iota)  // 1 << 10 = 1024
    MB                     // 1 << 20 = 1,048,576
    GB                     // 1 << 30 = 1,073,741,824
    TB                     // 1 << 40
)

// IOTA resets to 0 in each const block
const (
    A = iota  // 0
    B         // 1
)
const (
    C = iota  // 0 (new block, resets)
    D         // 1
)

// Constants are evaluated at COMPILE TIME
// They can be: boolean, numeric, string, character expressions
// NOT: runtime values (function calls, variables)
const x2 = len("hello")   // OK: len of string literal is compile-time known
// const y = os.Getenv("X")  // COMPILE ERROR: function call at runtime
```

---

## 7. Operators & Expressions

### 7.1 Operator Precedence (High to Low)

```
Precedence   Operators
─────────────────────────────────────────────
5 (highest)  *  /  %  <<  >>  &  &^
4            +  -  |  ^
3            ==  !=  <  <=  >  >=
2            &&
1 (lowest)   ||
```

```go
// Bitwise operators:
a & b    // AND
a | b    // OR
a ^ b    // XOR (binary), NOT (unary: ^a = bitwise complement)
a &^ b   // AND NOT (bit clear): clears bits in a where b has 1s
a << n   // left shift
a >> n   // right shift

// Examples
5 & 3     // 0101 & 0011 = 0001 = 1
5 | 3     // 0101 | 0011 = 0111 = 7
5 ^ 3     // 0101 ^ 0011 = 0110 = 6
^5        // bitwise NOT: ...11111010 (two's complement)
5 &^ 3    // 0101 &^ 0011 = 0100 = 4 (clear bits where 3 has 1s)
5 << 1    // 0101 << 1 = 1010 = 10
5 >> 1    // 0101 >> 1 = 0010 = 2
```

### 7.2 Increment/Decrement — Special Rules

```go
// In Go, ++ and -- are STATEMENTS, not expressions.
// They do NOT return a value. You cannot use them in expressions.

i := 0
i++      // OK: statement
i--      // OK: statement
// j := i++  // COMPILE ERROR: i++ is not an expression

// They can only be POSTFIX (no prefix ++ or --)
// ++i  // COMPILE ERROR: no prefix increment
```

### 7.3 Address & Dereference Operators

```go
x := 42
p := &x        // & = address-of: p is *int pointing to x
*p = 100       // * = dereference: write through pointer
fmt.Println(x) // 100

// Short-circuit evaluation applies to && and ||
// The right side is NOT evaluated if left determines result
if x != nil && x.Value > 0 { ... }  // Safe: if x is nil, right side skipped
```

---

## 8. Control Flow — Complete Rules

### 8.1 If Statement Rules

```go
// RULE: Condition must be a boolean expression (not int like C)
// if 1 { }  // COMPILE ERROR
// if n { }  // COMPILE ERROR (n is int)

// RULE: Braces are MANDATORY, even for single-statement bodies
// if x > 0  y = 1  // COMPILE ERROR (no braces)

// Standard form
if x > 0 {
    fmt.Println("positive")
} else if x < 0 {
    fmt.Println("negative")
} else {
    fmt.Println("zero")
}

// INIT STATEMENT form (extremely common in Go)
// Declares a variable scoped to the if/else blocks
if err := doSomething(); err != nil {
    return err
}
// err is NOT accessible here — scoped to the if statement

// More complex init form
if val, ok := m[key]; ok {
    fmt.Println("found:", val)
} else {
    fmt.Println("not found")
}
```

### 8.2 For Loop — All Three Forms

```go
// Go has ONLY ONE looping construct: for
// No while, do-while — all loops use 'for'

// FORM 1: C-style for loop
for i := 0; i < 10; i++ {
    fmt.Println(i)
}

// FORM 2: While-style (condition only)
n := 1
for n < 100 {
    n *= 2
}

// FORM 3: Infinite loop
for {
    if done() {
        break
    }
}

// FORM 4: Range over slice/array
for i, v := range []int{1, 2, 3} {
    fmt.Println(i, v)   // index, value
}

// Range: ignore index
for _, v := range []int{1, 2, 3} {
    fmt.Println(v)
}

// Range: index only
for i := range []int{1, 2, 3} {
    fmt.Println(i)
}

// FORM 5: Range over map
m := map[string]int{"a": 1, "b": 2}
for k, v := range m {
    fmt.Println(k, v)   // ORDER IS RANDOM — never assume map iteration order
}

// FORM 6: Range over string
for i, r := range "hello" {
    fmt.Printf("%d: %c\n", i, r)  // i=byte offset, r=rune
}

// FORM 7: Range over channel
for v := range ch {    // receives until channel is closed
    fmt.Println(v)
}

// FORM 8: Range over integer (Go 1.22+)
for i := range 5 {     // iterates 0, 1, 2, 3, 4
    fmt.Println(i)
}
```

### 8.3 Switch Statement Rules

```go
// RULE: No automatic fallthrough (unlike C)
// RULE: Cases don't need break (implicit at end of each case)
// RULE: Cases can have multiple values

switch x {
case 1, 2:
    fmt.Println("one or two")
case 3:
    fmt.Println("three")
default:
    fmt.Println("other")
}

// INIT STATEMENT form (same as if)
switch os := runtime.GOOS; os {
case "darwin":
    fmt.Println("macOS")
case "linux":
    fmt.Println("Linux")
}

// Expression-less switch (acts like if/else chain)
switch {
case x < 0:
    fmt.Println("negative")
case x == 0:
    fmt.Println("zero")
default:
    fmt.Println("positive")
}

// FALLTHROUGH: explicit opt-in to fall through to next case
switch x {
case 1:
    fmt.Println("one")
    fallthrough     // explicitly fall through
case 2:
    fmt.Println("one or two")  // executed if x==1 or x==2
}

// TYPE SWITCH (crucial for interfaces)
var i interface{} = "hello"
switch v := i.(type) {
case int:
    fmt.Println("int:", v)
case string:
    fmt.Println("string:", v)
case bool:
    fmt.Println("bool:", v)
default:
    fmt.Printf("unknown: %T\n", v)
}
```

### 8.4 Defer — Execution Model

```go
// RULE: defer pushes a function call onto a stack
// RULE: Deferred calls execute when the surrounding function RETURNS
// RULE: They execute in LIFO (last in, first out) order
// RULE: Deferred function arguments are evaluated IMMEDIATELY (not at deferred time)

func example() {
    defer fmt.Println("third")   // pushed first, runs last
    defer fmt.Println("second")  // pushed second, runs second
    defer fmt.Println("first")   // pushed last, runs first
    fmt.Println("before return")
}
// Output:
// before return
// first
// second
// third

// RULE: Defer runs even if the function panics
// (This is how recover() works)

// CRITICAL: Argument evaluation timing
x := 10
defer fmt.Println(x)  // x=10 is captured NOW, not when defer runs
x = 20
// Output: 10 (not 20!)

// EXCEPT: deferred function with closure captures variable by REFERENCE
x2 := 10
defer func() {
    fmt.Println(x2)  // x2 is captured by reference
}()
x2 = 20
// Output: 20 (the current value when defer executes)

// NAMED RETURN VALUES + DEFER: can modify return value
func doubleOrError(n int) (result int, err error) {
    defer func() {
        if err == nil {
            result *= 2  // modify named return value via defer!
        }
    }()
    result = n
    return  // naked return: returns current result and err
}
```

### 8.5 Goto, Break, Continue with Labels

```go
// Labels allow break/continue to target outer loops
outer:
for i := 0; i < 3; i++ {
    for j := 0; j < 3; j++ {
        if j == 1 {
            break outer    // breaks the OUTER loop, not just inner
        }
        if j == 0 {
            continue outer // continues the OUTER loop
        }
    }
}

// goto: unconditional jump (rarely used, avoid in practice)
func gotoExample() {
    i := 0
loop:
    if i < 5 {
        fmt.Println(i)
        i++
        goto loop  // jumps to label
    }
}
// RULE: goto cannot jump over variable declarations
// RULE: goto cannot jump into a block from outside it
```

---

## 9. Functions — Deep Dive

### 9.1 Function Anatomy

```go
// Full syntax:
func functionName(param1 type1, param2 type2) (return1 type3, return2 type4) {
    // body
    return val1, val2
}

// Multiple params of same type: can group
func add(x, y int) int {  // x and y are both int
    return x + y
}

// No return value
func printHello() {
    fmt.Println("hello")
}

// Single return value (no parentheses needed)
func square(n int) int {
    return n * n
}

// Multiple return values
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}

// NAMED return values (the variables are pre-declared)
func minMax(a, b int) (min, max int) {
    if a < b {
        return a, b  // explicit return still works
    }
    min = b          // named returns can be set like variables
    max = a
    return           // NAKED RETURN: returns current values of min, max
}
// RULE: Avoid naked returns in long functions (hurts readability)
```

### 9.2 Variadic Functions

```go
// Variadic parameter: receives zero or more arguments as a slice
func sum(nums ...int) int {
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}

sum(1, 2, 3)      // nums = []int{1, 2, 3}
sum()              // nums = []int{} (empty, not nil)

// SPREAD OPERATOR: pass slice to variadic
nums := []int{1, 2, 3}
sum(nums...)       // spreads the slice as individual arguments

// RULE: Variadic parameter must be the LAST parameter
// func f(a int, b ...string, c bool)  // COMPILE ERROR
// func f(a int, b ...string) {}       // OK
```

### 9.3 Functions as First-Class Values

```go
// Functions are values — can be assigned, passed, returned
func apply(nums []int, f func(int) int) []int {
    result := make([]int, len(nums))
    for i, n := range nums {
        result[i] = f(n)
    }
    return result
}

doubled := apply([]int{1, 2, 3}, func(n int) int {
    return n * 2
})

// Function types
type Transformer func(int) int

var double Transformer = func(n int) int { return n * 2 }
var triple Transformer = func(n int) int { return n * 3 }

funcs := []Transformer{double, triple}
funcs[0](5)  // 10
funcs[1](5)  // 15
```

### 9.4 Closures — Deep Rules

```go
// A closure is a function value that references variables from its outer scope.
// These variables OUTLIVE the outer function's return (they escape to heap).

func makeCounter() func() int {
    count := 0           // count lives on the heap (captured by closure)
    return func() int {
        count++          // references outer variable
        return count
    }
}

c1 := makeCounter()
c2 := makeCounter()
c1()  // 1
c1()  // 2
c2()  // 1  (c2 has its own independent count)

// CLOSURE CAPTURE TRAP (loop variable capture)
// Classic bug: capturing loop variable by reference

funcs := make([]func(), 5)
for i := 0; i < 5; i++ {
    funcs[i] = func() {
        fmt.Println(i)  // captures i by REFERENCE
    }
}
// When called, all functions print 5 (the final value of i after loop ends)!
funcs[0]()  // 5 (NOT 0!)
funcs[1]()  // 5 (NOT 1!)

// FIX 1: Create a new variable in each iteration
for i := 0; i < 5; i++ {
    i := i  // new i shadows outer i, each closure gets its own copy
    funcs[i] = func() { fmt.Println(i) }
}

// FIX 2: Pass as argument
for i := 0; i < 5; i++ {
    funcs[i] = func(n int) func() {
        return func() { fmt.Println(n) }
    }(i)  // i is passed by value as n
}

// Go 1.22+: Loop variable behavior changed — each iteration gets
// its own variable. The classic bug is fixed for range loops.
```

### 9.5 The init() Function

```go
// RULES for init():
// 1. Cannot be called explicitly — run automatically by runtime
// 2. Cannot be referenced by name
// 3. Takes no arguments, returns nothing
// 4. A package can have multiple init() functions (even in the same file)
// 5. Runs after all package-level variable initializations in the file
// 6. Runs before main() in package main
// 7. Multiple init() functions in same file: run in appearance order
// 8. Multiple init() functions across files: run in file name alphabetical order
//    (but dependency order takes precedence)

package mypackage

var db *sql.DB  // initialized before init() runs

func init() {
    var err error
    db, err = sql.Open("postgres", "connection_string")
    if err != nil {
        log.Fatal("cannot connect to database:", err)
    }
}
```

### 9.6 Panic and Recover

```go
// panic: stops normal execution, unwinds stack (running deferred functions)
// recover: captures a panic (ONLY useful inside deferred functions)

func safeDivide(a, b int) (result int, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("recovered from panic: %v", r)
        }
    }()
    return a / b, nil  // panics if b == 0
}

// recover() returns:
// - nil if there is no panic (or already recovered)
// - the value passed to panic() if there is one

// RULES:
// 1. panic() accepts any value (interface{})
// 2. recover() ONLY stops a panic if called in a deferred function
// 3. After recover(), execution does NOT resume at the point of panic —
//    the function returns normally from the deferred function
// 4. Panics propagate up goroutine stack; if they reach the top of a goroutine
//    without being recovered, the ENTIRE PROGRAM crashes

// WHEN TO USE PANIC:
// - Programming errors (impossible states, violated invariants)
// - During initialization (init() failures)
// - NOT for normal error handling (use error returns instead)
```

---

## 10. Arrays & Slices — Internal Architecture

### 10.1 Arrays — Value Type Semantics

```go
// Arrays are FIXED SIZE, VALUE TYPES
// Size is part of the type: [3]int ≠ [4]int

var a [5]int              // zero value: [0 0 0 0 0]
b := [3]string{"a","b","c"}
c := [...]int{1, 2, 3, 4} // ... = compiler counts elements (c is [4]int)

// ARRAY IS A VALUE: assignment copies all elements
x := [3]int{1, 2, 3}
y := x                  // y is a complete copy
y[0] = 99
fmt.Println(x[0])       // 1 (x is unchanged)

// Arrays are comparable (if element type is comparable)
[3]int{1,2,3} == [3]int{1,2,3}  // true

// Passing array to function: COPIES the whole array
// For large arrays, pass by pointer
func processArray(arr *[1000000]int) { ... }
```

### 10.2 Slice Internal Architecture

```
A slice is a VIEW into an underlying array. It is represented as a 3-field struct:

┌──────────────────────────────────────────────────────────┐
│                    SLICE HEADER (24 bytes on 64-bit)     │
│  ┌─────────────┬─────────────┬─────────────┐             │
│  │  ptr        │  len        │  cap        │             │
│  │  (8 bytes)  │  (8 bytes)  │  (8 bytes)  │             │
│  └──────┬──────┴─────────────┴─────────────┘             │
│         │                                                │
│         └──► Underlying Array:                           │
│              ┌───┬───┬───┬───┬───┬───┬───┬───┐          │
│              │ 0 │ 1 │ 2 │ 3 │ 4 │ 5 │ 6 │ 7 │          │
│              └───┴───┴───┴───┴───┴───┴───┴───┘          │
│                  ^───────────────^             ^         │
│                  ptr             ptr+len-1     ptr+cap-1 │
└──────────────────────────────────────────────────────────┘

len = number of elements accessible through this slice
cap = number of elements in underlying array from ptr to end
```

```go
// Creating slices
var s1 []int              // nil slice: ptr=nil, len=0, cap=0
s2 := []int{}             // empty slice: ptr≠nil, len=0, cap=0
s3 := []int{1, 2, 3}      // literal: len=3, cap=3
s4 := make([]int, 5)       // make(type, len): len=5, cap=5
s5 := make([]int, 3, 10)   // make(type, len, cap): len=3, cap=10

// Slicing operator: a[low:high:max]
a := []int{0, 1, 2, 3, 4, 5, 6, 7}

b := a[2:5]   // ptr→a[2], len=3, cap=6  (shares memory with a!)
              // Elements: [2, 3, 4]

b[0] = 99    // MODIFIES a[2] as well! They share the array.
fmt.Println(a[2])  // 99

// Three-index slice: a[low:high:max] — limits capacity
c := a[2:5:7] // ptr→a[2], len=3, cap=5
              // Prevents c from accessing a[7] even via append

// OMITTING INDICES:
a[:]    // = a[0:len(a)]         (entire slice)
a[:3]   // = a[0:3]
a[2:]   // = a[2:len(a)]
```

### 10.3 Append — Growth Rules

```go
// append returns a NEW slice header (possibly with new backing array)
// RULE: Always assign result back: s = append(s, elem)

s := make([]int, 3, 5)  // len=3, cap=5
s = append(s, 4)         // len=4, cap=5 (no new allocation)
s = append(s, 5)         // len=5, cap=5 (no new allocation)
s = append(s, 6)         // len=6, cap=10! (new backing array allocated)
                          // Capacity growth: typically 2x for small slices

// Growth algorithm (approximate, varies by Go version):
// cap < 256:   new cap ≈ 2 * old cap
// cap >= 256:  new cap grows by ~25% + 192
// (exact formula in runtime/slice.go)

// CRITICAL: After growth, modifications to the new slice
// NO LONGER affect the original backing array.

// Pre-allocate when you know the size to avoid repeated allocations:
s := make([]int, 0, 1000)  // empty slice, pre-allocated capacity
for i := 0; i < 1000; i++ {
    s = append(s, i)         // no re-allocation until cap exceeded
}

// Append multiple elements
a := []int{1, 2, 3}
b := []int{4, 5, 6}
c := append(a, b...)       // spread b into append
c2 := append(a, 4, 5, 6)  // same result
```

### 10.4 Copy

```go
// copy(dst, src): copies min(len(dst), len(src)) elements
// Returns number of elements copied
// Does NOT grow dst

src := []int{1, 2, 3, 4, 5}
dst := make([]int, 3)
n := copy(dst, src)      // copies 3 elements, n=3
fmt.Println(dst)         // [1 2 3]

// Copy overlapping slices: handled correctly
s := []int{1, 2, 3, 4, 5}
copy(s[1:], s[:])        // shift right by 1
// [1 1 2 3 4]  (s[0] is still 1)

// Duplicate a slice:
original := []int{1, 2, 3}
dup := make([]int, len(original))
copy(dup, original)

// Or using append trick:
dup2 := append([]int(nil), original...)
```

### 10.5 Slice Tricks Cheat Sheet

```go
s := []int{1, 2, 3, 4, 5}

// Delete element at index i (order matters: O(n))
i := 2
s = append(s[:i], s[i+1:]...)    // [1 2 4 5]

// Delete element at index i (order doesn't matter: O(1))
s[i] = s[len(s)-1]
s = s[:len(s)-1]                  // [1 2 5 4] or similar

// Insert element at index i
s = append(s[:i+1], s[i:]...)     // make room
s[i] = newValue

// Reverse a slice
for l, r := 0, len(s)-1; l < r; l, r = l+1, r-1 {
    s[l], s[r] = s[r], s[l]
}

// Filter in place (no new allocation)
n := 0
for _, v := range s {
    if keep(v) {
        s[n] = v
        n++
    }
}
s = s[:n]

// Check if slice contains element (no builtin — use loop or slices.Contains in 1.21+)
import "slices"
found := slices.Contains(s, 3)    // Go 1.21+
```

---

## 11. Maps — Internal Architecture

### 11.1 Map Internals

```
Go maps use a hash table with a bucket-based architecture:

┌─────────────────────────────────────────────────────────────┐
│                    MAP INTERNAL LAYOUT                      │
│                                                             │
│  hmap struct:                                               │
│  ┌──────┬──────────┬────────────┬────────────┬──────────┐  │
│  │count │ flags    │ B          │ buckets    │ oldbuckets│  │
│  │(len) │          │ (log2      │ (ptr to    │ (during   │  │
│  │      │          │ nbuckets)  │ bucket arr)│ grow)     │  │
│  └──────┴──────────┴────────────┴─────┬──────┴──────────┘  │
│                                        │                    │
│  Each bucket holds up to 8 k/v pairs:  │                    │
│  ┌─────────────────────────────────┐   │                    │
│  │ tophash[8]  (top byte of hash)  │◄──┘                    │
│  ├─────────────────────────────────┤                        │
│  │ keys[8]                         │                        │
│  ├─────────────────────────────────┤                        │
│  │ values[8]                       │                        │
│  ├─────────────────────────────────┤                        │
│  │ overflow *bucket                │  (overflow chaining)  │
│  └─────────────────────────────────┘                        │
└─────────────────────────────────────────────────────────────┘

Lookup: hash(key) → bucket index → compare tophash → compare keys
Insert: hash(key) → find/create bucket → place k/v
Growth: when avg 6.5 items/bucket → double buckets, incremental rehash
```

### 11.2 Map Operations — All Rules

```go
// CREATION
var m1 map[string]int    // nil map — CANNOT write to nil map!
m2 := map[string]int{}  // empty, initialized
m3 := make(map[string]int)           // empty, initialized
m4 := make(map[string]int, 100)      // hint: pre-allocate for ~100 entries

m5 := map[string]int{               // literal
    "alice": 25,
    "bob":   30,  // trailing comma REQUIRED in multi-line literals
}

// READ (safe for nil maps — returns zero value, no panic)
age := m5["alice"]       // 25
age2 := m5["unknown"]    // 0 (zero value for int, not an error)

// TWO-VALUE FORM: check existence
age3, ok := m5["alice"]   // ok=true, age3=25
age4, ok := m5["unknown"] // ok=false, age4=0
// RULE: Always use two-value form to distinguish "key not present" from "zero value"

// WRITE
m5["charlie"] = 35    // insert or update

// DELETE
delete(m5, "alice")   // remove key; safe if key doesn't exist

// LENGTH
len(m5)               // number of key-value pairs

// ITERATION (random order — never rely on it!)
for k, v := range m5 {
    fmt.Println(k, v)
}

// Iterate in sorted order:
import "sort"
keys := make([]string, 0, len(m5))
for k := range m5 {
    keys = append(keys, k)
}
sort.Strings(keys)
for _, k := range keys {
    fmt.Println(k, m5[k])
}

// MAP VALIDITY RULES:
// Key type must be COMPARABLE: bool, int, float, complex, string, pointer,
//                               channel, interface, array, struct with comparable fields
// NOT valid as keys: slice, map, function
// map[[]int]int   // COMPILE ERROR: slice is not comparable
```

### 11.3 Maps are Reference Types

```go
// Maps hold an implicit pointer — assigning copies the REFERENCE, not data
a := map[string]int{"x": 1}
b := a         // b points to the SAME underlying hash table

b["x"] = 99
fmt.Println(a["x"])  // 99 (a and b share the same map)

// To copy a map, you must copy manually
func copyMap(src map[string]int) map[string]int {
    dst := make(map[string]int, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

// Maps are NOT safe for concurrent use!
// Use sync.RWMutex or sync.Map for concurrent access
```

---

## 12. Structs — Complete Rules

### 12.1 Struct Definition and Initialization

```go
// DEFINITION
type Point struct {
    X float64   // exported field
    Y float64
}

type Person struct {
    Name    string
    Age     int
    Address Address  // nested struct
    Friends []*Person // slice of pointers (recursive)
}

type Address struct {
    Street string
    City   string
    Zip    string
}

// INITIALIZATION FORMS:
// Form 1: Positional (order-dependent, fragile)
p1 := Point{1.0, 2.0}

// Form 2: Named fields (preferred — order-independent)
p2 := Point{X: 1.0, Y: 2.0}
p3 := Point{X: 1.0}  // Y defaults to 0.0 (zero value)

// Form 3: new() — returns pointer, fields zero-initialized
p4 := new(Point)   // *Point, X=0, Y=0

// Form 4: Address-of composite literal
p5 := &Point{X: 1.0, Y: 2.0}  // *Point

// RULE: All fields not mentioned in named initialization get zero value
person := Person{
    Name: "Alice",
    Age:  30,
    Address: Address{
        City: "Delhi",
    },
    // Friends: nil (zero value for slice)
}

// ANONYMOUS STRUCT
point := struct{ X, Y int }{X: 1, Y: 2}

// STRUCT COMPARISON: comparable if ALL fields are comparable
p1 == p2         // true if all fields equal
// Structs with slices/maps/funcs cannot be compared with ==
```

### 12.2 Struct Methods

```go
type Rectangle struct {
    Width, Height float64
}

// VALUE RECEIVER: receives a COPY of the struct
// Use when: method doesn't modify struct, struct is small
func (r Rectangle) Area() float64 {
    return r.Width * r.Height
}

// POINTER RECEIVER: receives a pointer to the struct
// Use when: method modifies struct, or struct is large (avoid copying)
func (r *Rectangle) Scale(factor float64) {
    r.Width *= factor
    r.Height *= factor
}

// RULE: Go automatically takes/dereferences addresses:
rect := Rectangle{3, 4}
rect.Scale(2)       // Go auto-converts to (&rect).Scale(2)

rectPtr := &Rectangle{3, 4}
area := rectPtr.Area()  // Go auto-converts to (*rectPtr).Area()

// CONSISTENCY RULE: If any method has a pointer receiver,
// all methods should use pointer receivers.
// Mixed receivers on the same type cause subtle bugs with interfaces.

// Methods can be defined on any TYPE in the SAME PACKAGE
type Celsius float64

func (c Celsius) Fahrenheit() float64 {
    return float64(c)*9/5 + 32
}

// CANNOT define methods on types from other packages:
// func (s string) upper() string { ... }  // COMPILE ERROR
// Use a type definition to wrap it:
type MyString string
func (s MyString) upper() MyString { ... }  // OK
```

### 12.3 Struct Embedding — Composition

```go
// Embedding promotes fields and methods of the embedded type
type Animal struct {
    Name string
}

func (a Animal) Speak() string {
    return a.Name + " makes a sound"
}

type Dog struct {
    Animal          // EMBEDDING (not a field — no field name)
    Breed string
}

d := Dog{
    Animal: Animal{Name: "Rex"},
    Breed:  "Labrador",
}

// Promoted field access:
fmt.Println(d.Name)    // "Rex" — promoted from Animal
fmt.Println(d.Breed)   // "Labrador"

// Promoted method access:
d.Speak()              // "Rex makes a sound" — promoted from Animal

// Can also access via explicit embedded type name:
d.Animal.Name          // explicit path
d.Animal.Speak()       // explicit path

// OVERRIDING: Dog can define its own Speak() to shadow Animal's
func (d Dog) Speak() string {
    return d.Name + " barks"
}

// Now d.Speak() calls Dog's Speak (the outer one wins)
// d.Animal.Speak() still calls the embedded Animal's Speak

// MULTIPLE EMBEDDING
type Walker struct{}
type Runner struct{}

func (w Walker) Walk() string { return "walking" }
func (r Runner) Run() string  { return "running" }

type Athlete struct {
    Walker
    Runner
}

a := Athlete{}
a.Walk()  // OK
a.Run()   // OK

// AMBIGUITY: if both embedded types have same-named method → compile error
// Must use explicit path: a.Walker.Method() or a.Runner.Method()
```

### 12.4 Struct Tags

```go
// Tags are metadata strings on struct fields.
// They are accessible via reflection at runtime.
// Convention: `key:"value" key2:"value2"`

type User struct {
    ID       int    `json:"id" db:"user_id"`
    Name     string `json:"name" db:"full_name"`
    Email    string `json:"email,omitempty" validate:"email"`
    Password string `json:"-"`              // omit from JSON entirely
    Internal string `json:"-,"`             // field name is "-" (edge case)
}

// Common tag keys:
// json: used by encoding/json
//   omitempty: skip field if it's the zero value
//   -: skip field always
//   name: rename field in JSON
// db:      used by sqlx and other DB libraries
// yaml:    used by gopkg.in/yaml.v3
// xml:     used by encoding/xml
// validate: used by go-playground/validator
// form:    used by web frameworks

// Accessing tags via reflection:
import "reflect"

t := reflect.TypeOf(User{})
field := t.Field(0)   // ID field
tag := field.Tag.Get("json")  // "id"
dbTag := field.Tag.Get("db")  // "user_id"
```

---

## 13. Pointers — Memory Mental Model

### 13.1 Pointer Architecture

```
MEMORY LAYOUT:

Stack (function frames):          Heap (long-lived allocations):
┌─────────────────────┐          ┌─────────────────────────────┐
│  main() frame       │          │                             │
│  ┌───────────────┐  │          │  ┌──────────────────────┐   │
│  │ x int = 42   │  │          │  │ y int = 100          │   │
│  │ addr: 0x...10│  │          │  │ addr: 0xc000014080   │   │
│  │               │  │          │  └──────────────────────┘   │
│  │ p *int        │  │          │  ┌──────────────────────┐   │
│  │ = 0xc000014080│──┼──────────┼─►│ Person{...}          │   │
│  └───────────────┘  │          │  └──────────────────────┘   │
└─────────────────────┘          └─────────────────────────────┘
```

```go
// BASIC OPERATIONS
x := 42
p := &x          // p is *int, holds the address of x
*p = 100         // dereference: write through pointer
fmt.Println(x)   // 100

// POINTER TO STRUCT
type Point struct{ X, Y int }
pt := &Point{1, 2}
pt.X = 10        // Go auto-dereferences: (*pt).X = 10
fmt.Println(*pt) // {10 2}

// new() vs &T{}
p1 := new(int)        // *int, points to zero-value int on heap
p2 := new(Point)      // *Point, points to zero-value Point on heap
p3 := &Point{1, 2}    // *Point, points to initialized Point on heap

// These are functionally equivalent:
p4 := new(Point)
p5 := &Point{}        // both give *Point pointing to Point{0,0}

// WHEN TO USE POINTERS:
// 1. To modify the caller's data (method with pointer receiver)
// 2. Large structs (avoid copying overhead)
// 3. Optional fields (nil indicates absent)
// 4. Shared mutable state (multiple goroutines sharing data via pointer + sync)
// 5. Recursive data structures (linked list, tree nodes)

// WHEN NOT TO USE POINTERS:
// 1. Primitive types (int, float, bool) — copying is cheap
// 2. Small structs — copying may be cheaper than pointer indirection
// 3. Immutable data — value semantics are safer

// POINTER RULES:
// - Go has NO pointer arithmetic (no p++ or p+1 to advance pointers)
// - Use unsafe.Pointer for low-level pointer manipulation (avoid!)
// - Nil pointer dereference causes PANIC (not UB like C)

var p *int
_ = *p  // PANIC: runtime error: invalid memory address or nil pointer dereference
```

---

## 14. Interfaces — Implicit Contract System

### 14.1 Interface Architecture

```
An interface value is internally a two-word struct:

┌─────────────────────────────────────────────────────────────┐
│                  INTERFACE VALUE LAYOUT                     │
│                                                             │
│  ┌────────────────┬─────────────────────────────────────┐  │
│  │   type pointer │   value/pointer                     │  │
│  │   (*itab)      │   (data)                            │  │
│  └────────────────┴─────────────────────────────────────┘  │
│                                                             │
│  *itab contains:                                            │
│  ┌──────────────────────────────────────────────┐          │
│  │ inter *interfacetype  ← the interface type   │          │
│  │ type  *_type          ← the concrete type    │          │
│  │ fun   [...]uintptr    ← method dispatch table│          │
│  └──────────────────────────────────────────────┘          │
│                                                             │
│  nil interface: both type and value are nil                 │
│  typed nil:     type is set, value pointer is nil           │
│                 (NOT equal to nil interface!)               │
└─────────────────────────────────────────────────────────────┘
```

### 14.2 Interface Definition and Satisfaction

```go
// DEFINING AN INTERFACE
type Stringer interface {
    String() string
}

type Animal interface {
    Speak() string
    Name() string
}

// SATISFYING AN INTERFACE: Just have the right methods (no 'implements')
type Dog struct{ name string }

func (d Dog) Speak() string { return "Woof" }
func (d Dog) Name() string  { return d.name }
// Dog automatically satisfies Animal interface — no declaration needed

var a Animal = Dog{"Rex"}  // OK: Dog has both required methods
a.Speak()                  // dynamic dispatch → calls Dog.Speak()

// POINTER VS VALUE RECEIVER IN INTERFACES
type Counter struct{ n int }

func (c *Counter) Increment() { c.n++ }  // pointer receiver
func (c Counter) Value() int  { return c.n }  // value receiver

type Incrementer interface {
    Increment()
    Value() int
}

var i Incrementer = &Counter{}  // OK: *Counter has both methods
// var i2 Incrementer = Counter{}  // COMPILE ERROR!
// Counter value does NOT have Increment() (only *Counter does)
// *Counter has both Increment() and Value() (pointer receives value's methods too)

// RULE: *T satisfies any interface that T satisfies, PLUS interfaces
//       requiring pointer receiver methods.
//       T does NOT satisfy interfaces requiring pointer receiver methods.
```

### 14.3 Interface Composition

```go
// Interfaces can be composed from other interfaces
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type ReadWriter interface {
    Reader  // embeds Reader
    Writer  // embeds Writer
}

// Any type with Read() and Write() satisfies ReadWriter

// The standard library heavily uses interface composition:
// io.Reader, io.Writer, io.Closer, io.ReadWriter, io.ReadWriteCloser
// io.Seeker, io.ReadSeeker, io.WriterSeeker, etc.
```

### 14.4 The Empty Interface and Type Assertions

```go
// interface{} (or 'any' in Go 1.18+) accepts ANY value
var x interface{} = 42
var y any = "hello"

// TYPE ASSERTION: extract concrete type from interface
// Form 1: panics if type is wrong
s := y.(string)    // "hello" — OK
// n := y.(int)    // PANIC: interface holds string, not int

// Form 2: comma-ok idiom (safe)
n, ok := y.(int)   // n=0, ok=false (no panic)
s, ok := y.(string) // s="hello", ok=true

// TYPE SWITCH: best way to handle multiple types
func process(i interface{}) {
    switch v := i.(type) {
    case int:
        fmt.Printf("int: %d\n", v)
    case string:
        fmt.Printf("string: %s\n", v)
    case []int:
        fmt.Printf("slice of int: %v\n", v)
    case nil:
        fmt.Println("nil")
    default:
        fmt.Printf("unknown type: %T\n", v)
    }
}
```

### 14.5 The Typed-Nil Trap — Critical Rule

```go
// This is one of Go's most common confusing behaviors:

type MyError struct{ msg string }
func (e *MyError) Error() string { return e.msg }

func fail() error {            // return type is interface 'error'
    var err *MyError = nil     // nil pointer of concrete type
    return err                 // this is NOT a nil interface!
}

func main() {
    err := fail()
    if err != nil {           // TRUE! err is NOT nil!
        fmt.Println("error:", err)  // prints "<nil>" — confusing!
    }
}

// WHY: The returned error interface holds (type=*MyError, value=nil)
// A nil interface would hold (type=nil, value=nil)
// They are different!

// FIX: Return nil directly (not a nil pointer of a concrete type)
func fail2() error {
    return nil                 // returns (type=nil, value=nil) — nil interface
}

// GOLDEN RULE: Never return a *ConcreteType from a function
// that returns an interface type, when you mean to return nil.
// Return nil directly.
```

---

## 15. Error Handling — The Go Way

### 15.1 The Error Interface

```go
// The built-in error interface:
type error interface {
    Error() string
}

// Creating simple errors:
import "errors"
err1 := errors.New("something went wrong")

import "fmt"
err2 := fmt.Errorf("operation failed: %s", reason)

// Checking errors:
if err != nil {
    return err  // propagate up
}
```

### 15.2 Custom Error Types

```go
// Custom error with more context:
type ValidationError struct {
    Field   string
    Message string
    Value   interface{}
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on field %q: %s (got: %v)",
        e.Field, e.Message, e.Value)
}

// Using the custom error:
func validate(age int) error {
    if age < 0 || age > 150 {
        return &ValidationError{
            Field:   "age",
            Message: "must be between 0 and 150",
            Value:   age,
        }
    }
    return nil
}

// Extracting the concrete error with errors.As:
err := validate(-1)
var valErr *ValidationError
if errors.As(err, &valErr) {
    fmt.Println("Field:", valErr.Field)
}
```

### 15.3 Error Wrapping (Go 1.13+)

```go
// Wrapping: adds context while preserving the original error
func readConfig(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("readConfig: %w", err)  // %w wraps the error
    }
    // ...
}

// The error chain:
// readConfig error → wraps → os.PathError → wraps → syscall.Errno

// UNWRAPPING with errors.Is: checks if any error in the chain matches
err := readConfig("/missing/file")
if errors.Is(err, os.ErrNotExist) {
    fmt.Println("file not found")  // matches!
}

// UNWRAPPING with errors.As: finds first error in chain of given type
var pathErr *os.PathError
if errors.As(err, &pathErr) {
    fmt.Println("path:", pathErr.Path)
}

// errors.Is vs ==:
// err == io.EOF      ← only matches exact error (no unwrapping)
// errors.Is(err, io.EOF)  ← checks entire chain (use this!)

// Custom Unwrap() method (for custom error wrapping):
type AppError struct {
    Code int
    Err  error
}

func (e *AppError) Error() string { return fmt.Sprintf("[%d]: %v", e.Code, e.Err) }
func (e *AppError) Unwrap() error { return e.Err }  // enables errors.Is/As chain traversal
```

### 15.4 Error Handling Patterns

```go
// PATTERN 1: Guard clause (check early, return often)
func processFile(path string) error {
    if path == "" {
        return errors.New("path cannot be empty")
    }
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("processFile: %w", err)
    }
    defer f.Close()
    // ... rest of function
    return nil
}

// PATTERN 2: Error sentinel values (predefined package-level errors)
var (
    ErrNotFound = errors.New("not found")
    ErrTimeout  = errors.New("timeout")
    ErrInvalid  = errors.New("invalid input")
)

// Callers can compare with errors.Is(err, mypackage.ErrNotFound)

// PATTERN 3: Error handling in loops (accumulate? fail fast?)
// Fail fast (stop at first error):
for _, item := range items {
    if err := process(item); err != nil {
        return fmt.Errorf("processing item %v: %w", item, err)
    }
}

// Accumulate errors (process all, report all):
var errs []error
for _, item := range items {
    if err := process(item); err != nil {
        errs = append(errs, err)
    }
}
if len(errs) > 0 {
    return errors.Join(errs...)  // Go 1.20+: joins multiple errors
}

// PATTERN 4: Wrapping with operation context
func (s *Service) GetUser(id int) (*User, error) {
    user, err := s.db.QueryUser(id)
    if err != nil {
        return nil, fmt.Errorf("GetUser(id=%d): %w", id, err)
    }
    return user, nil
}
```

---

## 16. Goroutines — Concurrency Architecture

### 16.1 The Go Runtime Scheduler (M:N Threading)

```
┌──────────────────────────────────────────────────────────────────┐
│                 GO SCHEDULER ARCHITECTURE (M:N)                  │
│                                                                  │
│  G = Goroutine (lightweight, ~2KB stack, millions possible)      │
│  M = OS Thread  (expensive, ~1-8MB stack, limited)               │
│  P = Processor  (logical CPU, holds run queue, GOMAXPROCS of them)│
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │      P      │  │      P      │  │      P      │             │
│  │  ┌────────┐ │  │  ┌────────┐ │  │  ┌────────┐ │             │
│  │  │  M     │ │  │  │  M     │ │  │  │  M     │ │             │
│  │  └────────┘ │  │  └────────┘ │  │  └────────┘ │             │
│  │  run queue: │  │  run queue: │  │  run queue: │             │
│  │  [G G G G]  │  │  [G G G]    │  │  [G G]      │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
│                                                                  │
│  Global run queue: [G G G G G ...]  (overflow)                  │
│  Dead goroutines pool (for reuse)                                │
│                                                                  │
│  Goroutine states: RUNNABLE → RUNNING → WAITING → RUNNABLE       │
│                                           │                      │
│                                    (I/O, channel,                │
│                                     sleep, syscall)              │
└──────────────────────────────────────────────────────────────────┘
```

### 16.2 Creating and Managing Goroutines

```go
// Start a goroutine with 'go' keyword
go fmt.Println("hello from goroutine")  // fire and forget

// Goroutine with a closure
x := 42
go func() {
    fmt.Println(x)  // captures x by reference (be careful with concurrent access!)
}()

// Goroutine with a function call
go doWork(arg1, arg2)  // arguments evaluated NOW, in calling goroutine

// RULE: The main goroutine exiting kills ALL goroutines immediately.
// You MUST synchronize before main() returns.

// WAITING for goroutines to complete:
import "sync"

var wg sync.WaitGroup

for i := 0; i < 5; i++ {
    wg.Add(1)    // increment counter BEFORE starting goroutine
    go func(n int) {
        defer wg.Done()  // decrement counter when goroutine completes
        fmt.Println("goroutine", n)
    }(i)
}
wg.Wait()  // block until counter reaches 0
```

### 16.3 Race Conditions

```go
// RACE CONDITION: two goroutines access shared data without synchronization,
// and at least one access is a write.

// Buggy code:
var counter int

for i := 0; i < 1000; i++ {
    go func() {
        counter++  // RACE CONDITION: read-increment-write is not atomic
    }()
}
// Final value of counter is unpredictable (could be anything from 1 to 1000)

// Detect races: go run -race main.go  OR  go test -race ./...
// The race detector adds instrumentation and reports races at runtime

// FIX 1: sync.Mutex
var mu sync.Mutex
var counter2 int

for i := 0; i < 1000; i++ {
    go func() {
        mu.Lock()
        counter2++
        mu.Unlock()
    }()
}

// FIX 2: sync/atomic
import "sync/atomic"
var counter3 int64

for i := 0; i < 1000; i++ {
    go func() {
        atomic.AddInt64(&counter3, 1)  // atomic increment
    }()
}

// FIX 3: Channel (send work to one goroutine)
ch := make(chan int, 1000)
// ... send to channel, one goroutine receives and updates counter
```

---

## 17. Channels — Communication Rules

### 17.1 Channel Architecture

```
CHANNEL INTERNAL LAYOUT:

┌───────────────────────────────────────────────────────┐
│                   hchan struct                        │
│                                                       │
│  qcount   uint          ← current elements in queue  │
│  dataqsiz uint          ← circular queue size (cap)  │
│  buf      unsafe.Pointer← ring buffer (for buffered) │
│  elemsize uint16        ← element size in bytes      │
│  closed   uint32        ← 0=open, 1=closed           │
│  sendx    uint          ← send index in ring buffer  │
│  recvx    uint          ← receive index in ring buffer│
│  recvq    waitq         ← goroutines blocked on recv │
│  sendq    waitq         ← goroutines blocked on send │
│  lock     mutex                                       │
└───────────────────────────────────────────────────────┘
```

### 17.2 Channel Operations — Complete Rules

```go
// CREATING CHANNELS
var ch1 chan int            // nil channel (all operations block forever!)
ch2 := make(chan int)       // UNBUFFERED channel (cap=0)
ch3 := make(chan int, 10)   // BUFFERED channel (cap=10)

// UNBUFFERED: sender and receiver must be ready simultaneously
// BUFFERED: sender can proceed until buffer is full

// SEND
ch2 <- 42          // send 42 (blocks if no receiver ready on unbuffered)
ch3 <- 42          // send 42 (blocks only if buffer is full)

// RECEIVE
v := <-ch2         // receive (blocks if no sender ready on unbuffered)
v, ok := <-ch3     // ok=false means channel is closed and drained

// CLOSE
close(ch2)         // signal that no more values will be sent
// RULES about close:
// 1. Only SENDER should close a channel (receiver closing is bad practice)
// 2. Closing a closed channel → PANIC
// 3. Sending to a closed channel → PANIC
// 4. Receiving from a closed, empty channel → returns (zero value, false)
// 5. Receiving from a closed, non-empty channel → normal (value, true)

// RANGE over channel (receive until closed)
for v := range ch3 {
    fmt.Println(v)
}
// Loop exits when channel is closed and drained

// LENGTH and CAPACITY
len(ch3)    // number of elements currently queued
cap(ch3)    // buffer capacity (0 for unbuffered)

// DIRECTION TYPES: restrict channel to send-only or receive-only
func produce(out chan<- int) {    // chan<- : send-only
    out <- 42
}
func consume(in <-chan int) {     // <-chan : receive-only
    v := <-in
    _ = v
}
// Bidirectional chan int can be passed to either
// Direction restrictions are enforced at compile time
```

### 17.3 The Select Statement

```go
// select: like switch but for channel operations
// Blocks until ONE of the cases can proceed
// If multiple cases are ready: one is chosen RANDOMLY

select {
case v := <-ch1:
    fmt.Println("received from ch1:", v)
case ch2 <- 42:
    fmt.Println("sent to ch2")
case v, ok := <-ch3:
    if !ok {
        fmt.Println("ch3 was closed")
    } else {
        fmt.Println("received from ch3:", v)
    }
}

// NON-BLOCKING: select with default
select {
case v := <-ch:
    fmt.Println("received:", v)
default:
    fmt.Println("no value ready")  // runs immediately if no channel is ready
}

// TIMEOUT PATTERN
import "time"
select {
case result := <-compute():
    fmt.Println("result:", result)
case <-time.After(5 * time.Second):
    fmt.Println("timed out")
}

// DONE CHANNEL PATTERN (signal goroutine to stop)
done := make(chan struct{})  // empty struct: zero bytes

go func() {
    for {
        select {
        case <-done:
            return  // stop goroutine
        case work := <-workCh:
            process(work)
        }
    }
}()

// To stop the goroutine:
close(done)  // closing broadcasts to ALL receivers (unlike sending, which only one receives)
```

### 17.4 Channel Patterns

```go
// PIPELINE: chain of goroutines connected by channels
func generator(nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        for _, n := range nums {
            out <- n
        }
        close(out)
    }()
    return out
}

func square(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        for n := range in {
            out <- n * n
        }
        close(out)
    }()
    return out
}

// Usage: gen → square → print
gen := generator(2, 3, 4)
sq := square(gen)
for v := range sq {
    fmt.Println(v)  // 4, 9, 16
}

// FAN-OUT: one input channel → multiple worker goroutines
func fanOut(in <-chan int, numWorkers int) []<-chan int {
    outs := make([]<-chan int, numWorkers)
    for i := range numWorkers {
        outs[i] = worker(in)
    }
    return outs
}

// FAN-IN: merge multiple input channels into one
func fanIn(cs ...<-chan int) <-chan int {
    var wg sync.WaitGroup
    merged := make(chan int, 10)

    output := func(c <-chan int) {
        defer wg.Done()
        for v := range c {
            merged <- v
        }
    }

    wg.Add(len(cs))
    for _, c := range cs {
        go output(c)
    }

    go func() {
        wg.Wait()
        close(merged)
    }()
    return merged
}
```

---

## 18. Sync Package — Synchronization Primitives

### 18.1 sync.Mutex and sync.RWMutex

```go
// MUTEX: mutual exclusion lock
type SafeCounter struct {
    mu sync.Mutex
    n  int
}

func (c *SafeCounter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()  // ALWAYS use defer with Unlock to prevent deadlocks
    c.n++
}

func (c *SafeCounter) Value() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.n
}

// RW MUTEX: multiple readers OR one writer (better performance for read-heavy)
type SafeMap struct {
    mu sync.RWMutex
    m  map[string]int
}

func (s *SafeMap) Get(key string) (int, bool) {
    s.mu.RLock()    // multiple goroutines can hold RLock simultaneously
    defer s.mu.RUnlock()
    v, ok := s.m[key]
    return v, ok
}

func (s *SafeMap) Set(key string, val int) {
    s.mu.Lock()     // exclusive write lock
    defer s.mu.Unlock()
    s.m[key] = val
}

// DEADLOCK RULES:
// 1. Never call Lock() twice in the same goroutine without Unlock() in between
// 2. Always Unlock() in the same goroutine that Locked()
// 3. Use defer mu.Unlock() immediately after mu.Lock() to prevent forgetting
// 4. Lock ordering: if you must acquire multiple locks, always acquire in the same order
```

### 18.2 sync.WaitGroup

```go
// Used to wait for a collection of goroutines to finish
var wg sync.WaitGroup

for i := 0; i < 10; i++ {
    wg.Add(1)              // must be called BEFORE starting goroutine
    go func(id int) {
        defer wg.Done()    // called when goroutine finishes
        doWork(id)
    }(i)
}
wg.Wait()                  // blocks until all Done() calls balance Add() calls

// RULE: Add() must be called before the goroutine starts (not inside it)
// RULE: Done() should use defer to ensure it's called even on panic
// RULE: Counter must not go negative (panic)
```

### 18.3 sync.Once

```go
// Ensures a function is called exactly once, even with concurrent goroutines
var once sync.Once
var db *sql.DB

func getDB() *sql.DB {
    once.Do(func() {
        // This runs exactly once, even if called concurrently
        var err error
        db, err = sql.Open("postgres", dsn)
        if err != nil {
            log.Fatal(err)
        }
    })
    return db
}
// Perfect for lazy initialization of expensive resources
```

### 18.4 sync.Pool

```go
// Pool: stores temporary objects to reuse, reducing GC pressure
// RULE: Pool contents may be garbage collected at any time
// RULE: Not for use as a connection pool (use explicit lifecycle management)

var bufPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)  // create new buffer when pool is empty
    },
}

func processRequest(data []byte) string {
    buf := bufPool.Get().(*bytes.Buffer)  // get from pool (or create new)
    defer func() {
        buf.Reset()         // clear buffer
        bufPool.Put(buf)    // return to pool
    }()

    buf.Write(data)
    return buf.String()
}
```

### 18.5 sync.Map

```go
// sync.Map: concurrent map (optimized for specific use cases)
// Use when: keys are written once and read many times, OR when distinct goroutines
// access distinct sets of keys. For general concurrent map: use map + RWMutex.

var sm sync.Map

// Store
sm.Store("key", "value")

// Load
v, ok := sm.Load("key")
if ok {
    fmt.Println(v.(string))
}

// LoadOrStore: atomic load-if-present or store-and-return
actual, loaded := sm.LoadOrStore("key", "new_value")
// if "key" existed: actual=existing_value, loaded=true
// if "key" didn't exist: actual="new_value", loaded=false

// Delete
sm.Delete("key")

// Iterate
sm.Range(func(k, v interface{}) bool {
    fmt.Println(k, v)
    return true  // return false to stop iteration
})
```

---

## 19. Packages & Modules — Organization Rules

### 19.1 Package Organization

```
CANONICAL PROJECT LAYOUT:

myservice/
├── go.mod
├── go.sum
├── main.go              ← or cmd/myservice/main.go for larger projects
│
├── cmd/                 ← entry points (each subdirectory is a binary)
│   ├── server/
│   │   └── main.go
│   └── migrate/
│       └── main.go
│
├── internal/            ← private packages (cannot be imported externally)
│   ├── config/
│   ├── database/
│   └── auth/
│
├── pkg/                 ← public packages (can be imported by other modules)
│   └── api/
│
├── api/                 ← API definitions (OpenAPI, protobuf)
│
├── configs/             ← configuration files
├── docs/                ← documentation
├── scripts/             ← build/deployment scripts
└── testdata/            ← test fixtures (excluded from builds)
```

### 19.2 Exported vs Unexported

```go
package mypackage

// EXPORTED: starts with uppercase — visible from other packages
type User struct {
    Name  string    // exported field
    Email string    // exported field
    id    int       // UNEXPORTED field — only accessible within mypackage
}

// EXPORTED function
func NewUser(name, email string) *User {
    return &User{Name: name, Email: email, id: nextID()}
}

// UNEXPORTED function — only callable within mypackage
func nextID() int { ... }

// EXPORTED constant
const MaxRetries = 3

// UNEXPORTED variable
var defaultTimeout = 30 * time.Second

// RULE: Even unexported fields are accessible via reflection
// RULE: Unexported fields of structs from other packages cannot be
//       set directly, but can be accessed if the type is embedded
```

### 19.3 Internal Packages

```go
// The /internal/ directory is special:
// Only code rooted at the PARENT of the internal directory can import it.

// Layout:
// mymodule/
// ├── cmd/server/main.go          ← CAN import mymodule/internal/config
// ├── internal/
// │   └── config/config.go        ← package config
// └── pkg/
//     └── util/util.go            ← CAN import mymodule/internal/config

// Another module (github.com/other/module) CANNOT import mymodule/internal/config
// Attempting to do so causes a compile error:
// "use of internal package mymodule/internal/config not allowed"

// This is enforced by the Go toolchain, not by build tags.
```

---

## 20. Standard Library — Essential Packages

### 20.1 fmt — Formatted I/O

```go
import "fmt"

// PRINTING
fmt.Print("no newline")
fmt.Println("with newline")
fmt.Printf("formatted: %d, %s, %f\n", 42, "hello", 3.14)

// SPRINTF: format to string (doesn't print)
s := fmt.Sprintf("value: %v", x)

// FPRINTF: write to any io.Writer
fmt.Fprintf(os.Stderr, "error: %v\n", err)

// FORMAT VERBS:
// %v   default format (most common)
// %+v  struct with field names
// %#v  Go syntax representation
// %T   type name
// %d   integer (decimal)
// %b   integer (binary)
// %o   integer (octal)
// %x   integer (hex lowercase)
// %X   integer (hex uppercase)
// %f   float (decimal notation)
// %e   float (scientific notation)
// %g   float (whichever is more compact)
// %s   string
// %q   quoted string with escape sequences
// %p   pointer address
// %t   boolean
// %c   character (rune)
// Width and precision: %10d  %-10s  %8.2f  %010d

// SCANNING
var n int
fmt.Scan(&n)          // reads from stdin
fmt.Scanf("%d", &n)   // formatted scan
fmt.Sscan("42", &n)   // scan from string
fmt.Sscanf("x=42", "x=%d", &n)

// STRINGER INTERFACE: types can define how they're formatted
type Point struct{ X, Y int }
func (p Point) String() string {
    return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}
// Now fmt.Println(point) uses our String() method
```

### 20.2 strings Package

```go
import "strings"

// Testing
strings.Contains("seafood", "foo")      // true
strings.ContainsAny("hello", "aeiou")  // true (contains any of these chars)
strings.HasPrefix("hello", "he")       // true
strings.HasSuffix("hello", "lo")       // true
strings.Count("cheese", "e")           // 3

// Searching
strings.Index("chicken", "ken")        // 4 (byte index, -1 if not found)
strings.LastIndex("go gopher", "go")   // 3

// Transformation
strings.ToUpper("hello")               // "HELLO"
strings.ToLower("HELLO")               // "hello"
strings.Title("hello world")           // "Hello World" (deprecated, use golang.org/x/text)
strings.TrimSpace("  hello  ")         // "hello"
strings.Trim("--hello--", "-")         // "hello"
strings.TrimLeft("--hello--", "-")     // "hello--"
strings.TrimRight("--hello--", "-")    // "--hello"
strings.TrimPrefix("foobar", "foo")    // "bar"
strings.TrimSuffix("foobar", "bar")    // "foo"
strings.Replace("oink oink oink", "oink", "moo", 2)  // "moo moo oink"
strings.ReplaceAll("oink oink", "oink", "moo")         // "moo moo"

// Splitting / Joining
strings.Split("a,b,c", ",")           // ["a", "b", "c"]
strings.SplitN("a,b,c", ",", 2)       // ["a", "b,c"] — max 2 parts
strings.Fields("  foo bar  baz  ")    // ["foo", "bar", "baz"] — split on whitespace
strings.Join([]string{"a","b","c"}, "-")  // "a-b-c"
strings.Repeat("na", 4)               // "nananana"

// Efficient string building (avoids repeated allocation)
var sb strings.Builder
sb.WriteString("hello")
sb.WriteString(", ")
sb.WriteString("world")
result := sb.String()  // "hello, world"
// RULE: strings.Builder is NOT safe for concurrent use
```

### 20.3 strconv Package

```go
import "strconv"

// int → string
s := strconv.Itoa(42)              // "42"
s2 := strconv.FormatInt(-42, 10)   // base 10: "-42"
s3 := strconv.FormatInt(255, 16)   // base 16: "ff"
s4 := strconv.FormatUint(42, 2)    // base 2:  "101010"

// string → int
n, err := strconv.Atoi("42")       // 42, nil
n2, err := strconv.ParseInt("-42", 10, 64)   // base=10, bitSize=64
n3, err := strconv.ParseUint("ff", 16, 64)   // hex parsing

// float → string
s5 := strconv.FormatFloat(3.14159, 'f', 2, 64)  // "3.14"
// format: 'f'=decimal, 'e'=scientific, 'g'=shortest, 'b'=binary exponent
// prec: number of decimal places (-1 for shortest)
// bitSize: 32 or 64

// string → float
f, err := strconv.ParseFloat("3.14", 64)

// bool
b, err := strconv.ParseBool("true")    // true
s6 := strconv.FormatBool(true)          // "true"
```

### 20.4 os Package

```go
import "os"

// File operations
f, err := os.Open("file.txt")           // read-only
f2, err := os.Create("file.txt")        // create/truncate, read-write
f3, err := os.OpenFile("file.txt",      // full control
    os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
defer f.Close()

// Reading
data, err := os.ReadFile("file.txt")    // reads entire file (Go 1.16+)

// Writing
err = os.WriteFile("file.txt", data, 0644)  // writes entire file

// File info
info, err := os.Stat("file.txt")
info.Name()     // filename
info.Size()     // bytes
info.Mode()     // permissions
info.ModTime()  // modification time
info.IsDir()    // is directory?

// Environment
val := os.Getenv("HOME")            // get env variable
os.Setenv("KEY", "value")           // set env variable
os.Unsetenv("KEY")                  // unset
all := os.Environ()                 // []string of all env vars

// Program control
os.Exit(1)     // exit with code (skips defer calls! Avoid in most cases)
os.Args        // []string: command-line arguments (os.Args[0] = program name)

// Directories
err = os.Mkdir("mydir", 0755)       // create single directory
err = os.MkdirAll("a/b/c", 0755)    // create all parents too
entries, err := os.ReadDir(".")     // list directory contents
err = os.Remove("file.txt")         // remove file
err = os.RemoveAll("mydir")         // remove directory tree
err = os.Rename("old", "new")       // rename/move
```

### 20.5 io Package

```go
import "io"

// Core interfaces
type Reader interface {
    Read(p []byte) (n int, err error)
    // Reads up to len(p) bytes. Returns n bytes read and any error.
    // Returns (0, io.EOF) when no more data.
    // RULE: caller must process n>0 bytes BEFORE checking error
}

type Writer interface {
    Write(p []byte) (n int, err error)
    // Writes len(p) bytes. Returns n bytes written and any error.
    // Must return non-nil error if n < len(p).
}

// io utility functions
data, err := io.ReadAll(r)               // read all from reader
n, err := io.Copy(dst, src)              // copy from reader to writer
n, err := io.CopyN(dst, src, 1024)       // copy exactly N bytes
r2 := io.LimitReader(r, 1024*1024)      // limit reads to 1MB
r3 := io.TeeReader(r, logWriter)        // read + write to secondary writer
r4 := io.MultiReader(r1, r2, r3)        // concatenate readers
w2 := io.MultiWriter(w1, w2)            // write to multiple writers
```

### 20.6 net/http Package

```go
import "net/http"

// HTTP CLIENT
resp, err := http.Get("https://example.com")
if err != nil {
    return err
}
defer resp.Body.Close()                    // CRITICAL: always close body
body, err := io.ReadAll(resp.Body)

// Custom client with timeout (ALWAYS set timeouts on HTTP clients!)
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        DialContext: (&net.Dialer{
            Timeout:   30 * time.Second,
            KeepAlive: 30 * time.Second,
        }).DialContext,
        MaxIdleConns:          100,
        IdleConnTimeout:       90 * time.Second,
        TLSHandshakeTimeout:   10 * time.Second,
        ResponseHeaderTimeout: 10 * time.Second,
    },
}

// POST request
resp2, err := client.Post("https://api.example.com/data",
    "application/json", bytes.NewReader(jsonData))

// Custom request
req, err := http.NewRequestWithContext(ctx, "POST",
    "https://api.example.com/data",
    bytes.NewReader(jsonData))
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Authorization", "Bearer "+token)
resp3, err := client.Do(req)

// HTTP SERVER
mux := http.NewServeMux()

mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, "Hello, World!")
})

mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")               // Go 1.22+: extract path parameter
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)          // must call before writing body
    json.NewEncoder(w).Encode(map[string]string{"id": id})
})

server := &http.Server{
    Addr:         ":8080",
    Handler:      mux,
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 15 * time.Second,
    IdleTimeout:  60 * time.Second,
}

log.Fatal(server.ListenAndServe())
```

### 20.7 encoding/json Package

```go
import "encoding/json"

// MARSHALING (Go → JSON)
type Person struct {
    Name  string `json:"name"`
    Age   int    `json:"age,omitempty"`    // omit if Age==0
    Email string `json:"email"`
}

p := Person{Name: "Alice", Age: 30, Email: "alice@example.com"}
data, err := json.Marshal(p)           // []byte
// {"name":"Alice","age":30,"email":"alice@example.com"}

data2, err := json.MarshalIndent(p, "", "  ")  // pretty-printed

// UNMARSHALING (JSON → Go)
var p2 Person
err = json.Unmarshal(data, &p2)

// STREAMING (for large data / network)
encoder := json.NewEncoder(w)          // writes to io.Writer
encoder.SetIndent("", "  ")
err = encoder.Encode(p)

decoder := json.NewDecoder(r)          // reads from io.Reader
err = decoder.Decode(&p2)

// DYNAMIC JSON with map
var result map[string]interface{}
json.Unmarshal(jsonData, &result)

// Or using json.RawMessage to delay parsing
type Response struct {
    Status string          `json:"status"`
    Data   json.RawMessage `json:"data"`   // parse later
}
```

### 20.8 context Package

```go
import "context"

// Context carries: deadlines, cancellation signals, request-scoped values
// Pass context.Context as FIRST parameter to every function that might block

// Root contexts:
ctx := context.Background()  // never cancelled, no values, no deadline (use at top level)
ctx2 := context.TODO()       // like Background but signals "we should come back and fix this"

// Derived contexts:
ctx3, cancel := context.WithCancel(ctx)
defer cancel()   // ALWAYS call cancel to release resources

ctx4, cancel2 := context.WithTimeout(ctx, 5*time.Second)
defer cancel2()

ctx5, cancel3 := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
defer cancel3()

ctx6 := context.WithValue(ctx, "userID", 42)  // add request-scoped value

// Checking context status
select {
case <-ctx.Done():
    return ctx.Err()  // context.Canceled or context.DeadlineExceeded
default:
    // proceed
}

// Retrieving values
userID, ok := ctx.Value("userID").(int)  // type assertion needed

// RULES:
// 1. Context.Value keys should be unexported types to avoid collisions
// 2. Only put REQUEST-SCOPED values in context (auth tokens, trace IDs)
// 3. Never put optional parameters in context (use function params)
// 4. Cancel as soon as you don't need the operation (free resources)
```

### 20.9 time Package

```go
import "time"

// Current time
now := time.Now()              // Local time
utc := time.Now().UTC()        // UTC time

// Duration constants
time.Second                    // 1s
time.Millisecond               // 1ms
time.Minute                    // 1m
time.Hour                      // 1h
5 * time.Second                // 5s

// Parsing and formatting (reference time: Mon Jan 2 15:04:05 MST 2006)
// Go uses a reference time instead of format codes like strftime
layout := "2006-01-02 15:04:05"
t, err := time.Parse(layout, "2024-01-15 10:30:00")
s := t.Format(layout)          // "2024-01-15 10:30:00"
s2 := t.Format(time.RFC3339)   // "2024-01-15T10:30:00Z"

// Operations
t2 := t.Add(24 * time.Hour)    // add duration
t3 := t.AddDate(0, 1, 0)       // add 0 years, 1 month, 0 days
diff := t2.Sub(t)              // duration between times
t2.Before(t)                   // bool
t2.After(t)                    // bool
t.Equal(t2)                    // bool (preferred over == for time)

// Sleeping
time.Sleep(1 * time.Second)    // block current goroutine for 1 second

// Timers and Tickers
timer := time.NewTimer(5 * time.Second)
<-timer.C                      // blocks until timer fires
timer.Stop()                   // cancel timer (CALL THIS to avoid goroutine leak)

ticker := time.NewTicker(1 * time.Second)
defer ticker.Stop()            // ALWAYS stop ticker
for t := range ticker.C {
    fmt.Println("tick at", t)
}

// time.After (convenience, but leaks timer until it fires!)
select {
case result := <-workDone:
    fmt.Println(result)
case <-time.After(5 * time.Second):
    fmt.Println("timeout")
}
// RULE: time.After can cause a goroutine leak if used in a long-lived loop.
// Use time.NewTimer + Stop for production code.
```

---

## 21. Generics — Type Parameters

### 21.1 Generic Functions (Go 1.18+)

```go
// Type parameters are declared in square brackets
// constraint specifies what operations are allowed on the type

// ANY type (interface{} equivalent)
func PrintAny[T any](v T) {
    fmt.Println(v)
}

PrintAny(42)        // T inferred as int
PrintAny("hello")   // T inferred as string

// COMPARABLE constraint: allows == and !=
func Contains[T comparable](slice []T, item T) bool {
    for _, v := range slice {
        if v == item {
            return true
        }
    }
    return false
}

Contains([]int{1, 2, 3}, 2)        // true
Contains([]string{"a", "b"}, "c") // false

// ORDERED constraint: allows <, >, <=, >=
import "cmp"

func Min[T cmp.Ordered](a, b T) T {
    if a < b {
        return a
    }
    return b
}

Min(3, 5)      // 3 (int)
Min(3.14, 2.72) // 2.72 (float64)
```

### 21.2 Custom Type Constraints

```go
// Constraint is an interface
type Number interface {
    int | int8 | int16 | int32 | int64 |
    uint | uint8 | uint16 | uint32 | uint64 |
    float32 | float64
}

func Sum[T Number](nums []T) T {
    var total T
    for _, n := range nums {
        total += n
    }
    return total
}

Sum([]int{1, 2, 3})          // 6 (int)
Sum([]float64{1.1, 2.2})     // 3.3 (float64)

// Using ~ for underlying types (allows custom types based on int)
type Integer interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64
}

type Celsius float64   // underlying type float64
// A constraint with ~float64 would accept Celsius
```

### 21.3 Generic Types

```go
// Generic struct
type Stack[T any] struct {
    items []T
}

func (s *Stack[T]) Push(item T) {
    s.items = append(s.items, item)
}

func (s *Stack[T]) Pop() (T, bool) {
    if len(s.items) == 0 {
        var zero T
        return zero, false
    }
    item := s.items[len(s.items)-1]
    s.items = s.items[:len(s.items)-1]
    return item, true
}

func (s *Stack[T]) Len() int {
    return len(s.items)
}

// Usage
s := &Stack[int]{}
s.Push(1)
s.Push(2)
v, ok := s.Pop()   // v=2, ok=true

// Type inference with generic types
s2 := Stack[string]{}  // explicit type argument
```

---

## 22. Memory Model, Stack & Heap

### 22.1 Stack vs Heap — Escape Analysis

```
┌────────────────────────────────────────────────────────────────┐
│                  MEMORY LAYOUT PER GOROUTINE                   │
│                                                                │
│  Each goroutine starts with ~2KB stack (grows as needed)       │
│                                                                │
│  Stack (LIFO, fast, auto-managed):                             │
│  ┌───────────────────────────────┐                             │
│  │  main() frame                 │                             │
│  │  ├── local variables          │ ← fast allocation           │
│  │  └── function call arguments  │   just move stack pointer   │
│  │  foo() frame                  │                             │
│  │  ├── local variables          │                             │
│  │  └── ...                      │                             │
│  └───────────────────────────────┘                             │
│                                                                │
│  Heap (GC-managed, slower, shared across goroutines):          │
│  ┌──────────────────────────────────────────────────┐          │
│  │  Allocations that "escape" to heap               │          │
│  │  - Variables whose addresses are returned        │          │
│  │  - Variables captured by goroutines/closures     │          │
│  │  - Interface values holding concrete data        │          │
│  │  - Variables too large for stack                 │          │
│  └──────────────────────────────────────────────────┘          │
└────────────────────────────────────────────────────────────────┘
```

```go
// Escape analysis: compiler decides stack vs heap

// ON STACK: local, doesn't escape
func noEscape() {
    x := 42       // stays on stack
    _ = x
}

// ON HEAP: escapes via returned pointer
func escapes() *int {
    x := 42       // x escapes to heap because its address is returned
    return &x
}

// To see escape analysis:
// go build -gcflags="-m" ./...
// Output: "x escapes to heap" or "x does not escape"

// PRACTICAL RULES to reduce heap allocations:
// 1. Return values, not pointers, when the caller doesn't need to share
// 2. Avoid storing local variables in interfaces
// 3. Pre-allocate slices when size is known
// 4. Use sync.Pool for frequently allocated/freed objects
```

### 22.2 The Garbage Collector

```
GO GC: Concurrent, tri-color mark-and-sweep

┌────────────────────────────────────────────────────────┐
│                 GC CYCLE                               │
│                                                        │
│  1. MARK START (STW: stop the world, ~microseconds)    │
│     - Start write barrier                              │
│     - Scan goroutine stacks for roots                  │
│                                                        │
│  2. MARK (concurrent with application)                 │
│     - Trace from roots through all live objects        │
│     - Objects colored: white(unknown)→grey→black(live) │
│     - Write barrier ensures new allocations are tracked│
│                                                        │
│  3. MARK TERMINATION (STW: stop the world, ~microseconds)│
│     - Flush remaining work                             │
│     - Turn off write barrier                           │
│                                                        │
│  4. SWEEP (concurrent with application)                │
│     - Free white (unreachable) objects                 │
│     - Return memory to spans                           │
│                                                        │
│  GOGC=100 means: GC when heap doubles from last GC     │
│  GOGC=off disables GC                                  │
│  runtime.GC() forces a GC cycle                        │
└────────────────────────────────────────────────────────┘
```

---

## 23. Reflection

### 23.1 reflect Package

```go
import "reflect"

// reflect.TypeOf: returns the type descriptor
t := reflect.TypeOf(42)          // reflect.Type representing int
t.Kind()                          // reflect.Int
t.String()                        // "int"

t2 := reflect.TypeOf(User{})
t2.Kind()                         // reflect.Struct
t2.NumField()                     // number of fields
field := t2.Field(0)              // reflect.StructField
field.Name                        // "Name"
field.Type                        // reflect.Type of field
field.Tag.Get("json")             // json tag value

// reflect.ValueOf: returns the value
v := reflect.ValueOf(42)         // reflect.Value
v.Kind()                          // reflect.Int
v.Int()                           // 42 (as int64)

v2 := reflect.ValueOf("hello")
v2.String()                       // "hello"

// Modifying values requires a pointer
x := 42
v3 := reflect.ValueOf(&x).Elem()  // Elem() dereferences the pointer
v3.SetInt(100)                     // modifies x
fmt.Println(x)                     // 100

// Calling methods via reflection
v4 := reflect.ValueOf(&User{Name: "Alice"})
method := v4.MethodByName("String")
if method.IsValid() {
    results := method.Call(nil)
    fmt.Println(results[0].String())
}

// RULES:
// 1. Reflection is powerful but slow — use sparingly
// 2. Use type assertions instead of reflection when the type is known at compile time
// 3. reflect.Value.Interface() converts back to interface{}
// 4. Settable values require addressable variables (pointers)
```

---

## 24. Context Package

### 24.1 Context Propagation Pattern

```
REQUEST FLOW WITH CONTEXT:

HTTP Handler
    │
    ├── context.WithCancel(r.Context())
    │         │
    │         ├── Service Layer (passes ctx)
    │         │         │
    │         │         ├── DB Query (ctx with deadline)
    │         │         │   ctx.Done() → cancel long query
    │         │         │
    │         │         └── External API call (ctx)
    │         │             ctx.Done() → cancel HTTP request
    │         │
    │         └── Another goroutine (passes ctx)
    │                   ctx.Done() → goroutine stops
    │
    └── defer cancel() → propagates cancellation down the tree
```

```go
// CORRECT context key definition (avoid string collision)
type contextKey string
const userIDKey contextKey = "userID"

func withUserID(ctx context.Context, id int) context.Context {
    return context.WithValue(ctx, userIDKey, id)
}

func getUserID(ctx context.Context) (int, bool) {
    id, ok := ctx.Value(userIDKey).(int)
    return id, ok
}

// HTTP MIDDLEWARE PATTERN
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        userID, err := validateToken(token)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        ctx := withUserID(r.Context(), userID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## 25. Testing — Complete Rules

### 25.1 Test File Structure

```go
// File: math_test.go (must end in _test.go)
package math_test  // external test package (black-box testing)
// OR: package math  (internal test package, white-box testing)

import (
    "testing"
    "github.com/yourname/yourproject/math"
)

// TEST FUNCTION: must start with Test, takes *testing.T
func TestAdd(t *testing.T) {
    got := math.Add(2, 3)
    want := 5
    if got != want {
        t.Errorf("Add(2, 3) = %d, want %d", got, want)
    }
}

// t.Error/t.Errorf  - marks test failed, continues execution
// t.Fatal/t.Fatalf  - marks test failed, stops test function immediately
// t.Log/t.Logf      - prints only when test fails (or with -v flag)
// t.Skip/t.Skipf    - skip this test with a message
// t.Helper()        - marks caller as test helper (better error locations)
```

### 25.2 Table-Driven Tests

```go
// THE canonical Go testing pattern
func TestAddTableDriven(t *testing.T) {
    tests := []struct {
        name string
        a, b int
        want int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -1, -2, -3},
        {"mixed", -1, 5, 4},
        {"zeros", 0, 0, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {  // subtests: go test -run TestAddTableDriven/positive
            got := Add(tt.a, tt.b)
            if got != tt.want {
                t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
            }
        })
    }
}

// t.Run creates a SUBTEST that can be run individually:
// go test -run TestAddTableDriven/zeros
```

### 25.3 Benchmarks

```go
// BENCHMARK FUNCTION: must start with Benchmark, takes *testing.B
func BenchmarkAdd(b *testing.B) {
    // b.N is automatically adjusted by testing framework
    for i := 0; i < b.N; i++ {
        Add(2, 3)
    }
}

// Run: go test -bench=. -benchmem
// Output:
// BenchmarkAdd-8   1000000000   0.2848 ns/op   0 B/op   0 allocs/op

// Setup before benchmark loop:
func BenchmarkSort(b *testing.B) {
    data := make([]int, 1000)
    // ... fill data ...
    b.ResetTimer()    // don't count setup in benchmark
    for i := 0; i < b.N; i++ {
        sort.Ints(data)
    }
}

// Parallel benchmark:
func BenchmarkParallel(b *testing.B) {
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            // operation to benchmark
        }
    })
}
```

### 25.4 Test Helpers and Fixtures

```go
// testdata/ directory: place test fixture files here
// They are NOT compiled but are available via filesystem

// Example test file that reads fixtures:
func TestParse(t *testing.T) {
    data, err := os.ReadFile("testdata/input.json")
    if err != nil {
        t.Fatal(err)
    }
    // ...
}

// TestMain: special function for test setup/teardown
func TestMain(m *testing.M) {
    // SETUP: runs before any test
    setup()

    code := m.Run()  // runs all tests

    // TEARDOWN: runs after all tests
    teardown()

    os.Exit(code)
}

// t.TempDir(): creates a temp dir that is automatically cleaned up
func TestTempFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "test.txt")
    os.WriteFile(path, []byte("hello"), 0644)
    // dir is removed when test finishes
}
```

---

## 26. Build System, Tags & Tools

### 26.1 Build Tags / Build Constraints

```go
// Place at the TOP of the file, before package declaration
// Go 1.17+: new syntax

//go:build linux && amd64
// The above means: include this file only when GOOS=linux AND GOARCH=amd64

//go:build linux || darwin
// Include on linux OR darwin

//go:build !windows
// Exclude on windows

//go:build ignore
// Never include (useful for documentation examples)

// Old syntax (pre-1.17, still supported):
// +build linux amd64   ← space = AND
// +build linux darwin  ← space = AND, multiple lines = OR

// Custom tags:
//go:build integration

// Run tests with custom tag:
// go test -tags integration ./...

// PREDEFINED build tags:
// GOOS:   linux, darwin, windows, freebsd, ...
// GOARCH: amd64, arm64, 386, ...
// Special: ignore, cgo
```

### 26.2 go:generate

```go
//go:generate stringer -type=Status
//go:generate mockgen -source=interface.go -destination=mock.go

// Run: go generate ./...
// This runs the commands in comments starting with //go:generate
// Common use: generating code from interfaces, enums, protobufs
```

### 26.3 Compiler Directives

```go
//go:noinline   // prevent the compiler from inlining this function
func mustNotInline() {}

//go:nosplit    // prevent stack split check (dangerous, avoid unless you know what you're doing)

//go:linkname localName importpath.name  // link to unexported symbol in another package

//go:noescape  // function arguments don't escape to heap
```

---

## 27. Idiomatic Go Patterns

### 27.1 Accept Interfaces, Return Concrete Types

```go
// GOOD: accept interface → flexible, testable
func Process(r io.Reader) error {
    // works with os.File, bytes.Buffer, http.Response.Body, anything
}

// BAD: accept concrete type → rigid
func Process(f *os.File) error { ... }

// GOOD: return concrete type → caller can access all fields/methods
func NewHTTPClient(timeout time.Duration) *http.Client {
    return &http.Client{Timeout: timeout}
}

// EXCEPTION: return interface when hiding implementation is important
func NewStorage(cfg Config) Storage { ... }  // hides *s3Storage, *localStorage, etc.
```

### 27.2 Functional Options Pattern

```go
// For structs with many optional configuration fields
type Server struct {
    host    string
    port    int
    timeout time.Duration
    maxConn int
}

type Option func(*Server)

func WithHost(host string) Option {
    return func(s *Server) {
        s.host = host
    }
}

func WithPort(port int) Option {
    return func(s *Server) {
        s.port = port
    }
}

func WithTimeout(d time.Duration) Option {
    return func(s *Server) {
        s.timeout = d
    }
}

func NewServer(opts ...Option) *Server {
    s := &Server{
        host:    "localhost",   // sensible defaults
        port:    8080,
        timeout: 30 * time.Second,
        maxConn: 100,
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage: clean, self-documenting
s := NewServer(
    WithPort(9090),
    WithTimeout(60*time.Second),
)
```

### 27.3 Error Handling Patterns

```go
// Pattern: Single error check wrapping for hot paths
type errWriter struct {
    w   io.Writer
    err error
}

func (ew *errWriter) Write(buf []byte) {
    if ew.err != nil {
        return  // skip if already errored
    }
    _, ew.err = ew.w.Write(buf)
}

func writeHeader(w io.Writer, data Data) error {
    ew := &errWriter{w: w}
    ew.Write(data.Magic[:])
    ew.Write(data.Version[:])
    ew.Write(data.Checksum[:])
    return ew.err  // check once at the end
}
```

### 27.4 Worker Pool Pattern

```go
func workerPool(ctx context.Context, jobs <-chan Job, numWorkers int) <-chan Result {
    results := make(chan Result, numWorkers)
    var wg sync.WaitGroup

    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case job, ok := <-jobs:
                    if !ok {
                        return
                    }
                    result := process(job)
                    results <- result
                case <-ctx.Done():
                    return
                }
            }
        }()
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    return results
}
```

---

## 28. Performance Engineering

### 28.1 Profiling with pprof

```go
import (
    "net/http"
    _ "net/http/pprof"   // registers pprof HTTP handlers
)

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    // ... your application ...
}

// Then:
// go tool pprof http://localhost:6060/debug/pprof/heap
// go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
// go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### 28.2 Common Performance Rules

```go
// RULE 1: Avoid unnecessary allocations in hot paths
// BAD: allocates new string every call
func badConcat(parts []string) string {
    result := ""
    for _, p := range parts {
        result += p     // creates new string each iteration!
    }
    return result
}

// GOOD: single allocation
func goodConcat(parts []string) string {
    return strings.Join(parts, "")  // or strings.Builder
}

// RULE 2: Preallocate slices and maps when size is known
// BAD
func badMake(n int) []int {
    var s []int
    for i := 0; i < n; i++ {
        s = append(s, i)  // multiple reallocations
    }
    return s
}

// GOOD
func goodMake(n int) []int {
    s := make([]int, 0, n)  // preallocate capacity
    for i := 0; i < n; i++ {
        s = append(s, i)    // no reallocation
    }
    return s
}

// RULE 3: Avoid interface allocations in tight loops
// Converting value to interface causes heap allocation
// Use generics or typed code when performance matters

// RULE 4: Benchmark before optimizing
// go test -bench=. -benchmem -count=5
// Never optimize without measuring first
```

---

## 29. The Go Runtime Scheduler

### 29.1 Work Stealing Scheduler

```
WORK STEALING ALGORITHM:

Each P has a local run queue (deque - double-ended queue).

Normal operation:
┌───┐         ┌───┐         ┌───┐
│ P1│         │ P2│         │ P3│
│[G1│G2│G3]  │[G4│G5]      │[G6│G7│G8│G9]
└───┘         └───┘         └───┘
 M1 runs G1    M2 runs G4    M3 runs G6

When P1's queue is empty:
1. P1 checks GLOBAL queue first
2. If global queue empty → STEAL from P3's queue
   P3's queue: [G7│G8│G9]  (steals half: G8, G9)
   P1 gets:    [G8│G9]
   P3 keeps:   [G7]

GOROUTINE PREEMPTION (Go 1.14+):
- Signal-based preemption: goroutines can be preempted at any point
- Previously: only at function call sites (could starve if tight loop)

SCHEDULER EVENTS (when goroutine yields):
- Channel operations that block
- network I/O
- time.Sleep
- runtime.Gosched() (explicit yield)
- system calls (go uses non-blocking syscalls via netpoller)
- Preemption signal (Go 1.14+)
```

### 29.2 GOMAXPROCS

```go
import "runtime"

// GOMAXPROCS = number of OS threads that can execute Go code simultaneously
// Defaults to number of CPU cores
current := runtime.GOMAXPROCS(0)  // 0 = query, don't change
old := runtime.GOMAXPROCS(4)      // set to 4

// In production containers:
// If your container is limited to 2 CPUs, GOMAXPROCS should be 2
// Use https://github.com/uber-go/automaxprocs to set automatically

// Goroutine count
n := runtime.NumGoroutine()   // current number of goroutines

// Force GC
runtime.GC()

// Print stack of all goroutines (debugging)
buf := make([]byte, 1<<20)
n2 := runtime.Stack(buf, true)  // true = all goroutines
fmt.Printf("%s\n", buf[:n2])
```

---

## 30. Common Pitfalls & Rules to Never Break

### 30.1 The Critical Rules Checklist

```
╔═══════════════════════════════════════════════════════════════════╗
║                  NEVER BREAK THESE RULES                         ║
╠═══════════════════════════════════════════════════════════════════╣
║ MEMORY & POINTERS                                                 ║
║ ✗ Never return pointer to local variable in C-style (Go is fine) ║
║ ✗ Never dereference nil pointer (panic!)                          ║
║ ✗ Never use unsafe.Pointer unless you truly need to               ║
║                                                                   ║
║ GOROUTINES & CHANNELS                                             ║
║ ✗ Never close a channel from the receiver side                    ║
║ ✗ Never close a channel twice (panic!)                            ║
║ ✗ Never send on a closed channel (panic!)                         ║
║ ✗ Never write to a nil map (panic!)                               ║
║ ✗ Never access a map concurrently without sync                    ║
║ ✗ Never start a goroutine without a way to know when it ends      ║
║ ✗ Never ignore done channels / context cancellation               ║
║                                                                   ║
║ DEFER                                                             ║
║ ✗ Never defer without closing resources (file, db, lock)          ║
║ ✗ Never defer in a tight loop (defers accumulate!)                ║
║                                                                   ║
║ ERRORS                                                            ║
║ ✗ Never ignore errors (assign to _ is a smell — document why!)    ║
║ ✗ Never compare errors with == when wrapping is involved          ║
║ ✗ Never return (*ConcreteErr)(nil) where interface is expected     ║
║                                                                   ║
║ INTERFACES                                                        ║
║ ✗ Never define interfaces before you have 2+ implementations      ║
║ ✗ Never use interface{} when a concrete type will do              ║
║                                                                   ║
║ HTTP                                                              ║
║ ✗ Never forget defer resp.Body.Close()                            ║
║ ✗ Never use http.DefaultClient in production (no timeout!)        ║
║                                                                   ║
║ GENERAL                                                           ║
║ ✗ Never shadow err in nested if blocks without intention          ║
║ ✗ Never rely on map iteration order                               ║
║ ✗ Never assume goroutine execution order                          ║
╚═══════════════════════════════════════════════════════════════════╝
```

### 30.2 Common Pitfalls with Examples

```go
// PITFALL 1: Loop variable capture (pre-Go 1.22)
for _, v := range items {
    go func() {
        process(v)  // BUG: v is captured by reference, all goroutines see last value
    }()
}
// FIX:
for _, v := range items {
    v := v  // new variable per iteration
    go func() { process(v) }()
}

// PITFALL 2: defer in loop accumulates
for _, f := range files {
    f, _ := os.Open(f)
    defer f.Close()   // BAD: defers accumulate, all close at function exit
}
// FIX: wrap in function
for _, fname := range files {
    func() {
        f, _ := os.Open(fname)
        defer f.Close()  // closes at end of anonymous function
        process(f)
    }()
}

// PITFALL 3: Shadowing err
if err := doA(); err != nil {
    return err
}
result, err := doB()  // OK: err re-declared, but it's a new variable here
// This is fine — := redeclares err in the same scope

// But this is a trap:
err := doA()
if err != nil {
    if err := doB(); err != nil {  // inner err shadows outer err
        return err
    }
    // outer err still has doA's error here
}

// PITFALL 4: Slice memory leak
func getFirst(data []byte) []byte {
    return data[:5]  // BAD: keeps entire original data array in memory!
}
// FIX:
func getFirst(data []byte) []byte {
    result := make([]byte, 5)
    copy(result, data[:5])
    return result  // original data can be GC'd
}

// PITFALL 5: Goroutine leak
func leaky() {
    ch := make(chan int)
    go func() {
        val := <-ch  // this goroutine will NEVER exit if nobody sends
        fmt.Println(val)
    }()
    // ch never gets a value → goroutine blocked forever → leak
}
// FIX: Use done channel or timeout or context

// PITFALL 6: Mutex copied
type SafeCounter struct {
    mu sync.Mutex  // mutex must NOT be copied after first use
    n  int
}
// BAD: copying a sync.Mutex that's already been used can cause deadlock
func bad(c SafeCounter) { ... }   // c is a COPY — mutex state is copied!
// FIX: always pass mutex-containing structs by pointer
func good(c *SafeCounter) { ... }

// PITFALL 7: HTTP body not closed
resp, _ := http.Get(url)
body, _ := io.ReadAll(resp.Body)  // BAD if Close is not called
// FIX:
resp, err := http.Get(url)
if err != nil {
    return err
}
defer resp.Body.Close()  // ALWAYS defer close immediately after getting resp
body, err := io.ReadAll(resp.Body)
```

### 30.3 Go Proverbs (by Rob Pike)

```
"Don't communicate by sharing memory; share memory by communicating."
→ Use channels to transfer data ownership between goroutines.

"Concurrency is not parallelism."
→ Structure your program with goroutines; let the runtime decide parallelism.

"Channels orchestrate; mutexes serialize."
→ Use channels to coordinate goroutines; mutexes to protect shared state.

"The bigger the interface, the weaker the abstraction."
→ Prefer small, focused interfaces (io.Reader has one method for a reason).

"Make the zero value useful."
→ Design types so the zero value works without initialization.
  sync.Mutex, bytes.Buffer, sync.WaitGroup all work at zero value.

"interface{} says nothing."
→ Use it only when you truly can't know the type at compile time.

"Gofmt's style is no one's favorite, yet gofmt is everyone's favorite."
→ Format with gofmt. Stop arguing about style.

"A little copying is better than a little dependency."
→ Don't import a package for one function. Copy the 5-line utility.

"Syscall must always be guarded with build tags."
→ Platform-specific code belongs behind build constraints.

"Errors are values."
→ Errors are not special — they are just values. Handle them explicitly.

"Don't just check errors, handle them gracefully."
→ Wrap errors with context. Decide what to do with them.

"Design the architecture, name the components, document the details."
→ Think before writing. Name things well. Comment the why, not the what.
```

---

## Final Mental Model Summary

```
┌──────────────────────────────────────────────────────────────────────┐
│                    THE GO MENTAL MODEL HIERARCHY                     │
│                                                                      │
│  THINK IN TYPES:                                                     │
│    Value types (copy on assignment):                                 │
│      bool, int*, float*, complex*, string, array, struct             │
│    Reference types (share underlying data):                          │
│      slice, map, chan, pointer, func, interface                      │
│                                                                      │
│  THINK IN INTERFACES:                                                │
│    Small interfaces (1-3 methods) + composition                      │
│    Program to the interface; wire in implementations                 │
│                                                                      │
│  THINK IN ERRORS:                                                    │
│    Error is a return value, not an exception                         │
│    Wrap with context, check at every boundary                        │
│                                                                      │
│  THINK IN GOROUTINES:                                                │
│    Each independent concurrent activity = goroutine                  │
│    Communicate via channels (ownership transfer)                     │
│    Protect shared state with sync primitives                         │
│    Always have a way to stop goroutines (context/done channel)       │
│                                                                      │
│  THINK IN PACKAGES:                                                  │
│    Package = unit of compilation, encapsulation, and API surface     │
│    Exported = public API; unexported = implementation detail         │
│    Depend on interfaces, not concrete types                          │
│                                                                      │
│  THINK IN COMPOSITION:                                               │
│    No inheritance. Use struct embedding for behavior reuse           │
│    Satisfy interfaces by having the right methods                    │
│    Combine small pieces into larger behaviors                        │
└──────────────────────────────────────────────────────────────────────┘
```

---

*This guide covers the complete Go specification, runtime model, standard library essentials, idiomatic patterns, and engineering rules needed to think and program effectively in Go. Keep it as a reference — return to each section as you build deeper experience.*
