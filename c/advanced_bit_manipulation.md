# Advanced Bit Manipulation: The Byte as a Bit Array

## Core Mental Model

A single byte of 8 bits is structurally identical to a fixed-size boolean array of length 8.
The only difference is **representation** — instead of `arr[8]`, you have a packed `uint8_t`
where each "element" is 1 bit wide.

```
Byte value:   0   1   0   1   0   0   1   0
Array index:  7   6   5   4   3   2   1   0   ← (LSB = index 0, MSB = index 7)
```

The goal: build a complete "bit-array API" — `get(i)`, `set(i)`, `clear(i)`, `toggle(i)`,
`write(i, v)` — and go well beyond basic shifts to achieve arbitrary in-place transformations
using masks, XOR deltas, and lookup tables.

---

## Part 1 — The Bit-Array API: Core Primitives

All operations use `pos` as a zero-based bit index: `pos=0` is LSB, `pos=7` is MSB.

### 1.1 Read a Bit — `arr[i]`

```c
// C
static inline int bit_get(uint8_t b, int pos) {
    return (b >> pos) & 1;
}
```

```go
// Go
func bitGet(b byte, pos uint) int {
    return int((b >> pos) & 1)
}
```

```rust
// Rust
#[inline(always)]
fn bit_get(b: u8, pos: u8) -> bool {
    (b >> pos) & 1 == 1
}
```

### 1.2 Set a Bit to 1 — `arr[i] = 1`

```c
static inline uint8_t bit_set(uint8_t b, int pos) {
    return b | (1u << pos);
}
```

How it works:

```
b   = 0 1 0 0 0 0 1 0     (0x42)
OR    0 0 1 0 0 0 0 0     (1 << 5 = 0x20)
    = 0 1 1 0 0 0 1 0     (0x62) — bit 5 is now 1
```

### 1.3 Clear a Bit to 0 — `arr[i] = 0`

```c
static inline uint8_t bit_clear(uint8_t b, int pos) {
    return b & ~(1u << pos);
}
```

How it works:

```
b   = 0 1 1 0 0 0 1 0     (0x62)
~(1<<5) = 1 1 0 1 1 1 1 1 (0xDF)
AND   = 0 1 0 0 0 0 1 0   (0x42) — bit 5 is now 0
```

### 1.4 Toggle a Bit — `arr[i] ^= 1`

```c
static inline uint8_t bit_toggle(uint8_t b, int pos) {
    return b ^ (1u << pos);
}
```

XOR flips only the targeted bit; all other bits are unchanged (x XOR 0 = x).

### 1.5 Write Any Value — `arr[i] = v`

```c
static inline uint8_t bit_write(uint8_t b, int pos, int v) {
    uint8_t mask = 1u << pos;
    // Clear the bit slot, then OR in the new value
    return (b & ~mask) | ((v & 1u) << pos);
}
```

### 1.6 Complete C API — Zero Abstraction Cost

```c
typedef uint8_t BitByte;

static inline int      ba_get   (BitByte b, int i)        { return  (b >> i) & 1; }
static inline BitByte  ba_set   (BitByte b, int i)        { return b |  (1u << i); }
static inline BitByte  ba_clear (BitByte b, int i)        { return b & ~(1u << i); }
static inline BitByte  ba_toggle(BitByte b, int i)        { return b ^  (1u << i); }
static inline BitByte  ba_write (BitByte b, int i, int v) {
    uint8_t m = 1u << i;
    return (b & ~m) | ((v & 1u) << i);
}
```

These inline functions compile to **one or two CPU instructions** each. They are the
complete "array element" interface for a single byte.

---

## Part 2 — Eliminating Runtime Shifts: Pre-Computed Mask Tables

The shift in `1u << pos` happens at runtime when `pos` is a variable. You can eliminate all
runtime shift instructions by using a precomputed table — just an array lookup instead.

### 2.1 Mask Table (8 entries, constant memory)

```c
// Precomputed once. BIT_MASK[i] = (1u << i).
static const uint8_t BIT_MASK[8] = {
    0x01,   // BIT_MASK[0] = 0000 0001
    0x02,   // BIT_MASK[1] = 0000 0010
    0x04,   // BIT_MASK[2] = 0000 0100
    0x08,   // BIT_MASK[3] = 0000 1000
    0x10,   // BIT_MASK[4] = 0001 0000
    0x20,   // BIT_MASK[5] = 0010 0000
    0x40,   // BIT_MASK[6] = 0100 0000
    0x80,   // BIT_MASK[7] = 1000 0000
};

// Inverted masks for bit-clear operations.
static const uint8_t BIT_NMASK[8] = {
    0xFE, 0xFD, 0xFB, 0xF7, 0xEF, 0xDF, 0xBF, 0x7F
};

// All ops below: zero runtime shifts, one array read + one bitwise op.
static inline int     ba_get_m   (uint8_t b, int i) { return !!(b & BIT_MASK[i]); }
static inline uint8_t ba_set_m   (uint8_t b, int i) { return b |  BIT_MASK[i];  }
static inline uint8_t ba_clear_m (uint8_t b, int i) { return b &  BIT_NMASK[i]; }
static inline uint8_t ba_toggle_m(uint8_t b, int i) { return b ^  BIT_MASK[i];  }
```

### 2.2 Named Compile-Time Constants (no shift in generated binary)

When `pos` is a **compile-time constant**, the compiler evaluates `1u << pos` at compile
time and emits a literal byte constant — zero shift instructions in the output binary.

```c
#define BIT0  0x01u   // 0000 0001
#define BIT1  0x02u   // 0000 0010
#define BIT2  0x04u   // 0000 0100
#define BIT3  0x08u   // 0000 1000
#define BIT4  0x10u   // 0001 0000
#define BIT5  0x20u   // 0010 0000
#define BIT6  0x40u   // 0100 0000
#define BIT7  0x80u   // 1000 0000

// All pure AND / OR / XOR — no shift instruction emitted by compiler:
uint8_t reg = 0x52;
reg |=  BIT5;               // Set bit 5
reg &= ~BIT3;               // Clear bit 3
reg ^=  (BIT1 | BIT7);      // Toggle bits 1 and 7 simultaneously
```

---

## Part 3 — Whole-Byte Transformations Without Shifts

These are the most powerful patterns when you need to take a byte from one bit pattern
to another, independently of individual bit positions.

### 3.1 XOR Delta Mask — The Fastest Fixed Transformation

**Core insight**: the difference between any two bit patterns is a single XOR mask.
Compute the mask once; apply it in one instruction with no shifts.

```
Example:
  Input:   0 1 0 1 0 0 1 0   (0x52)
  Output:  1 0 1 1 0 1 0 0   (0xB4)

  Delta = Input XOR Output:
           1 1 1 0 0 1 1 0   (0xE6) ← positions that differ
```

```c
// Compute delta offline: delta = 0x52 ^ 0xB4 = 0xE6
#define TRANSFORM_DELTA  0xE6u

// Apply at runtime: one XOR, zero shifts, zero branches.
uint8_t result = input ^ TRANSFORM_DELTA;
```

This generalizes perfectly: **any set of bit flips** — toggling arbitrary positions to reach
a target pattern — collapses to a single XOR with the precomputed delta mask.

### 3.2 AND/OR Composition — Force Specific Bits Unconditionally

When you need to force certain bits to 1 and others to 0 regardless of their current value:

```c
uint8_t force_transform(uint8_t b) {
    b |=  0x94u;   // Force bits 7, 4, 2 HIGH:  1001 0100
    b &= ~0x62u;   // Force bits 6, 5, 1 LOW:   0110 0010
    return b;      // Bits not in either mask retain their original value
}
```

No shifts. No branches. Two bitwise instructions.

### 3.3 Lookup Table — O(1) Arbitrary Byte Transformation

The most general technique. For any function `f: uint8_t → uint8_t`, precompute all 256
outputs once, then apply in a single memory read with no shifts, no arithmetic, no branches.

```c
// One-time setup: build the full 256-entry transformation table.
static uint8_t g_lut[256];

void build_transform_lut(void) {
    for (int i = 0; i < 256; i++) {
        // Define any transformation here — bit reversal, nibble swap, custom permutation, etc.
        g_lut[i] = compute_my_transform((uint8_t)i);
    }
}

// Hot path at runtime: pure table lookup. O(1). Zero shifts. Zero arithmetic.
static inline uint8_t fast_transform(uint8_t b) {
    return g_lut[b];
}
```

This technique powers AES S-boxes, CRC tables, Hamming code lookups, and high-speed protocol
decoders. At 256 bytes, the entire table fits in a single cache line cluster and incurs
essentially zero runtime cost.

---

## Part 4 — Advanced In-Place Patterns

### 4.1 Swap Two Bits at Arbitrary Positions

```c
// Readable version
uint8_t bit_swap(uint8_t b, int i, int j) {
    int bi = (b >> i) & 1;
    int bj = (b >> j) & 1;
    if (bi ^ bj) {                        // Only act if they differ
        b ^= (1u << i) | (1u << j);      // Toggle both simultaneously
    }
    return b;
}

// Branchless version — no conditional jump
uint8_t bit_swap_branchless(uint8_t b, int i, int j) {
    uint8_t diff = ((b >> i) ^ (b >> j)) & 1u;   // 1 if bits differ, 0 if equal
    uint8_t mask = (diff << i) | (diff << j);
    return b ^ mask;                              // XOR toggles only if bits differ
}
```

### 4.2 Bit Reversal — Divide and Conquer (O(1), No Loop)

Reverse all 8 bits with exactly 3 mask-and-shift pairs:

```c
uint8_t bit_reverse(uint8_t b) {
    // Step 1: Swap upper nibble ↔ lower nibble
    b = ((b & 0xF0u) >> 4) | ((b & 0x0Fu) << 4);
    //          ^^^^^ select top 4          ^^^^^ select bottom 4

    // Step 2: Swap 2-bit groups within each nibble
    b = ((b & 0xCCu) >> 2) | ((b & 0x33u) << 2);
    //          ^^^^^ select pairs at [7:6,3:2]   ^^^^^ pairs at [5:4,1:0]

    // Step 3: Swap adjacent individual bits
    b = ((b & 0xAAu) >> 1) | ((b & 0x55u) << 1);
    //          ^^^^^ odd positions (7,5,3,1)      ^^^^^ even positions (6,4,2,0)
    return b;
}
```

Mask anatomy:

```
0xF0 = 1111 0000   upper nibble selector
0x0F = 0000 1111   lower nibble selector
0xCC = 1100 1100   upper-pair selector within each nibble
0x33 = 0011 0011   lower-pair selector
0xAA = 1010 1010   odd-position bits  (7, 5, 3, 1)
0x55 = 0101 0101   even-position bits (6, 4, 2, 0)
```

Each step halves the swap granularity: nibble → pair → single bit.

### 4.3 Bit Rotation — Circular Shift

```c
// Left rotate: MSB wraps around to LSB
uint8_t rotate_left(uint8_t b, int n) {
    n &= 7;                          // Clamp to [0, 7]
    return (b << n) | (b >> (8 - n));
}

// Right rotate: LSB wraps around to MSB
uint8_t rotate_right(uint8_t b, int n) {
    n &= 7;
    return (b >> n) | (b << (8 - n));
}

// Example: rotate_left(0b01010010, 2)
// Left:  0b01001000  (shift part)
// Right: 0b00000001  (wrap part)
// OR:    0b01001001
```

### 4.4 Bit-Field Extraction and Insertion

These are the "sub-array slice" operations for bits.

```c
// Extract `len` bits starting at `start` — like arr[start : start+len]
// Example: bits_extract(0b11010110, start=2, len=3) → 0b101
static inline uint8_t bits_extract(uint8_t b, int start, int len) {
    uint8_t mask = (1u << len) - 1u;   // e.g., len=3 → mask=0b00000111
    return (b >> start) & mask;
}

// Insert `val` into bits [start, start+len) of b — like arr[start:start+len] = val
// Example: bits_insert(0b11111111, start=2, len=3, val=0b101) → 0b11110111
static inline uint8_t bits_insert(uint8_t b, int start, int len, uint8_t val) {
    uint8_t mask    = ((1u << len) - 1u) << start;   // Positioned field mask
    uint8_t shifted = (val << start) & mask;          // Value positioned into field
    return (b & ~mask) | shifted;                     // Clear field, insert value
}
```

### 4.5 Branchless Conditional Bit Write

Writing a bit to a specific value without any branch is important in hot paths and
constant-time cryptographic code:

```c
// Method 1: Arithmetic negation trick
// -1 (0xFF...FF) when v=1; 0 when v=0 — works in two's complement
uint8_t bit_write_branchless(uint8_t b, int pos, int v) {
    uint8_t mask = 1u << pos;
    return (b & ~mask) | (((uint8_t)-(uint8_t)v) & mask);
}

// Method 2: Direct cast (clean when v is already 0 or 1)
uint8_t bit_write_direct(uint8_t b, int pos, int v) {
    return (b & ~(1u << pos)) | ((uint8_t)(v & 1) << pos);
}
```

### 4.6 Copy a Bit from One Position to Another

```c
// Copy the bit at position `src` into position `dst`
uint8_t bit_copy(uint8_t b, int src, int dst) {
    int v = (b >> src) & 1;
    uint8_t mask = 1u << dst;
    return (b & ~mask) | ((uint8_t)v << dst);
}
```

### 4.7 Population Count (Count Set Bits)

```c
// Method 1: Kernighan's loop — O(set_bits), each iteration clears lowest set bit
int popcount_kernighan(uint8_t b) {
    int count = 0;
    while (b) {
        b &= b - 1u;    // Clears the lowest set bit: 0110 1000 → 0110 0000
        count++;
    }
    return count;
}

// Method 2: Parallel bit summation — O(1), no loop, pure arithmetic
int popcount_parallel(uint8_t b) {
    b = b - ((b >> 1) & 0x55u);              // 2-bit partial sums
    b = (b & 0x33u) + ((b >> 2) & 0x33u);   // 4-bit partial sums
    b = (b + (b >> 4)) & 0x0Fu;             // 8-bit total sum
    return (int)b;
}

// Method 3: Compiler/hardware intrinsic (preferred in production)
int popcount_hw(uint8_t b) {
    return __builtin_popcount((unsigned)b);  // GCC/Clang → POPCNT instruction
}
```

### 4.8 Bit Isolation and Lowest-Set-Bit Tricks

```c
// Isolate only the lowest set bit (all others zeroed)
// 0b10110100 → 0b00000100
uint8_t isolate_lowest(uint8_t b) {
    return b & (uint8_t)(-b);    // Two's complement negation
}

// Clear only the lowest set bit
// 0b10110100 → 0b10110000
uint8_t clear_lowest(uint8_t b) {
    return b & (b - 1u);
}

// Smear bits from lowest set bit down to bit 0 (fill trailing zeros with ones)
// 0b10110100 → 0b10110111
uint8_t smear_lowest(uint8_t b) {
    return b | (b - 1u);
}

// Check if exactly one bit is set (power of two check)
int is_single_bit_set(uint8_t b) {
    return b != 0 && (b & (b - 1u)) == 0;
}

// Count trailing zeros (position of lowest set bit)
int ctz(uint8_t b) {
    return __builtin_ctz((unsigned)b);  // GCC/Clang
}
```

---

## Part 5 — C Bit Fields: Struct-Based Bit Array

C's `union` pattern gives you named field access to individual bits, making the byte look
exactly like a struct array:

```c
#include <stdint.h>

typedef union {
    struct {
        uint8_t b0 : 1;  // LSB (array index 0)
        uint8_t b1 : 1;
        uint8_t b2 : 1;
        uint8_t b3 : 1;
        uint8_t b4 : 1;
        uint8_t b5 : 1;
        uint8_t b6 : 1;
        uint8_t b7 : 1;  // MSB (array index 7)
    } bits;
    uint8_t raw;
} BitByte;

// Usage — reads and writes look like array element access:
BitByte x;
x.raw = 0x52;        // Load: 0101 0010
x.bits.b7 = 1;      // Set MSB: arr[7] = 1
x.bits.b1 = 0;      // Clear bit 1: arr[1] = 0
int v = x.bits.b4;  // Read bit 4: v = arr[4]
printf("0x%02X\n", x.raw);

// Hardware register modeling (common in embedded/kernel code):
typedef union {
    struct {
        uint8_t tx_ready    : 1;  // bit 0
        uint8_t rx_ready    : 1;  // bit 1
        uint8_t parity_err  : 1;  // bit 2
        uint8_t frame_err   : 1;  // bit 3
        uint8_t overflow    : 1;  // bit 4
        uint8_t reserved    : 3;  // bits 5-7
    };
    uint8_t raw;
} UARTStatusReg;
```

**Important caveat**: bit-field layout within a byte is **implementation-defined** in C —
endianness and padding are compiler-dependent. This is safe for in-process manipulation
but **not portable across ABIs** for serialization or network protocols.

---

## Part 6 — SWAR: The 64-bit Register as a Bit Array

SWAR (SIMD Within A Register) treats a wide integer as a packed array of sub-word values.
A `uint64_t` holds 8 bytes; all 8 can be manipulated independently in parallel with no SIMD
intrinsics — just integer operations.

### 6.1 Toggle a Bit in Every Byte Simultaneously

```c
// Toggle bit `pos` across all 8 bytes of a 64-bit word — one XOR, 8 bytes updated
uint64_t swar_toggle_bit_all_bytes(uint64_t word, int pos) {
    // Broadcast the per-byte mask to all 8 byte lanes
    uint64_t mask = 0x0101010101010101ULL << pos;
    return word ^ mask;
}
```

### 6.2 Parallel Population Count Across 8 Bytes

```c
// Count set bits in each of the 8 bytes independently — no loop, pure arithmetic
uint64_t swar_popcount8(uint64_t b) {
    // Parallel 2-bit sums
    b = b - ((b >> 1) & 0x5555555555555555ULL);
    // Parallel 4-bit sums
    b = (b & 0x3333333333333333ULL) + ((b >> 2) & 0x3333333333333333ULL);
    // Parallel 8-bit sums — each byte now holds its own bit count (0–8)
    b = (b + (b >> 4)) & 0x0F0F0F0F0F0F0F0FULL;
    return b;
}
// Grand total across all 8 bytes:
uint64_t swar_total_popcount(uint64_t b) {
    b = swar_popcount8(b);
    return (b * 0x0101010101010101ULL) >> 56;  // Horizontal sum via multiply
}
```

### 6.3 Apply a Per-Byte Bit Mask Across a Buffer

```c
// OR a mask into every byte of a 8-byte word in one operation
uint64_t swar_set_bit_all(uint64_t word, int pos) {
    uint64_t mask = 0x0101010101010101ULL << pos;
    return word | mask;
}

// Clear a bit in every byte simultaneously
uint64_t swar_clear_bit_all(uint64_t word, int pos) {
    uint64_t mask = 0x0101010101010101ULL << pos;
    return word & ~mask;
}
```

SWAR reduces a loop over 8 bytes into a single 64-bit operation — an 8x throughput gain
with no SIMD instruction set requirements.

---

## Part 7 — Rust: The Bit-Array API as a Type

```rust
/// Zero-cost wrapper around u8 exposing a complete bit-array interface.
/// All methods are `#[inline(always)]` — they compile to single instructions.
#[derive(Copy, Clone, Debug, PartialEq, Eq)]
pub struct BitByte(u8);

impl BitByte {
    pub fn new(raw: u8) -> Self { Self(raw) }
    pub fn raw(self) -> u8 { self.0 }

    #[inline(always)] pub fn get(self, pos: u8) -> bool {
        (self.0 >> pos) & 1 == 1
    }
    #[inline(always)] pub fn set(self, pos: u8) -> Self {
        Self(self.0 | (1 << pos))
    }
    #[inline(always)] pub fn clear(self, pos: u8) -> Self {
        Self(self.0 & !(1u8 << pos))
    }
    #[inline(always)] pub fn toggle(self, pos: u8) -> Self {
        Self(self.0 ^ (1 << pos))
    }
    #[inline(always)] pub fn write(self, pos: u8, v: bool) -> Self {
        let mask = 1u8 << pos;
        Self((self.0 & !mask) | ((v as u8) << pos))
    }

    // Standard library provides these as intrinsics on u8:
    #[inline(always)] pub fn reverse(self) -> Self    { Self(self.0.reverse_bits()) }
    #[inline(always)] pub fn popcount(self) -> u32    { self.0.count_ones() }
    #[inline(always)] pub fn ctz(self) -> u32         { self.0.trailing_zeros() }
    #[inline(always)] pub fn clz(self) -> u32         { self.0.leading_zeros() }
    #[inline(always)] pub fn rotate_left(self, n: u32) -> Self {
        Self(self.0.rotate_left(n))   // Wraps correctly: u8 rotation is mod 8
    }
    #[inline(always)] pub fn rotate_right(self, n: u32) -> Self {
        Self(self.0.rotate_right(n))
    }

    // Apply an XOR delta (like the 0x52 → 0xB4 example: delta = 0xE6)
    #[inline(always)] pub fn apply_delta(self, delta: u8) -> Self {
        Self(self.0 ^ delta)
    }

    // Extract a bit field [start, start+len)
    pub fn extract(self, start: u8, len: u8) -> u8 {
        let mask = (1u8 << len).wrapping_sub(1);
        (self.0 >> start) & mask
    }

    // Insert a value into a bit field
    pub fn insert(self, start: u8, len: u8, val: u8) -> Self {
        let mask = ((1u8 << len).wrapping_sub(1)) << start;
        Self((self.0 & !mask) | ((val << start) & mask))
    }
}

fn example() {
    let b = BitByte::new(0x52);                   // 0101 0010
    let b = b.set(7).clear(6).toggle(4);
    let b = b.apply_delta(0xE6);                  // XOR transform: 0x52 → 0xB4
    println!("{:08b}", b.raw());
}
```

---

## Part 8 — Go: Bit-Array API as a Method Set

```go
package bitbyte

import "math/bits"

// Byte is a uint8 with a complete bit-array method set.
type Byte uint8

// Get returns the bit at position pos as 0 or 1.
func (b Byte) Get(pos uint) int { return int((b >> pos) & 1) }

// Set forces bit at pos to 1.
func (b Byte) Set(pos uint) Byte { return b | (1 << pos) }

// Clear forces bit at pos to 0.
// &^ is Go's bit-clear (AND NOT) operator.
func (b Byte) Clear(pos uint) Byte { return b &^ (1 << pos) }

// Toggle flips the bit at pos.
func (b Byte) Toggle(pos uint) Byte { return b ^ (1 << pos) }

// Write sets bit at pos to v.
func (b Byte) Write(pos uint, v bool) Byte {
    if v {
        return b.Set(pos)
    }
    return b.Clear(pos)
}

// Reverse returns the bit-reversed byte. Uses math/bits intrinsic.
func (b Byte) Reverse() Byte { return Byte(bits.Reverse8(uint8(b))) }

// Popcount returns the number of set bits.
func (b Byte) Popcount() int { return bits.OnesCount8(uint8(b)) }

// RotateLeft rotates bits left by n positions.
func (b Byte) RotateLeft(n int) Byte {
    n &= 7
    return (b << uint(n)) | (b >> uint(8-n))
}

// ApplyDelta applies an XOR delta mask.
// Example: Byte(0x52).ApplyDelta(0xE6) == 0xB4
func (b Byte) ApplyDelta(delta Byte) Byte { return b ^ delta }

// Extract returns the `len`-bit field starting at bit `start`.
func (b Byte) Extract(start, length uint) Byte {
    mask := Byte((1 << length) - 1)
    return (b >> start) & mask
}
```

---

## Part 9 — The Shift-Free Principle: A Mental Reframe

Shifts appear in the **general** formulas because `1u << pos` is how a mask is built from
a runtime index. The key insight is: **decouple mask construction from the manipulation phase**.

```
Shift-dependent form:   result = b | (1u << pos)     ← shift at runtime
Shift-free form:        result = b | BIT_MASK[pos]   ← array read at runtime, shift was at init
```

| Situation | Technique | Shift Eliminated? |
|-----------|-----------|-------------------|
| `pos` is a compile-time constant | `1u << 3` → compiler folds to `0x08` | Yes — no instruction |
| `pos` is a runtime variable | Mask table `BIT_MASK[pos]` | Yes — array read replaces shift |
| Toggle a fixed set of bits | `b ^= DELTA_MASK` (one XOR) | Yes — no shift involved |
| Arbitrary byte → byte mapping | 256-entry LUT: `lut[b]` | Yes — one memory read |
| Bit reversal, rotation | Shift unavoidable — it is the operation | Not applicable |

The principle: **shift once (at table-build time), apply zero times (at runtime)**.
This is the same philosophy behind precomputed exponentiation tables in cryptography and
CRC polynomial tables in networking.

---

## Part 10 — Useful Mask Reference

```
Single-bit masks:
  0x01 = 0000 0001   0x02 = 0000 0010   0x04 = 0000 0100   0x08 = 0000 1000
  0x10 = 0001 0000   0x20 = 0010 0000   0x40 = 0100 0000   0x80 = 1000 0000

Group masks:
  0xFF = 1111 1111   all bits set
  0x00 = 0000 0000   all bits clear
  0xF0 = 1111 0000   upper nibble
  0x0F = 0000 1111   lower nibble
  0xAA = 1010 1010   odd-indexed bits  (7, 5, 3, 1)
  0x55 = 0101 0101   even-indexed bits (6, 4, 2, 0)
  0xCC = 1100 1100   upper-pair of each nibble (7,6 and 3,2)
  0x33 = 0011 0011   lower-pair of each nibble (5,4 and 1,0)

Complement rule:
  ~0xF0 = 0x0F   ~0xAA = 0x55   ~0xCC = 0x33   (inverts selection)

Two's complement tricks:
  b & -b           → isolate lowest set bit
  b & (b - 1)      → clear lowest set bit
  b | (b - 1)      → smear lowest set bit down through trailing zeros
  b == (b & -b)    → true if exactly one bit is set (power-of-two test)
```

---

## Summary

| Operation | Code Pattern | Shift-Free? |
|-----------|-------------|-------------|
| Read bit i | `(b >> i) & 1` or `BIT_MASK[i] & b` | With table: Yes |
| Set bit i | `b \| (1 << i)` or `b \| BIT_MASK[i]` | With table: Yes |
| Clear bit i | `b & ~(1 << i)` or `b & BIT_NMASK[i]` | With table: Yes |
| Toggle bit i | `b ^ (1 << i)` or `b ^ BIT_MASK[i]` | With table: Yes |
| Fixed bit-flip transform | `b ^ DELTA_MASK` | Yes |
| Any byte transform | `lut[b]` | Yes |
| Swap bits i, j | XOR-swap trick | Mostly |
| Reverse all bits | Divide-and-conquer masks | No (shifts are the op) |
| Rotate left/right | Dual-shift + OR | No (shifts are the op) |
| Extract bit field | `(b >> start) & field_mask` | Partially |
| Parallel ops on 8 bytes | SWAR with broadcast mask | Minimal |
