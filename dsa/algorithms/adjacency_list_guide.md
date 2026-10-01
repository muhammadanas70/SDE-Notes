# Adjacency Lists — The Complete Guide
### Theory → Memory Layout → C / Go / Rust → Algorithms → Hardware → Reverse Engineering

> **Mental anchor:** A graph is a *relation*. An adjacency list is that relation stored as
> **"for each node, the set of nodes it points to."** Everything else — BFS, Dijkstra, CFG
> recovery, call-graph clustering, attack-path search — is a question asked of that structure.

---

## Table of Contents

1. [Graph Foundations](#1-graph-foundations)
2. [The Three Classic Representations](#2-the-three-classic-representations)
3. [Adjacency List Anatomy](#3-adjacency-list-anatomy)
4. [Memory Layout — What the Machine Actually Sees](#4-memory-layout--what-the-machine-actually-sees)
5. [Variants and Design Decisions](#5-variants-and-design-decisions)
6. [C Implementation](#6-c-implementation)
7. [Go Implementation](#7-go-implementation)
8. [Rust Implementation](#8-rust-implementation)
9. [Algorithms on Adjacency Lists](#9-algorithms-on-adjacency-lists)
10. [Complexity, Cache Behavior, Hardware](#10-complexity-cache-behavior-hardware)
11. [Reverse Engineering View](#11-reverse-engineering-view)
12. [Pitfalls and Bugs](#12-pitfalls-and-bugs)
13. [Challenges](#13-challenges)
14. [The Expert Mental Model](#14-the-expert-mental-model)

---

## 1. Graph Foundations

A graph `G = (V, E)`: a set of **vertices** (nodes) `V` and a set of **edges** `E ⊆ V × V`.

| Term | Meaning |
|---|---|
| Directed edge `u → v` | Ordered pair. `u` is the *tail/source*, `v` the *head/target*. |
| Undirected edge `{u, v}` | Unordered pair. Stored as two directed arcs in most implementations. |
| Weighted | Each edge carries a value `w(u,v)` (cost, latency, capacity, probability). |
| Degree `deg(v)` | Undirected: number of incident edges. |
| In-degree / out-degree | Directed: edges arriving / leaving. |
| Self-loop | Edge `v → v`. Undirected self-loop adds **2** to degree by convention. |
| Parallel edges | Multiple edges between same pair → *multigraph*. |
| Simple graph | No self-loops, no parallel edges. |
| Path | Sequence of vertices where each consecutive pair is an edge. |
| Cycle | Path that returns to start. |
| DAG | Directed acyclic graph. Admits a topological order. |
| Connected | Undirected: every pair reachable. |
| Strongly connected | Directed: every pair mutually reachable. |
| SCC | Maximal strongly connected subgraph. |
| Sparse / Dense | `E ≪ V²` / `E ≈ V²`. |

### Handshake lemma (memorize)

```
Undirected:  sum over v of deg(v)            = 2E
Directed:    sum of out-degree = sum of in-degree = E
```

This tells you **exactly how many list entries an adjacency list stores**:
`E` for directed, `2E` for undirected (self-loops aside). Memory is `O(V + E)`.

### Small running example (used throughout)

```
Directed, weighted graph, V = 6, E = 8

            4              1
      0 ---------> 1 ---------> 3 ----+
      |            ^            |      |
      | 2          | 1          | 1    | 5
      v            |            v      v
      2 ---------> 4 <----------+      5
            3      |                   ^
                   +-------------------+
                          2
```

The **canonical edge set**:

```
 u  ->  v    w
 0  ->  1    4
 0  ->  2    2
 1  ->  3    1
 2  ->  4    3
 3  ->  5    5
 4  ->  1    1
 4  ->  5    2
 3  ->  4    1
```

Cycle present: `1 -> 3 -> 4 -> 1`. Node 5 is a sink (out-degree 0). Node 0 is a source (in-degree 0).

---

## 2. The Three Classic Representations

### 2.1 Adjacency Matrix

`M[u][v] = w` if edge exists, else sentinel (0, ∞, or -1).

```
         to:  0   1   2   3   4   5
        +------------------------------
 from 0 |     .   4   2   .   .   .
      1 |     .   .   .   1   .   .
      2 |     .   .   .   .   3   .
      3 |     .   .   .   .   1   5
      4 |     .   1   .   .   .   2
      5 |     .   .   .   .   .   .
```

* Space `Θ(V²)` regardless of `E`.
* Edge query `O(1)`. Neighbor iteration `Θ(V)` even if degree is 2.
* Great for dense graphs, Floyd–Warshall, bitset tricks (transitive closure with word-parallel OR).

### 2.2 Edge List

A flat array of `(u, v, w)` triples.

```
 idx:   0        1        2        3        4        5        6        7
      (0,1,4)  (0,2,2)  (1,3,1)  (2,4,3)  (3,5,5)  (4,1,1)  (4,5,2)  (3,4,1)
```

* Space `Θ(E)`. Perfect for Kruskal (sort by weight), Bellman–Ford, serialization, and as the *input* to build other forms.
* Neighbor query `O(E)`. Useless for traversal.

### 2.3 Adjacency List

```
 0 : [1(4), 2(2)]
 1 : [3(1)]
 2 : [4(3)]
 3 : [5(5), 4(1)]
 4 : [1(1), 5(2)]
 5 : []
```

* Space `Θ(V + E)`.
* Neighbor iteration `Θ(deg(u))` — **exactly the work you need, nothing more**.
* Edge query `O(deg(u))` (or `O(log deg)` if sorted, `O(1)` expected if each row is a hash set).

### 2.4 Head-to-head

| Operation | Matrix | Edge list | Adj list (linked) | Adj list (dynamic array) | CSR (frozen) |
|---|---|---|---|---|---|
| Space | `V²` | `E` | `V + E` | `V + E` | `V + E` |
| `has_edge(u,v)` | `O(1)` | `O(E)` | `O(deg u)` | `O(deg u)` | `O(deg u)` / `O(log deg u)` sorted |
| Iterate neighbors of `u` | `O(V)` | `O(E)` | `O(deg u)` | `O(deg u)` | `O(deg u)` |
| `add_edge` | `O(1)` | `O(1)` | `O(1)` | `O(1)` amortized | rebuild `O(V+E)` |
| `remove_edge` | `O(1)` | `O(E)` | `O(deg u)` | `O(deg u)` | rebuild |
| `add_vertex` | `O(V²)` resize | `O(1)` | `O(1)` | `O(1)` amortized | rebuild |
| Full BFS/DFS | `O(V²)` | `O(V·E)` naive | `O(V+E)` | `O(V+E)` | `O(V+E)` |
| Cache friendliness | row-major good | good sequential | **poor** (pointer chase) | good per row | **best** |

### 2.5 Sparse vs dense — the actual crossover

Assume 4-byte vertex ids, CSR-like list, bit-packed matrix:

```
list bytes   ≈ 4·E  + 4·(V+1)
matrix bytes =  V² / 8

4E ≈ V²/8   =>   E ≈ V²/32
```

So a bit-matrix beats a list on memory only when density exceeds roughly **3%**. With byte-per-cell or weighted cells the threshold drops by 8×–64×, so lists win almost everywhere. Real-world graphs (call graphs, CFGs, web, social, AD) have `E/V` between 1 and ~50 — **hopelessly sparse** relative to `V²`.

---

## 3. Adjacency List Anatomy

Two-level structure:

```
   Level 1: index by vertex id   ->  O(1) lookup of "row"
   Level 2: per-vertex container ->  the neighbors

   +-----+
   |  0  | ---> container{ 1, 2 }
   |  1  | ---> container{ 3 }
   |  2  | ---> container{ 4 }
   |  3  | ---> container{ 5, 4 }
   |  4  | ---> container{ 1, 5 }
   |  5  | ---> container{ }
   +-----+
```

The design space is **what Level 1 is** × **what Level 2 is**:

| Level 1 (vertex index) | Level 2 (neighbor container) | Result |
|---|---|---|
| Array of heads | Singly linked list | Classic textbook, C style |
| Array of dynamic arrays | Dynamic array (`Vec`, slice) | Go/Rust default |
| Array of dynamic arrays | Sorted array | Binary-search `has_edge` |
| Array | Hash set | `O(1)` `has_edge`, high overhead |
| Hash map (`key -> ...`) | Any of the above | Non-dense / string-keyed vertices |
| Single offsets array | Single flat target array | **CSR** (compressed sparse row) |

---

## 4. Memory Layout — What the Machine Actually Sees

### 4.1 Linked-list adjacency in C (x86-64, glibc malloc)

```c
typedef struct Edge { int to; int w; struct Edge *next; } Edge;   // 16 bytes
typedef struct { int n; Edge **head; } Graph;
```

```
Graph g (on heap or stack)
+--------+--------+
| n = 6  | head --+-----------+
+--------+--------+           |
   offset 0        8          v
                        head[] : array of 6 pointers (48 bytes)
                        +-------+-------+-------+-------+-------+-------+
                        | e0a   | e1a   | e2a   | e3a   | e4a   | NULL  |
                        +---+---+---+---+---+---+---+---+---+---+-------+
                            |       |       |       |       |
                            v       v       v       v       v
   Edge node (16 bytes):  +------+------+----------------+
                          | to   | w    | next           |
                          | 4B   | 4B   | 8B             |
                          +------+------+----------------+
                          off 0    off 4   off 8

   Vertex 0 chain:   [to=2,w=2 | next]--> [to=1,w=4 | next=NULL]
                     (head insertion => reversed order of insertion)

   glibc reality: malloc(16) returns a 32-byte chunk
   (16-byte usable min-chunk rules: 8B size header + 8B alignment padding)

        chunk (32 B)
   +----------+---------------------------+----------+
   | prev_sz  | size|flags (8B)           | user 16B | (+ 8B slack for next chunk's prev_size)
   +----------+---------------------------+----------+

   => ~32 bytes of heap per 8 bytes of information. 4x overhead.
   => nodes scattered across heap; each hop is a potential cache miss.
```

### 4.2 `Vec<Vec<(usize,u64)>>` in Rust

```
Graph { adj: Vec<Vec<(usize,u64)>> }

outer Vec header (24 bytes: ptr, cap, len — field ORDER is unspecified by rustc)
+------+------+------+
| ptr  | cap  | len=6|
+--+---+------+------+
   |
   v   heap block: 6 inner Vec headers × 24 B = 144 B, contiguous
   +--------------------+--------------------+--------------------+ ...
   | v0: ptr,cap,len=2  | v1: ptr,cap,len=1  | v2: ptr,cap,len=1  |
   +----------+---------+----------+---------+----------+---------+
              |                    |                    |
              v                    v                    v
        +-----------+          +-----------+        +-----------+
        | (1 , 4)   |          | (3 , 1)   |        | (4 , 3)   |
        | (2 , 2)   |          +-----------+        +-----------+
        +-----------+
        each tuple = 16 B (usize + u64), contiguous within a row

   Two dereferences to reach a neighbor: outer.ptr[u].ptr[i].
   Empty rows: ptr = dangling (align value, e.g. 0x8), cap = 0 -> NO allocation.
```

### 4.3 `[][]Edge` in Go

```
type Edge struct { To int; W int }         // 16 B
Adj [][]Edge                                // slice of slices

Slice header (24 B): { ptr, len, cap }   <- order is FIXED in Go (reflect.SliceHeader)

Adj header --> backing array of 6 slice headers (144 B)
   +--------------------+--------------------+ ...
   | ptr,len=2,cap=2    | ptr,len=1,cap=1    |
   +-------+------------+-------+------------+
           v                    v
        [{1,4},{2,2}]        [{3,1}]     (each backing array is GC-managed,
                                          size-classed by the Go allocator:
                                          16 B, 32 B, 48 B, 64 B, ...)
   append() growth: cap doubles until 256 elements, then ~1.25x + 192 (Go 1.18+ smoothing).
   Empty row: nil slice = {ptr=0,len=0,cap=0}, zero cost, iterates as empty.
```

### 4.4 CSR (Compressed Sparse Row) — the performance form

Two flat arrays. No per-row allocation. No pointers between edges.

```
offsets[V+1]  : offsets[u] .. offsets[u+1] is the slice of `targets` for u
targets[E]    : all neighbor ids, concatenated in source order
weights[E]    : parallel array (optional)

Canonical graph:

 vertex:      0        1     2     3        4        5
 neighbors:  [1, 2]   [3]   [4]   [5, 4]   [1, 5]    []

 offsets :  +---+---+---+---+---+---+---+
            | 0 | 2 | 3 | 4 | 6 | 8 | 8 |          (V+1 = 7 entries)
            +---+---+---+---+---+---+---+
              ^   ^   ^   ^   ^   ^   ^
              u=0 u=1 u=2 u=3 u=4 u=5 sentinel

 targets :  +---+---+---+---+---+---+---+---+
            | 1 | 2 | 3 | 4 | 5 | 4 | 1 | 5 |      (E = 8)
            +---+---+---+---+---+---+---+---+
 index:       0   1   2   3   4   5   6   7
              |___|   |   |   |___|   |___|
               u=0    u=1 u=2  u=3      u=4

 deg(u) = offsets[u+1] - offsets[u]        // O(1), free
 neighbors(u) = targets[offsets[u] .. offsets[u+1]]
```

CSR is the **frozen** form: perfect for read-heavy analysis (BFS over millions of nodes), terrible for incremental mutation. Build it once from an edge list via **counting sort**.

### 4.5 Byte cost per edge — summary

| Form | Bytes/edge (4B ids) | Notes |
|---|---|---|
| Linked list, `malloc` per edge | ~32 | glibc min chunk |
| Linked list, arena/pool allocated | 8–16 | `to` + `next` (+ `w`) |
| `Vec<u32>` rows | ~4 + row overhead (24 B/vertex + alloc header) | amortized doubling wastes up to 50% |
| CSR | 4 (+4 weight) | 4 B/vertex offset |

---

## 5. Variants and Design Decisions

### 5.1 Undirected: store both arcs

```
add_undirected(u, v, w):
    add_arc(u, v, w)
    add_arc(v, u, w)

Self-loop u == v: add ONCE (or you double-count it in traversals).
```

Optionally store a **twin index** (`rev`) so residual-graph algorithms (max-flow) can find the reverse arc in `O(1)`:

```
struct Arc { to: u32, cap: i64, rev: u32 }   // rev = index of the reverse arc in adj[to]
```

### 5.2 Directed graphs need the *transpose* sometimes

`adj[u]` gives out-neighbors. To ask "who points at `v`?" you need `radj` (reverse adjacency). Required by Kosaraju SCC, dominator computation, backward dataflow, and "who calls this function?" queries in RE.

```
adj  : 0->[1,2]  1->[3]  2->[4]  3->[5,4]  4->[1,5]  5->[]
radj : 0->[]     1->[0,4] 2->[0] 3->[1]    4->[2,3]  5->[3,4]
```

### 5.3 Vertex identity: dense ints vs arbitrary keys

If your nodes are strings (function names, hostnames, hashes) **intern them** to `0..V-1` first, keep a `Vec<Key>` for the reverse map, and run all algorithms on integers.

```
 names:  ["main","decrypt","send","recv"]      ids: 0,1,2,3
 map:    HashMap<String, usize>                 name -> id
 rev:    Vec<String>                            id   -> name
```

### 5.4 Duplicate-edge policy

| Policy | Cost | When |
|---|---|---|
| Allow duplicates (multigraph) | `O(1)` add | Call graphs where call-count matters |
| Dedup on insert | `O(deg)` or hash set | Simple graphs |
| Dedup after (sort+unique) | `O(E log E)` once | Bulk build (the CSR path) |

### 5.5 Ordering inside a row

* Insertion order → deterministic output, good for reproducible analysis.
* Sorted → binary search, merge-based intersection (triangle counting), stable diffs.
* Random → sometimes used to defeat adversarial worst cases in DFS.

### 5.6 Dynamic deletion

* Linked list: unlink node `O(deg)`.
* Vec: `swap_remove` `O(1)` after locating (destroys order) or `retain` `O(deg)`.
* Tombstones: mark dead, compact later.
* Vertex deletion: must scrub `v` from *every* row that references it (`O(V+E)` without `radj`, `O(deg)` with it).

---

## 6. C Implementation

C gives you the truth: every pointer, every allocation, every cache miss is visible.
Two implementations: **(A)** classic linked-list adjacency, **(B)** CSR.

### 6.1 (A) Linked-list adjacency — full, compilable

```c
// graph.c  —  gcc -std=c11 -O2 -Wall -Wextra -fsanitize=address,undefined graph.c -o graph
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <limits.h>
#include <stdbool.h>

typedef struct Edge {
    int to;
    int w;
    struct Edge *next;
} Edge;                       /* sizeof == 16, offsets: to@0 w@4 next@8 */

typedef struct {
    int    n;                 /* vertex count */
    int    m;                 /* arc count (directed arcs stored) */
    Edge **head;              /* head[u] -> first edge of u, or NULL */
} Graph;

static void *xmalloc(size_t sz) {
    void *p = malloc(sz);
    if (!p) { perror("malloc"); abort(); }
    return p;
}

Graph *graph_new(int n) {
    Graph *g = xmalloc(sizeof *g);
    g->n = n;
    g->m = 0;
    g->head = calloc((size_t)n, sizeof *g->head);   /* NULL = empty list */
    if (!g->head) { perror("calloc"); abort(); }
    return g;
}

/* O(1). Head insertion => neighbor order is reverse of insertion order. */
void add_arc(Graph *g, int u, int v, int w) {
    if (u < 0 || u >= g->n || v < 0 || v >= g->n) { fprintf(stderr, "bad vertex\n"); abort(); }
    Edge *e = xmalloc(sizeof *e);
    e->to = v; e->w = w;
    e->next = g->head[u];
    g->head[u] = e;
    g->m++;
}

void add_edge_undirected(Graph *g, int u, int v, int w) {
    add_arc(g, u, v, w);
    if (u != v) add_arc(g, v, u, w);       /* self-loop stored once */
}

/* O(deg u) */
bool has_arc(const Graph *g, int u, int v) {
    for (const Edge *e = g->head[u]; e; e = e->next)
        if (e->to == v) return true;
    return false;
}

/* O(deg u): remove first arc u->v using pointer-to-pointer (no special-case for head) */
bool remove_arc(Graph *g, int u, int v) {
    for (Edge **pp = &g->head[u]; *pp; pp = &(*pp)->next) {
        if ((*pp)->to == v) {
            Edge *dead = *pp;
            *pp = dead->next;
            free(dead);
            g->m--;
            return true;
        }
    }
    return false;
}

int out_degree(const Graph *g, int u) {
    int d = 0;
    for (const Edge *e = g->head[u]; e; e = e->next) d++;
    return d;
}

void graph_free(Graph *g) {
    for (int u = 0; u < g->n; u++) {
        Edge *e = g->head[u];
        while (e) { Edge *nx = e->next; free(e); e = nx; }
    }
    free(g->head);
    free(g);
}

void graph_print(const Graph *g) {
    for (int u = 0; u < g->n; u++) {
        printf("%d :", u);
        for (const Edge *e = g->head[u]; e; e = e->next) printf(" %d(%d)", e->to, e->w);
        putchar('\n');
    }
}
```

> **The `Edge **pp` idiom** — pointing at the *link* (either `head[u]` or some `node->next`) rather than at a
> node removes the "is it the head?" branch. Linus Torvalds' "good taste" example. Compilers emit
> tighter code and you eliminate a whole class of off-by-one unlink bugs.

### 6.2 BFS (unweighted shortest path)

```c
/* Returns dist[] (caller frees); dist[v] = -1 if unreachable. parent[] optional. */
int *bfs(const Graph *g, int src, int *parent /* may be NULL */) {
    int n = g->n;
    int *dist  = xmalloc((size_t)n * sizeof *dist);
    int *queue = xmalloc((size_t)n * sizeof *queue);   /* each vertex enqueued at most once */
    for (int i = 0; i < n; i++) { dist[i] = -1; if (parent) parent[i] = -1; }

    int qh = 0, qt = 0;
    dist[src] = 0;
    queue[qt++] = src;

    while (qh < qt) {
        int u = queue[qh++];
        for (const Edge *e = g->head[u]; e; e = e->next) {
            if (dist[e->to] == -1) {          /* mark on ENQUEUE, not dequeue */
                dist[e->to] = dist[u] + 1;
                if (parent) parent[e->to] = u;
                queue[qt++] = e->to;
            }
        }
    }
    free(queue);
    return dist;
}
```

**Why mark on enqueue?** Marking on dequeue lets a vertex be enqueued once per incoming edge → `O(E)` queue size and wrong `dist` bookkeeping. A flat array of size `n` as the queue is sufficient *because of* mark-on-enqueue.

```
BFS on canonical graph from 0:

 queue: [0]                 dist: 0=0
 pop 0  -> push 2,1         (head-insert reversed order: 2 then 1)
 queue: [2,1]               dist: 2=1, 1=1
 pop 2  -> push 4           dist: 4=2
 pop 1  -> push 3           dist: 3=2
 pop 4  -> 1 seen; push 5   dist: 5=3
 pop 3  -> 5 seen, 4 seen
 pop 5  -> done

 Layers:   L0 {0}   L1 {2,1}   L2 {4,3}   L3 {5}
```

### 6.3 DFS — recursive (3-color) and iterative

```c
enum { WHITE = 0, GRAY = 1, BLACK = 2 };

/* Recursive DFS with cycle detection. Returns true if a cycle is reachable from u.
   Stack depth = O(V) worst case: a 1M-node path WILL overflow an 8MB stack. */
static bool dfs_cycle(const Graph *g, int u, unsigned char *color) {
    color[u] = GRAY;                                   /* on current recursion stack */
    for (const Edge *e = g->head[u]; e; e = e->next) {
        if (color[e->to] == GRAY) return true;         /* back edge => cycle */
        if (color[e->to] == WHITE && dfs_cycle(g, e->to, color)) return true;
    }
    color[u] = BLACK;                                  /* fully explored */
    return false;
}

bool has_cycle(const Graph *g) {
    unsigned char *color = calloc((size_t)g->n, 1);
    bool found = false;
    for (int u = 0; u < g->n && !found; u++)
        if (color[u] == WHITE) found = dfs_cycle(g, u, color);
    free(color);
    return found;
}

/* Iterative DFS with an explicit stack + per-vertex edge cursor.
   The cursor array is what makes it a TRUE DFS (resume mid-adjacency-list). */
void dfs_iter(const Graph *g, int src, int *order /* preorder, length >= n */, int *count) {
    int n = g->n;
    unsigned char *seen = calloc((size_t)n, 1);
    const Edge **cur    = xmalloc((size_t)n * sizeof *cur);
    int *stack          = xmalloc((size_t)n * sizeof *stack);
    for (int i = 0; i < n; i++) cur[i] = g->head[i];

    int sp = 0, k = 0;
    stack[sp++] = src; seen[src] = 1; order[k++] = src;
    while (sp) {
        int u = stack[sp - 1];
        if (cur[u]) {
            int v = cur[u]->to;
            cur[u] = cur[u]->next;             /* advance cursor BEFORE descending */
            if (!seen[v]) { seen[v] = 1; order[k++] = v; stack[sp++] = v; }
        } else {
            sp--;                              /* postorder point: u is finished */
        }
    }
    *count = k;
    free(seen); free(cur); free(stack);
}
```

Edge classification during DFS (directed):

```
 Tree edge     u -> v   v was WHITE          (becomes DFS-tree edge)
 Back edge     u -> v   v is GRAY            (v is ancestor of u)   => CYCLE
 Forward edge  u -> v   v is BLACK, disc[u] < disc[v]   (v is descendant)
 Cross edge    u -> v   v is BLACK, disc[u] > disc[v]   (different subtree)
```

### 6.4 Topological sort (Kahn's algorithm) — also a cycle detector

```c
/* Returns number of vertices emitted. If < n, the graph has a cycle. */
int topo_sort(const Graph *g, int *out) {
    int n = g->n;
    int *indeg = calloc((size_t)n, sizeof *indeg);
    int *queue = xmalloc((size_t)n * sizeof *queue);
    for (int u = 0; u < n; u++)
        for (const Edge *e = g->head[u]; e; e = e->next) indeg[e->to]++;

    int qh = 0, qt = 0;
    for (int u = 0; u < n; u++) if (indeg[u] == 0) queue[qt++] = u;

    int k = 0;
    while (qh < qt) {
        int u = queue[qh++];
        out[k++] = u;
        for (const Edge *e = g->head[u]; e; e = e->next)
            if (--indeg[e->to] == 0) queue[qt++] = e->to;
    }
    free(indeg); free(queue);
    return k;
}
```

### 6.5 Dijkstra with a binary heap (lazy deletion)

```c
typedef struct { long long d; int v; } HeapItem;
typedef struct { HeapItem *a; size_t len, cap; } Heap;

static void heap_push(Heap *h, HeapItem x) {
    if (h->len == h->cap) {
        h->cap = h->cap ? h->cap * 2 : 16;
        h->a = realloc(h->a, h->cap * sizeof *h->a);
        if (!h->a) abort();
    }
    size_t i = h->len++;
    while (i > 0) {                                  /* sift up */
        size_t p = (i - 1) / 2;
        if (h->a[p].d <= x.d) break;
        h->a[i] = h->a[p]; i = p;
    }
    h->a[i] = x;
}

static HeapItem heap_pop(Heap *h) {
    HeapItem top = h->a[0];
    HeapItem x   = h->a[--h->len];
    if (h->len) {
        size_t i = 0;
        for (;;) {                                   /* sift down */
            size_t l = 2 * i + 1;
            if (l >= h->len) break;
            size_t r = l + 1;
            size_t c = (r < h->len && h->a[r].d < h->a[l].d) ? r : l;
            if (h->a[c].d >= x.d) break;
            h->a[i] = h->a[c]; i = c;
        }
        h->a[i] = x;
    }
    return top;
}

#define INF LLONG_MAX

/* Requires all weights >= 0. O((V+E) log V) with lazy deletion (heap may hold up to E entries). */
long long *dijkstra(const Graph *g, int src, int *parent) {
    long long *dist = xmalloc((size_t)g->n * sizeof *dist);
    for (int i = 0; i < g->n; i++) { dist[i] = INF; if (parent) parent[i] = -1; }
    Heap h = {0};
    dist[src] = 0;
    heap_push(&h, (HeapItem){0, src});

    while (h.len) {
        HeapItem it = heap_pop(&h);
        if (it.d != dist[it.v]) continue;            /* stale entry: a better path was already settled */
        for (const Edge *e = g->head[it.v]; e; e = e->next) {
            long long nd = it.d + e->w;              /* no overflow: d < INF here, w small */
            if (nd < dist[e->to]) {
                dist[e->to] = nd;
                if (parent) parent[e->to] = it.v;
                heap_push(&h, (HeapItem){nd, e->to});
            }
        }
    }
    free(h.a);
    return dist;
}
```

### 6.6 (B) CSR in C — build via counting sort

```c
typedef struct {
    int  n;
    int  m;
    int *off;        /* n+1 */
    int *dst;        /* m   */
    int *wt;         /* m   */
} CSR;

typedef struct { int u, v, w; } EdgeRec;

CSR csr_build(int n, const EdgeRec *el, int m) {
    CSR c = { .n = n, .m = m };
    c.off = calloc((size_t)n + 1, sizeof *c.off);
    c.dst = xmalloc((size_t)m * sizeof *c.dst);
    c.wt  = xmalloc((size_t)m * sizeof *c.wt);
    if (!c.off) abort();

    /* pass 1: histogram of out-degrees, stored shifted by +1 */
    for (int i = 0; i < m; i++) c.off[el[i].u + 1]++;

    /* pass 2: prefix sum -> off[u] = start index of u's row */
    for (int u = 0; u < n; u++) c.off[u + 1] += c.off[u];

    /* pass 3: scatter using a moving cursor per row */
    int *cur = xmalloc((size_t)n * sizeof *cur);
    memcpy(cur, c.off, (size_t)n * sizeof *cur);
    for (int i = 0; i < m; i++) {
        int pos = cur[el[i].u]++;
        c.dst[pos] = el[i].v;
        c.wt[pos]  = el[i].w;
    }
    free(cur);
    return c;
}

static inline int csr_deg(const CSR *c, int u) { return c->off[u + 1] - c->off[u]; }

/* Iteration: the hot loop of nearly every graph analysis engine */
long long csr_sum_weights_from(const CSR *c, int u) {
    long long s = 0;
    for (int i = c->off[u]; i < c->off[u + 1]; i++) s += c->wt[i];
    return s;
}
```

```
Counting-sort build, canonical graph (edge list order given in §1):

 edge list u: 0 0 1 2 3 3 4 4

 pass 1 (histogram at off[u+1]):   off = [0, 2, 1, 1, 2, 2, 0]
 pass 2 (prefix sum):              off = [0, 2, 3, 4, 6, 8, 8]
 pass 3 (scatter, stable):         dst = [1, 2, 3, 4, 5, 4, 1, 5]
```

### 6.7 `main` — wire it together

```c
int main(void) {
    Graph *g = graph_new(6);
    add_arc(g, 0, 1, 4); add_arc(g, 0, 2, 2); add_arc(g, 1, 3, 1); add_arc(g, 2, 4, 3);
    add_arc(g, 3, 5, 5); add_arc(g, 4, 1, 1); add_arc(g, 4, 5, 2); add_arc(g, 3, 4, 1);

    graph_print(g);

    int *d = bfs(g, 0, NULL);
    printf("bfs   :"); for (int i = 0; i < g->n; i++) printf(" %d", d[i]); putchar('\n');
    free(d);

    long long *dd = dijkstra(g, 0, NULL);
    printf("dijk  :"); for (int i = 0; i < g->n; i++) printf(" %lld", dd[i]); putchar('\n');
    free(dd);

    printf("cycle : %s\n", has_cycle(g) ? "yes" : "no");   /* yes: 1->3->4->1 */
    graph_free(g);
    return 0;
}
/* Expected dijkstra from 0: 0 4 2 5 5 7   (0->2->4 = 5 ; 0->1->3 = 5 ; 0->1->3->4 = 6 > 5 ; 4->5 = 7) */
```

### 6.8 C-specific engineering notes

* **Arena allocation**: allocate all `Edge` nodes from one big array; `next` becomes an `int` index. Drops 32 B/edge → 12 B/edge, kills malloc overhead, makes `graph_free` a single `free`.
* **Integer overflow**: `n * sizeof(...)` can overflow `size_t` on hostile input. Use `calloc(n, size)` or check with `__builtin_mul_overflow`.
* **Sanitizers**: always run with `-fsanitize=address,undefined` during development; adjacency code is pointer-heavy and use-after-free on unlink is the classic bug.
* **Vertex validation**: any graph built from *parsed input* (a file format, a network packet) must bounds-check vertex ids. Unchecked `head[v]` with attacker-controlled `v` is an out-of-bounds read/write primitive.

---

## 7. Go Implementation

Go's idiom: **slice of slices**. `nil` rows are free, `append` handles growth, the GC handles lifetime.
Generic (Go 1.18+) helps only when vertex keys are non-integer; algorithms stay on `int`.

### 7.1 Core type and mutation

```go
// graph.go   —   go run graph.go
package main

import (
	"container/heap"
	"fmt"
	"math"
)

type Edge struct {
	To int
	W  int64
}

type Graph struct {
	N   int
	Adj [][]Edge // Adj[u] is a slice; nil == no out-edges
}

func NewGraph(n int) *Graph {
	return &Graph{N: n, Adj: make([][]Edge, n)}
}

func (g *Graph) AddArc(u, v int, w int64) {
	g.Adj[u] = append(g.Adj[u], Edge{v, w}) // amortized O(1); may reallocate row
}

func (g *Graph) AddUndirected(u, v int, w int64) {
	g.AddArc(u, v, w)
	if u != v {
		g.AddArc(v, u, w)
	}
}

func (g *Graph) HasArc(u, v int) bool {
	for _, e := range g.Adj[u] { // e is a COPY (16 B); fine for small structs
		if e.To == v {
			return true
		}
	}
	return false
}

// RemoveArc: O(deg u), order-preserving. Use swap-remove if order is irrelevant.
func (g *Graph) RemoveArc(u, v int) bool {
	row := g.Adj[u]
	for i := range row {
		if row[i].To == v {
			g.Adj[u] = append(row[:i], row[i+1:]...) // shifts tail left
			return true
		}
	}
	return false
}

func (g *Graph) OutDegree(u int) int { return len(g.Adj[u]) }

// Transpose builds the reverse adjacency (who points at v?).
func (g *Graph) Transpose() *Graph {
	t := NewGraph(g.N)
	for u, row := range g.Adj {
		for _, e := range row {
			t.AddArc(e.To, u, e.W)
		}
	}
	return t
}
```

> **Go gotcha — aliasing.** `append(row[:i], row[i+1:]...)` mutates the *shared backing array*. If any other
> slice header (e.g., a neighbor list you handed to a caller) still views that array, it now sees shifted data.
> If you expose rows, either copy on the way out or document that rows are borrowed and invalidated by mutation.

### 7.2 BFS

```go
func (g *Graph) BFS(src int) (dist, parent []int) {
	dist = make([]int, g.N)
	parent = make([]int, g.N)
	for i := range dist {
		dist[i], parent[i] = -1, -1
	}
	queue := make([]int, 0, g.N) // capacity N: never reallocates (each vertex enqueued once)
	dist[src] = 0
	queue = append(queue, src)
	for head := 0; head < len(queue); head++ {
		u := queue[head]
		for _, e := range g.Adj[u] {
			if dist[e.To] == -1 {
				dist[e.To] = dist[u] + 1
				parent[e.To] = u
				queue = append(queue, e.To)
			}
		}
	}
	return
}

// PathTo reconstructs src->v from a parent array.
func PathTo(parent []int, v int) []int {
	var p []int
	for x := v; x != -1; x = parent[x] {
		p = append(p, x)
	}
	for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
		p[i], p[j] = p[j], p[i]
	}
	return p
}
```

### 7.3 DFS with timestamps and edge classification

```go
type EdgeKind uint8

const (
	Tree EdgeKind = iota
	Back
	Forward
	Cross
)

type DFSResult struct {
	Disc, Fin []int
	HasCycle  bool
	Order     []int // reverse postorder => topological order iff DAG
}

func (g *Graph) DFSAll() DFSResult {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	r := DFSResult{Disc: make([]int, g.N), Fin: make([]int, g.N)}
	color := make([]uint8, g.N)
	clock := 0
	post := make([]int, 0, g.N)

	// Go's stacks grow dynamically (starts 8KB, max 1GB on 64-bit),
	// so recursion depth of ~10^6 is survivable — unlike C's fixed 8MB.
	var visit func(u int)
	visit = func(u int) {
		color[u] = gray
		clock++
		r.Disc[u] = clock
		for _, e := range g.Adj[u] {
			switch color[e.To] {
			case white:
				visit(e.To)
			case gray:
				r.HasCycle = true // back edge
			}
		}
		color[u] = black
		clock++
		r.Fin[u] = clock
		post = append(post, u)
	}
	for u := 0; u < g.N; u++ {
		if color[u] == white {
			visit(u)
		}
	}
	for i := len(post) - 1; i >= 0; i-- {
		r.Order = append(r.Order, post[i])
	}
	return r
}
```

### 7.4 Kahn's topological sort

```go
func (g *Graph) TopoSort() ([]int, bool) {
	indeg := make([]int, g.N)
	for _, row := range g.Adj {
		for _, e := range row {
			indeg[e.To]++
		}
	}
	q := make([]int, 0, g.N)
	for u, d := range indeg {
		if d == 0 {
			q = append(q, u)
		}
	}
	for i := 0; i < len(q); i++ {
		for _, e := range g.Adj[q[i]] {
			if indeg[e.To]--; indeg[e.To] == 0 {
				q = append(q, e.To)
			}
		}
	}
	return q, len(q) == g.N // false => cycle
}
```

### 7.5 Dijkstra via `container/heap`

```go
type item struct {
	d int64
	v int
}
type pq []item

func (p pq) Len() int            { return len(p) }
func (p pq) Less(i, j int) bool  { return p[i].d < p[j].d }
func (p pq) Swap(i, j int)       { p[i], p[j] = p[j], p[i] }
func (p *pq) Push(x any)         { *p = append(*p, x.(item)) }
func (p *pq) Pop() any {
	old := *p
	n := len(old)
	x := old[n-1]
	*p = old[:n-1]
	return x
}

func (g *Graph) Dijkstra(src int) []int64 {
	dist := make([]int64, g.N)
	for i := range dist {
		dist[i] = math.MaxInt64
	}
	dist[src] = 0
	h := &pq{{0, src}}
	for h.Len() > 0 {
		it := heap.Pop(h).(item)
		if it.d != dist[it.v] {
			continue // stale
		}
		for _, e := range g.Adj[it.v] {
			if nd := it.d + e.W; nd < dist[e.To] {
				dist[e.To] = nd
				heap.Push(h, item{nd, e.To})
			}
		}
	}
	return dist
}
```

> `container/heap` uses `interface{}` boxing → every `Push(item{...})` allocates (16 B escape to heap).
> In hot paths, write a concrete heap on `[]item` (as in the C version) to eliminate that allocation.

### 7.6 Tarjan's SCC (single-pass, `O(V+E)`)

```go
func (g *Graph) SCC() (comp []int, ncomp int) {
	idx := make([]int, g.N)     // discovery index, 0 = unvisited
	low := make([]int, g.N)
	onStack := make([]bool, g.N)
	comp = make([]int, g.N)
	for i := range comp {
		comp[i] = -1
	}
	var stack []int
	counter := 0

	var strong func(u int)
	strong = func(u int) {
		counter++
		idx[u], low[u] = counter, counter
		stack = append(stack, u)
		onStack[u] = true
		for _, e := range g.Adj[u] {
			v := e.To
			if idx[v] == 0 {
				strong(v)
				low[u] = min(low[u], low[v])
			} else if onStack[v] {
				low[u] = min(low[u], idx[v]) // back/cross edge into current SCC candidate
			}
		}
		if low[u] == idx[u] { // u is the root of an SCC: pop it off
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp[w] = ncomp
				if w == u {
					break
				}
			}
			ncomp++
		}
	}
	for u := 0; u < g.N; u++ {
		if idx[u] == 0 {
			strong(u)
		}
	}
	return
}
```

`min` is a builtin from Go 1.21. On older toolchains define it yourself.

### 7.6.1 Tarjan trace on the canonical graph

```
DFS from 0 (Go iterates rows in insertion order):

 visit 0  idx=1 low=1  stack=[0]
  -> 1    idx=2 low=2  stack=[0,1]
   -> 3   idx=3 low=3  stack=[0,1,3]
    -> 5  idx=4 low=4  stack=[0,1,3,5]      5 has no out-edges
       low==idx => pop 5 => SCC#0 = {5}
    -> 4  idx=5 low=5  stack=[0,1,3,4]
       edge 4->1: 1 onStack => low[4]=min(5, idx[1]=2)=2
       edge 4->5: 5 popped, ignore
       low[4]=2 != idx[4]=5 => not root
    back in 3: low[3]=min(3, low[4]=2)=2   not root
   back in 1: low[1]=min(2, low[3]=2)=2 == idx[1] => ROOT
       pop until 1: {4,3,1} => SCC#1
 -> 2 (from 0)  idx=6, edge 2->4: 4 already assigned (not onStack) => ignore
       low==idx => SCC#2 = {2}
 back in 0: low==idx => SCC#3 = {0}

 Components: {5}, {1,3,4}, {2}, {0}     (reverse topological order of the condensation DAG)
```

### 7.7 CSR in Go

```go
type CSR struct {
	N   int
	Off []int32 // len N+1
	Dst []int32 // len M
	Wt  []int64 // len M
}

type EdgeRec struct {
	U, V int32
	W    int64
}

func BuildCSR(n int, el []EdgeRec) *CSR {
	c := &CSR{N: n, Off: make([]int32, n+1), Dst: make([]int32, len(el)), Wt: make([]int64, len(el))}
	for _, e := range el {
		c.Off[e.U+1]++
	}
	for u := 0; u < n; u++ {
		c.Off[u+1] += c.Off[u]
	}
	cur := make([]int32, n)
	copy(cur, c.Off[:n])
	for _, e := range el {
		p := cur[e.U]
		cur[e.U]++
		c.Dst[p], c.Wt[p] = e.V, e.W
	}
	return c
}

// Neighbors returns a sub-slice view: zero-copy, zero-alloc.
func (c *CSR) Neighbors(u int) []int32 { return c.Dst[c.Off[u]:c.Off[u+1]] }
```

### 7.8 `main`

```go
func main() {
	g := NewGraph(6)
	for _, e := range [][3]int64{{0, 1, 4}, {0, 2, 2}, {1, 3, 1}, {2, 4, 3}, {3, 5, 5}, {4, 1, 1}, {4, 5, 2}, {3, 4, 1}} {
		g.AddArc(int(e[0]), int(e[1]), e[2])
	}
	dist, parent := g.BFS(0)
	fmt.Println("bfs   ", dist, PathTo(parent, 5))
	fmt.Println("dijk  ", g.Dijkstra(0))
	fmt.Println("cycle ", g.DFSAll().HasCycle)
	comp, k := g.SCC()
	fmt.Println("scc   ", comp, k)
	_, ok := g.TopoSort()
	fmt.Println("dag?  ", ok)
}
```

### 7.9 Go-specific notes for analysts

* **Reading Go malware:** a graph in Go compiles to `runtime.growslice` calls (from `append`), `runtime.makeslice`, and `runtime.panicIndex` bounds-check tails. Adjacency-list loops show `ptr,len` pairs loaded at 24-byte strides (`lea rax,[rax+rax*2]; ... [base+rax*8]`).
* **`for _, e := range row`** copies each element. For large structs use index form `row[i]` or range over pointers.
* **Concurrency:** adjacency lists are read-safe across goroutines if no mutation. Parallel BFS: per-level frontier partitioned across workers, `atomic.CompareAndSwap` on the `visited` array.

---

## 8. Rust Implementation

Rust changes the *design question*. A pointer-linked graph (`Rc<RefCell<Node>>`, or `Box` nodes with back-pointers)
fights the borrow checker because graphs are **shared, cyclic, aliased** structures — exactly what ownership forbids.
The idiomatic resolution:

> **Nodes are integers. The graph owns all storage. Edges are indices, not references.**

An index is a *capability-free pointer*: it cannot dangle into freed memory (only into a wrong slot), it is `Copy`,
`Send`, and lets you mutate freely without aliasing `&mut`. This is what `petgraph`, rustc's own MIR CFG
(`BasicBlock(u32)` indices into `Vec<BasicBlockData>`), and LLVM-style arenas do.

### 8.1 Core type

```rust
// src/main.rs   —   cargo run --release
use std::cmp::Reverse;
use std::collections::{BinaryHeap, HashMap, VecDeque};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Edge {
    pub to: usize,
    pub w: u64,
}

#[derive(Clone, Debug, Default)]
pub struct Graph {
    adj: Vec<Vec<Edge>>,
}

impl Graph {
    pub fn new(n: usize) -> Self {
        Self { adj: vec![Vec::new(); n] } // Vec::new() doesn't allocate: ptr=dangling, cap=0
    }

    pub fn n(&self) -> usize { self.adj.len() }

    pub fn add_vertex(&mut self) -> usize {
        self.adj.push(Vec::new());
        self.adj.len() - 1
    }

    pub fn add_arc(&mut self, u: usize, v: usize, w: u64) {
        assert!(v < self.adj.len(), "target vertex out of range");
        self.adj[u].push(Edge { to: v, w }); // panics on bad u: bounds check
    }

    pub fn add_undirected(&mut self, u: usize, v: usize, w: u64) {
        self.add_arc(u, v, w);
        if u != v { self.add_arc(v, u, w); }
    }

    /// Borrowed view: zero-copy. Lifetime tied to &self => cannot mutate the graph while iterating.
    pub fn neighbors(&self, u: usize) -> &[Edge] { &self.adj[u] }

    pub fn out_degree(&self, u: usize) -> usize { self.adj[u].len() }

    pub fn has_arc(&self, u: usize, v: usize) -> bool {
        self.adj[u].iter().any(|e| e.to == v)
    }

    /// Order-preserving removal of all u->v arcs.
    pub fn remove_arcs(&mut self, u: usize, v: usize) -> usize {
        let before = self.adj[u].len();
        self.adj[u].retain(|e| e.to != v);
        before - self.adj[u].len()
    }

    pub fn transpose(&self) -> Graph {
        let mut t = Graph::new(self.n());
        for (u, row) in self.adj.iter().enumerate() {
            for e in row { t.add_arc(e.to, u, e.w); }
        }
        t
    }
}
```

**What the borrow checker just bought you:** `for e in g.neighbors(u) { g.add_arc(..) }` is a **compile error**
(shared borrow alive while requesting `&mut`). In C and Go the same code is a silent iterator-invalidation bug
(realloc moves the row under you). Ownership makes that bug class unrepresentable.

### 8.2 BFS

```rust
impl Graph {
    pub fn bfs(&self, src: usize) -> (Vec<Option<usize>>, Vec<Option<usize>>) {
        let n = self.n();
        let mut dist = vec![None; n];
        let mut parent = vec![None; n];
        let mut q = VecDeque::with_capacity(n);
        dist[src] = Some(0);
        q.push_back(src);
        while let Some(u) = q.pop_front() {
            let du = dist[u].unwrap();
            for e in &self.adj[u] {
                if dist[e.to].is_none() {
                    dist[e.to] = Some(du + 1);
                    parent[e.to] = Some(u);
                    q.push_back(e.to);
                }
            }
        }
        (dist, parent)
    }
}

pub fn path_to(parent: &[Option<usize>], mut v: usize) -> Vec<usize> {
    let mut p = vec![v];
    while let Some(pv) = parent[v] { p.push(pv); v = pv; }
    p.reverse();
    p
}
```

### 8.3 DFS — iterative with explicit cursor (no stack overflow), cycle detection, topological order

```rust
#[derive(Clone, Copy, PartialEq, Eq)]
enum Color { White, Gray, Black }

pub struct DfsInfo {
    pub has_cycle: bool,
    pub postorder: Vec<usize>,
    pub disc: Vec<u32>,
    pub fin: Vec<u32>,
}

impl Graph {
    pub fn dfs_all(&self) -> DfsInfo {
        let n = self.n();
        let mut color = vec![Color::White; n];
        let mut disc = vec![0u32; n];
        let mut fin = vec![0u32; n];
        let mut post = Vec::with_capacity(n);
        let mut clock = 0u32;
        let mut has_cycle = false;

        // stack of (vertex, next-edge-index): the cursor lives on the stack, not in the graph
        let mut stack: Vec<(usize, usize)> = Vec::new();

        for root in 0..n {
            if color[root] != Color::White { continue; }
            color[root] = Color::Gray;
            clock += 1; disc[root] = clock;
            stack.push((root, 0));

            while let Some(&(u, i)) = stack.last() {          // copy (u, cursor) out: no live borrow
                if let Some(e) = self.adj[u].get(i) {
                    stack.last_mut().unwrap().1 = i + 1;      // advance cursor BEFORE descending
                    match color[e.to] {
                        Color::White => {
                            color[e.to] = Color::Gray;
                            clock += 1; disc[e.to] = clock;
                            stack.push((e.to, 0));
                        }
                        Color::Gray => has_cycle = true,      // back edge
                        Color::Black => {}
                    }
                } else {
                    color[u] = Color::Black;
                    clock += 1; fin[u] = clock;
                    post.push(u);
                    stack.pop();
                }
            }
        }
        DfsInfo { has_cycle, postorder: post, disc, fin }
    }

    /// Some(order) iff DAG. Reverse postorder of a DAG is a topological order.
    pub fn topo_sort(&self) -> Option<Vec<usize>> {
        let info = self.dfs_all();
        if info.has_cycle { return None; }
        let mut o = info.postorder;
        o.reverse();
        Some(o)
    }

    /// Kahn's algorithm — iterative BFS-flavored alternative.
    pub fn topo_kahn(&self) -> Option<Vec<usize>> {
        let n = self.n();
        let mut indeg = vec![0usize; n];
        for row in &self.adj { for e in row { indeg[e.to] += 1; } }
        let mut q: VecDeque<usize> = (0..n).filter(|&u| indeg[u] == 0).collect();
        let mut out = Vec::with_capacity(n);
        while let Some(u) = q.pop_front() {
            out.push(u);
            for e in &self.adj[u] {
                indeg[e.to] -= 1;
                if indeg[e.to] == 0 { q.push_back(e.to); }
            }
        }
        (out.len() == n).then_some(out)
    }
}
```

Copying `(u, i)` out of the stack top (both are `Copy`) means no borrow of `stack` is alive when we `push`/`pop`, so the borrow checker is satisfied without tricks.

### 8.4 Dijkstra

```rust
impl Graph {
    pub fn dijkstra(&self, src: usize) -> Vec<Option<u64>> {
        let mut dist: Vec<Option<u64>> = vec![None; self.n()];
        let mut heap = BinaryHeap::new();            // max-heap: wrap in Reverse for min-heap
        dist[src] = Some(0);
        heap.push(Reverse((0u64, src)));
        while let Some(Reverse((d, u))) = heap.pop() {
            if dist[u].map_or(false, |best| d > best) { continue; } // stale entry
            for e in &self.adj[u] {
                let nd = d.checked_add(e.w).expect("path length overflow");
                if dist[e.to].map_or(true, |cur| nd < cur) {
                    dist[e.to] = Some(nd);
                    heap.push(Reverse((nd, e.to)));
                }
            }
        }
        dist
    }
}
```

`Reverse((u64, usize))` orders by tuple lexicographically → by distance first, vertex id as tiebreak — deterministic.
`f64` weights need a wrapper (`ordered_float::OrderedFloat`) because `f64` is not `Ord` (NaN).

### 8.5 Tarjan SCC (recursive; see note on depth)

```rust
impl Graph {
    pub fn scc(&self) -> (Vec<usize>, usize) {
        struct St<'a> {
            g: &'a Graph,
            idx: Vec<usize>,
            low: Vec<usize>,
            on: Vec<bool>,
            comp: Vec<usize>,
            stack: Vec<usize>,
            counter: usize,
            ncomp: usize,
        }
        impl<'a> St<'a> {
            fn strong(&mut self, u: usize) {
                let g = self.g; // copy the &'a Graph out so iterating doesn't borrow `self`
                self.counter += 1;
                self.idx[u] = self.counter;
                self.low[u] = self.counter;
                self.stack.push(u);
                self.on[u] = true;
                for e in &g.adj[u] {
                    let v = e.to;
                    if self.idx[v] == 0 {
                        self.strong(v);
                        self.low[u] = self.low[u].min(self.low[v]);
                    } else if self.on[v] {
                        self.low[u] = self.low[u].min(self.idx[v]);
                    }
                }
                if self.low[u] == self.idx[u] {
                    loop {
                        let w = self.stack.pop().unwrap();
                        self.on[w] = false;
                        self.comp[w] = self.ncomp;
                        if w == u { break; }
                    }
                    self.ncomp += 1;
                }
            }
        }
        let n = self.n();
        let mut st = St { g: self, idx: vec![0; n], low: vec![0; n], on: vec![false; n],
                          comp: vec![usize::MAX; n], stack: vec![], counter: 0, ncomp: 0 };
        for u in 0..n { if st.idx[u] == 0 { st.strong(u); } }
        (st.comp, st.ncomp)
    }
}
```

> **Depth warning:** Rust's main thread has 8 MB (Linux) and spawned threads 2 MB by default. A recursive DFS on a
> 10⁶-node chain will overflow → `SIGSEGV`/abort with "thread has overflowed its stack". For untrusted or huge
> graphs use the iterative pattern from §8.3, or run in `std::thread::Builder::new().stack_size(...)`.

### 8.6 CSR in Rust

```rust
pub struct Csr {
    off: Vec<u32>,   // len n+1
    dst: Vec<u32>,   // len m
    wt: Vec<u64>,    // len m
}

impl Csr {
    pub fn from_edges(n: usize, edges: &[(u32, u32, u64)]) -> Self {
        let mut off = vec![0u32; n + 1];
        for &(u, _, _) in edges { off[u as usize + 1] += 1; }
        for u in 0..n { off[u + 1] += off[u]; }                  // prefix sum
        let mut cur = off[..n].to_vec();
        let mut dst = vec![0u32; edges.len()];
        let mut wt = vec![0u64; edges.len()];
        for &(u, v, w) in edges {
            let p = cur[u as usize] as usize;
            cur[u as usize] += 1;
            dst[p] = v; wt[p] = w;
        }
        Self { off, dst, wt }
    }

    #[inline] pub fn n(&self) -> usize { self.off.len() - 1 }

    #[inline]
    pub fn neighbors(&self, u: usize) -> (&[u32], &[u64]) {
        let (a, b) = (self.off[u] as usize, self.off[u + 1] as usize);
        (&self.dst[a..b], &self.wt[a..b])
    }

    pub fn bfs(&self, src: usize) -> Vec<u32> {
        let mut dist = vec![u32::MAX; self.n()];
        let mut q = Vec::with_capacity(self.n());
        dist[src] = 0; q.push(src as u32);
        let mut head = 0;
        while head < q.len() {
            let u = q[head] as usize; head += 1;
            for &v in self.neighbors(u).0 {
                if dist[v as usize] == u32::MAX {
                    dist[v as usize] = dist[u] + 1;
                    q.push(v);
                }
            }
        }
        dist
    }
}
```

### 8.7 Non-integer vertex keys (strings, hashes, addresses)

```rust
pub struct Interner<K: std::hash::Hash + Eq + Clone> {
    ids: HashMap<K, usize>,
    keys: Vec<K>,
}

impl<K: std::hash::Hash + Eq + Clone> Interner<K> {
    pub fn new() -> Self { Self { ids: HashMap::new(), keys: Vec::new() } }
    pub fn intern(&mut self, k: &K, g: &mut Graph) -> usize {
        if let Some(&i) = self.ids.get(k) { return i; }
        let i = g.add_vertex();
        self.ids.insert(k.clone(), i);
        self.keys.push(k.clone());
        i
    }
    pub fn key(&self, id: usize) -> &K { &self.keys[id] }
}

// Example: call graph keyed by function address.
//   let mut g = Graph::new(0); let mut names = Interner::<u64>::new();
//   let a = names.intern(&0x140001000, &mut g);
//   let b = names.intern(&0x140001450, &mut g);
//   g.add_arc(a, b, 1);
```

### 8.8 `main`

```rust
fn main() {
    let mut g = Graph::new(6);
    for &(u, v, w) in &[(0,1,4),(0,2,2),(1,3,1),(2,4,3),(3,5,5),(4,1,1),(4,5,2),(3,4,1)] {
        g.add_arc(u, v, w);
    }
    let (dist, parent) = g.bfs(0);
    println!("bfs   {:?}  path->5 {:?}", dist, path_to(&parent, 5));
    println!("dijk  {:?}", g.dijkstra(0));
    println!("cycle {}", g.dfs_all().has_cycle);
    println!("scc   {:?}", g.scc());
    println!("topo  {:?}", g.topo_kahn());
}
```

### 8.9 Rust-specific notes

* **`Vec<Vec<T>>` vs flat CSR:** rows are separate allocations; each `push` may realloc. Build with `Vec` for flexibility, then `freeze()` into CSR for analysis passes.
* **`unsafe` speedups:** `get_unchecked` removes the bounds check in BFS's inner loop, ~5–15% on cache-resident graphs. Only when you have proven `v < n` at construction time (validate once in `add_arc`/`from_edges`).
* **`u32` ids** halve memory traffic vs `usize` and double the neighbors per 64-byte cache line (16 vs 8).
* **Reading Rust binaries:** a `Vec` index compiles to `cmp idx, len; jae → core::panicking::panic_bounds_check`. `Vec::push` shows `RawVec::grow_one` (or `finish_grow`) on the cold path. Inner `Vec` headers are 24 B, so `adj[u]` addressing is `lea t,[u+u*2]; [base+t*8]`.
* **Interior mutability trap:** `Vec<RefCell<Vec<Edge>>>` pushes runtime borrow checks (`BorrowMutError` panics) into every access. Prefer restructuring passes to take `&mut Graph` once.
* **Ecosystem:** `petgraph` (`Graph`, `StableGraph`, `GraphMap`, `Csr`) is the reference implementation — read `petgraph::csr::Csr` for a production CSR.

---

## 9. Algorithms on Adjacency Lists

Every algorithm below is a **different discipline for choosing the next vertex** to expand.
The adjacency list supplies "expand" in `O(deg)`; the discipline determines the total.

| Algorithm | Frontier discipline | Data structure | Time | Solves |
|---|---|---|---|---|
| BFS | oldest first | FIFO queue | `O(V+E)` | unweighted shortest path, levels, bipartite test |
| DFS | newest first | LIFO stack / recursion | `O(V+E)` | cycles, topo order, SCC, articulation points, bridges |
| Dijkstra | smallest tentative distance | binary heap | `O((V+E) log V)` | non-negative weighted shortest path |
| 0-1 BFS | dist 0 to front, 1 to back | deque | `O(V+E)` | edge weights ∈ {0,1} |
| Bellman–Ford | all edges, `V-1` rounds | edge list / adj list | `O(V·E)` | negative weights, negative-cycle detection |
| Kahn | in-degree 0 | FIFO queue | `O(V+E)` | topological sort |
| Prim | cheapest crossing edge | heap | `O(E log V)` | MST |
| Kruskal | cheapest edge globally | edge list + DSU | `O(E log E)` | MST |
| Tarjan / Kosaraju | DFS + low-link / two passes | stack (+ transpose) | `O(V+E)` | SCC |
| Dominators (Cooper–Harvey–Kennedy) | iterative dataflow over RPO | `radj` + arrays | ~`O(V²)` worst, near-linear in practice | CFG loops, SSA construction |

### 9.1 BFS invariant (why it gives shortest paths)

```
Loop invariant: the queue always holds vertices with dist = d followed by dist = d+1, never further.

 Level structure:
    L0 ──► L1 ──► L2 ──► L3
   {src}  (all vertices at exactly 1 hop) ...

 Every edge (u,v) satisfies  |dist[u] - dist[v]| <= 1 in undirected BFS
 (directed: dist[v] <= dist[u] + 1).
 => first discovery of v is via a shortest path.
```

**Bipartite test:** BFS 2-color by level parity; any edge joining equal parity → odd cycle → not bipartite.

### 9.2 DFS invariant (parenthesis theorem)

```
For any two vertices u, v the intervals [disc, fin] are either disjoint or one contains the other.

      disc[u]                         fin[u]
         |    disc[v]     fin[v]        |
         |      |___________|           |          => v is a descendant of u
         |___________________________ __|
```

* **Cycle ⇔ back edge** (in directed graphs).
* **Topological order = reverse of finish times.**
* **Bridges/articulation points:** DFS with `low[u] = min(disc[u], disc[back-edge targets], low[children])`. Edge `u→v` (tree) is a bridge iff `low[v] > disc[u]`.

### 9.3 Dijkstra correctness (the greedy-choice argument)

```
Claim: when v is popped with tentative distance d, d = true shortest distance.

Proof sketch: suppose a shorter path exists. It must leave the settled set S through some edge (x,y),
y not in S. dist[y] <= d(x) + w(x,y) <= (length of that path prefix) <= true(v) < d,
so y would have been popped before v. Contradiction.
The proof USES w >= 0 (prefix length <= whole path length).
```

**One negative edge breaks it.** Use Bellman–Ford, or Johnson's reweighting (Bellman–Ford once + Dijkstra from each source).

### 9.4 Bellman–Ford on adjacency lists

```
dist[src] = 0
repeat V-1 times:
    changed = false
    for u in 0..V:
        if dist[u] == INF: continue
        for (v,w) in adj[u]:
            if dist[u] + w < dist[v]: dist[v] = dist[u] + w; changed = true
    if !changed: break
one more pass: any further relaxation => reachable negative cycle
```

Intuition: after round `k`, `dist[v]` ≤ the best path using at most `k` edges. A simple path has at most `V-1` edges.

### 9.5 Strongly connected components and the condensation DAG

```
Canonical graph SCCs:  {0}  {2}  {1,3,4}  {5}

Condensation (collapse each SCC to one node) is ALWAYS a DAG:

     {0} ────► {1,3,4} ────► {5}
      │            ▲
      └──► {2} ────┘            (2->4 enters the big SCC)
```

Kosaraju: (1) DFS on `G`, record finish order; (2) DFS on transpose `Gᵀ` in *decreasing* finish order; each DFS tree in pass 2 is one SCC. Needs `radj` — the reason §5.2 matters.

### 9.6 Dominators (critical for RE and compilers)

Node `d` **dominates** `n` if every path from entry to `n` passes through `d`.

```
        entry
          │
          ▼
          A ◄──────────┐        idom(A)=entry
         ╱ ╲           │        idom(B)=A   idom(C)=A
        ▼   ▼          │        idom(D)=A   (D reachable via B or C: only A is common)
        B   C          │
         ╲ ╱           │        back edge D -> A  ==> A is a natural-loop header
          ▼            │        (edge whose target dominates its source)
          D ───────────┘
```

Natural-loop detection = find edges `t → h` where `h` dominates `t`. This is how decompilers (Ghidra, Hex-Rays, rev.ng) recover `while`/`for` from raw jumps — and why **control-flow flattening** hurts: it makes one dispatcher block dominate everything.

### 9.7 Union-Find as a cousin

Connected components of an *undirected* graph can skip adjacency lists entirely: iterate the edge list, `union(u,v)`. Near-`O(E α(V))`. Use adjacency lists when you need traversal *structure*, DSU when you only need the *partition*.

---

## 10. Complexity, Cache Behavior, Hardware

### 10.1 Big-O hides the real cost: memory latency

```
Access                         Approx. latency (modern x86)
────────────────────────────── ─────────────────────────
L1 hit                          ~1 ns   (4-5 cycles)
L2 hit                          ~3-4 ns
L3 hit                          ~10-15 ns
DRAM (miss)                     ~60-100 ns
TLB miss + page walk            + 10s of ns
```

BFS on a huge graph is **latency-bound**: the next address (`dist[v]`, `head[v]`) depends on data just loaded and is effectively random.

```
Linked-list adjacency traversal of one row:

  head[u] ──► node A ──► node B ──► node C          each arrow = a dependent load
   (miss)      (miss)      (miss)      (miss)        CPU cannot prefetch: address unknown
                                                     until previous load completes
  cost ≈ deg × DRAM latency

CSR traversal of one row:

  off[u], off[u+1]  ──►  dst[a .. b]  (contiguous)
                          [ v v v v v v v v ][ v v v v ...     hardware prefetcher streams it;
                           64-byte line = 16 × u32              one miss amortized over 16 neighbors
  cost ≈ 1 miss + deg/16 line fetches
```

### 10.2 Where each representation wins

| Workload | Best form | Why |
|---|---|---|
| Frequent inserts/deletes | `Vec<Vec>` / linked list | `O(1)` mutation |
| Build once, traverse many times | CSR | contiguous, prefetch-friendly, minimal memory |
| `has_edge` dominant | sorted rows or hash sets | `O(log d)` / `O(1)` |
| Dense (>~3% bit-matrix, >~25% byte-matrix) | matrix / bitset rows | word-parallel ops, `O(1)` lookup |
| Streaming/evolving huge graphs | CSR + delta log, periodically merged | LSM-style hybrid |
| GPU | CSR / COO | coalesced access, edge-parallel kernels |

### 10.3 Vertex ordering matters

Renumbering vertices so neighbors have nearby ids improves locality of `dist[]`/`visited[]` accesses. Techniques: BFS order, Reverse Cuthill–McKee, degree-sorting (put hubs first), METIS partitioning. Gains of 2–5× on large real graphs are routine.

### 10.4 Direction-optimizing BFS (Beamer)

When the frontier is huge, top-down BFS wastes time scanning edges into already-visited vertices. Switch to **bottom-up**: every *unvisited* vertex scans its (reverse) neighbors for any frontier member and stops at the first hit.

```
Top-down:  for u in frontier:    for v in adj[u]:   if unvisited(v): claim v
Bottom-up: for v in unvisited:   for u in radj[v]:  if in_frontier(u): claim v; break
```

Requires `radj`. Used by Graph500 champions.

### 10.5 Bitset visited array

`visited` as a `Vec<bool>`/`char[]` costs 1 B/vertex; as a bitset 1 bit/vertex → 8× more of it fits in L1/L2. For 10⁸ vertices: 100 MB vs 12.5 MB — the difference between DRAM-bound and partly cache-resident.

---

## 11. Reverse Engineering View

Graphs are not a side topic in malware analysis — **they are the native data model of the tooling.**

### 11.1 Graphs you use every day

| Graph | Nodes | Edges | Where you see it |
|---|---|---|---|
| **CFG** (control-flow graph) | basic blocks | jumps/fallthrough | Ghidra Function Graph, IDA graph view, Binary Ninja |
| **Call graph** | functions | call sites | Ghidra Function Call Graph, BinDiff, capa (function/basic-block scopes) |
| **Data-flow / def-use graph** | SSA values | uses | decompiler IR, taint engines |
| **Import/dependency graph** | modules | DLL imports | pestudio, Dependency Walker |
| **Process tree** | processes | parent→child | Sysmon EID 1, EDR, Volatility `pstree` |
| **Injection/IPC graph** | processes | `WriteProcessMemory`, `CreateRemoteThread`, pipe/ALPC | EDR telemetry |
| **Lateral-movement graph** | hosts/accounts | logon events | BloodHound / AD attack paths (shortest path = BFS) |
| **Infrastructure graph** | domains, IPs, certs, samples | resolves-to, signed-by, drops | passive DNS pivoting, VirusTotal graph, MISP |
| **Malware-family graph** | samples | code-similarity | BinDiff/Diaphora clustering, imphash/ssdeep neighbors |

### 11.2 Basic block → adjacency list

```
 Disassembly                           CFG as adjacency list

 0x1000: cmp  eax, 5                   B0 [0x1000..0x1005): succ = {B1, B2}
 0x1003: jne  0x1010                   B1 [0x1005..0x1010): succ = {B3}
 0x1005: call decrypt                  B2 [0x1010..0x1018): succ = {B3}
 0x100a: jmp  0x1018                   B3 [0x1018..)      : succ = {}  (ret)
 0x1010: xor  ecx, ecx
 0x1012: ...                                B0
 0x1018: ret                               /  \
                                          v    v
                                         B1    B2
                                          \    /
                                           v  v
                                            B3

 Conditional jump => out-degree 2.  Unconditional jump/fallthrough => 1.  ret => 0.
 Indirect jump (jmp rax / jump table) => out-degree N or UNKNOWN: the hard case.
```

**Why CFG recovery is undecidable in general:** indirect branches (`jmp [rax*8+table]`, `call rax`, vtable dispatch, computed gotos, ROP-like obfuscators) mean successor sets depend on runtime values. Disassemblers approximate with jump-table pattern matching and value-set analysis; malware exploits every gap.

### 11.3 Graph *shape* as an obfuscation fingerprint

```
Normal function CFG            Control-flow-flattened CFG (OLLVM -fla, Tigress, commercial protectors)

     B0                                   ┌────────────┐
    ╱  ╲                                  │ dispatcher │◄─────────────┐
   B1   B2                                └──┬─┬─┬─┬───┘              │
    ╲  ╱                                     │ │ │ │                  │
     B3                                      ▼ ▼ ▼ ▼                  │
                                            B0 B1 B2 B3 ───(set state var, jmp)──┘

   Degrees: small, varied           Degrees: dispatcher has in-degree ≈ N and out-degree ≈ N
                                    (switch/jump table); every real block has out-degree 1 → dispatcher
```

**Detection heuristic you can implement on an adjacency list:**

```
for each function CFG:
    d = block with maximum in-degree
    if in_degree(d) >= 0.6 * |blocks|  and  out_degree(d) >= 0.6 * |blocks|:
        flag FLATTENING_SUSPECT
    also: fraction of blocks whose sole successor is d  (> 0.7 => strong signal)
```

Other graph-shape indicators:

* **Opaque predicates:** conditional edge where one successor is *never* taken (dead branch) — the CFG has extra edges that dataflow (constant/SMT) proves infeasible.
* **Junk-block insertion:** blocks with in-degree = out-degree = 1 forming long chains of no-op semantics.
* **Function fan-in outliers:** in the call graph, a tiny function with enormous in-degree and no imports is very often the **string-decryption or API-hash-resolver routine**. Sort functions by in-degree first; that is the first thing to rename.
* **Fan-out outliers:** a function with huge out-degree calling many `GetProcAddress`-resolved pointers is the **capability dispatcher/command handler** (typical RAT `switch(cmd)`).

### 11.4 What a graph traversal looks like in a compiled binary

Illustrative C `for (Edge *e = g->head[u]; e; e = e->next) visit(e->to);` at `-O2` (shape, not verbatim):

```asm
; rdi = Graph*, esi = u
    mov   rax, [rdi+8]          ; g->head           (n@0, m@4, head@8)
    mov   rbx, [rax+rsi*8]      ; e = head[u]       (array of pointers: stride 8)
    test  rbx, rbx
    jz    .done
.loop:
    mov   edi, [rbx]            ; e->to             (offset 0)
    call  visit
    mov   rbx, [rbx+8]          ; e = e->next       (offset 8)  <-- THE linked-list signature
    test  rbx, rbx
    jnz   .loop
.done:
```

**Signatures to train your eye on:**

| Source construct | Binary signature |
|---|---|
| Linked list walk | `mov reg,[reg+off]; test reg,reg; jnz` — load-chase-test loop, tight backward branch |
| C array of pointers (`head[u]`) | `[base + idx*8]` |
| CSR row iteration | two loads `off[u]`, `off[u+1]` (`[base+u*4]`, `[base+u*4+4]`), then a counted loop over `dst[i]` with stride 4 |
| Rust `Vec<Vec<T>>` index | `lea t,[u+u*2]` then `[base+t*8]` (×24 stride), `cmp u,len; jae panic_bounds_check` |
| Go `[][]T` index | same ×24 stride; bounds fail → `call runtime.panicIndex` |
| Rust `Vec::push` | inline `cmp len,cap; je → RawVec::grow_one`/`grow_amortized`, then store, `inc len` |
| Go `append` | `cmp len,cap` then `call runtime.growslice` on the cold path |
| BFS | queue (array + head/tail or `VecDeque` with wrap `& (cap-1)`), visited/dist array test-and-set before enqueue |
| DFS recursion | self-call inside the neighbor loop; deep recursion shows large stack frames / stack probes |

### 11.5 Graph algorithms inside malware

* **Ransomware directory walk:** DFS/BFS over the filesystem tree (`FindFirstFileW`/`FindNextFileW` recursion, or an explicit worklist to avoid stack exhaustion). Explicit-queue variants with multiple worker threads (I/O completion ports) are the fast ones. A directory tree *is* an adjacency list keyed by directory.
* **Worm/lateral movement:** target-selection over a host graph (ARP table, AD trusts, SMB neighbors) — often BFS from the initial host with a visited set to avoid reinfecting.
* **P2P botnets:** peer tables are adjacency lists (Kademlia buckets in some Zeus/Storm variants); takedown analysis = graph partitioning and sinkholing high-degree peers.
* **Dependency resolution in loaders:** reflective loaders/manual mappers resolve imports by traversing the export directory and forwarders — forwarder chains are a graph walk with cycle risk (`A.dll!Foo → B.dll!Bar → A.dll!Foo`); hardened loaders bound the depth.

### 11.6 Graph *defense* engineering

**Process-tree reconstruction pitfall — PID reuse.** Key nodes by `(PID, process-start-time)` or the EDR-provided GUID (Sysmon `ProcessGuid`), never PID alone. PIDs recycle; keying by PID splices unrelated processes into false parent/child edges.

```
 Sysmon EID 1 events  ──►  adjacency list keyed by ProcessGuid

   winword.exe  ──►  cmd.exe  ──►  powershell.exe  ──►  rundll32.exe
   (root)           (child)        (child)              (child, suspicious: LOLBin chain)

 Hunt = path query on this graph:
   ancestor in {winword, excel, outlook}  ∧  descendant in {powershell, mshta, rundll32, regsvr32, wscript}
```

Sigma expresses the *edge* (one parent→child hop); multi-hop needs graph queries in the SIEM/EDR:

```yaml
title: Office Application Spawning Script Interpreter or LOLBin
id: 8f2b0c1e-0000-4a9e-9a11-adjacency-demo
status: experimental
logsource:
  category: process_creation
  product: windows
detection:
  parent:
    ParentImage|endswith:
      - '\winword.exe'
      - '\excel.exe'
      - '\outlook.exe'
      - '\powerpnt.exe'
  child:
    Image|endswith:
      - '\cmd.exe'
      - '\powershell.exe'
      - '\pwsh.exe'
      - '\wscript.exe'
      - '\cscript.exe'
      - '\mshta.exe'
      - '\rundll32.exe'
      - '\regsvr32.exe'
  condition: parent and child
falsepositives:
  - Legitimate macro-driven business automation
level: high
tags:
  - attack.execution
  - attack.t1204.002     # User Execution: Malicious File
  - attack.t1059         # Command and Scripting Interpreter
```

(The `id` is a placeholder — generate a real UUIDv4 before deploying.)

**Attack-path analysis:** in AD, nodes = users/groups/computers, edges = `MemberOf`, `HasSession`, `AdminTo`, `GenericAll`, … "Shortest path from owned principal to Domain Admins" is **BFS on an adjacency list** — the same 20 lines as §6.2, weighted by edge "cost" if you use Dijkstra. Defensive value: remove the *edges* (permissions/sessions) that lie on the most shortest paths (high **edge betweenness**).

**Infrastructure pivoting:**

```
        sample.exe ──contacts──► c2.example ──resolves──► 203.0.113.9 ──hosts──► other-c2.example
             │                                                │
             └──signed_by──► cert(SHA1:..)  ◄─── same cert ───┘    => cluster; BFS depth 2-3 from a seed IOC
```

Bound your BFS depth and prune high-degree hubs (shared hosting, CDNs, sinkholes) or the cluster explodes into false attribution — a **degree cap is a precision control**.

### 11.7 Attacker perspective: graphs as targets

* **Untrusted graph input** (file formats with node/edge tables — PE resources, ELF sections, PDF object graphs, font tables, image indexed formats) parsed into adjacency structures is a classic bug source: unchecked vertex ids → OOB; cyclic references → infinite recursion / stack exhaustion (DoS); huge declared `n` → allocation bombs. PDF xref/object-stream cycles and font `CFF` subroutine recursion are historical examples.
* **Reference-cycle memory leaks / UAF** in refcounted graphs (COM objects, Objective-C, `Rc` in Rust) — and use-after-free when a node is freed while other rows still hold its id/pointer (§12).
* **Analyzer evasion by graph inflation:** protectors generate CFGs with 10⁵–10⁶ junk blocks to exhaust dominator/dataflow time in analysis tools. Know your tool's timeouts and analyze in *slices* (function-local, or backward slice from an API call of interest).

---

## 12. Pitfalls and Bugs

| # | Bug | Symptom | Fix |
|---|---|---|---|
| 1 | Marking visited on **dequeue** in BFS | Duplicates in queue, `O(E)` queue, wrong parents | Mark when **enqueuing** |
| 2 | Undirected edge added once | Asymmetric reachability | Add both arcs (self-loop once) |
| 3 | Self-loop added twice | Degree off by 2, duplicate visits | Guard `u != v` |
| 4 | Dijkstra with negative weight | Wrong distances, no error | Bellman–Ford / Johnson; **validate** on insert |
| 5 | `INT_MAX + w` overflow | Negative distance from wraparound | Skip if `dist[u] == INF`; use checked/saturating add |
| 6 | Recursive DFS on deep graph | Stack overflow (C/Rust) | Iterative DFS with cursor array |
| 7 | Modifying a row while iterating it | Skipped/duplicated neighbors, UAF (C), aliasing (Go) | Snapshot, defer mutations, or rely on Rust's borrow checker |
| 8 | Vertex deletion leaves stale references | Dangling id → wrong node or OOB | Tombstones + generation counter, or scrub via `radj` |
| 9 | Re-using a `visited` array between runs | Later traversals see stale marks | Reset, or use "epoch" stamps: `seen[v] == epoch` |
| 10 | Weighted comparison with `float` and `==` | Nondeterministic ties | Integer weights, or epsilon + tiebreaker |
| 11 | PID-keyed process graph | Phantom parent-child edges | Key by `(pid, start_time)` / GUID |
| 12 | Trusting vertex ids from input | OOB read/write | Validate `0 <= id < n` at the boundary, once |
| 13 | Off-by-one in CSR `off` | Row bleeds into next row | `off` has `n+1` entries; last = `m` |
| 14 | Topological sort on cyclic graph, ignoring count | Partial order silently returned | Check `emitted == n` |

**Epoch trick (avoids `O(V)` reset per query):**

```c
static uint32_t seen[N]; static uint32_t epoch = 0;
void bfs(int s) { epoch++;  /* on wrap to 0, memset(seen,0,..) and set epoch=1 */
    ... if (seen[v] != epoch) { seen[v] = epoch; enqueue(v); } ... }
```

---

## 13. Challenges

*(Deliberate practice — attempt before looking anything up.)*

**C1 — Derive.** Prove that BFS on a graph with `V` vertices and `E` arcs in adjacency-list form runs in `Θ(V+E)` — and state precisely why the *same* algorithm on an adjacency matrix is `Θ(V²)`. Which term of `V+E` is the initialization and which is the traversal?

**C2 — Implement.** Extend the CSR builder (any of C/Go/Rust) to produce **sorted rows with duplicate arcs merged (min weight)**. Then implement `has_edge(u,v)` in `O(log deg(u))`. Confirm with a property test that it matches the naive `O(deg)` scan on 10⁵ random graphs.

**C3 — Break it.** In the C linked-list version, write a 5-line program that triggers a use-after-free using only the public API (`remove_arc` + a saved `Edge*`), catch it with ASan, then redesign the API so it cannot happen. What did Rust's `&[Edge]` return type make *impossible* by construction?

**C4 — Detect.** Given a CFG as an adjacency list (dump from Ghidra's `BasicBlockModel` via a Python script), implement the flattening heuristic of §11.3. Test it on an OLLVM `-mllvm -fla` build of a small C program vs the same program at `-O2`. Report the in-degree distribution of both. Which threshold separates them, and how would an adversary tune obfuscation to evade *your* threshold? (Adversarial thinking: what would APT-grade tooling do that OLLVM doesn't?)

**C5 — Hunt.** Build a process tree from Sysmon EID 1 JSON (keyed by `ProcessGuid`). Write a DFS that reports every path of length ≥ 3 that starts at an Office binary and ends at a network-connecting process (join EID 3). Explain why a single-hop Sigma rule misses `winword → explorer → cmd` (parent spoofing via `PROC_THREAD_ATTRIBUTE_PARENT_PROCESS`, MITRE T1134.004) and what telemetry lets you reconstruct the *true* parent.

**C6 — Feynman.** In five sentences, without the words "node", "edge", or "vertex", explain to a colleague why a linked-list adjacency list is usually *slower in practice* than a CSR despite identical asymptotics. If you had to use the word "cache", have you explained *why* the cache cares? (Hint: dependent loads, prefetchability, spatial locality.)

**C7 — Reverse.** Compile the C BFS from §6.2 at `-O0`, `-O2`, and `-O3 -march=native`. In Ghidra, locate the neighbor loop in each. Identify: the `e->next` load, the `dist[e->to] == -1` test, and where the compiler hoisted `g->head` out of the loop. What changed between `-O0` and `-O2` in how `qt` is held?

**C8 — Scale.** You have 200M edges, 20M vertices, 16 GB RAM. Estimate the memory of (a) malloc'd linked lists, (b) `Vec<Vec<(u32,u32)>>`, (c) CSR with `u32`. Which of them even fit? What changes if the graph is *streamed* from disk?

<details>
<summary>Answer sketch for C8 (try first)</summary>

```
(a) 200M × 32 B  + 20M × 8 B  ≈ 6.4 GB + 0.16 GB ≈ 6.6 GB   (fits, but every hop is a cache miss)
(b) 200M × 8 B   + 20M × (24 B header + ~16 B alloc overhead) ≈ 1.6 GB + 0.8 GB ≈ 2.4 GB,
    plus up to ~50% slack from doubling growth on the rows  ≈ 3 GB
(c) 200M × 4 B (dst) + 200M × 4 B (weights) + 20M × 4 B (off) = 0.8 + 0.8 + 0.08 ≈ 1.7 GB   (unweighted: 0.9 GB)

CSR wins ~4x over (a), plus far better locality. Streaming => semi-external algorithms:
keep O(V) state (dist/visited ≈ 80-160 MB) in RAM, stream `dst` sequentially from disk per pass.
```
</details>

---

## 14. The Expert Mental Model

**An adjacency list is a function from vertex to neighbor-set, stored so that "expand this vertex" costs exactly its degree and nothing else.** Everything else is a choice of *how that function is laid out in memory*: pointers chasing through the heap (flexible, slow, 32 B/edge), per-row growable arrays (the pragmatic default), or two flat arrays in CSR (`offsets` + `targets`) where the whole graph becomes a contiguous stream the CPU can prefetch. Asymptotics say `O(V+E)` for all three; the machine says a 10–50× spread. A top-tier analyst keeps both views at once: the algorithm as a *frontier discipline* (queue = BFS, stack = DFS, heap = Dijkstra, in-degree-zero = topological sort), and the data structure as a *memory-access pattern* — because that is what appears in the disassembly (`mov r,[r+8]; test; jnz` for a linked walk, a `×24` stride for `Vec<Vec>`), what governs analyzer performance on million-block protected binaries, and what an attacker abuses when they feed a parser a hostile vertex id. In your daily work the same object reappears at every layer — CFGs, call graphs, process trees, AD permissions, C2 infrastructure — and the *shape* of the graph (a dispatcher with in-degree ≈ N, a decrypt routine with absurd fan-in, a hub IP with thousands of neighbors) is itself a detection signal. **Learn to read structure from degree distributions, and to choose the representation from the workload, not from habit.**

---

### Appendix A — Cheat sheet

```
Undirected sum(deg) = 2E          Directed sum(out) = sum(in) = E
BFS: queue, mark on ENQUEUE       DFS: back edge <=> cycle (directed)
Topo: reverse postorder OR Kahn (emitted == n else cycle)
Dijkstra: w >= 0, lazy deletion, skip stale (d != dist[v])
SCC: Tarjan (low-link) or Kosaraju (needs transpose) -> condensation is a DAG
CSR: off[n+1], dst[m]; build = histogram -> prefix sum -> scatter
Sparse -> adjacency list.  Read-mostly + big -> CSR.  Dense or bitset ops -> matrix.
```

### Appendix B — Build and test commands

```bash
# C
gcc -std=c11 -O2 -Wall -Wextra -fsanitize=address,undefined graph.c -o graph && ./graph

# Go (1.21+ for builtin min)
go run graph.go

# Rust
cargo new adjlist && cd adjlist   # paste §8 into src/main.rs
cargo run --release
```

Expected output on the canonical graph: BFS distances `0 1 1 2 2 3`, Dijkstra `0 4 2 5 5 7`, cycle detected (`1→3→4→1`), SCC sizes `{5}, {1,3,4}, {2}, {0}`.

### Appendix C — Further study

* CLRS Ch. 20–24 (elementary graph algorithms, SSSP, MST); Sedgewick & Wayne *Algorithms* Ch. 4.
* Tarjan (1972) *Depth-first search and linear graph algorithms*; Cooper–Harvey–Kennedy *A Simple, Fast Dominance Algorithm*.
* Beamer et al. *Direction-Optimizing Breadth-First Search* (SC'12); the GAP Benchmark Suite.
* `petgraph` source (Rust); Go `golang.org/x/tools/go/callgraph`; LLVM `Dominators.h`; Ghidra `BasicBlockModel` / `ghidra.graph` package.
* Reverse-engineering graphs: BinDiff/Diaphora papers (graph-isomorphism-based function matching), Zynamics *Structural Comparison of Executable Objects*, Hex-Rays microcode CFG docs, OLLVM / Tigress obfuscation write-ups and deobfuscation research (e.g., Quarkslab, Synacktiv posts on flattening removal via symbolic execution).
