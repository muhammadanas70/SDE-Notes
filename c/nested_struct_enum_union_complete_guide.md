# Nested Struct, Enum, and Union: A Complete In-Depth Guide
> Nesting Patterns · Memory Layout · Linux Kernel · C · Rust · Go

---

## Preface: What This Guide Covers

The previous guide established the individual constructs. This guide is entirely about what
happens when you **nest them inside each other** — the most powerful and most misunderstood
dimension of these constructs.

Every pattern in systems code, operating systems, network stacks, compilers, and databases
relies on nested combinations. You will learn:

- Every valid nesting combination (6 combos × named/anonymous variants)
- Exact memory layout computed step-by-step for each combination
- When the compiler adds padding — and why — inside nested types
- How anonymous structs and unions eliminate naming noise
- Linux kernel idioms built from these nested patterns
- Rust's type system advantages when nesting enums and structs
- Go's embedding as a nesting strategy

---

## Table of Contents

1.  [Mental Model: Nesting as Interpretation Layers](#1-mental-model)
2.  [Nesting Taxonomy: All Valid Combinations](#2-nesting-taxonomy)
3.  [Struct inside Struct](#3-struct-inside-struct)
4.  [Union inside Struct — The Tagged Union](#4-union-inside-struct)
5.  [Struct inside Union — Overlapping Interpretations](#5-struct-inside-union)
6.  [Enum inside Struct — Discriminant Pattern (C)](#6-enum-inside-struct)
7.  [Struct inside Enum — Rust ADT Nesting](#7-struct-inside-enum)
8.  [Enum inside Enum — Nested ADTs (Rust)](#8-enum-inside-enum)
9.  [Union inside Union — Multi-layer Overlay](#9-union-inside-union)
10. [Anonymous Structs and Unions](#10-anonymous-structs-and-unions)
11. [The Grand Pattern: struct + enum + union Combined](#11-grand-pattern)
12. [Multi-Level Deep Nesting (3+ levels)](#12-deep-nesting)
13. [Bit Fields inside Nested Structs](#13-bit-fields-in-nested)
14. [Flexible Array Members in Nested Context](#14-flexible-array-members)
15. [Self-Referential Nested Types (Recursive)](#15-self-referential)
16. [Padding Inside Nested Types — The Full Story](#16-padding-in-nested)
17. [Linux Kernel Case Studies: Nested in Practice](#17-linux-kernel-nested)
18. [Network Subsystem: sk_buff Nested Anatomy](#18-skbuff-nested-anatomy)
19. [Performance: Nesting and Cache Behavior](#19-performance)
20. [Common Mistakes and Pitfalls](#20-mistakes)
21. [Quick Reference Decision Matrix](#21-decision-matrix)

---

## 1. Mental Model: Nesting as Interpretation Layers

Before any syntax, build the correct mental image.

**Nesting** means one type is placed physically inside another type's memory footprint.
The outer type's memory block contains the inner type's memory block as a sub-region.

```
OUTER TYPE MEMORY BLOCK
┌──────────────────────────────────────────────────────────────────────┐
│  field_a  │                   INNER TYPE                   │ field_c │
│  (4 bytes)│  ┌─────────────────────────────────────────┐  │ (2 bytes)│
│           │  │  inner.x  │  inner.y  │  inner.z        │  │         │
│           │  │  (4 bytes)│  (4 bytes)│  (8 bytes)      │  │         │
│           │  └─────────────────────────────────────────┘  │         │
└──────────────────────────────────────────────────────────────────────┘
             ▲                                            ▲
             inner starts here                           inner ends here

Physical RAM is flat. Nesting is just how we *name* regions of it.
```

**Three key insights about nesting:**

1. **Nesting is recursive:** Any valid type can appear inside any other type (with some
   language-specific rules). You can nest 10 levels deep if needed.

2. **The outer type's size grows** to accommodate the inner type plus alignment padding.

3. **Each language handles nesting differently:** C requires explicit naming (C11 adds
   anonymous), Rust enforces ownership through nesting, Go promotes embedded fields.

### The Six Nesting Combinations

```
Outer →      struct              union              enum
Inner ↓
struct    struct-in-struct   struct-in-union   struct-in-enum (Rust variant)
union     union-in-struct    union-in-union    union-in-enum  (rare)
enum      enum-in-struct     enum-in-union     enum-in-enum   (Rust nested ADT)
```

---

## 2. Nesting Taxonomy: All Valid Combinations

### Terminology You Must Know Before Proceeding

| Term | Meaning |
|------|---------|
| **Discriminant** | A tag/integer that tells which variant of a union or enum is currently active |
| **Anonymous struct/union** | A nested struct or union with no field name; its members are accessed directly on the outer type |
| **Named nested** | A nested type accessed via a field name: `outer.inner.field` |
| **Promotion** | In anonymous nesting and Go embedding, inner fields are promoted to the outer scope |
| **Tagged union** | A struct containing an enum (tag) plus a union (data); a safe pattern for variant types |
| **Overlay** | When union members share the same physical bytes — reading one after writing another is type punning |
| **Padding** | Invisible bytes the compiler inserts to satisfy alignment requirements |
| **Alignment** | The byte-boundary restriction on where a type may be placed in memory |
| **Tail padding** | Padding at the end of a struct so its size is a multiple of its largest member's alignment |

---

## 3. Struct Inside Struct

This is the most fundamental nesting. The inner struct's fields are laid out sequentially
inside the outer struct's memory, exactly as if they were individual fields — but they are
accessed as a named group.

### 3.1 C — Named Nested Struct

```c
#include <stdio.h>
#include <stddef.h>

/* ── Inner types ─────────────────────────────────────────────── */
struct Point {
    int x;   /* 4 bytes, align 4 */
    int y;   /* 4 bytes, align 4 */
};           /* sizeof = 8, alignof = 4 */

struct Dimensions {
    unsigned int width;   /* 4 bytes, align 4 */
    unsigned int height;  /* 4 bytes, align 4 */
};                        /* sizeof = 8, alignof = 4 */

/* ── Outer type embedding both ───────────────────────────────── */
struct Window {
    char           title[32];  /* 32 bytes @ offset 0  */
    struct Point   position;   /*  8 bytes @ offset 32 */
    struct Dimensions size;    /*  8 bytes @ offset 40 */
    int            z_order;    /*  4 bytes @ offset 48 */
    /* 4 bytes tail padding → total = 56 */
};

int main(void) {
    struct Window w = {
        .title    = "My Window",
        .position = { .x = 100, .y = 200 },
        .size     = { .width = 800, .height = 600 },
        .z_order  = 1,
    };

    /* Access via chained dot notation */
    printf("Position: (%d, %d)\n", w.position.x, w.position.y);
    printf("Size: %u x %u\n",      w.size.width,  w.size.height);

    /* offsetof shows physical position in memory */
    printf("offsetof position   = %zu\n", offsetof(struct Window, position));
    printf("offsetof position.x = %zu\n", offsetof(struct Window, position) +
                                           offsetof(struct Point, x));
    printf("sizeof(Window) = %zu\n", sizeof(struct Window));

    return 0;
}
```

**Memory Layout — Computed Step by Step:**

```
struct Window layout (x86-64, GCC):

Offset  Size  Field
──────  ────  ──────────────────────────────────────────
  0      32   title[32]           (32 chars, align 1)
 32       4   position.x          (int, align 4)
 36       4   position.y          (int, align 4)
 40       4   size.width          (unsigned int, align 4)
 44       4   size.height         (unsigned int, align 4)
 48       4   z_order             (int, align 4)
 52       4   [tail padding]      (to round 52 → 56? No — alignof = 4)
                                   Actually: 52 is divisible by 4 → sizeof = 52

Wait — let's recompute. title[32] has alignof = 1.
Position.x has alignof = 4. Since offset 32 is divisible by 4, no gap needed.
Alignof(Window) = max(1, 4, 4) = 4. sizeof(Window) = 52 rounded to multiple of 4 = 52.

Byte map:
 0                              31 32   35 36   39 40   43 44   47 48   51
 ┌──────────────────────────────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐
 │ title[32]                   │ │pos.x│ │pos.y│ │sz.w │ │sz.h │ │z_ord│
 └──────────────────────────────┘ └─────┘ └─────┘ └─────┘ └─────┘ └─────┘

sizeof(struct Window) = 52

The nested structs (Point, Dimensions) are TRANSPARENT in memory —
there is no extra overhead for the grouping. The bytes are identical
to if you had declared x, y, width, height, z_order as flat fields.
```

### 3.2 C — How Nested Struct Alignment Works

The nested struct's **alignment requirement propagates** to the outer struct:

```c
struct Inner {
    char  a;    /* align = 1 */
    double d;   /* align = 8 ← dominates Inner's alignment */
};
/* alignof(Inner) = 8, sizeof(Inner) = 16 (1 + 7 pad + 8) */

struct Outer {
    char  x;          /* offset 0, align 1  */
    /* 7 bytes padding here — because Inner needs align 8 */
    struct Inner in;  /* offset 8, align 8  */
    int   y;          /* offset 24, align 4 */
    /* 4 bytes tail padding — Outer's align = 8 */
};
/* sizeof(Outer) = 32 */

/*
 Byte map:
  0  1  2  3  4  5  6  7  8  9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28..31
  x  P  P  P  P  P  P  P  a  P  P  P  P  P  P  P [        d         ] [ y  ] P  P  P  P
  ↑                        ↑                       ↑
 Outer.x               Inner.a                  Inner.d

 KEY RULE: the outer struct must be aligned to max(all member alignments),
 including the alignment of ALL nested struct members (recursively).
*/
```

### 3.3 C — Nested Struct Pointer Arithmetic

Because the nested struct is at a fixed offset, you can compute its address:

```c
struct Outer *o = &some_outer;

/* These are all equivalent ways to get &Inner: */
struct Inner *pi1 = &o->in;
struct Inner *pi2 = (struct Inner *)((char *)o + offsetof(struct Outer, in));

/* And equivalently for a field inside Inner: */
double *pd1 = &o->in.d;
double *pd2 = (double *)((char *)o + offsetof(struct Outer, in)
                                   + offsetof(struct Inner, d));

/* This is the foundation of container_of() in the Linux kernel */
```

### 3.4 C — Multi-Level Nesting (Three or More Levels)

```c
struct Vector3 {
    float x, y, z;   /* 12 bytes, align 4 */
};

struct Transform {
    struct Vector3 position;   /* 12 bytes @ offset 0  */
    struct Vector3 rotation;   /* 12 bytes @ offset 12 */
    struct Vector3 scale;      /* 12 bytes @ offset 24 */
};                             /* sizeof = 36, align = 4 */

struct GameObject {
    char             name[64];  /*  64 bytes @ offset 0  */
    struct Transform transform; /*  36 bytes @ offset 64 */
    int              layer;     /*   4 bytes @ offset 100 */
    /* no padding: 104 % 4 == 0 */
};

/*
 Access at three levels:
   obj.transform.position.x

 Physical address of obj.transform.rotation.y:
   (char*)&obj
     + offsetof(GameObject, transform)      = 64
     + offsetof(Transform, rotation)        = 12
     + offsetof(Vector3, y)                 = 4
   = base + 80
*/

struct GameObject obj;
float *ry = &obj.transform.rotation.y;
/* ry == (float*)((char*)&obj + 80) */
```

**Three-level memory diagram:**

```
struct GameObject memory (total 104 bytes):
┌────────────────────────────────────────────────────────────────────────────────────┐
│ name[64]                                                                           │
│  0                                                                              63 │
├──────────────────────────────────────────────────────────────────────────────────┐ │
│ transform (36 bytes, @ offset 64)                                                │ │
│ ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐                  │ │
│ │ position (12 B)  │ │ rotation (12 B)  │ │ scale (12 B)     │                  │ │
│ │ x  │ y  │ z      │ │ x  │ y  │ z      │ │ x  │ y  │ z      │                  │ │
│ │ 64   68   72     │ │ 76   80   84     │ │ 88   92   96     │                  │ │
│ └──────────────────┘ └──────────────────┘ └──────────────────┘                  │ │
└──────────────────────────────────────────────────────────────────────────────────┘ │
│ layer                                                                              │
│ 100                                                                            103 │
└────────────────────────────────────────────────────────────────────────────────────┘
```

### 3.5 Rust — Nested Struct

```rust
// Rust structs nested the same way as C
// By default, Rust MAY reorder fields — use #[repr(C)] for guaranteed layout

#[derive(Debug, Clone, Copy)]
struct Vector3 {
    x: f32,
    y: f32,
    z: f32,
}

#[derive(Debug, Clone, Copy)]
struct Transform {
    position: Vector3,
    rotation: Vector3,
    scale:    Vector3,
}

#[derive(Debug)]
struct GameObject {
    name:      [u8; 64],
    transform: Transform,
    layer:     i32,
}

impl GameObject {
    pub fn new(name: &str) -> Self {
        let mut n = [0u8; 64];
        let bytes = name.as_bytes();
        n[..bytes.len()].copy_from_slice(bytes);
        GameObject {
            name:      n,
            transform: Transform {
                position: Vector3 { x: 0.0, y: 0.0, z: 0.0 },
                rotation: Vector3 { x: 0.0, y: 0.0, z: 0.0 },
                scale:    Vector3 { x: 1.0, y: 1.0, z: 1.0 },
            },
            layer: 0,
        }
    }

    pub fn move_by(&mut self, dx: f32, dy: f32, dz: f32) {
        // Access nested via chained field access
        self.transform.position.x += dx;
        self.transform.position.y += dy;
        self.transform.position.z += dz;
    }
}

// Nested struct update syntax — update only some inner fields
fn scale_half(g: &mut GameObject) {
    g.transform.scale = Vector3 {
        x: g.transform.scale.x * 0.5,
        ..g.transform.scale   // copy remaining fields from current
    };
}

// Destructuring a nested struct
fn print_position(g: &GameObject) {
    let GameObject {
        transform: Transform { position: Vector3 { x, y, z }, .. },
        ..
    } = g;
    println!("Position: ({x}, {y}, {z})");
}

fn main() {
    let mut obj = GameObject::new("Player");
    obj.move_by(1.0, 2.0, 0.5);
    print_position(&obj);

    // Memory sizes
    println!("sizeof Vector3   = {}", std::mem::size_of::<Vector3>());   // 12
    println!("sizeof Transform = {}", std::mem::size_of::<Transform>()); // 36
    println!("sizeof GameObject = {}", std::mem::size_of::<GameObject>()); // Rust may optimize
}
```

**Rust's Field Reordering vs `#[repr(C)]`:**

```rust
// Default Rust — compiler may reorder for optimal packing:
struct A {
    a: u8,    // 1 byte
    b: u64,   // 8 bytes
    c: u16,   // 2 bytes
}
// Rust likely reorders to: b(8), c(2), a(1), pad(5) = 16 bytes

// With nested struct:
struct Outer {
    x: u8,
    inner: A,   // Rust + Rust = both reordered freely
    y: u32,
}
// Rust may rearrange Outer's fields too — no guarantee

// To match C layout exactly — critical for FFI and kernel code:
#[repr(C)]
struct A {
    a: u8,    // offset 0, 1 byte
    // 7 bytes padding
    b: u64,   // offset 8, 8 bytes
    c: u16,   // offset 16, 2 bytes
    // 6 bytes tail padding
}              // sizeof = 24

#[repr(C)]
struct Outer {
    x:     u8,   // offset 0
    // 7 bytes padding (Inner needs align 8)
    inner: A,    // offset 8, 24 bytes
    y:     u32,  // offset 32
    // 4 bytes tail padding
}                // sizeof = 40
```

### 3.6 Go — Nested Struct vs Embedding

Go has two mechanisms: **explicit named field** (nested) and **embedding** (anonymous):

```go
package main

import (
    "fmt"
    "unsafe"
)

// ── Named nested (classic) ─────────────────────────────────────────
type Vector3 struct {
    X, Y, Z float32
}

type TransformNamed struct {
    Position Vector3  // named field — access: t.Position.X
    Rotation Vector3
    Scale    Vector3
}

// ── Embedded (anonymous field) ─────────────────────────────────────
// Go embedding = anonymous field. Fields AND methods promoted.
type WithEmbed struct {
    Vector3          // embedded — access: w.X directly!
    ExtraData int
}

func main() {
    // Named nested
    t := TransformNamed{
        Position: Vector3{1.0, 2.0, 3.0},
        Rotation: Vector3{0, 0, 0},
        Scale:    Vector3{1, 1, 1},
    }
    fmt.Println(t.Position.X)  // explicit chain

    // Embedded — promoted access
    w := WithEmbed{
        Vector3:   Vector3{10.0, 20.0, 30.0},
        ExtraData: 42,
    }
    fmt.Println(w.X)          // promoted! No need for w.Vector3.X
    fmt.Println(w.Vector3.X)  // explicit access also works

    // Memory layout — Go follows C rules (fields in declared order)
    fmt.Printf("sizeof TransformNamed: %d\n", unsafe.Sizeof(TransformNamed{}))
    fmt.Printf("sizeof WithEmbed:      %d\n", unsafe.Sizeof(WithEmbed{}))

    // offsetof equivalent in Go:
    tw := TransformNamed{}
    rotOff := uintptr(unsafe.Pointer(&tw.Rotation)) - uintptr(unsafe.Pointer(&tw))
    fmt.Printf("offset of Rotation: %d\n", rotOff)  // 12
}
```

**Go Embedding Promotion Rules:**

```
struct A { X int }                — A.X
struct B { A; Y int }             — B.X (promoted from A), B.Y
struct C { B; Z int }             — C.X (promoted from A via B), C.Y (promoted from B), C.Z

Promotion only works if the field name is UNIQUE in the outer scope.
If two embedded structs have a field with the same name, neither is promoted
— you must use the explicit path: c.B.A.X
```

---

## 4. Union Inside Struct — The Tagged Union

This is the **single most important nesting pattern** in systems programming.
It solves the problem: "I have a value that is ONE of several types — but which type
varies at runtime."

### What is a "Tag" / "Discriminant"?

Before the pattern: a **discriminant** (also called **tag**) is a field — usually an
integer or an enum — that tells you which union member is currently valid. Without it,
you cannot safely read a union. With it, you have a **tagged union** (also called a
discriminated union or variant record).

```
Without tag (unsafe):            With tag (safe):
┌─────────────┐                  ┌─────────────────────────────────┐
│ union data  │  ← which member? │ tag = INT   │ union data        │
│  as_int     │  ← you don't know│             │  as_int = 42      │
│  as_float   │                  │             │  (as_float invalid)│
└─────────────┘                  └─────────────────────────────────┘
```

### 4.1 C — Named Union Inside Struct

```c
#include <stdio.h>
#include <string.h>
#include <stdint.h>
#include <stddef.h>

/* Step 1: Define the discriminant enum */
typedef enum {
    KIND_NONE   = 0,
    KIND_INT    = 1,
    KIND_FLOAT  = 2,
    KIND_STR    = 3,
    KIND_BOOL   = 4,
} ValueKind;

/* Step 2: Define the union of all possible data types */
union ValueData {
    int64_t  as_int;         /*  8 bytes */
    double   as_float;       /*  8 bytes */
    struct {                 /* ← struct INSIDE union INSIDE struct */
        char  *ptr;          /*  8 bytes pointer */
        size_t len;          /*  8 bytes length  */
    } as_str;                /* 16 bytes total   */
    int      as_bool;        /*  4 bytes         */
};
/* sizeof(union ValueData) = 16 (largest member: as_str = 16 bytes) */

/* Step 3: Combine tag + union into a tagged union struct */
typedef struct {
    ValueKind      kind;   /* 4 bytes @ offset 0 */
    /* 4 bytes padding (union needs align 8) */
    union ValueData data;  /* 16 bytes @ offset 8 */
} Value;
/* sizeof(Value) = 24 */

/* ── Factory functions (construct safely) ──────────────────────── */
Value value_int(int64_t n) {
    Value v;
    v.kind       = KIND_INT;
    v.data.as_int = n;
    return v;
}

Value value_float(double d) {
    Value v;
    v.kind         = KIND_FLOAT;
    v.data.as_float = d;
    return v;
}

Value value_str(const char *s, size_t len) {
    Value v;
    v.kind          = KIND_STR;
    v.data.as_str.ptr = (char *)s;
    v.data.as_str.len = len;
    return v;
}

Value value_bool(int b) {
    Value v;
    v.kind         = KIND_BOOL;
    v.data.as_bool = b ? 1 : 0;
    return v;
}

/* ── Safe reader — checks tag before accessing union ───────────── */
int value_as_int(const Value *v, int64_t *out) {
    if (v->kind != KIND_INT) return -1;
    *out = v->data.as_int;
    return 0;
}

void value_print(const Value *v) {
    switch (v->kind) {
        case KIND_NONE:  printf("null\n");                              break;
        case KIND_INT:   printf("int: %lld\n", (long long)v->data.as_int); break;
        case KIND_FLOAT: printf("float: %g\n", v->data.as_float);      break;
        case KIND_STR:   printf("str: \"%.*s\"\n",
                                (int)v->data.as_str.len,
                                v->data.as_str.ptr);                   break;
        case KIND_BOOL:  printf("bool: %s\n",
                                v->data.as_bool ? "true" : "false");   break;
    }
}

int main(void) {
    Value vals[4] = {
        value_int(42),
        value_float(3.14),
        value_str("hello", 5),
        value_bool(1),
    };

    for (int i = 0; i < 4; i++)
        value_print(&vals[i]);

    printf("\nsizeof(Value) = %zu\n", sizeof(Value));
    printf("offsetof kind = %zu\n", offsetof(Value, kind));
    printf("offsetof data = %zu\n", offsetof(Value, data));

    return 0;
}
```

**Memory Layout:**

```
sizeof(union ValueData) = 16 bytes (as_str is 8+8=16, largest member)
alignof(union ValueData) = 8 bytes (pointer and size_t need align 8)

sizeof(Value) layout:

Byte:   0    1    2    3    4    5    6    7    8    9   10   11   12   13   14   15   16   17   18   19   20   21   22   23
        ┌────────────────┐ ┌────────────────┐ ┌───────────────────────────────────────────────────┐
        │  kind (4 bytes)│ │  PADDING (4 B) │ │  data (union, 16 bytes)                           │
        │  ValueKind int │ │ 00 00 00 00    │ │  as_int(8)  OR  as_float(8)  OR  ptr(8)+len(8)   │
        └────────────────┘ └────────────────┘ └───────────────────────────────────────────────────┘
         ↑ offset 0                ↑ offset 4  ↑ offset 8

Total: 24 bytes

The 4 bytes of padding between kind and data are because the union
has alignment 8 (due to int64_t, double, pointer), and offset 4
is not divisible by 8 — so the compiler pads to offset 8.
```

### 4.2 C — Anonymous Union Inside Struct (C11)

Anonymous means: the union has **no field name**. Its members are accessed **directly**
on the outer struct, as if they were fields of the outer struct.

```c
/* Anonymous union — members accessed WITHOUT a field name */
typedef struct {
    ValueKind kind;
    /* 4 bytes padding */
    union {          /* ← no name here! */
        int64_t  as_int;
        double   as_float;
        struct {
            char  *ptr;
            size_t len;
        } as_str;
        int      as_bool;
    };               /* ← no field name! members promoted to outer struct */
} Value2;

/* Usage — no .data. prefix needed: */
Value2 v;
v.kind   = KIND_INT;
v.as_int = 42;       /* direct access! */

Value2 s;
s.kind        = KIND_STR;
s.as_str.ptr  = "hello";  /* still need to go through named inner struct */
s.as_str.len  = 5;

/* This is IDENTICAL in memory to the named version.
   Anonymous only removes syntactic noise — no memory difference. */
```

**Rules for Anonymous Structs/Unions in C11:**

```
1. The anonymous struct/union must be a direct member of a struct or union (not a field).
2. It cannot have a tag (struct tag { ... } is forbidden for anonymous).
3. Its members must not conflict with other member names in the outer struct.
4. You cannot take the address of an anonymous struct/union itself.
5. Multiple anonymous structs/unions can exist in the same outer struct.
6. They can be nested: anonymous union inside anonymous struct inside struct.
```

### 4.3 Rust — Enum IS the Tagged Union (Compiler-Managed)

Rust's `enum` is literally a tagged union — the compiler manages the discriminant
automatically. You never write the union or tag manually:

```rust
// This Rust enum...
enum Value {
    None,
    Int(i64),
    Float(f64),
    Str(String),
    Bool(bool),
}

// ...is EQUIVALENT to this C struct (conceptually):
//   struct Value {
//       tag: discriminant (u8 or u16 or u32 — compiler chooses)
//       union {
//           /* nothing for None */
//           int64_t  as_int;    /* for Int */
//           double   as_float;  /* for Float */
//           String   as_str;    /* ptr(8)+len(8)+cap(8)=24 for Str */
//           bool     as_bool;   /* for Bool */
//       }
//   }
// sizeof determined by largest variant (String = 24 bytes)

use std::mem;

fn main() {
    println!("sizeof Value: {}", mem::size_of::<Value>());
    // Likely 32 bytes: 8 bytes discriminant region + 24 bytes String data

    // Pattern matching replaces the switch(tag) pattern:
    let vals = vec![
        Value::None,
        Value::Int(42),
        Value::Float(3.14),
        Value::Str(String::from("hello")),
        Value::Bool(true),
    ];

    for v in &vals {
        match v {
            Value::None         => println!("null"),
            Value::Int(n)       => println!("int: {n}"),
            Value::Float(f)     => println!("float: {f}"),
            Value::Str(s)       => println!("str: \"{s}\""),
            Value::Bool(b)      => println!("bool: {b}"),
        }
    }
}
```

**Rust Enum Memory Layout:**

```
enum Value (Rust default layout):

Variant None:
┌────────────────────────────────────────────────────────────────┐
│ discriminant = 0  │  (padding / unused space, 24 bytes)        │
└────────────────────────────────────────────────────────────────┘

Variant Int(i64):
┌────────────────────────────────────────────────────────────────┐
│ discriminant = 1  │  i64 value (8 bytes) │  unused (16 bytes)  │
└────────────────────────────────────────────────────────────────┘

Variant Str(String):
┌────────────────────────────────────────────────────────────────┐
│ discriminant = 3  │  ptr (8) │  len (8) │  cap (8)            │
└────────────────────────────────────────────────────────────────┘

Total size = discriminant_size + max(variant_data_sizes)
           ≈ 8 + 24 = 32 bytes (with alignment padding)

The compiler chooses discriminant width based on number of variants:
  0..=255 variants    → u8  discriminant (but may be padded)
  256..=65535 variants → u16
  etc.
```

### 4.4 Go — Simulated Tagged Union

Go has no union. The idiomatic simulation uses a struct with all possible fields plus a kind tag — this wastes memory but is type-safe:

```go
type ValueKind int

const (
    KindNone ValueKind = iota
    KindInt
    KindFloat
    KindStr
    KindBool
)

// All fields coexist in memory — only one is "active" by convention
type Value struct {
    Kind     ValueKind
    intVal   int64
    floatVal float64
    strVal   string  // string in Go = ptr(8) + len(8) = 16 bytes
    boolVal  bool
}

// sizeof(Value) = 8 (Kind) + 8 (intVal) + 8 (floatVal) + 16 (strVal) + 1 (bool) + 7 (padding)
// = 48 bytes — wasteful compared to C's 24 or Rust's 32

func ValueInt(n int64) Value   { return Value{Kind: KindInt, intVal: n} }
func ValueFloat(d float64) Value { return Value{Kind: KindFloat, floatVal: d} }
func ValueStr(s string) Value  { return Value{Kind: KindStr, strVal: s} }
func ValueBool(b bool) Value   { return Value{Kind: KindBool, boolVal: b} }

func (v Value) Print() {
    switch v.Kind {
    case KindNone:  fmt.Println("null")
    case KindInt:   fmt.Printf("int: %d\n", v.intVal)
    case KindFloat: fmt.Printf("float: %g\n", v.floatVal)
    case KindStr:   fmt.Printf("str: %q\n", v.strVal)
    case KindBool:  fmt.Printf("bool: %v\n", v.boolVal)
    }
}
```

---

## 5. Struct Inside Union — Overlapping Interpretations

When a struct is placed inside a union, multiple structs **share the same bytes**. Reading
struct A's fields after writing struct B's fields is **type punning** — interpreting the
same bytes with a different layout. In C this is defined behavior (C99+). In Rust it requires
`unsafe`. In Go it requires `unsafe.Pointer`.

The use cases:
- Protocol header parsing (TCP/IP headers have multiple views)
- Low-level hardware register access
- CPU instruction encoding (same bits = different fields depending on instruction type)
- Efficient data conversion without memcpy

### 5.1 C — Two Structs Sharing Memory in a Union

```c
#include <stdio.h>
#include <stdint.h>
#include <string.h>

/* Two different views of the same 8 bytes */
union Endpoint {
    struct {
        uint32_t ip;      /* IPv4 address */
        uint16_t port;    /* port number  */
        uint16_t _pad;
    } ipv4;

    struct {
        uint8_t  addr[6]; /* MAC address (6 bytes) */
        uint16_t type;    /* EtherType   (2 bytes) */
    } mac;

    uint8_t  raw[8];      /* raw byte view — all 8 bytes */
    uint64_t as_u64;      /* numeric view — all 8 bytes */
};

int main(void) {
    union Endpoint ep;

    /* Write as IPv4 */
    ep.ipv4.ip   = 0xC0A80101;  /* 192.168.1.1 */
    ep.ipv4.port = 8080;
    ep.ipv4._pad = 0;

    /* Read back as raw bytes */
    printf("Raw bytes: ");
    for (int i = 0; i < 8; i++)
        printf("%02X ", ep.raw[i]);
    printf("\n");

    /* Read back as MAC (type punning — defined in C99) */
    printf("First 3 MAC bytes: %02X:%02X:%02X\n",
           ep.mac.addr[0], ep.mac.addr[1], ep.mac.addr[2]);

    printf("sizeof(union Endpoint) = %zu\n", sizeof(union Endpoint)); /* 8 */

    return 0;
}
```

**Memory Diagram — All Views Overlapping:**

```
Physical bytes (8 bytes at address BASE):
  BASE+0  BASE+1  BASE+2  BASE+3  BASE+4  BASE+5  BASE+6  BASE+7
  ┌───────┬───────┬───────┬───────┬───────┬───────┬───────┬───────┐
  │       │       │       │       │       │       │       │       │
  └───────┴───────┴───────┴───────┴───────┴───────┴───────┴───────┘

View 1 — ipv4:
  ┌───────────────────────┬───────────────┬───────────────┐
  │   ip  (32-bit)        │  port (16-bit)│  _pad (16-bit)│
  └───────────────────────┴───────────────┴───────────────┘

View 2 — mac:
  ┌───────┬───────┬───────┬───────┬───────┬───────┬───────────────┐
  │addr[0]│addr[1]│addr[2]│addr[3]│addr[4]│addr[5]│  type (16-bit)│
  └───────┴───────┴───────┴───────┴───────┴───────┴───────────────┘

View 3 — raw:
  ┌───────┬───────┬───────┬───────┬───────┬───────┬───────┬───────┐
  │  [0]  │  [1]  │  [2]  │  [3]  │  [4]  │  [5]  │  [6]  │  [7]  │
  └───────┴───────┴───────┴───────┴───────┴───────┴───────┴───────┘

View 4 — as_u64:
  ┌───────────────────────────────────────────────────────────────┐
  │                    uint64_t (all 8 bytes)                     │
  └───────────────────────────────────────────────────────────────┘

All four views occupy EXACTLY THE SAME 8 bytes.
Writing through one view and reading through another = type punning.
```

### 5.2 C — Real-World: IP/TCP Header Parsing

```c
/* This pattern is everywhere in network code and device drivers */

/* An Ethernet frame header: 14 bytes
   But we might want to view the EtherType in two different ways */
union EthHeader {
    struct {
        uint8_t  dst_mac[6];
        uint8_t  src_mac[6];
        uint16_t ethertype;  /* big-endian: 0x0800=IPv4, 0x0806=ARP, 0x86DD=IPv6 */
    } eth;

    struct {
        uint8_t  dst_mac[6];
        uint8_t  src_mac[6];
        uint16_t len;        /* 802.3 frame: this field = payload length, not type */
    } ieee8023;

    uint8_t raw[14];
};

/* An IPv4 fragment offset field:
   The same 16-bit field contains both flags and offset */
union FragField {
    uint16_t raw;
    struct {
        /* NOTE: bit field order is implementation-defined — for illustration only */
        uint16_t offset   : 13;  /* fragment offset (in 8-byte units) */
        uint16_t more_frags : 1; /* MF flag */
        uint16_t dont_frag  : 1; /* DF flag */
        uint16_t reserved   : 1; /* reserved, must be 0 */
    } bits;
};
```

### 5.3 C — CPU Instruction Encoding (ARM Thumb Example)

```c
/* ARM Thumb-2 instruction encoding: same 32 bits, different field layout
   depending on instruction type */
union Thumb2Instr {
    uint32_t raw;

    struct {
        uint32_t imm8   : 8;
        uint32_t rd     : 4;   /* destination register */
        uint32_t imm3   : 3;
        uint32_t s      : 1;   /* set flags */
        uint32_t rn     : 4;   /* source register */
        uint32_t imm2   : 2;
        uint32_t type   : 2;
        uint32_t opcode : 5;
        uint32_t prefix : 3;
    } data_processing;

    struct {
        uint32_t imm12  : 12;
        uint32_t rt     : 4;   /* target register */
        uint32_t rn     : 4;   /* base register */
        uint32_t u      : 1;   /* add/subtract offset */
        uint32_t w      : 1;   /* write-back */
        uint32_t l      : 1;   /* load/store */
        uint32_t prefix : 9;
    } load_store;
};
```

### 5.4 Rust — Struct Inside Union (unsafe)

```rust
// Rust union with multiple struct variants
// All members must be Copy, OR use ManuallyDrop

#[repr(C)]
union Endpoint {
    ipv4: EndpointV4,
    mac:  EndpointMac,
    raw:  [u8; 8],
    as_u64: u64,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct EndpointV4 {
    ip:   u32,
    port: u16,
    pad:  u16,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct EndpointMac {
    addr: [u8; 6],
    typ:  u16,
}

fn main() {
    let mut ep = Endpoint { as_u64: 0 };

    // Write as IPv4
    unsafe {
        ep.ipv4 = EndpointV4 { ip: 0xC0A80101, port: 8080, pad: 0 };
    }

    // Read as raw — type punning (safe in terms of memory, unsafe in Rust)
    let bytes: [u8; 8] = unsafe { ep.raw };
    println!("Raw bytes: {:02X?}", bytes);

    // Reading mac fields after writing ipv4 — defined in C99, but in Rust:
    // must use unsafe because compiler cannot verify which variant is active
    let mac_type: u16 = unsafe { ep.mac.typ };
    println!("mac.type = 0x{:04X}", mac_type);

    println!("sizeof Endpoint = {}", std::mem::size_of::<Endpoint>()); // 8
}
```

### 5.5 Go — Struct Inside Union via unsafe

```go
package main

import (
    "fmt"
    "unsafe"
)

type EndpointV4 struct {
    IP   uint32
    Port uint16
    Pad  uint16
}

type EndpointMac struct {
    Addr [6]byte
    Type uint16
}

// No native union in Go — use unsafe to reinterpret
func reinterpretAsV4(mac *EndpointMac) *EndpointV4 {
    // Both structs are 8 bytes — safe to reinterpret IF sizes match
    return (*EndpointV4)(unsafe.Pointer(mac))
}

func reinterpretAsMac(v4 *EndpointV4) *EndpointMac {
    return (*EndpointMac)(unsafe.Pointer(v4))
}

func main() {
    ep := EndpointV4{IP: 0xC0A80101, Port: 8080, Pad: 0}
    mac := reinterpretAsMac(&ep)
    fmt.Printf("MAC view: %02X:%02X:%02X:%02X:%02X:%02X type=0x%04X\n",
        mac.Addr[0], mac.Addr[1], mac.Addr[2],
        mac.Addr[3], mac.Addr[4], mac.Addr[5],
        mac.Type)
    // This is undefined behavior in the Go spec — avoid in production
    // Use encoding/binary for safe cross-type byte interpretation
}
```

---

## 6. Enum Inside Struct — Discriminant Pattern (C)

In C, this is the **standard way to build a tagged union**: put an enum as the first
field of a struct to serve as the discriminant.

### 6.1 Basic Pattern

```c
/* The enum is the "type tag" — tells you what the struct represents */

typedef enum {
    NODE_LITERAL   = 0,
    NODE_IDENT     = 1,
    NODE_BINARY_OP = 2,
    NODE_UNARY_OP  = 3,
    NODE_CALL      = 4,
} NodeType;

/* AST (Abstract Syntax Tree) node — classic enum-in-struct example */
struct AstNode {
    NodeType type;          /* ← enum inside struct (discriminant) */
    /* 4 bytes padding (union needs align 8 on 64-bit) */
    union {
        struct {
            int64_t value;
        } literal;

        struct {
            char *name;
            int   name_len;
        } ident;

        struct {
            char        op;          /* '+', '-', '*', '/' */
            struct AstNode *left;    /* ← pointer to nested AstNode */
            struct AstNode *right;
        } binary;

        struct {
            char        op;
            struct AstNode *operand;
        } unary;

        struct {
            char          *name;
            struct AstNode **args;
            int             n_args;
        } call;
    };
};
```

### 6.2 Enum-in-Struct for State Machines

```c
/* TCP connection state machine: enum inside struct */

typedef enum {
    TCP_CLOSED      = 0,
    TCP_LISTEN      = 1,
    TCP_SYN_SENT    = 2,
    TCP_SYN_RCVD    = 3,
    TCP_ESTABLISHED = 4,
    TCP_FIN_WAIT1   = 5,
    TCP_FIN_WAIT2   = 6,
    TCP_CLOSE_WAIT  = 7,
    TCP_CLOSING     = 8,
    TCP_LAST_ACK    = 9,
    TCP_TIME_WAIT   = 10,
} TcpState;

typedef struct {
    TcpState  state;        /* ← enum inside struct */
    uint32_t  local_ip;
    uint32_t  remote_ip;
    uint16_t  local_port;
    uint16_t  remote_port;
    uint32_t  snd_seq;
    uint32_t  rcv_seq;
    uint32_t  snd_wnd;
    uint32_t  rcv_wnd;
} TcpConnection;

/* State transition function: reads the enum, updates it */
int tcp_transition(TcpConnection *conn, int event) {
    switch (conn->state) {
        case TCP_CLOSED:
            if (event == EVENT_LISTEN)   { conn->state = TCP_LISTEN;   return 0; }
            if (event == EVENT_CONNECT)  { conn->state = TCP_SYN_SENT; return 0; }
            break;
        case TCP_LISTEN:
            if (event == EVENT_SYN_RCVD) { conn->state = TCP_SYN_RCVD; return 0; }
            break;
        case TCP_SYN_RCVD:
            if (event == EVENT_ACK_RCVD) { conn->state = TCP_ESTABLISHED; return 0; }
            break;
        case TCP_ESTABLISHED:
            if (event == EVENT_CLOSE)    { conn->state = TCP_FIN_WAIT1; return 0; }
            if (event == EVENT_FIN_RCVD) { conn->state = TCP_CLOSE_WAIT; return 0; }
            break;
        /* ... more transitions ... */
        default: break;
    }
    return -1; /* invalid transition */
}
```

**Memory Layout with Enum-in-Struct:**

```
TcpConnection layout (x86-64):

Offset  Size  Field
──────  ────  ──────────────────────────────────────────
  0       4   state       (enum = int = 4 bytes)
  4       4   local_ip    (uint32_t)
  8       4   remote_ip   (uint32_t)
 12       2   local_port  (uint16_t)
 14       2   remote_port (uint16_t)
 16       4   snd_seq     (uint32_t)
 20       4   rcv_seq     (uint32_t)
 24       4   snd_wnd     (uint32_t)
 28       4   rcv_wnd     (uint32_t)
──────  ────
 32 bytes total (no padding — all fields fit cleanly)

Note: enum state sits at offset 0 — checking the state is one memory load.
On a 64-byte cache line, this entire struct (32 bytes) fits in HALF a cache line.
```

### 6.3 Rust — Enum Inside Struct

```rust
// Rust: enum is a first-class type, used as a struct field naturally

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum TcpState {
    Closed,
    Listen,
    SynSent,
    SynReceived,
    Established,
    FinWait1,
    FinWait2,
    CloseWait,
    Closing,
    LastAck,
    TimeWait,
}

#[derive(Debug)]
struct TcpConnection {
    state:       TcpState,   // enum inside struct — 1 byte (11 variants < 256)
    local_ip:    u32,
    remote_ip:   u32,
    local_port:  u16,
    remote_port: u16,
    snd_seq:     u32,
    rcv_seq:     u32,
    snd_wnd:     u32,
    rcv_wnd:     u32,
}

impl TcpConnection {
    pub fn new(local_ip: u32, local_port: u16) -> Self {
        TcpConnection {
            state:       TcpState::Closed,
            local_ip,
            remote_ip:   0,
            local_port,
            remote_port: 0,
            snd_seq:     0,
            rcv_seq:     0,
            snd_wnd:     65535,
            rcv_wnd:     65535,
        }
    }

    pub fn listen(&mut self) -> Result<(), &'static str> {
        match self.state {
            TcpState::Closed => { self.state = TcpState::Listen; Ok(()) }
            _ => Err("Can only listen from Closed state"),
        }
    }

    pub fn is_connected(&self) -> bool {
        self.state == TcpState::Established
    }
}

fn main() {
    let mut conn = TcpConnection::new(0xC0A80101, 443);
    println!("State: {:?}", conn.state);  // Closed
    conn.listen().unwrap();
    println!("State: {:?}", conn.state);  // Listen
    println!("sizeof TcpConnection = {}", std::mem::size_of::<TcpConnection>());
}
```

### 6.4 Go — iota Enum Inside Struct

```go
type TcpState int

const (
    TCPClosed TcpState = iota
    TCPListen
    TCPSynSent
    TCPSynReceived
    TCPEstablished
    TCPFinWait1
    TCPFinWait2
    TCPCloseWait
    TCPClosing
    TCPLastAck
    TCPTimeWait
)

func (s TcpState) String() string {
    names := []string{"Closed","Listen","SynSent","SynReceived",
                      "Established","FinWait1","FinWait2","CloseWait",
                      "Closing","LastAck","TimeWait"}
    if int(s) < len(names) { return names[s] }
    return fmt.Sprintf("TcpState(%d)", int(s))
}

type TcpConnection struct {
    State       TcpState  // enum inside struct (Go int)
    LocalIP     uint32
    RemoteIP    uint32
    LocalPort   uint16
    RemotePort  uint16
    SndSeq      uint32
    RcvSeq      uint32
    SndWnd      uint32
    RcvWnd      uint32
}

func (c *TcpConnection) Listen() error {
    if c.State != TCPClosed {
        return fmt.Errorf("can only listen from Closed, currently %s", c.State)
    }
    c.State = TCPListen
    return nil
}
```

---

## 7. Struct Inside Enum — Rust ADT Nesting

This is where Rust separates itself from C. In Rust, an `enum` variant **can contain a full
struct** (or any type). Each variant has its own data, privately typed.

### 7.1 Struct Variants in Rust Enum

```rust
// Rust enum with struct variants — each variant has named fields
#[derive(Debug)]
enum NetworkEvent {
    // Unit variant — no data
    Connected,

    // Tuple variant — positional fields
    DataReceived(u64, Vec<u8>),     // (socket_id, payload)

    // Struct variant — named fields (a mini-struct per variant)
    PacketLost {
        sequence_num: u32,
        retransmit:   bool,
    },

    // Another struct variant
    ConnectionClosed {
        socket_id:    u64,
        reason:       String,
        bytes_sent:   u64,
        bytes_recv:   u64,
    },

    // A variant holding a named struct type
    Error(std::io::Error),
}

fn handle_event(event: NetworkEvent) {
    match event {
        NetworkEvent::Connected => {
            println!("Connection established");
        }

        NetworkEvent::DataReceived(id, payload) => {
            println!("Socket {} received {} bytes", id, payload.len());
        }

        // Destructure the struct variant
        NetworkEvent::PacketLost { sequence_num, retransmit } => {
            println!("Lost packet #{}, retransmit={}", sequence_num, retransmit);
        }

        // Partial destructure with ..
        NetworkEvent::ConnectionClosed { socket_id, reason, .. } => {
            println!("Socket {} closed: {}", socket_id, reason);
        }

        NetworkEvent::Error(e) => {
            eprintln!("Network error: {}", e);
        }
    }
}
```

**Memory Layout of Rust Enum with Struct Variants:**

```
enum NetworkEvent:

All variants share the same memory block. The size = max(all variant sizes).

Variant Connected:
  ┌─────┐
  │ tag │  (no data)
  └─────┘

Variant DataReceived(u64, Vec<u8>):
  ┌─────┬─────────┬─────────────────────────────┐
  │ tag │ u64 (8) │ Vec<u8>: ptr(8)+len(8)+cap(8)│
  │     │         │ = 24 bytes                   │
  └─────┴─────────┴─────────────────────────────┘
  data size = 8 + 24 = 32 bytes

Variant PacketLost { sequence_num: u32, retransmit: bool }:
  ┌─────┬────────┬─────┐
  │ tag │  u32   │ bool│
  └─────┴────────┴─────┘
  data size = 4 + 1 = 5 bytes (+ padding)

Variant ConnectionClosed { socket_id: u64, reason: String, ... }:
  ┌─────┬─────────┬───────────────────────┬────────┬────────┐
  │ tag │ u64 (8) │ String: ptr+len+cap   │ u64 (8)│ u64 (8)│
  │     │         │ = 24 bytes            │        │        │
  └─────┴─────────┴───────────────────────┴────────┴────────┘
  data size = 8 + 24 + 8 + 8 = 48 bytes ← this dominates

sizeof(NetworkEvent) = discriminant + 48 bytes ≈ 56 bytes

PERFORMANCE TIP: The ConnectionClosed variant inflates the whole enum to 56 bytes.
If ConnectionClosed is rare, consider boxing it:
    ConnectionClosed(Box<ClosedInfo>),  // now variant data = 8 bytes (pointer)
This reduces the enum to ~16 bytes at the cost of a heap allocation.
```

### 7.2 Nested Named Struct Inside Enum Variant

```rust
// Define a standalone struct
#[derive(Debug, Clone)]
struct TcpHeader {
    src_port: u16,
    dst_port: u16,
    seq_num:  u32,
    ack_num:  u32,
    flags:    u8,
    window:   u16,
}

#[derive(Debug, Clone)]
struct UdpHeader {
    src_port: u16,
    dst_port: u16,
    length:   u16,
    checksum: u16,
}

#[derive(Debug, Clone)]
struct IcmpHeader {
    icmp_type: u8,
    code:      u8,
    checksum:  u16,
    rest:      u32,
}

// Enum variant holds the entire struct
#[derive(Debug)]
enum TransportPacket {
    Tcp { header: TcpHeader, payload: Vec<u8> },   // struct inside enum variant
    Udp { header: UdpHeader, payload: Vec<u8> },   // struct inside enum variant
    Icmp(IcmpHeader),                               // struct in tuple variant
    Unknown { proto: u8 },
}

impl TransportPacket {
    fn src_port(&self) -> Option<u16> {
        match self {
            TransportPacket::Tcp { header, .. } => Some(header.src_port),
            TransportPacket::Udp { header, .. } => Some(header.src_port),
            _                                   => None,
        }
    }

    fn payload_len(&self) -> usize {
        match self {
            TransportPacket::Tcp { payload, .. } => payload.len(),
            TransportPacket::Udp { payload, .. } => payload.len(),
            _                                    => 0,
        }
    }
}
```

### 7.3 Enum Variant Inside Another Enum Variant

```rust
// A JSON value type — classic recursive ADT
#[derive(Debug, Clone)]
enum JsonValue {
    Null,
    Bool(bool),
    Number(f64),
    Str(String),
    Array(Vec<JsonValue>),          // Vec of the SAME enum — recursive!
    Object(Vec<(String, JsonValue)>), // Vec of key-value pairs
}

// The recursion works because Vec<JsonValue> has a fixed size (pointer + len + cap = 24 bytes)
// The actual JsonValue data is on the heap inside the Vec.

impl JsonValue {
    fn type_name(&self) -> &'static str {
        match self {
            JsonValue::Null     => "null",
            JsonValue::Bool(_)  => "boolean",
            JsonValue::Number(_)=> "number",
            JsonValue::Str(_)   => "string",
            JsonValue::Array(_) => "array",
            JsonValue::Object(_)=> "object",
        }
    }

    fn get(&self, key: &str) -> Option<&JsonValue> {
        if let JsonValue::Object(pairs) = self {
            pairs.iter().find(|(k, _)| k == key).map(|(_, v)| v)
        } else {
            None
        }
    }
}

fn main() {
    let doc = JsonValue::Object(vec![
        ("name".into(), JsonValue::Str("Alice".into())),
        ("age".into(),  JsonValue::Number(30.0)),
        ("scores".into(), JsonValue::Array(vec![
            JsonValue::Number(95.0),
            JsonValue::Number(87.0),
            // Nested array inside array inside object!
        ])),
        ("active".into(), JsonValue::Bool(true)),
    ]);

    if let Some(name) = doc.get("name") {
        println!("Name type: {}", name.type_name());
    }
}
```

---

## 8. Enum Inside Enum — Nested ADTs (Rust)

A Rust enum variant can contain another enum. This creates **hierarchical sum types** —
a type that is "one of category A or category B, and if A then one of A1, A2, A3".

### 8.1 Hierarchical Error Types

```rust
// Inner enums — specific error categories
#[derive(Debug)]
enum NetworkError {
    ConnectionRefused,
    Timeout { after_ms: u64 },
    DnsResolutionFailed(String),
}

#[derive(Debug)]
enum ParseError {
    InvalidUtf8,
    UnexpectedEof,
    InvalidField { field: String, got: String },
}

#[derive(Debug)]
enum AuthError {
    InvalidToken,
    TokenExpired { expired_at: u64 },
    InsufficientPermissions { required: String },
}

// Outer enum — holds inner enums as variants
#[derive(Debug)]
enum AppError {
    Network(NetworkError),    // enum inside enum
    Parse(ParseError),        // enum inside enum
    Auth(AuthError),          // enum inside enum
    IoError(std::io::Error),
    Other(String),
}

// Implement From for ergonomic ? operator usage
impl From<NetworkError> for AppError {
    fn from(e: NetworkError) -> Self { AppError::Network(e) }
}
impl From<ParseError>   for AppError {
    fn from(e: ParseError)   -> Self { AppError::Parse(e) }
}
impl From<AuthError>    for AppError {
    fn from(e: AuthError)    -> Self { AppError::Auth(e) }
}

// Nested pattern matching
fn handle_error(err: &AppError) {
    match err {
        AppError::Network(NetworkError::Timeout { after_ms }) => {
            println!("Network timeout after {}ms — retry", after_ms);
        }
        AppError::Network(NetworkError::ConnectionRefused) => {
            println!("Connection refused — check server");
        }
        AppError::Auth(AuthError::TokenExpired { expired_at }) => {
            println!("Token expired at {} — refresh", expired_at);
        }
        AppError::Auth(AuthError::InsufficientPermissions { required }) => {
            println!("Need permission: {}", required);
        }
        AppError::Parse(ParseError::InvalidField { field, got }) => {
            println!("Bad value '{}' for field '{}'", got, field);
        }
        AppError::IoError(e) => {
            println!("I/O error: {}", e);
        }
        _ => println!("Other error: {:?}", err),
    }
}
```

### 8.2 Nested Enum Pattern — Protocol Command/Response

```rust
// Command pattern: a hierarchical command structure for a database
#[derive(Debug, Clone)]
enum Order {
    Asc,
    Desc,
}

#[derive(Debug, Clone)]
enum Filter {
    Eq(String, JsonValue),
    Ne(String, JsonValue),
    Gt(String, f64),
    Lt(String, f64),
    And(Box<Filter>, Box<Filter>),  // recursive!
    Or(Box<Filter>, Box<Filter>),
    Not(Box<Filter>),
}

#[derive(Debug, Clone)]
enum ReadCommand {
    FindOne { collection: String, filter: Filter },
    FindMany { collection: String, filter: Filter, order: Order, limit: usize },
    Count { collection: String, filter: Filter },
}

#[derive(Debug, Clone)]
enum WriteCommand {
    Insert { collection: String, doc: JsonValue },
    Update { collection: String, filter: Filter, patch: JsonValue },
    Delete { collection: String, filter: Filter },
}

// Top-level command: holds one of the category enums
#[derive(Debug, Clone)]
enum DbCommand {
    Read(ReadCommand),   // enum inside enum
    Write(WriteCommand), // enum inside enum
    Begin,
    Commit,
    Rollback,
}

// Using the nested structure:
fn execute(cmd: DbCommand) -> Result<JsonValue, AppError> {
    match cmd {
        DbCommand::Read(ReadCommand::FindOne { collection, filter }) => {
            println!("FindOne in {}", collection);
            Ok(JsonValue::Null)
        }
        DbCommand::Write(WriteCommand::Insert { collection, doc }) => {
            println!("Insert into {}", collection);
            Ok(doc)
        }
        DbCommand::Begin   => { println!("BEGIN"); Ok(JsonValue::Null) }
        DbCommand::Commit  => { println!("COMMIT"); Ok(JsonValue::Null) }
        DbCommand::Rollback => { println!("ROLLBACK"); Ok(JsonValue::Null) }
        _ => Ok(JsonValue::Null),
    }
}
```

---

## 9. Union Inside Union — Multi-Layer Overlay

A union member can itself be a union. This creates multi-level overlapping interpretations
of the same bytes.

### 9.1 C — Union Inside Union

```c
/* Three-level view of 8 bytes:
   - As a pair of 32-bit integers
   - As a 64-bit integer
   - As bytes
   AND each 32-bit integer can be viewed as two 16-bit integers */

union Pair16 {
    uint32_t as_u32;
    uint16_t words[2];   /* [0]=lower, [1]=upper (little-endian) */
    uint8_t  bytes[4];
};

union MultiView {
    uint64_t   as_u64;     /* view as single 64-bit integer */
    uint8_t    bytes[8];   /* view as 8 bytes */
    uint32_t   dwords[2];  /* view as two 32-bit integers */
    union Pair16 halves[2]; /* view as two sub-unions! */
};
/* sizeof(union MultiView) = 8 */

int main(void) {
    union MultiView mv;
    mv.as_u64 = 0x0102030405060708ULL;

    /* Various views of the same 8 bytes: */
    printf("as_u64:      0x%016llX\n", (unsigned long long)mv.as_u64);
    printf("bytes:       ");
    for (int i = 0; i < 8; i++) printf("%02X ", mv.bytes[i]);
    printf("\n");
    printf("dwords[0]:   0x%08X\n", mv.dwords[0]);
    printf("dwords[1]:   0x%08X\n", mv.dwords[1]);
    printf("halves[0].as_u32:  0x%08X\n", mv.halves[0].as_u32);
    printf("halves[0].words[0]: 0x%04X\n", mv.halves[0].words[0]);

    return 0;
}
```

**Memory Diagram — Multi-level union:**

```
Union MultiView — 8 bytes at address BASE:

Level 1 — as_u64:
  ┌──────────────────────────────────────────────────────────────────┐
  │                        uint64_t (8 bytes)                        │
  └──────────────────────────────────────────────────────────────────┘

Level 1 — bytes[8]:
  ┌────┬────┬────┬────┬────┬────┬────┬────┐
  │[0] │[1] │[2] │[3] │[4] │[5] │[6] │[7] │
  └────┴────┴────┴────┴────┴────┴────┴────┘

Level 1 — dwords[2]:
  ┌──────────────────────┬──────────────────────┐
  │    dwords[0] (u32)   │    dwords[1] (u32)   │
  └──────────────────────┴──────────────────────┘

Level 1 — halves[2] (each is union Pair16):
  ┌──────────────────────┬──────────────────────┐
  │     halves[0]        │     halves[1]         │
  │ ┌────────────────┐   │ ┌────────────────┐    │
  │ │ as_u32 (4B)    │   │ │ as_u32 (4B)    │    │
  │ ├────────┬───────┤   │ ├────────┬───────┤    │
  │ │words[0]│words[1]   │ │words[0]│words[1]    │
  │ │ (u16)  │ (u16) │   │ │ (u16)  │ (u16) │    │
  │ ├──┬──┬──┴──┬──┤ │   │ ├──┬──┬──┴──┬──┤ │    │
  │ │B0│B1│B2  │B3│ │   │ │B4│B5│B6  │B7│ │    │
  │ └──┴──┴────┴──┘ │   │ └──┴──┴────┴──┘ │    │
  └──────────────────────┴──────────────────────┘

All of the above views cover the SAME 8 bytes. Accessing any member
gives you a different interpretation of the same physical memory.
```

### 9.2 Linux Kernel — Union Inside Union (sk_buff)

```c
/* From include/linux/skbuff.h — union inside union in production code */
struct sk_buff {
    /* ... other fields ... */

    union {
        struct {
            unsigned long _skb_refdst;     /* routing destination */
        };
        struct {
            void (*destructor)(struct sk_buff *skb);
        };
    };

    union {
        __be16  inner_protocol;    /* protocol of inner header */
        __u8    inner_ipproto;     /* inner IP protocol */
    };                             /* 2 bytes, both share same memory */

    union {
        __u32  mark;               /* packet mark (iptables) */
        __u32  hash;               /* skb hash */
        __u32  reserved_tailroom;  /* tailroom reservation */
    };                             /* 4 bytes, three interpretations */
};
```

---

## 10. Anonymous Structs and Unions

Anonymous structs and unions are nested types **without a field name**. Their members
are promoted to the outer type's namespace — you access them as if they were direct fields.

### 10.1 C11 Anonymous Union — Complete Example

```c
#include <stdio.h>
#include <stdint.h>

/* Before C11: named inner union — verbose */
typedef struct {
    int type;
    union {
        int    i;
        double d;
        char  *s;
    } data;          /* ← named "data" */
} OldValue;

OldValue old;
old.type   = 1;
old.data.i = 42;    /* must go through .data */

/* C11: anonymous union — cleaner */
typedef struct {
    int type;
    union {
        int    i;    /* accessed as val.i */
        double d;    /* accessed as val.d */
        char  *s;    /* accessed as val.s */
    };               /* ← NO name here */
} NewValue;

NewValue val;
val.type = 1;
val.i    = 42;       /* direct! no .data prefix */
val.d    = 3.14;     /* overwrites val.i — union semantics */

/* Same memory layout as named version — purely syntactic sugar */
```

### 10.2 C11 Anonymous Struct inside Union

```c
/* Anonymous struct inside union: common in protocol headers */
typedef union {
    uint32_t raw;         /* view as raw 32-bit value */

    struct {              /* ← anonymous struct — fields accessed directly on union */
        uint32_t offset : 13;   /* fragment offset */
        uint32_t mf     : 1;    /* more fragments */
        uint32_t df     : 1;    /* don't fragment */
        uint32_t res    : 1;    /* reserved */
        uint32_t __pad  : 16;   /* unused upper 16 bits */
    };

    struct {
        uint16_t flags_and_offset;  /* combined low 16 bits */
        uint16_t upper;
    };
} IpFragField;

IpFragField frag;
frag.raw = 0x00004000;  /* Set DF bit */
printf("DF=%u, offset=%u\n", frag.df, frag.offset);  /* direct access */
```

### 10.3 Multi-Level Anonymous Nesting

```c
/* Anonymous union inside anonymous struct inside named struct */
typedef struct {
    uint8_t version;
    uint8_t type;

    struct {              /* anonymous struct — fields promoted */
        uint16_t total_length;
        uint16_t identifier;
        union {           /* anonymous union inside anonymous struct */
            uint16_t raw_flags;
            struct {
                uint16_t frag_offset : 13;
                uint16_t flags       : 3;
            };            /* anonymous struct inside anonymous union */
        };
        uint8_t  ttl;
        uint8_t  protocol;
    };                    /* no name — total_length, identifier, etc. accessible directly */

    uint32_t src_addr;
    uint32_t dst_addr;
} IpHeader;

IpHeader iph;
iph.version      = 4;         /* direct */
iph.total_length = 60;        /* promoted from anonymous struct */
iph.frag_offset  = 0;         /* promoted from anon struct inside anon union inside anon struct */
iph.flags        = 2;         /* DF bit in flags field */
iph.raw_flags    = 0x4000;    /* same memory as flags+frag_offset, different view */
```

**Nesting Diagram for Anonymous Types:**

```
IpHeader memory layout:

Byte  0: version          ← direct field
Byte  1: type             ← direct field
──────────────────────────────────────────────────
Anonymous struct starts here (fields promoted to IpHeader):
Byte  2-3: total_length   ← promoted
Byte  4-5: identifier     ← promoted
  ──────────────────────────────────────────
  Anonymous union (raw_flags OR bits):
  Byte  6-7: raw_flags    ← promoted (one view)
    ───────────────────────────────────────
    Anonymous struct (inside anon union, inside anon struct):
    Byte  6-7 bits 0-12: frag_offset ← promoted
    Byte  6-7 bits 13-15: flags      ← promoted
    ───────────────────────────────────────
  End anonymous struct (inside union)
  ──────────────────────────────────────────
Byte  8:   ttl            ← promoted
Byte  9:   protocol       ← promoted
──────────────────────────────────────────────────
Byte 10-13: src_addr      ← direct field
Byte 14-17: dst_addr      ← direct field

Total: 18 bytes (without alignment issues)
```

### 10.4 Rust — No Anonymous Structs (Workarounds)

```rust
// Rust has no anonymous structs. The equivalent patterns are:

// 1. Inline tuple struct variant (closest to anonymous struct in enum)
enum Packet {
    Tcp(u16, u16, u32, u32, u8),    // src, dst, seq, ack, flags — positional
    // This is "anonymous" but positional, not named
}

// 2. Use a local struct with a short name (idiomatic)
enum PacketBetter {
    Tcp(TcpFields),
    Udp(UdpFields),
}

// 3. Tuple struct for union-like overlay
#[repr(C)]
union IpFrag {
    raw:  u16,
    // Rust: no anonymous struct inside union — must name them
    bits: IpFragBits,
}

#[repr(C)]
#[derive(Clone, Copy)]
struct IpFragBits {
    // Rust has no native bit fields — use manual bit manipulation
    raw: u16,
}

impl IpFragBits {
    fn frag_offset(&self) -> u16 { self.raw & 0x1FFF }
    fn mf(&self)          -> bool { (self.raw >> 13) & 1 == 1 }
    fn df(&self)          -> bool { (self.raw >> 14) & 1 == 1 }
}
```

### 10.5 Go — Embedding as Anonymous Field Promotion

```go
// Go has anonymous (embedded) fields — semantically similar to anonymous struct

type Address struct {
    Street  string
    City    string
    Country string
    Zip     string
}

type Person struct {
    Name    string
    Age     int
    Address          // embedded — Address fields promoted to Person
    // Accessing: p.Street (not p.Address.Street)
}

type Employee struct {
    Person           // embedded — Person AND Address fields promoted to Employee
    // Accessing: e.Name, e.Street (all promoted!)
    Company string
    Salary  float64
}

func main() {
    e := Employee{
        Person:  Person{
            Name:    "Alice",
            Age:     30,
            Address: Address{Street: "123 Main St", City: "NYC", Country: "US", Zip: "10001"},
        },
        Company: "Acme Corp",
        Salary:  90000,
    }

    // Three levels of promotion — all accessible directly:
    fmt.Println(e.Name)      // from Person
    fmt.Println(e.Street)    // from Address via Person
    fmt.Println(e.Company)   // direct field

    // Explicit path also works:
    fmt.Println(e.Person.Name)
    fmt.Println(e.Person.Address.Street)
}
```

---

## 11. The Grand Pattern: struct + enum + union Combined

This is the **complete tagged union** pattern — combining all three constructs.
It appears in interpreters, virtual machines, compilers, event systems, and the Linux kernel.

### 11.1 C — The Full Pattern

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

/* ══════════════════════════════════════════════════════════════════
   COMPLETE TAGGED UNION: struct { enum tag; union { structs... } }
   ══════════════════════════════════════════════════════════════════ */

/* Step 1: Define all payload structs */
typedef struct {
    int64_t value;
} LiteralInt;

typedef struct {
    double value;
} LiteralFloat;

typedef struct {
    char  *ptr;
    size_t len;
} LiteralStr;

typedef struct AstNode* AstNodePtr;  /* forward declaration for recursion */

typedef struct {
    char       op;          /* '+', '-', '*', '/', '%' */
    AstNodePtr left;
    AstNodePtr right;
} BinaryOp;

typedef struct {
    char       op;          /* '-', '!', '~' */
    AstNodePtr operand;
} UnaryOp;

typedef struct {
    char      *name;
    int        name_len;
    AstNodePtr *args;
    int         n_args;
} CallExpr;

typedef struct {
    char      *name;
    int        name_len;
} Identifier;

/* Step 2: Define the discriminant enum */
typedef enum {
    AST_LITERAL_INT,
    AST_LITERAL_FLOAT,
    AST_LITERAL_STR,
    AST_BINARY_OP,
    AST_UNARY_OP,
    AST_CALL,
    AST_IDENT,
} AstKind;

/* Step 3: Build the tagged union struct */
typedef struct AstNode {
    AstKind kind;      /* ← enum (discriminant) */
    int     line;      /* source line number */
    /* 0 bytes padding if int+int = 8 bytes, union needs align 8 */
    union {            /* ← anonymous union of all payload structs */
        LiteralInt   as_int;
        LiteralFloat as_float;
        LiteralStr   as_str;
        BinaryOp     as_binary;
        UnaryOp      as_unary;
        CallExpr     as_call;
        Identifier   as_ident;
    };
} AstNode;

/* ── Constructors ─────────────────────────────────────────────── */
AstNode *ast_int(int64_t v) {
    AstNode *n = malloc(sizeof *n);
    n->kind       = AST_LITERAL_INT;
    n->line       = 0;
    n->as_int.value = v;
    return n;
}

AstNode *ast_binary(char op, AstNode *l, AstNode *r) {
    AstNode *n = malloc(sizeof *n);
    n->kind           = AST_BINARY_OP;
    n->line           = 0;
    n->as_binary.op   = op;
    n->as_binary.left = l;
    n->as_binary.right= r;
    return n;
}

/* ── Evaluator: checks tag, accesses correct union member ─────── */
int64_t eval(const AstNode *node) {
    switch (node->kind) {
        case AST_LITERAL_INT:
            return node->as_int.value;

        case AST_BINARY_OP:
            switch (node->as_binary.op) {
                case '+': return eval(node->as_binary.left)  + eval(node->as_binary.right);
                case '-': return eval(node->as_binary.left)  - eval(node->as_binary.right);
                case '*': return eval(node->as_binary.left)  * eval(node->as_binary.right);
                case '/': return eval(node->as_binary.left)  / eval(node->as_binary.right);
                default:  return 0;
            }

        case AST_UNARY_OP:
            switch (node->as_unary.op) {
                case '-': return -eval(node->as_unary.operand);
                default:  return 0;
            }

        default:
            return 0;
    }
}

int main(void) {
    /* Build AST for: (3 + 4) * 2 */
    AstNode *three  = ast_int(3);
    AstNode *four   = ast_int(4);
    AstNode *add    = ast_binary('+', three, four);
    AstNode *two    = ast_int(2);
    AstNode *result = ast_binary('*', add, two);

    printf("(3 + 4) * 2 = %lld\n", (long long)eval(result));  /* 14 */
    printf("sizeof(AstNode) = %zu\n", sizeof(AstNode));

    return 0;
}
```

**Complete Memory Layout:**

```
sizeof(AstNode) — Step-by-step calculation:

Fields in AstNode:
  kind:  AstKind (int) = 4 bytes, align 4, @ offset 0
  line:  int           = 4 bytes, align 4, @ offset 4
  union  (anonymous)   — starts at offset 8

Union members:
  LiteralInt  {int64_t}               = 8  bytes, align 8
  LiteralFloat{double}                = 8  bytes, align 8
  LiteralStr  {char*, size_t}         = 16 bytes, align 8
  BinaryOp    {char, ptr, ptr}        = 1+7pad+8+8 = 24 bytes, align 8
  UnaryOp     {char, ptr}             = 1+7pad+8   = 16 bytes, align 8
  CallExpr    {ptr, int, ptr*, int}   = 8+4+8+4    = 24 bytes, align 8
  Identifier  {ptr, int}              = 8+4        = 12 bytes, align 8

Largest union member: BinaryOp or CallExpr = 24 bytes

sizeof(union) = 24 bytes, alignof = 8

Padding between line(offset 4) and union(needs align 8):
  offset after line = 4 + 4 = 8. 8 is divisible by 8. No padding!

sizeof(AstNode) = 8 (kind+line) + 24 (union) = 32 bytes.

Byte map:
  0    3 4    7 8                                31
  ┌────┬─────┬─────────────────────────────────┐
  │kind│line │ union (24 bytes)                 │
  │4B  │4B   │ largest = BinaryOp:              │
  │    │     │ op(1)+pad(7)+left*(8)+right*(8)  │
  └────┴─────┴─────────────────────────────────┘
```

### 11.2 Rust — The Same Pattern, Compiler-Managed

```rust
// Rust enum = tag + union + compiler enforcement
// No separate tag/union needed — the compiler generates all of it

#[derive(Debug)]
enum AstNode {
    LiteralInt(i64),
    LiteralFloat(f64),
    LiteralStr(String),

    BinaryOp {
        op:    char,
        left:  Box<AstNode>,   // Box breaks recursion (heap ptr)
        right: Box<AstNode>,
    },

    UnaryOp {
        op:      char,
        operand: Box<AstNode>,
    },

    Call {
        name: String,
        args: Vec<AstNode>,
    },

    Ident(String),
}

impl AstNode {
    fn eval(&self) -> f64 {
        match self {
            AstNode::LiteralInt(n)   => *n as f64,
            AstNode::LiteralFloat(f) => *f,

            AstNode::BinaryOp { op, left, right } => {
                let l = left.eval();
                let r = right.eval();
                match op {
                    '+' => l + r,
                    '-' => l - r,
                    '*' => l * r,
                    '/' => l / r,
                    _   => panic!("Unknown op"),
                }
            }

            AstNode::UnaryOp { op, operand } => {
                match op {
                    '-' => -operand.eval(),
                    _   => panic!("Unknown unary op"),
                }
            }

            _ => 0.0,
        }
    }
}

fn main() {
    // Build: (3 + 4) * 2
    let expr = AstNode::BinaryOp {
        op:    '*',
        left:  Box::new(AstNode::BinaryOp {
            op:    '+',
            left:  Box::new(AstNode::LiteralInt(3)),
            right: Box::new(AstNode::LiteralInt(4)),
        }),
        right: Box::new(AstNode::LiteralInt(2)),
    };

    println!("Result: {}", expr.eval());  // 14.0
    println!("sizeof AstNode: {}", std::mem::size_of::<AstNode>());
}
```

---

## 12. Multi-Level Deep Nesting (3+ Levels)

Deep nesting is the norm in systems code, not the exception. Linux kernel structs
routinely have 4–6 levels. Understanding the memory layout algorithm for arbitrary depth
is essential.

### 12.1 The General Layout Algorithm (Any Depth)

```
ALGORITHM — compute layout of any nested type:

function layout(type T) → (size, alignment, field_offsets):
    if T is scalar (int, float, pointer):
        return (sizeof T, alignof T, {})

    if T is struct:
        offset = 0
        max_align = 1
        for each field F in T (in declaration order):
            (f_size, f_align, _) = layout(F.type)   ← recursive!
            offset = ceil(offset, f_align)            ← round up to alignment
            record F.offset = offset
            offset += f_size
            max_align = max(max_align, f_align)
        size = ceil(offset, max_align)               ← tail padding
        return (size, max_align, field_offsets)

    if T is union:
        max_size = 0
        max_align = 1
        for each member M in T:
            (m_size, m_align, _) = layout(M.type)
            max_size  = max(max_size, m_size)
            max_align = max(max_align, m_align)
        size = ceil(max_size, max_align)
        return (size, max_align, {all members at offset 0})

    if T is array [N]E:
        (e_size, e_align, _) = layout(E)
        return (N * e_size, e_align, {})

Note: ceil(x, n) = ((x + n - 1) / n) * n  — round x up to multiple of n
```

### 12.2 Full Example — Five-Level Nesting

```c
/* Level 5: smallest unit */
typedef struct {
    uint8_t  major;    /* 1 byte, align 1, @ 0 */
    uint8_t  minor;    /* 1 byte, align 1, @ 1 */
    uint16_t patch;    /* 2 bytes, align 2, @ 2 */
} Version;             /* sizeof = 4, alignof = 2 */

/* Level 4 */
typedef struct {
    char    name[32];  /* 32 bytes, align 1, @ 0 */
    Version version;   /* 4 bytes, align 2, @ 32 */
    /* tail: 36 → round to align 2 → 36. sizeof = 36. */
} Plugin;              /* sizeof = 36, alignof = 2 */

/* Level 3 */
typedef struct {
    uint32_t  count;   /* 4 bytes, align 4, @ 0 */
    Plugin   *plugins; /* 8 bytes, align 8, @ 8 (padded from 4) */
} PluginSet;           /* sizeof = 16, alignof = 8 */

/* Level 2 */
union ConfigData {
    PluginSet  plugins;          /* 16 bytes, align 8 */
    struct {
        uint32_t ip;
        uint16_t port;
        uint16_t _pad;
    } network;                   /*  8 bytes, align 4 */
    char      raw[16];           /* 16 bytes, align 1 */
};                               /* sizeof = 16, alignof = 8 */

/* Level 1 — outermost */
typedef enum {
    CFG_PLUGINS,
    CFG_NETWORK,
    CFG_EMPTY,
} ConfigKind;

typedef struct {
    ConfigKind  kind;       /*  4 bytes, align 4, @ 0 */
    uint32_t    flags;      /*  4 bytes, align 4, @ 4 */
    union ConfigData data;  /* 16 bytes, align 8, @ 8 */
} Config;                   /* sizeof = 24, alignof = 8 */
```

**Five-Level Memory Map:**

```
Config (24 bytes):
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ kind (4B) │ flags (4B) │ data: union ConfigData (16 bytes)                           │
│ @ 0       │ @ 4        │ @ 8                                                         │
│           │            │┌──────────────────────────────────────────────────────────┐ │
│           │            ││ plugins: PluginSet (16 bytes)                            │ │
│           │            ││ ┌─────────────────────┬──────────────────────────────┐  │ │
│           │            ││ │ count (4B) │PAD(4B)  │ *plugins (8B)                │  │ │
│           │            ││ │ @ 8        │ @ 12    │ @ 16 → heap: Plugin[]        │  │ │
│           │            ││ └─────────────────────┴──────────────────────────────┘  │ │
│           │            ││  ──────OR────────────────────────────────────────────    │ │
│           │            ││ network: { ip(4) + port(2) + _pad(2) } = 8 bytes        │ │
│           │            ││  ──────OR────────────────────────────────────────────    │ │
│           │            ││ raw[16]: all 16 bytes as chars                           │ │
│           │            │└──────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────────────┘

Plugin (pointed to by *plugins, on heap):
┌──────────────────────────────────────────────────────────────┐
│ name[32] (32B)   │ version: Version (4B)                     │
│ @ 0              │ ┌──────┬──────┬────────┐                  │
│                  │ │major │minor │ patch  │                  │
│                  │ │(1B)  │(1B)  │(2B)    │                  │
│                  │ └──────┴──────┴────────┘                  │
└──────────────────────────────────────────────────────────────┘
```

### 12.3 Linux Kernel: tcp_sock — Five-Level Inheritance Chain

The Linux kernel uses struct embedding (first-member embedding) to simulate inheritance.
`tcp_sock` has FIVE levels of nesting:

```
tcp_sock (TCP-specific data)
  └── inet_connection_sock (connection-oriented IP data)  [first member]
        └── inet_sock (IP-layer data)                     [first member]
              └── sock (generic socket)                   [first member]
                    └── sock_common (hash table linkage)  [first member]
```

```c
/* ── Level 5 — innermost ─────────────────────────────────────── */
struct sock_common {
    /* Socket lookup keys */
    union {
        __addrpair skc_addrpair;   /* combined src+dst addr for lookup */
        struct {
            __be32 skc_daddr;      /* foreign IPv4 address */
            __be32 skc_rcv_saddr;  /* bound local IPv4 address */
        };
    };
    union {
        unsigned int skc_hash;
        __u16        skc_u16hashes[2];
    };
    union {
        __portpair  skc_portpair;
        struct {
            __be16  skc_dport;    /* foreign port */
            __u16   __skc_dport_and_state; /* used to include state */
        };
    };
    unsigned short skc_family;    /* address family (AF_INET, AF_INET6, ...) */
    volatile unsigned char skc_state;  /* connection state */
    /* ... more fields ... */
};

/* ── Level 4 ──────────────────────────────────────────────────── */
struct sock {
    struct sock_common  __sk_common;  /* MUST be first — for casting */
    #define sk_state    __sk_common.skc_state
    #define sk_family   __sk_common.skc_family
    socket_lock_t        sk_lock;
    int                  sk_rcvbuf;
    int                  sk_sndbuf;
    struct sk_buff_head  sk_receive_queue;
    struct sk_buff_head  sk_write_queue;
    struct proto        *sk_prot;     /* protocol operations vtable */
    /* ... hundreds more fields ... */
};

/* ── Level 3 ──────────────────────────────────────────────────── */
struct inet_sock {
    struct sock         sk;          /* MUST be first */
    __be32              inet_saddr;  /* source IP */
    __be32              inet_daddr;  /* dest IP */
    __be16              inet_sport;  /* source port */
    __be16              inet_dport;  /* dest port */
    __u16               inet_id;     /* IP ID counter */
    __u8                tos;         /* type of service */
    __u8                min_ttl;
    /* ... */
};

/* ── Level 2 ──────────────────────────────────────────────────── */
struct inet_connection_sock {
    struct inet_sock    inet_conn;   /* MUST be first */
    struct request_sock_queue icsk_accept_queue;
    struct inet_bind_bucket  *icsk_bind_hash;
    struct timer_list         icsk_retransmit_timer;
    struct timer_list         icsk_delack_timer;
    const struct inet_connection_sock_af_ops *icsk_af_ops; /* AF vtable */
    const struct tcp_congestion_ops          *icsk_ca_ops; /* CC vtable */
    unsigned int              icsk_retransmit_cnt;
    /* ... */
};

/* ── Level 1 — outermost ─────────────────────────────────────── */
struct tcp_sock {
    struct inet_connection_sock inet_conn;  /* MUST be first */
    /* TCP sender state */
    u32   snd_una;        /* first byte we want ACK for */
    u32   snd_nxt;        /* next sequence number to send */
    u32   snd_up;         /* urgent pointer */
    /* TCP receiver state */
    u32   rcv_nxt;        /* what we want to receive next */
    u32   rcv_wnd;        /* current receiver window */
    /* RTT estimation */
    u32   srtt_us;        /* smoothed RTT in microseconds */
    u32   mdev_us;        /* medium deviation of RTT */
    u32   rttvar_us;
    u32   rto;            /* retransmission timeout */
    /* Congestion control */
    u32   snd_cwnd;       /* sending congestion window */
    u32   snd_ssthresh;   /* slow start threshold */
    /* ... hundreds more TCP-specific fields ... */
};
```

**Casting Between Levels — Why First-Member Rule Works:**

```
Memory layout of tcp_sock (simplified, first 8 bytes only):

Address BASE:
┌────────────────────────────────────────────────────────────────────┐
│  inet_connection_sock.inet_conn.sk.__sk_common.skc_daddr (4 bytes) │
│  inet_connection_sock.inet_conn.sk.__sk_common.skc_rcv_saddr (4B) │
│  ... (continues inside sock_common)                                 │
│  ... (sock fields after sock_common)                               │
│  ... (inet_sock fields after sock)                                 │
│  ... (inet_connection_sock fields after inet_sock)                 │
│  snd_una (first tcp_sock-specific field)                           │
│  ...                                                               │
└────────────────────────────────────────────────────────────────────┘

Because each struct's FIRST MEMBER is the parent struct,
the address of tcp_sock == address of inet_connection_sock
                        == address of inet_sock
                        == address of sock
                        == address of sock_common

So this cast is safe:
  struct tcp_sock   *tp  = ...;
  struct sock       *sk  = (struct sock *)tp;         /* upcast — safe */
  struct inet_sock  *isk = (struct inet_sock *)tp;    /* also safe */

And the kernel provides helpers:
  static inline struct tcp_sock *tcp_sk(const struct sock *sk) {
      return (struct tcp_sock *)sk;  /* downcast — know it's a tcp socket */
  }
```

---

## 13. Bit Fields Inside Nested Structs

Bit fields are integer fields with a specified number of bits. They frequently appear
**inside nested structs** in protocol headers, hardware registers, and OS flags.

### 13.1 What is a Bit Field?

```c
/* A bit field is a struct member with a bit-width specifier after : */
struct Flags {
    unsigned int read    : 1;  /* 1 bit — value 0 or 1 */
    unsigned int write   : 1;  /* 1 bit — value 0 or 1 */
    unsigned int execute : 1;  /* 1 bit — value 0 or 1 */
    unsigned int _unused : 29; /* remaining 29 bits of the 32-bit storage unit */
};
/* sizeof(struct Flags) = 4 (one 32-bit storage unit) */

struct Flags f;
f.read    = 1;
f.write   = 1;
f.execute = 0;
/* Internal bit layout (GCC x86, little-endian):
   bit 0 = read, bit 1 = write, bit 2 = execute, bits 3-31 = unused */
```

**Critical Bit Field Rules:**

```
RULE 1: Bit order within a storage unit is IMPLEMENTATION-DEFINED.
        On GCC/x86: fills from LSB to MSB.
        On some big-endian targets: fills from MSB to LSB.
        → NEVER use bit fields for network protocol fields.

RULE 2: If a bit field doesn't fit in the current storage unit,
        the compiler either places it in the NEXT storage unit
        or lets it straddle (implementation-defined).

RULE 3: You CANNOT take the address of a bit field:
        unsigned int *p = &f.read;  /* ERROR */

RULE 4: Bit fields cannot be members of a union in standard C
        (implementation-defined behavior in practice).

RULE 5: A bit field with width 0 forces alignment to the next
        storage unit boundary:
        struct S { int a : 8; int : 0; int b : 8; };
        /* a and b are in different storage units */
```

### 13.2 IP Header: Bit Fields in Nested Struct

```c
/* Real IP header (simplified — actual kernel uses separate byte access) */
struct iphdr {
#if __BYTE_ORDER == __LITTLE_ENDIAN
    uint8_t  ihl     : 4;   /* header length in 32-bit words (min=5, max=15) */
    uint8_t  version : 4;   /* IP version (4 or 6) */
#elif __BYTE_ORDER == __BIG_ENDIAN
    uint8_t  version : 4;
    uint8_t  ihl     : 4;
#endif
    uint8_t  tos;           /* type of service / DSCP+ECN */
    uint16_t tot_len;       /* total packet length */
    uint16_t id;            /* identification (for fragmentation) */
    uint16_t frag_off;      /* flags (3 bits) + fragment offset (13 bits) */
    uint8_t  ttl;           /* time to live */
    uint8_t  protocol;      /* L4 protocol (6=TCP, 17=UDP, 1=ICMP) */
    uint16_t check;         /* header checksum */
    uint32_t saddr;         /* source address */
    uint32_t daddr;         /* destination address */
} __attribute__((packed));  /* 20 bytes exactly, no padding */
```

**IP Header Memory Layout:**

```
IP Header (20 bytes, packed — no padding):

Byte: 0         1         2-3        4-5   6-7       8    9     10-11  12-15  16-19
      ┌─────────┬─────────┬──────────┬─────┬──────────┬────┬─────┬──────┬──────┬──────┐
      │ver │ihl │   tos   │ tot_len  │ id  │ frag_off │ttl │proto│check │ sadd │ dadd │
      │4b  │4b  │  8 bits │ 16 bits  │16b  │ 16 bits  │8b  │8b   │16b   │32b   │32b   │
      └─────────┴─────────┴──────────┴─────┴──────────┴────┴─────┴──────┴──────┴──────┘

The first byte contains TWO bit fields (ihl:4 and version:4).
On little-endian x86 with GCC:
  byte[0] bit 0-3 = ihl
  byte[0] bit 4-7 = version

frag_off (16 bits):
  bit 15 (MSB): reserved, must be 0
  bit 14:       DF (Don't Fragment)
  bit 13:       MF (More Fragments)
  bits 0-12:    Fragment Offset (in 8-byte units)
```

### 13.3 TCP Header: Bit Fields + Nested Structs

```c
/* TCP Flags — stored as bit fields inside the header struct */
struct tcphdr {
    uint16_t source;     /* source port */
    uint16_t dest;       /* destination port */
    uint32_t seq;        /* sequence number */
    uint32_t ack_seq;    /* acknowledgment number */
#if __BYTE_ORDER == __LITTLE_ENDIAN
    uint16_t res1  : 4;  /* reserved */
    uint16_t doff  : 4;  /* data offset (header length in 32-bit words) */
    uint16_t fin   : 1;  /* finish: no more data from sender */
    uint16_t syn   : 1;  /* synchronize sequence numbers */
    uint16_t rst   : 1;  /* reset the connection */
    uint16_t psh   : 1;  /* push function */
    uint16_t ack   : 1;  /* acknowledgment field significant */
    uint16_t urg   : 1;  /* urgent pointer field significant */
    uint16_t ece   : 1;  /* ECN-Echo */
    uint16_t cwr   : 1;  /* congestion window reduced */
#endif
    uint16_t window;     /* receive window size */
    uint16_t check;      /* checksum */
    uint16_t urg_ptr;    /* urgent pointer */
} __attribute__((packed));

/*
 TCP Header memory layout (20 bytes, no options):

 Bytes 0-1:   source port
 Bytes 2-3:   dest port
 Bytes 4-7:   sequence number
 Bytes 8-11:  acknowledgment number
 Bytes 12-13: [doff:4][res1:4][cwr:1][ece:1][urg:1][ack:1][psh:1][rst:1][syn:1][fin:1]
               ↑ header length       ↑ 8 control flags (1 bit each)
 Bytes 14-15: window size
 Bytes 16-17: checksum
 Bytes 18-19: urgent pointer

 Checking SYN flag in raw packet:
   uint16_t flags_byte = ntohs(*(uint16_t*)&tcp_hdr[12]);
   int syn = (flags_byte >> 1) & 1;  /* portable — no bit field */
*/
```

### 13.4 Bit Fields in sk_buff (Linux Kernel)

```c
/* sk_buff packs many flags into bit fields inside the struct */
struct sk_buff {
    /* ... many fields ... */

    /* These bit fields are packed INSIDE the struct — not a nested type,
       but the mechanism is the same: bit-packed fields within a storage unit */

    __u8 pkt_type:3,        /* PACKET_HOST, PACKET_BROADCAST, etc. */
         pfmemalloc:1,      /* skbuff was allocated from pfmemalloc reserves */
         cloned:1,          /* head may be cloned */
         nohdr:1,           /* this skb was cloned and is a child */
         fclone:2;          /* 00=FCLONE_UNAVAILABLE, 01=ORIGIN, 10=CLONE */

    __u8 peeked:1,          /* this packet has been seen already */
         head_frag:1,       /* skb is a page fragment */
         pfmemalloc:1,
         pp_recycle:1,
         ooo_okay:1,        /* allow the mapping of this skb */
         l4_hash:1,         /* indicate skb->hash is an L4 hash */
         sw_hash:1,         /* software hash has been set */
         no_fcs:1;          /* request NIC to treat last 4 bytes as not FCS */

    /* These are in 1-byte units, 8 flags per byte */
};
```

### 13.5 Rust — Bit Fields via bitflags Crate

```rust
// Rust has no native bit fields — use the bitflags crate
use bitflags::bitflags;

bitflags! {
    #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
    pub struct TcpFlags: u16 {
        const FIN = 0b0000_0001;  // bit 0
        const SYN = 0b0000_0010;  // bit 1
        const RST = 0b0000_0100;  // bit 2
        const PSH = 0b0000_1000;  // bit 3
        const ACK = 0b0001_0000;  // bit 4
        const URG = 0b0010_0000;  // bit 5
        const ECE = 0b0100_0000;  // bit 6
        const CWR = 0b1000_0000;  // bit 7
    }
}

#[repr(C)]
pub struct TcpHeader {
    pub src_port: u16,
    pub dst_port: u16,
    pub seq_num:  u32,
    pub ack_num:  u32,
    pub data_offset_and_flags: u16,  // upper 4 bits = doff, lower 12 = reserved+flags
    pub window:   u16,
    pub checksum: u16,
    pub urg_ptr:  u16,
}

impl TcpHeader {
    pub fn flags(&self) -> TcpFlags {
        // lower 8 bits of data_offset_and_flags are the TCP flags
        TcpFlags::from_bits_truncate(self.data_offset_and_flags & 0xFF)
    }

    pub fn data_offset(&self) -> u8 {
        ((self.data_offset_and_flags >> 12) & 0xF) as u8
    }

    pub fn is_syn(&self)    -> bool { self.flags().contains(TcpFlags::SYN) }
    pub fn is_ack(&self)    -> bool { self.flags().contains(TcpFlags::ACK) }
    pub fn is_syn_ack(&self)-> bool { self.flags().contains(TcpFlags::SYN | TcpFlags::ACK) }
}

fn main() {
    // Simulate a SYN-ACK packet
    let hdr = TcpHeader {
        src_port: 443,
        dst_port: 54321,
        seq_num:  1000,
        ack_num:  501,
        data_offset_and_flags: (5 << 12) | TcpFlags::SYN.bits() | TcpFlags::ACK.bits(),
        window:   65535,
        checksum: 0,
        urg_ptr:  0,
    };

    println!("SYN-ACK: {}", hdr.is_syn_ack());  // true
    println!("flags: {:?}", hdr.flags());
}
```

---

## 14. Flexible Array Members in Nested Context

A **flexible array member** (FAM) is a zero-length array as the **last field** of a struct.
It allows variable-length data to follow the fixed part of the struct in a single allocation.

### 14.1 What is a Flexible Array Member?

```c
/* C99 flexible array member */
struct Packet {
    uint32_t length;    /* fixed part: 4 bytes */
    uint8_t  type;      /* fixed part: 1 byte  */
    uint8_t  _pad[3];   /* manual padding (or compiler adds it) */
    uint8_t  data[];    /* FAM: zero size, MUST be last field */
};
/* sizeof(struct Packet) = 8 (only the fixed part!) */

/* FAM in a struct that is NESTED inside another struct:
   FAM cannot be in a nested struct unless it is the last field
   of the outermost struct. Otherwise the layout is ambiguous. */

/* VALID: FAM in outermost struct */
struct OuterValid {
    int        count;
    struct Packet pkt;    /* ERROR if Packet has FAM and is not the last field */
};

/* Actually, to use FAM correctly, the struct with FAM must be
   the actual last element, and you allocate extra space: */

/* ── Allocation pattern ────────────────────────────────────── */
size_t payload_size = 256;
struct Packet *pkt = malloc(sizeof(struct Packet) + payload_size);
pkt->length = payload_size;
pkt->type   = 0x01;
memset(pkt->data, 0, payload_size);  /* pkt->data[0..255] valid */
```

### 14.2 FAM in Linux Kernel Network Structs

```c
/* Netlink message: fixed header + variable attribute data */
struct nlmsghdr {
    __u32 nlmsg_len;     /* total message length including header */
    __u16 nlmsg_type;    /* message type */
    __u16 nlmsg_flags;   /* additional flags */
    __u32 nlmsg_seq;     /* sequence number */
    __u32 nlmsg_pid;     /* sending process PID */
};

/* After the nlmsghdr, the kernel expects the message body: */
/*
 Memory layout of a complete netlink message:
 ┌──────────────────────────────┬──────────────────────────────────────┐
 │ nlmsghdr (16 bytes)          │ message body (nlmsg_len - 16 bytes)  │
 │ len │ type │ flags│ seq │ pid│ [attribute 1] [attribute 2] ...      │
 └──────────────────────────────┴──────────────────────────────────────┘

 Each attribute is: struct nlattr { __u16 nla_len; __u16 nla_type; }
                    followed by attribute data
*/

/* sk_buff FAM equivalent — skb_shared_info uses this pattern:
   The shared info struct sits at the END of the data buffer, not allocated separately */
struct sk_buff {
    unsigned char *head;  /* points to start of buffer */
    unsigned char *data;  /* points to start of packet */
    unsigned char *tail;  /* points to end of packet data */
    unsigned char *end;   /* points to end of buffer — skb_shared_info lives HERE */
};

/* skb_shared_info is accessed as:
   struct skb_shared_info *shinfo = (struct skb_shared_info *)skb->end;
   This is a manual FAM — not using the C99 [] syntax,
   but achieving the same layout effect */
```

### 14.3 Rust — FAM Equivalent with Vec/Box<[T]>

```rust
// Rust has no FAM. The equivalent is to store a Vec<T> or Box<[T]>
// as the last field — but this adds an extra heap allocation.

// Option 1: Vec<u8> as the payload (heap allocated separately)
struct Packet {
    length:  u32,
    pkt_type: u8,
    data:    Vec<u8>,   // ptr + len + cap = 24 bytes (heap-allocated payload)
}

// Option 2: Box<[u8]> — same heap allocation, no capacity overhead
struct PacketSlice {
    length:  u32,
    pkt_type: u8,
    data:    Box<[u8]>,  // fat pointer: ptr + len = 16 bytes
}

// Option 3: For truly C-compatible FAM, use raw allocation:
use std::alloc::{alloc, Layout};
use std::ptr;

#[repr(C)]
struct PacketHeader {
    length:   u32,
    pkt_type: u8,
    _pad:     [u8; 3],
    // data follows immediately after in memory (like C FAM)
}

fn alloc_packet(payload_size: usize) -> *mut PacketHeader {
    let total = std::mem::size_of::<PacketHeader>() + payload_size;
    let layout = Layout::from_size_align(total, std::mem::align_of::<PacketHeader>()).unwrap();
    let ptr = unsafe { alloc(layout) as *mut PacketHeader };
    unsafe {
        (*ptr).length   = payload_size as u32;
        (*ptr).pkt_type = 0;
        (*ptr)._pad     = [0; 3];
        // Data at: (ptr as *mut u8).add(size_of::<PacketHeader>())
    }
    ptr
}
```

---

## 15. Self-Referential Nested Types (Recursive)

A **self-referential** type contains a pointer or reference to another instance of
itself. This is the foundation of linked lists, trees, graphs, and ASTs.

### 15.1 C — Self-Referential Struct (Linked List Node)

```c
/* A struct can contain a POINTER to itself — but not a direct instance
   (that would require infinite memory). */

struct ListNode {
    int              value;
    struct ListNode *next;   /* pointer to same type — self-referential */
};

/* Memory per node: 4 (value) + 4 (padding) + 8 (pointer) = 16 bytes */

/* Building a list: 1 → 2 → 3 → NULL */
struct ListNode *head = NULL;
for (int i = 3; i >= 1; i--) {
    struct ListNode *node = malloc(sizeof(struct ListNode));
    node->value = i;
    node->next  = head;
    head        = node;
}
```

**Memory Diagram — Linked List:**

```
Stack:
  head ─────────────────────────────────────────────────────┐
                                                             │
Heap:                                                        ▼
  ┌──────────┬──────────┐    ┌──────────┬──────────┐    ┌──────────┬──────────┐
  │ value: 1 │  next ───┼───►│ value: 2 │  next ───┼───►│ value: 3 │  next:   │
  │  (4 B)   │  (8 B)   │    │  (4 B)   │  (8 B)   │    │  (4 B)   │  NULL    │
  └──────────┴──────────┘    └──────────┴──────────┘    └──────────┴──────────┘
  16 bytes each node
```

### 15.2 C — Self-Referential via Intrusive List (Linux Kernel Style)

```c
/* Linux uses intrusive lists: the link is embedded INSIDE the data struct */
struct list_head {
    struct list_head *next;
    struct list_head *prev;
};  /* 16 bytes: two pointers */

struct Task {
    int              pid;
    char             name[16];
    struct list_head list;   /* embed the list_head — intrusive */
};

/* container_of: given a list_head pointer, get back to Task */
#define offsetof(type, member) __builtin_offsetof(type, member)
#define container_of(ptr, type, member) \
    ((type *)((char *)(ptr) - offsetof(type, member)))

/* Usage */
struct Task *t = malloc(sizeof(struct Task));
t->pid = 42;

/* Later, given a struct list_head *lh pointing to t->list: */
struct Task *back = container_of(lh, struct Task, list);
/* back == t */
```

**Memory Layout — Intrusive List:**

```
Task struct:
┌──────────────────┬──────────────────────────────────────┐
│ pid (4B) │name   │ list: list_head                       │
│          │(16B)  │ ┌──────────────────┬────────────────┐ │
│          │       │ │ next (8B)        │ prev (8B)      │ │
│          │       │ └──────────────────┴────────────────┘ │
└──────────────────┴──────────────────────────────────────┘
 offset 0   4       20 = offset of list
                    ↑
                    container_of(lh, Task, list) = lh - 20 = &Task

Multiple Tasks linked:
 Task A                          Task B                          Task C
 ┌─────┬───────────────────┐    ┌─────┬───────────────────┐    ┌─────┬────────┐
 │ ... │list.next──────────┼───►│ ... │list.next──────────┼───►│ ... │list.next─┐
 │     │list.prev◄─────────┼────│     │list.prev◄─────────┼────│     │list.prev │
 └─────┴───────────────────┘    └─────┴───────────────────┘    └─────┴──────────┘
                                                                              ↑
                                                                (circles back to A
                                                                 in a circular list)
```

### 15.3 Rust — Recursive Enum (Tree)

```rust
// Recursive types MUST use Box (or other indirection) to break infinite size

// Binary Search Tree
#[derive(Debug)]
enum BST {
    Empty,
    Node {
        value: i64,
        left:  Box<BST>,   // Box makes this a pointer — fixed size (8 bytes)
        right: Box<BST>,
    },
}

impl BST {
    pub fn new() -> Self { BST::Empty }

    pub fn insert(self, v: i64) -> Self {
        match self {
            BST::Empty => BST::Node {
                value: v,
                left:  Box::new(BST::Empty),
                right: Box::new(BST::Empty),
            },
            BST::Node { value, left, right } => {
                if v < value {
                    BST::Node { value, left: Box::new(left.insert(v)), right }
                } else if v > value {
                    BST::Node { value, left, right: Box::new(right.insert(v)) }
                } else {
                    BST::Node { value, left, right }  // duplicate — no insert
                }
            }
        }
    }

    pub fn contains(&self, v: i64) -> bool {
        match self {
            BST::Empty => false,
            BST::Node { value, left, right } => {
                if v == *value      { true }
                else if v < *value  { left.contains(v) }
                else                { right.contains(v) }
            }
        }
    }
}

fn main() {
    let tree = BST::new()
        .insert(5)
        .insert(3)
        .insert(7)
        .insert(1)
        .insert(4);

    println!("Contains 4: {}", tree.contains(4));  // true
    println!("Contains 6: {}", tree.contains(6));  // false

    // sizeof BST::Node = discriminant + i64(8) + Box(8) + Box(8)
    // = 1 + 7 (pad) + 8 + 8 + 8 = 32 bytes
    println!("sizeof BST: {}", std::mem::size_of::<BST>());
}
```

---

## 16. Padding Inside Nested Types — The Full Story

### 16.1 Three Sources of Padding in Nested Types

```
SOURCE 1 — INTERIOR PADDING (between fields):
  Inserted between fields to satisfy each field's alignment requirement.

SOURCE 2 — BOUNDARY PADDING (between outer fields and nested struct):
  The nested struct has an alignment requirement. The compiler pads the
  current offset to reach that alignment before placing the nested struct.

SOURCE 3 — TAIL PADDING (at end of struct or union):
  After all fields, the struct's size is rounded up to its own alignment
  (= max alignment of all fields). This allows arrays to keep elements aligned.
```

**Full Worked Example — All Three Sources:**

```c
struct Inner {
    char  a;    /* 1 byte, align 1 */
    /* 7 bytes INTERIOR PADDING (source 1) — next field needs align 8 */
    double b;   /* 8 bytes, align 8 */
    char   c;   /* 1 byte, align 1 */
    /* 7 bytes TAIL PADDING (source 3) — Inner.size rounds to 24 */
};
/* sizeof(Inner) = 24, alignof(Inner) = 8 */

struct Outer {
    char  x;    /* 1 byte, align 1, @ offset 0 */
    /* 7 bytes BOUNDARY PADDING (source 2) — Inner needs align 8 */
    Inner in;   /* 24 bytes, align 8, @ offset 8 */
    char  y;    /* 1 byte, align 1, @ offset 32 */
    /* 7 bytes TAIL PADDING (source 3) — Outer.size rounds to 40 */
};
/* sizeof(Outer) = 40, alignof(Outer) = 8 */

/*
 Byte map (40 bytes total):
  0   1   2   3   4   5   6   7   8   9  10  11  12  13  14  15  16..23  24  25..31  32  33..39
  x   P   P   P   P   P   P   P   a   P   P   P   P   P   P   P    b    c    P..P    y   P..P

  P = padding byte
  x = Outer.x               (offset 0)
  [pad 1-7]                  7 bytes boundary padding for Inner
  a = Inner.a               (offset 8)
  [pad 9-15]                 7 bytes interior padding inside Inner
  b = Inner.b               (offsets 16-23)
  c = Inner.c               (offset 24)
  [pad 25-31]                7 bytes tail padding of Inner
  y = Outer.y               (offset 32)
  [pad 33-39]                7 bytes tail padding of Outer
*/
```

### 16.2 Union in Struct — Padding Rules

```c
struct WithUnion {
    char  a;           /* 1 byte, @ 0 */
    /* How much padding before union? Depends on union's alignment */
    union {
        int   as_int;    /* align 4 */
        double as_dbl;   /* align 8 */
    } u;               /* alignof(u) = 8 — padding from offset 1 to offset 8 = 7 bytes */
    /* sizeof(u) = 8 (largest member, rounded to align 8) */
    char  b;           /* 1 byte, @ offset 16 */
    /* tail padding: align 8, size rounds from 17 to 24 → 7 bytes */
};
/* sizeof(WithUnion) = 24 */

/* Compare: if union only had int (align 4): */
struct WithUnion2 {
    char  a;
    /* padding 3 bytes (to align 4) */
    union {
        int as_int;   /* align 4, size 4 */
        char as_chars[4]; /* align 1, size 4 */
    } u;
    /* sizeof(u) = 4 */
    char  b;
    /* tail padding 3 bytes (to align 4) */
};
/* sizeof(WithUnion2) = 12 */
```

### 16.3 How #[repr(packed)] Removes Padding in Nested Structs

```c
/* C: __attribute__((packed)) removes ALL padding, even between nested structs */
struct __attribute__((packed)) PackedOuter {
    char   a;   /* offset 0 */
    int    b;   /* offset 1 — UNALIGNED! Usually access penalty on x86, crash on ARM */
    struct __attribute__((packed)) PackedInner {
        char   x;   /* offset 0 within inner */
        double y;   /* offset 1 — UNALIGNED! */
    } inner;        /* at offset 5 within Outer */
    char   c;   /* offset 5 + sizeof(PackedInner) = 5 + 9 = offset 14 */
};
/* sizeof(PackedOuter) = 15 — no padding anywhere */
/* WARNING: accessing b, inner.y etc. on ARM/MIPS without unaligned support = SIGBUS */
```

```rust
// Rust equivalent:
#[repr(C, packed)]
struct PackedOuter {
    a:     u8,
    b:     u32,      // at offset 1, potentially unaligned
    inner: PackedInner,
    c:     u8,
}

#[repr(C, packed)]
struct PackedInner {
    x: u8,
    y: f64,  // at offset 1 — unaligned
}

// Rust REQUIRES unsafe to read packed fields that may be unaligned:
let po = PackedOuter { a: 1, b: 2, inner: PackedInner { x: 3, y: 4.0 }, c: 5 };
let b_val = unsafe { std::ptr::read_unaligned(&po.b) };
```

### 16.4 Struct Field Reordering to Minimize Padding

```c
/* Rule: Arrange fields from LARGEST alignment to SMALLEST */

/* BAD — wastes 14 bytes of padding: */
struct Bad {
    char   a;    /* 1B, align 1 → offset 0 */
    /* 7B padding */
    double b;    /* 8B, align 8 → offset 8 */
    char   c;    /* 1B, align 1 → offset 16 */
    /* 3B padding */
    int    d;    /* 4B, align 4 → offset 20 */
    char   e;    /* 1B, align 1 → offset 24 */
    /* 7B tail padding */
};  /* sizeof = 32 */

/* GOOD — optimal, zero internal padding: */
struct Good {
    double b;    /* 8B, align 8 → offset 0 */
    int    d;    /* 4B, align 4 → offset 8 */
    char   a;    /* 1B, align 1 → offset 12 */
    char   c;    /* 1B, align 1 → offset 13 */
    char   e;    /* 1B, align 1 → offset 14 */
    /* 1B tail padding */
};  /* sizeof = 16 — 50% smaller! */

/* RULE: In a nested struct context, the nested struct behaves like
   a single field with its own size and alignment for layout purposes.
   You cannot reorder WITHIN a nested struct to affect the outer struct's
   padding — only the outer struct's field order matters for boundary padding. */
```

---

## 17. Linux Kernel Case Studies: Nested in Practice

### 17.1 sk_buff: The Most Complex Nested Structure in Linux

```c
/* sk_buff (Socket Buffer) — simplified but complete nesting diagram */

struct sk_buff {

    /* ── List linkage (nested list_head structs) ──────────────── */
    union {
        struct {
            struct sk_buff  *next;
            struct sk_buff  *prev;
            union {
                struct net_device *dev;
                unsigned long      dev_scratch;
            };
        };
        struct rb_node     rbnode;   /* for RB-tree queues (e.g., OFO queue) */
        struct list_head   list;
    };

    /* ── Socket association ───────────────────────────────────── */
    struct sock *sk;

    /* ── Timestamps ───────────────────────────────────────────── */
    union {
        ktime_t    tstamp;
        u64        skb_mstamp_ns;
    };

    /* ── Data pointers — the CORE of sk_buff ─────────────────── */
    unsigned char  *head;   /* start of buffer allocation */
    unsigned char  *data;   /* start of current packet data */
    unsigned char  *tail;   /* end of current packet data */
    unsigned char  *end;    /* end of buffer allocation */
    /*
      Layout of the buffer pointed to by head:
      head          data        tail      end
       │             │           │         │
       ▼             ▼           ▼         ▼
      ┌─────────────┬───────────┬──────────┐
      │  headroom   │  packet   │ tailroom │
      │(prepend hdr)│  data     │(append)  │
      └─────────────┴───────────┴──────────┘
      ↑ at end: struct skb_shared_info lives here (after the buffer)
    */

    /* ── Length fields ────────────────────────────────────────── */
    unsigned int len;        /* total length of all data */
    unsigned int data_len;   /* length of non-linear area */
    __u16        mac_len;
    __u16        hdr_len;

    /* ── Checksum (union of multiple interpretations) ─────────── */
    union {
        __wsum  csum;
        struct {
            __u16 csum_start;
            __u16 csum_offset;
        };
    };

    /* ── Packet type and protocol ─────────────────────────────── */
    __be16 protocol;     /* ETH_P_IP, ETH_P_IPV6, ETH_P_ARP, ... */
    __u16  transport_header;  /* offset from head to L4 header */
    __u16  network_header;    /* offset from head to L3 header */
    __u16  mac_header;        /* offset from head to L2 header */

    /* ── Marks and hashes ─────────────────────────────────────── */
    union {
        __u32  mark;             /* fwmark for iptables/nftables */
        __u32  reserved_tailroom;
    };

    union {
        __u32 hash;              /* skb hash value */
    };

    /* ── Bit fields (packed flags) ────────────────────────────── */
    __u8   pkt_type:3,
           pfmemalloc:1,
           cloned:1,
           nohdr:1,
           fclone:2;

    __u8   ip_summed:2,
           ooo_okay:1,
           l4_hash:1,
           sw_hash:1,
           wifi_acked_valid:1,
           wifi_acked:1,
           no_fcs:1;

    /* ── Priority and QoS ─────────────────────────────────────── */
    __u32 priority;
    int   skb_iif;      /* incoming interface index */
};
```

**sk_buff Nesting Tree:**

```
sk_buff
├── union [list linkage]
│   ├── struct
│   │   ├── sk_buff *next
│   │   ├── sk_buff *prev
│   │   └── union
│   │       ├── net_device *dev
│   │       └── unsigned long dev_scratch
│   ├── rb_node rbnode
│   └── list_head list
│       ├── list_head *next
│       └── list_head *prev
├── sock *sk
├── union [timestamps]
│   ├── ktime_t tstamp
│   └── u64 skb_mstamp_ns
├── unsigned char *head / *data / *tail / *end  [4 pointers]
├── unsigned int len, data_len
├── __u16 mac_len, hdr_len
├── union [checksum]
│   ├── __wsum csum
│   └── struct { csum_start; csum_offset }
├── __be16 protocol
├── __u16 transport_header / network_header / mac_header
├── union [mark]  { mark | reserved_tailroom }
├── union [hash]  { hash }
└── __u8 pkt_type:3, pfmemalloc:1, cloned:1, nohdr:1, fclone:2
    __u8 ip_summed:2, ooo_okay:1, l4_hash:1, sw_hash:1, ...
```

### 17.2 task_struct: Nested Scheduling Entities

```c
/* task_struct contains nested scheduling info — three schedulers, each
   with their own struct embedded in the task */

struct task_struct {
    /* ── CFS scheduler entity ─────────────────────────────────── */
    struct sched_entity se;      /* CFS (Completely Fair Scheduler) */
    /*
       sched_entity contains:
         struct load_weight load;     ← nested: weight for scheduling
         struct rb_node     run_node; ← nested: position in CFS red-black tree
         u64    exec_start;           ← last execution start time
         u64    sum_exec_runtime;     ← total runtime
         u64    vruntime;             ← virtual runtime (for CFS fairness)
    */

    /* ── RT scheduler entity ──────────────────────────────────── */
    struct sched_rt_entity  rt;   /* Real-Time scheduler */
    /*
       sched_rt_entity contains:
         struct list_head run_list;   ← nested: position in RT run queue
         unsigned long    timeout;
         unsigned long    watchdog_stamp;
         unsigned int     time_slice;
    */

    /* ── DL scheduler entity ──────────────────────────────────── */
    struct sched_dl_entity  dl;   /* Deadline scheduler */
    /*
       sched_dl_entity contains:
         struct rb_node   rb_node;    ← nested: position in DL tree
         u64              dl_runtime; ← WCET (worst-case execution time)
         u64              dl_deadline;
         u64              dl_period;
         u64              dl_bw;      ← bandwidth = runtime/period
    */

    /* Which scheduler is active? Not a union — all three entities exist
       simultaneously. The active scheduler is determined by the task's policy:
         SCHED_OTHER → uses se
         SCHED_FIFO / SCHED_RR → uses rt
         SCHED_DEADLINE → uses dl
    */
    const struct sched_class *sched_class;  /* pointer to scheduler vtable */

    /* ── Process hierarchy ────────────────────────────────────── */
    pid_t                pid;
    struct task_struct  *real_parent;   /* pointer to self type (recursive!) */
    struct list_head     children;      /* list_head: prev+next ptrs */
    struct list_head     sibling;       /* list_head: prev+next ptrs */

    /* ── Namespace nesting ───────────────────────────────────────*/
    struct nsproxy      *nsproxy;       /* container namespaces */
    /*
       nsproxy contains pointers to:
         struct uts_namespace   *uts_ns;   /* hostname etc. */
         struct ipc_namespace   *ipc_ns;   /* SysV IPC, POSIX MQ */
         struct mnt_namespace   *mnt_ns;   /* mount points */
         struct pid_namespace   *pid_ns_for_children;
         struct net             *net_ns;   /* network stack */
    */
};
```

### 17.3 The kobject Embedding Pattern

```c
/*
 Every kernel object (device, driver, class, etc.) embeds a kobject.
 This provides: reference counting, sysfs integration, hot-plug events.

 The pattern: embed kobject as the FIRST member. Then casting is safe.
*/

struct kobject {
    const char          *name;      /* object name in sysfs */
    struct list_head     entry;     /* linked list of kset siblings */
    struct kobject      *parent;    /* parent in kobject hierarchy */
    struct kset         *kset;      /* collection this object belongs to */
    const struct kobj_type *ktype;  /* operations (sysfs show/store, release) */
    struct kernfs_node  *sd;        /* sysfs directory entry */
    struct kref          kref;      /* reference count */
    unsigned int state_initialized:1;
    unsigned int state_in_sysfs:1;
    unsigned int state_add_uevent_sent:1;
    unsigned int state_remove_uevent_sent:1;
    unsigned int uevent_suppress:1;
};

struct kref {
    refcount_t refcount;   /* atomic reference count */
};

/* device embeds kobject: */
struct device {
    struct kobject   kobj;     /* MUST be first — enables safe casting */
    struct device   *parent;
    struct device_private *p;
    const char      *init_name;
    const struct device_type *type;
    struct bus_type *bus;
    struct device_driver *driver;
    void            *driver_data;
    /* ... */
};

/* platform_device embeds device: */
struct platform_device {
    const char   *name;
    int           id;
    bool          id_auto;
    struct device dev;       /* MUST be first — enables safe casting */
    u64          *dma_mask;
    struct resource *resource;
    unsigned int   num_resources;
    /* ... */
};

/*
 Casting hierarchy:
   platform_device → device → kobject (via first-member embedding)

   platform_device *pdev = ...;
   struct device   *dev  = &pdev->dev;                  /* embed access */
   struct kobject  *kobj = &pdev->dev.kobj;             /* double embed */
   struct device   *back = container_of(kobj, struct device, kobj);  /* reverse */
   struct platform_device *pback = to_platform_device(dev);
     → which uses: container_of(dev, struct platform_device, dev)
*/
```

---

## 18. Network Subsystem: sk_buff Nested Anatomy

The complete packet receive path shows how nested structs flow through the network stack:

```
PACKET JOURNEY — Nested Structs in Action:

NIC Driver
│  Allocates sk_buff:
│    skb = dev_alloc_skb(len)
│    skb->dev = net_device*       ← nested union access
│
│  DMA fills buffer between skb->data and skb->tail
│
▼
netif_receive_skb(skb)            ← L2 entry
│  Calls eth_type_trans(skb, dev):
│    skb->mac_header = skb->data - skb->head (offset)
│    skb->protocol = eth_hdr->h_proto
│      where: eth_hdr = (struct ethhdr*)skb->data
│                      = (struct ethhdr*)(skb->head + skb->mac_header)
│  Calls skb_pull(skb, ETH_HLEN):
│    skb->data += 14   (skip L2 header — now points to IP header)
│
▼
ip_rcv(skb, dev, pt, orig_dev)    ← L3 entry (net/ipv4/ip_input.c)
│  struct iphdr *iph = ip_hdr(skb):
│    = (struct iphdr*)(skb->head + skb->network_header)
│    struct iphdr:
│      version:4, ihl:4           (nested bit fields in struct)
│      tos, tot_len, id           (flat fields)
│      frag_off                   (flags+offset packed in __be16)
│      ttl, protocol, check       (flat fields)
│      saddr, daddr               (flat fields)
│  Verifies: iph->ihl >= 5, iph->version == 4
│  Calls ip_route_input(skb, ...):
│    Creates struct rtable (routing result)
│    rtable embeds struct dst_entry (destination cache)
│    skb_dst_set(skb, &rt->dst)   ← stores in skb->_skb_refdst (union)
│
▼
tcp_v4_rcv(skb)                   ← L4 entry (net/ipv4/tcp_ipv4.c)
│  struct tcphdr *th = tcp_hdr(skb):
│    = (struct tcphdr*)(skb->head + skb->transport_header)
│    struct tcphdr:
│      source, dest, seq, ack_seq  (flat fields)
│      doff:4, flags bits           (nested bit fields)
│      window, check, urg_ptr       (flat fields)
│  sk = __inet_lookup_skb(...)     ← hash table lookup
│    Returns struct sock* (actually tcp_sock* — 5-level nesting!)
│    tcp_sock.inet_conn.inet.sk.__sk_common.skc_state == TCP_ESTABLISHED ?
│
▼
tcp_rcv_established(sk, skb, th)
│  struct tcp_sock *tp = tcp_sk(sk)  ← cast sock* to tcp_sock* (safe, first-member)
│  Access TCP control block:
│    tp->rcv_nxt   (next expected sequence)
│    tp->snd_wnd   (send window)
│    tp->srtt_us   (smoothed RTT)
│
▼
sock_queue_rcv_skb(sk, skb)
│  __skb_queue_tail(&sk->sk_receive_queue, skb)
│    sk_receive_queue is struct sk_buff_head:
│      struct sk_buff_head {
│          struct sk_buff *next;
│          struct sk_buff *prev;
│          __u32           qlen;    ← number of skbs in queue
│      };
│  sk->sk_data_ready(sk)            ← wake up process waiting on recv()
│
▼
Application: recv(fd, buf, len, 0)
  tcp_recvmsg() → copy data from sk_receive_queue to user buffer
```

**Accessor Macro Pattern (How skb headers are accessed):**

```c
/* These macros compute pointer from skb->head + stored offset */

static inline struct ethhdr *eth_hdr(const struct sk_buff *skb) {
    return (struct ethhdr *)skb_mac_header(skb);
}

static inline unsigned char *skb_mac_header(const struct sk_buff *skb) {
    return skb->head + skb->mac_header;
    /* skb->mac_header is a __u16 offset — small, cache-friendly */
}

static inline struct iphdr *ip_hdr(const struct sk_buff *skb) {
    return (struct iphdr *)skb_network_header(skb);
}

static inline unsigned char *skb_network_header(const struct sk_buff *skb) {
    return skb->head + skb->network_header;
}

static inline struct tcphdr *tcp_hdr(const struct sk_buff *skb) {
    return (struct tcphdr *)skb_transport_header(skb);
}

/* This design stores OFFSETS (u16) not POINTERS (u64) — saves 24 bytes
   compared to storing three pointers, and avoids pointer invalidation
   when the skb is reallocated (head changes, offsets remain valid). */
```

---

## 19. Performance: Nesting and Cache Behavior

### 19.1 Hot/Cold Field Splitting in Nested Structs

```c
/* PROBLEM: A struct with both hot (frequently accessed) and
   cold (rarely accessed) data hurts cache performance.
   Even if you only access the hot fields, the whole struct's
   cache line(s) are loaded. */

/* BAD: Hot and cold data mixed */
struct Connection {
    int        fd;           /* HOT: checked on every syscall */
    uint32_t   state;        /* HOT: checked on every operation */
    char       name[64];     /* COLD: only for error messages/logging */
    uint64_t   stats[16];    /* COLD: updated every second */
    uint32_t   timeout_ms;   /* WARM: set once, rarely changed */
    /* Total: 4+4+64+128+4 = 204 bytes — spans 4 cache lines */
};
/* Accessing fd requires loading 64-byte cache line that includes cold data */

/* GOOD: Nested hot/cold separation */
struct ConnectionHot {
    int      fd;
    uint32_t state;
    uint32_t timeout_ms;
    uint32_t _pad;           /* pad to 16 bytes */
};  /* fits in < 1 cache line */

struct ConnectionCold {
    char     name[64];
    uint64_t stats[16];
};

struct ConnectionOptimized {
    struct ConnectionHot  hot;   /* frequently accessed — stays hot in cache */
    struct ConnectionCold *cold; /* rarely accessed — loaded only when needed */
};
/* hot = 16 bytes (1/4 cache line), cold pointer = 8 bytes — total = 24 bytes */
/* Accessing fd only loads 1 cache line; cold data only loaded if needed */
```

### 19.2 False Sharing with Nested Structs in Multi-Core

```c
/* PROBLEM: Two threads access different NESTED STRUCTS that happen
   to share a cache line → false sharing → cache line bouncing */

struct TwoCounters {
    struct { uint64_t val; } counter_a;   /* Thread A writes this */
    struct { uint64_t val; } counter_b;   /* Thread B writes this */
};
/* Both counters are 8 bytes each, total 16 bytes — ONE cache line!
   Thread A writing counter_a invalidates Thread B's cache copy → SLOW */

/* FIX: Pad to separate cache lines */
#define CACHELINE_ALIGNED __attribute__((aligned(64)))

struct TwoCountersFast {
    struct { uint64_t val; } counter_a CACHELINE_ALIGNED;
    struct { uint64_t val; } counter_b CACHELINE_ALIGNED;
};
/* Now counter_a and counter_b are on different cache lines.
   Thread A and Thread B never invalidate each other. */

/* Linux kernel macro: */
#define ____cacheline_aligned __attribute__((__aligned__(SMP_CACHE_BYTES)))

/* In network driver (struct net_device): */
struct netdev_queue {
    struct net_device  *dev;
    struct Qdisc       *qdisc;        /* TX queue discipline */
    struct Qdisc       *qdisc_sleeping;
    /* ... TX state ... */
} ____cacheline_aligned_in_smp;      /* each TX queue on its own cache line */
```

### 19.3 Enum Dispatch vs Virtual Table in Nested Context

```rust
// When a struct contains an enum that drives dispatch,
// the compiler can often inline the dispatch — no indirection.

struct Processor {
    mode:   ProcessingMode,   // enum — 1 byte
    buffer: Vec<u8>,
}

#[derive(Debug)]
enum ProcessingMode {
    Fast,
    Safe,
    Debug,
}

impl Processor {
    fn process(&self, data: &[u8]) -> Vec<u8> {
        // The compiler sees self.mode is an enum → branch prediction friendly
        // and can even specialize if there's only one variant in a monomorphized context
        match self.mode {
            ProcessingMode::Fast  => fast_process(data),
            ProcessingMode::Safe  => safe_process(data),
            ProcessingMode::Debug => debug_process(data),
        }
    }
}

// Alternative: trait object — heap pointer + vtable pointer = 16 bytes
// Every call goes through vtable → extra indirection
struct DynProcessor {
    handler: Box<dyn ProcessHandler>,   // 16 bytes: vtable ptr + data ptr
    buffer:  Vec<u8>,
}

trait ProcessHandler {
    fn process(&self, data: &[u8]) -> Vec<u8>;
}
```

---

## 20. Common Mistakes and Pitfalls

### 20.1 Mistake: Reading Wrong Union Member Without Checking Tag

```c
/* BUG: no tag check before union access */
union Data d;
d.as_float = 3.14f;
printf("%d\n", d.as_int);  /* UB: reading int after writing float */

/* FIX: always check tag */
struct TaggedData td;
td.tag = TAG_FLOAT;
td.data.as_float = 3.14f;

if (td.tag == TAG_INT) {
    printf("%d\n", td.data.as_int);  /* safe: tag checked */
}
```

### 20.2 Mistake: Taking Address of Anonymous Union Member

```c
struct Foo {
    int type;
    union {
        int   i;
        float f;
    };
};

struct Foo foo;
int *p = &foo.i;    /* OK — can take address of anonymous member */
/* But you cannot take address of the anonymous union itself: */
/* void *q = &foo.(union{...});  ← syntax error */
```

### 20.3 Mistake: Assuming Nested Struct Has No Padding Cost

```c
/* WRONG assumption: "nested struct costs nothing" */
struct Inner { char a; double b; };   /* sizeof = 16, alignof = 8 */
struct Outer { char x; struct Inner in; char y; };

/* Outer layout: x@0, pad@1-7, in@8-23, y@24, pad@25-31 → sizeof = 32 */
/* If you thought nested struct "just adds 16 bytes", you'd expect 18 bytes — WRONG */
/* The boundary padding (7 bytes before in) and tail padding (7 bytes after y) apply */
```

### 20.4 Mistake: Recursive Struct Without Pointer in C

```c
/* WRONG: cannot embed struct directly in itself */
struct Node {
    int         val;
    struct Node child;  /* ERROR: incomplete type, infinite size */
};

/* CORRECT: use pointer */
struct Node {
    int         val;
    struct Node *child;  /* pointer has fixed size (8 bytes) */
};
```

### 20.5 Mistake: Rust Enum with Large Variants

```rust
// BAD: one large variant inflates the entire enum
enum Command {
    Start,
    Stop,
    Configure([u8; 4096]),  // 4096 bytes — the whole enum is now 4096+ bytes!
}
// sizeof = 4096+ bytes. Even Command::Start wastes 4096 bytes.

// GOOD: Box the large variant
enum Command {
    Start,
    Stop,
    Configure(Box<[u8; 4096]>),  // 8 bytes (pointer) — heap allocated on demand
}
// sizeof = 16 bytes (tag + pointer)
```

### 20.6 Mistake: Bit Fields for Network Data

```c
/* WRONG: bit field order is implementation-defined — not portable */
struct BadIpFlags {
    unsigned int mf     : 1;  /* More Fragments */
    unsigned int df     : 1;  /* Don't Fragment */
    unsigned int res    : 1;  /* Reserved */
    unsigned int offset : 13;
};

/* On a different compiler/architecture, this bit layout may differ */

/* CORRECT: use explicit bit masks on integers */
#define IP_MF     0x2000  /* More Fragments */
#define IP_DF     0x4000  /* Don't Fragment */
#define IP_OFFSET 0x1FFF  /* Fragment Offset mask */

uint16_t frag_off = ntohs(iph->frag_off);
int mf     = (frag_off & IP_MF)     != 0;
int df     = (frag_off & IP_DF)     != 0;
int offset = (frag_off & IP_OFFSET) * 8; /* in bytes */
```

### 20.7 Mistake: Go — Modifying a Value-Receiver Embedded Struct

```go
type Inner struct { X int }
type Outer struct { Inner; Y int }

func (i Inner) SetX(v int) { i.X = v }  // VALUE receiver — modifies COPY

o := Outer{Inner: Inner{X: 0}, Y: 1}
o.SetX(42)
fmt.Println(o.X)  // 0 — the Set had no effect!

// FIX: pointer receiver
func (i *Inner) SetX(v int) { i.X = v }
o.SetX(42)
fmt.Println(o.X)  // 42
```

### 20.8 Mistake: Assuming C Anonymous Struct Members Are Named in C89/C90

```c
/* Anonymous structs/unions are C11. In C89/C99 without extensions:
   GCC allows them as an extension, but it is NOT standard C99.
   For portability, name the field or use -std=c11 */

/* C89/C99 incompatible (GCC extension): */
struct Foo {
    union { int a; float b; };  /* anonymous — GCC extension, not C99 standard */
};

/* Portable C99: */
struct Foo99 {
    union { int a; float b; } u;  /* named field */
};
foo99.u.a = 42;
```

---

## 21. Quick Reference Decision Matrix

### Which Nesting Pattern to Use?

```
QUESTION                                    PATTERN                   EXAMPLE
─────────────────────────────────────────────────────────────────────────────────────────
"I want to GROUP related fields together"   struct-in-struct          Point inside Rect

"I want to REUSE a struct by embedding it"  struct-in-struct          sock in inet_sock
 (with same-type casting)                   (first-member rule)

"I want Go-style COMPOSITION with           struct-in-struct +         embedding in Go,
 promoted field access"                     anonymous/embedding        kobject in device

"I have a field that can be ONE of several  union-in-struct            tagged union Value
 types at runtime — I'll track which"       (tagged union)             AstNode, sk_buff

"I need to VIEW the same bytes as           struct-in-union            EndpointV4/Mac
 different structured types"                (overlay)                  IP/TCP dual view

"I need a compile-time SAFE tagged union"   Rust enum with struct      enum Message,
 with exhaustive matching"                  variants                   Result, Option

"I have many flags/bits that need to        bit-fields-in-struct       TcpFlags in tcphdr
 be compact"                                (nested bit fields)        sk_buff flags

"I need a VALUE that is an error or a       enum-in-struct (C) or      TcpState in
 state — with type-safe access"             enum in Rust               TcpConnection

"I need hierarchical categorization         enum-in-enum (Rust)        AppError {
 of types/errors"                                                         Network(NetErr),
                                                                          Auth(AuthErr) }

"I need a variable-length payload           FAM in struct              sk_buff with
 after a fixed header"                      (flexible array member)    skb_shared_info

"I need to embed list/tree links in         self-referential struct    list_head in
 a struct without extra allocation"          + container_of            task_struct
```

### Memory Size Reference for Common Patterns

```
Pattern                          C Size              Rust Size           Go Size
───────────────────────────────────────────────────────────────────────────────────
struct { int; int; }             8                   8 (or less)         8
struct { char; int; }            8 (3B pad)          8 (reordered)       8 (3B pad)
union { int; double; }           8                   8                   N/A (unsafe)
struct { int tag; union{int;     12 (no pad if        varies             24 (all fields)
         double;} }              tag+int fit)
enum (4 variants, no data)       4 (C int)           1 byte              8 (int)
Rust enum { None; Some(u64) }    N/A                 16 (niche→8 for    N/A
                                                       Option<NonZero>)
Rust enum { A(u8); B([u8;1024])} N/A                 1026 (use Box!)    N/A
Linked list node (ptr+data)      data+8 (ptr)        data+8 (ptr)       data+8 (ptr)
Recursive tree node              data+16 (2 ptrs)    data+16 (2 Box)    data+16 (2 ptrs)
```

### Golden Rules for Nested Types

```
RULE 1 — MEMORY:
  The size of a struct/union is determined by a RECURSIVE algorithm.
  Always trace through ALL levels of nesting when computing sizeof.

RULE 2 — ALIGNMENT PROPAGATION:
  A nested struct's alignment requirement propagates to the outer struct.
  The outer struct's alignment = max(alignment of ALL nested fields, recursively).

RULE 3 — TAG BEFORE UNION:
  In C, ALWAYS check the tag/discriminant before reading a union member.
  In Rust, the compiler enforces this via exhaustive match.

RULE 4 — ANONYMOUS = SYNTACTIC SUGAR:
  Anonymous structs/unions have IDENTICAL memory layout to named ones.
  They only remove the intermediate field name from access syntax.

RULE 5 — FIRST-MEMBER RULE (Linux Kernel):
  If struct B starts with struct A as its first member, (A*)b == b.
  This enables safe casting between embedded struct types.
  container_of reverses this: given A*, recover the outer B*.

RULE 6 — RECURSIVE = POINTER:
  A struct cannot contain itself directly (infinite size).
  A struct CAN contain a POINTER to itself (8 bytes on 64-bit).
  Rust enum uses Box<T> for the same reason.

RULE 7 — RUST ENUM SIZE TRAP:
  The size of a Rust enum = size of LARGEST variant + discriminant.
  One large variant inflates ALL variants.
  Box large variants: enum Cmd { Big(Box<LargeData>) }

RULE 8 — NEVER BIT FIELDS FOR WIRE FORMAT:
  Bit field layout is implementation-defined. For network protocols,
  use integer types with explicit bit masks. This is portable.

RULE 9 — HOT/COLD SPLIT:
  If a nested struct mixes frequently-accessed and rarely-accessed fields,
  split into hot struct (small, fits in cache line) +
  pointer to cold struct (heap, loaded only when needed).

RULE 10 — GO EMBEDDING IS NOT INHERITANCE:
  Go embedding promotes fields and methods, but the embedded type
  has NO knowledge of the outer type. There is no 'super' pointer.
  Methods on the embedded type cannot access the outer struct's fields.
```

---

*End of Guide — Nested Struct, Enum, and Union: Complete In-Depth Coverage*

> **Cognitive Principle:** The mental model you need is **compositional thinking** —
> each nested type is just bytes at a computed offset. Once you internalize the layout
> algorithm (alignment, padding, union overlay), you can reason about ANY combination
> at any depth without guessing. Practice computing sizeof and offsetof by hand for
> every new nested type you encounter — this builds the *chunking* pattern that separates
> expert systems programmers from the rest.

Good intuition, but this mental model needs refinement for production systems work. Let me push your thinking deeper with some guiding questions, then we can solidify the framework.

## Initial Diagnostic Questions

1. **For `struct`**: You said "fields laid out sequentially." True, but:
   - What happens between fields? (Hint: padding and alignment)
   - In kernel network code, when you parse a packet header as a struct, what guarantees do you have about layout?
   - What's the difference between `struct` and `#[repr(C)]` struct? Why would that matter for a device driver?

2. **For `enum`**: You said "variants with discriminant." Good, but:
   - When you match on an enum, what's the actual cost? (How big is that discriminant? How does it affect memory layout?)
   - In error handling paths, what's the difference between `Option<T>` and `Result<T, E>` from a *systems* perspective?
   - If you're writing a network state machine, how would enum variants represent state transitions? What do you lose if you just use an `enum` vs. combining it with embedded types?

3. **For `union`**: This is where it gets interesting:
   - You said "reinterpret memory" — but *who* determines that reinterpretation? Hardware? Protocol specs? Your code?
   - In kernel work (e.g., parsing IPv4/IPv6 headers, hardware ring buffers), when is `union` the right choice vs. when should you avoid it?
   - What does Rust's safety model say about unions, and why?

## The Real Production Question

Here's what I want you to think about:

**Your current framework treats these as isolated solutions. But in real systems (especially kernel/protocol code), you often need *compositions*.**

For example, a network packet header might need:
- A fixed set of common fields (struct)
- One of several format variants (enum)
- The ability to reinterpret the same bytes in different ways depending on protocol version (union or unsafe reinterpretation)

**So the question becomes: How do you *combine* these constructs, and what are the trade-offs?**

---

## What I Need From You

Before I give you a refined model, tell me:

1. **What triggered this question?** Are you working on parsing something specific? Designing an error type? Building a state machine?

2. **In what context would you use each?** Give me a specific example from kernel/network code where you'd reach for:
   - A plain struct
   - An enum with associated data
   - A union or unsafe reinterpretation

3. **What's your concern?** Are you unsure about *when* to use each, or about *how they work under the hood*?

Once I understand your actual problem space, we can build a mental model that covers:
- Memory layout guarantees and padding
- Safety/unsafety boundaries
- Performance implications
- Composition patterns for complex protocol structures
- Testing strategies for these constructs

Sound good? What's the concrete problem you're solving?

# Struct, Enum, Union: Production Systems View

## **STRUCT** — "I need several things at once"

**Core idea**: Multiple independent values, each occupies its own memory space, laid out sequentially (with alignment padding).

**Memory reality**:
```
struct Packet {
    src_ip: u32,      // 4 bytes @ offset 0
    dst_ip: u32,      // 4 bytes @ offset 4
    port: u16,        // 2 bytes @ offset 8
    flags: u8,        // 1 byte @ offset 10
    // 1 byte padding @ offset 11 (alignment)
}
// Total: 12 bytes (not 11)
```

**Layout guarantees**:
- Default Rust: compiler chooses optimal layout (reorders fields). Fields may not be in declaration order.
- `#[repr(C)]`: fields in declaration order, C-compatible layout. **Required for hardware/protocol representations**.
- `#[repr(packed)]`: no padding, but loses alignment guarantees (unsafe, slower memory access).

**When to use**:
- Collecting related data together
- Hardware registers or memory-mapped I/O
- Protocol headers with fixed fields (use `#[repr(C)]`)

---

## **ENUM** — "I need one of several things"

**Core idea**: Exactly one variant is active at a time. Rust tags it with a discriminant (an internal integer identifying which variant). All variants share the same memory location, sized to fit the largest.

**Memory reality**:
```
enum Result<T, E> {
    Ok(T),        // variant 0
    Err(E),       // variant 1
}
// Size: sizeof(discriminant) + sizeof(max(T, E))
// Example: Result<u32, String> ≈ 1 + 32 = 33 bytes (aligned)
```

**Key points**:
- Discriminant is automatic and invisible; Rust enforces which variant you access.
- `Option<T>` exploits null-pointer optimization: `Option<&T>` has zero discriminant overhead.
- Enum with associated data binds state to values: `enum State { Ready(Config), Waiting(Timeout), Failed(Error) }`

**When to use**:
- Mutually exclusive states or outcomes
- Type-safe error handling
- State machines where each state carries different data
- Protocol message types (parse one of N message formats)

---

## **UNION** — "I need to reinterpret memory"

**Core idea**: All fields occupy the *same* memory space. Writing to one field overwrites others. Rust wraps it in `unsafe` because reinterpreting bytes is inherently unsafe.

**Memory reality**:
```
union IpHeader {
    as_u32: u32,          // all 4 bytes
    as_bytes: [u8; 4],    // same 4 bytes
    as_fields: IPv4 {
        version: u8,
        ihl: u8,
        dscp: u8,
        ecn: u8,
    }
}
// Size: 4 bytes (the largest field)
```

**Key points**:
- Read is undefined behavior if you write via one field and read via another (violates Rust's type safety).
- Valid only when reinterpreting bytes per spec (e.g., hardware layout, protocol standards).
- Requires `unsafe { }` blocks.

**When to use**:
- Hardware registers with overlapping bit fields
- Protocol headers where the same bytes represent different structures based on a format field
- Network packet parsing (IPv4 vs IPv6, TCP vs UDP)

---

## **Production Composition Pattern**

Real systems combine all three:

```
enum IpPacket {
    V4(#[repr(C)] struct IPv4Header { ... }),
    V6(#[repr(C)] struct IPv6Header { ... }),
}

struct NetworkFrame {
    link_header: #[repr(C)] struct EthernetHeader { ... },
    payload: IpPacket,
}
```

- **Struct** (`#[repr(C)]`): hardware/protocol fixed layouts
- **Enum**: mutually exclusive protocol variants
- **Union**: only when hardware/spec mandates byte reinterpretation, wrapped in safe abstractions

---

## **Key Trade-offs**

| Construct | Size Overhead | Safety | Flexibility | Use in Kernel |
|-----------|---------------|--------|-------------|---------------|
| **Struct** | Field sum + padding | Safe | High | Yes (primary) |
| **Enum** | Largest variant + discriminant | Safe | High | Yes (state/types) |
| **Union** | Largest field only | Unsafe | Low (reinterpretation only) | Rarely (last resort) |

For cloud network security and kernel work: **prefer struct + enum composition**. Use union only when hardware or protocol specs literally require byte reinterpretation, and wrap it in safe abstractions.

# Code Examples: Struct, Enum, Union in Systems Programming

## STRUCT Examples

### 1. Protocol Header with `#[repr(C)]` (Fixed Layout)

```rust
// Ethernet frame header — must match wire format exactly
#[repr(C)]
struct EthernetHeader {
    dst_mac: [u8; 6],      // 6 bytes
    src_mac: [u8; 6],      // 6 bytes
    ethertype: u16,        // 2 bytes, big-endian
}
// Total: 14 bytes, no padding

// IPv4 header — fixed fields, network byte order
#[repr(C)]
struct IPv4Header {
    version_ihl: u8,       // version (4 bits) + IHL (4 bits)
    dscp_ecn: u8,          // DSCP (6 bits) + ECN (2 bits)
    total_length: u16,     // big-endian
    identification: u16,
    flags_offset: u16,     // flags (3 bits) + fragment offset (13 bits)
    ttl: u8,
    protocol: u8,
    checksum: u16,
    src_ip: u32,           // big-endian
    dst_ip: u32,           // big-endian
}
// Total: 20 bytes minimum

// Usage: parse raw bytes from network device
fn parse_ipv4_packet(buffer: &[u8]) -> Option<&IPv4Header> {
    if buffer.len() < std::mem::size_of::<IPv4Header>() {
        return None;
    }
    // SAFETY: buffer is aligned and large enough; IPv4Header is #[repr(C)]
    unsafe {
        Some(&*(buffer.as_ptr() as *const IPv4Header))
    }
}
```

### 2. Device Register Structure (Memory-Mapped I/O)

```rust
// Example: Network device RX ring descriptor (Intel 82599 style)
#[repr(C)]
struct RxDescriptor {
    buffer_addr: u64,      // DMA address of packet buffer
    header_addr: u64,      // DMA address of header buffer
    pkt_len: u16,          // packet length
    hdr_len: u16,          // header length
    status: u16,           // descriptor status flags
    errors: u16,           // error codes
    vlan_tag: u16,         // VLAN info
}

// Memory-mapped device registers
#[repr(C)]
struct NicRegisters {
    ctrl: u32,             // Control register @ offset 0x0000
    status: u32,           // Status register @ offset 0x0008
    rctl: u32,             // RX control @ offset 0x0100
    rdlen: u32,            // RX descriptor ring length
    rdh: u32,              // RX descriptor head
    rdt: u32,              // RX descriptor tail
    rx_ring: u64,          // RX ring base address
}

// Unsafe access to device memory
fn enable_rx_ring(base: *mut NicRegisters, num_descriptors: u32) {
    unsafe {
        (*base).rdlen = num_descriptors as u32 * std::mem::size_of::<RxDescriptor>() as u32;
        (*base).rctl |= 0x00000002; // RCTL.EN = 1 (enable)
    }
}
```

### 3. Configuration Structure (Regular Data Struct)

```rust
#[derive(Clone)]
struct NetworkSecurityPolicy {
    ingress_acl: Vec<AclRule>,
    egress_acl: Vec<AclRule>,
    rate_limit_mbps: u32,
    encryption_enabled: bool,
    logging_level: LogLevel,
    timeout_seconds: u64,
}

impl NetworkSecurityPolicy {
    fn validate(&self) -> Result<(), PolicyError> {
        if self.rate_limit_mbps == 0 {
            return Err(PolicyError::InvalidRateLimit);
        }
        if self.timeout_seconds > 86400 {
            return Err(PolicyError::TimeoutTooLarge);
        }
        Ok(())
    }
}
```

---

## ENUM Examples

### 1. Protocol Message Types

```rust
enum EthernetPayload {
    Ipv4(Ipv4Packet),
    Ipv6(Ipv6Packet),
    Arp(ArpPacket),
    Vlan(VlanTag),
    Unknown(u16),  // ethertype value
}

struct Ipv4Packet {
    header: IPv4Header,
    payload: Ipv4TransportPayload,
}

enum Ipv4TransportPayload {
    Tcp(TcpSegment),
    Udp(UdpSegment),
    Icmp(IcmpMessage),
    Other(u8),  // protocol number
}

struct TcpSegment {
    header: TcpHeader,
    flags: TcpFlags,
    payload: Vec<u8>,
}

// Safe, type-driven parsing
fn process_packet(frame: &[u8]) -> Result<(), ParseError> {
    let eth_header = parse_ethernet_header(frame)?;
    
    match EthernetPayload::from_bytes(&frame[14..], eth_header.ethertype) {
        EthernetPayload::Ipv4(ipv4_pkt) => {
            log::info!("IPv4 packet from {}", ipv4_pkt.header.src_ip);
            match &ipv4_pkt.payload {
                Ipv4TransportPayload::Tcp(tcp_seg) => handle_tcp(tcp_seg)?,
                Ipv4TransportPayload::Udp(udp_seg) => handle_udp(udp_seg)?,
                Ipv4TransportPayload::Icmp(icmp_msg) => handle_icmp(icmp_msg)?,
                _ => log::debug!("Unknown transport protocol"),
            }
        }
        EthernetPayload::Ipv6(ipv6_pkt) => handle_ipv6(ipv6_pkt)?,
        EthernetPayload::Unknown(ethertype) => {
            log::warn!("Unknown ethertype: {:#06x}", ethertype);
        }
        _ => {}
    }
    Ok(())
}
```

### 2. State Machine with Associated Data

```rust
enum ConnectionState {
    Closed,
    Listen {
        backlog: u16,
    },
    SynRcvd {
        seq: u32,
        ack_seq: u32,
        timestamp: u64,
    },
    Established {
        seq: u32,
        ack_seq: u32,
        window_size: u16,
        last_activity: u64,
    },
    FinWait1 {
        seq: u32,
        ack_seq: u32,
    },
    FinWait2 {
        ack_seq: u32,
    },
    TimeWait {
        deadline: u64,
    },
    Closed {
        reason: String,
    },
}

fn transition_state(current: &mut ConnectionState, event: TcpEvent) -> Result<(), StateError> {
    *current = match (current.clone(), event) {
        (ConnectionState::Listen { backlog }, TcpEvent::SynReceived { seq, ack_seq }) => {
            ConnectionState::SynRcvd { seq, ack_seq, timestamp: current_time() }
        }
        (ConnectionState::SynRcvd { seq, ack_seq, .. }, TcpEvent::AckReceived { .. }) => {
            ConnectionState::Established {
                seq,
                ack_seq,
                window_size: 65535,
                last_activity: current_time(),
            }
        }
        (ConnectionState::Established { .. }, TcpEvent::FinReceived { ack_seq }) => {
            ConnectionState::FinWait1 { seq: next_seq(), ack_seq }
        }
        (state, event) => {
            return Err(StateError::InvalidTransition {
                current_state: format!("{:?}", state),
                event: format!("{:?}", event),
            })
        }
    };
    Ok(())
}
```

### 3. Error Handling with Enums

```rust
#[derive(Debug)]
enum NetworkError {
    ProtocolViolation {
        protocol: String,
        detail: String,
    },
    Timeout {
        elapsed_ms: u64,
        expected_ms: u64,
    },
    ChecksumMismatch {
        computed: u16,
        received: u16,
    },
    BufferTooSmall {
        required: usize,
        available: usize,
    },
    SecurityViolation {
        rule_id: u32,
        src_ip: u32,
        dst_ip: u32,
    },
    Unknown(String),
}

fn verify_tcp_checksum(segment: &TcpSegment, pseudo_header: &PseudoHeader) -> Result<(), NetworkError> {
    let computed = compute_checksum(segment, pseudo_header);
    if computed != segment.header.checksum {
        return Err(NetworkError::ChecksumMismatch {
            computed,
            received: segment.header.checksum,
        });
    }
    Ok(())
}

// Caller handles each error type appropriately
match verify_tcp_checksum(&segment, &pseudo_header) {
    Ok(_) => process_segment(&segment),
    Err(NetworkError::ChecksumMismatch { computed, received }) => {
        log::error!("Bad checksum: computed {:#06x}, got {:#06x}", computed, received);
        drop_packet();
    }
    Err(e) => {
        log::error!("TCP verification failed: {:?}", e);
    }
}
```

---

## UNION Examples

### 1. Hardware Register with Overlapping Bit Fields

```rust
// Some NICs expose registers that can be read as u32 or as bit fields
#[repr(C)]
union StatusRegister {
    raw: u32,
    fields: StatusFields,
}

#[repr(C)]
struct StatusFields {
    link_status: u8,       // bits 0-0
    speed: u8,             // bits 1-2 (00=10Mb, 01=100Mb, 10=1Gb, 11=reserved)
    duplex: u8,            // bit 3
    rx_enabled: u8,        // bit 4
    tx_enabled: u8,        // bit 5
    interrupt_pending: u8, // bit 6
    reserved: u16,
}

// UNSAFE: reinterpretation of bytes
fn check_link_status(base: *const u32) -> bool {
    unsafe {
        let reg = StatusRegister { raw: *base };
        reg.fields.link_status != 0
    }
}
```

### 2. Protocol Header with Variable Format (IPv4 Options)

```rust
// IPv4 header can have options (variable length)
// This is NOT ideal for union, but shows the pattern when hardware mandates it

#[repr(C)]
union IPv4OptionsUnion {
    as_bytes: [u8; 40],    // max IPv4 options: 15*4 - 20 = 40 bytes
    as_words: [u32; 10],   // for 32-bit processing
    timestamp: TimestampOption,
    route_record: RouteRecordOption,
}

#[repr(C)]
struct TimestampOption {
    type_code: u8,
    length: u8,
    pointer: u8,
    overflow_flag: u8,
    timestamp_data: [u32; 9],  // variable, but max 36 bytes for timestamps
}

// BETTER approach: don't use union, parse safely
enum IPv4Option {
    EndOfList,
    NoOp,
    SecurityRestricted,
    Timestamp { pointers: Vec<u32> },
    RecordRoute { addresses: Vec<u32> },
    Unknown { code: u8, data: Vec<u8> },
}

fn parse_ipv4_options(buffer: &[u8], ihl: u8) -> Result<Vec<IPv4Option>, ParseError> {
    let option_bytes = ((ihl as usize) * 4) - 20;
    if buffer.len() < option_bytes {
        return Err(ParseError::BufferTooSmall);
    }
    
    let mut options = Vec::new();
    let mut offset = 0;
    
    while offset < option_bytes {
        let code = buffer[offset];
        if code == 0 { // End of options
            break;
        }
        if code == 1 { // NOP
            offset += 1;
            continue;
        }
        
        let length = buffer.get(offset + 1).ok_or(ParseError::Incomplete)? as usize;
        let option_data = &buffer[offset..offset + length];
        
        let option = match code {
            68 => IPv4Option::Timestamp { pointers: parse_timestamps(option_data)? },
            7 => IPv4Option::RecordRoute { addresses: parse_addresses(option_data)? },
            _ => IPv4Option::Unknown { code, data: option_data.to_vec() },
        };
        
        options.push(option);
        offset += length;
    }
    
    Ok(options)
}
```

### 3. DMA Ring Descriptor with Interpreted Flags

```rust
// Network device RX descriptor: status flags can be read as bitfield or raw u16
#[repr(C)]
union DescriptorStatus {
    raw: u16,
    flags: DescriptorFlags,
}

#[repr(C)]
struct DescriptorFlags {
    descriptor_done: u8,     // bit 0
    end_of_packet: u8,       // bit 1
    ip_checksum_ok: u8,      // bit 2
    tcp_checksum_ok: u8,     // bit 3
    vp: u8,                  // bit 4 (VLAN present)
    ipv6: u8,                // bit 5
    tcp: u8,                 // bit 6
    udp: u8,                 // bit 7
    reserved: u8,
}

fn process_rx_descriptor(desc: &RxDescriptor) -> Result<PacketMetadata, Error> {
    unsafe {
        let status = DescriptorStatus { raw: desc.status };
        
        if status.flags.descriptor_done == 0 {
            return Err(Error::DescriptorNotReady);
        }
        
        let checksum_ok = status.flags.ip_checksum_ok != 0 && 
                         (status.flags.tcp_checksum_ok != 0 || status.flags.udp != 0);
        
        Ok(PacketMetadata {
            length: desc.pkt_len as usize,
            is_tcp: status.flags.tcp != 0,
            is_udp: status.flags.udp != 0,
            checksum_verified: checksum_ok,
            has_vlan: status.flags.vp != 0,
        })
    }
}
```

---

## Composition: Real-World Packet Parser

```rust
// All three constructs working together

#[repr(C)]
struct EthernetFrame {
    dst_mac: [u8; 6],
    src_mac: [u8; 6],
    ethertype: u16,
}

enum NetworkPacket {
    Ipv4 {
        header: IPv4Header,
        transport: TransportLayer,
    },
    Ipv6 {
        header: IPv6Header,
        transport: TransportLayer,
    },
    Arp(ArpPacket),
    Other(u16),
}

enum TransportLayer {
    Tcp(TcpHeader, Vec<u8>),
    Udp(UdpHeader, Vec<u8>),
    Icmp(IcmpMessage),
}

fn parse_frame(buffer: &[u8]) -> Result<(EthernetFrame, NetworkPacket), ParseError> {
    // Use struct for fixed ethernet header
    if buffer.len() < 14 {
        return Err(ParseError::BufferTooSmall);
    }
    
    let eth: &EthernetFrame = unsafe {
        &*(buffer.as_ptr() as *const EthernetFrame)
    };
    
    let ethertype = u16::from_be(eth.ethertype);
    
    // Use enum for protocol multiplexing
    let packet = match ethertype {
        0x0800 => {
            let ipv4_hdr = parse_ipv4(&buffer[14..])?;
            let transport = parse_transport_ipv4(&ipv4_hdr, &buffer[14 + 20..])?;
            NetworkPacket::Ipv4 {
                header: ipv4_hdr,
                transport,
            }
        }
        0x86DD => {
            let ipv6_hdr = parse_ipv6(&buffer[14..])?;
            let transport = parse_transport_ipv6(&ipv6_hdr, &buffer[14 + 40..])?;
            NetworkPacket::Ipv6 {
                header: ipv6_hdr,
                transport,
            }
        }
        0x0806 => NetworkPacket::Arp(parse_arp(&buffer[14..])?),
        other => NetworkPacket::Other(other),
    };
    
    Ok((*eth, packet))
}

// Process with type-safe matching
fn handle_packet(frame: EthernetFrame, packet: NetworkPacket) -> Result<(), Error> {
    match packet {
        NetworkPacket::Ipv4 { header, transport } => {
            log::info!("IPv4: {} -> {}", header.src_ip, header.dst_ip);
            match transport {
                TransportLayer::Tcp(tcp_hdr, payload) => {
                    apply_security_policy(header.src_ip, tcp_hdr.src_port)?;
                }
                TransportLayer::Udp(udp_hdr, payload) => {
                    handle_udp_flow(header.src_ip, udp_hdr.src_port, payload)?;
                }
                _ => {}
            }
        }
        NetworkPacket::Ipv6 { header, transport } => {
            // IPv6 handling
        }
        _ => {}
    }
    Ok(())
}
```

---

## Key Lessons from These Examples

1. **Struct + `#[repr(C)]`**: Protocol headers, hardware registers. Layout must match wire/hardware format.
2. **Enum**: Protocol multiplexing, state machines, safe error handling. No space overhead for discriminant in many cases.
3. **Union**: Only when hardware or protocol specs mandate byte reinterpretation. Always wrap in safe abstractions.
4. **Composition**: Real systems layer enums over structs, enums handle variant dispatch, unsafe unions are isolated.