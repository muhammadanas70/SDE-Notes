Graph algorithms are techniques for traversing, searching, and analyzing graphs — structures made of **nodes (vertices)** connected by **edges**. They power things like shortest-path routing, dependency resolution, network analysis, and cycle detection.

## Core Ideas

**Representation**: Most commonly an **adjacency list** — a map/array where each node points to a list of its neighbors. This is more space-efficient than an adjacency matrix for sparse graphs.

**Traversal — the foundation of almost everything:**

- **BFS (Breadth-First Search)**: Explore level by level using a **queue**. Visit all neighbors of a node before moving further out. Guarantees the *shortest path* in an unweighted graph.
- **DFS (Depth-First Search)**: Explore as deep as possible before backtracking, using a **stack** (or recursion). Good for detecting cycles, topological sorting, connected components.

**Common algorithms built on these:**
- **Dijkstra's** — shortest path in weighted graphs (BFS + priority queue)
- **Union-Find** — track connected components efficiently
- **Topological Sort** — ordering with dependencies (DFS-based)
- **A\*** — heuristic-guided shortest path

For LeetCode-style problems, BFS/DFS cover probably 80% of graph questions (number of islands, course schedule, clone graph, etc.), so let's nail those.

---

## BFS Example: Shortest path / level traversal on an unweighted graph

**Go:**

```go
package main

import "fmt"

func bfs(graph map[int][]int, start int) map[int]int {
	dist := map[int]int{start: 0}
	queue := []int{start}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		for _, neighbor := range graph[node] {
			if _, visited := dist[neighbor]; !visited {
				dist[neighbor] = dist[node] + 1
				queue = append(queue, neighbor)
			}
		}
	}
	return dist
}

func main() {
	graph := map[int][]int{
		1: {2, 3},
		2: {1, 4},
		3: {1, 4},
		4: {2, 3, 5},
		5: {4},
	}

	distances := bfs(graph, 1)
	for node, d := range distances {
		fmt.Printf("Node %d: distance %d\n", node, d)
	}
}
```

**Rust:**

```rust
use std::collections::{HashMap, VecDeque};

fn bfs(graph: &HashMap<i32, Vec<i32>>, start: i32) -> HashMap<i32, i32> {
    let mut dist: HashMap<i32, i32> = HashMap::new();
    dist.insert(start, 0);

    let mut queue: VecDeque<i32> = VecDeque::new();
    queue.push_back(start);

    while let Some(node) = queue.pop_front() {
        if let Some(neighbors) = graph.get(&node) {
            for &neighbor in neighbors {
                if !dist.contains_key(&neighbor) {
                    let d = dist[&node] + 1;
                    dist.insert(neighbor, d);
                    queue.push_back(neighbor);
                }
            }
        }
    }
    dist
}

fn main() {
    let mut graph: HashMap<i32, Vec<i32>> = HashMap::new();
    graph.insert(1, vec![2, 3]);
    graph.insert(2, vec![1, 4]);
    graph.insert(3, vec![1, 4]);
    graph.insert(4, vec![2, 3, 5]);
    graph.insert(5, vec![4]);

    let distances = bfs(&graph, 1);
    let mut nodes: Vec<&i32> = distances.keys().collect();
    nodes.sort();
    for node in nodes {
        println!("Node {}: distance {}", node, distances[node]);
    }
}
```

---

## Key differences you'll notice going Go → Rust

- Go's `map` is native and easy; Rust's `HashMap` needs explicit imports and slightly more ceremony around borrowing (`&`, `.get()`).
- Rust forces you to handle the "might not exist" case via `Option` (`if let Some(...)`), whereas Go uses the `value, ok := map[key]` idiom.
- Go's garbage collector means you don't think about ownership; Rust's borrow checker will flag if you try to mutate `graph` while iterating over it — a classic gotcha in Rust graph algorithms.

Want me to follow up with DFS, a weighted-graph example (Dijkstra), or a specific LeetCode problem (e.g., "Number of Islands" or "Course Schedule") worked through in both languages?

Let's break down all three — they're the backbone of most graph/tree problems.

## BFS (Breadth-First Search)

**What it does**: Explores a graph/tree level by level, visiting all neighbors at the current depth before moving to the next depth.

**Data structure**: Queue (FIFO — First In, First Out)

**Why it works**: Since you fully process one "layer" before starting the next, the first time you reach a node, you've reached it via the shortest possible path (in terms of number of edges, unweighted).

**Mental model**: Imagine dropping a stone in a pond — ripples expand outward evenly. BFS is that ripple.

**Use cases**: Shortest path (unweighted), level-order tree traversal, "minimum steps to reach X", finding connected components.

```
Start at A
Queue: [A]
Visit A, enqueue its neighbors (B, C) → Queue: [B, C]
Visit B, enqueue its neighbors → Queue: [C, D, E]
Visit C, enqueue its neighbors → Queue: [D, E, F]
... and so on
```

---

## DFS (Depth-First Search)

**What it does**: Goes as deep as possible down one path before backtracking to try another.

**Data structure**: Stack (LIFO — Last In, First Out) — either explicit, or implicit via **recursion** (the call stack *is* a stack).

**Mental model**: Like exploring a maze by always taking the first available turn, going until you hit a dead end, then backtracking to the last junction and trying the next option.

**Use cases**: Cycle detection, topological sort, path existence, counting connected components/islands, backtracking problems.

```
Start at A
Go to B (first neighbor)
Go to D (first neighbor of B)
Dead end → backtrack to B
Go to E (next neighbor of B)
...
```

**BFS vs DFS — the core tradeoff:**

| | BFS | DFS |
|---|---|---|
| Structure | Queue | Stack / recursion |
| Finds shortest path? | Yes (unweighted) | No |
| Memory | Can be worse (stores whole frontier) | Can be better (stores one path) |
| Natural fit for | "Closest"/"minimum" questions | "Does a path exist", exhaustive search |

---

## Priority Queue

**What it is**: Not FIFO or LIFO — it's a queue where each element has a **priority**, and you always pop the element with the highest (or lowest) priority first. Typically implemented as a **binary heap** under the hood, giving O(log n) insert/extract.

**Why graphs need it**: In a *weighted* graph, BFS's "level by level" guarantee breaks — a node 3 hops away with cheap edges might actually be closer than a node 1 hop away with an expensive edge. A priority queue lets you always expand the currently-cheapest-known node next. This is exactly **Dijkstra's algorithm** — it's BFS, but the queue is replaced with a min-priority-queue ordered by distance.

**Mental model**: A hospital ER triage line — not first-come-first-served, but most-critical-first.

**Use cases**: Dijkstra's shortest path, A*, task scheduling, "k closest points", merging k sorted lists.

---

### Quick reference in Go and Rust

| Concept | Go | Rust |
|---|---|---|
| Queue (BFS) | slice used as queue (`queue = queue[1:]`), or `container/list` | `VecDeque` (`push_back`/`pop_front`) |
| Stack (DFS) | slice used as stack (`append`/pop last), or plain recursion | `Vec` (`push`/`pop`), or plain recursion |
| Priority Queue | `container/heap` — you implement `Len`, `Less`, `Swap`, `Push`, `Pop` yourself (a bit verbose) | `std::collections::BinaryHeap` — built-in max-heap; for min-heap wrap values in `Reverse(...)` or implement `Ord` inverted |

Want me to show DFS and a priority-queue-based Dijkstra example next, side by side in Go and Rust — the way we did BFS?

This is the same graph from the BFS code example earlier — nodes 1 through 5, edges connecting them. The color shows what BFS discovers as it runs: node 1 is the start (distance 0), nodes 2 and 3 are its direct neighbors (distance 1), node 4 is reachable through either of them (distance 2), and node 5 is furthest out (distance 3).

Notice node 4 has *two* incoming edges — from node 2 and node 3 — but BFS only records it once, at distance 2, because that's the moment the queue *first* reaches it. Whichever of node 2 or 3 gets processed first in the queue is what "discovers" node 4; the second path to it is simply ignored since it's already marked visited. That's exactly the `if _, visited := dist[neighbor]; !visited` check in the Go code and `if !dist.contains_key(&neighbor)` in Rust — the guard that keeps BFS from reprocessing a node or producing an incorrect (longer) distance.

If you traced this with a stack instead (DFS), you might hit 1 → 2 → 4 → 5 → *then* backtrack to 3 — no clean "distance from start" ordering, just depth-first wandering with backtracking. That's the structural difference between the two.

Want to see the same graph traced with DFS next, or move on to a weighted version with a priority queue (Dijkstra)?

## Concept: What Recursion Actually Is

Recursion is a way of solving a problem by expressing it in terms of a **smaller version of itself**, plus a **stopping condition** that doesn't need further breakdown. That's the entire idea — everything else is detail.

**Why it exists:** some problems have a structure where "solve the whole thing" and "solve a smaller piece of the same thing" are literally the same operation — trees, nested structures, "combine the answer for this + the answer for the rest." Iteration (loops) can't express that self-similarity directly; recursion can.

## Mental Model: Two Parts, Always

Every correct recursive function has exactly two parts:

1. **Base case** — the smallest version of the problem, answered directly, no further recursion. This is what *stops* the recursion.
2. **Recursive case** — express the current problem in terms of a smaller subproblem, then trust that the smaller call already works.

That second word — **trust** — is the actual skill. This is called the **"leap of faith"**: when you write the recursive call, you don't trace through what it does internally. You just believe it correctly solves the smaller version, and you focus only on how to combine that smaller answer into the answer for your current, bigger input.

```text
factorial(n) = n * factorial(n - 1)     <- recursive case: express in terms of a smaller problem
factorial(0) = 1                         <- base case: smallest version, answered directly

You do NOT need to think about factorial(n-1)'s internals.
You only need to answer: "if I HAD factorial(n-1), how would I build factorial(n) from it?"
Answer: multiply by n. That's the whole design step.
```

## ASCII Diagram: The Call Stack

This is the part people skip and then get confused later. Recursion has two directions of travel: **going down** (each call makes a smaller call) and **coming back up** (each call uses the smaller answer to build its own).

```text
factorial(4)

GOING DOWN (each call pauses, waiting on a smaller call):

  factorial(4)  "I need factorial(3) first..."
     |
     v
  factorial(3)  "I need factorial(2) first..."
     |
     v
  factorial(2)  "I need factorial(1) first..."
     |
     v
  factorial(1)  "I need factorial(0) first..."
     |
     v
  factorial(0)  "I'm the base case. I know the answer: 1"  <- BOTTOM, turns around here


COMING BACK UP (each paused call resumes, now that it has its answer):

  factorial(0) returns 1
  factorial(1) resumes: 1 * 1 = 1,  returns 1
  factorial(2) resumes: 2 * 1 = 2,  returns 2
  factorial(3) resumes: 3 * 2 = 6,  returns 6
  factorial(4) resumes: 4 * 6 = 24, returns 24   <- final answer


Call stack at the deepest point (factorial(0) about to return):

  +----------------+
  | factorial(0)=1 |  <- top of stack, about to return
  +----------------+
  | factorial(1)   |  waiting on factorial(0)
  +----------------+
  | factorial(2)   |  waiting on factorial(1)
  +----------------+
  | factorial(3)   |  waiting on factorial(2)
  +----------------+
  | factorial(4)   |  waiting on factorial(3)  <- bottom, original call
  +----------------+
```

## Dry Run Table

| Step | Call | Waiting On | Once Resolved |
|---|---|---|---|
| 1 | `factorial(4)` | `factorial(3)` | `4 * 6 = 24` |
| 2 | `factorial(3)` | `factorial(2)` | `3 * 2 = 6` |
| 3 | `factorial(2)` | `factorial(1)` | `2 * 1 = 2` |
| 4 | `factorial(1)` | `factorial(0)` | `1 * 1 = 1` |
| 5 | `factorial(0)` | — (base case) | returns `1` directly |

Notice the answers get computed **bottom-up**, even though you wrote the function **top-down**. This mismatch between "how you write it" and "how it executes" is exactly what trips people up. That's why the leap of faith matters — you're not supposed to mentally simulate the whole stack every time.

## The General Framework: How to Design Any Recursive Solution

This is the actual repeatable process — apply this to any recursion problem you're given:

```text
1. What's the SMALLEST input where I can answer immediately, with no recursion?
   -> That's your base case. Write it first. Always.

2. Assume a magic function already exists that correctly solves a SLIGHTLY
   smaller version of this exact problem. What does it return to me?

3. Given that smaller answer, what's the minimum work I do to build
   the answer for MY input?
   -> That combining step is your recursive case.

4. Does step 3 actually make progress toward the base case?
   -> If the "smaller" call isn't strictly smaller, you get infinite recursion.

5. Do the base case and recursive case together cover EVERY possible input?
   -> Missing a case is the #1 source of "works on my example, crashes on theirs."
```

Applied to factorial: (1) smallest input is `n=0`, answer is `1`. (2) assume `factorial(n-1)` works. (3) multiply by `n`. (4) `n-1 < n`, always shrinking. (5) every non-negative integer eventually reaches 0.

## A Second Example: Summing an Array (to show "chunking" more directly)

This is closer to what you mean by "solve small chunks, then the big problem" — recursion on a *collection*, not just a number.

```text
sum([2, 4, 6, 8])

Think of it as: sum(array) = first_element + sum(rest_of_array)

  sum([2,4,6,8]) = 2 + sum([4,6,8])
  sum([4,6,8])   = 4 + sum([6,8])
  sum([6,8])     = 6 + sum([8])
  sum([8])       = 8 + sum([])
  sum([])        = 0                 <- base case: empty array, nothing to sum

Unwinding:
  sum([8])       = 8 + 0  = 8
  sum([6,8])     = 6 + 8  = 14
  sum([4,6,8])   = 4 + 14 = 18
  sum([2,4,6,8]) = 2 + 18 = 20
```

Every recursive call handles **one element** (the smallest possible "chunk") and delegates the rest — that's the literal answer to "solve that chunk, then trust the smaller problems solve themselves."

## Complexity

- `factorial`/`sum` style (one recursive call per level): `n` calls total → **O(n) time**, and **O(n) space** for the call stack itself (this is real memory, not free — each paused call sits on the stack until it resumes).
- This space cost is the reason very deep recursion (millions of calls) can crash with a stack overflow, and why some problems get rewritten iteratively once you understand the recursive shape.

## Go Implementation

```go
package main

import "fmt"

func Factorial(n int) int {
	if n == 0 { // base case
		return 1
	}
	return n * Factorial(n-1) // recursive case: trust Factorial(n-1), just combine
}

func Sum(arr []int) int {
	if len(arr) == 0 { // base case
		return 0
	}
	return arr[0] + Sum(arr[1:]) // recursive case: first element + sum of the rest
}

func main() {
	fmt.Println(Factorial(4)) // 24
	fmt.Println(Sum([]int{2, 4, 6, 8})) // 20
}
```

## Rust Implementation

```rust
fn factorial(n: u64) -> u64 {
    if n == 0 { // base case
        1
    } else {
        n * factorial(n - 1) // recursive case
    }
}

fn sum(arr: &[i64]) -> i64 {
    match arr {
        [] => 0,                          // base case
        [first, rest @ ..] => first + sum(rest), // recursive case
    }
}

fn main() {
    println!("{}", factorial(4)); // 24
    println!("{}", sum(&[2, 4, 6, 8])); // 20
}
```

**Rust note:** `[first, rest @ ..]` is pattern-matching a slice into "first element" and "everything else" in one line — this *is* the recursive decomposition, made visible directly in the syntax, which is a nice reinforcement of the mental model.

## Common Mistakes

| Mistake | Why it seems reasonable | Failure |
|---|---|---|
| Forgetting the base case entirely | "I'll add it later once the logic works" | Infinite recursion → stack overflow, every time |
| Base case doesn't cover all "smallest" inputs (e.g., only handling `n==0`, not negative `n`) | Testing only happy-path examples | Crashes or infinite-loops on untested small inputs |
| Recursive call doesn't shrink the problem (e.g., calling `factorial(n)` instead of `factorial(n-1)` by typo) | Easy copy-paste slip | Infinite recursion, identical call repeated forever |
| Trying to mentally trace the entire call stack every time instead of trusting the leap of faith | Feels like "really understanding" it | Slows you down enormously and doesn't scale to harder recursion (trees, backtracking) |

## Active Recall Questions

1. Without looking above, write out the base case and recursive case for computing the sum of digits of a number (e.g., `1234 -> 1+2+3+4 = 10`).
2. In the array `Sum` example, why does the recursion always terminate — what guarantees `len(arr)` eventually hits 0?
3. What would happen if `Factorial`'s base case checked `n == 1` instead of `n == 0`? Would it still work? What if someone calls `Factorial(0)`?
4. Explain the "leap of faith" in your own words — why is it *not* cheating to assume `factorial(n-1)` works before you've finished writing `factorial(n)`?

## Practice Problems

- **Beginner:** Write a recursive function to compute the sum of digits of a positive integer.
- **Intermediate:** Write a recursive function to reverse a string (no loops allowed) — figure out what the "smaller chunk" and "combine step" are.
- **Variation:** Write a recursive function to compute the `n`th Fibonacci number, then trace why its call-stack shape is different from `factorial`'s (it branches into *two* recursive calls per level, not one) — what does that do to the total number of calls?

Want to go through the Fibonacci variation together (Mode B — Solve With Me), since it's the natural next step once single-branch recursion clicks?