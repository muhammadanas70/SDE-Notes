# Bit Manipulation Guide — Go · Rust · C
*For DSA & System Programming*

---

## Table of Contents
1. [Foundations](#1-foundations)
2. [Core Operations](#2-core-operations)
3. [Arithmetic Tricks](#3-arithmetic-tricks)
4. [DSA Patterns](#4-dsa-patterns)
5. [System Programming Patterns](#5-system-programming-patterns)
6. [Quick Reference Cheat Sheet](#6-quick-reference-cheat-sheet)

---

## 1. Foundations

### Binary Representation

| Decimal | Binary (8-bit) | Hex  |
|---------|---------------|------|
| 0       | 0000 0000     | 0x00 |
| 1       | 0000 0001     | 0x01 |
| 127     | 0111 1111     | 0x7F |
| 128     | 1000 0000     | 0x80 |
| 255     | 1111 1111     | 0xFF |
| -1      | 1111 1111     | 0xFF (two's complement) |
| -128    | 1000 0000     | 0x80 (two's complement) |

### Two's Complement (Signed Integers)
```
Positive: stored as-is
Negative: flip all bits, then add 1

Example: -5 in 8-bit
  5  = 0000 0101
 ~5  = 1111 1010   (flip all bits)
 -5  = 1111 1011   (add 1)
```

### Operator Table

| Operator | Symbol | Go | Rust | C    | Description              |
|----------|--------|----|------|------|--------------------------|
| AND      | `&`    | `&`| `&`  | `&`  | 1 only if both are 1     |
| OR       | `\|`   | `\|`| `\|` | `\|` | 1 if either is 1         |
| XOR      | `^`    | `^`| `^`  | `^`  | 1 if bits differ         |
| NOT      | `~`    | `^` (unary) | `!` | `~` | Flip all bits   |
| Left shift  | `<<` | `<<` | `<<` | `<<` | Multiply by 2^n      |
| Right shift | `>>` | `>>` | `>>` | `>>` | Divide by 2^n (arithmetic for signed) |

> **Go quirk**: Unary NOT is `^x`, not `~x`. Bitwise XOR is also `^`.
> **Rust quirk**: Unary NOT for integers is `!x`, not `~x`.

---

## 2. Core Operations

### 2.1 Get, Set, Clear, Toggle a Bit

```c
// ─── C ───────────────────────────────────────────────────────────────────────
#include <stdio.h>
#include <stdint.h>

// Bit position: 0 = LSB (rightmost)
int  get_bit   (uint32_t n, int pos) { return (n >> pos) & 1; }
uint32_t set_bit   (uint32_t n, int pos) { return n | (1u << pos); }
uint32_t clear_bit (uint32_t n, int pos) { return n & ~(1u << pos); }
uint32_t toggle_bit(uint32_t n, int pos) { return n ^ (1u << pos); }

// Set bit to a specific value (0 or 1)
uint32_t assign_bit(uint32_t n, int pos, int val) {
    return (n & ~(1u << pos)) | ((uint32_t)val << pos);
}

int main() {
    uint32_t n = 0b1010;   // = 10
    printf("get bit 1: %d\n",  get_bit(n, 1));     // 1
    printf("set bit 0: %u\n",  set_bit(n, 0));     // 11 = 0b1011
    printf("clear bit 1: %u\n",clear_bit(n, 1));   // 8  = 0b1000
    printf("toggle bit 3: %u\n",toggle_bit(n, 3)); // 2  = 0b0010
}
```

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
package main

import "fmt"

func getBit   (n uint32, pos int) int    { return int((n >> pos) & 1) }
func setBit   (n uint32, pos int) uint32 { return n | (1 << pos) }
func clearBit (n uint32, pos int) uint32 { return n & ^(1 << pos) }
func toggleBit(n uint32, pos int) uint32 { return n ^ (1 << pos) }

func main() {
    var n uint32 = 0b1010 // 10
    fmt.Println(getBit(n, 1))    // 1
    fmt.Println(setBit(n, 0))    // 11
    fmt.Println(clearBit(n, 1))  // 8
    fmt.Println(toggleBit(n, 3)) // 2
}
```

```rust
// ─── Rust ────────────────────────────────────────────────────────────────────
fn get_bit   (n: u32, pos: u32) -> u32 { (n >> pos) & 1 }
fn set_bit   (n: u32, pos: u32) -> u32 { n | (1 << pos) }
fn clear_bit (n: u32, pos: u32) -> u32 { n & !(1 << pos) }
fn toggle_bit(n: u32, pos: u32) -> u32 { n ^ (1 << pos) }

fn main() {
    let n: u32 = 0b1010; // 10
    println!("{}", get_bit(n, 1));    // 1
    println!("{}", set_bit(n, 0));    // 11
    println!("{}", clear_bit(n, 1)); // 8
    println!("{}", toggle_bit(n, 3));// 2
}
```

---

### 2.2 Count Set Bits (Popcount)

**Naive — O(log n)**
```c
int popcount_naive(uint32_t n) {
    int count = 0;
    while (n) { count += n & 1; n >>= 1; }
    return count;
}
```

**Brian Kernighan's trick — O(k) where k = set bits**
The key insight: `n & (n-1)` clears the lowest set bit.
```c
int popcount_kernighan(uint32_t n) {
    int count = 0;
    while (n) { n &= n - 1; count++; }
    return count;
}
```

**Using hardware/builtins (fastest)**
```c
// C — GCC/Clang
int c = __builtin_popcount(n);      // 32-bit
int c = __builtin_popcountll(n);    // 64-bit
```

```go
// Go — math/bits
import "math/bits"
bits.OnesCount32(n)   // 32-bit
bits.OnesCount64(n)   // 64-bit
bits.OnesCount(n)     // native uint
```

```rust
// Rust — built into all integer types
n.count_ones()        // u32, u64, etc.
n.count_zeros()
```

---

### 2.3 Lowest & Highest Set Bit

```c
// ─── C ───────────────────────────────────────────────────────────────────────
// Lowest set bit (LSB isolation)
uint32_t lowest_set_bit(uint32_t n) { return n & (-n); }
// e.g. n=12=1100 → -n=...0100 → n & -n = 0100 = 4

// Clear lowest set bit
uint32_t clear_lowest(uint32_t n) { return n & (n - 1); }

// Position of lowest set bit (0-indexed)
int lsb_pos(uint32_t n) { return __builtin_ctz(n); }  // count trailing zeros

// Position of highest set bit
int msb_pos(uint32_t n) { return 31 - __builtin_clz(n); } // count leading zeros
```

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
import "math/bits"

lowestSetBit  := n & (-n)
clearLowest   := n & (n - 1)
lsbPos        := bits.TrailingZeros32(n)
msbPos        := 31 - bits.LeadingZeros32(n)
```

```rust
// ─── Rust ────────────────────────────────────────────────────────────────────
let lowest_set_bit = n & n.wrapping_neg();
let clear_lowest   = n & (n - 1);
let lsb_pos        = n.trailing_zeros();
let msb_pos        = 31 - n.leading_zeros();
```

---

### 2.4 Check, Next & Previous Power of Two

```c
// ─── C ───────────────────────────────────────────────────────────────────────
int   is_power_of_two  (uint32_t n) { return n > 0 && (n & (n-1)) == 0; }
// Why? Powers of 2 have exactly one bit set. n-1 flips all lower bits.
// 8 = 1000, 7 = 0111 → 8 & 7 = 0

uint32_t next_power_of_two(uint32_t n) {
    if (n == 0) return 1;
    n--;
    n |= n >> 1; n |= n >> 2; n |= n >> 4;
    n |= n >> 8; n |= n >> 16;
    return n + 1;
}
```

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
isPowerOfTwo    := n > 0 && (n & (n-1)) == 0
nextPowerOfTwo  := uint32(1) << bits.Len32(n)  // bits.Len32 = floor(log2)+1
```

```rust
// ─── Rust ────────────────────────────────────────────────────────────────────
let is_pow2     = n > 0 && n.is_power_of_two(); // built-in!
let next_pow2   = n.next_power_of_two();         // built-in!
```

---

### 2.5 Swapping and XOR Tricks

```c
// Swap without temp (XOR swap) — works when a != b
void xor_swap(int *a, int *b) {
    *a ^= *b;
    *b ^= *a;
    *a ^= *b;
}
// Note: prefer tmp variable in practice; XOR swap is UB if a == b (same address)

// Conditional swap: swap if a > b (branchless)
void cond_swap(int *a, int *b) {
    int diff = (*a - *b) & -((*a - *b) >> 31); // only works for small differences
    *a -= diff; *b += diff;
}
```

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
a, b = a^b, a^b^(a^b) // equivalent, but just use: a, b = b, a
```

```rust
// ─── Rust ────────────────────────────────────────────────────────────────────
std::mem::swap(&mut a, &mut b); // idiomatic
```

---

## 3. Arithmetic Tricks

### 3.1 Multiply / Divide by Powers of 2

```c
// Left shift  = multiply by 2^n
x << 3   // x * 8
x << 10  // x * 1024

// Right shift = floor divide by 2^n
x >> 3   // x / 8 (arithmetic shift for signed, logical for unsigned)

// Modulo by power of 2 (only works when divisor is 2^n)
x & (m - 1)   // x % m, when m is power of 2
// e.g. x & 7 == x % 8
```

### 3.2 Absolute Value (Branchless)

```c
// Works on 32-bit signed int
int abs_branchless(int n) {
    int mask = n >> 31;     // arithmetic shift: all 1s if negative, all 0s if positive
    return (n + mask) ^ mask;
}
```

### 3.3 Sign Detection

```c
int is_negative(int n) { return (n >> 31) & 1; }
int sign(int n)        { return (n >> 31) | ((-n) >> 31); } // -1, 0, +1
```

### 3.4 Efficient Modular Arithmetic

```c
// When buffer/table size is always a power of 2:
#define BUFFER_SIZE 1024   // must be power of 2
int idx = (idx + 1) & (BUFFER_SIZE - 1);  // wrap-around, no branch
```

---

## 4. DSA Patterns

### 4.1 Bitmask Subset Enumeration

**Enumerate all subsets of a set of n elements — O(2^n)**

```c
// ─── C ───────────────────────────────────────────────────────────────────────
// Each integer 0..(1<<n)-1 represents a subset via its bits
void enumerate_subsets(int n) {
    for (int mask = 0; mask < (1 << n); mask++) {
        printf("Subset: ");
        for (int i = 0; i < n; i++) {
            if (mask & (1 << i)) printf("%d ", i);
        }
        printf("(mask=%d)\n", mask);
    }
}

// Enumerate all subsets OF a given mask (e.g., find subsets of subset)
void subsets_of_mask(int mask) {
    for (int sub = mask; sub > 0; sub = (sub - 1) & mask) {
        printf("sub-mask: %d\n", sub);
    }
}
```

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
func enumerateSubsets(n int) {
    for mask := 0; mask < (1 << n); mask++ {
        subset := []int{}
        for i := 0; i < n; i++ {
            if mask&(1<<i) != 0 {
                subset = append(subset, i)
            }
        }
        fmt.Println(mask, "->", subset)
    }
}
```

```rust
// ─── Rust ────────────────────────────────────────────────────────────────────
fn enumerate_subsets(n: u32) {
    for mask in 0..(1u32 << n) {
        let subset: Vec<u32> = (0..n).filter(|&i| mask & (1 << i) != 0).collect();
        println!("{:b} -> {:?}", mask, subset);
    }
}
```

---

### 4.2 XOR — Find the Unique Element

```
Problem: Every element appears twice except one. Find the unique one.
Key insight: a ^ a = 0, a ^ 0 = a, XOR is commutative & associative.
```

```c
// ─── C ───────────────────────────────────────────────────────────────────────
int single_number(int* nums, int n) {
    int result = 0;
    for (int i = 0; i < n; i++) result ^= nums[i];
    return result;
}

// Two unique elements (all others appear twice)
void two_unique(int* nums, int n, int* a, int* b) {
    int xorAll = 0;
    for (int i = 0; i < n; i++) xorAll ^= nums[i];
    // xorAll = a ^ b; find any differing bit
    int diff_bit = xorAll & (-xorAll);  // lowest set bit
    *a = 0; *b = 0;
    for (int i = 0; i < n; i++) {
        if (nums[i] & diff_bit) *a ^= nums[i];
        else                    *b ^= nums[i];
    }
}
```

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
func singleNumber(nums []int) int {
    result := 0
    for _, v := range nums { result ^= v }
    return result
}
```

```rust
// ─── Rust ────────────────────────────────────────────────────────────────────
fn single_number(nums: &[i32]) -> i32 {
    nums.iter().fold(0, |acc, &x| acc ^ x)
}
```

---

### 4.3 Find Missing Number (0 to n)

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
// XOR of 0..n XOR'd with all array elements cancels duplicates
func missingNumber(nums []int) int {
    xor := 0
    for i, v := range nums {
        xor ^= i ^ v
    }
    return xor ^ len(nums)
}
```

```rust
// ─── Rust ────────────────────────────────────────────────────────────────────
fn missing_number(nums: &[i32]) -> i32 {
    let n = nums.len() as i32;
    (0..=n).fold(0, |acc, x| acc ^ x) ^ nums.iter().fold(0, |acc, &x| acc ^ x)
}
```

---

### 4.4 Bitmask DP (Travelling Salesman / Assignment)

```c
// ─── C ───────────────────────────────────────────────────────────────────────
// TSP with bitmask DP — O(n^2 * 2^n)
// dp[mask][i] = min cost to visit all cities in mask, ending at city i

#define INF 1e9
#define MAXN 20

double dist[MAXN][MAXN];
double dp[1 << MAXN][MAXN];

double tsp(int n) {
    int full = (1 << n) - 1;
    for (int mask = 0; mask <= full; mask++)
        for (int i = 0; i < n; i++)
            dp[mask][i] = INF;

    dp[1][0] = 0; // start at city 0

    for (int mask = 1; mask <= full; mask++) {
        for (int u = 0; u < n; u++) {
            if (!(mask & (1 << u))) continue;       // u not in mask
            if (dp[mask][u] == INF) continue;
            for (int v = 0; v < n; v++) {
                if (mask & (1 << v)) continue;      // v already visited
                int newMask = mask | (1 << v);
                double newCost = dp[mask][u] + dist[u][v];
                if (newCost < dp[newMask][v])
                    dp[newMask][v] = newCost;
            }
        }
    }

    double ans = INF;
    for (int i = 1; i < n; i++)
        ans = fmin(ans, dp[full][i] + dist[i][0]);
    return ans;
}
```

---

### 4.5 Count Bits in Range [0, n] — Digit DP

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
// Count total set bits from 0 to n
func countBits(n int) int {
    if n < 0 { return 0 }
    // At position p, bits cycle with period 2^(p+1)
    // In each full cycle: 2^p set bits
    total, p := 0, 1
    for p <= n {
        full := (n + 1) / (2 * p)
        rem  := (n + 1) % (2 * p)
        total += full * p + max(0, rem - p)
        p <<= 1
    }
    return total
}
func max(a, b int) int { if a > b { return a }; return b }
```

---

### 4.6 Bitset as Visited / Membership

```c
// ─── C — manual bitset (for large n) ─────────────────────────────────────────
#define MAXN 100000
uint64_t visited[MAXN / 64 + 1];

void   mark(int i)    { visited[i/64] |= (1ULL << (i%64)); }
void   unmark(int i)  { visited[i/64] &= ~(1ULL << (i%64)); }
int    is_set(int i)  { return (visited[i/64] >> (i%64)) & 1; }
```

```go
// ─── Go — using uint64 slice ──────────────────────────────────────────────────
type Bitset []uint64

func NewBitset(n int) Bitset    { return make(Bitset, (n+63)/64) }
func (b Bitset) Set(i int)      { b[i/64] |= 1 << (i % 64) }
func (b Bitset) Clear(i int)    { b[i/64] &^= 1 << (i % 64) }
func (b Bitset) IsSet(i int) bool { return b[i/64]>>(i%64)&1 == 1 }
```

```rust
// ─── Rust — using bit_vec crate or manual ─────────────────────────────────────
struct Bitset { data: Vec<u64> }

impl Bitset {
    fn new(n: usize) -> Self { Self { data: vec![0u64; (n + 63) / 64] } }
    fn set(&mut self, i: usize)    { self.data[i/64] |= 1 << (i % 64); }
    fn clear(&mut self, i: usize)  { self.data[i/64] &= !(1 << (i % 64)); }
    fn get(&self, i: usize) -> bool { (self.data[i/64] >> (i % 64)) & 1 == 1 }
}
```

---

## 5. System Programming Patterns

### 5.1 Flags and Bitfields

```c
// ─── C ───────────────────────────────────────────────────────────────────────
// Define flags as individual bits
typedef enum {
    FLAG_READ    = 1 << 0,   // 0001
    FLAG_WRITE   = 1 << 1,   // 0010
    FLAG_EXEC    = 1 << 2,   // 0100
    FLAG_HIDDEN  = 1 << 3,   // 1000
} FileFlags;

uint8_t perms = 0;
perms |= FLAG_READ | FLAG_WRITE;   // set
perms &= ~FLAG_WRITE;              // clear
if (perms & FLAG_READ) { /* readable */ }

// C bitfields in struct — compact storage
struct PacketHeader {
    uint8_t  version  : 4;  // 4 bits
    uint8_t  type     : 4;  // 4 bits
    uint16_t length   : 12; // 12 bits
    uint8_t  flags    : 4;  // 4 bits (total 24 bits = 3 bytes)
};
```

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
const (
    FlagRead   = 1 << iota  // 1
    FlagWrite               // 2
    FlagExec                // 4
    FlagHidden              // 8
)

type FileFlags uint8

func (f FileFlags) HasRead()  bool { return f&FlagRead != 0 }
func (f FileFlags) HasWrite() bool { return f&FlagWrite != 0 }
func (f *FileFlags) Set(flag FileFlags)   { *f |= flag }
func (f *FileFlags) Clear(flag FileFlags) { *f &^= flag }
func (f *FileFlags) Toggle(flag FileFlags) { *f ^= flag }

// Usage
var perms FileFlags
perms.Set(FlagRead | FlagWrite)
perms.Clear(FlagWrite)
```

```rust
// ─── Rust — using bitflags crate (or manual) ─────────────────────────────────
// Manual flags
mod flags {
    pub const READ  : u8 = 1 << 0;
    pub const WRITE : u8 = 1 << 1;
    pub const EXEC  : u8 = 1 << 2;
}

let mut perms: u8 = 0;
perms |= flags::READ | flags::WRITE;
perms &= !flags::WRITE;

// bitflags crate (preferred for production):
// use bitflags::bitflags;
// bitflags! {
//     struct Perms: u8 {
//         const READ  = 0b001;
//         const WRITE = 0b010;
//         const EXEC  = 0b100;
//     }
// }
```

---

### 5.2 Endianness Detection & Byte Swapping

```c
// ─── C ───────────────────────────────────────────────────────────────────────
#include <stdint.h>

int is_little_endian() {
    uint16_t n = 1;
    return *(uint8_t*)&n == 1;  // checks if low byte is stored first
}

// Manual byte swap (portable)
uint16_t bswap16(uint16_t n) { return (n << 8) | (n >> 8); }

uint32_t bswap32(uint32_t n) {
    return ((n & 0xFF000000) >> 24) |
           ((n & 0x00FF0000) >>  8) |
           ((n & 0x0000FF00) <<  8) |
           ((n & 0x000000FF) << 24);
}

// GCC builtins (faster)
uint32_t swapped = __builtin_bswap32(n);
uint64_t swapped = __builtin_bswap64(n);

// Network byte order helpers (POSIX)
#include <arpa/inet.h>
uint32_t net = htonl(host_val);  // host-to-network (big-endian)
uint32_t host = ntohl(net_val);  // network-to-host
```

```go
// ─── Go — encoding/binary ────────────────────────────────────────────────────
import (
    "encoding/binary"
    "math/bits"
)

// Check endianness at runtime
func isLittleEndian() bool {
    n := uint16(1)
    b := (*[2]byte)(unsafe.Pointer(&n))
    return b[0] == 1
}

// Byte swap
bits.ReverseBytes16(n)
bits.ReverseBytes32(n)
bits.ReverseBytes64(n)

// Serialize integers with explicit endianness
buf := make([]byte, 4)
binary.BigEndian.PutUint32(buf, 0xDEADBEEF)   // network order
binary.LittleEndian.PutUint32(buf, value)       // x86 native
v := binary.BigEndian.Uint32(buf)              // deserialize
```

```rust
// ─── Rust ────────────────────────────────────────────────────────────────────
// All integer types have built-in endian methods
let n: u32 = 0xDEADBEEF;
let be = n.to_be();                   // big-endian
let le = n.to_le();                   // little-endian
let n2 = u32::from_be(be);           // from big-endian
let swapped = n.swap_bytes();         // swap all bytes

// Check endianness at compile time
#[cfg(target_endian = "little")]
fn native_endian() -> &'static str { "little" }
```

---

### 5.3 Packing & Unpacking Data

```c
// ─── C — pack two 16-bit values into one 32-bit word ──────────────────────────
uint32_t pack(uint16_t hi, uint16_t lo) {
    return ((uint32_t)hi << 16) | lo;
}
void unpack(uint32_t word, uint16_t *hi, uint16_t *lo) {
    *hi = (uint16_t)(word >> 16);
    *lo = (uint16_t)(word & 0xFFFF);
}

// Packing RGB into 32-bit (0xAARRGGBB)
uint32_t rgba(uint8_t r, uint8_t g, uint8_t b, uint8_t a) {
    return ((uint32_t)a << 24) | ((uint32_t)r << 16) |
           ((uint32_t)g <<  8) | b;
}
uint8_t get_r(uint32_t color) { return (color >> 16) & 0xFF; }
uint8_t get_g(uint32_t color) { return (color >>  8) & 0xFF; }
uint8_t get_b(uint32_t color) { return  color        & 0xFF; }
uint8_t get_a(uint32_t color) { return (color >> 24) & 0xFF; }
```

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
func pack(hi, lo uint16) uint32   { return uint32(hi)<<16 | uint32(lo) }
func unpackHi(w uint32) uint16    { return uint16(w >> 16) }
func unpackLo(w uint32) uint16    { return uint16(w & 0xFFFF) }

func rgba(r, g, b, a uint8) uint32 {
    return uint32(a)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}
```

```rust
// ─── Rust ────────────────────────────────────────────────────────────────────
fn pack(hi: u16, lo: u16) -> u32 { (hi as u32) << 16 | lo as u32 }
fn unpack(w: u32) -> (u16, u16)  { ((w >> 16) as u16, (w & 0xFFFF) as u16) }

fn rgba(r: u8, g: u8, b: u8, a: u8) -> u32 {
    (a as u32) << 24 | (r as u32) << 16 | (g as u32) << 8 | b as u32
}
fn get_r(c: u32) -> u8 { ((c >> 16) & 0xFF) as u8 }
```

---

### 5.4 Memory Alignment

```c
// ─── C ───────────────────────────────────────────────────────────────────────
// Align address UP to next multiple of align (must be power of 2)
size_t align_up(size_t addr, size_t align) {
    return (addr + align - 1) & ~(align - 1);
}

// Align address DOWN
size_t align_down(size_t addr, size_t align) {
    return addr & ~(align - 1);
}

// Check if aligned
int is_aligned(size_t addr, size_t align) {
    return (addr & (align - 1)) == 0;
}

// Usage: allocate 4KB-aligned buffer
size_t page = 4096;
size_t aligned_addr = align_up((size_t)ptr, page);
```

```go
// ─── Go ──────────────────────────────────────────────────────────────────────
func alignUp  (addr, align uintptr) uintptr { return (addr + align - 1) &^ (align - 1) }
func alignDown(addr, align uintptr) uintptr { return addr &^ (align - 1) }
func isAligned(addr, align uintptr) bool    { return addr&(align-1) == 0 }
```

```rust
// ─── Rust ────────────────────────────────────────────────────────────────────
fn align_up  (addr: usize, align: usize) -> usize { (addr + align - 1) & !(align - 1) }
fn align_down(addr: usize, align: usize) -> usize { addr & !(align - 1) }
fn is_aligned(addr: usize, align: usize) -> bool  { addr & (align - 1) == 0 }
```

---

### 5.5 Fixed-Size Ring Buffer (Power-of-2 Trick)

```c
// ─── C ───────────────────────────────────────────────────────────────────────
// Size MUST be power of 2 — enables branchless wrap-around with & mask
#define RING_SIZE 256  // power of 2
#define RING_MASK (RING_SIZE - 1)

typedef struct {
    int data[RING_SIZE];
    int head, tail;
} RingBuf;

void push(RingBuf *rb, int val) {
    rb->data[rb->tail & RING_MASK] = val;
    rb->tail++;
}
int pop(RingBuf *rb) {
    int val = rb->data[rb->head & RING_MASK];
    rb->head++;
    return val;
}
int is_empty(RingBuf *rb) { return rb->head == rb->tail; }
int is_full (RingBuf *rb) { return rb->tail - rb->head == RING_SIZE; }
```

---

### 5.6 Bit-Parallel String Search (Bitap / Shift-And)

```c
// ─── C — Shift-And algorithm: O(n * m/64) with 64-bit words ──────────────────
// Finds all occurrences of pattern (len <= 64) in text
void shift_and(const char *text, int n, const char *pat, int m) {
    uint64_t D = 0, B[256] = {0};
    // Preprocess: for each character c, B[c] has bit i set if pat[i] == c
    for (int i = 0; i < m; i++) B[(unsigned char)pat[i]] |= 1ULL << i;

    uint64_t accept = 1ULL << (m - 1);
    for (int j = 0; j < n; j++) {
        D = ((D << 1) | 1) & B[(unsigned char)text[j]];
        if (D & accept)
            printf("Match at position %d\n", j - m + 1);
    }
}
```

---

## 6. Quick Reference Cheat Sheet

### Operations at a Glance

| Task | C | Go | Rust |
|------|---|----|------|
| Test bit i | `(n >> i) & 1` | `(n >> i) & 1` | `(n >> i) & 1` |
| Set bit i | `n \| (1 << i)` | `n \| (1 << i)` | `n \| (1 << i)` |
| Clear bit i | `n & ~(1 << i)` | `n &^ (1 << i)` | `n & !(1 << i)` |
| Toggle bit i | `n ^ (1 << i)` | `n ^ (1 << i)` | `n ^ (1 << i)` |
| Lowest set bit | `n & (-n)` | `n & (-n)` | `n & n.wrapping_neg()` |
| Clear lowest set | `n & (n-1)` | `n & (n-1)` | `n & (n-1)` |
| Is power of 2 | `n && !(n & (n-1))` | `n & (n-1) == 0` | `n.is_power_of_two()` |
| Popcount | `__builtin_popcount(n)` | `bits.OnesCount(n)` | `n.count_ones()` |
| CTZ (trailing zeros) | `__builtin_ctz(n)` | `bits.TrailingZeros(n)` | `n.trailing_zeros()` |
| CLZ (leading zeros) | `__builtin_clz(n)` | `bits.LeadingZeros(n)` | `n.leading_zeros()` |
| Byte swap 32 | `__builtin_bswap32(n)` | `bits.ReverseBytes32(n)` | `n.swap_bytes()` |
| Align up | `(n+a-1) & ~(a-1)` | `(n+a-1) &^ (a-1)` | `(n+a-1) & !(a-1)` |
| Unary NOT | `~n` | `^n` | `!n` |

### DSA Pattern Lookup

| Problem | Pattern |
|---------|---------|
| Represent a subset of n items | Integer with n bits, `1 << i` = include item i |
| Is x in the set represented by mask? | `mask & (1 << x) != 0` |
| Add x to set | `mask \| (1 << x)` |
| Remove x from set | `mask & ~(1 << x)` |
| Enumerate all subsets | `for mask in 0..(1<<n)` |
| Enumerate subsets of mask | `for sub = mask; sub > 0; sub = (sub-1) & mask` |
| XOR to find unique element | `fold XOR over array` |
| Find missing 0..n | `XOR(0..n) XOR XOR(array)` |
| Bitmask DP state | `dp[mask][i]`, mask = set of visited nodes |
| Modulo power of 2 | `x & (m-1)` |
| n-th bit in byte array | `arr[n/8] & (1 << (n%8))` |

### Common Pitfalls

```
1. SHIFT OVERFLOW:     1 << 31 is UB in C (signed); use 1u << 31 or 1ULL << 63
2. ARITHMETIC SHIFT:   right shift of signed negative is implementation-defined in C;
                       use unsigned types for bit manipulation
3. GO UNARY NOT:       use ^n (not ~n which doesn't compile)
4. RUST OVERFLOW:      left shifts can panic in debug; use wrapping_shl() if needed
5. XOR SWAP:           undefined behavior if both pointers alias the same variable in C
6. INT SIZE:           always use <stdint.h> (uint32_t, uint64_t) in C to avoid surprises
7. SIGNED BITMASK:     prefer unsigned integer types for bit fields and masks
8. ENDIANNESS:         struct bit-field layout is compiler/platform dependent in C;
                       use explicit shifts for portable serialization
```

---

*Guide covers: Go 1.21+ · Rust 1.75+ · C (C11/C17 with GCC/Clang)*
