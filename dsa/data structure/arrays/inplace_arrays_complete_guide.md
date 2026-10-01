# In-Place Array Operations: A Complete, In-Depth Guide

> **No copies. No extra allocation. Just raw memory manipulation.**

---

## Table of Contents

1. [What Does "In-Place" Actually Mean?](#1-what-does-in-place-actually-mean)
2. [Memory Model Fundamentals](#2-memory-model-fundamentals)
3. [Why Avoid Copies? The True Cost](#3-why-avoid-copies-the-true-cost)
4. [The Swap — Foundation of Everything](#4-the-swap--foundation-of-everything)
5. [Two-Pointer Technique](#5-two-pointer-technique)
6. [The Reversal Algorithm](#6-the-reversal-algorithm)
7. [Cyclic Replacement / Rotation](#7-cyclic-replacement--rotation)
8. [Dutch National Flag / Three-Way Partition](#8-dutch-national-flag--three-way-partition)
9. [In-Place Partitioning (Quicksort's Core)](#9-in-place-partitioning-quicksorts-core)
10. [Sliding Window (In-Place Compaction)](#10-sliding-window-in-place-compaction)
11. [Index Marking Technique](#11-index-marking-technique)
12. [XOR / Bit-Manipulation Tricks](#12-xor--bit-manipulation-tricks)
13. [In-Place Merge (Without Extra Space)](#13-in-place-merge-without-extra-space)
14. [Fisher-Yates In-Place Shuffle](#14-fisher-yates-in-place-shuffle)
15. [Floyd's Cycle Detection (In-Place)](#15-floyds-cycle-detection-in-place)
16. [In-Place Matrix Transformations](#16-in-place-matrix-transformations)
17. [Cache Behavior and Why It Matters](#17-cache-behavior-and-why-it-matters)
18. [Memory Safety: Rust's Perspective](#18-memory-safety-rusts-perspective)
19. [Trade-offs and When NOT to Use In-Place](#19-trade-offs-and-when-not-to-use-in-place)
20. [Mental Model Summary](#20-mental-model-summary)

---

## 1. What Does "In-Place" Actually Mean?

"In-place" means you **reuse the original array's memory** to produce the result. No auxiliary array. No hidden allocation. The transformation happens inside the existing buffer.

The formal definition:

- **Space complexity O(1)** — constant extra space (a few variables, indices, pointers), regardless of input size.
- **O(log n)** is sometimes called "quasi in-place" (e.g., recursive quicksort uses O(log n) stack frames).

### In-Place vs Out-of-Place

```
OUT-OF-PLACE: Reverse [1, 2, 3, 4, 5]
┌─────────────────────────────────────┐
│  Original Array (untouched)         │
│  addr 0x100: [1][2][3][4][5]        │
│                                     │
│  New Array (freshly malloc'd)       │
│  addr 0x200: [5][4][3][2][1]        │
│                                     │
│  Memory used: 2 × N × sizeof(T)     │
└─────────────────────────────────────┘

IN-PLACE: Reverse [1, 2, 3, 4, 5]
┌─────────────────────────────────────┐
│  Same Array, transformed            │
│  addr 0x100: [1][2][3][4][5]        │
│                  ↓ mutation         │
│  addr 0x100: [5][4][3][2][1]        │
│                                     │
│  Memory used: 1 × N × sizeof(T)     │
│             + O(1) for 2 pointers   │
└─────────────────────────────────────┘
```

### What "O(1) extra space" really means

```
In-place operation on int arr[N]:
┌──────────────────────────────────────────────────────┐
│ STACK FRAME                                          │
│  arr   → pointer to heap (8 bytes)  ← doesn't count │
│  left  → int index     (4 bytes)    ← O(1) extra    │
│  right → int index     (4 bytes)    ← O(1) extra    │
│  tmp   → int value     (4 bytes)    ← O(1) extra    │
│                                                      │
│ HEAP                                                 │
│  [e0][e1][e2]...[eN-1]  ← N elements, REUSED        │
└──────────────────────────────────────────────────────┘
The O(1) variables are a FIXED cost, not a function of N.
```

---

## 2. Memory Model Fundamentals

To truly understand in-place operations, you must understand how arrays live in memory.

### Arrays are Contiguous Blocks

```
int arr[6] = {10, 20, 30, 40, 50, 60};

Physical memory layout (assuming 4-byte int, little-endian):

Address   Byte0  Byte1  Byte2  Byte3   Interpreted
──────────────────────────────────────────────────────
0x1000  │ 0x0A │ 0x00 │ 0x00 │ 0x00 │  = 10  (arr[0])
0x1004  │ 0x14 │ 0x00 │ 0x00 │ 0x00 │  = 20  (arr[1])
0x1008  │ 0x1E │ 0x00 │ 0x00 │ 0x00 │  = 30  (arr[2])
0x100C  │ 0x28 │ 0x00 │ 0x00 │ 0x00 │  = 40  (arr[3])
0x1010  │ 0x32 │ 0x00 │ 0x00 │ 0x00 │  = 50  (arr[4])
0x1014  │ 0x3C │ 0x00 │ 0x00 │ 0x00 │  = 60  (arr[5])

arr[i] lives at:  base_address + i * sizeof(element)
```

### Why Contiguity Enables In-Place

Because `arr[i]` is a direct address computation, you can:
1. **Read** any element in O(1) — just compute the address.
2. **Write** any element in O(1) — write to that computed address.
3. **Swap** two elements in O(1) — read both, write both.

```
Swap arr[1] and arr[4]:

Before:
0x1000: [10][20][30][40][50][60]
              ↑           ↑
            [1]         [4]

Step 1: tmp = arr[1]   →  tmp = 20
Step 2: arr[1] = arr[4] →  write 50 at 0x1004
Step 3: arr[4] = tmp    →  write 20 at 0x1010

After:
0x1000: [10][50][30][40][20][60]
```

### Stack vs Heap: Where Does the Array Live?

```
C: Stack-allocated array
─────────────────────────────────
void foo() {
    int arr[5] = {1,2,3,4,5};
}

Stack:
┌──────────────┐  ← Stack grows downward
│  arr[4] = 5  │  0x7fff_0004
│  arr[3] = 4  │  0x7fff_0008
│  arr[2] = 3  │  0x7fff_000C
│  arr[1] = 2  │  0x7fff_0010
│  arr[0] = 1  │  0x7fff_0014  ← base (arr)
│  return addr │
└──────────────┘
Freed automatically when foo() returns.

C: Heap-allocated array
─────────────────────────────────
int *arr = malloc(5 * sizeof(int));

Stack:          Heap:
┌──────────┐    ┌────────────────────┐
│  arr ptr │───→│ [1][2][3][4][5]   │
│ 0x2000   │    │ at address 0x2000 │
└──────────┘    └────────────────────┘
Must be freed manually with free(arr).

Go/Rust: heap-allocated, GC/ownership manages lifetime.
```

---

## 3. Why Avoid Copies? The True Cost

### Time Cost

```
Copying N elements = O(N) time.

For N = 10,000,000 (10M ints, 40MB):
  memcpy speed ≈ 10 GB/s (modern DDR4)
  Time = 40MB / 10GB/s = 4ms per copy

If your algorithm copies 10 times → 40ms just in copying.
In-place: 0ms copying overhead.
```

### Memory Cost

```
Out-of-place sort of 1 billion integers:

Original: 4 GB
Copy:     4 GB
─────────────────
Total:    8 GB   ← might not even fit in RAM

In-place sort: 4 GB + O(log N) stack = 4 GB + ~120 bytes
```

### Cache Cost (The Hidden Enemy)

```
Cache hierarchy (approximate latencies):

L1 cache:   32 KB,   ~1 ns   (4 cycles)
L2 cache:   256 KB,  ~4 ns   (12 cycles)
L3 cache:   8 MB,    ~12 ns  (40 cycles)
DRAM:       GBs,     ~60 ns  (200 cycles)

When you copy:
  1. Read original → cache miss → DRAM fetch
  2. Write copy    → another cache line → evicts original
  3. Read copy     → might miss again

In-place: data stays in cache.
Accessing arr[i] and arr[j] uses the SAME cache lines.
```

---

## 4. The Swap — Foundation of Everything

Every in-place algorithm is ultimately built on swaps. You must understand all forms.

### Classic Three-Variable Swap

```
tmp = a;
a   = b;
b   = tmp;

Memory view:
          a         b       tmp
Before: [17]      [42]      [ ]
Step1:  [17]      [42]      [17]   ← tmp = a
Step2:  [42]      [42]      [17]   ← a = b
Step3:  [42]      [17]      [17]   ← b = tmp
After:  [42]      [17]      (gone)
```

### XOR Swap (No Temp Variable)

```
a ^= b;
b ^= a;
a ^= b;

Step-by-step with a=5, b=9:

Binary:  a = 0101,  b = 1001

Step 1:  a = a ^ b = 0101 ^ 1001 = 1100   (a holds XOR of both)
Step 2:  b = b ^ a = 1001 ^ 1100 = 0101   (b now holds original a=5)
Step 3:  a = a ^ b = 1100 ^ 0101 = 1001   (a now holds original b=9)

Final: a=9, b=5  ✓

CRITICAL WARNING: XOR swap fails if a and b are the SAME memory location!
  If &a == &b:
    a ^= b  →  a = a ^ a = 0   ← data destroyed!
  Always check: if (i != j) before XOR swapping arr[i] and arr[j].
```

### Arithmetic Swap (integers only, overflow risk)

```
a = a + b;
b = a - b;   // b = (a+b) - b = a
a = a - b;   // a = (a+b) - a = b

DANGER: a + b can overflow for large integers.
        Use with extreme caution or not at all.
```

### C Implementation — All Three Swap Variants

```c
#include <stdio.h>
#include <stdint.h>

// Safe, universal swap (works for any type via memcpy for structs)
void swap_classic(int *a, int *b) {
    int tmp = *a;
    *a = *b;
    *b = tmp;
}

// XOR swap — no temp, integers only, MUST ensure a != b (different addresses)
void swap_xor(int *a, int *b) {
    if (a == b) return;  // CRITICAL: same pointer = disaster without this check
    *a ^= *b;
    *b ^= *a;
    *a ^= *b;
}

// Generic swap using byte-level XOR (works for any POD type)
void swap_generic(void *a, void *b, size_t size) {
    if (a == b) return;
    uint8_t *p = (uint8_t *)a;
    uint8_t *q = (uint8_t *)b;
    for (size_t i = 0; i < size; i++) {
        p[i] ^= q[i];
        q[i] ^= p[i];
        p[i] ^= q[i];
    }
}

int main() {
    int arr[] = {10, 20, 30, 40, 50};
    // Swap index 0 and 4
    swap_classic(&arr[0], &arr[4]);
    // arr = [50, 20, 30, 40, 10]

    swap_xor(&arr[1], &arr[3]);
    // arr = [50, 40, 30, 20, 10]

    // Safe call: same index (no-op due to a==b guard)
    swap_xor(&arr[2], &arr[2]);  // safely does nothing

    for (int i = 0; i < 5; i++) printf("%d ", arr[i]);
    // Output: 50 40 30 20 10
    return 0;
}
```

### Rust Implementation — Safe Swap

```rust
// Rust's standard library: slice.swap(i, j)
// Under the hood it does exactly the classic 3-var swap but safely.

fn main() {
    let mut arr = [10, 20, 30, 40, 50];

    // Safe: Rust checks bounds, ensures i != j at compile/runtime level
    arr.swap(0, 4);
    // [50, 20, 30, 40, 10]

    // Manual swap using split_at_mut to get two mutable refs:
    // (Rust won't let you have two &mut to the same slice simultaneously)
    let (left, right) = arr.split_at_mut(3);
    std::mem::swap(&mut left[1], &mut right[0]);
    // Split: left=[50,20,30], right=[40,10]
    // Swaps left[1]=20 with right[0]=40
    // Result: [50, 40, 30, 20, 10]

    println!("{:?}", arr);
}

// Under the hood, std::mem::swap is:
// pub fn swap<T>(x: &mut T, y: &mut T) {
//     unsafe { ptr::swap_nonoverlapping(x, y, 1) }
// }
// Uses raw pointer ops — same as C's 3-var swap but verified safe by borrow checker.
```

### Go Implementation

```go
package main

import "fmt"

func swapClassic(arr []int, i, j int) {
    arr[i], arr[j] = arr[j], arr[i]  // Go tuple assignment — idiomatic
    // Compiler desugars this to:
    //   tmp := arr[i]
    //   arr[i] = arr[j]
    //   arr[j] = tmp
    // No actual tuple allocation on heap. Pure register swap.
}

func main() {
    arr := []int{10, 20, 30, 40, 50}
    swapClassic(arr, 0, 4)
    fmt.Println(arr) // [50 20 30 40 10]
}
```

---

## 5. Two-Pointer Technique

The most versatile in-place pattern. Two indices march through the array, either toward each other or in the same direction.

### Pattern A: Converging Pointers (from both ends)

```
Used for: Reversal, palindrome check, two-sum in sorted array,
          container with most water, trapping rain water.

arr = [1, 2, 3, 4, 5, 6, 7]
       L                 R

Iteration 1: swap(L=0, R=6) → [7,2,3,4,5,6,1],  L=1, R=5
Iteration 2: swap(L=1, R=5) → [7,6,3,4,5,2,1],  L=2, R=4
Iteration 3: swap(L=2, R=4) → [7,6,5,4,3,2,1],  L=3, R=3
L >= R: STOP

Visual of pointer movement:
Index:  0  1  2  3  4  5  6
        [7][6][5][4][3][2][1]
         L→         ←R
         
Converge from both ends. Process. Move inward. Stop when L≥R.
```

### Pattern B: Fast and Slow Pointers (same direction)

```
Used for: Remove duplicates, filter elements, partition.
          "Read pointer" runs ahead, "write pointer" marks valid position.

arr = [1, 1, 2, 2, 3, 4, 4]  (remove duplicates in sorted array)
       W                       W = write pointer (slow)
       R                       R = read pointer (fast)

State:    W    R    arr
Start:    0    0    [1,1,2,2,3,4,4]
R=0: arr[R]=1 != arr[W-1](none), write arr[W]=1, W=1, R=1
             1    [1,1,2,2,3,4,4]  W points to slot 1
R=1: arr[1]=1 == arr[0]=1, SKIP, R=2
R=2: arr[2]=2 != arr[0]=1, write arr[1]=2, W=2, R=3
             2    [1,2,2,2,3,4,4]  (arr[1] overwritten)
R=3: arr[3]=2 == arr[1]=2, SKIP, R=4
R=4: arr[4]=3 != arr[1]=2, write arr[2]=3, W=3, R=5
                  [1,2,3,2,3,4,4]
R=5: arr[5]=4 != arr[2]=3, write arr[3]=4, W=4, R=6
                       [1,2,3,4,3,4,4]
R=6: arr[6]=4 == arr[3]=4, SKIP, R=7
R=7: R==N, STOP.  Valid length = W = 4
Result: arr[0..3] = [1, 2, 3, 4]  ← unique elements, IN PLACE
```

### C: Two Pointer — Remove Duplicates from Sorted Array

```c
#include <stdio.h>

// Returns new length. arr is modified in-place.
// arr MUST be sorted.
int remove_duplicates(int *arr, int n) {
    if (n == 0) return 0;

    int write = 1;  // write pointer; arr[0] is always kept

    for (int read = 1; read < n; read++) {
        // Only write if current element differs from last written
        if (arr[read] != arr[write - 1]) {
            arr[write] = arr[read];  // overwrite in-place
            write++;
        }
        // If duplicate: read advances, write stays — element is "discarded"
    }
    return write;  // new logical length
}

int main() {
    int arr[] = {1, 1, 2, 2, 3, 4, 4};
    int n = sizeof(arr) / sizeof(arr[0]);
    int new_len = remove_duplicates(arr, n);
    // arr = [1,2,3,4,...] (first new_len elements are valid)
    printf("Length: %d\n", new_len);  // 4
    for (int i = 0; i < new_len; i++) printf("%d ", arr[i]);
    return 0;
}
```

### Rust: Two Pointer — Remove Duplicates

```rust
fn remove_duplicates(arr: &mut Vec<i32>) -> usize {
    if arr.is_empty() { return 0; }

    let mut write = 1usize;
    for read in 1..arr.len() {
        if arr[read] != arr[write - 1] {
            arr[write] = arr[read];  // in-place overwrite
            write += 1;
        }
    }
    arr.truncate(write);  // shrink the Vec's length (no reallocation)
    write
}

// Note: arr.dedup() in stdlib does exactly this.
// Under the hood: https://doc.rust-lang.org/src/alloc/vec/mod.rs
// Uses unsafe pointer arithmetic for performance but same logic.

fn main() {
    let mut arr = vec![1, 1, 2, 2, 3, 4, 4];
    let len = remove_duplicates(&mut arr);
    println!("{:?} len={}", arr, len); // [1, 2, 3, 4] len=4
}
```

### Go: Two Pointer — Remove Element (by value)

```go
package main

import "fmt"

// Remove all occurrences of 'val' from arr in-place.
// Returns the new length. Order of remaining elements preserved.
func removeElement(arr []int, val int) int {
    write := 0
    for read := 0; read < len(arr); read++ {
        if arr[read] != val {
            arr[write] = arr[read]  // compact valid elements leftward
            write++
        }
        // If arr[read] == val: read advances past it, write stays
        // effectively "skipping" the element
    }
    return write
}

func main() {
    arr := []int{3, 2, 2, 3, 4, 3, 5}
    n := removeElement(arr, 3)
    fmt.Println(arr[:n]) // [2 2 4 5]
}
```

---

## 6. The Reversal Algorithm

Reversing an array in-place is the building block for rotation, palindrome operations, and string manipulation.

### How Reversal Works

```
arr = [A, B, C, D, E]

Two pointers: L=0, R=4

Step 1: swap(arr[0], arr[4])
        [E, B, C, D, A]
         ✓           ✓   L=1, R=3

Step 2: swap(arr[1], arr[3])
        [E, D, C, B, A]
             ✓     ✓   L=2, R=2

Step 3: L == R (odd-length, center stays): STOP
        [E, D, C, B, A]   ← reversed!

For even-length [A,B,C,D]:
L=0,R=3: swap → [D,B,C,A], L=1, R=2
L=1,R=2: swap → [D,C,B,A], L=2, R=1
L >= R: STOP
```

### C: Reverse

```c
#include <stdio.h>

void reverse(int *arr, int left, int right) {
    // Reverse arr[left..right] inclusive, in-place
    while (left < right) {
        // Classic 3-var swap (compiler will likely use registers, zero overhead)
        int tmp   = arr[left];
        arr[left] = arr[right];
        arr[right]= tmp;
        left++;
        right--;
    }
}

// Rotate array LEFT by k positions using 3 reversals (O(N) time, O(1) space)
// [1,2,3,4,5,6,7], k=3 → [4,5,6,7,1,2,3]
void rotate_left(int *arr, int n, int k) {
    k = k % n;  // handle k > n
    reverse(arr, 0, k - 1);       // Reverse first k elements
    reverse(arr, k, n - 1);       // Reverse remaining n-k elements
    reverse(arr, 0, n - 1);       // Reverse entire array

    // Why this works:
    // Original:           [1,2,3 | 4,5,6,7]
    // After rev(0,k-1):  [3,2,1 | 4,5,6,7]
    // After rev(k,n-1):  [3,2,1 | 7,6,5,4]
    // After rev(0,n-1):  [4,5,6,7,1,2,3]   ← done!
}

int main() {
    int arr[] = {1, 2, 3, 4, 5, 6, 7};
    int n = 7;
    rotate_left(arr, n, 3);
    for (int i = 0; i < n; i++) printf("%d ", arr[i]);
    // 4 5 6 7 1 2 3
    return 0;
}
```

### Rust: Reverse and Rotate

```rust
fn reverse(arr: &mut [i32], mut left: usize, mut right: usize) {
    while left < right {
        arr.swap(left, right);  // Rust's built-in bounds-checked swap
        left += 1;
        right -= 1;
    }
}

fn rotate_left(arr: &mut [i32], k: usize) {
    let n = arr.len();
    if n == 0 { return; }
    let k = k % n;
    if k == 0 { return; }

    reverse(arr, 0, k - 1);
    reverse(arr, k, n - 1);
    reverse(arr, 0, n - 1);
}

fn main() {
    let mut arr = vec![1, 2, 3, 4, 5, 6, 7];
    rotate_left(&mut arr, 3);
    println!("{:?}", arr); // [4, 5, 6, 7, 1, 2, 3]

    // Rust stdlib also provides:
    arr.rotate_left(2);  // built-in, O(N), O(1) space
    println!("{:?}", arr); // [6, 7, 1, 2, 3, 4, 5]
}
```

### Go: Reverse and Rotate

```go
package main

import "fmt"

func reverse(arr []int, left, right int) {
    for left < right {
        arr[left], arr[right] = arr[right], arr[left]
        left++
        right--
    }
}

func rotateLeft(arr []int, k int) {
    n := len(arr)
    if n == 0 { return }
    k %= n
    reverse(arr, 0, k-1)
    reverse(arr, k, n-1)
    reverse(arr, 0, n-1)
}

func main() {
    arr := []int{1, 2, 3, 4, 5, 6, 7}
    rotateLeft(arr, 3)
    fmt.Println(arr) // [4 5 6 7 1 2 3]
}
```

---

## 7. Cyclic Replacement / Rotation

A fundamentally different approach to rotation: instead of reversals, follow the cycle that each element traces.

### How Cyclic Rotation Works

```
Rotate arr = [1,2,3,4,5,6,7] RIGHT by k=3:
Result should be: [5,6,7,1,2,3,4]

Each element needs to move to position: (i + k) % n

Element 1 at index 0 → goes to index 3
Element 2 at index 1 → goes to index 4
Element 3 at index 2 → goes to index 5
Element 4 at index 3 → goes to index 6
Element 5 at index 4 → goes to index 0
Element 6 at index 5 → goes to index 1
Element 7 at index 6 → goes to index 2

Cycle starting at index 0:
  0 → 3 → 6 → 2 → 5 → 1 → 4 → 0  (one big cycle when gcd(n,k)=1)

  Start: current=0, save=arr[0]=1
  Place arr[0]  at index 3: arr[3]=arr[0], save old arr[3]=4
  Place saved 4 at index 6: arr[6]=4,      save old arr[6]=7
  Place saved 7 at index 2: arr[2]=7,      save old arr[2]=3
  Place saved 3 at index 5: arr[5]=3,      save old arr[5]=6
  Place saved 6 at index 1: arr[1]=6,      save old arr[1]=2
  Place saved 2 at index 4: arr[4]=2,      save old arr[4]=5
  Place saved 5 at index 0: arr[0]=5  ← back to start

After: [5,6,7,1,2,3,4] ✓

When gcd(n,k) > 1, there are multiple cycles.
Number of cycles = gcd(n, k).
```

### C: Cyclic Rotation

```c
#include <stdio.h>

// Computes GCD (needed to know how many cycles exist)
int gcd(int a, int b) {
    return b == 0 ? a : gcd(b, a % b);
}

void rotate_right_cyclic(int *arr, int n, int k) {
    k = k % n;
    if (k == 0) return;

    int cycles = gcd(n, k);  // number of independent cycles
    int moved = 0;            // count how many elements placed

    for (int start = 0; start < cycles; start++) {
        int current = start;
        int save = arr[start];  // the element to place

        do {
            int next = (current + k) % n;  // destination index
            int tmp = arr[next];            // save what's at destination
            arr[next] = save;              // place our element
            save = tmp;                    // the displaced element is next to place
            current = next;
            moved++;
        } while (current != start);  // cycle complete when we return to start
    }
    // Total moves = n, total swaps ≈ n, O(1) extra space (save + tmp + indices)
}

int main() {
    int arr[] = {1, 2, 3, 4, 5, 6, 7};
    rotate_right_cyclic(arr, 7, 3);
    for (int i = 0; i < 7; i++) printf("%d ", arr[i]);
    // 5 6 7 1 2 3 4
    return 0;
}
```

### Rust: Cyclic Rotation

```rust
fn gcd(a: usize, b: usize) -> usize {
    if b == 0 { a } else { gcd(b, a % b) }
}

fn rotate_right_cyclic(arr: &mut [i32], k: usize) {
    let n = arr.len();
    if n == 0 { return; }
    let k = k % n;
    if k == 0 { return; }

    let cycles = gcd(n, k);

    for start in 0..cycles {
        let mut current = start;
        let mut save = arr[start];

        loop {
            let next = (current + k) % n;
            let tmp = arr[next];
            arr[next] = save;
            save = tmp;
            current = next;
            if current == start { break; }
        }
    }
}

fn main() {
    let mut arr = vec![1, 2, 3, 4, 5, 6, 7];
    rotate_right_cyclic(&mut arr, 3);
    println!("{:?}", arr); // [5, 6, 7, 1, 2, 3, 4]
}
```

---

## 8. Dutch National Flag / Three-Way Partition

Dijkstra's algorithm. Partition an array into three sections around a pivot — elements less than, equal to, and greater than the pivot — all in-place with one pass.

### The Three Regions

```
arr = [2, 0, 1, 1, 0, 2, 1, 0]  (values only 0, 1, 2)

Maintain 3 regions using 3 pointers:

low  = 0   → Everything before low is 0s
mid  = 0   → Everything between low and mid is 1s
high = n-1 → Everything after high is 2s

Initial:
 lo  mi                        hi
  ↓   ↓                         ↓
 [2,  0,  1,  1,  0,  2,  1,  0]
  ←0s→  ←1s→   ←unknown→  ←2s→

Case analysis based on arr[mid]:
  arr[mid] == 0: swap(arr[low], arr[mid]), low++, mid++
  arr[mid] == 1: mid++                (already in right place)
  arr[mid] == 2: swap(arr[mid], arr[high]), high--  (mid stays! new arr[mid] is unknown)

Step-by-step:
[2,0,1,1,0,2,1,0]  lo=0,mi=0,hi=7  arr[mi]=2: swap(0,7)→[0,0,1,1,0,2,1,2], hi=6
[0,0,1,1,0,2,1,2]  lo=0,mi=0,hi=6  arr[mi]=0: swap(0,0)→same, lo=1,mi=1
[0,0,1,1,0,2,1,2]  lo=1,mi=1,hi=6  arr[mi]=0: swap(1,1)→same, lo=2,mi=2
[0,0,1,1,0,2,1,2]  lo=2,mi=2,hi=6  arr[mi]=1: mi=3
[0,0,1,1,0,2,1,2]  lo=2,mi=3,hi=6  arr[mi]=1: mi=4
[0,0,1,1,0,2,1,2]  lo=2,mi=4,hi=6  arr[mi]=0: swap(2,4)→[0,0,0,1,1,2,1,2], lo=3,mi=5
[0,0,0,1,1,2,1,2]  lo=3,mi=5,hi=6  arr[mi]=2: swap(5,6)→[0,0,0,1,1,1,2,2], hi=5
[0,0,0,1,1,1,2,2]  lo=3,mi=6,hi=5  mi > hi: STOP

Result: [0,0,0,1,1,1,2,2] ✓  One pass, O(1) space!

Final regions:
 [0,0,0][1,1,1][2,2,2]
  lo..   mid..  hi..
```

### C: Dutch National Flag

```c
#include <stdio.h>

void dutch_flag(int *arr, int n) {
    int low = 0, mid = 0, high = n - 1;
    int tmp;

    while (mid <= high) {
        if (arr[mid] == 0) {
            // Swap with low region, advance both
            tmp = arr[low]; arr[low] = arr[mid]; arr[mid] = tmp;
            low++;
            mid++;
        } else if (arr[mid] == 1) {
            // Already in middle region
            mid++;
        } else {  // arr[mid] == 2
            // Swap with high region, DON'T advance mid
            // (the swapped-in element from high is unknown, must recheck)
            tmp = arr[mid]; arr[mid] = arr[high]; arr[high] = tmp;
            high--;
        }
    }
}

// Generalized: partition around any pivot value
void three_way_partition(int *arr, int n, int pivot) {
    int low = 0, mid = 0, high = n - 1;
    int tmp;

    while (mid <= high) {
        if (arr[mid] < pivot) {
            tmp = arr[low]; arr[low] = arr[mid]; arr[mid] = tmp;
            low++; mid++;
        } else if (arr[mid] == pivot) {
            mid++;
        } else {
            tmp = arr[mid]; arr[mid] = arr[high]; arr[high] = tmp;
            high--;
        }
    }
}

int main() {
    int arr[] = {2, 0, 1, 1, 0, 2, 1, 0};
    dutch_flag(arr, 8);
    for (int i = 0; i < 8; i++) printf("%d ", arr[i]);
    // 0 0 0 1 1 1 2 2
    return 0;
}
```

### Rust: Dutch National Flag

```rust
fn dutch_flag(arr: &mut [i32]) {
    if arr.is_empty() { return; }
    let (mut low, mut mid, mut high) = (0usize, 0usize, arr.len() - 1);

    while mid <= high {
        match arr[mid] {
            0 => {
                arr.swap(low, mid);
                low += 1;
                mid += 1;
            }
            1 => {
                mid += 1;
            }
            _ => {
                arr.swap(mid, high);
                if high == 0 { break; }  // underflow guard for usize
                high -= 1;
            }
        }
    }
}

fn main() {
    let mut arr = vec![2, 0, 1, 1, 0, 2, 1, 0];
    dutch_flag(&mut arr);
    println!("{:?}", arr); // [0, 0, 0, 1, 1, 1, 2, 2]
}
```

### Go: Dutch National Flag

```go
package main

import "fmt"

func dutchFlag(arr []int) {
    low, mid, high := 0, 0, len(arr)-1
    for mid <= high {
        switch arr[mid] {
        case 0:
            arr[low], arr[mid] = arr[mid], arr[low]
            low++
            mid++
        case 1:
            mid++
        default:
            arr[mid], arr[high] = arr[high], arr[mid]
            high--
        }
    }
}

func main() {
    arr := []int{2, 0, 1, 1, 0, 2, 1, 0}
    dutchFlag(arr)
    fmt.Println(arr) // [0 0 0 1 1 1 2 2]
}
```

---

## 9. In-Place Partitioning (Quicksort's Core)

Quicksort is entirely in-place. Its power comes from the partition step, which rearranges elements around a pivot.

### Lomuto Partition Scheme

```
arr = [3, 6, 8, 10, 1, 2, 1]  pivot = arr[high] = 1

i = lo - 1 = -1  (boundary of "small" region)
j = lo = 0       (scan pointer)

high=6, pivot=1

j=0: arr[0]=3 > 1, no swap.  j=1
j=1: arr[1]=6 > 1, no swap.  j=2
j=2: arr[2]=8 > 1, no swap.  j=3
j=3: arr[3]=10> 1, no swap.  j=4
j=4: arr[4]=1 ≤ 1, i=0, swap(arr[0],arr[4]) → [1,6,8,10,3,2,1], j=5
j=5: arr[5]=2 > 1, no swap.  j=6 (== high, stop)

Final: swap(arr[i+1], arr[high]) = swap(arr[1], arr[6])
     → [1,1,8,10,3,2,6]

Pivot (1) is now at index i+1=1.
Left of 1: [1]      → all ≤ pivot ✓
Right of 1: [8,10,3,2,6] → all > pivot ✓

Regions:
[1][1][8,10,3,2,6]
 ≤    pivot   >
```

### Hoare Partition Scheme (More Efficient)

```
arr = [3, 2, 1, 5, 4]  pivot = arr[0] = 3

i = lo - 1 = -1
j = hi + 1 = 5

Loop:
  i advances right until arr[i] >= pivot
  j advances left  until arr[j] <= pivot
  if i < j: swap(arr[i], arr[j])
  else: return j (partition point)

i=-1→0: arr[0]=3 >= 3 → stop at i=0
j=5→4:  arr[4]=4 > 3
j→3:    arr[3]=5 > 3
j→2:    arr[2]=1 <= 3 → stop at j=2
i=0 < j=2: swap(arr[0], arr[2]) → [1,2,3,5,4]

i=0→1: arr[1]=2 < 3
i→2:   arr[2]=3 >= 3 → stop at i=2
j=2→1: arr[1]=2 <= 3 → stop at j=1
i=2 > j=1: STOP. Return j=1 as partition index.

Left: arr[0..1] = [1,2]   all ≤ 3 ✓
Right: arr[2..4] = [3,5,4] all ≥ 3 ✓
```

### C: Quicksort (Lomuto, fully in-place)

```c
#include <stdio.h>

int partition_lomuto(int *arr, int lo, int hi) {
    int pivot = arr[hi];  // choose last element as pivot
    int i = lo - 1;       // i is the "last small element" index

    for (int j = lo; j < hi; j++) {
        if (arr[j] <= pivot) {
            i++;
            int tmp = arr[i]; arr[i] = arr[j]; arr[j] = tmp;
        }
    }
    // Place pivot in its correct sorted position
    int tmp = arr[i+1]; arr[i+1] = arr[hi]; arr[hi] = tmp;
    return i + 1;  // pivot's final index
}

void quicksort(int *arr, int lo, int hi) {
    if (lo < hi) {
        int pivot_idx = partition_lomuto(arr, lo, hi);
        quicksort(arr, lo, pivot_idx - 1);  // sort left of pivot
        quicksort(arr, pivot_idx + 1, hi);  // sort right of pivot
    }
    // Stack depth O(log n) average, O(n) worst case (already sorted input)
    // Data manipulation: O(1) extra (just indices + pivot value)
}

int main() {
    int arr[] = {3, 6, 8, 10, 1, 2, 1};
    int n = sizeof(arr) / sizeof(arr[0]);
    quicksort(arr, 0, n - 1);
    for (int i = 0; i < n; i++) printf("%d ", arr[i]);
    // 1 1 2 3 6 8 10
    return 0;
}
```

### Rust: Quicksort with Hoare Partition

```rust
fn partition_hoare(arr: &mut [i32], lo: usize, hi: usize) -> usize {
    let pivot = arr[lo];
    let mut i = lo as isize - 1;
    let mut j = hi as isize + 1;

    loop {
        loop { i += 1; if arr[i as usize] >= pivot { break; } }
        loop { j -= 1; if arr[j as usize] <= pivot { break; } }
        if i >= j { return j as usize; }
        arr.swap(i as usize, j as usize);
    }
}

fn quicksort(arr: &mut [i32], lo: usize, hi: usize) {
    if lo < hi {
        let p = partition_hoare(arr, lo, hi);
        if p > 0 { quicksort(arr, lo, p); }
        quicksort(arr, p + 1, hi);
    }
}

fn main() {
    let mut arr = vec![3, 6, 8, 10, 1, 2, 1];
    let hi = arr.len() - 1;
    quicksort(&mut arr, 0, hi);
    println!("{:?}", arr); // [1, 1, 2, 3, 6, 8, 10]
}
```

### Go: Quicksort

```go
package main

import "fmt"

func partition(arr []int, lo, hi int) int {
    pivot := arr[hi]
    i := lo - 1
    for j := lo; j < hi; j++ {
        if arr[j] <= pivot {
            i++
            arr[i], arr[j] = arr[j], arr[i]
        }
    }
    arr[i+1], arr[hi] = arr[hi], arr[i+1]
    return i + 1
}

func quicksort(arr []int, lo, hi int) {
    if lo < hi {
        p := partition(arr, lo, hi)
        quicksort(arr, lo, p-1)
        quicksort(arr, p+1, hi)
    }
}

func main() {
    arr := []int{3, 6, 8, 10, 1, 2, 1}
    quicksort(arr, 0, len(arr)-1)
    fmt.Println(arr) // [1 1 2 3 6 8 10]
}
```

---

## 10. Sliding Window (In-Place Compaction)

The sliding window finds subarrays satisfying a condition. When used for compaction (writing results back), it's in-place.

### In-Place Filter/Compaction via Write Pointer

```
Problem: Move all zeros to end, preserve non-zero order.
arr = [0, 1, 0, 3, 12]

Write pointer w=0. Read pointer r scans all elements.
When arr[r] != 0: copy to arr[w], w++.
After scan: fill arr[w..n-1] with 0.

r=0: arr[0]=0, skip
r=1: arr[1]=1, arr[w=0]=1, w=1
     [1, 1, 0, 3, 12]  (arr[0] overwritten with 1)
r=2: arr[2]=0, skip
r=3: arr[3]=3, arr[w=1]=3, w=2
     [1, 3, 0, 3, 12]
r=4: arr[4]=12, arr[w=2]=12, w=3
     [1, 3, 12, 3, 12]
End: fill [w..n-1] = [3,4] with 0:
     [1, 3, 12, 0, 0]   ✓

Why this is in-place: We only use arr itself + 2 index variables.
The "fill zeros" at end is just assignment to existing slots.
```

### C: Move Zeros

```c
#include <stdio.h>

void move_zeros(int *arr, int n) {
    int write = 0;

    // Phase 1: compact non-zeros to front
    for (int read = 0; read < n; read++) {
        if (arr[read] != 0) {
            arr[write++] = arr[read];
        }
    }

    // Phase 2: fill remaining positions with 0
    // No allocation — we're writing into the same array
    while (write < n) {
        arr[write++] = 0;
    }
}

// Alternative: swap-based (maintains relative position, no fill phase)
void move_zeros_swap(int *arr, int n) {
    int last_nonzero = 0;
    for (int i = 0; i < n; i++) {
        if (arr[i] != 0) {
            // Swap instead of overwrite preserves zeros' "positions"
            int tmp = arr[last_nonzero];
            arr[last_nonzero] = arr[i];
            arr[i] = tmp;
            last_nonzero++;
        }
    }
}

int main() {
    int arr[] = {0, 1, 0, 3, 12};
    move_zeros(arr, 5);
    for (int i = 0; i < 5; i++) printf("%d ", arr[i]);
    // 1 3 12 0 0
    return 0;
}
```

### Rust: Move Zeros

```rust
fn move_zeros(arr: &mut Vec<i32>) {
    let mut write = 0;
    for read in 0..arr.len() {
        if arr[read] != 0 {
            arr[write] = arr[read];
            write += 1;
        }
    }
    for i in write..arr.len() {
        arr[i] = 0;
    }
}

fn main() {
    let mut arr = vec![0, 1, 0, 3, 12];
    move_zeros(&mut arr);
    println!("{:?}", arr); // [1, 3, 12, 0, 0]
}
```

---

## 11. Index Marking Technique

A clever trick: use the array's own indices (or sign of values) as a secondary data structure. No extra array needed.

### Concept: Sign as a Boolean Flag

```
Problem: Find all duplicates in an array where values are in [1..N].
arr = [4, 3, 2, 7, 8, 2, 3, 1]  n=8

Key insight: value v maps to index v-1.
We "visit" index v-1 by negating arr[v-1].
If arr[v-1] is already negative: v is a duplicate!

Step through arr:
i=0: val=|arr[0]|=4 → mark index 3: arr[3]=-7  arr=[4,3,2,-7,8,2,3,1]
i=1: val=|arr[1]|=3 → mark index 2: arr[2]=-2  arr=[4,3,-2,-7,8,2,3,1]
i=2: val=|arr[2]|=2 → mark index 1: arr[1]=-3  arr=[4,-3,-2,-7,8,2,3,1]
i=3: val=|arr[3]|=7 → mark index 6: arr[6]=-3  arr=[4,-3,-2,-7,8,2,-3,1]
i=4: val=|arr[4]|=8 → mark index 7: arr[7]=-1  arr=[4,-3,-2,-7,8,2,-3,-1]
i=5: val=|arr[5]|=2 → check index 1: arr[1]=-3 (NEGATIVE!) → 2 is DUPLICATE
i=6: val=|arr[6]|=3 → check index 2: arr[2]=-2 (NEGATIVE!) → 3 is DUPLICATE
i=7: val=|arr[7]|=1 → mark index 0: arr[0]=-4

Duplicates found: [2, 3]
Total extra space: O(1) — we used the sign bits of existing values!
Restore: take abs of everything. arr=[4,3,2,7,8,2,3,1] restored.

ASCII view of sign-as-flag:
           +    -    -    -    +    +    -    -
arr = [    4,  -3,  -2,  -7,   8,   2,  -3,  -1]
index   [0]  [1]  [2]  [3]  [4]  [5]  [6]  [7]
val-1:  [3]  [2]  [1]  [6]  [7]  [1]  [2]  [0]
         ↑    ↑    ↑    ↑    ↑    ↑    ↑    ↑
         These indices are "marked visited" by negation.
```

### C: Find Duplicates via Index Marking

```c
#include <stdio.h>
#include <stdlib.h>  // for abs()

void find_duplicates(int *arr, int n) {
    printf("Duplicates: ");

    for (int i = 0; i < n; i++) {
        int val = abs(arr[i]);   // recover actual value even if already negated
        int idx = val - 1;       // map value → index

        if (arr[idx] < 0) {
            // Already marked → this val appeared before → DUPLICATE
            printf("%d ", val);
        } else {
            // First visit: negate to mark
            arr[idx] = -arr[idx];
        }
    }
    printf("\n");

    // Optional restore (if arr must remain unchanged after call)
    for (int i = 0; i < n; i++) {
        if (arr[i] < 0) arr[i] = -arr[i];
    }
}

// Find the MISSING number [1..N] using index marking
// arr contains n numbers from [1..N] with exactly one missing
int find_missing(int *arr, int n) {
    for (int i = 0; i < n; i++) {
        int idx = abs(arr[i]) - 1;
        if (arr[idx] > 0) arr[idx] = -arr[idx];
    }
    for (int i = 0; i < n; i++) {
        if (arr[i] > 0) return i + 1;  // this index was never visited → missing
    }
    return n + 1;  // all 1..n present, n+1 is missing
}

int main() {
    int arr[] = {4, 3, 2, 7, 8, 2, 3, 1};
    find_duplicates(arr, 8);  // Duplicates: 2 3

    int arr2[] = {1, 2, 4, 6, 3, 7, 8};
    printf("Missing: %d\n", find_missing(arr2, 7));  // Missing: 5
    return 0;
}
```

### Rust: Index Marking

```rust
fn find_duplicates(arr: &mut Vec<i32>) -> Vec<i32> {
    let mut result = vec![];

    for i in 0..arr.len() {
        let val = arr[i].unsigned_abs() as usize;
        let idx = val - 1;
        if arr[idx] < 0 {
            result.push(val as i32);
        } else {
            arr[idx] = -arr[idx];
        }
    }

    // Restore
    for x in arr.iter_mut() { *x = x.abs(); }
    result
}

fn main() {
    let mut arr = vec![4, 3, 2, 7, 8, 2, 3, 1];
    let dups = find_duplicates(&mut arr);
    println!("Duplicates: {:?}", dups); // [2, 3]
    println!("Restored: {:?}", arr);    // [4, 3, 2, 7, 8, 2, 3, 1]
}
```

### Go: Index Marking

```go
package main

import "fmt"

func findDuplicates(arr []int) []int {
    result := []int{}
    for i := 0; i < len(arr); i++ {
        val := arr[i]
        if val < 0 { val = -val }
        idx := val - 1
        if arr[idx] < 0 {
            result = append(result, val)
        } else {
            arr[idx] = -arr[idx]
        }
    }
    // Restore
    for i := range arr {
        if arr[i] < 0 { arr[i] = -arr[i] }
    }
    return result
}

func main() {
    arr := []int{4, 3, 2, 7, 8, 2, 3, 1}
    fmt.Println(findDuplicates(arr)) // [2 3]
}
```

---

## 12. XOR / Bit-Manipulation Tricks

XOR is the in-place wizard: commutative, associative, self-inverse (a^a=0), and leaves zero on identity (a^0=a).

### XOR Properties (Mathematical Foundation)

```
Property        Formula         Example
──────────────────────────────────────────
Commutative:    a ^ b = b ^ a   5^3 = 3^5
Associative:    (a^b)^c = a^(b^c)
Self-inverse:   a ^ a = 0       7^7 = 0
Identity:       a ^ 0 = a       5^0 = 5
Cancellation:   a^b^a = b       5^3^5 = 3

These properties make XOR ideal for "toggling" or "cancelling" values.
```

### Find Single Non-Duplicate (All Others Appear Twice)

```
arr = [4, 1, 2, 1, 2]

XOR all elements:
  result = 4 ^ 1 ^ 2 ^ 1 ^ 2
         = 4 ^ (1^1) ^ (2^2)
         = 4 ^ 0 ^ 0
         = 4

The pairs cancel out! The lone element survives.
Only O(1) extra space (one variable), one pass = O(N) time.
```

### Find Two Non-Duplicate Elements

```
arr = [1, 2, 1, 3, 2, 5]  two non-duplicates: 3 and 5

Step 1: XOR all → result = 3 ^ 5 = 110 ^ 101 = 011 = 6
        (bit 1 and bit 2 are set — represent differences between 3 and 5)

Step 2: Find any set bit in result (rightmost set bit):
        rightmost = result & (-result) = 011 & 101 = 001

Step 3: Partition array into two groups by that bit:
        bit 0 set:   1,1,3,5 (indices where bit0 is 1)
        bit 0 unset: 2,2     (indices where bit0 is 0)

Step 4: XOR each group separately:
        group1: 1^1^3^5 = 0^3^5 = 6... 
        
        Hmm let me redo: bit = 001
        group1 (bit0=1): 1,1,3,5  → 1^1^3^5 = 0^6 = 6? No.
        Actually: 1^1=0, 3^5=6, 0^6=6. Not right.
        
        Correct: 3=011, 5=101. bit=rightmost bit of XOR(3,5)=6=110
        rightmost set bit of 6 = 010 (bit 1)
        group (bit1=1): 2,2,3 → 2^2^3 = 3  ← first unique
        group (bit1=0): 1,1,5 → 1^1^5 = 5  ← second unique
        
        Found: 3 and 5! O(N) time, O(1) space.
```

### C: XOR Bit Tricks

```c
#include <stdio.h>

// Find single non-duplicate in O(N), O(1)
int find_single(int *arr, int n) {
    int result = 0;
    for (int i = 0; i < n; i++) {
        result ^= arr[i];  // pairs cancel, singleton survives
    }
    return result;
}

// Find two non-duplicates in O(N), O(1)
void find_two_singles(int *arr, int n, int *a, int *b) {
    int xor_all = 0;
    for (int i = 0; i < n; i++) xor_all ^= arr[i];  // = x ^ y

    // Isolate rightmost set bit (the bit where x and y differ)
    int diff_bit = xor_all & (-xor_all);

    *a = 0; *b = 0;
    for (int i = 0; i < n; i++) {
        if (arr[i] & diff_bit) *a ^= arr[i];  // group A
        else                   *b ^= arr[i];  // group B
    }
    // *a and *b are now the two non-duplicates
}

// In-place: mark visited positions using bit tricks (for value range [0..N-1])
// arr[i] = value originally; use bit N to mark visited
// Works when values fit in (N-1) bits
void find_missing_bit_trick(int *arr, int n) {
    int flag_bit = 1 << 30;  // use high bit as visited flag

    for (int i = 0; i < n; i++) {
        int val = arr[i] & ~flag_bit;  // strip flag to get real value
        if (val < n) arr[val] |= flag_bit;  // mark arr[val] as "seen val"
    }

    printf("Missing: ");
    for (int i = 0; i < n; i++) {
        if (!(arr[i] & flag_bit)) printf("%d ", i);  // never marked
    }
    printf("\n");

    // Restore by clearing flag bits
    for (int i = 0; i < n; i++) arr[i] &= ~flag_bit;
}

int main() {
    int arr1[] = {4, 1, 2, 1, 2};
    printf("Single: %d\n", find_single(arr1, 5));  // 4

    int arr2[] = {1, 2, 1, 3, 2, 5};
    int x, y;
    find_two_singles(arr2, 6, &x, &y);
    printf("Two singles: %d %d\n", x, y);  // 3 5

    return 0;
}
```

### Rust: XOR Tricks

```rust
fn find_single(arr: &[i32]) -> i32 {
    arr.iter().fold(0, |acc, &x| acc ^ x)
    // fold starts with 0, XORs every element.
    // Pairs cancel: a^a=0. Only unique element remains.
}

fn find_two_singles(arr: &[i32]) -> (i32, i32) {
    let xor_all: i32 = arr.iter().fold(0, |acc, &x| acc ^ x);
    let diff_bit = xor_all & (-xor_all);  // isolate rightmost set bit

    let (mut a, mut b) = (0i32, 0i32);
    for &x in arr {
        if x & diff_bit != 0 { a ^= x; }
        else                  { b ^= x; }
    }
    (a, b)
}

fn main() {
    let arr1 = vec![4, 1, 2, 1, 2];
    println!("Single: {}", find_single(&arr1)); // 4

    let arr2 = vec![1, 2, 1, 3, 2, 5];
    println!("Two singles: {:?}", find_two_singles(&arr2)); // (5, 3) or (3, 5)
}
```

---

## 13. In-Place Merge (Without Extra Space)

Merging two sorted halves of an array without any extra buffer. This is the hard one — it costs O(N log N) time when done naively in-place, versus O(N) with an extra buffer.

### Why In-Place Merge is Hard

```
Given sorted halves:
arr = [1, 3, 5 | 2, 4, 6]
       Left half  Right half

Naive out-of-place: create buffer, merge into it, copy back.

In-place: When you place 2 before 3, you must shift [3,5] right:
  [1, 2, 3, 5 | 4, 6]  → shifting is O(N) per insertion → O(N²) total.

Better approach: use rotation (reversal) to achieve O(N log N).
```

### Block Merge Using Rotation

```
arr = [1, 3, 5, 7 | 2, 4, 6, 8]

Find where right half's first element belongs in left half:
  right[0]=2, insert position in left = index 1 (after 1, before 3)

Rotate arr[1..4] (i.e., [3,5,7,2]) to place 2 at index 1:
  [1, 2, 3, 5, 7 | 4, 6, 8]
  
Now left half is [1,2] (merged), and we have:
  [3,5,7 | 4,6,8] remaining.
  
Recurse on [3,5,7,4,6,8]:
  right[0]=4, insert position = index 1 (after 3)
  Rotate [5,7,4] → [4,5,7]:
  [3,4,5,7,6,8]
  
And so on. Each rotation is O(N), and we do O(log N) of them → O(N log N).
```

### C: In-Place Merge (rotation-based)

```c
#include <stdio.h>

void reverse_range(int *arr, int lo, int hi) {
    while (lo < hi) {
        int tmp = arr[lo]; arr[lo] = arr[hi]; arr[hi] = tmp;
        lo++; hi--;
    }
}

// Rotate arr[lo..mid] and arr[mid+1..hi] so that right part comes first.
// Then reverse the whole to get "right part" shifted left.
// This achieves: arr[mid+1..hi] followed by arr[lo..mid]
void rotate_merge(int *arr, int lo, int mid, int hi) {
    reverse_range(arr, lo, mid);
    reverse_range(arr, mid + 1, hi);
    reverse_range(arr, lo, hi);
}

// Binary search: find insertion position of key in arr[lo..hi] (sorted)
int lower_bound(int *arr, int lo, int hi, int key) {
    while (lo < hi) {
        int mid = lo + (hi - lo) / 2;
        if (arr[mid] < key) lo = mid + 1;
        else hi = mid;
    }
    return lo;
}

// In-place merge of two sorted halves: arr[lo..mid] and arr[mid+1..hi]
void inplace_merge(int *arr, int lo, int mid, int hi) {
    if (lo >= mid || mid >= hi) return;
    if (mid - lo == 0 || hi - mid == 0) return;

    int mid2 = lo + (hi - lo) / 2;

    int lo2;
    if (mid2 <= mid) {
        lo2 = lower_bound(arr, mid + 1, hi + 1, arr[mid2]);
    } else {
        lo2 = lower_bound(arr, lo, mid + 1, arr[mid2]);
    }

    // Rotate to merge
    int new_mid = mid2 + (lo2 - mid - 1);
    rotate_merge(arr, mid2, mid, lo2 - 1);
    inplace_merge(arr, lo, mid2 - 1, new_mid);
    inplace_merge(arr, new_mid + 1, lo2 - 1 + (mid2 <= mid ? 1 : 0), hi);
}

// Simple O(N^2) in-place merge (insertion sort style — easier to understand)
void inplace_merge_simple(int *arr, int lo, int mid, int hi) {
    // i scans left half, j scans right half
    int i = lo, j = mid + 1;
    while (i <= mid && j <= hi) {
        if (arr[i] <= arr[j]) {
            i++;  // already in place
        } else {
            // arr[j] needs to go before arr[i]
            // Shift arr[i..j-1] one step right, insert arr[j] at position i
            int key = arr[j];
            int k = j;
            while (k > i) {
                arr[k] = arr[k-1];  // shift right
                k--;
            }
            arr[i] = key;
            i++;
            mid++;  // mid has shifted right by 1
            j++;
        }
    }
    // O(N^2) worst case but O(1) extra space
}

int main() {
    int arr[] = {1, 3, 5, 7, 2, 4, 6, 8};
    inplace_merge_simple(arr, 0, 3, 7);
    for (int i = 0; i < 8; i++) printf("%d ", arr[i]);
    // 1 2 3 4 5 6 7 8
    return 0;
}
```

### Rust: In-Place Merge (simple)

```rust
fn inplace_merge(arr: &mut [i32], mid: usize) {
    // arr[0..mid] and arr[mid..] are both sorted
    let mut i = 0;
    let mut j = mid;

    while i < j && j < arr.len() {
        if arr[i] <= arr[j] {
            i += 1;
        } else {
            // Rotate arr[i..=j] left by 1 (move arr[j] to position i)
            let key = arr[j];
            let mut k = j;
            while k > i {
                arr[k] = arr[k - 1];
                k -= 1;
            }
            arr[i] = key;
            i += 1;
            j += 1;
        }
    }
}

fn main() {
    let mut arr = vec![1, 3, 5, 7, 2, 4, 6, 8];
    inplace_merge(&mut arr, 4);
    println!("{:?}", arr); // [1, 2, 3, 4, 5, 6, 7, 8]

    // Note: Rust stdlib has slice::rotate_left which is O(N) and
    // can be used to build O(N log N) in-place merge sort.
}
```

---

## 14. Fisher-Yates In-Place Shuffle

The only correct way to shuffle an array with uniform probability, done in-place.

### Why Naive Shuffle is Wrong

```
WRONG (biased) shuffle:
for i in 0..n:
    j = random(0, n-1)  ← pick from ENTIRE array
    swap(arr[i], arr[j])

For n=3: 3^3 = 27 possible outcomes, but 3! = 6 permutations.
27 is not divisible by 6, so some permutations are more likely. BIASED.

CORRECT Fisher-Yates:
for i from n-1 downto 0:
    j = random(0, i)  ← pick from arr[0..i] (shrinking range)
    swap(arr[i], arr[j])

For n=3: 3 × 2 × 1 = 6 outcomes, exactly 6 permutations. UNIFORM.
```

### Visual of Fisher-Yates

```
arr = [A, B, C, D, E]  (n=5)

i=4: j = random(0..4) = 2  swap(arr[4], arr[2])
     [A, B, E, D, C]      ← C is fixed at end

i=3: j = random(0..3) = 0  swap(arr[3], arr[0])
     [D, B, E, A, C]      ← A is fixed at pos 3

i=2: j = random(0..2) = 2  swap(arr[2], arr[2])  (no-op)
     [D, B, E, A, C]      ← E is fixed at pos 2

i=1: j = random(0..1) = 0  swap(arr[1], arr[0])
     [B, D, E, A, C]      ← D is fixed at pos 1

i=0: done. B is fixed at pos 0.

Shuffle result: [B, D, E, A, C]
Only O(1) extra space (loop var i, random j, swap temp).
```

### C: Fisher-Yates

```c
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

void shuffle(int *arr, int n) {
    for (int i = n - 1; i > 0; i--) {
        // j in [0, i] inclusive
        int j = rand() % (i + 1);
        int tmp = arr[i]; arr[i] = arr[j]; arr[j] = tmp;
    }
}

int main() {
    srand(time(NULL));
    int arr[] = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10};
    shuffle(arr, 10);
    for (int i = 0; i < 10; i++) printf("%d ", arr[i]);
    // e.g.: 7 3 10 1 5 8 2 9 4 6 (random)
    return 0;
}
```

### Rust: Fisher-Yates

```rust
use std::time::{SystemTime, UNIX_EPOCH};

// Simple LCG for demo (use rand crate in production)
struct Lcg { state: u64 }
impl Lcg {
    fn new() -> Self {
        let seed = SystemTime::now().duration_since(UNIX_EPOCH)
            .unwrap().as_nanos() as u64;
        Lcg { state: seed }
    }
    fn next_usize(&mut self, bound: usize) -> usize {
        self.state = self.state.wrapping_mul(6364136223846793005)
            .wrapping_add(1442695040888963407);
        ((self.state >> 33) as usize) % bound
    }
}

fn shuffle(arr: &mut Vec<i32>) {
    let mut rng = Lcg::new();
    for i in (1..arr.len()).rev() {
        let j = rng.next_usize(i + 1);
        arr.swap(i, j);
    }
}

fn main() {
    let mut arr = vec![1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
    shuffle(&mut arr);
    println!("{:?}", arr);
}
```

### Go: Fisher-Yates

```go
package main

import (
    "fmt"
    "math/rand"
    "time"
)

func shuffle(arr []int) {
    r := rand.New(rand.NewSource(time.Now().UnixNano()))
    for i := len(arr) - 1; i > 0; i-- {
        j := r.Intn(i + 1)  // j in [0, i]
        arr[i], arr[j] = arr[j], arr[i]
    }
}

func main() {
    arr := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
    shuffle(arr)
    fmt.Println(arr)
}
```

---

## 15. Floyd's Cycle Detection (In-Place)

Two pointers moving at different speeds through an array-like structure to detect cycles. O(1) space.

### Tortoise and Hare

```
For finding duplicate in arr of n+1 elements, values in [1..n]:
Think of arr as a linked list where arr[i] points to arr[arr[i]].

arr = [3, 1, 3, 4, 2]  (n=4, duplicate is 3)

"Linked list" view:
index: 0→1→2→3→4
value: 3  1  3  4  2

Paths:
0 → arr[0]=3 → arr[3]=4 → arr[4]=2 → arr[2]=3 → arr[3]=4 → ...
                                              ↑ cycle starts here!

Floyd's algorithm:
Phase 1 (find meeting point):
  slow = arr[slow]       (moves 1 step)
  fast = arr[arr[fast]]  (moves 2 steps)

  slow  fast  (both start at arr[0])
  3     arr[arr[0]]=arr[3]=4
  arr[3]=4   arr[arr[4]]=arr[2]=3
  arr[4]=2   arr[arr[3]]=arr[4]=2   ← slow=2, fast=2 → MET

Phase 2 (find cycle entry = duplicate):
  Reset slow to arr[0]=3. Keep fast at meeting point=2.
  Move both 1 step at a time:
  slow=arr[3]=4, fast=arr[2]=3
  slow=arr[4]=2, fast=arr[3]=4
  slow=arr[2]=3, fast=arr[4]=2
  slow=arr[3]=4, fast=arr[2]=3
  ...wait let me redo from start:

  slow=0 (index, not value), fast=meeting=2 (index)
  Hmm: let's use index-based:
  
  Phase1: slow=0, fast=0
  step1: slow=arr[0]=3, fast=arr[arr[0]]=arr[3]=4
  step2: slow=arr[3]=4, fast=arr[arr[4]]=arr[2]=3
  step3: slow=arr[4]=2, fast=arr[arr[3]]=arr[4]=2 → MEET at index 2

  Phase2: slow=0, fast=2
  step1: slow=arr[0]=3, fast=arr[2]=3 → MEET at 3

  Duplicate = arr[meeting] = arr[3-1]? No, the meeting VALUE is 3.
  The cycle entry index is 3 → value 3 is the duplicate! ✓
```

### C: Floyd's Cycle Detection for Duplicate Finding

```c
#include <stdio.h>

// Find duplicate in arr where n+1 elements, values [1..n]
// O(N) time, O(1) space — treats array as implicit linked list
int find_duplicate(int *arr, int n) {
    // Phase 1: find meeting point
    int slow = arr[0];
    int fast = arr[arr[0]];

    while (slow != fast) {
        slow = arr[slow];
        fast = arr[arr[fast]];
    }

    // Phase 2: find cycle entry (= duplicate number)
    slow = 0;  // reset to start
    while (slow != fast) {
        slow = arr[slow];
        fast = arr[fast];  // both move 1 step now
    }

    return slow;  // this is the duplicate value
}

int main() {
    int arr[] = {3, 1, 3, 4, 2};  // n=4
    printf("Duplicate: %d\n", find_duplicate(arr, 4));  // 3

    int arr2[] = {1, 3, 4, 2, 2};  // n=4
    printf("Duplicate: %d\n", find_duplicate(arr2, 4));  // 2
    return 0;
}
```

### Rust: Floyd's Cycle Detection

```rust
fn find_duplicate(arr: &[usize]) -> usize {
    let mut slow = arr[0];
    let mut fast = arr[arr[0]];

    while slow != fast {
        slow = arr[slow];
        fast = arr[arr[fast]];
    }

    slow = 0;
    while slow != fast {
        slow = arr[slow];
        fast = arr[fast];
    }

    slow
}

fn main() {
    let arr = vec![3, 1, 3, 4, 2];
    println!("Duplicate: {}", find_duplicate(&arr)); // 3
}
```

---

## 16. In-Place Matrix Transformations

2D matrices flattened into 1D arrays. Rotations and transpositions happen in-place.

### Transpose a Matrix

```
Original 3x3 matrix (stored row-major in memory):

     col0 col1 col2
row0 [  1,  2,  3 ]    Memory: [1,2,3,4,5,6,7,8,9]
row1 [  4,  5,  6 ]    Index:   0 1 2 3 4 5 6 7 8
row2 [  7,  8,  9 ]

arr[i][j] = arr[i*n + j] in 1D layout.

Transpose: swap arr[i][j] with arr[j][i]
Only swap where j > i (upper triangle):

(0,1)↔(1,0): swap arr[1] and arr[3]   → 2 ↔ 4
(0,2)↔(2,0): swap arr[2] and arr[6]   → 3 ↔ 7
(1,2)↔(2,1): swap arr[5] and arr[7]   → 6 ↔ 8

Result:
[  1,  4,  7 ]
[  2,  5,  8 ]
[  3,  6,  9 ]

Memory: [1,4,7,2,5,8,3,6,9]
```

### Rotate 90° Clockwise = Transpose + Reverse Each Row

```
Original:        Transpose:       Reverse rows:
[1, 2, 3]        [1, 4, 7]        [7, 4, 1]
[4, 5, 6]  ───→  [2, 5, 8]  ───→  [8, 5, 2]
[7, 8, 9]        [3, 6, 9]        [9, 6, 3]

Two steps, both in-place!

Rotate 90° Counter-clockwise = Reverse Each Row + Transpose
Rotate 180° = Reverse entire flattened array
```

### C: Matrix Rotation

```c
#include <stdio.h>

void rotate_90_clockwise(int arr[], int n) {
    // Step 1: Transpose (swap upper triangle with lower)
    for (int i = 0; i < n; i++) {
        for (int j = i + 1; j < n; j++) {
            int tmp = arr[i*n + j];
            arr[i*n + j] = arr[j*n + i];
            arr[j*n + i] = tmp;
        }
    }

    // Step 2: Reverse each row
    for (int i = 0; i < n; i++) {
        int lo = i * n, hi = i * n + (n - 1);
        while (lo < hi) {
            int tmp = arr[lo]; arr[lo] = arr[hi]; arr[hi] = tmp;
            lo++; hi--;
        }
    }
}

void print_matrix(int arr[], int n) {
    for (int i = 0; i < n; i++) {
        for (int j = 0; j < n; j++) printf("%2d ", arr[i*n + j]);
        printf("\n");
    }
}

int main() {
    // 3x3 matrix stored as flat 1D array
    int mat[] = {1, 2, 3,
                 4, 5, 6,
                 7, 8, 9};
    rotate_90_clockwise(mat, 3);
    print_matrix(mat, 3);
    // 7  4  1
    // 8  5  2
    // 9  6  3
    return 0;
}
```

### Rust: Matrix Rotation

```rust
fn rotate_90_clockwise(arr: &mut Vec<i32>, n: usize) {
    // Transpose
    for i in 0..n {
        for j in (i+1)..n {
            let (a, b) = (i * n + j, j * n + i);
            arr.swap(a, b);
        }
    }
    // Reverse each row
    for i in 0..n {
        let (lo, hi) = (i * n, i * n + n - 1);
        let mut (mut l, mut r) = (lo, hi);
        while l < r {
            arr.swap(l, r);
            l += 1;
            r -= 1;
        }
    }
}

fn main() {
    let mut mat = vec![
        1, 2, 3,
        4, 5, 6,
        7, 8, 9,
    ];
    rotate_90_clockwise(&mut mat, 3);
    for i in 0..3 {
        println!("{:?}", &mat[i*3..i*3+3]);
    }
    // [7, 4, 1]
    // [8, 5, 2]
    // [9, 6, 3]
}
```

### Go: Matrix Rotation

```go
package main

import "fmt"

func rotate90(mat []int, n int) {
    // Transpose
    for i := 0; i < n; i++ {
        for j := i + 1; j < n; j++ {
            mat[i*n+j], mat[j*n+i] = mat[j*n+i], mat[i*n+j]
        }
    }
    // Reverse each row
    for i := 0; i < n; i++ {
        lo, hi := i*n, i*n+n-1
        for lo < hi {
            mat[lo], mat[hi] = mat[hi], mat[lo]
            lo++
            hi--
        }
    }
}

func main() {
    mat := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
    rotate90(mat, 3)
    for i := 0; i < 3; i++ {
        fmt.Println(mat[i*3 : i*3+3])
    }
    // [7 4 1]
    // [8 5 2]
    // [9 6 3]
}
```

---

## 17. Cache Behavior and Why It Matters

This is where in-place operations unlock real-world performance, not just theoretical space savings.

### Cache Lines and Locality

```
Modern CPU cache line = 64 bytes.
For int32 (4 bytes): one cache line holds 16 integers.

arr = [e0, e1, e2, ..., e15, e16, e17, ...]
       ├──────── cache line 1 ────────┤├── cache line 2...

Accessing arr[0] fetches the ENTIRE first cache line into L1.
Accessing arr[1]..arr[15] → ZERO extra memory fetches!

In-place two-pointer on arr[0..15]:
  L=0, R=15: both on same cache line → blazing fast
  L=8, R=23: cross cache lines → 2 cache lines in L1 → still fast

Out-of-place on arr[0..15] + copy[0..15]:
  arr on cache line A, copy on cache line B (different memory region)
  Every write to copy touches a DIFFERENT set of cache lines.
  Effective bandwidth HALVED.
```

### Spatial Locality Example: Reversal

```
In-place reversal:
  - Working set: 1 array, 1 cache region (or 2 at edges)
  - L1 cache hit rate: ~95%+ for small arrays

Out-of-place reversal:
  - Working set: 2 arrays
  - Write to destination may evict read from source
  - L1 cache hit rate drops as arrays grow

Benchmark expectation (N = 1,000,000 ints):
  In-place:      ~1.5 ms  (data stays in L3)
  Out-of-place:  ~3.0 ms  (double the memory traffic)
```

### TLB Pressure

```
TLB (Translation Lookaside Buffer): cache for virtual→physical address translations.
Size: ~64 entries (L1 TLB), ~1024 entries (L2 TLB).
Each entry covers 4KB page.

L1 TLB covers: 64 × 4KB = 256KB

In-place: One array. ~1 TLB entry per 1000 ints. Low TLB pressure.
Out-of-place: Two arrays. Double the pages. More TLB misses.
              Each TLB miss = ~100 cycle penalty.
```

### Write Amplification

```
CPU writes go through write buffers → L1 → L2 → L3 → DRAM.

In-place swap of arr[i] and arr[j]:
  READ:  arr[i], arr[j]   → 2 reads
  WRITE: arr[i], arr[j]   → 2 writes (same cache lines as reads → write-hit)
  Total cache writes: 2

Out-of-place copy then reverse:
  READ  arr[i]        → 1 read
  WRITE copy[n-1-i]   → 1 write (DIFFERENT cache line → write-miss likely)
  Total cache writes: N (one per element, all potentially cold)
```

---

## 18. Memory Safety: Rust's Perspective

Rust's borrow checker makes in-place operations safe by design. Understanding why illuminates the rules of in-place programming.

### The Aliasing Problem

```
In C, this is undefined behavior:
  int *a = &arr[0];
  int *b = &arr[0];  // two mutable pointers to same location
  *a = 5;
  *b = 10;
  // Which write wins? Compiler can reorder these freely!

Rust prevents this at COMPILE TIME:
  let mut arr = vec![1, 2, 3];
  let a = &mut arr[0];
  let b = &mut arr[0];  // ERROR: cannot borrow arr[0] mutably twice
  // Compiler rejects this program.
```

### How Rust Enables Safe In-Place via split_at_mut

```
Problem: swap arr[0] and arr[2] — you need two mutable references.
         Rust won't let you have &mut arr[0] and &mut arr[2] simultaneously.

Solution: split_at_mut divides the slice into non-overlapping parts.

let mut arr = vec![1, 2, 3, 4, 5];

// Split at index 3: left=[1,2,3], right=[4,5]
let (left, right) = arr.split_at_mut(3);

// Now we have:
//   left:  &mut [i32]  →  covers arr[0..2]
//   right: &mut [i32]  →  covers arr[3..4]
// They DO NOT OVERLAP → both mutable at once is safe!

std::mem::swap(&mut left[0], &mut right[1]);
// Swaps arr[0] with arr[4]: safe, no aliasing.

// Under the hood, split_at_mut uses unsafe:
// pub fn split_at_mut(&mut self, mid: usize) -> (&mut [T], &mut [T]) {
//     let len = self.len();
//     let ptr = self.as_mut_ptr();
//     unsafe {
//         (from_raw_parts_mut(ptr, mid),
//          from_raw_parts_mut(ptr.add(mid), len - mid))
//     }
// }
// The unsafe code is SOUND because mid <= len ensures no overlap.
```

### Rust's In-Place Iterator Adapters

```rust
// Rust's iter_mut() and map in-place:
let mut arr = vec![1, 2, 3, 4, 5];

// In-place mutation via iter_mut (no allocation):
for x in arr.iter_mut() {
    *x *= 2;  // modifies in-place
}
// arr = [2, 4, 6, 8, 10]

// In-place retain (filter without allocation):
arr.retain(|&x| x % 4 == 0);
// arr = [4, 8] — remove elements not divisible by 4

// Under the hood, retain() uses a write pointer (same as our 2-pointer technique)
// It's literally the fast/slow pointer pattern with write pointer.

// In-place sort:
let mut arr = vec![3, 1, 4, 1, 5, 9, 2, 6];
arr.sort_unstable();  // in-place quicksort, O(N log N), O(log N) stack
// arr = [1, 1, 2, 3, 4, 5, 6, 9]

// arr.sort() = in-place timsort (stable)
// arr.sort_unstable() = in-place pdqsort (faster, unstable)
```

### Go's Slice Header and In-Place Safety

```
Go slice = (pointer, length, capacity)

Passed to a function:
┌──────────────────────────────────────────────────────────┐
│  func modify(arr []int) {                                │
│      // arr is a COPY of the slice header                │
│      // but the DATA pointer points to SAME underlying   │
│      // array!                                           │
│      arr[0] = 99  // modifies the ORIGINAL array        │
│  }                                                       │
│                                                          │
│  Caller:  slice = {ptr:0x100, len:5, cap:5}             │
│  Callee:  arr   = {ptr:0x100, len:5, cap:5} ← same ptr │
└──────────────────────────────────────────────────────────┘

This means Go functions can do in-place ops by receiving []T.
They share the underlying array automatically.

CAVEAT: append() may allocate a new array if cap is exceeded,
        breaking the in-place contract! Always check if needed.
```

---

## 19. Trade-offs and When NOT to Use In-Place

In-place is not always better. A clear mental model requires knowing the downsides.

### When In-Place Is WRONG

```
1. CONCURRENCY
   ┌────────────────────────────────────────────────────┐
   │ Goroutine A: reverse(arr, 0, 4)                    │
   │ Goroutine B: read arr[2]                           │
   │                                                    │
   │ Race condition! In-place mutates shared state.     │
   │ Out-of-place copy → each thread has its own view.  │
   └────────────────────────────────────────────────────┘

2. NEED ORIGINAL DATA
   If you need both original and transformed:
   - In-place destroys original.
   - Must copy before transforming, or use out-of-place.

3. PERSISTENCE / AUDIT TRAIL
   Event sourcing, database write-ahead logs, git history:
   ALL are out-of-place by design. Mutation destroys history.

4. COMPLEX ALGORITHMS WHERE IN-PLACE IS SLOWER
   In-place merge: O(N log N) vs O(N) out-of-place.
   In some contexts, the extra O(N) memory is cheaper than
   the extra O(log N) time factor.

5. FUNCTIONAL PROGRAMMING STYLE
   Pure functions: no side effects, no mutation.
   Easier to reason about, test, compose.
   Immutability is a correctness guarantee.

6. SMALL N
   For N < 100, cache effects are negligible.
   Code clarity > micro-optimization.
```

### Complexity Trade-off Table

```
Operation              In-Place        Out-of-Place
────────────────────────────────────────────────────────
Reverse                O(N), O(1)      O(N), O(N)
Sort (comparison)      O(NlogN), O(1)* O(NlogN), O(N)
Merge two halves       O(NlogN), O(1)  O(N), O(N)
Remove duplicates      O(N), O(1)†     O(N), O(N)
Filter elements        O(N), O(1)      O(N), O(N)
Matrix rotate          O(N²), O(1)     O(N²), O(N²)
Find duplicates        O(N), O(1)‡     O(N), O(N)

* O(log N) for recursive call stack in quicksort
† Requires sorted input
‡ Uses index-marking trick (modifies input temporarily)
```

### Decision Framework

```
Should I use in-place?

START
  │
  ▼
Do you need to preserve the original array?
  │─── YES ──→ Out-of-place. Copy first if mutation is needed later.
  │
  ▼ NO
Is the operation concurrent (multiple threads)?
  │─── YES ──→ Out-of-place with immutable shared data, or locks.
  │
  ▼ NO
Is N very small (< ~100 elements)?
  │─── YES ──→ Either. Clarity wins. Out-of-place is fine.
  │
  ▼ NO
Does in-place increase time complexity significantly?
  │         (e.g., in-place merge is O(N log N) vs O(N))
  │─── YES ──→ Weigh time vs space. Profile.
  │
  ▼ NO
Is memory constrained? (embedded, large N near RAM limit)
  │─── YES ──→ In-place. Space matters.
  │
  ▼ NO
Default: In-place is usually fine. Choose for clarity.
```

---

## 20. Mental Model Summary

### The Core Intuitions

```
┌──────────────────────────────────────────────────────────────────┐
│                  IN-PLACE MENTAL MODEL                           │
│                                                                  │
│  1. ARRAY = CONTIGUOUS MEMORY BLOCK                              │
│     arr[i] = *(base_ptr + i * elem_size)                         │
│     Read/write any index in O(1). This is your superpower.       │
│                                                                  │
│  2. SWAP = ATOMIC UNIT OF IN-PLACE WORK                          │
│     Every rearrangement decomposes into swaps.                   │
│     Swap costs O(1). N swaps costs O(N).                         │
│                                                                  │
│  3. TWO-POINTER = THE UNIVERSAL PATTERN                          │
│     Converging: process from both ends toward center.            │
│     Fast+slow:  compact, filter, partition.                      │
│                                                                  │
│  4. INVARIANTS MAINTAIN STRUCTURE                                 │
│     Dutch flag: [0s][1s][unknowns][2s] — maintained at every    │
│     step. This is how you prove correctness.                     │
│                                                                  │
│  5. CLEVER ENCODING = FREE METADATA                               │
│     Sign bits, high bits, index arithmetic — use the data        │
│     itself to store state. Unlock O(1) space tricks.             │
│                                                                  │
│  6. LOCALITY = PERFORMANCE                                        │
│     In-place keeps working set small. Cache loves it.            │
│     One array = one region of memory = high hit rate.            │
│                                                                  │
│  7. MUTATION = RESPONSIBILITY                                     │
│     In-place destroys original. Know when that's acceptable.     │
│     Rust's borrow checker enforces this contract.                │
└──────────────────────────────────────────────────────────────────┘
```

### Pattern Cheat Sheet

```
PROBLEM                          TECHNIQUE
──────────────────────────────────────────────────────────────────
Reverse / rotate array          Reversal algorithm (3 reverses)
Sort in minimal space           Quicksort (Lomuto/Hoare partition)
Remove/filter elements          Fast+slow two pointers
Partition into groups           Dutch National Flag
Shuffle uniformly               Fisher-Yates (backward scan)
Find duplicates, missing        Index marking (sign as flag)
Cycle detection                 Floyd's tortoise & hare
Matrix rotate 90°               Transpose + reverse rows
Merge sorted halves in-place    Rotation-based merge
No-temp swap                    XOR swap (check aliasing!)
Parity / single element         XOR fold over array
```

### Space Complexity Quick Reference

```
O(1)   → True in-place. Only a constant number of extra variables.
          Examples: two-pointer, Dutch flag, reversal, swap.

O(log N) → Quasi in-place. Recursive stack space.
            Examples: quicksort, binary search, recursive merge.

O(N)   → NOT in-place. Linear extra space.
          Examples: merge sort (aux buffer), out-of-place copy.

The "in-place" guarantee:
  Space used by your algorithm = O(1) or O(log N)
  NOT counting the input array itself.
```

### Correctness Checklist for In-Place Operations

```
Before writing any in-place algorithm, ask:
  □ What is the invariant at each step?
  □ Do I handle the case where two pointers meet?
  □ Does my swap check for aliasing (same index)?
  □ Does my algorithm handle n=0, n=1, n=2 correctly?
  □ If using index marking: can I restore the array?
  □ If rotating: does k=0, k=n, k>n work?
  □ Am I truly O(1) extra space, or hiding a buffer somewhere?
  □ If recursive: what's the actual stack depth?
  □ Is the operation thread-safe in my context?
```

---

*End of Guide*

**Key takeaway:** In-place programming is about understanding memory as a resource — contiguous, addressable, finite — and using the data's own structure (indices, values, signs, bits) as the only metadata you need. The techniques in this guide cover every major pattern; mastering them gives you the vocabulary to recognize and solve any in-place problem.
