# Syntax Guide: C vs Rust vs Go
## A Complete Side-by-Side Reference

When you switch between C, Rust, and Go your fingers type the wrong thing because
each language made different choices about where types go, how to express
"no value", how to write loops, and dozens of other small things. This guide
puts every construct next to each other so your mental model stays sharp.

---

## Table of Contents

1.  [Anatomy of a Source File](#1-anatomy-of-a-source-file)
2.  [Variables and Bindings](#2-variables-and-bindings)
3.  [Primitive Types](#3-primitive-types)
4.  [Type Annotations: Where They Go](#4-type-annotations-where-they-go)
5.  [Functions](#5-functions)
6.  [Control Flow](#6-control-flow)
7.  [Pointers and References](#7-pointers-and-references)
8.  [Structs and Methods](#8-structs-and-methods)
9.  [Arrays, Slices, and Vectors](#9-arrays-slices-and-vectors)
10. [Strings](#10-strings)
11. [Enums and Sum Types](#11-enums-and-sum-types)
12. [Pattern Matching](#12-pattern-matching)
13. [Traits and Interfaces](#13-traits-and-interfaces)
14. [Generics](#14-generics)
15. [Closures and Function Types](#15-closures-and-function-types)
16. [Error Handling](#16-error-handling)
17. [Memory and Allocation Syntax](#17-memory-and-allocation-syntax)
18. [Concurrency](#18-concurrency)
19. [Modules, Packages, and Imports](#19-modules-packages-and-imports)
20. [Type Casting and Conversion](#20-type-casting-and-conversion)
21. [Constants and Statics](#21-constants-and-statics)
22. [Null / None / Nil](#22-null--none--nil)
23. [Operators](#23-operators)
24. [Semicolons, Braces, and Whitespace Rules](#24-semicolons-braces-and-whitespace-rules)
25. [Macros and Code Generation](#25-macros-and-code-generation)
26. [Comments](#26-comments)
27. [Complete Cheat Sheet](#27-complete-cheat-sheet)

---

## 1. Anatomy of a Source File

Understanding the skeleton of a file in each language first.

```
===== C =====                ===== Rust =====              ===== Go =====

#include <stdio.h>           use std::fmt;                 package main
#include <stdlib.h>          use std::collections::        
                               HashMap;                    import (
// File-level constant                                          "fmt"
#define MAX 100              // File-level constant             "os"
                             const MAX: usize = 100;       )
// File-level variable
int global = 0;              // File-level variable        // File-level variable
                             static GLOBAL: i32 = 0;      var Global int = 0
// Function declaration
int add(int, int);           // No forward declarations    // No forward declarations
                             // needed in Rust             // needed in Go

// Function definition
int add(int a, int b) {      fn add(a: i32, b: i32)       func add(a, b int) int {
    return a + b;                -> i32 {                      return a + b
}                                a + b    // no semicolon  }
                             }
int main(void) {             fn main() {                   func main() {
    printf("hello\n");           println!("hello");            fmt.Println("hello")
    return 0;                }                             }
}
```

**Key differences at a glance:**
- C requires forward declarations for functions used before their definition.
- Rust uses `use` to bring names into scope; Go uses `import`.
- Go requires a `package` declaration at the top of every file.
- `main` returns `void` (implicitly `return 0`) in C; nothing in Rust and Go.
- Rust: last expression without `;` is the implicit return value.

---

## 2. Variables and Bindings

This is where most confusion begins. The syntax for declaring and mutating variables differs significantly.

```
+------------------------------------------------------------------+
|   C                                                              |
+------------------------------------------------------------------+
  int x;              // uninitialized (UNDEFINED VALUE — dangerous)
  int x = 5;          // initialized
  int x, y, z;        // multiple declarations
  int x = 5, y = 10;  // multiple initialized
  const int x = 5;    // immutable — cannot reassign
  x = 10;             // mutation — always allowed (unless const)

+------------------------------------------------------------------+
|   Rust                                                           |
+------------------------------------------------------------------+
  let x;              // uninitialized — compile error if used before init
  let x = 5;          // initialized, IMMUTABLE by default
  let x: i32 = 5;     // with explicit type annotation
  let mut x = 5;      // MUTABLE — requires mut keyword
  let mut x: i32 = 5; // mutable with type
  x = 10;             // OK only if declared with `mut`
  let x = 5;          // shadowing: re-declare with same name
  let x = x + 1;      // x is now 6 (new binding, shadows old one)

+------------------------------------------------------------------+
|   Go                                                             |
+------------------------------------------------------------------+
  var x int           // zero-initialized (0 for int — ALWAYS safe)
  var x int = 5       // initialized with type
  var x = 5           // type inferred (x is int)
  x := 5              // SHORT DECLARATION — only inside functions
  x, y := 5, 10       // multiple short declarations
  var x, y int = 5, 10// multiple with explicit type
  x = 10              // mutation — always allowed (no const-by-default)
  x := 10             // ERROR if x already declared in same scope
```

**ASCII diagram — mutation rules:**
```
C:        declare → use → mutate     (all always allowed unless const)
           int x = 5;  x++;

Rust:     let x = 5;   x = 10;  // ERROR: cannot assign to immutable variable
          let mut x = 5;  x = 10;  // OK

Go:       x := 5;  x = 10;  // OK, all vars are mutable
          y := x;           // := always means "new binding"
          y = x             // = always means "assignment to existing"
```

**Rust shadowing vs mutation — these are NOT the same:**
```rust
let x = 5;
let x = x + 1;     // shadowing: new binding named x, value 6
// first x is gone from scope

let mut y = 5;
y = y + 1;         // mutation: same binding y, value 6

// Shadowing allows changing type:
let spaces = "   ";         // &str
let spaces = spaces.len();  // usize — totally different type, same name
// Mutation does NOT allow this:
let mut spaces = "   ";
spaces = spaces.len();      // ERROR: mismatched types
```

**Go zero values — every declared variable has a known safe default:**
```go
var i int       // 0
var f float64   // 0.0
var b bool      // false
var s string    // ""  (empty string)
var p *int      // nil
var sl []int    // nil  (nil slice)
var m map[string]int  // nil
```

**C uninitialized variables are a major bug source:**
```c
int x;           // x could be ANYTHING — whatever was in that stack address
printf("%d", x); // UNDEFINED BEHAVIOR
```

---

## 3. Primitive Types

```
+-------------------+------------------+------------------+------------------+
| Concept           |       C          |      Rust        |       Go         |
+-------------------+------------------+------------------+------------------+
| Signed integer    | int (platform)   | i8, i16, i32,    | int (platform)   |
| (various sizes)   | short, long,     | i64, i128        | int8, int16,     |
|                   | long long        | isize (platform) | int32, int64     |
+-------------------+------------------+------------------+------------------+
| Unsigned integer  | unsigned int     | u8, u16, u32,    | uint, uint8,     |
|                   | unsigned short   | u64, u128        | uint16, uint32,  |
|                   | size_t           | usize (platform) | uint64, uintptr  |
+-------------------+------------------+------------------+------------------+
| Floating point    | float (32-bit)   | f32              | float32          |
|                   | double (64-bit)  | f64              | float64          |
+-------------------+------------------+------------------+------------------+
| Boolean           | _Bool / bool     | bool             | bool             |
|                   | (C99+)           | true / false     | true / false     |
+-------------------+------------------+------------------+------------------+
| Character         | char (1 byte)    | char (4-byte     | rune (int32,     |
|                   | 'A' = 65         | Unicode scalar)  | Unicode code pt) |
|                   |                  | 'A', 'α', '🎉'   | byte = uint8     |
+-------------------+------------------+------------------+------------------+
| Byte              | unsigned char    | u8               | byte (= uint8)   |
+-------------------+------------------+------------------+------------------+
| Void / Unit       | void             | () "unit type"   | (no equivalent)  |
+-------------------+------------------+------------------+------------------+
| Never / Bottom    | (none)           | ! "never type"   | (none)           |
+-------------------+------------------+------------------+------------------+
| Pointer           | int*             | *const i32       | *int             |
|                   | void*            | *mut i32         | unsafe.Pointer   |
|                   | NULL             | (no null)        | nil              |
+-------------------+------------------+------------------+------------------+
| Reference         | (none, use ptr)  | &i32, &mut i32   | (none, use ptr)  |
+-------------------+------------------+------------------+------------------+
| String (owned)    | char* + malloc   | String           | string (immut.)  |
+-------------------+------------------+------------------+------------------+
| String (borrowed) | const char*      | &str             | (string is value)|
+-------------------+------------------+------------------+------------------+
| Array             | int arr[5]       | [i32; 5]         | [5]int           |
+-------------------+------------------+------------------+------------------+
| Slice             | int*, size_t len | &[i32]           | []int            |
+-------------------+------------------+------------------+------------------+
| Tuple             | struct (manual)  | (i32, f64, bool) | (no built-in)    |
+-------------------+------------------+------------------+------------------+

Rust integer literal syntax:
  1_000_000    // underscores for readability (= 1000000)
  0xFF         // hex
  0o77         // octal
  0b1010       // binary
  42u8         // type suffix: u8
  42i64        // type suffix: i64
  3.14f32      // float type suffix

Go integer literal syntax:
  1_000_000    // underscores (Go 1.13+)
  0xFF         // hex
  0o77         // octal (Go 1.13+, old: 077)
  0b1010       // binary (Go 1.13+)
  42           // untyped constant (takes type from context)

C integer literal syntax:
  1000000      // decimal
  0xFF         // hex
  077          // octal (leading zero!)
  42u          // unsigned
  42L          // long
  42LL         // long long
```

**Watch out: C's `int` and Go's `int` are platform-dependent (32-bit on 32-bit systems, 64-bit on 64-bit). Rust's `isize/usize` are also platform-dependent, but `i32/i64/u32/u64` are always exactly that size.**

---

## 4. Type Annotations: Where They Go

The most disorienting syntax difference for multi-language programmers.

```
C:    TYPE   name              int    x = 5;
             ^^^^ TYPE FIRST

Rust: name : TYPE              x    : i32  = 5;
           ^ COLON SEPARATOR

Go:   name   TYPE              x      int  = 5;
             ^^^^ TYPE AFTER, NO COLON
```

**This extends to all declarations:**

```
+---------------------------+----------------------------+----------------------------+
|           C               |           Rust             |           Go               |
+---------------------------+----------------------------+----------------------------+
| VARIABLE                  | VARIABLE                   | VARIABLE                   |
|   int x = 5;              |   let x: i32 = 5;          |   var x int = 5            |
|                           |                            |   x := 5  (inferred)       |
+---------------------------+----------------------------+----------------------------+
| FUNCTION PARAM            | FUNCTION PARAM             | FUNCTION PARAM             |
|   void f(int x)           |   fn f(x: i32)             |   func f(x int)            |
+---------------------------+----------------------------+----------------------------+
| FUNCTION RETURN           | FUNCTION RETURN            | FUNCTION RETURN            |
|   int f(void)             |   fn f() -> i32            |   func f() int             |
|   ^^^                     |          ^^^^^^            |            ^^^             |
|   before name             |          after params      |            after params    |
+---------------------------+----------------------------+----------------------------+
| STRUCT FIELD              | STRUCT FIELD               | STRUCT FIELD               |
|   struct { int x; }       |   struct S { x: i32 }      |   struct S { X int }       |
|           ^^^ ^^^         |              ^  ^^^        |              ^ ^^^         |
|           type name       |              name type     |              name type     |
+---------------------------+----------------------------+----------------------------+
| POINTER                   | POINTER/REFERENCE          | POINTER                    |
|   int *p;                 |   let p: *const i32;       |   var p *int               |
|   char **pp;              |   let p: *mut i32;         |   var pp **int             |
|   const int *p;           |   let r: &i32;             |                            |
|                           |   let r: &mut i32;         |                            |
+---------------------------+----------------------------+----------------------------+
| ARRAY                     | ARRAY                      | ARRAY                      |
|   int arr[5];             |   let arr: [i32; 5];       |   var arr [5]int           |
|   ^^^     ^               |              ^^^  ^        |         ^   ^^^            |
|   type  size suffix       |              type size     |         size type          |
+---------------------------+----------------------------+----------------------------+
| FUNCTION POINTER          | FUNCTION TYPE              | FUNCTION TYPE              |
|   int (*fp)(int, int);    |   let fp: fn(i32,i32)->i32 |   var fp func(int,int) int |
+---------------------------+----------------------------+----------------------------+
```

**The key insight:**
- C: type information is **scattered** around the name, especially for pointers and function pointers.
- Rust and Go: type always comes **after** the name with a clear separator (`:` for Rust, space for Go).

---

## 5. Functions

```
+-------------------------------+-------------------------------+-------------------------------+
|             C                 |             Rust              |             Go                |
+-------------------------------+-------------------------------+-------------------------------+
|                               |                               |                               |
| // No arguments               | fn hello() {                  | func hello() {                |
| void hello(void) {            |     println!("hi");           |     fmt.Println("hi")         |
|     printf("hi\n");           | }                             | }                             |
| }                             |                               |                               |
+-------------------------------+-------------------------------+-------------------------------+
| // One return value           | fn double(x: i32) -> i32 {   | func double(x int) int {      |
| int double(int x) {           |     x * 2  // implicit return |     return x * 2              |
|     return x * 2;             | }                             | }                             |
| }                             |                               |                               |
+-------------------------------+-------------------------------+-------------------------------+
| // Multiple params            | fn add(a: i32, b: i32)        | func add(a, b int) int {      |
| int add(int a, int b) {       |     -> i32 {                  |     return a + b              |
|     return a + b;             |     a + b                     | }                             |
| }                             | }                             |                               |
|                               |                               | // Same types, shorthand:     |
|                               |                               | // func add(a, b int) int     |
+-------------------------------+-------------------------------+-------------------------------+
| // Multiple return values     |                               | // MULTIPLE RETURNS           |
| // Not built-in:              | // Tuples for multiple return | func minmax(a,b int)(int,int){|
| // use output params          | fn min_max(a:i32,b:i32)       |     if a < b {                |
| void min_max(int a, int b,    |     -> (i32, i32) {           |         return a, b           |
|   int *out_min, int *out_max){|     if a < b {                |     }                         |
|     *out_min = a < b ? a : b; |         (a, b)                |     return b, a               |
|     *out_max = a < b ? b : a; |     } else {                  | }                             |
| }                             |         (b, a)                |                               |
|                               |     }                         | min, max := minmax(3, 7)      |
|                               | }                             |                               |
|                               | let (mn, mx) = min_max(3, 7); |                               |
+-------------------------------+-------------------------------+-------------------------------+
| // Variadic                   | // No traditional variadic    | // Variadic                   |
| int sum(int n, ...) {         | // Use slices or macros       | func sum(nums ...int) int {   |
|   va_list args;               |                               |     total := 0                |
|   va_start(args, n);          | // Macros can be variadic:    |     for _, n := range nums {  |
|   ...                         | // println!("{} {}", a, b)    |         total += n            |
| }                             |                               |     }                         |
|                               |                               |     return total              |
|                               |                               | }                             |
|                               |                               | sum(1, 2, 3)     // variadic  |
|                               |                               | sum(arr...)      // spread    |
+-------------------------------+-------------------------------+-------------------------------+
| // Named return (not a thing) | // Named return (not common)  | // NAMED RETURNS              |
|                               | fn divide(a:i32,b:i32)        | func divide(a,b int)          |
|                               |     -> i32 {                  |     (result int, err error) { |
|                               |     // just use variable name |     if b == 0 {               |
|                               | }                             |         err = errors.New("0") |
|                               |                               |         return  // naked rtrn |
|                               |                               |     }                         |
|                               |                               |     result = a / b            |
|                               |                               |     return                    |
|                               |                               | }                             |
+-------------------------------+-------------------------------+-------------------------------+
```

**Rust's implicit return — the most surprising rule:**
```rust
fn add(a: i32, b: i32) -> i32 {
    a + b         // NO semicolon → this IS the return value (expression)
}

fn add_v2(a: i32, b: i32) -> i32 {
    return a + b; // explicit return with semicolon — also fine
}

fn add_v3(a: i32, b: i32) -> i32 {
    a + b;        // BUG: semicolon makes this a STATEMENT
                  // function now implicitly returns () but declared i32
                  // COMPILE ERROR
}

// Inside control flow:
fn abs(x: i32) -> i32 {
    if x >= 0 {
        x      // no semicolon — returned from if block
    } else {
        -x     // no semicolon — returned from else block
    }          // entire if-else is an expression — no `return` needed
}
```

---

## 6. Control Flow

### 6.1 if / else

```c
// C
if (x > 0) {
    printf("pos\n");
} else if (x < 0) {
    printf("neg\n");
} else {
    printf("zero\n");
}
// Condition MUST be in parens
// No parens = common mistake when porting to Go/Rust
```

```rust
// Rust
if x > 0 {             // NO parens around condition
    println!("pos");
} else if x < 0 {
    println!("neg");
} else {
    println!("zero");
}

// if is an EXPRESSION in Rust:
let description = if x > 0 { "pos" } else { "neg" };
//                ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
//                all branches must return same type
```

```go
// Go
if x > 0 {             // NO parens around condition
    fmt.Println("pos")
} else if x < 0 {
    fmt.Println("neg")
} else {
    fmt.Println("zero")
}

// Go: init statement in if
if err := doSomething(); err != nil {
    //  ^^^^^^^^^^^^^^^^ executed before condition
    //                   err scoped to this if block only
    log.Fatal(err)
}
```

### 6.2 Loops

```
+---------------------+------------------------+---------------------+
| C                   | Rust                   | Go                  |
+---------------------+------------------------+---------------------+
| // Infinite loop    | // Infinite loop        | // Infinite loop    |
| while (1) {         | loop {                  | for {               |
|     ...             |     ...                 |     ...             |
| }                   |     break;              |     break           |
|                     | }                       | }                   |
+---------------------+------------------------+---------------------+
| // While loop       | // While loop           | // While loop       |
| while (x < 10) {    | while x < 10 {          | for x < 10 {        |
|     x++;            |     x += 1;             |     x++             |
| }                   | }                       | }                   |
+---------------------+------------------------+---------------------+
| // do-while loop    | // loop + break         | // (no do-while)    |
| do {                | loop {                  | for {               |
|     x++;            |     x += 1;             |     x++             |
| } while (x < 10);   |     if x >= 10 { break }|     if x >= 10 {    |
|                     | }                       |         break       |
|                     |                         |     }               |
|                     |                         | }                   |
+---------------------+------------------------+---------------------+
| // C-style for loop | // No C-style for loop  | // C-style for loop |
| for (int i=0;       | // Use range or while:  | for i:=0; i<n; i++ {|
|      i < n; i++) {  | for i in 0..n {         |     arr[i] = i      |
|     arr[i] = i;     |     arr[i] = i;         | }                   |
| }                   | }                       |                     |
+---------------------+------------------------+---------------------+
| // Range-based (C99 | // Ranges (idiomatic)   | // Range            |
|  needs for + index) | for i in 0..10 { }      | for i := range 10 { |
| for (int i=0;       | for i in 0..=10 { }     | } // Go 1.22+       |
|   i<10; i++) { }    | // 0..10  = [0,9]       |                     |
|                     | // 0..=10 = [0,10]      | for i,v := range sl{|
|                     |                         |     // i=idx, v=val |
|                     | for (i, v) in           | }                   |
|                     |    arr.iter().enumerate()|                    |
|                     |  { /* i=idx, v=&val */} |                     |
+---------------------+------------------------+---------------------+
| // break/continue   | // break/continue       | // break/continue   |
| break;              | break;                  | break               |
| continue;           | continue;               | continue            |
+---------------------+------------------------+---------------------+
| // Labeled loops    | // Labeled loops (Rust) | // Labeled loops    |
| // (no built-in,    | 'outer: for i in 0..5 { | outer:              |
|  use goto)          |   'inner: for j in 0..5{| for i:=0; i<5; i++{|
|                     |     if cond {           |  for j:=0; j<5; j++{|
|                     |       break 'outer;     |    if cond {        |
|                     |     }                   |      break outer    |
|                     |   }                     |    }                |
|                     | }                       |  }                  |
|                     |                         | }                   |
+---------------------+------------------------+---------------------+
```

**Rust: `loop` can return a value:**
```rust
let result = loop {
    counter += 1;
    if counter == 10 {
        break counter * 2;   // break with a value
    }
};
// result == 20
```

**Go: `range` forms:**
```go
// Over a slice:
for i, v := range slice { }      // i = index, v = copy of element
for i := range slice { }         // index only
for _, v := range slice { }      // value only (discard index with _)

// Over a map:
for k, v := range myMap { }      // key, value (order not guaranteed)
for k := range myMap { }         // key only

// Over a string (iterates Unicode runes, not bytes):
for i, r := range "héllo" { }    // i = byte index, r = rune (Unicode code point)

// Over a channel:
for v := range ch { }            // receives until channel closed

// Over integer (Go 1.22+):
for i := range 10 { }            // 0..9
```

### 6.3 Switch / Match

```c
// C: switch (integer/char only, falls through by default)
switch (x) {
    case 1:
        printf("one\n");
        break;      // MUST break manually — otherwise falls through!
    case 2:
        printf("two\n");
        break;
    case 3:
    case 4:
        printf("three or four\n"); // fall-through to combine cases
        break;
    default:
        printf("other\n");
}
```

```rust
// Rust: match (exhaustive, no fall-through, value is an expression)
match x {
    1 => println!("one"),          // no fall-through ever
    2 => println!("two"),
    3 | 4 => println!("three or four"),  // OR pattern
    5..=10 => println!("five to ten"),   // range pattern
    _ => println!("other"),        // _ = catch-all (required if not exhaustive)
}

// match is an expression:
let s = match x {
    1 => "one",
    2 => "two",
    _ => "other",
};

// with guard conditions:
match x {
    n if n < 0 => println!("negative"),
    0 => println!("zero"),
    n if n > 100 => println!("big"),
    _ => println!("normal"),
}
```

```go
// Go: switch (no fall-through by default, can switch on any type)
switch x {
case 1:
    fmt.Println("one")       // no break needed
case 2:
    fmt.Println("two")
case 3, 4:
    fmt.Println("three or four")  // comma for multiple values
default:
    fmt.Println("other")
}

// Explicit fall-through (opposite of C):
switch x {
case 1:
    fmt.Println("one")
    fallthrough             // explicitly fall to next case
case 2:
    fmt.Println("one or two")
}

// Expression-less switch (like if-else chain):
switch {
case x < 0:
    fmt.Println("negative")
case x == 0:
    fmt.Println("zero")
default:
    fmt.Println("positive")
}

// Type switch:
switch v := i.(type) {
case int:
    fmt.Printf("int: %d\n", v)
case string:
    fmt.Printf("string: %s\n", v)
default:
    fmt.Printf("unknown: %T\n", v)
}
```

---

## 7. Pointers and References

This is often the biggest confusion between the three languages.

### 7.1 Creating pointers

```
Operation          C                  Rust                    Go
-----------------  -----------------  ----------------------  -------------------
Take address       &x                 &x  (shared ref)        &x
                                      &mut x (mutable ref)
                   
Dereference        *p                 *p                      *p

Null pointer       NULL               (no null refs)          nil

Pointer type       int *              &i32 / &mut i32         *int
                   void *             *const i32 (raw)        *interface{}
                                      *mut i32 (raw mutable)  unsafe.Pointer

Declare + assign   int *p = &x;       let p = &x;             p := &x
                                      let p: &i32 = &x;       var p *int = &x

Write through      *p = 10;           *p = 10;  // needs &mut  *p = 10
pointer                               // shared ref: READ ONLY

Pointer arith      p + 1, p++         ptr.add(1) in unsafe    uintptr(unsafe.Pointer(p)) + 8
(to next element)  *(p + n)           ptr.offset(n) in unsafe  (rare, requires unsafe pkg)
```

### 7.2 The pointer/reference type syntax

```
C POINTER SYNTAX — confusing because * binds to name, not type:
    int *p;          // pointer to int
    int **pp;        // pointer to pointer to int
    int *arr[5];     // array of 5 pointers to int
    int (*arr)[5];   // pointer to array of 5 ints (parens change parsing!)
    const int *p;    // pointer to const int (can't change int through p)
    int * const p;   // const pointer to int (can't change p itself)
    const int * const p;  // const pointer to const int

RUST REFERENCE SYNTAX — clear and unambiguous:
    &i32             // shared reference to i32
    &mut i32         // mutable (exclusive) reference to i32
    *const i32       // raw pointer (immutable) — only in unsafe
    *mut i32         // raw pointer (mutable) — only in unsafe
    &&i32            // reference to reference
    &[i32]           // slice reference (fat pointer: ptr + len)
    &mut [i32]       // mutable slice reference
    &dyn Trait       // trait object reference (fat pointer: ptr + vtable)

GO POINTER SYNTAX — simpler than C:
    *int             // pointer to int
    **int            // pointer to pointer to int
    *[]int           // pointer to slice
    unsafe.Pointer   // raw untyped pointer (like void*)
    uintptr          // integer big enough to hold a pointer value
```

### 7.3 The null safety difference

```
C: Any pointer can be NULL. Dereferencing NULL → crash.
   You MUST check manually.
   int *p = get_value();
   if (p == NULL) { ... }   // easy to forget
   *p = 10;                 // crash if forgot to check

Rust: References are NEVER null. Guaranteed by type system.
      &i32 always points to a valid i32.
      To express "might not have a value" → Option<&i32>:
        None    → no value
        Some(r) → r is a valid &i32, definitely non-null
      You CANNOT dereference None — must handle it.

Go: Pointers CAN be nil. Dereferencing nil → runtime panic.
    var p *int = nil
    *p = 10  // panic: nil pointer dereference
    if p != nil { *p = 10 }  // must check
    // Go is like C in this regard — nil pointer panics are common bugs
```

---

## 8. Structs and Methods

### 8.1 Defining structs

```c
// C: struct keyword required at use site (unless typedef)
struct Point {
    int x;      // ; after each field
    int y;
};

// C: typedef to avoid writing 'struct' everywhere
typedef struct {
    int x;
    int y;
} Point;

// C: nested struct
typedef struct {
    Point origin;
    int width;
    int height;
} Rect;
```

```rust
// Rust: no struct keyword at use site, no semicolons between fields
struct Point {
    x: i32,    // comma after each field (except last is optional)
    y: i32,
}

// Rust: tuple struct (positional fields)
struct Color(u8, u8, u8);      // accessed as c.0, c.1, c.2

// Rust: unit struct (no fields — useful as marker)
struct Marker;

// Rust: nested
struct Rect {
    origin: Point,
    width: i32,
    height: i32,
}
```

```go
// Go: type keyword with struct
type Point struct {
    X int    // exported (starts with uppercase)
    Y int    // no semicolons — newlines or semicolons both work
}

// Go: unexported fields (lowercase, only accessible in same package)
type point struct {
    x int
    y int
}

// Go: embedded (anonymous) fields — a form of composition/inheritance
type ColorPoint struct {
    Point           // embedded — ColorPoint "inherits" X, Y fields
    Color  string
}
cp := ColorPoint{}
cp.X = 5            // accesses embedded Point.X directly
```

### 8.2 Creating struct instances

```
C:                          Rust:                        Go:
// Designated initializer   // Struct literal             // Struct literal
Point p = { .x=1, .y=2 };  let p = Point { x: 1, y: 2 };  p := Point{X: 1, Y: 2}
Point p = { 1, 2 };         // positional — risky         p := Point{1, 2}  // positional

// Stack allocated           // Stack allocated            // Stack allocated
Point p;                    let p: Point;                var p Point  // zero-initialized!
p.x = 1;                    // p uninitialized —          p.X = 1
p.y = 2;                    // must init before use       p.Y = 2

// Heap allocated            // Heap allocated             // Heap allocated
Point *p = malloc(          let p = Box::new(            p := &Point{X: 1, Y: 2}
    sizeof(Point));             Point { x: 1, y: 2 });   // OR:
p->x = 1;                                                p := new(Point)
p->y = 2;                                                p.X = 1  // auto-deref

// Update syntax (copy):     // Struct update syntax:     // No equivalent — must copy
// (no built-in — copy whole)let p2 = Point {            //  manually or use function
Point p2 = p;               //   x: 10,
p2.x = 10;                  //   ..p  // rest from p
                            // };
```

### 8.3 Methods

```c
// C: no methods — use free functions with struct pointer
void point_translate(Point *p, int dx, int dy) {
    p->x += dx;   // -> dereferences pointer AND accesses field
    p->y += dy;   // equivalent to (*p).y += dy
}

// Call:
Point p = {1, 2};
point_translate(&p, 3, 4);   // must pass address explicitly
```

```rust
// Rust: impl block defines methods
impl Point {
    // Associated function (like static method, no self)
    fn new(x: i32, y: i32) -> Self {
        Point { x, y }    // field shorthand: x means x: x
    }

    // Method taking shared reference (read-only)
    fn distance_from_origin(&self) -> f64 {
        ((self.x * self.x + self.y * self.y) as f64).sqrt()
    }

    // Method taking mutable reference (can modify)
    fn translate(&mut self, dx: i32, dy: i32) {
        self.x += dx;   // no -> in Rust, always . for fields
        self.y += dy;
    }

    // Method consuming self (takes ownership)
    fn into_tuple(self) -> (i32, i32) {
        (self.x, self.y)   // self moved into function, caller can't use after
    }
}

// Call:
let mut p = Point::new(1, 2);   // :: for associated functions
p.translate(3, 4);              // . for methods (auto-ref/deref)
let d = p.distance_from_origin();
```

```go
// Go: methods defined outside struct body, with receiver
// Value receiver (copy of struct — cannot mutate original):
func (p Point) Distance() float64 {
    return math.Sqrt(float64(p.X*p.X + p.Y*p.Y))
}

// Pointer receiver (can mutate, avoids copy for large structs):
func (p *Point) Translate(dx, dy int) {
    p.X += dx   // no -> in Go, auto-deref for pointer receivers
    p.Y += dy
}

// Constructor convention (no built-in constructors):
func NewPoint(x, y int) *Point {
    return &Point{X: x, Y: y}
}

// Call:
p := NewPoint(1, 2)
p.Translate(3, 4)    // Go auto-takes address: (&p).Translate(3,4)
d := p.Distance()
```

**The `->` operator:**
```
C:    p->field    =   (*p).field    (pointer dereference + field access)
      Must use -> when p is a pointer, . when p is a value

Rust: Always use .  — Rust auto-deref:
      p.field   works whether p is Point, &Point, &mut Point, Box<Point>
      No -> operator in safe Rust

Go:   Always use .  — Go auto-deref:
      p.Field   works whether p is Point or *Point
      No -> operator in Go
```

---

## 9. Arrays, Slices, and Vectors

### 9.1 Fixed-size arrays

```
Operation          C                     Rust                  Go
-----------------  --------------------  --------------------  --------------------
Declare            int arr[5];           let arr: [i32; 5];    var arr [5]int
Initialize         int arr[5]={1,2,3,4,5}let arr=[1,2,3,4,5]; arr := [5]int{1,2,3,4,5}
Init to zero       int arr[5]={0};       let arr=[0i32; 5];    var arr [5]int  // auto
Init fill value    memset(arr,0,sizeof)  let arr=[99i32; 5];   // no shorthand
Array size         5  (manual) or        arr.len()             len(arr)
                   sizeof(arr)/sizeof(arr[0])
Index              arr[2]                arr[2]                arr[2]
Bounds check       NONE (UB on OOB)     PANIC on OOB          PANIC on OOB
Pass to function   int* (decays!)        &[i32; 5] or &[i32]  [5]int (copy) or *[5]int
Size in type?      NO (decays to ptr)    YES: [i32; 5] ≠ [i32;6]  YES: [5]int ≠ [6]int
```

```c
// C: array decays to pointer when passed to functions — SIZE IS LOST
void process(int *arr, size_t n) {   // must pass size separately
    for (size_t i = 0; i < n; i++) arr[i] *= 2;
}
int a[5] = {1, 2, 3, 4, 5};
process(a, 5);   // a decays to int* — no sizeof available inside process()
```

```rust
// Rust: [i32; 5] and [i32; 6] are different types
// Pass by reference to avoid copying:
fn process(arr: &mut [i32]) {        // slice ref — works for any length
    for x in arr.iter_mut() { *x *= 2; }
}
let mut a = [1, 2, 3, 4, 5];
process(&mut a);   // coerces [i32; 5] to &mut [i32]
```

```go
// Go: [5]int is a VALUE TYPE — passing copies the entire array
func process(arr [5]int) { }    // receives a COPY
func processPtr(arr *[5]int) { } // receives pointer, can mutate
// Idiomatic Go: use slices instead of arrays for most things
```

### 9.2 Dynamic arrays / slices

```
+---------------------------+---------------------------+---------------------------+
|            C              |            Rust           |            Go             |
+---------------------------+---------------------------+---------------------------+
| // No built-in dynamic    | // Vec<T> — owned,        | // Slice — []T            |
| // array. Use malloc:     | // growable array         |                           |
| int *arr = malloc(        | let mut v: Vec<i32> =     | s := make([]int, 5)       |
|     n * sizeof(int));     |     Vec::new();           | s := make([]int, 5, 10)   |
|                           | let mut v = vec![1,2,3];  | // len=5, cap=10          |
|                           |                           |                           |
| // Append (manual):       | v.push(4);                | s = append(s, 4)          |
| arr = realloc(arr,        | v.extend([5,6,7]);        | s = append(s, 5, 6, 7)   |
|   (n+1)*sizeof(int));     |                           | s = append(s, other...)   |
| arr[n++] = 4;             |                           |                           |
|                           | // Length and capacity:   | len(s), cap(s)            |
|                           | v.len(), v.capacity()     |                           |
|                           |                           |                           |
| // Index:                 | v[2]                      | s[2]                      |
| arr[2]                    |                           |                           |
|                           | // Slice of Vec:          | // Sub-slice:             |
| // Slice (manual):        | &v[1..3]  // [1,3)        | s[1:3]  // [1,3)          |
| int *sub = arr + 1;       | &v[1..=3] // [1,3]        | s[1:]   // from 1         |
| size_t sub_len = 3;       | &v[..]    // all          | s[:3]   // to 3           |
|                           |                           | s[:]    // all            |
|                           | // Iterate:               | // Iterate:               |
| for (int i=0; i<n; i++)   | for x in &v { }          | for i, v := range s { }  |
|     arr[i]                | for x in v.iter() { }    | for _, v := range s { }  |
|                           | for x in v.iter_mut(){ } |                           |
|                           |                           |                           |
| // Free:                  | // Freed automatically    | // GC handles it          |
| free(arr);                | // when v drops           |                           |
+---------------------------+---------------------------+---------------------------+
```

**Go slice internals — critical to understand:**
```go
// A slice is a struct: { ptr *T, len int, cap int }

a := []int{1, 2, 3, 4, 5}  // len=5, cap=5

b := a[1:3]                 // b is a SLICE OF a — shares backing array!
                            // b = {ptr=&a[1], len=2, cap=4}
b[0] = 99                   // modifies a[1] too!
fmt.Println(a)              // [1 99 3 4 5]

// To avoid sharing, copy:
b2 := make([]int, 2)
copy(b2, a[1:3])            // b2 is independent
```

---

## 10. Strings

Strings are handled very differently in each language.

```
+---------------------------+---------------------------+---------------------------+
|            C              |           Rust            |            Go             |
+---------------------------+---------------------------+---------------------------+
| char *s = "hello";        | let s: &str = "hello";    | s := "hello"             |
| // Null-terminated        | // UTF-8, not null-term'd | // UTF-8, immutable       |
| // Mutable or immutable   | // Immutable string slice | // Value type (copy-on-   |
| // depending on context   | // Points to .text seg    |  assign is shallow copy)  |
|                           |                           |                           |
| // Mutable heap string:   | // Owned, heap string:    | // No owned/borrowed      |
| char *s = strdup("hello");| let s: String =           | // distinction in Go:     |
| // or:                    |   String::from("hello");  | // string is always imm.  |
| char *s = malloc(6);      | let s = "hello".to_string | // use []byte to mutate   |
| strcpy(s, "hello");       |                           |                           |
|                           | // Append:                | // Concat:                |
| strcat(s, " world");      | s.push_str(" world");     | s = s + " world"          |
|                           | s.push('!');              | s += " world"             |
|                           |                           |                           |
| // Length:                | // Length (bytes):        | // Length (bytes):        |
| strlen(s)                 | s.len()                   | len(s)                    |
| // (O(n) — scans for NUL) | // (O(1) stored)          | // (O(1) stored)          |
|                           | // Length (chars):        | // Length (runes/chars):  |
|                           | s.chars().count() // O(n) | utf8.RuneCountInString(s) |
|                           |                           |                           |
| // Index byte:            | // Index byte:            | // Index byte:            |
| s[2]  // char             | s.as_bytes()[2]           | s[2]  // byte, NOT char   |
|                           | // Index char — NO DIRECT | // Iterate chars:         |
|                           | // INDEXING (UTF-8 var.)  | for _, r := range s {}   |
|                           | s.chars().nth(2) // O(n)  |                           |
|                           |                           |                           |
| // Compare:               | // Compare:               | // Compare:               |
| strcmp(a, b) == 0         | a == b                    | a == b                    |
|                           |                           |                           |
| // Format:                | // Format:                | // Format:                |
| sprintf(buf, "%d", n);    | let s = format!("{}", n); | s := fmt.Sprintf("%d", n) |
|                           |                           |                           |
| // Substring:             | // Substring (by bytes):  | // Substring (by bytes):  |
| strndup(s+2, 3)           | &s[2..5]                  | s[2:5]                    |
|                           | // Panics if not on char  | // Panics if not on UTF-8 |
|                           | // boundary               | // boundary               |
+---------------------------+---------------------------+---------------------------+
```

**Rust `&str` vs `String` — the most common confusion:**
```rust
// &str: borrowed reference to string data (read-only, thin+len fat pointer)
//   - string literals: always &str, stored in binary
//   - slices of String: &str pointing into String's heap buffer

// String: owned heap-allocated string (can grow and mutate)
//   - has ptr + len + cap (like Vec<u8>)

let literal: &str = "hello";              // &str — in .text segment
let owned: String = String::from("hello"); // String — on heap
let borrowed: &str = &owned;             // borrow String as &str — zero cost
let borrowed: &str = &owned[0..3];       // slice: "hel"

// Function accepting either:
fn print(s: &str) { println!("{}", s); }
print(literal);            // OK directly
print(&owned);             // &String coerces to &str automatically
print(&owned[0..3]);       // slice of String to &str
```

**Go string bytes vs runes:**
```go
s := "héllo"   // é is 2 bytes in UTF-8

fmt.Println(len(s))             // 6 — byte count, NOT character count!
fmt.Println([]byte(s))          // [104 195 169 108 108 111]
for i, r := range s {           // range decodes UTF-8 correctly:
    fmt.Printf("%d: %c\n", i, r)// 0:h  1:é  3:l  4:l  5:o
}                                // note: i jumps from 1 to 3 (é is 2 bytes)

// Modify a string? Convert to []byte:
b := []byte(s)
b[0] = 'H'
s2 := string(b)                 // new string
```

---

## 11. Enums and Sum Types

```c
// C enums: just named integers, no data attached
typedef enum {
    COLOR_RED,    // = 0
    COLOR_GREEN,  // = 1
    COLOR_BLUE,   // = 2
} Color;

Color c = COLOR_RED;
if (c == COLOR_RED) { ... }

// C: no sum types — simulate with tagged unions (verbose, error-prone)
typedef enum { SHAPE_CIRCLE, SHAPE_RECT } ShapeKind;
typedef struct {
    ShapeKind kind;
    union {
        struct { float radius; }    circle;
        struct { float w, h; }      rect;
    };
} Shape;

// Must check kind before accessing union — compiler won't enforce this!
Shape s;
s.kind = SHAPE_CIRCLE;
s.circle.radius = 5.0f;
```

```rust
// Rust enums: can carry data — proper algebraic data types
#[derive(Debug)]
enum Color {
    Red,
    Green,
    Blue,
    Custom(u8, u8, u8),     // tuple variant with data
    Named { r: u8, g: u8, b: u8 }, // struct variant with named fields
}

let c = Color::Red;
let c = Color::Custom(255, 128, 0);
let c = Color::Named { r: 255, g: 0, b: 0 };

// Compiler enforces exhaustive handling:
match c {
    Color::Red => println!("red"),
    Color::Green => println!("green"),
    Color::Blue => println!("blue"),
    Color::Custom(r, g, b) => println!("rgb({},{},{})", r, g, b),
    Color::Named { r, g, b } => println!("rgb({},{},{})", r, g, b),
}

// The canonical Maybe/Option:
enum Option<T> {     // from std lib, you use this constantly
    Some(T),
    None,
}

// The canonical Result:
enum Result<T, E> {  // from std lib
    Ok(T),
    Err(E),
}
```

```go
// Go: NO sum types / algebraic data types
// Idiomatic Go uses:
// 1. Constants for simple enums:
type Color int
const (
    Red Color = iota  // = 0
    Green             // = 1
    Blue              // = 2
)

// 2. Interfaces for "open" variants:
type Shape interface {
    Area() float64
}
type Circle struct { Radius float64 }
type Rect   struct { W, H float64 }

func (c Circle) Area() float64 { return math.Pi * c.Radius * c.Radius }
func (r Rect)   Area() float64 { return r.W * r.H }

// 3. Type switch for dispatch:
func describe(s Shape) {
    switch v := s.(type) {
    case Circle:
        fmt.Printf("circle radius %.1f\n", v.Radius)
    case Rect:
        fmt.Printf("rect %.1f×%.1f\n", v.W, v.H)
    }
}
```

---

## 12. Pattern Matching

```c
// C: no pattern matching — use if/else or switch on integers only
if (opt.has_value) {
    process(opt.value);
} else {
    handle_empty();
}
```

```rust
// Rust: match is the primary tool, deeply integrated with enums

// --- Match on enum variants ---
match opt {
    Some(v) => process(v),
    None    => handle_empty(),
}

// --- Destructuring in match ---
let point = (3, -5);
match point {
    (0, 0) => println!("origin"),
    (x, 0) => println!("on x-axis at {}", x),
    (0, y) => println!("on y-axis at {}", y),
    (x, y) => println!("({}, {})", x, y),
}

// --- Struct destructuring ---
let p = Point { x: 5, y: 10 };
match p {
    Point { x: 0, y } => println!("on y-axis at {}", y),
    Point { x, y: 0 } => println!("on x-axis at {}", x),
    Point { x, y }    => println!("({}, {})", x, y),
}

// --- Nested patterns ---
match msg {
    Message::Quit => quit(),
    Message::Move { x, y } => move_to(x, y),
    Message::Write(text) => println!("{}", text),
    Message::ChangeColor(Color::Red) => println!("to red"),
    Message::ChangeColor(Color::Custom(r, g, b)) => println!("{}{}{}", r, g, b),
}

// --- if let: match one pattern, ignore rest ---
if let Some(v) = opt {        // "if opt is Some(v), bind v"
    process(v);               // much cleaner than full match for one case
}

// --- while let ---
while let Some(top) = stack.pop() {
    println!("{}", top);
}

// --- let destructuring (not match but same patterns) ---
let (a, b, c) = (1, 2, 3);        // tuple destructuring
let Point { x, y } = point;       // struct destructuring
let [first, .., last] = array;    // slice pattern (nightly/advanced)

// --- Guards in match ---
match x {
    n if n % 2 == 0 => println!("even"),
    n if n < 0      => println!("negative odd"),
    _               => println!("positive odd"),
}

// --- @ bindings: capture and test simultaneously ---
match x {
    n @ 1..=10 => println!("in range: {}", n),  // binds n if in range
    _           => println!("out of range"),
}
```

```go
// Go: type switch + if/else — no algebraic pattern matching
switch v := value.(type) {
case int:
    fmt.Printf("int: %d\n", v)
case string:
    fmt.Printf("string: %s\n", v)
}

// Destructuring assignment (limited):
a, b := 1, 2        // multiple assignment
a, b = b, a         // swap

// Struct fields — no pattern matching:
p := Point{X: 5, Y: 10}
x := p.X            // must access fields individually
```

---

## 13. Traits and Interfaces

### 13.1 Defining and implementing

```rust
// Rust: TRAITS define shared behavior
// Define a trait:
trait Greet {
    fn hello(&self) -> String;

    // Default implementation — can override or use as-is
    fn greet_twice(&self) -> String {
        format!("{} {}", self.hello(), self.hello())
    }
}

// Implement for a specific type:
struct Person { name: String }

impl Greet for Person {
    fn hello(&self) -> String {
        format!("Hello, {}!", self.name)
    }
}

// Multiple traits for one type:
impl std::fmt::Display for Person {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "Person({})", self.name)
    }
}

// Using trait bounds (compile-time polymorphism):
fn greet_all<T: Greet>(items: &[T]) {     // T must implement Greet
    for item in items { println!("{}", item.hello()); }
}

// Trait objects (runtime polymorphism):
fn greet_dynamic(item: &dyn Greet) {      // &dyn = fat pointer (data + vtable)
    println!("{}", item.hello());
}
```

```go
// Go: INTERFACES define shared behavior
// Define an interface:
type Greeter interface {
    Hello() string
    // No default implementations
}

// Implement — NO explicit declaration needed (implicit/structural):
type Person struct { Name string }

func (p Person) Hello() string {    // Person implicitly satisfies Greeter
    return "Hello, " + p.Name + "!"
}

// If a type has all methods → it satisfies the interface automatically
// No "implements Greeter" anywhere

// Using the interface:
func greetAll(items []Greeter) {    // slice of interface values
    for _, item := range items { fmt.Println(item.Hello()) }
}

// Empty interface (accepts anything):
func printAny(v interface{}) { fmt.Println(v) }   // Go <1.18
func printAny(v any) { fmt.Println(v) }           // Go 1.18+, 'any' = interface{}
```

**Key contrast:**
```
Rust traits:      EXPLICIT — "impl Greet for Person" declares the intent
Go interfaces:    IMPLICIT — if you have the methods, you satisfy it (duck typing)

Rust:    Trait bounds checked at compile time — monomorphized (fast, no vtable)
         OR as trait objects with &dyn Trait (vtable at runtime, slower)

Go:      Interface dispatch always uses vtable (equivalent to &dyn Trait)
         No monomorphization for interfaces in Go (generics can do it in 1.18+)
```

---

## 14. Generics

```c
// C: NO generics — use macros or void* (both lose type safety)
// Macro approach:
#define MAX(a, b) ((a) > (b) ? (a) : (b))
// No type checking — MAX("hello", 42) compiles without error

// void* approach:
void* max_ptr(void *a, void *b, size_t size,
              int (*cmp)(const void*, const void*)) {
    return cmp(a, b) > 0 ? a : b;
}
// Must cast back — error-prone
```

```rust
// Rust: GENERICS — zero-cost abstraction (monomorphized at compile time)

// Generic function:
fn largest<T: PartialOrd>(list: &[T]) -> &T {
    //      ^^^^^^^^^^^^^^^^^^^^^^^^
    //      T must implement PartialOrd (can be compared)
    let mut l = &list[0];
    for item in list {
        if item > l { l = item; }
    }
    l
}

// Multiple bounds with + syntax:
fn print_largest<T: PartialOrd + std::fmt::Display>(list: &[T]) { }

// Where clause (cleaner for complex bounds):
fn process<T, U>(t: &T, u: &U) -> String
where
    T: std::fmt::Display + Clone,
    U: std::fmt::Debug,
{ format!("{:?}", u) }

// Generic struct:
struct Pair<T> {
    first:  T,
    second: T,
}

impl<T: std::fmt::Display + PartialOrd> Pair<T> {
    fn cmp_display(&self) {
        if self.first >= self.second { println!("{}", self.first); }
        else { println!("{}", self.second); }
    }
}
```

```go
// Go: GENERICS (added in Go 1.18)

// Generic function:
func Largest[T constraints.Ordered](list []T) T {
    //       ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
    //       T must satisfy constraints.Ordered
    l := list[0]
    for _, v := range list {
        if v > l { l = v }
    }
    return l
}

// Calling: type inferred from argument
biggest := Largest([]int{3, 1, 4, 1, 5})
biggest := Largest([]string{"go", "rust", "c"})

// Custom constraint:
type Number interface {
    int | int8 | int16 | int32 | int64 |
    float32 | float64
}

func Sum[T Number](nums []T) T {
    var total T
    for _, n := range nums { total += n }
    return total
}

// Generic struct (Go 1.18+):
type Stack[T any] struct {
    items []T
}
func (s *Stack[T]) Push(v T)  { s.items = append(s.items, v) }
func (s *Stack[T]) Pop() (T, bool) {
    if len(s.items) == 0 {
        var zero T
        return zero, false
    }
    n := len(s.items) - 1
    v := s.items[n]
    s.items = s.items[:n]
    return v, true
}
```

---

## 15. Closures and Function Types

```c
// C: NO closures — only function pointers (can't capture local variables)
int add(int a, int b) { return a + b; }

// Function pointer type: int (*fp)(int, int)
int (*fp)(int, int) = add;
int result = fp(3, 4);

// Simulate capture with a context struct (verbose):
typedef struct { int factor; } MultCtx;
int multiply(void *ctx, int x) {
    return ((MultCtx*)ctx)->factor * x;
}
MultCtx ctx = {3};
// call: multiply(&ctx, 5) = 15
// But this doesn't work with APIs expecting a plain function pointer
```

```rust
// Rust: CLOSURES — capture environment by ref, mut ref, or move

let x = 5;

// Immutable closure (captures x by shared reference):
let add_x = |n| n + x;     // type inferred
add_x(3)                   // returns 8; x still usable after

// Mutable closure:
let mut count = 0;
let mut increment = || { count += 1; count };
increment();  // 1
increment();  // 2

// Move closure (takes ownership of captured variables):
let s = String::from("hello");
let greeting = move || println!("{}", s);   // s is moved into closure
// println!("{}", s);  // ERROR: s was moved
greeting();

// Closure types:
// Fn     — can be called multiple times, doesn't mutate or consume captures
// FnMut  — can be called multiple times, may mutate captures
// FnOnce — can only be called once (consumes captures)

// Function that takes a closure:
fn apply<F: Fn(i32) -> i32>(f: F, x: i32) -> i32 { f(x) }

// Explicit type annotation for closure:
let double: fn(i32) -> i32 = |x| x * 2;    // fn pointer type (non-capturing)
let triple: Box<dyn Fn(i32) -> i32> = Box::new(|x| x * 3);  // trait object
```

```go
// Go: CLOSURES — functions are first-class, close over variables by reference

x := 5

// Function literal (closure):
addX := func(n int) int { return n + x }
addX(3)   // 8; x is captured by reference (not copied)

// Returns a closure (adder factory):
func makeAdder(base int) func(int) int {
    return func(n int) int {
        return base + n    // base captured by reference, lives on heap
    }
}
add10 := makeAdder(10)
add10(5)  // 15

// Function type:
type BinaryFunc func(int, int) int

var f BinaryFunc = func(a, b int) int { return a + b }
f(3, 4)  // 7

// Function as parameter:
func apply(f func(int) int, x int) int { return f(x) }
apply(func(n int) int { return n * 2 }, 5)  // 10

// Go captures by REFERENCE — watch out:
funcs := make([]func(), 5)
for i := 0; i < 5; i++ {
    i := i                     // shadow i: create new variable per iteration
    funcs[i] = func() { fmt.Println(i) }  // captures new i
}
// Without shadowing, all closures would print 5 (the final value of i)
```

---

## 16. Error Handling

### Philosophy comparison

```
C:       Return codes + errno + out-parameters. Easy to ignore. No enforcement.
Rust:    Result<T, E> and Option<T>. Compiler forces handling. ? operator.
Go:      Multiple return values with error as last return. Convention, not forced.
```

### C error handling

```c
#include <stdio.h>
#include <errno.h>
#include <string.h>

FILE* f = fopen("file.txt", "r");
if (f == NULL) {
    fprintf(stderr, "Error: %s\n", strerror(errno));
    return -1;
}

// Classic pattern — goto for cleanup:
int process_file(const char *path) {
    FILE *f = NULL;
    char *buf = NULL;
    int result = 0;

    f = fopen(path, "r");
    if (!f) { result = -1; goto cleanup; }

    buf = malloc(1024);
    if (!buf) { result = -1; goto cleanup; }

    // ... work ...

cleanup:
    if (buf) free(buf);
    if (f)   fclose(f);
    return result;
}
```

### Rust error handling

```rust
use std::fs::File;
use std::io::{self, Read};

// Option<T>: either Some(value) or None
fn find_first(v: &[i32], target: i32) -> Option<usize> {
    v.iter().position(|&x| x == target)
    // returns Some(index) or None
}

let idx = find_first(&[1, 2, 3], 2);
match idx {
    Some(i) => println!("found at {}", i),
    None    => println!("not found"),
}

// Common Option combinators:
idx.unwrap()            // panics if None
idx.unwrap_or(0)        // returns 0 if None
idx.unwrap_or_else(||0) // lazily computed default
idx.expect("must exist")// panics with custom message if None
idx.map(|i| i * 2)     // transform Some value
idx.and_then(|i| ...)  // chain Option-returning operations

// Result<T, E>: either Ok(value) or Err(error)
fn read_file(path: &str) -> Result<String, io::Error> {
    let mut f = File::open(path)?;    // ? = return Err early if Err
    let mut content = String::new();
    f.read_to_string(&mut content)?;  // ? propagates error up
    Ok(content)                       // wrap success in Ok
}

// The ? operator desugars to:
// match expr { Ok(v) => v, Err(e) => return Err(e.into()) }

// Handling Result:
match read_file("file.txt") {
    Ok(content) => println!("{}", content),
    Err(e)      => eprintln!("Error: {}", e),
}

read_file("file.txt").unwrap()           // panics on Err
read_file("file.txt").unwrap_or_default()
read_file("file.txt").unwrap_or_else(|_| String::new())
read_file("file.txt").map(|s| s.len())  // Ok(length) or Err
```

### Go error handling

```go
import (
    "errors"
    "fmt"
    "os"
)

// Functions return (value, error):
func readFile(path string) (string, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return "", fmt.Errorf("readFile: %w", err)   // wrap error with context
    }
    return string(data), nil
}

// Call and check:
content, err := readFile("file.txt")
if err != nil {
    log.Fatal(err)
}
fmt.Println(content)

// Creating errors:
var ErrNotFound = errors.New("not found")        // sentinel error
err := fmt.Errorf("user %d: %w", id, ErrNotFound)  // wrapping

// Checking errors:
errors.Is(err, ErrNotFound)    // checks wrapped chain
errors.As(err, &target)        // extracts specific type from chain

// Custom error type:
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}

// Panic/recover (for truly unexpected errors, like C's abort):
func safeDiv(a, b int) (result int, err error) {
    defer func() {                    // defer runs on function exit
        if r := recover(); r != nil { // recover catches panic
            err = fmt.Errorf("recovered: %v", r)
        }
    }()
    return a / b, nil   // panics if b == 0
}
```

---

## 17. Memory and Allocation Syntax

```
+---------------------+-------------------------+---------------------+
| C                   | Rust                    | Go                  |
+---------------------+-------------------------+---------------------+
| // Heap allocate    | // Heap (Box)           | // Heap (new)       |
| int *p =            | let b = Box::new(5);    | p := new(int)       |
|   malloc(sizeof(int))|                        | *p = 5              |
| *p = 5;             |                         |                     |
|                     | // Vec (growable)       | // Slice (growable) |
| int *v = malloc(    | let v = vec![1, 2, 3]; | v := []int{1, 2, 3}|
|   3 * sizeof(int)); | let mut v = Vec::new();| v := make([]int, 3)|
| v[0]=1; v[1]=2;     | v.push(1);              | v = append(v, 1)   |
|                     |                         |                     |
| // Free             | // Auto on scope exit   | // GC               |
| free(p);            | // (Drop trait)         | // (automatic)      |
|                     |                         |                     |
| // Resize           | // Vec::resize          | // append handles   |
| p = realloc(p, n);  | v.resize(10, 0);        | v = append(v, ...)  |
|                     |                         |                     |
| // Zero-init        | // vec![0; n]           | make([]int, n)      |
| calloc(n, sizeof T) | vec![0i32; 10]          | // (always zeroed)  |
|                     |                         |                     |
| // Stack alloc only | // let x: T on stack    | // let x: T on stack|
| int x = 5;          | let x: i32 = 5;         | x := 5             |
| char buf[128];      | let buf = [0u8; 128];   | var buf [128]byte  |
+---------------------+-------------------------+---------------------+
```

---

## 18. Concurrency

### Creating threads/goroutines

```c
// C: POSIX threads (pthreads)
#include <pthread.h>

void *worker(void *arg) {
    int *n = (int *)arg;
    printf("worker: %d\n", *n);
    return NULL;
}

pthread_t tid;
int arg = 42;
pthread_create(&tid, NULL, worker, &arg);
pthread_join(tid, NULL);  // wait for thread
```

```rust
// Rust: std::thread
use std::thread;

let handle = thread::spawn(|| {
    println!("worker");
    42  // return value
});

let result = handle.join().unwrap();  // blocks, unwrap panics if thread panicked
println!("result: {}", result);

// Sharing data between threads: Arc<Mutex<T>>
use std::sync::{Arc, Mutex};
let shared = Arc::new(Mutex::new(0));
let shared2 = Arc::clone(&shared);

thread::spawn(move || {
    let mut n = shared2.lock().unwrap();
    *n += 1;
});
```

```go
// Go: goroutines (lightweight, not OS threads)
go func() {
    fmt.Println("worker")
}()  // starts immediately, runs concurrently

// Channels for communication:
ch := make(chan int)              // unbuffered: sync
ch := make(chan int, 10)          // buffered: async up to cap

// Send and receive:
go func() { ch <- 42 }()         // send
v := <-ch                        // receive (blocks until value available)
v, ok := <-ch                    // receive with closed check

// Close:
close(ch)                        // sender closes; receivers get zero value + ok=false

// Select: wait on multiple channels
select {
case v := <-ch1:
    fmt.Println("from ch1:", v)
case v := <-ch2:
    fmt.Println("from ch2:", v)
case <-ctx.Done():                // context cancellation
    return
default:                          // non-blocking
    fmt.Println("nothing ready")
}

// sync package:
var mu sync.Mutex
mu.Lock()
// critical section
mu.Unlock()

var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    // work
}()
wg.Wait()  // block until all Done()
```

---

## 19. Modules, Packages, and Imports

```c
// C: #include (textual inclusion, not a module system)
#include <stdio.h>         // search system include paths
#include "mylib.h"         // search relative to current file
// Header files: declarations only
// .c files: definitions
// Link with: gcc main.c mylib.c -o program

// Header guard (prevent double inclusion):
#ifndef MYLIB_H
#define MYLIB_H
// ... declarations ...
#endif
```

```rust
// Rust: modules and crates

// Declare submodule in the same file:
mod math {
    pub fn add(a: i32, b: i32) -> i32 { a + b }   // pub = public
    fn secret() {}                                  // private (default)
}

// Use items:
use math::add;          // bring add into scope
use std::collections::HashMap;
use std::io::{self, Read, Write};   // multiple from same path

// Submodule in separate file: math.rs or math/mod.rs
mod math;  // tells rustc to look for math.rs or math/mod.rs

// External crates (dependencies in Cargo.toml):
use serde::{Serialize, Deserialize};

// Visibility:
pub fn public_fn() {}         // visible to all
pub(crate) fn crate_fn() {}   // visible within same crate
pub(super) fn parent_fn() {}  // visible to parent module
fn private_fn() {}            // visible only in current module

// Re-exporting:
pub use math::add;            // makes math::add accessible as this module's add
```

```go
// Go: packages (directory = package)
// Every .go file starts with: package packagename
// Package name is the last path element by convention

// Import:
import "fmt"                    // standard library
import "os"
import "github.com/user/pkg"    // external

// Multiple imports (idiomatic):
import (
    "fmt"
    "os"
    "strings"

    "github.com/user/pkg"       // blank line separates stdlib from third-party
)

// Aliasing:
import f "fmt"                  // use f.Println(...)
import . "fmt"                  // use Println(...) directly (rarely used)
import _ "github.com/lib/pq"   // blank import: run init() but don't use

// Visibility: ONLY first-letter determines export status
// Uppercase = exported (public): Println, HTTP, URL
// Lowercase = unexported (private): println, http, url

// Within a package, all files share the same namespace
// No explicit declaration needed within same package
```

---

## 20. Type Casting and Conversion

```
+---------------------------+---------------------------+---------------------------+
|            C              |            Rust           |             Go            |
+---------------------------+---------------------------+---------------------------+
| // Implicit conversions:  | // Almost NO implicit     | // NO implicit numeric    |
| int i = 3.14;    // ok   | // numeric conversions    | // conversions            |
| float f = 5;     // ok   |                           |                           |
| char c = 65;     // ok   | // Explicit with `as`:    | // Explicit with          |
|                           | let i: i32 = 3;           | // type conversion:       |
| // Explicit cast:         | let f = i as f64;         | var i int = 3             |
| int i = (int)3.14; // 3  | let b = f as u8;          | f := float64(i)           |
| float f = (float)5;       | // as truncates!          | b := uint8(f)             |
|                           | // 256u8 as u8 = 0 (wrap) | // wraps on overflow      |
|                           |                           |                           |
| // Pointer cast:          | // Raw pointer cast:      | // unsafe.Pointer:        |
| int *p = (int*)voidptr;   | // only in unsafe:        | p := (*int)(unsafe.       |
|                           | let p: *mut i32 =         |     Pointer(voidptr))     |
|                           |   rawptr as *mut i32;     |                           |
|                           |                           |                           |
| // Numeric truncation:    | // Safe conversion via    | // No safe conversions    |
| // silent, no error       | // From/Into traits:      | // — use explicit cast    |
|                           | let i: i32 = 5;           | // and check range:       |
|                           | let j: i64 = i64::from(i);| i32 := int32(i64val)      |
|                           | let j: i64 = i.into();    |                           |
|                           | // From/Into are lossless |                           |
|                           | // TryFrom/TryInto:       |                           |
|                           | let r: Result<u8,_> =     |                           |
|                           |   u8::try_from(256i32);   |                           |
|                           | // Err(overflow)          |                           |
+---------------------------+---------------------------+---------------------------+
```

**Rust `as` vs `From/Into/TryFrom`:**
```rust
// as: always succeeds, may lose data (truncate, wrap)
let x: i32 = 1000;
let y = x as u8;    // 232 (1000 mod 256) — silent truncation!

// From/Into: only defined for lossless conversions
let a: i32 = 5;
let b: i64 = i64::from(a);   // always succeeds, i64 can hold any i32
let b: i64 = a.into();       // same via Into (the other direction)

// TryFrom/TryInto: for conversions that might fail
use std::convert::TryFrom;
let big: i64 = 300;
match i8::try_from(big) {
    Ok(small) => println!("{}", small),
    Err(_)    => println!("overflow"),     // 300 doesn't fit in i8
}
```

---

## 21. Constants and Statics

```c
// C:
#define MAX_SIZE 100        // Preprocessor macro — not typed, just text substitution
const int MAX = 100;        // Typed constant — can be used in most but not all contexts
// Note: C's const variable IS NOT truly constant (can be modified via pointer!)
// In function scope: const int x = 5; — compiler discourages but doesn't always prevent
//   int *p = (int*)&x; *p = 10;  // UB but may "work"

// Enum as integer constants:
enum { MAX_ITEMS = 100, BUFFER_SIZE = 4096 };
```

```rust
// Rust:
const MAX_SIZE: usize = 100;         // truly constant, evaluated at compile time
                                     // MUST have type annotation
                                     // can be used in array sizes, match arms, etc.

static GLOBAL: i32 = 0;             // lives for the entire program duration
                                     // unique memory address (unlike const which is inlined)
static mut COUNTER: u32 = 0;        // mutable static — accessing requires unsafe!

// Constant expressions (computed at compile time):
const BUFFER_SIZE: usize = 1024 * 4;
const ARRAY: [i32; 3] = [1, 2, 3];

// Using const in array size:
let arr: [u8; MAX_SIZE] = [0; MAX_SIZE];  // fine — MAX_SIZE is a compile-time constant
```

```go
// Go:
const MaxSize = 100           // untyped constant — type inferred from context
const MaxSize int = 100       // typed constant

// Grouped:
const (
    StatusOK  = 200
    StatusErr = 500
)

// iota — auto-incrementing in const block:
type Direction int
const (
    North Direction = iota   // = 0
    East                     // = 1
    South                    // = 2
    West                     // = 3
)

// iota patterns:
const (
    _  = iota          // skip 0
    KB = 1 << (10 * iota)  // 1 << 10 = 1024
    MB                     // 1 << 20
    GB                     // 1 << 30
)

// Go does NOT have const structs, const slices, or const maps
// Only: bool, numeric, string, rune constants
```

---

## 22. Null / None / Nil

This is one of the most important conceptual differences.

```
+---------------------------+---------------------------+---------------------------+
|            C              |            Rust           |            Go             |
+---------------------------+---------------------------+---------------------------+
| NULL                       | No null at all in safe   | nil                       |
|                            | Rust. To express absence |                           |
| Applies to:                | use Option<T>:           | Applies to:               |
|   Any pointer type         |   None — no value        |   *T (pointers)           |
|                            |   Some(v) — has value v  |   []T (slices)            |
| NULL = (void*)0            |                          |   map[K]V (maps)          |
|                            | Option<&str>:            |   chan T (channels)       |
| char *s = NULL;            |   None                   |   interface{}             |
| int *p = NULL;             |   Some("hello")          |   func types              |
|                            |                          |                           |
| // Must check manually:    | // Compiler forces you   | // Must check manually:   |
| if (p != NULL) {           | // to handle None:       | if p != nil {             |
|     *p = 5;                | match opt {              |     *p = 5               |
| }                          |   Some(v) => use(v),     | }                         |
|                            |   None    => handle(),   |                           |
| Forgot to check →          | }                        | Forgot to check →         |
| CRASH / UB                 | // Or use if let Some:   | RUNTIME PANIC             |
|                            | if let Some(v) = opt {   |                           |
|                            |     use(v);              | // nil slice is usable:   |
|                            | }                        | var s []int               |
|                            |                          | len(s) == 0  // OK        |
|                            |                          | append(s, 1)  // OK       |
|                            |                          | s[0] // PANIC             |
+---------------------------+---------------------------+---------------------------+
```

```rust
// Rust Option<T> quick reference:
let some: Option<i32> = Some(42);
let none: Option<i32> = None;

some.is_some()          // true
some.is_none()          // false
some.unwrap()           // 42 (panics if None)
some.unwrap_or(0)       // 42 (returns 0 if None)
some.expect("msg")      // 42 (panics with "msg" if None)
some.map(|x| x * 2)    // Some(84)
some.and_then(f)        // chains Option-returning f
some.or(Some(99))       // Some(42) (uses fallback only if None)
none.or(Some(99))       // Some(99)
some.filter(|&x| x > 0)// Some(42)
some.as_ref()           // Option<&i32>
```

---

## 23. Operators

### Arithmetic and comparison

```
All three languages share: +  -  *  /  %  ==  !=  <  >  <=  >=

DIFFERENCES:
Operation       C           Rust        Go
--------------  ----------  ----------  ----------
Integer div     /           /           /
Float div       /           /           /
Modulo          %           %           %
Power           pow(x,2.0)  x.powi(2)   math.Pow(x,2)
                            or x*x      or x*x
Bitwise AND     &           &           &
Bitwise OR      |           |           |
Bitwise XOR     ^           ^           ^
Bitwise NOT     ~x          !x          ^x   (note: ^ not ~ in Go!)
Left shift      <<          <<          <<
Right shift     >>          >>          >>
Logical AND     &&          &&          &&
Logical OR      ||          ||          ||
Logical NOT     !           !           !

Increment/decrement:
  C:     x++  x--  ++x  --x   (pre and post)
  Rust:  NO ++ or -- operator  (use x += 1, x -= 1)
  Go:    x++  x--  (ONLY post-increment; x++ is a STATEMENT not expression)
         i = j++  // ERROR in Go (not an expression)
```

### Assignment operators

```
C and Go:    +=  -=  *=  /=  %=  &=  |=  ^=  <<=  >>=
Rust:        +=  -=  *=  /=  %=  &=  |=  ^=  <<=  >>=

Note: Rust's assignment is a STATEMENT (returns ())
C's assignment is an EXPRESSION (returns the assigned value):
  C:   if ((x = get_value()) != 0) { ... }  // common C idiom
  Rust: if let Some(x) = get_value() { ... }  // Rust equivalent
  Go:   x = get_value(); if x != 0 { ... }   // Go: no assignment in if condition
                                              //  (except init statement)
```

### Range operators (Rust only)

```rust
1..5    // exclusive range: 1, 2, 3, 4
1..=5   // inclusive range: 1, 2, 3, 4, 5
..5     // from start to 5 (exclusive) — only in slice indexing
5..     // from 5 to end — only in slice indexing
..      // full range — only in slice indexing

// In for loops:
for i in 0..10 { }   // i = 0,1,...,9
for i in 0..=10 { }  // i = 0,1,...,10

// In slice indexing:
&arr[2..5]   // elements 2, 3, 4
&arr[..3]    // elements 0, 1, 2
&arr[3..]    // elements from 3 to end
```

### Dereference operator

```c
// C: * to dereference, & to take address
int x = 5;
int *p = &x;
*p = 10;         // writes through pointer

// Arrow -> for struct fields through pointer:
struct Point { int x, y; };
struct Point pt = {1, 2};
struct Point *pp = &pt;
pp->x = 5;      // = (*pp).x = 5
```

```rust
// Rust: * to dereference (usually auto-deref)
let x = 5i32;
let p = &x;
let val = *p;    // explicit deref — usually not needed in Rust
                 // Rust auto-derefs in most contexts

// Deref coercion:
let s = String::from("hello");
let r: &str = &s;     // String auto-derefs to str when you take &
// No -> in Rust — always use . :
let b = Box::new(Point { x: 1, y: 2 });
b.x;   // auto-derefs Box → accesses field
```

```go
// Go: * to dereference, & to take address
x := 5
p := &x
*p = 10       // explicit dereference for assignment
val := *p     // explicit dereference for reading

// But for struct fields: auto-deref
type Point struct { X, Y int }
pt := Point{1, 2}
pp := &pt
pp.X = 5     // Go auto-derefs: (*pp).X = 5
```

---

## 24. Semicolons, Braces, and Whitespace Rules

```
+---------------------------+---------------------------+---------------------------+
|            C              |            Rust           |             Go            |
+---------------------------+---------------------------+---------------------------+
| SEMICOLONS                | SEMICOLONS                | SEMICOLONS                |
| Required after every      | Required after statements | OPTIONAL — Go inserts     |
| statement. MANDATORY.     | NOT after expressions     | them automatically.       |
| Missing = compile error.  | that are return values.   |                           |
|                           |                           | The rule:                 |
| int x = 5;                | let x = 5;        // stmt | If the last token of a   |
| foo();                    | let y = x + 1;    // stmt | line can end a statement, |
| return 0;                 | foo();            // stmt | Go inserts ;              |
|                           | if cond { }       // OK,  | (identifiers, ), ], },   |
| No semicolons needed:     |     // not a statement    | literals, break, continue,|
|   struct/union definition | x + 1  // expression,     | return, ++ , -- etc.)     |
|   function definition     |        // NO semicolon    |                           |
|   preprocessor directives |        // when returned   | CONSEQUENCE: opening {    |
|                           |                           | MUST be on SAME LINE as   |
|                           | RULE: expression position | the if/for/func:          |
|                           | (last in block, right of  |                           |
|                           | let, after =) → NO semi   | if x > 0              // BAD
|                           | statement position → semi | {                     // Go inserts ; before {
|                           |                           |                           |
|                           | // Block is an expression:| if x > 0 {            // GOOD
|                           | let v = if c { 1 } else   | }                         |
|                           |           { 2 };  // stmt |                           |
+---------------------------+---------------------------+---------------------------+
| BRACES                    | BRACES                    | BRACES                    |
| Optional for single-line  | ALWAYS required           | ALWAYS required           |
| if/for/while:             | (no exception):           | (no exception):           |
|                           |                           |                           |
| if (x > 0)                | if x > 0 {                | if x > 0 {                |
|     do_thing();  // OK    |     do_thing();           |     doThing()             |
|                           | }                         | }                         |
| if (x > 0)                |                           |                           |
| {                         | // Single-line OK with    | // All on one line OK:    |
|     do_thing();           | // block syntax:          | if x > 0 { doThing() }   |
| } // C allows { on        | if x > 0 { do_thing(); } | // but { still required  |
|   // any line             |                           |                           |
+---------------------------+---------------------------+---------------------------+
| WHITESPACE                | WHITESPACE                | WHITESPACE                |
| Insignificant (mostly)    | Insignificant             | Insignificant EXCEPT for  |
|                           |                           | the semicolon-insertion   |
| Except: preprocessor      |                           | rule (linebreaks matter)  |
| directives must be on     |                           |                           |
| their own line            |                           | gofmt enforces style      |
+---------------------------+---------------------------+---------------------------+
```

**The Go semicolon gotcha illustrated:**
```go
// WRONG — Go inserts ; before the {
if x > 0
{                        // parsed as: if x > 0; {  → syntax error
    fmt.Println("yes")
}

// CORRECT
if x > 0 {
    fmt.Println("yes")
}

// WRONG — return breaks here
return
    someValue            // parsed as: return; someValue — returns nothing!

// CORRECT
return someValue         // same line

// WRONG — function call broken across lines
result := someFunction(
    arg1,
    arg2
)                        // OK actually — ) can end statement, but put ) on new line is fine
// But never break like:
result := someFunction(
    arg1,
    arg2)                // this is also fine

// The , at end of each line in multi-line slices/maps is REQUIRED:
m := map[string]int{
    "a": 1,
    "b": 2,   // REQUIRED trailing comma (not optional!)
}
```

---

## 25. Macros and Code Generation

```c
// C: preprocessor macros (textual substitution, no type safety)
#define PI 3.14159
#define MAX(a, b) ((a) > (b) ? (a) : (b))   // parentheses everywhere — essential
#define SWAP(T, a, b) do { T tmp = a; a = b; b = tmp; } while(0)

// Dangers:
MAX(x++, y++)   // x and y may be incremented TWICE — double evaluation bug
int z = MAX(1, 2) * 3;   // without outer parens: (1) > (2) ? 1 : 2 * 3 → wrong

// Stringification and concatenation:
#define STRINGIFY(x) #x
#define CONCAT(a, b) a##b

// Conditional compilation:
#ifdef DEBUG
    printf("debug: x=%d\n", x);
#endif
```

```rust
// Rust: hygienic macros (operate on tokens, type-safe, no double-eval)

// Declarative macros (macro_rules!):
macro_rules! my_vec {
    ( $( $x:expr ),* ) => {    // pattern: comma-separated expressions
        {
            let mut v = Vec::new();
            $( v.push($x); )*  // repeat for each $x
            v
        }
    };
}
let v = my_vec![1, 2, 3];   // expands to: {let mut v=...; v.push(1); ...}

// Common built-in macros:
println!("{} {}", a, b)   // formatted print with newline
print!("{}", a)           // no newline
format!("{}", a)          // returns String
eprintln!("{}", a)        // to stderr
vec![1, 2, 3]             // create Vec
dbg!(expression)          // prints file:line: expression = value, returns value
assert!(condition)        // panics if false
assert_eq!(a, b)          // panics with useful message if a != b
todo!()                   // panics with "not yet implemented"
unimplemented!()          // same
panic!("message")         // explicit panic
include_str!("file.txt")  // includes file as &str at compile time

// Procedural macros (more powerful, derive macros):
#[derive(Debug, Clone, PartialEq, Serialize)]
struct Point { x: f64, y: f64 }
// #[derive] generates implementations automatically
```

```go
// Go: NO macros or code generation in the language itself
// go generate: run external tools (stringer, mockgen, protoc, etc.)

//go:generate stringer -type=Direction  // directive for go generate tool

// Build tags (conditional compilation):
//go:build linux && amd64
// +build linux,amd64  // old syntax (before Go 1.17)
package main

// Compile-time assertions (workaround with zero-size arrays):
var _ [1]struct{}[unsafe.Sizeof(int32(0)) == 4]  // compile error if int32 != 4

// The init() function (runs at package initialization):
func init() {
    // setup code — runs before main()
    // each file can have multiple init()
}
```

---

## 26. Comments

```
C:                          Rust:                        Go:
// Single line              // Single line                // Single line
/* Multi
   line */                  /* Multi
                               line */                   /* Multi
                                                            line */

// Documentation:           /// Doc comment for          // Doc comment (plain //):
/* no standard format */    /// the next item            // Package name provides...
                            //! Inner doc comment        //
                            //! (for the module itself)  // Usage:
                            ///                          //   Example code here.
                            /// # Examples
                            /// ```
                            /// let x = 5;
                            /// assert_eq!(add(x, 3), 8);
                            /// ```
                            /// Generates docs with `cargo doc`
                                                        // Generate with `go doc`
```

---

## 27. Complete Cheat Sheet

```
===================================================================================
  VARIABLE DECLARATION
===================================================================================
C:     int x = 5;              int x;  (uninitialized — DANGEROUS)
Rust:  let x = 5;              let x: i32;  let mut x = 5;
Go:    x := 5                  var x int  (zero-initialized — safe)

===================================================================================
  TYPE ANNOTATION POSITION
===================================================================================
C:     TYPE name           int x           void foo(int x)           int foo(void)
Rust:  name: TYPE          x: i32          fn foo(x: i32)            fn foo() -> i32
Go:    name TYPE           x int           func foo(x int)           func foo() int

===================================================================================
  FUNCTION DEFINITION
===================================================================================
C:     int add(int a, int b) { return a + b; }
Rust:  fn add(a: i32, b: i32) -> i32 { a + b }  // implicit return (no semicolon)
Go:    func add(a, b int) int { return a + b }

===================================================================================
  STRUCT
===================================================================================
C:     typedef struct { int x; int y; } Point;   Point p = {1, 2};
Rust:  struct Point { x: i32, y: i32 }           let p = Point { x: 1, y: 2 };
Go:    type Point struct { X, Y int }             p := Point{X: 1, Y: 2}

===================================================================================
  METHOD
===================================================================================
C:     void point_move(Point *p, int dx) { p->x += dx; }
Rust:  impl Point { fn move(&mut self, dx: i32) { self.x += dx; } }
Go:    func (p *Point) Move(dx int) { p.X += dx }

===================================================================================
  ARRAY / SLICE / VEC
===================================================================================
C:     int arr[5] = {1,2,3,4,5};     int *dyn = malloc(n*sizeof(int));
Rust:  let arr: [i32;5] = [1,2,3,4,5]; let v = vec![1,2,3];
Go:    arr := [5]int{1,2,3,4,5}        s := []int{1,2,3}

===================================================================================
  MAP / HASHMAP
===================================================================================
C:     (no built-in — use custom or library)
Rust:  let mut m = HashMap::new();  m.insert("key", 1);  m.get("key");
Go:    m := map[string]int{"key": 1}   m["key"] = 1   v, ok := m["key"]

===================================================================================
  POINTER TAKE / DEREF
===================================================================================
C:     int *p = &x;   *p = 5;   p->field  (struct via ptr)
Rust:  let p = &x;   *p        (auto-deref for methods)  raw: *const i32
Go:    p := &x        *p = 5    p.Field   (auto-deref for structs)

===================================================================================
  NULL / NONE / NIL
===================================================================================
C:     NULL   (any pointer — easy to forget to check)
Rust:  None   (Option<T> — compiler forces handling)
Go:    nil    (pointers, slices, maps, channels, interfaces)

===================================================================================
  ERROR HANDLING
===================================================================================
C:     int err = do_thing(); if (err != 0) { ... }    errno
Rust:  let v = do_thing()?;  // ? propagates Err upward
       match do_thing() { Ok(v) => ..., Err(e) => ... }
Go:    v, err := doThing(); if err != nil { return err }

===================================================================================
  IF
===================================================================================
C:     if (x > 0) { }             parens REQUIRED
Rust:  if x > 0 { }               no parens, braces REQUIRED
Go:    if x > 0 { }               no parens, braces REQUIRED
Go:    if v := f(); v > 0 { }    init statement before condition

===================================================================================
  FOR LOOP
===================================================================================
C:     for (int i = 0; i < n; i++) { }
Rust:  for i in 0..n { }
Go:    for i := 0; i < n; i++ { }

===================================================================================
  WHILE LOOP
===================================================================================
C:     while (cond) { }
Rust:  while cond { }
Go:    for cond { }         (Go has no `while` keyword)

===================================================================================
  INFINITE LOOP
===================================================================================
C:     while (1) { }    for (;;) { }
Rust:  loop { }
Go:    for { }

===================================================================================
  RANGE / FOREACH
===================================================================================
C:     for (int i=0; i < n; i++) { arr[i]; }
Rust:  for item in collection { }    for (i, v) in arr.iter().enumerate() { }
Go:    for i, v := range collection { }    for _, v := range collection { }

===================================================================================
  MATCH / SWITCH
===================================================================================
C:     switch (x) { case 1: ...; break; default: ...; }   (falls through!)
Rust:  match x { 1 => ..., 2|3 => ..., _ => ... }        (exhaustive, no fallthru)
Go:    switch x { case 1: ...  case 2,3: ...  default: ...}(no fallthru by default)

===================================================================================
  CLOSURES
===================================================================================
C:     (no closures — function pointers only)
Rust:  |x| x + 1             move |x| captured + x
Go:    func(x int) int { return x + 1 }

===================================================================================
  PRINT
===================================================================================
C:     printf("%d %s\n", n, s);    fprintf(stderr, "err: %s\n", msg);
Rust:  println!("{} {}", n, s);    eprintln!("err: {}", msg);
Go:    fmt.Println(n, s)           fmt.Fprintf(os.Stderr, "err: %s\n", msg)
                                   fmt.Printf("%d %s\n", n, s)

===================================================================================
  ALLOCATE / FREE
===================================================================================
C:     int *p = malloc(sizeof(int));   *p = 5;   free(p);   p = NULL;
Rust:  let b = Box::new(5);            // freed automatically when b drops
Go:    p := new(int);                  *p = 5;   // GC frees eventually

===================================================================================
  IMPORT / INCLUDE
===================================================================================
C:     #include <stdio.h>     #include "myfile.h"
Rust:  use std::fmt;          use std::collections::HashMap;
Go:    import "fmt"           import ( "fmt"   "os" )

===================================================================================
  TYPE CAST
===================================================================================
C:     (int)x     (float)n      (struct Point*)voidptr
Rust:  x as i32   n as f64      (unsafe: ptr as *const Point)
Go:    int(x)     float64(n)    (*Point)(unsafe.Pointer(voidptr))

===================================================================================
  CONSTANTS
===================================================================================
C:     #define MAX 100         const int MAX = 100;
Rust:  const MAX: usize = 100;  static DATA: &str = "hello";
Go:    const Max = 100          const Max int = 100

===================================================================================
  ZERO VALUES / DEFAULTS
===================================================================================
C:     (none — uninitialized is UNDEFINED)   static/global: zeroed by OS
Rust:  (none — must initialize)              Default trait: T::default()
Go:    ALL variables zero-initialized:  0  false  ""  nil  []  {}

===================================================================================
  STRINGS
===================================================================================
C:     char *s = "hello";     strlen(s)   strcat   strcmp   sprintf
Rust:  let s: &str = "hello"; s.len()     s + &t   s == t   format!("{}", x)
Go:    s := "hello"           len(s)      s + t    s == t   fmt.Sprintf("%v", x)

===================================================================================
  INTERFACE / TRAIT
===================================================================================
C:     (manual vtable with function pointers in struct)
Rust:  trait Foo { fn bar(&self); }    impl Foo for MyType { ... }
Go:    type Foo interface { Bar() }    // MyType satisfies Foo if it has Bar()
       // No explicit declaration — structural/implicit

===================================================================================
  GOROUTINE / THREAD
===================================================================================
C:     pthread_create(&tid, NULL, fn, arg);   pthread_join(tid, NULL);
Rust:  let h = thread::spawn(|| { });          h.join().unwrap();
Go:    go func() { }()                         // no explicit join unless WaitGroup/chan

===================================================================================
  SEMICOLONS
===================================================================================
C:     REQUIRED after every statement
Rust:  REQUIRED after statements; NOT after expressions-as-values
Go:    INSERTED automatically by compiler — opening { must be on same line
===================================================================================
```

---

*This guide is a companion to the Memory Management guide. Together they build the complete mental model needed to read, write, and switch between C, Rust, and Go without confusion.*
