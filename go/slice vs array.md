## Arrays vs Slices in Go, Rust, and C

---

### C

**Array** — fixed-size, stack-allocated, *decays to a pointer* when passed to a function (size info is lost):

```c
int arr[5] = {1, 2, 3, 4, 5};
sizeof(arr); // 20 bytes (5 × 4)

void foo(int *p) { sizeof(p); } // 8 bytes — just a pointer now!
```

**"Slice"** — C has no built-in slice. You pass a pointer + length manually, or wrap them in a struct:

```c
typedef struct { int *ptr; size_t len; } Slice;

Slice s = { .ptr = arr + 1, .len = 3 }; // points into arr[1..3]
```

No bounds checking in either case. You own the pain.

---

### Go

**Array** — fixed-size, **value type** (assignment copies the whole thing). Size is part of the type: `[3]int ≠ [4]int`.

```go
a := [3]int{1, 2, 3}
b := a          // full copy
b[0] = 99       // a[0] is still 1
```

**Slice** — the idiomatic Go container. A lightweight **header**: `(pointer, length, capacity)`. Multiple slices can share the same backing array.

```go
s := []int{1, 2, 3, 4, 5}
t := s[1:4]       // shares memory with s
t[0] = 99         // s[1] is now 99 too!

append(s, 6)      // may allocate a new backing array if cap exceeded
```

| | Array | Slice |
|---|---|---|
| Size | Fixed, part of type | Dynamic |
| Semantics | Value (copied) | Reference (shared pointer) |
| Passing | Copies entire array | Copies 3-word header only |
| Use in practice | Rare | Almost always |

---

### Rust

**Array** — fixed-size, stack-allocated. Size is part of the type: `[i32; 3] ≠ [i32; 4]`.

```rust
let arr: [i32; 5] = [1, 2, 3, 4, 5];
let arr2 = arr;   // copied (if T: Copy), or moved
```

**Slice** — a **fat pointer**: `(pointer, length)`. Written as `[T]` (unsized) but always used behind a reference: `&[T]` or `&mut [T]`. Zero runtime overhead beyond that pointer+len.

```rust
let arr = [1, 2, 3, 4, 5];
let s: &[i32] = &arr[1..4];  // borrows arr[1..3], no copy

// Vec<T> also derefs to &[T]
let v = vec![1, 2, 3];
let s2: &[i32] = &v;
```

Rust's **borrow checker** enforces that a `&mut [T]` slice is the *only* live reference to that data — no silent aliasing like Go.

| | Array | Slice |
|---|---|---|
| Size | Fixed, part of type | Dynamic (runtime len) |
| Ownership | Owned value | Borrowed view |
| Allocation | Stack | Points into existing data |
| Mutable alias | Yes (if `mut`) | Only one `&mut` at a time |

---

### Summary table across all three

| | **C** | **Go** | **Rust** |
|---|---|---|---|
| Array type | `int[N]` | `[N]T` | `[T; N]` |
| Slice type | manual ptr+len | `[]T` (built-in) | `&[T]` (borrowed) |
| Bounds check | ❌ | ✅ (panic) | ✅ (panic) |
| Slice owns data? | depends | no (header only) | no (borrow) |
| Growing | manual realloc | `append()` | `Vec<T>` then deref |
| Safety | ❌ | ✅ | ✅ (+ aliasing rules) |

**Key insight:** in all three languages, a slice is just a *view* into memory — a pointer and a length (Go/Rust add capacity too). The difference is how much the language helps you manage it safely.