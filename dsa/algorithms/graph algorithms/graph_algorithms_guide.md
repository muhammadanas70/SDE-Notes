# The Complete Graph Algorithms Guide
### Mental Models, ASCII Architecture, and Production Implementations in Go, C, and Rust

---

## Table of Contents

1. [What Is a Graph, Really?](#1-what-is-a-graph-really)
2. [Graph Representations](#2-graph-representations)
3. [Breadth-First Search (BFS)](#3-breadth-first-search-bfs)
4. [Depth-First Search (DFS)](#4-depth-first-search-dfs)
5. [Connected Components](#5-connected-components)
6. [Cycle Detection](#6-cycle-detection)
7. [Topological Sorting](#7-topological-sorting)
8. [Shortest Path Algorithms](#8-shortest-path-algorithms)
9. [Minimum Spanning Trees](#9-minimum-spanning-trees)
10. [Union-Find / Disjoint Set Union](#10-union-find--disjoint-set-union)
11. [Strongly Connected Components](#11-strongly-connected-components)
12. [Complexity Cheat Sheet](#12-complexity-cheat-sheet)
13. [Decision Framework: Which Algorithm When](#13-decision-framework-which-algorithm-when)

---

## 1. What Is a Graph, Really?

### 1.1 The Core Idea

A graph is the most general data structure for representing **relationships**. Where an array models a sequence and a tree models a strict hierarchy, a graph makes no assumption about structure at all — any node can relate to any other node, in any pattern.

Formally: `G = (V, E)` where `V` is a set of **vertices** (nodes) and `E` is a set of **edges** (connections between vertices).

**Why it exists:** Trees are graphs with a constraint (no cycles, single parent). Linked lists are graphs with an even tighter constraint (each node has exactly one successor). The moment your data has relationships that don't fit those constraints — a road network where cities connect to many other cities, a social network where friendship is mutual, a build system where tasks depend on other tasks — you need the unconstrained model. That model is the graph.

**What simpler structures fail to capture:** An array can tell you "these items exist in this order." A tree can tell you "these items exist in a strict hierarchy." Neither can express "city A connects to city B, C, and D, and city B independently connects back to A and also to E." Graphs are the only structure that captures many-to-many, potentially cyclic relationships.

### 1.2 Types of Graphs

```text
DIRECTED vs UNDIRECTED

Undirected (edge is symmetric)      Directed (edge has direction)
                                     
    A --- B                             A ---> B
    |     |                             ^      |
    |     |                             |      v
    C --- D                             D <--- C

"A connects to B"                    "A points to B"
implies "B connects to A"            does NOT imply "B points to A"


WEIGHTED vs UNWEIGHTED

Unweighted                          Weighted
    A --- B                             A --5-- B
    |     |                             |       |
    |     |                             3       7
    C --- D                             |       |
                                         C --2-- D

Every edge costs "1" to traverse    Each edge carries a cost/distance


CYCLIC vs ACYCLIC

Cyclic                               Acyclic (DAG if directed)
  A -> B                               A -> B
  ^    |                                    |
  |    v                                    v
  D <- C                               D    C
(A->B->C->D->A is a cycle)          (no path returns to start)


SPARSE vs DENSE
Sparse: |E| is close to |V|          Dense: |E| is close to |V|^2
(most real-world graphs:             (e.g. a graph where almost
 road networks, social graphs,        every pair of nodes is
 dependency graphs)                   connected — rare in practice)
```

### 1.3 The Vocabulary You Must Own

| Term | Meaning |
|---|---|
| Vertex / Node | A single entity in the graph |
| Edge | A connection between two vertices |
| Degree | Number of edges touching a vertex (undirected) |
| In-degree / Out-degree | Edges pointing in / edges pointing out (directed) |
| Path | A sequence of vertices connected by edges, no repeats |
| Cycle | A path that starts and ends at the same vertex |
| Connected | Every vertex reachable from every other (undirected) |
| Strongly connected | Every vertex reachable from every other via directed edges |
| Weight / Cost | A number attached to an edge |
| Adjacency | Two vertices are adjacent if an edge connects them |

### 1.4 Real-World Systems Built on Graphs

- **Road/flight networks** → shortest paths (Dijkstra), routing
- **Build systems (Make, Bazel, `go build`, Cargo)** → topological sort of dependencies
- **Social networks** → BFS for "degrees of separation," connected components for communities
- **Compilers** → control-flow graphs, dependency graphs, SCC for detecting circular imports
- **Filesystems / package managers** → DAGs, cycle detection to reject circular `import`/`require`
- **Networking** → MST for minimum-cost network wiring, max-flow for bandwidth
- **Version control** → commit graphs (DAGs) with merge/rebase as graph operations

Every one of these is the *same* underlying structure. Once your mental model of a graph is solid, all of these domains become instances of one idea.


---

## 2. Graph Representations

Before any algorithm, you must decide **how the graph lives in memory**. This decision changes the time complexity of every algorithm you run on it. There are three standard representations.

### 2.1 Adjacency Matrix

**Mental model:** a 2D grid where `matrix[i][j] = 1` (or weight) means "an edge exists from `i` to `j`."

```text
Graph:                    Adjacency Matrix (|V| x |V|)
                                 A  B  C  D
   A --- B                   A [ 0, 1, 1, 0 ]
   |     |                   B [ 1, 0, 0, 1 ]
   C --- D                   C [ 1, 0, 0, 1 ]
                              D [ 0, 1, 1, 0 ]

matrix[A][B] = 1  ->  edge exists between A and B
matrix[A][C] = 1  ->  edge exists between A and C
matrix[A][D] = 0  ->  no direct edge
```

- **Space:** `O(V^2)` always, regardless of edge count.
- **Edge existence check `(u, v)`:** `O(1)` — direct array index.
- **Iterate neighbors of `u`:** `O(V)` — must scan the entire row even if `u` has 1 neighbor.
- **Best for:** dense graphs, or when you need frequent `O(1)` edge-existence queries (e.g. Floyd–Warshall, which is fundamentally matrix-shaped).
- **Bad for:** sparse graphs (wastes massive memory) and any algorithm that needs to enumerate neighbors repeatedly (BFS/DFS become `O(V^2)` instead of `O(V+E)`).

### 2.2 Adjacency List

**Mental model:** for every vertex, keep a list of only its actual neighbors.

```text
Graph:                    Adjacency List
   A --- B                A -> [B, C]
   |     |                B -> [A, D]
   C --- D                C -> [A, D]
                           D -> [B, C]

Each vertex only stores what it's actually connected to.
No wasted space for non-edges.
```

- **Space:** `O(V + E)` — proportional to what actually exists.
- **Edge existence check `(u, v)`:** `O(degree(u))` — must scan `u`'s list (or `O(1)` with a hash-set-backed list).
- **Iterate neighbors of `u`:** `O(degree(u))` — exactly the work needed, nothing wasted.
- **Best for:** almost everything in practice. Sparse real-world graphs (roads, social networks, dependency graphs) make this the default choice. BFS/DFS run in optimal `O(V + E)`.

**This is the representation you should reach for by default.** The matrix is a specialized tool for dense graphs or algorithms that are inherently matrix-shaped (Floyd–Warshall).

### 2.3 Edge List

**Mental model:** just a flat list of `(u, v, weight)` triples — no per-vertex structure at all.

```text
Graph:                    Edge List
   A --- B                [(A,B,1), (A,C,1), (B,D,1), (C,D,1)]
   |     |
   C --- D                Just the raw edges, unsorted, unindexed.
```

- **Space:** `O(E)`.
- **Best for:** algorithms that process edges as a flat sequence regardless of vertex structure — most notably **Kruskal's MST** (which sorts all edges by weight) and **Bellman–Ford** (which relaxes every edge, `|V|-1` times, in any order).
- **Bad for:** anything needing "give me the neighbors of vertex X" — you'd have to scan the whole list, `O(E)`.

### 2.4 Side-by-Side Comparison

| Operation | Adjacency Matrix | Adjacency List | Edge List |
|---|---|---|---|
| Space | O(V²) | O(V + E) | O(E) |
| Check edge (u,v) exists | O(1) | O(degree(u)) | O(E) |
| Enumerate neighbors of u | O(V) | O(degree(u)) | O(E) |
| Enumerate all edges | O(V²) | O(V + E) | O(E) |
| Add an edge | O(1) | O(1) | O(1) |
| Remove an edge | O(1) | O(degree(u)) | O(E) |
| Good for dense graphs | Yes | OK | OK |
| Good for sparse graphs | Wasteful | Yes | Yes |
| Natural fit for | Floyd–Warshall | BFS, DFS, Dijkstra | Kruskal, Bellman–Ford |

### 2.5 Cross-Language Implementation

| Aspect | Go | C | Rust |
|---|---|---|---|
| Adjacency list container | `map[int][]int` or `[][]int` | array of dynamically-grown `int*` arrays (manual `malloc`/`realloc`) | `Vec<Vec<usize>>` or `HashMap<usize, Vec<usize>>` |
| Weighted edge | `struct{ to, weight int }` | `struct Edge { int to, weight; }` | `struct Edge { to: usize, weight: i64 }` |
| Memory ownership | Garbage collected — no manual free | Programmer owns every `malloc`, must `free` | Ownership system — `Vec` drops automatically, borrow checker prevents dangling pointers |
| Growth strategy | Built-in slice `append` (amortized doubling) | Manual `realloc` (you implement doubling yourself) | Built-in `Vec::push` (amortized doubling) |

#### Go Implementation

```go
package main

import "fmt"

// Graph using adjacency list. Unweighted, directed (undirected = add both directions).
type Graph struct {
	numVertices int
	adjList     [][]int
}

func NewGraph(n int) *Graph {
	return &Graph{
		numVertices: n,
		adjList:     make([][]int, n),
	}
}

func (g *Graph) AddEdge(u, v int) {
	g.adjList[u] = append(g.adjList[u], v)
}

func (g *Graph) AddUndirectedEdge(u, v int) {
	g.adjList[u] = append(g.adjList[u], v)
	g.adjList[v] = append(g.adjList[v], u)
}

// Weighted version
type WeightedEdge struct {
	To     int
	Weight int
}

type WeightedGraph struct {
	numVertices int
	adjList     [][]WeightedEdge
}

func NewWeightedGraph(n int) *WeightedGraph {
	return &WeightedGraph{numVertices: n, adjList: make([][]WeightedEdge, n)}
}

func (g *WeightedGraph) AddEdge(u, v, w int) {
	g.adjList[u] = append(g.adjList[u], WeightedEdge{To: v, Weight: w})
}

func main() {
	g := NewGraph(4) // vertices 0,1,2,3 represent A,B,C,D
	g.AddUndirectedEdge(0, 1) // A-B
	g.AddUndirectedEdge(0, 2) // A-C
	g.AddUndirectedEdge(1, 3) // B-D
	g.AddUndirectedEdge(2, 3) // C-D

	for v := 0; v < g.numVertices; v++ {
		fmt.Printf("Vertex %d -> %v\n", v, g.adjList[v])
	}
}
```

**Key Go notes:**
- `[][]int` (slice of slices) is the idiomatic adjacency list — each inner slice grows via `append`'s amortized-doubling reallocation, which is a **heap allocation** the Go runtime manages; you never call `free`.
- If vertices are non-integer (strings, structs), switch to `map[string][]string`; lookups become `O(1)` average via Go's hash map, at the cost of hashing overhead per access.
- Passing `*Graph` (pointer receiver) avoids copying the whole struct on every method call — `adjList` is a slice header (pointer+len+cap) so it's cheap to copy, but the struct convention of pointer receivers keeps mutation semantics consistent.

#### C Implementation

```c
#include <stdio.h>
#include <stdlib.h>

// Adjacency list node (linked-list style, the classic C approach)
typedef struct AdjNode {
    int dest;
    int weight;
    struct AdjNode *next;
} AdjNode;

typedef struct Graph {
    int numVertices;
    AdjNode **adjList; // array of linked-list heads, one per vertex
} Graph;

AdjNode *createNode(int dest, int weight) {
    AdjNode *node = (AdjNode *)malloc(sizeof(AdjNode));
    if (!node) { fprintf(stderr, "malloc failed\n"); exit(1); }
    node->dest = dest;
    node->weight = weight;
    node->next = NULL;
    return node;
}

Graph *createGraph(int numVertices) {
    Graph *g = (Graph *)malloc(sizeof(Graph));
    g->numVertices = numVertices;
    g->adjList = (AdjNode **)malloc(numVertices * sizeof(AdjNode *));
    for (int i = 0; i < numVertices; i++) {
        g->adjList[i] = NULL; // empty list initially
    }
    return g;
}

// Prepend to the head of u's list — O(1), but reverses insertion order
void addEdge(Graph *g, int u, int v, int weight) {
    AdjNode *node = createNode(v, weight);
    node->next = g->adjList[u];
    g->adjList[u] = node;
}

void addUndirectedEdge(Graph *g, int u, int v, int weight) {
    addEdge(g, u, v, weight);
    addEdge(g, v, u, weight);
}

void printGraph(Graph *g) {
    for (int v = 0; v < g->numVertices; v++) {
        printf("Vertex %d ->", v);
        AdjNode *cur = g->adjList[v];
        while (cur != NULL) {
            printf(" %d", cur->dest);
            cur = cur->next;
        }
        printf("\n");
    }
}

// CRITICAL: manual memory management — every malloc needs a matching free
void freeGraph(Graph *g) {
    for (int i = 0; i < g->numVertices; i++) {
        AdjNode *cur = g->adjList[i];
        while (cur != NULL) {
            AdjNode *tmp = cur;
            cur = cur->next;
            free(tmp);
        }
    }
    free(g->adjList);
    free(g);
}

int main(void) {
    Graph *g = createGraph(4);
    addUndirectedEdge(g, 0, 1, 1);
    addUndirectedEdge(g, 0, 2, 1);
    addUndirectedEdge(g, 1, 3, 1);
    addUndirectedEdge(g, 2, 3, 1);

    printGraph(g);
    freeGraph(g); // without this line: memory leak, every single run
    return 0;
}
```

**Key C notes:**
- Each vertex's adjacency list is a classic **singly linked list of heap-allocated nodes**. This is the traditional C approach because C has no built-in dynamic array — you either hand-roll a resizable array (`malloc`+`realloc`+track `size`/`capacity`) or use linked nodes. Linked nodes avoid `realloc` churn but cost a pointer per node and poor cache locality (each `next` hop may be a cache miss, unlike a contiguous array).
- **Every `malloc` must be paired with a `free`.** `freeGraph` walks every list and frees every node before freeing the array of heads and the struct itself. Forget this and you leak memory on every graph you build — there's no garbage collector to save you.
- `addEdge` prepends (`O(1)`) rather than appends (`O(n)` without a tail pointer), which is why edges print in reverse insertion order. This is a deliberate, common C-specific trade-off.

#### Rust Implementation

```rust
use std::collections::HashMap;

// Adjacency list using Vec<Vec<usize>> for dense integer vertex IDs.
struct Graph {
    num_vertices: usize,
    adj_list: Vec<Vec<usize>>,
}

impl Graph {
    fn new(n: usize) -> Self {
        Graph {
            num_vertices: n,
            adj_list: vec![Vec::new(); n],
        }
    }

    fn add_edge(&mut self, u: usize, v: usize) {
        self.adj_list[u].push(v);
    }

    fn add_undirected_edge(&mut self, u: usize, v: usize) {
        self.adj_list[u].push(v);
        self.adj_list[v].push(u);
    }
}

// Weighted variant
#[derive(Debug, Clone, Copy)]
struct WeightedEdge {
    to: usize,
    weight: i64,
}

struct WeightedGraph {
    adj_list: Vec<Vec<WeightedEdge>>,
}

impl WeightedGraph {
    fn new(n: usize) -> Self {
        WeightedGraph { adj_list: vec![Vec::new(); n] }
    }
    fn add_edge(&mut self, u: usize, v: usize, weight: i64) {
        self.adj_list[u].push(WeightedEdge { to: v, weight });
    }
}

fn main() {
    let mut g = Graph::new(4); // 0..3 represent A,B,C,D
    g.add_undirected_edge(0, 1);
    g.add_undirected_edge(0, 2);
    g.add_undirected_edge(1, 3);
    g.add_undirected_edge(2, 3);

    for v in 0..g.num_vertices {
        println!("Vertex {} -> {:?}", v, g.adj_list[v]);
    }
}
```

**Key Rust notes:**
- `Vec<Vec<usize>>` is idiomatic when vertices are dense integers `0..n`. `self.adj_list[u].push(v)` is a **heap allocation** managed automatically — when `Graph` goes out of scope, every inner `Vec` is dropped recursively. No `free`, no leak, and the borrow checker guarantees no dangling references at compile time.
- Note `&mut self` on `add_edge`: Rust's ownership model requires an explicit mutable borrow to mutate `adj_list`. This is the compile-time enforcement of "only one mutator at a time" — the same invariant C trusts the programmer to maintain and Go doesn't enforce at all (data races are a runtime concern in Go, a compile-time-prevented impossibility in safe Rust).
- If vertex IDs are not small dense integers, use `HashMap<K, Vec<K>>` (shown imported above) instead of `Vec<Vec<usize>>` — same asymptotic behavior as Go's `map[K][]K`, backed by a hash table rather than direct indexing.

### 2.6 Cross-Language Comparison Table

| Aspect | Go | Rust |
|---|---|---|
| Dynamic arrays | `[]T` | `Vec<T>` |
| Hash maps | `map[K]V` | `HashMap<K, V>` |
| Null-like absence | `nil` slice/pointer | `Option<T>` |
| Memory management | Garbage collected | Ownership and borrowing |
| Linked structures | Pointers and structs, GC-managed | `Box`, references, ownership-tracked |
| Growth strategy | `append` (amortized 2x growth) | `Vec::push` (amortized 2x growth) |
| Mutation safety | Runtime only (data races possible) | Compile-time enforced (borrow checker) |


---

## 3. Breadth-First Search (BFS)

### 3.1 Concept

BFS explores a graph **level by level** — it visits every vertex at distance 1 from the start, then every vertex at distance 2, then distance 3, and so on. It never goes "deep" before going "wide."

**Why it exists:** if you want the *shortest path in terms of number of edges* (unweighted shortest path), BFS is the only search order that guarantees you discover each vertex through the shortest possible route the first time you see it. Depth-first exploration gives no such guarantee — it might stumble onto a vertex through a needlessly long path first.

### 3.2 Mental Model

BFS maintains a **queue** (FIFO) of vertices to visit, and a **visited set** to avoid processing any vertex twice.

**Invariant:** at the moment all vertices at distance `d` have been dequeued, the queue contains exactly the vertices at distance `d+1` — no more, no less. This is what makes BFS process the graph in strict, complete layers.

```text
Graph:                    BFS from A, level by level:

      A                   Level 0:  A
     / \                  Level 1:  B, C
    B   C                 Level 2:  D, E
    |   |
    D   E

Queue evolution (FIFO — enqueue at back, dequeue from front):

  Step 0: [A]                          visited = {A}
  Step 1: dequeue A, enqueue B,C       queue=[B,C]     visited={A,B,C}
  Step 2: dequeue B, enqueue D         queue=[C,D]     visited={A,B,C,D}
  Step 3: dequeue C, enqueue E         queue=[D,E]     visited={A,B,C,D,E}
  Step 4: dequeue D, no new neighbors  queue=[E]
  Step 5: dequeue E, no new neighbors  queue=[]        DONE
```

### 3.3 Dry Run

| Step | Queue (front→back) | Dequeued | Newly Enqueued | Visited Set |
|---|---|---|---|---|
| 1 | [A] | — | — | {A} |
| 2 | [] | A | B, C | {A, B, C} |
| 3 | [C] | B | D | {A, B, C, D} |
| 4 | [D] | C | E | {A, B, C, D, E} |
| 5 | [E] | D | — | {A, B, C, D, E} |
| 6 | [] | E | — | {A, B, C, D, E} |

### 3.4 Algorithm Derivation

1. **Problem:** given a graph and a start vertex `s`, visit every reachable vertex such that vertices closer to `s` (in edge count) are visited before farther ones.
2. **Naive idea:** recursively explore as far as possible (DFS) — fails the requirement, since DFS can reach a far vertex before a near one.
3. **Key observation:** if we always expand the *oldest discovered, not-yet-expanded* vertex first, we naturally process layer-by-layer. "Oldest discovered first" is exactly FIFO order — a queue.
4. **Invariant to maintain:** every vertex enters the queue exactly once, at the moment it is first discovered (not when it's processed). Mark it visited *at discovery time*, not at dequeue time — otherwise the same vertex can be enqueued multiple times by different neighbors before it's ever processed.
5. **Correctness sketch:** by induction on distance `d`. Base case: distance-0 vertex (the source) is trivially discovered first. Inductive step: if all distance-`d` vertices are dequeued before any distance-`(d+1)` vertex, then every distance-`(d+1)` vertex is discovered only while dequeuing some distance-`d` vertex (since a distance-`(d+1)` vertex's shortest path must pass through a distance-`d` vertex), so all distance-`(d+1)` vertices enter the queue before any distance-`(d+2)` vertex can be discovered. The layers never interleave.

### 3.5 Complexity Analysis

- Every vertex is enqueued and dequeued **exactly once**: `O(V)`.
- Every edge is examined **exactly once** (when we scan the adjacency list of the vertex it originates from): `O(E)`.
- **Total time: `O(V + E)`.**
- **Space:** the queue, visited set, and (optionally) a `parent`/`distance` array are each `O(V)` → **`O(V)` auxiliary space.**

This is optimal — you cannot discover a graph's structure while touching fewer than every vertex and every edge.

### 3.6 Go Implementation

```go
package main

import "fmt"

func BFS(adjList [][]int, start int) (distance []int, parent []int) {
	n := len(adjList)
	distance = make([]int, n)
	parent = make([]int, n)
	visited := make([]bool, n)

	for i := range distance {
		distance[i] = -1 // -1 = unreached
		parent[i] = -1
	}

	queue := []int{start}
	visited[start] = true
	distance[start] = 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:] // dequeue (O(1) amortized on a slice used this way is fine for teaching; use a ring buffer for production)

		for _, neighbor := range adjList[current] {
			if !visited[neighbor] {
				visited[neighbor] = true // mark visited at DISCOVERY time
				distance[neighbor] = distance[current] + 1
				parent[neighbor] = current
				queue = append(queue, neighbor)
			}
		}
	}
	return distance, parent
}

func main() {
	adj := [][]int{
		0: {1, 2}, // A -> B, C
		1: {0, 3}, // B -> A, D
		2: {0, 4}, // C -> A, E
		3: {1},    // D -> B
		4: {2},    // E -> C
	}
	dist, parent := BFS(adj, 0)
	fmt.Println("Distances:", dist)
	fmt.Println("Parents:  ", parent)
}
```

**Key Go notes:**
- Marking `visited[neighbor] = true` **when enqueuing**, not when dequeuing, is the single most important correctness detail — omit this and a vertex with multiple in-edges from the same layer gets enqueued multiple times, wasting work (though correctness survives, since it becomes idempotent — the *complexity* degrades toward `O(E)` extra work).
- `queue = queue[1:]` re-slices rather than copies — Go's slice header just moves its start pointer forward, `O(1)`. In long-running production code you'd use a proper ring-buffer-backed queue to avoid the underlying array only ever growing.

### 3.7 C Implementation

```c
#include <stdio.h>
#include <stdlib.h>

typedef struct AdjNode { int dest; struct AdjNode *next; } AdjNode;

typedef struct Queue {
    int *data;
    int head, tail, capacity;
} Queue;

Queue *createQueue(int capacity) {
    Queue *q = malloc(sizeof(Queue));
    q->data = malloc(capacity * sizeof(int));
    q->head = q->tail = 0;
    q->capacity = capacity;
    return q;
}
void enqueue(Queue *q, int val) { q->data[q->tail++] = val; }
int dequeue(Queue *q) { return q->data[q->head++]; }
int isEmpty(Queue *q) { return q->head == q->tail; }

// bfs over an adjacency-list graph represented as AdjNode* heads[]
void bfs(AdjNode **adjList, int numVertices, int start, int *distance, int *parent) {
    int *visited = calloc(numVertices, sizeof(int));
    for (int i = 0; i < numVertices; i++) { distance[i] = -1; parent[i] = -1; }

    Queue *q = createQueue(numVertices); // max V entries ever in the queue at once
    enqueue(q, start);
    visited[start] = 1;
    distance[start] = 0;

    while (!isEmpty(q)) {
        int current = dequeue(q);
        AdjNode *cur = adjList[current];
        while (cur != NULL) {
            int neighbor = cur->dest;
            if (!visited[neighbor]) {
                visited[neighbor] = 1;
                distance[neighbor] = distance[current] + 1;
                parent[neighbor] = current;
                enqueue(q, neighbor);
            }
            cur = cur->next;
        }
    }
    free(visited);
    free(q->data);
    free(q);
}

int main(void) {
    // Build A(0)-B(1), A-C(2), B-D(3), C-E(4)
    int n = 5;
    AdjNode *adjList[5] = {NULL, NULL, NULL, NULL, NULL};
    // helper omitted for brevity — assume addUndirectedEdge from Section 2 is used
    int distance[5], parent[5];
    bfs(adjList, n, 0, distance, parent);
    for (int i = 0; i < n; i++) printf("dist[%d]=%d parent[%d]=%d\n", i, distance[i], i, parent[i]);
    return 0;
}
```

**Key C notes:**
- The queue is a **fixed-capacity array with head/tail indices** — safe here because a vertex enters the queue at most once, so capacity `numVertices` is a hard, provable upper bound. This avoids `malloc`/`realloc` churn during the BFS itself.
- `calloc` (not `malloc`) zero-initializes `visited`, which is required since `0` means "not visited."
- Every `malloc`'d structure (`visited`, `q->data`, `q`) is freed at the end — in C, forgetting even one of these on a hot path is a slow memory leak that will eventually crash a long-running server.

### 3.8 Rust Implementation

```rust
use std::collections::VecDeque;

fn bfs(adj_list: &Vec<Vec<usize>>, start: usize) -> (Vec<i64>, Vec<Option<usize>>) {
    let n = adj_list.len();
    let mut distance = vec![-1i64; n];
    let mut parent: Vec<Option<usize>> = vec![None; n];
    let mut visited = vec![false; n];
    let mut queue: VecDeque<usize> = VecDeque::new();

    visited[start] = true;
    distance[start] = 0;
    queue.push_back(start);

    while let Some(current) = queue.pop_front() {
        for &neighbor in &adj_list[current] {
            if !visited[neighbor] {
                visited[neighbor] = true; // mark at discovery time
                distance[neighbor] = distance[current] + 1;
                parent[neighbor] = Some(current);
                queue.push_back(neighbor);
            }
        }
    }
    (distance, parent)
}

fn main() {
    let adj_list: Vec<Vec<usize>> = vec![
        vec![1, 2], // A -> B, C
        vec![0, 3], // B -> A, D
        vec![0, 4], // C -> A, E
        vec![1],    // D -> B
        vec![2],    // E -> C
    ];
    let (distance, parent) = bfs(&adj_list, 0);
    println!("Distances: {:?}", distance);
    println!("Parents:   {:?}", parent);
}
```

**Key Rust notes:**
- `VecDeque<usize>` is a genuine ring-buffer-backed double-ended queue — `pop_front` is a real `O(1)` operation, not a re-slice trick. This is the correct production queue, unlike the teaching-simplified Go `queue[1:]` shown above.
- `parent: Vec<Option<usize>>` uses `Option<usize>` instead of a sentinel `-1` — this is the idiomatic Rust way to represent "no parent," and the compiler *forces* you to handle the `None` case everywhere `parent` is read, eliminating an entire class of "forgot to check the sentinel" bugs that both the Go and C versions are vulnerable to.
- `for &neighbor in &adj_list[current]` borrows the vector immutably; the borrow checker guarantees `adj_list` cannot be mutated while BFS holds this reference, preventing the class of bugs where a graph is mutated mid-traversal.

### 3.9 Common Mistakes

| Mistake | Why it seems reasonable | Failure condition | Fix |
|---|---|---|---|
| Marking `visited` at **dequeue** time instead of enqueue time | "I'll just check when I actually process it" | A vertex reachable from 2+ frontier vertices gets enqueued multiple times; correctness usually survives but you do redundant work, and in weighted variants this breaks entirely | Mark visited the instant you enqueue |
| Using DFS (recursion/stack) expecting shortest-path distances | "I already wrote DFS, it visits everyone too" | DFS can reach a vertex via a long path before a short one; `distance` values become wrong | Use a FIFO queue, not a stack |
| Forgetting to reset `visited`/`distance` between multiple BFS calls (e.g., multi-source BFS reuse) | Arrays feel like they should just be reusable | Stale `true`/values from a previous run silently suppress correct re-visits | Reallocate or explicitly reset before each run |
| Assuming BFS distance = shortest path in a **weighted** graph | "It found *a* path, so it must be shortest" | BFS counts edges, not weights — a 5-edge path of weight 1 each looks "farther" than a true 1-edge path of weight 100, but BFS would rank the 5-edge path as "closer" | Use Dijkstra/Bellman-Ford for weighted shortest paths |

### 3.10 Active Recall Questions

1. Why must a vertex be marked visited at the moment it is *enqueued*, not when it is *dequeued*? Construct a small graph where getting this wrong causes a vertex to be processed twice.
2. Prove that when BFS finishes processing all vertices at distance `d`, the queue contains exactly the vertices at distance `d+1`.
3. What breaks if you replace the queue with a stack? Trace through the example graph in Section 3.2 using a stack instead and compare the visit order.
4. Given only the `parent` array BFS produces, how would you reconstruct the actual shortest path (not just its length) from the source to any vertex?
5. How would you modify BFS to run from **multiple sources simultaneously** (multi-source BFS), and what real-world problem does that solve? (Hint: think "nearest fire station to every building.")

### 3.11 Practice Problems

- **Beginner:** Given an unweighted graph and a source vertex, print the shortest distance (in edges) from the source to every other vertex.
- **Intermediate:** Given a 2D grid with walls, find the shortest path (in steps) from a start cell to an end cell, moving only up/down/left/right. (Model the grid as an implicit graph.)
- **Variation:** Multi-source BFS — given several starting cells simultaneously (e.g., multiple fires spreading on a grid), find the time at which every cell catches fire.


---

## 4. Depth-First Search (DFS)

### 4.1 Concept

DFS explores as **deep** as possible along one branch before backtracking. Where BFS is "wide first," DFS is "deep first" — it commits to a path and only retreats when it hits a dead end.

**Why it exists:** many problems care about *structure* (connectivity, cycles, ordering, paths existing at all) rather than *shortest distance*. DFS's stack-based (or recursive) nature naturally exposes structural properties — entry/exit times, back edges, ancestor relationships — that are exactly what cycle detection, topological sort, and SCC algorithms are built from.

### 4.2 Mental Model

DFS uses a **stack** (explicit, or implicit via recursion). The key structural idea is tracking, for each vertex, *when* it was first discovered and *when* the recursive call for it fully finished (all its descendants explored) — the **discovery time** and **finish time**.

```text
Graph:                     DFS from A (recursive, visiting neighbors in listed order):

      A
     / \                   Call stack over time:
    B   C
    |   |                  A -> B -> D  (D has no unvisited neighbors, backtrack)
    D   E                  backtrack to B (no more neighbors, backtrack)
                            backtrack to A -> C -> E (backtrack)
                            backtrack to C, backtrack to A. Done.

Visit order:  A, B, D, C, E
```

```text
Recursion / stack depth visualization:

depth 0: A                         [discover A]
depth 1:   B                       [discover B]
depth 2:     D                     [discover D]
depth 2:     D (finish, no more    [finish D]  <- backtrack
              neighbors)
depth 1:   B (finish)              [finish B]  <- backtrack
depth 0: A                         (still open, try next neighbor: C)
depth 1:   C                       [discover C]
depth 2:     E                     [discover E]
depth 2:     E (finish)            [finish E]  <- backtrack
depth 1:   C (finish)              [finish C]  <- backtrack
depth 0: A (finish)                [finish A]
```

### 4.3 Dry Run

| Step | Call Stack (top→bottom) | Action | Visited |
|---|---|---|---|
| 1 | [A] | discover A, visit neighbor B | {A} |
| 2 | [B, A] | discover B, visit neighbor D | {A, B} |
| 3 | [D, B, A] | discover D, no unvisited neighbors, finish D | {A, B, D} |
| 4 | [B, A] | no more unvisited neighbors of B, finish B | {A, B, D} |
| 5 | [A] | visit neighbor C | {A, B, D} |
| 6 | [C, A] | discover C, visit neighbor E | {A, B, D, C} |
| 7 | [E, C, A] | discover E, no unvisited neighbors, finish E | {A, B, D, C, E} |
| 8 | [C, A] | finish C | {A, B, D, C, E} |
| 9 | [A] | finish A | {A, B, D, C, E} |

### 4.4 Algorithm Derivation

1. **Problem:** visit every reachable vertex, but unlike BFS, we don't care about shortest distance — we care about exploring full paths and understanding structural relationships between vertices (ancestor/descendant, back edges).
2. **Key observation:** recursion is a natural fit — "explore this vertex fully" decomposes into "mark it visited, then recursively explore each unvisited neighbor fully."
3. **Invariant:** a vertex is marked visited when first discovered (same as BFS), but the *order* of exploration commits fully to one neighbor's entire subtree before trying the next neighbor — this is what recursion (or an explicit stack) gives you for free.
4. **Two flavors:** recursive (uses the language's call stack implicitly) or iterative (uses an explicit stack data structure). They visit vertices in a similar but not always identical order, and the iterative version avoids stack-overflow risk on very deep/large graphs.

### 4.5 Complexity Analysis

Identical asymptotic shape to BFS: every vertex is visited once (`O(V)`), every edge is examined once (`O(E)`) → **`O(V + E)` time**, **`O(V)` auxiliary space** (visited array + recursion/explicit stack depth, which in the worst case — a long chain — is `O(V)`).

**Recursion depth caveat:** recursive DFS on a graph with a very long path (e.g., a linked-list-shaped graph of 100,000 vertices) can overflow the call stack in languages with limited stack size (C, and to an extent Go/Rust's default thread stacks). This is a real, practical reason to prefer the iterative version for large or adversarial inputs.

### 4.6 Go Implementation

```go
package main

import "fmt"

// Recursive DFS
func DFS(adjList [][]int, start int) []int {
	visited := make([]bool, len(adjList))
	order := []int{}
	var visit func(v int)
	visit = func(v int) {
		visited[v] = true
		order = append(order, v)
		for _, neighbor := range adjList[v] {
			if !visited[neighbor] {
				visit(neighbor)
			}
		}
	}
	visit(start)
	return order
}

// Iterative DFS using an explicit stack — avoids call-stack overflow on deep graphs
func DFSIterative(adjList [][]int, start int) []int {
	visited := make([]bool, len(adjList))
	order := []int{}
	stack := []int{start}

	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1] // pop

		if visited[v] {
			continue
		}
		visited[v] = true
		order = append(order, v)

		// push in reverse so we visit neighbors in the same order as the recursive version
		for i := len(adjList[v]) - 1; i >= 0; i-- {
			neighbor := adjList[v][i]
			if !visited[neighbor] {
				stack = append(stack, neighbor)
			}
		}
	}
	return order
}

func main() {
	adj := [][]int{
		0: {1, 2}, // A -> B, C
		1: {3},    // B -> D
		2: {4},    // C -> E
		3: {},     // D
		4: {},     // E
	}
	fmt.Println("Recursive:", DFS(adj, 0))
	fmt.Println("Iterative:", DFSIterative(adj, 0))
}
```

**Key Go notes:**
- Note the iterative version marks `visited` **at pop time**, not push time — this is intentionally different from BFS. In DFS with an explicit stack, a vertex can legally be pushed multiple times before being popped once (by different ancestors), so the `if visited[v] { continue }` guard at pop time deduplicates. This is a subtly different (and easy to get wrong) discipline than BFS's "mark at enqueue" rule.
- The closure `visit` captures `visited`, `order`, and `adjList` by reference — idiomatic Go for recursive helpers that need shared mutable state without threading it through every parameter.

### 4.7 C Implementation

```c
#include <stdio.h>
#include <stdlib.h>

typedef struct AdjNode { int dest; struct AdjNode *next; } AdjNode;

void dfsRecursiveHelper(AdjNode **adjList, int v, int *visited, int *order, int *orderLen) {
    visited[v] = 1;
    order[(*orderLen)++] = v;
    AdjNode *cur = adjList[v];
    while (cur != NULL) {
        if (!visited[cur->dest]) {
            dfsRecursiveHelper(adjList, cur->dest, visited, order, orderLen);
        }
        cur = cur->next;
    }
}

void dfsRecursive(AdjNode **adjList, int numVertices, int start, int *order, int *orderLen) {
    int *visited = calloc(numVertices, sizeof(int));
    *orderLen = 0;
    dfsRecursiveHelper(adjList, start, visited, order, orderLen);
    free(visited);
}

// Iterative version with an explicit, manually managed stack (a plain array + top index)
void dfsIterative(AdjNode **adjList, int numVertices, int start, int *order, int *orderLen) {
    int *visited = calloc(numVertices, sizeof(int));
    int *stack = malloc(numVertices * numVertices * sizeof(int)); // generous upper bound
    int top = 0;
    *orderLen = 0;

    stack[top++] = start;
    while (top > 0) {
        int v = stack[--top]; // pop
        if (visited[v]) continue;
        visited[v] = 1;
        order[(*orderLen)++] = v;

        AdjNode *cur = adjList[v];
        // collect neighbors then push in reverse to mirror recursive order (optional detail)
        while (cur != NULL) {
            if (!visited[cur->dest]) stack[top++] = cur->dest;
            cur = cur->next;
        }
    }
    free(visited);
    free(stack);
}

int main(void) {
    // caller builds adjList as in Section 2, then calls dfsRecursive / dfsIterative
    return 0;
}
```

**Key C notes:**
- `dfsRecursiveHelper` uses the **C call stack directly** — each recursive call adds a stack frame. For a graph with a path of depth ~100,000+, this will overflow C's default thread stack (often ~1-8 MB) and crash with a segfault, *silently* in the sense that there's no exception to catch — this is why the iterative version exists as a real engineering necessity, not just an academic alternative.
- The iterative stack here is heap-`malloc`'d with a generous fixed bound for teaching simplicity; production C would use a dynamically growing stack (`realloc`-doubling, same pattern as Go's `append`) since a tight bound is easy to get wrong.

### 4.8 Rust Implementation

```rust
fn dfs_recursive_helper(adj_list: &Vec<Vec<usize>>, v: usize, visited: &mut Vec<bool>, order: &mut Vec<usize>) {
    visited[v] = true;
    order.push(v);
    for &neighbor in &adj_list[v] {
        if !visited[neighbor] {
            dfs_recursive_helper(adj_list, neighbor, visited, order);
        }
    }
}

fn dfs_recursive(adj_list: &Vec<Vec<usize>>, start: usize) -> Vec<usize> {
    let mut visited = vec![false; adj_list.len()];
    let mut order = Vec::new();
    dfs_recursive_helper(adj_list, start, &mut visited, &mut order);
    order
}

fn dfs_iterative(adj_list: &Vec<Vec<usize>>, start: usize) -> Vec<usize> {
    let mut visited = vec![false; adj_list.len()];
    let mut order = Vec::new();
    let mut stack = vec![start];

    while let Some(v) = stack.pop() {
        if visited[v] {
            continue;
        }
        visited[v] = true;
        order.push(v);

        for &neighbor in adj_list[v].iter().rev() {
            if !visited[neighbor] {
                stack.push(neighbor);
            }
        }
    }
    order
}

fn main() {
    let adj_list: Vec<Vec<usize>> = vec![
        vec![1, 2], // A -> B, C
        vec![3],    // B -> D
        vec![4],    // C -> E
        vec![],     // D
        vec![],     // E
    ];
    println!("Recursive: {:?}", dfs_recursive(&adj_list, 0));
    println!("Iterative: {:?}", dfs_iterative(&adj_list, 0));
}
```

**Key Rust notes:**
- `dfs_recursive_helper` takes `visited: &mut Vec<bool>` and `order: &mut Vec<usize>` as explicit mutable borrows threaded through every call — Rust has no implicit shared mutable closures the way Go's closures do; every function that mutates shared state must declare that borrow in its signature. This is more verbose but makes the mutation contract explicit and compiler-checked.
- Like C, deep recursion can overflow Rust's default stack (~8 MB on most platforms, configurable per-thread) — the iterative version using `Vec` as a stack (`push`/`pop`, both amortized `O(1)`) is the production-safe choice for large or adversarial graphs.

### 4.9 BFS vs DFS — When to Use Which

| Use Case | Correct Choice | Why |
|---|---|---|
| Shortest path, unweighted graph | BFS | Only BFS guarantees layer-order discovery |
| Detecting a cycle | DFS | Back-edge detection is natural with DFS's ancestor tracking |
| Topological sort | DFS (or Kahn's BFS-based algorithm) | Finish-time ordering directly yields topological order |
| Connected components | Either | Both visit exactly the reachable set; choice is about auxiliary structure needed |
| Maze / puzzle solving with path reconstruction | DFS (often with backtracking) | Naturally expresses "try this path fully, backtrack if it fails" |
| Finding *all* paths between two nodes | DFS with backtracking | BFS's queue doesn't naturally support path enumeration |
| Memory-constrained wide graphs | DFS | BFS's queue can hold `O(V)` vertices at the widest layer; DFS's stack only holds `O(depth)` |

### 4.10 Active Recall Questions

1. In the iterative DFS, why is `visited` checked and set at **pop** time rather than **push** time — and why would setting it at push time (as BFS does) produce a *different*, potentially incorrect, traversal order?
2. Construct a graph where recursive and iterative DFS (as coded above) visit vertices in a different order. What causes the difference?
3. Why can recursive DFS crash on a graph that iterative DFS handles fine, even though both are `O(V + E)`?
4. If you wanted DFS to also record each vertex's **finish time** (when its recursive call fully completes), where exactly would you insert that instrumentation in the recursive version?
5. Explain why DFS, not BFS, is the natural basis for cycle detection — what property of DFS's traversal directly reveals a "back edge" to an ancestor?


---

## 5. Connected Components

### 5.1 Concept

A **connected component** is a maximal set of vertices where every vertex can reach every other vertex (in an undirected graph). "Maximal" means you can't add any more vertices to the set without breaking that property.

**Why it exists:** many real questions reduce to "is the graph one piece, or several separate pieces?" — is a network fully wired, is a social graph one community or several isolated clusters, can data flow between two given nodes at all (ignoring path length entirely).

```text
Graph with TWO connected components:

   A --- B        E --- F
   |     |        |
   C --- D        G

Component 1: {A, B, C, D}
Component 2: {E, F, G}

No edge connects any vertex in Component 1 to any vertex in Component 2.
```

### 5.2 Mental Model & Derivation

The algorithm is almost embarrassingly simple once BFS/DFS exist: **run a full traversal from every not-yet-visited vertex; each traversal discovers exactly one component.**

1. Initialize a global `visited` array, all `false`.
2. For each vertex `v` from `0` to `V-1`: if `v` is not visited, run BFS or DFS from `v`. Every vertex touched by this traversal belongs to the same component; increment a component counter.
3. Repeat until every vertex has been visited by exactly one traversal.

**Correctness:** every vertex ends up visited by exactly the traversal that started in its own component, because a traversal from `v` reaches precisely the set of vertices reachable from `v` — which, in an undirected graph, is exactly `v`'s connected component (reachability is symmetric).

### 5.3 Complexity

Each vertex and edge is still examined exactly once across *all* the traversals combined (the outer loop just finds new starting points for unvisited vertices) → **`O(V + E)` time, `O(V)` space** — identical to a single BFS/DFS, because the work never overlaps between components.

### 5.4 Go Implementation

```go
package main

import "fmt"

func ConnectedComponents(adjList [][]int) [][]int {
	n := len(adjList)
	visited := make([]bool, n)
	var components [][]int

	for start := 0; start < n; start++ {
		if visited[start] {
			continue
		}
		// BFS from this unvisited vertex discovers one whole component
		component := []int{}
		queue := []int{start}
		visited[start] = true
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			component = append(component, v)
			for _, neighbor := range adjList[v] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}
		components = append(components, component)
	}
	return components
}

func main() {
	adj := [][]int{
		0: {1, 2}, 1: {0, 3}, 2: {0, 3}, 3: {1, 2}, // component {0,1,2,3}
		4: {5}, 5: {4, 6}, 6: {5}, // component {4,5,6}
	}
	fmt.Println(ConnectedComponents(adj))
}
```

### 5.5 Rust Implementation

```rust
use std::collections::VecDeque;

fn connected_components(adj_list: &Vec<Vec<usize>>) -> Vec<Vec<usize>> {
    let n = adj_list.len();
    let mut visited = vec![false; n];
    let mut components = Vec::new();

    for start in 0..n {
        if visited[start] {
            continue;
        }
        let mut component = Vec::new();
        let mut queue = VecDeque::new();
        queue.push_back(start);
        visited[start] = true;

        while let Some(v) = queue.pop_front() {
            component.push(v);
            for &neighbor in &adj_list[v] {
                if !visited[neighbor] {
                    visited[neighbor] = true;
                    queue.push_back(neighbor);
                }
            }
        }
        components.push(component);
    }
    components
}

fn main() {
    let adj_list: Vec<Vec<usize>> = vec![
        vec![1, 2], vec![0, 3], vec![0, 3], vec![1, 2], // component {0,1,2,3}
        vec![5], vec![4, 6], vec![5],                    // component {4,5,6}
    ];
    println!("{:?}", connected_components(&adj_list));
}
```

**C implementation note:** identical structure to the BFS code from Section 3 — wrap the BFS call in an outer `for` loop over all vertices, skipping any already marked `visited`, and collect each traversal's output into a new component array. (Omitted here since it is a mechanical composition of Sections 2 and 3's C code.)

### 5.6 A Faster Alternative for Static Queries: Union-Find

If you don't need to *enumerate* each component's members but only need to repeatedly answer "are `u` and `v` in the same component?" — especially while edges are being **added incrementally** — Union-Find (Section 10) answers each query in near-`O(1)` amortized time, which BFS/DFS-based component labeling cannot do without a full re-traversal.

### 5.7 Active Recall Questions

1. Why does the outer loop in `ConnectedComponents` need to check `visited[start]` before launching a new traversal — what would happen (correctness-wise and complexity-wise) if you removed that check?
2. Prove that in an undirected graph, "reachable from v" and "connected component containing v" are the same set.
3. Does this algorithm work unmodified on a **directed** graph? If not, what does the resulting partition actually represent instead? (This is a preview of Section 11 — weakly vs. strongly connected components.)


---

## 6. Cycle Detection

### 6.1 Concept

A **cycle** is a path that starts and ends at the same vertex, using at least one edge (and, conventionally, without immediately re-using the edge you just came from). Detecting cycles matters enormously: circular dependencies (`A` imports `B` imports `A`), deadlock detection (resource wait cycles), validating that a structure is truly a tree/DAG.

Directed and undirected graphs require **different algorithms** — this is a common source of bugs when people copy a cycle-detection routine from one context into the other.

### 6.2 Undirected Graphs: DFS with Parent Tracking

**Key insight:** in an undirected graph, every edge `(u, v)` is stored twice in the adjacency list — once as `u -> v` and once as `v -> u`. During DFS from `u`, the "neighbor" `v` you just came from will always appear in `u`'s own adjacency list as a "visited neighbor." That is *not* a cycle — it's just the edge you arrived on. A true cycle is when DFS finds a visited neighbor that is **not** the immediate parent.

```text
Undirected graph WITH a cycle:      Undirected graph WITHOUT a cycle (a tree):

   A --- B                              A --- B
   |     |                              |     |
   C --- D                              C     D

DFS from A: A -> B -> D -> C           DFS from A: A -> B, A -> C, B -> D
At C, neighbor A is visited AND        At every step, every visited neighbor
is NOT C's parent (D is C's parent)    IS the immediate parent -> no cycle
=> CYCLE DETECTED
```

### 6.3 Directed Graphs: DFS with Recursion-Stack Tracking

**Key insight:** in a directed graph, reaching an already-visited vertex is *not* automatically a cycle — it might just be a valid DAG "diamond" (two different paths converging on the same vertex, which is fine). A cycle exists **only** if you reach a vertex that is currently an ancestor of the current DFS call — i.e., it's still "on the stack" (its recursive call hasn't finished yet).

```text
Directed graph WITH a cycle:         Directed graph WITHOUT a cycle (valid DAG):

   A -> B                                A -> B
        |                                     |
        v                                     v
   D <- C                                D <- C
   (D -> A closes the cycle)             (no edge back to A)

   A -> B -> C -> D -> A                A -> B -> C -> D
   D->A points to A, which is
   still on the current recursion
   stack => CYCLE

Diamond pattern — NOT a cycle even though C is visited twice via different paths:

        A
       / \
      B   C
       \ /
        D
   A->B, A->C, B->D, C->D
   D is visited via B first, then "seen again" via C —
   but D is NOT an ancestor of C (D already finished), so no cycle.
```

This distinction — "is it visited" vs. "is it currently an ancestor (on the recursion stack)" — is exactly why directed cycle detection needs a **second boolean array**, `inRecursionStack`, on top of `visited`.

### 6.4 Complexity

Both variants are a straightforward DFS with `O(1)` extra work per vertex/edge → **`O(V + E)` time, `O(V)` space** (visited array, plus recursion-stack array for the directed case, plus recursion depth).

### 6.5 Go Implementation

```go
package main

import "fmt"

// ----- Undirected cycle detection -----
func HasCycleUndirected(adjList [][]int) bool {
	n := len(adjList)
	visited := make([]bool, n)

	var dfs func(v, parent int) bool
	dfs = func(v, parent int) bool {
		visited[v] = true
		for _, neighbor := range adjList[v] {
			if !visited[neighbor] {
				if dfs(neighbor, v) {
					return true
				}
			} else if neighbor != parent {
				return true // visited neighbor that isn't where we came from
			}
		}
		return false
	}

	for v := 0; v < n; v++ {
		if !visited[v] {
			if dfs(v, -1) {
				return true
			}
		}
	}
	return false
}

// ----- Directed cycle detection -----
func HasCycleDirected(adjList [][]int) bool {
	n := len(adjList)
	visited := make([]bool, n)
	inStack := make([]bool, n)

	var dfs func(v int) bool
	dfs = func(v int) bool {
		visited[v] = true
		inStack[v] = true
		for _, neighbor := range adjList[v] {
			if !visited[neighbor] {
				if dfs(neighbor) {
					return true
				}
			} else if inStack[neighbor] {
				return true // neighbor is a currently-active ancestor => back edge
			}
		}
		inStack[v] = false // done exploring v; remove it from the "active path"
		return false
	}

	for v := 0; v < n; v++ {
		if !visited[v] {
			if dfs(v) {
				return true
			}
		}
	}
	return false
}

func main() {
	undirectedWithCycle := [][]int{0: {1, 2}, 1: {0, 3}, 2: {0, 3}, 3: {1, 2}}
	fmt.Println("Undirected has cycle:", HasCycleUndirected(undirectedWithCycle))

	directedWithCycle := [][]int{0: {1}, 1: {2}, 2: {3}, 3: {0}}
	fmt.Println("Directed has cycle:", HasCycleDirected(directedWithCycle))
}
```

**Key Go note:** `inStack[v] = false` after the loop is the critical line that makes directed detection correct — it "pops" `v` from the logical recursion stack once all its descendants are fully explored, so a *later*, unrelated path that happens to revisit `v` (a diamond, not a cycle) is correctly not flagged.

### 6.6 Rust Implementation

```rust
fn has_cycle_undirected(adj_list: &Vec<Vec<usize>>) -> bool {
    let n = adj_list.len();
    let mut visited = vec![false; n];

    fn dfs(v: usize, parent: Option<usize>, adj_list: &Vec<Vec<usize>>, visited: &mut Vec<bool>) -> bool {
        visited[v] = true;
        for &neighbor in &adj_list[v] {
            if !visited[neighbor] {
                if dfs(neighbor, Some(v), adj_list, visited) {
                    return true;
                }
            } else if Some(neighbor) != parent {
                return true;
            }
        }
        false
    }

    for v in 0..n {
        if !visited[v] {
            if dfs(v, None, adj_list, &mut visited) {
                return true;
            }
        }
    }
    false
}

fn has_cycle_directed(adj_list: &Vec<Vec<usize>>) -> bool {
    let n = adj_list.len();
    let mut visited = vec![false; n];
    let mut in_stack = vec![false; n];

    fn dfs(v: usize, adj_list: &Vec<Vec<usize>>, visited: &mut Vec<bool>, in_stack: &mut Vec<bool>) -> bool {
        visited[v] = true;
        in_stack[v] = true;
        for &neighbor in &adj_list[v] {
            if !visited[neighbor] {
                if dfs(neighbor, adj_list, visited, in_stack) {
                    return true;
                }
            } else if in_stack[neighbor] {
                return true;
            }
        }
        in_stack[v] = false;
        false
    }

    for v in 0..n {
        if !visited[v] {
            if dfs(v, adj_list, &mut visited, &mut in_stack) {
                return true;
            }
        }
    }
    false
}

fn main() {
    let undirected: Vec<Vec<usize>> = vec![vec![1, 2], vec![0, 3], vec![0, 3], vec![1, 2]];
    println!("Undirected has cycle: {}", has_cycle_undirected(&undirected));

    let directed: Vec<Vec<usize>> = vec![vec![1], vec![2], vec![3], vec![0]];
    println!("Directed has cycle: {}", has_cycle_directed(&directed));
}
```

**Key Rust note:** `parent: Option<usize>` replaces the sentinel `-1` used in Go/C; the very first call passes `None`, and the comparison `Some(neighbor) != parent` correctly and safely handles "no parent yet" without any risk of an invalid sentinel value accidentally matching a real vertex ID (a real bug class in C/Go if you ever used `0` as a sentinel on a graph that has a vertex `0`).

**C implementation note:** structurally identical to the Go version — a recursive helper taking `(adjList, v, parent, visited)` for undirected, or `(adjList, v, visited, inStack)` for directed, both returning `int` (0/1) instead of `bool`. The critical `inStack[v] = 0;` line after the neighbor loop is exactly as essential as in Go.

### 6.7 Common Mistakes

| Mistake | Why it seems reasonable | Failure condition | Fix |
|---|---|---|---|
| Using the undirected algorithm (parent-check) on a directed graph | "It's the same idea, just following edges" | A DAG diamond (two paths converging, Section 6.3) gets falsely flagged as a cycle, because the converging vertex is "visited" but reached via a different, non-parent predecessor | Use the recursion-stack (`inStack`) technique for directed graphs |
| Forgetting to reset `inStack[v] = false` after finishing `v`'s exploration | Feels unnecessary — "v is visited, that's permanent" | `visited` should stay `true` forever (correct), but `inStack` must become `false` once `v`'s subtree is done — otherwise a later, unrelated path revisiting `v` is falsely flagged as a cycle | Always "pop" from `inStack` when a vertex's DFS call returns |
| Checking only `neighbor == parent` instead of tracking edge identity in a multigraph (multiple edges between the same two vertices) | Works for simple graphs | With two parallel edges between `u` and `v`, this incorrectly treats the second edge as "the same one we came from" | Track edge IDs, not just vertex parent, when parallel edges are possible |

### 6.8 Active Recall Questions

1. Walk through why the "diamond" graph in Section 6.3 is correctly identified as acyclic by the directed algorithm but would be incorrectly flagged by naively reusing the undirected algorithm's logic.
2. What is the precise difference between "visited" and "in the recursion stack"? Give a concrete vertex state where `visited[v] = true` and `inStack[v] = false` simultaneously, and explain what that state means.
3. How would you modify `HasCycleDirected` to return the actual cycle (as a list of vertices), not just a boolean?
4. Does Kahn's algorithm (Section 7) provide an alternative way to detect a cycle in a directed graph, without DFS at all? What is the tell-tale sign?


---

## 7. Topological Sorting

### 7.1 Concept

A **topological order** of a Directed Acyclic Graph (DAG) is a linear ordering of vertices such that for every directed edge `u -> v`, `u` appears **before** `v` in the ordering. It only exists for DAGs — a graph with a cycle has no valid topological order (a cycle demands `u` before `v` before ... before `u`, a contradiction).

**Why it exists:** this is the mathematical formalization of "dependency order" — course prerequisites, build-system task ordering, package installation order, spreadsheet cell recalculation order.

```text
Dependency graph:                Valid topological orders (multiple can exist!):

  compile -> link -> run           compile, link, run
     |                 ^
     v                 |           compile, test, link, run
   test  ---------------           (test can happen anywhere between compile and run,
                                    since nothing depends on its output here)
```

### 7.2 Algorithm 1: DFS-Based (Finish-Time Ordering)

**Key insight:** if you run DFS and record each vertex's **finish time** (the moment its recursive call fully returns — i.e., all its descendants are done), then **reversing** the finish-time order gives a valid topological order.

**Why this works:** when vertex `v` finishes, every vertex reachable from `v` has *already* finished (that's what "finishing" means — all descendants are done). So `v` finishes *before* any of its ancestors, meaning in finish-time order, descendants come before ancestors. Reversing that gives ancestors (dependencies) before descendants — exactly topological order.

```text
Graph: compile(0) -> link(1) -> run(2)
       compile(0) -> test(3)

DFS from 0: visit 0, visit 1, visit 2 (no neighbors, finish 2),
            finish 1, visit 3 (no neighbors, finish 3), finish 0

Finish order:        2, 1, 3, 0
Reverse (topo order): 0, 3, 1, 2   ->  compile, test, link, run   [valid]
```

### 7.3 Algorithm 2: Kahn's Algorithm (BFS-Based, In-Degree Driven)

**Key insight:** a vertex with **in-degree 0** (no incoming edges, i.e., no unfulfilled dependencies) can always safely be placed next in the order. After placing it, remove its outgoing edges — this may drop some neighbors' in-degree to 0, making them safe to place next.

```text
Graph: compile(0) -> link(1) -> run(2)
       compile(0) -> test(3)

In-degrees initially:  compile=0, link=1, test=1, run=1

Step 1: in-degree-0 queue = [compile]. Pop compile, output it.
        Remove compile's edges -> link becomes 0, test becomes 0.
        queue = [link, test]

Step 2: Pop link, output it. Remove link->run edge -> run becomes 0.
        queue = [test, run]

Step 3: Pop test, output it. No outgoing edges.
        queue = [run]

Step 4: Pop run, output it. Done.

Output order: compile, link, test, run   [valid topological order]
```

**Bonus — this IS cycle detection for free:** if the graph has a cycle, some vertices' in-degree never reaches 0 (they're stuck waiting on each other), so the algorithm terminates with fewer than `V` vertices output. That shortfall is a direct, correct signal of a cycle — no separate cycle-detection pass needed.

### 7.4 Dry Run (Kahn's Algorithm)

| Step | In-Degree-0 Queue | Popped | Output So Far | Updated In-Degrees |
|---|---|---|---|---|
| 1 | [compile] | — | [] | compile=0, link=1, test=1, run=1 |
| 2 | [] | compile | [compile] | link=0, test=0, run=1 |
| 3 | [link, test] | link | [compile, link] | run=0 |
| 4 | [test, run] | test | [compile, link, test] | (unchanged) |
| 5 | [run] | run | [compile, link, test, run] | (unchanged) |
| 6 | [] | — | [compile, link, test, run] | DONE — 4 vertices output = |V|, no cycle |

### 7.5 Complexity

Both algorithms: **`O(V + E)` time** — DFS-based is exactly one DFS pass plus a reversal (`O(V)`); Kahn's is exactly one pass computing in-degrees (`O(E)`) plus one BFS-like pass (`O(V + E)`). **Space: `O(V)`** for the queue/stack, in-degree array, and output list.

### 7.6 Go Implementation (Both Algorithms)

```go
package main

import "fmt"

// DFS-based topological sort
func TopoSortDFS(adjList [][]int) ([]int, bool) {
	n := len(adjList)
	visited := make([]bool, n)
	inStack := make([]bool, n)
	var finishOrder []int
	hasCycle := false

	var dfs func(v int)
	dfs = func(v int) {
		visited[v] = true
		inStack[v] = true
		for _, neighbor := range adjList[v] {
			if !visited[neighbor] {
				dfs(neighbor)
			} else if inStack[neighbor] {
				hasCycle = true
			}
		}
		inStack[v] = false
		finishOrder = append(finishOrder, v) // record finish time
	}

	for v := 0; v < n; v++ {
		if !visited[v] {
			dfs(v)
		}
	}
	if hasCycle {
		return nil, false
	}
	// reverse finishOrder
	for i, j := 0, len(finishOrder)-1; i < j; i, j = i+1, j-1 {
		finishOrder[i], finishOrder[j] = finishOrder[j], finishOrder[i]
	}
	return finishOrder, true
}

// Kahn's algorithm
func TopoSortKahn(adjList [][]int) ([]int, bool) {
	n := len(adjList)
	inDegree := make([]int, n)
	for _, neighbors := range adjList {
		for _, v := range neighbors {
			inDegree[v]++
		}
	}

	queue := []int{}
	for v := 0; v < n; v++ {
		if inDegree[v] == 0 {
			queue = append(queue, v)
		}
	}

	var order []int
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		order = append(order, v)
		for _, neighbor := range adjList[v] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != n {
		return nil, false // cycle detected: not all vertices could be ordered
	}
	return order, true
}

func main() {
	// compile(0) -> link(1) -> run(2); compile(0) -> test(3) -> run? no: test has no outgoing edge in this example
	adj := [][]int{0: {1, 3}, 1: {2}, 2: {}, 3: {}}
	order, ok := TopoSortDFS(adj)
	fmt.Println("DFS-based:", order, ok)
	order2, ok2 := TopoSortKahn(adj)
	fmt.Println("Kahn's:   ", order2, ok2)
}
```

### 7.7 Rust Implementation (Kahn's Algorithm)

```rust
use std::collections::VecDeque;

fn topo_sort_kahn(adj_list: &Vec<Vec<usize>>) -> Option<Vec<usize>> {
    let n = adj_list.len();
    let mut in_degree = vec![0usize; n];
    for neighbors in adj_list {
        for &v in neighbors {
            in_degree[v] += 1;
        }
    }

    let mut queue: VecDeque<usize> = (0..n).filter(|&v| in_degree[v] == 0).collect();
    let mut order = Vec::new();

    while let Some(v) = queue.pop_front() {
        order.push(v);
        for &neighbor in &adj_list[v] {
            in_degree[neighbor] -= 1;
            if in_degree[neighbor] == 0 {
                queue.push_back(neighbor);
            }
        }
    }

    if order.len() == n {
        Some(order)
    } else {
        None // cycle detected
    }
}

fn main() {
    let adj_list: Vec<Vec<usize>> = vec![vec![1, 3], vec![2], vec![], vec![]];
    match topo_sort_kahn(&adj_list) {
        Some(order) => println!("Topo order: {:?}", order),
        None => println!("Graph has a cycle — no topological order exists"),
    }
}
```

**Key Rust note:** the return type `Option<Vec<usize>>` makes "no valid ordering exists" a **type-level fact the caller cannot ignore** — Go's `(nil, false)` and C's typical `NULL`-return-with-out-parameter both rely on the caller remembering to check the boolean/null; Rust's `Option` forces a `match` or `.unwrap()`/`.expect()` at the call site, so silently proceeding with a `nil` order is a compile error, not a runtime bug waiting to happen.

**C implementation note:** Kahn's algorithm in C follows the identical structure to the Go version — compute `inDegree[]` with a single pass over all adjacency lists, seed a fixed-capacity array-backed queue with all in-degree-0 vertices, then repeatedly dequeue, output, and decrement neighbors' in-degrees, pushing any that hit zero. Compare `orderLen != numVertices` at the end to detect a cycle, exactly mirroring the Go and Rust logic.

### 7.8 Common Mistakes

| Mistake | Why it seems reasonable | Failure condition | Fix |
|---|---|---|---|
| Forgetting to reverse the DFS finish order | "DFS visits things in the right structural order already" | Output is in *reverse* topological order — dependents appear before their dependencies | Always reverse the finish-time list |
| Assuming a topological order is unique | Small examples often only have one valid order | Any DAG with "independent" branches (like `test` in Section 7.1) has multiple valid orderings; code that hardcodes an expected exact order will be needlessly brittle | Only assert the *relative* order constraint (edges point forward), not an exact sequence |
| Running topological sort on a graph without first confirming it's a DAG | "I'll just trust the input" | Silent wrong output (DFS-based) or a silently-truncated list (Kahn's) unless you explicitly check `len(order) != n` | Always check Kahn's output length, or track `hasCycle` in the DFS version |

### 7.9 Active Recall Questions

1. Prove that reversing DFS finish-time order always yields a valid topological order for a DAG. (Hint: consider what "finishing" a vertex means about its descendants.)
2. Why does Kahn's algorithm terminating with `order.len() < n` correctly and always indicate a cycle, with no false positives?
3. Given the "compile, test, link, run" example, list a second, different valid topological order and explain why both are equally correct.
4. How would you adapt Kahn's algorithm to detect **all** vertices that are stuck in (or depend on) a cycle, not just report that one exists?


---

## 8. Shortest Path Algorithms

### 8.1 The Landscape

There is no single "shortest path algorithm" — the right choice depends on whether weights exist, whether they can be negative, and whether you need one source or all pairs.

| Scenario | Algorithm | Time Complexity |
|---|---|---|
| Unweighted graph | BFS (Section 3) | O(V + E) |
| Weighted, non-negative weights, single source | Dijkstra | O((V + E) log V) with a binary heap |
| Weighted, negative weights allowed, single source | Bellman–Ford | O(V · E) |
| Weighted, all pairs | Floyd–Warshall | O(V³) |
| Weighted DAG, single source | DAG relaxation in topological order | O(V + E) |

### 8.2 Dijkstra's Algorithm

#### 8.2.1 Concept

Dijkstra finds the shortest path from a single source to every other vertex, in a graph where **all edge weights are non-negative**. It's a **greedy** algorithm: it repeatedly picks the unvisited vertex with the smallest known tentative distance, "finalizes" that distance (proves it can never improve), and uses it to relax (potentially improve) its neighbors' tentative distances.

**Why greedy works here (and only here):** once you finalize the vertex with the *globally smallest* tentative distance, no future relaxation could ever produce a shorter path to it — any such path would have to go through some other unfinalized vertex, whose distance is already `>=` the one you just finalized, and adding a non-negative edge weight on top can only make that path longer still. This proof **breaks** the instant a negative edge weight is allowed (Section 8.3 explains why).

#### 8.2.2 Mental Model & ASCII Diagram

```text
Graph (weighted, undirected):

        4
    A ----- B
    |       |
   1|       |2
    |       |
    C ----- D
        5

Dijkstra from A:

Initial: dist = {A:0, B:inf, C:inf, D:inf}

Step 1: finalize A (smallest tentative dist = 0)
        relax neighbors: B <- min(inf, 0+4)=4, C <- min(inf, 0+1)=1
        dist = {A:0, B:4, C:1, D:inf}

Step 2: finalize C (smallest unfinalized = 1)
        relax neighbors: D <- min(inf, 1+5)=6
        dist = {A:0, B:4, C:1, D:6}

Step 3: finalize B (smallest unfinalized = 4)
        relax neighbors: D <- min(6, 4+2)=6  (no improvement, 6==6)
        dist = {A:0, B:4, C:1, D:6}

Step 4: finalize D (smallest unfinalized = 6)
        no unvisited neighbors to relax
        DONE. Final: A=0, B=4, C=1, D=6
```

#### 8.2.3 Dry Run Table

| Step | Finalized Set | Priority Queue (dist, vertex) | Relaxed | dist[A,B,C,D] |
|---|---|---|---|---|
| 0 | {} | [(0,A)] | — | 0, ∞, ∞, ∞ |
| 1 | {A} | [(1,C),(4,B)] | B, C | 0, 4, 1, ∞ |
| 2 | {A,C} | [(4,B),(6,D)] | D | 0, 4, 1, 6 |
| 3 | {A,C,B} | [(6,D)] | D (no improvement) | 0, 4, 1, 6 |
| 4 | {A,C,B,D} | [] | — | 0, 4, 1, 6 |

#### 8.2.4 Algorithm Derivation

1. **Problem:** find the minimum-weight path from source `s` to every vertex, weights non-negative.
2. **Naive idea:** try every possible path — exponential, infeasible.
3. **Key observation:** the shortest path to *any* vertex must pass through vertices whose shortest paths are already known, in increasing order of distance from `s` — this suggests processing vertices in increasing distance order.
4. **Data structure need:** "repeatedly extract the minimum" is exactly what a **min-heap / priority queue** is for — this is why Dijkstra is always paired with one in an efficient implementation.
5. **Invariant:** once a vertex is popped from the priority queue (finalized), its `dist` value is the true shortest distance from `s` and will never change again.
6. **Correctness:** as argued in 8.2.1 — non-negative weights guarantee no future path can undercut an already-finalized minimum.

#### 8.2.5 Complexity Analysis

- Each vertex is pushed to the heap at most `O(degree(v))` times (once per relaxing edge) → total heap operations `O(E)`.
- Each heap push/pop is `O(log V)`.
- **Total: `O((V + E) log V)`** with a binary heap. (A Fibonacci heap achieves `O(E + V log V)`, rarely used in practice due to constant-factor overhead.)
- **Space:** `O(V)` for the distance array, `O(V)` for the heap (with possible duplicate entries — "lazy deletion" — which is fine as long as you skip stale entries on pop).

### 8.3 Why Dijkstra Fails With Negative Weights

```text
Graph with a negative edge:

     A --2--> B
     |        |
     5       -10
     |        |
     v        v
     C <------+

Dijkstra from A (INCORRECTLY):
  Finalize A (dist 0). Relax B (dist 2), C (dist 5).
  Finalize B next (smallest = 2) — but wait, B->C is -10,
  so the TRUE shortest path to C is A->B->C = 2 + (-10) = -8,
  far better than the "finalized" C = 5.

  Dijkstra already finalized C=5 believing nothing could beat it —
  but a negative edge from a LATER-processed vertex (B) proved that wrong.
  Dijkstra never revisits a finalized vertex, so it outputs the WRONG answer: 5, not -8.
```

This is exactly why negative weights require Bellman–Ford, which never "finalizes" anything early — it simply relaxes every edge repeatedly until nothing improves.

### 8.4 Bellman–Ford Algorithm

#### 8.4.1 Concept

Bellman–Ford computes shortest paths from a single source and correctly handles **negative edge weights** (though not negative *cycles* — a cycle whose total weight is negative has no well-defined shortest path, since you could loop it forever to decrease the "distance" without bound). It also **detects** negative cycles, which is often the actual point of running it.

**Key insight:** the shortest path between any two vertices in a graph with `V` vertices uses **at most `V - 1` edges** (a simple path can't repeat a vertex, so it has at most `V` vertices and `V - 1` edges). So: relax *every* edge, `V - 1` times, in any order. After `V - 1` rounds, every shortest path has necessarily been "found," because the relaxation of the `k`-th edge on the true shortest path can happen in round `k` regardless of processing order.

#### 8.4.2 Mental Model

```text
Graph:  A --1--> B --(-2)--> C --1--> D
                              ^________|
                                  3

dist = {A:0, B:inf, C:inf, D:inf}

Round 1 (relax every edge once, in listed order):
  A->B: dist[B] = min(inf, 0+1) = 1
  B->C: dist[C] = min(inf, 1-2) = -1
  C->D: dist[D] = min(inf, -1+1) = 0
  D->C: dist[C] = min(-1, 0+3) = -1 (no improvement)

Round 2: no edge can be relaxed further -> distances are final: A=0,B=1,C=-1,D=0

Round V (final check): if ANY edge can still be relaxed after V-1 rounds,
                        a negative cycle exists (distances would keep shrinking forever)
```

#### 8.4.3 Complexity

`V - 1` rounds, each relaxing every one of `E` edges → **`O(V · E)` time**, significantly slower than Dijkstra's `O((V+E) log V)`, which is the price paid for correctness under negative weights. **Space: `O(V)`.**

### 8.5 Floyd–Warshall Algorithm (All-Pairs Shortest Paths)

#### 8.5.1 Concept

Floyd–Warshall computes the shortest path between **every pair** of vertices simultaneously, using dynamic programming. It naturally operates on the **adjacency matrix** representation (Section 2.1) — this is the textbook example of an algorithm whose ideal representation is the matrix, not the list.

**Key insight (the DP state):** define `dist[k][i][j]` = shortest path from `i` to `j` using only intermediate vertices from `{0, 1, ..., k}`. The transition: either the shortest path from `i` to `j` doesn't use vertex `k` at all (`dist[k-1][i][j]`), or it does, splitting into `i -> k` and `k -> j` (`dist[k-1][i][k] + dist[k-1][k][j]`). Take the minimum of the two. Iterating `k` from `0` to `V-1` and collapsing the `k` dimension (each layer only needs the previous layer) gives the classic 3-nested-loop form.

```text
dist[i][j] = min( dist[i][j],  dist[i][k] + dist[k][j] )
             "don't route          "route through k"
              through k"

for k in 0..V:       # intermediate vertex allowed
  for i in 0..V:      # source
    for j in 0..V:    # destination
      dist[i][j] = min(dist[i][j], dist[i][k] + dist[k][j])
```

#### 8.5.2 Complexity

Three nested loops over `V` → **`O(V³)` time, `O(V²)` space** for the distance matrix. This is worse than running Dijkstra from every vertex (`O(V · (V+E) log V)`) for sparse graphs, but simpler to implement and better for dense graphs or when negative edges (but no negative cycles) are present.

### 8.6 Go Implementation

```go
package main

import (
	"container/heap"
	"fmt"
	"math"
)

type WeightedEdge struct{ To, Weight int }

// ----- Dijkstra -----
type pqItem struct {
	vertex, dist int
}
type PriorityQueue []pqItem

func (pq PriorityQueue) Len() int            { return len(pq) }
func (pq PriorityQueue) Less(i, j int) bool  { return pq[i].dist < pq[j].dist }
func (pq PriorityQueue) Swap(i, j int)       { pq[i], pq[j] = pq[j], pq[i] }
func (pq *PriorityQueue) Push(x interface{}) { *pq = append(*pq, x.(pqItem)) }
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}

func Dijkstra(adjList [][]WeightedEdge, source int) []int {
	n := len(adjList)
	dist := make([]int, n)
	for i := range dist {
		dist[i] = math.MaxInt32
	}
	dist[source] = 0

	pq := &PriorityQueue{{vertex: source, dist: 0}}
	heap.Init(pq)

	for pq.Len() > 0 {
		item := heap.Pop(pq).(pqItem)
		u := item.vertex
		if item.dist > dist[u] {
			continue // stale entry (lazy deletion) — a better path was already finalized
		}
		for _, edge := range adjList[u] {
			newDist := dist[u] + edge.Weight
			if newDist < dist[edge.To] {
				dist[edge.To] = newDist
				heap.Push(pq, pqItem{vertex: edge.To, dist: newDist})
			}
		}
	}
	return dist
}

// ----- Bellman-Ford -----
type Edge struct{ From, To, Weight int }

func BellmanFord(edges []Edge, numVertices, source int) (dist []int, hasNegativeCycle bool) {
	dist = make([]int, numVertices)
	for i := range dist {
		dist[i] = math.MaxInt32
	}
	dist[source] = 0

	for i := 0; i < numVertices-1; i++ {
		for _, e := range edges {
			if dist[e.From] != math.MaxInt32 && dist[e.From]+e.Weight < dist[e.To] {
				dist[e.To] = dist[e.From] + e.Weight
			}
		}
	}

	// One more round: if anything STILL improves, a negative cycle exists
	for _, e := range edges {
		if dist[e.From] != math.MaxInt32 && dist[e.From]+e.Weight < dist[e.To] {
			return dist, true
		}
	}
	return dist, false
}

// ----- Floyd-Warshall -----
func FloydWarshall(matrix [][]int) [][]int {
	n := len(matrix)
	dist := make([][]int, n)
	for i := range dist {
		dist[i] = make([]int, n)
		copy(dist[i], matrix[i])
	}

	for k := 0; k < n; k++ {
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if dist[i][k] != math.MaxInt32 && dist[k][j] != math.MaxInt32 {
					if dist[i][k]+dist[k][j] < dist[i][j] {
						dist[i][j] = dist[i][k] + dist[k][j]
					}
				}
			}
		}
	}
	return dist
}

func main() {
	adj := [][]WeightedEdge{
		0: {{1, 4}, {2, 1}},
		1: {{0, 4}, {3, 2}},
		2: {{0, 1}, {3, 5}},
		3: {{1, 2}, {2, 5}},
	}
	fmt.Println("Dijkstra:", Dijkstra(adj, 0))
}
```

**Key Go notes:**
- `container/heap` requires implementing the `heap.Interface` (`Len`, `Less`, `Swap`, `Push`, `Pop`) — Go has no built-in generic priority queue, unlike Rust's `BinaryHeap`. This is a real, recurring piece of boilerplate in Go graph code.
- The `if item.dist > dist[u] { continue }` line implements **lazy deletion**: rather than removing stale heap entries when a better distance is found, we simply skip them when popped. This is simpler than maintaining a "decrease-key" operation, which Go's heap doesn't support directly.
- Bellman–Ford operates on a flat **edge list** (Section 2.3), not an adjacency list — this is the natural representation since the algorithm just needs "all edges," independent of vertex structure.

### 8.7 Rust Implementation

```rust
use std::collections::BinaryHeap;
use std::cmp::Reverse;

#[derive(Clone, Copy)]
struct WeightedEdge { to: usize, weight: i64 }

fn dijkstra(adj_list: &Vec<Vec<WeightedEdge>>, source: usize) -> Vec<i64> {
    let n = adj_list.len();
    let mut dist = vec![i64::MAX; n];
    dist[source] = 0;

    // Reverse(...) turns Rust's max-heap BinaryHeap into a min-heap by distance
    let mut heap: BinaryHeap<Reverse<(i64, usize)>> = BinaryHeap::new();
    heap.push(Reverse((0, source)));

    while let Some(Reverse((d, u))) = heap.pop() {
        if d > dist[u] {
            continue; // stale entry, skip (lazy deletion)
        }
        for edge in &adj_list[u] {
            let new_dist = d + edge.weight;
            if new_dist < dist[edge.to] {
                dist[edge.to] = new_dist;
                heap.push(Reverse((new_dist, edge.to)));
            }
        }
    }
    dist
}

struct Edge { from: usize, to: usize, weight: i64 }

fn bellman_ford(edges: &[Edge], num_vertices: usize, source: usize) -> (Vec<i64>, bool) {
    let mut dist = vec![i64::MAX; num_vertices];
    dist[source] = 0;

    for _ in 0..num_vertices.saturating_sub(1) {
        for e in edges {
            if dist[e.from] != i64::MAX && dist[e.from] + e.weight < dist[e.to] {
                dist[e.to] = dist[e.from] + e.weight;
            }
        }
    }

    let mut has_negative_cycle = false;
    for e in edges {
        if dist[e.from] != i64::MAX && dist[e.from] + e.weight < dist[e.to] {
            has_negative_cycle = true;
            break;
        }
    }
    (dist, has_negative_cycle)
}

fn floyd_warshall(matrix: &Vec<Vec<i64>>) -> Vec<Vec<i64>> {
    let n = matrix.len();
    let mut dist = matrix.clone();

    for k in 0..n {
        for i in 0..n {
            for j in 0..n {
                if dist[i][k] != i64::MAX && dist[k][j] != i64::MAX {
                    let through_k = dist[i][k] + dist[k][j];
                    if through_k < dist[i][j] {
                        dist[i][j] = through_k;
                    }
                }
            }
        }
    }
    dist
}

fn main() {
    let adj_list: Vec<Vec<WeightedEdge>> = vec![
        vec![WeightedEdge { to: 1, weight: 4 }, WeightedEdge { to: 2, weight: 1 }],
        vec![WeightedEdge { to: 0, weight: 4 }, WeightedEdge { to: 3, weight: 2 }],
        vec![WeightedEdge { to: 0, weight: 1 }, WeightedEdge { to: 3, weight: 5 }],
        vec![WeightedEdge { to: 1, weight: 2 }, WeightedEdge { to: 2, weight: 5 }],
    ];
    println!("Dijkstra: {:?}", dijkstra(&adj_list, 0));
}
```

**Key Rust notes:**
- `BinaryHeap` is a **max-heap** by default; wrapping entries in `Reverse(...)` flips the comparison, turning it into a min-heap — a well-known, idiomatic Rust trick (and a common source of bugs for newcomers who forget the wrap and get the *largest* distance popped first instead of smallest).
- `i64::MAX` risks **overflow** if you ever add a weight to it without checking (`i64::MAX + 5` wraps or panics depending on build mode) — the explicit `if dist[e.from] != i64::MAX` guards in both Dijkstra and Bellman–Ford exist specifically to prevent "infinity plus a number" from silently becoming a small, wrong finite number. Go's `math.MaxInt32` (deliberately not `MaxInt64`, to leave headroom) plays the same defensive role.

**C implementation note:** Dijkstra in C requires hand-building a **binary min-heap** array (typically `int heap[]` storing `(dist, vertex)` pairs with manual `siftUp`/`siftDown` functions) since the C standard library has no heap/priority-queue type at all — this is meaningfully more code than Go or Rust, both of which get at least a heap-interface (Go) or a full heap type (Rust) from their standard libraries. Bellman–Ford and Floyd–Warshall in C are direct, mechanical translations of the Go loops shown above, using a flat `Edge edges[]` array and a 2D `int dist[V][V]` matrix respectively, with the same `INT_MAX` overflow-guard discipline.

### 8.8 Common Mistakes

| Mistake | Why it seems reasonable | Failure condition | Fix |
|---|---|---|---|
| Using Dijkstra on a graph with any negative edge | "It's just one negative edge, probably fine" | Silently wrong answers, as shown in Section 8.3 — no crash, no warning, just incorrect distances | Detect negative weights up front and switch to Bellman–Ford |
| Not implementing lazy deletion / stale-entry skipping in Dijkstra's heap | "I already relaxed it, why would it matter" | Without the `if d > dist[u]: continue` check, you process the same vertex multiple times with outdated distances, wasting work (correctness usually survives, but complexity degrades) | Always check staleness on pop |
| Running only `V - 1` rounds of Bellman-Ford and skipping the "one more round" negative-cycle check | "The problem states V-1 rounds suffice" | You get plausible-looking but silently wrong distances when a negative cycle exists — they're just not fully "converged" | Always run the extra validation round if negative cycles are possible in your input |
| Using Floyd–Warshall on a very large sparse graph "because it's simple" | "One algorithm, all pairs, easy" | `O(V³)` on `V = 10,000` is 10^12 operations — computationally infeasible, while `V` Dijkstra runs would be far cheaper on a sparse graph | Reserve Floyd–Warshall for small/dense graphs; use repeated Dijkstra for large sparse ones |

### 8.9 Active Recall Questions

1. Walk through the negative-edge counterexample in Section 8.3 by hand and identify the exact line in the Dijkstra pseudocode where the incorrect assumption is baked in.
2. Why does Bellman–Ford need exactly `V - 1` rounds (not more, not fewer) to guarantee correctness on a graph with no negative cycles?
3. In Floyd–Warshall, why must the loop order be `k` (outermost), then `i`, then `j` — what breaks if you reorder the loops so `i` is outermost?
4. Give a real-world scenario where negative edge weights are meaningful (Hint: think about arbitrage, or currency exchange).
5. When would you prefer running Dijkstra `V` times over Floyd–Warshall for an all-pairs problem, and when would you prefer the reverse?

### 8.10 Practice Problems

- **Beginner:** Given a weighted, non-negative graph, find the shortest distance from a source to a target vertex.
- **Intermediate:** Given a currency exchange graph where edge weights represent `-log(exchange rate)`, detect whether an arbitrage opportunity (a negative cycle) exists.
- **Variation:** Given a graph where you may skip at most `k` edges' weights (treat them as free), find the shortest path — how does this change the state you track during relaxation?


---

## 9. Minimum Spanning Trees

### 9.1 Concept

A **spanning tree** of a connected, undirected graph is a subgraph that connects all vertices using exactly `V - 1` edges and contains no cycle. A **minimum spanning tree (MST)** is the spanning tree whose total edge weight is smallest among all possible spanning trees.

**Why it exists:** the canonical use case is "connect everything as cheaply as possible" — wiring a network of buildings with minimum total cable length, connecting cities with minimum total road cost. There may be many spanning trees; MST finds the cheapest one.

```text
Graph:                          One possible MST (total weight 1+2+4=7):

    A --4-- B                       A       B
    |       |                       |       |
    1       2                       1       2
    |       |                       |       |
    C --5-- D                       C       D
     \      /
      3    6
       \  /
        E                    (edge A-B weight 4 and C-D weight 5 excluded —
                               a cheaper path already connects everything)
```

### 9.2 Kruskal's Algorithm

**Key insight:** sort all edges by weight, ascending. Greedily add each edge to the MST **unless** it would create a cycle (i.e., unless its two endpoints are already connected through previously-added edges). Stop once `V - 1` edges have been added.

**Why greedy works:** this is the **cut property** of MSTs — for any partition of vertices into two non-empty sets, the minimum-weight edge crossing that partition must be part of *some* MST. Processing edges in increasing weight order and skipping only those that would form a cycle is exactly the mechanism that respects this property at every step.

```text
Edges sorted by weight: (A,C,1), (B,D,2), (C,E,3), (A,B,4), (C,D,5), (D,E,6)

Add (A,C,1): A,C connected. MST edges: {A-C}
Add (B,D,2): B,D connected. MST edges: {A-C, B-D}
Add (C,E,3): C,E connected (E joins A,C's group). MST edges: {A-C, B-D, C-E}
Add (A,B,4): A is in {A,C,E}, B is in {B,D} — DIFFERENT groups, add it.
             MST edges: {A-C, B-D, C-E, A-B}  <- now 4 edges = V-1 for V=5. DONE.
Skip (C,D,5) and (D,E,6): would only be checked if we needed more edges.
```

This requires an efficient "are these two vertices already connected?" check — exactly what **Union-Find** (Section 10) provides.

### 9.3 Prim's Algorithm

**Key insight:** start from any vertex, and greedily grow a single connected tree by always adding the cheapest edge that connects a vertex *already in the tree* to a vertex *not yet in the tree*. This is structurally very similar to Dijkstra — both use a priority queue and grow a structure one vertex at a time — but Prim's relaxation compares **edge weight**, not **path distance from source**.

```text
Start at A. Tree = {A}

Step 1: cheapest edge leaving {A} is A-C (1). Add C. Tree = {A, C}
Step 2: cheapest edge leaving {A,C} is C-E (3) [A-B is 4, C-D is 5]. Add E. Tree = {A,C,E}
Step 3: cheapest edge leaving {A,C,E} is A-B (4). Add B. Tree = {A,C,E,B}
Step 4: cheapest edge leaving {A,C,E,B} is B-D (2). Add D. Tree = {A,C,E,B,D}
DONE — all 5 vertices connected, 4 edges used, same MST as Kruskal's (total weight 1+3+4+2=10 — note: different example numbers than 9.2's dry run, for illustration only)
```

### 9.4 Kruskal vs. Prim

| Aspect | Kruskal's | Prim's |
|---|---|---|
| Core data structure | Union-Find | Priority queue (min-heap) |
| Approach | Sort all edges globally, add greedily | Grow one tree, always add the cheapest crossing edge |
| Natural representation | Edge list | Adjacency list |
| Time complexity | O(E log E) (dominated by the sort) | O((V + E) log V) with a binary heap |
| Best for | Sparse graphs, or when edges are naturally given as a list | Dense graphs, or when you already have adjacency-list structure |

### 9.5 Complexity Analysis

- **Kruskal's:** sorting `E` edges is `O(E log E)`; Union-Find operations across all edges are nearly `O(E)` (amortized near-constant per operation, Section 10) → **total `O(E log E)`**.
- **Prim's:** identical shape to Dijkstra — each vertex/edge processed with heap operations → **`O((V + E) log V)`**.
- Both use **`O(V + E)` space** for the underlying graph structure, plus `O(V)` for Union-Find or the heap/visited tracking.

### 9.6 Go Implementation (Kruskal's, using Union-Find)

```go
package main

import (
	"fmt"
	"sort"
)

type Edge struct{ U, V, Weight int }

type DSU struct{ parent, rank []int }

func NewDSU(n int) *DSU {
	d := &DSU{parent: make([]int, n), rank: make([]int, n)}
	for i := range d.parent {
		d.parent[i] = i
	}
	return d
}
func (d *DSU) Find(x int) int {
	if d.parent[x] != x {
		d.parent[x] = d.Find(d.parent[x]) // path compression
	}
	return d.parent[x]
}
func (d *DSU) Union(x, y int) bool {
	rx, ry := d.Find(x), d.Find(y)
	if rx == ry {
		return false // already connected -> would form a cycle
	}
	if d.rank[rx] < d.rank[ry] {
		rx, ry = ry, rx
	}
	d.parent[ry] = rx
	if d.rank[rx] == d.rank[ry] {
		d.rank[rx]++
	}
	return true
}

func KruskalMST(edges []Edge, numVertices int) (mst []Edge, totalWeight int) {
	sort.Slice(edges, func(i, j int) bool { return edges[i].Weight < edges[j].Weight })
	dsu := NewDSU(numVertices)

	for _, e := range edges {
		if dsu.Union(e.U, e.V) { // returns true only if this edge connects two different components
			mst = append(mst, e)
			totalWeight += e.Weight
			if len(mst) == numVertices-1 {
				break
			}
		}
	}
	return mst, totalWeight
}

func main() {
	edges := []Edge{
		{0, 2, 1}, {1, 3, 2}, {2, 4, 3}, {0, 1, 4}, {2, 3, 5}, {3, 4, 6},
	}
	mst, total := KruskalMST(edges, 5)
	fmt.Println("MST edges:", mst)
	fmt.Println("Total weight:", total)
}
```

### 9.7 Rust Implementation (Prim's, using a Binary Heap)

```rust
use std::collections::BinaryHeap;
use std::cmp::Reverse;

struct WeightedEdge { to: usize, weight: i64 }

fn prim_mst(adj_list: &Vec<Vec<WeightedEdge>>, start: usize) -> (Vec<(usize, usize, i64)>, i64) {
    let n = adj_list.len();
    let mut in_mst = vec![false; n];
    let mut heap: BinaryHeap<Reverse<(i64, usize, usize)>> = BinaryHeap::new(); // (weight, from, to)
    let mut mst_edges = Vec::new();
    let mut total_weight = 0;

    in_mst[start] = true;
    for edge in &adj_list[start] {
        heap.push(Reverse((edge.weight, start, edge.to)));
    }

    while let Some(Reverse((weight, from, to))) = heap.pop() {
        if in_mst[to] {
            continue; // both endpoints already in the tree -> would form a cycle, skip
        }
        in_mst[to] = true;
        mst_edges.push((from, to, weight));
        total_weight += weight;

        for edge in &adj_list[to] {
            if !in_mst[edge.to] {
                heap.push(Reverse((edge.weight, to, edge.to)));
            }
        }
    }
    (mst_edges, total_weight)
}

fn main() {
    let adj_list: Vec<Vec<WeightedEdge>> = vec![
        vec![WeightedEdge{to:2,weight:1}, WeightedEdge{to:1,weight:4}],
        vec![WeightedEdge{to:0,weight:4}, WeightedEdge{to:3,weight:2}],
        vec![WeightedEdge{to:0,weight:1}, WeightedEdge{to:4,weight:3}],
        vec![WeightedEdge{to:1,weight:2}, WeightedEdge{to:4,weight:6}],
        vec![WeightedEdge{to:2,weight:3}, WeightedEdge{to:3,weight:6}],
    ];
    let (mst, total) = prim_mst(&adj_list, 0);
    println!("MST edges: {:?}", mst);
    println!("Total weight: {}", total);
}
```

**Key note on both:** Kruskal's `Union` returning `false` and Prim's `if in_mst[to] { continue }` are the same idea from two different angles — both are the mechanism that **rejects an edge that would form a cycle**, which is the one and only thing standing between "any spanning tree" and "a graph with extra useless edges."

**C implementation note:** Kruskal's in C combines the Union-Find implementation from Section 10 with `qsort()` (C's standard library sort, given a comparator function) over an `Edge[]` array, then the identical add-if-different-roots loop shown in Go. Prim's in C requires a hand-built binary min-heap (as discussed in Section 8.7) storing `(weight, from, to)` triples, with the same `if (inMST[to]) continue;` cycle-rejection check.


---

## 10. Union-Find / Disjoint Set Union

### 10.1 Concept

Union-Find (also called DSU — Disjoint Set Union) is a data structure that maintains a collection of **disjoint sets** and supports two operations extremely efficiently:

- **`Find(x)`** — which set does `x` belong to? (Returns a representative/"root" element of that set.)
- **`Union(x, y)`** — merge the sets containing `x` and `y` into one set.

**Why it exists:** any problem that reduces to "are these two things in the same group, and can I merge groups incrementally" is a Union-Find problem — Kruskal's MST (Section 9.2), incremental connected-component tracking (cheaper than re-running BFS every time an edge is added), detecting when adding an edge would create a cycle, and network connectivity queries.

### 10.2 Mental Model

Each set is represented as a **tree**, where every node points to its parent, and the root points to itself. `Find(x)` walks up parent pointers until it hits a root. `Union(x, y)` finds both roots and makes one the parent of the other.

```text
Initial state: every element is its own set (its own root)

  0   1   2   3   4
  (each is a tree of size 1, parent[i] = i)

After Union(0,1), Union(2,3), Union(1,2):

        0
        |
        1
        |
        2
       / \
      3   (2 is root of the merged {0,1,2,3} set)

  4 remains its own separate set.

Find(3) walks: 3 -> 2 (root). Returns 2.
Find(0) walks: 0 -> 1 -> 2 (root). Returns 2.
Since Find(3) == Find(0), 0 and 3 are in the same set.
```

### 10.3 The Two Optimizations That Make It Fast

**Without optimization**, naive Union-Find degrades to `O(n)` per operation in the worst case — repeatedly unioning in a bad order produces a long chain (a "linked list" tree), making `Find` walk the entire chain.

**Optimization 1 — Union by Rank (or Size):** when merging two trees, always attach the **shorter** tree under the **taller** tree's root (tracked via `rank`, an upper bound on tree height, or `size`, the element count). This keeps trees shallow — attaching short-under-tall can never increase the height of the taller tree.

**Optimization 2 — Path Compression:** every time `Find(x)` walks up to the root, make every node on that path point **directly** to the root, flattening the tree for all future queries.

```text
Before path compression, Find(3):        After Find(3) — every visited node
                                           now points directly to the root:
  root
   |                                        root
   a                                       / | \
   |                                      a  b  3
   b
   |
   3   <- Find(3) walks root->a->b->3      (a, b, and 3 all now point
                                             directly to root — future
                                             Find calls on any of them
                                             are O(1))
```

**Combined effect:** with both optimizations, the **amortized** time per operation is `O(α(n))`, where `α` is the *inverse Ackermann function* — a function that grows so slowly it is less than 5 for any `n` you could ever construct in practice (even `n` = the number of atoms in the observable universe). This is, for all practical purposes, **O(1) amortized**.

### 10.4 Dry Run

| Operation | parent array before | parent array after | Notes |
|---|---|---|---|
| Init(5) | — | [0,1,2,3,4] | Every element is its own root |
| Union(0,1) | [0,1,2,3,4] | [1,1,2,3,4] | 0's root (0) attaches under 1's root (1) |
| Union(2,3) | [1,1,2,3,4] | [1,1,3,3,4] | 2's root attaches under 3's root |
| Union(1,2) | [1,1,3,3,4] | [1,1,3,3,4]→root merge | Find(1)=1, Find(2)=3; attach 1 under 3 (or vice versa, by rank) |
| Find(0) | — | walks 0→1→3 (root), path-compresses 0 and 1 to point at 3 | |

### 10.5 Complexity

- **Without optimizations:** `O(n)` worst case per `Find`/`Union`.
- **With union-by-rank only:** `O(log n)` per operation.
- **With path compression only:** `O(log n)` amortized.
- **With both:** `O(α(n))` amortized — effectively constant.
- **Space:** `O(n)` for the parent (and rank/size) arrays.

### 10.6 Go Implementation

```go
package main

import "fmt"

type DSU struct {
	parent []int
	rank   []int
}

func NewDSU(n int) *DSU {
	d := &DSU{parent: make([]int, n), rank: make([]int, n)}
	for i := range d.parent {
		d.parent[i] = i // every element starts as its own root
	}
	return d
}

func (d *DSU) Find(x int) int {
	if d.parent[x] != x {
		d.parent[x] = d.Find(d.parent[x]) // path compression: point directly at the root
	}
	return d.parent[x]
}

func (d *DSU) Union(x, y int) bool {
	rootX, rootY := d.Find(x), d.Find(y)
	if rootX == rootY {
		return false // already in the same set
	}
	// union by rank: attach the shorter tree under the taller one
	if d.rank[rootX] < d.rank[rootY] {
		rootX, rootY = rootY, rootX
	}
	d.parent[rootY] = rootX
	if d.rank[rootX] == d.rank[rootY] {
		d.rank[rootX]++
	}
	return true
}

func (d *DSU) Connected(x, y int) bool {
	return d.Find(x) == d.Find(y)
}

func main() {
	dsu := NewDSU(5)
	dsu.Union(0, 1)
	dsu.Union(2, 3)
	dsu.Union(1, 2)
	fmt.Println("0 and 3 connected:", dsu.Connected(0, 3)) // true
	fmt.Println("0 and 4 connected:", dsu.Connected(0, 4)) // false
}
```

### 10.7 C Implementation

```c
#include <stdio.h>
#include <stdlib.h>

typedef struct { int *parent, *rank, n; } DSU;

DSU *dsuCreate(int n) {
    DSU *d = malloc(sizeof(DSU));
    d->parent = malloc(n * sizeof(int));
    d->rank = calloc(n, sizeof(int));
    d->n = n;
    for (int i = 0; i < n; i++) d->parent[i] = i;
    return d;
}

int dsuFind(DSU *d, int x) {
    if (d->parent[x] != x) {
        d->parent[x] = dsuFind(d, d->parent[x]); // path compression
    }
    return d->parent[x];
}

int dsuUnion(DSU *d, int x, int y) {
    int rootX = dsuFind(d, x);
    int rootY = dsuFind(d, y);
    if (rootX == rootY) return 0; // already connected

    if (d->rank[rootX] < d->rank[rootY]) {
        int tmp = rootX; rootX = rootY; rootY = tmp;
    }
    d->parent[rootY] = rootX;
    if (d->rank[rootX] == d->rank[rootY]) d->rank[rootX]++;
    return 1;
}

void dsuFree(DSU *d) {
    free(d->parent);
    free(d->rank);
    free(d);
}

int main(void) {
    DSU *d = dsuCreate(5);
    dsuUnion(d, 0, 1);
    dsuUnion(d, 2, 3);
    dsuUnion(d, 1, 2);
    printf("0 and 3 connected: %d\n", dsuFind(d, 0) == dsuFind(d, 3));
    printf("0 and 4 connected: %d\n", dsuFind(d, 0) == dsuFind(d, 4));
    dsuFree(d);
    return 0;
}
```

### 10.8 Rust Implementation

```rust
struct DSU {
    parent: Vec<usize>,
    rank: Vec<usize>,
}

impl DSU {
    fn new(n: usize) -> Self {
        DSU {
            parent: (0..n).collect(), // every element starts as its own root
            rank: vec![0; n],
        }
    }

    fn find(&mut self, x: usize) -> usize {
        if self.parent[x] != x {
            self.parent[x] = self.find(self.parent[x]); // path compression
        }
        self.parent[x]
    }

    fn union(&mut self, x: usize, y: usize) -> bool {
        let root_x = self.find(x);
        let root_y = self.find(y);
        if root_x == root_y {
            return false;
        }
        let (big, small) = if self.rank[root_x] < self.rank[root_y] {
            (root_y, root_x)
        } else {
            (root_x, root_y)
        };
        self.parent[small] = big;
        if self.rank[big] == self.rank[small] {
            self.rank[big] += 1;
        }
        true
    }

    fn connected(&mut self, x: usize, y: usize) -> bool {
        self.find(x) == self.find(y)
    }
}

fn main() {
    let mut dsu = DSU::new(5);
    dsu.union(0, 1);
    dsu.union(2, 3);
    dsu.union(1, 2);
    println!("0 and 3 connected: {}", dsu.connected(0, 3)); // true
    println!("0 and 4 connected: {}", dsu.connected(0, 4)); // false
}
```

**Key Rust note:** `find` and `union` both require `&mut self` because path compression **mutates** `parent` as a side effect of an operation that conceptually feels "read-only" (just asking "what's the root?"). This is a good example of a case where Rust's ownership rules force you to be explicit about something Go and C let you gloss over silently — `Find` is not really a pure query, it has a mutating side effect, and Rust's type signature makes that fact impossible to hide.

### 10.9 Common Mistakes

| Mistake | Why it seems reasonable | Failure condition | Fix |
|---|---|---|---|
| Implementing `Union`/`Find` without path compression or union-by-rank | "It's just following pointers, should be fast" | Degrades to `O(n)` per operation on adversarial input (e.g., always unioning in a chain: 0-1, 1-2, 2-3, ...) | Always implement at least one of the two optimizations; ideally both |
| Comparing `x == y` instead of `Find(x) == Find(y)` to check "same set" | "They're both in the structure, right?" | `x` and `y` being different elements doesn't mean anything about which set they're in — only their roots matter | Always compare through `Find` |
| Forgetting `Union` returns `false` (no-op) when already connected, and treating every call as "successful" | The naming "Union" sounds like it always merges something | In Kruskal's algorithm specifically, this breaks cycle detection — you'd add edges that create cycles because you didn't check the return value | Always check `Union`'s return value when it signals "was this a new connection" |

### 10.10 Active Recall Questions

1. Trace through 5 `Union` calls on a 6-element DSU **without** path compression, choosing unions that deliberately build the tallest possible tree. Then show how path compression on a single subsequent `Find` call flattens it.
2. Why does union-by-rank alone (without path compression) already guarantee `O(log n)` per operation? What tree shape does it prevent?
3. What does `Union(x, y)` returning `false` actually tell you, precisely? Why is this fact directly useful in Kruskal's MST algorithm?
4. Why is `rank` only an *upper bound* on tree height, not the exact height, once path compression is also in play?


---

## 11. Strongly Connected Components

### 11.1 Concept

In a **directed** graph, a **strongly connected component (SCC)** is a maximal set of vertices where every vertex can reach every other vertex **following edge directions**. This is the directed-graph analogue of connected components (Section 5), but strictly stronger: in an undirected graph, "reachable" is automatically symmetric; in a directed graph, `A` reaching `B` says nothing about `B` reaching `A` — SCC requires both directions to hold for every pair.

```text
Directed graph:

   A -> B -> C
   ^         |
   |         v
   +---------D -> E

SCC 1: {A, B, C, D}  (A->B->C->D->A is a cycle, so all four mutually reach each other)
SCC 2: {E}           (E is reachable from the cycle, but nothing is reachable FROM E
                       back into the cycle, so E is its own single-vertex SCC)
```

**Why it exists:** circular dependency detection in package managers/build systems (an SCC of size > 1 among modules means a genuine circular dependency), condensing a graph into a DAG of SCCs to simplify further analysis (this "condensation" is always acyclic by construction — see 11.5), and identifying tightly-coupled clusters in any directed relationship graph.

### 11.2 Kosaraju's Algorithm

**Key insight:** run DFS on the original graph to compute finish times (like topological sort's finish-time trick). Then run DFS again on the **transposed graph** (every edge reversed), processing vertices in **decreasing finish-time order**. Each DFS tree produced in this second pass is exactly one SCC.

**Why this works (intuition):** the vertex with the latest finish time in the first pass is, in some sense, "upstream" of everything it can reach. Running DFS from it on the *reversed* graph can only reach vertices that could originally reach it too — so anything found in that second-pass DFS tree must be mutually reachable with the start vertex, which is exactly the SCC definition. A full proof relies on properties of finish-time ordering relative to the DAG-of-SCCs structure, but this intuition is enough to correctly implement and reason about the algorithm.

```text
Original graph:  A -> B -> C -> A  (cycle),  C -> D,  D -> E

Step 1: DFS on original graph, record finish times.
        (one possible order) finish: E, D, C, B, A
        Decreasing finish-time order for step 2: A, B, C, D, E

Step 2: Build transposed graph (reverse every edge):
        B -> A, C -> B, A -> C  (cycle reversed, still a cycle)
        D -> C,  E -> D

Step 2: DFS on transposed graph, in order A, B, C, D, E:
        DFS from A: A -> C -> B -> A(visited) -> back to A, tree = {A, B, C}  <- one SCC
        DFS from D (next un-visited in the order): D -> C(visited, skip)... tree = {D}  <- one SCC
        DFS from E: tree = {E}  <- one SCC

Result: SCCs = {A,B,C}, {D}, {E}
```

### 11.3 Tarjan's Algorithm

**Key insight:** a **single DFS pass** (no transpose, no second traversal) using two per-vertex values: `disc[v]` (discovery time/order) and `low[v]` (the lowest discovery time reachable from `v`'s subtree, including via **one** back-edge to an ancestor still on the stack). If, after exploring all of `v`'s neighbors, `low[v] == disc[v]`, then `v` is the **root** of an SCC — pop everything off an explicit stack down to and including `v`; that popped group is one SCC.

```text
Graph:  A -> B -> C -> A (cycle),  C -> D

DFS from A:
  disc[A]=0, low[A]=0. Stack=[A]
  visit B: disc[B]=1, low[B]=1. Stack=[A,B]
    visit C: disc[C]=2, low[C]=2. Stack=[A,B,C]
      visit A: A already on stack! low[C] = min(low[C], disc[A]) = min(2,0) = 0
      visit D: disc[D]=3, low[D]=3. Stack=[A,B,C,D]
        D has no outgoing edges. low[D]==disc[D] (3==3) -> D is an SCC root.
        Pop D. SCC found: {D}
      back in C: low[C] = min(low[C], low[D]) — but D was already popped/finalized,
                 so this doesn't apply (D is not an ancestor still "in progress")
    back in C: low[C] = 0 (from the back-edge to A). low[C] != disc[C] (0 != 2) -> not a root
  back in B: low[B] = min(low[B], low[C]) = min(1, 0) = 0. low[B] != disc[B] -> not a root
back in A: low[A] = min(low[A], low[B]) = min(0,0) = 0. low[A] == disc[A] (0==0) -> A is a root!
  Pop stack down to and including A. Stack was [A,B,C]. SCC found: {C, B, A}

Result: SCCs = {A, B, C}, {D}   (same answer as Kosaraju's, found differently)
```

### 11.4 Kosaraju's vs. Tarjan's

| Aspect | Kosaraju's | Tarjan's |
|---|---|---|
| Number of DFS passes | 2 (original graph + transposed graph) | 1 |
| Needs graph transpose | Yes — extra `O(V+E)` construction | No |
| Core mechanism | Finish-time ordering, reused from topo sort | `disc`/`low` values + explicit stack |
| Conceptual simplicity | Easier to first understand and prove | More subtle, but more efficient in practice (single pass, no transpose) |
| Time complexity | O(V + E) | O(V + E) |

Both are asymptotically identical; **Tarjan's is generally preferred in production code** because it avoids building a second graph, while **Kosaraju's is often taught first** because its correctness argument builds directly and intuitively on the topological-sort finish-time idea you already have from Section 7.

### 11.5 The Condensation Graph

If you collapse every SCC into a single "super-vertex," the resulting graph (called the **condensation**) is **always a DAG** — this is a mathematical guarantee, not a special case. Why: if two super-vertices had edges both ways (a cycle between them), then by definition their constituent vertices would all mutually reach each other, meaning they were never separate SCCs to begin with — contradiction. This is precisely why SCC decomposition is a standard preprocessing step before running topological-sort-based analysis on graphs that aren't already acyclic.

### 11.6 Complexity

Both algorithms: **`O(V + E)` time** (Kosaraju's does 3 total graph passes — original DFS, transpose construction, transposed DFS — each `O(V+E)`, summing to still `O(V+E)`; Tarjan's does one pass with `O(1)` extra work per vertex/edge). **Space: `O(V)`** for `disc`/`low` arrays and the explicit stack (Tarjan's), or `O(V+E)` for the transposed adjacency list (Kosaraju's).

### 11.7 Go Implementation (Tarjan's)

```go
package main

import "fmt"

type Tarjan struct {
	adjList          [][]int
	disc, low        []int
	onStack          []bool
	stack            []int
	timer            int
	sccs             [][]int
}

func NewTarjan(adjList [][]int) *Tarjan {
	n := len(adjList)
	t := &Tarjan{adjList: adjList, disc: make([]int, n), low: make([]int, n), onStack: make([]bool, n)}
	for i := range t.disc {
		t.disc[i] = -1 // -1 = undiscovered
	}
	return t
}

func (t *Tarjan) dfs(v int) {
	t.disc[v] = t.timer
	t.low[v] = t.timer
	t.timer++
	t.stack = append(t.stack, v)
	t.onStack[v] = true

	for _, neighbor := range t.adjList[v] {
		if t.disc[neighbor] == -1 {
			t.dfs(neighbor)
			if t.low[neighbor] < t.low[v] {
				t.low[v] = t.low[neighbor]
			}
		} else if t.onStack[neighbor] {
			// back edge to an ancestor still on the stack
			if t.disc[neighbor] < t.low[v] {
				t.low[v] = t.disc[neighbor]
			}
		}
	}

	if t.low[v] == t.disc[v] {
		// v is the root of an SCC: pop everything down to and including v
		var scc []int
		for {
			w := t.stack[len(t.stack)-1]
			t.stack = t.stack[:len(t.stack)-1]
			t.onStack[w] = false
			scc = append(scc, w)
			if w == v {
				break
			}
		}
		t.sccs = append(t.sccs, scc)
	}
}

func (t *Tarjan) Run() [][]int {
	for v := range t.adjList {
		if t.disc[v] == -1 {
			t.dfs(v)
		}
	}
	return t.sccs
}

func main() {
	// A(0) -> B(1) -> C(2) -> A(0) cycle, C(2) -> D(3)
	adj := [][]int{0: {1}, 1: {2}, 2: {0, 3}, 3: {}}
	tarjan := NewTarjan(adj)
	fmt.Println("SCCs:", tarjan.Run())
}
```

### 11.8 Rust Implementation (Tarjan's)

```rust
struct Tarjan<'a> {
    adj_list: &'a Vec<Vec<usize>>,
    disc: Vec<i64>,
    low: Vec<i64>,
    on_stack: Vec<bool>,
    stack: Vec<usize>,
    timer: i64,
    sccs: Vec<Vec<usize>>,
}

impl<'a> Tarjan<'a> {
    fn new(adj_list: &'a Vec<Vec<usize>>) -> Self {
        let n = adj_list.len();
        Tarjan {
            adj_list,
            disc: vec![-1; n],
            low: vec![-1; n],
            on_stack: vec![false; n],
            stack: Vec::new(),
            timer: 0,
            sccs: Vec::new(),
        }
    }

    fn dfs(&mut self, v: usize) {
        self.disc[v] = self.timer;
        self.low[v] = self.timer;
        self.timer += 1;
        self.stack.push(v);
        self.on_stack[v] = true;

        for &neighbor in &self.adj_list[v] {
            if self.disc[neighbor] == -1 {
                self.dfs(neighbor);
                self.low[v] = self.low[v].min(self.low[neighbor]);
            } else if self.on_stack[neighbor] {
                self.low[v] = self.low[v].min(self.disc[neighbor]);
            }
        }

        if self.low[v] == self.disc[v] {
            let mut scc = Vec::new();
            loop {
                let w = self.stack.pop().unwrap();
                self.on_stack[w] = false;
                scc.push(w);
                if w == v {
                    break;
                }
            }
            self.sccs.push(scc);
        }
    }

    fn run(&mut self) -> Vec<Vec<usize>> {
        for v in 0..self.adj_list.len() {
            if self.disc[v] == -1 {
                self.dfs(v);
            }
        }
        self.sccs.clone()
    }
}

fn main() {
    let adj_list: Vec<Vec<usize>> = vec![vec![1], vec![2], vec![0, 3], vec![]];
    let mut tarjan = Tarjan::new(&adj_list);
    println!("SCCs: {:?}", tarjan.run());
}
```

**Key Rust note:** the `'a` lifetime parameter on `Tarjan<'a>` explicitly declares that this struct **borrows** `adj_list` rather than owning a copy — the compiler guarantees the `Tarjan` instance cannot outlive the graph it references, and that the graph cannot be mutated while `Tarjan` holds this borrow. Go and C have no equivalent compile-time guarantee; both simply trust the programmer not to mutate or free the graph out from under a long-lived analysis struct.

**C implementation note:** Tarjan's in C follows the identical `disc`/`low`/`onStack`/explicit-stack structure — the one C-specific wrinkle is that the explicit `stack` array (for SCC-popping, not the DFS recursion itself) needs the same "generous fixed bound or manual realloc-growth" treatment discussed for the iterative DFS stack in Section 4.7, since a vertex can be pushed onto it at most once (when first discovered) but the algorithm must still track a legitimate upper bound of `V` entries.

### 11.9 Common Mistakes

| Mistake | Why it seems reasonable | Failure condition | Fix |
|---|---|---|---|
| Using plain connected-components logic (Section 5) on a directed graph | "Reachability is reachability" | Produces **weakly** connected components (ignoring edge direction), which can be very different from true SCCs — e.g. a single long directed chain `A->B->C->D` is one weakly-connected component but four separate single-vertex SCCs | Use Tarjan's or Kosaraju's, which respect edge direction both ways |
| In Tarjan's, updating `low[v]` using `low[neighbor]` even when `neighbor` is on the stack but not a tree-child (i.e., via a back edge) | "It's still `low`, why not use it" | You must use `disc[neighbor]` (not `low[neighbor]`) for back edges — using `low[neighbor]` can incorrectly "leak" a lower value from outside `v`'s actual subtree, corrupting SCC boundaries | Tree edges: propagate `low[neighbor]`. Back edges: propagate `disc[neighbor]`, never `low[neighbor]` |
| Forgetting to check `onStack[neighbor]` and instead just checking `disc[neighbor] != -1` | "It's already visited, so update `low`" | A visited-but-already-finalized vertex (in a *previous*, fully-completed DFS tree) is not part of `v`'s current path and must NOT influence `v`'s `low` value | Only back-edges to a vertex still `onStack` count |

### 11.10 Active Recall Questions

1. Explain, in your own words, why the condensation graph (Section 11.5) is guaranteed to be acyclic. What would it mean, structurally, if it weren't?
2. Trace Tarjan's algorithm by hand on the graph `A -> B, B -> C, C -> A, B -> D, D -> E, E -> D` and identify all SCCs.
3. Why must back-edge relaxation use `disc[neighbor]`, not `low[neighbor]`? Construct a small example where using `low[neighbor]` instead would produce an incorrect SCC grouping.
4. How would you use SCC decomposition as a preprocessing step to run topological sort on a graph that isn't already a DAG?


---

## 12. Complexity Cheat Sheet

| Algorithm | Time | Space | Handles Negative Weights | Graph Type |
|---|---|---|---|---|
| BFS | O(V + E) | O(V) | N/A (unweighted) | Directed / Undirected |
| DFS | O(V + E) | O(V) | N/A (unweighted) | Directed / Undirected |
| Connected Components | O(V + E) | O(V) | N/A | Undirected |
| Cycle Detection (undirected) | O(V + E) | O(V) | N/A | Undirected |
| Cycle Detection (directed) | O(V + E) | O(V) | N/A | Directed |
| Topological Sort (DFS or Kahn's) | O(V + E) | O(V) | N/A | Directed Acyclic (DAG) |
| Dijkstra (binary heap) | O((V + E) log V) | O(V) | No | Weighted, non-negative |
| Bellman–Ford | O(V · E) | O(V) | Yes (detects negative cycles) | Weighted, directed |
| Floyd–Warshall | O(V³) | O(V²) | Yes (no negative cycles) | Weighted, all-pairs |
| Kruskal's MST | O(E log E) | O(V + E) | N/A | Weighted, undirected, connected |
| Prim's MST | O((V + E) log V) | O(V + E) | N/A | Weighted, undirected, connected |
| Union-Find (per op, amortized) | O(α(n)) ≈ O(1) | O(V) | N/A | N/A (auxiliary structure) |
| Kosaraju's SCC | O(V + E) | O(V + E) | N/A | Directed |
| Tarjan's SCC | O(V + E) | O(V) | N/A | Directed |

---

## 13. Decision Framework: Which Algorithm When

Walk through these questions, in order, whenever you face a new graph problem:

```text
1. Is the relationship symmetric (undirected) or one-way (directed)?
   -> Determines whether "connected components" or "SCC" applies,
      and whether cycle detection needs parent-tracking or stack-tracking.

2. Do edges have weights?
   NO  -> BFS gives shortest paths (in edge count) for free.
   YES -> continue to question 3.

3. Can weights be negative?
   NO  -> Dijkstra (single-source) or repeated-Dijkstra/Prim-style (all-pairs
          on a sparse graph).
   YES -> Bellman-Ford (single-source, also detects negative cycles) or
          Floyd-Warshall (all-pairs, dense graphs, no negative cycles).

4. Do you need ONE shortest path, or ALL-PAIRS shortest paths?
   ONE SOURCE  -> Dijkstra / Bellman-Ford / BFS depending on question 2-3.
   ALL PAIRS   -> Floyd-Warshall (dense/small) or V x Dijkstra (sparse/large).

5. Do you need to know if the graph has cycles, or produce a valid order
   respecting dependencies?
   CYCLE CHECK ONLY       -> DFS-based cycle detection (Section 6).
   VALID ORDER NEEDED     -> Topological sort (Section 7) -- which also
                              detects a cycle as a side effect.

6. Do you need the cheapest way to connect ALL vertices (not shortest
   path between two specific vertices)?
   YES -> Minimum Spanning Tree: Kruskal's (edge-list-native, sparse) or
          Prim's (adjacency-list-native, dense).

7. Do you need to repeatedly check/merge "are these two things connected",
   especially as edges are added incrementally over time?
   YES -> Union-Find, standalone or as Kruskal's underlying structure.

8. Is the graph directed, and do you need tightly-coupled mutual-reachability
   clusters (e.g., circular dependency detection)?
   YES -> Strongly Connected Components: Tarjan's (single-pass, preferred
          in production) or Kosaraju's (two-pass, often easier to first learn).
```

---

## Closing Notes on Building a Durable Mental Model

Every algorithm in this guide is a variation on exactly two primitive moves: **"explore in some order"** (BFS/DFS) and **"greedily commit to the locally-best choice, under conditions where that's provably globally optimal"** (Dijkstra, Prim's, Kruskal's). Bellman–Ford and Floyd–Warshall exist specifically because the greedy conditions break under negative weights, and they fall back to exhaustive relaxation instead. Topological sort and SCC decomposition are both, at their core, clever re-readings of what DFS's finish-time ordering is quietly telling you about a graph's structure.

If you can explain, without looking anything up:
- why Dijkstra's greedy step is correct and exactly where that correctness proof breaks with negative weights,
- why BFS discovers vertices in strict distance order but DFS does not,
- why path compression plus union-by-rank in Union-Find gets you to near-constant time,
- and why every condensation graph is provably acyclic,

— then you have internalized the actual mental models this guide was built to teach, not just memorized the code.

---

## 7. SCC, bridges, articulation points

### 7.1 Strongly connected components (SCC)

**Definition:** in a directed graph, `u` and `v` are in the same SCC if `u` reaches `v` **and** `v` reaches `u`. SCCs partition the vertices. Collapse each SCC to one node and you get the **condensation**, which is always a **DAG**.

```
   Original graph (6 vertices)               Condensation (a DAG)

        ┌────────┐                            
        ▼        │                            
       (0)──▶(1)─┘        (1→0 closes a cycle)     ┌───────────┐     ┌─────────┐     ┌─────┐
        │     │                                    │ {0,1,2}   │────▶│ {3,4}   │────▶│ {5} │
        ▼     │                                    └───────────┘     └─────────┘     └─────┘
       (2)◀───┘  2→0 also returns                   
        │                                          Each box = one SCC.
        ▼                                          Edges between boxes never form a cycle.
       (3)⇄(4)──▶(5)
```

**Why it matters:** dependency cycles (packages, modules), deadlock detection, 2-SAT (§12.3), web-graph structure, compilers (mutual recursion groups).

**Tarjan's algorithm — one DFS, linear time**

State per vertex: `idx[v]` (DFS discovery order), `low[v]` (smallest `idx` reachable from `v`'s DFS subtree using at most one back/cross edge to a vertex *still on the stack*), plus a stack of vertices "not yet assigned to an SCC".

Invariant: when a vertex `u` finishes with `low[u] == idx[u]`, nothing in its subtree can reach above `u`, so `u` is the **root** of an SCC — pop the stack down to `u`; those popped vertices are the component.

```
   DFS tree with idx / low
   
   (0) idx0 low0  ← root of SCC? low==idx → yes, pop until 0
    │
   (1) idx1 low0  ← back edge to 0 lowered low
    │
   (2) idx2 low0  ← edge 2→0 (0 is on stack) low = idx[0] = 0
   
   stack: [0,1,2]  → at finish of 0: pop 2,1,0 → SCC {0,1,2}
```

Tarjan emits components in **reverse topological order** of the condensation (sinks first) — handy for DP over the condensation.

**Kosaraju alternative:** (1) DFS on `G`, record finish order; (2) DFS on `Gᵀ` (transpose) in decreasing finish order — each tree is an SCC. Simpler to prove, two passes, needs the transpose.

#### Go
```go
type tarjan struct {
	g       *Graph
	idx     []int
	low     []int
	onStack []bool
	stack   []int
	counter int
	comp    []int
	ncomp   int
}

func SCC(g *Graph) (comp []int, count int) {
	t := &tarjan{
		g: g, idx: make([]int, g.N), low: make([]int, g.N),
		onStack: make([]bool, g.N), comp: make([]int, g.N),
	}
	for i := range t.idx {
		t.idx[i] = -1
	}
	for v := 0; v < g.N; v++ {
		if t.idx[v] == -1 {
			t.visit(v)
		}
	}
	return t.comp, t.ncomp
}

func (t *tarjan) visit(u int) {
	t.idx[u], t.low[u] = t.counter, t.counter
	t.counter++
	t.stack = append(t.stack, u)
	t.onStack[u] = true

	for _, e := range t.g.Adj[u] {
		v := e.To
		if t.idx[v] == -1 { // tree edge
			t.visit(v)
			t.low[u] = min(t.low[u], t.low[v])
		} else if t.onStack[v] { // edge into current SCC candidate
			t.low[u] = min(t.low[u], t.idx[v])
		}
		// else: v already assigned to a finished SCC → ignore
	}

	if t.low[u] == t.idx[u] { // u is an SCC root
		for {
			w := t.stack[len(t.stack)-1]
			t.stack = t.stack[:len(t.stack)-1]
			t.onStack[w] = false
			t.comp[w] = t.ncomp
			if w == u {
				break
			}
		}
		t.ncomp++
	}
}
```

#### C
```c
typedef struct {
    const Graph *g;
    int *idx, *low, *comp, *stack;
    char *on_stack;
    int counter, sp, ncomp;
} Tarjan;

static void tarjan_visit(Tarjan *t, int u) {
    t->idx[u] = t->low[u] = t->counter++;
    t->stack[t->sp++] = u;
    t->on_stack[u] = 1;

    for (int e = t->g->head[u]; e != -1; e = t->g->nxt[e]) {
        int v = t->g->to[e];
        if (t->idx[v] == -1) {
            tarjan_visit(t, v);
            if (t->low[v] < t->low[u]) t->low[u] = t->low[v];
        } else if (t->on_stack[v]) {
            if (t->idx[v] < t->low[u]) t->low[u] = t->idx[v];
        }
    }
    if (t->low[u] == t->idx[u]) {
        int w;
        do {
            w = t->stack[--t->sp];
            t->on_stack[w] = 0;
            t->comp[w] = t->ncomp;
        } while (w != u);
        t->ncomp++;
    }
}

/* comp[] receives component ids; returns number of components. */
int scc(const Graph *g, int *comp) {
    int n = g->n;
    Tarjan t = { .g = g, .comp = comp, .counter = 0, .sp = 0, .ncomp = 0 };
    t.idx      = malloc((size_t)n * sizeof(int));
    t.low      = malloc((size_t)n * sizeof(int));
    t.stack    = malloc((size_t)n * sizeof(int));
    t.on_stack = calloc((size_t)n, 1);
    for (int i = 0; i < n; i++) t.idx[i] = -1;
    for (int v = 0; v < n; v++) if (t.idx[v] == -1) tarjan_visit(&t, v);
    free(t.idx); free(t.low); free(t.stack); free(t.on_stack);
    return t.ncomp;
}
```

#### Rust
```rust
struct Tarjan<'a> {
    g: &'a Graph,
    idx: Vec<i32>,
    low: Vec<i32>,
    on_stack: Vec<bool>,
    stack: Vec<usize>,
    comp: Vec<usize>,
    counter: i32,
    ncomp: usize,
}

impl<'a> Tarjan<'a> {
    fn visit(&mut self, u: usize) {
        self.idx[u] = self.counter;
        self.low[u] = self.counter;
        self.counter += 1;
        self.stack.push(u);
        self.on_stack[u] = true;

        let g = self.g; // copy the shared reference so iterating doesn't borrow `self`
        for e in &g.adj[u] {
            let v = e.to;
            if self.idx[v] == -1 {
                self.visit(v);
                self.low[u] = self.low[u].min(self.low[v]);
            } else if self.on_stack[v] {
                self.low[u] = self.low[u].min(self.idx[v]);
            }
        }
        if self.low[u] == self.idx[u] {
            loop {
                let w = self.stack.pop().unwrap();
                self.on_stack[w] = false;
                self.comp[w] = self.ncomp;
                if w == u { break; }
            }
            self.ncomp += 1;
        }
    }
}

pub fn scc(g: &Graph) -> (Vec<usize>, usize) {
    let n = g.n;
    let mut t = Tarjan {
        g, idx: vec![-1; n], low: vec![0; n], on_stack: vec![false; n],
        stack: Vec::new(), comp: vec![0; n], counter: 0, ncomp: 0,
    };
    for v in 0..n {
        if t.idx[v] == -1 { t.visit(v); }
    }
    (t.comp, t.ncomp)
}
```

> Recursion depth = longest DFS path. For graphs with ≥10⁵–10⁶ vertices in a chain, run on a thread with a bigger stack (Rust: `std::thread::Builder::new().stack_size(...)`) or convert to iterative with an explicit `(u, edge_cursor)` stack as in §5.2.

### 7.2 Bridges and articulation points (cut vertices)

* **Bridge:** an undirected edge whose removal disconnects its component.
* **Articulation point:** a vertex whose removal disconnects its component.
* **Use:** network single-points-of-failure, road/graph robustness, biconnected components.

```
   (A)──(B)──(C)         Bridge: B–D          Articulation points: B and D
          │    │                              (removing B isolates A; removing D isolates E)
          │    │
         (D)───┘ ← makes B,C,D a cycle → B–C, C–D, D–B are NOT bridges
          │
         (E)                
```

**Core idea (DFS low-link):** `tin[u]` = discovery time. `low[u]` = earliest `tin` reachable from `u`'s subtree using tree edges downward and **at most one back edge**.

* Tree edge `u→v` is a **bridge** ⇔ `low[v] > tin[u]` (subtree of `v` cannot climb back to `u` or above).
* `u` (non-root) is an **articulation point** ⇔ some child `v` has `low[v] >= tin[u]`.
* Root is an articulation point ⇔ it has **≥ 2 DFS children**.

Subtle: *parallel edges.* If two edges connect `u` and `v`, neither is a bridge. Skip the tree edge to the parent **once** (Go/Rust below) or by edge id (`e ^ 1` in C).

#### Go
```go
func Bridges(g *Graph) (bridges [][2]int, cut []int) {
	tin := make([]int, g.N)
	low := make([]int, g.N)
	for i := range tin {
		tin[i] = -1
	}
	isCut := make([]bool, g.N)
	timer := 0

	var dfs func(u, p int)
	dfs = func(u, p int) {
		tin[u], low[u] = timer, timer
		timer++
		children, skipped := 0, false
		for _, e := range g.Adj[u] {
			v := e.To
			if v == p && !skipped { // skip parent edge exactly once
				skipped = true
				continue
			}
			if tin[v] != -1 { // back edge
				low[u] = min(low[u], tin[v])
				continue
			}
			dfs(v, u)
			children++
			low[u] = min(low[u], low[v])
			if low[v] > tin[u] {
				bridges = append(bridges, [2]int{u, v})
			}
			if p != -1 && low[v] >= tin[u] {
				isCut[u] = true
			}
		}
		if p == -1 && children > 1 {
			isCut[u] = true
		}
	}
	for v := 0; v < g.N; v++ {
		if tin[v] == -1 {
			dfs(v, -1)
		}
	}
	for v, c := range isCut {
		if c {
			cut = append(cut, v)
		}
	}
	return
}
```

#### C (uses the `e ^ 1` reverse-edge trick; build with `graph_add_undirected`)
```c
typedef struct {
    const Graph *g;
    int *tin, *low;
    char *is_cut;
    int (*bridges)[2];
    int nb, timer;
} BridgeCtx;

static void bridge_dfs(BridgeCtx *c, int u, int parent_edge) {
    const Graph *g = c->g;
    c->tin[u] = c->low[u] = c->timer++;
    int children = 0;
    for (int e = g->head[u]; e != -1; e = g->nxt[e]) {
        if ((e ^ 1) == parent_edge) continue;      /* don't walk back over the tree edge itself */
        int v = g->to[e];
        if (c->tin[v] != -1) {                     /* back edge */
            if (c->tin[v] < c->low[u]) c->low[u] = c->tin[v];
            continue;
        }
        bridge_dfs(c, v, e);
        children++;
        if (c->low[v] < c->low[u]) c->low[u] = c->low[v];
        if (c->low[v] > c->tin[u]) {
            c->bridges[c->nb][0] = u;
            c->bridges[c->nb][1] = v;
            c->nb++;
        }
        if (parent_edge != -1 && c->low[v] >= c->tin[u]) c->is_cut[u] = 1;
    }
    if (parent_edge == -1 && children > 1) c->is_cut[u] = 1;
}

/* out_bridges must hold up to n-1 pairs. Returns number of bridges. */
int find_bridges(const Graph *g, int (*out_bridges)[2], char *is_cut) {
    int n = g->n;
    BridgeCtx c = { .g = g, .bridges = out_bridges, .is_cut = is_cut };
    c.tin = malloc((size_t)n * sizeof(int));
    c.low = malloc((size_t)n * sizeof(int));
    for (int i = 0; i < n; i++) { c.tin[i] = -1; is_cut[i] = 0; }
    for (int v = 0; v < n; v++) if (c.tin[v] == -1) bridge_dfs(&c, v, -1);
    free(c.tin); free(c.low);
    return c.nb;
}
```

#### Rust
```rust
struct BridgeFinder<'a> {
    g: &'a Graph,
    tin: Vec<i32>,
    low: Vec<i32>,
    timer: i32,
    is_cut: Vec<bool>,
    bridges: Vec<(usize, usize)>,
}

impl<'a> BridgeFinder<'a> {
    fn dfs(&mut self, u: usize, p: Option<usize>) {
        self.tin[u] = self.timer;
        self.low[u] = self.timer;
        self.timer += 1;
        let (mut children, mut skipped) = (0, false);
        let g = self.g;
        for e in &g.adj[u] {
            let v = e.to;
            if Some(v) == p && !skipped {
                skipped = true;
                continue;
            }
            if self.tin[v] != -1 {
                self.low[u] = self.low[u].min(self.tin[v]);
                continue;
            }
            self.dfs(v, Some(u));
            children += 1;
            self.low[u] = self.low[u].min(self.low[v]);
            if self.low[v] > self.tin[u] {
                self.bridges.push((u, v));
            }
            if p.is_some() && self.low[v] >= self.tin[u] {
                self.is_cut[u] = true;
            }
        }
        if p.is_none() && children > 1 {
            self.is_cut[u] = true;
        }
    }
}

pub fn bridges_and_cuts(g: &Graph) -> (Vec<(usize, usize)>, Vec<usize>) {
    let mut f = BridgeFinder {
        g, tin: vec![-1; g.n], low: vec![0; g.n], timer: 0,
        is_cut: vec![false; g.n], bridges: Vec::new(),
    };
    for v in 0..g.n {
        if f.tin[v] == -1 { f.dfs(v, None); }
    }
    let cuts = (0..g.n).filter(|&v| f.is_cut[v]).collect();
    (f.bridges, cuts)
}
```

---

## 8. Shortest paths

### 8.0 Which algorithm? (memorize this table)

| Situation | Algorithm | Time |
|---|---|---|
| Unweighted | BFS | `O(V+E)` |
| Weights 0/1 | 0-1 BFS (deque) | `O(V+E)` |
| Non-negative weights, single source | **Dijkstra** (binary heap) | `O((V+E) log V)` |
| Negative weights allowed, single source | **Bellman-Ford** | `O(V·E)` |
| DAG (any weights) | topological-order relaxation | `O(V+E)` |
| All pairs, small/dense (`n` ≲ 500) | **Floyd-Warshall** | `O(V³)` |
| All pairs, sparse with negative edges | **Johnson** | `O(V·E log V)` |
| Single pair with good geometric guess | **A\*** | depends on heuristic |
| Huge static road network | Contraction Hierarchies / landmarks (ALT) | preprocessing + microsecond queries |

### 8.1 Dijkstra

**Idea:** grow a set of vertices whose distance is *final*. Always finalize the unfinalized vertex with the smallest tentative distance, then relax its outgoing edges.

**Why correct (the invariant):** when `u` has the smallest tentative distance among unfinalized vertices, any other route to `u` must pass through some unfinalized vertex with distance `≥ dist[u]`, and adding **non-negative** weights cannot make it shorter. **Negative edge ⇒ proof breaks ⇒ wrong answers.**

**Lazy-deletion heap:** instead of a decrease-key operation, push a new `(dist,v)` on every improvement and skip stale entries when popped (`d > dist[v]`). Heap size ≤ E.

```
   Running example, source 0.  Heap holds (dist,vertex).

   step   pop        relax                          dist[0..4]        heap after
   ────   ────────   ────────────────────────────   ───────────────   ──────────────────────────
   init                                             [0, ∞, ∞, ∞, ∞]   (0,0)
   1      (0,0)      0→1: 4      0→2: 1             [0, 4, 1, ∞, ∞]   (1,2)(4,1)
   2      (1,2)      2→1: 1+2=3<4 ✓   2→3: 6        [0, 3, 1, 6, ∞]   (3,1)(4,1)*(6,3)
   3      (3,1)      1→3: 3+1=4<6 ✓                 [0, 3, 1, 4, ∞]   (4,3)(4,1)*(6,3)*
   4      (4,3)      3→4: 4+3=7                     [0, 3, 1, 4, 7]   (4,1)*(6,3)*(7,4)
   5..7   stale entries (*) skipped: 4>dist[1]=3, 6>dist[3]=4      
   8      (7,4)      no out-edges                   done

   parent: 1←2, 2←0, 3←1, 4←3     shortest 0→4 = 0→2→1→3→4, cost 7
```

**Cost:** `O((V+E) log V)`. With a Fibonacci heap `O(E + V log V)` (rarely faster in practice). On dense graphs `O(V²)` array-scan version is better.

#### Go
```go
import "container/heap"

type item struct {
	node int
	dist int64
}
type minHeap []item

func (h minHeap) Len() int            { return len(h) }
func (h minHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h minHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x any)         { *h = append(*h, x.(item)) }
func (h *minHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func Dijkstra(g *Graph, src int) (dist []int64, parent []int) {
	dist = make([]int64, g.N)
	parent = make([]int, g.N)
	for i := range dist {
		dist[i], parent[i] = Inf, -1
	}
	dist[src] = 0
	h := &minHeap{{node: src, dist: 0}}
	for h.Len() > 0 {
		it := heap.Pop(h).(item)
		if it.dist > dist[it.node] { // stale entry
			continue
		}
		for _, e := range g.Adj[it.node] {
			if nd := it.dist + e.W; nd < dist[e.To] {
				dist[e.To] = nd
				parent[e.To] = it.node
				heap.Push(h, item{e.To, nd})
			}
		}
	}
	return
}
```

#### C (hand-written binary heap)
```c
typedef struct { int64_t d; int v; } HeapItem;
typedef struct { HeapItem *a; int size, cap; } MinHeap;

static void heap_push(MinHeap *h, int64_t d, int v) {
    if (h->size == h->cap) { h->cap *= 2; h->a = realloc(h->a, (size_t)h->cap * sizeof(HeapItem)); }
    int i = h->size++;
    h->a[i] = (HeapItem){ d, v };
    while (i > 0) {                       /* sift up */
        int p = (i - 1) / 2;
        if (h->a[p].d <= h->a[i].d) break;
        HeapItem t = h->a[p]; h->a[p] = h->a[i]; h->a[i] = t;
        i = p;
    }
}

static HeapItem heap_pop(MinHeap *h) {
    HeapItem top = h->a[0];
    h->a[0] = h->a[--h->size];
    int i = 0;
    for (;;) {                            /* sift down */
        int l = 2 * i + 1, r = l + 1, s = i;
        if (l < h->size && h->a[l].d < h->a[s].d) s = l;
        if (r < h->size && h->a[r].d < h->a[s].d) s = r;
        if (s == i) break;
        HeapItem t = h->a[s]; h->a[s] = h->a[i]; h->a[i] = t;
        i = s;
    }
    return top;
}

void dijkstra(const Graph *g, int src, int64_t *dist, int *parent) {
    for (int i = 0; i < g->n; i++) { dist[i] = INF; parent[i] = -1; }
    MinHeap h = { malloc(16 * sizeof(HeapItem)), 0, 16 };
    dist[src] = 0;
    heap_push(&h, 0, src);
    while (h.size > 0) {
        HeapItem it = heap_pop(&h);
        if (it.d > dist[it.v]) continue;          /* stale */
        for (int e = g->head[it.v]; e != -1; e = g->nxt[e]) {
            int v = g->to[e];
            int64_t nd = it.d + g->w[e];
            if (nd < dist[v]) {
                dist[v] = nd; parent[v] = it.v;
                heap_push(&h, nd, v);
            }
        }
    }
    free(h.a);
}
```

#### Rust
```rust
use std::cmp::Reverse;
use std::collections::BinaryHeap;

pub fn dijkstra(g: &Graph, src: usize) -> (Vec<i64>, Vec<Option<usize>>) {
    let mut dist = vec![INF; g.n];
    let mut parent = vec![None; g.n];
    let mut heap = BinaryHeap::new();          // max-heap, so wrap in Reverse for min-heap
    dist[src] = 0;
    heap.push(Reverse((0i64, src)));
    while let Some(Reverse((d, u))) = heap.pop() {
        if d > dist[u] { continue; }           // stale
        for e in &g.adj[u] {
            let nd = d + e.w;
            if nd < dist[e.to] {
                dist[e.to] = nd;
                parent[e.to] = Some(u);
                heap.push(Reverse((nd, e.to)));
            }
        }
    }
    (dist, parent)
}
```

### 8.2 Bellman-Ford (negative weights, negative-cycle detection)

**Idea:** relax **every edge**, `n-1` times. After round `k`, every shortest path using ≤ `k` edges is correct. Any shortest simple path has ≤ `n-1` edges, so `n-1` rounds suffice. If round `n` still improves something, a **negative cycle** reachable from the source exists (distances could decrease forever).

```
   negative cycle:        2
                 (A) ─────────▶ (B)
                  ▲               │
               -5 │               │ 1          A→B→C→A costs 2+1-5 = -2 < 0
                  └───── (C) ◀────┘            every lap lowers the total → no shortest path
```

* Early exit: stop if a full round changes nothing.
* Only vertices *reachable from a negative cycle* have undefined distance; propagate a `-∞` mark if you need per-vertex answers.
* `SPFA` = queue-based Bellman-Ford; fast on average, still `O(VE)` worst case.

#### Go
```go
func BellmanFord(n int, edges []WEdge, src int) (dist []int64, negCycle bool) {
	dist = make([]int64, n)
	for i := range dist {
		dist[i] = Inf
	}
	dist[src] = 0
	for round := 0; round < n-1; round++ {
		changed := false
		for _, e := range edges {
			if dist[e.U] != Inf && dist[e.U]+e.W < dist[e.V] {
				dist[e.V] = dist[e.U] + e.W
				changed = true
			}
		}
		if !changed {
			return dist, false
		}
	}
	for _, e := range edges { // n-th round: any improvement ⇒ negative cycle
		if dist[e.U] != Inf && dist[e.U]+e.W < dist[e.V] {
			return dist, true
		}
	}
	return dist, false
}
```

#### C
```c
typedef struct { int u, v; int64_t w; } WEdge;

/* returns 1 if a negative cycle is reachable from src */
int bellman_ford(int n, const WEdge *edges, int m, int src, int64_t *dist) {
    for (int i = 0; i < n; i++) dist[i] = INF;
    dist[src] = 0;
    for (int round = 0; round < n - 1; round++) {
        int changed = 0;
        for (int i = 0; i < m; i++) {
            const WEdge *e = &edges[i];
            if (dist[e->u] != INF && dist[e->u] + e->w < dist[e->v]) {
                dist[e->v] = dist[e->u] + e->w;
                changed = 1;
            }
        }
        if (!changed) return 0;
    }
    for (int i = 0; i < m; i++) {
        const WEdge *e = &edges[i];
        if (dist[e->u] != INF && dist[e->u] + e->w < dist[e->v]) return 1;
    }
    return 0;
}
```

#### Rust
```rust
pub fn bellman_ford(n: usize, edges: &[WEdge], src: usize) -> (Vec<i64>, bool) {
    let mut dist = vec![INF; n];
    dist[src] = 0;
    for _ in 0..n.saturating_sub(1) {
        let mut changed = false;
        for e in edges {
            if dist[e.u] != INF && dist[e.u] + e.w < dist[e.v] {
                dist[e.v] = dist[e.u] + e.w;
                changed = true;
            }
        }
        if !changed {
            return (dist, false);
        }
    }
    let neg = edges.iter().any(|e| dist[e.u] != INF && dist[e.u] + e.w < dist[e.v]);
    (dist, neg)
}
```

### 8.3 Floyd-Warshall (all pairs)

**DP idea:** `d_k[i][j]` = shortest `i→j` using only vertices `{0..k-1}` as intermediates. Either `k` isn't used (`d_{k-1}[i][j]`) or it is (`d_{k-1}[i][k] + d_{k-1}[k][j]`). Update in place, **outer loop must be `k`**.

```
   for k in 0..n:              i ──────────────▶ j
     for i in 0..n:             ╲              ▲
       for j in 0..n:            ╲──▶ (k) ───╱      is going through k shorter?
         d[i][j] = min(d[i][j], d[i][k] + d[k][j])
```

* Detect negative cycle: any `d[i][i] < 0` afterwards.
* Path reconstruction: store `next[i][j]` (first hop) and update it together with `d`.
* Also computes **transitive closure** with booleans (`reach[i][j] |= reach[i][k] && reach[k][j]`).

#### Go
```go
// d[i][j] = weight of edge i→j, Inf if none; d[i][i] = 0. Modified in place.
func FloydWarshall(d [][]int64) {
	n := len(d)
	for k := 0; k < n; k++ {
		for i := 0; i < n; i++ {
			if d[i][k] == Inf {
				continue
			}
			for j := 0; j < n; j++ {
				if d[k][j] == Inf {
					continue
				}
				if s := d[i][k] + d[k][j]; s < d[i][j] {
					d[i][j] = s
				}
			}
		}
	}
}
```

#### C
```c
/* d is a flat n*n array: d[i*n + j] */
void floyd_warshall(int n, int64_t *d) {
    for (int k = 0; k < n; k++)
        for (int i = 0; i < n; i++) {
            int64_t dik = d[(size_t)i * n + k];
            if (dik == INF) continue;
            for (int j = 0; j < n; j++) {
                int64_t dkj = d[(size_t)k * n + j];
                if (dkj == INF) continue;
                if (dik + dkj < d[(size_t)i * n + j]) d[(size_t)i * n + j] = dik + dkj;
            }
        }
}
```

#### Rust
```rust
pub fn floyd_warshall(d: &mut Vec<Vec<i64>>) {
    let n = d.len();
    for k in 0..n {
        for i in 0..n {
            if d[i][k] == INF { continue; }
            for j in 0..n {
                if d[k][j] == INF { continue; }
                let s = d[i][k] + d[k][j];
                if s < d[i][j] { d[i][j] = s; }
            }
        }
    }
}
```

### 8.4 Shortest paths on a DAG

Topologically sort, then relax outgoing edges of each vertex in that order. `O(V+E)`, negative weights allowed. Flip to `max` for **longest path / critical path** (project scheduling).

```go
func DagShortest(g *Graph, src int) []int64 {
	order, ok := TopoSort(g)
	if !ok {
		panic("graph has a cycle")
	}
	dist := make([]int64, g.N)
	for i := range dist {
		dist[i] = Inf
	}
	dist[src] = 0
	for _, u := range order {
		if dist[u] == Inf {
			continue
		}
		for _, e := range g.Adj[u] {
			dist[e.To] = min(dist[e.To], dist[u]+e.W)
		}
	}
	return dist
}
```

### 8.5 A\* search

Dijkstra + a **heuristic** `h(v)` estimating remaining cost to the goal. Pop the vertex with smallest `f(v) = g(v) + h(v)` where `g` = cost so far.

* **Admissible** (`h` never overestimates) ⇒ optimal path. **Consistent** (`h(u) ≤ w(u,v) + h(v)`) ⇒ each vertex is expanded once.
* Grid with 4-way moves: Manhattan distance. 8-way: Chebyshev/octile. Maps: straight-line distance / travel-speed.
* `h ≡ 0` ⇒ Dijkstra. Better `h` ⇒ fewer expansions.

```
   Dijkstra explores a disc          A* explores a corridor toward the goal
        . . . . . .                        . 
      . . . . . . . .                       . . 
     . . . S . . . . G                   S . . . . G
      . . . . . . . .                       . .
        . . . . . .                          .
```

```go
func AStar(g *Graph, src, dst int, h func(v int) int64) (int64, []int) {
	gcost := make([]int64, g.N)
	parent := make([]int, g.N)
	for i := range gcost {
		gcost[i], parent[i] = Inf, -1
	}
	gcost[src] = 0
	pq := &minHeap{{node: src, dist: h(src)}} // heap key is f = g + h
	for pq.Len() > 0 {
		it := heap.Pop(pq).(item)
		u := it.node
		if u == dst {
			return gcost[u], PathTo(parent, dst)
		}
		if it.dist > gcost[u]+h(u) { // stale
			continue
		}
		for _, e := range g.Adj[u] {
			if ng := gcost[u] + e.W; ng < gcost[e.To] {
				gcost[e.To] = ng
				parent[e.To] = u
				heap.Push(pq, item{e.To, ng + h(e.To)})
			}
		}
	}
	return Inf, nil
}
```

### 8.6 0-1 BFS

Weights are 0 or 1 ⇒ Dijkstra's heap is unnecessary. Use a deque: relax a 0-edge → **push front**, 1-edge → **push back**. `O(V+E)`.

```go
// Deque = "front stack" (push-front / pop-front at its end) + "back queue" (push-back / pop from head).
func ZeroOneBFS(g *Graph, src int) []int64 {
	dist := make([]int64, g.N)
	for i := range dist {
		dist[i] = Inf
	}
	dist[src] = 0
	var front []int
	back := []int{src}
	bh := 0
	for len(front) > 0 || bh < len(back) {
		var u int
		if len(front) > 0 {
			u = front[len(front)-1]
			front = front[:len(front)-1]
		} else {
			u = back[bh]
			bh++
		}
		for _, e := range g.Adj[u] {
			if nd := dist[u] + e.W; nd < dist[e.To] {
				dist[e.To] = nd
				if e.W == 0 {
					front = append(front, e.To) // push FRONT
				} else {
					back = append(back, e.To) // push BACK
				}
			}
		}
	}
	return dist
}
```
> A vertex can be enqueued more than once (when its distance improves); stale copies are harmless because relaxation re-checks `nd < dist`. In Rust use `VecDeque` (`push_front` / `push_back`).

### 8.7 Johnson's algorithm (all pairs, sparse, negative edges but no negative cycle)

1. Add a fake vertex `s` with 0-weight edges to every vertex.
2. Run Bellman-Ford from `s` → potentials `h[v]` (also detects negative cycles).
3. **Reweight:** `w'(u,v) = w(u,v) + h[u] − h[v] ≥ 0`.
4. Run Dijkstra from every vertex on `w'`.
5. Un-reweight: `dist(u,v) = dist'(u,v) − h[u] + h[v]`.

Why it works: any path `u ⇝ v` changes cost by the same constant `h[u] − h[v]`, so **the shortest path stays the same**, but all edges become non-negative. Total `O(V·E log V)`.