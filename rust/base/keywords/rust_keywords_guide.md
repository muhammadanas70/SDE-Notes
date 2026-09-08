# Rust Keywords — A Complete Engineering Reference

> Same test as the C# guide: not "what does `mut` do" but what compile-time proof it buys, and what runtime check it lets you delete. Rust's whole pitch is that almost every keyword below answers "how do I get rustc to prove this so nothing has to check it while the program is running." Where the C# guide kept finding a GC or a JIT quietly doing dynamic work behind a keyword, Rust keeps nearly all of that work at compile time — the CLR's safety net becomes the borrow checker, and once compilation succeeds, it costs nothing.

---

## 0. Mental Model First

```
                         ┌─────────────────────────────────────┐
                         │           RUST KEYWORDS               │
                         └───────────────────┬───────────────────┘
                                              │
        ┌───────────────┬────────────────────┼────────────────────┬───────────────┐
        │               │                    │                    │               │
        ▼               ▼                    ▼                    ▼               ▼
   TYPE DECLARE     OWNERSHIP/BIND      VISIBILITY/MOD       CONTROL FLOW     TRAITS/GENERICS
   struct enum      let mut ref         pub mod use           if else match    trait impl
   union fn type    move static         crate self Self       loop while for   dyn where
                     const              super extern as       in break         Self
                                                               continue return

        ┌───────────────┬────────────────────┬────────────────────┐
        │               │                    │                    │
        ▼               ▼                    ▼                    ▼
   UNSAFE/FFI       ASYNC                OPERATOR/CAST        LITERAL
   unsafe extern    async await          as (cast)             true false
   raw                                                          const (value)
```

Run the same four-step procedure from before on every keyword below:

1. **Which bucket** — where does it sit syntactically?
2. **Erasure test** — delete it, look at the generated code (MIR/LLVM IR/asm). Different?
3. **If "no"** — is this compile-time-only proof secretly buying back a runtime cost or capability?
4. **Anchor** — which C#/CLR concept is this replacing, and who proves it there instead (JIT heuristic vs. runtime check vs. GC)?

One structural note before the sections: several keywords are spelled once but mean two unrelated things depending on position — `as` (cast vs. import rename), `extern` (FFI ABI vs. the largely obsolete `extern crate`), `self`/`Self` (module path vs. method receiver vs. implementing type), `const` (item vs. `const fn` vs. const generic parameter). Same lesson as `new` and `using` in the C# guide: keyword ≠ concept.

---

## 1. Type & Data Declaration

### 1.1 `struct`

```rust
pub struct PacketHeader {
    pub version: u8,
    pub length: u16,
    pub checksum: u32,
}
```

Value type, stack-allocated by default, no hidden indirection — the direct analogue of C#'s `struct`, not `class`. There's no separate "reference type" keyword in Rust at all: everything is a value unless you explicitly put it behind `Box`/`Rc`/`Arc`/`&`. **Compile-time-only** in the sense that whether something ends up on the stack or heap is a property of how it's *used* (boxed or not), not of the `struct` keyword itself.

### 1.2 `enum`

```rust
pub enum ParseResult {
    Ok(Packet),
    Incomplete(usize),   // bytes still needed
    Err(ParseError),
}
```

This is the one where a same-named C# keyword means something structurally different. C#'s `enum` is a named integer. Rust's `enum` is a real sum type / tagged union — each variant can carry its own payload. Closer in spirit to C#'s `record` hierarchy (or a hand-rolled tagged union in C) than to C#'s `enum`. Layout is unspecified by default — conceptually a discriminant tag plus enough space for the largest variant — add `#[repr(u8)]`/`#[repr(C)]` if you need a guaranteed layout for FFI.

Plain C-style enums still exist too:
```rust
#[repr(u8)]
pub enum TcpState { Closed = 0, Listen = 1, Established = 4 }
```
This form *is* the direct C# `enum` analogue — pin the repr the same reason the C# guide told you to pin `byte` on hot-path enums.

### 1.3 `union` (brief)

```rust
#[repr(C)]
pub union RawValue { pub as_u32: u32, pub as_bytes: [u8; 4] }
```

C-style union — the compiler can't prove which field interpretation is currently valid, so reading a field requires `unsafe` (§6). Rare outside FFI/parsing code.

### 1.4 `fn` — and the part C#'s `delegate` section maps onto directly

```rust
fn checksum(data: &[u8]) -> u16 { /* ... */ 0 }

let f: fn(&[u8]) -> u16 = checksum;          // function pointer — 1 real word, resolved at compile time
let boxed: Box<dyn Fn(&[u8]) -> u16> = Box::new(checksum); // heap-allocated, vtable-dispatched
```

This single concept spans the entire compile-time → runtime spectrum the way no other Rust construct does: a bare named function has a **unique, zero-sized type** per function (nothing to erase — there was never anything there); coerced to a `fn` pointer it's a real, single-word runtime value; boxed as `dyn Fn` it becomes a genuine heap object with a vtable — the closest thing Rust has to C#'s `delegate`, multicast excluded (Rust has no built-in multicast; you'd hold a `Vec<Box<dyn Fn(...)>>` yourself).

### 1.5 `type`

```rust
type FlowKey = (u32, u16, u32, u16); // alias — zero runtime meaning, purely a compiler-facing name
```

Also used for associated types inside traits (§5). Both uses are 100% compile-time — no distinct runtime type is created, unlike C#'s `record`/`class` declarations which do create a real type.

---

## 2. Ownership, Binding & Aliasing

*This bucket doesn't exist in the C# guide — C# doesn't need it because the GC is doing this job for you at runtime. This is Rust's actual thesis, so it gets the deepest treatment.*

### 2.1 `let` / `mut`

```rust
let hdr = PacketHeader { version: 4, length: 1500, checksum: 0xDEAD };
// hdr.version = 6;              // compile error — hdr is not mutable

let mut hdr = PacketHeader { version: 4, length: 1500, checksum: 0xDEAD };
hdr.version = 6;                  // fine
```

`mut` compiles to **identical machine code** with or without it — it's a borrow-checker permission checked at every write site, then gone. This is worth sitting with, because there's genuinely no C# keyword on this list that maps to it: C# has no default-immutable local binding. Every C# local is implicitly what Rust would call `mut` unless the *type itself* is immutable (`readonly struct`, an immutable `record`). This is one of the rare cases where the honest answer is "C# doesn't have this," not "here's the equivalent" — the two languages disagree at the philosophy level, not just the syntax level.

### 2.2 `ref`

```rust
struct Connection { addr: String }
let maybe_conn: Option<Connection> = Some(Connection { addr: "10.0.0.1".into() });

match maybe_conn {
    Some(ref conn) => println!("addr: {}", conn.addr), // binds by reference; maybe_conn still usable after
    None => {}
}
// without `ref`, `Some(conn) => ...` would MOVE the Connection out of maybe_conn
```

Since the 2018 edition's "match ergonomics," explicit `ref` shows up far less — matching on `&Option<Connection>` directly lets the compiler infer by-reference bindings without you spelling it out. `ref` still matters when the scrutinee is owned and you want to avoid a move. 100% compile-time: it changes what the borrow checker will allow you to do with the binding afterward, not what instructions get emitted for the match itself.

### 2.3 `move`

```rust
let filter_name = String::from("geo-block");
let filter = move || println!("applying filter: {}", filter_name);
std::thread::spawn(filter); // requires F: Send + 'static — `move` is what makes this provable
```

Unlike most of this section, `move` is **not fully erased** — it changes what's physically stored inside the generated closure struct: an owned `String` inline versus a pointer to the enclosing stack frame. That's a real, visible difference in the closure's layout, which is exactly why `thread::spawn` (which needs the closure to outlive the current stack frame) requires it: a borrowing closure would leave a dangling reference the moment this function returns.

### 2.4 `const` vs. `static` — the exact C# pairing, same bug and all

```rust
pub const MAX_PACKET_SIZE: usize = 65535;   // inlined at every use site — no memory address at all
pub static PACKET_COUNTER_SEED: u64 = 0;    // one real, fixed memory location for the program's life
```

This is the C# guide's `const` vs. `static readonly` warning, transplanted almost unchanged: `const` is pasted into the call site at compile time — update it in one crate without recompiling a dependent crate, and the dependent keeps the old baked-in value, same mechanism as the C# DLL case. `static` gets a genuine, singular address. Rust's crate-recompilation model makes this easier to *avoid* hitting in practice than C#'s DLL-versioning story, but the underlying mechanism is identical. `static mut` exists too, and needs `unsafe` to touch — it's the one sanctioned way to get a genuinely aliased, mutable global past the borrow checker.

### 2.5 `'static` (lifetime, not the `static` keyword)

A reserved *lifetime* name, not the item keyword above — easy to conflate since they share six letters. Means "no borrow shorter than the whole program lives inside this." Fully compile-time; erased entirely once borrow-checking succeeds ("lifetime erasure" is a real, standard term — by the time you have a binary there is no lifetime information left anywhere). Shows up constantly in bounds like `F: Send + 'static` for exactly the reason `move` above needed it.

---

## 3. Visibility & Module System

```rust
mod filtering {
    pub struct GeoIpFilter;                 // visible outside `filtering`
    struct InternalLookupTable;              // visible to `filtering` and its descendant modules only
}
pub use filtering::GeoIpFilter;              // re-export
```

`pub(crate)`, `pub(super)`, `pub(in some::path)` refine this — a real parallel to C#'s `internal`/`protected internal`/`private protected` spectrum, though the default privacy boundary differs: Rust's default privacy is **module-scoped** (visible to the declaring module and its children), where C#'s `private` is **type-scoped**. A private field in Rust is visible to sibling code in the same module; a private field in C# is visible only inside the same class.

`mod`, `use`, `crate`, `super`, `self` (in paths) are all pure compile-time path resolution — no runtime representation whatsoever. This is actually a *cleaner* "100% erased" story than C#'s access modifiers: C# still has to guard against reflection peeking at `private` members at runtime (`MemberAccessException` exists for a reason). Rust has no general runtime reflection, so there's nothing for `pub`/privacy to guard against once compilation succeeds — it really is fully gone.

`extern` gets its real treatment in §6 (FFI); the `extern crate` form is largely obsolete since 2018.

---

## 4. Control Flow

### 4.1 `match` — and where it beats C#'s `switch`

```rust
let desc = match state {
    TcpState::Closed => "no connection",
    TcpState::Listen => "awaiting SYN",
    TcpState::Established => "data flowing",
    _ => "transitional",
};
```

Both languages check exhaustiveness at compile time — but C#'s switch *expression* only **warns** (CS8509) on a gap and inserts a runtime `SwitchExpressionException` for the unhandled case if you ignore the warning. Rust's `match` **refuses to compile** (E0004) unless it's exhaustive or has a `_` arm. Same proof, promoted from an optional warning to a mandatory gate — which is also why Rust code rarely needs a runtime "how did we get here" panic path for its matches the way ignored C# warnings do.

### 4.2 `loop` / `while` / `for` / `in`, `break`, `continue`, `return`

```rust
'outer: for i in 0..10 {
    for j in 0..10 {
        if i * j > 50 { break 'outer; }
    }
}

let result = loop {
    if let Some(v) = try_compute() { break v; } // `loop` can YIELD a value; while/for always yield ()
};
```

Labeled `break`, and `break` carrying a value out of a `loop`, are two small but real Rust-specific facts worth having cold — `while`/`for` can't do the latter since they don't guarantee the body ran at all.

---

## 5. Traits, Generics & Dispatch — the flagship section

### 5.1 `trait` / `impl Trait for Type`

```rust
pub trait PacketFilter {
    fn should_allow(&self, packet: &[u8]) -> bool;
}
pub struct GeoIpFilter;
impl PacketFilter for GeoIpFilter {
    fn should_allow(&self, packet: &[u8]) -> bool { true }
}
```

Contract, no state — the direct `interface` analogue. Purely compile-time as a *declaration*; how it gets dispatched is decided at each call site, which is the actual interesting question (below).

### 5.2 `dyn Trait` — the one keyword in this guide that's genuinely runtime-dispatched

```rust
pub fn build_pipeline() -> Vec<Box<dyn PacketFilter>> {
    vec![Box::new(GeoIpFilter), Box::new(RateLimiterFilter::default())]
}
for filter in &pipeline {
    if !filter.should_allow(packet) { return false; } // real indirect call, every time
}
```

```
Box<dyn PacketFilter>   ("fat pointer" — two words)
┌───────────────┬───────────────┐
│ data pointer    │ vtable pointer  │
└───────┬───────┴───────┬───────┘
        ▼                       ▼
  GeoIpFilter{}          [ should_allow fn ptr ]
  (heap-allocated)       [ drop fn ptr, size, align ]
```

This is Rust's `virtual`/`override`: a genuine vtable, a genuine indirect call the compiler cannot inline away. It's the counter-example to Rust's usual reputation, and worth remembering as such — the one place the language spends a real runtime cycle instead of a compile-time proof, and it does so deliberately, because sometimes you need one function to accept many unrelated concrete types at runtime (a plugin list, a heterogeneous collection) and no amount of compile-time proof can substitute for that.

### 5.3 Generics / `impl Trait` (arg or return position) — the static-dispatch alternative

```rust
pub fn run_filter<F: PacketFilter>(filter: &F, packet: &[u8]) -> bool {
    filter.should_allow(packet) // resolved at compile time, likely inlined — no vtable at all
}
```

The compiler generates a **separate specialized copy** of `run_filter` per concrete `F` actually used (monomorphization) — zero dispatch cost at runtime, paid for instead in binary size and compile time. This is precisely the C# guide's "`sealed` enables devirtualization" story, except it isn't a JIT's best-effort heuristic that might fire — generics make static dispatch the default and the guarantee.

### 5.4 `where`

```rust
pub fn run_filter<F>(filter: &F, packet: &[u8]) -> bool where F: PacketFilter { /* ... */ }
```

Same keyword, same job, same job title as C#'s `where T : constraint` — purely a readability alternative to inline bounds, zero semantic difference, 100% compile-time.

### 5.5 `Self` vs. `self`

```rust
impl GeoIpFilter {
    pub fn new(cidr: &str) -> Self { GeoIpFilter }         // Self = compile-time type alias, erased
    pub fn should_allow(&self, packet: &[u8]) -> bool { true } // &self = a real pointer, passed at the real call
}
```

Same-spelling, different-bucket problem again: `Self` (capital) is a pure compile-time type substitution with no runtime trace. `self` (lowercase) as a method's receiver parameter is the actual runtime value the call operates on — Rust's explicit version of C#'s implicit `this`.

---

## 6. Unsafe & FFI

### 6.1 `unsafe` — the keyword that runs the whole framework backwards

```rust
pub unsafe fn parse_raw(buffer: *const u8, len: usize) -> u16 {
    // caller-upheld invariant: buffer points to >= len valid, initialized bytes
    *(buffer as *const u16)
}
```

`unsafe` itself has **zero runtime footprint** — entering an unsafe block emits no instruction, no flag, nothing. It's a pure compile-time declaration: "I have manually proven what rustc can't." Everywhere else in this guide, a compile-time proof *adds* a runtime guarantee for free. This is the one place it runs in reverse:

```rust
let v = vec![1, 2, 3];
v.get(5);                              // safe: real runtime bounds check, returns None
unsafe { v.get_unchecked(5) };         // unsafe: NO bounds check emitted — UB if wrong, genuinely
                                        // cheaper at runtime, because your manual proof replaced
                                        // the check instead of buying you one
```

Sit with this one — it's the cleanest possible illustration that "compile-time vs. runtime" was never really about which is *better*, only about who's doing the proving.

### 6.2 `extern "C" fn` — real ABI consequences, not just a label

```rust
#[no_mangle]
pub extern "C" fn socket_open(domain: i32, ty: i32, proto: i32) -> i32 { 0 }

extern "C" { fn socket(domain: i32, ty: i32, protocol: i32) -> i32; } // declare, like C#'s DllImport
```

Direct counterpart to the C# guide's `[DllImport]`/`extern` section — this genuinely changes the calling convention the generated code uses, not just what the compiler will let you write.

### 6.3 `raw` (weak keyword, brief)

`&raw const place` / `&raw mut place` — takes a raw pointer to a place without creating an intermediate `&`/`&mut` reference, for cases where even a momentary reference would itself violate aliasing rules (packed-struct fields, building a pointer into a value already behind a raw pointer). Niche, but real, and squarely in your territory.

---

## 7. Async

```rust
pub async fn receive_packet(socket: &mut TcpStream) -> std::io::Result<Vec<u8>> {
    let mut buf = vec![0u8; 1500];
    let n = socket.read(&mut buf).await?;
    buf.truncate(n);
    Ok(buf)
}
```

Mechanically the same idea as C#'s `async`/`await`: the compiler rewrites this into a state machine, here one implementing `Future<Output = io::Result<Vec<u8>>>`. The sharp divergence: **there is no built-in executor**. A bare `Future` does nothing until something polls it — no thread pool, no epoll/IOCP wiring exists in the language or in `std` by default. That's the job of an external crate (tokio, async-std, embassy for `no_std`). C#'s `Task` ships bundled with the BCL's thread pool and I/O-completion machinery baked in; Rust's `Future` is a trait with a `poll` method and nothing else — you bring your own reactor. Worth being precise about, since it's the single biggest practical difference between two otherwise near-identical compiler transformations.

---

## 8. Operators & Casting

```rust
let big: u64 = 300;
let small = big as u8;                       // RUNTIME truncation: 300 % 256 = 44 — a real instruction

use std::collections::HashMap as Map;        // same keyword, 100% compile-time local alias, erased
```

Same spelling, two different buckets, same lesson as `is` in the C# guide's pattern-matching section: `as`-the-cast changes what the CPU actually does; `as`-the-import-rename is gone by the time you have a binary. `true`/`false` are plain literal keywords — no dedicated section needed, listed in the table below alongside `const` used as a value rather than an item.

---

## 9. Full Reserved Keyword Reference

### Strict keywords (2015 edition)
```
as        break     const     continue  crate     else      enum
extern    false     fn        for       if        impl      in
let       loop      match     mod       move      mut       pub
ref       return    self      Self      static    struct    super
trait     true      type      unsafe    use       where     while
```

### Strict keywords added in the 2018 edition
```
async     await     dyn
```
These were added via Rust's **edition** mechanism specifically so any pre-2018 code already using `async`, `await`, or `dyn` as ordinary identifiers wouldn't break when compiled under an older edition. This is the exact same engineering instinct the C# guide called out for contextual keywords like `where`/`async`/`var` — "when extending a stable public interface, prefer additions that can't break existing valid usage" — except Rust's edition system solves it a level up: the keyword-set itself is versioned per-crate, rather than every keyword individually having to stay "contextual" forever.

### Reserved for future use (not usable as identifiers; some have unstable/nightly-only behavior today)
```
abstract  become    box       do        final     macro     override
priv      try       typeof    unsized   virtual   yield
```
`try` is reserved for an eventual `try {}` block expression, currently nightly-only behind a feature gate. `yield` is reserved for a hypothetical generator/coroutine feature, likewise nightly-only. The rest currently do nothing at all — reserved purely to keep the door open.

### Weak / contextual keywords
```
union     macro_rules   raw       dyn*
```
Only special in specific syntactic positions — `union` is a completely ordinary identifier everywhere except right before a type declaration that looks like a union.

---

## 10. A Realistic Production Example

A lock-free token-bucket limiter — same problem as the C# guide's example, deliberately solved without a single `lock`, to show off the ownership/atomics idioms this bucket's section was building toward.

```rust
use std::sync::atomic::{AtomicI64, AtomicU64, Ordering};
use std::time::Instant;

pub struct TokenBucketLimiter {
    capacity: i64,
    refill_per_sec: i64,
    start: Instant,
    tokens: AtomicI64,
    last_refill_nanos: AtomicU64,
}

impl TokenBucketLimiter {
    pub fn new(capacity: i64, refill_per_sec: i64) -> Self {
        Self {
            capacity,
            refill_per_sec,
            start: Instant::now(),
            tokens: AtomicI64::new(capacity),
            last_refill_nanos: AtomicU64::new(0),
        }
    }

    pub fn try_acquire(&self, cost: i64) -> bool {
        self.refill();
        let mut current = self.tokens.load(Ordering::Acquire);
        loop {
            if current < cost {
                return false;
            }
            match self.tokens.compare_exchange_weak(
                current, current - cost, Ordering::AcqRel, Ordering::Acquire,
            ) {
                Ok(_) => return true,
                Err(actual) => current = actual,
            }
        }
    }

    fn refill(&self) {
        let now_nanos = self.start.elapsed().as_nanos() as u64;
        let last = self.last_refill_nanos.load(Ordering::Acquire);
        if now_nanos <= last {
            return;
        }
        let elapsed_secs = (now_nanos - last) as f64 / 1_000_000_000.0;
        let refill_amount = (elapsed_secs * self.refill_per_sec as f64) as i64;
        if refill_amount <= 0 {
            return;
        }
        if self
            .last_refill_nanos
            .compare_exchange(last, now_nanos, Ordering::AcqRel, Ordering::Acquire)
            .is_ok()
        {
            let mut current = self.tokens.load(Ordering::Acquire);
            loop {
                let new_val = (current + refill_amount).min(self.capacity);
                match self.tokens.compare_exchange_weak(
                    current, new_val, Ordering::AcqRel, Ordering::Acquire,
                ) {
                    Ok(_) => break,
                    Err(actual) => current = actual,
                }
            }
        }
    }
}
```

Notice what's *absent*: no `mut` anywhere in the public API (`try_acquire(&self, ...)`, not `&mut self`), despite this being a genuinely mutating, thread-shared rate limiter. That's interior mutability via the `Atomic*` types — the borrow checker's usual "one `&mut` or many `&`" rule is satisfied because the mutation happens through a type designed to be safely aliased, not because the rule was bent. This is exactly the kind of thing that trips up anyone whose mental model of `mut` is "if it changes, the binding must say `mut`" — the binding doesn't need to, because the *type* is doing the work instead.

---

## 11. Cross-Reference — Same Problem, C# vs. Rust

| Problem | C# answer | Rust answer | Who proves it, and when |
|---|---|---|---|
| Prevent aliased mutation | discipline / `lock` | borrow checker (`&` vs `&mut`, `mut`) | Rust: compiler, ahead of time. C#: you, at runtime, if you remember |
| Dynamic dispatch | `virtual`/`override` (vtable) | `dyn Trait` (vtable) | Both: runtime — the one place they agree |
| Guaranteed static dispatch | `sealed` *enables* JIT devirtualization (heuristic) | generics/`impl Trait` *force* monomorphization | Rust: compiler, guaranteed. C#: JIT, best-effort |
| Deterministic cleanup | `using`/`IDisposable`, must remember to write it | `Drop` trait, runs automatically at scope exit | Rust: compiler-inserted, can't forget. C#: your responsibility |
| Keep the GC from moving a buffer | `fixed` (pin, temporarily) | nothing needed — no GC to move it in the first place | Rust deletes the problem instead of solving it |
| Prevent overflow-driven buffer bugs | `checked` block (opt-in) | arithmetic panics in debug builds by default; explicit `wrapping_*`/`checked_*` methods otherwise | Rust: default-on in debug, explicit everywhere else |
| Generic constraint on a type parameter | `where T : IFoo` | `where T: Foo` | Identical keyword, identical job |

---

## 12. How to Actually Retain This

1. For every keyword: **(a)** what does rustc prove or reject at compile time, **(b)** what runtime check, allocation, lock, or dispatch does that proof delete? Most keywords here have a clean answer to (b). The ones where (b) is instead "nothing is deleted, something is added" — `dyn`, `as`-the-cast, any `Box`/heap allocation — are the exceptions, and they're the ones fighting the language's own grain; find those first.
2. Map every keyword to the runtime check it lets a *different* language skip having in the first place: `mut`/borrowing → no lock, no runtime alias check; lifetimes/`'static` → no GC, no refcounting, no use-after-free check; `match` exhaustiveness → no default-case runtime throw; generics/monomorphization → no vtable lookup. This sticks better than memorizing borrow-checker rules as isolated syntax.
3. Write from memory tomorrow, no reference, the 5 keywords with the thinnest C#/mainstream analogue: `move`, the `dyn` vs. generic/`impl Trait` split, `'static` as a bound (not the `static` item), match ergonomics' effect on `ref`, and the fact that `unsafe` *removes* a check rather than adding one.
4. Rebuild the §10 example changing one thing at a time: drop `unsafe` but keep a raw-pointer deref elsewhere and watch it refuse to compile; swap the `dyn PacketFilter` pipeline for a generic and watch dispatch become direct while binary size grows; remove `move` from a spawned closure and watch the `'static` bound fail. Reason through exactly which guarantee moved.
