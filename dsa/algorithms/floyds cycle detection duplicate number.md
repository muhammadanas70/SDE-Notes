# Floyd's Cycle-Finding Algorithm — A Complete Deep Dive via LeetCode 287 (Find the Duplicate Number)

> **Scope of this guide**: We don't just solve LC 287. We build the full mental model — the discrete-math reasoning that makes an array look like a linked list, the tortoise-and-hare proof (why it *must* terminate, why it *must* be correct), the ASCII architecture of the "rho" (ρ) shape, complexity/space trade-offs against every competing approach, and production-grade implementations in **Go, C, and Rust** with real error handling, not toy snippets.

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [The Core Insight: An Array Is a Linked List in Disguise](#2-the-core-insight-an-array-is-a-linked-list-in-disguise)
3. [Why a Duplicate Guarantees a Cycle (Pigeonhole Proof)](#3-why-a-duplicate-guarantees-a-cycle-pigeonhole-proof)
4. [The Rho (ρ) Shape — ASCII Architecture](#4-the-rho-ρ-shape--ascii-architecture)
5. [Floyd's Algorithm: Full Mathematical Derivation](#5-floyds-algorithm-full-mathematical-derivation)
6. [Step-by-Step Trace on a Real Example](#6-step-by-step-trace-on-a-real-example)
7. [Why Phase 2 Finds the Cycle Entrance — Full Proof](#7-why-phase-2-finds-the-cycle-entrance--full-proof)
8. [Complexity Analysis](#8-complexity-analysis)
9. [Alternative Approaches — Full Comparison](#9-alternative-approaches--full-comparison)
10. [Edge Cases and Correctness Pitfalls](#10-edge-cases-and-correctness-pitfalls)
11. [Implementation: Go](#11-implementation-go)
12. [Implementation: C](#12-implementation-c)
13. [Implementation: Rust](#13-implementation-rust)
14. [Real-World Applications of Floyd's Algorithm](#14-real-world-applications-of-floyds-algorithm)
15. [Expert Mental Model — How to Internalize This Forever](#15-expert-mental-model--how-to-internalize-this-forever)
16. [Testing Matrix](#16-testing-matrix)
17. [Summary Cheat Sheet](#17-summary-cheat-sheet)

---

## 1. Problem Statement

**LeetCode 287 — Find the Duplicate Number**

Given an array of integers `nums` containing `n + 1` integers where each integer is in the range `[1, n]` inclusive, there is exactly one repeated number. Assume the repeated number can repeat more than once. Find the repeated number.

**Constraints that matter enormously**:
- `1 <= n <= 10^5`
- `nums.length == n + 1`
- `1 <= nums[i] <= n`
- All values appear at least once except one value that appears **at least twice**.

**Follow-up constraints** (these are the whole reason Floyd's algorithm exists as the intended solution):
1. You must not modify the array `nums`.
2. You must use only constant, `O(1)` extra space.
3. Your runtime complexity should be less than `O(n^2)`.
4. Time complexity better than or equal to `O(n log n)` is a bonus (Floyd's gives `O(n)`).

Read the constraints again: **no modification, no extra data structure, sub-quadratic time**. A hash set is `O(n)` time but violates constraint 2. Sorting is `O(n log n)` but violates constraint 1 (or costs `O(n)` extra space if you copy first). Floyd's cycle detection is the *only* approach satisfying all four constraints simultaneously — and that's precisely why this problem is the canonical showcase for the algorithm.

---

## 2. The Core Insight: An Array Is a Linked List in Disguise

This is the single most important conceptual leap in the entire problem, and if you internalize nothing else, internalize this.

Because every value in `nums` is constrained to the range `[1, n]`, and the array has indices `[0, n]`, **every value in the array is also a valid index into the array**. This lets us treat the array as a **function**:

```
f(i) = nums[i]
```

This function maps every index `i ∈ [0, n]` to some value `nums[i] ∈ [1, n]`, which is itself a valid index. We can therefore **iterate** the function:

```
i₀ = 0
i₁ = f(i₀) = nums[0]
i₂ = f(i₁) = nums[nums[0]]
i₃ = f(i₂) = nums[nums[nums[0]]]
...
```

Each `iₖ` is simultaneously "the current array index" and "a node in an implicit linked list," where the "next pointer" of node `i` is `nums[i]`. This is exactly the structure of a **singly linked list** — every node points to exactly one other node (possibly itself, possibly one already visited).

```
ARRAY VIEW:                       IMPLICIT LINKED-LIST VIEW:

index:  0   1   2   3   4          0 ──▶ nums[0]
value: [3,  4,  2,  1,  2]          1 ──▶ nums[1]
                                    2 ──▶ nums[2]
                                    3 ──▶ nums[3]
                                    4 ──▶ nums[4]
```

Starting the walk at node `0` (never a "value," always safe to use as the universal start point since indices go `0..n` but values only go `1..n`), we get a deterministic path through the array. Because the domain (`n+1` indices) is finite and every node has **exactly one outgoing edge**, this structure is called a **functional graph** — and a functional graph on a finite domain always eventually cycles. It cannot wander forever without repeating a node (pigeonhole again, applied at the graph-walk level, not just the array-value level).

**Why index 0 as start, not value-space?** Index 0 is guaranteed to exist and is never itself constrained to `[1, n]` as a *value* (the problem doesn't require 0 to appear as a value) — so starting the walk there guarantees we're outside the "pure value cycle" until we walk into it, which is exactly the ρ-shape we need (explained in §4).

---

## 3. Why a Duplicate Guarantees a Cycle (Pigeonhole Proof)

We need to prove two things:

**Claim A**: The functional graph described above *must* contain a cycle.
**Claim B**: The node where the cycle begins (the cycle's "entry point") is exactly the duplicated value.

### Proof of Claim A

The graph has `n + 1` nodes (indices `0` through `n`), and every node has out-degree exactly 1 (since `f(i) = nums[i]` is a total function — every index has a value). A directed graph where every node has out-degree 1, walked from any starting node, must revisit a node within at most `n + 2` steps (there are only `n + 1` distinct nodes to visit before a repeat is forced — pigeonhole principle). Once a node is revisited, by definition of "the next node is a deterministic function of the current node," the walk from that point on repeats identically forever. This is a cycle. **∎**

This holds *regardless* of whether there's a duplicate value in the array — any total function on a finite domain, iterated, cycles. So Claim A alone doesn't yet explain why the duplicate matters. That's Claim B.

### Proof of Claim B

Consider node `0`. By problem constraints, no array value ever equals `0` (values are in `[1, n]`, not `[0, n]`). Therefore **node 0 has in-degree 0** in this functional graph — nothing ever points back to it. This means node `0` cannot be part of a cycle (a cycle requires every member to have in-degree ≥ 1 *within the cycle*). So node `0` is strictly a "tail" leading into the cycle, never inside it.

Now, values `1..n` are the possible destinations, and there are `n` such values but `n+1` nodes total (`0..n`) feeding into them (every node `0..n` has an outgoing edge landing in `[1, n]`). By pigeonhole, at least one value in `[1, n]` must be **pointed to by two or more different nodes** — i.e., at least one value has **in-degree ≥ 2**. This is only possible in a functional graph if that node is the **entry point of the cycle**: a node with in-degree ≥ 2 must be receiving one edge from "inside the loop" (the previous cycle node) and at least one edge from "outside the loop" (a tail node), and the only place two path-independent arrivals converge into a single deterministic future is the cycle's entrance.

**And this node with in-degree ≥ 2 is *precisely* the duplicated array value** — because "value `v` has in-degree ≥ 2" literally means "two different indices `i` and `j` satisfy `nums[i] = nums[j] = v`," which is the definition of `v` being a duplicate.

Since the problem guarantees exactly one duplicate, there is exactly one node with in-degree ≥ 2, and it is exactly the cycle's entry point. **∎**

This proof is the entire reason the problem is solvable with cycle detection: *the answer we want (the duplicate) is mathematically identical to the answer cycle detection is built to find (the cycle entrance).*

---

## 4. The Rho (ρ) Shape — ASCII Architecture

The functional graph we've built always has a specific shape called "rho" because it visually resembles the Greek letter **ρ**: a straight "tail" leading into a circular "loop."

```
                         RHO (ρ) SHAPE OF THE FUNCTIONAL GRAPH
                         ══════════════════════════════════════

    TAIL (μ nodes)                        CYCLE (λ nodes)

    ┌───┐    ┌───┐    ┌───┐         ┌───▶│ E │───┐
    │ 0 │───▶│ i1│───▶│ i2│───...──▶│    └───┘   │
    └───┘    └───┘    └───┘         │      │     │
     start                          │      ▼     │
                                     │    │ ? │   │
                                     │    └───┘   │
                                     │      │     ▼
                                     │      ▼   ┌───┐
                                     │    ┌───┐  │ ? │
                                     └────│ ? │◀─┘───┘
                                          └───┘

    E = "cycle Entrance" = the DUPLICATE VALUE (the answer)
    μ (mu)  = length of the tail (distance from start to E)
    λ (lambda) = length of the cycle (number of nodes in the loop)
```

Formal parameters, universally used in cycle-detection literature (Knuth, Floyd, Brent):

| Symbol | Meaning |
|---|---|
| `μ` (mu) | Number of steps from the start node to the cycle entrance (tail length) |
| `λ` (lambda) | Number of nodes in the cycle itself (cycle length) |
| `x₀` | The starting node (here, always index `0`) |
| `xᵢ` | The node after `i` steps of iterating `f` |

The key topological fact Floyd's algorithm exploits: **once you're on the cycle, every `λ` steps you return to the same node.** So `xᵢ = xⱼ` if and only if `i ≡ j (mod λ)` **and** both `i, j ≥ μ` (i.e., both indices are past the tail, inside the loop).

---

## 5. Floyd's Algorithm: Full Mathematical Derivation

Floyd's algorithm (a.k.a. "tortoise and hare") uses **two pointers** moving at different speeds through the same functional graph:

- **Slow pointer (tortoise)**: moves 1 step per iteration: `slow = f(slow)`
- **Fast pointer (hare)**: moves 2 steps per iteration: `fast = f(f(fast))`

It operates in **two phases**.

### Phase 1 — Detect *that* a cycle exists, and find *a* meeting point inside it

```
slow = f(start)      // 1 step
fast = f(f(start))   // 2 steps
while slow != fast:
    slow = f(slow)         // +1
    fast = f(f(fast))      // +2
// loop exits when slow == fast — guaranteed to happen (proved below)
```

**Why they must meet:** Once both pointers are inside the cycle (which is guaranteed to happen after at most `μ` steps for the slow pointer), consider their positions **modulo λ**. Every iteration, the fast pointer gains exactly 1 net step on the slow pointer (`+2` vs `+1`). Since the cycle has finite length `λ`, the "gap" between fast and slow, taken mod `λ`, decreases by 1 every iteration (equivalently, the gap cycles through every residue `0..λ-1`). The gap **must** hit `0 (mod λ)` within at most `λ` iterations after both are inside the loop — at that point `slow == fast` exactly, because being at the same position mod `λ` inside a cycle of length `λ` *means* being at the identical node. **∎ (they always meet, never loop forever)**

This is a **discrete pursuit problem**: two runners on a circular track of length `λ`, one twice as fast as the other, always meet — this is the same principle as two hands of a clock overlapping periodically.

### Phase 2 — Find the exact cycle entrance (the duplicate)

```
slow2 = start
while slow2 != fast:
    slow2 = f(slow2)   // +1 step
    fast   = f(fast)    // +1 step (now same speed!)
return slow2   // this is the cycle entrance = the duplicate value
```

Phase 2 resets one pointer to the start and moves **both** pointers one step at a time. The claim — proved rigorously in §7 — is that they now meet **exactly at the cycle entrance `E`**.

### Why This Works — Intuition Before the Formal Proof

At the moment Phase 1 ends, the slow pointer has taken `t` steps (`t ≥ μ`), and the fast pointer has taken `2t` steps, and both land on the same node. The extra distance the fast pointer covered, `2t - t = t`, must be an exact multiple of the cycle length `λ` (because the only way "twice as far" lands on the same node in a cycle is if the extra distance is a whole number of laps). So `t ≡ 0 (mod λ)`.

Now here's the elegant trick: since `t` is a multiple of `λ`, walking `μ` more steps from the meeting point (which is `t mod λ` steps into the cycle from `E`, i.e., since `t ≡ 0 mod λ`, **the meeting point is exactly `t mod λ = 0` steps past... no wait — careful**, let's redo this with full rigor in §7, because hand-waving here is exactly how people memorize the algorithm without understanding it (and then can't adapt it to variants).

---

## 6. Step-by-Step Trace on a Real Example

Let's trace `nums = [3, 4, 2, 1, 2]` (so `n = 4`, and value `2` is the duplicate — it appears at indices 2 and 4).

**Building the functional graph** (`f(i) = nums[i]`):

```
i:      0   1   2   3   4
f(i):   3   4   2   1   2
```

Walking from `0`: `0 → 3 → 1 → 4 → 2 → 4 → 2 → 4 → 2 → ...`

```
0 ──▶ 3 ──▶ 1 ──▶ 4 ──▶ 2
                   ▲     │
                   │     ▼
                   └── (back to 4)
```

So: tail = `[0, 3, 1]` (μ = 3 steps to reach the cycle), cycle = `[4, 2]` (λ = 2), and the cycle entrance is node `4`... 

Wait — let's double check against the pigeonhole proof: the duplicate value is `2`, appearing at indices 2 and 4 (`nums[2] = 2` and `nums[4] = 2`). Both index 2 and index 4 point **into** node `2`. So node `2` has in-degree 2 — meaning **node `2`**, not node `4`, is the true cycle entrance (the point where two different paths converge). Let's re-examine: is `4` really in the cycle, or is `4` part of the tail leading into `2`?

Path: `0 → 3 → 1 → 4 → 2 → 4 → 2 → 4 → ...` — yes, `4` and `2` alternate forever, so both are in the cycle (`λ = 2`, cycle = `{4, 2}`). But which one is the *entrance* — the first cycle node reached from the tail? The tail is `0 → 3 → 1`, and the very next step is `1 → 4`. So `4` is the first node of the cycle encountered — **`4` is the entrance by the walk-order definition**, yet node `2` is the one with in-degree ≥ 2 (from indices 2 *and* 4).

This looks like a contradiction, but it isn't — it's resolved by realizing **both `2` and `4` receive an edge from inside the cycle, but only `2` additionally receives an edge from *outside* the cycle** (from index 2, which is not on this particular walk from `0`, but exists as an array slot nonetheless). Node `4` receives its only inbound edge from node `2` (in-cycle). Node `2` receives inbound edges from node `4` (in-cycle, `nums[4]=2`) **and** from node `2` itself... no — from **index 2**, i.e. `nums[2] = 2`, which is a self-loop-contributing edge: index `2`'s outgoing edge also lands on value `2`. Index `2` is itself *inside* the cycle. So actually both of node `2`'s inbound edges (from index 4 and from index 2) come from within the cycle — meaning by my in-degree argument, `2` should still be identifiable, but the "outside vs inside" framing above was an oversimplification for this specific small example where the tail is short.

**The precise, always-correct statement (restated from §3) is**: the duplicate value is the node with in-degree ≥ 2 in the *whole* graph — that's `2` here (in-edges from index 2 and index 4). Floyd's Phase 2 is proven (§7) to land on this exact node regardless of tail/cycle geometry specifics — trust the proof, not ad hoc walk tracing, which is why §7 exists. Let's now trace the actual pointer movements to see Phase 2 land on `2`.

**Phase 1 (slow +1, fast +2), start = 0:**

| step | slow | fast |
|---|---|---|
| init | `f(0)=3` | `f(f(0))=f(3)=1` |
| 1 | `f(3)=1` | `f(f(1))=f(4)=2` |
| 2 | `f(1)=4` | `f(f(2))=f(2)=2` |
| 3 | `f(4)=2` | `f(f(2))=f(2)=2` |

At step 3, `slow=2` and `fast=2` — **they meet at node `2`.**

**Phase 2 (reset slow2 = start = 0, move both +1):**

| step | slow2 | fast |
|---|---|---|
| init | `0` | `2` (carried over from Phase 1) |
| 1 | `f(0)=3` | `f(2)=2` |
| 2 | `f(3)=1` | `f(2)=2` |
| 3 | `f(1)=4` | `f(2)=2` |
| 4 | `f(4)=2` | `f(2)=2` |

At step 4, `slow2 = 2` and `fast = 2` — **they meet at node `2`, which is the duplicate.** ✅ Matches our earlier in-degree analysis exactly.

This trace shows the beautiful part: even though Phase 1's meeting point (`2`, reached at Phase-1-step 3) happened to *already be* the entrance in this example, that's coincidental to this input. In general, Phase 1's meeting point is somewhere *inside* the cycle but not necessarily the entrance, and Phase 2 is what walks it back to the true entrance. §7 proves this always works.

---

## 7. Why Phase 2 Finds the Cycle Entrance — Full Proof

Let:
- `μ` = tail length (steps from start to entrance `E`)
- `λ` = cycle length
- At the end of Phase 1, slow has taken `t` steps, fast has taken `2t` steps, and `xₜ = x₂ₜ` (met).

**Step 1 — show `t` is a multiple of `λ`.**

Since both `t ≥ μ` and `2t ≥ μ` (both pointers are inside the cycle by the time they meet — provable separately: fast enters the cycle first since it moves faster, and slow entering the cycle before meeting is required for a meeting to even be possible, since two points can only "collide" on the cycle, not on the tail, because the tail is a simple path with no repeated nodes), we can write positions inside the cycle as **distance past `E`, mod `λ`**:

```
position(xₜ)  ≡ (t − μ)  (mod λ)
position(x₂ₜ) ≡ (2t − μ) (mod λ)
```

Setting them equal (since `xₜ = x₂ₜ`):

```
t − μ ≡ 2t − μ   (mod λ)
     t ≡ 2t       (mod λ)
     0 ≡ t         (mod λ)
```

So **`t` is an exact multiple of `λ`.** This is the crucial fact — call `t = kλ` for some positive integer `k`.

**Step 2 — show that walking `μ` more steps from both the start and the meeting point lands both pointers on `E` simultaneously.**

Phase 2 puts one pointer at the start (`x₀`) and one at the meeting point (`xₜ`), then advances both by 1 step per iteration.

- The pointer starting at `x₀`: after `μ` more steps, it is at `x_μ = E` (by definition of `μ` — that's exactly what "tail length" means).
- The pointer starting at `xₜ` (where `t = kλ`): after `μ` more steps, it is at `x_{t+μ} = x_{kλ + μ}`. Since position inside the cycle is periodic with period `λ`, `x_{kλ + μ}` has the same cycle-position as `x_{μ}` (because `kλ` is a whole number of extra laps around the cycle, contributing `0` net displacement mod `λ`). And `x_μ = E`. So `x_{kλ+μ} = E` too.

**Both pointers reach `E` after exactly `μ` additional steps, simultaneously.** Since they move in lockstep (1 step each per iteration) and both were correct positions to begin with, they don't just "eventually agree" — they agree **for the first time at exactly `E`**, because for any step count `s < μ`, the pointer from `x₀` is still in the tail (a simple path — all distinct nodes, no repeats) while the pointer from `xₜ` is already circling inside the loop; a tail node and a cycle-interior node are never equal (the tail, by the ρ-shape's definition, touches the cycle only at `E`). So the *first* coincidence is necessarily at `s = μ`, i.e., at `E` itself. **∎**

This is why Phase 2 is guaranteed — not heuristically likely, but mathematically certain — to output the cycle entrance, which by §3's proof is exactly the duplicated array value.

---

## 8. Complexity Analysis

| Resource | Cost | Why |
|---|---|---|
| Time | `O(n)` | Phase 1 takes at most `O(μ + λ)` steps (bounded by `n+1`, the total node count); Phase 2 takes exactly `μ` more steps. Both phases are linear in the graph size. |
| Space | `O(1)` | Only three integer pointers (`slow`, `fast`, and later `slow2`) are ever allocated — no matter how large `n` is. |
| Array mutation | None | The array is only ever *read* via `nums[i]`, never written. |

Compare this to the constraint list in §1 — Floyd's is the **only** technique satisfying all four simultaneously (no mutation, O(1) space, sub-quadratic time, and in fact optimal O(n) time).

---

## 9. Alternative Approaches — Full Comparison

Understanding *why* Floyd's is uniquely suited here requires knowing what every other approach costs.

### 9.1 Hash Set — track seen values

```
seen = {}
for x in nums:
    if x in seen: return x
    seen.add(x)
```
- Time: `O(n)`. Space: `O(n)` — **violates the O(1) space constraint.**

### 9.2 Sorting

```
sort(nums)
for i in 1..n: if nums[i] == nums[i-1]: return nums[i]
```
- Time: `O(n log n)`. Space: `O(1)` if in-place sort is allowed, but **violates the "don't modify nums" constraint** (or costs `O(n)` space if you copy first).

### 9.3 Binary Search on the Value Range (Not the Array!)

This is the other "intended" approach for the follow-up, and worth knowing because interviewers often ask you to produce *both* Floyd's and this.

```
lo, hi = 1, n
while lo < hi:
    mid = (lo + hi) / 2
    count = number of elements in nums that are ≤ mid
    if count > mid:
        hi = mid       // duplicate is in [lo, mid]
    else:
        lo = mid + 1   // duplicate is in [mid+1, hi]
return lo
```
- **Why this works**: by pigeonhole, if the count of values `≤ mid` exceeds `mid`, then by the same pigeonhole argument as §3, the duplicate must lie in `[1, mid]` (there are only `mid` slots for `mid` distinct values, but more than `mid` elements claim to live there).
- Time: `O(n log n)` (binary search over `log n` range values, each requiring an `O(n)` scan to count). Space: `O(1)`, no mutation.
- **Trade-off vs Floyd's**: satisfies all constraints but is asymptotically slower (`O(n log n)` vs `O(n)`). Prefer it when you want an approach that's easier to explain/derive under interview pressure, or when the "cycle in an array" trick doesn't obviously transfer to a variant of the problem (e.g., if the problem is tweaked so the array-as-function trick breaks, binary-search-on-answer often still applies).

### 9.4 Bit Manipulation (XOR-based) — does **not** actually work for this problem

Bit-counting approaches (count set bits per bit-position across `nums` vs. across `[1..n]`) work for "find the single missing/duplicate number" variants **only when there's exactly one missing and one duplicate** (a permutation-with-one-swap scenario), not for the general LC 287 setup where a value can repeat more than twice and there's no guarantee of a missing complement. It's included here specifically as a **pitfall**: candidates sometimes reach for XOR tricks from "Single Number" (LC 136) and misapply them here. Know the difference.

### 9.5 Negative Marking (In-Place Hashing) — violates "don't modify"

```
for x in nums:
    idx = abs(x)
    if nums[idx] < 0: return idx
    nums[idx] = -nums[idx]
```
- Time `O(n)`, space `O(1)`, but **mutates the array** (even though it restores sign conventions, it's modifying values in place during execution) — violates constraint 1. Included because it's an extremely common `O(n)`/`O(1)` trick for adjacent problems (e.g., "find all duplicates," "find missing number") where mutation *is* allowed — good to know the boundary of when it's legal.

### 9.6 Comparison Table

| Approach | Time | Space | Mutates array? | Satisfies all LC287 constraints? |
|---|---|---|---|---|
| Hash Set | O(n) | O(n) | No | ❌ (space) |
| Sort | O(n log n) | O(1)/O(n) | Yes/No | ❌ (mutation or space) |
| Binary search on value range | O(n log n) | O(1) | No | ✅ |
| Negative marking | O(n) | O(1) | Yes | ❌ (mutation) |
| **Floyd's cycle detection** | **O(n)** | **O(1)** | **No** | **✅ (best on every axis)** |

---

## 10. Edge Cases and Correctness Pitfalls

1. **Off-by-one in the starting node.** Always start the walk at index `0`, *not* at `nums[0]` directly as the first "step" without applying `f`. A common bug is initializing `slow = nums[0]` and `fast = nums[nums[0]]` vs. `slow = 0; slow = f(slow)` — these are equivalent if done correctly, but it's easy to accidentally apply `f` one time too many or too few, which desyncs the "same speed after phase 1" logic. Be disciplined: `slow = f(start)`, `fast = f(f(start))` before the loop condition check.
2. **The duplicate appearing more than twice.** The algorithm doesn't care how many times the value repeats — the in-degree of the cycle-entrance node can be higher than 2, and the proof in §3/§7 doesn't depend on it being exactly 2. No special-casing needed.
3. **Value `n` itself as the duplicate.** Since values range `[1, n]` and there are `n+1` slots, `n` is a perfectly valid duplicate candidate and the algorithm handles it identically to any other value — no boundary bug here as long as your loop bounds treat the array as `0..n` inclusive of index `n`.
4. **Never treat index `0`'s *value* as special.** `nums[0]` might equal the duplicate, might not — irrelevant to correctness. Don't special-case it.
5. **Infinite loop from a coding mistake, not a math mistake.** If you accidentally advance `fast` only once (typo: `fast = f(fast)` instead of `fast = f(f(fast))`) in Phase 1, `slow` and `fast` move at the same speed and may never meet within a bounded number of iterations if they start at different offsets — always double-check the "2-steps" is truly two applications of `f`, not one.
6. **Language-specific integer overflow.** Not relevant here since indices/values are bounded `≤ 10^5`, but worth noting as a general Floyd's-algorithm hygiene point when adapting this pattern to problems with large numeric ranges (e.g., pseudorandom number generator period-finding) — always confirm the "next" function can't silently overflow into undefined behavior in C.

---

## 11. Implementation: Go

Production-style Go, with explicit error handling for invariant violations (defensive — a "real world" library function should not trust caller input blindly even when a LeetCode judge guarantees it).

```go
// Package duplicate implements Floyd's cycle-detection algorithm
// to solve LeetCode 287 — Find the Duplicate Number — in O(n) time
// and O(1) extra space, without mutating the input slice.
package duplicate

import "errors"

// ErrInvalidInput is returned when nums does not satisfy the problem's
// preconditions: len(nums) == n+1 and every value in [1, n].
var ErrInvalidInput = errors.New("duplicate: nums does not satisfy problem constraints")

// FindDuplicate returns the single repeated value in nums using
// Floyd's tortoise-and-hare cycle detection, treating nums as an
// implicit functional graph where f(i) = nums[i].
//
// Time complexity:  O(n)
// Space complexity: O(1)
// nums is never modified.
func FindDuplicate(nums []int) (int, error) {
	n := len(nums) - 1 // values live in [1, n]
	if n < 1 {
		return 0, ErrInvalidInput
	}
	for _, v := range nums {
		if v < 1 || v > n {
			return 0, ErrInvalidInput
		}
	}

	// --- Phase 1: find a meeting point inside the cycle ---
	// Start both pointers at the "virtual" node 0 (a safe start since
	// no array value is ever 0, guaranteeing index 0 has in-degree 0
	// and therefore cannot be part of the cycle).
	slow, fast := nums[0], nums[nums[0]]
	for slow != fast {
		slow = nums[slow]
		fast = nums[nums[fast]]
	}

	// --- Phase 2: find the cycle entrance (== the duplicate) ---
	slow2 := 0
	for slow2 != fast {
		slow2 = nums[slow2]
		fast = nums[fast]
	}

	return slow2, nil
}
```

**Idiomatic-Go notes:**
- Returning `(int, error)` rather than panicking mirrors how a real Go library should treat precondition violations — LeetCode's judge guarantees valid input, but a reusable package shouldn't assume its caller does.
- The validation loop is `O(n)` and doesn't affect the algorithm's asymptotic bound, but in an actual production deployment you might gate it behind a `debug` build tag if the hot path needs to skip validation for performance — worth knowing as a real-world trade-off, not just LeetCode purity.

**Table-driven test (Go convention):**

```go
package duplicate

import "testing"

func TestFindDuplicate(t *testing.T) {
	cases := []struct {
		name string
		nums []int
		want int
	}{
		{"basic",              []int{1, 3, 4, 2, 2}, 2},
		{"duplicate is n",     []int{3, 1, 3, 4, 2}, 3},
		{"duplicate repeats 3x", []int{2, 2, 2, 2, 2}, 2},
		{"minimal n=1",        []int{1, 1}, 1},
		{"duplicate at start", []int{2, 6, 4, 1, 3, 1, 5}, 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := FindDuplicate(c.nums)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("FindDuplicate(%v) = %d, want %d", c.nums, got, c.want)
			}
		})
	}
}

func TestFindDuplicate_InvalidInput(t *testing.T) {
	_, err := FindDuplicate([]int{0, 1}) // 0 is out of the valid [1, n] range
	if err == nil {
		t.Fatal("expected ErrInvalidInput, got nil")
	}
}
```

---

## 12. Implementation: C

C requires us to be explicit about everything Go and Rust give us for free — no slice headers, no built-in bounds safety, manual reasoning about pointer/value semantics. This version follows LeetCode's actual C function signature convention.

```c
#include <stdio.h>
#include <stdlib.h>
#include <assert.h>

/*
 * findDuplicate — Floyd's cycle detection (tortoise & hare).
 *
 * Preconditions (caller's responsibility, asserted in debug builds):
 *   - numsSize == n + 1 for some n >= 1
 *   - every nums[i] is in the closed range [1, n]
 *   - exactly one value repeats (at least twice)
 *
 * Returns the repeated value.
 * Time:  O(n)
 * Space: O(1)
 * Does NOT modify nums.
 */
int findDuplicate(int *nums, int numsSize) {
    assert(nums != NULL);
    assert(numsSize >= 2); /* n >= 1 implies numsSize >= 2 */

#ifndef NDEBUG
    /* Debug-only validation: confirm every value is in [1, n].
     * Skipped in release builds (NDEBUG defined) to keep the hot
     * path strictly O(n) with minimal constant factor, matching
     * what a performance-critical embedded/systems context needs. */
    int n = numsSize - 1;
    for (int i = 0; i < numsSize; i++) {
        assert(nums[i] >= 1 && nums[i] <= n);
    }
#endif

    /* Phase 1: locate a meeting point inside the cycle. */
    int slow = nums[0];
    int fast = nums[nums[0]];

    while (slow != fast) {
        slow = nums[slow];
        fast = nums[nums[fast]];
    }

    /* Phase 2: walk one pointer from the true start (index 0) and
     * the other from the meeting point, one step at a time, until
     * they coincide — that node is the cycle entrance == duplicate. */
    int slow2 = 0;
    while (slow2 != fast) {
        slow2 = nums[slow2];
        fast  = nums[fast];
    }

    return slow2;
}

/* --- Minimal manual test harness (no external framework) --- */
static void run_case(const char *name, int *nums, int size, int expected) {
    int got = findDuplicate(nums, size);
    printf("[%s] expected=%d got=%d -> %s\n",
           name, expected, got, (got == expected) ? "PASS" : "FAIL");
}

int main(void) {
    int a[] = {1, 3, 4, 2, 2};
    run_case("basic", a, 5, 2);

    int b[] = {3, 1, 3, 4, 2};
    run_case("duplicate is n", b, 5, 3);

    int c[] = {2, 2, 2, 2, 2};
    run_case("duplicate repeats many times", c, 5, 2);

    int d[] = {1, 1};
    run_case("minimal n=1", d, 2, 1);

    return 0;
}
```

**C-specific notes for real-world code:**
- `assert()` calls compile away entirely when `NDEBUG` is defined (the standard release-build convention) — this is how you get "defensive in debug, zero-overhead in release" without hand-rolling a flag system.
- No dynamic allocation anywhere — this function is safe to call from an interrupt handler or other allocation-forbidden context (relevant if you were, say, embedding this logic in a kernel module or firmware — same shape of reasoning you'd apply in your eBPF/XDP work, where you can't allocate on the hot path either).
- Array decays to pointer at the call boundary (`int *nums`); we never take ownership or free anything, consistent with "read-only, no mutation" semantics.

---

## 13. Implementation: Rust

Rust gives us the chance to make the "read-only, no mutation" guarantee **enforced by the type system** rather than just a convention — a shared reference `&[usize]` cannot be mutated at all, so the compiler proves the constraint for us.

```rust
/// Errors returned when the input slice does not satisfy LC287's
/// preconditions.
#[derive(Debug, PartialEq, Eq)]
pub enum DuplicateError {
    TooShort,
    ValueOutOfRange { index: usize, value: usize, max: usize },
}

impl std::fmt::Display for DuplicateError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            DuplicateError::TooShort => write!(f, "nums must have length >= 2"),
            DuplicateError::ValueOutOfRange { index, value, max } => write!(
                f,
                "nums[{index}] = {value} is out of the required range [1, {max}]"
            ),
        }
    }
}

impl std::error::Error for DuplicateError {}

/// Finds the single duplicate value in `nums` using Floyd's
/// cycle-detection algorithm (tortoise and hare).
///
/// `nums` is taken as a shared slice `&[usize]` — the borrow checker
/// statically guarantees this function cannot mutate the caller's
/// data, enforcing the "don't modify nums" constraint at compile time
/// rather than by convention.
///
/// Time:  O(n)
/// Space: O(1)
pub fn find_duplicate(nums: &[usize]) -> Result<usize, DuplicateError> {
    let len = nums.len();
    if len < 2 {
        return Err(DuplicateError::TooShort);
    }
    let n = len - 1;

    for (i, &v) in nums.iter().enumerate() {
        if v < 1 || v > n {
            return Err(DuplicateError::ValueOutOfRange {
                index: i,
                value: v,
                max: n,
            });
        }
    }

    // --- Phase 1: find a meeting point inside the cycle ---
    let mut slow = nums[0];
    let mut fast = nums[nums[0]];

    while slow != fast {
        slow = nums[slow];
        fast = nums[nums[fast]];
    }

    // --- Phase 2: walk to the cycle entrance (the duplicate) ---
    let mut slow2 = 0usize;
    while slow2 != fast {
        slow2 = nums[slow2];
        fast = nums[fast];
    }

    Ok(slow2)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn basic_case() {
        assert_eq!(find_duplicate(&[1, 3, 4, 2, 2]), Ok(2));
    }

    #[test]
    fn duplicate_is_n() {
        assert_eq!(find_duplicate(&[3, 1, 3, 4, 2]), Ok(3));
    }

    #[test]
    fn duplicate_repeats_many_times() {
        assert_eq!(find_duplicate(&[2, 2, 2, 2, 2]), Ok(2));
    }

    #[test]
    fn minimal_n_equals_one() {
        assert_eq!(find_duplicate(&[1, 1]), Ok(1));
    }

    #[test]
    fn duplicate_at_start_of_walk() {
        assert_eq!(find_duplicate(&[2, 6, 4, 1, 3, 1, 5]), Ok(1));
    }

    #[test]
    fn rejects_too_short_input() {
        assert_eq!(find_duplicate(&[1]), Err(DuplicateError::TooShort));
    }

    #[test]
    fn rejects_out_of_range_value() {
        assert_eq!(
            find_duplicate(&[0, 1]),
            Err(DuplicateError::ValueOutOfRange { index: 0, value: 0, max: 1 })
        );
    }
}
```

**Rust-specific notes:**
- Using `usize` directly (rather than `i32` as LeetCode's signature technically expects) is idiomatic here because these values are *always* used as indices — a systems-Rust habit worth having: let the type reflect the value's actual role (index vs. arbitrary integer), not just "whatever the judge's signature says." If you need to match LeetCode's exact `Vec<i32>` signature, wrap this function with a thin adapter that validates non-negativity and casts.
- `Result<usize, DuplicateError>` with a proper `Display`/`Error` impl is the idiomatic way to surface precondition failures in a Rust library — panics (`unwrap`, `expect`) should be reserved for truly unrecoverable states, not for input validation a caller might reasonably want to handle.
- No `unsafe` blocks anywhere. This is a place where Rust's ownership model doesn't cost you anything relative to C — the algorithm is naturally free of aliasing/mutation hazards, so safe Rust expresses it with zero overhead versus the C version.

---

## 14. Real-World Applications of Floyd's Algorithm

Floyd's cycle detection is not a LeetCode-only curiosity — it's load-bearing infrastructure in several real systems, worth knowing because it demonstrates the *generality* of the "functional graph / rho shape" mental model beyond arrays:

1. **Linked list cycle detection (LC 141 / LC 142).** The original, canonical use case — detecting whether a linked list has a cycle (e.g., a corrupted or maliciously-crafted list causing an infinite loop in a naive traversal), and finding exactly where the cycle begins. This is directly applicable to defensive parsing of untrusted linked structures.

2. **Pseudorandom number generator (PRNG) period-finding.** Many PRNGs (e.g., simple LCGs — Linear Congruential Generators) are literally functions `f(state) = (a * state + c) mod m` iterated over and over — exactly a functional graph. Floyd's algorithm is the standard technique for finding a PRNG's period without unbounded memory, relevant when auditing weak PRNGs for security purposes (a period that's too short is a cryptographic weakness).

3. **Cryptographic collision search (Pollard's rho algorithm).** Pollard's rho for integer factorization and for discrete-logarithm attacks is *literally* Floyd's cycle detection applied to a pseudo-random function over integers mod N — the "rho" in Pollard's rho *is* this same ρ-shape. Used in real cryptanalysis tooling.

4. **Hash chain / bucket cycle detection.** In some open-addressing or chained hash-table implementations, a bug (or adversarial input in a hash-flooding attack) can create a cycle in the probe sequence; cycle detection in O(1) space is used defensively to bound worst-case lookup time.

5. **Functional dependency / state machine analysis.** Detecting infinite loops in state transition systems (e.g., verifying a protocol state machine or a build-dependency graph doesn't loop) where the transition function is deterministic and the state space is finite — the same rho-shape reasoning applies directly.

6. **Given your Cilium / eBPF / networking background specifically**: any place you're walking a fixed-arity, deterministic "next" structure with attacker-influenced input — e.g., verifying a routing/lookup table (like a trie or hash-based LPM structure) has no cyclic references before installing it into a kernel path — Floyd's O(1)-space guarantee is exactly the property you want when you can't allocate a "visited" set on a hot or memory-constrained path (this is precisely the same constraint that makes Floyd's the intended solution here: constant space, no mutation, single linear pass).

---

## 15. Expert Mental Model — How to Internalize This Forever

To truly own this pattern (not just memorize the two-phase loop), hold onto these five compressed ideas:

1. **"Constrained values that are also valid indices" is the tell.** Whenever you see an array where `nums[i] ∈ [some range matching valid indices]`, your first reflex should be: *can I treat this array as `f(i) = nums[i]` and walk it as an implicit graph?* This single pattern-recognition trigger solves LC 287, and transfers to problems like "First Missing Positive" (LC 41) and "Find All Duplicates in an Array" (LC 442), even though those use different specific techniques (in-place marking rather than cycle detection) — the *shared* insight is "index-value duality."

2. **Pigeonhole is the proof engine behind almost every `O(1)`-space "find the anomaly" problem.** More items than slots ⟹ something must collide. Internalize this as your default first move whenever a problem says "n+1 items in range [1,n]" or similar cardinality mismatches.

3. **The rho shape separates "how you got there" (tail) from "where you loop" (cycle).** This split — a simple, non-repeating path merging into a repeating loop — is the structural signature of *any* deterministic, finite-state, single-successor system. Once you can draw the ρ for a problem, Floyd's two-phase algorithm is just "mechanically walk the geometry you already understand," not a magic formula.

4. **Phase 1 finds *any* point on the cycle; Phase 2 finds the *specific* point (the entrance) by exploiting that `t` (Phase 1's step count) is provably a multiple of `λ`.** If you forget the algorithm's code but remember this one sentence, you can re-derive both loops from scratch under interview pressure — which is a far more valuable, durable skill than rote memorization.

5. **Constant space is a *design constraint*, not a curiosity — it's what makes this the "systems-appropriate" answer.** Every alternative in §9 either allocates linear extra memory or mutates shared state. In systems/kernel contexts (your domain), "cannot allocate, cannot mutate caller's data, must terminate in bounded time" is a *routine* real constraint (interrupt context, lock-free hot paths, read-only shared structures) — so this problem is genuinely representative of a recurring real engineering situation, not just an interview trick.

---

## 16. Testing Matrix

A thorough test suite for this algorithm (mirrored across all three language implementations above) should include:

| Case | Why it matters |
|---|---|
| Minimal input, `n = 1`, `nums = [1, 1]` | Smallest valid input; tail length `μ = 0`. |
| Duplicate value equals `n` (the maximum possible value) | Ensures no off-by-one at the upper boundary. |
| Duplicate appears 3+ times | Confirms the algorithm doesn't assume exactly 2 occurrences. |
| Duplicate positioned so the cycle entrance is reached immediately from index 0 (`μ = 0`) | Exercises the boundary where Phase 2's loop may terminate in 0 iterations. |
| Duplicate positioned so the tail is long relative to array size (`μ` close to `n`) | Exercises a long tail / short cycle geometry. |
| Large randomized input (`n = 10^5`) with a known injected duplicate | Performance/regression test — confirms true `O(n)` behavior, no accidental quadratic blowup. |
| Invalid input: value `0` present | Validates defensive precondition checking (Go/Rust versions return errors; C version asserts in debug builds). |
| Invalid input: value `> n` present | Same as above. |

---

## 17. Summary Cheat Sheet

```
PROBLEM:      n+1 values in [1, n] → exactly one duplicate.
KEY INSIGHT:  treat nums as a function f(i) = nums[i] → implicit linked list.
WHY A CYCLE
EXISTS:       finite domain + single successor ⟹ must repeat (pigeonhole).
WHY DUPLICATE
== ENTRANCE:  duplicate value ⟺ node with in-degree ≥ 2 ⟺ cycle entrance.

PHASE 1  (find any point on the cycle):
    slow = f(start);        fast = f(f(start))
    while slow != fast: slow = f(slow); fast = f(f(fast))

PHASE 2  (find the entrance / duplicate):
    slow2 = start
    while slow2 != fast: slow2 = f(slow2); fast = f(fast)
    return slow2

COMPLEXITY:   O(n) time, O(1) space, zero mutation.
PROOF CORE:   Phase-1 step count t satisfies t ≡ 0 (mod λ), which makes
              Phase 2's lockstep walk converge exactly at the entrance.
```
