# Complete Data Structures: Memory, Internals & Implementation
## A First-Principles Guide in C, Go, and Rust

> "Bad programmers worry about the code. Good programmers worry about data structures and their relationships."
> — Linus Torvalds

This guide is not a reference card. It builds the mental model from the ground up: from how individual bits sit in silicon, through the cache hierarchy, the stack and heap, all the way to complex data structures. Every "why" is answered.

---

## TABLE OF CONTENTS

- [Part I  — Memory Architecture](#part-i--memory-architecture)
- [Part II — Primitive Types in Memory](#part-ii--primitive-types-in-memory)
- [Part III — Arrays](#part-iii--arrays)
- [Part IV — Strings](#part-iv--strings)
- [Part V  — Linked Lists](#part-v--linked-lists)
- [Part VI — Stacks](#part-vi--stacks)
- [Part VII — Queues](#part-vii--queues)
- [Part VIII — Hash Tables](#part-viii--hash-tables)
- [Part IX — Trees](#part-ix--trees)
- [Part X  — Heaps](#part-x--heaps)
- [Part XI — Graphs](#part-xi--graphs)
- [Part XII — Tries](#part-xii--tries)
- [Part XIII — Bloom Filters](#part-xiii--bloom-filters)
- [Part XIV — Disjoint Set / Union-Find](#part-xiv--disjoint-set--union-find)
- [Part XV — Segment Trees](#part-xv--segment-trees)
- [Part XVI — Fenwick Tree / Binary Indexed Tree](#part-xvi--fenwick-tree--binary-indexed-tree)
- [Part XVII — Memory Management Models](#part-xvii--memory-management-models)
- [Part XVIII — Complexity Master Reference](#part-xviii--complexity-master-reference)

---

# PART I — MEMORY ARCHITECTURE

Before any data structure makes sense you must know where data actually lives and how
the CPU fetches it. This section explains the full memory hierarchy.

---

## 1.1 Physical RAM (DRAM)

Dynamic RAM stores each bit as a charge in a capacitor. Capacitors leak, so the hardware
must constantly refresh every cell (every ~64 ms). This is the "dynamic" in DRAM.

The memory chip is organized as a rectangular grid of cells:

```
          Column 0   Column 1   Column 2   Column 3
         +----------+----------+----------+----------+
Row 0    | capacitor| capacitor| capacitor| capacitor|
         +----------+----------+----------+----------+
Row 1    | capacitor| capacitor| capacitor| capacitor|
         +----------+----------+----------+----------+
Row 2    | capacitor| capacitor| capacitor| capacitor|
         +----------+----------+----------+----------+

To read address (Row=1, Col=2):
  1. RAS pulse — activates row 1, copies it into row buffer (sense amplifiers)
  2. CAS pulse — selects column 2 from row buffer
  3. Data is placed on data bus

RAS latency  ~15 ns
CAS latency  ~15 ns
Total round trip: ~30–50 ns  ≈  100–200 CPU cycles at 3 GHz
```

This huge latency penalty is why the cache hierarchy exists.

---

## 1.2 CPU Cache Hierarchy

Modern CPUs place several layers of faster, smaller memory between the core and main RAM.

```
                  +-----------+
   fastest        | Registers |  < 1 ns,  bytes,   inside ALU
   smallest       +-----------+
                       |
                  +-----------+
                  | L1 Cache  |  ~1  ns,  32–64 KB per core, split I$/D$
                  +-----------+
                       |
                  +-----------+
                  | L2 Cache  |  ~4  ns,  256 KB – 1 MB per core
                  +-----------+
                       |
                  +-----------+
                  | L3 Cache  |  ~15 ns,  4 – 64 MB, shared across cores
   slowest        +-----------+
   largest              |
                  +-----------+
                  | Main RAM  |  ~100 ns, GB range
                  +-----------+
                       |
                  +-----------+
                  | NVMe SSD  |  ~100 µs  (100,000 ns)
                  +-----------+
```

### Cache Lines — The Real Unit of Transfer

The CPU never fetches a single byte from RAM. It fetches a cache line, typically 64 bytes
on x86/x64 (128 bytes on Apple M-series).

```
You request: arr[1]  (at address 0x1004)

Cache fetches from 0x1000 to 0x103F (64 bytes = 16 int32s):
  [0x1000] arr[0]  <-- automatically loaded
  [0x1004] arr[1]  <-- the one you asked for
  [0x1008] arr[2]  <-- free ride
  [0x100C] arr[3]  <-- free ride
  ...
  [0x103C] arr[15] <-- free ride
```

This is **spatial locality**: accessing elements near each other is nearly free once
the first one is fetched.

**Temporal locality** means recently accessed data is likely to be accessed again soon,
so the cache keeps it around.

This distinction governs everything about choosing data structures:
- Arrays are cache-friendly — elements are contiguous, one miss loads many elements.
- Linked lists are cache-hostile — each pointer leads to a random heap address, causing
  a cache miss per node (pointer chasing).

### Cache Miss Cascade Example

```
Linked list traversal (5 nodes, addresses non-contiguous):

Step 1: Load node at 0x1000 → cache miss  → 100 ns wait → read next ptr = 0x5840
Step 2: Load node at 0x5840 → cache miss  → 100 ns wait → read next ptr = 0x0C10
Step 3: Load node at 0x0C10 → cache miss  → 100 ns wait → read next ptr = 0x3A20
...

Array traversal (5 elements at 0x1000–0x1014):

Step 1: Load arr[0] at 0x1000 → cache miss → 100 ns wait → cache line covers [0]–[15]
Step 2: Access arr[1] at 0x1004 → cache HIT  → 1 ns
Step 3: Access arr[2] at 0x1008 → cache HIT  → 1 ns
...

For 1000 elements:
Array:        ~16 misses (64 byte line / 4 byte int = 16 per line)
Linked list:  ~1000 misses
```

---

## 1.3 Stack vs Heap

Every process has a virtual address space (typically 48-bit on x86-64 = 256 TB addressable).
The OS layouts regions inside it:

```
Virtual Address Space (64-bit Linux process, simplified):

0xFFFFFFFFFFFFFFFF  +--------------------------+
                    |   Kernel space           |  (only OS can access)
0xFFFF800000000000  +--------------------------+
                    |                          |
                    |   Stack                  |  grows downward ↓
                    |   (default 8 MB limit)   |
                    |          |               |
                    |          v               |
                    |                          |
                    |   (unused/guard page)    |
                    |                          |
                    |          ^               |
                    |          |               |
                    |   Heap                   |  grows upward ↑
                    |   (managed by malloc)    |
                    |                          |
                    |   mmap region            |  (shared libs, file maps)
                    |                          |
                    |   BSS segment            |  uninitialised globals
                    |   Data segment           |  initialised globals/statics
                    |   Text segment           |  executable code (read-only)
0x0000000000400000  +--------------------------+
                    |   (null / unmapped)      |
0x0000000000000000  +--------------------------+
```

### The Stack in Detail

The stack is controlled by the stack pointer register (RSP on x86-64). Calling a function
pushes a stack frame; returning pops it. Allocation is just a register subtract — O(1) and
extremely fast.

```
Stack growth for: main() → foo(a=3) → bar(b=7)

High address 0x7FFF_0000:
  +-----------------------------+  ← initial RSP (before main)
  |  main() frame               |
  |  ├─ saved RBP               |  8 bytes
  |  ├─ return address (to OS)  |  8 bytes
  |  ├─ local int x = 10        |  4 bytes (+ 4 padding)
  +-----------------------------+
  |  foo() frame                |
  |  ├─ saved RBP               |  8 bytes
  |  ├─ return address (→ main) |  8 bytes
  |  ├─ arg a = 3               |  4 bytes (+ 4 padding)
  |  ├─ local int y = 99        |  4 bytes (+ 4 padding)
  +-----------------------------+
  |  bar() frame                |
  |  ├─ saved RBP               |  8 bytes
  |  ├─ return address (→ foo)  |  8 bytes
  |  ├─ arg b = 7               |  4 bytes (+ 4 padding)
  +-----------------------------+  ← current RSP
Low address
```

### The Heap in Detail

The heap is a large pool of memory the allocator manages. When you call `malloc(n)` in C,
`new` in C++, or the runtime allocates for Go/Rust, the allocator:
1. Looks for a free block of at least n bytes in its free list.
2. If none exists, requests more pages from the OS via `brk()` or `mmap()`.
3. Returns a pointer to the allocated block.

```
Heap state after several malloc/free calls:

+--------+--------+----------+--------+---------+----------+
| used   | FREE   | used     | FREE   | used    | FREE...  |
| 32 B   | 16 B   | 64 B     | 48 B   | 128 B   |          |
+--------+--------+----------+--------+---------+----------+

External fragmentation: many small free blocks that can't satisfy a large request
Internal fragmentation: allocated block is larger than requested (wasted padding)
```

---

## 1.4 Memory Alignment and Padding

Every type has an **alignment requirement**: it must sit at an address that is a multiple
of its size (or its strictest member's size for structs).

```
Type       Size   Alignment   Must sit at address multiple of
-------    ----   ---------   --------------------------------
char         1       1        any address
int16        2       2        0, 2, 4, 6, 8, ...
int32        4       4        0, 4, 8, 12, 16, ...
int64        8       8        0, 8, 16, 24, ...
float32      4       4        0, 4, 8, ...
float64      8       8        0, 8, 16, ...
pointer      8       8        0, 8, 16, ...   (on 64-bit)
```

The CPU can fetch a correctly-aligned value in one memory operation.
A misaligned access may require two reads and a stitch — or a hardware fault.

### Struct Padding Example

```c
// Naïve ordering — wastes space
struct Bad {
    char   a;        // offset 0,  size 1
    // [3 bytes padding — next field needs align 4]
    int    b;        // offset 4,  size 4
    char   c;        // offset 8,  size 1
    // [3 bytes padding — struct size must be multiple of largest alignment (4)]
};                   // sizeof(Bad) = 12 bytes (4 wasted)

// Optimised ordering — sorted by decreasing size
struct Good {
    int    b;        // offset 0,  size 4
    char   a;        // offset 4,  size 1
    char   c;        // offset 5,  size 1
    // [2 bytes padding — struct must be multiple of 4]
};                   // sizeof(Good) = 8 bytes (2 wasted)
```

Memory view of `Bad` (each cell = 1 byte):

```
Offset:  0    1    2    3    4    5    6    7    8    9   10   11
        [a ] [XX] [XX] [XX] [b0] [b1] [b2] [b3] [c ] [XX] [XX] [XX]
         ^              ^                            ^
         a         padding (3)                      c    padding (3)
```

Memory view of `Good`:

```
Offset:  0    1    2    3    4    5    6    7
        [b0] [b1] [b2] [b3] [a ] [c ] [XX] [XX]
         ^                    ^    ^
         b                    a    c   padding (2)
```

### Rule: sort struct fields largest to smallest to minimise waste.

In Rust you can use `#[repr(C)]` to guarantee C layout, or `#[repr(packed)]` to
eliminate padding entirely (at the cost of potentially slow unaligned accesses).
In Go, the compiler can reorder fields for efficiency, though it typically follows
declaration order. In C, you control layout directly.

---

## 1.5 Endianness

When a multi-byte value like `int32 = 0x12345678` is stored, which byte goes to the
lowest address? Two conventions exist:

```
Value: 0x12345678

Little-endian (x86, x86-64, ARM default):
  Address:  0x00  0x01  0x02  0x03
  Byte:     0x78  0x56  0x34  0x12   ← LSB first
  "little end" (least significant byte) at lowest address

Big-endian (network byte order, MIPS, SPARC, PowerPC):
  Address:  0x00  0x01  0x02  0x03
  Byte:     0x12  0x34  0x56  0x78   ← MSB first
  "big end" (most significant byte) at lowest address
```

Endianness matters when:
- Reading binary files written on a different architecture.
- Sending integers over a network (use `htonl`/`ntohl` in C).
- Memory-mapping structs from external sources.

Endianness has zero effect on performance for in-process computation.

---

## 1.6 Virtual Memory and Pages

The OS presents each process with its own illusion of a flat, private address space.
Under the hood, virtual addresses are translated to physical addresses via page tables.

```
Virtual Address Space (process view):
  0x00000000_00000000 to 0x00007FFF_FFFFFFFF (user space, 128 TB)

Physical Memory (actual DRAM chips):
  Physical frame 0x00000000 to 0x0000_FFFF_FFFF (depends on RAM installed)

Translation (via MMU hardware + OS page tables):

Virtual page 0x7FFF_1000  ──page table──►  Physical frame 0x004A_3000
Virtual page 0x7FFF_2000  ──page table──►  Physical frame 0x007B_1000
Virtual page 0x7FFF_3000  ──not mapped──►  PAGE FAULT → OS allocates frame

Page size: 4 KB (typical), 2 MB or 1 GB (huge pages)
TLB (Translation Lookaside Buffer): caches recent virtual→physical mappings
TLB hit:  < 1 ns
TLB miss: ~10-100 ns  (walk the page table in memory)
```

---

## 1.7 Pointers, Fat Pointers, and References

A **pointer** holds the address of something. On 64-bit systems: 8 bytes.

```
int x = 42;
int *p = &x;   // p holds the address, e.g. 0x7FFF_9A08

Memory:
  addr 0x7FFF_9A00:  [00 00 00 00 7F FF 9A 08]  ← p (8 bytes, holds address)
  addr 0x7FFF_9A08:  [00 00 00 2A]               ← x = 42 (4 bytes)

*p == 42      (dereference: follow the pointer to read x)
*p = 100      (write through the pointer to change x)
```

### Pointer Arithmetic

```c
int arr[5] = {10, 20, 30, 40, 50};
int *p = arr;         // p = 0x1000  (points to arr[0])

p + 1                 // 0x1000 + 1 * sizeof(int) = 0x1004  → arr[1]
p + 2                 // 0x1008  → arr[2]
*(p + 3)              // 40     (same as arr[3])

p++                   // p now = 0x1004
```

Arithmetic scales by the element size. This is why `arr[i]` and `*(arr + i)` are identical.

### Fat Pointers

Some languages embed length information alongside the pointer:

```
C pointer (thin):
  +-------------------+
  | address (8 bytes) |
  +-------------------+

Rust slice &[T] (fat):
  +-------------------+-------------------+
  | address (8 bytes) | length  (8 bytes) |
  +-------------------+-------------------+

Rust trait object &dyn Trait (fat):
  +-------------------+-------------------+
  | data ptr (8 bytes)| vtable ptr(8 bytes)|
  +-------------------+-------------------+

Go slice header:
  +-------------------+-------------------+-------------------+
  | data ptr (8 bytes)| len    (8 bytes)  | cap    (8 bytes)  |
  +-------------------+-------------------+-------------------+
```

Fat pointers make slice operations safe (bounds checking) and enable dynamic dispatch
without a separate length variable.

---

# PART II — PRIMITIVE TYPES IN MEMORY

---

## 2.1 Integer Types and Two's Complement

All modern CPUs use **two's complement** for signed integers. To negate: flip all bits, add 1.

```
8-bit examples:

Unsigned uint8:  range 0–255
  0  = 0000 0000
  1  = 0000 0001
255  = 1111 1111

Signed int8:  range -128 to +127
  0  = 0000 0000
  1  = 0000 0001
 127 = 0111 1111
 -1  = 1111 1111   (flip 0000 0001 → 1111 1110, + 1 = 1111 1111)
-128 = 1000 0000   (min; negating overflows — this is expected behaviour)

Addition -1 + 1 in hardware:
  1111 1111
+ 0000 0001
-----------
  0000 0000  ← result: 0  (carry out of bit 7 discarded)
```

The beauty of two's complement: the ALU uses the same addition circuit for both signed
and unsigned, which is why CPUs don't need separate signed/unsigned ADD instructions.

### Integer sizes across languages

```
Concept     C (typical 64-bit)  Go           Rust
---------   ------------------  ---------    -------
8-bit       char/uint8_t/int8_t  int8/uint8   i8/u8
16-bit      short/int16_t        int16/uint16 i16/u16
32-bit      int/int32_t          int32/uint32 i32/u32
64-bit      long long/int64_t    int64/uint64 i64/u64
platform    long (platform-dep.) int/uint     isize/usize
```

**Warning:** In C, `int` is not guaranteed 32 bits (it is on all common platforms, but
the spec says "at least 16"). Always use `<stdint.h>` (`int32_t`, `uint64_t`) in
portable code.

---

## 2.2 IEEE 754 Floating Point

```
float32 (single precision): 32 bits total
  Bit 31  : sign (s)
  Bits 30–23 : biased exponent (e), 8 bits, bias = 127
  Bits 22–0  : fraction/mantissa (f), 23 bits

  Value = (-1)^s  ×  2^(e - 127)  ×  (1.f)  [normalised]

float64 (double precision): 64 bits total
  Bit 63   : sign
  Bits 62–52 : biased exponent, 11 bits, bias = 1023
  Bits 51–0  : fraction, 52 bits

Special values:
  e=0,    f=0  → ±Zero
  e=0,    f≠0  → Subnormal (very small numbers near zero)
  e=255,  f=0  → ±Infinity
  e=255,  f≠0  → NaN (Not a Number)

Example: float32 value 1.0
  s=0, e=127 (0111 1111), f=0
  Binary: 0 01111111 00000000000000000000000
  Hex:    3F 80 00 00

Example: float32 value -5.75
  -5.75 = -1.4375 × 2^2
  s=1, e=129 (1000 0001), f=.4375 = .0111 → stored as 01110000...
  Binary: 1 10000001 01110000000000000000000
  Hex:    C0 B8 00 00
```

Critical floating-point pitfall:

```
0.1 + 0.2  ≠  0.3   in any IEEE 754 language

0.1 in float64 = 0.1000000000000000055511151231257827021181583404541015625
0.2 in float64 = 0.200000000000000011102230246251565404236316680908203125
sum            = 0.3000000000000000444089209850062616169452667236328125
```

Never use `==` to compare floats. Use `|a - b| < epsilon`.

---

## 2.3 Boolean and Character

```
bool: typically 1 byte in memory, despite needing only 1 bit.
  false = 0x00
  true  = 0x01  (C standard: any non-zero is truthy)

Rust:   bool is 1 byte, guaranteed 0 or 1
Go:     bool is 1 byte
C:      _Bool (or bool via stdbool.h) is 1 byte

char (ASCII, 1 byte):
  'A' = 0x41 = 65
  'a' = 0x61 = 97
  '0' = 0x30 = 48
  '\n'= 0x0A = 10
  '\0'= 0x00 = 0   (null terminator — vital for C strings)

Unicode (UTF-8 encoding):
  U+0041 'A'    → 0x41            (1 byte)
  U+00E9 'é'    → 0xC3 0xA9       (2 bytes)
  U+4E2D '中'   → 0xE4 0xB8 0xAD  (3 bytes)
  U+1F600 '😀' → 0xF0 0x9F 0x98 0x80 (4 bytes)

Rust's char is a Unicode scalar value — 4 bytes (u32 internally).
Go's rune is an alias for int32 — 4 bytes.
C has no built-in Unicode type; must use uint32_t or wchar_t.
```

---

## 2.4 Structs and Memory Layout

```
Struct layout rule: each field is placed at the next offset that satisfies its alignment.
The struct's own alignment is the maximum alignment of its fields.
The struct's size is rounded up to a multiple of its alignment.

Example in C:

struct Example {
    uint8_t  a;    // offset 0,  size 1, align 1
    // padding  1 byte (to align b to 2)
    uint16_t b;    // offset 2,  size 2, align 2
    // padding  4 bytes (to align c to 8)
    uint64_t c;    // offset 8,  size 8, align 8
    uint8_t  d;    // offset 16, size 1, align 1
    // padding  7 bytes (struct size must be multiple of 8)
};
// sizeof(Example) = 24 bytes

Memory map:
offset:  0    1    2    3    4    5    6    7    8-15  16   17-23
        [a ] [XX] [b0] [b1] [XX] [XX] [XX] [XX] [c  ] [d ] [XX...XX]
```

**Rust `#[repr(C)]` vs default:**

```rust
// Default Rust: compiler may reorder fields for optimal packing
struct RustDefault {
    a: u8,    // compiler may place this at offset 4 after u32
    b: u32,
    c: u16,
} // May be 8 bytes (a,c together, then b, no wasted space)

// C-compatible layout: fields in declaration order, C padding rules
#[repr(C)]
struct CCompat {
    a: u8,    // offset 0
    // 3 bytes padding
    b: u32,   // offset 4
    c: u16,   // offset 8
    // 2 bytes padding
} // 12 bytes

// Packed: no padding, potentially slow unaligned accesses
#[repr(packed)]
struct Packed {
    a: u8,   // offset 0
    b: u32,  // offset 1  (misaligned!)
    c: u16,  // offset 5  (misaligned!)
} // 7 bytes
```

---

# PART III — ARRAYS

---

## 3.1 Static Arrays

A static array is a contiguous block of memory holding N elements of the same type.
It is the simplest and most cache-friendly data structure possible.

```
int arr[6] = {10, 20, 30, 40, 50, 60};
Assume base address = 0x1000, int = 4 bytes.

Physical memory layout:
┌────────┬────────┬────────┬────────┬────────┬────────┐
│  10    │  20    │  30    │  40    │  50    │  60    │
│4 bytes │4 bytes │4 bytes │4 bytes │4 bytes │4 bytes │
└────────┴────────┴────────┴────────┴────────┴────────┘
0x1000   0x1004   0x1008   0x100C   0x1010   0x1014

Index formula:  address(i) = base + (i × sizeof(T))
  arr[0] = 0x1000 + (0 × 4) = 0x1000
  arr[3] = 0x1000 + (3 × 4) = 0x100C
  arr[5] = 0x1000 + (5 × 4) = 0x1014
```

### Why O(1) Access

The formula `base + (i × element_size)` is a single multiplication and addition — constant
regardless of array size. No traversal. No searching. Just arithmetic.

### Static Array Operations

```
Operation       Complexity   Why
-----------     ----------   -------------------------------------------
Access arr[i]   O(1)         Address arithmetic: base + i*size
Search          O(n)         Must examine each element — no structure to exploit
                             (If sorted: O(log n) with binary search)
Insert at i     O(n)         Must shift elements i+1..n-1 one position right
Insert at end   O(1)         No shift needed (if space available)
Delete at i     O(n)         Must shift elements i+1..n-1 one position left
Delete at end   O(1)         Just decrement length
```

### Multi-Dimensional Arrays

A 2D array `int M[3][4]` is laid out in row-major order in C (and Go, Rust):

```
M[3][4]:
        col0  col1  col2  col3
row0  [  0     1     2     3  ]
row1  [  4     5     6     7  ]
row2  [  8     9    10    11  ]

Memory (contiguous, row by row):
[0][1][2][3]  [4][5][6][7]  [8][9][10][11]
└── row 0 ──┘ └── row 1 ──┘ └── row 2  ──┘

Address formula: &M[r][c] = base + (r * num_cols + c) * sizeof(T)
M[1][2] = base + (1*4 + 2) * 4 = base + 24
```

Column-major order (Fortran, MATLAB, Julia) stores column-by-column:

```
Column-major layout of same M:
[0][4][8]  [1][5][9]  [2][6][10]  [3][7][11]
└─ col0 ─┘ └─ col1 ─┘ └─ col2 ─┘  └─ col3 ─┘
```

**Cache implication:** In row-major (C/Go/Rust), iterating `M[r][c]` over all columns
in the inner loop is cache-friendly. Iterating over rows in the inner loop causes
cache misses. Always match your loop order to your storage order.

---

## 3.2 Dynamic Arrays

A dynamic array grows as elements are added. It wraps a static array internally and
reallocates when full.

### Slice Header (Go)

```go
// reflect.SliceHeader
type slice struct {
    data unsafe.Pointer  // pointer to backing array  (8 bytes)
    len  int             // current element count     (8 bytes)
    cap  int             // allocated capacity        (8 bytes)
}
// Total: 24 bytes for the header itself (on 64-bit)
// The actual elements live on the heap, pointed to by data.
```

### Vec Layout (Rust)

```rust
pub struct Vec<T> {
    ptr: NonNull<T>,  // pointer to heap allocation  (8 bytes)
    cap: usize,       // allocated capacity          (8 bytes)
    len: usize,       // current element count       (8 bytes)
}
// Total: 24 bytes for the struct
```

### Growth Strategy — Amortized O(1) Push

When `len == cap`, a new backing array is allocated (typically 2× larger), elements are
copied, and the old array is freed.

```
Operations:    push 1  push 2  push 3  push 4  push 5
              -------  ------  ------  ------  ------
len:             1       2       3       4       5
cap:             1       2       4       4       8

copy cost at push 2: copy 1 element   (old cap was 1)
copy cost at push 3: copy 2 elements  (old cap was 2)
copy cost at push 5: copy 4 elements  (old cap was 4)

Total copy work for n pushes ≤ 1 + 2 + 4 + 8 + ... + n/2 + n ≤ 2n = O(n)
Per-push amortized cost: O(n)/n = O(1)  ← amortised constant time
```

The doubling factor (2×) is a balance between wasted capacity (memory) and copy frequency
(time). Factor 1.5× wastes less; Go uses ~1.25× for large slices. Rust uses exactly 2×.

---

## 3.3 Array Implementations

### C — Static and Dynamic (Vec-like)

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// ============================================================
// STATIC ARRAY  (stack allocated, fixed size)
// ============================================================
void static_array_demo(void) {
    int arr[5] = {10, 20, 30, 40, 50};
    int n = 5;

    // Access O(1)
    printf("arr[2] = %d\n", arr[2]);

    // Linear search O(n)
    for (int i = 0; i < n; i++) {
        if (arr[i] == 30) { printf("Found 30 at index %d\n", i); break; }
    }

    // Insert at index 2 (shift right)
    int new_arr[6];
    int insert_val = 99, insert_idx = 2;
    memcpy(new_arr, arr, insert_idx * sizeof(int));
    new_arr[insert_idx] = insert_val;
    memcpy(new_arr + insert_idx + 1, arr + insert_idx, (n - insert_idx) * sizeof(int));
    // new_arr: {10, 20, 99, 30, 40, 50}
}

// ============================================================
// DYNAMIC ARRAY  (heap allocated, grows automatically)
// ============================================================
typedef struct {
    int   *data;
    size_t len;
    size_t cap;
} Vec;

Vec vec_new(void) {
    return (Vec){ .data = NULL, .len = 0, .cap = 0 };
}

// Returns 0 on success, -1 on allocation failure
int vec_push(Vec *v, int val) {
    if (v->len >= v->cap) {
        size_t new_cap = (v->cap == 0) ? 1 : v->cap * 2;
        int *new_data  = realloc(v->data, new_cap * sizeof(int));
        if (!new_data) return -1;
        v->data = new_data;
        v->cap  = new_cap;
    }
    v->data[v->len++] = val;
    return 0;
}

int vec_pop(Vec *v, int *out) {
    if (v->len == 0) return -1;
    *out = v->data[--v->len];
    return 0;
}

// Insert at index idx — O(n)
int vec_insert(Vec *v, size_t idx, int val) {
    if (idx > v->len) return -1;
    if (vec_push(v, 0) != 0) return -1;                    // grow if needed
    memmove(v->data + idx + 1, v->data + idx,
            (v->len - idx - 1) * sizeof(int));              // shift right
    v->data[idx] = val;
    return 0;
}

// Remove at index idx — O(n)
void vec_remove(Vec *v, size_t idx) {
    if (idx >= v->len) return;
    memmove(v->data + idx, v->data + idx + 1,
            (v->len - idx - 1) * sizeof(int));              // shift left
    v->len--;
}

void vec_free(Vec *v) {
    free(v->data);
    v->data = NULL; v->len = 0; v->cap = 0;
}

void vec_print(const Vec *v) {
    printf("[");
    for (size_t i = 0; i < v->len; i++)
        printf("%s%d", i ? ", " : "", v->data[i]);
    printf("] len=%zu cap=%zu\n", v->len, v->cap);
}
```

### Go — Arrays and Slices

```go
package main

import "fmt"

// Arrays have fixed size, slices are dynamic.
// An array is a value type — copying an array copies all elements.
// A slice is a reference type (header pointing to backing array).

func staticArray() {
    // Array: value type, size is part of the type
    var arr [5]int = [5]int{10, 20, 30, 40, 50}
    copy_of_arr := arr          // full copy of all 5 ints
    copy_of_arr[0] = 999        // does NOT affect arr
    fmt.Println(arr[0])         // still 10

    // Access
    fmt.Println(arr[2])         // 30

    // Range loop (index + value)
    for i, v := range arr {
        fmt.Printf("arr[%d] = %d\n", i, v)
    }
}

func dynamicSlice() {
    // make([]T, length, capacity)
    s := make([]int, 0, 4)

    // Push — amortised O(1)
    s = append(s, 10, 20, 30, 40)
    fmt.Println(len(s), cap(s))  // 4 4

    // Append triggers reallocation
    s = append(s, 50)
    fmt.Println(len(s), cap(s))  // 5 8  (Go doubled to 8)

    // Slice of a slice — shares backing array
    // s2 and s reference the same underlying array segment
    s2 := s[1:4]           // {20, 30, 40}, len=3, cap=7
    s2[0] = 999            // modifies s[1] too!
    fmt.Println(s[1])      // 999

    // To avoid aliasing, copy:
    s3 := make([]int, len(s2))
    copy(s3, s2)           // independent copy

    // Insert at index 2 — O(n)
    idx := 2
    s = append(s, 0)                       // grow
    copy(s[idx+1:], s[idx:])               // shift right
    s[idx] = 777

    // Delete at index 2 — O(n)
    s = append(s[:idx], s[idx+1:]...)      // shift left + reslice

    // Delete preserving order (alternative syntax)
    // s = s[:idx+copy(s[idx:], s[idx+1:])]
}

// 2D slice (slice of slices — NOT contiguous in memory)
func matrix2D() {
    rows, cols := 3, 4
    m := make([][]int, rows)
    for i := range m {
        m[i] = make([]int, cols)
    }
    m[1][2] = 42
    fmt.Println(m[1][2])

    // Note: each row is a separate heap allocation — not cache-friendly!
    // For numeric computation, use a 1D slice with manual indexing:
    flat := make([]int, rows*cols)
    flat[1*cols+2] = 42  // M[1][2]
}
```

### Rust — Arrays, Slices, and Vec

```rust
fn static_arrays() {
    // [T; N] — fixed size, lives on stack, N is part of the type
    let arr: [i32; 5] = [10, 20, 30, 40, 50];
    let zeros = [0i32; 100];   // 100 zeros

    println!("{}", arr[2]);    // 30

    // Slices — fat pointer (ptr + len), no ownership
    let slice: &[i32] = &arr[1..4];   // [20, 30, 40]
    println!("{}", slice.len());       // 3

    // Iteration
    for (i, &v) in arr.iter().enumerate() {
        println!("arr[{i}] = {v}");
    }
}

fn dynamic_vec() {
    // Vec<T> — heap-allocated, owns its data
    let mut v: Vec<i32> = Vec::new();
    v.push(10);  // len=1, cap=1
    v.push(20);  // len=2, cap=2
    v.push(30);  // len=3, cap=4  (reallocated, doubled)
    v.push(40);  // len=4, cap=4

    println!("len={}, cap={}", v.len(), v.capacity());

    // Access — panics on out of bounds
    println!("{}", v[2]);          // 30

    // Safe access — returns Option<&T>
    if let Some(val) = v.get(10) {
        println!("{val}");
    }

    // Insert at index 1 — O(n)  (shifts elements right)
    v.insert(1, 999);             // [10, 999, 20, 30, 40]

    // Remove at index 1 — O(n)  (shifts elements left)
    v.remove(1);                  // [10, 20, 30, 40]

    // Swap-remove: swap with last, then pop — O(1), changes order
    v.swap_remove(1);             // fast but unordered

    // Pop last — O(1)
    if let Some(val) = v.pop() {
        println!("popped {val}");
    }

    // Pre-allocate to avoid reallocations
    let mut v2: Vec<i32> = Vec::with_capacity(1000);
    for i in 0..1000 { v2.push(i); }  // no reallocations occur

    // Deref to slice
    let slice: &[i32] = &v2;
    let slice2: &[i32] = v2.as_slice();
}

fn matrix_flat() {
    let rows = 3usize;
    let cols = 4usize;
    let mut m = vec![0i32; rows * cols];

    // Access M[r][c]
    m[1 * cols + 2] = 42;
    println!("{}", m[1 * cols + 2]);   // 42
}
```

---

# PART IV — STRINGS

Strings deserve their own section because they are arrays with special rules.

## 4.1 C Strings

```
C string "hello":
  Null-terminated: each string ends with '\0' (byte 0x00).

  Address: 0x5000
  +-----+-----+-----+-----+-----+-----+
  | 'h' | 'e' | 'l' | 'l' | 'o' | \0  |
  | 0x68| 0x65| 0x6C| 0x6C| 0x6F| 0x00|
  +-----+-----+-----+-----+-----+-----+

  strlen("hello") = 5  (does NOT count the \0)
  memory used     = 6 bytes

  Buffer overflow: writing past the '\0' corrupts adjacent memory.
  String literal "hello" lives in the read-only data segment.
  char s[] = "hello" — copies to stack (mutable).
  char *s  = "hello" — pointer to read-only literal (undefined to modify).
```

## 4.2 Go Strings

```go
// Go string is an immutable sequence of bytes (UTF-8 by convention).
// Internally: just a fat pointer: (data ptr, len).

type StringHeader struct {
    Data uintptr  // pointer to byte array
    Len  int      // byte count (not rune count!)
}

s := "héllo"
// len(s) = 6  (bytes, because 'é' is 2 bytes in UTF-8)
// len([]rune(s)) = 5  (Unicode code points)

// Indexing gives bytes, not characters:
fmt.Println(s[0])           // 104 = 'h' (ASCII)
fmt.Println(string(s[0]))   // "h"

// To iterate over Unicode code points (runes):
for i, r := range s {
    fmt.Printf("index %d: U+%04X '%c'\n", i, r, r)
}

// Strings are immutable. Concatenation allocates a new string.
a := "foo"
b := a + "bar"   // new allocation: "foobar"
// Use strings.Builder for efficient concatenation in a loop.
```

## 4.3 Rust Strings

```rust
// Two string types:
// &str  — immutable slice of UTF-8 bytes (fat pointer: ptr + len, no ownership)
// String — owned, heap-allocated, mutable, UTF-8 guaranteed

fn string_demo() {
    // &str (string slice) — can point to literal (static), stack, or heap
    let s1: &str = "hello";          // static lifetime, points to binary

    // String — owned heap allocation
    let mut s2 = String::from("hello");
    s2.push_str(", world");           // appends in place (may reallocate)
    s2.push('!');                     // appends single char

    // Byte count vs character count
    let s3 = "héllo";
    println!("{}", s3.len());         // 6 bytes
    println!("{}", s3.chars().count()); // 5 chars

    // Indexing by byte is unsafe (may split a multi-byte char):
    // let c = s3[1];  // COMPILE ERROR: cannot index str with integer

    // Safe character access:
    let third_char = s3.chars().nth(2);  // Some('l')

    // String to &str — free (just borrows)
    let slice: &str = &s2[..5];

    // &str to String — allocates
    let owned = slice.to_string();
    let owned2 = String::from(slice);
}
```

---

# PART V — LINKED LISTS

A linked list stores elements in nodes scattered across heap memory, each pointing to the next.

---

## 5.1 Singly Linked List

### Node Layout in Memory

```c
struct Node {
    int         data;  // 4 bytes
    // 4 bytes padding (on 64-bit, next pointer must be 8-byte aligned)
    struct Node *next; // 8 bytes
};
// sizeof(Node) = 16 bytes per node
```

Actual heap layout (nodes NOT contiguous):

```
         Heap (arbitrary addresses)

0x1000:  ┌──────────┬──────── ──┐
         │  data=10 │next=0x28A0│
         └──────────┴───────── ─┘
                         │
                         ▼
0x28A0:  ┌──────────┬───── ─────┐
         │ data=20  │next=0x0C10│
         └──────────┴────── ────┘
                         │
                         ▼
0x0C10:  ┌──────────┬──────────┐
         │  data=30 │ next=NULL│
         └──────────┴──────────┘

head → [10|0x28A0] → [20|0x0C10] → [30|NULL]
```

### Operations and Complexity

```
Operation              Complexity   Why
-------------------    ----------   -------------------------------------------
Prepend (push front)   O(1)         New node's next = old head; head = new node
Append (push back)     O(n)         Must traverse to find last node (no tail ptr)
Append with tail ptr   O(1)         Directly update tail->next
Access index i         O(n)         Must traverse from head — no random access
Search                 O(n)         Must check each node
Delete by value        O(n)         Must find the predecessor node first
Delete head            O(1)         head = head->next, free old head
Delete tail            O(n)         Must traverse to second-to-last node
Reverse                O(n)         Must reverse all next pointers
```

### Why Linked Lists Are Cache-Hostile

```
Array access pattern (arr[0], arr[1], arr[2]...):
  Read arr[0] at 0x1000 → cache miss → loads 0x1000–0x103F into L1 (64 bytes)
  Read arr[1] at 0x1004 → cache HIT  (already in L1)
  Read arr[2] at 0x1008 → cache HIT
  ...
  Cache misses per 64 ints: ~1

Linked list traversal:
  Read node at 0x1000 → cache miss → loads 0x1000–0x103F
  next ptr = 0x28A0 → different cache line
  Read node at 0x28A0 → cache miss → loads 0x28A0–0x28BF
  next ptr = 0x0C10 → different cache line
  Read node at 0x0C10 → cache miss
  ...
  Cache misses per node: ~1

For 1000 nodes, linked list has ~100× more cache misses than array.
```

### C Implementation

```c
#include <stdio.h>
#include <stdlib.h>

typedef struct SNode { int data; struct SNode *next; } SNode;
typedef struct { SNode *head; SNode *tail; size_t len; } SList;

SList slist_new(void) { return (SList){NULL, NULL, 0}; }

void slist_push_front(SList *l, int v) {
    SNode *n = malloc(sizeof *n);
    n->data = v; n->next = l->head;
    if (!l->head) l->tail = n;
    l->head = n; l->len++;
}

void slist_push_back(SList *l, int v) {
    SNode *n = malloc(sizeof *n);
    n->data = v; n->next = NULL;
    if (l->tail) l->tail->next = n; else l->head = n;
    l->tail = n; l->len++;
}

int slist_pop_front(SList *l) {
    SNode *old = l->head;
    int v = old->data;
    l->head = old->next;
    if (!l->head) l->tail = NULL;
    free(old); l->len--;
    return v;
}

SNode *slist_find(SList *l, int v) {
    for (SNode *c = l->head; c; c = c->next)
        if (c->data == v) return c;
    return NULL;
}

void slist_delete(SList *l, int v) {
    SNode **pp = &l->head;
    while (*pp && (*pp)->data != v) pp = &(*pp)->next;
    if (!*pp) return;
    SNode *dead = *pp;
    *pp = dead->next;
    if (!dead->next) l->tail = (l->head ? /* find new tail */ NULL : NULL);
    free(dead); l->len--;
    // Simplified: proper tail-tracking left as exercise
}

void slist_reverse(SList *l) {
    SNode *prev = NULL, *curr = l->head, *next;
    l->tail = l->head;
    while (curr) { next = curr->next; curr->next = prev; prev = curr; curr = next; }
    l->head = prev;
}

void slist_free(SList *l) {
    for (SNode *c = l->head; c; ) { SNode *next = c->next; free(c); c = next; }
    l->head = l->tail = NULL; l->len = 0;
}

void slist_print(SList *l) {
    for (SNode *c = l->head; c; c = c->next)
        printf("%d -> ", c->data);
    printf("NULL (len=%zu)\n", l->len);
}
```

### Go Implementation

```go
package main

import "fmt"

type ListNode struct {
    Data int
    Next *ListNode
}

type LinkedList struct {
    Head *ListNode
    Tail *ListNode
    Len  int
}

func (l *LinkedList) PushFront(v int) {
    n := &ListNode{Data: v, Next: l.Head}
    if l.Head == nil { l.Tail = n }
    l.Head = n
    l.Len++
}

func (l *LinkedList) PushBack(v int) {
    n := &ListNode{Data: v}
    if l.Tail != nil { l.Tail.Next = n } else { l.Head = n }
    l.Tail = n
    l.Len++
}

func (l *LinkedList) PopFront() (int, bool) {
    if l.Head == nil { return 0, false }
    v := l.Head.Data
    l.Head = l.Head.Next
    if l.Head == nil { l.Tail = nil }
    l.Len--
    return v, true
}

func (l *LinkedList) Find(v int) *ListNode {
    for c := l.Head; c != nil; c = c.Next {
        if c.Data == v { return c }
    }
    return nil
}

func (l *LinkedList) Reverse() {
    var prev *ListNode
    curr := l.Head
    l.Tail = l.Head
    for curr != nil {
        next := curr.Next
        curr.Next = prev
        prev = curr
        curr = next
    }
    l.Head = prev
}

func (l *LinkedList) Print() {
    for c := l.Head; c != nil; c = c.Next {
        fmt.Printf("%d -> ", c.Data)
    }
    fmt.Printf("nil (len=%d)\n", l.Len)
}
```

### Rust Implementation

```rust
// Rust's ownership system makes linked lists famously tricky.
// Box<Node<T>> is a heap-allocated Node owned by its parent.
// Option wraps the possibility of None (end of list).

pub struct LinkedList<T> {
    head: Link<T>,
    len:  usize,
}

type Link<T> = Option<Box<Node<T>>>;

struct Node<T> {
    data: T,
    next: Link<T>,
}

impl<T: std::fmt::Display + PartialEq> LinkedList<T> {
    pub fn new() -> Self { LinkedList { head: None, len: 0 } }

    pub fn push_front(&mut self, data: T) {
        let node = Box::new(Node { data, next: self.head.take() });
        self.head = Some(node);
        self.len += 1;
    }

    pub fn pop_front(&mut self) -> Option<T> {
        self.head.take().map(|node| {
            self.head = node.next;
            self.len -= 1;
            node.data
        })
    }

    pub fn peek(&self) -> Option<&T> {
        self.head.as_ref().map(|n| &n.data)
    }

    pub fn len(&self) -> usize { self.len }

    pub fn print(&self) {
        let mut curr = &self.head;
        while let Some(node) = curr {
            print!("{} -> ", node.data);
            curr = &node.next;
        }
        println!("None");
    }
}

// Custom Drop prevents stack overflow on very long lists
// (recursive default drop would blow the stack for 10,000+ nodes)
impl<T> Drop for LinkedList<T> {
    fn drop(&mut self) {
        let mut curr = self.head.take();
        while let Some(mut node) = curr {
            curr = node.next.take();
            // node is dropped here — Box frees heap memory
        }
    }
}
```

---

## 5.2 Doubly Linked List

Each node holds both forward and backward pointers. Enables O(1) removal when you have
the node, and backward traversal.

```
Node layout (C):
struct DNode {
    int         data;  // 4 bytes
    // 4 bytes padding
    struct DNode *prev; // 8 bytes
    struct DNode *next; // 8 bytes
};
// sizeof(DNode) = 24 bytes

Memory picture:
                  ┌────────────────────────────────────────┐
                  │                                        │
NULL ← [prev|10|next] ↔ [prev|20|next] ↔ [prev|30|next] → NULL
        head                                       tail
```

Extra capabilities over singly:
- Traverse backwards.
- Remove a node in O(1) given a pointer to it (vs O(n) for singly, where you need the predecessor).
- Doubly-linked list underlies Go's `container/list` and many LRU cache implementations.

---

## 5.3 Circular Linked List

The tail's `next` points back to the head (and for doubly circular, head's `prev` points to tail).

```
  ┌──────────────────────────────────┐
  ▼                                  │
[10|next] → [20|next] → [30|next]  ──┘
  ▲
 head

Use cases:
  - Round-robin scheduling
  - Music player playlists
  - CPU time-slicing simulators
```

---

## 5.4 Skip List

A skip list adds express lanes above the base linked list, enabling O(log n) operations
while maintaining a sorted order.

```
Level 3:  [−∞] ─────────────────────────────────── [50] ─── [+∞]
Level 2:  [−∞] ──────── [20] ─────────────────── [50] ─── [+∞]
Level 1:  [−∞] ─── [10] ─ [20] ─── [30] ─── [50] ─── [+∞]
Level 0:  [−∞] ─ [5] ─ [10] ─ [20] ─ [25] ─ [30] ─ [40] ─ [50] ─ [+∞]

To search for 30:
  Start at top-left (−∞, Level 3)
  → 50 > 30, drop to Level 2
  → 20 < 30, advance to 50 at Level 2; 50 > 30, drop to Level 1
  → 30 = 30, found!  (3 steps vs 7 for linear scan)
```

Node height is chosen randomly (coin flip per level), giving O(log n) expected height.
Skip lists are used in Redis sorted sets (ZSET), LevelDB memtables, and CockroachDB.

---

# PART VI — STACKS

A stack is a LIFO (Last-In, First-Out) container: the last element pushed is the first popped.

---

## 6.1 Memory Layout

### Array-Based Stack (preferred — cache friendly)

```
Array:  [10, 20, 30, __ , __]
Index:    0   1   2   3   4
                  ▲
                 top (index = 2)

Push 40:
  top++; arr[top] = 40;
  [10, 20, 30, 40, __]    top = 3

Pop:
  val = arr[top]; top--;
  returns 40;     top = 2
```

### The Process Call Stack

Every function call pushes a frame; every return pops it.

```
main() calls calculate(a=5) which calls multiply(x=5, y=3):

HIGH ADDRESS
┌──────────────────────────────────┐
│ multiply() frame                 │  ← RSP (stack pointer)
│   saved RBP                      │  8 bytes
│   return address (→ calculate)   │  8 bytes
│   int x  (arg, via register/ABI) │  [may be in register, not stack]
│   int y  (arg)                   │
│   int result = x*y               │  4 bytes local
├──────────────────────────────────┤
│ calculate() frame                │
│   saved RBP                      │  8 bytes
│   return address (→ main)        │  8 bytes
│   int a = 5                      │  4 bytes
│   int partial                    │  4 bytes
├──────────────────────────────────┤
│ main() frame                     │
│   argc, argv                     │
│   local variables                │
└──────────────────────────────────┘
LOW ADDRESS

Stack overflow: too many nested calls (e.g. infinite recursion) exhausts
the stack space (default 8 MB on Linux) → SIGSEGV.
```

### Why Recursion Can Overflow

Each function call adds a frame. With 1000 recursive calls, 1000 frames stack up.
A deep recursive tree search on a list of 100,000 nodes can easily overflow 8 MB.
Iterative solutions with an explicit heap-allocated stack avoid this.

---

## 6.2 Stack Operations

```
Operation   Complexity   Implementation
---------   ----------   --------------
push(v)     O(1)         arr[++top] = v
pop()       O(1)         return arr[top--]
peek()      O(1)         return arr[top]  (no removal)
isEmpty()   O(1)         return top == -1
isFull()    O(1)         return top == capacity - 1
```

### Use Cases

- **Function call management** — built into every CPU via the stack pointer.
- **Expression evaluation** — convert infix to postfix (Shunting-yard algorithm).
- **Balanced bracket checking** — push open brackets, pop and match on close.
- **Depth-First Search (DFS)** — explicit stack simulates recursion.
- **Undo/Redo** — maintain two stacks: undo stack and redo stack.
- **Browser history** — Back button = pop from history stack.

---

## 6.3 Stack Implementations

### C

```c
#include <stdio.h>
#include <stdlib.h>

typedef struct {
    int   *data;
    int    top;
    size_t cap;
} Stack;

Stack stack_new(size_t cap) {
    return (Stack){ .data = malloc(cap * sizeof(int)), .top = -1, .cap = cap };
}

int stack_push(Stack *s, int v) {
    if (s->top + 1 >= (int)s->cap) {
        // grow
        s->cap *= 2;
        s->data = realloc(s->data, s->cap * sizeof(int));
    }
    s->data[++s->top] = v;
    return 0;
}

int stack_pop(Stack *s) {
    if (s->top < 0) { fprintf(stderr, "stack underflow\n"); exit(1); }
    return s->data[s->top--];
}

int stack_peek(const Stack *s) { return s->data[s->top]; }
int stack_empty(const Stack *s) { return s->top < 0; }
void stack_free(Stack *s) { free(s->data); s->top = -1; }
```

### Go

```go
type Stack[T any] struct {
    data []T
}

func (s *Stack[T]) Push(v T)           { s.data = append(s.data, v) }
func (s *Stack[T]) Pop() (T, bool) {
    if len(s.data) == 0 { var z T; return z, false }
    v := s.data[len(s.data)-1]
    s.data = s.data[:len(s.data)-1]
    return v, true
}
func (s *Stack[T]) Peek() (T, bool) {
    if len(s.data) == 0 { var z T; return z, false }
    return s.data[len(s.data)-1], true
}
func (s *Stack[T]) Len() int { return len(s.data) }
```

### Rust

```rust
pub struct Stack<T> {
    data: Vec<T>,
}

impl<T> Stack<T> {
    pub fn new() -> Self { Stack { data: Vec::new() } }
    pub fn push(&mut self, v: T) { self.data.push(v); }
    pub fn pop(&mut self) -> Option<T> { self.data.pop() }
    pub fn peek(&self) -> Option<&T> { self.data.last() }
    pub fn is_empty(&self) -> bool { self.data.is_empty() }
    pub fn len(&self) -> usize { self.data.len() }
}
```

---

# PART VII — QUEUES

A queue is a FIFO (First-In, First-Out) container: the first element enqueued is the
first dequeued. Think of a line at a grocery store.

---

## 7.1 Simple Queue (Linked-List Based)

```
head → [10|next] → [20|next] → [30|NULL] ← tail

Enqueue 40: tail->next = new_node; tail = new_node
  head → [10] → [20] → [30] → [40] ← tail

Dequeue: returns 10, head = head->next
  head → [20] → [30] → [40] ← tail
```

---

## 7.2 Circular Buffer / Ring Buffer

The best queue for bounded FIFO: a fixed-size array treated as circular.

```
Capacity = 8.  head and tail are indices.

Initial state (empty):
  [__, __, __, __, __, __, __, __]
   0   1   2   3   4   5   6   7
   h=0, t=0, size=0

Enqueue 10, 20, 30:
  [10, 20, 30, __, __, __, __, __]
   ^         ^
  h=0       t=3, size=3

Dequeue → returns 10:
  [__, 20, 30, __, __, __, __, __]
       ^    ^
      h=1  t=3, size=2

Enqueue 40, 50, 60, 70, 80, 90:  (tail wraps around)
  [80, 20, 30, 40, 50, 60, 70, 90]  → wait, let's track properly:
  After enqueue 40,50,60,70: t=7
  [__, 20, 30, 40, 50, 60, 70, __]  h=1, t=7
  Enqueue 80: t = (7+1)%8 = 0
  [80, 20, 30, 40, 50, 60, 70, __]  h=1, t=0
  Enqueue 90: t = (0+1)%8 = 1  → but h=1, so FULL!

Full condition:  (t + 1) % cap == h
Empty condition: h == t
Size:            (t - h + cap) % cap

Modulo index arithmetic:  next_idx = (idx + 1) % capacity
```

Circular buffers are used in:
- Network packet buffers
- Audio/video streaming (ring buffers for audio samples)
- OS I/O buffers (keyboard input)
- Producer-consumer pipelines

---

## 7.3 Deque (Double-Ended Queue)

A deque allows push/pop from both ends.

```
push_front(5):   5 ← [5, 10, 20, 30]
push_back(40):       [5, 10, 20, 30, 40]
pop_front():     returns 5,  [10, 20, 30, 40]
pop_back():      returns 40, [10, 20, 30]
```

Typically implemented with a circular buffer (fixed capacity) or a doubly linked list.
Rust's `VecDeque` uses a ring buffer. Go's `list.List` is a doubly-linked-list deque.

---

## 7.4 Priority Queue

A priority queue returns the highest-priority element, not the FIFO element. Implemented
with a heap (see Part X). The queue appears "sorted" by priority at each pop.

---

## 7.5 Queue Implementations

### C — Circular Buffer

```c
#include <stdio.h>
#include <stdbool.h>

#define QUEUE_CAP 8

typedef struct {
    int  data[QUEUE_CAP];
    int  head, tail, size;
} Queue;

void  queue_init(Queue *q) { q->head = q->tail = q->size = 0; }
bool  queue_full(const Queue *q) { return q->size == QUEUE_CAP; }
bool  queue_empty(const Queue *q) { return q->size == 0; }

bool queue_enqueue(Queue *q, int v) {
    if (queue_full(q)) return false;
    q->data[q->tail] = v;
    q->tail = (q->tail + 1) % QUEUE_CAP;
    q->size++;
    return true;
}

bool queue_dequeue(Queue *q, int *out) {
    if (queue_empty(q)) return false;
    *out = q->data[q->head];
    q->head = (q->head + 1) % QUEUE_CAP;
    q->size--;
    return true;
}
```

### Go

```go
type Queue[T any] struct {
    data       []T
    head, tail int
    size       int
    cap        int
}

func NewQueue[T any](cap int) *Queue[T] {
    return &Queue[T]{data: make([]T, cap), cap: cap}
}

func (q *Queue[T]) Enqueue(v T) bool {
    if q.size == q.cap { return false }
    q.data[q.tail] = v
    q.tail = (q.tail + 1) % q.cap
    q.size++
    return true
}

func (q *Queue[T]) Dequeue() (T, bool) {
    var z T
    if q.size == 0 { return z, false }
    v := q.data[q.head]
    q.head = (q.head + 1) % q.cap
    q.size--
    return v, true
}
```

### Rust — VecDeque

```rust
use std::collections::VecDeque;

fn queue_demo() {
    let mut q: VecDeque<i32> = VecDeque::new();

    // Enqueue (push to back)
    q.push_back(10);
    q.push_back(20);
    q.push_back(30);

    // Dequeue (pop from front)
    println!("{:?}", q.pop_front());  // Some(10)
    println!("{:?}", q.pop_front());  // Some(20)

    // Deque: push/pop front AND back
    q.push_front(5);
    println!("{:?}", q.pop_back());   // Some(30)
}
```

---

# PART VIII — HASH TABLES

A hash table achieves expected O(1) insert, delete, and lookup by mapping keys to array
indices via a hash function.

---

## 8.1 Hash Functions

A hash function takes an arbitrary-size key and returns a fixed-size integer (the hash).

```
djb2 algorithm for strings:

uint32_t djb2(const char *s) {
    uint32_t hash = 5381;
    while (*s)
        hash = ((hash << 5) + hash) + (unsigned char)(*s++);  // hash*33 + c
    return hash;
}

"cat"  → 3527539
"dog"  → 3563093
"act"  → 3524785  (different from "cat" — good: not just sorted)

bucket = hash % table_size
```

Good hash functions:
- Distribute keys uniformly (low collision rate)
- Are fast to compute
- Avalanche: one bit change in input flips ~50% of output bits

Production hash functions:
- **SipHash** (Rust default, Go default) — cryptographically secure, protects against hash-flooding DoS
- **FNV-1a** — fast, non-cryptographic
- **xxHash** — extremely fast, good distribution
- **MurmurHash3** — widely used in non-security contexts

---

## 8.2 Collision Resolution — Separate Chaining

Each bucket holds a linked list of entries that hash to the same index.

```
Table size = 7.  Entries: {"cat":1, "dog":2, "sun":3, "act":4, "nap":5}

hash("cat") % 7 = 2
hash("dog") % 7 = 5
hash("sun") % 7 = 2   ← collision with "cat"
hash("act") % 7 = 0
hash("nap") % 7 = 0   ← collision with "act"

Bucket array:
index │  chain
──────┼──────────────────────────────────────
  0   │  [act:4] → [nap:5] → NULL
  1   │  NULL
  2   │  [cat:1] → [sun:3] → NULL
  3   │  NULL
  4   │  NULL
  5   │  [dog:2] → NULL
  6   │  NULL

Load factor α = entries / buckets = 5/7 ≈ 0.71
Average chain length ≈ α = 0.71
```

Complexity with separate chaining:
- **Best case:** O(1) — lookup, uniform distribution
- **Average case:** O(1 + α) — O(1) for small load factor
- **Worst case:** O(n) — all keys collide into one chain

---

## 8.3 Collision Resolution — Open Addressing

All entries live in the array itself. On collision, probe for the next open slot.

### Linear Probing

```
Table size = 8.  Insert: "cat"(h=3), "dog"(h=1), "sun"(h=3), "fog"(h=4)

Insert "cat" at h=3: slot 3 empty → place here
  [_, _, _, cat, _, _, _, _]

Insert "dog" at h=1: slot 1 empty → place here
  [_, dog, _, cat, _, _, _, _]

Insert "sun" at h=3: slot 3 occupied (cat) → probe slot 4
  Slot 4 empty → place "sun" here
  [_, dog, _, cat, sun, _, _, _]

Insert "fog" at h=4: slot 4 occupied (sun) → probe slot 5
  Slot 5 empty → place "fog" here
  [_, dog, _, cat, sun, fog, _, _]

Lookup "fog": hash=4, slot 4 = sun (no) → probe 5 = fog (yes!)
```

Primary clustering: collisions cluster together, making future collisions more likely.

### Quadratic Probing

Probe at i+1², i+2², i+3² to reduce primary clustering.

### Double Hashing

Use a second hash function for the probe step: `step = hash2(key)`.
No clustering, but harder to implement.

---

## 8.4 Robin Hood Hashing (Rust's HashMap)

Robin Hood hashing is a variant of open addressing where entries are moved to reduce
the maximum probe distance.

```
"Rich" elements (close to home) give their slot to "poor" elements (far from home).

Terminology:
  PSL = Probe Sequence Length = how far from ideal slot the entry actually is

Example:
  Insert "a" (ideal slot 2, PSL=0): [_, _, a, _, _, _, _, _]
  Insert "b" (ideal slot 2, collision, PSL=1): probe slot 3
    Slot 3 has entry "c" (ideal slot 3, PSL=0)
    Our PSL (1) > "c"'s PSL (0) → evict "c", place "b" here
    Continue inserting "c" further on
    [_, _, a, b, c, _, _, _]

Benefits:
  - Variance in PSL is minimised
  - Lookup can stop when current entry's PSL < probe count (key can't exist further)
  - Cache performance is excellent
```

Rust's `std::collections::HashMap` uses hashbrown (SwissTable implementation):
- 64-byte groups matching cache lines
- SIMD operations to check 16 slots at once
- Robin Hood inspired but with metadata bytes

---

## 8.5 Load Factor and Rehashing

```
Load factor α = number_of_entries / table_capacity

Too low (α < 0.25):  few collisions, wasted memory
Too high (α > 0.75): many collisions, slow
Typical rehash threshold: 0.75

Rehashing procedure:
1. Allocate new table, typically 2× size
2. Re-insert every entry (must re-hash, since table_size changed)
3. Free old table

Cost of one rehash: O(n)
Frequency: once every n/2 inserts (load goes 0.375 → 0.75)
Amortized cost per insert: O(n) / (n/2) = O(1)
```

---

## 8.6 Go Map Internals

Go maps use a hand-crafted hash table in the runtime (src/runtime/map.go):

```
type hmap struct {
    count     int        // number of entries
    flags     uint8
    B         uint8      // log2 of number of buckets
    noverflow  uint16
    hash0     uint32     // hash seed (random, per-map, prevents hash flooding)
    buckets   unsafe.Pointer  // pointer to array of 2^B bmap structs
    oldbuckets unsafe.Pointer // during rehashing
    nevacuate uintptr
    extra     *mapextra
}

type bmap struct {
    tophash [8]uint8   // top 8 bits of hash for 8 entries
    // keys   [8]keyType  (not in source; computed by compiler)
    // values [8]valType
    // overflow *bmap
}
```

Each bucket holds 8 key-value pairs. `tophash` stores the top byte of each key's hash,
enabling fast "is this slot occupied and roughly matching" checks without reading the key.

---

## 8.7 Hash Table Implementation

### C — Separate Chaining

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define HTABLE_CAP 16

typedef struct HTEntry { char *key; int val; struct HTEntry *next; } HTEntry;
typedef struct { HTEntry *buckets[HTABLE_CAP]; int count; } HTable;

static uint32_t djb2(const char *s) {
    uint32_t h = 5381;
    while (*s) h = ((h << 5) + h) + (unsigned char)(*s++);
    return h;
}

void ht_set(HTable *t, const char *key, int val) {
    uint32_t idx = djb2(key) % HTABLE_CAP;
    for (HTEntry *e = t->buckets[idx]; e; e = e->next) {
        if (strcmp(e->key, key) == 0) { e->val = val; return; }
    }
    HTEntry *e = malloc(sizeof *e);
    e->key = strdup(key); e->val = val; e->next = t->buckets[idx];
    t->buckets[idx] = e; t->count++;
}

int ht_get(HTable *t, const char *key, int *out) {
    for (HTEntry *e = t->buckets[djb2(key) % HTABLE_CAP]; e; e = e->next) {
        if (strcmp(e->key, key) == 0) { *out = e->val; return 1; }
    }
    return 0;
}

void ht_delete(HTable *t, const char *key) {
    uint32_t idx = djb2(key) % HTABLE_CAP;
    HTEntry **pp = &t->buckets[idx];
    while (*pp) {
        if (strcmp((*pp)->key, key) == 0) {
            HTEntry *dead = *pp; *pp = dead->next;
            free(dead->key); free(dead); t->count--; return;
        }
        pp = &(*pp)->next;
    }
}
```

### Go

```go
// Go's built-in map is a hash table.
func hashTableDemo() {
    // Creation
    m := make(map[string]int)         // or map[string]int{}

    // Insert / update
    m["cat"] = 1
    m["dog"] = 2
    m["sun"] = 3

    // Lookup — comma-ok idiom
    if v, ok := m["cat"]; ok {
        fmt.Println("cat:", v)
    }

    // Delete
    delete(m, "sun")

    // Iteration (random order — map has no order guarantee)
    for k, v := range m {
        fmt.Printf("%s: %d\n", k, v)
    }

    // Check existence without value
    _, exists := m["ghost"]
    fmt.Println(exists)  // false

    // Map of slices (common pattern)
    graph := make(map[string][]string)
    graph["a"] = append(graph["a"], "b", "c")
}
```

### Rust

```rust
use std::collections::HashMap;

fn hash_table_demo() {
    // HashMap<K, V> uses SipHash by default (DoS resistant)
    let mut m: HashMap<&str, i32> = HashMap::new();

    // Insert
    m.insert("cat", 1);
    m.insert("dog", 2);

    // Update (insert_or_modify)
    m.entry("cat").and_modify(|v| *v += 10).or_insert(0);

    // Lookup
    if let Some(&v) = m.get("cat") {
        println!("cat: {v}");
    }

    // Remove
    m.remove("dog");

    // Iteration
    for (k, v) in &m {
        println!("{k}: {v}");
    }

    // Count words (classic pattern)
    let words = vec!["hello", "world", "hello", "rust"];
    let mut counts: HashMap<&str, usize> = HashMap::new();
    for w in words {
        *counts.entry(w).or_insert(0) += 1;
    }
}
```

---

# PART IX — TREES

Trees are hierarchical structures. Every node has at most one parent and zero or more children.

---

## 9.1 Binary Tree

Each node has at most two children: left and right.

```
Node layout in memory:

struct TreeNode {
    int            val;   // 4 bytes
    // 4 bytes padding
    struct TreeNode *left; // 8 bytes
    struct TreeNode *right;// 8 bytes
};
// sizeof(TreeNode) = 24 bytes

Tree structure:
              [50]          ← root
             /    \
          [30]    [70]
          / \     / \
       [20][40] [60][80]

Heap memory (nodes are scattered — each malloc is random address):
  root  at 0x5000 → left=0x2A00, right=0x3B00
  30    at 0x2A00 → left=0x1C00, right=0x4D00
  70    at 0x3B00 → left=0x6E00, right=0x7F00
  ...

No contiguous layout — pointer chasing for every operation.
```

### Tree Traversals and Their Properties

```
Tree:
        A
       / \
      B   C
     / \   \
    D   E   F

In-order   (Left, Root, Right):  D B E A C F
  → For a BST, produces sorted output

Pre-order  (Root, Left, Right):  A B D E C F
  → Useful for copying/serialising a tree

Post-order (Left, Right, Root):  D E B F C A
  → Useful for deleting a tree (children before parent)

Level-order (BFS):               A B C D E F
  → Uses a queue; visits level by level
```

---

## 9.2 Binary Search Tree (BST)

BST invariant: for every node N, all values in N's left subtree < N.val < all values in
N's right subtree.

```
Valid BST:
        50
       /  \
      30   70
     /  \  / \
    20  40 60  80

Insert 45:
  45 < 50 → go left
  45 > 30 → go right
  45 > 40 → go right (40's right is NULL)
  Insert 45 as right child of 40

        50
       /  \
      30   70
     /  \  / \
    20  40 60  80
          \
          45
```

### BST Operations and Complexity

```
Operation   Average   Worst (degenerate/sorted input)
---------   -------   --------------------------------
Search      O(log n)  O(n)
Insert      O(log n)  O(n)
Delete      O(log n)  O(n)
Min/Max     O(log n)  O(n)

Worst case occurs when inserting sorted data:
Insert 1, 2, 3, 4, 5:
  1
   \
    2
     \
      3
       \
        4     ← degenerates into a linked list, height = n
```

### BST Delete — Three Cases

```
Case 1: Delete leaf (no children) — just remove
Case 2: Delete node with one child — replace node with its child
Case 3: Delete node with two children — replace with in-order successor
        (smallest value in right subtree), then delete that successor

Delete 30 (has two children: 20 and 40):
  In-order successor of 30 = 40 (smallest in right subtree, which is 40)
  Replace 30's value with 40, then delete node 40
        50                      50
       /  \         →          /  \
      30   70                 40   70
     /  \  / \               /    / \
    20  40 60  80           20   60  80
```

---

## 9.3 AVL Tree (Adelson-Velsky and Landis, 1962)

AVL trees maintain a **balance factor** at each node: BF = height(left) - height(right).
BF must be -1, 0, or +1 at all times. Rebalancing via rotations keeps height O(log n).

### Rotations

```
Right Rotation (RR):
      z                  y
     / \                / \
    y   T4   →        x    z
   / \                     / \
  x   T3               T3   T4
 / \
T1  T2

Left Rotation (LL):
    z                    y
   / \                  / \
  T1   y       →       z    x
      / \             / \
     T2  x           T1  T2
        / \
       T3  T4

Left-Right Rotation (LR): left rotate at y, then right rotate at z
Right-Left Rotation (RL): right rotate at y, then left rotate at z
```

After every insert/delete, walk back up the path updating heights and rotating as needed.

---

## 9.4 Red-Black Tree

Red-black trees use node coloring to maintain approximate balance.

### Properties

```
1. Every node is RED or BLACK.
2. The root is BLACK.
3. Every leaf (NIL sentinel) is BLACK.
4. If a node is RED, both its children are BLACK. (No two consecutive reds)
5. For each node, all paths to descendant leaves contain the same number of
   BLACK nodes. (Black-height is uniform)

These ensure: height ≤ 2 × log₂(n+1)  → O(log n) guaranteed
```

```
Example red-black tree (R=red, B=black):

          13(B)
         /     \
        8(R)   17(R)
       / \      / \
      1(B) 11(B) 15(B) 25(B)
            /
           9(R)

Black-height from root: 2 (counting 8→1 or 8→11→9 gives 2 black nodes each)
```

### When to Use AVL vs Red-Black

```
Feature            AVL                 Red-Black
--------------     ---                 ---------
Balance            Stricter            Looser (2× log n vs 1.44× log n)
Rotations/insert   At most 2           At most 2
Rotations/delete   O(log n)            At most 3
Lookup speed       Slightly faster     Slightly slower (more unbalanced)
Insert/delete      Slightly slower     Slightly faster (fewer rotations)
Use case           Read-heavy          Write-heavy, general purpose

Go, Java, C++ std::map, Linux kernel — all use red-black trees.
```

---

## 9.5 B-Tree

B-trees are generalised search trees designed for disk I/O. Each node holds many keys
and children, matching the size of a disk page (4 KB, 16 KB, etc.).

```
B-Tree of order 3 (max 3 children per node, max 2 keys per node):

             [20, 50]
           /    |     \
    [10,15]  [30,40]  [60,70,80]

Properties:
  - Every node has at most 2t-1 keys, minimum t-1 keys (except root)
  - Non-leaf nodes with k keys have k+1 children
  - All leaves at same depth
  - Keys within a node are sorted

Why for disk: one disk read fetches an entire node (page).
With t=512 (4KB page, 8-byte keys), depth = log₅₁₂(n)
  For n=1 billion: depth = log₅₁₂(10⁹) ≈ 3 disk reads!
  A BST would require log₂(10⁹) ≈ 30 disk reads.
```

### B+ Tree

B+ trees store data only in leaves (internal nodes just have keys for navigation).
Leaves form a linked list for efficient range scans.

```
B+ Tree:
  Internal:     [20, 50]
               /    |     \
             /      |       \
  Leaves: [10,15] [20,30,40] [50,60,70]
             ↔          ↔          ↔    (doubly linked)

Benefit: range query "30 to 65" = find leaf with 30, scan right until > 65
B+ trees are the structure behind MySQL InnoDB indexes and most DBMS indexes.
```

---

## 9.6 Tree Implementations

### C — BST

```c
#include <stdio.h>
#include <stdlib.h>

typedef struct BST { int val; struct BST *left, *right; } BST;

BST *bst_insert(BST *root, int val) {
    if (!root) {
        BST *n = malloc(sizeof *n);
        n->val = val; n->left = n->right = NULL;
        return n;
    }
    if      (val < root->val) root->left  = bst_insert(root->left,  val);
    else if (val > root->val) root->right = bst_insert(root->right, val);
    return root;
}

int bst_search(BST *root, int val) {
    while (root) {
        if      (val == root->val) return 1;
        else if (val <  root->val) root = root->left;
        else                       root = root->right;
    }
    return 0;
}

// In-order traversal → sorted output
void bst_inorder(BST *root) {
    if (!root) return;
    bst_inorder(root->left);
    printf("%d ", root->val);
    bst_inorder(root->right);
}

BST *bst_min(BST *root) {
    while (root->left) root = root->left;
    return root;
}

BST *bst_delete(BST *root, int val) {
    if (!root) return NULL;
    if      (val < root->val) root->left  = bst_delete(root->left,  val);
    else if (val > root->val) root->right = bst_delete(root->right, val);
    else {
        if (!root->left)  { BST *r = root->right; free(root); return r; }
        if (!root->right) { BST *l = root->left;  free(root); return l; }
        BST *successor = bst_min(root->right);
        root->val   = successor->val;
        root->right = bst_delete(root->right, successor->val);
    }
    return root;
}

void bst_free(BST *root) {
    if (!root) return;
    bst_free(root->left);
    bst_free(root->right);
    free(root);
}
```

### Go — Generic BST

```go
package main

import (
    "cmp"
    "fmt"
)

type BSTNode[T cmp.Ordered] struct {
    Val   T
    Left  *BSTNode[T]
    Right *BSTNode[T]
}

func bstInsert[T cmp.Ordered](root *BSTNode[T], val T) *BSTNode[T] {
    if root == nil { return &BSTNode[T]{Val: val} }
    switch cmp.Compare(val, root.Val) {
    case -1: root.Left  = bstInsert(root.Left,  val)
    case +1: root.Right = bstInsert(root.Right, val)
    }
    return root
}

func bstSearch[T cmp.Ordered](root *BSTNode[T], val T) bool {
    for root != nil {
        switch cmp.Compare(val, root.Val) {
        case 0:  return true
        case -1: root = root.Left
        case +1: root = root.Right
        }
    }
    return false
}

func bstInorder[T cmp.Ordered](root *BSTNode[T], out *[]T) {
    if root == nil { return }
    bstInorder(root.Left,  out)
    *out = append(*out, root.Val)
    bstInorder(root.Right, out)
}

// Use standard library for production: container/heap, golang.org/x/exp/slices
```

### Rust — BST

```rust
pub enum BST<T: Ord> {
    Empty,
    Node {
        val:   T,
        left:  Box<BST<T>>,
        right: Box<BST<T>>,
    },
}

impl<T: Ord + Copy + std::fmt::Display> BST<T> {
    pub fn new() -> Self { BST::Empty }

    pub fn insert(self, v: T) -> Self {
        match self {
            BST::Empty => BST::Node {
                val: v, left: Box::new(BST::Empty), right: Box::new(BST::Empty)
            },
            BST::Node { val, left, right } => {
                if      v < val { BST::Node { val, left: Box::new(left.insert(v)), right } }
                else if v > val { BST::Node { val, left, right: Box::new(right.insert(v)) } }
                else            { BST::Node { val, left, right } }  // duplicate: no-op
            }
        }
    }

    pub fn contains(&self, v: T) -> bool {
        match self {
            BST::Empty => false,
            BST::Node { val, left, right } => {
                if      v == *val { true }
                else if v <  *val { left.contains(v) }
                else              { right.contains(v) }
            }
        }
    }

    pub fn inorder(&self) {
        match self {
            BST::Empty => {}
            BST::Node { val, left, right } => {
                left.inorder();
                print!("{val} ");
                right.inorder();
            }
        }
    }
}
```

---

# PART X — HEAPS

A heap is a complete binary tree satisfying the **heap property**: every node's priority
is at least as high as its children's (max-heap), or at most (min-heap).

---

## 10.1 Binary Heap — Array Representation

The clever insight: a complete binary tree maps perfectly to a dense array.

```
Max-heap as tree:
           100
          /   \
        90     85
       / \    / \
      70  60 65  75
     / \
    30  50

Max-heap as array (0-indexed):
Index:  0    1    2    3    4    5    6    7    8
Value: [100, 90,  85,  70,  60,  65,  75,  30,  50]

Index arithmetic (0-indexed):
  Parent of i:       (i - 1) / 2
  Left child of i:   2*i + 1
  Right child of i:  2*i + 2

Check:
  Node at index 1 (90):
    Parent = (1-1)/2 = 0 → index 0 = 100 ✓
    Left   = 2*1+1   = 3 → index 3 = 70  ✓
    Right  = 2*1+2   = 4 → index 4 = 60  ✓
```

This array layout gives excellent cache performance — the entire heap is one contiguous block.

---

## 10.2 Heap Operations

### Push (Insert) — O(log n)

```
Push 95 into max-heap [100, 90, 85, 70, 60, 65, 75, 30, 50]:

1. Append at end (last position):
   [100, 90, 85, 70, 60, 65, 75, 30, 50, 95]
                                           ^ index 9

2. Sift up (bubble up) — compare with parent until heap property restored:
   Parent of 9 = (9-1)/2 = 4 → index 4 = 60 < 95 → swap
   [100, 90, 85, 70, 95, 65, 75, 30, 50, 60]
                        ^ now at index 4

   Parent of 4 = (4-1)/2 = 1 → index 1 = 90 < 95 → swap
   [100, 95, 85, 70, 90, 65, 75, 30, 50, 60]
              ^ now at index 1

   Parent of 1 = (1-1)/2 = 0 → index 0 = 100 > 95 → stop

Final: [100, 95, 85, 70, 90, 65, 75, 30, 50, 60]
At most log₂(n) swaps → O(log n)
```

### Pop Max — O(log n)

```
Pop from [100, 95, 85, 70, 90, 65, 75, 30, 50, 60]:

1. Save root value (100), move last element to root, shrink:
   [60, 95, 85, 70, 90, 65, 75, 30, 50]

2. Sift down — compare with children, swap with larger child:
   60 at index 0. Children: 95 (idx 1), 85 (idx 2). 95 > 60 → swap with left
   [95, 60, 85, 70, 90, 65, 75, 30, 50]

   60 at index 1. Children: 70 (idx 3), 90 (idx 4). 90 > 60 → swap with right
   [95, 90, 85, 70, 60, 65, 75, 30, 50]

   60 at index 4. Children: 30 (idx 9—out of range), 50 (idx 10—out). Leaf → stop.

Returns 100. At most log₂(n) swaps → O(log n)
```

### Build Heap from Array — O(n)

```
Naïve: insert n elements one by one → O(n log n)
Smart: start from last internal node (index n/2 - 1), sift down each

For array [3, 1, 6, 5, 2, 4], n=6:
  Last internal node = 6/2 - 1 = 2 (value 6)

  Sift down index 2 (val=6): children 3(idx5),None → 6>4, ok (no, wait)
  ...
  After full heapify: [6, 5, 4, 1, 2, 3]

Why O(n)?
  Nodes at height h need at most h sifts. There are n/2^(h+1) nodes at height h.
  Total work = Σ h × n/2^(h+1) = n × Σ h/2^h = n × 2 = O(n)
```

### Operations Summary

```
Operation     Complexity   Note
-----------   ----------   ----
Build heap    O(n)         Floyd's algorithm
Push          O(log n)     Sift up
Pop           O(log n)     Sift down
Peek min/max  O(1)         Root element
Heapsort      O(n log n)   Build + n pops; in-place, no extra memory
```

---

## 10.3 Heap Implementations

### C

```c
#include <stdio.h>
#include <stdlib.h>

typedef struct { int *data; int len, cap; } Heap; // max-heap

Heap heap_new(int cap) {
    return (Heap){ .data = malloc(cap * sizeof(int)), .cap = cap };
}

static void swap(int *a, int *b) { int t = *a; *a = *b; *b = t; }

static void sift_up(Heap *h, int i) {
    while (i > 0) {
        int p = (i - 1) / 2;
        if (h->data[p] < h->data[i]) { swap(&h->data[p], &h->data[i]); i = p; }
        else break;
    }
}

static void sift_down(Heap *h, int i) {
    while (1) {
        int largest = i, l = 2*i+1, r = 2*i+2;
        if (l < h->len && h->data[l] > h->data[largest]) largest = l;
        if (r < h->len && h->data[r] > h->data[largest]) largest = r;
        if (largest == i) break;
        swap(&h->data[i], &h->data[largest]);
        i = largest;
    }
}

void heap_push(Heap *h, int v) {
    if (h->len == h->cap) { h->cap *= 2; h->data = realloc(h->data, h->cap * sizeof(int)); }
    h->data[h->len++] = v;
    sift_up(h, h->len - 1);
}

int heap_pop(Heap *h) {
    int top = h->data[0];
    h->data[0] = h->data[--h->len];
    sift_down(h, 0);
    return top;
}

int heap_peek(Heap *h) { return h->data[0]; }
void heap_free(Heap *h) { free(h->data); }

// Build heap from existing array in O(n)
void heapify(int *arr, int n) {
    for (int i = n/2 - 1; i >= 0; i--) {
        // inline sift_down for arr
        int j = i;
        while (1) {
            int lg = j, l = 2*j+1, r = 2*j+2;
            if (l < n && arr[l] > arr[lg]) lg = l;
            if (r < n && arr[r] > arr[lg]) lg = r;
            if (lg == j) break;
            int t = arr[j]; arr[j] = arr[lg]; arr[lg] = t;
            j = lg;
        }
    }
}
```

### Go

```go
import "container/heap"

// heap.Interface implementation for a min-heap of ints
type MinHeap []int

func (h MinHeap) Len() int           { return len(h) }
func (h MinHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h MinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *MinHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *MinHeap) Pop() any {
    old := *h; n := len(old)
    x := old[n-1]; *h = old[:n-1]
    return x
}

func heapDemo() {
    h := &MinHeap{5, 2, 8, 1, 9}
    heap.Init(h)               // O(n) build

    heap.Push(h, 3)            // O(log n)
    fmt.Println((*h)[0])       // min element: 1
    fmt.Println(heap.Pop(h))   // removes and returns 1 → O(log n)
}
```

### Rust

```rust
use std::collections::BinaryHeap;
use std::cmp::Reverse;

fn heap_demo() {
    // Max-heap by default
    let mut max_heap: BinaryHeap<i32> = BinaryHeap::new();
    max_heap.push(5);
    max_heap.push(2);
    max_heap.push(8);
    println!("{}", max_heap.peek().unwrap());   // 8

    // Min-heap using Reverse wrapper
    let mut min_heap: BinaryHeap<Reverse<i32>> = BinaryHeap::new();
    min_heap.push(Reverse(5));
    min_heap.push(Reverse(2));
    min_heap.push(Reverse(8));
    println!("{}", min_heap.peek().unwrap().0); // 2
    if let Some(Reverse(v)) = min_heap.pop() {
        println!("popped min: {v}");            // 2
    }

    // Build from vec in O(n)
    let v = vec![5, 1, 8, 3, 6];
    let h: BinaryHeap<i32> = BinaryHeap::from(v);  // O(n)
}
```

---

# PART XI — GRAPHS

A graph G = (V, E) is a set of vertices V connected by edges E.

---

## 11.1 Graph Terminology

```
Directed (digraph): edges have direction  A → B ≠ B → A
Undirected: edges are symmetric            A — B = B — A
Weighted: edges have a numeric value      A ─5─ B
Cycle: a path that returns to its start
DAG: Directed Acyclic Graph
Connected: every pair of vertices has a path between them (undirected)
Strongly connected: every vertex reachable from every other (directed)
```

---

## 11.2 Representation

### Adjacency Matrix

```
Vertices: {0, 1, 2, 3}
Edges: 0→1, 0→2, 1→3, 2→3

     0  1  2  3
  0 [0, 1, 1, 0]
  1 [0, 0, 0, 1]
  2 [0, 0, 0, 1]
  3 [0, 0, 0, 0]

Memory: O(V²)   — bad for sparse graphs
Check edge (u,v): O(1)  — just matrix[u][v]
Find all neighbours of u: O(V)  — scan entire row
Best when: dense graph, frequent edge existence checks
```

### Adjacency List

```
0: [1, 2]
1: [3]
2: [3]
3: []

Memory: O(V + E)  — great for sparse graphs
Check edge (u,v): O(degree(u))
Find all neighbours of u: O(degree(u))
Best when: sparse graph, traversals, most real-world graphs
```

---

## 11.3 BFS — Breadth-First Search

```
Graph (undirected):
  0 -- 1 -- 3
  |    |
  2 -- 4

BFS from vertex 0 (using a queue):

Start: queue=[0], visited={0}
Step 1: dequeue 0, enqueue neighbours 1, 2
  order=[0], queue=[1,2], visited={0,1,2}
Step 2: dequeue 1, enqueue unvisited neighbours 3, 4
  order=[0,1], queue=[2,3,4], visited={0,1,2,3,4}
Step 3: dequeue 2 → no unvisited neighbours
  order=[0,1,2]
Step 4: dequeue 3 → no unvisited
  order=[0,1,2,3]
Step 5: dequeue 4 → no unvisited
  order=[0,1,2,3,4]

Properties:
  Visits in order of distance from source
  Finds shortest path (in unweighted graph)
  Uses O(V) extra memory for the queue
  Time: O(V + E)
```

---

## 11.4 DFS — Depth-First Search

```
Graph (undirected), same as above. DFS from 0 (recursive):

Visit 0 → recurse into 1 → recurse into 3 → backtrack
        → recurse into 4 → backtrack
        backtrack to 0
        → recurse into 2 → backtrack
Order: 0, 1, 3, 4, 2

Properties:
  Explores as deep as possible before backtracking
  Detects cycles
  Used in topological sort, SCC (Tarjan's algorithm), maze solving
  Time: O(V + E)
  Space: O(V) for the recursion stack (or explicit stack)
```

---

## 11.5 Topological Sort

Topological sort orders vertices of a DAG so that for every edge u→v, u comes before v.
Used in build systems (Makefile), dependency resolution, course prerequisites.

```
DAG:
  compile_a → link → run
  compile_b → link

Topological order: compile_a, compile_b, link, run
  (or compile_b, compile_a, link, run — both valid)

Kahn's algorithm (BFS-based):
1. Compute in-degree of every vertex
2. Enqueue all vertices with in-degree 0
3. While queue not empty:
   a. Dequeue u, add to result
   b. For each neighbour v of u: decrease in-degree(v) by 1
      If in-degree(v) == 0, enqueue v
4. If result has all V vertices: valid sort; else: cycle exists
```

---

## 11.6 Graph Implementations

### C — Adjacency List

```c
#include <stdio.h>
#include <stdlib.h>
#include <stdbool.h>
#include <string.h>

#define MAXV 100

typedef struct AdjNode { int dest; struct AdjNode *next; } AdjNode;
typedef struct { AdjNode *head[MAXV]; int V; } Graph;

Graph graph_new(int V) { Graph g; g.V = V; memset(g.head, 0, sizeof g.head); return g; }

void graph_add_edge(Graph *g, int u, int v) {
    AdjNode *n = malloc(sizeof *n);
    n->dest = v; n->next = g->head[u]; g->head[u] = n;
    // For undirected: also add v→u
}

void graph_bfs(Graph *g, int src) {
    bool visited[MAXV] = {0};
    int queue[MAXV], head = 0, tail = 0;
    visited[src] = true; queue[tail++] = src;
    while (head < tail) {
        int u = queue[head++];
        printf("%d ", u);
        for (AdjNode *n = g->head[u]; n; n = n->next)
            if (!visited[n->dest]) { visited[n->dest] = true; queue[tail++] = n->dest; }
    }
    printf("\n");
}

void graph_dfs_util(Graph *g, int u, bool *visited) {
    visited[u] = true; printf("%d ", u);
    for (AdjNode *n = g->head[u]; n; n = n->next)
        if (!visited[n->dest]) graph_dfs_util(g, n->dest, visited);
}

void graph_dfs(Graph *g, int src) {
    bool visited[MAXV] = {0};
    graph_dfs_util(g, src, visited);
    printf("\n");
}
```

### Go

```go
package main

import "fmt"

type Graph struct {
    adj map[int][]int
    V   int
}

func NewGraph(V int) *Graph {
    return &Graph{adj: make(map[int][]int), V: V}
}

func (g *Graph) AddEdge(u, v int) {
    g.adj[u] = append(g.adj[u], v)
    g.adj[v] = append(g.adj[v], u)  // undirected
}

func (g *Graph) BFS(src int) {
    visited := make(map[int]bool)
    queue   := []int{src}
    visited[src] = true
    for len(queue) > 0 {
        u := queue[0]; queue = queue[1:]
        fmt.Printf("%d ", u)
        for _, v := range g.adj[u] {
            if !visited[v] { visited[v] = true; queue = append(queue, v) }
        }
    }
    fmt.Println()
}

func (g *Graph) DFS(src int) {
    visited := make(map[int]bool)
    var dfs func(int)
    dfs = func(u int) {
        visited[u] = true
        fmt.Printf("%d ", u)
        for _, v := range g.adj[u] {
            if !visited[v] { dfs(v) }
        }
    }
    dfs(src)
    fmt.Println()
}

func (g *Graph) TopologicalSort() []int {
    indegree := make([]int, g.V)
    for _, neighbours := range g.adj {
        for _, v := range neighbours { indegree[v]++ }
    }
    queue := []int{}
    for i := 0; i < g.V; i++ { if indegree[i] == 0 { queue = append(queue, i) } }
    order := []int{}
    for len(queue) > 0 {
        u := queue[0]; queue = queue[1:]
        order = append(order, u)
        for _, v := range g.adj[u] {
            indegree[v]--
            if indegree[v] == 0 { queue = append(queue, v) }
        }
    }
    return order
}
```

### Rust

```rust
use std::collections::{HashMap, VecDeque};

pub struct Graph {
    adj: HashMap<usize, Vec<usize>>,
    v:   usize,
}

impl Graph {
    pub fn new(v: usize) -> Self {
        Graph { adj: HashMap::new(), v }
    }

    pub fn add_edge(&mut self, u: usize, v: usize) {
        self.adj.entry(u).or_default().push(v);
        self.adj.entry(v).or_default().push(u);
    }

    pub fn bfs(&self, src: usize) -> Vec<usize> {
        let mut visited = vec![false; self.v];
        let mut queue   = VecDeque::new();
        let mut order   = Vec::new();
        visited[src] = true; queue.push_back(src);
        while let Some(u) = queue.pop_front() {
            order.push(u);
            if let Some(neighbors) = self.adj.get(&u) {
                for &v in neighbors {
                    if !visited[v] { visited[v] = true; queue.push_back(v); }
                }
            }
        }
        order
    }

    pub fn dfs(&self, src: usize) -> Vec<usize> {
        let mut visited = vec![false; self.v];
        let mut order   = Vec::new();
        self.dfs_util(src, &mut visited, &mut order);
        order
    }

    fn dfs_util(&self, u: usize, visited: &mut Vec<bool>, order: &mut Vec<usize>) {
        visited[u] = true; order.push(u);
        if let Some(neighbors) = self.adj.get(&u) {
            for &v in neighbors {
                if !visited[v] { self.dfs_util(v, visited, order); }
            }
        }
    }
}
```

---

# PART XII — TRIES

A trie (prefix tree) stores strings by their characters. Every path from root to a marked
node represents a stored word.

---

## 12.1 Structure and Memory

```
Inserted words: "cat", "car", "card", "care", "bat"

              root
             /    \
            c      b
            |      |
            a      a
           / \     |
          t   r    t(*)
         (*)  |
             / \
            d   e
           (*) (*)

(*) = marked as word end

Node layout:
struct TrieNode {
    struct TrieNode *children[26];  // 26 pointers × 8 bytes = 208 bytes
    bool             is_end;        // 1 byte + 7 padding
};
// 216 bytes per node — expensive for large alphabets!

Optimisation: compressed trie / Patricia trie merges single-child chains
  "care" → c-a-r-e  (4 nodes) in standard trie
         → "care"   (1 node with string label) in Patricia trie
```

---

## 12.2 Operations

```
Operation   Complexity      Note
---------   ---------       ----
Insert      O(m)            m = length of string
Search      O(m)            Follow characters; check is_end
Delete      O(m)            Mark is_end = false; optionally prune
Prefix search O(m + output) Find prefix node, then collect all words below
Autocomplete  O(m + output) Same as prefix search
```

Trie vs Hash Map:
- Hash map: O(m) per operation, but no prefix queries
- Trie: O(m) per operation, supports all prefix/autocomplete queries
- Trie uses more memory; hash map is simpler

---

## 12.3 Trie Implementation

### C

```c
#include <stdio.h>
#include <stdlib.h>
#include <stdbool.h>
#include <string.h>

typedef struct TrieNode {
    struct TrieNode *children[26];
    bool is_end;
} TrieNode;

TrieNode *trie_new_node(void) {
    TrieNode *n = calloc(1, sizeof *n);  // calloc zeros all children to NULL
    return n;
}

void trie_insert(TrieNode *root, const char *word) {
    TrieNode *curr = root;
    while (*word) {
        int idx = *word - 'a';
        if (!curr->children[idx]) curr->children[idx] = trie_new_node();
        curr = curr->children[idx];
        word++;
    }
    curr->is_end = true;
}

bool trie_search(TrieNode *root, const char *word) {
    TrieNode *curr = root;
    while (*word) {
        int idx = *word - 'a';
        if (!curr->children[idx]) return false;
        curr = curr->children[idx];
        word++;
    }
    return curr->is_end;
}

bool trie_starts_with(TrieNode *root, const char *prefix) {
    TrieNode *curr = root;
    while (*prefix) {
        int idx = *prefix - 'a';
        if (!curr->children[idx]) return false;
        curr = curr->children[idx];
        prefix++;
    }
    return true;  // prefix exists regardless of is_end
}

static void collect_all(TrieNode *node, char *buf, int depth) {
    if (node->is_end) printf("%s\n", buf);
    for (int i = 0; i < 26; i++) {
        if (node->children[i]) {
            buf[depth] = 'a' + i;
            buf[depth + 1] = '\0';
            collect_all(node->children[i], buf, depth + 1);
        }
    }
}

void trie_autocomplete(TrieNode *root, const char *prefix) {
    TrieNode *curr = root;
    while (*prefix) {
        int idx = *prefix - 'a';
        if (!curr->children[idx]) return;
        curr = curr->children[idx];
        prefix++;
    }
    char buf[256];
    strncpy(buf, prefix - strlen(prefix), 255);
    collect_all(curr, buf, strlen(buf));
}

void trie_free(TrieNode *root) {
    if (!root) return;
    for (int i = 0; i < 26; i++) trie_free(root->children[i]);
    free(root);
}
```

### Go

```go
package main

type TrieNode struct {
    children [26]*TrieNode
    isEnd    bool
}

type Trie struct{ root *TrieNode }

func NewTrie() *Trie { return &Trie{root: &TrieNode{}} }

func (t *Trie) Insert(word string) {
    curr := t.root
    for _, ch := range word {
        idx := ch - 'a'
        if curr.children[idx] == nil { curr.children[idx] = &TrieNode{} }
        curr = curr.children[idx]
    }
    curr.isEnd = true
}

func (t *Trie) Search(word string) bool {
    curr := t.root
    for _, ch := range word {
        idx := ch - 'a'
        if curr.children[idx] == nil { return false }
        curr = curr.children[idx]
    }
    return curr.isEnd
}

func (t *Trie) StartsWith(prefix string) bool {
    curr := t.root
    for _, ch := range prefix {
        idx := ch - 'a'
        if curr.children[idx] == nil { return false }
        curr = curr.children[idx]
    }
    return true
}
```

### Rust

```rust
pub struct TrieNode {
    children: [Option<Box<TrieNode>>; 26],
    is_end:   bool,
}

impl TrieNode {
    fn new() -> Self {
        TrieNode { children: std::array::from_fn(|_| None), is_end: false }
    }
}

pub struct Trie { root: TrieNode }

impl Trie {
    pub fn new() -> Self { Trie { root: TrieNode::new() } }

    pub fn insert(&mut self, word: &str) {
        let mut curr = &mut self.root;
        for b in word.bytes() {
            let idx = (b - b'a') as usize;
            curr = curr.children[idx].get_or_insert_with(|| Box::new(TrieNode::new()));
        }
        curr.is_end = true;
    }

    pub fn search(&self, word: &str) -> bool {
        let mut curr = &self.root;
        for b in word.bytes() {
            let idx = (b - b'a') as usize;
            match &curr.children[idx] {
                Some(node) => curr = node,
                None => return false,
            }
        }
        curr.is_end
    }

    pub fn starts_with(&self, prefix: &str) -> bool {
        let mut curr = &self.root;
        for b in prefix.bytes() {
            let idx = (b - b'a') as usize;
            match &curr.children[idx] {
                Some(node) => curr = node,
                None => return false,
            }
        }
        true
    }
}
```

---

# PART XIII — BLOOM FILTERS

A Bloom filter is a space-efficient probabilistic data structure. It can tell you:
- **Definitely NOT in set** (no false negatives)
- **Possibly in set** (small false positive rate)

Used in: databases (avoid disk lookup for missing keys), network routers, spell checkers,
Google BigTable, Cassandra.

---

## 13.1 Structure

```
Bit array of m bits, k independent hash functions.

Insert "cat":
  h1("cat") = 3
  h2("cat") = 7
  h3("cat") = 12
  Set bits 3, 7, 12 to 1.

bit array (m=16):
  index: 0  1  2  3  4  5  6  7  8  9 10 11 12 13 14 15
  bits:  0  0  0  1  0  0  0  1  0  0  0  0  1  0  0  0

Query "dog":
  h1("dog") = 3  → bit 3 = 1
  h2("dog") = 5  → bit 5 = 0 ← NOT ALL BITS SET → definitely NOT in filter

Query "act" (not inserted):
  h1("act") = 3  → 1
  h2("act") = 7  → 1
  h3("act") = 12 → 1 ← all bits set → FALSE POSITIVE! (not actually in set)

False positive rate: ε ≈ (1 - e^(-kn/m))^k
  where n = number of inserted elements
  Optimal k = (m/n) × ln(2)

For 1% false positive rate with n=1,000,000 elements:
  m ≈ 9.6 × n = 9,600,000 bits ≈ 1.2 MB
  vs. a hash set of strings ≈ several hundred MB
```

Bloom filters never support deletion (setting a bit to 0 could affect other keys).
Counting Bloom filters use counters instead of bits to allow deletions.

### C Implementation

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <math.h>

#define BLOOM_BITS 1024

typedef struct {
    uint8_t bits[BLOOM_BITS / 8];
    int     m;  // number of bits
    int     k;  // number of hash functions
} Bloom;

// Multiple hash functions via seed variation (Kirsch-Mitzenmacher)
static uint32_t murmur_seed(const char *key, uint32_t seed) {
    uint32_t h = seed;
    while (*key) { h ^= (unsigned char)(*key++); h *= 0x5bd1e995; h ^= h >> 15; }
    return h;
}

Bloom bloom_new(int m, int k) { Bloom b; memset(&b, 0, sizeof b); b.m = m; b.k = k; return b; }

void bloom_add(Bloom *b, const char *key) {
    for (int i = 0; i < b->k; i++) {
        uint32_t idx = murmur_seed(key, i * 0xDEADBEEF) % b->m;
        b->bits[idx / 8] |= (1u << (idx % 8));
    }
}

int bloom_check(Bloom *b, const char *key) {
    for (int i = 0; i < b->k; i++) {
        uint32_t idx = murmur_seed(key, i * 0xDEADBEEF) % b->m;
        if (!(b->bits[idx / 8] & (1u << (idx % 8)))) return 0; // definitely absent
    }
    return 1; // possibly present
}
```

---

# PART XIV — DISJOINT SET / UNION-FIND

Tracks which elements belong to the same partition. Efficiently merges groups.

---

## 14.1 Structure and Operations

```
Initially: n elements, each in its own set:
  parent[i] = i   (each element is its own root)
  rank[i]   = 0

Elements: {0, 1, 2, 3, 4}
Sets: {0} {1} {2} {3} {4}

Union(0, 1): merge sets containing 0 and 1
  parent[1] = 0   (or 0→1 depending on rank)
  Sets: {0,1} {2} {3} {4}

Union(2, 3):
  parent[3] = 2
  Sets: {0,1} {2,3} {4}

Union(0, 2):
  Sets: {0,1,2,3} {4}

Find(3):
  parent[3] = 2 → parent[2] = 0 → parent[0] = 0 (root!)
  Returns 0

Find(1):
  parent[1] = 0 → root → returns 0
  Same root → 0 and 3 are in the same set!
```

### Path Compression

After `Find(3)`, update all nodes on the path to point directly to root:

```
Before:  3 → 2 → 0 (root)
After:   3 → 0 (direct)   (and 2 → 0 already)
```

### Union by Rank

Always attach the shorter tree under the taller one. This keeps trees flat.

```
Combined with path compression:
  Time per operation: O(α(n)) — inverse Ackermann function
  α(n) < 5 for all practical values of n (essentially O(1))
```

---

## 14.2 Implementations

### C

```c
typedef struct { int *parent, *rank; int n; } UnionFind;

UnionFind uf_new(int n) {
    UnionFind uf;
    uf.n = n;
    uf.parent = malloc(n * sizeof(int));
    uf.rank   = calloc(n, sizeof(int));
    for (int i = 0; i < n; i++) uf.parent[i] = i;
    return uf;
}

int uf_find(UnionFind *uf, int x) {
    if (uf->parent[x] != x)
        uf->parent[x] = uf_find(uf, uf->parent[x]);  // path compression
    return uf->parent[x];
}

void uf_union(UnionFind *uf, int x, int y) {
    int rx = uf_find(uf, x), ry = uf_find(uf, y);
    if (rx == ry) return;
    if (uf->rank[rx] < uf->rank[ry]) { int t = rx; rx = ry; ry = t; }
    uf->parent[ry] = rx;
    if (uf->rank[rx] == uf->rank[ry]) uf->rank[rx]++;
}

int uf_connected(UnionFind *uf, int x, int y) { return uf_find(uf, x) == uf_find(uf, y); }
void uf_free(UnionFind *uf) { free(uf->parent); free(uf->rank); }
```

### Go

```go
type UnionFind struct { parent, rank []int }

func NewUF(n int) UnionFind {
    uf := UnionFind{parent: make([]int, n), rank: make([]int, n)}
    for i := range uf.parent { uf.parent[i] = i }
    return uf
}

func (uf *UnionFind) Find(x int) int {
    if uf.parent[x] != x { uf.parent[x] = uf.Find(uf.parent[x]) }
    return uf.parent[x]
}

func (uf *UnionFind) Union(x, y int) {
    rx, ry := uf.Find(x), uf.Find(y)
    if rx == ry { return }
    if uf.rank[rx] < uf.rank[ry] { rx, ry = ry, rx }
    uf.parent[ry] = rx
    if uf.rank[rx] == uf.rank[ry] { uf.rank[rx]++ }
}

func (uf *UnionFind) Connected(x, y int) bool { return uf.Find(x) == uf.Find(y) }
```

### Rust

```rust
pub struct UnionFind { parent: Vec<usize>, rank: Vec<usize> }

impl UnionFind {
    pub fn new(n: usize) -> Self {
        UnionFind { parent: (0..n).collect(), rank: vec![0; n] }
    }

    pub fn find(&mut self, x: usize) -> usize {
        if self.parent[x] != x {
            self.parent[x] = self.find(self.parent[x]);  // path compression
        }
        self.parent[x]
    }

    pub fn union(&mut self, x: usize, y: usize) {
        let (rx, ry) = (self.find(x), self.find(y));
        if rx == ry { return; }
        match self.rank[rx].cmp(&self.rank[ry]) {
            std::cmp::Ordering::Less    => self.parent[rx] = ry,
            std::cmp::Ordering::Greater => self.parent[ry] = rx,
            std::cmp::Ordering::Equal   => { self.parent[ry] = rx; self.rank[rx] += 1; }
        }
    }

    pub fn connected(&mut self, x: usize, y: usize) -> bool { self.find(x) == self.find(y) }
}
```

---

# PART XV — SEGMENT TREES

A segment tree answers range queries and performs point/range updates in O(log n).

---

## 15.1 Structure

```
Array: [2, 4, 5, 7, 8, 9]   (0-indexed, n=6)

Segment tree for range SUM:

                   [0,5]=35
                 /           \
          [0,2]=11          [3,5]=24
          /     \           /      \
       [0,1]=6  [2,2]=5  [3,4]=15  [5,5]=9
       /    \             /    \
   [0,0]=2 [1,1]=4   [3,3]=7 [4,4]=8

Stored in array (1-indexed, like heap):
  tree[1]  = 35  (root, range [0,5])
  tree[2]  = 11  (range [0,2])
  tree[3]  = 24  (range [3,5])
  tree[4]  = 6   (range [0,1])
  tree[5]  = 5   (range [2,2])
  tree[6]  = 15  (range [3,4])
  tree[7]  = 9   (range [5,5])
  ...
  tree[8]  = 2
  tree[9]  = 4
  tree[12] = 7
  tree[13] = 8

Array size needed: 4 × n to be safe.

Parent of node i: i/2
Left child: 2*i
Right child: 2*i+1
```

### Range Sum Query [l, r]

```
Query sum [1, 4] on array [2, 4, 5, 7, 8, 9]:

Start at root [0,5]:
  [0,5] does not fully fit [1,4] → recurse both halves
  Left [0,2]: partially overlaps [1,4]
    Left [0,1]: partially overlaps
      Left [0,0]: [0,0] < [1,4] → out of range → return 0
      Right [1,1]: [1,1] inside [1,4] → return 4
    Right [2,2]: [2,2] inside [1,4] → return 5
  Right [3,5]: partially overlaps [1,4]
    Left [3,4]: [3,4] inside [1,4] → return 15
    Right [5,5]: [5,5] > [1,4] → out of range → return 0

Total = 0 + 4 + 5 + 15 + 0 = 24 ✓ (4+5+7+8=24)
```

---

## 15.2 Implementation

### C — Range Sum Segment Tree

```c
#include <stdio.h>
#include <stdlib.h>

#define MAX_N 100005

int tree[4 * MAX_N];

void build(int *arr, int node, int l, int r) {
    if (l == r) { tree[node] = arr[l]; return; }
    int mid = (l + r) / 2;
    build(arr, 2*node,   l,     mid);
    build(arr, 2*node+1, mid+1, r  );
    tree[node] = tree[2*node] + tree[2*node+1];
}

void update(int node, int l, int r, int idx, int val) {
    if (l == r) { tree[node] = val; return; }
    int mid = (l + r) / 2;
    if (idx <= mid) update(2*node,   l,     mid, idx, val);
    else            update(2*node+1, mid+1, r,   idx, val);
    tree[node] = tree[2*node] + tree[2*node+1];
}

int query(int node, int l, int r, int ql, int qr) {
    if (qr < l || r < ql) return 0;                 // out of range
    if (ql <= l && r <= qr) return tree[node];       // fully inside
    int mid = (l + r) / 2;
    return query(2*node,   l,     mid, ql, qr)
         + query(2*node+1, mid+1, r,   ql, qr);
}

int main(void) {
    int arr[] = {2, 4, 5, 7, 8, 9};
    int n = 6;
    build(arr, 1, 0, n-1);
    printf("sum [1,4] = %d\n", query(1, 0, n-1, 1, 4));  // 24
    update(1, 0, n-1, 2, 10);  // arr[2] = 10
    printf("sum [0,2] = %d\n", query(1, 0, n-1, 0, 2));  // 2+4+10 = 16
    return 0;
}
```

### Go

```go
package main

import "fmt"

type SegTree struct {
    tree []int
    n    int
}

func NewSegTree(arr []int) *SegTree {
    n := len(arr)
    t := &SegTree{tree: make([]int, 4*n), n: n}
    t.build(arr, 1, 0, n-1)
    return t
}

func (t *SegTree) build(arr []int, node, l, r int) {
    if l == r { t.tree[node] = arr[l]; return }
    mid := (l + r) / 2
    t.build(arr, 2*node,   l,     mid)
    t.build(arr, 2*node+1, mid+1, r  )
    t.tree[node] = t.tree[2*node] + t.tree[2*node+1]
}

func (t *SegTree) Update(idx, val int) { t.update(1, 0, t.n-1, idx, val) }
func (t *SegTree) update(node, l, r, idx, val int) {
    if l == r { t.tree[node] = val; return }
    mid := (l + r) / 2
    if idx <= mid { t.update(2*node,   l,     mid, idx, val) } else
                  { t.update(2*node+1, mid+1, r,   idx, val) }
    t.tree[node] = t.tree[2*node] + t.tree[2*node+1]
}

func (t *SegTree) Query(ql, qr int) int { return t.query(1, 0, t.n-1, ql, qr) }
func (t *SegTree) query(node, l, r, ql, qr int) int {
    if qr < l || r < ql { return 0 }
    if ql <= l && r <= qr { return t.tree[node] }
    mid := (l + r) / 2
    return t.query(2*node, l, mid, ql, qr) + t.query(2*node+1, mid+1, r, ql, qr)
}
```

---

# PART XVI — FENWICK TREE (BINARY INDEXED TREE)

The Fenwick tree is simpler than a segment tree and excels at prefix sum problems.

---

## 16.1 The Core Trick

Every index i stores the sum of a specific range whose length is the lowest set bit of i.

```
Index in binary and what it stores:

i=1  (001): stores sum of arr[1..1]          length = 1  (bit 0)
i=2  (010): stores sum of arr[1..2]          length = 2  (bit 1)
i=3  (011): stores sum of arr[3..3]          length = 1
i=4  (100): stores sum of arr[1..4]          length = 4  (bit 2)
i=5  (101): stores sum of arr[5..5]          length = 1
i=6  (110): stores sum of arr[5..6]          length = 2
i=7  (111): stores sum of arr[7..7]          length = 1
i=8 (1000): stores sum of arr[1..8]          length = 8

lowbit(i) = i & (-i)   ← the value of the lowest set bit

Array: [1, 3, 5, 7, 9, 11, 13, 15]
BIT:  [_, 1, 4, 5, 16, 9, 20, 13, 64]
       0  1  2  3   4  5   6   7   8
```

### Prefix Sum and Update

```
prefix_sum(i): walk from i toward 0, removing lowest bit each step
  sum(6): i=6 → add bit[6]; i = 6 - lowbit(6) = 6 - 2 = 4
          i=4 → add bit[4]; i = 4 - lowbit(4) = 4 - 4 = 0 → stop
  Result = bit[6] + bit[4] = 20 + 16 = 36 = 11+13+9+7+5+3+1 - but 1-indexed so sum of [1,6]

update(i, delta): walk from i toward n, adding lowest bit each step
  update(3, +1): i=3 → bit[3] += 1; i = 3 + lowbit(3) = 3 + 1 = 4
                 i=4 → bit[4] += 1; i = 4 + 4 = 8
                 i=8 → bit[8] += 1; i = 8 + 8 = 16 > n → stop
  All nodes responsible for index 3 are updated.
```

Both operations run in O(log n) — at most log₂(n) steps.

---

## 16.2 Implementations

### C

```c
#include <stdio.h>
#include <string.h>

#define BIT_MAX 100005

int bit[BIT_MAX];
int n;

int lowbit(int x) { return x & (-x); }

void update(int i, int delta) {
    for (; i <= n; i += lowbit(i)) bit[i] += delta;
}

int prefix_sum(int i) {
    int s = 0;
    for (; i > 0; i -= lowbit(i)) s += bit[i];
    return s;
}

int range_sum(int l, int r) { return prefix_sum(r) - prefix_sum(l - 1); }

void build(int *arr, int size) {
    n = size;
    memset(bit, 0, sizeof bit);
    for (int i = 1; i <= n; i++) update(i, arr[i]);
}
```

### Go

```go
type BIT struct { tree []int; n int }

func NewBIT(n int) *BIT { return &BIT{tree: make([]int, n+1), n: n} }

func (b *BIT) Update(i, delta int) {
    for ; i <= b.n; i += i & (-i) { b.tree[i] += delta }
}

func (b *BIT) Prefix(i int) int {
    s := 0
    for ; i > 0; i -= i & (-i) { s += b.tree[i] }
    return s
}

func (b *BIT) Range(l, r int) int { return b.Prefix(r) - b.Prefix(l-1) }
```

### Rust

```rust
pub struct BIT { tree: Vec<i64>, n: usize }

impl BIT {
    pub fn new(n: usize) -> Self { BIT { tree: vec![0; n+1], n } }

    pub fn update(&mut self, mut i: usize, delta: i64) {
        while i <= self.n { self.tree[i] += delta; i += i & i.wrapping_neg(); }
    }

    pub fn prefix(&self, mut i: usize) -> i64 {
        let mut s = 0i64;
        while i > 0 { s += self.tree[i]; i -= i & i.wrapping_neg(); }
        s
    }

    pub fn range(&self, l: usize, r: usize) -> i64 {
        self.prefix(r) - self.prefix(l - 1)
    }
}
```

---

# PART XVII — MEMORY MANAGEMENT MODELS

How C, Go, and Rust each handle the lifecycle of heap-allocated memory.

---

## 17.1 C — Manual Memory Management

```c
// Allocation:
void *ptr = malloc(size);       // allocate size bytes, uninitialized
void *ptr = calloc(n, size);    // allocate n×size bytes, zeroed
void *ptr = realloc(ptr, new);  // resize existing allocation

// Deallocation:
free(ptr);                      // return memory to allocator

// Common errors:
// 1. Memory leak: forget to free
//    char *s = malloc(100);
//    if (error) return;  // <-- leaked! malloc'd but never freed

// 2. Use-after-free: use pointer after freeing
//    free(ptr);
//    printf("%d\n", *ptr);  // UNDEFINED BEHAVIOUR

// 3. Double-free: free same pointer twice
//    free(ptr);
//    free(ptr);  // UNDEFINED BEHAVIOUR

// 4. Buffer overflow: write past allocation
//    int *arr = malloc(5 * sizeof(int));
//    arr[10] = 999;  // UNDEFINED BEHAVIOUR, heap corruption

// Tools: Valgrind, AddressSanitizer (ASAN) detect these at runtime.
```

The C allocator (typically ptmalloc2 on Linux glibc) manages a heap using:
- Free lists organised by size bins (fast, small, large)
- Coalescing adjacent free chunks
- Memory returned to OS via `brk()`/`munmap()` when large chunks freed

---

## 17.2 Go — Garbage Collection (Tricolor Mark-and-Sweep)

Go's runtime automatically reclaims unreachable heap objects.

```
The GC runs concurrently with the program using tri-color mark-and-sweep:

Colors:
  WHITE = not yet visited (candidate for collection)
  GREY  = reachable, but children not yet scanned
  BLACK = reachable, children scanned (safe to keep)

Phase 1 — Mark (concurrent, program still runs):
  1. All objects start WHITE.
  2. Root objects (stack vars, globals) marked GREY.
  3. Pick a GREY object, mark its children GREY, mark itself BLACK.
  4. Repeat until no GREY objects remain.
  → All reachable objects are now BLACK.

Phase 2 — Sweep (concurrent):
  All WHITE objects are garbage → return to allocator.

Write barrier: if the program assigns a BLACK object's field to point to a WHITE
object while GC is running, the WHITE object is shaded GREY — prevents incorrect
collection of newly-reachable objects.

Escape analysis: the Go compiler decides whether a variable lives on the stack
(cheap, no GC pressure) or must escape to the heap (GC managed).

Example:
  func foo() *int {
      x := 42      // x escapes to heap — pointer returned to caller
      return &x
  }
  func bar() int {
      x := 42      // x stays on stack — not reachable after bar() returns
      return x
  }

GC pauses in modern Go: < 1 ms (often microseconds).
```

---

## 17.3 Rust — Ownership and Borrowing

Rust achieves memory safety without a garbage collector through compile-time rules.

```
The Three Rules of Ownership:
1. Each value has exactly one owner.
2. When the owner goes out of scope, the value is dropped (memory freed).
3. Ownership can be moved or borrowed, but not both simultaneously in conflicting ways.

// MOVE: ownership transfers
let s1 = String::from("hello");
let s2 = s1;         // s1 is MOVED into s2
// println!("{s1}"); // COMPILE ERROR: s1 no longer valid

// CLONE: explicit deep copy
let s1 = String::from("hello");
let s2 = s1.clone();  // both s1 and s2 are valid, s2 is a separate allocation
println!("{s1} {s2}");

// BORROW: temporary reference, no ownership transfer
fn print_len(s: &String) { println!("{}", s.len()); }
let s = String::from("hello");
print_len(&s);         // borrow s for the duration of the call
println!("{s}");       // s still valid — was not moved

// MUTABLE BORROW: at most ONE mutable reference at a time
let mut s = String::from("hello");
let r1 = &mut s;      // OK
// let r2 = &mut s;   // COMPILE ERROR: cannot have two mutable borrows
r1.push_str(" world");
println!("{r1}");

// Lifetimes: the compiler tracks how long references are valid
fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {
    if x.len() > y.len() { x } else { y }
}
// 'a means: the returned reference lives as long as the shorter of x and y
```

### Smart Pointers

```rust
// Box<T>: single ownership, heap allocation
let b = Box::new(5);         // 5 is heap-allocated, b owns it
println!("{}", *b);          // 5; b is dropped at end of scope, frees heap

// Rc<T>: reference counting (shared ownership, single-threaded only)
use std::rc::Rc;
let a = Rc::new(String::from("hello"));
let b = Rc::clone(&a);       // reference count = 2
println!("{}", Rc::strong_count(&a));  // 2

// Arc<T>: atomic reference counting (thread-safe shared ownership)
use std::sync::Arc;
let a = Arc::new(vec![1,2,3]);
let b = Arc::clone(&a);
// a and b can be sent to different threads safely

// RefCell<T>: interior mutability (runtime borrow checking)
use std::cell::RefCell;
let x = RefCell::new(5);
*x.borrow_mut() += 1;        // runtime borrow check; panics if violated
println!("{}", x.borrow());  // 6
```

### Memory Layout Comparison

```
Stack frame at function entry:
  C:    no automatic cleanup; must call free() manually
  Go:   GC tracks heap allocations; stack vars freed automatically
  Rust: compiler inserts Drop calls at end of scope; zero-cost, zero-GC

Heap allocation:
  C:    malloc/free; undefined if you forget or double-free
  Go:   allocates freely; GC handles collection, occasional pauses
  Rust: ownership ensures every allocation has exactly one responsible owner;
        dropped deterministically at scope exit — RAII

Thread safety:
  C:    programmer's responsibility; easy to make mistakes
  Go:   goroutines + channels; race detector available
  Rust: Send/Sync traits enforced at compile time; data races are compile errors
```

---

# PART XVIII — COMPLEXITY MASTER REFERENCE

```
DATA STRUCTURE — OPERATIONS TIME COMPLEXITY
==========================================

──────────────────────────────────────────────────────────────────────────────────
Structure              Access   Search   Insert   Delete   Space
──────────────────────────────────────────────────────────────────────────────────
Static Array           O(1)     O(n)     O(n)     O(n)     O(n)
                                         O(1) end O(1) end
Dynamic Array (Vec)    O(1)     O(n)     O(1)*    O(n)     O(n)
                                         amort    O(1) end
Sorted Array           O(1)     O(log n) O(n)     O(n)     O(n)
                                bin srch

Singly Linked List     O(n)     O(n)     O(1) front O(1) front O(n)
                                         O(n) back  O(n) by val
Doubly Linked List     O(n)     O(n)     O(1) ends  O(1) given ptr O(n)

Stack (array)          O(1)top  O(n)     O(1)push  O(1)pop   O(n)
Queue (ring buffer)    O(1)ends O(n)     O(1)enq   O(1)deq   O(n)
Deque (VecDeque)       O(1)ends O(n)     O(1)ends  O(1)ends  O(n)

Hash Table (avg)       O(1)     O(1)     O(1)      O(1)      O(n)
Hash Table (worst)     O(n)     O(n)     O(n)      O(n)      O(n)
                                         (all keys collide)

BST (balanced)         O(log n) O(log n) O(log n)  O(log n)  O(n)
BST (degenerate)       O(n)     O(n)     O(n)      O(n)      O(n)
AVL Tree               O(log n) O(log n) O(log n)  O(log n)  O(n)
Red-Black Tree         O(log n) O(log n) O(log n)  O(log n)  O(n)
B-Tree (order t)       O(log n) O(log n) O(log n)  O(log n)  O(n)
                                         t splits   t merges

Binary Heap (max)      O(1)peek O(n)     O(log n)  O(log n)  O(n)
                                         push      pop-max

Trie                   O(m)     O(m)     O(m)      O(m)      O(m×ALPHA)
                       m=key len                              ALPHA=alphabet size

Segment Tree           –        –        O(log n)  O(log n)  O(n)
                                         point upd point upd
                                         range query: O(log n)

Fenwick Tree (BIT)     –        –        O(log n)  O(log n)  O(n)
                                         prefix sum: O(log n)

Graph (adj matrix)     O(1)     O(V)     O(1)      O(1)      O(V²)
Graph (adj list)       O(deg)   O(V+E)   O(1)      O(E)      O(V+E)
BFS/DFS                –        O(V+E)   –         –         O(V)

Disjoint Set (UF)      –        O(α(n))  O(α(n))  O(α(n))   O(n)
Bloom Filter           –        O(k)     O(k)      N/A       O(m bits)
Skip List (expected)   O(log n) O(log n) O(log n)  O(log n)  O(n log n)
──────────────────────────────────────────────────────────────────────────────────
* amortized

α(n) = inverse Ackermann ≈ O(1) for all practical n
m = string length (Trie)
k = number of hash functions (Bloom filter)
```

---

## When to Choose What — Decision Framework

```
Need fast random access?
  Yes → Array (O(1) index)
  No  ↓

Need fast insert/delete at arbitrary position?
  Yes, with node identity (pointer to node) → Linked list (O(1) with pointer)
  Yes, at front/back only                   → Deque / VecDeque
  Yes, any position by value                → Balanced BST

Need LIFO order?
  → Stack (array-backed)

Need FIFO order?
  → Queue (ring buffer for fixed size, or linked list)

Need ordering + fast search + insert + delete?
  Read-heavy, strict balance needed: AVL Tree
  Write-heavy, general:              Red-Black Tree
  Disk-resident data, range queries: B+ Tree

Need O(1) average lookup by key?
  → Hash Map / Hash Set

Need key prefix queries / autocomplete?
  → Trie

Need range queries (sum/min/max over [l,r]) with updates?
  Point updates, range queries: Segment Tree or Fenwick Tree
  Range updates, range queries: Segment Tree with lazy propagation

Need to check set membership with minimal memory (OK with false positives)?
  → Bloom Filter

Need to track connected components / merging groups?
  → Disjoint Set (Union-Find)

Sparse graph algorithms (BFS, DFS, Dijkstra)?
  → Adjacency List

Dense graph, many edge-existence checks?
  → Adjacency Matrix

Priority queue (process highest priority first)?
  → Binary Heap (or d-ary heap for specific access patterns)
```

---

## Space-Time Tradeoffs Summary

```
Data Structure     Time Advantage              Space Cost
--------------     ---------------             ----------
Hash Table         O(1) avg lookup             Extra memory for empty buckets, load factor
Trie               Prefix queries              O(m × ALPHABET) per node
Segment Tree       O(log n) range queries      4n nodes
Bloom Filter       Tiny for huge n             Fixed bit array, false positives
Sorted Array       O(log n) search (binary)    None extra, but O(n) insert
Skip List          O(log n) avg all ops        O(n log n) extra pointers

The cache reality:
  Array:       1 cache miss per 16 elements (int32, 64-byte line)
  Linked list: 1 cache miss per element (pointer chasing)
  Hash table:  1-2 cache misses per lookup (hash → bucket → chain)
  BST:         O(log n) cache misses (pointer chasing down tree)
  Heap:        Excellent cache use (array-backed, sequential access)
```

---

## Final Mental Model

```
QUESTION 1: Is data access by position (index)?
  → Array family (Vec, slice, array)

QUESTION 2: Does insertion order matter more than random access?
  → Linked list (if O(1) node operations needed)
  → Deque (if only front/back matters)

QUESTION 3: Is there a key-value relationship needing fast lookup?
  → Hash table (O(1) avg, unordered)
  → BST (O(log n), ordered, range queries)
  → Trie (if keys are strings, prefix queries needed)

QUESTION 4: Does priority drive processing order?
  → Heap

QUESTION 5: Are elements hierarchically related?
  → Tree (BST, AVL, Red-Black, B-tree)
  → Union-Find (flat partition tracking)

QUESTION 6: Are elements connected with relationships?
  → Graph (adjacency list for sparse, matrix for dense)

QUESTION 7: Are range aggregate queries needed on arrays?
  → Segment Tree or Fenwick Tree

QUESTION 8: Is approximate membership with minimal memory acceptable?
  → Bloom Filter

The deepest insight: data structures are interfaces between your algorithm's
logical requirements and the hardware's physical constraints (cache, RAM bandwidth,
latency). Understanding the memory layout of each structure lets you predict
performance before profiling and make sound architectural decisions from first principles.
```

---

# Data Structures: A Complete Deep Dive
## From Memory Bits to Linux Kernel — C, Go, and Rust Implementations

---

## Table of Contents

- [Part I: Memory Architecture — The Foundation of Everything](#part-i-memory-architecture)
  - [1. The Memory Hierarchy](#1-the-memory-hierarchy)
  - [2. How the CPU Reads Data — Cache Lines](#2-how-the-cpu-reads-data)
  - [3. Process Memory Layout](#3-process-memory-layout)
  - [4. Virtual Memory and the TLB](#4-virtual-memory-and-the-tlb)
  - [5. Data Alignment and Struct Padding](#5-data-alignment-and-struct-padding)
  - [6. Endianness](#6-endianness)
  - [7. Pointers — The Bedrock of All Data Structures](#7-pointers)
  - [8. Stack vs Heap Allocation](#8-stack-vs-heap-allocation)

- [Part II: Primitive Types in Memory](#part-ii-primitive-types-in-memory)
  - [9. Integers](#9-integers)
  - [10. Floating Point — IEEE 754](#10-floating-point)
  - [11. Booleans and Bit Fields](#11-booleans-and-bit-fields)

- [Part III: Arrays — The Base of Everything](#part-iii-arrays)
  - [12. Static Arrays in Memory](#12-static-arrays-in-memory)
  - [13. Dynamic Arrays (Vectors/Slices)](#13-dynamic-arrays)
  - [14. Multidimensional Arrays — Row-Major vs Column-Major](#14-multidimensional-arrays)
  - [15. Array of Structures vs Structure of Arrays](#15-aos-vs-soa)
  - [16. C Arrays and Pointer Arithmetic](#16-c-arrays)
  - [17. Go Slices — The Real Internals](#17-go-slices)
  - [18. Rust Vec<T>](#18-rust-vec)

- [Part IV: Strings](#part-iv-strings)
  - [19. C Strings — Null-terminated](#19-c-strings)
  - [20. Go Strings — Immutable UTF-8 Slice Header](#20-go-strings)
  - [21. Rust Strings — String vs &str](#21-rust-strings)

- [Part V: Linked Lists](#part-v-linked-lists)
  - [22. Singly Linked List](#22-singly-linked-list)
  - [23. Doubly Linked List](#23-doubly-linked-list)
  - [24. Circular Linked List](#24-circular-linked-list)
  - [25. XOR Linked List](#25-xor-linked-list)
  - [26. Intrusive vs Non-intrusive Lists](#26-intrusive-vs-non-intrusive)
  - [27. Linked List in C, Go, Rust](#27-linked-list-implementations)
  - [28. Linux Kernel list_head](#28-linux-list_head)

- [Part VI: Stacks](#part-vi-stacks)
  - [29. Array-based Stack](#29-array-based-stack)
  - [30. Linked-list-based Stack](#30-linked-list-stack)
  - [31. The Hardware Call Stack — Stack Frames](#31-the-call-stack)
  - [32. Stack Implementations](#32-stack-implementations)

- [Part VII: Queues and Circular Buffers](#part-vii-queues)
  - [33. Simple Queue](#33-simple-queue)
  - [34. Circular Buffer — Ring Buffer](#34-circular-buffer)
  - [35. Deque — Double-ended Queue](#35-deque)
  - [36. Priority Queue](#36-priority-queue)
  - [37. Queue Implementations](#37-queue-implementations)
  - [38. Linux Kernel kfifo](#38-linux-kfifo)

- [Part VIII: Hash Tables](#part-viii-hash-tables)
  - [39. Hash Functions — What Makes a Good One](#39-hash-functions)
  - [40. Separate Chaining](#40-separate-chaining)
  - [41. Open Addressing](#41-open-addressing)
  - [42. Robin Hood Hashing](#42-robin-hood-hashing)
  - [43. Load Factor and Rehashing](#43-load-factor-and-rehashing)
  - [44. C Hash Table Implementation](#44-c-hash-table)
  - [45. Go map — The Real Internals](#45-go-map-internals)
  - [46. Rust HashMap — SwissTable / hashbrown](#46-rust-hashmap)
  - [47. Linux Kernel Hash Tables](#47-linux-hash-tables)

- [Part IX: Trees](#part-ix-trees)
  - [48. Binary Tree — Anatomy](#48-binary-tree)
  - [49. Binary Search Tree (BST)](#49-binary-search-tree)
  - [50. AVL Tree — Self-balancing](#50-avl-tree)
  - [51. Red-Black Tree — The Industry Standard](#51-red-black-tree)
  - [52. B-Tree and B+ Tree](#52-b-tree)
  - [53. Trie — Prefix Tree](#53-trie)
  - [54. Segment Tree](#54-segment-tree)
  - [55. Fenwick Tree — Binary Indexed Tree](#55-fenwick-tree)
  - [56. Tree Implementations in C, Go, Rust](#56-tree-implementations)
  - [57. Linux Kernel rbtree](#57-linux-rbtree)

- [Part X: Heaps](#part-x-heaps)
  - [58. Binary Heap — Array Layout](#58-binary-heap)
  - [59. Heap Operations — Insert, Delete, Heapify](#59-heap-operations)
  - [60. Fibonacci Heap — Amortized Excellence](#60-fibonacci-heap)
  - [61. Heap Implementations](#61-heap-implementations)

- [Part XI: Graphs](#part-xi-graphs)
  - [62. Adjacency Matrix](#62-adjacency-matrix)
  - [63. Adjacency List](#63-adjacency-list)
  - [64. BFS and DFS — Traversal Mechanics](#64-bfs-and-dfs)
  - [65. Graph Implementations](#65-graph-implementations)

- [Part XII: Advanced Data Structures](#part-xii-advanced)
  - [66. Skip List](#66-skip-list)
  - [67. Bloom Filter](#67-bloom-filter)
  - [68. Union-Find — Disjoint Set Union](#68-union-find)
  - [69. LRU Cache](#69-lru-cache)

- [Part XIII: Linux Kernel Data Structures — In Depth](#part-xiii-linux-kernel)
  - [70. list_head — Intrusive Doubly Linked List](#70-list_head-deep-dive)
  - [71. hlist_head / hlist_node — Hash Lists](#71-hlist)
  - [72. rbtree — Red-Black in the Kernel](#72-rbtree-kernel)
  - [73. XArray — The New Radix Tree](#73-xarray)
  - [74. kfifo — Lock-free Circular Buffer](#74-kfifo-deep-dive)
  - [75. RCU — Read-Copy-Update](#75-rcu)
  - [76. Per-CPU Variables](#76-per-cpu-variables)
  - [77. Wait Queues](#77-wait-queues)
  - [78. The SLUB Memory Allocator](#78-slub-allocator)

- [Part XIV: Memory Allocators](#part-xiv-allocators)
  - [79. How malloc Works Internally](#79-malloc-internals)
  - [80. Arena and Pool Allocators](#80-arena-and-pool-allocators)
  - [81. Bump Allocators](#81-bump-allocators)

- [Part XV: Language Deep Dives](#part-xv-language-deep-dives)
  - [82. C — Direct Memory Ownership Model](#82-c-memory-model)
  - [83. Go — Runtime, GC, and Escape Analysis](#83-go-runtime)
  - [84. Rust — Ownership, Borrowing, and Zero-Cost Abstractions](#84-rust-ownership)

- [Part XVI: Mental Models — Choosing the Right Structure](#part-xvi-mental-models)
  - [85. Decision Framework](#85-decision-framework)
  - [86. Complexity Reference Table](#86-complexity-table)
  - [87. Cache Behavior Comparison](#87-cache-behavior)

---

# Part I: Memory Architecture — The Foundation of Everything

## 1. The Memory Hierarchy

Every data structure is, at its core, an arrangement of bytes in memory. Before you can reason about data structures, you must understand the hardware they run on.

Modern systems have a layered memory hierarchy. Each layer is faster and smaller than the next.

```
+==============================================================================+
|                         MEMORY HIERARCHY                                     |
+==============================================================================+
|                                                                              |
|   CPU CORE                                                                   |
|   +------------------------------------------------------------------+       |
|   |  Registers  |  ~0.3 ns  |  ~32-64 regs (64-bit each)  | ~1 cycle |       |
|   +------------------------------------------------------------------+       |
|                                     |                                        |
|   +------------------------------------------------------------------+       |
|   |  L1 Cache |  ~1-4 ns | 32-64 KB (separate I$ and D$) | ~4 cycles |       |
|   +------------------------------------------------------------------+       |
|                                     |                                        |
|   +------------------------------------------------------------------+       |
|   |  L2 Cache   |  ~4-12 ns |  256 KB - 1 MB            | ~12 cycles |       |
|   +------------------------------------------------------------------+       |
|                                     |                                        |
|   +------------------------------------------------------------------+       |
|   |  L3 Cache |  ~30-50ns |  4 - 64 MB (shared by cores)| ~40 cycles |       |
|   +------------------------------------------------------------------+       |
+==============================================================|===============+
                                                               |
+==============================================================v===============+
|   DRAM (Main Memory)    |  ~60-100 ns  |  GB to TB  |  ~200 cycles           |
+==============================================================================+
                                                               |
+==============================================================v===============+
|   NVMe SSD              |  ~50-100 µs  |  TB        |  ~100,000 cycles       |
+==============================================================================+
                                                               |
+==============================================================v===============+
|   HDD                   |  ~5-10 ms    |  TB        |  ~10,000,000 cycles    |
+==============================================================================+
```

**Why this matters for data structures:**
- An algorithm that "only" does O(n log n) work but constantly misses cache can be 10-100x slower than an O(n²) algorithm that is cache-friendly.
- A cache miss to DRAM costs ~200 CPU cycles. In those 200 cycles, your CPU could have done 200 integer additions.
- This is why cache-friendly data structures (arrays) often beat theoretically superior ones (linked lists) in practice.

---

## 2. How the CPU Reads Data — Cache Lines

The CPU never fetches a single byte from RAM. It always fetches a **cache line** — a block of 64 contiguous bytes (on most x86-64 systems).

```
CACHE LINE = 64 bytes (always)

RAM Layout:
            cache line 0         cache line 1         cache line 2
           <--------64B-------> <--------64B-------> <--------64B------->
Addr:  0x00                0x40                0x80                0xC0
       |                    |                    |                    |
       v                    v                    v                    v
       +====================+====================+====================+
RAM:   | B0 B1 B2 ... B63   | B64 B65 ... B127   | B128 B129 ... B191 |
       +====================+====================+====================+

When CPU accesses address 0x44 (byte 4 of cache line 1):
  -> Entire cache line 1 (bytes 0x40-0x7F) is loaded into L1 cache
  -> Subsequent accesses to 0x40-0x7F are L1 cache hits (~1-4 ns)
  -> NOT just the byte at 0x44 — the whole 64 bytes

Example: int arr[16]; (16 * 4 = 64 bytes = exactly ONE cache line)
  arr[0] access -> loads all 16 ints into cache at once
  arr[1]-arr[15] -> FREE, they're already in L1!
```

**Spatial locality** — accessing nearby memory addresses. Arrays exploit this perfectly. Linked lists destroy it because nodes are scattered across the heap.

**Temporal locality** — re-accessing recently used data. A cache keeps recently used lines. Repeated access to the same data hits the cache.

**False Sharing** — when two CPU cores have different data in the same cache line:

```
Cache Line 64 bytes:
  +--------+--------+--------+--------+--------+--------+--------+--------+
  | var_A  | var_B  | var_C  | padding....................................|
  +--------+--------+--------+--------+--------+--------+--------+--------+
    Core0     Core1

Core 0 writes var_A -> invalidates the WHOLE cache line on Core 1.
Core 1 must re-fetch from L3/RAM even though it only cares about var_B.
-> Performance collapse. Fixed with alignment padding to separate cache lines.
```

---

## 3. Process Memory Layout

When a program runs, the OS gives it a virtual address space. On a 64-bit Linux system:

```
Virtual Address Space (x86-64 Linux, not to scale)

Address         Region          Description
============================================================
0xFFFFFFFFFFFFFFFF
                +------------------+
                |  Kernel Space    |  Kernel code, data, page tables
                |  (128 TB)        |  Not accessible from userspace
                +------------------+
0xFFFF800000000000

    [Non-canonical hole — hardware enforced, 16 EB unusable]

0x00007FFFFFFFFFFF
                +------------------+
                |  Stack           |  Function call frames, local vars
                |  (grows DOWN ↓)  |  Limited (~8 MB default on Linux)
                |        ↓         |  Guarded by kernel
                |                  |
                |  [unmapped gap]  |  Stack overflow -> SIGSEGV
                |                  |
                |        ↑         |
                |  Heap            |  malloc/new/Box allocations
                |  (grows UP ↑)    |  Can grow to available memory
                +------------------+
                |  Memory-Mapped   |  mmap(), .so shared libraries,
                |  Region          |  anonymous mappings, file mappings
                +------------------+
                |  BSS Segment     |  Uninitialized global/static vars
                |  (zeroed by OS)  |  e.g., int x; (global)
                +------------------+
                |  Data Segment    |  Initialized global/static vars
                |  (from binary)   |  e.g., int x = 5; (global)
                +------------------+
                |  Text Segment    |  Program machine code (read-only)
                |  (read-only)     |  Shared between processes
                +------------------+
0x0000000000400000
                |  [reserved]      |
0x0000000000000000
============================================================

Key Insight: Stack and Heap SHARE the virtual address space.
They grow toward each other. If they meet -> OOM / stack overflow.
```

**In C, how local variables live:**
```c
void foo() {
    int a = 10;      // On the STACK (part of foo's stack frame)
    int *b = malloc(4); // 'b' (the pointer) is on STACK, but *b is on HEAP
    // When foo() returns, 'a' is gone (stack frame popped)
    // *b lives until free(b) or process exit -> HEAP persists
    free(b);
}
```

---

## 4. Virtual Memory and the TLB

Each process sees its own virtual address space. The hardware MMU (Memory Management Unit) translates virtual addresses to physical ones via **page tables**.

```
Virtual Address (64-bit, x86-64 uses 48 bits)

  Bits 47-39   Bits 38-30   Bits 29-21   Bits 20-12   Bits 11-0
  +----------+ +----------+ +----------+ +----------+ +--------+
  | PML4 idx | | PDP idx  | | PD idx   | | PT idx   | | Offset |
  |  9 bits  | |  9 bits  | |  9 bits  | |  9 bits  | | 12bits |
  +----------+ +----------+ +----------+ +----------+ +--------+
       |              |            |            |          |
       |     Page Table Walk (each level is a 4KB page of 512 entries)
       v
  PML4 table -> PDP table -> PD table -> PT table -> Physical Page
                                                         |
                                                    + Offset (12 bits = 4KB)
                                                         |
                                                    Physical Address

TLB (Translation Lookaside Buffer):
  Hardware cache of recently used VA->PA mappings
  ~1500 entries (L1 TLB), ~4096 (L2 TLB)
  TLB miss = full page table walk = ~50-100 cycles
  Huge pages (2MB/1GB) reduce TLB pressure for large arrays
```

This is why **cache locality** is doubly important: not only cache lines but also TLB pressure matters for large data structure traversal.

---

## 5. Data Alignment and Struct Padding

The CPU reads data most efficiently when the data's address is a multiple of its size. This is called **natural alignment**.

```
Alignment Rules (x86-64):
  char   (1 byte)  -> any address (divisible by 1)
  short  (2 bytes) -> address divisible by 2
  int    (4 bytes) -> address divisible by 4
  long   (8 bytes) -> address divisible by 8
  float  (4 bytes) -> address divisible by 4
  double (8 bytes) -> address divisible by 8
  pointer(8 bytes) -> address divisible by 8

MISALIGNED ACCESS EXAMPLE:
  If a 4-byte int is at address 0x03:
    bytes: [0x03][0x04][0x05][0x06]
    -> CPU may need TWO memory reads and combine them
    -> On some architectures: SIGBUS (crash!)
    -> On x86: hardware handles it, but slower
```

The compiler inserts **padding** between struct members to ensure alignment:

```
struct Bad {           struct Good {
    char   a;   //1B      int64_t d;  //8B @ offset 0
    // 7B PAD            int64_t b;  //8B @ offset 8
    int64_t b;  //8B      int32_t e;  //4B @ offset 16
    char   c;   //1B      int32_t f;  //4B @ offset 20
    // 3B PAD             char    a;  //1B @ offset 24
    int32_t e;  //4B      char    c;  //1B @ offset 25
    // 3B PAD             // 6B PAD
    int32_t f;  //4B  };  Total: 32B
    // 4B PAD
};  Total: 32B        struct Better {    // Sorted by size
                          int64_t b;  //8B
                          int64_t d;  //8B
                          int32_t e;  //4B
                          int32_t f;  //4B
                          char    a;  //1B
                          char    c;  //1B
                          // 6B PAD
                      };  Total: 32B  <- Same here due to struct alignment
                                         but avoids internal padding waste

Rule: Sort fields LARGEST to SMALLEST for minimum padding.
```

In C/Go/Rust you can inspect alignment:
```c
// C
printf("%zu\n", offsetof(struct Foo, field)); // byte offset of field
printf("%zu\n", sizeof(struct Foo));           // total size with padding

// Rust
println!("{}", std::mem::align_of::());
println!("{}", std::mem::size_of::());
```

In Rust, `#[repr(C)]` uses C-compatible layout. `#[repr(packed)]` removes padding (dangerous: misaligned access). `#[repr(align(N))]` forces minimum alignment.

---

## 6. Endianness

Endianness describes the byte order used to store multi-byte values in memory.

```
Value: 0x01020304 (int32, 4 bytes)

Little-Endian (x86, x86-64, ARM default):
  Address: 0x100  0x101  0x102  0x103
  Value:   [0x04] [0x03] [0x02] [0x01]
           LSB                   MSB
  Least significant byte stored at LOWEST address.

Big-Endian (network byte order, SPARC, some MIPS):
  Address: 0x100  0x101  0x102  0x103
  Value:   [0x01] [0x02] [0x03] [0x04]
           MSB                   LSB
  Most significant byte stored at LOWEST address.

Why it matters:
  1. Binary file formats (must specify endianness)
  2. Network protocols (TCP/IP uses big-endian = "network byte order")
  3. Cross-platform serialization (protobuf, flatbuffers handle this)
  4. Reading memory dumps / debugging
  
htonl() / ntohl() in C: "host to network long" — converts between native and network order.
```

---

## 7. Pointers — The Bedrock of All Data Structures

A pointer is a variable whose value is a memory address. On 64-bit systems, all pointers are 8 bytes regardless of what they point to.

```
int x = 42;
int *p = &x;

Memory:
  Name  Address   Contents
  x     0x7FF0    [0x2A 0x00 0x00 0x00]  // 42 in little-endian
  p     0x7FF8    [0xF0 0x7F 0x00 0x00 0x00 0x00 0x00 0x00]  // address of x

  *p = 42  (dereference: follow the address, read the value)
  &x = 0x7FF0  (address-of: get the address of x)

Pointer arithmetic:
  int arr[5] = {10, 20, 30, 40, 50};
  int *p = arr;       // p points to arr[0]
  p + 1               // p + 1*sizeof(int) = next element, NOT next byte
  *(p + 3) = arr[3]   // dereference 3 positions ahead

  char  *p: p+1 advances 1 byte
  int   *p: p+1 advances 4 bytes
  long  *p: p+1 advances 8 bytes
  T     *p: p+1 advances sizeof(T) bytes  <- type system encodes element size

NULL pointer: address 0x0 (invalid, accessing it -> segfault)
  Linux maps page 0 as non-accessible for exactly this reason.

Pointer width:
  64-bit system: 8 bytes (can address 2^64 = 16 exabytes)
  32-bit system: 4 bytes (can address 2^32 = 4 GB)
```

**Double pointer** (`int **pp`): pointer to a pointer. Used for:
- Modifying a pointer from a function (pass `&p` to `func(int **pp)`)
- Implementing arrays of strings (`char **argv`)
- Building trees, linked lists in C

---

## 8. Stack vs Heap Allocation

```
STACK ALLOCATION                    HEAP ALLOCATION
====================                ====================
void foo() {                        void foo() {
    int x = 10;      // STACK           int *x = malloc(4); // HEAP
    int arr[100];    // STACK           *x = 10;
}                                       // MUST call free(x)
                                    }

Stack Frame for foo():
+------------------+ <- higher addr
| return address   | // where to go when foo() returns
+------------------+
| saved registers  | // caller-saved registers
+------------------+
| local var: x     | // 4 bytes for int x
+------------------+
| local arr[100]   | // 400 bytes for int arr[100]
+------------------+ <- rsp (stack pointer)

STACK:                              HEAP:
- Fixed size per frame              - Dynamically sized
- Allocation = 1 instruction        - Allocation = malloc() call (slow)
  (subtract from stack pointer)       takes ~50-200ns, may call kernel
- Deallocation = automatic          - Deallocation = explicit (or GC)
  (restore stack pointer)           - Fragmentation possible
- No fragmentation                  - Can allocate any size
- Size known at compile time        - Size can be runtime-determined
- Thread-local (each thread has     - Shared across threads
  its own stack)                      (synchronization needed)
- Default ~8MB on Linux             - Limited by available RAM

WHEN TO USE STACK:
  - Small, fixed-size data
  - Short lifetime (function scope)
  - Performance critical

WHEN TO USE HEAP:
  - Large data structures
  - Data that outlives the function
  - Size unknown at compile time
  - Shared between threads
```

---

# Part II: Primitive Types in Memory

## 9. Integers

Integers are the simplest data type. Their representation in memory is direct binary encoding.

```
Unsigned integers: straight binary
  uint8:  0 to 255          (2^8  - 1)
  uint16: 0 to 65535        (2^16 - 1)
  uint32: 0 to 4294967295   (2^32 - 1)
  uint64: 0 to 1.8 * 10^19  (2^64 - 1)

Signed integers: Two's Complement
  Why Two's Complement? Because x + (-x) = 0 naturally, no special hardware.
  
  To negate: flip all bits, add 1
  
  int8 range: -128 to 127
  
  Binary:
    0 = 0000 0000
    1 = 0000 0001
   127 = 0111 1111  <- max positive (MSB = 0 means positive)
  -128 = 1000 0000  <- most negative (MSB = 1 means negative)
  -127 = 1000 0001
   -1  = 1111 1111  <- All ones = -1 (elegant!)
    
  Overflow behavior:
    C/Go: int overflow is UNDEFINED BEHAVIOR in C (technically)
          Go: wraps around (two's complement)
    Rust: debug mode: PANIC on overflow
          release mode: wraps around (by design, explicit)
```

---

## 10. Floating Point — IEEE 754

```
IEEE 754 Single Precision (float32 = 4 bytes = 32 bits):

  Bit 31  | Bits 30-23  | Bits 22-0
  Sign (1) | Exponent (8) | Mantissa (23)
  
  Value = (-1)^sign * 1.mantissa * 2^(exponent - 127)

Example: 3.14159 in float32
  Sign: 0 (positive)
  3.14159 in binary: 11.001001000011111...
  Normalized: 1.1001001000011111... * 2^1
  Exponent: 1 + 127 = 128 = 10000000
  Mantissa: 10010010000111111011011 (23 bits after implicit 1.)

Memory (little-endian):
  [0x56] [0x0E] [0x49] [0x40]
  
CRITICAL: 0.1 + 0.2 != 0.3 in floating point!
  0.1 cannot be represented exactly in binary (like 1/3 in decimal)
  0.1 ≈ 0.1000000000000000055511151...
  0.2 ≈ 0.2000000000000000111022302...
  0.1 + 0.2 ≈ 0.3000000000000000444089210...

Special values:
  +Infinity: exponent=255, mantissa=0, sign=0
  -Infinity: exponent=255, mantissa=0, sign=1
  NaN: exponent=255, mantissa!=0 (Not a Number)
  +0 and -0: both exist! (sign bit differs, but == in comparison)
  
float64 (double): 1 sign + 11 exponent + 52 mantissa = 64 bits
```

---

## 11. Booleans and Bit Fields

```
bool in most languages uses 1 BYTE even though it only needs 1 BIT.
Why? CPU addresses memory at byte granularity. A single-bit bool
would require read-modify-write operations.

  bool b = true;
  Memory: [0x01]  (1 byte, value 1 = true, 0 = false)

Bit fields (C):
  struct Flags {
      unsigned int read    : 1;   // only 1 bit
      unsigned int write   : 1;   // only 1 bit  
      unsigned int execute : 1;   // only 1 bit
      unsigned int        : 5;    // 5 bits padding to fill byte
  };
  // Total: 1 byte (efficiently packed)
  
  Used in: TCP/IP headers, filesystem inode flags, hardware registers
  Linux kernel uses bit fields extensively for compact flag storage.

Bitwise operations on integers for flag packing:
  #define READ_FLAG    (1 << 0)  // 0001
  #define WRITE_FLAG   (1 << 1)  // 0010
  #define EXEC_FLAG    (1 << 2)  // 0100
  
  int flags = READ_FLAG | WRITE_FLAG;  // 0011
  flags & READ_FLAG  // test: non-zero if read set
  flags &= ~WRITE_FLAG; // clear write flag
```

---

# Part III: Arrays — The Base of Everything

## 12. Static Arrays in Memory

An array is a contiguous block of memory holding elements of the same type. This is the most cache-friendly structure possible.

```
int arr[5] = {10, 20, 30, 40, 50};

Physical Memory Layout (int = 4 bytes, base = 0x1000):

  Index:   [  0  ] [  1  ] [  2  ] [  3  ] [  4  ]
  Offset:  +0      +4      +8      +12     +16
  Addr:    0x1000  0x1004  0x1008  0x100C  0x1010
           |       |       |       |       |
  Bytes:   10  00  20  00  30  00  40  00  50  00
           00  00  00  00  00  00  00  00  00  00
           (little-endian, showing 4 bytes per int)

ADDRESS FORMULA (why O(1) access):
  address(arr[i]) = base_address + i * sizeof(element)
  address(arr[3]) = 0x1000 + 3 * 4 = 0x100C
  
This is a SINGLE ARITHMETIC OPERATION.
No traversal, no pointer following, pure math.
This is WHY array indexing is O(1) — it's one ADD and one MULTIPLY.

All 5 ints fit in 20 bytes.
A 64-byte cache line holds 3 complete arrays like this (if aligned).
Accessing arr[0] automatically caches arr[1]-arr[15] (for larger arrays).
```

**Operations and WHY their complexity:**

| Operation | Complexity | Reason |
|-----------|-----------|--------|
| Access arr[i] | O(1) | Direct address calculation |
| Search (unsorted) | O(n) | Must check each element |
| Search (sorted) | O(log n) | Binary search halves space each step |
| Insert at end | O(1) amortized | Just write to next slot |
| Insert at middle | O(n) | Must shift all elements right |
| Delete at end | O(1) | Decrement length |
| Delete at middle | O(n) | Must shift all elements left |

---

## 13. Dynamic Arrays

A dynamic array (also called a vector or resizable array) wraps a static array with automatic resizing.

```
Dynamic Array Internal State:
  +--------+--------+--------+
  | ptr    | length |capacity|
  +--------+--------+--------+
      |
      v
  Heap: [10][20][30][__][__]  <- allocated capacity=5, used length=3

When you push element 40:
  length(3) < capacity(5), so:
  buf[3] = 40, length = 4
  -> O(1)

When length == capacity and you push 50:
  REALLOCATION:
  1. Allocate new buffer: capacity * GROWTH_FACTOR (typically 2x)
     new_buf = malloc(5 * 2 * sizeof(int)) = 40 bytes
  2. Copy old data: memcpy(new_buf, old_buf, length * sizeof(int))
     -> O(n) copy
  3. free(old_buf)
  4. ptr = new_buf, capacity = 10

WHY AMORTIZED O(1) for push:
  If we double each time:
  Sizes: 1, 2, 4, 8, 16, 32, ..., n
  Total copies: 1 + 2 + 4 + ... + n/2 = n - 1 < n
  
  So n pushes cause at most n total copies.
  Average copies per push = n/n = 1 = O(1) amortized.
  
  If growth factor = 1.5: Less memory waste, slightly more copies, still O(1) amortized.
  Rust uses 2x. Go uses ~1.25x for large slices. C++ std::vector uses 2x.
```

---

## 14. Multidimensional Arrays — Row-Major vs Column-Major

```
2D Array: int mat[3][4] (3 rows, 4 columns)

Logical view:       Physical memory (C uses ROW-MAJOR order):
[0,0][0,1][0,2][0,3]  
[1,0][1,1][1,2][1,3]  -> [0,0][0,1][0,2][0,3][1,0][1,1][1,2][1,3][2,0]...
[2,0][2,1][2,2][2,3]

Address formula (row-major):
  mat[i][j] = base + (i * num_cols + j) * sizeof(element)
  mat[1][2] = base + (1*4 + 2) * 4 = base + 24

COLUMN-MAJOR (Fortran, MATLAB):
  Physical: [0,0][1,0][2,0][0,1][1,1][2,1]...
  mat[i][j] = base + (j * num_rows + i) * sizeof(element)

CRITICAL PERFORMANCE IMPLICATION:

Row-major traversal (C-order, CACHE FRIENDLY):
  for i in 0..rows:
    for j in 0..cols:     <- inner loop moves along row (contiguous memory)
      sum += mat[i][j]    <- sequential memory access, excellent cache use
  
Column-major traversal (CACHE HOSTILE in C):
  for j in 0..cols:
    for i in 0..rows:     <- inner loop jumps by 'cols' elements each step
      sum += mat[i][j]    <- stride = cols * sizeof(T) — cache thrashing!

For a 1000x1000 int matrix:
  Row-major loop:    ~2ms  (cache friendly)
  Column-major loop: ~15ms (cache hostile, factor 7x slower!)
  
Same Big-O, wildly different real performance.
```

---

## 15. AoS vs SoA

**Array of Structures (AoS)** vs **Structure of Arrays (SoA)** is one of the most important practical performance decisions.

```
Suppose you have particles with position and color:

AoS (Array of Structures):                SoA (Structure of Arrays):
struct Particle {                         struct Particles {
    float x, y, z;   // 12B                  float *x;     // all X coords
    uint8_t r, g, b; // 3B                   float *y;     // all Y coords
    uint8_t pad;     // 1B (padding)         float *z;     // all Z coords
};                   // Total: 16B           uint8_t *r;   // all R values
Particle particles[1000];                    uint8_t *g;
                                             uint8_t *b;
Memory layout:                               int count;
[x0 y0 z0 r0 g0 b0 _][x1 y1 z1 r1 g1 b1 _] };
...|___|                                 Memory layout:
   |                                     [x0][x1][x2]...[x999] <- contiguous
   position data mixed with color data   [y0][y1][y2]...[y999]
                                         [z0][z1][z2]...[z999]

If your loop ONLY updates positions (physics simulation):

AoS: for each particle, you load 16B but only USE 12B (positions).
     Every cache line = 4 particles. You're loading color data you DON'T NEED.
     Cache utilization: 75% (12/16 bytes useful)

SoA: You iterate over x[], y[], z[] arrays separately.
     Cache lines contain ONLY position data. 100% cache utilization.
     SIMD (SSE/AVX) can process 4-8 floats at once from contiguous arrays.
     
SoA is the basis of Data-Oriented Design (DoD), used extensively in:
  - Game engines (ECS - Entity Component System)
  - Scientific computing
  - GPU computing (CUDA/OpenCL use SoA layout)
```

---

## 16. C Arrays and Pointer Arithmetic

```c
#include 
#include 
#include 

// Static array — lives on stack, size must be compile-time constant
void static_array_demo(void) {
    int arr[5] = {1, 2, 3, 4, 5};
    
    // Array name decays to pointer to first element in most contexts
    int *ptr = arr;              // equivalent to &arr[0]
    
    // Pointer arithmetic — scaled by sizeof(int) = 4 bytes
    printf("%d\n", *(ptr + 2)); // arr[2] = 3
    printf("%d\n", ptr[2]);     // same thing, [] is just syntax sugar
    printf("%d\n", 2[ptr]);     // also same! x[y] == *(x+y) == *(y+x) == y[x]
    
    // Array does NOT carry its size at runtime!
    // sizeof(arr) = 20 works only in same scope (compiler knows the type)
    // If passed to function: sizeof(ptr) = 8 (pointer size, not array size)
}

// Dynamic array — lives on heap, size determined at runtime
int* create_dynamic_array(int n) {
    int *arr = malloc(n * sizeof(int));  // allocate n ints
    if (!arr) return NULL;               // ALWAYS check malloc return
    
    for (int i = 0; i < n; i++) {
        arr[i] = i * i;
    }
    return arr;  // caller must free()
}

// 2D dynamic array
int** create_2d_array(int rows, int cols) {
    // Method 1: Array of pointers (rows can have different lengths)
    int **arr = malloc(rows * sizeof(int *));
    for (int i = 0; i < rows; i++) {
        arr[i] = malloc(cols * sizeof(int));
        // Each row is a separate heap allocation — NOT contiguous!
        // Bad for cache. Good for ragged arrays.
    }
    return arr;
    
    // Method 2: Single contiguous allocation (better cache behavior)
    // int *flat = malloc(rows * cols * sizeof(int));
    // Access: flat[i * cols + j]
}

// Realloc — resize a heap array
int* resize_array(int *arr, int old_size, int new_size) {
    int *new_arr = realloc(arr, new_size * sizeof(int));
    if (!new_arr) {
        // realloc failed, original arr is still valid!
        return NULL;
    }
    // If new_size > old_size, new elements are UNINITIALIZED
    // new_arr may or may not equal arr (may have been moved)
    return new_arr;
}
```

---

## 17. Go Slices — The Real Internals

Go slices are the most important Go data structure to understand deeply.

```go
// A slice is a 3-word struct: (pointer, length, capacity)
// In runtime/slice.go:
// type slice struct {
//     array unsafe.Pointer
//     len   int
//     cap   int
// }

package main

import (
    "fmt"
    "unsafe"
)

func sliceInternals() {
    // Backing array: [0][1][2][3][4][5][6][7][8][9]
    backing := [10]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
    
    // Slice s1: points to backing[2], len=4, cap=8
    s1 := backing[2:6]  // elements 2,3,4,5
    
    // Memory layout:
    // s1.array -> &backing[2] (address 0x1008 if backing starts at 0x1000)
    // s1.len  = 4
    // s1.cap  = 8  (from index 2 to end = 10 - 2 = 8)
    
    // s1 and backing SHARE the same underlying array!
    s1[0] = 999      // This also changes backing[2]!
    fmt.Println(backing[2]) // 999
    
    // Append within capacity: NO new allocation
    s2 := s1[:6]    // extend to len=6, cap still 8, same backing array
    
    // Append beyond capacity: NEW allocation, copy, disconnect
    s3 := append(s1, 10, 11, 12, 13, 14) // exceeds cap=8 from our position
    // Now s3 has its OWN backing array — modifying s3 doesn't affect backing
    
    // DANGER: Append within capacity can corrupt data
    a := []int{1, 2, 3, 4, 5}
    b := a[:3]        // b = [1, 2, 3], cap = 5, shares backing with a
    b = append(b, 99) // len(b) < cap(b), writes 99 to a[3]!
    fmt.Println(a)    // [1 2 3 99 5] — a was modified!
    
    // Safe copy: use copy() to create independent slice
    c := make([]int, len(b))
    copy(c, b) // always allocates independent backing array
    
    // Slice header size: always 24 bytes (3 * 8-byte words on 64-bit)
    fmt.Println(unsafe.Sizeof(s1)) // 24
}

// nil slice vs empty slice
func nilVsEmpty() {
    var s1 []int           // nil slice: {nil, 0, 0}
    s2 := []int{}          // empty slice: {non-nil ptr, 0, 0}
    s3 := make([]int, 0)   // empty slice: {non-nil ptr, 0, 0}
    
    fmt.Println(s1 == nil)  // true
    fmt.Println(s2 == nil)  // false
    fmt.Println(len(s1))    // 0 (safe to call on nil slice)
    fmt.Println(cap(s1))    // 0
    
    // append works on nil slice
    s1 = append(s1, 1, 2, 3) // allocates backing array
}

// Growth strategy in Go (Go 1.18+):
// If cap < 256: newcap = oldcap * 2
// If cap >= 256: grows by ~1.25x with smoothing to avoid too-small increments
// These details can change between Go versions.
```

---

## 18. Rust Vec<T>

```rust
use std::alloc::{alloc, dealloc, Layout};
use std::ptr;

// Vec internal structure (simplified):
// pub struct Vec {
//     buf: RawVec,  // (ptr: NonNull, cap: usize, alloc: A)
//     len: usize,
// }
// Total: 24 bytes on 64-bit (pointer + capacity + length)

fn vec_internals() {
    let mut v: Vec = Vec::new();
    // v.ptr -> dangling (non-null but invalid, 0-capacity special case)
    // v.len = 0, v.cap = 0
    
    v.push(1); // cap was 0 -> allocates for 4 elements (Rust starts at 4)
    v.push(2);
    v.push(3);
    v.push(4);
    // v.len = 4, v.cap = 4
    
    v.push(5); // len == cap -> reallocate to cap * 2 = 8
    // v.len = 5, v.cap = 8
    
    // Pre-allocate: avoid reallocations when size is known
    let mut v2: Vec = Vec::with_capacity(1000);
    // v2.len = 0, v2.cap = 1000, ONE allocation upfront
    
    // Shrink
    v2.shrink_to_fit(); // release unused capacity, may reallocate
    
    // From raw parts (unsafe, advanced):
    // let v3 = Vec::from_raw_parts(ptr, len, cap);
    // Gives you full control over the backing allocation
    
    // Slices from Vec: Vec derefs to &[T]
    let slice: &[i32] = &v; // borrow the whole vec as a slice
    let sub: &[i32] = &v[1..3]; // sub-slice [2, 3]
    
    // Ownership rules prevent dangling pointers:
    let s = v.as_ptr(); // raw pointer — valid as long as v is alive and not mutated
    v.push(6);          // this MIGHT reallocate, making s invalid!
    // If we tried to use s now, that's undefined behavior.
    // Rust's borrow checker prevents this: you can't hold s and mutate v simultaneously.
}

// Rust's ownership makes certain bugs impossible:
fn ownership_demo() {
    let v1 = vec![1, 2, 3];
    let v2 = v1;           // v1 is MOVED into v2
    // println!("{:?}", v1); // COMPILE ERROR: v1 was moved
    
    let v3 = vec![1, 2, 3];
    let v4 = v3.clone();   // explicit deep copy
    println!("{:?}", v3);  // still valid
    
    // Borrowing:
    let v5 = vec![1, 2, 3];
    let r = &v5;           // immutable borrow
    println!("{:?}", r);   // OK
    // v5.push(4);         // ERROR: cannot mutate while borrowed
}
```

---

# Part IV: Strings

## 19. C Strings — Null-terminated

```c
// C strings are arrays of chars terminated by '\0' (null byte = 0x00)
// They have NO length field — you must scan to find the end.

char s[] = "hello";

// Memory layout (6 bytes):
//  Index: [0]  [1]  [2]  [3]  [4]  [5]
//  Char:  'h'  'e'  'l'  'l'  'o'  '\0'
//  Hex:   0x68 0x65 0x6C 0x6C 0x6F 0x00

// strlen("hello") = O(n): must scan until '\0'
// String literals are stored in the read-only .rodata section
const char *literal = "hello"; // pointer to read-only memory
// literal[0] = 'H'; // UNDEFINED BEHAVIOR (segfault on most systems)

char mutable[] = "hello"; // copies to stack, CAN be modified
mutable[0] = 'H';         // OK

// Buffer overflow — the classic C vulnerability:
char buf[8];
strcpy(buf, "this is too long!"); // writes past buf! stack smashing!
// strncpy(buf, src, sizeof(buf) - 1) is safer but still has caveats
// snprintf(buf, sizeof(buf), ...) is the modern approach

// String operations are all O(n) unless strlen is cached:
// strlen, strcpy, strcmp, strcat — ALL require scanning to '\0'

// NULL vs empty string:
char *null_str = NULL;   // pointer is null — no string at all
char *empty_str = "";    // pointer to valid memory containing just '\0'
strlen(null_str);        // CRASH (undefined behavior, usually segfault)
strlen(empty_str);       // 0 (safe)
```

---

## 20. Go Strings — Immutable UTF-8 Slice Header

```go
// A Go string is a 2-word struct: (pointer, length)
// type StringHeader struct {
//     Data uintptr
//     Len  int
// }
// Strings in Go are IMMUTABLE and always valid UTF-8.

package main

import (
    "fmt"
    "unicode/utf8"
)

func goStrings() {
    s := "Hello, 世界"
    
    // String header: ptr to UTF-8 bytes + byte count (NOT rune count)
    // len(s) = byte length, NOT character count!
    fmt.Println(len(s))                    // 13 (7 ASCII + 3 bytes each for 世 and 界)
    fmt.Println(utf8.RuneCountInString(s)) // 9 (9 Unicode code points)
    
    // Indexing gives bytes, not runes:
    fmt.Printf("%c\n", s[0])  // 'H' (works for ASCII)
    fmt.Printf("%c\n", s[7])  // first byte of '世' (0xE4), NOT '世'
    
    // Correct iteration over Unicode characters:
    for i, r := range s {
        fmt.Printf("byte %d: %c (%d)\n", i, r, r)
        // i is byte offset, r is rune (Unicode code point)
    }
    
    // String is a read-only byte slice under the hood:
    b := []byte(s)    // COPIES the bytes (string is immutable, []byte is mutable)
    b[0] = 'h'
    fmt.Println(string(b)) // "hello, 世界"
    fmt.Println(s)          // "Hello, 世界" — unchanged
    
    // String concatenation:
    // s1 + s2: allocates new backing array, copies both — O(n)
    // For building strings in a loop, use strings.Builder (like StringBuilder)
    
    // String interning: string literals in Go are stored in .rodata
    // Two identical string literals MAY share the same backing memory
    // (but you cannot rely on this for pointer equality)
}
```

---

## 21. Rust Strings — String vs &str

```rust
// Rust has two string types:
// &str  = string SLICE: (ptr, len) — a borrowed view into string data
//         Immutable, can point to heap, stack, or .rodata
// String = owned, heap-allocated, growable UTF-8 string
//          Internally: Vec that is guaranteed valid UTF-8
//          (ptr, len, cap) — 24 bytes on 64-bit

fn rust_strings() {
    // &str — string slice (borrowed, no ownership)
    let s1: &str = "hello";      // points to .rodata (binary's read-only data)
    let s2: &str = &String::from("world"); // points to heap (but lifetime!)
    
    // String — owned heap string
    let mut owned: String = String::from("hello");
    owned.push_str(", world"); // mutates in place
    owned.push('!');
    
    // Convert between them:
    let slice: &str = &owned;    // borrow as slice (zero-copy)
    let owned2: String = slice.to_string();  // or: String::from(slice) — copies
    
    // Indexing by bytes (not chars):
    // let c = owned[0]; // COMPILE ERROR: Rust won't let you index by byte
    //                     because a UTF-8 char may be 1-4 bytes
    let c = &owned[0..1]; // OK: byte slice [0..1) = 1 byte
    
    // Iterate over chars (Unicode code points):
    for ch in "héllo".chars() {
        print!("{} ", ch); // h é l l o
    }
    
    // Iterate over bytes:
    for b in "héllo".bytes() {
        print!("{} ", b); // 104 195 169 108 108 111
    }
    
    // len() = byte count, chars().count() = character count
    let s = "héllo";
    println!("{}", s.len());              // 6 (é is 2 bytes in UTF-8)
    println!("{}", s.chars().count());    // 5

    // String is UTF-8 guaranteed — invalid UTF-8 is caught at construction
    // let invalid = String::from_utf8(vec![0xFF, 0xFE]); // Err(...)
}
```

---

# Part V: Linked Lists

## 22. Singly Linked List

A linked list stores elements in nodes, where each node contains data and a pointer to the next node. Nodes are NOT contiguous in memory.

```
SINGLY LINKED LIST — Memory Reality

head
 |
 v
+-------+------+         +-------+------+         +-------+------+
| data  | next |-------->| data  | next |-------->| data  | next |---> NULL
|  10   |0x7B20|         |  20   |0x3F40|         |  30   | NULL |
+-------+------+         +-------+------+         +-------+------+
  0x7A10                   0x7B20                   0x3F40

NOTICE: Nodes are scattered in heap memory (0x7A10, 0x7B20, 0x3F40).
        No spatial locality. Each access to next node is a cache miss.

Structure of a node:
  +----------+----------+
  |  data    |  *next   |
  | (T bytes)| (8 bytes)|
  +----------+----------+
  Total: sizeof(T) + 8 bytes per node (plus malloc overhead ~16-32B)

For int nodes: 4 + 8 = 12 bytes, padded to 16 bytes per node.
For array int[1000]: 4000 bytes total.
For linked list int[1000]: 16 * 1000 = 16000 bytes. 4x more memory!
Plus: each node is a separate malloc call → heap fragmentation.
```

**Operations:**

| Operation | Complexity | Reason |
|-----------|-----------|--------|
| Access by index | O(n) | Must traverse from head |
| Search | O(n) | Linear scan |
| Insert at head | O(1) | Just update head pointer |
| Insert at tail (with tail ptr) | O(1) | Direct tail access |
| Insert after known node | O(1) | Just update pointers |
| Delete at head | O(1) | Update head pointer |
| Delete arbitrary (with ptr) | O(1) | Update predecessor's next |
| Delete by value | O(n) | Must find node first |

**Why use linked lists despite cache miss penalty?**
- Constant-time insertion/deletion without shifting (unlike arrays)
- Useful when: elements are large, insertions/deletions dominate over reads
- When you have a pointer to the node already (no search needed)
- When the data structure size changes frequently and unpredictably

---

## 23. Doubly Linked List

```
DOUBLY LINKED LIST

NULL <--+-------+-------+-------+--> +-------+-------+-------+--> +-------+-------+-------+--> NULL
        | prev  | data  | next  |    | prev  | data  | next  |    | prev  | data  | next  |
        | NULL  |  10   |  0x30 |    | 0x10  |  20   |  0x50 |    | 0x30  |  30   | NULL  |
        +-------+-------+-------+    +-------+-------+-------+    +-------+-------+-------+
          0x10                          0x30                          0x50

Each node: sizeof(T) + 8 (prev) + 8 (next) = sizeof(T) + 16 bytes

Benefits over singly linked:
  - Can traverse BACKWARD (O(1) with pointer to node)
  - Delete a node in O(1) given ONLY a pointer to that node
    (singly linked needs predecessor — must scan from head O(n))
  - Simpler implementation of deque operations

head = 0x10
tail = 0x50

Insert before node at 0x30 (new node = 0x20 with data=15):
  new->prev = 0x10  (node_30->prev)
  new->next = 0x30  (the node itself)
  node_10->next = 0x20  (predecessor's next)
  node_30->prev = 0x20  (the node's prev)
  — All O(1), no traversal!
```

---

## 24. Circular Linked List

```
CIRCULAR LINKED LIST (Singly)

head
 |
 v
+------+------+    +------+------+    +------+------+
| data | next |--->| data | next |--->| data | next |
|  10  |      |    |  20  |      |    |  30  |      |
+------+------+    +------+------+    +------+------+
        ^                                    |
        |____________________________________|
                    (last node points back to head)

No NULL terminator — the list wraps around.

Use cases:
  - Round-robin scheduling (CPU scheduler, time-sharing)
  - Circular buffer implemented as linked list
  - Music player playlist that loops
  - Linux kernel process lists

DANGER: Traversal must track start to avoid infinite loop.
  node = head;
  do {
      process(node);
      node = node->next;
  } while (node != head);
```

---

## 25. XOR Linked List

An XOR linked list stores only one pointer per node instead of two (for doubly linked lists), using the XOR trick.

```
XOR Doubly Linked List

Each node stores: both = prev_addr XOR next_addr

To get next: next = prev_addr XOR both
To get prev: prev = both XOR next_addr

Node A (0x10): both = NULL XOR 0x30 = 0x30
Node B (0x30): both = 0x10 XOR 0x50 = 0x60
Node C (0x50): both = 0x30 XOR NULL = 0x30

Traversal forward (start at A, prev=NULL):
  At A: next = NULL XOR 0x30 = 0x30 ✓
  At B: next = 0x10 XOR 0x60 = 0x50 ✓ (prev was 0x10, both=0x60)
  At C: next = 0x30 XOR 0x30 = NULL ✓

Memory: saves 8 bytes per node (vs doubly linked)
Cost: cannot follow arbitrary pointers (must traverse from head)
      garbage collectors cannot trace XOR pointers
      practically unused in modern code (GC incompatible)
      interesting CS concept, Linux kernel does NOT use this
```

---

## 26. Intrusive vs Non-intrusive

This distinction is crucial for systems programming.

```
NON-INTRUSIVE (Standard library approach):
  The container OWNS the nodes. The node is an internal detail.
  
  struct Node {
      int data;          // your data is INSIDE the node
      Node *next;        // linkage pointer
  };
  
  Memory:
  [node: data=10, next→] [node: data=20, next→] [node: data=30, NULL]
  
  + Simple API
  - Each node requires separate allocation
  - Object can only be in ONE list at a time
  - Node allocation/deallocation is tied to container

INTRUSIVE (Linux kernel approach):
  The node/linkage is INSIDE your data structure.
  
  struct Task {
      int pid;
      char name[64];
      struct ListNode node;  // embedded link — SEPARATE from data
      int priority;
  };
  
  Memory:
  [pid][name][NODE: prev, next][priority]  <- all one allocation
  
  Advantages:
  + ONE allocation per object (not two: object + container node)
  + Object can be in MULTIPLE lists simultaneously (multiple ListNode fields)
  + Cache friendly: accessing list neighbors also loads part of their data
  + No extra heap allocation for the link — list is intrinsic to the object
  + Given pointer to node, recover pointer to containing object via offset math
  
  Disadvantages:
  - More complex API
  - Must embed the link in the struct definition
  - Object knows about its container (coupling)
  
Linux kernel uses ONLY intrusive data structures.
Every kernel object (task, socket, inode, timer, etc.) has list_head/rb_node embedded.
```

---

## 27. Linked List Implementations

### C — Singly Linked List

```c
#include 
#include 

// Node definition
typedef struct Node {
    int data;
    struct Node *next;
} Node;

// Linked list handle
typedef struct {
    Node *head;
    Node *tail;
    size_t size;
} LinkedList;

// Create a new node (heap allocation)
Node* node_create(int data) {
    Node *n = malloc(sizeof(Node));
    if (!n) return NULL;
    n->data = data;
    n->next = NULL;
    return n;
}

// Insert at head: O(1)
void list_push_front(LinkedList *list, int data) {
    Node *n = node_create(data);
    if (!n) return;
    n->next = list->head;
    list->head = n;
    if (!list->tail) list->tail = n;
    list->size++;
}

// Insert at tail: O(1) with tail pointer
void list_push_back(LinkedList *list, int data) {
    Node *n = node_create(data);
    if (!n) return;
    if (list->tail) list->tail->next = n;
    else list->head = n;
    list->tail = n;
    list->size++;
}

// Search: O(n)
Node* list_find(LinkedList *list, int data) {
    for (Node *cur = list->head; cur; cur = cur->next)
        if (cur->data == data) return cur;
    return NULL;
}

// Delete by value: O(n) — needs predecessor
void list_delete(LinkedList *list, int data) {
    Node *prev = NULL, *cur = list->head;
    while (cur && cur->data != data) {
        prev = cur;
        cur = cur->next;
    }
    if (!cur) return; // not found
    if (prev) prev->next = cur->next;
    else list->head = cur->next;
    if (!cur->next) list->tail = prev;
    free(cur);
    list->size--;
}

// Reverse in-place: O(n), O(1) space
void list_reverse(LinkedList *list) {
    Node *prev = NULL, *cur = list->head, *next = NULL;
    list->tail = list->head;
    while (cur) {
        next = cur->next;
        cur->next = prev;
        prev = cur;
        cur = next;
    }
    list->head = prev;
}

// Free entire list: O(n)
void list_free(LinkedList *list) {
    Node *cur = list->head, *next;
    while (cur) {
        next = cur->next;
        free(cur);
        cur = next;
    }
    list->head = list->tail = NULL;
    list->size = 0;
}
```

### Go — Generic Linked List

```go
package list

// Generic node using Go 1.18+ generics
type Node[T any] struct {
    Value T
    Next  *Node[T]
}

type SinglyList[T comparable] struct {
    Head *Node[T]
    Tail *Node[T]
    Size int
}

func (l *SinglyList[T]) PushFront(v T) {
    n := &Node[T]{Value: v, Next: l.Head}
    if l.Tail == nil {
        l.Tail = n
    }
    l.Head = n
    l.Size++
}

func (l *SinglyList[T]) PushBack(v T) {
    n := &Node[T]{Value: v}
    if l.Tail != nil {
        l.Tail.Next = n
    } else {
        l.Head = n
    }
    l.Tail = n
    l.Size++
}

func (l *SinglyList[T]) Find(v T) *Node[T] {
    for n := l.Head; n != nil; n = n.Next {
        if n.Value == v {
            return n
        }
    }
    return nil
}

// Go's container/list provides a doubly linked list
// It is non-intrusive and stores interface{} (pre-generics) or any (with generics)
import "container/list"

func stdListDemo() {
    l := list.New() // doubly linked list
    l.PushBack(1)
    l.PushBack(2)
    e := l.PushBack(3) // returns *list.Element
    l.InsertBefore(99, e) // insert 99 before element 3
    
    for e := l.Front(); e != nil; e = e.Next() {
        fmt.Println(e.Value)
    }
}
```

### Rust — Safe Doubly Linked List

```rust
// Rust makes linked lists famously difficult to implement safely.
// The reason: Rust's ownership model requires single ownership,
// but doubly linked lists have shared ownership (each node is owned by
// its predecessor AND successor AND possibly external references).

use std::collections::LinkedList; // Standard library (uses unsafe internally)

fn rust_linked_list() {
    let mut list: LinkedList = LinkedList::new();
    list.push_back(1);
    list.push_back(2);
    list.push_front(0);
    
    // Cursors (nightly feature): allows O(1) insert/delete at arbitrary positions
    // list.cursor_front_mut().insert_after(99);
    
    for val in &list {
        println!("{}", val);
    }
    
    list.pop_front(); // O(1)
    list.pop_back();  // O(1)
}

// Safe singly linked list using Option<Box>:
// Option<Box<Node>> = either None or an owned heap-allocated node
type Link = Option<Box<Node>>;

struct Node {
    elem: T,
    next: Link,
}

struct Stack {
    head: Link,
}

impl Stack {
    fn push(&mut self, elem: T) {
        let new_node = Box::new(Node {
            elem,
            next: self.head.take(), // take() replaces head with None, returns old value
        });
        self.head = Some(new_node);
    }

    fn pop(&mut self) -> Option {
        self.head.take().map(|node| {
            self.head = node.next;
            node.elem
        })
    }
    // When Stack drops, it recursively drops all nodes automatically!
    // Rust's Drop trait handles cleanup without explicit free() calls.
}
```

---

## 28. Linux Kernel list_head

The Linux kernel's intrusive doubly linked list is one of the most used data structures in the kernel. It is implemented in `include/linux/list.h`.

```c
// The list_head struct — just two pointers
struct list_head {
    struct list_head *next, *prev;
};

// Example: the kernel's task list
// In include/linux/sched.h:
struct task_struct {
    volatile long       state;
    void               *stack;
    pid_t               pid;
    // ... hundreds of fields ...
    struct list_head    tasks;      // link in the global task list
    struct list_head    children;   // link in parent's children list
    struct list_head    sibling;    // link in parent's children list (same)
    // ... more fields ...
};

// Memory layout of a task_struct node in the list:
//
//  +----task_struct A----+   +----task_struct B----+
//  |  pid = 1234        |   |  pid = 1235          |
//  |  ...               |   |  ...                 |
//  |  tasks:            |   |  tasks:              |
//  |  +------+------+   |   |  +------+------+     |
//  |  | prev | next |   | prev | next |      |     |
//  |  +------+------+   |   |  +------+------+     |
//  |  ...               |   |  ...                 |
//  +--------------------+   +----------------------+
//
// 'next' and 'prev' point to the list_head INSIDE another task_struct,
// NOT to the beginning of task_struct!

// The INIT_LIST_HEAD macro initializes a list head to point to itself:
// (empty circular list)
#define INIT_LIST_HEAD(ptr) do {            \
    (ptr)->next = (ptr);                    \
    (ptr)->prev = (ptr);                    \
} while (0)

// Adding to end of list:
// list_add_tail(new, head) — inserts new before head (i.e., at tail)
static inline void __list_add(struct list_head *new,
                               struct list_head *prev,
                               struct list_head *next) {
    next->prev = new;
    new->next = next;
    new->prev = prev;
    prev->next = new;
}

// The container_of macro — the MAGIC of intrusive lists
// Given a pointer to an embedded list_head, find the containing struct.
//
// #define container_of(ptr, type, member) ({          \
//     void *__mptr = (void *)(ptr);                   \
//     ((type *)(__mptr - offsetof(type, member)));    \
// })
//
// Example:
// struct list_head *pos = some_list_head_pointer;
// struct task_struct *task = container_of(pos, struct task_struct, tasks);
//
// Mathematics:
//   &task->tasks = (char*)task + offsetof(task_struct, tasks)
//   So: task = (char*)&task->tasks - offsetof(task_struct, tasks)
//             = (task_struct *)((char*)pos - offsetof(task_struct, tasks))

// Iterating the task list:
// list_for_each_entry(pos, head, member) — iterates using container_of
//
// Example (iterating all processes):
// struct task_struct *task;
// list_for_each_entry(task, &init_task.tasks, tasks) {
//     printk("PID: %d, CMD: %s\n", task->pid, task->comm);
// }
//
// This expands to:
// for (task = list_first_entry(&init_task.tasks, struct task_struct, tasks);
//      &task->tasks != &init_task.tasks;
//      task = list_next_entry(task, tasks))

// list_for_each_entry_safe — safe version for deletion during iteration
// Saves 'next' before processing, so deleting 'pos' doesn't break iteration:
// list_for_each_entry_safe(pos, n, head, member)
```

**Key operations (all O(1) except traversal):**

```c
list_add(new, head)         // insert after head (like push_front for FIFO stack)
list_add_tail(new, head)    // insert before head (append, like push_back)
list_del(entry)             // remove entry from list
list_move(list, head)       // remove from current list, add to head of another
list_move_tail(list, head)  // remove from current, add to tail of another
list_empty(head)            // check if list is empty (head->next == head)
list_splice(list, head)     // join two lists
```

---

# Part VI: Stacks

## 29. Array-based Stack

```
STACK — Last In, First Out (LIFO)

Physical memory layout (array-based):

           top
            |
+----+----+----v---+----+----+----+----+
|  1 |  2 |  3  .  |    |    |    |    |
+----+----+----+---+----+----+----+----+
  0    1    2          3    4    5    6
        capacity = 8

top = 2 (index of top element)

PUSH(4):
  top++; arr[top] = 4;
  top is now 3

+----+----+----+----+----+----+----+----+
|  1 |  2 |  3 |  4 |    |    |    |    |
+----+----+----+----+----+----+----+----+

POP():
  val = arr[top]; top--;
  Returns 4, top is now 2

PEEK():
  return arr[top] without modifying top

OVERFLOW: push when top == capacity-1 (fixed array) or realloc (dynamic)
UNDERFLOW: pop when top == -1

WHY O(1) for all operations:
  Push: arr[++top] = val — one increment, one write
  Pop:  val = arr[top--] — one read, one decrement
  Peek: return arr[top]  — one read
  No traversal, no pointer chasing, single array access.
```

---

## 30. Linked-list-based Stack

```
LINKED LIST STACK

head (top of stack)
  |
  v
+------+------+    +------+------+    +------+------+
|  3   | next |--->|  2   | next |--->|  1   | next |---> NULL
+------+------+    +------+------+    +------+------+

PUSH(4): Create new node, point to current head, update head
  new_node = {data: 4, next: head}
  head = new_node

head
  |
  v
+------+------+    +------+------+    ...
|  4   | next |--->|  3   | next |-->
+------+------+    +------+------+

POP(): Save head->data, advance head to head->next, free old head
  val = head->data
  old = head
  head = head->next
  free(old)

Advantages over array stack:
  - No overflow (limited only by heap memory)
  - No need to resize/copy
Disadvantages:
  - Extra memory per node (8 bytes for pointer + malloc overhead ~16 bytes)
  - Cache-unfriendly (nodes scattered in memory)
  - Slower due to malloc/free per push/pop
  
For most use cases, array-based stack is FASTER due to cache efficiency.
```

---

## 31. The Hardware Call Stack — Stack Frames

The most important stack in systems programming is the CPU's own call stack.

```
CALL STACK (x86-64 Linux, System V ABI)

When main() calls foo() which calls bar():

High Memory (stack base)
+========================+ <- Initial RSP (stack pointer)
|  argc, argv, envp      |  (passed from kernel to _start)
+========================+

+========================+ <- RSP when main() starts
|  main's local vars     |
|  main's saved regs     |
|  [return addr to libc] |
+========================+ <- RSP when foo() is called

+========================+ <- RSP when foo() starts
|  foo's local vars      |  <- RBP-8, RBP-16, etc.
|  foo's saved regs      |
|  return addr to main   |  <- (RBP+8): the address in main after the call
+------------------------+ <- RBP (frame base pointer for foo)
|  previous RBP (main's) |  <- what RBP was in main (saved)
+========================+ <- RSP when bar() is called

+========================+ <- RSP when bar() starts
|  bar's local vars      |
|  bar's saved regs      |
|  return addr to foo    |
+------------------------+ <- RBP (frame base pointer for bar)
|  previous RBP (foo's)  |
+========================+ <- Current RSP

Low Memory (stack grows down ↓)

KEY REGISTERS:
  RSP (Stack Pointer): points to TOP of stack (lowest used address)
  RBP (Base Pointer):  points to current frame's base (for debugger unwinding)
  
FUNCTION CALL MECHANICS:
  CALL instruction:
    1. PUSH return address (RIP + sizeof(call instruction)) onto stack
    2. JMP to function address
  
  Function prologue:
    PUSH RBP           ; save caller's frame pointer
    MOV RBP, RSP       ; set new frame pointer
    SUB RSP, N         ; allocate N bytes for local variables
  
  Function epilogue:
    MOV RSP, RBP       ; restore stack pointer (deallocate locals)
    POP RBP            ; restore caller's frame pointer
    RET                ; pop return address, jump to it

ARGUMENT PASSING (x86-64 System V ABI):
  First 6 integer args: RDI, RSI, RDX, RCX, R8, R9
  First 8 float args:   XMM0-XMM7
  Additional args:      pushed onto stack (right to left)
  Return value:         RAX (integer), XMM0 (float)
```

**Stack Overflow:**
```
Recursive function without base case:
  factorial(n) calls factorial(n-1) calls factorial(n-2) ...
  Each call adds a frame (~100-200 bytes)
  At ~8000 frames: RSP crosses into a guard page (kernel-mapped as non-accessible)
  -> SIGSEGV (Segmentation Fault)
  Linux default stack size: 8MB (ulimit -s)
  Can be changed: ulimit -s unlimited or pthread_attr_setstacksize()
```

---

## 32. Stack Implementations

### C

```c
#define STACK_MAX 1024

typedef struct {
    int data[STACK_MAX];
    int top;
} IntStack;

void stack_init(IntStack *s)    { s->top = -1; }
bool stack_empty(IntStack *s)   { return s->top == -1; }
bool stack_full(IntStack *s)    { return s->top == STACK_MAX - 1; }
bool stack_push(IntStack *s, int v) {
    if (stack_full(s)) return false;
    s->data[++s->top] = v;
    return true;
}
bool stack_pop(IntStack *s, int *v) {
    if (stack_empty(s)) return false;
    *v = s->data[s->top--];
    return true;
}
int  stack_peek(IntStack *s)    { return s->data[s->top]; }
```

### Go

```go
// Generic stack using a slice
type Stack[T any] struct {
    data []T
}

func (s *Stack[T]) Push(v T) {
    s.data = append(s.data, v)
}

func (s *Stack[T]) Pop() (T, bool) {
    var zero T
    if len(s.data) == 0 {
        return zero, false
    }
    n := len(s.data) - 1
    val := s.data[n]
    s.data = s.data[:n]
    return val, true
}

func (s *Stack[T]) Peek() (T, bool) {
    var zero T
    if len(s.data) == 0 {
        return zero, false
    }
    return s.data[len(s.data)-1], true
}

func (s *Stack[T]) Len() int { return len(s.data) }
```

### Rust

```rust
struct Stack {
    data: Vec,
}

impl Stack {
    fn new() -> Self { Stack { data: Vec::new() } }
    fn push(&mut self, v: T) { self.data.push(v); }
    fn pop(&mut self) -> Option { self.data.pop() }
    fn peek(&self) -> Option { self.data.last() }
    fn is_empty(&self) -> bool { self.data.is_empty() }
    fn len(&self) -> usize { self.data.len() }
}
```

---

# Part VII: Queues and Circular Buffers

## 33. Simple Queue

A queue is First In, First Out (FIFO). The element added first is removed first.

```
QUEUE — FIFO (First In, First Out)

Array-based (naive — wastes space):
  head                    tail
   |                       |
   v                       v
  +----+----+----+----+----+----+----+----+
  |    |    |  A |  B |  C |  D |    |    |
  +----+----+----+----+----+----+----+----+
            dequeue end              enqueue end

Problem: head and tail move ONLY forward.
After many enqueue/dequeue cycles, head drifts right.
Even if the left side is empty, we can't use it.
-> Wasted space -> Need circular buffer.
```

---

## 34. Circular Buffer — Ring Buffer

The circular buffer solves the "drifting head" problem by wrapping around.

```
CIRCULAR BUFFER (Ring Buffer) — Size = 8

Initial state (empty):
  head = 0, tail = 0
  +----+----+----+----+----+----+----+----+
  |    |    |    |    |    |    |    |    |
  +----+----+----+----+----+----+----+----+
   0    1    2    3    4    5    6    7
  head=tail=0 (when head==tail: buffer is empty)

After enqueue A, B, C, D (tail advances):
  head = 0, tail = 4
  +----+----+----+----+----+----+----+----+
  | A  | B  | C  | D  |    |    |    |    |
  +----+----+----+----+----+----+----+----+
  head=0           tail=4

After dequeue A, B (head advances):
  head = 2, tail = 4
  +----+----+----+----+----+----+----+----+
  |    |    | C  | D  |    |    |    |    |
  +----+----+----+----+----+----+----+----+
            head=2   tail=4

After enqueue E, F, G, H, I (tail wraps around):
  head = 2, tail = 1 (wrapped: 4+5=9 -> 9%8=1, but wait: full check needed)
  
FULL when: (tail + 1) % size == head
EMPTY when: head == tail

  +----+----+----+----+----+----+----+----+
  | I  |    | C  | D  | E  | F  | G  | H  |
  +----+----+----+----+----+----+----+----+
  tail=1  head=2

ENQUEUE:
  if FULL: error (or overwrite oldest — depends on use case)
  buf[tail] = item
  tail = (tail + 1) % capacity

DEQUEUE:
  if EMPTY: error
  item = buf[head]
  head = (head + 1) % capacity

All operations: O(1). No memory allocation during operation.
Memory: capacity * sizeof(T). Fixed. No fragmentation.

WHY circular buffers are perfect for:
  - Producer-consumer between threads/processes
  - Interrupt handlers (kernel writes, user reads)
  - Network buffers (network card -> kernel buffer)
  - Audio buffers (tiny latency, predictable timing)
  - Linux kfifo uses this exact design
```

---

## 35. Deque — Double-ended Queue

```
DEQUE (Double-Ended Queue) allows insertion and deletion at BOTH ends.

Implemented as a doubly linked list OR a circular buffer:

Doubly linked deque:
  NULL <-- [A] <--> [B] <--> [C] <--> [D] --> NULL
            ^                           ^
           front                       back

push_front(X): O(1) — prepend
push_back(Y):  O(1) — append
pop_front():   O(1) — remove from front
pop_back():    O(1) — remove from back

Alternative: Deque as block array (Go's list, C++ std::deque):
  A deque is implemented as an array of pointers to fixed-size blocks.
  
  Block 0:  [_, _, A, B, C]  <- partially filled from middle
  Block 1:  [D, E, F, G, H]  <- full
  Block 2:  [I, J, _, _, _]  <- partially filled from beginning
  Index map: [ptr0, ptr1, ptr2]
  
  This allows O(1) push/pop at both ends without copying.
  Random access is O(1) but with one extra pointer dereference.
  Better cache behavior than linked list.
```

---

## 36. Priority Queue

A priority queue always dequeues the element with the highest (or lowest) priority.

```
PRIORITY QUEUE (Min-Heap implementation)

Always dequeue the SMALLEST element:

Internal heap array: [1, 4, 3, 7, 8, 5, 9]

As a tree:
          1
         / \
        4   3
       / \ / \
      7  8 5  9

insert(2):
  Add 2 at end: [1, 4, 3, 7, 8, 5, 9, 2]
  Bubble up: parent(7) = (7-1)/2 = 3 -> arr[3] = 7 > 2, swap
  Bubble up: parent(3) = (3-1)/2 = 1 -> arr[1] = 4 > 2, swap
  Done: [1, 2, 3, 7, 4, 5, 9, 8]
  -> O(log n) because we climb at most log2(n) levels

extract_min():
  Save root (1), put last element (8) at root
  [8, 2, 3, 7, 4, 5, 9]
  Heapify down: min-child of 8 is min(2,3)=2, swap
  [2, 8, 3, 7, 4, 5, 9]
  Heapify down: min-child of 8 is min(7,4)=4, swap
  [2, 4, 3, 7, 8, 5, 9]
  4's children: 7 and 8, both >= 4, stop
  -> O(log n)

Use cases:
  - Dijkstra's shortest path
  - A* search
  - Task scheduling (highest priority first)
  - Huffman coding
  - Linux kernel CFS scheduler (uses an rbtree, but conceptually a priority queue)
```

---

## 37. Queue Implementations

### C — Circular Buffer Queue

```c
#define QUEUE_CAP 1024

typedef struct {
    int data[QUEUE_CAP];
    int head, tail, size;
} IntQueue;

void queue_init(IntQueue *q)       { q->head = q->tail = q->size = 0; }
bool queue_empty(IntQueue *q)      { return q->size == 0; }
bool queue_full(IntQueue *q)       { return q->size == QUEUE_CAP; }

bool queue_enqueue(IntQueue *q, int v) {
    if (queue_full(q)) return false;
    q->data[q->tail] = v;
    q->tail = (q->tail + 1) % QUEUE_CAP;
    q->size++;
    return true;
}

bool queue_dequeue(IntQueue *q, int *v) {
    if (queue_empty(q)) return false;
    *v = q->data[q->head];
    q->head = (q->head + 1) % QUEUE_CAP;
    q->size--;
    return true;
}
```

### Go

```go
type Queue[T any] struct {
    data []T
    head, tail, size int
}

func NewQueue[T any](cap int) *Queue[T] {
    return &Queue[T]{data: make([]T, cap)}
}

func (q *Queue[T]) Enqueue(v T) bool {
    if q.size == len(q.data) {
        return false // full
    }
    q.data[q.tail] = v
    q.tail = (q.tail + 1) % len(q.data)
    q.size++
    return true
}

func (q *Queue[T]) Dequeue() (T, bool) {
    var zero T
    if q.size == 0 {
        return zero, false
    }
    val := q.data[q.head]
    q.head = (q.head + 1) % len(q.data)
    q.size--
    return val, true
}
```

### Rust

```rust
use std::collections::VecDeque;

// VecDeque is a circular buffer backed by a Vec
// Efficient push/pop at both ends: O(1) amortized
let mut q: VecDeque = VecDeque::new();
q.push_back(1);
q.push_back(2);
q.push_back(3);
q.pop_front();  // 1
q.push_front(0); // push to front too!

// For priority queue, use BinaryHeap
use std::collections::BinaryHeap;
use std::cmp::Reverse;

let mut max_heap: BinaryHeap = BinaryHeap::new();
max_heap.push(5);
max_heap.push(1);
max_heap.push(3);
println!("{}", max_heap.peek().unwrap()); // 5

// Min-heap using Reverse wrapper:
let mut min_heap: BinaryHeap<Reverse> = BinaryHeap::new();
min_heap.push(Reverse(5));
min_heap.push(Reverse(1));
println!("{}", min_heap.peek().unwrap().0); // 1
```

---

## 38. Linux Kernel kfifo

The kernel's `kfifo` (`include/linux/kfifo.h`) is a highly optimized, lock-free (for single producer/consumer) circular buffer.

```c
// kfifo internal structure:
struct __kfifo {
    unsigned int    in;     // write pointer (absolute, never wrapped)
    unsigned int    out;    // read pointer (absolute, never wrapped)
    unsigned int    mask;   // size - 1 (size is always power of 2)
    unsigned int    esize;  // element size
    void            *data;  // the buffer
};

// Why size must be power of 2:
//   index = position % size
//   If size = power of 2: index = position & mask (ONE fast bitwise AND)
//   If size = arbitrary:  index = position % size (DIVISION — much slower)

// Why 'in' and 'out' are NEVER wrapped (they keep incrementing):
//   This avoids a race condition in lock-free SPSC (Single Producer Single Consumer):
//   If we used wrapped indices, we'd need atomic read-modify-write to wrap.
//   With absolute indices: producer only writes 'in', consumer only reads 'in'.
//   Consumer only writes 'out', producer only reads 'out'.
//   On x86: reading/writing aligned 32-bit integers is atomic.
//   So no lock needed for SPSC!
//
//   len = in - out  (works even after overflow of unsigned int because
//                    unsigned subtraction wraps around correctly in C)
//   actual_index = (in or out) & mask  <- reduces to 0..size-1

// Usage example (kernel style):
DECLARE_KFIFO(myfifo, unsigned char, 1024); // declares kfifo of 1024 bytes
INIT_KFIFO(myfifo);

// In interrupt handler (producer):
kfifo_in(&myfifo, data, len); // enqueue

// In process context (consumer):
kfifo_out(&myfifo, buf, len); // dequeue

// kfifo is used by:
//   - Serial/UART drivers (buffering bytes from hardware interrupts)
//   - USB HID drivers
//   - Input event queuing
//   - Many network drivers
```

---

# Part VIII: Hash Tables

## 39. Hash Functions — What Makes a Good One

A hash function maps a key of arbitrary size to a fixed-size integer (the hash). The hash determines which bucket the key-value pair goes into.

```
Good hash function properties:
  1. DETERMINISTIC: same key always produces same hash
  2. UNIFORM: distributes keys evenly across all buckets
  3. FAST: O(1) computation
  4. AVALANCHE: small change in key -> large change in hash (for security)

Simple (bad) hash for strings: sum of ASCII values
  hash("abc") = 97 + 98 + 99 = 294
  hash("bca") = 98 + 99 + 97 = 294  <- COLLISION! same hash for anagram
  hash("cab") = 99 + 97 + 98 = 294  <- all anagrams collide
  Very poor distribution!

Polynomial rolling hash (better):
  hash(s) = s[0]*p^(n-1) + s[1]*p^(n-2) + ... + s[n-1]*p^0  (mod M)
  Common: p=31, M=10^9+7

djb2 (Dan Bernstein, popular in practice):
  hash = 5381
  for each char c: hash = hash * 33 ^ c  (or: hash = ((hash << 5) + hash) ^ c)

FNV-1a (Fowler-Noll-Vo): Fast, good distribution
  hash = FNV_OFFSET_BASIS
  for each byte b: hash = hash ^ b; hash = hash * FNV_PRIME

xxHash, MurmurHash3, SipHash: Modern, very fast, excellent distribution
  SipHash (used by Rust HashMap, Python 3.4+): designed to resist
  hash-flooding attacks where an attacker crafts inputs to cause many collisions.

CRC32, MD5, SHA-*: Cryptographic or CRC hashes — too slow for hash tables.
```

---

## 40. Separate Chaining

```
HASH TABLE WITH SEPARATE CHAINING

Size = 7, hash(k) = k % 7

Keys inserted: 10, 23, 17, 5, 8, 30, 13

  10 % 7 = 3
  23 % 7 = 2
  17 % 7 = 3  (collision with 10!)
   5 % 7 = 5
   8 % 7 = 1
  30 % 7 = 2  (collision with 23!)
  13 % 7 = 6

Buckets (each is a linked list of entries):

[0]: -> NULL
[1]: -> [8 | *] -> NULL
[2]: -> [23 | *] -> [30 | *] -> NULL
[3]: -> [10 | *] -> [17 | *] -> NULL
[4]: -> NULL
[5]: -> [5 | *] -> NULL
[6]: -> [13 | *] -> NULL

LOOKUP(17):
  1. h = 17 % 7 = 3
  2. Traverse bucket[3]: check 10 (not 17), check 17 (found!)
  O(1 + chain_length)

MEMORY LAYOUT:
  Array of pointers (one per bucket): 7 * 8 bytes = 56 bytes
  Plus heap nodes for each entry.
  Nodes are scattered -> cache unfriendly for long chains.

LOAD FACTOR α = n / m  (n=keys stored, m=buckets)
  α < 0.7: most lookups are O(1)
  α = 1.0: average chain length = 1
  α = 2.0: average chain length = 2 (each lookup checks ~2 nodes on average)
```

---

## 41. Open Addressing

Instead of linked lists, open addressing stores all elements in the array itself.

```
OPEN ADDRESSING — Linear Probing

Size = 11, hash(k) = k % 11, probe = (hash + i) % 11

Insert: 20, 30, 2, 13, 25

  20 % 11 = 9  -> slot 9 empty, store 20
  30 % 11 = 8  -> slot 8 empty, store 30
   2 % 11 = 2  -> slot 2 empty, store 2
  13 % 11 = 2  -> slot 2 taken (2)! probe slot 3 -> empty, store 13
  25 % 11 = 3  -> slot 3 taken (13)! probe 4 -> empty, store 25

Table:
  [0][ ][1][ ][2][2][3][13][4][25][5][ ][6][ ][7][ ][8][30][9][20][10][ ]

CLUSTERING PROBLEM with linear probing:
  When many keys hash nearby, they form "clusters".
  Long clusters -> long probe sequences -> O(n) worst case.
  
  Primary clustering: consecutive occupied slots.
  Load factor must stay below 0.7 for good performance.

QUADRATIC PROBING: probe = (hash + i*i) % size
  Better spread than linear, less clustering.
  May fail to probe all slots if size is not prime.

DOUBLE HASHING: probe = (hash1 + i * hash2) % size
  Best distribution, eliminates clustering.
  But: 2 hash computations per probe.
  
DELETION PROBLEM with open addressing:
  Cannot simply remove an element — it would break probe chains!
  
  Table: [_, _, 2, 13, 25, ...]
  Delete 2 (slot 2): if we zero slot 2, lookup 13 would:
    hash(13)=2 -> slot 2 is empty -> INCORRECTLY conclude 13 not found!
    
  Solution: "tombstone" markers — mark slot as deleted, not empty.
    Probe stops at EMPTY but NOT at tombstone.
    Tombstone slots can be reused by subsequent insertions.
    Danger: too many tombstones -> rebuild table.
```

---

## 42. Robin Hood Hashing

Robin Hood hashing is an open addressing scheme that minimizes the maximum probe length.

```
ROBIN HOOD HASHING

Invariant: if two elements are contending for a slot, the one that is
           "richer" (closer to its ideal slot) gives way to the "poorer" one.
           This minimizes the MAXIMUM probe distance.

"Probe length" = actual_slot - ideal_slot

Example:
  Size = 8
  A hashes to slot 2, probe length = 0 (at ideal slot)
  B hashes to slot 2, placed at slot 3, probe length = 1
  C hashes to slot 3, placed at... slot 3 is taken by B (probe=1)
    Slot 4 is free, but C would have probe=1, same as B.
    By Robin Hood: place C at slot 4 (probe=1), that's fine.
    But if D hashes to slot 0 and probe length becomes 5 (placed at slot 5),
    and E hashes to slot 5 (probe=0), then:
      E wants slot 5, E has probe=0, D in slot 5 has probe=5
      D is "poorer" (farther from home), so D "steals" from E:
      Actually Robin Hood is: the INCOMING element "steals" from elements
      with shorter probe distance.
      
    When inserting element with probe_len P at slot S:
      if probe_len[S] < P:
        swap incoming with element at S
        continue inserting the displaced element

Benefits:
  - Very uniform probe lengths (~log log n average, tiny max)
  - Better cache performance vs chaining
  - Rust's old HashMap used this
  
Rust's current HashMap (since 2020) uses Swiss Table instead (see below).
```

---

## 43. Load Factor and Rehashing

```
LOAD FACTOR CONTROL

α = number_of_elements / table_capacity

As α increases:
  - Separate chaining: chain length grows, lookup time degrades linearly
  - Open addressing: probe length increases rapidly (especially > 0.7)
  
Typical thresholds:
  Separate chaining: rehash when α > 1.0 or α > 2.0
  Open addressing:   rehash when α > 0.7 (Go) or α > 0.875 (Go's 7/8ths threshold)
  Robin Hood:        rehash when α > 0.9 (probe lengths stay manageable)
  Swiss Table (Rust):rehash when α > 0.875

REHASHING PROCESS:
  1. Allocate new table of size typically 2x current
  2. For each element in old table:
     a. Compute new hash with new table size
     b. Insert into new table (may probe new positions)
  3. Free old table
  
  Cost: O(n) time, O(n) extra space temporarily
  Amortized O(1) per insertion (same argument as dynamic array)
  
SHRINK ON DELETE: Some implementations also shrink when α drops below 0.25
  to reclaim memory. Python dicts shrink; C++ unordered_map does not.
```

---

## 44. C Hash Table

```c
#include 
#include 
#include 

#define INITIAL_CAP 16
#define MAX_LOAD    0.75

typedef struct Entry {
    char        *key;
    int          value;
    struct Entry *next;  // for chaining
} Entry;

typedef struct {
    Entry   **buckets;
    size_t    count;
    size_t    capacity;
} HashMap;

// FNV-1a hash
static uint32_t hash_fnv1a(const char *key) {
    uint32_t h = 2166136261u;
    while (*key) {
        h ^= (unsigned char)*key++;
        h *= 16777619u;
    }
    return h;
}

HashMap* hmap_new(void) {
    HashMap *m = malloc(sizeof(HashMap));
    m->capacity = INITIAL_CAP;
    m->count = 0;
    m->buckets = calloc(m->capacity, sizeof(Entry *)); // zeroed = NULL
    return m;
}

void hmap_put(HashMap *m, const char *key, int value) {
    // Rehash if load factor exceeded
    if ((double)m->count / m->capacity >= MAX_LOAD) {
        size_t new_cap = m->capacity * 2;
        Entry **new_buckets = calloc(new_cap, sizeof(Entry *));
        // Rehash all existing entries
        for (size_t i = 0; i < m->capacity; i++) {
            for (Entry *e = m->buckets[i]; e;) {
                Entry *next = e->next;
                size_t idx = hash_fnv1a(e->key) % new_cap;
                e->next = new_buckets[idx];
                new_buckets[idx] = e;
                e = next;
            }
        }
        free(m->buckets);
        m->buckets = new_buckets;
        m->capacity = new_cap;
    }
    
    size_t idx = hash_fnv1a(key) % m->capacity;
    // Check if key exists
    for (Entry *e = m->buckets[idx]; e; e = e->next) {
        if (strcmp(e->key, key) == 0) {
            e->value = value; // update
            return;
        }
    }
    // New entry
    Entry *e = malloc(sizeof(Entry));
    e->key = strdup(key);
    e->value = value;
    e->next = m->buckets[idx];
    m->buckets[idx] = e;
    m->count++;
}

int* hmap_get(HashMap *m, const char *key) {
    size_t idx = hash_fnv1a(key) % m->capacity;
    for (Entry *e = m->buckets[idx]; e; e = e->next)
        if (strcmp(e->key, key) == 0) return &e->value;
    return NULL; // not found
}
```

---

## 45. Go map — The Real Internals

```
GO MAP INTERNALS (runtime/map.go)

A Go map is a hash table with the following key characteristics:

hmap struct:
  +----------+----------+----------+----------+----------+----------+
  | count    | flags    | B        | noverflow| hash0    | buckets  |
  | (int)    | (uint8)  | (uint8)  | (uint16) | (uint32) | (ptr)    |
  +----------+----------+----------+----------+----------+----------+
  +----------+----------+----------+
  |oldbuckets| nevacuate| extra    |
  | (ptr)    | (uintptr)| (ptr)    |
  +----------+----------+----------+

  count: number of key-value pairs
  B: log2 of number of buckets (num_buckets = 2^B)
  hash0: hash seed (randomized per map instance for hash-flooding protection)
  buckets: pointer to array of 2^B bmap buckets

bmap (bucket) struct — each bucket holds 8 key-value pairs:
  +--------+--------+--------+--------+--------+--------+--------+--------+
  | tophash| tophash| tophash| tophash| tophash| tophash| tophash| tophash|  <- 8 bytes
  +--------+--------+--------+--------+--------+--------+--------+--------+
  |  key0  |  key1  |  key2  |  key3  |  key4  |  key5  |  key6  |  key7  |  <- 8 * keysize
  +--------+--------+--------+--------+--------+--------+--------+--------+
  | value0 | value1 | value2 | value3 | value4 | value5 | value6 | value7 |  <- 8 * valsize
  +--------+--------+--------+--------+--------+--------+--------+--------+
  | overflow pointer (to next bmap if this bucket is full)                |  <- 8 bytes
  +-----------------------------------------------------------------------+

tophash: top 8 bits of the hash value for each key in this bucket
  - Allows fast rejection: compare tophash before comparing full key
  - Special values: 0=empty, 1=evacuated empty, 2=evacuated, 5-255=valid

LOOKUP(key):
  1. hash = hashfunc(key, hash0)         O(1)
  2. bucket_idx = hash & (2^B - 1)       O(1) — low bits of hash
  3. top = uint8(hash >> (64-8))         O(1) — high 8 bits
  4. for each slot in bucket[bucket_idx]:
       if slot.tophash == top:           O(1) per slot — fast rejection
           if slot.key == key: return val O(k) where k = key size
  5. follow overflow pointer if present
  
INCREMENTAL REHASHING:
  When the map grows, it doesn't rehash all at once (would cause latency spike).
  Instead:
  - oldbuckets = current buckets
  - buckets = new (double-size) array
  - On each access (read or write), evacuate 1-2 old buckets to new
  - After all buckets evacuated, oldbuckets is freed
  
  This amortizes the rehash cost across operations — no single O(n) pause.
```

```go
// Go map gotchas:
func mapGotchas() {
    m := map[string]int{}
    
    // Zero value on missing key (no error):
    val := m["missing"] // val = 0 (zero value for int), no panic
    
    // Two-value idiom to distinguish missing vs zero-valued:
    val, ok := m["missing"]
    if !ok { // ok = false -> key not present
        fmt.Println("not found")
    }
    
    // Maps are NOT safe for concurrent use:
    // Concurrent read+write -> runtime panic: "concurrent map read and write"
    // Use sync.RWMutex or sync.Map for concurrent access.
    
    // Iteration order is RANDOMIZED intentionally (since Go 1.0):
    // "for k, v := range m" gives random order every time.
    // This prevents code from depending on implementation details.
    
    // Map cannot be compared with == (only with nil):
    // var m1, m2 map[string]int
    // m1 == m2 // COMPILE ERROR
    
    // Struct keys must be comparable (no slices, maps, or functions as keys):
    type Key struct{ x, y int }
    coords := map[Key]string{{1, 2}: "A"}
    
    // Maps store pointers internally — the backing bucket array is on the heap.
    // A nil map behaves like an empty map for reads, panics on writes.
    var nilMap map[string]int
    _ = nilMap["key"]  // fine: returns zero value
    // nilMap["key"] = 1  // PANIC: assignment to entry in nil map
}
```

---

## 46. Rust HashMap — SwissTable / hashbrown

```
RUST HASHMAP (using SwissTable / hashbrown since Rust 1.36)

SwissTable uses SIMD instructions for ultra-fast multi-slot probing.

Internal layout:
  - Open addressing with quadratic probing + Robin Hood displacement
  - Uses 1 control byte per slot (stored separately from key-value data)
  
Control bytes array (packed):
  +----+----+----+----+----+----+----+----+  ...  +----+----+
  | c0 | c1 | c2 | c3 | c4 | c5 | c6 | c7 |  ...  | cN |cN+1|
  +----+----+----+----+----+----+----+----+  ...  +----+----+
  Each control byte:
    0b00000000 (0x00) = EMPTY
    0b11111110 (0xFE) = DELETED (tombstone)
    0b0xxxxxxx = FULL, x = low 7 bits of hash
    
  Key-value data stored in a separate contiguous array.

LOOKUP using SIMD (x86 SSE2):
  1. Compute h = hash(key)
  2. target = h2 = low 7 bits of h    <- the control byte we're looking for
  3. slot = h1 = remaining bits       <- starting slot
  4. Load 16 control bytes at once using _mm_loadu_si128
  5. Compare all 16 against target using _mm_cmpeq_epi8
  6. This produces a bitmask of matching slots — check keys at those slots
  7. If none match and any EMPTY in the group: key not present
  8. Otherwise: advance by 16 and repeat

This allows checking 16 slots in ~2 CPU instructions instead of 16!
Modern x86 can check 32 slots at once with AVX2.

Result: HashMap lookup is ~1.5-3x faster than traditional Robin Hood.
```

```rust
use std::collections::HashMap;

fn rust_hashmap_usage() {
    // Default: SipHash 1-3 (cryptographic, DOS resistant but slower)
    let mut map: HashMap = HashMap::new();
    
    // For performance-critical code without security concerns, use FxHashMap:
    // (from rustc-hash crate)
    // let mut map: FxHashMap = FxHashMap::default();
    
    map.insert("one".to_string(), 1);
    map.insert("two".to_string(), 2);
    
    // Entry API — most efficient for insert-or-update:
    map.entry("three".to_string()).or_insert(3);
    *map.entry("one".to_string()).or_insert(0) += 10; // increment or insert 0
    
    // Lookup:
    if let Some(v) = map.get("one") {
        println!("{}", v); // immutable reference
    }
    
    // BTreeMap: sorted hash map, O(log n) operations
    // Use when you need ordered iteration or range queries
    use std::collections::BTreeMap;
    let mut btree: BTreeMap = BTreeMap::new();
    btree.insert("a".to_string(), 1);
    btree.insert("c".to_string(), 3);
    btree.insert("b".to_string(), 2);
    for (k, v) in &btree { // ALWAYS in sorted order
        println!("{}: {}", k, v); // a: 1, b: 2, c: 3
    }
}
```

---

## 47. Linux Kernel Hash Tables

```c
// Linux kernel uses hlist_head/hlist_node for hash table buckets.
// WHY hlist instead of list_head?
// list_head uses circular doubly linked list.
// For hash table buckets: most buckets are EMPTY or have just 1-2 elements.
// A circular list requires the head itself to be a list_head (16 bytes).
// hlist_head only needs ONE pointer (8 bytes), halving bucket array size.

struct hlist_head {
    struct hlist_node *first;  // 8 bytes — just one pointer
};

struct hlist_node {
    struct hlist_node *next;
    struct hlist_node **pprev;  // pointer to the pointer that points to this node
    // Why **pprev instead of *prev?
    // The head uses a single 'first' pointer, not a full hlist_node.
    // pprev allows uniform deletion: *pprev = next
    // Works whether pprev points to head->first or node->next
};

// Example: PID hash table in Linux kernel
// In kernel/pid.c:
// #define PIDHASH_SZ 4096
// static struct hlist_head pid_hash[PIDHASH_SZ];
//
// Each pid_namespace has a hash table mapping pid numbers to task_structs.

// hlist operations:
// hlist_add_head(node, head): insert node at head of list
// hlist_del(node): remove node from list
// hlist_for_each_entry(pos, head, member): iterate
// hlist_for_each_entry_safe(pos, n, head, member): iterate with deletion

// Example usage (simplified from kernel):
struct pid_struct {
    unsigned int         nr;       // the PID number
    struct hlist_node    pid_chain; // embedded in hash table
    struct task_struct  *task;
};

// Lookup by PID:
// idx = pid_hashfn(pid_nr);  // hash the PID number
// hlist_for_each_entry(p, &pid_hash[idx], pid_chain) {
//     if (p->nr == pid_nr) return p->task;
// }
```

---

# Part IX: Trees

## 48. Binary Tree — Anatomy

```
BINARY TREE NODE

+----------+----------+----------+
|  left*   |   data   |  right*  |
|  (8B)    |  (4B+4B) |  (8B)    |
+----------+----------+----------+
Total: 24 bytes per node (with padding)

A tree:
         A
        / \
       B   C
      / \   \
     D   E   F

In memory (each node is a separate heap allocation):
  node_A: {left=&node_B, data=A, right=&node_C}  @ 0x1000
  node_B: {left=&node_D, data=B, right=&node_E}  @ 0x2000
  node_C: {left=NULL,    data=C, right=&node_F}  @ 0x3000
  node_D: {left=NULL,    data=D, right=NULL}      @ 0x4000
  node_E: {left=NULL,    data=E, right=NULL}      @ 0x5000
  node_F: {left=NULL,    data=F, right=NULL}      @ 0x6000

No spatial locality! Each pointer dereference is a potential cache miss.
Deep trees (height h) require O(h) pointer dereferences for lookup.

TRAVERSAL ORDERS:
  In-order   (L, root, R): D, B, E, A, C, F  <- sorted order for BST!
  Pre-order  (root, L, R): A, B, D, E, C, F  <- reconstruct tree structure
  Post-order (L, R, root): D, E, B, F, C, A  <- delete tree safely
  Level-order (BFS):        A, B, C, D, E, F  <- useful for printing/serializing
  
PROPERTIES:
  Height h = max depth of any leaf from root
  Perfect binary tree: all levels full, n = 2^(h+1) - 1 nodes, h = log2(n+1)-1
  Complete binary tree: all levels except last full, last level left-filled
  Degenerate (skewed) tree: each node has one child -> effectively a linked list!
```

---

## 49. Binary Search Tree (BST)

```
BST PROPERTY: For every node N:
  All values in N's left subtree < N's value
  All values in N's right subtree > N's value

Building a BST from: 8, 3, 10, 1, 6, 14, 4, 7, 13

Insert 8:          Insert 3:          Insert 10:
    8                  8                   8
                      / \                 / \
                     3              ->   3   10
                     
After all inserts:
          8
         / \
        3   10
       / \    \
      1   6    14
         / \   /
        4   7 13

SEARCH(13):
  8: 13 > 8, go right
  10: 13 > 10, go right
  14: 13 < 14, go left
  13: found! 4 comparisons, O(log n) for balanced tree

INSERT(5):
  8: 5 < 8, go left
  3: 5 > 3, go right
  6: 5 < 6, go left
  4: 5 > 4, go right
  NULL: insert here
  -> 3 comparisons

DELETE(6) — three cases:
  Case 1: node has no children (leaf) — just remove
  Case 2: node has one child — replace node with its child
  Case 3: node has two children (like 6 which has 4 and 7):
    Find in-order successor (smallest in right subtree) = 7
    OR in-order predecessor (largest in left subtree) = 5 (or 4)
    Copy successor's value to node (6->7)
    Delete the successor (which has at most 1 child, easier case)

BST WORST CASE:
  Insert sorted data: 1, 2, 3, 4, 5 (all go right, degenerate to linked list)
          1
           \
            2
             \
              3
               \
                4
                 \
                  5
  
  Height = n, all operations O(n). Solution: self-balancing trees.
```

---

## 50. AVL Tree — Self-balancing

```
AVL TREE
  Named after Adelson-Velsky and Landis (1962).
  INVARIANT: For every node, |height(left) - height(right)| <= 1
  
  Height difference = "balance factor" = height(right) - height(left)
  Valid balance factors: -1, 0, +1
  
  Guarantees: height <= 1.44 * log2(n+2) - 0.328
              So search/insert/delete are always O(log n)

When balance factor becomes -2 or +2: ROTATION needed.

4 ROTATION CASES:

1. LEFT-LEFT (LL): inserted into left child's left subtree
         z                 y
        /                 / \
       y      ->         x   z
      /
     x
   rotate_right(z)

2. RIGHT-RIGHT (RR): mirror of LL
   rotate_left(z)

3. LEFT-RIGHT (LR): inserted into left child's right subtree
         z              z             x
        /              /             / \
       y     ->       x     ->      y   z
        \            /
         x          y
   rotate_left(y), then rotate_right(z)

4. RIGHT-LEFT (RL): mirror of LR

ROTATIONS are O(1) — just pointer reassignments.
But must update balance factors/heights up the path: O(log n).

Each node stores height (or just balance factor):
  struct AVLNode {
      int data;
      int height;  // 4 extra bytes per node
      AVLNode *left, *right;
  };
  
PERFORMANCE vs BST:
  AVL trees are MORE strictly balanced than Red-Black trees.
  Faster lookups (shorter height).
  More rotations on insert/delete (slower writes).
  
Use AVL when: lookup-heavy workloads (more reads than writes).
Use Red-Black when: balanced read/write or write-heavy workloads.
```

---

## 51. Red-Black Tree — The Industry Standard

```
RED-BLACK TREE (RBT) — 5 Properties:

1. Every node is RED or BLACK.
2. The root is BLACK.
3. Every leaf (NIL sentinel) is BLACK.
4. If a node is RED, both its children are BLACK. (No consecutive reds)
5. All paths from any node to descendant NIL nodes contain the same number
   of BLACK nodes (black-height is uniform).

These 5 rules guarantee: height <= 2 * log2(n+1)
So all operations are O(log n) guaranteed.

Example:
             10(B)
            /     \
          5(R)    20(R)
         / \      / \
       3(B) 7(B)15(B) 25(B)
       / \ / \  / \   / \
      N  N N N N  N  N   N  <- NIL (all BLACK)

INSERT(6):
  Normal BST insert: 6 goes as left child of 7 (red node)
  New node starts RED. Parent 7 is BLACK -> no violation! Done.
  
INSERT(4):
  BST: 4 goes as left child of 5 (red parent) -> RED-RED violation!
  Uncle of 4 is 7(B). CASE: uncle is BLACK.
  -> Rotation: right-rotate at 10, recolor
  
  Before:      After rotate + recolor:
      10(B)           5(B)
      /               / \
    5(R)    ->     3(R)  10(R)
    /                    /
  3(R)                  7(B)
  
Red-Black rotations are similar to AVL but with recoloring.
At most 2 rotations per insert (3 for AVL), O(1) rotations.
Maximum O(log n) recolorings going up the tree.

COMPARED TO AVL:
  AVL: max height 1.44 log n, faster lookup
  RBT: max height 2.0 log n,  faster insert/delete (fewer rotations)
  RBT wins in practice for mixed workloads (why STL, Linux use it)
  
NODE STRUCTURE:
  struct RBNode {
      int            key;
      int            value;
      enum           { RED, BLACK } color;
      struct RBNode *left, *right, *parent;
  };
  
WHERE RBT IS USED:
  - C++ std::map, std::set (RBT)
  - Java TreeMap, TreeSet (RBT)
  - Python's sortedcontainers (AVL)
  - Linux CFS scheduler (rb_root)
  - Linux virtual memory areas (vm_area_struct)
  - Linux epoll (event polling)
  - Nginx timer management
```

---

## 52. B-Tree and B+ Tree

```
B-TREE — designed for disk I/O (databases, file systems)

Motivation: Binary trees have O(log2 n) levels. Each level = disk seek (expensive!).
B-trees have HIGH FANOUT: each node can have hundreds of children.
Result: O(log_t n) levels where t = min degree. For t=512: O(log_512 n) levels.

B-TREE ORDER t (min degree):
  - Each non-root internal node has at least t-1 keys and t children.
  - Each node has at most 2t-1 keys and 2t children.
  - All leaves are at the same depth.

B-TREE WITH t=3 (each node: min 2 keys, max 5 keys):

         [10 | 20 | 30]           <- root: 3 keys, 4 children
        /    |    |    \
  [3|7] [15|17] [22|25] [35|40]  <- leaf nodes

Each node typically fits in ONE disk block (4KB or 16KB).
Reading a node = one disk seek.

For 1 BILLION records:
  Binary tree: log2(10^9) ≈ 30 levels = 30 disk seeks
  B-tree t=512: log_512(10^9) ≈ 3 levels = 3 disk seeks!

B+ TREE (most databases use this, not B-tree):
  - ALL data stored in LEAF nodes only
  - Internal nodes contain only KEYS (routing keys, not actual data)
  - Leaf nodes are linked together (doubly linked list!)
  
         [10 | 20]               <- internal node: keys only
        /    |    \
  [3|7|9] [10|15|17] [20|25|30] <- leaves: actual (key,value) pairs
      |->         |->         |->|  <- linked list of leaves!
  
Benefits of B+ over B:
  - Range queries: just scan the linked leaf list! No traversal back up.
    "Give me all records where 10 <= key <= 25": find first leaf, scan right.
  - Internal nodes hold more keys (no data = smaller nodes = higher fanout)
  - More cache-friendly for sequential scans
  
Used by: PostgreSQL, MySQL InnoDB, SQLite, Oracle, MongoDB (WiredTiger),
         Linux ext4 (htree directory indexing), APFS, NTFS, HFS+.
```

---

## 53. Trie — Prefix Tree

```
TRIE (Prefix Tree / Digital Search Tree)

Each edge represents a CHARACTER, not a key.
Keys are built along root-to-leaf paths.

Inserting: "cat", "car", "card", "care", "bat", "bar"

          [root]
         /      \
        c         b
        |         |
        a         a
       / \       / \
      t   r     t   r
          |
         [d,e]
         d   e

Marking word endings with '*':
          [root]
         /      \
        c         b
        |         |
        a         a
       / \       / \
      t* r      t*  r*
          |
         d*  e*

LOOKUP "care":
  root -> c (1 hop) -> a (2 hops) -> r (3 hops) -> e (4 hops) -> found! O(4)
  
  In general: O(k) where k = key length (number of characters).
  This is INDEPENDENT of n (number of keys stored)!
  
  Compare to BST: O(k log n) — each comparison is O(k) AND there are O(log n) comparisons.
  Trie wins for string lookups when k < log n.

MEMORY LAYOUT (array-based trie node):
  struct TrieNode {
      TrieNode *children[26];  // one pointer per letter, 26 * 8 = 208 bytes!
      bool is_end;
  };
  
  Wasteful if alphabet large or keys sparse.
  Alternative: HashMap children (denser, but slower access)
  
COMPRESSED TRIE (Radix Tree / Patricia Tree):
  Merge single-child chains into one edge:
  Instead of c -> a -> t, store one edge "cat"
  Saves memory, same O(k) lookup.
  Linux kernel radix tree / XArray uses a variant of this.

USE CASES:
  - Autocomplete / search suggestions
  - Spell checkers
  - IP routing tables (each bit of IP address = one level)
  - Dictionary implementations
  - Longest prefix matching
```

---

## 54. Segment Tree

```
SEGMENT TREE — for range queries on arrays

Problem: Given array A[0..n-1], support:
  - Range query: compute something (sum, min, max) over A[l..r]
  - Point update: change A[i] to new value
  Both in O(log n) time.

Array A = [1, 3, -2, 8, -7]

Segment tree stores precomputed sums for all intervals:
  Each node covers a segment [l, r] and stores sum(A[l..r]).

                    [3]          <- node 0: sum[0..4] = 3
                 /       \
             [2]           [1]   <- sum[0..2]=2, sum[3..4]=1
            /   \         /   \
          [4]   [-2]   [8]   [-7] <- sum[0..1]=4, [2..2]=-2, [3..3]=8, [4..4]=-7
         /   \
        [1]  [3]                <- sum[0..0]=1, [1..1]=3

STORED AS ARRAY (like heap, root at index 1):
  Index:  [1]  [2]  [3]  [4]  [5]  [6]  [7]  [8]  [9]
  Value:  [ 3] [ 2] [ 1] [ 4] [-2] [ 8] [-7] [ 1] [ 3]
  
  Left child of i  = 2*i
  Right child of i = 2*i + 1
  Parent of i      = i / 2
  
  Array needs 4*n entries to be safe.

QUERY sum[1..3]:
  Query [1..3] on tree rooted at [0..4]:
    [0..4] not fully in [1..3], split:
      Left child [0..2]: overlaps [1..3], split again:
        [0..1]: overlaps [1..3], split:
          [0..0]: outside [1..3], skip
          [1..1]: fully inside [1..3], return 3
        [2..2]: fully inside [1..3], return -2
      Right child [3..4]: overlaps [1..3], split:
        [3..3]: fully inside [1..3], return 8
        [4..4]: outside [1..3], skip
  Result: 3 + (-2) + 8 = 9 ✓

O(log n) per query: at most 4*log n nodes visited.
O(log n) per update: update O(log n) ancestors.

LAZY PROPAGATION: for range updates (add 5 to all elements in [l..r]):
  Instead of updating all affected leaves (O(n) worst case),
  store a "lazy tag" at the highest covering node.
  Propagate lazily only when children need to be accessed.
  Both update and query remain O(log n).
```

---

## 55. Fenwick Tree — Binary Indexed Tree

```
FENWICK TREE (Binary Indexed Tree, BIT)

Supports:
  - Prefix sum query: sum(A[1..i]) in O(log n)
  - Point update: A[i] += delta in O(log n)
  
Simpler to implement than segment tree. Less memory (just n+1 array).

STRUCTURE: Each index i is "responsible" for a range of elements.
  The range length for index i = lowest set bit of i (= i & -i)
  
  i & (-i) extracts the lowest set bit:
  i=1  (0001): 1 & (-1) = 1 & (1111) = 0001 -> responsible for 1 element
  i=2  (0010): 2 & (-2) = 2 & (1110) = 0010 -> responsible for 2 elements
  i=4  (0100): 4 & (-4) = 4 & (1100) = 0100 -> responsible for 4 elements
  i=6  (0110): 6 & (-6) = 6 & (1010) = 0010 -> responsible for 2 elements
  i=8  (1000): 8 & (-8) = 8 & (1000) = 1000 -> responsible for 8 elements

BIT Array (1-indexed), original: A = [_, 1, 3, 2, 8, 5, 4, 6, 7]

  BIT[1] = A[1]         = 1    (covers [1..1])
  BIT[2] = A[1]+A[2]    = 4    (covers [1..2])
  BIT[3] = A[3]         = 2    (covers [3..3])
  BIT[4] = A[1]+...+A[4]= 14   (covers [1..4])
  BIT[5] = A[5]         = 5    (covers [5..5])
  BIT[6] = A[5]+A[6]   = 9    (covers [5..6])
  BIT[7] = A[7]         = 6    (covers [7..7])
  BIT[8] = A[1]+...+A[8]= 36   (covers [1..8])

PREFIX SUM query(7) = sum(A[1..7]):
  i=7 (0111): bit[7] = 6, i -= i&(-i) -> i = 6
  i=6 (0110): bit[6] = 9, i -= i&(-i) -> i = 4
  i=4 (0100): bit[4] = 14, i -= i&(-i) -> i = 0, stop
  Result: 6 + 9 + 14 = 29  ✓ (1+3+2+8+5+4+6=29)
  Only 3 iterations for n=8! = log2(8)

UPDATE A[3] += 5:
  i=3 (0011): bit[3] += 5, i += i&(-i) -> i = 4
  i=4 (0100): bit[4] += 5, i += i&(-i) -> i = 8
  i=8 (1000): bit[8] += 5, i += i&(-i) -> i = 16, stop
  Also 3 iterations!
```

---

## 56. Tree Implementations in C, Go, Rust

### C — Binary Search Tree

```c
typedef struct BSTNode {
    int key;
    struct BSTNode *left, *right;
} BSTNode;

BSTNode* bst_insert(BSTNode *root, int key) {
    if (!root) {
        BSTNode *n = malloc(sizeof(BSTNode));
        n->key = key; n->left = n->right = NULL;
        return n;
    }
    if (key < root->key) root->left  = bst_insert(root->left,  key);
    else if (key > root->key) root->right = bst_insert(root->right, key);
    return root; // duplicate: return unchanged
}

BSTNode* bst_search(BSTNode *root, int key) {
    while (root && root->key != key)
        root = (key < root->key) ? root->left : root->right;
    return root;
}

// In-order traversal: visits nodes in sorted order
void bst_inorder(BSTNode *root, void (*visit)(int)) {
    if (!root) return;
    bst_inorder(root->left, visit);
    visit(root->key);
    bst_inorder(root->right, visit);
}

// Free entire tree
void bst_free(BSTNode *root) {
    if (!root) return;
    bst_free(root->left);
    bst_free(root->right);
    free(root); // post-order: free children before parent
}
```

### Go — BST with Generics

```go
type BST[T constraints.Ordered] struct {
    key   T
    left  *BST[T]
    right *BST[T]
}

func (t *BST[T]) Insert(key T) *BST[T] {
    if t == nil {
        return &BST[T]{key: key}
    }
    switch {
    case key < t.key:
        t.left = t.left.Insert(key)
    case key > t.key:
        t.right = t.right.Insert(key)
    }
    return t
}

func (t *BST[T]) Contains(key T) bool {
    for t != nil {
        switch {
        case key < t.key:
            t = t.left
        case key > t.key:
            t = t.right
        default:
            return true
        }
    }
    return false
}
```

### Rust — BST with Box (Ownership)

```rust
// Rust's ownership makes tree implementation instructive:
// Option<Box<Node>> = either no node, or an exclusively owned heap node

#[derive(Debug)]
struct BST {
    root: Option<Box<Node>>,
}

#[derive(Debug)]
struct Node {
    key: T,
    left: Option<Box<Node>>,
    right: Option<Box<Node>>,
}

impl BST {
    fn new() -> Self { BST { root: None } }
    
    fn insert(&mut self, key: T) {
        fn insert_node(node: &mut Option<Box<Node>>, key: T) {
            match node {
                None => *node = Some(Box::new(Node { key, left: None, right: None })),
                Some(n) => {
                    if key < n.key { insert_node(&mut n.left, key); }
                    else if key > n.key { insert_node(&mut n.right, key); }
                }
            }
        }
        insert_node(&mut self.root, key);
    }
    
    fn contains(&self, key: &T) -> bool {
        let mut node = &self.root;
        while let Some(n) = node {
            match key.cmp(&n.key) {
                std::cmp::Ordering::Equal   => return true,
                std::cmp::Ordering::Less    => node = &n.left,
                std::cmp::Ordering::Greater => node = &n.right,
            }
        }
        false
    }
}
// When BST drops, it recursively drops all Boxes (automatic deallocation!)
```

---

## 57. Linux Kernel rbtree

```c
// Linux rbtree: include/linux/rbtree.h
// The kernel's red-black tree is INTRUSIVE — you embed rb_node in your struct.

struct rb_node {
    unsigned long  __rb_parent_color;  // parent pointer + color in low 2 bits
    struct rb_node *rb_right;
    struct rb_node *rb_left;
};
// WHY store color in parent pointer?
//   rb_node pointers are always aligned to at least 4 bytes.
//   Low 2 bits are always 0 due to alignment.
//   These bits can store the color (0=red, 1=black) WITHOUT extra memory!
//   This is a common kernel optimization trick.

struct rb_root {
    struct rb_node *rb_node;  // pointer to root node
};

// EXAMPLE: Virtual Memory Areas (vm_area_struct)
// Each process has a red-black tree of memory mappings.
// In include/linux/mm_types.h:
struct vm_area_struct {
    unsigned long   vm_start;  // start address
    unsigned long   vm_end;    // end address
    struct rb_node  vm_rb;     // embedded in process's mm->mm_rb tree
    // ... many more fields
};

// Lookup: find VMA containing a given address
struct vm_area_struct *find_vma(struct mm_struct *mm, unsigned long addr) {
    struct rb_node *rb_node = mm->mm_rb.rb_node;
    struct vm_area_struct *vma = NULL;
    
    while (rb_node) {
        struct vm_area_struct *tmp;
        tmp = rb_entry(rb_node, struct vm_area_struct, vm_rb);
        // rb_entry = container_of
        
        if (tmp->vm_end > addr) {
            vma = tmp;
            if (tmp->vm_start <= addr) break;  // found
            rb_node = rb_node->rb_left;
        } else {
            rb_node = rb_node->rb_right;
        }
    }
    return vma;
}

// Insert a new VMA: call rb_insert_color after manual BST insertion
// rb_insert_color handles rebalancing (rotations + recoloring)

// Linux CFS Scheduler also uses rbtree:
// Tasks are sorted by vruntime (virtual runtime) in an rbtree.
// The leftmost node (min vruntime) is cached as rb_leftmost.
// Picking the next task to run: just access rb_leftmost. O(1)!
// Inserting a task after it runs (updated vruntime): O(log n).
```

---

# Part X: Heaps

## 58. Binary Heap — Array Layout

```
BINARY HEAP stored as array (key insight: NO pointers needed!)

A binary heap is a COMPLETE BINARY TREE.
A complete binary tree can be stored in an array without wasted space
because there are no "missing" nodes to represent.

Max-Heap property: parent >= both children

Tree:
           16
         /    \
       14      10
      /  \    /  \
     8    7  9    3
    / \
   2   4

Array:  [16][14][10][ 8][ 7][ 9][ 3][ 2][ 4]
Index:   [0] [1] [2] [3] [4] [5] [6] [7] [8]

INDEX RELATIONSHIPS (0-based):
  Parent(i)      = (i - 1) / 2
  Left child(i)  = 2 * i + 1
  Right child(i) = 2 * i + 2
  
  Verify:
  Left(0)  = 1 -> arr[1] = 14 ✓
  Right(0) = 2 -> arr[2] = 10 ✓
  Parent(3) = 1 -> arr[1] = 14 (parent of 8 is 14) ✓
  Parent(7) = 3 -> arr[3] = 8  (parent of 2 is 8) ✓

WHY THIS IS CACHE FRIENDLY:
  The array stores nodes in level-order (BFS order).
  Parent and children are at nearby indices.
  Cache line spans multiple related nodes.
  Contrast with pointer-based tree: parent and child may be far apart in memory.
```

---

## 59. Heap Operations — Insert, Delete, Heapify

```
INSERT into Max-Heap: O(log n)
  Example: Insert 15 into heap [16, 14, 10, 8, 7, 9, 3, 2, 4]

  Step 1: Add 15 at end (index 9)
    [16, 14, 10, 8, 7, 9, 3, 2, 4, 15]

  Step 2: BUBBLE UP (sift up) — compare with parent until heap property restored
    i=9, parent=(9-1)/2=4, arr[4]=7. Is 15 > 7? YES -> swap
    [16, 14, 10, 8, 15, 9, 3, 2, 4, 7]
    
    i=4, parent=(4-1)/2=1, arr[1]=14. Is 15 > 14? YES -> swap
    [16, 15, 10, 8, 14, 9, 3, 2, 4, 7]
    
    i=1, parent=(1-1)/2=0, arr[0]=16. Is 15 > 16? NO -> stop
    Final: [16, 15, 10, 8, 14, 9, 3, 2, 4, 7]
    
    Max 3 swaps = log2(10) ≈ 3 = O(log n)

EXTRACT MAX: O(log n)
  Step 1: Save root (max value = 16)
  Step 2: Move last element to root
    [7, 15, 10, 8, 14, 9, 3, 2, 4]
  Step 3: SIFT DOWN — swap with larger child until heap property restored
    i=0 (7), children: arr[1]=15, arr[2]=10. Larger is 15 > 7 -> swap
    [15, 7, 10, 8, 14, 9, 3, 2, 4]
    
    i=1 (7), children: arr[3]=8, arr[4]=14. Larger is 14 > 7 -> swap
    [15, 14, 10, 8, 7, 9, 3, 2, 4]
    
    i=4 (7), children: arr[9] (out of bounds). Stop.
    Return 16.

BUILD HEAP from unsorted array: O(n) — NOT O(n log n)!
  Start from last non-leaf (n/2 - 1), sift down each:
  
  Why O(n)?
  Half the nodes are leaves: 0 swaps each
  Quarter at level 1: at most 1 swap each
  Eighth at level 2: at most 2 swaps each
  ...
  Total: sum_{k=0}^{log n} (n / 2^(k+1)) * k
       = n * sum_{k=0}^{log n} k / 2^(k+1)
       = n * (2 - (log n + 2)/2^(log n))  <- geometric series
       ≈ 2n = O(n)
  
  This is WHY heapSort can build the heap in O(n) then sort in O(n log n).

HEAPSORT: O(n log n) time, O(1) extra space
  1. Build max-heap from array: O(n)
  2. Repeatedly extract max, put at end: n * O(log n) = O(n log n)
  3. Result: sorted in ascending order, in-place!
```

---

## 60. Fibonacci Heap — Amortized Excellence

```
FIBONACCI HEAP — Theoretical gem, complex implementation

Operations (amortized):
  insert:       O(1)   vs O(log n) for binary heap
  find-min:     O(1)   vs O(1)
  union:        O(1)   vs O(n)
  decrease-key: O(1)   vs O(log n)
  delete-min:   O(log n) vs O(log n)
  delete:       O(log n) vs O(log n)

WHY FASTER? Lazy approach:
  - insert: just add to root list (no rebalancing)
  - union: just concatenate root lists
  - decrease-key: cut node, add to root list (mark parent if already lost a child)
  - delete-min: CONSOLIDATE — then pay all deferred costs

STRUCTURE:
  A collection of heap-ordered trees (not just one tree).
  Trees don't need to be complete — no strict shape constraint.
  
  Root list (doubly linked circular):
    [16] <-> [15] <-> [10] <-> [7]
    |
    children are heap-ordered subtrees

USED IN: Dijkstra's shortest path O((V + E) log V) -> O(V log V + E) with Fibonacci heap
         Prim's MST algorithm O(E log V) -> O(E + V log V)
         
PRACTICAL NOTE:
  Fibonacci heaps have large constant factors.
  For most practical purposes, binary heap outperforms Fibonacci heap
  due to cache effects, even though Fibonacci wins asymptotically.
  Pairing heap is a simpler alternative with similar amortized bounds.
```

---

## 61. Heap Implementations

### C

```c
typedef struct {
    int *data;
    int  size;
    int  capacity;
} MaxHeap;

void heap_swap(MaxHeap *h, int i, int j) {
    int tmp = h->data[i]; h->data[i] = h->data[j]; h->data[j] = tmp;
}

void heap_push(MaxHeap *h, int val) {
    if (h->size == h->capacity) { /* realloc */ }
    h->data[h->size++] = val;
    int i = h->size - 1;
    while (i > 0) {
        int p = (i - 1) / 2;
        if (h->data[p] < h->data[i]) { heap_swap(h, p, i); i = p; }
        else break;
    }
}

int heap_pop(MaxHeap *h) {
    int max = h->data[0];
    h->data[0] = h->data[--h->size];
    int i = 0;
    while (1) {
        int l = 2*i+1, r = 2*i+2, largest = i;
        if (l < h->size && h->data[l] > h->data[largest]) largest = l;
        if (r < h->size && h->data[r] > h->data[largest]) largest = r;
        if (largest == i) break;
        heap_swap(h, i, largest);
        i = largest;
    }
    return max;
}
```

### Go

```go
import "container/heap"

// Must implement heap.Interface:
type IntHeap []int

func (h IntHeap) Len() int           { return len(h) }
func (h IntHeap) Less(i, j int) bool { return h[i] < h[j] } // min-heap
func (h IntHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *IntHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *IntHeap) Pop() any {
    old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x
}

h := &IntHeap{5, 3, 1, 4, 2}
heap.Init(h)            // O(n) build
heap.Push(h, 0)         // O(log n)
fmt.Println(heap.Pop(h)) // 0, O(log n)
```

---

# Part XI: Graphs

## 62. Adjacency Matrix

```
GRAPH: 5 vertices (0-4), edges: 0-1, 0-4, 1-2, 1-3, 1-4, 2-3, 3-4

ADJACENCY MATRIX:
  Square matrix, size V x V, mat[i][j] = 1 if edge i->j exists.
  
       0   1   2   3   4
  0  [ 0   1   0   0   1 ]
  1  [ 1   0   1   1   1 ]
  2  [ 0   1   0   1   0 ]
  3  [ 0   1   1   0   1 ]
  4  [ 1   1   0   1   0 ]
  
  For weighted graphs: mat[i][j] = weight (0 or INF if no edge)
  For undirected: symmetric (mat[i][j] == mat[j][i])
  
  Memory: O(V^2). For 1000 vertices: 1000*1000*4 = 4MB. For 1M vertices: 4TB!
  
  Check if edge exists: O(1) — just index the array
  Iterate all edges from vertex v: O(V) — scan whole row
  Add edge: O(1)
  Add vertex: O(V^2) — must extend the matrix
  
  WHEN TO USE: Dense graphs (E ≈ V^2), frequent edge existence queries
  NOT FOR: Sparse graphs (E << V^2) — wastes memory
```

---

## 63. Adjacency List

```
ADJACENCY LIST (most common representation)

  Vertex 0: [1, 4]
  Vertex 1: [0, 2, 3, 4]
  Vertex 2: [1, 3]
  Vertex 3: [1, 2, 4]
  Vertex 4: [0, 1, 3]

Memory: O(V + E). For sparse graphs, much better than matrix.
For the internet graph (billions of vertices, sparse connections): essential.

Iterate all edges from vertex v: O(degree(v)) — only actual neighbors
Check if edge (u, v) exists: O(degree(u)) — scan u's list

IMPLEMENTATION OPTIONS:
  1. Array of arrays (fixed-size adjacency):
     int adj[V][MAX_DEGREE];  // wasteful if degrees vary widely
  
  2. Array of linked lists (dynamic):
     struct AdjList { int vertex; struct AdjList *next; };
     struct AdjList *graph[V];
     
  3. Array of vectors (most practical):
     vector<int> adj[V];  // C++ / Go: [][]int
  
  4. Compressed Sparse Row (CSR) — best for static graphs and cache:
     Two arrays:
       edges[]: all neighbors, sorted by source vertex, contiguous
       offsets[]: edges[offsets[v]..offsets[v+1]-1] = neighbors of v
     
     For the graph above:
     offsets: [0, 2, 6, 8, 11, 14]  (vertex i's neighbors start at offsets[i])
     edges:   [1,4, 0,2,3,4, 1,3, 1,2,4, 0,1,3]
     
     Memory: O(V + E), fully contiguous, excellent cache behavior.
     Iteration: iterate edges[offsets[v]..offsets[v+1]-1] — sequential memory!
     Cannot add/remove edges (static structure).
     Used in: graph databases, PageRank, social network analysis.
```

---

## 64. BFS and DFS — Traversal Mechanics

```
BFS (Breadth-First Search) — explores level by level using a QUEUE

Starting from vertex 0:
  Visit 0.
  Queue = [0]
  
  Dequeue 0. Mark visited. Enqueue neighbors 1, 4.
  Queue = [1, 4]. Visited = {0}
  
  Dequeue 1. Mark visited. Enqueue unvisited neighbors 2, 3.
  Queue = [4, 2, 3]. Visited = {0, 1}
  
  Dequeue 4. Mark visited. Enqueue unvisited neighbors 3 (already there).
  Queue = [2, 3]. Visited = {0, 1, 4}
  
  ... continues until queue empty
  BFS Order: 0, 1, 4, 2, 3

BFS properties:
  - Finds SHORTEST PATH (in unweighted graphs) from source to any vertex
  - O(V + E) time, O(V) space for the queue + visited set
  - "Visited" array prevents revisiting (crucial for cycles)

DFS (Depth-First Search) — explores as deep as possible using a STACK (or recursion)

Stack (or recursive call stack):
  Visit 0. Push unvisited neighbors.
  Stack: [1, 4]
  
  Pop 4. Visit 4. Push unvisited neighbors 3.
  Stack: [1, 3]
  
  Pop 3. Visit 3. Push unvisited neighbors 2.
  Stack: [1, 2]
  
  Pop 2. Visit 2. Push unvisited neighbor 1.
  Stack: [1, 1] (but 1 might be marked visited already by the time we pop)
  
  DFS Order (depends on neighbor order): 0, 4, 3, 2, 1

DFS properties:
  - Detects cycles (back edges)
  - Topological sort (for DAGs)
  - Finding strongly connected components (Tarjan's, Kosaraju's)
  - Maze solving
  - O(V + E) time, O(V) space for the stack
  
MEMORY COMPARISON:
  BFS: stores ALL nodes at current level -> can use O(V) memory (wide graphs)
  DFS: stores ONE path from root to current node -> O(depth) memory
  For a balanced tree of depth h=log n: BFS uses O(n/2) memory at last level,
  DFS uses O(log n) memory. DFS wins on memory for trees.
```

---

## 65. Graph Implementations

### C — Adjacency List

```c
#include 

#define MAX_V 1000

typedef struct AdjNode {
    int vertex;
    int weight;
    struct AdjNode *next;
} AdjNode;

typedef struct {
    AdjNode *head[MAX_V];
    int V, E;
} Graph;

Graph* graph_new(int V) {
    Graph *g = calloc(1, sizeof(Graph));
    g->V = V;
    return g;
}

void graph_add_edge(Graph *g, int u, int v, int w) {
    AdjNode *n = malloc(sizeof(AdjNode));
    n->vertex = v; n->weight = w;
    n->next = g->head[u];
    g->head[u] = n;
    g->E++;
}

// BFS
void graph_bfs(Graph *g, int src) {
    bool *visited = calloc(g->V, sizeof(bool));
    int *queue = malloc(g->V * sizeof(int));
    int head = 0, tail = 0;
    
    visited[src] = true;
    queue[tail++] = src;
    
    while (head < tail) {
        int u = queue[head++];
        printf("Visit %d\n", u);
        for (AdjNode *n = g->head[u]; n; n = n->next) {
            if (!visited[n->vertex]) {
                visited[n->vertex] = true;
                queue[tail++] = n->vertex;
            }
        }
    }
    free(visited); free(queue);
}
```

---

# Part XII: Advanced Data Structures

## 66. Skip List

```
SKIP LIST — probabilistic data structure, alternative to balanced BST

A skip list is a hierarchy of sorted linked lists.
Layer 0: complete list (all elements)
Layer 1: ~half the elements (every ~2nd)
Layer 2: ~quarter of elements (every ~4th)
...

Example: elements 1, 3, 5, 7, 9, 12, 17, 21

Layer 3: [1] -----------------------------------------> [21] -> NULL
Layer 2: [1] ----------------> [12] ---------------> [21] -> NULL
Layer 1: [1] -----> [5] ------> [12] ------> [17] -> [21] -> NULL
Layer 0: [1] -> [3] -> [5] -> [7] -> [9] -> [12] -> [17] -> [21] -> NULL

Each node has:
  - value
  - array of 'next' pointers (one per level this node participates in)
  
Node height determined randomly on insert: coin flip, P(height=k) = 2^(-k)

SEARCH(9):
  Start at top-left (layer 3, node 1).
  Layer 3: 1 -> NULL (next is NULL or > 9), go down
  Layer 2: 1 -> 12 (12 > 9), go down at 1
  Layer 1: 1 -> 5 -> 12 (12 > 9), go down at 5
  Layer 0: 5 -> 7 -> 9, found!
  
Nodes visited: 1, 1, 5, 5, 7, 9 = 6 nodes. log2(8) = 3, so ~2 nodes/level. ✓

Expected time: O(log n) for search, insert, delete (with high probability).
Same as balanced BST but:
  + Simpler implementation (no rotations!)
  + Concurrent access easier (CAS-based lock-free skip lists)
  + Insert/delete don't require traversal from root for rebalancing
  - Extra memory per node (height pointers)
  - Not worst-case O(log n) (probabilistic)

Redis uses a skip list for its sorted set (ZSET) data structure.
LevelDB/RocksDB use skip lists for in-memory mutable state (MemTable).
Linux CFS scheduler considered using skip list (went with rbtree).
```

---

## 67. Bloom Filter

```
BLOOM FILTER — space-efficient probabilistic set

Supports: "is element X in the set?" with:
  - NO FALSE NEGATIVES: if filter says "not in set", it's definitely not
  - POSSIBLE FALSE POSITIVES: if filter says "in set", it might not be

Useful when: you want to avoid expensive lookups for items definitely absent.

STRUCTURE: bit array of m bits + k hash functions

Example: m=20 bits, k=3 hash functions (h1, h2, h3)

Initially: all zeros
0000 0000 0000 0000 0000

Insert "cat":
  h1("cat") = 3  -> set bit 3
  h2("cat") = 7  -> set bit 7
  h3("cat") = 14 -> set bit 14
0001 0001 0000 0001 0000
      3       7              14

Insert "dog":
  h1("dog") = 1  -> set bit 1
  h2("dog") = 7  -> set bit 7 (already set)
  h3("dog") = 19 -> set bit 19
0101 0001 0000 0001 0001
   1    3    7             14          19

Query "cat":
  Check bits 3, 7, 14: all set -> "possibly in set" ✓ (true positive)

Query "fish":
  h1("fish") = 3  -> bit 3 set
  h2("fish") = 7  -> bit 7 set
  h3("fish") = 6  -> bit 6 NOT set -> "definitely not in set" ✓ (true negative)

Query "bird":
  h1("bird") = 1  -> bit 1 set
  h2("bird") = 7  -> bit 7 set
  h3("bird") = 14 -> bit 14 set
  -> "possibly in set" — but "bird" was never inserted! FALSE POSITIVE!

FALSE POSITIVE RATE: (1 - e^(-kn/m))^k where n = elements inserted
  With k=3, n=2, m=20: P ≈ (1-e^(-0.3))^3 ≈ 0.024 = 2.4%
  
SPACE EFFICIENCY:
  For 1 million URLs with 1% false positive rate:
  Bloom filter: ~9.6 MB
  Hash table: ~50+ MB (each URL ≈ 50 bytes average)
  ~5x more memory efficient!

CANNOT DELETE — removing an element would clear bits shared by other elements.
(Counting Bloom Filter: use counters instead of bits, allows deletion, 4-8x more memory)

USED BY:
  - Google BigTable, Apache HBase: check if row exists before disk lookup
  - Redis: RedisBloom module
  - Akamai CDN: filter one-hit-wonders from cache
  - Chrome browser: safe browsing malware URL check
  - PostgreSQL: hash joins (to filter outer table rows)
  - Linux: disk scheduler (to avoid unnecessary block device reads)
```

---

## 68. Union-Find — Disjoint Set Union

```
UNION-FIND (Disjoint Set Union, DSU)

Tracks a collection of elements partitioned into disjoint sets.
Operations:
  find(x): which set does x belong to? (returns "representative" of the set)
  union(x, y): merge the sets containing x and y

NAIVE IMPLEMENTATION (array, each element stores set ID):
  find: O(1), union: O(n) — must update all elements in one set

TREE IMPLEMENTATION:
  Each element is a node. Each node points to its parent. Roots point to themselves.
  Set representative = root of tree.
  
  Initially: [0]  [1]  [2]  [3]  [4]  (each element is its own set)
  parent:    [0]  [1]  [2]  [3]  [4]
  
  union(0, 1): root(0)=0, root(1)=1, parent[1] = 0
  parent: [0] [0] [2] [3] [4]
  Tree: 0-1  2  3  4
  
  union(1, 2): root(1)=0, root(2)=2, parent[2] = 0
  parent: [0] [0] [0] [3] [4]
  Tree: 0-1-2  3  4
  
  find(2): 2 -> 0 (root), O(depth)

OPTIMIZATION 1 — UNION BY RANK:
  Always attach smaller tree under root of larger tree.
  Prevents degenerate tall trees. Height stays O(log n).
  
OPTIMIZATION 2 — PATH COMPRESSION:
  During find, flatten the path: point all traversed nodes directly to root.
  
  find(4) without compression: 4 -> 3 -> 1 -> 0 (root)
  After path compression:
  parent[4] = 0, parent[3] = 0, parent[1] = 0 (already is 0)
  
  Next find(4): 4 -> 0 (direct!)

WITH BOTH OPTIMIZATIONS:
  find and union are nearly O(1) — amortized O(α(n)) where α is the
  inverse Ackermann function. For any practical n, α(n) <= 4.
  Effectively O(1) for all practical purposes.

USE CASES:
  - Kruskal's MST algorithm
  - Detecting cycles in undirected graphs
  - Network connectivity (are A and B connected?)
  - Percolation problems
  - Image segmentation (connected components)
  - Compiler: checking if two types are unified (Hindley-Milner)
```

---

## 69. LRU Cache

```
LRU CACHE (Least Recently Used)

Eviction policy: when cache is full, evict the LEAST RECENTLY USED item.
All operations must be O(1).

DESIGN: HashMap + Doubly Linked List

HashMap: key -> pointer to list node
List: ordered by recency (head = most recent, tail = least recent)

+----------+      +---------+    +---------+    +---------+
| HashMap  |      | Node A  | <->| Node B  | <->| Node C  |
| "A" -> * |----> |(key,val)|    |(key,val)|    |(key,val)|
| "B" -> * |----------^          |         |    |         |
| "C" -> * |---------------------+----^----+    |         |
+----------+                          |---------^---------+
                  head (MRU)                        tail (LRU)

GET("A"):
  1. HashMap["A"] -> pointer to Node A in list: O(1)
  2. Move Node A to HEAD of list: O(1) (doubly linked, have prev/next)
  3. Return A's value

PUT("D", val) when cache full (capacity 3):
  1. If "D" already in map: update value, move to head
  2. Else:
     a. Remove TAIL node (Node C, LRU): O(1) doubly linked list
     b. Delete HashMap["C"]: O(1) hash map
     c. Create new Node D at HEAD: O(1)
     d. HashMap["D"] = &Node D: O(1)
  All O(1)!

WHY DOUBLY LINKED (not singly)?
  Moving a node to head requires updating its predecessor's 'next'.
  With only a forward pointer, finding predecessor = O(n).
  Doubly linked: Node has *prev, so predecessor update is O(1).
  
WHY HASH MAP + LIST (not just one structure)?
  HashMap alone: O(1) get/put but can't efficiently find LRU
  List alone: O(1) LRU eviction but O(n) lookup
  Combined: O(1) for everything — the HashMap indexes INTO the list.
  
Used by: CPU caches (hardware), OS page cache, DNS resolvers,
         database buffer pools, CDN edge caches, browser caches.
```

---

# Part XIII: Linux Kernel Data Structures — In Depth

## 70. list_head — Deep Dive (covered above, additional details)

```c
// The LIST_HEAD macro creates an initialized empty circular list:
// LIST_HEAD(mylist) creates:
//   struct list_head mylist = { .next = &mylist, .prev = &mylist }
// An empty list: head.next == head.prev == &head (points to itself)

// THREAD SAFETY:
// list_head is NOT inherently thread-safe.
// For concurrent access, the kernel wraps list operations in:
//   spin_lock / spin_unlock (for short critical sections)
//   rcu_read_lock / rcu_read_unlock + list_add_rcu / list_del_rcu (for RCU-protected lists)

// SORTED LIST (list_head variant):
// The kernel has no generic sorted list insertion.
// You insert in sorted order manually:
void sorted_insert(struct list_head *head, struct my_entry *new) {
    struct my_entry *entry;
    list_for_each_entry(entry, head, list) {
        if (entry->priority > new->priority) {
            list_add_tail(&new->list, &entry->list); // insert before entry
            return;
        }
    }
    list_add_tail(&new->list, head); // add at end if largest
}

// Used for: timer list (sorted by expiry), wait queues (sorted by priority)

// REAL KERNEL CODE: how processes are linked
// In kernel/sched/core.c, when a new task is created (fork):
// list_add_tail_rcu(&p->tasks, &init_task.tasks);
// This adds the new task to the global task list (RCU-protected).

// Iterating from userspace analog:
// cat /proc/[pid]/status reads task_struct via this exact list.
```

---

## 71. hlist — Hash Lists

```c
// hlist_head (8 bytes) vs list_head (16 bytes) for bucket heads.
// Saves 8 bytes per bucket. For PIDHASH_SZ=4096 buckets: 32KB saved.
// For large hash tables with millions of buckets: significant.

// The **pprev trick:
// hlist_node:
//   struct hlist_node *next;
//   struct hlist_node **pprev;  // pointer to the pointer that points to us
//   
//   For the first node: pprev = &head->first
//   For other nodes:    pprev = &prev_node->next
//
// Deletion of a node:
//   *node->pprev = node->next;  // works uniformly regardless of position!
//   if (node->next) node->next->pprev = node->pprev;
//   
//   This works whether we're the first node (pprev points into hlist_head)
//   or a middle node (pprev points into previous hlist_node).
//   No special case for "is this the first node?" — elegant!

// KERNEL EXAMPLE: inode cache hash table
// In fs/inode.c:
// #define I_HASHBITS  i_hash_shift
// static struct hlist_head *inode_hashtable;
//
// struct inode contains: struct hlist_node i_hash;
//
// Lookup inode by (superblock, inode_number):
// head = inode_hashtable + hash(sb, ino);
// hlist_for_each_entry(inode, head, i_hash) {
//     if (inode->i_ino == ino && inode->i_sb == sb)
//         return inode;
// }
```

---

## 72. rbtree — Red-Black in the Kernel (additional details)

```c
// Linux rbtree does NOT include the comparison function or key type.
// WHY? Because the kernel supports many data types, and embedding the
// comparison in the tree would require function pointers (vtables), adding
// overhead and preventing inlining.
//
// Instead: YOU implement the comparison and walk the tree yourself.
// The kernel provides ONLY the balancing (rb_insert_color, rb_erase).

// Pattern for using rbtree:
struct my_entry {
    int key;
    int value;
    struct rb_node rb;  // embedded rb_node
};

// Insert into rbtree:
int my_insert(struct rb_root *root, struct my_entry *new) {
    struct rb_node **link = &root->rb_node, *parent = NULL;
    
    // Walk to correct position (standard BST insert)
    while (*link) {
        struct my_entry *entry = rb_entry(*link, struct my_entry, rb);
        parent = *link;
        
        if (new->key < entry->key)
            link = &(*link)->rb_left;
        else if (new->key > entry->key)
            link = &(*link)->rb_right;
        else
            return -EEXIST; // duplicate
    }
    
    // Link the new node
    rb_link_node(&new->rb, parent, link);
    
    // Rebalance (kernel handles all rotation/recoloring logic)
    rb_insert_color(&new->rb, root);
    return 0;
}

// Augmented rbtree (Linux 3.10+):
// Allows storing additional information per node (e.g., subtree max/min).
// Used for interval trees (vm_area_struct: track max vm_end in subtree).
// When a node is inserted/rotated, a callback updates the augmented data.
// interval_tree in lib/interval_tree.c uses this for vma_interval_tree.

// CFS Scheduler usage:
// In kernel/sched/fair.c:
// Each runqueue (per-CPU) has: struct rb_root_cached tasks_timeline;
// rb_root_cached: rb_root + cached leftmost node pointer
// Picking next task = rb_first_cached(&cfs_rq->tasks_timeline) — O(1)!
// (The leftmost = minimum vruntime = next to run)
```

---

## 73. XArray — The New Radix Tree

```c
// XArray (introduced in Linux 4.20) replaces the older radix tree.
// Used primarily for the PAGE CACHE: maps file offsets to struct page*.

// Conceptually: a very fast, sparse array indexed by unsigned long.
// Internally: a multi-level tree with variable branching factor.
// The leaves store the values. Internal nodes store pointers to children.

// XArray stores:
//   - Arbitrary pointers (aligned to at least 4 bytes)
//   - Small unsigned integers (1, 2, 3...) stored as "values" (using low bits)
//   - Marks: 3 bits per entry (for dirty, accessed, etc.)

// Page cache usage (simplified):
// struct address_space {
//     struct xarray i_pages;  // maps page_index -> struct page*
//     ...
// };
//
// Looking up a page at file offset:
//   struct page *page = xa_load(&inode->i_mapping->i_pages, page_index);
//   // page_index = file_offset / PAGE_SIZE
//
// Storing a new page:
//   xa_store(&inode->i_mapping->i_pages, page_index, page, GFP_KERNEL);

// XArray advantages over radix tree:
//   - Simpler API
//   - Built-in locking (xa_lock)
//   - Better support for sparse/dense arrays
//   - Faster for sequential access patterns
//   - Marks/tags stored efficiently alongside pointers

// Operations:
//   xa_load(xa, index)              O(log n)
//   xa_store(xa, index, entry, gfp) O(log n)
//   xa_erase(xa, index)             O(log n)
//   xa_find(xa, &index, max, filter) O(log n) — find next entry with filter
//   xa_for_each(xa, index, entry)   iterate all entries
//   xa_for_each_range(xa, i, entry, first, last) iterate range

// The IDR (ID allocator) is also built on XArray:
//   Allocates small integer IDs to kernel objects.
//   e.g., file descriptor allocation, PID allocation (newer kernels)
```

---

## 74. kfifo — Deep Dive

```c
// kfifo is designed for SPSC (Single Producer Single Consumer) without locks.
// MPMC (multiple producers/consumers) requires external locking.

// Why no lock for SPSC?
// Memory barrier analysis:
//
// Producer (writes 'in'):
//   1. Write data to kfifo->buf[kfifo->in & kfifo->mask]
//   2. smp_wmb() — memory write barrier (ensures data written before 'in' updated)
//   3. WRITE_ONCE(kfifo->in, kfifo->in + len)
//
// Consumer (reads 'out', reads 'in'):
//   1. READ_ONCE(kfifo->in) — read producer's index
//   2. smp_rmb() — memory read barrier (ensures 'in' read before data)
//   3. Read data from kfifo->buf[kfifo->out & kfifo->mask]
//   4. smp_wmb()
//   5. WRITE_ONCE(kfifo->out, kfifo->out + len)
//
// On x86: smp_wmb/smp_rmb are no-ops (TSO — total store order guarantees this).
// On ARM/Power: these are actual barriers (weaker memory models).
//
// WRITE_ONCE/READ_ONCE: prevent compiler from caching in register or reordering.

// kfifo initialization:
DEFINE_KFIFO(fifo, u8, 4096);   // static 4096-byte kfifo
// or dynamic:
struct kfifo *fifo = kfifo_alloc(4096, GFP_KERNEL, NULL);

// Producer (interrupt handler):
unsigned int len = kfifo_in(&fifo, data, sizeof(data));

// Consumer (process context, blocking):
unsigned int len = kfifo_out(&fifo, buf, sizeof(buf));
// Returns bytes read.

// Check available space:
kfifo_avail(&fifo)  // space for writing
kfifo_len(&fifo)    // bytes available for reading
kfifo_is_empty(&fifo)
kfifo_is_full(&fifo)

// kfifo used in:
// drivers/char/random.c (entropy pool — now uses CRC, but concept similar)
// drivers/input/input.c (input event queuing)
// drivers/usb/gadget/ (USB endpoint buffers)
// net/core/ (packet queuing in some paths)
// sound/core/ (ALSA audio ring buffers)
```

---

## 75. RCU — Read-Copy-Update

```
RCU is one of the most important synchronization mechanisms in the Linux kernel.
It allows READS to proceed with ZERO synchronization cost (no locks, no atomics).

The Fundamental Idea:
  - Readers: never block, never modify data, extremely cheap
  - Writers: make a copy, modify the copy, atomically publish the new version,
             wait for existing readers to finish, then free the old version.

EXAMPLE: Updating a pointer to a data structure

Old state: gptr -> [old_data]

Writer:
  1. new_data = kmalloc(...)    // allocate new structure
  2. copy old_data to new_data  // make a copy
  3. modify new_data            // make changes
  4. rcu_assign_pointer(gptr, new_data)  // ATOMIC pointer swap
     // Includes a write memory barrier — ensures new_data fully written
     // before gptr is updated.
  5. synchronize_rcu()          // wait for all active readers to finish
     // This is the "grace period" — wait until all CPUs have passed
     // through a quiescent state (context switch, idle, etc.)
  6. kfree(old_data)            // safe to free now

Reader (in rcu_read_lock / rcu_read_unlock):
  rcu_read_lock();              // disable preemption (very cheap!)
  ptr = rcu_dereference(gptr); // read pointer (includes read memory barrier)
  use(ptr);                    // safe: writer won't free old data during RCU read section
  rcu_read_unlock();            // reenable preemption

ZERO COST FOR READERS (on non-preemptible kernels):
  rcu_read_lock() = preempt_disable() = decrement preempt counter (1 instruction)
  rcu_read_unlock() = preempt_enable() = increment preempt counter (1 instruction)
  
  Compare to rwlock: reader must still do an atomic increment/decrement of
  read_count, which causes cache line contention. RCU has NO such contention.

WHEN TO USE RCU:
  - Read-mostly data structures (routing tables, module lists, process lists)
  - Data must be traversable without blocking
  - Updates are infrequent
  
USED IN:
  - task_list: list of all processes (list_for_each_entry_rcu)
  - module list: loaded kernel modules
  - network routing tables
  - VFS dcache (directory entry cache)
  - network protocol handlers
  - file descriptor tables
  - RCU-protected hash tables (thousands of users in kernel)

CALL_RCU (deferred freeing):
  Instead of synchronize_rcu() (blocks writer), use call_rcu(old, callback).
  callback is called after the grace period ends (in a workqueue/softirq).
  Writer continues immediately, freeing happens asynchronously.
  Better for writers in interrupt context or latency-sensitive code.
```

---

## 76. Per-CPU Variables

```c
// PER-CPU VARIABLES: one copy of a variable per CPU core.
// No locking needed (each CPU touches only its own copy).
// No cache line bouncing between CPUs.

// Declaration:
DEFINE_PER_CPU(int, my_counter); // one 'int' per CPU

// Usage:
void increment(void) {
    int *counter = get_cpu_ptr(&my_counter); // disables preemption, get this CPU's copy
    (*counter)++;
    put_cpu_ptr(&my_counter); // re-enable preemption
}

// Or with explicit CPU number:
this_cpu_inc(my_counter); // atomic increment of current CPU's copy (no preempt disable needed)
per_cpu(my_counter, cpu); // access specific CPU's copy (need appropriate locking)

// Memory layout of per-CPU variables:
// Physical memory has N contiguous sections, one per CPU:
//
//  CPU0 section         CPU1 section         CPU2 section
//  +----------------+   +----------------+   +----------------+
//  | my_counter=0   |   | my_counter=0   |   | my_counter=0   |
//  | other_var=...  |   | other_var=...  |   | other_var=...  |
//  +----------------+   +----------------+   +----------------+
//
// The compiler emits code using a base pointer (GS segment on x86-64).
// GS:offset accesses the current CPU's data.
// Switching CPUs changes the GS base pointer.
// Each CPU's data is in its OWN cache line — no false sharing!

// EXAMPLES in kernel:
// - Network softirq stats: per-cpu network packet counters
// - slab allocator: per-cpu free lists (avoid lock contention)
// - scheduler: per-cpu run queues (struct rq)
// - vmstat: per-cpu page allocation statistics
// - CPU timers and local APIC timers
```

---

## 77. Wait Queues

```c
// Wait queues allow processes to sleep until a condition becomes true.
// They are essentially a list of sleeping tasks + a wakeup mechanism.

// Declaration:
DECLARE_WAITQUEUE_HEAD(my_wq); // statically initialized wait queue
// or: init_waitqueue_head(&my_wq);

// Sleeping (process context, e.g., read() syscall waiting for data):
wait_event(my_wq, condition);
// Expands to a loop:
//   while (!condition) {
//       add current task to wait queue (state = TASK_INTERRUPTIBLE)
//       schedule() // yield CPU, wake up only when condition might be true
//   }
//   remove task from wait queue

// With timeout:
wait_event_timeout(my_wq, condition, timeout_jiffies);

// Interruptible (can be woken by signals like SIGINT):
wait_event_interruptible(my_wq, condition);

// Waking up (e.g., interrupt handler signals data available):
wake_up(&my_wq);           // wake ONE waiting task
wake_up_all(&my_wq);       // wake ALL waiting tasks

// INTERNAL STRUCTURE:
struct wait_queue_head {
    spinlock_t           lock;  // protects the list
    struct list_head     head;  // list of wait_queue_entry
};

struct wait_queue_entry {
    unsigned int         flags;
    void                *private; // the sleeping task_struct*
    wait_queue_func_t    func;    // callback to wake this entry
    struct list_head     entry;   // link in the wait queue list
};

// USED BY EVERY blocking I/O operation:
// read() on pipe, socket, char device
// write() when buffer full
// poll/select/epoll (fundamental to async I/O)
// mutex_lock_interruptible
// down_interruptible (semaphore)
// msleep, schedule_timeout
```

---

## 78. The SLUB Memory Allocator

```
SLUB ALLOCATOR — kernel's default object cache since Linux 2.6.23

The problem: malloc/free is too slow for frequent small kernel allocations.
Calling the buddy allocator (page allocator) for every 64-byte struct
would be catastrophically slow. SLUB/SLAB solves this.

CONCEPT: Pre-allocate "slabs" of pages, divide into same-sized objects,
         maintain per-CPU freelists for fast allocation without locks.

STRUCTURE:

  kmem_cache (one per object type):
  +-------------------------+
  | name: "task_struct"     |
  | object_size: 9936       |  <- size of one task_struct
  | cpu_cache[0..N]         |  <- per-CPU allocation caches
  | node[0..M]              |  <- per-NUMA-node partial slab lists
  +-------------------------+
  
  Per-CPU cache (cpu_cache):
  +------------------------------------------+
  | freelist: [obj_ptr, obj_ptr, ...] (stack)|  <- pop for alloc, push for free
  | slab: ptr to current slab page           |
  +------------------------------------------+
  
  Slab page (one or more pages):
  +------+------+------+------+------+------+
  | obj0 | obj1 | obj2 | obj3 | obj4 | obj5 |  <- N objects of same size
  +------+------+------+------+------+------+
  Free objects linked via freelist pointers embedded INSIDE the objects.
  (The first word of each free object points to next free object.)

ALLOCATION PATH:
  1. Get pointer from per-CPU freelist (NO LOCK): O(1)
     If freelist empty: goto step 2
  2. Get a slab from partial list (spin_lock):
     A partial slab has some free, some allocated objects.
  3. If no partial slabs: allocate new slab (buddy allocator: 1-2 pages)
  
  Fast path (step 1): single pointer read + pointer update = ~5 cycles!
  Compare to malloc: ~50-200ns for small allocations.

KMALLOC vs KMEM_CACHE:
  kmalloc(size, flags): like malloc, uses size-based caches (kmalloc-8, kmalloc-16, ..., kmalloc-8192)
  kmem_cache_alloc(cache, flags): allocate from a specific named cache
  
  Named caches: task_struct cache, mm_struct cache, files_struct cache, dentry cache...
  
  Benefits of named caches:
  - Objects of same size (no fragmentation within slab)
  - Constructor/destructor for partial initialization
  - SLUB debugging: detect use-after-free, buffer overflows
  - Tracing and statistics per object type

kmalloc(4096, GFP_KERNEL)   // kernel can sleep waiting for memory
kmalloc(4096, GFP_ATOMIC)   // interrupt context, cannot sleep, may fail
kmalloc(4096, GFP_NOWAIT)   // don't wait (no memory reclaim)
kfree(ptr)                   // return to slab (NO size argument needed!
                             // SLUB tracks size via page metadata)
```

---

# Part XIV: Memory Allocators

## 79. How malloc Works Internally

```
malloc() in glibc (ptmalloc2) — the most common Linux userspace allocator

HEAP STRUCTURE:
  The heap starts at 'brk' (program break — end of data segment).
  sbrk(n) extends the heap by n bytes (syscall).
  mmap() used for large allocations (> 128KB typically).

HEAP CHUNKS:
  Every allocated region is preceded by a chunk header:
  
  Free chunk:
  +----------+----------+-----------+-----------+----------+
  | prev_sz  |   size   | fd (fwd)  | bk (bwd)  |  ...     |
  |  (8B)    |  (8B)    |  (8B)     |  (8B)     |  ...     |
  +----------+----------+-----------+-----------+----------+
  
  Allocated chunk:
  +----------+----------+-------------------------------+
  | prev_sz  |   size   |       user data               |
  |  (8B)    |  (8B)    |       (n bytes)               |
  +----------+----------+-------------------------------+
  
  size field encodes:
    bits 3+: actual chunk size (multiple of 8 or 16 bytes)
    bit 2:   IS_MMAPPED — was this mmap'd?
    bit 1:   NON_MAIN_ARENA — belongs to thread arena?
    bit 0:   PREV_INUSE — is previous chunk in use?

FREE BINS:
  ptmalloc maintains several free lists (bins):
  
  Fastbins: 8 small sizes (16, 24, 32, ..., 80 bytes), singly linked, LIFO
            Extremely fast: no coalescing, immediate reuse.
  
  Small bins: sizes 16..504 bytes, 62 doubly linked circular lists (one per size)
              Exact fit. O(1) allocation.
  
  Large bins: sizes 512+, 63 lists covering size ranges
              Best-fit within bin. O(log n) or O(1) with skip list trick.
  
  Unsorted bin: recently freed chunks go here first.
               On allocation request, unsorted bin is searched + bins refilled.
  
  Top chunk: the "wilderness" — the unallocated region at top of heap.
             Allocation from top: just increment the pointer (similar to bump alloc).
             Freed chunks adjacent to top are merged back into top.

malloc(8):
  1. Round up to minimum chunk size (16-24 bytes typically)
  2. Check fastbin[index]: if not empty, return head. Done! O(1)
  3. Check small bins[index]: if not empty, return. O(1)
  4. Process unsorted bin, refill sorted bins
  5. Check large bins, find best fit
  6. Split from top chunk
  7. If top exhausted: sbrk() to extend heap
  
free(ptr):
  1. Optionally coalesce with adjacent free chunks (merge) to reduce fragmentation
  2. Put in fastbin (if small enough) or sorted bins
  
THREAD SAFETY: ptmalloc uses one mutex per "arena" (one arena per 2-8 threads).
  Heavy threading: arena contention -> jemalloc / tcmalloc (thread-local caches).
```

---

## 80. Arena and Pool Allocators

```c
// ARENA ALLOCATOR (Region Allocator):
// Allocate many small objects from a pre-allocated region.
// No individual frees — free the ENTIRE arena at once.

typedef struct {
    char *start;
    char *current;
    char *end;
} Arena;

Arena arena_new(size_t size) {
    char *mem = malloc(size);
    return (Arena){ mem, mem, mem + size };
}

void* arena_alloc(Arena *a, size_t size) {
    // Align to 8 bytes:
    size = (size + 7) & ~7;
    if (a->current + size > a->end) return NULL; // out of memory
    void *ptr = a->current;
    a->current += size;
    return ptr; // JUST A POINTER BUMP! 2-3 instructions.
}

void arena_reset(Arena *a) { a->current = a->start; } // O(1)! reuse all memory
void arena_free(Arena *a)  { free(a->start); } // one free for thousands of objects

// BENEFITS:
// - Allocation: 2-3 instructions (pointer bump) vs ~50-200ns for malloc
// - No individual frees needed (zero overhead for each object)
// - Zero fragmentation (objects packed contiguously)
// - Cache friendly (objects allocated sequentially are adjacent in memory)
// - Natural for request/response cycle: allocate all objects for one request,
//   free entire arena when request completes.

// USED BY:
// - Compilers (allocate AST nodes, all freed at end of compilation)
// - Web servers (per-request allocation)
// - Game engines (per-frame allocation)
// - Linux kernel slab allocator (conceptually similar)

// POOL ALLOCATOR:
// Like an arena, but all objects are the SAME SIZE.
// Maintains a freelist of freed objects for reuse.

typedef struct PoolBlock {
    struct PoolBlock *next;
} PoolBlock;

typedef struct {
    char     *memory;
    PoolBlock *freelist;
    size_t    obj_size;
} Pool;

void* pool_alloc(Pool *p) {
    if (p->freelist) {
        void *obj = p->freelist;
        p->freelist = p->freelist->next; // pop from freelist
        return obj;
    }
    // allocate from backing arena...
    return NULL;
}

void pool_free(Pool *p, void *obj) {
    PoolBlock *b = obj;
    b->next = p->freelist;  // push to freelist (O(1))
    p->freelist = b;
}

// Pool allocators are used for:
// - Networking: fixed-size packet buffers (skbuff in Linux)
// - Databases: fixed-size page buffers
// - Games: particle systems (millions of same-size particles)
// - Linux: kmem_cache IS a pool allocator for kernel objects
```

---

## 81. Bump Allocators

```
BUMP ALLOCATOR (Linear Allocator, Stack Allocator):
The simplest possible allocator.

start                          current                 end
  |                                |                    |
  v                                v                    v
  +================================+--------------------+
  | allocated | allocated | alloc  |  free space        |
  +================================+--------------------+

Alloc(n):
  if current + n > end: OOM error
  ptr = current
  current += align_up(n, alignment)  // one addition + alignment mask
  return ptr

Free: not supported individually! Reset all at once.

Performance: Allocation = 1 addition + 1 comparison = ~1ns
Compare: malloc = ~50-100ns

Rust's entire test framework uses a bump allocator for test output buffering.
WebAssembly's standard allocator in many runtimes is a bump allocator.
Garbage collectors use bump allocation for the young generation (nursery).

In Rust (via bumpalo crate):
  let bump = Bump::new();
  let x = bump.alloc(5);     // x: &mut i32, allocated in bump arena
  let y = bump.alloc("hi");  // another allocation
  // When bump drops, ALL allocations freed at once.
  
JEMALLOC (used by Firefox, FreeBSD, Rust pre-1.32):
  - Thread-local caches per size class
  - Arenas per group of threads
  - Excellent for multi-threaded workloads
  - Detailed statistics API
  
TCMALLOC (Thread-Caching malloc, by Google):
  - Per-thread caches (size-class free lists in TLS)
  - Allocations from thread cache: NO lock, NO atomic, ~2 instructions!
  - Large allocations go to shared central heap (with lock)
  - Used by Google's production systems, Chrome
```

---

# Part XV: Language Deep Dives

## 82. C — Direct Memory Ownership Model

```c
// C gives you complete, explicit control over memory.
// There is NO automatic memory management.

// OWNERSHIP RULES (by convention, not enforced):
//   1. The function that allocates memory "owns" it until it transfers ownership
//   2. Ownership must be explicitly transferred via return or parameters
//   3. Every malloc must have exactly one matching free (no more, no less)

// COMMON BUGS:

// 1. Use-after-free:
int *p = malloc(sizeof(int));
*p = 42;
free(p);
*p = 100; // UNDEFINED BEHAVIOR (might work, might crash, might corrupt memory)
p = NULL; // Best practice: set to NULL after free to catch accidental reuse

// 2. Double free:
free(p); // already freed above! -> heap corruption, potential security vulnerability

// 3. Memory leak:
void leak(void) {
    int *p = malloc(1000 * sizeof(int));
    // function returns without freeing p
    // p goes out of scope, malloc'd memory is lost forever
    // process memory grows unboundedly if called repeatedly
}

// 4. Buffer overflow (most dangerous):
char buf[10];
strcpy(buf, "hello world!"); // 12 bytes -> writes past buf!
// Can overwrite return address -> code execution (classic exploit technique)

// TOOLS FOR CATCHING BUGS:
// Valgrind (memcheck): detects leaks, use-after-free, invalid reads/writes (10x slower)
// AddressSanitizer (ASan): compile-time instrumentation, 2x slower
//   gcc -fsanitize=address -g
// MemorySanitizer: detect uninitialized reads (Clang only)
// LeakSanitizer: detect leaks (built into ASan)

// POINTER QUALIFIERS:
const int *p = &x;    // cannot modify *p, can modify p itself
int * const p = &x;   // cannot modify p (pointer), can modify *p
const int * const p;  // cannot modify either

// RESTRICT keyword: promise to compiler that this pointer is the ONLY way
// to access this memory (enables optimizations):
void add(int *restrict a, const int *restrict b, int n) {
    for (int i = 0; i < n; i++) a[i] += b[i]; // compiler can auto-vectorize!
}

// FLEXIBLE ARRAY MEMBER (C99): variable-length struct at end
struct Message {
    int length;
    char data[]; // zero-length array — must be last member
};
struct Message *msg = malloc(sizeof(struct Message) + len);
// msg->data is an array of 'len' chars, contiguous with the struct
// Used in: kernel IPC messages, network packets
```

---

## 83. Go — Runtime, GC, and Escape Analysis

```go
// Go manages memory automatically via a garbage collector.
// But understanding what goes where (stack vs heap) is still important.

// ESCAPE ANALYSIS:
// The Go compiler determines at compile time whether a variable can live
// on the stack or must be heap-allocated (escaped to heap).
// Run: go build -gcflags="-m" to see escape analysis output.

package main

func noEscape() int {
    x := 10        // x stays on stack — doesn't escape
    return x       // copy returned, x dies with stack frame
}

func escapes() *int {
    x := 10        // x ESCAPES to heap — its address is returned
    return &x      // caller holds pointer; stack frame will be gone after return
                   // Go: "&x escapes to heap"
}

func escapeToSlice() []int {
    s := make([]int, 3) // backing array may escape to heap (if large or unknown size)
    return s             // Go must keep it alive beyond this function
}

// GARBAGE COLLECTOR (Go 1.x: Tricolor Mark-and-Sweep, Concurrent):
//
// Three phases:
// 1. MARK: starting from GC roots (global vars, stack vars), trace all
//    reachable objects using tricolor algorithm (white/gray/black).
//    - White: not yet seen
//    - Gray:  discovered but children not processed
//    - Black: fully processed (reachable, children scanned)
//
// 2. SWEEP: reclaim all white (unreachable) objects.
//
// Go GC is CONCURRENT: runs mostly alongside your application.
// Short "stop the world" pauses: ~100 microseconds in Go 1.14+.
// (Java CMS/G1: can be milliseconds. Go optimizes for low latency.)
//
// Write barrier: when the program writes a pointer during concurrent GC,
// the write barrier ensures the GC's invariants are maintained.

// GOROUTINE STACKS:
// Each goroutine starts with a 2KB-8KB stack (not a fixed 8MB like OS threads).
// Stacks grow automatically (1.4+: contiguous stack copying).
// This is why Go can have millions of goroutines cheaply.
//
// Mechanism: each function checks if stack needs to grow.
// If not enough: allocate larger stack, COPY all data (including updating pointers).
// This is why you can't store Go stack pointers across stack growth!

// MEMORY MODEL (happens-before):
// Go has a defined memory model for goroutines.
// Without synchronization, concurrent access is undefined behavior.
//
// Primitives that provide synchronization:
//   sync.Mutex, sync.RWMutex
//   channel send/receive
//   sync.Once
//   sync.WaitGroup
//   sync/atomic operations

// sync.Map for concurrent hash map:
import "sync"
var m sync.Map
m.Store("key", 42)
val, ok := m.Load("key")
m.Delete("key")
m.Range(func(k, v any) bool {
    fmt.Println(k, v)
    return true // continue iteration
})
// sync.Map: optimized for read-heavy, writes-to-distinct-keys workloads
// Uses a "read" map (lock-free) + "dirty" map (locked) internally.
```

---

## 84. Rust — Ownership, Borrowing, and Zero-Cost Abstractions

```rust
// Rust's ownership system prevents memory bugs at compile time.
// "Zero-cost abstractions": safe code runs as fast as unsafe code.

// OWNERSHIP RULES:
//   1. Each value has exactly one owner.
//   2. When the owner goes out of scope, the value is dropped (freed).
//   3. There can be either ONE mutable reference OR any number of immutable references.

// MOVE SEMANTICS:
let s1 = String::from("hello"); // s1 owns the String
let s2 = s1;                    // MOVED: s1 is no longer valid
// println!("{}", s1);          // COMPILE ERROR: s1 was moved

// CLONE for explicit copy:
let s3 = s2.clone();            // deep copy — s2 and s3 are independent

// COPY TYPES (cheap, stored entirely on stack):
// i32, f64, bool, char, tuples of Copy types, arrays of Copy types
let x = 5;
let y = x; // COPIED, not moved (i32 is Copy)
println!("{}", x); // still valid

// BORROWING:
fn calculate_length(s: &String) -> usize { // & = immutable borrow
    s.len()
    // s is a reference, not owner. When this function returns, s is just dropped.
    // The String s points to is NOT freed (we don't own it).
}

let s = String::from("hello");
let len = calculate_length(&s); // lend s without giving up ownership
println!("{} has length {}", s, len); // s still valid!

// MUTABLE BORROWING:
fn append_world(s: &mut String) { s.push_str(", world"); }
let mut s = String::from("hello");
append_world(&mut s);
// RULE: Only ONE mutable reference at a time (no data races!)
// let r1 = &mut s;
// let r2 = &mut s; // COMPILE ERROR: cannot borrow `s` as mutable more than once

// LIFETIMES — compiler ensures references don't outlive what they point to:
fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {
    // 'a is a lifetime parameter: "both inputs live at least as long as the output"
    if x.len() > y.len() { x } else { y }
}
// Without lifetime annotation, compiler can't know which input the output borrows from.

// SMART POINTERS:
use std::rc::Rc;   // Reference counted (single thread): Rc<T>
use std::sync::Arc; // Atomic reference count (multi-thread): Arc<T>
use std::cell::RefCell; // Interior mutability (runtime borrow checking): RefCell<T>

// Tree with shared child ownership (impossible with &T alone):
#[derive(Debug)]
struct Tree {
    value: i32,
    children: Vec<Rc<Tree>>,
}

// For concurrent shared ownership:
let shared = Arc::new(42);
let clone1 = Arc::clone(&shared); // increment ref count atomically
let clone2 = Arc::clone(&shared);
// When all Arcs drop: ref count = 0, value freed.

// UNSAFE RUST:
// Some things require unsafe:
// - Dereference raw pointers (*const T, *mut T)
// - Call unsafe functions (FFI, transmute, read volatile)
// - Implement unsafe traits
// - Mutate static variables
// - Access union fields
//
// Unsafe doesn't turn off the borrow checker — it just allows the 5 above.
// Use sparingly and encapsulate in safe abstractions.

unsafe {
    let ptr: *mut i32 = Box::into_raw(Box::new(42)); // manually managed pointer
    *ptr = 100; // unsafe dereference
    println!("{}", *ptr);
    Box::from_raw(ptr); // convert back to Box to free memory
}
```