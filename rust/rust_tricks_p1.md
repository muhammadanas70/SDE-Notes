# 20 Rust Tricks: A Complete In-Depth Guide
> Smart techniques for **safer**, **faster**, and more **expressive** code

---

## Table of Contents

1. [Pattern Matching](#1-pattern-matching)
2. [Let-else for Early Return](#2-let-else-for-early-return)
3. [? for Error Propagation](#3--for-error-propagation)
4. [Iterator Adapters](#4-iterator-adapters)
5. [Collect like a Pro](#5-collect-like-a-pro)
6. [Option Combinators](#6-option-combinators)
7. [Result Combinators](#7-result-combinators)
8. [Destructuring in Function Args](#8-destructuring-in-function-args)
9. [Enums with Data](#9-enums-with-data)
10. [Newtype Pattern](#10-newtype-pattern)
11. [Prefer &str over String](#11-prefer-str-over-string)
12. [Use Lifetimes Wisely](#12-use-lifetimes-wisely)
13. [Vec::with_capacity](#13-vecwith_capacity)
14. [HashMap Entry API](#14-hashmap-entry-api)
15. [Cow for Flexible Data](#15-cow-for-flexible-data)
16. [const & static](#16-const--static)
17. [cfg Attributes](#17-cfg-attributes)
18. [tokio::select! for Concurrency](#18-tokioselect-for-concurrency)
19. [Tracing for Observability](#19-tracing-for-observability)
20. [Benchmark with cargo bench](#20-benchmark-with-cargo-bench)

---

## 1. Pattern Matching

### What It Is

Pattern matching in Rust is one of the language's most powerful features. Unlike `switch/case` in other languages, Rust's `match` is:

- **Exhaustive**: the compiler forces you to handle every possible case
- **Structural**: it works recursively on the shape of data, not just equality
- **Binding**: it extracts inner values into named variables while matching
- **Guard-able**: arms can have `if` conditions called guards

It is not merely a cleaner `if/else`. It is the fundamental mechanism by which Rust programs reason about and decompose data.

### Mental Model

Think of pattern matching as a **shape-checker and extractor combined**. You describe the shape of the data you expect, and Rust simultaneously verifies the data fits that shape AND extracts pieces of it into named bindings.

```
Data                     Pattern                 Binding
─────────────────────────────────────────────────────────────────
Some(42)       matches   Some(v)         extracts  v = 42
Ok("hello")    matches   Ok(s)           extracts  s = "hello"
(1, true, 'a') matches   (x, true, c)   extracts  x = 1, c = 'a'
Point { x: 3 } matches   Point { x }    extracts  x = 3
```

### How `match` Evaluates

```
                   match value { ... }
                          |
          ┌───────────────▼───────────────┐
          │    Arm 1: Some(v) if v > 10   │ <── guard condition
          │             => v              │
          └───────────────┬───────────────┘
                          | no match (guard fails or shape wrong)
          ┌───────────────▼───────────────┐
          │    Arm 2: Some(v)             │
          │             => v              │
          └───────────────┬───────────────┘
                          | no match (shape wrong)
          ┌───────────────▼───────────────┐
          │    Arm 3: None                │
          │             => 0              │
          └───────────────┬───────────────┘
                          |
                    (all arms exhausted)
                    COMPILE ERROR if any
                    reachable case is missing
```

### The Infographic Example — Dissected

```rust
fn process(value: Option<i32>) -> i32 {
    match value {
        Some(v) if v > 10 => v,  // guard: shape IS Some(v), AND v > 10
        Some(v)           => v,  // shape IS Some(v), no guard (catches rest)
        None              => 0,  // shape IS None
    }
}

fn main() {
    println!("{}", process(Some(20))); // arm 1: 20
    println!("{}", process(Some(5)));  // arm 2: 5
    println!("{}", process(None));     // arm 3: 0
}
```

### Enum Matching (All Variant Shapes)

```rust
#[derive(Debug)]
enum Message {
    Quit,                          // unit variant
    Move { x: i32, y: i32 },      // struct variant
    Write(String),                 // tuple variant
    ChangeColor(u8, u8, u8),       // multi-field tuple variant
}

fn handle(msg: Message) {
    match msg {
        Message::Quit                    => println!("Quit"),
        Message::Move { x, y }          => println!("Move to ({x}, {y})"),
        Message::Write(text)             => println!("Write: {text}"),
        Message::ChangeColor(r, g, b)   => println!("Color: rgb({r},{g},{b})"),
    }
}
```

### Struct Matching

```rust
struct Point { x: i32, y: i32 }

fn classify(p: Point) -> &'static str {
    match p {
        Point { x: 0, y: 0 }  => "origin",
        Point { x: 0, .. }    => "on y-axis",  // .. ignores other fields
        Point { y: 0, .. }    => "on x-axis",
        Point { x, y } if x == y => "on main diagonal",
        _                     => "somewhere else",
    }
}
```

### Tuple Matching

```rust
fn fizzbuzz(n: u32) -> &'static str {
    match (n % 3, n % 5) {
        (0, 0) => "FizzBuzz",
        (0, _) => "Fizz",
        (_, 0) => "Buzz",
        _      => "number",
    }
}
```

### Range Patterns

```rust
fn grade(score: u32) -> char {
    match score {
        90..=100 => 'A',
        80..=89  => 'B',
        70..=79  => 'C',
        60..=69  => 'D',
        0..=59   => 'F',
        _        => panic!("Invalid score {score}"),
    }
}
```

### Or-Patterns (`|`)

```rust
fn is_punctuation(c: char) -> bool {
    matches!(c, '.' | ',' | '!' | '?' | ';' | ':')
}

fn describe(n: i32) -> &'static str {
    match n {
        0           => "zero",
        1 | 2 | 3   => "small",
        4..=9       => "medium",
        10 | 20 | 30 => "round",
        _           => "other",
    }
}
```

### Binding with `@` (Bind and Test Simultaneously)

```rust
fn describe_number(n: i32) -> String {
    match n {
        x @ 1..=10  => format!("{x} is between 1 and 10"),
        x @ 11..=20 => format!("{x} is between 11 and 20"),
        x           => format!("{x} is out of range"),
    }
}
// x is bound AND tested in the same arm
```

### `if let` — Single-Pattern Match

```rust
// Use when you only care about one variant
let opt: Option<i32> = Some(42);

if let Some(n) = opt {
    println!("Got: {n}");
} else {
    println!("Got nothing");
}

// Chain with else if let
if let Some(n) = opt {
    println!("Some({n})");
} else if let Ok(n) = "5".parse::<i32>() {
    println!("Parsed: {n}");
} else {
    println!("Nothing");
}
```

### `while let` — Loop Until Pattern Fails

```rust
let mut stack = vec![1, 2, 3, 4];
while let Some(top) = stack.pop() {
    println!("Popped: {top}");
}
// Output: 4, 3, 2, 1
```

### The `matches!` Macro

```rust
// Quick boolean check without moving the value
let x: Option<i32> = Some(15);

let is_big_some = matches!(x, Some(n) if n > 10); // true
let is_none     = matches!(x, None);               // false

// Useful in .filter()
let opts = vec![Some(1), None, Some(5), None, Some(10)];
let big: Vec<_> = opts.iter()
    .filter(|o| matches!(o, Some(n) if *n > 4))
    .collect();
```

### Exhaustiveness: The Compiler's Guarantee

```rust
enum Color { Red, Green, Blue, Cyan }

fn name(c: Color) -> &'static str {
    match c {
        Color::Red   => "red",
        Color::Green => "green",
        Color::Blue  => "blue",
        // COMPILE ERROR: pattern `Cyan` not covered
        // Adding `_ => "other"` or handling Cyan fixes it
    }
}
```

This compile-time guarantee means you can never forget to handle a new variant you add to an enum. Every `match` will break at compile time, telling you exactly where you missed it.

---

## 2. Let-else for Early Return

### What It Is

`let-else` (stabilized in Rust 1.65) is a construct that says: "bind this pattern OR execute this diverging block." It keeps the happy path flat, at the same indentation level, by making every failed match an immediate exit.

### Mental Model

Think of `let-else` as a **contract validator at the door**. Each line is a guard: "If the data is the shape I expect, come in. Otherwise, leave immediately." All the guards succeed before any real logic runs, and the logic runs flat — not deeply nested.

### The Problem: Pyramid of Doom

```
WITHOUT let-else:                         WITH let-else:

fn process(data: Option<Input>) {         fn process(data: Option<Input>) {
    if let Some(input) = data {               let Some(input) = data
        if input.is_valid() {                 else { return; };
            if let Ok(n) = input.parse() {
                if n > 0 {                    if !input.is_valid() { return; }
                    // actual logic
                    // buried at indent 4   let Ok(n) = input.parse()
                }                             else { return; };
            }
        }
    }
}                                             if n <= 0 { return; }

                                              // actual logic: flat, at top level
                                          }
```

### Visual Architecture

```
NESTED (arrow anti-pattern):           FLAT (let-else):

fn f() {                               fn f() {
  if let Some(a) = get_a() {             let Some(a) = get_a() else { return; };
    if let Ok(b) = parse(a) {            let Ok(b)  = parse(a) else { return; };
      if let Some(c) = find(b) {         let Some(c) = find(b) else { return; };
        // work                          // work — same scope, no indentation
      }                                }
    }
  }
}

Depth: 3                               Depth: 0
```

### The Infographic Example — Dissected

```rust
fn get_user(id: u64) -> Option<User> { /* db call */ None }

fn process_user(id: u64) -> Result<(), AppError> {
    // "Give me the User, or return Err(NotFound) immediately"
    let Some(user) = get_user(id) else {
        return Err(AppError::NotFound);
    };

    // `user` is bound HERE, at the same scope level as the function body
    println!("Found: {}", user.name);
    Ok(())
}
```

### All Diverging Possibilities in `else`

The `else` block must diverge — it must never "fall through." The compiler enforces this.

```rust
fn diverge_examples(opt: Option<i32>, items: &[i32]) {
    // 1. return
    let Some(x) = opt else { return; };

    // 2. panic!
    let Some(y) = opt else { panic!("Expected a value!") };

    // 3. continue (only valid inside a loop)
    for &item in items {
        let val = item.checked_mul(2) else { continue; };
        println!("{val}");
    }

    // 4. break (only valid inside a loop)
    let mut iter = items.iter();
    loop {
        let Some(&n) = iter.next() else { break; };
        println!("{n}");
    }

    // 5. std::process::exit
    let Some(z) = opt else {
        eprintln!("Fatal: missing required value");
        std::process::exit(1);
    };

    // 6. return Err (in functions returning Result)
    let Some(w) = opt else {
        return; // simplified; real code might return Err
    };

    let _ = (x, y, z, w);
}
```

### Real-World: Configuration Parsing

```rust
use std::collections::HashMap;

#[derive(Debug)]
struct DbConfig {
    host: String,
    port: u16,
    db: String,
}

fn parse_config(raw: &str) -> Result<DbConfig, String> {
    let map: HashMap<&str, &str> = raw.lines()
        .filter_map(|line| line.split_once('='))
        .collect();

    let Some(&host) = map.get("host") else {
        return Err("Config missing 'host'".into());
    };

    let Some(&port_str) = map.get("port") else {
        return Err("Config missing 'port'".into());
    };

    let Ok(port) = port_str.parse::<u16>() else {
        return Err(format!("Invalid port value: '{port_str}'"));
    };

    let Some(&db) = map.get("database") else {
        return Err("Config missing 'database'".into());
    };

    Ok(DbConfig {
        host: host.to_string(),
        port,
        db: db.to_string(),
    })
}
```

### Critical Rule: Bindings Not Available in `else`

```rust
let Some(value) = some_option else {
    // `value` is NOT in scope here
    // The else block must diverge, so it doesn't matter
    return;
};
// `value` IS in scope here
println!("{value}");
```

### Combining with Destructuring

```rust
struct Config { debug: bool, workers: usize, name: Option<String> }

fn setup(opt: Option<Config>) {
    // Destructure right in the let-else pattern
    let Some(Config { debug, workers, name }) = opt else {
        println!("No config provided, using defaults");
        return;
    };

    let name = name.unwrap_or_else(|| "unnamed".to_string());
    println!("debug={debug}, workers={workers}, name={name}");
}
```

### `if let` vs `let-else`: Choose Based on Shape

```rust
// Use if let when the match IS the body of work
if let Some(user) = get_user(id) {
    process(user);
}

// Use let-else when the match is a PRECONDITION for work
let Some(user) = get_user(id) else { return; };
// ... more work that needs user
```

---

## 3. ? for Error Propagation

### What It Is

The `?` operator is Rust's ergonomic mechanism for propagating errors up the call stack. It short-circuits on failure: if the expression is `Err(e)` or `None`, it immediately returns from the current function with an appropriate error value. If it is `Ok(v)` or `Some(v)`, it unwraps the value and continues.

### Mental Model

Think of `?` as **"try, or bail"**. Each `?` is a checkpoint: "If this worked, give me the value. If it failed, I'm done — propagate the error up to whoever called me."

### What `?` Actually Expands To

```rust
// With ?:
let data = read_file(path)?;

// Equivalent to (roughly):
let data = match read_file(path) {
    Ok(value) => value,
    Err(e)    => return Err(From::from(e)),
};
//                          ^^^^^^^^^^^^
//                    Automatic type conversion!
//                    This is why different error types work.
```

### Error Propagation Flow

```
fn process() -> Result<Output, AppError> {
                                               |
    let a = step_one()?;       Err(IoErr)  ───┤
    let b = step_two(a)?;      Err(ParseErr)──┤──► return Err(AppError::from(e))
    let c = step_three(b)?;    Err(Custom) ───┤
    Ok(transform(c))                           |
}

If all succeed:
  step_one -> Ok(a) -> step_two(a) -> Ok(b) -> step_three(b) -> Ok(c) -> Ok(transform(c))
  return Ok(transform(c))

If step_two fails:
  step_one -> Ok(a) -> step_two(a) -> Err(e) -> return Err(AppError::from(e))
```

### The Infographic Example — Dissected

```rust
use serde::Deserialize;

#[derive(Deserialize)]
struct Config { name: String, value: i32 }

fn load_config(path: &str) -> Result<Config, Box<dyn std::error::Error>> {
    // ? on read_file: if Err(io::Error), converts and returns it
    let data = std::fs::read_to_string(path)?;

    // ? on from_str: if Err(serde_json::Error), converts and returns it
    let cfg: Config = serde_json::from_str(&data)?;

    Ok(cfg)
}
```

### The `From` Trait: Automatic Error Conversion

The `?` operator calls `From::from(err)` on the error before returning. This is what allows different error types to work with `?` in the same function.

```rust
use std::{io, num::ParseIntError};

#[derive(Debug)]
enum AppError {
    Io(io::Error),
    Parse(ParseIntError),
    Custom(String),
}

// Implement From for each error type
impl From<io::Error>       for AppError { fn from(e: io::Error)       -> Self { AppError::Io(e) } }
impl From<ParseIntError>   for AppError { fn from(e: ParseIntError)   -> Self { AppError::Parse(e) } }

fn read_and_parse(path: &str) -> Result<i32, AppError> {
    let content = std::fs::read_to_string(path)?; // io::Error -> AppError::Io
    let n: i32  = content.trim().parse()?;         // ParseIntError -> AppError::Parse
    Ok(n * 2)
}
```

### Using `thiserror` — Clean Error Definitions

```rust
use thiserror::Error;

#[derive(Debug, Error)]
enum AppError {
    #[error("IO failure: {0}")]
    Io(#[from] std::io::Error),               // #[from] generates From impl

    #[error("Parse failure: {0}")]
    Parse(#[from] std::num::ParseIntError),

    #[error("Validation: {message}")]
    Validation { message: String },
}

fn process_file(path: &str) -> Result<i32, AppError> {
    let content = std::fs::read_to_string(path)?;   // auto From
    let n: i32  = content.trim().parse()?;           // auto From
    if n < 0 {
        return Err(AppError::Validation { message: format!("{n} must be non-negative") });
    }
    Ok(n)
}
```

### `?` on `Option`

Inside a function returning `Option<T>`, `?` propagates `None`:

```rust
fn find_first_even_doubled(nums: &[i32]) -> Option<i32> {
    let first = nums.first()?;           // None if slice is empty
    let even  = nums.iter().find(|&&n| n % 2 == 0)?; // None if none found
    let doubled = even.checked_mul(2)?;  // None on overflow
    Some(doubled)
}
```

### `?` in `main()`

```rust
fn main() -> Result<(), Box<dyn std::error::Error>> {
    let text    = std::fs::read_to_string("data.txt")?;
    let number: i32 = text.trim().parse()?;
    println!("Number doubled: {}", number * 2);
    Ok(())
}
```

### `anyhow` — Application-Level Error Handling

For binary applications (not libraries), `anyhow` removes all boilerplate:

```rust
use anyhow::{Context, Result};

fn load_users(path: &str) -> Result<Vec<String>> {
    let content = std::fs::read_to_string(path)
        .with_context(|| format!("Failed to read '{path}'"))?;

    let users: Vec<String> = serde_json::from_str(&content)
        .context("Failed to parse users JSON")?;

    Ok(users)
}
```

### Call Stack Visualization

```
main()
  └─► load_config("app.json")
        └─► fs::read_to_string("app.json")  <── OS syscall fails: ENOENT
               │
               └─► Err(io::Error { kind: NotFound })
                            │
          ◄──────────────── ? ──── AppError::from(io_err)
        returns Err(AppError::Io(...))
  ◄────────────────────────────────
returns Err(...)
main prints error and exits
```

---

## 4. Iterator Adapters

### What They Are

Iterator adapters are lazy transformation methods on iterators. They form a composable pipeline that expresses *what* you want done to data, not *how* to loop over it. They are zero-cost abstractions: the Rust compiler optimizes iterator chains into machine code equivalent to hand-written loops — no virtual dispatch, no heap allocation.

### Mental Model

Think of iterators as a **lazy assembly line**. Each adapter adds a station to the line. Nothing actually happens until a consuming method (`.collect()`, `.sum()`, `.for_each()`, etc.) turns on the machine. Only then does data flow through all stations, one item at a time.

```
Source (vec, range, etc.)
    |
    ▼
┌──────────────────┐
│   .filter(pred)  │  ─── gate: passes items where pred returns true
└────────┬─────────┘
         |
         ▼
┌──────────────────┐
│   .map(func)     │  ─── transformer: applies func to each item
└────────┬─────────┘
         |
         ▼
┌──────────────────┐
│   .take(n)       │  ─── limiter: stops after n items
└────────┬─────────┘
         |
         ▼
┌──────────────────┐
│   .collect()     │  ─── consumer: materializes results into a collection
└──────────────────┘
```

### The Infographic Example — Dissected

```rust
struct Item { active: bool, value: i32 }

fn sum_active_values(items: &[Item]) -> i32 {
    items.iter()                  // Iterator<Item = &Item>
        .filter(|x| x.active)    // keeps only active items
        .map(|x| x.value)        // extracts the i32 value
        .sum()                   // CONSUMES: adds all values
}
```

### Lazy Evaluation Proof

```rust
fn side_effect_demo() {
    let expensive = || {
        println!("COMPUTED");
        42
    };

    // Building the pipeline — nothing prints yet
    let pipeline = vec![1, 2, 3].into_iter()
        .filter(|&n| { println!("filter {n}"); n > 1 })
        .map(|n|    { println!("map {n}");    n * 10  });

    println!("Pipeline built, no work done yet");

    // NOW work happens, item by item
    let result: Vec<i32> = pipeline.collect();
    // Output:
    // Pipeline built, no work done yet
    // filter 1
    // filter 2
    // map 2
    // filter 3
    // map 3
}
```

### Complete Adapter Reference

```rust
fn all_adapters() {
    let nums = vec![1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

    // ─── TRANSFORMING ───────────────────────────────────────────────────

    // map: transform each element T -> U
    let sq: Vec<i32> = nums.iter().map(|&n| n * n).collect();

    // flat_map: transform then flatten (T -> impl IntoIterator<U>)
    let words = vec!["hello world", "foo bar"];
    let tokens: Vec<&str> = words.iter()
        .flat_map(|s| s.split_whitespace())
        .collect();
    // ["hello", "world", "foo", "bar"]

    // flatten: flattens one level of nesting
    let nested = vec![vec![1,2], vec![3,4]];
    let flat: Vec<i32> = nested.into_iter().flatten().collect();

    // enumerate: adds (index, item) pairs
    for (i, &val) in nums.iter().enumerate() {
        println!("{i}: {val}");
    }

    // zip: pairs two iterators element-by-element
    let letters = vec!['a', 'b', 'c'];
    let pairs: Vec<_> = nums.iter().zip(letters.iter()).collect();

    // chain: concatenates two iterators
    let first  = vec![1, 2, 3];
    let second = vec![4, 5, 6];
    let all: Vec<i32> = first.iter().chain(second.iter()).copied().collect();

    // scan: like fold but yields intermediate state
    let running_sum: Vec<i32> = nums.iter()
        .scan(0, |acc, &n| { *acc += n; Some(*acc) })
        .collect();
    // [1, 3, 6, 10, 15, 21, 28, 36, 45, 55]

    // ─── FILTERING ──────────────────────────────────────────────────────

    // filter: keep elements matching predicate
    let evens: Vec<&i32> = nums.iter().filter(|&&n| n % 2 == 0).collect();

    // filter_map: filter and transform simultaneously (Option-returning fn)
    let parsed: Vec<i32> = vec!["1", "two", "3"]
        .iter()
        .filter_map(|s| s.parse::<i32>().ok())
        .collect();
    // [1, 3]

    // take: first n items
    let first3: Vec<&i32> = nums.iter().take(3).collect();

    // skip: drop first n items
    let last3: Vec<&i32> = nums.iter().skip(7).collect();

    // take_while: take while predicate holds (stops at first false)
    let small: Vec<&i32> = nums.iter().take_while(|&&n| n < 5).collect();

    // skip_while: skip while predicate holds, then take rest
    let rest: Vec<&i32> = nums.iter().skip_while(|&&n| n < 5).collect();

    // step_by: step through iterator with stride
    let every_other: Vec<&i32> = nums.iter().step_by(2).collect();

    // ─── CONSUMING ──────────────────────────────────────────────────────

    let total: i32       = nums.iter().sum();
    let product: i32     = nums.iter().product();
    let count            = nums.iter().filter(|&&n| n > 5).count();
    let has_even         = nums.iter().any(|&n| n % 2 == 0);
    let all_positive     = nums.iter().all(|&n| n > 0);
    let first_even       = nums.iter().find(|&&n| n % 2 == 0);
    let even_pos         = nums.iter().position(|&n| n % 2 == 0);
    let maximum          = nums.iter().max();
    let minimum          = nums.iter().min();
    let max_by_key       = nums.iter().max_by_key(|&&n| -(n as i32)); // min value via max_by_key

    // fold: general accumulation
    let sum_of_sq: i32 = nums.iter().fold(0, |acc, &n| acc + n * n);

    // reduce: like fold but uses first element as initial accumulator
    let sum2 = nums.iter().copied().reduce(|acc, n| acc + n);

    // for_each: side effects (consumes)
    nums.iter().for_each(|n| print!("{n} "));
}
```

### Implementing a Custom Iterator

```rust
struct Fibonacci {
    a: u64,
    b: u64,
}

impl Fibonacci {
    fn new() -> Self { Fibonacci { a: 0, b: 1 } }
}

impl Iterator for Fibonacci {
    type Item = u64;

    fn next(&mut self) -> Option<u64> {
        let next = self.a + self.b;
        self.a = self.b;
        self.b = next;
        Some(self.a)
    }
}

// Usage: first 10 Fibonacci numbers
let fibs: Vec<u64> = Fibonacci::new().take(10).collect();
// [1, 1, 2, 3, 5, 8, 13, 21, 34, 55]

// Sum of Fibonacci numbers under 100
let sum: u64 = Fibonacci::new()
    .take_while(|&n| n < 100)
    .sum();
```

---

## 5. Collect like a Pro

### What It Is

`.collect()` is a consuming iterator method that assembles the stream of values into a concrete collection. The **turbofish** syntax `::<Type>` or a type annotation tells Rust what container to build. This is more powerful than it appears: you can collect into `Vec`, `HashMap`, `HashSet`, `String`, `Result<Vec<_>,_>`, `Option<Vec<_>>`, and any type implementing `FromIterator`.

### Mental Model

Think of `.collect()` as a **funnel**. The iterator stream of values flows into the funnel, and the output container shape is determined by the type you specify. Rust uses the `FromIterator` trait to define how each container is assembled.

```
Iterator<Item = T>
         |
         | (stream of T values)
         |
         ▼
   .collect::<Container<T>>()
         |
         | (calls Container::from_iter)
         |
         ▼
  Container<T>  (Vec, HashMap, HashSet, String, Result, ...)
```

### Type-to-Collection Map

```
Iterator of...           Collects into...
──────────────────────────────────────────────────────
T                    →   Vec<T>
(K, V)               →   HashMap<K, V>
T: Hash + Eq         →   HashSet<T>
T: Ord               →   BTreeSet<T>
(K: Ord, V)          →   BTreeMap<K, V>
char                 →   String
Result<T, E>         →   Result<Vec<T>, E>  (stops at first error)
Option<T>            →   Option<Vec<T>>     (stops at first None)
```

### The Infographic Example — Dissected

```rust
struct User { id: u64, name: String }

fn get_user_ids(users: &[User]) -> Vec<u64> {
    // Type annotation: Rust knows to build Vec<u64>
    let ids: Vec<u64> = users.iter()
        .map(|u| u.id)
        .collect();
    ids
}

// Explicit turbofish — same result:
let ids = users.iter().map(|u| u.id).collect::<Vec<u64>>();

// Turbofish with _ for inference:
let ids = users.iter().map(|u| u.id).collect::<Vec<_>>();
```

### Collecting into Different Containers

```rust
use std::collections::{HashMap, HashSet, BTreeMap, BTreeSet};

let data = vec![3, 1, 4, 1, 5, 9, 2, 6, 5];

// Vec — ordered, allows duplicates
let v: Vec<i32> = data.iter().copied().collect();

// HashSet — unordered, unique values
let set: HashSet<i32> = data.iter().copied().collect();
// {1, 2, 3, 4, 5, 6, 9}  (no duplicates, random order)

// BTreeSet — sorted, unique
let bset: BTreeSet<i32> = data.iter().copied().collect();
// {1, 2, 3, 4, 5, 6, 9}  (always sorted)

// HashMap from (key, value) tuples
let pairs = vec![("alice", 30), ("bob", 25), ("carol", 35)];
let map: HashMap<&str, i32> = pairs.into_iter().collect();

// BTreeMap — sorted by key
let bmap: BTreeMap<&str, i32> = vec![("c", 3), ("a", 1), ("b", 2)]
    .into_iter().collect();
// Keys in order: a, b, c

// String from chars
let chars = vec!['R', 'u', 's', 't'];
let s: String = chars.into_iter().collect();
// "Rust"
```

### Collecting into Result — Transactional Semantics

```rust
// If ALL succeed: Ok(Vec<T>)
// If ANY fails:   Err(E)  (short-circuits at first error)
fn parse_all(strs: &[&str]) -> Result<Vec<i32>, std::num::ParseIntError> {
    strs.iter()
        .map(|s| s.parse::<i32>())
        .collect()
}

fn main() {
    println!("{:?}", parse_all(&["1", "2", "3"]));    // Ok([1, 2, 3])
    println!("{:?}", parse_all(&["1", "oops", "3"])); // Err(invalid digit found)
}
```

### Collecting into Option

```rust
// If ALL are Some: Some(Vec<T>)
// If ANY is None:  None
fn double_all(opts: &[Option<i32>]) -> Option<Vec<i32>> {
    opts.iter()
        .map(|o| o.map(|n| n * 2))
        .collect()
}

println!("{:?}", double_all(&[Some(1), Some(2), Some(3)])); // Some([2, 4, 6])
println!("{:?}", double_all(&[Some(1), None,    Some(3)])); // None
```

### `partition` — Split into Two Collections

```rust
let numbers = vec![1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

let (evens, odds): (Vec<i32>, Vec<i32>) = numbers.iter()
    .partition(|&&n| n % 2 == 0);
// evens: [2, 4, 6, 8, 10]
// odds:  [1, 3, 5, 7, 9]
```

### `unzip` — Split Pair Iterator

```rust
let pairs = vec![(1, 'a'), (2, 'b'), (3, 'c')];
let (nums, chars): (Vec<i32>, Vec<char>) = pairs.into_iter().unzip();
// nums:  [1, 2, 3]
// chars: ['a', 'b', 'c']
```

### `size_hint` and Why `.collect()` Is Efficient

```rust
// Many iterators know their size ahead of time.
// .collect() uses size_hint() to pre-allocate the Vec — equivalent to with_capacity().
// So:
let v: Vec<i32> = (0..1000).map(|n| n * 2).collect();
// This pre-allocates for 1000 elements before filling — zero reallocations.
```

---

## 6. Option Combinators

### What They Are

`Option<T>` represents a value that may or may not exist. Instead of pattern-matching everywhere, Rust provides a rich combinator API — methods that transform, chain, or consume Options without manually writing `match`. They enable null-safe pipelines that compose cleanly.

### Mental Model

Think of `Option<T>` as a **box that is either full or empty**. Combinators let you perform operations on the box from outside, without opening it. If the box is empty (`None`), combinators return `None` and skip the operation. If it has a value (`Some(v)`), they apply the transformation.

### State Transitions

```
    map(f):      Some(T) ──► Some(f(T))
                 None    ──► None

    and_then(f): Some(T) ──► f(T)   [where f: T -> Option<U>]
                 None    ──► None

    filter(p):   Some(T) ──► Some(T) if p(T) else None
                 None    ──► None

    unwrap_or(d):Some(T) ──► T
                 None    ──► d

    or(other):   Some(T) ──► Some(T)
                 None    ──► other
```

### The Infographic Example — Dissected

```rust
struct User    { profile: Option<Profile> }
struct Profile { name: String }

fn get_profile_name(user: Option<User>) -> Option<String> {
    user
        .as_ref()                            // &Option<User> -> Option<&User>  (no move)
        .and_then(|u| u.profile.as_ref())   // Option<&User> -> Option<&Profile>
        .map(|p| p.name.clone())             // Option<&Profile> -> Option<String>
}
```

### Complete Combinator Reference

```rust
fn demonstrate_option_combinators() {
    let some: Option<i32> = Some(42);
    let none: Option<i32> = None;

    // ─── TRANSFORMING ────────────────────────────────────────────────────

    // map: apply fn to inner value; None passes through
    assert_eq!(some.map(|n| n * 2), Some(84));
    assert_eq!(none.map(|n| n * 2), None);

    // map_or: map with a default for the None case
    assert_eq!(none.map_or(0, |n| n * 2), 0);
    assert_eq!(some.map_or(0, |n| n * 2), 84);

    // map_or_else: lazy default (fn only called when None)
    assert_eq!(none.map_or_else(|| 99, |n| n * 2), 99);

    // and_then: chain — fn returns Option, avoids Option<Option<T>>
    let result = some.and_then(|n| if n > 10 { Some(n * 2) } else { None });
    assert_eq!(result, Some(84));

    // ─── UNWRAPPING SAFELY ───────────────────────────────────────────────

    // unwrap_or: use default value if None
    assert_eq!(none.unwrap_or(0), 0);
    assert_eq!(some.unwrap_or(0), 42);

    // unwrap_or_else: lazy default (fn only called when None)
    assert_eq!(none.unwrap_or_else(|| 99), 99);

    // unwrap_or_default: use Default trait implementation
    let n: i32 = none.unwrap_or_default(); // 0 (i32's Default)

    // ─── FILTERING ───────────────────────────────────────────────────────

    // filter: Some only if predicate holds, else None
    assert_eq!(some.filter(|&n| n > 10),  Some(42));
    assert_eq!(some.filter(|&n| n > 100), None);
    assert_eq!(none.filter(|&n| n > 10),  None);

    // ─── CONVERSION ──────────────────────────────────────────────────────

    // ok_or: Option<T> -> Result<T, E>
    let r: Result<i32, &str> = some.ok_or("missing");      // Ok(42)
    let r: Result<i32, &str> = none.ok_or("missing");      // Err("missing")

    // ok_or_else: lazy version
    let r = none.ok_or_else(|| format!("Error: no value at line {}", line!()));

    // as_ref: &Option<T> -> Option<&T>  (borrow without consuming)
    let opt_string: Option<String> = Some("hello".to_string());
    let opt_str: Option<&String> = opt_string.as_ref(); // didn't consume opt_string!

    // as_deref: &Option<String> -> Option<&str>
    let opt_deref: Option<&str> = opt_string.as_deref(); // Some("hello")

    // ─── COMBINING ───────────────────────────────────────────────────────

    // or: fallback to another Option if None
    let combined = none.or(Some(5));    // Some(5)
    let combined = some.or(Some(999));  // Some(42) — original wins

    // or_else: lazy fallback
    let combined = none.or_else(|| Some(compute_backup()));

    // zip: combine two Options into Option of tuple
    let a: Option<i32>   = Some(1);
    let b: Option<&str>  = Some("hi");
    let zipped = a.zip(b); // Some((1, "hi"))
    let zipped = a.zip(None::<&str>); // None

    // unzip
    let (oa, ob): (Option<i32>, Option<&str>) = Some((1, "hi")).unzip();

    // flatten: Option<Option<T>> -> Option<T>
    let nested: Option<Option<i32>> = Some(Some(42));
    assert_eq!(nested.flatten(), Some(42));

    // ─── CHECKING ────────────────────────────────────────────────────────

    let _ = some.is_some();                        // true
    let _ = none.is_none();                        // true
    let _ = some.is_some_and(|&n| n > 40);         // true (Rust 1.70+)
}

fn compute_backup() -> i32 { 99 }
```

### Null-Safe Deep Navigation

```rust
struct Company { ceo: Option<Person> }
struct Person  { address: Option<Address> }
struct Address { city: Option<String> }

fn ceo_city(company: &Company) -> Option<&str> {
    company.ceo.as_ref()
        .and_then(|p| p.address.as_ref())
        .and_then(|a| a.city.as_deref())
}

// Same with ? operator (in function returning Option):
fn ceo_city_v2(company: &Company) -> Option<&str> {
    let ceo  = company.ceo.as_ref()?;
    let addr = ceo.address.as_ref()?;
    addr.city.as_deref()
}
```

---

## 7. Result Combinators

### What They Are

`Result<T, E>` represents operations that either succeed (`Ok(T)`) or fail (`Err(E)`). Like Option, Result has a rich combinator API for transforming success values, transforming errors, recovering from errors, and chaining fallible operations — all without pattern matching.

### Mental Model

Think of `Result` as a **train on two parallel tracks**: the Ok track (success) and the Err track (failure). Combinators that work on the success value are "applied only on the Ok track"; errors pass through. Combinators that work on the error value are "applied only on the Err track"; values pass through.

```
Ok(T)  ──── map(f)      ──────────────── Ok(f(T))
Err(E) ──── map(f)      ──────────────── Err(E)     [f skipped]

Ok(T)  ──── and_then(f) ──────────────── f(T)        [f: T -> Result<U,E>]
Err(E) ──── and_then(f) ──────────────── Err(E)      [f skipped]

Ok(T)  ──── map_err(g)  ──────────────── Ok(T)       [g skipped]
Err(E) ──── map_err(g)  ──────────────── Err(g(E))

Ok(T)  ──── or_else(h)  ──────────────── Ok(T)       [h skipped]
Err(E) ──── or_else(h)  ──────────────── h(E)        [h: E -> Result<T,F>]
```

### The Infographic Example — Dissected

```rust
fn some_fallible_fn() -> Result<i32, String> { Ok(21) }
fn fallback()          -> Result<i32, String> { Ok(0) }

fn process() -> Result<i32, String> {
    let value = some_fallible_fn()
        .map(|v| v * 2)              // transform Ok value
        .or_else(|_| fallback())?;   // on error, try fallback; ? propagates remaining err
    Ok(value)
}
```

### Complete Combinator Reference

```rust
fn demonstrate_result_combinators() {
    let ok:  Result<i32, String> = Ok(42);
    let err: Result<i32, String> = Err("oops".to_string());

    // ─── TRANSFORMING OK VALUE ────────────────────────────────────────────

    // map: transform Ok(T) -> Ok(U); Err passes through
    assert_eq!(ok.as_ref().map(|&n| n * 2),  Ok(84));
    assert_eq!(err.as_ref().map(|&n| n * 2), Err(&"oops".to_string()));

    // and_then: chain a fallible function; Err passes through
    let chained: Result<String, String> = ok
        .and_then(|n| if n > 0 { Ok(n.to_string()) } else { Err("neg".to_string()) });

    // ─── TRANSFORMING ERROR VALUE ─────────────────────────────────────────

    // map_err: transform Err(E) -> Err(F); Ok passes through
    let remapped: Result<i32, i32> = err.as_ref().map_err(|e| e.len() as i32);

    // ─── RECOVERING FROM ERRORS ───────────────────────────────────────────

    // unwrap_or: default value on Err
    assert_eq!(err.as_ref().unwrap_or(&0), &0);
    assert_eq!(ok.as_ref().unwrap_or(&0),  &42);

    // unwrap_or_else: lazy default (fn only called on Err)
    let val = err.unwrap_or_else(|e| {
        eprintln!("Warning: {e}");
        0
    });

    // unwrap_or_default: uses Default trait
    let val: i32 = Err::<i32, &str>("fail").unwrap_or_default(); // 0

    // or: try another Result
    let recovered: Result<i32, String> = Err("fail".into()).or(Ok(99));

    // or_else: lazy recovery
    let recovered = err.or_else(|e| {
        if e == "oops" { Ok(0) } else { Err(e) }
    });

    // ─── CONVERSION ──────────────────────────────────────────────────────

    // ok: Result<T,E> -> Option<T>  (discards error)
    let opt: Option<i32> = ok.ok();     // Some(42)
    let opt: Option<i32> = err.err().map(|_| 0); // workaround

    // err: Result<T,E> -> Option<E>  (discards Ok value)
    let opt_e: Option<String> = Err::<i32, String>("fail".into()).err(); // Some("fail")

    // ─── CHECKING ────────────────────────────────────────────────────────

    let _ = ok.is_ok();                         // true
    let _ = err.is_err();                       // true
    let _ = ok.is_ok_and(|&n| n > 40);          // true (Rust 1.70+)
    let _ = err.is_err_and(|e| e == "oops");     // true
}
```

### Chaining Multiple Fallible Operations

```rust
use std::{num::ParseIntError, io};

#[derive(Debug)]
enum Error {
    Io(io::Error),
    Parse(ParseIntError),
    Validation(String),
}

fn load_positive_number(path: &str) -> Result<i32, Error> {
    std::fs::read_to_string(path)
        .map_err(Error::Io)
        .and_then(|content| {
            content.trim()
                .parse::<i32>()
                .map_err(Error::Parse)
        })
        .and_then(|n| {
            if n > 0 { Ok(n) }
            else { Err(Error::Validation(format!("{n} must be positive"))) }
        })
}
```

### Collecting Results

```rust
fn parse_all(inputs: &[&str]) -> Result<Vec<i32>, std::num::ParseIntError> {
    inputs.iter()
        .map(|s| s.parse::<i32>())
        .collect::<Result<Vec<_>, _>>() // stops at first Err
}
```
