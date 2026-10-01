# Rust DSA Debugging & Introspection Guide

A practical toolkit for seeing what Rust is actually doing under the hood while
solving DSA problems — from quick inline checks to memory-level truth.
Ordered cheapest → deepest. Use the cheap tools by default; reach for the
deeper ones when a solution *compiles* but you don't trust it, or when the
borrow checker is fighting you and you don't know why.

---

## 1. `dbg!()` instead of `println!`

The fastest way to get step-by-step visibility with zero restructuring.

### Why it's better than `println!`
`dbg!(expr)` prints the file, line number, the *source text* of the
expression, and its value — then returns the value unchanged, so you can
wrap it around any sub-expression inline without breaking the surrounding
code.

```rust
fn two_sum(nums: &[i32], target: i32) -> Option<(usize, usize)> {
    for i in 0..nums.len() {
        for j in (i + 1)..nums.len() {
            if dbg!(nums[i] + nums[j]) == target {
                return Some((i, j));
            }
        }
    }
    None
}
```

Output looks like:
```
[src/main.rs:4:16] nums[i] + nums[j] = 7
[src/main.rs:4:16] nums[i] + nums[j] = 9
```

### Practical patterns for DSA
- Wrap **loop indices and the value about to be mutated**, not just final
  results: `dbg!(arr[j], arr[j+1]);` right before a swap tells you exactly
  what state triggered it.
- On recursive functions, `dbg!` the input at the top of the function to see
  the call tree unfold in order — cheap substitute for a call-stack view:
  ```rust
  fn fib(n: u64) -> u64 {
      dbg!(n);
      if n <= 1 { return n; }
      fib(n - 1) + fib(n - 2)
  }
  ```
- Use `eprintln!("{i}: {arr:?}")` for a full-array snapshot per iteration
  when you want to watch a structure evolve (sorting, DP tables) rather than
  one value at a time.
- Remove or gate behind `#[cfg(debug_assertions)]` before submitting — `dbg!`
  is a debugging tool, not instrumentation to ship.

**When to reach past this:** once you need to inspect state *without* adding
print statements everywhere, or you need to pause execution and poke around
interactively — that's step 2.

---

## 2. Real step debugging (gdb / lldb)

This is the direct equivalent of what `sys.settrace` fakes in Python —
except it's a real debugger reading real DWARF debug info, so you see move
and drop points, not just values.

### Setup

Compile with debug info (default for `cargo build`, not `--release`):
```bash
cargo build   # debug profile keeps symbols by default
```

**Linux:** `rust-gdb target/debug/your_binary`
**macOS:** `rust-lldb target/debug/your_binary`
**VS Code (either OS):** install the **CodeLLDB** extension — this is the
easiest day-to-day workflow: click left of a line number to set a
breakpoint, hit F5, then step with F10 (over) / F11 (into).

### Core commands (gdb / rust-gdb)
| Action | Command |
|---|---|
| Set breakpoint | `break src/main.rs:14` or `break fib` |
| Run | `run` |
| Step over | `next` (`n`) |
| Step into | `step` (`s`) |
| Continue to next breakpoint | `continue` (`c`) |
| Print a variable | `print arr` or `p arr[i]` |
| Print all locals in scope | `info locals` |
| Show call stack | `bt` (backtrace) |
| Watch a variable (break on change) | `watch arr[j]` |
| Quit | `quit` |

### A DSA-specific workflow
For every problem, before you trust a full run:
1. Set a breakpoint **inside your core loop** (the swap line, the
   partition step, the memo lookup).
2. Run, then single-step (`n`) through 2–3 iterations by hand.
3. At each stop, `info locals` and check: does the state match what you
   expect *before* this line executes, and does it match what you expect
   *after*?
4. On recursion, use `bt` at the deepest point to see the actual call chain
   — this is the fastest way to catch "off-by-one base case" bugs, because
   you'll see one extra (or one missing) frame immediately.

Most logic bugs in DSA code live in the first 2–3 iterations of the core
loop — this workflow catches them before you waste time debugging with
print statements on the full input.

---

## 3. Reading the compiler instead of fighting it

Rust's compiler errors are unusually information-dense — the habit to build
is reading them as *diagnosis*, not noise to scroll past.

### `rustc --explain`
Every error has a long-form explanation with a minimal reproducing example
and the fix:
```bash
rustc --explain E0502
```
Run this the first few times you see a new error code — the explanations
teach the underlying rule (e.g. *why* a mutable and immutable borrow can't
coexist), which generalizes to the next 10 times you hit something similar.

### Diagnosing borrow-checker rejections
When something is rejected, don't guess-fix — identify which of two things
is actually happening:
- **A live borrow overlaps a mutation** — you're holding a `&` or `&mut`
  reference (often implicitly, via an iterator or a `.get()` return) while
  trying to mutate the same data.
- **A moved value is being reused** — ownership of a non-`Copy` value
  transferred (into a function, into a struct field, into a closure) and
  you're using the original binding afterward.

The error message names which one it is; `rustc --explain` on the specific
code fills in the mental model.

### `cargo expand`
Shows macro-expanded code — useful when you're unsure what `?`, `for`,
`matches!`, or a derive macro actually desugars to under the hood:
```bash
cargo install cargo-expand
cargo expand --bin your_binary
```

### `cargo clippy`
Catches non-idiomatic patterns that usually correlate with a wrong mental
model rather than just style:
```bash
cargo clippy
```
A recurring `clippy::needless_clone` is a strong signal you didn't actually
need ownership at that point — a good moment to go back and ask why you
reached for `.clone()` instead of a reference.

---

## 4. Miri — ground truth for memory behavior

`dbg!` and a debugger tell you what your program *did*. Miri tells you
whether what it did was actually **defined behavior** — critical for DSA
code using raw pointers, unchecked indexing, or interior mutability
(`RefCell`, `Cell`), where a program can silently "work" today and be wrong.

### Setup
```bash
rustup component add miri --toolchain nightly
cargo +nightly miri run
```

### What it catches that normal execution won't
- Out-of-bounds access via unsafe indexing or raw pointer arithmetic
- Use-after-free / dangling references
- Aliasing violations (two `&mut` references to overlapping data, even if
  you never observe wrong output)
- Reading uninitialized memory
- Integer overflow in unsafe contexts that would otherwise be masked

### When to reach for it
- Any time you write `unsafe` in a DSA solution (custom linked structures,
  manual memory tricks for performance).
- When a solution using `RefCell`/`Rc<RefCell<_>>` (common in tree/graph
  problems) passes your test cases but you're not fully sure the borrowing
  is sound.
- As a periodic sanity pass on solutions you plan to keep as reference —
  "passes tests" and "is correct" are different claims, and Miri closes
  most of the gap between them.

---

## 5. Godbolt (Compiler Explorer) — how Rust actually thinks

[godbolt.org](https://godbolt.org) compiles a snippet and shows you the
generated assembly side-by-side, with the source lines highlighted against
the instructions they produced. This is where "zero-cost abstraction"
claims stop being a slogan and become something you can verify.

### What to look for in DSA code
- **Bounds-check elision**: compare `arr[i]` (indexing) against
  `arr.iter()` (iterator) versions of the same loop — with optimizations on
  (`-O` / `opt-level=3`), iterator versions often eliminate the bounds
  check entirely, indexing sometimes doesn't. Seeing this in the assembly
  is more convincing than reading it as a rule.
- **Tail-call behavior on recursion**: check whether your recursive
  function actually gets optimized into a loop, or whether every call
  really does grow the stack — relevant for understanding why some
  "clean" recursive DSA solutions blow the stack on large inputs and others
  don't.
- **Monomorphization cost**: for generic functions, Godbolt shows you the
  concrete instantiated code per type — useful for building intuition about
  what generics actually cost (usually: nothing at runtime, more compile
  time and binary size).

### Practical workflow
1. Paste a small, self-contained function (not the whole solution).
2. Toggle optimization level (`-O0` vs `-O3`) and diff the output mentally.
3. Compare two implementations of the same logic (index vs iterator, loop
   vs recursion) side by side.

This is the slowest of the six tools to get value from, but it's the one
that actually answers "how does Rust think" at the level you asked about —
everything else tells you about your program's behavior, this tells you
about the compiler's reasoning.

---

## 6. Closing the loop into your existing practice

You already run a Socratic pair-programming protocol for DSA practice. The
addition that ties all five tools above together:

**After a solution compiles, before moving to the next problem:**
1. Run it once under `rust-gdb`/`rust-lldb` with a breakpoint in the core
   loop or recursive call.
2. Narrate out loud, line by line, what's actually happening to *ownership
   and memory* — not just "what's the value now" but "who owns this right
   now, and did anything just move or get dropped."
3. If the problem used `unsafe`, `RefCell`, or raw pointers, run it through
   Miri before considering it done.
4. If something about the compiler's behavior surprised you (an
   optimization, a rejected borrow you didn't expect), take the minimal
   reproducing snippet to Godbolt or `rustc --explain` and resolve the
   *why*, not just the fix.

This turns each problem into two passes: one to get it working, one to
verify you understand what Rust actually did to make it work — which is
the thing that builds the instinct, not the working solution itself.

---

## Quick reference

| Question | Tool |
|---|---|
| What's this value right now? | `dbg!()` |
| What's happening line-by-line, right now, interactively? | `rust-gdb` / `rust-lldb` / CodeLLDB |
| Why did the compiler reject this? | `rustc --explain`, read borrow vs move |
| What does this macro/desugaring actually expand to? | `cargo expand` |
| Is this pattern idiomatic, or a sign of a wrong model? | `cargo clippy` |
| Is this `unsafe`/`RefCell` code actually sound? | `cargo +nightly miri run` |
| What did the compiler actually generate, and is it as cheap as I think? | godbolt.org |
