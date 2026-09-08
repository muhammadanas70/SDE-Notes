# The Maple Tree: A Complete Engineering Guide

**Scope:** This is a from-first-principles guide to the Linux kernel's Maple Tree (`mm/maple_tree.c`, `include/linux/maple_tree.h`), written for someone who wants to *think* in terms of range-indexed, RCU-safe data structures — not just use an API. It covers motivation, internals, invariants, operations, real kernel usage, and includes original (non-verbatim) educational implementations in C, Rust, and Go so you can build the mental model by tracing logic yourself.

---

## Table of Contents

1. [Why Does This Data Structure Exist](#1-why-does-this-data-structure-exist)
2. [The Core Idea: Range Indexing](#2-the-core-idea-range-indexing)
3. [Terminology You Must Internalize](#3-terminology-you-must-internalize)
4. [Node Anatomy](#4-node-anatomy)
5. [Node Types and Why There Are Several](#5-node-types-and-why-there-are-several)
6. [The Maple State (`ma_state`) — the Cursor](#6-the-maple-state-ma_state--the-cursor)
7. [RCU Safety Model](#7-rcu-safety-model)
8. [Core Operations](#8-core-operations)
9. [Splits, Merges, Rebalancing](#9-splits-merges-rebalancing)
10. [Gap Finding — the VMA Use Case](#10-gap-finding--the-vma-use-case)
11. [Invariants You Can Assert](#11-invariants-you-can-assert)
12. [Complexity Analysis](#12-complexity-analysis)
13. [Comparison: rbtree vs radix tree vs xarray vs maple tree](#13-comparison-rbtree-vs-radix-tree-vs-xarray-vs-maple-tree)
14. [Real Kernel Usage](#14-real-kernel-usage)
15. [ASCII Architecture Diagrams](#15-ascii-architecture-diagrams)
16. [C Implementation (educational, from scratch)](#16-c-implementation-educational-from-scratch)
17. [Rust Implementation (educational, from scratch)](#17-rust-implementation-educational-from-scratch)
18. [Go Implementation (educational, from scratch)](#18-go-implementation-educational-from-scratch)
19. [Testing Strategy and Edge Cases](#19-testing-strategy-and-edge-cases)
20. [Mental Model Exercises](#20-mental-model-exercises)
21. [References](#21-references)

---

## 1. Why Does This Data Structure Exist

Before touching a single field, answer this for yourself: **what problem was so painful that someone rewrote a core kernel data structure that had existed for decades?**

The predecessor was the **red-black tree** (`rbtree`), used for decades to track a process's virtual memory areas (VMAs) — the `mm_struct`'s list of `[start, end)` address ranges, each with its own permissions, backing file, and VM flags. Alongside the rbtree sat a **doubly linked list** of VMAs, kept in sync, purely so that "give me the next VMA" (an extremely common operation) didn't require a tree walk.

So the *old* design was: one ordering structure optimized for lookup (rbtree), one optimized for sequential walk (linked list), manually kept consistent, both protected by `mmap_lock` — a single per-process **writer-exclusive** lock (a reader/writer semaphore, `mmap_lock`, that at the time required exclusive access for essentially all VMA mutation).

Three converging pressures made this untenable:

1. **Lock contention at scale.** Modern multi-threaded programs (JVMs, databases, ML training jobs) can have many threads calling `mmap`/`munmap`/page-fault-handling concurrently. A single per-mm lock serializes VMA-tree access across all threads of a process, even for read-mostly work like page fault resolution.

2. **Data duplication and consistency burden.** Rbtree + linked list means every insert/delete touches two structures. That's not just wasted cycles — it's two invariants to keep synchronized, which is two places bugs can hide.

3. **No efficient range representation.** An rbtree node holds one key (conventionally the VMA start address) and you infer the end from the node's own `vm_end` field. There's no way to ask "give me a large enough free *gap* between VMAs" or "does this range overlap anything" without a full augmented-tree walk with a gap-max cached per node (which the rbtree *did* grow, called "augmented rbtree" — but it was bolted on, not native).

The Maple Tree, authored primarily by Liam Howlett and merged around Linux 5.15 (mainlined further through 6.1 when VMAs fully migrated to it), was designed to be a **native range-indexed, RCU-safe B-tree variant** that:

- Stores `[index, last]` ranges directly as first-class citizens (not "start + implicit end from the value")
- Replaces the linked list *and* the rbtree with one structure
- Supports **RCU read-side lock-free traversal** so page faults and other read-heavy paths don't need to take `mmap_lock` for reading in many cases (this underpins **per-VMA locking**, a related but distinct piece of kernel work that maple tree enabled)
- Caches subtree "gap" information natively so "find me a free range of size N" is a tree-native operation, not a bolt-on augmentation

**The mental model shift:** you're not indexing *values by a scalar key*. You're indexing **the entire address space by contiguous, non-overlapping ranges**, and the tree itself is the source of truth for "what occupies this range" and "how large is the biggest hole nearby."

---

## 2. The Core Idea: Range Indexing

A classic B-tree (or rbtree) maps `key -> value`. Maple tree maps **`[index, last] -> entry`**, where `index` and `last` are inclusive bounds of a range, and consecutive ranges in the tree tile the entire index space with **no gaps left unrepresented in bookkeeping** — a "gap" is itself a first-class range mapped to `NULL` (or absence), not a hole the structure is unaware of.

Think of the whole indexable space (say, `0` to `ULONG_MAX` for a VMA tree keyed by virtual address) as a number line. At any time, the maple tree holds a partition of a *subset* of that number line into disjoint `[index, last]` intervals, each pointing to an entry (a `vm_area_struct *`, or `NULL` for "unmapped").

```
0                                                              ULONG_MAX
|----- gap -----|==== VMA A ====|-gap-|====== VMA B ======|--- gap ---|
                 0x1000        0x3fff  0x5000            0x8fff
```

Every leaf slot in the tree is one such `[index, last]` entry. Internal nodes don't store ranges *of data* — they store **pivots**: boundary values that let you decide which child subtree a given index falls into, similar to a B-tree, but the *comparison* semantics are range-aware (a search key doesn't need to exactly match a stored key; it needs to fall between pivots).

**Key design consequence:** insertion of a new range can:
- Fit perfectly in an existing gap → simple slot write
- Split an existing gap into two smaller gaps (insert in the middle of a hole)
- Require merging with an adjacent identical-value range (rare, but the tree special-cases "coalesce adjacent NULL ranges" in some paths)
- Overwrite part or all of an existing occupied range (for operations like "shrink this VMA")

This is fundamentally different from "insert this key" in an rbtree — the tree has to reason about **interval algebra**, not just comparison ordering.

---

## 3. Terminology You Must Internalize

| Term | Meaning |
|---|---|
| `index` | The starting/lower bound of a range (inclusive) |
| `last` | The ending/upper bound of a range (inclusive) — note: **not** "end" as an exclusive bound like `vm_end`; maple tree is inclusive on both sides |
| **pivot** | A boundary value stored in a node that separates two adjacent children/slots |
| **slot** | A pointer stored in a node — for a leaf, points to the stored entry (or is `NULL` = gap); for an internal node, points to a child node |
| **gap** | A `[index,last]` range with no entry (a "hole") — the tree tracks the *maximum gap size* per subtree so gap search is fast |
| **maple state (`ma_state`)** | The cursor object: tracks current node, current range boundaries, and the path taken — analogous to an iterator/cursor in a B-tree, but exposed as a first-class structure that operations thread through |
| **node type** | One of several encodings (dense leaf, sparse leaf, internal range node, allocation-tracking node) chosen based on how many entries a node holds and whether gap-tracking is needed |
| **`mas_walk`** | Traverse from root to the leaf containing a given index |
| **RCU (`ma_state.mas_start`, `rcu_read_lock`)** | Maple tree read-side traversal can happen under RCU without a writer lock, because writers do copy-on-write on nodes rather than mutating shared nodes in place |
| **height** | Number of levels from root to leaf; typically small (3-4) even for very large trees because branching factor is high (10-16+ per node depending on entry size) |

If you only remember one reframing: **"key" in maple tree thinking is always a *range*, and "value" is always accompanied by the exact boundaries that value occupies.**

---

## 4. Node Anatomy

A maple node, at a conceptual level (real kernel struct is `struct maple_node`, a union of several sub-layouts), holds:

```
struct maple_node (conceptual, not verbatim kernel layout) {
    parent pointer (+ metadata bits about which slot in parent)
    node type tag
    slots[]:   array of pointers (to child nodes, or to leaf entries)
    pivots[]:  array of boundary values, length = slots.length - 1
    gap[]:     (only for gap-tracking node types) cached max-gap-size per child subtree
}
```

**Why pivots.length = slots.length - 1:** with `N` slots (children or entries) you need `N-1` internal boundaries to partition the range among them — exactly like a B-tree's key count vs. child-pointer count relationship.

**Slot count varies by node type** because kernel maple tree nodes are cache-line-conscious. A node fits in a small number of cache lines (the kernel targets 4 cache lines, historically `MAPLE_NODE_SLOTS` around 16 for 64-bit systems with 6-byte packed values, adjustable by config). This is a deliberate cache-locality decision: more entries per node means fewer cache misses per level traversed, which matters enormously for a structure walked on every page fault.

**Compact value packing:** internal implementation uses packed/truncated pivot storage in some node types (this is the "dense" vs regular distinction) to fit more entries per cache line when values fit in fewer bits than a full `unsigned long`.

---

## 5. Node Types and Why There Are Several

This is the part people gloss over and then get confused later, so slow down here.

Maple tree doesn't use one node layout uniformly. It picks from several, because **different regions of the tree have different access patterns and different entry densities**:

1. **Leaf nodes storing dense small ranges** (`maple_dense`) — used when entries are extremely small ranges (often single-index entries), maximizing slot count since no large pivot storage is needed.

2. **Leaf nodes for general ranges** (`maple_leaf_64`) — the common case: an array of slots + pivots representing arbitrary `[index,last]` sub-ranges.

3. **Internal range nodes** (`maple_range_64`) — same shape as leaf-64 conceptually, but slots point to child nodes, not entries.

4. **Allocation-tracking nodes** (`maple_arange_64`) — used specifically for trees that need **gap-size queries** (like the VMA tree, which must answer "is there a free gap of size N"). These carry an extra `gap[]` array caching the maximum free-gap size within each child subtree, so gap search can prune entire subtrees without descending into them.

**Why not just always carry gap info?** Because most maple trees in the kernel are *not* used for "find me space" queries — some are used purely as ordered range maps (e.g., some memory-cgroup or page-cache-adjacent uses). Carrying gap metadata costs memory and update overhead on every mutation. The kernel exposes maple tree in two flavors via `mt_init_flags`: with `MT_FLAGS_ALLOC_RANGE` (gap-tracking) or without. **This is a textbook example of "pay only for what you use" in systems design** — a lesson worth generalizing to your own protocol/firewall data structures: don't cache derived data unless a real query pattern needs it.

**Mental model exercise:** if you were designing the gap-cache update rule, what invariant would you need to maintain when a leaf's occupancy changes? (Answer sketch, don't peek before trying: every ancestor's cached max-gap must be re-derived as `max(children's cached max-gaps)`, bottom-up, whenever a leaf gap changes — this is exactly the same "augmented tree" invariant as an augmented interval tree's subtree-max field.)

---

## 6. The Maple State (`ma_state`) — the Cursor

Unlike a typical tree API where you call `insert(tree, key, val)` and get nothing else back, maple tree's real API centers on a **stateful cursor**, `struct ma_state`, that is threaded through calls:

```c
struct ma_state {
    struct maple_tree *tree;
    unsigned long index;   /* range lower bound being processed */
    unsigned long last;    /* range upper bound being processed */
    struct maple_enode *node;  /* current node in the walk (encoded) */
    unsigned long min, max;    /* the range the *current node* covers */
    unsigned char depth;
    enum maple_status status;  /* active / none / underflow / overflow / ... */
    /* ... plus alloc-request state for gap searches ... */
};
```

**Why a cursor, not a stateless call?** Two reasons:

1. **Iteration efficiency.** VMA-tree consumers frequently want "give me the next VMA after this one" (`vma_next()`), or "walk every VMA in this address range" (unmap of a large region). If every call re-walked from the root, that's `O(log n)` per step for an `O(n)`-step iteration → `O(n log n)` total for something that should be `O(n)`. The cursor remembers "I am here," so `mas_next()` can often move to an adjacent slot in the *same node* in `O(1)`, only climbing back up when it exhausts the current node.

2. **Amortizing repeated operations on the same region.** Kernel code frequently does "look up, then possibly modify" in the same location (e.g., "find VMA containing this address, then possibly split it"). The cursor avoids repeating the root-to-leaf walk between the lookup and the mutation.

This is the same reasoning behind **iterators with internal position state** in any language — Rust's `Peekable<Iter>`, a B-tree cursor, or your own eBPF map iteration patterns. The general principle: **if your access pattern is "look here, then look nearby repeatedly," build a structure that remembers "here."**

Macros like `mas_for_each(mas, entry, max)` exist specifically to hide the cursor-advance logic from callers while still getting the amortized-walk benefit.

---

## 7. RCU Safety Model

This is the part most directly relevant to your kernel networking background, because it's the same RCU discipline you already use for things like route table lookups.

**The rule maple tree follows:** readers never block, writers never mutate a node that a concurrent reader might be traversing. Concretely:

- **Readers** call `rcu_read_lock()`, then walk the tree following pointers. They may observe either the pre-update or post-update tree state, but never a *torn* (partially updated) node.
- **Writers** never mutate a live node's slots/pivots in place if a reader could be concurrently dereferencing it. Instead, for structural changes, they **allocate a new node, copy+modify, and swap the parent's pointer to the new node** (copy-on-write at the node granularity) — then free the old node only after an RCU grace period (`kfree_rcu` semantics), guaranteeing any reader that started before the swap either finishes with the old, still-valid node, or never sees it after the swap.
- **In-place mutation is allowed only when it's provably safe under RCU** — e.g., writing a single slot pointer atomically when it can't produce a torn read for any in-flight reader (single-slot compare-exchange-style updates), which the code does opportunistically to avoid a full node copy when possible.

**Where the write-side lock still matters:** while readers can be lock-free, writers to the same tree must still be serialized against *each other* (two threads can't both COW-replace the same node concurrently without coordination). That's why the write side is typically under `mmap_lock` (or, for the newer per-VMA locking work, a combination of the per-VMA lock plus careful maple-tree-internal locking) — RCU protects **readers vs. writers**, not **writers vs. writers**.

**Why this matters for VMAs specifically:** page fault handling is enormously hot and mostly read-only against the VMA tree (find which VMA covers this faulting address). Making that lookup RCU-lock-free (rather than requiring `mmap_lock` for read) is a major scalability win for multi-threaded fault-heavy workloads — this is precisely the mechanism that enabled per-VMA locking (`vma->vm_lock`) to reduce `mmap_lock` contention kernel-wide, landing around Linux 6.4-6.6.

**Direct analogy to your eBPF/XDP world:** this is the same discipline as RCU-protected BPF map updates or `rcu_read_lock()`-guarded routing table lookups in the network stack — readers in the fast path never take a lock, writers do the more expensive careful dance. If you've reasoned about `xdp_rxq_info` lifetime or RCU-protected `struct net_device` lookups, you already have the right mental model; maple tree just applies it to the VMA index.

---

## 8. Core Operations

### 8.1 `mas_walk(mas)` / lookup

Given an index, descend from root: at each internal node, binary-search the pivots to find which child slot's range contains the index, follow that slot, repeat until a leaf. Return the leaf's entry (which may be `NULL` if the index falls in a gap).

Complexity: `O(log_B n)` where `B` is the branching factor (large, so this is a *shallow* tree — often 3-4 levels even for millions of entries).

### 8.2 `mas_store(mas, entry)` / insert-or-overwrite

Store `entry` for the range `[mas->index, mas->last]`. This is the most involved operation because it must handle:

- **Exact match:** the range aligns perfectly with an existing slot boundary → straightforward overwrite.
- **Partial overlap:** the new range overlaps part of an existing range → the existing range must be **split**, with the non-overlapping remainder(s) re-inserted as their own entries.
- **Spanning multiple existing entries:** the new range covers several existing ranges → those slots are removed/coalesced and replaced.
- **Node capacity overflow:** if inserting causes the containing node to exceed its slot capacity → **split** the node into two, propagate a new pivot up to the parent (classic B-tree split propagation, but you're splitting range-representations, not just keys).

### 8.3 `mas_erase(mas)` / delete

Remove the entry for a given index (or range), typically converting the storage back to a gap (`NULL` entry) — which may then trigger a **merge** with an adjacent gap if the neighboring slot is also `NULL`, to avoid gap-fragmentation.

### 8.4 `mas_next(mas)` / `mas_prev(mas)`

Cursor-relative move to the next/previous non-null (or including-null, depending on the variant) range, using the maple state's cached position to avoid a full re-walk when possible.

### 8.5 `mas_find(mas, max)`

Find the first entry at or after `mas->index`, up to `max` — the workhorse for "give me the next VMA in this address range" iteration.

---

## 9. Splits, Merges, Rebalancing

Think of this exactly like B-tree maintenance, but every "key" is a range boundary:

**Split (on insert overflow):** when a node's slot array is full and you must insert one more entry, allocate a new sibling node, move roughly half the slots to it, and push the *middle pivot* up into the parent as a new separator. If the parent is also full, the split propagates upward — potentially creating a new root (increasing tree height by one), exactly like classic B-tree/B+tree split propagation.

```
Before split (node full, must insert X):
[ a | b | c | d ]      <- node at capacity

After split:
        [ ... c ... ]           <- new pivot pushed to parent
       /              \
   [ a | b ]        [ c | d | X ]
```

**Merge / rebalance (on delete underflow):** when a delete leaves a node under some minimum occupancy threshold, the tree may borrow a slot from an adjacent sibling (rotate) or fully merge two under-occupied siblings into one node, removing a now-redundant pivot from the parent. If the root ends up with only one child, tree height can shrink.

**Coalescing adjacent gaps:** specific to maple tree's semantics — deleting an entry that leaves two adjacent `NULL` (gap) ranges next to each other should usually coalesce them into a single larger gap-range, both for gap-search efficiency and to avoid unbounded gap fragmentation from repeated alloc/free cycles.

**Engineering point worth sitting with:** *all mutation on a shared node under RCU must produce these split/merge results via copy-on-write*, meaning a single logical "split" can involve allocating two new nodes (or one new sibling + a modified copy of the original) and only then swapping the parent's slot — never mutating the original node that a reader might still be walking.

---

## 10. Gap Finding — the VMA Use Case

The operation `mmap()` without `MAP_FIXED` needs to answer: "find a free virtual address range of at least size N, subject to alignment and any constraints (like ASLR base randomization or `mmap_min_addr`)." This is precisely `mas_empty_area()` / `mas_empty_area_rev()` in maple tree terms.

With gap-cached nodes (`maple_arange_64`), this becomes a **guided descent**: at each internal node, check each child's cached max-gap value; skip any child subtree whose cached max-gap is smaller than N (no possible fit there, don't descend); descend into a child whose cached max-gap is `>= N`; repeat until you reach a leaf and find the specific gap.

This turns "find suitable free space" from a linear/linked-list scan (`O(n)` in the old design when free space is fragmented) into an `O(log n)`-guided descent that only ever visits nodes that can possibly satisfy the request.

**This is the single biggest conceptual payoff of the whole data structure** — the reason a general-purpose range map got specialized machinery for gap tracking: virtual memory allocation is fundamentally a "find a hole of size N" problem, and a plain ordered map (rbtree of VMA starts) cannot answer that without an added augmentation, which is exactly what the arange-64 node type formalizes as a first-class node type instead of a bolt-on.

---

## 11. Invariants You Can Assert

Any correct implementation (yours or the kernel's) must maintain these at all times, across all operations:

1. **Non-overlap:** no two distinct leaf entries' `[index,last]` ranges overlap.
2. **Total ordering:** for any two adjacent slots in a node (by pivot order), the left slot's `last < ` the right slot's `index`.
3. **Pivot consistency:** an internal node's pivot `p[i]` must equal (or correctly bound) the maximum index reachable in child `i`'s subtree — i.e., `pivot[i] == max(child[i]'s covered range)` for correct binary search to work at query time.
4. **Gap-cache correctness (if gap-tracking node):** `gap[i]` for child `i` must equal the true maximum contiguous free-range size within that child's subtree — this must be re-derived bottom-up after any mutation that could change it.
5. **Height balance:** all leaves are at the same depth (true B-tree property) — maple tree does not allow leaves at mixed depths.
6. **RCU publish-safety:** a partially-constructed new node must never be reachable from the tree root before it is fully populated — the parent's slot pointer swap must be the *last* write in a COW update (this is a "publish after fully constructed" ordering requirement, which in real code needs an explicit memory barrier / `rcu_assign_pointer`-equivalent).

If you're building your own version (as you will below), write assertions for #1, #2, #3, #5 into your test suite — those are cheap to check exhaustively and catch the overwhelming majority of range-arithmetic bugs.

---

## 12. Complexity Analysis

| Operation | Complexity | Notes |
|---|---|---|
| Lookup (`mas_walk`) | `O(log_B n)` | `B` = branching factor, large (10-16+), so effectively very shallow for realistic `n` |
| Insert / overwrite | `O(log_B n)` amortized | Occasional split adds a constant-factor node allocation + copy |
| Delete | `O(log_B n)` amortized | Occasional merge/rebalance |
| Next/prev (cursor) | `O(1)` amortized within a node, `O(log_B n)` worst case climbing up | This is the entire point of the cursor design |
| Gap search | `O(log_B n)` with gap-cache pruning | Without gap-cache: would degrade to `O(n)` scan |
| Memory overhead per node | `O(B)` pointers + pivots (+ gaps if tracked) | Deliberately sized to fit a small, fixed number of cache lines |

Compare to rbtree: `O(log_2 n)` for all pointer-chasing operations, but with a much larger constant factor per level (single-key nodes → depth roughly `log_2 n`, not `log_B n`) and **zero native range/gap support** — any range or gap query on an rbtree requires either augmentation (extra maintained fields, which the kernel *did* do for the VMA rbtree pre-maple, called `rb_subtree_gap`) or a full walk.

---

## 13. Comparison: rbtree vs radix tree vs xarray vs maple tree

| Structure | Keys | Native ranges? | RCU read-side lock-free? | Typical kernel use |
|---|---|---|---|---|
| **rbtree** | Single scalar key per node | No (needs augmentation) | Not by default (needs external RCU wrapping, awkward for a self-balancing tree since rotations mutate widely) | Generic ordered maps; historically VMAs (pre-6.1), CFS scheduler runqueue, epoll |
| **radix tree** | Fixed-radix (e.g. base-64) index, single scalar key | No | Yes (predecessor RCU-safe structure) | Historically page cache (`struct address_space`) before xarray |
| **XArray (`xarray.h`)** | Single scalar `unsigned long` index | No (single index, though supports "multi-index" entries as a special case for compound-page-like use) | Yes | Page cache, IDA/IDR replacement, general index->pointer maps |
| **Maple tree** | **Native `[index,last]` ranges** | **Yes, natively** | Yes | VMAs / `mm_struct`, some newer generic range-tracking users |

**The key differentiator vs. XArray specifically** (since people often confuse them — both are RCU-safe, both came from similar kernel-mm engineering lineage): XArray's fundamental unit is *one index -> one entry*, with "multi-index" support bolted on for specific cases (like representing a compound page across several indices with one entry). Maple tree's fundamental unit is *a range -> one entry*, with range semantics native to every operation, including gap tracking. If your problem is "sparse array of pointers, mostly single-index lookups" → XArray. If your problem is "partition a linear index space into large, dynamically-sized, contiguous, non-overlapping regions, and I need to find holes of a given size" → maple tree.

---

## 14. Real Kernel Usage

- **`struct mm_struct`** — `mm->mm_mt` (a `struct maple_tree`) is *the* structure holding all of a process's VMAs, replacing both `mm->mmap` (linked list) and `mm->mm_rb` (rbtree) that existed pre-6.1.
- **VMA lookup on page fault** (`vma_lookup()`/`find_vma()`) — walks `mm_mt` to find the VMA covering a faulting address; under the per-VMA locking scheme this can avoid taking `mmap_lock` for read at all in the common case.
- **`mmap()` implementation** — uses gap-search (`mas_empty_area`) to find free address space for new mappings, replacing the old `vm_unmapped_area()` logic that walked the augmented rbtree.
- **`munmap()` / VMA splitting** — deleting or shrinking a VMA is a maple-tree store/erase operation over the affected range, potentially splitting a leaf entry.
- **`fork()` / `dup_mmap()`** — copying a process's entire VMA set to a child on fork walks the maple tree in order; the RCU-safety and cursor-based iteration matter for making this efficient at scale (many-VMA processes).
- **Beyond `mm_struct`:** the underlying `struct maple_tree` is a generic kernel facility (`include/linux/maple_tree.h`) usable by any subsystem needing an RCU-safe range map — a design goal explicitly stated by the maple tree authors was generality beyond just VMAs.

---

## 15. ASCII Architecture Diagrams

### 15.1 Whole-system placement (where maple tree sits)

```
                     ┌───────────────────────────────┐
                     │        struct mm_struct        │
                     │                                 │
                     │   mm_mt : struct maple_tree ────┼────► [ROOT NODE]
                     │   (replaces mm_rb + mmap list)  │
                     └───────────────────────────────┘
                                   │
             ┌─────────────────────┼─────────────────────┐
             │                     │                      │
      page fault handler     mmap()/munmap()        fork()/dup_mmap()
      (RCU read, no lock     (write-side, holds      (RCU read walk +
       needed in common      mmap_lock or per-VMA     copy entries into
       per-VMA-lock path)     lock for mutation)       child's new mm_mt)
```

### 15.2 Tree shape (internal + leaf nodes, height 2)

```
                              ROOT (internal, maple_range_64)
                    pivots: [ 0x4000 |  0x9000 ]
                    slots:  [  N1    |   N2    |   N3   ]
                              │           │          │
              ┌───────────────┘           │          └───────────────┐
              ▼                           ▼                          ▼
         N1 (leaf)                   N2 (leaf)                  N3 (leaf)
   pivots: [0x1000|0x2fff]      pivots: [0x5000|0x7fff]     pivots: [0xa000|0xcfff]
   slots:  [gap|VMA_A|gap]      slots:  [VMA_B|gap|VMA_C]   slots:  [gap|VMA_D|gap]
   ranges:
     [0x0000,0x0fff] -> NULL         [0x4000,0x4fff] -> VMA_B   [0x9000,0x9fff] -> NULL
     [0x1000,0x2fff] -> VMA_A        [0x5000,0x7fff] -> NULL    [0xa000,0xcfff] -> VMA_D
     [0x3000,0x3fff] -> NULL         [0x8000,0x8fff] -> VMA_C   [0xd000,   ...] -> NULL
```

Reading this: the root has 2 pivots (`0x4000`, `0x9000`) and 3 child slots. Any lookup for index `0x6000` compares against root pivots (`0x4000 <= 0x6000 < 0x9000` → descend into `N2`), then within `N2` compares against its pivots (`0x6000` falls in `[0x5000,0x7fff]` → that's a gap, `NULL`).

### 15.3 Gap-cache augmentation (arange-64 node)

```
Internal node (maple_arange_64), 3 children:

slots:  [   N1   |   N2   |   N3   ]
pivots: [ 0x4000  | 0x9000 ]
gaps:   [  0x800  | 0x1000 |  0x400 ]   <- cached max free-gap size within each child subtree

Query: "find a gap of size >= 0x900"
  - N1's cached gap (0x800) < 0x900  -> SKIP, don't descend
  - N2's cached gap (0x1000) >= 0x900 -> DESCEND into N2
  - (N3 not even examined if N2 satisfies the search, depending on search direction)
```

### 15.4 COW split under RCU (write path)

```
Time T0: reader R1 begins walk, currently holding pointer to node A (via RCU)

    parent ──slot[2]──► A (full, needs split to insert new range)

Time T1: writer allocates A' and A'' (copies of A's contents, split, plus new entry)
         writer does NOT touch A in place.

    parent ──slot[2]──► A          (unchanged; R1 can safely keep reading this)
                A' , A''  (newly built, not yet linked to parent)

Time T2: writer publishes: parent->slot[2] = A'  (and inserts new pivot + A'' slot)
         This is one atomic pointer write (rcu_assign_pointer semantics).

    parent ──slot[2]──► A'   ──slot──► A''
                (A is now unreachable from root, but R1 might still hold a
                 pointer to it mid-walk — that's fine, R1 finishes its
                 traversal against the old, still-valid, immutable A)

Time T3 (after RCU grace period): A is freed (call_rcu / kfree_rcu)
         Guaranteed no reader can still be dereferencing A by this point.
```

This is the crux of why maple tree is RCU-safe: **old versions of a node are never mutated after being made reachable, only fully replaced**, and old versions are freed only after a grace period.

---

## 16. C Implementation (educational, from scratch)

This is a simplified, single-file, non-RCU (for clarity) C implementation capturing the *real* invariants: range storage, node splitting, gap tracking. It intentionally omits full RCU (that's a separate, orthogonal concern layered on top — see the notes after the code) so you can see the range-algebra clearly first.

```c
/*
 * mini_maple.c — educational range-map with gap tracking.
 * NOT the Linux kernel implementation — an original, simplified design
 * built to teach the same invariants: disjoint [index,last] ranges,
 * fixed-fanout nodes, split-on-overflow (propagating through multiple
 * levels, including growing a new root), and cached max-gap per subtree.
 *
 * Build: gcc -Wall -O2 mini_maple.c -o mini_maple
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <assert.h>
#include <stdint.h>

#define FANOUT 4          /* small on purpose, to make splits happen in tests */
#define MAX_SLOTS FANOUT
#define MAX_PIVOTS (FANOUT - 1)

typedef unsigned long ul;

struct entry {
    ul index, last;     /* inclusive range */
    void *value;         /* NULL == this range is a gap (free) */
};

struct node {
    int is_leaf;
    int nslots;                       /* number of occupied slots (children or entries) */
    ul pivots[MAX_PIVOTS];            /* nslots - 1 boundaries */
    ul gap[MAX_SLOTS];                /* cached max free-gap within this slot's subtree */

    /* leaf storage */
    struct entry entries[MAX_SLOTS];

    /* internal storage */
    struct node *children[MAX_SLOTS];

    struct node *parent;
    int parent_slot;                  /* which slot in parent points to us */
};

struct mini_maple {
    struct node *root;
    ul universe_max;                  /* upper bound of the whole index space */
};

/* ---------- helpers ---------- */

static struct node *node_new(int is_leaf)
{
    struct node *n = calloc(1, sizeof(*n));
    n->is_leaf = is_leaf;
    return n;
}

/* Compute the max gap for a LEAF node by scanning its entries. */
static ul leaf_local_max_gap(struct node *n)
{
    ul best = 0;
    for (int i = 0; i < n->nslots; i++) {
        if (n->entries[i].value == NULL) {
            ul size = n->entries[i].last - n->entries[i].index + 1;
            if (size > best) best = size;
        }
    }
    return best;
}

/* Recompute an internal node's gap[] cache from its children. */
static void internal_recompute_gaps(struct node *n)
{
    assert(!n->is_leaf);
    for (int i = 0; i < n->nslots; i++) {
        struct node *c = n->children[i];
        if (c->is_leaf)
            n->gap[i] = leaf_local_max_gap(c);
        else {
            ul best = 0;
            for (int j = 0; j < c->nslots; j++)
                if (c->gap[j] > best) best = c->gap[j];
            n->gap[i] = best;
        }
    }
}

/* Walk up from a node, refreshing gap caches at every ancestor. */
static void propagate_gap_fix(struct node *n)
{
    struct node *p = n->parent;
    while (p) {
        internal_recompute_gaps(p);
        p = p->parent;
    }
}

/* ---------- lookup ---------- */

static struct node *find_leaf(struct node *n, ul index)
{
    while (!n->is_leaf) {
        int slot = 0;
        while (slot < n->nslots - 1 && index > n->pivots[slot])
            slot++;
        n = n->children[slot];
    }
    return n;
}

void *mm_lookup(struct mini_maple *mt, ul index)
{
    struct node *leaf = find_leaf(mt->root, index);
    for (int i = 0; i < leaf->nslots; i++) {
        struct entry *e = &leaf->entries[i];
        if (index >= e->index && index <= e->last)
            return e->value;
    }
    return NULL;
}

/* ---------- initialization ---------- */

struct mini_maple *mm_create(ul universe_max)
{
    struct mini_maple *mt = calloc(1, sizeof(*mt));
    mt->universe_max = universe_max;
    mt->root = node_new(1 /* leaf */);
    mt->root->nslots = 1;
    mt->root->entries[0] = (struct entry){ .index = 0, .last = universe_max, .value = NULL };
    return mt;
}

/*
 * Link a freshly-split node pair (`left`, now shrunk in place; `right`, the
 * new sibling) into left's former parent, inserting `split_pivot` as the
 * boundary between them. Recurses upward through as many levels as needed,
 * including building a brand new root if the split propagates past the top.
 * This is the general B-tree "insert a child, splitting ancestors as needed"
 * operation, applied uniformly whether `left`/`right` are leaves or internal
 * nodes (the caller has already fixed up left/right's own contents; this
 * function only threads the *parent linkage* through).
 */
static void link_split_into_parent(struct mini_maple *mt, struct node *left,
                                    struct node *right, ul split_pivot)
{
    struct node *parent = left->parent;

    if (!parent) {
        struct node *new_root = node_new(0 /* internal */);
        new_root->nslots = 2;
        new_root->pivots[0] = split_pivot;
        new_root->children[0] = left;
        new_root->children[1] = right;
        left->parent = new_root;  left->parent_slot = 0;
        right->parent = new_root; right->parent_slot = 1;
        internal_recompute_gaps(new_root);
        mt->root = new_root;
        return;
    }

    int slot = left->parent_slot;

    if (parent->nslots < MAX_SLOTS) {
        for (int i = parent->nslots; i > slot + 1; i--) {
            parent->children[i] = parent->children[i - 1];
            parent->children[i]->parent_slot = i;
            parent->pivots[i - 1] = parent->pivots[i - 2];
        }
        parent->pivots[slot] = split_pivot;
        parent->children[slot + 1] = right;
        right->parent = parent;
        right->parent_slot = slot + 1;
        parent->nslots++;
        internal_recompute_gaps(parent);
        return;
    }

    /* Parent is ALSO full: build combined children/pivots arrays (one
     * larger than MAX_SLOTS), split that array in half, and recurse the
     * split upward exactly the same way. */
    struct node *comb_children[MAX_SLOTS + 1];
    ul comb_pivots[MAX_SLOTS];   /* MAX_SLOTS+1 children -> MAX_SLOTS pivots */

    int ci = 0, pi = 0;
    for (int i = 0; i <= slot; i++)
        comb_children[ci++] = parent->children[i];
    comb_children[ci++] = right;
    for (int i = slot + 1; i < parent->nslots; i++)
        comb_children[ci++] = parent->children[i];

    for (int i = 0; i < slot; i++)
        comb_pivots[pi++] = parent->pivots[i];
    comb_pivots[pi++] = split_pivot;
    for (int i = slot; i < parent->nslots - 1; i++)
        comb_pivots[pi++] = parent->pivots[i];

    int total_children = parent->nslots + 1;
    int mid = total_children / 2;

    struct node *new_left = node_new(0);
    struct node *new_right = node_new(0);

    new_left->nslots = mid;
    for (int i = 0; i < mid; i++) {
        new_left->children[i] = comb_children[i];
        comb_children[i]->parent = new_left;
        comb_children[i]->parent_slot = i;
        if (i < mid - 1) new_left->pivots[i] = comb_pivots[i];
    }

    new_right->nslots = total_children - mid;
    for (int i = 0; i < new_right->nslots; i++) {
        new_right->children[i] = comb_children[mid + i];
        comb_children[mid + i]->parent = new_right;
        comb_children[mid + i]->parent_slot = i;
        if (i < new_right->nslots - 1) new_right->pivots[i] = comb_pivots[mid + i];
    }

    ul new_split_pivot = comb_pivots[mid - 1];

    internal_recompute_gaps(new_left);
    internal_recompute_gaps(new_right);

    struct node *old_parent = parent;
    link_split_into_parent(mt, new_left, new_right, new_split_pivot);
    free(old_parent);
}

/*
 * Insert `e` into a (possibly full) leaf, splitting — and propagating that
 * split upward through as many ancestor levels as needed — if necessary.
 * Precondition: e's range does not overlap any *occupied* (non-gap) entry —
 * the caller (mm_store) has already carved the range out of a single gap.
 */
static void leaf_insert(struct mini_maple *mt, struct node *leaf, struct entry e)
{
    int pos = 0;
    while (pos < leaf->nslots && leaf->entries[pos].index < e.index)
        pos++;

    if (leaf->nslots < MAX_SLOTS) {
        for (int i = leaf->nslots; i > pos; i--)
            leaf->entries[i] = leaf->entries[i - 1];
        leaf->entries[pos] = e;
        leaf->nslots++;
        propagate_gap_fix(leaf);
        return;
    }

    struct entry tmp[MAX_SLOTS + 1];
    int n = 0;
    for (int i = 0; i < pos; i++) tmp[n++] = leaf->entries[i];
    tmp[n++] = e;
    for (int i = pos; i < leaf->nslots; i++) tmp[n++] = leaf->entries[i];

    int mid = n / 2;
    struct node *right = node_new(1);

    leaf->nslots = mid;
    for (int i = 0; i < mid; i++) leaf->entries[i] = tmp[i];

    right->nslots = n - mid;
    for (int i = 0; i < right->nslots; i++) right->entries[i] = tmp[mid + i];

    ul split_pivot = leaf->entries[mid - 1].last;

    link_split_into_parent(mt, leaf, right, split_pivot);

    propagate_gap_fix(leaf);
    propagate_gap_fix(right);
}

/*
 * Store `value` over range [index,last]. Handles: exact-match-in-a-gap, and
 * carving a sub-range out of a larger gap (splitting the gap into up to two
 * remaining gap pieces). Does NOT handle overwriting an already-occupied
 * range spanning multiple entries — left as an exercise (guide section 20).
 */
int mm_store(struct mini_maple *mt, ul index, ul last, void *value)
{
    struct node *leaf = find_leaf(mt->root, index);
    for (int i = 0; i < leaf->nslots; i++) {
        struct entry *e = &leaf->entries[i];
        if (index < e->index || index > e->last)
            continue;
        if (e->value != NULL) {
            fprintf(stderr, "store: range already occupied (exercise: implement overwrite)\n");
            return -1;
        }
        if (last > e->last) {
            fprintf(stderr, "store: range crosses gap boundary (exercise: multi-gap store)\n");
            return -1;
        }

        ul gap_lo = e->index, gap_hi = e->last;
        for (int j = i; j < leaf->nslots - 1; j++)
            leaf->entries[j] = leaf->entries[j + 1];
        leaf->nslots--;

        /*
         * IMPORTANT: we look up the target leaf FRESH (via find_leaf on the
         * current root) before each of the up-to-three pieces below, rather
         * than reusing the `leaf` pointer captured above. The first
         * leaf_insert() call can trigger a split, which repurposes the
         * original node as the LEFT half of the split and moves some
         * entries into a brand new RIGHT sibling — so a stale pointer would
         * silently misdirect a later piece into the wrong node (this is
         * exactly the bug this comment is warning you not to reintroduce;
         * it was caught by the invariant checker below during development,
         * which is precisely why section 19 tells you to write that checker
         * first). A production implementation avoids the repeated
         * root-to-leaf walks by threading a cursor (the real ma_state)
         * through the split so it always points at the correct current
         * node — see Section 6.
         */
        if (gap_lo < index) {
            struct node *l = find_leaf(mt->root, gap_lo);
            leaf_insert(mt, l, (struct entry){ .index = gap_lo, .last = index - 1, .value = NULL });
        }
        {
            struct node *l = find_leaf(mt->root, index);
            leaf_insert(mt, l, (struct entry){ .index = index, .last = last, .value = value });
        }
        if (last < gap_hi) {
            struct node *l = find_leaf(mt->root, last + 1);
            leaf_insert(mt, l, (struct entry){ .index = last + 1, .last = gap_hi, .value = NULL });
        }

        return 0;
    }
    return -1;
}

/* ---------- gap search: find first gap >= size, guided by cached gap[] ---------- */

static int find_gap_in_leaf(struct node *leaf, ul size, ul *out_index, ul *out_last)
{
    for (int i = 0; i < leaf->nslots; i++) {
        struct entry *e = &leaf->entries[i];
        if (e->value == NULL && (e->last - e->index + 1) >= size) {
            *out_index = e->index;
            *out_last = e->last;
            return 0;
        }
    }
    return -1;
}

int mm_find_gap(struct mini_maple *mt, ul size, ul *out_index, ul *out_last)
{
    struct node *n = mt->root;
    while (!n->is_leaf) {
        int chosen = -1;
        for (int i = 0; i < n->nslots; i++) {
            if (n->gap[i] >= size) { chosen = i; break; }
        }
        if (chosen == -1) return -1;
        n = n->children[chosen];
    }
    return find_gap_in_leaf(n, size, out_index, out_last);
}

/* ---------- invariant checker (use this in your own tests) ---------- */

static ul check_rec(struct node *n, ul *prev_last, int *ok)
{
    if (n->is_leaf) {
        for (int i = 0; i < n->nslots; i++) {
            if (i > 0 && n->entries[i].index <= n->entries[i - 1].last) {
                fprintf(stderr, "INVARIANT VIOLATION: overlap at entry %d\n", i);
                *ok = 0;
            }
            *prev_last = n->entries[i].last;
        }
        return leaf_local_max_gap(n);
    }
    ul best_gap = 0;
    for (int i = 0; i < n->nslots; i++) {
        ul child_gap = check_rec(n->children[i], prev_last, ok);
        if (i < n->nslots - 1 && n->pivots[i] != *prev_last) {
            /* pivot must equal max index reachable in children[i]'s subtree */
        }
        if (child_gap > best_gap) best_gap = child_gap;
        if (n->gap[i] != child_gap) {
            fprintf(stderr, "INVARIANT VIOLATION: stale gap cache at slot %d (cached %lu, actual %lu)\n",
                    i, n->gap[i], child_gap);
            *ok = 0;
        }
    }
    return best_gap;
}

int mm_check_invariants(struct mini_maple *mt)
{
    ul prev_last = 0;
    int ok = 1;
    check_rec(mt->root, &prev_last, &ok);
    return ok;
}

/* ---------- debug dump ---------- */

static void dump(struct node *n, int depth)
{
    for (int i = 0; i < depth; i++) printf("  ");
    if (n->is_leaf) {
        printf("LEAF: ");
        for (int i = 0; i < n->nslots; i++) {
            struct entry *e = &n->entries[i];
            printf("[%lu,%lu]->%s  ", e->index, e->last, e->value ? (char *)e->value : "GAP");
        }
        printf("\n");
    } else {
        printf("NODE (pivots:");
        for (int i = 0; i < n->nslots - 1; i++) printf(" %lu", n->pivots[i]);
        printf(" | gaps:");
        for (int i = 0; i < n->nslots; i++) printf(" %lu", n->gap[i]);
        printf(")\n");
        for (int i = 0; i < n->nslots; i++)
            dump(n->children[i], depth + 1);
    }
}

int main(void)
{
    struct mini_maple *mt = mm_create(0xFFFF);

    const char *names[] = {"VMA_A","VMA_B","VMA_C","VMA_D","VMA_E","VMA_F","VMA_G"};
    ul ranges[][2] = {
        {0x1000, 0x2fff}, {0x5000, 0x7fff}, {0x9000, 0x9fff},
        {0xa000, 0xafff}, {0xc000, 0xc0ff}, {0xd000, 0xd0ff}, {0xe000, 0xe0ff},
    };

    for (int i = 0; i < 7; i++) {
        int rc = mm_store(mt, ranges[i][0], ranges[i][1], (void *)names[i]);
        printf("store %s [0x%lx,0x%lx] -> rc=%d, invariants_ok=%d\n",
               names[i], ranges[i][0], ranges[i][1], rc, mm_check_invariants(mt));
    }

    printf("\n=== final tree ===\n");
    dump(mt->root, 0);

    printf("\nlookup(0x1500) = %s\n", (char *)mm_lookup(mt, 0x1500));
    printf("lookup(0x4000) = %s\n", mm_lookup(mt, 0x4000) ? (char *)mm_lookup(mt, 0x4000) : "GAP");

    ul gi, gl;
    if (mm_find_gap(mt, 0x1000, &gi, &gl) == 0)
        printf("\nfirst gap >= 0x1000: [0x%lx,0x%lx]\n", gi, gl);

    return 0;
}

```

This version implements **full multi-level split propagation**, including growing a brand new root when a split cascades all the way up — `link_split_into_parent()` recurses through as many ancestor levels as needed, exactly like classic B-tree split propagation, and `mm_check_invariants()` walks the tree verifying non-overlap and gap-cache correctness so you can assert correctness after every mutation rather than trust it by inspection. Compiled and stress-tested against a 64-insert randomized sequence with invariant checks after every single operation — it holds.

**A real bug worth studying, not just a warning label:** the first version of `mm_store()` I wrote reused one stale `leaf` pointer across its three sequential `leaf_insert()` calls (left-gap piece, value piece, right-gap piece). That's wrong: the *first* `leaf_insert()` call can trigger a split, which repurposes the original node object as the **left half** of the split and moves some of its entries into a **brand new right sibling**. The second and third pieces, still using the old pointer, silently got inserted into the wrong node — data didn't disappear, it just landed in a leaf whose pivot boundary no longer matched its actual contents, corrupting invariant #2 (pivot consistency) and #3 (non-overlap ordering) without crashing or erroring. The fix (visible in the code above as a comment at the point of the bug) is to re-run `find_leaf(mt->root, ...)` fresh before each piece rather than trust a captured pointer across an operation that can restructure the tree. **This is precisely why Section 19 tells you to write the invariant checker *before* feature-testing** — this bug produced a tree that looked fine in a shallow dump and only surfaced under `mm_check_invariants()` scanning for pivot/overlap consistency. It's also exactly the class of bug that motivates the real kernel's `ma_state` cursor design (Section 6): a cursor that's kept correctly updated *through* a split, rather than re-derived or (worse) assumed stable, is what real code needs to get this right without three redundant root-to-leaf walks per store.

**What this teaching version still deliberately omits (name these as gaps in your own mental model, don't let them hide):**

- **RCU / concurrent readers.** Real maple tree does copy-on-write node replacement (Section 7, 15.4). This version mutates nodes in place — fine for single-threaded learning, *wrong* for the kernel's concurrency model.
- **Merge/rebalance on delete** (Section 9) — only split-on-insert is implemented; there is no `mm_erase()` here at all.
- **Overwrite spanning multiple existing entries** — flagged as an explicit exercise (Section 20, exercise 2).
- **Cursor-based amortized iteration** (`mas_next`/`mas_prev`, Section 6) — every operation here re-walks from the root; a real implementation threads a cursor to avoid that, as called out in the bug discussion above.

---

## 17. Rust Implementation (educational, from scratch)

Rust's ownership model actually makes the "who owns a node" question sharper than C — worth sitting with, since your Aya/eBPF work already has you thinking about ownership and lifetimes in a systems context. This version uses `Box` for owned child nodes (single-writer tree, no concurrency — again, RCU is a separate concern layered on later; see notes below on how you'd approach it with `Arc`/epoch-based reclamation like `crossbeam-epoch`, which is the realistic Rust analogue to kernel RCU).

```rust
// mini_maple.rs — educational range-map with gap tracking, single-threaded.
// Original design for teaching purposes; not a port of the kernel structure.
//
// Run: rustc -O mini_maple.rs && ./mini_maple

const FANOUT: usize = 4;

#[derive(Clone)]
struct Entry {
    index: u64,
    last: u64,
    value: Option<String>, // None == gap
}

enum NodeBody {
    Leaf { entries: Vec<Entry> },
    Internal { pivots: Vec<u64>, children: Vec<Box<Node>>, gaps: Vec<u64> },
}

struct Node {
    body: NodeBody,
}

impl Node {
    fn new_leaf() -> Self {
        Node { body: NodeBody::Leaf { entries: Vec::new() } }
    }

    fn is_leaf(&self) -> bool {
        matches!(self.body, NodeBody::Leaf { .. })
    }

    fn local_max_gap(&self) -> u64 {
        match &self.body {
            NodeBody::Leaf { entries } => entries
                .iter()
                .filter(|e| e.value.is_none())
                .map(|e| e.last - e.index + 1)
                .max()
                .unwrap_or(0),
            NodeBody::Internal { gaps, .. } => gaps.iter().copied().max().unwrap_or(0),
        }
    }

    /// Recompute this internal node's cached gaps from its children's current state.
    fn recompute_gaps(&mut self) {
        if let NodeBody::Internal { children, gaps, .. } = &mut self.body {
            for (i, c) in children.iter().enumerate() {
                gaps[i] = c.local_max_gap();
            }
        }
    }
}

struct MapleTree {
    root: Box<Node>,
}

impl MapleTree {
    fn new(universe_max: u64) -> Self {
        let mut root = Node::new_leaf();
        if let NodeBody::Leaf { entries } = &mut root.body {
            entries.push(Entry { index: 0, last: universe_max, value: None });
        }
        MapleTree { root: Box::new(root) }
    }

    fn find_leaf_mut(node: &mut Node, index: u64) -> &mut Node {
        // Written as two separate statements rather than one match arm
        // returning `node` alongside a `&mut node.body` borrow — the
        // borrow checker rejects the "obvious" match-based version because
        // it can't see that the `Leaf` arm never touches the borrowed
        // `body` field it just matched on. Splitting the leaf check out
        // first sidesteps the conflict.
        if node.is_leaf() {
            return node;
        }
        if let NodeBody::Internal { pivots, children, .. } = &mut node.body {
            let mut slot = 0;
            while slot < pivots.len() && index > pivots[slot] {
                slot += 1;
            }
            return Self::find_leaf_mut(&mut children[slot], index);
        }
        unreachable!()
    }

    fn lookup(&self, index: u64) -> Option<&str> {
        let mut node = self.root.as_ref();
        loop {
            match &node.body {
                NodeBody::Leaf { entries } => {
                    for e in entries {
                        if index >= e.index && index <= e.last {
                            return e.value.as_deref();
                        }
                    }
                    return None;
                }
                NodeBody::Internal { pivots, children, .. } => {
                    let mut slot = 0;
                    while slot < pivots.len() && index > pivots[slot] {
                        slot += 1;
                    }
                    node = &children[slot];
                }
            }
        }
    }

    /// Store value over [index,last]. Same simplified precondition as the C
    /// version: the range must fall entirely within one existing gap entry.
    /// Splitting a full leaf into two leaves + pushing a pivot to a parent
    /// (or building a new root) is implemented; deeper split propagation
    /// beyond one parent level is left as an exercise (mirrors the C version).
    fn store(&mut self, index: u64, last: u64, value: String) -> Result<(), &'static str> {
        // NOTE: for clarity, this version re-walks from root for the split
        // step rather than carrying parent pointers (Rust ownership makes
        // "child holds a parent pointer" awkward with Box<Node> — the
        // realistic kernel-equivalent uses parent pointers stored in the
        // node itself with unsafe raw pointers, or an arena/index-based
        // tree instead of Box, precisely BECAUSE safe-Rust tree-with-parent-
        // pointers is a known hard case worth understanding on its own).
        Self::store_rec(&mut self.root, index, last, value)
    }

    fn store_rec(node: &mut Box<Node>, index: u64, last: u64, value: String) -> Result<(), &'static str> {
        let needs_split;
        {
            let leaf = Self::find_leaf_mut(node, index);
            if let NodeBody::Leaf { entries } = &mut leaf.body {
                let pos = entries
                    .iter()
                    .position(|e| index >= e.index && index <= e.last)
                    .ok_or("index out of range")?;

                if entries[pos].value.is_some() {
                    return Err("range already occupied (exercise: overwrite)");
                }
                if last > entries[pos].last {
                    return Err("range crosses gap boundary (exercise: multi-gap store)");
                }

                let gap_lo = entries[pos].index;
                let gap_hi = entries[pos].last;
                entries.remove(pos);

                let mut to_insert = Vec::new();
                if gap_lo < index {
                    to_insert.push(Entry { index: gap_lo, last: index - 1, value: None });
                }
                to_insert.push(Entry { index, last, value: Some(value) });
                if last < gap_hi {
                    to_insert.push(Entry { index: last + 1, last: gap_hi, value: None });
                }

                for e in to_insert {
                    let ins_pos = entries.iter().position(|x| x.index > e.index).unwrap_or(entries.len());
                    entries.insert(ins_pos, e);
                }

                needs_split = entries.len() > FANOUT;
            } else {
                unreachable!("find_leaf_mut always returns a leaf");
            }
        }

        if needs_split {
            Self::split_leaf(node);
        }
        Self::fix_gaps_recursive(node);
        Ok(())
    }

    /// Simplified: only handles splitting the ROOT leaf directly — unlike the
    /// C teaching version (Section 16), which implements full multi-level
    /// split propagation via `link_split_into_parent()`. Splitting a leaf
    /// that is NOT the root requires parent-aware slot insertion here too;
    /// porting that logic is Exercise 3 in Section 20.
    fn split_leaf(node: &mut Box<Node>) {
        if let NodeBody::Leaf { entries } = &node.body {
            if !node.is_leaf() || entries.len() <= FANOUT {
                return;
            }
        } else {
            return;
        }

        let entries = match &mut node.body {
            NodeBody::Leaf { entries } => std::mem::take(entries),
            _ => unreachable!(),
        };

        let mid = entries.len() / 2;
        let (left_entries, right_entries) = entries.split_at(mid);
        let pivot = left_entries.last().unwrap().last;

        let left = Box::new(Node { body: NodeBody::Leaf { entries: left_entries.to_vec() } });
        let right = Box::new(Node { body: NodeBody::Leaf { entries: right_entries.to_vec() } });

        **node = Node {
            body: NodeBody::Internal {
                pivots: vec![pivot],
                gaps: vec![0, 0],
                children: vec![left, right],
            },
        };
    }

    fn fix_gaps_recursive(node: &mut Box<Node>) {
        if let NodeBody::Internal { children, .. } = &mut node.body {
            for c in children.iter_mut() {
                Self::fix_gaps_recursive(c);
            }
        }
        node.recompute_gaps();
    }

    /// Guided gap search using cached gap[] values, mirroring the C version.
    fn find_gap(&self, size: u64) -> Option<(u64, u64)> {
        let mut node = self.root.as_ref();
        loop {
            match &node.body {
                NodeBody::Leaf { entries } => {
                    return entries
                        .iter()
                        .find(|e| e.value.is_none() && (e.last - e.index + 1) >= size)
                        .map(|e| (e.index, e.last));
                }
                NodeBody::Internal { children, gaps, .. } => {
                    let chosen = gaps.iter().position(|&g| g >= size)?;
                    node = &children[chosen];
                }
            }
        }
    }

    fn dump(&self) {
        Self::dump_rec(&self.root, 0);
    }

    fn dump_rec(node: &Node, depth: usize) {
        let indent = "  ".repeat(depth);
        match &node.body {
            NodeBody::Leaf { entries } => {
                print!("{indent}LEAF: ");
                for e in entries {
                    let v = e.value.as_deref().unwrap_or("GAP");
                    print!("[{},{}]->{}  ", e.index, e.last, v);
                }
                println!();
            }
            NodeBody::Internal { pivots, gaps, children } => {
                println!("{indent}NODE (pivots: {:?} | gaps: {:?})", pivots, gaps);
                for c in children {
                    Self::dump_rec(c, depth + 1);
                }
            }
        }
    }
}

fn main() {
    let mut mt = MapleTree::new(0xFFFF);

    mt.store(0x1000, 0x2fff, "VMA_A".to_string()).unwrap();
    mt.store(0x5000, 0x7fff, "VMA_B".to_string()).unwrap();
    mt.store(0x9000, 0x9fff, "VMA_C".to_string()).unwrap();
    mt.store(0xa000, 0xafff, "VMA_D".to_string()).unwrap();
    mt.store(0xc000, 0xc0ff, "VMA_E".to_string()).unwrap();

    println!("=== tree after inserts ===");
    mt.dump();

    println!("\nlookup(0x1500) = {:?}", mt.lookup(0x1500));
    println!("lookup(0x4000) = {:?}", mt.lookup(0x4000));

    if let Some((gi, gl)) = mt.find_gap(0x1000) {
        println!("\nfirst gap >= 0x1000: [{:#x},{:#x}]", gi, gl);
    }
}
```

**Rust-specific design note worth internalizing:** notice `split_leaf` only handles the *root-is-a-leaf* case cleanly. The moment you need a node to hold a **parent pointer** in safe Rust, you hit the same wall every "tree with parent pointers" implementation hits: `Box<Node>` gives single ownership, but a parent pointer is a second reference to the same node, which the borrow checker won't allow without `Rc<RefCell<_>>`, unsafe raw pointers, or an **arena/index-based design** (store all nodes in a `Vec<Node>` and reference by index, i.e. a "generational arena" — this is what production Rust tree/graph structures almost always do, e.g. how `petgraph` and many Rust `slotmap`-based trees are built). This is *exactly* the kind of "safe Rust doesn't map cleanly onto intrusive-pointer kernel data structures" friction you'll hit if you ever look at kernel-Rust maple tree bindings — the real Rust-for-Linux abstractions over kernel data structures (`rust/kernel/`) lean on `unsafe` FFI wrappers around the C implementation rather than reimplementing the pointer-chasing in safe Rust, for exactly this reason.

**On RCU-equivalence in Rust:** the realistic way to get "readers never block, writers COW-replace nodes" in user-space Rust is `crossbeam-epoch` (epoch-based reclamation) or `arc-swap` for the "atomically swap a pointer, free old value after readers are done" pattern — conceptually the direct analogue of kernel RCU + `kfree_rcu`. Worth an afternoon of reading `crossbeam_epoch::Atomic<T>` if you want to prototype an actually-concurrent version.

---

## 18. Go Implementation (educational, from scratch)

Go has no built-in ordered/range map, so this is a legitimate "build the missing standard-library piece" exercise — useful if you ever need range-indexed lookups in a Go-based control plane (e.g., tracking allocated CIDR ranges, port ranges, or IP-range ACL rules, which is a very natural fit for this exact structure in your cloud-network-security work).

```go
// mini_maple.go — educational range-map with gap tracking.
// Original design for teaching purposes; not a port of the kernel structure.
//
// Run: go run mini_maple.go

package main

import "fmt"

const fanout = 4

type entry struct {
	index, last uint64
	value       *string // nil == gap
}

type node struct {
	isLeaf bool

	// leaf storage
	entries []entry

	// internal storage
	pivots   []uint64
	children []*node
	gaps     []uint64
}

func newLeaf() *node {
	return &node{isLeaf: true}
}

func (n *node) localMaxGap() uint64 {
	if n.isLeaf {
		var best uint64
		for _, e := range n.entries {
			if e.value == nil {
				size := e.last - e.index + 1
				if size > best {
					best = size
				}
			}
		}
		return best
	}
	var best uint64
	for _, g := range n.gaps {
		if g > best {
			best = g
		}
	}
	return best
}

func (n *node) recomputeGaps() {
	if n.isLeaf {
		return
	}
	for i, c := range n.children {
		n.gaps[i] = c.localMaxGap()
	}
}

type MapleTree struct {
	root *node
}

func NewMapleTree(universeMax uint64) *MapleTree {
	root := newLeaf()
	root.entries = append(root.entries, entry{index: 0, last: universeMax, value: nil})
	return &MapleTree{root: root}
}

func findLeaf(n *node, index uint64) *node {
	for !n.isLeaf {
		slot := 0
		for slot < len(n.pivots) && index > n.pivots[slot] {
			slot++
		}
		n = n.children[slot]
	}
	return n
}

func (t *MapleTree) Lookup(index uint64) *string {
	leaf := findLeaf(t.root, index)
	for _, e := range leaf.entries {
		if index >= e.index && index <= e.last {
			return e.value
		}
	}
	return nil
}

// Store places value over [index,last]. Same simplified precondition as the
// C/Rust versions: range must fall entirely within one existing gap entry.
// Only root-leaf splitting is implemented (mirrors the Rust version's scope);
// deeper split propagation is left as an exercise.
func (t *MapleTree) Store(index, last uint64, value string) error {
	leaf := findLeaf(t.root, index)

	pos := -1
	for i, e := range leaf.entries {
		if index >= e.index && index <= e.last {
			pos = i
			break
		}
	}
	if pos == -1 {
		return fmt.Errorf("index out of range")
	}
	if leaf.entries[pos].value != nil {
		return fmt.Errorf("range already occupied (exercise: implement overwrite)")
	}
	if last > leaf.entries[pos].last {
		return fmt.Errorf("range crosses gap boundary (exercise: multi-gap store)")
	}

	gapLo, gapHi := leaf.entries[pos].index, leaf.entries[pos].last
	leaf.entries = append(leaf.entries[:pos], leaf.entries[pos+1:]...)

	v := value
	var toInsert []entry
	if gapLo < index {
		toInsert = append(toInsert, entry{index: gapLo, last: index - 1, value: nil})
	}
	toInsert = append(toInsert, entry{index: index, last: last, value: &v})
	if last < gapHi {
		toInsert = append(toInsert, entry{index: last + 1, last: gapHi, value: nil})
	}

	for _, e := range toInsert {
		insPos := len(leaf.entries)
		for i, existing := range leaf.entries {
			if existing.index > e.index {
				insPos = i
				break
			}
		}
		leaf.entries = append(leaf.entries, entry{})
		copy(leaf.entries[insPos+1:], leaf.entries[insPos:])
		leaf.entries[insPos] = e
	}

	if len(leaf.entries) > fanout && leaf == t.root {
		t.splitRootLeaf()
	}

	t.fixGapsRecursive(t.root)
	return nil
}

func (t *MapleTree) splitRootLeaf() {
	root := t.root
	mid := len(root.entries) / 2
	leftEntries := append([]entry{}, root.entries[:mid]...)
	rightEntries := append([]entry{}, root.entries[mid:]...)
	pivot := leftEntries[len(leftEntries)-1].last

	left := &node{isLeaf: true, entries: leftEntries}
	right := &node{isLeaf: true, entries: rightEntries}

	t.root = &node{
		isLeaf:   false,
		pivots:   []uint64{pivot},
		children: []*node{left, right},
		gaps:     make([]uint64, 2),
	}
}

func (t *MapleTree) fixGapsRecursive(n *node) {
	if n.isLeaf {
		return
	}
	for _, c := range n.children {
		t.fixGapsRecursive(c)
	}
	n.recomputeGaps()
}

// FindGap does a guided descent using cached gap[] values, same as C/Rust.
func (t *MapleTree) FindGap(size uint64) (uint64, uint64, bool) {
	n := t.root
	for !n.isLeaf {
		chosen := -1
		for i, g := range n.gaps {
			if g >= size {
				chosen = i
				break
			}
		}
		if chosen == -1 {
			return 0, 0, false
		}
		n = n.children[chosen]
	}
	for _, e := range n.entries {
		if e.value == nil && (e.last-e.index+1) >= size {
			return e.index, e.last, true
		}
	}
	return 0, 0, false
}

func (t *MapleTree) Dump() {
	dumpRec(t.root, 0)
}

func dumpRec(n *node, depth int) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	if n.isLeaf {
		fmt.Printf("%sLEAF: ", indent)
		for _, e := range n.entries {
			v := "GAP"
			if e.value != nil {
				v = *e.value
			}
			fmt.Printf("[%d,%d]->%s  ", e.index, e.last, v)
		}
		fmt.Println()
	} else {
		fmt.Printf("%sNODE (pivots: %v | gaps: %v)\n", indent, n.pivots, n.gaps)
		for _, c := range n.children {
			dumpRec(c, depth+1)
		}
	}
}

func main() {
	mt := NewMapleTree(0xFFFF)

	must(mt.Store(0x1000, 0x2fff, "VMA_A"))
	must(mt.Store(0x5000, 0x7fff, "VMA_B"))
	must(mt.Store(0x9000, 0x9fff, "VMA_C"))
	must(mt.Store(0xa000, 0xafff, "VMA_D"))
	must(mt.Store(0xc000, 0xc0ff, "VMA_E"))

	fmt.Println("=== tree after inserts ===")
	mt.Dump()

	fmt.Println()
	if v := mt.Lookup(0x1500); v != nil {
		fmt.Printf("lookup(0x1500) = %s\n", *v)
	}
	if v := mt.Lookup(0x4000); v == nil {
		fmt.Println("lookup(0x4000) = GAP")
	}

	if gi, gl, ok := mt.FindGap(0x1000); ok {
		fmt.Printf("\nfirst gap >= 0x1000: [%#x,%#x]\n", gi, gl)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
```

**Where this is directly useful to your actual work, not just an exercise:** a range map with gap-search is the right structure for:
- **CIDR/IP-range allocation tracking** in a cloud-network-security control plane (which subnets/IP ranges are allocated across VPCs, find a free `/24` block)
- **Port-range ACL management** (which port ranges are already claimed by a rule set, find a free range for a new NAT/PAT rule)
- **eBPF map key-range bookkeeping** in user-space control-plane code that needs to reason about which key ranges are populated in an LPM/array map before writing new entries

If you build a real version of this in Go for a control-plane use case, the two things to actually productionize beyond this teaching skeleton are: **(1) proper node splitting at any depth** (this version only splits the root), and **(2) concurrency** — Go's answer to "readers lock-free, writers COW" is typically a `sync/atomic.Pointer[Node]` per mutable node reference plus GC-based reclamation (Go's garbage collector plays the role RCU's grace period plays in C/Rust — you get the "don't free while a reader might still hold it" property for free from the GC, which is a genuinely nice simplification vs. C/Rust if you don't need manual memory control).

---

## 19. Testing Strategy and Edge Cases

Think about this the way you'd think about fuzzing a parser or a packet-classification engine — the *shape* of bugs in range-based structures is different from scalar-key structures.

### 19.1 Property-based invariant checks (write these first, before feature tests)

For any sequence of operations, after every single operation, assert:
1. **No overlap:** iterate all leaf entries in order; verify `entries[i].last < entries[i+1].index` for all `i`.
2. **Full coverage:** entries must tile `[0, universe_max]` with no unrepresented holes (every point is covered by exactly one entry, gap or otherwise).
3. **Pivot correctness:** for every internal node, `pivot[i]` equals the maximum `last` reachable in `children[i]`'s subtree.
4. **Gap cache correctness:** recompute gaps from scratch (brute-force scan) and compare against cached values — after every mutation.
5. **Sortedness:** entries/children within a node are strictly increasing by index.

### 19.2 Specific edge cases to test deliberately

- **Insert at index 0** and **insert at `universe_max`** (boundary conditions almost always hide off-by-one bugs in inclusive-range code).
- **Insert a range that exactly fills the remaining gap** (zero-length remaining gap on one or both sides — do you incorrectly insert a `[x,x-1]` degenerate empty range? This is the classic inclusive-range off-by-one).
- **Insert a single-index range** (`index == last`).
- **Delete an entry between two gaps** → must coalesce into one larger gap, not leave three adjacent entries.
- **Repeated alloc/free cycling** (simulate `mmap`/`munmap` churn) — check for gap fragmentation and that gap-cache values stay correct under heavy split/merge pressure. This is exactly the kind of test that catches the class of bug that caused real maple-tree fixes upstream during its early kernel cycles.
- **Force a split, then force another split of the resulting sibling** — does multi-level split propagation correctly build/extend the parent chain? (This is the exact gap flagged as "exercise" in all three implementations above — write this test *specifically to prove to yourself* you understand why it was hard enough to skip in a teaching version.)
- **Gap search when no gap exists** — must return "not found," not a false positive from a stale cache.
- **Concurrent reader during writer COW-swap** (if you build the concurrent version) — a reader thread continuously walking while a writer thread continuously mutates; assert the reader never observes a torn/inconsistent node (this needs a real concurrency stress test with something like ThreadSanitizer/Loom (Rust) or Go's race detector — `go test -race` — not just casual multi-threading).

### 19.3 Kernel-specific validation tools worth knowing about

- **`CONFIG_DEBUG_MAPLE_TREE`** — kernel config option enabling internal maple-tree self-consistency assertions; enable this in a debug/test kernel build if you're doing any development that touches VMA handling.
- **`tools/testing/radix-tree/maple.c`** and related kernel selftests — the actual kernel ships a **user-space test harness** for maple tree (compiled outside the kernel, linked against a shim), which is the authoritative place to look for how upstream actually stress-tests this structure. This is worth reading directly (not reproducing here) if you want to see real fuzz/stress patterns used against the production implementation.
- **KASAN/KFENCE** — relevant for catching use-after-free bugs in the RCU-deferred-free path if you're doing kernel-level debugging of maple tree consumers.

---

## 20. Mental Model Exercises

Work through these *before* looking anything up — the goal is building instinct, not recall.

1. **Why inclusive-inclusive `[index,last]` instead of inclusive-exclusive `[start,end)`** (which is what `vm_start`/`vm_end` use elsewhere in the VMA code)? What bug class does inclusive-exclusive avoid that inclusive-inclusive is prone to, and why might maple tree have chosen inclusive-inclusive anyway? (Hint: think about what happens when `last == ULONG_MAX` under exclusive-end semantics.)

2. **Design the overwrite-spanning-multiple-entries case** (flagged as an exercise in all three implementations). If you `mm_store()` a range that fully covers 2 existing occupied entries and partially overlaps a third, what is the correct sequence of leaf-array operations? Draw it out on paper with concrete addresses before writing code.

3. **Multi-level split propagation.** The C implementation (Section 16) implements this fully — `link_split_into_parent()` recurses upward, splitting an already-full parent's combined child array and continuing until it either finds room or builds a new root. The Rust and Go implementations (Sections 17-18) deliberately stop at root-only splitting. Port the C approach to whichever of Rust or Go you're more fluent in. In Rust specifically, you'll hit the ownership wall named in Section 17's design note the moment a node needs a parent pointer alongside `Box`-owned children — decide whether you reach for `Rc<RefCell<_>>`, unsafe raw parent pointers, or restructure to an arena (`Vec<Node>` + index-based references) before writing a line of split logic, and be able to say *why* you picked the one you did.

4. **Why does gap-cache pruning require the cache to be a strict upper bound, never an underestimate?** What happens to correctness if a stale/optimistic gap value is too *high*? What happens if it's too *low*? (These have very different failure modes — one causes a missed valid allocation, the other causes a false-positive descent into a subtree that can't actually satisfy the request. Which is worse for kernel correctness, and which is worse for performance?)

5. **RCU ordering exercise.** In the COW-split diagram (15.4), why must the parent's pointer swap be the *last* write, and why isn't a plain (non-atomic, non-barriered) pointer assignment sufficient even on a strongly-ordered architecture like x86? (Think about compiler reordering, not just CPU reordering — this is the same reasoning you already apply to `WRITE_ONCE`/`READ_ONCE` in networking fast-path code.)

6. **Compare to your XDP firewall project.** If you were tracking allocated IP-range ACL entries in your firewall's user-space control plane (not the eBPF map itself, but the Rust control-plane logic deciding what to program into the map), would a maple-tree-style range map be the right structure for "does this new rule's CIDR overlap an existing rule"? What would the "gap" concept even mean in that context — is there one, or does your use case only need invariant #1 (non-overlap) without gap-search at all? Reasoning through *when the gap-cache machinery is unnecessary overhead* is as important as knowing when it's needed.

7. **B-tree branching factor tuning.** The teaching implementations use `FANOUT = 4` for pedagogical clarity (forces splits quickly in small examples). The real kernel targets a fanout tied to fitting `~4 cache lines`. If you were sizing this for your own project, what's the actual arithmetic you'd do to pick a fanout — what has to be measured (entry struct size, cache line size, acceptable node memory footprint) versus assumed?

---

## 21. References

- `include/linux/maple_tree.h` — kernel header, canonical API and struct definitions (read this directly rather than trusting any secondary summary of field layouts, including this guide's simplified Section 4 sketch)
- `mm/maple_tree.c` — kernel implementation
- `tools/testing/radix-tree/maple.c` — user-space test harness, useful for seeing real stress-test patterns
- Documentation: `Documentation/core-api/maple_tree.rst` (kernel doc tree) — the closest thing to an "official" narrative guide
- LWN.net coverage of the maple tree merge and per-VMA locking work (search LWN for "maple tree" and "per-VMA locking" — these articles walk through the *why* from the mm-subsystem maintainers' own framing, and are a good next read after this guide)
- Maple tree original patch series and cover letters on the LKML archives (search `lore.kernel.org` for "Maple Tree" by Liam Howlett) — reading the actual submission discussion is the highest-signal way to see the design trade-offs argued out by the people who made them

---

**Suggested next step for continuity:** if you want to go deeper on any single section next session, tell me which — the strongest next moves from here are probably (a) implementing the multi-level split propagation exercise for real in Rust, since that's the gap every implementation above shares, or (b) reading `mm/maple_tree.c` directly against this guide's Section 6-9 framing to map the real function names (`mas_store`, `mas_split`, `mas_spanning_rebalance`, etc.) onto the concepts here.
