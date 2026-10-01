# The Complete Branch and Bound Guide — Theory, Architecture & Implementation

An in-depth, implementation-grounded reference for building a correct mental model of Branch and Bound (B&B). Every concept is paired with ASCII architecture diagrams and working code in **C**, **Go**, and **Rust**.

---

## Table of Contents

1. [What Branch and Bound Actually Is](#1-what-branch-and-bound-actually-is)
2. [The State Space Tree](#2-the-state-space-tree)
3. [Bounding Functions — The Heart of B&B](#3-bounding-functions--the-heart-of-bb)
4. [Branch and Bound vs Backtracking vs DP vs Greedy](#4-branch-and-bound-vs-backtracking-vs-dp-vs-greedy)
5. [The Three Search Strategies: FIFO, LIFO, LC](#5-the-three-search-strategies-fifo-lifo-lc)
6. [General Algorithm Architecture](#6-general-algorithm-architecture)
7. [Worked Example: 0/1 Knapsack — By Hand](#7-worked-example-01-knapsack--by-hand)
8. [0/1 Knapsack — C Implementation](#8-01-knapsack--c-implementation)
9. [0/1 Knapsack — Go Implementation](#9-01-knapsack--go-implementation)
10. [0/1 Knapsack — Rust Implementation](#10-01-knapsack--rust-implementation)
11. [Traveling Salesman Problem via B&B](#11-traveling-salesman-problem-via-bb)
12. [TSP — Rust Implementation (Reduced Matrix Method)](#12-tsp--rust-implementation-reduced-matrix-method)
13. [Job Assignment Problem](#13-job-assignment-problem)
14. [Complexity Analysis](#14-complexity-analysis)
15. [Bounding Function Design Patterns](#15-bounding-function-design-patterns)
16. [Parallel & Distributed Branch and Bound](#16-parallel--distributed-branch-and-bound)
17. [Real-World Applications](#17-real-world-applications)
18. [Common Pitfalls](#18-common-pitfalls)
19. [Mental Model Cheat Sheet](#19-mental-model-cheat-sheet)

---

## 1. What Branch and Bound Actually Is

Branch and Bound is an algorithm design paradigm for solving **combinatorial optimization problems** — problems where you're searching for the best solution (minimum cost or maximum value) among an enormous, often exponential, set of candidate solutions, and exhaustive search is computationally infeasible.

The name describes the two mechanical operations it alternates between:

- **Branch** — split a problem into smaller subproblems (this is identical to what backtracking does: pick a variable, try each of its possible values, recurse).
- **Bound** — before recursing into a subproblem, compute a *cheap, optimistic estimate* (a bound) of the best possible outcome achievable from that subproblem. If that estimate is already worse than the best solution you've found so far, **discard the entire subtree without exploring it**.

That second operation — pruning based on a bound — is what separates B&B from plain backtracking or brute force. Backtracking prunes only when a partial solution becomes *infeasible* (violates a constraint). Branch and Bound additionally prunes when a partial solution is *feasible but provably not optimal*, even though it hasn't been fully explored yet.

```
BRUTE FORCE:          Explore every leaf of the search tree.  O(2^n) or O(n!) always.

BACKTRACKING:         Explore every node, but stop descending a branch
                      the instant a CONSTRAINT is violated (infeasibility).

BRANCH AND BOUND:     Explore every node, but stop descending a branch
                      the instant its BOUND proves it cannot beat the
                      best solution found so far (sub-optimality) —
                      even if the branch is still perfectly feasible.
```

This is why B&B is the workhorse behind real integer/mixed-integer linear programming solvers (CPLEX, Gurobi, SCIP, OR-Tools) — problems like scheduling, resource allocation, VLSI placement, and route optimization are all, at core, combinatorial optimization problems with astronomically large search spaces that only become tractable through aggressive, bound-driven pruning.

---

## 2. The State Space Tree

Every B&B algorithm implicitly (or explicitly) builds a **state space tree**: the root represents "no decisions made yet," each edge represents a decision (include/exclude an item, assign a job to a worker, visit a city), and each node represents a partial solution — a state.

```
                              ┌─────────────┐
                              │  ROOT        │
                              │  (no items   │
                              │   decided)   │
                              │  bound = 100 │
                              └──────┬───────┘
                    include item1 ──┤├── exclude item1
                        ┌────────────┘└────────────┐
                        ▼                          ▼
                ┌───────────────┐          ┌───────────────┐
                │ Node A         │          │ Node B         │
                │ bound = 95     │          │ bound = 80     │
                └───────┬────────┘          └───────┬────────┘
           include item2┤├exclude item2  include item2┤├exclude item2
              ┌──────────┘└──────────┐        ┌────────┘└─────────┐
              ▼                      ▼         ▼                  ▼
      ┌───────────────┐     ┌───────────────┐ ┌──────────┐ ┌──────────────┐
      │ Node C         │     │ Node D         │ │ Node E    │ │ Node F        │
      │ bound = 92     │     │ bound = 60     │ │ bound=78  │ │ PRUNED         │
      │ (still explore)│     │ ✂ PRUNE:       │ │ ✂ PRUNE:  │ │ bound=45 < 92  │
      │                │     │ 60 < best(92)  │ │78 < best  │ │ (best found)   │
      └───────────────┘     └───────────────┘ └──────────┘ └──────────────┘
```

Key architectural facts about this tree:

- **The tree is never materialized in full.** It is generated lazily, node by node, as the algorithm decides which branch to expand next. This is precisely what makes B&B tractable — the *implicit* tree may have 2^n nodes, but the *explored* tree can be a tiny fraction of that.
- Each node carries: the **partial solution** (decisions made so far), the **bound** (best achievable value from this point on, assuming best-case completion), and enough state to generate children (branch).
- A node is pruned (its subtree is never explored) under any of three conditions:
  1. **Bound test** — the node's bound is worse than the current best known solution (the core B&B pruning rule).
  2. **Feasibility test** — the partial solution already violates a hard constraint.
  3. **Solution test** — the node is already a complete solution; it becomes a candidate for "best found so far" instead of being branched further.

---

## 3. Bounding Functions — The Heart of B&B

The entire efficiency of a B&B algorithm rests on the quality of its **bounding function**. This is the one piece of the algorithm that is problem-specific and requires genuine design work — everything else (the tree traversal, the pruning logic) is generic scaffolding.

A bounding function must be:

- **Optimistic (a valid bound)** — for a *maximization* problem, it must never *underestimate* what's achievable from this node (otherwise you might wrongly prune the branch containing the true optimum). For *minimization*, it must never *overestimate*.
- **Cheap to compute** — if computing the bound is as expensive as solving the subproblem outright, you've gained nothing. Bounds are typically computed via **relaxation**: solve an easier version of the problem (e.g., allow fractional item selection instead of binary, or drop a constraint) and use that easier solution's value as the bound.
- **As tight as possible** — a loose bound (e.g., "the bound is always +infinity") prunes nothing and degrades B&B into brute force. A tight bound (equal to the true optimal completion) prunes maximally, in the limit making the algorithm effectively polynomial for that instance.

```
BOUND QUALITY SPECTRUM (maximization problem):

  Loose bound                                          Tight bound
  ────────────────────────────────────────────────────────────────▶
  bound = +∞              bound = sum of all           bound = LP-relaxation
  (never prunes,          remaining item values        optimum (very close to
   = brute force)         (weak, ignores capacity)       true integer optimum)

  Prunes: nothing         Prunes: some branches         Prunes: most branches
  Speed: O(2^n)           Speed: better, still slow     Speed: often near-linear
                                                          in practice
```

**Common bounding techniques across problem types:**

| Technique | Idea | Example use |
|---|---|---|
| **LP relaxation** | Drop integrality constraints, solve as continuous linear program | 0/1 Knapsack, integer programming |
| **Fractional greedy bound** | Fill remaining capacity greedily, allowing fractional last item | Knapsack |
| **Minimum spanning tree bound** | MST cost as a lower bound for tour cost | TSP |
| **Reduced cost matrix** | Row/column reduction lower-bounds assignment cost | TSP, Job Assignment |
| **Constraint relaxation** | Temporarily ignore one hard constraint | Scheduling, bin packing |

---

## 4. Branch and Bound vs Backtracking vs DP vs Greedy

This comparison is where most conceptual confusion lives — all four paradigms attack overlapping problem classes.

```
┌────────────────┬───────────────────┬───────────────────┬────────────────────┐
│                │ Explores           │ Prunes on          │ Guarantees optimal │
├────────────────┼───────────────────┼───────────────────┼────────────────────┤
│ Greedy         │ One path only     │ Never backtracks   │ NO (unless problem │
│                │ (locally optimal  │                    │ has matroid/greedy │
│                │ choice each step) │                    │ -choice property)  │
├────────────────┼───────────────────┼───────────────────┼────────────────────┤
│ Dynamic        │ All distinct       │ Merges identical   │ YES — but requires │
│ Programming    │ subproblems, once │ overlapping         │ optimal substructure│
│                │ each (memoized)   │ subproblems         │ + overlapping subp. │
├────────────────┼───────────────────┼───────────────────┼────────────────────┤
│ Backtracking   │ Feasible partial  │ Constraint          │ YES (exhaustive     │
│                │ solutions         │ violation only      │ over feasible set) │
├────────────────┼───────────────────┼───────────────────┼────────────────────┤
│ Branch and     │ Feasible AND      │ Constraint violation│ YES — exact,        │
│ Bound          │ potentially       │ + bound proves      │ but with far fewer  │
│                │ optimal partial   │ sub-optimality       │ nodes explored than │
│                │ solutions         │                      │ backtracking alone  │
└────────────────┴───────────────────┴───────────────────┴────────────────────┘
```

- **Use DP** when the problem has optimal substructure *and* overlapping subproblems that can be indexed compactly (e.g., knapsack DP table indexed by `(item, remaining capacity)`) — DP is often faster than B&B when it applies, but its memory/table size can explode (e.g., knapsack DP is pseudo-polynomial, blowing up with large weight ranges).
- **Use B&B** when the state space is too large or too sparse for a DP table (e.g., TSP has no compact overlapping-subproblem structure at scale; permutation-based problems in general), but you can still design a good bound.
- **Use Backtracking** when you need *all* feasible solutions, or the problem has no meaningful notion of "better" (pure constraint satisfaction, e.g., Sudoku, N-Queens counting).
- **Use Greedy** only when you can prove the greedy-choice property holds — otherwise it gives a fast but potentially wrong answer, useful as a bound/heuristic *inside* a B&B algorithm rather than as a standalone solver.

---

## 5. The Three Search Strategies: FIFO, LIFO, LC

How you choose *which* live node to expand next defines the flavor of B&B:

```
FIFO (Breadth-First) B&B              LIFO (Depth-First) B&B
────────────────────────              ───────────────────────
Uses a QUEUE.                         Uses a STACK (or recursion).
Explores level by level.              Explores one path to the bottom
Finds a feasible complete solution    before backtracking.
late — but explores broadly early.    Finds SOME complete solution fast
Memory: can blow up (holds whole      (giving an early "best found so
level's worth of nodes).              far" to prune against) — but risks
                                       wasting time on a bad first path.
                                       Memory: O(depth) — usually much
                                       better than FIFO.

LC (Least Cost / Best-First) B&B
─────────────────────────────────
Uses a PRIORITY QUEUE keyed by each node's bound.
Always expands the MOST PROMISING live node next
(lowest bound for minimization, highest for maximization).
This is provably the strategy that explores the FEWEST
nodes for a given bounding function — it is the "smartest"
default and the one virtually all production solvers use.
Memory: can still be large (heap holds many live nodes),
but the node COUNT explored is minimal.
```

```
LC (Best-First) Branch and Bound — architecture:

                     ┌───────────────────────────┐
                     │      Priority Queue         │
                     │  (min-heap keyed by bound)  │
                     │                              │
                     │  [Node bound=42] ◀── next!   │
                     │  [Node bound=55]             │
                     │  [Node bound=61]             │
                     │  [Node bound=88]             │
                     └──────────┬───────────────────┘
                                │ pop lowest bound
                                ▼
                     ┌───────────────────────────┐
                     │   Is it a complete          │
                     │   solution?                 │
                     └─────┬─────────────┬─────────┘
                     yes   │             │  no
                           ▼             ▼
                  Update "best found"   Branch: generate children,
                  if better than         compute each child's bound,
                  current best           push feasible + promising
                                          children back onto the queue
                                                │
                                                ▼
                                     (loop until queue empty
                                      or best bound in queue ≥
                                      current best solution)
```

**LC is what all three code implementations below use** (via a binary heap / priority queue), since it's the strategy that best demonstrates why B&B is powerful in practice.

---

## 6. General Algorithm Architecture

Pseudocode for the canonical LC (best-first) Branch and Bound, independent of problem:

```
function BRANCH_AND_BOUND(root):
    best_solution := null
    best_value    := -infinity          // for maximization; +infinity for minimization

    priority_queue := new PriorityQueue(ordered by node.bound, best-first)
    priority_queue.push(root)

    while priority_queue is not empty:
        node := priority_queue.pop()    // the most promising live node

        // Pruning check #1: has the global best already surpassed
        // what this node could possibly achieve? If so, EVERYTHING
        // still in the queue is worse too (since it's a priority
        // queue ordered by bound) — we can stop entirely.
        if node.bound <= best_value:    // (maximization; flip for min)
            break                       // nothing left can beat best_value

        if node.is_complete_solution():
            if node.value > best_value:
                best_value    := node.value
                best_solution := node
            continue                    // no children to branch into

        for child in node.branch():     // generate feasible children
            child.bound := compute_bound(child)
            if child.bound > best_value:   // pruning check #2
                priority_queue.push(child)
            // else: DISCARD — this subtree cannot beat best_value

    return best_solution, best_value
```

This is the exact skeleton every implementation below follows, specialized per problem only in `branch()` (how a node produces children) and `compute_bound()` (the problem-specific relaxation).

---

## 7. Worked Example: 0/1 Knapsack — By Hand

**Problem:** given items with `(weight, value)`, and a capacity `W`, choose a subset maximizing total value without exceeding `W`. Each item is either fully included or fully excluded (hence "0/1").

Items sorted by value/weight ratio (a standard preprocessing step that makes the bound tight):

| Item | Weight | Value | Ratio |
|------|--------|-------|-------|
| 1    | 10     | 60    | 6.0   |
| 2    | 20     | 100   | 5.0   |
| 3    | 30     | 120   | 4.0   |

Capacity `W = 50`.

**Bounding function:** the classic **fractional knapsack bound** — greedily fill remaining capacity in ratio order, allowing the *last* item to be taken fractionally. This is an LP relaxation of the integrality constraint and is provably an upper bound on any integral completion.

```
Root (nothing decided, capacity=50, value=0):
    Bound = fill greedily: item1(10,60) + item2(20,100) + 20/30 of item3(120*20/30=80)
          = 60 + 100 + 80 = 240

Branch on item1:
  ├─ Include item1 (weight=10, value=60, remaining cap=40):
  │     Bound = 60 + item2(20,100) + 20/30*item3(80) = 60+100+80 = 240
  │
  └─ Exclude item1 (weight=0, value=0, remaining cap=50):
        Bound = 0 + item2(20,100) + item3 fractional(30/30*120=120) capped by 30 remaining
              = 0 + 100 + 30/30*120... recompute: remaining cap after item2=30, item3 weight
                30 fits exactly → 0+100+120 = 220

Since "Include item1" (240) > "Exclude item1" (220), explore Include-item1 branch first
(this is exactly what the priority queue does automatically by bound value).

Continue branching "Include item1" on item2:
  ├─ Include item1+item2 (weight=30, value=160, remaining cap=20):
  │     Bound = 160 + 20/30*item3(80) = 160+80 = 240
  └─ Include item1, exclude item2 (weight=10, value=60, remaining cap=40):
        Bound = 60 + full item3(120, weight 30 fits in 40) = 60+120=180 (this branch
        is now WORSE than 240 and will be explored later/pruned relative to it)

Continue "Include item1+item2" on item3:
  └─ Include all three? weight=10+20+30=60 > 50 → INFEASIBLE, this child discarded.
  └─ Include item1+item2, exclude item3: weight=30, value=160 → COMPLETE SOLUTION.
        best_value := 160

Now the priority queue's next-highest bound is checked against best_value=160.
Any live node with bound <= 160 is pruned immediately. The "Include item1, exclude
item2" branch (bound=180) is still > 160, so it's explored — but ultimately its
best completion (item1 + item3 = 60+120=180, weight=40<=50) IS feasible and complete,
value=180 > 160 → best_value updated to 180.

Final answer: value = 180, achieved by {item1, item3} (weight 40, value 180).
```

This trace shows the two defining behaviors: **best-first ordering** (exploring the highest-bound branch first, which tends to find good solutions early) and **bound-based pruning** (discarding branches once proven inferior) — together examining a small fraction of the full 2³ = 8-leaf search space.

---

## 8. 0/1 Knapsack — C Implementation

C requires manually implementing the priority queue (as a binary heap) since the standard library has none. This also makes the memory layout of the B&B state fully explicit — valuable for understanding what's actually happening under a Go or Rust abstraction.

```c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    int weight;
    int value;
} Item;

// A node in the state-space tree.
// 'level' = index of the next item to decide (0..n means "decided items 0..level-1")
// 'included' = bitmask-free explicit decision path isn't stored; we recompute
//              weight/value cumulatively instead, which is cheaper than storing
//              the whole path for this problem.
typedef struct {
    int level;
    int weight;      // cumulative weight of included items so far
    int value;       // cumulative value of included items so far
    double bound;    // upper bound on best achievable value from this node
} Node;

static Item *g_items;
static int g_n;
static int g_capacity;

// Fractional knapsack bound: fill remaining capacity greedily by ratio order
// (items must be PRE-SORTED by value/weight descending before calling this).
double compute_bound(Node *node) {
    if (node->weight >= g_capacity) return 0.0; // infeasible node, bound = 0

    double bound = (double)node->value;
    int remaining = g_capacity - node->weight;
    int i = node->level;

    while (i < g_n && g_items[i].weight <= remaining) {
        remaining -= g_items[i].weight;
        bound += g_items[i].value;
        i++;
    }
    if (i < g_n) {
        // take the fractional part of the next item — LP relaxation
        bound += (double)remaining * ((double)g_items[i].value / g_items[i].weight);
    }
    return bound;
}

// ---- Binary max-heap of Node, ordered by 'bound' (best-first / LC strategy) ----
typedef struct {
    Node *data;
    int size;
    int capacity;
} MaxHeap;

MaxHeap *heap_create(int capacity) {
    MaxHeap *h = malloc(sizeof(MaxHeap));
    h->data = malloc(sizeof(Node) * capacity);
    h->size = 0;
    h->capacity = capacity;
    return h;
}

void heap_swap(Node *a, Node *b) { Node tmp = *a; *a = *b; *b = tmp; }

void heap_push(MaxHeap *h, Node node) {
    if (h->size == h->capacity) {
        h->capacity *= 2;
        h->data = realloc(h->data, sizeof(Node) * h->capacity);
    }
    int i = h->size++;
    h->data[i] = node;
    while (i > 0) {
        int parent = (i - 1) / 2;
        if (h->data[parent].bound >= h->data[i].bound) break;
        heap_swap(&h->data[parent], &h->data[i]);
        i = parent;
    }
}

Node heap_pop(MaxHeap *h) {
    Node top = h->data[0];
    h->data[0] = h->data[--h->size];
    int i = 0;
    while (1) {
        int left = 2 * i + 1, right = 2 * i + 2, largest = i;
        if (left < h->size && h->data[left].bound > h->data[largest].bound) largest = left;
        if (right < h->size && h->data[right].bound > h->data[largest].bound) largest = right;
        if (largest == i) break;
        heap_swap(&h->data[i], &h->data[largest]);
        i = largest;
    }
    return top;
}

int item_cmp(const void *a, const void *b) {
    const Item *ia = a, *ib = b;
    double ra = (double)ia->value / ia->weight;
    double rb = (double)ib->value / ib->weight;
    return (ra < rb) - (ra > rb); // descending by ratio
}

int knapsack_branch_and_bound(Item *items, int n, int capacity) {
    g_items = items;
    g_n = n;
    g_capacity = capacity;

    qsort(items, n, sizeof(Item), item_cmp); // sort by ratio — required for the bound

    MaxHeap *pq = heap_create(64);
    int best_value = 0;

    Node root = { .level = 0, .weight = 0, .value = 0 };
    root.bound = compute_bound(&root);
    heap_push(pq, root);

    long nodes_explored = 0;

    while (pq->size > 0) {
        Node node = heap_pop(pq);
        nodes_explored++;

        // Pruning check: if even the optimistic bound can't beat the best found,
        // stop entirely — priority queue guarantees everything else is <= this too.
        if (node.bound <= best_value) break;

        if (node.level == n) continue; // no more items to decide — dead end, not counted as solution here

        // Branch: child that INCLUDES items[node.level]
        Node include = node;
        include.weight += items[node.level].weight;
        include.value  += items[node.level].value;
        include.level  = node.level + 1;
        if (include.weight <= capacity) {
            if (include.value > best_value) best_value = include.value; // complete-ish improvement
            include.bound = compute_bound(&include);
            if (include.bound > best_value) heap_push(pq, include);
        }

        // Branch: child that EXCLUDES items[node.level]
        Node exclude = node;
        exclude.level = node.level + 1;
        exclude.bound = compute_bound(&exclude);
        if (exclude.bound > best_value) heap_push(pq, exclude);
    }

    printf("Nodes explored: %ld (out of 2^%d = %d possible leaves)\n",
           nodes_explored, n, 1 << n);

    free(pq->data);
    free(pq);
    return best_value;
}

int main(void) {
    Item items[] = {
        {10, 60},
        {20, 100},
        {30, 120},
    };
    int n = sizeof(items) / sizeof(items[0]);
    int capacity = 50;

    int best = knapsack_branch_and_bound(items, n, capacity);
    printf("Best value: %d\n", best); // expected: 180

    return 0;
}
```

**Build & run:** `gcc -O2 -o knapsack knapsack.c && ./knapsack`

---

## 9. 0/1 Knapsack — Go Implementation

Go's `container/heap` package provides the heap interface scaffolding; you supply the ordering (`Less`) and storage.

```go
package main

import (
	"container/heap"
	"fmt"
	"sort"
)

// Item represents a knapsack item.
type Item struct {
	Weight, Value int
}

// Node represents a state in the branch-and-bound search tree.
type Node struct {
	Level  int     // index of the next item to decide
	Weight int     // cumulative weight of included items
	Value  int     // cumulative value of included items
	Bound  float64 // optimistic upper bound on achievable value
}

// NodeHeap implements heap.Interface as a MAX-heap ordered by Bound
// (best-first / "LC" branch and bound strategy).
type NodeHeap []Node

func (h NodeHeap) Len() int            { return len(h) }
func (h NodeHeap) Less(i, j int) bool  { return h[i].Bound > h[j].Bound } // max-heap
func (h NodeHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *NodeHeap) Push(x interface{}) { *h = append(*h, x.(Node)) }
func (h *NodeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// computeBound implements the fractional-knapsack LP relaxation bound.
// items MUST be pre-sorted by value/weight descending.
func computeBound(node Node, items []Item, capacity int) float64 {
	if node.Weight >= capacity {
		return 0
	}
	bound := float64(node.Value)
	remaining := capacity - node.Weight
	i := node.Level

	for i < len(items) && items[i].Weight <= remaining {
		remaining -= items[i].Weight
		bound += float64(items[i].Value)
		i++
	}
	if i < len(items) {
		bound += float64(remaining) * (float64(items[i].Value) / float64(items[i].Weight))
	}
	return bound
}

// KnapsackBranchAndBound solves 0/1 knapsack via best-first Branch and Bound.
func KnapsackBranchAndBound(items []Item, capacity int) (bestValue int, nodesExplored int) {
	sort.Slice(items, func(i, j int) bool {
		ratioI := float64(items[i].Value) / float64(items[i].Weight)
		ratioJ := float64(items[j].Value) / float64(items[j].Weight)
		return ratioI > ratioJ // descending
	})

	pq := &NodeHeap{}
	heap.Init(pq)

	root := Node{Level: 0, Weight: 0, Value: 0}
	root.Bound = computeBound(root, items, capacity)
	heap.Push(pq, root)

	for pq.Len() > 0 {
		node := heap.Pop(pq).(Node)
		nodesExplored++

		// Priority-queue pruning: everything remaining has bound <= node.Bound,
		// so if this node can't beat bestValue, nothing else in the queue can either.
		if node.Bound <= float64(bestValue) {
			break
		}
		if node.Level == len(items) {
			continue
		}

		// Branch: include items[node.Level]
		include := node
		include.Weight += items[node.Level].Weight
		include.Value += items[node.Level].Value
		include.Level = node.Level + 1
		if include.Weight <= capacity {
			if include.Value > bestValue {
				bestValue = include.Value
			}
			include.Bound = computeBound(include, items, capacity)
			if include.Bound > float64(bestValue) {
				heap.Push(pq, include)
			}
		}

		// Branch: exclude items[node.Level]
		exclude := node
		exclude.Level = node.Level + 1
		exclude.Bound = computeBound(exclude, items, capacity)
		if exclude.Bound > float64(bestValue) {
			heap.Push(pq, exclude)
		}
	}

	return bestValue, nodesExplored
}

func main() {
	items := []Item{
		{Weight: 10, Value: 60},
		{Weight: 20, Value: 100},
		{Weight: 30, Value: 120},
	}
	capacity := 50

	best, nodes := KnapsackBranchAndBound(items, capacity)
	fmt.Printf("Nodes explored: %d (out of 2^%d = %d possible leaves)\n",
		nodes, len(items), 1<<len(items))
	fmt.Printf("Best value: %d\n", best) // expected: 180
}
```

**Run:** `go run knapsack.go`

---

## 10. 0/1 Knapsack — Rust Implementation

Rust's `std::collections::BinaryHeap` is a max-heap by default, and its ordering is driven by implementing `Ord`/`PartialOrd` — this gives the cleanest, most type-safe expression of the LC strategy of the three languages, at the cost of needing `Eq`/`Ord` boilerplate for floating-point bounds (which don't implement `Ord` natively due to `NaN`).

```rust
use std::cmp::Ordering;
use std::collections::BinaryHeap;

#[derive(Clone, Copy, Debug)]
struct Item {
    weight: u32,
    value: u32,
}

#[derive(Clone, Copy, Debug)]
struct Node {
    level: usize,
    weight: u32,
    value: u32,
    bound: f64,
}

// BinaryHeap is a max-heap ordered by Ord; we define Ord to compare by `bound`
// so the heap always pops the most promising (highest-bound) node first —
// this IS the "LC" (least-cost / best-first) branch and bound strategy.
impl PartialEq for Node {
    fn eq(&self, other: &Self) -> bool {
        self.bound == other.bound
    }
}
impl Eq for Node {}
impl PartialOrd for Node {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}
impl Ord for Node {
    fn cmp(&self, other: &Self) -> Ordering {
        // total_cmp avoids panicking on NaN, which f64's default PartialOrd can't guarantee
        self.bound.total_cmp(&other.bound)
    }
}

/// Fractional-knapsack LP relaxation bound. `items` must be pre-sorted by
/// value/weight descending — this is what makes the bound valid and tight.
fn compute_bound(node: &Node, items: &[Item], capacity: u32) -> f64 {
    if node.weight >= capacity {
        return 0.0;
    }
    let mut bound = node.value as f64;
    let mut remaining = capacity - node.weight;
    let mut i = node.level;

    while i < items.len() && items[i].weight <= remaining {
        remaining -= items[i].weight;
        bound += items[i].value as f64;
        i += 1;
    }
    if i < items.len() {
        bound += remaining as f64 * (items[i].value as f64 / items[i].weight as f64);
    }
    bound
}

/// Solves 0/1 knapsack via best-first Branch and Bound.
/// Returns (best_value, nodes_explored).
fn knapsack_branch_and_bound(mut items: Vec<Item>, capacity: u32) -> (u32, u64) {
    items.sort_by(|a, b| {
        let ratio_a = a.value as f64 / a.weight as f64;
        let ratio_b = b.value as f64 / b.weight as f64;
        ratio_b.total_cmp(&ratio_a) // descending
    });

    let mut pq: BinaryHeap<Node> = BinaryHeap::new();
    let mut best_value: u32 = 0;
    let mut nodes_explored: u64 = 0;

    let mut root = Node { level: 0, weight: 0, value: 0, bound: 0.0 };
    root.bound = compute_bound(&root, &items, capacity);
    pq.push(root);

    while let Some(node) = pq.pop() {
        nodes_explored += 1;

        // Priority-queue pruning: since the heap always yields the highest
        // remaining bound, once this node fails to beat best_value, no
        // remaining node in the queue can either — safe to stop entirely.
        if node.bound <= best_value as f64 {
            break;
        }
        if node.level == items.len() {
            continue;
        }

        // Branch: include items[node.level]
        let item = items[node.level];
        let include_weight = node.weight + item.weight;
        if include_weight <= capacity {
            let include_value = node.value + item.value;
            if include_value > best_value {
                best_value = include_value;
            }
            let mut include = Node {
                level: node.level + 1,
                weight: include_weight,
                value: include_value,
                bound: 0.0,
            };
            include.bound = compute_bound(&include, &items, capacity);
            if include.bound > best_value as f64 {
                pq.push(include);
            }
        }

        // Branch: exclude items[node.level]
        let mut exclude = Node {
            level: node.level + 1,
            weight: node.weight,
            value: node.value,
            bound: 0.0,
        };
        exclude.bound = compute_bound(&exclude, &items, capacity);
        if exclude.bound > best_value as f64 {
            pq.push(exclude);
        }
    }

    (best_value, nodes_explored)
}

fn main() {
    let items = vec![
        Item { weight: 10, value: 60 },
        Item { weight: 20, value: 100 },
        Item { weight: 30, value: 120 },
    ];
    let capacity = 50;
    let n = items.len();

    let (best, nodes) = knapsack_branch_and_bound(items, capacity);
    println!("Nodes explored: {} (out of 2^{} = {} possible leaves)", nodes, n, 1u32 << n);
    println!("Best value: {}", best); // expected: 180
}
```

**Run:** `cargo run --release` (release mode matters for realistic node-count/timing comparisons).

---

## 11. Traveling Salesman Problem via B&B

TSP asks: given `n` cities and pairwise travel costs, find the minimum-cost tour that visits every city exactly once and returns to the start. It's NP-hard, and B&B is one of the classic *exact* (not approximate) solution methods for moderately sized instances.

**The reduced-cost-matrix bounding technique:**

1. Represent costs as an `n x n` matrix, with `∞` on the diagonal (no self-loops) and for any edge already excluded from consideration.
2. **Row reduction**: subtract the minimum value of each row from every entry in that row. The sum of all row-minimums is a valid lower bound contribution (every tour must "pay" at least the row minimum to leave each city).
3. **Column reduction**: same operation on columns (every tour must "pay" at least the column minimum to arrive at each city).
4. The sum of all row and column reduction amounts is the **lower bound** for the root node.
5. When branching (deciding to include or exclude edge `(i, j)` in the tour), the child node's bound is the parent's bound plus the reduction cost of the matrix after fixing that edge (setting row `i` and column `j` to `∞`, and setting `(j, i)` to `∞` to prevent premature sub-cycles).

```
Reduced Cost Matrix — architecture of one branching step:

  Parent matrix M, bound = 25 (after row+col reduction)

  Choose to branch on edge (city A → city B):

  ┌─────────────────────────┐        ┌─────────────────────────┐
  │ INCLUDE edge A→B:         │        │ EXCLUDE edge A→B:         │
  │  - set row A → ∞          │        │  - set M[A][B] = ∞        │
  │  - set col B → ∞          │        │    (forbid this edge,     │
  │  - set M[B][A] = ∞         │        │    but city A, B stay      │
  │    (prevents 2-city        │        │    open for OTHER edges)   │
  │    sub-cycle A→B→A)        │        │                            │
  │  - re-reduce rows/cols     │        │  - re-reduce rows/cols     │
  │  - child.bound = parent    │        │  - child.bound = parent    │
  │    .bound + reduction_cost │        │    .bound + reduction_cost │
  └─────────────────────────┘        └─────────────────────────┘
```

This bound is significantly tighter than a naive "sum of minimum edges" because the row+column reduction captures interaction between choices (an edge's cost is partially "shared" across the row and column commitments), making it practical for exact TSP solving up to roughly 15-25 cities depending on cost structure — beyond that, exact B&B TSP becomes impractical and one switches to approximation (e.g., Christofides) or metaheuristics (simulated annealing, genetic algorithms, Lin-Kernighan).

---

## 12. TSP — Rust Implementation (Reduced Matrix Method)

```rust
use std::collections::BinaryHeap;
use std::cmp::Ordering;

const INF: f64 = f64::INFINITY;

#[derive(Clone)]
struct TspNode {
    matrix: Vec<Vec<f64>>,
    path: Vec<usize>,
    cost: f64,      // accumulated actual edge costs so far
    bound: f64,     // cost + reduced-matrix lower bound on the remainder
    level: usize,   // number of cities fixed into the path so far
}

impl PartialEq for TspNode {
    fn eq(&self, other: &Self) -> bool { self.bound == other.bound }
}
impl Eq for TspNode {}
impl PartialOrd for TspNode {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> { Some(self.cmp(other)) }
}
impl Ord for TspNode {
    fn cmp(&self, other: &Self) -> Ordering {
        // BinaryHeap is a max-heap; we want MIN bound popped first (minimization
        // problem), so we invert the comparison here.
        other.bound.total_cmp(&self.bound)
    }
}

/// Row+column reduction: subtracts row minimums, then column minimums, from
/// the matrix in place. Returns the total amount subtracted — this is the
/// incremental lower-bound contribution from this reduction step.
fn reduce_matrix(matrix: &mut Vec<Vec<f64>>) -> f64 {
    let n = matrix.len();
    let mut reduction = 0.0;

    for i in 0..n {
        let min_val = matrix[i].iter().cloned().fold(INF, f64::min);
        if min_val > 0.0 && min_val < INF {
            for j in 0..n {
                if matrix[i][j] < INF {
                    matrix[i][j] -= min_val;
                }
            }
            reduction += min_val;
        }
    }
    for j in 0..n {
        let mut min_val = INF;
        for i in 0..n {
            if matrix[i][j] < min_val { min_val = matrix[i][j]; }
        }
        if min_val > 0.0 && min_val < INF {
            for i in 0..n {
                if matrix[i][j] < INF {
                    matrix[i][j] -= min_val;
                }
            }
            reduction += min_val;
        }
    }
    reduction
}

fn solve_tsp(cost: Vec<Vec<f64>>) -> (f64, Vec<usize>) {
    let n = cost.len();
    let mut root_matrix = cost.clone();
    for i in 0..n { root_matrix[i][i] = INF; }

    let root_bound = reduce_matrix(&mut root_matrix);
    let root = TspNode {
        matrix: root_matrix,
        path: vec![0],       // fix city 0 as the start
        cost: 0.0,
        bound: root_bound,
        level: 1,
    };

    let mut pq = BinaryHeap::new();
    pq.push(root);

    let mut best_cost = INF;
    let mut best_path = vec![];

    while let Some(node) = pq.pop() {
        if node.bound >= best_cost { break; } // priority queue guarantees no improvement possible

        if node.level == n {
            let last = node.path[node.level - 1];
            let total = node.cost + cost[last][0]; // close the tour back to start
            if total < best_cost {
                best_cost = total;
                best_path = node.path.clone();
            }
            continue;
        }

        let current_city = node.path[node.level - 1];
        for next_city in 0..n {
            if node.matrix[current_city][next_city] == INF { continue; }
            if node.path.contains(&next_city) { continue; }

            let mut child_matrix = node.matrix.clone();
            let edge_cost = child_matrix[current_city][next_city];

            // Fix this edge: forbid leaving current_city again, forbid
            // arriving at next_city from elsewhere, forbid the immediate
            // reverse edge (prevents a premature sub-cycle).
            for j in 0..n { child_matrix[current_city][j] = INF; }
            for i in 0..n { child_matrix[i][next_city] = INF; }
            child_matrix[next_city][current_city] = INF;

            let reduction = reduce_matrix(&mut child_matrix);
            let mut child_path = node.path.clone();
            child_path.push(next_city);

            let child = TspNode {
                matrix: child_matrix,
                path: child_path,
                cost: node.cost + edge_cost,
                bound: node.cost + edge_cost + reduction,
                level: node.level + 1,
            };

            if child.bound < best_cost {
                pq.push(child);
            }
        }
    }

    (best_cost, best_path)
}

fn main() {
    // Classic 4-city TSP instance; INF for no self-loop.
    let cost = vec![
        vec![INF, 10.0, 15.0, 20.0],
        vec![10.0, INF, 35.0, 25.0],
        vec![15.0, 35.0, INF, 30.0],
        vec![20.0, 25.0, 30.0, INF],
    ];

    let (best_cost, best_path) = solve_tsp(cost);
    println!("Best tour cost: {}", best_cost); // expected: 80
    println!("Best tour path: {:?}", best_path);
}
```

---

## 13. Job Assignment Problem

**Problem:** assign `n` jobs to `n` workers, one job per worker, minimizing total cost, given a cost matrix `cost[worker][job]`. This is structurally the *same* reduced-matrix bounding technique as TSP (in fact, it's a special case of the assignment problem which TSP's bound is built from) — worth recognizing so you don't treat every new-looking problem as needing a brand new algorithm.

```
Bounding for Job Assignment (conceptually identical to TSP's row/col reduction):

  cost matrix (worker × job):          After row reduction (subtract row min):
  ┌────┬────┬────┬────┐               ┌────┬────┬────┬────┐
  │  9 │  2 │  7 │  8 │  row min=2    │  7 │  0 │  5 │  6 │
  │  6 │  4 │  3 │  7 │  row min=3    │  3 │  1 │  0 │  4 │
  │  5 │  8 │  1 │  8 │  row min=1    │  4 │  7 │  0 │  7 │
  │  7 │  6 │  9 │  4 │  row min=4    │  3 │  2 │  5 │  0 │
  └────┴────┴────┴────┘               └────┴────┴────┴────┘
  Row reduction total = 2+3+1+4 = 10 → contributes to the lower bound directly.
  Column reduction is then applied on top, same as in TSP.
```

The B&B tree here branches on "assign worker `i` to job `j`" vs "don't assign worker `i` to job `j`," with the same reduced-matrix lower bound recomputed at each node — the Rust `solve_tsp` code above is >80% directly reusable for this problem with the row-exclusivity constraint swapped in for the tour-subcycle constraint.

---

## 14. Complexity Analysis

```
Worst case (bound is useless / brute force):  O(2^n)  for subset-selection problems
                                                O(n!)   for permutation problems (TSP, assignment)

Best case (bound is perfect):                  O(n)   — every wrong branch pruned at depth 1

Practical/typical case:                        Highly instance-dependent. A good bound can
                                                turn an intractable O(2^n) problem into one
                                                that explores a polynomial-sized fraction of
                                                nodes for real-world instances — this is why
                                                B&B has no clean average-case complexity
                                                formula; its performance is empirically
                                                validated per problem class, not derived
                                                analytically like DP or sorting algorithms.
```

Space complexity is dominated by the size of the priority queue, which in the worst case can hold `O(2^n)` live nodes (if pruning is weak) but in well-bounded instances stays small relative to the total search space. This is the practical tradeoff against LIFO/DFS-style B&B, which uses only `O(depth)` = `O(n)` space at the cost of potentially exploring nodes in a less optimal order.

---

## 15. Bounding Function Design Patterns

When designing a bound for a *new* problem, these are the recurring strategies, roughly ordered from simplest to most sophisticated:

1. **Constraint relaxation** — drop one hard constraint (integrality, capacity, precedence) and solve the easier problem. The relaxed optimum is a valid bound because any feasible solution to the original problem is also feasible for the relaxed one.
2. **Greedy completion** — greedily complete the partial solution ignoring some interaction effects (the knapsack fractional-fill bound is this).
3. **Combinatorial relaxation** — replace a hard combinatorial constraint with a simpler necessary condition (MST as a TSP bound: every tour contains a spanning tree, so tour cost ≥ MST cost).
4. **Matrix reduction** — row/column reduction as a lower bound for assignment-style problems (TSP, job assignment).
5. **Lagrangian relaxation** — move a difficult constraint into the objective function with a penalty multiplier, solve the easier unconstrained problem; used in advanced integer programming, generally more sophisticated than what's needed for typical algorithmic interview/competition problems but standard in industrial-strength solvers.
6. **LP relaxation** — for problems that can be expressed as integer linear programs, solve the continuous LP relaxation (via simplex or interior point methods) as the bound; this is what commercial MILP solvers (CPLEX, Gurobi) do at every B&B node, and is the most generally powerful technique, at the cost of needing an LP solver as a subroutine.

**A rule of thumb:** if you can solve a relaxed version of your subproblem in polynomial time, and that relaxed optimum is provably at least as good as any feasible integral solution, you have a valid bound. The engineering craft is making that bound as tight as your time budget allows.

---

## 16. Parallel & Distributed Branch and Bound

Because different subtrees are largely independent once branched, B&B parallelizes naturally — this is heavily used in production MILP solvers running on multi-core/multi-node clusters.

```
                         ┌───────────────────┐
                         │   Shared best-value │
                         │   (atomic / locked)  │
                         └──────────┬──────────┘
              ┌───────────────────┼───────────────────┐
              ▼                    ▼                    ▼
     ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
     │ Worker thread 1   │  │ Worker thread 2   │  │ Worker thread 3   │
     │ own local PQ      │  │ own local PQ      │  │ own local PQ      │
     │ pulls work,        │  │ pulls work,        │  │ pulls work,        │
     │ pushes new         │  │ pushes new         │  │ pushes new         │
     │ children,          │  │ children,          │  │ children,          │
     │ checks/updates      │  │ checks/updates      │  │ checks/updates      │
     │ shared best-value   │  │ shared best-value   │  │ shared best-value   │
     └─────────────────┘  └─────────────────┘  └─────────────────┘
```

Key engineering concerns in a parallel B&B (directly relevant if you're building this in Go with goroutines or Rust with `std::thread`/`rayon`):

- **Work stealing** — idle workers pull nodes from busy workers' queues to balance load, since subtree sizes are unpredictable.
- **Best-value synchronization** — the shared best-value must be read/written atomically (or via a lock), and workers should re-check it against their own local nodes' bounds frequently, since a node one worker pushed might become prunable the instant *another* worker finds a better solution.
- **Termination detection** — the algorithm is done when every worker's local queue is empty AND no work is in flight — a classic distributed termination problem, usually solved with a global counter of "outstanding nodes" or a termination barrier.
- Go's goroutines + channels map naturally onto a work-stealing dispatcher; Rust's `rayon` crate provides work-stealing thread pools out of the box and pairs well with `std::sync::atomic` for the shared best-value.

---

## 17. Real-World Applications

- **Mixed-Integer Linear Programming (MILP) solvers** — CPLEX, Gurobi, SCIP, and Google OR-Tools all use Branch and Bound (specifically "Branch and Cut," which adds cutting-plane refinement to a B&B core) as their fundamental exact-solving engine. Nearly every "solve this optimization problem" enterprise tool sits on top of B&B.
- **Job/task scheduling** — assigning tasks to CPUs/machines to minimize makespan or cost, common in cluster schedulers and CI/CD pipeline optimizers.
- **VLSI circuit design** — placement and routing problems (minimizing wire length / chip area) are combinatorial optimization problems solved historically with B&B variants.
- **Network routing / bandwidth allocation** — optimal path or flow assignment under capacity constraints, relevant to the kind of packet-processing and network engineering work involving BGP/routing decisions.
- **Compiler register allocation** (graph-coloring-adjacent problems) — some register allocation formulations use B&B-style exact search for small problem instances (e.g., optimal instruction scheduling in embedded/DSP compilers where code quality matters more than compile time).
- **Bioinformatics** — genome assembly and phylogenetic tree construction use B&B for exact solutions on small instances before falling back to heuristics at scale.

---

## 18. Common Pitfalls

- **Using a bound that isn't actually valid.** If your "optimistic" bound can sometimes be *worse* than the true achievable value (for maximization) or *better* than the true minimum (for minimization), you will silently prune away the actual optimal solution and return a wrong answer with no indication anything went wrong. Always prove your bound direction mathematically before trusting the algorithm's output.
- **Forgetting to update the best-found value from non-leaf nodes.** In several problems (knapsack above included), a *partial* solution can already represent a valid, complete-enough candidate (e.g., "include" branches that fit capacity are themselves valid item selections, not just intermediate states) — missing this means comparing against a worse baseline than necessary and pruning less aggressively than possible.
- **Sorting/preprocessing being required for the bound to be valid, but being forgotten.** The knapsack fractional bound is only correct if items are pre-sorted by ratio — an unsorted greedy fill is neither optimal nor a valid bound.
- **Recomputing the bound from scratch when incremental updates are possible.** Especially in matrix-reduction methods (TSP, assignment), recomputing the *entire* bound at each node instead of incrementally updating from the parent's already-reduced matrix turns an efficient algorithm into a slow one — asymptotically it can turn a well-pruned search into one with high constant-factor overhead per node.
- **Choosing DFS/LIFO when you actually need best-first behavior, or vice versa.** DFS finds *a* feasible solution fast (good for getting an early prune baseline in memory-constrained settings) but not necessarily a *good* one — if node count matters more than memory, LC/best-first is almost always the better default.
- **Floating-point bound comparisons without an epsilon or ordering-safe comparator.** As shown in the Rust code, raw floats don't implement `Ord` due to `NaN`; using `total_cmp` (or explicit epsilon-based comparisons) avoids silent bugs or panics when integrating with heap containers that require a total order.
- **Ignoring integer overflow in cost/value accumulation** in C in particular — cumulative costs in deep trees can overflow `int` for large instances; use wider integer types or check bounds explicitly, especially in production numeric code.

---

## 19. Mental Model Cheat Sheet

- **B&B = backtracking + one extra pruning rule**: prune not just on infeasibility, but on *provable sub-optimality*, using a bound.
- **The bound must never lie in your favor.** Maximization → bound must never underestimate. Minimization → bound must never overestimate. This single invariant is the correctness foundation of the entire algorithm.
- **The bound comes from relaxation** — solve an easier version of the subproblem (drop integrality, drop a constraint, ignore an interaction) and use its value.
- **Best-first (LC) search, via a priority queue keyed on bound, explores the fewest nodes** for a given bound quality — it's the default strategy in essentially every production solver.
- **A node either gets pruned, becomes a new best solution, or gets branched into children** — there is no fourth outcome.
- **When the queue's next-best bound can't beat your current best, you're done** — this is the termination condition, and it's what makes B&B *exact* (not approximate) despite skipping most of the search space.
- **DP, greedy, and backtracking are not competitors to B&B so much as building blocks inside it** — greedy completion is often the bounding function; DP relaxations can serve as bounds; backtracking is B&B with the bound disabled.
- **Parallel B&B is "embarrassingly parallel" at the branching level, but requires careful shared-state synchronization** for the best-value and termination detection — the same concerns you'd recognize from any concurrent work-stealing system.

---

*End of guide.*
