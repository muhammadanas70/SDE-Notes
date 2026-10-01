# Union-Find (Disjoint Set Union) — The Complete Mental-Model Guide

**Languages:** Go · Rust · C  **Diagrams:** ASCII only  **Level:** from zero to advanced variants

> **Code status:** every listing was written to be compilable (Go 1.21+, Rust 2021 edition / 1.70+, C11) and was traced by hand against the dry runs below, but it was **not executed** in this session. Compile and run the test blocks yourself; that is part of the learning.

---

## Table of Contents

1. [Concept and Why It Matters](#1-concept-and-why-it-matters)
2. [The Mental Model: A Forest in an Array](#2-the-mental-model-a-forest-in-an-array)
3. [Real Architecture: Layers, Memory, Cache](#3-real-architecture-layers-memory-cache)
4. [The Evolution Ladder: Quick-Find → Quick-Union → Weighted → Compressed](#4-the-evolution-ladder)
5. [Full Dry Run (10 elements, 9 unions)](#5-full-dry-run)
6. [Algorithm Derivation, Invariants, Correctness](#6-algorithm-derivation-invariants-correctness)
7. [Complexity Analysis (with the Ackermann intuition)](#7-complexity-analysis)
8. [Core Implementations: Go, Rust, C (+ tests)](#8-core-implementations)
9. [Real-World Code: Kruskal, Grid Islands, Grouping by Keys, Cycle Detection](#9-real-world-code)
10. [Manipulations and Advanced Variants](#10-manipulations-and-advanced-variants)
11. [Go vs Rust vs C: What Actually Differs](#11-go-vs-rust-vs-c)
12. [Common Mistakes (Failure Analysis)](#12-common-mistakes-failure-analysis)
13. [Pattern Recognition: When Is It Union-Find?](#13-pattern-recognition)
14. [Active Recall Questions](#14-active-recall-questions)
15. [Practice Problems](#15-practice-problems)
16. [Key Takeaways and Portable Progress Summary](#16-key-takeaways-and-portable-progress-summary)

---

# 1. Concept and Why It Matters

## 1.1 The problem

You have `n` items. Over time you receive two kinds of events:

* **"a and b belong together"** (merge their groups)
* **"are a and b in the same group?"** (query)

Nothing else. Groups only ever **merge**; they never split.

Formally this maintains a **partition** of `{0, 1, ..., n-1}` into disjoint sets, where "same set" is an **equivalence relation**:

| Property   | Meaning                               |
| ---------- | ------------------------------------- |
| Reflexive  | `a ~ a`                               |
| Symmetric  | `a ~ b  ⇒  b ~ a`                     |
| Transitive | `a ~ b` and `b ~ c  ⇒  a ~ c`         |

Transitivity is the whole point. If you are told `0~1` and `1~2`, you must answer "yes" to `0~2` without ever being told it directly.

## 1.2 The abstract data type (ADT)

| Operation        | Meaning                                                | Typical name          |
| ---------------- | ------------------------------------------------------ | --------------------- |
| `MakeSet(x)`     | create a new singleton set `{x}`                       | constructor / add     |
| `Find(x)`        | return the **representative** (canonical id) of x's set | find                  |
| `Union(a, b)`    | merge the sets containing a and b                      | union / merge / link  |
| `Connected(a,b)` | `Find(a) == Find(b)`                                   | same-set              |
| `Count()`        | number of disjoint sets                                | components            |
| `Size(x)`        | number of elements in x's set                          | component size        |

The **key contract**:

```text
Find(x) == Find(y)   <=>   x and y are in the same set
```

The representative can be *any* member; what matters is that it is **stable between unions** (it may only change when the set is merged with another).

## 1.3 Why simpler approaches fail

Imagine `n = 10^6` items and `q = 10^6` mixed operations.

| Approach                          | Union cost     | Query cost | Total for n,q = 10^6 |
| --------------------------------- | -------------- | ---------- | -------------------- |
| Store edges, run BFS/DFS per query | O(1)           | O(V+E)     | ~10^12 (too slow)    |
| `label[]` array, relabel on union  | O(n)           | O(1)       | ~10^12 (too slow)    |
| Union-Find (weighted + compressed) | ~O(α(n))       | ~O(α(n))   | ~10^6 × tiny constant |

`α(n)` is the inverse Ackermann function; it is at most 4 for any `n` you can physically store. Section 7 derives what that means.

## 1.4 Where it is used in practice

| Domain                         | How Union-Find is used                                          |
| ------------------------------ | --------------------------------------------------------------- |
| Minimum spanning tree          | Kruskal's algorithm: "does this edge close a cycle?"            |
| Network / social connectivity  | "Are these two machines/users in the same cluster?"             |
| Image processing               | Connected-component labeling of pixels (Hoshen–Kopelman style)  |
| Compilers / type systems       | Unification in type inference; alias analysis (Steensgaard)     |
| Automata theory                | Equivalence testing of DFA states (Hopcroft–Karp)               |
| Physics simulation             | Percolation experiments                                         |
| Offline graph algorithms       | Tarjan's offline LCA; offline dynamic connectivity (rollback)   |
| Data cleaning                  | Merging duplicate records / accounts sharing an identifier      |
| Game / maze generation         | Randomized Kruskal mazes                                        |

## 1.5 When it is the *wrong* tool

* You need **deletions / splits** online (use link-cut trees, Euler-tour trees, or offline tricks in Section 10).
* You need the **actual path** between two nodes (use BFS/DFS).
* You need **shortest distance** (BFS / Dijkstra).
* The graph is **directed** and you need reachability (use SCC / DFS). Union-Find models *undirected* connectivity only.

---

# 2. The Mental Model: A Forest in an Array

## 2.1 Core idea

Each set is stored as a **rooted tree**. Every element stores only a pointer to its **parent**. The **root** points to itself. The root **is** the representative.

```text
Set {0,1,2,3}  as a tree:        parent[] array:

        0   <-- root                 index:   0  1  2  3
       /|\                           parent: [0, 0, 0, 2]
      1 2 ...                        
        |                            Read it as: "0's parent is 0 (root),
        3                             1's parent is 0, 2's parent is 0,
                                      3's parent is 2".
```

The whole structure is a **forest** (many trees) stored in one flat integer array, where **array index = node id** and **array value = parent id**. No node objects, no pointers in the heap sense, just indices.

## 2.2 Ten singletons, then a few unions

```text
Initially: every element is its own root (10 trees, each of size 1)

  (0) (1) (2) (3) (4) (5) (6) (7) (8) (9)

  parent = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
```

After `union(0,1)`, `union(2,3)`, `union(1,3)`:

```text
            0               4   5   6   7   8   9
           / \
          1   2
              |
              3

  parent = [0, 0, 0, 2, 4, 5, 6, 7, 8, 9]
            ^  ^  ^  ^
            |  |  |  +-- 3's parent is 2
            |  |  +----- 2's parent is 0 (2's tree hung under 0's tree)
            |  +-------- 1's parent is 0
            +----------- 0 is a root (points to itself)
```

## 2.3 Find = walk up to the root

```text
Find(3):   3 ---> 2 ---> 0 (parent[0]==0, stop)   answer: 0
```

## 2.4 Union = link two roots

```text
Union(a, b):
    ra = Find(a)
    rb = Find(b)
    if ra == rb: already together, do nothing
    else: make one root the child of the other  (ONE pointer write)

Before:   ra          rb           After:      ra
         / \          |                       / | \
        ..  ..       ..                      .. .. rb
                                                    |
                                                   ..
```

**The single most important sentence in this guide:** *Union operates on **roots**, not on the elements you were handed.*

## 2.5 Invariants (must hold after every public operation)

| #  | Invariant                                                                              |
| -- | -------------------------------------------------------------------------------------- |
| I1 | Following `parent` from any node always terminates at a root (no cycles except self-loop at root) |
| I2 | A node is a root **iff** `parent[x] == x`                                              |
| I3 | Every node is in exactly one tree (partition property)                                 |
| I4 | For every root `r`: `size[r]` = number of nodes in r's tree (`size` of non-roots is stale/unused) |
| I5 | `count` = number of roots                                                              |
| I6 | Two nodes are in the same set iff they share the same root                             |

Every correctness bug in Union-Find is a broken invariant. When debugging, check I2 and I4 first.

---

# 3. Real Architecture: Layers, Memory, Cache

## 3.1 Logical layers

```text
+--------------------------------------------------------------------+
| Client code: Kruskal, grid labeling, equation solver, dedupe       |
+--------------------------------------------------------------------+
| Public API:   Union(a,b)  Connected(a,b)  Size(x)  Count()         |
+--------------------------------------------------------------------+
| Root logic:   Find(x) = climb + compress    Link(ra,rb) = by size  |
+--------------------------------------------------------------------+
| Storage:      parent[]   size[] (or fused)   count   [diff[]...]   |
+--------------------------------------------------------------------+
| Memory:       contiguous heap arrays -> cache lines -> RAM         |
+--------------------------------------------------------------------+
```

Everything interesting lives in two places: **Find's climb** (pointer-chasing through the array) and **Link's decision** (which root becomes the child).

## 3.2 Actual memory layout per language (64-bit platform)

### Go

```text
type UnionFind struct { parent []int; size []int; count int }

The struct itself (56 bytes; heap-allocated because NewUnionFind returns
a pointer that escapes):

+---------------------------- UnionFind ----------------------------+
| parent: [ptr 8B | len 8B | cap 8B ]   <- slice header (24B)      |
| size:   [ptr 8B | len 8B | cap 8B ]   <- slice header (24B)      |
| count:  [int 8B]                                                 |
+-------------------------------------------------------------------+
          |                                   |
          v                                   v
  heap backing array (n * 8 bytes)    heap backing array (n * 8 bytes)
  +----+----+----+----+---            +----+----+----+----+---
  | p0 | p1 | p2 | p3 | ...           | s0 | s1 | s2 | s3 | ...
  +----+----+----+----+---            +----+----+----+----+---

Notes: Go's `int` is 8 bytes. Two arrays => 16 bytes per element.
       Garbage collector owns both backing arrays; nothing to free by hand.
```

### Rust

```text
pub struct UnionFind { parent: Vec<usize>, size: Vec<usize>, count: usize }

The struct (56 bytes) lives wherever you put it (usually the stack).
Each Vec is a 24-byte handle {ptr, cap, len} (field order is unspecified):

STACK                                       HEAP
+----------------------- UnionFind ---------+
| parent: Vec { ptr, cap, len } ------------+--> [ p0 | p1 | p2 | ... ]  (n * 8 B)
| size:   Vec { ptr, cap, len } ------------+--> [ s0 | s1 | s2 | ... ]  (n * 8 B)
| count:  usize                             |
+-------------------------------------------+

Ownership: the struct OWNS both heap buffers. When the struct is dropped,
Drop frees both. Only one &mut can exist at a time, so no data races.
```

### C

```text
typedef struct { int *parent; int *size; int n; int cap; int count; } UnionFind;

+----------------------- UnionFind (32 B with padding) ----+
| parent: [ptr 8B]   size: [ptr 8B]                        |
| n: 4B   cap: 4B   count: 4B   (+4B padding)              |
+------------------------------------------------------------+
        |                       |
        v                       v
  malloc'd int[cap]       malloc'd int[cap]     (4 bytes per element)
  +----+----+----+---      +----+----+----+---
  | p0 | p1 | p2 | ...     | s0 | s1 | s2 | ...

Notes: YOU own the memory: uf_free() must free parent, size, then the struct.
       `int` is 4 bytes => half the footprint of Go/Rust's 8-byte words.
```

## 3.3 Why memory footprint and layout matter

For `n = 10^7`:

| Layout                                          | Bytes / element | Total       |
| ----------------------------------------------- | --------------- | ----------- |
| Go `[]int` × 2 (parent + size)                   | 16              | 160 MB      |
| Rust `Vec<usize>` × 2                            | 16              | 160 MB      |
| Rust/Go with 32-bit ids (`u32`/`int32`) × 2      | 8               | 80 MB       |
| **Fused** single `int32` array (Section 3.4)     | 4               | **40 MB**   |

Smaller footprint = more entries per **cache line** (typically 64 bytes):

```text
One 64-byte cache line holds:
   8 x  int64 entries  (Go int / Rust usize)
  16 x  int32 entries  (C int, Rust u32)

  |<-------------------- 64 bytes -------------------->|
  | p[k] p[k+1] p[k+2] ... p[k+15]  (int32 version)    |
```

Find is a **chain of dependent loads** (you cannot load `parent[parent[x]]` until `parent[x]` arrives). A cache miss costs on the order of ~100 ns while an L1 hit costs ~1 ns (orders of magnitude, hardware dependent). That is why **path compression** matters beyond asymptotics: it shortens the dependent chain for future queries.

```text
Dependent-load chain of Find(x) on a depth-3 path:

  load parent[x]  --> a       (must finish first)
  load parent[a]  --> b       (depends on a)
  load parent[b]  --> root    (depends on b)
  load parent[root]==root     (loop exit test)

Each arrow can be a cache miss. Depth 1 after compression = 1-2 loads.
```

## 3.4 Fused encoding: parent and size in one array

A root has no parent, so its slot is free. Store **negative size** at roots:

```text
p[x] >= 0  :  x is a non-root, p[x] is its parent
p[x] <  0  :  x is a root, size of its set is -p[x]

Example (sets {0,1,2,3}, {4,5}, {6}):
 index:  0   1  2  3   4   5   6
 p[]:  [-4,  0, 0, 2, -2,  4, -1]
        ^root      ^root      ^root
```

Saves half the memory and one cache miss per Union. Implementation is in Section 8.3 (C). Trade-off: slightly harder to read; root test becomes `p[x] < 0`.

---

# 4. The Evolution Ladder

Each step fixes a specific weakness of the previous one. Understanding *why* each exists is the mental model.

## 4.1 Step 1: Quick-Find (eager)

Store the **set label directly**: `id[x]` is x's set id. Find is a single array read.

```text
id = [0, 1, 2, 3, 4, 5]        Union(1, 4):  relabel every 4 as 1
id = [0, 1, 2, 3, 1, 5]

Find(x) = id[x]                              O(1)
Union   = scan the whole array               O(n)
```

`n-1` unions cost `Θ(n²)`. Fine for tiny n, hopeless for big n.

## 4.2 Step 2: Quick-Union (lazy)

Store **parent pointers** and only change one pointer per Union. But trees can degenerate into chains:

```text
Union(0,1), Union(1,2), Union(2,3), Union(3,4)   with "parent[ra] = rb":

  0 -> 1 -> 2 -> 3 -> 4         (a linked list, height n-1)

Find(0) walks n-1 steps.  Union costs O(height).  Worst case Θ(n) each.
```

## 4.3 Step 3: Weighted Union (by size) — fix the height

**Rule:** always attach the **smaller tree's root** under the **larger tree's root**.

```text
Without rule (chain):        With rule (small under big):

 0 -> 1 -> 2 -> 3           merging {0,1,2} (size 3) with {3} (size 1):
                                    0
                                  / | \
                                 1  2  3       <- height stays 1
```

**Theorem:** with union-by-size, every tree has height ≤ ⌊log₂ n⌋.

**Proof.** Track one node `x`. Its depth increases by 1 only when its tree (size `s`) is attached under a root whose tree has size `≥ s`. After merging, the tree containing `x` has size `≥ 2s`. Sizes never exceed `n`, so the size can double at most `log₂ n` times, hence depth(x) ≤ log₂ n. ∎

Now Find and Union are both `O(log n)` worst case, guaranteed.

## 4.4 Step 3b: Union by rank (equivalent idea)

`rank[r]` is an upper bound on the height of the tree rooted at `r`. Attach the lower-rank root under the higher-rank root; if equal, pick one and increment its rank.

```text
rank 0 + rank 0  ->  new root has rank 1
rank 2 + rank 1  ->  rank stays 2 (short tree hangs under tall tree)
rank 2 + rank 2  ->  new rank 3
```

**Lemma:** a root of rank `r` has at least `2^r` nodes, so `r ≤ log₂ n`. Size-based and rank-based give the same bounds. Size is more useful in practice because it doubles as `Size(x)` for free.

## 4.5 Step 4: Path Compression — flatten as you go

Find already visits every node on the path from `x` to the root. Use that walk to **re-point** visited nodes closer to the root. The cost is paid once; every later Find on those nodes is cheaper.

Start with a chain (deepest node is 4):

```text
parent pointers:  4 -> 3 -> 2 -> 1 -> 0        (0 is root)
```

### Variant A: Full compression (two-pass or recursive)

Every node on the path points **directly to the root**.

```text
After Find(4):
        0
      / | \ \
     1  2  3  4
```

### Variant B: Path halving (one pass, iterative)

Every **other** node on the path is re-pointed to its grandparent, then we jump to that grandparent.

```text
step: x=4: parent[4] = parent[parent[4]] = 2;   x = 2
      x=2: parent[2] = parent[parent[2]] = 0;   x = 0 (root) stop

Result:      0
            / \
           1   2        (3 -> 2 unchanged, 4 -> 2)
               / \
              3   4
```

### Variant C: Path splitting

**Every** node on the path is re-pointed to its grandparent; then we move to the *old parent*.

```text
Result:   0
         / \
        1   2          (1 -> 0, 2 -> 0, 3 -> 1, 4 -> 2)
        |   |
        3   4
```

| Variant  | Passes | Recursion | Flattening       | Typical use            |
| -------- | ------ | --------- | ---------------- | ---------------------- |
| Full     | 1–2    | often     | maximum          | textbooks              |
| Halving  | 1      | no        | ~half per visit  | **production default** |
| Splitting| 1      | no        | ~half per visit  | alternative to halving |

All three, combined with union by size/rank, achieve the same `O(α(n))` amortized bound. **Path halving is the one to memorize**: iterative, tiny, no stack risk.

## 4.6 The ladder at a glance

| Version                            | Find             | Union            | m ops on n elems         |
| ---------------------------------- | ---------------- | ---------------- | ------------------------ |
| Quick-Find                         | O(1)             | O(n)             | O(m·n)                   |
| Quick-Union                        | O(n) worst       | O(n) worst       | O(m·n)                   |
| Weighted (size/rank) only          | O(log n)         | O(log n)         | O(m log n)               |
| Path compression only              | O(log n) amort.  | O(log n) amort.  | O(m log_{1+m/n} n)       |
| **Weighted + compression**         | **O(α(n)) amort.** | **O(α(n)) amort.** | **O(m α(n))**          |

---

# 5. Full Dry Run

Setup: `n = 10`, **union by size** (ties: the first argument's root stays root), **path halving** in Find.

Initial state: `parent = [0,1,2,3,4,5,6,7,8,9]`, all `size = 1`, `count = 10`.

| Step | Operation      | Roots found (ra, rb) | Action                                             | `parent` array after            | count |
| ---- | -------------- | -------------------- | -------------------------------------------------- | ------------------------------- | ----- |
| 1    | `union(0,1)`   | 0, 1                 | sizes 1,1 (tie) → `parent[1]=0`, `size[0]=2`       | `[0,0,2,3,4,5,6,7,8,9]`         | 9     |
| 2    | `union(2,3)`   | 2, 3                 | tie → `parent[3]=2`, `size[2]=2`                   | `[0,0,2,2,4,5,6,7,8,9]`         | 8     |
| 3    | `union(4,5)`   | 4, 5                 | tie → `parent[5]=4`, `size[4]=2`                   | `[0,0,2,2,4,4,6,7,8,9]`         | 7     |
| 4    | `union(6,7)`   | 6, 7                 | tie → `parent[7]=6`, `size[6]=2`                   | `[0,0,2,2,4,4,6,6,8,9]`         | 6     |
| 5    | `union(1,3)`   | 0, 2                 | sizes 2,2 (tie) → `parent[2]=0`, `size[0]=4`       | `[0,0,0,2,4,4,6,6,8,9]`         | 5     |
| 6    | `union(5,7)`   | 4, 6                 | tie → `parent[6]=4`, `size[4]=4`                   | `[0,0,0,2,4,4,4,6,8,9]`         | 4     |
| 7    | `union(3,7)`   | 0, 4                 | Find(3) halves: `parent[3]=0`. Find(7) halves: `parent[7]=4`. Tie → `parent[4]=0`, `size[0]=8` | `[0,0,0,0,0,4,4,4,8,9]` | 3 |
| 8    | `union(8,9)`   | 8, 9                 | tie → `parent[9]=8`, `size[8]=2`                   | `[0,0,0,0,0,4,4,4,8,8]`         | 2     |
| 9    | `union(0,9)`   | 0, 8                 | `size[0]=8 ≥ size[8]=2` → `parent[8]=0`, `size[0]=10` | `[0,0,0,0,0,4,4,4,0,8]`       | 1     |

Forest snapshots:

```text
After step 4 (four pairs + two singletons):
   0     2     4     6     8   9
   |     |     |     |
   1     3     5     7

After step 6:
      0           4
     / \         / \
    1   2       5   6
        |           |
        3           7

After step 7 (note 3 was flattened up to 0, 7 up to 4, and 4 hangs under 0):
            0
        / / \  \
       1 2   3   4
                /|\
               5 6 7

Final (step 9):
            0
       /  / |  \  \
      1  2  3   4   8
               /|\  |
              5 6 7 9
```

Extra Find to see halving on the last remaining depth-2 path:

```text
Find(9):   x=9: parent[9] = parent[parent[9]] = parent[8] = 0;  x = 0  -> root 0
Now parent[9] == 0 (depth 1). The tree is flatter than before.
```

Contrast with **Quick-Find** on the same 9 operations: each Union scans 10 slots = 90 slot visits. Union-Find did ~2 pointer writes per Union. At n=10 nobody cares; at n=10^6 it is the difference between seconds and hours.

---

# 6. Algorithm Derivation, Invariants, Correctness

## 6.1 Derivation (Problem-Solving Framework)

1. **Restate:** maintain a partition under merge and same-set queries.
2. **Inputs/outputs:** `n`, then a stream of `Union(a,b)` / `Connected(a,b)`; output booleans (and maybe counts/sizes).
3. **Constraints:** `n, m` up to 10^5–10^7; needs near-constant time per op.
4. **Brute force:** relabel on union (Quick-Find), Θ(n) per union.
5. **Observation:** we only need the *representative*, not the full list. So store **one pointer per element**.
6. **Lazy linking:** union just needs to link **two representatives**: O(1) once we have them.
7. **Cost moves to Find:** height of trees. Bound height via **weighting**.
8. **Amortize further:** Find walks a path anyway; **compress** it.
9. **Invariant:** every node reaches its set's unique root; `size[root]` is exact.
10. **Correctness argument:** below.

## 6.2 Correctness

**Claim 1 (Find is correct).** `Find(x)` returns the unique root of x's tree.
*Proof.* By I1 the walk terminates; by I2 it stops exactly at a root. Path halving sets `parent[x]` to `parent[parent[x]]`, which is an **ancestor** of `x` in the same tree; it never leaves the tree, so I1, I3 are preserved and x still reaches the same root. ∎

**Claim 2 (Union is correct).** After `Union(a,b)` with `ra ≠ rb`, the elements of both sets share one root.
*Proof.* Setting `parent[rb] = ra` makes every node of rb's tree reach `ra` through `rb`. `ra` is still a root. Nothing else changed, so the partition is exactly the old one with the two sets merged. I4 holds because `size[ra] += size[rb]`. ∎

**Claim 3 (`Connected` is correct).** By I6, same set ⇔ same root ⇔ `Find(a)==Find(b)`.

**Claim 4 (compression never breaks weighting).** Compression changes heights only downward; it does not change which nodes are in a tree, so `size[root]` stays exact. (`rank` may become an *overestimate* of height. That is fine, since only the upper-bound property is needed. See Section 12.)

## 6.3 Cycle detection insight (used by Kruskal)

Processing an undirected edge `(u,v)`:

```text
Find(u) == Find(v)   =>  u and v are ALREADY connected  =>  this edge closes a cycle
Find(u) != Find(v)   =>  edge joins two components       =>  Union them
```

`Union` returning a boolean ("did I actually merge?") makes this one line of client code.

---

# 7. Complexity Analysis

## 7.1 Operation counts before Big-O

`n = 8`, worst-case unions `Union(0,1), Union(1,2), ..., Union(6,7)` (a chain-building sequence):

| Structure                   | Work per union (approx.)              | Total work |
| --------------------------- | ------------------------------------- | ---------- |
| Quick-Find                  | scan 8 slots × 7 unions               | 56         |
| Quick-Union (parent[ra]=rb) | Find(a) walks 0,1,2,…,6 steps         | 0+1+…+6 = 21 |
| Weighted                    | height ≤ 3 always                     | ≤ 7×(2×3)=42 worst bound, typically ~14 |

Scale `n` to 10^6 and the quadratic ones become 10^12 versus ~10^7. Counting operations on small numbers first makes the growth obvious.

## 7.2 Bounds per variant

| Variant                     | Bound                          | Reason                                           |
| --------------------------- | ------------------------------ | ------------------------------------------------ |
| Weighted only               | height ≤ log₂ n                | size at least doubles each time a node gets deeper |
| Compression only            | O(log n) amortized             | Tarjan–van Leeuwen; no height guarantee for one op |
| Weighted + compression      | **O(α(n)) amortized per op**   | Tarjan 1975 (upper); Fredman–Saks 1989 (matching lower bound in cell-probe model) |

**Amortized** means: total cost of any sequence of `m` operations is `O(m·α(n))`. Individual Finds can still be slow (`O(log n)`); they pay for flattening that makes later ops cheap. Contrast with worst-case-per-op guarantees.

## 7.3 The inverse Ackermann function

Ackermann-style hierarchy (CLRS definition): `A₀(j) = j+1`, `A_k(j) = A_{k-1} applied (j+1) times to j`.

| k | A_k(1)                                         |
| - | ---------------------------------------------- |
| 0 | 2                                              |
| 1 | 3                                              |
| 2 | 7                                              |
| 3 | 2047                                           |
| 4 | astronomically large (≫ 10^80, atoms in the observable universe) |

`α(n) = min{ k : A_k(1) ≥ n }`:

| n range                          | α(n) |
| -------------------------------- | ---- |
| 0 – 2                            | 0    |
| 3                                | 1    |
| 4 – 7                            | 2    |
| 8 – 2047                         | 3    |
| 2048 – A₄(1) (beyond any real n) | 4    |

Meaning: `O(α(n))` is **constant for all practical purposes** but *provably not* constant in theory.

## 7.4 Space

| Component                 | Space |
| ------------------------- | ----- |
| `parent[]`                | Θ(n)  |
| `size[]` or `rank[]`      | Θ(n) (or 0 with fused encoding) |
| Recursion stack (recursive Find) | O(log n) with weighting; O(n) without |
| Iterative Find (halving)  | O(1) auxiliary |

## 7.5 Bounds for the variants in Section 10

| Variant                      | Find             | Union            | Extra space |
| ---------------------------- | ---------------- | ---------------- | ----------- |
| Weighted (potential) UF      | O(α(n))          | O(α(n))          | Θ(n) diffs  |
| Rollback UF (no compression) | O(log n) worst   | O(log n) worst   | history stack ≤ n−1 |
| Offline dyn. connectivity    | —                | O(m log T · log n) total | O(n + m log T) |

---

# 8. Core Implementations

Design decisions used in all three languages (and why):

| Decision                 | Choice              | Reason                                         |
| ------------------------ | ------------------- | ---------------------------------------------- |
| Linking rule             | union by **size**   | gives `Size()` for free                        |
| Compression              | **path halving**    | one pass, iterative, no stack depth issue      |
| `Union` return value     | `bool` (merged?)    | powers Kruskal, cycle detection, counting      |
| Component counter        | maintained          | O(1) `Count()`                                 |
| Growable                 | `MakeSet()`         | needed for string keys / streaming data        |

## 8.1 Go (1.21+)

`uf.go`:

```go
package main

import "fmt"

// UnionFind is a disjoint-set forest with union by size and path halving.
// Amortized O(alpha(n)) per operation.
type UnionFind struct {
	parent []int
	size   []int // exact only at roots
	count  int   // number of disjoint sets
}

// NewUnionFind creates n singleton sets {0}, {1}, ..., {n-1}.
func NewUnionFind(n int) *UnionFind {
	if n < 0 {
		panic("union-find: negative size")
	}
	u := &UnionFind{
		parent: make([]int, n),
		size:   make([]int, n),
		count:  n,
	}
	for i := 0; i < n; i++ {
		u.parent[i] = i
		u.size[i] = 1
	}
	return u
}

// MakeSet appends a new singleton and returns its id.
func (u *UnionFind) MakeSet() int {
	id := len(u.parent)
	u.parent = append(u.parent, id)
	u.size = append(u.size, 1)
	u.count++
	return id
}

// Find returns the representative of x, halving the path as it climbs.
func (u *UnionFind) Find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]] // point at grandparent
		x = u.parent[x]                      // jump to it
	}
	return x
}

// Union merges the sets of a and b. It reports whether a merge happened.
func (u *UnionFind) Union(a, b int) bool {
	ra, rb := u.Find(a), u.Find(b)
	if ra == rb {
		return false
	}
	if u.size[ra] < u.size[rb] { // small tree goes under big tree
		ra, rb = rb, ra
	}
	u.parent[rb] = ra
	u.size[ra] += u.size[rb]
	u.count--
	return true
}

func (u *UnionFind) Connected(a, b int) bool { return u.Find(a) == u.Find(b) }
func (u *UnionFind) Size(x int) int           { return u.size[u.Find(x)] }
func (u *UnionFind) Count() int               { return u.count }

func main() {
	uf := NewUnionFind(10)
	pairs := [][2]int{{0, 1}, {2, 3}, {4, 5}, {6, 7}, {1, 3}, {5, 7}, {3, 7}, {8, 9}, {0, 9}}
	for _, p := range pairs {
		merged := uf.Union(p[0], p[1])
		fmt.Printf("union(%d,%d) merged=%-5v count=%d parent=%v\n", p[0], p[1], merged, uf.Count(), uf.parent)
	}
	fmt.Println("connected(4,9):", uf.Connected(4, 9), " size(5):", uf.Size(5))
}
```

**Line-by-line notes**

* `u.parent[x] = u.parent[u.parent[x]]` is the halving step: skip one level.
* `x = u.parent[x]` then moves to the (new) parent, which is the old grandparent, so each iteration skips one node.
* Swapping `ra, rb` keeps the rest of the code branch-free: after the swap `ra` is always the larger root.
* Go: slices are `(ptr,len,cap)` headers, so `u.parent[i]` is a bounds-checked load; an invalid id panics. `append` in `MakeSet` may reallocate the backing array (amortized O(1)); do not keep old slice aliases.
* `NewUnionFind` returns a pointer so all methods share one struct (pointer receivers).

`uf_test.go` (differential testing against a naive oracle):

```go
package main

import (
	"math/rand"
	"testing"
)

func TestBasics(t *testing.T) {
	uf := NewUnionFind(5)
	if uf.Count() != 5 {
		t.Fatal("initial count")
	}
	if !uf.Union(0, 1) || uf.Union(1, 0) { // second call must report no merge
		t.Fatal("union return value")
	}
	uf.Union(2, 3)
	uf.Union(1, 3)
	if !uf.Connected(0, 2) || uf.Connected(0, 4) {
		t.Fatal("connectivity")
	}
	if uf.Size(3) != 4 || uf.Count() != 2 {
		t.Fatal("size/count")
	}
}

func TestEmptyAndSingle(t *testing.T) {
	if NewUnionFind(0).Count() != 0 {
		t.Fatal("n=0")
	}
	uf := NewUnionFind(1)
	if uf.Union(0, 0) || uf.Count() != 1 {
		t.Fatal("self union must be a no-op")
	}
}

func TestAgainstOracle(t *testing.T) {
	const n = 200
	rng := rand.New(rand.NewSource(1))
	uf := NewUnionFind(n)
	label := make([]int, n) // naive Quick-Find as ground truth
	for i := range label {
		label[i] = i
	}
	for step := 0; step < 5000; step++ {
		a, b := rng.Intn(n), rng.Intn(n)
		if rng.Intn(2) == 0 {
			uf.Union(a, b)
			la, lb := label[a], label[b]
			if la != lb {
				for i := range label {
					if label[i] == lb {
						label[i] = la
					}
				}
			}
		} else if uf.Connected(a, b) != (label[a] == label[b]) {
			t.Fatalf("mismatch at step %d for (%d,%d)", step, a, b)
		}
	}
}

func BenchmarkUnionFind(b *testing.B) {
	const n = 1 << 20
	uf := NewUnionFind(n)
	rng := rand.New(rand.NewSource(2))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uf.Union(rng.Intn(n), rng.Intn(n))
	}
}
```

Run: `go test -bench . ./...`

## 8.2 Rust (2021 edition, safe code only)

`src/main.rs`:

```rust
/// Disjoint-set forest: union by size + path halving.
pub struct UnionFind {
    parent: Vec<usize>,
    size: Vec<usize>, // exact only at roots
    count: usize,
}

impl UnionFind {
    pub fn new(n: usize) -> Self {
        Self {
            parent: (0..n).collect(),
            size: vec![1; n],
            count: n,
        }
    }

    pub fn make_set(&mut self) -> usize {
        let id = self.parent.len();
        self.parent.push(id);
        self.size.push(1);
        self.count += 1;
        id
    }

    /// Needs `&mut self` because path halving WRITES to `parent`.
    pub fn find(&mut self, mut x: usize) -> usize {
        while self.parent[x] != x {
            let grand = self.parent[self.parent[x]];
            self.parent[x] = grand; // skip one level
            x = grand;              // jump to the grandparent
        }
        x
    }

    /// Read-only find: no compression, usable through `&self`.
    pub fn find_readonly(&self, mut x: usize) -> usize {
        while self.parent[x] != x {
            x = self.parent[x];
        }
        x
    }

    /// Returns true if two different sets were merged.
    pub fn union(&mut self, a: usize, b: usize) -> bool {
        let (mut ra, mut rb) = (self.find(a), self.find(b));
        if ra == rb {
            return false;
        }
        if self.size[ra] < self.size[rb] {
            std::mem::swap(&mut ra, &mut rb); // ra is now the larger root
        }
        self.parent[rb] = ra;
        self.size[ra] += self.size[rb];
        self.count -= 1;
        true
    }

    pub fn connected(&mut self, a: usize, b: usize) -> bool {
        self.find(a) == self.find(b)
    }

    pub fn size_of(&mut self, x: usize) -> usize {
        let r = self.find(x);
        self.size[r]
    }

    pub fn count(&self) -> usize {
        self.count
    }
}

fn main() {
    let mut uf = UnionFind::new(10);
    let pairs = [(0, 1), (2, 3), (4, 5), (6, 7), (1, 3), (5, 7), (3, 7), (8, 9), (0, 9)];
    for (a, b) in pairs {
        let merged = uf.union(a, b);
        println!("union({a},{b}) merged={merged} count={}", uf.count());
    }
    println!("connected(4,9)={} size(5)={}", uf.connected(4, 9), uf.size_of(5));
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn basics() {
        let mut uf = UnionFind::new(5);
        assert!(uf.union(0, 1));
        assert!(!uf.union(1, 0));
        uf.union(2, 3);
        uf.union(1, 3);
        assert!(uf.connected(0, 2));
        assert!(!uf.connected(0, 4));
        assert_eq!(uf.size_of(3), 4);
        assert_eq!(uf.count(), 2);
    }

    #[test]
    fn empty_and_self_union() {
        assert_eq!(UnionFind::new(0).count(), 0);
        let mut uf = UnionFind::new(1);
        assert!(!uf.union(0, 0));
    }

    #[test]
    fn matches_naive_oracle() {
        // tiny xorshift PRNG: no external crates needed
        let mut s: u64 = 0x9E37_79B9_7F4A_7C15;
        let mut next = move |m: usize| -> usize {
            s ^= s << 13;
            s ^= s >> 7;
            s ^= s << 17;
            (s % m as u64) as usize
        };
        let n = 200;
        let mut uf = UnionFind::new(n);
        let mut label: Vec<usize> = (0..n).collect();
        for _ in 0..5000 {
            let (a, b) = (next(n), next(n));
            if next(2) == 0 {
                uf.union(a, b);
                let (la, lb) = (label[a], label[b]);
                if la != lb {
                    for l in label.iter_mut() {
                        if *l == lb {
                            *l = la;
                        }
                    }
                }
            } else {
                assert_eq!(uf.connected(a, b), label[a] == label[b]);
            }
        }
    }
}
```

**Rust-specific explanation**

* **Ownership:** `UnionFind` owns two `Vec`s. Moving the struct moves three words per Vec, not the data. Dropping it frees both buffers automatically.
* **Why `find(&mut self)`:** compression mutates. The type system therefore *forces* callers to hold exclusive access, which also rules out data races at compile time. `find_readonly(&self)` trades compression for shared access.
* **`self.parent[self.parent[x]]`** compiles because the index expression is evaluated (a `usize` copy) before the outer borrow takes effect.
* **`self.find(a) == self.find(b)`**: two sequential `&mut` borrows; each ends when `find` returns its `usize`.
* **`std::mem::swap`** swaps two local `usize`s; no allocation, no clone.
* Out-of-range ids panic (bounds check). For untrusted input, add `debug_assert!`/checked wrappers.
* No `unwrap()` is needed anywhere in the hot path; indexing panics are the only failure mode, and they indicate a caller bug.

## 8.3 C (C11)

`uf.c`:

```c
#include <assert.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>

typedef struct {
    int *parent;
    int *size;   /* exact only at roots */
    int  n;      /* number of elements in use */
    int  cap;    /* allocated capacity */
    int  count;  /* number of disjoint sets */
} UnionFind;

UnionFind *uf_new(int n) {
    if (n < 0) return NULL;
    UnionFind *u = malloc(sizeof *u);
    if (!u) return NULL;
    int cap = n > 0 ? n : 1;                 /* malloc(0) may return NULL */
    u->parent = malloc((size_t)cap * sizeof *u->parent);
    u->size   = malloc((size_t)cap * sizeof *u->size);
    if (!u->parent || !u->size) {
        free(u->parent); free(u->size); free(u);
        return NULL;
    }
    u->n = n; u->cap = cap; u->count = n;
    for (int i = 0; i < n; i++) { u->parent[i] = i; u->size[i] = 1; }
    return u;
}

void uf_free(UnionFind *u) {
    if (!u) return;
    free(u->parent);
    free(u->size);
    free(u);
}

/* Returns the new element id, or -1 on allocation failure. */
int uf_make_set(UnionFind *u) {
    if (u->n == u->cap) {
        int ncap = u->cap * 2;               /* doubling: amortized O(1) */
        int *np = realloc(u->parent, (size_t)ncap * sizeof *np);
        if (!np) return -1;
        u->parent = np;                      /* assign right away: old ptr may be invalid */
        int *ns = realloc(u->size, (size_t)ncap * sizeof *ns);
        if (!ns) return -1;                  /* parent already grew: still consistent */
        u->size = ns;
        u->cap = ncap;
    }
    int id = u->n++;
    u->parent[id] = id;
    u->size[id] = 1;
    u->count++;
    return id;
}

int uf_find(UnionFind *u, int x) {
    while (u->parent[x] != x) {
        u->parent[x] = u->parent[u->parent[x]];  /* path halving */
        x = u->parent[x];
    }
    return x;
}

bool uf_union(UnionFind *u, int a, int b) {
    int ra = uf_find(u, a), rb = uf_find(u, b);
    if (ra == rb) return false;
    if (u->size[ra] < u->size[rb]) { int t = ra; ra = rb; rb = t; }
    u->parent[rb] = ra;
    u->size[ra] += u->size[rb];
    u->count--;
    return true;
}

bool uf_connected(UnionFind *u, int a, int b) { return uf_find(u, a) == uf_find(u, b); }
int  uf_size(UnionFind *u, int x)            { return u->size[uf_find(u, x)]; }

#ifdef UF_MAIN
int main(void) {
    UnionFind *uf = uf_new(10);
    if (!uf) { fputs("out of memory\n", stderr); return 1; }
    int pairs[][2] = {{0,1},{2,3},{4,5},{6,7},{1,3},{5,7},{3,7},{8,9},{0,9}};
    for (size_t i = 0; i < sizeof pairs / sizeof pairs[0]; i++) {
        bool m = uf_union(uf, pairs[i][0], pairs[i][1]);
        printf("union(%d,%d) merged=%d count=%d\n", pairs[i][0], pairs[i][1], m, uf->count);
    }
    assert(uf_connected(uf, 4, 9));
    assert(uf_size(uf, 5) == 10);
    assert(uf->count == 1);
    assert(!uf_union(uf, 2, 2));
    int id = uf_make_set(uf);
    assert(id == 10 && uf->count == 2 && !uf_connected(uf, 0, 10));
    uf_free(uf);
    puts("all C checks passed");
    return 0;
}
#endif
```

Build/run: `gcc -std=c11 -Wall -Wextra -fsanitize=address,undefined -DUF_MAIN uf.c -o uf && ./uf`

**C-specific explanation**

* You manage lifetime: every `uf_new` needs one `uf_free`. The sanitizer flags leaks and out-of-bounds indexing.
* `realloc` may move the block: never keep an old pointer to `parent` across `uf_make_set`.
* No bounds checking: an id `>= n` is undefined behavior. Validate at your API boundary.
* `int` is 4 bytes here, so the structure is half the size of the Go/Rust versions.

### 8.3.1 C: fused `int32` encoding (Section 3.4)

The smallest, fastest variant: one array, negative values mark roots.

```c
#include <stdint.h>
#include <stdlib.h>

/* p[x] < 0  => root with set size -p[x];   p[x] >= 0 => parent id */
typedef struct { int32_t *p; int32_t n; int32_t count; } FusedUF;

FusedUF *fuf_new(int32_t n) {
    FusedUF *u = malloc(sizeof *u);
    if (!u) return NULL;
    u->p = malloc((size_t)(n > 0 ? n : 1) * sizeof *u->p);
    if (!u->p) { free(u); return NULL; }
    for (int32_t i = 0; i < n; i++) u->p[i] = -1;
    u->n = n; u->count = n;
    return u;
}
void fuf_free(FusedUF *u) { if (u) { free(u->p); free(u); } }

int32_t fuf_find(FusedUF *u, int32_t x) {
    int32_t *p = u->p;
    while (p[x] >= 0) {
        int32_t par = p[x];
        if (p[par] < 0) return par;   /* parent is the root: done */
        p[x] = p[par];                /* halve: point at grandparent */
        x = p[par];                   /* p[par] is grandparent (or was just read) */
    }
    return x;                          /* x itself is a root */
}

int fuf_union(FusedUF *u, int32_t a, int32_t b) {
    int32_t ra = fuf_find(u, a), rb = fuf_find(u, b);
    if (ra == rb) return 0;
    if (u->p[ra] > u->p[rb]) { int32_t t = ra; ra = rb; rb = t; } /* more negative = bigger */
    u->p[ra] += u->p[rb];  /* sizes add (as negatives) */
    u->p[rb]  = ra;        /* rb now points at ra */
    u->count--;
    return 1;
}
int32_t fuf_size(FusedUF *u, int32_t x) { return -u->p[fuf_find(u, x)]; }
```

Careful with the halving step here: the root's slot holds a negative size, so you must **never** read it as a parent index. That is exactly why the `if (p[par] < 0) return par;` guard exists.

---

# 9. Real-World Code

All snippets below reuse the core `UnionFind` type from Section 8 of the same language (put them in the same package / crate / translation unit).

## 9.1 Kruskal's minimum spanning tree

Idea: sort edges by weight; take an edge iff it connects two different components. `Union` returning a boolean *is* the cycle test.

```text
Graph (4 nodes)                       Sorted edges: (2,3,4) (0,3,5) (0,2,6) (0,1,10) (1,3,15)

   0 ---10--- 1                       take (2,3):4    components: {0} {1} {2,3}
   | \        |                       take (0,3):5    components: {0,2,3} {1}
   6  5      15                       skip (0,2):6    Find(0)==Find(2)  -> cycle!
   |   \      |                       take (0,1):10   components: {0,1,2,3}   n-1 edges: STOP
   2 -4- 3 ---+                       MST weight = 4+5+10 = 19
```

### Go

```go
package main

import (
	"fmt"
	"sort"
)

type Edge struct{ U, V, W int }

// Kruskal returns the MST weight, the chosen edges, and whether the graph is connected.
func Kruskal(n int, edges []Edge) (total int, mst []Edge, ok bool) {
	sorted := make([]Edge, len(edges))
	copy(sorted, edges) // do not mutate the caller's slice
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].W < sorted[j].W })

	uf := NewUnionFind(n)
	for _, e := range sorted {
		if uf.Union(e.U, e.V) { // true => joins two components => not a cycle
			total += e.W
			mst = append(mst, e)
			if len(mst) == n-1 {
				break // a spanning tree has exactly n-1 edges
			}
		}
	}
	return total, mst, n <= 1 || len(mst) == n-1
}

func kruskalDemo() {
	edges := []Edge{{0, 1, 10}, {0, 2, 6}, {0, 3, 5}, {1, 3, 15}, {2, 3, 4}}
	total, mst, ok := Kruskal(4, edges)
	fmt.Println(total, mst, ok) // 19 [{2 3 4} {0 3 5} {0 1 10}] true
}
```

### Rust

```rust
#[derive(Clone, Copy, Debug)]
pub struct Edge {
    pub u: usize,
    pub v: usize,
    pub w: u64,
}

/// Returns (total weight, chosen edges), or None if the graph is disconnected.
pub fn kruskal(n: usize, edges: &[Edge]) -> Option<(u64, Vec<Edge>)> {
    let mut sorted = edges.to_vec(); // copy: caller keeps their slice untouched
    sorted.sort_by_key(|e| e.w);

    let mut uf = UnionFind::new(n);
    let mut total = 0u64;
    let mut mst = Vec::with_capacity(n.saturating_sub(1));
    for e in sorted {
        if uf.union(e.u, e.v) {
            total += e.w;
            mst.push(e);
            if mst.len() + 1 == n {
                break;
            }
        }
    }
    if n <= 1 || mst.len() + 1 == n { Some((total, mst)) } else { None }
}

#[cfg(test)]
mod kruskal_tests {
    use super::*;
    #[test]
    fn small_graph() {
        let e = [
            Edge { u: 0, v: 1, w: 10 }, Edge { u: 0, v: 2, w: 6 }, Edge { u: 0, v: 3, w: 5 },
            Edge { u: 1, v: 3, w: 15 }, Edge { u: 2, v: 3, w: 4 },
        ];
        let (total, mst) = kruskal(4, &e).expect("connected");
        assert_eq!(total, 19);
        assert_eq!(mst.len(), 3);
        assert!(kruskal(5, &e).is_none()); // node 4 is isolated
    }
}
```

### C

```c
typedef struct { int u, v, w; } Edge;

static int cmp_edge(const void *a, const void *b) {
    const Edge *x = a, *y = b;
    return (x->w > y->w) - (x->w < y->w);   /* never subtract: overflow-safe */
}

/* Sorts `edges` IN PLACE. `out` must have room for n-1 edges.
   Returns total weight (>= 0 assumed), -1 if disconnected, -2 on OOM. */
long long kruskal(int n, Edge *edges, size_t m, Edge *out, int *out_len) {
    qsort(edges, m, sizeof *edges, cmp_edge);
    UnionFind *uf = uf_new(n);
    if (!uf) return -2;
    long long total = 0;
    int taken = 0;
    for (size_t i = 0; i < m && taken < n - 1; i++) {
        if (uf_union(uf, edges[i].u, edges[i].v)) {
            out[taken++] = edges[i];
            total += edges[i].w;
        }
    }
    uf_free(uf);
    *out_len = taken;
    return (n <= 1 || taken == n - 1) ? total : -1;
}
```

**Complexity:** sorting `O(E log E)` dominates; the Union-Find part is `O(E α(V))`.

## 9.2 Grid islands, online (streaming land cells)

Problem: an `m×n` grid starts as water. Cells turn into land one at a time; after each addition report the number of islands (4-directional adjacency).

```text
m=3, n=3, positions: (0,0) (0,1) (1,2) (2,1)

add (0,0):   1 . .     islands=1
             . . .
             . . .

add (0,1):   1 1 .     islands=1   (merged with neighbor)
             . . .
             . . .

add (1,2):   1 1 .     islands=2
             . . 1
             . . .

add (2,1):   1 1 .     islands=3   -> answer [1,1,2,3]
             . . 1
             . 1 .
```

Trick: flatten `(r,c)` to `id = r*n + c`. Track `islands` yourself, since only *land* cells count.

```go
func NumIslands2(m, n int, positions [][2]int) []int {
	uf := NewUnionFind(m * n)
	land := make([]bool, m*n)
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	islands := 0
	res := make([]int, 0, len(positions))

	for _, p := range positions {
		r, c := p[0], p[1]
		id := r*n + c
		if land[id] { // duplicate position: state unchanged
			res = append(res, islands)
			continue
		}
		land[id] = true
		islands++ // provisional: new island
		for _, d := range dirs {
			nr, nc := r+d[0], c+d[1]
			if nr < 0 || nr >= m || nc < 0 || nc >= n {
				continue
			}
			nid := nr*n + nc
			if land[nid] && uf.Union(id, nid) {
				islands-- // merged two islands into one
			}
		}
		res = append(res, islands)
	}
	return res
}
```

A DFS/BFS recount after each addition costs `O(mn)` each; Union-Find makes each addition `O(α)`, which is why this is *the* classic online example.

## 9.3 Grouping by keys (strings, emails, arbitrary types)

Union-Find wants dense integer ids. Real data has strings. Pattern: **map key → id on first sight**, then use the integer structure.

### Go: merge accounts that share an email

```go
import "sort"

// accounts[i] = {name, email1, email2, ...}
func AccountsMerge(accounts [][]string) [][]string {
	uf := NewUnionFind(0) // growable
	id := map[string]int{}
	name := map[string]string{}
	emails := []string{} // id -> email

	get := func(e string) int {
		if v, ok := id[e]; ok {
			return v
		}
		v := uf.MakeSet()
		id[e] = v
		emails = append(emails, e)
		return v
	}

	for _, acc := range accounts {
		if len(acc) < 2 {
			continue // no emails: nothing to merge on
		}
		first := get(acc[1])
		name[acc[1]] = acc[0]
		for _, e := range acc[2:] {
			uf.Union(first, get(e))
			name[e] = acc[0]
		}
	}

	groups := map[int][]string{}
	for i, e := range emails {
		r := uf.Find(i)
		groups[r] = append(groups[r], e)
	}
	out := make([][]string, 0, len(groups))
	for _, g := range groups { // NOTE: map iteration order is random
		sort.Strings(g)
		out = append(out, append([]string{name[g[0]]}, g...))
	}
	return out
}
```

### Rust: a generic keyed wrapper

```rust
use std::collections::HashMap;
use std::hash::Hash;

pub struct KeyedUnionFind<K: Hash + Eq + Clone> {
    ids: HashMap<K, usize>,
    keys: Vec<K>, // id -> key (for reporting groups)
    uf: UnionFind,
}

impl<K: Hash + Eq + Clone> KeyedUnionFind<K> {
    pub fn new() -> Self {
        Self { ids: HashMap::new(), keys: Vec::new(), uf: UnionFind::new(0) }
    }

    fn id(&mut self, k: &K) -> usize {
        if let Some(&i) = self.ids.get(k) {
            return i;
        }
        let i = self.uf.make_set();
        self.ids.insert(k.clone(), i);
        self.keys.push(k.clone());
        i
    }

    pub fn union(&mut self, a: &K, b: &K) -> bool {
        let (ia, ib) = (self.id(a), self.id(b));
        self.uf.union(ia, ib)
    }

    pub fn connected(&mut self, a: &K, b: &K) -> bool {
        let (ia, ib) = (self.id(a), self.id(b));
        self.uf.connected(ia, ib)
    }

    pub fn groups(&mut self) -> Vec<Vec<K>> {
        let mut by_root: HashMap<usize, Vec<K>> = HashMap::new();
        for i in 0..self.keys.len() {
            let r = self.uf.find(i);
            by_root.entry(r).or_default().push(self.keys[i].clone());
        }
        by_root.into_values().collect()
    }
}
```

**Design notes:** hashing costs far more than the Union-Find itself. If keys are known up front (offline), **compress coordinates once** (sort + dedupe, or one pass with a map), then run the dense-array structure. `K: Clone` is needed because both the map and the vector own a copy of the key; use `Rc<str>`/interned ids if keys are large.

## 9.4 Cycle detection in an undirected graph

```go
func HasCycle(n int, edges [][2]int) bool {
	uf := NewUnionFind(n)
	for _, e := range edges {
		if !uf.Union(e[0], e[1]) {
			return true // endpoints already connected: this edge closes a cycle
		}
	}
	return false
}
```

Same code with different meaning gives: "is this graph a tree?" (`no cycle` and `Count()==1`), and "which edge is redundant?" (return the first `e` where `Union` fails).

---

# 10. Manipulations and Advanced Variants

## 10.1 Per-component aggregates (sum / min / max / anything associative)

Store the aggregate **at the root**; combine it whenever two roots link.

```go
type AggUF struct {
	parent, size []int
	sum          []int64
	max          []int
	count        int
}

func NewAggUF(vals []int) *AggUF {
	n := len(vals)
	a := &AggUF{parent: make([]int, n), size: make([]int, n), sum: make([]int64, n), max: make([]int, n), count: n}
	for i, v := range vals {
		a.parent[i], a.size[i], a.sum[i], a.max[i] = i, 1, int64(v), v
	}
	return a
}

func (a *AggUF) Find(x int) int {
	for a.parent[x] != x {
		a.parent[x] = a.parent[a.parent[x]]
		x = a.parent[x]
	}
	return x
}

func (a *AggUF) Union(x, y int) bool {
	rx, ry := a.Find(x), a.Find(y)
	if rx == ry {
		return false
	}
	if a.size[rx] < a.size[ry] {
		rx, ry = ry, rx
	}
	a.parent[ry] = rx
	a.size[rx] += a.size[ry]
	a.sum[rx] += a.sum[ry] // combine aggregates into the surviving root
	if a.max[ry] > a.max[rx] {
		a.max[rx] = a.max[ry]
	}
	a.count--
	return true
}

func (a *AggUF) SumOf(x int) int64 { return a.sum[a.Find(x)] }
func (a *AggUF) MaxOf(x int) int   { return a.max[a.Find(x)] }
```

Rule: the aggregate must be **mergeable** (`agg(A ∪ B) = f(agg(A), agg(B))`). Sum, min, max, gcd, xor, count-of-property all qualify. Median does not.

## 10.2 Enumerating the members of a set (circular-list splice)

Keep a `next[]` array forming a **circular linked list per set**. Merging two circular lists is a single swap:

```text
Before:  set A:  a -> a2 -> a3 -> (back to a)
         set B:  b -> b2 -> (back to b)

swap(next[ra], next[rb]):

After:   a -> b2 -> b -> a2 -> a3 -> (back to a)   one circular list, O(1)
```

```go
// next[i] = i initially. Call inside Union, after choosing roots ra, rb:
next[ra], next[rb] = next[rb], next[ra]

// Enumerate the set containing x:
func members(next []int, x int) []int {
	out := []int{x}
	for y := next[x]; y != x; y = next[y] {
		out = append(out, y)
	}
	return out
}
```

Cost: O(1) to maintain, O(|set|) to list. Alternative: recompute groups in one pass via `Find` for all elements (as in `AccountsMerge`).

## 10.3 Weighted (potential) Union-Find: relations, not just membership

Now each element stores a **numeric relation to its parent**, so you can answer "how much is `a` more than `b`?" when they are connected.

Define `diff[x] = value(x) − value(parent[x])`. After compression, `diff[x] = value(x) − value(root)`.

```text
Facts: val[1]-val[0]=5,  val[2]-val[1]=3       =>  val[2]-val[0]=8  (derived, never stated)

        0                 diff[0] = 0 (root)
       / \
      1   2               diff[1] = 5   (val1 - val0)
                          diff[2] = 8   (val2 - val0)   Diff(2,1) = 8 - 5 = 3
```

**Find with compression must add up offsets along the path:**

```text
Find(x):  p = parent[x]
          root = Find(p)          # now diff[p] is relative to root
          diff[x] += diff[p]      # x's offset to root = (x->p) + (p->root)
          parent[x] = root
```

**Union(a, b, d)** asserts `val[a] − val[b] = d`. After finding roots, `diff[a] = val[a]−val[ra]` and `diff[b] = val[b]−val[rb]`, so:

```text
val[ra] − val[rb] = (val[b] + d − diff[a]) − (val[b] − diff[b]) = d − diff[a] + diff[b]
```

If both are already in the same set, the fact is **consistent iff** `diff[a] − diff[b] == d`.

### Go

```go
type WeightedUF struct {
	parent []int
	size   []int
	diff   []int64 // diff[x] = val[x] - val[parent[x]]; roots keep 0
}

func NewWeightedUF(n int) *WeightedUF {
	w := &WeightedUF{parent: make([]int, n), size: make([]int, n), diff: make([]int64, n)}
	for i := range w.parent {
		w.parent[i], w.size[i] = i, 1
	}
	return w
}

// Find returns the root and leaves diff[x] relative to that root.
// Recursion depth is <= log2(n) thanks to union by size.
func (w *WeightedUF) Find(x int) int {
	if w.parent[x] == x {
		return x
	}
	p := w.parent[x]
	root := w.Find(p)
	w.diff[x] += w.diff[p] // diff[p] is now relative to root
	w.parent[x] = root
	return root
}

// Union records val[a]-val[b]=d. Returns false only if it contradicts earlier facts.
func (w *WeightedUF) Union(a, b int, d int64) bool {
	ra, rb := w.Find(a), w.Find(b)
	if ra == rb {
		return w.diff[a]-w.diff[b] == d
	}
	delta := d - w.diff[a] + w.diff[b] // val[ra] - val[rb]
	if w.size[ra] < w.size[rb] {
		w.parent[rb], w.diff[rb] = ra, -delta // val[rb]-val[ra]
		w.size[ra] += w.size[rb]
	} else {
		w.parent[ra], w.diff[ra] = rb, delta
		w.size[rb] += w.size[ra]
	}
	return true
}

// Diff returns val[a]-val[b] if the two are connected.
func (w *WeightedUF) Diff(a, b int) (int64, bool) {
	if w.Find(a) != w.Find(b) {
		return 0, false
	}
	return w.diff[a] - w.diff[b], true
}
```

Example: `Union(1,0,5)`; `Union(2,1,3)`; `Diff(2,0)` → `8`; `Union(2,0,7)` → `false` (contradiction).

### Rust

```rust
pub struct WeightedUf {
    parent: Vec<usize>,
    size: Vec<usize>,
    diff: Vec<i64>, // diff[x] = val[x] - val[parent[x]]
}

impl WeightedUf {
    pub fn new(n: usize) -> Self {
        Self { parent: (0..n).collect(), size: vec![1; n], diff: vec![0; n] }
    }

    pub fn find(&mut self, x: usize) -> usize {
        let p = self.parent[x];
        if p == x {
            return x;
        }
        let root = self.find(p);
        self.diff[x] += self.diff[p];
        self.parent[x] = root;
        root
    }

    /// Records val[a] - val[b] = d. Returns false if it contradicts earlier facts.
    pub fn union(&mut self, a: usize, b: usize, d: i64) -> bool {
        let (ra, rb) = (self.find(a), self.find(b));
        if ra == rb {
            return self.diff[a] - self.diff[b] == d;
        }
        let delta = d - self.diff[a] + self.diff[b]; // val[ra] - val[rb]
        if self.size[ra] < self.size[rb] {
            self.parent[rb] = ra;
            self.diff[rb] = -delta;
            self.size[ra] += self.size[rb];
        } else {
            self.parent[ra] = rb;
            self.diff[ra] = delta;
            self.size[rb] += self.size[ra];
        }
        true
    }

    pub fn diff_between(&mut self, a: usize, b: usize) -> Option<i64> {
        if self.find(a) != self.find(b) { None } else { Some(self.diff[a] - self.diff[b]) }
    }
}
```

### C

```c
typedef struct { int *parent, *size; long long *diff; int n; } WUF;

WUF *wuf_new(int n) {
    WUF *w = malloc(sizeof *w);
    if (!w) return NULL;
    w->parent = malloc((size_t)n * sizeof(int));
    w->size   = malloc((size_t)n * sizeof(int));
    w->diff   = calloc((size_t)n, sizeof(long long));   /* zeroed */
    if (!w->parent || !w->size || !w->diff) { free(w->parent); free(w->size); free(w->diff); free(w); return NULL; }
    w->n = n;
    for (int i = 0; i < n; i++) { w->parent[i] = i; w->size[i] = 1; }
    return w;
}

int wuf_find(WUF *w, int x) {
    if (w->parent[x] == x) return x;
    int p = w->parent[x];
    int root = wuf_find(w, p);
    w->diff[x] += w->diff[p];
    w->parent[x] = root;
    return root;
}

/* records val[a]-val[b]=d; returns 0 on contradiction, 1 otherwise */
int wuf_union(WUF *w, int a, int b, long long d) {
    int ra = wuf_find(w, a), rb = wuf_find(w, b);
    if (ra == rb) return (w->diff[a] - w->diff[b]) == d;
    long long delta = d - w->diff[a] + w->diff[b];       /* val[ra]-val[rb] */
    if (w->size[ra] < w->size[rb]) { w->parent[rb] = ra; w->diff[rb] = -delta; w->size[ra] += w->size[rb]; }
    else                           { w->parent[ra] = rb; w->diff[ra] =  delta; w->size[rb] += w->size[ra]; }
    return 1;
}
```

**Multiplicative version** (currency rates, "a / b = 2.0"): replace `+` with `×`, `−` with `÷`, and the identity `0` with `1`: store `ratio[x] = val[x]/val[parent]`, in Find do `ratio[x] *= ratio[p]`, and in Union set `ratio[ra] = d * ratio[b] / ratio[a]`. Beware floating-point drift; prefer exact rationals or logarithms when possible. **Additive overflow:** long chains of large diffs can overflow `i64`; check bounds if inputs are adversarial.

## 10.4 Parity Union-Find (bipartite / "two camps" constraints)

Special case of the weighted version with XOR. `par[x]` = parity of `x` relative to its parent. Use it for "these two must be **different**" (enemies, 2-coloring, odd-cycle detection).

```go
type ParityUF struct {
	parent, size []int
	par          []uint8 // par[x] = color(x) XOR color(parent[x])
}

func NewParityUF(n int) *ParityUF {
	p := &ParityUF{parent: make([]int, n), size: make([]int, n), par: make([]uint8, n)}
	for i := range p.parent {
		p.parent[i], p.size[i] = i, 1
	}
	return p
}

func (u *ParityUF) Find(x int) int {
	if u.parent[x] == x {
		return x
	}
	p := u.parent[x]
	r := u.Find(p)
	u.par[x] ^= u.par[p] // parity to root = (x->p) xor (p->root)
	u.parent[x] = r
	return r
}

// Differ records "a and b must have different colors".
// Returns false if that contradicts earlier facts (an odd cycle exists).
func (u *ParityUF) Differ(a, b int) bool {
	ra, rb := u.Find(a), u.Find(b)
	if ra == rb {
		return u.par[a]^u.par[b] == 1
	}
	if u.size[ra] < u.size[rb] {
		ra, rb = rb, ra // formula below is symmetric, so swapping is safe
	}
	u.parent[rb] = ra
	u.par[rb] = u.par[a] ^ u.par[b] ^ 1
	u.size[ra] += u.size[rb]
	return true
}
```

Bipartite test of a graph: `for each edge (u,v): if !Differ(u,v) → not bipartite`.

## 10.5 Rollback Union-Find (undo the last merges)

**Goal:** support `Snapshot()` and `Rollback(snapshot)`.
**Requirement:** **no path compression** (compression rewrites many pointers, so undoing it would need logging all of them). Keep **union by size** so Find stays `O(log n)` worst case.

**Undo log:** push the *child root* (the one that was attached) on every real merge. To undo: pop child `c`, let `p = parent[c]`, then `size[p] -= size[c]`, `parent[c] = c`, `count++`.

```text
Before union(ra, rb):        After (rb attached, history=[..., rb]):

  ra          rb                    ra
 / \          |                    / | \
..  ..       ..                  .. .. rb
                                        |
                                       ..
Rollback: pop rb -> parent[rb]=rb, size[ra]-=size[rb]  => original picture restored
```

### Go

```go
type RollbackUF struct {
	parent, size []int
	history      []int // child roots that were attached, in order
	count        int
}

func NewRollbackUF(n int) *RollbackUF {
	r := &RollbackUF{parent: make([]int, n), size: make([]int, n), count: n}
	for i := range r.parent {
		r.parent[i], r.size[i] = i, 1
	}
	return r
}

func (r *RollbackUF) Find(x int) int { // NO compression
	for r.parent[x] != x {
		x = r.parent[x]
	}
	return x
}

func (r *RollbackUF) Union(a, b int) bool {
	ra, rb := r.Find(a), r.Find(b)
	if ra == rb {
		return false // nothing pushed: history only records real merges
	}
	if r.size[ra] < r.size[rb] {
		ra, rb = rb, ra
	}
	r.parent[rb] = ra
	r.size[ra] += r.size[rb]
	r.count--
	r.history = append(r.history, rb)
	return true
}

func (r *RollbackUF) Snapshot() int { return len(r.history) }
func (r *RollbackUF) Count() int    { return r.count }

func (r *RollbackUF) Rollback(to int) {
	for len(r.history) > to {
		c := r.history[len(r.history)-1]
		r.history = r.history[:len(r.history)-1]
		p := r.parent[c]
		r.size[p] -= r.size[c]
		r.parent[c] = c
		r.count++
	}
}
```

### Rust

```rust
pub struct RollbackUf {
    parent: Vec<usize>,
    size: Vec<usize>,
    history: Vec<usize>,
    count: usize,
}

impl RollbackUf {
    pub fn new(n: usize) -> Self {
        Self { parent: (0..n).collect(), size: vec![1; n], history: Vec::new(), count: n }
    }

    pub fn find(&self, mut x: usize) -> usize { // &self: no mutation, no compression
        while self.parent[x] != x {
            x = self.parent[x];
        }
        x
    }

    pub fn union(&mut self, a: usize, b: usize) -> bool {
        let (mut ra, mut rb) = (self.find(a), self.find(b));
        if ra == rb {
            return false;
        }
        if self.size[ra] < self.size[rb] {
            std::mem::swap(&mut ra, &mut rb);
        }
        self.parent[rb] = ra;
        self.size[ra] += self.size[rb];
        self.count -= 1;
        self.history.push(rb);
        true
    }

    pub fn snapshot(&self) -> usize { self.history.len() }
    pub fn count(&self) -> usize { self.count }

    pub fn rollback(&mut self, to: usize) {
        while self.history.len() > to {
            let c = self.history.pop().expect("history is non-empty inside the loop");
            let p = self.parent[c];
            self.size[p] -= self.size[c];
            self.parent[c] = c;
            self.count += 1;
        }
    }
}
```

Rust bonus: `find(&self)` is naturally read-only here, so many readers can share the structure between mutations.

### C

```c
typedef struct { int *parent, *size, *hist; int hlen, count, n; } RbUF;

RbUF *rb_new(int n) {
    RbUF *r = malloc(sizeof *r);
    if (!r) return NULL;
    int cap = n > 0 ? n : 1;
    r->parent = malloc((size_t)cap * sizeof(int));
    r->size   = malloc((size_t)cap * sizeof(int));
    r->hist   = malloc((size_t)cap * sizeof(int));   /* at most n-1 real merges */
    if (!r->parent || !r->size || !r->hist) { free(r->parent); free(r->size); free(r->hist); free(r); return NULL; }
    r->n = n; r->count = n; r->hlen = 0;
    for (int i = 0; i < n; i++) { r->parent[i] = i; r->size[i] = 1; }
    return r;
}
int rb_find(const RbUF *r, int x) { while (r->parent[x] != x) x = r->parent[x]; return x; }

int rb_union(RbUF *r, int a, int b) {
    int ra = rb_find(r, a), rb = rb_find(r, b);
    if (ra == rb) return 0;
    if (r->size[ra] < r->size[rb]) { int t = ra; ra = rb; rb = t; }
    r->parent[rb] = ra; r->size[ra] += r->size[rb]; r->count--;
    r->hist[r->hlen++] = rb;
    return 1;
}
int  rb_snapshot(const RbUF *r) { return r->hlen; }
void rb_rollback(RbUF *r, int to) {
    while (r->hlen > to) {
        int c = r->hist[--r->hlen];
        int p = r->parent[c];
        r->size[p] -= r->size[c];
        r->parent[c] = c;
        r->count++;
    }
}
void rb_free(RbUF *r) { if (r) { free(r->parent); free(r->size); free(r->hist); free(r); } }
```

## 10.6 Offline dynamic connectivity (edges appear *and disappear*)

Union-Find cannot delete edges directly. But if you know **all events in advance** (offline), you can build a **segment tree over time**: each edge is alive during a time interval `[start, end)`; insert it into the O(log T) tree nodes that cover its interval. Then DFS the tree: entering a node, **Union** its edges; at a leaf, answer the query for that time; leaving, **Rollback**.

```text
Time axis T = 4:      t=0   t=1   t=2   t=3
Edge (u,v) alive [1,3):     [-----------)
                             covers t=1,t=2

Segment tree over [0,4):
                     [0,4)
                    /     \
               [0,2)       [2,4)
               /   \       /   \
           [0,1) [1,2)  [2,3) [3,4)
                  ^        ^
        edge stored here (two nodes cover exactly [1,3))

DFS: enter node -> union its edges -> recurse -> ROLLBACK on exit.
Every edge is added and undone O(log T) times.
```

```go
type TimedEdge struct{ U, V, Start, End int } // active during times [Start, End)

// OfflineComponents returns, for each time t in [0,T), the number of components.
func OfflineComponents(n, T int, edges []TimedEdge) []int {
	if T <= 0 {
		return nil
	}
	uf := NewRollbackUF(n)
	seg := make([][][2]int, 4*T)

	var add func(node, l, r, ql, qr int, e [2]int)
	add = func(node, l, r, ql, qr int, e [2]int) {
		if qr <= l || r <= ql {
			return
		}
		if ql <= l && r <= qr {
			seg[node] = append(seg[node], e)
			return
		}
		m := (l + r) / 2
		add(2*node, l, m, ql, qr, e)
		add(2*node+1, m, r, ql, qr, e)
	}
	for _, e := range edges {
		add(1, 0, T, e.Start, e.End, [2]int{e.U, e.V})
	}

	ans := make([]int, T)
	var dfs func(node, l, r int)
	dfs = func(node, l, r int) {
		snap := uf.Snapshot()
		for _, e := range seg[node] {
			uf.Union(e[0], e[1])
		}
		if r-l == 1 {
			ans[l] = uf.Count()
		} else {
			m := (l + r) / 2
			dfs(2*node, l, m)
			dfs(2*node+1, m, r)
		}
		uf.Rollback(snap) // undo everything this node added
	}
	dfs(1, 0, T)
	return ans
}
```

Total cost `O(E log T · log n)`. Recursion depth is `O(log T)`.

## 10.7 "Deleting" an element (virtual-node trick)

To remove or *move* element `x` out of its set: leave the old id in place as a dead placeholder, allocate a **fresh id** via `MakeSet()`, and remap `x → newId` in your key map. The old node still occupies its set (counting toward sizes), so keep a separate `alive[]` count if sizes matter.

## 10.8 Concurrency

`Find` **mutates** (compression), so two "readers" calling `Find` at once is a data race.

| Language | What happens / what to do                                                                 |
| -------- | ----------------------------------------------------------------------------------------- |
| Go       | Race detector (`go test -race`) will flag it. Guard with `sync.Mutex` (an `RWMutex` read lock is *not* enough while `Find` writes), or use a compression-free `Find` under `RLock`. |
| Rust     | `Find(&mut self)` makes shared concurrent use a **compile error**. Use `Mutex<UnionFind>`, or a read-only find on `&self`, or a lock-free design with `AtomicUsize` parents and CAS (Anderson–Woll style). |
| C        | No protection. Use a mutex, or lock-free CAS on `parent[]` (`<stdatomic.h>`); tricky to get right. |

## 10.9 Other variants worth knowing exist

| Variant                     | Idea                                                                |
| --------------------------- | ------------------------------------------------------------------- |
| Randomized linking          | link by random priority instead of size: same α bounds in expectation |
| Persistent Union-Find       | persistent array for `parent`; O(log n) per access                   |
| Union-Find on a grid        | flatten `(r,c)` to one index; 4- or 8-neighbor unions                 |
| Offline LCA (Tarjan)        | DFS + union child into parent; answer queries when both visited       |
| Component labeling in images| two-pass scan + Union-Find on provisional labels                      |
| Interval "next free slot"   | `parent[i]=i+1` on use; Find gives next free index (a one-directional UF) |

---

# 11. Go vs Rust vs C

| Aspect                        | Go                                   | Rust                                    | C                                       |
| ----------------------------- | ------------------------------------ | --------------------------------------- | --------------------------------------- |
| Storage of parents            | `[]int` (8 B/elt)                    | `Vec<usize>` (8 B/elt)                  | `int*` / `int32_t*` (4 B/elt)           |
| Who frees memory              | garbage collector                    | ownership + `Drop`                      | you (`free`)                            |
| Bad index                     | runtime panic                        | runtime panic (bounds check)            | **undefined behavior**                  |
| Growable                      | `append` (may reallocate)            | `Vec::push`                             | `realloc` (may move: fix pointers)      |
| Find that compresses          | pointer receiver, just works         | needs `&mut self`                       | pointer arg                             |
| Read-only find                | separate method, manual              | `&self` version, type-enforced          | `const` pointer                         |
| Concurrency safety            | race detector (runtime)              | compile-time (`&mut` + `Send/Sync`)     | none                                    |
| Recursion depth danger        | goroutine stacks grow                | main thread ~8 MB; spawned threads default 2 MB | ~8 MB default process stack |
| Absence of a value            | `nil`, ok-flags (`(int,bool)`)       | `Option<T>`                             | sentinel values (`-1`)                  |
| Generic keys                  | map + int ids (or generics)          | `HashMap<K,usize>` + trait bounds       | you write hashing / use ids             |
| Overflow behavior             | wraps silently                       | panics in debug, wraps in release       | signed overflow is UB                   |

Implementation-complexity summary: **C** is smallest but unforgiving; **Go** is the shortest safe code; **Rust** is the same length as Go but pushes concurrency and aliasing correctness onto the compiler.

---

# 12. Common Mistakes (Failure Analysis)

| # | Mistake | Why it looks reasonable | Minimal counterexample | How to catch it |
|---|---------|-------------------------|------------------------|------------------|
| 1 | `parent[a] = b` without finding roots | "union means link a and b" | `union(0,1); union(0,2)` → `parent[0]=1` then overwritten by `parent[0]=2`; `connected(0,1)` is now false | Test: after any sequence, compare with naive oracle |
| 2 | Comparing sizes of `a`,`b` instead of `ra`,`rb` | variable names look alike | `size[a]` is stale when `a` is a non-root | Assert `parent[r]==r` before reading `size[r]` |
| 3 | No `ra == rb` check | "linking twice is harmless" | `union(0,1); union(0,1)` → self-link `parent[0]=0` and `size[0]` doubles to 4, count wrongly decremented | Return-value test: second union must return false |
| 4 | Checking `parent[a] == parent[b]` for connectivity | "same parent = same set" | siblings-of-cousins: nodes at different depths in one tree have different parents | Always compare **roots** |
| 5 | Recursive Find without weighting on a chain | "recursion is cleaner" | 10^6-element chain: deep recursion overflows C/Rust-thread stacks | Use iterative halving |
| 6 | Path compression with rollback | "faster is better" | Compression rewrites unlogged pointers; rollback cannot restore | Use no-compression Find for rollback |
| 7 | 1-based input with `n`-sized arrays | inputs "1..n" | index `n` out of range | Allocate `n+1` or subtract 1 |
| 8 | Assuming deletion works | "just undo the union" | splitting requires knowing which unions formed the set | Use offline segment-tree + rollback (10.6) |
| 9 | Counting components by scanning roots after each op | simple to write | O(n) per query | Maintain `count` |
| 10 | Weighted UF overflow / wrong delta sign | algebra is easy to flip | `Union(1,0,5)` should give `Diff(1,0)=5`, not `-5` | Assert with 2–3 hand-computed cases |
| 11 | Treating `rank` as exact height after compression | "rank = height" | after compression height < rank | Treat rank as **upper bound** only |
| 12 | Sharing one structure across threads | "Find is a read" | compression writes; concurrent Finds race | `go test -race`, Rust type errors, TSan for C |
| 13 | Using Union-Find for directed reachability | "connectivity" sounds alike | `a→b` does not imply `b→a` | Ask: is the relation symmetric? |
| 14 | Fused encoding: halving reads root slot as parent | slot holds a number | `p[root] = -3` used as index → crash | Guard `p[par] < 0` before halving |

Debug recipe: reproduce with the **smallest** sequence, print `parent` and `size` after each step, and check invariants I1–I6 (Section 2.5) at each line.

---

# 13. Pattern Recognition

**Never rely on a "smells like Union-Find" hunch. Justify with these questions:**

1. Is the relation **symmetric and transitive** (an equivalence)?
2. Do groups only **merge** (no splits) in the online version?
3. Are queries of the form **"same group?" / "how many groups?" / "how big is the group?"**
4. Do I need only **membership**, not paths or distances?

If 1–4 hold, Union-Find fits. Typical statements that map to it:

| Problem wording                                   | Union-Find role                          |
| ------------------------------------------------- | ---------------------------------------- |
| "connected components", "friend circles"          | `Count()` after unions                   |
| "redundant edge / does adding this edge form a cycle?" | `Union` returns false                |
| "minimum spanning tree"                           | Kruskal                                  |
| "merge accounts / records sharing a value"        | key→id + groups                          |
| "islands as land appears"                         | online grid unions                       |
| "equalities and inequalities `a==b`, `a!=b`"      | union all `==`, then verify each `!=`    |
| "ratios / offsets between variables"              | weighted UF                              |
| "two camps / enemies / bipartite"                 | parity UF                                |
| "answer queries as edges are removed"             | reverse time (add edges), or offline rollback |
| "next available slot" / "skip used indices"       | `parent[i] = i+1` pointer jumping        |

**Reverse-time trick:** if the problem *removes* edges/cells online but you know the sequence offline, process it **backwards** as *additions*.

---

# 14. Active Recall Questions

Try these on paper first. Answers are intentionally not included.

1. **Tracing.** With union-by-size (ties keep the first root) and path halving, starting from 8 singletons, apply `union(0,1) union(2,3) union(0,2) union(4,5) union(6,7) union(4,6) union(1,5)`. Write the final `parent` array and draw the forest. What is the maximum depth?
2. **Invariant.** After `Find(x)` with path halving, which of I1–I6 could *change*, and why does none of them break?
3. **Complexity.** Give a sequence of `n−1` unions that makes plain Quick-Union (no weighting, `parent[ra]=rb`) build a chain. Then show why the same sequence cannot build a chain with union by size.
4. **Edge cases.** What must `Union(x, x)` return, and what happens to `count` and `size` if your code forgets the `ra == rb` check?
5. **Transfer.** In the weighted UF, why does `diff[root]` need to stay `0`? What breaks if you forget to reset it when a root becomes a child? (Hint: which formula reads `diff[p]`?)
6. **Design.** Why can't a rollback Union-Find use path compression, and what does it use instead to keep Find fast?
7. **Language.** Why does the Rust `find` take `&mut self` while the rollback version's `find` takes `&self`? What would go wrong in Go if two goroutines call `Find` concurrently?

---

# 15. Practice Problems

Solutions withheld. Attempt first; send me your attempt (Go, Rust, or C) and I will review it in **Code Review** mode.

**Beginner**
* *Number of Provinces:* given an `n×n` adjacency matrix `isConnected`, count the provinces (connected components). Then explain what `Count()` gives you for free.
* Also: implement Quick-Find and Quick-Union yourself in your weakest language, and write the oracle test from Section 8.

**Intermediate**
* *Redundant Connection:* a tree with one extra edge is given as an edge list; return the edge that can be removed to leave a tree. State the invariant that makes the answer correct, and say why "the last edge that fails to merge" is the right pick for that statement.
* *Satisfiability of Equality Equations:* strings like `"a==b"`, `"b!=a"`. Decide whether all can hold simultaneously. Justify why two passes are needed.

**Variation (changes a constraint)**
* *Evaluate Division:* given `a/b = 2.0`, `b/c = 3.0`, answer queries `a/c`, `c/a`, `a/x`. Adapt the weighted UF from 10.3 to the multiplicative case; explain what plays the role of the "0" identity and what you do for unknown variables.
* *Harder variant:* edges are added **and removed** at known times; report the component count at each time. Explain why plain Union-Find fails, and design the solution from Sections 10.5–10.6 without looking at the code.

---

# 16. Key Takeaways and Portable Progress Summary

## Key takeaways

1. Union-Find stores a **forest in one array**: `parent[x]`, roots point to themselves.
2. **Union links roots, never arbitrary elements.** Most bugs violate this.
3. **Union by size** bounds tree height by `log₂ n`; **path halving** flattens trees as a side effect of Find. Together: **O(α(n))** amortized, effectively constant.
4. `Union` returning a **bool** unlocks cycle detection, Kruskal, and counting.
5. Roots are the natural home for **aggregates** (size, sum, max) and, in weighted variants, for **offsets/parity** relative to the root.
6. **Rollback** requires dropping compression; combined with a **segment tree over time**, it solves offline dynamic connectivity.
7. **Memory layout is real performance:** 32-bit ids, or the fused negative-size encoding, cut memory 2–4× and improve cache behavior; `Find` is a chain of dependent loads.
8. `Find` mutates: in Rust that shows up in the type (`&mut self`); in Go and C it becomes your concurrency problem.
9. Union-Find answers *membership* questions only: no paths, no distances, no online deletions.

## Portable progress summary (paste into a new conversation)

```text
=== DSA Mentor Progress Snapshot ===
Topic covered: Union-Find (Disjoint Set Union), Go + Rust + C
Topics introduced:  quick-find, quick-union, union by size/rank, path
                    compression (full/halving/splitting), inverse Ackermann,
                    weighted/parity UF, rollback UF, offline dynamic
                    connectivity, Kruskal, grid islands, keyed grouping
Mastery levels (0-4, honest self-rating after attempts):
  Core UF concept ............ [ ]   Weighted / parity ........ [ ]
  Path compression ........... [ ]   Rollback + offline ...... [ ]
  Complexity reasoning ....... [ ]   Kruskal application ...... [ ]
Problems attempted:           (fill in)
Solved independently:         (fill in)
Recurring mistakes:           (fill in, e.g. "forgot roots", "1-based index")
Go-specific gaps:             (fill in)
Rust-specific gaps:           (fill in)
C-specific gaps:              (fill in)
Suggested next step: solve the Beginner + Intermediate problems from Section 15,
                     then revisit Section 14 questions 1, 3 and 5.
```

Mastery is earned by **deriving and implementing without looking**, not by reading. A good self-test: close this file, and write the Go, Rust, or C core from a blank screen, then explain out loud *why* halving does not break the size invariant.
