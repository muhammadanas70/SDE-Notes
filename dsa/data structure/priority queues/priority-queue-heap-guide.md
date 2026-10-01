# Priority Queues & Binary Heaps — A Systems Engineer's Field Guide

> Scope: this document treats the Priority Queue as an **abstract data type (ADT)**
> and the Binary (Min-)Heap as the **concrete data structure** that implements it
> efficiently. Everything is built up from invariants, not memorized code. C, Go,
> and Rust implementations are production-oriented, not textbook toys, and the
> applications section is deliberately biased toward networking, cloud security,
> and kernel-adjacent systems work.

---

## Table of Contents

1. [ADT vs Data Structure — the distinction that matters](#1-adt-vs-data-structure)
2. [The conceptual ladder](#2-the-conceptual-ladder)
3. [Complete binary trees and array representation](#3-complete-binary-trees-and-array-representation)
4. [The heap invariant](#4-the-heap-invariant)
5. [Core operations derived from the invariant](#5-core-operations-derived-from-the-invariant)
   - 5.1 [Sift-Up (bubble up)](#51-sift-up-bubble-up)
   - 5.2 [Sift-Down (bubble down / heapify)](#52-sift-down-bubble-down--heapify)
   - 5.3 [Insert](#53-insert)
   - 5.4 [Extract-Min](#54-extract-min)
   - 5.5 [Decrease-Key / Increase-Key](#55-decrease-key--increase-key)
   - 5.6 [Delete(i) — arbitrary element removal](#56-deletei--arbitrary-element-removal)
6. [Build-Heap: why it's O(n), not O(n log n)](#6-build-heap-why-its-on-not-on-log-n)
7. [Heap Sort](#7-heap-sort)
8. [Priority Queue ADT — formal interface](#8-priority-queue-adt--formal-interface)
9. [Beyond the binary heap: the heap family](#9-beyond-the-binary-heap-the-heap-family)
10. [Real-world applications in your domain](#10-real-world-applications-in-your-domain)
11. [Concurrency, cache, and memory layout](#11-concurrency-cache-and-memory-layout)
12. [C implementation (production-grade)](#12-c-implementation)
13. [Go implementation (generics + container/heap)](#13-go-implementation)
14. [Rust implementation (Ord-based, ownership-aware)](#14-rust-implementation)
15. [Testing strategy](#15-testing-strategy)
16. [Debugging heap bugs manually](#16-debugging-heap-bugs-manually)
17. [Edge cases checklist](#17-edge-cases-checklist)
18. [Complexity summary](#18-complexity-summary)
19. [Further reading](#19-further-reading)

---

## 1. ADT vs Data Structure

This distinction is the single most important mental model for this whole topic,
and it generalizes far beyond heaps.

- **Abstract Data Type (ADT)** = a *contract*. It specifies **what** operations
  exist and **what** they must guarantee, but says nothing about **how** they're
  implemented. `insert(x)`, `peek_min()`, `extract_min()` — that's the Priority
  Queue ADT. You could implement it with an unsorted array (`insert` O(1),
  `extract_min` O(n)), a sorted array (`insert` O(n), `extract_min` O(1)), a
  balanced BST (`O(log n)` both), or a binary heap (`O(log n)` insert,
  `O(log n)` extract, `O(1)` peek).

- **Data Structure** = a *concrete implementation* with a specific memory layout
  and specific algorithms operating on it. The binary heap is one implementation
  choice among several for the Priority Queue ADT — it happens to be the
  best general-purpose choice because it's array-backed (cache-friendly, no
  pointer chasing, no allocator pressure per node) and gives `O(log n)` worst
  case for both insert and extract with a tiny constant factor.

Why this matters in production: when you see `heap.Push()` in Go's
`container/heap` or `std::collections::BinaryHeap` in Rust, you're looking at
the **data structure**. When you design a rate limiter, a timer wheel, or a
Dijkstra-based path-cost engine, you should reason first at the **ADT** level
("I need the minimum-cost item, cheaply, repeatedly, with fast insertion") and
only *then* pick the concrete structure. Sometimes the binary heap is wrong —
e.g. if you need `O(1)` `decrease-key` at scale (large graphs, many relaxations)
a **Fibonacci heap** or **pairing heap** is asymptotically or empirically
better; if your priorities are small bounded integers, a **radix heap** or
**bucket queue** beats all of them.

**Guiding question for yourself before writing code:** *What are the exact
operations I need, how often does each happen relative to the others, and what
is the value distribution of the keys?* That single question eliminates most
wrong data-structure choices before you write a line of code.

---

## 2. The conceptual ladder

Build understanding bottom-up, in this order. Skipping a rung is where most
"I memorized the code but don't understand why it works" bugs come from.

```text
Binary Tree
    |
    v
Complete Binary Tree          <-- structural constraint (shape property)
    |
    v
Array representation           <-- implicit pointers via index arithmetic
    |
    v
Heap invariant                 <-- ordering constraint (heap property)
    |
    v
Sift-Up                        <-- restore invariant after insert-at-end
    |
    v
Sift-Down                      <-- restore invariant after replace-root
    |
    v
Insert / Extract-Min            <-- composed from the two primitives above
    |
    v
Build-Heap (heapify)            <-- O(n) bulk construction
    |
    v
Priority Queue                  <-- the ADT the heap now implements
    |
    v
Heap Sort                       <-- reuse the same structure for O(n log n) sort
```

Two *independent* properties define a binary heap, and conflating them is the
most common source of confusion:

1. **Shape property**: the tree is a *complete* binary tree (structural).
2. **Heap property**: every parent respects the ordering relation with its
   children (semantic — this is what makes it "min" or "max").

The shape property is what lets you use a flat array with index arithmetic
instead of pointers. The heap property is what gives you `O(1)` access to the
extreme element. They are proven and maintained separately, and every
operation you write is really "temporarily violate one, then restore it in
`O(log n)`."

---

## 3. Complete binary trees and array representation

A **complete binary tree** of height `h` has every level fully filled except
possibly the last, and the last level is filled **left to right** with no gaps.
This is what allows a lossless mapping to a flat array with no wasted slots and
no stored child/parent pointers.

```text
Index:      0   1   2   3   4   5   6
Value:      2   5   7   9   6   8  10

Tree view:

                 2                  <- index 0 (root)
               /   \
              5      7              <- indices 1, 2
             / \    / \
            9   6  8  10            <- indices 3, 4, 5, 6
```

Index arithmetic (0-indexed array):

```text
parent(i)       = (i - 1) / 2      (integer division)
left_child(i)   = 2*i + 1
right_child(i)  = 2*i + 2
```

Why this arithmetic is correct: in a complete binary tree filled level-order,
level `k` starts at index `2^k - 1`. A node at index `i` on level `k` has
children at the start of level `k+1`, offset by `2*(i - (2^k - 1))`. Working
through the algebra collapses to `2i+1` and `2i+2`. You don't need to re-derive
this every time, but you *should* be able to, because it's the same
level-order indexing trick used in segment trees, Fenwick/BIT trees, and
tournament brackets.

**Mental model check:** given index `i`, you can compute parent/children in
`O(1)` with no memory access beyond the array itself — this is why heaps beat
pointer-based trees for cache behavior. A single cache line (64 bytes) holds
16 `int32` heap entries; a pointer-based BST node touches a new cache line on
almost every pointer dereference.

**1-indexed variant** (common in textbooks and in some production code because
the arithmetic is cleaner):

```text
parent(i)      = i / 2
left_child(i)  = 2*i
right_child(i) = 2*i + 1
```

Pick one convention and be consistent; mixing them is a classic off-by-one
source. The C implementation below uses 0-indexed (matches C array semantics
directly); if you ever port to a 1-indexed scheme, waste the `[0]` slot as a
sentinel.

---

## 4. The heap invariant

**Min-heap invariant**: for every node `i` (other than the root),

```text
heap[parent(i)] <= heap[i]
```

Equivalently: every path from the root to a leaf is non-decreasing. This gives
you exactly one strong guarantee — **the minimum of the whole collection is
always at the root, `heap[0]`, in O(1)** — and *no other ordering guarantee*.
Siblings are **not** ordered relative to each other. `heap[1]` and `heap[2]`
have no required relation. This surprises people the first time they print a
heap array and see it "not sorted" — it's not supposed to be. Confusing "heap
order" with "sorted order" is the second most common conceptual bug (the first
being shape vs heap property).

Max-heap: flip the inequality, `heap[parent(i)] >= heap[i]`. Everything below
mirrors directly by inverting the comparator; production code should
parameterize on a comparator function/trait rather than hand-writing two
copies.

**Invariant as a proof obligation**: every operation on the heap must be
understood as "start from a valid heap, perform a local violation, restore
global validity in the cheapest way possible." This framing is what lets you
derive `sift_up`/`sift_down` from scratch instead of memorizing them.

---

## 5. Core operations derived from the invariant

### 5.1 Sift-Up (bubble up)

**When needed:** you just appended a new element at the end of the array
(cheapest possible insertion point for a complete tree — it's the next open
slot, no shifting required). This may violate the invariant *only* along the
path from that new leaf up to the root, because every other edge in the tree
is untouched.

**Derivation:** compare the new element to its parent. If it's smaller
(min-heap), the invariant is violated at that edge — swap them. Now recurse
upward from the new position. Since each step strictly moves toward the root
along a single path, and tree height is `O(log n)` for `n` complete-tree
nodes, this terminates in `O(log n)` swaps.

```text
Insert 1 into:
        2
       / \
      5   7
     / \
    9   6

Step 1: append at next free slot (index 5)
        2
       / \
      5   7
     / \  /
    9  6 1

Step 2: compare 1 with parent 7 -> 1 < 7, swap
        2
       / \
      5   1
     / \  /
    9  6 7

Step 3: compare 1 with parent 2 -> 1 < 2, swap
        1
       / \
      5   2
     / \  /
    9  6 7

Step 4: 1 is now root, no parent -> stop.
```

### 5.2 Sift-Down (bubble down / heapify)

**When needed:** you just overwrote the root with some other value (typically
the last element, after removing the true root for `extract_min`), or you're
bulk-heapifying an unordered subtree. This may violate the invariant *only*
downward from that position, because every subtree not containing the
modified node is still internally valid.

**Derivation:** compare the node to **both** children (not just one — this is
a common bug). Find the smaller of the two children. If the node is larger
than that smaller child, the invariant is violated — swap with the *smaller*
child (swapping with the larger child would fix one edge but could break the
other, or fail to fix the violation at all). Recurse downward into the
subtree you swapped into. Terminates in `O(log n)` because you move down one
level per step and height is `O(log n)`.

```text
Root replaced with 9 (violates invariant):
        9
       / \
      2   3
     / \  / \
    5  6 8  4

Step 1: children of 9 are {2, 3}; smaller is 2 -> swap
        2
       / \
      9   3
     / \  / \
    5  6 8  4

Step 2: children of 9 (now at old-2's slot) are {5, 6}; smaller is 5 -> swap
        2
       / \
      5   3
     / \  / \
    9  6 8  4

Step 3: 9 is now a leaf, no children -> stop.
```

**Why "compare to both children" matters:** if you only compared to the left
child, a heap like `parent=9, left=8, right=1` would swap with `8`, producing
`parent=8, left=9, right=1` — still broken on the right edge (`8 > 1`). Always
compute `min(left, right)` first (handling the case where `right` doesn't
exist because the heap size is odd), then compare the node against that.

### 5.3 Insert

```text
insert(x):
    heap.append(x)              # place at next free slot, index = size-1
    sift_up(index = size - 1)
```
`O(log n)` amortized `O(1)` for the append if using a dynamic array (occasional
`O(n)` resize amortizes out), `O(log n)` worst case for the sift.

### 5.4 Extract-Min

```text
extract_min():
    if size == 0: error("empty")
    min_val = heap[0]
    heap[0] = heap[size - 1]     # move last element to root
    size -= 1
    if size > 0:
        sift_down(0)
    return min_val
```

The trick — moving the **last** element to the root rather than, say, the
right child — is what keeps the shape property intact for free: removing the
last element of a complete tree always preserves completeness, and you only
need to fix the heap-property violation you just introduced at the root, via
sift-down. `O(log n)`.

### 5.5 Decrease-Key / Increase-Key

Needed by Dijkstra, Prim's algorithm, and A* — when an element already in the
queue gets a better (smaller) priority and needs to move up without a full
extract+reinsert.

```text
decrease_key(i, new_value):
    assert new_value <= heap[i]      # for a min-heap, key only goes down
    heap[i] = new_value
    sift_up(i)                        # only ever needs to move toward root
```

**The catch in real systems:** a plain array-backed binary heap gives you
`O(log n)` decrease-key *only if you already know the array index `i`* of the
element. In graph algorithms you're usually updating by *identity* (vertex ID),
not by array index, and the index of a given vertex moves every time a swap
happens. Production Dijkstra implementations solve this with a **handle
map** — an auxiliary hash map or array from vertex ID to current heap index,
updated on every swap. This is the detail that's missing from 90% of
"Dijkstra with a heap" tutorial code and is exactly the kind of thing that
bites you in a real network path-cost engine. See §12–14 for handle-tracking
implementations.

### 5.6 Delete(i) — arbitrary element removal

Needed for things like: a scheduled timer gets cancelled before it fires, or a
flow gets torn down while its eviction timestamp is still in the queue.

```text
delete(i):
    heap[i] = heap[size - 1]
    size -= 1
    if i < size:
        # the replacement value could violate the invariant in EITHER direction
        if i > 0 and heap[i] < heap[parent(i)]:
            sift_up(i)
        else:
            sift_down(i)
```

Note it can require **either** sift direction depending on whether the
replacement value is smaller or larger than what was there — a detail that's
easy to get wrong if you assume delete is "just like extract."

---

## 6. Build-Heap: why it's O(n), not O(n log n)

Naively inserting `n` elements one at a time costs `sum(O(log k))` for
`k = 1..n`, which is `O(n log n)`. But if you already have all `n` elements
in an array and just want to establish the heap property over the whole
thing, you can do dramatically better by calling `sift_down` on every
**non-leaf** node, starting from the *last* non-leaf and working back to the
root:

```text
build_heap(arr):
    n = len(arr)
    for i in range(n // 2 - 1, -1, -1):   # last non-leaf down to root
        sift_down(arr, i, n)
```

`n/2 - 1` is the index of the last internal node (everything after that is a
leaf and trivially satisfies the heap property by itself — a single node is a
valid heap of size 1).

**Why this is O(n), not O(n log n):** the key insight is that `sift_down`'s
cost is bounded by the **height of the subtree rooted at that node**, not by
the height of the whole tree. Most nodes in a complete binary tree are near
the *bottom*, where subtree height is small.

```text
Level from bottom (h)   Number of nodes (approx)   Max sift-down cost
        0                        n/2                       0
        1                        n/4                       1
        2                        n/8                       2
        3                       n/16                       3
        ...
        h                       n/2^(h+1)                  h
```

Total work:

```text
T(n) = sum_{h=0}^{log n} (n / 2^(h+1)) * h
     = (n/2) * sum_{h=0}^{log n} h / 2^h
```

The sum `sum h/2^h` converges to a constant (2) as the number of terms grows
(it's a standard arithmetico-geometric series), so:

```text
T(n) = (n/2) * O(1) = O(n)
```

**Intuition without the algebra:** most of the "mass" of a complete binary
tree lives near the leaves, and leaves need zero sift-down work. Only a tiny
fraction of nodes (the ones near the root) pay the full `O(log n)` cost, and
that fraction shrinks geometrically as you go up. This is the single most
important asymptotic result to internalize about heaps because it's the
*reason* `std::make_heap`, Go's `heap.Init`, and Rust's `BinaryHeap::from(vec)`
are all `O(n)` rather than `O(n log n)` — and it generalizes to any
bottom-up bulk-construction pattern you'll meet elsewhere (e.g. building a
segment tree bottom-up is the same trick).

**Contrast — why inserting one at a time is worse:** with repeated
`insert`, *every* element pays up to `O(log n)` for `sift_up`, because
`sift_up`'s cost is bounded by the *depth from the root*, not the height of
the subtree below it, and most elements are deep in the tree by the time it's
full. This asymmetry (`sift_down` cheap on average, `sift_up` expensive on
average, for a full/near-full heap) is exactly why bulk construction should
always prefer `build_heap` (bottom-up sift-down) over `n` inserts
(bottom-up sift-up) when you have all the data up front.

---

## 7. Heap Sort

Once you have `build_heap` (O(n)) and `extract_min`/`extract_max` (O(log n)
each), **in-place, comparison-based, O(n log n) sort** falls out immediately:
using a **max-heap**, repeatedly swap the root (current max) with the last
unsorted element, shrink the heap's logical size by one, and sift-down the new
root.

```text
heap_sort(arr):
    build_max_heap(arr)                      # O(n)
    for end in range(len(arr) - 1, 0, -1):
        swap(arr[0], arr[end])               # move max to its final position
        sift_down(arr, 0, end)               # heap now logically size `end`
    # arr is sorted ascending, in place
```

Properties worth internalizing (these come up in interviews and in real
"which sort do I pick" decisions):

- **In-place**: `O(1)` extra memory — no merge buffers like mergesort.
- **Not stable**: equal elements can be reordered (the swaps don't preserve
  original relative order).
- **Worst-case O(n log n) guaranteed** — unlike quicksort's `O(n^2)`
  worst case, which matters if an adversary controls input ordering (relevant
  in security contexts: an attacker who can influence sort input and knows
  you use quicksort with a naive pivot can trigger algorithmic-complexity
  DoS; heapsort and introsort — which falls back to heapsort — don't have
  that failure mode).
- **Poor cache locality relative to quicksort** in practice, because sift-down
  jumps across the array by `O(log n)`-sized strides rather than scanning
  contiguous runs — this is why, despite equal asymptotic complexity,
  quicksort/introsort usually beats heapsort on real hardware, and why
  standard library implementations (`std::sort`, Go's `sort.Sort`) use
  introsort-style hybrids rather than plain heapsort.

---

## 8. Priority Queue ADT — formal interface

Independent of implementation, a priority queue guarantees:

```text
PriorityQueue<T> where T has a total order (or a comparator):

  new()                    -> PriorityQueue<T>
  is_empty()               -> bool
  size()                   -> usize
  insert(item: T)          -> void
  peek()                   -> Option<&T>      # highest priority, no removal
  extract()                -> Option<T>       # highest priority, with removal
  # optional, needed by graph algorithms:
  decrease_key(handle, new_priority) -> void
  delete(handle)           -> void
  merge(other: PriorityQueue<T>) -> PriorityQueue<T>
```

"Highest priority" is implementation-defined by direction: a **min-priority
queue** treats numerically smaller as higher priority (shortest path cost,
earliest deadline, smallest RTT) — this is overwhelmingly the more common case
in networking/scheduling code, hence this document's focus on min-heaps. A
**max-priority queue** is the mirror image (highest bandwidth first, most
severe alert first).

**Note on `merge`:** binary heaps are bad at this — merging two binary heaps
of size `n` and `m` costs `O(n + m)` (you have to rebuild). This is precisely
the operation that motivates **binomial heaps**, **Fibonacci heaps**, and
**pairing heaps**, all of which support `O(log n)` or amortized `O(1)` merge.
If your workload frequently merges independent priority queues (e.g.
per-flow queues getting consolidated), a binary heap is the wrong choice —
see §9.

---

## 9. Beyond the binary heap: the heap family

| Structure          | Insert        | Find-min | Extract-min   | Decrease-key   | Merge        | Notes / when to reach for it |
|---------------------|---------------|----------|----------------|-----------------|--------------|-------------------------------|
| Unsorted array       | O(1)          | O(n)     | O(n)           | O(1)            | O(1)         | Only if extract is rare       |
| Sorted array         | O(n)          | O(1)     | O(1)           | O(n)            | O(n+m)       | Only if insert is rare        |
| Binary heap          | O(log n)      | O(1)     | O(log n)       | O(log n)*       | O(n+m)       | Default choice — this doc     |
| d-ary heap           | O(log_d n)    | O(1)     | O(d·log_d n)   | O(log_d n)*     | O(n+m)       | d>2 trades insert for extract |
| Binomial heap        | O(log n)      | O(log n) | O(log n)       | O(log n)        | O(log n)     | Good when merge is frequent   |
| Fibonacci heap       | O(1) amort.   | O(1)     | O(log n) amort.| O(1) amort.     | O(1)         | Best asymptotics; heavy consts|
| Pairing heap         | O(1) amort.   | O(1)     | O(log n) amort.| O(log n)** amort| O(1)         | Simple, fast in practice       |
| Radix / bucket heap  | O(1)          | O(1)     | O(C) buckets   | O(1)            | n/a          | Bounded integer keys only     |

`*` requires a handle/index map, as discussed in §5.5.
`**` pairing heap decrease-key is amortized O(log n) in the best known
analysis, though it performs excellently in practice — this asymptotic vs.
empirical gap is a classic algorithms lesson (see Fredman's work on pairing
heaps).

**Practical guidance for systems work:**

- **Default to a binary heap.** It's cache-friendly, has the smallest constant
  factors, needs no pointers/allocations per element, and covers 95% of real
  use cases (scheduling, top-K, event queues, rate limiting).
- **Reach for a d-ary heap (d=4 is common)** when inserts vastly outnumber
  extracts — fewer levels means cheaper sift-up, at the cost of more
  comparisons per sift-down. Linux's `perf` tooling and several JVM GC
  implementations use 4-ary or higher heaps for exactly this reason.
- **Reach for a Fibonacci or pairing heap** only when you have a
  decrease-key-heavy workload at large scale — the textbook example is
  Dijkstra/Prim on dense graphs with millions of edges, where the number of
  `decrease_key` calls dominates. For most real network topologies (bounded
  degree, not millions of edges), the constant-factor overhead of Fibonacci
  heaps (multiple pointers per node, lazy consolidation bookkeeping) makes a
  plain binary heap with a handle map faster in wall-clock time despite worse
  asymptotics. **Measure before reaching for the "better" asymptotics** — this
  is a recurring lesson in production algorithm engineering.
- **Reach for a radix/bucket heap** when priorities are small bounded
  integers with limited range (e.g. TTL values 0–255, or hop counts) — you
  get `O(1)` insert and near-`O(1)` extract by bucketing, which is exactly how
  some **timer wheel** implementations work (see §10.3) and how Dial's
  algorithm speeds up Dijkstra for bounded integer edge weights.

---

## 10. Real-world applications in your domain

### 10.1 Dijkstra's algorithm / OSPF-style shortest-path computation

Every link-state routing protocol (OSPF, IS-IS) computes shortest paths via a
Dijkstra-family algorithm over the link-state database. The min-priority
queue holds `(cost, vertex)` pairs; you repeatedly extract the minimum-cost
unvisited vertex, relax its outgoing edges, and `decrease_key` any neighbor
whose tentative cost improves. This is the canonical example of why
**decrease-key with a handle map matters in production**: without it, you'd
push a new `(new_cost, vertex)` entry every time a cost improves and simply
skip stale entries on extraction (lazy deletion) — which is simpler to
implement correctly, uses more memory, but avoids the handle-tracking
complexity entirely. **This lazy-deletion trick is what most real
implementations actually use** (including Go's and Rust's standard-library
examples) because handle-map bookkeeping is a common source of bugs and the
memory overhead is rarely the bottleneck. Know both approaches and the
trade-off: handle map = tighter memory, more code complexity; lazy deletion =
simpler code, transient extra memory, needs a "is this entry stale" check on
pop (compare popped cost against the current best-known cost for that
vertex).

### 10.2 Packet / flow scheduling and QoS

Weighted Fair Queuing (WFQ), Deficit Round Robin variants, and various
priority-based packet schedulers conceptually assign each packet (or flow) a
**virtual finish time** and dequeue in finish-time order — a textbook
min-priority-queue application. In the Linux kernel, `net/sched/*` qdisc
implementations (e.g. `sch_fq`, `sch_fq_codel`) use heap-like or
sorted-structure logic (`sch_fq` specifically uses a red-black tree keyed by
scheduled departure time, which is an alternative `O(log n)` ordered
structure — a reminder that **red-black trees and heaps are both valid
answers to "give me ordered access"**, and the choice depends on whether you
need full ordered traversal (`rbtree`) or only ever need the *extreme*
element (`heap`); rbtrees additionally give `O(log n)` arbitrary lookup and
in-order iteration that a heap does not).

### 10.3 Kernel and userspace timer management

Any system managing many pending timers (connection timeouts, retransmission
timers, TCP keepalives, session/flow expiry) needs "give me the next timer to
fire" cheaply. Two competing designs, both worth knowing cold:

- **Min-heap of `(deadline, callback)`**: `O(log n)` insert/cancel/fire, works
  for arbitrary deadline distributions. Good default for a userspace
  scheduler or an eBPF-adjacent control-plane component.
- **Timer wheel** (hashed array of buckets by time slot, used by the Linux
  kernel's `timer_list` / `hrtimer` infrastructure): amortized `O(1)` insert
  and cancel by bucketing timers into discretized time slots, walked once per
  tick. This is effectively a **bucket/radix-heap idea** applied to
  wall-clock deadlines — it trades the heap's flexibility (exact arbitrary
  ordering) for near-constant time when most operations happen within a
  bounded horizon and tick granularity is acceptable. The Linux kernel's
  `hrtimer` subsystem actually uses a **red-black tree** (not a heap or a
  simple wheel) precisely because it needs *exact* nanosecond ordering rather
  than bucketed granularity — another example of "pick the structure that
  matches your actual ordering precision requirement."

**Guiding question:** does your system need *exact* min ordering
(→ heap or rbtree), or can it tolerate bucketed/quantized ordering in exchange
for O(1) amortized behavior (→ timer wheel / radix heap)? This single
question separates a huge class of scheduling-structure decisions.

### 10.4 Rate limiting and connection tracking eviction

Token-bucket and leaky-bucket rate limiters generally don't need a priority
queue (they're O(1) counter-based), but **eviction policies for bounded
connection-tracking tables** (conntrack, NAT tables, TLS session caches) often
do: when the table is full and a new flow arrives, you need "evict the
oldest / soonest-to-expire / least-recently-used entry" cheaply. A min-heap
keyed by expiry time or last-access time gives you `O(log n)` eviction
selection; many production systems instead use an **LRU list** (O(1) via
doubly-linked list + hash map) when the eviction policy is purely
recency-based, and reserve heaps for cases where the eviction key isn't
strictly access-order (e.g. TTL-based expiry mixed with priority classes,
where you need the true minimum of a computed value, not just "the oldest
insertion").

### 10.5 Top-K / anomaly detection in traffic analysis

"Show me the top-K talkers by byte count" or "top-K source IPs by connection
rate" over a sliding window is a classic **bounded min-heap of size K**
pattern: maintain a size-K min-heap; for each new candidate, if it's larger
than the current minimum (`heap[0]`), evict the min and insert the candidate.
This runs in `O(n log K)` for `n` total observations, which is far better
than sorting everything (`O(n log n)`) when `K << n` — exactly the situation
in flow-analytics and IDS/IPS top-talker dashboards.

### 10.6 Huffman coding / compression

Building a Huffman tree (used in DEFLATE/gzip, and relevant if you ever touch
compressed tunneling or log-shipping pipelines) repeatedly extracts the two
lowest-frequency nodes and reinserts their merged parent — a textbook
min-heap application, and a good "does this data structure actually apply
here" exercise: try implementing Huffman-tree construction yourself using the
heap you build in §12–14 as a way to validate your implementation end-to-end.

### 10.7 Event-driven simulation / discrete event scheduling

Any discrete-event network simulator (or a testbed harness you build for
protocol validation) needs "process events in timestamp order" — a min-heap
of `(event_time, event)` is the standard core data structure for such
simulators (ns-3, OMNeT++ internals use heap-like or tree-like event queues
for exactly this reason).

---

## 11. Concurrency, cache, and memory layout

This is where "I understand the algorithm" and "I can ship this in a
multi-core network data path" diverge, and it's worth being deliberate about.

### 11.1 Cache behavior

- **Array-backed binary heap**: sequential-ish access pattern for small
  heaps (root and its first couple of levels fit in one or two cache lines),
  but sift operations on large heaps jump by `O(log n)`-sized index strides,
  which defeats hardware prefetchers for big `n`. Still dramatically better
  than a pointer-chasing tree (BST, Fibonacci heap) because there's no
  indirection — each comparison is a direct array load, not a pointer
  dereference into potentially-unmapped-in-cache memory.
- **d-ary heaps (d=4)** improve cache behavior further for insert-heavy
  workloads: each node's 4 children are more likely to share a cache line
  than a binary heap's 2, and the reduced height means fewer cache-line
  touches per sift-up.

### 11.2 Concurrency patterns

A binary heap has **no natural safe concurrent-access story** — a single
mutex around the whole structure is the correct default and is what you
should reach for first; it's simple, correct, and for most control-plane
uses (route computation, timer management) contention is not the bottleneck.

For high-throughput **data-plane** scenarios (e.g. per-packet scheduling
decisions in a fast path), a single global lock around a heap is often the
wrong answer. Standard approaches, roughly in order of complexity:

1. **Sharding / per-CPU heaps**: give each CPU core (or each RX queue) its own
   local heap, and only merge/coordinate at coarser granularity (e.g.
   periodically, or via a lock-free MPSC queue feeding a single consolidation
   thread). This is the same idea behind per-CPU data structures throughout
   the Linux kernel network stack and is almost always the right first move
   before reaching for a lock-free heap.
2. **Batching**: instead of taking a lock per single insert/extract, batch
   many operations and apply them under one critical section — amortizes lock
   overhead across many logical operations.
3. **Lock-free skip-list-based priority queues**: real lock-free priority
   queues in the literature (e.g. Sundell & Tsigas' lock-free priority queue)
   are typically built on **skip lists**, not arrays, because array-based
   heaps have a fundamental problem for lock-free designs — sift operations
   touch `O(log n)` *different* array slots that aren't adjacent in any
   consistent pattern, making fine-grained lock-free synchronization very
   hard to get correct (ABA problems, non-local invariant violations mid-
   operation). If you truly need lock-free, budget serious time for
   correctness verification (model checking, TLA+, or at minimum heavy
   stress-testing with ThreadSanitizer/loom in Rust) — this is not a "roll
   your own confidently" area.
4. **RCU-style read-mostly patterns** don't map well to priority queues
   because every `extract_min` is a *write* (structural mutation), unlike
   read-heavy lookup structures — keep this in mind before assuming RCU
   tricks from other parts of the kernel network stack transfer directly.

**Guiding question:** is this heap on the control plane (route computation,
timer management — contention is rarely the bottleneck, use a mutex) or the
data plane (per-packet decisions in the fast path — contention *will* be the
bottleneck, and you should shard/batch before reaching for lock-free)?

### 11.3 Memory allocation discipline

In kernel or embedded-adjacent contexts, dynamic reallocation of the backing
array (`realloc`/`Vec::push` growth) is often unacceptable (unpredictable
latency, potential allocation failure in atomic/interrupt contexts). Two
production patterns:

- **Pre-sized capacity**: allocate the max expected size up front
  (`heap_init(capacity)`), reject or block on overflow rather than growing.
- **Fixed-size ring/pool allocation for elements**, with the heap array
  holding indices/handles into that pool rather than values directly —
  decouples "growing the logical heap" from "allocating memory for payloads,"
  which is the standard pattern in kernel timer/event-queue code where the
  event structs themselves are pool-allocated (e.g. `kmem_cache` in Linux).


---

## 12. C implementation

Production-oriented: generic via a comparator function pointer, dynamic
array with controlled growth, explicit error handling (no exceptions in C),
and a handle-map extension for `decrease_key` (needed for Dijkstra-style use).
This is written the way you'd want it in a systems codebase: no hidden
allocation failures, no silent truncation, `size_t` throughout.

```c
/* min_heap.h */
#ifndef MIN_HEAP_H
#define MIN_HEAP_H

#include <stddef.h>
#include <stdbool.h>

/* Comparator: returns <0 if a has higher priority (should be closer to root),
 * 0 if equal priority, >0 if b has higher priority. For a min-heap over
 * plain integers this is just (a - b); for structs, compare the key field. */
typedef int (*heap_cmp_fn)(const void *a, const void *b);

/* Optional: called whenever two elements are swapped, so callers can keep an
 * external handle->index map in sync for decrease_key support. Pass NULL if
 * you don't need decrease_key. */
typedef void (*heap_swap_notify_fn)(void *ctx, size_t idx_a, size_t idx_b);

typedef struct {
    void        **data;        /* array of opaque element pointers */
    size_t        size;        /* number of elements currently stored */
    size_t        capacity;    /* allocated slots */
    heap_cmp_fn   cmp;
    heap_swap_notify_fn on_swap;   /* may be NULL */
    void         *swap_ctx;
} min_heap_t;

/* Returns 0 on success, -1 on allocation failure. initial_capacity may be 0
 * (first insert will allocate a default capacity). */
int  heap_init(min_heap_t *h, size_t initial_capacity, heap_cmp_fn cmp,
               heap_swap_notify_fn on_swap, void *swap_ctx);
void heap_destroy(min_heap_t *h);  /* frees h->data; does NOT free elements */

int  heap_push(min_heap_t *h, void *item);        /* 0 ok, -1 OOM */
bool heap_peek(const min_heap_t *h, void **out);   /* false if empty */
bool heap_pop(min_heap_t *h, void **out);          /* false if empty */

/* i is the current array index of the element (tracked externally via
 * on_swap if you need it to stay valid across pushes/pops). */
void heap_decrease_key(min_heap_t *h, size_t i);
void heap_delete_at(min_heap_t *h, size_t i);

size_t heap_size(const min_heap_t *h);
bool   heap_is_empty(const min_heap_t *h);

/* Validates the heap invariant over the whole array — O(n). Use in tests
 * and debug builds, not on a hot path. */
bool heap_check_invariant(const min_heap_t *h);

#endif /* MIN_HEAP_H */
```

```c
/* min_heap.c */
#include "min_heap.h"
#include <stdlib.h>
#include <string.h>
#include <assert.h>

#define DEFAULT_CAPACITY 16

static inline size_t parent_idx(size_t i) { return (i - 1) / 2; }
static inline size_t left_idx(size_t i)   { return 2 * i + 1; }
static inline size_t right_idx(size_t i)  { return 2 * i + 2; }

static void heap_swap(min_heap_t *h, size_t a, size_t b) {
    if (a == b) return;
    void *tmp = h->data[a];
    h->data[a] = h->data[b];
    h->data[b] = tmp;
    if (h->on_swap) h->on_swap(h->swap_ctx, a, b);
}

static int heap_grow(min_heap_t *h) {
    size_t new_cap = h->capacity == 0 ? DEFAULT_CAPACITY : h->capacity * 2;
    void **new_data = realloc(h->data, new_cap * sizeof(void *));
    if (!new_data) return -1;      /* h->data untouched by realloc on failure */
    h->data = new_data;
    h->capacity = new_cap;
    return 0;
}

int heap_init(min_heap_t *h, size_t initial_capacity, heap_cmp_fn cmp,
              heap_swap_notify_fn on_swap, void *swap_ctx) {
    assert(cmp != NULL);
    memset(h, 0, sizeof(*h));
    h->cmp = cmp;
    h->on_swap = on_swap;
    h->swap_ctx = swap_ctx;
    if (initial_capacity > 0) {
        h->data = malloc(initial_capacity * sizeof(void *));
        if (!h->data) return -1;
        h->capacity = initial_capacity;
    }
    return 0;
}

void heap_destroy(min_heap_t *h) {
    free(h->data);
    h->data = NULL;
    h->size = h->capacity = 0;
}

static void sift_up(min_heap_t *h, size_t i) {
    while (i > 0) {
        size_t p = parent_idx(i);
        if (h->cmp(h->data[i], h->data[p]) < 0) {
            heap_swap(h, i, p);
            i = p;
        } else {
            break;                 /* invariant restored */
        }
    }
}

static void sift_down(min_heap_t *h, size_t i) {
    for (;;) {
        size_t l = left_idx(i);
        size_t r = right_idx(i);
        size_t smallest = i;

        if (l < h->size && h->cmp(h->data[l], h->data[smallest]) < 0)
            smallest = l;
        if (r < h->size && h->cmp(h->data[r], h->data[smallest]) < 0)
            smallest = r;

        if (smallest == i) break;  /* invariant restored */
        heap_swap(h, i, smallest);
        i = smallest;
    }
}

int heap_push(min_heap_t *h, void *item) {
    if (h->size == h->capacity) {
        if (heap_grow(h) != 0) return -1;
    }
    h->data[h->size] = item;
    sift_up(h, h->size);
    h->size++;
    return 0;
}

bool heap_peek(const min_heap_t *h, void **out) {
    if (h->size == 0) return false;
    *out = h->data[0];
    return true;
}

bool heap_pop(min_heap_t *h, void **out) {
    if (h->size == 0) return false;
    *out = h->data[0];
    h->size--;
    if (h->size > 0) {
        h->data[0] = h->data[h->size];
        if (h->on_swap) h->on_swap(h->swap_ctx, 0, h->size); /* moved node */
        sift_down(h, 0);
    }
    return true;
}

void heap_decrease_key(min_heap_t *h, size_t i) {
    assert(i < h->size);
    sift_up(h, i);   /* caller already lowered h->data[i]'s key in place */
}

void heap_delete_at(min_heap_t *h, size_t i) {
    assert(i < h->size);
    h->size--;
    if (i == h->size) return;      /* was the last element, nothing to fix */
    h->data[i] = h->data[h->size];
    if (h->on_swap) h->on_swap(h->swap_ctx, i, h->size);
    /* replacement could violate the invariant upward or downward */
    if (i > 0 && h->cmp(h->data[i], h->data[parent_idx(i)]) < 0)
        sift_up(h, i);
    else
        sift_down(h, i);
}

size_t heap_size(const min_heap_t *h)     { return h->size; }
bool   heap_is_empty(const min_heap_t *h) { return h->size == 0; }

bool heap_check_invariant(const min_heap_t *h) {
    for (size_t i = 1; i < h->size; i++) {
        if (h->cmp(h->data[i], h->data[parent_idx(i)]) < 0)
            return false;   /* child smaller than parent: invariant broken */
    }
    return true;
}
```

**Notes on the design choices:**

- `void *` elements + comparator function pointer is the idiomatic C
  "generic container" pattern (same approach `qsort`/`bsearch` use). It costs
  an indirect call per comparison; if profiling shows the comparator call is
  hot, the fix is code generation (a macro-based or `#include`-template
  specialization per concrete type) — the same trade-off you'd face with
  C++ templates or Rust generics vs. dynamic dispatch, made explicit because
  C has no generics.
- `heap_swap_notify_fn` is the handle-map hook mentioned in §5.5 — without
  it, `decrease_key`/`delete_at` are useless for graph algorithms because
  callers can't find "where is vertex X in the array right now" after any
  prior swap. A typical caller maintains `size_t *vertex_to_index` and
  updates it inside the `on_swap` callback.
- `realloc` failure explicitly leaves `h->data` untouched — a classic C bug
  is `h->data = realloc(h->data, ...)`, which leaks the original block and
  crashes on next access if `realloc` returns `NULL`. The code above avoids
  that by realloc'ing into a temporary first.
- No `free()` of the elements themselves in `heap_destroy` — ownership of
  element memory is left to the caller, matching how `qsort`-style C APIs
  behave; document this loudly in real code, since ownership ambiguity is a
  top source of C memory bugs.

**Minimal usage example (min-heap of `int`):**

```c
static int int_cmp(const void *a, const void *b) {
    int ia = *(const int *)a, ib = *(const int *)b;
    return (ia > ib) - (ia < ib);
}

int main(void) {
    min_heap_t h;
    heap_init(&h, 0, int_cmp, NULL, NULL);

    int values[] = {7, 2, 9, 1, 5, 3};
    for (size_t i = 0; i < 6; i++) heap_push(&h, &values[i]);

    void *min;
    while (heap_pop(&h, &min)) {
        printf("%d\n", *(int *)min);   /* prints 1 2 3 5 7 9 */
    }
    heap_destroy(&h);
    return 0;
}
```

---

## 13. Go implementation

Go's standard library ships `container/heap`, which is an *interface*, not a
ready-made type — you implement `sort.Interface` plus `Push`/`Pop`, and the
package supplies `Init` (build-heap), `Push`, `Pop`, `Fix` (decrease/increase
key at a known index), and `Remove`. Using it correctly, with Go generics
(1.18+) for type safety, and with an explicit index-tracking pattern for
Dijkstra-style use is the production-idiomatic approach — reimplementing the
heap algorithms yourself in Go is rarely justified since the stdlib version
is well-tested and this is exactly the kind of "know when to use the library
vs. roll your own" judgment call worth internalizing.

```go
// pqueue.go
package pqueue

import "container/heap"

// Item is one element of the queue. Index is maintained automatically by
// the heap package's Swap implementation below — this is the Go analogue
// of the C handle-map / on_swap callback from Section 12.
type Item[T any] struct {
	Value    T
	Priority int // smaller = higher priority (min-heap)
	index    int // maintained internally; do not set directly
}

// innerHeap implements heap.Interface. It is unexported: callers interact
// only through PriorityQueue, which wraps it for a safer public API.
type innerHeap[T any] []*Item[T]

func (h innerHeap[T]) Len() int { return len(h) }

func (h innerHeap[T]) Less(i, j int) bool {
	return h[i].Priority < h[j].Priority // min-heap: smaller priority first
}

func (h innerHeap[T]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *innerHeap[T]) Push(x any) {
	item := x.(*Item[T])
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *innerHeap[T]) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid retaining a reference (GC-friendliness)
	item.index = -1
	*h = old[:n-1]
	return item
}

// PriorityQueue is the public, generic API. It wraps container/heap so
// callers never have to remember to call heap.Init/heap.Fix themselves.
type PriorityQueue[T any] struct {
	h innerHeap[T]
}

func New[T any]() *PriorityQueue[T] {
	return &PriorityQueue[T]{h: innerHeap[T]{}}
}

// NewFromSlice builds a queue from existing items in O(n) via heap.Init,
// exactly the build-heap algorithm derived in Section 6.
func NewFromSlice[T any](items []*Item[T]) *PriorityQueue[T] {
	h := innerHeap[T](items)
	for i, it := range h {
		it.index = i
	}
	heap.Init(&h)
	return &PriorityQueue[T]{h: h}
}

func (pq *PriorityQueue[T]) Len() int { return pq.h.Len() }

func (pq *PriorityQueue[T]) Push(value T, priority int) *Item[T] {
	item := &Item[T]{Value: value, Priority: priority}
	heap.Push(&pq.h, item)
	return item // caller keeps this handle for DecreasePriority/Remove
}

func (pq *PriorityQueue[T]) Pop() (T, bool) {
	if pq.h.Len() == 0 {
		var zero T
		return zero, false
	}
	item := heap.Pop(&pq.h).(*Item[T])
	return item.Value, true
}

func (pq *PriorityQueue[T]) Peek() (T, bool) {
	if pq.h.Len() == 0 {
		var zero T
		return zero, false
	}
	return pq.h[0].Value, true
}

// DecreasePriority is the Go analogue of Section 5.5's decrease_key. The
// item's own `index` field (kept correct by Swap above) is what makes this
// O(log n) instead of an O(n) linear scan to find the item first.
func (pq *PriorityQueue[T]) DecreasePriority(item *Item[T], newPriority int) {
	if newPriority > item.Priority {
		panic("newPriority must not increase priority value in a min-heap")
	}
	item.Priority = newPriority
	heap.Fix(&pq.h, item.index) // re-establishes invariant in O(log n)
}

// Remove supports timer cancellation / flow teardown (Section 5.6 / 10.3).
func (pq *PriorityQueue[T]) Remove(item *Item[T]) {
	heap.Remove(&pq.h, item.index)
}
```

**Example: Dijkstra's algorithm using this queue**, tying together §5.5,
§10.1, and the handle-tracking pattern:

```go
package main

import "fmt"

type edge struct {
	to     int
	weight int
}

// dijkstra returns shortest distances from src to all vertices in graph,
// represented as an adjacency list. Demonstrates decrease_key via the
// PriorityQueue's item handles (Section 5.5 / 10.1).
func dijkstra(graph [][]edge, src int) []int {
	const inf = int(^uint(0) >> 1)
	dist := make([]int, len(graph))
	for i := range dist {
		dist[i] = inf
	}
	dist[src] = 0

	pq := New[int]() // Value = vertex id
	itemOf := make([]*Item[int], len(graph))
	itemOf[src] = pq.Push(src, 0)

	for pq.Len() > 0 {
		u, _ := pq.Peek()
		uDist := dist[u]
		pq.Pop()

		for _, e := range graph[u] {
			nd := uDist + e.weight
			if nd < dist[e.to] {
				dist[e.to] = nd
				if itemOf[e.to] == nil {
					itemOf[e.to] = pq.Push(e.to, nd)
				} else {
					pq.DecreasePriority(itemOf[e.to], nd)
				}
			}
		}
	}
	return dist
}

func main() {
	graph := [][]edge{
		0: {{1, 4}, {2, 1}},
		1: {{3, 1}},
		2: {{1, 2}, {3, 5}},
		3: {},
	}
	fmt.Println(dijkstra(graph, 0)) // [0 3 1 4]
}
```

**Why this Go version matters beyond syntax:** it demonstrates the
**lazy-vs-handle-tracked decrease-key trade-off** from §10.1 concretely —
here we chose the handle-tracked approach (`itemOf` array +
`DecreasePriority`) rather than lazy re-insertion with staleness checks.
Either is valid; know which one you're choosing and why.

---

## 14. Rust implementation

Rust's `std::collections::BinaryHeap<T>` is a **max-heap** by default and
requires `T: Ord`. For a min-heap, the idiomatic pattern is `Reverse<T>` from
`std::cmp`, which flips the `Ord` implementation — this is worth
understanding rather than memorizing, since it's the same "invert the
comparator" idea from §4 applied via the type system instead of a runtime
function pointer (contrast with C's function-pointer comparator and Go's
`Less` method — three different language idioms for the same underlying
customization point).

```rust
use std::cmp::Reverse;
use std::collections::BinaryHeap;

fn basic_min_heap_demo() {
    let mut heap: BinaryHeap<Reverse<i32>> = BinaryHeap::new();
    for v in [7, 2, 9, 1, 5, 3] {
        heap.push(Reverse(v));
    }
    while let Some(Reverse(v)) = heap.pop() {
        print!("{v} "); // 1 2 3 5 7 9
    }

    // O(n) build-heap (Section 6) — From<Vec<T>> uses bottom-up heapify,
    // not n individual pushes.
    let data = vec![7, 2, 9, 1, 5, 3];
    let heap2: BinaryHeap<Reverse<i32>> =
        data.into_iter().map(Reverse).collect();
    assert_eq!(heap2.peek(), Some(&Reverse(1)));
}
```

For production use with `decrease_key` support (Dijkstra, §5.5/§10.1),
`BinaryHeap` alone is insufficient — it has no `decrease_key`/`Fix`
operation, and no handle concept at all. The idiomatic Rust answer for
Dijkstra is almost always **lazy deletion** (§10.1) rather than a
handle-tracked heap, because it composes cleanly with `BinaryHeap`'s safe
API and avoids unsafe/index-juggling code:

```rust
use std::cmp::{Ordering, Reverse};
use std::collections::BinaryHeap;

const INF: u64 = u64::MAX;

struct Graph {
    adj: Vec<Vec<(usize, u64)>>, // adj[u] = list of (v, weight)
}

// Lazy-deletion Dijkstra: instead of decrease_key, we push a fresh
// (cost, vertex) entry every time we find a better cost, and simply skip
// any popped entry whose cost is stale (worse than the current best known
// distance). This is the trade-off discussed in Section 10.1: more memory
// churn (duplicate entries), zero index-tracking complexity.
fn dijkstra(graph: &Graph, src: usize) -> Vec<u64> {
    let n = graph.adj.len();
    let mut dist = vec![INF; n];
    dist[src] = 0;

    // Reverse<(cost, vertex)> turns Rust's max-heap into a min-heap ordered
    // by cost (Ord on tuples is lexicographic, so this also ties-break on
    // vertex id, which is a nice deterministic-output side effect).
    let mut pq: BinaryHeap<Reverse<(u64, usize)>> = BinaryHeap::new();
    pq.push(Reverse((0, src)));

    while let Some(Reverse((cost, u))) = pq.pop() {
        if cost > dist[u] {
            continue; // stale entry — a cheaper path to u was already found
        }
        for &(v, w) in &graph.adj[u] {
            let nd = cost + w;
            if nd < dist[v] {
                dist[v] = nd;
                pq.push(Reverse((nd, v)));
            }
        }
    }
    dist
}
```

**Hand-rolled min-heap in Rust**, for cases where you need it as a building
block yourself (e.g. embedded/no_std context where `std::collections` isn't
available, or you need the handle-map decrease_key that `BinaryHeap` doesn't
offer). This mirrors the C implementation in §12 but leans on Rust's type
system for safety — no raw pointers, no manual free, ownership of elements
is unambiguous (the `Vec<T>` owns them):

```rust
pub struct MinHeap<T: Ord> {
    data: Vec<T>,
}

impl<T: Ord> MinHeap<T> {
    pub fn new() -> Self {
        MinHeap { data: Vec::new() }
    }

    pub fn with_capacity(cap: usize) -> Self {
        MinHeap { data: Vec::with_capacity(cap) }
    }

    /// O(n) build-heap from an existing Vec — Section 6's bottom-up
    /// sift-down, not n individual pushes.
    pub fn from_vec(data: Vec<T>) -> Self {
        let mut h = MinHeap { data };
        if h.data.len() > 1 {
            let start = h.data.len() / 2;
            for i in (0..start).rev() {
                h.sift_down(i);
            }
        }
        h
    }

    pub fn len(&self) -> usize { self.data.len() }
    pub fn is_empty(&self) -> bool { self.data.is_empty() }
    pub fn peek(&self) -> Option<&T> { self.data.first() }

    pub fn push(&mut self, item: T) {
        self.data.push(item);
        self.sift_up(self.data.len() - 1);
    }

    pub fn pop(&mut self) -> Option<T> {
        if self.data.is_empty() {
            return None;
        }
        let n = self.data.len();
        self.data.swap(0, n - 1);
        let min = self.data.pop(); // removes the (now-last) former root
        if !self.data.is_empty() {
            self.sift_down(0);
        }
        min
    }

    fn sift_up(&mut self, mut i: usize) {
        while i > 0 {
            let p = (i - 1) / 2;
            if self.data[i] < self.data[p] {
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
            let r = 2 * i + 2;
            let mut smallest = i;
            if l < n && self.data[l] < self.data[smallest] {
                smallest = l;
            }
            if r < n && self.data[r] < self.data[smallest] {
                smallest = r;
            }
            if smallest == i {
                break;
            }
            self.data.swap(i, smallest);
            i = smallest;
        }
    }

    /// O(n) invariant check — for tests/debug builds (Section 15/16).
    pub fn check_invariant(&self) -> bool {
        for i in 1..self.data.len() {
            let p = (i - 1) / 2;
            if self.data[i] < self.data[p] {
                return false;
            }
        }
        true
    }
}
```

**Ordering customization without wrapping in `Reverse` everywhere:** if
you're storing a domain struct (e.g. a flow record) and want min-heap
semantics natively, implement `Ord` yourself with the comparison flipped,
rather than scattering `Reverse` at every call site:

```rust
#[derive(Eq, PartialEq)]
struct ScheduledEvent {
    deadline_ns: u64,
    flow_id: u32,
}

impl Ord for ScheduledEvent {
    fn cmp(&self, other: &Self) -> Ordering {
        // Reversed on purpose: BinaryHeap is a max-heap, and we want the
        // *smallest* deadline to be treated as "greatest" priority so it
        // pops first.
        other.deadline_ns.cmp(&self.deadline_ns)
            .then_with(|| other.flow_id.cmp(&self.flow_id))
    }
}
impl PartialOrd for ScheduledEvent {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}
```

**Why this matters for correctness review:** any time you see a
`BinaryHeap<T>` in a Rust codebase, the very first thing to check in review
is *which direction* `Ord` goes for `T` — is it a natural max-heap, a
`Reverse`-wrapped min-heap, or a custom flipped `Ord` impl like above? This
is the single most common Rust-specific heap bug: someone assumes
`BinaryHeap` is a min-heap (because that's the more common need) and gets
silently wrong ordering with no compiler error.

---

## 15. Testing strategy

Layer your tests the way you'd test any core data structure in a systems
codebase — unit tests alone are insufficient for something whose bugs are
often "wrong under a specific interleaving of operations," which is exactly
what property-based and fuzz testing are built for.

1. **Invariant-based unit tests** (all three languages): after every
   `push`/`pop`/`decrease_key`/`delete`, assert `check_invariant()` /
   `heap_check_invariant()`. This turns "is the heap still valid" from a
   design question into an automated, always-on assertion — cheap insurance
   against regressions in sift logic.

2. **Reference-model differential testing**: maintain a naive reference
   priority queue (e.g. a sorted `Vec`/slice, or repeatedly scanning for the
   min) alongside the heap under test. Apply the *same* randomized sequence
   of operations to both, and assert their observable outputs
   (`pop()` sequence) match. This catches subtle bugs that invariant checks
   alone miss — e.g. an off-by-one in size tracking that happens to still
   leave the heap "valid" but drops or duplicates an element.

3. **Property-based testing** (Rust: `proptest` or `quickcheck`; Go:
   table-driven tests with `testing/quick` or manually generated random
   sequences; C: write your own generator harness or use a fuzzing harness
   directly): generate random sequences of `push(random_value)` /
   `pop()` / `decrease_key(random_index, random_smaller_value)` calls of
   random length, and assert:
   - the invariant holds after every operation,
   - `pop()` always returns the minimum of everything currently "in" the
     heap (cross-check against the reference model),
   - `len()`/`size()` matches the number of successful pushes minus pops,
   - popping until empty yields a fully sorted sequence (this is literally
     testing heap sort's correctness as a side effect).

4. **Fuzz testing** (`cargo fuzz` for Rust with `libFuzzer`, or `go test
   -fuzz` for Go 1.18+, or AFL/libFuzzer harnesses around the C API):
   especially valuable for the C implementation, where memory-safety bugs
   (use-after-free from an incorrect `heap_swap_notify_fn`, buffer overrun
   from a capacity-tracking bug) are possible in a way they structurally
   aren't in Go/Rust. Fuzz the sequence of API calls, not just input values.

5. **Boundary/edge-case unit tests explicitly**, independent of randomized
   testing (randomized testing rarely hits these reliably on its own):
   - empty heap: `peek`/`pop` on empty must fail safely, not crash.
   - single-element heap: push then pop, invariant trivially holds.
   - all-equal-priority elements: stresses the "no ordering guarantee among
     equal keys" property — make sure your code doesn't accidentally assume
     strict ordering.
   - already-sorted input to `build_heap`/`from_vec` (ascending and
     descending) — this is often where an off-by-one in the "last non-leaf
     index" calculation shows up, since it changes how many sift-downs
     actually do any swapping.
   - power-of-two-minus-one sizes (`1, 3, 7, 15, 31, ...`) — a perfectly
     complete tree with no partially-filled last level, a common source of
     boundary bugs in the `left/right < size` checks.
   - decrease_key called with a value that doesn't actually decrease
     (should be rejected/asserted, not silently corrupt the heap).

6. **Performance/complexity regression tests**: for a large `n` (e.g.
   1M elements), assert wall-clock time for `build_heap` scales roughly
   linearly and `push`/`pop` scale roughly logarithmically as `n` grows
   across a few orders of magnitude — not a substitute for Big-O analysis,
   but catches accidental `O(n)` regressions in what should be `O(log n)`
   code (e.g. someone "fixing a bug" by adding an `O(n)` linear scan).

---

## 16. Debugging heap bugs manually

When a heap-based system misbehaves in production (wrong scheduling order,
Dijkstra producing wrong distances, a timer firing at the wrong time), work
through these in order — this is the manual-debugging mental model your
guidelines specifically want you to build, not just "attach a debugger and
hope":

1. **Reproduce with the smallest failing sequence.** Binary-search the
   operation log: replay the first half, does the invariant still hold and
   does behavior still diverge from the reference model? This isolates the
   exact operation (a specific `push`, `decrease_key`, or `delete`) that
   first introduces the discrepancy, rather than debugging the whole system
   at once.

2. **Dump the array and manually verify shape + heap property separately**
   (per §2's distinction). Print `heap[0..size]` and for each index compute
   `parent/left/right` by hand for a handful of nodes:
   - Shape violation (rare if you always append/remove at `size-1`, but
     possible with a size-tracking bug): does the array actually represent a
     *complete* tree of `size` nodes, or is there a gap because `size` was
     decremented/incremented incorrectly somewhere?
   - Heap-property violation: is there any `i` where
     `heap[i] < heap[parent(i)]`? Run `check_invariant()` and, if it fails,
     print the specific `(i, parent(i))` pair — that pinpoints exactly which
     sift operation didn't do its job.

3. **Check the direction of the comparator first**, always, before assuming
   the sift logic is broken. This is disproportionately the actual bug: a
   flipped comparator (in C, `heap_cmp_fn` returning the wrong sign; in Go,
   `Less` implemented backward; in Rust, forgetting `Reverse` or writing
   `Ord` the wrong direction) produces a heap that's *internally consistent*
   (passes `check_invariant` for the *opposite* ordering) but functionally
   wrong. If `check_invariant()` fails, suspect the sift code; if
   `check_invariant()` *passes* but results are wrong, suspect the
   comparator direction or a `Reverse`/wrapper mismatch.

4. **For decrease_key/handle-map bugs specifically**: verify the invariant
   "the handle map always reflects the element's *current* array index" by
   asserting, after every swap, that `data[handle_map[id]] == element_with_id`
   for a few sampled elements. The most common bug class here is updating
   the handle map in some code paths (e.g. inside `sift_up`) but forgetting
   it in others (e.g. the final swap-with-last-element in `delete`/`pop`) —
   grep every place the array is mutated and confirm the handle map update
   is co-located with it, ideally structurally enforced (as in the C design
   above, where *all* swaps funnel through one `heap_swap` function that
   always fires the notify callback — this is a design principle worth
   generalizing: **make the invariant-maintaining code impossible to bypass,
   not just documented**).

5. **Check for the "compare only one child" bug in sift_down** (§5.2) — if
   you see values that are *almost* sorted but with adjacent-level
   inversions clustering on one side of the tree (e.g. always the right
   subtree), suspect a sift_down that only compares against `left` or only
   swaps unconditionally with `left` without checking `right`.

6. **In concurrent contexts**, before suspecting the heap algorithm itself,
   verify the *locking discipline* — dump a sequence-numbered log of every
   lock acquire/release around heap operations and check for a window where
   two threads observed the same "size" or overlapping index ranges. Heap
   bugs under concurrency are overwhelmingly synchronization bugs, not
   algorithm bugs, if the single-threaded path is already fuzz-tested clean
   per §15.

7. **Instrument, don't guess, for performance regressions**: if a heap
   operation that should be `O(log n)` is showing up hot in a profiler at
   large `n`, check first whether the comparator itself is expensive (e.g.
   comparing large structs by value instead of by a precomputed key, or a
   comparator that does a hidden allocation/syscall) before suspecting the
   heap algorithm — the `O(log n)` bound is on the *number of comparisons*,
   not on their cost.

---

## 17. Edge cases checklist

Run through this list explicitly whenever reviewing or writing heap code —
treat it as a code-review checklist, not just a testing afterthought:

- [ ] Empty heap: `peek`/`pop`/`extract_min` handle it without UB/panic/crash.
- [ ] Single element: push then pop round-trips correctly; sift routines
      terminate immediately (no out-of-bounds child access).
- [ ] Two elements: exercises the "only a left child exists, no right child"
      branch of `sift_down` — verify `right_idx(i) < size` guards are
      correct and don't read out of bounds.
- [ ] Odd vs. even `size` generally: confirms the `right child may not
      exist` boundary is handled on every code path, not just the obvious
      one.
- [ ] Duplicate priorities: no code path assumes strict (`<`) ordering
      where non-strict (`<=`) is actually required, or vice versa — decide
      once whether ties are broken arbitrarily (fine for pure priority
      queues) or need a secondary key (e.g. insertion order for FIFO
      tie-breaking, common in scheduler fairness requirements) and be
      consistent.
- [ ] `decrease_key` called with a value that is *not* actually smaller —
      should be rejected (assert/error), because calling `sift_up` when
      `sift_down` was actually needed will silently corrupt the invariant
      without crashing.
- [ ] `delete_at` on the *last* index — must not attempt to move
      "the last element" onto itself in a way that double-counts or
      corrupts size (see the C implementation's explicit
      `if (i == h->size) return;` guard).
- [ ] Build-heap on already-sorted (ascending) input, and on already-sorted
      (descending, i.e. already a valid max-heap being built as a min-heap)
      input — both should still produce a valid result, exercising very
      different amounts of actual swapping.
- [ ] Capacity growth boundary in array-backed implementations (C, or a
      hand-rolled Rust/Go version): push exactly at the old capacity and
      confirm the resize happens *before* the write, not after (an
      off-by-one here is a classic buffer overrun in C).
- [ ] Integer overflow in priority values, if priorities are computed
      (e.g. `cost + weight` in Dijkstra) — verify you're not silently
      wrapping (`u32`/`u64` overflow) in a way that makes a very expensive
      path look artificially cheap; this is a real correctness *and*
      security concern if `weight` is attacker-influenced (e.g. a
      manipulated link-state advertisement).
- [ ] Handle-map consistency after every mutating operation (§16.4) if
      `decrease_key`/`delete` by identity is supported.
- [ ] Comparator direction reviewed explicitly (§16.3) — don't assume;
      verify against a concrete example each time you touch comparator code.
- [ ] Thread-safety story documented explicitly (single mutex? sharded?
      lock-free?) rather than left implicit — "is this heap safe to share
      across threads" should never be a question a caller has to guess at.

---

## 18. Complexity summary

| Operation                         | Binary heap (array) | Notes |
|-----------------------------------|----------------------|-------|
| `peek` / `find-min`               | O(1)                 | read `heap[0]` |
| `insert` / `push`                 | O(log n)             | append + sift-up |
| `extract-min` / `pop`             | O(log n)             | move last to root + sift-down |
| `decrease-key` (index known)      | O(log n)             | sift-up only |
| `delete(i)` (index known)         | O(log n)             | sift-up or sift-down |
| `build-heap` from n elements      | O(n)                 | bottom-up sift-down, §6 |
| `merge` two heaps of size n, m    | O(n + m)             | rebuild; use binomial/Fib/pairing if frequent |
| `heap sort`                       | O(n log n) time, O(1) extra space | in-place, not stable |
| Space overhead per element        | O(1), no pointers    | just the array slot |

---

## 19. Further reading

- **CLRS** (*Introduction to Algorithms*), Ch. 6 (Heapsort) and Ch. 19–20
  (Fibonacci heaps, van Emde Boas trees) — the canonical formal treatment,
  including the amortized-analysis proof of build-heap's O(n) bound this
  document summarized in §6.
- **Sedgewick & Wayne**, *Algorithms* — practical, code-forward treatment of
  binary heaps and heapsort, good for cross-checking implementation details.
- **Fredman & Tarjan (1987)**, "Fibonacci Heaps and Their Uses in Improved
  Network Optimization Algorithms" — the original Fibonacci heap paper;
  directly relevant given your Dijkstra/network-optimization context.
- **Fredman, Sedgewick, Sleator, Tarjan (1986)**, "The Pairing Heap: A New
  Form of Self-Adjusting Heap" — worth reading given pairing heaps'
  practical edge over Fibonacci heaps despite weaker proven bounds.
- **Linux kernel source**: `kernel/time/hrtimer.c` (red-black-tree-based
  high-resolution timers) and `kernel/time/timer.c` (classic timer wheel) —
  read both to internalize the §10.3 trade-off between exact ordered
  structures and bucketed/quantized ones in a real, heavily-optimized
  codebase.
- **Go**: `container/heap` package source and godoc — short enough to read
  end-to-end in one sitting; do so, since §13 builds directly on it.
- **Rust**: `std::collections::BinaryHeap` source
  (`library/alloc/src/collections/binary_heap.rs`) — worth reading to see
  the real bottom-up `from_vec`/`heapify` implementation behind
  `From<Vec<T>>`, and how `PeekMut` is implemented to allow in-place
  priority mutation of the root without a full pop/push.
- **RFC 2328** (OSPF v2) §16 — describes Dijkstra's algorithm as used in
  practice for link-state routing, directly connecting §10.1 to a real
  protocol specification you likely already reference.
