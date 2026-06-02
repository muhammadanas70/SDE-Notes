# 🦀 The Complete Rust Mastery Guide
## Rules, Mental Models, Concepts, and Internals — Explained In Depth

> **Philosophy**: Rust is not a language you learn by writing code first.
> Rust is a language you learn by building a correct mental model first.
> Every "rule" the compiler enforces is not a restriction — it is a guarantee.
> When you internalize **why** the rule exists, the rule disappears and understanding remains.

---

# TABLE OF CONTENTS

```
PART I  — FOUNDATIONS
  01. Why Rust Exists — The Problem Space
  02. How Rust Thinks About Memory
  03. The Stack and the Heap — Rust's Memory Architecture
  04. Ownership — The Central Rule
  05. Move Semantics — Values in Motion
  06. Copy Types — When Moving Is Cheap Enough to Clone

PART II — BORROWING AND REFERENCES
  07. References — Borrowing Without Owning
  08. The Borrow Checker — Rust's Core Enforcement Engine
  09. Lifetimes — Naming How Long References Live
  10. Lifetime Elision — When Rust Infers Lifetimes
  11. Advanced Lifetimes — Subtyping, Variance, HRTB

PART III — THE TYPE SYSTEM
  12. Primitive Types — Rust's Atomic Units
  13. Compound Types — Tuples and Arrays
  14. Structs — Named Product Types
  15. Enums — Named Sum Types
  16. The Option Type — Null Safety Redefined
  17. The Result Type — Error as Value
  18. Pattern Matching — Destructuring Reality
  19. Type Inference — What Rust Knows Without Being Told

PART IV — TRAITS AND GENERICS
  20. Traits — Shared Behavior Interfaces
  21. Generics — Writing Code for All Types
  22. Trait Bounds — Constraining Generics
  23. Associated Types — Types Inside Traits
  24. Trait Objects — Dynamic Dispatch
  25. The Standard Library Traits — Essential Taxonomy
  26. Operator Overloading via Traits

PART V — FUNCTIONS, CLOSURES, ITERATORS
  27. Functions — Rust's Callable Units
  28. Closures — Functions That Capture Environment
  29. Fn / FnMut / FnOnce — The Closure Trait Hierarchy
  30. Iterators — The Lazy Sequence Protocol
  31. Iterator Adapters — Functional Data Pipelines
  32. Custom Iterators — Implementing Iterator

PART VI — ERROR HANDLING
  33. The Philosophy of Error Handling in Rust
  34. Option<T> in Depth
  35. Result<T, E> in Depth
  36. The ? Operator — Error Propagation
  37. Custom Error Types
  38. Error Libraries — thiserror and anyhow

PART VII — SMART POINTERS
  39. Box<T> — Heap Allocation with Ownership
  40. Rc<T> — Reference Counted Shared Ownership
  41. Arc<T> — Atomic Reference Counted (Thread-Safe)
  42. Cell<T> and RefCell<T> — Interior Mutability
  43. Mutex<T> and RwLock<T> — Thread-Safe Interior Mutability
  44. Cow<T> — Clone on Write
  45. Pin<T> — Pinning Values in Memory

PART VIII — CONCURRENCY
  46. The Fearless Concurrency Philosophy
  47. Threads — OS-Level Parallelism
  48. Message Passing — Channels
  49. Shared State — Arc + Mutex
  50. Send and Sync — Concurrency Safety Traits
  51. Atomic Types — Lock-Free Primitives
  52. Rayon — Data Parallelism (Ecosystem)

PART IX — ASYNC/AWAIT
  53. The Future Trait — Deferred Computation
  54. async/await Syntax
  55. Executors and Runtimes
  56. tokio — The Dominant Async Runtime
  57. Async Traits
  58. Pin and Unpin in Async Context
  59. Streams — Async Iterators

PART X — MODULES AND CRATES
  60. The Module System
  61. Visibility Rules — pub, pub(crate), pub(super)
  62. use Declarations and Paths
  63. Crates — Compilation Units
  64. Cargo — The Build System and Package Manager
  65. Workspaces — Multi-Crate Projects
  66. Features — Conditional Compilation

PART XI — MACROS
  67. Why Macros Exist
  68. Declarative Macros — macro_rules!
  69. Procedural Macros — derive, attribute, function-like
  70. Common Built-in Macros

PART XII — UNSAFE RUST
  71. What unsafe Unlocks
  72. Raw Pointers — *const T and *mut T
  73. Unsafe Functions and Blocks
  74. FFI — Calling C from Rust
  75. unsafe Traits
  76. The Rustonomicon Mental Model

PART XIII — ADVANCED TYPE SYSTEM
  77. Newtype Pattern
  78. Type Aliases
  79. PhantomData<T>
  80. Zero-Sized Types (ZSTs)
  81. Never Type (!)
  82. impl Trait — Opaque Return Types
  83. const Generics
  84. Higher-Kinded Types (workarounds)

PART XIV — MEMORY LAYOUT AND INTERNALS
  85. How Rust Lays Out Types in Memory
  86. repr Attributes
  87. Alignment and Padding
  88. Unions
  89. The Drop Trait — Deterministic Cleanup
  90. Drop Order — When Things Are Cleaned Up

PART XV — STRINGS AND COLLECTIONS
  91. String vs &str — The Two String Types
  92. Vec<T> — The Dynamic Array
  93. HashMap and BTreeMap
  94. HashSet and BTreeSet
  95. VecDeque, LinkedList, BinaryHeap
  96. Slices — Views into Sequences

PART XVI — MENTAL MODELS AND PATTERNS
  97. The Builder Pattern in Rust
  98. The State Machine Pattern
  99. The Typestate Pattern
  100. RAII — Resource Acquisition Is Initialization
  101. Common Rust Idioms
  102. Anti-Patterns to Avoid
```

---

# PART I — FOUNDATIONS

---

## 01. Why Rust Exists — The Problem Space

### The C/C++ Problem

Before Rust, systems programming had an unsolved tension:

```
Performance          Safety
    │                  │
    ▼                  ▼
C / C++          Java / Python
  (fast)           (safe)
    │                  │
    └──────┬───────────┘
           │
      CHOOSE ONE
      (Before Rust)
```

In C/C++:
- Memory is manually managed
- **Use-after-free**: You access memory after freeing it
- **Double free**: You free the same memory twice
- **Buffer overflow**: You write past the end of an allocation
- **Data races**: Two threads write to the same data simultaneously

These bugs are not theoretical. They are responsible for ~70% of CVEs (security vulnerabilities) in large C/C++ codebases (Microsoft, Google data).

### What Rust Promises

Rust makes a guarantee that is unprecedented in a systems language:

```
If your code compiles, it is free from:
  ✓ Use-after-free
  ✓ Dangling pointers
  ✓ Double frees
  ✓ Buffer overflows (in safe code)
  ✓ Data races (across threads)
  ✓ Null pointer dereferences
  ✓ Iterator invalidation
```

This is not runtime checking. There is no garbage collector. The compiler **proves** these properties at compile time using a system called the **borrow checker**.

### How Rust Achieves This — The Three Pillars

```
┌─────────────────────────────────────────────────────────────┐
│                    RUST'S THREE PILLARS                      │
├─────────────────┬─────────────────┬───────────────────────┤
│   OWNERSHIP     │   BORROWING     │    LIFETIMES            │
│                 │                 │                         │
│ Every value has │ You can lend    │ The compiler tracks     │
│ exactly ONE     │ references      │ how long every          │
│ owner at a      │ (shared or      │ reference is valid      │
│ time            │ exclusive)      │ and rejects invalid     │
│                 │                 │ ones                    │
│ When owner      │ Rules prevent   │                         │
│ goes away,      │ aliasing +      │ No runtime cost.        │
│ value is        │ mutation        │ All compile-time.       │
│ destroyed       │ simultaneously  │                         │
└─────────────────┴─────────────────┴───────────────────────┘
```

---

## 02. How Rust Thinks About Memory

Rust's type system encodes **ownership of memory** into the type itself. This is a fundamentally different model than GC languages.

### The Resource Lifecycle

```
┌──────────────────────────────────────────────────────────────┐
│                  VALUE LIFECYCLE IN RUST                      │
│                                                               │
│   CREATE ──► USE ──► [LEND] ──► [LEND BACK] ──► DESTROY     │
│     │                                                 │       │
│     │  Stack frame allocated or                       │       │
│     │  heap allocation (Box::new, Vec::new, etc.)     │       │
│     │                                                 │       │
│     │                               drop() called    │       │
│     │                               automatically    │       │
│     │                               when owner leaves│       │
│     │                               scope            │       │
│     └─────────────────────────────────────────────────┘       │
└──────────────────────────────────────────────────────────────┘
```

### The Fundamental Guarantee

In Rust, memory is **never freed while it is still accessible**. The compiler guarantees this by tracking:

1. Who **owns** every value (single owner at a time)
2. Who **borrows** a reference (tracked by the borrow checker)
3. How long **references remain valid** (lifetimes)

---

## 03. The Stack and the Heap — Rust's Memory Architecture

Understanding where data lives is critical to understanding Rust.

### Stack Memory

```
                    STACK (grows downward)
                    ┌────────────────────────────┐
  High address      │                            │
                    │  ┌──────────────────────┐  │
  main() frame      │  │  x: i32 = 5          │  │  ◄── 4 bytes, inline
                    │  │  y: bool = true       │  │  ◄── 1 byte, inline
                    │  │  arr: [i32; 3]        │  │  ◄── 12 bytes, inline
                    │  └──────────────────────┘  │
                    │  ┌──────────────────────┐  │
  foo() frame       │  │  a: i32 = 10         │  │  ◄── pushed when foo() called
                    │  │  b: f64 = 3.14       │  │
                    │  └──────────────────────┘  │  ◄── popped when foo() returns
                    │                            │
  Low address       └────────────────────────────┘

  Stack properties:
  ✓ Extremely fast allocation (just move stack pointer)
  ✓ LIFO order (Last In First Out)
  ✓ Size must be known at compile time
  ✓ Automatically cleaned up on scope exit
  ✗ Limited size (~8MB default on most OS)
  ✗ Cannot outlive the function that created it
```

### Heap Memory

```
                    HEAP (grows upward, fragmented)
                    ┌────────────────────────────────────────┐
  Low address       │                                        │
                    │  [used: Vec data]  [free]  [used: Box] │
                    │                                        │
                    │  Heap is a large pool managed by       │
                    │  the allocator (jemalloc, system, etc) │
                    │                                        │
  High address      └────────────────────────────────────────┘

  Heap properties:
  ✓ Large (limited only by system RAM)
  ✓ Size can be determined at runtime
  ✓ Data can outlive its creating function
  ✗ Slower to allocate (must find free block)
  ✗ Must be explicitly managed (Rust: automatic via ownership)
  ✗ Can fragment over time
```

### Stack vs Heap — Which Types Go Where

```
TYPE                     WHERE          WHY
─────────────────────────────────────────────────────────────
i32, u64, f64, bool      Stack          Fixed size, Copy type
char                     Stack          Fixed size (4 bytes)
[i32; 5] (array)         Stack          Fixed total size
(i32, bool) (tuple)      Stack          Fixed total size
&T (reference)           Stack          Just a pointer (8 bytes)
struct { x: i32, y: i32} Stack          Fixed size (if no heap inside)
─────────────────────────────────────────────────────────────
String                   Stack+Heap     Stack: (ptr, len, cap)
                                        Heap: actual character bytes
Vec<T>                   Stack+Heap     Stack: (ptr, len, cap)
                                        Heap: actual element storage
Box<T>                   Stack+Heap     Stack: pointer
                                        Heap: the T value
HashMap<K,V>             Stack+Heap     Complex internal structure
─────────────────────────────────────────────────────────────
```

### The String Example — A Concrete Memory Picture

```rust
let s = String::from("hello");
```

```
STACK                           HEAP
┌─────────────────────┐         ┌─────────────────┐
│ s: String           │         │                 │
│ ┌─────────────────┐ │         │  h e l l o      │
│ │ ptr: ────────────┼─┼────────►  (5 bytes)      │
│ │ len: 5          │ │         │                 │
│ │ cap: 5          │ │         └─────────────────┘
│ └─────────────────┘ │
└─────────────────────┘

ptr  = pointer to heap buffer
len  = how many bytes are currently used
cap  = total bytes allocated (capacity)
```

This three-word (ptr, len, cap) layout is the **fat pointer** pattern used by `String`, `Vec<T>`, and many other Rust types.

---

## 04. Ownership — The Central Rule

Ownership is the cornerstone of Rust's memory safety. It consists of **three rules** that are always enforced:

```
┌─────────────────────────────────────────────────────────────┐
│                  THE THREE OWNERSHIP RULES                   │
│                                                             │
│  RULE 1: Each value in Rust has an owner.                   │
│                                                             │
│  RULE 2: There can only be ONE owner at a time.             │
│                                                             │
│  RULE 3: When the owner goes out of scope,                  │
│           the value is dropped (freed).                     │
└─────────────────────────────────────────────────────────────┘
```

### Rule 1: Each Value Has an Owner

```rust
let x = 5;        // x owns the integer 5
let s = String::from("hello"); // s owns the String "hello"
```

The owner is the **variable binding** that holds the value.

### Rule 2: Only One Owner at a Time

This is where Rust diverges from every other language:

```rust
let s1 = String::from("hello");
let s2 = s1;  // ownership MOVES from s1 to s2

// s1 is now INVALID. The compiler knows this.
println!("{}", s1); // ERROR: use of moved value: `s1`
```

```
BEFORE MOVE:
STACK                           HEAP
┌─────────┐                     ┌─────────┐
│ s1: ptr─┼─────────────────────►  hello  │
│    len:5│                     └─────────┘
│    cap:5│
└─────────┘

AFTER let s2 = s1:
STACK                           HEAP
┌─────────┐                     ┌─────────┐
│ s1: ????│   (INVALIDATED)     │  hello  │
│         │                     └────┬────┘
├─────────┤                          │
│ s2: ptr─┼──────────────────────────┘
│    len:5│
│    cap:5│
└─────────┘

s1 is no longer valid. Only s2 has ownership.
Rust NEVER does a shallow copy that would create two owners.
```

**Why this rule exists**: If both `s1` and `s2` could be dropped, the heap memory would be freed twice → **double free** vulnerability. Rust prevents this at compile time.

### Rule 3: Drop When Owner Goes Out of Scope

```rust
{
    let s = String::from("hello"); // s comes into scope
    
    // s is valid here
    println!("{}", s);
    
} // scope ends → s is dropped → heap memory freed automatically
// s is no longer valid here
```

This is equivalent to C++'s RAII (Resource Acquisition Is Initialization), but enforced systematically.

### The drop() Function

Rust calls `drop()` (the `Drop` trait's method) automatically, but you can call it explicitly if you need to free something early:

```rust
let s = String::from("hello");
drop(s);     // explicitly drop — frees heap memory NOW
// s is invalid from here on
```

---

## 05. Move Semantics — Values in Motion

A **move** transfers ownership of a value from one binding to another. The original binding becomes invalid.

### What a Move Is NOT

A move is **not** a memory copy. No bytes are copied to a new location. What happens:
1. The value is "logically" transferred
2. The original binding is marked invalid by the compiler
3. The new binding is the sole owner

```
MOVE SEMANTICS VISUALIZATION:

let a = String::from("world");   // a owns value at heap addr 0x1000
let b = a;                        // b takes ownership, a invalidated

   a ──(DEAD)                    b ──► [heap: "world" @ 0x1000]

No data was copied. The OWNERSHIP was transferred.
```

### Where Moves Happen

Moves happen in these situations:

```rust
// 1. Assignment
let s1 = String::from("hello");
let s2 = s1;  // move

// 2. Passing to a function
fn takes_ownership(s: String) { /* s is moved here */ }
let s = String::from("hello");
takes_ownership(s);  // s is moved INTO the function
// s is invalid here

// 3. Returning from a function
fn gives_ownership() -> String {
    String::from("hello")  // returned value is moved OUT
}
let s = gives_ownership(); // s receives ownership

// 4. Collecting into a collection
let v: Vec<String> = vec!["a".to_string(), "b".to_string()];
let first = v.into_iter().next().unwrap(); // moves first element
```

### The Function Call Ownership Pattern

```
BEFORE calling process(s):                 AFTER calling process(s):
┌──────────────────┐                       ┌──────────────────┐
│ main()           │                       │ main()           │
│   s ──► "hello"  │                       │   s: DEAD        │
└──────────────────┘                       └──────────────────┘
         │                                          
         │ move on function call                    
         ▼                                          
┌──────────────────┐                       ┌──────────────────┐
│ process(s: String)│                      │ process returned  │
│   s ──► "hello"  │                       │   s dropped here │
└──────────────────┘                       └──────────────────┘
```

To get the value back, you either:
- Return it from the function
- Use a reference (borrowing — next section)

---

## 06. Copy Types — When Moving Is Cheap Enough to Clone

Some types implement the `Copy` trait. For these types, **assignment creates a copy** instead of a move. The original remains valid.

### Which Types Are Copy

```
COPY TYPES (implement Copy trait):
┌────────────────────────────────────────────────┐
│  All integer types:   i8, i16, i32, i64, i128  │
│                       u8, u16, u32, u64, u128  │
│                       isize, usize             │
│                                                │
│  Floating point:      f32, f64                 │
│  Boolean:             bool                     │
│  Character:           char                     │
│  Unit type:           ()                       │
│  Raw pointers:        *const T, *mut T         │
│  References:          &T (shared refs)         │
│                                                │
│  Arrays of Copy:      [i32; 5]                 │
│  Tuples of Copy:      (i32, bool, f64)         │
└────────────────────────────────────────────────┘

NOT COPY (heap-allocating types):
┌────────────────────────────────────────────────┐
│  String, Vec<T>, Box<T>, HashMap, ...          │
│  Any type containing a non-Copy type           │
└────────────────────────────────────────────────┘
```

### Copy vs Clone

```
COPY:  Automatic, implicit, bitwise copy
       No drop() called on original (it's still valid)
       Must be cheap — no heap allocation
       Annotated with: #[derive(Copy, Clone)]

CLONE: Explicit, call .clone() method
       Can do deep copy (allocates new heap memory)
       Can be expensive
       Annotated with: #[derive(Clone)]
```

```rust
// Copy example
let x: i32 = 5;
let y = x;   // x is COPIED (not moved)
println!("{} {}", x, y); // Both valid: x=5, y=5

// Clone example  
let s1 = String::from("hello");
let s2 = s1.clone(); // Deep copy — new heap allocation
println!("{} {}", s1, s2); // Both valid: independent copies
```

```
COPY (i32):                     CLONE (String):
                                
STACK                           STACK              HEAP
┌───────┐                       ┌──────┐           ┌──────┐
│ x = 5 │                       │  s1 ─┼──────────►│hello │
├───────┤                       ├──────┤           └──────┘
│ y = 5 │  ◄── independent copy │  s2 ─┼──────────►│hello │
└───────┘                       └──────┘           └──────┘
  Both valid                    Two separate heap buffers
```

### Making Your Types Copy

```rust
#[derive(Copy, Clone, Debug)]  // Copy requires Clone
struct Point {
    x: f64,  // f64 is Copy
    y: f64,  // f64 is Copy
}
// Point is now Copy — all fields must be Copy

#[derive(Clone, Debug)]
struct Person {
    name: String,  // String is NOT Copy
    age: u32,
}
// Person cannot be Copy because String is not Copy
```

---

# PART II — BORROWING AND REFERENCES

---

## 07. References — Borrowing Without Owning

A **reference** lets you refer to a value without taking ownership of it. References allow you to use a value without consuming it.

### Shared References (&T)

```rust
let s = String::from("hello");
let r = &s;   // r is a reference to s; s still owns the data

println!("{}", r);  // use through reference
println!("{}", s);  // s is still valid — ownership not transferred
```

```
STACK                           HEAP
┌──────────────────┐            ┌─────────┐
│ s: String        │            │  hello  │
│   ptr ───────────┼────────────►         │
│   len: 5         │            └─────────┘
│   cap: 5         │                 ▲
├──────────────────┤                 │
│ r: &String       │                 │
│   ptr ───────────┼─────────────────┘
└──────────────────┘
  (r points to s, which points to heap)
```

### Mutable References (&mut T)

```rust
let mut s = String::from("hello");
let r = &mut s;  // mutable reference
r.push_str(", world");  // can modify through mutable reference
println!("{}", s);  // "hello, world"
```

### The Borrowing Rules — Core Constraints

```
┌─────────────────────────────────────────────────────────────┐
│                    THE BORROWING RULES                       │
│                                                             │
│  RULE 1: At any given time, you can have EITHER:            │
│          • ONE mutable reference (&mut T)                   │
│          • OR ANY NUMBER of shared references (&T)          │
│          NEVER BOTH at the same time                        │
│                                                             │
│  RULE 2: References must ALWAYS be valid.                   │
│          A reference cannot outlive what it points to.      │
└─────────────────────────────────────────────────────────────┘
```

### Why These Rules? — The Aliasing + Mutation Problem

The rules prevent **aliasing combined with mutation**, which is the root cause of most memory bugs:

```
SCENARIO: Two mutable references to same data

let mut v = vec![1, 2, 3];
let first = &v[0];   // reference to first element
v.push(4);           // may reallocate vec! first is now dangling!
println!("{}", first); // USE AFTER FREE — UNDEFINED BEHAVIOR

Rust REFUSES to compile this.
```

```
SHARED REFERENCES (multiple readers):

let v = vec![1, 2, 3];
let r1 = &v;
let r2 = &v;    // OK — multiple shared refs allowed
let r3 = &v;    // OK — no mutation, no problem
// r1, r2, r3 can all coexist safely

MUTABLE REFERENCE (exclusive writer):

let mut v = vec![1, 2, 3];
let r1 = &mut v;
// let r2 = &mut v;  // ERROR: cannot borrow as mutable twice
// let r3 = &v;      // ERROR: cannot borrow as immutable while
                     // mutably borrowed
r1.push(4);  // Only r1 can access v here
```

### Non-Lexical Lifetimes (NLL)

Modern Rust (since 2018 edition) uses **Non-Lexical Lifetimes** — the borrow checker understands that references end when they are **last used**, not when they go out of syntactic scope:

```rust
let mut s = String::from("hello");

let r1 = &s;          // shared borrow starts
let r2 = &s;          // another shared borrow
println!("{} {}", r1, r2);  // r1 and r2 are LAST USED here
                             // their borrows END here (NLL)

let r3 = &mut s;      // OK! r1 and r2 are no longer active
r3.push_str(" world");
println!("{}", r3);
```

---

## 08. The Borrow Checker — Rust's Core Enforcement Engine

The borrow checker is a compile-time analysis that tracks:
1. The **lifetime** of every reference
2. **Who has what kind of access** at every point in the program
3. Whether any access **violates the rules**

### Mental Model of the Borrow Checker

Think of the borrow checker as tracking a **loan ledger**:

```
LOAN LEDGER for variable `v`:

Time  │ Operation           │ Ledger State
──────┼─────────────────────┼──────────────────────────────────
t=1   │ let mut v = vec![]  │ v: owned, mutable
t=2   │ let r1 = &v         │ v: shared-borrowed (by r1)
t=3   │ let r2 = &v         │ v: shared-borrowed (by r1, r2)
t=4   │ println!(r1, r2)    │ r1, r2 last used → borrows end
t=5   │ let r3 = &mut v     │ v: mut-borrowed (by r3)
t=6   │ r3.push(4)          │ v: mut-borrowed (by r3)
t=7   │ println!(r3)        │ r3 last used → borrow ends
t=8   │ v.push(5)           │ v: owned again, mutable
```

### Common Borrow Checker Errors and What They Mean

```rust
// ERROR 1: Use of moved value
let s = String::from("hello");
let s2 = s;
println!("{}", s); // error[E0382]: use of moved value: `s`
// FIX: Use &s to borrow, or .clone() to copy

// ERROR 2: Cannot borrow as mutable more than once
let mut s = String::from("hello");
let r1 = &mut s;
let r2 = &mut s; // error[E0499]: cannot borrow `s` as mutable
                 // more than once at a time
// FIX: Use r1 before creating r2, or use a scope

// ERROR 3: Cannot borrow as mutable because also borrowed as immutable
let mut s = String::from("hello");
let r1 = &s;
let r2 = &mut s; // error[E0502]: cannot borrow `s` as mutable
                 // because it is also borrowed as immutable
// FIX: Use r1 before creating r2 (NLL handles this in many cases)

// ERROR 4: Dangling reference
fn dangle() -> &String {  // error[E0106]: missing lifetime specifier
    let s = String::from("hello");
    &s  // ERROR: s is dropped at end of function, reference invalid
}
// FIX: Return String (owned), not &String (reference)
```

---

## 09. Lifetimes — Naming How Long References Live

Lifetimes are Rust's way of tracking **how long references remain valid**. Every reference has a lifetime, but most are inferred by the compiler.

### The Core Problem Lifetimes Solve

```
DANGLING REFERENCE (what lifetimes prevent):

{
    let r;
    {
        let x = 5;
        r = &x;   // r refers to x
    }             // x is DROPPED here
    println!("{}", r); // r is DANGLING — x is gone!
}

The borrow checker catches this: the lifetime of r ('a)
must be ≤ the lifetime of x ('b), but 'a > 'b here.
```

### Lifetime Annotation Syntax

```rust
// 'a is a lifetime parameter (like a generic, but for time)
fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {
    if x.len() > y.len() { x } else { y }
}
```

The annotation `'a` does **not** change how long references live. It is a **constraint** that tells the compiler: "the returned reference will be valid for at least as long as both `x` and `y`."

```
LIFETIME VISUALIZATION:

string1 = String::from("long string");
let result;
{
    let string2 = String::from("xyz");
    result = longest(string1.as_str(), string2.as_str());
    println!("{}", result);  // OK here
}
// println!("{}", result); // Would ERROR: string2 dropped above
```

```
Time ──────────────────────────────────────────────────────►
       string1 alive: ████████████████████████████████████
       string2 alive: ████████████████
       result valid:  ████████████████  ← limited to shorter
                                         of the two inputs
```

### When Lifetime Annotations Are Required

```rust
// When a function returns a reference derived from its inputs:
fn first_word<'a>(s: &'a str) -> &'a str { ... }

// When a struct holds references:
struct Important<'a> {
    part: &'a str,  // 'a: the struct can't outlive this string
}

// When a method on a struct with references gets complex:
impl<'a> Important<'a> {
    fn level(&self) -> i32 { 3 }
    fn announce(&self, announcement: &str) -> &str {
        self.part
    }
}
```

---

## 10. Lifetime Elision — When Rust Infers Lifetimes

Most lifetime annotations are **inferred** by the compiler using **lifetime elision rules**. These are deterministic rules, not magic.

### The Three Elision Rules

```
The compiler applies these rules in order:

RULE 1: Each reference parameter gets its own lifetime parameter.
        fn foo(x: &str, y: &str) 
           becomes:
        fn foo<'a, 'b>(x: &'a str, y: &'b str)

RULE 2: If there is exactly ONE input lifetime parameter,
        that lifetime is assigned to all output lifetime parameters.
        fn foo<'a>(x: &'a str) -> &str
           becomes:
        fn foo<'a>(x: &'a str) -> &'a str

RULE 3: If there are multiple input lifetime parameters but one is
        &self or &mut self, the lifetime of self is assigned to
        all output lifetime parameters.
        fn foo<'a, 'b>(&'a self, x: &'b str) -> &str
           becomes:
        fn foo<'a, 'b>(&'a self, x: &'b str) -> &'a str
```

```rust
// These are IDENTICAL — the second is what the first expands to:
fn first_word(s: &str) -> &str { ... }          // elided
fn first_word<'a>(s: &'a str) -> &'a str { ... } // explicit

// This CANNOT be elided — ambiguous which input lifetime to use:
fn longest(x: &str, y: &str) -> &str { ... }    // ERROR
fn longest<'a>(x: &'a str, y: &'a str) -> &'a str { ... } // OK
```

---

## 11. Advanced Lifetimes — Subtyping, Variance, HRTB

### Lifetime Subtyping

`'a: 'b` means "'a **outlives** 'b" — 'a is a subtype of 'b in the lifetime lattice.

```rust
// 'a: 'b means 'a lives at least as long as 'b
fn longer<'a: 'b, 'b>(s1: &'a str, s2: &'b str) -> &'b str {
    if s1.len() > s2.len() { s1 } else { s2 }
}
```

### The `'static` Lifetime

`'static` is the longest possible lifetime — for the entire duration of the program.

```rust
// String literals are 'static — baked into binary
let s: &'static str = "I live forever";

// Static variables are 'static
static GREETING: &str = "Hello, world!";

// Trait objects often require 'static:
fn takes_closure(f: impl Fn() + 'static) { ... }
// Because the closure might be stored and called later
```

### Higher-Ranked Trait Bounds (HRTB)

For closures that work with references of **any** lifetime:

```rust
// This means: for ALL lifetimes 'a, F implements Fn(&'a str)
fn apply<F>(f: F) where F: for<'a> Fn(&'a str) {
    f("hello");
    f("world");
}

// Shorthand (common pattern):
fn apply<F: Fn(&str)>(f: F) {  // compiler infers the HRTB
    f("hello");
}
```

### Variance

Variance describes how lifetimes relate when types are composed:

```
COVARIANT:     &'a T is covariant in 'a
               Longer lifetimes can be used where shorter expected
               &'long can substitute for &'short

CONTRAVARIANT: fn(T) is contravariant in T
               fn(&'short) can substitute for fn(&'long)

INVARIANT:     &mut T is invariant in T
               &mut 'long cannot substitute for &mut 'short or vice versa
               (prevents aliasing bugs)
```

---

# PART III — THE TYPE SYSTEM

---

## 12. Primitive Types — Rust's Atomic Units

### Integer Types

```
SIGNED INTEGERS (can be negative):
┌──────────┬────────┬──────────────────────────────────┐
│  Type    │  Bits  │  Range                           │
├──────────┼────────┼──────────────────────────────────┤
│  i8      │    8   │  -128 to 127                     │
│  i16     │   16   │  -32,768 to 32,767               │
│  i32     │   32   │  -2,147,483,648 to 2,147,483,647 │
│  i64     │   64   │  -9.2e18 to 9.2e18               │
│  i128    │  128   │  -1.7e38 to 1.7e38               │
│  isize   │ arch   │  pointer-sized signed (64-bit OS: 64 bits) │
└──────────┴────────┴──────────────────────────────────┘

UNSIGNED INTEGERS (non-negative only):
┌──────────┬────────┬──────────────────────────────────┐
│  Type    │  Bits  │  Range                           │
├──────────┼────────┼──────────────────────────────────┤
│  u8      │    8   │  0 to 255                        │
│  u16     │   16   │  0 to 65,535                     │
│  u32     │   32   │  0 to 4,294,967,295              │
│  u64     │   64   │  0 to 18.4e18                    │
│  u128    │  128   │  0 to 3.4e38                     │
│  usize   │ arch   │  pointer-sized unsigned (used for indexing) │
└──────────┴────────┴──────────────────────────────────┘

DEFAULT: i32 for integer literals (fastest on modern hardware)
```

### Integer Literals

```rust
let decimal     = 98_222;     // decimal with visual separator
let hex         = 0xff;       // hex prefix
let octal       = 0o77;       // octal prefix
let binary      = 0b1111_0000; // binary prefix
let byte        = b'A';       // u8 byte literal

// Type suffix
let x: i32 = 42;
let y = 42i64;   // type annotated in literal
let z = 42_u32;  // also valid
```

### Integer Overflow

```rust
// In DEBUG builds: overflow panics at runtime
// In RELEASE builds: overflow wraps (two's complement)

// Explicit overflow handling:
let x: u8 = 255;
let (result, overflowed) = x.overflowing_add(1); // (0, true)
let result = x.wrapping_add(1);     // 0 (wraps)
let result = x.saturating_add(1);   // 255 (saturates at max)
let result = x.checked_add(1);      // None (overflow detected)
```

### Float Types

```rust
let f32_val: f32 = 3.14;  // 32-bit, ~7 decimal digits precision
let f64_val: f64 = 3.14;  // 64-bit, ~15 decimal digits (DEFAULT)

// Special values:
let inf = f64::INFINITY;
let neg_inf = f64::NEG_INFINITY;
let nan = f64::NAN;
nan == nan  // FALSE — NaN is never equal to itself
```

### Boolean and Char

```rust
let t: bool = true;
let f: bool = false;

let c: char = 'z';       // char is 4 bytes (Unicode scalar value)
let heart = '❤';         // any Unicode character
let emoji = '😀';        // emoji too

// char range: U+0000 to U+D7FF and U+E000 to U+10FFFF
```

### The Unit Type `()`

```rust
// () is a zero-sized type representing "no value"
// Functions with no return value implicitly return ()
fn do_nothing() { }          // returns ()
fn do_nothing() -> () { }    // explicit, same thing

// Used in Option<()>, Result<(), Error>, etc.
let nothing: () = ();
```

---

## 13. Compound Types — Tuples and Arrays

### Tuples

```rust
// Fixed-length, ordered collection of DIFFERENT types
let t: (i32, f64, bool) = (500, 6.4, true);

// Destructuring
let (a, b, c) = t;

// Index access
let first = t.0;  // 500 (i32)
let second = t.1; // 6.4 (f64)
let third = t.2;  // true (bool)

// Single-element tuple (not a parenthesized expression)
let single = (5,);  // tuple
let not_tuple = (5); // just the integer 5
```

### Arrays

```rust
// Fixed-length, same type, stack-allocated
let a: [i32; 5] = [1, 2, 3, 4, 5]; // [type; size]

// Initialize all elements to same value
let b = [3; 5]; // [3, 3, 3, 3, 3]

// Indexing
let first = a[0]; // bounds checked at runtime in debug
let last = a[4];

// Out-of-bounds panics:
// let oops = a[10]; // runtime panic: index out of bounds
```

```
MEMORY LAYOUT:
let a: [i32; 5] = [1, 2, 3, 4, 5];

┌───────┬───────┬───────┬───────┬───────┐
│   1   │   2   │   3   │   4   │   5   │
│ i32   │ i32   │ i32   │ i32   │ i32   │
│ a[0]  │ a[1]  │ a[2]  │ a[3]  │ a[4]  │
└───────┴───────┴───────┴───────┴───────┘
 4 bytes each = 20 bytes total, contiguous in memory
```

---

## 14. Structs — Named Product Types

Structs are the primary way to group related data. They are **product types** — the set of all possible structs is the product of all possible values for each field.

### Named Field Structs

```rust
struct User {
    username: String,
    email: String,
    sign_in_count: u64,
    active: bool,
}

let user = User {
    email: String::from("user@example.com"),
    username: String::from("user123"),
    active: true,
    sign_in_count: 1,
};

// Field access
println!("{}", user.email);

// Struct update syntax (spread from another struct)
let user2 = User {
    email: String::from("new@example.com"),
    ..user  // rest of fields from user (note: moves user!)
};
```

### Tuple Structs

```rust
// Named tuples — useful for newtype pattern
struct Color(i32, i32, i32);
struct Point(f64, f64, f64);

let black = Color(0, 0, 0);
let origin = Point(0.0, 0.0, 0.0);

let r = black.0;  // field access by index
```

### Unit Structs

```rust
// Zero-sized struct — no data, but can implement traits
struct AlwaysEqual;
struct Marker;

let subject = AlwaysEqual;
// Useful as marker types, for implementing traits without data
```

### Methods on Structs

```rust
struct Rectangle {
    width: f64,
    height: f64,
}

impl Rectangle {
    // Associated function (no self) — called as Rectangle::new(...)
    fn new(width: f64, height: f64) -> Self {
        Rectangle { width, height }  // shorthand when field name == variable name
    }

    // Method with immutable self reference
    fn area(&self) -> f64 {
        self.width * self.height
    }

    // Method with mutable self reference
    fn scale(&mut self, factor: f64) {
        self.width *= factor;
        self.height *= factor;
    }

    // Method that consumes self (takes ownership)
    fn into_square(self) -> Rectangle {
        let side = self.width.min(self.height);
        Rectangle { width: side, height: side }
    }
}

let mut rect = Rectangle::new(10.0, 5.0);
let area = rect.area();        // &self
rect.scale(2.0);              // &mut self
let square = rect.into_square(); // self — rect is moved/consumed
```

### Memory Layout of Structs

```
struct Point { x: f64, y: f64, z: f64 }

Memory layout (64-bit):
┌──────────────┬──────────────┬──────────────┐
│  x: f64      │  y: f64      │  z: f64      │
│  (8 bytes)   │  (8 bytes)   │  (8 bytes)   │
└──────────────┴──────────────┴──────────────┘
 Total: 24 bytes, alignment: 8 bytes

struct Mixed { a: u8, b: u32, c: u8 }

Without repr(C) — Rust may reorder/pad:
┌──────┬───────────┬──────┬─────────────┬──────┐
│ b:u32│  padding  │ a:u8 │  padding    │ c:u8 │
│ 4 b  │           │ 1 b  │             │ 1 b  │  ← compiler reorders
└──────┴───────────┴──────┴─────────────┴──────┘
```

---

## 15. Enums — Named Sum Types

Enums are **sum types** — a value is exactly ONE of the listed variants. This is fundamentally different from C enums.

### Basic Enums

```rust
enum Direction {
    North,
    South,
    East,
    West,
}

let dir = Direction::North;
```

### Enums with Data — Algebraic Data Types

Rust enums can carry data in each variant, making them **algebraic data types**:

```rust
enum Message {
    Quit,                        // no data
    Move { x: i32, y: i32 },    // named fields (like a struct)
    Write(String),               // tuple variant
    ChangeColor(i32, i32, i32),  // multiple values
}

let m1 = Message::Quit;
let m2 = Message::Move { x: 10, y: 20 };
let m3 = Message::Write(String::from("hello"));
let m4 = Message::ChangeColor(255, 0, 128);
```

```
MEMORY LAYOUT OF ENUM:

enum Message {
    Quit,                     // 0 bytes of data
    Move { x: i32, y: i32 }, // 8 bytes of data
    Write(String),            // 24 bytes (ptr+len+cap)
    ChangeColor(i32,i32,i32), // 12 bytes
}

Rust allocates: [discriminant (1+ bytes)] + [max(variant data sizes)]

┌─────────────┬─────────────────────────────────────────┐
│discriminant │          variant data (max size)         │
│  (tag)      │                                         │
└─────────────┴─────────────────────────────────────────┘
              ▲── size of largest variant (Write: 24 bytes)

Total size: 1 (discriminant) + 24 (String) + padding = 32 bytes
```

### Methods on Enums

```rust
impl Message {
    fn call(&self) {
        match self {
            Message::Quit => println!("Quit"),
            Message::Move { x, y } => println!("Move to ({}, {})", x, y),
            Message::Write(text) => println!("Text: {}", text),
            Message::ChangeColor(r, g, b) => println!("Color: {} {} {}", r, g, b),
        }
    }
}
```

---

## 16. The Option Type — Null Safety Redefined

`Option<T>` is Rust's solution to the billion-dollar mistake (null pointers).

```rust
enum Option<T> {
    Some(T),  // contains a value of type T
    None,     // represents absence of a value
}
```

### Why Option is Better Than Null

In languages with null, any reference might be null — you never know unless you check, and forgetting to check causes crashes.

In Rust, if a function returns `Option<String>`, the **type system forces you to handle the None case**. You cannot accidentally use a None as if it were Some.

```rust
// Returns None if divisor is 0 — impossible to ignore!
fn divide(a: f64, b: f64) -> Option<f64> {
    if b == 0.0 { None } else { Some(a / b) }
}

let result = divide(10.0, 2.0);

// Must handle both cases:
match result {
    Some(value) => println!("Result: {}", value),
    None => println!("Cannot divide by zero"),
}

// Or use if let:
if let Some(value) = result {
    println!("Result: {}", value);
}

// Chaining with ?  in Option-returning functions:
fn calc(a: f64, b: f64, c: f64) -> Option<f64> {
    let step1 = divide(a, b)?;  // returns None if divide fails
    let step2 = divide(step1, c)?;
    Some(step2)
}
```

### Option Methods

```rust
let some: Option<i32> = Some(5);
let none: Option<i32> = None;

some.unwrap()              // 5 (panics if None)
some.unwrap_or(0)          // 5 (default if None)
some.unwrap_or_else(|| 42) // 5 (closure if None)
some.expect("msg")         // 5 (panics with msg if None)

some.is_some()  // true
none.is_none()  // true

some.map(|x| x * 2)       // Some(10)
none.map(|x| x * 2)       // None

some.and_then(|x| if x > 3 { Some(x) } else { None }) // Some(5)
none.or(Some(99))          // Some(99)

// Convert Option to Result:
some.ok_or("error")        // Ok(5)
none.ok_or("error")        // Err("error")
```

---

## 17. The Result Type — Error as Value

`Result<T, E>` represents a computation that might fail.

```rust
enum Result<T, E> {
    Ok(T),   // success, contains value of type T
    Err(E),  // failure, contains error of type E
}
```

```rust
use std::fs::File;
use std::io;

fn open_file(path: &str) -> Result<File, io::Error> {
    File::open(path)
}

match open_file("hello.txt") {
    Ok(file) => println!("File opened: {:?}", file),
    Err(e) => println!("Error: {}", e),
}
```

---

## 18. Pattern Matching — Destructuring Reality

Pattern matching is one of Rust's most powerful features. It allows you to destructure data and branch on its shape simultaneously.

### The `match` Expression

```rust
match VALUE {
    PATTERN_1 => EXPRESSION_1,
    PATTERN_2 => EXPRESSION_2,
    _ => DEFAULT_EXPRESSION,
}
```

**Key property**: `match` is an **expression** — it returns a value. All arms must return the same type.

```rust
let x: i32 = 5;

let description = match x {
    1 => "one",
    2 | 3 => "two or three",       // OR patterns
    4..=6 => "four through six",   // range pattern
    _ => "something else",          // wildcard
};
// description = "four through six"
```

### Matching Enums with Data

```rust
enum Shape {
    Circle { radius: f64 },
    Rectangle { width: f64, height: f64 },
    Triangle(f64, f64, f64),  // three sides
}

fn area(shape: &Shape) -> f64 {
    match shape {
        Shape::Circle { radius } => std::f64::consts::PI * radius * radius,
        Shape::Rectangle { width, height } => width * height,
        Shape::Triangle(a, b, c) => {
            // Heron's formula
            let s = (a + b + c) / 2.0;
            (s * (s - a) * (s - b) * (s - c)).sqrt()
        }
    }
}
```

### Guards — Additional Conditions

```rust
let pair = (0, -2);

match pair {
    (x, y) if x == y => println!("Equal"),
    (x, y) if x + y == 0 => println!("Sum is zero"),  // matches (0, -2) but not (0, 2)? Wait, 0+(-2)=-2, so it won't match this
    (x, _) if x % 2 == 0 => println!("First is even"),
    _ => println!("Other"),
}
```

### Destructuring Structs, Tuples, Arrays

```rust
// Struct destructuring
struct Point { x: i32, y: i32 }
let p = Point { x: 0, y: 7 };
let Point { x, y } = p;  // x=0, y=7

// Tuple destructuring
let (a, b, c) = (1, 2, 3);

// Array/slice destructuring
let arr = [1, 2, 3, 4, 5];
let [first, second, ..] = arr;  // first=1, second=2, rest ignored
let [.., last] = arr;            // last=5

// Nested destructuring
let ((x1, y1), (x2, y2)) = ((0, 0), (5, 10));

// Ignoring fields with ..
struct Point3D { x: i32, y: i32, z: i32 }
let p = Point3D { x: 0, y: 5, z: 10 };
let Point3D { y, .. } = p;  // only care about y
```

### `if let` and `while let`

```rust
// if let: match one pattern, ignore others
let maybe = Some(7);
if let Some(n) = maybe {
    println!("Got {}", n);
} else {
    println!("Nothing");
}

// while let: loop while pattern matches
let mut stack = vec![1, 2, 3];
while let Some(top) = stack.pop() {
    println!("{}", top);  // prints 3, 2, 1
}

// let-else (stabilized in Rust 1.65): require a pattern or else
let Some(n) = maybe else {
    panic!("Expected Some");
};
println!("n = {}", n);
```

### Bindings with `@`

```rust
let x = 5;
match x {
    n @ 1..=12 => println!("Got {} (1-12)", n),
    n @ 13..=19 => println!("Got {} (teen)", n),
    n => println!("Got {}", n),
}
```

---

## 19. Type Inference — What Rust Knows Without Being Told

Rust has **Hindley-Milner type inference** — it can infer types from how values are used, not just from their initial form.

```rust
// All equivalent:
let x: Vec<i32> = Vec::new();
let x = Vec::<i32>::new();
let mut x = Vec::new();
x.push(1i32);  // Rust infers Vec<i32> from the push

// Inference works across statements:
let mut map = std::collections::HashMap::new();
map.insert("key", 42u32);
// Rust infers: HashMap<&str, u32>

// Inference with closures:
let numbers = vec![1, 2, 3, 4, 5];
let doubled: Vec<i32> = numbers.iter().map(|x| x * 2).collect();
// The collect() type is driven by the Vec<i32> annotation
```

### The Turbofish `::<>`

When inference cannot determine the type, use turbofish:

```rust
let parsed: i32 = "42".parse().unwrap();  // annotation on binding
let parsed = "42".parse::<i32>().unwrap(); // turbofish on method
```

---

# PART IV — TRAITS AND GENERICS

---

## 20. Traits — Shared Behavior Interfaces

A **trait** defines a set of methods that a type must implement. Similar to interfaces in Java/Go, but more powerful.

### Defining and Implementing Traits

```rust
trait Animal {
    // Required method — must be implemented
    fn name(&self) -> &str;
    fn sound(&self) -> &str;
    
    // Default method — may be overridden
    fn describe(&self) -> String {
        format!("{} says {}", self.name(), self.sound())
    }
}

struct Dog { name: String }
struct Cat { name: String }

impl Animal for Dog {
    fn name(&self) -> &str { &self.name }
    fn sound(&self) -> &str { "woof" }
    // describe() uses default implementation
}

impl Animal for Cat {
    fn name(&self) -> &str { &self.name }
    fn sound(&self) -> &str { "meow" }
    fn describe(&self) -> String {
        format!("{} says {} elegantly", self.name(), self.sound())
    }
}
```

### The Orphan Rule

```
┌─────────────────────────────────────────────────────────┐
│                    THE ORPHAN RULE                       │
│                                                         │
│  You can implement a trait for a type ONLY IF:          │
│  • The TRAIT is defined in your crate, OR               │
│  • The TYPE is defined in your crate                    │
│  (at least one must be "local" to your crate)           │
│                                                         │
│  You CANNOT implement std::fmt::Display for Vec<i32>    │
│  because both Display and Vec are from std (external).  │
│                                                         │
│  This prevents incoherence (conflicting implementations)│
└─────────────────────────────────────────────────────────┘
```

---

## 21. Generics — Writing Code for All Types

Generics allow you to write code that works for any type that satisfies given constraints.

### Generic Functions

```rust
// T is a type parameter — a placeholder for any type
fn largest<T: PartialOrd>(list: &[T]) -> &T {
    let mut largest = &list[0];
    for item in list {
        if item > largest {
            largest = item;
        }
    }
    largest
}

// Works for any type implementing PartialOrd:
let numbers = vec![34, 50, 25, 100, 65];
let chars = vec!['y', 'm', 'a', 'q'];
println!("{}", largest(&numbers)); // 100
println!("{}", largest(&chars));   // y
```

### Generic Structs and Enums

```rust
struct Pair<T> {
    first: T,
    second: T,
}

impl<T> Pair<T> {
    fn new(first: T, second: T) -> Self {
        Self { first, second }
    }
}

// Specialized impl for types that support comparison and display:
impl<T: PartialOrd + std::fmt::Display> Pair<T> {
    fn cmp_display(&self) {
        if self.first >= self.second {
            println!("Larger: {}", self.first);
        } else {
            println!("Larger: {}", self.second);
        }
    }
}
```

### Monomorphization — Zero-Cost Generics

```
Generics in Rust are ZERO COST at runtime.

The compiler performs MONOMORPHIZATION:
it generates a separate copy of the function for each concrete type used.

fn largest<T: PartialOrd>(list: &[T]) -> &T { ... }

After monomorphization:
fn largest_i32(list: &[i32]) -> &i32 { ... }  // generated
fn largest_char(list: &[char]) -> &char { ... } // generated

Binary contains concrete functions, no runtime overhead.
Cost: larger binary size (code bloat).
```

---

## 22. Trait Bounds — Constraining Generics

Trait bounds specify what capabilities a generic type must have.

### Syntax Forms

```rust
// Inline bound:
fn print<T: Display>(x: T) { println!("{}", x); }

// Multiple bounds with +:
fn print<T: Display + Debug + Clone>(x: T) { ... }

// Where clause (cleaner for complex bounds):
fn process<T, U>(t: T, u: U) -> String
where
    T: Display + Clone,
    U: Debug + PartialOrd,
{
    format!("{:?}", u)
}

// impl Trait in parameter position (syntactic sugar):
fn print(x: impl Display) { println!("{}", x); }
// Equivalent to: fn print<T: Display>(x: T)
```

---

## 23. Associated Types — Types Inside Traits

Associated types let traits define placeholder types that implementations specify.

```rust
trait Container {
    type Item;  // associated type — implementors define this
    
    fn first(&self) -> Option<&Self::Item>;
    fn len(&self) -> usize;
    fn is_empty(&self) -> bool { self.len() == 0 }
}

struct Stack<T> {
    data: Vec<T>,
}

impl<T> Container for Stack<T> {
    type Item = T;  // specify the associated type
    
    fn first(&self) -> Option<&T> {
        self.data.first()
    }
    
    fn len(&self) -> usize {
        self.data.len()
    }
}
```

### Associated Types vs Generics

```
ASSOCIATED TYPE: trait Iter { type Item; fn next() -> Option<Self::Item> }
  - One implementation per type
  - Iterator for Vec<i32> has Item=i32 — fixed
  - Cleaner API: no need to specify type at call site

GENERIC PARAMETER: trait Into<T> { fn into(self) -> T; }
  - Multiple implementations possible (String can be Into<String>, Into<Vec<u8>>, etc.)
  - Caller can specify: into::<String>()
  - More flexible, more verbose
```

---

## 24. Trait Objects — Dynamic Dispatch

When you need a collection of different types that share a trait, use **trait objects** (`dyn Trait`).

```rust
trait Draw {
    fn draw(&self);
}

struct Button { label: String }
struct Image { src: String }

impl Draw for Button {
    fn draw(&self) { println!("Drawing button: {}", self.label); }
}
impl Draw for Image {
    fn draw(&self) { println!("Drawing image: {}", self.src); }
}

// Trait object: Box<dyn Draw> — heterogeneous collection
let components: Vec<Box<dyn Draw>> = vec![
    Box::new(Button { label: "OK".to_string() }),
    Box::new(Image { src: "logo.png".to_string() }),
];

for component in &components {
    component.draw();  // dynamic dispatch — vtable lookup at runtime
}
```

### Static vs Dynamic Dispatch

```
STATIC DISPATCH (generics / impl Trait):
  ┌────────────────────────────────┐
  │ fn draw_all<T: Draw>(items: &[T])│
  │                                │
  │ Compiler generates separate    │
  │ code for each T at compile time│
  │ Direct function call — fast    │
  │ Binary grows with each type    │
  └────────────────────────────────┘

DYNAMIC DISPATCH (dyn Trait):
  ┌─────────────────────────────────────────────┐
  │ Box<dyn Draw>                                │
  │                                             │
  │ Fat pointer: [data ptr | vtable ptr]        │
  │                                             │
  │ vtable:                                     │
  │ ┌──────────┬──────────┬──────────┐          │
  │ │ drop fn  │ size     │ draw fn  │  ...     │
  │ └──────────┴──────────┴──────────┘          │
  │                                             │
  │ At runtime: look up function in vtable      │
  │ One copy of code (smaller binary)           │
  │ Slight runtime overhead (pointer dereference│
  │ + indirect call)                            │
  │ Enables heterogeneous collections           │
  └─────────────────────────────────────────────┘
```

### Object Safety

Not all traits can be used as `dyn Trait`. A trait is **object-safe** if:
- It has no methods that return `Self`
- It has no generic methods
- All methods have receivers (`&self`, `&mut self`, or `self`)

```rust
// NOT object safe:
trait Clone {
    fn clone(&self) -> Self;  // returns Self — can't be dyn
}

// Object safe:
trait Draw {
    fn draw(&self);  // no Self in return, no generics
}
```

---

## 25. The Standard Library Traits — Essential Taxonomy

```
DISPLAY & DEBUG:
  fmt::Display   — human-readable formatting ({})
  fmt::Debug     — debug formatting ({:?}) — #[derive(Debug)]

COMPARISON:
  PartialEq      — == and != (not all values comparable, e.g., NaN)
  Eq             — full equivalence (all values comparable)
  PartialOrd     — < > <= >= (partial ordering, e.g., floats)
  Ord            — total ordering (all values comparable)

ARITHMETIC:
  Add            — + operator
  Sub            — - operator
  Mul            — * operator
  Div            — / operator
  Rem            — % operator
  Neg            — unary - operator

MEMORY:
  Clone          — explicit duplication (.clone())
  Copy           — implicit bitwise copy (marker trait)
  Drop           — custom cleanup logic

CONVERSION:
  From<T>        — infallible conversion from T
  Into<T>        — infallible conversion to T (auto from From)
  TryFrom<T>     — fallible conversion from T → Result
  TryInto<T>     — fallible conversion to T → Result
  AsRef<T>       — cheap reference conversion
  AsMut<T>       — cheap mutable reference conversion

ITERATION:
  Iterator       — next() method, enables for loops
  IntoIterator   — converts to Iterator
  FromIterator   — collects from iterator (for collect())
  Extend         — extends from iterator

CLOSURES:
  Fn             — callable, can call multiple times, shared borrow
  FnMut          — callable, can call multiple times, mutable borrow
  FnOnce         — callable, can call only once, takes ownership

I/O:
  Read           — read bytes
  Write          — write bytes
  BufRead        — buffered reading (read_line)
  Seek           — seek in a stream

POINTERS:
  Deref          — * dereference operator (-> T)
  DerefMut       — mutable dereference
  Index          — [] indexing
  IndexMut       — mutable [] indexing

CONCURRENCY:
  Send           — type safe to transfer across threads
  Sync           — type safe to share reference across threads

HASHING:
  Hash           — hashable (for HashMap keys)
  Hasher         — the hashing algorithm
  BuildHasher    — creates Hashers
```

---

## 26. Operator Overloading via Traits

```rust
use std::ops::Add;

#[derive(Debug, Clone, Copy)]
struct Vector2 {
    x: f64,
    y: f64,
}

impl Add for Vector2 {
    type Output = Vector2;  // required associated type

    fn add(self, other: Vector2) -> Vector2 {
        Vector2 {
            x: self.x + other.x,
            y: self.y + other.y,
        }
    }
}

let v1 = Vector2 { x: 1.0, y: 2.0 };
let v2 = Vector2 { x: 3.0, y: 4.0 };
let v3 = v1 + v2;  // calls Add::add
```

---

# PART V — FUNCTIONS, CLOSURES, ITERATORS

---

## 27. Functions — Rust's Callable Units

```rust
// Basic function:
fn add(x: i32, y: i32) -> i32 {
    x + y  // no semicolon = expression = return value
}

// With early return:
fn divide(a: f64, b: f64) -> Option<f64> {
    if b == 0.0 {
        return None;  // explicit return
    }
    Some(a / b)  // implicit return (last expression)
}

// Never-returning function:
fn crash(msg: &str) -> ! {
    panic!("{}", msg)
    // ! means this function never returns (diverging function)
}

// Functions are first-class values:
let f: fn(i32, i32) -> i32 = add;  // function pointer
let result = f(3, 4);  // 7
```

### Function Items vs Function Pointers vs Closures

```
FUNCTION ITEM:    fn add(x: i32, y: i32) -> i32 { x + y }
  - Zero-sized type (no data)
  - Monomorphized at compile time
  - fn(i32, i32) -> i32 is its type

FUNCTION POINTER: fn(i32, i32) -> i32
  - 8-byte pointer to function
  - Can only point to non-capturing functions/closures
  - Can be stored in arrays, passed as C callbacks

CLOSURE:          |x, y| x + y
  - Struct-like type generated by compiler
  - Captures environment (may have data)
  - Implements Fn/FnMut/FnOnce traits
  - Cannot be stored as fn pointer if it captures
```

---

## 28. Closures — Functions That Capture Environment

A closure is an anonymous function that can **capture variables from its enclosing scope**.

```rust
let x = 4;
let equal_to_x = |z| z == x;  // captures x by reference
println!("{}", equal_to_x(4)); // true

// Closure with explicit types (usually inferred):
let multiply = |a: i32, b: i32| -> i32 { a * b };

// Multi-line closure:
let process = |v: Vec<i32>| {
    let filtered: Vec<i32> = v.into_iter().filter(|&n| n > 0).collect();
    filtered.len()
};
```

### How Closures Capture

```
CAPTURING RULES:
The compiler determines HOW to capture based on how the variable is used:

1. If the closure only READS the variable:     captures by &T (shared ref)
2. If the closure MODIFIES the variable:       captures by &mut T
3. If the closure needs to OWN the variable:   captures by value (move)

You can FORCE capture by value with the `move` keyword:
let s = String::from("hello");
let closure = move || println!("{}", s);  // s is MOVED into closure
// s is no longer valid here
```

### Closure Memory Layout

```
let x = 10;
let y = 20;
let closure = |z| x + y + z;

The compiler generates something like:
struct __Closure {
    x: &i32,   // reference to captured x
    y: &i32,   // reference to captured y
}
impl Fn(i32) -> i32 for __Closure {
    fn call(&self, z: i32) -> i32 {
        *self.x + *self.y + z
    }
}
```

---

## 29. Fn / FnMut / FnOnce — The Closure Trait Hierarchy

```
HIERARCHY:
  FnOnce ← FnMut ← Fn

  Every Fn implements FnMut
  Every FnMut implements FnOnce
  
  Fn: strongest requirement (most restrictive for closure, most flexible for caller)
  FnOnce: weakest requirement (least restrictive for closure, most restrictive for caller)
```

```
┌──────────────────────────────────────────────────────┐
│   TRAIT    │  CAPTURES   │  CALLS   │  WHEN USED     │
├────────────┼─────────────┼──────────┼────────────────┤
│  FnOnce    │  By value   │  Once    │ Consumes captured│
│            │  (moves)    │  only    │ values          │
├────────────┼─────────────┼──────────┼────────────────┤
│  FnMut     │  By &mut    │  Multiple│ Mutates captured│
│            │  reference  │  times   │ state           │
├────────────┼─────────────┼──────────┼────────────────┤
│  Fn        │  By &       │  Multiple│ Read-only access│
│            │  reference  │  times   │ to captured     │
└──────────────────────────────────────────────────────┘
```

```rust
// FnOnce — can be called only once (consumes a captured value)
let s = String::from("hello");
let consume = move || {
    drop(s);  // s is consumed (dropped) when called
};
consume();    // OK
// consume(); // ERROR: FnOnce can only be called once

// FnMut — mutates captured state
let mut count = 0;
let mut increment = || {
    count += 1;  // mutates captured count
    count
};
println!("{}", increment()); // 1
println!("{}", increment()); // 2

// Fn — read-only access
let greeting = String::from("hello");
let say_hello = || println!("{}", greeting);  // just reads
say_hello();
say_hello();  // can call multiple times
```

---

## 30. Iterators — The Lazy Sequence Protocol

The `Iterator` trait is the foundation for processing sequences in Rust.

```rust
trait Iterator {
    type Item;
    fn next(&mut self) -> Option<Self::Item>;
    // ... many default methods built on next()
}
```

### The Iterator State Machine

```
Iterator State:

┌─────────────┐
│  [1, 2, 3]  │  ← source data
└─────────────┘
       │
       ▼ .iter() or .into_iter()
┌─────────────────────────────┐
│  IntoIter / Iter / IterMut  │  ← iterator wrapper with internal state
│  current_index: 0           │
└─────────────────────────────┘
       │
       ▼ .next() called by for loop
  Some(&1)  →  Some(&2)  →  Some(&3)  →  None
```

### Three Ways to Create an Iterator from a Collection

```rust
let v = vec![1, 2, 3];

// .iter()      → yields &T (borrows elements)
for x in v.iter() { println!("{}", x); }  // x: &i32

// .iter_mut()  → yields &mut T (mutably borrows elements)
for x in v.iter_mut() { *x *= 2; }        // x: &mut i32

// .into_iter() → yields T (consumes collection, takes ownership)
for x in v.into_iter() { println!("{}", x); } // x: i32, v is consumed
// v is no longer valid after into_iter() consumes it
```

---

## 31. Iterator Adapters — Functional Data Pipelines

Iterator adapters are **lazy** — they don't do any work until consumed.

```
PIPELINE:

source.iter()     ← create iterator
  .filter(|x| ..)  ← adapter (lazy, no work yet)
  .map(|x| ..)     ← adapter (lazy, no work yet)
  .take(5)         ← adapter (lazy, no work yet)
  .collect()       ← CONSUMER — drives the pipeline, does work
```

### Key Adapters

```rust
let numbers = vec![1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

// map — transform each element
let doubled: Vec<i32> = numbers.iter().map(|&x| x * 2).collect();

// filter — keep elements matching predicate
let evens: Vec<&i32> = numbers.iter().filter(|&&x| x % 2 == 0).collect();

// filter_map — filter and transform in one step
let results: Vec<i32> = numbers.iter()
    .filter_map(|&x| if x > 5 { Some(x * 10) } else { None })
    .collect();

// fold — reduce to single value (like reduce/inject in other languages)
let sum: i32 = numbers.iter().fold(0, |acc, &x| acc + x); // 55

// enumerate — add index to each element
for (i, val) in numbers.iter().enumerate() {
    println!("Index {}: {}", i, val);
}

// zip — combine two iterators element by element
let letters = vec!['a', 'b', 'c'];
let paired: Vec<(i32, char)> = numbers.iter().copied()
    .zip(letters.iter().copied()).collect();

// chain — concatenate two iterators
let more = vec![11, 12, 13];
let all: Vec<i32> = numbers.iter().copied()
    .chain(more.iter().copied()).collect();

// flat_map — map then flatten
let words = vec!["hello world", "foo bar"];
let chars: Vec<&str> = words.iter()
    .flat_map(|s| s.split_whitespace()).collect();

// take / skip
let first_three: Vec<i32> = numbers.iter().copied().take(3).collect();
let after_three: Vec<i32> = numbers.iter().copied().skip(3).collect();

// take_while / skip_while
let head: Vec<i32> = numbers.iter().copied().take_while(|&x| x < 5).collect();

// peekable — look at next without consuming
let mut iter = numbers.iter().peekable();
if let Some(&&first) = iter.peek() {
    println!("First (without consuming): {}", first);
}

// windows / chunks (on slices)
for window in numbers.windows(3) {
    println!("{:?}", window); // [1,2,3], [2,3,4], ...
}
for chunk in numbers.chunks(3) {
    println!("{:?}", chunk);  // [1,2,3], [4,5,6], ...
}
```

### Key Consumers

```rust
// sum, product
let sum: i32 = numbers.iter().sum();     // 55
let product: i32 = numbers.iter().product(); // 3628800

// count
let n = numbers.iter().filter(|&&x| x > 5).count(); // 5

// any / all
let any_even = numbers.iter().any(|&x| x % 2 == 0); // true
let all_positive = numbers.iter().all(|&x| x > 0);   // true

// find / position
let first_even = numbers.iter().find(|&&x| x % 2 == 0); // Some(&2)
let pos = numbers.iter().position(|&x| x == 5);          // Some(4)

// max / min
let maximum = numbers.iter().max(); // Some(&10)
let minimum = numbers.iter().min(); // Some(&1)

// collect into various types
let set: std::collections::HashSet<i32> = numbers.into_iter().collect();
let string: String = vec!['h','e','l','l','o'].into_iter().collect();

// for_each (eager map, when you don't want to collect)
numbers.iter().for_each(|x| println!("{}", x));
```

---

## 32. Custom Iterators — Implementing Iterator

```rust
struct Counter {
    count: u32,
    max: u32,
}

impl Counter {
    fn new(max: u32) -> Counter {
        Counter { count: 0, max }
    }
}

impl Iterator for Counter {
    type Item = u32;

    fn next(&mut self) -> Option<u32> {
        if self.count < self.max {
            self.count += 1;
            Some(self.count)
        } else {
            None
        }
    }
}

// Now Counter has access to ALL iterator methods for free!
let sum: u32 = Counter::new(5)
    .zip(Counter::new(5).skip(1))
    .map(|(a, b)| a * b)
    .filter(|x| x % 3 == 0)
    .sum();
```

---

# PART VI — ERROR HANDLING

---

## 33. The Philosophy of Error Handling in Rust

Rust makes a deliberate distinction between:

```
RECOVERABLE ERRORS:  Use Result<T, E>
  - File not found
  - Network timeout
  - Parse failure
  - Invalid input
  → The program can handle these and continue

UNRECOVERABLE ERRORS: Use panic!()
  - Bug in the program (invariant violated)
  - Index out of bounds (programming error)
  - Integer overflow (in debug mode)
  → The program cannot reasonably continue
```

Rust's philosophy: **errors are values**. An error is just another kind of data that the type system forces you to handle.

---

## 34. Option<T> in Depth

```rust
// Creating Options:
let some: Option<i32> = Some(42);
let none: Option<i32> = None;

// Pattern matching (most explicit):
match some {
    Some(n) => println!("{}", n),
    None => println!("nothing"),
}

// Method chaining (functional style):
let result = some
    .filter(|&n| n > 10)     // keep if predicate true
    .map(|n| n * 2)           // transform the value
    .unwrap_or(0);            // extract or use default

// Converting between Option and Result:
some.ok_or("missing")         // Option → Result<i32, &str>
some.ok_or_else(|| "missing") // lazy version

// Combining Options:
let a: Option<i32> = Some(1);
let b: Option<i32> = Some(2);
let c: Option<i32> = None;

// and_then (flatmap — chain operations that might fail):
let r = a.and_then(|x| b.map(|y| x + y)); // Some(3)
let r = a.and_then(|_| c);                 // None (because c is None)

// or / or_else (fallback):
c.or(a)                     // Some(1) — use a if c is None
c.or_else(|| Some(99))      // Some(99)

// zip — combine two Options into Option<(A, B)>:
a.zip(b)  // Some((1, 2))
a.zip(c)  // None

// transpose — Option<Result<T, E>> ↔ Result<Option<T>, E>:
let opt_result: Option<Result<i32, &str>> = Some(Ok(42));
let res_opt: Result<Option<i32>, &str> = opt_result.transpose(); // Ok(Some(42))

// flatten — Option<Option<T>> → Option<T>:
let nested: Option<Option<i32>> = Some(Some(5));
nested.flatten()  // Some(5)
```

---

## 35. Result<T, E> in Depth

```rust
use std::num::ParseIntError;

// Creating Results:
let ok: Result<i32, String> = Ok(42);
let err: Result<i32, String> = Err("something failed".to_string());

// Pattern matching:
match "42".parse::<i32>() {
    Ok(n) => println!("Parsed: {}", n),
    Err(e) => println!("Error: {}", e),
}

// Method chaining:
let result = "42"
    .parse::<i32>()
    .map(|n| n * 2)           // transform Ok value
    .map_err(|e| e.to_string()) // transform Err value
    .unwrap_or(0);             // default on Err

// Result combinators:
ok.map(|n| n + 1)             // Ok(43)
ok.map_err(|e| e.len())       // Ok(42)
err.map(|n| n + 1)            // Err("something failed")
err.map_err(|e| e.len())      // Err(16)

ok.and(err)                   // Err("something failed")
ok.and_then(|n| if n > 0 { Ok(n) } else { Err("negative".to_string()) })

err.or(ok)                    // Ok(42)
err.or_else(|_| Ok(0))        // Ok(0)

// unwrap variants:
ok.unwrap()                   // 42 (panics if Err)
ok.expect("should work")      // 42 (panics with message if Err)
ok.unwrap_or(0)               // 42 (default if Err)
ok.unwrap_or_else(|_| 0)      // 42 (closure if Err)
ok.unwrap_or_default()        // 42 (Default::default() if Err)

// Checking:
ok.is_ok()                    // true
err.is_err()                  // true

// Converting Result to Option:
ok.ok()                       // Some(42) (drops error)
ok.err()                      // None
err.err()                     // Some("something failed")

// flatten — Result<Result<T, E>, E> → Result<T, E>:
let nested: Result<Result<i32, &str>, &str> = Ok(Ok(5));
nested.flatten()              // Ok(5)
```

---

## 36. The `?` Operator — Error Propagation

The `?` operator is syntactic sugar for propagating errors up the call stack.

```rust
// WITHOUT ? (verbose):
fn parse_and_double(s: &str) -> Result<i32, std::num::ParseIntError> {
    let n = match s.parse::<i32>() {
        Ok(n) => n,
        Err(e) => return Err(e),
    };
    Ok(n * 2)
}

// WITH ? (idiomatic):
fn parse_and_double(s: &str) -> Result<i32, std::num::ParseIntError> {
    let n = s.parse::<i32>()?;  // returns Err if parse fails
    Ok(n * 2)
}

// ? EXPANDS TO approximately:
// match expr {
//     Ok(val) => val,
//     Err(e) => return Err(From::from(e)),  // From::from for type conversion!
// }
```

### `?` with From — Automatic Error Conversion

```rust
use std::num::ParseIntError;
use std::io;

#[derive(Debug)]
enum AppError {
    Parse(ParseIntError),
    Io(io::Error),
}

impl From<ParseIntError> for AppError {
    fn from(e: ParseIntError) -> Self { AppError::Parse(e) }
}
impl From<io::Error> for AppError {
    fn from(e: io::Error) -> Self { AppError::Io(e) }
}

fn do_work() -> Result<i32, AppError> {
    let content = std::fs::read_to_string("num.txt")?; // io::Error → AppError::Io
    let n = content.trim().parse::<i32>()?;             // ParseIntError → AppError::Parse
    Ok(n * 2)
}
```

### `?` with Option

`?` also works in functions returning `Option<T>`:

```rust
fn first_char(s: &str) -> Option<char> {
    let c = s.chars().next()?;  // returns None if empty
    Some(c)
}
```

---

## 37. Custom Error Types

```rust
use std::fmt;

#[derive(Debug)]
pub enum DatabaseError {
    ConnectionFailed { host: String, port: u16 },
    QueryFailed { query: String, cause: String },
    NotFound { id: u64 },
}

impl fmt::Display for DatabaseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            DatabaseError::ConnectionFailed { host, port } =>
                write!(f, "Connection to {}:{} failed", host, port),
            DatabaseError::QueryFailed { query, cause } =>
                write!(f, "Query '{}' failed: {}", query, cause),
            DatabaseError::NotFound { id } =>
                write!(f, "Record {} not found", id),
        }
    }
}

// Implement std::error::Error (empty body — trait has defaults):
impl std::error::Error for DatabaseError {}

// Now DatabaseError works with trait objects Box<dyn Error>
```

---

## 38. Error Libraries — thiserror and anyhow

### thiserror (for library authors — define precise errors)

```rust
use thiserror::Error;

#[derive(Error, Debug)]
pub enum AppError {
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Parse error: {0}")]
    Parse(#[from] std::num::ParseIntError),

    #[error("Not found: {key}")]
    NotFound { key: String },

    #[error("Invalid input: {0}")]
    Invalid(String),
}
// Generates Display and From implementations automatically
```

### anyhow (for applications — flexible error handling)

```rust
use anyhow::{Result, Context, anyhow, bail};

fn read_config(path: &str) -> Result<Config> {
    let content = std::fs::read_to_string(path)
        .with_context(|| format!("Failed to read config from {}", path))?;

    let config: Config = serde_json::from_str(&content)
        .context("Failed to parse config")?;

    if config.port == 0 {
        bail!("Port cannot be zero");  // early return with error
    }
    
    Ok(config)
}
```

---

# PART VII — SMART POINTERS

---

## 39. Box<T> — Heap Allocation with Ownership

`Box<T>` is the simplest smart pointer — it puts a value on the heap and gives you a single owned pointer to it.

```rust
let b = Box::new(5);         // 5 is on the heap
println!("{}", b);           // auto-derefs: prints 5
println!("{}", *b);          // explicit deref: also 5

// b is dropped when it goes out of scope → heap memory freed
```

### When to Use Box<T>

```
1. RECURSIVE TYPES that would have infinite size:

   // ERROR: type has infinite size
   enum List {
       Cons(i32, List),  // List contains List contains List...
   }

   // CORRECT: Box breaks the cycle (known size: ptr + i32)
   enum List {
       Cons(i32, Box<List>),
       Nil,
   }

2. LARGE DATA to avoid stack overflow or expensive moves:

   let big_data = Box::new([0u8; 1_000_000]);  // heap, not stack

3. TRAIT OBJECTS:

   let animal: Box<dyn Animal> = Box::new(Dog { name: "Rex".to_string() });
```

### Box Memory Layout

```
STACK                           HEAP
┌─────────────────┐             ┌────────────┐
│ b: Box<i32>     │             │            │
│ ptr: ───────────┼─────────────►  5 (i32)   │
└─────────────────┘             └────────────┘
 8 bytes (pointer)               4 bytes

When b is dropped:
  1. Box::drop() calls deallocate on the heap pointer
  2. Heap memory is freed
```

---

## 40. Rc<T> — Reference Counted Shared Ownership

`Rc<T>` enables **multiple owners** of the same data. A reference count tracks how many owners exist; when count reaches 0, the data is freed.

```rust
use std::rc::Rc;

let a = Rc::new(5);         // count: 1
let b = Rc::clone(&a);      // count: 2 (NOT a deep clone — just increments count)
let c = Rc::clone(&a);      // count: 3

println!("Count: {}", Rc::strong_count(&a)); // 3

drop(b);                    // count: 2
drop(c);                    // count: 1
// a dropped here:           count: 0 → heap freed
```

```
MEMORY LAYOUT:

STACK      │     HEAP
───────────┼──────────────────────────────────────────
a: Rc<i32> │  ┌────────────────────────────────────┐
  ptr ─────┼─►│ strong_count: 3                    │
           │  │ weak_count: 0                      │
b: Rc<i32> │  │ value: 5                           │
  ptr ─────┼─►└────────────────────────────────────┘
           │        ▲
c: Rc<i32> │        │
  ptr ─────┼────────┘
```

**Critical limitation**: `Rc<T>` is **NOT thread-safe**. It uses non-atomic integer operations. Use `Arc<T>` for multi-threaded code.

### Rc::downgrade — Weak References

```rust
use std::rc::{Rc, Weak};

let strong = Rc::new(5);
let weak: Weak<i32> = Rc::downgrade(&strong);  // weak count+1, strong count unchanged

// Weak doesn't prevent deallocation
// Must upgrade to use:
match weak.upgrade() {
    Some(val) => println!("{}", val),  // still alive
    None => println!("value dropped"), // original Rc gone
}
```

**Use case**: Break reference cycles. If A has Rc to B and B has Rc to A, neither is ever dropped. Use Weak for the "child→parent" direction.

---

## 41. Arc<T> — Atomic Reference Counted

`Arc<T>` is the thread-safe version of `Rc<T>`. Uses atomic operations for the reference count.

```rust
use std::sync::Arc;
use std::thread;

let data = Arc::new(vec![1, 2, 3]);

let mut handles = vec![];
for _ in 0..3 {
    let data_clone = Arc::clone(&data);  // clone the Arc, not the Vec
    let handle = thread::spawn(move || {
        println!("{:?}", data_clone);    // read-only shared access
    });
    handles.push(handle);
}

for handle in handles { handle.join().unwrap(); }
```

**Why `Arc<T>` but not `Rc<T>` in threads?**: The `Send` and `Sync` traits enforce this. `Rc<T>` doesn't implement `Send` — the compiler prevents moving it into another thread.

---

## 42. Cell<T> and RefCell<T> — Interior Mutability

**Interior mutability**: the ability to mutate data even through shared (immutable) references. This sidesteps the normal rules — but enforces them at **runtime** instead of compile time.

### Cell<T> — For Copy Types

```rust
use std::cell::Cell;

let cell = Cell::new(5);
println!("{}", cell.get()); // 5
cell.set(10);
println!("{}", cell.get()); // 10

// Key: cell is NOT mut, yet we modified it!
// Works because Cell uses UnsafeCell internally
// But Cell is single-threaded only
```

### RefCell<T> — Dynamic Borrow Checking

```rust
use std::cell::RefCell;

let data = RefCell::new(vec![1, 2, 3]);

// Borrow immutably:
let read = data.borrow();  // returns Ref<Vec<i32>>
println!("{:?}", *read);

// Borrow mutably:
drop(read);  // must drop before mutably borrowing
let mut write = data.borrow_mut(); // returns RefMut<Vec<i32>>
write.push(4);

// If rules violated, panic at runtime:
// let r1 = data.borrow();
// let r2 = data.borrow_mut();  // RUNTIME PANIC: already borrowed
```

### When to Use RefCell<T>

```
Use RefCell<T> when:
  - You need interior mutability
  - You know the borrow rules are safe but the compiler can't see it
  - You're in single-threaded code

Common pattern: Rc<RefCell<T>> for shared mutable data in single-threaded code
Common pattern: Arc<Mutex<T>> for shared mutable data in multi-threaded code

BORROW CHECKING COMPARISON:

         │  COMPILE TIME  │  RUNTIME
─────────┼────────────────┼──────────────────────────
 OWNED   │   T            │   N/A
 SHARED  │   &T           │   Ref<T> (from RefCell)
 MUTABLE │   &mut T       │   RefMut<T> (from RefCell)
 SHARED  │   Rc<T>        │   Rc<RefCell<T>>
 MUTABLE │   Arc<Mutex<T>>│   Arc<RwLock<T>>
```

---

## 43. Mutex<T> and RwLock<T> — Thread-Safe Interior Mutability

```rust
use std::sync::{Arc, Mutex};

let counter = Arc::new(Mutex::new(0));
let mut handles = vec![];

for _ in 0..10 {
    let c = Arc::clone(&counter);
    let h = std::thread::spawn(move || {
        let mut num = c.lock().unwrap();  // blocks until lock acquired
        *num += 1;
        // MutexGuard dropped here → lock released
    });
    handles.push(h);
}
for h in handles { h.join().unwrap(); }
println!("{}", *counter.lock().unwrap()); // 10
```

```
MUTEX STATES:

     lock() called        lock released (guard dropped)
          │                        │
UNLOCKED──┼──► LOCKED ─────────────┼──► UNLOCKED
          │         │              │
          │   Other threads        │
          │   calling lock()       │
          │   BLOCK here           │
          └───◄────────────────────┘
                  (waiting)
```

### RwLock — Multiple Readers OR One Writer

```rust
use std::sync::RwLock;

let lock = RwLock::new(5);

// Multiple readers simultaneously:
let r1 = lock.read().unwrap();
let r2 = lock.read().unwrap();
println!("{} {}", *r1, *r2);
drop(r1); drop(r2);

// One writer at a time (blocks readers and other writers):
let mut w = lock.write().unwrap();
*w += 1;
```

---

## 44. Cow<'a, B> — Clone on Write

`Cow` (Clone on Write) is a smart pointer for efficiently handling "might need to mutate" scenarios:

```rust
use std::borrow::Cow;

fn process(input: &str) -> Cow<str> {
    if input.contains(' ') {
        Cow::Owned(input.replace(' ', "_"))  // had to allocate
    } else {
        Cow::Borrowed(input)  // no allocation needed
    }
}

let a = process("hello");       // Borrowed — no allocation
let b = process("hello world"); // Owned — allocated a new String
```

---

## 45. Pin<T> — Pinning Values in Memory

`Pin<T>` guarantees that a value **will not be moved in memory** after being pinned. This is crucial for self-referential structs and async/await.

```rust
use std::pin::Pin;

// Pin<Box<T>> — pinned heap allocation
let mut boxed = Box::new(5);
let pinned: Pin<Box<i32>> = Box::pin(5);

// Cannot move out of pinned data.
// The value's memory address is stable forever.
```

This is explored more in Part IX (Async/Await).

---

# PART VIII — CONCURRENCY

---

## 46. The Fearless Concurrency Philosophy

Rust's ownership and type systems enable **fearless concurrency** — the compiler prevents data races at compile time.

```
DATA RACE: Two conditions met simultaneously:
  1. Two or more threads access the same data
  2. At least one thread writes
  3. No synchronization

Rust PREVENTS data races because:
  - Send trait: only types safe to transfer to another thread
    can be moved into threads
  - Sync trait: only types safe to reference from multiple threads
    can have references shared across threads
  - Ownership: only one mutable reference OR many immutable
    references at a time
```

---

## 47. Threads — OS-Level Parallelism

```rust
use std::thread;
use std::time::Duration;

// Spawn a thread:
let handle = thread::spawn(|| {
    for i in 1..=5 {
        println!("spawned: {}", i);
        thread::sleep(Duration::from_millis(1));
    }
});

// Main thread continues:
for i in 1..=3 {
    println!("main: {}", i);
    thread::sleep(Duration::from_millis(1));
}

handle.join().unwrap();  // wait for spawned thread to finish
```

### Sharing Data with Threads — move Closures

```rust
let v = vec![1, 2, 3];

let handle = thread::spawn(move || {
    // v is MOVED into the thread — thread owns it now
    println!("{:?}", v);
});

// v is not accessible in main thread anymore
handle.join().unwrap();
```

### Thread-Local Storage

```rust
use std::cell::RefCell;

thread_local! {
    static COUNTER: RefCell<u32> = RefCell::new(0);
}

COUNTER.with(|c| {
    *c.borrow_mut() += 1;
    println!("counter: {}", c.borrow());
});
```

---

## 48. Message Passing — Channels

Channels provide a way to communicate between threads. Rust's motto: **"Do not communicate by sharing memory; share memory by communicating."**

```rust
use std::sync::mpsc;  // multiple producer, single consumer
use std::thread;

let (tx, rx) = mpsc::channel();

thread::spawn(move || {
    tx.send("hello from thread").unwrap();
    tx.send("another message").unwrap();
    // tx dropped here, channel closed
});

// Receive:
let msg = rx.recv().unwrap();       // blocks until message arrives
println!("{}", msg);                 // "hello from thread"

// Or iterate until channel closed:
for msg in rx {
    println!("{}", msg);
}
```

### Multiple Producers

```rust
let (tx, rx) = mpsc::channel();
let tx2 = tx.clone();  // clone the sender for multiple producers

thread::spawn(move || { tx.send(1).unwrap(); });
thread::spawn(move || { tx2.send(2).unwrap(); });

for val in rx { println!("{}", val); }
```

---

## 49. Shared State — Arc + Mutex

```
                 ┌─────────────────────────┐
                 │    Arc<Mutex<Data>>      │
                 │                         │
   Thread 1 ────►│  Mutex guards the data  │◄──── Thread 2
                 │  Arc enables shared     │
                 │  ownership across       │◄──── Thread 3
                 │  thread boundaries      │
                 └─────────────────────────┘

   Thread wants to access data:
     1. Call .lock() → blocks if locked
     2. Receive MutexGuard<Data>
     3. Access/modify through guard
     4. Drop guard (end of scope or explicit) → lock released
```

---

## 50. Send and Sync — Concurrency Safety Traits

These are **marker traits** — no methods, just a compile-time tag.

```
SEND:   A type T is Send if it is safe to TRANSFER ownership to another thread.
        Most types are Send. Exceptions:
          - Rc<T> (non-atomic reference count — use Arc<T>)
          - Raw pointers (unknown safety)
          - MutexGuard (must be unlocked by same thread... actually it is Send)
          
SYNC:   A type T is Sync if it is safe to SHARE a reference &T with another thread.
        T is Sync if and only if &T is Send.
        Most types are Sync. Exceptions:
          - Cell<T>, RefCell<T> (no synchronization — use Mutex<T>)
          - Rc<T>
```

```rust
// These are automatically implemented if all fields are Send/Sync:
struct MyData {
    x: i32,     // Send + Sync
    s: String,  // Send + Sync
}
// MyData is automatically Send + Sync

// Opt out with PhantomData:
use std::marker::PhantomData;
struct NotSend {
    _marker: PhantomData<*const ()>,  // raw pointer makes it !Send
}
```

---

## 51. Atomic Types — Lock-Free Primitives

For simple shared counters/flags, atomics are more efficient than Mutex:

```rust
use std::sync::atomic::{AtomicI32, Ordering};
use std::sync::Arc;

let counter = Arc::new(AtomicI32::new(0));

let c = Arc::clone(&counter);
std::thread::spawn(move || {
    c.fetch_add(1, Ordering::SeqCst);
});

println!("{}", counter.load(Ordering::SeqCst));
```

### Memory Ordering

```
Ordering controls CPU/compiler memory reordering:

Relaxed:    No sync guarantees. Only atomicity.
            Use for: independent counters that don't guard other data

Acquire:    This load "acquires" — sees all writes before the
            corresponding Release operation.
            Use for: reading a flag that guards other data

Release:    This store "releases" — all previous writes are visible
            to threads that do an Acquire load of this value.
            Use for: writing data then setting a flag

AcqRel:     Combined Acquire + Release. For operations that both
            load and store (e.g., compare_exchange).

SeqCst:     Sequentially consistent — strongest guarantee.
            Total global ordering of all SeqCst operations.
            Use for: simplicity when performance not critical
```

---

# PART IX — ASYNC/AWAIT

---

## 53. The Future Trait — Deferred Computation

A `Future` represents a computation that **might not have completed yet**.

```rust
pub trait Future {
    type Output;
    fn poll(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Self::Output>;
}

pub enum Poll<T> {
    Ready(T),    // computation complete, here's the result
    Pending,     // not done yet, executor will be notified when ready
}
```

```
FUTURE STATE MACHINE:

┌──────────────┐           poll()          ┌──────────────┐
│   PENDING    │ ─────────────────────────► │    READY     │
│  (not done)  │ ◄───────────────────────── │   (done!)    │
└──────────────┘     Pending returned       └──────────────┘

When Pending is returned:
  - Future registers a "waker" with the runtime
  - Executor parks this task
  - When data arrives (I/O, timer, etc.) waker wakes the task
  - Executor polls again
```

---

## 54. async/await Syntax

`async` and `await` are syntactic sugar over the `Future` machinery.

```rust
// async fn returns an impl Future<Output = T>
async fn fetch_data(url: &str) -> Result<String, reqwest::Error> {
    let response = reqwest::get(url).await?;  // .await suspends here
    let text = response.text().await?;         // suspends again
    Ok(text)
}
```

### What `async fn` Expands To

```rust
// This:
async fn add(a: i32, b: i32) -> i32 {
    a + b
}

// Is roughly equivalent to:
fn add(a: i32, b: i32) -> impl Future<Output = i32> {
    // Returns a state machine struct:
    AddFuture { a, b, state: AddState::Start }
}

// The compiler generates an anonymous state machine:
struct AddFuture { a: i32, b: i32, state: AddState }
enum AddState { Start }

impl Future for AddFuture {
    type Output = i32;
    fn poll(mut self: Pin<&mut Self>, _cx: &mut Context) -> Poll<i32> {
        Poll::Ready(self.a + self.b)  // simple case, no suspension
    }
}
```

### Async State Machine (with suspension points)

```rust
async fn two_awaits() -> i32 {
    let a = do_something().await;  // suspension point 1
    let b = do_other().await;      // suspension point 2
    a + b
}
```

```
Generated state machine:

enum TwoAwaitsState {
    Start,
    WaitingForA { /* future A */ },
    WaitingForB { a: i32, /* future B */ },
    Done,
}

poll() transitions:
  Start → (start future A) → WaitingForA
  WaitingForA → (A ready) → WaitingForB { a }
  WaitingForB → (B ready) → Done
  Done → (unreachable, Poll::Ready returned)
```

---

## 55. Executors and Runtimes

A `Future` is a lazy description of work. An **executor** is what actually drives futures to completion by calling `poll()`.

```
ASYNC EXECUTION ARCHITECTURE:

┌─────────────────────────────────────────────────────────┐
│                    YOUR CODE                            │
│   async fn main() { ... }                               │
└────────────────────────┬────────────────────────────────┘
                         │
                         ▼ tokio::main or async_std::main
┌─────────────────────────────────────────────────────────┐
│                  ASYNC RUNTIME                          │
│                                                         │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │
│  │  Task Queue │    │  Reactor    │    │  Thread Pool│ │
│  │  (futures   │    │  (epoll/    │    │  (work      │ │
│  │  waiting to │    │  kqueue/    │    │  stealing   │ │
│  │  be polled) │    │  IOCP)      │    │  executor)  │ │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘ │
│         │                  │                  │        │
│         └──────────────────┴──────────────────┘        │
│                         │                               │
│                   OS KERNEL I/O                         │
└─────────────────────────────────────────────────────────┘
```

---

## 56. tokio — The Dominant Async Runtime

```toml
# Cargo.toml
[dependencies]
tokio = { version = "1", features = ["full"] }
```

```rust
#[tokio::main]  // expands to: runtime.block_on(main())
async fn main() {
    // Spawn concurrent tasks:
    let h1 = tokio::spawn(async { do_work_1().await });
    let h2 = tokio::spawn(async { do_work_2().await });
    
    let (r1, r2) = tokio::join!(h1, h2);  // await both concurrently
    
    // Timeout:
    let result = tokio::time::timeout(
        std::time::Duration::from_secs(5),
        expensive_operation()
    ).await;
    
    // Channel:
    let (tx, mut rx) = tokio::sync::mpsc::channel(32);
    tokio::spawn(async move { tx.send("hello").await.unwrap(); });
    println!("{}", rx.recv().await.unwrap());
}
```

---

## 57. Async Traits

Traits with async methods require extra handling because `impl Trait` in trait positions is complex:

```rust
// As of Rust 1.75+, async fn in traits is stable:
trait AsyncService {
    async fn process(&self, input: &str) -> String;
}

// For older Rust, use the `async-trait` crate:
use async_trait::async_trait;

#[async_trait]
trait AsyncServiceOld {
    async fn process(&self, input: &str) -> String;
}
```

---

## 58. Pin and Unpin in Async Context

Why `Pin`? Async state machines may be **self-referential** — they contain references to their own fields. If moved, those references would be dangling.

```
SELF-REFERENTIAL FUTURE:

struct MyFuture {
    data: String,
    ptr: *const String,  // points to self.data!
}

If MyFuture moves in memory, self.data moves, but ptr still
points to the old location → dangling pointer!

Pin<P> prevents the value from moving, making self-references safe.
```

```rust
// Unpin: type is safe to move even when pinned (most types)
// !Unpin: type MUST NOT be moved after pinning (async state machines)

// pin a future:
let future = async { 42 };
tokio::pin!(future);  // future is now Pin<&mut impl Future>
```

---

# PART X — MODULES AND CRATES

---

## 60. The Module System

```
CRATE: The root compilation unit (a binary or library)
MODULE: A namespace within a crate
PATH: How you refer to items in modules
```

```
PROJECT STRUCTURE:
my_project/
├── Cargo.toml          ← package manifest
└── src/
    ├── main.rs         ← binary crate root (fn main)
    ├── lib.rs          ← library crate root (optional)
    ├── utils.rs        ← module (file = mod utils)
    └── network/        ← module directory
        ├── mod.rs      ← module root for 'network'
        ├── http.rs     ← submodule (mod http inside network)
        └── tcp.rs      ← submodule (mod tcp inside network)
```

```rust
// In main.rs or lib.rs:
mod utils;           // file-based module: src/utils.rs
mod network;         // directory-based module: src/network/mod.rs

// Inline module:
mod config {
    pub struct Config {
        pub debug: bool,
    }
    
    pub fn default() -> Config {
        Config { debug: false }
    }
}
```

---

## 61. Visibility Rules

```
VISIBILITY SPECIFIERS:

  pub             → public to all (anyone can use this)
  pub(crate)      → public within this crate only
  pub(super)      → public to parent module
  pub(in path)    → public to specific module path
  (none)          → private — only accessible within same module
                    and its descendants
```

```rust
mod outer {
    pub fn public_fn() {}           // visible anywhere
    pub(crate) fn crate_fn() {}    // visible in this crate
    pub(super) fn parent_fn() {}   // visible in parent module
    fn private_fn() {}             // visible only in `outer` and children

    mod inner {
        fn can_call_private() {
            super::private_fn();  // child can access parent's private
        }
    }
}
```

---

## 62. `use` Declarations and Paths

```rust
// Absolute path (from crate root):
use std::collections::HashMap;
use crate::network::http::Client;

// Relative path:
use self::utils::parse;
use super::config::Config;

// Multiple items:
use std::io::{Read, Write, BufReader};

// Rename with as:
use std::io::Error as IoError;
use std::collections::HashMap as Map;

// Glob import (use sparingly):
use std::io::prelude::*;

// Re-exporting:
pub use self::utils::parse;  // makes parse accessible from this module's API
```

### Paths in Code

```rust
// Full paths (no use declaration needed):
let mut map = std::collections::HashMap::new();

// After use:
use std::collections::HashMap;
let mut map = HashMap::new();

// Nested path syntax (batch imports):
use std::{
    collections::{HashMap, HashSet},
    io::{self, Write},  // self imports the module itself
    fmt,
};
```

---

## 63. Crates — Compilation Units

```
BINARY CRATE: has fn main(), produces executable
LIBRARY CRATE: no fn main(), produces .rlib for use by others

ONE PACKAGE can contain:
  - One library crate (src/lib.rs)
  - Multiple binary crates (src/main.rs, src/bin/tool1.rs, ...)
  - Test crates (tests/ directory)
  - Benchmark crates (benches/ directory)
```

---

## 64. Cargo — The Build System and Package Manager

```toml
# Cargo.toml — package manifest
[package]
name = "my-app"
version = "0.1.0"
edition = "2021"      # Rust edition (2015, 2018, 2021)
authors = ["You <you@example.com>"]
description = "My application"
license = "MIT"

[dependencies]
serde = { version = "1.0", features = ["derive"] }
tokio = { version = "1", features = ["full"] }
anyhow = "1.0"

[dev-dependencies]     # only for tests/benchmarks
tempfile = "3"

[build-dependencies]   # only for build scripts (build.rs)
cc = "1"

[features]
default = ["feature1"]
feature1 = []
feature2 = ["dep/feature"]

[profile.release]
opt-level = 3
lto = true
codegen-units = 1
```

### Key Cargo Commands

```
cargo new <name>          # create new package
cargo new --lib <name>    # create library
cargo build               # build debug
cargo build --release     # build optimized
cargo run                 # build and run
cargo test                # run all tests
cargo bench               # run benchmarks
cargo doc --open          # build and open docs
cargo fmt                 # format code (rustfmt)
cargo clippy              # lint (clippy)
cargo add <crate>         # add dependency
cargo remove <crate>      # remove dependency
cargo update              # update Cargo.lock
cargo publish             # publish to crates.io
cargo audit               # check for security vulnerabilities
```

---

## 65. Workspaces — Multi-Crate Projects

```toml
# Root Cargo.toml
[workspace]
members = [
    "api-server",
    "database",
    "common",
    "cli",
]
resolver = "2"
```

```
workspace/
├── Cargo.toml       ← workspace manifest
├── Cargo.lock       ← shared lock file (single version of each dep)
├── api-server/
│   ├── Cargo.toml
│   └── src/main.rs
├── database/
│   ├── Cargo.toml
│   └── src/lib.rs
├── common/
│   ├── Cargo.toml
│   └── src/lib.rs
└── cli/
    ├── Cargo.toml
    └── src/main.rs
```

---

## 66. Features — Conditional Compilation

```rust
// In Cargo.toml:
[features]
json = ["serde_json"]
async = ["tokio"]

// In code:
#[cfg(feature = "json")]
pub fn parse_json(s: &str) -> serde_json::Value { ... }

#[cfg(not(feature = "async"))]
pub fn sync_fetch(url: &str) -> String { ... }

// cfg on modules:
#[cfg(target_os = "windows")]
mod windows_impl;

#[cfg(target_os = "linux")]
mod linux_impl;
```

---

# PART XI — MACROS

---

## 67. Why Macros Exist

Macros provide **metaprogramming** — code that writes code. They solve problems that functions cannot:

```
FUNCTIONS:      - Fixed number of arguments
                - Arguments are evaluated (types must match)
                - Cannot generate new code or types

MACROS:         - Variadic (any number of arguments): println!("{} {} {}", a, b, c)
                - Arguments are NOT pre-evaluated (lazy)
                - Can generate arbitrary code
                - Can work with syntax, not just values
                - Can implement traits automatically: #[derive(Debug)]
```

---

## 68. Declarative Macros — macro_rules!

```rust
// Define a simple macro:
macro_rules! say_hello {
    () => {
        println!("Hello!");
    };
    ($name:expr) => {
        println!("Hello, {}!", $name);
    };
}

say_hello!();           // Hello!
say_hello!("World");    // Hello, World!
```

### Pattern Variables (Metavariables)

```
METAVARIABLE TYPES:
  expr    → any expression
  ident   → an identifier (variable/function name)
  ty      → a type
  pat     → a pattern
  stmt    → a statement
  block   → a block { }
  item    → an item (fn, struct, impl, etc.)
  meta    → attribute content
  tt      → a single token tree (very general)
  literal → a literal value
  path    → a path (std::io::Error)
  vis     → visibility (pub, pub(crate))
  lifetime → a lifetime ('a)
```

### Repetition in Macros

```rust
// Create a vector from any number of elements:
macro_rules! my_vec {
    ( $( $x:expr ),* ) => {
        {
            let mut v = Vec::new();
            $( v.push($x); )*  // repeat for each element
            v
        }
    };
}

let v = my_vec![1, 2, 3, 4];  // expands to push each element

// HashMap macro:
macro_rules! hashmap {
    ( $($key:expr => $val:expr),* ) => {
        {
            let mut m = ::std::collections::HashMap::new();
            $( m.insert($key, $val); )*
            m
        }
    };
}

let m = hashmap!["one" => 1, "two" => 2, "three" => 3];
```

---

## 69. Procedural Macros

Procedural macros receive a token stream and output a token stream. They are Rust code that runs at compile time.

### Three Kinds

```
1. DERIVE MACROS: #[derive(MyTrait)]
   Generate trait implementations

2. ATTRIBUTE MACROS: #[my_attribute]
   Transform the item they're attached to

3. FUNCTION-LIKE MACROS: my_macro!(...)
   Look like declarative macros but are more powerful
```

### Custom Derive Example (using proc-macro2 and quote)

```rust
// In a proc-macro crate:
use proc_macro::TokenStream;
use quote::quote;
use syn;

#[proc_macro_derive(HelloMacro)]
pub fn hello_macro_derive(input: TokenStream) -> TokenStream {
    let ast: syn::DeriveInput = syn::parse(input).unwrap();
    let name = &ast.ident;

    let gen = quote! {
        impl HelloMacro for #name {
            fn hello_macro() {
                println!("Hello! My name is {}!", stringify!(#name));
            }
        }
    };
    gen.into()
}

// Usage:
#[derive(HelloMacro)]
struct Pancakes;

Pancakes::hello_macro(); // "Hello! My name is Pancakes!"
```

---

## 70. Common Built-in Macros

```rust
// Output:
println!("value: {}", x);    // print with newline
print!("no newline");         // print without newline
eprintln!("error: {}", e);   // print to stderr

// Formatting:
let s = format!("{:?}", val);    // format to String
let s = format!("{:>10}", "hi"); // right-aligned in 10 chars
let s = format!("{:.2}", 3.14159); // 2 decimal places: "3.14"

// Panicking:
panic!("fatal error");
assert!(condition, "message if false");
assert_eq!(left, right, "message");
assert_ne!(a, b);
debug_assert!(condition);  // only in debug builds

// Compile-time:
todo!()         // marks unimplemented code, always panics
unimplemented!() // marks intentionally unimplemented, panics
unreachable!()   // marks code path that shouldn't be reached

// File/line info:
file!()      // current file name
line!()      // current line number
column!()    // current column
env!("VAR")  // value of environment variable at compile time
option_env!("VAR") // Option<&str> — None if not set

// Type/size inspection:
std::mem::size_of::<T>()  // size of type in bytes
std::mem::align_of::<T>() // alignment of type in bytes

// Concatenation:
concat!("hello", " ", "world")  // "hello world" (compile-time)
concat_idents!(foo, bar)         // foobar (identifier concatenation)

// Include files:
include_str!("file.txt")        // &'static str from file
include_bytes!("file.bin")      // &'static [u8] from file
include!(concat!(env!("OUT_DIR"), "/generated.rs")) // include generated code
```

---

# PART XII — UNSAFE RUST

---

## 71. What `unsafe` Unlocks

`unsafe` is not "unsafe code" — it is code where **you take on the responsibility** of maintaining invariants that the compiler cannot verify.

```
WHAT unsafe ALLOWS:
  1. Dereference raw pointers (*const T, *mut T)
  2. Call unsafe functions and methods
  3. Access or modify mutable static variables
  4. Implement unsafe traits
  5. Access union fields

WHAT unsafe DOES NOT ALLOW:
  - Ignore the borrow checker (it still checks ownership)
  - Create undefined behavior for free (you must still be correct)
  - Bypass type checking
  - Do anything safe code can't, if the invariants hold
```

```
SAFE RUST                    UNSAFE RUST
┌──────────────┐             ┌──────────────────────────┐
│ Borrow       │             │ All safe rules PLUS:      │
│ checker      │             │ - Raw pointer derefs      │
│ enforces     │             │ - FFI calls               │
│ rules        │             │ - Mutable statics         │
│ automatically│             │ - Union field access      │
│              │             │                          │
│ You: write   │             │ You: manually verify     │
│ code         │             │ safety invariants        │
└──────────────┘             └──────────────────────────┘
```

---

## 72. Raw Pointers — *const T and *mut T

```rust
let mut x = 5i32;

// Creating raw pointers (SAFE — no dereferencing yet):
let r1: *const i32 = &x;      // raw immutable pointer
let r2: *mut i32 = &mut x;    // raw mutable pointer

// Raw pointers:
// - CAN be null
// - Have no lifetime tracking
// - Can alias (two *mut can point to same location)
// - Must be dereferenced inside unsafe block

unsafe {
    println!("{}", *r1);   // dereference — UNSAFE operation
    *r2 = 10;              // write through raw pointer — UNSAFE
    println!("{}", *r1);   // 10
}

// Creating from raw address (highly unsafe):
let address = 0x012345usize;
let ptr = address as *const i32;
// unsafe { let val = *ptr; }  // would likely segfault — just for illustration

// offset/add/sub for pointer arithmetic:
let arr = [1i32, 2, 3, 4, 5];
let ptr = arr.as_ptr();
unsafe {
    let third = ptr.add(2);  // ptr + 2 * sizeof(i32)
    println!("{}", *third);  // 3
}
```

---

## 73. Unsafe Functions and Blocks

```rust
// Declaring an unsafe function:
unsafe fn dangerous() {
    // This function is unsafe to call
    // Callers must ensure preconditions
}

// Calling unsafe code requires an unsafe block:
unsafe {
    dangerous();
}

// Creating a safe abstraction over unsafe code:
pub fn split_at_mut(slice: &mut [i32], mid: usize) -> (&mut [i32], &mut [i32]) {
    let len = slice.len();
    assert!(mid <= len);
    let ptr = slice.as_mut_ptr();
    
    unsafe {
        (
            std::slice::from_raw_parts_mut(ptr, mid),
            std::slice::from_raw_parts_mut(ptr.add(mid), len - mid),
        )
    }
    // The unsafe is justified: mid <= len ensures no overlap
    // The outer function is safe to call
}
```

---

## 74. FFI — Calling C from Rust

```rust
extern "C" {
    fn abs(input: i32) -> i32;  // declare C function
    fn strlen(s: *const u8) -> usize;
}

unsafe {
    println!("{}", abs(-3));  // 3
}

// Using libc crate for C types:
use std::os::raw::c_int;

extern "C" {
    fn qsort(
        base: *mut std::ffi::c_void,
        nmemb: usize,
        size: usize,
        compar: Option<unsafe extern "C" fn(*const std::ffi::c_void, *const std::ffi::c_void) -> c_int>
    );
}

// Calling Rust from C:
#[no_mangle]  // preserve function name for C linker
pub extern "C" fn call_from_c() -> i32 {
    42
}
```

---

## 75. Unsafe Traits

```rust
// Declaring an unsafe trait:
unsafe trait GloballyUnique {
    fn id() -> u64;
    // Contract: Each impl type must have a truly unique ID
    // The compiler cannot verify this — implementor must ensure
}

// Implementing an unsafe trait (must use unsafe impl):
unsafe impl GloballyUnique for MyType {
    fn id() -> u64 { 42 }
}

// The classic unsafe traits:
// unsafe impl Send for MyType { }  // I guarantee MyType is thread-safe to send
// unsafe impl Sync for MyType { }  // I guarantee &MyType is thread-safe to share
```

---

# PART XIII — ADVANCED TYPE SYSTEM

---

## 77. Newtype Pattern

Wrap a type in a single-field tuple struct to create a **distinct type** with the same underlying representation.

```rust
struct Meters(f64);
struct Kilograms(f64);

// Now you can't accidentally add Meters and Kilograms:
fn add_distance(a: Meters, b: Meters) -> Meters {
    Meters(a.0 + b.0)
}

let m = Meters(5.0);
let kg = Kilograms(70.0);
// add_distance(m, kg); // ERROR! type mismatch — Meters vs Kilograms

// Also useful for implementing foreign traits on foreign types:
struct Wrapper(Vec<String>);
impl std::fmt::Display for Wrapper {  // can't impl Display for Vec<String> directly
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "[{}]", self.0.join(", "))
    }
}
```

---

## 78. Type Aliases

```rust
// Type alias — same type, different name
type Kilometers = i32;  // Kilometers IS i32, not a distinct type

let x: i32 = 5;
let y: Kilometers = x;  // WORKS — same type

// Useful for long generic types:
type Result<T> = std::result::Result<T, std::io::Error>;
type Thunk = Box<dyn Fn() -> String>;

// With generics:
type Point<T> = (T, T, T);
let p: Point<f64> = (1.0, 2.0, 3.0);
```

---

## 79. PhantomData<T>

`PhantomData<T>` is a zero-sized type that tells the compiler a type "logically uses" `T` without storing it.

```rust
use std::marker::PhantomData;

struct TypedId<T> {
    value: u64,
    _phantom: PhantomData<T>,  // tells compiler this is "for type T"
}

struct User;
struct Product;

type UserId = TypedId<User>;
type ProductId = TypedId<Product>;

let user_id = TypedId::<User> { value: 1, _phantom: PhantomData };
let product_id = TypedId::<Product> { value: 1, _phantom: PhantomData };

// fn find_user(id: UserId)  — won't accept ProductId!
// Zero cost — PhantomData is zero bytes
```

### Uses of PhantomData

```
1. VARIANCE CONTROL: PhantomData<T> makes the struct covariant in T
                     PhantomData<fn(T)> makes it contravariant
                     PhantomData<fn(T) -> T> makes it invariant

2. LIFETIME BINDING: struct StrSplit<'a> { _phantom: PhantomData<&'a str> }
                     Tells compiler this struct borrows data of lifetime 'a

3. SEND/SYNC CONTROL: PhantomData<*const ()> makes the type !Send + !Sync

4. TYPED IDs: As shown above — distinct types for same underlying value
```

---

## 80. Zero-Sized Types (ZSTs)

Types with no fields take 0 bytes in memory but are still valid types:

```rust
struct Marker;           // 0 bytes
struct Tag<T>(PhantomData<T>); // 0 bytes (PhantomData is ZST)
enum Never {}            // 0 bytes, no valid values
```

```
ZST PROPERTIES:
  - size_of::<ZST>() == 0
  - align_of::<ZST>() == 1
  - Can be created (if not uninhabited)
  - Can have methods
  - Vec<ZST> is free — no allocation needed (internally stores just length)
  - &ZST has a non-null address (but accessing memory at it is undefined)
```

---

## 81. Never Type (`!`)

`!` is the **bottom type** — it has no values. Functions that never return have type `!`.

```rust
fn diverge() -> ! {
    panic!("This function never returns")
}

fn infinite() -> ! {
    loop {}
}

// ! coerces to any type:
let x: i32 = {
    if true { 5 } else { panic!() }  // panic! has type !, coerces to i32
};

// break, continue, return also have type !:
let y: i32 = loop {
    break 42;  // break <value> makes loop expression return 42
};
```

---

## 82. `impl Trait` — Opaque Return Types

`impl Trait` in return position creates an **opaque type** — the caller knows the type implements the trait, but not what it is.

```rust
// impl Trait in return position:
fn make_adder(x: i32) -> impl Fn(i32) -> i32 {
    move |y| x + y  // returns a closure — exact type hidden
}

let add5 = make_adder(5);
println!("{}", add5(3));  // 8

// Each call to make_adder returns the SAME concrete type
// (unlike Box<dyn Trait> which can be different types)

// In trait position (RPIT = Return Position Impl Trait):
fn evens() -> impl Iterator<Item = i32> {
    (0..).filter(|x| x % 2 == 0)
}
```

### impl Trait vs dyn Trait

```
impl Trait:
  - Static dispatch (monomorphized)
  - No heap allocation
  - All returned values must be SAME concrete type
  - Works in function parameters and return types

dyn Trait:
  - Dynamic dispatch (vtable)
  - Usually heap-allocated (Box<dyn Trait>)
  - Can be DIFFERENT concrete types (heterogeneous)
  - Only works with object-safe traits
```

---

## 83. Const Generics

Generics over constant values (integers, booleans, chars):

```rust
// Generic over array SIZE — a constant:
struct Matrix<const ROWS: usize, const COLS: usize> {
    data: [[f64; COLS]; ROWS],
}

impl<const R: usize, const C: usize> Matrix<R, C> {
    fn new() -> Self {
        Matrix { data: [[0.0; C]; R] }
    }
    
    fn rows(&self) -> usize { R }
    fn cols(&self) -> usize { C }
}

let m: Matrix<3, 4> = Matrix::new();
println!("{}x{}", m.rows(), m.cols());  // 3x4

// std uses this: [T; N] is generic over const N: usize
```

---

# PART XIV — MEMORY LAYOUT AND INTERNALS

---

## 85. How Rust Lays Out Types in Memory

### Primitive Layout

```
Type         Size    Alignment   Layout
─────────────────────────────────────────────────────────
bool         1       1           0 or 1
u8/i8        1       1           two's complement
u16/i16      2       2           little-endian on x86
u32/i32      4       4           
u64/i64      8       8           
u128/i128    16      16          
usize/isize  8       8           (on 64-bit arch)
f32          4       4           IEEE 754 single
f64          8       8           IEEE 754 double
char         4       4           Unicode scalar value
()           0       1           zero-sized
&T           8       8           pointer (on 64-bit)
&[T]         16      8           fat pointer: (ptr, len)
&str         16      8           fat pointer: (ptr, len)
Box<T>       8       8           pointer
Option<&T>   8       8           null pointer optimization!
```

### Struct Layout Rules (Default)

```rust
struct Example {
    a: u8,   // 1 byte
    b: u32,  // 4 bytes
    c: u8,   // 1 byte
}

// Rust may reorder fields for optimal packing.
// Default repr is "Rust" repr — layout is NOT guaranteed.

// Actual memory (may be reordered by compiler):
// ┌──────┬──────┬──────┬──────┬──────┬──────┐
// │ b(4B)│ a(1B)│ c(1B)│  pad │  pad │      │
// └──────┴──────┴──────┴──────┴──────┴──────┘
// Size: 8 bytes (4 + 1 + 1 + 2 padding for 4-byte alignment)

// Or possibly:
// ┌──────┬──────┬──────┬──────┬──────┬──────┐
// │ b(4B)│ a(1B)│ c(1B)│  pad │      │      │
// └──────┴──────┴──────┴──────┴──────┴──────┘
```

### Null Pointer Optimization

Option is special — for reference types, `Option<&T>` is the same size as `&T`:

```rust
assert_eq!(std::mem::size_of::<&i32>(), 8);
assert_eq!(std::mem::size_of::<Option<&i32>>(), 8);  // NO extra byte!
// None is represented as the null pointer (0x0)
// Some(&val) is represented as the non-null pointer
```

---

## 86. repr Attributes

Control the memory layout of types:

```rust
// Default Rust layout (may reorder fields, no guarantees):
struct RustLayout { x: u8, y: u32, z: u16 }

// C-compatible layout (fields in declaration order, C padding rules):
#[repr(C)]
struct CLayout { x: u8, y: u32, z: u16 }
// Memory: [x:1][pad:3][y:4][z:2][pad:2] = 12 bytes

// Packed — no padding (may be unaligned, unsafe to take references):
#[repr(packed)]
struct Packed { x: u8, y: u32, z: u16 }
// Memory: [x:1][y:4][z:2] = 7 bytes (but unaligned access!)

// Specific size for enum discriminant:
#[repr(u8)]
enum SmallEnum { A, B, C }  // discriminant stored as u8

// Transparent — same layout as single non-ZST field:
#[repr(transparent)]
struct Wrapper(i32);  // identical layout to i32
// Required for safe FFI newtype wrappers

// Alignment override:
#[repr(align(64))]
struct CacheLine { data: [u8; 64] }  // 64-byte aligned (cache-friendly)
```

---

## 89. The Drop Trait — Deterministic Cleanup

```rust
struct Resource {
    name: String,
}

impl Drop for Resource {
    fn drop(&mut self) {
        println!("Dropping resource: {}", self.name);
        // Custom cleanup: close file, release lock, free memory, etc.
    }
}

{
    let r1 = Resource { name: "first".to_string() };
    let r2 = Resource { name: "second".to_string() };
    println!("Using resources");
}
// Prints:
// "Using resources"
// "Dropping resource: second"   ← dropped LAST declared first
// "Dropping resource: first"    ← dropped in reverse order

// You CANNOT call drop() on a type manually (double-free risk):
// r1.drop();  // ERROR: explicit use of Drop::drop
// Instead:
drop(r1);  // std::mem::drop — takes ownership and calls drop
```

### Drop Order

```
DROP ORDER:
  1. Variables drop in REVERSE order of declaration (LIFO)
  2. Struct fields drop in DECLARATION order
  3. Tuple elements drop in order (first to last)
  4. Array elements drop in order (index 0 to N-1)
  5. Temporary values drop at end of statement (usually)
```

---

# PART XV — STRINGS AND COLLECTIONS

---

## 91. String vs &str — The Two String Types

```
┌─────────────────────────────────────────────────────────┐
│          THE TWO STRING TYPES                           │
├───────────────────┬─────────────────────────────────────┤
│  &str             │  String                             │
├───────────────────┼─────────────────────────────────────┤
│  String slice     │  Owned string                       │
│  (borrowed view)  │                                     │
├───────────────────┼─────────────────────────────────────┤
│  Fat pointer:     │  Stack:                             │
│  (ptr, len)       │  (ptr, len, capacity)               │
│  16 bytes         │  24 bytes                           │
├───────────────────┼─────────────────────────────────────┤
│  No ownership     │  Owns heap buffer                   │
│  Cannot mutate    │  Can grow/shrink                    │
│  (usually)        │                                     │
├───────────────────┼─────────────────────────────────────┤
│  String literals  │  Created with:                      │
│  are &'static str │  String::from("...")                 │
│                   │  "...".to_string()                  │
│                   │  format!("{}", ...)                  │
└───────────────────┴─────────────────────────────────────┘
```

### String Operations

```rust
// Creation:
let s1 = String::from("hello");
let s2 = "world".to_string();
let s3 = "hello".to_owned();

// Appending:
let mut s = String::from("hello");
s.push_str(", world");   // append &str
s.push('!');              // append char

// Concatenation:
let s1 = String::from("hello ");
let s2 = String::from("world");
let s3 = s1 + &s2;  // s1 is MOVED, s2 is borrowed
// s3 = "hello world", s1 is invalid, s2 still valid

// Format (doesn't move):
let s1 = String::from("hello");
let s2 = String::from("world");
let s3 = format!("{} {}", s1, s2);  // both s1, s2 still valid

// Slicing:
let s = String::from("hello");
let slice: &str = &s[1..3];  // "el" — byte indices, NOT char indices!
// Slicing in middle of multi-byte char panics!

// UTF-8 awareness:
let s = String::from("नमस्ते"); // Devanagari (multi-byte)
// s.chars():  iterates over Unicode chars
// s.bytes():  iterates over raw bytes
// s.len():    number of BYTES (not chars)
// s.chars().count(): number of Unicode scalar values

// Contains, starts_with, ends_with:
s.contains("hello");
s.starts_with("he");
s.ends_with("lo");

// Split and collect:
let words: Vec<&str> = "hello world foo".split_whitespace().collect();
let parts: Vec<&str> = "a,b,c".split(',').collect();

// Trim:
"  hello  ".trim();         // "hello"
"  hello  ".trim_start();   // "hello  "
"  hello  ".trim_end();     // "  hello"

// Replace:
"hello world".replace("world", "Rust");  // "hello Rust"

// Parse (String → T):
let n: i32 = "42".parse().unwrap();
let f: f64 = "3.14".parse().unwrap();
```

---

## 92. Vec<T> — The Dynamic Array

```
Vec<T> INTERNALS:

STACK                               HEAP
┌──────────────────────┐            ┌────────────────────────────┐
│ Vec<i32>             │            │                            │
│  ptr: ───────────────┼────────────►  [1] [2] [3] [_] [_]     │
│  len: 3              │            │   ^-- len --^  ^- unused-^ │
│  cap: 5              │            │   ^----------- cap ------^ │
└──────────────────────┘            └────────────────────────────┘

len: number of elements currently in the Vec
cap: total capacity allocated (may be > len)
     When len == cap and you push, Vec reallocates (typically 2x growth)
```

```rust
// Creation:
let mut v: Vec<i32> = Vec::new();
let mut v = Vec::with_capacity(10);  // allocate for 10 elements upfront
let v = vec![1, 2, 3, 4, 5];        // macro
let v = vec![0; 100];                // 100 zeros

// Adding:
v.push(6);            // append to end O(1) amortized
v.insert(0, 99);      // insert at index O(n) — shifts elements
v.extend([7, 8, 9]);  // extend from iterator

// Removing:
let last = v.pop();          // remove and return last O(1)
let item = v.remove(0);      // remove at index O(n) — shifts
let item = v.swap_remove(0); // remove at index O(1) — swaps with last

// Accessing:
let first = v[0];         // panics if out of bounds
let first = v.get(0);     // Option<&T> — safe

// Slices:
let slice: &[i32] = &v;
let partial: &[i32] = &v[1..3];

// Iteration:
for x in &v { println!("{}", x); }          // immutable borrow
for x in &mut v { *x *= 2; }               // mutable borrow
for x in v.into_iter() { println!("{}", x); } // consume

// Sorting:
v.sort();                                    // requires Ord
v.sort_by(|a, b| a.cmp(b));               // custom comparator
v.sort_by_key(|x| x.abs());               // sort by key

// Deduplication (must be sorted first):
v.sort(); v.dedup();

// Capacity management:
v.shrink_to_fit();   // release excess capacity
v.reserve(20);       // ensure at least 20 more elements can fit
```

---

## 93. HashMap and BTreeMap

```rust
use std::collections::HashMap;

let mut scores: HashMap<String, i32> = HashMap::new();

// Inserting:
scores.insert("Alice".to_string(), 10);
scores.insert("Bob".to_string(), 20);

// Entry API — efficient insert-or-update:
scores.entry("Alice".to_string()).or_insert(0);  // insert only if absent
scores.entry("Carol".to_string()).or_insert(50); // Carol: 50

// Modify if exists:
let count = scores.entry("Alice".to_string()).or_insert(0);
*count += 5;  // Alice: 15

// Lookup:
scores.get("Alice")        // Option<&i32>
scores["Alice"]            // i32, panics if missing
scores.contains_key("Bob") // bool

// Removal:
scores.remove("Bob");      // Option<i32>

// Iteration:
for (key, val) in &scores { println!("{}: {}", key, val); }

// Building from iterator:
let scores: HashMap<&str, i32> = vec![("Alice", 10), ("Bob", 20)]
    .into_iter().collect();
```

```
HASHMAP vs BTREEMAP:

HashMap<K, V>:
  - Uses a hash function (SipHash by default)
  - O(1) average for insert/lookup/remove
  - No ordering — iteration order is random
  - Best for: most use cases

BTreeMap<K, V>:
  - Uses a B-tree (sorted)
  - O(log n) for insert/lookup/remove
  - Keys are always in sorted order
  - Best for: range queries, ordered iteration, fixed-size keys
  - K must implement Ord

BTreeMap extra operations:
  - range(start..end)  — iterate over a range of keys
  - entry API (same as HashMap)
  - first_key_value() / last_key_value()
```

---

## 96. Slices — Views into Sequences

A **slice** is a dynamically-sized view into a contiguous sequence. Slices don't own data — they borrow it.

```rust
// Slice of array:
let arr = [1, 2, 3, 4, 5];
let slice: &[i32] = &arr[1..4];  // [2, 3, 4]

// Slice of Vec:
let vec = vec![1, 2, 3, 4, 5];
let slice: &[i32] = &vec[..];  // entire vec as slice

// &str IS a slice of a String:
let s = String::from("hello");
let slice: &str = &s[1..3];  // "el"

// Slice fat pointer:
// ┌──────────┬──────────┐
// │  ptr     │  len     │  16 bytes
// └──────────┴──────────┘
//    points     number of
//    to first   elements
//    element

// Slice methods:
slice.len()
slice.is_empty()
slice.first()          // Option<&T>
slice.last()           // Option<&T>
slice.contains(&val)
slice.iter()
slice.chunks(3)        // sub-slices of at most 3 elements
slice.windows(3)       // overlapping sub-slices of 3 elements
slice.split_at(2)      // (&[0..2], &[2..])
```

---

# PART XVI — MENTAL MODELS AND PATTERNS

---

## 97. The Builder Pattern in Rust

The builder pattern constructs complex objects step by step:

```rust
#[derive(Debug)]
struct HttpRequest {
    url: String,
    method: String,
    headers: Vec<(String, String)>,
    body: Option<String>,
    timeout: std::time::Duration,
}

struct HttpRequestBuilder {
    url: String,
    method: String,
    headers: Vec<(String, String)>,
    body: Option<String>,
    timeout: std::time::Duration,
}

impl HttpRequestBuilder {
    fn new(url: impl Into<String>) -> Self {
        HttpRequestBuilder {
            url: url.into(),
            method: "GET".to_string(),
            headers: Vec::new(),
            body: None,
            timeout: std::time::Duration::from_secs(30),
        }
    }

    fn method(mut self, method: impl Into<String>) -> Self {
        self.method = method.into();
        self  // return self for chaining
    }

    fn header(mut self, key: impl Into<String>, val: impl Into<String>) -> Self {
        self.headers.push((key.into(), val.into()));
        self
    }

    fn body(mut self, body: impl Into<String>) -> Self {
        self.body = Some(body.into());
        self
    }

    fn timeout(mut self, secs: u64) -> Self {
        self.timeout = std::time::Duration::from_secs(secs);
        self
    }

    fn build(self) -> HttpRequest {
        HttpRequest {
            url: self.url,
            method: self.method,
            headers: self.headers,
            body: self.body,
            timeout: self.timeout,
        }
    }
}

let req = HttpRequestBuilder::new("https://api.example.com/data")
    .method("POST")
    .header("Content-Type", "application/json")
    .header("Authorization", "Bearer token123")
    .body(r#"{"key": "value"}"#)
    .timeout(10)
    .build();
```

---

## 98. The State Machine Pattern

Rust's enums make state machines safe and explicit:

```rust
enum TrafficLight {
    Red,
    Yellow,
    Green,
}

impl TrafficLight {
    fn next(self) -> TrafficLight {
        match self {
            TrafficLight::Red => TrafficLight::Green,
            TrafficLight::Green => TrafficLight::Yellow,
            TrafficLight::Yellow => TrafficLight::Red,
        }
    }

    fn duration(&self) -> std::time::Duration {
        match self {
            TrafficLight::Red => std::time::Duration::from_secs(60),
            TrafficLight::Yellow => std::time::Duration::from_secs(5),
            TrafficLight::Green => std::time::Duration::from_secs(45),
        }
    }
}
```

---

## 99. The Typestate Pattern

Encode **state in types** — make invalid state transitions compile errors:

```rust
struct Door<State> {
    _state: std::marker::PhantomData<State>,
}

struct Open;
struct Closed;
struct Locked;

impl Door<Closed> {
    fn new() -> Self { Door { _state: std::marker::PhantomData } }
    fn open(self) -> Door<Open> { Door { _state: std::marker::PhantomData } }
    fn lock(self) -> Door<Locked> { Door { _state: std::marker::PhantomData } }
}

impl Door<Open> {
    fn close(self) -> Door<Closed> { Door { _state: std::marker::PhantomData } }
    fn walk_through(&self) { println!("Walking through!"); }
}

impl Door<Locked> {
    fn unlock(self) -> Door<Closed> { Door { _state: std::marker::PhantomData } }
}

let door = Door::<Closed>::new();
// door.walk_through();  // ERROR: Door<Closed> has no method walk_through
// door.open().lock();   // ERROR: Door<Open> has no method lock
let door = door.open();
door.walk_through();  // OK
let door = door.close().lock();
let door = door.unlock().open();
door.walk_through();  // OK again
```

---

## 100. RAII — Resource Acquisition Is Initialization

Rust's ownership model enforces RAII automatically:

```rust
// RAII for a database connection:
struct DbConnection {
    // connection handle...
}

impl DbConnection {
    fn new(url: &str) -> Self {
        println!("Opening connection to {}", url);
        DbConnection { /* ... */ }
    }
}

impl Drop for DbConnection {
    fn drop(&mut self) {
        println!("Closing connection");  // ALWAYS runs, even on panic
    }
}

{
    let conn = DbConnection::new("postgres://localhost/mydb");
    // Use conn...
}  // conn.drop() called automatically here
```

---

## 101. Common Rust Idioms

```rust
// 1. Use ? liberally in functions returning Result
fn read_number(path: &str) -> Result<i32, Box<dyn std::error::Error>> {
    let content = std::fs::read_to_string(path)?;
    let n = content.trim().parse()?;
    Ok(n)
}

// 2. impl Into<T> for flexible arguments:
fn greet(name: impl Into<String>) {
    let name = name.into();
    println!("Hello, {}!", name);
}
greet("Alice");           // &str → String
greet(String::from("Bob")); // String → String

// 3. impl AsRef<T> for even more flexible arguments:
fn count_chars(s: impl AsRef<str>) -> usize {
    s.as_ref().chars().count()
}

// 4. Iterator chains over explicit loops:
let sum_of_squares: i32 = (1..=10).map(|x| x * x).sum();

// 5. Use collect() with explicit type annotation:
let words: Vec<_> = "hello world".split_whitespace().collect();
let chars: String = "hello".chars().map(|c| c.to_uppercase().next().unwrap()).collect();

// 6. Leverage if let chains:
if let Some(user) = find_user(id) {
    if let Ok(email) = parse_email(&user.email) {
        send_notification(email);
    }
}

// 7. Default trait for sensible defaults:
#[derive(Default)]
struct Config {
    debug: bool,     // defaults to false
    max_connections: usize,  // defaults to 0
    name: String,    // defaults to ""
}
let config = Config { debug: true, ..Config::default() };

// 8. Clippy-recommended: use .copied() instead of .map(|x| *x)
let nums = vec![1, 2, 3];
let doubled: Vec<i32> = nums.iter().copied().map(|x| x * 2).collect();

// 9. Avoid unnecessary clones with borrows:
fn print_config(config: &Config) { /* use config by reference */ }

// 10. Return early to reduce nesting:
fn process(value: Option<i32>) -> Option<i32> {
    let v = value?;     // return None early if None
    Some(v * 2)
}
```

---

## 102. Anti-Patterns to Avoid

```rust
// ANTI-PATTERN 1: .unwrap() everywhere in production code
let file = File::open("data.txt").unwrap();  // panic-prone
// Better: handle the error or propagate with ?

// ANTI-PATTERN 2: Cloning to satisfy the borrow checker
fn process(s: String) { ... }
let s = String::from("hello");
process(s.clone()); // unnecessary clone
// Better: redesign to take &str or borrow

// ANTI-PATTERN 3: Large enums with heap data in each variant
// Use Box<LargeVariant> for large variants to keep enum small

// ANTI-PATTERN 4: Using Vec<Box<dyn Trait>> when all types are known
// Use enum instead for exhaustive matching and no heap allocation

// ANTI-PATTERN 5: match on booleans
match condition {
    true => do_a(),
    false => do_b(),
}
// Better: if/else

// ANTI-PATTERN 6: Implementing Drop when you should implement DerefMut
// Only implement Drop for RAII patterns (resources to release)

// ANTI-PATTERN 7: Arc<Mutex<T>> when single-threaded (use Rc<RefCell<T>>)
// Arc's atomic ops have overhead; use Rc when threads aren't involved

// ANTI-PATTERN 8: String manipulation with format! in a loop
let mut result = String::new();
for s in items {
    result = format!("{}{}", result, s);  // allocates each iteration!
}
// Better:
let result: String = items.join("");  // or items.iter().collect()

// ANTI-PATTERN 9: Using index-based loops when iterators work
for i in 0..v.len() { println!("{}", v[i]); }  // bounds check each time
// Better:
for x in &v { println!("{}", x); }
```

---

# FINAL SECTION — THE RUST MENTAL MODEL SUMMARY

---

## Putting It All Together — The Rust Mind

```
THE RUST PROGRAMMER'S MENTAL CHECKLIST:

When you write ANY line of Rust, ask:

1. WHO OWNS this value?
   → Follow the owner from creation to drop

2. IS anyone BORROWING it?
   → Track all &T and &mut T in scope
   → &mut T: exclusive (no other borrows allowed)
   → &T: shared (many allowed, but no &mut T simultaneously)

3. How LONG do the borrows LIVE?
   → A reference cannot outlive what it points to
   → If returning a reference, it must come from an input

4. Does this MOVE or COPY?
   → Primitive types (i32, bool, etc.): Copy
   → Heap-owning types (String, Vec, etc.): Move
   → After a move, the original is invalid

5. What THREAD SAFETY do I need?
   → Single thread: Rc<RefCell<T>>
   → Multi-thread: Arc<Mutex<T>> or Arc<RwLock<T>>
   → Read-only sharing: Arc<T>

6. Is this a RECOVERABLE error or a BUG?
   → Recoverable: Result<T, E>, use ?
   → Bug/invariant violation: panic!, assert!
   → External input: always Result, never panic

7. Do I need DYNAMIC or STATIC dispatch?
   → Known types, performance critical: generics (impl Trait)
   → Unknown types, heterogeneous: trait objects (dyn Trait)

8. Am I being LAZY with iterators?
   → Chains: .map().filter().take() are lazy (no work yet)
   → Consumers: .collect(), .sum(), .for_each() drive computation

9. Do I need UNSAFE?
   → Prefer safe abstractions
   → If unsafe, document the invariants you're upholding
   → Keep unsafe blocks minimal and well-tested
```

```
THE OWNERSHIP FLOW DIAGRAM:

Value Created (Stack or Heap)
        │
        ▼
  Owner Binding
        │
    ┌───┴────────────────────────────┐
    │                                │
    ▼                                ▼
  LEND (borrow)               TRANSFER (move)
    │                                │
    ├── &T ──► read-only             ├── into function
    │          (any number)          ├── into collection
    │                                ├── assign to other var
    └── &mut T ► exclusive          └── return from function
              (one at a time)              │
                    │                     ▼
                    │              New Owner Binding
                    │
                    └── Borrow ends (last use / scope end)
                              │
                              ▼
                         Owner alone again
                              │
                              ▼
                    Scope ends → drop() called
                              │
                              ▼
                       Memory freed
```

---

## The Borrow Checker's View of Every Program

```
For EVERY POINT in the program, the borrow checker maintains:

  For each place (variable/field):
    EXCLUSIVE ACCESS:  place has unique access (can read AND write)
                       No borrows outstanding
    SHARED ACCESS:     place has shared access (can only read)
                       Multiple &T borrows outstanding
    BORROWED OUT:      place is borrowed elsewhere
      ├── SHARED: one or more &T borrows active
      └── EXCLUSIVE: one &mut T borrow active

  Invariant maintained at ALL TIMES:
    A place is NEVER in both SHARED and EXCLUSIVE borrow states.
    A place is NEVER accessed through an expired reference.
    A place is NEVER used after it has been MOVED.
```

---

*This guide covers Rust from first principles through advanced internals.*
*The goal is not to memorize rules, but to internalize the WHY behind each rule.*
*When you understand WHY Rust disallows something, the rule is obvious.*
*When you understand WHY Rust allows something, you use it confidently.*

*Keep this mental model: Rust's compiler is a proof engine.*
*Your job is to write code that the compiler can prove is memory-safe.*
*Every annotation (lifetimes, trait bounds, unsafe) is a hint to the prover.*
