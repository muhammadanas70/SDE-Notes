# Priority Queues: The Complete Mental-Model Guide (Go · C · Rust)

> Goal of this document: after reading it you should be able to **derive** every priority-queue
> operation from first principles, **predict** what the memory looks like at every step, **choose**
> the right variant for a problem, and **implement** it in Go, C, and Rust without looking anything up.
>
> All diagrams are plain ASCII. Every code block marked with a `// file:` comment was compiled and
> run (see [Section 18](#18-how-the-code-in-this-guide-was-verified)).

---

## Table of Contents

1. [What a priority queue really is (ADT vs data structure)](#1-what-a-priority-queue-really-is)
2. [Why simpler structures fail](#2-why-simpler-structures-fail)
3. [The binary heap: mental model](#3-the-binary-heap-mental-model)
4. [Memory architecture: how a heap lives in RAM](#4-memory-architecture)
5. [Core manipulations, one by one](#5-core-manipulations)
6. [Complexity: the *why*, including the O(n) heapify proof](#6-complexity-analysis)
7. [Language mechanics: Go vs C vs Rust](#7-language-mechanics)
8. [Implementation: Go](#8-implementation-go)
9. [Implementation: C](#9-implementation-c)
10. [Implementation: Rust](#10-implementation-rust)
11. [Advanced manipulations: update, remove, lazy deletion, indexed heaps](#11-advanced-manipulations)
12. [Heap variants: d-ary, binomial, Fibonacci, pairing, radix, bucket](#12-heap-variants)
13. [Problem patterns (with code)](#13-problem-patterns)
14. [Real-world architecture](#14-real-world-architecture)
15. [Common mistakes and how to debug them](#15-common-mistakes-and-debugging)
16. [Active recall (attempt before scrolling)](#16-active-recall)
17. [Independent practice and progress tracker](#17-independent-practice-and-progress-tracker)
18. [How the code in this guide was verified](#18-how-the-code-in-this-guide-was-verified)

---

## 1. What a priority queue really is

### 1.1 The abstract data type (ADT)

A **priority queue** is a collection of items, each with a *priority*, that supports:

| Operation        | Meaning                                              |
| ---------------- | ---------------------------------------------------- |
| `push(x)`        | Insert an item                                       |
| `peek()`         | Look at the highest-priority item (do not remove)    |
| `pop()`          | Remove and return the highest-priority item          |
| `len()/empty()`  | Size queries                                         |

"Highest priority" is defined by a **strict weak ordering** you provide. A *min-priority queue*
treats the smallest key as highest priority; a *max-priority queue* treats the largest key as highest.
They are the same structure with the comparison flipped.

Optional operations that appear in real systems (Section 11):

| Operation                     | Used by                                  |
| ----------------------------- | ---------------------------------------- |
| `heapify(array)` (bulk build) | heapsort, top-k, initial task loading    |
| `replace(x)` / `pushpop(x)`   | top-k streaming, sliding structures      |
| `decrease_key(id, k)`         | Dijkstra, Prim, A*                       |
| `update(id, k)` / `fix`       | schedulers, rate-limiters                |
| `remove(id)`                  | cancelling timers/tasks                  |
| `merge(a, b)` / `meld`        | mergeable heaps                          |

### 1.2 ADT versus data structure

```text
   ADT (the contract)                 Data structures (the implementations)
 +---------------------+            +----------------------------------------+
 | push / peek / pop   | <--------- | unsorted array      sorted array       |
 | "who is most        |            | binary heap  <-- what everyone uses    |
 |  important now?"    |            | d-ary heap          binomial heap      |
 +---------------------+            | Fibonacci heap      pairing heap       |
                                    | balanced BST        bucket/radix queue |
                                    +----------------------------------------+
```

A priority queue is **not** "a heap". A heap is the most common *implementation*. Confusing the
two is the root of many interview mistakes ("can a priority queue iterate in sorted order?" No,
the ADT doesn't promise that; a binary heap doesn't provide it).

### 1.3 What the queue does *not* promise

* It does **not** keep everything sorted. It only guarantees that the *top* is correct.
* It is **not stable**: two items with equal priority may pop in any order (fix in 11.5).
* It does **not** support fast search for an arbitrary item (fix with an index map, 11.3).

---

## 2. Why simpler structures fail

Suppose you need to repeatedly extract the minimum from a changing set of `n` items.

```text
Structure          push        pop-min     peek-min    Problem
-----------------  ----------  ----------  ----------  ---------------------------------
Unsorted array     O(1)        O(n)        O(n)        scan everything to find the min
Sorted array       O(n)        O(1)*       O(1)        shift elements on every insert
Sorted linked list O(n)        O(1)        O(1)        walk the list to find position
Balanced BST       O(log n)    O(log n)    O(log n)    correct but heavy: pointers, rotations
Binary heap        O(log n)    O(log n)    O(1)        <-- balanced cost, tiny constant
```

`*` pop-min from the *front* of an array is O(n) unless you keep it reversed (pop from the back).

The insight: **you don't need a full order, only the minimum.** A total order is expensive to
maintain (it encodes ~n log n bits of information). A *partial* order that always exposes the
minimum is much cheaper. The heap is exactly that partial order.

---

## 3. The binary heap: mental model

### 3.1 Two invariants

A binary heap satisfies two independent rules. Keep them separate in your head:

1. **Shape property:** the tree is a *complete* binary tree: every level is full except possibly the
   last, and the last level is filled left to right.
2. **Heap-order property (min-heap):** every node's key is `<=` the keys of its children.
   (For a max-heap, `>=`.)

```text
Valid min-heap (n = 7)               INVALID: order violated       INVALID: shape violated
                                     (child 2 < parent 3)          (hole in last level)

            1                                  1                            1
         /     \                            /     \                      /     \
        3       2                          3       5                    3       2
       / \     / \                        / \     / \                  /       / \
      7   4   9   5                      2   4   9   6                7       9   5
                                         ^
                                         2 < 3, but 3 is its parent
```

Important consequences of heap-order:

* The root is the global minimum (follow parents upward from any node: keys never increase).
* Siblings have **no** relationship to each other. `3` and `2` above are in "wrong" order and
  that is completely legal. This is why a heap is only a *partial* order.
* Any subtree of a heap is itself a heap. This is what makes recursion/sift loops correct.

### 3.2 Completeness lets us delete the pointers

A complete binary tree can be stored in an array **with no pointers at all**, using level order:

```text
tree:                            array (0-indexed):
              1  (i=0)
           /       \             index:  0   1   2   3   4   5   6
        3 (1)       2 (2)        value: [1] [3] [2] [7] [4] [9] [5]
        /  \        /  \                 ^   ^------^   ^------^
     7(3)  4(4)   9(5)  5(6)           root  level 1    level 2 ...
```

Index arithmetic (memorize both conventions; bugs come from mixing them):

```text
0-indexed:   parent(i) = (i - 1) / 2      left(i) = 2i + 1     right(i) = 2i + 2
1-indexed:   parent(i) = i / 2            left(i) = 2i         right(i) = 2i + 1
```

Derivation (0-indexed): level `k` starts at index `2^k - 1` and has `2^k` nodes. Node `i` at
offset `o` within its level has children at offset `2o` and `2o+1` in the next level, whose start
is `2^(k+1) - 1 = 2(2^k - 1) + 1`. So left child = `2(2^k - 1 + o) + 1 = 2i + 1`. ∎

Useful facts:

```text
height of heap with n nodes            = floor(log2 n)
number of leaves                       = ceil(n / 2)
first leaf index (0-indexed)           = floor(n / 2)
last internal node index (0-indexed)   = floor(n / 2) - 1     <-- heapify starts here
```

### 3.3 The single primitive: restore order locally

All heap manipulations are built from **two repair moves**. Learn them and everything else is
composition.

```text
SIFT-UP (a.k.a. bubble up, percolate up, swim)
   Use when: a node may be TOO SMALL for its position (violates its parent).
   Move it up while it is smaller than its parent.

SIFT-DOWN (a.k.a. bubble down, percolate down, sink, heapify-down)
   Use when: a node may be TOO LARGE for its position (violates its children).
   Swap it with its SMALLER child while that child is smaller than it.
```

Why "smaller child" in sift-down? Because after the swap, the smaller child becomes the parent of
the other child, and heap-order between them must still hold:

```text
Sift-down with x = 9 at the root; children are 4 and 6.

   Swap with 6 (larger child)  -> BROKEN          Swap with 4 (smaller child) -> OK
              6                                              4
            /   \                                          /   \
           9     4    <- 6 > 4 but 6 is 4's parent        9     6    <- 4 <= 6, order holds
```

---

## 4. Memory architecture

### 4.1 Layout in a contiguous buffer

```text
   Logical tree                        Physical memory (ints, 4 bytes each)

          2                            address:  0x1000 0x1004 0x1008 0x100C 0x1010 0x1014
        /   \                          value:    [  2  ][  5  ][  3  ][  9  ][  6  ][  8  ]
       5     3                          index:      0      1      2      3      4      5
      / \   /
     9   6 8

   parent/child hops are index arithmetic on ONE array:
   node 1 (value 5) -> children at 3 and 4 -> addresses 0x100C and 0x1010
```

Compare with a pointer-based tree (e.g., a BST node or a `[]*Item` heap):

```text
Array heap (values inline)                Pointer heap ([]*Item, Go container/heap style)

  slice header                             slice header           heap-allocated Items
 +-----+-----+-----+                      +-----+-----+-----+     scattered across the heap
 | ptr | len | cap |                      | ptr | len | cap |
 +--+--+-----+-----+                      +--+--+-----+-----+
    |                                        |
    v                                        v
 [ 2 | 5 | 3 | 9 | 6 | 8 ]                [ p0 | p1 | p2 | p3 ]
   contiguous, cache friendly                |    |    |    |
                                             v    v    v    v
                                          +----+ ...              each hop = pointer chase
                                          |Item|                   (likely cache miss)
                                          +----+
```

### 4.2 Per-language ownership of that buffer

```text
GO                                   C                                RUST
slice header (24 B) on stack         struct PQ on stack               Vec<T> (24 B) on stack
+-----+-----+-----+                  +---------------+                +-----+-----+-----+
| ptr | len | cap |                  | data  --------+--> malloc'd    | ptr | cap | len |
+--+--+-----+-----+                  | elem (size)   |    buffer      +--+--+-----+-----+
   |                                 | len, cap      |                   |
   v                                 | cmp (fn ptr)  |                   v
 backing array on GC heap            +---------------+                heap buffer, freed
 (freed by garbage collector,        you free() it manually           automatically when the
  after nothing references it)       (or leak / double free)          Vec is dropped (RAII)
```

* **Go:** growth by `append` doubles capacity (roughly; the exact growth policy changes with size and version).
  Popped slots still hold references until you zero them (a GC retention leak for pointer elements).
* **C:** you own everything. Generic C heaps store elements as raw bytes plus an element size and a comparator function pointer.
* **Rust:** `Vec` owns the buffer; moving an element out requires a safe swap/`pop` or careful `unsafe` "hole" tricks (std's `BinaryHeap` uses a `Hole` internally).

### 4.3 Cache behaviour (why heaps are fast, and where they aren't)

A 64-byte cache line holds 16 `int32` values.

```text
 cache line 0 (indices 0..15)                          cache line 1 (16..31)
 [ 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 ]            [ 16 17 ... ]

 Top 4 levels of the heap (indices 0..14) fit in ONE line  -> the hot part of sift-down is cache resident.
 Deep levels: child of i is 2i+1: jumps grow geometrically -> each level down is likely a NEW cache line.
```

* Siblings (`2i+1`, `2i+2`) are adjacent, so comparing the two children is one cache access.
* For huge heaps the bottom levels miss the cache and even the TLB. Mitigations: **d-ary heaps** (shallower tree, Section 12.1), or **B-heap** layouts that group subtrees into pages (Poul-Henning Kamp's well-known argument for virtual-memory-aware heaps).
* Storing 16-byte `(key, id)` structs inline beats a `[]*Item` heap for the same reason.

---

## 5. Core manipulations

All examples are **min-heaps** on 0-indexed arrays.

### 5.1 `peek()`: O(1)

Return `a[0]` (or "nothing" if empty). Go: `(T, bool)`. C: pointer or `NULL`. Rust: `Option<&T>`.

### 5.2 `push(x)`: append, then sift-up

```text
Algorithm
  1. Append x at index n (the first free slot: this preserves the SHAPE property).
  2. While i > 0 and a[i] < a[parent(i)]: swap them, i = parent(i)     (repairs HEAP-ORDER).
```

Why is it correct? Before the append, the heap is valid. The only possibly violated edge is
`(parent(i), i)`. After swapping, the moved-up value `x` is smaller than the old parent, which was
already `<=` its other child and everything below it, so `x` is smaller than all of them. The only
edge that can now be violated is the one above. Induction on levels.

Dry run: heap `[2, 5, 3, 9, 6]`, `push(1)`.

| Step | Array state          | i | Operation                                    | Result             |
| ---- | -------------------- | - | -------------------------------------------- | ------------------ |
| 0    | `[2,5,3,9,6]`        | – | initial                                      | valid              |
| 1    | `[2,5,3,9,6,1]`      | 5 | append at index 5                            | shape OK, order ? |
| 2    | `[2,5,1,9,6,3]`      | 2 | parent(5)=2 holds `3`; `1 < 3` so swap       | continue           |
| 3    | `[1,5,2,9,6,3]`      | 0 | parent(2)=0 holds `2`; `1 < 2` so swap       | continue           |
| 4    | `[1,5,2,9,6,3]`      | 0 | `i == 0`, root reached                       | valid              |

```text
 [2,5,3,9,6] + 1               step 2                       step 3 (final)

       2                          2                             1
      / \                        / \                           / \
     5   3                      5   1  <- swapped              5   2  <- swapped
    / \  /                     / \  /                         / \  /
   9  6 1  <- appended        9  6 3                         9  6 3
```

Cost: at most `floor(log2 n)` swaps. For uniformly random insertion order the *average* is O(1)
(about 2.6 comparisons is the well-known figure), because most elements settle near the bottom.

### 5.3 `pop()`: swap root with last, shrink, sift-down

```text
Algorithm
  1. top = a[0]
  2. Move the LAST element to the root: a[0] = a[n-1]; n--        (keeps SHAPE valid)
  3. Sift a[0] down: while it has a smaller child that is < it, swap with the smaller child.
  4. Return top
```

Why move the *last* element? Removing the root leaves a hole at index 0. The only removal that keeps
the shape complete is removing the last slot, so we relocate that value to fill the root and let
sift-down fix order.

Dry run: heap `[1, 5, 2, 9, 6, 3]`, `pop()`.

| Step | Array                | Operation                                          | Result       |
| ---- | -------------------- | -------------------------------------------------- | ------------ |
| 0    | `[1,5,2,9,6,3]`      | top = 1                                            | saved        |
| 1    | `[3,5,2,9,6]`        | move last (`3`) to root, shrink                    | order broken |
| 2    | `[2,5,3,9,6]`        | children of 0 are `5`,`2`; smaller is `2 < 3`: swap | continue     |
| 3    | `[2,5,3,9,6]`        | children of 2 would be idx 5,6: none               | done, returns 1 |

```text
   [1,5,2,9,6,3]           after moving 3 to root         after sift-down
         1                         3                            2
        / \                       / \                          / \
       5   2        ==>          5   2          ==>           5   3
      / \  /                    / \                          / \
     9  6 3                    9   6                        9   6
```

### 5.4 `heapify(array)`: Floyd's bottom-up build, O(n)

The naive way is `n` pushes: O(n log n). Floyd's trick: **sift-down every internal node, starting
from the last one and walking backwards to index 0.**

```text
for i = floor(n/2) - 1  down to  0:
      sift_down(i)
```

Why backwards? When we process node `i`, both of its subtrees are already valid heaps (they were
processed earlier because their indices are larger). So `sift_down(i)` is exactly the "one bad root
over two valid heaps" situation it is designed for.

Dry run: `[9, 4, 7, 1, 6, 3, 8]` (n = 7, internal nodes are indices 2, 1, 0).

| Step | i | Array before          | Action                                                 | Array after           |
| ---- | - | --------------------- | ------------------------------------------------------ | --------------------- |
| 1    | 2 | `[9,4,7,1,6,3,8]`     | children `3`,`8`; smaller `3 < 7` so swap              | `[9,4,3,1,6,7,8]`     |
| 2    | 1 | `[9,4,3,1,6,7,8]`     | children `1`,`6`; smaller `1 < 4`: swap; then leaf     | `[9,1,3,4,6,7,8]`     |
| 3    | 0 | `[9,1,3,4,6,7,8]`     | children `1`,`3`; swap with `1`; then children `4`,`6`; swap with `4` | `[1,4,3,9,6,7,8]` |

```text
 initial                       after i=2                after i=1                after i=0 (done)
      9                            9                        9                         1
     / \                          / \                      / \                       / \
    4   7                        4   3                    1   3                     4   3
   / \ / \                      / \ / \                  / \ / \                   / \ / \
  1  6 3  8                    1  6 7  8                4  6 7  8                 9  6 7  8
```

(This exact result, `[1 4 3 9 6 7 8]`, is asserted in the Go, Rust, and C tests.)

### 5.4.1 Heapify vs. repeated push: which is which

```text
 n pushes:  sift-UP each new element.   Worst case = sum of depths of all nodes    = Theta(n log n)
 Floyd:     sift-DOWN each internal node. Worst case = sum of HEIGHTS of all nodes = Theta(n)
```

The asymmetry: **most nodes are near the bottom**. Depth is large for most nodes (bad for sift-up),
while height is small for most nodes (good for sift-down).

### 5.5 `replace(x)` / `pushpop(x)`: fused operations

* `replace(x)`: pop the top, then push `x`, but implemented as *overwrite root with x, sift-down*. One O(log n) pass instead of two. Requires a non-empty heap. Returns the old top.
* `pushpop(x)`: push `x` then pop. If `x <= top` the answer is just `x` and the heap is untouched (O(1)); else it's `replace(x)`.

These are the right primitive for **streaming top-k** (Section 13.1).

### 5.6 `heapsort` (in place, O(n log n), O(1) extra space)

Use a **max-heap** in the array itself.

```text
 Phase 1: heapify the whole array as a MAX-heap.                     [ heap region .............. ]
 Phase 2: repeat: swap a[0] with a[end]; shrink the heap region;
          sift-down a[0].                                             [ heap region ...... | sorted ]

 e.g.   [ 9 6 7 1 4 3 | 8 ]      heap region shrinks, sorted suffix grows
```

Each phase-2 iteration puts the current maximum into its final position at the end.
Heapsort is not stable, and it is less cache-friendly than quicksort, but it has a guaranteed
O(n log n) worst case with O(1) extra memory.

### 5.7 `merge` (meld)

Binary heaps do **not** merge efficiently. Concatenate arrays and heapify: O(n + m). Variants
designed for melding (binomial, Fibonacci, pairing, leftist, skew) are in Section 12.

---

## 6. Complexity analysis

### 6.1 Why `push`/`pop` are O(log n)

A complete tree with `n` nodes has height `h = floor(log2 n)`. Each sift moves along one
root-to-leaf path, doing O(1) work per level, so O(h) = O(log n).

Concrete counts for `n = 1,000,000`: `h = 19`. A pop performs at most 19 levels x 2 comparisons
(child vs child, winner vs x) ≈ 38 comparisons. A balanced BST with rotations does comparable
comparison counts but with more pointer traffic.

Comparison-saving trick (used by Rust's `BinaryHeap` and Python's `heapq`): **bottom-up sift-down.**
The element moved to the root came from the *bottom* level, so it will almost certainly sink nearly
to the bottom. So: walk down always choosing the smaller child *without comparing to x* (1
comparison per level), reach the bottom, then sift `x` *up* a short way. This saves roughly half the
comparisons on average. Section 8/10 implementations use the simple textbook form for clarity.

### 6.2 Proof that heapify is O(n)

Let a node's **height** be the longest path length down to a leaf. In a heap of `n` nodes there are
at most `ceil(n / 2^(h+1))` nodes of height `h`. Sifting down a height-`h` node costs at most `h` swaps.

```text
 Level (height)   Nodes      Max swaps per node    Total swaps
 ---------------  ---------  --------------------  -------------------
 height 0 (leaf)  ~ n/2      0                     0   (leaves are skipped entirely)
 height 1         ~ n/4      1                     n/4
 height 2         ~ n/8      2                     2n/8
 height 3         ~ n/16     3                     3n/16
 ...              ...        ...                   ...
 height h         ~ n/2^(h+1) h                    h * n / 2^(h+1)
```

Total work:

```text
  T(n) <= sum_{h=0}^{log n}  ceil(n / 2^(h+1)) * h
        <= n * sum_{h=0}^{inf} h / 2^(h+1)
         = n * (1/2) * sum_{h>=0} h / 2^h
         = n * (1/2) * 2            (since sum_{h>=0} h x^h = x/(1-x)^2, at x = 1/2 equals 2)
         = n
```

So heapify is Θ(n). **Combined with heapsort:** heapify O(n) + n pops O(n log n) = O(n log n).

### 6.3 Lower bound: you cannot beat log n for both

If `push` and `pop-min` were both o(log n), then pushing n items and popping n times would sort in
o(n log n), which contradicts the Ω(n log n) comparison-sorting lower bound. So in the comparison
model some operation must cost Ω(log n) (amortized). Fibonacci heaps get O(1) amortized push and
decrease-key by paying O(log n) amortized for pop.

### 6.4 Complexity table across implementations

| Structure             | push        | peek | pop-min      | decrease-key      | merge        | build from n |
| --------------------- | ----------- | ---- | ------------ | ----------------- | ------------ | ------------ |
| Unsorted array        | O(1)        | O(n) | O(n)         | O(1) (with index) | O(1)/O(n)    | O(n)         |
| Sorted array          | O(n)        | O(1) | O(1) or O(n) | O(n)              | O(n)         | O(n log n)   |
| **Binary heap**       | O(log n)*   | O(1) | O(log n)     | O(log n)†         | O(n)         | **O(n)**     |
| d-ary heap            | O(log_d n)  | O(1) | O(d log_d n) | O(log_d n)†       | O(n)         | O(n)         |
| Binomial heap         | O(1) amort. | O(1)‡| O(log n)     | O(log n)          | O(log n)     | O(n)         |
| Fibonacci heap        | O(1)        | O(1) | O(log n) am. | O(1) amortized    | O(1)         | O(n)         |
| Pairing heap          | O(1)        | O(1) | O(log n) am. | see note§         | O(1)         | O(n)         |
| Balanced BST          | O(log n)    | O(log n) | O(log n) | O(log n)          | O(n)         | O(n log n)   |

`*` average O(1) for random input, worst case O(log n).
`†` requires an id→position index (Section 11.3); otherwise you cannot even *find* the element in O(1).
`‡` with a maintained min pointer.
`§` the precise decrease-key bound for pairing heaps is not fully settled; known bounds sit between Ω(log log n) and roughly O(2^(2·sqrt(log log n))) amortized.

### 6.5 Space

* Binary heap: **O(n) total, O(1) auxiliary** (heapsort/in-place heapify use no extra memory).
* Indexed heap: adds an O(N) position table for the ID universe of size `N`.
* Lazy-deletion Dijkstra: heap may hold up to O(E) stale entries.

### 6.6 Dijkstra's complexity is a heap choice

```text
 Heap used             Dijkstra time
 --------------------  --------------------------------
 unsorted array        O(V^2)          (best for very dense graphs)
 binary heap           O((V + E) log V)   <- what you use in practice
 Fibonacci heap        O(E + V log V)     <- theoretical best, poor constants in practice
```

---

## 7. Language mechanics

| Aspect                | Go                                                     | C                                                          | Rust                                                                 |
| --------------------- | ------------------------------------------------------ | ---------------------------------------------------------- | -------------------------------------------------------------------- |
| Std-lib PQ            | `container/heap` (interface based, min-heap by `Less`) | none (POSIX has none; write your own)                      | `std::collections::BinaryHeap` (**max**-heap)                        |
| Generic elements      | Generics (1.18+) or `interface{}`/`any`                | `void*` + element size + comparator function pointer       | Generics with `Ord` (trait bound)                                    |
| Min-heap from std     | Change `Less` to `<`                                   | Comparator returns `<0` when `a` should be above `b`       | Wrap in `std::cmp::Reverse`                                          |
| "Absent" result       | `(T, bool)` or zero value                              | return code / `NULL`                                       | `Option<T>` / `Option<&T>`                                           |
| Memory                | GC; popped slots must be zeroed to release pointers    | manual `malloc`/`realloc`/`free`                           | `Vec` RAII; no leaks, no dangling                                    |
| Moving elements       | plain assignment                                       | `memcpy` (element size)                                    | `swap` (safe) or `unsafe` Hole (std)                                 |
| Float keys            | works, but `NaN` breaks ordering                       | comparator must handle `NaN` yourself                      | `f64` is not `Ord`; wrap with `total_cmp`                            |
| Mutating a key        | you must call `heap.Fix`                               | you must re-sift yourself                                  | not allowed while inside (ownership); use `peek_mut` or remove/re-add |
| Concurrency           | protect with `sync.Mutex`                              | protect with mutex/atomics yourself                        | `Mutex<BinaryHeap<T>>`                                               |

### Go: the `container/heap` interface is confusing. Here is the truth.

```text
 You implement (sort.Interface + 2):          The package's functions drive the algorithm:
   Len(), Less(i,j), Swap(i,j)                 heap.Init(h)      heapify           O(n)
   Push(x any)   <-- raw append               heap.Push(h, x)   calls h.Push(x), then sift-up
   Pop() any     <-- raw remove-last          heap.Pop(h)       swaps 0 and n-1, sift-down,
                                                                 then calls h.Pop() to remove last
                                              heap.Fix(h, i)    re-sift after key change
                                              heap.Remove(h, i) remove element at index i
```

Rule: **call `heap.Push/Pop`, never `h.Push/Pop` directly.** Your `Push`/`Pop` methods only
append/truncate the slice; the package does the sifting.

### Rust: `BinaryHeap` facts

* Max-heap. `BinaryHeap::from(vec)` heapifies in O(n).
* `push` O(1) average, O(log n) worst; `pop` O(log n); `peek` O(1).
* `iter()` yields **arbitrary (internal) order**. Use `into_sorted_vec()` for ascending order, which consumes the heap.
* `peek_mut()` returns a guard; mutating through it sifts down when the guard is dropped. `PeekMut::pop(guard)` pops.
* Ordering must stay consistent while items are in the heap. Changing a key via interior mutability is a logic error (safe, but the heap's behaviour becomes unspecified).

---

## 8. Implementation: Go

Files live in one `package main` directory (`go mod init pqdemo`). Tested on Go 1.22 (generics need 1.18+; `slices` in tests needs 1.21+).

### 8.1 A generic binary heap from scratch (hole-based sift, less-func)

```go
// file: go/heap.go
package main

// Heap is a binary heap ordered by less: less(a, b) == true means a has HIGHER
// priority than b (a should be closer to the root).
//   min-heap on ints: func(a, b int) bool { return a < b }
//   max-heap on ints: func(a, b int) bool { return a > b }
type Heap[T any] struct {
	data []T
	less func(a, b T) bool
}

// New creates an empty heap.
func New[T any](less func(a, b T) bool) *Heap[T] {
	return &Heap[T]{less: less}
}

// FromSlice heapifies items IN PLACE in O(n). The heap takes ownership of the
// slice: do not keep using items afterwards.
func FromSlice[T any](items []T, less func(a, b T) bool) *Heap[T] {
	h := &Heap[T]{data: items, less: less}
	for i := len(h.data)/2 - 1; i >= 0; i-- { // last internal node down to root
		h.siftDown(i)
	}
	return h
}

func (h *Heap[T]) Len() int { return len(h.data) }

// Peek returns the top element without removing it.
func (h *Heap[T]) Peek() (T, bool) {
	var zero T
	if len(h.data) == 0 {
		return zero, false
	}
	return h.data[0], true
}

// Push inserts x: append at the end (keeps the tree complete), then sift up.
func (h *Heap[T]) Push(x T) {
	h.data = append(h.data, x)
	h.siftUp(len(h.data) - 1)
}

// Pop removes and returns the top element.
func (h *Heap[T]) Pop() (T, bool) {
	var zero T
	n := len(h.data)
	if n == 0 {
		return zero, false
	}
	top := h.data[0]
	last := n - 1
	h.data[0] = h.data[last]
	h.data[last] = zero // release references so the GC can reclaim pointees
	h.data = h.data[:last]
	if last > 0 {
		h.siftDown(0)
	}
	return top, true
}

// Replace overwrites the top with x and returns the old top in ONE sift-down.
// It panics on an empty heap (check Len first).
func (h *Heap[T]) Replace(x T) T {
	top := h.data[0]
	h.data[0] = x
	h.siftDown(0)
	return top
}

// siftUp: "hole" technique. Instead of swapping at every level (3 writes per
// level), remember x, shift parents down into the hole (1 write per level),
// and write x once at the end.
func (h *Heap[T]) siftUp(i int) {
	x := h.data[i]
	for i > 0 {
		p := (i - 1) / 2
		if !h.less(x, h.data[p]) {
			break
		}
		h.data[i] = h.data[p]
		i = p
	}
	h.data[i] = x
}

func (h *Heap[T]) siftDown(i int) {
	n := len(h.data)
	x := h.data[i]
	for {
		l := 2*i + 1
		if l >= n {
			break // i is a leaf
		}
		c := l // c = index of the higher-priority child
		if r := l + 1; r < n && h.less(h.data[r], h.data[l]) {
			c = r
		}
		if !h.less(h.data[c], x) {
			break // x is at least as good as both children
		}
		h.data[i] = h.data[c]
		i = c
	}
	h.data[i] = x
}
```

Line-by-line notes:

* `siftUp` uses `!less(x, parent)` to stop. That means **equal keys do not swap** (fewer moves, but not stable).
* In `siftDown`, `r < n` guards a missing right child (a node can have only a left child at the bottom level).
* `h.data[last] = zero` matters for `Heap[*BigStruct]` or `Heap[string]`: otherwise the truncated tail slot still references the popped value, preventing collection.
* `FromSlice` aliases the caller's slice. This is the same contract as `heap.Init(h)` operating on `h`.

### 8.2 Using the standard library: `container/heap`, `Fix`, `Remove`

```go
// file: go/containerheap.go
package main

import "container/heap"

// ---- Canonical example: min-heap of ints ----
type IntHeap []int

func (h IntHeap) Len() int           { return len(h) }
func (h IntHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h IntHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *IntHeap) Push(x any)        { *h = append(*h, x.(int)) } // raw append; heap.Push sifts
func (h *IntHeap) Pop() any { // raw "remove last"; heap.Pop moved the top there first
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// ---- Updatable queue: each Task remembers its own index ----
type Task struct {
	Name  string
	Prio  int
	index int // maintained by Swap/Push/Pop; needed for Fix and Remove
}

type TaskQueue []*Task

func (q TaskQueue) Len() int           { return len(q) }
func (q TaskQueue) Less(i, j int) bool { return q[i].Prio < q[j].Prio }
func (q TaskQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}
func (q *TaskQueue) Push(x any) {
	t := x.(*Task)
	t.index = len(*q)
	*q = append(*q, t)
}
func (q *TaskQueue) Pop() any {
	old := *q
	n := len(old)
	t := old[n-1]
	old[n-1] = nil // avoid retaining the pointer
	t.index = -1   // marks "no longer in the heap"
	*q = old[:n-1]
	return t
}

// demoTaskQueue shows update (Fix) and cancellation (Remove). Pop order: c, a.
func demoTaskQueue() []string {
	q := &TaskQueue{}
	heap.Init(q)
	a := &Task{Name: "a", Prio: 5}
	b := &Task{Name: "b", Prio: 3}
	c := &Task{Name: "c", Prio: 8}
	for _, t := range []*Task{a, b, c} {
		heap.Push(q, t)
	}
	c.Prio = 1               // priority changed while inside the heap ...
	heap.Fix(q, c.index)     // ... so we MUST repair the heap: sift-up here
	heap.Remove(q, b.index)  // cancel task b in O(log n)
	var order []string
	for q.Len() > 0 {
		order = append(order, heap.Pop(q).(*Task).Name)
	}
	return order
}
```

Go-specific implications:

* `[]*Task` heaps chase pointers (each `*Task` is a separate allocation, see Section 4.1). For hot paths, a `[]struct{...}` heap like 8.1 is markedly friendlier to the cache, and `heap.Interface` boxes values into `any`, which may allocate.
* `heap.Fix` does `down` then `up`: only one of them will move anything.

### 8.3 Dijkstra (lazy deletion), scheduler, top-k, k-way merge, running median, heapsort

```go
// file: go/algos.go
package main

import "math"

// ---------- Dijkstra with lazy deletion ----------
type Edge struct{ to, w int }
type distNode struct{ node, dist int }

// Dijkstra returns shortest distances from src; unreachable nodes stay math.MaxInt.
// Requires non-negative weights.
func Dijkstra(g [][]Edge, src int) []int {
	dist := make([]int, len(g))
	for i := range dist {
		dist[i] = math.MaxInt
	}
	dist[src] = 0
	h := New(func(a, b distNode) bool { return a.dist < b.dist })
	h.Push(distNode{src, 0})
	for h.Len() > 0 {
		cur, _ := h.Pop()
		if cur.dist > dist[cur.node] {
			continue // stale entry: a shorter path was already found
		}
		for _, e := range g[cur.node] {
			if nd := cur.dist + e.w; nd < dist[e.to] {
				dist[e.to] = nd
				h.Push(distNode{e.to, nd}) // no decrease-key: push a duplicate
			}
		}
	}
	return dist
}

// ---------- Event scheduler with FIFO tie-breaking (stability via sequence number) ----------
type job struct {
	at   int64
	seq  uint64
	name string
}

type Scheduler struct {
	h   *Heap[job]
	seq uint64
}

func NewScheduler() *Scheduler {
	return &Scheduler{h: New(func(a, b job) bool {
		if a.at != b.at {
			return a.at < b.at
		}
		return a.seq < b.seq // equal times run in insertion order
	})}
}

func (s *Scheduler) Schedule(at int64, name string) {
	s.seq++
	s.h.Push(job{at, s.seq, name})
}

// PopDue returns the names of all jobs with at <= now, in order.
func (s *Scheduler) PopDue(now int64) []string {
	var out []string
	for {
		j, ok := s.h.Peek()
		if !ok || j.at > now {
			return out
		}
		s.h.Pop()
		out = append(out, j.name)
	}
}

// ---------- Top-k largest, streaming, O(n log k) time, O(k) space ----------
func TopK(nums []int, k int) []int {
	if k <= 0 {
		return nil
	}
	h := New(func(a, b int) bool { return a < b }) // MIN-heap: root is the k-th largest so far
	for _, x := range nums {
		if h.Len() < k {
			h.Push(x)
		} else if top, _ := h.Peek(); x > top {
			h.Replace(x)
		}
	}
	out := make([]int, 0, h.Len())
	for h.Len() > 0 {
		v, _ := h.Pop()
		out = append(out, v) // ascending
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out // descending
}

// ---------- K-way merge of sorted slices, O(N log k) ----------
type cursor struct{ val, list, idx int }

func MergeK(lists [][]int) []int {
	h := New(func(a, b cursor) bool { return a.val < b.val })
	for i, l := range lists {
		if len(l) > 0 {
			h.Push(cursor{l[0], i, 0})
		}
	}
	var out []int
	for h.Len() > 0 {
		c, _ := h.Pop()
		out = append(out, c.val)
		if n := c.idx + 1; n < len(lists[c.list]) {
			h.Push(cursor{lists[c.list][n], c.list, n})
		}
	}
	return out
}

// ---------- Running median: two heaps ----------
// Invariants: every element of lo <= every element of hi, and
//             len(lo) == len(hi) or len(lo) == len(hi)+1.
type MedianFinder struct{ lo, hi *Heap[int] } // lo: max-heap, hi: min-heap

func NewMedianFinder() *MedianFinder {
	return &MedianFinder{
		lo: New(func(a, b int) bool { return a > b }),
		hi: New(func(a, b int) bool { return a < b }),
	}
}

func (m *MedianFinder) Add(x int) {
	if top, ok := m.lo.Peek(); !ok || x <= top {
		m.lo.Push(x)
	} else {
		m.hi.Push(x)
	}
	if m.lo.Len() > m.hi.Len()+1 {
		v, _ := m.lo.Pop()
		m.hi.Push(v)
	} else if m.hi.Len() > m.lo.Len() {
		v, _ := m.hi.Pop()
		m.lo.Push(v)
	}
}

// Median must not be called on an empty finder.
func (m *MedianFinder) Median() float64 {
	a, _ := m.lo.Peek()
	if m.lo.Len() > m.hi.Len() {
		return float64(a)
	}
	b, _ := m.hi.Peek()
	return (float64(a) + float64(b)) / 2
}

// ---------- In-place heapsort (max-heap) ----------
func HeapSort(a []int) {
	n := len(a)
	for i := n/2 - 1; i >= 0; i-- {
		siftDownMax(a, i, n)
	}
	for end := n - 1; end > 0; end-- {
		a[0], a[end] = a[end], a[0]
		siftDownMax(a, 0, end) // heap region is now a[:end]
	}
}

func siftDownMax(a []int, i, n int) {
	for {
		l := 2*i + 1
		if l >= n {
			return
		}
		c := l
		if r := l + 1; r < n && a[r] > a[l] {
			c = r
		}
		if a[c] <= a[i] {
			return
		}
		a[i], a[c] = a[c], a[i]
		i = c
	}
}
```

### 8.4 `main.go`, tests, and a benchmark

```go
// file: go/main.go
package main

import "fmt"

func main() {
	g := [][]Edge{{{1, 4}, {2, 1}}, {{3, 1}}, {{1, 2}, {3, 5}}, {}, {}}
	fmt.Println("dist:", Dijkstra(g, 0))
	fmt.Println("top3:", TopK([]int{5, 1, 9, 3, 7, 8}, 3))
}
```

```go
// file: go/heap_test.go
package main

import (
	"container/heap"
	"math"
	"math/rand"
	"slices"
	"sort"
	"testing"
)

func minInt(a, b int) bool { return a < b }

func TestPopsSorted(t *testing.T) {
	h := New(minInt)
	in := []int{5, 3, 8, 1, 9, 2, 7}
	for _, x := range in {
		h.Push(x)
	}
	var got []int
	for h.Len() > 0 {
		v, _ := h.Pop()
		got = append(got, v)
	}
	if len(got) != len(in) || !sort.IntsAreSorted(got) {
		t.Fatalf("got %v", got)
	}
	if _, ok := h.Pop(); ok {
		t.Fatal("pop on empty must report false")
	}
}

func TestHeapifyMatchesDryRun(t *testing.T) {
	h := FromSlice([]int{9, 4, 7, 1, 6, 3, 8}, minInt)
	want := []int{1, 4, 3, 9, 6, 7, 8} // the dry run in Section 5.4
	if !slices.Equal(h.data, want) {
		t.Fatalf("got %v want %v", h.data, want)
	}
}

func TestPushDryRun(t *testing.T) {
	h := FromSlice([]int{2, 5, 3, 9, 6}, minInt)
	h.Push(1)
	if !slices.Equal(h.data, []int{1, 5, 2, 9, 6, 3}) {
		t.Fatalf("got %v", h.data)
	}
	h.Pop()
	if !slices.Equal(h.data, []int{2, 5, 3, 9, 6}) {
		t.Fatalf("got %v", h.data)
	}
}

func TestRandomAgainstSort(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for trial := 0; trial < 200; trial++ {
		n := r.Intn(100)
		in := make([]int, n)
		for i := range in {
			in[i] = r.Intn(50) // many duplicates on purpose
		}
		h := New(minInt)
		for _, x := range in {
			h.Push(x)
		}
		want := slices.Clone(in)
		slices.Sort(want)
		for _, w := range want {
			if v, _ := h.Pop(); v != w {
				t.Fatalf("mismatch: %d vs %d", v, w)
			}
		}
	}
}

func TestContainerHeapFixRemove(t *testing.T) {
	if got := demoTaskQueue(); !slices.Equal(got, []string{"c", "a"}) {
		t.Fatalf("got %v", got)
	}
	h := &IntHeap{5, 2, 8}
	heap.Init(h)
	heap.Push(h, 1)
	if v := heap.Pop(h).(int); v != 1 {
		t.Fatalf("got %d", v)
	}
}

func TestDijkstra(t *testing.T) {
	g := [][]Edge{{{1, 4}, {2, 1}}, {{3, 1}}, {{1, 2}, {3, 5}}, {}, {}}
	want := []int{0, 3, 1, 4, math.MaxInt}
	if got := Dijkstra(g, 0); !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestSchedulerFIFOTies(t *testing.T) {
	s := NewScheduler()
	s.Schedule(10, "b1")
	s.Schedule(5, "a")
	s.Schedule(10, "b2")
	s.Schedule(20, "late")
	if got := s.PopDue(10); !slices.Equal(got, []string{"a", "b1", "b2"}) {
		t.Fatalf("got %v", got)
	}
	if got := s.PopDue(100); !slices.Equal(got, []string{"late"}) {
		t.Fatalf("got %v", got)
	}
}

func TestTopKMergeMedianSort(t *testing.T) {
	if got := TopK([]int{5, 1, 9, 3, 7, 8}, 3); !slices.Equal(got, []int{9, 8, 7}) {
		t.Fatalf("topk %v", got)
	}
	if got := TopK([]int{1}, 0); got != nil {
		t.Fatalf("k=0 %v", got)
	}
	got := MergeK([][]int{{1, 4, 7}, {2, 5}, {}, {3, 6, 8, 9}})
	if !slices.Equal(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatalf("merge %v", got)
	}
	m := NewMedianFinder()
	var meds []float64
	for _, x := range []int{5, 15, 1, 3} {
		m.Add(x)
		meds = append(meds, m.Median())
	}
	if !slices.Equal(meds, []float64{5, 10, 5, 4}) {
		t.Fatalf("median %v", meds)
	}
	a := []int{9, 4, 7, 1, 6, 3, 8, 3}
	HeapSort(a)
	if !sort.IntsAreSorted(a) {
		t.Fatalf("heapsort %v", a)
	}
}

// Benchmarks: compare the generic inline-value heap with container/heap (interface + boxing).
// Numbers depend on your machine and Go version; measure, don't assume.
//   go test -bench . -benchmem -run XXX
func BenchmarkGenericHeap(b *testing.B) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < b.N; i++ {
		h := New(minInt)
		for j := 0; j < 1024; j++ {
			h.Push(r.Int())
		}
		for h.Len() > 0 {
			h.Pop()
		}
	}
}

func BenchmarkContainerHeap(b *testing.B) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < b.N; i++ {
		h := &IntHeap{}
		for j := 0; j < 1024; j++ {
			heap.Push(h, r.Int())
		}
		for h.Len() > 0 {
			heap.Pop(h)
		}
	}
}
```

Run: `go mod init pqdemo && go test ./... && go run .`

---

## 9. Implementation: C

C has no generics, so a reusable heap stores **raw bytes**: an element size, a growable byte buffer,
and a comparator function pointer. The comparator contract used below is:

```text
 cmp(a, b) < 0   =>  a has HIGHER priority than b (a belongs nearer the root)
 cmp(a, b) >= 0  =>  a does not outrank b
```

Two classic C pitfalls addressed in the code:

* **Never** write `return x - y;` in a comparator: it overflows for large/opposite-sign ints. Use `(x > y) - (x < y)`.
* `size_t` is unsigned. `(i - 1) / 2` at `i == 0` wraps to a huge value, so always guard `i > 0`; and loop downward with `for (i = n; i > 0; i--) use(i-1)` instead of `i >= 0`.

### 9.1 Generic byte-oriented heap

```c
// file: c/pq.c
#include <assert.h>
#include <limits.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef int (*pq_cmp)(const void *a, const void *b);

typedef struct {
    unsigned char *data; /* heap array, elem * cap bytes            */
    unsigned char *tmp;  /* one-element scratch used as the "hole"  */
    size_t elem, len, cap;
    pq_cmp cmp;
} PQ;

static inline void *at(const PQ *q, size_t i) { return q->data + i * q->elem; }

int pq_init(PQ *q, size_t elem, size_t cap, pq_cmp cmp) {
    if (cap == 0) cap = 8;
    q->data = malloc(elem * cap);
    q->tmp = malloc(elem);
    if (!q->data || !q->tmp) { free(q->data); free(q->tmp); return -1; }
    q->elem = elem; q->len = 0; q->cap = cap; q->cmp = cmp;
    return 0;
}

void pq_free(PQ *q) {
    free(q->data);
    free(q->tmp);
    memset(q, 0, sizeof *q);
}

static void sift_up(PQ *q, size_t i) {
    unsigned char *x = q->tmp;
    memcpy(x, at(q, i), q->elem);            /* lift x out, leaving a hole at i   */
    while (i > 0) {
        size_t p = (i - 1) / 2;
        if (q->cmp(x, at(q, p)) >= 0) break; /* x does not outrank its parent     */
        memcpy(at(q, i), at(q, p), q->elem); /* slide parent down into the hole   */
        i = p;
    }
    memcpy(at(q, i), x, q->elem);            /* drop x into its final position    */
}

static void sift_down(PQ *q, size_t i) {
    size_t n = q->len;
    unsigned char *x = q->tmp;
    memcpy(x, at(q, i), q->elem);
    for (;;) {
        size_t l = 2 * i + 1;
        if (l >= n) break;                   /* leaf */
        size_t c = l, r = l + 1;
        if (r < n && q->cmp(at(q, r), at(q, l)) < 0) c = r; /* higher-priority child */
        if (q->cmp(at(q, c), x) >= 0) break; /* child does not outrank x          */
        memcpy(at(q, i), at(q, c), q->elem);
        i = c;
    }
    memcpy(at(q, i), x, q->elem);
}

static int grow(PQ *q) {
    size_t ncap = q->cap * 2;
    void *p = realloc(q->data, ncap * q->elem);
    if (!p) return -1;                       /* old buffer is still valid on failure */
    q->data = p;
    q->cap = ncap;
    return 0;
}

int pq_push(PQ *q, const void *x) {
    if (q->len == q->cap && grow(q) != 0) return -1;
    memcpy(at(q, q->len), x, q->elem);
    q->len++;
    sift_up(q, q->len - 1);
    return 0;
}

/* Copies the top into *out (if non-NULL) and removes it. Returns -1 when empty. */
int pq_pop(PQ *q, void *out) {
    if (q->len == 0) return -1;
    if (out) memcpy(out, at(q, 0), q->elem);
    q->len--;
    if (q->len > 0) {
        memcpy(at(q, 0), at(q, q->len), q->elem); /* last element to the root */
        sift_down(q, 0);
    }
    return 0;
}

const void *pq_peek(const PQ *q) { return q->len ? at(q, 0) : NULL; }

/* Overwrite the top with x and restore order. Heap must be non-empty. */
void pq_replace_top(PQ *q, const void *x) {
    assert(q->len > 0);
    memcpy(at(q, 0), x, q->elem);
    sift_down(q, 0);
}

/* Bulk build: copy n elements in, then Floyd's bottom-up heapify in O(n). */
int pq_from_array(PQ *q, const void *arr, size_t n, size_t elem, pq_cmp cmp) {
    if (pq_init(q, elem, n ? n : 1, cmp) != 0) return -1;
    memcpy(q->data, arr, n * elem);
    q->len = n;
    for (size_t i = n / 2; i > 0; i--) sift_down(q, i - 1);
    return 0;
}

/* Comparators */
static int cmp_int_min(const void *a, const void *b) {
    int x = *(const int *)a, y = *(const int *)b;
    return (x > y) - (x < y);  /* smaller int = higher priority */
}
static int cmp_int_max(const void *a, const void *b) {
    int x = *(const int *)a, y = *(const int *)b;
    return (y > x) - (y < x);  /* larger int = higher priority */
}
```

Design notes:

* `tmp` is the scratch "hole" slot, allocated once, so each sift does **one** write per level, not a 3-write swap.
* `grow` writes to a temporary pointer first: `q->data = realloc(q->data, ...)` would leak the buffer on failure.
* `pq_pop(q, NULL)` discards the element. Elements containing owned pointers (e.g. `char *`) need your own destructor logic; the heap only copies bytes.
* Not thread-safe. Wrap with a mutex.

### 9.2 Indexed min-heap with decrease-key (what Dijkstra actually wants)

```c
// file: c/pq.c  (continued)
/* Heap of integer IDs in [0, cap), keyed by long. pos[id] = index inside heap[], or -1. */
typedef struct {
    int *heap;   /* heap[i] = id stored at heap position i */
    int *pos;    /* pos[id] = position of id, or -1 if absent */
    long *key;   /* key[id] = priority of id */
    int n;
} IPQ;

int ipq_init(IPQ *q, int cap) {
    q->heap = malloc((size_t)cap * sizeof(int));
    q->pos  = malloc((size_t)cap * sizeof(int));
    q->key  = malloc((size_t)cap * sizeof(long));
    q->n = 0;
    if (!q->heap || !q->pos || !q->key) { free(q->heap); free(q->pos); free(q->key); return -1; }
    for (int i = 0; i < cap; i++) q->pos[i] = -1;
    return 0;
}
void ipq_free(IPQ *q) { free(q->heap); free(q->pos); free(q->key); }

static void ipq_swap(IPQ *q, int i, int j) {
    int a = q->heap[i], b = q->heap[j];
    q->heap[i] = b; q->heap[j] = a;
    q->pos[b] = i;  q->pos[a] = j;          /* THE index-maintenance invariant */
}
static void ipq_up(IPQ *q, int i) {
    while (i > 0) {
        int p = (i - 1) / 2;
        if (q->key[q->heap[i]] < q->key[q->heap[p]]) { ipq_swap(q, i, p); i = p; }
        else break;
    }
}
static void ipq_down(IPQ *q, int i) {
    for (;;) {
        int l = 2 * i + 1;
        if (l >= q->n) break;
        int c = l, r = l + 1;
        if (r < q->n && q->key[q->heap[r]] < q->key[q->heap[l]]) c = r;
        if (q->key[q->heap[c]] < q->key[q->heap[i]]) { ipq_swap(q, i, c); i = c; }
        else break;
    }
}
int  ipq_contains(const IPQ *q, int id) { return q->pos[id] != -1; }
void ipq_push(IPQ *q, int id, long k) {
    q->key[id] = k;
    q->heap[q->n] = id;
    q->pos[id] = q->n;
    q->n++;
    ipq_up(q, q->n - 1);
}
void ipq_decrease(IPQ *q, int id, long k) { q->key[id] = k; ipq_up(q, q->pos[id]); }
/* General update: only one of up/down can move anything. */
void ipq_update(IPQ *q, int id, long k) {
    q->key[id] = k;
    ipq_up(q, q->pos[id]);
    ipq_down(q, q->pos[id]);
}
int ipq_pop(IPQ *q, long *k_out) {
    int id = q->heap[0];
    if (k_out) *k_out = q->key[id];
    q->n--;
    if (q->n > 0) {
        q->heap[0] = q->heap[q->n];
        q->pos[q->heap[0]] = 0;
        ipq_down(q, 0);
    }
    q->pos[id] = -1;
    return id;
}
```

### 9.3 Dijkstra with decrease-key, top-k, running median, heapsort

```c
// file: c/pq.c  (continued)
#define NV 5
#define NE 8
typedef struct { int to, next; long w; } Edge;
typedef struct { int head[NV]; Edge e[NE]; int m; } Graph; /* adjacency lists in arrays */

static void graph_init(Graph *g) {
    for (int i = 0; i < NV; i++) g->head[i] = -1;
    g->m = 0;
}
static void add_edge(Graph *g, int u, int v, long w) {
    g->e[g->m] = (Edge){v, g->head[u], w};
    g->head[u] = g->m++;
}

/* Non-negative weights: once popped, a node's distance is final. */
static void dijkstra(const Graph *g, int src, long dist[NV]) {
    IPQ q;
    assert(ipq_init(&q, NV) == 0);
    for (int i = 0; i < NV; i++) dist[i] = LONG_MAX;
    dist[src] = 0;
    ipq_push(&q, src, 0);
    while (q.n > 0) {
        long d;
        int u = ipq_pop(&q, &d);
        for (int i = g->head[u]; i != -1; i = g->e[i].next) {
            int v = g->e[i].to;
            long nd = d + g->e[i].w;
            if (nd < dist[v]) {
                dist[v] = nd;
                if (ipq_contains(&q, v)) ipq_decrease(&q, v, nd);
                else ipq_push(&q, v, nd);
            }
        }
    }
    ipq_free(&q);
}

/* Top-k largest: min-heap of size k. Writes descending values into out[0..k). */
static int top_k(const int *a, size_t n, size_t k, int *out) {
    if (k == 0) return 0;
    PQ q;
    if (pq_init(&q, sizeof(int), k + 1, cmp_int_min) != 0) return -1;
    for (size_t i = 0; i < n; i++) {
        if (q.len < k) pq_push(&q, &a[i]);
        else if (a[i] > *(const int *)pq_peek(&q)) pq_replace_top(&q, &a[i]);
    }
    size_t cnt = q.len;
    for (size_t i = cnt; i > 0; i--) pq_pop(&q, &out[i - 1]); /* smallest goes last */
    pq_free(&q);
    return (int)cnt;
}

/* Running median with two heaps. */
typedef struct { PQ lo, hi; } Median; /* lo: max-heap, hi: min-heap */

static int median_init(Median *m) {
    if (pq_init(&m->lo, sizeof(int), 8, cmp_int_max) != 0) return -1;
    if (pq_init(&m->hi, sizeof(int), 8, cmp_int_min) != 0) { pq_free(&m->lo); return -1; }
    return 0;
}
static void median_free(Median *m) { pq_free(&m->lo); pq_free(&m->hi); }
static void median_add(Median *m, int x) {
    const int *top = pq_peek(&m->lo);
    if (!top || x <= *top) pq_push(&m->lo, &x); else pq_push(&m->hi, &x);
    int v;
    if (m->lo.len > m->hi.len + 1)      { pq_pop(&m->lo, &v); pq_push(&m->hi, &v); }
    else if (m->hi.len > m->lo.len)     { pq_pop(&m->hi, &v); pq_push(&m->lo, &v); }
}
static double median_get(const Median *m) { /* non-empty only */
    const int *a = pq_peek(&m->lo);
    if (m->lo.len > m->hi.len) return *a;
    const int *b = pq_peek(&m->hi);
    return ((double)*a + (double)*b) / 2.0;
}

/* In-place heapsort on ints (max-heap). */
static void sift_down_max(int *a, size_t i, size_t n) {
    for (;;) {
        size_t l = 2 * i + 1;
        if (l >= n) return;
        size_t c = l;
        if (l + 1 < n && a[l + 1] > a[l]) c = l + 1;
        if (a[c] <= a[i]) return;
        int t = a[i]; a[i] = a[c]; a[c] = t;
        i = c;
    }
}
static void heap_sort(int *a, size_t n) {
    for (size_t i = n / 2; i > 0; i--) sift_down_max(a, i - 1, n);
    for (size_t end = n; end > 1; end--) {
        int t = a[0]; a[0] = a[end - 1]; a[end - 1] = t;
        sift_down_max(a, 0, end - 1);
    }
}
```

### 9.4 C tests

```c
// file: c/pq.c  (continued)
int main(void) {
    /* generic heap: pops sorted */
    int in[] = {5, 3, 8, 1, 9, 2, 7};
    PQ q;
    assert(pq_init(&q, sizeof(int), 2, cmp_int_min) == 0); /* tiny cap forces realloc */
    for (size_t i = 0; i < sizeof in / sizeof *in; i++) assert(pq_push(&q, &in[i]) == 0);
    int prev = INT_MIN, v;
    while (pq_pop(&q, &v) == 0) { assert(v >= prev); prev = v; }
    assert(pq_pop(&q, &v) == -1 && pq_peek(&q) == NULL);
    pq_free(&q);

    /* heapify matches the dry run of Section 5.4 */
    int arr[] = {9, 4, 7, 1, 6, 3, 8};
    assert(pq_from_array(&q, arr, 7, sizeof(int), cmp_int_min) == 0);
    int want[] = {1, 4, 3, 9, 6, 7, 8};
    assert(memcmp(q.data, want, sizeof want) == 0);
    pq_free(&q);

    /* Dijkstra with decrease-key */
    Graph g; graph_init(&g);
    add_edge(&g, 0, 1, 4); add_edge(&g, 0, 2, 1);
    add_edge(&g, 1, 3, 1); add_edge(&g, 2, 1, 2); add_edge(&g, 2, 3, 5);
    long dist[NV];
    dijkstra(&g, 0, dist);
    assert(dist[0] == 0 && dist[1] == 3 && dist[2] == 1 && dist[3] == 4 && dist[4] == LONG_MAX);

    /* top-k */
    int nums[] = {5, 1, 9, 3, 7, 8}, top[3];
    assert(top_k(nums, 6, 3, top) == 3 && top[0] == 9 && top[1] == 8 && top[2] == 7);

    /* running median: 5 -> 5, 15 -> 10, 1 -> 5, 3 -> 4 */
    Median m; assert(median_init(&m) == 0);
    int xs[] = {5, 15, 1, 3};
    double exp[] = {5, 10, 5, 4};
    for (int i = 0; i < 4; i++) { median_add(&m, xs[i]); assert(median_get(&m) == exp[i]); }
    median_free(&m);

    /* heapsort */
    int hs[] = {9, 4, 7, 1, 6, 3, 8, 3};
    heap_sort(hs, 8);
    for (int i = 1; i < 8; i++) assert(hs[i - 1] <= hs[i]);

    puts("C: all tests passed");
    return 0;
}
```

Build: `gcc -std=c11 -Wall -Wextra -g -fsanitize=address,undefined c/pq.c -o pq && ./pq`
(Because `assert` is compiled out with `-DNDEBUG`, never put required side effects only inside `assert`; here the tests build without `-DNDEBUG`. In production code, `dijkstra` should check `ipq_init` explicitly.)

---

## 10. Implementation: Rust

Tested on Rust 1.75 (stable). Everything is safe Rust.

### 10.1 Using `std::collections::BinaryHeap`

```rust
// file: rust/main.rs
use std::cmp::{Ordering, Reverse};
use std::collections::BinaryHeap;

// ---- Custom priority type: highest `prio` first, ties broken by smaller name first ----
#[derive(Debug, PartialEq, Eq)]
struct Job {
    prio: u32,
    name: String,
}
impl Ord for Job {
    fn cmp(&self, other: &Self) -> Ordering {
        self.prio
            .cmp(&other.prio)
            .then_with(|| other.name.cmp(&self.name)) // reversed: smaller name = greater
    }
}
impl PartialOrd for Job {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other)) // always delegate to Ord: keeps the two consistent
    }
}

// ---- f64 is only PartialOrd (NaN!). Wrap it with a total order. ----
#[derive(Debug, Clone, Copy)]
struct F(f64);
impl Ord for F {
    fn cmp(&self, other: &Self) -> Ordering {
        self.0.total_cmp(&other.0) // IEEE 754 totalOrder: NaN gets a fixed position
    }
}
impl PartialOrd for F {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}
impl PartialEq for F {
    fn eq(&self, other: &Self) -> bool {
        self.cmp(other) == Ordering::Equal
    }
}
impl Eq for F {}

fn std_heap_tour() {
    // BinaryHeap is a MAX-heap.
    let mut h = BinaryHeap::new();
    for x in [5, 1, 8, 3] {
        h.push(x);
    }
    assert_eq!(h.peek(), Some(&8));
    assert_eq!(h.pop(), Some(8));

    // MIN-heap: wrap with Reverse.
    let mut m = BinaryHeap::new();
    for x in [5, 1, 8, 3] {
        m.push(Reverse(x));
    }
    assert_eq!(m.pop(), Some(Reverse(1)));

    // Tuples compare lexicographically: (distance, node) pairs "just work".
    let mut t = BinaryHeap::new();
    t.push(Reverse((7u64, 2usize)));
    t.push(Reverse((3, 9)));
    t.push(Reverse((3, 4)));
    assert_eq!(t.pop(), Some(Reverse((3, 4))));

    // From<Vec<T>> heapifies in O(n); into_sorted_vec is ascending; iter() is arbitrary order.
    let h2 = BinaryHeap::from(vec![9, 4, 7, 1, 6, 3, 8]);
    assert_eq!(h2.into_sorted_vec(), vec![1, 3, 4, 6, 7, 8, 9]);

    // peek_mut: modify the top in place; the heap re-sifts when the guard drops.
    let mut h3 = BinaryHeap::from(vec![10, 7, 5]);
    if let Some(mut top) = h3.peek_mut() {
        *top = 1; // 10 -> 1, sifts down on drop
    }
    assert_eq!(h3.peek(), Some(&7));

    // Custom Ord
    let mut jobs = BinaryHeap::new();
    jobs.push(Job { prio: 2, name: "b".into() });
    jobs.push(Job { prio: 9, name: "z".into() });
    jobs.push(Job { prio: 9, name: "a".into() });
    assert_eq!(jobs.pop().unwrap().name, "a");

    // Floats
    let mut fh = BinaryHeap::new();
    for x in [2.5, -1.0, 9.75] {
        fh.push(F(x));
    }
    assert_eq!(fh.pop().unwrap().0, 9.75);

    // append merges two heaps (moves everything from `other`); cost is O(n + m) style
    let mut a = BinaryHeap::from(vec![1, 5]);
    let mut b = BinaryHeap::from(vec![3, 9]);
    a.append(&mut b);
    assert!(b.is_empty());
    assert_eq!(a.into_sorted_vec(), vec![1, 3, 5, 9]);
}
```

### 10.2 A generic binary heap from scratch (safe, swap-based)

```rust
// file: rust/main.rs  (continued)
/// less(a, b) == true means `a` has HIGHER priority than `b`.
pub struct Heap<T, F: Fn(&T, &T) -> bool> {
    data: Vec<T>,
    less: F,
}

impl<T, F: Fn(&T, &T) -> bool> Heap<T, F> {
    pub fn new(less: F) -> Self {
        Heap { data: Vec::new(), less }
    }

    /// Takes ownership of `v` and heapifies it in O(n).
    pub fn from_vec(v: Vec<T>, less: F) -> Self {
        let mut h = Heap { data: v, less };
        for i in (0..h.data.len() / 2).rev() {
            h.sift_down(i);
        }
        h
    }

    pub fn len(&self) -> usize { self.data.len() }
    pub fn is_empty(&self) -> bool { self.data.is_empty() }
    pub fn peek(&self) -> Option<&T> { self.data.first() }
    pub fn as_slice(&self) -> &[T] { &self.data }

    pub fn push(&mut self, x: T) {
        self.data.push(x);
        let last = self.data.len() - 1;
        self.sift_up(last);
    }

    pub fn pop(&mut self) -> Option<T> {
        if self.data.is_empty() {
            return None;
        }
        let last = self.data.len() - 1;
        self.data.swap(0, last);        // top goes to the end...
        let top = self.data.pop();      // ...and is removed; ownership moves to the caller
        if !self.data.is_empty() {
            self.sift_down(0);
        }
        top
    }

    fn sift_up(&mut self, mut i: usize) {
        while i > 0 {
            let p = (i - 1) / 2;
            if (self.less)(&self.data[i], &self.data[p]) {
                self.data.swap(i, p);
                i = p;
            } else {
                break;
            }
        }
    }

    fn sift_down(&mut self, mut i: usize) {
        let n = self.data.len();
        loop {
            let l = 2 * i + 1;
            if l >= n {
                break;
            }
            let mut c = l;
            let r = l + 1;
            if r < n && (self.less)(&self.data[r], &self.data[l]) {
                c = r;
            }
            if (self.less)(&self.data[c], &self.data[i]) {
                self.data.swap(i, c);
                i = c;
            } else {
                break;
            }
        }
    }
}
```

Rust-specific implications:

* `pop` returns `Option<T>` and **moves** the element out: there is no aliasing and nothing can dangle. In C you copy bytes; in Go you copy a value (or pointer).
* `(self.less)(&a, &b)`: the extra parentheses are needed to call a *field* that holds a closure (otherwise Rust parses it as a method call).
* `self.data.swap` needs `&mut self.data` while `self.less` is borrowed *immutably* only inside the `if` condition, and that borrow ends before the swap. Disjoint field borrows plus NLL make this compile.
* This version does 3 writes per level (swap). `std`'s `BinaryHeap` uses an `unsafe` `Hole` (like the C/Go "hole" versions) to do 1 write per level. Start safe, optimize when measured.

### 10.3 Indexed heap with decrease-key (safe Rust)

```rust
// file: rust/main.rs  (continued)
/// Min-heap of IDs in [0, cap), keyed by u64, with O(log n) decrease-key/update.
pub struct IndexedMinHeap {
    heap: Vec<usize>,        // heap[i] = id at position i
    pos: Vec<Option<usize>>, // pos[id] = Some(position) or None if absent
    key: Vec<u64>,           // key[id]
}

impl IndexedMinHeap {
    pub fn new(cap: usize) -> Self {
        IndexedMinHeap { heap: Vec::new(), pos: vec![None; cap], key: vec![0; cap] }
    }
    pub fn is_empty(&self) -> bool { self.heap.is_empty() }
    pub fn contains(&self, id: usize) -> bool { self.pos[id].is_some() }

    fn swap(&mut self, i: usize, j: usize) {
        self.heap.swap(i, j);
        self.pos[self.heap[i]] = Some(i); // index-maintenance invariant
        self.pos[self.heap[j]] = Some(j);
    }
    fn up(&mut self, mut i: usize) {
        while i > 0 {
            let p = (i - 1) / 2;
            if self.key[self.heap[i]] < self.key[self.heap[p]] {
                self.swap(i, p);
                i = p;
            } else {
                break;
            }
        }
    }
    fn down(&mut self, mut i: usize) {
        let n = self.heap.len();
        loop {
            let l = 2 * i + 1;
            if l >= n { break; }
            let mut c = l;
            if l + 1 < n && self.key[self.heap[l + 1]] < self.key[self.heap[l]] {
                c = l + 1;
            }
            if self.key[self.heap[c]] < self.key[self.heap[i]] {
                self.swap(i, c);
                i = c;
            } else {
                break;
            }
        }
    }

    /// Insert `id` with key `k`, or lower its key if it is already present.
    pub fn push_or_decrease(&mut self, id: usize, k: u64) {
        match self.pos[id] {
            Some(i) => {
                if k < self.key[id] {
                    self.key[id] = k;
                    self.up(i);
                }
            }
            None => {
                self.key[id] = k;
                self.heap.push(id);
                let i = self.heap.len() - 1;
                self.pos[id] = Some(i);
                self.up(i);
            }
        }
    }

    pub fn pop(&mut self) -> Option<(usize, u64)> {
        if self.heap.is_empty() {
            return None;
        }
        let last = self.heap.len() - 1;
        self.swap(0, last);
        let id = self.heap.pop().unwrap();
        self.pos[id] = None;
        if !self.heap.is_empty() {
            self.down(0);
        }
        Some((id, self.key[id]))
    }
}
```

### 10.4 Algorithms: Dijkstra (both styles), top-k, k-way merge, median, scheduler, heapsort

```rust
// file: rust/main.rs  (continued)
type Graph = Vec<Vec<(usize, u64)>>; // adjacency list: (neighbor, weight)

/// Lazy-deletion Dijkstra using std's max-heap wrapped in Reverse.
fn dijkstra_lazy(g: &Graph, src: usize) -> Vec<u64> {
    let mut dist = vec![u64::MAX; g.len()];
    dist[src] = 0;
    let mut h = BinaryHeap::new();
    h.push(Reverse((0u64, src)));
    while let Some(Reverse((d, u))) = h.pop() {
        if d > dist[u] {
            continue; // stale
        }
        for &(v, w) in &g[u] {
            let nd = d + w;
            if nd < dist[v] {
                dist[v] = nd;
                h.push(Reverse((nd, v)));
            }
        }
    }
    dist
}

/// Same answer with a true decrease-key heap: at most V entries, no duplicates.
fn dijkstra_indexed(g: &Graph, src: usize) -> Vec<u64> {
    let mut dist = vec![u64::MAX; g.len()];
    dist[src] = 0;
    let mut h = IndexedMinHeap::new(g.len());
    h.push_or_decrease(src, 0);
    while let Some((u, d)) = h.pop() {
        for &(v, w) in &g[u] {
            let nd = d + w;
            if nd < dist[v] {
                dist[v] = nd;
                h.push_or_decrease(v, nd);
            }
        }
    }
    dist
}

/// Top-k largest of a stream: min-heap of size k, O(n log k).
fn top_k(nums: &[i32], k: usize) -> Vec<i32> {
    if k == 0 {
        return vec![];
    }
    let mut h = BinaryHeap::with_capacity(k + 1);
    for &x in nums {
        if h.len() < k {
            h.push(Reverse(x));
        } else if let Some(mut top) = h.peek_mut() {
            let Reverse(smallest) = *top;
            if x > smallest {
                *top = Reverse(x); // replace: one sift-down when `top` drops
            }
        }
    }
    let mut v: Vec<i32> = h.into_iter().map(|Reverse(x)| x).collect();
    v.sort_unstable_by(|a, b| b.cmp(a));
    v
}

/// K-way merge of sorted lists, O(N log k).
fn merge_k(lists: &[Vec<i32>]) -> Vec<i32> {
    let mut h = BinaryHeap::new();
    for (i, l) in lists.iter().enumerate() {
        if let Some(&v) = l.first() {
            h.push(Reverse((v, i, 0usize)));
        }
    }
    let mut out = Vec::new();
    while let Some(Reverse((v, i, j))) = h.pop() {
        out.push(v);
        if let Some(&nx) = lists[i].get(j + 1) {
            h.push(Reverse((nx, i, j + 1)));
        }
    }
    out
}

/// Running median: lo = max-heap (lower half), hi = min-heap (upper half).
struct MedianFinder {
    lo: BinaryHeap<i32>,
    hi: BinaryHeap<Reverse<i32>>,
}
impl MedianFinder {
    fn new() -> Self {
        MedianFinder { lo: BinaryHeap::new(), hi: BinaryHeap::new() }
    }
    fn add(&mut self, x: i32) {
        match self.lo.peek() {
            Some(&top) if x > top => self.hi.push(Reverse(x)),
            _ => self.lo.push(x),
        }
        if self.lo.len() > self.hi.len() + 1 {
            let v = self.lo.pop().unwrap();
            self.hi.push(Reverse(v));
        } else if self.hi.len() > self.lo.len() {
            let Reverse(v) = self.hi.pop().unwrap();
            self.lo.push(v);
        }
    }
    fn median(&self) -> Option<f64> {
        let &a = self.lo.peek()?;
        if self.lo.len() > self.hi.len() {
            Some(a as f64)
        } else {
            let Reverse(b) = *self.hi.peek()?;
            Some((a as f64 + b as f64) / 2.0)
        }
    }
}

/// Event queue with FIFO tie-breaking via a monotonically increasing sequence number.
struct EventQueue {
    h: BinaryHeap<Reverse<(u64, u64, String)>>, // (time, seq, name)
    seq: u64,
}
impl EventQueue {
    fn new() -> Self { EventQueue { h: BinaryHeap::new(), seq: 0 } }
    fn schedule(&mut self, at: u64, name: &str) {
        self.seq += 1;
        self.h.push(Reverse((at, self.seq, name.to_string())));
    }
    fn pop_due(&mut self, now: u64) -> Vec<String> {
        let mut out = Vec::new();
        while let Some(Reverse((at, _, _))) = self.h.peek() {
            if *at > now {
                break;
            }
            let Reverse((_, _, name)) = self.h.pop().unwrap();
            out.push(name);
        }
        out
    }
}

/// In-place heapsort for any Ord slice.
fn heap_sort<T: Ord>(a: &mut [T]) {
    let n = a.len();
    for i in (0..n / 2).rev() {
        sift_down_max(a, i, n);
    }
    for end in (1..n).rev() {
        a.swap(0, end);
        sift_down_max(a, 0, end);
    }
}
fn sift_down_max<T: Ord>(a: &mut [T], mut i: usize, n: usize) {
    loop {
        let l = 2 * i + 1;
        if l >= n {
            return;
        }
        let mut c = l;
        if l + 1 < n && a[l + 1] > a[l] {
            c = l + 1;
        }
        if a[c] <= a[i] {
            return;
        }
        a.swap(i, c);
        i = c;
    }
}
```

### 10.5 Rust tests (`main`)

```rust
// file: rust/main.rs  (continued)
fn main() {
    std_heap_tour();

    // manual heap
    let mut h = Heap::new(|a: &i32, b: &i32| a < b);
    for x in [5, 3, 8, 1, 9, 2, 7] {
        h.push(x);
    }
    let mut got = vec![];
    while let Some(v) = h.pop() {
        got.push(v);
    }
    assert_eq!(got, vec![1, 2, 3, 5, 7, 8, 9]);

    // heapify matches the dry run of Section 5.4
    let hv = Heap::from_vec(vec![9, 4, 7, 1, 6, 3, 8], |a: &i32, b: &i32| a < b);
    assert_eq!(hv.as_slice(), &[1, 4, 3, 9, 6, 7, 8]);
    assert!(!hv.is_empty() && hv.len() == 7 && hv.peek() == Some(&1));

    // Dijkstra: both styles must agree
    let g: Graph = vec![vec![(1, 4), (2, 1)], vec![(3, 1)], vec![(1, 2), (3, 5)], vec![], vec![]];
    let want = vec![0, 3, 1, 4, u64::MAX];
    assert_eq!(dijkstra_lazy(&g, 0), want);
    assert_eq!(dijkstra_indexed(&g, 0), want);

    assert_eq!(top_k(&[5, 1, 9, 3, 7, 8], 3), vec![9, 8, 7]);
    assert_eq!(top_k(&[1], 0), Vec::<i32>::new());
    assert_eq!(merge_k(&[vec![1, 4, 7], vec![2, 5], vec![], vec![3, 6, 8, 9]]), (1..=9).collect::<Vec<_>>());

    let mut m = MedianFinder::new();
    assert_eq!(m.median(), None);
    let mut meds = vec![];
    for x in [5, 15, 1, 3] {
        m.add(x);
        meds.push(m.median().unwrap());
    }
    assert_eq!(meds, vec![5.0, 10.0, 5.0, 4.0]);

    let mut q = EventQueue::new();
    q.schedule(10, "b1");
    q.schedule(5, "a");
    q.schedule(10, "b2");
    q.schedule(20, "late");
    assert_eq!(q.pop_due(10), vec!["a", "b1", "b2"]);
    assert_eq!(q.pop_due(100), vec!["late"]);

    let mut v = vec![9, 4, 7, 1, 6, 3, 8, 3];
    heap_sort(&mut v);
    assert_eq!(v, vec![1, 3, 3, 4, 6, 7, 8, 9]);
    let mut empty: Vec<i32> = vec![];
    heap_sort(&mut empty);

    println!("Rust: all tests passed");
}
```

Build: `rustc -O --edition 2021 rust/main.rs && ./main`  (warnings about unused methods are harmless).
In a real project, move the `assert`s into `#[test]` functions and run `cargo test`.

---

## 11. Advanced manipulations

### 11.1 Changing a key: `fix`, `update`, `decrease`, `increase`

Once an item lives inside the heap and its priority changes, the heap-order invariant may break
**only around that node**:

```text
 new key is HIGHER priority than before  -> may violate parent edge  -> sift-UP
 new key is LOWER priority than before   -> may violate child edges  -> sift-DOWN
 unsure                                  -> call both; at most one moves anything
```

This is Go's `heap.Fix`, and `ipq_update` in C. Prerequisite: you must **know the node's index**.
Heaps don't support search, so store the index alongside the element (Go: `Task.index`; C: `pos[]`;
Rust: `pos: Vec<Option<usize>>`). Every swap must update both nodes' recorded indices. That is the
single invariant of an indexed heap: `pos[heap[i]] == i` for all `i`.

### 11.2 Removing an arbitrary element at index `i`

```text
 1. last = n - 1
 2. swap a[i] with a[last]; shrink by one; (if i == last you are done)
 3. the element now at i came from the bottom: it may be too big OR too small for this spot
    -> fix(i): sift-up then sift-down
```

Why both directions? The moved element is from a *different branch*, so it might be smaller than
`i`'s parent (needs to go up) or larger than `i`'s children (needs to go down).

```text
 remove 4 (index 1) from [1,4,2,9,6,8,3]  -> swap in last (3):  [1,3,2,9,6,8]
        1                                           1
      /   \                                       /   \
     4     2       ==>    move 3 up? 3>1 no;     3     2     3 <= 9,6: no sift-down needed
    / \   / \             sift-down? children   / \   /
   9   6 8   3            9,6 both > 3          9   6 8
```

### 11.3 Indexed heaps in one picture

```text
 id space (size N)                   heap array (size n <= N)
 +----+----+----+----+----+         +----+----+----+
 |pos |pos |pos |pos |pos |         |id=3|id=0|id=4|      heap[i]  = id at position i
 | -1 |  1 | -1 |  0 |  2 |         +----+----+----+      pos[id]   = position of id (or -1)
 +----+----+----+----+----+           0    1    2         key[id]   = its priority
   id0  id1  id2  id3  id4
                                    Invariant:  pos[heap[i]] == i   and   heap[pos[id]] == id
```

Cost: extra O(N) memory and two arrays to keep synchronized in exchange for O(log n)
`decrease_key`/`remove`/`contains` by ID, and no duplicates.

### 11.4 Lazy deletion (the pragmatic alternative)

Instead of updating in place, **push a new entry and ignore stale ones when popped**.

```text
 Dijkstra: dist[v] improves from 9 to 5.

 heap holds (9,v) and now also (5,v).
 pop -> (5,v): process normally.
 later pop -> (9,v): d=9 > dist[v]=5, so it is stale: skip it.
```

| Aspect             | Lazy deletion                         | Indexed heap                            |
| ------------------ | ------------------------------------- | --------------------------------------- |
| Code complexity    | very low                              | moderate (index bookkeeping)            |
| Heap size          | up to O(E) entries                    | at most O(V) entries                    |
| Extra memory       | none                                  | O(V) position table                     |
| Best when          | sparse graphs, few updates, std heaps | many updates, large N, updates are hot  |
| Works with std PQ? | yes (Rust `BinaryHeap`, Go generic)   | no (needs custom heap)                  |

The same trick cancels timers: mark the job `cancelled` in a side table and drop it when it surfaces.

### 11.5 Making equal priorities deterministic (stability)

Heaps are not stable. Add a **monotonic sequence number** as a tiebreaker: compare `(priority, seq)`.
See `Scheduler` (Go) and `EventQueue` (Rust). Cost: 8 more bytes per item. Benefit: FIFO among
equals and reproducible behaviour in tests.

### 11.6 Bounded / capped priority queues

Keep only the best `k`: use a heap of the *opposite* polarity of what you want to keep
(top-k largest -> min-heap of size k) so the root is always "the first to be evicted".

```text
 want the 3 LARGEST of stream 5,1,9,3,7,8         min-heap of size 3 (root = smallest kept)

 5      [5]
 1      [1,5]
 9      [1,5,9]
 3      3 > root(1): replace -> [3,5,9]
 7      7 > root(3): replace -> [5,7,9]
 8      8 > root(5): replace -> [7,8,9]      answer: 9,8,7 (sorted at the end)
```

### 11.7 Double-ended priority queues

Need both `pop_min` and `pop_max`? Options: two heaps with lazy deletion; a min-max heap
(alternating min/max levels, O(1) peek of both, O(log n) pops); an interval heap; or a balanced BST
(`BTreeMap` in Rust; there is no built-in ordered map in Go's stdlib).

### 11.8 Merging heaps

Binary heap: append + heapify, O(n + m). If you need frequent merges, use a leftist/skew/pairing/
binomial/Fibonacci heap (Section 12).

### 11.9 Thread safety

Priority queues are rarely lock-free. The standard pattern is a mutex around the heap plus a
condition variable/channel so consumers can block when it is empty. Go: `sync.Mutex` (+ `sync.Cond`).
Rust: `Mutex<BinaryHeap<T>>` + `Condvar`. C: `pthread_mutex_t` + `pthread_cond_t`.

---

## 12. Heap variants

### 12.1 d-ary heap

Each node has `d` children. Indices (0-indexed): `parent(i) = (i - 1) / d`, `children = d*i + 1 .. d*i + d`.

```text
 d = 4 (n = 21):

 level 0:                            [0]
 level 1:          [1]     [2]     [3]     [4]
 level 2:  [5..8]  [9..12] [13..16] [17..20]        height = log_4 n  (half of log_2 n)
```

| Effect                       | Result                                                                 |
| ---------------------------- | ---------------------------------------------------------------------- |
| Shallower tree               | `push` / `decrease_key` do fewer levels: O(log_d n)                    |
| More children to scan        | `pop` does d comparisons per level: O(d log_d n)                       |
| Better spatial locality      | children of a node are contiguous (d=4 with 16-byte items = one cache line) |
| When it wins                 | workloads dominated by `push`/`decrease_key`, e.g. Dijkstra on dense graphs, and large heaps |

A 4-ary min-heap is a common production choice; for instance, historically the Go runtime's per-P
timer heap was 4-ary (implementation details vary by Go version, so treat that as background, not contract).
Choosing `d ≈ 1 + E/V` balances Dijkstra's push-heavy and pop-heavy costs.

### 12.2 Leftist / skew heaps

Pointer-based heaps whose merge is the core operation (push = merge with a singleton; pop = merge the
two children). Merge is O(log n) (leftist, worst case) or amortized O(log n) (skew). Elegant and
recursive, but poor cache behaviour.

### 12.3 Binomial heap

A *forest* of binomial trees, at most one of each order, like the binary representation of `n`.

```text
 n = 13 = 1101b  ->  trees of order 3, 2, 0

 B0:  o          B2:      o            B3:            o
                         /|\ ...                 / /  \  \ ...
 merging two B_k trees (linking roots, smaller root wins) yields one B_{k+1}: just like binary addition with carry.
```

`push` is a merge with a B0 (amortized O(1)), `merge` is O(log n), `pop` is O(log n).

### 12.4 Fibonacci heap

Lazy binomial-style forest with cascading cuts:

* `push`, `merge`, `peek`: O(1)
* `decrease_key`: O(1) *amortized* (cut the node, mark its parent)
* `pop`: O(log n) amortized (consolidate the root list)

Theoretically ideal for Dijkstra (O(E + V log V)), but pointer-heavy with large constants; binary
and d-ary heaps typically win in practice unless the graph is huge and dense. Recognize it for
interviews and theory; rarely implement it.

### 12.5 Pairing heap

A simple multi-way tree with O(1) push/merge and a two-pass pairing on `pop`. In practice it is
often the fastest pointer-based heap, though its analysis is still not fully settled.

### 12.6 Bucket queues, radix heaps, and timing wheels: when keys are integers

If priorities are small non-negative integers you can beat the comparison-model bound.

```text
 Dial's algorithm / bucket queue: keys 0..C-1

 bucket[0] -> items with key 0
 bucket[1] -> items with key 1        pop-min: advance a cursor to the next non-empty bucket
 bucket[2] -> ...                      O(1) push, amortized O(1) pop for bounded key ranges

 Hierarchical timing wheel: buckets of increasing granularity (ms, s, min, ...)
   O(1) insert/cancel, used by kernel timers and by runtimes such as Tokio (which uses a timer wheel, not a heap).
```

Radix heaps exploit monotone keys (Dijkstra's popped keys never decrease) to get very fast integer-key queues.

### 12.7 Why not a balanced BST?

A BST gives sorted iteration, `predecessor`/`successor`, and arbitrary `delete` by key, at the price
of pointer nodes, rotations, and worse constants. Use a heap when you only ever need "the best one".
Use an ordered map/set when you also need ranges, order statistics, or search.

---

## 13. Problem patterns

Recognizing *why* a heap applies matters more than memorizing problems. A heap fits when:

```text
 - you repeatedly need the current best (min/max) of a CHANGING collection, and
 - you do NOT need the whole thing sorted at every moment.
```

| # | Pattern                          | Heap polarity / trick                                    | Examples                                            |
| - | -------------------------------- | -------------------------------------------------------- | --------------------------------------------------- |
| 1 | Top-k / k-th largest             | **min**-heap of size k (evict the weakest)               | k largest numbers, k most frequent words            |
| 2 | Top-k smallest / k-th smallest   | **max**-heap of size k                                   | k closest points to origin                          |
| 3 | K-way merge                      | min-heap of `(value, list, index)` cursors               | merge k sorted lists/files, external sort           |
| 4 | Two heaps (streaming median)     | max-heap lower half + min-heap upper half                | running median, sliding-window median (+ lazy delete) |
| 5 | Greedy scheduling                | heap of end times / deadlines                            | meeting rooms II, task scheduler, CPU scheduling    |
| 6 | Shortest-path / best-first       | min-heap on distance / f = g + h                         | Dijkstra, Prim's MST, A*                            |
| 7 | Repeated merging of smallest     | min-heap; pop two, push combined                         | Huffman coding, minimum cost to connect ropes       |
| 8 | Event simulation                 | min-heap on event time (+ seq tie-breaker)               | discrete-event simulators, timers, retry queues     |
| 9 | Lazy-deletion sliding structures | heap + "expired" test on pop                             | sliding-window maximum (or use a monotonic deque)   |
| 10| Heapsort / partial sort          | in-place max-heap                                        | guaranteed O(n log n), O(1) space; `nth_element`-style |

Worked-out code: top-k (8.3/9.3/10.4), k-way merge (Go/Rust), running median (all three), Dijkstra
(all three), event scheduling (Go/Rust).

### 13.1 Top-k: why the polarity is *inverted*

```text
 want the K LARGEST  ->  keep a MIN-heap of size K
 root = smallest of the K best = "the bar a newcomer must beat".
 newcomer x > root  -> x deserves a seat; evict root (replace = single sift-down).
 newcomer x <= root -> cannot be in the top K; ignore in O(1).
```

Time: O(n log k). If `k ≈ n` just sort; if `k` is tiny, nearly all candidates are rejected in O(1).
When the whole array is in memory and only one order statistic is needed, quickselect (average O(n))
is an alternative; the heap wins for **streams** and when you need memory O(k).

### 13.2 Two heaps: invariants make the median O(1)

```text
   lo (MAX-heap)           hi (MIN-heap)
   lower half              upper half
   top = largest of lower  top = smallest of upper

   [ 1  3  5 ]  |  [ 8  10  15 ]         median = (5 + 8) / 2  when sizes equal
       ^ root                ^ root         median = root of lo   when lo has one extra

   Invariants:  max(lo) <= min(hi)     and     len(lo) - len(hi) in {0, 1}
```

Every `add` inserts on the correct side, then rebalances by at most one move. O(log n) per add,
O(1) per median query.

### 13.3 Dijkstra: why non-negative weights make a heap sufficient

Invariant: when a node is popped from the min-heap, its distance is final. Proof sketch: any
alternate path to it must pass through some still-unsettled node whose tentative distance is
>= the popped one, and the remaining edges cannot reduce the total because weights are >= 0. A
negative edge invalidates this (use Bellman-Ford).

---

## 14. Real-world architecture

### 14.1 Timer/scheduler queue (event loops, runtimes, job schedulers)

```text
             +---------------------+          +---------------------------------+
 schedule -->|  min-heap by `when` |--peek--->| event loop: sleep until top.when |
 cancel   -->|  (when, seq, task)  |<--pop----| run all tasks with when <= now   |
             +---------------------+          +---------------------------------+
                      ^                                    |
                      +---- periodic tasks re-push --------+
```

* Heap gives O(1) "when is the next thing due?" (so the loop knows exactly how long to sleep) and O(log n) inserts.
* Cancel = lazy deletion flag, or indexed removal.
* Alternatives for enormous timer counts: hierarchical timing wheels (O(1)), used by the Linux kernel's timer subsystem and by Tokio.
* Real example of the tradeoff: several language runtimes have used per-thread/per-core timer heaps, and the specific data structure has changed across versions, so check the source for the version you use.

### 14.2 OS/process scheduling

Textbook priority scheduling uses a heap or array of run queues per priority level. Note that Linux's
default CFS/EEVDF scheduler uses a **red-black tree** (an ordered structure keyed by virtual
runtime), not a binary heap: a good example of needing ordered removal/search beyond "peek the
best".

### 14.3 Routing and navigation (Dijkstra / A*)

```text
              +--------+   pop best f   +-----------------------+
 start ------>|  open  |--------------->| expand neighbours     |
              |  heap  |<-- push/dec ---| relax edges g+w < g?  |
              +--------+                +-----------------------+
                   ^  (indexed heap or lazy duplicates)   |
                   +----------------------------------------+
   closed set / dist[] table (array indexed by node id)
```

A* is Dijkstra with key `f = g + h(n)` where `h` is an admissible heuristic.

### 14.4 External sorting / log merging (k-way merge)

```text
 sorted run 1 --+
 sorted run 2 --+--> [ min-heap of k cursors ] --> merged output stream (one pass over data, O(N log k))
 sorted run 3 --+        (LSM-tree compaction, database merge-sort, `sort -m`)
```

Each pop yields the globally next record, and the cursor that supplied it is advanced and re-pushed.

### 14.5 Compression: Huffman

```text
 frequencies: a:45 b:13 c:12 d:16 e:9 f:5
 repeat: pop two smallest, push their combined node       (f:5 + e:9 = 14, then c:12 + b:13 = 25, ...)
 min-heap gives the greedy step in O(log n): total O(n log n)
```

### 14.6 Rate limiters, retry queues, and caches

* Retry with backoff: heap keyed by `next_attempt_time`.
* Expiring caches (TTL): heap keyed by `expires_at` to evict the earliest-expiring entries.
* Top-k analytics (trending items, heavy hitters): bounded heaps as in 11.6.

---

## 15. Common mistakes and debugging

For each mistake: why it looks reasonable, when it fails, a minimal counterexample, and how to catch it.

| # | Mistake | Why it seems fine | Minimal failing case | How to detect / fix |
| - | ------- | ----------------- | -------------------- | ------------------- |
| 1 | Expecting sorted iteration/printing of the array | The root is minimal, so "it looks sorted" | `[1,3,2]` is a valid min-heap; array is not sorted | Pop repeatedly or `into_sorted_vec`; never rely on array order |
| 2 | Sift-down swaps with the **larger** child | "Just swap with any smaller-than-x child" | `x=9`, children `4`,`6` -> put 6 above 4 | Assert the invariant after every operation in tests (below) |
| 3 | Forgetting the `r < n` bounds check | Most nodes have two children | `n = 2`: node 0 has only a left child | Test sizes 0,1,2,3 explicitly |
| 4 | 1-indexed formulas on a 0-indexed array (or vice versa) | `i/2` vs `(i-1)/2` look alike | `parent(1)` should be 0; `1/2 = 0` works, but `parent(2) = 1` is wrong (should be 0) | Write index math as three named helpers and unit-test them |
| 5 | `size_t` underflow (C/Rust): `(i-1)/2` at `i=0`, or `for (i = n/2 - 1; i >= 0; i--)` | Works for signed `int` in examples | `i` unsigned: `i >= 0` is always true -> infinite loop / OOB | Loop as `for (i = n/2; i > 0; i--) use(i-1)`, or iterate `(0..n/2).rev()` |
| 6 | Mutating a key inside the heap and not repairing | The struct field is right there | Change root's key to a large value, pop -> wrong element | Use `Fix`/`update`; in Rust the borrow checker mostly prevents it (use `peek_mut`) |
| 7 | Inconsistent comparator (`<=` where strict needed; `NaN`; `a-b` overflow) | Passes small tests | C: `return a - b` with `INT_MIN` and `1` | Use `(a>b)-(a<b)`; wrap floats (`total_cmp`); property-test with random data |
| 8 | Confusing `container/heap`'s `h.Push` and `heap.Push` | Same names | Calling `h.Push(x)` appends without sifting -> broken order | Always call package functions `heap.Push/Pop/Fix/Remove` |
| 9 | Go `Pop` leaves references in the backing array | Length shrank, seems gone | `[]*BigObj` heap never releases objects | `old[n-1] = nil` before truncating |
| 10| Rust: assuming `BinaryHeap` is a min-heap | Python's `heapq` is min | `BinaryHeap::from([3,1,2]).pop()` gives `3` | Wrap in `Reverse`, or flip `Ord` |
| 11| Rust: iterating `heap.iter()` and expecting order | Looks like a collection | order is internal layout | `into_sorted_vec()` or pop in a loop |
| 12| Dijkstra without the stale check with lazy deletion | Still gives correct distances | Correct but does redundant work: can degrade toward O(VE) | `if d > dist[u] { continue }` |
| 13| Dijkstra with negative edges | Code "works" on examples | `A->B(2), A->C(3), C->B(-2)` | Use Bellman-Ford, or reweighting (Johnson) |
| 14| Using a max-heap for "k largest" | Intuitive polarity | heap grows to n; O(n log n) time, O(n) space | Use min-heap of size k |
| 15| Building by `n` pushes when the data is ready | It's the API you know | Fine for correctness, Θ(n log n) instead of Θ(n) | Use `heapify` / `BinaryHeap::from` / `heap.Init` |
| 16| Duplicate keys and expecting FIFO | Insertion order feels natural | two events at the same time run in arbitrary order | Add a sequence number tiebreaker |

**Universal invariant checker** (drop into tests; O(n)):

```text
 for i in 1 .. n-1:
     assert !less(a[i], a[parent(i)])        // no child outranks its parent
 // and after every operation: len matches, shape is implicit in the array
```

Combine it with **randomized differential testing**: run the same random operations on your heap
and on a trivially-correct reference (a sorted slice or an unsorted array with linear scan) and
compare every `pop`. The Go test `TestRandomAgainstSort` uses this approach.

**Debugging protocol:** (1) shrink the failing input to the smallest sequence of operations,
(2) print the array after each operation, (3) check the invariant after each operation to find the
*first* violating step, (4) inspect the index math and the child-selection code at that step.

---

## 16. Active recall

Try these **without scrolling up**. Then check yourself against the text.

1. **Concept.** Explain why a heap is a *partial* order and give a valid min-heap of 5 elements in which two siblings are out of order.
2. **Tracing.** Starting from the empty min-heap, push `7, 3, 9, 1, 5, 2` one at a time, writing the array after every push. Then perform two pops and write the array after each.
3. **Tracing.** Heapify `[8, 6, 7, 5, 3, 0, 9]` into a *max*-heap with Floyd's algorithm. Show the array after each `sift_down`.
4. **Complexity.** Why is heapify O(n) but n pushes O(n log n)? Answer using *height vs depth*.
5. **Edge cases.** What are the children of index 4 in a heap of size 8? Of index 3? Which indices are leaves for n = 10?
6. **Design.** You need `decrease_key` for a Dijkstra on a graph of 10^6 nodes and 10^7 edges. Compare lazy deletion with an indexed heap in memory and expected heap size.
7. **Transfer.** Why must the top-k **largest** algorithm use a **min**-heap? What changes for the k **smallest**?
8. **Language.** In Go, what does `heap.Push(h, x)` do that `h.Push(x)` does not? In Rust, how do you get a min-heap out of `BinaryHeap`? In C, what must a comparator return, and why is `return a - b;` wrong?
9. **Debug.** Your heap-based scheduler runs two jobs scheduled for the same instant in a different order on every run. Why, and how do you make it deterministic?
10. **Prediction.** Given `d = 4` (0-indexed), what are the parent and children of index 13?

---

## 17. Independent practice and progress tracker

Do **not** look up solutions first. Use the framework: restate, small case by hand, brute force,
find the invariant, choose polarity, then code.

**Beginner**
1. Implement `is_min_heap(array)` and use it as an invariant checker for your own heap.
2. Find the k-th largest element of an unsorted array using a heap of size k. Then explain when quickselect is better.

**Intermediate**
3. Merge k sorted linked lists (or slices) into one sorted list. State complexity in terms of both `N` and `k`.
4. Schedule tasks so that the same task type is separated by a cooldown of `n` intervals (max-heap by remaining count plus a cooldown queue).
5. Implement a *stable* priority queue and prove FIFO among equal priorities using a sequence number.

**Variations (change an assumption)**
6. Running median where you must also **remove** elements (sliding window). Hint: lazy deletion with counts.
7. Dijkstra when the weights are integers in `[0, C]`. Can you beat `O(E log V)`? (Think Dial's buckets.)
8. Convert your generic heap to a **d-ary heap** with `d` as a parameter; benchmark d = 2, 4, 8 on push-heavy and pop-heavy workloads.
9. Implement a min-max heap (or two heaps + lazy deletion) supporting `pop_min` and `pop_max`.

**Mastery ladder** (Level 0 unfamiliar -> Level 4 transfer)

| Topic                              | Level (self-rate) | Notes / recurring mistakes |
| ---------------------------------- | ----------------- | -------------------------- |
| Heap shape + index math            |                   |                            |
| Sift-up / sift-down                |                   |                            |
| Heapify + O(n) proof               |                   |                            |
| Top-k / polarity reasoning         |                   |                            |
| Two heaps / median                 |                   |                            |
| Indexed heap / decrease-key        |                   |                            |
| Lazy deletion trade-offs           |                   |                            |
| Go container/heap details          |                   |                            |
| C generic heap (memory, cmp)       |                   |                            |
| Rust BinaryHeap / Reverse / Ord    |                   |                            |

Paste this table (filled in) into a new conversation to resume where you left off.

**Key takeaways**

* A heap is a **complete binary tree** (shape) + **parent beats children** (order), stored in an **array** with index arithmetic.
* Every manipulation is `sift_up` or `sift_down`; `push` = append + up, `pop` = last-to-root + down, `update/remove` = swap + up and/or down, `heapify` = down on every internal node from the bottom.
* Heapify is Θ(n) because most nodes have tiny height; pushes are Θ(log n) worst case because sift-up cost is depth.
* Choose polarity by asking "who gets evicted first?" (top-k largest -> min-heap).
* Updating priorities needs an index (indexed heap) or a lazy-deletion strategy.
* Heaps are not sorted and not stable: use sequence numbers when order among equals matters.
* Go, C, and Rust differ mostly in *who owns memory* and *how the comparator is expressed*, not in the algorithm.

---

## 18. How the code in this guide was verified

Every block marked with a `// file:` comment was extracted and run in a Linux container with:

* **Go 1.22.2**: `go vet`, `go test`, `go run`
* **gcc 13.3** (C11): compiled with `-Wall -Wextra` and AddressSanitizer + UndefinedBehaviorSanitizer, then run
* **rustc 1.75**: compiled and run

The tests assert the exact array states from the dry runs in Sections 5.2 to 5.4. The Go benchmarks
compile and run but their numbers are hardware- and version-dependent, so none are quoted here. The
prose claims about the internals of other runtimes (Section 12.1 and 14) are background information
that I did not verify against source in this session, so check them against your version before
relying on them.
