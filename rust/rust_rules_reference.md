# Rust — Complete Rules Reference

Organized by concept. Framed for systems programmers who already know C and Go.

---

## 1. Ownership Rules

**Rule O-1.** Every value has exactly one owner at any given moment. Ownership is a compile-time concept, not a runtime mechanism.

**Rule O-2.** When the owner goes out of scope, the value is dropped. The compiler inserts the `drop` call automatically. There is no GC, no `free()`, no destructor you must remember to call.

**Rule O-3.** Ownership is transferred (moved) by default on assignment, function argument passing, and return from a function — for types that do not implement `Copy`.

**Rule O-4.** After a move, the original binding is invalidated at compile time. Attempting to use it is a compile error, not undefined behavior. The compiler catches this before any binary is produced.

**Rule O-5.** Ownership transfer on function arguments means the function becomes the new owner. If you want the caller to retain ownership, you must pass a reference or clone the value.

**Rule O-6.** Returning a value from a function transfers ownership to the caller.

**Rule O-7.** Drop is called on local variables in reverse order of their declaration when the scope ends. Drop is called on struct fields in the order they are declared in the struct.

**Rule O-8.** You cannot manually call `drop()` as a method. You use `std::mem::drop(value)` to force an early drop. After that call, the binding is moved and invalid.

**Rule O-9.** If a type implements `Drop`, you cannot move individual fields out of an instance of that type. The whole value must be consumed together.

**Rule O-10.** `std::mem::forget(value)` takes ownership without calling drop. Use only in FFI or specialized memory management. This is safe in Rust's memory model but may leak resources.

**Rule O-11.** `ManuallyDrop<T>` wraps a value and prevents automatic drop. You must call `ManuallyDrop::drop()` explicitly when ready. Used in low-level allocators and FFI.

---

## 2. Move Semantics Rules

**Rule MV-1.** Assignment of a non-`Copy` type moves the value. The source binding becomes uninitialized and unusable.

**Rule MV-2.** Passing a non-`Copy` value to a function moves it into the function. The original binding is gone after the call.

**Rule MV-3.** The `move` keyword on a closure forces the closure to take ownership of all variables it captures from the enclosing scope, rather than borrowing them.

**Rule MV-4.** Struct initialization by value moves each non-`Copy` field from its source into the struct.

**Rule MV-5.** Struct update syntax `MyStruct { field: new_val, ..other }` moves the remaining fields out of `other`. After this, `other` is partially moved and cannot be used as a whole, but individual fields not moved from it may still be accessible (if not moved).

**Rule MV-6.** Pattern destructuring in `let` bindings and `match` arms can move values out of a compound type if the match arm does not use `ref`.

**Rule MV-7.** Returning a local variable typically results in a move to the caller. The compiler applies Return Value Optimization (RVO / NRVO) to elide the actual copy in many cases, but the ownership semantics are still a move.

---

## 3. Copy Trait Rules

**Rule CP-1.** A type implements `Copy` if and only if it can be duplicated by copying its raw bits — no heap allocation, no file handles, no resources that require cleanup.

**Rule CP-2.** If a type implements `Copy`, assignment duplicates the value. Both the source and destination bindings remain valid.

**Rule CP-3.** A type can implement `Copy` only if all of its fields also implement `Copy`.

**Rule CP-4.** A type that implements `Drop` cannot implement `Copy`. These two traits are mutually exclusive. If your type needs custom cleanup, it cannot be `Copy`.

**Rule CP-5.** Types that are `Copy` by default in std: all primitive integers (`i8`..`i64`, `u8`..`u64`, `i128`, `u128`, `isize`, `usize`), floats (`f32`, `f64`), `bool`, `char`, shared references `&T` (regardless of whether `T` is `Copy`), raw pointers `*const T` and `*mut T`, arrays `[T; N]` where `T: Copy`, tuples where all elements are `Copy`.

**Rule CP-6.** Types that are NOT `Copy`: `String`, `Vec<T>`, `Box<T>`, `Rc<T>`, `Arc<T>`, `File`, `MutexGuard`, `&mut T`, and any type with a custom `Drop` impl.

**Rule CP-7.** `Clone` is a supertrait of `Copy`. Every `Copy` type must also implement `Clone`. `Clone` is the explicit version (calling `.clone()`); `Copy` is the implicit version (assignment).

**Rule CP-8.** You can derive both: `#[derive(Copy, Clone)]`. The derive for `Clone` on a `Copy` type is implemented trivially by copying bits.

---

## 4. Borrowing Rules

**Rule BR-1.** The fundamental borrow rule: at any given point in the program, for a given value, you may have either **one mutable reference** (`&mut T`) or **any number of shared references** (`&T`), but never both simultaneously. This is enforced at compile time.

**Rule BR-2.** `&T` is a shared reference. Multiple `&T` references to the same value can exist at the same time. `&T` does not permit mutation of the pointed-to value (except through interior mutability).

**Rule BR-3.** `&mut T` is an exclusive reference. While an `&mut T` reference exists to a value, no other reference (shared or exclusive) to that same value may exist. Aliasing is prohibited.

**Rule BR-4.** A reference must not outlive the value it refers to. The compiler enforces this through lifetime analysis.

**Rule BR-5.** You cannot move a value while a reference to it is alive (shared or exclusive).

**Rule BR-6.** You cannot mutate a value through a shared reference `&T` unless the value uses interior mutability (`Cell<T>`, `RefCell<T>`, `Mutex<T>`, `UnsafeCell<T>`).

**Rule BR-7.** Reborrowing: you can create a new reference from an existing reference. `&*r` reborrowing a `&mut T` gives a `&T`. Reborrowing `&mut *r` gives a new `&mut T` but temporarily suspends the original. This is used in function calls to avoid giving away the `&mut T` permanently.

**Rule BR-8.** Non-Lexical Lifetimes (NLL): the borrow checker tracks the actual last use of a reference, not the end of its syntactic scope. A borrow ends at its last use, not at the closing brace of its enclosing block.

**Rule BR-9.** Disjoint field borrows: you can hold multiple `&mut` references to different fields of the same struct simultaneously, because the borrow checker knows they are non-overlapping. You cannot do this through methods without explicit splitting.

**Rule BR-10.** `split_at_mut()` on slices is the standard pattern for getting two non-overlapping `&mut` slices from a single slice. The compiler cannot prove disjoint indexing through raw index operations, so you need the API.

---

## 5. Lifetime Rules

**Rule LT-1.** Every reference has a lifetime. A lifetime is a region of code during which the reference is valid. Most lifetimes are inferred; you name them explicitly only when needed.

**Rule LT-2.** Lifetime parameters are written with a leading apostrophe: `'a`. They are declared in angle brackets alongside type parameters: `fn foo<'a>(x: &'a str) -> &'a str`.

**Rule LT-3.** A lifetime annotation does not change how long a value lives. It describes a relationship between lifetimes that already exist. It is a constraint on the caller, not a mechanism that extends storage duration.

**Rule LT-4.** `'static` is the longest lifetime. A `'static` reference is valid for the entire duration of the program. String literals are `&'static str`. Global constants have `'static` lifetime.

**Rule LT-5.** You cannot return a reference to a local variable. The local is dropped when the function returns; the reference would dangle. The compiler rejects this. Return an owned value or ensure the data lives in the caller.

**Rule LT-6.** Lifetime elision rules (applied in order when lifetimes in function signatures are omitted):
- Each omitted lifetime in input position gets its own distinct lifetime parameter.
- If there is exactly one input lifetime (after applying rule 1), all omitted output lifetimes are assigned that lifetime.
- If `&self` or `&mut self` is one of the inputs, all omitted output lifetimes are assigned the lifetime of `self`.
If none of these rules produces a unique determination, the compiler requires explicit annotation.

**Rule LT-7.** Structs that contain references must have lifetime annotations declaring that the struct cannot outlive the references it holds.

```rust
struct Packet<'a> {
    data: &'a [u8],    // Packet cannot outlive the buffer it references
}
```

**Rule LT-8.** Lifetime subtyping: `'long: 'short` means `'long` outlives `'short`. A reference with a longer lifetime can be used where a shorter lifetime is expected. This is covariance.

**Rule LT-9.** Variance rules:
- `&'a T` is covariant over `'a`: a `&'long T` can be used where a `&'short T` is needed.
- `&'a mut T` is invariant over `'a` and over `T`: you cannot substitute a longer lifetime for a shorter one, or vice versa. This prevents use-after-free through mutable aliasing.
- Function types `fn(T) -> U` are contravariant over their argument types and covariant over their return types.
- `PhantomData<T>` inherits the variance of `T`. Use `PhantomData<*mut T>` to make a type invariant over `T` when holding raw pointers.

**Rule LT-10.** Higher-Ranked Trait Bounds (HRTBs): `for<'a> F: Fn(&'a T) -> &'a U` means "for any lifetime `'a`". Used when a closure or function must work with references of any lifetime.

**Rule LT-11.** The anonymous lifetime `'_` tells the compiler to infer the lifetime. Useful in `impl` blocks and type aliases to suppress redundant annotation while still acknowledging that a lifetime exists.

---

## 6. Type System Rules

**Rule TS-1.** Rust is statically and strongly typed. Types are known at compile time. There is no implicit conversion between types.

**Rule TS-2.** Integer literals default to `i32`. Floating-point literals default to `f64`. You must cast explicitly with `as` or convert with `From`/`Into` traits.

**Rule TS-3.** The `as` cast: performs numeric conversions and pointer casts. Truncates on integer narrowing. Does not check for overflow in release mode. Use with awareness of what bits get dropped.

**Rule TS-4.** Integer overflow in debug builds causes a panic. In release builds, it wraps (two's complement). Use `checked_*`, `saturating_*`, `wrapping_*`, or `overflowing_*` methods for explicit behavior regardless of build mode.

**Rule TS-5.** Structs are product types. All fields must be initialized at construction. There are no default field values unless you implement `Default`.

**Rule TS-6.** Enums are sum types (tagged unions). A value is exactly one variant at a time. The discriminant is automatically managed. `match` on an enum is exhaustive — all variants must be handled.

**Rule TS-7.** Tuple structs: `struct Point(f64, f64)`. Fields accessed by index `.0`, `.1`. Useful for newtype patterns.

**Rule TS-8.** The newtype pattern: `struct Meters(f64)`. Creates a distinct type from an inner type. Enables type-safe distinctions that the compiler enforces, with zero runtime overhead.

**Rule TS-9.** Unit structs: `struct Marker;`. No fields, zero size. Used as type-level markers, commonly with traits.

**Rule TS-10.** `Sized` is a marker trait. By default, all generic type parameters have an implicit `T: Sized` bound. To work with dynamically sized types (DSTs), use `T: ?Sized` to opt out of this requirement. DSTs include `str`, `[T]`, and trait objects `dyn Trait`.

**Rule TS-11.** Type aliases (`type Alias = SomeType;`) do not create new types. The alias and the original type are interchangeable. For a new type with enforced distinction, use the newtype pattern.

**Rule TS-12.** Union types share the same memory for all fields. Reading a field is always `unsafe` because the compiler cannot know which variant is active. Primarily used for FFI with C unions.

---

## 7. Trait Rules

**Rule TR-1.** Orphan rule: you may implement a trait for a type only if the trait is defined in your crate or the type is defined in your crate (or both). You cannot implement a foreign trait for a foreign type. This ensures coherence: at most one implementation of a trait for any type, globally.

**Rule TR-2.** Coherence: there can only be one `impl Trait for Type` for any specific combination of trait and type. Overlapping implementations are rejected by the compiler.

**Rule TR-3.** Blanket implementations: `impl<T: Display> ToString for T` implements `ToString` for every type that implements `Display`. These are legal but interact with coherence — a blanket impl for a foreign trait or foreign type can conflict with local impls.

**Rule TR-4.** Default implementations: a trait can provide default method bodies. Types implementing the trait may override them or inherit the defaults.

**Rule TR-5.** Supertraits: `trait A: B` means any type implementing `A` must also implement `B`. You can call `B`'s methods on `A`'s `self`.

**Rule TR-6.** Object safety rules (required for `dyn Trait`):
- Methods must not have generic type parameters.
- Methods must not use `Self` as a return type (except in `Box<Self>` or similar).
- Methods must not require `Self: Sized`.
- The trait must not have associated constants (in most cases).
- All methods must be dispatchable through a vtable.

**Rule TR-7.** Static dispatch (`impl Trait` or generic bounds): the compiler generates a separate copy of the code for each concrete type. Zero overhead, no vtable. Creates code bloat if used with many types.

**Rule TR-8.** Dynamic dispatch (`dyn Trait`): uses a vtable (fat pointer: data pointer + vtable pointer). One compiled version of the code, dispatch at runtime. Small overhead per call. Necessary when the concrete type is not known at compile time.

**Rule TR-9.** `impl Trait` in function argument position is syntactic sugar for an anonymous generic: `fn f(x: impl Display)` means `fn f<T: Display>(x: T)`. Each call site must use the same concrete type.

**Rule TR-10.** `impl Trait` in return position (RPIT): `fn f() -> impl Trait` means the function returns some type that implements the trait, but the exact type is hidden. The concrete type is fixed per function; two call sites get the same underlying type.

**Rule TR-11.** `dyn Trait` must be behind a pointer (`&dyn Trait`, `Box<dyn Trait>`, `Arc<dyn Trait>`) because the size of the concrete type is not known at compile time.

**Rule TR-12.** Associated types: a trait defines a placeholder type that implementors must specify. `type Item;` in the trait, `type Item = u8;` in the `impl`. Cleaner than a type parameter when the type is uniquely determined by the implementation. See: `Iterator::Item`, `Deref::Target`.

**Rule TR-13.** `From<T>` and `Into<T>` are reflexive: every type implements `From<T>` for itself. If you implement `From<A> for B`, you get `Into<B> for A` for free. Implement `From`, consume `Into`.

---

## 8. Generics Rules

**Rule GN-1.** Generic type parameters (`fn f<T>(x: T)`) are resolved at compile time through monomorphization. Each distinct concrete type used with the generic produces a separate compiled function. Zero runtime overhead.

**Rule GN-2.** Trait bounds constrain generics: `fn f<T: Clone + Debug>(x: T)`. The function can only be called with types that implement all specified traits.

**Rule GN-3.** `where` clauses: for complex bounds, move them to a `where` clause for readability.

```rust
fn f<T, U>(x: T, y: U) -> T
where
    T: Clone + Debug,
    U: Into<T>,
{ ... }
```

**Rule GN-4.** Const generics: `struct Buffer<const N: usize>([u8; N])`. The const parameter is part of the type. Different values of `N` produce different types. Operations between `Buffer<4>` and `Buffer<8>` are compile errors.

**Rule GN-5.** Turbofish syntax `::<>` provides explicit type parameters when the compiler cannot infer them: `Vec::<u8>::new()`, `"5".parse::<i32>()`.

**Rule GN-6.** Phantom type parameters (`PhantomData<T>`): when a struct logically owns a `T` but does not actually store a `T`, use `PhantomData<T>` to inform the type system and drop checker about the relationship. Needed in unsafe code with raw pointers.

**Rule GN-7.** `T: 'static` bound means `T` contains no references shorter than `'static`. It does not mean `T` must live forever; it means it can, if needed. Required by `std::thread::spawn` for data passed to threads.

---

## 9. Memory Model and Smart Pointer Rules

**Rule MM-1.** Stack allocation requires size known at compile time. All local variables of known size are stack-allocated by default.

**Rule MM-2.** `Box<T>` allocates `T` on the heap and owns it. When the `Box` is dropped, the heap allocation is freed. Zero overhead versus a raw C heap pointer; the overhead is in the abstraction layer, not the allocation.

**Rule MM-3.** `Box<T>` derefs to `&T` and `&mut T` automatically via `Deref` and `DerefMut`. You rarely need to write `*box_val` explicitly.

**Rule MM-4.** `Rc<T>` is a reference-counted pointer for single-threaded shared ownership. Clone increments the count; drop decrements it. When count reaches zero, the value is dropped.

**Rule MM-5.** `Rc<T>` is NOT `Send` and NOT `Sync`. It cannot be used across thread boundaries. Use `Arc<T>` for multithreaded shared ownership.

**Rule MM-6.** `Arc<T>` uses atomic reference counting. It is `Send + Sync` if `T: Send + Sync`. The atomics make it more expensive than `Rc`; only use `Arc` when you need thread safety.

**Rule MM-7.** `Rc` and `Arc` can create reference cycles, causing memory leaks because the count never reaches zero. Use `Weak<T>` (`Rc::downgrade`, `Arc::downgrade`) for back-references to break cycles.

**Rule MM-8.** `Cell<T>` allows interior mutability for `Copy` types through a shared reference. `get()` copies out, `set()` replaces the value. No borrow checking at runtime; the restriction is that only `Copy` types are allowed (so there is no aliasing of a non-Copy value).

**Rule MM-9.** `RefCell<T>` allows interior mutability for non-`Copy` types. Enforces borrow rules at runtime. `borrow()` panics if an exclusive borrow is active. `borrow_mut()` panics if any borrow (shared or exclusive) is active. Use in single-threaded contexts only.

**Rule MM-10.** `Mutex<T>` provides exclusive access to `T` across threads. `lock()` blocks until the lock is acquired. Returns a `MutexGuard<T>` which releases the lock when dropped.

**Rule MM-11.** Mutex poisoning: if a thread panics while holding a `MutexGuard`, the mutex is marked poisoned. Subsequent `lock()` calls return `Err(PoisonError)`. You can call `.into_inner()` on the error to recover the guard and continue.

**Rule MM-12.** `RwLock<T>` allows multiple simultaneous readers or one exclusive writer, never both. `read()` for shared access, `write()` for exclusive. Same poisoning behavior as `Mutex`.

**Rule MM-13.** `UnsafeCell<T>` is the primitive that all interior mutability types are built on. It is the only legal way to obtain a `*mut T` from a shared `&T`. Wrapping a type in `UnsafeCell` opts out of the rule that shared references imply immutability.

**Rule MM-14.** `MaybeUninit<T>` is how you work with potentially uninitialized memory safely. Never transmute uninitialized memory to `T` directly; use `MaybeUninit::assume_init()` after you have guaranteed initialization.

---

## 10. Concurrency Rules

**Rule CC-1.** `Send` is a marker trait. A type is `Send` if it is safe to transfer ownership to another thread. The compiler automatically implements `Send` for types whose fields are all `Send`.

**Rule CC-2.** `Sync` is a marker trait. A type `T` is `Sync` if `&T` is `Send` — meaning it is safe to share a reference to `T` across threads. Formally: `T: Sync` iff `&T: Send`.

**Rule CC-3.** Types that are NOT `Send`: `Rc<T>`, raw pointers `*const T` and `*mut T` (by default), `MutexGuard<T>` (cannot send a held lock to another thread), `Rc<RefCell<T>>`.

**Rule CC-4.** Types that are NOT `Sync`: `Cell<T>`, `RefCell<T>`, `Rc<T>`, `UnsafeCell<T>` (the primitive). `Mutex<T>` is `Sync` (you share a reference to the mutex, which is the point).

**Rule CC-5.** `Send` and `Sync` are unsafe traits. Implementing them manually bypasses the compiler's automatic derivation. You assert that your type upholds thread-safety invariants by hand.

**Rule CC-6.** `std::thread::spawn` requires the closure to be `Send + 'static`. `'static` means no references to stack-local data (they would be freed while the thread runs). `Send` means all captured data is safe to transfer.

**Rule CC-7.** `std::thread::scope` creates a scope in which spawned threads can borrow non-`'static` data from the enclosing scope. The scope blocks until all threads in it complete, guaranteeing the stack data outlives all borrows.

**Rule CC-8.** Atomics (`std::sync::atomic`): lock-free, low-overhead shared state. The memory ordering parameter (`Ordering`) controls synchronization guarantees:
- `Relaxed`: no synchronization, only atomicity. For counters with no dependencies.
- `Acquire`: synchronizes with a `Release` on the same variable. Ensures that operations before the `Release` are visible after the `Acquire`.
- `Release`: pairs with `Acquire`.
- `AcqRel`: both `Acquire` and `Release`. For read-modify-write operations.
- `SeqCst`: total sequential consistency across all threads and all atomic variables. Most expensive. Use when you need a global ordering guarantee.

**Rule CC-9.** `std::sync::mpsc`: multiple-producer, single-consumer channel. `Sender<T>` is `Send + Clone`; `Receiver<T>` is `Send` but not `Clone`. If `T` is not `Send`, the channel cannot be used across threads.

---

## 11. Async / Await Rules

**Rule AW-1.** `async fn` and `async { }` blocks return a `Future`. A `Future` is a value representing a computation that may not have completed yet. The future does nothing until it is polled.

**Rule AW-2.** `.await` on a future resumes it. If the future is not ready, the current task yields control to the executor and will be woken up when the future is ready to make progress.

**Rule AW-3.** You must be inside an `async` context to use `.await`. You cannot call `.await` in a synchronous function. Use `block_on` from an executor runtime to bridge synchronous code.

**Rule AW-4.** An async runtime (executor) is required. `tokio`, `async-std`, and `smol` are the main ones. The standard library provides the `Future` and `Waker` infrastructure, not the executor itself.

**Rule AW-5.** Async tasks sent to a multi-threaded executor must have all types across `.await` points be `Send`. The task may migrate between OS threads at each yield point, so all state must be safe to send.

**Rule AW-6.** Never block inside `async` code with synchronous blocking calls (`std::thread::sleep`, synchronous I/O, `Mutex::lock` on a highly contended mutex). This blocks the OS thread the executor is running on, starving other tasks. Use async-aware equivalents (`tokio::time::sleep`, `tokio::sync::Mutex`).

**Rule AW-7.** `Pin<P>` prevents moving the value behind pointer `P` after it has been pinned. Async state machines are self-referential (a future may hold a pointer to one of its own fields) and must not be moved after construction. The `Pin` type enforces this.

**Rule AW-8.** `Unpin` is a marker trait. A type is `Unpin` if it is safe to move even after being pinned. Most types are `Unpin`. A type becomes `!Unpin` by containing a `PhantomPinned` field or a field that is `!Unpin`.

**Rule AW-9.** To create a future that must be pinned: `Box::pin(future)` or `pin_mut!(future)` (from `pin-utils`) or `std::pin::pin!()` (stable since Rust 1.68).

**Rule AW-10.** `select!` macro: poll multiple futures and proceed with whichever completes first. Remaining futures are dropped. Be aware of cancellation safety — not all futures are safe to cancel mid-computation.

---

## 12. Error Handling Rules

**Rule EH-1.** `Option<T>` is `Some(T)` or `None`. It models the possible absence of a value. There is no null pointer in safe Rust; you use `Option` instead.

**Rule EH-2.** `Result<T, E>` is `Ok(T)` or `Err(E)`. It models a computation that can fail. The error type carries information about what went wrong.

**Rule EH-3.** Both `Option` and `Result` are `#[must_use]`. The compiler warns if you ignore them. If you truly want to discard a result, use `let _ = risky_call();`.

**Rule EH-4.** The `?` operator: applied to a `Result<T, E>` in a function returning `Result<T, F>`, it unwraps `Ok(T)` or returns early with `Err(F)`. The error type is converted via `From::from`. Applied to `Option<T>`, it returns `None` early from a function returning `Option`.

**Rule EH-5.** `unwrap()` panics if the value is `None` or `Err`. Only acceptable in tests, examples, or when you have a guarantee the value is `Some`/`Ok` that the type system cannot express. Not acceptable in library code or production paths.

**Rule EH-6.** `expect("message")` is `unwrap()` with a custom panic message. Preferred over `unwrap()` because the message documents why you believed this was safe.

**Rule EH-7.** `panic!` is for unrecoverable errors — violations of invariants that the program cannot continue from. Not for expected error conditions. Use `Result` for expected failure modes.

**Rule EH-8.** Panics propagate up the call stack via stack unwinding by default. A panic in a thread does not affect other threads (the thread is killed, the handle returns `Err`). Panics across FFI boundaries are undefined behavior — catch them with `std::panic::catch_unwind` before the boundary.

**Rule EH-9.** Error type design: in library code, use `thiserror` to derive `Display` and `Error` for custom error enums. In application code, use `anyhow` for ergonomic error propagation without defining your own error types.

**Rule EH-10.** Implement `From<OtherError> for MyError` so that `?` can automatically convert between error types. This is the mechanism that makes error propagation composable.

**Rule EH-11.** `Option` and `Result` combinator methods: `.map()`, `.and_then()` (flatMap), `.or_else()`, `.unwrap_or()`, `.unwrap_or_else()`, `.ok()` (convert `Option` to `Result`), `.ok_or()` (convert `Option` to `Result` with an error), `.transpose()` (swap `Option<Result<T,E>>` and `Result<Option<T>,E>`).

---

## 13. Pattern Matching Rules

**Rule PM-1.** `match` is exhaustive. Every possible pattern must be covered. The compiler rejects non-exhaustive matches. Use `_` as a wildcard catch-all.

**Rule PM-2.** Patterns are evaluated top to bottom. The first matching arm wins.

**Rule PM-3.** Guards: `Some(x) if x > 0` adds an extra condition to a pattern. If the guard fails, matching continues to the next arm.

**Rule PM-4.** `@` bindings: `n @ 1..=100` binds the matched value to `n` while also testing it against the range pattern.

**Rule PM-5.** Destructuring in patterns:
- Structs: `Point { x, y }` or `Point { x: px, y: py }`.
- Enums: `Some(val)`, `Ok(v)`, `Err(e)`.
- Tuples: `(a, b, c)`.
- References: `&x` in a pattern dereferences and binds the inner value.

**Rule PM-6.** `..` ignores remaining fields in a struct pattern (`Packet { src, .. }`) or remaining elements in a tuple/slice pattern (`[first, .., last]`).

**Rule PM-7.** `_` ignores a single value without binding. `_x` binds but suppresses unused-variable warnings. The distinction matters: `_` immediately drops the value; `_x` keeps it alive until end of scope.

**Rule PM-8.** Match ergonomics (RFC 2005): when matching a reference `&T`, Rust automatically adjusts bindings to be by-reference. `match &some_val { SomeVariant(x) => ... }` gives `x` as a reference. Understanding this avoids needing explicit `ref` in most cases.

**Rule PM-9.** Irrefutable patterns are patterns that always match (binding `let x = val;`, function parameters). Refutable patterns may fail to match (`Some(x)`, a range). Irrefutable patterns are required in `let`, `for`, and function parameters. `if let` and `while let` accept refutable patterns.

**Rule PM-10.** `if let Some(x) = option_val` is shorthand for a `match` when you only care about one variant. `while let` loops as long as the pattern matches.

**Rule PM-11.** Slice patterns: `match slice { [first, rest @ ..] => ... }` matches slices by structure. Useful for parsers and protocol frame handling.

---

## 14. Closures Rules

**Rule CL-1.** Closures are anonymous functions that can capture variables from their enclosing scope. They are distinct types, each unique to their definition site.

**Rule CL-2.** The compiler determines the capture mode automatically by what the closure does with each captured variable:
- If the closure only reads the variable: captures by shared reference `&T`.
- If the closure mutates the variable: captures by mutable reference `&mut T`.
- If the closure consumes the variable (moves it): captures by value (moves it in).

**Rule CL-3.** The `move` keyword forces capture by value for all variables, regardless of what the closure does with them. Necessary when the closure outlives the scope of the captured variables (e.g., when spawning threads or returning closures).

**Rule CL-4.** The three `Fn` traits, from most to least restrictive:
- `FnOnce`: the closure can be called at most once because it consumes captures. All closures implement `FnOnce`.
- `FnMut`: the closure can be called multiple times; it may mutate captures. All `FnMut` closures implement `FnOnce`.
- `Fn`: the closure can be called multiple times and does not mutate or consume captures. All `Fn` closures implement `FnMut` and `FnOnce`.

**Rule CL-5.** The compiler assigns the most permissive trait that the closure's body allows. If it only reads captures, it implements `Fn`. If it mutates captures, it implements `FnMut` (but not `Fn`). If it consumes captures, it implements only `FnOnce`.

**Rule CL-6.** Function pointers `fn(T) -> U` implement `Fn`, `FnMut`, and `FnOnce`. They are a subtype of all three closure traits. They capture nothing.

**Rule CL-7.** `Box<dyn Fn() -> T>` is a heap-allocated closure with dynamic dispatch. Used when you need to store closures of different types in the same container or return a closure from a function without knowing its concrete type.

**Rule CL-8.** Closures returned from functions usually require `move` and `impl Fn` return type, or `Box<dyn Fn>`. The lifetime of captured references must outlive the closure.

---

## 15. Iterator Rules

**Rule IT-1.** The `Iterator` trait requires one method: `fn next(&mut self) -> Option<Self::Item>`. Returns `Some(item)` while items remain, `None` when exhausted.

**Rule IT-2.** Iterators are lazy. Adapter methods (`map`, `filter`, `flat_map`, etc.) do not execute immediately. They build a chain of adapters that executes only when a consuming method is called.

**Rule IT-3.** Consuming methods drive execution: `collect()`, `sum()`, `product()`, `count()`, `fold()`, `for_each()`, `any()`, `all()`, `find()`, `position()`, `max()`, `min()`.

**Rule IT-4.** The three iterator constructors on collections:
- `.iter()`: iterates over `&T` (shared references). The collection is not consumed.
- `.iter_mut()`: iterates over `&mut T` (mutable references). The collection is not consumed.
- `.into_iter()`: consumes the collection and iterates over `T` (owned values).

**Rule IT-5.** `for x in collection` is syntactic sugar for calling `.into_iter()` and then `.next()` in a loop. To avoid consuming the collection, write `for x in &collection` (calls `.iter()`) or `for x in &mut collection` (calls `.iter_mut()`).

**Rule IT-6.** `IntoIterator` is the trait for types that can be converted to an iterator. Implementing it enables the `for` loop.

**Rule IT-7.** `FromIterator<T>` enables `.collect()`. `Vec<T>`, `String`, `HashMap<K,V>`, `HashSet<T>`, `BTreeMap<K,V>` all implement it. You often need a type annotation to tell the compiler which collection to produce.

**Rule IT-8.** `DoubleEndedIterator` adds `.next_back()`. Enables `.rev()` to reverse the iteration direction. Slices, `Vec`, `BTreeMap` iterators implement this.

**Rule IT-9.** `ExactSizeIterator` adds `.len()`. Enables efficient allocation in `collect()` because the capacity is known upfront.

**Rule IT-10.** Iterator fusion: the compiler often inlines and collapses chains of adapters into a single loop with zero intermediate allocations. A chain like `.map().filter().map()` typically compiles to a single loop over the source data.

**Rule IT-11.** `Peekable` wraps an iterator and adds `.peek()` which looks at the next item without consuming it. Essential for recursive-descent parsers and protocol frame parsers.

**Rule IT-12.** `chain(a, b)` or `a.chain(b)`: iterates `a` to completion, then `b`. Both must yield the same `Item` type.

**Rule IT-13.** `zip(a, b)` or `a.zip(b)`: yields `(a_item, b_item)` pairs. Stops when either is exhausted. For kernel-adjacent code: useful for pairing a buffer of keys with a buffer of values.

---

## 16. Unsafe Rules

**Rule US-1.** `unsafe` is not a way to disable the borrow checker. The borrow checker, lifetime checker, and type checker all operate normally inside `unsafe` blocks. `unsafe` only unlocks five specific additional capabilities.

**Rule US-2.** The five things you can do only inside `unsafe { }` or `unsafe fn`:
1. Dereference raw pointers (`*const T`, `*mut T`).
2. Call `unsafe` functions and methods (including all FFI functions).
3. Read or write to `static mut` variables.
4. Implement `unsafe` traits (e.g., `Send`, `Sync`).
5. Access fields of a `union`.

**Rule US-3.** You are responsible for ensuring no undefined behavior occurs in `unsafe` blocks. The contract: you assert to the compiler that invariants hold which the type system cannot verify. If those invariants are wrong, you have undefined behavior.

**Rule US-4.** Undefined behavior (UB) in Rust — the following are always UB, even in `unsafe`:
- Data races: two threads accessing the same memory concurrently, at least one writing, without synchronization.
- Dereferencing a null, dangling, or unaligned pointer.
- Creating a reference that is null, dangling, or unaligned (`&*ptr` where `ptr` is invalid).
- Use-after-free.
- Use of uninitialized memory.
- Invalid values for a type: a `bool` that is not 0 or 1, a `char` with an invalid Unicode codepoint, an enum discriminant that has no corresponding variant.
- Two aliased `&mut T` references to the same memory (even in unsafe, this is UB).
- Violating pointer provenance rules (relevant when converting integers to pointers).
- Unwinding (panic) across an FFI boundary.

**Rule US-5.** Raw pointers (`*const T`, `*mut T`):
- Creating a raw pointer is safe. Dereferencing is unsafe.
- Can be null. Must be checked before dereferencing.
- Must be properly aligned for type `T`.
- Must point to a valid, initialized value of type `T` (or a valid allocation of correct size and alignment) before dereferencing.
- Can alias. Unlike references, there is no compiler-enforced uniqueness for `*mut T`. This is why dereferencing is unsafe.
- Raw pointers do not have lifetimes. The programmer tracks validity manually.

**Rule US-6.** The aliasing model (Stacked Borrows / Tree Borrows): Rust has a formal memory model for unsafe code. An `&mut T` reference grants exclusive access for its lifetime. Creating a raw pointer from an `&mut T` and then using the reference again invalidates the pointer under the model. In practice: once you convert an `&mut T` to a raw pointer, do not use the original reference again until the raw pointer is done.

**Rule US-7.** `unsafe fn` vs `unsafe { }`: marking a function `unsafe fn` signals to callers that they must uphold certain preconditions. The body of an `unsafe fn` does not automatically gain `unsafe` powers — you still need `unsafe { }` blocks inside for the specific unsafe operations.

**Rule US-8.** The guideline for `unsafe` blocks: minimize their scope. Wrap the minimum necessary code. Document in a `// SAFETY:` comment exactly which invariant you are asserting and why it holds.

```rust
// SAFETY: `ptr` was obtained from `Box::into_raw` and has not been freed.
//         We are the sole owner at this point; no other references exist.
let val = unsafe { Box::from_raw(ptr) };
```

**Rule US-9.** Sound unsafe code: an `unsafe` block is "sound" if it is impossible to trigger UB through its public API using only safe code from outside. Unsound code compiles and may run, but it violates Rust's safety guarantee and can produce UB when combined with unrelated safe code.

---

## 17. FFI Rules

**Rule FF-1.** All `extern "C"` functions are implicitly `unsafe`. Every call to a C function must be inside `unsafe { }`.

**Rule FF-2.** Use `#[repr(C)]` on structs that are passed to C code. Without it, the compiler may reorder fields for alignment. With it, the layout matches C's struct layout rules.

**Rule FF-3.** Use `#[repr(C)]` on enums that correspond to C enums. The discriminant type can be specified with `#[repr(C, i32)]` etc.

**Rule FF-4.** Use types from the `libc` crate for C-compatible types: `libc::c_int`, `libc::c_char`, `libc::size_t`, etc. Do not assume `i32 == int` without checking the target platform.

**Rule FF-5.** Null pointers: use `std::ptr::null()` and `std::ptr::null_mut()`. Represent nullable C pointers as `Option<NonNull<T>>` at the Rust boundary — the compiler guarantees `Option<NonNull<T>>` has the same layout as a raw pointer (null optimization).

**Rule FF-6.** Never allow a Rust panic to unwind across an FFI boundary. The behavior is undefined. Catch panics with `std::panic::catch_unwind` before the boundary, or compile the crate with `panic = "abort"`.

**Rule FF-7.** C strings (`char *`) and Rust strings (`&str`, `String`) are different. Rust strings are UTF-8 and not null-terminated. Use `CString` (owned) and `CStr` (borrowed) for FFI with C strings. `CString::new("hello").unwrap()` creates a null-terminated C string. `.as_ptr()` gives the `*const c_char`.

**Rule FF-8.** Lifetime of FFI objects: when a C API returns a pointer and you are responsible for freeing it, encapsulate it in a Rust struct with a `Drop` impl that calls the C free function. This is the RAII pattern, same as C++.

**Rule FF-9.** `bindgen` generates Rust FFI bindings from C headers automatically. Always use it for large APIs to avoid manual transcription errors. Review the output for correctness.

**Rule FF-10.** To export a Rust function to C: annotate with `#[no_mangle]` and `extern "C"`. `#[no_mangle]` prevents name mangling. Rust name-mangles by default, making symbols unresolvable from C.

---

## 18. Module and Visibility Rules

**Rule MD-1.** Everything in Rust is private by default. This includes functions, structs, struct fields, enums, traits, modules, constants, and type aliases.

**Rule MD-2.** `pub` makes an item visible to the parent module and anything that can see the parent.

**Rule MD-3.** `pub(crate)` makes an item visible anywhere within the same crate. Not visible to external crates.

**Rule MD-4.** `pub(super)` makes an item visible to the parent module only.

**Rule MD-5.** `pub(in path)` makes an item visible within a specific ancestor module.

**Rule MD-6.** `pub` on a struct makes the struct visible, but not its fields. Each field must be individually marked `pub` if you want it accessible from outside the module.

**Rule MD-7.** Child modules can access private items of their parent modules. Privacy is not enforced downward, only upward.

**Rule MD-8.** `use` brings items into scope: `use std::collections::HashMap;`. Does not affect visibility — it only creates a local alias.

**Rule MD-9.** `pub use` re-exports an item at the current module's public interface. Used to flatten module hierarchies in library APIs: internal organization is free, but you control what the public API surface looks like.

**Rule MD-10.** Module files: a module `mod foo;` is resolved to either `foo.rs` or `foo/mod.rs`. The 2018 and later edition also supports `foo/some_module.rs` patterns without `mod.rs`.

**Rule MD-11.** `mod foo { ... }` declares an inline module. `mod foo;` (with semicolon) tells the compiler to look for the module's content in a file.

**Rule MD-12.** The crate root is `main.rs` for binaries, `lib.rs` for libraries. These are the roots of the module tree.

---

## 19. Macro Rules

**Rule MC-1.** Declarative macros (`macro_rules!`) perform pattern matching on token trees and expand to code at compile time. They are hygienic: identifiers introduced by the macro do not conflict with identifiers in the call site.

**Rule MC-2.** Macro syntax: `$name:kind` captures a token of a given kind. Common kinds: `expr` (expression), `ty` (type), `ident` (identifier), `literal` (literal value), `pat` (pattern), `stmt` (statement), `block` (block), `tt` (single token tree — the most permissive).

**Rule MC-3.** Repetition in `macro_rules!`: `$( $x:expr ),*` matches zero or more comma-separated expressions. `+` for one or more. `?` for zero or one.

**Rule MC-4.** Declarative macros must be defined before they are used in a file (or the containing module must be declared before the using module). `#[macro_export]` exports a macro at crate root, making it importable.

**Rule MC-5.** Procedural macros run as compiler plugins during compilation. They receive a `TokenStream` and return a `TokenStream`. They must live in a dedicated crate with `proc-macro = true` in `Cargo.toml`.

**Rule MC-6.** Derive macros (`#[derive(MyTrait)]`): can only be applied to structs and enums. They generate code for the annotated type. They do not modify the original type.

**Rule MC-7.** Attribute macros (`#[my_attribute]`): can be applied to any item. They receive the item's tokens and may transform them arbitrarily.

**Rule MC-8.** Function-like proc macros (`my_macro!(...)`): look like macro invocations but are procedural. Can parse arbitrary syntax.

**Rule MC-9.** `stringify!(expr)` converts an expression to a string literal at compile time without evaluating it. `concat!("a", "b")` concatenates string literals at compile time. `include_str!("file.txt")` includes a file as a string literal at compile time. `include_bytes!("file.bin")` as a byte array.

**Rule MC-10.** `dbg!(expr)` prints the file, line, expression, and its value to stderr, then returns the value. Useful for debugging without restructuring code.

---

## 20. Cargo and Crate Rules

**Rule CG-1.** A crate is the smallest compilation unit. A workspace can contain multiple crates. Each crate compiles independently.

**Rule CG-2.** Binary crates have `main.rs` (or multiple under `src/bin/`). Library crates have `lib.rs`. A crate can have both.

**Rule CG-3.** `Cargo.toml` defines the crate: name, version, edition, dependencies, features, build scripts.

**Rule CG-4.** `Cargo.lock` locks exact dependency versions. For applications (binaries), commit it to version control for reproducible builds. For libraries, do not commit it; let downstream consumers resolve versions.

**Rule CG-5.** Semantic versioning: `major.minor.patch`. Breaking changes require a major version bump. The default dependency spec `"1.2"` in Cargo means `>=1.2.0, <2.0.0` (compatible with the specified version per SemVer).

**Rule CG-6.** Features: conditional compilation and optional dependencies. `#[cfg(feature = "my_feature")]` gates code behind a feature. A dependency with `optional = true` is not compiled unless the feature enabling it is active.

**Rule CG-7.** Build scripts (`build.rs`): run on the host machine before compilation. Output directives: `cargo:rustc-link-lib=`, `cargo:rustc-link-search=`, `cargo:rustc-cfg=`, `cargo:rerun-if-changed=`. Used for linking C libraries, generating bindings, and querying the environment.

**Rule CG-8.** Workspaces: multiple crates under one `[workspace]` in a root `Cargo.toml`. They share a single `Cargo.lock` and a single `target/` directory. Dependencies are deduplicated across workspace members.

**Rule CG-9.** `dev-dependencies` are only compiled for tests and benchmarks. They do not appear in the crate's public dependency graph.

**Rule CG-10.** Editions (2015, 2018, 2021): the edition is per crate, not per workspace. Different editions can coexist in a workspace. Editions introduce opt-in syntax and semantic changes without breaking existing code.

---

## 21. no_std Rules

**Rule NS-1.** `#![no_std]` at the crate root removes the `std` library. Only `core` is available by default. `core` contains everything that does not require an operating system or heap allocator.

**Rule NS-2.** To use heap allocation in `no_std`, explicitly import `alloc`: `extern crate alloc;`. You must supply a global allocator via `#[global_allocator]`.

**Rule NS-3.** In `no_std` without `alloc`, you cannot use `String`, `Vec`, `Box`, `Arc`, `Rc`. Use fixed-size arrays, stack-allocated types, and `heapless` crate equivalents.

**Rule NS-4.** A `panic_handler` must be provided in `no_std` crates: `#[panic_handler] fn panic(_info: &PanicInfo) -> !`. Without it, the crate will not compile.

**Rule NS-5.** The `eh_personality` language item may be required depending on the target and panic strategy. When using `panic = "abort"`, this is not needed.

**Rule NS-6.** For kernel work (rust-for-linux), the kernel provides its own allocator and runtime. Crates targeting the kernel must use `no_std` and use the kernel-provided abstractions from `kernel::prelude`.

---

## 22. Type Coercion Rules

**Rule TC-1.** Coercions are implicit type conversions applied by the compiler in coercion sites (function argument positions, let bindings with type annotations, return expressions).

**Rule TC-2.** Deref coercions: if `T: Deref<Target = U>`, then `&T` can be coerced to `&U`.
- `&String` → `&str`
- `&Vec<T>` → `&[T]`
- `&Box<T>` → `&T`
- Multiple deref steps apply transitively.

**Rule TC-3.** Unsized coercions: `[T; N]` can be coerced to `[T]` (array to slice). A concrete type `T` implementing a trait can be coerced to `dyn Trait` (sized to unsized).

**Rule TC-4.** Pointer weakening:
- `&mut T` can be coerced to `&T`.
- `*mut T` can be coerced to `*const T`.
- `&T` can be coerced to `*const T`.
- `&mut T` can be coerced to `*mut T`.

**Rule TC-5.** Coercions do not apply in generic contexts without explicit bounds. `fn f<T>(x: T)` does not coerce the argument. `fn f(x: &str)` does apply deref coercions on the call argument.

---

## 23. Slice Rules

**Rule SL-1.** A slice `&[T]` is a fat pointer: a data pointer plus a length. It is a view into a contiguous sequence of elements. It does not own the data.

**Rule SL-2.** Slices have no fixed size at compile time. They are DSTs (dynamically sized types). They must always be used behind a pointer or reference.

**Rule SL-3.** Array `[T; N]` has a fixed size known at compile time. It coerces to `&[T]` automatically. Operations on slices work on arrays through this coercion.

**Rule SL-4.** Indexing with `slice[i]` panics on out-of-bounds in debug and release. For safe non-panicking access: `slice.get(i)` returns `Option<&T>`.

**Rule SL-5.** `split_at(mid)` splits a slice into two non-overlapping slices at index `mid`. `split_at_mut(mid)` does the same for mutable slices — this is safe because the two halves are disjoint.

**Rule SL-6.** `str` is `[u8]` with the invariant that the bytes are valid UTF-8. It is a DST. Always behind `&str` (borrowed) or `String` (owned). Indexing into a `str` by byte offset may produce an invalid Unicode boundary — use `.chars()`, `.bytes()`, or character boundary methods.

---

## 24. Integer and Arithmetic Rules

**Rule IA-1.** Integer overflow:
- Debug builds: overflow causes a panic.
- Release builds: overflow wraps (two's complement), same as C unsigned overflow.
- Use `checked_add(n)` → `Option<T>`: returns `None` on overflow.
- Use `saturating_add(n)`: clamps at min/max value.
- Use `wrapping_add(n)`: always wraps, explicit about intent.
- Use `overflowing_add(n)` → `(T, bool)`: returns result and overflow flag.

**Rule IA-2.** Integer casting with `as`:
- Casting a larger type to a smaller type truncates: `300u16 as u8 == 44`.
- Casting a signed negative value to unsigned wraps: `-1i32 as u32 == u32::MAX`.
- Casting a float to an integer truncates toward zero. Casting out-of-range float to integer is saturating (as of Rust 1.45).

**Rule IA-3.** Floating-point arithmetic follows IEEE 754. `f32` has ~7 significant digits; `f64` has ~15. NaN != NaN. Use `.is_nan()`, `.is_infinite()`, `.is_finite()` for checks.

**Rule IA-4.** Integer division truncates toward zero, same as C. `-7 / 2 == -3`. Use `div_euclean` for Euclidean division if you need the result to always be positive.

---

## 25. Drop and Destructor Rules

**Rule DR-1.** `Drop::drop` is called automatically when a value goes out of scope. You implement `Drop` to run cleanup logic (closing file descriptors, freeing C memory, etc.).

**Rule DR-2.** You cannot call `value.drop()` directly. Use `std::mem::drop(value)` to force an early drop.

**Rule DR-3.** Fields are dropped in declaration order. The struct body is dropped before the struct's `Drop::drop` impl runs? No — the `Drop::drop` method runs first, then the fields are dropped in order.

**Rule DR-4.** If a struct implements `Drop`, you cannot move out of its fields. The whole value must go through the `drop` path. Work around this with `Option<T>` fields and `.take()`, or `ManuallyDrop<T>`.

**Rule DR-5.** Destructors are guaranteed to run except in: `std::mem::forget`, `ManuallyDrop`, process abort, and some forms of cycle via `Rc`/`Arc`. Rust does not guarantee destructors in the presence of memory leaks — "memory safety" and "resource cleanup" are related but distinct.

---

## Summary: Mental Model of Compiler Enforcement

The Rust compiler enforces these layers in sequence:

```
Layer 1 — Type System          (types, traits, generics, coercions)
Layer 2 — Ownership System     (moves, Copy, Drop)
Layer 3 — Borrow Checker       (borrowing rules, lifetimes, NLL)
Layer 4 — Concurrency System   (Send, Sync, Pin)
Layer 5 — Unsafe Contract      (you assert invariants the above cannot verify)
```

Every compile error is the compiler catching a violation of one of these layers. When you see a borrow checker error, it is Layer 3 telling you that your reference aliasing or lifetime relationship breaks Rule BR-1 through BR-10. When you see a "cannot move out of" error, it is Layer 2 enforcing Rule O-9 or Rule DR-4.

Understanding which layer an error comes from is the first step to fixing it correctly, rather than fighting the compiler with workarounds.

# Rust — Core Topics and Conceptual Map

Since you already work in C and Go, I'll frame these relative to your existing mental models. This is a map of the territory, not a tutorial.

---

## 1. Ownership, Borrowing, and the Borrow Checker

This is Rust's central idea. Everything else flows from it.

**Ownership** — every value has exactly one owner. When the owner goes out of scope, the value is dropped. No GC, no `free()`, deterministic cleanup.

**Move semantics** — assignment moves ownership by default (unlike C where copy is default, unlike Go where the runtime manages this). After a move, the original binding is invalid at compile time.

**Borrowing** — you can lend a value as a shared reference `&T` (many readers) or an exclusive reference `&mut T` (one writer, no readers). The borrow checker enforces these rules statically. This is essentially a compile-time read-write lock — which maps well to your networking mental model.

**Key invariant:** at any point in time, you may have either one `&mut T` *or* any number of `&T`, but never both. This prevents data races at compile time, which is the core of Rust's safety guarantee.

---

## 2. Lifetimes

Lifetimes are annotations that tell the compiler how long references are valid. Most of the time the compiler infers them (lifetime elision rules). You write them explicitly when the compiler cannot figure out the relationship on its own, typically in structs that hold references, or in function signatures with multiple reference parameters.

```rust
fn longest<'a>(x: &'a str, y: &'a str) -> &'a str { ... }
```

Think of a lifetime as a region of code, not a duration in time. The `'a` above says: "the returned reference lives at least as long as both inputs." This matters enormously in systems code where you're working with buffers, packet data, and kernel memory regions.

---

## 3. Type System and Traits

Rust's type system is algebraic.

**Structs** — product types. Named fields, memory layout predictable (with `#[repr(C)]` for FFI).

**Enums** — sum types (tagged unions). This is more powerful than C enums. Rust enums carry data per variant, which is how `Option<T>` and `Result<T, E>` work.

**Traits** — interfaces/typeclasses. Define behavior, not data. Unlike Go interfaces (implicit, structural), Rust traits are explicit (you `impl Trait for Type`). This gives you zero-cost abstractions — the compiler monomorphizes generic code, producing machine code as efficient as hand-written specializations.

Key built-in traits to internalize: `Copy`, `Clone`, `Drop`, `Send`, `Sync`, `Iterator`, `From`/`Into`, `Display`/`Debug`, `Deref`, `AsRef`.

**Generics** — Rust generics are resolved at compile time (monomorphization). No runtime dispatch overhead unless you explicitly opt into it with `dyn Trait` (dynamic dispatch via vtable, like C++ virtual functions).

---

## 4. Memory Model and Smart Pointers

**Stack vs Heap** — same mental model as C. `Box<T>` is a heap allocation. It owns its data, drops when it goes out of scope.

**`Rc<T>` / `Arc<T>`** — reference-counted smart pointers. `Rc` is single-threaded, `Arc` is atomic (thread-safe). Used when multiple owners are needed. `Arc` is your go-to for shared state across threads.

**`Cell<T>` / `RefCell<T>`** — interior mutability. They let you mutate through a shared reference, moving borrow checking from compile time to runtime. Use sparingly. `RefCell` panics at runtime if borrow rules are violated.

**`Mutex<T>` / `RwLock<T>`** — the thread-safe versions of interior mutability for concurrent access. Maps directly to `pthread_mutex_t` in your C mental model.

---

## 5. Error Handling

No exceptions. No `errno`. Two primary mechanisms:

**`Option<T>`** — represents presence or absence. `Some(T)` or `None`. Replaces null pointer patterns.

**`Result<T, E>`** — represents success or failure. `Ok(T)` or `Err(E)`. This is idiomatic Rust error handling.

**The `?` operator** — early return on error, propagating up. It's syntactic sugar for pattern matching and returning `Err` if the value is an error. This composes cleanly in network parsing code.

**Error crates** — in production: `thiserror` for library error types, `anyhow` for application-level error handling.

---

## 6. Closures and Iterators

**Closures** — anonymous functions that capture their environment. Three flavors: `Fn` (borrows), `FnMut` (mutably borrows), `FnOnce` (consumes captured variables). The compiler figures out which one based on what the closure body does.

**Iterators** — lazy, composable, zero-cost. The `Iterator` trait with its adapter methods (`map`, `filter`, `flat_map`, `fold`, `chain`, `take`, `enumerate`…) produce code the compiler fully optimizes. This is how you write expressive data transformation code without runtime overhead.

---

## 7. Concurrency

**`Send` and `Sync`** — marker traits that encode thread-safety in the type system. A type is `Send` if it can be transferred across threads. It is `Sync` if it can be shared across threads via shared references. The compiler enforces these. This is how Rust eliminates data races at compile time.

**Threads** — `std::thread::spawn`. Ownership rules mean data passed to a thread must be `Send`.

**Channels** — `std::sync::mpsc` for message passing (multiple producer, single consumer). Aligns with the networking mental model of passing packets between processing stages.

**Async/Await** — cooperative multitasking without OS thread overhead. Built on `Future` trait. Requires an executor runtime (`tokio`, `async-std`). Essential for high-performance network I/O. Conceptually similar to Go's goroutines but explicit and zero-cost.

---

## 8. Unsafe Rust

This is where you live in systems programming. `unsafe` does not disable the borrow checker — it unlocks five additional capabilities:

- Dereference raw pointers (`*const T`, `*mut T`)
- Call unsafe functions (including FFI)
- Access or mutate `static mut` globals
- Implement unsafe traits
- Access fields of `union` types

**The mental model:** `unsafe` blocks are contracts. You are telling the compiler "I have verified the invariants the type system cannot." Your job is to write safe abstractions over unsafe code and ensure the unsafe block's invariants hold.

This is where kernel-adjacent code, device drivers, and DMA buffer management live.

---

## 9. FFI — Foreign Function Interface

Critical for your work. Rust can call C and be called from C.

`extern "C"` declares or defines functions using the C calling convention. `#[repr(C)]` makes structs layout-compatible with C structs. The `libc` crate provides C type bindings. The `bindgen` tool auto-generates Rust bindings from C headers.

For Linux kernel module work, there is the `rust-for-linux` effort in the kernel tree, which uses Rust's type system to provide safe abstractions over kernel APIs (spinlocks, reference counting, memory allocation).

---

## 10. Modules, Crates, and Cargo

**Crate** — compilation unit. Binary or library.

**Module system** — hierarchical namespace. `pub`/`pub(crate)`/private visibility. Controls API surface. Think of it as a structured way to enforce encapsulation.

**Cargo** — build system and package manager. `Cargo.toml` declares dependencies and build configuration. Features flags allow conditional compilation. Workspaces allow multi-crate repositories. For systems code, you will use build scripts (`build.rs`) to compile C code or generate bindings.

---

## 11. Macros

**Declarative macros** (`macro_rules!`) — pattern-matching on token trees. Used for code generation where functions are insufficient.

**Procedural macros** — compiler plugins that transform syntax trees. Three kinds: derive macros (`#[derive(Debug)]`), attribute macros, function-like macros. `serde` is the canonical example — it generates serialization code at compile time from struct definitions.

---

## 12. `no_std` Environments

For kernel modules, embedded targets, or bootloader-level code, you work without the standard library. `#![no_std]` drops `std`, leaving only `core` (no heap allocation) and optionally `alloc` (heap allocation without OS-specific I/O). This is where Rust meets bare metal.

---

## The Mental Model Hierarchy

```
Ownership & Lifetimes
        ↓
Type System & Traits
        ↓
Memory Model (stack, heap, smart pointers)
        ↓
Concurrency (Send/Sync, threads, async)
        ↓
Unsafe & FFI
        ↓
no_std / Kernel / Embedded
```

Each layer builds on the one above it. You cannot reason about `unsafe` well without a solid model of ownership. You cannot design good async code without understanding the type system.

---

## Questions to guide your next steps

Given your background, the areas that will challenge you most are:

- Do you have a clear mental model of when a type is `Send` vs `Sync` and what that means for your network buffer design?
- How do Rust lifetimes map to the lifetime of kernel-allocated packet buffers in your current work?
- Where in your existing code would `unsafe` FFI boundaries appear, and what invariants would you need to uphold there?

Which of these areas do you want to go deep on first?
